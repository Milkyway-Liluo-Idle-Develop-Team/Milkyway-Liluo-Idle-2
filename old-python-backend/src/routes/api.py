import json
import threading
import time

import flask

import data.data
from core.auth import require_http_auth
from game.settlement import skip_time
from models import database, User
from services import gameplay_service
from services import market_service
from services import user_service

data_manager = data.data.DataManager()

# 令牌桶 API 限速器
class _TokenBucket:
    __slots__ = ("rate", "capacity", "tokens", "last_refill")
    def __init__(self, rate: float, capacity: float):
        self.rate = rate
        self.capacity = capacity
        self.tokens = capacity
        self.last_refill = time.monotonic()

    def consume(self, n: float = 1.0) -> bool:
        now = time.monotonic()
        elapsed = now - self.last_refill
        self.tokens = min(self.capacity, self.tokens + elapsed * self.rate)
        self.last_refill = now
        if self.tokens >= n:
            self.tokens -= n
            return True
        return False


# 登录限速：每 IP 15 分钟窗口内最多 10 次，即 10/900 ≈ 0.0111 tokens/s
_LOGIN_RATE = 10.0 / 900.0
_LOGIN_CAPACITY = 10.0
_buckets: dict[str, _TokenBucket] = {}
_buckets_lock = threading.Lock()
_CLEANUP_EVERY = 3600
_next_cleanup_at = time.monotonic() + _CLEANUP_EVERY


def _check_login_rate_limit(ip: str) -> bool:
    global _next_cleanup_at
    with _buckets_lock:
        now = time.monotonic()
        if now >= _next_cleanup_at:
            _next_cleanup_at = now + _CLEANUP_EVERY
            stale = [k for k, b in _buckets.items() if b.tokens >= _LOGIN_CAPACITY * 0.98]
            for k in stale:
                del _buckets[k]
        bucket = _buckets.get(ip)
        if bucket is None:
            bucket = _TokenBucket(_LOGIN_RATE, _LOGIN_CAPACITY)
            _buckets[ip] = bucket
        return bucket.consume(1.0)


api_bp = flask.Blueprint("api", __name__)


@api_bp.route("/")
def index():
    return "Milkyway Liluo Idle server v1.0", 200


@api_bp.route("/api/toutou", methods=["POST"])
@require_http_auth
def handle_toutou(uid):
    body = flask.request.get_json()
    target = body.get("target")
    return {"msg": f"User {uid} has toutoued {target}"}, 200


@api_bp.route("/api/heartbeat", methods=["POST"])
@require_http_auth
def handle_heartbeat(uid):
    _ = uid  # authenticated
    return "ok", 200


@api_bp.route("/api/me", methods=["GET"])
@require_http_auth
def handle_me(uid):
    session = database.get_db()
    user = session.get(User, uid)
    if user is None:
        return {"success": False, "error": "User not found"}, 404
    return {
        "success": True,
        "user": {
            "uid": user.uid,
            "username": user.username,
            "email": user.email,
            "created_at": user.created_at,
        },
    }, 200


@api_bp.route("/api/gameplay", methods=["GET"])
@require_http_auth
def handle_gameplay(uid):
    payload = gameplay_service.build_gameplay_payload(uid)
    return {"success": True, "data": payload}, 200


@api_bp.route("/api/debug/skip_time", methods=["POST", "OPTIONS"])
@require_http_auth
def handle_skip_time(uid):
    if flask.request.method == "OPTIONS":
        return "", 204
    body = flask.request.get_json(silent=True) or {}
    seconds = body.get("seconds")
    try:
        seconds = float(seconds)
    except (TypeError, ValueError):
        return {"success": False, "error": "seconds must be a number"}, 400
    if seconds < 0:
        return {"success": False, "error": "seconds must be >= 0"}, 400
    log = skip_time(uid, seconds)
    return {"success": True, "log": log}, 200


@api_bp.route("/api/actions", methods=["GET"])
def handle_actions():
    with open(data_manager.actions, "r", encoding="utf-8") as f:
        actions = json.load(f)

    level_production: list[float] = []
    try:
        with open(data_manager.level_production, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                parts = line.split(",")
                if len(parts) >= 2:
                    try:
                        level_production.append(float(parts[1]))
                    except ValueError:
                        pass
    except FileNotFoundError:
        pass

    return {"items": actions.get("items", []), "events": actions.get("events", []), "level_production": level_production}


@api_bp.route("/api/market", methods=["GET"])
@require_http_auth
def handle_market(uid):
    data = market_service.get_market_snapshot(uid)
    return {"success": True, "data": data}, 200


@api_bp.route("/api/market/listings", methods=["POST", "OPTIONS"])
@require_http_auth
def handle_market_create_listing(uid):
    if flask.request.method == "OPTIONS":
        return "", 204
    body = flask.request.get_json(silent=True) or {}
    item_id = (body.get("item_id") or "").strip()
    quantity = body.get("quantity")
    unit_price = body.get("unit_price")
    if not item_id:
        return {"success": False, "error": "Missing item_id"}, 400
    try:
        data = market_service.create_listing(uid, item_id=item_id, quantity=quantity, unit_price=unit_price)
    except ValueError as e:
        return {"success": False, "error": str(e)}, 400
    return {"success": True, "data": data}, 200


@api_bp.route("/api/market/buy", methods=["POST", "OPTIONS"])
@require_http_auth
def handle_market_buy(uid):
    if flask.request.method == "OPTIONS":
        return "", 204
    body = flask.request.get_json(silent=True) or {}
    listing_id = body.get("listing_id")
    quantity = body.get("quantity")
    try:
        data = market_service.buy_listing(uid, listing_id=listing_id, quantity=quantity)
    except ValueError as e:
        return {"success": False, "error": str(e)}, 400
    return {"success": True, "data": data}, 200


@api_bp.route("/api/market/listings/<int:listing_id>/cancel", methods=["POST", "OPTIONS"])
@require_http_auth
def handle_market_cancel_listing(uid, listing_id: int):
    if flask.request.method == "OPTIONS":
        return "", 204
    try:
        data = market_service.cancel_listing(uid, listing_id=listing_id)
    except ValueError as e:
        return {"success": False, "error": str(e)}, 400
    return {"success": True, "data": data}, 200


@api_bp.route("/api/register", methods=["POST"])
def handle_register():
    body = flask.request.get_json(silent=True) or {}
    username = (body.get("username") or "").strip()
    email = (body.get("email") or "").strip()
    password = body.get("password") or ""

    if not username or not email or not password:
        return {"success": False, "error": "Missing username, email or password"}, 400

    try:
        result = user_service.register(username, email, password)
    except ValueError as e:
        return {"success": False, "error": str(e)}, 400

    resp = flask.make_response({"success": True, **result})
    resp.set_cookie("token", result["token"], httponly=True, samesite="Lax", secure=True)
    return resp, 200


@api_bp.route("/api/login", methods=["POST"])
def handle_login():
    body = flask.request.get_json(silent=True) or {}
    username = (body.get("username") or "").strip()
    password = body.get("password") or ""

    if not username or not password:
        return {"success": False, "error": "Missing username or password"}, 400

    ip = flask.request.remote_addr or "unknown"
    if not _check_login_rate_limit(ip):
        return {"success": False, "error": "Too many attempts, try again later"}, 429

    result = user_service.login(username, password)
    if result is None:
        return {"success": False, "error": "Invalid credentials"}, 401

    resp = flask.make_response({"success": True, **result})
    resp.set_cookie("token", result["token"], httponly=True, samesite="Lax", secure=True)
    return resp, 200


@api_bp.route("/api/logout", methods=["POST"])
@require_http_auth
def handle_logout(uid):
    _ = uid  # authenticated
    token = flask.request.cookies.get("token", "")
    user_service.logout(token)
    resp = flask.make_response({"success": True})
    resp.delete_cookie("token")
    return resp, 200
