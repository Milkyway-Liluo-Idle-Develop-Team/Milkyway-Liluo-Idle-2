import * as impl from './impl'

interface user_login_data {
    username: string,
    mail: string,
    token?: string,
}

// Password should only exist in transient request payloads, never in persisted user state.
interface login_request_data {
    username: string,
    password: string,
}

interface item_extra{
    id: string,
    value: number,
}

interface detailed_item_data{
    basic_item: impl.item,
    extra: [item_extra],
    num: number,
}

interface detailed_equipment_data{
    basic_item: impl.item,
    extra: [item_extra],
    num: number,
    position: impl.equipment_position
}

interface detailed_tool_data{
    basic_item: impl.item,
    extra: [item_extra],
    num: number,
    position: impl.tool_position
}

interface user_item_data {
    user_backpack_data: [detailed_item_data],
    user_equipment_data: [detailed_equipment_data],
    user_tool_data: [detailed_tool_data]
}

interface user_event_data {
    user_unlocked_loop_events: [string], // id of events
    user_unlocked_upgrade_events: [string],
    user_finished_upgrade_events: [string],
}

interface user_action_data {
    user_last_action: Date,
    user_last_event: string, // id of event
}

interface user_total_data{
    user_item_data: user_item_data,
    user_event_data: user_event_data,
    user_action_data: user_action_data,
    user_login_data: user_login_data,
}
