// Command genregistry generates or updates id_registry.json from actions.json.
//
// Usage:
//
//	go run ./cmd/genregistry
//
// The tool reads ../../base/actions.json and writes
// internal/gameconfig/data/id_registry.json.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edrowsluo/new-mli/backend/internal/gameconfig"
)

func main() {
	// Paths relative to backend/ directory (where go run is executed).
	actionsPath := filepath.Join("..", "base", "actions.json")
	registryPath := filepath.Join("internal", "gameconfig", "data", "id_registry.json")

	if err := gameconfig.GenerateRegistry(actionsPath, registryPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("generated:", registryPath)
}
