# Equipment System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Equip/Unequip system — move items from inventory to slots, apply/remove attribute modifiers with semantic diff reasons.

**Architecture:** Add `equipped map[string]item.Item` to PlayerSession, DB-backed via `player_equipment` table in existing 00001 migration. Proto `InventoryDiff.reason` enum distinguishes EVENT/EQUIP/UNEQUIP. WS handlers `inventory.equip`/`inventory.unequip` registered via session.Manager.

**Tech Stack:** Go 1.26, SQLite, protobuf, sqlc

---

### Task 1: Proto — add reason enum, equip messages, StateFull.equipment

**Files:**
- Modify: `proto/state.proto`
- Generate: `internal/pb/state.pb.go`
- Create: `proto/equipment.proto`
- Generate: `internal/pb/equipment.pb.go`

- [ ] **Step 1: Add reason enum and equipment messages**

Add to `proto/state.proto` (before `InventoryDiff`):
```proto
enum InventoryChangeReason {
  INVENTORY_CHANGE_UNSPECIFIED = 0;
  EVENT = 1;
  EQUIP = 2;
  UNEQUIP = 3;
}
```

Change `InventoryDiff`:
```proto
message InventoryDiff {
  int32 item_id = 1;
  int32 item_state = 2;
  double quantity_delta = 3;
  InventoryChangeReason reason = 4;
}
```

Append to `StateFull`:
```proto
message StateFull {
  repeated InventoryFull inventory = 1;
  repeated AttributeFull attribute = 2;
  repeated SkillXPFull skill_xp = 3;
  repeated BestiaryFull bestiary = 4;
  repeated EventExecutionFull event_execution = 5;
  map<string, ItemIdentity> equipment = 6;
}
```

Add after `EventQueueDiff`:
```proto
message ItemIdentity {
  int32 item_id = 1;
  int32 item_state = 2;
}
```

Create `proto/equipment.proto`:
```proto
syntax = "proto3";
package mli.v1;
option go_package = "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/pb";

import "proto/state.proto";

message EquipReq {
  int32 item_id = 1;
  int32 item_state = 2;
  string slot = 3;
}

message UnequipReq {
  string slot = 1;
}

message EquipUnequipResponse {
  map<string, ItemIdentity> equipped = 1;
}
```

- [ ] **Step 2: Generate proto code**

Run: `cd backend && buf generate`
Expected: no errors, `state.pb.go` and `equipment.pb.go` updated/created.

- [ ] **Step 3: Verify build**

Run: `go build ./internal/pb/...`
Expected: no errors.

---

### Task 2: DB — add player_equipment to migration and queries

**Files:**
- Modify: `internal/db/migrations/00001_init_users.sql`
- Create: `internal/db/queries/equipment.sql`
- Generate: `internal/db/gen/equipment.sql.go`

- [ ] **Step 1: Add table to migration**

Append to `00001_init_users.sql`:
```sql
CREATE TABLE IF NOT EXISTS player_equipment (
    user_id    INTEGER NOT NULL,
    slot       TEXT    NOT NULL,
    item_id    INTEGER NOT NULL,
    item_state INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, slot)
);
```

- [ ] **Step 2: Add SQL queries**

Create `internal/db/queries/equipment.sql`:
```sql
-- name: LoadEquipment :many
SELECT slot, item_id, item_state FROM player_equipment WHERE user_id = ?;

-- name: UpsertEquipment :exec
INSERT INTO player_equipment (user_id, slot, item_id, item_state)
VALUES (?, ?, ?, ?)
ON CONFLICT (user_id, slot) DO UPDATE SET item_id = excluded.item_id, item_state = excluded.item_state;

-- name: DeleteEquipment :exec
DELETE FROM player_equipment WHERE user_id = ? AND slot = ?;
```

- [ ] **Step 3: Regenerate sqlc**

Run: `go generate ./internal/db/...` (or `sqlc generate` depending on setup)
Expected: `internal/db/gen/equipment.sql.go` created.

- [ ] **Step 4: Verify build**

Run: `go build ./internal/db/...`
Expected: no errors.

---

### Task 3: Inventory bucket — add reason parameter

**Files:**
- Modify: `internal/inventory/bucket.go`
- Modify: `internal/inventory/state.go`
- Modify: `internal/event/state.go`
- Modify: `internal/event/settle.go`
- Modify: `internal/event/event_test.go`

- [ ] **Step 1: Add reason field to Bucket**

In `bucket.go`, change `add` to accept reason and store it:
```go
type changeEntry struct {
	qty    float64
	reason pb.InventoryChangeReason
}

type Bucket struct {
	changes map[item.Item]*changeEntry
}
```

Update `add`:
```go
func (b *Bucket) add(it item.Item, qty float64, reason pb.InventoryChangeReason) {
	if e, ok := b.changes[it]; ok {
		e.qty += qty
	} else {
		b.changes[it] = &changeEntry{qty: qty, reason: reason}
	}
}
```

Update `MergeInPlace`:
```go
func (b *Bucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*Bucket)
	for it, e := range ob.changes {
		if existing, ok := b.changes[it]; ok {
			existing.qty += e.qty
		} else {
			b.changes[it] = &changeEntry{qty: e.qty, reason: e.reason}
		}
	}
}
```

Update `SerializeDiff` to include reason:
```go
func (b *Bucket) SerializeDiff() (proto.Message, error) {
	if len(b.changes) == 0 {
		return nil, nil
	}
	diffs := make([]*pb.InventoryDiff, 0, len(b.changes))
	for it, e := range b.changes {
		diffs = append(diffs, &pb.InventoryDiff{
			ItemId:        int32(it.ID),
			ItemState:     int32(it.State),
			QuantityDelta: e.qty,
			Reason:        e.reason,
		})
	}
	return &pb.StateDiff{Inventory: diffs}, nil
}
```

- [ ] **Step 2: Update State.Add and State.Deduct**

In `state.go`, add a private `addWithReason`:
```go
func (s *State) add(it item.Item, qty float64, reason pb.InventoryChangeReason) {
	s.slots[it] += qty
	if s.slots[it] == 0 {
		delete(s.slots, it)
	}
	s.dirty[it] = true
	if s.recorder != nil {
		b := s.recorder.Bucket("inventory")
		if b != nil {
			b.(*Bucket).add(it, qty, reason)
		}
	}
}
```

Change `Add` to default to EVENT:
```go
func (s *State) Add(it item.Item, qty float64) {
	s.add(it, qty, pb.InventoryChangeReason_EVENT)
}
```

Change `Deduct` to default to EVENT:
```go
func (s *State) Deduct(it item.Item, qty float64) {
	s.add(it, -qty, pb.InventoryChangeReason_EVENT)
}
```

Add internal setter for equip/unequip use:
```go
// addEquipChange records inventory change from equip (removal) or unequip (return).
func (s *State) addEquipChange(it item.Item, qty float64, equipped bool) {
	reason := pb.InventoryChangeReason_EQUIP
	if !equipped {
		reason = pb.InventoryChangeReason_UNEQUIP
	}
	s.add(it, qty, reason)
}
```

- [ ] **Step 3: Update all callers of bucket.add**

In `event/state.go` - the `SettlementCtx.AddItem`/`DeductItem` impl on PlayerSession calls `inv.Add`/`inv.Deduct` (already uses EVENT by default).
In `event/settle.go` - calls `ctx.AddItem`/`ctx.DeductItem` which goes through PlayerSession → inv.Add/Deduct.
All current callers get EVENT reason automatically.

- [ ] **Step 4: Verify build and tests**

Run: `go build ./... && go test ./internal/inventory/... ./internal/event/... ./internal/session/...`
Expected: all pass.

---

### Task 4: PlayerSession — Equip/Unequip/Equipped methods

**Files:**
- Modify: `internal/session/session.go`
- Modify: `internal/session/session_test.go`

- [ ] **Step 1: Add equipped field and setters**

Add to `PlayerSession` struct:
```go
equipped map[string]item.Item // slot → item
```

Add getters/setters:
```go
func (s *PlayerSession) Equipped(slot string) (item.Item, bool) {
	it, ok := s.equipped[slot]
	return it, ok
}
```

- [ ] **Step 2: Implement Equip**

```go
func (s *PlayerSession) Equip(ctx context.Context, it item.Item, slot string) error {
	def, ok := gameconfig.GetItemDefByID(it.ID)
	if !ok {
		return apperror.NotFound("item not found")
	}
	if !def.IsEquipment() {
		return apperror.BadRequest("item is not equipment")
	}

	// Validate slot.
	if !validSlot(def, slot) {
		return apperror.BadRequest("item cannot be equipped in slot " + slot)
	}

	// Check inventory.
	if !s.inv.Has(it, 1) {
		return apperror.BadRequest("item not in inventory")
	}

	// Unequip old item in same slot.
	if old, ok := s.equipped[slot]; ok {
		s.unequipInternal(ctx, slot, old)
	}

	// Deduct from inventory with EQUIP reason.
	s.inv.addEquipChange(it, -1, true)

	// Add modifiers.
	mods, err := def.Modifiers(it, attribute.Get())
	if err != nil {
		return err
	}
	s.attr.AddModifiers("equipment:"+def.StringID(), mods)

	// Cache equipped.
	s.equipped[slot] = it
	return nil
}

func validSlot(def item.ItemDef, slot string) bool {
	// Equipment items have position requirements from their details.
	// For now, accept any slot; slot validation will use the
	// equipment_position_requirements from actions.json when exposed via ItemDef.
	return true
}
```

- [ ] **Step 3: Implement Unequip**

```go
func (s *PlayerSession) Unequip(ctx context.Context, slot string) error {
	it, ok := s.equipped[slot]
	if !ok {
		return apperror.NotFound("no item equipped in slot")
	}
	return s.unequipInternal(ctx, slot, it)
}

func (s *PlayerSession) unequipInternal(ctx context.Context, slot string, it item.Item) error {
	def, _ := gameconfig.GetItemDefByID(it.ID)

	// Remove modifiers.
	if def != nil {
		s.attr.RemoveModifiers("equipment:" + def.StringID())
	}

	// Return to inventory with UNEQUIP reason.
	s.inv.addEquipChange(it, 1, false)

	// Remove from equipped.
	delete(s.equipped, slot)
	return nil
}
```

- [ ] **Step 4: Implement loadEquipment for reconnect**

```go
func (s *PlayerSession) loadEquipment(ctx context.Context, q *dbgen.Queries) error {
	rows, err := q.LoadEquipment(ctx, s.UserID)
	if err != nil {
		return fmt.Errorf("load equipment: %w", err)
	}
	for _, r := range rows {
		it := item.Item{ID: item.ID(r.ItemID), State: item.State(r.ItemState)}
		def, ok := gameconfig.GetItemDefByID(it.ID)
		if !ok {
			continue
		}
		mods, err := def.Modifiers(it, attribute.Get())
		if err != nil {
			return err
		}
		s.attr.AddModifiers("equipment:"+def.StringID(), mods)
		s.equipped[r.Slot] = it
	}
	return nil
}
```

- [ ] **Step 5: Wire loadEquipment into CreateSession**

In `CreateSession`, after loading all subsystems, add:
```go
if err := sess.loadEquipment(ctx, database.Queries); err != nil {
	return nil, err
}
```

- [ ] **Step 6: Flush equipment to DB**

Add to `FlushAll` (or as a separate method called at appropriate times):
```go
// FlushEquipment writes equipped items to DB. Call after Equip/Unequip.
func (s *PlayerSession) FlushEquipment(ctx context.Context, q *dbgen.Queries) error {
	for slot, it := range s.equipped {
		if err := q.UpsertEquipment(ctx, dbgen.UpsertEquipmentParams{
			UserID:    s.UserID,
			Slot:      slot,
			ItemID:    int64(it.ID),
			ItemState: int64(it.State),
		}); err != nil {
			return err
		}
	}
	return nil
}
```

Actually — since `FlushAll` runs in a transaction each tick, integrate equipment upsert/delete there. Add to `FlushAll`:
```go
// Upsert equipment.
for slot, it := range s.equipped {
	if err := q.UpsertEquipment(ctx, dbgen.UpsertEquipmentParams{
		UserID: s.UserID, Slot: slot,
		ItemID: int64(it.ID), ItemState: int64(it.State),
	}); err != nil {
		return err
	}
}
```

And on Unequip, the delete needs to happen too. Since we can't easily know which slots were deleted in the current tick structure, change `FlushAll` to also handle the full equipment sync:
For now, just upsert in FlushAll. Deletion of equipped slots from DB is handled by `unequipInternal` directly:
- After `delete(s.equipped, slot)`, call `q.DeleteEquipment(ctx, userID, slot)` inline.

Actually simpler: in `unequipInternal`, don't call DB directly. Instead, track deleted slots in a `deletedSlots []string` field, and FlushAll handles them. Let's keep it simple for Phase 1 and do inline DB delete in unequipInternal when a DB handle is available.

Wait — `unequipInternal` doesn't have DB access. And `FlushAll` does. Let's add `deletedSlots` field:
```go
deletedSlots []string
```

In `unequipInternal`:
```go
s.deletedSlots = append(s.deletedSlots, slot)
```

In `FlushAll`:
```go
for _, slot := range s.deletedSlots {
	if err := q.DeleteEquipment(ctx, dbgen.DeleteEquipmentParams{
		UserID: s.UserID, Slot: slot,
	}); err != nil {
		return err
	}
}
s.deletedSlots = nil
```

- [ ] **Step 7: Verify build and tests**

Run: `go build ./... && go test ./internal/session/...`
Expected: all pass.

---

### Task 5: WS handlers — register inventory.equip and inventory.unequip

**Files:**
- Create: `internal/inventory/ws.go`
- Modify: `internal/session/session.go` (pick up from Task 4)

- [ ] **Step 1: Create WS handler registration**

Create `internal/inventory/ws.go`:
```go
package inventory

import (
	"context"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/session"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/wsx"
	"google.golang.org/protobuf/proto"
)

func RegisterWS(hub *wsx.Hub, mgr *session.Manager, database *db.DB) {
	session.HandleSessionTyped(hub, mgr, "inventory.equip", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.EquipReq) error {
		it := item.Item{ID: item.ID(req.ItemId), State: item.State(req.ItemState)}
		if err := sess.Equip(ctx, it, req.Slot); err != nil {
			return err
		}
		if err := sess.FlushEquipment(); err != nil {
			return err
		}
		c.Reply(wsx.Inbound{ID: ""}, buildEquipResponse(sess))
		return nil
	})

	session.HandleSessionTyped(hub, mgr, "inventory.unequip", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.UnequipReq) error {
		if err := sess.Unequip(ctx, req.Slot); err != nil {
			return err
		}
		if err := sess.FlushEquipment(); err != nil {
			return err
		}
		c.Reply(wsx.Inbound{ID: ""}, buildEquipResponse(sess))
		return nil
	})
}

func buildEquipResponse(sess *session.PlayerSession) proto.Message {
	resp := &pb.EquipUnequipResponse{
		Equipped: make(map[string]*pb.ItemIdentity),
	}
	// Need a way to enumerate equipped slots — add a method or iterate known slots.
	return resp
}
```

**Note:** This is a skeleton — the Reply signature needs the inbound request to extract ID for request/response correlation. Revise in next task.

Actually, looking at the Reply method: `func (c *Conn) Reply(in Inbound, payload proto.Message) bool`. It needs the inbound message. The typed handlers don't expose `in` directly. Let's handle this in Task 6 (cleanup/wiring).

For now, use `wsx.Conn.Send` with a manually constructed Outbound:
```go
c.Send(wsx.Outbound{Type: "inventory.equip.ok", Payload: resp})
```

- [ ] **Step 2: Add AllEquipped method to PlayerSession**

In `session/session.go`:
```go
func (s *PlayerSession) AllEquipped() map[string]item.Item {
	out := make(map[string]item.Item, len(s.equipped))
	for k, v := range s.equipped {
		out[k] = v
	}
	return out
}
```

- [ ] **Step 3: Wire into main.go**

In `cmd/server/main.go`, after session manager creation:
```go
inventory.RegisterWS(hub, sessMgr)
```

Wait — `inventory` package would need to import `session` which imports `inventory` — circular import. Break circle: register handlers in a higher package (like `cmd/server`) or create a separate `handlers` package.

Simplest approach: register in `cmd/server/main.go` directly:
```go
session.HandleSessionTyped(sessMgr, hub, "inventory.equip", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.EquipReq) error {
	it := item.Item{ID: item.ID(req.ItemId), State: item.State(req.ItemState)}
	if err := sess.Equip(ctx, it, req.Slot); err != nil {
		return err
	}
	// DB flush handled by game loop FlushAll
	c.Send(wsx.Outbound{Type: "inventory.equip.ok", Payload: buildEquipResponse(sess)})
	return nil
})
```

This avoids the circular import entirely.

- [ ] **Step 4: Verify build**

Run: `go build ./cmd/server/...`
Expected: no errors.

---

### Task 6: Equip and Unequip flow with FlushAll integration

Goal: Make Equip/Unequip flush equipment changes via the existing FlushAll mechanism.

- [ ] **Step 1: Integrate equipment flush into FlushAll**

In `session/session.go` `FlushAll`:
```go
// Upsert equipment.
for slot, it := range s.equipped {
	if err := q.UpsertEquipment(ctx, dbgen.UpsertEquipmentParams{
		UserID: s.UserID, Slot: slot,
		ItemID: int64(it.ID), ItemState: int64(it.State),
	}); err != nil {
		return err
	}
}
// Delete unequipped slots since last flush.
for _, slot := range s.deletedSlots {
	if err := q.DeleteEquipment(ctx, dbgen.DeleteEquipmentParams{
		UserID: s.UserID, Slot: slot,
	}); err != nil {
		return err
	}
}
s.deletedSlots = s.deletedSlots[:0]
```

- [ ] **Step 2: Remove inline DB calls from Equip/Unequip**

Equip and Unequip should NOT call DB directly. They just modify in-memory state (inv, attr, equipped, deletedSlots). The game loop's FlushAll handles persistence.

- [ ] **Step 3: Verify build and run full test suite**

Run: `go build ./... && go test ./...`
Expected: all pass.

---

### Task 7: Tests for equipment system

**Files:**
- Create: `internal/session/equipment_test.go` (in `session_test` package)

- [ ] **Step 1: Test basic Equip**

```go
func TestEquip(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	// Give the session inventory and an item.
	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	invSt.Add(item.Item{ID: 1, State: 0}, 1) // wooden_sword
	s.SetInv(invSt)

	err := s.Equip(context.Background(), item.Item{ID: 1, State: 0}, "main_hand")
	if err != nil {
		t.Fatal(err)
	}

	// Verify equipped.
	got, ok := s.Equipped("main_hand")
	if !ok {
		t.Fatal("equipped slot should be set")
	}
	if got.ID != 1 {
		t.Errorf("want item 1, got %v", got.ID)
	}

	// Verify inventory deducted.
	if s.Inv().Get(item.Item{ID: 1, State: 0}) != 0 {
		t.Error("inventory should have 0 after equip")
	}

	// Verify modifiers applied.
	physID, _ := attribute.Get().AttrID("physical_power")
	val := s.Attr().GetFinal(physID)
	def, _ := attribute.Get().Def(physID)
	if val <= def.DefaultValue {
		t.Error("attribute modifier should increase physical_power")
	}
}
```

- [ ] **Step 2: Test Equip + Unequip roundtrip**

```go
func TestEquipUnequip(t *testing.T) {
	mgr := newTestManager(t)
	s, cleanup := newLockedSession(t, mgr, 1)
	defer cleanup()

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	invSt.Add(item.Item{ID: 1, State: 0}, 1)
	s.SetInv(invSt)

	// Equip.
	s.Equip(context.Background(), item.Item{ID: 1, State: 0}, "main_hand")

	// Unequip.
	err := s.Unequip(context.Background(), "main_hand")
	if err != nil {
		t.Fatal(err)
	}

	// Slot should be empty.
	_, ok := s.Equipped("main_hand")
	if ok {
		t.Error("slot should be empty after unequip")
	}

	// Inventory should be restored.
	if s.Inv().Get(item.Item{ID: 1, State: 0}) != 1 {
		t.Error("inventory should have item back after unequip")
	}
}
```

- [ ] **Step 3: Test inventory diff reason**

```go
func TestEquipInventoryDiffReason(t *testing.T) {
	reg := record.NewRegistry()
	reg.Register(inventory.Provider)
	reg.Register(attribute.Provider)

	_, q := openInvDB(t)
	invSt, _ := inventory.Load(context.Background(), q, 1)
	invSt.Add(item.Item{ID: 1, State: 0}, 1)

	mgr := session.NewManager(reg, nil)
	s := session.New(uuid.New(), 1, testLogger())
	s.SetInv(invSt)
	mgr.Add(s)

	locked, _ := mgr.LockSession(s.ID)
	defer mgr.UnlockSession(locked)

	rec := mgr.NewRecorder()
	locked.SetRecorder(rec)
	rec.PushNamespace("action")

	locked.Equip(context.Background(), item.Item{ID: 1, State: 0}, "main_hand")

	rec.PopNamespace()
	locked.ClearRecorder()

	diff, _ := reg.BuildDiff(rec)
	if len(diff.Inventory) != 1 {
		t.Fatalf("want 1 inventory change, got %d", len(diff.Inventory))
	}
	if diff.Inventory[0].Reason != pb.InventoryChangeReason_EQUIP {
		t.Errorf("want reason EQUIP, got %v", diff.Inventory[0].Reason)
	}
}
```

- [ ] **Step 4: Run test suite**

Run: `go test ./internal/session/... -v -run="TestEquip"`
Expected: all equipment tests pass.

---

### Task 8: Cleanup — proto enum naming, unused imports

- [ ] **Step 1: Ensure proto enum prefix consistency**

protobuf generates `pb.InventoryChangeReason_EQUIP` (with SCREAMING_SNAKE_CASE prefix). Verify all code uses generated names.

- [ ] **Step 2: Run full test suite**

Run: `go build ./... && go test ./...`
Expected: all 9 packages pass.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add equipment system with Equip/Unequip, InventoryChangeReason enum"
```
