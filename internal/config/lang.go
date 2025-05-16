// internal/config/lang.go
package config

import (
	"github.com/hensybex/issues_analyzer/internal/parser"
)

type LanguageConfig struct {
	AnalyzerExe string
	AnalyzeArgs []string
	FixArgs     []string
	PreFixCmds  [][]string
	Parser      parser.Parser
}

var Supported = map[string]LanguageConfig{
	"go": {
		AnalyzerExe: "golangci-lint",
		AnalyzeArgs: []string{"run", "{dir}", "--issues-exit-code=0", "--output.json.path", "stdout"},
		PreFixCmds:  [][]string{{"go", "fmt", "./..."}, {"goimports", "-w", "."}},
		Parser:      parser.GolangciParser{},
	},
	"python": {
		AnalyzerExe: "ruff",
		AnalyzeArgs: []string{"check", "{dir}", "--format=json", "--exit-zero"},
		FixArgs:     []string{"check", "{dir}", "--exit-zero", "--fix"},
		Parser:      parser.RuffParser{},
	},
	"dart": {
		AnalyzerExe: "dart",
		AnalyzeArgs: []string{"analyze", "--format=json"},
		FixArgs:     []string{"fix", "--apply"},
		Parser:      parser.DartParser{},
	},
	"flutter": {
		AnalyzerExe: "dart",
		AnalyzeArgs: []string{"analyze", "--format=json"},
		FixArgs:     []string{"fix", "--apply"},
		Parser:      parser.DartParser{},
	},
}
