package inventory

import (
	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

// Bucket collects InventoryChangeRecords within a single namespace.
// Records with the same item.Item identity are automatically merged.
type Bucket struct {
	changes map[item.Item]float64
}

var _ record.RecordBucket = (*Bucket)(nil)

func newBucket() *Bucket {
	return &Bucket{changes: make(map[item.Item]float64)}
}

func (b *Bucket) add(it item.Item, qty float64) {
	b.changes[it] += qty
}

func (b *Bucket) SystemName() string { return "inventory" }

func (b *Bucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*Bucket)
	for it, qty := range ob.changes {
		b.changes[it] += qty
	}
}

func (b *Bucket) SerializeDiff() (proto.Message, error) {
	if len(b.changes) == 0 {
		return nil, nil
	}
	diffs := make([]*pb.InventoryDiff, 0, len(b.changes))
	for it, qty := range b.changes {
		diffs = append(diffs, &pb.InventoryDiff{
			ItemId:        int32(it.ID),
			ItemState:     int32(it.State),
			QuantityDelta: qty,
		})
	}
	return &pb.StateDiff{Inventory: diffs}, nil
}

func (b *Bucket) IsEmpty() bool { return len(b.changes) == 0 }
