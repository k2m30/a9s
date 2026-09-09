// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchBackupPlansPage fetches a single page of Backup plans.
func FetchBackupPlansPage(ctx context.Context, api BackupListBackupPlansAPI, continuationToken string) (resource.FetchResult, error) {
	// A session that never wired Backup is an empty page, not a crash. Every
	// sibling fetcher guards the same way, and the coverage join then sees no
	// cache entry, which it reads as "cannot tell".
	if api == nil {
		return resource.FetchResult{}, nil
	}
	input := &backup.ListBackupPlansInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.ListBackupPlansOutput, error) {
		return api.ListBackupPlans(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Backup plans: %w", err)
	}

	selectionAPI, _ := api.(BackupListBackupSelectionsAPI)
	getSelectionAPI, _ := api.(BackupGetBackupSelectionAPI)

	var resources []resource.Resource

	for _, plan := range output.BackupPlansList {
		planName := ""
		if plan.BackupPlanName != nil {
			planName = *plan.BackupPlanName
		}

		planID := ""
		if plan.BackupPlanId != nil {
			planID = *plan.BackupPlanId
		}

		creationDate := ""
		if plan.CreationDate != nil {
			creationDate = plan.CreationDate.Format("2006-01-02 15:04")
		}

		lastExecution := ""
		if plan.LastExecutionDate != nil {
			lastExecution = plan.LastExecutionDate.Format("2006-01-02 15:04")
		}

		// Enumerate the plan's selection resource ARNs and tag-based
		// conditions so sibling pivots (s3, ddb, efs, dbi, ec2, ebs, …) can
		// match via cache scan. One ListBackupSelections page walk plus one
		// GetBackupSelection per selection — bounded by plan count; selections
		// per plan typically ≤3.
		resourcesCSV, notResourcesCSV, selectionTagsCSV, complete := enumerateBackupPlanResources(ctx, selectionAPI, getSelectionAPI, planID)

		r := resource.Resource{
			ID:   planID,
			Name: planName,
			Fields: map[string]string{
				"plan_name":      planName,
				"plan_id":        planID,
				"creation_date":  creationDate,
				"last_execution": lastExecution,
				"resources":      resourcesCSV,
				"not_resources":  notResourcesCSV,
				// selection_tags — "k=v" comma-joined tag conditions from
				// BackupSelection.ListOfTags, required for the ec2:backup and
				// ebs:backup related-panel pivots (tag-based cross-ref, zero
				// extra calls beyond this already-loaded backup cache).
				"selection_tags": selectionTagsCSV,
			},
			RawStruct: plan,
		}
		if !complete {
			// The coverage join reads this as "this plan may select anything",
			// which is the difference between a plan that protects nothing and
			// a plan nobody could finish reading.
			r.Fields[backupSelectionsPartialField] = "true"
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// backupSelectionsPartialField marks a plan row whose selection enumeration
// did not reach the end of the list.
const backupSelectionsPartialField = "selections_partial"

// enumerateBackupPlanResources walks the plan's selections and returns
// comma-separated lists of resource ARNs, excluded ARNs, and tag-selection
// conditions covered by the plan, plus whether the plan's reach is fully known.
//
// The three lists correspond to BackupSelection.Resources (include list, may
// contain wildcards), BackupSelection.NotResources (exclude list, same
// wildcard semantics), and the plan's tag conditions rendered as "k=v" pairs
// with the "aws:ResourceTag/" ConditionKey prefix stripped so callers can
// match directly against a resource's own tags. Tag conditions arrive in two
// shapes AWS accepts interchangeably: the flat ListOfTags, and the structured
// Conditions whose StringEquals and StringLike carry the same key and value.
//
// complete is false when any call failed, the list ran past the page cap, or a
// selection carried a Conditions block the flat list cannot represent.
// Whatever was read is still returned: a partial list can only match more
// resources, and the flag is what stops the coverage join from reading a short
// list as a plan that protects nothing.
func enumerateBackupPlanResources(
	ctx context.Context,
	selectionAPI BackupListBackupSelectionsAPI,
	getSelectionAPI BackupGetBackupSelectionAPI,
	planID string,
) (string, string, string, bool) {
	if selectionAPI == nil || getSelectionAPI == nil || planID == "" {
		return "", "", "", false
	}
	var resources, notResources, selectionTags []string
	var nextToken *string
	complete := false
	representable := true
	for range PerParentPageCap {
		listOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.ListBackupSelectionsOutput, error) {
			return selectionAPI.ListBackupSelections(ctx, &backup.ListBackupSelectionsInput{
				BackupPlanId: aws.String(planID),
				NextToken:    nextToken,
			})
		})
		// Breaking leaves complete=false, which the caller renders as
		// selections_partial on the plan row, so the operator is told the
		// selection list is short.
		// no finding: the walk's completeness is the recorder here.
		if err != nil || listOut == nil {
			break
		}
		for _, sel := range listOut.BackupSelectionsList {
			if sel.SelectionId == nil {
				continue
			}
			selOut, selErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.GetBackupSelectionOutput, error) {
				return getSelectionAPI.GetBackupSelection(ctx, &backup.GetBackupSelectionInput{
					BackupPlanId: aws.String(planID),
					SelectionId:  sel.SelectionId,
				})
			})
			// The plan row carries selections_partial and never claims the
			// selection set is whole, exactly as for the list walk's break.
			// no finding: the false return is the recorder.
			if selErr != nil || selOut == nil || selOut.BackupSelection == nil {
				return strings.Join(resources, ","), strings.Join(notResources, ","), strings.Join(selectionTags, ","), false
			}
			resources = append(resources, selOut.BackupSelection.Resources...)
			notResources = append(notResources, selOut.BackupSelection.NotResources...)
			tags, folded := backupSelectionTagConditions(*selOut.BackupSelection)
			selectionTags = append(selectionTags, tags...)
			representable = representable && folded
		}
		if nextToken = listOut.NextToken; nextToken == nil || *nextToken == "" {
			complete = true
			break
		}
	}
	return strings.Join(resources, ","), strings.Join(notResources, ","), strings.Join(selectionTags, ","), complete && representable
}

// backupSelectionTagConditions renders one selection's tag conditions as "k=v"
// pairs, from both shapes AWS accepts, and says whether that rendering keeps
// the selection's meaning.
//
// ListOfTags is an OR of its entries, which is what the flat list is. A
// Conditions block is an AND across StringEquals, StringLike, StringNotEquals
// and StringNotLike, so the only block the flat list carries unchanged is one
// holding a single positive parameter. Anything else — an exclusion, or two
// parameters that must both hold — reads as wider than the plan really is, and
// a plan that looks wider hides the "not covered" finding this join exists to
// report. Such a block is not modelled; the selection is reported unfolded
// instead, and the caller turns that into an abstention.
func backupSelectionTagConditions(sel backuptypes.BackupSelection) (tags []string, representable bool) {
	// A condition AWS returned without a key or a value is one this fold drops,
	// and a dropped condition narrows what the plan appears to reach — the
	// direction that invents a "not covered" finding rather than hiding one. So
	// representable counts what folded, never what was merely present.
	dropped := 0
	add := func(key, value *string) {
		if key == nil || value == nil {
			dropped++
			return
		}
		tags = append(tags, strings.TrimPrefix(*key, "aws:ResourceTag/")+"="+*value)
	}
	for _, cond := range sel.ListOfTags {
		add(cond.ConditionKey, cond.ConditionValue)
	}
	c := sel.Conditions
	if c == nil {
		return tags, dropped == 0
	}
	positives := slices.Concat(c.StringEquals, c.StringLike)
	for _, cond := range positives {
		add(cond.ConditionKey, cond.ConditionValue)
	}
	return tags, dropped == 0 &&
		len(c.StringNotEquals)+len(c.StringNotLike) == 0 &&
		len(positives) <= 1
}
