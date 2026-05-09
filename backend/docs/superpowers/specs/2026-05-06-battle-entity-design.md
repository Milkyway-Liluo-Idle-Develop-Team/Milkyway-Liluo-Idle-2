# 战斗系统：BattleEntity 属性设计分析

## 1. 现状对比

### 1.1 当前 Go 后端 attributes.json（11个属性）

| 属性 ID | 默认值 | group | 说明 |
|---------|--------|-------|------|
| physical_power | 10 | combat | 物理攻击力 |
| magic_damage | 0 | combat | 魔法伤害 |
| accuracy | 0 | combat | 命中率 |
| attack_interval | 2 | combat | 攻击间隔（秒） |
| defense | 0 | combat | 防御 |
| hatred_multiplier | 0 | combat | 暴击倍率（命名有误，实际是仇恨倍率） |
| hp_recovery | 0 | combat | 生命恢复 |
| exp_gain_multiplier | 0 | production | 经验获取倍率 |
| felling_production_multiplier | 0 | production | 伐木产出倍率 |
| mining_production_multiplier | 0 | production | 采矿产出倍率 |
| crafting_production_multiplier | 0 | production | 制作产出倍率 |

### 1.2 旧 Python 后端 INIT_BATTLE_DATA（25个属性）

旧后端用松散 dict 维护，包含完整战斗所需的全部属性：

| 属性 | 默认值 | 说明 |
|------|--------|------|
| hp | 100 | 生命上限 |
| mp | 100 | 魔力上限 |
| sp | 100 | 耐力上限 |
| physical_power | 20 | 物理伤害 |
| magic_power | 20 | 奥术伤害 |
| attack_interval | 2.0 | 攻击间隔 |
| critical | 0.0 | 暴击率 |
| critical_rate | 2.0 | 暴击倍率 |
| block | 20.0 | 格挡值 |
| block_possibility_multiplier | 0.0 | 格挡概率倍率加成 |
| block_rate | 0.0 | 格挡减伤率 |
| accuracy | 20.0 | 精准值 |
| accuracy_possibility_multiplier | 0.0 | 命中概率倍率加成 |
| evade | 20.0 | 闪避值 |
| evade_possibility_multiplier | 0.0 | 闪避概率倍率加成 |
| magic_instance | 0.33 | 奥术抵抗 |
| final_damage_multiplier | 0.0 | 最终伤害加成 |
| defense | 10.0 | 防御值 |
| final_damage_reduce | 0.0 | 最终伤害减免 |
| hatred | 100.0 | 仇恨值 |
| hp_recovery | 0.0 | 生命回复（每秒） |
| mp_recovery | 0.0 | 魔力回复（每秒） |
| sp_recovery | 0.0 | 耐力回复（每秒） |

### 1.3 缺失分析

当前 Go 后端 **缺失 18 个战斗属性**，这些是伤害计算、闪避格挡、资源消耗不可或缺的：

**资源上限类（3个）：**
- `hp` / `max_hp` — 生命值上限
- `mp` / `max_mp` — 魔力值上限
- `sp` / `max_sp` — 耐力值上限

> 注：旧后端只有 `hp` 一个属性，Max 通过 stats 中的同名属性计算。新后端可以用 `hp` 作为上限属性，当前值作为 BattleEntity 的运行时字段。

**伤害机制类（4个）：**
- `magic_power` — 奥术伤害（当前只有 `magic_damage`，名称需统一）
- `critical` — 暴击率
- `critical_rate` / `critical_multiplier` — 暴击倍率
- `final_damage_multiplier` — 最终伤害加成

**防御机制类（5个）：**
- `block` — 格挡值
- `block_possibility_multiplier` — 格挡概率倍率
- `block_rate` — 格挡减伤率
- `magic_instance` — 奥术抵抗
- `final_damage_reduce` — 最终伤害减免

**命中/闪避类（4个）：**
- `evade` — 闪避值
- `evade_possibility_multiplier` — 闪避概率倍率
- `accuracy_possibility_multiplier` — 命中概率倍率
- `hatred` — 仇恨值（当前 `hatred_multiplier` 是倍率，需要基础值）

**资源恢复类（2个）：**
- `mp_recovery` — 魔力回复
- `sp_recovery` — 耐力回复

---

## 2. BattleEntity 结构设计

### 2.1 设计原则

1. **属性最终值来源统一**：玩家的最终属性来自 `attribute.Instance.GetFinal()`；敌人也使用 `attribute.Instance`（临时实例，战斗结束后丢弃），这样 buff/debuff 机制可以复用同一套 modifier 系统。
2. **运行时状态隔离**：HP/MP/SP 当前值、存活状态、冷却时间、活跃效果等属于**运行时状态**，不在属性系统中存储。
3. **技能数据独立**：技能定义来自 `gameconfig.BattleSkill`，但运行时选择逻辑和冷却状态在 BattleEntity 中维护。

### 2.2 核心接口

```go
package battle

// Team 表示实体阵营
type Team int

const (
	TeamPlayer Team = iota
	TeamEnemy
)

// BattleEntity 是战斗中玩家和敌人的通用接口
type BattleEntity interface {
	// --- 身份 ---
	EntityID() string   // 实例唯一ID
	Name() string
	Team() Team

	// --- 存活状态 ---
	Alive() bool
	SetAlive(bool)

	// --- 资源（当前值 + 最大值来自属性系统）---
	HP() float64
	SetHP(float64)
	MaxHP() float64 // = attr.GetFinal(hpAttrID)

	MP() float64
	SetMP(float64)
	MaxMP() float64 // = attr.GetFinal(mpAttrID)

	SP() float64
	SetSP(float64)
	MaxSP() float64 // = attr.GetFinal(spAttrID)

	// --- 行动时间 ---
	NextReadyTime() float64      // 绝对时间（秒，基于 battle time）
	SetNextReadyTime(float64)
	LastActionDuration() float64 // 上次行动耗时（用于前端进度条）
	SetLastActionDuration(float64)

	// --- 属性查询（委托给 attribute.Instance）---
	// GetFinal 读取属性系统的最终计算值
	GetFinal(attrID attribute.AttributeID) float64

	// --- 效果系统 ---
	ActiveEffects() []ActiveEffect
	ApplyEffect(effect ActiveEffect, now float64)
	RefreshStats(now float64) // 清理过期效果并重新计算 max hp/mp/sp

	// --- 技能系统 ---
	Skills() map[string]*BattleSkill        // 可用技能映射 id -> skill
	SkillPlan() []SkillPlanEntry            // 优先级计划
	BasicSkillID() string
	Cooldowns() map[string]float64          // skill_id -> 冷却到期时间(绝对时间)
	SetCooldown(skillID string, expiresAt float64)

	// --- 战斗记录 ---
	LastSkillID() string
	SetLastSkillID(string)
	LastSkillName() string
	SetLastSkillName(string)
}
```

### 2.3 ActiveEffect（活跃效果）

```go
// EffectMode 效果作用方式
type EffectMode int

const (
	EffectModeFlat EffectMode = iota       // 固定值加减
	EffectModePercentMult                  // 百分比乘算
)

// ActiveEffect 是一个实时生效的 buff/debuff
type ActiveEffect struct {
	SourceSkillID string             // 来源技能ID，用于同技能效果覆盖
	Attribute     attribute.AttributeID // 目标属性
	Mode          EffectMode
	Value         float64
	ExpiresAt     *float64           // nil 表示永久（直到战斗结束）
}
```

### 2.4 PlayerBattleEntity（玩家战斗实体）

```go
// PlayerBattleEntity 包装 PlayerSession 进入战斗
type PlayerBattleEntity struct {
	userID int64
	name   string

	// 属性系统：直接引用 PlayerSession 的 attribute.Instance
	// 战斗中的临时 buff 通过 AddModifiers("battle:buff_xxx", mods) 添加
	attr *attribute.Instance

	// --- 运行时状态 ---
	hp  float64
	mp  float64
	sp  float64
	alive bool

	nextReadyTime      float64
	lastActionDuration float64

	cooldowns     map[string]float64  // skill_id -> expires_at
	activeEffects []ActiveEffect

	// --- 技能 ---
	skills       map[string]*BattleSkill
	skillPlan    []SkillPlanEntry
	basicSkillID string

	// --- 记录 ---
	lastSkillID   string
	lastSkillName string
}
```

**关键行为：**
- `MaxHP()` 调用 `attr.GetFinal(hpAttrID)`，属性系统已包含装备、等级、buff 的完整计算
- 当 `MaxHP` 变化时（如 buff 过期），当前 HP 按比例保持：`newHP = newMax * (oldHP / oldMax)`
- 战斗结束时，清理所有 `battle:` 前缀的 modifier：`attr.RemoveModifiers("battle:...")`

### 2.5 EnemyBattleEntity（敌人战斗实体）

```go
// EnemyBattleEntity 从 enemy definition 构建
type EnemyBattleEntity struct {
	enemyID    string // 如 "goblin"
	instanceID string // 本次战斗实例ID（如 "goblin_0"）
	name       string

	// 敌人拥有独立的 attribute.Instance（临时，战斗结束丢弃）
	attr *attribute.Instance

	// --- 运行时状态（同玩家）---
	hp  float64
	mp  float64
	sp  float64
	alive bool

	nextReadyTime      float64
	lastActionDuration float64

	cooldowns     map[string]float64
	activeEffects []ActiveEffect

	// --- 技能 ---
	skills       map[string]*BattleSkill
	skillPlan    []SkillPlanEntry
	basicSkillID string
	basicDamageType string // "physical" | "magic"

	// --- 记录 ---
	lastSkillID   string
	lastSkillName string

	// --- 掉落与奖励 ---
	drops     []DropEntry
	expReward float64
}
```

**构建流程：**
1. 从 `gameconfig` 读取敌人定义的基础属性
2. 创建临时 `attribute.Instance`
3. 用敌人的基础属性值作为 `OpOverride` modifier 注入（source = `enemy:base`）
4. 后续 buff/debuff 通过 `AddModifiers("enemy:buff_xxx", mods)` 添加

---

## 3. 属性系统扩展建议（attributes.json）

### 3.1 需要新增的 combat 属性

```json
[
  {"id": "hp",                    "name": "生命值上限",       "default_value": 100, "min_value": 1,   "direction": "positive", "group": "combat", "desc": "角色生命上限"},
  {"id": "mp",                    "name": "魔力值上限",       "default_value": 100, "min_value": 0,   "direction": "positive", "group": "combat", "desc": "角色魔力上限"},
  {"id": "sp",                    "name": "耐力值上限",       "default_value": 100, "min_value": 0,   "direction": "positive", "group": "combat", "desc": "角色耐力上限"},
  {"id": "magic_power",           "name": "奥术伤害",         "default_value": 20,  "min_value": 0,   "direction": "positive", "group": "combat", "desc": "角色基础奥术攻击力"},
  {"id": "critical",              "name": "暴击率",           "default_value": 0,   "min_value": 0,   "max_value": 1,   "direction": "positive", "group": "combat", "desc": "暴击触发概率"},
  {"id": "critical_rate",         "name": "暴击倍率",         "default_value": 2,   "min_value": 1,   "direction": "positive", "group": "combat", "desc": "暴击时的伤害倍率"},
  {"id": "block",                 "name": "格挡值",           "default_value": 0,   "min_value": 0,   "direction": "positive", "group": "combat", "desc": "影响格挡概率"},
  {"id": "block_possibility_multiplier", "name": "格挡概率倍率", "default_value": 0, "min_value": 0, "direction": "positive", "group": "combat", "desc": "格挡概率的乘算加成"},
  {"id": "block_rate",            "name": "格挡减伤率",       "default_value": 0.2, "min_value": 0,   "direction": "positive", "group": "combat", "desc": "格挡时伤害减免比例"},
  {"id": "evade",                 "name": "闪避值",           "default_value": 20,  "min_value": 0,   "direction": "positive", "group": "combat", "desc": "影响闪避概率"},
  {"id": "evade_possibility_multiplier", "name": "闪避概率倍率", "default_value": 0, "min_value": 0, "direction": "positive", "group": "combat", "desc": "闪避概率的乘算加成"},
  {"id": "accuracy_possibility_multiplier", "name": "命中概率倍率", "default_value": 0, "min_value": 0, "direction": "positive", "group": "combat", "desc": "命中概率的乘算加成"},
  {"id": "magic_instance",        "name": "奥术抵抗",         "default_value": 0.33,"min_value": 0,   "direction": "positive", "group": "combat", "desc": "减少受到的奥术伤害"},
  {"id": "final_damage_multiplier","name": "最终伤害加成",    "default_value": 0,   "min_value": 0,   "direction": "positive", "group": "combat", "desc": "所有造成伤害的乘算加成"},
  {"id": "final_damage_reduce",   "name": "最终伤害减免",     "default_value": 0,   "min_value": 0,   "direction": "positive", "group": "combat", "desc": "所有受到伤害的乘算减免"},
  {"id": "hatred",                "name": "仇恨值",           "default_value": 100, "min_value": 0,   "direction": "positive", "group": "combat", "desc": "影响敌人目标选择"},
  {"id": "mp_recovery",           "name": "魔力回复",         "default_value": 0,   "min_value": 0,   "direction": "positive", "group": "combat", "desc": "每秒恢复的魔力值"},
  {"id": "sp_recovery",           "name": "耐力回复",         "default_value": 0,   "min_value": 0,   "direction": "positive", "group": "combat", "desc": "每秒恢复的耐力值"}
]
```

### 3.2 需要修正的属性

- 当前 `magic_damage`（ID=2）应更名为 `magic_power`，与旧后端及文档保持一致
- 当前 `hatred_multiplier`（ID=6）命名正确，但需要新增 `hatred` 基础属性

---

## 4. 运行时状态 vs 属性系统值 对照表

| 数据 | 属于运行时状态 | 属于属性系统 |
|------|-------------|-------------|
| 当前 HP / MP / SP | ✅ | ❌ |
| Max HP / MP / SP | ❌ | ✅（hp/mp/sp 属性） |
| 物理/奥术攻击力 | ❌ | ✅（physical_power / magic_power） |
| 暴击率/暴击倍率 | ❌ | ✅（critical / critical_rate） |
| 格挡值/格挡减伤率 | ❌ | ✅（block / block_rate） |
| 命中/闪避值 | ❌ | ✅（accuracy / evade） |
| 最终伤害加成/减免 | ❌ | ✅（final_damage_multiplier / final_damage_reduce） |
| 仇恨值 | ❌ | ✅（hatred） |
| 资源回复速率 | ❌ | ✅（hp_recovery / mp_recovery / sp_recovery） |
| 下次行动时间 | ✅ | ❌ |
| 技能冷却 | ✅ | ❌ |
| buff/debuff 效果 | ✅（以 ActiveEffect 形式） | ✅（同步注入 attribute.Instance 的临时 modifier） |
| 是否存活 | ✅ | ❌ |
| 上次使用技能 | ✅ | ❌ |

---

## 5. 技能数据结构

```go
// BattleSkill 运行时战斗技能（从 gameconfig.BattleSkill + 运行时条件扩展）
type BattleSkill struct {
	ID          string
	Name        string
	Description string
	TargetType  string // "single" | "aoe" | "self"

	// 伤害配置
	Damage *DamageProfile

	// 消耗
	MPCost float64
	SPCost float64

	// 时间
	CastTime float64 // 施放时间 = 占用行动的时间片
	Cooldown float64 // 冷却时间

	// 效果
	Effects []SkillEffect

	// 条件（仅玩家自定义技能计划需要）
	Condition *SkillCondition

	// 标记
	IsBasic       bool // 是否为默认基础攻击
	IsSupport     bool // 是否纯辅助技能（无伤害）
	PhysicalStyle string // "melee" | "ranged"，影响经验分配
}

type DamageProfile struct {
	Type       string  // "physical" | "magic"
	Flat       float64 // 固伤加算
	Multiplier float64 // 攻击力的乘算倍率
}

type SkillEffect struct {
	Target     string  // "self" | "target"
	Attribute  attribute.AttributeID
	Mode       EffectMode
	Value      float64
	Duration   float64 // 秒，0 表示瞬时
}

// SkillPlanEntry 技能计划中的优先级条目
type SkillPlanEntry struct {
	SkillID   string
	Priority  int
	Condition *SkillCondition
}
```

---

## 6. 下一步行动建议

按依赖顺序，建议分以下步骤实施：

### Step 1: 补齐属性系统（attributes.json + attr_registry.json）
- 添加缺失的 18 个战斗属性
- 运行 `genregistry` 生成 numeric ID
- 更新 `hatred_multiplier` 的 desc（当前 desc 写"暴击倍率加成"是错误的，应为"仇恨值倍率"）
- 确认 `magic_damage` 是否统一为 `magic_power`

### Step 2: 定义 BattleEntity 接口与基础结构
- 在 `internal/battle/`（或复用 `internal/session/battle.go`）中定义接口
- 实现 `ActiveEffect` 管理（过期清理、同技能覆盖）
- 实现 `RefreshStats`（Max 变化时按比例调整当前值）

### Step 3: 实现 PlayerBattleEntity
- 从 `PlayerSession` 构建，引用其 `attribute.Instance`
- 从已装备武器收集 `BattleSkill`
- 注入玩家技能等级带来的基础属性 modifier（如 strength → physical_power）

### Step 4: 实现 EnemyBattleEntity
- 从敌人定义构建临时 `attribute.Instance`
- 实现敌人技能选择逻辑（优先级 + 条件检定）

### Step 5: 伤害计算引擎
- 物理/奥术伤害公式
- 闪避/格挡/暴击判定
- 效果应用

### Step 6: 战斗主循环（BattleSession）
- 事件驱动的时间推进
- 波次生成、死亡结算、经验/掉落分配
