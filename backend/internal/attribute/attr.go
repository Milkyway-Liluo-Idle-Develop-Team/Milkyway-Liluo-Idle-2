// Package attribute implements the RPG-style attribute system with a modifier
// stack (ADD →MULTIPLY →OVERRIDE), lazy evaluation, dirty propagation, and
// temporary computation contexts.
//
// The package is split into:
//   - attr.go: core types (AttributeID, Modifier, enums)
//   - registry.go: global read-only registry, attribute definitions, dep graph
//   - instance.go: per-PlayerSession modifier state, dirty tracking
//   - calculation.go: GetFinal, markDirty, dependency propagation
//   - context.go: temporary computation overrides (Context)
//   - loader.go: embedded data loading
//   - bucket.go: record.RecordBucket for diff tracking
//   - provider.go: record.SystemProvider for data packet construction
package attribute

import "fmt"

// AttributeID is a stable numeric identifier for an attribute.
// Zero means invalid / not found.
type AttributeID int32

// String returns the string representation, preferring the registered name.
func (id AttributeID) String() string {
	if id == 0 {
		return "<invalid-attr>"
	}
	if reg == nil {
		return fmt.Sprintf("<attr-%d>", int32(id))
	}
	if s, ok := reg.AttrString(id); ok {
		return s
	}
	return fmt.Sprintf("<attr-%d>", int32(id))
}

// OpType describes how a Modifier affects the target attribute.
type OpType string

const (
	OpAdd      OpType = "ADD"
	OpMultiply OpType = "MULTIPLY"
	OpOverride OpType = "OVERRIDE"
)

// DisplayMode controls how a modifier is shown in the frontend UI.
// It does not affect computation.
type DisplayMode string

const (
	DisplayFixed     DisplayMode = "FIXED"
	DisplayPercent   DisplayMode = "PERCENT"
	DisplayPerSecond DisplayMode = "PER_SECOND"
)

// Direction indicates whether a larger value is better or worse,
// used for frontend color coding.
type Direction string

const (
	DirPositive Direction = "positive"
	DirNegative Direction = "negative"
)

// Modifier is the smallest unit that changes an attribute's value.
// Either Value or RefAttr is set —never both.
type Modifier struct {
	AttrID   AttributeID // target attribute
	Op       OpType
	Value    float64      // fixed value (when RefAttr is 0)
	RefAttr  AttributeID  // reference to another attribute (when Value is 0)
	Display  DisplayMode
	Source   string       // "equipment:sword_001", "base", "buff:xxx", "skill:strength"
	Priority int
}

func (m Modifier) String() string {
	if m.RefAttr != 0 {
		return fmt.Sprintf("%s(%s ref=%d)", m.Op, m.AttrID, m.RefAttr)
	}
	return fmt.Sprintf("%s(%s %.3f)", m.Op, m.AttrID, m.Value)
}

// IsRef reports whether this modifier references another attribute.
func (m Modifier) IsRef() bool { return m.RefAttr != 0 }
