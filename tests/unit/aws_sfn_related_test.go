package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func sfnCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("sfn") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("sfn related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("sfn related checker for %s not found", target)
	return nil
}

// sfnDescribeFake answers DescribeStateMachine with out.
type sfnDescribeFake struct {
	awsclient.SFNAPI
	out *sfn.DescribeStateMachineOutput
}

func (f *sfnDescribeFake) DescribeStateMachine(context.Context, *sfn.DescribeStateMachineInput, ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	return f.out, nil
}

func sfnLoggingClients(level sfntypes.LogLevel, groupARNs ...string) *awsclient.ServiceClients {
	lc := &sfntypes.LoggingConfiguration{Level: level}
	for _, g := range groupARNs {
		lc.Destinations = append(lc.Destinations, sfntypes.LogDestination{CloudWatchLogsLogGroup: &sfntypes.CloudWatchLogsLogGroup{LogGroupArn: aws.String(g)}})
	}
	return &awsclient.ServiceClients{SFN: &sfnDescribeFake{out: &sfn.DescribeStateMachineOutput{
		Name:                 aws.String("order-fulfillment-workflow"),
		StateMachineArn:      aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
		RoleArn:              aws.String("arn:aws:iam::123456789012:role/sfn-exec"),
		LoggingConfiguration: lc,
	}}}
}

// A state machine logs to the group its LoggingConfiguration.Destinations
// names (DescribeStateMachine), whatever the group is called.
func TestRelated_SFN_Logs_Found(t *testing.T) {
	const group = "/app/orders/sfn-history"
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: group, Fields: map[string]string{}},
			{ID: "/aws/vendedlogs/states/order-fulfillment-workflow", Fields: map[string]string{}},
		}},
	}
	clients := sfnLoggingClients(sfntypes.LogLevelError, "arn:aws:logs:us-east-1:123456789012:log-group:"+group+":*")

	checker := sfnCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, sfnSrcResource(), cache)

	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != group {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), group)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

// At level OFF a state machine logs nothing, even with a destination left
// configured.
func TestRelated_SFN_Logs_NoMatch(t *testing.T) {
	const group = "/aws/vendedlogs/states/order-fulfillment-workflow"
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: group, Fields: map[string]string{}}}},
	}
	clients := sfnLoggingClients(sfntypes.LogLevelOff, "arn:aws:logs:us-east-1:123456789012:log-group:"+group+":*")

	checker := sfnCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, sfnSrcResource(), cache)

	if result.State() != domain.RelatedResolved || result.Count() != 0 {
		t.Errorf("state = %v, Count = %d, want a resolved 0", result.State(), result.Count())
	}
}

func TestRelated_SFN_Logs_EmptyID(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/vendedlogs/states/order-fulfillment-workflow",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	src := resource.Resource{ID: ""}
	checker := sfnCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count())
	}
}

// A row without its state machine ARN cannot be described
// (DescribeStateMachine takes the ARN), so every pivot that reads the
// description answers unknown, never a proven 0.
func TestRelated_SFN_Logs_CacheMissNoClients(t *testing.T) {
	src := resource.Resource{ID: "order-fulfillment-workflow", Fields: map[string]string{}}
	clients := sfnLoggingClients(sfntypes.LogLevelAll, "arn:aws:logs:us-east-1:123456789012:log-group:/app/orders/sfn-history:*")
	for _, target := range []string{"logs", "role", "kms", "lambda"} {
		result := sfnCheckerByTarget(t, target)(context.Background(), clients, src, resource.ResourceCache{})
		if result.State() != domain.RelatedUnknown {
			t.Errorf("sfn -> %s on a row without its ARN: state = %v, Count = %d, want unknown", target, result.State(), result.Count())
		}
	}
}

func sfnSrcResource() resource.Resource {
	return resource.Resource{
		ID: "order-fulfillment-workflow",
		Fields: map[string]string{
			"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
		},
	}
}

func TestRelated_SFN_Alarm_Found(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "sfn-failures",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			Namespace: aws.String("AWS/States"),
			AlarmName: aws.String("sfn-failures"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("StateMachineArn"),
					Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, sfnSrcResource(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "sfn-failures" {
		t.Errorf("ResourceIDs = %v, want [sfn-failures]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_SFN_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "sfn-other-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("sfn-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("StateMachineArn"),
					Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:other-workflow"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, sfnSrcResource(), cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_SFN_Alarm_NoDimensions(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "sfn-nodim-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:  aws.String("sfn-nodim-alarm"),
			Dimensions: []cwtypes.Dimension{},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, sfnSrcResource(), cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no dimensions)", result.Count())
	}
}

func TestRelated_SFN_Alarm_EmptyARN(t *testing.T) {
	src := resource.Resource{
		ID:     "order-fulfillment-workflow",
		Fields: map[string]string{"arn": ""},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "sfn-failures",
				RawStruct: cwtypes.MetricAlarm{
					AlarmName: aws.String("sfn-failures"),
					Dimensions: []cwtypes.Dimension{
						{
							Name:  aws.String("StateMachineArn"),
							Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
						},
					},
				},
			},
		}},
	}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown — empty arn field)", result.Count())
	}
}

func TestRelated_SFN_Alarm_CacheMissNoClients(t *testing.T) {
	cache := resource.ResourceCache{}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, sfnSrcResource(), cache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count())
	}
}

// StateMachineListItem carries no RoleArn; sfn→role needs DescribeStateMachine.

func TestRelated_SFN_Role_EmptyARN(t *testing.T) {
	source := resource.Resource{
		ID:   "order-fulfillment-workflow",
		Name: "order-fulfillment-workflow",
	}
	checker := sfnCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN short-circuit)", result.Count())
	}
	if result.TargetType() != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "role")
	}
}

func TestRelated_SFN_Role_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "order-fulfillment-workflow",
		Name: "order-fulfillment-workflow",
		Fields: map[string]string{
			"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
		},
	}
	checker := sfnCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients — describe unavailable)", result.Count())
	}
}

func TestRelated_SFN_EbRule_Match(t *testing.T) {
	src := resource.Resource{
		ID:   "order-fulfillment-workflow",
		Name: "order-fulfillment-workflow",
		Fields: map[string]string{
			"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
		},
	}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeAPI{
			RuleNames: []string{"rule-start", "rule-monitor", "rule-retry"},
		},
	}
	checker := sfnCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 3 {
		t.Errorf("Count = %d, want 3", result.Count())
	}
	if len(result.ResourceIDs()) != 3 {
		t.Errorf("ResourceIDs = %v, want 3 entries", result.ResourceIDs())
	}
}

func TestRelated_SFN_EbRule_Empty(t *testing.T) {
	src := resource.Resource{
		ID:     "order-fulfillment-workflow",
		Name:   "order-fulfillment-workflow",
		Fields: map[string]string{},
	}
	checker := sfnCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN field)", result.Count())
	}
}

func TestRelated_SFN_EbRule_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:   "order-fulfillment-workflow",
		Name: "order-fulfillment-workflow",
		Fields: map[string]string{
			"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
		},
		RawStruct: "not-a-state-machine",
	}
	checker := sfnCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}
