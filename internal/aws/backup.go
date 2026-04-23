package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("backup", []string{"plan_name", "plan_id", "creation_date", "last_execution", "resources"})

	resource.RegisterPaginated("backup", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchBackupPlansPage(ctx, c.Backup, continuationToken)
	})
}

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

	output, err := api.ListBackupPlans(ctx, input)
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
		resourcesCSV := enumerateBackupPlanResources(ctx, selectionAPI, getSelectionAPI, planID)

		r := resource.Resource{
			ID:     planID,
			Name:   planName,
			Status: "",
			Fields: map[string]string{
				"plan_name":      planName,
				"plan_id":        planID,
				"creation_date":  creationDate,
				"last_execution": lastExecution,
				"resources":      resourcesCSV,
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

// enumerateBackupPlanResources walks the plan's selections and returns a
// comma-separated list of every resource ARN covered by the plan. Returns
// "" when the plan has no selections, the API shape is unavailable, or any
// enumeration call fails (partial data is still better than zero, but an
// empty aggregation lets the caller degrade cleanly to the "no resources"
// signal rather than surfacing a half-built list).
func enumerateBackupPlanResources(
	ctx context.Context,
	selectionAPI BackupListBackupSelectionsAPI,
	getSelectionAPI BackupGetBackupSelectionAPI,
	planID string,
) string {
	if selectionAPI == nil || getSelectionAPI == nil || planID == "" {
		return ""
	}
	listOut, err := selectionAPI.ListBackupSelections(ctx, &backup.ListBackupSelectionsInput{
		BackupPlanId: aws.String(planID),
	})
	if err != nil || listOut == nil {
		return ""
	}
	var arns []string
	for _, sel := range listOut.BackupSelectionsList {
		if sel.SelectionId == nil {
			continue
		}
		selOut, selErr := getSelectionAPI.GetBackupSelection(ctx, &backup.GetBackupSelectionInput{
			BackupPlanId: aws.String(planID),
			SelectionId:  sel.SelectionId,
		})
		if selErr != nil || selOut == nil || selOut.BackupSelection == nil {
			continue
		}
		arns = append(arns, selOut.BackupSelection.Resources...)
	}
	return strings.Join(arns, ",")
}
