package battle

// chooseEnemyTarget picks a player for the enemy to attack based on accumulated
// hate plus static hatred attributes. If no hate exists, it falls back to uniform
// random selection among alive players.
func (s *BattleSession) chooseEnemyTarget(enemy *EnemyBattleEntity, candidates []*PlayerBattleEntity) *PlayerBattleEntity {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	type weighted struct {
		player *PlayerBattleEntity
		weight float64
	}

	weights := make([]weighted, 0, len(candidates))
	enemyHate := s.HateMap[enemy.EntityID()]
	for _, p := range candidates {
		w := s.computeTargetWeight(p, enemyHate)
		weights = append(weights, weighted{player: p, weight: w})
	}

	// If all weights are zero, fall back to uniform random.
	total := 0.0
	for _, w := range weights {
		total += w.weight
	}
	if total <= 0 {
		return candidates[s.rng.Intn(len(candidates))]
	}

	// Weighted random selection.
	r := s.rng.Float64() * total
	for _, w := range weights {
		r -= w.weight
		if r <= 0 {
			return w.player
		}
	}
	return candidates[len(candidates)-1]
}

// computeTargetWeight calculates how attractive a player is as a target.
// Base weight = accumulated hate from damage dealt to this enemy.
// Then scaled by the player's hatred attributes (higher hatred = more likely to be targeted).
func (s *BattleSession) computeTargetWeight(p *PlayerBattleEntity, enemyHate map[int64]float64) float64 {
	hate := enemyHate[p.EntityID()]

	// Static hatred multiplier from player attributes.
	hatred := p.GetFinal(AttrHatred)
	hatredMult := p.GetFinal(AttrHatredMultiplier)
	if hatredMult < 0 {
		hatredMult = 0
	}

	// Weighted combination: accumulated hate dominates, but static hatred
	// acts as a baseline so even a player who hasn't attacked yet can draw aggro.
	baseWeight := 1.0 + hatred*(1.0+hatredMult)
	return hate + baseWeight
}
