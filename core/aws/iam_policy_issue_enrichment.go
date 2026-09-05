// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_policy_issue_enrichment.go — Wave 2 issue enrichment for the policy resource type.
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// iam-policy canonical FindingCodes.
const (
	iamPolicyCodeAdminStar domain.FindingCode = "iam-policy.admin-star"
	iamPolicyCodePrivEsc   domain.FindingCode = "policy.privilege-escalation"

	iamPolicyAdminStarDetail = "This policy allows every action on every resource, so anyone holding it is an " +
		"account administrator. Replace the \"*\" action and resource with the specific ones its holders need."
	iamPolicyPrivEscDetail = "This policy grants a combination of actions that lets its holder grant itself full " +
		"administrator, even though no single action looks privileged. Split the combination across separate " +
		"policies or remove the escalation actions."

	// privEscComboRowCap bounds the Combo rows listed on one finding; the
	// remainder is summarised in a trailing row.
	privEscComboRowCap = 10
)

// EnrichIAMPolicy calls GetPolicy + GetPolicyVersion per customer-managed policy
// (capped at EnrichmentCap) to detect wildcard-admin policies.
//
// Findings:
//   - Policy document contains Statement with Effect=Allow, Action=*, Resource=* → "!" finding "admin star (allows * on *)"
//
// AWS-managed policies (ARN starts with "arn:aws:iam::aws:policy/") are skipped.
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
		// Skip AWS-managed policies.
		if strings.HasPrefix(policyARN, "arn:aws:iam::aws:policy/") {
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
		// Check if the policy is not attached to any entity — orphan.
		attachCount := r.Fields["attachment_count"]
		if attachCount == "0" {
			riskVal = riskUnattached
		}
		if isAdminStarPolicy(doc) {
			riskVal = riskAdminPolicy
			setWave2Finding(&result, r.ID, iamPolicyCodeAdminStar, "admin star (allows * on *)", "!", "iam-policy", []domain.DetailRow{
				{Label: "Allowed actions", Value: "all (*)", Tier: "!"},
				{Label: "On resources", Value: "all (*)", Tier: "!"},
			}, iamPolicyAdminStarDetail)
		} else if combos := policyPrivEscCombos(doc); len(combos) > 0 {
			// An admin policy matches nearly every combination; reporting it
			// twice would say the same thing in two voices, so admin wins.
			riskVal = riskPrivEsc
			setWave2Finding(&result, r.ID, iamPolicyCodePrivEsc,
				"allows privilege escalation: "+combos[0], "!", "iam-policy",
				privEscComboRows(combos), iamPolicyPrivEscDetail)
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

// privEscComboRows lists the matched combinations, capped so a policy that
// matches dozens does not push the rest of the Attention section off screen.
func privEscComboRows(combos []string) []domain.DetailRow {
	shown := combos
	var overflow int
	if len(shown) > privEscComboRowCap {
		overflow = len(shown) - privEscComboRowCap
		shown = shown[:privEscComboRowCap]
	}
	rows := make([]domain.DetailRow, 0, len(shown)+1)
	for _, c := range shown {
		rows = append(rows, domain.DetailRow{Label: "Combo", Value: c, Tier: "!"})
	}
	if overflow > 0 {
		rows = append(rows, domain.DetailRow{Label: "Combo", Value: fmt.Sprintf("… +%d more", overflow), Tier: "!"})
	}
	return rows
}
