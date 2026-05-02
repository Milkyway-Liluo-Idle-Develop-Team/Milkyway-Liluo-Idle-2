# actions.json 格式说明

`actions.json` 是游戏核心数据配置文件，定义了所有**物品（items）**和**事件/行动（events）**的元数据。后端结算引擎和前端 UI 均依赖此文件。

---

## 根结构

```json
{
    "items": [ /* Item 数组 */ ],
    "events": [ /* Event 数组 */ ]
}
```

---

## Item 对象

表示游戏中可获取、可使用的物品。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 唯一标识符，如 `"oak_logs"` |
| `name` | string | 是 | 显示名称 |
| `tool` | bool | 是 | 是否为工具 |
| `equipment` | bool | 是 | 是否为装备 |
| `upgradable` | bool | 是 | 是否可升级/强化 |
| `classification` | string | 是 | 分类：`"resources"` / `"equipment"` / ... |
| `tool_details` | object | 否 | `tool == true` 时必填，见下方 |
| `equipment_details` | object | 否 | `equipment == true` 时必填，见下方 |
| `upgrade_details` | object | 否 | `upgradable == true` 时必填，见下方 |

### tool_details

| 字段 | 类型 | 说明 |
|------|------|------|
| `tool_position_requirement` | array | 工具生效位置及数值要求 |
| `skills` | null / array | 关联技能 |
| `tool_basic_data` | object | 工具对产出的加成系数等 |
| `tool_type` | string | 工具类型，如 `"axe"` |
| `requirements` | null / array | 装备该工具的前置条件 |

### equipment_details

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 装备类型，如 `"weapon"` |
| `equipment_position_requirements` | array | 装备位置要求 |
| `element` | string | 元素属性，如 `"physical"` |
| `skills` | null / array | 关联技能 |
| `equipment_basic_data` | object | 基础数值（伤害、命中、攻速等） |
| `requirements` | null / array | 穿戴要求 |

### upgrade_details

| 字段 | 类型 | 说明 |
|------|------|------|
| `max_upgrade` | int | 最大升级等级 |
| `enhance_slot` | int | 强化槽位数 |
| `forge_slot` | int | 锻造槽位数 |
| `upgrade_curve` | array | 每阶段的推荐等级、成功率、属性倍率 |

---

## Event 对象

表示玩家可以执行的行动或剧情节点。后端结算引擎根据 `type` 字段区分处理方式。

### 公共字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 唯一标识符 |
| `type` | string | 是 | 事件类型：`"loop"` / `"instant"` / `"upgrade"` |
| `name` | string | 是 | 显示名称 |
| `description` | string | 是 | 描述文本 |
| `need_skill` | string | 是 | 关联技能 ID，`"none"` 表示无 |
| `requirements` | null / array | 是 | 执行前提条件，见下方 Requirement 格式 |
| `experience` | number | 是 | 完成后获得的经验值 |
| `map` | string | 是 | 所属地图/场景 |

### type = "loop" 特有字段

循环行动，玩家放入队列后按 `loop_time` 周期性执行。

| 字段 | 类型 | 说明 |
|------|------|------|
| `loop_time` | number | 每次循环所需秒数 |
| `rewards` | array | 每次循环的产出，见 Reward 格式 |

### type = "instant" 特有字段

即时行动，执行一次立即生效（不需要在队列中循环）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `rewards` | array | 执行后的产出 |

### type = "upgrade" 特有字段

升级/解锁行动，执行一次后永久解锁该事件 ID。

- 无额外特有字段。
- 解锁后该 `event_id` 会进入玩家的 `unlocked_events` 集合。

---

## Requirement 对象

用于 `requirements` 数组，描述技能、物品、流体或事件的门槛/消耗。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | `"skill"` / `"item"` / `"fluid"` / `"event"` |
| `id` | string | 是 | 对应资源的 ID |
| `comparison_types` | string | 否 | 比较运算符：`"bigger"` / `"equal"` / `"smaller"` / `"bigger_or_equal"` / `"smaller_or_equal"` |
| `value` | number | 否 | 比较/消耗数值 |

### 结算规则

- **`item` / `fluid` 且 `comparison_types` 为 `null`**：视为**消耗**。执行时会从玩家库存/流体中扣除 `value` 数量。
- **其他情况**：视为**门槛检查**。满足条件即可执行，不会扣除。

---

## Reward 对象

用于 `rewards` 数组。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 物品 ID |
| `num` | number | 是 | 产出数量（优先读取） |
| `value` | number | 否 | 兼容字段，`num` 为 null 时回退读取 |

---

## 后端读取方式

- 启动时不预加载整个文件。
- `game/settlement.py` 中的 `get_events_map()` 首次调用时懒加载并缓存到内存中的全局变量 `_events_map`。
- 事件查找通过 `events_map[event_id]` 进行 O(1) 访问。
