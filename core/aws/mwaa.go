// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// mwaa.* FindingCodes (docs/resources/mwaa.md). The eleven mwaaCode* state
// codes below classify Environment.Status; the two background codes
// (mwaaCodeLastUpdateFailed, mwaaCodeWebserverPublic) stand on every live
// status.
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
	mwaaCodeUnrecognised     domain.FindingCode = "mwaa.warn.unrecognised_status"
)

// mwaaListPageSize caps ListEnvironments MaxResults at the MWAA API limit —
// the shared DefaultPageSize (50) exceeds it and AWS rejects the call with a
// ValidationException ("Member must have value less than or equal to 25").
const mwaaListPageSize = 25

// FetchMWAAEnvironmentsPage fetches a single page of MWAA environments.
// ListEnvironments returns names only, so every field and every Finding
// comes from a per-name GetEnvironment call (in-fetcher N+1, as for EKS).
// Per-name GetEnvironment failures are aggregated rather than dropping the
// whole page; a ListEnvironments failure returns an error, never an empty
// success (AccessDenied contract).
func FetchMWAAEnvironmentsPage(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
	input := &mwaa.ListEnvironmentsInput{
		MaxResults: aws.Int32(mwaaListPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	listOutput, err := c.MWAA.ListEnvironments(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing MWAA environments: %w", err)
	}

	var resources []resource.Resource
	var tally describeTally
	for _, name := range listOutput.Environments {
		rows := tally.one("GetEnvironment", name, name, func() (*resource.Resource, error) {
			out, err := c.MWAA.GetEnvironment(ctx, &mwaa.GetEnvironmentInput{Name: aws.String(name)})
			if err != nil {
				return nil, err
			}
			if out.Environment == nil {
				return nil, UnusableAnswerErr{Call: "GetEnvironment", Field: "environment"}
			}
			r := buildMWAAResource(name, out.Environment)
			return &r, nil
		}, func(err error) resource.Resource { return buildMWAADegradedResource(name, err) })
		resources = append(resources, rows...)
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: listOutput.NextToken != nil,
			NextToken:   aws.ToString(listOutput.NextToken),
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, tally.err("mwaa-fetch: GetEnvironment")
}

// buildMWAAResource constructs a Resource from a GetEnvironment response.
func buildMWAAResource(name string, env *mwaatypes.Environment) resource.Resource {
	findings, attentionDetails := computeMWAAFindings(env)

	var securityGroupIDs, subnetIDs []string
	if env.NetworkConfiguration != nil {
		securityGroupIDs = env.NetworkConfiguration.SecurityGroupIds
		subnetIDs = env.NetworkConfiguration.SubnetIds
	}

	var dagProcessingLogGroup, schedulerLogGroup, webserverLogGroup, workerLogGroup, taskLogGroup string
	if env.LoggingConfiguration != nil {
		dagProcessingLogGroup = mwaaLogGroup(env.LoggingConfiguration.DagProcessingLogs)
		schedulerLogGroup = mwaaLogGroup(env.LoggingConfiguration.SchedulerLogs)
		webserverLogGroup = mwaaLogGroup(env.LoggingConfiguration.WebserverLogs)
		workerLogGroup = mwaaLogGroup(env.LoggingConfiguration.WorkerLogs)
		taskLogGroup = mwaaLogGroup(env.LoggingConfiguration.TaskLogs)
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

	return withWave1("mwaa", resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"name":                      name,
			"status":                    string(env.Status),
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
		AttentionDetails: attentionDetails,
	}, findings...)
}

// buildMWAADegradedResource builds the name-only degraded row for an
// environment whose GetEnvironment call failed — ListEnvironments returns
// names only, so there are no list fields to
// carry forward. err is the GetEnvironment call's error (nil for a nil
// Environment body); it decides whether the row renders mwaa's own "details
// denied" sentence or the neutral "details unavailable" one.
func buildMWAADegradedResource(name string, err error) resource.Resource {
	finding := degradedDetailsFinding("mwaa", err)
	return withWave1("mwaa", resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"name": name,
		},
	}, []domain.Finding{finding}...)
}

// computeMWAAFindings builds the ordered Finding slice for one environment:
// the state-bucket finding (if Status != AVAILABLE), then the
// last-update-failed background finding, then the webserver-public
// background finding — the precedence order in docs/resources/mwaa.md.
func computeMWAAFindings(env *mwaatypes.Environment) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	var findings []domain.Finding
	var attentionDetails map[domain.FindingCode]domain.AttentionDetail

	if env.LastUpdate != nil && env.LastUpdate.Status == mwaatypes.UpdateStatusFailed {
		errCode := ""
		errMessage := ""
		if env.LastUpdate.Error != nil {
			errCode = aws.ToString(env.LastUpdate.Error.ErrorCode)
			errMessage = aws.ToString(env.LastUpdate.Error.ErrorMessage)
		}
		findings = append(findings, wave1Finding(mwaaCodeLastUpdateFailed))
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
		findings = append(findings, wave1Finding(mwaaCodeWebserverPublic))
		if attentionDetails == nil {
			attentionDetails = map[domain.FindingCode]domain.AttentionDetail{}
		}
		attentionDetails[mwaaCodeWebserverPublic] = domain.AttentionDetail{Rows: []domain.DetailRow{
			{Label: "Access mode", Value: domain.HumanizeStatusPhrase(string(env.WebserverAccessMode)), Tier: "~"},
		}}
	}

	return withState("mwaa", string(env.Status), findings), attentionDetails
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

// mwaaLogGroup extracts the bare log group name from a
// ModuleLoggingConfiguration's CloudWatchLogGroupArn, returning "" for a nil
// pointer or field.
func mwaaLogGroup(m *mwaatypes.ModuleLoggingConfiguration) string {
	if m == nil {
		return ""
	}
	return mwaaLogGroupNameFromARN(aws.ToString(m.CloudWatchLogGroupArn))
}

// mwaaLogGroupNameFromARN is the log group a CloudWatch Logs log-group ARN
// names, or "" for an empty or malformed input.
func mwaaLogGroupNameFromARN(arn string) string {
	name, _ := logsRefToID(arn, domain.RefContext{})
	return name
}

// mwaaInt32ToStr renders an *int32 field as a decimal string, "" for nil.
func mwaaInt32ToStr(v *int32) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(int(*v))
}
