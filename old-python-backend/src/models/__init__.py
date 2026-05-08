from sqlalchemy import (
    ForeignKey,
    REAL,
    TEXT,
    INTEGER,
)
from sqlalchemy.orm import (
    DeclarativeBase,
    Mapped,
    mapped_column,
    relationship,
)


class Base(DeclarativeBase):
    pass


class User(Base):
    __tablename__ = "user"

    uid: Mapped[int] = mapped_column(INTEGER, primary_key=True, autoincrement=True)
    username: Mapped[str] = mapped_column(TEXT, unique=True, nullable=False)
    email: Mapped[str] = mapped_column(TEXT, unique=True, nullable=False)
    password_hash: Mapped[str] = mapped_column(TEXT, nullable=False)
    created_at: Mapped[float] = mapped_column(REAL, nullable=False)

    sessions = relationship("UserSession", back_populates="user", cascade="all, delete-orphan")
    player_state = relationship("PlayerState", back_populates="user", uselist=False, cascade="all, delete-orphan")
    inventory_items = relationship("PlayerInventory", back_populates="user", cascade="all, delete-orphan")
    skills = relationship("PlayerSkill", back_populates="user", cascade="all, delete-orphan")
    unlocked_events = relationship("PlayerUnlockedEvent", back_populates="user", cascade="all, delete-orphan")
    event_progress = relationship("PlayerEventProgress", back_populates="user", cascade="all, delete-orphan")
    seen_items = relationship("PlayerSeenItem", back_populates="user", cascade="all, delete-orphan")
    equipment_items = relationship("PlayerEquipment", back_populates="user", cascade="all, delete-orphan")
    tool_items = relationship("PlayerTool", back_populates="user", cascade="all, delete-orphan")
    market_account = relationship("MarketAccount", back_populates="user", uselist=False, cascade="all, delete-orphan")
    market_listings = relationship(
        "MarketListing",
        back_populates="seller",
        cascade="all, delete-orphan",
        foreign_keys="MarketListing.seller_uid",
    )


class UserSession(Base):
    __tablename__ = "user_session"

    token: Mapped[str] = mapped_column(TEXT, primary_key=True)
    uid: Mapped[int] = mapped_column(INTEGER, ForeignKey("user.uid"), nullable=False)
    created_at: Mapped[float] = mapped_column(REAL, nullable=False)
    expires_at: Mapped[float] = mapped_column(REAL, nullable=True)

    user = relationship("User", back_populates="sessions")


class PlayerState(Base):
    __tablename__ = "player_state"

    uid: Mapped[int] = mapped_column(
        INTEGER, ForeignKey("user.uid"), primary_key=True
    )
    queue_json: Mapped[str] = mapped_column(TEXT, nullable=True)
    queue_index: Mapped[int] = mapped_column(INTEGER, nullable=False, default=0)
    queue_progress_seconds: Mapped[float] = mapped_column(
        REAL, nullable=False, default=0.0
    )
    last_sync_time: Mapped[float] = mapped_column(REAL, nullable=False, default=0.0)
    active_battle_id: Mapped[str] = mapped_column(TEXT, nullable=True)

    user = relationship("User", back_populates="player_state")


class ActiveBattle(Base):
    __tablename__ = "active_battle"

    uid: Mapped[int] = mapped_column(
        INTEGER, ForeignKey("user.uid"), primary_key=True
    )
    battle_id: Mapped[str] = mapped_column(TEXT, nullable=False)
    session_json: Mapped[str] = mapped_column(TEXT, nullable=False)
    updated_at: Mapped[float] = mapped_column(REAL, nullable=False, default=0.0)


class PlayerInventory(Base):
    __tablename__ = "player_inventory"

    uid: Mapped[int] = mapped_column(
        INTEGER, ForeignKey("user.uid"), primary_key=True
    )
    item_id: Mapped[str] = mapped_column(TEXT, primary_key=True)
    item_state: Mapped[int] = mapped_column(INTEGER, primary_key=True, default=0)
    quantity: Mapped[int] = mapped_column(INTEGER, nullable=False, default=0)
    fractional: Mapped[float] = mapped_column(REAL, nullable=False, default=0.0)

    user = relationship("User", back_populates="inventory_items")


class PlayerSkill(Base):
    __tablename__ = "player_skill"

    uid: Mapped[int] = mapped_column(
        INTEGER, ForeignKey("user.uid"), primary_key=True
    )
    skill_id: Mapped[str] = mapped_column(TEXT, primary_key=True)
    level: Mapped[int] = mapped_column(INTEGER, nullable=False, default=1)
    exp: Mapped[float] = mapped_column(REAL, nullable=False, default=0.0)

    user = relationship("User", back_populates="skills")


class PlayerUnlockedEvent(Base):
    __tablename__ = "player_unlocked_event"

    uid: Mapped[int] = mapped_column(
        INTEGER, ForeignKey("user.uid"), primary_key=True
    )
    event_id: Mapped[str] = mapped_column(TEXT, primary_key=True)

    user = relationship("User", back_populates="unlocked_events")


class PlayerEventProgress(Base):
    __tablename__ = "player_event_progress"

    uid: Mapped[int] = mapped_column(
        INTEGER, ForeignKey("user.uid"), primary_key=True
    )
    event_id: Mapped[str] = mapped_column(TEXT, primary_key=True)
    completed_count: Mapped[int] = mapped_column(INTEGER, nullable=False, default=0)

    user = relationship("User", back_populates="event_progress")


class PlayerSeenItem(Base):
    __tablename__ = "player_seen_item"

    uid: Mapped[int] = mapped_column(
        INTEGER, ForeignKey("user.uid"), primary_key=True
    )
    item_id: Mapped[str] = mapped_column(TEXT, primary_key=True)
    first_seen_at: Mapped[float] = mapped_column(REAL, nullable=False, default=0.0)

    user = relationship("User", back_populates="seen_items")


class PlayerEquipment(Base):
    __tablename__ = "player_equipment"

    uid: Mapped[int] = mapped_column(
        INTEGER, ForeignKey("user.uid"), primary_key=True
    )
    slot: Mapped[str] = mapped_column(TEXT, primary_key=True)
    item_id: Mapped[str] = mapped_column(TEXT, nullable=False)
    anchor_slot: Mapped[str] = mapped_column(TEXT, nullable=True)
    enhance_level: Mapped[int] = mapped_column(INTEGER, nullable=False, default=0)
    enhance_fail_count: Mapped[int] = mapped_column(INTEGER, nullable=False, default=0)

    user = relationship("User", back_populates="equipment_items")


class PlayerTool(Base):
    __tablename__ = "player_tool"

    uid: Mapped[int] = mapped_column(
        INTEGER, ForeignKey("user.uid"), primary_key=True
    )
    slot: Mapped[str] = mapped_column(TEXT, primary_key=True)
    item_id: Mapped[str] = mapped_column(TEXT, nullable=False)
    anchor_slot: Mapped[str] = mapped_column(TEXT, nullable=True)
    enhance_level: Mapped[int] = mapped_column(INTEGER, nullable=False, default=0)
    enhance_fail_count: Mapped[int] = mapped_column(INTEGER, nullable=False, default=0)

    user = relationship("User", back_populates="tool_items")


class MarketAccount(Base):
    __tablename__ = "market_account"

    uid: Mapped[int] = mapped_column(INTEGER, ForeignKey("user.uid"), primary_key=True)
    currency_code: Mapped[str] = mapped_column(TEXT, nullable=False)
    balance: Mapped[int] = mapped_column(INTEGER, nullable=False, default=0)
    updated_at: Mapped[float] = mapped_column(REAL, nullable=False, default=0.0)

    user = relationship("User", back_populates="market_account")


class MarketListing(Base):
    __tablename__ = "market_listing"

    listing_id: Mapped[int] = mapped_column(INTEGER, primary_key=True, autoincrement=True)
    seller_uid: Mapped[int | None] = mapped_column(INTEGER, ForeignKey("user.uid"), nullable=True)
    seller_name: Mapped[str] = mapped_column(TEXT, nullable=False)
    item_id: Mapped[str] = mapped_column(TEXT, nullable=False)
    quantity_total: Mapped[int] = mapped_column(INTEGER, nullable=False)
    quantity_available: Mapped[int] = mapped_column(INTEGER, nullable=False)
    unit_price: Mapped[int] = mapped_column(INTEGER, nullable=False)
    currency_code: Mapped[str] = mapped_column(TEXT, nullable=False)
    status: Mapped[str] = mapped_column(TEXT, nullable=False, default="active")
    created_at: Mapped[float] = mapped_column(REAL, nullable=False, default=0.0)
    updated_at: Mapped[float] = mapped_column(REAL, nullable=False, default=0.0)
    expires_at: Mapped[float | None] = mapped_column(REAL, nullable=True)

    seller = relationship("User", back_populates="market_listings", foreign_keys=[seller_uid])


class MarketTradeRecord(Base):
    __tablename__ = "market_trade_record"

    trade_id: Mapped[int] = mapped_column(INTEGER, primary_key=True, autoincrement=True)
    listing_id: Mapped[int] = mapped_column(INTEGER, ForeignKey("market_listing.listing_id"), nullable=False)
    seller_uid: Mapped[int | None] = mapped_column(INTEGER, ForeignKey("user.uid"), nullable=True)
    seller_name: Mapped[str] = mapped_column(TEXT, nullable=False)
    buyer_uid: Mapped[int] = mapped_column(INTEGER, ForeignKey("user.uid"), nullable=False)
    buyer_name: Mapped[str] = mapped_column(TEXT, nullable=False)
    item_id: Mapped[str] = mapped_column(TEXT, nullable=False)
    quantity: Mapped[int] = mapped_column(INTEGER, nullable=False)
    unit_price: Mapped[int] = mapped_column(INTEGER, nullable=False)
    total_price: Mapped[int] = mapped_column(INTEGER, nullable=False)
    currency_code: Mapped[str] = mapped_column(TEXT, nullable=False)
    status: Mapped[str] = mapped_column(TEXT, nullable=False, default="settled")
    created_at: Mapped[float] = mapped_column(REAL, nullable=False, default=0.0)

    listing = relationship("MarketListing")
    seller = relationship("User", foreign_keys=[seller_uid])
    buyer = relationship("User", foreign_keys=[buyer_uid])
