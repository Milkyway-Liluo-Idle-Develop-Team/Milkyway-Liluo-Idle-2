"""
RPG 属性系统核心模块。

设计思路：Modifier 收集 + 分层聚合。
- Modifier: 来自装备、工具、buff 等来源的属性修饰器
- AttributeSet: 一次构建，O(1) 查询最终属性值
- collect_modifiers: 从各来源收集所有 modifier

最终公式: final = (base + Σflat) × (1 + Σpercent_add) × Π(1 + percent_mult_i)
"""

import json
import csv
from dataclasses import dataclass

from data.data import DataManager

_data_manager = DataManager()

# ---------------------------------------------------------------------------
# items_map 缓存（与 settlement.py 的 _events_map 同模式）
# ---------------------------------------------------------------------------
_items_map: dict[str, dict] | None = None
_level_production_rows: list[float] | None = None
PRODUCTION_SKILLS: tuple[str, ...] = (
    "felling",
    "mining",
    "planting",
    "crafting",
    "forging",
    "enhancing",
)


def get_items_map() -> dict[str, dict]:
    global _items_map
    if _items_map is None:
        with open(_data_manager.actions, "r", encoding="utf-8") as f:
            actions = json.load(f)
        _items_map = {i["id"]: i for i in actions.get("items", [])}
    return _items_map


def _load_level_production_rows() -> list[float]:
    global _level_production_rows
    if _level_production_rows is not None:
        return _level_production_rows

    rows: list[tuple[int, float]] = []
    with open(_data_manager.level_production, "r", encoding="utf-8") as f:
        reader = csv.reader(f)
        for row in reader:
            if len(row) < 2:
                continue
            try:
                level = int(row[0].strip())
                mult = float(row[1].strip())
            except (TypeError, ValueError):
                continue
            if level <= 0:
                continue
            rows.append((level, mult))

    rows.sort(key=lambda x: x[0])
    if not rows:
        _level_production_rows = [1.0]
        return _level_production_rows

    max_level = rows[-1][0]
    table = [1.0] * max_level
    for level, mult in rows:
        table[level - 1] = mult
    _level_production_rows = table
    return _level_production_rows


def get_level_production_multiplier(level: int) -> float:
    """按等级返回循环产出倍率（CSV 第二列）。"""
    lv = max(1, int(level))
    table = _load_level_production_rows()
    if lv <= len(table):
        return float(table[lv - 1])

    if len(table) >= 2 and table[-2] > 0:
        growth_ratio = table[-1] / table[-2]
    else:
        growth_ratio = 1.003
    multiplier = float(table[-1] * (growth_ratio ** (lv - len(table))))
    return min(multiplier, 1_000_000.0)


def _to_float(value, default: float = 0.0) -> float:
    try:
        return float(value)
    except (TypeError, ValueError):
        return default


def _to_int(value, default: int | None = 0) -> int | None:
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def resolve_item_ability_multiplier(item_data: dict, enhance_level: int) -> float:
    """按强化等级读取 upgrade_curve 的 ability_multiplier（线性插值）。"""
    details = item_data.get("upgrade_details") or {}
    curve = details.get("upgrade_curve") or []

    # 默认 0 级能力倍数为 1.0
    points_map: dict[int, float] = {0: 1.0}
    for row in curve:
        if not isinstance(row, dict):
            continue
        if row.get("ability_multiplier") is None:
            continue
        level = _to_int(row.get("level"), None)
        if level is None:
            continue
        points_map[level] = _to_float(row.get("ability_multiplier"), 1.0)

    if not points_map:
        return 1.0

    points = sorted(points_map.items(), key=lambda x: x[0])
    lv = max(0, int(enhance_level))
    if lv <= points[0][0]:
        return float(points[0][1])

    for idx in range(1, len(points)):
        prev_lv, prev_val = points[idx - 1]
        cur_lv, cur_val = points[idx]
        if lv > cur_lv:
            continue
        if cur_lv == prev_lv:
            return float(cur_val)
        ratio = (lv - prev_lv) / (cur_lv - prev_lv)
        return float(prev_val + (cur_val - prev_val) * ratio)

    return float(points[-1][1])


def compute_scaled_item_data(
    item_data: dict,
    slot_type: str,
    enhance_level: int,
) -> dict[str, float]:
    """统一装备/工具属性计算：基础 + 强化增量 * 强化倍率。"""
    if slot_type == "equipment":
        details = item_data.get("equipment_details") or {}
        basic_key = "equipment_basic_data"
        upgrade_key = "equipment_upgrade_data"
    elif slot_type == "tool":
        details = item_data.get("tool_details") or {}
        basic_key = "tool_basic_data"
        upgrade_key = "tool_upgrade_data"
    else:
        return {}

    basic_data = details.get(basic_key) or {}
    upgrade_data = details.get(upgrade_key) or {}
    ability = resolve_item_ability_multiplier(item_data, enhance_level)

    out: dict[str, float] = {}
    keys = set(basic_data.keys()) | set(upgrade_data.keys())
    for attr in keys:
        base_val = _to_float(basic_data.get(attr), 0.0)
        inc_val = _to_float(upgrade_data.get(attr), 0.0)
        value = base_val + inc_val * ability
        if abs(value) < 1e-12:
            continue
        out[str(attr)] = float(value)
    return out


# ---------------------------------------------------------------------------
# Modifier
# ---------------------------------------------------------------------------
@dataclass(slots=True)
class Modifier:
    """属性修饰器。

    attribute: 属性 ID，如 "physical_damage"
    value:     数值
    mod_type:  "flat" | "percent_add" | "percent_mult"
    source:    来源标识（调试用），如 "equip:wooden_sword"
    """
    attribute: str
    value: float
    mod_type: str
    source: str = ""


# ---------------------------------------------------------------------------
# AttributeSet
# ---------------------------------------------------------------------------
class AttributeSet:
    """收集 modifier 并计算最终属性值。一次构建，多次查询。"""

    __slots__ = ("_finals",)

    def __init__(self, base_values: dict[str, float], modifiers: list[Modifier]):
        self._finals: dict[str, float] = _compute(base_values, modifiers)

    def get(self, attr: str, default: float = 0.0) -> float:
        """O(1) 查询最终属性值。"""
        return self._finals.get(attr, default)

    def to_dict(self) -> dict[str, float]:
        """返回所有最终属性的副本，用于序列化。"""
        return dict(self._finals)


def _compute(
    base_values: dict[str, float],
    modifiers: list[Modifier],
) -> dict[str, float]:
    """单次遍历 modifiers，按属性分组聚合，计算最终值。"""
    flat: dict[str, float] = {}
    pct_add: dict[str, float] = {}
    pct_mult: dict[str, float] = {}

    for m in modifiers:
        a = m.attribute
        if m.mod_type == "flat":
            flat[a] = flat.get(a, 0.0) + m.value
        elif m.mod_type == "percent_add":
            pct_add[a] = pct_add.get(a, 0.0) + m.value
        elif m.mod_type == "percent_mult":
            if a not in pct_mult:
                pct_mult[a] = 1.0
            pct_mult[a] *= (1.0 + m.value)

    all_attrs = set(base_values) | set(flat) | set(pct_add) | set(pct_mult)
    result: dict[str, float] = {}
    for attr in all_attrs:
        base = base_values.get(attr, 0.0)
        v = (base + flat.get(attr, 0.0)) \
            * (1.0 + pct_add.get(attr, 0.0)) \
            * pct_mult.get(attr, 1.0)
        result[attr] = v
    return result


# ---------------------------------------------------------------------------
# Modifier 收集器
# ---------------------------------------------------------------------------
def collect_modifiers(
    items_map: dict[str, dict],
    equipment: dict,  # slot -> PlayerEquipment ORM 对象
    tools: dict,      # slot -> PlayerTool ORM 对象
) -> list[Modifier]:
    """从所有来源收集 modifier 列表。

    未来扩展点：在此函数签名中添加 skills, buffs 等参数。
    """
    mods: list[Modifier] = []
    seen_equipment_pieces: set[tuple[str, str]] = set()
    seen_tool_pieces: set[tuple[str, str]] = set()

    # 装备 → flat modifier
    for slot, equip in equipment.items():
        anchor = getattr(equip, "anchor_slot", None) or slot
        piece_key = (anchor, equip.item_id)
        if piece_key in seen_equipment_pieces:
            continue
        seen_equipment_pieces.add(piece_key)
        item_data = items_map.get(equip.item_id)
        if not item_data or not item_data.get("equipment"):
            continue
        enhance_level = max(0, _to_int(getattr(equip, "enhance_level", 0), 0))
        scaled_data = compute_scaled_item_data(item_data, "equipment", enhance_level)
        for attr, value in scaled_data.items():
            mods.append(Modifier(
                attribute=attr,
                value=float(value),
                mod_type="flat",
                source=f"equip:{equip.item_id}@{enhance_level}",
            ))

    # 工具 → flat modifier（属性值本身即“加成值”，如 0.15 代表 +15%）
    for slot, tool in tools.items():
        anchor = getattr(tool, "anchor_slot", None) or slot
        piece_key = (anchor, tool.item_id)
        if piece_key in seen_tool_pieces:
            continue
        seen_tool_pieces.add(piece_key)
        item_data = items_map.get(tool.item_id)
        if not item_data or not item_data.get("tool"):
            continue
        enhance_level = max(0, _to_int(getattr(tool, "enhance_level", 0), 0))
        scaled_data = compute_scaled_item_data(item_data, "tool", enhance_level)
        for attr, value in scaled_data.items():
            mods.append(Modifier(
                attribute=attr,
                value=float(value),
                mod_type="flat",
                source=f"tool:{tool.item_id}@{enhance_level}",
            ))

    return mods
