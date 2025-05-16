package analyzer

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hensybex/issues_analyzer/internal/config"
	"github.com/hensybex/issues_analyzer/internal/fs"
	"github.com/hensybex/issues_analyzer/internal/model"
)

// Analyze выполняет build (для Go) и lint/analyze, не падая на exit-кодах,
// и возвращает два блока с путями, отсчитанными от dir.
func Analyze(lang, dir string, fix bool) (Report, error) {
	cfg, ok := config.Supported[lang]
	if !ok {
		return Report{}, errors.New("language not supported")
	}

	// 1) Build/compile step (Go only)
	compileRaw := runCommand(dir, "go", "build", "./...")

	// 2) Lint/Analysis step
	args := make([]string, len(cfg.AnalyzeArgs))
	for i, a := range cfg.AnalyzeArgs {
		args[i] = strings.ReplaceAll(a, "{dir}", dir)
	}
	if (lang == "dart" || lang == "flutter") && !containsDirPlaceholder(cfg.AnalyzeArgs) {
		args = append(args, dir)
	}
	lintRaw := runCommand(dir, cfg.AnalyzerExe, args...)

	// 3) Parse lint JSON → []Issue
	var issues []model.Issue
	if trimmed := strings.TrimSpace(lintRaw.stdout); trimmed != "" {
		parsed, perr := cfg.Parser.Parse(lintRaw.stdout, dir)
		if perr != nil {
			return Report{}, fmt.Errorf("parse error: %w", perr)
		}
		issues = parsed
	}

	// 4) Собираем текстовые блоки, делая пути относительными к dir
	linterBlock := buildLinterBlock(dir, issues)
	compilerBlock := buildCompilerBlock(dir, compileRaw.rawLines)

	return Report{Linter: linterBlock, Compiler: compilerBlock}, nil
}

func containsDirPlaceholder(args []string) bool {
	for _, a := range args {
		if strings.Contains(a, "{dir}") {
			return true
		}
	}
	return false
}

type cmdResult struct {
	stdout   string
	rawLines []string
}

func runCommand(dir, exe string, argv ...string) cmdResult {
	cmd := exec.Command(exe, argv...)
	cmd.Dir = dir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	_ = cmd.Run()

	stdout := outBuf.String()
	stderr := errBuf.String()

	lines := []string{}
	combined := stderr
	if combined != "" && stdout != "" {
		combined += "\n"
	}
	combined += stdout
	for _, l := range strings.Split(strings.TrimRight(combined, "\n"), "\n") {
		lines = append(lines, l)
	}
	return cmdResult{stdout: stdout, rawLines: lines}
}

func buildLinterBlock(rootDir string, issues []model.Issue) string {
	var sb strings.Builder
	sb.WriteString("--- Linter Errors ---\n")
	if len(issues) == 0 {
		sb.WriteString("No linter errors found.\n\n")
		return sb.String()
	}

	byFile := map[string][]model.Issue{}
	var files []string
	for _, is := range issues {
		rel, err := filepath.Rel(rootDir, is.File)
		if err != nil {
			rel = is.File
		}
		is.File = rel
		if _, seen := byFile[rel]; !seen {
			files = append(files, rel)
		}
		byFile[rel] = append(byFile[rel], is)
	}

	for _, file := range files {
		sb.WriteString(file + ":\n\n")
		for _, is := range byFile[file] {
			src := strings.TrimSpace(fs.Line(filepath.Join(rootDir, file), is.Line))
			sb.WriteString(src + "\n")
			sb.WriteString(fmt.Sprintf("%d:%d: %s\n\n",
				is.Line, is.Column, is.Message,
			))
		}
	}

	return sb.String()
}

func buildCompilerBlock(rootDir string, raw []string) string {
	var sb strings.Builder
	sb.WriteString("--- Compiler Errors ---\n")
	if len(raw) == 0 {
		sb.WriteString("No compiler errors found.\n")
		return sb.String()
	}

	byDir := map[string]map[string][][3]string{}
	var dirs []string

	for _, l := range raw {
		parts := strings.SplitN(l, ":", 4)
		if len(parts) < 4 {
			continue
		}
		absFile := parts[0]
		relFile, err := filepath.Rel(rootDir, absFile)
		if err != nil {
			relFile = absFile
		}
		dir := filepath.Dir(relFile)
		line, col, msg := parts[1], parts[2], strings.TrimSpace(parts[3])

		if _, ok := byDir[dir]; !ok {
			byDir[dir] = map[string][][3]string{}
			dirs = append(dirs, dir)
		}
		byDir[dir][relFile] = append(byDir[dir][relFile], [3]string{line, col, msg})
	}

	for _, dir := range dirs {
		sb.WriteString("# " + dir + ":\n\n")
		for file, errs := range byDir[dir] {
			sb.WriteString(file + ":\n\n")
			for _, e := range errs {
				src := strings.TrimSpace(fs.Line(filepath.Join(rootDir, file), atoi(e[0])))
				sb.WriteString(src + "\n")
				sb.WriteString(fmt.Sprintf("%s:%s: %s\n\n",
					e[0], e[1], e[2],
				))
			}
		}
	}
	return sb.String()
}

func atoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
