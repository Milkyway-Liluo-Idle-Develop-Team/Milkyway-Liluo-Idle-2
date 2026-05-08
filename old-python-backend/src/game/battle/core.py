"""战斗系统基础工具函数与常量。"""

from __future__ import annotations

from typing import Any

INIT_BATTLE_DATA: dict[str, float] = {
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
    "accuracy": 20.0,
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

SKILL_NAME_MAP: dict[str, str] = {
    "strength": "力量",
    "ranging": "远程",
    "resilience": "坚韧",
    "stamina": "耐力",
    "intelligence": "智力",
    "defense": "防御",
    "magic": "魔法",
}


from game.core_utils import to_float as _to_float, compare as _compare, clamp as _clamp
