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
		LifecycleKey:  "status",
		Aliases:       []string{"backup", "backup-plans"},
		Category:      "BACKUP",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "backup/home?region="+region+"#/backupplan/details/"+r.ID)
		},
		Columns: []domain.Column{
			{Key: "plan_name", Title: "Plan Name", Path: "BackupPlanName", Width: 32},
			{Key: "status", Title: "Status", Width: 40},
			{Key: "plan_id", Title: "Plan ID", Path: "BackupPlanId", Width: 38},
			{Key: "creation_date", Title: "Created", Path: "CreationDate", Width: 22},
			{Key: "last_execution", Title: "Last Execution", Path: "LastExecutionDate", Width: 22},
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
			{Code: backupCodeJobFailed, Phrase: "<N job(s)> failed in last 24h", Severity: domain.SevBroken, Source: "wave2", Detail: "A backup job for this plan did not complete in the last day, so the recovery points you expect for that window do not exist. Open the job for its status message: an IAM permission, a resource deleted mid-job, or a vault lock rule are the usual causes. A plan cannot be rerun, so once it is fixed either start an on-demand backup for each resource that was missed or wait for the next scheduled run."},
			{Code: backupCodeJobPartial, Phrase: "partial: <N> of <M resource(s)> skipped", Severity: domain.SevWarn, Source: "wave2", Detail: "The plan ran but skipped some of the resources it selects, so those resources have no recovery point for this window even though the job reports progress. Check the job's resource list against the plan's selection, and the backup role's permissions on the resources that were missed."},
		},
	},
}

var backupChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:         "Stack Events",
		ShortName:    "cfn_events",
		LifecycleKey: "resource_status",
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
			{Code: CodeCfnEventFailed, Phrase: "<failure status, lowercased>", Severity: domain.SevBroken, Source: "wave1", Detail: "This resource operation failed. What happens next depends on the stack's failure options and on what else depended on this resource: CloudFormation may roll the stack back, keep the resources that already succeeded, or carry on with independent ones. Read this event's status reason and then the stack's terminal status before acting."},
			{Code: CodeCfnEventInProgress, Phrase: "<in-progress status, lowercased>", Severity: domain.SevWarn, Source: "wave1", Detail: "The step is still running, so the stack is mid-change and its resources may be replaced or briefly unavailable until it finishes. Wait for the matching completion event rather than starting another operation on the stack."},
			{Code: CodeCfnEventDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"},
		},
	},
	{
		Name:         "Stack Resources",
		ShortName:    "cfn_resources",
		LifecycleKey: "resource_status",
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
			{Code: CodeCfnResourceFailed, Phrase: "<failure status, lowercased>", Severity: domain.SevBroken, Source: "wave1", Detail: "CloudFormation could not create, update or delete this resource, so the stack no longer matches its template and a rollback may have left the resource behind. Read the status reason, fix the underlying cause, then retry the stack operation or import the resource back."},
			{Code: CodeCfnResourceInProgress, Phrase: "<in-progress status, lowercased>", Severity: domain.SevWarn, Source: "wave1", Detail: "The resource is mid-operation, so its live configuration matches neither the old template nor the new one yet. Wait for it to settle before reading its attributes or starting another stack change."},
			{Code: CodeCfnResourceDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"},
		},
	},
}
