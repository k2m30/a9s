// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

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
	optIn := backupRegionOptIn(ctx, api)

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

		sels, complete := enumerateBackupSelections(ctx, selectionAPI, getSelectionAPI, planID)

		resources = append(resources, resource.Resource{
			ID:   planID,
			Name: planName,
			Fields: map[string]string{
				"plan_name":      planName,
				"plan_id":        planID,
				"creation_date":  creationDate,
				"last_execution": lastExecution,
			},
			RawStruct: BackupPlanRow{BackupPlansListMember: plan, Selections: sels, selectionsComplete: complete, optIn: optIn},
		})
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

// BackupPlanRow is a backup plan row's RawStruct: the plan as listed, plus
// every selection of it the fetcher read. Coverage is decided per selection,
// so each selection is kept whole.
type BackupPlanRow struct {
	backuptypes.BackupPlansListMember
	Selections []backuptypes.BackupSelection

	selectionsComplete bool
	// optIn is the Region's ResourceTypeOptInPreference; nil when it was not
	// read.
	optIn map[string]bool
}

// backupRegionOptIn reads which resource types AWS Backup is switched on for
// in the Region, or nil when the call is refused.
func backupRegionOptIn(ctx context.Context, api BackupListBackupPlansAPI) map[string]bool {
	settingsAPI, ok := api.(BackupDescribeRegionSettingsAPI)
	if !ok {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.DescribeRegionSettingsOutput, error) {
		return settingsAPI.DescribeRegionSettings(ctx, &backup.DescribeRegionSettingsInput{})
	})
	// Every coverage reader turns a nil map into "cannot tell" where the opt-in
	// decides.
	// no finding: the nil return is the recorder.
	if err != nil || out == nil || out.ResourceTypeOptInPreference == nil {
		return nil
	}
	return out.ResourceTypeOptInPreference
}

// enumerateBackupSelections reads every selection of the plan, and reports
// whether it reached the end of the list. complete is false when any call
// failed or the list ran past the page cap; whatever was read is still
// returned, because a selection that was read covers what it covers whatever
// the unread ones say.
func enumerateBackupSelections(
	ctx context.Context,
	selectionAPI BackupListBackupSelectionsAPI,
	getSelectionAPI BackupGetBackupSelectionAPI,
	planID string,
) ([]backuptypes.BackupSelection, bool) {
	if selectionAPI == nil || getSelectionAPI == nil || planID == "" {
		return nil, false
	}
	var sels []backuptypes.BackupSelection
	var nextToken *string
	for range PerParentPageCap {
		listOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.ListBackupSelectionsOutput, error) {
			return selectionAPI.ListBackupSelections(ctx, &backup.ListBackupSelectionsInput{
				BackupPlanId: aws.String(planID),
				NextToken:    nextToken,
			})
		})
		// Every coverage reader turns the false return into "cannot tell".
		// no finding: the false return is the recorder.
		if err != nil || listOut == nil {
			return sels, false
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
			// no finding: the false return is the recorder, as for the list walk.
			if selErr != nil || selOut == nil || selOut.BackupSelection == nil {
				return sels, false
			}
			sels = append(sels, *selOut.BackupSelection)
		}
		if nextToken = listOut.NextToken; nextToken == nil || *nextToken == "" {
			return sels, true
		}
	}
	return sels, false
}
