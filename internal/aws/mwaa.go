package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/mwaa"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// mwaa.* FindingCodes — docs/resources/mwaa.md §3.2/§4. The eleven
// mwaaCode* state codes below classify Environment.Status; the two
// background codes (mwaaCodeLastUpdateFailed, mwaaCodeWebserverPublic) fire
// on ANY status. Every code here is color-bearing — colorMWAA in
// catalog_data.go derives row color from the worst-severity Finding present,
// with no state-vs-background distinction (docs/resources/mwaa.md §4: no
// glyph-on-green case exists for mwaa — every signal moves the row off
// green).
const (
	mwaaCodeCreating         domain.FindingCode = "mwaa.warn.creating"
	mwaaCodeCreatingSnapshot domain.FindingCode = "mwaa.warn.creating_snapshot"
	mwaaCodePending          domain.FindingCode = "mwaa.warn.pending"
	mwaaCodeUpdating         domain.FindingCode = "mwaa.warn.updating"
	mwaaCodeRollingBack      domain.FindingCode = "mwaa.warn.rolling_back"
	mwaaCodeMaintenance      domain.FindingCode = "mwaa.warn.maintenance"
	mwaaCodeCreateFailed     domain.FindingCode = "mwaa.broken.create_failed"
	mwaaCodeUpdateFailed     domain.FindingCode = "mwaa.broken.update_failed"
	mwaaCodeUnavailable      domain.FindingCode = "mwaa.broken.unavailable"
	mwaaCodeDeleting         domain.FindingCode = "mwaa.dim.deleting"
	mwaaCodeDeleted          domain.FindingCode = "mwaa.dim.deleted"
	mwaaCodeLastUpdateFailed domain.FindingCode = "mwaa.warn.last_update_failed"
	mwaaCodeWebserverPublic  domain.FindingCode = "mwaa.warn.webserver_public"
)

// mwaaListPageSize caps ListEnvironments MaxResults at the MWAA API limit —
// the shared DefaultPageSize (50) exceeds it and AWS rejects the call with a
// ValidationException ("Member must have value less than or equal to 25").
const mwaaListPageSize = 25

// mwaaDetailsDeniedDetail is mwaa's own §4 S5 sentence for the
// details-denied finding — deliberately NOT the generic degraded_resource.go
// text, matching transferDetailsDeniedDetail/ltDetailsDeniedDetail's
// precedent.
const mwaaDetailsDeniedDetail = "Access to environment details was denied; only the name is visible."

// FetchMWAAEnvironmentsPage fetches a single page of MWAA environments.
// ListEnvironments returns names only (docs/resources/mwaa.md §3.1 — no Wave
// 1 signal), so every field and every Finding comes from a per-name
// GetEnvironment call (in-fetcher N+1, the EKS pattern — no separate
// mwaa_issue_enrichment.go). Per-name GetEnvironment failures are aggregated
// (E3/E5) rather than dropping the whole page; a ListEnvironments failure
// returns an error, never an empty success (AccessDenied contract).
func FetchMWAAEnvironmentsPage(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
	input := &mwaa.ListEnvironmentsInput{
		MaxResults: aws.Int32(mwaaListPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*mwaa.ListEnvironmentsOutput, error) {
		return c.MWAA.ListEnvironments(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing MWAA environments: %w", err)
	}

	total := len(listOutput.Environments)
	var resources []resource.Resource
	var failures []string
	for _, name := range listOutput.Environments {
		getOutput, getErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*mwaa.GetEnvironmentOutput, error) {
			return c.MWAA.GetEnvironment(ctx, &mwaa.GetEnvironmentInput{Name: aws.String(name)})
		})
		if getErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", name, getErr.Error()))
			resources = append(resources, buildMWAADegradedResource(name))
			continue
		}
		if getOutput.Environment == nil {
			failures = append(failures, fmt.Sprintf("%s: nil environment in response", name))
			resources = append(resources, buildMWAADegradedResource(name))
			continue
		}
		resources = append(resources, buildMWAAResource(name, getOutput.Environment))
	}

	isTruncated := listOutput.NextToken != nil
	var nextToken string
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, AggregateFailures("mwaa-fetch: GetEnvironment", failures, total)
}

// buildMWAAResource constructs a Resource from a GetEnvironment response.
func buildMWAAResource(name string, env *mwaatypes.Environment) resource.Resource {
	findings, attentionDetails := computeMWAAFindings(env)

	rawStatus := string(env.Status)
	statusPhrase := phraseFromFindings(findings)
	if statusPhrase == "" && rawStatus != "" && rawStatus != string(mwaatypes.EnvironmentStatusAvailable) {
		// Defensive parity with rds.go's phraseFromFindings fallback: an
		// undocumented future status value still surfaces as raw text
		// instead of silently rendering blank. AVAILABLE with zero findings
		// legitimately stays "" — docs/resources/mwaa.md §4 "Healthy rows
		// render blank".
		statusPhrase = rawStatus
	}

	var securityGroupIDs, subnetIDs []string
	if env.NetworkConfiguration != nil {
		securityGroupIDs = env.NetworkConfiguration.SecurityGroupIds
		subnetIDs = env.NetworkConfiguration.SubnetIds
	}

	var dagProcessingLogGroup, schedulerLogGroup, webserverLogGroup, workerLogGroup, taskLogGroup string
	if env.LoggingConfiguration != nil {
		dagProcessingLogGroup = mwaaLogGroupNameFromARN(mwaaModuleLogGroupARN(env.LoggingConfiguration.DagProcessingLogs))
		schedulerLogGroup = mwaaLogGroupNameFromARN(mwaaModuleLogGroupARN(env.LoggingConfiguration.SchedulerLogs))
		webserverLogGroup = mwaaLogGroupNameFromARN(mwaaModuleLogGroupARN(env.LoggingConfiguration.WebserverLogs))
		workerLogGroup = mwaaLogGroupNameFromARN(mwaaModuleLogGroupARN(env.LoggingConfiguration.WorkerLogs))
		taskLogGroup = mwaaLogGroupNameFromARN(mwaaModuleLogGroupARN(env.LoggingConfiguration.TaskLogs))
	}

	lastUpdateStatus := ""
	lastUpdateError := ""
	if env.LastUpdate != nil {
		lastUpdateStatus = string(env.LastUpdate.Status)
		lastUpdateError = mwaaUpdateErrorText(env.LastUpdate.Error)
	}

	createdAt := ""
	if env.CreatedAt != nil {
		createdAt = env.CreatedAt.Format(time.RFC3339)
	}

	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"name":                      name,
			"status":                    statusPhrase,
			"airflow_version":           aws.ToString(env.AirflowVersion),
			"environment_class":         aws.ToString(env.EnvironmentClass),
			"min_workers":               mwaaInt32ToStr(env.MinWorkers),
			"max_workers":               mwaaInt32ToStr(env.MaxWorkers),
			"schedulers":                mwaaInt32ToStr(env.Schedulers),
			"min_webservers":            mwaaInt32ToStr(env.MinWebservers),
			"max_webservers":            mwaaInt32ToStr(env.MaxWebservers),
			"webserver_access_mode":     string(env.WebserverAccessMode),
			"endpoint_management":       string(env.EndpointManagement),
			"weekly_maintenance_window": aws.ToString(env.WeeklyMaintenanceWindowStart),
			"webserver_url":             aws.ToString(env.WebserverUrl),
			"arn":                       aws.ToString(env.Arn),
			"source_bucket_arn":         aws.ToString(env.SourceBucketArn),
			"dag_s3_path":               aws.ToString(env.DagS3Path),
			"kms_key":                   aws.ToString(env.KmsKey),
			"execution_role_arn":        aws.ToString(env.ExecutionRoleArn),
			"service_role_arn":          aws.ToString(env.ServiceRoleArn),
			"celery_executor_queue":     aws.ToString(env.CeleryExecutorQueue),
			"security_group_ids":        strings.Join(securityGroupIDs, ","),
			"subnet_ids":                strings.Join(subnetIDs, ","),
			"dag_processing_log_group":  dagProcessingLogGroup,
			"scheduler_log_group":       schedulerLogGroup,
			"webserver_log_group":       webserverLogGroup,
			"worker_log_group":          workerLogGroup,
			"task_log_group":            taskLogGroup,
			"created_at":                createdAt,
			"last_update_status":        lastUpdateStatus,
			"last_update_error":         lastUpdateError,
		},
		RawStruct:        env,
		Findings:         findings,
		AttentionDetails: attentionDetails,
	}
}

// buildMWAADegradedResource builds the name-only degraded row for an
// environment whose GetEnvironment call failed. ListEnvironments returns
// names only (docs/resources/mwaa.md §3.1), so unlike transfer/lt's rich
// degradation there are no list fields to carry forward — the shared
// details-denied finding is appended with mwaa's own §4 sentence rather than
// the generic degraded_resource.go text.
func buildMWAADegradedResource(name string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"name":   name,
			"status": detailsDeniedPhrase,
		},
		RawStruct: &mwaatypes.Environment{Name: aws.String(name)},
		Findings: []domain.Finding{{
			Code:     DetailsDeniedCode("mwaa"),
			Phrase:   detailsDeniedPhrase,
			Detail:   mwaaDetailsDeniedDetail,
			Severity: domain.SevWarn,
			Source:   "wave1",
		}},
	}
}

// computeMWAAFindings builds the ordered Finding slice for one environment:
// the state-bucket finding (if Status != AVAILABLE), then the
// last-update-failed background finding, then the webserver-public
// background finding — docs/resources/mwaa.md §4 precedence order. Both
// background findings apply regardless of Status (they stack after the
// state finding on a non-green row, and are the only findings on an
// AVAILABLE row).
func computeMWAAFindings(env *mwaatypes.Environment) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	var findings []domain.Finding
	var attentionDetails map[domain.FindingCode]domain.AttentionDetail

	if f, ok := mwaaStateFinding(env.Status); ok {
		findings = append(findings, f)
	}

	if env.LastUpdate != nil && env.LastUpdate.Status == mwaatypes.UpdateStatusFailed {
		errCode := ""
		errMessage := ""
		if env.LastUpdate.Error != nil {
			errCode = aws.ToString(env.LastUpdate.Error.ErrorCode)
			errMessage = aws.ToString(env.LastUpdate.Error.ErrorMessage)
		}
		findings = append(findings, domain.Finding{
			Code:     mwaaCodeLastUpdateFailed,
			Phrase:   "last update failed",
			Detail:   fmt.Sprintf("Last update failed: %s. Still on previous config.", errMessage),
			Severity: domain.SevWarn,
			Source:   "wave1",
		})
		attentionDetails = map[domain.FindingCode]domain.AttentionDetail{
			mwaaCodeLastUpdateFailed: {
				Rows: []domain.DetailRow{
					{Label: "Error Code", Value: domain.HumanizeStatusPhrase(errCode)},
					{Label: "Error Message", Value: errMessage},
				},
			},
		}
	}

	switch env.WebserverAccessMode {
	case mwaatypes.WebserverAccessModePublicOnly, mwaatypes.WebserverAccessModePublicAndPrivate:
		findings = append(findings, domain.Finding{
			Code:   mwaaCodeWebserverPublic,
			Phrase: "webserver public",
			Detail: fmt.Sprintf("Airflow webserver is reachable from the internet (access mode: %s).",
				domain.HumanizeStatusPhrase(string(env.WebserverAccessMode))),
			Severity: domain.SevWarn,
			Source:   "wave1",
		})
	}

	return findings, attentionDetails
}

// mwaaStateFinding maps Environment.Status to its docs/resources/mwaa.md §4
// state-bucket Finding. ok is false for AVAILABLE (Healthy — no finding).
func mwaaStateFinding(status mwaatypes.EnvironmentStatus) (domain.Finding, bool) {
	switch status {
	case mwaatypes.EnvironmentStatusCreating:
		return domain.Finding{
			Code: mwaaCodeCreating, Phrase: "creating",
			Detail:   "Environment is being provisioned; Airflow is not yet reachable.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusCreatingSnapshot:
		return domain.Finding{
			Code: mwaaCodeCreatingSnapshot, Phrase: "creating snapshot",
			Detail:   "The environment is snapshotting its metadata database before an update or upgrade.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusPending:
		return domain.Finding{
			Code: mwaaCodePending, Phrase: "pending: awaiting VPC endpoints",
			Detail:   "Creation is paused until the required VPC endpoints exist in your VPC.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusUpdating:
		return domain.Finding{
			Code: mwaaCodeUpdating, Phrase: "updating",
			Detail:   "Environment update in progress; workers may be replaced.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusRollingBack:
		return domain.Finding{
			Code: mwaaCodeRollingBack, Phrase: "rolling back: update failed",
			Detail:   "Update or upgrade failed; the environment is restoring the latest metadata snapshot.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusMaintenance:
		return domain.Finding{
			Code: mwaaCodeMaintenance, Phrase: "maintenance in progress",
			Detail:   "Scheduled maintenance is running; the environment may be briefly unavailable.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusCreateFailed:
		return domain.Finding{
			Code: mwaaCodeCreateFailed, Phrase: "create failed",
			Detail:   "Environment creation failed and the environment was not created.",
			Severity: domain.SevBroken, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusUpdateFailed:
		return domain.Finding{
			Code: mwaaCodeUpdateFailed, Phrase: "update failed: rolled back",
			Detail:   "Update failed; environment was restored to its previous state and is usable.",
			Severity: domain.SevBroken, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusUnavailable:
		return domain.Finding{
			Code: mwaaCodeUnavailable, Phrase: "unavailable: not stable",
			Detail:   "Environment failed and did not return to a stable state; contact AWS support.",
			Severity: domain.SevBroken, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusDeleting:
		return domain.Finding{
			Code: mwaaCodeDeleting, Phrase: "deleting",
			Detail:   "Environment is being deleted.",
			Severity: domain.SevDim, Source: "wave1",
		}, true
	case mwaatypes.EnvironmentStatusDeleted:
		return domain.Finding{
			Code: mwaaCodeDeleted, Phrase: "deleted",
			Detail:   "Environment has been deleted.",
			Severity: domain.SevDim, Source: "wave1",
		}, true
	}
	return domain.Finding{}, false
}

// mwaaUpdateErrorText formats a LastUpdate.Error as a single detail-view
// fact string ("<code>: <message>"), falling back to whichever half is
// present. Returns "" when err is nil or both halves are empty.
func mwaaUpdateErrorText(err *mwaatypes.UpdateError) string {
	if err == nil {
		return ""
	}
	code := aws.ToString(err.ErrorCode)
	message := aws.ToString(err.ErrorMessage)
	switch {
	case code != "" && message != "":
		return fmt.Sprintf("%s: %s", code, message)
	case message != "":
		return message
	default:
		return code
	}
}

// mwaaModuleLogGroupARN extracts CloudWatchLogGroupArn from a
// ModuleLoggingConfiguration, returning "" for a nil pointer or field.
func mwaaModuleLogGroupARN(m *mwaatypes.ModuleLoggingConfiguration) string {
	if m == nil {
		return ""
	}
	return aws.ToString(m.CloudWatchLogGroupArn)
}

// mwaaLogGroupNameFromARN extracts the bare log group name from a CloudWatch
// Logs log-group ARN (arn:aws:logs:region:account:log-group:NAME:*),
// mirroring opensearch_related.go's checkOpenSearchLogs extraction. Returns
// "" for an empty or malformed input.
func mwaaLogGroupNameFromARN(arn string) string {
	if arn == "" {
		return ""
	}
	_, name, ok := strings.Cut(arn, ":log-group:")
	if !ok {
		return ""
	}
	return strings.TrimSuffix(name, ":*")
}

// mwaaInt32ToStr renders an *int32 field as a decimal string, "" for nil.
func mwaaInt32ToStr(v *int32) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(int(*v))
}
