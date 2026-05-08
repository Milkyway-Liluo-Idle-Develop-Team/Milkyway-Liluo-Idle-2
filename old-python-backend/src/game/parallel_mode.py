"""战斗/生产并行能力判断。"""

from typing import Any

from data.data import load_actions as _load_actions
from game.context import PlayerContext

_double_body_event_id_cache: str | None = None


def resolve_double_body_event_id(actions: dict[str, Any] | None = None) -> str:
    """定位“二重身”升级事件 id。找不到时回退到 `double_body`。"""
    global _double_body_event_id_cache
    if _double_body_event_id_cache is not None:
        return _double_body_event_id_cache

    actions_obj = actions or _load_actions()
    for event in actions_obj.get("events", []):
        if str(event.get("type") or "") != "upgrade":
            continue
        event_id = str(event.get("id") or "")
        event_name = str(event.get("name") or "")
        low_id = event_id.lower()
        if "二重身" in event_name or "double_body" in low_id or "doppel" in low_id:
            _double_body_event_id_cache = event_id or "double_body"
            return _double_body_event_id_cache

    _double_body_event_id_cache = "double_body"
    return _double_body_event_id_cache


def has_double_body_unlocked(ctx: PlayerContext, actions: dict[str, Any] | None = None) -> bool:
    event_id = resolve_double_body_event_id(actions)
    return ctx.get_event_count(event_id) >= 1

