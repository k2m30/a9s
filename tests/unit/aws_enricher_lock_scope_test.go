// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// lockScopeBarrier releases once want calls are inside it at the same moment.
// An enricher that makes the call under its result mutex can never get two in
// at once, so the wait runs out and met stays false — the assertion fails
// instead of the suite hanging.
type lockScopeBarrier struct {
	mu       sync.Mutex
	inFlight int
	want     int
	gate     chan struct{}
	met      bool
}

const lockScopeBarrierWait = 300 * time.Millisecond

func newLockScopeBarrier(want int) *lockScopeBarrier {
	return &lockScopeBarrier{want: want, gate: make(chan struct{})}
}

func (b *lockScopeBarrier) arrive() {
	b.mu.Lock()
	b.inFlight++
	if b.inFlight >= b.want && !b.met {
		b.met = true
		close(b.gate)
	}
	b.mu.Unlock()

	select {
	case <-b.gate:
	case <-time.After(lockScopeBarrierWait):
	}

	b.mu.Lock()
	b.inFlight--
	b.mu.Unlock()
}

func (b *lockScopeBarrier) assertMet(t *testing.T, call string) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.met {
		t.Errorf("%s never ran %d calls at once within %v — the enricher holds the result mutex across the AWS call, so every row waits for the one before it", call, b.want, lockScopeBarrierWait)
	}
}

type lockScopeSNSFake struct {
	awsclient.SNSAPI
	barrier *lockScopeBarrier
}

func (f *lockScopeSNSFake) ListSubscriptionsByTopic(_ context.Context, in *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	arn := aws.ToString(in.TopicArn)
	return &sns.ListSubscriptionsByTopicOutput{Subscriptions: []snstypes.Subscription{
		{SubscriptionArn: aws.String(arn + ":11111111-2222-3333-4444-555555555555"), Protocol: aws.String("sqs")},
	}}, nil
}

// The topic is unencrypted, so the posture read has a finding to merge and a
// merge that never happens is visible as a missing finding.
func (f *lockScopeSNSFake) GetTopicAttributes(_ context.Context, in *sns.GetTopicAttributesInput, _ ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	f.barrier.arrive()
	return &sns.GetTopicAttributesOutput{Attributes: map[string]string{
		"TopicArn":               aws.ToString(in.TopicArn),
		"SubscriptionsConfirmed": "1",
	}}, nil
}

func TestEnrichSNSSubscriptions_TopicPostureReadsOverlap(t *testing.T) {
	const topicA = "arn:aws:sns:us-east-1:123456789012:orders-events"
	const topicB = "arn:aws:sns:us-east-1:123456789012:billing-events"

	barrier := newLockScopeBarrier(2)
	fake := &lockScopeSNSFake{barrier: barrier}

	res, err := w2Enricher(t, "sns")(context.Background(), &awsclient.ServiceClients{SNS: fake},
		[]resource.Resource{w2Res(topicA, nil), w2Res(topicB, nil)}, nil)
	if err != nil {
		t.Fatalf("EnrichSNSSubscriptions: %v", err)
	}

	barrier.assertMet(t, "GetTopicAttributes")
	for _, id := range []string{topicA, topicB} {
		w2AssertFindingRaisedOnce(t, res, id, "sns.no-kms")
		if got := res.FieldUpdates[id]["subs_count"]; got != "1" {
			t.Errorf("subs_count for %s = %q, want %q", id, got, "1")
		}
		if why, skipped := res.TruncatedIDs[id]; skipped {
			t.Errorf("%s was marked truncated (%q) — both topics answered", id, why)
		}
	}
}

type lockScopeR53Fake struct {
	awsclient.Route53API
	barrier *lockScopeBarrier
}

func (f *lockScopeR53Fake) GetHostedZone(_ context.Context, in *route53.GetHostedZoneInput, _ ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error) {
	return &route53.GetHostedZoneOutput{HostedZone: &r53types.HostedZone{
		Id:     in.Id,
		Name:   aws.String("acme-corp.example."),
		Config: &r53types.HostedZoneConfig{PrivateZone: false},
	}}, nil
}

func (f *lockScopeR53Fake) ListQueryLoggingConfigs(_ context.Context, _ *route53.ListQueryLoggingConfigsInput, _ ...func(*route53.Options)) (*route53.ListQueryLoggingConfigsOutput, error) {
	f.barrier.arrive()
	return &route53.ListQueryLoggingConfigsOutput{}, nil
}

func (f *lockScopeR53Fake) ListResourceRecordSets(context.Context, *route53.ListResourceRecordSetsInput, ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{}, nil
}

func TestEnrichRoute53Zone_QueryLoggingReadsOverlap(t *testing.T) {
	const zoneA = "Z0A1B2C3D4E5F6G7H8I0"
	const zoneB = "Z0A1B2C3D4E5F6G7H8I1"

	barrier := newLockScopeBarrier(2)
	fake := &lockScopeR53Fake{barrier: barrier}

	zones := make([]resource.Resource, 0, 2)
	for _, id := range []string{zoneA, zoneB} {
		r := w2Res(id, nil)
		r.Fields["zone_id"] = id
		zones = append(zones, r)
	}

	// The address lists are loaded, so a mark on a zone comes from the reads
	// this test overlaps.
	addressesLoaded := resource.ResourceCache{"eip": {Resources: []resource.Resource{}}, "ec2": {Resources: []resource.Resource{}}}
	res, err := w2Enricher(t, "r53")(context.Background(), &awsclient.ServiceClients{Route53: fake}, zones, addressesLoaded)
	if err != nil {
		t.Fatalf("EnrichRoute53Zone: %v", err)
	}

	barrier.assertMet(t, "ListQueryLoggingConfigs")
	for _, id := range []string{zoneA, zoneB} {
		w2AssertFindingRaisedOnce(t, res, id, "r53.query-logging-off")
		if why, skipped := res.TruncatedIDs[id]; skipped {
			t.Errorf("%s was marked truncated (%q) — both zones answered", id, why)
		}
	}
}

type lockScopeAPIGWFake struct {
	awsclient.APIGatewayV2API
	barrier *lockScopeBarrier
}

func (f *lockScopeAPIGWFake) GetStages(_ context.Context, _ *apigatewayv2.GetStagesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
	return &apigatewayv2.GetStagesOutput{Items: []apigwv2types.Stage{{
		StageName:         aws.String("prod"),
		AccessLogSettings: &apigwv2types.AccessLogSettings{DestinationArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/apigw/prod")},
	}}}, nil
}

func (f *lockScopeAPIGWFake) GetAuthorizers(_ context.Context, _ *apigatewayv2.GetAuthorizersInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetAuthorizersOutput, error) {
	f.barrier.arrive()
	return &apigatewayv2.GetAuthorizersOutput{}, nil
}

func TestEnrichAPIGatewayStage_AuthorizerReadsOverlap(t *testing.T) {
	const apiA = "a1b2c3d4e5"
	const apiB = "f6g7h8i9j0"

	barrier := newLockScopeBarrier(2)
	fake := &lockScopeAPIGWFake{barrier: barrier}

	apis := make([]resource.Resource, 0, 2)
	for _, id := range []string{apiA, apiB} {
		r := w2Res(id, nil)
		r.Fields["protocol"] = "HTTP"
		apis = append(apis, r)
	}

	res, err := w2Enricher(t, "apigw")(context.Background(), &awsclient.ServiceClients{APIGatewayV2: fake}, apis, nil)
	if err != nil {
		t.Fatalf("EnrichAPIGatewayStage: %v", err)
	}

	barrier.assertMet(t, "GetAuthorizers")
	for _, id := range []string{apiA, apiB} {
		w2AssertFindingRaisedOnce(t, res, id, "apigw.no-authorizer")
		if got := res.FieldUpdates[id]["stages_count"]; got != "1" {
			t.Errorf("stages_count for %s = %q, want %q", id, got, "1")
		}
		if why, skipped := res.TruncatedIDs[id]; skipped {
			t.Errorf("%s was marked truncated (%q) — both APIs answered", id, why)
		}
	}
}
