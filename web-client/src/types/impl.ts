// 所有和乘算有关的东西都需要 +1, 例如 multiplier = 0.05 意思就是 1.05 倍

export interface item{
    id: string,
    name: string,
    tool: boolean,
    equipment: boolean,
    upgradable: boolean,
    classification: string,
    tool_details?: tool_details,
    equipment_details?: equipment_details,
    upgrade_details?: upgrade_details,
}

enum equipment_type{
    "weapon" = 0,
    "shield" = 1,
    "wear" = 2,
    "necklace" = 3,
    "relics" = 4, // etc...
}

export enum equipment_position{
    "main_hand" = 1, // 主手
    "side_hand" = 2, // 副手
    "head" = 3, // 头部
    "chest" = 4, // 胸部
    "leg" = 5, // 腿部
    "feet" = 6, // 足部
    "necklace" = 7, // 项链
    "treasure" = 8, // 宝物
    "nothing" = 9, // 不占用空间, 不在显示中
}

enum element{
    "physical" = 1,
    "fire" = 2,
    "water" = 3,
    "wind" = 4,
    "earth" = 5, // etc..
}

interface equipment_position_requirement{
    position: string, // in equipment_position
    value: number,
}

interface battle_skill_damage{
    type: string, // "physical" | "magic"
    flat: number,
    multiplier: number,
}

interface battle_skill_effect{
    target: string, // "self" | "target"
    attribute: string, // in battle_data key
    mode: string, // "flat" | "percent_multiplier"
    value: number, // flat: +value, percent_multiplier: +0.2 => *1.2
    duration_seconds?: number, // 可选，留空表示持续到战斗结束（或被同源效果覆盖）
}

interface battle_skill{
    id: string,
    name?: string,
    description: string,
    target_type: string, // e.g. "single", "self", "all_enemy"
    damage?: battle_skill_damage, // 辅助技能可不填
    effects?: battle_skill_effect[], // 辅助技能/复合技能的属性变化
    cooldown?: number, // 默认 0（基础技能）
    cast_time?: number, // 默认武器 attack_interval
    is_support?: boolean, // true 时可仅靠 effects 生效，不造成伤害
    is_basic?: boolean,
}

interface equipment_basic_data{
    physical_power: number, // 物理基础伤害(加算，下同)
    magic_power: number,// 奥术基础伤害
    power_multiplier: number, // 最终基础伤害(乘算)
    attack_interval: number, // 基础攻击速度
    attack_speed: number, // 攻击速度加成(加算)
    final_attack_speed_multiplier: number, // 最终攻击速度加成(乘算)
    critical: number, // 暴击率(加算)
    critical_possibility_multiplier: number // 最终暴击率 (乘算)
    critical_rate: number, // 暴击伤害
    block: number, // 格挡值(加算)，魔法伤害不经过格挡计算
    block_multiplier: number, // 格挡值(乘算)
    block_possibility_multiplier: number, // 最终格挡值倍率 (乘算)
    block_rate: number, // 格挡减伤 (加算)
    block_rate_multiplier: number, // 最终格挡倍率 (乘算)
    hp_recovery: number, // 生命自动回复 (每秒)
    mp_recovery: number, // 魔力自动回复 (每秒)
    sp_recovery: number, // 耐力自动回复 (每秒)
    overall_recovery_speed: number, // 自然恢复速率倍率
    accuracy: number, // 精准度
    accuracy_multiplier: number, // 最终精准度 (乘算)
    accuracy_possibility_multiplier: number // 最终命中概率乘算
    evade: number, // 闪避度
    evade_multiplier: number, // 最终闪避概率 (乘算)
    evade_possibility_multiplier: number // 最终闪避概率乘算
    magic_instance: number, // 奥术抵抗 (加算)
    magic_instance_multiplier: number // 奥术抵抗 (乘算)
    final_damage_multiplier: number, // 最终伤害倍率 (进攻方乘算)
    defense: number, // 防御值(加算)
    defense_multiplier: number, // 防御值(乘算)
    final_damage_induce: number, // 最终伤害减少 (防御方乘算)
    hatred: number, // 仇恨(加算)
    hatred_multiplier: number, // 仇恨值(乘算)
    max_hp: number, // 生命值上限（加算）
    max_mp: number, // 魔力值上限（加算）
    max_sp: number, // 耐力值上限（加算）
    hp_multiplier: number, // 生命值上限 (乘算)
    mp_multiplier: number, // 魔力值上限 (乘算)
    sp_multiplier: number, // 耐力值上限 (乘算)
    

    // 其他属性, 例如掉落加成等等等, 需要再写

}

enum requirement_type{
    "skill" = 1,
    "event" = 2,
    "item" = 3,
}

enum comparison_type{
    "bigger" = 1,
    "equal" = 2,
    "smaller" = 3,
    "bigger_or_equal" = 4,
    "smaller_or_equal" = 5,
}

interface requirement{
    type: string, // in equipment_type
    id: string,
    value?: number,
    comparison_type?: string // in comparison_type: event 默认值是 1, 因此 "没有 xx event" 可以设为小于 1. 如果是 item 同样不需要 comparison, 直接减少 value 的值的对应物品.
}

interface reward{
    id: string,
    num: number,
}

function block_possibility(block:number=0, block_multiplier:number=0):number {
    return  1 - 100 / (100 + block) / ( 1 + block_multiplier )
}

function block_rate(block_rate: number=0, block_rate_multiplier:number = 0) {
    return 1 - 100 / (100 + block_rate) / ( 1 + block_rate_multiplier )
}

function magic_damage(magic_damage: number = 0, magic_instance: number = 0){
    return magic_damage / ( 1 + ( magic_instance / 100 ) )
}

function physical_damage(physical_damage: number =0, defense: number = 0, final_damage_multiplier: number, final_damage_induce: number):number {
    return (0.9 + Math.random() * 0.2) * (physical_damage) ** 2 / (defense + physical_damage) * final_damage_multiplier / final_damage_induce;
}

function clamp(value: number, min: number, max: number): number {
    return Math.max(min, Math.min(value, max));
}

function evade_possibility(accuracy: number = 0, accuracy_multiplier: number = 0, evade: number = 0, evade_multiplier: number = 0) {
    return clamp((1 / (1 + Math.pow(2, clamp(accuracy / evade, 0, 10))) * evade_multiplier / accuracy_multiplier), 0, 1);
}

interface equipment_details{
    type: string, // in equipment_type
    equipment_position_requirements: equipment_position_requirement[],
    element?: string, // in element
    battle_skills?: battle_skill[],
    equipment_basic_data: Partial<equipment_basic_data>,
    requirement?: requirement[],
}

export enum tool_position{
    "felling" = 1, // 砍伐工具
    "mining" = 2, // 采矿工具
    "planting" = 3, // 种植工具
    "crafting" = 4, // 制造工具
    "forging" = 5, // 锻造工具
    "enhancing" = 6, // 赋能工具
}

enum action_skills{
    "felling" = 1,
    "mining" = 2,
    "planting" = 3,
    "crafting" = 4,
    "forging" = 5,
    "enhancing" = 6,
    "trade" = 7,
    "none" = 8,
}

enum skills{
    "felling" = 1,
    "mining" = 2,
    "planting" = 3,
    "crafting" = 4,
    "forging" = 5,
    "enhancing" = 6,
    "trading" = 7,
    "strength" = 8,
    "ranging" = 9,
    "resilience" = 10,
    "stamina" = 11,
    "magic" = 12,
    "intelligence" = 13,
    "defense" = 14,
}

interface tool_position_requirement{
    tool_position: string // in tool_position
    value: number,
}

interface tool_basic_data{
    felling_production_multiplier: number,
    felling_level_buff: number,
    felling_speed_multiplier: number,
    mining_production_multiplier: number,
    mining_level_buff: number, 
    mining_speed_multiplier: number,
    planting_recycle_multipler: number,
    planting_speed_multiplier: number,
    planting_production_multiplier: number,
    crafting_production_multiplier: number,
    crafting_level_buff: number,
    crafting_speed_multiplier: number,
    forging_production_multiplier: number,
    forging_level_buff: number,
    forging_speed_multiplier: number,
    enhancing_level_buff: number,
    enhancing_success_rate_multiplier: number,
}

function plant_recycle_possibility(basic_rate: number, planting_recycle_multipler: number) {
    return clamp(basic_rate * (1 + planting_recycle_multipler), 0, 1);
}

function enhancing_success_possibility(basic_rate: number, recommend_level: number, enhance_level:number, enhancing_success_rate_multiplier: number) {
    return basic_rate * (enhance_level > recommend_level ? (1 + (enhance_level - recommend_level) / 35) ** 2 : Math.pow(0.99, recommend_level - enhance_level)) * ( 1 + enhancing_success_rate_multiplier );
}

interface tool_details{
    tool_position_requirement: tool_position_requirement[],
    tool_basic_data: Partial<tool_basic_data>,
    tool_type: string,
    requirement?: [requirement]
}

interface upgrade_data{
    level: number,
    recommend_level: number,
    basic_success_rate: number,
    ability_multiplier: number, // 能力倍数, 简单粗暴数值乘相应倍数
}

interface upgrade_details{
    max_upgrade: number,
    enhance_slot: number,
    forge_slot: number,
    upgrade_curve: upgrade_data[] // 我觉得一个一个填写还是太史山了, 我这边改成 Linear Curve

    /*
    
        例如 如果 有点
        {
            "level": 3,
            "recommend_level": 10,
            "basic_success_rate": 20
        },{
            "level":5,
            "recommend_level": 20,
            "basic_sucdess_rate": 30
        }
        
        那么 level = 4 的点可以直接通过线性求得

        recommend_level = 15, basic_success_rate = 25

    */
}

enum event_type{
    "loop" = 1, // 循环行动, 就好像绝大多数放置游戏的那种
    "instant" = 2, // 即时行动, 可以马上执行 (希望不会因为这个导致太多请求)
    "upgrade" = 3, // 升级行动, 可能需要时间, 反正只能执行一次, 收益一般也很高
    "repeat_upgrade" = 4
}

interface event{
    id: string,
    name: string,
    description: string,
    type: string // in event_type,
    repeat_time?: number, // 单位是重复次数
    need_skill: string // in action_skills 是什么就填到什么栏里, 一般来说即时行动和升级行动放一起, 这一块就写 none
    requirements: requirement[],
    loop_time?: number, // 单位是秒
    experience?: number,  // 一般来说, 即时行动只会给交易经验，但是目前没有做即时行动（）
    map: string,
}

export interface all_actions{
    items: item[];
    events: event[];
    enemies: enemy_data[];
    battles: battle_field_data[];
}

// 现在还没做技能, 先默认一个敌人只会使用 奥术 / 物理 攻击

enum damage_type {
    "physical" = 1,
    "magic" = 2,
}

export interface battle_data{
    hp: number,
    mp: number,
    sp: number,
    physical_power: number,
    magic_power: number,
    attack_interval: number,
    critical: number,
    critical_rate: number,
    block: number,
    block_possibility_multiplier:number,
    block_rate: number,
    accuracy: number,
    accuracy_possibility_multiplier: number,
    evade: number,
    evade_possibility_multiplier: number,
    magic_instance: number,
    final_damage_multiplier: number,
    defense: number,
    final_damage_reduce: number,
    hatred: number,
    hp_recovery: number,
    mp_recovery: number,
    sp_recovery: number,
}

export interface production_data{
    felling_production_multiplier: number, // 循环行动产出加成
    felling_speed_multiplier: number, // 循环行动加速
    mining_production_multiplier: number,
    mining_speed_multiplier: number,
    planting_recycle_multipler: number,
    planting_speed_multiplier: number,
    planting_production_multiplier: number,
    crafting_production_multiplier: number,
    crafting_speed_multiplier: number,
    forging_production_multiplier: number,
    forging_speed_multiplier: number,
    enhancing_success_rate_multiplier: number,
}

export const init_battle_data: battle_data = {
    hp: 100,
    mp: 100,
    sp: 100,
    physical_power: 20,
    magic_power: 20,
    attack_interval: 2,
    critical: 0,
    critical_rate: 2,
    block: 20,
    block_possibility_multiplier: 0,
    block_rate: 0,
    accuracy: 40,
    accuracy_possibility_multiplier: 0,
    evade: 20,
    evade_possibility_multiplier: 0,
    magic_instance: 0.33,
    final_damage_multiplier: 0,
    defense: 10,
    final_damage_reduce: 0,
    hatred: 100,
    hp_recovery: 0,
    mp_recovery: 0,
    sp_recovery: 0
}

export interface enemy_data{
    id: string,
    name: string,
    enemy_battle_data: Partial<battle_data>,
    basic_damage_type: string,
    battle_skill?: single_battle_skill[],
    rewards: reward[],
}

export interface enemy_combination_data{
    enemies: string[], // 以后可能改成接口，比如单个猪有buff啥的
    weight: number
}

export enum combination_type {
    "week" = 0,
    "strong" = 1,
    "boss" = 2,
}

export interface battle_field_data{
    id: string,
    name: string,
    interval: number,
    week_enemy_combinations: enemy_combination_data[],
    strong_enemy_combinations: enemy_combination_data[],
    boss_enemy_combinations: enemy_combination_data[],
    combination_loop: string[], // in combination_type
    map: string,
}

export enum single_skill_condition_key {
    "self_hp" = 1, // 自身的 hp (数值)
    "self_hp_ratio" = 2, // 自身的 hp (比例)
    "self_mp" = 3, // 自身的 mp (数值)
    "self_mp_ratio" = 4, // 自身的 mp (比例)
    "self_sp" = 5, // 自身的 sp (数值)
    "self_sp_ratio" = 6, // 自身的 sp (比例)
    "target_hp" = 7, // 目标的 hp (数值)
    "target_hp_ratio" = 8, // 目标的 hp (比例)
    "any_enemy_hp" = 9, // 任意敌方的hp (数值)
    "any_enemy_hp_ratio" = 10, // 任意敌方的 hp (比例), 后面还会加
}

export enum logic_type {
    "and" = 1,
    "or" = 2,
    "nor" = 3, 
}

export interface single_skill_condition {
    key: string // from single_skill_condition_key,
    comparison_type: string // from comparison_type,
    value: number, 
}

export interface skill_conditions{
    logic_type: string // from logic_type,
    complex_condition?: skill_conditions
    normal_condition?: single_skill_condition[]
}

export interface single_battle_skill{
    battle_skill: battle_skill,
    condition?: skill_conditions,
    priority: number,
}
