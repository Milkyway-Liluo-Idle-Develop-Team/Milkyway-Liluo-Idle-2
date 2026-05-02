package equipment

import (
	"github.com/edrowsluo/new-mli/backend/internal/item"
	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

// Bucket collects equipment changes within a single namespace.
// Per slot, only the last action is retained.
type Bucket struct {
	actions map[string]slotAction
}

type slotAction struct {
	item   item.Item
	action pb.EquipAction
}

var _ record.RecordBucket = (*Bucket)(nil)

func newBucket() *Bucket {
	return &Bucket{actions: make(map[string]slotAction)}
}

func (b *Bucket) addAction(slot string, it item.Item, action pb.EquipAction) {
	b.actions[slot] = slotAction{item: it, action: action}
}

func (b *Bucket) SystemName() string { return "equipment" }

func (b *Bucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*Bucket)
	for slot, a := range ob.actions {
		b.actions[slot] = a
	}
}

func (b *Bucket) SerializeDiff() (proto.Message, error) {
	if len(b.actions) == 0 {
		return nil, nil
	}
	diffs := make([]*pb.EquipmentDiff, 0, len(b.actions))
	for slot, a := range b.actions {
		diffs = append(diffs, &pb.EquipmentDiff{
			Slot:      slot,
			ItemId:    int32(a.item.ID),
			ItemState: int32(a.item.State),
			Action:    a.action,
		})
	}
	return &pb.StateDiff{Equipment: diffs}, nil
}

func (b *Bucket) IsEmpty() bool { return len(b.actions) == 0 }
