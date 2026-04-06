package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func snsSubCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("sns-sub") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("sns-sub related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("sns-sub related checker for %s not found", target)
	return nil
}

// --- SNS Topic Checker Tests ---

func TestRelated_SNSSub_Topic_Match(t *testing.T) {
	topicARN := "arn:aws:sns:us-east-1:123456789012:my-topic"
	res := resource.Resource{
		ID: "arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4",
		Fields: map[string]string{
			"topic_arn": topicARN,
			"protocol":  "email",
			"endpoint":  "user@example.com",
		},
	}
	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: topicARN, Fields: map[string]string{"topic_arn": topicARN}},
		}},
	}

	checker := snsSubCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != topicARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, topicARN)
	}
}

func TestRelated_SNSSub_Topic_NoMatch(t *testing.T) {
	res := resource.Resource{
		ID: "arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4",
		Fields: map[string]string{
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:my-topic",
			"protocol":  "email",
			"endpoint":  "user@example.com",
		},
	}
	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "arn:aws:sns:us-east-1:123456789012:other-topic"},
		}},
	}

	checker := snsSubCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- Lambda Checker Tests ---

func TestRelated_SNSSub_Lambda_Match(t *testing.T) {
	lambdaARN := "arn:aws:lambda:us-east-1:123456789012:function:my-function"
	res := resource.Resource{
		ID: "arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4",
		Fields: map[string]string{
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:my-topic",
			"protocol":  "lambda",
			"endpoint":  lambdaARN,
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: lambdaARN},
		}},
	}

	checker := snsSubCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != lambdaARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, lambdaARN)
	}
}

func TestRelated_SNSSub_Lambda_WrongProtocol(t *testing.T) {
	res := resource.Resource{
		ID: "arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4",
		Fields: map[string]string{
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:my-topic",
			"protocol":  "sqs",
			"endpoint":  "arn:aws:sqs:us-east-1:123456789012:my-queue",
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "arn:aws:lambda:us-east-1:123456789012:function:my-function"},
		}},
	}

	checker := snsSubCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong protocol)", result.Count)
	}
}

// --- SQS Checker Tests ---

func TestRelated_SNSSub_SQS_Match(t *testing.T) {
	// The SQS checker extracts the queue name from the endpoint ARN (last ":" segment).
	// The cache entry must match by ID or Name against that extracted queue name.
	queueName := "my-queue"
	sqsARN := "arn:aws:sqs:us-east-1:123456789012:" + queueName
	res := resource.Resource{
		ID: "arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4",
		Fields: map[string]string{
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:my-topic",
			"protocol":  "sqs",
			"endpoint":  sqsARN,
		},
	}
	// The checker parses queueName from the endpoint ARN and matches sqsRes.ID == queueName.
	cache := resource.ResourceCache{
		"sqs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: queueName},
		}},
	}

	checker := snsSubCheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != queueName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, queueName)
	}
}

func TestRelated_SNSSub_SQS_WrongProtocol(t *testing.T) {
	res := resource.Resource{
		ID: "arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4",
		Fields: map[string]string{
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:my-topic",
			"protocol":  "lambda",
			"endpoint":  "arn:aws:lambda:us-east-1:123456789012:function:my-function",
		},
	}
	cache := resource.ResourceCache{
		"sqs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "arn:aws:sqs:us-east-1:123456789012:my-queue"},
		}},
	}

	checker := snsSubCheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong protocol)", result.Count)
	}
}

// --- Nil Clients / Empty Cache Tests ---

// snsSubResForTarget returns a resource whose protocol field is appropriate for
// exercising the cache-miss path of the given target checker.
// The sns checker looks up by topic_arn regardless of protocol.
// The lambda checker requires protocol=lambda to proceed to the cache lookup.
// The sqs checker requires protocol=sqs to proceed to the cache lookup.
func snsSubResForTarget(target string) resource.Resource {
	protocol := "email"
	endpoint := "user@example.com"
	switch target {
	case "lambda":
		protocol = "lambda"
		endpoint = "arn:aws:lambda:us-east-1:123456789012:function:my-function"
	case "sqs":
		protocol = "sqs"
		endpoint = "arn:aws:sqs:us-east-1:123456789012:my-queue"
	}
	return resource.Resource{
		ID: "arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4",
		Fields: map[string]string{
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:my-topic",
			"protocol":  protocol,
			"endpoint":  endpoint,
		},
	}
}

func TestRelated_SNSSub_NilClients(t *testing.T) {
	emptyCache := resource.ResourceCache{}

	for _, target := range []string{"sns", "lambda", "sqs"} {
		checker := snsSubCheckerByTarget(t, target)
		res := snsSubResForTarget(target)
		result := checker(context.Background(), nil, res, emptyCache)
		if result.Count != -1 {
			t.Errorf("target=%s: Count = %d, want -1 (nil clients, empty cache)", target, result.Count)
		}
	}
}

func TestRelated_SNSSub_EmptyCache(t *testing.T) {
	for _, target := range []string{"sns", "lambda", "sqs"} {
		checker := snsSubCheckerByTarget(t, target)
		res := snsSubResForTarget(target)
		result := checker(context.Background(), nil, res, resource.ResourceCache{})
		if result.Count != -1 {
			t.Errorf("target=%s: Count = %d, want -1 (empty cache)", target, result.Count)
		}
	}
}

// --- Navigable Field Registration ---

func TestNavigableFields_SNSSub(t *testing.T) {
	expected := map[string]string{
		"TopicArn": "sns",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigable("sns-sub", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for sns-sub", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

// --- Demo Checker Test ---

func TestRelatedDemo_SNSSub_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("sns-sub")
	if checker == nil {
		t.Fatal("no demo checker registered for sns-sub")
	}

	src := resource.Resource{
		ID: "arn:aws:sns:us-east-1:123456789012:alarm-notifications:b2c3d4e5-f6a7-8901-bcde-f12345678901",
		Fields: map[string]string{
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:alarm-notifications",
			"protocol":  "lambda",
			"endpoint":  "arn:aws:lambda:us-east-1:123456789012:function:cloudwatch-slack-notifier",
		},
	}
	results := checker(src)
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
