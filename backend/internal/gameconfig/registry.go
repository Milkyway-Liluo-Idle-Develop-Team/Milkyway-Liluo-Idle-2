package gameconfig

import (
	"encoding/json"
	"fmt"
	"sort"
)

// loadRegistry parses the embedded id_registry.json and validates basic
// invariants: no duplicate numeric ids within a category.
func loadRegistry() (*IDRegistry, error) {
	var reg IDRegistry
	if err := json.Unmarshal(registryJSON, &reg); err != nil {
		return nil, fmt.Errorf("unmarshal id_registry.json: %w", err)
	}

	if err := validateRegistry(&reg); err != nil {
		return nil, fmt.Errorf("validate registry: %w", err)
	}

	return &reg, nil
}

func validateRegistry(reg *IDRegistry) error {
	checkDup := func(name string, m map[string]int64) error {
		seen := make(map[int64]string, len(m))
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			id := m[k]
			if id <= 0 {
				return fmt.Errorf("%s: id for %q must be > 0, got %d", name, k, id)
			}
			if prev, ok := seen[id]; ok {
				return fmt.Errorf("%s: duplicate numeric id %d assigned to %q and %q", name, id, prev, k)
			}
			seen[id] = k
		}
		return nil
	}

	if err := checkDup("items", reg.Items); err != nil {
		return err
	}
	if err := checkDup("events", reg.Events); err != nil {
		return err
	}
	if err := checkDup("skills", reg.Skills); err != nil {
		return err
	}
	if err := checkDup("maps", reg.Maps); err != nil {
		return err
	}
	if err := checkDup("battle_skills", reg.BattleSkills); err != nil {
		return err
	}

	return nil
}

// checkConsistency verifies that every string id in actions.json has a
// matching entry in the registry.  This is a fatal error because missing
// mappings would lead to unstable ids at runtime.
func checkConsistency(reg *IDRegistry, cfg *ActionConfig) error {
	for _, it := range cfg.Items {
		if _, ok := reg.Items[it.ID]; !ok {
			return fmt.Errorf("item %q exists in actions.json but has no entry in id_registry.json; run `go run ./cmd/genregistry`", it.ID)
		}
	}
	for _, ev := range cfg.Events {
		if _, ok := reg.Events[ev.ID]; !ok {
			return fmt.Errorf("event %q exists in actions.json but has no entry in id_registry.json; run `go run ./cmd/genregistry`", ev.ID)
		}
		if ev.NeedSkill != "" {
			if _, ok := reg.Skills[ev.NeedSkill]; !ok {
				return fmt.Errorf("skill %q (need_skill of event %q) has no entry in id_registry.json; run `go run ./cmd/genregistry`", ev.NeedSkill, ev.ID)
			}
		}
		for _, req := range ev.Requirements {
			switch req.Type {
			case string(ReqTypeSkill):
				if _, ok := reg.Skills[req.ID]; !ok {
					return fmt.Errorf("skill %q (requirement of event %q) has no entry in id_registry.json; run `go run ./cmd/genregistry`", req.ID, ev.ID)
				}
			case string(ReqTypeFluid):
				// Fluids are a special form of items.
				if _, ok := reg.Items[req.ID]; !ok {
					return fmt.Errorf("fluid %q (requirement of event %q) has no entry in id_registry.json; run `go run ./cmd/genregistry`", req.ID, ev.ID)
				}
			}
		}
		for _, rew := range ev.Rewards {
			if rew.IsExperience() && rew.SkillID != "" {
				if _, ok := reg.Skills[rew.SkillID]; !ok {
					return fmt.Errorf("skill %q (reward of event %q) has no entry in id_registry.json; run `go run ./cmd/genregistry`", rew.SkillID, ev.ID)
				}
			}
		}
	}
	for _, it := range cfg.Items {
		if it.ToolDetails != nil {
			for _, req := range it.ToolDetails.Requirements {
				if req.Type == string(ReqTypeSkill) {
					if _, ok := reg.Skills[req.ID]; !ok {
						return fmt.Errorf("skill %q (tool requirement of item %q) has no entry in id_registry.json; run `go run ./cmd/genregistry`", req.ID, it.ID)
					}
				}
			}
		}
		if it.EquipmentDetails != nil {
			for _, req := range it.EquipmentDetails.Requirements {
				if req.Type == string(ReqTypeSkill) {
					if _, ok := reg.Skills[req.ID]; !ok {
						return fmt.Errorf("skill %q (equipment requirement of item %q) has no entry in id_registry.json; run `go run ./cmd/genregistry`", req.ID, it.ID)
					}
				}
			}
			for _, bs := range it.EquipmentDetails.BattleSkills {
				if _, ok := reg.BattleSkills[bs.ID]; !ok {
					return fmt.Errorf("battle_skill %q (item %q) has no entry in id_registry.json; run `go run ./cmd/genregistry`", bs.ID, it.ID)
				}
			}
		}
	}

	// Also verify that every map referenced by events is in the registry.
	for _, ev := range cfg.Events {
		if _, ok := reg.Maps[ev.Map]; !ok {
			return fmt.Errorf("map %q (event %q) has no entry in id_registry.json; run `go run ./cmd/genregistry`", ev.Map, ev.ID)
		}
	}

	return nil
}
