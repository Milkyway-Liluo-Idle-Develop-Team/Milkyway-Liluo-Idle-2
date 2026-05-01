package bestiary

import (
	"sort"

	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
)

// Provider implements record.SystemProvider for the bestiary system.
var Provider record.SystemProvider = &provider{}

type provider struct{}

func (p *provider) SystemName() string            { return "bestiary" }
func (p *provider) NewBucket() record.RecordBucket { return newBucket() }

func (p *provider) SerializeFull(state any) (proto.Message, error) {
	st, ok := state.(*State)
	if !ok {
		return nil, nil
	}

	var out []*pb.BestiaryFull
	for eid := range st.events {
		out = append(out, &pb.BestiaryFull{Type: "event", Id: eid.String()})
	}
	for it := range st.items {
		out = append(out, &pb.BestiaryFull{Type: "item", Id: it.String()})
	}
	for mid := range st.areas {
		out = append(out, &pb.BestiaryFull{Type: "area", Id: mid.String()})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Id < out[j].Id
	})

	return &pb.StateFull{Bestiary: out}, nil
}
