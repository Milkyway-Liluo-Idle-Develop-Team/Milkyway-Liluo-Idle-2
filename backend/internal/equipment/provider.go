package equipment

import (
	pb "github.com/edrowsluo/new-mli/backend/pb"
	"github.com/edrowsluo/new-mli/backend/internal/record"
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
	for slot, it := range all {
		out[slot] = &pb.ItemIdentity{
			ItemId:    int32(it.ID),
			ItemState: int32(it.State),
		}
	}
	return &pb.StateFull{Equipment: out}, nil
}
