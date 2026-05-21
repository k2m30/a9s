// catalogen is a go:generate–driven binary that reads the installed catalog
// and emits markdown documentation. It does NOT generate any Go code.
//
// Usage (via go generate):
//
//	go run ../../cmd/catalogen
//
// Or directly:
//
//	go run ./cmd/catalogen
//
// Output files (relative to the repo root):
//   - docs/attention-signals.md  — findings × severity table
//   - docs/related-resources.md  — generated from Related defs
//   - docs/resources/<short>.md  — per-resource markdown (section-marker mode)
//
// When the catalog is empty (PR-04a), no output files are written.
// Per-category PRs (04b–04m) populate the catalog and the generator begins
// producing content.
//
// Verify flag: run with -verify to assert that every catalog entry has a
// corresponding docs/resources/<short>.md. Exits non-zero on violations.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
)

func main() {
	// Install the catalog before any catalog.All / catalog.Find call.
	aws.Install()

	verify := flag.Bool("verify", false, "verify every catalog entry has a matching docs/resources/<short>.md")
	flag.Parse()

	if *verify {
		if err := runVerify(); err != nil {
			log.Fatalf("catalogen -verify: %v", err)
		}
		return
	}

	if err := run(); err != nil {
		log.Fatalf("catalogen: %v", err)
	}
}

// run generates all markdown outputs from the installed catalog.
// When the catalog is empty it is a no-op.
func run() error {
	types := catalog.All()
	if len(types) == 0 {
		// Catalog not yet populated — nothing to generate.
		return nil
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("locate repo root: %w", err)
	}

	if err := generateAttentionSignals(repoRoot, types); err != nil {
		return fmt.Errorf("attention-signals.md: %w", err)
	}

	if err := generateRelatedResources(repoRoot, types); err != nil {
		return fmt.Errorf("related-resources.md: %w", err)
	}

	for _, rt := range types {
		if err := generateResourceDoc(repoRoot, rt); err != nil {
			return fmt.Errorf("docs/resources/%s.md: %w", rt.ShortName, err)
		}
	}

	return nil
}

// runVerify checks that every catalog entry has a matching docs/resources/<short>.md.
func runVerify() error {
	types := catalog.All()
	if len(types) == 0 {
		return nil
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("locate repo root: %w", err)
	}

	var missing []string
	for _, rt := range types {
		path := filepath.Join(repoRoot, "docs", "resources", rt.ShortName+".md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, rt.ShortName)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing docs/resources/<short>.md for: %s", strings.Join(missing, ", "))
	}
	return nil
}

// generateAttentionSignals writes docs/attention-signals.md between its
// BEGIN/END GENERATED markers. Content is a Findings × Severity table.
func generateAttentionSignals(repoRoot string, types []catalog.ResourceTypeDef) error {
	path := filepath.Join(repoRoot, "docs", "attention-signals.md")

	var rows strings.Builder
	rows.WriteString("| Type | Code | Phrase | Severity | Source |\n")
	rows.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, rt := range types {
		for _, f := range rt.Findings {
			fmt.Fprintf(&rows, "| %s | %s | %s | %s | %s |\n",
				rt.ShortName, string(f.Code), f.Phrase, severityLabel(f.Severity), f.Source)
		}
	}

	return updateGeneratedSection(path, "findings-table", rows.String())
}

// generateRelatedResources writes docs/related-resources.md between its markers.
func generateRelatedResources(repoRoot string, types []catalog.ResourceTypeDef) error {
	path := filepath.Join(repoRoot, "docs", "related-resources.md")

	var rows strings.Builder
	rows.WriteString("| Source Type | Target Type | Display Name | Needs Target Cache |\n")
	rows.WriteString("| --- | --- | --- | --- |\n")
	for _, rt := range types {
		for _, rel := range rt.Related {
			needsCache := "no"
			if rel.NeedsTargetCache {
				needsCache = "yes"
			}
			fmt.Fprintf(&rows, "| %s | %s | %s | %s |\n",
				rt.ShortName, rel.TargetType, rel.DisplayName, needsCache)
		}
	}

	return updateGeneratedSection(path, "related-table", rows.String())
}

// generateResourceDoc updates the per-resource docs/resources/<short>.md using
// section markers. If the file does not exist, a stub is created.
func generateResourceDoc(repoRoot string, rt catalog.ResourceTypeDef) error {
	path := filepath.Join(repoRoot, "docs", "resources", rt.ShortName+".md")

	// Header section content.
	header := fmt.Sprintf("%s — %s. Lifecycle key: `%s`.\n",
		rt.ShortName, rt.Category, lifecycleKey(rt))

	// Findings section content.
	var findingsContent strings.Builder
	if len(rt.Findings) > 0 {
		findingsContent.WriteString("| Code | Phrase | Severity | Source |\n")
		findingsContent.WriteString("| --- | --- | --- | --- |\n")
		for _, f := range rt.Findings {
			fmt.Fprintf(&findingsContent, "| %s | %s | %s | %s |\n",
				string(f.Code), f.Phrase, severityLabel(f.Severity), f.Source)
		}
	}

	// Related section content.
	var relatedContent strings.Builder
	if len(rt.Related) > 0 {
		relatedContent.WriteString("| Target Type | Display Name | Approximate? |\n")
		relatedContent.WriteString("| --- | --- | --- |\n")
		for _, rel := range rt.Related {
			approx := "no"
			if rel.NeedsTargetCache {
				approx = "yes"
			}
			fmt.Fprintf(&relatedContent, "| %s | %s | %s |\n",
				rel.TargetType, rel.DisplayName, approx)
		}
	}

	// If the file does not exist, create a stub.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		stub := buildStub(rt, header, findingsContent.String(), relatedContent.String())
		return os.WriteFile(path, []byte(stub), 0o600)
	}

	// File exists — update each generated section in place.
	if err := updateGeneratedSection(path, "header", header); err != nil {
		return err
	}
	if err := updateGeneratedSection(path, "findings", findingsContent.String()); err != nil {
		return err
	}
	return updateGeneratedSection(path, "related", relatedContent.String())
}

// buildStub returns the full content of a new per-resource markdown stub.
func buildStub(rt catalog.ResourceTypeDef, header, findings, related string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", rt.Name)
	fmt.Fprintf(&b, "<!-- BEGIN GENERATED: header -->\n%s<!-- END GENERATED: header -->\n\n", header)
	b.WriteString("## Why this matters\n\n(TODO: write narrative)\n\n")
	fmt.Fprintf(&b, "## Findings\n\n<!-- BEGIN GENERATED: findings -->\n%s<!-- END GENERATED: findings -->\n\n", findings)
	b.WriteString("## Workflow\n\n(TODO: write narrative)\n\n")
	fmt.Fprintf(&b, "## Related Resources\n\n<!-- BEGIN GENERATED: related -->\n%s<!-- END GENERATED: related -->\n", related)
	return b.String()
}

// updateGeneratedSection replaces the content between
// <!-- BEGIN GENERATED: <section> --> and <!-- END GENERATED: <section> -->
// in the file at path. If neither marker exists, the section is appended.
// If the file does not exist, it is created with only the generated section.
func updateGeneratedSection(path, section, content string) error {
	begin := fmt.Sprintf("<!-- BEGIN GENERATED: %s -->", section)
	end := fmt.Sprintf("<!-- END GENERATED: %s -->", section)

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create file with just this section.
			body := fmt.Sprintf("%s\n%s%s\n", begin, content, end)
			return os.WriteFile(path, []byte(body), 0o600)
		}
		return err
	}

	existing := string(raw)

	beginIdx := strings.Index(existing, begin)
	endIdx := strings.Index(existing, end)

	if beginIdx == -1 || endIdx == -1 {
		// Markers absent — append the block.
		appended := existing + "\n" + begin + "\n" + content + end + "\n"
		return os.WriteFile(path, []byte(appended), 0o600) //nolint:gosec // path is derived from repoRoot+catalog short names, not user input
	}

	// Replace content between markers (exclusive).
	before := existing[:beginIdx+len(begin)]
	after := existing[endIdx:]
	updated := before + "\n" + content + after
	return os.WriteFile(path, []byte(updated), 0o600) //nolint:gosec // path is derived from repoRoot+catalog short names, not user input
}

// findRepoRoot walks up from the current working directory to find the repo
// root (directory containing go.mod).
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found in any parent directory")
}

// severityLabel returns the human-readable label for a domain.Severity value.
func severityLabel(s domain.Severity) string {
	switch s {
	case domain.SevOK:
		return "ok"
	case domain.SevWarn:
		return "warn"
	case domain.SevBroken:
		return "broken"
	case domain.SevDim:
		return "dim"
	default:
		return fmt.Sprintf("severity(%d)", int(s))
	}
}

// lifecycleKey returns the effective lifecycle key for a resource type.
func lifecycleKey(rt catalog.ResourceTypeDef) string {
	if rt.LifecycleKey == "" {
		return "state"
	}
	return rt.LifecycleKey
}

// readLines is a helper used to parse existing markdown files line by line.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

// ensure readLines is used (it is referenced by future per-category PRs that
// may need line-level parsing for section updates in large existing files).
var _ = readLines
