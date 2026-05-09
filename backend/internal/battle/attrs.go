package battle

import "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"

// well-known attribute IDs, populated once at init time.
// Callers must ensure attribute.Load() has been called before using these.
var (
	AttrPhysicalPower              attribute.AttributeID
	AttrMagicPower                 attribute.AttributeID
	AttrAccuracy                   attribute.AttributeID
	AttrAttackInterval             attribute.AttributeID
	AttrDefense                    attribute.AttributeID
	AttrHatredMultiplier           attribute.AttributeID
	AttrHPRecovery                 attribute.AttributeID
	AttrHP                         attribute.AttributeID
	AttrMP                         attribute.AttributeID
	AttrSP                         attribute.AttributeID
	AttrCritical                   attribute.AttributeID
	AttrCriticalRate               attribute.AttributeID
	AttrBlock                      attribute.AttributeID
	AttrBlockPossibilityMultiplier attribute.AttributeID
	AttrBlockRate                  attribute.AttributeID
	AttrEvade                      attribute.AttributeID
	AttrEvadePossibilityMultiplier attribute.AttributeID
	AttrAccuracyPossibilityMultiplier attribute.AttributeID
	AttrMagicInstance              attribute.AttributeID
	AttrFinalDamageMultiplier      attribute.AttributeID
	AttrFinalDamageReduce          attribute.AttributeID
	AttrHatred                     attribute.AttributeID
	AttrMPRecovery                 attribute.AttributeID
	AttrSPRecovery                 attribute.AttributeID
)

// LoadAttrIDs resolves numeric attribute IDs from the global registry.
// Safe to call multiple times (idempotent).
func LoadAttrIDs() {
	if AttrPhysicalPower != 0 {
		return // already loaded
	}
	reg := attribute.Get()
	AttrPhysicalPower = mustAttr("physical_power", reg)
	AttrMagicPower = mustAttr("magic_power", reg)
	AttrAccuracy = mustAttr("accuracy", reg)
	AttrAttackInterval = mustAttr("attack_interval", reg)
	AttrDefense = mustAttr("defense", reg)
	AttrHatredMultiplier = mustAttr("hatred_multiplier", reg)
	AttrHPRecovery = mustAttr("hp_recovery", reg)
	AttrHP = mustAttr("hp", reg)
	AttrMP = mustAttr("mp", reg)
	AttrSP = mustAttr("sp", reg)
	AttrCritical = mustAttr("critical", reg)
	AttrCriticalRate = mustAttr("critical_rate", reg)
	AttrBlock = mustAttr("block", reg)
	AttrBlockPossibilityMultiplier = mustAttr("block_possibility_multiplier", reg)
	AttrBlockRate = mustAttr("block_rate", reg)
	AttrEvade = mustAttr("evade", reg)
	AttrEvadePossibilityMultiplier = mustAttr("evade_possibility_multiplier", reg)
	AttrAccuracyPossibilityMultiplier = mustAttr("accuracy_possibility_multiplier", reg)
	AttrMagicInstance = mustAttr("magic_instance", reg)
	AttrFinalDamageMultiplier = mustAttr("final_damage_multiplier", reg)
	AttrFinalDamageReduce = mustAttr("final_damage_reduce", reg)
	AttrHatred = mustAttr("hatred", reg)
	AttrMPRecovery = mustAttr("mp_recovery", reg)
	AttrSPRecovery = mustAttr("sp_recovery", reg)
}

func mustAttr(name string, reg *attribute.Registry) attribute.AttributeID {
	id, ok := reg.AttrID(name)
	if !ok {
		panic("battle: missing attribute " + name)
	}
	return id
}
