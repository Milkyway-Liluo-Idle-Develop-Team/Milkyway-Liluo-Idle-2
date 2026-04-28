package record

import (
	"encoding/json"
	"fmt"
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
// diff JSON payload.
//
// Processing:
//  1. PopAll namespaces from the Recorder.
//  2. For each system, take the first namespace's bucket as the accumulator.
//  3. Merge buckets from remaining namespaces into the accumulator.
//  4. Collect non-empty buckets and serialize each via the provider.
//  5. Assemble the final JSON object keyed by "{system}_changes".
func (r *Registry) BuildDiff(rec *Recorder) (json.RawMessage, error) {
	namespaces := rec.PopAll()
	if len(namespaces) == 0 {
		return json.RawMessage("{}"), nil
	}

	// For each system, merge its buckets across all namespaces.
	// First pass: find the first non-empty bucket per system as the base.
	merged := make(map[string]RecordBucket)

	for _, ns := range namespaces {
		for sysName, b := range ns.buckets {
			if b.IsEmpty() {
				continue
			}
			if existing, ok := merged[sysName]; ok {
				existing.MergeInPlace(b)
			} else {
				// Shallow copy the bucket so we don't mutate the original.
				merged[sysName] = b
			}
		}
	}

	return r.buildDiffJSON(merged)
}

// BuildFullSnapshot calls each registered provider's SerializeFull with the
// corresponding state object and assembles the complete state packet.
func (r *Registry) BuildFullSnapshot(states map[string]any) (json.RawMessage, error) {
	parts := make([]jsonPiece, 0, len(states))

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
		if data == nil || string(data) == "null" {
			continue
		}
		parts = append(parts, jsonPiece{key: name, value: data})
	}

	return assembleJSON(parts), nil
}

func (r *Registry) buildDiffJSON(merged map[string]RecordBucket) (json.RawMessage, error) {
	parts := make([]jsonPiece, 0, len(merged))

	// Use registration order for deterministic output.
	for _, name := range r.order {
		b, ok := merged[name]
		if !ok {
			continue
		}
		data, err := b.SerializeDiff()
		if err != nil {
			return nil, fmt.Errorf("record: serialize diff for %q: %w", name, err)
		}
		if data == nil || string(data) == "null" || string(data) == "[]" {
			continue
		}
		parts = append(parts, jsonPiece{key: name + "_changes", value: data})
	}

	return assembleJSON(parts), nil
}

// --- JSON helpers ---

type jsonPiece struct {
	key   string
	value json.RawMessage
}

func assembleJSON(parts []jsonPiece) json.RawMessage {
	if len(parts) == 0 {
		return json.RawMessage("{}")
	}

	buf := []byte("{")
	for i, p := range parts {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = append(buf, p.key...)
		buf = append(buf, '"', ':')
		buf = append(buf, p.value...)
	}
	buf = append(buf, '}')
	return json.RawMessage(buf)
}
