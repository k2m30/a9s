// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_policy_doc_cache.go provides a session-scoped, concurrency-safe cache for
// decoded IAM policy documents. Owned by the session runtime (tui.Model's
// embedded sessionRuntime), passed to detail enrichers via
// DetailEnrichmentCtx and wired into ServiceClients for the Wave 2 issue
// pass. Cache lifetime is tied to the session; profile/region
// rotation rebuilds the cache so entries from a previous account are not
// returned to the next one.
package aws

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/k2m30/a9s/v3/core/domain"
)

// PolicyDocumentCache is a concurrency-safe cache for decoded IAM policy documents.
// Zero value is ready to use — no constructor needed.
//
// Cache keys use explicit prefixes to namespace by policy type:
//   - managed policies: "managed:<policyArn>"
//   - inline policies:  "inline:<roleName>/<policyName>", and
//     "inline:<user|group>/<name>/<policyName>" for a user's or a group's
//
// Unlike DetailDocCache, these keys are stable (never version-stamped), so
// SetIfNewer never needs the version-eviction bookkeeping opAwareDocStore
// offers — it always passes resource "".
type PolicyDocumentCache struct {
	opAwareDocStore
	flightOnce sync.Once
	// flight holds a *singleflight.Group. It is typed any because the
	// group's call records carry an error field, and govet's deepequalerrors
	// flags every reflect.DeepEqual over a value an error-typed field is
	// reachable from, ServiceClients included.
	flight any
}

func (c *PolicyDocumentCache) flights() *singleflight.Group {
	c.flightOnce.Do(func() { c.flight = &singleflight.Group{} })
	g, _ := c.flight.(*singleflight.Group)
	return g
}

// load returns key's document, calling read when the cache does not hold it.
// Concurrent misses of one key share one read, and the cache is consulted
// again inside the flight, so a read that finished between a caller's miss
// and its join is not repeated. A failed read is not kept: the next caller
// asks again. The shared read runs under the context of the caller that
// started it; a caller that joined it and is still live does not take that
// caller's cancellation as its own answer, and reads again, once.
func (c *PolicyDocumentCache) load(ctx context.Context, key string, read func(context.Context) (any, error)) (any, error) {
	for attempt := 0; ; attempt++ {
		if doc := c.get(key); doc != nil {
			return doc, nil
		}
		doc, err, shared := c.flights().Do(key, func() (any, error) {
			if doc := c.get(key); doc != nil {
				return doc, nil
			}
			doc, err := read(ctx)
			if err == nil {
				c.set(key, doc)
			}
			return doc, err
		})
		if attempt > 0 || !shared || ctx.Err() != nil || !isContextEnded(err) {
			return doc, err
		}
	}
}

func isContextEnded(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// Get returns the cached document for the given key, or nil if absent.
func (c *PolicyDocumentCache) Get(key string) any { return c.get(key) }

// Set stores a document in the cache, unconditionally — the op-oblivious
// write path every caller outside the on-demand detail-enrichment engine
// uses.
func (c *PolicyDocumentCache) Set(key string, doc any) { c.set(key, doc) }

// SetIfNewer stores doc under key per opAwareDocStore.setIfNewer's contract.
// Returns whether the write was accepted.
func (c *PolicyDocumentCache) SetIfNewer(key string, doc any, opID domain.Gen) bool {
	return c.setIfNewer(key, "", doc, opID)
}

// ManagedKey returns the cache key for a managed policy document.
func ManagedKey(policyArn string) string {
	return "managed:" + policyArn
}

// InlineKey returns the cache key for an inline policy document.
func InlineKey(roleName, policyName string) string {
	return "inline:" + roleName + "/" + policyName
}

// holderInlineKey is InlineKey for any holder kind. IAM names contain no
// "/", so a user's or a group's key has one segment more than any role's and
// the three never meet.
func holderInlineKey(kind, holder, policyName string) string {
	if kind == holderRole {
		return InlineKey(holder, policyName)
	}
	return "inline:" + kind + "/" + holder + "/" + policyName
}
