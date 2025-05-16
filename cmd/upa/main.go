package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hensybex/issues_analyzer/analyzer" // New path
)

func main() {
	lang := flag.String("language", "", "Language key: go, python, dart, flutter")
	dir := flag.String("dir", ".", "Project directory to analyze")
	out := flag.String("out", "project_analysis_report.txt", "Output report file")
	fix := flag.Bool("fix", false, "Attempt auto-fix")
	flag.Parse()

	if *lang == "" {
		fmt.Fprintln(os.Stderr, "-language is required")
		os.Exit(2)
	}

	rep, err := analyzer.Analyze(*lang, *dir, *fix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upa: %v\n", err)
		os.Exit(1)
	}

	// Записываем готовые строки в файл
	combined := rep.Linter + "\n" + rep.Compiler
	if err := os.WriteFile(*out, []byte(combined), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Analysis written to", *out)
}
