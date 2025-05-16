// internal/parser/dart.go
package parser

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/hensybex/issues_analyzer/internal/model"
)

// ------------------------------------------------------------------
// helpers -----------------------------------------------------------
// ------------------------------------------------------------------

// cleanPath removes any accidental double-prefix of projectDir and
// ensures the path is absolute+clean.
func cleanPath(projectDir, raw string) string {
	p := filepath.Clean(raw)

	// Already absolute?
	if filepath.IsAbs(p) {
		// If it contains two copies of projectDir, keep the second.
		if idx := strings.LastIndex(p, projectDir); idx != -1 {
			p = p[idx:]
		}
		return filepath.Clean(p)
	}

	// Relative → join once.
	return filepath.Join(projectDir, p)
}

// ------------------------------------------------------------------
// JSON shapes -------------------------------------------------------
// ------------------------------------------------------------------

type dartDiag struct {
	Diagnostics []struct {
		ProblemMessage string `json:"problemMessage"`
		Code           string `json:"code"`
		Location       struct {
			File  string `json:"file"`
			Range struct {
				Start struct {
					Line int `json:"line"`
					Col  int `json:"column"`
				} `json:"start"`
			} `json:"range"`
		} `json:"location"`
	} `json:"diagnostics"`
}

// ------------------------------------------------------------------
// Parser implementation --------------------------------------------
// ------------------------------------------------------------------

type DartParser struct{}

func (DartParser) Parse(jsonStr, projectDir string) ([]model.Issue, error) {
	var d dartDiag
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		return nil, err
	}

	var out []model.Issue
	for _, diag := range d.Diagnostics {
		abs := cleanPath(projectDir, diag.Location.File)

		out = append(out, model.Issue{
			File:       abs,
			Line:       diag.Location.Range.Start.Line,
			Column:     diag.Location.Range.Start.Col,
			Message:    diag.ProblemMessage,
			FromLinter: diag.Code,
		})
	}
	return out, nil
}
