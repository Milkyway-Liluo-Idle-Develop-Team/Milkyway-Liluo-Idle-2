// Package inventory manages player item storage: quantity tracking,
// dirty-based DB persistence, and record integration for diff packets.
package inventory

import (
	"context"
	"fmt"

	"github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/item"
	pb "github.com/edrowsluo/new-mli/backend/pb"
	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// State holds the in-memory inventory for one player. Quantities are
// float64 (fractional parts survive across settlement cycles).
// The player-facing count is floor(quantity).
type State struct {
	userID int64
	slots  map[item.Item]float64
	dirty  map[item.Item]bool

	recorder *record.Recorder
}

// Load reads all inventory rows for the given user into a new State.
func Load(ctx context.Context, q *dbgen.Queries, userID int64) (*State, error) {
	rows, err := q.LoadInventory(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("inventory: load: %w", err)
	}

	slots := make(map[item.Item]float64, len(rows))
	for _, r := range rows {
		it := item.Item{ID: item.ID(r.ItemID), State: item.State(r.ItemState)}
		slots[it] = r.Quantity
	}

	return &State{
		userID: userID,
		slots:  slots,
		dirty:  make(map[item.Item]bool),
	}, nil
}

// Flush writes every dirty slot to the database using UPSERT.
// Executes inside a single transaction.
func (s *State) Flush(ctx context.Context, q *dbgen.Queries) error {
	if len(s.dirty) == 0 {
		return nil
	}

	for it := range s.dirty {
		qty := s.slots[it]
		err := q.UpsertInventory(ctx, dbgen.UpsertInventoryParams{
			UserID:    s.userID,
			ItemID:    int64(it.ID),
			ItemState: int64(it.State),
			Quantity:  qty,
		})
		if err != nil {
			return fmt.Errorf("inventory: upsert %v: %w", it, err)
		}
	}

	s.dirty = make(map[item.Item]bool)
	return nil
}

// Add increases the quantity of the given item identity.
// Writes an InventoryChangeRecord with EVENT reason to the current recorder (if set).
func (s *State) Add(it item.Item, qty float64) {
	s.add(it, qty, pb.InventoryChangeReason_EVENT)
}

// Deduct decreases quantity. Callers should check Has first.
func (s *State) Deduct(it item.Item, qty float64) {
	s.add(it, -qty, pb.InventoryChangeReason_EVENT)
}

// AddEquipChange records inventory change from equip (removal) or unequip (return).
func (s *State) AddEquipChange(it item.Item, qty float64, equipped bool) {
	reason := pb.InventoryChangeReason_EQUIP
	if !equipped {
		reason = pb.InventoryChangeReason_UNEQUIP
	}
	s.add(it, qty, reason)
}

func (s *State) add(it item.Item, qty float64, reason pb.InventoryChangeReason) {
	s.slots[it] += qty
	if s.slots[it] == 0 {
		delete(s.slots, it)
	}
	s.dirty[it] = true
	s.record(it, qty, reason)
}

// Get returns the current quantity (including unflushed changes).
func (s *State) Get(it item.Item) float64 {
	return s.slots[it]
}

// Display returns the player-visible count.
func (s *State) Display(it item.Item) int {
	return int(s.slots[it])
}

// Has reports whether at least qty items are present.
func (s *State) Has(it item.Item, qty float64) bool {
	return s.slots[it] >= qty
}

// All returns all non-zero inventory slots for snapshot serialization.
func (s *State) All() map[item.Item]float64 {
	out := make(map[item.Item]float64, len(s.slots))
	for it, q := range s.slots {
		if q > 0 {
			out[it] = q
		}
	}
	return out
}

func (s *State) record(it item.Item, qty float64, reason pb.InventoryChangeReason) {
	if s.recorder == nil {
		return
	}
	b := s.recorder.Bucket("inventory")
	if b != nil {
		b.(*Bucket).add(it, qty, reason)
	}
}

// SetRecorder attaches a Recorder for the current execution cycle.
func (s *State) SetRecorder(rec *record.Recorder) {
	s.recorder = rec
}

// ClearRecorder detaches the current Recorder.
func (s *State) ClearRecorder() {
	s.recorder = nil
}
