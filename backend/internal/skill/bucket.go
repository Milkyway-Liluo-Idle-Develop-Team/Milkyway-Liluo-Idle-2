package skill

import (
	pb "github.com/edrowsluo/new-mli/backend/internal/pb"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
	"github.com/edrowsluo/new-mli/backend/internal/record"
	"google.golang.org/protobuf/proto"
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

func (b *Bucket) SerializeDiff() (proto.Message, error) {
	if len(b.changes) == 0 {
		return nil, nil
	}
	diffs := make([]*pb.SkillXPDiff, 0, len(b.changes))
	for id, r := range b.changes {
		diffs = append(diffs, &pb.SkillXPDiff{
			SkillId:  int64(id),
			XpDelta:  r.xpDelta,
			NewLevel: r.newLevel,
		})
	}
	return &pb.StateDiff{SkillXp: diffs}, nil
}

func (b *Bucket) IsEmpty() bool { return len(b.changes) == 0 }
