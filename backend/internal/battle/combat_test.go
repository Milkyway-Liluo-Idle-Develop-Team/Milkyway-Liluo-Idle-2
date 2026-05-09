package battle

import (
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
)

func TestSkillWouldApplyEffect(t *testing.T) {
	ppID, _ := attribute.Get().AttrID("physical_power")
	defID, _ := attribute.Get().AttrID("defense")

	caster := NewPlayerBattleEntity(1, "Caster", attribute.NewInstance())
	target := NewEnemyBattleEntity("goblin", "goblin_0", "Goblin", map[string]float64{
		"hp": 200,
	})

	now := 10.0
	expireSoon := 5.0
	expireLater := 20.0

	buffSkill := &BattleSkill{
		ID:   "buff:power",
		Name: "Power Buff",
		Effects: []SkillEffect{
			{Target: "target", Attribute: ppID, Mode: EffectModeFlat, Value: 10, Duration: 30},
		},
	}

	// Case 1: no existing effect → should apply.
	if !skillWouldApplyEffect(buffSkill, caster, target, now) {
		t.Error("expected true when no existing effect")
	}

	// Case 2: identical effect already active (same value, not expired) → should NOT apply.
	target.ApplyEffect(ActiveEffect{
		SourceSkillID: "buff:power",
		Attribute:     ppID,
		Mode:          EffectModeFlat,
		Value:         10,
		ExpiresAt:     &expireLater,
	}, now)
	if skillWouldApplyEffect(buffSkill, caster, target, now) {
		t.Error("expected false when identical effect is already active")
	}

	// Case 3: same key but different value → should apply (overwrite).
	buffSkillDifferent := &BattleSkill{
		ID:   "buff:power",
		Name: "Power Buff Stronger",
		Effects: []SkillEffect{
			{Target: "target", Attribute: ppID, Mode: EffectModeFlat, Value: 20, Duration: 30},
		},
	}
	if !skillWouldApplyEffect(buffSkillDifferent, caster, target, now) {
		t.Error("expected true when value differs")
	}

	// Case 4: existing effect has expired → should apply.
	target2 := NewEnemyBattleEntity("goblin", "goblin_1", "Goblin", map[string]float64{
		"hp": 200,
	})
	target2.ApplyEffect(ActiveEffect{
		SourceSkillID: "buff:power",
		Attribute:     ppID,
		Mode:          EffectModeFlat,
		Value:         10,
		ExpiresAt:     &expireSoon,
	}, now)
	if !skillWouldApplyEffect(buffSkill, caster, target2, now) {
		t.Error("expected true when existing effect has expired")
	}

	// Case 5: self-target buff, no existing effect on caster → should apply.
	selfBuffSkill := &BattleSkill{
		ID:   "buff:self",
		Name: "Self Buff",
		Effects: []SkillEffect{
			{Target: "self", Attribute: defID, Mode: EffectModeFlat, Value: 5, Duration: 30},
		},
	}
	if !skillWouldApplyEffect(selfBuffSkill, caster, target, now) {
		t.Error("expected true for self-target with no existing effect")
	}

	// Case 6: skill with multiple effects, one unchanged + one new → should apply.
	mixedSkill := &BattleSkill{
		ID:   "buff:mixed",
		Name: "Mixed Buff",
		Effects: []SkillEffect{
			{Target: "target", Attribute: ppID, Mode: EffectModeFlat, Value: 10, Duration: 30},
			{Target: "target", Attribute: defID, Mode: EffectModeFlat, Value: 5, Duration: 30},
		},
	}
	// target already has ppID=10 from case 2, but not defID.
	if !skillWouldApplyEffect(mixedSkill, caster, target, now) {
		t.Error("expected true when at least one effect would change")
	}

	// Case 7: all effects already present with same values → should NOT apply.
	target3 := NewEnemyBattleEntity("goblin", "goblin_2", "Goblin", map[string]float64{
		"hp": 200,
	})
	target3.ApplyEffect(ActiveEffect{
		SourceSkillID: "buff:mixed",
		Attribute:     ppID,
		Mode:          EffectModeFlat,
		Value:         10,
		ExpiresAt:     &expireLater,
	}, now)
	target3.ApplyEffect(ActiveEffect{
		SourceSkillID: "buff:mixed",
		Attribute:     defID,
		Mode:          EffectModeFlat,
		Value:         5,
		ExpiresAt:     &expireLater,
	}, now)
	if skillWouldApplyEffect(mixedSkill, caster, target3, now) {
		t.Error("expected false when all effects already present with same values")
	}
}
