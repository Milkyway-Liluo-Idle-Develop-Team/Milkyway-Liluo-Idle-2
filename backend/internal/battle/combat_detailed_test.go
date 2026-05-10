package battle_test

import (
	"testing"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
)

func init() {
	if !attribute.IsLoaded() {
		if err := attribute.Load(); err != nil {
			panic(err)
		}
	}
	battle.LoadAttrIDs()
}

func makeHighPowerPlayer(userID int64, name string) *battle.PlayerBattleEntity {
	inst := attribute.NewInstance()
	reg := attribute.Get()

	var mods []attribute.Modifier
	if id, ok := reg.AttrID("physical_power"); ok {
		mods = append(mods, attribute.Modifier{AttrID: id, Op: attribute.OpOverride, Value: 200, Source: "test"})
	}
	if id, ok := reg.AttrID("accuracy"); ok {
		mods = append(mods, attribute.Modifier{AttrID: id, Op: attribute.OpOverride, Value: 1000, Source: "test"})
	}
	if len(mods) > 0 {
		inst.AddModifiers("test", mods)
	}

	p := battle.NewPlayerBattleEntity(userID, name, inst)
	p.SetHP(p.MaxHP())
	p.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
		gameconfig.BattleSkillID(1): {
			ID: gameconfig.BattleSkillID(1),
			Name: "基础攻击",
			Damage: &battle.DamageProfile{
				Type:       "physical",
				Flat:       0,
				Multiplier: 1.0,
			},
			CastTime: 2.0,
			IsBasic:  true,
		},
	})
	p.SetBasicSkillID(gameconfig.BattleSkillID(1))
	p.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: gameconfig.BattleSkillID(1), Priority: 0}})
	return p
}

func makeWeakEnemy(instanceIdx int, name string, hp float64) *battle.EnemyBattleEntity {
	enemy := battle.NewEnemyBattleEntity(1, instanceIdx, name, map[string]float64{
		"hp":              hp,
		"defense":         0,
		"evade":           1,
		"attack_interval": 5,
	})
	enemy.SetHP(enemy.MaxHP())
	enemy.SetNextReadyTime(5.0)
	enemy.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
		gameconfig.BattleSkillID(1): {
			ID: gameconfig.BattleSkillID(1),
			Name: "基础攻击",
			Damage: &battle.DamageProfile{
				Type:       "physical",
				Flat:       0,
				Multiplier: 1.0,
			},
			CastTime: 2.0,
			IsBasic:  true,
		},
	})
	enemy.SetBasicSkillID(gameconfig.BattleSkillID(1))
	enemy.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: gameconfig.BattleSkillID(1), Priority: 0}})
	return enemy
}

// TestCombatLogFields verifies that every field in the BattleLog produced by
// a player attack is populated correctly.
func TestCombatLogFields(t *testing.T) {
	p := makeHighPowerPlayer(1, "Hero")
	enemy := makeWeakEnemy(0, "Goblin", 10)

	cfg := battle.BattleConfig{
		NumericID: 1,
		ID:        "test",
		Name:      "Test",
		Map:       "village",
		Interval:  10.0,
	}
	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p})
	sess.Enemies = append(sess.Enemies, enemy)
	sess.NextWaveTime = nil

	// Align player ready time with enemy so both attack at t=2.
	p.SetNextReadyTime(2.0)

	logs := sess.AdvanceByDelta(2.0)

	var attackLog *battle.BattleLog
	for i := range logs {
		if logs[i].Type == battle.BattleLogTypePlayerAttack {
			attackLog = &logs[i]
			break
		}
	}
	if attackLog == nil {
		t.Fatalf("expected player_attack log, got: %+v", logs)
	}

	if attackLog.SkillID != gameconfig.BattleSkillID(1) {
		t.Errorf("skill_id want basic_attack, got %d", attackLog.SkillID)
	}
	if attackLog.AttackerEntityID != p.EntityID() {
		t.Errorf("attacker_entity_id want %d, got %d", p.EntityID(), attackLog.AttackerEntityID)
	}
	if attackLog.DefenderEntityID != enemy.EntityID() {
		t.Errorf("defender_entity_id want %d, got %d", enemy.EntityID(), attackLog.DefenderEntityID)
	}
	if attackLog.Damage <= 0 {
		t.Errorf("damage should be > 0, got %v", attackLog.Damage)
	}
	if attackLog.Evaded {
		t.Error("attack should not have evaded")
	}
	if attackLog.DefenderHP < 0 {
		t.Errorf("defender_hp should be >= 0, got %v", attackLog.DefenderHP)
	}
}

// TestCombatHPChanges verifies defender HP drops after being attacked and that
// attacker MP/SP are deducted when the skill has costs.
func TestCombatHPChanges(t *testing.T) {
	p := makeHighPowerPlayer(1, "Hero")
	enemy := makeWeakEnemy(0, "Goblin", 50)

	cfg := battle.BattleConfig{
		NumericID: 1,
		ID:        "test",
		Name:      "Test",
		Map:       "village",
		Interval:  10.0,
	}
	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p})
	sess.Enemies = append(sess.Enemies, enemy)
	sess.NextWaveTime = nil

	p.SetNextReadyTime(2.0)

	enemyHPBefore := enemy.HP()
	playerMPBefore := p.MP()
	playerSPBefore := p.SP()

	logs := sess.AdvanceByDelta(2.0)

	var attackLog *battle.BattleLog
	for i := range logs {
		if logs[i].Type == battle.BattleLogTypePlayerAttack {
			attackLog = &logs[i]
			break
		}
	}
	if attackLog == nil {
		t.Fatalf("expected player_attack log")
	}

	// Defender HP should have dropped.
	if enemy.HP() >= enemyHPBefore {
		t.Errorf("enemy HP should drop after attack: before=%v after=%v", enemyHPBefore, enemy.HP())
	}
	// Defender HP in log should match actual HP.
	if enemy.HP() != attackLog.DefenderHP {
		t.Errorf("log defender_hp %v != actual enemy.HP() %v", attackLog.DefenderHP, enemy.HP())
	}

	// basic_attack has no MP/SP cost, so they should be unchanged.
	if p.MP() != playerMPBefore {
		t.Errorf("player MP should not change for zero-cost skill: before=%v after=%v", playerMPBefore, p.MP())
	}
	if p.SP() != playerSPBefore {
		t.Errorf("player SP should not change for zero-cost skill: before=%v after=%v", playerSPBefore, p.SP())
	}
}

// TestEnemyDeathCombatLog verifies that when an enemy is killed, both
// player_attack and enemy_died logs are emitted and the enemy is marked dead.
func TestEnemyDeathCombatLog(t *testing.T) {
	p := makeHighPowerPlayer(1, "Hero")
	// Very low HP enemy: will die in one hit.
	enemy := makeWeakEnemy(0, "Goblin", 5)

	cfg := battle.BattleConfig{
		NumericID: 1,
		ID:        "test",
		Name:      "Test",
		Map:       "village",
		Interval:  10.0,
	}
	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p})
	sess.Enemies = append(sess.Enemies, enemy)
	sess.NextWaveTime = nil

	p.SetNextReadyTime(2.0)

	logs := sess.AdvanceByDelta(2.0)

	var foundAttack, foundDeath bool
	for _, l := range logs {
		if l.Type == battle.BattleLogTypePlayerAttack {
			foundAttack = true
		}
		if l.Type == battle.BattleLogTypeEnemyDied {
			foundDeath = true
			if l.AttackerEntityID != enemy.EntityID() {
				t.Errorf("enemy_died attacker_entity_id want %d, got %d", enemy.EntityID(), l.AttackerEntityID)
			}
		}
	}
	if !foundAttack {
		t.Error("missing player_attack log")
	}
	if !foundDeath {
		t.Error("missing enemy_died log")
	}
	if enemy.Alive() {
		t.Error("enemy should be dead")
	}
}

// TestPlayerCooldownAfterAttack verifies that after a player attacks, their
// next_ready_time is advanced by the skill's cast_time.
func TestPlayerCooldownAfterAttack(t *testing.T) {
	p := makeHighPowerPlayer(1, "Hero")
	enemy := makeWeakEnemy(0, "Goblin", 50)

	cfg := battle.BattleConfig{
		NumericID: 1,
		ID:        "test",
		Name:      "Test",
		Map:       "village",
		Interval:  10.0,
	}
	sess := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{p})
	sess.Enemies = append(sess.Enemies, enemy)
	sess.NextWaveTime = nil

	p.SetNextReadyTime(2.0)

	logs := sess.AdvanceByDelta(2.0)

	var attackLog *battle.BattleLog
	for i := range logs {
		if logs[i].Type == battle.BattleLogTypePlayerAttack {
			attackLog = &logs[i]
			break
		}
	}
	if attackLog == nil {
		t.Fatalf("expected player_attack log")
	}

	// After attacking at t=2 with cast_time=2, next_ready should be t=4.
	if p.NextReadyTime() != 4.0 {
		t.Errorf("player next_ready_time want 4.0, got %v", p.NextReadyTime())
	}
	if p.LastSkillID() != gameconfig.BattleSkillID(1) {
		t.Errorf("player last_skill_id want basic_attack, got %d", p.LastSkillID())
	}
}
