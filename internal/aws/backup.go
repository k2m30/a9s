package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/backup"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("backup", []string{"plan_name", "plan_id", "creation_date", "last_execution"})
	resource.Register("backup", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchBackupPlans(ctx, c.Backup)
	})
}

// FetchBackupPlans calls the Backup ListBackupPlans API and returns a slice of
// generic Resource structs.
func FetchBackupPlans(ctx context.Context, api BackupListBackupPlansAPI) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := api.ListBackupPlans(ctx, &backup.ListBackupPlansInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching Backup plans: %w", err)
		}

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

			r := resource.Resource{
				ID:     planID,
				Name:   planName,
				Status: "",
				Fields: map[string]string{
					"plan_name":      planName,
					"plan_id":        planID,
					"creation_date":  creationDate,
					"last_execution": lastExecution,
				},
				RawStruct: plan,
			}

			resources = append(resources, r)
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}
