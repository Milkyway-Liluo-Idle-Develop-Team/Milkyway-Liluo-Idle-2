"""战斗实体状态管理（属性刷新、自然恢复、时间推进）。"""

from __future__ import annotations

from typing import Any

from game.battle.core import _to_float, _clamp


def _refresh_entity_stats(entity: dict[str, Any], now: float) -> None:
    active_effects = entity.get("active_effects")
    if not isinstance(active_effects, list):
        active_effects = []

    kept_effects: list[dict[str, Any]] = []
    for effect in active_effects:
        if not isinstance(effect, dict):
            continue
        expires_at = effect.get("expires_at")
        if expires_at is not None and _to_float(expires_at, 0.0) <= now:
            continue
        kept_effects.append(effect)
    entity["active_effects"] = kept_effects

    base_stats = dict(entity.get("base_stats") or entity.get("stats") or {})
    flat_bonus: dict[str, float] = {}
    mult_bonus: dict[str, float] = {}

    for effect in kept_effects:
        attr = str(effect.get("attribute") or "")
        if not attr:
            continue
        mode = str(effect.get("mode") or "flat")
        value = _to_float(effect.get("value"), 0.0)
        if mode == "percent_multiplier":
            mult_bonus[attr] = mult_bonus.get(attr, 1.0) * (1.0 + value)
        else:
            flat_bonus[attr] = flat_bonus.get(attr, 0.0) + value

    new_stats: dict[str, float] = {}
    for attr in set(base_stats.keys()) | set(flat_bonus.keys()) | set(mult_bonus.keys()):
        val = _to_float(base_stats.get(attr), 0.0) + flat_bonus.get(attr, 0.0)
        val *= mult_bonus.get(attr, 1.0)
        new_stats[attr] = val
    entity["stats"] = new_stats

    for key in ("hp", "mp", "sp"):
        max_key = f"max_{key}"
        old_max = max(1e-6, _to_float(entity.get(max_key), _to_float(new_stats.get(key), 0.0)))
        old_cur = _to_float(entity.get(key), 0.0)
        ratio = old_cur / old_max
        new_max = max(0.0, _to_float(new_stats.get(key), _to_float(entity.get(max_key), 0.0)))
        entity[max_key] = new_max
        if new_max <= 0:
            entity[key] = 0.0
        else:
            entity[key] = _clamp(new_max * ratio, 0.0, new_max)


def _apply_entity_natural_recovery(entity: dict[str, Any], elapsed_seconds: float) -> None:
    if elapsed_seconds <= 0:
        return
    stats = entity.get("stats") or {}

    max_hp = max(0.0, _to_float(entity.get("max_hp"), 0.0))
    max_mp = max(0.0, _to_float(entity.get("max_mp"), 0.0))
    max_sp = max(0.0, _to_float(entity.get("max_sp"), 0.0))

    hp_now = _to_float(entity.get("hp"), 0.0)
    mp_now = _to_float(entity.get("mp"), 0.0)
    sp_now = _to_float(entity.get("sp"), 0.0)

    hp_recovery = _to_float(stats.get("hp_recovery"), 0.0)
    mp_recovery = _to_float(stats.get("mp_recovery"), 0.0)
    sp_recovery = _to_float(stats.get("sp_recovery"), 0.0)

    entity["hp"] = _clamp(hp_now + hp_recovery * elapsed_seconds, 0.0, max_hp)
    if "mp" in entity or max_mp > 0:
        entity["mp"] = _clamp(mp_now + mp_recovery * elapsed_seconds, 0.0, max_mp)
    if "sp" in entity or max_sp > 0:
        entity["sp"] = _clamp(sp_now + sp_recovery * elapsed_seconds, 0.0, max_sp)


def _apply_natural_recovery(session: dict[str, Any], elapsed_seconds: float) -> None:
    if elapsed_seconds <= 0:
        return

    player = session.get("player") or {}
    if player.get("alive"):
        _apply_entity_natural_recovery(player, elapsed_seconds)

    for enemy in session.get("enemies") or []:
        if enemy.get("alive"):
            _apply_entity_natural_recovery(enemy, elapsed_seconds)


def _advance_time(session: dict[str, Any], target_time: float) -> None:
    now_time = _to_float(session.get("time"), 0.0)
    if target_time <= now_time:
        return
    elapsed = target_time - now_time
    _apply_natural_recovery(session, elapsed)
    session["time"] = target_time

    player = session.get("player")
    if isinstance(player, dict):
        _refresh_entity_stats(player, target_time)
    for enemy in session.get("enemies") or []:
        if isinstance(enemy, dict):
            _refresh_entity_stats(enemy, target_time)
