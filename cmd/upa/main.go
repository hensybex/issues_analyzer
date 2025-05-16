package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hensybex/issues_analyzer/upa/internal/analyzer"
)

func main() {
	lang := flag.String("language", "", "Language key: go, python, dart, flutter")
	dir := flag.String("dir", ".", "Project directory to analyze")
	out := flag.String("out", "project_analysis_report.txt", "Output report file")
	fix := flag.Bool("fix", false, "Attempt auto-fix")
	aiderDir := flag.String("aider-dir", ".", "Root dir for /add paths")
	flag.Parse()

	if *lang == "" {
		fmt.Fprintln(os.Stderr, "-language is required")
		os.Exit(2)
	}

	if err := analyzer.Run(*lang, *dir, *out, *aiderDir, *fix); err != nil {
		fmt.Fprintf(os.Stderr, "upa: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Analysis written to", *out)
}
