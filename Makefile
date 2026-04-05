.PHONY: build install test lint fmt run clean cover integration security coverage verify-readonly demo readme check-readme

BINARY   = a9s
CMD      = ./cmd/a9s
BUILD_TS ?= $(shell date -u +%Y%m%d%H%M%S)
VERSION ?= dev-$(BUILD_TS)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
GOFILES  = $(shell find . -type f -name '*.go' -not -path './vendor/*')

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

install:
	go install -trimpath -ldflags "$(LDFLAGS)" $(CMD)

test:
	go test ./tests/unit/ -count=1 -timeout 120s -race

lint:
	golangci-lint run ./...

fmt:
	gofmt -w $(GOFILES)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) coverage.out coverage.html

integration:
	go test -tags integration ./tests/integration/ -v -count=1 -timeout 60s

security:
	govulncheck ./...

coverage:
	go test ./internal/... ./tests/... -coverpkg=./internal/... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

cover: coverage

verify-readonly:
	@echo "Checking for write API calls in internal/aws/..."
	@if grep -rn '\.\(Create\|Delete\|Update\|Put\|Modify\|Terminate\|Stop\|Reboot\|RunInstances\|Execute\|Send\|Publish\|Remove\)[A-Z]' internal/aws/*.go \
		| grep -v '_test.go' \
		| grep -v 'errors.go' \
		| grep -v 'interfaces.go' \
		| grep -v 'client.go' \
		| grep -v 'profile.go' \
		| grep -v 'regions.go' \
		| grep -v '\/\/' \
		| grep -v 'Describe\|List\|Get\|Search\|Lookup\|BatchGet\|Scan' \
		| grep -v 'CreateDate\|CreateTime\|StartRecord\|StartTime\|StopTime\|StopDate' ; then \
		echo "FAIL: Write API calls detected!"; exit 1; \
	else \
		echo "PASS: All API calls are read-only"; \
	fi

demo:
	vhs docs/demos/demo.tape

readme:
	@scripts/generate-readme.sh > README.md
	@echo "README.md regenerated from docs/shared/ snippets"

check-readme:
	@tmpfile="$$(mktemp)"; \
	scripts/generate-readme.sh > "$$tmpfile"; \
	diff -q README.md "$$tmpfile" > /dev/null 2>&1 || (rm -f "$$tmpfile"; echo "FAIL: README.md is out of sync — run 'make readme'" && exit 1); \
	rm -f "$$tmpfile"
	@echo "PASS: README.md is in sync with docs/shared/"
