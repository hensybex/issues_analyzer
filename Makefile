# Makefile for issues_analyzer

BIN         := upa
LANGUAGE    ?= flutter
DIR         ?= /Users/hensybex/Desktop/projects/shveya/app/lib/screens/admin
AIDER_DIR   ?= /Users/hensybex/Desktop/projects/shveya/app/
OUT         ?= project_analysis_report.txt

.PHONY: build run clean

build:
	@echo "→ Building $(BIN)…"
	go build -o $(BIN) ./cmd/upa

run: build
	@echo "→ Running analysis:"
	@echo "   language:   $(LANGUAGE)"
	@echo "   directory:  $(DIR)"
	@echo "   aider-dir:  $(AIDER_DIR)"
	@echo "   output:     $(OUT)"
	./$(BIN) \
	  -language   $(LANGUAGE) \
	  -dir        $(DIR) \
	  -aider-dir  $(AIDER_DIR) \
	  -out        $(OUT)

clean:
	@echo "→ Cleaning..."
	rm -f $(BIN) $(OUT)