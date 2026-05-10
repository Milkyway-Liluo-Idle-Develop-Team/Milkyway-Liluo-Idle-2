package wsx

// Wire opcodes for Envelope. The server uses these instead of string types
// to reduce packet size. Client and server must share this mapping.
const (
	// Client → Server request opcodes (1–99)
	OpcodePing            = 1
	OpcodeWhoami          = 2
	OpcodeInventoryEquip  = 10
	OpcodeInventoryUnequip = 11
	OpcodeQueueAppend     = 20
	OpcodeQueueRemove     = 21
	OpcodeQueueMove       = 22
	OpcodeQueueSet        = 23
	OpcodeBattleStart     = 30
	OpcodeBattleStop      = 31
	OpcodeBattleState     = 32

	// Server → Client push opcodes (100–199, no matching request)
	OpcodeStateFull        = 100
	OpcodeStateDiff        = 101
	OpcodeBattleEventBatch = 102
	OpcodeBattleSnapshot   = 103

	// Server → Client error opcode (generic)
	OpcodeError = 255
)

// typeToOpcode maps internal string type names to wire opcodes.
// Every Outbound.Type used in the codebase must have an entry here.
var typeToOpcode = map[string]int32{
	"ping":                 OpcodePing,
	"auth.whoami":          OpcodeWhoami,
	"inventory.equip":      OpcodeInventoryEquip,
	"inventory.unequip":    OpcodeInventoryUnequip,
	"queue.append":         OpcodeQueueAppend,
	"queue.remove":         OpcodeQueueRemove,
	"queue.move":           OpcodeQueueMove,
	"queue.set":            OpcodeQueueSet,
	"battle.start":         OpcodeBattleStart,
	"battle.stop":          OpcodeBattleStop,
	"battle.state":         OpcodeBattleState,

	// Push types
	"state.full":           OpcodeStateFull,
	"state.diff":           OpcodeStateDiff,
	"battle.event_batch":   OpcodeBattleEventBatch,
	"battle.snapshot":      OpcodeBattleSnapshot,

	// Reply suffixes (computed automatically by Reply/ReplyError)
	"ping.ok":                 OpcodePing + 1000,
	"auth.whoami.ok":          OpcodeWhoami + 1000,
	"inventory.equip.ok":      OpcodeInventoryEquip + 1000,
	"inventory.unequip.ok":    OpcodeInventoryUnequip + 1000,
	"queue.append.ok":         OpcodeQueueAppend + 1000,
	"queue.remove.ok":         OpcodeQueueRemove + 1000,
	"queue.move.ok":           OpcodeQueueMove + 1000,
	"queue.set.ok":            OpcodeQueueSet + 1000,
	"battle.start.ok":         OpcodeBattleStart + 1000,
	"battle.stop.ok":          OpcodeBattleStop + 1000,
	"battle.state.ok":         OpcodeBattleState + 1000,

	"ping.err":                OpcodePing + 2000,
	"auth.whoami.err":         OpcodeWhoami + 2000,
	"inventory.equip.err":     OpcodeInventoryEquip + 2000,
	"inventory.unequip.err":   OpcodeInventoryUnequip + 2000,
	"queue.append.err":        OpcodeQueueAppend + 2000,
	"queue.remove.err":        OpcodeQueueRemove + 2000,
	"queue.move.err":          OpcodeQueueMove + 2000,
	"queue.set.err":           OpcodeQueueSet + 2000,
	"battle.start.err":        OpcodeBattleStart + 2000,
	"battle.stop.err":         OpcodeBattleStop + 2000,
	"battle.state.err":        OpcodeBattleState + 2000,

	"error": OpcodeError,
}

// opcodeToType is the reverse mapping for inbound decoding.
var opcodeToType map[int32]string

func init() {
	opcodeToType = make(map[int32]string, len(typeToOpcode))
	for s, o := range typeToOpcode {
		opcodeToType[o] = s
	}
}

// resolveOpcode returns the opcode for an Outbound message.
// If Opcode is explicitly set (>0) it is returned directly.
// Otherwise the Type string is looked up in the mapping table.
// As a last resort it returns OpcodeError.
func resolveOpcode(msg Outbound) int32 {
	if msg.Opcode > 0 {
		return msg.Opcode
	}
	if op, ok := typeToOpcode[msg.Type]; ok {
		return op
	}
	return OpcodeError
}
