package event

import (
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
)

// SettlementHook is a callback invoked around each Settle call.
type SettlementHook func(ctx SettlementCtx, delta float64)

// SettlementCtx is the interface the settlement engine uses to read/write
// player state. PlayerSession implements this by delegating to its subsystems.
type SettlementCtx interface {
	HasItem(it item.Item, qty float64) bool
	GetItemQty(it item.Item) float64
	AddItem(it item.Item, qty float64)
	DeductItem(it item.Item, qty float64)
	AddXP(skillID gameconfig.SkillID, xp float64)
	GetAttr(id attribute.AttributeID) float64
	GetSkillLevel(skillID gameconfig.SkillID) float64
	UnlockEvent(id gameconfig.EventID)
	IsEventUnlocked(id gameconfig.EventID) bool
}

// Settle advances the active event queues by delta seconds.
func (st *State) Settle(ctx SettlementCtx, delta float64) {
	if delta <= 0 {
		return
	}
	for _, h := range st.beforeHooks {
		h(ctx, delta)
	}
	remaining := delta
	for _, q := range st.queues {
		if remaining <= 0 {
			break
		}
		remaining = st.settleQueue(ctx, q, remaining)
	}
	for _, h := range st.afterHooks {
		h(ctx, delta)
	}
}

// BeforeSettle registers a hook that runs before each Settle cycle.
func (st *State) BeforeSettle(h SettlementHook) {
	st.beforeHooks = append(st.beforeHooks, h)
}

// AfterSettle registers a hook that runs after each Settle cycle.
func (st *State) AfterSettle(h SettlementHook) {
	st.afterHooks = append(st.afterHooks, h)
}

func (st *State) settleQueue(ctx SettlementCtx, q *Queue, delta float64) float64 {
	remaining := delta
	for remaining > 0 {
		idx := q.firstActive()
		if idx < 0 {
			break
		}
		ev, ok := gameconfig.GetEventByID(q.Entries[idx].EventID)
		if !ok {
			st.consume(q, idx)
			continue
		}

		if !st.checkReqs(ctx, ev) {
			if !st.swapSatisfied(ctx, q) {
				return remaining
			}
			continue
		}

		switch ev.Type {
		case gameconfig.EventTypeLoop:
			consumed := st.settleLoop(ctx, q, idx, ev, remaining)
			remaining -= consumed
		case gameconfig.EventTypeInstant, gameconfig.EventTypeUpgrade:
			st.settleInstant(ctx, q, idx, ev)
		}
	}
	return remaining
}

func (st *State) settleLoop(ctx SettlementCtx, q *Queue, idx int, ev gameconfig.Event, delta float64) float64 {
	lt := derefLoopTime(ev.LoopTime)
	if lt <= 0 {
		st.consume(q, idx)
		return 0
	}

	entry := &q.Entries[idx]

	timeCycles := int((delta + entry.Progress) / lt)
	if timeCycles == 0 {
		entry.Progress += delta
		st.markQueueCurrent(q.ID)
		return delta
	}

	actual := timeCycles
	if entry.TargetCycles > 0 && actual > entry.TargetCycles {
		actual = entry.TargetCycles
	}
	for _, req := range ev.Requirements {
		if !req.IsConsumption() || req.Value == nil {
			continue
		}
		held := ctx.GetItemQty(req.ResolvedItem)
		maxForThis := int(held / *req.Value)
		if maxForThis < actual {
			actual = maxForThis
		}
	}

	if actual == 0 {
		entry.Progress += delta
		st.markQueueCurrent(q.ID)
		return delta
	}

	factor := 1.0
	if attrID, ok := attribute.Get().AttrID(ev.ProductionAttrName); ok {
		mult := ctx.GetAttr(attrID)
		factor = 1.0 + mult
	}

	for _, req := range ev.Requirements {
		if !req.IsConsumption() || req.Value == nil {
			continue
		}
		ctx.DeductItem(req.ResolvedItem, *req.Value*float64(actual))
	}
	for _, rew := range ev.Rewards {
		switch {
		case rew.IsItem():
			ctx.AddItem(rew.ResolvedItem, rew.ItemQuantity()*float64(actual)*factor)
		case rew.IsExperience():
			ctx.AddXP(rew.ResolvedSkillID, rew.Value*float64(actual))
		}
	}

	ctx.UnlockEvent(entry.EventID)
	st.recordExecution(entry.EventID, actual)

	consumed := float64(actual) * lt
	entry.Progress = (delta + entry.Progress) - consumed

	if entry.TargetCycles > 0 {
		entry.TargetCycles -= actual
		if entry.TargetCycles <= 0 {
			st.consume(q, idx)
			return consumed
		}
	}

	st.markQueueCurrent(q.ID)
	return consumed
}

func (st *State) settleInstant(ctx SettlementCtx, q *Queue, idx int, ev gameconfig.Event) {
	entry := &q.Entries[idx]

	for _, rew := range ev.Rewards {
		switch {
		case rew.IsItem():
			ctx.AddItem(rew.ResolvedItem, rew.ItemQuantity())
		case rew.IsExperience():
			ctx.AddXP(rew.ResolvedSkillID, rew.Value)
		}
	}

	ctx.UnlockEvent(entry.EventID)
	st.recordExecution(entry.EventID, 1)
	st.consume(q, idx)
}

func (st *State) checkReqs(ctx SettlementCtx, ev gameconfig.Event) bool {
	for _, req := range ev.Requirements {
		if req.IsConsumption() {
			continue
		}
		switch req.Type {
		case string(gameconfig.ReqTypeSkill):
			if req.Value != nil && ctx.GetSkillLevel(gameconfig.SkillID(req.ResolvedID)) < *req.Value {
				return false
			}
		case string(gameconfig.ReqTypeEvent):
			if !ctx.IsEventUnlocked(gameconfig.EventID(req.ResolvedID)) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (st *State) swapSatisfied(ctx SettlementCtx, q *Queue) bool {
	cur := q.firstActive()
	if cur < 0 {
		return false
	}
	for i := cur + 1; i < len(q.Entries); i++ {
		ev, ok := gameconfig.GetEventByID(q.Entries[i].EventID)
		if !ok {
			continue
		}
		if st.checkReqs(ctx, ev) {
			q.Entries[cur], q.Entries[i] = q.Entries[i], q.Entries[cur]
			st.markQueueFull(q.ID)
			return true
		}
	}
	return false
}

func derefLoopTime(lt *float64) float64 {
	if lt == nil { return 0 }
	return *lt
}
