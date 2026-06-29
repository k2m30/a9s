// controller_regression_test.go — regression tests for five controller-path bugs
// fixed in the headless/PR-C work.
//
// Each test is annotated with the pre-fix failure so it is clear why the test
// would have failed before the corresponding fix.
//
//   Fix1 (P1-2): Handle silently dropped stale ResourcesLoaded (IsStale guard).
//   Fix2 (P2-3): ApplyListFieldUpdates now applies to every matching stack list.
//   Fix3 (P2-5): MenuSelected clamps cursor when the visible list shrinks.
//   Fix4 (P2-1): ApplyDetailEnrichmentForResource sets ds.Resource on the match.
//   Fix5 (P2-4): ensureDetailState seeds ds.Findings with the resource's wave-1
//                findings so the Attention section shows them — guarded by
//                TestApplyDetailFinding_PreservesWave1Findings.
//
// The TestApplyDetailFindingForResource_* tests below are general regression
// tests for applyFindingToState's wave-2 strip/append behaviour and the
// finding→Attention flow. They are NOT P2-4 guards (a resource with no wave-1
// findings renders identically before and after the P2-4 fix); the dedicated
// P2-4 guard is TestApplyDetailFinding_PreservesWave1Findings.
package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fix 1 (P1-2): Stale ResourcesLoaded must be dropped by Handle
// ─────────────────────────────────────────────────────────────────────────────

// TestHandle_StaleResourcesLoaded_Dropped verifies that Handle discards a
// ResourcesLoaded whose Gen does not match the session's AvailabilityGen.
//
// Pre-fix failure: Handle called handleResourcesLoadedEvent unconditionally,
// so the stale one-row message overwrote the three-row list to one row.
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
		Resources:    fakeEC2Resources(),
	})
	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 3 {
		t.Fatalf("precondition: want 3 rows after fresh load, got %d", len(lb.Rows))
	}

	// Gen=99 is stale because session seeds AvailabilityGen=1 (99 != 1).
	// The message carries only 1 resource — if the stale guard is absent it
	// overwrites the three-row list and the assertion below fails.
	_, _ = c.Handle(messages.ResourcesLoaded{ //nolint:ineffassign,staticcheck // return values intentionally ignored
		ResourceType: "ec2",
		Resources:    fakeEC2Resources()[:1],
		Gen:          domain.Gen(99),
	})
	lb2 := listBodyOrFail(t, c)
	if len(lb2.Rows) != 3 {
		t.Fatalf("Fix1: stale ResourcesLoaded (Gen=99) was applied — rows went from 3 to %d; Handle must guard with IsStale", len(lb2.Rows))
	}

	// Sanity: Gen=0 (AcceptZeroGen sentinel) IS applied even though session gen=1.
	_, _ = c.Handle(messages.ResourcesLoaded{ //nolint:ineffassign,staticcheck // return values intentionally ignored
		ResourceType: "ec2",
		Resources:    fakeEC2Resources()[:1],
		Gen:          0,
	})
	lb3 := listBodyOrFail(t, c)
	if len(lb3.Rows) != 1 {
		t.Fatalf("Fix1: fresh ResourcesLoaded (Gen=0) should shrink list to 1; got %d", len(lb3.Rows))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Fix 2 (P2-3): ApplyListFieldUpdates applies to every matching stack list
// ─────────────────────────────────────────────────────────────────────────────

// TestApplyListFieldUpdates_UpdatesBothStackedLists verifies that field updates
// are applied to all same-type list screens on the stack, not only the top one.
//
// Pre-fix failure: only the top list's Rows were mutated. After popping back to
// the underlying list, the old field value was still shown.
func TestApplyListFieldUpdates_UpdatesBothStackedLists(t *testing.T) {
	targetID := fakeEC2Resources()[0].ID // "i-0aaa111111111111a"

	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 3 {
		t.Fatalf("precondition: want 3 rows, got %d", len(lb.Rows))
	}

	// Push a child list of the same type and seed it with the same rows.
	c.PushChildListScreen("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)

	lb2 := listBodyOrFail(t, c)
	if len(lb2.Rows) != 3 {
		t.Fatalf("precondition screen2: want 3 rows, got %d", len(lb2.Rows))
	}

	// Apply field update targeting the resource on both screens.
	c.ApplyListFieldUpdates("ec2", map[string]map[string]string{
		targetID: {"state": "terminated"},
	})

	// Top list (screen 2) must reflect the update.
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

	// Pop to screen 1 and assert the underlying list also has the update.
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

// ─────────────────────────────────────────────────────────────────────────────
// Fix 3 (P2-5): MenuSelected clamps cursor when visible list shrinks
// ─────────────────────────────────────────────────────────────────────────────

// TestMenuSelected_ClampsCursorWhenVisibleShrinks verifies that MenuSelected
// returns ok=true when the cursor position is beyond the visible list (because
// an attention filter reduced the visible count after the cursor moved).
//
// Pre-fix failure: MenuSelected returned ok=false when cursor >= len(visible),
// making Enter a no-op on the highlighted last item.
func TestMenuSelected_ClampsCursorWhenVisibleShrinks(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 4 {
		t.Skip("need >= 4 resource types")
	}

	c := newBaseController()

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

	// Enable attention-only filter; visible list shrinks to the 2 types with issues.
	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	vs2 := c.Snapshot()
	if vs2.Body.Menu == nil {
		t.Fatal("menu body nil after attention toggle")
	}
	visible := vs2.Body.Menu.Entries
	if len(visible) == 0 {
		t.Skip("attention filter produced 0 entries — cannot test clamp")
	}

	// MenuSelected must return ok=true regardless of cursor position.
	td, ok := c.MenuSelected()
	if !ok {
		t.Fatalf("Fix3: MenuSelected returned ok=false (cursorBefore=%d, visible=%d); must clamp to last entry", cursorBefore, len(visible))
	}

	// The returned type must be one of the visible entries.
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

// ─────────────────────────────────────────────────────────────────────────────
// helpers for detail tests
// ─────────────────────────────────────────────────────────────────────────────

// newControllerAtDetail pushes a list then a detail screen for res/resourceType
// and calls EnsureDetailState so Snapshot().Body.Detail is non-nil.
func newControllerAtDetail(res resource.Resource, resourceType string) *app.Controller {
	c := newListController(resourceType)
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

// ─────────────────────────────────────────────────────────────────────────────
// Fix 4 (P2-1): ApplyDetailEnrichmentForResource replaces ds.Resource
// ─────────────────────────────────────────────────────────────────────────────

// TestApplyDetailEnrichmentForResource_UpdatesDetailFields verifies that
// ApplyDetailEnrichmentForResource replaces ds.Resource with the enriched
// resource so the detail body projects the enriched fields.
//
// Pre-fix failure: only the wave-2 finding was stored; ds.Resource was
// unchanged, so the projection still showed pre-enrichment fields.
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

	c := newControllerAtDetail(baseRes, "ec2")

	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatal("precondition: Body.Detail is nil")
	}

	c.ApplyDetailEnrichmentForResource("ec2", baseRes.ID, enrichedRes, finding, nil)

	vs2 := c.Snapshot()
	if vs2.Body.Detail == nil {
		t.Fatal("Fix4: Body.Detail became nil after enrichment")
	}

	// The enriched field "iam_profile_arn" / "web-role" must appear in Fields.
	// Without ds.Resource replacement the projector only sees the base resource
	// and the IAM value is absent.
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
//
// Pre-fix failure: without iterating the full stack, the matching underlying
// detail was never updated.
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

	c := newControllerAtDetail(resA, "ec2")

	// Push a second detail for resB on top of resA's detail.
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

	// Enrich resA while resB is the top screen.
	c.ApplyDetailEnrichmentForResource("ec2", resA.ID, enrichedA, finding, nil)

	// Pop back to resA's detail and verify the enrichment landed.
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

// ─────────────────────────────────────────────────────────────────────────────
// Fix 5 (P2-4): Wave-2 finding lands in Attention block and replaces correctly
// ─────────────────────────────────────────────────────────────────────────────

// TestApplyDetailFindingForResource_LandsInAttentionBlock verifies that
// applying a wave-2 finding results in an Attention section in the Fields slice.
// General regression test for applyFindingToState — guards against a future
// regression where the wave-2 finding fails to reach the Attention block.
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

	c := newControllerAtDetail(res, "ec2")

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
// block must contain the second finding but not the first. General regression
// test for applyFindingToState's wave-2 strip-then-append behaviour.
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

	c := newControllerAtDetail(res, "ec2")

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
//
// Pre-fix failure: an early version of applyFindingToState cleared ds.Resource
// or corrupted state such that the field list became empty.
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

	c := newControllerAtDetail(res, "ec2")

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

	// After the finding the field count must be >= before (attention rows added).
	fieldsAfter := len(vs2.Body.Detail.Fields)
	if fieldsAfter < fieldsBefore {
		t.Errorf("Fix5 fields: field count dropped from %d to %d — resource fields lost after wave-2 apply", fieldsBefore, fieldsAfter)
	}

	// The original resource field "state=impaired" must still appear.
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

// TestApplyDetailFinding_PreservesWave1Findings is the actual P2-4 guard: when a
// resource already carries wave-1 (fetcher-emitted) findings and a wave-2
// enrichment finding is applied, BOTH must remain in the Attention block.
//
// Pre-fix failure: buildDetailFieldItems did `r.Findings = ds.Findings`, which
// overwrote the resource's wave-1 findings with only the wave-2 entry, so the
// wave-1 issue vanished from the Attention section / status projection. The fix
// merges wave-1 (non-"wave2:" Source) with ds.Findings (wave-2). The other Fix-5
// tests use resources with no wave-1 findings, so they pass on both code paths
// and do NOT exercise this merge.
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

	c := newControllerAtDetail(res, "ec2")

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

// ─────────────────────────────────────────────────────────────────────────────
// Fix P2 (APIError): Handle(APIError) must clear Loading and surface a flash
// ─────────────────────────────────────────────────────────────────────────────

// TestController_APIErrorClearsListLoadingAndFlashes is the regression guard for
// the headless-controller APIError handling path added in commit 56910d32.
//
// Pre-fix failure: Controller.Handle dropped messages.APIError entirely, so the
// active list screen stayed stuck with Loading=true and no error flash was set.
// The fix routes APIError through core.HandleAPIError and applies the returned
// intents: ClearActiveListLoadingIntent (clears Loading) and FlashIntent
// (surfaces Header.Flash).
//
// Test strategy:
//  1. Navigate to an ec2 list screen (Loading=true, no resources seeded yet).
//  2. Assert the precondition: Loading==true so the test is meaningful.
//  3. Deliver messages.APIError with Gen=0 (AcceptZeroGen=true — never stale).
//  4. Assert both intents were applied: Loading==false and Header.Flash is an error.
func TestController_APIErrorClearsListLoadingAndFlashes(t *testing.T) {
	c := newListController("ec2")

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

	// Assert Fix 1: ClearActiveListLoadingIntent must have cleared Loading.
	if vs.Body.List == nil {
		t.Fatal("Body.List is nil after Handle(APIError) — list screen was unexpectedly popped")
	}
	if vs.Body.List.Loading {
		t.Error("APIError left list stuck Loading=true — review P2 regression: ClearActiveListLoadingIntent was not applied by Handle(APIError)")
	}

	// Assert Fix 2: FlashIntent must have surfaced an error flash in the header.
	if !vs.Header.Flash.IsError {
		t.Error("APIError did not set Header.Flash.IsError=true — review P2 regression: FlashIntent was not applied by Handle(APIError)")
	}
	if vs.Header.Flash.Text == "" {
		t.Error("APIError produced an empty Header.Flash.Text — review P2 regression: FlashIntent was applied but carried no error message")
	}
}
