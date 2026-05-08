import json
import logging
import queue
import threading

from routes.ws_dispatch import WS_MSG_TYPE, _MESSAGE_HANDLERS

logger = logging.getLogger(__name__)


# ============================================================================
# Serialization abstraction
# ============================================================================
def _encode_message(payload: dict) -> str:
    return json.dumps(payload, ensure_ascii=False)


def _decode_message(raw: str) -> dict:
    return json.loads(raw)


# ============================================================================
# Connection registry & outbound queues
# ============================================================================
connections: dict[int, object] = {}
_outbound_queues: dict[int, queue.Queue] = {}


def _send(ws, payload: dict, req_id: int | None = None) -> None:
    if req_id is not None:
        payload = {**payload, "req_id": req_id}
    data = _encode_message(payload)
    lock = getattr(ws, "_send_lock", None)
    if lock is not None:
        with lock:
            ws.send(data)
    else:
        ws.send(data)


def notify_player(uid: int, payload: dict) -> bool:
    """从任意线程调用，向指定玩家的 WS 推送消息."""
    q = _outbound_queues.get(uid)
    if q is None:
        return False
    q.put(payload)
    return True


# ============================================================================
# WebSocket connection handler
# ============================================================================
def handle_ws(ws, uid: int) -> None:
    old_ws = connections.get(uid)
    if old_ws is not None and old_ws is not ws:
        try:
            old_ws.send(_encode_message({"type": WS_MSG_TYPE.ERROR, "message": "Another connection has been established"}))
            old_ws.close()
        except Exception:
            pass
    connections[uid] = ws

    # 注册出站队列和 send 锁
    outbound_q = queue.Queue(maxsize=256)
    _outbound_queues[uid] = outbound_q
    ws._send_lock = threading.Lock()
    stop_event = threading.Event()

    def sender_loop():
        while not stop_event.is_set():
            try:
                payload = outbound_q.get(timeout=0.5)
            except queue.Empty:
                continue
            if payload is None:
                break
            try:
                _send(ws, payload)
            except Exception:
                logger.debug("WS sender exiting for uid=%s", uid)
                break

    sender_thread = threading.Thread(target=sender_loop, daemon=True)
    sender_thread.start()

    logger.info("WS connected uid=%s", uid)
    try:
        while True:
            raw = ws.receive()
            if raw is None:
                break
            try:
                msg = _decode_message(raw)
            except json.JSONDecodeError:
                _send(ws, {"type": WS_MSG_TYPE.ERROR, "message": "Invalid JSON"})
                continue

            mtype = msg.get("type")
            req_id = msg.get("req_id")

            handler = _MESSAGE_HANDLERS.get(mtype)
            if handler is None:
                _send(ws, {"type": WS_MSG_TYPE.ERROR, "message": f"Unknown message type: {mtype}"}, req_id)
                continue

            try:
                payload = handler(uid, msg)
                if payload is not None:
                    _send(ws, payload, req_id)
            except ValueError as e:
                logger.warning("WS %s error for uid=%s: %s", mtype, uid, e)
                _send(ws, {"type": WS_MSG_TYPE.ERROR, "message": str(e)}, req_id)
            except Exception as e:
                logger.exception("WS %s internal error for uid=%s", mtype, uid)
                _send(ws, {"type": WS_MSG_TYPE.ERROR, "message": f"Internal error: {str(e)}"}, req_id)

    finally:
        stop_event.set()
        try:
            outbound_q.put_nowait(None)
        except queue.Full:
            pass
        sender_thread.join(timeout=2.0)

        if connections.get(uid) is ws:
            logger.info("WS disconnected uid=%s", uid)
            del connections[uid]
        _outbound_queues.pop(uid, None)
        del ws._send_lock

        from game.battle.session import remove_session
        remove_session(uid)
