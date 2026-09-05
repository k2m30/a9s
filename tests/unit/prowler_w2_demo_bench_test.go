package unit_test

// prowler_w2_demo_bench_test.go — the demo-bench half of the w2 Prowler batch.
//
// The per-type tests assert an enricher emits the right finding for the input
// it is given. This one asserts the demo account actually contains such an
// input: every new code is carried by exactly one row after the real fetcher
// and enricher path, so the finding is visible when someone opens the app and
// no other row is quietly tripping the same condition.
//
// Exactly one matters in both directions. Zero means the witness fixture never
// reaches the condition and every screenshot of this signal is empty. More
// than one means a fixture that was supposed to be healthy for this row is not,
// which is how a demo bench stops being a bench and starts being noise.

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w2BenchWitness names one finding code and the demo row that must carry it.
var w2BenchWitnesses = []struct {
	shortName string
	code      string
}{
	{"s3", "s3.public"},
	{"s3", "s3.versioning-off"},
	{"s3", "s3.mfa-delete-off"},
	{"s3", "s3.access-logging-off"},
	{"s3", "s3.no-lifecycle"},
	{"s3", "s3.no-object-lock"},

	{"redis", "redis.encryption-at-rest-off"},
	{"redis", "redis.encryption-in-transit-off"},
	{"redis", "redis.no-auth"},
	{"redis", "redis.no-backup"},

	{"dbi", "dbi.single-az"},
	{"dbi", "dbi.minor-upgrade-off"},
	{"dbi", "dbi.iam-auth-off"},
	{"dbi", "dbi.default-master-user"},
	{"dbi", "dbi.ca-cert-expiring"},
	{"dbi", "dbi.engine-deprecated"},

	{"dbc", "dbc.single-az"},
	{"dbc", "dbc.minor-upgrade-off"},
	{"dbc", "dbc.iam-auth-off"},
	{"dbc", "dbc.default-master-user"},

	{"dbi-snap", "dbi-snap.public"},
	{"dbc-snap", "dbc-snap.public"},

	{"ddb", "ddb.deletion-protection-off"},
	{"ddb", "ddb.cross-account-policy"},
	{"ddb", "ddb.public-policy"},

	{"opensearch", "opensearch.public"},
	{"opensearch", "opensearch.https-not-enforced"},
	{"opensearch", "opensearch.node-to-node-tls-off"},

	{"redshift", "redshift.audit-logging-off"},
	{"redshift", "redshift.require-ssl-off"},

	{"efs", "efs.unencrypted"},
	{"efs", "efs.public-policy"},
	{"efs", "efs.no-backup-policy"},
}

func TestW2DemoBenchOneWitnessPerFinding(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	merged := make(map[string][]resource.Resource)
	for _, td := range resource.AllResourceTypes() {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}
		merged[td.ShortName] = mergeWave2Findings(t, td, fixtures, cache, clients)
	}

	for _, w := range w2BenchWitnesses {
		t.Run(w.shortName+"/"+w.code, func(t *testing.T) {
			rows := merged[w.shortName]
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
				t.Errorf("%s: %d demo rows carry %q (%v); want exactly 1 witness",
					w.shortName, len(carriers), w.code, carriers)
			}
		})
	}
}

// normalizeRowText strips punctuation and case so "Audit logging: off" and the
// phrase "audit logging off" compare equal.
func normalizeRowText(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ':', '.', ',':
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// TestW2DetailAttentionNeverRepeatsItself pins U11 per finding: a supporting
// row must add something its OWN finding's phrase does not already say.
//
// Comparing rows only against each other, as this test first did, misses the
// case that actually shipped — one row under one phrase, saying the same
// thing. The row and the phrase are rendered one line apart, so "Audit
// logging: off" under "audit logging off" is the detail block printing one
// fact twice. The comparison that catches it is row against its own phrase.
func TestW2DetailAttentionNeverRepeatsItself(t *testing.T) {
	batchTypes := map[string]bool{
		"s3": true, "redis": true, "dbi": true, "dbc": true, "dbi-snap": true,
		"dbc-snap": true, "ddb": true, "opensearch": true, "redshift": true, "efs": true,
	}

	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, td := range resource.AllResourceTypes() {
		if !batchTypes[td.ShortName] {
			continue
		}
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}
		for _, res := range mergeWave2Findings(t, td, fixtures, cache, clients) {
			for _, f := range res.Findings {
				ad, ok := res.AttentionDetails[f.Code]
				if !ok {
					continue
				}
				phrase := normalizeRowText(f.Phrase)
				for _, row := range ad.Rows {
					switch {
					case normalizeRowText(row.Label+" "+row.Value) == phrase,
						normalizeRowText(row.Value) == phrase:
						t.Errorf("%s/%s %s: row %q: %q adds nothing to the phrase %q",
							td.ShortName, res.ID, f.Code, row.Label, row.Value, f.Phrase)
					}
				}
			}
		}
	}
}

// w2RowlessCodes are the findings whose phrase is the whole fact. Each once
// carried a supporting row that restated it — "Encrypted transport: not
// required" under the phrase "HTTPS not enforced" is one sentence printed
// twice, a paraphrase rather than an exact repeat, which is why the
// normalized comparison above cannot catch it and this list is explicit.
//
// A code belongs here when an operator reading the phrase already knows
// everything the row would tell them. It does NOT belong here when the row
// carries a value the phrase lacks — the master username, the certificate
// expiry date, the require_ssl setting — so those keep their rows.
var w2RowlessCodes = map[string]string{
	"redis.encryption-at-rest-off":    "phrase already says encryption at rest is off",
	"redis.encryption-in-transit-off": "phrase already says encryption in transit is off",
	"s3.mfa-delete-off":               "phrase already says MFA delete is off",
	"s3.access-logging-off":           "phrase already says access logging is off",
	"s3.no-object-lock":               "phrase already says object lock is off",
	"opensearch.https-not-enforced":   "phrase already says HTTPS is not enforced",
	"opensearch.node-to-node-tls-off": "phrase already says node-to-node encryption is off",
	"ddb.deletion-protection-off":     "phrase already says deletion protection is off",
	"efs.no-backup-policy":            "phrase already says automatic backups are off",
	"redshift.audit-logging-off":      "phrase already says audit logging is off",
}

func TestW2RowlessFindingsCarryNoSupportingRow(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, td := range resource.AllResourceTypes() {
		for _, res := range mergeWave2Findings(t, td, byType[td.ShortName], cache, clients) {
			for code, ad := range res.AttentionDetails {
				why, rowless := w2RowlessCodes[string(code)]
				if !rowless || len(ad.Rows) == 0 {
					continue
				}
				t.Errorf("%s/%s %s carries rows %v; %s", td.ShortName, res.ID, code, ad.Rows, why)
			}
		}
	}
}

// w2RowValueWords pins the vocabulary of a rendered row value. A setting reads
// on or off; a property reads yes or no. Neither the SDK's enum casing
// ("DISABLED") nor Go's bool literal ("false") is a word an operator uses, and
// both reach a row only by handing `string(<SDK enum>)` or a %v straight to
// the value — so they are pinned out by shape here rather than site by site.
//
// A value that is an identifier, a number, a date or a parenthesised aside
// naming the setting to change is not vocabulary and is left alone.
var w2RowValueWords = map[string]string{
	"dbi.single-az":            "on/off or yes/no, not a bool literal",
	"dbc.single-az":            "on/off or yes/no, not a bool literal",
	"dbi.minor-upgrade-off":    "on/off or yes/no, not a bool literal",
	"dbc.minor-upgrade-off":    "on/off or yes/no, not a bool literal",
	"dbi.iam-auth-off":         "on/off or yes/no, not a bool literal",
	"dbc.iam-auth-off":         "on/off or yes/no, not a bool literal",
	"efs.no-backup-policy":     "on/off or yes/no, not an SDK enum",
	"redshift.require-ssl-off": "on/off or yes/no, not a bool literal",
}

func TestW2RowValuesAreWordsNotEnumsOrBools(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, td := range resource.AllResourceTypes() {
		for _, res := range mergeWave2Findings(t, td, byType[td.ShortName], cache, clients) {
			for code, ad := range res.AttentionDetails {
				want, pinned := w2RowValueWords[string(code)]
				if !pinned {
					continue
				}
				for _, row := range ad.Rows {
					// A value naming the setting an operator changes, as a
					// parenthesised aside, is quoting that setting's literal
					// value — "false (require_ssl)" is what the console shows
					// and what they edit. Rewording it to "off" would
					// misreport the parameter, so the aside form is exempt
					// from the vocabulary rule rather than stripped and then
					// judged on what is left.
					if regexp.MustCompile(`\([^()]+\)$`).MatchString(strings.TrimSpace(row.Value)) {
						continue
					}
					v := strings.TrimSpace(row.Value)
					switch {
					case v == "true" || v == "false":
						t.Errorf("%s/%s %s row %q = %q: a Go bool literal is not a word; want %s",
							td.ShortName, res.ID, code, row.Label, row.Value, want)
					case v != "" && v == strings.ToUpper(v) && v != strings.ToLower(v):
						t.Errorf("%s/%s %s row %q = %q: SDK enum casing is not a word; want %s",
							td.ShortName, res.ID, code, row.Label, row.Value, want)
					}
				}
			}
		}
	}
}
