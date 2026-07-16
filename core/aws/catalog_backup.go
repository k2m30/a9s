// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/catalog"
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
		Columns: []domain.Column{
			{Key: "plan_name", Title: "Plan Name", Width: 32, Sortable: true},
			{Key: "plan_id", Title: "Plan ID", Width: 38, Sortable: true},
			{Key: "creation_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "last_execution", Title: "Last Execution", Width: 22, Sortable: true},
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
			{Code: backupCodeJobFailed, Phrase: "<N> jobs failed in last 24h", Severity: domain.SevBroken, Source: "wave2"},
			{Code: backupCodeJobPartial, Phrase: "partial: <N> of <M> resources skipped", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
}

var backupChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "Stack Events",
		ShortName: "cfn_events",
		Columns:   resource.CfnEventColumns(),
		Color:     colorWave1OrHealthy,
		FieldKeys: []string{
			"timestamp", "logical_resource_id", "resource_type",
			"resource_status", "resource_status_reason",
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
		Columns:   resource.CfnResourceColumns(),
		Color:     colorWave1OrHealthy,
		FieldKeys: []string{
			"logical_resource_id", "physical_resource_id", "resource_type",
			"resource_status", "drift_status", "last_updated",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchCfnResources(ctx, c.CloudFormation, parentCtx["stack_name"], continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeCfnResourceFailed, Phrase: "<status, lowercased>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCfnResourceInProgress, Phrase: "<status, lowercased>", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeCfnResourceDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"},
		},
	},
}
