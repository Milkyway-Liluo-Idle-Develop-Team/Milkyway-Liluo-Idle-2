package skill

import (
	"encoding/json"
	"sort"

	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// Provider implements record.SystemProvider for the skill system.
var Provider record.SystemProvider = &provider{}

type provider struct{}

func (p *provider) SystemName() string            { return "skill_xp" }
func (p *provider) NewBucket() record.RecordBucket { return newBucket() }

func (p *provider) SerializeFull(state any) (json.RawMessage, error) {
	st, ok := state.(*State)
	if !ok {
		return json.RawMessage("null"), nil
	}

	type skillFull struct {
		SkillID int64   `json:"skill_id"`
		Level   float64 `json:"level"`
		XP      float64 `json:"xp"`
	}

	all := st.All()
	out := make([]skillFull, 0, len(all))
	for id, slot := range all {
		out = append(out, skillFull{
			SkillID: int64(id),
			Level:   slot.Level,
			XP:      slot.XP,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].SkillID < out[j].SkillID })

	return json.Marshal(out)
}

// LoadCurve parses the embedded level_exp_requirement.CSV and returns
// a LevelCurve suitable for passing to skill.Load.
func LoadCurve() (LevelCurve, error) {
	return gameconfig.LoadLevelCurve()
}
