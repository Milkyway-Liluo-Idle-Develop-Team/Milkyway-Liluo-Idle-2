package event

import (
	"context"
	"fmt"

	dbgen "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db/gen"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
)

// State holds the player's active event queues.
type State struct {
	userID int64
	queues map[int]*Queue

	dirty map[int]bool

	recorder    *record.Recorder
	beforeHooks []SettlementHook
	afterHooks  []SettlementHook
}

// Queue is a serial list of events for one queue_id.
// No gaps: consumed entries are removed and subsequent entries shift down.
type Queue struct {
	ID      int
	Entries []QueueEntry
}

// QueueEntry is one event in a queue. Position is the current execution
// order (0 = head). It is reassigned when entries before it are removed.
type QueueEntry struct {
	Position     int
	EventID      gameconfig.EventID
	TargetCycles int     // -1 = infinite
	Progress     float64
}

// firstActive returns 0 if the queue has entries, -1 if empty.
func (q *Queue) firstActive() int {
	if len(q.Entries) == 0 {
		return -1
	}
	return 0
}

// HasActive returns true if any queue has at least one event entry.
func (st *State) HasActive() bool {
	for _, q := range st.queues {
		if len(q.Entries) > 0 {
			return true
		}
	}
	return false
}

// Load reads all active events for the given user, grouped by queue.
// Rows with event_id=0 are skipped (old tombstones from tail cleanup).
func Load(ctx context.Context, q *dbgen.Queries, userID int64) (*State, error) {
	rows, err := q.LoadActiveEvents(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("event: load: %w", err)
	}

	queues := make(map[int]*Queue)
	for _, r := range rows {
		if r.EventID == 0 {
			continue
		}
		id := int(r.QueueID)
		que, ok := queues[id]
		if !ok {
			que = &Queue{ID: id}
			queues[id] = que
		}
		que.Entries = append(que.Entries, QueueEntry{
			Position:     int(r.Position),
			EventID:      gameconfig.EventID(r.EventID),
			TargetCycles: int(r.TargetCycles),
			Progress:     r.Progress,
		})
	}

	if len(queues) == 0 {
		queues[0] = &Queue{ID: 0}
	}

	return &State{
		userID: userID,
		queues: queues,
		dirty:  make(map[int]bool),
	}, nil
}

// Flush writes dirty queues: upserts current entries at their Position,
// then zeroes out DB rows from the next position onward.
func (st *State) Flush(ctx context.Context, q *dbgen.Queries) error {
	if len(st.dirty) == 0 {
		return nil
	}

	for id := range st.dirty {
		queue := st.queues[id]
		if queue == nil {
			continue
		}
		for _, entry := range queue.Entries {
			err := q.UpsertActiveEvent(ctx, dbgen.UpsertActiveEventParams{
				UserID:       st.userID,
				QueueID:      int64(id),
				EventID:      int64(entry.EventID),
				Position:     int64(entry.Position),
				TargetCycles: int64(entry.TargetCycles),
				Progress:     entry.Progress,
			})
			if err != nil {
				return fmt.Errorf("event: upsert %d/%d: %w", id, entry.Position, err)
			}
		}
		// Zero out positions beyond the queue.
		tail := 0
		if len(queue.Entries) > 0 {
			tail = queue.Entries[len(queue.Entries)-1].Position + 1
		}
		if err := q.ClearTailPositions(ctx, dbgen.ClearTailPositionsParams{
			UserID:   st.userID,
			QueueID:  int64(id),
			Position: int64(tail),
		}); err != nil {
			return fmt.Errorf("event: clear tail %d: %w", id, err)
		}
	}

	st.dirty = make(map[int]bool)
	return nil
}

// consume removes the entry at idx. Subsequent entries shift down and
// their Position is reassigned.
func (st *State) consume(q *Queue, idx int) {
	q.Entries = append(q.Entries[:idx], q.Entries[idx+1:]...)
	for i := idx; i < len(q.Entries); i++ {
		q.Entries[i].Position = i
	}
	st.markQueueFull(q.ID)
}

func (st *State) recordExecution(eventID gameconfig.EventID, cycles int) {
	if st.recorder == nil { return }
	b := st.recorder.Bucket("event_execution")
	if b != nil {
		b.(*execBucket).addExecution(eventID, cycles)
	}
}

func (st *State) markQueueCurrent(id int) {
	if st.recorder == nil { return }
	b := st.recorder.Bucket("event_queue")
	if b != nil {
		qb := b.(*queueBucket)
		qb.st = st
		qb.markQueue(id, false)
	}
}

func (st *State) markQueueFull(id int) {
	st.dirty[id] = true
	if st.recorder == nil { return }
	b := st.recorder.Bucket("event_queue")
	if b != nil {
		qb := b.(*queueBucket)
		qb.st = st
		qb.markQueue(id, true)
	}
}

// Enqueue appends an event to the end of the queue.
func (st *State) Enqueue(queueID int, eventID gameconfig.EventID, targetCycles int) {
	q, ok := st.queues[queueID]
	if !ok {
		q = &Queue{ID: queueID}
		st.queues[queueID] = q
	}
	pos := 0
	if len(q.Entries) > 0 {
		pos = q.Entries[len(q.Entries)-1].Position + 1
	}
	q.Entries = append(q.Entries, QueueEntry{
		Position:     pos,
		EventID:      eventID,
		TargetCycles: targetCycles,
	})
	st.markQueueFull(queueID)
}

// MoveEntry moves the entry at fromPos to toPos, shifting entries between.
func (st *State) MoveEntry(queueID int, fromPos, toPos int) {
	q, ok := st.queues[queueID]
	if !ok || fromPos < 0 || fromPos >= len(q.Entries) || toPos < 0 || toPos >= len(q.Entries) {
		return
	}
	if fromPos == toPos {
		return
	}
	entry := q.Entries[fromPos]
	// Remove from old position.
	q.Entries = append(q.Entries[:fromPos], q.Entries[fromPos+1:]...)
	// Insert at new position.
	if toPos == len(q.Entries) {
		q.Entries = append(q.Entries, entry)
	} else {
		q.Entries = append(q.Entries[:toPos], append([]QueueEntry{entry}, q.Entries[toPos:]...)...)
	}
	// Reassign positions.
	for i := range q.Entries {
		q.Entries[i].Position = i
	}
	st.markQueueFull(queueID)
}

// InsertEntry inserts a new event at the given position, shifting later entries down.
func (st *State) InsertEntry(queueID int, pos int, eventID gameconfig.EventID, targetCycles int) {
	q, ok := st.queues[queueID]
	if !ok {
		q = &Queue{ID: queueID}
		st.queues[queueID] = q
	}
	if pos < 0 || pos > len(q.Entries) {
		pos = len(q.Entries)
	}
	entry := QueueEntry{EventID: eventID, TargetCycles: targetCycles}
	if pos == len(q.Entries) {
		q.Entries = append(q.Entries, entry)
	} else {
		q.Entries = append(q.Entries[:pos], append([]QueueEntry{entry}, q.Entries[pos:]...)...)
	}
	for i := range q.Entries {
		q.Entries[i].Position = i
	}
	st.markQueueFull(queueID)
}

// RemoveEntry removes the entry at the given position.
func (st *State) RemoveEntry(queueID int, pos int) {
	q, ok := st.queues[queueID]
	if !ok || pos < 0 || pos >= len(q.Entries) {
		return
	}
	st.consume(q, pos)
}

// ClearQueue removes all entries from a queue.
func (st *State) ClearQueue(queueID int) {
	q, ok := st.queues[queueID]
	if !ok {
		return
	}
	if len(q.Entries) == 0 {
		return
	}
	q.Entries = q.Entries[:0]
	st.markQueueFull(queueID)
}

// SetRecorder / ClearRecorder —standard lifecycle.
func (st *State) SetRecorder(rec *record.Recorder) { st.recorder = rec }
func (st *State) ClearRecorder()                    { st.recorder = nil }
