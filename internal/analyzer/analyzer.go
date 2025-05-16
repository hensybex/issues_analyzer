package analyzer

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/hensybex/issues_analyzer/upa/internal/config"
	"github.com/hensybex/issues_analyzer/upa/internal/report"
	"github.com/hensybex/issues_analyzer/upa/internal/runner"
)

func Run(lang, dir, out, aiderDir string, fix bool) error {
	cfg, ok := config.Supported[lang]
	if !ok {
		return errors.New("language not supported")
	}

	// replace {dir} placeholder
	replaceDir := func(args []string) []string {
		out := make([]string, len(args))
		for i, a := range args {
			out[i] = strings.ReplaceAll(a, "{dir}", dir)
		}
		return out
	}

	// pre-fix commands
	if fix && len(cfg.PreFixCmds) > 0 {
		for _, cmd := range cfg.PreFixCmds {
			_ = runner.Fatalf(dir, cmd...)
		}
	}

	// fix
	if fix && len(cfg.FixArgs) > 0 {
		fixArgs := replaceDir(cfg.FixArgs)
		_, _ = runner.Run(dir, append([]string{cfg.AnalyzerExe}, fixArgs...)...)
	}

	// build pass (Go only)
	w := report.NewWriter()
	if lang == "go" {
		if res, _ := runner.Run(dir, "go", "build", "./..."); res != nil && res.Stderr != "" {
			lines := strings.Split(strings.TrimSpace(res.Stderr), "\n")
			w.AddBuildErrors(lines)
		}
	}

	// analyze
	analArgs := replaceDir(cfg.AnalyzeArgs)
	res, err := runner.Run(dir, append([]string{cfg.AnalyzerExe}, analArgs...)...)
	if err != nil {
		return err
	}

	issues, err := cfg.Parser.Parse(res.Stdout, dir)
	if err != nil {
		return err
	}

	w.AddAiderPrompt()
	w.AddIssues(issues, filepath.Rel(dir, aiderDir))
	return w.Flush(out)
}
