package demo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	smithycbor "github.com/aws/smithy-go/encoding/cbor"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// registerChildHandlers registers all child-view SDK operation handlers.
func registerChildHandlers(t *Transport) {
	registerCWLogsChildHandlers(t)
	registerCWAlarmChildHandlers(t)
	registerCFNChildHandlers(t)
	registerELBChildHandlers(t)
	registerIAMChildHandlers(t)
	registerECRChildHandlers(t)
	registerCodeBuildChildHandlers(t)
	registerCodePipelineChildHandlers(t)
	registerASGChildHandlers(t)
	registerRDSChildHandlers(t)
	registerSNSChildHandlers(t)
	registerSFNChildHandlers(t)
	registerEBChildHandlers(t)
	registerECSChildHandlers(t)
}

// ---------------------------------------------------------------------------
// CloudWatch Logs — child view operations (awsjson11, service "logs")
// ---------------------------------------------------------------------------

func registerCWLogsChildHandlers(t *Transport) {
	// DescribeLogStreams — used by log_streams child view
	t.Handle("logs", "DescribeLogStreams", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["log_streams"](map[string]string{"log_group_name": "demo-group"})
		streams := ExtractSDK[cwlogstypes.LogStream](resources)

		out := &cloudwatchlogs.DescribeLogStreamsOutput{
			LogStreams: streams,
		}
		return JSONResponseCamelCase(out)
	})

	// GetLogEvents — used by log_events child view
	t.Handle("logs", "GetLogEvents", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["log_events"](map[string]string{
			"log_group_name":  "demo-group",
			"log_stream_name": "demo-stream",
		})
		events := ExtractSDK[cwlogstypes.OutputLogEvent](resources)

		out := &cloudwatchlogs.GetLogEventsOutput{
			Events: events,
		}
		return JSONResponseCamelCase(out)
	})

	// FilterLogEvents — used by lambda_invocations, ecs_svc_logs, lambda_invocation_logs child views
	t.Handle("logs", "FilterLogEvents", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["lambda_invocations"](map[string]string{"function_name": "demo-fn"})
		events := ExtractSDK[cwlogstypes.FilteredLogEvent](resources)

		out := &cloudwatchlogs.FilterLogEventsOutput{
			Events: events,
		}
		return JSONResponseCamelCase(out)
	})
}

// ---------------------------------------------------------------------------
// CloudWatch — child view operations (awsquery XML, service "monitoring")
// ---------------------------------------------------------------------------

func registerCWAlarmChildHandlers(t *Transport) {
	t.Handle("monitoring", "DescribeAlarmHistory", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["alarm_history"](map[string]string{"alarm_name": "demo-alarm"})
		items := ExtractSDK[cwtypes.AlarmHistoryItem](resources)

		historyItems := make(smithycbor.List, 0, len(items))
		for _, item := range items {
			m := smithycbor.Map{
				"HistoryItemType": smithycbor.String(string(item.HistoryItemType)),
				"HistorySummary":  smithycbor.String(aws.ToString(item.HistorySummary)),
				"AlarmName":       smithycbor.String(aws.ToString(item.AlarmName)),
			}
			if item.Timestamp != nil {
				// CBOR timestamps must be encoded as Tag{ID: 1} (epoch-seconds tag)
				m["Timestamp"] = &smithycbor.Tag{ID: 1, Value: smithycbor.Float64(float64(item.Timestamp.Unix()))}
			}
			historyItems = append(historyItems, m)
		}

		response := smithycbor.Map{
			"AlarmHistoryItems": historyItems,
		}
		return CBORResponse(response), nil
	})
}

// ---------------------------------------------------------------------------
// CloudFormation — child view operations (awsquery XML, service "cloudformation")
// ---------------------------------------------------------------------------

func registerCFNChildHandlers(t *Transport) {
	// DescribeStackEvents — used by cfn_events child view
	t.Handle("cloudformation", "DescribeStackEvents", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["cfn_events"](map[string]string{"stack_name": "demo-stack"})
		events := ExtractSDK[cfntypes.StackEvent](resources)

		var sb strings.Builder
		sb.WriteString(`<StackEvents>`)
		for i, event := range events {
			logicalID := aws.ToString(event.LogicalResourceId)
			resourceType := aws.ToString(event.ResourceType)
			status := string(event.ResourceStatus)
			reason := aws.ToString(event.ResourceStatusReason)
			timestamp := ""
			if event.Timestamp != nil {
				timestamp = event.Timestamp.UTC().Format("2006-01-02T15:04:05Z")
			}
			eventID := aws.ToString(event.EventId)
			if eventID == "" {
				eventID = fmt.Sprintf("demo-event-%d", i+1)
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<EventId>%s</EventId>`, xmlEscape(eventID))
			if timestamp != "" {
				fmt.Fprintf(&sb, `<Timestamp>%s</Timestamp>`, timestamp)
			}
			fmt.Fprintf(&sb, `<LogicalResourceId>%s</LogicalResourceId>`, xmlEscape(logicalID))
			fmt.Fprintf(&sb, `<ResourceType>%s</ResourceType>`, xmlEscape(resourceType))
			fmt.Fprintf(&sb, `<ResourceStatus>%s</ResourceStatus>`, xmlEscape(status))
			if reason != "" {
				fmt.Fprintf(&sb, `<ResourceStatusReason>%s</ResourceStatusReason>`, xmlEscape(reason))
			}
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</StackEvents>`)

		body := awsQueryXML("DescribeStackEvents", "http://cloudformation.amazonaws.com/doc/2010-05-15/", sb.String())
		return XMLResponse(body), nil
	})

	// ListStackResources — used by cfn_resources child view
	t.Handle("cloudformation", "ListStackResources", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["cfn_resources"](map[string]string{"stack_name": "demo-stack"})
		summaries := ExtractSDK[cfntypes.StackResourceSummary](resources)

		var sb strings.Builder
		sb.WriteString(`<StackResourceSummaries>`)
		for _, s := range summaries {
			logicalID := aws.ToString(s.LogicalResourceId)
			physicalID := aws.ToString(s.PhysicalResourceId)
			resourceType := aws.ToString(s.ResourceType)
			status := string(s.ResourceStatus)
			lastUpdated := ""
			if s.LastUpdatedTimestamp != nil {
				lastUpdated = s.LastUpdatedTimestamp.UTC().Format("2006-01-02T15:04:05Z")
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<LogicalResourceId>%s</LogicalResourceId>`, xmlEscape(logicalID))
			if physicalID != "" {
				fmt.Fprintf(&sb, `<PhysicalResourceId>%s</PhysicalResourceId>`, xmlEscape(physicalID))
			}
			fmt.Fprintf(&sb, `<ResourceType>%s</ResourceType>`, xmlEscape(resourceType))
			fmt.Fprintf(&sb, `<ResourceStatus>%s</ResourceStatus>`, xmlEscape(status))
			if lastUpdated != "" {
				fmt.Fprintf(&sb, `<LastUpdatedTimestamp>%s</LastUpdatedTimestamp>`, lastUpdated)
			}
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</StackResourceSummaries>`)

		body := awsQueryXML("ListStackResources", "http://cloudformation.amazonaws.com/doc/2010-05-15/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// ELBv2 — child view operations (awsquery XML, service "elasticloadbalancing")
// ---------------------------------------------------------------------------

func registerELBChildHandlers(t *Transport) {
	// DescribeListeners — used by elb_listeners child view
	t.Handle("elasticloadbalancing", "DescribeListeners", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["elb_listeners"](map[string]string{
			"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/demo/1234",
		})
		listeners := ExtractSDK[elbv2types.Listener](resources)
		return buildELBListenersXML(listeners), nil
	})

	// DescribeRules — used by elb_listener_rules child view
	t.Handle("elasticloadbalancing", "DescribeRules", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["elb_listener_rules"](map[string]string{})
		rules := ExtractSDK[elbv2types.Rule](resources)
		return buildELBRulesXML(rules), nil
	})

	// DescribeTargetHealth — used by tg_health child view
	t.Handle("elasticloadbalancing", "DescribeTargetHealth", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["tg_health"](map[string]string{
			"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/demo/1234",
		})
		healths := ExtractSDK[elbv2types.TargetHealthDescription](resources)
		return buildELBTargetHealthXML(healths), nil
	})
}

func buildELBListenersXML(listeners []elbv2types.Listener) *http.Response {
	var sb strings.Builder
	sb.WriteString(`<Listeners>`)
	for _, l := range listeners {
		arn := aws.ToString(l.ListenerArn)
		protocol := string(l.Protocol)
		port := int32(0)
		if l.Port != nil {
			port = *l.Port
		}

		fmt.Fprintf(&sb, `<member>`)
		fmt.Fprintf(&sb, `<ListenerArn>%s</ListenerArn>`, xmlEscape(arn))
		fmt.Fprintf(&sb, `<Protocol>%s</Protocol>`, xmlEscape(protocol))
		fmt.Fprintf(&sb, `<Port>%d</Port>`, port)
		fmt.Fprintf(&sb, `<DefaultActions/>`)
		fmt.Fprintf(&sb, `</member>`)
	}
	sb.WriteString(`</Listeners>`)

	body := awsQueryXML("DescribeListeners", "http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/", sb.String())
	return XMLResponse(body)
}

func buildELBRulesXML(rules []elbv2types.Rule) *http.Response {
	var sb strings.Builder
	sb.WriteString(`<Rules>`)
	for _, r := range rules {
		ruleArn := aws.ToString(r.RuleArn)
		priority := aws.ToString(r.Priority)
		isDefault := "false"
		if r.IsDefault != nil && *r.IsDefault {
			isDefault = "true"
		}

		fmt.Fprintf(&sb, `<member>`)
		fmt.Fprintf(&sb, `<RuleArn>%s</RuleArn>`, xmlEscape(ruleArn))
		fmt.Fprintf(&sb, `<Priority>%s</Priority>`, xmlEscape(priority))
		fmt.Fprintf(&sb, `<IsDefault>%s</IsDefault>`, isDefault)
		fmt.Fprintf(&sb, `<Conditions/>`)
		fmt.Fprintf(&sb, `<Actions/>`)
		fmt.Fprintf(&sb, `</member>`)
	}
	sb.WriteString(`</Rules>`)

	body := awsQueryXML("DescribeRules", "http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/", sb.String())
	return XMLResponse(body)
}

func buildELBTargetHealthXML(healths []elbv2types.TargetHealthDescription) *http.Response {
	var sb strings.Builder
	sb.WriteString(`<TargetHealthDescriptions>`)
	for _, h := range healths {
		targetID := ""
		port := int32(0)
		if h.Target != nil {
			targetID = aws.ToString(h.Target.Id)
			if h.Target.Port != nil {
				port = *h.Target.Port
			}
		}
		state := ""
		reason := ""
		description := ""
		if h.TargetHealth != nil {
			state = string(h.TargetHealth.State)
			reason = string(h.TargetHealth.Reason)
			description = aws.ToString(h.TargetHealth.Description)
		}

		fmt.Fprintf(&sb, `<member>`)
		fmt.Fprintf(&sb, `<Target><Id>%s</Id><Port>%d</Port></Target>`, xmlEscape(targetID), port)
		fmt.Fprintf(&sb, `<TargetHealth>`)
		fmt.Fprintf(&sb, `<State>%s</State>`, xmlEscape(state))
		if reason != "" {
			fmt.Fprintf(&sb, `<Reason>%s</Reason>`, xmlEscape(reason))
		}
		if description != "" {
			fmt.Fprintf(&sb, `<Description>%s</Description>`, xmlEscape(description))
		}
		fmt.Fprintf(&sb, `</TargetHealth>`)
		fmt.Fprintf(&sb, `</member>`)
	}
	sb.WriteString(`</TargetHealthDescriptions>`)

	body := awsQueryXML("DescribeTargetHealth", "http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/", sb.String())
	return XMLResponse(body)
}

// ---------------------------------------------------------------------------
// IAM — child view operations (awsquery XML, service "iam")
// ---------------------------------------------------------------------------

func registerIAMChildHandlers(t *Transport) {
	// ListAttachedRolePolicies — used by role_policies child view
	t.Handle("iam", "ListAttachedRolePolicies", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["role_policies"](map[string]string{"role_name": "demo-role"})

		var sb strings.Builder
		sb.WriteString(`<IsTruncated>false</IsTruncated>`)
		sb.WriteString(`<AttachedPolicies>`)
		for _, r := range resources {
			if !strings.EqualFold(r.Fields["policy_type"], "managed") {
				continue
			}
			policyName := r.Fields["policy_name"]
			policyArn := r.Fields["policy_arn"]
			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<PolicyName>%s</PolicyName>`, xmlEscape(policyName))
			fmt.Fprintf(&sb, `<PolicyArn>%s</PolicyArn>`, xmlEscape(policyArn))
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</AttachedPolicies>`)

		body := awsQueryXML("ListAttachedRolePolicies", "https://iam.amazonaws.com/doc/2010-05-08/", sb.String())
		return XMLResponse(body), nil
	})

	// ListRolePolicies — used by role_policies child view (inline policies)
	t.Handle("iam", "ListRolePolicies", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["role_policies"](map[string]string{"role_name": "demo-role"})

		var sb strings.Builder
		sb.WriteString(`<IsTruncated>false</IsTruncated>`)
		sb.WriteString(`<PolicyNames>`)
		for _, r := range resources {
			if !strings.EqualFold(r.Fields["policy_type"], "inline") {
				continue
			}
			policyName := r.Fields["policy_name"]
			fmt.Fprintf(&sb, `<member>%s</member>`, xmlEscape(policyName))
		}
		sb.WriteString(`</PolicyNames>`)

		body := awsQueryXML("ListRolePolicies", "https://iam.amazonaws.com/doc/2010-05-08/", sb.String())
		return XMLResponse(body), nil
	})

	// GetGroup — used by iam_group_members child view
	t.Handle("iam", "GetGroup", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["iam_group_members"](map[string]string{"group_name": "demo-group"})
		users := ExtractSDK[iamtypes.User](resources)

		var sb strings.Builder
		sb.WriteString(`<IsTruncated>false</IsTruncated>`)
		sb.WriteString(`<Users>`)
		for _, u := range users {
			userName := aws.ToString(u.UserName)
			userID := aws.ToString(u.UserId)
			arn := aws.ToString(u.Arn)
			path := aws.ToString(u.Path)
			createDate := ""
			if u.CreateDate != nil {
				createDate = u.CreateDate.UTC().Format("2006-01-02T15:04:05Z")
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<UserName>%s</UserName>`, xmlEscape(userName))
			fmt.Fprintf(&sb, `<UserId>%s</UserId>`, xmlEscape(userID))
			fmt.Fprintf(&sb, `<Arn>%s</Arn>`, xmlEscape(arn))
			fmt.Fprintf(&sb, `<Path>%s</Path>`, xmlEscape(path))
			if createDate != "" {
				fmt.Fprintf(&sb, `<CreateDate>%s</CreateDate>`, createDate)
			}
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</Users>`)
		sb.WriteString(`<Group>`)
		sb.WriteString(`<GroupName>demo-group</GroupName>`)
		sb.WriteString(`<GroupId>AGPADEMO</GroupId>`)
		sb.WriteString(`<Arn>arn:aws:iam::123456789012:group/demo-group</Arn>`)
		sb.WriteString(`<Path>/</Path>`)
		sb.WriteString(`<CreateDate>2025-01-01T00:00:00Z</CreateDate>`)
		sb.WriteString(`</Group>`)

		body := awsQueryXML("GetGroup", "https://iam.amazonaws.com/doc/2010-05-08/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// ECR — child view operations (awsjson11, service "ecr")
// ---------------------------------------------------------------------------

func registerECRChildHandlers(t *Transport) {
	t.Handle("ecr", "DescribeImages", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["ecr_images"](map[string]string{
			"repository_name": "demo-repo",
			"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/demo-repo",
		})
		images := ExtractSDK[ecrtypes.ImageDetail](resources)

		out := &ecr.DescribeImagesOutput{
			ImageDetails: images,
		}
		return JSONResponseCamelCase(out)
	})
}

// ---------------------------------------------------------------------------
// CodeBuild — child view operations (awsjson11, service "codebuild")
// ---------------------------------------------------------------------------

func registerCodeBuildChildHandlers(t *Transport) {
	// ListBuildsForProject — used by cb_builds child view (step 1)
	t.Handle("codebuild", "ListBuildsForProject", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["cb_builds"](map[string]string{"project_name": "demo-project"})
		builds := ExtractSDK[cbtypes.Build](resources)

		ids := make([]string, 0, len(builds))
		for _, b := range builds {
			if b.Id != nil {
				ids = append(ids, *b.Id)
			}
		}

		out := &codebuild.ListBuildsForProjectOutput{
			Ids: ids,
		}
		return JSONResponseCamelCase(out)
	})

	// BatchGetBuilds — used by cb_builds child view (step 2, fetches full build details)
	t.Handle("codebuild", "BatchGetBuilds", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["cb_builds"](map[string]string{"project_name": "demo-project"})
		builds := ExtractSDK[cbtypes.Build](resources)

		out := &codebuild.BatchGetBuildsOutput{
			Builds: builds,
		}
		return JSONResponseCamelCase(out)
	})
}

// ---------------------------------------------------------------------------
// CodePipeline — child view operations (awsjson11, service "codepipeline")
// ---------------------------------------------------------------------------

func registerCodePipelineChildHandlers(t *Transport) {
	// GetPipelineState — used by pipeline_stages child view
	t.Handle("codepipeline", "GetPipelineState", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["pipeline_stages"](map[string]string{"pipeline_name": "demo-pipeline"})
		stageStates := buildPipelineStageStates(resources)

		out := &codepipeline.GetPipelineStateOutput{
			PipelineName:    aws.String("demo-pipeline"),
			PipelineVersion: aws.Int32(1),
			StageStates:     stageStates,
		}
		return JSONResponseCamelCase(out)
	})
}

// buildPipelineStageStates converts pipeline_stages fixtures into SDK StageState structs.
// Each fixture resource represents a stage-action row. Consecutive rows with the same
// non-empty stage_name field start a new stage.
func buildPipelineStageStates(resources []resource.Resource) []cptypes.StageState {
	var states []cptypes.StageState
	var currentStage *cptypes.StageState

	for _, r := range resources {
		stageName := r.Fields["stage_name"]
		stageStatus := r.Fields["stage_status"]
		actionName := r.Fields["action_name"]
		actionStatus := r.Fields["action_status"]

		// Start a new stage when the stage_name is non-empty
		if stageName != "" {
			if currentStage != nil {
				states = append(states, *currentStage)
			}
			latestExec := &cptypes.StageExecution{}
			if stageStatus != "" {
				latestExec.Status = cptypes.StageExecutionStatus(stageStatus)
			}
			currentStage = &cptypes.StageState{
				StageName:       aws.String(stageName),
				LatestExecution: latestExec,
				ActionStates:    []cptypes.ActionState{},
			}
		}

		if currentStage == nil {
			continue
		}

		// Add action state to current stage
		actionExec := &cptypes.ActionExecution{}
		if actionStatus != "" {
			actionExec.Status = cptypes.ActionExecutionStatus(actionStatus)
		}
		currentStage.ActionStates = append(currentStage.ActionStates, cptypes.ActionState{
			ActionName:      aws.String(actionName),
			LatestExecution: actionExec,
		})
	}

	if currentStage != nil {
		states = append(states, *currentStage)
	}

	return states
}

// ---------------------------------------------------------------------------
// AutoScaling — child view operations (awsquery XML, service "autoscaling")
// ---------------------------------------------------------------------------

func registerASGChildHandlers(t *Transport) {
	t.Handle("autoscaling", "DescribeScalingActivities", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["asg_activities"](map[string]string{"asg_name": "demo-asg"})
		activities := ExtractSDK[asgtypes.Activity](resources)

		var sb strings.Builder
		sb.WriteString(`<Activities>`)
		for _, a := range activities {
			activityID := aws.ToString(a.ActivityId)
			asgName := aws.ToString(a.AutoScalingGroupName)
			cause := aws.ToString(a.Cause)
			desc := aws.ToString(a.Description)
			status := string(a.StatusCode)
			startTime := ""
			if a.StartTime != nil {
				startTime = a.StartTime.UTC().Format("2006-01-02T15:04:05Z")
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<ActivityId>%s</ActivityId>`, xmlEscape(activityID))
			fmt.Fprintf(&sb, `<AutoScalingGroupName>%s</AutoScalingGroupName>`, xmlEscape(asgName))
			fmt.Fprintf(&sb, `<Cause>%s</Cause>`, xmlEscape(cause))
			fmt.Fprintf(&sb, `<Description>%s</Description>`, xmlEscape(desc))
			fmt.Fprintf(&sb, `<StatusCode>%s</StatusCode>`, xmlEscape(status))
			if startTime != "" {
				fmt.Fprintf(&sb, `<StartTime>%s</StartTime>`, startTime)
			}
			fmt.Fprintf(&sb, `<Progress>100</Progress>`)
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</Activities>`)

		body := awsQueryXML("DescribeScalingActivities", "http://autoscaling.amazonaws.com/doc/2011-01-01/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// RDS — child view operations (awsquery XML, service "rds")
// ---------------------------------------------------------------------------

func registerRDSChildHandlers(t *Transport) {
	// DescribeEvents — used by dbi_events child view
	t.Handle("rds", "DescribeEvents", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["dbi_events"](map[string]string{"db_identifier": "demo-db"})
		events := ExtractSDK[rdstypes.Event](resources)

		var sb strings.Builder
		sb.WriteString(`<Events>`)
		for _, e := range events {
			sourceID := aws.ToString(e.SourceIdentifier)
			message := aws.ToString(e.Message)
			date := ""
			if e.Date != nil {
				date = e.Date.UTC().Format("2006-01-02T15:04:05Z")
			}

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<SourceIdentifier>%s</SourceIdentifier>`, xmlEscape(sourceID))
			fmt.Fprintf(&sb, `<Message>%s</Message>`, xmlEscape(message))
			if date != "" {
				fmt.Fprintf(&sb, `<Date>%s</Date>`, date)
			}
			if len(e.EventCategories) > 0 {
				fmt.Fprintf(&sb, `<EventCategories>`)
				for _, cat := range e.EventCategories {
					fmt.Fprintf(&sb, `<EventCategory>%s</EventCategory>`, xmlEscape(cat))
				}
				fmt.Fprintf(&sb, `</EventCategories>`)
			}
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</Events>`)

		body := awsQueryXML("DescribeEvents", "http://rds.amazonaws.com/doc/2014-10-31/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// SNS — child view operations (awsquery XML, service "sns")
// ---------------------------------------------------------------------------

func registerSNSChildHandlers(t *Transport) {
	// ListSubscriptionsByTopic — used by sns_subscriptions child view
	t.Handle("sns", "ListSubscriptionsByTopic", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["sns_subscriptions"](map[string]string{
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:demo-topic",
		})
		subs := ExtractSDK[snstypes.Subscription](resources)

		var sb strings.Builder
		sb.WriteString(`<Subscriptions>`)
		for _, sub := range subs {
			topicArn := aws.ToString(sub.TopicArn)
			protocol := aws.ToString(sub.Protocol)
			endpoint := aws.ToString(sub.Endpoint)
			subArn := aws.ToString(sub.SubscriptionArn)
			owner := aws.ToString(sub.Owner)

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<TopicArn>%s</TopicArn>`, xmlEscape(topicArn))
			fmt.Fprintf(&sb, `<Protocol>%s</Protocol>`, xmlEscape(protocol))
			fmt.Fprintf(&sb, `<Endpoint>%s</Endpoint>`, xmlEscape(endpoint))
			fmt.Fprintf(&sb, `<SubscriptionArn>%s</SubscriptionArn>`, xmlEscape(subArn))
			fmt.Fprintf(&sb, `<Owner>%s</Owner>`, xmlEscape(owner))
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</Subscriptions>`)

		body := awsQueryXML("ListSubscriptionsByTopic", "http://sns.amazonaws.com/doc/2010-03-31/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// Step Functions — child view operations (awsjson10, service "states")
// ---------------------------------------------------------------------------

func registerSFNChildHandlers(t *Transport) {
	// ListExecutions — used by sfn_executions child view
	t.Handle("states", "ListExecutions", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["sfn_executions"](map[string]string{
			"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:demo-sm",
		})
		executions := ExtractSDK[sfntypes.ExecutionListItem](resources)

		out := &sfn.ListExecutionsOutput{
			Executions: executions,
		}
		return JSONResponseCamelCase(out)
	})

	// GetExecutionHistory — used by sfn_execution_history child view
	t.Handle("states", "GetExecutionHistory", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["sfn_execution_history"](map[string]string{})
		events := ExtractSDK[sfntypes.HistoryEvent](resources)

		out := &sfn.GetExecutionHistoryOutput{
			Events: events,
		}
		return JSONResponseCamelCase(out)
	})
}

// ---------------------------------------------------------------------------
// EventBridge — child view operations (awsjson11, service "events")
// ---------------------------------------------------------------------------

func registerEBChildHandlers(t *Transport) {
	// ListTargetsByRule — used by eb_rule_targets child view
	t.Handle("events", "ListTargetsByRule", func(_ *http.Request) (*http.Response, error) {
		resources := childDemoData["eb_rule_targets"](map[string]string{})
		targets := ExtractSDK[eventbridgetypes.Target](resources)

		out := &eventbridge.ListTargetsByRuleOutput{
			Targets: targets,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// ECS — child view operations (awsjson11, service "ecs")
// Note: DescribeServices, ListTasks, DescribeTasks are in registerECSFullHandlers.
// This adds DescribeTaskDefinition for the ecs_svc_logs child view.
// ---------------------------------------------------------------------------

func registerECSChildHandlers(t *Transport) {
	// DescribeTaskDefinition — used by ecs_svc_logs to get log configuration
	t.Handle("ecs", "DescribeTaskDefinition", func(_ *http.Request) (*http.Response, error) {
		logGroup := "/aws/ecs/acme-services/api-gateway"
		streamPrefix := "api-gateway"
		out := &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				Family:   aws.String("api-gateway"),
				Revision: 12,
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name:  aws.String("app"),
						Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service:latest"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         logGroup,
								"awslogs-stream-prefix": streamPrefix,
								"awslogs-region":        "us-east-1",
							},
						},
					},
				},
			},
		}
		return JSONResponseCamelCase(out)
	})
}
