"""战斗模拟模块。纯计算，不依赖 DB。"""

from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from game.context import PlayerContext


@dataclass(slots=True)
class CombatResult:
    victory: bool
    duration: float  # 一次战斗耗时（秒）


def simulate_combat(ctx: "PlayerContext", enemy: dict) -> CombatResult:
    """基于玩家属性和敌人数据，模拟一场战斗。

    伤害公式: damage = max(1, atk - enemy_def)
    击杀时间: kill_time = ceil(enemy_hp / player_dmg) * player_interval
    玩家死亡时间: death_time = ceil(player_hp / enemy_dmg) * enemy_interval
    victory = kill_time <= death_time (或玩家无法被击杀)
    """
    player_atk = ctx.attr_set.get("physical_damage", 1.0)
    player_def = ctx.attr_set.get("defense", 0.0)
    player_acc = ctx.attr_set.get("accuracy", 10.0)
    player_interval = ctx.attr_set.get("attack_interval", 2.0)

    enemy_hp = float(enemy.get("hp", 50))
    enemy_atk = float(enemy.get("attack", 5))
    enemy_def = float(enemy.get("defense", 0))
    enemy_interval = float(enemy.get("attack_interval", 3))

    # 玩家对敌人的每次伤害
    player_dmg = max(1.0, player_atk - enemy_def)
    # 敌人对玩家的每次伤害
    enemy_dmg = max(1.0, enemy_atk - player_def)

    # 击杀所需攻击次数
    import math
    hits_to_kill = math.ceil(enemy_hp / player_dmg)
    kill_time = hits_to_kill * player_interval

    # 目前简化模型：玩家总是胜利，duration = kill_time
    # 未来可以引入玩家 HP 和死亡判定
    return CombatResult(victory=True, duration=max(kill_time, player_interval))
