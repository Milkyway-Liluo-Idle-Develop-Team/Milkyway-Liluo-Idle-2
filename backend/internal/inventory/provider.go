package inventory

import (
	"encoding/json"
	"sort"

	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// Provider implements record.SystemProvider for the inventory system.
var Provider record.SystemProvider = &provider{}

type provider struct{}

func (p *provider) SystemName() string            { return "inventory" }
func (p *provider) NewBucket() record.RecordBucket { return newBucket() }

func (p *provider) SerializeFull(state any) (json.RawMessage, error) {
	st, ok := state.(*State)
	if !ok {
		return json.RawMessage("null"), nil
	}

	type invFull struct {
		ItemID    int32   `json:"item_id"`
		ItemState int32   `json:"item_state"`
		Quantity  float64 `json:"quantity"`
	}

	all := st.All()
	out := make([]invFull, 0, len(all))
	for it, qty := range all {
		out = append(out, invFull{
			ItemID:    int32(it.ID),
			ItemState: int32(it.State),
			Quantity:  qty,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ItemID != out[j].ItemID {
			return out[i].ItemID < out[j].ItemID
		}
		return out[i].ItemState < out[j].ItemState
	})

	return json.Marshal(out)
}
