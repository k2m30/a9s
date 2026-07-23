// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// coalesce.go wraps the three AWS clients whose narrow related-checker AND
// on-demand detail-enricher independently call the identical operation on
// the identical resource on every detail open: sfn's DescribeStateMachine
// (checkSFNRole + checkSFNKMS + checkSFNLambda + enrichSfn — up to 4x
// concurrently), sns's GetTopicAttributes (checkSNSKMS + checkSNSRole +
// enrichSns), and s3's GetBucketPolicy (the s3→role related checker +
// enrichS3). golang.org/x/sync/singleflight coalesces concurrent identical
// in-flight calls into one underlying call and shares its result across
// every caller waiting on that key; it does NOT cache — a call made after
// the previous one already completed always re-executes. Shared in-flight
// result, uncached subsequent opens: no staleness risk, only the redundant
// concurrent duplicate work is removed.
//
// Wired only at the live client bootstrap (CreateServiceClients, below in
// this package). Demo mode's fakes (core/demo/client.go) are instant,
// in-process, and deterministic — coalescing them would add complexity for
// zero benefit, so they stay undecorated (core/demo/client.go overwrites
// these three fields with typed fakes immediately after calling
// CreateServiceClients, discarding whatever real client this file
// constructed).
//
// The three constructors are exported (returning the widest interface, not
// the concrete decorator type) so both the live bootstrap and external
// tests (tests/unit, an exported-symbols-only package) can construct one —
// the decorator struct types themselves stay unexported implementation
// detail.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"golang.org/x/sync/singleflight"
)

// coalesceBypassKey is the unexported context-key type for
// WithCoalesceBypass/CoalesceBypassed — unexported so no other package can
// collide with or forge the marker.
type coalesceBypassKey struct{}

// WithCoalesceBypass marks ctx so a coalescing decorator's Do call below is
// skipped for this one call. Wired by enrichDetail (detail_enrich_engine.go)
// when DetailEnrichmentCtx.SkipCache is set (an explicit Ctrl+R refresh), and
// by core/runtime's related-checker fan-out for an explicit detail refresh:
// the refresh must not join whatever pre-refresh call is already in flight on
// the same key, because a shared singleflight response can't be freshened
// after the fact — only a call that actually re-executes can. Exported so
// core/runtime (a separate package) can wrap the same marker onto a related
// checker's context.
func WithCoalesceBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, coalesceBypassKey{}, true)
}

// CoalesceBypassed reports whether ctx carries the WithCoalesceBypass marker.
// Exported as the read-side contract probe alongside WithCoalesceBypass.
func CoalesceBypassed(ctx context.Context) bool {
	v, _ := ctx.Value(coalesceBypassKey{}).(bool)
	return v
}

// coalescingSFN wraps SFNAPI, coalescing concurrent identical
// DescribeStateMachine calls (same StateMachineArn) into one underlying
// call. SFNAPI (sfn_interfaces.go) is already the complete aggregate of
// every SFN operation asserted anywhere in core/aws, so embedding it alone
// is sufficient — no narrower interface elsewhere needs a separate
// pass-through.
//
// Callers must treat the shared *sfn.DescribeStateMachineOutput as
// read-only: every current consumer (the four listed above) only reads
// from it, never mutates it, so sharing one pointer across concurrent
// callers is safe.
//
// This does not violate "transport carries no session state" (this
// package's own convention, e.g. client.go's header comment): a
// singleflight.Group retains nothing once its in-flight call completes —
// it is purely request-shape behavior, like RetryOnThrottle's backoff loop,
// not persisted state. A call made after the previous one finished always
// re-executes; there is no cache and no staleness.
type coalescingSFN struct {
	SFNAPI
	g singleflight.Group
}

// NewCoalescingSFN wraps api with in-flight DescribeStateMachine call
// coalescing. Exported for construction at the live client bootstrap
// (client.go) and from external tests; the concrete decorator type stays
// unexported.
func NewCoalescingSFN(api SFNAPI) SFNAPI {
	return &coalescingSFN{SFNAPI: api}
}

func (c *coalescingSFN) DescribeStateMachine(ctx context.Context, params *sfn.DescribeStateMachineInput, optFns ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	key := aws.ToString(params.StateMachineArn)
	if CoalesceBypassed(ctx) {
		// Explicit refresh: forget any in-flight call under this key so a
		// later concurrent caller starts a fresh flight instead of joining
		// the stale one, then call through directly — this call itself must
		// not join (or register as) a shared flight either.
		c.g.Forget(key)
		return c.SFNAPI.DescribeStateMachine(ctx, params, optFns...)
	}
	v, err, _ := c.g.Do(key, func() (any, error) {
		return c.SFNAPI.DescribeStateMachine(ctx, params, optFns...)
	})
	if err != nil {
		return nil, err
	}
	return v.(*sfn.DescribeStateMachineOutput), nil
}

// SNSFullAPI widens SNSAPI with the two SNS operations used only via a
// narrow type-assertion elsewhere in core/aws (catalog_messaging.go's sns
// and sns_subscriptions fetchers: SNSListTopicsAPI, SNSListSubscriptionsAPI)
// — SNSAPI itself does not embed them (sns_interfaces.go documents why: they
// are paginated, fetcher-only operations). Without this widening,
// coalescingSNS embedding only SNSAPI would satisfy SNSAPI but FAIL both of
// those assertions, breaking the sns and sns_subscriptions fetchers through
// the decorator. Exported alongside NewCoalescingSNS because it appears in
// that constructor's exported signature.
type SNSFullAPI interface {
	SNSAPI
	SNSListTopicsAPI
	SNSListSubscriptionsAPI
}

// coalescingSNS wraps SNSFullAPI, coalescing concurrent identical
// GetTopicAttributes calls (same TopicArn) into one underlying call —
// checkSNSKMS, checkSNSRole, and enrichSns each call it independently on
// every sns detail open.
//
// Callers must treat the shared *sns.GetTopicAttributesOutput as read-only:
// every current consumer only reads from it.
//
// Does not violate "transport carries no session state" — see
// coalescingSFN's doc comment; the same reasoning applies verbatim.
type coalescingSNS struct {
	SNSFullAPI
	g singleflight.Group
}

// NewCoalescingSNS wraps api with in-flight GetTopicAttributes call
// coalescing. Exported for construction at the live client bootstrap
// (client.go) and from external tests; the concrete decorator type stays
// unexported.
func NewCoalescingSNS(api SNSFullAPI) SNSFullAPI {
	return &coalescingSNS{SNSFullAPI: api}
}

func (c *coalescingSNS) GetTopicAttributes(ctx context.Context, params *sns.GetTopicAttributesInput, optFns ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	key := aws.ToString(params.TopicArn)
	if CoalesceBypassed(ctx) {
		// See coalescingSFN.DescribeStateMachine's bypass branch — same
		// explicit-refresh reasoning applies verbatim.
		c.g.Forget(key)
		return c.SNSFullAPI.GetTopicAttributes(ctx, params, optFns...)
	}
	v, err, _ := c.g.Do(key, func() (any, error) {
		return c.SNSFullAPI.GetTopicAttributes(ctx, params, optFns...)
	})
	if err != nil {
		return nil, err
	}
	return v.(*sns.GetTopicAttributesOutput), nil
}

// S3FullAPI widens S3API with the six S3 operations used only via a narrow
// type-assertion elsewhere in core/aws: S3GetBucketPolicyAPI (s3_related.go
// AND s3_detail_enrichment.go — this is the exact duplication this file
// fixes), S3GetBucketCorsAPI and S3GetBucketLifecycleAPI
// (s3_detail_enrichment.go), and S3GetBucketTaggingAPI,
// S3GetBucketEncryptionAPI, S3GetBucketLoggingAPI (s3_related.go). S3API
// itself does not embed any of them (s3_interfaces.go's per-interface
// comments say "not part of the S3API aggregate since no fetcher or related
// checker needs it" — true for Cors/Lifecycle when written, no longer true
// for GetBucketPolicy). Without this widening, coalescingS3 embedding only
// S3API would satisfy S3API but FAIL every one of those six assertions,
// breaking the s3 related panel and s3 detail enrichment entirely. Exported
// alongside NewCoalescingS3 because it appears in that constructor's
// exported signature.
type S3FullAPI interface {
	S3API
	S3GetBucketPolicyAPI
	S3GetBucketCorsAPI
	S3GetBucketLifecycleAPI
	S3GetBucketTaggingAPI
	S3GetBucketEncryptionAPI
	S3GetBucketLoggingAPI
}

// coalescingS3 wraps S3FullAPI, coalescing concurrent identical
// GetBucketPolicy calls (same Bucket) into one underlying call — the
// s3→role related checker (s3_related.go) and enrichS3
// (s3_detail_enrichment.go) each call it independently on every s3 detail
// open.
//
// Callers must treat the shared *s3.GetBucketPolicyOutput as read-only:
// every current consumer only reads from it.
//
// Does not violate "transport carries no session state" — see
// coalescingSFN's doc comment; the same reasoning applies verbatim.
type coalescingS3 struct {
	S3FullAPI
	g singleflight.Group
}

// NewCoalescingS3 wraps api with in-flight GetBucketPolicy call coalescing.
// Exported for construction at the live client bootstrap (client.go) and
// from external tests; the concrete decorator type stays unexported.
func NewCoalescingS3(api S3FullAPI) S3FullAPI {
	return &coalescingS3{S3FullAPI: api}
}

func (c *coalescingS3) GetBucketPolicy(ctx context.Context, params *s3.GetBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	key := aws.ToString(params.Bucket)
	if CoalesceBypassed(ctx) {
		// See coalescingSFN.DescribeStateMachine's bypass branch — same
		// explicit-refresh reasoning applies verbatim.
		c.g.Forget(key)
		return c.S3FullAPI.GetBucketPolicy(ctx, params, optFns...)
	}
	v, err, _ := c.g.Do(key, func() (any, error) {
		return c.S3FullAPI.GetBucketPolicy(ctx, params, optFns...)
	})
	if err != nil {
		return nil, err
	}
	return v.(*s3.GetBucketPolicyOutput), nil
}
