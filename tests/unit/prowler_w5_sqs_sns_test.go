package unit

// prowler_w5_sqs_sns_test.go — batch w5 rows 1-4: the queue and topic
// posture signals, plus the plain-HTTP subscription.
//
// Assertions are on literal code and phrase strings rather than on the
// production constants, so a rename is caught rather than followed. The
// Code/Phrase/Severity/Source quadruple and the non-empty Detail sentence go
// through the shared w2 helpers (prowler_w2_common_test.go); only what is
// specific to these four rows lives here.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	snssvc "github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	sqssvc "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Policy documents, in the shapes GetQueueAttributes / GetTopicAttributes
// actually return them: a JSON string, not a struct.
// ---------------------------------------------------------------------------

// w5PublicPolicy grants the given actions to every principal with no
// condition — the shape iampolicy.Evaluate reports as Public.
func w5PublicPolicy(resourceARN string, actions ...string) string {
	quoted := make([]string, 0, len(actions))
	for _, a := range actions {
		quoted = append(quoted, fmt.Sprintf("%q", a))
	}
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Id": "%s/policy",
  "Statement": [
    {
      "Sid": "AllowEveryone",
      "Effect": "Allow",
      "Principal": "*",
      "Action": [%s],
      "Resource": "%s"
    }
  ]
}`, resourceARN, strings.Join(quoted, ", "), resourceARN)
}

// w5OwnAccountPolicy is the healthy counterpart: the same resource, granted
// only to the account that owns it.
func w5OwnAccountPolicy(resourceARN string, actions ...string) string {
	quoted := make([]string, 0, len(actions))
	for _, a := range actions {
		quoted = append(quoted, fmt.Sprintf("%q", a))
	}
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "OwnerOnly",
      "Effect": "Allow",
      "Principal": {"AWS": "arn:aws:iam::123456789012:root"},
      "Action": [%s],
      "Resource": "%s"
    }
  ]
}`, strings.Join(quoted, ", "), resourceARN)
}

// w5ConditionedPublicPolicy is a wildcard principal scoped by a source-VPCE
// condition. Rule 6 of the batch contract: Exposure.Conditioned is NOT a
// finding — a scoped grant is how a queue is legitimately shared.
func w5ConditionedPublicPolicy(resourceARN string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "sqs:SendMessage",
      "Resource": "%s",
      "Condition": {"StringEquals": {"aws:SourceVpce": "vpce-0a1b2c3d4e5f60001"}}
    }
  ]
}`, resourceARN)
}

// ---------------------------------------------------------------------------
// Row 1 — sqs.public-policy
// ---------------------------------------------------------------------------

// w5SQSFake serves GetQueueAttributes per queue URL and records the
// AttributeNames each call asked for. The recorded names are the subject of
// their own assertion: the enricher's explicit list is what decides whether
// Policy is in the response at all, so a predicate that reads Policy while
// the request never asks for it is silently dead.
type w5SQSFake struct {
	awsclient.SQSAPI
	attrs    map[string]map[string]string
	errByURL map[string]error
	// mu guards requested: the enricher fans its per-queue calls out through
	// ForEachParallel, so every recording write here is concurrent.
	mu        sync.Mutex
	requested map[string][]sqstypes.QueueAttributeName
}

func newW5SQSFake() *w5SQSFake {
	return &w5SQSFake{
		attrs:     map[string]map[string]string{},
		errByURL:  map[string]error{},
		requested: map[string][]sqstypes.QueueAttributeName{},
	}
}

func (f *w5SQSFake) GetQueueAttributes(_ context.Context, in *sqssvc.GetQueueAttributesInput, _ ...func(*sqssvc.Options)) (*sqssvc.GetQueueAttributesOutput, error) {
	url := ""
	if in != nil && in.QueueUrl != nil {
		url = *in.QueueUrl
	}
	if in != nil {
		f.mu.Lock()
		f.requested[url] = in.AttributeNames
		f.mu.Unlock()
	}
	if err, ok := f.errByURL[url]; ok {
		return nil, err
	}
	a := f.attrs[url]
	if a == nil {
		a = map[string]string{}
	}
	return &sqssvc.GetQueueAttributesOutput{Attributes: a}, nil
}

var _ awsclient.SQSAPI = (*w5SQSFake)(nil)

// w5QueueURL and w5QueueARN mirror what the sqs fetcher stores in Fields.
func w5QueueURL(name string) string {
	return "https://sqs.us-east-1.amazonaws.com/123456789012/" + name
}

func w5QueueARN(name string) string {
	return "arn:aws:sqs:us-east-1:123456789012:" + name
}

// w5QueueRes builds one sqs row the way FetchSQSQueuesPage does — the
// enricher keys its call off Fields["queue_url"], so a row without one is
// skipped entirely.
func w5QueueRes(name string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"queue_name": name,
			"queue_url":  w5QueueURL(name),
			"arn":        w5QueueARN(name),
		},
	}
}

// A queue whose policy allows every principal is reachable by anyone who
// knows the URL. The supporting rows name who is allowed and what they may
// do, because the phrase alone does not say which actions were granted.
func TestW5_SQSPublicPolicy_Positive(t *testing.T) {
	name := "acme-orders-public"
	f := newW5SQSFake()
	f.attrs[w5QueueURL(name)] = map[string]string{
		"QueueArn":       w5QueueARN(name),
		"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:acme-orders-dlq","maxReceiveCount":"5"}`,
		"KmsMasterKeyId": "alias/aws/sqs",
		"Policy":         w5PublicPolicy(w5QueueARN(name), "sqs:SendMessage", "sqs:ReceiveMessage"),
	}

	res, err := awsclient.EnrichSQSAttributes(
		context.Background(),
		&awsclient.ServiceClients{SQS: f},
		[]resource.Resource{w5QueueRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)

	w2AssertFinding(t, res.Findings[name], "sqs.public-policy",
		"queue policy open to anyone", domain.SevBroken, "wave2")

	rows := w2Rows(t, res, name, "sqs.public-policy")
	w2AssertRow(t, rows, "Principal", "*")
	// iampolicy sorts PublicActions, so the joined value is deterministic
	// regardless of the order the statement listed them in.
	w2AssertRow(t, rows, "Actions", "sqs:ReceiveMessage, sqs:SendMessage")
}

// The enricher asks for an explicit AttributeNames list. Policy has to be on
// it: GetQueueAttributes returns only what was requested, so a predicate
// reading out.Attributes["Policy"] against a request that never named it can
// never fire, and no amount of fixture data would reveal that.
func TestW5_SQSEnricherRequestsThePolicyAttribute(t *testing.T) {
	name := "acme-orders-public"
	f := newW5SQSFake()
	f.attrs[w5QueueURL(name)] = map[string]string{"QueueArn": w5QueueARN(name)}

	_, err := awsclient.EnrichSQSAttributes(
		context.Background(),
		&awsclient.ServiceClients{SQS: f},
		[]resource.Resource{w5QueueRes(name)},
		nil,
	)
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}

	f.mu.Lock()
	asked, ok := f.requested[w5QueueURL(name)]
	f.mu.Unlock()
	if !ok {
		t.Fatalf("GetQueueAttributes was never called for %s", name)
	}
	for _, n := range asked {
		if n == sqstypes.QueueAttributeNamePolicy || n == sqstypes.QueueAttributeNameAll {
			return
		}
	}
	t.Errorf("GetQueueAttributes asked for %v; the Policy attribute is not among them, so the public-policy predicate reads an attribute AWS was never asked to return", asked)
}

// An account-scoped policy is the ordinary case and must stay silent.
func TestW5_SQSPublicPolicy_OwnAccountPolicyIsHealthy(t *testing.T) {
	name := "acme-orders"
	f := newW5SQSFake()
	f.attrs[w5QueueURL(name)] = map[string]string{
		"QueueArn":       w5QueueARN(name),
		"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:acme-orders-dlq","maxReceiveCount":"5"}`,
		"KmsMasterKeyId": "alias/aws/sqs",
		"Policy":         w5OwnAccountPolicy(w5QueueARN(name), "sqs:SendMessage"),
	}

	res, err := awsclient.EnrichSQSAttributes(
		context.Background(),
		&awsclient.ServiceClients{SQS: f},
		[]resource.Resource{w5QueueRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings[name], "sqs.public-policy")
}

// No Policy attribute at all is the default for a queue nobody has shared.
// Absence is not misconfiguration.
func TestW5_SQSPublicPolicy_NoPolicyAttributeIsHealthy(t *testing.T) {
	name := "acme-orders"
	f := newW5SQSFake()
	f.attrs[w5QueueURL(name)] = map[string]string{
		"QueueArn":       w5QueueARN(name),
		"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:acme-orders-dlq","maxReceiveCount":"5"}`,
		"KmsMasterKeyId": "alias/aws/sqs",
	}

	res, err := awsclient.EnrichSQSAttributes(
		context.Background(),
		&awsclient.ServiceClients{SQS: f},
		[]resource.Resource{w5QueueRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings[name], "sqs.public-policy")
}

// A wildcard principal scoped by a condition is a deliberate, bounded grant.
// Rule 6: Conditioned is not a finding.
func TestW5_SQSPublicPolicy_ConditionedWildcardIsHealthy(t *testing.T) {
	name := "acme-orders-vpce"
	f := newW5SQSFake()
	f.attrs[w5QueueURL(name)] = map[string]string{
		"QueueArn":       w5QueueARN(name),
		"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:acme-orders-dlq","maxReceiveCount":"5"}`,
		"KmsMasterKeyId": "alias/aws/sqs",
		"Policy":         w5ConditionedPublicPolicy(w5QueueARN(name)),
	}

	res, err := awsclient.EnrichSQSAttributes(
		context.Background(),
		&awsclient.ServiceClients{SQS: f},
		[]resource.Resource{w5QueueRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings[name], "sqs.public-policy")
}

// Open item 1 of dev's round 0, inverted here: EnrichSQSAttributes used to
// emit sqs.missing-dlq for a missing dead-letter queue AND for a missing KMS
// key, taking its phrase from whichever row happened to be first. Rule 4
// makes those two independent conditions, so a queue missing only encryption
// must not report the dead-letter-queue code, and a queue missing only the
// dead-letter queue must not report the encryption one.
func TestW5_SQSOneCodePerCondition(t *testing.T) {
	tests := []struct {
		name       string
		queue      string
		attrs      map[string]string
		wantCodes  []string
		absentCode string
	}{
		{
			name:  "encryption missing only",
			queue: "acme-unencrypted",
			attrs: map[string]string{
				"RedrivePolicy": `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:acme-dlq","maxReceiveCount":"5"}`,
			},
			wantCodes:  []string{"sqs.no-kms"},
			absentCode: "sqs.missing-dlq",
		},
		{
			name:       "dead-letter queue missing only",
			queue:      "acme-no-dlq",
			attrs:      map[string]string{"KmsMasterKeyId": "alias/aws/sqs"},
			wantCodes:  []string{"sqs.missing-dlq"},
			absentCode: "sqs.no-kms",
		},
		{
			name:      "both missing",
			queue:     "acme-bare",
			attrs:     map[string]string{},
			wantCodes: []string{"sqs.missing-dlq", "sqs.no-kms"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newW5SQSFake()
			attrs := map[string]string{"QueueArn": w5QueueARN(tc.queue)}
			for k, v := range tc.attrs {
				attrs[k] = v
			}
			f.attrs[w5QueueURL(tc.queue)] = attrs

			res, err := awsclient.EnrichSQSAttributes(
				context.Background(),
				&awsclient.ServiceClients{SQS: f},
				[]resource.Resource{w5QueueRes(tc.queue)},
				nil,
			)
			w2AssertEnricherInvariants(t, res, err)

			for _, code := range tc.wantCodes {
				if _, ok := w2Find(res.Findings[tc.queue], code); !ok {
					t.Errorf("no finding %q; got %v", code, w2Codes(res.Findings[tc.queue]))
				}
			}
			if tc.absentCode != "" {
				w2AssertNoCode(t, res.Findings[tc.queue], tc.absentCode)
			}
		})
	}
}

// One queue failing its own GetQueueAttributes marks that queue truncated —
// the row renders `?` rather than a confident healthy — while the other
// queues in the same batch are still evaluated.
func TestW5_SQSPublicPolicy_APIErrorOnOneQueueTruncatesOnlyThatQueue(t *testing.T) {
	broken, healthy := "acme-broken", "acme-orders-public"
	f := newW5SQSFake()
	f.errByURL[w5QueueURL(broken)] = errors.New("AccessDenied: not authorized to perform sqs:GetQueueAttributes")
	f.attrs[w5QueueURL(healthy)] = map[string]string{
		"QueueArn":       w5QueueARN(healthy),
		"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:acme-dlq","maxReceiveCount":"5"}`,
		"KmsMasterKeyId": "alias/aws/sqs",
		"Policy":         w5PublicPolicy(w5QueueARN(healthy), "sqs:SendMessage"),
	}

	res, _ := awsclient.EnrichSQSAttributes(
		context.Background(),
		&awsclient.ServiceClients{SQS: f},
		[]resource.Resource{w5QueueRes(broken), w5QueueRes(healthy)},
		nil,
	)
	w2AssertEnricherShape(t, res)

	if _, marked := res.TruncatedIDs[broken]; !marked {
		t.Errorf("TruncatedIDs[%q] = false; a queue whose attribute call failed is unknown, not healthy", broken)
	}
	if len(res.Findings[broken]) != 0 {
		t.Errorf("failed queue %q carries findings %v; nothing was observed about it", broken, w2Codes(res.Findings[broken]))
	}
	if _, marked := res.TruncatedIDs[healthy]; marked {
		t.Errorf("TruncatedIDs[%q] = true; one queue's failure must not mark the others unknown", healthy)
	}
	w2AssertFinding(t, res.Findings[healthy], "sqs.public-policy",
		"queue policy open to anyone", domain.SevBroken, "wave2")
}

func TestW5_SQSPublicPolicy_NilClientReturnsEmptyResult(t *testing.T) {
	res, err := awsclient.EnrichSQSAttributes(
		context.Background(),
		&awsclient.ServiceClients{},
		[]resource.Resource{w5QueueRes("acme-orders")},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v on a session with no SQS client", res.Findings)
	}
}

func TestW5_SQSPublicPolicy_CatalogDef(t *testing.T) {
	w2AssertFindingDef(t, "sqs", "sqs.public-policy",
		"queue policy open to anyone", domain.SevBroken, "wave2")
}

// ---------------------------------------------------------------------------
// Rows 2, 3 — sns.public-policy, sns.no-kms
// ---------------------------------------------------------------------------

// w5SNSFake serves both calls EnrichSNSSubscriptions makes: the subscription
// listing it already walked, and the topic attributes rows 2 and 3 read.
type w5SNSFake struct {
	awsclient.SNSAPI
	topicAttrs map[string]map[string]string
	subs       map[string][]snstypes.Subscription
	attrErr    map[string]error
}

func newW5SNSFake() *w5SNSFake {
	return &w5SNSFake{
		topicAttrs: map[string]map[string]string{},
		subs:       map[string][]snstypes.Subscription{},
		attrErr:    map[string]error{},
	}
}

func (f *w5SNSFake) GetTopicAttributes(_ context.Context, in *snssvc.GetTopicAttributesInput, _ ...func(*snssvc.Options)) (*snssvc.GetTopicAttributesOutput, error) {
	arn := ""
	if in != nil && in.TopicArn != nil {
		arn = *in.TopicArn
	}
	if err, ok := f.attrErr[arn]; ok {
		return nil, err
	}
	a := f.topicAttrs[arn]
	if a == nil {
		a = map[string]string{}
	}
	return &snssvc.GetTopicAttributesOutput{Attributes: a}, nil
}

func (f *w5SNSFake) ListSubscriptionsByTopic(_ context.Context, in *snssvc.ListSubscriptionsByTopicInput, _ ...func(*snssvc.Options)) (*snssvc.ListSubscriptionsByTopicOutput, error) {
	arn := ""
	if in != nil && in.TopicArn != nil {
		arn = *in.TopicArn
	}
	return &snssvc.ListSubscriptionsByTopicOutput{Subscriptions: f.subs[arn]}, nil
}

var _ awsclient.SNSAPI = (*w5SNSFake)(nil)

func w5TopicARN(name string) string {
	return "arn:aws:sns:us-east-1:123456789012:" + name
}

// w5TopicRes builds one sns row. The sns enricher keys on r.ID, which the
// fetcher sets to the topic ARN.
func w5TopicRes(name string) resource.Resource {
	return resource.Resource{
		ID:     w5TopicARN(name),
		Name:   name,
		Fields: map[string]string{"topic_name": name, "arn": w5TopicARN(name)},
	}
}

// w5HealthySub is one confirmed subscriber, so the topic does not also trip
// the pre-existing no-subscribers / all-pending findings and the assertions
// below are about rows 2 and 3 alone.
func w5HealthySub(topic string) []snstypes.Subscription {
	return []snstypes.Subscription{{
		SubscriptionArn: aws.String(w5TopicARN(topic) + ":11111111-2222-3333-4444-555555555555"),
		TopicArn:        aws.String(w5TopicARN(topic)),
		Protocol:        aws.String("sqs"),
		Endpoint:        aws.String("arn:aws:sqs:us-east-1:123456789012:acme-orders"),
	}}
}

// A topic whose access policy allows every principal can be published to, or
// subscribed to, by anyone.
func TestW5_SNSPublicPolicy_Positive(t *testing.T) {
	name := "acme-alerts-public"
	arn := w5TopicARN(name)
	f := newW5SNSFake()
	f.subs[arn] = w5HealthySub(name)
	f.topicAttrs[arn] = map[string]string{
		"TopicArn":       arn,
		"KmsMasterKeyId": "alias/aws/sns",
		"Policy":         w5PublicPolicy(arn, "SNS:Publish", "SNS:Subscribe"),
	}

	res, err := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{SNS: f},
		[]resource.Resource{w5TopicRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)

	w2AssertFinding(t, res.Findings[arn], "sns.public-policy",
		"topic policy open to anyone", domain.SevBroken, "wave2")

	rows := w2Rows(t, res, arn, "sns.public-policy")
	w2AssertRow(t, rows, "Principal", "*")
	w2AssertRow(t, rows, "Actions", "SNS:Publish, SNS:Subscribe")
}

func TestW5_SNSPublicPolicy_OwnAccountPolicyIsHealthy(t *testing.T) {
	name := "acme-alerts"
	arn := w5TopicARN(name)
	f := newW5SNSFake()
	f.subs[arn] = w5HealthySub(name)
	f.topicAttrs[arn] = map[string]string{
		"TopicArn":       arn,
		"KmsMasterKeyId": "alias/aws/sns",
		"Policy":         w5OwnAccountPolicy(arn, "SNS:Publish"),
	}

	res, err := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{SNS: f},
		[]resource.Resource{w5TopicRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings[arn], "sns.public-policy")
}

// No Policy attribute is unknown, not open.
func TestW5_SNSPublicPolicy_NoPolicyAttributeIsHealthy(t *testing.T) {
	name := "acme-alerts"
	arn := w5TopicARN(name)
	f := newW5SNSFake()
	f.subs[arn] = w5HealthySub(name)
	f.topicAttrs[arn] = map[string]string{"TopicArn": arn, "KmsMasterKeyId": "alias/aws/sns"}

	res, err := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{SNS: f},
		[]resource.Resource{w5TopicRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings[arn], "sns.public-policy")
}

// A topic with no KmsMasterKeyId stores every message unencrypted at rest.
func TestW5_SNSNoKMS_Positive(t *testing.T) {
	name := "acme-alerts-unencrypted"
	arn := w5TopicARN(name)
	f := newW5SNSFake()
	f.subs[arn] = w5HealthySub(name)
	f.topicAttrs[arn] = map[string]string{
		"TopicArn": arn,
		"Policy":   w5OwnAccountPolicy(arn, "SNS:Publish"),
	}

	res, err := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{SNS: f},
		[]resource.Resource{w5TopicRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)

	w2AssertFinding(t, res.Findings[arn], "sns.no-kms",
		"not encrypted with KMS", domain.SevWarn, "wave2")

	// The value is pinned; the label is left to the rendered-surface gate,
	// which forbids the SDK's own field name on a rendered row.
	rows := w2Rows(t, res, arn, "sns.no-kms")
	var got []string
	for _, r := range rows {
		got = append(got, r.Label+": "+r.Value)
		if r.Value == "none" {
			return
		}
	}
	t.Errorf("no supporting row valued %q; got %v — the row exists to say the topic has no key at all", "none", got)
}

// An empty KmsMasterKeyId is the same fact as an absent one: AWS omits the
// attribute for an unencrypted topic, and some responses carry it empty.
func TestW5_SNSNoKMS_EmptyKeyIDIsTheSameAsAbsent(t *testing.T) {
	name := "acme-alerts-emptykey"
	arn := w5TopicARN(name)
	f := newW5SNSFake()
	f.subs[arn] = w5HealthySub(name)
	f.topicAttrs[arn] = map[string]string{"TopicArn": arn, "KmsMasterKeyId": ""}

	res, err := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{SNS: f},
		[]resource.Resource{w5TopicRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertFinding(t, res.Findings[arn], "sns.no-kms",
		"not encrypted with KMS", domain.SevWarn, "wave2")
}

func TestW5_SNSNoKMS_EncryptedTopicIsHealthy(t *testing.T) {
	name := "acme-alerts"
	arn := w5TopicARN(name)
	f := newW5SNSFake()
	f.subs[arn] = w5HealthySub(name)
	f.topicAttrs[arn] = map[string]string{
		"TopicArn":       arn,
		"KmsMasterKeyId": "arn:aws:kms:us-east-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab",
	}

	res, err := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{SNS: f},
		[]resource.Resource{w5TopicRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings[arn], "sns.no-kms")
}

// Rule 4: two independent conditions on one topic produce two findings, each
// with its own code and its own supporting rows.
func TestW5_SNSPublicAndUnencrypted_AreTwoFindings(t *testing.T) {
	name := "acme-alerts-both"
	arn := w5TopicARN(name)
	f := newW5SNSFake()
	f.subs[arn] = w5HealthySub(name)
	f.topicAttrs[arn] = map[string]string{
		"TopicArn": arn,
		"Policy":   w5PublicPolicy(arn, "SNS:Publish"),
	}

	res, err := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{SNS: f},
		[]resource.Resource{w5TopicRes(name)},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)

	w2AssertFinding(t, res.Findings[arn], "sns.public-policy",
		"topic policy open to anyone", domain.SevBroken, "wave2")
	w2AssertFinding(t, res.Findings[arn], "sns.no-kms",
		"not encrypted with KMS", domain.SevWarn, "wave2")

	if _, ok := res.AttentionDetails[arn][domain.FindingCode("sns.public-policy")]; !ok {
		t.Error("the two findings share one AttentionDetail entry; rows are keyed per code so each condition keeps its own evidence")
	}
}

// A topic whose GetTopicAttributes call fails is unknown for rows 2 and 3.
// It must not be reported healthy, and it must not stop the other topics.
func TestW5_SNSTopicAttributes_APIErrorTruncatesOnlyThatTopic(t *testing.T) {
	broken, healthy := "acme-broken", "acme-alerts-public"
	brokenARN, healthyARN := w5TopicARN(broken), w5TopicARN(healthy)

	f := newW5SNSFake()
	f.subs[brokenARN] = w5HealthySub(broken)
	f.subs[healthyARN] = w5HealthySub(healthy)
	f.attrErr[brokenARN] = errors.New("AuthorizationError: not authorized to perform SNS:GetTopicAttributes")
	f.topicAttrs[healthyARN] = map[string]string{
		"TopicArn":       healthyARN,
		"KmsMasterKeyId": "alias/aws/sns",
		"Policy":         w5PublicPolicy(healthyARN, "SNS:Publish"),
	}

	res, _ := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{SNS: f},
		[]resource.Resource{w5TopicRes(broken), w5TopicRes(healthy)},
		nil,
	)
	w2AssertEnricherShape(t, res)

	if _, marked := res.TruncatedIDs[brokenARN]; !marked {
		t.Errorf("TruncatedIDs[%q] = false; a topic whose attributes could not be read is unknown, not healthy", broken)
	}
	w2AssertNoCode(t, res.Findings[brokenARN], "sns.public-policy")
	w2AssertNoCode(t, res.Findings[brokenARN], "sns.no-kms")

	if _, marked := res.TruncatedIDs[healthyARN]; marked {
		t.Errorf("TruncatedIDs[%q] = true; one topic's failure must not mark the others unknown", healthy)
	}
	w2AssertFinding(t, res.Findings[healthyARN], "sns.public-policy",
		"topic policy open to anyone", domain.SevBroken, "wave2")
}

// The cap bounds how many topics are inspected. With EnrichmentCap+1 topics
// the enricher must still return a well-formed result and must not inspect
// past the cap, which is the whole reason the cap exists.
func TestW5_SNSTopicAttributes_CapPlusOne(t *testing.T) {
	f := newW5SNSFake()
	var rows []resource.Resource
	for i := range awsclient.EnrichmentCap + 1 {
		name := fmt.Sprintf("acme-topic-%03d", i)
		arn := w5TopicARN(name)
		f.subs[arn] = w5HealthySub(name)
		f.topicAttrs[arn] = map[string]string{
			"TopicArn": arn,
			"Policy":   w5PublicPolicy(arn, "SNS:Publish"),
		}
		rows = append(rows, w5TopicRes(name))
	}

	res, err := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{SNS: f},
		rows,
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)

	if len(res.Findings) > awsclient.EnrichmentCap {
		t.Errorf("%d topics carry findings with a cap of %d; the enricher inspected past its own bound",
			len(res.Findings), awsclient.EnrichmentCap)
	}
	if len(res.Findings) == 0 {
		t.Error("no topic carries a finding; the cap must bound the work, not eliminate it")
	}
}

func TestW5_SNSTopicAttributes_NilClientReturnsEmptyResult(t *testing.T) {
	res, err := awsclient.EnrichSNSSubscriptions(
		context.Background(),
		&awsclient.ServiceClients{},
		[]resource.Resource{w5TopicRes("acme-alerts")},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v on a session with no SNS client", res.Findings)
	}
}

func TestW5_SNSCatalogDefs(t *testing.T) {
	w2AssertFindingDef(t, "sns", "sns.public-policy",
		"topic policy open to anyone", domain.SevBroken, "wave2")
	w2AssertFindingDef(t, "sns", "sns.no-kms",
		"not encrypted with KMS", domain.SevWarn, "wave2")
}

// ---------------------------------------------------------------------------
// Row 4 — sns-sub.plain-http (wave 1, three call sites)
// ---------------------------------------------------------------------------

// w5SNSSubListFake serves the top-level subscription listing.
type w5SNSSubListFake struct {
	subs []snstypes.Subscription
}

func (f *w5SNSSubListFake) ListSubscriptions(_ context.Context, _ *snssvc.ListSubscriptionsInput, _ ...func(*snssvc.Options)) (*snssvc.ListSubscriptionsOutput, error) {
	return &snssvc.ListSubscriptionsOutput{Subscriptions: f.subs}, nil
}

var _ awsclient.SNSListSubscriptionsAPI = (*w5SNSSubListFake)(nil)

// w5SNSSubByTopicFake serves the by-topic child listing — the second of the
// three sites that build sns-sub rows.
type w5SNSSubByTopicFake struct {
	subs []snstypes.Subscription
}

func (f *w5SNSSubByTopicFake) ListSubscriptionsByTopic(_ context.Context, _ *snssvc.ListSubscriptionsByTopicInput, _ ...func(*snssvc.Options)) (*snssvc.ListSubscriptionsByTopicOutput, error) {
	return &snssvc.ListSubscriptionsByTopicOutput{Subscriptions: f.subs}, nil
}

var _ awsclient.SNSListSubscriptionsByTopicAPI = (*w5SNSSubByTopicFake)(nil)

func w5Sub(protocol, endpoint string) snstypes.Subscription {
	return snstypes.Subscription{
		SubscriptionArn: aws.String(w5TopicARN("acme-alerts") + ":aaaaaaaa-bbbb-cccc-dddd-" + protocol + "0000000"),
		TopicArn:        aws.String(w5TopicARN("acme-alerts")),
		Protocol:        aws.String(protocol),
		Endpoint:        aws.String(endpoint),
		Owner:           aws.String("123456789012"),
	}
}

// An http subscription hands every notification to the network in clear
// text. https, and the AWS-internal protocols, do not.
func TestW5_SNSSubPlainHTTP_ByProtocol(t *testing.T) {
	tests := []struct {
		protocol string
		endpoint string
		want     bool
	}{
		{"http", "http://hooks.acme-corp.com/sns/orders", true},
		{"https", "https://hooks.acme-corp.com/sns/orders", false},
		{"sqs", "arn:aws:sqs:us-east-1:123456789012:acme-orders", false},
		{"lambda", "arn:aws:lambda:us-east-1:123456789012:function:acme-notify", false},
		{"email", "ops@acme-corp.com", false},
	}

	for _, tc := range tests {
		t.Run(tc.protocol, func(t *testing.T) {
			out, err := awsclient.FetchSNSSubscriptionsPage(
				context.Background(),
				&w5SNSSubListFake{subs: []snstypes.Subscription{w5Sub(tc.protocol, tc.endpoint)}},
				"",
			)
			if err != nil {
				t.Fatalf("FetchSNSSubscriptionsPage: %v", err)
			}
			if len(out.Resources) != 1 {
				t.Fatalf("got %d rows, want 1", len(out.Resources))
			}
			fs := out.Resources[0].Findings
			if !tc.want {
				w2AssertNoCode(t, fs, "sns-sub.plain-http")
				return
			}
			w2AssertFinding(t, fs, "sns-sub.plain-http",
				"delivers over plain HTTP", domain.SevWarn, "wave1")
		})
	}
}

// The endpoint row names where the traffic goes, with the path stripped: a
// webhook path is frequently the shared secret that authenticates the
// caller, and rule 7 keeps secrets out of rendered rows.
func TestW5_SNSSubPlainHTTP_EndpointRowDropsThePath(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"path and query", "http://hooks.acme-corp.com/sns/orders?token=s3cr3t", "http://hooks.acme-corp.com"},
		{"fragment", "http://hooks.acme-corp.com/sns#orders", "http://hooks.acme-corp.com"},
		{"port kept", "http://hooks.acme-corp.com:8080/sns/orders", "http://hooks.acme-corp.com:8080"},
		// Userinfo is the other half of the same rule and the sharper half:
		// a webhook that authenticates by embedding its credentials in the
		// URL puts them in front of the host, so keeping "scheme://host"
		// literally would render the credential on the row.
		{"userinfo dropped", "http://acme:hunter2secret@hooks.acme-corp.com/sns/orders", "http://hooks.acme-corp.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := awsclient.FetchSNSSubscriptionsPage(
				context.Background(),
				&w5SNSSubListFake{subs: []snstypes.Subscription{w5Sub("http", tc.endpoint)}},
				"",
			)
			if err != nil {
				t.Fatalf("FetchSNSSubscriptionsPage: %v", err)
			}
			r := out.Resources[0]
			ad, ok := r.AttentionDetails[domain.FindingCode("sns-sub.plain-http")]
			if !ok {
				t.Fatalf("no AttentionDetail rows for sns-sub.plain-http; got %v", r.AttentionDetails)
			}
			var endpoint string
			for _, row := range ad.Rows {
				if row.Label == "Endpoint" {
					endpoint = row.Value
				}
			}
			if endpoint != tc.want {
				t.Errorf("Endpoint row = %q, want %q — scheme and host only", endpoint, tc.want)
			}
			if strings.Contains(endpoint, "hunter2secret") {
				t.Errorf("Endpoint row = %q carries the credential embedded in the endpoint", endpoint)
			}
		})
	}
}

// The by-topic child fetcher renders the same rows as the top-level list, so
// it must derive the same finding. This is the site the ARN-only helper
// signature used to hide.
func TestW5_SNSSubPlainHTTP_ByTopicFetcherAgrees(t *testing.T) {
	out, err := awsclient.FetchSNSTopicSubscriptions(
		context.Background(),
		&w5SNSSubByTopicFake{subs: []snstypes.Subscription{
			w5Sub("http", "http://hooks.acme-corp.com/sns/orders"),
		}},
		w5TopicARN("acme-alerts"),
		"",
	)
	if err != nil {
		t.Fatalf("FetchSNSTopicSubscriptions: %v", err)
	}
	if len(out.Resources) != 1 {
		t.Fatalf("got %d rows, want 1", len(out.Resources))
	}
	w2AssertFinding(t, out.Resources[0].Findings, "sns-sub.plain-http",
		"delivers over plain HTTP", domain.SevWarn, "wave1")
}

// The third site: a cache-restored row arrives with Fields and no Findings,
// and the classifier recomputes from Fields alone. A protocol-derived
// finding that the fetcher sees and the classifier does not is the row
// changing colour depending on where it came from.
func TestW5_SNSSubPlainHTTP_ColourFallbackAgreesWithTheFetcher(t *testing.T) {
	td := resource.FindResourceType("sns-sub")
	if td == nil {
		t.Fatal("sns-sub not registered")
	}
	out, err := awsclient.FetchSNSSubscriptionsPage(
		context.Background(),
		&w5SNSSubListFake{subs: []snstypes.Subscription{
			w5Sub("http", "http://hooks.acme-corp.com/sns/orders"),
		}},
		"",
	)
	if err != nil {
		t.Fatalf("FetchSNSSubscriptionsPage: %v", err)
	}
	fetched := out.Resources[0]
	bare := resource.Resource{ID: fetched.ID, Fields: fetched.Fields}

	if got, want := td.ResolveColor(bare), td.ResolveColor(fetched); got != want {
		t.Errorf("cache-restored row colour %v, fetched row colour %v — the classifier and the fetcher disagree; Fields = %v",
			got, want, fetched.Fields)
	}
	if got := td.ResolveColor(bare); got != resource.ColorWarning {
		t.Errorf("cache-restored plain-HTTP subscription colour = %v, want ColorWarning", got)
	}
}

// A subscription AWS reports as Deleted is gone; rule 4 says a deleted
// resource emits no posture finding, whatever its protocol was.
func TestW5_SNSSubPlainHTTP_DeletedSubscriptionEmitsNoPostureFinding(t *testing.T) {
	sub := w5Sub("http", "http://hooks.acme-corp.com/sns/orders")
	sub.SubscriptionArn = aws.String("Deleted")

	out, err := awsclient.FetchSNSSubscriptionsPage(
		context.Background(),
		&w5SNSSubListFake{subs: []snstypes.Subscription{sub}},
		"",
	)
	if err != nil {
		t.Fatalf("FetchSNSSubscriptionsPage: %v", err)
	}
	w2AssertNoCode(t, out.Resources[0].Findings, "sns-sub.plain-http")
}

// The code is declared on BOTH sns-sub literals — the top-level type and the
// topic's child view — because both render the same rows, and a code with no
// FindingDef on the literal a row is rendered under is invisible there.
func TestW5_SNSSubCatalogDefOnBothLiterals(t *testing.T) {
	w2AssertFindingDef(t, "sns-sub", "sns-sub.plain-http",
		"delivers over plain HTTP", domain.SevWarn, "wave1")

	child := catalog.ChildOnly("sns_subscriptions")
	if child == nil {
		t.Fatal(`catalog.ChildOnly("sns_subscriptions") returned nil`)
	}
	for _, fd := range child.Findings {
		if string(fd.Code) != "sns-sub.plain-http" {
			continue
		}
		if fd.Phrase != "delivers over plain HTTP" {
			t.Errorf("child literal FindingDef Phrase = %q, want %q", fd.Phrase, "delivers over plain HTTP")
		}
		if fd.Severity != domain.SevWarn {
			t.Errorf("child literal FindingDef Severity = %v, want %v", fd.Severity, domain.SevWarn)
		}
		if fd.Source != "wave1" {
			t.Errorf("child literal FindingDef Source = %q, want %q", fd.Source, "wave1")
		}
		return
	}
	t.Error(`no FindingDef for "sns-sub.plain-http" on the sns_subscriptions child literal; the topic's subscription list renders the same rows as the top-level type and would not know the code`)
}
