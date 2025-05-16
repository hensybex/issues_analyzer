package model

type Issue struct {
	File       string
	Line       int
	Column     int
	Message    string
	Suggestion string
	FromLinter string
}
