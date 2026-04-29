package attribute

import (
	"encoding/json"

	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// Bucket collects dirty attribute IDs within a single namespace.
// Duplicate IDs are naturally deduplicated via the internal map.
type Bucket struct {
	instance *Instance
	dirty    map[AttributeID]bool
}

var _ record.RecordBucket = (*Bucket)(nil)

// NewBucket creates an empty Bucket. The Instance reference is set lazily
// when MarkDirty is first called via the Recorder.
func NewBucket(inst *Instance) *Bucket {
	return &Bucket{
		instance: inst,
		dirty:    make(map[AttributeID]bool),
	}
}

// setInstance binds the bucket to an Instance.
func (b *Bucket) setInstance(inst *Instance) {
	b.instance = inst
}

// MarkDirty records an attribute ID as dirty in this bucket.
func (b *Bucket) MarkDirty(id AttributeID) {
	b.dirty[id] = true
}

// SystemName returns "attribute".
func (b *Bucket) SystemName() string { return "attribute" }

// MergeInPlace merges another Bucket's dirty set into this one.
func (b *Bucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*Bucket)
	for id := range ob.dirty {
		b.dirty[id] = true
	}
}

// SerializeDiff computes the final value and collects the full modifier
// state for every dirty attribute, then serializes the result.
func (b *Bucket) SerializeDiff() (json.RawMessage, error) {
	if len(b.dirty) == 0 {
		return json.RawMessage("[]"), nil
	}

	type modWire struct {
		Source  string  `json:"source"`
		Op      string  `json:"op"`
		Value   float64 `json:"value,omitempty"`
		RefAttr string  `json:"ref_attr,omitempty"`
		Display string  `json:"display,omitempty"`
	}

	type attrDiff struct {
		AttrID     string    `json:"attr_id"`
		FinalValue float64   `json:"final_value"`
		Modifiers  []modWire `json:"modifiers"`
	}

	out := make([]attrDiff, 0, len(b.dirty))
	for id := range b.dirty {
		finalVal := b.instance.GetFinal(id)
		mods := b.instance.ModifiersFor(id)

		wmods := make([]modWire, 0, len(mods))
		for _, m := range mods {
			wm := modWire{
				Source:  m.Source,
				Op:      string(m.Op),
				Display: string(m.Display),
			}
			if m.IsRef() {
				if s, ok := b.instance.reg.AttrString(m.RefAttr); ok {
					wm.RefAttr = s
				}
			} else {
				wm.Value = m.Value
			}
			wmods = append(wmods, wm)
		}

		strID, _ := b.instance.reg.AttrString(id)
		out = append(out, attrDiff{
			AttrID:     strID,
			FinalValue: finalVal,
			Modifiers:  wmods,
		})
	}

	return json.Marshal(out)
}

// IsEmpty reports whether no attributes are dirty.
func (b *Bucket) IsEmpty() bool {
	return len(b.dirty) == 0
}
