// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_group_issue_enrichment.go — Wave 2 issue enrichment for the iam-group resource type.
package aws

import (
	"cmp"
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
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
//   - GetGroup.Users empty → "~" finding "group has no members (orphan)"
//   - ListAttachedGroupPolicies empty AND ListGroupPolicies empty → "~" finding "group has no policies (no-op group)"
//
// Skip when clients.IAM == nil.
func EnrichIAMGroup(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
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

	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		groupName := r.Fields["group_name"]
		if groupName == "" {
			groupName = r.ID
		}
		if groupName == "" {
			return
		}

		// Paginate members via GetGroup (uses Marker/IsTruncated).
		var allUsers []iamtypes.User
		memberTruncated := false
		var groupMarker *string
		memberPages := 0
		var memberErr error
		memberFirstCallErrd := false
		for {
			if memberPages >= PerParentPageCap {
				memberTruncated = true
				break
			}
			groupOut, err := getGroupAPI.GetGroup(ctx, &iam.GetGroupInput{
				GroupName: aws.String(groupName),
				Marker:    groupMarker,
			})
			if err != nil {
				memberErr = err
				if memberPages == 0 {
					memberFirstCallErrd = true
				} else {
					memberTruncated = true
				}
				break
			}
			memberPages++
			allUsers = append(allUsers, groupOut.Users...)
			if groupOut.IsTruncated {
				groupMarker = groupOut.Marker
			} else {
				break
			}
		}

		// Paginate attached policies.
		var allAttached []iamtypes.AttachedPolicy
		attachedTruncated := false
		var attachedMarker *string
		attachedPages := 0
		var attachedErr error
		attachedFirstCallErrd := false
		for {
			if attachedPages >= PerParentPageCap {
				attachedTruncated = true
				break
			}
			attachedOut, err := attachedPoliciesAPI.ListAttachedGroupPolicies(ctx, &iam.ListAttachedGroupPoliciesInput{
				GroupName: aws.String(groupName),
				Marker:    attachedMarker,
			})
			if err != nil {
				attachedErr = err
				if attachedPages == 0 {
					attachedFirstCallErrd = true
				} else {
					attachedTruncated = true
				}
				break
			}
			attachedPages++
			allAttached = append(allAttached, attachedOut.AttachedPolicies...)
			if attachedOut.IsTruncated {
				attachedMarker = attachedOut.Marker
			} else {
				break
			}
		}

		// Paginate inline policies.
		var allInline []string
		inlineTruncated := false
		var inlineMarker *string
		inlinePages := 0
		var inlineErr error
		inlineFirstCallErrd := false
		for {
			if inlinePages >= PerParentPageCap {
				inlineTruncated = true
				break
			}
			inlineOut, err := inlinePoliciesAPI.ListGroupPolicies(ctx, &iam.ListGroupPoliciesInput{
				GroupName: aws.String(groupName),
				Marker:    inlineMarker,
			})
			if err != nil {
				inlineErr = err
				if inlinePages == 0 {
					inlineFirstCallErrd = true
				} else {
					inlineTruncated = true
				}
				break
			}
			inlinePages++
			allInline = append(allInline, inlineOut.PolicyNames...)
			if inlineOut.IsTruncated {
				inlineMarker = inlineOut.Marker
			} else {
				break
			}
		}

		mu.Lock()
		defer mu.Unlock()

		switch walkErr := cmp.Or(memberErr, attachedErr, inlineErr); {
		case walkErr != nil:
			MarkSkipped(&result, r.ID, &failures, walkErr)
		case memberTruncated || attachedTruncated || inlineTruncated:
			// A page cap, not a failed call: there is no error to record.
			result.TruncatedIDs[r.ID] = true
		}

		// If any first call failed, we have no data at all — skip findings for this group.
		if memberFirstCallErrd || attachedFirstCallErrd || inlineFirstCallErrd {
			return
		}

		memberCount := len(allUsers)
		memberCountStr := resource.FormatExact(memberCount)
		if memberTruncated {
			memberCountStr = resource.FormatTruncated(memberCount)
		}
		result.FieldUpdates[r.ID] = map[string]string{
			"member_count": memberCountStr,
		}

		var rows []domain.DetailRow

		if memberCount == 0 && !memberTruncated {
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

		if adminPolicy := adminAttachedPolicyName(allAttached); adminPolicy != "" {
			setWave2Finding(&result, r.ID, iamGroupCodeAdminAttached, "~", "iam-group",
				adminAttachedRows(adminPolicy))

		}

		if len(rows) == 0 {
			return
		}
		setWave2Finding(&result, r.ID, iamGroupCodeOrphanOrNoop, "~", "iam-group", rows)
	})
	MarkInformationalOnly(&result)
	return result, nil
}
