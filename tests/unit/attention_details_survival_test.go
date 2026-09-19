package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// ListRow carries no AttentionDetails field, so Core.ResourceCache is the only
// exported seam that exposes them; AttentionDetails are in-memory only (never
// on cache.Row/TypeFile).

const (
	attnSurvivalProfile = "attn-survival-prof"
	attnSurvivalRegion  = "us-east-1"
	attnSurvivalType    = "dbi"
)

// attnRawFixture is a minimal AWS-SDK-shaped struct satisfying dbi's real
// default column Paths via fieldpath.ExtractScalar's case-insensitive
// field-name fallback (mirrors s3RawFixture in qa_cache_lifecycle_test.go).
type attnRawFixture struct {
	Name string
}

func newAttnController(t *testing.T, profile, region string) (*runtime.Core, *app.Controller) {
	t.Helper()
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return core, ctrl
}

func attnResource(id, status string, findings ...domain.Finding) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: id,
		Type: attnSurvivalType,
		Fields: map[string]string{
			"status": status,
		},
		RawStruct: attnRawFixture{Name: id},
		Findings:  findings,
	}
}

func TestControllerReapply_AttentionDetails_SurviveFreshFetchReplace(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	resource.SetPaginatedForTest(attnSurvivalType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, nil
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(attnSurvivalType) })

	core, ctrl := newAttnController(t, attnSurvivalProfile, attnSurvivalRegion)

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: attnSurvivalType})

	wave2Finding := domain.Finding{
		Code:     domain.FindingCode("dbi.maintenance-overdue"),
		Phrase:   "maintenance overdue",
		Severity: domain.SevBroken,
		Source:   "wave2:" + attnSurvivalType,
	}

	initial := []resource.Resource{
		attnResource("dbi-instance-a", "available"),
		attnResource("dbi-instance-b", "available"),
	}
	if _, _, err := deliverAttnResourcesLoaded(ctrl, initial, false); err != nil {
		t.Fatalf("initial ResourcesLoaded delivery: %v", err)
	}

	details := map[string]map[domain.FindingCode]domain.AttentionDetail{
		"dbi-instance-a": {
			wave2Finding.Code: {
				Rows: []domain.DetailRow{
					{Label: "Action", Value: "system-update", Tier: "~"},
				},
			},
		},
	}
	ctrl.ApplyEnrichmentState(attnSurvivalType, 1, false, map[string][]domain.Finding{
		"dbi-instance-a": {wave2Finding},
	}, details)

	// ApplyEnrichmentState only fills the controller's enrichment maps; the
	// RowStore-backed row picks up the finding and details on the next
	// ResourcesLoaded delivery.

	fresh := []resource.Resource{
		attnResource("dbi-instance-a", "available"),
		attnResource("dbi-instance-b", "available"),
	}
	if _, _, err := deliverAttnResourcesLoaded(ctrl, fresh, false); err != nil {
		t.Fatalf("fresh ResourcesLoaded delivery: %v", err)
	}

	entry, ok := core.ResourceCache(attnSurvivalType)
	if !ok {
		t.Fatal("Core.ResourceCache after fresh-fetch replace: type not observed")
	}
	row, ok := findDomainResource(entry.Resources, "dbi-instance-a")
	if !ok {
		t.Fatal("Core.ResourceCache after fresh-fetch replace missing dbi-instance-a")
	}

	if len(row.Findings) != 1 || row.Findings[0].Code != wave2Finding.Code {
		t.Fatalf("post-replace dbi-instance-a.Findings = %+v, want 1 finding with Code=%q — the re-apply step must re-attach the stored Wave-2 finding", row.Findings, wave2Finding.Code)
	}

	ad, ok := row.AttentionDetails[wave2Finding.Code]
	if !ok {
		t.Fatalf("post-replace dbi-instance-a.AttentionDetails missing entry for Code=%q — AttentionDetails dropped across the fresh-fetch replace even though the Wave-2 Finding survived", wave2Finding.Code)
	}
	if len(ad.Rows) != 1 {
		t.Fatalf("post-replace dbi-instance-a.AttentionDetails[%q].Rows = %+v, want 1 row", wave2Finding.Code, ad.Rows)
	}
	gotRow := ad.Rows[0]
	if gotRow.Label != "Action" || gotRow.Value != "system-update" || gotRow.Tier != "~" {
		t.Errorf("post-replace dbi-instance-a.AttentionDetails[%q].Rows[0] = %+v, want {Label:Action Value:system-update Tier:~}", wave2Finding.Code, gotRow)
	}

	rowB, ok := findDomainResource(entry.Resources, "dbi-instance-b")
	if !ok {
		t.Fatal("Core.ResourceCache after fresh-fetch replace missing dbi-instance-b")
	}
	if len(rowB.AttentionDetails) != 0 {
		t.Errorf("post-replace dbi-instance-b.AttentionDetails = %+v, want empty — this resource was never enriched", rowB.AttentionDetails)
	}
}

// deliverAttnResourcesLoaded delivers a ResourcesLoaded event through the real
// menu-syncing seam (Controller.Handle), mirroring deliverVerifyFetch in
// qa_cache_lifecycle_test.go. Gen:0 always passes the staleness guard.
func deliverAttnResourcesLoaded(ctrl *app.Controller, resources []resource.Resource, truncated bool) (app.ViewState, []runtime.TaskRequest, error) {
	vs, tasks := handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: attnSurvivalType,
		Resources:    resources,
		Pagination:   &resource.PaginationMeta{IsTruncated: truncated},
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})
	return vs, tasks, nil
}

// findDomainResource returns the domain.Resource with the given ID, or
// (domain.Resource{}, false).
func findDomainResource(rows []domain.Resource, id string) (domain.Resource, bool) {
	for _, r := range rows {
		if r.ID == id {
			return r, true
		}
	}
	return domain.Resource{}, false
}

func TestHandleAvailabilityChecked_Wave2Carry_AttentionDetailsSurviveWithFinding(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	c := newSaveCacheRegressionCore(t, false)

	carriedDetailCode := domain.FindingCode("s3-public-access")
	enrichedRow := resource.Resource{
		ID:   "s3-bucket-x",
		Name: "s3-bucket-x",
		Type: "s3",
		Fields: map[string]string{
			"status": "public access block incomplete",
		},
		Findings: []domain.Finding{
			{
				Code:     carriedDetailCode,
				Phrase:   "public access block incomplete",
				Severity: domain.SevBroken,
				Source:   "wave2:s3",
			},
		},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			carriedDetailCode: {
				Rows: []domain.DetailRow{
					{Label: "Action", Value: "system-update", Tier: "~"},
				},
			},
		},
	}
	// s3-bucket-y carries its own fresh Wave-2 finding and details; the carry only
	// backfills rows missing their own Wave-2 entry.
	freshOwnDetailCode := domain.FindingCode("s3-encryption-disabled")

	c.Session().RowStore.Observe("s3", []resource.Resource{enrichedRow}, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)
	c.Session().AvailChecked = 0
	c.Session().AvailTotal = 1
	c.Session().AvailQueue = nil

	bareRow := resource.Resource{
		ID:   "s3-bucket-x",
		Name: "s3-bucket-x",
		Type: "s3",
	}
	freshWithOwnWave2 := resource.Resource{
		ID:   "s3-bucket-y",
		Name: "s3-bucket-y",
		Type: "s3",
		Findings: []domain.Finding{
			{
				Code:     freshOwnDetailCode,
				Phrase:   "encryption disabled",
				Severity: domain.SevBroken,
				Source:   "wave2:s3",
			},
		},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			freshOwnDetailCode: {
				Rows: []domain.DetailRow{
					{Label: "Action", Value: "enable-encryption", Tier: "!"},
				},
			},
		},
	}
	_, _ = c.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        2,
		Truncated:    false,
		Resources:    []resource.Resource{bareRow, freshWithOwnWave2},
		Gen:          c.AvailabilityGen(),
	})

	snap := c.Session().RowStore.Snapshot("s3")
	if len(snap.Rows) != 2 {
		t.Fatalf("RowStore.Snapshot(\"s3\").Rows has %d entries, want 2", len(snap.Rows))
	}

	rowX, okX := findResourceByID(snap.Rows, "s3-bucket-x")
	if !okX {
		t.Fatal("RowStore.Snapshot(\"s3\") missing s3-bucket-x")
	}
	if len(rowX.Findings) != 1 || rowX.Findings[0].Code != carriedDetailCode {
		t.Fatalf("carried s3-bucket-x.Findings = %+v, want 1 finding with Code=%q", rowX.Findings, carriedDetailCode)
	}
	adX, ok := rowX.AttentionDetails[carriedDetailCode]
	if !ok {
		t.Fatalf("carried s3-bucket-x.AttentionDetails missing entry for Code=%q — a bare Wave-1 probe result must not blank the carried AttentionDetails alongside the carried finding (C6b)", carriedDetailCode)
	}
	if len(adX.Rows) != 1 || adX.Rows[0].Label != "Action" || adX.Rows[0].Value != "system-update" || adX.Rows[0].Tier != "~" {
		t.Errorf("carried s3-bucket-x.AttentionDetails[%q].Rows = %+v, want [{Action system-update ~}]", carriedDetailCode, adX.Rows)
	}

	rowY, okY := findResourceByID(snap.Rows, "s3-bucket-y")
	if !okY {
		t.Fatal("RowStore.Snapshot(\"s3\") missing s3-bucket-y")
	}
	if len(rowY.Findings) != 1 || rowY.Findings[0].Code != freshOwnDetailCode {
		t.Fatalf("s3-bucket-y.Findings = %+v, want its OWN fresh finding with Code=%q (not clobbered by any carry)", rowY.Findings, freshOwnDetailCode)
	}
	adY, ok := rowY.AttentionDetails[freshOwnDetailCode]
	if !ok {
		t.Fatalf("s3-bucket-y.AttentionDetails missing its OWN fresh entry for Code=%q", freshOwnDetailCode)
	}
	if len(adY.Rows) != 1 || adY.Rows[0].Label != "Action" || adY.Rows[0].Value != "enable-encryption" || adY.Rows[0].Tier != "!" {
		t.Errorf("s3-bucket-y.AttentionDetails[%q].Rows = %+v, want [{Action enable-encryption !}]", freshOwnDetailCode, adY.Rows)
	}
	if _, staleCarried := rowY.AttentionDetails[carriedDetailCode]; staleCarried {
		t.Errorf("s3-bucket-y.AttentionDetails unexpectedly carries Code=%q from an unrelated row — a row with its own fresh Wave-2 finding must never receive grafted stale details", carriedDetailCode)
	}
}

// findResourceByID returns the resource.Resource with the given ID, or
// (resource.Resource{}, false).
func findResourceByID(rows []resource.Resource, id string) (resource.Resource, bool) {
	for _, r := range rows {
		if r.ID == id {
			return r, true
		}
	}
	return resource.Resource{}, false
}
