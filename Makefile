BIN := bin

.PHONY: test build run-worker run-manager clean

# Run the full test suite.
test:
	go test ./...

# Build all binaries into $(BIN)/.
build:
	go build -o $(BIN)/worker ./cmd/worker
	go build -o $(BIN)/manager ./cmd/manager

# Run each entrypoint directly.
run-worker:
	go run ./cmd/worker

run-manager:
	go run ./cmd/manager

clean:
	rm -rf $(BIN)
