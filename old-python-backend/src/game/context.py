"""玩家运行时上下文，封装一次结算周期内的所有状态。"""

import contextvars
import json
import time

from sqlalchemy import select
from sqlalchemy.orm import Session

from models import (
    database,
    PlayerState,
    PlayerSkill,
    PlayerUnlockedEvent,
    PlayerEventProgress,
    PlayerSeenItem,
    PlayerEquipment,
    PlayerTool,
)
from game.attributes import (
    get_items_map,
    collect_modifiers,
    AttributeSet,
    PRODUCTION_SKILLS,
    get_level_production_multiplier,
)
from game.inventory import InventoryContainer, DbStore, InsufficientItem

LEGACY_EVENT_PROGRESS_ALIASES: dict[str, tuple[str, ...]] = {
    "home_expanding": tuple(f"home_expanding_{idx}" for idx in range(1, 11)),
}

DEFAULT_PLAYER_SKILLS: tuple[str, ...] = (
    "felling",
    "mining",
    "planting",
    "crafting",
    "forging",
    "enhancing",
    "trading",
    "strength",
    "ranging",
    "resilience",
    "stamina",
    "intelligence",
    "defense",
    "magic",
)

# 请求级别 dirty-key 存储，替代全局 dict 避免跨请求泄漏。
# 每个线程 / asyncio task 通过 ContextVar 持有独立 dict，请求结束自动 GC。
_dirty_registry_ctx: contextvars.ContextVar[dict[int, set[str]]] = contextvars.ContextVar(
    "dirty_registry", default=None
)
_newly_seen_registry_ctx: contextvars.ContextVar[dict[int, set[str]]] = contextvars.ContextVar(
    "newly_seen_registry", default=None
)


def _get_dirty_dict() -> dict[int, set[str]]:
    d = _dirty_registry_ctx.get()
    if d is None:
        d = {}
        _dirty_registry_ctx.set(d)
    return d


def _get_newly_seen_dict() -> dict[int, set[str]]:
    d = _newly_seen_registry_ctx.get()
    if d is None:
        d = {}
        _newly_seen_registry_ctx.set(d)
    return d


def get_player_state(session: Session, uid: int) -> PlayerState | None:
    return session.execute(
        select(PlayerState).where(PlayerState.uid == uid)
    ).scalar_one_or_none()


def create_player_state(session: Session, uid: int) -> None:
    now = time.time()
    state = PlayerState(uid=uid, last_sync_time=now)
    session.add(state)
    for skill_id in DEFAULT_PLAYER_SKILLS:
        session.add(PlayerSkill(uid=uid, skill_id=skill_id, level=1, exp=0.0))
    session.commit()


def register_dirty_keys(uid: int, *keys: str) -> None:
    if keys:
        _get_dirty_dict().setdefault(uid, set()).update(keys)


def pop_dirty_keys(uid: int) -> set[str]:
    return _get_dirty_dict().pop(uid, set())


def register_newly_seen(uid: int, *item_ids: str) -> None:
    if item_ids:
        _get_newly_seen_dict().setdefault(uid, set()).update(item_ids)


def pop_newly_seen(uid: int) -> set[str]:
    return _get_newly_seen_dict().pop(uid, set())


class PlayerContext:
    """玩家运行时上下文。

    将原来 _load_runtime_state 返回的 7 元素 tuple 封装为对象，
    提供 load / save / to_state_response 一站式方法。
    """

    __slots__ = (
        "uid", "session", "state",
        "inventory", "skills", "unlocked",
        "event_progress", "seen_items",
        "equipment", "tools", "attr_set",
        "_dirty", "_newly_seen",
    )

    def __init__(
        self,
        uid: int,
        session: Session,
        state: PlayerState,
        inventory: InventoryContainer,
        skills: dict[str, PlayerSkill],
        unlocked: set[str],
        event_progress: dict[str, PlayerEventProgress],
        seen_items: set[str],
        equipment: dict[str, PlayerEquipment],
        tools: dict[str, PlayerTool],
        attr_set: AttributeSet,
    ):
        self.uid = uid
        self.session = session
        self.state = state
        self.inventory = inventory
        self.skills = skills
        self.unlocked = unlocked
        self.event_progress = event_progress
        self.seen_items = seen_items
        self.equipment = equipment
        self.tools = tools
        self.attr_set = attr_set
        self._dirty: set[str] = set()
        self._newly_seen: set[str] = set()

    @classmethod
    def load(cls, session: Session, uid: int) -> "PlayerContext":
        """从 DB 加载所有玩家状态，构建 AttributeSet。"""
        state = get_player_state(session, uid)
        if state is None:
            raise ValueError(f"Player state not found for uid: {uid}")
        container = InventoryContainer(uid, DbStore(session, uid))
        container.load_all()
        skills = {sk.skill_id: sk for sk in state.user.skills}
        unlocked = {ue.event_id for ue in state.user.unlocked_events}
        event_progress = {
            ep.event_id: ep for ep in state.user.event_progress
        }
        for canonical_event_id, legacy_event_ids in LEGACY_EVENT_PROGRESS_ALIASES.items():
            legacy_total = 0
            for legacy_event_id in legacy_event_ids:
                legacy_obj = event_progress.get(legacy_event_id)
                if legacy_obj is None:
                    continue
                legacy_total += int(legacy_obj.completed_count)
            if legacy_total <= 0:
                continue

            canonical_obj = event_progress.get(canonical_event_id)
            if canonical_obj is None:
                canonical_obj = PlayerEventProgress(
                    uid=uid,
                    event_id=canonical_event_id,
                    completed_count=legacy_total,
                )
                session.add(canonical_obj)
                event_progress[canonical_event_id] = canonical_obj
            elif int(canonical_obj.completed_count) < legacy_total:
                canonical_obj.completed_count = legacy_total

        seen_items = {si.item_id for si in state.user.seen_items}
        for (item_id, _state), row in container.items():
            if row.quantity > 0:
                seen_items.add(item_id)
        equipment = {e.slot: e for e in state.user.equipment_items}
        tools = {t.slot: t for t in state.user.tool_items}

        items_map = get_items_map()
        mods = collect_modifiers(items_map, equipment, tools)

        base_values: dict[str, float] = {}
        for skill_id in PRODUCTION_SKILLS:
            skill_obj = skills.get(skill_id)
            skill_level = int(skill_obj.level) if skill_obj else 1
            level_multiplier = get_level_production_multiplier(skill_level)
            base_values[f"{skill_id}_reward_mult"] = max(0.0, level_multiplier - 1.0)
            base_values[f"{skill_id}_level_production_multiplier"] = level_multiplier

        attr_set = AttributeSet(base_values, mods)

        return cls(
            uid=uid,
            session=session,
            state=state,
            inventory=container,
            skills=skills,
            unlocked=unlocked,
            event_progress=event_progress,
            seen_items=seen_items,
            equipment=equipment,
            tools=tools,
            attr_set=attr_set,
        )

    def mark_dirty(self, *keys: str) -> None:
        self._dirty.update(keys)

    def get_event_count(self, event_id: str) -> int:
        obj = self.event_progress.get(event_id)
        return int(obj.completed_count) if obj is not None else 0

    def mark_event_completed(self, event_id: str, delta: int = 1) -> None:
        if delta <= 0:
            return
        obj = self.event_progress.get(event_id)
        if obj is None:
            obj = PlayerEventProgress(uid=self.uid, event_id=event_id, completed_count=0)
            self.session.add(obj)
            self.event_progress[event_id] = obj
        obj.completed_count += delta
        self._dirty.add(f"event_progress:{event_id}")

    def inventory_to_state(self) -> list[dict[str, int | str]]:
        """Serialize inventory as a list of entries. State is omitted when 0."""
        return self.inventory.to_list()

    def has_seen_item(self, item_id: str) -> bool:
        return item_id in self.seen_items

    def mark_item_seen(self, item_id: str) -> None:
        if item_id and item_id not in self.seen_items:
            self.seen_items.add(item_id)
            self._newly_seen.add(item_id)
            self._dirty.add("new_seen_items")

    def save(self) -> None:
        """提交变更到 DB。ORM unit-of-work 自动追踪 inventory 变更。"""
        existing_event_rows = {ue.event_id: ue for ue in self.state.user.unlocked_events}
        for event_id in self.unlocked:
            if event_id not in existing_event_rows:
                self.state.user.unlocked_events.append(
                    PlayerUnlockedEvent(uid=self.uid, event_id=event_id)
                )
        for event_id, row in list(existing_event_rows.items()):
            if event_id not in self.unlocked:
                self.state.user.unlocked_events.remove(row)

        existing_seen_items = {si.item_id: si for si in self.state.user.seen_items}
        for item_id in self.seen_items:
            if item_id not in existing_seen_items:
                self.state.user.seen_items.append(
                    PlayerSeenItem(
                        uid=self.uid,
                        item_id=item_id,
                        first_seen_at=self.state.last_sync_time,
                    )
                )
        database.commit_or_flush(self.session)

        # Journal → dirty keys
        journal = self.inventory.flush_journal()
        for entry in journal:
            key = f"item:{entry.item_id}:{entry.state}" if entry.state != 0 else f"item:{entry.item_id}"
            self._dirty.add(key)
        new_items = self.inventory.flush_new_items()
        for item_id in new_items:
            if item_id and item_id not in self.seen_items:
                self.seen_items.add(item_id)
                self._newly_seen.add(item_id)

        register_dirty_keys(self.uid, *self._dirty)
        if self._newly_seen:
            register_newly_seen(self.uid, *self._newly_seen)
        self._dirty.clear()
        self._newly_seen.clear()

