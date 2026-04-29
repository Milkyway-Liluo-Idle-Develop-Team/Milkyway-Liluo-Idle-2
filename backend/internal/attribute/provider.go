package attribute

import (
	"encoding/json"
	"sort"

	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// Provider implements record.SystemProvider for the attribute system.
var Provider record.SystemProvider = &provider{}

type provider struct{}

func (p *provider) SystemName() string            { return "attribute" }
func (p *provider) NewBucket() record.RecordBucket { return &Bucket{dirty: make(map[AttributeID]bool)} }

// SerializeFull produces the complete attribute state for a full-snapshot
// packet. The state parameter must be an *Instance.
func (p *provider) SerializeFull(state any) (json.RawMessage, error) {
	inst, ok := state.(*Instance)
	if !ok {
		return json.RawMessage("null"), nil
	}

	type modFull struct {
		Source  string  `json:"source"`
		Op      string  `json:"op"`
		Value   float64 `json:"value,omitempty"`
		Display string  `json:"display,omitempty"`
	}

	type attrFull struct {
		AttrID     string    `json:"attr_id"`
		Name       string    `json:"name"`
		FinalValue float64   `json:"final_value"`
		Group      string    `json:"group"`
		Direction  Direction `json:"direction"`
		Modifiers  []modFull `json:"modifiers"`
	}

	allIDs := inst.reg.AllIDs()
	out := make([]attrFull, 0, len(allIDs))

	for _, id := range allIDs {
		def, _ := inst.reg.Def(id)
		finalVal := inst.GetFinal(id)
		mods := inst.ModifiersFor(id)

		wmods := make([]modFull, 0, len(mods))
		for _, m := range mods {
			mf := modFull{
				Source:  m.Source,
				Op:      string(m.Op),
				Display: string(m.Display),
			}
			if !m.IsRef() {
				mf.Value = m.Value
			}
			wmods = append(wmods, mf)
		}

		strID, _ := inst.reg.AttrString(id)
		out = append(out, attrFull{
			AttrID:     strID,
			Name:       def.Name,
			FinalValue: finalVal,
			Group:      def.Group,
			Direction:  def.Direction,
			Modifiers:  wmods,
		})
	}

	// Deterministic output by attribute ID.
	sort.Slice(out, func(i, j int) bool { return out[i].AttrID < out[j].AttrID })

	return json.Marshal(out)
}
