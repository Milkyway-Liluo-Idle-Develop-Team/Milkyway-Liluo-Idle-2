package bestiary

import (
	pb "github.com/edrowsluo/new-mli/backend/pb"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

type unlockEntry struct {
	typ string
	id  string
}

func event(id gameconfig.EventID) unlockEntry {
	return unlockEntry{typ: "event", id: id.String()}
}

func itemUnlock(it item.Item) unlockEntry {
	if def, ok := gameconfig.GetItemDefByID(it.ID); ok {
		return unlockEntry{typ: "item", id: def.StringID()}
	}
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

func (b *Bucket) SerializeDiff() (proto.Message, error) {
	if len(b.entries) == 0 {
		return nil, nil
	}
	diffs := make([]*pb.BestiaryDiff, 0, len(b.entries))
	for _, e := range b.entries {
		diffs = append(diffs, &pb.BestiaryDiff{Type: e.typ, Id: e.id})
	}
	return &pb.StateDiff{Bestiary: diffs}, nil
}

func (b *Bucket) IsEmpty() bool { return len(b.entries) == 0 }
