// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// SNSPublicPolicy names the one demo topic whose access policy grants
// publish or subscribe to every principal (sns.public-policy), and SNSNoKMS
// the one topic with no KmsMasterKeyId (sns.no-kms). Every other topic
// carries an account-scoped policy and a KMS key.
// The sns rows are keyed by topic ARN, which is what the list renders and
// what the witness gate compares against.
const (
	SNSPublicPolicy = SNSPublicPolicyARN
	SNSNoKMS        = SNSNoKMSARN
)

const (
	snsPublicPolicyTopicName = "sns-public-policy"
	snsNoKMSTopicName        = "sns-unencrypted"

	SNSPublicPolicyARN = "arn:aws:sns:us-east-1:123456789012:" + snsPublicPolicyTopicName
	SNSNoKMSARN        = "arn:aws:sns:us-east-1:123456789012:" + snsNoKMSTopicName
)

// SNSDemoKMSKeyID is the at-rest key every demo topic but SNSNoKMS carries.
const SNSDemoKMSKeyID = "a1b2c3d4-5678-90ab-cdef-111111111111"

// SNSSubPlainHTTP is the subscription ARN of the one demo sns-sub row
// delivering over unencrypted HTTP (sns-sub.plain-http). Every other
// subscription uses https, sqs, lambda or email.
const SNSSubPlainHTTP = SNSPublicPolicyARN + ":9f8e7d6c-5b4a-3210-9876-543210fedcba"

// SNSFixtures holds typed fixture data for SNS.
type SNSFixtures struct {
	Topics        []snstypes.Topic
	Subscriptions []snstypes.Subscription
	// SubscriptionsByTopic maps topic ARN to its subscriptions.
	SubscriptionsByTopic map[string][]snstypes.Subscription
	// TopicAttributes maps topic ARN to its GetTopicAttributes response —
	// backs the sns:kms and sns:role related-panel pivots (checkSNSKMS /
	// checkSNSRole).
	TopicAttributes map[string]map[string]string
}

// NewSNSFixtures constructs SNSFixtures from the canonical demo data.
var sharedSNSFixtures = sync.OnceValue(func() *SNSFixtures {
	topics := []snstypes.Topic{
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications")},
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:order-events")},
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications")},
		// S3 healthy-bucket event notifications topic (checkS3SNS pivot).
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:" + S3EventsTopicName)},
		// Redis prod ops pager topic — required for redis→sns related-panel pivot.
		// The prod-redis-sessions member cluster NotificationConfiguration.TopicArn
		// points here so checkRedisSNS resolves a non-zero count for the demo showroom.
		{TopicArn: aws.String(ProdRedisSNSTopicARN)},
		// SES bounce/complaint notifications topic (checkSESSns pivot).
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:" + SESBounceTopicName)},
		// Backup vault alert topic (checkBackupSNS pivot).
		// GetBackupVaultNotifications("acme-prod-vault").SNSTopicArn points here.
		// checkBackupSNS resolves this ARN against sns resource cache by name (last ":" segment).
		{TopicArn: aws.String(BackupAlertsSNSTopicARN)},
		// Issue: zero subscribers → "~" (orphan topic). Required for
		// EnrichSNSSubscriptions's Wave-2 issue check. Deliberately has no
		// SubscriptionsByTopic entry so ListSubscriptionsByTopic returns empty.
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:staging-deploy-alerts")},
		// Issue: every subscriber is SubscriptionArn=="PendingConfirmation" → "~"
		// (sns.all-pending-confirmation). Required for EnrichSNSSubscriptions's
		// Wave-2 all-pending check.
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:webhook-integration-pending")},
		// SNSPublicPolicy: the only topic whose access policy names a
		// wildcard principal. Also the topic SNSSubPlainHTTP hangs off.
		{TopicArn: aws.String(SNSPublicPolicyARN)},
		// SNSNoKMS: the only topic served without a KmsMasterKeyId.
		{TopicArn: aws.String(SNSNoKMSARN)},
	}

	subscriptions := []snstypes.Subscription{
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
			Protocol:        aws.String("email"),
			Endpoint:        aws.String("oncall@acme-corp.com"),
			SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications:a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
			Owner:           aws.String("123456789012"),
		},
		// Issue: SubscriptionArn=="PendingConfirmation" → Warning (never confirmed)
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:order-events"),
			Protocol:        aws.String("email"),
			Endpoint:        aws.String("pending-recipient@partner.example.com"),
			SubscriptionArn: aws.String("PendingConfirmation"),
			Owner:           aws.String("123456789012"),
		},
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
			Protocol:        aws.String("lambda"),
			Endpoint:        aws.String("arn:aws:lambda:us-east-1:123456789012:function:cloudwatch-slack-notifier"),
			SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications:b2c3d4e5-f6a7-8901-bcde-f12345678901"),
			Owner:           aws.String("123456789012"),
		},
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:order-events"),
			Protocol:        aws.String("sqs"),
			Endpoint:        aws.String("arn:aws:sqs:us-east-1:123456789012:order-processing-queue"),
			SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012"),
			Owner:           aws.String("123456789012"),
		},
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications"),
			Protocol:        aws.String("https"),
			Endpoint:        aws.String("https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXX"),
			SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications:d4e5f6a7-b8c9-0123-def0-234567890123"),
			Owner:           aws.String("123456789012"),
		},
		// Issue: SubscriptionArn=="Deleted" → Dim (subscription torn down but
		// still enumerable via ListSubscriptions for a retention window).
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:order-events"),
			Protocol:        aws.String("email"),
			Endpoint:        aws.String("former-partner@decommissioned.example.com"),
			SubscriptionArn: aws.String("Deleted"),
			Owner:           aws.String("123456789012"),
		},
	}

	// SNSSubPlainHTTP: the only subscription delivering over unencrypted
	// HTTP. Every other subscription above uses https, sqs, lambda or email.
	subscriptions = append(subscriptions, snstypes.Subscription{
		TopicArn:        aws.String(SNSPublicPolicyARN),
		Protocol:        aws.String("http"),
		Endpoint:        aws.String("http://hooks.acme-corp.com/sns/orders"),
		SubscriptionArn: aws.String(SNSSubPlainHTTP),
		Owner:           aws.String("123456789012"),
	})

	// Grouped by each subscription's own TopicArn, not by position — the
	// six base subscriptions above are NOT laid out in topic-contiguous
	// blocks (order-events subscriptions interleave with alarm-notifications
	// and deploy-notifications), so a positional slice split silently
	// mis-groups them.
	subsByTopic := map[string][]snstypes.Subscription{}
	for _, s := range subscriptions {
		arn := aws.ToString(s.TopicArn)
		subsByTopic[arn] = append(subsByTopic[arn], s)
	}
	// webhook-integration-pending — 100% of subscribers PendingConfirmation.
	subsByTopic["arn:aws:sns:us-east-1:123456789012:webhook-integration-pending"] = []snstypes.Subscription{
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:webhook-integration-pending"),
			Protocol:        aws.String("https"),
			Endpoint:        aws.String("https://partner-webhooks.example.com/sns-callback"),
			SubscriptionArn: aws.String("PendingConfirmation"),
			Owner:           aws.String("123456789012"),
		},
	}
	// Every graph-root-reachable topic must have at least one subscription
	// so sns→sns_subscriptions drill lands on non-empty content.
	subsByTopic["arn:aws:sns:us-east-1:123456789012:"+S3EventsTopicName] = minimalSubscriptions("arn:aws:sns:us-east-1:123456789012:"+S3EventsTopicName, "sqs", "arn:aws:sqs:us-east-1:123456789012:s3-events-queue")
	subsByTopic[ProdRedisSNSTopicARN] = minimalSubscriptions(ProdRedisSNSTopicARN, "email", "redis-oncall@acme-corp.com")
	subsByTopic["arn:aws:sns:us-east-1:123456789012:"+SESBounceTopicName] = minimalSubscriptions("arn:aws:sns:us-east-1:123456789012:"+SESBounceTopicName, "lambda", "arn:aws:lambda:us-east-1:123456789012:function:ses-bounce-handler")
	subsByTopic[BackupAlertsSNSTopicARN] = minimalSubscriptions(BackupAlertsSNSTopicARN, "email", "backup-ops@acme-corp.com")
	subsByTopic[SNSNoKMSARN] = minimalSubscriptions(SNSNoKMSARN, "sqs", "arn:aws:sqs:us-east-1:123456789012:acme-alerts-queue")

	// TopicAttributes — required for the sns:kms and sns:role related-panel
	// pivots (checkSNSKMS / checkSNSRole via GetTopicAttributes). The
	// alarm-notifications topic carries an access policy granting the CI
	// deploy role publish access, which is what the sns:role pivot keys off.
	// Every topic but SNSNoKMS carries an at-rest encryption key: an
	// unencrypted topic is a finding, so exactly one demo row may be
	// missing it, and the sns:kms pivot resolves for the rest.
	topicAttributes := map[string]map[string]string{
		"arn:aws:sns:us-east-1:123456789012:alarm-notifications": {
			"Policy": `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-ci-deploy-role"},"Action":"sns:Publish","Resource":"arn:aws:sns:us-east-1:123456789012:alarm-notifications"}]}`,
		},
		SNSPublicPolicyARN: {
			"Policy": `{"Version":"2012-10-17","Statement":[{"Sid":"AllowEveryone","Effect":"Allow","Principal":"*","Action":["sns:Publish","sns:Subscribe"],"Resource":"` + SNSPublicPolicyARN + `"}]}`,
		},
	}

	// Every real topic returns GetTopicAttributes with at least these common
	// fields — SubscriptionsConfirmed/SubscriptionsPending are derived from
	// subsByTopic above so the counts can never drift out of sync with the
	// subscription fixtures. alarm-notifications keeps its KmsMasterKeyId/
	// Policy entry above; this loop only adds to it.
	topicDisplayNames := map[string]string{
		"arn:aws:sns:us-east-1:123456789012:alarm-notifications":  "Alarm Notifications",
		"arn:aws:sns:us-east-1:123456789012:order-events":         "Order Events",
		"arn:aws:sns:us-east-1:123456789012:deploy-notifications": "Deploy Notifications",
		"arn:aws:sns:us-east-1:123456789012:" + S3EventsTopicName: "S3 Event Notifications",
		ProdRedisSNSTopicARN: "Redis Production Alerts",
		"arn:aws:sns:us-east-1:123456789012:" + SESBounceTopicName: "SES Bounce Notifications",
		BackupAlertsSNSTopicARN: "Backup Vault Alerts",
		"arn:aws:sns:us-east-1:123456789012:staging-deploy-alerts":       "Staging Deploy Alerts",
		"arn:aws:sns:us-east-1:123456789012:webhook-integration-pending": "Webhook Integration (Pending)",
		SNSPublicPolicyARN: "Public Order Events",
		SNSNoKMSARN:        "Unencrypted Alerts",
	}
	for arn, displayName := range topicDisplayNames {
		confirmed, pending, deleted := 0, 0, 0
		for _, s := range subsByTopic[arn] {
			switch aws.ToString(s.SubscriptionArn) {
			case "PendingConfirmation":
				pending++
			// Deleted subscriptions count in AWS's own SubscriptionsDeleted
			// attribute — neither confirmed nor pending.
			case "Deleted":
				deleted++
			default:
				confirmed++
			}
		}
		attrs := topicAttributes[arn]
		if attrs == nil {
			attrs = map[string]string{}
		}
		if arn != SNSNoKMSARN {
			attrs["KmsMasterKeyId"] = SNSDemoKMSKeyID
		}
		attrs["TopicArn"] = arn
		attrs["Owner"] = "123456789012"
		attrs["DisplayName"] = displayName
		attrs["SubscriptionsConfirmed"] = strconv.Itoa(confirmed)
		attrs["SubscriptionsPending"] = strconv.Itoa(pending)
		attrs["SubscriptionsDeleted"] = strconv.Itoa(deleted)
		attrs["EffectiveDeliveryPolicy"] = `{"http":{"defaultHealthyRetryPolicy":{"minDelayTarget":20,"maxDelayTarget":20,"numRetries":3,"numMaxDelayRetries":0,"numMinDelayRetries":0,"numNoDelayRetries":0,"backoffFunction":"linear"},"disableSubscriptionOverrides":false}}`
		topicAttributes[arn] = attrs
	}

	return &SNSFixtures{
		Topics:               topics,
		Subscriptions:        subscriptions,
		SubscriptionsByTopic: subsByTopic,
		TopicAttributes:      topicAttributes,
	}
})

func NewSNSFixtures() *SNSFixtures {
	return sharedSNSFixtures()
}

// minimalSubscriptions returns a single canonical confirmed subscription so
// sns→sns_subscriptions drill lands on non-empty content. Used for every
// graph-root-reachable topic ARN.
func minimalSubscriptions(topicARN, protocol, endpoint string) []snstypes.Subscription {
	return []snstypes.Subscription{
		{
			TopicArn:        aws.String(topicARN),
			Protocol:        aws.String(protocol),
			Endpoint:        aws.String(endpoint),
			SubscriptionArn: aws.String(topicARN + ":11111111-2222-3333-4444-555555555555"),
			Owner:           aws.String("123456789012"),
		},
	}
}

func init() {
	Register(Pin{ShortName: "sns", Rows: 11, Issues: 0, CoverageGaps: []string{"dim"}})
	Register(Pin{ShortName: "sns-sub", Rows: 7, Issues: 2, CoverageGaps: []string{"broken"}})
}
