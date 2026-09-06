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
//   - docs/attention-signals.md  — ONLY the marked signals section; the
//     "Not yet implemented" list and the surface prose are hand-maintained
//   - docs/related-resources.md  — ONLY the marked related-table section;
//     the per-type contract prose (mechanisms, citations, budget-excluded
//     annotations) is hand-maintained and NOT generated
//   - docs/resources/<short>.md  — per-resource markdown (section-marker mode)
//
// When the catalog is empty, no output files are written; the generator
// produces content once the catalog is populated.
//
// Verify flag: run with -verify to assert that every catalog entry has a
// corresponding docs/resources/<short>.md. Exits non-zero on violations.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
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

	if err := generateSignalTables(repoRoot, types); err != nil {
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
				escapeMarkdownCell(rt.ShortName), escapeMarkdownCell(rel.TargetType),
				escapeMarkdownCell(rel.DisplayName), needsCache)
		}
	}

	return updateGeneratedSection(path, "related-table", rows.String())
}

// generateResourceDoc updates the per-resource docs/resources/<short>.md using
// section markers. If the file does not exist, a stub is created.
func generateResourceDoc(repoRoot string, rt catalog.ResourceTypeDef) error {
	path := filepath.Join(repoRoot, "docs", "resources", rt.ShortName+".md")

	// Header section content.
	header := fmt.Sprintf("%s — %s. %s\n",
		rt.ShortName, rt.Category, lifecycleFragment(rt))

	// Findings section content.
	var findingsContent strings.Builder
	if len(rt.Findings) > 0 {
		findingsContent.WriteString("| Code | Phrase | Severity | Source | Detail |\n")
		findingsContent.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, f := range rt.Findings {
			fmt.Fprintf(&findingsContent, "| %s | %s | %s | %s | %s |\n",
				escapeMarkdownCell(string(f.Code)), escapeMarkdownCell(f.Phrase),
				severityLabel(f.Severity), escapeMarkdownCell(f.Source), detailCell(f))
		}
	}

	// Related section content.
	var relatedContent strings.Builder
	if len(rt.Related) > 0 {
		relatedContent.WriteString("| Target Type | Display Name | Truncated? |\n")
		relatedContent.WriteString("| --- | --- | --- |\n")
		for _, rel := range rt.Related {
			truncated := "no"
			if rel.Truncated {
				truncated = "yes"
			}
			fmt.Fprintf(&relatedContent, "| %s | %s | %s |\n",
				escapeMarkdownCell(rel.TargetType), escapeMarkdownCell(rel.DisplayName), truncated)
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
	if err := updateOptionalSection(path, "badge", badgeNote(rt)+"\n"); err != nil {
		return err
	}
	return updateGeneratedSection(path, "related", relatedContent.String())
}

// badgeNote is the §4 S1 note saying which waves feed this type's issue
// badge: only a type that registers a Wave 2 enricher can have Wave 2
// findings counted.
func badgeNote(rt catalog.ResourceTypeDef) string {
	if _, ok := aws.Wave2EnricherFor(rt.ShortName); ok {
		return "Badge aggregation for `" + rt.ShortName + "`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher."
	}
	return "Badge aggregation for `" + rt.ShortName + "`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count."
}

// buildStub returns the full content of a new per-resource markdown stub.
func buildStub(rt catalog.ResourceTypeDef, header, findings, related string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", rt.Name)
	fmt.Fprintf(&b, "<!-- BEGIN GENERATED: header -->\n%s<!-- END GENERATED: header -->\n\n", header)
	b.WriteString("## Why this matters\n\n_Narrative pending._\n\n")
	fmt.Fprintf(&b, "## Findings\n\n<!-- BEGIN GENERATED: findings -->\n%s<!-- END GENERATED: findings -->\n\n", findings)
	b.WriteString("## Workflow\n\n_Narrative pending._\n\n")
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

// updateOptionalSection is updateGeneratedSection for a section a page
// opts into by carrying its markers; a page without them is left alone
// rather than having the block appended.
func updateOptionalSection(path, section, content string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(string(raw), fmt.Sprintf("<!-- BEGIN GENERATED: %s -->", section)) {
		return nil
	}
	return updateGeneratedSection(path, section, content)
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

// escapeMarkdownCell escapes characters that are special inside a markdown
// table cell: backslash (escape prefix), pipe (cell delimiter), and the
// emphasis markers asterisk/underscore/backtick, which markdownlint's MD037
// flags when they appear unbalanced (e.g. literal "*" in a catalog phrase).
func escapeMarkdownCell(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`|`, `\|`,
		`*`, `\*`,
		`_`, `\_`,
		"`", "\\`",
	)
	return replacer.Replace(s)
}

// detailCell is the Detail column of a findings table: the declared S5
// sentence, or an em dash for a finding that renders none.
func detailCell(f catalog.FindingDef) string {
	if f.Detail == "" {
		return "—"
	}
	return escapeMarkdownCell(f.Detail)
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

// lifecycleFragment returns the "Lifecycle key: …" header sentence. When the
// Wave 1 fetcher declares its field keys and the effective lifecycle key is
// not among them, the type has no row-color driver — naming a key would send
// QA hunting for a field that never exists.
func lifecycleFragment(rt catalog.ResourceTypeDef) string {
	key := lifecycleKey(rt)
	if len(rt.FieldKeys) > 0 && !slices.Contains(rt.FieldKeys, key) {
		return "Lifecycle key: none (the list API returns no lifecycle field)."
	}
	return fmt.Sprintf("Lifecycle key: `%s`.", key)
}

// generateSignalTables writes the per-category signal tables of
// docs/attention-signals.md from the registered FindingDefs — the page's whole
// signal inventory, so nothing outside the block states a signal.
func generateSignalTables(repoRoot string, types []catalog.ResourceTypeDef) error {
	path := filepath.Join(repoRoot, "docs", "attention-signals.md")

	// Child types carry FindingDefs of their own and are stored in a map, so
	// they arrive in no particular order.
	children := catalog.AllChildren()
	slices.SortFunc(children, func(a, b catalog.ResourceTypeDef) int {
		return strings.Compare(a.ShortName, b.ShortName)
	})
	types = append(slices.Clone(types), children...)

	var categories []string
	for _, rt := range types {
		if len(rt.Findings) > 0 && !slices.Contains(categories, rt.Category) {
			categories = append(categories, rt.Category)
		}
	}

	var b strings.Builder
	for _, category := range categories {
		fmt.Fprintf(&b, "\n### %s\n\n", categoryLabel(category))
		b.WriteString("| shortName | Name | Wave | Code | Phrase | Severity | Detail |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, rt := range types {
			if rt.Category != category {
				continue
			}
			for _, f := range rt.Findings {
				fmt.Fprintf(&b, "| `%s` | %s | %s | `%s` | %s | %s | %s |\n",
					escapeMarkdownCell(rt.ShortName), escapeMarkdownCell(rt.Name),
					escapeMarkdownCell(f.Source), f.Code,
					phraseCell(f.Phrase), severityLabel(f.Severity), detailCell(f))
			}
		}
	}
	b.WriteString("\n")

	return updateGeneratedSection(path, "signals", b.String())
}

// categoryLabel title-cases a catalog category ("COMPUTE" -> "Compute") for
// the heading the hand-written signal tables already use.
func categoryLabel(category string) string {
	if category == "" {
		return "Other"
	}
	lower := strings.ToLower(category)
	return strings.ToUpper(lower[:1]) + lower[1:]
}

// phraseCell renders a finding's operator-facing phrase verbatim, so a reader
// can search the page for the words the row shows. Only the pipe is escaped,
// which would otherwise end the cell; a phrase carrying an asterisk or an
// underscore goes in a code span, where markdown reads neither as emphasis.
func phraseCell(phrase string) string {
	cell := strings.ReplaceAll(phrase, `|`, `\|`)
	if strings.ContainsAny(phrase, "*_") && !strings.Contains(phrase, "`") {
		return "`" + cell + "`"
	}
	return cell
}
