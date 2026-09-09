// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// handlers_resources.go — Update()-switch session-mutation handlers.
//
// Platform-agnostic (c *Core) Handle* methods for the five resource/detail
// mutation events the TUI adapter forwards from its Update() switch:
//
//	HandleResourcesLoaded     — ResourceCache write-through + Wave-2 probe
//	                            re-dispatch on enrichment-rerun token match.
//	HandleEnrichDetailResult  — detail-view enrichment error surface.
//	HandleRelatedCheckResult  — RelatedCache append; CachedPages and
//	                            LazyAddedResources merge; flash on errors.
//	HandleIdentityLoaded      — session.Identity write + domain-mirror emit
//	                            (renderer reads the mirror, not awsclient).
//	HandleIdentityError       — fetch-flag clear; adapter handles view note.
//
// Plus two utility methods used by the adapter:
//
//	(*Core).AllRegions        — call-through to awsclient.AllRegions so the
//	                            adapter need not import core/aws.
//	(*Core).ResetRuleSets     — session.RuleSets swap + Clients rewire (used
//	                            by SES refresh paths in handleRefresh).
//
// Companion file: handlers.go owns the flash / session / view-stack
// handlers. Each shell-level concern lives in its own file (alongside
// handlers_availability.go / handlers_navigate.go / handlers_related.go)
// so a Core handler grep stays narrow.
package runtime

import (
	"errors"
	"slices"
	"strings"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// ResourcesLoadedEvent is the adapter-translated form of
// messages.ResourcesLoaded. Adapters compute no view-state-dependent
// fields when populating it — every field maps 1:1 to the source message
// — so the Core handler is a pure function of session + event.
type ResourcesLoadedEvent struct {
	ResourceType string
	Resources    []resource.Resource
	Pagination   *resource.PaginationMeta
	Append       bool
	TypeGen      domain.Gen
	// ListSeq is the per-type request sequence the fetch that produced this
	// result was dispatched at (messages.ResourcesLoaded.ListSeq). Ordering
	// between two results for one list is its question, not TypeGen's:
	// TypeGen says only whether this result answers the enrichment rerun that
	// is currently outstanding, and a result can hold the current rerun token
	// while a later request has already superseded it.
	ListSeq domain.Gen
	// ScreenID names the list screen instance whose own fetch produced this
	// result (messages.ResourcesLoaded.ScreenID). It is the key the ordering
	// guard draws sequences per, so a drill and the list under it order their
	// own requests and not each other's.
	ScreenID domain.Gen
	Err      error
	// Provenance identifies which fetch pipeline produced this event — mirrors
	// messages.ResourcesLoaded.Provenance verbatim (every production caller
	// forwards it unchanged: internal/tui/runtime_adapter_resources.go,
	// orchestrator.go's HandleEvent). Only a
	// Provenance.CanonicalList() result may seed the cross-view
	// PatchResourceCache entry or dispatch the list-open Wave-2 probe below —
	// a filtered/by-ID/child result sharing this ResourceType is never the
	// type's global population, and treating it as one would let a narrow
	// drill's subset silently replace (or appear to seed) the canonical
	// per-type cache, and enrich against rows that were never observed into
	// RowStore in the first place (see the canonical-provenance gate in
	// core/app.Controller.handleResourcesLoadedEvent, which
	// gates the RowStore write itself on the same predicate).
	Provenance messages.FetchProvenance
}

// HandleResourcesLoaded owns the session-state portion of the post-fetch
// processing for a top-level resource list. It does NOT touch the view
// stack — the adapter still routes the message through updateActiveView
// for view-side processing (ResourceListModel.Update absorbs Resources
// and writes the rich entry back via cacheTopLevelResourceList).
//
// What this method does:
//
//   - Emits ClearFlash so any active "Refreshing..." flash dismisses.
//   - When the loaded type is not yet cached (and the message is not an
//     Append page), emits PatchResourceCache so cross-view navigation
//     (e.g. related-navigate to a not-yet-visited type) finds an entry.
//     The !alreadyCached guard mirrors the original case body — the
//     view-side cacheTopLevelResourceList write wins when both run.
//   - On a paginated partial-success (Err non-nil with Resources present)
//     emits a FlashIntent so the `!` log records the failure.
//   - On enrichment-rerun match (TypeGen non-zero AND matches the per-type
//     gen captured at Ctrl+R dispatch), reseeds RowStore (OriginFetch) and
//     emits a TaskKindProbeEnrich task. Stale rerun tokens are silently
//     dropped.
//   - On a normal (non-rerun), non-Append, error-free top-level list load,
//     emits a TaskKindProbeEnrich task when the type is a registered
//     top-level resource (not a child view) with a registered Wave-2 issue
//     enricher. This is the list-open Wave-2 dispatch: opening a list for
//     the first time this session must enrich it exactly like the
//     bootstrap availability sweep does, instead of leaving row flags and
//     the menu issue badge unset until the user happens to hit Ctrl+R.
//     Gated OFF on Append (a load-more page must not re-trigger the probe —
//     already enriched/queued on page 1) and on Err != nil (a failed load
//     has nothing new and reliable to enrich). See
//     tests/unit/runtime_list_open_enrich_dispatch_test.go for the pinned
//     contract this branch satisfies.
func (c *Core) HandleResourcesLoaded(ev ResourcesLoadedEvent) ([]UIIntent, []TaskRequest) {
	// Canonicalize once at the chokepoint: an alias-opened list ("buckets",
	// "workgroups") must key the gen-guard map, RowStore reseed, task scope,
	// and HasIssueEnricher lookup by the same ShortName that
	// RowStore/EnrichmentTypeGen/ProbeResources use everywhere else — the raw
	// alias would dispatch a task under a scope no enrichment reads and never
	// find rows to enrich (mirrors handleEnrichmentChecked's canonicalization
	// in handlers_availability.go).
	resType := ev.ResourceType
	if td := resource.FindResourceType(resType); td != nil {
		resType = td.ShortName
	}

	// A superseded result answers a request a later one has already replaced,
	// whatever its rerun token says: the token below tells which enrichment
	// rerun a result belongs to, never which of two results for the list is
	// newer. Nothing it carries may land — not the reseed, not the enrich
	// dispatch, and not the cross-view cache seed below, all of which would
	// write the older page. This is the one place the question is asked: the
	// answer leaves as a ListResultVerdict, and the screen state that applies
	// the result reads it (StampListResult) rather than asking again. A
	// superseded result still reaches that screen state, which owes the
	// discarded request the retirement of the activity flag it raised.
	superseded := c.ListResultSuperseded(ev.ScreenID, ev.ListSeq)
	verdict := ListResultVerdict{ResourceType: resType, Superseded: superseded}
	if superseded {
		return []UIIntent{verdict}, nil
	}

	intents := []UIIntent{verdict, ClearFlash{}}

	// Cross-view cache seed: only a genuinely canonical top-level result may
	// seed the shared per-type ResourceCache entry. A filtered/by-ID/child
	// result sharing this ResourceType is never the type's global
	// population — seeding PatchResourceCache from it would let that
	// narrower subset masquerade as "this type is already cached" for every
	// other, not-yet-visited view of the same type (mirrors
	// the identical RowStore-write gate in core/app). ev.Err == nil
	// is required too: a partial-success composite error (some resources
	// landed AND something failed — e.g. the IAM policy fetcher's inline-group
	// enumeration failing while managed-policy pagination still reports
	// exact) must never seed the cross-view cache as if it were a complete
	// population — a later, genuinely complete fetch would then see
	// HasResourceCache already true and skip reseeding, leaving the
	// incomplete result authoritative indefinitely.
	if resType != "" && !ev.Append && ev.Err == nil && ev.Provenance.CanonicalList() {
		if !c.HasResourceCache(resType) {
			intents = append(intents, PatchResourceCache{
				ResourceType: resType,
				Entry: &domain.ListViewCacheEntry{
					Resources:  ev.Resources,
					Pagination: ev.Pagination,
				},
			})
		}
	}

	if ev.Err != nil {
		_, region := c.session.CurrentPair()
		intents = append(intents, FlashIntent{
			Text:    failureLine("fetch "+resType, ev.Err, region),
			IsError: true,
		})
	}

	var tasks []TaskRequest
	if ev.TypeGen != 0 && ev.TypeGen == c.session.EnrichmentTypeGenGet(resType) {
		// ObserveRows below is this reseed's only destination. The
		// enrichment-rerun reseed is a genuine fetch result — OriginFetch,
		// wholesale replace, not an append.
		c.ObserveRows(resType, ev.Resources, ev.Pagination, session.OriginFetch, false)
		tasks = append(tasks, TaskRequest{
			Key: TaskKey{Kind: TaskKindProbeEnrich, Scope: resType},
		})
	} else if ev.TypeGen == 0 && ev.Err == nil && !ev.Append && ev.Provenance.CanonicalList() &&
		resource.FindResourceType(resType) != nil && c.HasIssueEnricher(resType) {
		// List-open Wave-2 dispatch (converges the web/headless and TUI
		// lanes onto one producer — see HandleEvent's messages.ResourcesLoaded
		// case, which forwards only the tasks this branch returns so its own
		// intents are never double-applied on the web/headless lane).
		// RowStore already holds this load's rows by the time this task
		// executes: applyResourcesLoaded (both the TUI's
		// ctrl.HandleResourcesLoadedEvent and the web/headless
		// Controller.handleResourcesLoadedEvent path) routes a top-level
		// canonical list's rows through Core.ObserveRows before either
		// caller reaches this method, so no explicit reseed is needed here
		// (unlike the rerun branch above, which is invoked in a context
		// where that write-through is not guaranteed to have happened yet).
		// The Provenance.CanonicalList() gate is what makes that guarantee
		// hold: a filtered/by-ID/child result never reaches ObserveRows at
		// all (applyResourcesLoaded only routes a topLevelCanonical call
		// through it), so dispatching this probe for one would enrich
		// against whatever the type's RowStore entry already happened to
		// hold — stale rows, or none.
		// EnrichListOpenPending marks this dispatch so a concurrently running
		// sweep's own refill (refillEnrichSweep, handlers_availability.go)
		// recognizes resType already has an outstanding probe if it later
		// pops the same type off session.EnrichQueue, and absorbs it into
		// the sweep instead of dispatching a second, redundant
		// TaskKindProbeEnrich for it (#462/#463 defect 2b).
		if c.session.EnrichListOpenPending == nil {
			c.session.EnrichListOpenPending = make(map[string]bool)
		}
		c.session.EnrichListOpenPending[resType] = true
		tasks = append(tasks, TaskRequest{
			Key: TaskKey{Kind: TaskKindProbeEnrich, Scope: resType},
		})
	}

	return intents, tasks
}

// StampListFailure is the failure lane's half of StampListResult: it
// canonicalises the type the failed request named and answers, once, whether a
// later request for that list has already superseded it. A failure is the
// other outcome of the same request, so it is routed and discarded by the same
// rules its paired success would have been — an alias failure that cannot find
// its canonical screen leaves that screen loading for ever, and a stale one
// that installs its error marks a screen already waiting on a newer fetch.
//
// A message the seam never saw keeps the caller's own type, which is what a
// hand-built failure with no paired fetch hands in.
func (c *Core) StampListFailure(msg messages.APIError) messages.APIError {
	if msg.ResourceType == "" {
		return msg
	}
	msg.ResourceType = resource.CanonicalShortName(msg.ResourceType)
	msg.Superseded = c.ListResultSuperseded(msg.ScreenID, msg.ListSeq)
	return msg
}

// StampListResult writes HandleResourcesLoaded's verdict onto the message the
// hosts hand on to their screen state: the canonical short name that seam
// keyed its work by, and whether the result was superseded. Both hosts route a
// messages.ResourcesLoaded through the seam first and stamp with this, so the
// canonicalisation and the supersession check happen once per message and
// every later reader — the delivery gate, the RowStore write, the flag
// retirement — agrees with what the seam actually did.
//
// A message the seam never saw is returned unchanged: an unstamped
// ResourceType is still the caller's own, which is what a direct test seam
// hands in.
func StampListResult(msg messages.ResourcesLoaded, intents []UIIntent) messages.ResourcesLoaded {
	for _, in := range intents {
		v, ok := in.(ListResultVerdict)
		if !ok {
			continue
		}
		msg.ResourceType = v.ResourceType
		msg.Superseded = v.Superseded
		return msg
	}
	return msg
}

// RefreshListEnrichment prepares a top-level list refresh for rt: the single
// mutation-list Core method shared by the TUI's list-refresh-with-enricher
// branch (internal/tui/runtime_adapter_navigate.go) and the headless/web
// list-refresh branch (handleActionRefresh, core/app/actions_list.go).
//
// Strips rt's RowStore-retained rows of any stale wave2
// Findings/AttentionDetails, bumps the per-type enrichment generation, and
// clears EnrichmentRan + EnrichmentTruncatedIDs so the rerun is treated as
// genuinely fresh rather than a duplicate of an earlier run.
//
// Returns the bumped token. Callers must stamp it onto the refetch's
// dispatch (the TUI wraps its own tea.Cmd's outgoing Msg since its fetch does
// not go through a runtime.TaskRequest; the headless/web lane attaches it via
// FetchResourcesPayload.TypeGen on the KindFetchResources task instead) so
// HandleResourcesLoaded's rerun branch (TypeGen != 0 && TypeGen == current)
// applies this rerun's own result and rejects a stale, superseded completion.
//
// No-op (returns 0) when rt has no registered issue enricher — callers must
// treat a 0 return as "nothing to stamp; dispatch the normal, unstamped
// refetch."
func (c *Core) RefreshListEnrichment(rt string) domain.Gen {
	canon := resource.CanonicalShortName(rt)
	if !c.HasIssueEnricher(canon) {
		return 0
	}
	c.AmendRows(canon, stripWave2FindingsRows)
	tok := c.BumpEnrichmentTypeGen(canon)
	c.DeleteEnrichmentRan(canon)
	c.DeleteEnrichmentTruncatedIDs(canon)
	return tok
}

// ClearAllWave2Findings strips wave2 findings from every row RowStore retains
// for every type this session has touched. Used by
// Controller.RestartAvailabilitySweep (core/app) so a main-menu Ctrl+R never
// rehydrates stale wave2 attention state on the next list-open, mirroring the
// TUI's own main-menu Ctrl+R path.
//
// Every retained type's rows live in exactly one RowStore entry, so the
// type-name set gathered here is the union of the
// full (ResourceCacheKeys) and Partial (lazy, via ForEachLazyResourceCache)
// entries plus ProbeOriginTypeNames' Origin=Probe/Disk entries.
func (c *Core) ClearAllWave2Findings() {
	canons := make(map[string]struct{})
	for _, k := range c.ResourceCacheKeys() {
		canons[k] = struct{}{}
	}
	c.ForEachLazyResourceCache(func(rt string, _ []resource.Resource) {
		canons[rt] = struct{}{}
	})
	for _, k := range c.ProbeOriginTypeNames() {
		canons[k] = struct{}{}
	}
	for canon := range canons {
		c.AmendRows(canon, stripWave2FindingsRows)
	}
}

// stripWave2FindingsRows returns a copy of rows with every wave2-sourced
// Finding removed — the shared AmendRows callback RefreshListEnrichment and
// ClearAllWave2Findings both need: a stale wave2 verdict must not survive
// into the next enrichment pass. ApplyWave2ToRow with nil results IS the
// clear, and it is the only code that knows a row's wave-1 AttentionDetails
// are not the fold's to discard.
func stripWave2FindingsRows(rows []resource.Resource) []resource.Resource {
	if len(rows) == 0 {
		return rows
	}
	out := make([]resource.Resource, len(rows))
	copy(out, rows)
	for i := range out {
		ApplyWave2ToRow(&out[i], resource.ResourceTypeDef{}, nil, nil)
	}
	return out
}

// EnrichDetailResultEvent is the adapter-translated form of
// messages.EnrichDetailResult restricted to the fields Core inspects.
// EnrichedRes stays adapter-side because the renderer passes the result
// through updateActiveView for the detail view; the Core handler only
// decides whether to emit an error flash and whether a pending sticky
// refresh (ResourceID/OperationID, below) has been satisfied.
type EnrichDetailResultEvent struct {
	ResourceType string
	ResourceID   string
	OperationID  domain.Gen
	Err          error
}

// HandleEnrichDetailResult emits a single FlashIntent on enrichment
// failure. The success path is otherwise a no-op from Core's perspective —
// the adapter shim derives wave-1 findings and routes the enriched resource
// through updateActiveView so the detail view rebuilds its field list —
// except for one session-state decision that belongs here rather than in
// either adapter: on success, clear a pending sticky-refresh demand for the
// resource (session.PendingDetailRefresh, see beginDetailWorkloadLocked's
// "sticky refresh" doc comment) once fresh data has actually landed for an
// operation at least as new as the one that demanded the refresh. This is
// the single call both the TUI (runtime_adapter_resources.go, direct Core
// call) and web/headless (foldEnrichDetailResultLocked) lanes make, so
// clearing it here — rather than in one lane's fold only — guarantees a
// successful Ctrl+R retires its own demand regardless of which lane handled
// it. A stale-op call cannot mis-clear it: its OperationID is strictly less
// than the recorded one. An error leaves the demand recorded — a failed
// refresh must never downgrade the next open back to cache.
//
// awsclient.ErrDetailEnrichSkipped is a third, distinct outcome from either
// success or failure: the enricher fetched nothing at all (a disk-seeded
// row with no live RawStruct yet) and reports that honestly rather than as a
// completed success. It must neither flash (this is an expected, self-healing
// transient, not a failure to surface) nor clear a pending sticky-refresh
// demand (an operation that did nothing must not consume the demand that
// would have retried it once live data actually lands) — so it returns
// early, before both the flash branch and the demand-clearing success path.
func (c *Core) HandleEnrichDetailResult(ev EnrichDetailResultEvent) ([]UIIntent, []TaskRequest) {
	if errors.Is(ev.Err, awsclient.ErrDetailEnrichSkipped) {
		return nil, nil
	}
	if ev.Err != nil {
		_, region := c.session.CurrentPair()
		return []UIIntent{FlashIntent{
			Text:    failureLine("enrich "+ev.ResourceType, ev.Err, region),
			IsError: true,
		}}, nil
	}
	key := RelatedCacheKey(ev.ResourceType, ev.ResourceID)
	if recorded, ok := c.PendingDetailRefreshGet(key); ok && ev.OperationID >= recorded {
		c.PendingDetailRefreshClear(key)
	}
	return nil, nil
}

// RelatedCheckResultEvent is the adapter-translated form of
// messages.RelatedCheckResult. SourceResourceID is the canonical source
// resource ID the adapter resolved from messages.SourceResourceID with a
// fallback to the active detail view's resource. The fallback resolution
// is adapter-side because the active view is renderer state; the Core
// handler treats SourceResourceID as authoritative.
type RelatedCheckResultEvent struct {
	ResourceType       string
	SourceResourceID   string
	DefDisplayName     string
	Result             resource.RelatedCheckResult
	CachedPages        map[string]resource.ResourceCacheEntry
	LazyAddedResources map[string][]resource.Resource
	LazyAddError       error
}

// HandleRelatedCheckResult owns the session-state decisions for an
// async related-check result. The actual session writes are emitted as
// intents (PatchRelatedCache, PatchResourceCache, PatchLazyResourceCache)
// so applyIntents is the single locus of session-cache mutation; this
// keeps the handler-result graph diff-able and keeps session-cache
// mutation in one place. The adapter shim
// still routes the message through updateActiveView after applying the
// returned intents so the detail view's right-column model receives
// the result.
//
// Field canonicalisation: CachedPages and LazyAddedResources keys may be
// aliases (e.g. "rds" instead of canonical "dbi"). Core resolves each
// key via resource.FindResourceType so ResourceCache and LazyResourceCache
// are always keyed by the canonical ShortName — matching the read paths
// that look up by ShortName.
func (c *Core) HandleRelatedCheckResult(ev RelatedCheckResultEvent) ([]UIIntent, []TaskRequest) {
	var intents []UIIntent

	if ev.SourceResourceID != "" {
		intents = append(intents, PatchRelatedCache{
			ResourceType:   ev.ResourceType,
			SourceID:       ev.SourceResourceID,
			DefDisplayName: ev.DefDisplayName,
			Result:         ev.Result,
		})
	}

	// CachedPages: insert each first-page result verbatim if neither
	// ResourceCache nor LazyResourceCache already holds an entry for the
	// canonical type. The skip-when-present guards prevent CachedPages
	// from evicting richer entries (top-level fetch result, prior
	// lazy-add merge). Decision is in Core; the write is done by the
	// adapter via PatchResourceCache. addedInBatch preserves
	// first-write-wins semantics when two aliases in CachedPages
	// canonicalise to the same ShortName.
	addedInBatch := map[string]struct{}{}
	for aliasName, entry := range ev.CachedPages {
		shortName := resource.CanonicalShortName(aliasName)
		if _, dup := addedInBatch[shortName]; dup {
			continue
		}
		// A type's rows live in exactly one RowStore entry regardless of
		// which lane wrote them — Gen!=0 alone (full OR Partial) is the
		// "already has an entry" test.
		if c.session.RowStore.Snapshot(shortName).Gen != 0 {
			continue
		}
		intents = append(intents, PatchResourceCache{
			ResourceType: shortName,
			Entry: &domain.ListViewCacheEntry{
				Resources:  entry.Resources,
				Pagination: resolveCachedPagePagination(entry),
			},
		})
		addedInBatch[shortName] = struct{}{}
	}

	// LazyAddedResources: append-dedup merge into the lazy (Partial) RowStore
	// entry. Core computes the merged slices and emits a single
	// PatchLazyResourceCache carrying the full Adds map.
	var lazyAdds map[string][]resource.Resource
	for aliasName, extra := range ev.LazyAddedResources {
		if len(extra) == 0 {
			continue
		}
		shortName := resource.CanonicalShortName(aliasName)
		existing := c.session.RowStore.Snapshot(shortName).Rows
		known := make(map[string]struct{}, len(existing))
		for _, r := range existing {
			known[r.ID] = struct{}{}
		}
		merged := existing
		for _, r := range extra {
			if _, dup := known[r.ID]; dup {
				continue
			}
			known[r.ID] = struct{}{}
			merged = append(merged, r)
		}
		if lazyAdds == nil {
			lazyAdds = make(map[string][]resource.Resource)
		}
		lazyAdds[shortName] = merged
	}
	if lazyAdds != nil {
		intents = append(intents, PatchLazyResourceCache{Adds: lazyAdds})
	}

	// One result, one sentence. A related check can fail twice — the checker
	// itself and the by-ID lazy add beside it — and both flashes land on the
	// same status line, so a second intent only overwrites the first and the
	// operator never learns there were two. Identical lines collapse: the two
	// halves often fail on the same denied call, and saying it twice is not
	// more information.
	_, region := c.session.CurrentPair()
	var lines []string
	if ev.LazyAddError != nil {
		lines = append(lines, failureLine("related-fetch", ev.LazyAddError, region))
	}
	if err := ev.Result.Err(); err != nil {
		if line := failureLine("related "+ev.Result.TargetType(), err, region); !slices.Contains(lines, line) {
			lines = append(lines, line)
		}
	}
	if len(lines) > 0 {
		intents = append(intents, FlashIntent{Text: strings.Join(lines, "; "), IsError: true})
	}
	return intents, nil
}

// IdentityLoadedEvent is the adapter-translated form of
// messages.IdentityLoaded. Identity is the awsclient-typed pointer
// (carried as `any` on the message itself to keep the messages package
// renderer-neutral). Core type-asserts it and converts to the domain
// mirror before emitting SetIdentityIntent.
type IdentityLoadedEvent struct {
	Identity any
}

// HandleIdentityLoaded writes the resolved caller identity into session
// state, clears IdentityFetching, and emits SetIdentityIntent paired with
// HeaderInvalidateIntent so the next View() picks up the new badge / role.
// A nil or wrong-typed Identity clears IdentityFetching but emits no
// further intents — a non-matching Identity type is silently dropped.
func (c *Core) HandleIdentityLoaded(ev IdentityLoadedEvent) ([]UIIntent, []TaskRequest) {
	c.session.IdentityFetching = false
	awsID, ok := ev.Identity.(*awsclient.CallerIdentity)
	if !ok || awsID == nil {
		return nil, nil
	}
	c.session.Identity = awsID
	return []UIIntent{
		SetIdentityIntent{Identity: domainCallerIdentityFrom(awsID)},
		HeaderInvalidateIntent{},
	}, nil
}

// IdentityErrorEvent is the adapter-translated form of
// messages.IdentityError. Err is the renderer-visible message string
// shown on the IdentityModel; HandleIdentityError clears
// IdentityFetching so the header drops the spinner and lets the
// adapter render the error directly on the view.
type IdentityErrorEvent struct {
	Err string
}

// HandleIdentityError clears IdentityFetching and any previously-resolved
// Identity — a failed fetch on the current profile/region must never leave
// a prior pair's ARN displayed in the header indefinitely. ClearIdentityIntent
// is the same intent HandleProfileSelected/HandleRegionSelected use to drop a
// stale identity on rotation; it clears both the adapter's identity-overlay
// mirror (TUI rendererState / Controller.identityResult) and — combined with
// the session write above — the header, which derives its badge/role fresh
// from session.Identity every render. The adapter shim additionally calls
// IdentityModel.SetError when an identity view is active — that branch needs
// renderer-side view-stack inspection so it stays out of Core.
func (c *Core) HandleIdentityError(ev IdentityErrorEvent) ([]UIIntent, []TaskRequest) {
	c.session.IdentityFetching = false
	hadIdentity := c.session.Identity != nil
	c.session.Identity = nil
	_ = ev.Err // not surfaced as a flash today; reserved for future hook
	if hadIdentity {
		return []UIIntent{ClearIdentityIntent{}}, nil
	}
	return nil, nil
}

// AllRegions returns the commercial-partition region catalogue. Exposed
// on Core so the adapter does not need to import core/aws directly
// for the region selector.
func (c *Core) AllRegions() []awsclient.AWSRegion {
	return awsclient.AllRegions()
}

// ResetRuleSets swaps Session.RuleSets to a fresh store and rewires the
// retained ServiceClients transport so any in-flight blocked
// DescribeActiveReceiptRuleSet calls write to the orphaned old store on
// completion. Called by the SES refresh paths in handleRefresh (detail
// view and resource list view, when ResourceType == "ses"). Exposed on
// Core so the adapter does not need to import core/session for the
// NewRuleSetStore call.
func (c *Core) ResetRuleSets() {
	c.session.RuleSets = session.NewRuleSetStore()
	if c.session.Clients != nil {
		c.session.Clients.SetRuleSets(c.session.RuleSets)
	}
}

// domainCallerIdentityFrom converts an *awsclient.CallerIdentity to the
// renderer-shaped *domain.CallerIdentity mirror. UserID is intentionally
// dropped — only ARN-parser internals use it and the adapter never
// reads it.
func domainCallerIdentityFrom(id *awsclient.CallerIdentity) *domain.CallerIdentity {
	if id == nil {
		return nil
	}
	return &domain.CallerIdentity{
		AccountID:     id.AccountID,
		AccountAlias:  id.AccountAlias,
		Arn:           id.Arn,
		RoleName:      id.RoleName,
		UserName:      id.UserName,
		SessionName:   id.SessionName,
		IdentityName:  id.IdentityName,
		IsAssumedRole: id.IsAssumedRole,
	}
}
