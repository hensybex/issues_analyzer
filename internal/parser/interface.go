package parser

import "github.com/hensybex/issues_analyzer/internal/model"

type Parser interface {
	Parse(json string, projectDir string) ([]model.Issue, error)
}
