package skill

import (
	"encoding/json"

	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/record"
)

// Bucket collects SkillXpRecords within a single namespace.
// Records with the same SkillID are merged (XpDelta summed, NewLevel = last).
type Bucket struct {
	changes map[gameconfig.SkillID]*skillRec
}

type skillRec struct {
	xpDelta  float64
	newLevel float64
}

var _ record.RecordBucket = (*Bucket)(nil)

func newBucket() *Bucket {
	return &Bucket{changes: make(map[gameconfig.SkillID]*skillRec)}
}

func (b *Bucket) add(skillID gameconfig.SkillID, xpDelta, newLevel float64) {
	if r, ok := b.changes[skillID]; ok {
		r.xpDelta += xpDelta
		r.newLevel = newLevel
	} else {
		b.changes[skillID] = &skillRec{xpDelta: xpDelta, newLevel: newLevel}
	}
}

func (b *Bucket) SystemName() string { return "skill_xp" }

func (b *Bucket) MergeInPlace(other record.RecordBucket) {
	ob := other.(*Bucket)
	for id, r := range ob.changes {
		if existing, ok := b.changes[id]; ok {
			existing.xpDelta += r.xpDelta
			existing.newLevel = r.newLevel
		} else {
			b.changes[id] = &skillRec{xpDelta: r.xpDelta, newLevel: r.newLevel}
		}
	}
}

type skillWire struct {
	SkillID  int64   `json:"skill_id"`
	XpDelta  float64 `json:"xp_delta"`
	NewLevel float64 `json:"new_level"`
}

func (b *Bucket) SerializeDiff() (json.RawMessage, error) {
	if len(b.changes) == 0 {
		return json.RawMessage("[]"), nil
	}
	out := make([]skillWire, 0, len(b.changes))
	for id, r := range b.changes {
		out = append(out, skillWire{
			SkillID:  int64(id),
			XpDelta:  r.xpDelta,
			NewLevel: r.newLevel,
		})
	}
	return json.Marshal(out)
}

func (b *Bucket) IsEmpty() bool { return len(b.changes) == 0 }
