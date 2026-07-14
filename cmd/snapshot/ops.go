package main

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	backupsvc "github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	smithy "github.com/aws/smithy-go"
)

// ---------------------------------------------------------------------------
// cb — CodeBuild Projects
// ---------------------------------------------------------------------------

type cbData struct {
	Projects []cbProject `json:"projects"`
}

type cbProject struct {
	Name             string       `json:"name"`
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
	Value string `json:"value"`
	Type  string `json:"type"`
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

	var projects []cbProject
	for start := 0; start < len(names); start += 100 {
		end := min(start+100, len(names))
		batch, err := client.BatchGetProjects(ctx, &codebuild.BatchGetProjectsInput{Names: names[start:end]})
		if err != nil {
			return nil, err
		}
		for _, p := range batch.Projects {
			proj := cbProject{Name: aws.ToString(p.Name), ServiceRole: aws.ToString(p.ServiceRole)}
			if p.EncryptionKey != nil {
				proj.EncryptionKey = *p.EncryptionKey
			}
			if p.Source != nil {
				proj.SourceType = string(p.Source.Type)
				proj.SourceLocation = aws.ToString(p.Source.Location)
			}
			if p.Artifacts != nil {
				proj.ArtifactsType = string(p.Artifacts.Type)
				proj.ArtifactsLoc = aws.ToString(p.Artifacts.Location)
			}
			if p.Environment != nil {
				proj.EnvImage = aws.ToString(p.Environment.Image)
				for _, ev := range p.Environment.EnvironmentVariables {
					proj.EnvVars = append(proj.EnvVars, cbEnvVar{
						Name:  aws.ToString(ev.Name),
						Value: aws.ToString(ev.Value),
						Type:  string(ev.Type),
					})
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
			proj.LatestBuild = captureLatestBuild(ctx, client, proj.Name)
			projects = append(projects, proj)
		}
	}

	return cbData{Projects: projects}, nil
}

func captureLatestBuild(ctx context.Context, client *codebuild.Client, projectName string) *cbBuildInfo {
	listOut, err := client.ListBuildsForProject(ctx, &codebuild.ListBuildsForProjectInput{
		ProjectName: aws.String(projectName),
	})
	if err != nil {
		return &cbBuildInfo{Outcome: "error", ErrorCode: apiErrorCode(err)}
	}
	if len(listOut.Ids) == 0 {
		return &cbBuildInfo{Outcome: "none"}
	}
	getOut, err := client.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{Ids: listOut.Ids[:1]})
	if err != nil {
		return &cbBuildInfo{Outcome: "error", ErrorCode: apiErrorCode(err)}
	}
	if len(getOut.Builds) == 0 {
		return &cbBuildInfo{Outcome: "none"}
	}
	b := getOut.Builds[0]
	return &cbBuildInfo{
		Outcome:      "found",
		BuildStatus:  string(b.BuildStatus),
		CurrentPhase: aws.ToString(b.CurrentPhase),
		EndTime:      snapFormatTime(b.EndTime),
	}
}

func apiErrorCode(err error) string {
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		return apiErr.ErrorCode()
	}
	return err.Error()
}

// ---------------------------------------------------------------------------
// pipeline — CodePipelines
// ---------------------------------------------------------------------------

type pipelineData struct {
	Pipelines []pipelineInfo `json:"pipelines"`
}

type pipelineInfo struct {
	Name        string         `json:"name"`
	RoleArn     string         `json:"role_arn,omitempty"`
	State       *pipelineState `json:"state,omitempty"`
	Declaration *pipelineDecl  `json:"declaration,omitempty"`
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

type pipelineDecl struct {
	Outcome        string             `json:"outcome"`
	Code           string             `json:"error_code,omitempty"`
	ArtifactStores []pipelineArtStore `json:"artifact_stores,omitempty"`
	Actions        []pipelineAction   `json:"actions,omitempty"`
	ActionRoleArns []string           `json:"action_role_arns,omitempty"`
}

type pipelineArtStore struct {
	Location          string `json:"location,omitempty"`
	Region            string `json:"region,omitempty"`
	EncryptionKeyID   string `json:"encryption_key_id,omitempty"`
	EncryptionKeyType string `json:"encryption_key_type,omitempty"`
}

type pipelineAction struct {
	StageName     string            `json:"stage_name"`
	ActionName    string            `json:"action_name"`
	Category      string            `json:"category"`
	Owner         string            `json:"owner"`
	Provider      string            `json:"provider"`
	Configuration map[string]string `json:"configuration,omitempty"`
	RoleArn       string            `json:"role_arn,omitempty"`
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
		if err != nil {
			info.State = &pipelineState{Outcome: "error", Code: apiErrorCode(err)}
		} else {
			st := &pipelineState{Outcome: "ok"}
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
				st.Stages = append(st.Stages, stg)
			}
			info.State = st
		}

		declOut, err := client.GetPipeline(ctx, &codepipeline.GetPipelineInput{Name: aws.String(name)})
		if err != nil {
			info.Declaration = &pipelineDecl{Outcome: "error", Code: apiErrorCode(err)}
		} else if declOut.Pipeline != nil {
			decl := &pipelineDecl{Outcome: "ok"}
			info.RoleArn = aws.ToString(declOut.Pipeline.RoleArn)

			if declOut.Pipeline.ArtifactStore != nil {
				decl.ArtifactStores = append(decl.ArtifactStores, toArtifactStore(declOut.Pipeline.ArtifactStore))
			}
			for region, as := range declOut.Pipeline.ArtifactStores {
				store := toArtifactStore(&as)
				store.Region = region
				decl.ArtifactStores = append(decl.ArtifactStores, store)
			}

			for _, stage := range declOut.Pipeline.Stages {
				for _, action := range stage.Actions {
					pa := pipelineAction{
						StageName:  aws.ToString(stage.Name),
						ActionName: aws.ToString(action.Name),
					}
					if action.ActionTypeId != nil {
						pa.Category = string(action.ActionTypeId.Category)
						pa.Owner = string(action.ActionTypeId.Owner)
						pa.Provider = aws.ToString(action.ActionTypeId.Provider)
					}
					if len(action.Configuration) > 0 {
						pa.Configuration = action.Configuration
					}
					if action.RoleArn != nil {
						pa.RoleArn = *action.RoleArn
						decl.ActionRoleArns = append(decl.ActionRoleArns, *action.RoleArn)
					}
					decl.Actions = append(decl.Actions, pa)
				}
			}
			info.Declaration = decl
		}

		pipelines = append(pipelines, info)
	}

	return pipelineData{Pipelines: pipelines}, nil
}

func toArtifactStore(as *cptypes.ArtifactStore) pipelineArtStore {
	store := pipelineArtStore{Location: aws.ToString(as.Location)}
	if as.EncryptionKey != nil {
		store.EncryptionKeyID = aws.ToString(as.EncryptionKey.Id)
		store.EncryptionKeyType = string(as.EncryptionKey.Type)
	}
	return store
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
	HasPackages         bool   `json:"has_packages"`
	DomainEncryptionKey string `json:"domain_encryption_key,omitempty"`
}

func captureCodeArtifact(ctx context.Context, cfg aws.Config) (any, error) {
	client := codeartifact.NewFromConfig(cfg)

	var repos []codeArtifactRepo
	domainKeyCache := map[string]string{}
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

			pkgOut, err := client.ListPackages(ctx, &codeartifact.ListPackagesInput{
				Domain:      r.DomainName,
				DomainOwner: r.DomainOwner,
				Repository:  r.Name,
				MaxResults:  aws.Int32(1),
			})
			if err != nil {
				repo.PackageCountOutcome = "error"
				repo.PackageCountCode = apiErrorCode(err)
			} else {
				repo.PackageCountOutcome = "ok"
				repo.HasPackages = len(pkgOut.Packages) > 0
			}

			cacheKey := aws.ToString(r.DomainName) + "|" + aws.ToString(r.DomainOwner)
			if key, ok := domainKeyCache[cacheKey]; ok {
				repo.DomainEncryptionKey = key
			} else {
				domOut, err := client.DescribeDomain(ctx, &codeartifact.DescribeDomainInput{
					Domain:      r.DomainName,
					DomainOwner: r.DomainOwner,
				})
				if err == nil && domOut.Domain != nil {
					repo.DomainEncryptionKey = aws.ToString(domOut.Domain.EncryptionKey)
				}
				domainKeyCache[cacheKey] = repo.DomainEncryptionKey
			}

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

			evOut, err := client.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
				StackName: s.StackName,
			})
			if err != nil {
				stack.RecentEventsOutcome = "error"
				stack.RecentEventsCode = apiErrorCode(err)
			} else {
				stack.RecentEventsOutcome = "ok"
				for _, ev := range evOut.StackEvents {
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

			streamOut, err := client.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
				LogGroupName: g.LogGroupName,
				OrderBy:      "LastEventTime",
				Descending:   aws.Bool(true),
				Limit:        aws.Int32(1),
			})
			if err != nil {
				info.LastEventOutcome = "error"
				info.LastEventErrorCode = apiErrorCode(err)
			} else {
				info.LastEventOutcome = "ok"
				if len(streamOut.LogStreams) > 0 {
					info.LastEventTimestamp = opsFormatEpochMillis(streamOut.LogStreams[0].LastEventTimestamp)
				}
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
	IsLogging                 bool   `json:"is_logging"`
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
		if err != nil {
			info.StatusOutcome = "error"
			info.StatusErrorCode = apiErrorCode(err)
		} else {
			info.StatusOutcome = "ok"
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

func captureCTEvents(ctx context.Context, cfg aws.Config) (any, error) {
	client := cloudtrail.NewFromConfig(cfg)

	var events []ctEventInfo
	truncated := false
	paginator := cloudtrail.NewLookupEventsPaginator(client, &cloudtrail.LookupEventsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range out.Events {
			info := ctEventInfo{
				EventId:         aws.ToString(e.EventId),
				EventName:       aws.ToString(e.EventName),
				EventTime:       snapFormatTime(e.EventTime),
				Username:        aws.ToString(e.Username),
				EventSource:     aws.ToString(e.EventSource),
				ReadOnly:        aws.ToString(e.ReadOnly),
				AccessKeyId:     aws.ToString(e.AccessKeyId),
				CloudTrailEvent: aws.ToString(e.CloudTrailEvent),
			}
			for _, r := range e.Resources {
				info.ResourceTypes = append(info.ResourceTypes, aws.ToString(r.ResourceType))
				info.ResourceNames = append(info.ResourceNames, aws.ToString(r.ResourceName))
			}
			events = append(events, info)
			if len(events) >= ctEventsCap {
				truncated = true
				break
			}
		}
		if truncated {
			break
		}
	}

	return ctEventsData{Events: events, Truncated: truncated}, nil
}

// ---------------------------------------------------------------------------
// backup — Backup Plans
// ---------------------------------------------------------------------------

type backupData struct {
	Plans []backupPlanInfo `json:"plans"`
}

type backupPlanInfo struct {
	BackupPlanId      string   `json:"backup_plan_id"`
	BackupPlanName    string   `json:"backup_plan_name"`
	CreationDate      string   `json:"creation_date,omitempty"`
	DeletionDate      string   `json:"deletion_date,omitempty"`
	LastExecutionDate string   `json:"last_execution_date,omitempty"`
	VersionId         string   `json:"version_id,omitempty"`
	SelectionsOutcome string   `json:"selections_outcome"`
	SelectionsCode    string   `json:"selections_error_code,omitempty"`
	IamRoleArns       []string `json:"iam_role_arns,omitempty"`
	RecentJobsOutcome string   `json:"recent_jobs_outcome"`
	RecentJobsCode    string   `json:"recent_jobs_error_code,omitempty"`
	RecentJobStates   []string `json:"recent_job_states,omitempty"`
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

			selOut, err := client.ListBackupSelections(ctx, &backupsvc.ListBackupSelectionsInput{
				BackupPlanId: p.BackupPlanId,
			})
			if err != nil {
				info.SelectionsOutcome = "error"
				info.SelectionsCode = apiErrorCode(err)
			} else {
				info.SelectionsOutcome = "ok"
				for _, s := range selOut.BackupSelectionsList {
					info.IamRoleArns = append(info.IamRoleArns, aws.ToString(s.IamRoleArn))
				}
			}

			plans = append(plans, info)
		}
	}

	since := time.Now().Add(-24 * time.Hour)
	jobsByPlan := map[string][]string{}
	jobsOutcome := "ok"
	jobsCode := ""
	jobPaginator := backupsvc.NewListBackupJobsPaginator(client, &backupsvc.ListBackupJobsInput{
		ByCreatedAfter: aws.Time(since),
	})
	for jobPaginator.HasMorePages() {
		out, err := jobPaginator.NextPage(ctx)
		if err != nil {
			jobsOutcome = "error"
			jobsCode = apiErrorCode(err)
			break
		}
		for _, j := range out.BackupJobs {
			planID := ""
			if j.CreatedBy != nil {
				planID = aws.ToString(j.CreatedBy.BackupPlanId)
			}
			jobsByPlan[planID] = append(jobsByPlan[planID], string(j.State))
		}
	}

	for i := range plans {
		plans[i].RecentJobsOutcome = jobsOutcome
		plans[i].RecentJobsCode = jobsCode
		plans[i].RecentJobStates = jobsByPlan[plans[i].BackupPlanId]
	}

	return backupData{Plans: plans}, nil
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
	EnforceWorkGroupConfiguration bool   `json:"enforce_workgroup_configuration"`
	OutputLocation                string `json:"output_location,omitempty"`
	EncryptionKmsKey              string `json:"encryption_kms_key,omitempty"`
	CustomerContentKmsKey         string `json:"customer_content_kms_key,omitempty"`
	LoggingEnabled                bool   `json:"logging_enabled"`
	LogGroup                      string `json:"log_group,omitempty"`
	ExecutionRole                 string `json:"execution_role,omitempty"`
	BytesScannedCutoffSet         bool   `json:"bytes_scanned_cutoff_set"`
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

	var workgroups []athenaWorkGroup
	for _, s := range summaries {
		wg := athenaWorkGroup{
			Name:         s.Name,
			State:        s.State,
			CreationTime: s.CreationTime,
			Description:  s.Description,
		}

		getOut, err := client.GetWorkGroup(ctx, &athena.GetWorkGroupInput{WorkGroup: aws.String(s.Name)})
		switch {
		case err != nil:
			wg.GetOutcome = "error"
			wg.GetErrorCode = apiErrorCode(err)
		case getOut.WorkGroup != nil && getOut.WorkGroup.Configuration != nil:
			wg.GetOutcome = "ok"
			c := getOut.WorkGroup.Configuration
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
		default:
			wg.GetOutcome = "ok"
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
	Name                  string            `json:"name"`
	Role                  string            `json:"role,omitempty"`
	ScriptLocation        string            `json:"script_location,omitempty"`
	SecurityConfiguration string            `json:"security_configuration,omitempty"`
	LogUri                string            `json:"log_uri,omitempty"`
	CreatedOn             string            `json:"created_on,omitempty"`
	LastModifiedOn        string            `json:"last_modified_on,omitempty"`
	DefaultArguments      map[string]string `json:"default_arguments,omitempty"`
	Connections           []string          `json:"connections,omitempty"`
	RunsOutcome           string            `json:"runs_outcome"`
	RunsErrorCode         string            `json:"runs_error_code,omitempty"`
	LatestRunState        string            `json:"latest_run_state,omitempty"`
	LatestRunErrorMessage string            `json:"latest_run_error_message,omitempty"`
	LatestRunStartedOn    string            `json:"latest_run_started_on,omitempty"`
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
			info := glueJobInfo{
				Name:                  aws.ToString(j.Name),
				Role:                  aws.ToString(j.Role),
				SecurityConfiguration: aws.ToString(j.SecurityConfiguration),
				LogUri:                aws.ToString(j.LogUri),
				CreatedOn:             snapFormatTime(j.CreatedOn),
				LastModifiedOn:        snapFormatTime(j.LastModifiedOn),
				DefaultArguments:      j.DefaultArguments,
			}
			if j.Command != nil {
				info.ScriptLocation = aws.ToString(j.Command.ScriptLocation)
			}
			if j.Connections != nil {
				info.Connections = j.Connections.Connections
			}

			runsOut, err := client.GetJobRuns(ctx, &glue.GetJobRunsInput{
				JobName:    j.Name,
				MaxResults: aws.Int32(1),
			})
			if err != nil {
				info.RunsOutcome = "error"
				info.RunsErrorCode = apiErrorCode(err)
			} else {
				info.RunsOutcome = "ok"
				if len(runsOut.JobRuns) > 0 {
					r := runsOut.JobRuns[0]
					info.LatestRunState = string(r.JobRunState)
					info.LatestRunErrorMessage = aws.ToString(r.ErrorMessage)
					info.LatestRunStartedOn = snapFormatTime(r.StartedOn)
				}
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
	Deleted                      bool               `json:"deleted"`
	Processing                   bool               `json:"processing"`
	UpgradeProcessing            bool               `json:"upgrade_processing"`
	DomainProcessingStatus       string             `json:"domain_processing_status,omitempty"`
	ServiceSoftwareUpdateAvail   bool               `json:"service_software_update_available"`
	ServiceSoftwareAutoUpdate    string             `json:"service_software_automated_update_date,omitempty"`
	EncryptionAtRestEnabled      bool               `json:"encryption_at_rest_enabled"`
	EncryptionAtRestKmsKeyId     string             `json:"encryption_at_rest_kms_key_id,omitempty"`
	VpcId                        string             `json:"vpc_id,omitempty"`
	SubnetIds                    []string           `json:"subnet_ids,omitempty"`
	SecurityGroupIds             []string           `json:"security_group_ids,omitempty"`
	CustomEndpointEnabled        bool               `json:"custom_endpoint_enabled"`
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

	var domains []opensearchDomainInfo
	for start := 0; start < len(names); start += 5 {
		end := min(start+5, len(names))
		batch := names[start:end]

		descOut, err := client.DescribeDomains(ctx, &opensearch.DescribeDomainsInput{DomainNames: batch})
		if err != nil {
			for _, n := range batch {
				domains = append(domains, opensearchDomainInfo{
					DomainName:        n,
					EngineType:        engineByName[n],
					DescribeOutcome:   "error",
					DescribeErrorCode: apiErrorCode(err),
				})
			}
			continue
		}

		found := map[string]bool{}
		for _, ds := range descOut.DomainStatusList {
			name := aws.ToString(ds.DomainName)
			found[name] = true
			info := opensearchDomainInfo{
				DomainName:             name,
				EngineType:             engineByName[name],
				DescribeOutcome:        "ok",
				Arn:                    aws.ToString(ds.ARN),
				Deleted:                aws.ToBool(ds.Deleted),
				Processing:             aws.ToBool(ds.Processing),
				UpgradeProcessing:      aws.ToBool(ds.UpgradeProcessing),
				DomainProcessingStatus: string(ds.DomainProcessingStatus),
			}
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
			domains = append(domains, info)
		}
		for _, n := range batch {
			if !found[n] {
				domains = append(domains, opensearchDomainInfo{
					DomainName:      n,
					EngineType:      engineByName[n],
					DescribeOutcome: "not_found",
				})
			}
		}
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
	LoggingEnabled               bool       `json:"logging_enabled"`
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

			cfgOut, err := client.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: d.Id})
			if err != nil {
				info.ConfigOutcome = "error"
				info.ConfigErrorCode = apiErrorCode(err)
			} else if cfgOut.DistributionConfig != nil {
				info.ConfigOutcome = "ok"
				if cfgOut.DistributionConfig.Logging != nil {
					info.LoggingEnabled = aws.ToBool(cfgOut.DistributionConfig.Logging.Enabled)
					info.LoggingBucket = aws.ToString(cfgOut.DistributionConfig.Logging.Bucket)
				}
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
	DNSSECOutcome            string        `json:"dnssec_outcome"`
	DNSSECErrorCode          string        `json:"dnssec_error_code,omitempty"`
	ServeSignature           string        `json:"serve_signature,omitempty"`
	KeySigningKeyStatuses    []string      `json:"key_signing_key_statuses,omitempty"`
	VPCAssociationsOutcome   string        `json:"vpc_associations_outcome"`
	VPCAssociationsErrorCode string        `json:"vpc_associations_error_code,omitempty"`
	VPCs                     []r53VPCAssoc `json:"vpcs,omitempty"`
}

type r53VPCAssoc struct {
	VPCId     string `json:"vpc_id"`
	VPCRegion string `json:"vpc_region"`
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

			if !zone.PrivateZone {
				dnssecOut, err := client.GetDNSSEC(ctx, &route53.GetDNSSECInput{HostedZoneId: z.Id})
				if err != nil {
					zone.DNSSECOutcome = "error"
					zone.DNSSECErrorCode = apiErrorCode(err)
				} else {
					zone.DNSSECOutcome = "ok"
					if dnssecOut.Status != nil {
						zone.ServeSignature = aws.ToString(dnssecOut.Status.ServeSignature)
					}
					for _, ksk := range dnssecOut.KeySigningKeys {
						zone.KeySigningKeyStatuses = append(zone.KeySigningKeyStatuses, aws.ToString(ksk.Status))
					}
				}
			} else {
				zone.DNSSECOutcome = "skipped_private_zone"

				vpcOut, err := client.GetHostedZone(ctx, &route53.GetHostedZoneInput{Id: z.Id})
				if err != nil {
					zone.VPCAssociationsOutcome = "error"
					zone.VPCAssociationsErrorCode = apiErrorCode(err)
				} else {
					zone.VPCAssociationsOutcome = "ok"
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
