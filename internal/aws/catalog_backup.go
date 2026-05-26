package aws

import (
	"context"
	"fmt"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func colorBackup(_ domain.Resource) domain.Color { return domain.ColorHealthy }

var backupTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Backup Plans",
		ShortName:     "backup",
		Aliases:       []string{"backup", "backup-plans"},
		Category:      "BACKUP",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "plan_name", Title: "Plan Name", Width: 32, Sortable: true},
			{Key: "plan_id", Title: "Plan ID", Width: 38, Sortable: true},
			{Key: "creation_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "last_execution", Title: "Last Execution", Width: 22, Sortable: true},
		},
		// Wave 2 enricher surfaces plans whose recent backup jobs have
		// failed — Wave 1 list is declarative config.
		Color: colorBackup,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchBackupPlansPage(ctx, c.Backup, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichBackupJobs, Priority: 100},
		FieldKeys: []string{"plan_name", "plan_id", "creation_date", "last_execution", "resources", "not_resources"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkBackupRole},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkBackupKMS},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkBackupSNS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("backup")},
		},
		IssueEnricherFieldKeys: []string{"status"},
	},
}

var backupChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "Stack Events",
		ShortName: "cfn_events",
		Columns:   resource.CfnEventColumns(),
		FieldKeys: []string{
			"timestamp", "logical_resource_id", "resource_type",
			"resource_status", "resource_status_reason",
		},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCfnEvents(ctx, c.CloudFormation, parentCtx["stack_name"], continuationToken)
		},
	},
	{
		Name:      "Stack Resources",
		ShortName: "cfn_resources",
		Columns:   resource.CfnResourceColumns(),
		Color:     colorWave1OrHealthy,
		FieldKeys: []string{
			"logical_resource_id", "physical_resource_id", "resource_type",
			"resource_status", "drift_status", "last_updated",
		},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCfnResources(ctx, c.CloudFormation, parentCtx["stack_name"], continuationToken)
		},
	},
}
