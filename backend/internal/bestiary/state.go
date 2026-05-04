// Package bestiary tracks which events, items, and areas the player has
// discovered. Events and items are persisted to DB; areas are inferred.
package bestiary

import (
	"context"
	"fmt"

	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// State holds the player's discovered content.
type State struct {
	userID int64
	events map[gameconfig.EventID]bool
	items  map[item.Item]bool
	areas  map[gameconfig.MapID]bool

	// dirtyEvents tracks events newly unlocked this cycle, for Flush.
	dirtyEvents map[gameconfig.EventID]bool
	// dirtyItems tracks items newly discovered this cycle, for Flush.
	dirtyItems map[item.Item]bool

	recorder *record.Recorder
}

// New creates an empty State.
func New(userID int64) *State {
	return &State{
		userID:      userID,
		events:      make(map[gameconfig.EventID]bool),
		items:       make(map[item.Item]bool),
		areas:       make(map[gameconfig.MapID]bool),
		dirtyEvents: make(map[gameconfig.EventID]bool),
		dirtyItems:  make(map[item.Item]bool),
	}
}

// LoadEvents bulk-adds already-unlocked events without writing records.
// Called on connect to rebuild state from player_unlocked_events.
func (s *State) LoadEvents(ids []gameconfig.EventID) {
	for _, id := range ids {
		s.events[id] = true
	}
}

// LoadItems bulk-adds already-known items without writing records.
// Called on connect to rebuild state from player_inventory.
func (s *State) LoadItems(items []item.Item) {
	for _, it := range items {
		s.items[it] = true
	}
}

// LoadDiscoveredItems bulk-adds already-discovered items from the database.
func (s *State) LoadDiscoveredItems(rows []dbgen.PlayerDiscoveredItem) {
	for _, r := range rows {
		it := item.Item{ID: item.ID(r.ItemID), State: 0}
		s.items[it] = true
	}
}

// LoadAreas bulk-adds already-visited areas without writing records.
func (s *State) LoadAreas(ids []gameconfig.MapID) {
	for _, id := range ids {
		s.areas[id] = true
	}
}

// UnlockEvent records discovery of an event during play. Idempotent.
func (s *State) UnlockEvent(id gameconfig.EventID) {
	if s.events[id] { return }
	s.events[id] = true
	s.dirtyEvents[id] = true
	s.record(event(id))
}

// Flush writes newly unlocked events and discovered items to the database.
func (s *State) Flush(ctx context.Context, q *dbgen.Queries) error {
	for id := range s.dirtyEvents {
		if err := q.UpsertUnlockedEvent(ctx, dbgen.UpsertUnlockedEventParams{
			UserID:  s.userID,
			EventID: int64(id),
		}); err != nil {
			return fmt.Errorf("bestiary: upsert event %d: %w", id, err)
		}
	}
	s.dirtyEvents = make(map[gameconfig.EventID]bool)

	for it := range s.dirtyItems {
		if err := q.UpsertDiscoveredItem(ctx, dbgen.UpsertDiscoveredItemParams{
			UserID: s.userID,
			ItemID: int64(it.ID),
		}); err != nil {
			return fmt.Errorf("bestiary: upsert item %v: %w", it, err)
		}
	}
	s.dirtyItems = make(map[item.Item]bool)
	return nil
}

// UnlockItem records discovery of an item identity during play. Idempotent.
func (s *State) UnlockItem(it item.Item) {
	if s.items[it] { return }
	s.items[it] = true
	s.dirtyItems[it] = true
	s.record(itemUnlock(it))
}

// UnlockArea records discovery of an area during play. Idempotent.
func (s *State) UnlockArea(id gameconfig.MapID) {
	if s.areas[id] { return }
	s.areas[id] = true
	s.record(area(id))
}

func (s *State) record(r unlockEntry) {
	if s.recorder == nil { return }
	b := s.recorder.Bucket("bestiary")
	if b != nil {
		b.(*Bucket).add(r)
	}
}

// HasEvent reports whether the given event has been unlocked.
func (s *State) HasEvent(id gameconfig.EventID) bool { return s.events[id] }

// SetRecorder attaches a Recorder for the current execution cycle.
func (s *State) SetRecorder(rec *record.Recorder) { s.recorder = rec }

// ClearRecorder detaches the current Recorder.
func (s *State) ClearRecorder() { s.recorder = nil }
