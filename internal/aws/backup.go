package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchBackupPlans calls the Backup ListBackupPlans API and returns a slice of
// generic Resource structs.
func FetchBackupPlans(ctx context.Context, api BackupListBackupPlansAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchBackupPlansPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchBackupPlansPage fetches a single page of Backup plans.
func FetchBackupPlansPage(ctx context.Context, api BackupListBackupPlansAPI, continuationToken string) (resource.FetchResult, error) {
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

		// Enumerate the plan's selection resource ARNs so sibling pivots
		// (s3, ddb, efs, dbi, …) can match via cache scan. One
		// ListBackupSelections + one GetBackupSelection per selection —
		// bounded by plan count; selections per plan typically ≤3.
		resourcesCSV, notResourcesCSV := enumerateBackupPlanResources(ctx, selectionAPI, getSelectionAPI, planID)

		r := resource.Resource{
			ID:    planID,
			Name:  planName,
			Fields: map[string]string{
				"plan_name":      planName,
				"plan_id":        planID,
				"creation_date":  creationDate,
				"last_execution": lastExecution,
				"resources":      resourcesCSV,
				"not_resources":  notResourcesCSV,
			},
			RawStruct: plan,
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

// enumerateBackupPlanResources walks the plan's selections and returns
// comma-separated lists of resource ARNs and excluded ARNs covered by the
// plan. Returns ("", "") when the plan has no selections, the API shape is
// unavailable, or any enumeration call fails. The two return values correspond
// to BackupSelection.Resources (include list, may contain wildcards) and
// BackupSelection.NotResources (exclude list, same wildcard semantics).
func enumerateBackupPlanResources(
	ctx context.Context,
	selectionAPI BackupListBackupSelectionsAPI,
	getSelectionAPI BackupGetBackupSelectionAPI,
	planID string,
) (string, string) {
	if selectionAPI == nil || getSelectionAPI == nil || planID == "" {
		return "", ""
	}
	listOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.ListBackupSelectionsOutput, error) {
		return selectionAPI.ListBackupSelections(ctx, &backup.ListBackupSelectionsInput{
			BackupPlanId: aws.String(planID),
		})
	})
	if err != nil || listOut == nil {
		return "", ""
	}
	var resources, notResources []string
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
		if selErr != nil || selOut == nil || selOut.BackupSelection == nil {
			// fail closed — partial enumeration would drop NotResources exclusions,
			// causing false-positive backup coverage. Return empty pair so the caller
			// degrades cleanly rather than claiming incorrect coverage.
			return "", ""
		}
		resources = append(resources, selOut.BackupSelection.Resources...)
		notResources = append(notResources, selOut.BackupSelection.NotResources...)
	}
	return strings.Join(resources, ","), strings.Join(notResources, ",")
}
