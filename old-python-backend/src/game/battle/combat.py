"""战斗伤害计算与攻击执行。"""

from __future__ import annotations

import math
import random
from typing import Any

from game.battle.core import _to_float, _clamp
from game.battle.entity import _refresh_entity_stats
from game.battle.skills import (
    _skill_has_damage,
    _skill_requires_effect_change,
    _skill_would_apply_effect,
    _apply_skill_effects,
    _evaluate_skill_conditions,
)
from game.battle.rewards import (
    _record_enemy_death,
    _add_pending_skill_exp,
    _finish_wave_if_needed,
    _handle_player_down,
)


def _calc_damage_result(attacker: dict[str, Any], defender: dict[str, Any], skill: dict[str, Any]) -> dict[str, Any]:
    damage_cfg = skill.get("damage") or {}
    damage_type = str(damage_cfg.get("type") or "physical").lower()
    flat = _to_float(damage_cfg.get("flat"), 0.0)
    mult = _to_float(damage_cfg.get("multiplier"), 1.0)

    attacker_acc = max(1e-6, _to_float(attacker.get("accuracy"), 1.0))
    defender_evade = max(1e-6, _to_float(defender.get("evade"), 1.0))
    evade_possibility = 1.0 / (1.0 + (_clamp(attacker_acc / defender_evade, 0.0, 10.0) ** 2))
    evaded = random.random() < evade_possibility

    rand_ratio = 0.9 + random.random() * 0.2

    if damage_type == "magic":
        attack_power = max(0.0, flat + _to_float(attacker.get("magic_power"), 0.0) * mult)
        raw_damage = (
            rand_ratio
            * attack_power
            / max(1e-6, 1.0 + _to_float(defender.get("magic_instance"), 0.0))
            * (1.0 + _to_float(attacker.get("final_damage_multiplier"), 0.0))
            / max(1e-6, 1.0 + _to_float(defender.get("final_damage_reduce"), 0.0))
        )
        raw_damage = max(0.0, raw_damage)
        return {
            "damage": 0.0 if evaded else raw_damage,
            "raw_damage": raw_damage,
            "damage_type": "magic",
            "evaded": evaded,
            "blocked": False,
            "blocked_reduction": 0.0,
        }

    attack_power = max(0.0, flat + _to_float(attacker.get("physical_power"), 0.0) * mult)
    defense = max(0.0, _to_float(defender.get("defense"), 0.0))
    original_damage = (
        rand_ratio
        * (attack_power ** 2)
        / max(1e-6, attack_power + defense)
        * (1.0 + _to_float(attacker.get("final_damage_multiplier"), 0.0))
        / max(1e-6, 1.0 + _to_float(defender.get("final_damage_reduce"), 0.0))
    )
    original_damage = max(0.0, original_damage)

    if evaded:
        return {
            "damage": 0.0,
            "raw_damage": original_damage,
            "damage_type": "physical",
            "evaded": True,
            "blocked": False,
            "blocked_reduction": 0.0,
        }

    block = max(0.0, _to_float(defender.get("block"), 0.0))
    block_pos_mult = max(0.0, _to_float(defender.get("block_possibility_multiplier"), 0.0))
    block_possibility = 1.0 - 100.0 / (100.0 + block) / max(1e-6, 1.0 + block_pos_mult)
    block_possibility = _clamp(block_possibility, 0.0, 1.0)
    blocked = random.random() < block_possibility
    blocked_reduction = 0.0
    if blocked:
        block_rate = max(1.0, _to_float(defender.get("block_rate"), 1.0))
        blocked_damage = max(0.0, original_damage / block_rate)
        blocked_reduction = max(0.0, original_damage - blocked_damage)
        original_damage = blocked_damage

    if random.random() < _clamp(_to_float(attacker.get("critical"), 0.0), 0.0, 1.0):
        original_damage *= max(1.0, _to_float(attacker.get("critical_rate"), 1.0))
    original_damage = max(0.0, original_damage)

    return {
        "damage": original_damage,
        "raw_damage": original_damage + blocked_reduction,
        "damage_type": "physical",
        "evaded": False,
        "blocked": blocked,
        "blocked_reduction": blocked_reduction,
    }


def _choose_player_skill(session: dict[str, Any], target_enemy: dict[str, Any] | None) -> dict[str, Any]:
    available_skills = session["player"]["skills"]
    basic_skill_id = session["player"]["basic_skill_id"]
    now = session["time"]
    plan = session["player"]["skill_plan"]
    alive_enemies = [e for e in session["enemies"] if e["alive"]]

    for entry in plan:
        skill_id = entry["skill_id"]
        skill = available_skills.get(skill_id)
        if not skill:
            continue
        if _to_float(session["player"]["cooldowns"].get(skill_id), 0.0) > now:
            continue
        mp_cost = max(0.0, _to_float(skill.get("mp_cost"), 0.0))
        sp_cost = max(0.0, _to_float(skill.get("sp_cost"), 0.0))
        if _to_float(session["player"].get("mp"), 0.0) < mp_cost:
            continue
        if _to_float(session["player"].get("sp"), 0.0) < sp_cost:
            continue
        if not _evaluate_skill_conditions(entry.get("condition"), session["player"], target_enemy, alive_enemies):
            continue
        if (
            not _skill_has_damage(skill)
            and _skill_requires_effect_change(skill)
            and not _skill_would_apply_effect(
            skill,
            session["player"],
            target_enemy,
            now,
        )
        ):
            continue
        return skill

    return available_skills.get(basic_skill_id) or {
        "id": "__basic_attack__",
        "name": "基础攻击",
        "damage": {"type": "physical", "flat": 0.0, "multiplier": 1.0},
        "cooldown": 0.0,
        "cast_time": _to_float(session["player"]["stats"].get("attack_interval"), 2.0),
    }


def _choose_enemy_skill(
    session: dict[str, Any],
    enemy: dict[str, Any],
    target_player: dict[str, Any] | None,
) -> dict[str, Any]:
    available_skills = enemy.get("skills") or {}
    basic_skill_id = str(enemy.get("basic_skill_id") or "__enemy_basic_attack__")
    now = _to_float(session.get("time"), 0.0)
    plan = enemy.get("skill_plan") or []
    alive_allies = [e for e in session.get("enemies") or [] if isinstance(e, dict) and e.get("alive")]

    for entry in plan:
        skill_id = str(entry.get("skill_id") or "")
        if not skill_id:
            continue
        skill = available_skills.get(skill_id)
        if not skill:
            continue
        if _to_float((enemy.get("cooldowns") or {}).get(skill_id), 0.0) > now:
            continue

        mp_cost = max(0.0, _to_float(skill.get("mp_cost"), 0.0))
        sp_cost = max(0.0, _to_float(skill.get("sp_cost"), 0.0))
        if _to_float(enemy.get("mp"), 0.0) < mp_cost:
            continue
        if _to_float(enemy.get("sp"), 0.0) < sp_cost:
            continue
        if not _evaluate_skill_conditions(entry.get("condition"), enemy, target_player, alive_allies):
            continue
        if (
            not _skill_has_damage(skill)
            and _skill_requires_effect_change(skill)
            and not _skill_would_apply_effect(
            skill,
            enemy,
            target_player,
            now,
        )
        ):
            continue
        return skill

    fallback = available_skills.get(basic_skill_id)
    if isinstance(fallback, dict):
        return fallback
    return {
        "id": "__enemy_basic_attack__",
        "name": "基础攻击",
        "damage": {"type": str(enemy.get("basic_damage_type") or "physical"), "flat": 0.0, "multiplier": 1.0},
        "cooldown": 0.0,
        "cast_time": _to_float((enemy.get("stats") or {}).get("attack_interval"), 2.0),
        "effects": [],
    }


def _process_player_attack(session: dict[str, Any], logs: list[dict[str, Any]]) -> None:
    alive_enemies = [e for e in session["enemies"] if e["alive"]]
    if not alive_enemies:
        return

    player = session["player"]
    target = alive_enemies[0]
    skill = _choose_player_skill(session, target)
    mp_cost = max(0.0, _to_float(skill.get("mp_cost"), 0.0))
    sp_cost = max(0.0, _to_float(skill.get("sp_cost"), 0.0))
    if mp_cost > 0:
        player["mp"] = _clamp(_to_float(player.get("mp"), 0.0) - mp_cost, 0.0, _to_float(player.get("max_mp"), 0.0))
        _add_pending_skill_exp(session, "intelligence", 5.0 * mp_cost)
    if sp_cost > 0:
        player["sp"] = _clamp(_to_float(player.get("sp"), 0.0) - sp_cost, 0.0, _to_float(player.get("max_sp"), 0.0))
        _add_pending_skill_exp(session, "stamina", 5.0 * sp_cost)

    applied_effects = _apply_skill_effects(session, skill, player, target)

    damage_result = {
        "damage": 0.0,
        "evaded": False,
        "blocked": False,
        "damage_type": "physical",
    }
    dmg = 0.0
    if _skill_has_damage(skill):
        damage_result = _calc_damage_result(player["stats"], target["stats"], skill)
        dmg = _to_float(damage_result.get("damage"), 0.0)
        target["hp"] = _clamp(target["hp"] - dmg, 0.0, target["max_hp"])

        if dmg > 0:
            damage_type = str(damage_result.get("damage_type") or "physical").lower()
            if damage_type == "magic":
                _add_pending_skill_exp(session, "magic", 1.0 * dmg)
            else:
                physical_style = str(skill.get("physical_style") or "melee").lower()
                if physical_style == "ranged":
                    _add_pending_skill_exp(session, "ranging", 0.5 * dmg)
                    _add_pending_skill_exp(session, "strength", 0.5 * dmg)
                else:
                    _add_pending_skill_exp(session, "strength", 0.8 * dmg)
                    _add_pending_skill_exp(session, "ranging", 0.2 * dmg)

    cast_time = max(0.1, _to_float(skill.get("cast_time"), _to_float(player["stats"].get("attack_interval"), 2.0)))
    cooldown = max(0.0, _to_float(skill.get("cooldown"), 0.0))

    player["next_ready_time"] = session["time"] + cast_time
    player["last_action_duration"] = cast_time
    player["last_skill_id"] = skill.get("id")
    player["last_skill_name"] = skill.get("name") or skill.get("id")
    player["cooldowns"][skill["id"]] = session["time"] + cooldown

    logs.append(
        {
            "type": "player_attack",
            "skill_id": skill.get("id"),
            "skill_name": skill.get("name") or skill.get("id"),
            "target_id": target["enemy_id"],
            "target_instance_id": target["instance_id"],
            "damage": round(dmg, 3),
            "target_hp": round(target["hp"], 3),
            "evaded": bool(damage_result.get("evaded")),
            "blocked": bool(damage_result.get("blocked")),
            "mp_cost": round(mp_cost, 3),
            "sp_cost": round(sp_cost, 3),
            "effects": applied_effects,
        }
    )

    if target["hp"] <= 0 and target["alive"]:
        _record_enemy_death(session, target, logs)


def _process_enemy_attack(session: dict[str, Any], enemy: dict[str, Any], logs: list[dict[str, Any]]) -> None:
    if not enemy["alive"] or not session["player"]["alive"]:
        return

    player = session["player"]
    skill = _choose_enemy_skill(session, enemy, player)
    mp_cost = max(0.0, _to_float(skill.get("mp_cost"), 0.0))
    sp_cost = max(0.0, _to_float(skill.get("sp_cost"), 0.0))
    if mp_cost > 0:
        enemy["mp"] = _clamp(_to_float(enemy.get("mp"), 0.0) - mp_cost, 0.0, _to_float(enemy.get("max_mp"), 0.0))
    if sp_cost > 0:
        enemy["sp"] = _clamp(_to_float(enemy.get("sp"), 0.0) - sp_cost, 0.0, _to_float(enemy.get("max_sp"), 0.0))

    applied_effects = _apply_skill_effects(session, skill, enemy, player)

    damage_result = {
        "damage": 0.0,
        "raw_damage": 0.0,
        "blocked_reduction": 0.0,
        "evaded": False,
        "blocked": False,
    }
    dmg = 0.0
    raw_damage = 0.0
    blocked_reduction = 0.0
    evaded = False
    if _skill_has_damage(skill):
        damage_result = _calc_damage_result(enemy["stats"], player["stats"], skill)
        dmg = _to_float(damage_result.get("damage"), 0.0)
        raw_damage = _to_float(damage_result.get("raw_damage"), dmg)
        blocked_reduction = _to_float(damage_result.get("blocked_reduction"), 0.0)
        evaded = bool(damage_result.get("evaded"))
        player["hp"] = _clamp(player["hp"] - dmg, 0.0, player["max_hp"])

    cast_time = max(0.1, _to_float(skill.get("cast_time"), _to_float(enemy["stats"].get("attack_interval"), 2.0)))
    cooldown = max(0.0, _to_float(skill.get("cooldown"), 0.0))
    enemy["next_ready_time"] = session["time"] + cast_time
    enemy["last_action_duration"] = cast_time
    enemy["last_skill_id"] = skill.get("id")
    enemy["last_skill_name"] = skill.get("name") or skill.get("id")
    if not isinstance(enemy.get("cooldowns"), dict):
        enemy["cooldowns"] = {}
    enemy["cooldowns"][skill["id"]] = session["time"] + cooldown

    if evaded:
        _add_pending_skill_exp(session, "ranging", 2.0 * max(0.0, raw_damage))
    else:
        if dmg > 0:
            _add_pending_skill_exp(session, "resilience", 1.5 * dmg)
            _add_pending_skill_exp(session, "defense", 0.5 * dmg)
        if blocked_reduction > 0:
            _add_pending_skill_exp(session, "defense", 2.0 * blocked_reduction)

    logs.append(
        {
            "type": "enemy_attack",
            "enemy_id": enemy["enemy_id"],
            "enemy_instance_id": enemy["instance_id"],
            "enemy_name": enemy["name"],
            "skill_id": skill.get("id"),
            "skill_name": skill.get("name") or skill.get("id"),
            "damage": round(dmg, 3),
            "raw_damage": round(raw_damage, 3),
            "blocked_reduction": round(blocked_reduction, 3),
            "evaded": evaded,
            "blocked": bool(damage_result.get("blocked")),
            "player_hp": round(player["hp"], 3),
            "effects": applied_effects,
            "mp_cost": round(mp_cost, 3),
            "sp_cost": round(sp_cost, 3),
        }
    )

    if player["hp"] <= 0 and player["alive"]:
        _handle_player_down(session, logs)
