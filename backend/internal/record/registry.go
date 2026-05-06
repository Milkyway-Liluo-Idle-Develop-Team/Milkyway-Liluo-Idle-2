package record

import (
	"fmt"

	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"google.golang.org/protobuf/proto"
)

// Registry holds all registered SystemProviders and provides the global view
// needed to construct data packets.
type Registry struct {
	providers map[string]SystemProvider
	order     []string
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]SystemProvider),
	}
}

// Register adds a SystemProvider. Panics on duplicate system name so
// registration errors are caught at startup.
func (r *Registry) Register(p SystemProvider) {
	name := p.SystemName()
	if _, ok := r.providers[name]; ok {
		panic(fmt.Sprintf("record: duplicate system provider %q", name))
	}
	r.providers[name] = p
	r.order = append(r.order, name)
}

// Provider returns the SystemProvider for the given system name, or nil.
func (r *Registry) Provider(name string) SystemProvider {
	return r.providers[name]
}

// Providers returns all registered providers in registration order.
func (r *Registry) Providers() []SystemProvider {
	out := make([]SystemProvider, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.providers[name])
	}
	return out
}

// BuildDiff drains the Recorder's namespaces and produces the incremental
// diff as a StateDiff proto message.
//
// Processing:
//  1. PopAll namespaces from the Recorder.
//  2. For each system, merge same-system buckets across namespaces.
//  3. Collect non-empty buckets and serialize each via SerializeDiff.
//  4. Merge per-system results into the final StateDiff.
func (r *Registry) BuildDiff(rec *Recorder) (*pb.StateDiff, error) {
	namespaces := rec.PopAll()
	if len(namespaces) == 0 {
		return &pb.StateDiff{}, nil
	}

	merged := make(map[string]RecordBucket)
	for _, ns := range namespaces {
		for sysName, b := range ns.buckets {
			if b.IsEmpty() {
				continue
			}
			if existing, ok := merged[sysName]; ok {
				existing.MergeInPlace(b)
			} else {
				merged[sysName] = b
			}
		}
	}

	return r.buildDiffProto(merged)
}

// BuildFullSnapshot calls each registered provider's SerializeFull with the
// corresponding state object and assembles the complete state packet.
func (r *Registry) BuildFullSnapshot(states map[string]any) (*pb.StateFull, error) {
	stateFull := &pb.StateFull{}

	for _, name := range r.order {
		state, ok := states[name]
		if !ok {
			continue
		}
		prov := r.providers[name]
		data, err := prov.SerializeFull(state)
		if err != nil {
			return nil, fmt.Errorf("record: serialize full for %q: %w", name, err)
		}
		if data == nil {
			continue
		}
		mergeStateFull(stateFull, data)
	}

	return stateFull, nil
}

func (r *Registry) buildDiffProto(merged map[string]RecordBucket) (*pb.StateDiff, error) {
	stateDiff := &pb.StateDiff{}

	for _, name := range r.order {
		b, ok := merged[name]
		if !ok {
			continue
		}
		data, err := b.SerializeDiff()
		if err != nil {
			return nil, fmt.Errorf("record: serialize diff for %q: %w", name, err)
		}
		if data == nil {
			continue
		}
		mergeStateDiff(stateDiff, data)
	}

	return stateDiff, nil
}

// mergeStateDiff merges a per-system proto message (typically *pb.StateDiff
// with exactly one repeated field populated) into the accumulator.
func mergeStateDiff(acc *pb.StateDiff, msg proto.Message) {
	switch m := msg.(type) {
	case *pb.StateDiff:
		acc.Inventory = append(acc.Inventory, m.Inventory...)
		acc.Attribute = append(acc.Attribute, m.Attribute...)
		acc.SkillXp = append(acc.SkillXp, m.SkillXp...)
		acc.Bestiary = append(acc.Bestiary, m.Bestiary...)
		acc.EventExecution = append(acc.EventExecution, m.EventExecution...)
		acc.EventQueue = append(acc.EventQueue, m.EventQueue...)
		acc.Equipment = append(acc.Equipment, m.Equipment...)
	}
}

// mergeStateFull merges a per-system proto message (typically *pb.StateFull
// with exactly one repeated field populated) into the accumulator.
func mergeStateFull(acc *pb.StateFull, msg proto.Message) {
	switch m := msg.(type) {
	case *pb.StateFull:
		acc.Inventory = append(acc.Inventory, m.Inventory...)
		acc.Attribute = append(acc.Attribute, m.Attribute...)
		acc.SkillXp = append(acc.SkillXp, m.SkillXp...)
		acc.Bestiary = append(acc.Bestiary, m.Bestiary...)
		acc.EventExecution = append(acc.EventExecution, m.EventExecution...)
		if len(m.Equipment) > 0 {
			if acc.Equipment == nil {
				acc.Equipment = make(map[string]*pb.ItemIdentity, len(m.Equipment))
			}
			for slot, ident := range m.Equipment {
				acc.Equipment[slot] = ident
			}
		}
	}
}
