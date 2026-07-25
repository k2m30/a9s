package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// rolePolicyRes builds a minimal role_policies resource for enrich tests.
func rolePolicyRes(id, name, policyType string) resource.Resource {
	arn := ""
	if policyType == "Managed" {
		arn = id
	}
	return resource.Resource{
		ID:   id,
		Name: name,
		Fields: map[string]string{
			"policy_name": name,
			"policy_arn":  arn,
			"policy_type": policyType,
			"role_name":   "test-role",
		},
		RawStruct: awsclient.RolePolicyRow{
			PolicyName: name,
			PolicyArn:  arn,
			PolicyType: policyType,
		},
	}
}

// withDocument returns a copy of res with the given document injected into RawStruct.
func withDocument(res resource.Resource, doc map[string]any) resource.Resource {
	enriched := res
	row := enriched.RawStruct.(awsclient.RolePolicyRow)
	row.Document = doc
	enriched.RawStruct = row
	return enriched
}

func newEnrichApp() tui.Model {
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// ---------------------------------------------------------------------------
// TestDetailView_EnrichResult_UpdatesRawStruct
// Verifies that after EnrichDetailResultMsg, the enriched resource (with
// Document) is available in the YAML view content rendered from RawStruct.
// ---------------------------------------------------------------------------

func TestDetailView_EnrichResult_UpdatesRawStruct(t *testing.T) {
	// Test via YAML model directly — YAML view renders from RawStruct
	// and will include Document if present.
	k := keys.Default()

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/test-policy", "test-policy", "Managed")
	enrichedRes := withDocument(res, map[string]any{
		"Version": "2012-10-17",
		"Statement": []any{
			map[string]any{
				"Effect":   "Allow",
				"Action":   "s3:GetObject",
				"Resource": "arn:aws:s3:::my-bucket/*",
			},
		},
	})

	y := views.NewYAMLWithCtrl(enrichedRes, "role_policies", k, nil)
	y.SetSize(120, 40)

	content := stripANSI(strings.Join(y.ContentLines(), "\n"))
	if !strings.Contains(content, "Document") {
		t.Error("YAML view should render Document field from enriched RawStruct")
	}
	if !strings.Contains(content, "2012-10-17") {
		t.Error("YAML view should render policy Version 2012-10-17 from Document")
	}
	if !strings.Contains(content, "Allow") {
		t.Error("YAML view should render Allow effect from Document's Statement")
	}
}

// ---------------------------------------------------------------------------
// TestDetailView_EnrichResult_YAMLViewShowsDocument
// Verifies that after enrichment the YAML view contains Document content.
// Uses YAMLModel directly since pressing 'y' returns a cmd that must be
// driven separately to create the YAML view with the enriched resource.
// ---------------------------------------------------------------------------

func TestDetailView_EnrichResult_YAMLViewShowsDocument(t *testing.T) {
	k := keys.Default()

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/yaml-test-policy", "yaml-test-policy", "Managed")
	enrichedRes := withDocument(res, map[string]any{
		"Version": "2012-10-17",
		"Statement": []any{
			map[string]any{"Effect": "Allow", "Action": "logs:*", "Resource": "*"},
		},
	})

	y := views.NewYAMLWithCtrl(enrichedRes, "role_policies", k, nil)
	y.SetSize(120, 40)

	content := stripANSI(strings.Join(y.ContentLines(), "\n"))
	if !strings.Contains(content, "Document") {
		t.Error("expected YAML view to contain Document section")
	}
	if !strings.Contains(content, "Statement") {
		t.Error("expected YAML view to contain Statement")
	}
}

// ---------------------------------------------------------------------------
// TestDetailView_EnrichResult_JSONViewShowsDocument
// Verifies that the JSON view renders Document from an enriched resource.
// ---------------------------------------------------------------------------

func TestDetailView_EnrichResult_JSONViewShowsDocument(t *testing.T) {
	k := keys.Default()

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/json-test-policy", "json-test-policy", "Managed")
	enrichedRes := withDocument(res, map[string]any{
		"Version": "2012-10-17",
		"Statement": []any{
			map[string]any{"Effect": "Deny", "Action": "*", "Resource": "*"},
		},
	})

	j := views.NewJSONWithCtrl(enrichedRes, "role_policies", k, nil)
	j.SetSize(120, 40)

	content := stripANSI(strings.Join(j.ContentLines(), "\n"))
	if !strings.Contains(content, "Document") {
		t.Error("expected JSON view to contain Document")
	}
	if !strings.Contains(content, "Deny") {
		t.Error("expected JSON view to contain Deny effect")
	}
}

// ---------------------------------------------------------------------------
// TestDetailView_EnrichResult_IgnoresMismatchedResourceID
// Verifies that an EnrichDetailResultMsg with a wrong resource ID is ignored
// by the app — the detail view stays at its original resource.
// ---------------------------------------------------------------------------

func TestDetailView_EnrichResult_IgnoresMismatchedResourceID(t *testing.T) {
	m := newEnrichApp()

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/my-policy", "my-policy", "Managed")

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Capture view before mismatch enrichment
	beforeContent := stripANSI(rootViewContent(m))

	// Send enrichment result with WRONG resource ID — should be silently ignored
	wrongRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "role_policies",
		ResourceID:   "wrong-id-that-does-not-match",
		EnrichedRes:  wrongRes,
	})

	afterContent := stripANSI(rootViewContent(m))

	// The view should be unchanged — mismatched ID was discarded
	if beforeContent != afterContent {
		t.Error("detail view should be unchanged when EnrichDetailResultMsg has mismatched resource ID")
	}
}

// ---------------------------------------------------------------------------
// TestApp_NavigateToRolePoliciesDetail_DispatchesEnrichment
// Verifies that navigating to role_policies detail returns a non-nil cmd,
// indicating the enrichment dispatch was triggered.
// ---------------------------------------------------------------------------

func TestApp_NavigateToRolePoliciesDetail_DispatchesEnrichment(t *testing.T) {
	m := newEnrichApp()

	// Verify role_policies has a detail enricher registered
	if !resource.HasDetailEnricher("role_policies") {
		t.Fatal("expected role_policies detail enricher to be registered")
	}

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/test", "test", "Managed")

	_, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	if cmd == nil {
		t.Fatal("expected a non-nil command after navigating to role_policies detail (enrichment dispatch)")
	}
}

// ---------------------------------------------------------------------------
// TestDetailView_EnrichResult_InlinePolicy_YAMLShowsDocument
// Verifies inline policy documents appear in YAML view after enrichment.
// ---------------------------------------------------------------------------

func TestDetailView_EnrichResult_InlinePolicy_YAMLShowsDocument(t *testing.T) {
	k := keys.Default()

	res := rolePolicyRes("deny-all", "deny-all", "Inline")
	enrichedRes := withDocument(res, map[string]any{
		"Version": "2012-10-17",
		"Statement": []any{
			map[string]any{"Effect": "Deny", "Action": "*", "Resource": "*"},
		},
	})

	y := views.NewYAMLWithCtrl(enrichedRes, "role_policies", k, nil)
	y.SetSize(120, 40)

	content := stripANSI(strings.Join(y.ContentLines(), "\n"))
	if !strings.Contains(content, "Document") {
		t.Error("expected YAML view to show Document section for inline policy after enrichment")
	}
	if !strings.Contains(content, "Inline") {
		t.Error("expected YAML view to show PolicyType Inline")
	}
}

// ---------------------------------------------------------------------------
// TestDetailView_EnrichResult_AcceptsMatchingID
// Verifies that EnrichDetailResultMsg with matching resource ID is accepted,
// i.e., the update call succeeds without panic.
// ---------------------------------------------------------------------------

func TestDetailView_EnrichResult_AcceptsMatchingID(t *testing.T) {
	m := newEnrichApp()

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/my-policy", "my-policy", "Managed")

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	enrichedRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	// Should not panic, update should be accepted
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "role_policies",
		ResourceID:   res.ID, // matching ID
		EnrichedRes:  enrichedRes,
	})

	// App should still render a view without error
	content := stripANSI(rootViewContent(m))
	if content == "" {
		t.Error("view should not be empty after enrichment with matching ID")
	}
	// Detail view should still show the policy fields
	if !strings.Contains(content, "my-policy") {
		t.Errorf("detail view should still show policy name after enrichment, got:\n%s", content)
	}
}

func TestEnrichResult_WrongResourceType_IsIgnored(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/rt-test", "rt-test", "Managed")
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Send enrichment result with WRONG resource type but matching ID
	enrichedRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "wrong-type",
		ResourceID:   res.ID,
		EnrichedRes:  enrichedRes,
	})

	content := stripANSI(rootViewContent(m))
	if strings.Contains(content, "2012-10-17") {
		t.Error("detail view should NOT show document from wrong resource type")
	}
}

func TestEnrichResult_ErrorShowsFlashMessage(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/err-test", "err-test", "Managed")
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Send enrichment result with an error. HandleEnrichDetailResult applies
	// the failure as a FlashIntent (core/runtime/handlers_resources.go).
	// internal/tui/app.go's messages.EnrichDetailResult case (handleEnrichDetailResult)
	// calls m.ctrl.Handle directly and forwards only the returned TaskRequests,
	// never the ViewState or its Flash — unlike Model.applyIntents
	// (app_dispatch.go), which re-emits a FlashIntent as a messages.Flash cmd.
	// The FlashIntent therefore never reaches m.flash (or a returned cmd), so
	// this test verifies the rendered view and stays RED until that TUI-side
	// gap is closed.
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "role_policies",
		ResourceID:   res.ID,
		Err:          fmt.Errorf("GetPolicy: access denied"),
	})

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "enrich failed") {
		t.Errorf("expected view to show 'enrich failed' flash, got:\n%s", view)
	}
}

// TestEnrichResult_ErrorFlash_AdvancesGenSoStalePendingTickCannotClearIt pins
// #261's flash-lifecycle fix: dispatchDetailOpResultIntents
// (internal/tui/runtime_adapter_resources.go) used to direct-mutate m.flash
// for an enrichment-error FlashIntent, so flash.gen was never bumped — a
// tick already pending from an EARLIER flash (stamped with the OLD gen)
// could then clear the brand-new error almost immediately, and no
// error-history entry was ever recorded (AppendErrorHistoryIntent only
// fires by routing through Core.HandleFlash). It now bumps m.flash.gen and
// routes through Core.HandleFlash + dispatchHandlerResult — the same path
// messages.Flash itself takes.
func TestEnrichResult_ErrorFlash_AdvancesGenSoStalePendingTickCannotClearIt(t *testing.T) {
	m := newEnrichApp()

	// Establish a baseline flash (simulating the EARLIER flash whose
	// auto-clear tick — stamped with genBefore — is the "stale pending
	// tick" this fix must survive).
	m, _ = rootApplyMsg(m, messages.Flash{Text: "earlier notice"})
	genBefore := m.FlashGen()
	if genBefore == 0 {
		t.Fatalf("baseline flash.gen should be >0 after Flash, got %d", genBefore)
	}

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/gen-bump-test", "gen-bump-test", "Managed")
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	m, cmd := rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "role_policies",
		ResourceID:   res.ID,
		Err:          fmt.Errorf("GetPolicy: access denied"),
	})

	genAfter := m.FlashGen()
	if genAfter <= genBefore {
		t.Fatalf("flash.gen after the enrichment error = %d, want strictly greater than the pre-error baseline %d — a direct m.flash mutation (the regression) never bumps gen", genAfter, genBefore)
	}

	// The returned cmd must carry the new flash's own auto-clear tick — do
	// NOT execute it (it is a tea.Tick auto-clear timer; running it would
	// deliver ClearFlash and erase the flash before assertions below run —
	// see qa_error_log_test.go's TestErrorHistoryAccumulation_NonErrorFlashesNotAdded
	// for the same established idiom).
	if cmd == nil {
		t.Fatal("dispatchDetailOpResultIntents returned a nil cmd for an error FlashIntent — want the auto-clear tick to still be scheduled")
	}

	// Simulate the STALE tick from the EARLIER flash arriving now (stamped
	// with genBefore, not the new error flash's genAfter). Before the fix,
	// genAfter == genBefore, so this would have incorrectly cleared the
	// brand-new error.
	m, _ = rootApplyMsg(m, messages.ClearFlash{Gen: genBefore})

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "enrich failed") {
		t.Errorf("a stale ClearFlash stamped with the PRE-error gen (%d) cleared the new error flash (now at gen %d) — the gen bump must make them distinct. View:\n%s", genBefore, genAfter, view)
	}

	// Error-history entry recorded: pressing "!" must open the error log
	// viewer, not flash "No errors this session" (the observable proxy
	// qa_error_log_test.go's TestErrorHistoryAccumulation_ErrorFlashesAddToHistory
	// uses — Model has no direct error-history accessor from this package).
	m, bangCmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})
	if bangCmd != nil {
		if msg := bangCmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}
	afterBang := stripANSI(rootViewContent(m))
	if strings.Contains(afterBang, "No errors this session") {
		t.Error("no error-history entry recorded for the enrichment failure — dispatchDetailOpResultIntents must route through Core.HandleFlash (which emits AppendErrorHistoryIntent), not a direct m.flash mutation")
	}
}

func TestEnrichResult_StaleGeneration_IsDiscarded(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Capture the operation ID active BEFORE this detail even opens — the
	// Navigate below begins a brand-new DetailOperation via
	// Core.BeginDetailOperation, so this pre-existing value is guaranteed
	// stale once that happens.
	staleOp := m.Core().ActiveDetailOp()

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/gen-test", "gen-test", "Managed")
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Replay an enrichment result stamped with the pre-open operation ID —
	// stale relative to the operation the Navigate above just began.
	enrichedRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "role_policies",
		ResourceID:   res.ID,
		EnrichedRes:  enrichedRes,
		OperationID:  staleOp,
	})

	content := stripANSI(rootViewContent(m))
	if strings.Contains(content, "2012-10-17") {
		t.Error("detail view should NOT show document from stale generation")
	}
}

func TestYAMLView_DirectFromList_EnrichmentUpdatesContent(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/yaml-direct", "yaml-direct", "Managed")

	// Open YAML view directly (as if pressing y from resource list)
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetYAML,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Simulate enrichment result arriving
	enrichedRes := withDocument(res, map[string]any{
		"Version": "2012-10-17",
		"Statement": []any{
			map[string]any{"Effect": "Allow", "Action": "s3:*", "Resource": "*"},
		},
	})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "role_policies",
		ResourceID:   res.ID,
		EnrichedRes:  enrichedRes,
	})

	content := stripANSI(rootViewContent(m))
	if !strings.Contains(content, "Document") {
		t.Error("expected YAML view to show Document after enrichment")
	}
	if !strings.Contains(content, "Statement") {
		t.Error("expected YAML view to show Statement after enrichment")
	}
}

func TestJSONView_DirectFromList_EnrichmentUpdatesContent(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/json-direct", "json-direct", "Managed")

	// Open JSON view directly
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetJSON,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Simulate enrichment result
	enrichedRes := withDocument(res, map[string]any{
		"Version": "2012-10-17",
		"Statement": []any{
			map[string]any{"Effect": "Deny", "Action": "*", "Resource": "*"},
		},
	})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "role_policies",
		ResourceID:   res.ID,
		EnrichedRes:  enrichedRes,
	})

	content := stripANSI(rootViewContent(m))
	if !strings.Contains(content, "Document") {
		t.Error("expected JSON view to show Document after enrichment")
	}
	if !strings.Contains(content, "Deny") {
		t.Error("expected JSON view to show Deny effect after enrichment")
	}
}

func TestYAMLView_WrongResourceType_EnrichmentIgnored(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/yaml-guard", "yaml-guard", "Managed")

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetYAML,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Send enrichment with wrong resource type
	enrichedRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: "wrong-type",
		ResourceID:   res.ID,
		EnrichedRes:  enrichedRes,
	})

	content := stripANSI(rootViewContent(m))
	if strings.Contains(content, "2012-10-17") {
		t.Error("YAML view should NOT show document from wrong resource type")
	}
}

func TestPolicyDocCache_SessionScoped_SameClientUsesCache(t *testing.T) {
	// Same ServiceClients instance, same policy opened twice — second uses cache.
	var cache awsclient.PolicyDocumentCache

	cache.Set(awsclient.ManagedKey("arn:aws:iam::123456789012:policy/test"), map[string]any{"cached": true})

	got := cache.Get(awsclient.ManagedKey("arn:aws:iam::123456789012:policy/test"))
	if got == nil {
		t.Fatal("expected cache hit for same key on same cache instance")
	}
	doc := got.(map[string]any)
	if doc["cached"] != true {
		t.Error("expected cached document")
	}
}

func TestPolicyDocCache_DifferentInstances_DoNotShareCache(t *testing.T) {
	// Two ServiceClients instances (simulating profile switch) have independent caches.
	var cache1, cache2 awsclient.PolicyDocumentCache

	cache1.Set(awsclient.InlineKey("my-role", "trust-policy"), map[string]any{"from": "account-1"})

	// cache2 should have nothing — different instance, different session
	got := cache2.Get(awsclient.InlineKey("my-role", "trust-policy"))
	if got != nil {
		t.Fatal("different PolicyDocumentCache instances must not share data")
	}
}

func TestPolicyDocCache_InlineKeysDistinctPerRole(t *testing.T) {
	// Same policy name on different roles must not collide.
	var cache awsclient.PolicyDocumentCache

	cache.Set(awsclient.InlineKey("role-a", "trust-policy"), map[string]any{"role": "a"})
	cache.Set(awsclient.InlineKey("role-b", "trust-policy"), map[string]any{"role": "b"})

	gotA := cache.Get(awsclient.InlineKey("role-a", "trust-policy")).(map[string]any)
	gotB := cache.Get(awsclient.InlineKey("role-b", "trust-policy")).(map[string]any)

	if gotA["role"] != "a" {
		t.Errorf("expected role-a document, got %v", gotA["role"])
	}
	if gotB["role"] != "b" {
		t.Errorf("expected role-b document, got %v", gotB["role"])
	}
}

func TestPolicyDocCache_ZeroValueSafe(t *testing.T) {
	// Zero-value cache must work without initialization.
	var cache awsclient.PolicyDocumentCache

	got := cache.Get("nonexistent")
	if got != nil {
		t.Fatal("expected nil from zero-value cache")
	}

	// Set should not panic on zero-value cache
	cache.Set("key", "value")
	if cache.Get("key") != "value" {
		t.Fatal("expected value after Set on zero-value cache")
	}
}

// TestPolicyDocCache_SetIfNewer_StaleOpRefused_NewerOpReplaces mirrors
// TestDetailDocCache_SetIfNewer_StaleOpRefused_NewerOpReplaces
// (detail_doc_cache_test.go) for PolicyDocumentCache: the two caches are
// independently implemented (no shared underlying type), so a stale-op
// write-refusal bug in one is not automatically caught by testing the
// other. Assumed API surface: SetIfNewer(key string, doc any, opID
// domain.Gen) bool, additive alongside the existing plain Set every
// pre-existing TestPolicyDocCache_* test above still uses unmodified.
func TestPolicyDocCache_SetIfNewer_StaleOpRefused_NewerOpReplaces(t *testing.T) {
	var cache awsclient.PolicyDocumentCache
	key := awsclient.ManagedKey("arn:aws:iam::123456789012:policy/test")

	if ok := cache.SetIfNewer(key, "op-7-value", domain.Gen(7)); !ok {
		t.Fatal("SetIfNewer(op 7) on an empty key must succeed")
	}
	if got := cache.Get(key); got != "op-7-value" {
		t.Fatalf("Get after op-7 write = %v, want %q", got, "op-7-value")
	}

	if ok := cache.SetIfNewer(key, "op-5-value", domain.Gen(5)); ok {
		t.Error("SetIfNewer(op 5) reported success — a strictly-older op must be refused")
	}
	if got := cache.Get(key); got != "op-7-value" {
		t.Errorf("Get after refused op-5 write = %v, want op-7's value %q unchanged", got, "op-7-value")
	}

	if ok := cache.SetIfNewer(key, "op-9-value", domain.Gen(9)); !ok {
		t.Error("SetIfNewer(op 9) reported failure — a strictly-newer op must replace the recorded writer's value")
	}
	if got := cache.Get(key); got != "op-9-value" {
		t.Errorf("Get after op-9 write = %v, want %q", got, "op-9-value")
	}
}

// TestPolicyDocCache_SetIfNewer_OpZero_WritesEmptyKey_DoesNotBlockLaterOp
// mirrors the DetailDocCache opID-0 axis: op 0 writes a never-written key
// successfully but never raises the bar, so a later op 3 write still lands.
func TestPolicyDocCache_SetIfNewer_OpZero_WritesEmptyKey_DoesNotBlockLaterOp(t *testing.T) {
	var cache awsclient.PolicyDocumentCache
	key := awsclient.InlineKey("my-role", "trust-policy")

	if ok := cache.SetIfNewer(key, "op-0-value", domain.Gen(0)); !ok {
		t.Fatal("SetIfNewer(op 0) on an empty key must succeed")
	}
	if got := cache.Get(key); got != "op-0-value" {
		t.Fatalf("Get after op-0 write = %v, want %q", got, "op-0-value")
	}

	if ok := cache.SetIfNewer(key, "op-3-value", domain.Gen(3)); !ok {
		t.Error("SetIfNewer(op 3) reported failure — an op-0 write must never block a later op from writing the same key")
	}
	if got := cache.Get(key); got != "op-3-value" {
		t.Errorf("Get after op-3 write = %v, want %q", got, "op-3-value")
	}
}

// TestPolicyDocCache_SetIfNewer_NoEviction_UnrelatedKeysNeverInterfere pins
// the PolicyDocumentCache half of #261's version-keyed eviction contract
// (item e): PolicyDocumentCache's keys are stable (never version-stamped),
// so its own SetIfNewer always passes resource="" to opAwareDocStore — no
// eviction bookkeeping applies at all. Two keys that would collide under
// DetailDocCache's ":"-stripping scheme (both share the "managed" prefix)
// must NOT evict each other here.
func TestPolicyDocCache_SetIfNewer_NoEviction_UnrelatedKeysNeverInterfere(t *testing.T) {
	var cache awsclient.PolicyDocumentCache
	keyA := awsclient.ManagedKey("arn:aws:iam::123456789012:policy/policy-a")
	keyB := awsclient.ManagedKey("arn:aws:iam::123456789012:policy/policy-b")

	if ok := cache.SetIfNewer(keyA, "doc-a", domain.Gen(5)); !ok {
		t.Fatal("SetIfNewer(keyA) must succeed")
	}
	if ok := cache.SetIfNewer(keyB, "doc-b", domain.Gen(5)); !ok {
		t.Fatal("SetIfNewer(keyB) must succeed")
	}

	if got := cache.Get(keyA); got != "doc-a" {
		t.Errorf("Get(keyA) after writing keyB = %v, want %q (PolicyDocumentCache has no eviction)", got, "doc-a")
	}
	if got := cache.Get(keyB); got != "doc-b" {
		t.Errorf("Get(keyB) = %v, want %q", got, "doc-b")
	}
}

func TestRefresh_OnDetailView_DispatchesEnrichment(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/refresh-test", "refresh-test", "Managed")
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Press Ctrl+R to refresh
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	// Should return a batched command (related checks + enrichment)
	if cmd == nil {
		t.Fatal("expected a command on refresh")
	}
}

// ---------------------------------------------------------------------------
// TestDetailOperationTasks_NoEnricher_ReturnsNilEnrichTask
// Verifies that Core.DetailOperationTasks gates the enrich task on
// resource.GetDetailEnricher — "ec2" has no detail enricher, so the
// operation it builds must carry no enrich task at all.
// ---------------------------------------------------------------------------

func TestDetailOperationTasks_NoEnricher_ReturnsNilEnrichTask(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Confirm ec2 has no detail enricher (guard against future registration).
	if resource.HasDetailEnricher("ec2") {
		t.Skip("ec2 now has a detail enricher — update this test to use a type without one")
	}

	ec2Res := resource.Resource{
		ID:   "i-1234567890abcdef0",
		Name: "test-instance",
		Fields: map[string]string{
			"instance_id": "i-1234567890abcdef0",
			"state":       "running",
		},
	}

	_, tasks := m.Core().BeginDetailOperation("ec2", ec2Res, false)

	if enrichTask := findTaskKind(tasks, runtime.KindEnrichDetail); enrichTask != nil {
		t.Error("BeginDetailOperation should return no KindEnrichDetail task when no enricher is registered for the type")
	}
}

// ---------------------------------------------------------------------------
// TestDetailOperationTasks_WithEnricher_ExecutesToEnrichDetailResult
// Verifies that Core.BeginDetailOperation builds a non-nil enrich task when
// an enricher is registered, and that executing it (Core.ExecuteTaskAt)
// produces an EnrichDetailResult carrying the correct ResourceType and
// ResourceID. (BeginDetailOperation folded the former separate
// DetailOperationTasks call into its own return values — #261
// boundary-sealing wave.)
// ---------------------------------------------------------------------------

func TestDetailOperationTasks_WithEnricher_ExecutesToEnrichDetailResult(t *testing.T) {
	if !resource.HasDetailEnricher("role_policies") {
		t.Fatal("expected role_policies detail enricher to be registered")
	}

	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/enrich-direct", "enrich-direct", "Managed")

	_, tasks := m.Core().BeginDetailOperation("role_policies", res, false)
	enrichTask := findTaskKind(tasks, runtime.KindEnrichDetail)
	if enrichTask == nil {
		t.Fatal("BeginDetailOperation should return a KindEnrichDetail task when an enricher is registered")
	}

	ev, err := m.Core().ExecuteTaskAt(context.Background(), *enrichTask, m.Core().CaptureDispatch())
	if err != nil {
		t.Fatalf("ExecuteTaskAt: unexpected error: %v", err)
	}
	resultMsg, ok := ev.(messages.EnrichDetailResult)
	if !ok {
		t.Fatalf("ExecuteTaskAt should return EnrichDetailResult, got %T", ev)
	}
	if resultMsg.ResourceType != "role_policies" {
		t.Errorf("EnrichDetailResult.ResourceType = %q, want %q", resultMsg.ResourceType, "role_policies")
	}
	if resultMsg.ResourceID != res.ID {
		t.Errorf("EnrichDetailResult.ResourceID = %q, want %q", resultMsg.ResourceID, res.ID)
	}
}
