package unit

// The Detail sentence has one owner:
// catalog.FindingDef.Detail. Every finding's Detail (domain.Finding.Detail)
// equals its definition's; no fetcher or enricher carries its own inline
// sentence.
//
// Two independent proof surfaces: a full-catalog demo-bench walk (every
// registered type, drained through its own Fetcher/Wave2Enricher with
// demo.NewServiceClients()), and the batch-w1 demo bench (pw1ComputeBench,
// prowler_w1_status_phrase_test.go) that a sibling suite of emitter tests
// already exercises and relies on.

import (
	"context"
	"fmt"
	"sort"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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
// whose Detail does not equal its type's declared FindingDef.Detail.
// defsByCode is built once per type from td.Findings, since a code's meaning
// (and its Detail) is scoped to the type that registers it.
//
// An empty emitted Detail counts. A code emitted from two places, one of which
// stamps the sentence and one of which forgets, renders with a reason on some
// rows and without one on others. A code whose definition declares no
// sentence still matches an emitter that stamps none, so the check stays quiet
// for the many findings that carry a phrase alone.
func checkRowsAgainstDefinitions(shortName string, defsByCode map[domain.FindingCode]string, findings []domain.Finding) []detailMismatch {
	var out []detailMismatch
	for _, f := range findings {
		declared := defsByCode[f.Code]
		if f.Detail != declared {
			out = append(out, detailMismatch{shortName: shortName, code: f.Code, emitted: f.Detail, declared: declared})
		}
	}
	return out
}

// detailBenchResult is one full-catalog demo-bench walk's findings: every
// mismatch between an emitted Detail and its FindingDef, and every
// (shortName, code) pair actually observed carrying a non-empty Detail — the
// coverage half of the contract, since a definition and an emitter that both
// silently agree on "" are not proof of anything.
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

	// The cross-ref enrichers scan other types' caches, and each one names the
	// caches it scans on its own registration. Loading exactly that union is
	// what a demo user who has opened those lists sees; typing the union out
	// here instead would go stale the first time an enricher gained a scan,
	// silently, because an unloaded cache produces no finding rather than an
	// error.
	needed := map[string]bool{}
	for _, e := range awsclient.AllWave2() {
		for _, name := range e.Enricher.Reads {
			needed[name] = true
		}
	}
	for _, td := range resource.AllResourceTypes() {
		if !needed[td.ShortName] {
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
// non-empty Detail equals its FindingDef's Detail.
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

// TestDetailContract_EveryDeclaredDetailHasABenchWitness covers what
// TestDetailContract_FullCatalogDemoBench cannot see: a definition that
// declares a sentence for a code the demo fixtures never produce. There is no
// row to compare, so only a coverage check finds it.
func TestDetailContract_EveryDeclaredDetailHasABenchWitness(t *testing.T) {
	result := runFullCatalogDetailBench(t)

	// Union in the codes pw1ComputeBench produces too: its real sg/ebs/ami
	// cross-ref cache fires Wave-2 findings (e.g. ec2.internet-exposed) the
	// full-catalog walk's empty cache cannot, and a code seen on either bench is
	// a code a demo user can actually see.
	for _, row := range pw1ComputeBench(t) {
		for _, f := range row.res.Findings {
			if f.Detail != "" {
				result.witnessed[row.typeName+"/"+string(f.Code)] = true
			}
		}
	}

	// A code the fixture registry already declares as a coverage gap is not a
	// forgotten definition: it is a state the demo provably cannot stage
	// beside healthy rows — SES exposes one account-wide enforcement status,
	// so its three mutually exclusive codes cannot all be staged at once,
	// and OpenSearch's per-item describe is batched, so a denial degrades
	// every row rather than one. The registry is the single place those gaps
	// are declared and ratcheted down; reading it here keeps this gate from
	// growing a second, competing list.
	gaps := fixtures.CoverageGaps()

	var missing []string
	for _, td := range resource.AllResourceTypes() {
		for _, def := range td.Findings {
			if def.Detail == "" {
				continue
			}
			if gaps[td.ShortName+":"+string(def.Code)] {
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
			td := catalog.FindAny(row.typeName)
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
