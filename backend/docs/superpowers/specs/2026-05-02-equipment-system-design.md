# Equipment System Design

## Context

Implement the Equip/Unequip mechanism per design doc 2.3. Equipping an item moves it from inventory, places it in a slot, and activates its attribute modifiers. Unequipping returns it to inventory.

## Design Decisions

- **Equipment consumes inventory**: Equip deducts 1 from inventory; Unequip returns it.
- **Semantic inventory diffs**: `InventoryDiff.reason` enum (EVENT/EQUIP/UNEQUIP) so frontend can render different animations.
- **DB table in 00001 migration**: Single migration file, add `player_equipment` table.
- **Reconnect**: Load equipment from DB, replay modifiers directly (no records generated).
- **Slot validation**: Check item's `equipment_position_requirements` against requested slot.
- **One item per slot**: Equipping to an occupied slot auto-unequips the old item first.

## API

```go
func (s *PlayerSession) Equip(ctx, it, slot) error
func (s *PlayerSession) Unequip(ctx, slot) error
func (s *PlayerSession) Equipped(slot) (item.Item, bool)
```

## Proto

- `InventoryChangeReason` enum (EVENT=1, EQUIP=2, UNEQUIP=3)
- `InventoryDiff.reason` field
- `EquipReq`, `UnequipReq`, `EquipUnequipResponse`, `ItemIdentity`
- `StateFull.equipment` map field

## DB

```sql
CREATE TABLE player_equipment (
    user_id INTEGER NOT NULL, slot TEXT NOT NULL,
    item_id INTEGER NOT NULL, item_state INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, slot)
);
```

## Flow

**Equip**: validate slot → unequip old → inv.Deduct(1) → attr.AddModifiers → DB upsert → record(reason=EQUIP)
**Unequip**: attr.RemoveModifiers → inv.Add(1) → DB delete → record(reason=UNEQUIP)
**Reconnect**: DB load → foreach row: inv.AddModifiers directly, no records
