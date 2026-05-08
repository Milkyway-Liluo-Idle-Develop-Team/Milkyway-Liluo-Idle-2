import contextvars
import os
import threading
from contextlib import contextmanager
from functools import wraps

from alembic import command
from alembic.config import Config
from alembic.script import ScriptDirectory
from flask import g
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker

from models import Base

# Re-export for reset_dev_db convenience
__all__ = ["Base", "engine"]

_DB_PATH = os.environ.get("DATABASE_URL") or os.path.join(
    os.path.dirname(__file__), "..", "database.db"
)
engine = create_engine(
    _DB_PATH if _DB_PATH.startswith("sqlite://") else f"sqlite:///{_DB_PATH}",
    echo=False,
)
SessionLocal = sessionmaker(bind=engine)

_db_session_var: contextvars.ContextVar = contextvars.ContextVar("db_session", default=None)


def get_db():
    """获取当前请求上下文中的 SQLAlchemy Session，没有则新建。

    优先读取 scoped_session() 设置的上下文变量（WebSocket 场景），
    否则回退到 flask.g（HTTP 请求场景）。
    """
    sess = _db_session_var.get()
    if sess is not None:
        return sess
    if "db" not in g:
        g.db = SessionLocal()
    return g.db


def close_db(e=None) -> None:
    """关闭当前请求上下文中的 Session。"""
    db = g.pop("db", None)
    if db is not None:
        db.close()


@contextmanager
def scoped_session():
    """获取或创建一个 SQLAlchemy Session，通过 contextvars 暴露给 get_db()。

    如果当前上下文已存在 session（嵌套调用），直接复用；
    否则新建 session，在退出时统一 commit/rollback。

    适用于 WebSocket 等需要比 flask.g 更小作用域的场景。
    """
    existing = _db_session_var.get()
    if existing is not None:
        yield existing
        return
    sess = SessionLocal()
    token = _db_session_var.set(sess)
    try:
        yield sess
        sess.commit()
    except Exception:
        sess.rollback()
        raise
    finally:
        _db_session_var.reset(token)
        sess.close()


def is_managed_session() -> bool:
    """当前是否处于 scoped_session / player_atomic 管理的 session 上下文中。"""
    return _db_session_var.get() is not None


def commit_or_flush(session) -> None:
    """在 managed session 内执行 flush，否则执行 commit。

    用于 @player_atomic 内部或嵌套调用场景：managed 时 flush 让外层统一 commit，
    非 managed 时直接 commit 保证数据持久化。
    """
    if is_managed_session():
        session.flush()
    else:
        session.commit()


# uid-level locks for player_atomic
_uid_locks: dict[int, threading.RLock] = {}
_uid_locks_global_lock = threading.Lock()


def _get_uid_lock(uid: int) -> threading.RLock:
    with _uid_locks_global_lock:
        lock = _uid_locks.get(uid)
        if lock is None:
            lock = threading.RLock()
            _uid_locks[uid] = lock
        return lock


def player_atomic(func):
    """玩家 uid 级别的原子性装饰器。

    保证同一 uid 的并发操作串行执行，且每个操作在独立的 scoped_session 中完成。
    """
    @wraps(func)
    def wrapper(uid: int, *args, **kwargs):
        lock = _get_uid_lock(uid)
        with lock:
            with scoped_session():
                return func(uid, *args, **kwargs)
    return wrapper


def init_db() -> None:
    """初始化数据库：检查版本一致性，仅在全新数据库时自动建表。"""
    server_dir = os.path.join(os.path.dirname(__file__), "..", "..")
    alembic_ini = os.path.join(server_dir, "alembic.ini")
    alembic_cfg = Config(alembic_ini)
    alembic_cfg.set_main_option("sqlalchemy.url", engine.url.render_as_string(hide_password=False))
    migrations_dir = os.path.join(server_dir, "migrations")
    alembic_cfg.set_main_option("script_location", migrations_dir)

    with engine.connect() as conn:
        has_tables = conn.execute(
            text("SELECT name FROM sqlite_master WHERE type='table' AND name='user'")
        ).fetchone() is not None
        version_row = conn.execute(
            text("SELECT version_num FROM alembic_version")
        ).fetchone() if conn.execute(
            text("SELECT name FROM sqlite_master WHERE type='table' AND name='alembic_version'")
        ).fetchone() is not None else None

    if not has_tables:
        Base.metadata.create_all(bind=engine)
        command.stamp(alembic_cfg, "head")
    elif version_row is None:
        command.stamp(alembic_cfg, "head")
    else:
        current_version = version_row[0]
        script = ScriptDirectory.from_config(alembic_cfg)
        head_version = script.get_current_head()
        if current_version != head_version:
            raise RuntimeError(
                f"数据库版本不匹配：当前 {current_version}，代码期望 {head_version}。"
                f"请手动执行 'alembic upgrade head' 或 'alembic downgrade {head_version}' 后再启动。"
            )

    session = SessionLocal()
    try:
        session.commit()
    finally:
        session.close()


