package session

import (
	"testing"

	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
)

func TestIsProgressOnlyDiff(t *testing.T) {
	tests := []struct {
		name string
		diff *pb.StateDiff
		want bool
	}{
		{
			name: "nil diff",
			diff: nil,
			want: true,
		},
		{
			name: "empty diff",
			diff: &pb.StateDiff{},
			want: true,
		},
		{
			name: "progress-only current scope",
			diff: &pb.StateDiff{
				EventQueue: []*pb.EventQueueDiff{
					{QueueId: 0, Scope: "current", Entries: []*pb.EventQueueEntry{{Progress: 0.5}}},
				},
			},
			want: true,
		},
		{
			name: "full scope queue change (consume/enqueue/reorder)",
			diff: &pb.StateDiff{
				EventQueue: []*pb.EventQueueDiff{
					{QueueId: 0, Scope: "full", Entries: []*pb.EventQueueEntry{{EventId: 1}}},
				},
			},
			want: false,
		},
		{
			name: "mixed current and full scope",
			diff: &pb.StateDiff{
				EventQueue: []*pb.EventQueueDiff{
					{QueueId: 0, Scope: "current", Entries: []*pb.EventQueueEntry{{Progress: 0.5}}},
					{QueueId: 1, Scope: "full", Entries: []*pb.EventQueueEntry{{EventId: 2}}},
				},
			},
			want: false,
		},
		{
			name: "current scope with execution is not progress-only",
			diff: &pb.StateDiff{
				EventQueue: []*pb.EventQueueDiff{
					{QueueId: 0, Scope: "current", Entries: []*pb.EventQueueEntry{{Progress: 0.5}}},
				},
				EventExecution: []*pb.EventExecutionDiff{{EventId: 1, Cycles: 1}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isProgressOnlyDiff(tt.diff)
			if got != tt.want {
				t.Errorf("isProgressOnlyDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}
