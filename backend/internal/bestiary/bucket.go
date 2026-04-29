package bestiary

import (
	"encoding/json"

	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
)

type unlockEntry struct {
	typ string
	id  string
}

func event(id gameconfig.EventID) unlockEntry {
	return unlockEntry{typ: "event", id: id.String()}
}

func itemUnlock(it item.Item) unlockEntry {
	// Use numeric encoding: "item_id/item_state"
	return unlockEntry{typ: "item", id: it.String()}
}

func area(id gameconfig.MapID) unlockEntry {
	return unlockEntry{typ: "area", id: id.String()}
}

// Bucket collects BestiaryUnlockRecords within a single namespace.
// Duplicate (type, id) pairs are deduplicated.
type Bucket struct {
	entries map[string]unlockEntry // key = typ+"/"+id
}

var _ record.RecordBucket = (*Bucket)(nil)

func newBucket() *Bucket {
	return &Bucket{entries: make(map[string]unlockEntry)}
}

func (b *Bucket) add(e unlockEntry) {
	key := e.typ + "/" + e.id
	b.entries[key] = e
}

func (b *Bucket) SystemName() string { return "bestiary" }

func (b *Bucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*Bucket)
	for k, e := range ob.entries {
		b.entries[k] = e
	}
}

type unlockWire struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func (b *Bucket) SerializeDiff() (json.RawMessage, error) {
	if len(b.entries) == 0 {
		return json.RawMessage("[]"), nil
	}
	out := make([]unlockWire, 0, len(b.entries))
	for _, e := range b.entries {
		out = append(out, unlockWire{Type: e.typ, ID: e.id})
	}
	return json.Marshal(out)
}

func (b *Bucket) IsEmpty() bool { return len(b.entries) == 0 }
