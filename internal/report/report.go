package report

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hensybex/issues_analyzer/upa/internal/fs"
	"github.com/hensybex/issues_analyzer/upa/internal/model"
)

const aiderPrompt = `The following is a list of linting/compiler errors from my project.
Please help me fix them. For each file, address the listed issues.
Apply changes directly to the files.`

type Writer struct {
	lines []string
}

func NewWriter() *Writer { return &Writer{} }

func (w *Writer) AddAiderPrompt() {
	w.lines = append(w.lines, aiderPrompt)
}

func (w *Writer) AddBuildErrors(lines []string) {
	w.lines = append(w.lines, "--- Go Build Errors ---")
	w.lines = append(w.lines, lines...)
}

func (w *Writer) AddIssues(issues []model.Issue, aiderRoot string) {
	if len(issues) == 0 {
		w.lines = append(w.lines, "No linter errors found.")
		return
	}
	relSet := make(map[string]struct{})
	for _, is := range issues {
		relSet[is.File] = struct{}{}
	}
	var addLine = "/add"
	for f := range relSet {
		addLine += " " + filepath.Join(aiderRoot, f)
	}
	w.lines = append(w.lines, addLine)

	for _, is := range issues {
		src := fs.Line(is.File, is.Line)
		w.lines = append(w.lines,
			fmt.Sprintf("\n%s:%d:%d: %s", is.File, is.Line, is.Column, is.Message),
			src,
		)
	}
}

func (w *Writer) Flush(outPath string) error {
	return os.WriteFile(outPath, []byte(joinLines(w.lines)), 0o644)
}

func joinLines(sl []string) string {
	out := ""
	for _, l := range sl {
		out += l + "\n"
	}
	return out
}
