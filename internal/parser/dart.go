package parser

import (
	"encoding/json"
	"path/filepath"

	"github.com/hensybex/issues_analyzer/upa/internal/model"
)

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

type DartParser struct{}

func (DartParser) Parse(jsonStr, projectDir string) ([]model.Issue, error) {
	var d dartDiag
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		return nil, err
	}
	var out []model.Issue
	for _, diag := range d.Diagnostics {
		abs := filepath.Join(projectDir, diag.Location.File)
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
