package aws

import (
	"context"
	"fmt"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func colorGlue(_ domain.Resource) domain.Color { return domain.ColorHealthy }

func colorAthena(r domain.Resource) domain.Color {
	switch r.Fields["state"] {
	case "ENABLED":
		return domain.ColorHealthy
	case "DISABLED":
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

var dataTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Glue Jobs",
		ShortName:     "glue",
		Aliases:       []string{"glue", "glue-jobs"},
		Category:      "DATA & ANALYTICS",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "job_name", Title: "Job Name", Width: 32, Sortable: true},
			{Key: "glue_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "worker_type", Title: "Worker Type", Width: 14, Sortable: true},
			{Key: "num_workers", Title: "Workers", Width: 9, Sortable: true},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "glue_runs",
			Key:            "enter",
			ContextKeys:    map[string]string{"job_name": "ID"},
			DisplayNameKey: "job_name",
		}},
		Color: colorGlue,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchGlueJobsPage(ctx, c.Glue, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichGlueJobStatus, Priority: 10},
		FieldKeys: []string{"job_name", "glue_version", "worker_type", "num_workers", "last_modified"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkGlueRole, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkGlueAlarms, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkGlueLogs, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkGlueCFN},
			{TargetType: "s3", DisplayName: "S3 (script bucket)", Checker: checkGlueS3},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkGlueKMS},
			{TargetType: "athena", DisplayName: "Athena WorkGroups", Checker: checkGlueAthena},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkGlueSecrets},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("glue")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Role", TargetType: "role"},
		},
		IssueEnricherFieldKeys: []string{"last_run"},
	},
	{
		Name:          "Athena Workgroups",
		ShortName:     "athena",
		Aliases:       []string{"athena", "workgroups"},
		Category:      "DATA & ANALYTICS",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "workgroup_name", Title: "Workgroup", Width: 28, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
			{Key: "engine_version", Title: "Engine", Width: 28, Sortable: true},
		},
		Color: colorAthena,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchAthenaWorkgroupsPage(ctx, c.Athena, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichAthenaWorkGroup, Priority: 100},
		FieldKeys: []string{"workgroup_name", "state", "description", "engine_version", "result_output_location"},
		Related: []domain.RelatedDef{
			{TargetType: "s3", DisplayName: "S3 Buckets (results)", Checker: checkAthenaS3},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkAthenaKMS},
			{TargetType: "glue", DisplayName: "Glue Data Catalog", Checker: checkAthenaGlue},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkAthenaLogs},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkAthenaRole},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("athena")},
		},
	},
}
