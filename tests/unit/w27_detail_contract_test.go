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

// TestDetailContract_FullCatalogDemoBench walks every registered type's own
// Fetcher and, when registered, its Wave2Enricher, against demo.NewServiceClients().
// Any resulting finding with a non-empty Detail must equal its FindingDef's
// Detail. Deliberately red today: round 0 populated no FindingDef.Detail, so
// every finding still carrying its old inline *Detail constant mismatches its
// (empty) definition.
func TestDetailContract_FullCatalogDemoBench(t *testing.T) {
	clients := demo.NewServiceClients()
	cache := resource.ResourceCache{}

	var mismatches []detailMismatch
	observed := 0

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
					observed++
				}
			}
			mismatches = append(mismatches, checkRowsAgainstDefinitions(td.ShortName, defsByCode, r.Findings)...)
		}
	}

	if observed == 0 {
		t.Fatal("the demo bench produced zero findings with a non-empty Detail across the whole catalog; " +
			"the walk is broken, not the contract")
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
		t.Errorf("%d finding(s) emit a Detail their FindingDef does not declare (this shrinks to zero as each type's "+
			"round moves its sentence onto FindingDef and the emitter reads from there):\n%s",
			len(mismatches), joinLines(b))
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
