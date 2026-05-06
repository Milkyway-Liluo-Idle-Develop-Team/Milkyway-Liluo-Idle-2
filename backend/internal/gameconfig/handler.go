package gameconfig

import (
	"encoding/json"
	"net/http"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/httpx"
)

// ConfigResponse bundles every static data file the client needs.
type ConfigResponse struct {
	Actions       json.RawMessage `json:"actions"`
	IDRegistry    json.RawMessage `json:"id_registry"`
	Attributes    json.RawMessage `json:"attributes"`
	AttrRegistry  json.RawMessage `json:"attr_registry"`
	LevelCurveCSV string          `json:"level_curve_csv"`
}

// ServeConfig returns the complete game configuration as a single JSON blob.
// This is consumed by the CLI (and future web client) on startup so it can
// resolve IDs, names, and formulas locally without repeated server round-trips.
func ServeConfig(w http.ResponseWriter, r *http.Request) {
	resp := ConfigResponse{
		Actions:       ActionsJSON(),
		IDRegistry:    RegistryJSON(),
		Attributes:    attribute.AttributesJSON(),
		AttrRegistry:  attribute.AttrRegistryJSON(),
		LevelCurveCSV: string(LevelCurveCSV()),
	}
	httpx.JSON(w, http.StatusOK, resp)
}
