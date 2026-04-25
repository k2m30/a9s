// iam_group_issue_enrichment.go — Wave 2 issue enrichment for the iam-group resource type.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("iam-group", EnrichIAMGroup, 100)
	resource.RegisterIssueEnricherFieldKeys("iam-group", []string{"member_count"})
}

// EnrichIAMGroup calls GetGroup + ListAttachedGroupPolicies per group
// (capped at EnrichmentCap) to surface orphan groups and no-op groups.
//
// Findings:
//   - GetGroup.Users empty → "~" finding "group has no members (orphan)"
//   - ListAttachedGroupPolicies empty AND ListGroupPolicies empty → "~" finding "group has no policies (no-op group)"
//
// Skip when clients.IAM == nil.
func EnrichIAMGroup(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.IAM == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	getGroupAPI, ok1 := clients.IAM.(IAMGetGroupAPI)
	attachedPoliciesAPI, ok2 := clients.IAM.(IAMListAttachedGroupPoliciesAPI)
	inlinePoliciesAPI, ok3 := clients.IAM.(IAMListGroupPoliciesAPI)
	if !ok1 || !ok2 || !ok3 {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}

	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		groupName := r.Fields["group_name"]
		if groupName == "" {
			groupName = r.ID
		}
		if groupName == "" {
			continue
		}

		// Paginate members via GetGroup (uses Marker/IsTruncated).
		var allUsers []iamtypes.User
		memberTruncated := false
		var groupMarker *string
		memberPages := 0
		memberFirstCallErrd := false
		for {
			if memberPages >= PerParentPageCap {
				memberTruncated = true
				truncatedIDs[r.ID] = true
				break
			}
			groupOut, err := getGroupAPI.GetGroup(ctx, &iam.GetGroupInput{
				GroupName: aws.String(groupName),
				Marker:    groupMarker,
			})
			if err != nil {
				truncated = true
				truncatedIDs[r.ID] = true
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
		attachedFirstCallErrd := false
		for {
			if attachedPages >= PerParentPageCap {
				attachedTruncated = true
				truncatedIDs[r.ID] = true
				break
			}
			attachedOut, err := attachedPoliciesAPI.ListAttachedGroupPolicies(ctx, &iam.ListAttachedGroupPoliciesInput{
				GroupName: aws.String(groupName),
				Marker:    attachedMarker,
			})
			if err != nil {
				truncated = true
				truncatedIDs[r.ID] = true
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
		inlineFirstCallErrd := false
		for {
			if inlinePages >= PerParentPageCap {
				inlineTruncated = true
				truncatedIDs[r.ID] = true
				break
			}
			inlineOut, err := inlinePoliciesAPI.ListGroupPolicies(ctx, &iam.ListGroupPoliciesInput{
				GroupName: aws.String(groupName),
				Marker:    inlineMarker,
			})
			if err != nil {
				truncated = true
				truncatedIDs[r.ID] = true
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

		// If any first call failed, we have no data at all — skip findings for this group.
		if memberFirstCallErrd || attachedFirstCallErrd || inlineFirstCallErrd {
			continue
		}

		memberCount := len(allUsers)
		memberCountStr := resource.FormatExact(memberCount)
		if memberTruncated {
			memberCountStr = resource.FormatApproximate(memberCount)
		}
		fieldUpdates[r.ID] = map[string]string{
			"member_count": memberCountStr,
		}

		var rows []resource.FindingRow

		if memberCount == 0 && !memberTruncated {
			rows = append(rows, resource.FindingRow{
				Label: "Members",
				Value: "group has no members (orphan)",
				Tier:  "~",
			})
		}

		if len(allAttached) == 0 && len(allInline) == 0 && !attachedTruncated && !inlineTruncated {
			rows = append(rows, resource.FindingRow{
				Label: "Policies",
				Value: "group has no policies (no-op group)",
				Tier:  "~",
			})
		}

		if len(rows) == 0 {
			continue
		}
		findings[r.ID] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  rows[0].Value,
			Rows:     rows,
		}
	}
	// Group findings are severity "~" (informational); IssueCount stays 0.
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}
