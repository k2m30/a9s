// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// coalesce.go wraps the four AWS clients whose narrow related-checker AND
// on-demand detail-enricher independently call the identical operation on
// the identical resource on every detail open: sfn's DescribeStateMachine
// (checkSFNRole + checkSFNKMS + checkSFNLambda + enrichSfn — up to 4x
// concurrently), sns's GetTopicAttributes (checkSNSKMS + checkSNSRole +
// enrichSns), s3's GetBucketPolicy (the s3→role related checker + enrichS3),
// and lambda's GetFunction (checkLambdaECR + enrichLambda).
// golang.org/x/sync/singleflight coalesces concurrent identical
// in-flight calls into one underlying call and shares its result across
// every caller waiting on that key. Layered on top, completedResultMemo
// (below) retains a completed operation-scoped call's result so a caller
// arriving AFTER the flight already finished — e.g. the web drain, which
// runs the related-checker batch to completion before dispatching
// enrichment, releasing the singleflight entry in between — reuses it
// instead of re-fetching: within one operation, a key is fetched at most
// once, ever, independent of whether callers happened to overlap in time.
// opID == 0 (no active DetailOperation) bypasses the memo entirely, keeping
// every non-operation caller's exact pre-existing re-fetch-every-time
// semantics.
//
// Every decorator keys its singleflight group (and its completedResultMemo)
// as the active core/runtime.DetailOperation's ID (WithDetailOp) plus the
// existing per-call key: the enricher and its related checkers, opened
// together under one operation, share one in-flight call AND one memoized
// result per underlying API key; a refresh begins a brand-new operation — a
// new ID, a new namespace — so it is structurally unable to join whatever
// pre-refresh call is still in flight, or reuse whatever pre-refresh result
// is memoized, under the old ID. No Forget call is needed for this: the
// namespaces simply never collide.
//
// The memo lookup and the singleflight join/execute decision are folded into
// one shared helper, coalesceCall (below the four decorator types) — see its
// doc comment for why a caller can never observe "no memo, no in-flight
// entry" while a result for its key actually exists, without needing a
// second lock.
//
// Wired only at the live client bootstrap (CreateServiceClients, below in
// this package). Demo mode's fakes (core/demo/client.go) are instant,
// in-process, and deterministic — coalescing them would add complexity for
// zero benefit, so they stay undecorated (core/demo/client.go overwrites
// these four fields with typed fakes immediately after calling
// CreateServiceClients, discarding whatever real client this file
// constructed).
//
// The four constructors are exported (returning the widest interface, not
// the concrete decorator type) so both the live bootstrap and external
// tests (tests/unit, an exported-symbols-only package) can construct one —
// the decorator struct types themselves stay unexported implementation
// detail.
//
// Each decorator also records every call it makes (executed vs served from
// the memo) to two independent, off-by-default observers: an optional
// *CallLedger (call_ledger.go) a test opts into via the ...WithLedger
// constructor variant, and the process-wide core/trace stream (on only when
// trace.Enabled(), e.g. cmd/a9s's --trace flag). Neither adds a lock or an
// allocation on the live bootstrap's undecorated path.
package aws

import (
	"container/list"
	"context"
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/trace"

	"golang.org/x/sync/singleflight"
)

// detailOpKey is the unexported context-key type for
// WithDetailOp/DetailOpFromContext — unexported so no other package can
// collide with or forge the marker.
type detailOpKey struct{}

// WithDetailOp marks ctx with id, the active core/runtime.DetailOperation's
// ID, so every coalescing decorator below can key its singleflight group
// per-operation via DetailOpFromContext. Every AWS call made on behalf of
// one operation — its detail enricher and every one of its related
// checkers — should be wrapped with the SAME id, so they coalesce with each
// other; a call made under a later operation (a fresh detail open, or an
// explicit Ctrl+R refresh, both of which mint a new ID via
// core/runtime.Core.BeginDetailOperation) can never join it.
func WithDetailOp(ctx context.Context, id domain.Gen) context.Context {
	return context.WithValue(ctx, detailOpKey{}, id)
}

// DetailOpFromContext returns the operation ID WithDetailOp attached to ctx,
// or 0 when none was attached (no operation — coalesces only against other
// same-key calls that also carry no operation).
func DetailOpFromContext(ctx context.Context) domain.Gen {
	id, _ := ctx.Value(detailOpKey{}).(domain.Gen)
	return id
}

// coalesceKey renders the ctx's operation ID (DetailOpFromContext) and the
// caller's own per-call key into the one singleflight key every decorator
// below uses, so two calls only coalesce when both the operation AND the
// underlying resource identifier match.
func coalesceKey(ctx context.Context, key string) string {
	return strconv.FormatUint(uint64(DetailOpFromContext(ctx)), 10) + "/" + key
}

// maxCompletedResultMemoEntries bounds each decorator's completedResultMemo.
// 128 comfortably covers every key a realistic session touches concurrently
// (a handful of open/refreshed detail operations, one key each); an
// abandoned operation's entries age out under normal LRU eviction pressure
// from newer operations rather than via any explicit per-operation cleanup.
const maxCompletedResultMemoEntries = 128

// completedResultMemo is a small, thread-safe, bounded LRU that retains one
// AWS call's result (value AND error — see coalescedResult) per coalesceKey
// (opID + underlying resource key) after its singleflight flight completes.
//
// singleflight.Group alone only coalesces CONCURRENT calls sharing a key —
// once the in-flight call finishes, the group forgets it, so a call arriving
// AFTER completion always re-executes. The web drain runs sequentially (it
// drains the related-checker batch to completion before dispatching
// enrichment, releasing the singleflight entry in between), so sfn/sns/s3's
// related checker and its detail enricher — both wanting the identical AWS
// call for the identical operation — fetch it twice, possibly observing two
// different snapshots of the same resource. This memo closes that gap:
// within one operation a key is fetched at most once, EVER, not merely while
// concurrent callers happen to overlap — coalesceCall's inner recheck
// (below) is what makes that guarantee hold even when a caller's OWN check
// races a flight's completion, not just when it races a flight's start.
//
// Per-decorator (one instance alongside each decorator's own
// singleflight.Group) rather than shared across sfn/sns/s3: the underlying
// per-call keys (StateMachineArn, TopicArn, Bucket) are arbitrary strings
// with no type tag, so a shared memo could theoretically collide two
// different resource kinds' identically-spelled keys under the same
// operation. One memo per decorator, exactly mirroring its own `g` field,
// makes that collision structurally impossible instead of merely unlikely.
//
// opID == 0 (no active DetailOperation — a caller outside any detail
// open/refresh) is NEVER memoized: only a real operation's calls are
// deduplicated across its own lifetime, so every non-operation flow keeps
// its exact pre-existing re-fetch-every-time semantics. A failed fetch is
// also never memoized unless the caller's shouldMemoize predicate says
// otherwise (S3's benign NoSuchBucketPolicy) — an AWS error is otherwise
// retryable on the next call within the same operation, not permanently
// pinned to a transient failure for the operation's remaining lifetime.
type completedResultMemo struct {
	mu    sync.Mutex
	index map[string]*list.Element
	order *list.List
}

type completedResultMemoItem struct {
	key   string
	value any
}

func newCompletedResultMemo() *completedResultMemo {
	return &completedResultMemo{
		index: make(map[string]*list.Element),
		order: list.New(),
	}
}

// get retrieves the memoized value for key, promoting it to
// most-recently-used. ok is false on a miss.
func (m *completedResultMemo) get(key string) (v any, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	el, ok := m.index[key]
	if !ok {
		return nil, false
	}
	m.order.MoveToFront(el)
	return el.Value.(*completedResultMemoItem).value, true
}

// set stores value for key, evicting the least-recently-used entry if the
// insert would exceed maxCompletedResultMemoEntries.
func (m *completedResultMemo) set(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if el, ok := m.index[key]; ok {
		m.order.MoveToFront(el)
		el.Value.(*completedResultMemoItem).value = value
		return
	}
	el := m.order.PushFront(&completedResultMemoItem{key: key, value: value})
	m.index[key] = el
	if m.order.Len() > maxCompletedResultMemoEntries {
		back := m.order.Back()
		if back != nil {
			m.order.Remove(back)
			delete(m.index, back.Value.(*completedResultMemoItem).key)
		}
	}
}

// coalescingSFN wraps SFNAPI, coalescing concurrent identical
// DescribeStateMachine calls (same operation, same StateMachineArn) into one
// underlying call. SFNAPI (sfn_interfaces.go) is already the complete
// aggregate of every SFN operation asserted anywhere in core/aws, so
// embedding it alone is sufficient — no narrower interface elsewhere needs a
// separate pass-through.
//
// Callers must treat the shared *sfn.DescribeStateMachineOutput as
// read-only: every current consumer (the four listed above) only reads
// from it, never mutates it, so sharing one pointer across concurrent
// callers is safe.
//
// This does not violate "transport carries no session state" (this
// package's own convention, e.g. client.go's header comment): singleflight.Group
// retains nothing once its in-flight call completes, and memo (below) is
// keyed by operation ID, so a superseded operation's entries are never
// looked up again by any caller — no code path re-attaches an old operation's
// ID to a new call — and simply age out under normal LRU eviction pressure
// from newer entries (completedResultMemo, bounded). A call made with no
// active operation (opID == 0), or under any operation other than the one
// that produced the memoized entry, always re-executes; this is targeted,
// bounded, per-operation memoization, not a general-purpose cache.
type coalescingSFN struct {
	SFNAPI
	g      singleflight.Group
	memo   *completedResultMemo
	ledger *CallLedger
}

// NewCoalescingSFN wraps api with in-flight DescribeStateMachine call
// coalescing. Exported for construction at the live client bootstrap
// (client.go) and from external tests; the concrete decorator type stays
// unexported.
func NewCoalescingSFN(api SFNAPI) SFNAPI {
	return NewCoalescingSFNWithLedger(api, nil)
}

// NewCoalescingSFNWithLedger is NewCoalescingSFN plus an explicit
// *CallLedger (nil behaves identically to NewCoalescingSFN) — the test
// harness's opt-in to per-call recording, additive over the production
// constructor above.
func NewCoalescingSFNWithLedger(api SFNAPI, ledger *CallLedger) SFNAPI {
	return &coalescingSFN{SFNAPI: api, memo: newCompletedResultMemo(), ledger: ledger}
}

func (c *coalescingSFN) DescribeStateMachine(ctx context.Context, params *sfn.DescribeStateMachineInput, optFns ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	return coalesceCall(ctx, &c.g, c.memo, c.ledger, "sfn.DescribeStateMachine", aws.ToString(params.StateMachineArn),
		func() (*sfn.DescribeStateMachineOutput, error) {
			return c.SFNAPI.DescribeStateMachine(ctx, params, optFns...)
		},
		alwaysMemoizeSuccess[*sfn.DescribeStateMachineOutput],
	)
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
// GetTopicAttributes calls (same operation, same TopicArn) into one
// underlying call — checkSNSKMS, checkSNSRole, and enrichSns each call it
// independently on every sns detail open.
//
// Callers must treat the shared *sns.GetTopicAttributesOutput as read-only:
// every current consumer only reads from it.
//
// Does not violate "transport carries no session state" — see
// coalescingSFN's doc comment; the same reasoning applies verbatim.
type coalescingSNS struct {
	SNSFullAPI
	g      singleflight.Group
	memo   *completedResultMemo
	ledger *CallLedger
}

// NewCoalescingSNS wraps api with in-flight GetTopicAttributes call
// coalescing. Exported for construction at the live client bootstrap
// (client.go) and from external tests; the concrete decorator type stays
// unexported.
func NewCoalescingSNS(api SNSFullAPI) SNSFullAPI {
	return NewCoalescingSNSWithLedger(api, nil)
}

// NewCoalescingSNSWithLedger is NewCoalescingSNS plus an explicit
// *CallLedger (nil behaves identically to NewCoalescingSNS) — the test
// harness's opt-in to per-call recording, additive over the production
// constructor above.
func NewCoalescingSNSWithLedger(api SNSFullAPI, ledger *CallLedger) SNSFullAPI {
	return &coalescingSNS{SNSFullAPI: api, memo: newCompletedResultMemo(), ledger: ledger}
}

func (c *coalescingSNS) GetTopicAttributes(ctx context.Context, params *sns.GetTopicAttributesInput, optFns ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	return coalesceCall(ctx, &c.g, c.memo, c.ledger, "sns.GetTopicAttributes", aws.ToString(params.TopicArn),
		func() (*sns.GetTopicAttributesOutput, error) {
			return c.SNSFullAPI.GetTopicAttributes(ctx, params, optFns...)
		},
		alwaysMemoizeSuccess[*sns.GetTopicAttributesOutput],
	)
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
// GetBucketPolicy calls (same operation, same Bucket) into one underlying
// call — the s3→role related checker (s3_related.go) and enrichS3
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
	g      singleflight.Group
	memo   *completedResultMemo
	ledger *CallLedger
}

// NewCoalescingS3 wraps api with in-flight GetBucketPolicy call coalescing.
// Exported for construction at the live client bootstrap (client.go) and
// from external tests; the concrete decorator type stays unexported.
func NewCoalescingS3(api S3FullAPI) S3FullAPI {
	return NewCoalescingS3WithLedger(api, nil)
}

// NewCoalescingS3WithLedger is NewCoalescingS3 plus an explicit *CallLedger
// (nil behaves identically to NewCoalescingS3) — the test harness's opt-in
// to per-call recording, additive over the production constructor above.
func NewCoalescingS3WithLedger(api S3FullAPI, ledger *CallLedger) S3FullAPI {
	return &coalescingS3{S3FullAPI: api, memo: newCompletedResultMemo(), ledger: ledger}
}

func (c *coalescingS3) GetBucketPolicy(ctx context.Context, params *s3.GetBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	return coalesceCall(ctx, &c.g, c.memo, c.ledger, "s3.GetBucketPolicy", aws.ToString(params.Bucket),
		func() (*s3.GetBucketPolicyOutput, error) {
			return c.S3FullAPI.GetBucketPolicy(ctx, params, optFns...)
		},
		// A benign NoSuchBucketPolicy (most buckets have no policy) is a
		// definitive, memoizable answer for the rest of the operation, not a
		// transient failure — the only one of the four decorators whose
		// shouldMemoize predicate accepts a non-nil error.
		func(_ *s3.GetBucketPolicyOutput, err error) bool {
			return err == nil || s3BenignAbsenceErr(err, "NoSuchBucketPolicy")
		},
	)
}

// coalescingLambda wraps LambdaAPI, coalescing concurrent identical
// GetFunction calls (same operation, same FunctionName) into one underlying
// call — checkLambdaECR (lambda_related.go) and enrichLambda
// (lambda_detail_enrichment.go) each call it independently on every Image-
// package-type Lambda detail open. LambdaAPI is already the complete
// aggregate of every Lambda operation asserted anywhere in core/aws
// (LambdaGetFunctionAPI, LambdaListEventSourceMappingsAPI — apigw_related.go,
// related_common.go, secrets_related_extra.go), so embedding it alone is
// sufficient — no narrower interface elsewhere needs a separate pass-through,
// unlike SNSFullAPI/S3FullAPI's widening.
//
// Callers must treat the shared *lambda.GetFunctionOutput as read-only:
// every current consumer only reads from it.
//
// Does not violate "transport carries no session state" — see
// coalescingSFN's doc comment; the same reasoning applies verbatim. No
// benign-absence memoization (contrast coalescingS3): a Lambda function
// disappearing out from under an open detail view is a genuine anomaly, not
// a common, expected state the way an S3 bucket having no policy is — so
// GetFunction errors are never memoized, exactly like SFN/SNS.
type coalescingLambda struct {
	LambdaAPI
	g      singleflight.Group
	memo   *completedResultMemo
	ledger *CallLedger
}

// NewCoalescingLambda wraps api with in-flight GetFunction call coalescing.
// Exported for construction at the live client bootstrap (client.go) and
// from external tests; the concrete decorator type stays unexported.
func NewCoalescingLambda(api LambdaAPI) LambdaAPI {
	return NewCoalescingLambdaWithLedger(api, nil)
}

// NewCoalescingLambdaWithLedger is NewCoalescingLambda plus an explicit
// *CallLedger (nil behaves identically to NewCoalescingLambda) — the test
// harness's opt-in to per-call recording, additive over the production
// constructor above.
func NewCoalescingLambdaWithLedger(api LambdaAPI, ledger *CallLedger) LambdaAPI {
	return &coalescingLambda{LambdaAPI: api, memo: newCompletedResultMemo(), ledger: ledger}
}

func (c *coalescingLambda) GetFunction(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	return coalesceCall(ctx, &c.g, c.memo, c.ledger, "lambda.GetFunction", aws.ToString(params.FunctionName),
		func() (*lambda.GetFunctionOutput, error) {
			return c.LambdaAPI.GetFunction(ctx, params, optFns...)
		},
		alwaysMemoizeSuccess[*lambda.GetFunctionOutput],
	)
}

// alwaysMemoizeSuccess is the shouldMemoize predicate shared by every
// decorator except coalescingS3: memoize exactly when the call succeeded,
// exactly the pre-existing `err == nil` gate each one used before this file
// was collapsed onto coalesceCall.
func alwaysMemoizeSuccess[T any](_ T, err error) bool {
	return err == nil
}

// coalescedResult is the (value, error) pair completedResultMemo retains per
// coalesceCall instantiation — carrying both halves lets a definitive,
// memoizable error (S3's benign NoSuchBucketPolicy) be stored exactly like a
// successful value, without each decorator inventing its own error-carrying
// wrapper type.
type coalescedResult[T any] struct {
	out T
	err error
}

// recordCoalesceCall records one call outcome to the ledger (nil-receiver
// safe, a no-op when the caller opted out) and, when enabled, to the
// process-wide trace stream.
func recordCoalesceCall(ledger *CallLedger, opID domain.Gen, api, argKey string, executed bool) {
	outcome, outcomeStr := CallServed, "served"
	if executed {
		outcome, outcomeStr = CallExecuted, "executed"
	}
	ledger.record(opID, api, argKey, outcome)
	if trace.Enabled() {
		trace.Emit(trace.Event{Kind: trace.KindAWSCall, OperationID: uint64(opID), API: api, Args: argKey, Outcome: outcomeStr})
	}
}

// coalesceCall is the single "check memo → join-or-become-leader →
// re-check memo → execute at most once" sequence every coalescing
// decorator's method reduces to: call is the underlying AWS invocation,
// shouldMemoize decides whether a (value, err) pair is a safe permanent
// answer for the rest of the operation to reuse.
//
// The race this closes: with completedResultMemo.get and singleflight.Group.Do
// as two independent steps, a caller could miss the memo and then, before it
// reached Do, observe an entirely
// different goroutine's flight for the SAME key run to completion (execute
// the real call, record it, populate the memo, return from its closure, and
// have singleflight delete its now-finished in-flight entry) inside that
// gap. Arriving at Do afterward, the caller finds neither a memo hit nor an
// in-flight entry to join, so Do hands it a brand-new leadership — and,
// without a recheck, it blindly re-executes the identical call singleflight
// and the memo both exist to prevent, even though the answer was already
// sitting in the memo the entire time. Each primitive is individually
// correctly locked, so -race never flags this; it is a logic-level
// check-then-act race between two synchronization primitives that do not
// share a critical section.
//
// The fix does not add a second lock. Do already guarantees at most one
// closure runs per key at any moment, and every OTHER concurrent caller for
// that key is either blocked waiting to share this closure's result or has
// not reached Do yet at all — so re-checking the memo as the very first
// thing INSIDE the closure (i.e. only once Do has made this goroutine the
// sole, serialized leader for the key) observes every prior leader's
// memo.set with a real happens-before edge, via the memo's own mutex. That
// is the entire fix: fold the memo recheck into the singleflight closure's
// leadership window, rather than performing it before a leadership decision
// has even been made. A per-key mutex (guarding the outer check and Do as
// one critical section) would achieve the same serialization but requires
// its own lifecycle (creating, sharing, and eventually discarding one mutex
// per key) for a guarantee Do already provides for free.
func coalesceCall[T any](
	ctx context.Context,
	g *singleflight.Group,
	memo *completedResultMemo,
	ledger *CallLedger,
	api string,
	argKey string,
	call func() (T, error),
	shouldMemoize func(T, error) bool,
) (T, error) {
	opID := DetailOpFromContext(ctx)
	key := coalesceKey(ctx, argKey)

	memoHit := func() (coalescedResult[T], bool) {
		if opID == 0 {
			return coalescedResult[T]{}, false
		}
		v, ok := memo.get(key)
		if !ok {
			return coalescedResult[T]{}, false
		}
		return v.(coalescedResult[T]), true
	}

	if r, ok := memoHit(); ok {
		recordCoalesceCall(ledger, opID, api, argKey, false)
		return r.out, r.err
	}

	var executed bool
	v, _, shared := g.Do(key, func() (any, error) {
		// See coalesceCall's doc comment: this recheck, not the one above,
		// is what closes the race — it runs only once Do has granted this
		// goroutine sole leadership for key.
		if r, ok := memoHit(); ok {
			recordCoalesceCall(ledger, opID, api, argKey, false)
			return r, r.err
		}
		executed = true
		out, callErr := call()
		recordCoalesceCall(ledger, opID, api, argKey, true)
		r := coalescedResult[T]{out: out, err: callErr}
		if opID != 0 && shouldMemoize(out, callErr) {
			memo.set(key, r)
		}
		return r, callErr
	})
	if shared && !executed {
		// Joined another goroutine's already-in-flight identical call: no
		// second AWS request, so this caller's ask is recorded too, as
		// served — the ledger must be able to see BOTH askers to prove
		// dedup happened, not just infer it from a single executed entry.
		recordCoalesceCall(ledger, opID, api, argKey, false)
	}
	r := v.(coalescedResult[T])
	return r.out, r.err
}
