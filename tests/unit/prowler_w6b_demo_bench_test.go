package unit_test

// prowler_w6b_demo_bench_test.go — the demo-bench half of batch w6b.
//
// The per-row tests prove a fetcher or enricher emits the right finding for
// the input it is handed. These prove the demo account actually contains such
// an input: every code this batch registers is carried by exactly one row
// after the real fetcher and enricher path, so the signal is visible when
// someone opens the app, and no row that was meant to be healthy is quietly
// tripping the same condition.

import (
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w6bBenchWitnesses names each code with the ONE demo row that must carry it.
// The witness value comes from the fixture constant rather than a literal, so
// renaming the demo resource moves this pin with it instead of silently
// leaving the bench pointing at a row that no longer exists.
var w6bBenchWitnesses = []struct {
	shortName string
	code      string
	witness   string
}{
	{"cfn", "cfn.termination-protection-off", fixtures.CFNTerminationProtectionOff},
	{"cfn", "cfn.output-secret", fixtures.CFNOutputSecret},

	{"cb", "cb.public-builds", fixtures.CBPublicBuilds},
	{"cb", "cb.buildspec-from-source", fixtures.CBBuildspecFromSource},
	{"cb", "cb.source-url-credential", fixtures.CBSourceURLCredential},
	{"cb", "cb.env-secret", fixtures.CBEnvSecret},

	{"ecr", "ecr.public-policy", fixtures.ECRPublicPolicy},
	{"ecr", "ecr.scan-on-push-off", fixtures.ECRScanOnPushOff},
	{"ecr", "ecr.mutable-tags", fixtures.ECRMutableTags},
	{"ecr", "ecr.no-lifecycle-policy", fixtures.ECRNoLifecycle},

	{"glue", "glue.no-security-configuration", fixtures.GlueNoSecurityConfig},
	{"glue", "glue.continuous-logging-off", fixtures.GlueLoggingOff},
	{"glue", "glue.argument-secret", fixtures.GlueArgumentSecret},

	{"eks", "eks.public-endpoint", fixtures.EKSPublicEndpoint},
	{"eks", "eks.control-plane-logging-off", fixtures.EKSLoggingIncomplete},
	{"eks", "eks.secrets-not-kms", fixtures.EKSSecretsNoKMS},
	{"eks", "eks.version-unsupported", fixtures.EKSVersionUnsupported},
}

// w6bBatchTypes is the set this batch owns, shared by every sweep below.
var w6bBatchTypes = map[string]bool{
	"cfn": true, "cb": true, "ecr": true, "codeartifact": true, "glue": true, "eks": true,
}

// w6bMergedDemoRows folds every batch type's demo rows the way the app folds
// them, so a gate reading them sees the same detail block a user does.
func w6bMergedDemoRows(t *testing.T) map[string][]resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	merged := make(map[string][]resource.Resource)
	for _, td := range resource.AllResourceTypes() {
		if !w6bBatchTypes[td.ShortName] {
			continue
		}
		if len(byType[td.ShortName]) == 0 {
			continue
		}
		merged[td.ShortName] = mergeWave2Findings(t, td, byType[td.ShortName], cache, clients)
	}
	return merged
}

// Exactly one matters in both directions. Zero means the witness fixture never
// reaches the condition and every screenshot of the signal is empty. More than
// one means a row that was supposed to be healthy is not, which is how a demo
// bench stops being a bench and becomes noise.
func TestW6BDemoBenchOneWitnessPerFinding(t *testing.T) {
	merged := w6bMergedDemoRows(t)

	for _, w := range w6bBenchWitnesses {
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
				t.Fatalf("%d demo rows carry %q (%v); want exactly 1", len(carriers), w.code, carriers)
			}
			if carriers[0] != w.witness {
				t.Errorf("%q is carried by %q; the witness constant names %q",
					w.code, carriers[0], w.witness)
			}
		})
	}
}

// w6bNormalizeRowText strips punctuation and case so "Security configuration:
// none" and the phrase "no security configuration" can be compared as words.
func w6bNormalizeRowText(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ':', '.', ',':
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// TestW6BDetailAttentionNeverRepeatsItself pins U11 for this batch: a
// supporting row must add something its OWN finding's phrase does not already
// say. The row and the phrase render one line apart, so a row that normalises
// to its phrase is the detail block printing one fact twice.
func TestW6BDetailAttentionNeverRepeatsItself(t *testing.T) {
	for shortName, rows := range w6bMergedDemoRows(t) {
		for _, res := range rows {
			for _, f := range res.Findings {
				ad, ok := res.AttentionDetails[f.Code]
				if !ok {
					continue
				}
				phrase := w6bNormalizeRowText(f.Phrase)
				for _, row := range ad.Rows {
					switch {
					case w6bNormalizeRowText(row.Label+" "+row.Value) == phrase,
						w6bNormalizeRowText(row.Value) == phrase:
						t.Errorf("%s/%s %s: row %q: %q adds nothing to the phrase %q",
							shortName, res.ID, f.Code, row.Label, row.Value, f.Phrase)
					}
				}
			}
		}
	}
}

// w6bRowlessCodes are the findings whose phrase is the whole fact. Each one
// was proposed in the batch table with a supporting row that restates it —
// "Termination protection: off" under "termination protection off" — and the
// paraphrases among them ("Tag mutability: mutable" under "tags are mutable")
// slip past the normalised comparison above, which is why this list is
// explicit rather than derived.
//
// A code belongs here when an operator reading the phrase already knows
// everything the row would tell them. It does NOT belong here when the row
// carries a value the phrase lacks — the CIDR ranges, the buildspec path, the
// disabled log types, the argument name, the key that held the credential — so
// those keep their rows.
var w6bRowlessCodes = map[string]string{
	"cfn.termination-protection-off": "phrase already says termination protection is off",
	"cb.public-builds":               "phrase already says the build results are publicly visible",
	"ecr.scan-on-push-off":           "phrase already says scan on push is off",
	"ecr.mutable-tags":               "phrase already says the tags are mutable",
	"ecr.no-lifecycle-policy":        "phrase already says there is no lifecycle policy",
	"glue.no-security-configuration": "phrase already says there is no security configuration",
	"eks.secrets-not-kms":            "phrase already says secrets are not encrypted with KMS",
}

func TestW6BRowlessFindingsCarryNoSupportingRow(t *testing.T) {
	for shortName, rows := range w6bMergedDemoRows(t) {
		for _, res := range rows {
			for code, ad := range res.AttentionDetails {
				why, rowless := w6bRowlessCodes[string(code)]
				if !rowless || len(ad.Rows) == 0 {
					continue
				}
				t.Errorf("%s/%s %s carries rows %v; %s", shortName, res.ID, code, ad.Rows, why)
			}
		}
	}
}

// TestW6BRowValuesAreWordsNotEnumsOrBools pins the vocabulary of every
// rendered row this batch adds. The SDK's enum casing ("PUBLIC_READ",
// "MUTABLE") and Go's bool literals reach a row only by handing
// string(<SDK enum>) or a %v straight to the value, so they are pinned out by
// shape here rather than site by site.
//
// A value that is an identifier, a number, a range, a path or a parenthesised
// aside naming the setting to change is not vocabulary and is left alone.
func TestW6BRowValuesAreWordsNotEnumsOrBools(t *testing.T) {
	batchCodePrefixes := []string{"cfn.", "cb.", "ecr.", "codeartifact.", "glue.", "eks."}

	for shortName, rows := range w6bMergedDemoRows(t) {
		for _, res := range rows {
			for code, ad := range res.AttentionDetails {
				inBatch := false
				for _, p := range batchCodePrefixes {
					if strings.HasPrefix(string(code), p) {
						inBatch = true
						break
					}
				}
				if !inBatch {
					continue
				}
				for _, row := range ad.Rows {
					v := strings.TrimSpace(row.Value)
					switch {
					case v == "true" || v == "false":
						t.Errorf("%s/%s %s row %q = %q: a Go bool literal is not a word an operator uses",
							shortName, res.ID, code, row.Label, row.Value)
					case v != "" && strings.ContainsRune(v, '_') && v == strings.ToUpper(v) && v != strings.ToLower(v):
						t.Errorf("%s/%s %s row %q = %q: SDK enum casing is not a word an operator uses",
							shortName, res.ID, code, row.Label, row.Value)
					}
				}
			}
		}
	}
}

// Every finding this batch emits on a demo row carries an operator sentence.
// A registered code with an empty Detail renders a blank line where the
// explanation belongs, and no per-row test can see that for the demo path.
func TestW6BDemoFindingsAllCarryADetailSentence(t *testing.T) {
	want := make(map[string]bool, len(w6bBenchWitnesses))
	for _, w := range w6bBenchWitnesses {
		want[w.code] = true
	}
	for shortName, rows := range w6bMergedDemoRows(t) {
		for _, res := range rows {
			for _, f := range res.Findings {
				if !want[string(f.Code)] {
					continue
				}
				if strings.TrimSpace(f.Detail) == "" {
					t.Errorf("%s/%s %s has an empty Detail; every posture finding states what is wrong and what fixing it takes",
						shortName, res.ID, f.Code)
				}
			}
		}
	}
}

// Dev moved two witness constants so no demo cluster's Version had to change.
// The half of that bargain a bench cannot see is that every demo cluster still
// reports a version the registry publishes: a cluster on a minor the registry
// has never heard of is classified "unknown" and silently drops out of row 18
// altogether, which looks exactly like a healthy cluster.
func TestW6BEKSDemoClusterVersionsAreAllInTheRegistry(t *testing.T) {
	rows := w6bMergedDemoRows(t)["eks"]
	if len(rows) == 0 {
		t.Fatal("no demo rows for eks")
	}

	flagged := 0
	for _, r := range rows {
		if r.Fields["version"] == "" {
			t.Errorf("%s reports no version; row 18 cannot classify it", r.ID)
		}
		for _, f := range r.Findings {
			if string(f.Code) == "eks.version-unsupported" {
				flagged++
				if !strings.Contains(f.Phrase, r.Fields["version"]) {
					t.Errorf("%s is flagged for version %q but its phrase reads %q",
						r.ID, r.Fields["version"], f.Phrase)
				}
			}
		}
	}
	if flagged != 1 {
		t.Errorf("%d demo clusters are out of standard support; the bench expects exactly the one witness", flagged)
	}
}
