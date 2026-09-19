package unit_test

// backup_selection_coverage_bench_test.go — the backup coverage evaluator on
// the demo bench, folded through runtime.ApplyWave2ToRow and rendered through
// the list and detail builders the app uses.

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"

	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	a9sruntime "github.com/k2m30/a9s/v3/core/runtime"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

func bk551DemoRows(t *testing.T, short string) []resource.Resource {
	t.Helper()
	td := resource.FindResourceType(short)
	if td == nil {
		t.Fatalf("%s not registered", short)
	}
	rows, ok := unit.DrainFixtures(t, *td, demo.NewServiceClients())
	if !ok {
		t.Fatalf("%s has no Wave-1 fetcher", short)
	}
	return rows
}

func bk551DemoFolded(t *testing.T) map[string][]resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	cache := resource.ResourceCache{
		"backup":   resource.ResourceCacheEntry{Resources: bk551DemoRows(t, "backup")},
		"ebs-snap": resource.ResourceCacheEntry{Resources: bk551DemoRows(t, "ebs-snap")},
	}
	folded := map[string][]resource.Resource{}
	for _, short := range []string{"ebs", "dbi", "dbc", "ddb"} {
		td := resource.FindResourceType(short)
		rows := bk551DemoRows(t, short)
		enricher, ok := awsclient.Wave2EnricherFor(short)
		if !ok || enricher.Fn == nil {
			t.Fatalf("%s has no wave-2 enricher", short)
		}
		res, err := enricher.Fn(context.Background(), clients, rows, cache)
		if err != nil && res.Findings == nil {
			t.Fatalf("%s wave-2: %v", short, err)
		}
		for i := range rows {
			a9sruntime.ApplyWave2ToRow(&rows[i], *td, res.Findings, res.AttentionDetails)
		}
		folded[short] = rows
	}
	return folded
}

var bk551EnumShape = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$|^([A-Z][a-z0-9]+){2,}$`)

func TestBackupCoverageBench_WitnessesRenderTheirPhrase(t *testing.T) {
	folded := bk551DemoFolded(t)
	witnesses := []struct {
		short  string
		id     string
		code   domain.FindingCode
		phrase string
	}{
		{"ebs", fixtures.EBSNotInBackupPlan, awsclient.CodeEBSNotInBackupPlan, "not covered by a backup plan"},
		{"dbi", fixtures.DBINotInBackupPlan, awsclient.CodeDBINotInBackupPlan, "not covered by a backup plan"},
		{"dbc", fixtures.DBCNotInBackupPlan, awsclient.CodeDBCNotInBackupPlan, "not covered by a backup plan"},
		{"ddb", fixtures.DDBNotInBackupPlan, awsclient.CodeDDBNotInBackupPlan, "not covered by a backup plan"},
	}
	for _, w := range witnesses {
		t.Run(w.short, func(t *testing.T) {
			td := resource.FindResourceType(w.short)
			rows := folded[w.short]
			idx := slices.IndexFunc(rows, func(r resource.Resource) bool { return r.ID == w.id })
			if idx < 0 {
				t.Fatalf("witness %s not in the demo %s list", w.id, w.short)
			}
			row := rows[idx]
			if !slices.ContainsFunc(row.Findings, func(f domain.Finding) bool { return f.Code == w.code && f.Phrase == w.phrase }) {
				t.Fatalf("witness findings = %+v, want %s %q among them", row.Findings, w.code, w.phrase)
			}
			// Some witnesses also carry a finding that outranks coverage, which
			// is what lets this check tell the selector from first-wins.
			top, _ := domain.TopFinding(row.Findings)

			cell, ok := listStatusCellFor(t, *td, rows, w.id)
			if !ok {
				t.Fatalf("no Status cell for %s", w.id)
			}
			if phrase, _, _ := strings.Cut(cell, " (+"); phrase != top.Phrase {
				t.Errorf("Status cell = %q, want the top finding's %q", cell, top.Phrase)
			}
			if got, want := td.ResolveColor(row), w5ColorOfSeverity(top.Severity); got != want {
				t.Errorf("row colour = %v, want %v (the colour of %s)", got, want, top.Code)
			}

			ad, ok := row.AttentionDetails[w.code]
			if !ok || len(ad.Rows) == 0 {
				t.Fatalf("no supporting rows for %s", w.code)
			}
			for _, r := range ad.Rows {
				v, _, _ := strings.Cut(r.Value, " (")
				if strings.Contains(strings.ToLower(r.Value), w.phrase) {
					t.Errorf("supporting row %s: %q restates the phrase", r.Label, r.Value)
				}
				if v == "true" || v == "false" || bk551EnumShape.MatchString(v) {
					t.Errorf("supporting row %s: %q is a Go bool or SDK enum", r.Label, r.Value)
				}
			}
			n := 0
			for _, v := range detailAttentionValuesFor(t, row, w.short) {
				if strings.Contains(strings.ToLower(v), w.phrase) {
					n++
				}
			}
			if n != 1 {
				t.Errorf("detail Attention block states %q %d times, want once", w.phrase, n)
			}
		})
	}
}

// The demo carries a resource excluded by one selection's NotResources and
// taken in by another selection of the same plan: the row where a plan-wide
// exclusion would report a covered resource as uncovered.
func TestBackupCoverageBench_ExclusionInOneSelectionOnlyStaysCovered(t *testing.T) {
	folded := bk551DemoFolded(t)
	codes := map[string]domain.FindingCode{
		"ebs": awsclient.CodeEBSNotInBackupPlan, "dbi": awsclient.CodeDBINotInBackupPlan,
		"dbc": awsclient.CodeDBCNotInBackupPlan, "ddb": awsclient.CodeDDBNotInBackupPlan,
	}
	arnOf := func(short string, r resource.Resource) string {
		if short == "ebs" {
			return "arn:aws:ec2:us-east-1:123456789012:volume/" + r.ID
		}
		return r.Fields["arn"]
	}

	found := 0
	for _, plan := range bk551DemoRows(t, "backup") {
		sels, _ := awsclient.BackupPlanSelections(plan)
		for short, rows := range folded {
			for _, r := range rows {
				arn := arnOf(short, r)
				if arn == "" {
					continue
				}
				excludedSomewhere := slices.ContainsFunc(sels, func(s backuptypes.BackupSelection) bool {
					return len(s.NotResources) > 0 &&
						awsclient.BackupSelectionCovers(backuptypes.BackupSelection{Resources: s.NotResources}, arn, nil)
				})
				if !excludedSomewhere {
					continue
				}
				if covered, _ := awsclient.BackupPlanCovers(plan, arn, nil, false); !covered {
					continue
				}
				found++
				for _, f := range r.Findings {
					if f.Code == codes[short] {
						t.Errorf("%s %s: plan %s covers it through another selection, yet it reads %q", short, r.ID, plan.ID, f.Phrase)
					}
				}
			}
		}
	}
	if found == 0 {
		t.Error("no demo resource is excluded by one selection and covered by another of the same plan, so the per-selection rule has no witness")
	}
}
