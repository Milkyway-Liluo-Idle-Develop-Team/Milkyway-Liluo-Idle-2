package attribute

import "github.com/edrowsluo/new-mli/backend/internal/record"

// Instance holds the modifier state, cache, and dirty flags for one player.
// It is created when a WebSocket connection is established, lives for the
// connection's duration, and is garbage-collected on disconnect.
type Instance struct {
	reg *Registry

	// byAttr[attrID] = all modifiers targeting that attribute, sorted by priority.
	byAttr map[AttributeID][]Modifier

	// bySource[source] records which attrIDs a source touches, so RemoveModifiers
	// can efficiently rebuild the affected attr lists.
	bySource map[string]sourceInfo

	cached map[AttributeID]float64
	dirty  map[AttributeID]bool

	// recorder is set by the settlement engine during execution cycles.
	// When non-nil, markDirty writes an AttributeDirtyRecord to the current
	// namespace. Nil outside of settlement (no-op for dirty marking).
	recorder *record.Recorder
}

type sourceInfo struct {
	mods    []Modifier
	attrIDs []AttributeID // which attrs this source contributed to
}

// NewInstance creates an empty Instance and mounts the static modifiers from
// the global Registry. The player-specific modifiers (equipment, tools,
// skills, buffs) are added later via AddModifiers / UpdateModifiers.
func NewInstance() *Instance {
	r := Get()
	inst := &Instance{
		reg:      r,
		byAttr:   make(map[AttributeID][]Modifier),
		bySource: make(map[string]sourceInfo),
		cached:   make(map[AttributeID]float64),
		dirty:    make(map[AttributeID]bool),
	}

	// Mount static modifiers (from attributes.json's modifiers section).
	staticMods := r.StaticMods()
	if len(staticMods) > 0 {
		for _, m := range staticMods {
			inst.byAttr[m.AttrID] = append(inst.byAttr[m.AttrID], m)
		}
		// Mark all static-modified attrs as dirty.
		for _, m := range staticMods {
			inst.markDirty(m.AttrID)
		}
	}

	return inst
}

// SetRecorder attaches a Recorder for the current execution cycle.
// Subsequent markDirty calls will write records to it.
func (inst *Instance) SetRecorder(rec *record.Recorder) {
	inst.recorder = rec
}

// ClearRecorder detaches the current Recorder.
func (inst *Instance) ClearRecorder() {
	inst.recorder = nil
}

// AddModifiers adds a batch of modifiers from a given source. If the source
// already exists, it is atomically replaced (UpdateModifiers semantics).
// Marks affected attributes as dirty.
func (inst *Instance) AddModifiers(source string, mods []Modifier) {
	if existing, ok := inst.bySource[source]; ok {
		inst.removeSourceLocked(source, existing)
	}

	attrIDs := make([]AttributeID, 0, len(mods))
	for _, m := range mods {
		attrIDs = append(attrIDs, m.AttrID)
		inst.byAttr[m.AttrID] = append(inst.byAttr[m.AttrID], m)
	}
	inst.bySource[source] = sourceInfo{mods: mods, attrIDs: attrIDs}

	for _, aid := range attrIDs {
		inst.markDirty(aid)
	}
}

// RemoveModifiers removes all modifiers from the given source.
func (inst *Instance) RemoveModifiers(source string) {
	info, ok := inst.bySource[source]
	if !ok {
		return
	}
	inst.removeSourceLocked(source, info)
}

func (inst *Instance) removeSourceLocked(source string, info sourceInfo) {
	for _, aid := range info.attrIDs {
		// Rebuild the attr's modifier list, filtering out this source.
		old := inst.byAttr[aid]
		filtered := make([]Modifier, 0, len(old))
		for _, m := range old {
			if m.Source != source {
				filtered = append(filtered, m)
			}
		}
		inst.byAttr[aid] = filtered
		inst.markDirty(aid)
	}
	delete(inst.bySource, source)
}

// UpdateModifiers atomically replaces all modifiers from the given source.
func (inst *Instance) UpdateModifiers(source string, mods []Modifier) {
	if existing, ok := inst.bySource[source]; ok {
		inst.removeSourceLocked(source, existing)
	}
	inst.AddModifiers(source, mods)
}

// Dirty reports whether the given attribute needs recalculation.
func (inst *Instance) Dirty(attrID AttributeID) bool {
	return inst.dirty[attrID]
}

// DirtyCount returns the number of dirty attributes (for diagnostics).
func (inst *Instance) DirtyCount() int { return len(inst.dirty) }

// ModifiersFor returns all active modifiers for the given attribute,
// in priority order. Used during diff packet construction to send the
// complete modifier state alongside the final value.
func (inst *Instance) ModifiersFor(attrID AttributeID) []Modifier {
	return append([]Modifier(nil), inst.byAttr[attrID]...)
}
