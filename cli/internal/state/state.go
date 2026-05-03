package state

import (
	"fmt"
	"time"

	pb "github.com/edrowsluo/new-mli/backend/pb"
	"google.golang.org/protobuf/proto"
)

// GameConfig holds the static game configuration fetched on startup.
type GameConfig struct {
	Actions      map[string]interface{} // parsed from actions.json
	IDRegistry   map[string]int64       // parsed from id_registry.json
	Attributes   map[string]interface{} // parsed from attributes.json
	AttrRegistry map[string]int64       // parsed from attr_registry.json
	LevelCurve   []LevelCurveEntry
}

type LevelCurveEntry struct {
	Level int
	XP    float64
}

// SkillSlot holds live skill data.
type SkillSlot struct {
	Level float64
	XP    float64
}

// ItemKey identifies an item in inventory.
type ItemKey struct {
	ID    int32
	State int32
}

// EquippedItem holds what's in a slot.
type EquippedItem struct {
	ID    int32
	State int32
}

// EventQueueEntry mirrors pb.EventQueueEntry for local use.
type EventQueueEntry struct {
	EventID       int64
	TargetCycles  int32
	Progress      float64
}

// LogEntry is a single line in the client log panel.
type LogEntry struct {
	Time    time.Time
	Type    string // e.g. "info", "warn", "event"
	Message string
}

// GameState holds all mutable player state received from the server.
type GameState struct {
	Config       *GameConfig
	UserID       int64
	Username     string
	Skills       map[int64]*SkillSlot
	Inventory    map[ItemKey]float64
	Equipment    map[string]*EquippedItem
	EventQueues  map[int32][]EventQueueEntry
	Log          []LogEntry
	LogLimit     int
}

// NewGameState creates an empty state ready to receive diffs.
func NewGameState() *GameState {
	return &GameState{
		Skills:      make(map[int64]*SkillSlot),
		Inventory:   make(map[ItemKey]float64),
		Equipment:   make(map[string]*EquippedItem),
		EventQueues: make(map[int32][]EventQueueEntry),
		Log:         make([]LogEntry, 0, 100),
		LogLimit:    200,
	}
}

// ApplyDiff merges a server StateDiff into local state.
func (s *GameState) ApplyDiff(diff *pb.StateDiff) {
	for _, inv := range diff.Inventory {
		key := ItemKey{ID: inv.ItemId, State: inv.ItemState}
		s.Inventory[key] += inv.QuantityDelta
		if s.Inventory[key] <= 0 {
			delete(s.Inventory, key)
		}
	}
	for _, sk := range diff.SkillXp {
		slot, ok := s.Skills[sk.SkillId]
		if !ok {
			slot = &SkillSlot{}
			s.Skills[sk.SkillId] = slot
		}
		slot.XP += sk.XpDelta
		slot.Level = sk.NewLevel
	}
	for _, eq := range diff.Equipment {
		switch eq.Action {
		case pb.EquipAction_EQUIP_ACTION_EQUIP:
			s.Equipment[eq.Slot] = &EquippedItem{ID: eq.ItemId, State: eq.ItemState}
		case pb.EquipAction_EQUIP_ACTION_UNEQUIP:
			delete(s.Equipment, eq.Slot)
		}
	}
	for _, qd := range diff.EventQueue {
		entries := make([]EventQueueEntry, 0, len(qd.Entries))
		for _, e := range qd.Entries {
			entries = append(entries, EventQueueEntry{
				EventID:      e.EventId,
				TargetCycles: e.TargetCycles,
				Progress:     e.Progress,
			})
		}
		s.EventQueues[qd.QueueId] = entries
	}
}

// Logf appends a formatted message to the client log.
func (s *GameState) Logf(typ, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	s.Log = append(s.Log, LogEntry{Time: time.Now(), Type: typ, Message: msg})
	if len(s.Log) > s.LogLimit {
		s.Log = s.Log[len(s.Log)-s.LogLimit:]
	}
}

// --- Protobuf helpers ---

// DecodeStateDiff extracts a StateDiff from an envelope payload.
func DecodeStateDiff(env *pb.Envelope) (*pb.StateDiff, error) {
	var diff pb.StateDiff
	if err := proto.Unmarshal(env.Payload, &diff); err != nil {
		return nil, err
	}
	return &diff, nil
}

// DecodePong extracts a Pong from an envelope payload.
func DecodePong(env *pb.Envelope) (*pb.Pong, error) {
	var pong pb.Pong
	if err := proto.Unmarshal(env.Payload, &pong); err != nil {
		return nil, err
	}
	return &pong, nil
}

// DecodeEquipResponse extracts an EquipUnequipResponse from an envelope payload.
func DecodeEquipResponse(env *pb.Envelope) (*pb.EquipUnequipResponse, error) {
	var resp pb.EquipUnequipResponse
	if err := proto.Unmarshal(env.Payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
