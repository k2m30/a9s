package unit

// wave2_field_updates_test.go — TDD contracts for Wave-2 enricher FieldUpdates and
// Wave-1 fetcher-computed field additions introduced in the 017-issue-counts branch.
//
// These tests will FAIL until the coder lands the corresponding production changes.
// Once the coder's implementation is merged, all tests in this file must PASS.
//
// Groups:
//   Group 1  (#1–14)  Wave-2 enricher FieldUpdates
//   Group 2  (#15–16) Pure Wave-1 column-source checks (EC2 status, Redshift path)
//   Group 3  (#17–22) Fetcher-computed Wave-1 field additions
//   Group 4  (#23–24) Cosmetic format fields (ACM days_left, AMI deprecated)

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	codeartifactsvc "github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2svc "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	secretsmanagertypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// Group 1: Wave-2 enricher FieldUpdates
// ─────────────────────────────────────────────────────────────────────────────

// Test #1 — tg: health_summary FieldUpdates

// tgHealthFakeW2 implements ELBv2API for target-group health testing.
// It embeds the interface and overrides only DescribeTargetHealth.
// Named W2 to avoid conflict with tgHealthFake in enrichment_tg_findings_test.go.
type tgHealthFakeW2 struct {
	awsclient.ELBv2API
	// results maps TargetGroupArn → target health descriptions.
	results map[string][]elbtypes.TargetHealthDescription
}

func (f *tgHealthFakeW2) DescribeTargetHealth(
	_ context.Context,
	in *elbv2svc.DescribeTargetHealthInput,
	_ ...func(*elbv2svc.Options),
) (*elbv2svc.DescribeTargetHealthOutput, error) {
	arn := ""
	if in != nil && in.TargetGroupArn != nil {
		arn = *in.TargetGroupArn
	}
	descs := f.results[arn]
	return &elbv2svc.DescribeTargetHealthOutput{TargetHealthDescriptions: descs}, nil
}

var _ awsclient.ELBv2API = (*tgHealthFakeW2)(nil)

// TestEnrichTargetGroupHealth_WritesHealthSummary verifies that EnrichTargetGroupHealth
// populates FieldUpdates["health_summary"] with either "N/M healthy" for a TG with some
// unhealthy targets, or "ORPHAN" for a TG with no targets.
func TestEnrichTargetGroupHealth_WritesHealthSummary(t *testing.T) {
	tgARN1 := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/tg-unhealthy/aabbccdd11"
	tgARN2 := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/tg-orphan/aabbccdd22"

	fake := &tgHealthFakeW2{
		results: map[string][]elbtypes.TargetHealthDescription{
			tgARN1: {
				{
					TargetHealth: &elbtypes.TargetHealth{
						State: elbtypes.TargetHealthStateEnumHealthy,
					},
				},
				{
					TargetHealth: &elbtypes.TargetHealth{
						State:  elbtypes.TargetHealthStateEnumUnhealthy,
						Reason: elbtypes.TargetHealthReasonEnumFailedHealthChecks,
					},
				},
			},
			// tgARN2 has no targets → ORPHAN
			tgARN2: {},
		},
	}

	const tgName1 = "tg-unhealthy"
	const tgName2 = "tg-orphan"
	resources := []resource.Resource{
		{ID: tgName1, Name: tgName1, Fields: map[string]string{"target_group_arn": tgARN1}},
		{ID: tgName2, Name: tgName2, Fields: map[string]string{"target_group_arn": tgARN2}},
	}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), &awsclient.ServiceClients{ELBv2: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// tgName1: "1/2 healthy" pattern (FieldUpdates is keyed by r.ID, which is the bare name)
	fu1, ok := result.FieldUpdates[tgName1]
	if !ok {
		t.Errorf("FieldUpdates missing entry for %q", tgName1)
	} else {
		hs := fu1["health_summary"]
		matched, _ := regexp.MatchString(`^\d+/\d+ healthy$`, hs)
		if !matched {
			t.Errorf("%s health_summary = %q, want pattern %%d/%%d healthy", tgName1, hs)
		}
	}

	// tgName2: "ORPHAN"
	fu2, ok := result.FieldUpdates[tgName2]
	if !ok {
		t.Errorf("FieldUpdates missing entry for %q", tgName2)
	} else {
		if fu2["health_summary"] != "ORPHAN" {
			t.Errorf("%s health_summary = %q, want %q", tgName2, fu2["health_summary"], "ORPHAN")
		}
	}
}

// Test #2 — vpc: flow_logs FieldUpdates

// vpcFlowLogFakeW is a local helper for vpc FieldUpdates test.
// (Cannot reuse vpcFlowLogFake from aws_vpc_enricher_test.go without conflict.)
type vpcFlowLogFakeW struct {
	awsclient.EC2API
	results map[string][]ec2types.FlowLog
}

func (f *vpcFlowLogFakeW) DescribeFlowLogs(
	_ context.Context,
	in *ec2.DescribeFlowLogsInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeFlowLogsOutput, error) {
	vpcID := ""
	if in != nil {
		for _, filter := range in.Filter {
			if filter.Name != nil && *filter.Name == "resource-id" && len(filter.Values) > 0 {
				vpcID = filter.Values[0]
				break
			}
		}
	}
	logs := f.results[vpcID]
	return &ec2.DescribeFlowLogsOutput{FlowLogs: logs}, nil
}

// TestEnrichVPCFlowLogs_WritesFlowLogsField verifies that EnrichVPCFlowLogs
// sets FieldUpdates[vpcID]["flow_logs"] == "no" when no ACTIVE flow logs exist.
func TestEnrichVPCFlowLogs_WritesFlowLogsField(t *testing.T) {
	vpcID := "vpc-11111111"

	fake := &vpcFlowLogFakeW{
		results: map[string][]ec2types.FlowLog{
			vpcID: {}, // empty → no active flow logs
		},
	}

	resources := []resource.Resource{
		{ID: vpcID, Name: vpcID},
	}

	result, err := awsclient.EnrichVPCFlowLogs(context.Background(), &awsclient.ServiceClients{EC2: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[vpcID]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for vpc %q", vpcID)
	}
	if fu["flow_logs"] != "no" {
		t.Errorf("vpc flow_logs = %q, want %q", fu["flow_logs"], "no")
	}
}

// Test #3 — tgw: att_status FieldUpdates

// TestEnrichTGWAttachments_WritesAttStatus verifies that EnrichTGWAttachments
// populates FieldUpdates[tgwID]["att_status"] with a non-empty value when
// at least one attachment is in a failed state.
func TestEnrichTGWAttachments_WritesAttStatus(t *testing.T) {
	tgwID := "tgw-00000099"

	fake := &tgwAttachmentFake{
		results: map[string][]ec2types.TransitGatewayAttachment{
			tgwID: {
				{
					TransitGatewayId:           aws.String(tgwID),
					TransitGatewayAttachmentId: aws.String("tgw-attach-z001"),
					State:                      ec2types.TransitGatewayAttachmentStateFailed,
				},
			},
		},
	}

	resources := tgwResources(tgwID)
	result, err := awsclient.EnrichTGWAttachments(context.Background(), &awsclient.ServiceClients{EC2: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[tgwID]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for tgw %q", tgwID)
	}
	attStatus := fu["att_status"]
	if attStatus == "" {
		t.Errorf("tgw att_status is empty, want non-empty (e.g. \"1 issues\") when attachment is failed")
	}
}

// Test #4 — sqs: dlq FieldUpdates

// TestEnrichSQSAttributes_WritesDLQField verifies that EnrichSQSAttributes
// sets FieldUpdates[queueID]["dlq"] == "yes" when RedrivePolicy is present,
// and "no" when absent.
func TestEnrichSQSAttributes_WritesDLQField(t *testing.T) {
	nameWithDLQ := "my-queue-with-dlq"
	nameWithoutDLQ := "my-queue-no-dlq"

	// reuse sqsURLFor and sqsGetQueueAttributesFake from aws_sqs_enricher_test.go (same package)
	fake := &sqsGetQueueAttributesFake{
		results: map[string]map[string]string{
			sqsURLFor(nameWithDLQ): {
				"RedrivePolicy": `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:dlq","maxReceiveCount":"5"}`,
			},
			sqsURLFor(nameWithoutDLQ): {
				// No RedrivePolicy
			},
		},
	}

	resources := sqsResources(nameWithDLQ, nameWithoutDLQ)
	result, err := awsclient.EnrichSQSAttributes(context.Background(), &awsclient.ServiceClients{SQS: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Queue with DLQ → "yes"
	fu1, ok := result.FieldUpdates[nameWithDLQ]
	if !ok {
		t.Fatalf("FieldUpdates missing for queue %q", nameWithDLQ)
	}
	if fu1["dlq"] != "yes" {
		t.Errorf("dlq for %q = %q, want %q", nameWithDLQ, fu1["dlq"], "yes")
	}

	// Queue without DLQ → "no"
	fu2, ok := result.FieldUpdates[nameWithoutDLQ]
	if !ok {
		t.Fatalf("FieldUpdates missing for queue %q", nameWithoutDLQ)
	}
	if fu2["dlq"] != "no" {
		t.Errorf("dlq for %q = %q, want %q", nameWithoutDLQ, fu2["dlq"], "no")
	}
}

// Test #5 — sns: subs_count FieldUpdates

// snsFakeW is a local minimal fake for the subs_count test; avoids conflict with
// snsListSubscriptionsByTopicFake from aws_sns_enricher_test.go.
type snsFakeW struct {
	awsclient.SNSAPI
	results map[string][]snstypes.Subscription
}

func (f *snsFakeW) ListSubscriptionsByTopic(
	_ context.Context,
	in *sns.ListSubscriptionsByTopicInput,
	_ ...func(*sns.Options),
) (*sns.ListSubscriptionsByTopicOutput, error) {
	arn := ""
	if in != nil && in.TopicArn != nil {
		arn = *in.TopicArn
	}
	subs := f.results[arn]
	return &sns.ListSubscriptionsByTopicOutput{Subscriptions: subs}, nil
}

// TestEnrichSNSSubscriptions_WritesSubsCount verifies that EnrichSNSSubscriptions
// populates FieldUpdates[topicARN]["subs_count"] with the number of confirmed
// subscriptions in the fake response.
func TestEnrichSNSSubscriptions_WritesSubsCount(t *testing.T) {
	topicARN := "arn:aws:sns:us-east-1:123456789012:my-topic"
	confirmedARN1 := "arn:aws:sns:us-east-1:123456789012:my-topic:sub-aaa"
	confirmedARN2 := "arn:aws:sns:us-east-1:123456789012:my-topic:sub-bbb"
	confirmedARN3 := "arn:aws:sns:us-east-1:123456789012:my-topic:sub-ccc"

	fake := &snsFakeW{
		results: map[string][]snstypes.Subscription{
			topicARN: {
				{SubscriptionArn: aws.String(confirmedARN1)},
				{SubscriptionArn: aws.String(confirmedARN2)},
				{SubscriptionArn: aws.String(confirmedARN3)},
			},
		},
	}

	resources := []resource.Resource{
		{ID: topicARN, Name: "my-topic"},
	}

	result, err := awsclient.EnrichSNSSubscriptions(context.Background(), &awsclient.ServiceClients{SNS: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[topicARN]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for topic %q", topicARN)
	}
	if fu["subs_count"] != "3" {
		t.Errorf("subs_count = %q, want %q", fu["subs_count"], "3")
	}
}

// Test #6 — sfn: last_run FieldUpdates (coder must add this)

// sfnFakeW implements SFNAPI for the last_run FieldUpdates test.
type sfnFakeW struct {
	awsclient.SFNAPI
	// results maps stateMachineArn → executions.
	results map[string][]sfntypes.ExecutionListItem
}

func (f *sfnFakeW) ListExecutions(
	_ context.Context,
	in *sfn.ListExecutionsInput,
	_ ...func(*sfn.Options),
) (*sfn.ListExecutionsOutput, error) {
	arn := ""
	if in != nil && in.StateMachineArn != nil {
		arn = *in.StateMachineArn
	}
	execs := f.results[arn]
	return &sfn.ListExecutionsOutput{Executions: execs}, nil
}

// TestEnrichStepFunctionsStatus_WritesLastRun verifies that EnrichStepFunctionsStatus
// populates FieldUpdates[sfnARN]["last_run"] with a value containing "FAILED" when
// the latest execution is in FAILED state.
func TestEnrichStepFunctionsStatus_WritesLastRun(t *testing.T) {
	sfnARN := "arn:aws:states:us-east-1:123456789012:stateMachine:my-sfn"

	now := time.Now()
	fake := &sfnFakeW{
		results: map[string][]sfntypes.ExecutionListItem{
			sfnARN: {
				{
					ExecutionArn:    aws.String(sfnARN + ":exec-001"),
					StateMachineArn: aws.String(sfnARN),
					Name:            aws.String("exec-001"),
					Status:          sfntypes.ExecutionStatusFailed,
					StartDate:       &now,
					StopDate:        &now,
				},
			},
		},
	}

	const sfnName = "my-sfn"
	resources := []resource.Resource{
		{ID: sfnName, Name: sfnName, Fields: map[string]string{"arn": sfnARN}},
	}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), &awsclient.ServiceClients{SFN: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FieldUpdates is keyed by r.ID (the bare name), per fetcher contract.
	fu, ok := result.FieldUpdates[sfnName]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for sfn %q — coder must add last_run FieldUpdates to EnrichStepFunctionsStatus", sfnName)
	}
	lastRun := fu["last_run"]
	if !strings.Contains(lastRun, "FAILED") {
		t.Errorf("sfn last_run = %q, want value containing %q", lastRun, "FAILED")
	}
}

// Test #7 — policy: risk FieldUpdates (coder must add this)

// TestEnrichIAMPolicy_WritesRiskField verifies that EnrichIAMPolicy populates
// FieldUpdates[policyID]["risk"] == "ADMIN_ALL" for a policy with Effect:Allow Action:* Resource:*.
func TestEnrichIAMPolicy_WritesRiskField(t *testing.T) {
	policyARN := "arn:aws:iam::123456789012:policy/AdminStarPolicy"
	adminDoc := `{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Action": "*",
			"Resource": "*"
		}]
	}`

	fake := &iamPolicyFake{
		getPolicyResults: map[string]*iam.GetPolicyOutput{
			policyARN: {
				Policy: &iamtypes.Policy{
					Arn:              aws.String(policyARN),
					DefaultVersionId: aws.String("v1"),
				},
			},
		},
		getPolicyVersionResults: map[string]*iam.GetPolicyVersionOutput{
			policyARN: {
				PolicyVersion: &iamtypes.PolicyVersion{
					Document: aws.String(url.QueryEscape(adminDoc)),
				},
			},
		},
	}

	resources := []resource.Resource{
		{
			ID:   policyARN,
			Name: "AdminStarPolicy",
			RawStruct: iamtypes.Policy{
				Arn:              aws.String(policyARN),
				PolicyName:       aws.String("AdminStarPolicy"),
				DefaultVersionId: aws.String("v1"),
			},
		},
	}

	result, err := awsclient.EnrichIAMPolicy(context.Background(), &awsclient.ServiceClients{IAM: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[policyARN]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for policy %q — coder must add risk FieldUpdates to EnrichIAMPolicy", policyARN)
	}
	if fu["risk"] != "ADMIN_ALL" {
		t.Errorf("policy risk = %q, want %q", fu["risk"], "ADMIN_ALL")
	}
}

// Test #8 — waf: rules_summary FieldUpdates (coder must add this)

// TestEnrichWAF_WritesRulesSummary verifies that EnrichWAFLogging populates
// FieldUpdates[wafARN]["rules_summary"] containing "0 rules" for a WebACL
// with no rules configured.
func TestEnrichWAF_WritesRulesSummary(t *testing.T) {
	wafARN := "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/aaaaaaaa-bbbb"

	// reuse wafLoggingFake from aws_waf_enricher_test.go (same package)
	fake := &wafLoggingFake{
		resourcesResults: map[string]*wafv2.ListResourcesForWebACLOutput{
			wafARN: {ResourceArns: []string{}},
		},
	}

	resources := []resource.Resource{
		{ID: wafARN, Name: "my-waf", Fields: map[string]string{"arn": wafARN}},
	}

	result, err := awsclient.EnrichWAFLogging(context.Background(), &awsclient.ServiceClients{WAFv2: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[wafARN]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for waf %q — coder must add rules_summary FieldUpdates to EnrichWAFLogging", wafARN)
	}
	rulesSummary := fu["rules_summary"]
	if !strings.Contains(rulesSummary, "0 rules") {
		t.Errorf("waf rules_summary = %q, want value containing %q", rulesSummary, "0 rules")
	}
}

// Test #9 — apigw: stages_count FieldUpdates (coder must add this)

// TestEnrichAPIGatewayStage_WritesStagesCount verifies that EnrichAPIGatewayStage
// populates FieldUpdates[apiID]["stages_count"] with the number of stages.
func TestEnrichAPIGatewayStage_WritesStagesCount(t *testing.T) {
	apiID1 := "aabbccdd11"
	apiID2 := "eeff001122"

	// Stage builder: good throttling + access logs = no finding.
	goodStage := func(name string) apigwtypes.Stage {
		return apigwtypes.Stage{
			StageName: aws.String(name),
			DefaultRouteSettings: &apigwtypes.RouteSettings{
				ThrottlingBurstLimit: aws.Int32(1000),
				ThrottlingRateLimit:  aws.Float64(500),
			},
			AccessLogSettings: &apigwtypes.AccessLogSettings{
				DestinationArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/api-gw/" + name),
			},
		}
	}

	// reuse apigwGetStagesFake and apigwResources from aws_apigw_enricher_test.go (same package)
	fake := &apigwGetStagesFake{
		results: map[string][]apigwtypes.Stage{
			apiID1: {goodStage("prod"), goodStage("dev")}, // 2 stages
			apiID2: {},                                    // 0 stages (orphan)
		},
	}

	resources := apigwResources(apiID1, apiID2)
	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), &awsclient.ServiceClients{APIGatewayV2: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// apiID1: 2 stages
	fu1, ok := result.FieldUpdates[apiID1]
	if !ok {
		t.Fatalf("FieldUpdates missing for api %q — coder must add stages_count FieldUpdates to EnrichAPIGatewayStage", apiID1)
	}
	if fu1["stages_count"] != "2" {
		t.Errorf("api %q stages_count = %q, want %q", apiID1, fu1["stages_count"], "2")
	}

	// apiID2: 0 stages
	fu2, ok := result.FieldUpdates[apiID2]
	if !ok {
		t.Fatalf("FieldUpdates missing for api %q — coder must add stages_count FieldUpdates to EnrichAPIGatewayStage", apiID2)
	}
	if fu2["stages_count"] != "0" {
		t.Errorf("api %q stages_count = %q, want %q", apiID2, fu2["stages_count"], "0")
	}
}

// Test #10 — pipeline: last_status FieldUpdates (coder must add this)

// cpGetPipelineStateFake implements CodePipelineAPI for last_status test.
type cpGetPipelineStateFake struct {
	awsclient.CodePipelineAPI
	// results maps pipeline name → GetPipelineStateOutput.
	results map[string]*codepipeline.GetPipelineStateOutput
}

func (f *cpGetPipelineStateFake) GetPipelineState(
	_ context.Context,
	in *codepipeline.GetPipelineStateInput,
	_ ...func(*codepipeline.Options),
) (*codepipeline.GetPipelineStateOutput, error) {
	name := ""
	if in != nil && in.Name != nil {
		name = *in.Name
	}
	out, ok := f.results[name]
	if !ok {
		return &codepipeline.GetPipelineStateOutput{}, nil
	}
	return out, nil
}

// TestEnrichCodePipelineStatus_WritesLastStatus verifies that EnrichCodePipelineStatus
// populates FieldUpdates[pipelineKey]["last_status"] containing "FAILED" or a stage name
// when the pipeline has a failed stage.
func TestEnrichCodePipelineStatus_WritesLastStatus(t *testing.T) {
	pipelineName := "my-deploy-pipeline"

	fake := &cpGetPipelineStateFake{
		results: map[string]*codepipeline.GetPipelineStateOutput{
			pipelineName: {
				PipelineName: aws.String(pipelineName),
				StageStates: []cptypes.StageState{
					{
						StageName: aws.String("Build"),
						LatestExecution: &cptypes.StageExecution{
							Status: cptypes.StageExecutionStatusFailed,
						},
					},
				},
			},
		},
	}

	resources := []resource.Resource{
		{ID: pipelineName, Name: pipelineName},
	}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), &awsclient.ServiceClients{CodePipeline: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[pipelineName]
	if !ok {
		t.Fatalf("FieldUpdates missing for pipeline %q — coder must add last_status FieldUpdates to EnrichCodePipelineStatus", pipelineName)
	}
	lastStatus := fu["last_status"]
	if !strings.Contains(lastStatus, "FAILED") && !strings.Contains(lastStatus, "Build") {
		t.Errorf("pipeline last_status = %q, want value containing \"FAILED\" or stage name \"Build\"", lastStatus)
	}
}

// Test #11 — cb: last_build FieldUpdates (coder must add this)

// cbFakeW implements CodeBuildAPI for the last_build test.
type cbFakeW struct {
	awsclient.CodeBuildAPI
	// listBuilds maps project name → build IDs.
	listBuilds map[string][]string
	// batchBuilds maps build ID → Build.
	batchBuilds map[string]cbtypes.Build
}

func (f *cbFakeW) ListBuildsForProject(
	_ context.Context,
	in *codebuild.ListBuildsForProjectInput,
	_ ...func(*codebuild.Options),
) (*codebuild.ListBuildsForProjectOutput, error) {
	name := ""
	if in != nil && in.ProjectName != nil {
		name = *in.ProjectName
	}
	ids := f.listBuilds[name]
	return &codebuild.ListBuildsForProjectOutput{Ids: ids}, nil
}

func (f *cbFakeW) BatchGetBuilds(
	_ context.Context,
	in *codebuild.BatchGetBuildsInput,
	_ ...func(*codebuild.Options),
) (*codebuild.BatchGetBuildsOutput, error) {
	var builds []cbtypes.Build
	for _, id := range in.Ids {
		if b, ok := f.batchBuilds[id]; ok {
			builds = append(builds, b)
		}
	}
	return &codebuild.BatchGetBuildsOutput{Builds: builds}, nil
}

// TestEnrichCodeBuildStatus_WritesLastBuild verifies that EnrichCodeBuildStatus
// populates FieldUpdates[projectName]["last_build"] containing "FAILED" when
// the latest build status is FAILED.
func TestEnrichCodeBuildStatus_WritesLastBuild(t *testing.T) {
	projectName := "my-build-project"
	buildID := projectName + ":aabbcc001122"

	now := time.Now()
	fake := &cbFakeW{
		listBuilds: map[string][]string{
			projectName: {buildID},
		},
		batchBuilds: map[string]cbtypes.Build{
			buildID: {
				Id:            aws.String(buildID),
				ProjectName:   aws.String(projectName),
				BuildStatus:   cbtypes.StatusTypeFailed,
				BuildComplete: true,
				EndTime:       &now,
			},
		},
	}

	resources := []resource.Resource{
		{ID: projectName, Name: projectName},
	}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), &awsclient.ServiceClients{CodeBuild: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[projectName]
	if !ok {
		t.Fatalf("FieldUpdates missing for project %q — coder must add last_build FieldUpdates to EnrichCodeBuildStatus", projectName)
	}
	lastBuild := fu["last_build"]
	if !strings.Contains(lastBuild, "FAILED") {
		t.Errorf("cb last_build = %q, want value containing %q", lastBuild, "FAILED")
	}
}

// Test #12 — codeartifact: package_count FieldUpdates (coder must add this)

// codeArtifactPackageFake implements CodeArtifactAPI for package_count FieldUpdates test.
// It overrides ListPackages to return a controlled set of packages per domain/repository.
type codeArtifactPackageFake struct {
	awsclient.CodeArtifactAPI
	// listPackagesResults maps "domain/repo" → slice of PackageSummary.
	listPackagesResults map[string][]codeartifacttypes.PackageSummary
}

func (f *codeArtifactPackageFake) ListPackages(
	_ context.Context,
	in *codeartifactsvc.ListPackagesInput,
	_ ...func(*codeartifactsvc.Options),
) (*codeartifactsvc.ListPackagesOutput, error) {
	key := ""
	if in != nil {
		domain := aws.ToString(in.Domain)
		repo := aws.ToString(in.Repository)
		key = domain + "/" + repo
	}
	pkgs := f.listPackagesResults[key]
	return &codeartifactsvc.ListPackagesOutput{Packages: pkgs}, nil
}

// GetRepositoryPermissionsPolicy returns a non-error empty output (no policy configured).
func (f *codeArtifactPackageFake) GetRepositoryPermissionsPolicy(
	_ context.Context,
	_ *codeartifactsvc.GetRepositoryPermissionsPolicyInput,
	_ ...func(*codeartifactsvc.Options),
) (*codeartifactsvc.GetRepositoryPermissionsPolicyOutput, error) {
	return &codeartifactsvc.GetRepositoryPermissionsPolicyOutput{}, nil
}

var _ awsclient.CodeArtifactAPI = (*codeArtifactPackageFake)(nil)

// TestEnrichCodeArtifactRepository_WritesPackageCount verifies that EnrichCodeArtifactRepository
// populates FieldUpdates[repoKey]["package_count"] == "0" for an empty repository.
func TestEnrichCodeArtifactRepository_WritesPackageCount(t *testing.T) {
	repoName := "my-empty-repo"
	domainName := "my-domain"
	repoID := "arn:aws:codeartifact:us-east-1:123456789012:repository/my-domain/my-empty-repo"

	fake := &codeArtifactPackageFake{
		listPackagesResults: map[string][]codeartifacttypes.PackageSummary{
			domainName + "/" + repoName: {}, // empty → package_count = 0
		},
	}

	resources := []resource.Resource{
		{
			ID:   repoID,
			Name: repoName,
			Fields: map[string]string{
				"repo_name":   repoName,
				"domain_name": domainName,
				"domain":      domainName,
			},
		},
	}

	result, err := awsclient.EnrichCodeArtifactRepository(context.Background(), &awsclient.ServiceClients{CodeArtifact: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[repoID]
	if !ok {
		t.Fatalf("FieldUpdates missing for repo %q — coder must add package_count FieldUpdates to EnrichCodeArtifactRepository", repoID)
	}
	if fu["package_count"] != "0" {
		t.Errorf("codeartifact package_count = %q, want %q", fu["package_count"], "0")
	}
}

// Test #13 — glue: last_run FieldUpdates (coder must add this)

// glueFakeW implements GlueAPI for the last_run test.
type glueFakeW struct {
	awsclient.GlueAPI
	// results maps job name → job runs.
	results map[string][]gluetypes.JobRun
}

func (f *glueFakeW) GetJobRuns(
	_ context.Context,
	in *glue.GetJobRunsInput,
	_ ...func(*glue.Options),
) (*glue.GetJobRunsOutput, error) {
	name := ""
	if in != nil && in.JobName != nil {
		name = *in.JobName
	}
	runs := f.results[name]
	return &glue.GetJobRunsOutput{JobRuns: runs}, nil
}

// TestEnrichGlueJobStatus_WritesLastRun verifies that EnrichGlueJobStatus
// populates FieldUpdates[jobKey]["last_run"] containing "ERROR" or "FAILED" when
// the latest job run is in an error state.
func TestEnrichGlueJobStatus_WritesLastRun(t *testing.T) {
	jobName := "my-glue-etl-job"

	now := time.Now()
	fake := &glueFakeW{
		results: map[string][]gluetypes.JobRun{
			jobName: {
				{
					JobName:      aws.String(jobName),
					JobRunState:  gluetypes.JobRunStateError,
					CompletedOn:  &now,
					ErrorMessage: aws.String("SparkException: job failed"),
				},
			},
		},
	}

	resources := []resource.Resource{
		{ID: jobName, Name: jobName},
	}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), &awsclient.ServiceClients{Glue: fake}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[jobName]
	if !ok {
		t.Fatalf("FieldUpdates missing for glue job %q — coder must add last_run FieldUpdates to EnrichGlueJobStatus", jobName)
	}
	lastRun := fu["last_run"]
	if !strings.Contains(lastRun, "ERROR") && !strings.Contains(lastRun, "FAILED") {
		t.Errorf("glue last_run = %q, want value containing \"ERROR\" or \"FAILED\"", lastRun)
	}
}

// Test #14 — retired. The backup resource no longer has a `last_status` field;
// spec §4 (docs/resources/backup.md) routes job-state Wave-2 findings through
// the unified Status column via `status` FieldUpdates + EnrichmentFinding.
// The authoritative coverage lives in tests/unit/aws_backup_issue_enrichment_test.go.

// ─────────────────────────────────────────────────────────────────────────────
// Group 2: Pure Wave-1 column-source tests
// ─────────────────────────────────────────────────────────────────────────────

// Test #15 — EC2 instance_status field from fetcher

// ec2FetchFakeW implements EC2FetchInstancesAPI for instance_status test.
// It overrides both DescribeInstances and DescribeInstanceStatus.
type ec2FetchFakeW struct {
	awsclient.EC2API
	instances []ec2types.Instance
	statuses  []ec2types.InstanceStatus
}

func (f *ec2FetchFakeW) DescribeInstances(
	_ context.Context,
	_ *ec2.DescribeInstancesInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{Instances: f.instances},
		},
	}, nil
}

func (f *ec2FetchFakeW) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{InstanceStatuses: f.statuses}, nil
}

// TestEC2_HealthColumnReadsInstanceStatus verifies that FetchEC2InstancesPage calls
// DescribeInstanceStatus and merges the returned status into Fields["instance_status"]
// for instances where the API returns a non-empty status.
func TestEC2_HealthColumnReadsInstanceStatus(t *testing.T) {
	instanceID := "i-0a1b2c3d4e5f60001"

	fake := &ec2FetchFakeW{
		instances: []ec2types.Instance{
			{
				InstanceId:   aws.String(instanceID),
				State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
				InstanceType: ec2types.InstanceTypeT2Micro,
			},
		},
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String(instanceID),
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}

	result, err := awsclient.FetchEC2InstancesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	if r.Fields["instance_status"] == "" {
		t.Errorf("instance_status field is empty — FetchEC2InstancesPage must merge instance status from DescribeInstanceStatus")
	}
}

// Test #16 — Redshift PendingModifiedValues column path

// TestRedshift_PendingColumnPath verifies that the Redshift default view definition
// contains a column whose Path points to PendingModifiedValues (or the Key matches
// "pending_modified_values") as the attention column for pending changes.
func TestRedshift_PendingColumnPath(t *testing.T) {
	t.Skip("waiting on coder to add PendingModifiedValues column to Redshift list view — update assertion once path is known")
}

// ─────────────────────────────────────────────────────────────────────────────
// Group 3: Fetcher-computed Wave-1 field additions
// ─────────────────────────────────────────────────────────────────────────────

// Test #17 — rtb: blackhole_count field

// rtbFake implements EC2DescribeRouteTablesAPI for blackhole test.
type rtbFake struct {
	awsclient.EC2DescribeRouteTablesAPI
	tables []ec2types.RouteTable
}

func (f *rtbFake) DescribeRouteTables(
	_ context.Context,
	_ *ec2.DescribeRouteTablesInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{RouteTables: f.tables}, nil
}

// TestFetchRTB_WritesBlackholeCount verifies that FetchRouteTablesPage computes
// Fields["blackhole_routes_count"] (or "blackhole_count") as "1" for a route table
// with one route in blackhole state.
func TestFetchRTB_WritesBlackholeCount(t *testing.T) {
	rtbID := "rtb-00blackhole01"

	fake := &rtbFake{
		tables: []ec2types.RouteTable{
			{
				RouteTableId: aws.String(rtbID),
				Routes: []ec2types.Route{
					{
						DestinationCidrBlock: aws.String("10.0.0.0/8"),
						State:                ec2types.RouteStateActive,
					},
					{
						DestinationCidrBlock: aws.String("172.16.0.0/12"),
						State:                ec2types.RouteStateBlackhole,
						NatGatewayId:         aws.String("nat-deleted"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchRouteTablesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	// Try both possible field key names.
	count := r.Fields["blackhole_routes_count"]
	if count == "" {
		count = r.Fields["blackhole_count"]
	}
	if count != "1" {
		t.Errorf("blackhole count field = %q (checked blackhole_routes_count and blackhole_count), want %q", count, "1")
	}
}

// Test #18 — eip: status field

// eipFake implements EC2DescribeAddressesAPI for eip status test.
type eipFake struct {
	awsclient.EC2DescribeAddressesAPI
	addresses []ec2types.Address
}

func (f *eipFake) DescribeAddresses(
	_ context.Context,
	_ *ec2.DescribeAddressesInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{Addresses: f.addresses}, nil
}

// TestFetchEIP_WritesStatus verifies that FetchElasticIPs sets Fields["status"] == "UNATTACHED"
// for an Elastic IP with no AssociationId, InstanceId, or NetworkInterfaceId.
func TestFetchEIP_WritesStatus(t *testing.T) {
	fake := &eipFake{
		addresses: []ec2types.Address{
			{
				AllocationId: aws.String("eipalloc-00aa11bb22"),
				PublicIp:     aws.String("198.51.100.42"),
				Domain:       ec2types.DomainTypeVpc,
				// No AssociationId, InstanceId, NetworkInterfaceId → UNATTACHED
			},
		},
	}

	resources, err := awsclient.FetchElasticIPs(context.Background(), fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	status := r.Fields["status"]
	if status != "UNATTACHED" {
		t.Errorf("eip status = %q, want %q — coder must compute status field in FetchElasticIPs", status, "UNATTACHED")
	}
}

// Test #20 — dbc: multi-warning §4 status phrase (formerly cis_flags)

// docdbFake implements DocDBDescribeDBClustersAPI for dbc tests.
type docdbFake struct {
	awsclient.DocDBDescribeDBClustersAPI
	clusters []docdbtypes.DBCluster
}

func (f *docdbFake) DescribeDBClusters(
	_ context.Context,
	_ *docdb.DescribeDBClustersInput,
	_ ...func(*docdb.Options),
) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{DBClusters: f.clusters}, nil
}

// TestFetchDBC_MultiWarningStatusPhrase verifies that FetchDocDBClustersPage
// collapses a cluster with multiple Wave-1 warnings into a single §4 status
// phrase with a (+N) suffix — the post-refactor replacement for the legacy
// `cis_flags` column (removed per universal rule U10, no jargon columns).
// See docs/resources/dbc.md §4.
func TestFetchDBC_MultiWarningStatusPhrase(t *testing.T) {
	clusterID := "docdb-multi-warn-cluster"
	fake := &docdbFake{
		clusters: []docdbtypes.DBCluster{
			{
				DBClusterIdentifier:   aws.String(clusterID),
				Status:                aws.String("available"),
				StorageEncrypted:      aws.Bool(false),
				BackupRetentionPeriod: aws.Int32(0),
				DeletionProtection:    aws.Bool(false),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{IsClusterWriter: aws.Bool(true)},
				},
			},
		},
	}

	result, err := awsclient.FetchDocDBClustersPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]

	// Jargon column gone — no cis_flags anywhere.
	if _, ok := r.Fields["cis_flags"]; ok {
		t.Error("Fields[\"cis_flags\"] unexpectedly present — universal rule U10 requires its removal")
	}

	// §4 top phrase for delete-protection off + unencrypted + no backups:
	// precedence order = delete-protection first, then not-encrypted, then
	// no-automated-backups — so the top is "delete-protection off" with (+2).
	const want = "delete-protection off (+2)"
	// Per Phase-03 PR-03e: fetcher no longer writes Resource.Status; the
	// display phrase lives in Fields["status"], findings carry severity.
	if r.Fields["status"] != want {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], want)
	}

	// Findings enumerate every active warning (rule 7 — detail view renders each individually).
	wantFindings := []string{"delete-protection off", "not encrypted at rest", "no automated backups"}
	if len(r.Findings) != len(wantFindings) {
		t.Fatalf("Findings count = %d, want %d (%v)", len(r.Findings), len(wantFindings), r.Findings)
	}
	for i, phrase := range wantFindings {
		if r.Findings[i].Phrase != phrase {
			t.Errorf("Findings[%d].Phrase = %q, want %q", i, r.Findings[i].Phrase, phrase)
		}
		if r.Findings[i].Source != "wave1" {
			t.Errorf("Findings[%d].Source = %q, want %q", i, r.Findings[i].Source, "wave1")
		}
	}
}

// Test #21 — secrets: status=OVERDUE field

// secretsFake implements SecretsManagerListSecretsAPI for status test.
type secretsFake struct {
	awsclient.SecretsManagerListSecretsAPI
	secrets []secretsmanagertypes.SecretListEntry
}

func (f *secretsFake) ListSecrets(
	_ context.Context,
	_ *secretsmanager.ListSecretsInput,
	_ ...func(*secretsmanager.Options),
) (*secretsmanager.ListSecretsOutput, error) {
	return &secretsmanager.ListSecretsOutput{SecretList: f.secrets}, nil
}

// TestFetchSecrets_WritesStatus verifies that FetchSecretsPage sets
// Fields["status"] == "OVERDUE" for a secret whose NextRotationDate is in the past
// and RotationEnabled is true.
func TestFetchSecrets_WritesStatus(t *testing.T) {
	pastDate := time.Now().Add(-48 * time.Hour)
	rotEnabled := true
	fake := &secretsFake{
		secrets: []secretsmanagertypes.SecretListEntry{
			{
				Name:             aws.String("my-overdue-secret"),
				RotationEnabled:  &rotEnabled,
				NextRotationDate: &pastDate,
			},
		},
	}

	result, err := awsclient.FetchSecretsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	status := r.Fields["status"]
	if status != "OVERDUE" {
		t.Errorf("secret status = %q, want %q — coder must compute status=OVERDUE when NextRotationDate is past and RotationEnabled=true", status, "OVERDUE")
	}
}

// Test #22 — ssm: risk=STALE field

// ssmFake implements SSMDescribeParametersAPI for risk test.
type ssmFake struct {
	awsclient.SSMDescribeParametersAPI
	params []ssmtypes.ParameterMetadata
}

func (f *ssmFake) DescribeParameters(
	_ context.Context,
	_ *ssm.DescribeParametersInput,
	_ ...func(*ssm.Options),
) (*ssm.DescribeParametersOutput, error) {
	return &ssm.DescribeParametersOutput{Parameters: f.params}, nil
}

// TestFetchSSM_WritesRisk verifies that FetchSSMParametersPage sets
// Fields["risk"] == "STALE" for a SecureString parameter whose LastModifiedDate
// is more than 365 days ago.
func TestFetchSSM_WritesRisk(t *testing.T) {
	staleDate := time.Now().Add(-400 * 24 * time.Hour) // 400 days ago
	fake := &ssmFake{
		params: []ssmtypes.ParameterMetadata{
			{
				Name:             aws.String("/prod/db-password"),
				Type:             ssmtypes.ParameterTypeSecureString,
				LastModifiedDate: &staleDate,
			},
		},
	}

	result, err := awsclient.FetchSSMParametersPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	risk := r.Fields["risk"]
	if risk != "STALE" {
		t.Errorf("ssm risk = %q, want %q — coder must compute risk=STALE for SecureString parameters not modified in >365 days", risk, "STALE")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Group 4: Cosmetic format fields
// ─────────────────────────────────────────────────────────────────────────────

// Test #23 — acm: days_left field

// acmListFake implements ACMListCertificatesAPI for the days_left test.
type acmListFake struct {
	awsclient.ACMListCertificatesAPI
	certs []acmtypes.CertificateSummary
}

func (f *acmListFake) ListCertificates(
	_ context.Context,
	_ *acm.ListCertificatesInput,
	_ ...func(*acm.Options),
) (*acm.ListCertificatesOutput, error) {
	return &acm.ListCertificatesOutput{CertificateSummaryList: f.certs}, nil
}

// TestFetchACM_WritesDaysLeft verifies that FetchACMCertificatesPage computes
// Fields["days_left"] matching "^[0-9]+ days$" or "expired" for a certificate
// with NotAfter set to 5 days from now.
func TestFetchACM_WritesDaysLeft(t *testing.T) {
	notAfter := time.Now().Add(5 * 24 * time.Hour)
	fake := &acmListFake{
		certs: []acmtypes.CertificateSummary{
			{
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/days-left-test"),
				DomainName:     aws.String("example.com"),
				Status:         acmtypes.CertificateStatusIssued,
				NotAfter:       &notAfter,
				InUse:          aws.Bool(true),
			},
		},
	}

	result, err := awsclient.FetchACMCertificatesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	daysLeft := r.Fields["days_left"]
	if daysLeft == "" {
		t.Errorf("days_left field is empty — coder must compute days_left in FetchACMCertificatesPage")
		return
	}
	matched, _ := regexp.MatchString(`^[0-9]+ days$|^expired$`, daysLeft)
	if !matched {
		t.Errorf("days_left = %q, want pattern \"<N> days\" or \"expired\"", daysLeft)
	}
}

// Test #24 — ami: deprecated field

// amiListFake implements EC2DescribeImagesAPI for the deprecated test.
type amiListFake struct {
	awsclient.EC2DescribeImagesAPI
	images []ec2types.Image
}

func (f *amiListFake) DescribeImages(
	_ context.Context,
	_ *ec2.DescribeImagesInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{Images: f.images}, nil
}

// TestFetchAMI_WritesDeprecated verifies that FetchAMIsPage sets a non-empty
// Fields["deprecated"] value (e.g. "yes" or "3mo ago") for an image whose
// DeprecationTime is 3 months ago.
func TestFetchAMI_WritesDeprecated(t *testing.T) {
	threeMonthsAgo := time.Now().Add(-90 * 24 * time.Hour).Format(time.RFC3339)
	fake := &amiListFake{
		images: []ec2types.Image{
			{
				ImageId:         aws.String("ami-deprecated001"),
				Name:            aws.String("my-old-ami"),
				State:           ec2types.ImageStateAvailable,
				DeprecationTime: aws.String(threeMonthsAgo),
				OwnerId:         aws.String("123456789012"),
			},
		},
	}

	result, err := awsclient.FetchAMIsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	deprecated := r.Fields["deprecated"]
	if deprecated == "" {
		t.Errorf("deprecated field is empty — coder must compute deprecated field in FetchAMIsPage when DeprecationTime is set")
	}
}
