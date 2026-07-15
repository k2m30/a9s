// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// colorGlue prefers colorFromAnyFinding so real fetched resources (Findings
// populated by EnrichGlueJobStatus, Source: "wave2") color from their own
// Finding; glue jobs carry no other structural signal at fetch time, so
// there is no raw-field fallback to keep.
func colorGlue(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

// colorAthena prefers colorFromAnyFinding so real fetched resources
// (Findings populated by the fetcher's DISABLED-state wave1 Finding, Source:
// "wave1", and EnrichAthenaWorkGroup, Source: "wave2:athena") color from
// their own Finding; the raw-field switch below is the identical-precedence
// fallback for callers that construct a Resource with only Fields set.
func colorAthena(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
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
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchGlueJobsPage(ctx, c.Glue, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichGlueJobStatus, Priority: 10},
		FieldKeys: []string{"job_name", "glue_version", "worker_type", "num_workers", "last_modified"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkGlueRole},
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
		Findings: []catalog.FindingDef{
			{Code: glueCodeLatestRunFailed, Phrase: "latest run <STATUS>", Severity: domain.SevBroken, Source: "wave2"},
		},
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
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchAthenaWorkgroupsPage(ctx, c.Athena, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichAthenaWorkGroup, Priority: 100},
		FieldKeys: []string{"workgroup_name", "state", "description", "engine_version", "result_output_location"},
		Related: []domain.RelatedDef{
			{TargetType: "s3", DisplayName: "S3 Buckets (results)", Checker: checkAthenaS3},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkAthenaKMS},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkAthenaLogs},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkAthenaRole},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("athena")},
		},
		Findings: []catalog.FindingDef{
			{Code: athenaCodeWorkgroupDisabled, Phrase: "disabled", Severity: domain.SevWarn, Source: "wave1"},
			{Code: athenaCodeGovernanceMisconfigured, Phrase: "EnforceWorkGroupConfiguration (<N> findings)", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "Managed Airflow",
		ShortName:     "mwaa",
		Aliases:       []string{"mwaa", "airflow"},
		Category:      "DATA & ANALYTICS",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 32, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "airflow_version", Title: "Airflow", Width: 10, Sortable: true},
			{Key: "environment_class", Title: "Class", Width: 14, Sortable: true},
			{Key: "max_workers", Title: "Workers", Width: 9, Sortable: true},
			{Key: "schedulers", Title: "Schedulers", Width: 10, Sortable: true},
			{Key: "webserver_access_mode", Title: "Access", Width: 16, Sortable: true},
			{Key: "created_at", Title: "Created", Width: 22, Sortable: true},
		},
		Color:   colorAnyFindingOrHealthy,
		Fetcher: fetcherWithClients(FetchMWAAEnvironmentsPage),
		FieldKeys: []string{
			"name", "status", "airflow_version", "environment_class",
			"min_workers", "max_workers", "schedulers", "min_webservers", "max_webservers",
			"webserver_access_mode", "endpoint_management", "weekly_maintenance_window",
			"webserver_url", "arn", "source_bucket_arn", "dag_s3_path", "kms_key",
			"execution_role_arn", "service_role_arn", "celery_executor_queue",
			"security_group_ids", "subnet_ids",
			"dag_processing_log_group", "scheduler_log_group", "webserver_log_group",
			"worker_log_group", "task_log_group",
			"created_at", "last_update_status", "last_update_error",
		},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkMWAAAlarms, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkMWAAKMS},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkMWAALogs},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkMWAARole},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkMWAAS3},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkMWAASG},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkMWAASubnet},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("mwaa")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "ExecutionRoleArn", TargetType: "role"},
			{FieldPath: "KmsKey", TargetType: "kms"},
			{FieldPath: "SourceBucketArn", TargetType: "s3"},
		},
		Findings: []catalog.FindingDef{
			{Code: mwaaCodeCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: mwaaCodeCreatingSnapshot, Phrase: "creating snapshot", Severity: domain.SevWarn, Source: "wave1"},
			{Code: mwaaCodePending, Phrase: "pending: awaiting VPC endpoints", Severity: domain.SevWarn, Source: "wave1"},
			{Code: mwaaCodeUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: mwaaCodeRollingBack, Phrase: "rolling back: update failed", Severity: domain.SevWarn, Source: "wave1"},
			{Code: mwaaCodeMaintenance, Phrase: "maintenance in progress", Severity: domain.SevWarn, Source: "wave1"},
			{Code: mwaaCodeCreateFailed, Phrase: "create failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: mwaaCodeUpdateFailed, Phrase: "update failed: rolled back", Severity: domain.SevBroken, Source: "wave1"},
			{Code: mwaaCodeUnavailable, Phrase: "unavailable: not stable", Severity: domain.SevBroken, Source: "wave1"},
			{Code: mwaaCodeDeleting, Phrase: "deleting", Severity: domain.SevDim, Source: "wave1"},
			{Code: mwaaCodeDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"},
			{Code: mwaaCodeLastUpdateFailed, Phrase: "last update failed", Severity: domain.SevWarn, Source: "wave1"},
			{Code: mwaaCodeWebserverPublic, Phrase: "webserver public", Severity: domain.SevWarn, Source: "wave1"},
			DetailsDeniedFindingDef("mwaa"),
			DetailsUnavailableFindingDef("mwaa"),
		},
	},
}

var dataChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "Job Runs",
		ShortName: "glue_runs",
		Columns:   resource.GlueRunColumns(),
		CopyField: "error_message",
		Color:     colorWave1OrHealthy,
		FieldKeys: []string{
			"run_id_short", "job_run_state", "started_on",
			"execution_time_human", "error_message", "dpu_hours",
			"run_id", "job_name",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchGlueJobRuns(ctx, c.Glue, parentCtx["job_name"], continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeGlueRunFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeGlueRunTimeout, Phrase: "timeout", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeGlueRunError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeGlueRunExpired, Phrase: "expired", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeGlueRunRunning, Phrase: "running", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeGlueRunStarting, Phrase: "starting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeGlueRunStopping, Phrase: "stopping", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeGlueRunWaiting, Phrase: "waiting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeGlueRunStopped, Phrase: "stopped", Severity: domain.SevDim, Source: "wave1"},
		},
	},
	{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
		FieldKeys: []string{"key", "size", "last_modified", "storage_class"},
		Children: []domain.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "@parent.bucket", "prefix": "ID"},
			DisplayNameKey: "bucket",
			DrillCondition: func(r domain.Resource) bool { return r.Fields["kind"] == "folder" },
		}},
		// RelatedContextFromIDs extracts the bucket name from related IDs encoded as
		// "bucket|key". Used when navigating to s3_objects from the related panel
		// (e.g., from a CloudTrail event detail view).
		RelatedContextFromIDs: func(relatedIDs []string) map[string]string {
			for _, id := range relatedIDs {
				parts := strings.SplitN(id, "|", 2)
				if len(parts) != 2 || parts[0] == "" {
					continue
				}
				bucket := parts[0]
				key := parts[1]
				// Derive the prefix (folder path) from the key so the child view
				// lands on the folder containing the object, not the bucket root.
				// Example: key="prod/config.json" → prefix="prod/"
				// Example: key="landing/2026/04/07/x.parquet" → prefix="landing/2026/04/07/"
				// Example: key="build-4821.tar.gz" → prefix=""
				prefix := ""
				if idx := strings.LastIndex(key, "/"); idx >= 0 {
					prefix = key[:idx+1]
				}
				return map[string]string{"bucket": bucket, "prefix": prefix}
			}
			return map[string]string{"bucket": "", "prefix": ""}
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchS3Objects(ctx, c.S3, parentCtx["bucket"], parentCtx["prefix"], continuationToken)
		}),
	},
}
