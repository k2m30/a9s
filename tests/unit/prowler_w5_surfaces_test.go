package unit_test

// prowler_w5_surfaces_test.go — the rendered-surface half of batch w5.
//
// The per-row tests assert an enricher emits the right finding for the input
// it is handed. Nothing there proves the demo account contains such an
// input, that the row an operator reads says anything the phrase did not
// already say, or that the colour and the status cell agree about which
// finding is worst. Those are the checks here, and they are the ones a
// screenshot of the signal actually depends on.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w5BatchTypes are the eight short names batch w5 touches.
var w5BatchTypes = []string{"sqs", "sns", "sns-sub", "msk", "kinesis", "sfn", "ses", "eb"} //nolint:gochecknoglobals // test-only list

// w5BenchWitnesses names each new finding code and the demo fixture constant
// that names the ONE row expected to carry it. Referencing the constant
// rather than repeating its value is what makes the constant part of the
// contract: renaming the fixture row without updating the witness breaks the
// build instead of quietly emptying the bench.
var w5BenchWitnesses = []struct { //nolint:gochecknoglobals // test-only table
	shortName string
	code      string
	witness   string
}{
	{"sqs", "sqs.public-policy", fixtures.SQSPublicPolicy},

	{"sns", "sns.public-policy", fixtures.SNSPublicPolicy},
	{"sns", "sns.no-kms", fixtures.SNSNoKMS},

	{"sns-sub", "sns-sub.plain-http", fixtures.SNSSubPlainHTTP},

	{"msk", "msk.public-access", fixtures.MSKPublic},
	{"msk", "msk.unauthenticated", fixtures.MSKUnauthenticated},

	{"kinesis", "kinesis.unencrypted", fixtures.KinesisUnencrypted},
	{"kinesis", "kinesis.min-retention", fixtures.KinesisMinRetention},

	{"sfn", "sfn.logging-off", fixtures.SFNLoggingOff},
	{"sfn", "sfn.no-cmk", fixtures.SFNNoCMK},
	{"sfn", "sfn.definition-secret", fixtures.SFNDefinitionSecret},

	{"ses", "ses.dkim-off", fixtures.SESDKIMOff},

	{"eb", "eb.managed-updates-off", fixtures.EBManagedUpdatesOff},
	{"eb", "eb.enhanced-health-off", fixtures.EBEnhancedHealthOff},
	{"eb", "eb.cloudwatch-logs-off", fixtures.EBCWLogsOff},
}

// w5Bench fetches and enriches the demo rows for every type in the batch,
// the way the running app does.
func w5Bench(t *testing.T) map[string][]resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	inBatch := map[string]bool{}
	for _, s := range w5BatchTypes {
		inBatch[s] = true
	}

	out := map[string][]resource.Resource{}
	for _, td := range resource.AllResourceTypes() {
		if !inBatch[td.ShortName] {
			continue
		}
		out[td.ShortName] = mergeWave2Findings(t, td, byType[td.ShortName], cache, clients)
	}
	return out
}

// Exactly one demo row carries each new code, and it is the row the witness
// constant names. Zero means the signal is invisible in the demo account and
// every screenshot of it is empty; more than one means a row that was meant
// to be healthy for this condition is not, which turns the bench into noise.
func TestW5DemoBenchWitnessPerFinding(t *testing.T) {
	bench := w5Bench(t)

	for _, w := range w5BenchWitnesses {
		t.Run(w.shortName+"/"+w.code, func(t *testing.T) {
			rows := bench[w.shortName]
			if len(rows) == 0 {
				t.Fatalf("no demo rows for type %q", w.shortName)
			}
			var carriers []string
			for _, r := range rows {
				for _, f := range r.Findings {
					if string(f.Code) == w.code {
						carriers = append(carriers, r.ID)
						break
					}
				}
			}
			sort.Strings(carriers)
			if len(carriers) != 1 {
				t.Fatalf("%d demo rows carry %q (%v); want exactly the one witness %q",
					len(carriers), w.code, carriers, w.witness)
			}
			if carriers[0] != w.witness {
				t.Errorf("%q is carried by %q, but the fixture contract names %q as its witness",
					w.code, carriers[0], w.witness)
			}
		})
	}
}

// w5Normalize strips punctuation and case so "Logging level: off" and the
// phrase "execution logging off" can be compared as words.
func w5Normalize(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ':', '.', ',':
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// U11, per finding: a supporting row must add something its OWN phrase does
// not already say. The row and the phrase are rendered one line apart, so a
// row that normalises to the phrase is the detail block printing one fact
// twice. Only this comparison catches it — the per-row tests assert a row
// exists, never that it is worth reading.
func TestW5DetailAttentionNeverRepeatsItself(t *testing.T) {
	for shortName, rows := range w5Bench(t) {
		for _, res := range rows {
			for _, f := range res.Findings {
				ad, ok := res.AttentionDetails[f.Code]
				if !ok {
					continue
				}
				phrase := w5Normalize(f.Phrase)
				for _, row := range ad.Rows {
					switch {
					case w5Normalize(row.Label+" "+row.Value) == phrase,
						w5Normalize(row.Value) == phrase:
						t.Errorf("%s/%s %s: row %q: %q adds nothing to the phrase %q",
							shortName, res.ID, f.Code, row.Label, row.Value, f.Phrase)
					}
				}
			}
		}
	}
}

// Values are words. Neither the SDK's enum casing nor Go's bool literal is
// something an operator says, and both reach a rendered row only by handing
// `string(<SDK enum>)` or a %v straight to the value.
//
// A value that is an identifier, a number, a date, or a parenthesised aside
// naming the setting to change is not vocabulary and is left alone.
func TestW5RowValuesAreWordsNotEnumsOrBools(t *testing.T) {
	for shortName, rows := range w5Bench(t) {
		for _, res := range rows {
			for code, ad := range res.AttentionDetails {
				for _, row := range ad.Rows {
					v := strings.TrimSpace(row.Value)
					if v == "" || strings.HasSuffix(v, ")") {
						continue
					}
					if _, err := strconv.Atoi(v); err == nil {
						continue
					}
					switch {
					case v == "true" || v == "false":
						t.Errorf("%s/%s %s row %q = %q: a Go bool literal is not a word an operator uses",
							shortName, res.ID, code, row.Label, row.Value)
					case v != strings.ToLower(v) && v == strings.ToUpper(v):
						t.Errorf("%s/%s %s row %q = %q: SDK enum casing is not a word an operator uses",
							shortName, res.ID, code, row.Label, row.Value)
					}
				}
			}
		}
	}
}

// A row's colour and its Status cell must select the SAME finding. Two
// selectors over one finding set is the shape where a row shows red while
// the cell explains the warning, and vice versa.
func TestW5ColourAndStatusPhraseSelectTheSameFinding(t *testing.T) {
	for shortName, rows := range w5Bench(t) {
		td := resource.FindResourceType(shortName)
		if td == nil {
			t.Fatalf("%s not registered", shortName)
		}
		for _, r := range rows {
			if len(r.Findings) == 0 {
				continue
			}
			top, ok := domain.TopFinding(r.Findings)
			if !ok {
				continue
			}
			if got, want := td.ResolveColor(r), w5ColorOfSeverity(top.Severity); got != want {
				t.Errorf("%s row %q: colour %v, but the top finding %q is %v — the colour selector and the status cell disagree about which finding wins",
					shortName, r.ID, got, top.Code, want)
			}
			// StatusPhrase appends " (+N)" when other issues are present.
			// The pin is which finding it named, not the counter.
			phrase, _, _ := strings.Cut(domain.StatusPhrase(r.Findings), " (+")
			if phrase != top.Phrase {
				t.Errorf("%s row %q: status cell names %q, but the colour's top finding is %q",
					shortName, r.ID, phrase, top.Phrase)
			}
		}
	}
}

func w5ColorOfSeverity(s domain.Severity) resource.Color {
	switch s {
	case domain.SevBroken:
		return resource.ColorBroken
	case domain.SevWarn:
		return resource.ColorWarning
	case domain.SevDim:
		return resource.ColorDim
	default:
		return resource.ColorHealthy
	}
}

// A classifier that returned the first finding instead of the worst would
// read this row as a warning. Order is deliberate: the milder finding comes
// first, so first-wins and worst-wins give different answers. Every type in
// the batch is checked, not a sample — the ruling that the seven raw-field
// classifiers convert is only closed when each one is pinned.
func TestW5WarnThenBrokenReadsAsBroken(t *testing.T) {
	for _, short := range w5BatchTypes {
		t.Run(short, func(t *testing.T) {
			td := resource.FindResourceType(short)
			if td == nil {
				t.Fatalf("%s not registered", short)
			}
			r := resource.Resource{
				ID: short + "-probe",
				Findings: []domain.Finding{
					{Code: domain.FindingCode(short + ".probe.warn"), Phrase: "warn", Severity: domain.SevWarn, Source: "wave1"},
					{Code: domain.FindingCode(short + ".probe.broken"), Phrase: "broken", Severity: domain.SevBroken, Source: "wave1"},
				},
			}
			if got := td.ResolveColor(r); got != resource.ColorBroken {
				t.Errorf("ResolveColor with a warn and a broken finding = %v, want ColorBroken", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Row 11 — the ses Type column
// ---------------------------------------------------------------------------

// The Type column renders Fields["identity_type"] verbatim, so whatever the
// fetcher stores there is what an operator reads. DOMAIN and EMAIL_ADDRESS
// are SDK enum constants; the column shows words.
func TestW5SESTypeColumnRendersWordsNotEnums(t *testing.T) {
	rows := w5Bench(t)["ses"]
	if len(rows) == 0 {
		t.Fatal("no demo ses rows")
	}
	allowed := map[string]bool{"domain": true, "email address": true}
	seen := map[string]bool{}
	for _, r := range rows {
		got := r.Fields["identity_type"]
		seen[got] = true
		if !allowed[got] {
			t.Errorf("ses row %q Type cell = %q; the column shows %q or %q, never an SDK enum constant",
				r.ID, got, "domain", "email address")
		}
	}
	// A bench where every identity is a domain would pass the loop above
	// without ever rendering the second word.
	if !seen["domain"] || !seen["email address"] {
		t.Errorf("the demo ses rows render %v; both Type values must appear or one of them is never exercised", seen)
	}
}

// The rename above is a class change, not a cell change: ses_related.go
// branches on the same field to decide whether an identity is a domain or a
// single address. A comparison left against the old enum text keeps
// compiling and silently stops matching, which is how the related panel
// would go empty without a single test failing.
//
// Scanning string literals rather than the file text is deliberate: the
// prose in those files legitimately names the enum when explaining the AWS
// API, and a grep would fire on every comment.
func TestW5NoProductionComparisonAgainstTheOldIdentityTypeEnum(t *testing.T) {
	files, err := filepath.Glob("../../core/aws/ses*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no core/aws/ses*.go files found; the scan would assert nothing")
	}

	stale := map[string]bool{"EMAIL_ADDRESS": true, "DOMAIN": true}
	fset := token.NewFileSet()
	found := false
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, err := strconv.Unquote(lit.Value)
			if err != nil || !stale[v] {
				return true
			}
			found = true
			t.Errorf("%s: string literal %q — identity_type now carries the rendered word, so this comparison never matches; use the sesv2types enum against the SDK value, or the rendered word against the field",
				fset.Position(lit.Pos()), v)
			return true
		})
	}
	_ = found
}

// w5EBStatusWitnesses names each Elastic Beanstalk configuration finding and
// the environment whose Status cell must carry its phrase.
var w5EBStatusWitnesses = map[string]string{ //nolint:gochecknoglobals // test-only table
	"managed platform updates off":    "eb.managed-updates-off",
	"enhanced health reporting off":   "eb.enhanced-health-off",
	"log streaming to CloudWatch off": "eb.cloudwatch-logs-off",
}

// A finding an operator never reads is not a finding. Each of the three
// Elastic Beanstalk configuration phrases has to reach a Status cell, which
// means its witness must not be sharing that cell with an equal-severity
// health finding — StatusPhrase breaks a severity tie by order, so the
// configuration phrase loses silently and the row reads about something
// else.
func TestW5EBConfigurationPhrasesReachTheStatusCell(t *testing.T) {
	rows := w5Bench(t)["eb"]
	if len(rows) == 0 {
		t.Fatal("no demo eb rows")
	}

	seen := map[string]string{}
	for _, r := range rows {
		phrase, _, _ := strings.Cut(domain.StatusPhrase(r.Findings), " (+")
		if _, wanted := w5EBStatusWitnesses[phrase]; wanted {
			seen[phrase] = r.ID
		}
	}

	for phrase, code := range w5EBStatusWitnesses {
		if seen[phrase] == "" {
			t.Errorf("no demo environment's Status cell reads %q; %s exists but never reaches the column an operator reads", phrase, code)
		}
	}
}
