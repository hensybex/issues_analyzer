// internal/analyzer/analyzer.go
package analyzer

import (
	"bytes"
	"errors"
	"fmt"
	"log" // <- Added for logging
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	// "sort" // Add if you want to sort files/dirs alphabetically later

	"github.com/hensybex/issues_analyzer/internal/config"
	"github.com/hensybex/issues_analyzer/internal/fs"
	"github.com/hensybex/issues_analyzer/internal/model"
)

// Analyze выполняет build (для Go) и lint/analyze, не падая на exit-кодах,
// и возвращает два блока с путями, отсчитанными от dir.
func Analyze(lang, dir string, fix bool) (Report, error) {
	log.Printf("[DEBUG] Analyze called with lang=%s, dir=%s, fix=%t", lang, dir, fix)
	cfg, ok := config.Supported[lang]
	if !ok {
		log.Printf("[ERROR] Language not supported: %s", lang)
		return Report{}, errors.New("language not supported")
	}

	var compileRaw cmdResult // For Go, this will hold build output

	if lang == "go" {
		// 1) Build/compile step (Go only)
		log.Printf("[DEBUG] Running Go build step for directory: %s", dir)
		// Use a temporary variable to avoid shadowing if we make runCommand return more
		goBuildResult := runCommand(dir, "go", "build", "./...")
		compileRaw = goBuildResult                                   // Assign to compileRaw
		log.Printf("[DEBUG] Go build stdout: %s", compileRaw.stdout) // Log raw stdout
		// compileRaw.rawLines already contains combined stdout and stderr.
		// No need to log stderr separately if it's already in rawLines.
	}

	// 2) Lint/Analysis step
	args := make([]string, len(cfg.AnalyzeArgs))
	for i, a := range cfg.AnalyzeArgs {
		args[i] = strings.ReplaceAll(a, "{dir}", dir)
	}
	if (lang == "dart" || lang == "flutter") && !containsDirPlaceholder(cfg.AnalyzeArgs) {
		args = append(args, dir)
	}
	log.Printf("[DEBUG] Running linter: %s with args: %v (dir context: %s)", cfg.AnalyzerExe, args, dir)
	lintRaw := runCommand(dir, cfg.AnalyzerExe, args...)
	log.Printf("[DEBUG] Linter stdout: %s", lintRaw.stdout)
	// Log linter stderr if it's not empty and potentially interesting
	// For linters outputting JSON to stdout, stderr might contain useful debug info or errors.
	// However, runCommand already combines stderr into rawLines. For lintRaw, we mostly care about lintRaw.stdout.
	// If lintRaw.rawLines (which includes stderr) is needed for debugging linter itself:
	// log.Printf("[DEBUG] Linter combined raw lines output (%d lines)", len(lintRaw.rawLines))
	// for i, line := range lintRaw.rawLines {
	//    log.Printf("[DEBUG] Linter raw line %d: %s", i, line)
	// }

	// 3) Parse lint JSON → []Issue
	var issues []model.Issue
	if trimmed := strings.TrimSpace(lintRaw.stdout); trimmed != "" {
		log.Printf("[DEBUG] Attempting to parse linter JSON output (length: %d)", len(lintRaw.stdout))
		parsed, perr := cfg.Parser.Parse(lintRaw.stdout, dir)
		if perr != nil {
			log.Printf("[ERROR] Parse error for linter output: %v. Raw JSON: %s", perr, lintRaw.stdout)
			return Report{}, fmt.Errorf("parse error: %w. Raw JSON: %s", perr, lintRaw.stdout)
		}
		issues = parsed
		log.Printf("[DEBUG] Successfully parsed %d issues from linter output", len(issues))
	} else {
		log.Printf("[DEBUG] Linter JSON output was empty.")
	}

	// 4) Собираем текстовые блоки, делая пути относительными к dir
	linterBlock := buildLinterBlock(dir, issues)
	var compilerBlock string
	if lang == "go" {
		log.Printf("[DEBUG] Building compiler block for Go from %d raw lines", len(compileRaw.rawLines))
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
	rawLines []string // Combined stdout and stderr
}

func runCommand(dir, exe string, argv ...string) cmdResult {
	cmd := exec.Command(exe, argv...)
	cmd.Dir = dir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	log.Printf("[DEBUG] Executing command: %s %v in dir: %s", exe, argv, dir)
	err := cmd.Run()
	if err != nil {
		// Log the error from cmd.Run() itself, as it might indicate issues
		// like command not found, or non-zero exit that isn't an ExitError.
		// If it's an ExitError, stderr will contain the details.
		log.Printf("[INFO] Command %s %v finished with error: %v. Stderr might contain details.", exe, argv, err)
	}

	stdout := outBuf.String()
	stderr := errBuf.String()

	var lines []string
	// Combine stderr first, then stdout, as errors often go to stderr.
	// Ensure that if both are empty, combined is empty.
	// Ensure that if one is empty, no extra newline is added.
	combinedOutput := ""
	if stderr != "" {
		combinedOutput += stderr
	}
	if stdout != "" {
		if combinedOutput != "" { // Add newline only if stderr had content
			combinedOutput += "\n"
		}
		combinedOutput += stdout
	}

	if strings.TrimSpace(combinedOutput) != "" {
		for _, l := range strings.Split(strings.TrimRight(combinedOutput, "\n"), "\n") {
			lines = append(lines, l)
		}
	}

	log.Printf("[DEBUG] Command %s finished. Stdout len: %d, Stderr len: %d, Combined lines: %d", exe, len(stdout), len(stderr), len(lines))
	return cmdResult{stdout: stdout, rawLines: lines}
}

func buildLinterBlock(rootDir string, issues []model.Issue) string {
	var sb strings.Builder
	sb.WriteString("--- Linter Errors ---\n")
	if len(issues) == 0 {
		sb.WriteString("No linter errors found.\n\n")
		return sb.String()
	}
	log.Printf("[DEBUG] Building linter block with %d issues.", len(issues))

	byFile := map[string][]model.Issue{}
	var files []string // To maintain order of appearance
	for _, is := range issues {
		// Ensure is.File is absolute before making it relative
		absIssueFile := is.File
		if !filepath.IsAbs(absIssueFile) {
			absIssueFile = filepath.Join(rootDir, is.File) // Assuming is.File might be relative from parser
		}
		absIssueFile = filepath.Clean(absIssueFile)

		rel, err := filepath.Rel(rootDir, absIssueFile)
		if err != nil {
			log.Printf("[WARN] Failed to make path relative for linter: %s (rootDir: %s). Error: %v", absIssueFile, rootDir, err)
			rel = absIssueFile // Use the absolute path if Rel fails
		}
		rel = filepath.ToSlash(rel)
		is.File = rel // Update issue's file to relative path for reporting

		if _, seen := byFile[rel]; !seen {
			files = append(files, rel)
		}
		byFile[rel] = append(byFile[rel], is)
	}
	// sort.Strings(files) // Optional: sort files alphabetically for consistent report order

	for _, file := range files {
		sb.WriteString(file + ":\n\n")
		for _, is := range byFile[file] {
			// Path for fs.Line should be absolute or relative to CWD.
			// We need the original absolute path or reconstruct it carefully.
			// If `is.File` was made relative, join with rootDir.
			// Original `is.File` from parser might be absolute or relative to project dir.
			// `absIssueFile` captured above is the correct absolute path.
			// However, `fs.Line` needs the path as it is on disk.
			// `filepath.Join(rootDir, file)` should work if `file` is `rel`.

			pathForFsLine := filepath.Join(rootDir, file) // 'file' is rel here

			src := strings.TrimSpace(fs.Line(pathForFsLine, is.Line))
			sb.WriteString(src + "\n")
			sb.WriteString(fmt.Sprintf("%d:%d: %s\n\n",
				is.Line, is.Column, is.Message,
			))
		}
	}

	return sb.String()
}

// buildCompilerBlock is modified to use regex for more robust error parsing.
func buildCompilerBlock(rootDir string, raw []string) string {
	log.Printf("[DEBUG] buildCompilerBlock: Received %d raw lines to process for rootDir: %s", len(raw), rootDir)
	var sb strings.Builder
	sb.WriteString("--- Compiler Errors ---\n")

	errorRegex := regexp.MustCompile(`^([^:#\s].*?):([0-9]+):(?:([0-9]+):)?\s*(.*)`)

	byDir := make(map[string]map[string][][3]string)
	var dirs []string
	foundErrors := false

	if len(raw) == 0 {
		log.Printf("[DEBUG] buildCompilerBlock: No raw lines to process.")
	}

	for i, line := range raw {
		log.Printf("[DEBUG] buildCompilerBlock: Processing line %d: %s", i, line)
		matches := errorRegex.FindStringSubmatch(line)

		if len(matches) == 5 {
			foundErrors = true
			filePathFromCompiler := strings.TrimSpace(matches[1])
			lineStr := strings.TrimSpace(matches[2])
			colStr := strings.TrimSpace(matches[3])
			msg := strings.TrimSpace(matches[4])
			log.Printf("[DEBUG] buildCompilerBlock: Matched! File:'%s', Line:'%s', Col:'%s', Msg:'%s'", filePathFromCompiler, lineStr, colStr, msg)

			if colStr == "" {
				colStr = "0"
			}

			var absFile string
			if filepath.IsAbs(filePathFromCompiler) {
				absFile = filepath.Clean(filePathFromCompiler)
			} else {
				absFile = filepath.Join(rootDir, filePathFromCompiler)
				absFile = filepath.Clean(absFile) // Clean after join
			}
			log.Printf("[DEBUG] buildCompilerBlock: Absolute path determined: '%s'", absFile)

			relFile, err := filepath.Rel(rootDir, absFile)
			if err != nil {
				log.Printf("[WARN] buildCompilerBlock: filepath.Rel failed for absFile:'%s', rootDir:'%s'. Error: %v. Using absFile as relFile.", absFile, rootDir, err)
				relFile = absFile
			}
			relFile = filepath.ToSlash(relFile)
			log.Printf("[DEBUG] buildCompilerBlock: Relative path: '%s'", relFile)

			dir := filepath.ToSlash(filepath.Dir(relFile))
			if dir == "." && relFile != "." { // Avoid '.'" for top-level files, make it empty like before.
				dir = ""
			} else if relFile == "." { // if relFile itself is "." (e.g. error points to dir)
				dir = "" // Or handle as a special case
			}

			if _, ok := byDir[dir]; !ok {
				byDir[dir] = make(map[string][][3]string)
				dirs = append(dirs, dir)
			}
			if _, ok := byDir[dir][relFile]; !ok {
				byDir[dir][relFile] = make([][3]string, 0)
			}
			byDir[dir][relFile] = append(byDir[dir][relFile], [3]string{lineStr, colStr, msg})
			log.Printf("[DEBUG] buildCompilerBlock: Added error for dir:'%s', file:'%s'", dir, relFile)

		} else if strings.HasPrefix(line, "# ") {
			log.Printf("[DEBUG] buildCompilerBlock: Ignoring package directive line: %s", line)
		} else if strings.TrimSpace(line) != "" { // Don't log for empty lines
			log.Printf("[DEBUG] buildCompilerBlock: Line did not match error regex and is not a package directive: %s", line)
		}
	}

	if !foundErrors {
		log.Printf("[DEBUG] buildCompilerBlock: No parseable compiler errors found after processing all lines.")
		sb.WriteString("No compiler errors found.\n")
		if len(raw) > 0 { // If there was output but nothing parsed, mention it.
			sb.WriteString(fmt.Sprintf(" (%d lines of raw compiler output were processed)\n", len(raw)))
		}
		return sb.String()
	}
	log.Printf("[DEBUG] buildCompilerBlock: Found %d error entries across directories/files. Structure: %+v", len(byDir), byDir)

	for _, dir := range dirs {
		dirDisplayName := dir
		if dirDisplayName == "" {
			dirDisplayName = "(project root)" // Clarify display for root files
		} else {
			// Ensure it doesn't end with a slash if it's not root, then add one for display
			dirDisplayName = strings.TrimSuffix(dirDisplayName, "/")
		}
		sb.WriteString("# " + dirDisplayName + "/:\n\n")

		filesInDirKeys := make([]string, 0, len(byDir[dir]))
		for k := range byDir[dir] {
			filesInDirKeys = append(filesInDirKeys, k)
		}
		// sort.Strings(filesInDirKeys) // Optional: sort files for deterministic output

		for _, fileKey := range filesInDirKeys {
			errs := byDir[dir][fileKey]
			sb.WriteString(fileKey + ":\n\n")
			for _, e := range errs {
				lineNumber := atoi(e[0])

				// pathForFsLine should be the absolute path to the file
				// fileKey is relFile. So join with rootDir.
				//pathForFsLine := filepath.Join(rootDir, fileKey)
				// If fileKey was an absolute path (because Rel failed), Join might be weird.
				// Let's re-evaluate pathForFsLine based on how absFile was determined.
				// We need the original absolute path that corresponds to `fileKey`.
				// This is tricky because we only stored relFile as key.
				// For simplicity, assume filepath.Join(rootDir, fileKey) is generally correct.
				// A more robust way would be to store absFile with the error data.

				// fs.Line needs the absolute path of the source file.
				// `fileKey` is relative to `rootDir`.
				truePathForFsLine := filepath.Join(rootDir, fileKey)
				log.Printf("[DEBUG] buildCompilerBlock: Getting line %d for file (used for fs.Line): '%s'", lineNumber, truePathForFsLine)

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
