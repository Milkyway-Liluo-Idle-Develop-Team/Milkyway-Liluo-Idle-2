"""战斗运行时主循环与公共接口。"""

from __future__ import annotations

import json
import time
from typing import Any

from models import database
from game import parallel_mode
from game.context import PlayerContext
from game.queue import set_queue
from game.settlement import settle_player
from game.battle.core import _to_float, _clamp
from data.data import load_actions as _load_actions
from game.battle.entity import _advance_time
from game.battle.session import _sessions, _persist_session, _load_session, _ensure_session_runtime_shape
from game.battle.skills import _sanitize_player_skill_plan
from game.battle.rewards import (
    _spawn_wave,
    _finish_wave_if_needed,
    _handle_player_down,
    _build_player_runtime_from_ctx,
    _apply_pending_wave_rewards,
)
from game.battle.combat import _process_player_attack, _process_enemy_attack


def list_battles(map_id: str | None = None) -> list[dict[str, Any]]:
    actions = _load_actions()
    out: list[dict[str, Any]] = []
    for battle in actions.get("battles", []):
        if map_id and str(battle.get("map") or "") != map_id:
            continue
        out.append(
            {
                "id": str(battle.get("id") or ""),
                "name": battle.get("name") or str(battle.get("id") or ""),
                "map": str(battle.get("map") or "unknown"),
                "interval": _to_float(battle.get("interval"), 3.0),
            }
        )
    return out


def _next_event_time(session: dict[str, Any]) -> float | None:
    candidates: list[float] = []
    if session.get("respawn_time") is not None:
        candidates.append(_to_float(session["respawn_time"], 0.0))
    if session.get("next_wave_time") is not None:
        candidates.append(_to_float(session["next_wave_time"], 0.0))

    alive_enemies = [e for e in session["enemies"] if e["alive"]]
    if session["player"]["alive"] and alive_enemies:
        candidates.append(_to_float(session["player"]["next_ready_time"], 0.0))
        for enemy in alive_enemies:
            candidates.append(_to_float(enemy["next_ready_time"], 0.0))

    if not candidates:
        return None
    return min(candidates)


def _advance_one_event(session: dict[str, Any]) -> list[dict[str, Any]]:
    logs: list[dict[str, Any]] = []
    if not session.get("running"):
        return logs

    event_time = _next_event_time(session)
    if event_time is None:
        session["running"] = False
        logs.append({"type": "stopped"})
        return logs

    _advance_time(session, event_time)

    if session.get("respawn_time") is not None and session["time"] >= session["respawn_time"]:
        session["respawn_time"] = None
        player = session["player"]
        player["alive"] = True
        player["hp"] = player["max_hp"]
        player["mp"] = player["max_mp"]
        player["sp"] = player["max_sp"]
        player["next_ready_time"] = session["time"] + max(0.1, _to_float(player["stats"].get("attack_interval"), 2.0))
        session["next_wave_time"] = session["time"] + max(0.1, _to_float(session["battle"].get("interval"), 3.0))
        logs.append({"type": "player_respawn", "next_wave_in": _to_float(session["battle"].get("interval"), 3.0)})

    if session.get("next_wave_time") is not None and session["time"] >= session["next_wave_time"] and session["player"]["alive"]:
        _spawn_wave(session, logs)

    alive_enemies = [e for e in session["enemies"] if e["alive"]]
    if (
        alive_enemies
        and session["player"]["alive"]
        and session["time"] >= _to_float(session["player"].get("next_ready_time"), 0.0)
    ):
        _process_player_attack(session, logs)

    alive_enemies = [e for e in session["enemies"] if e["alive"]]
    if alive_enemies and session["player"]["alive"]:
        for enemy in alive_enemies:
            if session["time"] >= _to_float(enemy.get("next_ready_time"), 0.0):
                _process_enemy_attack(session, enemy, logs)
                if not session["player"]["alive"]:
                    break

    if session["player"]["alive"]:
        _finish_wave_if_needed(session, logs)

    return logs


def _catch_up_session(session: dict[str, Any], now_wall: float | None = None) -> list[dict[str, Any]]:
    if now_wall is None:
        now_wall = time.time()

    last_wall = _to_float(session.get("last_wall_time"), now_wall)
    if now_wall <= last_wall:
        return []

    if not session.get("running"):
        session["last_wall_time"] = now_wall
        return []

    target_time = _to_float(session.get("time"), 0.0) + (now_wall - last_wall)
    logs: list[dict[str, Any]] = []

    while session.get("running"):
        next_time = _next_event_time(session)
        if next_time is None:
            session["running"] = False
            logs.append({"type": "stopped"})
            break
        if next_time > target_time:
            break
        logs.extend(_advance_one_event(session))

    if session.get("running"):
        _advance_time(session, target_time)
    session["last_wall_time"] = now_wall
    return logs


def _snapshot(session: dict[str, Any], logs: list[dict[str, Any]] | None = None) -> dict[str, Any]:
    next_time = _next_event_time(session)
    next_step_in = None if next_time is None else max(0.0, next_time - session["time"])

    if not session.get("running"):
        status = "stopped"
    elif session.get("respawn_time") is not None:
        status = "respawn"
    elif session.get("next_wave_time") is not None and not any(e["alive"] for e in session["enemies"]):
        status = "between_waves"
    else:
        status = "fighting"

    player = session["player"]
    player_ready_in = max(0.0, _to_float(player.get("next_ready_time"), 0.0) - _to_float(session.get("time"), 0.0))
    player_total_cd = max(0.1, _to_float(player.get("last_action_duration"), _to_float((player.get("stats") or {}).get("attack_interval"), 2.0)))
    player_cd_progress = _clamp(1.0 - player_ready_in / player_total_cd, 0.0, 1.0)
    return {
        "battle_id": session["battle"]["id"],
        "battle_name": session["battle"]["name"],
        "map": session["battle"]["map"],
        "status": status,
        "time": round(_to_float(session["time"], 0.0), 3),
        "wave_number": int(session["wave_number"]),
        "wave_type": session.get("wave_type"),
        "next_step_in_seconds": next_step_in,
        "player": {
            "name": player["name"],
            "alive": bool(player["alive"]),
            "hp": round(_to_float(player["hp"], 0.0), 3),
            "max_hp": round(_to_float(player["max_hp"], 1.0), 3),
            "mp": round(_to_float(player["mp"], 0.0), 3),
            "max_mp": round(_to_float(player["max_mp"], 1.0), 3),
            "sp": round(_to_float(player["sp"], 0.0), 3),
            "max_sp": round(_to_float(player["max_sp"], 1.0), 3),
            "next_ready_in_seconds": round(player_ready_in, 3),
            "action_cooldown_seconds": round(player_total_cd, 3),
            "action_cooldown_progress": round(player_cd_progress, 6),
            "last_skill_id": str(player.get("last_skill_id") or ""),
            "last_skill_name": str(player.get("last_skill_name") or player.get("last_skill_id") or ""),
        },
        "enemies": [
            {
                "instance_id": e["instance_id"],
                "enemy_id": e["enemy_id"],
                "name": e["name"],
                "alive": bool(e["alive"]),
                "hp": round(_to_float(e["hp"], 0.0), 3),
                "max_hp": round(_to_float(e["max_hp"], 1.0), 3),
                "next_ready_in_seconds": round(
                    max(0.0, _to_float(e.get("next_ready_time"), 0.0) - _to_float(session.get("time"), 0.0)),
                    3,
                ),
                "action_cooldown_seconds": round(
                    max(0.1, _to_float(e.get("last_action_duration"), _to_float((e.get("stats") or {}).get("attack_interval"), 2.0))),
                    3,
                ),
                "action_cooldown_progress": round(
                    _clamp(
                        1.0
                        - max(0.0, _to_float(e.get("next_ready_time"), 0.0) - _to_float(session.get("time"), 0.0))
                        / max(0.1, _to_float(e.get("last_action_duration"), _to_float((e.get("stats") or {}).get("attack_interval"), 2.0))),
                        0.0,
                        1.0,
                    ),
                    6,
                ),
                "last_skill_id": str(e.get("last_skill_id") or ""),
                "last_skill_name": str(e.get("last_skill_name") or e.get("last_skill_id") or ""),
            }
            for e in session["enemies"]
        ],
        "logs": logs or [],
    }


@database.player_atomic
def is_battle_running(uid: int) -> bool:
    session = _sessions.get(uid)
    if not session:
        session = _load_session(uid)
        if session is None:
            return False
    _ensure_session_runtime_shape(session)
    _catch_up_session(session)
    _persist_session(session)
    return bool(session.get("running"))


@database.player_atomic
def start_battle(uid: int, battle_id: str, player_skills: Any = None) -> dict[str, Any]:
    result = settle_player(uid)
    ctx = result.ctx

    actions = _load_actions()
    battle_map = {str(b.get("id") or ""): b for b in actions.get("battles", [])}
    battle = battle_map.get(battle_id)
    if battle is None:
        raise ValueError(f"Invalid battle id: {battle_id}")

    enemy_map = {str(e.get("id") or ""): e for e in actions.get("enemies", [])}
    item_map = {str(i.get("id") or ""): i for i in actions.get("items", [])}

    if not parallel_mode.has_double_body_unlocked(ctx, actions):
        queue = json.loads(ctx.state.queue_json) if ctx.state.queue_json else []
        if queue:
            # 未解锁二重身时，开战自动中断生产队列。
            set_queue(uid, [])
            db = database.get_db()
            ctx = PlayerContext.load(db, uid)

    runtime = _build_player_runtime_from_ctx(ctx, item_map)

    player_stats = runtime["stats"]
    interval = max(0.1, _to_float(battle.get("interval"), 3.0))
    player = {
        "name": runtime["name"],
        "alive": True,
        "stats": dict(player_stats),
        "base_stats": dict(player_stats),
        "hp": max(1.0, _to_float(player_stats.get("hp"), 100.0)),
        "max_hp": max(1.0, _to_float(player_stats.get("hp"), 100.0)),
        "mp": max(0.0, _to_float(player_stats.get("mp"), 100.0)),
        "max_mp": max(0.0, _to_float(player_stats.get("mp"), 100.0)),
        "sp": max(0.0, _to_float(player_stats.get("sp"), 100.0)),
        "max_sp": max(0.0, _to_float(player_stats.get("sp"), 100.0)),
        "next_ready_time": interval + max(0.1, _to_float(player_stats.get("attack_interval"), 2.0)),
        "last_action_duration": max(0.1, _to_float(player_stats.get("attack_interval"), 2.0)),
        "last_skill_id": runtime["basic_skill_id"],
        "last_skill_name": (runtime["skills"].get(runtime["basic_skill_id"]) or {}).get("name") or runtime["basic_skill_id"],
        "skills": runtime["skills"],
        "basic_skill_id": runtime["basic_skill_id"],
        "skill_plan": _sanitize_player_skill_plan(player_skills),
        "cooldowns": {},
        "active_effects": [],
    }

    session = {
        "uid": uid,
        "running": True,
        "time": 0.0,
        "battle": {
            "id": battle_id,
            "name": battle.get("name") or battle_id,
            "map": battle.get("map") or "unknown",
            "interval": interval,
            "combination_loop": [str(x) for x in (battle.get("combination_loop") or ["weak"])],
            "combinations": {
                "weak": list(battle.get("weak_enemy_combinations") or []),
                "strong": list(battle.get("strong_enemy_combinations") or []),
                "boss": list(battle.get("boss_enemy_combinations") or []),
            },
        },
        "enemy_map": enemy_map,
        "item_map": item_map,
        "player": player,
        "wave_number": 0,
        "wave_type": None,
        "enemies": [],
        "pending_rewards": [],
        "pending_skill_exp": {},
        "next_wave_time": interval,
        "respawn_time": None,
        "last_wall_time": time.time(),
    }

    _ensure_session_runtime_shape(session)
    _sessions[uid] = session
    _persist_session(session)
    return _snapshot(session, logs=[{"type": "battle_started", "battle_id": battle_id}])


@database.player_atomic
def get_battle_state(uid: int) -> dict[str, Any] | None:
    session = _sessions.get(uid)
    if session is None:
        session = _load_session(uid)
        if session is None:
            return None
    _ensure_session_runtime_shape(session)
    logs = _catch_up_session(session)
    _persist_session(session)
    return _snapshot(session, logs=logs)


@database.player_atomic
def stop_battle(uid: int) -> dict[str, Any]:
    session = _sessions.get(uid)
    if session is None:
        session = _load_session(uid)
        if session is None:
            raise ValueError("No active battle session")
    _ensure_session_runtime_shape(session)
    _catch_up_session(session)
    session["running"] = False
    session["last_wall_time"] = time.time()
    _persist_session(session)
    return _snapshot(session, logs=[{"type": "battle_stopped"}])


@database.player_atomic
def step_battle(uid: int) -> dict[str, Any]:
    session = _sessions.get(uid)
    if session is None:
        session = _load_session(uid)
        if session is None:
            raise ValueError("No active battle session")
    _ensure_session_runtime_shape(session)
    if not session.get("running"):
        return _snapshot(session, logs=[{"type": "battle_stopped"}])

    logs = _catch_up_session(session)
    if not logs and session.get("running"):
        next_time = _next_event_time(session)
        if next_time is not None and next_time <= _to_float(session.get("time"), 0.0) + 1e-6:
            logs = _advance_one_event(session)
    _persist_session(session)
    return _snapshot(session, logs=logs)
