package attribute

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
)

//go:embed data/attributes.json
var attributesJSON []byte

//go:embed data/attr_registry.json
var attrRegistryJSON []byte

// AttributesJSON returns the raw embedded attributes.json bytes.
func AttributesJSON() []byte { return attributesJSON }

// AttrRegistryJSON returns the raw embedded attr_registry.json bytes.
func AttrRegistryJSON() []byte { return attrRegistryJSON }

// reg is the global, read-only registry populated at startup.
var reg *Registry

// Load parses the embedded data files, validates consistency, and builds the
// global Registry. It is safe to call multiple times (idempotent).
func Load() error {
	if reg != nil {
		return nil
	}

	var cfg attrsConfig
	if err := json.Unmarshal(attributesJSON, &cfg); err != nil {
		return fmt.Errorf("attribute: unmarshal attributes.json: %w", err)
	}

	var attrReg attrRegistryFile
	if err := json.Unmarshal(attrRegistryJSON, &attrReg); err != nil {
		return fmt.Errorf("attribute: unmarshal attr_registry.json: %w", err)
	}

	// Convert map[string]int32 →map[string]AttributeID.
	attrIDs := make(map[string]AttributeID, len(attrReg.Attributes))
	for s, id := range attrReg.Attributes {
		attrIDs[s] = AttributeID(id)
	}

	r, err := newRegistry(cfg, attrIDs)
	if err != nil {
		return fmt.Errorf("attribute: build registry: %w", err)
	}

	reg = r
	return nil
}

// Get returns the global attribute registry. Panics if Load has not been
// called or returned an error.
func Get() *Registry {
	if reg == nil {
		panic("attribute: Load() must be called before Get()")
	}
	return reg
}

// IsLoaded reports whether Load has been called successfully.
func IsLoaded() bool { return reg != nil }

// attrRegistryFile is the on-disk format of attr_registry.json.
type attrRegistryFile struct {
	Version    string           `json:"version"`
	Attributes map[string]int32 `json:"attributes"`
}

// GenerateAttrRegistry reads attributes.json and produces (or updates) an
// attribute registry file. It preserves existing mappings and only allocates
// new ids for new attribute string ids (max+1).
func GenerateAttrRegistry(attrPath, registryPath string) error {
	data, err := os.ReadFile(attrPath)
	if err != nil {
		return fmt.Errorf("read attributes.json: %w", err)
	}

	var cfg attrsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse attributes.json: %w", err)
	}

	// Load existing registry if present.
	var existing attrRegistryFile
	existingMap := make(map[string]int32)
	if b, err := os.ReadFile(registryPath); err == nil {
		if err := json.Unmarshal(b, &existing); err != nil {
			return fmt.Errorf("parse existing attr_registry.json: %w", err)
		}
		existingMap = existing.Attributes
	}
	if existingMap == nil {
		existingMap = make(map[string]int32)
	}

	// Merge: preserve existing IDs, allocate new ones.
	max := int32(0)
	for _, id := range existingMap {
		if id > max {
			max = id
		}
	}

	for _, def := range cfg.Attributes {
		if _, ok := existingMap[def.ID]; !ok {
			max++
			existingMap[def.ID] = max
		}
	}

	out := attrRegistryFile{
		Version:    existing.Version,
		Attributes: existingMap,
	}

	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal attr_registry: %w", err)
	}
	raw = append(raw, '\n')

	if err := os.WriteFile(registryPath, raw, 0644); err != nil {
		return fmt.Errorf("write attr_registry: %w", err)
	}
	return nil
}
