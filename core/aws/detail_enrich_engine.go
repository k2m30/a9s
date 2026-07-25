// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_enrich_engine.go is the single generic engine behind every
// on-demand detail enricher (resource.DetailEnricher). Each enrichX function
// in this package is a thin wrapper: it builds a detailEnrichSpec describing
// its resource-specific unwrap/id/cache/fetch/wrap steps and calls
// enrichDetail. No enricher hand-rolls the assert/unwrap/cache/attach flow.
package aws

import (
	"context"
	"fmt"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/jsonyaml"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/trace"
)

// docCache is the minimal cache contract shared by PolicyDocumentCache and
// DetailDocCache — both already satisfy it via their existing Get/Set/
// SetIfNewer methods, so no adapter type is needed.
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

	// cache returns the session-scoped cache this enricher reads/writes, or
	// nil for an uncached enricher — cacheKey is never called in that case.
	cache func(*DetailEnrichmentCtx) docCache

	// cacheKey derives the cache key for a cached enricher from the id
	// validated above.
	cacheKey func(id string, r R, res resource.Resource) string

	// fetch is the only genuinely per-resource logic: narrow-interface
	// client capability asserts, RetryOnThrottle-wrapped API calls, and
	// payload transforms. id is the value id() already validated —
	// role_policies ignores it where its branching needs row fields instead.
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
	// transient state, not an error: silently skipping beats flashing
	// "enrich failed" at the operator over something that self-heals within
	// seconds (the first open after live rows land, or Ctrl+R, re-enriches
	// normally). Deriving a wrapper from res.ID alone was rejected — it
	// would embed a zero SDK struct and render misleading empty fields on
	// the YAML/JSON views instead of just deferring. This must be checked
	// before requiring dctx.Clients below: session caches (and so a valid
	// dctx) are constructed before the AWS clients are, so a pre-connect
	// open of a disk-seeded row has a non-nil dctx with a nil Clients — it
	// must hit this silent-skip, not the Clients error below.
	if res.RawStruct == nil {
		return res, nil
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

	var cacheKey string
	var cacheName string
	if cache != nil {
		cacheKey = spec.cacheKey(id, item, res)
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

	// Every coalescing decorator (core/aws/coalesce.go) keys its singleflight
	// group by this operation's ID: calls this enricher and its sibling
	// related checkers make while opening/refreshing THIS detail share
	// in-flight work, but an explicit refresh mints a brand-new operation ID
	// (core/runtime.Core.BeginDetailOperation), so it can never join a
	// pre-refresh call still in flight under the old one — no bypass call
	// needed.
	fetchCtx := WithDetailOp(ctx, dctx.OpID)
	payload, err := spec.fetch(fetchCtx, dctx.Clients, id, item, res)
	if err != nil {
		return res, err
	}

	if cache != nil {
		// Op-aware write (item 3, #261 boundary wave): dctx.OpID is 0 for a
		// caller with no active DetailOperation (Set's own semantics apply
		// unchanged); non-zero for a real operation, where a write from an
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
