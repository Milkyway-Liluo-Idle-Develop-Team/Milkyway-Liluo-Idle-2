import json
import math
from typing import Any

from data.data import load_actions as _load_actions
from data.level_exp_requirements import LEVEL_UP_TOTAL_EXP_REQUIREMENTS
from game.attributes import get_level_production_multiplier, compute_scaled_item_data
from game.context import PlayerContext, pop_dirty_keys, pop_newly_seen
from game.equipment import build_equipment_view
from game.settlement import (
    check_requirements,
    check_unlock_requirements,
    settle_player,
    _effective_loop_time,
    _skill_output_multiplier,
    estimate_affordable_iterations,
)
from game.actions import _skill_progress, _upgrade_max_executions
from models import database

PRODUCTION_SKILL_ORDER: tuple[str, ...] = (
    "felling",
    "mining",
    "planting",
    "crafting",
    "forging",
    "enhancing",
)

COMBAT_SKILL_ORDER: tuple[str, ...] = (
    "strength",
    "ranging",
    "resilience",
    "stamina",
    "intelligence",
    "defense",
    "magic",
)

SKILL_NAME_MAP: dict[str, str] = {
    "felling": "砍伐",
    "mining": "采矿",
    "planting": "种植",
    "crafting": "制造",
    "forging": "锻造",
    "enhancing": "赋能",
    "trading": "贸易",
    "strength": "力量",
    "ranging": "远程",
    "resilience": "坚韧",
    "stamina": "耐力",
    "intelligence": "智力",
    "defense": "防御",
    "magic": "魔法",
    "none": "通用",
}

MAP_NAME_MAP: dict[str, str] = {
    "village": "村庄",
}

CLASSIFICATION_NAME_MAP: dict[str, str] = {
    "important": "重要",
    "resources": "资源",
    "ores": "矿物",
    "tool": "工具",
    "equipment": "装备",
    "fuel": "燃料",
    "animal_materials": "动物材料",
}

CLASSIFICATION_ORDER: tuple[str, ...] = (
    "important",
    "resources",
    "ores",
    "tool",
    "equipment",
    "fuel",
    "animal_materials",
)

COMPARISON_TEXT_MAP: dict[str, str] = {
    "bigger": "大于",
    "equal": "等于",
    "smaller": "小于",
    "bigger_or_equal": "大于等于",
    "smaller_or_equal": "小于等于",
}

BATTLE_BASE_DATA: dict[str, float] = {
    "hp": 100.0,
    "mp": 100.0,
    "sp": 100.0,
    "physical_power": 20.0,
    "magic_power": 20.0,
    "attack_interval": 2.0,
    "critical": 0.0,
    "critical_rate": 2.0,
    "block": 20.0,
    "block_possibility_multiplier": 0.0,
    "block_rate": 0.0,
    "accuracy": 40.0,
    "accuracy_possibility_multiplier": 0.0,
    "evade": 20.0,
    "evade_possibility_multiplier": 0.0,
    "magic_instance": 0.33,
    "final_damage_multiplier": 0.0,
    "defense": 10.0,
    "final_damage_reduce": 0.0,
    "hatred": 100.0,
    "hp_recovery": 0.0,
    "mp_recovery": 0.0,
    "sp_recovery": 0.0,
}

BATTLE_ATTR_VIEW: tuple[tuple[str, str, bool], ...] = (
    ("hp", "生命上限", False),
    ("mp", "魔力上限", False),
    ("sp", "耐力上限", False),
    ("physical_power", "物理攻击", False),
    ("magic_power", "奥术攻击", False),
    ("attack_interval", "攻击间隔(秒)", False),
    ("critical", "暴击率", True),
    ("critical_rate", "暴击倍率", False),
    ("block", "格挡值", False),
    ("block_possibility_multiplier", "格挡概率加成", True),
    ("block_rate", "格挡减伤", True),
    ("accuracy", "精准值", False),
    ("accuracy_possibility_multiplier", "命中概率加成", True),
    ("evade", "闪避值", False),
    ("evade_possibility_multiplier", "闪避概率加成", True),
    ("magic_instance", "奥术抵抗", True),
    ("defense", "防御值", False),
    ("hatred", "仇恨值", False),
    ("final_damage_multiplier", "最终伤害加成", True),
    ("final_damage_reduce", "最终伤害减免", True),
    ("hp_recovery", "生命恢复/秒", False),
    ("mp_recovery", "魔力恢复/秒", False),
    ("sp_recovery", "耐力恢复/秒", False),
)


from game.core_utils import compare as _compare, to_float as _to_float


def _extract_required_skills(requirements: list[dict] | None) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    for req in requirements or []:
        if req.get("type") != "skill":
            continue
        skill_id = str(req.get("id") or "")
        out.append(
            {
                "skill_id": skill_id,
                "skill_name": SKILL_NAME_MAP.get(skill_id, skill_id),
                "comparison_types": req.get("comparison_types") or "bigger_or_equal",
                "comparison_text": COMPARISON_TEXT_MAP.get(
                    req.get("comparison_types") or "bigger_or_equal",
                    "大于等于",
                ),
                "value": _to_float(req.get("value"), 0.0),
            }
        )
    return out


def _extract_cost_items(
    requirements: list[dict] | None,
    item_map: dict[str, dict[str, Any]],
) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    for req in requirements or []:
        if req.get("type") != "item":
            continue
        if req.get("comparison_types") is not None:
            continue
        value = _to_float(req.get("value"), 0.0)
        if value <= 0:
            continue
        item_id = str(req.get("id") or "")
        item_name = (item_map.get(item_id) or {}).get("name") or item_id
        out.append({"item_id": item_id, "item_name": item_name, "value": int(value)})
    return out


def _extract_reward_preview(
    event: dict[str, Any],
    ctx: PlayerContext,
    item_map: dict[str, dict[str, Any]],
) -> list[dict[str, Any]]:
    rewards = event.get("rewards") or []
    if not rewards:
        return []

    skill_id = str(event.get("need_skill") or "none")
    skill_output_multiplier = _skill_output_multiplier(ctx, skill_id)
    skill_flat = 0.0 if skill_id == "none" else _to_float(ctx.attr_set.get(f"{skill_id}_reward_flat", 0.0), 0.0)
    common_mult = _to_float(ctx.attr_set.get("reward_mult", 0.0), 0.0)
    common_flat = _to_float(ctx.attr_set.get("reward_flat", 0.0), 0.0)

    out: list[dict[str, Any]] = []
    for rew in rewards:
        reward_type = str(rew.get("type") or "item").lower()
        if reward_type != "item":
            continue

        item_id = str(rew.get("id") or "")
        if not item_id:
            continue
        base_raw = rew.get("num")
        if base_raw is None:
            base_raw = rew.get("value", 0)
        base_value = _to_float(base_raw, 0.0)
        if base_value <= 0:
            continue

        effective_value = (
            base_value
            * skill_output_multiplier
            * (1.0 + common_mult)
            + skill_flat
            + common_flat
        )
        out.append(
            {
                "item_id": item_id,
                "item_name": (item_map.get(item_id) or {}).get("name") or item_id,
                "base_value": base_value,
                "effective_value": max(0.0, effective_value),
            }
        )

    return out


def _extract_display_experience(event: dict) -> float:
    rewards = event.get("rewards") or []
    total = 0.0
    for rew in rewards:
        if str(rew.get("type") or "").lower() != "experience":
            continue
        total += _to_float(rew.get("value", rew.get("num", 0)), 0.0)
    # 兼容旧数据
    if total <= 0:
        total = _to_float(event.get("experience"), 0.0)
    return total


def _is_skill_blocked(requirements: list[dict] | None, ctx: PlayerContext) -> bool:
    for req in requirements or []:
        if req.get("type") != "skill":
            continue
        skill_id = req.get("id")
        skill = ctx.skills.get(skill_id)
        actual = float(skill.level) if skill else 0.0
        expected = _to_float(req.get("value"), 0.0)
        comp = req.get("comparison_types")
        if not _compare(actual, expected, comp):
            return True
    return False


def _extract_active_loop(
    ctx: PlayerContext,
    events_map: dict[str, dict[str, Any]],
) -> dict[str, Any] | None:
    raw_queue = json.loads(ctx.state.queue_json) if ctx.state.queue_json else []
    index = int(ctx.state.queue_index)
    if index < 0 or index >= len(raw_queue):
        return None

    item = raw_queue[index]
    if isinstance(item, str):
        event_id = item
    elif isinstance(item, dict):
        event_id = str(item.get("event_id") or "")
    else:
        return None
    if not event_id:
        return None
    event = events_map.get(event_id)
    if not event or event.get("type") != "loop":
        return None

    duration_seconds = _effective_loop_time(event, ctx)
    elapsed_seconds = max(0.0, float(ctx.state.queue_progress_seconds))
    affordable_iterations = estimate_affordable_iterations(event.get("requirements"), ctx)
    # Respect queue item iteration limit
    item_iterations = item.get("iterations") if isinstance(item, dict) else None
    if item_iterations is not None and item_iterations > 0:
        completed = int(item.get("completed", 0)) if isinstance(item, dict) else 0
        affordable_iterations = min(affordable_iterations, item_iterations - completed) if affordable_iterations is not None else (item_iterations - completed)
    return {
        "event_id": event_id,
        "elapsed_seconds": elapsed_seconds,
        "duration_seconds": duration_seconds,
        "available_iterations": affordable_iterations,
    }


def _extract_queue(
    ctx: PlayerContext,
    events_map: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    raw_queue = json.loads(ctx.state.queue_json) if ctx.state.queue_json else []
    index = int(ctx.state.queue_index)
    progress = float(ctx.state.queue_progress_seconds)

    items: list[dict[str, Any]] = []
    for i, raw in enumerate(raw_queue):
        if isinstance(raw, str):
            event_id = raw
            item_iterations = None
            item_completed = 0
        elif isinstance(raw, dict):
            event_id = str(raw.get("event_id") or "")
            item_iterations = raw.get("iterations") if raw.get("iterations") not in (None, 0) else None
            item_completed = int(raw.get("completed") or 0)
        else:
            continue
        event = events_map.get(event_id)
        if not event:
            continue
        remaining = None
        if item_iterations is not None and item_iterations > 0:
            remaining = max(0, item_iterations - item_completed)
        items.append(
            {
                "index": i,
                "event_id": event_id,
                "name": event.get("name") or event_id,
                "type": event.get("type") or "unknown",
                "map": event.get("map") or "unknown",
                "map_name": MAP_NAME_MAP.get(event.get("map") or "unknown", event.get("map") or "unknown"),
                "is_current": i == index,
                "is_executable": check_requirements(event.get("requirements"), ctx),
                "iterations": item_iterations,
                "completed": item_completed,
                "remaining": remaining,
            }
        )

    return {
        "items": items,
        "index": index,
        "progress_seconds": progress,
    }


def _skill_level(ctx: PlayerContext, skill_id: str, default: int = 1) -> int:
    skill = ctx.skills.get(skill_id)
    if skill is None:
        return default
    return max(1, int(skill.level))


def _collect_equipment_pieces(ctx: PlayerContext, item_map: dict[str, dict]) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    seen: set[tuple[str, str]] = set()
    for slot_id, row in ctx.equipment.items():
        anchor = str(getattr(row, "anchor_slot", None) or slot_id)
        piece_key = (anchor, str(row.item_id))
        if piece_key in seen:
            continue
        seen.add(piece_key)
        item_data = item_map.get(str(row.item_id)) or {}
        if not item_data.get("equipment"):
            continue
        details = item_data.get("equipment_details") or {}
        try:
            enhance_level = max(0, int(getattr(row, "enhance_level", 0) or 0))
        except (TypeError, ValueError):
            enhance_level = 0
        out.append(
            {
                "id": str(row.item_id),
                "type": str(details.get("type") or ""),
                "enhance_level": enhance_level,
                "basic": compute_scaled_item_data(item_data, "equipment", enhance_level),
            }
        )
    return out


def _build_profile_payload(ctx: PlayerContext, item_map: dict[str, dict]) -> dict[str, Any]:
    production_attributes: list[dict[str, Any]] = []
    for skill_id in PRODUCTION_SKILL_ORDER:
        skill_obj = ctx.skills.get(skill_id)
        base_level = int(skill_obj.level) if skill_obj else 1
        level_buff = _to_float(ctx.attr_set.get(f"{skill_id}_level_buff", 0.0), 0.0)
        effective_level = max(1, int(math.floor(base_level + level_buff)))

        production_bonus = _to_float(ctx.attr_set.get(f"{skill_id}_production_multiplier", 0.0), 0.0)
        speed_bonus = _to_float(ctx.attr_set.get(f"{skill_id}_speed_multiplier", 0.0), 0.0)

        level_multiplier = get_level_production_multiplier(effective_level)
        total_output_multiplier = max(0.0, (1.0 + production_bonus) * level_multiplier)
        total_speed_multiplier = max(0.05, 1.0 + speed_bonus)

        production_attributes.append(
            {
                "skill_id": skill_id,
                "skill_name": SKILL_NAME_MAP.get(skill_id, skill_id),
                "base_level": base_level,
                "effective_level": effective_level,
                "level_multiplier": level_multiplier,
                "production_multiplier": production_bonus,
                "speed_multiplier": speed_bonus,
                "total_output_multiplier": total_output_multiplier,
                "total_speed_multiplier": total_speed_multiplier,
            }
        )

    pieces = _collect_equipment_pieces(ctx, item_map)

    def sum_attr(*keys: str) -> float:
        total = 0.0
        for piece in pieces:
            basic = piece["basic"]
            for key in keys:
                if key in basic:
                    total += _to_float(basic.get(key), 0.0)
        return total

    def product_attr(*keys: str) -> float:
        result = 1.0
        has_value = False
        for piece in pieces:
            basic = piece["basic"]
            for key in keys:
                if key in basic:
                    result *= (1.0 + _to_float(basic.get(key), 0.0))
                    has_value = True
        return result if has_value else 1.0

    resilience_lv = _skill_level(ctx, "resilience")
    stamina_lv = _skill_level(ctx, "stamina")
    intelligence_lv = _skill_level(ctx, "intelligence")
    strength_lv = _skill_level(ctx, "strength")
    ranging_lv = _skill_level(ctx, "ranging")
    defense_lv = _skill_level(ctx, "defense")
    magic_lv = _skill_level(ctx, "magic")

    resilience_bonus_lv = max(0, resilience_lv - 1)
    stamina_bonus_lv = max(0, stamina_lv - 1)
    intelligence_bonus_lv = max(0, intelligence_lv - 1)
    strength_bonus_lv = max(0, strength_lv - 1)
    ranging_bonus_lv = max(0, ranging_lv - 1)
    defense_bonus_lv = max(0, defense_lv - 1)
    magic_bonus_lv = max(0, magic_lv - 1)

    hp = (BATTLE_BASE_DATA["hp"] + 5.0 * resilience_bonus_lv + sum_attr("max_hp", "hp")) * product_attr(
        "max_hp_multiplier",
        "hp_multiplier",
    )
    sp = (BATTLE_BASE_DATA["sp"] + 1.0 * stamina_bonus_lv + sum_attr("max_sp", "sp")) * product_attr(
        "max_sp_multiplier",
        "sp_multiplier",
    )
    mp = (BATTLE_BASE_DATA["mp"] + 1.0 * intelligence_bonus_lv + sum_attr("max_mp", "mp")) * product_attr(
        "max_mp_multiplier",
        "mp_multiplier",
    )

    power_multiplier = product_attr("power_multiplier")
    physical_power = (
        (BATTLE_BASE_DATA["physical_power"] + 1.0 * strength_bonus_lv + sum_attr("physical_power"))
        * power_multiplier
        * (1.005 ** strength_bonus_lv)
    )
    magic_power = (
        (BATTLE_BASE_DATA["magic_power"] + 1.0 * magic_bonus_lv + sum_attr("magic_power", "magic_damage"))
        * power_multiplier
        * (1.005 ** magic_bonus_lv)
    )

    weapon_intervals: list[float] = []
    for piece in pieces:
        if piece.get("type") != "weapon":
            continue
        interval = _to_float(piece["basic"].get("attack_interval"), 0.0)
        if interval > 0:
            weapon_intervals.append(interval)
    weapon_interval = max(weapon_intervals) if weapon_intervals else BATTLE_BASE_DATA["attack_interval"]
    attack_interval = max(
        0.1,
        weapon_interval / max(0.05, product_attr("attack_speed", "final_attack_speed_multiplier")),
    )

    critical = min(
        1.0,
        max(
            0.0,
            (BATTLE_BASE_DATA["critical"] + sum_attr("critical"))
            * product_attr("critical_possibility_multiplier"),
        ),
    )
    critical_rate = BATTLE_BASE_DATA["critical_rate"] + sum_attr("critical_rate", "critical_multiplier")

    block = (BATTLE_BASE_DATA["block"] + 1.0 * defense_bonus_lv + sum_attr("block")) * product_attr("block_multiplier")
    block_possibility_multiplier = (
        (1.0 + BATTLE_BASE_DATA["block_possibility_multiplier"])
        * product_attr("block_possibility_multiplier")
        - 1.0
    )
    block_rate = (BATTLE_BASE_DATA["block_rate"] + sum_attr("block_rate")) * product_attr("block_rate_multiplier")

    recovery_multiplier = product_attr("overall_recovery_speed")
    hp_recovery = (BATTLE_BASE_DATA["hp_recovery"] + sum_attr("hp_recovery")) * recovery_multiplier
    sp_recovery = (BATTLE_BASE_DATA["sp_recovery"] + 0.02 * stamina_bonus_lv + sum_attr("sp_recovery")) * recovery_multiplier
    mp_recovery = (BATTLE_BASE_DATA["mp_recovery"] + 0.02 * intelligence_bonus_lv + sum_attr("mp_recovery")) * recovery_multiplier

    accuracy = (BATTLE_BASE_DATA["accuracy"] + 0.5 * ranging_bonus_lv + sum_attr("accuracy")) * product_attr("accuracy_multiplier")
    accuracy_possibility_multiplier = (
        (1.0 + BATTLE_BASE_DATA["accuracy_possibility_multiplier"])
        * product_attr("accuracy_possibility_multiplier")
        - 1.0
    )

    evade = (BATTLE_BASE_DATA["evade"] + 0.5 * ranging_bonus_lv + sum_attr("evade")) * product_attr("evade_multiplier")
    evade_possibility_multiplier = (
        (1.0 + BATTLE_BASE_DATA["evade_possibility_multiplier"])
        * product_attr("evade_possibility_multiplier")
        - 1.0
    )

    magic_instance = (BATTLE_BASE_DATA["magic_instance"] + sum_attr("magic_instance")) * product_attr("magic_instance_multiplier")
    defense = (BATTLE_BASE_DATA["defense"] + 1.0 * defense_bonus_lv + sum_attr("defense")) * product_attr("defense_multiplier")
    hatred = (BATTLE_BASE_DATA["hatred"] + sum_attr("hatred")) * product_attr("hatred_multiplier")

    final_damage_multiplier = (
        (1.0 + BATTLE_BASE_DATA["final_damage_multiplier"])
        * product_attr("final_damage_multiplier")
        - 1.0
    )
    final_damage_reduce = (
        (1.0 + BATTLE_BASE_DATA["final_damage_reduce"])
        * product_attr("final_damage_reduce", "final_damage_induce")
        - 1.0
    )

    computed_battle: dict[str, float] = {
        "hp": hp,
        "mp": mp,
        "sp": sp,
        "physical_power": physical_power,
        "magic_power": magic_power,
        "attack_interval": attack_interval,
        "critical": critical,
        "critical_rate": critical_rate,
        "block": block,
        "block_possibility_multiplier": block_possibility_multiplier,
        "block_rate": block_rate,
        "accuracy": accuracy,
        "accuracy_possibility_multiplier": accuracy_possibility_multiplier,
        "evade": evade,
        "evade_possibility_multiplier": evade_possibility_multiplier,
        "magic_instance": magic_instance,
        "defense": defense,
        "hatred": hatred,
        "final_damage_multiplier": final_damage_multiplier,
        "final_damage_reduce": final_damage_reduce,
        "hp_recovery": hp_recovery,
        "mp_recovery": mp_recovery,
        "sp_recovery": sp_recovery,
    }

    battle_attributes: list[dict[str, Any]] = []
    for attr_id, attr_name, as_percent in BATTLE_ATTR_VIEW:
        battle_attributes.append(
            {
                "id": attr_id,
                "name": attr_name,
                "base": BATTLE_BASE_DATA.get(attr_id, 0.0),
                "value": _to_float(computed_battle.get(attr_id, BATTLE_BASE_DATA.get(attr_id, 0.0)), 0.0),
                "as_percent": as_percent,
            }
        )

    return {
        "production_attributes": production_attributes,
        "battle_attributes": battle_attributes,
    }


def _build_skill_payload(ctx: PlayerContext, skill_order: tuple[str, ...]) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    for skill_id in skill_order:
        skill_obj = ctx.skills.get(skill_id)
        level = int(skill_obj.level) if skill_obj else 1
        exp = float(skill_obj.exp) if skill_obj else 0.0
        progress, current_exp, next_exp = _skill_progress(level, exp)
        out.append(
            {
                "id": skill_id,
                "name": SKILL_NAME_MAP.get(skill_id, skill_id),
                "level": level,
                "exp": exp,
                "level_progress": progress,
                "current_level_total_exp": current_exp,
                "next_level_total_exp": next_exp,
                "level_production_multiplier": ctx.attr_set.get(
                    f"{skill_id}_level_production_multiplier",
                    1.0,
                ),
            }
        )
    return out




def _inventory_to_list(ctx: PlayerContext) -> list[dict[str, int | str]]:
    return ctx.inventory.to_list()


def _parse_item_dirty_key(key: str) -> tuple[str, int] | None:
    if not key.startswith("item:"):
        return None
    parts = key.split(":")
    # item:id  -> state defaults to 0
    # item:id:state
    if len(parts) == 2:
        return parts[1], 0
    if len(parts) >= 3:
        return parts[1], int(parts[2])
    return None

def _build_gameplay_payload_internal(
    ctx: PlayerContext,
    actions: dict[str, Any],
    *,
    include_static: bool = True,
) -> dict[str, Any]:
    items = actions.get("items", [])
    events = actions.get("events", [])
    events_map = {
        str(event.get("id") or ""): event
        for event in events
        if str(event.get("id") or "")
    }
    item_map = {item.get("id"): item for item in items}

    production_skills = _build_skill_payload(ctx, PRODUCTION_SKILL_ORDER)
    combat_skills = _build_skill_payload(ctx, COMBAT_SKILL_ORDER)

    payload: dict[str, Any] = {
        "production_skills": production_skills,
        "combat_skills": combat_skills,
        "equipment_view": build_equipment_view(ctx, item_map),
        # Raw state (for frontend computed display)
        "skills": {
            k: {"level": int(v.level), "exp": float(v.exp)}
            for k, v in ctx.skills.items()
        },
        "inventory": _inventory_to_list(ctx),
        "equipment": {slot: e.item_id for slot, e in ctx.equipment.items()},
        "tools": {slot: t.item_id for slot, t in ctx.tools.items()},
        "event_counts": {
            event_id: int(progress.completed_count)
            for event_id, progress in ctx.event_progress.items()
        },
        "seen_items": sorted(ctx.seen_items),
        "unlocked_events": sorted(ctx.unlocked),
        "attributes": ctx.attr_set.to_dict(),
        "queue_items": (
            json.loads(ctx.state.queue_json) if ctx.state.queue_json else []
        ),
        "queue_index": int(ctx.state.queue_index),
        "queue_progress_seconds": float(ctx.state.queue_progress_seconds),
    }

    if include_static:
        maps: list[dict[str, str]] = []
        seen_maps: set[str] = set()
        for event in events:
            map_id = str(event.get("map") or "unknown")
            if map_id in seen_maps:
                continue
            seen_maps.add(map_id)
            maps.append({"id": map_id, "name": MAP_NAME_MAP.get(map_id, map_id)})

        payload["maps"] = maps

    return payload


@database.player_atomic
def build_gameplay_payload(uid: int) -> dict[str, Any]:
    result = settle_player(uid)
    actions = _load_actions()
    return _build_gameplay_payload_internal(result.ctx, actions, include_static=True)


@database.player_atomic
def build_gameplay_light_payload(uid: int, *, settled: bool = False) -> dict[str, Any]:
    if not settled:
        result = settle_player(uid)
        ctx = result.ctx
    else:
        session = database.get_db()
        ctx = PlayerContext.load(session, uid)
    actions = _load_actions()
    return _build_gameplay_payload_internal(ctx, actions, include_static=False)


# ---------------------------------------------------------------------------
# Delta patch — raw state only
# ---------------------------------------------------------------------------

@database.player_atomic
def build_patch_payload(uid: int) -> dict[str, Any]:
    """Build a raw-state patch payload from dirty keys.

    Returns {"patch": {...}} containing only changed raw state fields.
    Display views (loop_events, item_panels, profile, etc.) are computed
    on the frontend and are no longer included in delta patches.
    """
    dirty_raw = pop_dirty_keys(uid)
    if not dirty_raw:
        return {}

    skill_ids = {k[6:] for k in dirty_raw if k.startswith("skill:")}
    item_dirty_keys = [k for k in dirty_raw if k.startswith("item:")]
    event_progress_ids = {k[15:] for k in dirty_raw if k.startswith("event_progress:")}
    eq_slots = {k[10:] for k in dirty_raw if k.startswith("equipment:")}
    tool_slots = {k[5:] for k in dirty_raw if k.startswith("tool:")}
    has_queue = "queue" in dirty_raw
    has_queue_progress = "queue_progress" in dirty_raw
    has_unlocked = "unlocked" in dirty_raw
    has_new_seen_items = "new_seen_items" in dirty_raw

    if not any([
        skill_ids, item_dirty_keys, event_progress_ids, eq_slots, tool_slots,
        has_queue, has_queue_progress, has_unlocked, has_new_seen_items,
    ]):
        return {}

    session = database.get_db()
    ctx = PlayerContext.load(session, uid)

    patch: dict[str, Any] = {}

    if skill_ids:
        patch["skills"] = {}
        for sid in skill_ids:
            sk = ctx.skills.get(sid)
            if sk:
                patch["skills"][sid] = {"level": int(sk.level), "exp": float(sk.exp)}

    if item_dirty_keys:
        patch["inventory"] = []
        for key in item_dirty_keys:
            parsed = _parse_item_dirty_key(key)
            if not parsed:
                continue
            item_id, state = parsed
            qty = ctx.inventory.quantity_of(item_id, state)
            if qty > 0:
                entry: dict[str, int | str] = {"id": item_id, "qty": qty}
                if state != 0:
                    entry["state"] = state
                patch["inventory"].append(entry)

    if eq_slots:
        patch["equipment"] = {}
        for slot in eq_slots:
            eq = ctx.equipment.get(slot)
            patch["equipment"][slot] = eq.item_id if eq else None

    if tool_slots:
        patch["tools"] = {}
        for slot in tool_slots:
            t = ctx.tools.get(slot)
            patch["tools"][slot] = t.item_id if t else None

    if event_progress_ids:
        patch["event_counts"] = {}
        for eid in event_progress_ids:
            patch["event_counts"][eid] = ctx.get_event_count(eid)

    if has_new_seen_items:
        newly_seen = pop_newly_seen(uid)
        if newly_seen:
            patch["new_seen_items"] = sorted(newly_seen)

    if has_unlocked:
        patch["unlocked_events"] = sorted(ctx.unlocked)

    if has_queue:
        patch["queue_items"] = (
            json.loads(ctx.state.queue_json) if ctx.state.queue_json else []
        )
        patch["queue_index"] = int(ctx.state.queue_index)
        patch["queue_progress_seconds"] = float(ctx.state.queue_progress_seconds)

    if has_queue_progress:
        patch["queue_progress_seconds"] = float(ctx.state.queue_progress_seconds)
        patch["queue_index"] = int(ctx.state.queue_index)

    if eq_slots or tool_slots:
        patch["attributes"] = ctx.attr_set.to_dict()
    elif skill_ids:
        # Only skill levels changed; modifiers from equipment/tools are unchanged.
        # Send only the attributes derived from those skill levels.
        changed_attrs: dict[str, float] = {}
        full_attrs = ctx.attr_set.to_dict()
        for sid in skill_ids:
            for attr_key in (f"{sid}_reward_mult", f"{sid}_level_production_multiplier"):
                if attr_key in full_attrs:
                    changed_attrs[attr_key] = full_attrs[attr_key]
        if changed_attrs:
            patch["attributes"] = changed_attrs

    return {"patch": patch}
