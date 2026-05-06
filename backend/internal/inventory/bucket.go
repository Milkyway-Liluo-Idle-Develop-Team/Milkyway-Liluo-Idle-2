package inventory

import (
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

type changeEntry struct {
	qty    float64
	reason pb.InventoryChangeReason
}

// Bucket collects InventoryChangeRecords within a single namespace.
// Records with the same item.Item identity are automatically merged.
type Bucket struct {
	changes map[item.Item]*changeEntry
}

var _ record.RecordBucket = (*Bucket)(nil)

func newBucket() *Bucket {
	return &Bucket{changes: make(map[item.Item]*changeEntry)}
}

func (b *Bucket) add(it item.Item, qty float64, reason pb.InventoryChangeReason) {
	if e, ok := b.changes[it]; ok {
		e.qty += qty
		if e.qty == 0 {
			delete(b.changes, it)
		}
	} else {
		b.changes[it] = &changeEntry{qty: qty, reason: reason}
	}
}

func (b *Bucket) SystemName() string { return "inventory" }

func (b *Bucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*Bucket)
	for it, e := range ob.changes {
		if existing, ok := b.changes[it]; ok {
			existing.qty += e.qty
			if existing.qty == 0 {
				delete(b.changes, it)
			}
		} else {
			b.changes[it] = &changeEntry{qty: e.qty, reason: e.reason}
		}
	}
}

func (b *Bucket) SerializeDiff() (proto.Message, error) {
	if len(b.changes) == 0 {
		return nil, nil
	}
	diffs := make([]*pb.InventoryDiff, 0, len(b.changes))
	for it, e := range b.changes {
		diffs = append(diffs, &pb.InventoryDiff{
			ItemId:        int32(it.ID),
			ItemState:     int32(it.State),
			QuantityDelta: e.qty,
			Reason:        e.reason,
		})
	}
	return &pb.StateDiff{Inventory: diffs}, nil
}

func (b *Bucket) IsEmpty() bool { return len(b.changes) == 0 }
