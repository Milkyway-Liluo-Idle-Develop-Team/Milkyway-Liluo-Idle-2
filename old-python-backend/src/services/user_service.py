import secrets
import time

from sqlalchemy import select

from models import database, User
from core.auth import hash_password, verify_password, create_session, delete_session
from game.context import get_player_state, create_player_state
from services import market_service

SESSION_TTL_DAYS = 7


# ---------------------------------------------------------------------------
# User helpers
# ---------------------------------------------------------------------------
def create_user(session, username: str, email: str, password_hash: str) -> int:
    user = User(
        username=username,
        email=email,
        password_hash=password_hash,
        created_at=time.time(),
    )
    session.add(user)
    session.commit()
    session.refresh(user)
    return user.uid


def get_user_by_username(session, username: str) -> User | None:
    return session.execute(
        select(User).where(User.username == username)
    ).scalar_one_or_none()


def get_user_by_email(session, email: str) -> User | None:
    return session.execute(
        select(User).where(User.email == email)
    ).scalar_one_or_none()


# ---------------------------------------------------------------------------
# Service functions
# ---------------------------------------------------------------------------
def register(username: str, email: str, password: str) -> dict:
    """注册新用户并返回 {uid, token, username, email, created_at}。校验失败时抛出 ValueError。"""
    if len(password) < 6:
        raise ValueError("Password must be at least 6 characters")
    session = database.get_db()
    if get_user_by_username(session, username):
        raise ValueError("Username already exists")
    if get_user_by_email(session, email):
        raise ValueError("Email already exists")

    now = time.time()
    pw_hash = hash_password(password)
    uid = create_user(session, username, email, pw_hash)
    create_player_state(session, uid)
    market_service.ensure_market_account(session, uid)
    token = secrets.token_hex(32)
    expires_at = now + SESSION_TTL_DAYS * 86400
    create_session(session, token, uid, expires_at=expires_at)
    return {"uid": uid, "token": token, "username": username, "email": email, "created_at": now}


def login(username: str, password: str) -> dict | None:
    """验证用户凭据，成功返回 {uid, token, username, email, created_at}，失败返回 None。"""
    session = database.get_db()
    user = get_user_by_username(session, username)
    if not user or not verify_password(password, user.password_hash):
        return None

    if get_player_state(session, user.uid) is None:
        create_player_state(session, user.uid)
    market_service.ensure_market_account(session, user.uid)

    token = secrets.token_hex(32)
    expires_at = time.time() + SESSION_TTL_DAYS * 86400
    create_session(session, token, user.uid, expires_at=expires_at)
    return {
        "uid": user.uid,
        "token": token,
        "username": user.username,
        "email": user.email,
        "created_at": user.created_at,
    }


def logout(token: str) -> None:
    """登出用户，删除对应的 session。"""
    if not token:
        return
    session = database.get_db()
    delete_session(session, token)
