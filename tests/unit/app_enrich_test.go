package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
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

	// Verify role_policies has an enricher registered
	if !resource.HasEnricher("role_policies") {
		t.Fatal("expected role_policies enricher to be registered")
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
