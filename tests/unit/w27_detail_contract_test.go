package unit

// w27_detail_contract_test.go — task w27 row 1: the Detail sentence has one
// owner. catalog.FindingDef gained a Detail field in round 0, but nothing
// declares it yet, so every finding that still carries its own inline
// *Detail constant (domain.Finding.Detail, set directly by the fetcher or
// enricher) currently disagrees with its empty definition. This is
// deliberately red until each type's round moves its sentence onto
// FindingDef and the emitter starts reading from there instead.
//
// Two independent proof surfaces, per the dispatch: a full-catalog demo-bench
// walk (every registered type, drained through its own Fetcher/Wave2Enricher
// with demo.NewServiceClients()), and the existing batch-w1 demo bench
// (pw1ComputeBench, prowler_w1_status_phrase_test.go) that a sibling suite of
// emitter tests already exercises and relies on.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// detailMismatch names one finding whose emitted Detail disagrees with its
// registered FindingDef.Detail.
type detailMismatch struct {
	shortName string
	code      domain.FindingCode
	emitted   string
	declared  string
}

func (m detailMismatch) String() string {
	return fmt.Sprintf("%s/%s: emitted %q, definition declares %q", m.shortName, m.code, m.emitted, m.declared)
}

// checkRowsAgainstDefinitions walks rows' Findings and reports every finding
// whose non-empty Detail does not equal its type's declared FindingDef.Detail.
// defsByCode is built once per type from td.Findings, since a code's meaning
// (and its Detail) is scoped to the type that registers it.
func checkRowsAgainstDefinitions(shortName string, defsByCode map[domain.FindingCode]string, findings []domain.Finding) []detailMismatch {
	var out []detailMismatch
	for _, f := range findings {
		if f.Detail == "" {
			continue
		}
		declared := defsByCode[f.Code]
		if f.Detail != declared {
			out = append(out, detailMismatch{shortName: shortName, code: f.Code, emitted: f.Detail, declared: declared})
		}
	}
	return out
}

// detailBenchResult is one full-catalog demo-bench walk's findings: every
// mismatch between an emitted Detail and its FindingDef, and every
// (shortName, code) pair that was actually witnessed carrying a non-empty
// Detail — the coverage half of the contract, since a definition and an
// emitter that both silently agree on "" are not proof of anything.
type detailBenchResult struct {
	mismatches []detailMismatch
	witnessed  map[string]bool // "shortName/code"
}

// runFullCatalogDetailBench walks every registered type's own Fetcher and,
// when registered, its Wave2Enricher, against demo.NewServiceClients().
func runFullCatalogDetailBench(t *testing.T) detailBenchResult {
	t.Helper()
	clients := demo.NewServiceClients()
	cache := resource.ResourceCache{}

	// sg.unused scans the "eni" cache, the vpc-peer route findings the "rtb"
	// cache, the not-in-backup-plan / no-snapshot findings the "backup" and
	// "ebs-snap" caches, cf.origin-bucket-missing the "s3" cache, and
	// r53.dangling-record the "eip", "ec2" and "eni" caches (all zero-call
	// enrichers); load them all so the bench sees what a demo user who has
	// opened those lists sees.
	for _, td := range resource.AllResourceTypes() {
		switch td.ShortName {
		case "eni", "rtb", "backup", "ebs-snap", "s3", "eip", "ec2":
		default:
			continue
		}
		if rows, ok := DrainFixtures(t, td, clients); ok {
			cache[td.ShortName] = resource.ResourceCacheEntry{Resources: rows}
		}
	}

	result := detailBenchResult{witnessed: map[string]bool{}}

	for _, td := range resource.AllResourceTypes() {
		resources, ok := DrainFixtures(t, td, clients)
		if !ok {
			continue
		}

		defsByCode := make(map[domain.FindingCode]string, len(td.Findings))
		for _, def := range td.Findings {
			defsByCode[def.Code] = def.Detail
		}

		res := awsclient.IssueEnricherResult{}
		if e, ok := awsclient.Wave2EnricherFor(td.ShortName); ok && e.Fn != nil {
			var err error
			res, err = e.Fn(context.Background(), clients, resources, cache)
			if err != nil {
				t.Fatalf("demo %s Wave2Enricher: %v", td.ShortName, err)
			}
		}

		for i := range resources {
			r := resources[i]
			runtime.ApplyWave2ToRow(&r, td, res.Findings, res.AttentionDetails)
			for _, f := range r.Findings {
				if f.Detail != "" {
					result.witnessed[td.ShortName+"/"+string(f.Code)] = true
				}
			}
			result.mismatches = append(result.mismatches, checkRowsAgainstDefinitions(td.ShortName, defsByCode, r.Findings)...)
		}
	}
	return result
}

// TestDetailContract_FullCatalogDemoBench asserts every finding with a
// non-empty Detail equals its FindingDef's Detail. Deliberately red today:
// round 0 populated no FindingDef.Detail, so every finding still carrying its
// old inline *Detail constant mismatches its (empty) definition.
func TestDetailContract_FullCatalogDemoBench(t *testing.T) {
	result := runFullCatalogDetailBench(t)

	if len(result.witnessed) == 0 {
		t.Fatal("the demo bench produced zero findings with a non-empty Detail across the whole catalog; " +
			"the walk is broken, not the contract")
	}

	if len(result.mismatches) > 0 {
		mismatches := result.mismatches
		sort.Slice(mismatches, func(i, j int) bool {
			if mismatches[i].shortName != mismatches[j].shortName {
				return mismatches[i].shortName < mismatches[j].shortName
			}
			return mismatches[i].code < mismatches[j].code
		})
		var b []string
		for _, m := range mismatches {
			b = append(b, m.String())
		}
		t.Errorf("%d finding(s) emit a Detail their FindingDef does not declare (this shrinks to zero as each type's "+
			"round moves its sentence onto FindingDef and the emitter reads from there):\n%s",
			len(mismatches), joinLines(b))
	}
}

// TestDetailContract_EveryDeclaredDetailHasABenchWitness closes the class
// TestDetailContract_FullCatalogDemoBench cannot see: that test only checks
// findings that actually carry a non-empty Detail, so a definition that
// declares one while its emitter forgets to stamp it renders phrase-only
// with nothing catching it (both sides silently agree on ""). Proven with a
// throwaway mutation during this verify round: deleting sg.go's
// catalog.Detail(sgCodeDangerousPorts) call left
// TestDetailContract_FullCatalogDemoBench green.
func TestDetailContract_EveryDeclaredDetailHasABenchWitness(t *testing.T) {
	result := runFullCatalogDetailBench(t)

	// Union in pw1ComputeBench's witnesses too: its real sg/ebs/ami
	// cross-ref cache fires Wave-2 findings (e.g. ec2.internet-exposed) the
	// full-catalog walk's empty cache cannot, and a code witnessed by either
	// bench is a code a demo user can actually see.
	for _, row := range pw1ComputeBench(t) {
		for _, f := range row.res.Findings {
			if f.Detail != "" {
				result.witnessed[row.typeName+"/"+string(f.Code)] = true
			}
		}
	}

	var missing []string
	for _, td := range resource.AllResourceTypes() {
		for _, def := range td.Findings {
			if def.Detail == "" {
				continue
			}
			if !result.witnessed[td.ShortName+"/"+string(def.Code)] {
				missing = append(missing, td.ShortName+"/"+string(def.Code))
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("%d FindingDef(s) declare a Detail that no demo-bench witness ever carries — either the emitter "+
			"forgot to stamp it, or the demo fixtures never trigger the finding:\n%s",
			len(missing), joinLines(missing))
	}
}

// TestDetailContract_ExistingBatchW1Bench reruns the same invariant over
// pw1ComputeBench's rows (prowler_w1_status_phrase_test.go) — the demo bench a
// sibling suite of emitter tests already builds and relies on, with real
// cross-referenced ResourceCache entries (sg/ebs/ami) that the full-catalog
// walk above does not construct. A mismatch caught only here would mean the
// full-catalog walk's empty cache silently hid it.
func TestDetailContract_ExistingBatchW1Bench(t *testing.T) {
	rows := pw1ComputeBench(t)
	if len(rows) == 0 {
		t.Fatal("pw1ComputeBench produced zero rows; nothing was checked")
	}

	defsByType := map[string]map[domain.FindingCode]string{}
	var mismatches []detailMismatch
	for _, row := range rows {
		defsByCode, ok := defsByType[row.typeName]
		if !ok {
			td := catalog.Find(row.typeName)
			if td == nil {
				t.Fatalf("%s has no catalog entry", row.typeName)
			}
			defsByCode = make(map[domain.FindingCode]string, len(td.Findings))
			for _, def := range td.Findings {
				defsByCode[def.Code] = def.Detail
			}
			defsByType[row.typeName] = defsByCode
		}
		mismatches = append(mismatches, checkRowsAgainstDefinitions(row.typeName, defsByCode, row.res.Findings)...)
	}

	if len(mismatches) > 0 {
		sort.Slice(mismatches, func(i, j int) bool {
			if mismatches[i].shortName != mismatches[j].shortName {
				return mismatches[i].shortName < mismatches[j].shortName
			}
			return mismatches[i].code < mismatches[j].code
		})
		var b []string
		for _, m := range mismatches {
			b = append(b, m.String())
		}
		t.Errorf("%d finding(s) on the existing batch-w1 bench emit a Detail their FindingDef does not declare:\n%s",
			len(mismatches), joinLines(b))
	}
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += "  " + l + "\n"
	}
	return out
}

// TestDetailContract_TenDevopsSentencesVerbatim pins TASKDIR/detail_sentences.md's
// nine single-state sentences character-for-character (row 3/5 ruling: "the
// ten sentences ... are declared on the definitions under row 1 verbatim").
// The tenth, tgw.attachment-transitional, is not one of the nine: devops
// wrote three per-state bullets for it and the ruling has dev consolidate
// them into one sentence naming pending acceptance as the state that needs a
// person, so it is pinned separately by shape, not by verbatim text.
func TestDetailContract_TenDevopsSentencesVerbatim(t *testing.T) {
	verbatim := map[domain.FindingCode]string{
		"elb.state.provisioning":    "The load balancer is still being built and is not yet accepting traffic. This normally clears in a few minutes; if it does not, its subnets are usually out of free IP addresses.",
		"elb.state.active_impaired": "The load balancer is serving traffic but could not set up or scale in at least one availability zone, so capacity there is degraded. Check that every attached subnet has spare IP addresses.",
		"elb.state.failed":          "The load balancer could not be created and will not recover on its own. It has to be deleted and recreated; nothing routes through it in the meantime.",
		"tgw.state.pending":         "The gateway is still being created and does not route yet. Attachments created now stay pending until it comes up.",
		"tgw.state.modifying":       "A configuration change is being applied. Routing across the gateway can be inconsistent until it settles.",
		"tgw.state.deleting":        "The gateway is being torn down. Every attachment on it goes away and any traffic still routed through it will stop.",
		"tgw.state.failed":          "The gateway could not be created and will not recover. It has to be recreated, and anything routed through it has no path.",
		"tgw.state.deleted":         "This gateway is gone. AWS keeps returning it for a while after deletion, so route tables that still point at it are dead references worth cleaning up.",
		"tgw.attachment-failed":     "The network behind this attachment has no path across the gateway. Failed attachments do not retry; delete and recreate the attachment.",
	}
	for code, want := range verbatim {
		got := catalog.Detail(code)
		if got != want {
			t.Errorf("catalog.Detail(%q) = %q, want devops's sentence verbatim:\n  %q", code, got, want)
		}
	}
}

// TestDetailContract_TransitionalAttachmentCode_OneSentence pins the
// consolidated shape for tgw.attachment-transitional: one sentence covering
// all three transitional states, naming pending acceptance as the one that
// needs a person (row 1's ruling on this code specifically).
func TestDetailContract_TransitionalAttachmentCode_OneSentence(t *testing.T) {
	const code domain.FindingCode = "tgw.attachment-transitional"
	got := catalog.Detail(code)
	if got == "" {
		t.Fatalf("catalog.Detail(%q) is empty", code)
	}
	if !strings.Contains(strings.ToLower(got), "pending acceptance") {
		t.Errorf("catalog.Detail(%q) = %q, want it to name pending acceptance as the state that needs a person", code, got)
	}
	if !strings.Contains(strings.ToLower(got), "person") && !strings.Contains(strings.ToLower(got), "approve") {
		t.Errorf("catalog.Detail(%q) = %q, want it to say the owning account/person must act", code, got)
	}
}
