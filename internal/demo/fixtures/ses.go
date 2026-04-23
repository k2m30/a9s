package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// Exported ID/ARN constants — referenced by this file, sibling fixtures, and QA tests.
const (
	// SESGraphRootIdentity is the domain identity used as the demo graph-root.
	// It resolves every §2 related-panel pivot for "ses" in the scenario harness.
	SESGraphRootIdentity = "acme-corp.com"

	// SESConfigSetName is the configuration set that wires SES events to EB/Kinesis/SNS.
	SESConfigSetName = "es-events-prod"

	// SESInboundLambdaName is the Lambda function invoked by the SES v1 receipt rule.
	SESInboundLambdaName = "acme-inbound-parser"

	// SESInboundBucketName is the S3 bucket where SES inbound receipt rules store mail.
	SESInboundBucketName = "acme-inbound-mail"

	// SESBounceTopicName is the SNS topic that receives SES bounce/complaint events.
	SESBounceTopicName = "ses-bounces"

	// SESFirehoseStreamName is the Kinesis Firehose delivery stream for SES events.
	//
	// Note: the kinesis fetcher (internal/aws/kinesis.go) lists Kinesis Data Streams,
	// not Firehose delivery streams. The SES→kinesis pivot (checkSESKinesis) collects
	// the DeliveryStreamArn directly from GetConfigurationSetEventDestinations — it does
	// NOT cross-reference the kinesis resource cache. Therefore SESFirehoseStreamARN
	// does not need a matching entry in kinesis.go; the count is non-zero as long as
	// the fake's GetConfigurationSetEventDestinations returns the ARN below.
	SESFirehoseStreamName = "ses-event-stream"

	// SESFirehoseStreamARN is the Firehose delivery stream ARN used in event destinations.
	SESFirehoseStreamARN = "arn:aws:firehose:us-east-1:123456789012:deliverystream/ses-event-stream"

	// SESEventBusARN is the default EventBridge bus ARN used as an event destination.
	// It resolves against EventBridge fixtures which have rules on the "default" bus.
	SESEventBusARN = "arn:aws:events:us-east-1:123456789012:event-bus/default"

	// sesInboundLambdaARN is the full ARN for acme-inbound-parser (not exported as const
	// because sibling lambda.go uses SESInboundLambdaName to build the ARN consistently).
	sesInboundLambdaARN = "arn:aws:lambda:us-east-1:123456789012:function:" + SESInboundLambdaName

	// sesBouncesTopicARN is the ARN for the ses-bounces SNS topic.
	sesBouncesTopicARN = "arn:aws:sns:us-east-1:123456789012:" + SESBounceTopicName

	// sesFirehoseIAMRoleARN is the IAM role used by the Firehose event destination.
	sesFirehoseIAMRoleARN = "arn:aws:iam::123456789012:role/ses-firehose-delivery-role"
)

// SESFixtures holds typed fixture data for SESv2 (identities, account-level config,
// event destinations) and SESv1 (active receipt rule set for inbound mail routing).
type SESFixtures struct {
	// Identities is the full list returned by sesv2:ListEmailIdentities.
	// All identities use the acme-corp.com domain so the r53 pivot resolves
	// against the existing NewR53Fixtures() zone "acme-corp.com.".
	Identities []sesv2types.IdentityInfo

	// GetAccountDefault is the HEALTHY account shape used by ./a9s --demo.
	// Account-level distress shapes (PROBATION/SHUTDOWN/over-quota) are constructed
	// inline in QA tests and are NOT in this fixture file.
	GetAccountDefault *sesv2.GetAccountOutput

	// GetEmailIdentityByName maps identity name → GetEmailIdentity response.
	// The graph-root "acme-corp.com" maps to ConfigurationSetName "es-events-prod";
	// other identities return nil ConfigurationSetName (no config set).
	GetEmailIdentityByName map[string]*sesv2.GetEmailIdentityOutput

	// EventDestinationsByConfigSet maps configuration set name → event destinations.
	// "es-events-prod" returns three destinations: EventBridge, Firehose, SNS.
	EventDestinationsByConfigSet map[string]*sesv2.GetConfigurationSetEventDestinationsOutput

	// ActiveReceiptRuleSet is the SES v1 active rule set used by the demo fake.
	// It contains one rule with S3Action (acme-inbound-mail) and LambdaAction
	// (acme-inbound-parser) to satisfy the lambda and s3 §2 related-panel pivots.
	ActiveReceiptRuleSet *ses.DescribeActiveReceiptRuleSetOutput
}

// NewSESFixtures constructs SESFixtures from the canonical demo data.
func NewSESFixtures() *SESFixtures {
	return &SESFixtures{
		Identities:                   buildSESIdentities(),
		GetAccountDefault:            buildSESAccountHealthy(),
		GetEmailIdentityByName:       buildSESEmailIdentityMap(),
		EventDestinationsByConfigSet: buildSESEventDestinations(),
		ActiveReceiptRuleSet:         buildSESActiveReceiptRuleSet(),
	}
}

// ---------------------------------------------------------------------------
// Identity list (§2.1)
// ---------------------------------------------------------------------------

func buildSESIdentities() []sesv2types.IdentityInfo {
	return []sesv2types.IdentityInfo{
		// FIXTURE: healthy-domain / graph-root-mailer
		// Verified domain; sending enabled; no glyph. This is also the graph-root —
		// GetEmailIdentity returns ConfigurationSetName = "es-events-prod" so every
		// §2 pivot resolves ≥ 1 in the scenario harness.
		{
			IdentityName:       aws.String(SESGraphRootIdentity),
			IdentityType:       sesv2types.IdentityTypeDomain,
			SendingEnabled:     true,
			VerificationStatus: sesv2types.VerificationStatusSuccess,
		},
		// FIXTURE: healthy-email
		// Verified transactional sender; Healthy.
		{
			IdentityName:       aws.String("noreply@acme-corp.com"),
			IdentityType:       sesv2types.IdentityTypeEmailAddress,
			SendingEnabled:     true,
			VerificationStatus: sesv2types.VerificationStatusSuccess,
		},
		// FIXTURE: warn-pending-email — covers §3.1 PENDING
		// Yellow row, S4 "pending verification".
		{
			IdentityName:       aws.String("alerts@acme-corp.com"),
			IdentityType:       sesv2types.IdentityTypeEmailAddress,
			SendingEnabled:     true,
			VerificationStatus: sesv2types.VerificationStatusPending,
		},
		// FIXTURE: broken-failed-domain — covers §3.1 FAILED
		// Hard DNS failure. Red row, S4 "verification failed".
		{
			IdentityName:       aws.String("ses-failed.acme-corp.com"),
			IdentityType:       sesv2types.IdentityTypeDomain,
			SendingEnabled:     true,
			VerificationStatus: sesv2types.VerificationStatusFailed,
		},
		// FIXTURE: broken-temp-failure-domain — covers §3.1 TEMPORARY_FAILURE
		// Red row, S4 "verify: temp failure".
		{
			IdentityName:       aws.String("temp.acme-corp.com"),
			IdentityType:       sesv2types.IdentityTypeDomain,
			SendingEnabled:     true,
			VerificationStatus: sesv2types.VerificationStatusTemporaryFailure,
		},
		// FIXTURE: broken-not-started-domain — covers §3.1 NOT_STARTED
		// Red row, S4 "verification not started".
		{
			IdentityName:       aws.String("notstarted.acme-corp.com"),
			IdentityType:       sesv2types.IdentityTypeDomain,
			SendingEnabled:     true,
			VerificationStatus: sesv2types.VerificationStatusNotStarted,
		},
		// FIXTURE: warn-sending-disabled — covers §3.1 SendingEnabled==false on verified
		// Verified but sending paused. Yellow, S4 "sending disabled".
		{
			IdentityName:       aws.String("suppressed@acme-corp.com"),
			IdentityType:       sesv2types.IdentityTypeEmailAddress,
			SendingEnabled:     false,
			VerificationStatus: sesv2types.VerificationStatusSuccess,
		},
		// FIXTURE: warn-ses-multi — U7a: multi-W1 suffix test vehicle
		// Both FAILED verification AND sending disabled. Fetcher produces
		// Status = "verification failed (+1)", Issues = ["verification failed", "sending disabled"].
		{
			IdentityName:       aws.String("broken.acme-corp.com"),
			IdentityType:       sesv2types.IdentityTypeDomain,
			SendingEnabled:     false,
			VerificationStatus: sesv2types.VerificationStatusFailed,
		},
	}
}

// ---------------------------------------------------------------------------
// Account-level GetAccount (§2.2 — demo default shape)
// ---------------------------------------------------------------------------

func buildSESAccountHealthy() *sesv2.GetAccountOutput {
	return &sesv2.GetAccountOutput{
		EnforcementStatus: aws.String("HEALTHY"),
		SendingEnabled:    true,
		SendQuota: &sesv2types.SendQuota{
			Max24HourSend:   50000.0,
			SentLast24Hours: 1200.0, // ~2.4% — well below 80% threshold
			MaxSendRate:     14.0,
		},
	}
}

// ---------------------------------------------------------------------------
// Per-identity GetEmailIdentity map (§2.1 graph-root wiring)
// ---------------------------------------------------------------------------

func buildSESEmailIdentityMap() map[string]*sesv2.GetEmailIdentityOutput {
	return map[string]*sesv2.GetEmailIdentityOutput{
		// Graph-root: returns ConfigurationSetName so the eb-rule, kinesis, and sns
		// pivots can call GetConfigurationSetEventDestinations.
		SESGraphRootIdentity: {
			ConfigurationSetName: aws.String(SESConfigSetName),
		},
		// All other identities have no configuration set configured.
	}
}

// ---------------------------------------------------------------------------
// Event destinations (§2.1 graph-root wiring)
// ---------------------------------------------------------------------------

func buildSESEventDestinations() map[string]*sesv2.GetConfigurationSetEventDestinationsOutput {
	// All SES event types that make sense to route.
	allEventTypes := []sesv2types.EventType{
		sesv2types.EventTypeSend,
		sesv2types.EventTypeDelivery,
		sesv2types.EventTypeBounce,
		sesv2types.EventTypeComplaint,
	}

	return map[string]*sesv2.GetConfigurationSetEventDestinationsOutput{
		SESConfigSetName: {
			EventDestinations: []sesv2types.EventDestination{
				// EventBridge destination → eb-rule pivot
				{
					Name:               aws.String("eb-default-bus"),
					Enabled:            true,
					MatchingEventTypes: allEventTypes,
					EventBridgeDestination: &sesv2types.EventBridgeDestination{
						EventBusArn: aws.String(SESEventBusARN),
					},
				},
				// Kinesis Firehose destination → kinesis pivot
				// Note: the kinesis fetcher lists Kinesis Data Streams, not Firehose.
				// checkSESKinesis reads DeliveryStreamArn directly from this response —
				// it does not cross-reference the kinesis resource cache. The count is
				// non-zero from this fixture alone; no entry needed in kinesis.go.
				{
					Name:               aws.String("firehose-event-stream"),
					Enabled:            true,
					MatchingEventTypes: allEventTypes,
					KinesisFirehoseDestination: &sesv2types.KinesisFirehoseDestination{
						DeliveryStreamArn: aws.String(SESFirehoseStreamARN),
						IamRoleArn:        aws.String(sesFirehoseIAMRoleARN),
					},
				},
				// SNS destination → sns pivot
				{
					Name:               aws.String("sns-bounces"),
					Enabled:            true,
					MatchingEventTypes: []sesv2types.EventType{sesv2types.EventTypeBounce, sesv2types.EventTypeComplaint},
					SnsDestination: &sesv2types.SnsDestination{
						TopicArn: aws.String(sesBouncesTopicARN),
					},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// SES v1 active receipt rule set (§2.3)
// ---------------------------------------------------------------------------

func buildSESActiveReceiptRuleSet() *ses.DescribeActiveReceiptRuleSetOutput {
	return &ses.DescribeActiveReceiptRuleSetOutput{
		Metadata: &sestypes.ReceiptRuleSetMetadata{
			Name:             aws.String("acme-inbound-prod"),
			CreatedTimestamp: aws.Time(time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)),
		},
		Rules: []sestypes.ReceiptRule{
			{
				Name:    aws.String("route-support-to-lambda-and-s3"),
				Enabled: true,
				Recipients: []string{
					"support@acme-corp.com",
					"invoices@acme-corp.com",
				},
				Actions: []sestypes.ReceiptAction{
					// S3Action → s3 pivot
					{
						S3Action: &sestypes.S3Action{
							BucketName: aws.String(SESInboundBucketName),
						},
					},
					// LambdaAction → lambda pivot
					{
						LambdaAction: &sestypes.LambdaAction{
							FunctionArn: aws.String(sesInboundLambdaARN),
						},
					},
				},
			},
		},
	}
}
