package unit

// hubs_scripts_test.go pins the two shell hubs that keep unrelated tasks out
// of each other's files: the changelog fragment assembler and the branch
// file-overlap check.
//
// Both scripts resolve their repo root from their own location, so every case
// copies the script into a throw-away root and drives it there — the tree's
// own CHANGELOG.md and branches are never touched.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// hubsScriptRoot builds a throw-away repo root holding a copy of the named
// script under scripts/, and returns the root.
func hubsScriptRoot(t *testing.T, script string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(projectRoot(t), "scripts", script))
	if err != nil {
		t.Fatalf("reading scripts/%s: %v", script, err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", script), src, 0o755); err != nil { //nolint:gosec // an executable copy is the point
		t.Fatal(err)
	}
	return root
}

// runHubsScript runs root/scripts/<script> and returns its combined output and
// exit code.
func runHubsScript(t *testing.T, root, script string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), filepath.Join(root, "scripts", script), args...) //nolint:gosec // path is built from t.TempDir
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("running %s %v: %v\n%s", script, args, err, out)
	}
	return string(out), exitErr.ExitCode()
}

const hubsBaseChangelog = `# Changelog

## [Unreleased]

### Added

- A line that was already here.

## [3.47.0] - 2026-07-07

### Fixed

- An older release line.
`

// writeChangelogFixture lays a CHANGELOG.md and a changelog.d/ (always
// carrying the README the assembler must ignore) into root.
func writeChangelogFixture(t *testing.T, root string, fragments map[string]string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(hubsBaseChangelog), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "changelog.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Changelog fragments\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, body := range fragments {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// unreleasedSections parses root/CHANGELOG.md and returns the "### <Section>"
// bodies of the Unreleased block, keyed by section name.
func unreleasedSections(t *testing.T, root string) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\n\n\n") {
		t.Errorf("assembly left consecutive blank lines, which mdlint refuses:\n%s", raw)
	}
	sections := make(map[string][]string)
	inside := false
	current := ""
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "## [Unreleased]"):
			inside = true
			continue
		case strings.HasPrefix(line, "## ["):
			inside = false
			continue
		}
		if !inside {
			continue
		}
		if strings.HasPrefix(line, "### ") {
			current = strings.TrimSpace(strings.TrimPrefix(line, "### "))
			if _, dup := sections[current]; dup {
				t.Errorf("Unreleased carries the section %q twice — the assembler must create it once", current)
			}
			sections[current] = []string{}
			continue
		}
		if strings.TrimSpace(line) == "" || current == "" {
			continue
		}
		sections[current] = append(sections[current], strings.TrimSpace(line))
	}
	return sections
}

func fragmentNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "changelog.d"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// TestChangelogAssemble_MovesFragmentLinesIntoUnreleased pins the write half
// of `make changelog`: a fragment's lines land under Unreleased's matching
// section, a section the changelog does not have yet is created, and the
// consumed fragment is gone so the next run has nothing to do. Assembly is
// the only writer of CHANGELOG.md, so a task that never edits it cannot
// conflict with another task in it.
func TestChangelogAssemble_MovesFragmentLinesIntoUnreleased(t *testing.T) {
	root := hubsScriptRoot(t, "changelog-assemble.sh")
	writeChangelogFixture(t, root, map[string]string{
		"task-alpha.md": "## Added\n\n- Alpha added a thing.\n\n## Fixed\n\n- Alpha fixed a thing.\n",
	})

	out, exit := runHubsScript(t, root, "changelog-assemble.sh")
	if exit != 0 {
		t.Fatalf("assemble exit = %d, want 0\n%s", exit, out)
	}

	sections := unreleasedSections(t, root)
	added := strings.Join(sections["Added"], "\n")
	if !strings.Contains(added, "- Alpha added a thing.") {
		t.Errorf("Unreleased Added does not carry the fragment line; got:\n%s", added)
	}
	if !strings.Contains(added, "- A line that was already here.") {
		t.Errorf("assembly dropped a line that was already in Unreleased Added; got:\n%s", added)
	}
	fixed := strings.Join(sections["Fixed"], "\n")
	if !strings.Contains(fixed, "- Alpha fixed a thing.") {
		t.Errorf("Unreleased has no Fixed section carrying the fragment line; got:\n%s", fixed)
	}

	names := fragmentNames(t, root)
	for _, n := range names {
		if n == "task-alpha.md" {
			t.Errorf("assembly left changelog.d/task-alpha.md behind; changelog.d holds %v", names)
		}
	}
	if len(names) != 1 || names[0] != "README.md" {
		t.Errorf("changelog.d = %v, want only README.md", names)
	}

	raw, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	released := string(raw)[strings.Index(string(raw), "## [3.47.0]"):]
	if strings.Contains(released, "Alpha") {
		t.Errorf("assembly wrote fragment lines below the Unreleased heading:\n%s", released)
	}
}

// TestChangelogAssemble_MergesTwoFragmentsIntoOneSection pins that two tasks
// naming the same section produce one section, in file-name order — the case
// that made CHANGELOG.md a conflict hub in the first place.
func TestChangelogAssemble_MergesTwoFragmentsIntoOneSection(t *testing.T) {
	root := hubsScriptRoot(t, "changelog-assemble.sh")
	writeChangelogFixture(t, root, map[string]string{
		"task-alpha.md": "## Added\n\n- Alpha added a thing.\n",
		"task-beta.md":  "## Added\n\n- Beta added a thing.\n",
	})

	out, exit := runHubsScript(t, root, "changelog-assemble.sh")
	if exit != 0 {
		t.Fatalf("assemble exit = %d, want 0\n%s", exit, out)
	}

	added := strings.Join(unreleasedSections(t, root)["Added"], "\n")
	alpha := strings.Index(added, "- Alpha added a thing.")
	beta := strings.Index(added, "- Beta added a thing.")
	if alpha == -1 || beta == -1 {
		t.Fatalf("Unreleased Added is missing a fragment line; got:\n%s", added)
	}
	if alpha > beta {
		t.Errorf("fragments assembled out of file-name order; got:\n%s", added)
	}
	if got := fragmentNames(t, root); len(got) != 1 {
		t.Errorf("changelog.d = %v, want only README.md", got)
	}
}

// TestChangelogAssemble_IdempotentWithNoFragments pins that running the
// assembler on a tree with nothing to assemble leaves CHANGELOG.md byte for
// byte as it was — the landing checklist runs it unconditionally.
func TestChangelogAssemble_IdempotentWithNoFragments(t *testing.T) {
	root := hubsScriptRoot(t, "changelog-assemble.sh")
	writeChangelogFixture(t, root, nil)

	out, exit := runHubsScript(t, root, "changelog-assemble.sh")
	if exit != 0 {
		t.Fatalf("assemble exit = %d, want 0\n%s", exit, out)
	}
	raw, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != hubsBaseChangelog {
		t.Errorf("assembler rewrote CHANGELOG.md with no fragments present; got:\n%s", raw)
	}
}

// TestChangelogAssemble_NeverDeletesContentItDidNotWrite pins the one thing
// assembly must never do. It deletes each fragment as it consumes it, so a
// line it silently drops instead of writing is gone from the tree with no
// copy anywhere and exit 0 to say it went fine. Both cases below are inputs
// the script accepts today and answers with a deleted fragment and a
// CHANGELOG.md that never received the content.
func TestChangelogAssemble_NeverDeletesContentItDidNotWrite(t *testing.T) {
	cases := []struct {
		name      string
		changelog string
		fragment  string
		line      string
	}{
		{
			// Keep a Changelog names six sections; a fragment heading outside
			// that set, from a typo or a section the project adds later, has
			// no place to land.
			name:      "section the assembler does not know",
			changelog: hubsBaseChangelog,
			fragment:  "## Added\n\n- Alpha added.\n\n## Notes\n\n- Alpha noted something.\n",
			line:      "- Alpha noted something.",
		},
		{
			// Every line is filed under the heading above it, so a fragment
			// written as a bare list has no heading to be filed under.
			name:      "fragment with no section heading",
			changelog: hubsBaseChangelog,
			fragment:  "- Alpha forgot the heading.\n",
			line:      "- Alpha forgot the heading.",
		},
		{
			// The whole fragment lands in the Unreleased block, so a
			// changelog without one has nowhere to put any of it.
			name:      "changelog with no Unreleased heading",
			changelog: "# Changelog\n\n## [3.47.0] - 2026-07-07\n\n### Fixed\n\n- An older release line.\n",
			fragment:  "## Added\n\n- Alpha added.\n",
			line:      "- Alpha added.",
		},
		{
			// A dropped line that reads as part of a line already in the
			// file is still a dropped line. The entry that swallows it says
			// something else, and nobody is looking for the difference.
			name:      "dropped line is a substring of a line already present",
			changelog: "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- Alpha noted something. It also did more.\n",
			fragment:  "## Notes\n\n- Alpha noted something.\n",
			line:      "- Alpha noted something.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := hubsScriptRoot(t, "changelog-assemble.sh")
			writeChangelogFixture(t, root, map[string]string{"task-alpha.md": tc.fragment})
			if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(tc.changelog), 0o600); err != nil {
				t.Fatal(err)
			}

			out, exit := runHubsScript(t, root, "changelog-assemble.sh")

			raw, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
			if err != nil {
				t.Fatal(err)
			}
			// Whole lines only: a bullet that survives as part of a longer
			// sentence someone else wrote has not survived.
			if slices.Contains(strings.Split(string(raw), "\n"), tc.line) {
				return
			}
			for _, n := range fragmentNames(t, root) {
				if n == "task-alpha.md" {
					// Refusing and keeping the fragment is a fine answer:
					// the content is still on disk to fix and re-run.
					if exit == 0 {
						t.Errorf("%q never reached CHANGELOG.md but the assembler reported success\n%s", tc.line, out)
					}
					return
				}
			}
			t.Errorf("%q never reached CHANGELOG.md and the fragment was deleted anyway (exit %d) — "+
				"the content is gone with nothing to recover it from\n%s", tc.line, exit, out)
		})
	}
}

// TestChangelogAssemble_CheckRefusesWhileAFragmentExists pins the
// ready-to-release gate: an unassembled fragment fails the check even when
// its lines happen to be in Unreleased already, because the release notes are
// cut from CHANGELOG.md and a fragment left on disk is a task's entry that
// nobody assembled.
func TestChangelogAssemble_CheckRefusesWhileAFragmentExists(t *testing.T) {
	root := hubsScriptRoot(t, "changelog-assemble.sh")
	writeChangelogFixture(t, root, map[string]string{
		"task-alpha.md": "## Added\n\n- A line that was already here.\n",
	})

	out, exit := runHubsScript(t, root, "changelog-assemble.sh", "--check")
	if exit != 1 {
		t.Fatalf("--check exit = %d, want 1 while changelog.d/task-alpha.md exists\n%s", exit, out)
	}
	if !strings.Contains(out, "task-alpha.md") {
		t.Errorf("--check output does not name the unassembled fragment:\n%s", out)
	}
}

// TestChangelogAssemble_CheckPassesWithNoFragments is the negative half: an
// assembled tree must not fail the release gate.
func TestChangelogAssemble_CheckPassesWithNoFragments(t *testing.T) {
	root := hubsScriptRoot(t, "changelog-assemble.sh")
	writeChangelogFixture(t, root, nil)

	out, exit := runHubsScript(t, root, "changelog-assemble.sh", "--check")
	if exit != 0 {
		t.Fatalf("--check exit = %d, want 0 on a tree with no fragments\n%s", exit, out)
	}
}

// TestMdlintCoversChangelogFragments pins that the fragments are linted like
// the file they are assembled into. Unlinted fragment prose lands in
// CHANGELOG.md, which mdlint does check, so the failure would surface only
// after assembly, on the orchestrator's commit rather than the task's.
func TestMdlintCoversChangelogFragments(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(projectRoot(t), "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "markdownlint-cli2") {
			continue
		}
		if !strings.Contains(line, "changelog.d/**/*.md") {
			t.Errorf("mdlint globs do not cover the changelog fragments:\n%s", line)
		}
		return
	}
	t.Error("no markdownlint-cli2 invocation found in the Makefile")
}

// gitInHubsRepo runs one git command in root and fails the test on error.
func gitInHubsRepo(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=a9s", "GIT_AUTHOR_EMAIL=a9s@example.invalid",
		"GIT_COMMITTER_NAME=a9s", "GIT_COMMITTER_EMAIL=a9s@example.invalid",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// hubsCommitFile writes one file in root and commits it on the current branch.
func hubsCommitFile(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	gitInHubsRepo(t, root, "add", name)
	gitInHubsRepo(t, root, "commit", "-m", "touch "+name)
}

// newOverlapRepo builds a repo with main, and three task branches:
// task/a and task/b both touch shared.md, task/c touches nothing they do.
func newOverlapRepo(t *testing.T) string {
	t.Helper()
	root := hubsScriptRoot(t, "task-file-overlap.sh")
	gitInHubsRepo(t, root, "init", "-b", "main")
	hubsCommitFile(t, root, "base.md", "# base\n")

	gitInHubsRepo(t, root, "checkout", "-b", "task/a")
	hubsCommitFile(t, root, "shared.md", "# a\n")

	gitInHubsRepo(t, root, "checkout", "main")
	gitInHubsRepo(t, root, "checkout", "-b", "task/b")
	hubsCommitFile(t, root, "shared.md", "# b\n")
	hubsCommitFile(t, root, "only-b.md", "# b\n")

	gitInHubsRepo(t, root, "checkout", "main")
	gitInHubsRepo(t, root, "checkout", "-b", "task/c")
	hubsCommitFile(t, root, "only-c.md", "# c\n")

	gitInHubsRepo(t, root, "checkout", "main")
	return root
}

// TestTaskFileOverlap_NamesTheSharedFile pins the tool behind the "no two
// live tasks touch the same file" rule: two branches that both touch a file
// fail, and the failure names the file and both branches, so the orchestrator
// can serialise them without reading two diffs.
func TestTaskFileOverlap_NamesTheSharedFile(t *testing.T) {
	root := newOverlapRepo(t)

	out, exit := runHubsScript(t, root, "task-file-overlap.sh", "task/a", "task/b")
	if exit != 1 {
		t.Fatalf("overlap exit = %d, want 1 for two branches sharing shared.md\n%s", exit, out)
	}
	if !strings.Contains(out, "shared.md") {
		t.Errorf("output does not name the shared file:\n%s", out)
	}
	for _, branch := range []string{"task/a", "task/b"} {
		if !strings.Contains(out, branch) {
			t.Errorf("output does not name %s as an owner of shared.md:\n%s", branch, out)
		}
	}
	if strings.Contains(out, "only-b.md") {
		t.Errorf("output names only-b.md, which only one branch touches:\n%s", out)
	}
}

// TestTaskFileOverlap_PassesOnDisjointBranches is the negative half: disjoint
// branches must pass, or the rule would serialise every pair of tasks.
func TestTaskFileOverlap_PassesOnDisjointBranches(t *testing.T) {
	root := newOverlapRepo(t)

	out, exit := runHubsScript(t, root, "task-file-overlap.sh", "task/a", "task/c")
	if exit != 0 {
		t.Fatalf("overlap exit = %d, want 0 for disjoint branches\n%s", exit, out)
	}
}
