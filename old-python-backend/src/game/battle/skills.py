"""战斗技能系统（收集、规范化、条件评估、效果应用）。"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from game.battle.core import _to_float, _compare, INIT_BATTLE_DATA

if TYPE_CHECKING:
    from game.context import PlayerContext


def _extract_skill_cost(sk: dict[str, Any], resource: str) -> float:
    key_main = "mp_cost" if resource == "mp" else "sp_cost"
    key_alt = "mana_cost" if resource == "mp" else "stamina_cost"
    key_need = "need_mp" if resource == "mp" else "need_sp"
    key_short = "mp" if resource == "mp" else "sp"

    for key in (key_main, key_alt, key_need):
        if sk.get(key) is not None:
            return max(0.0, _to_float(sk.get(key), 0.0))

    cost_obj = sk.get("cost")
    if isinstance(cost_obj, dict):
        for key in (key_main, key_alt, key_need, key_short):
            if cost_obj.get(key) is not None:
                return max(0.0, _to_float(cost_obj.get(key), 0.0))

    return 0.0


def _extract_physical_style(sk: dict[str, Any], damage_cfg: dict[str, Any]) -> str:
    for source in (sk, damage_cfg):
        if not isinstance(source, dict):
            continue
        for key in ("physical_style", "attack_style", "range_type", "physical_type"):
            value = source.get(key)
            if value is None:
                continue
            text = str(value).strip().lower()
            if text in ("ranged", "range", "remote"):
                return "ranged"
            if text in ("melee", "close"):
                return "melee"
    return "melee"


def _normalize_skill_effect_mode(raw: Any) -> str:
    mode = str(raw or "flat").strip().lower()
    if mode in ("flat", "add", "additive", "plus", "value"):
        return "flat"
    if mode in ("multiplier", "percent", "percent_mult", "percent_multiplier", "mul"):
        return "percent_multiplier"
    return "flat"


def _normalize_skill_effects(sk: dict[str, Any]) -> list[dict[str, Any]]:
    raw_effects = sk.get("effects")
    if raw_effects is None:
        raw_effects = sk.get("stat_effects")
    if raw_effects is None:
        raw_effects = sk.get("buffs")

    if not isinstance(raw_effects, list):
        return []

    out: list[dict[str, Any]] = []
    for effect in raw_effects:
        if not isinstance(effect, dict):
            continue
        attr = str(effect.get("attribute") or effect.get("id") or effect.get("key") or "")
        if not attr:
            continue
        target = str(effect.get("target") or "self").strip().lower()
        if target not in ("self", "target"):
            target = "self"
        mode = _normalize_skill_effect_mode(effect.get("mode") or effect.get("operation") or effect.get("type"))
        value = _to_float(effect.get("value"), _to_float(effect.get("num"), 0.0))
        duration = effect.get("duration_seconds")
        if duration is None:
            duration = effect.get("duration")
        duration_seconds = _to_float(duration, 0.0)
        if duration_seconds <= 0:
            duration_seconds = None

        out.append(
            {
                "target": target,
                "attribute": attr,
                "mode": mode,
                "value": value,
                "duration_seconds": duration_seconds,
            }
        )
    return out


def _skill_has_damage(skill: dict[str, Any]) -> bool:
    damage_cfg = skill.get("damage")
    if not isinstance(damage_cfg, dict):
        return False
    flat = _to_float(damage_cfg.get("flat"), 0.0)
    mult = _to_float(damage_cfg.get("multiplier"), 1.0)
    if abs(flat) < 1e-12 and abs(mult) < 1e-12:
        return False
    return True


def _normalize_battle_skill(
    sk: dict[str, Any],
    *,
    default_cast_time: float,
    default_damage_type: str = "physical",
) -> dict[str, Any]:
    damage_cfg_raw = sk.get("damage")
    if isinstance(damage_cfg_raw, dict):
        damage_cfg = {
            "type": str(damage_cfg_raw.get("type") or default_damage_type),
            "flat": _to_float(damage_cfg_raw.get("flat"), 0.0),
            "multiplier": _to_float(damage_cfg_raw.get("multiplier"), 1.0),
        }
    else:
        damage_cfg = None
    effects = _normalize_skill_effects(sk)

    is_support = bool(sk.get("is_support")) or (
        bool(effects) and not isinstance(damage_cfg_raw, dict)
    )

    normalized = {
        "id": str(sk.get("id") or ""),
        "name": sk.get("name") or str(sk.get("id") or ""),
        "description": sk.get("description") or "",
        "target_type": sk.get("target_type") or "single",
        "damage": None if is_support else damage_cfg,
        "cooldown": _to_float(sk.get("cooldown"), 0.0),
        "cast_time": _to_float(sk.get("cast_time"), default_cast_time),
        "mp_cost": _extract_skill_cost(sk, "mp"),
        "sp_cost": _extract_skill_cost(sk, "sp"),
        "physical_style": _extract_physical_style(sk, damage_cfg or {"type": default_damage_type}),
        "is_basic": bool(sk.get("is_basic")),
        "is_support": is_support,
        "effects": effects,
    }
    return normalized


def _collect_player_battle_skills(
    ctx: PlayerContext,
    item_map: dict[str, dict[str, Any]],
    attack_interval: float,
) -> tuple[dict[str, dict[str, Any]], str]:
    skills: dict[str, dict[str, Any]] = {}
    basic_skill_id: str | None = None

    seen: set[tuple[str, str]] = set()
    for slot_id, row in ctx.equipment.items():
        anchor = str(getattr(row, "anchor_slot", None) or slot_id)
        piece_key = (anchor, str(row.item_id))
        if piece_key in seen:
            continue
        seen.add(piece_key)

        item = item_map.get(str(row.item_id)) or {}
        details = item.get("equipment_details") or {}
        for sk in details.get("battle_skills") or []:
            normalized = _normalize_battle_skill(
                sk if isinstance(sk, dict) else {},
                default_cast_time=attack_interval,
                default_damage_type="physical",
            )
            skill_id = str(normalized.get("id") or "")
            if not skill_id:
                continue
            skills[skill_id] = normalized
            if normalized["is_basic"] and basic_skill_id is None:
                basic_skill_id = skill_id

    if basic_skill_id is None:
        basic_skill_id = "__basic_attack__"
        skills[basic_skill_id] = {
            "id": basic_skill_id,
            "name": "基础攻击",
            "description": "默认基础攻击",
            "target_type": "single",
            "damage": {"type": "physical", "flat": 0.0, "multiplier": 1.0},
            "cooldown": 0.0,
            "cast_time": attack_interval,
            "mp_cost": 0.0,
            "sp_cost": 0.0,
            "physical_style": "melee",
            "is_basic": True,
        }

    return skills, basic_skill_id


def _collect_enemy_battle_skills(enemy_def: dict[str, Any], attack_interval: float) -> tuple[dict[str, dict[str, Any]], str]:
    skills: dict[str, dict[str, Any]] = {}
    basic_skill_id: str | None = None
    default_damage_type = str(enemy_def.get("basic_damage_type") or "physical")

    raw_plan = enemy_def.get("battle_skill")
    if raw_plan is None:
        raw_plan = enemy_def.get("battle_skills")
    if not isinstance(raw_plan, list):
        raw_plan = []

    for entry in raw_plan:
        if not isinstance(entry, dict):
            continue
        raw_skill = entry.get("battle_skill")
        if not isinstance(raw_skill, dict):
            raw_skill = entry
        normalized = _normalize_battle_skill(
            raw_skill,
            default_cast_time=attack_interval,
            default_damage_type=default_damage_type,
        )
        skill_id = str(normalized.get("id") or "")
        if not skill_id:
            continue
        skills[skill_id] = normalized
        if normalized.get("is_basic") and basic_skill_id is None:
            basic_skill_id = skill_id

    if basic_skill_id is None:
        basic_skill_id = "__enemy_basic_attack__"
        skills[basic_skill_id] = {
            "id": basic_skill_id,
            "name": "基础攻击",
            "description": "默认基础攻击",
            "target_type": "single",
            "damage": {"type": default_damage_type, "flat": 0.0, "multiplier": 1.0},
            "cooldown": 0.0,
            "cast_time": attack_interval,
            "mp_cost": 0.0,
            "sp_cost": 0.0,
            "physical_style": "melee",
            "is_basic": True,
            "is_support": False,
            "effects": [],
        }

    return skills, basic_skill_id


def _sanitize_skill_plan(raw: Any) -> list[dict[str, Any]]:
    if not isinstance(raw, list):
        return []
    out: list[dict[str, Any]] = []
    for entry in raw:
        if not isinstance(entry, dict):
            continue
        skill_id = _extract_skill_id(entry)
        if not skill_id:
            continue
        out.append(
            {
                "skill_id": skill_id,
                "priority": int(_to_float(entry.get("priority"), 0.0)),
                "condition": entry.get("condition") if isinstance(entry.get("condition"), dict) else None,
            }
        )
    out.sort(key=lambda x: x["priority"], reverse=True)
    return out


def _extract_skill_id(config_entry: dict[str, Any]) -> str:
    if "skill_id" in config_entry:
        return str(config_entry.get("skill_id") or "")
    battle_skill = config_entry.get("battle_skill")
    if isinstance(battle_skill, dict):
        return str(battle_skill.get("id") or "")
    return str(config_entry.get("id") or "")


def _sanitize_player_skill_plan(raw: Any) -> list[dict[str, Any]]:
    return _sanitize_skill_plan(raw)


def _effect_will_change_target(target: dict[str, Any], effect: dict[str, Any], now: float, source_skill_id: str) -> bool:
    attr = str(effect.get("attribute") or "")
    if not attr:
        return False

    mode = str(effect.get("mode") or "flat")
    value = _to_float(effect.get("value"), 0.0)
    duration_seconds = effect.get("duration_seconds")
    if duration_seconds is None:
        expires_at = None
    else:
        expires_at = now + max(0.0, _to_float(duration_seconds, 0.0))

    active_effects = target.get("active_effects")
    if not isinstance(active_effects, list):
        return True

    for ex in active_effects:
        if not isinstance(ex, dict):
            continue
        if str(ex.get("source_skill_id") or "") != source_skill_id:
            continue
        if str(ex.get("attribute") or "") != attr:
            continue
        if str(ex.get("mode") or "flat") != mode:
            continue
        if abs(_to_float(ex.get("value"), 0.0) - value) > 1e-9:
            return True
        ex_exp = ex.get("expires_at")
        if ex_exp is None and expires_at is None:
            return False
        if ex_exp is None or expires_at is None:
            return True
        return _to_float(ex_exp, 0.0) + 1e-6 < _to_float(expires_at, 0.0)

    return True


def _skill_would_apply_effect(skill: dict[str, Any], caster: dict[str, Any], target: dict[str, Any] | None, now: float) -> bool:
    effects = skill.get("effects")
    if not isinstance(effects, list) or not effects:
        return False
    source_skill_id = str(skill.get("id") or "")
    for effect in effects:
        if not isinstance(effect, dict):
            continue
        effect_target = str(effect.get("target") or "self").lower()
        dst = caster if effect_target != "target" else target
        if dst is None:
            continue
        if _effect_will_change_target(dst, effect, now, source_skill_id):
            return True
    return False


def _skill_requires_effect_change(skill: dict[str, Any]) -> bool:
    return bool(skill.get("require_effect_change"))


def _apply_skill_effects(
    session: dict[str, Any],
    skill: dict[str, Any],
    caster: dict[str, Any],
    target: dict[str, Any] | None,
) -> list[dict[str, Any]]:
    effects = skill.get("effects")
    if not isinstance(effects, list) or not effects:
        return []

    applied: list[dict[str, Any]] = []
    now = _to_float(session.get("time"), 0.0)
    source_skill_id = str(skill.get("id") or "")
    for effect in effects:
        if not isinstance(effect, dict):
            continue
        effect_target = str(effect.get("target") or "self").lower()
        dst = caster if effect_target != "target" else target
        if dst is None:
            continue

        attr = str(effect.get("attribute") or "")
        mode = str(effect.get("mode") or "flat")
        value = _to_float(effect.get("value"), 0.0)
        if not attr:
            continue

        duration_seconds = effect.get("duration_seconds")
        expires_at = None
        if duration_seconds is not None:
            expires_at = now + max(0.0, _to_float(duration_seconds, 0.0))

        active_effects = dst.get("active_effects")
        if not isinstance(active_effects, list):
            active_effects = []
            dst["active_effects"] = active_effects

        replaced = False
        for idx, ex in enumerate(active_effects):
            if not isinstance(ex, dict):
                continue
            if str(ex.get("source_skill_id") or "") != source_skill_id:
                continue
            if str(ex.get("attribute") or "") != attr:
                continue
            if str(ex.get("mode") or "flat") != mode:
                continue
            active_effects[idx] = {
                "source_skill_id": source_skill_id,
                "attribute": attr,
                "mode": mode,
                "value": value,
                "expires_at": expires_at,
            }
            replaced = True
            break
        if not replaced:
            active_effects.append(
                {
                    "source_skill_id": source_skill_id,
                    "attribute": attr,
                    "mode": mode,
                    "value": value,
                    "expires_at": expires_at,
                }
            )

        _refresh_entity_stats(dst, now)
        applied.append(
            {
                "target": "self" if dst is caster else "target",
                "attribute": attr,
                "mode": mode,
                "value": value,
                "duration_seconds": duration_seconds,
            }
        )

    return applied


def _single_condition_match(
    cond: dict[str, Any],
    player: dict[str, Any],
    target: dict[str, Any] | None,
    alive_enemies: list[dict[str, Any]],
) -> bool:
    key = str(cond.get("key") or "")
    comparison = cond.get("comparison_type")
    value = _to_float(cond.get("value"), 0.0)

    def _hp_ratio(entity: dict[str, Any]) -> float:
        return _to_float(entity.get("hp"), 0.0) / max(1.0, _to_float(entity.get("max_hp"), 1.0))

    if key == "self_hp":
        return _compare(_to_float(player.get("hp"), 0.0), value, comparison)
    if key == "self_hp_ratio":
        return _compare(_hp_ratio(player), value, comparison)
    if key == "self_mp":
        return _compare(_to_float(player.get("mp"), 0.0), value, comparison)
    if key == "self_mp_ratio":
        return _compare(_to_float(player.get("mp"), 0.0) / max(1.0, _to_float(player.get("max_mp"), 1.0)), value, comparison)
    if key == "self_sp":
        return _compare(_to_float(player.get("sp"), 0.0), value, comparison)
    if key == "self_sp_ratio":
        return _compare(_to_float(player.get("sp"), 0.0) / max(1.0, _to_float(player.get("max_sp"), 1.0)), value, comparison)

    if key == "target_hp" and target is not None:
        return _compare(_to_float(target.get("hp"), 0.0), value, comparison)
    if key == "target_hp_ratio" and target is not None:
        return _compare(_hp_ratio(target), value, comparison)

    if key == "any_enemy_hp":
        for enemy in alive_enemies:
            if _compare(_to_float(enemy.get("hp"), 0.0), value, comparison):
                return True
        return False
    if key == "any_enemy_hp_ratio":
        for enemy in alive_enemies:
            if _compare(_hp_ratio(enemy), value, comparison):
                return True
        return False

    return True


def _evaluate_skill_conditions(
    conds: dict[str, Any] | None,
    player: dict[str, Any],
    target: dict[str, Any] | None,
    alive_enemies: list[dict[str, Any]],
) -> bool:
    if not conds:
        return True

    logic = str(conds.get("logic_type") or "and").lower()
    pieces: list[bool] = []

    normal_conds = conds.get("normal_condition")
    if isinstance(normal_conds, list) and normal_conds:
        pieces.append(all(_single_condition_match(c, player, target, alive_enemies) for c in normal_conds if isinstance(c, dict)))

    complex_cond = conds.get("complex_condition")
    if isinstance(complex_cond, dict):
        pieces.append(_evaluate_skill_conditions(complex_cond, player, target, alive_enemies))

    if not pieces:
        return True
    if logic == "or":
        return any(pieces)
    if logic == "nor":
        return not any(pieces)
    return all(pieces)
