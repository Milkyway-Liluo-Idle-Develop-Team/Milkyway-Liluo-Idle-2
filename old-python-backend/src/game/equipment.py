"""装备/工具穿脱逻辑（支持多槽位占用）。"""

from collections import defaultdict
from typing import Literal

from models import database, PlayerEquipment, PlayerTool
from game.inventory import InsufficientItem
from game.attributes import get_items_map, compute_scaled_item_data
from game.context import PlayerContext
from game.settlement import settle_player, SettlementResult

SlotType = Literal["tool", "equipment"]

TOOL_SLOT_ORDER: tuple[str, ...] = (
    "felling",
    "mining",
    "planting",
    "crafting",
    "forging",
    "enhancing",
)

EQUIPMENT_SLOT_ORDER: tuple[str, ...] = (
    "main_hand",
    "side_hand",
    "head",
    "chest",
    "leg",
    "feet",
    "necklace",
    "treasure",
)

TOOL_SLOT_NAME_MAP: dict[str, str] = {
    "felling": "砍伐",
    "mining": "采矿",
    "planting": "种植",
    "crafting": "制造",
    "forging": "锻造",
    "enhancing": "赋能",
}

EQUIPMENT_SLOT_NAME_MAP: dict[str, str] = {
    "main_hand": "主手",
    "side_hand": "副手",
    "head": "头部",
    "chest": "胸部",
    "leg": "腿部",
    "feet": "足部",
    "necklace": "项链",
    "treasure": "珍宝",
}


def _slot_base(slot_id: str) -> str:
    return slot_id.split("#", 1)[0]


def _slot_label(slot_type: SlotType, slot_id: str) -> str:
    base = _slot_base(slot_id)
    if slot_type == "tool":
        return TOOL_SLOT_NAME_MAP.get(base, base)
    return EQUIPMENT_SLOT_NAME_MAP.get(base, base)


def _ordered_bases(slot_type: SlotType) -> tuple[str, ...]:
    if slot_type == "tool":
        return TOOL_SLOT_ORDER
    return EQUIPMENT_SLOT_ORDER


def build_slot_instances(ctx: PlayerContext, slot_type: SlotType) -> list[str]:
    out: list[str] = []
    for base in _ordered_bases(slot_type):
        raw_count = ctx.attr_set.get(f"{base}_slot_count", 1.0)
        try:
            count = max(1, int(raw_count))
        except (TypeError, ValueError):
            count = 1
        if count <= 1:
            out.append(base)
            continue
        for idx in range(1, count + 1):
            out.append(f"{base}#{idx}")
    return out


def _resolve_slot_type_by_slot(slot_id: str) -> SlotType:
    base = _slot_base(slot_id)
    if base in TOOL_SLOT_ORDER:
        return "tool"
    if base in EQUIPMENT_SLOT_ORDER:
        return "equipment"
    raise ValueError(f"Unknown slot: {slot_id}")


def _requirements_from_item(item_data: dict, slot_type: SlotType) -> dict[str, int]:
    req_map: dict[str, int] = defaultdict(int)
    if slot_type == "tool":
        reqs = (
            item_data
            .get("tool_details", {})
            .get("tool_position_requirement", [])
        )
        for req in reqs:
            base = str(req.get("tool_position") or "")
            if not base:
                continue
            try:
                count = max(1, int(req.get("value", 1)))
            except (TypeError, ValueError):
                count = 1
            req_map[base] += count
    else:
        reqs = (
            item_data
            .get("equipment_details", {})
            .get("equipment_position_requirements", [])
        )
        for req in reqs:
            base = str(req.get("position") or "")
            if not base or base == "nothing":
                continue
            try:
                count = max(1, int(req.get("value", 1)))
            except (TypeError, ValueError):
                count = 1
            req_map[base] += count
    return dict(req_map)


def _rows_dict_by_type(ctx: PlayerContext, slot_type: SlotType):
    return ctx.tools if slot_type == "tool" else ctx.equipment


def _model_by_type(slot_type: SlotType):
    return PlayerTool if slot_type == "tool" else PlayerEquipment


def _piece_anchor(row, fallback_slot: str) -> str:
    return str(getattr(row, "anchor_slot", None) or fallback_slot)


def _piece_enhance_level(row) -> int:
    try:
        return max(0, int(getattr(row, "enhance_level", 0) or 0))
    except (TypeError, ValueError):
        return 0


def _piece_enhance_fail_count(row) -> int:
    try:
        return max(0, int(getattr(row, "enhance_fail_count", 0) or 0))
    except (TypeError, ValueError):
        return 0


def _return_to_inventory(ctx: PlayerContext, item_id: str, delta: int = 1) -> None:
    if delta <= 0:
        return
    ctx.inventory.add(item_id, delta)



def _remove_anchor_piece(ctx: PlayerContext, slot_type: SlotType, anchor_slot: str) -> str | None:
    rows = _rows_dict_by_type(ctx, slot_type)
    to_remove: list[tuple[str, object]] = []
    for slot_id, row in rows.items():
        if _piece_anchor(row, slot_id) == anchor_slot:
            to_remove.append((slot_id, row))

    if not to_remove:
        return None

    removed_item_id = str(to_remove[0][1].item_id)
    for slot_id, row in to_remove:
        ctx.session.delete(row)
        del rows[slot_id]
        ctx.mark_dirty(f"{slot_type}:{slot_id}")

    _return_to_inventory(ctx, removed_item_id, 1)
    return removed_item_id


def set_piece_enhance_level(
    ctx: PlayerContext,
    slot_type: SlotType,
    anchor_slot: str,
    enhance_level: int,
) -> int:
    """设置一件多槽位装备/工具的强化等级（按 anchor_slot 同步）。"""
    return set_piece_enhance_state(
        ctx=ctx,
        slot_type=slot_type,
        anchor_slot=anchor_slot,
        enhance_level=enhance_level,
        enhance_fail_count=None,
    )


def set_piece_enhance_state(
    ctx: PlayerContext,
    slot_type: SlotType,
    anchor_slot: str,
    enhance_level: int | None = None,
    enhance_fail_count: int | None = None,
) -> int:
    """设置一件多槽位装备/工具的强化状态（按 anchor_slot 同步）。"""
    rows = _rows_dict_by_type(ctx, slot_type)
    safe_level = None if enhance_level is None else max(0, int(enhance_level))
    safe_fail = None if enhance_fail_count is None else max(0, int(enhance_fail_count))
    changed = 0
    for slot_id, row in rows.items():
        if _piece_anchor(row, slot_id) != anchor_slot:
            continue
        needs_update = False
        if safe_level is not None and _piece_enhance_level(row) != safe_level:
            row.enhance_level = safe_level
            needs_update = True
        if safe_fail is not None and _piece_enhance_fail_count(row) != safe_fail:
            row.enhance_fail_count = safe_fail
            needs_update = True
        if not needs_update:
            continue
        ctx.mark_dirty(f"{slot_type}:{slot_id}")
        changed += 1
    return changed


def _choose_target_slots(
    all_slots: list[str],
    requirements: dict[str, int],
    clicked_slot: str,
    occupied_by_other: set[str],
) -> list[str]:
    clicked_base = _slot_base(clicked_slot)
    if clicked_base not in requirements:
        raise ValueError("该装备不能放入当前槽位")

    slots_by_base: dict[str, list[str]] = defaultdict(list)
    for slot_id in all_slots:
        slots_by_base[_slot_base(slot_id)].append(slot_id)

    selected: list[str] = []
    for base, need in requirements.items():
        candidates = slots_by_base.get(base, [])
        if not candidates:
            raise ValueError(f"缺少槽位: {base}")

        chosen: list[str] = []
        if base == clicked_base:
            if clicked_slot in occupied_by_other:
                raise ValueError("当前槽位被占用")
            chosen.append(clicked_slot)
            for slot_id in candidates:
                if len(chosen) >= need:
                    break
                if slot_id == clicked_slot or slot_id in occupied_by_other:
                    continue
                chosen.append(slot_id)
        else:
            for slot_id in candidates:
                if len(chosen) >= need:
                    break
                if slot_id in occupied_by_other:
                    continue
                chosen.append(slot_id)

        if len(chosen) < need:
            raise ValueError("没有足够空闲槽位")
        selected.extend(chosen)

    return selected


@database.player_atomic
def equip_item(uid: int, item_id: str, slot: str, slot_type: SlotType | None = None) -> dict:
    result = settle_player(uid)
    ctx = result.ctx
    session = database.get_db()

    if slot_type is None:
        slot_type = _resolve_slot_type_by_slot(slot)

    items_map = get_items_map()
    item_data = items_map.get(item_id)
    if not item_data:
        raise ValueError(f"Unknown item: {item_id}")

    if slot_type == "tool" and not item_data.get("tool", False):
        raise ValueError(f"Item {item_id} is not a tool")
    if slot_type == "equipment" and not item_data.get("equipment", False):
        raise ValueError(f"Item {item_id} is not an equipment")


    all_slots = build_slot_instances(ctx, slot_type)
    if slot not in all_slots:
        raise ValueError(f"Invalid slot: {slot}")

    req_map = _requirements_from_item(item_data, slot_type)
    if not req_map:
        req_map = {_slot_base(slot): 1}

    rows = _rows_dict_by_type(ctx, slot_type)

    # 若当前槽位已有装备，先按主槽位整体卸下（回背包）
    clicked_row = rows.get(slot)
    if clicked_row is not None:
        _remove_anchor_piece(ctx, slot_type, _piece_anchor(clicked_row, slot))

    occupied_by_other = set(rows.keys())
    target_slots = _choose_target_slots(all_slots, req_map, slot, occupied_by_other)

    ctx.inventory.consume(item_id, 1)

    model_cls = _model_by_type(slot_type)
    for slot_id in target_slots:
        row = model_cls(
            uid=uid,
            slot=slot_id,
            item_id=item_id,
            anchor_slot=slot,
            enhance_level=0,
            enhance_fail_count=0,
        )
        session.add(row)
        rows[slot_id] = row
        ctx.mark_dirty(f"{slot_type}:{slot_id}")

    ctx.save()
    return SettlementResult.build_state_response(ctx, elapsed=0.0, log=[])


@database.player_atomic
def unequip_item(uid: int, slot: str, slot_type: SlotType | None = None) -> dict:
    result = settle_player(uid)
    ctx = result.ctx

    if slot_type is None:
        slot_type = _resolve_slot_type_by_slot(slot)

    rows = _rows_dict_by_type(ctx, slot_type)
    row = rows.get(slot)
    if row is None:
        raise ValueError(f"No equipped item in slot {slot}")

    anchor = _piece_anchor(row, slot)
    removed_item = _remove_anchor_piece(ctx, slot_type, anchor)
    if removed_item is None:
        raise ValueError("No equipped item to unequip")

    ctx.save()
    return SettlementResult.build_state_response(ctx, elapsed=0.0, log=[])


def build_equipment_view(ctx: PlayerContext, item_map: dict[str, dict]) -> dict:
    # TODO: 移动到前端判断，这是前端的职责
    def _build_slot_cells(slot_type: SlotType) -> list[dict]:
        rows = _rows_dict_by_type(ctx, slot_type)
        slots = build_slot_instances(ctx, slot_type)
        cells: list[dict] = []
        for slot_id in slots:
            row = rows.get(slot_id)
            item_id = row.item_id if row else None
            anchor = _piece_anchor(row, slot_id) if row else None
            enhance_level = _piece_enhance_level(row) if row else 0
            enhance_fail_count = _piece_enhance_fail_count(row) if row else 0
            attr_preview: list[dict[str, float | str]] = []
            if row and item_id:
                item_data = item_map.get(item_id) or {}
                scaled_data = compute_scaled_item_data(item_data, slot_type, enhance_level)
                attr_preview = [
                    {"key": attr, "value": float(val)}
                    for attr, val in sorted(scaled_data.items(), key=lambda kv: kv[0])
                ]
            cells.append(
                {
                    "slot_type": slot_type,
                    "slot_id": slot_id,
                    "slot_name": _slot_label(slot_type, slot_id),
                    "item_id": item_id,
                    "item_name": (item_map.get(item_id or "") or {}).get("name") if item_id else None,
                    "anchor_slot": anchor,
                    "is_disabled": bool(row and anchor and anchor != slot_id),
                    "enhance_level": enhance_level if row else None,
                    "enhance_fail_count": enhance_fail_count if row else None,
                    "attribute_preview": attr_preview,
                }
            )
        return cells

    return {
        "production_slots": _build_slot_cells("tool"),
        "battle_slots": _build_slot_cells("equipment"),
        "equipable_items": _build_equipable_items(ctx, item_map),
    }


def _build_equipable_items(ctx: PlayerContext, item_map: dict[str, dict]) -> list[dict]:
    out: list[dict] = []
    for (item_id, _state), row in ctx.inventory.items():
        if row.quantity <= 0:
            continue
        item_data = item_map.get(item_id) or {}

        if item_data.get("tool"):
            req_map = _requirements_from_item(item_data, "tool")
            out.append(
                {
                    "id": item_id,
                    "name": item_data.get("name") or item_id,
                    "quantity": ctx.inventory.quantity_of(item_id),
                    "slot_type": "tool",
                    "required_slots": sorted(req_map.keys()),
                }
            )

        if item_data.get("equipment"):
            req_map = _requirements_from_item(item_data, "equipment")
            out.append(
                {
                    "id": item_id,
                    "name": item_data.get("name") or item_id,
                    "quantity": ctx.inventory.quantity_of(item_id),
                    "slot_type": "equipment",
                    "required_slots": sorted(req_map.keys()),
                }
            )

    return sorted(out, key=lambda x: (x["slot_type"], x["name"]))
