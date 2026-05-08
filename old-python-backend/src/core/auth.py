import functools
import json
import time

import flask

import time

from sqlalchemy import select

from models import database, UserSession
import bcrypt


def hash_password(password: str) -> str:
    return bcrypt.hashpw(password.encode("utf-8"), bcrypt.gensalt()).decode("utf-8")


def verify_password(password: str, password_hash: str) -> bool:
    return bcrypt.checkpw(password.encode("utf-8"), password_hash.encode("utf-8"))


def create_session(session, token: str, uid: int, expires_at: float | None = None) -> None:
    sess = UserSession(
        token=token,
        uid=uid,
        created_at=time.time(),
        expires_at=expires_at,
    )
    session.add(sess)
    session.commit()


def get_session(session, token: str) -> UserSession | None:
    return session.execute(
        select(UserSession).where(UserSession.token == token)
    ).scalar_one_or_none()


def delete_session(session, token: str) -> None:
    sess = get_session(session, token)
    if sess:
        session.delete(sess)
        session.commit()


def parse_token(token: str) -> int | None:
    if not token:
        return None
    sess = database.SessionLocal()
    try:
        db_session = get_session(sess, token)
        if db_session is None:
            return None
        if db_session.expires_at is not None and db_session.expires_at < time.time():
            delete_session(sess, token)
            return None
        return db_session.uid
    finally:
        sess.close()


def require_ws_auth(handler):
    """WebSocket 连接时鉴权装饰器。期望客户端通过 Cookie 传入 token。"""
    @functools.wraps(handler)
    def wrapper(ws):
        token = flask.request.cookies.get("token", "")
        if not token:
            ws.send(json.dumps({"type": "error", "message": "Missing token cookie"}, ensure_ascii=False))
            return
        uid = parse_token(token)
        if uid is None:
            ws.send(json.dumps({"type": "error", "message": "Invalid or expired token"}, ensure_ascii=False))
            return
        return handler(ws, uid)
    return wrapper


def require_http_auth(handler):
    """HTTP 路由鉴权装饰器。期望客户端通过 Cookie 传入 token。"""
    @functools.wraps(handler)
    def wrapper(*args, **kwargs):
        if flask.request.method == "OPTIONS":
            return "", 204
        token = flask.request.cookies.get("token", "")
        if not token:
            return {"success": False, "error": "Missing token cookie"}, 401
        uid = parse_token(token)
        if uid is None:
            return {"success": False, "error": "Invalid or expired token"}, 401
        return handler(uid, *args, **kwargs)
    return wrapper
