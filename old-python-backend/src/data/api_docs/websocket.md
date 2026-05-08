# WebSocket API 文档

## 连接

### 端点
```
ws://localhost:26411/ws
```

### 认证
WebSocket 连接通过 Cookie 进行认证。客户端需要在连接时携带 `token` Cookie。

```javascript
// 浏览器端示例
const ws = new WebSocket("ws://localhost:26411/ws");
// Cookie 由浏览器自动发送

// Python 示例
import websocket
ws = websocket.create_connection("ws://localhost:26411/ws", cookie="token=YOUR_TOKEN")
```

### 认证失败响应
如果 token 缺失或无效，服务器会发送错误消息：
```json
{"type": "error", "message": "Missing token cookie"}
```
或
```json
{"type": "error", "message": "Invalid or expired token"}
```

### 多连接处理
同一用户同时只能有一个活跃的 WebSocket 连接。当新连接建立时，旧连接会收到错误消息并被关闭：
```json
{"type": "error", "message": "Another connection has been established"}
```

---

## 消息格式

所有消息使用 JSON 格式。每条消息必须包含 `type` 字段标识消息类型。

### 客户端 → 服务器

#### 1. sync - 同步状态

触发离线结算并返回完整的玩家状态。

**请求：**
```json
{
    "type": "sync"
}
```

**响应：** `state` 消息（见下方）

**使用场景：** 客户端首次连接后、需要刷新状态时。

---

#### 2. set_queue - 设置行动队列

设置新的行动队列。会先结算旧队列的所有进度，然后替换为新队列。

**请求：**
```json
{
    "type": "set_queue",
    "queue": ["felling_oak_tree", "making_oak_plank"]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| queue | string[] | 行动 ID 列表，按执行顺序排列 |

**响应：** `state` 消息

**错误情况：**
- `queue` 不是数组 → `{"type": "error", "message": "queue must be a list"}`
- 包含无效的行动 ID → `ValueError`

---

#### 3. instant - 执行即时行动

执行一个即时类型的行动。

**请求：**
```json
{
    "type": "instant",
    "event_id": "some_instant_action"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| event_id | string | 即时行动的 ID |

**响应：** `state` 消息

**错误情况：**
- `event_id` 缺失 → `{"type": "error", "message": "event_id required"}`
- 行动不存在 → `ValueError`
- 行动不是 instant 类型 → `ValueError`
- 需求不满足 → `ValueError`

---

#### 4. upgrade - 执行升级行动

执行一个升级类型的行动（解锁里程碑）。

**请求：**
```json
{
    "type": "upgrade",
    "event_id": "starting_dialog_1"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| event_id | string | 升级行动的 ID |

**响应：** `state` 消息

**错误情况：**
- `event_id` 缺失 → `{"type": "error", "message": "event_id required"}`
- 行动不存在 → `ValueError`
- 行动不是 upgrade 类型 → `ValueError`
- 已经解锁过 → `ValueError`
- 需求不满足 → `ValueError`

---

#### 5. equip - 穿戴装备/工具

将背包中的一件装备或工具穿戴到指定槽位。

**请求：**
```json
{
    "type": "equip",
    "item_id": "wooden_sword",
    "slot": "main_hand"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| item_id | string | 物品 ID |
| slot | string | 目标槽位 |

**合法装备槽位：** `main_hand`, `head`, `chest`, `leg`, `feet`
**合法工具槽位：** `felling`, `mining`, `crafting`

**响应：** `state` 消息

**行为说明：**
- 先触发结算，再执行装备操作
- 从背包扣除 1 个该物品
- 如果目标槽位已有装备，旧装备自动退回背包
- 装备/卸下后会重新计算属性

**错误情况：**
- `item_id` 或 `slot` 缺失 → `{"type": "error", "message": "item_id and slot required"}`
- 物品不存在 → `ValueError`
- 物品不是装备也不是工具 → `ValueError`
- 物品不能装备到指定槽位 → `ValueError`
- 背包中没有该物品 → `ValueError`

---

#### 6. unequip - 卸下装备/工具

从指定槽位卸下装备或工具，放回背包。

**请求：**
```json
{
    "type": "unequip",
    "slot": "main_hand"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| slot | string | 要卸下的槽位 |

**响应：** `state` 消息

**错误情况：**
- `slot` 缺失 → `{"type": "error", "message": "slot required"}`
- 该槽位没有装备 → `ValueError`
- 未知的槽位名称 → `ValueError`

---

### 服务器 → 客户端

#### state - 完整状态

所有成功操作的统一响应格式。包含玩家的完整状态快照。

```json
{
    "type": "state",
    "data": {
        "uid": 1,
        "inventory": [
            {"id": "oak_logs", "qty": 150},
            {"id": "oak_plank", "qty": 320},
            {"id": "wooden_stick", "qty": 80},
            {"id": "enchanted_gem", "state": 3, "qty": 2}
        ],
        "skills": {
            "felling": {"level": 15, "exp": 23330.0},
            "crafting": {"level": 12, "exp": 16165.0},
            "mining": {"level": 5, "exp": 2140.0}
        },
        "unlocked_events": [
            "starting_dialog_1",
            "starting_dialog_2",
            "starting_dialog_3",
            "starting_dialog_4",
            "starting_dialog_5",
            "home_expanding_1"
        ],
        "queue": ["felling_oak_tree"],
        "queue_index": 0,
        "queue_progress_seconds": 1.35,
        "last_sync_time": 1713340800.0,
        "settled_seconds": 3600.0,
        "settlement_log": [
            {
                "event_id": "felling_oak_tree",
                "iterations": 1800,
                "experience": 36000
            }
        ],
        "equipment": {
            "main_hand": "wooden_sword"
        },
        "tools": {
            "felling": "wooden_axe",
            "mining": "wooden_pickaxe"
        },
        "attributes": {
            "physical_damage": 10.0,
            "accuracy": 10.0,
            "attack_interval": 2.0,
            "felling_production_multiplier": 0.1,
            "mining_production_multiplier": 0.1
        }
    }
}
```

**data 字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| uid | integer | 玩家 ID |
| inventory | array | 背包物品，每项为 {id, qty, state?}。state 仅在非 0 时存在，缺省为 0 |
| skills | object | 技能状态，键为技能 ID，值为 {level, exp} |
| unlocked_events | string[] | 已解锁的升级行动 ID 列表（升序排列） |
| queue | string[] | 当前行动队列 |
| queue_index | integer | 当前执行到的队列位置（从 0 开始） |
| queue_progress_seconds | float | 当前行动已累积的秒数 |
| last_sync_time | float | 上次同步的 Unix 时间戳 |
| settled_seconds | float | 本次结算计算的经过秒数 |
| settlement_log | array | 结算日志，记录每个行动的执行次数和经验 |
| equipment | object | 已穿戴的装备，键为槽位，值为物品 ID |
| tools | object | 已装备的工具，键为槽位，值为物品 ID |
| attributes | object | 当前生效的属性值，键为属性 ID，值为最终数值 |

#### error - 错误消息

```json
{
    "type": "error",
    "message": "详细错误说明"
}
```

所有 `ValueError` 异常和业务逻辑错误都会转化为此格式。
内部异常会被捕获并返回 `"Internal error: ..."` 格式的消息。

---

## 典型交互流程

### 1. 新用户首次连接
```
客户端                           服务器
  |-- POST /api/register ------->|      注册账号
  |<-- Set-Cookie: token=xxx ----|
  |                              |
  |-- WS /ws (cookie: token) --->|      建立 WebSocket
  |<-- (连接成功) ----------------|
  |                              |
  |-- {"type": "sync"} --------->|      获取初始状态
  |<-- {"type": "state", ...} ---|      返回空白角色
  |                              |
  |-- {"type": "upgrade",        |      新手引导
  |    "event_id":               |
  |    "starting_dialog_1"} ---->|
  |<-- {"type": "state", ...} ---|
  |                              |
  |-- ... (完成 5 段对话) ------->|
  |                              |
  |-- {"type": "set_queue",      |      开始挂机
  |    "queue":                  |
  |    ["felling_oak_tree"]} --->|
  |<-- {"type": "state", ...} ---|
```

### 2. 离线后重新连接
```
客户端                           服务器
  |-- WS /ws (cookie: token) --->|      重新连接
  |<-- (连接成功) ----------------|
  |                              |
  |-- {"type": "sync"} --------->|      触发离线结算
  |<-- {"type": "state",         |      返回结算后状态
  |     "data": {                |      包含离线期间的
  |       "settled_seconds":     |      所有产出
  |         86400,               |
  |       "settlement_log": [    |
  |         {"event_id":         |
  |          "felling_oak_tree", |
  |          "iterations": 43200}|
  |       ], ...}} --------------|
```

### 3. 装备操作
```
客户端                           服务器
  |-- {"type": "equip",          |      穿戴木剑
  |    "item_id": "wooden_sword",|
  |    "slot": "main_hand"} ---->|
  |<-- {"type": "state", ...} ---|      属性已更新
  |                              |
  |-- {"type": "unequip",        |      卸下木剑
  |    "slot": "main_hand"} ---->|
  |<-- {"type": "state", ...} ---|      属性已还原
```
