package event

import (
	"sort"

	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

// ExecProvider is the SystemProvider for event_execution records.
var ExecProvider record.SystemProvider = &execProvider{}

type execProvider struct{}

func (p *execProvider) SystemName() string            { return "event_execution" }
func (p *execProvider) NewBucket() record.RecordBucket { return newExecBucket() }

func (p *execProvider) SerializeFull(state any) (proto.Message, error) {
	st, ok := state.(*State)
	if !ok {
		return nil, nil
	}

	var out []*pb.EventExecutionFull
	for _, q := range st.queues {
		for _, e := range q.Entries {
			out = append(out, &pb.EventExecutionFull{
				QueueId:      int32(q.ID),
				EventId:      int64(e.EventID),
				TargetCycles: int32(e.TargetCycles),
				Progress:     e.Progress,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].QueueId != out[j].QueueId {
			return out[i].QueueId < out[j].QueueId
		}
		return out[i].EventId < out[j].EventId
	})

	return &pb.StateFull{EventExecution: out}, nil
}

// QueueProvider is the SystemProvider for event_queue changes (queue structure).
var QueueProvider record.SystemProvider = &queueProvider{}

type queueProvider struct{}

func (p *queueProvider) SystemName() string            { return "event_queue" }
func (p *queueProvider) NewBucket() record.RecordBucket { return newQueueBucket() }

func (p *queueProvider) SerializeFull(state any) (proto.Message, error) {
	// Full snapshot for queues is handled by the event_execution provider
	// which serializes all entries. event_queue only produces diff data.
	return nil, nil
}
