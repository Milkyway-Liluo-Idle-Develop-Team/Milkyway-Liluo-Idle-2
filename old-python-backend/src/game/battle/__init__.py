"""战斗系统模块。拆分自原 battle_runtime.py，按职责分层。

目录结构：
- core.py      基础工具函数与常量
- entity.py    实体状态管理（属性刷新、自然恢复）
- skills.py    技能系统（收集、规范化、条件评估、效果应用）
- session.py   Session 管理（内存缓存、持久化、运行时 shape 修复）
- combat.py    战斗伤害计算与攻击执行
- rewards.py   奖励与波次系统
- runtime.py   主循环与公共接口
"""

from game.battle.runtime import (
    start_battle,
    get_battle_state,
    stop_battle,
    step_battle,
    list_battles,
    is_battle_running,
)
from game.battle.session import _sessions
from game.battle.core import _to_float, _clamp
from game.battle.entity import _refresh_entity_stats
from game.battle.skills import _normalize_battle_skill
from game.battle.combat import _calc_damage_result, _choose_player_skill
from game.battle.rewards import _spawn_wave, _pick_weighted_combination, _normalize_enemy
from game.battle.runtime import _snapshot, _advance_one_event, _catch_up_session

__all__ = [
    "start_battle",
    "get_battle_state",
    "stop_battle",
    "step_battle",
    "list_battles",
    "is_battle_running",
]
