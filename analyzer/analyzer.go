// internal/analyzer/analyzer.go
package analyzer

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	// "sort"

	"github.com/hensybex/issues_analyzer/internal/config"
	"github.com/hensybex/issues_analyzer/internal/fs"
	"github.com/hensybex/issues_analyzer/internal/model"
)

// Analyze выполняет build (для Go) и lint/analyze, не падая на exit-кодах,
// и возвращает два блока с путями, отсчитанными от dir.
func Analyze(lang, dir string, fix bool) (Report, error) {
	cfg, ok := config.Supported[lang]
	if !ok {
		log.Printf("[ERROR] Language not supported: %s", lang)
		return Report{}, errors.New("language not supported")
	}

	var compileRaw cmdResult

	if lang == "go" {
		goBuildResult := runCommand(dir, "go", "build", "./...")
		compileRaw = goBuildResult
		// compileRaw.rawLines contains combined stderr and stdout
	}

	args := make([]string, len(cfg.AnalyzeArgs))
	for i, a := range cfg.AnalyzeArgs {
		args[i] = strings.ReplaceAll(a, "{dir}", dir)
	}
	if (lang == "dart" || lang == "flutter") && !containsDirPlaceholder(cfg.AnalyzeArgs) {
		args = append(args, dir)
	}

	lintRaw := runCommand(dir, cfg.AnalyzerExe, args...)

	var issues []model.Issue
	if trimmed := strings.TrimSpace(lintRaw.stdout); trimmed != "" {
		parsed, perr := cfg.Parser.Parse(lintRaw.stdout, dir)
		if perr != nil {
			log.Printf("[ERROR] Parse error for linter output: %v. Raw JSON: %s", perr, lintRaw.stdout)
			return Report{}, fmt.Errorf("parse error: %w. Raw JSON: %s", perr, lintRaw.stdout)
		}
		issues = parsed
	} else {
	}

	linterBlock := buildLinterBlock(dir, issues)
	var compilerBlock string
	if lang == "go" {
		compilerBlock = buildCompilerBlock(dir, compileRaw.rawLines)
	} else {
		compilerBlock = "--- Compiler Errors ---\nNot applicable for this language.\n"
	}

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

	err := cmd.Run()
	if err != nil {
		log.Printf("[INFO] Command %s %v finished with error: %v. Stderr might contain details.", exe, argv, err)
	}

	stdout := outBuf.String()
	stderr := errBuf.String()

	var lines []string
	combinedOutput := ""
	if stderr != "" {
		combinedOutput += stderr
	}
	if stdout != "" {
		if combinedOutput != "" {
			combinedOutput += "\n"
		}
		combinedOutput += stdout
	}

	if strings.TrimSpace(combinedOutput) != "" {
		for _, l := range strings.Split(strings.TrimRight(combinedOutput, "\n"), "\n") {
			lines = append(lines, l)
		}
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
		absIssueFile := is.File
		if !filepath.IsAbs(absIssueFile) {
			absIssueFile = filepath.Join(rootDir, is.File)
		}
		absIssueFile = filepath.Clean(absIssueFile)

		rel, err := filepath.Rel(rootDir, absIssueFile)
		if err != nil {
			log.Printf("[WARN] Failed to make path relative for linter: %s (rootDir: %s). Error: %v", absIssueFile, rootDir, err)
			rel = absIssueFile
		}
		rel = filepath.ToSlash(rel)
		is.File = rel

		if _, seen := byFile[rel]; !seen {
			files = append(files, rel)
		}
		byFile[rel] = append(byFile[rel], is)
	}

	for _, fileKey := range files { // Changed loop variable to fileKey for clarity
		sb.WriteString(fileKey + ":\n\n") // fileKey is the relative path
		for _, is := range byFile[fileKey] {
			pathForFsLine := filepath.Join(rootDir, fileKey) // Use fileKey (relative path) with rootDir
			src := strings.TrimSpace(fs.Line(pathForFsLine, is.Line))
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

	errorRegex := regexp.MustCompile(`^([^:#\s].*?):([0-9]+):(?:([0-9]+):)?\s*(.*)`)

	byDir := make(map[string]map[string][][3]string)
	var dirs []string
	foundErrors := false

	if len(raw) == 0 {
	}

	for _, line := range raw {
		matches := errorRegex.FindStringSubmatch(line)

		if len(matches) == 5 {
			foundErrors = true
			filePathFromCompiler := strings.TrimSpace(matches[1])
			lineStr := strings.TrimSpace(matches[2])
			colStr := strings.TrimSpace(matches[3])
			msg := strings.TrimSpace(matches[4])

			if colStr == "" {
				colStr = "0"
			}

			var absFile string
			if filepath.IsAbs(filePathFromCompiler) {
				absFile = filepath.Clean(filePathFromCompiler)
			} else {
				absFile = filepath.Join(rootDir, filePathFromCompiler)
				absFile = filepath.Clean(absFile)
			}

			relFile, err := filepath.Rel(rootDir, absFile)
			if err != nil {
				log.Printf("[WARN] buildCompilerBlock: filepath.Rel failed for absFile:'%s', rootDir:'%s'. Error: %v. Using absFile as relFile.", absFile, rootDir, err)
				relFile = absFile
			}
			relFile = filepath.ToSlash(relFile)

			dir := filepath.ToSlash(filepath.Dir(relFile))
			if dir == "." && relFile != "." {
				dir = ""
			} else if relFile == "." {
				dir = ""
			}

			if _, ok := byDir[dir]; !ok {
				byDir[dir] = make(map[string][][3]string)
				dirs = append(dirs, dir)
			}
			if _, ok := byDir[dir][relFile]; !ok {
				byDir[dir][relFile] = make([][3]string, 0)
			}
			byDir[dir][relFile] = append(byDir[dir][relFile], [3]string{lineStr, colStr, msg})

		} else if strings.HasPrefix(line, "# ") {
		} else if strings.TrimSpace(line) != "" {
		}
	}

	if !foundErrors {
		sb.WriteString("No compiler errors found.\n")
		if len(raw) > 0 {
			sb.WriteString(fmt.Sprintf(" (%d lines of raw compiler output were processed, but none matched the expected error format 'file:line:col:message'. The actual output received was:\n", len(raw)))
			for _, rawLine := range raw {
				sb.WriteString(fmt.Sprintf("  %s\n", rawLine))
			}
			sb.WriteString(")\n")
		}
		return sb.String()
	}

	for _, dirKey := range dirs { // Changed loop variable to dirKey for clarity
		dirDisplayName := dirKey
		if dirDisplayName == "" {
			dirDisplayName = "(project root)"
		} else {
			dirDisplayName = strings.TrimSuffix(dirDisplayName, "/")
		}
		sb.WriteString("# " + dirDisplayName + "/:\n\n")

		filesInDirKeys := make([]string, 0, len(byDir[dirKey]))
		for k := range byDir[dirKey] {
			filesInDirKeys = append(filesInDirKeys, k)
		}
		// sort.Strings(filesInDirKeys) // Optional

		for _, fileKey := range filesInDirKeys { // fileKey is the relative path
			errs := byDir[dirKey][fileKey]
			sb.WriteString(fileKey + ":\n\n")
			for _, e := range errs {
				lineNumber := atoi(e[0])

				truePathForFsLine := filepath.Join(rootDir, fileKey) // Use fileKey (relative path) with rootDir

				srcLine := strings.TrimSpace(fs.Line(truePathForFsLine, lineNumber))
				if strings.HasPrefix(srcLine, "[could not open source]") || strings.HasPrefix(srcLine, "[line out of range]") {
					log.Printf("[WARN] buildCompilerBlock: fs.Line failed for '%s:%d' - %s", truePathForFsLine, lineNumber, srcLine)
				}
				sb.WriteString(srcLine + "\n")
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
