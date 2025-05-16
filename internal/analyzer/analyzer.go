package analyzer

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
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

	// Regex to capture Go build errors:
	// Group 1: File path (anything not starting with '#', ':', or whitespace, followed by anything, non-greedy)
	// Group 2: Line number
	// Group 3: Column number (optional, non-capturing group for the colon, then capturing group for digits)
	// Group 4: Message
	errorRegex := regexp.MustCompile(`^([^:#\s].*?):([0-9]+):(?:([0-9]+):)?\s*(.*)`)

	// byDir stores errors structured by directory, then by file.
	// map[directoryPath]map[filePath][][3]string{lineNumber, columnNumber, message}
	byDir := make(map[string]map[string][][3]string)
	// dirs maintains the order of appearance of directories.
	var dirs []string

	foundErrors := false

	for _, line := range raw {
		matches := errorRegex.FindStringSubmatch(line)

		// A match will have 5 elements: the full match, then 4 capture groups.
		if len(matches) == 5 {
			foundErrors = true
			// matches[0] is the full matched string
			filePathFromCompiler := strings.TrimSpace(matches[1]) // Group 1: File path
			lineStr := strings.TrimSpace(matches[2])              // Group 2: Line number
			colStr := strings.TrimSpace(matches[3])               // Group 3: Column number (can be empty)
			msg := strings.TrimSpace(matches[4])                  // Group 4: Message

			if colStr == "" {
				colStr = "0" // Default column if not specified by the compiler output.
			}

			// Determine the absolute path of the reported file.
			// Compiler paths can be absolute or relative to the build directory (rootDir).
			var absFile string
			if filepath.IsAbs(filePathFromCompiler) {
				absFile = filepath.Clean(filePathFromCompiler)
			} else {
				absFile = filepath.Join(rootDir, filePathFromCompiler)
			}

			relFile, err := filepath.Rel(rootDir, absFile)
			if err != nil {
				relFile = absFile // If Rel fails, use the (cleaned) absolute path.
			}
			relFile = filepath.ToSlash(relFile) // Ensure consistent slash usage

			dir := filepath.ToSlash(filepath.Dir(relFile))
			if dir == "." { // Files in the rootDir itself
				dir = "" // Represent root with an empty string or a specific marker
			}

			if _, ok := byDir[dir]; !ok {
				byDir[dir] = make(map[string][][3]string)
				dirs = append(dirs, dir) // Add new directory to maintain order
			}
			// Ensure file entry exists for this directory
			if _, ok := byDir[dir][relFile]; !ok {
				byDir[dir][relFile] = make([][3]string, 0)
			}
			byDir[dir][relFile] = append(byDir[dir][relFile], [3]string{lineStr, colStr, msg})

		} else if strings.HasPrefix(line, "# ") {
			// This is a package directive line, e.g., "# path/to/package".
			// Currently ignored for structured error reporting, but could be logged.
		} else {
			// Line doesn't match common error pattern or known directives.
			// These could be linker errors, verbose messages, or unknown formats.
			// To capture "all", you might append these to a general "other/unparsed errors" section.
			// For now, they are skipped by this structured parser.
		}
	}

	if !foundErrors {
		sb.WriteString("No compiler errors found.\n")
		return sb.String()
	}

	// To sort directories alphabetically (optional, otherwise order of appearance is kept)
	// sort.Strings(dirs)

	for _, dir := range dirs {
		dirDisplayName := dir
		if dirDisplayName == "" {
			// For files in the root directory, display appropriately.
			// You could use ".", rootDir, or a custom label.
			dirDisplayName = "."
		}
		sb.WriteString("# " + dirDisplayName + "/:\n\n")

		// To sort files within a directory alphabetically (optional)
		// filesInDir := make([]string, 0, len(byDir[dir]))
		// for file := range byDir[dir] {
		// 	filesInDir = append(filesInDir, file)
		// }
		// sort.Strings(filesInDir)
		// for _, file := range filesInDir {
		//  errs := byDir[dir][file]
		//  ...
		// }

		// Current: Iterates files in the order they were added (effectively, by first error in that file)
		for file, errs := range byDir[dir] {
			sb.WriteString(file + ":\n\n")
			for _, e := range errs {
				lineNumber := atoi(e[0]) // Line string

				// Construct the absolute path for fs.Line
				// `file` is relFile here.
				pathForFsLine := filepath.Join(rootDir, file)
				if dir == "" && strings.HasPrefix(file, "/") { // If relFile somehow became abs path at root
					pathForFsLine = file
				}

				srcLine := strings.TrimSpace(fs.Line(pathForFsLine, lineNumber))
				sb.WriteString(srcLine + "\n")
				sb.WriteString(fmt.Sprintf("%s:%s: %s\n\n",
					e[0], e[1], e[2], // line, col, msg
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
