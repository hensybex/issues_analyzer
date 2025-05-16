package parser

import (
	"encoding/json"

	"github.com/hensybex/issues_analyzer/upa/internal/model"
)

type ruffDiag struct {
	Filename string `json:"filename"`
	Message  string `json:"message"`
	Code     string `json:"code"`
	Location struct {
		Row int `json:"row"`
		Col int `json:"column"`
	} `json:"location"`
}

type RuffParser struct{}

func (RuffParser) Parse(jsonStr, _ string) ([]model.Issue, error) {
	var diags []ruffDiag
	if err := json.Unmarshal([]byte(jsonStr), &diags); err != nil {
		return nil, err
	}
	var out []model.Issue
	for _, d := range diags {
		out = append(out, model.Issue{
			File:       d.Filename,
			Line:       d.Location.Row,
			Column:     d.Location.Col,
			Message:    d.Message,
			FromLinter: d.Code,
		})
	}
	return out, nil
}
