package event

import (
	"encoding/json"
	"sort"

	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// ExecProvider is the SystemProvider for event_execution records.
var ExecProvider record.SystemProvider = &execProvider{}

type execProvider struct{}

func (p *execProvider) SystemName() string            { return "event_execution" }
func (p *execProvider) NewBucket() record.RecordBucket { return newExecBucket() }

func (p *execProvider) SerializeFull(state any) (json.RawMessage, error) {
	st, ok := state.(*State)
	if !ok {
		return json.RawMessage("null"), nil
	}

	type entry struct {
		QueueID      int     `json:"queue_id"`
		EventID      int64   `json:"event_id"`
		TargetCycles int     `json:"target_cycles"`
		Progress     float64 `json:"progress"`
	}

	var out []entry
	for _, q := range st.queues {
		for _, e := range q.Entries {
			out = append(out, entry{
				QueueID:      q.ID,
				EventID:      int64(e.EventID),
				TargetCycles: e.TargetCycles,
				Progress:     e.Progress,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].QueueID != out[j].QueueID {
			return out[i].QueueID < out[j].QueueID
		}
		return out[i].EventID < out[j].EventID
	})

	return json.Marshal(out)
}

// QueueProvider is the SystemProvider for event_queue changes (queue structure).
var QueueProvider record.SystemProvider = &queueProvider{}

type queueProvider struct{}

func (p *queueProvider) SystemName() string            { return "event_queue" }
func (p *queueProvider) NewBucket() record.RecordBucket { return newQueueBucket() }

func (p *queueProvider) SerializeFull(state any) (json.RawMessage, error) {
	// Full snapshot for queues is handled by the event_execution provider
	// which serializes all entries. event_queue only produces diff data.
	return json.RawMessage("null"), nil
}
