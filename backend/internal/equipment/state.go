// Package equipment manages player equipment slots: which item.Item is
// mounted in which slot.
package equipment

import (
	"context"
	"fmt"

	dbgen "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db/gen"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
)

// State holds the in-memory equipment slots for one player.
type State struct {
	userID  int64
	slots   map[string]item.Item
	dirty   map[string]bool
	deleted map[string]bool

	recorder *record.Recorder
}

// NewState creates a bare in-memory State with no slots. Used in tests
// and as the initial value before CreateSession overwrites it.
func NewState(userID int64) *State {
	return &State{
		userID:  userID,
		slots:   make(map[string]item.Item),
		dirty:   make(map[string]bool),
		deleted: make(map[string]bool),
	}
}

// Load reads all equipment rows for the given user into a new State.
// Does not write records —used on connect/reconnect.
func Load(ctx context.Context, q *dbgen.Queries, userID int64) (*State, error) {
	rows, err := q.LoadEquipment(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("equipment: load: %w", err)
	}

	slots := make(map[string]item.Item, len(rows))
	for _, r := range rows {
		slots[r.Slot] = item.Item{ID: item.ID(r.ItemID), State: item.State(r.ItemState)}
	}

	return &State{
		userID:  userID,
		slots:   slots,
		dirty:   make(map[string]bool),
		deleted: make(map[string]bool),
	}, nil
}

// Equip places it in slot. If the slot was occupied the old item is
// silently replaced; the caller is responsible for higher-level
// coordination (e.g. inventory return).
func (s *State) Equip(slot string, it item.Item) {
	s.slots[slot] = it
	delete(s.deleted, slot)
	s.dirty[slot] = true
	s.record(slot, it, pb.EquipAction_EQUIP_ACTION_EQUIP)
}

// Unequip removes the item from slot. No-op if the slot is empty.
func (s *State) Unequip(slot string) {
	it, ok := s.slots[slot]
	if !ok {
		return
	}
	delete(s.slots, slot)
	delete(s.dirty, slot)
	s.deleted[slot] = true
	s.record(slot, it, pb.EquipAction_EQUIP_ACTION_UNEQUIP)
}

// Get returns the item in slot and whether the slot is occupied.
func (s *State) Get(slot string) (item.Item, bool) {
	it, ok := s.slots[slot]
	return it, ok
}

// All returns a copy of slot -> item.
func (s *State) All() map[string]item.Item {
	out := make(map[string]item.Item, len(s.slots))
	for k, v := range s.slots {
		out[k] = v
	}
	return out
}

// Flush UPSERTs every dirty slot and DELETEs every deleted slot.
func (s *State) Flush(ctx context.Context, q *dbgen.Queries) error {
	for slot := range s.dirty {
		it := s.slots[slot]
		if err := q.UpsertEquipment(ctx, dbgen.UpsertEquipmentParams{
			UserID:    s.userID,
			Slot:      slot,
			ItemID:    int64(it.ID),
			ItemState: int64(it.State),
		}); err != nil {
			return fmt.Errorf("equipment: upsert %q: %w", slot, err)
		}
	}
	for slot := range s.deleted {
		if err := q.DeleteEquipment(ctx, dbgen.DeleteEquipmentParams{
			UserID: s.userID,
			Slot:   slot,
		}); err != nil {
			return fmt.Errorf("equipment: delete %q: %w", slot, err)
		}
	}
	s.dirty = make(map[string]bool)
	s.deleted = make(map[string]bool)
	return nil
}

func (s *State) record(slot string, it item.Item, action pb.EquipAction) {
	if s.recorder == nil {
		return
	}
	b := s.recorder.Bucket("equipment")
	if b != nil {
		b.(*Bucket).addAction(slot, it, action)
	}
}

// SetRecorder attaches a Recorder for the current execution cycle.
func (s *State) SetRecorder(rec *record.Recorder) { s.recorder = rec }

// ClearRecorder detaches the current Recorder.
func (s *State) ClearRecorder() { s.recorder = nil }
