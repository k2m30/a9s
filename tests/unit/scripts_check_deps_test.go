package unit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// scripts/check-deps-current.sh is a pre-push gate: it must distinguish
// "everything current" (0), "something is outdated" (1) and "the check could
// not run" (2). Conflating 2 with 0 would let a push through unchecked, which
// is the whole failure mode the gate exists to prevent.
//
// The script reads its inputs from the working directory it is invoked in
// (go.mod and .github/workflows/*.yml) and shells out through ${GO}, ${GH} and
// ${CURL}, so every case below runs it against a throw-away root with stub
// executables and asserts only the exit code and the reported lines.

const (
	depsExitCurrent  = 0
	depsExitOutdated = 1
	depsExitSkipped  = 2
)

func depsScriptPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	script := filepath.Join(repoRoot, "scripts", "check-deps-current.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("check-deps-current.sh not found at %s: %v", script, err)
	}
	return script
}

type depsFixture struct {
	root     string
	binDir   string
	ghLogTxt string
}

// newDepsFixture builds a throw-away repo root: a go.mod carrying the go
// directive, a .github/workflows tree, and a bin dir for the stubs.
func newDepsFixture(t *testing.T, goDirective string) *depsFixture {
	t.Helper()
	root := t.TempDir()
	binDir := filepath.Join(root, "stubbin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDepsFile(t, filepath.Join(root, "go.mod"),
		"module example.com/fixture\n\ngo "+goDirective+"\n")
	return &depsFixture{root: root, binDir: binDir, ghLogTxt: filepath.Join(root, "gh-calls.log")}
}

func writeDepsFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *depsFixture) workflow(t *testing.T, name, body string) {
	t.Helper()
	writeDepsFile(t, filepath.Join(f.root, ".github", "workflows", name), body)
}

func (f *depsFixture) writeStub(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(f.binDir, name)
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// goStub answers the two invocations the module and toolchain sections make:
// the direct-module listing and `list -m -versions go`. It refuses -json so a
// change of listing form fails loudly instead of returning nonsense.
func (f *depsFixture) goStub(t *testing.T, moduleReport, goVersionsLine string) string {
	t.Helper()
	reportPath := filepath.Join(f.root, "go-modules.txt")
	writeDepsFile(t, reportPath, moduleReport)
	return f.writeStub(t, "go-stub", `
for a in "$@"; do
  case "$a" in
    -versions) echo `+shellQuote(goVersionsLine)+`; exit 0 ;;
    -json) echo "stub: -json listing is not stubbed; see depsFixture.goStub" >&2; exit 97 ;;
  esac
done
cat `+shellQuote(reportPath)+`
`)
}

func (f *depsFixture) goStubFailing(t *testing.T) string {
	t.Helper()
	return f.writeStub(t, "go-stub", "echo 'go: module lookup disabled by GOFLAGS=-mod=vendor' >&2\nexit 1\n")
}

// ghStub logs every invocation so dedup and skip-list behaviour can be
// asserted, then answers `api repos/<owner>/<repo>/releases/latest` from a
// case table. Repos absent from the table exit 1, mimicking "no releases".
func (f *depsFixture) ghStub(t *testing.T, latest map[string]string) string {
	t.Helper()
	var cases strings.Builder
	for repo, tag := range latest {
		cases.WriteString("  " + shellQuote(repo) + ") echo " + shellQuote(tag) + "; exit 0 ;;\n")
	}
	return f.writeStub(t, "gh-stub", `
printf '%s\n' "$*" >> `+shellQuote(f.ghLogTxt)+`
repo=""
for a in "$@"; do
  case "$a" in
    repos/*) repo="${a#repos/}"; repo="${repo%%/releases*}"; repo="${repo%%/tags*}" ;;
  esac
done
case "$repo" in
`+cases.String()+`  *) echo "gh: no release found for $repo" >&2; exit 1 ;;
esac
`)
}

func (f *depsFixture) ghStubFailing(t *testing.T) string {
	t.Helper()
	return f.writeStub(t, "gh-stub", `
printf '%s\n' "$*" >> `+shellQuote(f.ghLogTxt)+`
echo 'gh: dial tcp: lookup api.github.com: no such host' >&2
exit 1
`)
}

func (f *depsFixture) ghCalls(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(f.ghLogTxt)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type depsResult struct {
	out  string
	code int
}

func (f *depsFixture) run(t *testing.T, extraEnv ...string) depsResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", depsScriptPath(t)) //nolint:gosec // fixed script path from the repo tree
	cmd.Dir = f.root
	env := append(os.Environ(),
		"GO="+filepath.Join(f.binDir, "go-stub"),
		"GH="+filepath.Join(f.binDir, "gh-stub"),
		"CURL="+filepath.Join(f.binDir, "curl-stub"),
	)
	cmd.Env = append(env, extraEnv...)
	raw, err := cmd.CombinedOutput()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running check-deps-current.sh: %v\n%s", err, raw)
	}
	return depsResult{out: string(raw), code: code}
}

func (r depsResult) requireCode(t *testing.T, want int) {
	t.Helper()
	if r.code != want {
		t.Fatalf("exit code = %d, want %d\n--- output ---\n%s", r.code, want, r.out)
	}
}

// requireLine asserts some single output line carries all of the tokens, so
// the report's contract is pinned without freezing its exact phrasing.
func (r depsResult) requireLine(t *testing.T, tokens ...string) {
	t.Helper()
	if r.findLine(tokens) == "" {
		t.Errorf("no output line contains all of %v\n--- output ---\n%s", tokens, r.out)
	}
}

func (r depsResult) requireNoLine(t *testing.T, tokens ...string) {
	t.Helper()
	if got := r.findLine(tokens); got != "" {
		t.Errorf("unexpected output line %q containing all of %v\n--- output ---\n%s", got, tokens, r.out)
	}
}

func (r depsResult) findLine(tokens []string) string {
	for _, line := range strings.Split(r.out, "\n") {
		all := true
		for _, tok := range tokens {
			if !strings.Contains(line, tok) {
				all = false
				break
			}
		}
		if all {
			return line
		}
	}
	return ""
}

const depsWorkflowCurrent = `name: ci
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0  # v7.0.0
`

// depsModulesCurrent is what the direct-module listing prints when nothing is
// outdated: go emits no line at all for a module the template skips, so the
// report is empty rather than blank-padded.
const depsModulesCurrent = ""

// depsGoVersionsCurrent mirrors `go list -m -versions go`: the module name,
// then an unsorted version list including release candidates.
const depsGoVersionsCurrent = "go 1.9.2rc2 1.2.2 1.25.0 1.25.1 1.26rc1 1.26rc2 1.26.0 1.26.4 1.26.5"

// (a) Nothing outdated: the gate stays silent and lets the push through.
func TestCheckDepsCurrent_AllCurrentExitsZero(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "ci.yml", depsWorkflowCurrent)
	f.goStub(t, depsModulesCurrent, depsGoVersionsCurrent)
	f.ghStub(t, map[string]string{"actions/checkout": "v7.0.0"})

	res := f.run(t)
	res.requireCode(t, depsExitCurrent)
	res.requireNoLine(t, "->")
}

// (b) One outdated direct module, a newer toolchain patch, and one outdated
// action pin: all three sections report and the gate fails the push. Only the
// action that actually moved is named; a pin already at the latest release
// must stay out of the report or the gate cries wolf every push.
func TestCheckDepsCurrent_ReportsAllThreeSectionsAndExitsOne(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "ci.yml", `name: ci
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0  # v7.0.0
      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c  # v6.4.0
`)
	f.goStub(t, "module golang.org/x/tools v0.49.0 -> v0.50.0\n",
		"go 1.9.2rc2 1.26rc1 1.26.0 1.26.4 1.26.5 1.26.6")
	f.ghStub(t, map[string]string{
		"actions/checkout": "v7.0.0",
		"actions/setup-go": "v7.0.0",
	})

	res := f.run(t)
	res.requireCode(t, depsExitOutdated)
	res.requireLine(t, "module", "golang.org/x/tools", "v0.49.0", "->", "v0.50.0")
	res.requireLine(t, "toolchain", "1.26.5", "->", "1.26.6")
	res.requireLine(t, "action", "actions/setup-go", "v6.4.0", "->", "v7.0.0")

	res.requireNoLine(t, "actions/checkout", "->")
}

// A release candidate is never a newer patch: 1.26rc3 must not be reported as
// an upgrade from 1.26.5, and neither must a newer minor.
func TestCheckDepsCurrent_ToolchainIgnoresPrereleasesAndOtherMinors(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "ci.yml", depsWorkflowCurrent)
	f.goStub(t, depsModulesCurrent,
		"go 1.9.2rc2 1.26.0 1.26.5 1.26.6rc1 1.26.6beta1 1.27rc1 1.27.0 1.27.1")
	f.ghStub(t, map[string]string{"actions/checkout": "v7.0.0"})

	res := f.run(t)
	res.requireCode(t, depsExitCurrent)
	res.requireNoLine(t, "toolchain", "->")
}

// (c) The check could not run. A gate that cannot answer must not answer
// "fine": the section is reported as skipped and the exit code is 2, distinct
// from both 0 and 1, so the Makefile fails the push. DEPS_CHECK_OFFLINE_OK=1
// is the explicit opt-out for working offline.
func TestCheckDepsCurrent_UnreachableSectionExitsTwo(t *testing.T) {
	t.Run("gh unreachable", func(t *testing.T) {
		f := newDepsFixture(t, "1.26.5")
		f.workflow(t, "ci.yml", depsWorkflowCurrent)
		f.goStub(t, depsModulesCurrent, depsGoVersionsCurrent)
		f.ghStubFailing(t)

		res := f.run(t)
		res.requireCode(t, depsExitSkipped)
		res.requireLine(t, "SKIP")

		ok := f.run(t, "DEPS_CHECK_OFFLINE_OK=1")
		ok.requireCode(t, depsExitCurrent)
	})

	// Same contract on the other lane: a failing `go list` must not be
	// silently treated as "no modules are outdated".
	t.Run("go unreachable", func(t *testing.T) {
		f := newDepsFixture(t, "1.26.5")
		f.workflow(t, "ci.yml", depsWorkflowCurrent)
		f.goStubFailing(t)
		f.ghStub(t, map[string]string{"actions/checkout": "v7.0.0"})

		res := f.run(t)
		res.requireCode(t, depsExitSkipped)
		res.requireLine(t, "SKIP")

		ok := f.run(t, "DEPS_CHECK_OFFLINE_OK=1")
		ok.requireCode(t, depsExitCurrent)
	})

	// An outdated item found by a section that did run still fails the push:
	// a skip elsewhere must not mask a real finding.
	t.Run("outdated module plus unreachable gh", func(t *testing.T) {
		f := newDepsFixture(t, "1.26.5")
		f.workflow(t, "ci.yml", depsWorkflowCurrent)
		f.goStub(t, "module golang.org/x/tools v0.49.0 -> v0.50.0\n", depsGoVersionsCurrent)
		f.ghStubFailing(t)

		res := f.run(t)
		res.requireLine(t, "module", "golang.org/x/tools", "v0.49.0", "->", "v0.50.0")
		if res.code == depsExitCurrent {
			t.Fatalf("exit code = 0 with an outdated module reported\n--- output ---\n%s", res.out)
		}

		ok := f.run(t, "DEPS_CHECK_OFFLINE_OK=1")
		ok.requireCode(t, depsExitOutdated)
		ok.requireLine(t, "module", "golang.org/x/tools", "v0.49.0", "->", "v0.50.0")
	})
}

// (d) Every action in this repo is SHA-pinned, so the trailing `# vX.Y.Z`
// comment is the only readable current version. It must drive the comparison
// in both directions, and a SHA with no comment must be surfaced rather than
// silently passing.
func TestCheckDepsCurrent_ShaPinnedActionsComparedThroughComment(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "ci.yml", `name: ci
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0  # v7.0.0
      - uses: github/codeql-action/init@7188fc363630916deb702c7fdcf4e481b751f97a  # v4.37.1
      - uses: acme/no-comment@1111111111111111111111111111111111111111
`)
	f.goStub(t, depsModulesCurrent, depsGoVersionsCurrent)
	f.ghStub(t, map[string]string{
		"actions/checkout":     "v7.0.0",
		"github/codeql-action": "v4.37.3",
		"acme/no-comment":      "v2.0.0",
	})

	res := f.run(t)
	res.requireCode(t, depsExitOutdated)
	res.requireLine(t, "action", "github/codeql-action", "v4.37.1", "->", "v4.37.3")
	res.requireNoLine(t, "actions/checkout", "->")
	res.requireLine(t, "acme/no-comment", "unpinned-by-tag")
}

// A sub-path action resolves against its repository, not the sub-path, and the
// repository is queried once no matter how many steps or workflows use it.
func TestCheckDepsCurrent_ActionsDedupedPerRepository(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "ci.yml", `name: ci
on: [push]
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0  # v7.0.0
      - uses: github/codeql-action/init@7188fc363630916deb702c7fdcf4e481b751f97a  # v4.37.1
      - uses: github/codeql-action/analyze@7188fc363630916deb702c7fdcf4e481b751f97a  # v4.37.1
`)
	f.workflow(t, "release.yml", `name: release
on: [push]
jobs:
  b:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0  # v7.0.0
`)
	f.goStub(t, depsModulesCurrent, depsGoVersionsCurrent)
	f.ghStub(t, map[string]string{
		"actions/checkout":     "v7.0.0",
		"github/codeql-action": "v4.37.1",
	})

	res := f.run(t)
	res.requireCode(t, depsExitCurrent)

	seen := map[string]int{}
	for _, call := range f.ghCalls(t) {
		for _, repo := range []string{"actions/checkout", "github/codeql-action"} {
			if strings.Contains(call, "repos/"+repo+"/") {
				seen[repo]++
			}
		}
	}
	for _, repo := range []string{"actions/checkout", "github/codeql-action"} {
		if seen[repo] != 1 {
			t.Errorf("gh queried %s %d times, want exactly 1\ncalls: %v", repo, seen[repo], f.ghCalls(t))
		}
	}
}

// (e) Local and container actions have no GitHub release to compare against;
// querying them would burn API calls and report a phantom upgrade.
func TestCheckDepsCurrent_IgnoresLocalAndDockerUses(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "ci.yml", `name: ci
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: ./.github/actions/setup
      - uses: ./local-action
      - uses: docker://alpine:3.19
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0  # v7.0.0
`)
	f.goStub(t, depsModulesCurrent, depsGoVersionsCurrent)
	f.ghStub(t, map[string]string{"actions/checkout": "v7.0.0"})

	res := f.run(t)
	res.requireCode(t, depsExitCurrent)

	for _, call := range f.ghCalls(t) {
		for _, forbidden := range []string{"local-action", "docker", "alpine", ".github/actions"} {
			if strings.Contains(call, forbidden) {
				t.Errorf("gh was queried for a non-registry use: %q", call)
			}
		}
	}
}

// A gate nothing invokes protects nothing: check-deps must sit in the
// ready-to-push prerequisite list, immediately after security. Ordering
// matters because the two are the same class of check (the tree is fine, the
// world moved on) and the docs enumeration is order-sensitive.
func TestCheckDepsCurrent_WiredIntoReadyToPush(t *testing.T) {
	targets := parseMakefileReadyToPush(t, "../../Makefile")

	idx := -1
	for i, target := range targets {
		if target == "check-deps" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("ready-to-push prerequisites %v do not include check-deps", targets)
	}
	if idx == 0 || targets[idx-1] != "security" {
		t.Errorf("check-deps sits at position %d in %v, want it directly after security", idx, targets)
	}
}

// A SHA pin with no `# vX.Y.Z` comment is reported but cannot be compared. The
// gate's own doctrine is that an unverifiable item must not pass: printing a
// line and then exiting 0 tells the human "look at this" and the Makefile
// "nothing to see", which is the one combination that lets a push through.
func TestCheckDepsCurrent_UnpinnedActionFailsTheGate(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "ci.yml", `name: ci
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0  # v7.0.0
      - uses: acme/no-comment@1111111111111111111111111111111111111111
`)
	f.goStub(t, depsModulesCurrent, depsGoVersionsCurrent)
	f.ghStub(t, map[string]string{
		"actions/checkout": "v7.0.0",
		"acme/no-comment":  "v2.0.0",
	})

	res := f.run(t)
	res.requireLine(t, "acme/no-comment", "unpinned-by-tag")
	if res.code == depsExitCurrent {
		t.Fatalf("exit code = 0 while reporting an unverifiable action pin\n--- output ---\n%s", res.out)
	}
}

// A branch or moving ref (`@main`) is not a semantic version, so no tag lookup
// can resolve it. That is a property of the pin, not of the network: the gate
// must not answer "the actions section could not run" when every API call it
// made succeeded, or a routine workflow edit becomes an unexplainable exit 2.
func TestCheckDepsCurrent_BranchRefIsNotReportedAsUnreachable(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "ci.yml", `name: ci
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0  # v7.0.0
      - uses: acme/branchy@main
`)
	f.goStub(t, depsModulesCurrent, depsGoVersionsCurrent)
	f.ghStub(t, map[string]string{
		"actions/checkout": "v7.0.0",
		"acme/branchy":     "v1.4.0",
	})

	res := f.run(t)
	res.requireNoLine(t, "SKIP")
	if res.code == depsExitSkipped {
		t.Errorf("exit code = 2 (section unverifiable) though every gh call succeeded\n--- output ---\n%s", res.out)
	}
}

// github/codeql-action's newest *release* is a codeql-bundle-*, not a version
// tag, so releases/latest cannot drive the comparison. Falling back to the tag
// list is what keeps a real repo in this workflow set from being misreported.
func TestCheckDepsCurrent_FallsBackToTagsWhenLatestReleaseIsNotAVersion(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "ci.yml", `name: ci
on: [push]
jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: github/codeql-action/init@7188fc363630916deb702c7fdcf4e481b751f97a  # v4.37.1
`)
	f.goStub(t, depsModulesCurrent, depsGoVersionsCurrent)
	f.writeStub(t, "gh-stub", `
printf '%s\n' "$*" >> `+shellQuote(f.ghLogTxt)+`
case "$*" in
  *releases/latest*) echo "codeql-bundle-v2.20.3"; exit 0 ;;
  *"/tags"*) printf '%s\n' v4.37.3 v4.37.10 v4.37.1 codeql-bundle-v2.20.3 v3.29.0; exit 0 ;;
esac
echo "gh: unexpected call: $*" >&2
exit 1
`)

	res := f.run(t)
	res.requireCode(t, depsExitOutdated)
	res.requireLine(t, "action", "github/codeql-action", "v4.37.1", "->", "v4.37.10")
	res.requireNoLine(t, "codeql-bundle")
	res.requireNoLine(t, "SKIP")
}

// The same action pinned at two versions across workflows: deduplication must
// be per (action, pinned version), not per action, or the stale pin hides
// behind whichever workflow the scan reached first. The API is still queried
// once, because the answer does not depend on the pin.
func TestCheckDepsCurrent_SameActionAtTwoVersionsBothEvaluated(t *testing.T) {
	f := newDepsFixture(t, "1.26.5")
	f.workflow(t, "a-current.yml", `name: a
on: [push]
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@0000000000000000000000000000000000000000  # v7.0.0
`)
	f.workflow(t, "b-stale.yml", `name: b
on: [push]
jobs:
  b:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c  # v6.4.0
`)
	f.goStub(t, depsModulesCurrent, depsGoVersionsCurrent)
	f.ghStub(t, map[string]string{"actions/setup-go": "v7.0.0"})

	res := f.run(t)
	res.requireCode(t, depsExitOutdated)
	res.requireLine(t, "action", "actions/setup-go", "v6.4.0", "->", "v7.0.0", "b-stale.yml")

	calls := 0
	for _, call := range f.ghCalls(t) {
		if strings.Contains(call, "repos/actions/setup-go/") {
			calls++
		}
	}
	if calls != 1 {
		t.Errorf("gh queried actions/setup-go %d times, want exactly 1\ncalls: %v", calls, f.ghCalls(t))
	}
}
