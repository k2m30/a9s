.PHONY: build install test test-race lint gofix fmt run clean cover integration security coverage verify-readonly demo readme check-readme mdlint snapshot snapshot-update ready-to-push ready-to-release generate

BINARY   = a9s
CMD      = ./cmd/a9s
BUILD_TS ?= $(shell date -u +%Y%m%d%H%M%S)
VERSION ?= dev-$(BUILD_TS)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
GOFILES  = $(shell find . -type f -name '*.go' -not -path './vendor/*')

generate:
	go generate ./...

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

install:
	go install -trimpath -ldflags "$(LDFLAGS)" $(CMD)

test:
	go test ./tests/unit/ -count=1 -timeout 120s

test-race:
	go test ./tests/unit/ -count=1 -timeout 120s -race

lint:
	golangci-lint run ./...

gofix:
	@if go fix -inline -diff ./... 2>&1 | grep -q '^'; then \
		echo "FAIL: unfixed //go:fix inline directives — run 'go fix -inline ./...'"; exit 1; \
	else \
		echo "PASS: no unfixed inline directives"; \
	fi

fmt:
	gofmt -w $(GOFILES)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) coverage.out coverage.html

integration:
	go test -tags integration ./tests/integration/ -v -count=1 -timeout 300s

security:
	govulncheck ./...

mdlint:
	markdownlint-cli2 "docs/**/*.md" "CLAUDE.md" "CONTRIBUTING.md" "CHANGELOG.md"

coverage:
	go test ./internal/... ./tests/... -coverpkg=./internal/... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

cover: coverage

# False-positive exclusions (line 56+):
#   CreateDate/CreateTime/StartRecord/StartTime/StopTime/StopDate — timestamp field names, not API calls
#   ExecuteCommandConfiguration — ECS cluster struct field read via NavigableField, not an API call
verify-readonly:
	@echo "Checking for write API calls in internal/aws/..."
	@if grep -rn '\.\(Create\|Delete\|Update\|Put\|Modify\|Terminate\|Stop\|Reboot\|RunInstances\|Execute\|Send\|Publish\|Remove\)[A-Z][A-Za-z0-9]*(' internal/aws/*.go \
		| grep -v '_test.go' \
		| grep -v 'errors.go' \
		| grep -v 'interfaces.go' \
		| grep -v '_interfaces.go' \
		| grep -v 'client.go' \
		| grep -v 'profile.go' \
		| grep -v 'regions.go' \
		| grep -v '\/\/' \
		| grep -v 'Describe\|List\|Get\|Search\|Lookup\|BatchGet\|Scan' \
		| grep -v 'CreateDate\|CreateTime\|StartRecord\|StartTime\|StopTime\|StopDate' \
		| grep -v 'ExecuteCommandConfiguration' ; then \
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

# Snapshot tests run the existing golden-file tests (issue119, issue140,
# ec2_related_view, ctdetail_demo, scenario_*_visual_test.go) and verify the
# rendered output matches the committed golden files byte-for-byte.
# Use `make snapshot-update` to regenerate goldens after intentional changes.
snapshot:
	go test ./tests/unit/ -run 'Golden|Scenario' -count=1
	go test -tags integration ./tests/integration/ -run 'Visual|Scenario' -count=1

snapshot-update:
	UPDATE_GOLDEN=1 go test ./tests/unit/ -run 'Golden|Scenario' -count=1
	UPDATE_GOLDEN=1 go test -tags integration ./tests/integration/ -run 'Visual|Scenario' -count=1

# Stage 6 — Pre-push gate. The single command every PR must pass before push.
# See docs/development-process.md.
ready-to-push: test-race lint security gofix verify-readonly check-readme snapshot mdlint
	@echo "PASS: ready-to-push gate green"

# Stage 7 — Pre-release gate. Run before tagging a release. Subsumes ready-to-push
# plus the full demo-mode integration suite. See docs/development-process.md.
ready-to-release: ready-to-push integration
	@echo "Manual checklist (not automatable, must be confirmed by release owner):"
	@echo "  [ ] CHANGELOG.md updated for this version"
	@echo "  [ ] releases/vX.Y.Z.md written"
	@echo "  [ ] docs/architecture.md aligned with current codebase"
	@echo "  [ ] Busywork audit on tests added/modified in this release complete"
	@echo "PASS: ready-to-release automated gates green"
