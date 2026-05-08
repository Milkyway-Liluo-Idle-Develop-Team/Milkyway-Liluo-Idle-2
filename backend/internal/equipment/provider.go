package equipment

import (
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

// Provider implements record.SystemProvider for the equipment system.
var Provider record.SystemProvider = &provider{}

type provider struct{}

func (p *provider) SystemName() string             { return "equipment" }
func (p *provider) NewBucket() record.RecordBucket { return newBucket() }

func (p *provider) SerializeFull(state any) (proto.Message, error) {
	st, ok := state.(*State)
	if !ok || st == nil {
		return nil, nil
	}

	all := st.All()
	if len(all) == 0 {
		return nil, nil
	}

	out := make(map[string]*pb.ItemIdentity, len(all))
	for slot, entry := range all {
		out[slot] = &pb.ItemIdentity{
			ItemId:    int32(entry.Item.ID),
			ItemState: int32(entry.Item.State),
		}
	}
	return &pb.StateFull{Equipment: out}, nil
}
