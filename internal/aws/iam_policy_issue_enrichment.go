// iam_policy_issue_enrichment.go — Wave 2 issue enrichment for the policy resource type.
package aws

import (
	"context"
	"strings"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("policy", EnrichIAMPolicy, 100)
	resource.RegisterIssueEnricherFieldKeys("policy", []string{"risk"})
}

// EnrichIAMPolicy calls GetPolicy + GetPolicyVersion per customer-managed policy
// (capped at EnrichmentCap) to detect wildcard-admin policies.
//
// Findings:
//   - Policy document contains Statement with Effect=Allow, Action=*, Resource=* → "!" finding "admin star (CIS IAM.16)"
//
// AWS-managed policies (ARN starts with "arn:aws:iam::aws:policy/") are skipped.
// Skip when clients.IAM == nil.
func EnrichIAMPolicy(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.IAM == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	getPolicyAPI, ok1 := clients.IAM.(IAMGetPolicyAPI)
	getPolicyVersionAPI, ok2 := clients.IAM.(IAMGetPolicyVersionAPI)
	if !ok1 || !ok2 {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	issueCount := 0
	fieldUpdates := make(map[string]map[string]string)
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		// Resolve the policy ARN — prefer RawStruct, fall back to r.ID when it is an ARN.
		policyARN, ok := extractIAMPolicyARN(r)
		if !ok || policyARN == "" {
			// Fallback: r.ID may be an ARN (tests and demo mode set it directly).
			if strings.HasPrefix(r.ID, "arn:") {
				policyARN = r.ID
			}
		}
		if policyARN == "" {
			continue
		}
		// Skip AWS-managed policies.
		if strings.HasPrefix(policyARN, "arn:aws:iam::aws:policy/") {
			continue
		}
		doc, err := FetchManagedPolicyDocument(ctx, getPolicyAPI, getPolicyVersionAPI, policyARN)
		if err != nil {
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		riskVal := ""
		// Check if the policy is not attached to any entity — orphan.
		attachCount := r.Fields["attachment_count"]
		if attachCount == "0" {
			riskVal = "ORPHAN"
		}
		if isAdminStarPolicy(doc) {
			riskVal = "ADMIN_ALL"
			findings[r.ID] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  "admin star (CIS IAM.16)",
				Rows: []resource.FindingRow{
					{Label: "Action", Value: "*", Tier: "!"},
					{Label: "Resource", Value: "*", Tier: "!"},
				},
			}
			issueCount++
		}
		fieldUpdates[r.ID] = map[string]string{
			"risk": riskVal,
		}
	}
	return IssueEnricherResult{IssueCount: issueCount, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}

// extractIAMPolicyARN extracts the ARN from a resource whose RawStruct is an iamtypes.Policy
// or a PolicyEnriched wrapper. Returns ("", false) if the type is unrecognized or ARN is nil.
func extractIAMPolicyARN(r resource.Resource) (string, bool) {
	switch p := r.RawStruct.(type) {
	case iamtypes.Policy:
		if p.Arn != nil {
			return *p.Arn, true
		}
	case PolicyEnriched:
		if p.Arn != nil {
			return *p.Arn, true
		}
	}
	return "", false
}

// isAdminStarPolicy reports whether a decoded policy document grants unrestricted admin access
// (Effect=Allow + Action=* + Resource=* in any statement).
func isAdminStarPolicy(doc any) bool {
	if doc == nil {
		return false
	}
	// doc is a map[string]any from json.Unmarshal via FetchManagedPolicyDocument.
	m, ok := doc.(map[string]any)
	if !ok {
		// If it was a string (raw JSON), do a simple substring check.
		if s, ok2 := doc.(string); ok2 {
			return isAdminStarPolicyString(s)
		}
		return false
	}
	stmts, ok := m["Statement"]
	if !ok {
		return false
	}
	stmtList, ok := stmts.([]any)
	if !ok {
		return false
	}
	for _, stmt := range stmtList {
		sm, ok := stmt.(map[string]any)
		if !ok {
			continue
		}
		effect, _ := sm["Effect"].(string)
		if !strings.EqualFold(effect, "Allow") {
			continue
		}
		if matchesStar(sm["Action"]) && matchesStar(sm["Resource"]) {
			return true
		}
	}
	return false
}

// matchesStar returns true if the policy field value is "*" (string) or ["*"] (slice).
func matchesStar(v any) bool {
	switch val := v.(type) {
	case string:
		return val == "*"
	case []any:
		for _, item := range val {
			if s, ok := item.(string); ok && s == "*" {
				return true
			}
		}
	}
	return false
}

// isAdminStarPolicyString does a quick substring check for admin-star in a raw JSON string.
func isAdminStarPolicyString(doc string) bool {
	return (strings.Contains(doc, `"Effect":"Allow"`) || strings.Contains(doc, `"Effect": "Allow"`)) &&
		(strings.Contains(doc, `"Action":"*"`) || strings.Contains(doc, `"Action": "*"`)) &&
		(strings.Contains(doc, `"Resource":"*"`) || strings.Contains(doc, `"Resource": "*"`))
}
