.PHONY: build test lint fmt run clean cover integration

BINARY=a9s
CMD=./cmd/a9s

build:
	go build -o $(BINARY) $(CMD)

test:
	go test ./internal/... ./tests/... -v -count=1 -coverprofile=coverage.out -covermode=atomic

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) coverage.out coverage.html

integration:
	go test -tags integration ./tests/integration/ -v -count=1 -timeout 60s

cover:
	go test ./internal/... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out
	@echo "---"
	@echo "Checking coverage thresholds..."
	@go tool cover -func=coverage.out | grep -E "^github.com/k2m30/a9s/internal/(aws|app)/" | awk '{print $$1, $$3}'
