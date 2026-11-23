package main

import (
	"cmp"
	"context"
	"encoding/json"
	"maps"
	"net/url"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	backupsvc "github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	caTypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// ---------------------------------------------------------------------------
// cb — CodeBuild Projects
// ---------------------------------------------------------------------------

type cbData struct {
	Projects []cbProject `json:"projects"`
}

type cbProject struct {
	Name string `json:"name"`
	readOutcome
	ServiceRole      string       `json:"service_role,omitempty"`
	EncryptionKey    string       `json:"encryption_key,omitempty"`
	SourceType       string       `json:"source_type,omitempty"`
	SourceLocation   string       `json:"source_location,omitempty"`
	ArtifactsType    string       `json:"artifacts_type,omitempty"`
	ArtifactsLoc     string       `json:"artifacts_location,omitempty"`
	EnvImage         string       `json:"env_image,omitempty"`
	LogsGroupName    string       `json:"logs_group_name,omitempty"`
	LogsStatus       string       `json:"logs_status,omitempty"`
	VpcId            string       `json:"vpc_id,omitempty"`
	SecurityGroupIds []string     `json:"security_group_ids,omitempty"`
	SubnetIds        []string     `json:"subnet_ids,omitempty"`
	EnvVars          []cbEnvVar   `json:"env_vars,omitempty"`
	LatestBuild      *cbBuildInfo `json:"latest_build,omitempty"`
}

type cbEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
	Type  string `json:"type"`
}

// A PLAINTEXT value (the type CodeBuild assumes when none is set) is the secret
// itself; the reference types hold only the Parameter Store name or Secrets
// Manager id where the secret lives.
func cbEnvVarOf(ev cbtypes.EnvironmentVariable) cbEnvVar {
	v := cbEnvVar{Name: aws.ToString(ev.Name), Type: string(ev.Type)}
	switch ev.Type {
	case cbtypes.EnvironmentVariableTypeParameterStore, cbtypes.EnvironmentVariableTypeSecretsManager:
		v.Value = aws.ToString(ev.Value)
	}
	return v
}

// A Git source URL may embed the repository credential as userinfo. CodeCommit
// ARNs and S3 paths are not URLs and pass through unchanged.
func cbSourceLocationOf(loc string) string {
	u, err := url.Parse(loc)
	if err != nil || u.Scheme == "" || u.User == nil {
		return loc
	}
	u.User = nil
	return u.String()
}

type cbBuildInfo struct {
	Outcome      string `json:"outcome"`
	ErrorCode    string `json:"error_code,omitempty"`
	BuildStatus  string `json:"build_status,omitempty"`
	CurrentPhase string `json:"current_phase,omitempty"`
	EndTime      string `json:"end_time,omitempty"`
}

func captureCB(ctx context.Context, cfg aws.Config) (any, error) {
	client := codebuild.NewFromConfig(cfg)

	var names []string
	paginator := codebuild.NewListProjectsPaginator(client, &codebuild.ListProjectsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		names = append(names, out.Projects...)
	}

	rows := describeListed(names, 100, func(batch []string) (map[string]cbtypes.Project, []string, error) {
		out, err := client.BatchGetProjects(ctx, &codebuild.BatchGetProjectsInput{Names: batch})
		if err != nil {
			return nil, nil, err
		}
		found := map[string]cbtypes.Project{}
		for _, p := range out.Projects {
			found[aws.ToString(p.Name)] = p
		}
		return found, out.ProjectsNotFound, nil
	})

	projects := make([]cbProject, 0, len(rows))
	for _, row := range rows {
		proj := cbProject{Name: row.id, readOutcome: row.read}
		if p := row.item; p != nil {
			proj.ServiceRole = aws.ToString(p.ServiceRole)
			proj.EncryptionKey = aws.ToString(p.EncryptionKey)
			if p.Source != nil {
				proj.SourceType = string(p.Source.Type)
				proj.SourceLocation = cbSourceLocationOf(aws.ToString(p.Source.Location))
			}
			if p.Artifacts != nil {
				proj.ArtifactsType = string(p.Artifacts.Type)
				proj.ArtifactsLoc = aws.ToString(p.Artifacts.Location)
			}
			if p.Environment != nil {
				proj.EnvImage = aws.ToString(p.Environment.Image)
				for _, ev := range p.Environment.EnvironmentVariables {
					proj.EnvVars = append(proj.EnvVars, cbEnvVarOf(ev))
				}
			}
			if p.LogsConfig != nil && p.LogsConfig.CloudWatchLogs != nil {
				proj.LogsGroupName = aws.ToString(p.LogsConfig.CloudWatchLogs.GroupName)
				proj.LogsStatus = string(p.LogsConfig.CloudWatchLogs.Status)
			}
			if p.VpcConfig != nil {
				proj.VpcId = aws.ToString(p.VpcConfig.VpcId)
				proj.SecurityGroupIds = p.VpcConfig.SecurityGroupIds
				proj.SubnetIds = p.VpcConfig.Subnets
			}
		}
		projects = append(projects, proj)
	}
	captureLatestBuilds(ctx, client, projects)

	return cbData{Projects: projects}, nil
}

// BatchGetBuilds takes at most 100 ids.
func captureLatestBuilds(ctx context.Context, client *codebuild.Client, projects []cbProject) {
	var ids []string
	var asking []int
	for i := range projects {
		id, info := captureLatestBuild(ctx, client, projects[i].Name)
		projects[i].LatestBuild = info
		if info == nil {
			ids = append(ids, id)
			asking = append(asking, i)
		}
	}
	rows := describeListed(ids, 100, func(batch []string) (map[string]cbtypes.Build, []string, error) {
		out, err := client.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{Ids: batch})
		if err != nil {
			return nil, nil, err
		}
		found := map[string]cbtypes.Build{}
		for _, b := range out.Builds {
			found[aws.ToString(b.Id)] = b
		}
		return found, out.BuildsNotFound, nil
	})
	for i, id := range ids {
		row := rowOf(rows, id)
		info := &cbBuildInfo{Outcome: row.read.Outcome, ErrorCode: row.read.ErrorCode}
		if b := row.item; b != nil {
			info = &cbBuildInfo{Outcome: "found", BuildStatus: string(b.BuildStatus), CurrentPhase: aws.ToString(b.CurrentPhase), EndTime: snapFormatTime(b.EndTime)}
		}
		projects[asking[i]].LatestBuild = info
	}
}

// captureLatestBuild returns the id of the project's newest build, or the
// record of a project with no build to read.
func captureLatestBuild(ctx context.Context, client *codebuild.Client, projectName string) (string, *cbBuildInfo) {
	listed, err := pageAll(ctx, &codebuild.ListBuildsForProjectInput{ProjectName: aws.String(projectName)},
		func(ctx context.Context, in *codebuild.ListBuildsForProjectInput) (*codebuild.ListBuildsForProjectOutput, error) {
			return client.ListBuildsForProject(ctx, in)
		},
		func(out *codebuild.ListBuildsForProjectOutput) []string { return out.Ids }, onePage)
	if err != nil {
		info := &cbBuildInfo{}
		info.Outcome, info.ErrorCode = outcomeOf(err)
		return "", info
	}
	if len(listed) == 0 {
		return "", &cbBuildInfo{Outcome: outcomeNone}
	}
	return listed[0], nil
}

// ---------------------------------------------------------------------------
// pipeline — CodePipelines
// ---------------------------------------------------------------------------

type pipelineData struct {
	Pipelines []pipelineInfo `json:"pipelines"`
}

type pipelineInfo struct {
	Name  string         `json:"name"`
	State *pipelineState `json:"state,omitempty"`
}

type pipelineState struct {
	Outcome string        `json:"outcome"`
	Code    string        `json:"error_code,omitempty"`
	Stages  []pipelineStg `json:"stages,omitempty"`
}

type pipelineStg struct {
	StageName          string `json:"stage_name"`
	LatestStatus       string `json:"latest_execution_status,omitempty"`
	LatestStartTime    string `json:"latest_start_time,omitempty"`
	LatestLastUpdate   string `json:"latest_last_status_change,omitempty"`
	ApprovalTokenExist bool   `json:"approval_token_pending,omitempty"`
}

func capturePipeline(ctx context.Context, cfg aws.Config) (any, error) {
	client := codepipeline.NewFromConfig(cfg)

	var names []string
	paginator := codepipeline.NewListPipelinesPaginator(client, &codepipeline.ListPipelinesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range out.Pipelines {
			names = append(names, aws.ToString(p.Name))
		}
	}

	var pipelines []pipelineInfo
	for _, name := range names {
		info := pipelineInfo{Name: name}

		stateOut, err := client.GetPipelineState(ctx, &codepipeline.GetPipelineStateInput{Name: aws.String(name)})
		info.State = &pipelineState{}
		info.State.Outcome, info.State.Code = outcomeOf(err)
		if stateOut != nil {
			for _, s := range stateOut.StageStates {
				stg := pipelineStg{StageName: aws.ToString(s.StageName)}
				if s.LatestExecution != nil {
					stg.LatestStatus = string(s.LatestExecution.Status)
				}
				for _, a := range s.ActionStates {
					if a.LatestExecution != nil && a.LatestExecution.Token != nil {
						stg.ApprovalTokenExist = true
					}
					if a.LatestExecution != nil && a.LatestExecution.LastStatusChange != nil {
						stg.LatestLastUpdate = snapFormatTime(a.LatestExecution.LastStatusChange)
					}
				}
				info.State.Stages = append(info.State.Stages, stg)
			}
		}

		pipelines = append(pipelines, info)
	}

	return pipelineData{Pipelines: pipelines}, nil
}

// ---------------------------------------------------------------------------
// codeartifact — CodeArtifact Repos
// ---------------------------------------------------------------------------

type codeArtifactData struct {
	Repositories []codeArtifactRepo `json:"repositories"`
}

type codeArtifactRepo struct {
	Name                string `json:"name"`
	Arn                 string `json:"arn,omitempty"`
	DomainName          string `json:"domain_name,omitempty"`
	DomainOwner         string `json:"domain_owner,omitempty"`
	AdministratorAcct   string `json:"administrator_account,omitempty"`
	CreatedTime         string `json:"created_time,omitempty"`
	Description         string `json:"description,omitempty"`
	PackageCountOutcome string `json:"package_count_outcome"`
	PackageCountCode    string `json:"package_count_error_code,omitempty"`
	PackageCount        int    `json:"package_count,omitempty"`
}

func captureCodeArtifact(ctx context.Context, cfg aws.Config) (any, error) {
	client := codeartifact.NewFromConfig(cfg)

	var repos []codeArtifactRepo
	paginator := codeartifact.NewListRepositoriesPaginator(client, &codeartifact.ListRepositoriesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range out.Repositories {
			repo := codeArtifactRepo{
				Name:              aws.ToString(r.Name),
				Arn:               aws.ToString(r.Arn),
				DomainName:        aws.ToString(r.DomainName),
				DomainOwner:       aws.ToString(r.DomainOwner),
				AdministratorAcct: aws.ToString(r.AdministratorAccount),
				CreatedTime:       snapFormatTime(r.CreatedTime),
				Description:       aws.ToString(r.Description),
			}

			pkgs, err := pageAll(ctx, &codeartifact.ListPackagesInput{
				Domain:      r.DomainName,
				DomainOwner: r.DomainOwner,
				Repository:  r.Name,
			},
				func(ctx context.Context, in *codeartifact.ListPackagesInput) (*codeartifact.ListPackagesOutput, error) {
					return client.ListPackages(ctx, in)
				},
				func(out *codeartifact.ListPackagesOutput) []caTypes.PackageSummary { return out.Packages }, allPages)
			repo.PackageCountOutcome, repo.PackageCountCode = outcomeOf(err)
			repo.PackageCount = len(pkgs)

			repos = append(repos, repo)
		}
	}

	return codeArtifactData{Repositories: repos}, nil
}

// ---------------------------------------------------------------------------
// cfn — CloudFormation Stacks
// ---------------------------------------------------------------------------

type cfnData struct {
	Stacks []cfnStack `json:"stacks"`
}

type cfnStack struct {
	StackId             string        `json:"stack_id"`
	StackName           string        `json:"stack_name"`
	StackStatus         string        `json:"stack_status"`
	StackStatusReason   string        `json:"stack_status_reason,omitempty"`
	CreationTime        string        `json:"creation_time,omitempty"`
	LastUpdatedTime     string        `json:"last_updated_time,omitempty"`
	ParentId            string        `json:"parent_id,omitempty"`
	RootId              string        `json:"root_id,omitempty"`
	RoleARN             string        `json:"role_arn,omitempty"`
	NotificationARNs    []string      `json:"notification_arns,omitempty"`
	DriftStatus         string        `json:"drift_status,omitempty"`
	DriftLastCheckTime  string        `json:"drift_last_check_time,omitempty"`
	RecentEventsOutcome string        `json:"recent_events_outcome"`
	RecentEventsCode    string        `json:"recent_events_error_code,omitempty"`
	RecentFailedEvent   *cfnEventInfo `json:"recent_failed_event,omitempty"`
}

type cfnEventInfo struct {
	LogicalResourceId    string `json:"logical_resource_id,omitempty"`
	ResourceStatus       string `json:"resource_status,omitempty"`
	ResourceStatusReason string `json:"resource_status_reason,omitempty"`
	Timestamp            string `json:"timestamp,omitempty"`
}

func captureCFN(ctx context.Context, cfg aws.Config) (any, error) {
	client := cloudformation.NewFromConfig(cfg)

	var stacks []cfnStack
	paginator := cloudformation.NewDescribeStacksPaginator(client, &cloudformation.DescribeStacksInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range out.Stacks {
			stack := cfnStack{
				StackId:           aws.ToString(s.StackId),
				StackName:         aws.ToString(s.StackName),
				StackStatus:       string(s.StackStatus),
				StackStatusReason: aws.ToString(s.StackStatusReason),
				CreationTime:      snapFormatTime(s.CreationTime),
				LastUpdatedTime:   snapFormatTime(s.LastUpdatedTime),
				ParentId:          aws.ToString(s.ParentId),
				RootId:            aws.ToString(s.RootId),
				RoleARN:           aws.ToString(s.RoleARN),
				NotificationARNs:  s.NotificationARNs,
			}
			if s.DriftInformation != nil {
				stack.DriftStatus = string(s.DriftInformation.StackDriftStatus)
				stack.DriftLastCheckTime = snapFormatTime(s.DriftInformation.LastCheckTimestamp)
			}

			events, err := pageAll(ctx, &cloudformation.DescribeStackEventsInput{StackName: s.StackName},
				func(ctx context.Context, in *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error) {
					return client.DescribeStackEvents(ctx, in)
				},
				func(out *cloudformation.DescribeStackEventsOutput) []cfntypes.StackEvent { return out.StackEvents }, onePage)
			stack.RecentEventsOutcome, stack.RecentEventsCode = outcomeOf(err)
			for _, ev := range events {
				if string(ev.ResourceStatus) != "" && stackEventIsFailed(ev.ResourceStatus) {
					stack.RecentFailedEvent = &cfnEventInfo{
						LogicalResourceId:    aws.ToString(ev.LogicalResourceId),
						ResourceStatus:       string(ev.ResourceStatus),
						ResourceStatusReason: aws.ToString(ev.ResourceStatusReason),
						Timestamp:            snapFormatTime(ev.Timestamp),
					}
					break
				}
			}

			stacks = append(stacks, stack)
		}
	}

	return cfnData{Stacks: stacks}, nil
}

func stackEventIsFailed(status cfntypes.ResourceStatus) bool {
	s := string(status)
	return len(s) >= 6 && s[len(s)-6:] == "FAILED"
}

// ---------------------------------------------------------------------------
// alarm — CloudWatch Alarms
// ---------------------------------------------------------------------------

type alarmData struct {
	Alarms []alarmInfo `json:"alarms"`
}

type alarmInfo struct {
	AlarmName               string           `json:"alarm_name"`
	AlarmArn                string           `json:"alarm_arn,omitempty"`
	StateValue              string           `json:"state_value"`
	StateReason             string           `json:"state_reason,omitempty"`
	StateUpdatedTimestamp   string           `json:"state_updated_timestamp,omitempty"`
	ActionsEnabled          bool             `json:"actions_enabled"`
	AlarmActions            []string         `json:"alarm_actions,omitempty"`
	OKActions               []string         `json:"ok_actions,omitempty"`
	InsufficientDataActions []string         `json:"insufficient_data_actions,omitempty"`
	Namespace               string           `json:"namespace,omitempty"`
	MetricName              string           `json:"metric_name,omitempty"`
	Period                  int32            `json:"period,omitempty"`
	Dimensions              []alarmDimension `json:"dimensions,omitempty"`
}

type alarmDimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func captureAlarm(ctx context.Context, cfg aws.Config) (any, error) {
	client := cloudwatch.NewFromConfig(cfg)

	var alarms []alarmInfo
	paginator := cloudwatch.NewDescribeAlarmsPaginator(client, &cloudwatch.DescribeAlarmsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, a := range out.MetricAlarms {
			info := alarmInfo{
				AlarmName:               aws.ToString(a.AlarmName),
				AlarmArn:                aws.ToString(a.AlarmArn),
				StateValue:              string(a.StateValue),
				StateReason:             aws.ToString(a.StateReason),
				StateUpdatedTimestamp:   snapFormatTime(a.StateUpdatedTimestamp),
				ActionsEnabled:          aws.ToBool(a.ActionsEnabled),
				AlarmActions:            a.AlarmActions,
				OKActions:               a.OKActions,
				InsufficientDataActions: a.InsufficientDataActions,
				Namespace:               aws.ToString(a.Namespace),
				MetricName:              aws.ToString(a.MetricName),
				Period:                  aws.ToInt32(a.Period),
			}
			for _, d := range a.Dimensions {
				info.Dimensions = append(info.Dimensions, alarmDimension{
					Name:  aws.ToString(d.Name),
					Value: aws.ToString(d.Value),
				})
			}
			alarms = append(alarms, info)
		}
	}

	return alarmData{Alarms: alarms}, nil
}

// ---------------------------------------------------------------------------
// logs — CloudWatch Log Groups
// ---------------------------------------------------------------------------

type logsData struct {
	LogGroups []logGroupInfo `json:"log_groups"`
}

type logGroupInfo struct {
	LogGroupName       string `json:"log_group_name"`
	Arn                string `json:"arn,omitempty"`
	CreationTime       string `json:"creation_time,omitempty"`
	RetentionInDaysSet bool   `json:"retention_in_days_set"`
	RetentionInDays    int32  `json:"retention_in_days,omitempty"`
	StoredBytes        int64  `json:"stored_bytes"`
	KmsKeyId           string `json:"kms_key_id,omitempty"`
	MetricFilterCount  int32  `json:"metric_filter_count"`
	LastEventOutcome   string `json:"last_event_outcome"`
	LastEventErrorCode string `json:"last_event_error_code,omitempty"`
	LastEventTimestamp string `json:"last_event_timestamp,omitempty"`
}

func captureLogs(ctx context.Context, cfg aws.Config) (any, error) {
	client := cloudwatchlogs.NewFromConfig(cfg)

	var groups []logGroupInfo
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(client, &cloudwatchlogs.DescribeLogGroupsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range out.LogGroups {
			info := logGroupInfo{
				LogGroupName:      aws.ToString(g.LogGroupName),
				Arn:               aws.ToString(g.Arn),
				CreationTime:      opsFormatEpochMillis(g.CreationTime),
				StoredBytes:       aws.ToInt64(g.StoredBytes),
				KmsKeyId:          aws.ToString(g.KmsKeyId),
				MetricFilterCount: aws.ToInt32(g.MetricFilterCount),
			}
			if g.RetentionInDays != nil {
				info.RetentionInDaysSet = true
				info.RetentionInDays = *g.RetentionInDays
			}

			streams, err := pageAll(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
				LogGroupName: g.LogGroupName,
				OrderBy:      "LastEventTime",
				Descending:   aws.Bool(true),
				Limit:        aws.Int32(1),
			},
				func(ctx context.Context, in *cloudwatchlogs.DescribeLogStreamsInput) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
					return client.DescribeLogStreams(ctx, in)
				},
				func(out *cloudwatchlogs.DescribeLogStreamsOutput) []cwltypes.LogStream { return out.LogStreams }, onePage)
			info.LastEventOutcome, info.LastEventErrorCode = outcomeOf(err)
			if len(streams) > 0 {
				info.LastEventTimestamp = opsFormatEpochMillis(streams[0].LastEventTimestamp)
			}

			groups = append(groups, info)
		}
	}

	return logsData{LogGroups: groups}, nil
}

func opsFormatEpochMillis(ms *int64) string {
	if ms == nil {
		return ""
	}
	t := time.UnixMilli(*ms).UTC()
	return t.Format("2006-01-02 15:04")
}

// ---------------------------------------------------------------------------
// trail — CloudTrail Trails
// ---------------------------------------------------------------------------

type trailData struct {
	Trails []trailInfo `json:"trails"`
}

type trailInfo struct {
	Name                      string `json:"name"`
	TrailARN                  string `json:"trail_arn,omitempty"`
	HomeRegion                string `json:"home_region,omitempty"`
	S3BucketName              string `json:"s3_bucket_name,omitempty"`
	SnsTopicARN               string `json:"sns_topic_arn,omitempty"`
	KmsKeyId                  string `json:"kms_key_id,omitempty"`
	CloudWatchLogsLogGroupArn string `json:"cloudwatch_logs_log_group_arn,omitempty"`
	CloudWatchLogsRoleArn     string `json:"cloudwatch_logs_role_arn,omitempty"`
	LogFileValidationEnabled  bool   `json:"log_file_validation_enabled"`
	IsMultiRegionTrail        bool   `json:"is_multi_region_trail"`
	IsOrganizationTrail       bool   `json:"is_organization_trail"`
	StatusOutcome             string `json:"status_outcome"`
	StatusErrorCode           string `json:"status_error_code,omitempty"`
	IsLogging                 bool   `json:"is_logging,omitempty"`
	LatestDeliveryError       string `json:"latest_delivery_error,omitempty"`
	LatestDeliveryTime        string `json:"latest_delivery_time,omitempty"`
	StopLoggingTime           string `json:"stop_logging_time,omitempty"`
	StartLoggingTime          string `json:"start_logging_time,omitempty"`
}

func captureTrail(ctx context.Context, cfg aws.Config) (any, error) {
	client := cloudtrail.NewFromConfig(cfg)

	descOut, err := client.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{})
	if err != nil {
		return nil, err
	}

	var trails []trailInfo
	for _, t := range descOut.TrailList {
		info := trailInfo{
			Name:                      aws.ToString(t.Name),
			TrailARN:                  aws.ToString(t.TrailARN),
			HomeRegion:                aws.ToString(t.HomeRegion),
			S3BucketName:              aws.ToString(t.S3BucketName),
			SnsTopicARN:               aws.ToString(t.SnsTopicARN),
			KmsKeyId:                  aws.ToString(t.KmsKeyId),
			CloudWatchLogsLogGroupArn: aws.ToString(t.CloudWatchLogsLogGroupArn),
			CloudWatchLogsRoleArn:     aws.ToString(t.CloudWatchLogsRoleArn),
			LogFileValidationEnabled:  aws.ToBool(t.LogFileValidationEnabled),
			IsMultiRegionTrail:        aws.ToBool(t.IsMultiRegionTrail),
			IsOrganizationTrail:       aws.ToBool(t.IsOrganizationTrail),
		}

		statusOut, err := client.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{Name: t.TrailARN})
		info.StatusOutcome, info.StatusErrorCode = outcomeOf(err)
		if statusOut != nil {
			info.IsLogging = aws.ToBool(statusOut.IsLogging)
			info.LatestDeliveryError = aws.ToString(statusOut.LatestDeliveryError)
			info.LatestDeliveryTime = snapFormatTime(statusOut.LatestDeliveryTime)
			info.StopLoggingTime = snapFormatTime(statusOut.StopLoggingTime)
			info.StartLoggingTime = snapFormatTime(statusOut.StartLoggingTime)
		}

		trails = append(trails, info)
	}

	return trailData{Trails: trails}, nil
}

// ---------------------------------------------------------------------------
// ct-events — CloudTrail Events (newest 200, truncated marker)
// ---------------------------------------------------------------------------

type ctEventsData struct {
	Events    []ctEventInfo `json:"events"`
	Truncated bool          `json:"truncated"`
}

type ctEventInfo struct {
	EventId         string   `json:"event_id"`
	EventName       string   `json:"event_name,omitempty"`
	EventTime       string   `json:"event_time,omitempty"`
	Username        string   `json:"username,omitempty"`
	EventSource     string   `json:"event_source,omitempty"`
	ReadOnly        string   `json:"read_only,omitempty"`
	AccessKeyId     string   `json:"access_key_id,omitempty"`
	CloudTrailEvent string   `json:"cloud_trail_event,omitempty"`
	ResourceTypes   []string `json:"resource_types,omitempty"`
	ResourceNames   []string `json:"resource_names,omitempty"`
}

const ctEventsCap = 200

// requestParameters and responseElements echo the API call's own payload,
// which carries whatever the caller sent (environment variables, job
// arguments); the rest of the event is CloudTrail's envelope.
func ctEventEnvelopeOf(raw string) string {
	var event map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &event) != nil {
		return ""
	}
	delete(event, "requestParameters")
	delete(event, "responseElements")
	out, err := json.Marshal(event)
	if err != nil {
		return ""
	}
	return string(out)
}

func captureCTEvents(ctx context.Context, cfg aws.Config) (any, error) {
	client := cloudtrail.NewFromConfig(cfg)

	var events []ctEventInfo
	paginator := cloudtrail.NewLookupEventsPaginator(client, &cloudtrail.LookupEventsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range out.Events {
			if len(events) == ctEventsCap {
				return ctEventsData{Events: events, Truncated: true}, nil
			}
			info := ctEventInfo{
				EventId:         aws.ToString(e.EventId),
				EventName:       aws.ToString(e.EventName),
				EventTime:       snapFormatTime(e.EventTime),
				Username:        aws.ToString(e.Username),
				EventSource:     aws.ToString(e.EventSource),
				ReadOnly:        aws.ToString(e.ReadOnly),
				AccessKeyId:     aws.ToString(e.AccessKeyId),
				CloudTrailEvent: ctEventEnvelopeOf(aws.ToString(e.CloudTrailEvent)),
			}
			for _, r := range e.Resources {
				info.ResourceTypes = append(info.ResourceTypes, aws.ToString(r.ResourceType))
				info.ResourceNames = append(info.ResourceNames, aws.ToString(r.ResourceName))
			}
			events = append(events, info)
		}
		if len(events) == ctEventsCap {
			return ctEventsData{Events: events, Truncated: paginator.HasMorePages()}, nil
		}
	}

	return ctEventsData{Events: events}, nil
}

// ---------------------------------------------------------------------------
// backup — Backup Plans
// ---------------------------------------------------------------------------

type backupData struct {
	Plans []backupPlanInfo `json:"plans"`
	// RegionOptIn is the Region's ResourceTypeOptInPreference: the resource
	// types AWS Backup protects here, which coverage is judged against.
	RegionOptIn backupRegionOptIn `json:"region_opt_in"`
}

type backupRegionOptIn struct {
	readOutcome
	ResourceTypes map[string]bool `json:"resource_types,omitempty"`
}

type backupPlanInfo struct {
	BackupPlanId      string            `json:"backup_plan_id"`
	BackupPlanName    string            `json:"backup_plan_name"`
	CreationDate      string            `json:"creation_date,omitempty"`
	DeletionDate      string            `json:"deletion_date,omitempty"`
	LastExecutionDate string            `json:"last_execution_date,omitempty"`
	VersionId         string            `json:"version_id,omitempty"`
	SelectionsOutcome string            `json:"selections_outcome"`
	SelectionsCode    string            `json:"selections_error_code,omitempty"`
	IamRoleArns       []string          `json:"iam_role_arns,omitempty"`
	Selections        []backupSelection `json:"selections,omitempty"`
	RecentJobsOutcome string            `json:"recent_jobs_outcome"`
	RecentJobsCode    string            `json:"recent_jobs_error_code,omitempty"`
	RecentJobStates   []string          `json:"recent_job_states,omitempty"`
}

type backupSelection struct {
	SelectionId string `json:"selection_id"`
	readOutcome
	Resources    []string          `json:"resources,omitempty"`
	NotResources []string          `json:"not_resources,omitempty"`
	Conditions   []backupCondition `json:"conditions,omitempty"`
}

type backupCondition struct {
	Type  string `json:"type"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

func backupConditionsOf(sel *backuptypes.BackupSelection) []backupCondition {
	var out []backupCondition
	for _, t := range sel.ListOfTags {
		out = append(out, backupCondition{Type: string(t.ConditionType), Key: aws.ToString(t.ConditionKey), Value: aws.ToString(t.ConditionValue)})
	}
	if c := sel.Conditions; c != nil {
		for typ, params := range map[string][]backuptypes.ConditionParameter{
			"StringEquals": c.StringEquals, "StringLike": c.StringLike, "StringNotEquals": c.StringNotEquals, "StringNotLike": c.StringNotLike,
		} {
			for _, p := range params {
				out = append(out, backupCondition{Type: typ, Key: aws.ToString(p.ConditionKey), Value: aws.ToString(p.ConditionValue)})
			}
		}
	}
	slices.SortFunc(out, func(a, b backupCondition) int {
		return cmp.Or(cmp.Compare(a.Type, b.Type), cmp.Compare(a.Key, b.Key), cmp.Compare(a.Value, b.Value))
	})
	return out
}

func captureBackupSelection(ctx context.Context, client *backupsvc.Client, planID, selectionID string) backupSelection {
	got, read := readOne(func() (*backuptypes.BackupSelection, error) {
		out, err := client.GetBackupSelection(ctx, &backupsvc.GetBackupSelectionInput{BackupPlanId: aws.String(planID), SelectionId: aws.String(selectionID)})
		if err != nil {
			return nil, err
		}
		return out.BackupSelection, nil
	})
	sel := backupSelection{SelectionId: selectionID, readOutcome: read}
	if got != nil {
		sel.Resources = got.Resources
		sel.NotResources = got.NotResources
		sel.Conditions = backupConditionsOf(got)
	}
	return sel
}

func captureBackupRegionOptIn(ctx context.Context, client *backupsvc.Client) backupRegionOptIn {
	out, err := client.DescribeRegionSettings(ctx, &backupsvc.DescribeRegionSettingsInput{})
	opt := backupRegionOptIn{readOutcome: readOf(err)}
	if out != nil {
		opt.ResourceTypes = out.ResourceTypeOptInPreference
	}
	return opt
}

func captureBackup(ctx context.Context, cfg aws.Config) (any, error) {
	client := backupsvc.NewFromConfig(cfg)

	var plans []backupPlanInfo
	planPaginator := backupsvc.NewListBackupPlansPaginator(client, &backupsvc.ListBackupPlansInput{})
	for planPaginator.HasMorePages() {
		out, err := planPaginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range out.BackupPlansList {
			info := backupPlanInfo{
				BackupPlanId:      aws.ToString(p.BackupPlanId),
				BackupPlanName:    aws.ToString(p.BackupPlanName),
				CreationDate:      snapFormatTime(p.CreationDate),
				DeletionDate:      snapFormatTime(p.DeletionDate),
				LastExecutionDate: snapFormatTime(p.LastExecutionDate),
				VersionId:         aws.ToString(p.VersionId),
			}

			members, err := pageAll(ctx, &backupsvc.ListBackupSelectionsInput{BackupPlanId: p.BackupPlanId},
				func(ctx context.Context, in *backupsvc.ListBackupSelectionsInput) (*backupsvc.ListBackupSelectionsOutput, error) {
					return client.ListBackupSelections(ctx, in)
				},
				func(out *backupsvc.ListBackupSelectionsOutput) []backuptypes.BackupSelectionsListMember {
					return out.BackupSelectionsList
				}, allPages)
			info.SelectionsOutcome, info.SelectionsCode = outcomeOf(err)
			for _, s := range members {
				info.IamRoleArns = append(info.IamRoleArns, aws.ToString(s.IamRoleArn))
				if id := aws.ToString(s.SelectionId); id != "" {
					info.Selections = append(info.Selections, captureBackupSelection(ctx, client, info.BackupPlanId, id))
				}
			}

			plans = append(plans, info)
		}
	}

	since := time.Now().Add(-24 * time.Hour)
	jobs, jobsErr := pageAll(ctx, &backupsvc.ListBackupJobsInput{ByCreatedAfter: aws.Time(since)},
		func(ctx context.Context, in *backupsvc.ListBackupJobsInput) (*backupsvc.ListBackupJobsOutput, error) {
			return client.ListBackupJobs(ctx, in)
		},
		func(out *backupsvc.ListBackupJobsOutput) []backuptypes.BackupJob { return out.BackupJobs }, allPages)
	jobsByPlan := map[string][]string{}
	for _, j := range jobs {
		planID := ""
		if j.CreatedBy != nil {
			planID = aws.ToString(j.CreatedBy.BackupPlanId)
		}
		jobsByPlan[planID] = append(jobsByPlan[planID], string(j.State))
	}

	for i := range plans {
		plans[i].RecentJobsOutcome, plans[i].RecentJobsCode = outcomeOf(jobsErr)
		plans[i].RecentJobStates = jobsByPlan[plans[i].BackupPlanId]
	}

	return backupData{Plans: plans, RegionOptIn: captureBackupRegionOptIn(ctx, client)}, nil
}

// ---------------------------------------------------------------------------
// athena — Athena Workgroups
// ---------------------------------------------------------------------------

type athenaData struct {
	WorkGroups []athenaWorkGroup `json:"workgroups"`
}

type athenaWorkGroup struct {
	Name                          string `json:"name"`
	State                         string `json:"state"`
	CreationTime                  string `json:"creation_time,omitempty"`
	Description                   string `json:"description,omitempty"`
	GetOutcome                    string `json:"get_outcome"`
	GetErrorCode                  string `json:"get_error_code,omitempty"`
	EnforceWorkGroupConfiguration bool   `json:"enforce_workgroup_configuration,omitempty"`
	OutputLocation                string `json:"output_location,omitempty"`
	EncryptionKmsKey              string `json:"encryption_kms_key,omitempty"`
	CustomerContentKmsKey         string `json:"customer_content_kms_key,omitempty"`
	LoggingEnabled                bool   `json:"logging_enabled,omitempty"`
	LogGroup                      string `json:"log_group,omitempty"`
	ExecutionRole                 string `json:"execution_role,omitempty"`
	BytesScannedCutoffSet         bool   `json:"bytes_scanned_cutoff_set,omitempty"`
	BytesScannedCutoffPerQuery    int64  `json:"bytes_scanned_cutoff_per_query,omitempty"`
}

func captureAthena(ctx context.Context, cfg aws.Config) (any, error) {
	client := athena.NewFromConfig(cfg)

	var summaries []athenaSummaryEntry
	paginator := athena.NewListWorkGroupsPaginator(client, &athena.ListWorkGroupsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, wg := range out.WorkGroups {
			summaries = append(summaries, athenaSummaryEntry{
				Name:         aws.ToString(wg.Name),
				State:        string(wg.State),
				CreationTime: snapFormatTime(wg.CreationTime),
				Description:  aws.ToString(wg.Description),
			})
		}
	}

	listed := map[string]athenaSummaryEntry{}
	var names []string
	for _, s := range summaries {
		listed[s.Name] = s
		names = append(names, s.Name)
	}
	rows := describeListed(names, 1, func(batch []string) (map[string]athenatypes.WorkGroup, []string, error) {
		out, err := client.GetWorkGroup(ctx, &athena.GetWorkGroupInput{WorkGroup: aws.String(batch[0])})
		if err != nil {
			return nil, nil, err
		}
		return describedAs(batch[0], out.WorkGroup), nil, nil
	})

	var workgroups []athenaWorkGroup
	for _, row := range rows {
		s := listed[row.id]
		wg := athenaWorkGroup{
			Name:         row.id,
			State:        s.State,
			CreationTime: s.CreationTime,
			Description:  s.Description,
			GetOutcome:   row.read.Outcome,
			GetErrorCode: row.read.ErrorCode,
		}
		if row.item != nil && row.item.Configuration != nil {
			c := row.item.Configuration
			wg.EnforceWorkGroupConfiguration = aws.ToBool(c.EnforceWorkGroupConfiguration)
			wg.ExecutionRole = aws.ToString(c.ExecutionRole)
			if c.BytesScannedCutoffPerQuery != nil {
				wg.BytesScannedCutoffSet = true
				wg.BytesScannedCutoffPerQuery = *c.BytesScannedCutoffPerQuery
			}
			if c.ResultConfiguration != nil {
				wg.OutputLocation = aws.ToString(c.ResultConfiguration.OutputLocation)
				if c.ResultConfiguration.EncryptionConfiguration != nil {
					wg.EncryptionKmsKey = aws.ToString(c.ResultConfiguration.EncryptionConfiguration.KmsKey)
				}
			}
			if c.CustomerContentEncryptionConfiguration != nil {
				wg.CustomerContentKmsKey = aws.ToString(c.CustomerContentEncryptionConfiguration.KmsKey)
			}
			if c.MonitoringConfiguration != nil && c.MonitoringConfiguration.CloudWatchLoggingConfiguration != nil {
				cwl := c.MonitoringConfiguration.CloudWatchLoggingConfiguration
				wg.LoggingEnabled = aws.ToBool(cwl.Enabled)
				wg.LogGroup = aws.ToString(cwl.LogGroup)
			}
		}

		workgroups = append(workgroups, wg)
	}

	return athenaData{WorkGroups: workgroups}, nil
}

type athenaSummaryEntry struct {
	Name         string
	State        string
	CreationTime string
	Description  string
}

// ---------------------------------------------------------------------------
// glue — Glue Jobs
// ---------------------------------------------------------------------------

type glueData struct {
	Jobs []glueJobInfo `json:"jobs"`
}

type glueJobInfo struct {
	Name                  string   `json:"name"`
	Role                  string   `json:"role,omitempty"`
	ScriptLocation        string   `json:"script_location,omitempty"`
	SecurityConfiguration string   `json:"security_configuration,omitempty"`
	LogUri                string   `json:"log_uri,omitempty"`
	CreatedOn             string   `json:"created_on,omitempty"`
	LastModifiedOn        string   `json:"last_modified_on,omitempty"`
	DefaultArgumentKeys   []string `json:"default_argument_keys,omitempty"`
	Connections           []string `json:"connections,omitempty"`
	RunsOutcome           string   `json:"runs_outcome"`
	RunsErrorCode         string   `json:"runs_error_code,omitempty"`
	LatestRunState        string   `json:"latest_run_state,omitempty"`
	LatestRunErrorMessage string   `json:"latest_run_error_message,omitempty"`
	LatestRunStartedOn    string   `json:"latest_run_started_on,omitempty"`
}

// Default arguments are free-form and routinely carry credentials, so the
// record keeps which arguments exist, never their values.
func glueJobInfoOf(j gluetypes.Job) glueJobInfo {
	info := glueJobInfo{
		Name:                  aws.ToString(j.Name),
		Role:                  aws.ToString(j.Role),
		SecurityConfiguration: aws.ToString(j.SecurityConfiguration),
		LogUri:                aws.ToString(j.LogUri),
		CreatedOn:             snapFormatTime(j.CreatedOn),
		LastModifiedOn:        snapFormatTime(j.LastModifiedOn),
		DefaultArgumentKeys:   slices.Sorted(maps.Keys(j.DefaultArguments)),
	}
	if j.Command != nil {
		info.ScriptLocation = aws.ToString(j.Command.ScriptLocation)
	}
	if j.Connections != nil {
		info.Connections = j.Connections.Connections
	}
	return info
}

func captureGlue(ctx context.Context, cfg aws.Config) (any, error) {
	client := glue.NewFromConfig(cfg)

	var jobs []glueJobInfo
	paginator := glue.NewGetJobsPaginator(client, &glue.GetJobsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, j := range out.Jobs {
			info := glueJobInfoOf(j)
			runs, err := pageAll(ctx, &glue.GetJobRunsInput{
				JobName:    j.Name,
				MaxResults: aws.Int32(1),
			},
				func(ctx context.Context, in *glue.GetJobRunsInput) (*glue.GetJobRunsOutput, error) {
					return client.GetJobRuns(ctx, in)
				},
				func(out *glue.GetJobRunsOutput) []gluetypes.JobRun { return out.JobRuns }, onePage)
			info.RunsOutcome, info.RunsErrorCode = outcomeOf(err)
			if len(runs) > 0 {
				r := runs[0]
				info.LatestRunState = string(r.JobRunState)
				info.LatestRunErrorMessage = aws.ToString(r.ErrorMessage)
				info.LatestRunStartedOn = snapFormatTime(r.StartedOn)
			}

			jobs = append(jobs, info)
		}
	}

	return glueData{Jobs: jobs}, nil
}

// ---------------------------------------------------------------------------
// opensearch — OpenSearch Domains
// ---------------------------------------------------------------------------

type opensearchData struct {
	Domains []opensearchDomainInfo `json:"domains"`
}

type opensearchDomainInfo struct {
	DomainName                   string             `json:"domain_name"`
	EngineType                   string             `json:"engine_type,omitempty"`
	DescribeOutcome              string             `json:"describe_outcome"`
	DescribeErrorCode            string             `json:"describe_error_code,omitempty"`
	Arn                          string             `json:"arn,omitempty"`
	Deleted                      bool               `json:"deleted,omitempty"`
	Processing                   bool               `json:"processing,omitempty"`
	UpgradeProcessing            bool               `json:"upgrade_processing,omitempty"`
	DomainProcessingStatus       string             `json:"domain_processing_status,omitempty"`
	ServiceSoftwareUpdateAvail   bool               `json:"service_software_update_available,omitempty"`
	ServiceSoftwareAutoUpdate    string             `json:"service_software_automated_update_date,omitempty"`
	EncryptionAtRestEnabled      bool               `json:"encryption_at_rest_enabled,omitempty"`
	EncryptionAtRestKmsKeyId     string             `json:"encryption_at_rest_kms_key_id,omitempty"`
	VpcId                        string             `json:"vpc_id,omitempty"`
	SubnetIds                    []string           `json:"subnet_ids,omitempty"`
	SecurityGroupIds             []string           `json:"security_group_ids,omitempty"`
	CustomEndpointEnabled        bool               `json:"custom_endpoint_enabled,omitempty"`
	CustomEndpointCertificateArn string             `json:"custom_endpoint_certificate_arn,omitempty"`
	LogPublishingOptions         []opensearchLogOpt `json:"log_publishing_options,omitempty"`
}

type opensearchLogOpt struct {
	LogType                   string `json:"log_type"`
	Enabled                   bool   `json:"enabled"`
	CloudWatchLogsLogGroupArn string `json:"cloudwatch_logs_log_group_arn,omitempty"`
}

func captureOpenSearch(ctx context.Context, cfg aws.Config) (any, error) {
	client := opensearch.NewFromConfig(cfg)

	namesOut, err := client.ListDomainNames(ctx, &opensearch.ListDomainNamesInput{})
	if err != nil {
		return nil, err
	}

	var names []string
	engineByName := map[string]string{}
	for _, d := range namesOut.DomainNames {
		name := aws.ToString(d.DomainName)
		names = append(names, name)
		engineByName[name] = string(d.EngineType)
	}

	// DescribeDomains takes at most 5 domain names per call.
	rows := describeListed(names, 5, func(batch []string) (map[string]ostypes.DomainStatus, []string, error) {
		out, err := client.DescribeDomains(ctx, &opensearch.DescribeDomainsInput{DomainNames: batch})
		if err != nil {
			return nil, nil, err
		}
		found := map[string]ostypes.DomainStatus{}
		for _, ds := range out.DomainStatusList {
			found[aws.ToString(ds.DomainName)] = ds
		}
		return found, nil, nil
	})

	domains := make([]opensearchDomainInfo, 0, len(rows))
	for _, row := range rows {
		info := opensearchDomainInfo{
			DomainName:        row.id,
			EngineType:        engineByName[row.id],
			DescribeOutcome:   row.read.Outcome,
			DescribeErrorCode: row.read.ErrorCode,
		}
		if ds := row.item; ds != nil {
			info.Arn = aws.ToString(ds.ARN)
			info.Deleted = aws.ToBool(ds.Deleted)
			info.Processing = aws.ToBool(ds.Processing)
			info.UpgradeProcessing = aws.ToBool(ds.UpgradeProcessing)
			info.DomainProcessingStatus = string(ds.DomainProcessingStatus)
			if ds.ServiceSoftwareOptions != nil {
				info.ServiceSoftwareUpdateAvail = aws.ToBool(ds.ServiceSoftwareOptions.UpdateAvailable)
				info.ServiceSoftwareAutoUpdate = snapFormatTime(ds.ServiceSoftwareOptions.AutomatedUpdateDate)
			}
			if ds.EncryptionAtRestOptions != nil {
				info.EncryptionAtRestEnabled = aws.ToBool(ds.EncryptionAtRestOptions.Enabled)
				info.EncryptionAtRestKmsKeyId = aws.ToString(ds.EncryptionAtRestOptions.KmsKeyId)
			}
			if ds.VPCOptions != nil {
				info.VpcId = aws.ToString(ds.VPCOptions.VPCId)
				info.SubnetIds = ds.VPCOptions.SubnetIds
				info.SecurityGroupIds = ds.VPCOptions.SecurityGroupIds
			}
			if ds.DomainEndpointOptions != nil {
				info.CustomEndpointEnabled = aws.ToBool(ds.DomainEndpointOptions.CustomEndpointEnabled)
				info.CustomEndpointCertificateArn = aws.ToString(ds.DomainEndpointOptions.CustomEndpointCertificateArn)
			}
			for logType, opt := range ds.LogPublishingOptions {
				info.LogPublishingOptions = append(info.LogPublishingOptions, opensearchLogOpt{
					LogType:                   string(logType),
					Enabled:                   aws.ToBool(opt.Enabled),
					CloudWatchLogsLogGroupArn: aws.ToString(opt.CloudWatchLogsLogGroupArn),
				})
			}
		}
		domains = append(domains, info)
	}

	return opensearchData{Domains: domains}, nil
}

// ---------------------------------------------------------------------------
// cf — CloudFront Distributions
// ---------------------------------------------------------------------------

type cfData struct {
	Distributions []cfDistribution `json:"distributions"`
}

type cfDistribution struct {
	Id                           string     `json:"id"`
	ARN                          string     `json:"arn,omitempty"`
	DomainName                   string     `json:"domain_name,omitempty"`
	Status                       string     `json:"status"`
	Enabled                      bool       `json:"enabled"`
	WebACLId                     string     `json:"web_acl_id,omitempty"`
	ViewerCertARN                string     `json:"viewer_cert_acm_arn,omitempty"`
	CloudFrontDefaultCertificate bool       `json:"cloudfront_default_certificate"`
	MinimumProtocolVersion       string     `json:"minimum_protocol_version,omitempty"`
	Origins                      []cfOrigin `json:"origins,omitempty"`
	LambdaFunctionARNs           []string   `json:"lambda_function_arns,omitempty"`
	ConfigOutcome                string     `json:"config_outcome"`
	ConfigErrorCode              string     `json:"config_error_code,omitempty"`
	LoggingEnabled               bool       `json:"logging_enabled,omitempty"`
	LoggingBucket                string     `json:"logging_bucket,omitempty"`
}

type cfOrigin struct {
	DomainName string `json:"domain_name"`
}

func captureCF(ctx context.Context, cfg aws.Config) (any, error) {
	client := cloudfront.NewFromConfig(cfg)

	var dists []cfDistribution
	paginator := cloudfront.NewListDistributionsPaginator(client, &cloudfront.ListDistributionsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		if out.DistributionList == nil {
			break
		}
		for _, d := range out.DistributionList.Items {
			info := cfDistribution{
				Id:         aws.ToString(d.Id),
				ARN:        aws.ToString(d.ARN),
				DomainName: aws.ToString(d.DomainName),
				Status:     aws.ToString(d.Status),
				Enabled:    aws.ToBool(d.Enabled),
				WebACLId:   aws.ToString(d.WebACLId),
			}
			if d.ViewerCertificate != nil {
				info.ViewerCertARN = aws.ToString(d.ViewerCertificate.ACMCertificateArn)
				info.CloudFrontDefaultCertificate = aws.ToBool(d.ViewerCertificate.CloudFrontDefaultCertificate)
				info.MinimumProtocolVersion = string(d.ViewerCertificate.MinimumProtocolVersion)
			}
			if d.Origins != nil {
				for _, o := range d.Origins.Items {
					info.Origins = append(info.Origins, cfOrigin{DomainName: aws.ToString(o.DomainName)})
				}
			}
			if d.DefaultCacheBehavior != nil && d.DefaultCacheBehavior.LambdaFunctionAssociations != nil {
				for _, lfa := range d.DefaultCacheBehavior.LambdaFunctionAssociations.Items {
					info.LambdaFunctionARNs = append(info.LambdaFunctionARNs, aws.ToString(lfa.LambdaFunctionARN))
				}
			}
			if d.CacheBehaviors != nil {
				for _, cb := range d.CacheBehaviors.Items {
					if cb.LambdaFunctionAssociations != nil {
						for _, lfa := range cb.LambdaFunctionAssociations.Items {
							info.LambdaFunctionARNs = append(info.LambdaFunctionARNs, aws.ToString(lfa.LambdaFunctionARN))
						}
					}
				}
			}

			distCfg, read := readOne(func() (*cftypes.DistributionConfig, error) {
				out, err := client.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: d.Id})
				if err != nil {
					return nil, err
				}
				return out.DistributionConfig, nil
			})
			info.ConfigOutcome, info.ConfigErrorCode = read.Outcome, read.ErrorCode
			if distCfg != nil && distCfg.Logging != nil {
				info.LoggingEnabled = aws.ToBool(distCfg.Logging.Enabled)
				info.LoggingBucket = aws.ToString(distCfg.Logging.Bucket)
			}

			dists = append(dists, info)
		}
	}

	return cfData{Distributions: dists}, nil
}

// ---------------------------------------------------------------------------
// r53 — Route 53 Hosted Zones
// ---------------------------------------------------------------------------

type r53Data struct {
	HostedZones []r53Zone `json:"hosted_zones"`
}

type r53Zone struct {
	Id                       string        `json:"id"`
	Name                     string        `json:"name"`
	PrivateZone              bool          `json:"private_zone"`
	ResourceRecordSetCount   int64         `json:"resource_record_set_count"`
	RecordsOutcome           string        `json:"records_outcome"`
	RecordsErrorCode         string        `json:"records_error_code,omitempty"`
	Records                  []r53Record   `json:"records,omitempty"`
	VPCAssociationsOutcome   string        `json:"vpc_associations_outcome,omitempty"`
	VPCAssociationsErrorCode string        `json:"vpc_associations_error_code,omitempty"`
	VPCs                     []r53VPCAssoc `json:"vpcs,omitempty"`
}

type r53VPCAssoc struct {
	VPCId     string `json:"vpc_id"`
	VPCRegion string `json:"vpc_region"`
}

type r53Record struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	SetIdentifier string   `json:"set_identifier,omitempty"`
	AliasDNSName  string   `json:"alias_dns_name,omitempty"`
	Values        []string `json:"values,omitempty"`
}

func captureR53Records(ctx context.Context, client *route53.Client, zoneID *string) ([]r53Record, error) {
	sets, err := pageAll(ctx, &route53.ListResourceRecordSetsInput{HostedZoneId: zoneID},
		func(ctx context.Context, in *route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error) {
			return client.ListResourceRecordSets(ctx, in)
		},
		func(out *route53.ListResourceRecordSetsOutput) []r53types.ResourceRecordSet {
			return out.ResourceRecordSets
		}, allPages)
	records := make([]r53Record, 0, len(sets))
	for _, s := range sets {
		r := r53Record{Name: aws.ToString(s.Name), Type: string(s.Type), SetIdentifier: aws.ToString(s.SetIdentifier)}
		if s.AliasTarget != nil {
			r.AliasDNSName = aws.ToString(s.AliasTarget.DNSName)
		}
		for _, v := range s.ResourceRecords {
			r.Values = append(r.Values, aws.ToString(v.Value))
		}
		records = append(records, r)
	}
	return records, err
}

func captureR53(ctx context.Context, cfg aws.Config) (any, error) {
	client := route53.NewFromConfig(cfg)

	var zones []r53Zone
	paginator := route53.NewListHostedZonesPaginator(client, &route53.ListHostedZonesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, z := range out.HostedZones {
			zone := r53Zone{
				Id:                     aws.ToString(z.Id),
				Name:                   aws.ToString(z.Name),
				ResourceRecordSetCount: aws.ToInt64(z.ResourceRecordSetCount),
			}
			if z.Config != nil {
				zone.PrivateZone = z.Config.PrivateZone
			}

			var err error
			zone.Records, err = captureR53Records(ctx, client, z.Id)
			zone.RecordsOutcome, zone.RecordsErrorCode = outcomeOf(err)

			if zone.PrivateZone {
				vpcOut, err := client.GetHostedZone(ctx, &route53.GetHostedZoneInput{Id: z.Id})
				zone.VPCAssociationsOutcome, zone.VPCAssociationsErrorCode = outcomeOf(err)
				if vpcOut != nil {
					for _, v := range vpcOut.VPCs {
						zone.VPCs = append(zone.VPCs, r53VPCAssoc{
							VPCId:     aws.ToString(v.VPCId),
							VPCRegion: string(v.VPCRegion),
						})
					}
				}
			}

			zones = append(zones, zone)
		}
	}

	return r53Data{HostedZones: zones}, nil
}
