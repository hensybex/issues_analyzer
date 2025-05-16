package parser

import (
	"encoding/json"

	"github.com/hensybex/issues_analyzer/internal/model"
)

type golangciIssue struct {
	Text       string `json:"Text"`
	FromLinter string `json:"FromLinter"`
	Pos        struct {
		Filename string `json:"Filename"`
		Line     int    `json:"Line"`
		Column   int    `json:"Column"`
	} `json:"Pos"`
}

type GolangciParser struct{}

func (GolangciParser) Parse(jsonStr, _ string) ([]model.Issue, error) {
	var wrap struct {
		Issues []golangciIssue `json:"Issues"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &wrap); err != nil {
		return nil, err
	}
	var out []model.Issue
	for _, is := range wrap.Issues {
		out = append(out, model.Issue{
			File:       is.Pos.Filename,
			Line:       is.Pos.Line,
			Column:     is.Pos.Column,
			Message:    is.Text,
			FromLinter: is.FromLinter,
		})
	}
	return out, nil
}
