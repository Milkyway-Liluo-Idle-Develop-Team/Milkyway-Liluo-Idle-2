import json
import time

from models import database
from game.context import register_dirty_keys, get_player_state
from game.settlement import settle_player, get_events_map


# ---------------------------------------------------------------------------
# Queue item format
# ---------------------------------------------------------------------------
# New format: list of dicts: {"event_id": str, "iterations": int | None, "completed": int}
# Old format: list of strings (auto-migrated on load)
# iterations = None or 0 means unlimited
# ---------------------------------------------------------------------------

def _normalize_queue_item(item):
    """Convert old string format or partial dict to canonical dict."""
    if isinstance(item, str):
        return {"event_id": item, "iterations": None, "completed": 0}
    if isinstance(item, dict):
        return {
            "event_id": str(item.get("event_id") or ""),
            "iterations": item.get("iterations") if item.get("iterations") not in (None, 0) else None,
            "completed": int(item.get("completed") or 0),
        }
    return {"event_id": str(item), "iterations": None, "completed": 0}


def _normalize_queue(queue):
    """Ensure queue is a list of canonical dict items."""
    if not isinstance(queue, list):
        return []
    return [_normalize_queue_item(i) for i in queue]


def _item_to_raw(item):
    """Canonical dict to raw storage (strip None iterations for compactness)."""
    raw = {"event_id": item["event_id"], "completed": item["completed"]}
    if item["iterations"] is not None:
        raw["iterations"] = item["iterations"]
    return raw


# ---------------------------------------------------------------------------
# Core helpers
# ---------------------------------------------------------------------------
def _load_state_queue(session, uid: int) -> tuple[list[dict], int, float]:
    state = get_player_state(session, uid)
    if state is None:
        raise ValueError(f"Player state not found for uid: {uid}")
    raw = json.loads(state.queue_json) if state.queue_json else []
    queue = _normalize_queue(raw)
    return queue, int(state.queue_index), float(state.queue_progress_seconds)


def _save_state_queue(
    session,
    uid: int,
    queue: list[dict],
    index: int,
    progress: float,
) -> None:
    state = get_player_state(session, uid)
    if state is None:
        raise ValueError(f"Player state not found for uid: {uid}")
    state.queue_json = json.dumps([_item_to_raw(i) for i in queue])
    state.queue_index = index
    state.queue_progress_seconds = progress
    state.last_sync_time = time.time()
    database.commit_or_flush(session)


def _validate_event_ids(event_ids: list[str]) -> list[str]:
    events_map = get_events_map()
    invalid = [eid for eid in event_ids if eid not in events_map]
    if invalid:
        raise ValueError(f"Invalid event ids: {invalid}")
    return event_ids


# ---------------------------------------------------------------------------
# Public operations
# ---------------------------------------------------------------------------
@database.player_atomic
def set_queue(uid: int, queue: list) -> dict:
    """结算旧队列，设置新队列，并重置进度。

    queue 接受旧格式 list[str] 或新格式 list[dict{"event_id": ..., "iterations": ...}]
    """
    settle_player(uid)

    normalized = _normalize_queue(queue)
    event_ids = [i["event_id"] for i in normalized]
    _validate_event_ids(event_ids)

    session = database.get_db()
    state = get_player_state(session, uid)
    if state is None:
        raise ValueError(f"Player state not found for uid: {uid}")
    state.queue_json = json.dumps([_item_to_raw(i) for i in normalized])
    state.queue_index = 0
    state.queue_progress_seconds = 0.0
    state.last_sync_time = time.time()
    database.commit_or_flush(session)
    register_dirty_keys(uid, "queue")

    return {
        "success": True,
        "queue": normalized,
        "queue_index": 0,
        "queue_progress_seconds": 0.0,
    }


@database.player_atomic
def queue_append(uid: int, event_id: str, iterations: int | None = None) -> dict:
    """追加事件到队列末尾。"""
    settle_player(uid)
    _validate_event_ids([event_id])

    session = database.get_db()
    queue, index, progress = _load_state_queue(session, uid)
    queue.append(_normalize_queue_item({"event_id": event_id, "iterations": iterations, "completed": 0}))
    _save_state_queue(session, uid, queue, index, progress)
    register_dirty_keys(uid, "queue")

    return {
        "success": True,
        "queue": queue,
        "queue_index": index,
        "queue_progress_seconds": progress,
    }


@database.player_atomic
def queue_remove(uid: int, index: int) -> dict:
    """移除指定索引的事件。只能移除当前进度之后的事件。"""
    settle_player(uid)

    session = database.get_db()
    queue, current_index, progress = _load_state_queue(session, uid)

    if index < 0 or index >= len(queue):
        raise ValueError(f"Queue index out of range: {index}")
    if index == current_index:
        raise ValueError("Cannot remove current queue item")

    queue.pop(index)
    # 若删除的是当前项之前的已完成项，需调整 current_index
    new_index = current_index
    if index < current_index:
        new_index = max(0, current_index - 1)
    _save_state_queue(session, uid, queue, new_index, progress)
    register_dirty_keys(uid, "queue")

    return {
        "success": True,
        "queue": queue,
        "queue_index": new_index,
        "queue_progress_seconds": progress,
    }


@database.player_atomic
def queue_insert(uid: int, index: int, event_id: str, iterations: int | None = None) -> dict:
    """在指定位置插入事件。只能插入到当前进度之后。"""
    settle_player(uid)
    _validate_event_ids([event_id])

    session = database.get_db()
    queue, current_index, progress = _load_state_queue(session, uid)

    if index < 0 or index > len(queue):
        raise ValueError(f"Queue insert index out of range: {index}")
    if index <= current_index:
        raise ValueError("Cannot insert before or at current queue item")

    queue.insert(index, _normalize_queue_item({"event_id": event_id, "iterations": iterations, "completed": 0}))
    _save_state_queue(session, uid, queue, current_index, progress)
    register_dirty_keys(uid, "queue")

    return {
        "success": True,
        "queue": queue,
        "queue_index": current_index,
        "queue_progress_seconds": progress,
    }


@database.player_atomic
def queue_replace(uid: int, index: int, event_id: str, iterations: int | None = None) -> dict:
    """替换指定位置的事件。只能替换当前进度之后的事件。"""
    settle_player(uid)
    _validate_event_ids([event_id])

    session = database.get_db()
    queue, current_index, progress = _load_state_queue(session, uid)

    if index < 0 or index >= len(queue):
        raise ValueError(f"Queue index out of range: {index}")
    if index <= current_index:
        raise ValueError("Cannot replace current or past queue items")

    queue[index] = _normalize_queue_item({"event_id": event_id, "iterations": iterations, "completed": 0})
    _save_state_queue(session, uid, queue, current_index, progress)
    register_dirty_keys(uid, "queue")

    return {
        "success": True,
        "queue": queue,
        "queue_index": current_index,
        "queue_progress_seconds": progress,
    }


@database.player_atomic
def queue_swap(uid: int, from_index: int, to_index: int) -> dict:
    """交换两个位置的事件。不能交换已完成项，允许和当前项交换（会中断当前活动）。"""
    settle_player(uid)

    session = database.get_db()
    queue, current_index, progress = _load_state_queue(session, uid)

    if from_index < 0 or from_index >= len(queue):
        raise ValueError(f"Queue from_index out of range: {from_index}")
    if to_index < 0 or to_index >= len(queue):
        raise ValueError(f"Queue to_index out of range: {to_index}")
    if from_index < current_index or to_index < current_index:
        raise ValueError("Cannot swap past queue items")

    # 若和当前项交换，重置进度让新项从头开始
    new_progress = progress
    if from_index == current_index or to_index == current_index:
        new_progress = 0.0

    queue[from_index], queue[to_index] = queue[to_index], queue[from_index]
    _save_state_queue(session, uid, queue, current_index, new_progress)
    register_dirty_keys(uid, "queue")

    return {
        "success": True,
        "queue": queue,
        "queue_index": current_index,
        "queue_progress_seconds": new_progress,
    }


@database.player_atomic
def queue_bring_to_front(uid: int, index: int) -> dict:
    """将指定位置的事件提到当前执行位置（插队到当前项前面），中断当前活动。"""
    settle_player(uid)

    session = database.get_db()
    queue, current_index, progress = _load_state_queue(session, uid)

    if index < 0 or index >= len(queue):
        raise ValueError(f"Queue index out of range: {index}")
    if index == current_index:
        return {
            "success": True,
            "queue": queue,
            "queue_index": current_index,
            "queue_progress_seconds": progress,
        }
    if index < current_index:
        raise ValueError("Cannot bring past or current items to front")

    item = queue.pop(index)
    queue.insert(current_index, item)
    _save_state_queue(session, uid, queue, current_index, 0.0)
    register_dirty_keys(uid, "queue")

    return {
        "success": True,
        "queue": queue,
        "queue_index": current_index,
        "queue_progress_seconds": 0.0,
    }
