package attribute

import "sort"

// AttrDef is the metadata for a single attribute, loaded from attributes.json.
type AttrDef struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	DefaultValue float64   `json:"default_value"`
	MinValue     *float64  `json:"min_value,omitempty"`
	MaxValue     *float64  `json:"max_value,omitempty"`
	Direction    Direction `json:"direction"`
	Group        string    `json:"group"`
	Desc         string    `json:"desc,omitempty"`
}

// staticModJSON mirrors the modifiers section in attributes.json.
type staticModJSON struct {
	AttrID   string  `json:"attr_id"`
	Op       OpType  `json:"op"`
	Value    float64 `json:"value,omitempty"`
	RefAttr  string  `json:"ref_attr,omitempty"`
	Display  string  `json:"display,omitempty"`
	Priority int     `json:"priority,omitempty"`
}

// attrsConfig is the root structure of attributes.json.
type attrsConfig struct {
	Version    string         `json:"version"`
	Attributes []AttrDef      `json:"attributes"`
	Modifiers  []staticModJSON `json:"modifiers,omitempty"`
}

// Registry is the global, read-only attribute registry.
// It is populated at startup from the embedded attributes.json and
// attr_registry.json, and shared by all PlayerSession instances.
type Registry struct {
	// Definitions indexed by numeric and string id.
	byID     map[AttributeID]AttrDef
	byString map[string]AttrDef

	// Bidirectional string ↔ numeric mapping.
	attrIDs    map[string]AttributeID
	idToString map[AttributeID]string

	// Static modifiers applied to every player on creation.
	staticMods []Modifier

	// Dependency graph: forward[attrID] = list of attrIDs that attrID depends on.
	// reverse[attrID] = list of attrIDs that depend on attrID.
	// Slices are immutable after construction via newRegistry.
	forward map[AttributeID][]AttributeID
	reverse map[AttributeID][]AttributeID
}

// newRegistry builds a Registry from parsed config and numeric ID mapping.
func newRegistry(cfg attrsConfig, attrIDs map[string]AttributeID) (*Registry, error) {
	r := &Registry{
		byID:       make(map[AttributeID]AttrDef, len(cfg.Attributes)),
		byString:   make(map[string]AttrDef, len(cfg.Attributes)),
		attrIDs:    attrIDs,
		idToString: make(map[AttributeID]string, len(attrIDs)),
		forward:    make(map[AttributeID][]AttributeID),
		reverse:    make(map[AttributeID][]AttributeID),
	}

	// Build reverse index string → numeric.
	for s, id := range attrIDs {
		r.idToString[id] = s
	}

	for _, def := range cfg.Attributes {
		id, ok := attrIDs[def.ID]
		if !ok {
			return nil, &missingAttrIDError{def.ID}
		}
		if _, dup := r.byID[id]; dup {
			return nil, &duplicateAttrIDError{id, def.ID}
		}
		r.byID[id] = def
		r.byString[def.ID] = def
	}

	// Resolve static modifiers.
	for _, sm := range cfg.Modifiers {
		targetID, ok := attrIDs[sm.AttrID]
		if !ok {
			return nil, &missingAttrIDError{sm.AttrID}
		}
		display := DisplayFixed
		if sm.Display != "" {
			display = DisplayMode(sm.Display)
		}
		m := Modifier{
			AttrID:   targetID,
			Op:       sm.Op,
			Display:  display,
			Source:   "base",
			Priority: sm.Priority,
		}
		if sm.RefAttr != "" {
			refID, ok := attrIDs[sm.RefAttr]
			if !ok {
				return nil, &missingAttrIDError{sm.RefAttr}
			}
			m.RefAttr = refID
		} else {
			m.Value = sm.Value
		}
		r.staticMods = append(r.staticMods, m)

		// Record dependency.
		if m.IsRef() {
			r.forward[targetID] = append(r.forward[targetID], m.RefAttr)
			r.reverse[m.RefAttr] = append(r.reverse[m.RefAttr], targetID)
		}
	}

	// Detect circular dependencies.
	if err := r.checkCycles(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Registry) checkCycles() error {
	// Standard DFS-based cycle detection.
	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // fully processed
	)
	state := make(map[AttributeID]int)

	var dfs func(id AttributeID) []AttributeID
	dfs = func(id AttributeID) []AttributeID {
		state[id] = gray
		for _, dep := range r.forward[id] {
			switch state[dep] {
			case gray:
				return []AttributeID{dep, id}
			case white:
				if cycle := dfs(dep); cycle != nil {
					if cycle[0] == id {
						return append(cycle, id)
					}
					return append(cycle, id)
				}
			}
		}
		state[id] = black
		return nil
	}

	for id := range r.byID {
		if state[id] == white {
			if cycle := dfs(id); cycle != nil {
				return &cycleError{cycle}
			}
		}
	}
	return nil
}

// --- Accessors ---

// Def returns the attribute definition for the given numeric id.
func (r *Registry) Def(id AttributeID) (AttrDef, bool) {
	d, ok := r.byID[id]
	return d, ok
}

// DefByString returns the attribute definition for the given string id.
func (r *Registry) DefByString(s string) (AttrDef, bool) {
	d, ok := r.byString[s]
	return d, ok
}

// AttrID returns the numeric id for the given string id.
func (r *Registry) AttrID(s string) (AttributeID, bool) {
	id, ok := r.attrIDs[s]
	return id, ok
}

// AttrString returns the string id for the given numeric id. O(1).
func (r *Registry) AttrString(id AttributeID) (string, bool) {
	s, ok := r.idToString[id]
	return s, ok
}

// AllIDs returns all attribute numeric ids in ascending order.
func (r *Registry) AllIDs() []AttributeID {
	out := make([]AttributeID, 0, len(r.byID))
	for id := range r.byID {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Count returns the number of registered attributes.
func (r *Registry) Count() int { return len(r.byID) }

// StaticMods returns the static modifier templates (from attributes.json).
func (r *Registry) StaticMods() []Modifier {
	return append([]Modifier(nil), r.staticMods...)
}

// ReverseDeps returns the IDs of attributes that depend on the given attrID.
// The returned slice is read-only (Registry is immutable after load).
func (r *Registry) ReverseDeps(attrID AttributeID) []AttributeID {
	return r.reverse[attrID]
}

// ForwardDeps returns the IDs of attributes that attrID depends on.
func (r *Registry) ForwardDeps(attrID AttributeID) []AttributeID {
	return r.forward[attrID]
}

// --- Error types ---

type missingAttrIDError struct{ id string }
func (e *missingAttrIDError) Error() string {
	return "attribute: id \"" + e.id + "\" found in attributes.json but missing from attr_registry.json; run genregistry"
}

type duplicateAttrIDError struct {
	id  AttributeID
	str string
}
func (e *duplicateAttrIDError) Error() string {
	return "attribute: duplicate numeric id " + e.id.String() + " for \"" + e.str + "\""
}

type cycleError struct {
	cycle []AttributeID
}
func (e *cycleError) Error() string {
	s := "attribute: circular dependency detected: "
	for i := len(e.cycle) - 1; i >= 0; i-- {
		if i < len(e.cycle)-1 {
			s += " -> "
		}
		s += e.cycle[i].String()
	}
	return s
}
