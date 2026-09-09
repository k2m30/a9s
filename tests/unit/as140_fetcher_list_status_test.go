// as140_fetcher_list_status_test.go — fetcher→list rendering of non-finding
// lifecycle/status text.
//
// The renderer's status column reads findings first, then
// Fields[lifecycleKey]. Production fetchers for status-column types (dbi, dbc,
// redis, ddb, eks, ng, asg, eb, cfn, cf, acm, kinesis, ses, eni, kms, ecs-svc,
// ecs, ecs-task, redshift, efs, dbi-snap, dbc-snap) write Fields["status"] for
// the lifecycle steady-state text, so each of those catalog entries must
// declare LifecycleKey: "status"; with it empty the 2-layer read falls back to
// Fields["state"] (empty) and the "available" / "ACTIVE" / "running" text
// vanishes from the list view.
//
// This test exercises the full fetcher→list path: a Resource shaped exactly
// like the fetcher emits (Fields["status"] populated, Findings empty) MUST
// render the steady-state phrase in the list view.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestAS140_FetcherListPath_NonFindingStatusVisible exercises the fetcher→list
// contract: when a fetcher emits Fields["status"] = <lifecycle phrase> with
// empty Findings (healthy / non-broken / non-transitional row), the list
// status column MUST display the phrase (as domain.HumanizeStatusPhrase
// renders it — e.g. raw "ACTIVE"/"CREATE_COMPLETE" enums lowercase and
// de-snake into "active"/"create complete"; an already-lowercase phrase like
// "available" or "in-use" passes through unchanged). This regression-pins
// the LifecycleKey: "status" declaration in core/catalog/types_*.go for
// every type whose status column key is "status".
func TestAS140_FetcherListPath_NonFindingStatusVisible(t *testing.T) {
	ensureNoColor(t)

	// Each case mirrors what a production fetcher emits for a healthy row:
	// Fields["status"] carries the raw lifecycle phrase exactly as the AWS
	// SDK returns it; Findings is empty (no Wave-1 broken/warn/transitional
	// finding fired). The assertion below humanizes statusPhrase the same
	// way the render chokepoint does before checking the list view.
	cases := []struct {
		shortName    string // catalog ShortName
		id           string // resource id for this row
		statusPhrase string // value the fetcher would set Fields["status"] to
	}{
		// DATABASES & STORAGE — types_databases.go
		{"dbi", "prod-dbi-healthy", "available"},
		{"dbc", "prod-dbc-healthy", "available"},
		{"redis", "prod-redis-healthy", "available"},
		{"ddb", "prod-ddb-healthy", "ACTIVE"},
		{"redshift", "prod-redshift-healthy", "available"},
		{"efs", "prod-efs-healthy", "available"},
		{"dbi-snap", "prod-dbi-snap-healthy", "available"},
		{"dbc-snap", "prod-dbc-snap-healthy", "available"},

		// CONTAINERS — types_containers.go
		{"eks", "prod-eks-healthy", "ACTIVE"},
		{"ng", "prod-ng-healthy", "ACTIVE"},

		// COMPUTE — types_compute.go
		{"ecs-svc", "prod-ecs-svc-healthy", "ACTIVE"},
		{"ecs", "prod-ecs-healthy", "ACTIVE"},
		{"ecs-task", "prod-ecs-task-healthy", "RUNNING"},
		{"asg", "prod-asg-healthy", "Healthy"},
		{"eb", "prod-eb-healthy", "Ready"},

		// DNS & CDN — types_dns_cdn.go
		{"cf", "prod-cf-healthy", "Deployed"},
		{"acm", "prod-acm-healthy", "ISSUED"},

		// MESSAGING — types_messaging.go
		{"kinesis", "prod-kinesis-healthy", "ACTIVE"},
		{"ses", "prod-ses-healthy", "Verified"},

		// CI/CD — types_cicd.go
		{"cfn", "prod-cfn-healthy", "CREATE_COMPLETE"},

		// NETWORKING — types_networking.go
		{"eni", "prod-eni-healthy", "in-use"},

		// SECRETS & CONFIG — types_secrets.go
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
				ID:   tc.id,
				Name: tc.id,
				// Findings intentionally empty — mirrors a fetcher's healthy
				// row that emits no Wave-1 broken/warn/transitional phrase.
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

// TestAS140_FetcherListPath_FindingsBeatLifecycle exercises the other half of
// the contract: when a Wave-1 finding is active, the merged
// "<top> (+N)" phrase from phraseFromFindings(r.Findings) wins over the
// Fields["status"] fallback. This pins the layer-1-wins-over-layer-2 order.
func TestAS140_FetcherListPath_FindingsBeatLifecycle(t *testing.T) {
	ensureNoColor(t)

	td := resource.FindResourceType("dbi")
	if td == nil {
		t.Fatal("catalog has no resource type \"dbi\"")
	}
	if td.LifecycleKey != "status" {
		t.Fatalf("AS-140: dbi catalog must declare LifecycleKey=\"status\"; got %q", td.LifecycleKey)
	}

	// Wave-1 fetcher emitted a "stopped" finding; Fields["status"] mirrors
	// the merged phrase (what real fetchers do via phraseFromFindings).
	// applyEnrichment later adds a Wave-2 "maintenance scheduled" finding —
	// the renderer must stack them into "stopped (+1)" via phraseFromFindings(r.Findings).
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
