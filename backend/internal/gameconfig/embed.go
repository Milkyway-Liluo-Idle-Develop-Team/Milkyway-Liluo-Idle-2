package gameconfig

import _ "embed"

//go:embed data/actions.json
var actionsJSON []byte

//go:embed data/id_registry.json
var registryJSON []byte

// ActionsJSON returns the raw embedded actions.json bytes.
func ActionsJSON() []byte { return actionsJSON }

// RegistryJSON returns the raw embedded id_registry.json bytes.
func RegistryJSON() []byte { return registryJSON }
