package unit_test

// qa_networking_row_values_and_doc_quotes_test.go — acceptance round 2, rulings J, K and L.
//
// J. A rendered row value is a word an operator reads, not a Go bool literal
//    and not an SDK enum spelling. A setting is enabled/disabled, a property
//    yes/no.
// K. sg.ingress.dangerous-ports must actually carry the Detail sentence its
//    doc row has been promising.
// L. A §4 Detail cell quotes what the code produces, whole and verbatim.
//    Quoting a prefix with an invented ending, or quoting a sentence for a
//    finding that renders none, both put words in the app's mouth.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	a9sruntime "github.com/k2m30/a9s/v3/core/runtime"
)

// netTypes is every registered type that ships a §4 table. The rulings below
// are not properties of five networking types; a value that reads as a Go bool
// or a doc that quotes a sentence the app never says is the same defect
// wherever it happens, and scoping the sweep to the batch that first hit it
// left every other type unwatched.
func netTypes(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, td := range resource.AllResourceTypes() {
		if _, err := os.Stat(netDocPath(td.ShortName)); err == nil {
			out = append(out, td.ShortName)
		}
	}
	sort.Strings(out)
	if len(out) < 5 {
		t.Fatalf("only %d registered types have a docs/resources page; the sweep would prove nothing", len(out))
	}
	return out
}

func netDocPath(shortName string) string {
	return "../../docs/resources/" + shortName + ".md"
}

// TestNetworkingDocs_EveryRegisteredTypeHasAPage names the types the sweep
// cannot see. A type with no §4 page is not covered by any assertion here, so
// the gap has to be visible rather than silently skipped.
func TestNetworkingDocs_EveryRegisteredTypeHasAPage(t *testing.T) {
	var missing []string
	for _, td := range resource.AllResourceTypes() {
		if _, err := os.Stat(netDocPath(td.ShortName)); err != nil {
			missing = append(missing, td.ShortName)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("%d registered type(s) have no docs/resources page, so nothing checks what they render: %v",
			len(missing), missing)
	}
}

// netRow is one AttentionDetail row as an operator sees it, plus enough
// context to name the literal that produced it.
type netRow struct {
	shortName string
	resID     string
	code      domain.FindingCode
	label     string
	value     string
}

// netBench walks the demo bench for one type and returns every rendered
// AttentionDetail row plus the Detail sentence of every finding, from both
// waves.
func netBench(t *testing.T, shortName string) (rows []netRow, details map[domain.FindingCode]string) {
	t.Helper()
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()

	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s: not registered", shortName)
	}
	fixtures := byType[shortName]
	if len(fixtures) == 0 {
		t.Fatalf("%s: no demo fixtures — this gate cannot see the type", shortName)
	}

	// The wave-2 result is folded onto each row before anything is read off
	// it, because the fold is what decides which findings survive and how
	// AttentionDetails is keyed. Reading the result map instead would assert
	// against a shape the renderer never sees.
	benchRows := append([]resource.Resource(nil), fixtures...)
	if enricher, ok := awsclient.Wave2EnricherFor(shortName); ok && enricher.Fn != nil {
		result, err := enricher.Fn(t.Context(), clients, fixtures, cache)
		if err != nil {
			t.Fatalf("%s: Wave-2 enricher returned error: %v", shortName, err)
		}
		for i := range benchRows {
			a9sruntime.ApplyWave2ToRow(&benchRows[i], *td, result.Findings, result.AttentionDetails)
		}
	}

	details = map[domain.FindingCode]string{}
	for _, res := range benchRows {
		for _, f := range res.Findings {
			if f.Detail != "" {
				details[f.Code] = f.Detail
			}
		}
		for code, d := range res.AttentionDetails {
			for _, r := range d.Rows {
				rows = append(rows, netRow{shortName, res.ID, code, r.Label, r.Value})
			}
		}
	}
	return rows, details
}

// ── J. Values are words ───────────────────────────────────────────────────

// netAsidePattern strips a trailing parenthesised aside, which by ruling is
// where a literal identifier may live ("disabled (BlockPublicAcls)").
var netAsidePattern = regexp.MustCompile(`\s*\([^()]*\)\s*$`)

// netAllCapsValue matches a value spelled like an SDK enum constant.
var netAllCapsValue = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// TestNetworkingRowValues_AreWordsNotLiterals sweeps every rendered row of the
// five networking types. A bool literal tells the operator the shape of the
// field in the SDK, not what is true of their account; an all-caps value is
// the enum spelling leaking through.
func TestNetworkingRowValues_AreWordsNotLiterals(t *testing.T) {
	var bad []string
	for _, short := range netTypes(t) {
		rows, _ := netBench(t, short)
		for _, r := range rows {
			v := strings.TrimSpace(netAsidePattern.ReplaceAllString(r.value, ""))
			switch {
			case v == "true" || v == "false":
				bad = append(bad, fmt.Sprintf("%s/%s %s: %q = %q is a Go bool literal; use enabled/disabled or yes/no",
					r.shortName, r.resID, r.code, r.label, r.value))
			case netAllCapsValue.MatchString(v) && len(v) > 1:
				bad = append(bad, fmt.Sprintf("%s/%s %s: %q = %q is an SDK enum spelling; use lowercase words",
					r.shortName, r.resID, r.code, r.label, r.value))
			}
		}
	}
	if len(bad) == 0 {
		return
	}
	sort.Strings(bad)
	t.Errorf("%d rendered row value(s) are not words:\n  %s", len(bad), strings.Join(bad, "\n  "))
}

// netFindRow returns the rendered row for (code, label) on any resource.
func netFindRow(t *testing.T, shortName string, code domain.FindingCode, label string) (netRow, bool) {
	t.Helper()
	rows, _ := netBench(t, shortName)
	for _, r := range rows {
		if r.code == code && r.label == label {
			return r, true
		}
	}
	return netRow{}, false
}

// TestSubnetAutoPublicIP_RowValueIsEnabled is the ruling's own worked example.
func TestSubnetAutoPublicIP_RowValueIsEnabled(t *testing.T) {
	r, ok := netFindRow(t, "subnet", "subnet.auto-public-ip", "Public address on launch")
	if !ok {
		t.Fatal(`no rendered row labelled "Public address on launch" for subnet.auto-public-ip`)
	}
	if r.value != "enabled" {
		t.Errorf("Public address on launch = %q, want %q", r.value, "enabled")
	}
}

// TestELBInvalidHeaders_RowValueIsDisabled pins the same rule where the type's
// own sibling rows (Deletion Protection, Access Logs) already say "disabled".
func TestELBInvalidHeaders_RowValueIsDisabled(t *testing.T) {
	r, ok := netFindRow(t, "elb", "elb.invalid-headers-kept", "Drop invalid headers")
	if !ok {
		t.Fatal(`no rendered row labelled "Drop invalid headers" for elb.invalid-headers-kept`)
	}
	if r.value != "disabled" {
		t.Errorf("Drop invalid headers = %q, want %q", r.value, "disabled")
	}
}

// ── K. sg.ingress.dangerous-ports carries a Detail ────────────────────────

// TestSGDangerousPorts_CarriesDetail pins that the finding renders the S5
// sentence its doc row promises. Asserted through the rendered finding rather
// than the constant so this test names no new production symbol; dev may call
// the constant sgDangerousPortsDetail.
func TestSGDangerousPorts_CarriesDetail(t *testing.T) {
	_, details := netBench(t, "sg")
	for _, code := range []domain.FindingCode{"sg.ingress.dangerous-ports", "sg.ingress.wide-open"} {
		if details[code] == "" {
			t.Errorf("%s renders no Detail sentence; the operator sees a phrase with nothing under it", code)
		}
	}
}

// ── L. Doc quotes equal the constants ─────────────────────────────────────

// netDocQuotePattern matches the backtick-quoted S5 cell at the end of a §4
// table row.
var netDocQuotePattern = regexp.MustCompile("\\|\\s*`([^`]+)`\\s*\\|?\\s*$")

// netDocQuotes returns every S5 quote in the type's §4 table.
func netDocQuotes(t *testing.T, shortName string) []string {
	t.Helper()
	path := netDocPath(shortName)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) < 7 {
			continue
		}
		if m := netDocQuotePattern.FindStringSubmatch(line); m != nil {
			out = append(out, strings.TrimSpace(m[1]))
		}
	}
	return out
}

// detailConstants returns every operator Detail sentence the code can render:
// the string constants named *Detail declared under core/aws. That is the
// oracle a §4 cell is checked against, rather than the demo bench, because a
// finding with no demo fixture still has its constant and a cell must not be
// judged by whether someone remembered to plant a witness.
func detailConstants(t *testing.T) map[string]bool {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "core", "aws")
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob core/aws: %v", err)
	}

	out := map[string]bool{}
	fset := token.NewFileSet()
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ValueSpec:
				// const/var xxxDetail = "..."
				for i, name := range node.Names {
					if !strings.HasSuffix(name.Name, "Detail") || i >= len(node.Values) {
						continue
					}
					if v, folded := foldStringExpr(node.Values[i]); folded && v != "" {
						out[v] = true
					}
				}
			case *ast.KeyValueExpr:
				// Detail: "..." or Detail: fmt.Sprintf("...", …)
				key, isIdent := node.Key.(*ast.Ident)
				if !isIdent || key.Name != "Detail" {
					return true
				}
				if v, folded := foldStringExpr(node.Value); folded && v != "" {
					out[v] = true
					return true
				}
				if call, isCall := node.Value.(*ast.CallExpr); isCall && len(call.Args) > 0 {
					if v, folded := foldStringExpr(call.Args[0]); folded && v != "" {
						out[v] = true
					}
				}
			}
			return true
		})
	}
	if len(out) == 0 {
		t.Fatal("no *Detail string constants found under core/aws; the oracle would pass everything")
	}
	return out
}

// foldStringExpr evaluates a string literal or a chain of them joined by +,
// which is how the longer Detail sentences are wrapped in source.
func foldStringExpr(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		if err != nil {
			return "", false
		}
		return s, true
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		l, lok := foldStringExpr(v.X)
		r, rok := foldStringExpr(v.Y)
		if !lok || !rok {
			return "", false
		}
		return l + r, true
	}
	return "", false
}

// docPlaceholder matches the two ways a sentence names a value it fills in at
// render time: a `<...>` placeholder in a doc cell, and a printf verb in the
// format string a finding builds its Detail from.
var docPlaceholder = regexp.MustCompile(`<[^>]*>|%[a-zA-Z]`)

// docQuoteMatches reports whether quote names constant, treating a value the
// finding substitutes as equal on both sides. Without that a template cell
// could never match the template it was copied from.
func docQuoteMatches(quote, constant string) bool {
	if quote == constant {
		return true
	}
	return docPlaceholder.ReplaceAllString(quote, "\x00") ==
		docPlaceholder.ReplaceAllString(constant, "\x00")
}

// docQuoteBurnDown is every type whose §4 cells still quote sentences the code
// cannot say, with the exact number of such cells today.
//
// One reason covers all of them: the pages were written before the Detail
// constants existed or drifted from them afterwards, so a cell describes what
// someone expected the app to say. Two shapes hide in that: a type with Detail
// constants whose cells no longer match them, and a type that emits no Detail
// at all (acm, alarm, cf, r53 and their kind), where the cells cannot be fixed
// by quoting — the sentence has to be written or the cell dropped. Task w27
// settles both by putting Detail on FindingDef and letting catalogen write the
// cells, and deletes this map.
//
// Exact counts, so the list can only shrink: a type that improves without its
// number moving fails here too.
var docQuoteBurnDown = map[string]int{
	"acm":          11,
	"alarm":        6,
	"apigw":        1,
	"asg":          11,
	"athena":       2,
	"backup":       2,
	"cb":           1,
	"cf":           6,
	"cfn":          7,
	"codeartifact": 3,
	"dbc":          9,
	"dbc-snap":     1,
	"dbi":          14,
	"dbi-snap":     1,
	"ddb":          4,
	"eb":           5,
	"eb-rule":      4,
	"ebs":          7,
	"ebs-snap":     8,
	"ec2":          9,
	"ecr":          3,
	"ecs-svc":      8,
	"ecs-task":     14,
	"efs":          6,
	"eks":          5,
	"glue":         4,
	"iam-group":    2,
	"iam-user":     5,
	"kinesis":      3,
	"kms":          9,
	"lambda":       3,
	"logs":         3,
	"lt":           1,
	"msk":          7,
	"nat":          3,
	"ng":           23,
	"opensearch":   5,
	"pipeline":     4,
	"policy":       3,
	"r53":          3,
	"redis":        10,
	"redshift":     12,
	"role":         5,
	"rtb":          2,
	"secrets":      7,
	"ses":          8,
	"sfn":          2,
	"sns":          3,
	"sqs":          5,
	"ssm":          3,
	"tg":           3,
	"trail":        4,
	"vpc":          1,
	"vpc-peer":     5,
	"waf":          2,
}

// TestNetworkingDocQuotes_EqualADetailConstant checks every §4 Detail cell on
// every resource page against the constants the code can actually render.
//
// A quote that matches nothing is either invented or has drifted from the
// constant it once copied; either way the doc is telling an operator the app
// says something it does not.
func TestNetworkingDocQuotes_EqualADetailConstant(t *testing.T) {
	constants := detailConstants(t)
	unmatched := map[string]int{}

	for _, short := range netTypes(t) {
		for _, quote := range netDocQuotes(t, short) {
			matched := false
			for constant := range constants {
				if docQuoteMatches(quote, constant) {
					matched = true
					break
				}
			}
			if matched {
				continue
			}
			unmatched[short]++
			if _, carried := docQuoteBurnDown[short]; !carried {
				t.Errorf("docs/resources/%s.md §4 quotes a Detail sentence no finding renders:\n  %q\n"+
					"quote the finding's Detail constant verbatim (a <placeholder> may stand for a value "+
					"the finding fills in), or drop the cell if the finding has none", short, quote)
			}
		}
	}

	for short, want := range docQuoteBurnDown {
		switch got := unmatched[short]; {
		case got == 0:
			t.Errorf("docs/resources/%s.md no longer quotes anything unrenderable — delete its docQuoteBurnDown entry", short)
		case got > want:
			t.Errorf("docs/resources/%s.md quotes %d unrenderable sentences, up from the %d carried; "+
				"the burn-down only shrinks", short, got, want)
		case got < want:
			t.Errorf("docs/resources/%s.md is down to %d unrenderable sentences from %d — lower its "+
				"docQuoteBurnDown entry to %d so the gate holds the ground you took", short, got, want, got)
		}
	}
}

// TestNetworkingDocs_NoDetailLengthCap pins the removal of the note capping
// Detail text at 100 characters: it contradicts quoting the constant whole,
// and a constant longer than the cap cannot satisfy both.
func TestNetworkingDocs_NoDetailLengthCap(t *testing.T) {
	for _, short := range netTypes(t) {
		path := netDocPath(short)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			low := strings.ToLower(line)
			if strings.Contains(low, "detail text") && strings.Contains(low, "100 char") {
				t.Errorf("docs/resources/%s.md:%d still caps Detail text at 100 characters, which no longer holds "+
					"now that the cell quotes the whole constant:\n  %s", short, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// TestNetworkingDocs_NoProseDescribingShippedCodeAsAGap pins that the docs
// stop describing an implemented check as something to consider implementing.
//
// sg.go's internet-facing predicate has checked `::/0` alongside `0.0.0.0/0`
// since it was written, while sg.md's §4.1 still recommends extending the
// check to IPv6 and §5 still records the gap. An operator reading the doc
// concludes their IPv6 exposure is unwatched and goes looking elsewhere, which
// is worse than the doc saying nothing at all.
func TestNetworkingDocs_NoProseDescribingShippedCodeAsAGap(t *testing.T) {
	b, err := os.ReadFile(netDocPath("sg"))
	if err != nil {
		t.Fatalf("read sg.md: %v", err)
	}
	for i, line := range strings.Split(string(b), "\n") {
		low := strings.ToLower(line)
		if !strings.Contains(low, "ipv6") {
			continue
		}
		switch {
		case strings.Contains(low, "recommend extending"):
			t.Errorf("docs/resources/sg.md:%d recommends extending the admin-port check to IPv6, "+
				"which core/aws/sg.go already does:\n  %s", i+1, strings.TrimSpace(line))
		case strings.Contains(low, "gap"):
			t.Errorf("docs/resources/sg.md:%d records the IPv6 check as a gap, which it is not:\n  %s",
				i+1, strings.TrimSpace(line))
		}
	}
}
