package main

import (
	"fmt"
	"github.com/hensybex/issues_analyzer/analyzer"
)

func main() {
	// 1) протестим Go
	repGo, err := analyzer.Analyze("go", "/Users/hensybex/Desktop/projects/autocoder/api", false)
	if err != nil {
		panic(err)
	}
	fmt.Println("=== Go Linter:")
	fmt.Println(repGo.Linter)
	fmt.Println("=== Go Compiler:")
	fmt.Println(repGo.Compiler)
}
