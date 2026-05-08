"""核心结算引擎。离线回溯模式：根据 elapsed 时间回放队列动作。"""

import json
import math
import random
import time
from bisect import bisect_right
from typing import Any

from sqlalchemy.orm import Session

from models import database
from models import PlayerSkill
from data.data import DataManager
from data.level_exp_requirements import LEVEL_UP_TOTAL_EXP_REQUIREMENTS
from game.attributes import get_level_production_multiplier, PRODUCTION_SKILLS
from game.context import PlayerContext
from game.combat import simulate_combat

data_manager = DataManager()

# ---------------------------------------------------------------------------
# 数据缓存
# ---------------------------------------------------------------------------
_events_map: dict[str, dict] | None = None


def get_events_map() -> dict[str, dict]:
    global _events_map
    if _events_map is None:
        with open(data_manager.actions, "r", encoding="utf-8") as f:
            actions = json.load(f)
        _events_map = {e["id"]: e for e in actions.get("events", [])}
    return _events_map


# ---------------------------------------------------------------------------
# 需求检查 / 消耗 / 奖励 / 经验
# ---------------------------------------------------------------------------
from game.core_utils import compare as _compare, to_float as _to_float


def _requirement_expected_value(req: dict) -> float:
    if req.get("value") is not None:
        return _to_float(req.get("value"), 0.0)
    # event 前置未写 value 时，默认要求触发 >=1 次
    if req.get("type") == "event":
        return 1.0
    return 0.0


def _event_count(ctx: PlayerContext, event_id: str | None) -> float:
    if not event_id:
        return 0.0
    return float(ctx.get_event_count(event_id))


def _skill_output_multiplier(ctx: PlayerContext, skill_id: str | None) -> float:
    """读取“个人信息-生产属性”的总产出倍率（total_output_multiplier）。"""
    if not skill_id or skill_id == "none":
        return 1.0
    if skill_id not in PRODUCTION_SKILLS:
        return 1.0

    skill_obj = ctx.skills.get(skill_id)
    base_level = int(skill_obj.level) if skill_obj else 1
    level_buff = _to_float(ctx.attr_set.get(f"{skill_id}_level_buff", 0.0), 0.0)
    effective_level = max(1, int(math.floor(base_level + level_buff)))
    production_bonus = _to_float(ctx.attr_set.get(f"{skill_id}_production_multiplier", 0.0), 0.0)
    level_multiplier = get_level_production_multiplier(effective_level)
    return max(0.0, (1.0 + production_bonus) * level_multiplier)


def estimate_affordable_iterations(
    reqs: list[dict] | None,
    ctx: PlayerContext,
) -> int | None:
    """估算当前资源下最多可执行次数。

    返回:
    - None: 无显式 item/fluid 成本（可视为不受资源上限约束）
    - >=0: 受成本限制时的最大可执行次数
    """
    if not reqs:
        return None
    if not check_requirements(reqs, ctx):
        return 0

    limits: list[int] = []
    for req in reqs:
        if req.get("comparison_types") is not None:
            continue
        rtype = req.get("type")
        if rtype not in ("item", "fluid"):
            continue
        rid = req.get("id")
        cost = int(_to_float(req.get("value"), 0.0))
        if cost <= 0:
            continue

        qty = ctx.inventory.quantity_of(rid)
        limits.append(qty // cost)

    if not limits:
        return None
    return max(0, min(limits))


def check_requirements(
    reqs: list[dict] | None,
    ctx: PlayerContext,
) -> bool:
    if not reqs:
        return True
    for req in reqs:
        rtype = req.get("type")
        rid = req.get("id")
        value = _requirement_expected_value(req)
        comp = req.get("comparison_types")

        if rtype == "skill":
            sk = ctx.skills.get(rid)
            actual = sk.level if sk else 0
        elif rtype in ("item", "fluid"):
            if comp is None and value <= 0:
                actual = 1 if ctx.has_seen_item(rid) else 0
                value = 1
            else:
                actual = ctx.inventory.quantity_of(rid)
        elif rtype == "event":
            actual = _event_count(ctx, rid)
        else:
            continue

        if not _compare(actual, value, comp):
            return False
    return True


def check_unlock_requirements(
    reqs: list[dict] | None,
    ctx: PlayerContext,
) -> bool:
    """用于展示层的解锁判定：忽略 skill 阻塞，但保留其他门槛。"""
    if not reqs:
        return True
    for req in reqs:
        rtype = req.get("type")
        rid = req.get("id")
        value = _requirement_expected_value(req)
        comp = req.get("comparison_types")

        if rtype == "skill":
            # 技能不足时仍显示，但不可执行
            continue
        if rtype in ("item", "fluid"):
            if comp is None:
                actual = 1 if ctx.has_seen_item(rid) else 0
                value = 1
            else:
                actual = ctx.inventory.quantity_of(rid)
        elif rtype == "event":
            actual = _event_count(ctx, rid)
        else:
            continue

        if not _compare(actual, value, comp):
            return False
    return True


def refresh_unlocked_events(ctx: PlayerContext, events_map: dict[str, dict] | None = None) -> None:
    """O(n) 扫描事件表，重建当前可见（已解锁）事件集合。"""
    if events_map is None:
        events_map = get_events_map()

    unlocked_now: set[str] = set()
    for event_id, event in events_map.items():
        if check_unlock_requirements(event.get("requirements"), ctx):
            if event.get("type") == "upgrade":
                raw_limit = event.get("repeat_time")
                if raw_limit is None:
                    # 兼容历史数据：旧字段写在 loop_time
                    raw_limit = event.get("loop_time")
                max_exec = 1
                if raw_limit is not None:
                    try:
                        max_exec = max(1, int(raw_limit))
                    except (TypeError, ValueError):
                        max_exec = 1
                if ctx.get_event_count(event_id) >= max_exec:
                    continue
            unlocked_now.add(event_id)
    if ctx.unlocked != unlocked_now:
        ctx.unlocked = unlocked_now
        ctx.mark_dirty("unlocked")


def apply_costs(
    reqs: list[dict] | None,
    ctx: PlayerContext,
) -> None:
    """仅对 item/fluid 类型且没有 comparison_types 的进行扣除。"""
    if not reqs:
        return
    for req in reqs:
        rtype = req.get("type")
        if rtype not in ("item", "fluid"):
            continue
        if req.get("comparison_types") is not None:
            continue
        rid = req.get("id")
        value = int(_to_float(req.get("value"), 0.0))
        if value <= 0:
            continue
        if rtype in ("item", "fluid"):
            ctx.inventory.consume(rid, value)


def apply_rewards(
    rewards: list[dict] | None,
    ctx: PlayerContext,
    skill_id: str | None = None,
) -> float:
    total_exp_gained = 0.0
    if not rewards:
        return total_exp_gained
    for rew in rewards:
        rtype = str(rew.get("type") or "item").lower()
        if rtype == "experience":
            exp_raw = rew.get("value")
            if exp_raw is None:
                exp_raw = rew.get("num", 0)

            reward_skill_id = rew.get("skill_id")
            if not reward_skill_id:
                rid = rew.get("id")
                if isinstance(rid, str) and rid.endswith("_experience"):
                    reward_skill_id = rid[: -len("_experience")]
            if not reward_skill_id:
                reward_skill_id = skill_id

            total_exp_gained += apply_experience(exp_raw, reward_skill_id, ctx)
            continue

        rid = rew.get("id")
        base_num = rew.get("num")
        if base_num is None:
            base_num = rew.get("value", 0)
        num = _resolve_quantity(float(base_num), ctx, skill_id)
        if num <= 0:
            continue
        ctx.inventory.add(rid, num)
    return total_exp_gained


def apply_experience(
    raw_exp: Any,
    skill_id: str | None,
    ctx: PlayerContext,
) -> float:
    if not skill_id or skill_id == "none" or raw_exp is None:
        return 0.0
    try:
        exp_val = float(raw_exp)
    except (TypeError, ValueError):
        return 0.0
    if exp_val <= 0:
        return 0.0

    # 经验倍率
    exp_mult = 1.0 + ctx.attr_set.get("exp_gain_multiplier", 0.0)
    exp_val *= exp_mult
    # 产出倍率同样作用于经验收益
    if skill_id and skill_id != "none":
        exp_val *= _skill_output_multiplier(ctx, skill_id)
    exp_val *= (1.0 + ctx.attr_set.get("reward_mult", 0.0))

    sk = ctx.skills.get(skill_id)
    if sk is None:
        sk = PlayerSkill(uid=ctx.uid, skill_id=skill_id, level=1, exp=0.0)
        ctx.session.add(sk)
        ctx.skills[skill_id] = sk
    total_exp = max(0.0, float(sk.exp)) + exp_val
    sk.exp = total_exp
    sk.level = bisect_right(LEVEL_UP_TOTAL_EXP_REQUIREMENTS, total_exp) + 1
    ctx.mark_dirty(f"skill:{skill_id}")
    return exp_val


# ---------------------------------------------------------------------------
# 奖励数量修饰 + 概率取整
# ---------------------------------------------------------------------------
def _resolve_quantity(base_num: float, ctx: PlayerContext, skill_id: str | None) -> int:
    """计算属性修饰后的实际奖励数量，小数部分按概率取整。"""
    result = base_num
    # 技能专属乘算
    if skill_id and skill_id != "none":
        result *= _skill_output_multiplier(ctx, skill_id)
    # 通用乘算
    result *= (1.0 + ctx.attr_set.get("reward_mult", 0.0))
    # 技能专属加算
    if skill_id and skill_id != "none":
        result += ctx.attr_set.get(f"{skill_id}_reward_flat", 0.0)
    # 通用加算
    result += ctx.attr_set.get("reward_flat", 0.0)
    return _probabilistic_round(result)


def _probabilistic_round(value: float) -> int:
    """小数部分按概率取整。例如 3.7 → 70% 概率得 4，30% 概率得 3。"""
    base = int(value)
    frac = value - base
    if frac <= 0:
        return base
    if random.random() < frac:
        return base + 1
    return base


# ---------------------------------------------------------------------------
# loop_time 属性修饰
# ---------------------------------------------------------------------------
def _effective_loop_time(event: dict, ctx: PlayerContext) -> float:
    """计算属性修饰后的实际 loop_time。实际时间 = 基础时间 / (1 + 技能速度加成)。"""
    base = float(event.get("loop_time", 1))
    if base <= 0:
        base = 1.0

    # 战斗类事件用战斗模拟的 duration
    if event.get("combat"):
        enemy = event.get("enemy", {})
        result = simulate_combat(ctx, enemy)
        return max(result.duration, 0.1)

    # 非战斗：受技能速度加成影响
    skill_id = event.get("need_skill")
    if skill_id and skill_id != "none":
        speed_mult = ctx.attr_set.get(f"{skill_id}_speed_multiplier", 0.0)
        if speed_mult > 0:
            base = base / (1.0 + speed_mult)
    return max(base, 0.1)


# ---------------------------------------------------------------------------
# 核心结算
# ---------------------------------------------------------------------------
def _normalize_queue(raw_queue: list) -> list[dict]:
    """将原始队列数据规范化。"""
    out: list[dict] = []
    for item in raw_queue:
        if isinstance(item, str):
            out.append({"event_id": item, "iterations": None, "completed": 0})
        elif isinstance(item, dict):
            out.append({
                "event_id": str(item.get("event_id") or ""),
                "iterations": item.get("iterations") if item.get("iterations") not in (None, 0) else None,
                "completed": int(item.get("completed") or 0),
            })
    return out


def _process_loop_item(
    ctx: PlayerContext,
    queue_item: dict,
    event: dict,
    index: int,
    progress: float,
    remaining: float,
    log: list[dict],
) -> tuple[int, float, float, bool]:
    """处理 loop 类型队列项。

    返回: (new_index, new_progress, new_remaining, should_continue_main_loop)
    """
    loop_time = _effective_loop_time(event, ctx)
    skill_id = event.get("need_skill")
    event_id = queue_item["event_id"]

    total_time_budget = progress + remaining
    max_iterations_by_time = int(total_time_budget // loop_time)
    max_iterations_by_cost = estimate_affordable_iterations(event.get("requirements"), ctx)
    if max_iterations_by_cost is None:
        target_iterations = max_iterations_by_time
    else:
        target_iterations = min(max_iterations_by_time, max_iterations_by_cost)

    item_iterations = queue_item.get("iterations")
    if item_iterations is not None and item_iterations > 0:
        remaining_iters = item_iterations - queue_item.get("completed", 0)
        target_iterations = min(target_iterations, remaining_iters)

    iterations = 0
    exp_gain_total = 0.0

    for _ in range(target_iterations):
        if not check_requirements(event.get("requirements"), ctx):
            break
        apply_costs(event.get("requirements"), ctx)
        exp_gain_total += apply_rewards(event.get("rewards"), ctx, skill_id)
        # 兼容旧数据：若仍存在 event.experience 字段，继续发放
        exp_gain_total += apply_experience(event.get("experience"), skill_id, ctx)
        iterations += 1
        queue_item["completed"] = queue_item.get("completed", 0) + 1

    if max_iterations_by_time <= 0:
        return index, total_time_budget, 0.0, False

    total_seconds_spent = 0.0
    if iterations > 0:
        total_seconds_spent = max(0.0, iterations * loop_time - progress)
    new_remaining = max(0.0, remaining - total_seconds_spent)

    if iterations >= max_iterations_by_time:
        new_progress = min(loop_time, new_remaining)
        new_remaining = 0.0
    else:
        new_progress = 0.0

    if iterations == 0 and new_progress == 0:
        return index, new_progress, new_remaining, False

    if iterations > 0:
        ctx.mark_event_completed(event_id, iterations)
        log.append({
            "event_id": event_id,
            "iterations": iterations,
            "experience": exp_gain_total,
        })

    item_limit_reached = False
    if item_iterations is not None and item_iterations > 0:
        if queue_item.get("completed", 0) >= item_iterations:
            item_limit_reached = True

    if item_limit_reached:
        return index + 1, 0.0, new_remaining, True
    if new_remaining <= 0 and abs(new_progress) < 1e-9:
        return index + 1, 0.0, new_remaining, True
    if new_remaining > 0 and iterations < max_iterations_by_time:
        return index, new_progress, new_remaining, False

    return index, new_progress, new_remaining, True


def _process_instant_item(ctx: PlayerContext, event: dict, log: list[dict]) -> None:
    """处理 instant 类型队列项。"""
    skill_id = event.get("need_skill")
    event_id = event["id"]
    apply_rewards(event.get("rewards"), ctx, skill_id)
    # 兼容旧数据：若仍存在 event.experience 字段，继续发放
    apply_experience(event.get("experience"), skill_id, ctx)
    ctx.mark_event_completed(event_id, 1)
    log.append({"event_id": event_id, "iterations": 1})


def _process_upgrade_item(ctx: PlayerContext, event: dict, log: list[dict]) -> None:
    """处理 upgrade 类型队列项。"""
    skill_id = event.get("need_skill")
    event_id = event["id"]
    apply_rewards(event.get("rewards"), ctx, skill_id)
    # 兼容旧数据：若仍存在 event.experience 字段，继续发放
    apply_experience(event.get("experience"), skill_id, ctx)
    ctx.unlocked.add(event_id)
    ctx.mark_event_completed(event_id, 1)
    log.append({"event_id": event_id, "iterations": 1})


def _simulate_elapsed(ctx: PlayerContext, elapsed: float) -> list[dict]:
    """模拟消耗 elapsed 秒，直接读写 ctx.state 的队列状态，返回日志。"""
    raw_queue = json.loads(ctx.state.queue_json) if ctx.state.queue_json else []
    queue = _normalize_queue(raw_queue)

    index: int = ctx.state.queue_index
    progress: float = ctx.state.queue_progress_seconds
    events_map = get_events_map()
    log: list[dict] = []
    remaining = float(elapsed)

    while remaining > 0 and index < len(queue):
        queue_item = queue[index]
        event_id = queue_item["event_id"]
        event = events_map.get(event_id)
        if not event:
            index += 1
            progress = 0.0
            continue

        if not check_requirements(event.get("requirements"), ctx):
            break

        etype = event.get("type")

        if etype == "loop":
            index, progress, remaining, should_continue = _process_loop_item(
                ctx, queue_item, event, index, progress, remaining, log
            )
            if not should_continue:
                break
        elif etype == "instant":
            _process_instant_item(ctx, event, log)
            index += 1
            progress = 0.0
        elif etype == "upgrade":
            _process_upgrade_item(ctx, event, log)
            index += 1
            progress = 0.0
        else:
            index += 1
            progress = 0.0

    # 清理已完成的队列项，避免已完成活动堆积在队列头部
    if index >= len(queue):
        queue = []
        index = 0
        progress = 0.0
    elif index > 0:
        queue = queue[index:]
        index = 0

    ctx.state.queue_json = json.dumps(queue)
    ctx.state.queue_index = index
    ctx.state.queue_progress_seconds = progress
    return log


class SettlementResult(dict):
    """settle_player 的返回对象：本身是一个 dict（兼容旧接口），
    同时可通过 .ctx / .elapsed / .log 访问 settlement 的原始结果。

    .ctx 带有上下文安全检查：只有在创建它的同一个 managed session
    生命周期内才能访问，防止 session 已关闭后误用导致 DetachedInstanceError。
    """

    def __init__(self, ctx: PlayerContext, elapsed: float, log: list[dict]):
        super().__init__(self.build_state_response(ctx, elapsed, log))
        self._ctx = ctx
        self.elapsed = elapsed
        self.log = log

    @property
    def ctx(self) -> PlayerContext:
        current = database.get_db()
        if self._ctx.session is not current:
            raise RuntimeError(
                "SettlementResult.ctx 只能在创建它的同一个 managed session "
                "生命周期内访问。当前 session 已结束或发生了切换。"
            )
        return self._ctx

    @staticmethod
    def build_state_response(ctx: PlayerContext, elapsed: float, log: list[dict]) -> dict:
        """将 PlayerContext 序列化为前端 state JSON。"""
        return {
            "uid": ctx.uid,
            "inventory": ctx.inventory_to_state(),
            "skills": {
                k: {"level": v.level, "exp": v.exp}
                for k, v in ctx.skills.items()
            },
            "unlocked_events": sorted(ctx.unlocked),
            "event_counts": {
                event_id: int(progress.completed_count)
                for event_id, progress in ctx.event_progress.items()
            },
            "seen_items": sorted(ctx.seen_items),
            "queue": (
                json.loads(ctx.state.queue_json) if ctx.state.queue_json else []
            ),
            "queue_index": ctx.state.queue_index,
            "queue_progress_seconds": ctx.state.queue_progress_seconds,
            "last_sync_time": ctx.state.last_sync_time,
            "settled_seconds": elapsed,
            "settlement_log": log,
            "equipment": {
                slot: e.item_id for slot, e in ctx.equipment.items()
            },
            "tools": {slot: t.item_id for slot, t in ctx.tools.items()},
            "attributes": ctx.attr_set.to_dict(),
        }


@database.player_atomic
def settle_player(uid: int, elapsed: float | None = None) -> SettlementResult:
    session = database.get_db()
    ctx = PlayerContext.load(session, uid)

    if elapsed is None:
        now = time.time()
        elapsed = now - ctx.state.last_sync_time
    else:
        now = ctx.state.last_sync_time + elapsed

    old_queue_json = ctx.state.queue_json
    old_queue_index = ctx.state.queue_index
    old_queue_progress = ctx.state.queue_progress_seconds

    log = _simulate_elapsed(ctx, elapsed)

    ctx.state.last_sync_time = now

    if ctx.state.queue_json != old_queue_json:
        ctx.mark_dirty("queue")
    elif ctx.state.queue_index != old_queue_index or abs(ctx.state.queue_progress_seconds - old_queue_progress) > 1e-9:
        ctx.mark_dirty("queue_progress")

    refresh_unlocked_events(ctx, get_events_map())
    ctx.save()

    return SettlementResult(ctx, elapsed, log)


@database.player_atomic
def skip_time(uid: int, seconds: float) -> list[dict]:
    """跳过指定秒数，直接推进玩家状态（用于测试）。

    与 settle_player 的区别：
    - settle_player 根据 last_sync_time 计算 elapsed（依赖当前真实时间）
    - skip_time 直接传入要推进的秒数，不依赖 time.time()
    """
    session = database.get_db()
    ctx = PlayerContext.load(session, uid)
    old_queue_json = ctx.state.queue_json
    old_queue_index = ctx.state.queue_index
    old_queue_progress = ctx.state.queue_progress_seconds
    log = _simulate_elapsed(ctx, seconds)
    if ctx.state.queue_json != old_queue_json:
        ctx.mark_dirty("queue")
    elif ctx.state.queue_index != old_queue_index or abs(ctx.state.queue_progress_seconds - old_queue_progress) > 1e-9:
        ctx.mark_dirty("queue_progress")
    refresh_unlocked_events(ctx, get_events_map())
    ctx.save()
    return log
