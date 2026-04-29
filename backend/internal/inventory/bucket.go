package inventory

import (
	"encoding/json"

	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
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

type invWire struct {
	ItemID    int32   `json:"item_id"`
	ItemState int32   `json:"item_state"`
	QtyDelta  float64 `json:"quantity_delta"`
}

func (b *Bucket) SerializeDiff() (json.RawMessage, error) {
	if len(b.changes) == 0 {
		return json.RawMessage("[]"), nil
	}
	out := make([]invWire, 0, len(b.changes))
	for it, qty := range b.changes {
		out = append(out, invWire{
			ItemID:    int32(it.ID),
			ItemState: int32(it.State),
			QtyDelta:  qty,
		})
	}
	return json.Marshal(out)
}

func (b *Bucket) IsEmpty() bool { return len(b.changes) == 0 }
