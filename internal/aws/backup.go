package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/backup"

	"github.com/k2m30/a9s/internal/resource"
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
	output, err := api.ListBackupPlans(ctx, &backup.ListBackupPlansInput{})
	if err != nil {
		return nil, err
	}

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

		planArn := ""
		if plan.BackupPlanArn != nil {
			planArn = *plan.BackupPlanArn
		}

		creationDate := ""
		if plan.CreationDate != nil {
			creationDate = plan.CreationDate.Format("2006-01-02T15:04:05Z07:00")
		}

		lastExecution := ""
		if plan.LastExecutionDate != nil {
			lastExecution = plan.LastExecutionDate.Format("2006-01-02T15:04:05Z07:00")
		}

		versionID := ""
		if plan.VersionId != nil {
			versionID = *plan.VersionId
		}

		deletionDate := ""
		if plan.DeletionDate != nil {
			deletionDate = plan.DeletionDate.Format("2006-01-02T15:04:05Z07:00")
		}

		// Build DetailData
		detail := map[string]string{
			"Plan Name":      planName,
			"Plan ID":        planID,
			"Plan ARN":       planArn,
			"Creation Date":  creationDate,
			"Last Execution": lastExecution,
			"Deletion Date":  deletionDate,
			"Version ID":     versionID,
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(plan, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  plan,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
