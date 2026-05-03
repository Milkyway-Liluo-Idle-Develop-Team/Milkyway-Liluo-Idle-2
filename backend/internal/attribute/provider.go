package attribute

import (
	pb "github.com/edrowsluo/new-mli/backend/pb"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

// Provider implements record.SystemProvider for the attribute system.
var Provider record.SystemProvider = &provider{}

type provider struct{}

func (p *provider) SystemName() string            { return "attribute" }
func (p *provider) NewBucket() record.RecordBucket { return &Bucket{dirty: make(map[AttributeID]bool)} }

// SerializeFull produces the complete attribute state for a full-snapshot
// packet. The state parameter must be an *Instance.
func (p *provider) SerializeFull(state any) (proto.Message, error) {
	inst, ok := state.(*Instance)
	if !ok {
		return nil, nil
	}

	allIDs := inst.reg.AllIDs()
	out := make([]*pb.AttributeFull, 0, len(allIDs))

	for _, id := range allIDs {
		def, _ := inst.reg.Def(id)
		finalVal := inst.GetFinal(id)
		mods := inst.ModifiersFor(id)

		wmods := make([]*pb.ModifierWire, 0, len(mods))
		for _, m := range mods {
			mf := &pb.ModifierWire{
				Source:  m.Source,
				Op:      string(m.Op),
				Display: string(m.Display),
			}
			if m.IsRef() {
				if s, ok := inst.reg.AttrString(m.RefAttr); ok {
					mf.RefAttr = s
				}
			} else {
				mf.Value = m.Value
			}
			wmods = append(wmods, mf)
		}

		strID, _ := inst.reg.AttrString(id)
		out = append(out, &pb.AttributeFull{
			AttrId:     strID,
			Name:       def.Name,
			FinalValue: finalVal,
			Group:      def.Group,
			Direction:  string(def.Direction),
			Modifiers:  wmods,
		})
	}

	return &pb.StateFull{Attribute: out}, nil
}
