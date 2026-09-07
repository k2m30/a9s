package unit

// cites_docs_citations_test.go pins how docs/resources/*.md may refer to
// docs/attention-signals.md, and pins the hand-written signal tables in those
// resource docs against the shipped catalog.
//
// The signals page carries one generated table per category under `## Signals`
// and nothing else per type: there are no Wave 1/2/3 cells, no Source cells and
// no stable line numbers to point at. A citation that names one of those is a
// pointer into a page that no longer exists, and a reader who follows it lands
// nowhere. Two shapes, both resolved against the page itself, are the only
// citations that can still be checked after the next regeneration: one names a
// signal row, one names a hand-written section.
//
// The hand-written §4 tables are the other half: a doc that spells a list text
// or a severity the emitter never produces reads as the contract while the code
// reads as the bug.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A citation of one signal row, e.g.
// `docs/attention-signals.md § Signals § NETWORKING` row `vpce`.
var reCanonicalSignalsCitation = regexp.MustCompile(
	"`docs/attention-signals\\.md § Signals § ([^`]+)` row `([^`]+)`")

// A citation of a hand-written section of the page, e.g.
// `docs/attention-signals.md § Visualization Surfaces`. Only headings outside
// the generated block can be cited this way; a category heading inside it
// carries rows, so a citation of it names one.
var reSectionSignalsCitation = regexp.MustCompile(
	"`docs/attention-signals\\.md § ([^`]+)`")

// A generated table row: the first cell is the shortName in a code span.
var reGeneratedSignalRow = regexp.MustCompile("^\\|\\s*`([^`]+)`\\s*\\|")

// signalsCategories parses the generated block of docs/attention-signals.md
// into category heading -> set of shortNames registered under it. The page is
// the source of truth for both; hardcoding either here would make the gate
// stale the next time catalogen runs.
func signalsCategories(t *testing.T) map[string]map[string]bool {
	t.Helper()
	generated, _ := attentionSignalsDoc(t)

	categories := map[string]map[string]bool{}
	current := ""
	for _, line := range strings.Split(generated, "\n") {
		trimmed := strings.TrimSpace(line)
		if heading, ok := strings.CutPrefix(trimmed, "### "); ok {
			current = strings.TrimSpace(heading)
			if _, seen := categories[current]; !seen {
				categories[current] = map[string]bool{}
			}
			continue
		}
		if current == "" {
			continue
		}
		if m := reGeneratedSignalRow.FindStringSubmatch(trimmed); m != nil {
			categories[current][m[1]] = true
		}
	}
	if len(categories) == 0 {
		t.Fatal("docs/attention-signals.md: generated signals block has no `### <CATEGORY>` headings")
	}
	return categories
}

var reMarkdownHeading = regexp.MustCompile(`^#{2,3} (.+)$`)

// signalsSections parses the headings of docs/attention-signals.md that live
// outside the generated block — the page's hand-written sections, which are the
// only ones a doc can cite as a whole.
func signalsSections(t *testing.T) map[string]bool {
	t.Helper()
	_, handWritten := attentionSignalsDoc(t)

	sections := map[string]bool{}
	for _, line := range strings.Split(handWritten, "\n") {
		m := reMarkdownHeading.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		heading := strings.TrimSpace(m[1])
		// `## Signals` is the heading the generated block hangs under. Citing it
		// whole would name every signal of every type, which is no citation at
		// all — a signal is cited by its category and row.
		if heading == "Signals" {
			continue
		}
		sections[heading] = true
	}
	if len(sections) == 0 {
		t.Fatal("docs/attention-signals.md: no hand-written `##`/`###` headings outside the generated block")
	}
	return sections
}

// signalsCitingFiles returns every file the citation shape governs: the
// resource docs, plus the one production file that sends a reader to the page.
func signalsCitingFiles(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(projectRoot(t), "docs", "resources", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("docs/resources/*.md matched no files")
	}
	sort.Strings(paths)
	return append(paths, filepath.Join(projectRoot(t), "core", "aws", "eks.go"))
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(raw), "\n")
}

// relToRoot renders an absolute path the way a reader would type it, so the
// failure message is a location they can open.
func relToRoot(t *testing.T, path string) string {
	t.Helper()
	rel, err := filepath.Rel(projectRoot(t), path)
	if err != nil {
		return path
	}
	return rel
}

// TestCitesSignalsCitationShape walks every mention of the signals page in
// docs/resources/*.md and in core/aws/eks.go and resolves it against the page:
// a signal citation must name a category the generated block has a heading for
// and a type registered under it, a section citation must name a heading the
// page carries outside that block.
//
// The `generatedFrom:` list in the YAML front matter is a list of source files,
// not a citation into a heading; it is the one exempt shape.
func TestCitesSignalsCitationShape(t *testing.T) {
	categories := signalsCategories(t)
	sections := signalsSections(t)

	var (
		badShape   []string
		badCategry []string
		badType    []string
		badSection []string
	)

	for _, path := range signalsCitingFiles(t) {
		rel := relToRoot(t, path)
		lines := readLines(t, path)

		inFrontMatter := false
		for i, line := range lines {
			if strings.TrimSpace(line) == "---" && (i == 0 || inFrontMatter) {
				inFrontMatter = i == 0
				continue
			}

			for _, m := range reCanonicalSignalsCitation.FindAllStringSubmatch(line, -1) {
				category, shortName := m[1], m[2]
				types, ok := categories[category]
				if !ok {
					badCategry = append(badCategry, fmt.Sprintf(
						"%s:%d: cites category %q, which is not a `### ` heading under `## Signals`", rel, i+1, category))
					continue
				}
				if !types[shortName] {
					badType = append(badType, fmt.Sprintf(
						"%s:%d: cites row %q under %q, which that category's table does not carry", rel, i+1, shortName, category))
				}
			}

			residue := reCanonicalSignalsCitation.ReplaceAllString(line, "")

			for _, m := range reSectionSignalsCitation.FindAllStringSubmatch(residue, -1) {
				if !sections[m[1]] {
					badSection = append(badSection, fmt.Sprintf(
						"%s:%d: cites section %q, which is not a hand-written `##`/`###` heading of the page", rel, i+1, m[1]))
				}
			}
			residue = reSectionSignalsCitation.ReplaceAllString(residue, "")

			if inFrontMatter && strings.TrimSpace(residue) == "- docs/attention-signals.md" {
				continue
			}
			if strings.Contains(residue, "attention-signals") {
				badShape = append(badShape, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
			}
		}
	}

	if len(badShape) > 0 {
		t.Errorf("%d mention(s) of docs/attention-signals.md are neither of the two citation shapes — "+
			"`docs/attention-signals.md § Signals § <CATEGORY>` row `<type>` for a signal, "+
			"`docs/attention-signals.md § <heading>` for a hand-written section. No line numbers, no `Wave N cell`, "+
			"no `Source cell`, no quoted cell text the generated table does not carry. Where the old citation carried a "+
			"fact the page no longer has, the fact belongs in the doc's own sentence under its real source:\n%s",
			len(badShape), strings.Join(badShape, "\n"))
	}
	if len(badSection) > 0 {
		t.Errorf("%d citation(s) name a section docs/attention-signals.md does not have outside its generated block:\n%s",
			len(badSection), strings.Join(badSection, "\n"))
	}
	if len(badCategry) > 0 {
		t.Errorf("%d citation(s) name a category docs/attention-signals.md does not have:\n%s",
			len(badCategry), strings.Join(badCategry, "\n"))
	}
	if len(badType) > 0 {
		t.Errorf("%d citation(s) name a type that is not registered under the category they cite:\n%s",
			len(badType), strings.Join(badType, "\n"))
	}
}

// TestCitesCodeartifactPublicAccessDescribedByTheEngine pins that
// docs/resources/codeartifact.md no longer describes the public-access signal
// as a string match on the policy document. The policy engine in
// core/iampolicy/evaluate.go evaluates the statement; a doc that promises a
// substring search describes a check a9s does not run, and an operator reading
// it would expect a policy written any other way to go unflagged.
//
// The engine file was core/aws/catalog_security.go here until the citation
// batch: that file holds the SECURITY & IAM catalog literal and its color
// classifiers and names codeartifact nowhere, while
// core/aws/codeartifact_issue_enrichment.go calls iampolicy.Evaluate for the
// verdict. Pinning the old path would have put a false citation in the doc to
// make this test pass; do not restore it.
func TestCitesCodeartifactPublicAccessDescribedByTheEngine(t *testing.T) {
	path := filepath.Join(projectRoot(t), "docs", "resources", "codeartifact.md")
	lines := readLines(t, path)

	var offenders []string
	for i, line := range lines {
		if strings.Contains(line, `"Principal":"*"`) {
			offenders = append(offenders, fmt.Sprintf("%s:%d: %s", relToRoot(t, path), i+1, strings.TrimSpace(line)))
		}
	}
	if len(offenders) > 0 {
		t.Errorf("%d line(s) in docs/resources/codeartifact.md still describe the public-access signal as a "+
			"`\"Principal\":\"*\"` string match. Say what the policy engine decides — a statement that grants to any "+
			"principal — and cite core/iampolicy/evaluate.go:\n%s", len(offenders), strings.Join(offenders, "\n"))
	}

	if !strings.Contains(strings.Join(lines, "\n"), "core/iampolicy/evaluate.go") {
		t.Errorf("docs/resources/codeartifact.md does not cite core/iampolicy/evaluate.go, the file that decides " +
			"the public-access signal")
	}
}

// docSignalRow is one row of a resource doc's hand-written §4 table.
type docSignalRow struct {
	line       int
	stateWord  string
	listText   string
	rawLineTxt string
}

var reDocTableSeparator = regexp.MustCompile(`^\|[\s|:-]+\|$`)

// parseDocSignalTable reads the hand-written wave-to-surface table out of a
// resource doc: the one whose header carries a "State bucket" column and a
// "List text" column.
func parseDocSignalTable(t *testing.T, path string) []docSignalRow {
	t.Helper()
	lines := readLines(t, path)

	stateCol, textCol := -1, -1
	var rows []docSignalRow
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			stateCol, textCol = -1, -1
			continue
		}
		cells := splitTableRow(trimmed)
		if stateCol == -1 {
			for c, cell := range cells {
				switch {
				case strings.EqualFold(cell, "State bucket"):
					stateCol = c
				case strings.HasPrefix(cell, "List text"):
					textCol = c
				}
			}
			continue
		}
		if reDocTableSeparator.MatchString(trimmed) {
			continue
		}
		if stateCol >= len(cells) || textCol >= len(cells) || textCol == -1 {
			continue
		}
		rows = append(rows, docSignalRow{
			line:       i + 1,
			stateWord:  cells[stateCol],
			listText:   strings.Trim(cells[textCol], "`"),
			rawLineTxt: trimmed,
		})
	}
	if len(rows) == 0 {
		t.Fatalf("%s: no hand-written table with a \"State bucket\" and a \"List text\" column found", relToRoot(t, path))
	}
	return rows
}

func splitTableRow(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// catalogPhraseSeverities returns, for one shortName, every phrase the
// generated signals table carries and the severity words it carries it under.
// A phrase can be shared by two codes at different severities, so the value is
// a set.
func catalogPhraseSeverities(t *testing.T, shortName string) map[string]map[string]bool {
	t.Helper()
	generated, _ := attentionSignalsDoc(t)

	phrases := map[string]map[string]bool{}
	for _, line := range strings.Split(generated, "\n") {
		trimmed := strings.TrimSpace(line)
		m := reGeneratedSignalRow.FindStringSubmatch(trimmed)
		if m == nil || m[1] != shortName {
			continue
		}
		cells := splitTableRow(trimmed)
		// | shortName | Name | Wave | Code | Phrase | Severity | Detail |
		if len(cells) < 6 {
			continue
		}
		phrase, severity := strings.Trim(cells[4], "`"), strings.Trim(cells[5], "`")
		if phrases[phrase] == nil {
			phrases[phrase] = map[string]bool{}
		}
		phrases[phrase][severity] = true
	}
	if len(phrases) == 0 {
		t.Fatalf("docs/attention-signals.md: generated block carries no rows for %q", shortName)
	}
	return phrases
}

// severityWordForBucket maps the catalog's severity to the word the resource
// docs' §4 tables spell in their "State bucket" column.
var severityWordForBucket = map[string]string{
	"broken": "Broken",
	"warn":   "Warning",
	"dim":    "Dim",
}

// assertDocTableMatchesCatalog pins that every list text in a resource doc's
// hand-written §4 table is a phrase the emitter actually produces, at the
// severity the catalog gives it. A doc row that spells its own wording is a
// second definition of the finding, and the reader cannot tell which one ships.
func assertDocTableMatchesCatalog(t *testing.T, shortName string) {
	t.Helper()
	path := filepath.Join(projectRoot(t), "docs", "resources", shortName+".md")
	rel := relToRoot(t, path)
	phrases := catalogPhraseSeverities(t, shortName)

	known := make([]string, 0, len(phrases))
	for phrase := range phrases {
		known = append(known, phrase)
	}
	sort.Strings(known)

	for _, row := range parseDocSignalTable(t, path) {
		severities, ok := phrases[row.listText]
		if !ok {
			t.Errorf("%s:%d: the table promises list text %q, which no %s finding produces. "+
				"Say the phrase the catalog carries, or defer to the generated table. Registered phrases: %q\n  %s",
				rel, row.line, row.listText, shortName, known, row.rawLineTxt)
			continue
		}
		matched := false
		for severity := range severities {
			if severityWordForBucket[severity] == row.stateWord {
				matched = true
				break
			}
		}
		if !matched {
			want := make([]string, 0, len(severities))
			for severity := range severities {
				want = append(want, severityWordForBucket[severity])
			}
			sort.Strings(want)
			t.Errorf("%s:%d: the table puts %q in the %s bucket; the catalog ships that phrase as %q\n  %s",
				rel, row.line, row.listText, row.stateWord, want, row.rawLineTxt)
		}
	}
}

// TestCitesEksSignalTableMatchesTheCatalog pins the eks hand-written §4 table
// against the shipped definitions. Its health-issue row quoted a placeholder
// wording and called the finding Broken where the definition is warn, and the
// failed-cluster row promised a sentence the emitter never assembled.
func TestCitesEksSignalTableMatchesTheCatalog(t *testing.T) {
	assertDocTableMatchesCatalog(t, "eks")
}

// TestCitesCtEventsSignalTableMatchesTheCatalog pins the ct-events
// hand-written §4 table against the shipped definitions, for the same reason.
func TestCitesCtEventsSignalTableMatchesTheCatalog(t *testing.T) {
	assertDocTableMatchesCatalog(t, "ct-events")
}

var (
	reSmokeCapture  = regexp.MustCompile(`capture-pane[^>]*>\s*"\$CAPDIR/([A-Za-z0-9_]+\.txt)"`)
	reSmokeDetailNg = regexp.MustCompile(`^ng[a-z0-9_]*_detail\.txt$|^ng_detail\.txt$`)
	reSmokeOpenDtl  = regexp.MustCompile(`send-keys[^\n]*'d'`)
)

// TestCitesSmokeDemoCapturesNgDetail pins that the demo smoke walks an EKS node
// group into its detail view and asserts the health-issue cause there. The
// health issue is a supporting row now, so the only place it can be seen is the
// detail body — a list-only capture proves nothing about it.
func TestCitesSmokeDemoCapturesNgDetail(t *testing.T) {
	path := filepath.Join(projectRoot(t), "scripts", "smoke-demo.sh")
	rel := relToRoot(t, path)
	lines := readLines(t, path)

	capture, captureLine := "", -1
	lastCaptureBefore := -1
	for i, line := range lines {
		m := reSmokeCapture.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if reSmokeDetailNg.MatchString(m[1]) {
			capture, captureLine = m[1], i
			break
		}
		lastCaptureBefore = i
	}
	if capture == "" {
		t.Fatalf("%s: no ng detail capture — the sibling walks send `:<type>`, a filter, `d`, then "+
			"capture-pane into \"$CAPDIR/<name>.txt\" and assert on it with expect", rel)
	}

	opened := false
	for _, line := range lines[lastCaptureBefore+1 : captureLine] {
		if reSmokeOpenDtl.MatchString(line) {
			opened = true
			break
		}
	}
	if !opened {
		t.Errorf("%s:%d: %s is captured without a `d` keypress since the previous capture, so it is a list "+
			"capture, not a detail one", rel, captureLine+1, capture)
	}

	wantExpect := regexp.MustCompile(`^\s*expect\s+` + regexp.QuoteMeta(capture) + `\s+"health issue"`)
	asserted := false
	for _, line := range lines {
		if wantExpect.MatchString(line) {
			asserted = true
			break
		}
	}
	if !asserted {
		t.Errorf("%s: %s is captured but nothing asserts the health-issue row on it — "+
			`add expect %s "health issue" "<label>"`, rel, capture, capture)
	}
}
