package main

import (
	"fmt"
	"github.com/hensybex/issues_analyzer/internal/analyzer"
)

func main() {
	// 1) протестим Go
	repGo, err := analyzer.Analyze("go", "/Users/hensybex/Desktop/projects/shveya/api", false)
	if err != nil {
		panic(err)
	}
	fmt.Println("=== Go Linter:")
	fmt.Println(repGo.Linter)
	fmt.Println("=== Go Compiler:")
	fmt.Println(repGo.Compiler)

	// 2) протестим Flutter
	repFl, err := analyzer.Analyze("flutter", "/Users/hensybex/Desktop/projects/shveya/app/", false)
	if err != nil {
		panic(err)
	}
	fmt.Println("=== FL Linter:")
	fmt.Println(repFl.Linter)
	fmt.Println("=== FL Compiler:")
	fmt.Println(repFl.Compiler)
}
