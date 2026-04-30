package event

import (
	"encoding/json"

	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// --- event_execution ---

type execBucket struct {
	cycles map[gameconfig.EventID]int
}

var _ record.RecordBucket = (*execBucket)(nil)

func newExecBucket() *execBucket { return &execBucket{cycles: make(map[gameconfig.EventID]int)} }

func (b *execBucket) addExecution(id gameconfig.EventID, cycles int) {
	b.cycles[id] += cycles
}

func (b *execBucket) SystemName() string { return "event_execution" }

func (b *execBucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*execBucket)
	for id, c := range ob.cycles {
		b.cycles[id] += c
	}
}

type execWire struct {
	EventID int64 `json:"event_id"`
	Cycles  int   `json:"cycles"`
}

func (b *execBucket) SerializeDiff() (json.RawMessage, error) {
	if len(b.cycles) == 0 {
		return json.RawMessage("[]"), nil
	}
	out := make([]execWire, 0, len(b.cycles))
	for id, c := range b.cycles {
		out = append(out, execWire{EventID: int64(id), Cycles: c})
	}
	return json.Marshal(out)
}

func (b *execBucket) IsEmpty() bool { return len(b.cycles) == 0 }

// --- event_queue ---

type queueBucket struct {
	marks map[int]bool // queueID → full=true
}

var _ record.RecordBucket = (*queueBucket)(nil)

func newQueueBucket() *queueBucket { return &queueBucket{marks: make(map[int]bool)} }

func (b *queueBucket) markQueue(id int, full bool) {
	if existing, ok := b.marks[id]; ok {
		b.marks[id] = existing || full
	} else {
		b.marks[id] = full
	}
}

func (b *queueBucket) SystemName() string { return "event_queue" }

func (b *queueBucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*queueBucket)
	for id, full := range ob.marks {
		if existing, ok := b.marks[id]; ok {
			b.marks[id] = existing || full
		} else {
			b.marks[id] = full
		}
	}
}

type queueWire struct {
	QueueID int    `json:"queue_id"`
	Scope   string `json:"scope"` // "current" or "full"
}

func (b *queueBucket) SerializeDiff() (json.RawMessage, error) {
	if len(b.marks) == 0 {
		return json.RawMessage("[]"), nil
	}
	out := make([]queueWire, 0, len(b.marks))
	for id, full := range b.marks {
		scope := "current"
		if full {
			scope = "full"
		}
		out = append(out, queueWire{QueueID: id, Scope: scope})
	}
	return json.Marshal(out)
}

func (b *queueBucket) IsEmpty() bool { return len(b.marks) == 0 }
