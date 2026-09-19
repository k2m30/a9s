// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestHandle_StaleResourcesLoaded_Dropped verifies that Handle discards a
// ResourcesLoaded whose Gen does not match the session's AvailabilityGen.
func TestHandle_StaleResourcesLoaded_Dropped(t *testing.T) {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	// Seed three rows via Gen=0 (AcceptZeroGen=true passes the staleness guard).
	_, _ = c.Handle(messages.ResourcesLoaded{ //nolint:ineffassign,staticcheck // return values intentionally ignored
		ResourceType: "ec2",
		Resources:    fakeEC2Resources(), Provenance: messages.FetchProvenanceCanonicalList,
	})
	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 3 {
		t.Fatalf("precondition: want 3 rows after fresh load, got %d", len(lb.Rows))
	}

	// Gen=99 is stale because session seeds AvailabilityGen=1 (99 != 1).
	_, _ = c.Handle(messages.ResourcesLoaded{ //nolint:ineffassign,staticcheck // return values intentionally ignored
		ResourceType: "ec2",
		Resources:    fakeEC2Resources()[:1],
		Gen:          domain.Gen(99), Provenance: messages.FetchProvenanceCanonicalList,
	})
	lb2 := listBodyOrFail(t, c)
	if len(lb2.Rows) != 3 {
		t.Fatalf("Fix1: stale ResourcesLoaded (Gen=99) was applied — rows went from 3 to %d; Handle must guard with IsStale", len(lb2.Rows))
	}

	// Sanity: Gen=0 (AcceptZeroGen sentinel) IS applied even though session gen=1.
	_, _ = c.Handle(messages.ResourcesLoaded{ //nolint:ineffassign,staticcheck // return values intentionally ignored
		ResourceType: "ec2",
		Resources:    fakeEC2Resources()[:1],
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})
	lb3 := listBodyOrFail(t, c)
	if len(lb3.Rows) != 1 {
		t.Fatalf("Fix1: fresh ResourcesLoaded (Gen=0) should shrink list to 1; got %d", len(lb3.Rows))
	}
}

// TestApplyListFieldUpdates_UpdatesBothStackedLists verifies that field updates
// are applied to all same-type list screens on the stack, not only the top one.
func TestApplyListFieldUpdates_UpdatesBothStackedLists(t *testing.T) {
	targetID := fakeEC2Resources()[0].ID // "i-0aaa111111111111a"

	c := newListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 3 {
		t.Fatalf("precondition: want 3 rows, got %d", len(lb.Rows))
	}

	c.PushChildListScreen("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)

	lb2 := listBodyOrFail(t, c)
	if len(lb2.Rows) != 3 {
		t.Fatalf("precondition screen2: want 3 rows, got %d", len(lb2.Rows))
	}

	c.ApplyListFieldUpdates("ec2", map[string]map[string]string{
		targetID: {"state": "terminated"},
	})

	topRes := c.GetListVisibleResources()
	updatedTop := ""
	for _, r := range topRes {
		if r.ID == targetID {
			updatedTop = r.Fields["state"]
			break
		}
	}
	if updatedTop != "terminated" {
		t.Errorf("Fix2: screen2 resource %q Fields[state]=%q, want %q", targetID, updatedTop, "terminated")
	}

	c.Apply(app.Action{Kind: app.ActionBack})

	underlyingRes := c.GetListVisibleResources()
	updatedUnderlying := ""
	for _, r := range underlyingRes {
		if r.ID == targetID {
			updatedUnderlying = r.Fields["state"]
			break
		}
	}
	if updatedUnderlying != "terminated" {
		t.Errorf("Fix2: screen1 (underlying) resource %q Fields[state]=%q, want %q after pop — ApplyListFieldUpdates did not propagate to the stacked list", targetID, updatedUnderlying, "terminated")
	}
}

// TestMenuSelected_ClampsCursorWhenVisibleShrinks verifies that MenuSelected
// returns ok=true when the cursor position is beyond the visible list (because
// an attention filter reduced the visible count after the cursor moved).
func TestMenuSelected_ClampsCursorWhenVisibleShrinks(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 4 {
		t.Skip("need >= 4 resource types")
	}

	c := newBaseController(t)

	// Inject issue counts for exactly 2 resource types so the attention filter
	// produces a visible list of 2 entries.
	type1 := allTypes[0].ShortName
	type2 := allTypes[1].ShortName

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenu{ResourceType: type1, Issues: 1},
		runtime.PatchMenu{ResourceType: type2, Issues: 1},
	})

	// Move cursor well past position 1 so it will be >= len(visible)==2.
	for i := 0; i < 5; i++ {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}

	vs := c.Snapshot()
	if vs.Body.Menu == nil {
		t.Fatal("precondition: not on menu screen")
	}
	cursorBefore := vs.Body.Menu.Selected

	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	vs2 := c.Snapshot()
	if vs2.Body.Menu == nil {
		t.Fatal("menu body nil after attention toggle")
	}
	visible := vs2.Body.Menu.Entries
	if len(visible) == 0 {
		t.Skip("attention filter produced 0 entries — cannot test clamp")
	}

	td, ok := c.MenuSelected()
	if !ok {
		t.Fatalf("Fix3: MenuSelected returned ok=false (cursorBefore=%d, visible=%d); must clamp to last entry", cursorBefore, len(visible))
	}

	found := false
	for _, e := range visible {
		if e.ShortName == td.ShortName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Fix3: MenuSelected returned %q which is not in visible entries — cursor was not clamped correctly", td.ShortName)
	}
}

// newControllerAtDetail pushes a list then a detail screen for res/resourceType
// and calls EnsureDetailState so Snapshot().Body.Detail is non-nil.
func newControllerAtDetail(t *testing.T, res resource.Resource, resourceType string) *app.Controller {
	t.Helper()
	c := newListController(t, resourceType)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: resourceType, ResourceID: res.ID},
		},
	})
	c.EnsureDetailState(res, resourceType)
	return c
}

// attentionFieldRows returns the FieldRows from body.Fields with Path=="Attention".
func attentionFieldRows(body *app.DetailBody) []app.FieldRow {
	var out []app.FieldRow
	for _, f := range body.Fields {
		if f.Path == "Attention" {
			out = append(out, f)
		}
	}
	return out
}

// TestApplyDetailEnrichmentForResource_UpdatesDetailFields verifies that
// ApplyDetailEnrichmentForResource replaces ds.Resource with the enriched
// resource so the detail body projects the enriched fields.
func TestApplyDetailEnrichmentForResource_UpdatesDetailFields(t *testing.T) {
	baseRes := resource.Resource{
		ID:   "i-0aaa111111111111a",
		Name: "web-server",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0aaa111111111111a",
			"state":       "running",
		},
	}
	enrichedRes := resource.Resource{
		ID:   "i-0aaa111111111111a",
		Name: "web-server",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id":     "i-0aaa111111111111a",
			"state":           "running",
			"iam_profile_arn": "arn:aws:iam::555555555555:instance-profile/web-role",
		},
	}
	finding := &domain.Finding{
		Code:     "ec2.iam-enrichment",
		Phrase:   "IAM profile enriched",
		Severity: domain.SevWarn,
		Source:   "wave2:ec2-enricher",
	}

	c := newControllerAtDetail(t, baseRes, "ec2")

	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatal("precondition: Body.Detail is nil")
	}

	c.ApplyDetailEnrichmentForResource("ec2", baseRes.ID, enrichedRes, finding, nil)

	vs2 := c.Snapshot()
	if vs2.Body.Detail == nil {
		t.Fatal("Fix4: Body.Detail became nil after enrichment")
	}

	enrichedValueFound := false
	for _, f := range vs2.Body.Detail.Fields {
		if strings.Contains(f.Value, "web-role") || strings.Contains(f.Key, "iam_profile") {
			enrichedValueFound = true
			break
		}
	}
	if !enrichedValueFound {
		t.Errorf("Fix4: enriched value 'web-role' / key 'iam_profile_arn' not found in Detail.Fields — ds.Resource was not replaced by ApplyDetailEnrichmentForResource")
	}
}

// TestApplyDetailEnrichmentForResource_TargetsStackedDetail verifies that
// enrichment reaches a detail screen stacked beneath the active screen.
func TestApplyDetailEnrichmentForResource_TargetsStackedDetail(t *testing.T) {
	resA := resource.Resource{
		ID:   "i-0aaa111111111111a",
		Name: "instance-A",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0aaa111111111111a",
			"state":       "running",
		},
	}
	resB := resource.Resource{
		ID:   "i-0bbb222222222222b",
		Name: "instance-B",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0bbb222222222222b",
			"state":       "stopped",
		},
	}

	c := newControllerAtDetail(t, resA, "ec2")

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: resB.ID},
		},
	})
	c.EnsureDetailState(resB, "ec2")

	enrichedA := resource.Resource{
		ID:   resA.ID,
		Name: resA.Name,
		Type: "ec2",
		Fields: map[string]string{
			"instance_id":     resA.ID,
			"state":           "running",
			"iam_profile_arn": "arn:aws:iam::555555555555:instance-profile/role-for-A",
		},
	}
	finding := &domain.Finding{
		Code:     "ec2.stacked-enrich",
		Phrase:   "stacked enrichment applied",
		Severity: domain.SevWarn,
		Source:   "wave2:test",
	}

	c.ApplyDetailEnrichmentForResource("ec2", resA.ID, enrichedA, finding, nil)

	c.Apply(app.Action{Kind: app.ActionBack})

	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatal("Fix4 stacked: Body.Detail is nil after pop to resA detail")
	}

	enrichedValueFound := false
	for _, f := range vs.Body.Detail.Fields {
		if strings.Contains(f.Value, "role-for-A") {
			enrichedValueFound = true
			break
		}
	}
	if !enrichedValueFound {
		t.Errorf("Fix4 stacked: enriched value 'role-for-A' not found in resA detail Fields after pop — ApplyDetailEnrichmentForResource did not update the stacked detail")
	}
}

// TestApplyDetailFindingForResource_LandsInAttentionBlock verifies that
// applying a wave-2 finding results in an Attention section in the Fields slice.
func TestApplyDetailFindingForResource_LandsInAttentionBlock(t *testing.T) {
	res := resource.Resource{
		ID:   "i-0aaa111111111111a",
		Name: "web-server",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0aaa111111111111a",
			"state":       "impaired",
		},
	}

	c := newControllerAtDetail(t, res, "ec2")

	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatal("precondition: Body.Detail is nil")
	}
	if rows := attentionFieldRows(vs.Body.Detail); len(rows) != 0 {
		t.Fatalf("precondition: want 0 attention rows before finding, got %d", len(rows))
	}

	wave2 := domain.Finding{
		Code:     "ec2.status-impaired",
		Phrase:   "instance status check failed",
		Severity: domain.SevBroken,
		Source:   "wave2:ec2-status",
	}
	c.ApplyDetailFindingForResource("ec2", res.ID, &wave2, nil)

	vs2 := c.Snapshot()
	if vs2.Body.Detail == nil {
		t.Fatal("Fix5: Body.Detail is nil after applying finding")
	}

	attn := attentionFieldRows(vs2.Body.Detail)
	if len(attn) == 0 {
		t.Errorf("Fix5: Attention section absent from Fields after ApplyDetailFindingForResource — applyFindingToState stripped the finding")
	}

	phraseFound := false
	for _, row := range attn {
		if strings.Contains(row.Value, "status check") || strings.Contains(row.Key, "status check") {
			phraseFound = true
			break
		}
	}
	if !phraseFound && len(attn) > 0 {
		t.Errorf("Fix5: attention rows present but phrase %q not found in any row", wave2.Phrase)
	}
}

// TestApplyDetailFindingForResource_SecondApplyReplacesFirst verifies that
// a second wave-2 finding from the same source replaces the first. The Attention
// block must contain the second finding but not the first.
func TestApplyDetailFindingForResource_SecondApplyReplacesFirst(t *testing.T) {
	res := resource.Resource{
		ID:   "i-0aaa111111111111a",
		Name: "web-server",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0aaa111111111111a",
			"state":       "running",
		},
	}

	c := newControllerAtDetail(t, res, "ec2")

	first := domain.Finding{
		Code:     "ec2.first-finding",
		Phrase:   "first enrichment finding",
		Severity: domain.SevWarn,
		Source:   "wave2:ec2-enricher",
	}
	second := domain.Finding{
		Code:     "ec2.second-finding",
		Phrase:   "second enrichment finding",
		Severity: domain.SevWarn,
		Source:   "wave2:ec2-enricher",
	}

	c.ApplyDetailFindingForResource("ec2", res.ID, &first, nil)
	c.ApplyDetailFindingForResource("ec2", res.ID, &second, nil)

	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatal("Fix5 replace: Body.Detail is nil")
	}

	attn := attentionFieldRows(vs.Body.Detail)
	if len(attn) == 0 {
		t.Fatalf("Fix5 replace: no attention rows after second apply")
	}

	firstFound := false
	secondFound := false
	for _, row := range attn {
		if strings.Contains(row.Value, "first enrichment") || strings.Contains(row.Key, "first enrichment") {
			firstFound = true
		}
		if strings.Contains(row.Value, "second enrichment") || strings.Contains(row.Key, "second enrichment") {
			secondFound = true
		}
	}
	if firstFound {
		t.Errorf("Fix5 replace: first finding still in Attention after second apply — strip did not remove prior wave-2 entry")
	}
	if !secondFound {
		t.Errorf("Fix5 replace: second finding not in Attention after replace")
	}
}

// TestApplyDetailFindingForResource_ResourceFieldsPreserved verifies that
// applying a wave-2 finding does not destroy the resource's own field projections.
func TestApplyDetailFindingForResource_ResourceFieldsPreserved(t *testing.T) {
	res := resource.Resource{
		ID:   "i-0aaa111111111111a",
		Name: "web-server",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0aaa111111111111a",
			"state":       "impaired",
			"type":        "t3.large",
		},
	}

	c := newControllerAtDetail(t, res, "ec2")

	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatal("precondition: Body.Detail is nil")
	}
	fieldsBefore := len(vs.Body.Detail.Fields)
	if fieldsBefore == 0 {
		t.Fatal("precondition: no fields before finding — projector not running")
	}

	wave2 := domain.Finding{
		Code:     "ec2.status-impaired",
		Phrase:   "instance status check failed",
		Severity: domain.SevBroken,
		Source:   "wave2:ec2-status",
	}
	c.ApplyDetailFindingForResource("ec2", res.ID, &wave2, nil)

	vs2 := c.Snapshot()
	if vs2.Body.Detail == nil {
		t.Fatal("Fix5 fields: Body.Detail nil after applying wave-2 finding")
	}

	fieldsAfter := len(vs2.Body.Detail.Fields)
	if fieldsAfter < fieldsBefore {
		t.Errorf("Fix5 fields: field count dropped from %d to %d — resource fields lost after wave-2 apply", fieldsBefore, fieldsAfter)
	}

	stateFound := false
	for _, f := range vs2.Body.Detail.Fields {
		if strings.Contains(f.Value, "impaired") {
			stateFound = true
			break
		}
	}
	if !stateFound {
		t.Errorf("Fix5 fields: resource field 'state=impaired' lost after wave-2 finding — applyFindingToState corrupted ds.Resource")
	}
}

// attentionContains reports whether any Attention FieldRow contains substr in
// its Key or Value.
func attentionContains(body *app.DetailBody, substr string) bool {
	for _, f := range attentionFieldRows(body) {
		if strings.Contains(f.Value, substr) || strings.Contains(f.Key, substr) {
			return true
		}
	}
	return false
}

// TestApplyDetailFinding_PreservesWave1Findings verifies that when a
// resource already carries wave-1 (fetcher-emitted) findings and a wave-2
// enrichment finding is applied, BOTH remain in the Attention block.
func TestApplyDetailFinding_PreservesWave1Findings(t *testing.T) {
	res := resource.Resource{
		ID:   "i-0aaa111111111111a",
		Name: "web-server",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0aaa111111111111a",
			"state":       "running",
		},
		Findings: []domain.Finding{{
			Code:     "ec2.wave1-issue",
			Phrase:   "wave one fetcher issue",
			Severity: domain.SevBroken,
			Source:   "ec2-fetcher", // NOT "wave2:"-prefixed — this is a wave-1 finding
		}},
	}

	c := newControllerAtDetail(t, res, "ec2")

	vs0 := c.Snapshot()
	if vs0.Body.Detail == nil {
		t.Fatal("precondition: Body.Detail is nil")
	}
	if !attentionContains(vs0.Body.Detail, "wave one") {
		t.Fatalf("precondition: wave-1 finding not in Attention before enrichment")
	}

	wave2 := domain.Finding{
		Code:     "ec2.wave2-issue",
		Phrase:   "wave two enrichment issue",
		Severity: domain.SevWarn,
		Source:   "wave2:ec2-enricher",
	}
	c.ApplyDetailFindingForResource("ec2", res.ID, &wave2, nil)

	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatal("Body.Detail nil after wave-2 apply")
	}
	if !attentionContains(vs.Body.Detail, "wave two") {
		t.Errorf("P2-4: wave-2 finding missing from Attention after apply")
	}
	if !attentionContains(vs.Body.Detail, "wave one") {
		t.Errorf("P2-4: wave-1 finding LOST from Attention after wave-2 apply — buildDetailFieldItems replaced r.Findings instead of merging wave-1 + wave-2")
	}
}

// TestController_APIErrorClearsListLoadingAndFlashes verifies that
// Handle(messages.APIError) applies the intents core.HandleAPIError returns:
// ClearActiveListLoadingIntent (clears Loading) and FlashIntent (surfaces
// Header.Flash).
func TestController_APIErrorClearsListLoadingAndFlashes(t *testing.T) {
	c := newListController(t, "ec2")

	// Precondition: list screen exists and Loading is true (fetch not yet drained).
	pre := c.Snapshot()
	if pre.Body.List == nil {
		t.Fatal("precondition: Body.List is nil — controller not on a list screen")
	}
	if !pre.Body.List.Loading {
		t.Fatal("precondition: Body.List.Loading is false — test is not meaningful unless Loading starts true")
	}

	// Deliver the failed fetch. Gen=0 satisfies AcceptZeroGen=true so IsStale
	// never discards this message regardless of session AvailabilityGen.
	c.Handle(messages.APIError{ //nolint:ineffassign,staticcheck // return values intentionally ignored; asserting via Snapshot
		ResourceType: "ec2",
		Err:          errors.New("AccessDeniedException: User is not authorized to perform ec2:DescribeInstances"),
		Gen:          0,
	})

	vs := c.Snapshot()

	if vs.Body.List == nil {
		t.Fatal("Body.List is nil after Handle(APIError) — list screen was unexpectedly popped")
	}
	if vs.Body.List.Loading {
		t.Error("APIError left list stuck Loading=true — review P2 regression: ClearActiveListLoadingIntent was not applied by Handle(APIError)")
	}

	if !vs.Header.Flash.IsError {
		t.Error("APIError did not set Header.Flash.IsError=true — review P2 regression: FlashIntent was not applied by Handle(APIError)")
	}
	if vs.Header.Flash.Text == "" {
		t.Error("APIError produced an empty Header.Flash.Text — review P2 regression: FlashIntent was applied but carried no error message")
	}
}
