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
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// netTypes are the five types this batch owns.
var netTypes = []string{"sg", "subnet", "elb", "tgw", "vpce"}

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

	details = map[domain.FindingCode]string{}
	collect := func(resID string, fs []domain.Finding, ad map[domain.FindingCode]domain.AttentionDetail) {
		for _, f := range fs {
			if f.Detail != "" {
				details[f.Code] = f.Detail
			}
		}
		for code, d := range ad {
			for _, r := range d.Rows {
				rows = append(rows, netRow{shortName, resID, code, r.Label, r.Value})
			}
		}
	}

	for _, res := range fixtures {
		collect(res.ID, res.Findings, res.AttentionDetails)
	}

	if enricher, ok := awsclient.Wave2EnricherFor(shortName); ok && enricher.Fn != nil {
		result, err := enricher.Fn(t.Context(), clients, fixtures, cache)
		if err != nil {
			t.Fatalf("%s: Wave-2 enricher returned error: %v", shortName, err)
		}
		for resID, fs := range result.Findings {
			collect(resID, fs, result.AttentionDetails[resID])
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
	for _, short := range netTypes {
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
	path := "../../docs/resources/" + shortName + ".md"
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

// TestNetworkingDocQuotes_EqualTheRenderedDetail is the standing form of the
// prefix check. Both directions matter: a quote with no matching Detail is a
// sentence the app never says, and a Detail with no quote means the doc has
// stopped describing what ships.
func TestNetworkingDocQuotes_EqualTheRenderedDetail(t *testing.T) {
	for _, short := range netTypes {
		t.Run(short, func(t *testing.T) {
			_, details := netBench(t, short)

			shipped := map[string]domain.FindingCode{}
			for code, d := range details {
				shipped[d] = code
			}

			quoted := map[string]bool{}
			for _, q := range netDocQuotes(t, short) {
				quoted[q] = true
				if _, ok := shipped[q]; !ok {
					t.Errorf("§4 quotes a sentence the code never renders:\n  quoted: %q\n"+
						"  quote the finding's whole Detail constant verbatim, or drop the cell if the finding has none", q)
				}
			}
			for d, code := range shipped {
				if !quoted[d] {
					t.Errorf("%s renders a Detail sentence that no §4 row quotes:\n  %q", code, d)
				}
			}
		})
	}
}

// TestNetworkingDocs_NoDetailLengthCap pins the removal of the note capping
// Detail text at 100 characters: it contradicts quoting the constant whole,
// and a constant longer than the cap cannot satisfy both.
func TestNetworkingDocs_NoDetailLengthCap(t *testing.T) {
	for _, short := range netTypes {
		path := "../../docs/resources/" + short + ".md"
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
