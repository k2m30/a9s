package unit_test

import (
	"context"
	"testing"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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
		RawStruct: awsclient.SQSQueueAttributesRow{
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
					Namespace: new("AWS/SQS"),
					Dimensions: []cwtypes.Dimension{
						{Name: new("QueueName"), Value: new("payment-processing")},
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
					Namespace: new("AWS/SQS"),
					Dimensions: []cwtypes.Dimension{
						{Name: new("QueueName"), Value: new("some-other-queue")},
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

// --- Lambda checker nil-clients test ---

// TestRelated_SQS_Lambda_NilClients verifies that the lambda checker returns
// Count:-1 when clients are nil (API call cannot proceed).
func TestRelated_SQS_Lambda_NilClients(t *testing.T) {
	res := sqsPaymentRes()
	checker := sqsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, nil)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkSQSEbRule — Pattern C: ListRuleNamesByTarget on queue ARN
// ---------------------------------------------------------------------------

// TestRelated_SQS_EbRule_Match verifies that a queue with a QueueArn attribute,
// and a fake EventBridge returning 3 rule names, yields Count=3.
func TestRelated_SQS_EbRule_Match(t *testing.T) {
	src := resource.Resource{
		ID:   "payment-processing",
		Name: "payment-processing",
		Fields: map[string]string{
			"queue_arn": "arn:aws:sqs:us-east-1:123456789012:payment-processing",
		},
		RawStruct: awsclient.SQSQueueAttributesRow{
			QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/payment-processing",
			QueueName: "payment-processing",
			Attributes: map[string]string{
				"QueueArn": "arn:aws:sqs:us-east-1:123456789012:payment-processing",
			},
		},
	}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeUS1{
			ruleNames: []string{"rule-order", "rule-payment", "rule-dlq"},
		},
	}
	checker := sqsCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 3 {
		t.Errorf("Count = %d, want 3", result.Count)
	}
	if len(result.ResourceIDs) != 3 {
		t.Errorf("ResourceIDs = %v, want 3 entries", result.ResourceIDs)
	}
}

// TestRelated_SQS_EbRule_Empty verifies that a queue with an empty QueueArn
// attribute returns Count=0.
func TestRelated_SQS_EbRule_Empty(t *testing.T) {
	src := resource.Resource{
		ID:   "payment-processing",
		Name: "payment-processing",
		RawStruct: awsclient.SQSQueueAttributesRow{
			QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/payment-processing",
			QueueName: "payment-processing",
			Attributes: map[string]string{
				"QueueArn": "",
			},
		},
	}
	checker := sqsCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty QueueArn)", result.Count)
	}
}

// TestRelated_SQS_EbRule_WrongRawStruct verifies that a non-SQSQueueAttributesRow
// RawStruct returns Count=0 (assertStruct fails, queueARN is empty).
func TestRelated_SQS_EbRule_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "payment-processing",
		RawStruct: "not-an-sqs-row",
	}
	checker := sqsCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	// When assertStruct fails, queueARN stays ""; empty QueueArn → Count=0.
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct, empty QueueArn fallback)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkSQSSNS — Pattern C reverse two-hop: sns-sub cache → topic ARNs
// ---------------------------------------------------------------------------

// TestRelated_SQS_SNS_Match verifies that subscriptions with protocol=sqs and
// matching endpoint resolve back to unique topic ARNs.
func TestRelated_SQS_SNS_Match(t *testing.T) {
	res := sqsPaymentRes()
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "sub-1",
				Fields: map[string]string{
					"protocol":  "sqs",
					"endpoint":  "arn:aws:sqs:us-east-1:123456789012:payment-processing",
					"topic_arn": "arn:aws:sns:us-east-1:123456789012:order-events",
				},
			},
			{
				ID: "sub-2",
				Fields: map[string]string{
					"protocol":  "sqs",
					"endpoint":  "arn:aws:sqs:us-east-1:123456789012:payment-processing",
					"topic_arn": "arn:aws:sns:us-east-1:123456789012:order-events", // same topic
				},
			},
		}},
	}

	checker := sqsCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	// Both subscriptions point to the same topic — deduplicated to 1.
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (two subs to same topic → deduplication)", result.Count)
	}
}

// TestRelated_SQS_SNS_NoMatch verifies that zero matching subscriptions yields Count=0.
func TestRelated_SQS_SNS_NoMatch(t *testing.T) {
	res := sqsPaymentRes()
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "sub-unrelated",
				Fields: map[string]string{
					"protocol":  "sqs",
					"endpoint":  "arn:aws:sqs:us-east-1:123456789012:different-queue",
					"topic_arn": "arn:aws:sns:us-east-1:123456789012:other-topic",
				},
			},
		}},
	}

	checker := sqsCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching subscriptions)", result.Count)
	}
}

// TestRelated_SQS_SNS_WrongProtocolFiltered verifies that non-sqs protocol subscriptions
// are filtered out even if they list this queue's endpoint.
func TestRelated_SQS_SNS_WrongProtocolFiltered(t *testing.T) {
	res := sqsPaymentRes()
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "sub-http",
				Fields: map[string]string{
					"protocol":  "https",
					"endpoint":  "https://payment-processing.example.com",
					"topic_arn": "arn:aws:sns:us-east-1:123456789012:order-events",
				},
			},
		}},
	}

	checker := sqsCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (https protocol must not match)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkSQSKMS — reads kms_key_id from Fields (no API call)
// ---------------------------------------------------------------------------

// TestRelated_SQS_KMS_Present verifies that a queue with a kms_key_id field
// returns Count=1 with the key ID.
func TestRelated_SQS_KMS_Present(t *testing.T) {
	res := resource.Resource{
		ID:   "payment-processing",
		Name: "payment-processing",
		Fields: map[string]string{
			"kms_key_id": "mrk-abc1234567890def",
		},
		RawStruct: awsclient.SQSQueueAttributesRow{
			QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/payment-processing",
			QueueName: "payment-processing",
			Attributes: map[string]string{
				"QueueArn": "arn:aws:sqs:us-east-1:123456789012:payment-processing",
			},
		},
	}

	checker := sqsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (kms_key_id present)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "mrk-abc1234567890def" {
		t.Errorf("ResourceIDs = %v, want [mrk-abc1234567890def]", result.ResourceIDs)
	}
}

// TestRelated_SQS_KMS_Absent verifies that a queue with no kms_key_id returns Count=0.
func TestRelated_SQS_KMS_Absent(t *testing.T) {
	res := sqsPaymentRes()
	// sqsPaymentRes has no kms_key_id in Fields.
	checker := sqsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no kms_key_id)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkSQSLambda — Pattern A: ListEventSourceMappings API call
// ---------------------------------------------------------------------------

// TestRelated_SQS_Lambda_Match verifies that a fake Lambda client returning one
// mapping yields Count=1 with the function name extracted from the ARN.
func TestRelated_SQS_Lambda_Match(t *testing.T) {
	res := sqsPaymentRes()
	clients := &awsclient.ServiceClients{
		Lambda: &fakeLambdaListESM{
			mappings: []string{"arn:aws:lambda:us-east-1:123456789012:function:process-payments"},
		},
	}

	checker := sqsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "process-payments" {
		t.Errorf("ResourceIDs = %v, want [process-payments]", result.ResourceIDs)
	}
}

// TestRelated_SQS_Lambda_Empty verifies that no event source mappings returns Count=0.
func TestRelated_SQS_Lambda_Empty(t *testing.T) {
	res := sqsPaymentRes()
	clients := &awsclient.ServiceClients{
		Lambda: &fakeLambdaListESM{mappings: nil},
	}

	checker := sqsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no mappings)", result.Count)
	}
}

// TestRelated_SQS_Lambda_NoQueueARN verifies that a queue with empty QueueArn
// returns Count=0 without calling the API.
func TestRelated_SQS_Lambda_NoQueueARN(t *testing.T) {
	res := resource.Resource{
		ID:   "payment-processing",
		Name: "payment-processing",
		RawStruct: awsclient.SQSQueueAttributesRow{
			QueueURL:   "https://sqs.us-east-1.amazonaws.com/123456789012/payment-processing",
			QueueName:  "payment-processing",
			Attributes: map[string]string{
				// QueueArn deliberately absent.
			},
		},
	}

	clients := &awsclient.ServiceClients{
		Lambda: &fakeLambdaListESM{},
	}

	checker := sqsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty QueueArn skips API call)", result.Count)
	}
}
