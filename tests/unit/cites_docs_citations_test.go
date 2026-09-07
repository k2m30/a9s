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
	waveWord   string
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

	waveCol, stateCol, textCol := -1, -1, -1
	var rows []docSignalRow
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			waveCol, stateCol, textCol = -1, -1, -1
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
			line:       i + 1,
			stateWord:  cells[stateCol],
			listText:   strings.Trim(cells[textCol], "`"),
			rawLineTxt: trimmed,
		}
		if waveCol >= 0 && waveCol < len(cells) {
			row.waveWord = cells[waveCol]
		}
		rows = append(rows, row)
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

func catalogSignalsFor(t *testing.T, shortName string) []catalogSignal {
	t.Helper()
	signals := catalogSignalsByType(t)[shortName]
	if len(signals) == 0 {
		t.Fatalf("docs/attention-signals.md: generated block carries no rows for %q", shortName)
	}
	return signals
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

// assertDocTableMatchesCatalog pins that every list text in a resource doc's
// hand-written §4 table is a phrase the emitter actually produces, at the
// severity and in the wave the catalog gives it. A doc row that spells its own
// wording, bucket or wave is a second definition of the finding, and the reader
// cannot tell which one ships.
func assertDocTableMatchesCatalog(t *testing.T, shortName string) {
	t.Helper()
	path := filepath.Join(projectRoot(t), "docs", "resources", shortName+".md")
	rel := relToRoot(t, path)
	signals := catalogSignalsFor(t, shortName)

	known := make([]string, 0, len(signals))
	for _, sig := range signals {
		known = append(known, sig.phrase)
	}
	sort.Strings(known)

	for _, row := range parseDocSignalTable(t, path) {
		var matching []catalogSignal
		for _, sig := range signals {
			if sig.phrase == row.listText {
				matching = append(matching, sig)
			}
		}
		if len(matching) == 0 {
			t.Errorf("%s:%d: the table promises list text %q, which no %s finding produces. "+
				"Say the phrase the catalog carries, or defer to the generated table. Registered phrases: %q\n  %s",
				rel, row.line, row.listText, shortName, known, row.rawLineTxt)
			continue
		}

		if !anySignal(matching, func(sig catalogSignal) bool {
			return severityWordForBucket[sig.severity] == row.stateWord
		}) {
			t.Errorf("%s:%d: the table puts %q in the %s bucket; the catalog ships that phrase as %q\n  %s",
				rel, row.line, row.listText, row.stateWord, signalWords(matching, func(sig catalogSignal) string {
					return severityWordForBucket[sig.severity]
				}), row.rawLineTxt)
		}

		if row.waveWord != "" && !anySignal(matching, func(sig catalogSignal) bool {
			return sig.wave == "wave"+row.waveWord
		}) {
			t.Errorf("%s:%d: the table calls %q a wave-%s signal; the catalog ships it as %q\n  %s",
				rel, row.line, row.listText, row.waveWord, signalWords(matching, func(sig catalogSignal) string {
					return sig.wave
				}), row.rawLineTxt)
		}
	}

	assertDocSignalSectionsAgree(t, path)
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

// assertDocSignalSectionsAgree pins a resource doc's §3 signal list against its
// own §4 table. §4 says every signal from §3 lands on a surface, so the two
// describe the same set: a §3 bullet with no §4 row is a signal the doc claims
// and never renders, and the §4 table is already pinned to the catalog, so a §3
// list that disagrees with it disagrees with what ships.
func assertDocSignalSectionsAgree(t *testing.T, path string) {
	t.Helper()
	lines := readLines(t, path)

	inSection3 := false
	sectionBuckets := map[string]int{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := reMarkdownHeading.FindStringSubmatch(trimmed); m != nil {
			inSection3 = strings.HasPrefix(m[1], "3.") || strings.HasPrefix(m[1], "3 ")
		}
		if !inSection3 {
			continue
		}
		if m := reDocSignalBucket.FindStringSubmatch(trimmed); m != nil {
			sectionBuckets[m[1]]++
		}
	}
	if len(sectionBuckets) == 0 {
		t.Fatalf("%s: §3 has no `**State bucket**:` bullets to compare against the §4 table", relToRoot(t, path))
	}

	tableBuckets := map[string]int{}
	for _, row := range parseDocSignalTable(t, path) {
		tableBuckets[row.stateWord]++
	}

	if !reflect.DeepEqual(sectionBuckets, tableBuckets) {
		t.Errorf("%s: §3 describes signals in the buckets %s while the §4 table renders %s. "+
			"§4 says every signal from §3 lands on a surface, so a bucket in one and not the other is a signal "+
			"the doc either never renders or renders without describing", relToRoot(t, path),
			formatBucketCounts(sectionBuckets), formatBucketCounts(tableBuckets))
	}
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

// TestCitesCodeartifactSignalTableMatchesTheCatalog pins the codeartifact
// hand-written §4 table the same way: it puts the public-access finding on a
// Healthy row while the catalog ships it Broken, and promises an unused-registry
// wording nothing emits.
func TestCitesCodeartifactSignalTableMatchesTheCatalog(t *testing.T) {
	assertDocTableMatchesCatalog(t, "codeartifact")
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
