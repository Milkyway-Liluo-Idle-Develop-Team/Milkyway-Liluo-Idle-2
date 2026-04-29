package attribute

// Context provides temporary overrides for a single attribute computation.
// It does not affect the persistent modifier state, cache, or dirty flags.
//
// Typical use cases:
//   - Event settlement with a one-time buff
//   - Previewing the effect of an equipment upgrade
//   - Battle simulation with temporary stat changes
type Context struct {
	tempBase map[AttributeID]float64
	tempMods []Modifier
}

// NewContext creates an empty computation context.
func NewContext() *Context {
	return &Context{
		tempBase: make(map[AttributeID]float64),
	}
}

// SetBase overrides the default value for the given attribute within this
// context. The persistent default value is unchanged.
func (ctx *Context) SetBase(attrID AttributeID, value float64) {
	ctx.tempBase[attrID] = value
}

// AddMod adds a temporary ADD modifier for this computation.
func (ctx *Context) AddMod(attrID AttributeID, value float64) {
	ctx.tempMods = append(ctx.tempMods, Modifier{
		AttrID:  attrID,
		Op:      OpAdd,
		Value:   value,
		Display: DisplayFixed,
		Source:  "temp",
	})
}

// AddMult adds a temporary MULTIPLY modifier.
func (ctx *Context) AddMult(attrID AttributeID, multiplier float64) {
	ctx.tempMods = append(ctx.tempMods, Modifier{
		AttrID:  attrID,
		Op:      OpMultiply,
		Value:   multiplier,
		Display: DisplayPercent,
		Source:  "temp",
	})
}
