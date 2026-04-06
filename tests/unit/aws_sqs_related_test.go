package unit_test

import (
	"context"
	"testing"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func sqsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("sqs") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("sqs related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("sqs related checker for %s not found", target)
	return nil
}

// sqsPaymentRes is the canonical SQS test resource representing a payment
// processing queue with a known ARN in its Attributes.
func sqsPaymentRes() resource.Resource {
	return resource.Resource{
		ID:   "payment-processing",
		Name: "payment-processing",
		Fields: map[string]string{
			"queue_name": "payment-processing",
			"queue_url":  "https://sqs.us-east-1.amazonaws.com/123456789012/payment-processing",
		},
		RawStruct: aws.SQSQueueAttributesRow{
			QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/payment-processing",
			QueueName: "payment-processing",
			Attributes: map[string]string{
				"QueueArn": "arn:aws:sqs:us-east-1:123456789012:payment-processing",
			},
		},
	}
}

// --- SNS Subscription Checker Tests ---

func TestRelated_SQS_SNSSub_Match(t *testing.T) {
	res := sqsPaymentRes()
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "arn:aws:sns:us-east-1:123456789012:my-topic:sub-id",
				Fields: map[string]string{
					"protocol": "sqs",
					"endpoint": "arn:aws:sqs:us-east-1:123456789012:payment-processing",
				},
			},
		}},
	}

	checker := sqsCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_SQS_SNSSub_NoMatch(t *testing.T) {
	res := sqsPaymentRes()
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "arn:aws:sns:us-east-1:123456789012:other-topic:sub-id",
				Fields: map[string]string{
					"protocol": "sqs",
					"endpoint": "arn:aws:sqs:us-east-1:123456789012:different-queue",
				},
			},
		}},
	}

	checker := sqsCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_SQS_SNSSub_WrongProtocol(t *testing.T) {
	res := sqsPaymentRes()
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "arn:aws:sns:us-east-1:123456789012:my-topic:sub-id",
				Fields: map[string]string{
					"protocol": "email",
					"endpoint": "someone@example.com",
				},
			},
		}},
	}

	checker := sqsCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong protocol)", result.Count)
	}
}

// --- CloudWatch Alarm Checker Tests ---

func TestRelated_SQS_Alarm_Match(t *testing.T) {
	res := sqsPaymentRes()
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "sqs-depth-alarm",
				RawStruct: cwtypes.MetricAlarm{
					Namespace: strPtr("AWS/SQS"),
					Dimensions: []cwtypes.Dimension{
						{Name: strPtr("QueueName"), Value: strPtr("payment-processing")},
					},
				},
			},
		}},
	}

	checker := sqsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "sqs-depth-alarm" {
		t.Errorf("ResourceIDs = %v, want [sqs-depth-alarm]", result.ResourceIDs)
	}
}

func TestRelated_SQS_Alarm_NoMatch(t *testing.T) {
	res := sqsPaymentRes()
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "other-alarm",
				RawStruct: cwtypes.MetricAlarm{
					Namespace: strPtr("AWS/SQS"),
					Dimensions: []cwtypes.Dimension{
						{Name: strPtr("QueueName"), Value: strPtr("some-other-queue")},
					},
				},
			},
		}},
	}

	checker := sqsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- Nil Clients / Empty Cache Tests ---

func TestRelated_SQS_NilClients(t *testing.T) {
	res := sqsPaymentRes()
	emptyCache := resource.ResourceCache{}

	for _, target := range []string{"sns-sub", "alarm"} {
		checker := sqsCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, emptyCache)
		if result.Count != -1 {
			t.Errorf("target=%s: Count = %d, want -1 (nil clients, empty cache)", target, result.Count)
		}
	}
}

func TestRelated_SQS_EmptyCache(t *testing.T) {
	res := sqsPaymentRes()

	for _, target := range []string{"sns-sub", "alarm"} {
		checker := sqsCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, resource.ResourceCache{})
		if result.Count != -1 {
			t.Errorf("target=%s: Count = %d, want -1 (empty cache)", target, result.Count)
		}
	}
}

// --- Stub Checker Assertions ---

func TestRelated_SQS_LambdaStub(t *testing.T) {
	defs := resource.GetRelated("sqs")
	for _, def := range defs {
		if def.TargetType == "lambda" {
			if def.Checker != nil {
				t.Error("sqs lambda: expected nil Checker (stub)")
			}
			return
		}
	}
	t.Error("sqs lambda related def not found")
}

func TestRelated_SQS_CfnStub(t *testing.T) {
	defs := resource.GetRelated("sqs")
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Error("sqs cfn: expected nil Checker (stub)")
			}
			return
		}
	}
	t.Error("sqs cfn related def not found")
}

// --- Demo Checker Test ---

func TestRelatedDemo_SQS_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("sqs")
	if checker == nil {
		t.Fatal("no demo checker registered for sqs")
	}
	results := checker(sqsPaymentRes())
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
