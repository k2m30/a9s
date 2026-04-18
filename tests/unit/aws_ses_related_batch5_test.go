// aws_ses_related_batch5_test.go contains TDD Red tests for the SES related-panel
// checkers T054–T058 (ses→eb-rule, ses→kinesis, ses→lambda, ses→s3, ses→sns).
// Tests are written before the coder replaces the stubs in stubs_related.go
// with real implementations — initial failures on Match/Empty cases are expected.
// ses→lambda and ses→s3 require SES v1 SDK which is not yet in go.mod; only the
// WrongRawStruct test is written for those pairs until SES v1 is added.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sesCheckerByTarget is defined in aws_ses_related_test.go.

// sesSrcIdentity returns a canonical SES domain identity resource for batch-5 tests.
func sesSrcIdentity(identityName string) resource.Resource {
	return resource.Resource{
		ID:   identityName,
		Name: identityName,
		Fields: map[string]string{
			"identity_name": identityName,
			"identity_type": "DOMAIN",
		},
		RawStruct: sesv2types.IdentityInfo{
			IdentityName: aws.String(identityName),
			IdentityType: sesv2types.IdentityTypeDomain,
		},
	}
}

// ---------------------------------------------------------------------------
// T054 — checkSESEbRule (forward: SESv2 GetConfigurationSetEventDestinations →
//         EventBridgeDestination entries)
// ---------------------------------------------------------------------------

// TestRelated_SES_EbRule_MatchTwoDestinations verifies that checkSESEbRule counts
// EventBridgeDestination entries from GetConfigurationSetEventDestinations.
func TestRelated_SES_EbRule_MatchTwoDestinations(t *testing.T) {
	identityName := "acme-corp.com"
	configSetName := "acme-transactional-config"

	fakeSESv2 := newFakeSESv2WithEventDestinations(identityName, configSetName, []sesv2types.EventDestination{
		{
			Name:    aws.String("eb-dest-1"),
			Enabled: true,
			EventBridgeDestination: &sesv2types.EventBridgeDestination{
				EventBusArn: aws.String("arn:aws:events:us-east-1:123456789012:event-bus/default"),
			},
		},
		{
			Name:    aws.String("eb-dest-2"),
			Enabled: true,
			EventBridgeDestination: &sesv2types.EventBridgeDestination{
				EventBusArn: aws.String("arn:aws:events:us-east-1:123456789012:event-bus/custom-bus"),
			},
		},
		// This destination is Kinesis — should not be counted by eb-rule checker.
		{
			Name:    aws.String("kinesis-dest"),
			Enabled: true,
			KinesisFirehoseDestination: &sesv2types.KinesisFirehoseDestination{
				DeliveryStreamArn: aws.String("arn:aws:firehose:us-east-1:123456789012:deliverystream/acme-stream"),
				IamRoleArn:        aws.String("arn:aws:iam::123456789012:role/ses-firehose-role"),
			},
		},
	})
	clients := &awsclient.ServiceClients{
		SESv2: fakeSESv2,
	}

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, sesSrcIdentity(identityName), resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (two EventBridgeDestinations)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_EbRule_NoDestinations verifies that checkSESEbRule returns
// Count=0 when GetConfigurationSetEventDestinations returns no EventBridgeDestination.
func TestRelated_SES_EbRule_NoDestinations(t *testing.T) {
	identityName := "acme-corp.com"

	fakeSESv2 := newFakeSESv2Empty(identityName)
	clients := &awsclient.ServiceClients{
		SESv2: fakeSESv2,
	}

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, sesSrcIdentity(identityName), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no event destinations)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_EbRule_WrongRawStruct verifies that checkSESEbRule returns
// Count=-1 when RawStruct is not sesv2types.IdentityInfo.
func TestRelated_SES_EbRule_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "acme-corp.com",
		RawStruct: "not-an-identity-info",
	}

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T055 — checkSESKinesis (forward: SESv2 GetConfigurationSetEventDestinations →
//         KinesisFirehoseDestination.DeliveryStreamArn)
// ---------------------------------------------------------------------------

// TestRelated_SES_Kinesis_MatchOneDestination verifies that checkSESKinesis counts
// KinesisFirehoseDestination entries from GetConfigurationSetEventDestinations.
func TestRelated_SES_Kinesis_MatchOneDestination(t *testing.T) {
	identityName := "acme-corp.com"
	configSetName := "acme-transactional-config"
	streamARN := "arn:aws:firehose:us-east-1:123456789012:deliverystream/acme-stream"

	fakeSESv2 := newFakeSESv2WithEventDestinations(identityName, configSetName, []sesv2types.EventDestination{
		{
			Name:    aws.String("kinesis-dest"),
			Enabled: true,
			KinesisFirehoseDestination: &sesv2types.KinesisFirehoseDestination{
				DeliveryStreamArn: aws.String(streamARN),
				IamRoleArn:        aws.String("arn:aws:iam::123456789012:role/ses-firehose-role"),
			},
		},
		// EventBridge destination — should not be counted by kinesis checker.
		{
			Name:    aws.String("eb-dest"),
			Enabled: true,
			EventBridgeDestination: &sesv2types.EventBridgeDestination{
				EventBusArn: aws.String("arn:aws:events:us-east-1:123456789012:event-bus/default"),
			},
		},
	})
	clients := &awsclient.ServiceClients{
		SESv2: fakeSESv2,
	}

	checker := sesCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, sesSrcIdentity(identityName), resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (one KinesisFirehoseDestination)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_Kinesis_NoDestinations verifies that checkSESKinesis returns
// Count=0 when no KinesisFirehoseDestination is present.
func TestRelated_SES_Kinesis_NoDestinations(t *testing.T) {
	identityName := "acme-corp.com"

	fakeSESv2 := newFakeSESv2Empty(identityName)
	clients := &awsclient.ServiceClients{
		SESv2: fakeSESv2,
	}

	checker := sesCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, sesSrcIdentity(identityName), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no Kinesis Firehose destinations)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_Kinesis_WrongRawStruct verifies that checkSESKinesis returns
// Count=-1 when RawStruct is not sesv2types.IdentityInfo.
func TestRelated_SES_Kinesis_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "acme-corp.com",
		RawStruct: 42,
	}

	checker := sesCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T056 — checkSESLambda (forward: ses:DescribeActiveReceiptRuleSet →
//         Rules[].Actions[].LambdaAction.FunctionArn)
// NOTE: Full Match/Empty tests require SES v1 SDK (not yet in go.mod).
// Only the WrongRawStruct test is written here — remaining tests to be added
// by QA after the coder adds SES v1 SDK + SESDescribeActiveReceiptRuleSetAPI.
// ---------------------------------------------------------------------------

// TestRelated_SES_Lambda_WrongRawStruct verifies that checkSESLambda returns
// Count=-1 when RawStruct is not sesv2types.IdentityInfo.
func TestRelated_SES_Lambda_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "acme-corp.com",
		RawStruct: "not-an-identity-info",
	}

	checker := sesCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T057 — checkSESS3 (forward: ses:DescribeActiveReceiptRuleSet →
//         Rules[].Actions[].S3Action.BucketName)
// NOTE: Full Match/Empty tests require SES v1 SDK (not yet in go.mod).
// Only the WrongRawStruct test is written here — remaining tests to be added
// by QA after the coder adds SES v1 SDK + SESDescribeActiveReceiptRuleSetAPI.
// ---------------------------------------------------------------------------

// TestRelated_SES_S3_WrongRawStruct verifies that checkSESS3 returns Count=-1
// when RawStruct is not sesv2types.IdentityInfo.
func TestRelated_SES_S3_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "acme-corp.com",
		RawStruct: struct{}{},
	}

	checker := sesCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T058 — checkSESSns (forward: SESv2 GetConfigurationSetEventDestinations →
//         SnsDestination.TopicArn, + ses v1 GetIdentityNotificationAttributes)
// The SESv2 config-set path is tested here. SES v1 notification attributes
// tests will be added after SES v1 SDK is in go.mod.
// ---------------------------------------------------------------------------

// TestRelated_SES_Sns_MatchFromConfigSetDestination verifies that checkSESSns
// counts SnsDestination entries from GetConfigurationSetEventDestinations.
func TestRelated_SES_Sns_MatchFromConfigSetDestination(t *testing.T) {
	identityName := "acme-corp.com"
	configSetName := "acme-transactional-config"
	topicARN := "arn:aws:sns:us-east-1:123456789012:acme-ses-notifications"

	fakeSESv2 := newFakeSESv2WithEventDestinations(identityName, configSetName, []sesv2types.EventDestination{
		{
			Name:    aws.String("sns-dest"),
			Enabled: true,
			SnsDestination: &sesv2types.SnsDestination{
				TopicArn: aws.String(topicARN),
			},
		},
	})
	clients := &awsclient.ServiceClients{
		SESv2: fakeSESv2,
	}

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, sesSrcIdentity(identityName), resource.ResourceCache{})

	if result.Count < 1 {
		t.Errorf("Count = %d, want >= 1 (SnsDestination in config set)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_Sns_NoDestinations verifies that checkSESSns returns Count=0
// when no SNS-related destinations are present and the identity has no
// notification attributes.
func TestRelated_SES_Sns_NoDestinations(t *testing.T) {
	identityName := "acme-corp.com"

	fakeSESv2 := newFakeSESv2Empty(identityName)
	clients := &awsclient.ServiceClients{
		SESv2: fakeSESv2,
	}

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, sesSrcIdentity(identityName), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SNS destinations)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_Sns_WrongRawStruct verifies that checkSESSns returns Count=-1
// when RawStruct is not sesv2types.IdentityInfo.
func TestRelated_SES_Sns_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "acme-corp.com",
		RawStruct: []byte("wrong"),
	}

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}
