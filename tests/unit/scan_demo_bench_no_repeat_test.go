package unit_test

// scan_demo_bench_no_repeat_test.go — the bench-wide proof that nothing
// renders one condition twice.
//
// The per-caller pins say what one enricher does with one input. This walks
// every demo row of every registered type through its real Wave-1 fetcher and
// its real Wave-2 enricher, folded the way the app folds it, and answers the
// question the per-caller pins cannot: which callers repeat themselves today.
//
// Two shapes count as repetition, and they fail in different places:
//
//   - the same finding code twice on one resource, visible only in the
//     enricher's own result — runtime.ApplyWave2ToRow drops the second by
//     code, so the duplicate costs a supporting row rather than showing as a
//     second row in the detail view;
//   - two supporting rows naming the same scanner hit under one finding,
//     which is what the reader actually sees: one leak listed twice, once
//     per reason the scanner had for reporting it. Rows whose value is not a
//     scanner kind are out of scope — a listener row, a target row and a
//     container row legitimately repeat their label once per item.

import (
	"context"
	"fmt"
	"sort"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

func TestScanDemoBench_NoResourceReportsOneConditionTwice(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	// The kinds secretscan.Hit can carry. A supporting row whose value is one
	// of these names a scanner hit, and two such rows under one label are one
	// leak counted twice.
	scannerKinds := map[string]bool{
		"aws-access-key": true, "private-key": true, "jwt": true,
		"keyword": true, "high-entropy": true,
	}

	var duplicateFindings, duplicateRows []string

	for _, td := range resource.AllResourceTypes() {
		rows := byType[td.ShortName]
		if len(rows) == 0 {
			continue
		}

		var result awsclient.IssueEnricherResult
		if enricher, ok := awsclient.Wave2EnricherFor(td.ShortName); ok && enricher.Fn != nil {
			r, err := enricher.Fn(context.Background(), clients, rows, cache)
			if err != nil {
				t.Fatalf("%s: Wave-2 enricher returned error: %v", td.ShortName, err)
			}
			result = r
			for id, fs := range result.Findings {
				count := map[domain.FindingCode]int{}
				for _, f := range fs {
					count[f.Code]++
				}
				for code, n := range count {
					if n > 1 {
						duplicateFindings = append(duplicateFindings,
							fmt.Sprintf("%s/%s emits %s %d times", td.ShortName, id, code, n))
					}
				}
			}
		}

		merged := make([]resource.Resource, len(rows))
		copy(merged, rows)
		for i := range merged {
			runtime.ApplyWave2ToRow(&merged[i], td, result.Findings, result.AttentionDetails)
			for code, ad := range merged[i].AttentionDetails {
				count := map[string]int{}
				for _, row := range ad.Rows {
					if scannerKinds[row.Value] {
						count[row.Label]++
					}
				}
				for label, n := range count {
					if n > 1 {
						duplicateRows = append(duplicateRows,
							fmt.Sprintf("%s/%s %s names the hit %q in %d rows", td.ShortName, merged[i].ID, code, label, n))
					}
				}
			}
		}
	}

	sort.Strings(duplicateFindings)
	sort.Strings(duplicateRows)
	for _, offender := range duplicateFindings {
		t.Errorf("duplicate finding: %s", offender)
	}
	for _, offender := range duplicateRows {
		t.Errorf("repeated scanner hit: %s", offender)
	}
}
