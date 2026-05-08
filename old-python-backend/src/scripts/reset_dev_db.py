"""开发期一键重置数据库。

用法：在 MilkywayLiluoIdleServer/ 目录下执行
    python -m scripts.reset_dev_db

这会删除 database.db 并用当前 SQLAlchemy 模型重建 schema。
alembic_version 表会被 stamp 为当前 head，保持与 alembic 兼容。
"""

import os
import sys

# 把 src/ 加入路径，方便直接运行
src_dir = os.path.join(os.path.dirname(__file__), "..")
sys.path.insert(0, os.path.abspath(src_dir))

from models import Base
from models.database import engine, _DB_PATH
from alembic import command
from alembic.config import Config
from alembic.script import ScriptDirectory


def main() -> None:
    db_file = _DB_PATH
    if db_file.startswith("sqlite:///"):
        db_file = db_file[len("sqlite:///"):]

    if os.path.exists(db_file):
        os.remove(db_file)
        print(f"removed {db_file}")
    else:
        print(f"{db_file} does not exist, creating fresh")

    Base.metadata.create_all(bind=engine)
    print("schema created from models")

    # stamp alembic head so init_db() doesn't complain
    server_dir = os.path.join(os.path.dirname(__file__), "..", "..")
    alembic_ini = os.path.join(server_dir, "alembic.ini")
    if os.path.exists(alembic_ini):
        alembic_cfg = Config(alembic_ini)
        alembic_cfg.set_main_option(
            "sqlalchemy.url", engine.url.render_as_string(hide_password=False)
        )
        migrations_dir = os.path.join(server_dir, "migrations")
        alembic_cfg.set_main_option("script_location", migrations_dir)
        command.stamp(alembic_cfg, "head")
        print("alembic stamped to head")
    else:
        print("warning: alembic.ini not found, skipped stamp")


if __name__ == "__main__":
    main()
