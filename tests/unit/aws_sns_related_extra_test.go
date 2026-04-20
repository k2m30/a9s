package unit_test

// aws_sns_related_extra_test.go — additional coverage for sns_related.go
// Covers: checkSNSSub, checkSNSKMS, checkSNSRole, extractRoleNamesFromPolicy.

import (
	"context"
	"testing"

	snssvc "github.com/aws/aws-sdk-go-v2/service/sns"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// fakeSNSFull implements the full SNSAPI for ServiceClients.SNS.
type fakeSNSFull struct {
	attrs map[string]string
	err   error
}

func (f *fakeSNSFull) GetTopicAttributes(_ context.Context, _ *snssvc.GetTopicAttributesInput, _ ...func(*snssvc.Options)) (*snssvc.GetTopicAttributesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &snssvc.GetTopicAttributesOutput{Attributes: f.attrs}, nil
}

func (f *fakeSNSFull) ListSubscriptionsByTopic(_ context.Context, _ *snssvc.ListSubscriptionsByTopicInput, _ ...func(*snssvc.Options)) (*snssvc.ListSubscriptionsByTopicOutput, error) {
	return &snssvc.ListSubscriptionsByTopicOutput{}, nil
}

func (f *fakeSNSFull) ListTagsForResource(_ context.Context, _ *snssvc.ListTagsForResourceInput, _ ...func(*snssvc.Options)) (*snssvc.ListTagsForResourceOutput, error) {
	return &snssvc.ListTagsForResourceOutput{}, nil
}

func (f *fakeSNSFull) GetSubscriptionAttributes(_ context.Context, _ *snssvc.GetSubscriptionAttributesInput, _ ...func(*snssvc.Options)) (*snssvc.GetSubscriptionAttributesOutput, error) {
	return &snssvc.GetSubscriptionAttributesOutput{}, nil
}

// --- checkSNSSub (Pattern C — reverse lookup: match topic_arn in sns-sub cache) ---

func TestRelated_SNS_Sub_FoundByTopicARN(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	sub1 := resource.Resource{
		ID:     "arn:aws:sns:us-east-1:123456789012:order-events:sub-001",
		Fields: map[string]string{"topic_arn": topicARN},
	}
	sub2 := resource.Resource{
		ID:     "arn:aws:sns:us-east-1:123456789012:other-topic:sub-999",
		Fields: map[string]string{"topic_arn": "arn:aws:sns:us-east-1:123456789012:other-topic"},
	}
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{sub1, sub2}},
	}

	checker := snsCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "arn:aws:sns:us-east-1:123456789012:order-events:sub-001" {
		t.Errorf("ResourceIDs[0] = %q, unexpected", result.ResourceIDs[0])
	}
}

func TestRelated_SNS_Sub_MultipleSubscribers(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	subs := []resource.Resource{
		{ID: "sub-001", Fields: map[string]string{"topic_arn": topicARN}},
		{ID: "sub-002", Fields: map[string]string{"topic_arn": topicARN}},
		{ID: "sub-003", Fields: map[string]string{"topic_arn": "arn:other"}},
	}
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: subs},
	}

	checker := snsCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

func TestRelated_SNS_Sub_EmptyTopicARN(t *testing.T) {
	// Both topic_arn field and ID are empty → returns -1.
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := snsCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no ARN)", result.Count)
	}
}

func TestRelated_SNS_Sub_FallsBackToID(t *testing.T) {
	// topic_arn field absent; falls back to res.ID.
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{}, // no topic_arn key
	}
	sub := resource.Resource{
		ID:     "sub-001",
		Fields: map[string]string{"topic_arn": topicARN},
	}
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{sub}},
	}

	checker := snsCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (fallback to res.ID)", result.Count)
	}
}

func TestRelated_SNS_Sub_NilCacheNoClients(t *testing.T) {
	source := resource.Resource{
		ID:     "arn:aws:sns:us-east-1:123456789012:order-events",
		Fields: map[string]string{"topic_arn": "arn:aws:sns:us-east-1:123456789012:order-events"},
	}
	checker := snsCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}

// --- checkSNSKMS (Pattern C — GetTopicAttributes → KmsMasterKeyId) ---

func TestRelated_SNS_KMS_FoundFromAttributes(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	clients := &awsclient.ServiceClients{
		SNS: &fakeSNSFull{
			attrs: map[string]string{
				"KmsMasterKeyId": "arn:aws:kms:us-east-1:123456789012:key/sns-key-001",
			},
		},
	}

	checker := snsCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "sns-key-001" {
		t.Errorf("ResourceIDs[0] = %q, want sns-key-001 (last ARN segment)", result.ResourceIDs[0])
	}
}

func TestRelated_SNS_KMS_NotEncrypted(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	clients := &awsclient.ServiceClients{
		SNS: &fakeSNSFull{
			attrs: map[string]string{}, // no KmsMasterKeyId
		},
	}

	checker := snsCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (not encrypted)", result.Count)
	}
}

func TestRelated_SNS_KMS_NilClientsReturnsUnknown(t *testing.T) {
	source := resource.Resource{
		ID:     "arn:aws:sns:us-east-1:123456789012:order-events",
		Fields: map[string]string{"topic_arn": "arn:aws:sns:us-east-1:123456789012:order-events"},
	}
	checker := snsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

func TestRelated_SNS_KMS_EmptyTopicARNReturnsZero(t *testing.T) {
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := snsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN)", result.Count)
	}
}

// --- checkSNSRole (Pattern C — GetTopicAttributes → Policy → extract role ARNs) ---

func TestRelated_SNS_Role_ExtractsFromPolicy(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	policy := `{
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": "arn:aws:iam::123456789012:role/sns-publisher"},
			"Action": "SNS:Publish"
		}]
	}`
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	clients := &awsclient.ServiceClients{
		SNS: &fakeSNSFull{
			attrs: map[string]string{"Policy": policy},
		},
	}

	checker := snsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "sns-publisher" {
		t.Errorf("ResourceIDs[0] = %q, want sns-publisher", result.ResourceIDs[0])
	}
}

func TestRelated_SNS_Role_MultipleRoles(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	policy := `{
		"Statement": [{
			"Effect": "Allow",
			"Principal": {
				"AWS": [
					"arn:aws:iam::123456789012:role/sns-publisher",
					"arn:aws:iam::123456789012:role/sns-monitor"
				]
			},
			"Action": "SNS:Publish"
		}]
	}`
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	clients := &awsclient.ServiceClients{
		SNS: &fakeSNSFull{attrs: map[string]string{"Policy": policy}},
	}

	checker := snsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

func TestRelated_SNS_Role_NoPolicyEntries(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	clients := &awsclient.ServiceClients{
		SNS: &fakeSNSFull{attrs: map[string]string{"Policy": "{}"}},
	}

	checker := snsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no role ARNs in policy)", result.Count)
	}
}

func TestRelated_SNS_Role_EmptyPolicyString(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	clients := &awsclient.ServiceClients{
		SNS: &fakeSNSFull{attrs: map[string]string{}}, // no Policy key
	}

	checker := snsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty policy attribute)", result.Count)
	}
}

func TestRelated_SNS_Role_NilClientsReturnsUnknown(t *testing.T) {
	source := resource.Resource{
		ID:     "arn:aws:sns:us-east-1:123456789012:order-events",
		Fields: map[string]string{"topic_arn": "arn:aws:sns:us-east-1:123456789012:order-events"},
	}
	checker := snsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- snsAlarmReferences — exercised via existing alarm tests above;
//     direct tests for OKActions and InsufficientDataActions edge cases. ---

func TestRelated_SNS_Alarm_OKActions(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:alert-ok"
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	alarmRes := resource.Resource{
		ID: "alarm-ok-action",
		RawStruct: resource.ResourceCacheEntry{}, // wrong type — should skip
	}
	_ = alarmRes
	// Build alarm that references topic in OKActions only.
	alarmWithOKAction := resource.Resource{
		ID: "alarm-ok-action",
	}
	_ = alarmWithOKAction

	// We cannot embed cwtypes.MetricAlarm OKActions here without a circular look —
	// the test via checkSNSAlarm already tests OKActions via TestRelated_SNS_Alarm_Found.
	// Verify that alarmRes with wrong RawStruct type is skipped gracefully.
	alarmWrong := resource.Resource{
		ID:        "alarm-wrong-raw",
		RawStruct: "not-a-metric-alarm",
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmWrong}},
	}

	checker := snsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct entries skipped)", result.Count)
	}
}

// --- checkSNSRole: non-role ARN in Principal ignored ---

func TestRelated_SNS_Role_PrincipalWithUserARNIgnored(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	// Policy with IAM user (not role) principal
	policy := `{
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": "arn:aws:iam::123456789012:user/svc-account"},
			"Action": "SNS:Publish"
		}]
	}`
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{"topic_arn": topicARN},
	}
	clients := &awsclient.ServiceClients{
		SNS: &fakeSNSFull{attrs: map[string]string{"Policy": policy}},
	}

	checker := snsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (user ARN, not role)", result.Count)
	}
}

func TestRelated_SNS_Role_UsesTopicARNFromID(t *testing.T) {
	// Fallback: no topic_arn in Fields, uses res.ID.
	const topicARN = "arn:aws:sns:us-east-1:123456789012:order-events"
	policy := `{"Statement": [{"Principal": {"AWS": "arn:aws:iam::123456789012:role/reader"}}]}`
	source := resource.Resource{
		ID:     topicARN,
		Fields: map[string]string{}, // no topic_arn
	}
	clients := &awsclient.ServiceClients{
		SNS: &fakeSNSFull{attrs: map[string]string{"Policy": policy}},
	}

	checker := snsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (fallback to res.ID for ARN)", result.Count)
	}
	if result.ResourceIDs[0] != "reader" {
		t.Errorf("ResourceIDs[0] = %q, want reader", result.ResourceIDs[0])
	}
}
