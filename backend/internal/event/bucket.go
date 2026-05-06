package event

import (
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	"google.golang.org/protobuf/proto"
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

func (b *execBucket) SerializeDiff() (proto.Message, error) {
	if len(b.cycles) == 0 {
		return nil, nil
	}
	diffs := make([]*pb.EventExecutionDiff, 0, len(b.cycles))
	for id, c := range b.cycles {
		diffs = append(diffs, &pb.EventExecutionDiff{
			EventId: int64(id),
			Cycles:  int32(c),
		})
	}
	return &pb.StateDiff{EventExecution: diffs}, nil
}

func (b *execBucket) IsEmpty() bool { return len(b.cycles) == 0 }

// --- event_queue ---

type queueBucket struct {
	st    *State       // set by State when marking; read in SerializeDiff
	marks map[int]bool // queueID →full=true
}

var _ record.RecordBucket = (*queueBucket)(nil)

func newQueueBucket() *queueBucket {
	return &queueBucket{marks: make(map[int]bool)}
}

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
	// Keep the last State reference set (either bucket should point to the same State).
	if ob.st != nil {
		b.st = ob.st
	}
}

func (b *queueBucket) SerializeDiff() (proto.Message, error) {
	if len(b.marks) == 0 {
		return nil, nil
	}
	diffs := make([]*pb.EventQueueDiff, 0, len(b.marks))
	for id, full := range b.marks {
		scope := "current"
		if full {
			scope = "full"
		}
		var pbEnts []*pb.EventQueueEntry
		if b.st != nil {
			if q, ok := b.st.queues[id]; ok {
				entries := q.Entries
				if !full {
					if len(entries) > 1 {
						entries = entries[:1]
					}
				}
				pbEnts = make([]*pb.EventQueueEntry, len(entries))
				for i, e := range entries {
					pbEnts[i] = &pb.EventQueueEntry{
						Position:     int32(e.Position),
						EventId:      int64(e.EventID),
						TargetCycles: int32(e.TargetCycles),
						Progress:     e.Progress,
					}
				}
			}
		}
		diffs = append(diffs, &pb.EventQueueDiff{
			QueueId: int32(id),
			Scope:   scope,
			Entries: pbEnts,
		})
	}
	return &pb.StateDiff{EventQueue: diffs}, nil
}

func (b *queueBucket) IsEmpty() bool { return len(b.marks) == 0 }
