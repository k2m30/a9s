// iam_role_issue_enrichment.go — Wave 2 issue enrichment for the role resource type.
package aws

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.IAM == nil {
		return result, nil
	}
	getRoleAPI, ok := clients.IAM.(IAMGetRoleAPI)
	if !ok {
		return result, nil
	}
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		roleName := r.Fields["role_name"]
		if roleName == "" {
			roleName = r.ID
		}
		if roleName == "" {
			return
		}
		// Skip AWS service-linked roles.
		if strings.HasPrefix(r.Fields["path"], "/aws-service-role/") {
			return
		}
		out, err := getRoleAPI.GetRole(ctx, &iam.GetRoleInput{
			RoleName: aws.String(roleName),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			result.TruncatedIDs[r.ID] = true
			return
		}
		if out.Role == nil {
			return
		}
		isDormant := false
		if out.Role.RoleLastUsed == nil || out.Role.RoleLastUsed.LastUsedDate == nil {
			isDormant = true
		} else if time.Since(*out.Role.RoleLastUsed.LastUsedDate) > 90*24*time.Hour {
			isDormant = true
		}
		if isDormant {
			setWave2Finding(&result, r.ID, iamRoleCodeDormant, "dormant role (>90d)", "~", "iam-role", nil, "")
		}
	})
	// Dormant-role findings are severity "~" (informational); IssueCount stays 0.
	result.IssueCount = 0
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result, nil
}
