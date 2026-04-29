package bestiary

import (
	"encoding/json"
	"sort"

	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// Provider implements record.SystemProvider for the bestiary system.
var Provider record.SystemProvider = &provider{}

type provider struct{}

func (p *provider) SystemName() string            { return "bestiary" }
func (p *provider) NewBucket() record.RecordBucket { return newBucket() }

func (p *provider) SerializeFull(state any) (json.RawMessage, error) {
	st, ok := state.(*State)
	if !ok {
		return json.RawMessage("null"), nil
	}

	type entry struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}

	var out []entry
	for eid := range st.events {
		out = append(out, entry{Type: "event", ID: eid.String()})
	}
	for it := range st.items {
		out = append(out, entry{Type: "item", ID: it.String()})
	}
	for mid := range st.areas {
		out = append(out, entry{Type: "area", ID: mid.String()})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].ID < out[j].ID
	})

	return json.Marshal(out)
}
