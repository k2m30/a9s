package unit_test

// The per-type tests assert an enricher emits the right finding for the input
// it is given. These assert the demo account contains such an input: every
// posture code is carried by exactly one row after the real fetcher and
// enricher path.
//
// Exactly one matters in both directions. Zero means the finding is never
// visible in the demo. More than one means a fixture meant to be healthy for
// that code is not.

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
	{"dbi", "dbi.ca-cert-expiring-urgent"},
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

// w2BenchWitnessCount overrides the default of one carrier for a code a demo
// row can reach by more than one route. s3.public has two: AWS reporting the
// bucket policy public, and an access control list granting a public group.
// The bench shows one bucket for each, so an operator sees both phrasings of
// the same finding.
var w2BenchWitnessCount = map[string]int{
	"s3.public": 2,
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
			want, ok := w2BenchWitnessCount[w.code]
			if !ok {
				want = 1
			}
			if len(carriers) != want {
				t.Errorf("%s: %d demo rows carry %q (%v); want exactly %d witness(es)",
					w.shortName, len(carriers), w.code, carriers, want)
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

// TestW2DetailAttentionNeverRepeatsItself pins that a supporting row adds
// something its own finding's phrase does not already say. The row and the
// phrase render one line apart, so "Audit logging: off" under "audit logging
// off" prints one fact twice; the comparison is row against its own phrase.
func TestW2DetailAttentionNeverRepeatsItself(t *testing.T) {
	batchTypes := map[string]bool{
		"s3": true, "redis": true, "dbi": true, "dbc": true, "dbi-snap": true,
		"dbc-snap": true, "ddb": true, "opensearch": true, "redshift": true, "efs": true,
		"ebs": true,
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

// w2RowlessCodes are the findings whose phrase is the whole fact. A supporting
// row would restate it — "Encrypted transport: not required" under the phrase
// "HTTPS not enforced" is one sentence printed twice, a paraphrase rather than
// an exact repeat, which the normalized comparison above cannot catch, so this
// list is explicit.
//
// A code belongs here when an operator reading the phrase already knows
// everything the row would tell them. It does NOT belong here when the row
// carries a value the phrase lacks — the master username, the certificate
// expiry date, the require_ssl setting.
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
