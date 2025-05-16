BIN := upa
.PHONY: build run

build:
	go build -o $(BIN) ./cmd/upa

run: build
	./$(BIN) $(ARGS)