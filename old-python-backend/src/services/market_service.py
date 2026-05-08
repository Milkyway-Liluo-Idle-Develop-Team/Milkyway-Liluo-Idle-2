import json
import os
import threading
import time
from datetime import datetime, timezone

from sqlalchemy import func, select

from data.data import DataManager
from game.context import PlayerContext
from game.inventory import InsufficientItem
from models import (
    MarketAccount,
    MarketListing,
    MarketTradeRecord,
    User,
)
from models import database

MARKET_CURRENCY_CODE = os.environ.get("MARKET_CURRENCY_CODE", "coin")
MARKET_CURRENCY_NAME = os.environ.get("MARKET_CURRENCY_NAME", "金币")
MARKET_CURRENCY_ICON = os.environ.get("MARKET_CURRENCY_ICON", "🪙")
MARKET_CURRENCY_PRECISION = int(os.environ.get("MARKET_CURRENCY_PRECISION", "0"))
MARKET_DEFAULT_BALANCE = int(os.environ.get("MARKET_DEFAULT_BALANCE", "1000"))
MARKET_LISTING_TTL_SECONDS = int(os.environ.get("MARKET_LISTING_TTL_SECONDS", str(7 * 86400)))
MARKET_RECENT_TRADE_LIMIT = int(os.environ.get("MARKET_RECENT_TRADE_LIMIT", "12"))
MARKET_ACTIVE_LISTING_LIMIT = int(os.environ.get("MARKET_ACTIVE_LISTING_LIMIT", "24"))

_MARKET_LOCK = threading.RLock()
_data_manager = DataManager()
_item_name_map: dict[str, str] | None = None

_DEMO_LISTINGS: tuple[dict[str, int | str], ...] = (
    {"seller_name": "Aster", "item_id": "oak_logs", "quantity": 120, "unit_price": 6},
    {"seller_name": "Nemu", "item_id": "oak_plank", "quantity": 80, "unit_price": 14},
    {"seller_name": "Shion", "item_id": "wooden_stick", "quantity": 60, "unit_price": 18},
    {"seller_name": "Mio", "item_id": "wooden_sword", "quantity": 8, "unit_price": 140},
    {"seller_name": "Ruri", "item_id": "wooden_staff", "quantity": 5, "unit_price": 168},
)


def _now_iso(ts: float | None = None) -> str:
    value = ts if ts is not None else time.time()
    return datetime.fromtimestamp(value, tz=timezone.utc).isoformat()


def _load_item_name_map() -> dict[str, str]:
    global _item_name_map
    if _item_name_map is not None:
        return _item_name_map
    with open(_data_manager.actions, "r", encoding="utf-8") as f:
        data = json.load(f)
    _item_name_map = {
        str(item.get("id") or ""): str(item.get("name") or item.get("id") or "")
        for item in data.get("items", [])
        if item.get("id")
    }
    return _item_name_map


def _item_name(item_id: str) -> str:
    return _load_item_name_map().get(item_id, item_id)


def _get_user(session, uid: int) -> User:
    user = session.execute(select(User).where(User.uid == uid)).scalar_one_or_none()
    if user is None:
        raise ValueError("用户不存在")
    return user


def _expire_old_listings(session, now_ts: float) -> None:
    rows = session.execute(
        select(MarketListing).where(
            MarketListing.status == "active",
            MarketListing.expires_at.is_not(None),
            MarketListing.expires_at <= now_ts,
        )
    ).scalars().all()
    for row in rows:
        row.status = "expired"
        row.updated_at = now_ts


def ensure_market_account(session, uid: int) -> MarketAccount:
    account = session.execute(
        select(MarketAccount).where(MarketAccount.uid == uid)
    ).scalar_one_or_none()
    if account is None:
        account = MarketAccount(
            uid=uid,
            currency_code=MARKET_CURRENCY_CODE,
            balance=MARKET_DEFAULT_BALANCE,
            updated_at=time.time(),
        )
        session.add(account)
    elif account.currency_code != MARKET_CURRENCY_CODE:
        account.currency_code = MARKET_CURRENCY_CODE
        account.updated_at = time.time()
    return account


def seed_demo_listings(session) -> None:
    now_ts = time.time()
    existing = session.execute(
        select(MarketListing).where(
            MarketListing.seller_uid.is_(None),
            MarketListing.status == "active",
            MarketListing.quantity_available > 0,
        )
    ).scalars().first()
    if existing is not None:
        return

    item_map = _load_item_name_map()
    for seed in _DEMO_LISTINGS:
        item_id = str(seed["item_id"])
        if item_id not in item_map:
            continue
        session.add(
            MarketListing(
                seller_uid=None,
                seller_name=str(seed["seller_name"]),
                item_id=item_id,
                quantity_total=int(seed["quantity"]),
                quantity_available=int(seed["quantity"]),
                unit_price=int(seed["unit_price"]),
                currency_code=MARKET_CURRENCY_CODE,
                status="active",
                created_at=now_ts,
                updated_at=now_ts,
                expires_at=now_ts + MARKET_LISTING_TTL_SECONDS,
            )
        )


def _serialize_listing(listing: MarketListing, viewer_uid: int) -> dict:
    quantity_available = max(0, int(listing.quantity_available))
    unit_price = max(0, int(listing.unit_price))
    return {
        "id": int(listing.listing_id),
        "sellerUid": int(listing.seller_uid) if listing.seller_uid is not None else None,
        "sellerName": listing.seller_name,
        "itemId": listing.item_id,
        "itemName": _item_name(listing.item_id),
        "quantityTotal": int(listing.quantity_total),
        "quantityAvailable": quantity_available,
        "unitPrice": unit_price,
        "totalPrice": quantity_available * unit_price,
        "currencyCode": listing.currency_code,
        "status": listing.status,
        "listedAt": _now_iso(listing.created_at),
        "expiresAt": _now_iso(listing.expires_at) if listing.expires_at else None,
        "sellerIsSelf": listing.seller_uid == viewer_uid,
    }


def _serialize_trade(record: MarketTradeRecord, viewer_uid: int) -> dict:
    if record.buyer_uid == viewer_uid:
        trade_type = "buy"
        counterpart = record.seller_name
        counterpart_uid = record.seller_uid
    elif record.seller_uid == viewer_uid:
        trade_type = "sell"
        counterpart = record.buyer_name
        counterpart_uid = record.buyer_uid
    else:
        trade_type = "market"
        counterpart = f"{record.seller_name} → {record.buyer_name}"
        counterpart_uid = record.buyer_uid if record.buyer_uid is not None else record.seller_uid
    return {
        "id": int(record.trade_id),
        "listingId": int(record.listing_id),
        "sellerUid": int(record.seller_uid) if record.seller_uid is not None else None,
        "buyerUid": int(record.buyer_uid) if record.buyer_uid is not None else None,
        "sellerName": record.seller_name,
        "buyerName": record.buyer_name,
        "itemId": record.item_id,
        "itemName": _item_name(record.item_id),
        "quantity": int(record.quantity),
        "unitPrice": int(record.unit_price),
        "totalPrice": int(record.total_price),
        "currencyCode": record.currency_code,
        "status": record.status,
        "createdAt": _now_iso(record.created_at),
        "type": trade_type,
        "counterpart": counterpart,
        "counterpartUid": int(counterpart_uid) if counterpart_uid is not None else None,
    }


def get_market_snapshot(uid: int) -> dict:
    with _MARKET_LOCK:
        session = database.get_db()
        now_ts = time.time()
        _expire_old_listings(session, now_ts)
        account = ensure_market_account(session, uid)
        seed_demo_listings(session)
        session.commit()

        listings = session.execute(
            select(MarketListing)
            .where(
                MarketListing.status == "active",
                MarketListing.quantity_available > 0,
            )
            .order_by(MarketListing.updated_at.desc(), MarketListing.listing_id.desc())
            .limit(MARKET_ACTIVE_LISTING_LIMIT)
        ).scalars().all()

        my_listings = session.execute(
            select(MarketListing)
            .where(
                MarketListing.seller_uid == uid,
                MarketListing.status == "active",
                MarketListing.quantity_available > 0,
            )
            .order_by(MarketListing.updated_at.desc(), MarketListing.listing_id.desc())
        ).scalars().all()

        trade_records = session.execute(
            select(MarketTradeRecord)
            .order_by(MarketTradeRecord.created_at.desc(), MarketTradeRecord.trade_id.desc())
            .limit(MARKET_RECENT_TRADE_LIMIT)
        ).scalars().all()

        active_count = session.execute(
            select(func.count()).select_from(MarketListing).where(
                MarketListing.status == "active",
                MarketListing.quantity_available > 0,
            )
        ).scalar_one()

        volume_24h = session.execute(
            select(func.coalesce(func.sum(MarketTradeRecord.total_price), 0)).where(
                MarketTradeRecord.status == "settled",
                MarketTradeRecord.created_at >= now_ts - 86400,
                MarketTradeRecord.currency_code == MARKET_CURRENCY_CODE,
            )
        ).scalar_one()

        return {
            "currency": {
                "code": MARKET_CURRENCY_CODE,
                "name": MARKET_CURRENCY_NAME,
                "icon": MARKET_CURRENCY_ICON,
                "precision": MARKET_CURRENCY_PRECISION,
            },
            "walletBalance": int(account.balance),
            "activeListings": int(active_count or 0),
            "totalVolume24h": int(volume_24h or 0),
            "lastUpdatedAt": _now_iso(now_ts),
            "listings": [_serialize_listing(row, uid) for row in listings],
            "myListings": [_serialize_listing(row, uid) for row in my_listings],
            "tradeRecords": [_serialize_trade(row, uid) for row in trade_records],
        }


def create_listing(uid: int, item_id: str, quantity: int, unit_price: int) -> dict:
    with _MARKET_LOCK:
        session = database.get_db()
        now_ts = time.time()
        _expire_old_listings(session, now_ts)
        _get_user(session, uid)
        ensure_market_account(session, uid)

        item_id = str(item_id or "").strip()
        if not item_id or item_id not in _load_item_name_map():
            raise ValueError("无效的物品")
        try:
            quantity = int(quantity)
            unit_price = int(unit_price)
        except (TypeError, ValueError):
            raise ValueError("数量和单价必须为整数")
        if quantity <= 0:
            raise ValueError("上架数量必须大于 0")
        if unit_price <= 0:
            raise ValueError("单价必须大于 0")

        ctx = PlayerContext.load(session, uid)
        if ctx.inventory.quantity_of(item_id) < quantity:
            raise ValueError("库存不足，无法上架")
        try:
            ctx.inventory.consume(item_id, quantity)
        except InsufficientItem:
            raise ValueError("库存不足，无法上架")

        seller = _get_user(session, uid)
        listing = MarketListing(
            seller_uid=uid,
            seller_name=seller.username,
            item_id=item_id,
            quantity_total=quantity,
            quantity_available=quantity,
            unit_price=unit_price,
            currency_code=MARKET_CURRENCY_CODE,
            status="active",
            created_at=now_ts,
            updated_at=now_ts,
            expires_at=now_ts + MARKET_LISTING_TTL_SECONDS,
        )
        session.add(listing)
        session.commit()
        session.refresh(listing)
        return {"listing": _serialize_listing(listing, uid)}


def buy_listing(uid: int, listing_id: int, quantity: int) -> dict:
    with _MARKET_LOCK:
        session = database.get_db()
        now_ts = time.time()
        _expire_old_listings(session, now_ts)
        buyer = _get_user(session, uid)
        buyer_account = ensure_market_account(session, uid)

        try:
            listing_id = int(listing_id)
            quantity = int(quantity)
        except (TypeError, ValueError):
            raise ValueError("挂单编号和数量必须为整数")
        if quantity <= 0:
            raise ValueError("购买数量必须大于 0")

        listing = session.execute(
            select(MarketListing).where(MarketListing.listing_id == listing_id)
        ).scalar_one_or_none()
        if listing is None:
            raise ValueError("挂单不存在")
        if listing.status != "active" or int(listing.quantity_available) <= 0:
            raise ValueError("该挂单已不可购买")
        if listing.expires_at is not None and float(listing.expires_at) <= now_ts:
            listing.status = "expired"
            listing.updated_at = now_ts
            session.commit()
            raise ValueError("该挂单已过期")
        if listing.seller_uid == uid:
            raise ValueError("不能购买自己的挂单")
        if int(listing.quantity_available) < quantity:
            raise ValueError("挂单余量不足")

        total_price = int(listing.unit_price) * quantity
        if int(buyer_account.balance) < total_price:
            raise ValueError(f"{MARKET_CURRENCY_NAME}不足")

        buyer_account.balance = int(buyer_account.balance) - total_price
        buyer_account.updated_at = now_ts
        if listing.seller_uid is not None:
            seller_account = ensure_market_account(session, int(listing.seller_uid))
            seller_account.balance = int(seller_account.balance) + total_price
            seller_account.updated_at = now_ts

        ctx = PlayerContext.load(session, uid)
        ctx.inventory.add(listing.item_id, quantity)
        listing.quantity_available = int(listing.quantity_available) - quantity
        listing.updated_at = now_ts
        if int(listing.quantity_available) <= 0:
            listing.quantity_available = 0
            listing.status = "sold_out"

        trade = MarketTradeRecord(
            listing_id=int(listing.listing_id),
            seller_uid=listing.seller_uid,
            seller_name=listing.seller_name,
            buyer_uid=uid,
            buyer_name=buyer.username,
            item_id=listing.item_id,
            quantity=quantity,
            unit_price=int(listing.unit_price),
            total_price=total_price,
            currency_code=MARKET_CURRENCY_CODE,
            status="settled",
            created_at=now_ts,
        )
        session.add(trade)
        session.commit()
        session.refresh(trade)
        return {"trade": _serialize_trade(trade, uid)}


def cancel_listing(uid: int, listing_id: int) -> dict:
    with _MARKET_LOCK:
        session = database.get_db()
        now_ts = time.time()
        _expire_old_listings(session, now_ts)
        ensure_market_account(session, uid)

        try:
            listing_id = int(listing_id)
        except (TypeError, ValueError):
            raise ValueError("挂单编号必须为整数")

        listing = session.execute(
            select(MarketListing).where(MarketListing.listing_id == listing_id)
        ).scalar_one_or_none()
        if listing is None or listing.seller_uid != uid:
            raise ValueError("挂单不存在或无权撤销")
        if listing.status != "active" or int(listing.quantity_available) <= 0:
            raise ValueError("该挂单当前不可撤销")

        remaining = int(listing.quantity_available)
        ctx = PlayerContext.load(session, uid)
        ctx.inventory.add(listing.item_id, remaining)
        listing.quantity_available = 0
        listing.status = "cancelled"
        listing.updated_at = now_ts
        session.commit()
        return {"listingId": listing_id, "restoredQuantity": remaining}
