import json
import os
from typing import Any

class DataManager:
    def __init__(self):
        self.root = os.path.dirname(__file__)
        self.actions = os.path.join(self.root, "actions.json")
        self.level_production = os.path.join(self.root, "level_production.CSV")


_actions_cache: dict[str, Any] | None = None


def load_actions() -> dict[str, Any]:
    """Load actions.json with module-level cache. Single source of truth."""
    global _actions_cache
    if _actions_cache is None:
        dm = DataManager()
        with open(dm.actions, "r", encoding="utf-8") as f:
            _actions_cache = json.load(f)
    return _actions_cache
