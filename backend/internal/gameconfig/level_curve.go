package gameconfig

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"strings"
)

//go:embed data/level_exp_requirement.CSV
var levelCSV []byte

// LevelCurveCSV returns the raw embedded level_exp_requirement.CSV bytes.
func LevelCurveCSV() []byte { return levelCSV }

// LoadLevelCurve returns the cumulative XP thresholds from the embedded CSV.
// The CSV format is: level,exp (header) followed by rows. Exp is cumulative.
func LoadLevelCurve() ([]float64, error) {
	r := csv.NewReader(strings.NewReader(string(levelCSV)))
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("level curve: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("level curve: empty file")
	}

	// Skip header.
	// Level 1 requires 0 XP; subsequent entries shift by one.
	curve := make([]float64, 0, len(rows))
	curve = append(curve, 0)
	for _, row := range rows[1:] {
		if len(row) < 2 {
			continue
		}
		exp, err := strconv.ParseFloat(row[1], 64)
		if err != nil {
			return nil, fmt.Errorf("level curve: row %q: %w", row[0], err)
		}
		// CSV stores scientific notation (1.00593E+11) which strconv handles.
		curve = append(curve, math.Round(exp))
	}
	return curve, nil
}
