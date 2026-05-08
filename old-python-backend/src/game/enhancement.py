"""装备/工具强化逻辑。"""

from __future__ import annotations

import math
import random
import time
from typing import Literal

from game.attributes import compute_scaled_item_data, get_items_map
from game.context import PlayerContext
from game.equipment import set_piece_enhance_state
from game.inventory import InsufficientItem
from game.settlement import (
    apply_experience,
    check_requirements,
    check_unlock_requirements,
    get_events_map,
    refresh_unlocked_events,
    settle_player,
)
from models import database
from services import gameplay_service

SlotType = Literal["tool", "equipment"]

ENHANCE_CAST_SECONDS = 5.0


from game.core_utils import clamp as _clamp, to_float as _to_float


def _to_int(value, default: int = 0) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def _normalize_slot_type(value: str) -> SlotType:
    text = str(value or "").strip().lower()
    if text not in ("tool", "equipment"):
        raise ValueError("slot_type must be 'tool' or 'equipment'")
    return text  # type: ignore[return-value]


def _find_piece_row(ctx: PlayerContext, slot_type: SlotType, anchor_slot: str):
    rows = ctx.tools if slot_type == "tool" else ctx.equipment
    for slot_id, row in rows.items():
        anchor = str(getattr(row, "anchor_slot", None) or slot_id)
        if anchor == anchor_slot:
            return slot_id, row
    return None, None


def _piece_state(ctx: PlayerContext, slot_type: SlotType, anchor_slot: str) -> tuple[str, int, int]:
    slot_id, row = _find_piece_row(ctx, slot_type, anchor_slot)
    if row is None:
        raise ValueError("指定槽位没有可强化装备")
    item_id = str(getattr(row, "item_id", "") or "")
    if not item_id:
        raise ValueError("指定槽位没有可强化装备")
    level = max(0, _to_int(getattr(row, "enhance_level", 0), 0))
    fail_count = max(0, _to_int(getattr(row, "enhance_fail_count", 0), 0))
    _ = slot_id
    return item_id, level, fail_count


def _curve_nodes(item_data: dict) -> list[dict]:
    details = item_data.get("upgrade_details") or {}
    rows = details.get("upgrade_curve") or []
    nodes: list[dict] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        nodes.append({"level": _to_int(row.get("level"), 0), "row": row})
    nodes.sort(key=lambda x: x["level"])
    if not nodes:
        nodes.append({"level": 0, "row": {"level": 0, "recommend_level": 1, "basic_success_rate": 1.0}})
    return nodes


def _interp_numeric(nodes: list[dict], level: int, field: str, default: float) -> float:
    lv = max(0, int(level))
    if lv <= nodes[0]["level"]:
        return _to_float(nodes[0]["row"].get(field), default)
    if lv >= nodes[-1]["level"]:
        return _to_float(nodes[-1]["row"].get(field), default)

    for idx in range(1, len(nodes)):
        left = nodes[idx - 1]
        right = nodes[idx]
        if lv > right["level"]:
            continue
        l_level = int(left["level"])
        r_level = int(right["level"])
        l_value = _to_float(left["row"].get(field), default)
        r_value = _to_float(right["row"].get(field), l_value)
        if r_level <= l_level:
            return r_value
        ratio = (lv - l_level) / (r_level - l_level)
        return l_value + (r_value - l_value) * ratio
    return _to_float(nodes[-1]["row"].get(field), default)


def _requirements_to_map(reqs: list[dict] | None) -> dict[str, float]:
    out: dict[str, float] = {}
    for req in reqs or []:
        if not isinstance(req, dict):
            continue
        item_id = str(req.get("id") or "").strip()
        if not item_id:
            continue
        value = req.get("num")
        if value is None:
            value = req.get("value", 0)
        amount = max(0.0, _to_float(value, 0.0))
        out[item_id] = amount
    return out


def _interp_requirements(nodes: list[dict], level: int) -> list[dict]:
    lv = max(0, int(level))
    if len(nodes) <= 1:
        req_map = _requirements_to_map(nodes[0]["row"].get("requirements"))
        return [{"item_id": item_id, "value": int(math.floor(amount))} for item_id, amount in sorted(req_map.items()) if amount > 0]

    if lv <= nodes[0]["level"]:
        req_map = _requirements_to_map(nodes[0]["row"].get("requirements"))
        return [{"item_id": item_id, "value": int(math.floor(amount))} for item_id, amount in sorted(req_map.items()) if amount > 0]

    if lv >= nodes[-1]["level"]:
        req_map = _requirements_to_map(nodes[-1]["row"].get("requirements"))
        return [{"item_id": item_id, "value": int(math.floor(amount))} for item_id, amount in sorted(req_map.items()) if amount > 0]

    left = nodes[0]
    right = nodes[-1]
    for idx in range(1, len(nodes)):
        cand_left = nodes[idx - 1]
        cand_right = nodes[idx]
        if lv > cand_right["level"]:
            continue
        left = cand_left
        right = cand_right
        break

    l_level = int(left["level"])
    r_level = int(right["level"])
    ratio = 0.0 if r_level <= l_level else (lv - l_level) / (r_level - l_level)
    l_map = _requirements_to_map(left["row"].get("requirements"))
    r_map = _requirements_to_map(right["row"].get("requirements"))
    all_ids = sorted(set(l_map) | set(r_map))
    out: list[dict] = []
    for item_id in all_ids:
        l_val = l_map.get(item_id, 0.0)
        r_val = r_map.get(item_id, 0.0)
        value = l_val + (r_val - l_val) * ratio
        floored = int(math.floor(max(0.0, value)))
        if floored <= 0:
            continue
        out.append({"item_id": item_id, "value": floored})
    return out


def _enhancing_level(ctx: PlayerContext) -> int:
    skill = ctx.skills.get("enhancing")
    base_level = int(getattr(skill, "level", 1) or 1)
    buff = _to_float(ctx.attr_set.get("enhancing_level_buff", 0.0), 0.0)
    return max(1, int(math.floor(base_level + buff)))


def _final_success_rate(ctx: PlayerContext, basic_success_rate: float, recommend_level: float) -> float:
    enhancing_level = _enhancing_level(ctx)
    recommend = max(1.0, float(recommend_level))
    basic = _clamp(float(basic_success_rate), 0.0, 1.0)

    if enhancing_level < recommend:
        adjusted = basic * (0.99 ** (recommend - enhancing_level))
    else:
        adjusted = basic * (1.0 + (((enhancing_level - recommend) / 35.0) ** 2))

    success_mult = 1.0 + _to_float(ctx.attr_set.get("enhancing_success_rate_multiplier", 0.0), 0.0)
    return _clamp(adjusted * success_mult, 0.0, 1.0)


def _poisson_conditional_success_rate(probability: float, fail_count: int) -> float:
    """P(X=k | X>=k), X~Poisson(lambda=1/p)."""
    p = _clamp(float(probability), 0.0, 1.0)
    if p <= 0.0:
        return 0.0
    if p >= 1.0:
        return 1.0

    lam = 1.0 / p
    k = max(0, int(fail_count))

    if lam <= 0:
        return 1.0

    log_pmf_k = -lam + k * math.log(lam) - math.lgamma(k + 1.0)
    if log_pmf_k < -740:
        return p

    pmf = math.exp(log_pmf_k)
    if pmf <= 0.0:
        return p

    tail = pmf
    term = pmf
    n = k
    for _ in range(10000):
        n += 1
        term *= lam / n
        tail += term
        if term <= tail * 1e-12:
            break

    if tail <= 0.0:
        return p
    return _clamp(pmf / tail, 0.0, 1.0)


def _compute_enhance_rates(
    ctx: PlayerContext,
    nodes: list[dict],
    level: int,
    fail_count: int,
) -> dict[str, float]:
    """计算强化相关的各类成功率。"""
    recommend_level = _interp_numeric(nodes, level, "recommend_level", 1.0)
    basic_success_rate = _clamp(_interp_numeric(nodes, level, "basic_success_rate", 1.0), 0.0, 1.0)
    display_success_rate = _final_success_rate(ctx, basic_success_rate, recommend_level)
    real_success_rate = _poisson_conditional_success_rate(display_success_rate, fail_count)
    return {
        "recommend_level": float(recommend_level),
        "basic_success_rate": float(basic_success_rate),
        "display_success_rate": float(display_success_rate),
        "real_success_rate": float(real_success_rate),
    }


def _compute_enhance_requirements(
    ctx: PlayerContext,
    nodes: list[dict],
    level: int,
    item_id: str,
    items_map: dict[str, dict],
) -> tuple[list[dict], bool]:
    """计算强化材料需求并检查库存是否充足。"""
    reqs = _interp_requirements(nodes, level)
    req_map: dict[str, int] = {entry["item_id"]: int(entry["value"]) for entry in reqs}
    req_map[item_id] = req_map.get(item_id, 0) + 1

    req_list: list[dict] = []
    missing_items: list[str] = []
    for req_item_id in sorted(req_map.keys()):
        needed = max(0, int(req_map[req_item_id]))
        if needed <= 0:
            continue
        owned = ctx.inventory.quantity_of(req_item_id)
        lacking = max(0, needed - owned)
        if lacking > 0:
            missing_items.append(req_item_id)
        req_list.append(
            {
                "item_id": req_item_id,
                "item_name": (items_map.get(req_item_id) or {}).get("name") or req_item_id,
                "needed": needed,
                "owned": owned,
                "lacking": lacking,
                "is_protection": req_item_id == item_id,
            }
        )

    return req_list, not missing_items


def _build_attr_preview(
    item_data: dict,
    slot_type: SlotType,
    level: int,
    next_level: int,
) -> tuple[list[dict], list[dict], list[dict]]:
    """构建当前/下一级属性预览和差分对比。

    返回: (current_attrs_list, next_attrs_list, diff_preview)
    """
    current_attrs = compute_scaled_item_data(item_data, slot_type, level)
    next_attrs = compute_scaled_item_data(item_data, slot_type, next_level)
    attr_keys = sorted(set(current_attrs.keys()) | set(next_attrs.keys()))
    attr_preview = [
        {
            "key": key,
            "current": float(current_attrs.get(key, 0.0)),
            "next": float(next_attrs.get(key, 0.0)),
        }
        for key in attr_keys
        if abs(float(current_attrs.get(key, 0.0))) > 1e-12 or abs(float(next_attrs.get(key, 0.0))) > 1e-12
    ]
    current_list = [
        {"key": key, "value": float(value)}
        for key, value in sorted(current_attrs.items(), key=lambda kv: kv[0])
    ]
    next_list = [
        {"key": key, "value": float(value)}
        for key, value in sorted(next_attrs.items(), key=lambda kv: kv[0])
    ]
    return current_list, next_list, attr_preview


def _build_preview(ctx: PlayerContext, slot_type: SlotType, anchor_slot: str, items_map: dict[str, dict]) -> dict:
    item_id, enhance_level, fail_count = _piece_state(ctx, slot_type, anchor_slot)
    item_data = items_map.get(item_id)
    if not item_data:
        raise ValueError(f"Unknown item: {item_id}")
    if not item_data.get("upgradable", False):
        raise ValueError("该装备不可强化")

    details = item_data.get("upgrade_details") or {}
    max_upgrade = max(0, _to_int(details.get("max_upgrade"), 0))
    if max_upgrade <= 0:
        raise ValueError("该装备不可强化")

    at_max = enhance_level >= max_upgrade
    nodes = _curve_nodes(item_data)

    rates = _compute_enhance_rates(ctx, nodes, enhance_level, fail_count)
    req_list, has_enough = _compute_enhance_requirements(ctx, nodes, enhance_level, item_id, items_map)
    current_attrs, next_attrs, attr_preview = _build_attr_preview(
        item_data, slot_type, enhance_level, min(max_upgrade, enhance_level + 1)
    )

    return {
        "slot_type": slot_type,
        "anchor_slot": anchor_slot,
        "item_id": item_id,
        "item_name": item_data.get("name") or item_id,
        "current_level": enhance_level,
        "current_fail_count": fail_count,
        "max_upgrade": max_upgrade,
        "at_max_level": at_max,
        "recommend_level": rates["recommend_level"],
        "basic_success_rate": rates["basic_success_rate"],
        "display_success_rate": rates["display_success_rate"],
        "real_success_rate": rates["real_success_rate"],
        "enhancing_level": _enhancing_level(ctx),
        "requirements": req_list,
        "has_enough_requirements": has_enough,
        "current_attributes": current_attrs,
        "next_attributes": next_attrs,
        "attribute_preview": attr_preview,
        "cast_seconds": ENHANCE_CAST_SECONDS,
        "exp_on_execute": float(50.0 * (1.005 ** rates["recommend_level"])),
    }


def _can_use_enhancement(ctx: PlayerContext, *, require_executable: bool) -> bool:
    events_map = get_events_map()
    for event in events_map.values():
        if event.get("type") != "loop":
            continue
        if event.get("need_skill") != "enhancing":
            continue
        if require_executable:
            if check_requirements(event.get("requirements"), ctx):
                return True
        elif check_unlock_requirements(event.get("requirements"), ctx):
            return True
    return False


@database.player_atomic
def get_enhance_preview(uid: int, slot_type: str, anchor_slot: str) -> dict:
    result = settle_player(uid)
    ctx = result.ctx

    safe_slot_type = _normalize_slot_type(slot_type)
    safe_anchor_slot = str(anchor_slot or "").strip()
    if not safe_anchor_slot:
        raise ValueError("anchor_slot required")

    if not _can_use_enhancement(ctx, require_executable=False):
        raise ValueError("当前不可进行赋能行动")
    items_map = get_items_map()
    return _build_preview(ctx, safe_slot_type, safe_anchor_slot, items_map)


@database.player_atomic
def execute_enhancement(uid: int, slot_type: str, anchor_slot: str) -> dict:
    result = settle_player(uid)
    ctx = result.ctx

    safe_slot_type = _normalize_slot_type(slot_type)
    safe_anchor_slot = str(anchor_slot or "").strip()
    if not safe_anchor_slot:
        raise ValueError("anchor_slot required")

    if not _can_use_enhancement(ctx, require_executable=True):
        raise ValueError("当前不可进行赋能行动")
    items_map = get_items_map()

    preview = _build_preview(ctx, safe_slot_type, safe_anchor_slot, items_map)
    if preview["at_max_level"]:
        raise ValueError("已达最大强化等级")

    requirements = preview.get("requirements") or []
    for req in requirements:
        lacking = int(req.get("lacking", 0) or 0)
        if lacking > 0:
            raise ValueError("强化材料不足")

    for req in requirements:
        ctx.inventory.consume(str(req.get("item_id") or ""), int(req.get("needed", 0) or 0))

    current_level = int(preview["current_level"])
    current_fail_count = int(preview["current_fail_count"])
    roll_success_rate = _clamp(float(preview["real_success_rate"]), 0.0, 1.0)
    success = random.random() < roll_success_rate

    if success:
        set_piece_enhance_state(
            ctx=ctx,
            slot_type=safe_slot_type,
            anchor_slot=safe_anchor_slot,
            enhance_level=current_level + 1,
            enhance_fail_count=0,
        )
    else:
        set_piece_enhance_state(
            ctx=ctx,
            slot_type=safe_slot_type,
            anchor_slot=safe_anchor_slot,
            enhance_fail_count=current_fail_count + 1,
        )

    exp_gain = 50.0 * (1.005 ** float(preview["recommend_level"]))
    apply_experience(exp_gain, "enhancing", ctx)
    refresh_unlocked_events(ctx)

    ctx.save()

    patch_payload = gameplay_service.build_patch_payload(uid)
    ctx_after = PlayerContext.load(ctx.session, uid)
    refreshed = _build_preview(ctx_after, safe_slot_type, safe_anchor_slot, items_map)

    return {
        "slot_type": safe_slot_type,
        "anchor_slot": safe_anchor_slot,
        "success": success,
        "cast_seconds": ENHANCE_CAST_SECONDS,
        "rolled_success_rate": roll_success_rate,
        "display_success_rate": float(preview["display_success_rate"]),
        "exp_gain": float(exp_gain),
        "preview": refreshed,
        "patch": (patch_payload.get("patch") if isinstance(patch_payload, dict) else None) or {},
    }
