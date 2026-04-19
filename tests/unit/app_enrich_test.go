package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
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
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
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

	y := views.NewYAML(enrichedRes, "role_policies", k)
	y.SetSize(120, 40)

	content := stripANSI(y.View())
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

	y := views.NewYAML(enrichedRes, "role_policies", k)
	y.SetSize(120, 40)

	content := stripANSI(y.View())
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

	j := views.NewJSON(enrichedRes, "role_policies", k)
	j.SetSize(120, 40)

	content := stripANSI(j.View())
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

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Capture view before mismatch enrichment
	beforeContent := stripANSI(rootViewContent(m))

	// Send enrichment result with WRONG resource ID — should be silently ignored
	wrongRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResultMsg{
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

	_, cmd := rootApplyMsg(m, messages.NavigateMsg{
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

	y := views.NewYAML(enrichedRes, "role_policies", k)
	y.SetSize(120, 40)

	content := stripANSI(y.View())
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

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	enrichedRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	// Should not panic, update should be accepted
	m, _ = rootApplyMsg(m, messages.EnrichDetailResultMsg{
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
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/rt-test", "rt-test", "Managed")
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Send enrichment result with WRONG resource type but matching ID
	enrichedRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResultMsg{
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
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/err-test", "err-test", "Managed")
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Send enrichment result with an error
	_, cmd := rootApplyMsg(m, messages.EnrichDetailResultMsg{
		ResourceType: "role_policies",
		ResourceID:   res.ID,
		Err:          fmt.Errorf("GetPolicy: access denied"),
	})

	// The app.go handler returns a FlashMsg command on error
	if cmd == nil {
		t.Fatal("expected a flash command on enrichment error")
	}
	msg := cmd()
	flash, ok := msg.(messages.FlashMsg)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	if !flash.IsError {
		t.Error("expected flash to be an error")
	}
	if !strings.Contains(flash.Text, "enrich failed") {
		t.Errorf("expected flash text to contain 'enrich failed', got %q", flash.Text)
	}
}

func TestEnrichResult_StaleGeneration_IsDiscarded(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/gen-test", "gen-test", "Managed")
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Send enrichment result with a stale generation (999 != current enrichGen)
	enrichedRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResultMsg{
		ResourceType: "role_policies",
		ResourceID:   res.ID,
		EnrichedRes:  enrichedRes,
		Generation:   999, // stale — does not match enrichGen (which is 1)
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
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/yaml-direct", "yaml-direct", "Managed")

	// Open YAML view directly (as if pressing y from resource list)
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
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
	m, _ = rootApplyMsg(m, messages.EnrichDetailResultMsg{
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
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/json-direct", "json-direct", "Managed")

	// Open JSON view directly
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
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
	m, _ = rootApplyMsg(m, messages.EnrichDetailResultMsg{
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
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/yaml-guard", "yaml-guard", "Managed")

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetYAML,
		ResourceType: "role_policies",
		Resource:     &res,
	})

	// Send enrichment with wrong resource type
	enrichedRes := withDocument(res, map[string]any{"Version": "2012-10-17"})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResultMsg{
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

func TestRefresh_OnDetailView_DispatchesEnrichment(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/refresh-test", "refresh-test", "Managed")
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
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
// TestHandleEnrichDetail_NoEnricher_ReturnsNilCmd
// Verifies that handleEnrichDetail returns a nil command when no enricher is
// registered for the resource type. "ec2" has no detail enricher.
// ---------------------------------------------------------------------------

func TestHandleEnrichDetail_NoEnricher_ReturnsNilCmd(t *testing.T) {
	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
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

	// Dispatch EnrichDetailMsg directly — exercises handleEnrichDetail.
	_, cmd := rootApplyMsg(m, messages.EnrichDetailMsg{
		ResourceType: "ec2",
		Resource:     ec2Res,
	})

	// No enricher registered → cmd must be nil.
	if cmd != nil {
		t.Error("handleEnrichDetail should return nil cmd when no enricher is registered for the type")
	}
}

// ---------------------------------------------------------------------------
// TestHandleEnrichDetail_WithEnricher_ReturnsEnrichDetailResultMsg
// Verifies that handleEnrichDetail with a registered enricher returns a cmd
// that, when executed, produces an EnrichDetailResultMsg with the correct
// ResourceType and ResourceID.
// ---------------------------------------------------------------------------

func TestHandleEnrichDetail_WithEnricher_ReturnsEnrichDetailResultMsg(t *testing.T) {
	if !resource.HasDetailEnricher("role_policies") {
		t.Fatal("expected role_policies detail enricher to be registered")
	}

	app := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	res := rolePolicyRes("arn:aws:iam::123456789012:policy/enrich-direct", "enrich-direct", "Managed")

	// Dispatch EnrichDetailMsg directly to exercise handleEnrichDetail.
	_, cmd := rootApplyMsg(m, messages.EnrichDetailMsg{
		ResourceType: "role_policies",
		Resource:     res,
	})

	if cmd == nil {
		t.Fatal("handleEnrichDetail should return a non-nil cmd when an enricher is registered")
	}

	// Execute the cmd — it calls the enricher and returns EnrichDetailResultMsg.
	result := cmd()
	resultMsg, ok := result.(messages.EnrichDetailResultMsg)
	if !ok {
		t.Fatalf("cmd() should return EnrichDetailResultMsg, got %T", result)
	}
	if resultMsg.ResourceType != "role_policies" {
		t.Errorf("EnrichDetailResultMsg.ResourceType = %q, want %q", resultMsg.ResourceType, "role_policies")
	}
	if resultMsg.ResourceID != res.ID {
		t.Errorf("EnrichDetailResultMsg.ResourceID = %q, want %q", resultMsg.ResourceID, res.ID)
	}
}
