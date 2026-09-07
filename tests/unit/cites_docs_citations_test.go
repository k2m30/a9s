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
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
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

// Prose that names a cell of the per-type tables the generated block replaced,
// with or without naming the file it is describing.
var reSignalsCellReference = regexp.MustCompile(`Wave [0-9] cell|Source cell|Source column`)

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
		badCellRef []string
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

			if m := reSignalsCellReference.FindString(line); m != "" {
				badCellRef = append(badCellRef, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
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
	if len(badCellRef) > 0 {
		t.Errorf("%d line(s) describe a Wave or Source cell of the signals page. The page has no such cells — one "+
			"generated table per category, keyed by code. Dropping the file name does not make the claim true:\n%s",
			len(badCellRef), strings.Join(badCellRef, "\n"))
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
	condition  string
	waveWord   string
	stateWord  string
	glyphCell  string
	surfaceTxt string
	listText   string
	// deferred is set when the row's condition carries the page's
	// "not implemented" marker in the one shape the docs spell it.
	deferred bool
	// markerOffShape is set when the row says NOT IMPLEMENTED in any other
	// wording, which no reader and no gate can resolve against the page.
	markerOffShape bool
}

var reDocTableSeparator = regexp.MustCompile(`^\|[\s|:-]+\|$`)

// The one shape a deferred row spells its marker in. Every doc that defers a
// row today writes exactly this, and the gate resolves the row against
// `docs/attention-signals.md § Not yet implemented` through it, so a second
// wording is a row nothing can check.
var reDeferredMarker = regexp.MustCompile(`— NOT IMPLEMENTED \(backlog; no emission in code as of \d{4}-\d{2}-\d{2}\)`)

const deferredMarkerShape = "— NOT IMPLEMENTED (backlog; no emission in code as of <YYYY-MM-DD>)"

// parseDocSignalTable reads the hand-written wave-to-surface table out of a
// resource doc: the one whose header carries a "State bucket" column and a
// "List text" column. The second result is false for a doc that carries no
// such table — an implementation plan rather than a resource design — which
// the completeness walk skips rather than fails.
func parseDocSignalTable(t *testing.T, path string) ([]docSignalRow, bool) {
	t.Helper()
	lines := readLines(t, path)

	waveCol, stateCol, textCol, condCol := -1, -1, -1, 0
	glyphCol, surfaceCol := -1, -1
	var rows []docSignalRow
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			waveCol, stateCol, textCol, condCol = -1, -1, -1, 0
			glyphCol, surfaceCol = -1, -1
			continue
		}
		cells := splitTableRow(trimmed)
		if stateCol == -1 {
			for c, cell := range cells {
				switch {
				case strings.EqualFold(cell, "Wave"):
					waveCol = c
				case strings.EqualFold(cell, "State bucket"):
					stateCol = c
				case strings.HasPrefix(cell, "List text"):
					textCol = c
				case strings.HasPrefix(cell, "Signal"):
					condCol = c
				case strings.EqualFold(cell, "Severity"):
					glyphCol = c
				case strings.HasPrefix(cell, "Surfaces"):
					surfaceCol = c
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
		row := docSignalRow{
			line:      i + 1,
			stateWord: cells[stateCol],
			listText:  strings.Trim(cells[textCol], "`"),
		}
		if condCol < len(cells) {
			row.condition = cells[condCol]
		}
		if glyphCol >= 0 && glyphCol < len(cells) {
			row.glyphCell = docGlyphCell(cells[glyphCol])
		}
		if surfaceCol >= 0 && surfaceCol < len(cells) {
			row.surfaceTxt = cells[surfaceCol]
		}
		row.deferred = reDeferredMarker.MatchString(row.condition)
		row.markerOffShape = !row.deferred && strings.Contains(row.condition, "NOT IMPLEMENTED")
		if waveCol >= 0 && waveCol < len(cells) {
			row.waveWord = cells[waveCol]
		}
		rows = append(rows, row)
	}
	return rows, len(rows) > 0
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
func catalogSignalsByType(t *testing.T) map[string][]catalogSignal {
	t.Helper()
	generated, _ := attentionSignalsDoc(t)

	byType := map[string][]catalogSignal{}
	for _, line := range strings.Split(generated, "\n") {
		trimmed := strings.TrimSpace(line)
		m := reGeneratedSignalRow.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		cells := splitTableRow(trimmed)
		// | shortName | Name | Wave | Code | Phrase | Severity | Detail |
		if len(cells) < 6 {
			continue
		}
		byType[m[1]] = append(byType[m[1]], catalogSignal{
			wave:     strings.Trim(cells[2], "`"),
			code:     strings.Trim(cells[3], "`"),
			phrase:   strings.Trim(cells[4], "`"),
			severity: strings.Trim(cells[5], "`"),
		})
	}
	if len(byType) == 0 {
		t.Fatal("docs/attention-signals.md: generated block carries no signal rows")
	}
	return byType
}

// catalogSignal is one row of the generated signals table.
type catalogSignal struct {
	wave     string
	code     string
	phrase   string
	severity string
}

// severityWordForBucket maps the catalog's severity to the word the resource
// docs' §4 tables spell in their "State bucket" column.
var severityWordForBucket = map[string]string{
	"broken": "Broken",
	"warn":   "Warning",
	"dim":    "Dim",
}

// The kinds of contradiction the completeness walk reports. A reader fixing a
// doc needs to know which cell is wrong, so each row of the failure output
// names one.
const (
	kindPhrase       = "phrase nothing emits"
	kindMissingRow   = "shipped finding without a row"
	kindSection      = "section mismatch"
	kindWave         = "wrong wave"
	kindBucket       = "wrong bucket"
	kindMarker       = "stale not-implemented marker"
	kindGlyph        = "wrong severity glyph"
	kindSurfaces     = "wrong surfaces"
	kindNoSignals    = "claims no signals in a wave that ships one"
	kindNoFinding    = "claims no finding row for a registered finding"
	kindSuppressed   = "claims a surface is suppressed"
	kindTierInList   = "puts the tier in the list"
	kindQuotedPhrase = "quotes a phrase nothing emits"
	kindQuotedColour = "quotes a phrase under the wrong colour"
)

// expectedGlyph derives the Severity cell of a §4 row from the finding alone.
// The renderer is the only source: internal/tui/views/detail_fields.go builds
// the detail-view Attention section, one entry per finding of either wave, and
// gives it `!` at SevBroken and `~` otherwise. A finding that is not
// Severity.IsIssue() — SevDim, SevOK — is skipped there, so it has no tier.
//
// Not the list decorator. core/app/list_columns.go resolveListRowSeverity
// reaches its glyph branch only for a row td.ResolveColor calls Healthy, and
// tests/unit/qa_color_findings_conformance_test.go holds every registered type
// to "colour is the worst finding across both waves" with an empty divergence
// allowlist, so a Healthy row carries no warn or broken finding and that branch
// cannot fire. A glyph derived from the list row is not to be restored.
func expectedGlyph(sig catalogSignal) string {
	switch sig.severity {
	case "broken":
		return "!"
	case "warn":
		return "~"
	}
	return "n/a"
}

// expectedS1 derives whether a finding reaches surface S1, the menu `issues:N`
// count and the list-title `!N` suffix. Same one source: core/app/list_body.go
// counts a row whose wave-1 colour is an issue (`ResolveColor(Wave1Only(r)).IsIssue()`,
// which is Warning or Broken and not Dim), and `listHasBadgeFinding` plus the
// findings-map branch add a wave-2 finding only at SevBroken — "~ findings do
// not bump".
func expectedS1(sig catalogSignal) bool {
	if sig.wave == "wave2" {
		return sig.severity == "broken"
	}
	return sig.severity == "broken" || sig.severity == "warn"
}

func shortNameOf(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

var reSurfaceCell = regexp.MustCompile(`S[1-5]`)

// expectedSurfaces derives the whole Surfaces cell from the finding. S2 and S4
// are the row's colour and its cause text, which every finding with a row has.
// S3 and S5 are the tier and the sentence of the detail-view Attention section,
// so they follow that section's own issue-severity test — a Dim finding is
// skipped there and reaches neither. S1 is expectedS1.
func expectedSurfaces(sig catalogSignal) string {
	surfaces := []string{"S2", "S4"}
	if expectedGlyph(sig) != "n/a" {
		surfaces = []string{"S2", "S3", "S4", "S5"}
	}
	if expectedS1(sig) {
		surfaces = append([]string{"S1"}, surfaces...)
	}
	return strings.Join(surfaces, ", ")
}

// docGlyphCell normalises the Severity cell: the docs write it as `n/a`, as a
// code-spanned glyph, and once as "`!` (counted)".
func docGlyphCell(cell string) string {
	cell = strings.TrimSpace(strings.Trim(strings.Fields(cell + " ")[0], "`"))
	if cell == "" {
		return "n/a"
	}
	return cell
}

// docOffender is one contradiction between a resource doc and the shipped
// definitions, at the line a reader opens to fix it.
type docOffender struct {
	line int
	kind string
	want string
}

var reConditionWord = regexp.MustCompile(`[a-z0-9]+`)

// Words that carry no identity: they appear in half the conditions on the page
// and matching on them would resolve a deferred row against any line at all.
var conditionStopWords = map[string]bool{
	"with": true, "that": true, "than": true, "from": true, "this": true,
	"when": true, "which": true, "there": true, "their": true, "have": true,
	"been": true, "into": true, "over": true, "only": true, "more": true,
	"some": true, "same": true, "each": true, "also": true, "wave": true,
	"signal": true, "warning": true, "broken": true, "healthy": true,
}

// conditionWords reduces a condition to the words that identify it: four
// letters or more, lowercased, punctuation and markdown dropped.
func conditionWords(text string) map[string]bool {
	words := map[string]bool{}
	for _, w := range reConditionWord.FindAllString(strings.ToLower(text), -1) {
		if len(w) >= 4 && !conditionStopWords[w] {
			words[w] = true
		}
	}
	return words
}

// notYetImplementedConditions parses `docs/attention-signals.md § Not yet
// implemented` into shortName -> the conditions its line records, one per
// semicolon-separated clause. That section is the page's register of what the
// designs describe and no code emits, so it is the only place a doc may point
// at for a row it renders and the emitter never raises.
func notYetImplementedConditions(t *testing.T) map[string][]string {
	t.Helper()
	_, handWritten := attentionSignalsDoc(t)
	idx := strings.Index(handWritten, "## Not yet implemented")
	if idx == -1 {
		t.Fatal("docs/attention-signals.md: no `## Not yet implemented` heading")
	}

	byType := map[string][]string{}
	for _, line := range strings.Split(handWritten[idx:], "\n") {
		m := reNotYetImplementedLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		for _, clause := range strings.Split(m[2], ";") {
			byType[m[1]] = append(byType[m[1]], clause)
		}
	}
	if len(byType) == 0 {
		t.Fatal("docs/attention-signals.md: `## Not yet implemented` records no `- `type`` lines")
	}
	return byType
}

// pageRecordsCondition reports whether one of the page's lines for this type
// records the doc's deferred row. The two sides never spell a condition
// identically — one quotes the SDK field, the other the operator's words — so
// the rule is that they share at least one identifying word, singular and
// plural counting as one. A row that shares none is pointing at a line the page
// does not carry, and neither a reader nor this gate can tell which line it
// meant.
func pageRecordsCondition(rowCondition string, pageConditions []string) bool {
	row := conditionWords(reDeferredMarker.ReplaceAllString(rowCondition, ""))
	for _, page := range pageConditions {
		for w := range conditionWords(page) {
			for r := range row {
				if strings.HasPrefix(r, w) || strings.HasPrefix(w, r) {
					return true
				}
			}
		}
	}
	return false
}

// docCompletenessOffenders resolves one resource doc against the shipped
// definitions: every phrase its §4 table promises is one the emitter produces,
// at the severity and in the wave the catalog gives it; every finding the
// catalog ships for the type has a row; §3 and §4 describe the same signals;
// and a row the doc renders for a condition nothing emits is recorded as
// deferred on the signals page. A doc that spells its own wording, bucket or
// wave is a second definition of the finding, and the reader cannot tell which
// one ships.
//
// The second result is false when the doc carries no §3/§4 at all, or no type
// the catalog knows — an implementation plan, not a resource design.
func docCompletenessOffenders(t *testing.T, path string) ([]docOffender, bool) {
	t.Helper()
	rows, ok := parseDocSignalTable(t, path)
	if !ok {
		return nil, false
	}
	shortName := shortNameOf(path)
	signals := catalogSignalsByType(t)[shortName]
	if len(signals) == 0 {
		return nil, false
	}

	known := make([]string, 0, len(signals))
	for _, sig := range signals {
		known = append(known, sig.phrase)
	}
	sort.Strings(known)

	deferredConditions := notYetImplementedConditions(t)[shortName]

	var offenders []docOffender
	rendered := map[string]int{}
	for _, row := range rows {
		if row.markerOffShape {
			offenders = append(offenders, docOffender{row.line, kindMarker, fmt.Sprintf(
				"the row defers %q in a wording no gate can resolve; the one shape is %q",
				row.listText, deferredMarkerShape)})
			continue
		}
		if row.deferred {
			if !pageRecordsCondition(row.condition, deferredConditions) {
				offenders = append(offenders, docOffender{row.line, kindMarker, fmt.Sprintf(
					"the row defers %q, and `docs/attention-signals.md § Not yet implemented` records no such "+
						"condition for `%s` (it records %q); record it there, or delete the row",
					row.listText, shortName, deferredConditions)})
			}
			continue
		}

		rendered[row.listText]++

		var matching []catalogSignal
		for _, sig := range signals {
			if sig.phrase == row.listText {
				matching = append(matching, sig)
			}
		}
		if len(matching) == 0 {
			offenders = append(offenders, docOffender{row.line, kindPhrase, fmt.Sprintf(
				"the row promises list text %q, which no `%s` finding produces; say a phrase the catalog carries: %q",
				row.listText, shortName, known)})
			continue
		}

		if !anySignal(matching, func(sig catalogSignal) bool {
			return severityWordForBucket[sig.severity] == row.stateWord
		}) {
			offenders = append(offenders, docOffender{row.line, kindBucket, fmt.Sprintf(
				"the row puts %q in the %s bucket; the catalog ships that phrase as %q",
				row.listText, row.stateWord, signalWords(matching, func(sig catalogSignal) string {
					return severityWordForBucket[sig.severity]
				}))})
		}

		if row.waveWord != "" && !anySignal(matching, func(sig catalogSignal) bool {
			return sig.wave == "wave"+row.waveWord
		}) {
			offenders = append(offenders, docOffender{row.line, kindWave, fmt.Sprintf(
				"the row calls %q a wave-%s signal; the catalog ships it as %q",
				row.listText, row.waveWord, signalWords(matching, func(sig catalogSignal) string {
					return sig.wave
				}))})
		}

		if strings.Contains(row.condition, "no finding row") {
			offenders = append(offenders, docOffender{row.line, kindNoFinding, fmt.Sprintf(
				"the row says %q has no finding row, and the catalog registers it as %q; "+
					"a signal in the generated table is a finding by construction",
				row.listText, signalWords(matching, func(sig catalogSignal) string { return sig.code }))})
		}

		if row.glyphCell != "" && !anySignal(matching, func(sig catalogSignal) bool {
			return expectedGlyph(sig) == row.glyphCell
		}) {
			offenders = append(offenders, docOffender{row.line, kindGlyph, fmt.Sprintf(
				"the row gives %q the glyph %q; the renderer gives a %s %s finding %q",
				row.listText, row.glyphCell, matching[0].wave, matching[0].severity, expectedGlyph(matching[0]))})
		}

		if row.surfaceTxt != "" {
			listed := strings.Join(reSurfaceCell.FindAllString(row.surfaceTxt, -1), ", ")
			if !anySignal(matching, func(sig catalogSignal) bool { return expectedSurfaces(sig) == listed }) {
				offenders = append(offenders, docOffender{row.line, kindSurfaces, fmt.Sprintf(
					"the row reaches %q for %q; a %s %s finding reaches %q",
					listed, row.listText, matching[0].wave, matching[0].severity, expectedSurfaces(matching[0]))})
			}
		}
	}

	// One row per shipped finding, counted rather than looked up: two codes can
	// share a phrase (`modifying` is both `redshift.warn.modifying` and
	// `redshift.warn.availability_modifying`), and a single row for the pair
	// documents one of the two conditions and hides the other.
	codesForPhrase := map[string][]string{}
	for _, sig := range signals {
		codesForPhrase[sig.phrase] = append(codesForPhrase[sig.phrase], sig.code)
	}
	for phrase, codes := range codesForPhrase {
		if rendered[phrase] >= len(codes) {
			continue
		}
		sort.Strings(codes)
		offenders = append(offenders, docOffender{rows[0].line, kindMissingRow, fmt.Sprintf(
			"the §4 table renders %d row(s) for %q and the catalog ships %d finding(s) under that phrase (%s); "+
				"the table is one row per signal, so a shipped finding missing from it is a signal the doc tells "+
				"the reader does not exist",
			rendered[phrase], phrase, len(codes), strings.Join(codes, ", "))})
	}

	offenders = append(offenders, docSentenceOffenders(t, path, signals)...)

	sectionBuckets, sectionLine := docSectionBuckets(t, path)
	tableBuckets := map[string]int{}
	for _, row := range rows {
		if row.deferred || row.markerOffShape {
			continue
		}
		tableBuckets[row.stateWord]++
	}
	if !reflect.DeepEqual(sectionBuckets, tableBuckets) {
		offenders = append(offenders, docOffender{sectionLine, kindSection, fmt.Sprintf(
			"§3 describes signals in the buckets %s while the §4 table renders %s; §4 says every signal from §3 "+
				"lands on a surface, so a bucket in one and not the other is a signal the doc either never renders "+
				"or renders without describing",
			formatBucketCounts(sectionBuckets), formatBucketCounts(tableBuckets))})
	}

	sort.SliceStable(offenders, func(i, j int) bool { return offenders[i].line < offenders[j].line })
	return offenders, true
}

// Two shapes of the same claim: "no Wave 2 signals" and "there is no Wave 2,".
// A wave named without a following clause boundary is about an API call rather
// than a signal — "no Wave 2 per-resource fan-out is needed" is true of a type
// that ships wave-2 findings from the list response.
var reNoWaveSignals = regexp.MustCompile(`(?i)no wave ([123])(?:(?: [a-z]+)* signals|[,.])`)

var reS3Suppressed = regexp.MustCompile(`S3 (is )?suppress`)

// The §4.1 paragraph quotes what the operator reads off the list, so a code
// span in it is a promise about a Status cell. Resolving those spans against
// the type's generated table is the same rule the §4 table rows already carry
// — a phrase spelled here and nowhere in the catalog is a second definition of
// the finding, and the reader cannot tell which one ships.
//
// Not every span on the line is such a promise. Four shapes name something
// other than list text and are read past: an SDK field or shape
// (`StateReason`, `CertificateDetail.FailureReason`), an API enum constant
// (`UPDATE_FAILED`), a resource type the reader can pivot to (`ct-events`),
// and a comparison, which states a condition rather than a cell
// (`Status == FAILED`).
var (
	reCodeSpan      = regexp.MustCompile("`([^`]+)`")
	reSDKIdentifier = regexp.MustCompile(`^[a-z]*[A-Z][A-Za-z0-9]*$|^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)+$`)
	reAPIEnum       = regexp.MustCompile(`^[A-Z][A-Z0-9_*]*$`)
	reComparison    = regexp.MustCompile(`==| [<>] `)
	reColourWord    = regexp.MustCompile(`(?i)\b(red|yellow|dim|grey|gray)\b`)
)

// colourSeverity maps the colour word a §4.1 sentence puts beside a phrase to
// the severity the catalog registers that phrase under. Green has no entry: a
// healthy row carries no finding, so a phrase quoted as green is a phrase the
// list never shows for a reason the catalog knows.
var colourSeverity = map[string]string{
	"red": "broken", "yellow": "warn", "dim": "dim", "grey": "dim", "gray": "dim",
}

// listTextClaim is one code span of a §4.1 paragraph, with the colour word the
// sentence puts in front of it. The colour is the last one between the end of
// the previous span and the start of this one, which is where the docs put it
// ("a red row reading `public access policy`"); a phrase with no colour word in
// front of it is checked for existence only.
type listTextClaim struct {
	text   string
	colour string
}

// listTextClaims splits a §4.1 line into the spans that promise list text. A
// span the type ships as a phrase is a claim whatever else it looks like:
// `alarm` is both a phrase of `alarm_history` and the short name of another
// type, and reading it as the short name would drop the row's colour check.
func listTextClaims(line string, phrases, pivots map[string]bool) []listTextClaim {
	var claims []listTextClaim
	prev := 0
	for _, m := range reCodeSpan.FindAllStringSubmatchIndex(line, -1) {
		text := line[m[2]:m[3]]
		colour := ""
		if c := reColourWord.FindAllString(line[prev:m[0]], -1); len(c) > 0 {
			colour = strings.ToLower(c[len(c)-1])
		}
		prev = m[1]
		if !phrases[text] && (reSDKIdentifier.MatchString(text) || reAPIEnum.MatchString(text) ||
			reComparison.MatchString(text) || pivots[text]) {
			continue
		}
		claims = append(claims, listTextClaim{text, colour})
	}
	return claims
}

// The §4.1 paragraph answers one question — what the operator can read off the
// list without opening detail — so naming the tier in it says the tier is on
// the list row. It is not: it is an entry in the detail-view Attention section
// (internal/tui/views/detail_fields.go), which is the thing opening detail
// shows. A page that wants to say where the tier lives says it in §5.
var (
	reUXReviewLine = regexp.MustCompile(`^At 3am`)
	reTierToken    = regexp.MustCompile("`!`|`~`|glyph")
)

// docSentenceOffenders finds prose that contradicts the table below it: a
// sentence telling the reader a wave carries nothing for this type while the
// catalog ships a finding in it, and a sentence saying S3 is suppressed on a
// coloured row. S3 is the tier of the detail-view Attention section, which
// renders for every issue-severity finding of either wave and knows nothing
// about the row's colour — there is no case in which it is suppressed. Both
// sentences are read before the table that disagrees with them.
func docSentenceOffenders(t *testing.T, path string, signals []catalogSignal) []docOffender {
	t.Helper()
	shipped := map[string]int{}
	phrases := map[string]bool{}
	known := make([]string, 0, len(signals))
	for _, sig := range signals {
		shipped[sig.wave]++
		if !phrases[sig.phrase] {
			phrases[sig.phrase] = true
			known = append(known, sig.phrase)
		}
	}
	sort.Strings(known)
	pivots := pivotNames(t)

	var offenders []docOffender
	for i, line := range readLines(t, path) {
		if reUXReviewLine.MatchString(line) {
			if tok := reTierToken.FindString(line); tok != "" {
				offenders = append(offenders, docOffender{i + 1, kindTierInList, fmt.Sprintf(
					"the §4.1 paragraph names %s while answering what the list shows without opening detail; "+
						"the tier is an entry in the detail-view Attention section", tok)})
			}
			for _, claim := range listTextClaims(line, phrases, pivots) {
				matching := signalsWithPhrase(signals, claim.text)
				if len(matching) == 0 {
					offenders = append(offenders, docOffender{i + 1, kindQuotedPhrase, fmt.Sprintf(
						"the §4.1 paragraph quotes %q as what the list shows, and no `%s` finding produces it; "+
							"quote a phrase the catalog carries: %q", claim.text, shortNameOf(path), known)})
					continue
				}
				want, ok := colourSeverity[claim.colour]
				if !ok {
					continue
				}
				if !anySignal(matching, func(sig catalogSignal) bool { return sig.severity == want }) {
					offenders = append(offenders, docOffender{i + 1, kindQuotedColour, fmt.Sprintf(
						"the §4.1 paragraph calls the row carrying %q %s, and the catalog ships that phrase as %q",
						claim.text, claim.colour, signalWords(matching, func(sig catalogSignal) string {
							return sig.severity
						}))})
				}
			}
		}
		if reS3Suppressed.MatchString(line) {
			offenders = append(offenders, docOffender{i + 1, kindSuppressed, fmt.Sprintf(
				"the sentence says S3 is suppressed; S3 is the detail-view Attention tier, which every %s finding "+
					"of issue severity reaches whatever colour its row is", shortNameOf(path))})
		}
		m := reNoWaveSignals.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if n := shipped["wave"+m[1]]; n > 0 {
			offenders = append(offenders, docOffender{i + 1, kindNoSignals, fmt.Sprintf(
				"the sentence says wave %s carries no signals, and the catalog ships %d of them in that wave",
				m[1], n)})
		}
	}
	return offenders
}

// signalsWithPhrase returns every finding of the type registered under one
// phrase; a phrase can be shared by two codes at different severities.
func signalsWithPhrase(signals []catalogSignal, phrase string) []catalogSignal {
	var matching []catalogSignal
	for _, sig := range signals {
		if sig.phrase == phrase {
			matching = append(matching, sig)
		}
	}
	return matching
}

func anySignal(signals []catalogSignal, pred func(catalogSignal) bool) bool {
	for _, sig := range signals {
		if pred(sig) {
			return true
		}
	}
	return false
}

func signalWords(signals []catalogSignal, get func(catalogSignal) string) []string {
	seen := map[string]bool{}
	var words []string
	for _, sig := range signals {
		if w := get(sig); !seen[w] {
			seen[w] = true
			words = append(words, w)
		}
	}
	sort.Strings(words)
	return words
}

var reDocSignalBucket = regexp.MustCompile(`\*\*State bucket\*\*:\s*([A-Za-z]+)`)

// docSectionBuckets counts the buckets a doc's §3 signal bullets describe, and
// returns the line of the first one so a mismatch points somewhere. A bullet
// carrying the deferred marker is left out on both sides of the comparison: it
// describes a signal the doc has already told the reader does not ship.
func docSectionBuckets(t *testing.T, path string) (map[string]int, int) {
	t.Helper()
	lines := readLines(t, path)

	inSection3, deferred := false, false
	firstLine := 1
	buckets := map[string]int{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := reMarkdownHeading.FindStringSubmatch(trimmed); m != nil {
			inSection3 = strings.HasPrefix(m[1], "3.") || strings.HasPrefix(m[1], "3 ")
			deferred = false
		}
		if !inSection3 {
			continue
		}
		// The marker sits on the bullet's own line and the bucket on a
		// sub-bullet under it, so the flag is carried from one to the other and
		// dropped at the next top-level bullet.
		if strings.HasPrefix(line, "- ") {
			deferred = strings.Contains(line, "NOT IMPLEMENTED")
		}
		if m := reDocSignalBucket.FindStringSubmatch(trimmed); m != nil && !deferred {
			if len(buckets) == 0 {
				firstLine = i + 1
			}
			buckets[m[1]]++
		}
	}
	return buckets, firstLine
}

func formatBucketCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s×%d", k, counts[k])
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// TestCompleteResourceDocsListExactlyWhatShips walks every resource design doc
// and resolves its hand-written signal sections against the shipped
// definitions. There is no per-doc opt-in: a doc that describes signals is a
// doc a reader trusts, and one that describes them wrong is a contract the
// reader follows into a behaviour a9s does not have. Docs with no §3/§4 table,
// and docs whose name is no type the catalog knows, are skipped by that
// property — they are implementation plans, not designs.
func TestCompleteResourceDocsListExactlyWhatShips(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(projectRoot(t), "docs", "resources", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("docs/resources/*.md matched no files")
	}
	sort.Strings(paths)

	walked, offending, total := 0, 0, 0
	byKind := map[string]int{}
	for _, path := range paths {
		offenders, ok := docCompletenessOffenders(t, path)
		if !ok {
			continue
		}
		walked++
		if len(offenders) == 0 {
			continue
		}
		offending++
		total += len(offenders)

		rel := relToRoot(t, path)
		report := make([]string, len(offenders))
		for i, off := range offenders {
			byKind[off.kind]++
			report[i] = fmt.Sprintf("  %s:%d: %s — %s", rel, off.line, off.kind, off.want)
		}
		t.Errorf("%s contradicts the shipped definitions in %d place(s):\n%s",
			rel, len(offenders), strings.Join(report, "\n"))
	}

	if offending > 0 {
		kinds := make([]string, 0, len(byKind))
		for k, n := range byKind {
			kinds = append(kinds, fmt.Sprintf("%s×%d", k, n))
		}
		sort.Strings(kinds)
		t.Errorf("%d of %d resource docs contradict the shipped definitions, %d offender(s) in all: %s",
			offending, walked, total, strings.Join(kinds, ", "))
	}
	if walked == 0 {
		t.Fatal("docs/resources/*.md: no doc carried a §4 signal table and a catalog type")
	}
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

var reNotYetImplementedLine = regexp.MustCompile("^- `([a-z0-9-]+)` — (.+)$")

// TestCitesNotYetImplementedNamesNothingThatShips pins the page's "Not yet
// implemented" list: it is the place for conditions no code emits, so a line
// whose type already carries a finding of that name in the generated table is
// telling the reader a shipped signal is missing.
func TestCitesNotYetImplementedNamesNothingThatShips(t *testing.T) {
	_, handWritten := attentionSignalsDoc(t)
	idx := strings.Index(handWritten, "## Not yet implemented")
	if idx == -1 {
		t.Fatal("docs/attention-signals.md: no `## Not yet implemented` heading")
	}

	byType := catalogSignalsByType(t)

	for i, line := range strings.Split(handWritten[idx:], "\n") {
		m := reNotYetImplementedLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		shortName, text := m[1], strings.ToLower(m[2])
		for _, sig := range byType[shortName] {
			if strings.Contains(text, strings.ToLower(sig.phrase)) || strings.Contains(text, strings.ToLower(sig.code)) {
				t.Errorf("docs/attention-signals.md, %d line(s) into `## Not yet implemented`: %q is listed as not "+
					"implemented, but the generated table ships %q for `%s` as %q. Drop the line, or say what part of "+
					"the condition is still missing in words the shipped finding does not use:\n  %s",
					i, sig.phrase, sig.phrase, shortName, sig.code, strings.TrimSpace(line))
			}
		}
	}
}

// TestCitesNotYetImplementedCitationsResolveToALine pins that a resource doc
// citing `§ Not yet implemented` finds its own deferred signal there. The
// section is per-type, so a citation from a type it carries no line for
// attributes the doc's deferral to a page that does not record it.
func TestCitesNotYetImplementedCitationsResolveToALine(t *testing.T) {
	_, handWritten := attentionSignalsDoc(t)
	idx := strings.Index(handWritten, "## Not yet implemented")
	if idx == -1 {
		t.Fatal("docs/attention-signals.md: no `## Not yet implemented` heading")
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(handWritten[idx:], "\n") {
		if m := reNotYetImplementedLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			listed[m[1]] = true
		}
	}

	const citation = "`docs/attention-signals.md § Not yet implemented`"
	var offenders []string
	for _, path := range signalsCitingFiles(t) {
		shortName := strings.TrimSuffix(filepath.Base(path), ".md")
		if listed[shortName] {
			continue
		}
		for i, line := range readLines(t, path) {
			if strings.Contains(line, citation) {
				offenders = append(offenders, fmt.Sprintf("%s:%d: %s", relToRoot(t, path), i+1, strings.TrimSpace(line)))
			}
		}
	}
	if len(offenders) > 0 {
		t.Errorf("%d citation(s) of `§ Not yet implemented` come from a type that section carries no line for. "+
			"Either the deferred signal belongs on the page, or the sentence should say what is deferred without "+
			"citing a section that does not record it:\n%s", len(offenders), strings.Join(offenders, "\n"))
	}
}

// ─── docs/related-resources.md — the related-panel contract ─────────────────

// The related contract page is edited under its own rules and its sections are
// renumbered whenever a pivot is added, so a line number in a citation is a
// pointer that rots on the next edit. Headings do not: every per-type block is
// a `### ` heading named for the type, and the three policy sections are `## `
// headings. One shape, resolved against the page.
var (
	reRelatedMention    = regexp.MustCompile(`related-resources\.md`)
	reRelatedLineNumber = regexp.MustCompile(`\blines?\s+[0-9]`)
	reRelatedFrontEntry = regexp.MustCompile(`^\s*-\s+docs/related-resources\.md\s*$`)
)

// relatedSections parses the headings of docs/related-resources.md, stripped of
// the backticks the per-type ones are spelled with. The page is the source of
// truth; a hand list here would let a heading be renamed without the gate
// noticing.
func relatedSections(t *testing.T) []string {
	t.Helper()
	var sections []string
	for _, line := range readLines(t, filepath.Join(projectRoot(t), "docs", "related-resources.md")) {
		if m := reMarkdownHeading.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			sections = append(sections, strings.TrimSpace(strings.ReplaceAll(m[1], "`", "")))
		}
	}
	if len(sections) == 0 {
		t.Fatal("docs/related-resources.md: no `##`/`###` headings")
	}
	// Longest first, so `sns-sub` is preferred over `sns` where both would fit.
	sort.SliceStable(sections, func(i, j int) bool { return len(sections[i]) > len(sections[j]) })
	return sections
}

// relatedHeadingCited resolves the text that follows a `§` in a citation. The
// citation names a heading and then narrows it in prose ("§ Per-type contract,
// row `acm`"), so the heading is a prefix of that text rather than the whole of
// it, and the character after it must not continue a word — `sns` does not
// resolve a citation of `sns-sub`.
func relatedHeadingCited(after string, sections []string) (string, bool) {
	text := strings.TrimSpace(strings.ReplaceAll(after, "`", ""))
	for _, heading := range sections {
		if !strings.HasPrefix(text, heading) {
			continue
		}
		rest := text[len(heading):]
		if rest == "" || !strings.ContainsAny(rest[:1], "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-") {
			return heading, true
		}
	}
	return "", false
}

// TestCitesRelatedContractCitedByHeading holds every mention of the related
// contract page in docs/resources/*.md to one shape: the file name, then `§`,
// then a heading the page carries. A mention that names a line number, or that
// names no heading at all, sends the reader to a place they have to search for.
//
// The `generatedFrom:` list in the YAML front matter is a list of source files,
// not a citation into a heading; it is the one exempt shape, the same exemption
// TestCitesSignalsCitationShape makes.
func TestCitesRelatedContractCitedByHeading(t *testing.T) {
	sections := relatedSections(t)

	paths, err := filepath.Glob(filepath.Join(projectRoot(t), "docs", "resources", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("docs/resources/*.md matched no files")
	}
	sort.Strings(paths)

	var byLine, byNothing, byUnknown []string
	for _, path := range paths {
		rel := relToRoot(t, path)
		for i, line := range readLines(t, path) {
			locs := reRelatedMention.FindAllStringIndex(line, -1)
			if locs == nil || reRelatedFrontEntry.MatchString(line) {
				continue
			}
			if reRelatedLineNumber.MatchString(line) {
				byLine = append(byLine, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				continue
			}
			for _, loc := range locs {
				after := strings.TrimLeft(line[loc[1]:], "` ")
				if !strings.HasPrefix(after, "§") {
					byNothing = append(byNothing, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
					continue
				}
				if _, ok := relatedHeadingCited(strings.TrimPrefix(after, "§"), sections); !ok {
					byUnknown = append(byUnknown, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				}
			}
		}
	}

	if len(byLine) > 0 {
		t.Errorf("%d citation(s) of docs/related-resources.md name a line number. The page is renumbered whenever a "+
			"pivot is added, so the number points at whatever moved into that line; cite the heading:\n%s",
			len(byLine), strings.Join(byLine, "\n"))
	}
	if len(byNothing) > 0 {
		t.Errorf("%d mention(s) of docs/related-resources.md name no section — `docs/related-resources.md § <heading>` "+
			"is the shape, with the heading spelled as the page spells it:\n%s",
			len(byNothing), strings.Join(byNothing, "\n"))
	}
	if len(byUnknown) > 0 {
		t.Errorf("%d citation(s) of docs/related-resources.md name a heading the page does not carry:\n%s",
			len(byUnknown), strings.Join(byUnknown, "\n"))
	}
}

// pivotNames is every resource type the catalog registers, which is also every
// heading name the related contract page can carry for a type.
func pivotNames(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, sn := range resource.AllShortNames() {
		names[sn] = true
	}
	if len(names) == 0 {
		t.Fatal("resource.AllShortNames() is empty; aws.Install() did not run")
	}
	return names
}

// A §2 block heading names its pivot and nothing else. The panel renders one
// row per registered pivot and has no row for a heading that annotates an
// absence ("`dbc` (intentionally absent)"), so a heading carrying prose is a
// block the reader cannot match to anything on screen.
var reRelatedPivotHeading = regexp.MustCompile("^### `([a-z0-9-]+)`$")

// docRelatedPivots reads the `### ` headings of a resource doc's §2 — the
// related-panel section — which is one block per pivot the doc promises. The
// second result is false for a doc with no §2 at all.
func docRelatedPivots(t *testing.T, path string) ([]string, map[string]int, []string, bool) {
	t.Helper()
	var order, offShape []string
	lines := map[string]int{}
	inSection2, sawHeading := false, false
	for i, line := range readLines(t, path) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inSection2 = strings.HasPrefix(strings.TrimPrefix(trimmed, "## "), "2.")
			continue
		}
		if !inSection2 || !strings.HasPrefix(trimmed, "### ") {
			continue
		}
		sawHeading = true
		m := reRelatedPivotHeading.FindStringSubmatch(trimmed)
		if m == nil {
			offShape = append(offShape, fmt.Sprintf("  %s:%d: %s", relToRoot(t, path), i+1, trimmed))
			continue
		}
		if _, seen := lines[m[1]]; !seen {
			order = append(order, m[1])
			lines[m[1]] = i + 1
		}
	}
	return order, lines, offShape, sawHeading
}

// TestCitesRelatedPanelSectionMatchesRegistry resolves each resource doc's §2
// against the pivots the catalog registers for that type. §2 is the reader's
// answer to "what can I pivot to from here", and the panel is built from the
// catalog's `Related` entries, so a block for a pivot nothing registers
// promises a row the panel never renders, and a registered pivot with no block
// is a row the reader meets with no explanation of what it is or how it was
// found.
//
// Docs for a type the catalog registers no pivots for are implementation plans
// rather than designs, and are skipped by that property.
func TestCitesRelatedPanelSectionMatchesRegistry(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(projectRoot(t), "docs", "resources", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("docs/resources/*.md matched no files")
	}
	sort.Strings(paths)

	walked, offending := 0, 0
	for _, path := range paths {
		registered := map[string]bool{}
		var want []string
		for _, def := range resource.GetRelated(shortNameOf(path)) {
			if !registered[def.TargetType] {
				registered[def.TargetType] = true
				want = append(want, def.TargetType)
			}
		}
		if len(want) == 0 {
			continue
		}
		documented, lines, offShape, ok := docRelatedPivots(t, path)
		if !ok {
			continue
		}
		walked++

		report := offShape
		for _, name := range documented {
			if !registered[name] {
				report = append(report, fmt.Sprintf("  %s:%d: §2 describes the pivot `%s`, which the catalog does "+
					"not register for this type", relToRoot(t, path), lines[name], name))
			}
		}
		sort.Strings(want)
		for _, name := range want {
			if _, ok := lines[name]; !ok {
				report = append(report, fmt.Sprintf("  %s: the catalog registers the pivot `%s` and §2 describes "+
					"no block for it", relToRoot(t, path), name))
			}
		}
		if len(report) == 0 {
			continue
		}
		offending++
		t.Errorf("%s §2 and the registered pivots disagree in %d place(s):\n%s",
			relToRoot(t, path), len(report), strings.Join(report, "\n"))
	}

	if walked == 0 {
		t.Fatal("docs/resources/*.md: no doc carried a §2 and a type with registered pivots")
	}
	if offending > 0 {
		t.Errorf("%d of %d resource docs describe a related panel the catalog does not build", offending, walked)
	}
}
