package gameconfig

import _ "embed"

//go:embed data/actions.json
var actionsJSON []byte

//go:embed data/id_registry.json
var registryJSON []byte
