// Command genregistry generates or updates id_registry.json from actions.json
// and attr_registry.json from attributes.json.
//
// Usage:
//
//	go run ./cmd/genregistry
//
// Inputs:
//
//	../base/actions.json    → internal/gameconfig/data/id_registry.json
//	../base/attributes.json → internal/attribute/data/attr_registry.json
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
)

func main() {
	// Paths relative to backend/ directory.
	actionsPath := filepath.Join("..", "base", "actions.json")
	attrPath := filepath.Join("..", "base", "attributes.json")

	registryPath := filepath.Join("internal", "gameconfig", "data", "id_registry.json")
	attrRegistryPath := filepath.Join("internal", "attribute", "data", "attr_registry.json")

	// Generate id_registry.json (items, events, skills, maps, battle_skills).
	if err := gameconfig.GenerateRegistry(actionsPath, registryPath); err != nil {
		fmt.Fprintf(os.Stderr, "id_registry: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("generated:", registryPath)

	// Generate attr_registry.json (attributes).
	if err := attribute.GenerateAttrRegistry(attrPath, attrRegistryPath); err != nil {
		fmt.Fprintf(os.Stderr, "attr_registry: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("generated:", attrRegistryPath)
}
