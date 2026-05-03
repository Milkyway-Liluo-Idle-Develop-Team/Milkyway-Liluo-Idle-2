interface item_action
{
    name: string
    activate(): void
}

interface item 
{
    id: string,
    name: string,
    classification: string[],
    actions: item_action[]
}

interface item_upgrade_curve
{
    level: number,
    recommand_level: number
    basic_success_rate: number,
    abality_multiplier: number
}

interface item_upgrade_action extends item_action
{
    max_level: number
    enhance_slot: number
    forge_slot: number
    upgrade_curve: item_upgrade_curve[]
}

interface item_requirement
{
    check(): boolean
}

interface item_equip_requirement extends item_requirement
{
    position: string,
    value: number
}

interface item_equip_action extends item_action
{
    type: string
    requirements: item_requirement[]
    element: string[],
    skills: [],
    attributes: unknown[],
}

function main() {
    let items: item[] = 
    [
        {
            id: "wooden_sword",
            name: "木剑",
            classification: ["equipment", "wood", "sword"],
            actions: 
            [
                {
                    name: "upgrade",
                    max_level: 30,
                    enhance_slot: 1,
                    forge_slot: 1,
                    upgrade_curve:
                    [
                        {
                            level: 0,
                            recommand_level: 20,
                            basic_success_rate: .52,
                            abality_multiplier: 1
                        },
                        {
                            level: 5,
                            recommand_level: 30,
                            basic_success_rate: .37,
                            abality_multiplier: 1.5
                        }
                    ],
                    activate() {}
                } as item_upgrade_action,
                {
                    name: "equip",
                    type: "weapon",
                    requirements: 
                    [
                        { 
                            position: "main_hand",
                            value: 1,
                            check() { return true }
                        } as item_equip_requirement
                    ],
                    element: ["physical"],
                    skills: [],
                    attributes: 
                    [
                        { physical_damage: 10 },
                        { accuracy: 10 },
                        { attack_interval: 2 }
                    ],
                    activate() {}
                } as item_equip_action
            ]
        }
    ]
}