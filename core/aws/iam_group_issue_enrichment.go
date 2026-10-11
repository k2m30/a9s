// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_group_issue_enrichment.go — Wave 2 issue enrichment for the iam-group resource type.
package aws

import (
	"context"
	"errors"
	"sync"
	"time"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// iam-group canonical FindingCodes.
const (
	iamGroupCodeOrphanOrNoop  domain.FindingCode = "iam-group.orphan-or-noop"
	iamGroupCodeAdminAttached domain.FindingCode = "iam-group.admin-attached"
)

// EnrichIAMGroup calls GetGroup + ListAttachedGroupPolicies per group
// (capped at EnrichmentCap) to surface orphan groups and no-op groups.
//
// Findings:
//   - GetGroup.Users empty on a group older than 30 days → "~" finding "group has no members (orphan)"
//   - ListAttachedGroupPolicies empty AND ListGroupPolicies empty → "~" finding "group has no policies (no-op group)"
func EnrichIAMGroup(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.IAM == nil {
		return result, nil
	}
	getGroupAPI, ok1 := clients.IAM.(IAMGetGroupAPI)
	attachedPoliciesAPI, ok2 := clients.IAM.(IAMListAttachedGroupPoliciesAPI)
	inlinePoliciesAPI, ok3 := clients.IAM.(IAMListGroupPoliciesAPI)
	if !ok1 || !ok2 || !ok3 {
		return result, nil
	}

	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		groupName := r.Fields["group_name"]
		if groupName == "" {
			groupName = r.ID
		}
		if groupName == "" {
			return
		}

		allUsers, membersComplete, memberErr := iamGroupUsers(ctx, getGroupAPI, groupName)
		allAttached, attachedComplete, attachedErr := iamGroupAttachedPolicies(ctx, attachedPoliciesAPI, groupName)
		allInline, inlineComplete, inlineErr := iamGroupInlinePolicies(ctx, inlinePoliciesAPI, groupName)
		memberTruncated, attachedTruncated, inlineTruncated := !membersComplete, !attachedComplete, !inlineComplete

		mu.Lock()
		defer mu.Unlock()

		// Each walk answers its own question: a failed one leaves only the
		// signals that read it unevaluated, through its truncated flag, since
		// PageAll reports an errored walk incomplete.
		for _, err := range []error{memberErr, attachedErr, inlineErr} {
			if err != nil {
				MarkSkipped(&result, r.ID, &failures, err)
			}
		}
		// A walk stopped at the page cap leaves a check open only when the
		// pages read did not already decide it: one member rules out an
		// orphan, one policy rules out a no-op group, and an administrator
		// policy seen is the admin finding.
		adminPolicy := adminAttachedPolicyName(allAttached)
		membersOpen := memberTruncated && len(allUsers) == 0
		policiesOpen := (attachedTruncated || inlineTruncated) && len(allAttached) == 0 && len(allInline) == 0
		adminOpen := attachedTruncated && adminPolicy == ""
		if membersOpen || policiesOpen || adminOpen {
			markUninspected(&result, r.ID, CheckCap)
		}

		memberCount := len(allUsers)
		if memberErr == nil || memberCount > 0 {
			memberCountStr := resource.FormatExact(memberCount)
			if memberTruncated {
				memberCountStr = resource.FormatTruncated(memberCount)
			}
			result.FieldUpdates[r.ID] = map[string]string{
				"member_count": memberCountStr,
			}
		}

		var rows []domain.DetailRow

		// A group created minutes ago has no members yet by construction;
		// only one left empty for a month is evidence of an orphan.
		grp, _ := assertStruct[iamtypes.Group](r.RawStruct)
		aged := grp.CreateDate != nil && time.Since(*grp.CreateDate) > 30*24*time.Hour

		if memberCount == 0 && !memberTruncated && aged {
			rows = append(rows, domain.DetailRow{
				Label: "Members",
				Value: "group has no members (orphan)",
				Tier:  "~",
			})
		}

		if len(allAttached) == 0 && len(allInline) == 0 && !attachedTruncated && !inlineTruncated {
			rows = append(rows, domain.DetailRow{
				Label: "Policies",
				Value: "group has no policies (no-op group)",
				Tier:  "~",
			})
		}

		if adminPolicy != "" {
			setWave2Finding(&result, r.ID, iamGroupCodeAdminAttached, adminAttachedRows(adminPolicy))
		}

		if len(rows) == 0 {
			return
		}
		setWave2Finding(&result, r.ID, iamGroupCodeOrphanOrNoop, rows)
	})
	MarkInformationalOnly(&result)
	// See EnrichIAMRoleLastUsed: the flag and the composite error are two
	// separate answers, and both are returned.
	return result, errors.Join(loopErr, AggregateFailures("group membership", failures, n))
}
