"""战斗 Session 管理（内存缓存、持久化、运行时 shape 修复）。"""

from __future__ import annotations

import json
import time
from typing import Any

from sqlalchemy import select

from models import database
from models import ActiveBattle, PlayerState
from game.battle.core import _to_float
from game.battle.entity import _refresh_entity_stats
from game.battle.skills import _collect_enemy_battle_skills

# uid -> active battle session (runtime cache)
_sessions: dict[int, dict[str, Any]] = {}


def remove_session(uid: int) -> None:
    """移除缓存的战斗 session，例如 WebSocket 断开时清理。"""
    _sessions.pop(uid, None)


def get_active_battle(session, uid: int) -> ActiveBattle | None:
    return session.execute(
        select(ActiveBattle).where(ActiveBattle.uid == uid)
    ).scalar_one_or_none()


def set_active_battle(session, uid: int, battle_id: str, session_json: str) -> None:
    row = get_active_battle(session, uid)
    if row is None:
        row = ActiveBattle(uid=uid, battle_id=battle_id, session_json=session_json, updated_at=time.time())
        session.add(row)
    else:
        row.battle_id = battle_id
        row.session_json = session_json
        row.updated_at = time.time()
    session.commit()


def delete_active_battle(session, uid: int) -> None:
    row = get_active_battle(session, uid)
    if row:
        session.delete(row)
        session.commit()


def update_player_active_battle_id(session, uid: int, battle_id: str | None) -> None:
    state = session.execute(
        select(PlayerState).where(PlayerState.uid == uid)
    ).scalar_one_or_none()
    if state is not None:
        state.active_battle_id = battle_id
        session.commit()


def _ensure_entity_runtime_shape(entity: dict[str, Any]) -> None:
    if not isinstance(entity.get("stats"), dict):
        entity["stats"] = {}
    if not isinstance(entity.get("base_stats"), dict):
        entity["base_stats"] = dict(entity.get("stats") or {})
    if not isinstance(entity.get("active_effects"), list):
        entity["active_effects"] = []
    if not isinstance(entity.get("cooldowns"), dict):
        entity["cooldowns"] = {}
    if entity.get("last_action_duration") is None:
        entity["last_action_duration"] = max(0.1, _to_float((entity.get("stats") or {}).get("attack_interval"), 2.0))
    _refresh_entity_stats(entity, _to_float(entity.get("last_ready_refresh_time"), 0.0))


def _ensure_session_runtime_shape(session: dict[str, Any]) -> None:
    player = session.get("player")
    if isinstance(player, dict):
        _ensure_entity_runtime_shape(player)
        if not isinstance(player.get("skills"), dict):
            player["skills"] = {}
        if not player.get("basic_skill_id"):
            player["basic_skill_id"] = "__basic_attack__"
        if not isinstance(player.get("skill_plan"), list):
            player["skill_plan"] = []
    enemies = session.get("enemies")
    if not isinstance(enemies, list):
        session["enemies"] = []
        return
    for enemy in enemies:
        if not isinstance(enemy, dict):
            continue
        _ensure_entity_runtime_shape(enemy)
        enemy_id = str(enemy.get("enemy_id") or "")
        enemy_map = session.get("enemy_map") or {}
        enemy_def = enemy_map.get(enemy_id) if isinstance(enemy_map, dict) else None
        if not isinstance(enemy.get("skills"), dict) or not enemy.get("skills"):
            attack_interval = max(0.1, _to_float((enemy.get("stats") or {}).get("attack_interval"), 2.0))
            if isinstance(enemy_def, dict):
                skills, basic_skill_id = _collect_enemy_battle_skills(enemy_def, attack_interval)
                enemy["skills"] = skills
                enemy["basic_skill_id"] = basic_skill_id
            else:
                enemy["skills"] = {}
        if not isinstance(enemy.get("skill_plan"), list):
            raw_skill_plan = enemy_def.get("battle_skill") if isinstance(enemy_def, dict) else []
            enemy["skill_plan"] = _sanitize_skill_plan(raw_skill_plan)
        if not enemy.get("basic_skill_id"):
            enemy["basic_skill_id"] = "__enemy_basic_attack__"


def _persist_session(session: dict[str, Any]) -> None:
    """将战斗 session 持久化到 SQLite，并同步 player_state.active_battle_id。
    NOTE: 使用独立 Session 提交，与 player_atomic 的事务分离。如果外层
    player_atomic 回滚，此处的战斗数据已提交不会回滚。
    """
    sess = database.SessionLocal()
    try:
        uid = session["uid"]
        set_active_battle(
            sess, uid, session["battle"]["id"], json.dumps(session, ensure_ascii=False)
        )
        update_player_active_battle_id(
            sess, uid, session["battle"]["id"] if session.get("running") else None
        )
    finally:
        sess.close()


def _load_session(uid: int) -> dict[str, Any] | None:
    """从 SQLite 加载战斗 session 到内存缓存，不存在则返回 None。"""
    sess = database.SessionLocal()
    try:
        row = get_active_battle(sess, uid)
        if row is None:
            return None
        session = json.loads(row.session_json)
        _ensure_session_runtime_shape(session)
        _sessions[uid] = session
        return session
    finally:
        sess.close()


# Re-export for backward compatibility in tests
from game.battle.skills import _sanitize_skill_plan as _sanitize_skill_plan
