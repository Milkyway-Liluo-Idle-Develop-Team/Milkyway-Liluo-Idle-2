"use strict";
// 所有和乘算有关的东西都需要 +1, 例如 multiplier = 0.05 意思就是 1.05 倍
Object.defineProperty(exports, "__esModule", { value: true });
exports.combination_type = exports.init_battle_data = exports.tool_position = exports.equipment_position = void 0;
var equipment_type;
(function (equipment_type) {
    equipment_type[equipment_type["weapon"] = 0] = "weapon";
    equipment_type[equipment_type["shield"] = 1] = "shield";
    equipment_type[equipment_type["wear"] = 2] = "wear";
    equipment_type[equipment_type["necklace"] = 3] = "necklace";
    equipment_type[equipment_type["relics"] = 4] = "relics";
})(equipment_type || (equipment_type = {}));
var equipment_position;
(function (equipment_position) {
    equipment_position[equipment_position["main_hand"] = 1] = "main_hand";
    equipment_position[equipment_position["side_hand"] = 2] = "side_hand";
    equipment_position[equipment_position["head"] = 3] = "head";
    equipment_position[equipment_position["chest"] = 4] = "chest";
    equipment_position[equipment_position["leg"] = 5] = "leg";
    equipment_position[equipment_position["feet"] = 6] = "feet";
    equipment_position[equipment_position["necklace"] = 7] = "necklace";
    equipment_position[equipment_position["treasure"] = 8] = "treasure";
    equipment_position[equipment_position["nothing"] = 9] = "nothing";
})(equipment_position || (exports.equipment_position = equipment_position = {}));
var element;
(function (element) {
    element[element["physical"] = 1] = "physical";
    element[element["fire"] = 2] = "fire";
    element[element["water"] = 3] = "water";
    element[element["wind"] = 4] = "wind";
    element[element["earth"] = 5] = "earth";
})(element || (element = {}));
var requirement_type;
(function (requirement_type) {
    requirement_type[requirement_type["skill"] = 1] = "skill";
    requirement_type[requirement_type["event"] = 2] = "event";
    requirement_type[requirement_type["item"] = 3] = "item";
})(requirement_type || (requirement_type = {}));
var comparison_type;
(function (comparison_type) {
    comparison_type[comparison_type["bigger"] = 1] = "bigger";
    comparison_type[comparison_type["equal"] = 2] = "equal";
    comparison_type[comparison_type["smaller"] = 3] = "smaller";
    comparison_type[comparison_type["bigger_or_equal"] = 4] = "bigger_or_equal";
    comparison_type[comparison_type["smaller_or_equal"] = 5] = "smaller_or_equal";
})(comparison_type || (comparison_type = {}));
function block_possibility(block, block_multiplier) {
    if (block === void 0) { block = 0; }
    if (block_multiplier === void 0) { block_multiplier = 0; }
    return 1 - 100 / (100 + block) / (1 + block_multiplier);
}
function block_rate(block_rate, block_rate_multiplier) {
    if (block_rate === void 0) { block_rate = 0; }
    if (block_rate_multiplier === void 0) { block_rate_multiplier = 0; }
    return 1 - 100 / (100 + block_rate) / (1 + block_rate_multiplier);
}
function magic_damage(magic_damage, magic_instance) {
    if (magic_damage === void 0) { magic_damage = 0; }
    if (magic_instance === void 0) { magic_instance = 0; }
    return magic_damage / (1 + (magic_instance / 100));
}
function physical_damage(physical_damage, defense, final_damage_multiplier, final_damage_induce) {
    if (physical_damage === void 0) { physical_damage = 0; }
    if (defense === void 0) { defense = 0; }
    return (0.9 + Math.random() * 0.2) * Math.pow((physical_damage), 2) / (defense + physical_damage) * final_damage_multiplier / final_damage_induce;
}
function clamp(value, min, max) {
    return Math.max(min, Math.min(value, max));
}
function evade_possibility(accuracy, accuracy_multiplier, evade, evade_multiplier) {
    if (accuracy === void 0) { accuracy = 0; }
    if (accuracy_multiplier === void 0) { accuracy_multiplier = 0; }
    if (evade === void 0) { evade = 0; }
    if (evade_multiplier === void 0) { evade_multiplier = 0; }
    return clamp((1 / (1 + Math.pow(2, clamp(accuracy / evade, 0, 10))) * evade_multiplier / accuracy_multiplier), 0, 1);
}
var tool_position;
(function (tool_position) {
    tool_position[tool_position["felling"] = 1] = "felling";
    tool_position[tool_position["mining"] = 2] = "mining";
    tool_position[tool_position["planting"] = 3] = "planting";
    tool_position[tool_position["crafting"] = 4] = "crafting";
    tool_position[tool_position["forging"] = 5] = "forging";
    tool_position[tool_position["enhancing"] = 6] = "enhancing";
})(tool_position || (exports.tool_position = tool_position = {}));
var action_skills;
(function (action_skills) {
    action_skills[action_skills["felling"] = 1] = "felling";
    action_skills[action_skills["mining"] = 2] = "mining";
    action_skills[action_skills["planting"] = 3] = "planting";
    action_skills[action_skills["crafting"] = 4] = "crafting";
    action_skills[action_skills["forging"] = 5] = "forging";
    action_skills[action_skills["enhancing"] = 6] = "enhancing";
    action_skills[action_skills["trade"] = 7] = "trade";
    action_skills[action_skills["none"] = 8] = "none";
})(action_skills || (action_skills = {}));
function plant_recycle_possibility(basic_rate, planting_recycle_multipler) {
    return clamp(basic_rate * (1 + planting_recycle_multipler), 0, 1);
}
function enhancing_success_possibility(basic_rate, recommend_level, enhance_level, enhancing_success_rate_multiplier) {
    return basic_rate * (enhance_level > recommend_level ? Math.pow((1 + (enhance_level - recommend_level) / 35), 2) : Math.pow(0.99, recommend_level - enhance_level)) * (1 + enhancing_success_rate_multiplier);
}
var event_type;
(function (event_type) {
    event_type[event_type["loop"] = 1] = "loop";
    event_type[event_type["instant"] = 2] = "instant";
    event_type[event_type["upgrade"] = 3] = "upgrade";
    event_type[event_type["repeat_upgrade"] = 4] = "repeat_upgrade";
})(event_type || (event_type = {}));
// 现在还没做技能, 先默认一个敌人只会使用 奥术 / 物理 攻击
var damage_type;
(function (damage_type) {
    damage_type[damage_type["physical"] = 1] = "physical";
    damage_type[damage_type["magic"] = 2] = "magic";
})(damage_type || (damage_type = {}));
exports.init_battle_data = {
    hp: 100,
    mp: 100,
    sp: 100,
    physical_power: 20,
    magic_power: 20,
    attack_interval: 2,
    critical: 0,
    critical_rate: 2,
    block: 0,
    block_possibility_multiplier: 0,
    block_rate: 20,
    accuracy: 40,
    evade: 20,
    magic_instance: 0.33,
    final_damage_multiplier: 0,
    defense: 10,
    final_damage_induce: 0,
    hatred: 0,
    hp_recovery: 0,
    mp_recovery: 0,
    sp_recovery: 0
};
var combination_type;
(function (combination_type) {
    combination_type[combination_type["weak"] = 0] = "weak";
    combination_type[combination_type["strong"] = 1] = "strong";
    combination_type[combination_type["boss"] = 2] = "boss";
})(combination_type || (exports.combination_type = combination_type = {}));
