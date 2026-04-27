package gameconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// IDRegistry is the on-disk format that maps string identifiers to stable
// numeric ids.  It is the single source of truth for numeric id assignment.
// Once an id is allocated it must never be reused, even if the corresponding
// entity is later removed from actions.json.
type IDRegistry struct {
	Version      string           `json:"version"`
	Items        map[string]int64 `json:"items"`
	Events       map[string]int64 `json:"events"`
	Skills       map[string]int64 `json:"skills"`
	Maps         map[string]int64 `json:"maps"`
	BattleSkills map[string]int64 `json:"battle_skills"`
}

// GenerateRegistry reads actions.json and produces (or updates) an
// IDRegistry.  It is safe to call repeatedly: existing mappings are preserved
// and only new string ids receive fresh ids (max+1 in each category).
//
// Typical usage from a small CLI or a go:generate directive:
//
//	go run ./cmd/genregistry
func GenerateRegistry(actionsPath, registryPath string) error {
	data, err := os.ReadFile(actionsPath)
	if err != nil {
		return fmt.Errorf("read actions.json: %w", err)
	}

	var cfg ActionConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse actions.json: %w", err)
	}

	// Load existing registry if present.
	var existing IDRegistry
	if b, err := os.ReadFile(registryPath); err == nil {
		if err := json.Unmarshal(b, &existing); err != nil {
			return fmt.Errorf("parse existing registry: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read existing registry: %w", err)
	}

	// Seed from existing or create fresh maps.
	reg := IDRegistry{
		Version:      existing.Version,
		Items:        copyMap(existing.Items),
		Events:       copyMap(existing.Events),
		Skills:       copyMap(existing.Skills),
		Maps:         copyMap(existing.Maps),
		BattleSkills: copyMap(existing.BattleSkills),
	}

	// Collect all string ids from actions.json.
	itemIDs := make([]string, 0, len(cfg.Items))
	for _, it := range cfg.Items {
		itemIDs = append(itemIDs, it.ID)
	}
	sort.Strings(itemIDs)

	eventIDs := make([]string, 0, len(cfg.Events))
	for _, ev := range cfg.Events {
		eventIDs = append(eventIDs, ev.ID)
	}
	sort.Strings(eventIDs)

	skillIDs := collectSkillIDs(cfg)
	fluidIDs := collectFluidIDs(cfg) // fluids are treated as items
	mapIDs := collectMapIDs(cfg)
	bsIDs := collectBattleSkillIDs(cfg)

	// Fluids are a special form of items; merge fluid IDs into items.
	allItemIDs := append(itemIDs, fluidIDs...)
	sort.Strings(allItemIDs)

	// Allocate fresh ids for anything new.
	mergeIDs(reg.Items, allItemIDs)
	mergeIDs(reg.Events, eventIDs)
	mergeIDs(reg.Skills, skillIDs)
	mergeIDs(reg.Maps, mapIDs)
	mergeIDs(reg.BattleSkills, bsIDs)

	// Detect removed ids (present in registry but absent from actions.json).
	removed := detectRemoved(reg.Items, allItemIDs)
	removed = append(removed, detectRemoved(reg.Events, eventIDs)...)
	removed = append(removed, detectRemoved(reg.Skills, skillIDs)...)
	removed = append(removed, detectRemoved(reg.Maps, mapIDs)...)
	removed = append(removed, detectRemoved(reg.BattleSkills, bsIDs)...)
	for _, r := range removed {
		fmt.Fprintln(os.Stderr, "warn: id_registry contains deprecated entry", r)
	}

	// Pretty-print with deterministic key order.
	out, err := marshalRegistry(reg)
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	if err := os.WriteFile(registryPath, out, 0o644); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}
	return nil
}

func copyMap(m map[string]int64) map[string]int64 {
	if m == nil {
		return make(map[string]int64)
	}
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func mergeIDs(dst map[string]int64, keys []string) {
	max := int64(0)
	for _, v := range dst {
		if v > max {
			max = v
		}
	}
	for _, k := range keys {
		if _, ok := dst[k]; !ok {
			max++
			dst[k] = max
		}
	}
}

func detectRemoved(dst map[string]int64, keys []string) []string {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	var out []string
	for k := range dst {
		if _, ok := set[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}

func collectSkillIDs(cfg ActionConfig) []string {
	set := make(map[string]struct{})
	for _, ev := range cfg.Events {
		if ev.NeedSkill != "" {
			set[ev.NeedSkill] = struct{}{}
		}
		for _, req := range ev.Requirements {
			if req.Type == string(ReqTypeSkill) {
				set[req.ID] = struct{}{}
			}
		}
		for _, rew := range ev.Rewards {
			if rew.IsExperience() && rew.SkillID != "" {
				set[rew.SkillID] = struct{}{}
			}
		}
	}
	for _, it := range cfg.Items {
		if it.ToolDetails != nil {
			for _, req := range it.ToolDetails.Requirements {
				if req.Type == string(ReqTypeSkill) {
					set[req.ID] = struct{}{}
				}
			}
		}
		if it.EquipmentDetails != nil {
			for _, req := range it.EquipmentDetails.Requirements {
				if req.Type == string(ReqTypeSkill) {
					set[req.ID] = struct{}{}
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectFluidIDs(cfg ActionConfig) []string {
	set := make(map[string]struct{})
	for _, ev := range cfg.Events {
		for _, req := range ev.Requirements {
			if req.Type == string(ReqTypeFluid) {
				set[req.ID] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectMapIDs(cfg ActionConfig) []string {
	set := make(map[string]struct{})
	for _, ev := range cfg.Events {
		set[ev.Map] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectBattleSkillIDs(cfg ActionConfig) []string {
	set := make(map[string]struct{})
	for _, it := range cfg.Items {
		if it.EquipmentDetails != nil {
			for _, bs := range it.EquipmentDetails.BattleSkills {
				set[bs.ID] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// marshalRegistry writes the registry as compact JSON with sorted keys.
func marshalRegistry(reg IDRegistry) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	buf.WriteString(fmt.Sprintf("  \"version\": %q,\n", reg.Version))

	sections := []struct {
		name string
		m    map[string]int64
	}{
		{"items", reg.Items},
		{"events", reg.Events},
		{"skills", reg.Skills},
		{"maps", reg.Maps},
		{"battle_skills", reg.BattleSkills},
	}

	for si, sec := range sections {
		buf.WriteString(fmt.Sprintf("  \"%s\": {\n", sec.name))
		keys := make([]string, 0, len(sec.m))
		for k := range sec.m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for ki, k := range keys {
			buf.WriteString(fmt.Sprintf("    %q: %d", k, sec.m[k]))
			if ki < len(keys)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString("  }")
		if si < len(sections)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}
