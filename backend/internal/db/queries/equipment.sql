-- name: LoadEquipment :many
SELECT slot, item_id, item_state, anchor_slot FROM player_equipment WHERE user_id = ?;

-- name: UpsertEquipment :exec
INSERT INTO player_equipment (user_id, slot, item_id, item_state, anchor_slot)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (user_id, slot) DO UPDATE SET item_id = excluded.item_id, item_state = excluded.item_state, anchor_slot = excluded.anchor_slot;

-- name: DeleteEquipment :exec
DELETE FROM player_equipment WHERE user_id = ? AND slot = ?;

-- name: DeleteEquipmentByAnchor :exec
DELETE FROM player_equipment WHERE user_id = ? AND anchor_slot = ?;
