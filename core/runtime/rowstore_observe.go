// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// rowstore_observe.go — RowStore dual-write for the two events HandleEvent
// must never apply intents for (task #17 wave 1 — row-store unification).
//
// messages.ResourcesLoaded and messages.RelatedCheckResult are each already
// owned by a dedicated, intent-returning Core method
// (HandleResourcesLoaded / HandleRelatedCheckResult) that existing callers
// (the TUI adapter, Controller.Handle) invoke directly — NOT through
// HandleEvent. Wiring HandleEvent's switch to call those methods for these
// two message types would make Controller.Handle (core/app/handle.go,
// out of this stage's scope) apply the same intents a second time via its
// existing, separate ResourcesLoaded/RelatedCheckResult pipeline — a real
// behavior change. This file gives HandleEvent's ResourcesLoaded/
// RelatedCheckResult cases a session-mutation-only path (RowStore dual-write,
// no intents/tasks) so a generic HandleEvent caller (this stage's
// differential-harness pin, a future headless caller) sees the store update
// without touching core/app-owned intent application.
package runtime

import (
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// observeResourcesLoadedRows feeds RowStore the same canonicalization +
// Fetch-origin write HandleResourcesLoaded's ProbeResources reseed performs,
// without applying any of HandleResourcesLoaded's intents/tasks. Only
// msg.Provenance.CanonicalList() is eligible: a filtered drill, a by-ID
// lookup, or a child fetch shares this same message shape but is never the
// type's global population, and must not replace (append=false) or extend
// (append=true) the shared per-type RowStore entry a canonical fetch owns.
func (c *Core) observeResourcesLoadedRows(msg messages.ResourcesLoaded) {
	if msg.ResourceType == "" || !msg.Provenance.CanonicalList() {
		return
	}
	canon := msg.ResourceType
	if td := resource.FindResourceType(msg.ResourceType); td != nil {
		canon = td.ShortName
	}
	c.ObserveRows(canon, msg.Resources, msg.Pagination, session.OriginFetch, msg.Append)
}

// observeRelatedCheckResultRows feeds RowStore the same CachedPages
// (full first-page results, Fetch-origin) and LazyAddedResources (sparse
// FetchByIDs adds, Partial) dual-write HandleRelatedCheckResult's intents
// would otherwise apply via PatchResourceCache/PatchLazyResourceCache.
func (c *Core) observeRelatedCheckResultRows(msg messages.RelatedCheckResult) {
	for aliasName, entry := range msg.CachedPages {
		canon := resource.CanonicalShortName(aliasName)
		c.ObserveRows(canon, entry.Resources, resolveCachedPagePagination(entry), session.OriginFetch, false)
	}
	for aliasName, extra := range msg.LazyAddedResources {
		if len(extra) == 0 {
			continue
		}
		canon := resource.CanonicalShortName(aliasName)
		c.ObservePartialRows(canon, extra)
	}
}

// resolveCachedPagePagination derives the PaginationMeta to store for one
// RelatedCheckResult CachedPages entry: entry.Pagination when present, or a
// synthesized {IsTruncated: true} when the entry reports truncation without
// carrying pagination detail (e.g. a related-checker's NeedsTargetCache
// prefetch that only inspected a first page). Truncation is a UNION of
// entry.IsTruncated and entry.Pagination.IsTruncated, never a downgrade —
// matching this codebase's standing rule that a truncation signal only ever
// strengthens (C5's nil-Pagination-is-conservatively-truncated, RowStore's
// refusal to replace an exact entry with a stale truncated subset, the
// availability path's refusal to promote an unobserved count to Exact): a
// future constructor that ever sets the two fields independently must not
// have this silently turn a true "N+" into a confident, wrong "N". Copies
// the struct rather than mutating it in place, since the same *PaginationMeta
// also reaches PatchResourceCache's write of the canonical ResourceCache
// entry. Shared by observeRelatedCheckResultRows's RowStore dual-write above
// and HandleRelatedCheckResult's PatchResourceCache intent
// (handlers_resources.go) so the two writers of the same CachedPages data
// cannot diverge on which entries the truncation assumption applies to.
func resolveCachedPagePagination(entry resource.ResourceCacheEntry) *resource.PaginationMeta {
	if entry.Pagination == nil {
		if entry.IsTruncated {
			return &resource.PaginationMeta{IsTruncated: true}
		}
		return nil
	}
	truncated := entry.IsTruncated || entry.Pagination.IsTruncated
	if entry.Pagination.IsTruncated == truncated {
		return entry.Pagination
	}
	meta := *entry.Pagination
	meta.IsTruncated = true
	return &meta
}
