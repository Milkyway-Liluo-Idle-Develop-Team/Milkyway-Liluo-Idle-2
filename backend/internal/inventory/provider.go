package inventory

import (
	"sort"

	pb "github.com/edrowsluo/new-mli/backend/pb"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

// Provider implements record.SystemProvider for the inventory system.
var Provider record.SystemProvider = &provider{}

type provider struct{}

func (p *provider) SystemName() string            { return "inventory" }
func (p *provider) NewBucket() record.RecordBucket { return newBucket() }

func (p *provider) SerializeFull(state any) (proto.Message, error) {
	st, ok := state.(*State)
	if !ok {
		return nil, nil
	}

	all := st.All()
	out := make([]*pb.InventoryFull, 0, len(all))
	for it, qty := range all {
		out = append(out, &pb.InventoryFull{
			ItemId:    int32(it.ID),
			ItemState: int32(it.State),
			Quantity:  qty,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ItemId != out[j].ItemId {
			return out[i].ItemId < out[j].ItemId
		}
		return out[i].ItemState < out[j].ItemState
	})

	return &pb.StateFull{Inventory: out}, nil
}
