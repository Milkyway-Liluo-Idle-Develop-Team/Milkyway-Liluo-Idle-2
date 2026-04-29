package attribute

import "math"

// GetFinal returns the computed final value for the given attribute.
// If the attribute is not dirty and has a cached value, the cache is returned
// immediately (hot path). Otherwise a full recalculation is performed.
func (inst *Instance) GetFinal(attrID AttributeID) float64 {
	return inst.getFinal(attrID, nil, nil)
}

// GetFinalWithContext returns the computed value with temporary overrides.
// Temp modifiers and temp base values are applied on top of the persistent
// state but do not affect the cache or dirty flags.
func (inst *Instance) GetFinalWithContext(attrID AttributeID, ctx *Context) float64 {
	return inst.getFinal(attrID, ctx, nil)
}

func (inst *Instance) getFinal(attrID AttributeID, ctx *Context, visited map[AttributeID]bool) float64 {
	// Cache hit: not dirty, no temp context.
	if ctx == nil && !inst.dirty[attrID] {
		if v, ok := inst.cached[attrID]; ok {
			return v
		}
	}

	// Circular dependency detection.
	if visited == nil {
		visited = make(map[AttributeID]bool)
	}
	if visited[attrID] {
		// Circular reference: return current cached or default.
		def, _ := inst.reg.Def(attrID)
		return def.DefaultValue
	}
	visited[attrID] = true
	defer delete(visited, attrID)

	// Start with default value (or temp override).
	var value float64
	if ctx != nil {
		if v, ok := ctx.tempBase[attrID]; ok {
			value = v
		} else {
			value = inst.defaultValue(attrID)
		}
	} else {
		value = inst.defaultValue(attrID)
	}

	// Collect modifiers: persistent + temp.
	mods := inst.byAttr[attrID]
	if ctx != nil && len(ctx.tempMods) > 0 {
		for _, m := range ctx.tempMods {
			if m.AttrID == attrID {
				mods = append(mods, m)
			}
		}
	}

	// Phase 1: ADD.
	for _, m := range mods {
		if m.Op != OpAdd {
			continue
		}
		if m.IsRef() {
			value += inst.getFinal(m.RefAttr, ctx, visited)
		} else {
			value += m.Value
		}
	}

	// Phase 2: MULTIPLY (effect = 1 + value).
	for _, m := range mods {
		if m.Op != OpMultiply {
			continue
		}
		var multiplier float64
		if m.IsRef() {
			multiplier = inst.getFinal(m.RefAttr, ctx, visited)
		} else {
			multiplier = m.Value
		}
		value *= (1 + multiplier)
	}

	// Phase 3: OVERRIDE — last one wins.
	for _, m := range mods {
		if m.Op != OpOverride {
			continue
		}
		if m.IsRef() {
			value = inst.getFinal(m.RefAttr, ctx, visited)
		} else {
			value = m.Value
		}
	}

	// Clamp to [MinValue, MaxValue].
	def, ok := inst.reg.Def(attrID)
	if ok {
		if def.MinValue != nil {
			value = math.Max(value, *def.MinValue)
		}
		if def.MaxValue != nil {
			value = math.Min(value, *def.MaxValue)
		}
	}

	// Cache (only in non-context mode).
	if ctx == nil {
		inst.cached[attrID] = value
		inst.dirty[attrID] = false
	}

	return value
}

func (inst *Instance) defaultValue(attrID AttributeID) float64 {
	if def, ok := inst.reg.Def(attrID); ok {
		return def.DefaultValue
	}
	return 0
}

// markDirty marks the given attribute and all downstream dependent attributes
// as needing recalculation. If a Recorder is attached, writes a dirty record
// to the current namespace.
func (inst *Instance) markDirty(attrID AttributeID) {
	if inst.dirty[attrID] {
		return // already dirty, no need to propagate again
	}
	inst.dirty[attrID] = true

	// Write to recorder if in an execution cycle.
	if inst.recorder != nil {
		b := inst.recorder.Bucket("attribute")
		if b != nil {
			attrB := b.(*Bucket)
			attrB.setInstance(inst)
			attrB.MarkDirty(attrID)
		}
	}

	// Propagate to reverse dependencies.
	for _, depID := range inst.reg.ReverseDeps(attrID) {
		inst.markDirty(depID)
	}
}
