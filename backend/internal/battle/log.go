package battle

// BattleLog is a single event in the combat log.
type BattleLog struct {
	Type string `json:"type"`

	// Attack logs.
	SkillID       string  `json:"skill_id,omitempty"`
	SkillName     string  `json:"skill_name,omitempty"`
	TargetID      string  `json:"target_id,omitempty"`
	Damage        float64 `json:"damage,omitempty"`
	RawDamage     float64 `json:"raw_damage,omitempty"`
	DamageType    string  `json:"damage_type,omitempty"`
	Evaded        bool    `json:"evaded,omitempty"`
	Blocked       bool    `json:"blocked,omitempty"`
	BlockedReduction float64 `json:"blocked_reduction,omitempty"`
	MPCost        float64 `json:"mp_cost,omitempty"`
	SPCost        float64 `json:"sp_cost,omitempty"`
	Effects       []AppliedEffect `json:"effects,omitempty"`

	// Target state after hit.
	TargetHP      float64 `json:"target_hp,omitempty"`
	PlayerHP      float64 `json:"player_hp,omitempty"`

	// Enemy-specific.
	EnemyID       string `json:"enemy_id,omitempty"`
	EnemyInstanceID string `json:"enemy_instance_id,omitempty"`
	EnemyName     string `json:"enemy_name,omitempty"`

	// Meta.
	WaveNumber    int     `json:"wave_number,omitempty"`
	NextWaveIn    float64 `json:"next_wave_in,omitempty"`
	BattleID      string  `json:"battle_id,omitempty"`
}

// AppliedEffect records an effect that was actually applied.
type AppliedEffect struct {
	Target    string  `json:"target"` // "self" | "target"
	Attribute string  `json:"attribute"`
	Mode      string  `json:"mode"`
	Value     float64 `json:"value"`
	Duration  float64 `json:"duration_seconds,omitempty"`
}
