// The renderer's status column reads findings first, then
// Fields[lifecycleKey]. Production fetchers for status-column types write
// Fields["status"] for the lifecycle steady-state text, so each of those
// catalog entries must declare LifecycleKey: "status"; with it empty the
// 2-layer read falls back to Fields["state"] (empty) and the steady-state text
// vanishes from the list view.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestAS140_FetcherListPath_NonFindingStatusVisible(t *testing.T) {
	ensureNoColor(t)

	cases := []struct {
		shortName    string
		id           string
		statusPhrase string
	}{
		{"dbi", "prod-dbi-healthy", "available"},
		{"dbc", "prod-dbc-healthy", "available"},
		{"redis", "prod-redis-healthy", "available"},
		{"ddb", "prod-ddb-healthy", "ACTIVE"},
		{"redshift", "prod-redshift-healthy", "available"},
		{"efs", "prod-efs-healthy", "available"},
		{"dbi-snap", "prod-dbi-snap-healthy", "available"},
		{"dbc-snap", "prod-dbc-snap-healthy", "available"},

		{"eks", "prod-eks-healthy", "ACTIVE"},
		{"ng", "prod-ng-healthy", "ACTIVE"},

		{"ecs-svc", "prod-ecs-svc-healthy", "ACTIVE"},
		{"ecs", "prod-ecs-healthy", "ACTIVE"},
		{"ecs-task", "prod-ecs-task-healthy", "RUNNING"},
		{"asg", "prod-asg-healthy", "Healthy"},
		{"eb", "prod-eb-healthy", "Ready"},

		{"cf", "prod-cf-healthy", "Deployed"},
		{"acm", "prod-acm-healthy", "ISSUED"},

		{"kinesis", "prod-kinesis-healthy", "ACTIVE"},
		{"ses", "prod-ses-healthy", "Verified"},

		{"cfn", "prod-cfn-healthy", "CREATE_COMPLETE"},

		{"eni", "prod-eni-healthy", "in-use"},

		{"kms", "prod-kms-healthy", "Enabled"},
	}

	for _, tc := range cases {
		t.Run(tc.shortName, func(t *testing.T) {
			td := resource.FindResourceType(tc.shortName)
			if td == nil {
				t.Fatalf("catalog has no resource type %q — AS-140 regression coverage now misaligned with the catalog", tc.shortName)
			}
			if td.LifecycleKey != "status" {
				t.Fatalf("AS-140: catalog type %q must declare LifecycleKey=\"status\" so the 2-layer renderer reads the fetcher-emitted Fields[\"status\"]; got LifecycleKey=%q",
					tc.shortName, td.LifecycleKey)
			}

			res := resource.Resource{
				ID:       tc.id,
				Name:     tc.id,
				Findings: nil,
				Fields: map[string]string{
					"status": tc.statusPhrase,
				},
			}

			c := openListControllerWithConfig(t, tc.shortName, configForType(tc.shortName))
			joined := wave3RowCellsJoined(t, c, tc.shortName, []resource.Resource{res})
			wantPhrase := domain.HumanizeStatusPhrase(tc.statusPhrase)
			if !strings.Contains(joined, wantPhrase) {
				t.Errorf("AS-140 regression: list row for %q missing fetcher-emitted Fields[\"status\"] = %q (humanized: %q); got: %q",
					tc.shortName, tc.statusPhrase, wantPhrase, joined)
			}
		})
	}
}

func TestAS140_FetcherListPath_FindingsBeatLifecycle(t *testing.T) {
	ensureNoColor(t)

	td := resource.FindResourceType("dbi")
	if td == nil {
		t.Fatal("catalog has no resource type \"dbi\"")
	}
	if td.LifecycleKey != "status" {
		t.Fatalf("AS-140: dbi catalog must declare LifecycleKey=\"status\"; got %q", td.LifecycleKey)
	}

	res := resource.Resource{
		ID:   "prod-dbi-stacked",
		Name: "prod-dbi-stacked",
		Findings: []domain.Finding{
			{Code: domain.FindingCode("dbi.broken.stopped"), Phrase: "stopped", Severity: domain.SevBroken, Source: "wave1"},
			{Code: domain.FindingCode("dbi.warn.maintenance"), Phrase: "maintenance scheduled", Severity: domain.SevWarn, Source: "wave2"},
		},
		Fields: map[string]string{
			"status": "stopped", // fetcher's pre-enrichment merged phrase (Wave-1 only)
		},
	}

	c := openListControllerWithConfig(t, "dbi", configForType("dbi"))
	joined := wave3RowCellsJoined(t, c, "dbi", []resource.Resource{res})
	if !strings.Contains(joined, "stopped (+1)") {
		t.Errorf("AS-140: list row should compose Wave-1+Wave-2 stack as %q via phraseFromFindings; got: %q",
			"stopped (+1)", joined)
	}
}
