// iam_role_issue_enrichment.go — Wave 2 issue enrichment for the role resource type.
package aws

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// iam-role canonical FindingCodes.
const (
	iamRoleCodeDormant domain.FindingCode = "iam-role.dormant"
)

// EnrichIAMRoleLastUsed calls GetRole per role (capped at EnrichmentCap) to detect dormant roles.
//
// Findings:
//   - RoleLastUsed.LastUsedDate is nil OR time.Since(LastUsedDate) > 90 days → "~" finding "dormant role (>90d)"
//
// AWS service-linked roles (Path starts with "/aws-service-role/") are skipped.
// Skip when clients.IAM == nil.
func EnrichIAMRoleLastUsed(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.IAM == nil {
		return result, nil
	}
	getRoleAPI, ok := clients.IAM.(IAMGetRoleAPI)
	if !ok {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		roleName := r.Fields["role_name"]
		if roleName == "" {
			roleName = r.ID
		}
		if roleName == "" {
			continue
		}
		// Skip AWS service-linked roles.
		if strings.HasPrefix(r.Fields["path"], "/aws-service-role/") {
			continue
		}
		out, err := getRoleAPI.GetRole(ctx, &iam.GetRoleInput{
			RoleName: aws.String(roleName),
		})
		if err != nil {
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
		}
		if out.Role == nil {
			continue
		}
		isDormant := false
		if out.Role.RoleLastUsed == nil || out.Role.RoleLastUsed.LastUsedDate == nil {
			isDormant = true
		} else if time.Since(*out.Role.RoleLastUsed.LastUsedDate) > 90*24*time.Hour {
			isDormant = true
		}
		if isDormant {
			setWave2Finding(&result, r.ID, iamRoleCodeDormant, "dormant role (>90d)", "~", "iam-role", nil)
		}
	}
	// Dormant-role findings are severity "~" (informational); IssueCount stays 0.
	result.IssueCount = 0
	result.Truncated = truncated
	return result, nil
}
