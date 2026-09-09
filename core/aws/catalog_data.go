// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"net/url"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
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
	return colorFromFindings(athenaStateFindings(r.Fields["state"]))
}

var dataTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Glue Jobs",
		ShortName:     "glue",
		Aliases:       []string{"glue", "glue-jobs"},
		Category:      "DATA & ANALYTICS",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "gluestudio/home?region="+region+"#/editor/job/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "job_name", Title: "Job Name", Path: "Name", Width: 32},
			{Key: "state", Title: "Status", Width: 12},
			{Key: "last_run", Title: "Last Run", Width: 14},
			{Key: "glue_version", Title: "Version", Path: "GlueVersion", Width: 10},
			{Key: "worker_type", Title: "Worker Type", Path: "WorkerType", Width: 14},
			{Key: "num_workers", Title: "Workers", Path: "NumberOfWorkers", Width: 9},
			{Key: "last_modified", Title: "Last Modified", Path: "LastModifiedOn", Width: 22},
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
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkGlueAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkGlueLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkGlueCFN, Truncated: true},
			{TargetType: "s3", DisplayName: "S3 (script bucket)", Checker: checkGlueS3},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkGlueKMS},
			{TargetType: "athena", DisplayName: "Athena WorkGroups", Checker: checkGlueAthena, Truncated: true},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkGlueSecrets},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("glue")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Role", TargetType: "role"},
		},
		IssueEnricherFieldKeys: []string{"last_run"},
		Findings: []catalog.FindingDef{
			{Code: glueCodeLatestRunFailed, Phrase: "latest run <STATUS>", Severity: domain.SevBroken, Source: "wave2", Detail: "The job's most recent run did not finish, so whatever it feeds has been stale since then. Check the run's error message and CloudWatch logs for the cause, then rerun the job."},
			{Code: CodeGlueNoSecurityConfiguration, Phrase: "no security configuration", Severity: domain.SevWarn, Source: "wave1", Detail: "This job names no security configuration, so its S3 output, its CloudWatch log stream and its job bookmarks are all written without encryption at rest. Create a security configuration with a KMS key and attach it to the job."},
			{Code: CodeGlueContinuousLoggingOff, Phrase: "continuous logging off", Severity: domain.SevWarn, Source: "wave1", Detail: "Continuous logging is off, so driver and executor output only appears after the run ends and is lost entirely when a run is killed, leaving failures with no diagnostics. Add the continuous CloudWatch logging argument to the job's default arguments."},
			{Code: CodeGlueArgumentSecret, Phrase: "credential in job arguments", Severity: domain.SevBroken, Source: "wave1", Detail: "A default argument on this job holds what looks like a credential, and default arguments are readable by anyone who can describe the job and are echoed into run history. Move the value into Secrets Manager, pass its name instead, and rotate the exposed credential."},
		},
	},
	{
		Name:           "Athena Workgroups",
		ShortName:      "athena",
		HumanizeFields: []string{"state"},
		Aliases:        []string{"athena", "workgroups"},
		Category:       "DATA & ANALYTICS",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "athena/home?region="+region+"#/workgroups/details/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "workgroup_name", Title: "Workgroup", Path: "Name", Width: 28},
			{Title: "Status", Path: "State", Width: 12},
			{Title: "Cost Cap", Path: "Configuration.BytesScannedCutoffPerQuery", Width: 12},
			{Key: "description", Title: "Description", Path: "Description", Width: 30},
			{Key: "engine_version", Title: "Engine", Path: "EngineVersion.EffectiveEngineVersion", Width: 28},
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
			{Code: athenaCodeWorkgroupDisabled, Phrase: "disabled", Severity: domain.SevWarn, Source: "wave1", Detail: "Workgroup is administratively disabled — queries submitted against it are rejected until re-enabled."},
			{Code: athenaCodeSettingsNotEnforced, Phrase: "settings can be overridden per query", Severity: domain.SevWarn, Source: "wave2", Detail: "Every query submitted to this workgroup may override the settings it defines, so the result location and encryption configured here are advisory rather than binding. Turn on the workgroup's configuration enforcement so its settings apply to every query."},
			{Code: athenaCodeResultsUnencrypted, Phrase: "query results stored unencrypted", Severity: domain.SevWarn, Source: "wave2", Detail: "Query results are written to S3 with no encryption configured, so whatever a query returns is readable by anyone who can read the results bucket. Set an encryption option on the workgroup's result configuration."},
		},
	},
	{
		Name:           "Managed Airflow",
		ShortName:      "mwaa",
		HumanizeFields: []string{"webserver_access_mode", "endpoint_management", "last_update_status", "Status"},
		Aliases:        []string{"mwaa", "airflow"},
		Category:       "DATA & ANALYTICS",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "mwaa/home?region="+region+"#environments/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Path: "Name", Width: 32},
			{Key: "status", Title: "Status", Width: 32},
			{Key: "airflow_version", Title: "Airflow", Path: "AirflowVersion", Width: 10},
			{Key: "environment_class", Title: "Class", Path: "EnvironmentClass", Width: 14},
			{Key: "max_workers", Title: "Workers", Path: "MaxWorkers", Width: 9},
			{Key: "schedulers", Title: "Schedulers", Path: "Schedulers", Width: 10},
			{Key: "webserver_access_mode", Title: "Access", Path: "WebserverAccessMode", Width: 16},
			{Key: "created_at", Title: "Created", Path: "CreatedAt", Width: 22},
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
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkMWAAAlarms, NeedsTargetCache: true, Truncated: true},
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
			{Code: mwaaCodeCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1", Detail: "Environment is being provisioned; Airflow is not yet reachable."},
			{Code: mwaaCodeCreatingSnapshot, Phrase: "creating snapshot", Severity: domain.SevWarn, Source: "wave1", Detail: "The environment is snapshotting its metadata database before an update or upgrade."},
			{Code: mwaaCodePending, Phrase: "pending: awaiting VPC endpoints", Severity: domain.SevWarn, Source: "wave1", Detail: "Creation is paused until the required VPC endpoints exist in your VPC."},
			{Code: mwaaCodeUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1", Detail: "Environment update in progress; workers may be replaced."},
			{Code: mwaaCodeRollingBack, Phrase: "rolling back: update failed", Severity: domain.SevWarn, Source: "wave1", Detail: "Update or upgrade failed; the environment is restoring the latest metadata snapshot."},
			{Code: mwaaCodeMaintenance, Phrase: "maintenance in progress", Severity: domain.SevWarn, Source: "wave1", Detail: "Scheduled maintenance is running; the environment may be briefly unavailable."},
			{Code: mwaaCodeCreateFailed, Phrase: "create failed", Severity: domain.SevBroken, Source: "wave1", Detail: "Environment creation failed and the environment was not created."},
			{Code: mwaaCodeUpdateFailed, Phrase: "update failed: rolled back", Severity: domain.SevBroken, Source: "wave1", Detail: "Update failed; environment was restored to its previous state and is usable."},
			{Code: mwaaCodeUnavailable, Phrase: "unavailable: not stable", Severity: domain.SevBroken, Source: "wave1", Detail: "Environment failed and did not return to a stable state; contact AWS support."},
			{Code: mwaaCodeDeleting, Phrase: "deleting", Severity: domain.SevDim, Source: "wave1", Detail: "Environment is being deleted."},
			{Code: mwaaCodeDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1", Detail: "Environment has been deleted."},
			{Code: mwaaCodeLastUpdateFailed, Phrase: "last update failed", Severity: domain.SevWarn, Source: "wave1", Detail: "The last update to this environment failed, so it is still running its previous configuration; the error code and message are listed below. Fix the cause and update again."},
			{Code: mwaaCodeWebserverPublic, Phrase: "webserver public", Severity: domain.SevWarn, Source: "wave1", Detail: "The Airflow web server answers from the public internet, so its login page is reachable by anyone; the access mode is listed below. Switch the environment to private-only access from your VPC."},
			DetailsDeniedFindingDef("mwaa", "Access to environment details was denied; only the name is visible."),
			DetailsUnavailableFindingDef("mwaa"),
		},
	},
}

var dataChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "Job Runs",
		ShortName: "glue_runs",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			job := r.Fields["job_name"]
			if job == "" {
				return ""
			}
			return consolelink.Regional(region, "gluestudio/home?region="+region+"#/editor/job/"+url.PathEscape(job))
		},
		Columns:   resource.GlueRunColumns(),
		CopyField: "error_message",
		Color:     colorAnyFindingOrHealthy,
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
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			bucket := r.Fields["bucket"]
			key := r.Fields["key"]
			if bucket == "" || key == "" {
				return ""
			}
			return consolelink.Global(region, "s3/object/"+url.PathEscape(bucket)+"?prefix="+url.QueryEscape(key))
		},
		Columns:   resource.S3ObjectColumns(),
		FieldKeys: []string{"key", "size", "size_raw", "last_modified", "storage_class", "kind", "bucket"},
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
