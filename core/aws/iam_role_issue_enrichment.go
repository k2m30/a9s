// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_role_issue_enrichment.go — Wave 2 issue enrichment for the role resource type.
package aws

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// iam-role canonical FindingCodes.
const (
	iamRoleCodeDormant       domain.FindingCode = "iam-role.dormant"
	iamRoleCodeAdminAttached domain.FindingCode = "role.admin-attached"
)

// EnrichIAMRoleLastUsed calls GetRole per role (capped at EnrichmentCap) to detect dormant roles,
// and writes the role's last use, which only GetRole returns, under "last_used".
//
// Findings:
//   - RoleLastUsed.LastUsedDate is nil OR time.Since(LastUsedDate) > 90 days → "~" finding "dormant role (>90d)"
//
// A role no finding can stand on (a service-linked role) is read for its last
// use only: the column is the role's, whatever its kind.
func EnrichIAMRoleLastUsed(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.IAM == nil {
		return result, nil
	}
	getRoleAPI, ok := clients.IAM.(IAMGetRoleAPI)
	if !ok {
		return result, nil
	}
	var fieldOnly []resource.Resource
	for _, r := range resources {
		if !rowAppliesAny("role", r, iamRoleCodeDormant, iamRoleCodeAdminAttached) {
			fieldOnly = append(fieldOnly, r)
		}
	}
	fieldOnly = fieldOnlyCap(&result, fieldOnly, resourceIDsOf)
	resources = capAtEnrichmentCap(&result, resources, func(r resource.Resource) bool {
		return rowAppliesAny("role", r, iamRoleCodeDormant, iamRoleCodeAdminAttached)
	}, resourceIDsOf)
	n := len(resources) + len(fieldOnly)
	var failures []Failure
	var mu sync.Mutex
	fieldErr := forEachFieldRow(ctx, &result, resourceIDs(fieldOnly), EnrichmentParallelism, func(i int) {
		r := fieldOnly[i]
		out, err := getRoleAPI.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(roleNameOf(r))})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		setRoleLastUsed(&result, r.ID, out.Role)
	})
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		roleName := roleNameOf(r)
		if roleName == "" {
			return
		}
		attachedRole, _, aerr := listAttachedRolePolicies(ctx, clients.IAM, roleName)
		adminPolicy := adminAttachedPolicyName(attachedRole)
		if aerr != nil {
			mu.Lock()
			MarkSkipped(&result, r.ID, &failures, aerr)
			mu.Unlock()
		}
		out, err := getRoleAPI.GetRole(ctx, &iam.GetRoleInput{
			RoleName: aws.String(roleName),
		})
		mu.Lock()
		defer mu.Unlock()
		if adminPolicy != "" {
			setWave2Finding(&result, r.ID, iamRoleCodeAdminAttached, adminAttachedRows(adminPolicy))

		}
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if out.Role == nil {
			MarkSkipped(&result, r.ID, &failures, UnusableAnswerErr{Call: "GetRole", Field: "role"})
			return
		}
		setRoleLastUsed(&result, r.ID, out.Role)
		isDormant := false
		if out.Role.RoleLastUsed == nil || out.Role.RoleLastUsed.LastUsedDate == nil {
			// IAM never records a last-used date for a role nobody has
			// assumed yet, so age since creation is the only evidence that
			// the role has had time to be used at all.
			isDormant = out.Role.CreateDate != nil && time.Since(*out.Role.CreateDate) > days(roleDormantAfterDays)
		} else if time.Since(*out.Role.RoleLastUsed.LastUsedDate) > days(roleDormantAfterDays) {
			isDormant = true
		}
		if isDormant {
			setWave2Finding(&result, r.ID, iamRoleCodeDormant, nil, strconv.Itoa(roleDormantAfterDays))
		}
	})
	return result, errors.Join(fieldErr, loopErr, AggregateFailures("role last-used", failures, n))
}

func roleNameOf(r resource.Resource) string {
	if name := r.Fields["role_name"]; name != "" {
		return name
	}
	return r.ID
}

// setRoleLastUsed writes the role's last use, which only GetRole returns,
// under "last_used".
func setRoleLastUsed(result *IssueEnricherResult, id string, role *iamtypes.Role) {
	if role != nil && role.RoleLastUsed != nil && role.RoleLastUsed.LastUsedDate != nil {
		result.FieldUpdates[id] = map[string]string{"last_used": role.RoleLastUsed.LastUsedDate.Format("2006-01-02 15:04")}
	}
}
