"""Generic inventory container with built-in change tracking.

Design:
- Container works with any object that satisfies `InventoryRowLike` (Protocol).
- `PlayerInventory` ORM model already satisfies this — no parallel dataclass needed.
- `Store` is the persistence adapter: fetch + create in one interface.
  Different tables implement their own Store, optionally adding table-specific methods.

Design invariants:
- `quantity` is a non-negative integer.
- `consume` never deletes the row even when quantity reaches 0.
- `add` creates a row on demand via the store.
- `fractional` is a server-side accumulator in `[0, 1)`.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Iterator, Protocol, runtime_checkable

from sqlalchemy import select
from sqlalchemy.orm import Session

from models import PlayerInventory


# ── Protocols ──────────────────────────────────────────────────────────────

@runtime_checkable
class InventoryRowLike(Protocol):
    """Protocol that ORM models (and test doubles) satisfy.

    PlayerInventory already matches this — no extra dataclass required.
    """
    item_id: str
    item_state: int
    quantity: int
    fractional: float


class Store(Protocol):
    """Persistence adapter for one inventory table.

    Merges fetch + create. Implementations can add table-specific methods
    (e.g. expire_listings, withdraw_lock, etc.).
    """

    def fetch_one(self, item_id: str, state: int) -> InventoryRowLike | None: ...

    def fetch_all(self) -> list[InventoryRowLike]: ...

    def create(self, uid: int, item_id: str, state: int) -> InventoryRowLike: ...


# ── Journal entry ─────────────────────────────────────────────────────────

@dataclass(slots=True)
class InventoryEntry:
    """A single change record for the journal."""
    item_id: str
    state: int = 0
    quantity: int = 0   # positive = add, negative = consume


# ── DB-backed store ────────────────────────────────────────────────────────

class DbStore:
    """Store backed by a PlayerInventory table.

    Additional table-specific methods (e.g. expiry, locking) can be added
    on other Store implementations without touching this one.
    """

    __slots__ = ("_session", "_uid")

    def __init__(self, session: Session, uid: int):
        self._session = session
        self._uid = uid

    def fetch_one(self, item_id: str, state: int) -> PlayerInventory | None:
        return self._session.get(PlayerInventory, (self._uid, item_id, state))

    def fetch_all(self) -> list[PlayerInventory]:
        return list(
            self._session.execute(
                select(PlayerInventory).where(PlayerInventory.uid == self._uid)
            ).scalars().all()
        )

    def create(self, uid: int, item_id: str, state: int) -> PlayerInventory:
        row = PlayerInventory(
            uid=uid, item_id=item_id, item_state=state, quantity=0, fractional=0.0
        )
        self._session.add(row)
        return row


# ── Container ──────────────────────────────────────────────────────────────

class InsufficientItem(ValueError):
    """Raised by `consume` when the container does not have enough of an item."""

    def __init__(self, item_id: str, state: int, requested: int, available: int):
        self.item_id = item_id
        self.state = state
        self.requested = requested
        self.available = available
        super().__init__(
            f"Insufficient item {item_id!r} (state={state}): "
            f"need {requested}, have {available}"
        )


class InventoryContainer:
    """Generic inventory with built-in journal for change tracking.

    Storage-agnostic: works with any Store implementation.
    Different inventory tables just need their own Store.

    Usage:
        store = DbStore(session, uid)
        container = InventoryContainer(uid, store)
        container.add("wood", 5)
        container.consume("wood", 2)
        journal = container.flush_journal()
    """

    __slots__ = ("_uid", "_rows", "_journal", "_new_items", "_store", "_full_loaded")

    def __init__(self, uid: int, store: Store | None = None):
        self._uid = uid
        self._rows: dict[tuple[str, int], InventoryRowLike] = {}
        self._journal: list[InventoryEntry] = []
        self._new_items: set[str] = set()
        self._store = store
        self._full_loaded = False

    # ── internal ──

    def _ensure(self, item_id: str, state: int = 0) -> InventoryRowLike:
        key = (item_id, state)
        row = self._rows.get(key)
        if row is not None:
            return row
        if self._store is not None and not self._full_loaded:
            loaded = self._store.fetch_one(item_id, state)
            if loaded is not None:
                self._rows[key] = loaded
                return loaded
        if self._store is None:
            raise RuntimeError(
                f"Cannot create row for ({item_id!r}, {state}): no store set"
            )
        row = self._store.create(self._uid, item_id, state)
        self._rows[key] = row
        return row

    def load_all(self) -> None:
        """Eagerly load all rows from the store (e.g. for snapshot)."""
        if self._store is None or self._full_loaded:
            return
        for row in self._store.fetch_all():
            key = (row.item_id, int(row.item_state))
            if key not in self._rows:
                self._rows[key] = row
        self._full_loaded = True

    # ── mutation ──

    def add(self, item_id: str, qty: int | float, state: int = 0) -> int:
        """Grant items. Fractional part is accumulated via `fractional`.

        Returns the number of whole units actually added.
        """
        if qty <= 0:
            return 0
        row = self._ensure(item_id, state)
        total = row.fractional + float(qty)
        whole = int(total)
        row.fractional = total - whole
        if whole != 0:
            row.quantity += whole
            self._journal.append(InventoryEntry(item_id, state, whole))
        self._new_items.add(item_id)
        return whole

    def consume(self, item_id: str, qty: int, state: int = 0) -> None:
        """Deduct items. Raises InsufficientItem if not enough."""
        if qty <= 0:
            return
        row = self._ensure(item_id, state)
        if row.quantity < qty:
            raise InsufficientItem(item_id, state, qty, row.quantity)
        row.quantity -= qty
        self._journal.append(InventoryEntry(item_id, state, -qty))

    # ── queries ──

    def quantity_of(self, item_id: str, state: int = 0) -> int:
        key = (item_id, state)
        row = self._rows.get(key)
        if row is not None:
            return int(row.quantity)
        if self._store is not None and not self._full_loaded:
            loaded = self._store.fetch_one(item_id, state)
            if loaded is not None:
                self._rows[key] = loaded
                return int(loaded.quantity)
        return 0

    def total_quantity(self, item_id: str) -> int:
        self.load_all()
        total = 0
        for (iid, _state), row in self._rows.items():
            if iid == item_id:
                total += int(row.quantity)
        return total

    def snapshot(self) -> dict[tuple[str, int], int]:
        """Return {(item_id, state): quantity} for before/after diffing."""
        self.load_all()
        return {
            key: int(row.quantity)
            for key, row in self._rows.items()
            if row.quantity > 0
        }

    def iter_by_item_id(self, item_id: str) -> Iterator[tuple[int, InventoryRowLike]]:
        self.load_all()
        for (iid, state), row in self._rows.items():
            if iid == item_id:
                yield (state, row)

    def items(self) -> Iterator[tuple[tuple[str, int], InventoryRowLike]]:
        """Iterate over all (key, row) pairs. Loads all if store attached."""
        self.load_all()
        return iter(self._rows.items())

    # ── journal / diff ──

    def flush_journal(self) -> list[InventoryEntry]:
        """Pop and return the change journal, then clear it."""
        entries = self._journal[:]
        self._journal.clear()
        return entries

    def flush_new_items(self) -> set[str]:
        """Pop and return newly-seen item IDs, then clear."""
        items = self._new_items.copy()
        self._new_items.clear()
        return items

    def merge_journal(self, entries: list[InventoryEntry]) -> None:
        """Replay external change entries into this container."""
        for e in entries:
            if e.quantity > 0:
                self.add(e.item_id, e.quantity, e.state)
            elif e.quantity < 0:
                self.consume(e.item_id, -e.quantity, e.state)

    # ── serialization ──

    def to_list(self) -> list[dict[str, int | str]]:
        """Serialize inventory as a list of {id, qty[, state]} entries."""
        self.load_all()
        out: list[dict[str, int | str]] = []
        for (_item_id, state), row in self._rows.items():
            if row.quantity <= 0:
                continue
            entry: dict[str, int | str] = {"id": row.item_id, "qty": int(row.quantity)}
            if state != 0:
                entry["state"] = state
            out.append(entry)
        return out
