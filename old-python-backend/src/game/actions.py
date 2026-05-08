"""即时行动 / 升级行动的执行逻辑。"""

from data.level_exp_requirements import LEVEL_UP_TOTAL_EXP_REQUIREMENTS
from models import database
from game.context import PlayerContext
from game.attributes import get_items_map
from game.settlement import (
    settle_player,
    SettlementResult,
    get_events_map,
    check_requirements,
    _effective_loop_time,
    refresh_unlocked_events,
    apply_costs,
    apply_rewards,
    apply_experience,
)


def _upgrade_max_executions(event: dict) -> int:
    raw = event.get("repeat_time")
    if raw is None:
        # 兼容历史数据：旧字段写在 loop_time
        raw = event.get("loop_time")
    if raw is None:
        return 1
    try:
        return max(1, int(raw))
    except (TypeError, ValueError):
        return 1


def _collect_item_changes(
    before: dict[tuple[str, int], int],
    after: dict[tuple[str, int], int],
    items_map: dict[str, dict],
) -> list[dict]:
    out: list[dict] = []
    all_keys = set(before) | set(after)
    for item_id, state in sorted(all_keys):
        b = int(before.get((item_id, state), 0))
        a = int(after.get((item_id, state), 0))
        if a == b:
            continue
        item_meta = items_map.get(item_id) or {}
        entry = {
            "item_id": item_id,
            "delta": a - b,
            "quantity": a,
            "name": item_meta.get("name") or item_id,
            "classification": item_meta.get("classification") or "other",
        }
        if state != 0:
            entry["state"] = state
        out.append(entry)
    return out


def _collect_skill_changes(
    before: dict[str, tuple[int, float]],
    after: dict[str, tuple[int, float]],
) -> list[dict]:
    out: list[dict] = []
    all_ids = set(before) | set(after)
    for skill_id in sorted(all_ids):
        b_level, b_exp = before.get(skill_id, (1, 0.0))
        a_level, a_exp = after.get(skill_id, (1, 0.0))
        if a_level == b_level and abs(a_exp - b_exp) < 1e-9:
            continue
        level_progress, current_total_exp, next_total_exp = _skill_progress(a_level, a_exp)
        out.append(
            {
                "skill_id": skill_id,
                "delta_exp": float(a_exp - b_exp),
                "level": int(a_level),
                "exp": float(a_exp),
                "level_progress": float(level_progress),
                "current_level_total_exp": float(current_total_exp),
                "next_level_total_exp": float(next_total_exp),
            }
        )
    return out


def _skill_progress(level: int, exp: float) -> tuple[float, float, float]:
    safe_level = max(1, int(level))
    safe_exp = max(0.0, float(exp))

    prev_need = 0.0 if safe_level <= 1 else float(LEVEL_UP_TOTAL_EXP_REQUIREMENTS[safe_level - 2])
    if safe_level - 1 < len(LEVEL_UP_TOTAL_EXP_REQUIREMENTS):
        next_need = float(LEVEL_UP_TOTAL_EXP_REQUIREMENTS[safe_level - 1])
    else:
        next_need = prev_need

    if next_need <= prev_need:
        return 1.0, prev_need, next_need
    ratio = (safe_exp - prev_need) / (next_need - prev_need)
    return max(0.0, min(1.0, ratio)), prev_need, next_need


@database.player_atomic
def execute_loop_once(uid: int, event_id: str) -> dict:
    result = settle_player(uid)
    ctx = result.ctx

    events_map = get_events_map()
    event = events_map.get(event_id)
    if not event:
        raise ValueError(f"Invalid event id: {event_id}")
    if event.get("type") != "loop":
        raise ValueError(f"Event {event_id} is not a loop action")

    if not check_requirements(event.get("requirements"), ctx):
        raise ValueError(f"Requirements not met for {event_id}")

    before_inv = ctx.inventory.snapshot()
    before_skills = {
        skill_id: (int(obj.level), float(obj.exp))
        for skill_id, obj in ctx.skills.items()
    }
    before_unlocked = set(ctx.unlocked)

    skill_id = event.get("need_skill")
    apply_costs(event.get("requirements"), ctx)
    exp_gain = apply_rewards(event.get("rewards"), ctx, skill_id)
    # 兼容旧数据：若仍存在 event.experience 字段，继续发放
    exp_gain += apply_experience(event.get("experience"), skill_id, ctx)
    ctx.mark_event_completed(event_id, 1)
    refresh_unlocked_events(ctx, events_map)

    ctx.save()

    after_inv = ctx.inventory.snapshot()
    after_skills = {
        skill_id: (int(obj.level), float(obj.exp))
        for skill_id, obj in ctx.skills.items()
    }
    items_map = get_items_map()

    unlocked_added = sorted(ctx.unlocked - before_unlocked)
    unlocked_removed = sorted(before_unlocked - ctx.unlocked)

    return {
        "event_id": event_id,
        "loop_time": _effective_loop_time(event, ctx),
        "event_count": ctx.get_event_count(event_id),
        "experience": exp_gain,
        "item_changes": _collect_item_changes(before_inv, after_inv, items_map),
        "skill_changes": _collect_skill_changes(before_skills, after_skills),
        "unlocked_added": unlocked_added,
        "unlocked_removed": unlocked_removed,
    }


@database.player_atomic
def execute_instant(uid: int, event_id: str) -> dict:
    result = settle_player(uid)
    ctx = result.ctx

    events_map = get_events_map()
    event = events_map.get(event_id)
    if not event:
        raise ValueError(f"Invalid event id: {event_id}")
    if event.get("type") != "instant":
        raise ValueError(f"Event {event_id} is not an instant action")

    if not check_requirements(event.get("requirements"), ctx):
        raise ValueError(f"Requirements not met for {event_id}")

    skill_id = event.get("need_skill")
    apply_costs(event.get("requirements"), ctx)
    apply_rewards(event.get("rewards"), ctx, skill_id)
    # 兼容旧数据：若仍存在 event.experience 字段，继续发放
    apply_experience(event.get("experience"), skill_id, ctx)
    ctx.mark_event_completed(event_id, 1)
    refresh_unlocked_events(ctx, events_map)

    ctx.save()

    return SettlementResult.build_state_response(
        ctx,
        elapsed=0.0,
        log=[{"event_id": event_id, "iterations": 1}],
    )


@database.player_atomic
def execute_upgrade(uid: int, event_id: str) -> dict:
    result = settle_player(uid)
    ctx = result.ctx

    events_map = get_events_map()
    event = events_map.get(event_id)
    if not event:
        raise ValueError(f"Invalid event id: {event_id}")
    if event.get("type") != "upgrade":
        raise ValueError(f"Event {event_id} is not an upgrade action")

    max_exec = _upgrade_max_executions(event)
    if ctx.get_event_count(event_id) >= max_exec:
        raise ValueError(f"Upgrade {event_id} already unlocked")

    if not check_requirements(event.get("requirements"), ctx):
        raise ValueError(f"Requirements not met for {event_id}")

    skill_id = event.get("need_skill")
    apply_costs(event.get("requirements"), ctx)
    apply_rewards(event.get("rewards"), ctx, skill_id)
    # 兼容旧数据：若仍存在 event.experience 字段，继续发放
    apply_experience(event.get("experience"), skill_id, ctx)
    ctx.unlocked.add(event_id)
    ctx.mark_event_completed(event_id, 1)
    refresh_unlocked_events(ctx, events_map)

    ctx.save()

    return SettlementResult.build_state_response(
        ctx,
        elapsed=0.0,
        log=[{"event_id": event_id, "iterations": 1}],
    )
