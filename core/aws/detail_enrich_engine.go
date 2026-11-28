// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_enrich_engine.go is the single generic engine behind every
// on-demand detail enricher (resource.DetailEnricher). Each enrichX function
// in this package is a thin wrapper: it builds a detailEnrichSpec describing
// its resource-specific unwrap/id/cache/fetch/wrap steps and calls
// enrichDetail. No enricher hand-rolls the assert/unwrap/cache/attach flow.
package aws

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/jsonyaml"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/trace"
)

// ErrDetailEnrichSkipped is the exact error enrichDetail returns (never
// wrapped, so errors.Is matches it directly) when it skips a disk-cache-seeded
// row with no live RawStruct yet — see the res.RawStruct == nil branch below.
// Distinguishes "there was nothing to do yet, this is not a failure" from a
// genuine fetch/cache error so core/runtime.HandleEnrichDetailResult can skip
// the user-facing flash AND — the reason this needs to be distinguishable at
// all — leave any pending sticky-refresh demand (session.PendingDetailRefresh)
// recorded rather than clearing it: an operation that fetched nothing must
// not be reported as having satisfied the refresh that demanded it, or a
// later refresh landing after live data finally arrives silently no-ops
// forever instead of retrying enrichment.
var ErrDetailEnrichSkipped = errors.New("detail enrichment skipped: resource has no live data yet")

// docCache is the minimal cache contract shared by PolicyDocumentCache and
// DetailDocCache; both satisfy it via their Get/Set/SetIfNewer methods.
type docCache interface {
	Get(key string) any
	Set(key string, doc any)
	// SetIfNewer stores doc under key unless opID is strictly older than the
	// opID a prior SetIfNewer call recorded for that key — see either
	// concrete type's own doc comment for the full contract. enrichDetail
	// uses this instead of bare Set so a stale, already-superseded
	// operation's late-arriving write can never overwrite the fresh entry a
	// newer operation (open, refresh) already wrote.
	SetIfNewer(key string, doc any, opID domain.Gen) bool
}

// policyDocsCache and detailDocsCache adapt DetailEnrichmentCtx's two
// concrete cache fields to docCache for use as a detailEnrichSpec.cache
// value, converting a nil *PolicyDocumentCache/*DetailDocCache into a true
// nil docCache — assigning a nil concrete pointer directly to an interface
// variable would produce a non-nil interface holding a nil pointer, which
// would defeat enrichDetail's `cache == nil` guard.
func policyDocsCache(dctx *DetailEnrichmentCtx) docCache {
	if dctx.PolicyDocs == nil {
		return nil
	}
	return dctx.PolicyDocs
}

func detailDocsCache(dctx *DetailEnrichmentCtx) docCache {
	if dctx.DetailDocs == nil {
		return nil
	}
	return dctx.DetailDocs
}

// cacheDisplayName names cache for the trace stream — the two concrete
// types docCache abstracts over are otherwise indistinguishable to
// enrichDetail once behind the interface.
func cacheDisplayName(cache docCache) string {
	switch cache.(type) {
	case *PolicyDocumentCache:
		return "policy"
	case *DetailDocCache:
		return "detail"
	default:
		return "unknown"
	}
}

// unwrapEnriched builds the standard two-case RawStruct unwrap: accept the
// raw SDK type R directly, or the enricher's own wrapper W (re-enrichment
// fires on detail→YAML→JSON, which each re-trigger enrichment), extracting
// R from a W via inner.
func unwrapEnriched[R, W any](inner func(W) R) func(any) (R, bool) {
	return func(raw any) (R, bool) {
		if v, ok := raw.(R); ok {
			return v, true
		}
		if w, ok := raw.(W); ok {
			return inner(w), true
		}
		var zero R
		return zero, false
	}
}

// detailEnrichSpec describes one on-demand detail enricher's per-resource
// behavior. R is the resource's SDK RawStruct type; P is the fetched (or
// cached) payload type wrap attaches to it.
type detailEnrichSpec[R any, P any] struct {
	// unwrap extracts R from res.RawStruct, accepting both the original SDK
	// type and the enricher's own wrapper (re-enrichment fires on
	// detail→YAML→JSON, which each re-trigger enrichment).
	unwrap func(any) (R, bool)

	// id validates the resource carries what fetch needs, returning the
	// enricher's exact missing-identifier error when it doesn't, and the
	// identifier itself on success — threaded through to cacheKey/fetch so
	// they don't need to re-derive it from R's pointer fields. May read
	// res.Fields (role_policies keys its inline-policy identity off
	// Fields["role_name"], not anything on R).
	id func(R, resource.Resource) (string, error)

	// answer, when set, reads the item again through its Get call, and the
	// members the list call of listType leaves out (that type's ListOmits)
	// are taken from it; every other member stays as the row listed it. The
	// answer is cached under "answer:" and cacheKey when the enricher is
	// cached. nil leaves the listed item as it is.
	answer   func(ctx context.Context, c *ServiceClients, id string) (*R, error)
	listType string

	// cache returns the session-scoped cache this enricher reads/writes, or
	// nil for an uncached enricher — cacheKey is never called in that case.
	cache func(*DetailEnrichmentCtx) docCache

	// cacheKey derives the cache key for a cached enricher from the id
	// validated above.
	cacheKey func(id string, r R, res resource.Resource) string

	// fetch is the only genuinely per-resource logic: narrow-interface
	// client capability asserts, API calls, and
	// payload transforms. id is the value id() already validated —
	// role_policies ignores it where its branching needs row fields instead.
	// nil for an enricher whose answer is all it reads: wrap gets P's zero.
	fetch func(ctx context.Context, c *ServiceClients, id string, r R, res resource.Resource) (P, error)

	// wrap builds the enriched RawStruct from the unwrapped item and the
	// fetched (or cached) payload.
	wrap func(R, P) any
}

// enrichDetail runs the shared on-demand detail-enrichment flow: assert the
// session's *DetailEnrichmentCtx (and its required cache, if any), unwrap
// the resource's RawStruct, validate its identifier, consult the cache on a
// hit, fetch on a miss, populate the cache, and attach the wrapped result to
// res.RawStruct.
func enrichDetail[R, P any](ctx context.Context, clients any, res resource.Resource, spec detailEnrichSpec[R, P]) (resource.Resource, error) {
	dctx, ok := clients.(*DetailEnrichmentCtx)
	if !ok || dctx == nil {
		return res, fmt.Errorf("invalid detail-enrichment context")
	}

	var cache docCache
	if spec.cache != nil {
		cache = spec.cache(dctx)
		if cache == nil {
			return res, fmt.Errorf("invalid detail-enrichment context")
		}
	}

	// A disk-cache-seeded row carries Fields only (cache.Row has no
	// RawStruct field) until the live refetch lands. That's a documented
	// transient state, not a user-facing error: ErrDetailEnrichSkipped beats
	// flashing "enrich failed" at the operator over something that self-heals
	// within seconds (the first open after live rows land, or Ctrl+R,
	// re-enriches normally) — but it must still be a non-nil, distinguishable
	// error rather than nil, so the caller (core/runtime.HandleEnrichDetailResult)
	// can tell "nothing happened yet" apart from "this genuinely succeeded"
	// and leave any pending sticky-refresh demand recorded instead of
	// consuming it for an operation that fetched nothing. A wrapper derived
	// from res.ID alone would embed a zero SDK struct and render misleading
	// empty fields on the YAML/JSON views instead of just deferring.
	// This must be checked before requiring dctx.Clients below:
	// session caches (and so a valid dctx) are constructed before the AWS
	// clients are, so a pre-connect open of a disk-seeded row has a non-nil
	// dctx with a nil Clients — it must hit this skip, not the Clients error
	// below.
	if res.RawStruct == nil {
		return res, ErrDetailEnrichSkipped
	}

	if dctx.Clients == nil {
		return res, fmt.Errorf("invalid detail-enrichment context")
	}

	item, ok := spec.unwrap(res.RawStruct)
	if !ok {
		return res, fmt.Errorf("unexpected RawStruct type: %T", res.RawStruct)
	}

	id, err := spec.id(item, res)
	if err != nil {
		return res, err
	}

	// Every coalescing decorator (core/aws/coalesce.go) keys its singleflight
	// group by this operation's ID: calls this enricher and its sibling
	// related checkers make while opening/refreshing THIS detail share
	// in-flight work, but an explicit refresh mints a brand-new operation ID
	// (core/runtime.Core.BeginDetailOperation), so it can never join a
	// pre-refresh call still in flight under the old one.
	fetchCtx := WithDetailOp(ctx, dctx.OpID)
	var cacheKey string
	if cache != nil {
		cacheKey = spec.cacheKey(id, item, res)
	}
	if spec.answer != nil {
		got, answerErr := cachedAnswer(dctx, cache, "answer:"+cacheKey, func() (*R, error) {
			return spec.answer(fetchCtx, dctx.Clients, id)
		})
		if answerErr != nil {
			return res, answerErr
		}
		var omits []string
		if td := catalog.FindAny(spec.listType); td != nil {
			omits = td.ListOmits
		}
		item = takeOmitted(item, got, omits)
	}
	if spec.fetch == nil {
		var none P
		res.RawStruct = spec.wrap(item, none)
		return res, nil
	}

	var cacheName string
	if cache != nil {
		cacheName = cacheDisplayName(cache)
		// dctx.SkipCache (set for an explicit refresh) bypasses only the
		// READ — cacheKey is still computed and the fetch result below is
		// still WRITTEN under it, so a subsequent non-refresh open benefits
		// from the freshly-refreshed entry.
		if !dctx.SkipCache {
			if cached, hit := cache.Get(cacheKey).(P); hit {
				if trace.Enabled() {
					trace.Emit(trace.Event{Kind: trace.KindCache, OperationID: uint64(dctx.OpID), Cache: cacheName, Key: cacheKey, CacheOutcome: "hit"})
				}
				res.RawStruct = spec.wrap(item, cached)
				return res, nil
			}
		}
		if trace.Enabled() {
			reason := ""
			if dctx.SkipCache {
				reason = "refresh-skip"
			}
			trace.Emit(trace.Event{Kind: trace.KindCache, OperationID: uint64(dctx.OpID), Cache: cacheName, Key: cacheKey, CacheOutcome: "miss", Reason: reason})
		}
	}

	payload, err := spec.fetch(fetchCtx, dctx.Clients, id, item, res)
	if err != nil {
		// A partial payload is what was read; it is shown with the error and
		// never cached, so the next open asks again for what failed.
		if _, partial := errors.AsType[domain.PartialDetailErr](err); partial {
			res.RawStruct = spec.wrap(item, payload)
		}
		return res, err
	}

	if cache != nil {
		// Op-aware write: dctx.OpID is 0 for a caller with no active
		// DetailOperation (Set's own semantics apply); non-zero for a
		// real operation, where a write from an
		// operation strictly older than the recorded writer is refused rather
		// than silently overwriting a fresher entry a newer operation (or an
		// explicit refresh) already wrote.
		accepted := cache.SetIfNewer(cacheKey, payload, dctx.OpID)
		if trace.Enabled() {
			reason := "accepted"
			if !accepted {
				reason = "refused-stale-writer"
			}
			trace.Emit(trace.Event{Kind: trace.KindCache, OperationID: uint64(dctx.OpID), Cache: cacheName, Key: cacheKey, CacheOutcome: "write", Reason: reason})
		}
	}

	res.RawStruct = spec.wrap(item, payload)
	return res, nil
}

// cachedAnswer is the Get answer read once per session under key, as the
// payload beside it is: an explicit refresh reads it again, and a failed read
// is never stored. An uncached enricher reads it every time.
func cachedAnswer[R any](dctx *DetailEnrichmentCtx, cache docCache, key string, read func() (R, error)) (R, error) {
	if cache != nil && !dctx.SkipCache {
		if got, hit := cache.Get(key).(R); hit {
			return got, nil
		}
	}
	got, err := read()
	if err == nil && cache != nil {
		cache.SetIfNewer(key, got, dctx.OpID)
	}
	return got, err
}

// parseJSONOrRaw attempts to json.Unmarshal s into a generic value, falling
// back to the raw string when it doesn't parse as JSON — the shared idiom
// behind sfn definitions, cfn templates, sns attribute values, and s3
// policies: all prefer structured YAML/JSON rendering but must still
// degrade to something copyable when the payload isn't JSON (e.g. a CFN
// template authored as YAML).
func parseJSONOrRaw(s string) any {
	parsed, ok := jsonyaml.ParseStrict(s)
	if !ok {
		return s
	}
	return parsed
}

// takeOmitted is listed with the members named in omits taken from got, the
// Get call's answer: the list call never returns those, and every other
// member is what the row listed, so the detail and the row agree on it.
func takeOmitted[T any](listed T, got *T, omits []string) T {
	if got == nil {
		return listed
	}
	out := listed
	dst, src := reflect.ValueOf(&out).Elem(), reflect.ValueOf(got).Elem()
	for _, name := range omits {
		dst.FieldByName(name).Set(src.FieldByName(name))
	}
	return out
}
