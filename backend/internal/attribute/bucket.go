package attribute

import (
	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
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
func (b *Bucket) SerializeDiff() (proto.Message, error) {
	if len(b.dirty) == 0 {
		return nil, nil
	}

	diffs := make([]*pb.AttributeDiff, 0, len(b.dirty))
	for id := range b.dirty {
		finalVal := b.instance.GetFinal(id)
		mods := b.instance.ModifiersFor(id)

		wmods := make([]*pb.ModifierWire, 0, len(mods))
		for _, m := range mods {
			wm := &pb.ModifierWire{
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
		diffs = append(diffs, &pb.AttributeDiff{
			AttrId:     strID,
			FinalValue: finalVal,
			Modifiers:  wmods,
		})
	}

	return &pb.StateDiff{Attribute: diffs}, nil
}

// IsEmpty reports whether no attributes are dirty.
func (b *Bucket) IsEmpty() bool {
	return len(b.dirty) == 0
}
