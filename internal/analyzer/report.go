package analyzer

// Report хранит два готовых блока текста — для линтера и для компиляции.
type Report struct {
	Linter   string
	Compiler string
}
