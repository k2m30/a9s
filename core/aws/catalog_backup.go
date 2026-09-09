// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// colorBackup classifies a Backup plan. All signals come from
// EnrichBackupJobs (backup_issue_enrichment.go, Source: "wave2:backup") —
// spec §3.1 has zero Wave 1 signals for this resource.
func colorBackup(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

var backupTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Backup Plans",
		ShortName:     "backup",
		Aliases:       []string{"backup", "backup-plans"},
		Category:      "BACKUP",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "backup/home?region="+region+"#/backupplan/details/"+r.ID)
		},
		Columns: []domain.Column{
			{Key: "plan_name", Title: "Plan Name", Path: "BackupPlanName", Width: 32, Sortable: true},
			{Key: "status", Title: "Status", Width: 40},
			{Key: "plan_id", Title: "Plan ID", Path: "BackupPlanId", Width: 38, Sortable: true},
			{Key: "creation_date", Title: "Created", Path: "CreationDate", Width: 22, Sortable: true},
			{Key: "last_execution", Title: "Last Execution", Path: "LastExecutionDate", Width: 22, Sortable: true},
		},
		// Wave 2 enricher surfaces plans whose recent backup jobs have
		// failed — Wave 1 list is declarative config.
		Color: colorBackup,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchBackupPlansPage(ctx, c.Backup, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichBackupJobs, Priority: 100},
		// selection_tags — required for the ec2:backup and ebs:backup
		// related-panel pivots (tag-based selection cross-ref).
		FieldKeys: []string{"plan_name", "plan_id", "creation_date", "last_execution", "resources", "not_resources", "selection_tags"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkBackupRole},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkBackupKMS},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkBackupSNS, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("backup")},
		},
		IssueEnricherFieldKeys: []string{"status"},
		Findings: []catalog.FindingDef{
			{Code: backupCodeJobFailed, Phrase: "<N job(s)> failed in last 24h", Severity: domain.SevBroken, Source: "wave2"},
			{Code: backupCodeJobPartial, Phrase: "partial: <N> of <M resource(s)> skipped", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
}

var backupChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "Stack Events",
		ShortName: "cfn_events",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["stack_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "cloudformation/home?region="+region+"#/stacks/events?stackId="+arn)
		},
		Columns: resource.CfnEventColumns(),
		Color:   colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"timestamp", "logical_resource_id", "resource_type",
			"resource_status", "resource_status_reason", "stack_arn",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchCfnEvents(ctx, c.CloudFormation, parentCtx["stack_name"], continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeCfnEventFailed, Phrase: "<status, lowercased>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCfnEventInProgress, Phrase: "<status, lowercased>", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeCfnEventDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"},
		},
	},
	{
		Name:      "Stack Resources",
		ShortName: "cfn_resources",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["stack_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "cloudformation/home?region="+region+"#/stacks/resources?stackId="+arn)
		},
		Columns: resource.CfnResourceColumns(),
		Color:   colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"logical_resource_id", "physical_resource_id", "resource_type",
			"resource_status", "drift_status", "last_updated", "stack_arn",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			result, err := FetchCfnResources(ctx, c.CloudFormation, parentCtx["stack_name"], continuationToken)
			if err != nil {
				return result, err
			}
			// StackResourceSummary carries no stack ARN of its own (unlike
			// StackEvent's StackId), so it's stamped on from parentCtx here
			// rather than threaded through FetchCfnResources' own signature.
			for i := range result.Resources {
				result.Resources[i].Fields["stack_arn"] = parentCtx["stack_arn"]
			}
			return result, nil
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeCfnResourceFailed, Phrase: "<status, lowercased>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCfnResourceInProgress, Phrase: "<status, lowercased>", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeCfnResourceDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"},
		},
	},
}
