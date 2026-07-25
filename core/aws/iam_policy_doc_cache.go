// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_policy_doc_cache.go provides a session-scoped, concurrency-safe cache for
// decoded IAM policy documents. Owned by the session runtime (tui.Model's
// embedded sessionRuntime) and passed to detail enrichers via
// DetailEnrichmentCtx. Cache lifetime is tied to the session; profile/region
// rotation rebuilds the cache so entries from a previous account are not
// returned to the next one.
package aws

import (
	"github.com/k2m30/a9s/v3/core/domain"
)

// PolicyDocumentCache is a concurrency-safe cache for decoded IAM policy documents.
// Zero value is ready to use — no constructor needed.
//
// Cache keys use explicit prefixes to namespace by policy type:
//   - managed policies: "managed:<policyArn>"
//   - inline policies:  "inline:<roleName>/<policyName>"
//
// Unlike DetailDocCache, these keys are stable (never version-stamped), so
// SetIfNewer never needs the version-eviction bookkeeping opAwareDocStore
// offers — it always passes resource "".
type PolicyDocumentCache struct {
	opAwareDocStore
}

// Get returns the cached document for the given key, or nil if absent.
func (c *PolicyDocumentCache) Get(key string) any { return c.get(key) }

// Set stores a document in the cache, unconditionally — the op-oblivious
// write path every caller outside the on-demand detail-enrichment engine
// uses unchanged.
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
