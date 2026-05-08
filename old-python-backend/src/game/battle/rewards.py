"""战斗奖励与波次系统。"""

from __future__ import annotations

import math
import random
import time
from bisect import bisect_right
from typing import Any

from data.level_exp_requirements import LEVEL_UP_TOTAL_EXP_REQUIREMENTS
from game.battle.core import _to_float, _clamp, INIT_BATTLE_DATA, SKILL_NAME_MAP
from data.data import load_actions as _load_actions
from game.battle.entity import _refresh_entity_stats
from game.battle.skills import _collect_enemy_battle_skills, _sanitize_skill_plan
from game.context import PlayerContext
from game.settlement import apply_rewards, get_events_map, refresh_unlocked_events
from models import database, PlayerSkill
from services import gameplay_service


def _pick_weighted_combination(combinations: list[dict[str, Any]]) -> list[str]:
    if not combinations:
        return []
    total = 0.0
    weights: list[float] = []
    for c in combinations:
        total += max(0.0, _to_float(c.get("weight"), 0.0))
        weights.append(total)
    if total <= 0:
        return [str(e) for e in (combinations[0].get("enemies") or [])]
    v = random.random() * total
    for idx, edge in enumerate(weights):
        if v <= edge:
            return [str(e) for e in (combinations[idx].get("enemies") or [])]
    return [str(e) for e in (combinations[-1].get("enemies") or [])]


def _normalize_enemy(enemy_def: dict[str, Any]) -> dict[str, Any]:
    data = dict(INIT_BATTLE_DATA)
    for key, value in (enemy_def.get("enemy_battle_data") or {}).items():
        if key == "final_damage_induce":
            data["final_damage_reduce"] = _to_float(value, data.get("final_damage_reduce", 0.0))
            continue
        data[key] = _to_float(value, data.get(key, 0.0))
    data["attack_interval"] = max(0.1, _to_float(data.get("attack_interval"), INIT_BATTLE_DATA["attack_interval"]))
    data["hp"] = max(1.0, _to_float(data.get("hp"), INIT_BATTLE_DATA["hp"]))
    return data


def _extract_battle_attr_map(ctx: PlayerContext, item_map: dict[str, dict[str, Any]]) -> dict[str, float]:
    profile = gameplay_service._build_profile_payload(ctx, item_map)
    out: dict[str, float] = {}
    for attr in profile.get("battle_attributes", []):
        attr_id = str(attr.get("id") or "")
        if not attr_id:
            continue
        out[attr_id] = _to_float(attr.get("value"), 0.0)
    return out


def _build_player_runtime_from_ctx(
    ctx: PlayerContext,
    item_map: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    from game.battle.skills import _collect_player_battle_skills

    attrs = _extract_battle_attr_map(ctx, item_map)

    max_hp = max(1.0, _to_float(attrs.get("hp"), 100.0))
    max_mp = max(0.0, _to_float(attrs.get("mp"), 100.0))
    max_sp = max(0.0, _to_float(attrs.get("sp"), 100.0))
    attack_interval = max(0.1, _to_float(attrs.get("attack_interval"), 2.0))

    skills, basic_skill_id = _collect_player_battle_skills(ctx, item_map, attack_interval)

    return {
        "name": ctx.state.user.username,
        "stats": {
            "hp": max_hp,
            "mp": max_mp,
            "sp": max_sp,
            "physical_power": _to_float(attrs.get("physical_power"), INIT_BATTLE_DATA["physical_power"]),
            "magic_power": _to_float(attrs.get("magic_power"), INIT_BATTLE_DATA["magic_power"]),
            "attack_interval": attack_interval,
            "critical": _to_float(attrs.get("critical"), INIT_BATTLE_DATA["critical"]),
            "critical_rate": _to_float(attrs.get("critical_rate"), INIT_BATTLE_DATA["critical_rate"]),
            "block": _to_float(attrs.get("block"), INIT_BATTLE_DATA["block"]),
            "block_possibility_multiplier": _to_float(attrs.get("block_possibility_multiplier"), INIT_BATTLE_DATA["block_possibility_multiplier"]),
            "block_rate": _to_float(attrs.get("block_rate"), INIT_BATTLE_DATA["block_rate"]),
            "accuracy": _to_float(attrs.get("accuracy"), INIT_BATTLE_DATA["accuracy"]),
            "evade": _to_float(attrs.get("evade"), INIT_BATTLE_DATA["evade"]),
            "magic_instance": _to_float(attrs.get("magic_instance"), INIT_BATTLE_DATA["magic_instance"]),
            "final_damage_multiplier": _to_float(attrs.get("final_damage_multiplier"), INIT_BATTLE_DATA["final_damage_multiplier"]),
            "final_damage_reduce": _to_float(attrs.get("final_damage_reduce"), INIT_BATTLE_DATA["final_damage_reduce"]),
            "defense": _to_float(attrs.get("defense"), INIT_BATTLE_DATA["defense"]),
            "hp_recovery": _to_float(attrs.get("hp_recovery"), INIT_BATTLE_DATA["hp_recovery"]),
            "mp_recovery": _to_float(attrs.get("mp_recovery"), INIT_BATTLE_DATA["mp_recovery"]),
            "sp_recovery": _to_float(attrs.get("sp_recovery"), INIT_BATTLE_DATA["sp_recovery"]),
        },
        "skills": skills,
        "basic_skill_id": basic_skill_id,
    }


def _spawn_wave(session: dict[str, Any], logs: list[dict[str, Any]]) -> None:
    session["wave_number"] += 1
    loop = session["battle"]["combination_loop"]
    wave_type = loop[(session["wave_number"] - 1) % len(loop)] if loop else "weak"
    session["wave_type"] = wave_type

    combinations = session["battle"]["combinations"].get(wave_type, [])
    chosen_enemy_ids = _pick_weighted_combination(combinations)

    enemies: list[dict[str, Any]] = []
    for idx, enemy_id in enumerate(chosen_enemy_ids, start=1):
        enemy_def = session["enemy_map"].get(enemy_id)
        if not enemy_def:
            continue
        stats = _normalize_enemy(enemy_def)
        attack_interval = max(0.1, _to_float(stats.get("attack_interval"), 2.0))
        skills, basic_skill_id = _collect_enemy_battle_skills(enemy_def, attack_interval)
        raw_skill_plan = enemy_def.get("battle_skill")
        if raw_skill_plan is None:
            raw_skill_plan = enemy_def.get("battle_skills")
        skill_plan = _sanitize_skill_plan(raw_skill_plan)
        enemies.append(
            {
                "instance_id": f"{session['wave_number']}-{idx}",
                "enemy_id": enemy_id,
                "name": enemy_def.get("name") or enemy_id,
                "stats": dict(stats),
                "base_stats": dict(stats),
                "hp": float(stats["hp"]),
                "max_hp": float(stats["hp"]),
                "mp": max(0.0, _to_float(stats.get("mp"), INIT_BATTLE_DATA["mp"])),
                "max_mp": max(0.0, _to_float(stats.get("mp"), INIT_BATTLE_DATA["mp"])),
                "sp": max(0.0, _to_float(stats.get("sp"), INIT_BATTLE_DATA["sp"])),
                "max_sp": max(0.0, _to_float(stats.get("sp"), INIT_BATTLE_DATA["sp"])),
                "next_ready_time": session["time"] + attack_interval,
                "last_action_duration": attack_interval,
                "last_skill_id": basic_skill_id,
                "last_skill_name": (skills.get(basic_skill_id) or {}).get("name") or basic_skill_id,
                "skills": skills,
                "basic_skill_id": basic_skill_id,
                "skill_plan": skill_plan,
                "cooldowns": {},
                "active_effects": [],
                "basic_damage_type": str(enemy_def.get("basic_damage_type") or "physical"),
                "rewards": list(enemy_def.get("rewards") or []),
                "alive": True,
            }
        )

    session["enemies"] = enemies
    session["next_wave_time"] = None
    session["player"]["next_ready_time"] = session["time"] + max(0.1, _to_float(session["player"]["stats"].get("attack_interval"), 2.0))

    logs.append(
        {
            "type": "wave_spawn",
            "wave_number": session["wave_number"],
            "wave_type": wave_type,
            "enemies": [e["enemy_id"] for e in enemies],
        }
    )


def _record_enemy_death(session: dict[str, Any], enemy: dict[str, Any], logs: list[dict[str, Any]]) -> None:
    enemy["alive"] = False
    enemy["hp"] = 0.0
    session["pending_rewards"].extend(enemy.get("rewards") or [])
    item_map = session.get("item_map") or {}
    reward_preview: list[dict[str, Any]] = []
    for rew in enemy.get("rewards") or []:
        if not isinstance(rew, dict):
            continue
        reward_type = str(rew.get("type") or "item").lower()
        raw_val = rew.get("num")
        if raw_val is None:
            raw_val = rew.get("value", 0)
        value = max(0.0, _to_float(raw_val, 0.0))
        if value <= 0:
            continue
        if reward_type == "experience":
            skill_id = str(rew.get("skill_id") or "")
            if not skill_id:
                rid = str(rew.get("id") or "")
                if rid.endswith("_experience"):
                    skill_id = rid[: -len("_experience")]
                elif rid:
                    skill_id = rid
            reward_preview.append(
                {
                    "type": "experience",
                    "skill_id": skill_id,
                    "skill_name": SKILL_NAME_MAP.get(skill_id, skill_id),
                    "value": value,
                }
            )
            continue
        item_id = str(rew.get("id") or "")
        if not item_id:
            continue
        reward_preview.append(
            {
                "type": "item",
                "item_id": item_id,
                "item_name": (item_map.get(item_id) or {}).get("name") or item_id,
                "value": value,
            }
        )
    logs.append(
        {
            "type": "enemy_down",
            "enemy_id": enemy["enemy_id"],
            "enemy_name": enemy["name"],
            "reward_preview": reward_preview,
        }
    )


def _add_pending_skill_exp(session: dict[str, Any], skill_id: str, amount: float) -> None:
    if not skill_id:
        return
    delta = _to_float(amount, 0.0)
    if delta <= 0:
        return
    pending = session.setdefault("pending_skill_exp", {})
    pending[skill_id] = _to_float(pending.get(skill_id), 0.0) + delta


def _apply_skill_exp_exact(ctx: PlayerContext, skill_id: str, exp_val: float) -> float:
    if not skill_id:
        return 0.0
    gain = _to_float(exp_val, 0.0)
    if gain <= 0:
        return 0.0
    sk = ctx.skills.get(skill_id)
    if sk is None:
        sk = PlayerSkill(uid=ctx.uid, skill_id=skill_id, level=1, exp=0.0)
        ctx.session.add(sk)
        ctx.skills[skill_id] = sk
    total_exp = max(0.0, float(sk.exp)) + gain
    sk.exp = total_exp
    sk.level = bisect_right(LEVEL_UP_TOTAL_EXP_REQUIREMENTS, total_exp) + 1
    return gain


def _apply_pending_wave_rewards(session: dict[str, Any]) -> dict[str, Any]:
    rewards = list(session.get("pending_rewards") or [])
    session["pending_rewards"] = []
    pending_skill_exp = dict(session.get("pending_skill_exp") or {})
    session["pending_skill_exp"] = {}
    if not rewards and not pending_skill_exp:
        return {"item_changes": [], "skill_changes": []}

    db = database.get_db()
    ctx = PlayerContext.load(db, session["uid"])

    before_inv = ctx.inventory.snapshot()
    before_skills = {skill_id: (int(obj.level), float(obj.exp)) for skill_id, obj in ctx.skills.items()}

    normalized_rewards: list[dict[str, Any]] = []
    for rew in rewards:
        if not isinstance(rew, dict):
            continue
        r = dict(rew)
        if str(r.get("type") or "item").lower() == "experience" and not r.get("skill_id"):
            rid = str(r.get("id") or "")
            if rid:
                r["skill_id"] = rid
        normalized_rewards.append(r)

    apply_rewards(normalized_rewards, ctx, skill_id=None)
    for skill_id, exp_val in pending_skill_exp.items():
        _apply_skill_exp_exact(ctx, str(skill_id), _to_float(exp_val, 0.0))
    refresh_unlocked_events(ctx, get_events_map())
    ctx.state.last_sync_time = time.time()
    ctx.save()

    after_inv = ctx.inventory.snapshot()
    after_skills = {skill_id: (int(obj.level), float(obj.exp)) for skill_id, obj in ctx.skills.items()}

    item_map = {str(item.get("id") or ""): item for item in (_load_actions().get("items") or [])}

    item_changes: list[dict[str, Any]] = []
    for item_id, state in sorted(set(before_inv) | set(after_inv)):
        b = int(before_inv.get((item_id, state), 0))
        a = int(after_inv.get((item_id, state), 0))
        if a == b:
            continue
        entry = {
            "item_id": item_id,
            "item_name": (item_map.get(item_id) or {}).get("name") or item_id,
            "delta": a - b,
            "quantity": a,
        }
        if state != 0:
            entry["state"] = state
        item_changes.append(entry)

    skill_changes: list[dict[str, Any]] = []
    for skill_id in sorted(set(before_skills) | set(after_skills)):
        b_level, b_exp = before_skills.get(skill_id, (1, 0.0))
        a_level, a_exp = after_skills.get(skill_id, (1, 0.0))
        if b_level == a_level and abs(a_exp - b_exp) < 1e-9:
            continue
        skill_changes.append(
            {
                "skill_id": skill_id,
                "before_level": b_level,
                "after_level": a_level,
                "before_exp": b_exp,
                "after_exp": a_exp,
            }
        )

    runtime = _build_player_runtime_from_ctx(ctx, item_map)
    player = session["player"]
    old_max_hp = max(1.0, _to_float(player["max_hp"], 1.0))
    old_max_mp = max(1.0, _to_float(player["max_mp"], 1.0))
    old_max_sp = max(1.0, _to_float(player["max_sp"], 1.0))

    hp_ratio = _to_float(player["hp"], old_max_hp) / old_max_hp
    mp_ratio = _to_float(player["mp"], old_max_mp) / old_max_mp
    sp_ratio = _to_float(player["sp"], old_max_sp) / old_max_sp

    player["base_stats"] = dict(runtime["stats"])
    player["stats"] = dict(runtime["stats"])
    player["skills"] = runtime["skills"]
    player["basic_skill_id"] = runtime["basic_skill_id"]
    player["name"] = runtime["name"]
    player["max_hp"] = _to_float(player["stats"].get("hp"), 100.0)
    player["max_mp"] = _to_float(player["stats"].get("mp"), 100.0)
    player["max_sp"] = _to_float(player["stats"].get("sp"), 100.0)
    player["hp"] = _clamp(player["max_hp"] * hp_ratio, 0.0, player["max_hp"])
    player["mp"] = _clamp(player["max_mp"] * mp_ratio, 0.0, player["max_mp"])
    player["sp"] = _clamp(player["max_sp"] * sp_ratio, 0.0, player["max_sp"])
    _refresh_entity_stats(player, _to_float(session.get("time"), 0.0))

    return {
        "item_changes": item_changes,
        "skill_changes": skill_changes,
        "battle_action_exp": [
            {
                "skill_id": skill_id,
                "skill_name": SKILL_NAME_MAP.get(skill_id, skill_id),
                "value": _to_float(exp_val, 0.0),
            }
            for skill_id, exp_val in sorted(pending_skill_exp.items())
            if _to_float(exp_val, 0.0) > 0
        ],
    }


def _finish_wave_if_needed(session: dict[str, Any], logs: list[dict[str, Any]]) -> None:
    alive_enemies = [e for e in session["enemies"] if e["alive"]]
    if alive_enemies:
        return

    rewards_summary = _apply_pending_wave_rewards(session)
    session["next_wave_time"] = session["time"] + max(0.1, _to_float(session["battle"].get("interval"), 3.0))
    session["wave_type"] = None

    logs.append(
        {
            "type": "wave_clear",
            "wave_number": session["wave_number"],
            "rewards": rewards_summary,
        }
    )


def _handle_player_down(session: dict[str, Any], logs: list[dict[str, Any]]) -> None:
    session["player"]["alive"] = False
    session["player"]["hp"] = 0.0

    rewards_summary = _apply_pending_wave_rewards(session)

    session["enemies"] = []
    session["wave_type"] = None
    session["next_wave_time"] = None
    session["respawn_time"] = session["time"] + 30.0

    logs.append(
        {
            "type": "player_down",
            "respawn_in": 30.0,
            "rewards": rewards_summary,
        }
    )
