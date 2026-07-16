.PHONY: build install test test-budget test-race lint gofix fmt run clean cover integration e2e e2e-install security coverage verify-readonly verify-zero-init demo readme check-readme mdlint snapshot snapshot-update smoke smoke-live smoke-related smoke-related-live smoke-costs check-no-real-data install-hooks ready-to-push ready-to-release generate

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
	go test ./... -count=1 -timeout 300s

test-race:
	go test ./... -count=1 -timeout 300s -race -shuffle=on

# AS-104: capture wall time of `make test` and write test-budget.json. The
# CI `test-budget` job (.github/workflows/ci.yml) runs this, then invokes
# `scripts/test-budget-gate.sh gate` to fail the build at the 5-minute mark.
test-budget:
	@scripts/test-budget-gate.sh capture

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
	go test -tags integration ./tests/integration/... -v -count=1 -timeout 300s

# e2e runs the Playwright browser tests: a real Chromium drives keystrokes
# through the web UI (app.js -> POST /action -> #main swap) and asserts
# navigation + rendering. Catches the dead-keys / 404-asset / stale-chrome /
# DOM-duplication class of bug that Go/curl tests cannot, because they never
# execute the page JavaScript. global-setup builds the binary and boots
# `a9s --demo --web`. Run `make e2e-install` once first (Node 18+ required).
# Kept out of ready-to-push: it needs Node + a browser binary, so it is a
# separate lane (also run in CI via .github/workflows/e2e.yml).
e2e:
	cd tests/e2e && npx playwright test

# e2e-live runs the Playwright suite against a real read-only AWS profile.
# Usage: make e2e-live PROFILE=my-readonly-profile [REGION=us-east-1]
# Requires valid AWS credentials for the named profile and Node 18+ with the
# Chromium binary installed (run make e2e-install once first).
# The live-readonly.spec.ts assertions are data-agnostic: no names or counts
# are hardcoded, so they work against any AWS account.
e2e-live:
	@test -n "$(PROFILE)" || { echo "PROFILE is required: make e2e-live PROFILE=my-readonly-profile [REGION=us-east-1]" >&2; exit 1; }
	cd tests/e2e && A9S_E2E_PROFILE="$(PROFILE)" A9S_E2E_REGION="$(REGION)" npx playwright test live-readonly.spec.ts

e2e-install:
	cd tests/e2e && npm ci && npx playwright install chromium

security:
	govulncheck ./...

# TODO(AS-36): drop the docs/historical/refactor/** exclusion once phase-03 lands
# and refactor-doc churn stops. Excluding here keeps unrelated doc PRs from being
# gated by in-flight refactor-doc lint regressions (see AS-85).
mdlint:
	markdownlint-cli2 "docs/**/*.md" "!docs/historical/refactor/**" "CLAUDE.md" "CONTRIBUTING.md" "CHANGELOG.md"

coverage:
	go test ./core/... ./internal/... ./tests/... -coverpkg=./core/...,./internal/... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

cover: coverage

# False-positive exclusions (line 56+):
#   CreateDate/CreateTime/StartRecord/StartTime/StopTime/StopDate — timestamp field names, not API calls
#   ExecuteCommandConfiguration — ECS cluster struct field read via NavigableField, not an API call
#   CreateServiceClients — local helper in core/aws/client.go that constructs SDK client structs, not an API call
#   ExecuteTaskAt — local runtime executor helper, not an API call
verify-readonly:
	@echo "Checking for write API calls in core/aws/ and core/runtime/..."
	@if grep -rn '\.\(Create\|Delete\|Update\|Put\|Modify\|Terminate\|Stop\|Reboot\|RunInstances\|Execute\|Send\|Publish\|Remove\)[A-Z][A-Za-z0-9]*(' core/aws/*.go core/runtime/*.go \
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
		| grep -v 'ExecuteCommandConfiguration' \
		| grep -v 'CreateServiceClients' \
		| grep -v 'ExecuteTaskAt' ; then \
		echo "FAIL: Write API calls detected!"; exit 1; \
	else \
		echo "PASS: All API calls are read-only"; \
	fi

# AS-820: lock in AS-795 invariant. init() bodies must not return to
# core/aws/ or core/catalog/ after the AS-795b..p migration.
# core/resource/projection_init.go is excluded — AS-731 removes that
# package wholesale.
verify-zero-init:
	@echo "Checking for init() bodies in core/aws/ and core/catalog/..."
	@if grep -rln '^func init()' core/aws/ core/catalog/ 2>/dev/null; then \
		echo "FAIL: init() bodies found in core/aws/ or core/catalog/ — AS-795 invariant is migrated catalog literals, not package init()"; \
		exit 1; \
	else \
		echo "PASS: no init() bodies in core/aws/ or core/catalog/"; \
	fi

demo:
	vhs docs/demos/demo.tape

readme:
	@go run ./cmd/readmegen/ > README.md
	@echo "README.md regenerated from docs/shared/ snippets"

check-readme:
	@tmpfile="$$(mktemp)"; \
	go run ./cmd/readmegen/ > "$$tmpfile"; \
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

# smoke drives the real binary in a headless tmux session over the demo
# fixtures and asserts the rendered surfaces end to end (menu counts,
# humanized statuses, per-row issue causes, the reference bucket's related
# panel). ~30s. Requires tmux.
smoke:
	./scripts/smoke-demo.sh

# smoke-live drives the same walk against a real *readonly* AWS profile with
# data-independent assertions (sweep reaches verified, no raw enum cells, no
# fetch errors on drills). Companion to the Stage 6 live sub-rule; not part
# of ready-to-push (needs credentials).
# Usage: make smoke-live PROFILE=<readonly-profile> REGION=<region>
smoke-live:
	PROFILE="$(PROFILE)" REGION="$(REGION)" ./scripts/smoke-readonly.sh

# smoke-related drives the RELATED panel end to end over the demo fixtures:
# exact fixture witness badges, the zero-count-row cursor skip, count-1
# drills landing on the target detail, a circular drill re-showing cached
# counts with the depth badge intact, and the ec2 IAM Role pivot. ~30s.
# Requires tmux. Part of ready-to-push alongside `make smoke`.
smoke-related:
	./scripts/smoke-related-demo.sh

# smoke-costs drives the Cost Explorer end to end over the demo fixtures:
# open-at-current-month grid, month→week→day zoom with data, zoom-out past
# year without a CE validation error, metric cycle, account pivot, the
# planted growth-story drill to usage types, the 14-day resource boundary
# message, synthetic resource rows, help section, and `-c costs` startup.
# Fixtures anchor to the current month, so assertions stay evergreen. ~60s.
# Requires tmux. Part of ready-to-push alongside `make smoke`.
smoke-costs:
	./scripts/smoke-costs-demo.sh

# smoke-related-live drives the same RELATED-panel walk against a real
# *readonly* AWS profile with data-independent assertions (a settled counted
# badge, a count-1/N drill landing on a detail or list frame, a surviving
# "(?)" row staying actionable with no dead end). Companion to the Stage 6
# live sub-rule; not part of ready-to-push (needs credentials).
# Usage: make smoke-related-live PROFILE=<readonly-profile> REGION=<region>
smoke-related-live:
	PROFILE="$(PROFILE)" REGION="$(REGION)" ./scripts/smoke-related-readonly.sh

# check-no-real-data blocks real AWS/environment identifiers (account IDs in
# ARNs, previously-leaked profile/secret names) from entering tracked files.
# Part of ready-to-push and CI; also runs as a pre-commit hook (make install-hooks).
check-no-real-data:
	./scripts/check-no-real-data.sh

# install-hooks points core.hooksPath at .githooks so the real-data scan runs
# on every commit. Run once per clone.
install-hooks:
	git config core.hooksPath .githooks
	@grep -qxF '.githooks/sensitive_patterns.txt' .git/info/exclude 2>/dev/null || printf '%s\n' '.githooks/sensitive_patterns.txt' >> .git/info/exclude
	@echo "installed .githooks (hooks active; sensitive_patterns.txt ignored via .git/info/exclude, not tracked .gitignore)"

# Stage 6 — Pre-push gate. The single command every PR must pass before push.
# See docs/development-process.md.
ready-to-push: check-no-real-data test-race lint security gofix verify-readonly verify-zero-init check-readme snapshot mdlint smoke smoke-related smoke-costs
	@echo "PASS: ready-to-push gate green"

# Stage 7 — Pre-release gate. ADDITIVE on top of Stage 6: it does NOT re-run
# ready-to-push — a green `make ready-to-push` on this same tree is the
# prerequisite (Stage 6 precedes Stage 7 by process; re-running test-race/
# lint/security here doubled every release for no signal). This target adds
# only what Stage 6 lacks: the full demo-mode integration suite AND the live
# read-only smokes (smoke-live + smoke-related-live) — a real-AWS pass is a
# mandatory pre-tag gate, not a checklist line. The live smokes require an
# explicit PROFILE and REGION (no default) and refuse any profile whose name
# is not *readonly*. Requires read-only AWS credentials and tmux.
# See docs/development-process.md.
# Usage: make ready-to-release PROFILE=<readonly-profile> REGION=<region>
ready-to-release: integration smoke-live smoke-related-live
	@echo "Prerequisite (NOT re-run here): green 'make ready-to-push' on this same tree — Stage 6."
	@echo "Manual checklist (not automatable, must be confirmed by release owner):"
	@echo "  [ ] CHANGELOG.md updated for this version"
	@echo "  [ ] releases/vX.Y.Z.md written"
	@echo "  [ ] docs/architecture.md aligned with current codebase"
	@echo "  [ ] Busywork audit on tests added/modified in this release complete"
	@echo "PASS: ready-to-release automated gates green (incl. live read-only smokes)"
