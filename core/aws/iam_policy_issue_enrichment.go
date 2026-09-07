// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_policy_issue_enrichment.go — Wave 2 issue enrichment for the policy resource type.
package aws

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// iam-policy canonical FindingCodes.
const (
	iamPolicyCodeAdminStar domain.FindingCode = "iam-policy.admin-star"
	iamPolicyCodePrivEsc   domain.FindingCode = "policy.privilege-escalation"
)

// EnrichIAMPolicy calls GetPolicy + GetPolicyVersion per customer-managed policy
// (capped at EnrichmentCap) to detect wildcard-admin policies.
//
// Findings:
//   - Policy document contains Statement with Effect=Allow, Action=*, Resource=* → "!" finding "admin star (allows * on *)"
//
// AWS-managed policies (account "aws", resource under "policy/") are skipped.
// Skip when clients.IAM == nil.
func EnrichIAMPolicy(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.IAM == nil {
		return result, nil
	}
	getPolicyAPI, ok1 := clients.IAM.(IAMGetPolicyAPI)
	getPolicyVersionAPI, ok2 := clients.IAM.(IAMGetPolicyVersionAPI)
	if !ok1 || !ok2 {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		// Resolve the policy ARN — prefer RawStruct, fall back to r.ID when it is an ARN.
		policyARN, ok := extractIAMPolicyARN(r)
		if !ok || policyARN == "" {
			// Fallback: r.ID may be an ARN (tests and demo mode set it directly).
			if strings.HasPrefix(r.ID, "arn:") {
				policyARN = r.ID
			}
		}
		if policyARN == "" {
			return
		}
		// An AWS-managed policy is not editable by the account, so reading
		// its document for permission findings tells an operator nothing
		// they can act on.
		if _, awsOwned := awsManagedPolicy(policyARN); awsOwned {
			return
		}
		doc, err := FetchManagedPolicyDocument(ctx, getPolicyAPI, getPolicyVersionAPI, policyARN)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			truncated = true
			result.TruncatedIDs[r.ID] = true
			return
		}
		riskVal := ""
		// The orphan finding itself is wave1 (orphanUnattachedPolicyFinding);
		// only the Risk column value is decided here.
		if r.Fields["attachment_count"] == "0" {
			riskVal = riskUnattached
		}
		if isAdminStarPolicy(doc) {
			riskVal = riskAdminPolicy
			setWave2Finding(&result, r.ID, iamPolicyCodeAdminStar, "admin star (allows * on *)", "!", "iam-policy", []domain.DetailRow{
				{Label: "Allowed actions", Value: "all (*)", Tier: "!"},
				{Label: "On resources", Value: "all (*)", Tier: "!"},
			})

		} else if combos := policyPrivEscCombos(doc); len(combos) > 0 {
			// An admin policy matches nearly every combination; reporting it
			// twice would say the same thing in two voices, so admin wins.
			riskVal = riskPrivEsc
			setWave2Finding(&result, r.ID, iamPolicyCodePrivEsc,
				catalog.Phrase(iamPolicyCodePrivEsc), "!", "iam-policy",
				privEscComboRows(combos))

		}
		result.FieldUpdates[r.ID] = map[string]string{
			"risk": riskVal,
		}
	})
	result.Truncated = truncated
	return result, nil
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

// policyPrivEscCombos re-encodes the decoded policy document and runs it
// through iampolicy's privilege-escalation matcher. FetchManagedPolicyDocument
// hands back the unmarshalled document, so the round trip is what keeps this
// on the one policy engine instead of a second ad-hoc walk of the map.
func policyPrivEscCombos(doc any) []string {
	if doc == nil {
		return nil
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil
	}
	parsed, err := iampolicy.Parse(string(raw))
	if err != nil {
		return nil
	}
	return parsed.PrivilegeEscalation()
}

// privEscComboRows lists the matched combinations. The sink bounds the list.
func privEscComboRows(combos []string) []domain.DetailRow {
	rows := make([]domain.DetailRow, 0, len(combos))
	for _, c := range combos {
		rows = append(rows, domain.DetailRow{Label: "Combo", Value: c, Tier: "!"})
	}
	return rows
}
