package skill

import (
	"sort"

	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

// Provider implements record.SystemProvider for the skill system.
var Provider record.SystemProvider = &provider{}

type provider struct{}

func (p *provider) SystemName() string            { return "skill_xp" }
func (p *provider) NewBucket() record.RecordBucket { return newBucket() }

func (p *provider) SerializeFull(state any) (proto.Message, error) {
	st, ok := state.(*State)
	if !ok {
		return nil, nil
	}

	all := st.All()
	out := make([]*pb.SkillXPFull, 0, len(all))
	for id, slot := range all {
		out = append(out, &pb.SkillXPFull{
			SkillId: int64(id),
			Level:   slot.Level,
			Xp:      slot.XP,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].SkillId < out[j].SkillId })

	return &pb.StateFull{SkillXp: out}, nil
}

// LoadCurve parses the embedded level_exp_requirement.CSV and returns
// a LevelCurve suitable for passing to skill.Load.
func LoadCurve() (LevelCurve, error) {
	return gameconfig.LoadLevelCurve()
}
