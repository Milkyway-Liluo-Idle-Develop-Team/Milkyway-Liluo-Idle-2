from game import battle, parallel_mode
from game.context import PlayerContext, register_dirty_keys
from game.settlement import settle_player, get_events_map, check_requirements
from game.queue import (
    set_queue,
    queue_append,
    queue_remove,
    queue_insert,
    queue_replace,
    queue_swap,
    queue_bring_to_front,
    _load_state_queue,
    _save_state_queue,
)
from game.actions import execute_instant, execute_upgrade
from game.equipment import equip_item, unequip_item
from game.enhancement import get_enhance_preview, execute_enhancement
from models import database
from services import gameplay_service


# ============================================================================
# Message type constants -- mirrors proto WsMessageType
# ============================================================================
class WS_MSG_TYPE:
    UNKNOWN = "unknown"
    ERROR = "error"
    DELTA = "delta"
    GAMEPLAY = "gameplay"
    GAMEPLAY_LIGHT = "gameplay_light"

    ACTION_LOOP = "action_loop"
    ACTION_LOOP_STOP = "action_loop_stop"
    SYNC = "sync"
    INSTANT = "instant"
    UPGRADE = "upgrade"
    EQUIP = "equip"
    UNEQUIP = "unequip"
    ENHANCE_PREVIEW = "enhance_preview"
    ENHANCE_EXECUTE = "enhance_execute"
    SET_QUEUE = "set_queue"
    QUEUE_APPEND = "queue_append"
    QUEUE_REMOVE = "queue_remove"
    QUEUE_INSERT = "queue_insert"
    QUEUE_REPLACE = "queue_replace"
    QUEUE_SWAP = "queue_swap"
    QUEUE_BRING_TO_FRONT = "queue_bring_to_front"

    BATTLE_LIST = "battle_list"
    BATTLE_START = "battle_start"
    BATTLE_STATE = "battle_state"
    BATTLE_STOP = "battle_stop"


# ============================================================================
# Message handlers -- each returns a payload dict to be sent back to client.
# Validation failures are signaled by raising ValueError.
# ============================================================================
def _handle_sync(uid: int, msg: dict) -> dict:
    settle_player(uid)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_set_queue(uid: int, msg: dict) -> dict:
    queue_data = msg.get("queue", [])
    if not isinstance(queue_data, list):
        raise ValueError("queue must be a list")
    set_queue(uid, queue_data)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_queue_append(uid: int, msg: dict) -> dict:
    event_id = str(msg.get("event_id") or "").strip()
    if not event_id:
        raise ValueError("event_id required")
    iterations_raw = msg.get("iterations")
    iterations = int(iterations_raw) if iterations_raw is not None and str(iterations_raw).strip() != "" else None
    queue_append(uid, event_id, iterations=iterations)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_queue_remove(uid: int, msg: dict) -> dict:
    index = msg.get("index")
    if not isinstance(index, int):
        raise ValueError("index must be an integer")
    queue_remove(uid, index)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_queue_insert(uid: int, msg: dict) -> dict:
    index = msg.get("index")
    event_id = str(msg.get("event_id") or "").strip()
    if not isinstance(index, int):
        raise ValueError("index must be an integer")
    if not event_id:
        raise ValueError("event_id required")
    iterations_raw = msg.get("iterations")
    iterations = int(iterations_raw) if iterations_raw is not None and str(iterations_raw).strip() != "" else None
    queue_insert(uid, index, event_id, iterations=iterations)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_queue_replace(uid: int, msg: dict) -> dict:
    index = msg.get("index")
    event_id = str(msg.get("event_id") or "").strip()
    if not isinstance(index, int):
        raise ValueError("index must be an integer")
    if not event_id:
        raise ValueError("event_id required")
    iterations_raw = msg.get("iterations")
    iterations = int(iterations_raw) if iterations_raw is not None and str(iterations_raw).strip() != "" else None
    queue_replace(uid, index, event_id, iterations=iterations)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_queue_swap(uid: int, msg: dict) -> dict:
    from_index = msg.get("from_index")
    to_index = msg.get("to_index")
    if not isinstance(from_index, int) or not isinstance(to_index, int):
        raise ValueError("from_index and to_index must be integers")
    queue_swap(uid, from_index, to_index)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_queue_bring_to_front(uid: int, msg: dict) -> dict:
    index = msg.get("index")
    if not isinstance(index, int):
        raise ValueError("index must be an integer")
    queue_bring_to_front(uid, index)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_instant(uid: int, msg: dict) -> dict:
    event_id = msg.get("event_id")
    if not event_id:
        raise ValueError("event_id required")
    execute_instant(uid, event_id)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_upgrade(uid: int, msg: dict) -> dict:
    event_id = msg.get("event_id")
    if not event_id:
        raise ValueError("event_id required")
    execute_upgrade(uid, event_id)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_equip(uid: int, msg: dict) -> dict:
    item_id = msg.get("item_id")
    slot = msg.get("slot")
    if not item_id or not slot:
        raise ValueError("item_id and slot required")
    equip_item(uid, item_id, slot)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_unequip(uid: int, msg: dict) -> dict:
    slot = msg.get("slot")
    if not slot:
        raise ValueError("slot required")
    unequip_item(uid, slot)
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_enhance_preview(uid: int, msg: dict) -> dict:
    slot_type = msg.get("slot_type")
    anchor_slot = msg.get("anchor_slot")
    if not slot_type or not anchor_slot:
        raise ValueError("slot_type and anchor_slot required")
    data = get_enhance_preview(uid, str(slot_type), str(anchor_slot))
    return {"type": WS_MSG_TYPE.ENHANCE_PREVIEW, "data": data}


def _handle_enhance_execute(uid: int, msg: dict) -> dict:
    slot_type = msg.get("slot_type")
    anchor_slot = msg.get("anchor_slot")
    if not slot_type or not anchor_slot:
        raise ValueError("slot_type and anchor_slot required")
    data = execute_enhancement(uid, str(slot_type), str(anchor_slot))
    return {"type": WS_MSG_TYPE.ENHANCE_EXECUTE, "data": data}


def _handle_gameplay(uid: int, msg: dict) -> dict:
    data = gameplay_service.build_gameplay_payload(uid)
    return {"type": WS_MSG_TYPE.GAMEPLAY, "data": data}


def _handle_gameplay_light(uid: int, msg: dict) -> dict:
    data = gameplay_service.build_gameplay_light_payload(uid)
    return {"type": WS_MSG_TYPE.GAMEPLAY_LIGHT, "data": data}


def _is_production_blocked_by_battle(uid: int) -> bool:
    if not battle.is_battle_running(uid):
        return False
    with database.scoped_session() as session:
        ctx = PlayerContext.load(session, uid)
        return not parallel_mode.has_double_body_unlocked(ctx)


def _handle_action_loop(uid: int, msg: dict) -> dict:
    event_id = str(msg.get("event_id") or "").strip()
    if not event_id:
        raise ValueError("event_id required")
    settle_player(uid)
    event = get_events_map().get(event_id)
    if event is None:
        raise ValueError(f"Invalid event id: {event_id}")
    if event.get("type") != "loop":
        raise ValueError(f"Event {event_id} is not a loop action")
    if _is_production_blocked_by_battle(uid):
        raise ValueError("当前不可进行生产行动")
    with database.scoped_session() as session:
        ctx = PlayerContext.load(session, uid)
        if not check_requirements(event.get("requirements"), ctx):
            raise ValueError(f"Requirements not met for {event_id}")
    iterations_raw = msg.get("iterations")
    iterations = int(iterations_raw) if iterations_raw is not None and str(iterations_raw).strip() != "" else None
    if iterations is not None and iterations > 0:
        set_queue(uid, [{"event_id": event_id, "iterations": iterations}])
    else:
        set_queue(uid, [event_id])
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_action_loop_stop(uid: int, msg: dict) -> dict:
    settle_player(uid)
    with database.scoped_session() as session:
        queue_data, current_index, progress = _load_state_queue(session, uid)
        if current_index < len(queue_data):
            queue_data.pop(current_index)
        _save_state_queue(session, uid, queue_data, current_index, 0.0)
    register_dirty_keys(uid, "queue")
    return {"type": WS_MSG_TYPE.DELTA, "data": gameplay_service.build_patch_payload(uid)}


def _handle_battle_list(uid: int, msg: dict) -> dict:
    map_id = msg.get("map")
    data = battle.list_battles(map_id=map_id or None)
    return {"type": WS_MSG_TYPE.BATTLE_LIST, "data": data}


def _handle_battle_start(uid: int, msg: dict) -> dict:
    battle_id = msg.get("battle_id")
    if not battle_id:
        raise ValueError("battle_id required")
    player_skills = msg.get("player_skills")
    data = battle.start_battle(uid, battle_id=battle_id, player_skills=player_skills)
    return {"type": WS_MSG_TYPE.BATTLE_STATE, "data": data}


def _handle_battle_state(uid: int, msg: dict) -> dict:
    data = battle.get_battle_state(uid)
    return {"type": WS_MSG_TYPE.BATTLE_STATE, "data": data}


def _handle_battle_stop(uid: int, msg: dict) -> dict:
    data = battle.stop_battle(uid)
    return {"type": WS_MSG_TYPE.BATTLE_STATE, "data": data}


_MESSAGE_HANDLERS: dict[str, ...] = {
    WS_MSG_TYPE.SYNC: _handle_sync,
    WS_MSG_TYPE.SET_QUEUE: _handle_set_queue,
    WS_MSG_TYPE.QUEUE_APPEND: _handle_queue_append,
    WS_MSG_TYPE.QUEUE_REMOVE: _handle_queue_remove,
    WS_MSG_TYPE.QUEUE_INSERT: _handle_queue_insert,
    WS_MSG_TYPE.QUEUE_REPLACE: _handle_queue_replace,
    WS_MSG_TYPE.QUEUE_SWAP: _handle_queue_swap,
    WS_MSG_TYPE.QUEUE_BRING_TO_FRONT: _handle_queue_bring_to_front,
    WS_MSG_TYPE.INSTANT: _handle_instant,
    WS_MSG_TYPE.UPGRADE: _handle_upgrade,
    WS_MSG_TYPE.EQUIP: _handle_equip,
    WS_MSG_TYPE.UNEQUIP: _handle_unequip,
    WS_MSG_TYPE.ENHANCE_PREVIEW: _handle_enhance_preview,
    WS_MSG_TYPE.ENHANCE_EXECUTE: _handle_enhance_execute,
    WS_MSG_TYPE.GAMEPLAY: _handle_gameplay,
    WS_MSG_TYPE.GAMEPLAY_LIGHT: _handle_gameplay_light,
    WS_MSG_TYPE.ACTION_LOOP: _handle_action_loop,
    WS_MSG_TYPE.ACTION_LOOP_STOP: _handle_action_loop_stop,
    WS_MSG_TYPE.BATTLE_LIST: _handle_battle_list,
    WS_MSG_TYPE.BATTLE_START: _handle_battle_start,
    WS_MSG_TYPE.BATTLE_STATE: _handle_battle_state,
    WS_MSG_TYPE.BATTLE_STOP: _handle_battle_stop,
}
