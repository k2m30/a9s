// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package session owns the in-memory orchestration state for the active
// profile/region session: fetch-availability queue, Wave 2 enrichment queue,
// per-type caches, and the generation counters that invalidate stale async
// results on profile/region switch or refresh.
//
// Session is held as Session *session.Session on tui.Model. Access sites use
// m.Session.RelatedGen etc. directly for scalar fields; every cached
// resource-list row (top-level fetch, Wave-1 probe, disk seed, or sparse
// FetchByIDs drill) goes through RowStore (see rowstore.go) rather than a
// session map — there is no separate ResourceCache/LazyResourceCache field.
//
// Rules of ownership:
//
//   - Only session-scoped orchestration state belongs here. UI shell concerns
//     (view stack, header, input mode, theme) stay on the surrounding Model.
//   - Maps that handler paths write into directly (EnrichmentRan,
//     EnrichmentTypeGen, EnrichmentTruncatedIDs) MUST be constructed by
//     New(). The availability/enrich queues stay nil until a probe retains
//     its first batch — they are built in place.
//   - There is no parallel EnrichmentFindings map on tui.Model or on Session;
//     Wave 2 findings are written directly onto each cached
//     `resource.Resource.Findings` slice
//     (Source = "wave2:<short>") and r.AttentionDetails, via applyEnrichment
//     in internal/tui/app_enrich_fold.go. The cached rows are the authority;
//     runtime.RuntimeState.EnrichmentFindings and PatchDetail.EnrichmentFindings
//     are derived adapter-payload surfaces, not a second source of truth.
//   - Session rotation (profile/region switch) MUST bump every generation and
//     replace/clear the caches, so in-flight messages tagged with old gens are
//     discarded by the handlers' gen guards.
package session

import (
	"maps"
	"sync"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ProbeStatusRecord is the session-scoped record of one resource type's
// most recent scan outcome (#462 — per-probe status + duration). Plain
// fields only: session must not import core/runtime (runtime already
// imports session, the reverse would cycle), so runtime.Core.ScanStatus
// converts these into the runtime-owned ProbeStatus/ProbeOutcome types.
type ProbeStatusRecord struct {
	Outcome  string
	Duration time.Duration
	Err      string
	At       time.Time

	// AvailOutcome/AvailDuration/AvailErr are the Wave-1 availability probe's
	// OWN outcome/duration/error classification for this type, distinct from
	// Outcome/Duration/Err above (which fold in a Wave-2 enrichment rerun,
	// see runtime.handleEnrichmentChecked). Set equal to the aggregate
	// fields by handleAvailabilityChecked (a fresh availability probe IS the
	// new baseline); left untouched by handleEnrichmentChecked, which reads
	// them to recompute Outcome/Duration/Err from the baseline instead of
	// from whatever aggregate a PRIOR enrichment rerun already folded in —
	// without this, re-running enrichment (list-open / Ctrl+R ProbeEnrich
	// without a fresh availability probe) would re-accumulate Duration and
	// keep a stale partial/Err from an earlier failed enrichment (#462/#463
	// external review, defect 1).
	AvailOutcome  string
	AvailDuration time.Duration
	AvailErr      string
}

// Session owns the in-memory orchestration state for the active
// profile/region session.
type Session struct {
	// Session identity — set by the caller (tui.New / handler) before/after
	// Rotate. Writers on the Bubble Tea update goroutine (profile/region
	// switch handlers, connect success/failure handlers in
	// core/runtime/handlers.go) MUST write both fields together via
	// SetProfileRegion rather than direct field assignment — see pairMu's
	// doc comment for the cross-goroutine race this guards against.
	// Same-goroutine reads on the update path (e.g. Core.Profile()/Region())
	// may still read the fields directly; only the cross-goroutine
	// cache-store accessors and the writers need the lock.
	Profile string
	Region  string

	// cacheRoot is the cache.Root() value captured once, at New() time. Every
	// ensureCacheStoreLocked call uses this pinned value instead of a live
	// cache.Root() re-read, so a late async writer (e.g. a leaked
	// availability-save goroutine from a never-Closed Controller) can never
	// follow a since-changed A9S_CONFIG_FOLDER into a directory that has
	// nothing to do with the session it belongs to. Not recaptured by
	// Rotate() — the root is process-stable in production, and recapturing it
	// would reintroduce the exact test-isolation leak this field closes.
	cacheRoot string

	// Session-scoped AWS transport. Set by handleClientsReady; cleared
	// explicitly by handlers, not by Rotate (the caller decides whether to
	// reuse the still-valid old clients on a rotation that may fail).
	Clients *awsclient.ServiceClients

	// PreSuppliedClients is the bootstrap-channel transport supplied by tests
	// or demo mode via WithClients. Lives across Rotate because it represents
	// a static input, not a live session.
	PreSuppliedClients *awsclient.ServiceClients

	// Identity is the resolved caller identity (account ID, ARN, role). Set
	// by handleIdentityLoaded; cleared by Rotate so a stale identity from the
	// pre-rotate session cannot leak into the next.
	Identity *awsclient.CallerIdentity

	// IdentityFetching latches that an identity fetch is in flight, so the
	// header can show a spinner. Cleared by Rotate.
	IdentityFetching bool

	// ConnectGen is the staleness counter for AWS connect attempts. Bumped on
	// every profile/region switch so a slow pre-switch ClientsReadyMsg arriving
	// after the user has switched again is rejected by the gen guard.
	// Rotate() bumps this; handlers MUST NOT bump it manually.
	ConnectGen domain.Gen

	// PendingRefresh marks that a successful ClientsReady should re-fetch the
	// active resource list (set by profile/region switch handlers, and true
	// by default from New() so a navigation issued before the first connect
	// replays once connected — C10). Cleared by Rotate; re-set to true after
	// Rotate in the switch handlers.
	PendingRefresh bool

	// Rollback target for an in-flight profile/region switch. Captured BEFORE
	// Rotate (via local vars) so the rapid A→B→C case keeps A as the rollback
	// target. Cleared by Rotate; restored explicitly by the switch handler.
	HasPrevState bool
	PrevProfile  string
	PrevRegion   string

	// Command is the one-shot resource short name to navigate to on the first
	// ClientsReadyMsg (from the -c CLI flag). Cleared by the handler after use;
	// not cleared by Rotate (a profile switch should not lose the flag if the
	// initial connect failed and rolled back).
	Command string

	// CommandArmed latches that the live (cached) connect path has decided
	// the one-shot -c navigation for PendingCommand is eligible to fire
	// (Command was set and StackDepth==1 at ClientsReady time), but must wait
	// for handleAvailabilityCacheLoaded to seed RowStore's disk-cached rows
	// first so the navigation never races the availability-cache seed
	// (the deferred -c navigation, D11). Consumed (cleared) by handleAvailabilityCacheLoaded; not
	// cleared by Rotate for the same reason Command survives it.
	CommandArmed bool

	// PendingCommand carries the resource short name captured from Command at
	// ClientsReady time, for handleAvailabilityCacheLoaded to consume once
	// CommandArmed is set (Command itself is cleared immediately on every
	// ClientsReadyMsg, armed or not — see the Command field doc).
	PendingCommand string

	// NoCache disables on-disk availability caching and background probes
	// (set by the --no-cache / --demo CLI flags). Survives Rotate — it is a
	// static policy, not session state.
	NoCache bool

	// SweptPairs records, per profile+"--"+region pair visited this process
	// lifetime, whether the Wave-1 availability sweep has run to completion
	// at least once. Session-lifetime BY DESIGN: unlike every other
	// Rotate-cleared queue/counter, Rotate() must NOT clear this map — the
	// whole point is that a profile/region switch back to an
	// already-fully-swept pair skips the redundant full-menu sweep instead
	// of re-running it from scratch. Read/written under pairMu, same as
	// Profile/Region, via PairSwept/MarkPairSwept/ClearPairSwept.
	SweptPairs map[string]bool

	// pairMu guards two related things that must be observed together
	// atomically across goroutines: the live Profile/Region pair, and
	// CacheStore + its cacheStore{Profile,Region} load-stamp below.
	//
	// Profile/Region are written on the Bubble Tea update goroutine by the
	// profile/region-switch handlers (HandleProfileSelected/
	// HandleRegionSelected/handleClientsReadyFailure/
	// handleClientsReadySuccess in core/runtime/handlers.go) — all via
	// SetProfileRegion, never by direct field assignment. They are read
	// cross-goroutine by EnsureCacheStore/WithCacheStore/ReadCacheStore's own
	// callers, which historically read c.session.Profile/Region at the Core
	// accessor call site BEFORE entering this lock — that pre-lock read was
	// the actual data race (a profile switch's field write interleaving with
	// the detached availability-cache-save writer goroutine's read, caught by
	// -race on CI run 28839454135: Core.HandleProfileSelected's write at
	// handlers.go:418 against Core.WithCacheStore's read at accessors.go:97,
	// reached via the single-writer goroutine runAvailabilitySaveLoop spawns
	// in core/app/menu.go). EnsureCacheStore/WithCacheStore/ReadCacheStore
	// now read the pair via CurrentPair while already holding pairMu, closing
	// that gap for every caller in one place rather than one at a time.
	//
	// TUI tea.Cmd goroutines and concurrent web drains both reach
	// EnsureCacheStore/CacheStore, and Rotate clears CacheStore from the
	// event-handling goroutine on a profile/region switch — all must
	// serialize on this lock (Codex P1 / CodeRabbit race).
	pairMu sync.Mutex

	// CacheStore is the loaded per-type disk cache (C7) for the pair recorded
	// in cacheStoreProfile/cacheStoreRegion. nil until LoadDir has run for a
	// pair (either at startup via TaskKindLoadAvailCache, or after a pair
	// switch). The HARD INVARIANT (C7 "load before save") is structural:
	// Put/SaveType are methods on *cache.Store, and the only way to obtain
	// one is cache.LoadDir — so no save can happen for a pair before its own
	// load. Cleared (set to nil) by Rotate so a pair switch never lets writes
	// for the OLD pair's Store race a save for the NEW pair (C9). Access only
	// through EnsureCacheStore — never read/write this field directly
	// outside pairMu.
	CacheStore *cache.Store

	// cacheStoreProfile/cacheStoreRegion are the pair CacheStore was loaded
	// for. EnsureCacheStore compares these against the caller's current pair
	// and reloads via cache.LoadDir when they disagree, so a task dispatched
	// for pair A that executes after a switch to pair B never writes into
	// A's directory using a memoized Store (see EnsureCacheStore).
	cacheStoreProfile string
	cacheStoreRegion  string

	// Wave 1 availability scan.
	AvailabilityGen domain.Gen // bumped on profile/region switch to cancel stale probes
	AvailQueue      []string   // resource short names remaining to probe
	AvailChecked    int        // number probed so far in current gen
	AvailTotal      int        // total types to probe in current gen

	// AvailSweepPending latches that handleAvailabilityCacheLoaded built
	// AvailQueue (menu/RowStore already seeded from disk) but held back the
	// first probe-dispatch batch because Clients was still nil at that
	// instant — the disk-cache load races ahead of the AWS connect on
	// startup (Init fires both concurrently via tea.Batch for instant-paint,
	// C1), so dispatching probes here would run them against a nil
	// transport and fail every one with "AWS clients not initialized"
	// instead of actually probing. Consumed by the next successful
	// HandleClientsReady, which drains AvailQueue's first batch now that a
	// real transport exists. Not cleared by Rotate for the same reason
	// AvailQueue itself is reset there instead of surviving — a pair switch
	// always restarts the sweep from HandleClientsReady's own dispatch, so a
	// stale pending flag from the old pair is moot once AvailQueue is nil.
	AvailSweepPending bool

	// Wave 2 issue-enrichment dispatch.
	//
	// ProbeResources/ProbeTruncated DIED in task #17 wave 1 stage 2 (row-store
	// unification): every read/write site in core/session, core/runtime,
	// and internal/tui now goes through RowStore (Origin=OriginProbe/OriginDisk/
	// OriginFetch as appropriate) instead of these two maps. See RowStore's doc
	// comment for the semantics this replaces.
	EnrichQueue   []string   // resource types pending Wave 2 enrichment
	EnrichmentGen domain.Gen // session-wide gen counter for Wave 2
	EnrichChecked int        // number of enrichment probes completed in current gen
	EnrichTotal   int        // total enrichment probes to run in current gen

	// EnrichSweepMembers is the set of resource types with a currently
	// outstanding TaskKindProbeEnrich dispatch created by the sweep itself
	// (startEnrichment's initial window or handleEnrichmentChecked's queue
	// refill, core/runtime/handlers_availability.go). A completion for a
	// member type frees its slot and drives the sweep's refill + EnrichChecked
	// bookkeeping; a completion for a non-member (e.g. a list-open probe,
	// core/runtime/handlers_resources.go) applies its payload but never
	// touches EnrichQueue or this counter (#462/#463 defect 2). Rebuilt
	// (not appended) every time a fresh sweep starts. Only touched from
	// Core.HandleEvent's serial dispatch loop — no mutex, same convention
	// as EnrichQueue.
	EnrichSweepMembers map[string]bool

	// EnrichListOpenPending marks a resource type with an outstanding
	// list-open Wave-2 probe (HandleResourcesLoaded) not yet completed. When
	// the sweep's own refill would otherwise pop the same type off
	// EnrichQueue, it finds this flag, skips issuing a second physical
	// dispatch, and folds the type into EnrichSweepMembers instead — so the
	// list-open probe's own completion later drives the sweep's refill/
	// counter bookkeeping in its place (#462/#463 defect 2b: a list-open
	// probe racing a queued sweep entry must not be physically dispatched
	// twice). Only touched from Core.HandleEvent's serial dispatch loop — no
	// mutex, same convention as EnrichQueue.
	EnrichListOpenPending map[string]bool

	// Per-type Wave 2 finding state (feature 018-enrichment-visibility).
	// NOTE: there is no parallel EnrichmentFindings map;
	// Wave 2 findings live on each cached resource.Resource.Findings slice
	// (see internal/tui/app_enrich_fold.go applyEnrichment). The Wave-2 progress
	// / control maps below remain here because they are session-scoped and are
	// cleared on Session.Rotate() — they are not the authority for finding data.
	EnrichmentRan          map[string]bool
	EnrichmentTruncatedIDs map[string]map[string]bool

	// EnrichmentTypeGen is the per-type Wave-2 enrichment counter, guarded by
	// enrichmentTypeGenMu — like ProbeStatus above, a dispatch-time snapshot
	// (Core.CaptureDispatch, read by a tea.Cmd/executor goroutine) and the
	// handler-loop writer (BumpEnrichmentTypeGen et al.) run on different
	// goroutines under web/headless hosts, so a bare map clone or index would
	// race the concurrent increment. Access only through
	// EnrichmentTypeGenGet/EnrichmentTypeGenBump/EnrichmentTypeGenSnapshot/
	// EnrichmentTypeGenReset — never read/write this field directly outside
	// enrichmentTypeGenMu.
	EnrichmentTypeGen   map[string]domain.Gen
	enrichmentTypeGenMu sync.Mutex

	// RowStore is the session-scoped, per-type row store (task #17 wave 1/3 —
	// row-store unification). The single source of truth for every cached
	// resource-list row this session has observed, replacing the former
	// ResourceCache/LazyResourceCache maps entirely (Stage 3): a type's rows
	// live in exactly one TypeRows entry regardless of which lane last wrote
	// them (Wave-1 probe, disk seed, top-level fetch, or a sparse FetchByIDs
	// drill — see Origin/Partial). Never nil after New()/Rotate.
	RowStore *RowStore

	// Session-scoped caches + stale-result guards.
	RelatedCache *RelatedCacheLRU

	// FilteredRows is the session home for server-side-filtered related-drill
	// results (C6). Kept outside RowStore because a filtered subset stored
	// under the type's canonical row-store key would poison that type's
	// global row set for every other consumer.
	FilteredRows *FilteredRowsLRU
	RelatedGen   domain.Gen // bumped on refresh/profile/region switch
	EnrichGen    domain.Gen // bumped on refresh/profile/region switch (detail-enrichment only)
	EnrichResKey string     // "resourceType:resourceID" of last detail-enrichment dispatch

	// Feature-specific session caches. These used to hang off *ServiceClients
	// but that blurred the AWS-transport/session-state boundary; they live
	// here instead and are passed to detail enrichers via DetailEnrichmentCtx.
	PolicyDocCache *awsclient.PolicyDocumentCache

	// IAMPolicies is the per-session cache for IAM policy resources, keyed by
	// both PolicyName and ARN. Replaces the package-level globals previously in
	// core/aws/iam_policies.go. Wired into *ServiceClients.IAMPolicies on
	// every ClientsReadyMsg so FetchIAMPoliciesByIDsFull uses the session store.
	IAMPolicies *policyStore

	// IdentityStore is the per-session cache for the AWS caller's account ID
	// used by Pattern-C related checkers. Replaces the package-level globals
	// previously in core/aws/identity_cache.go (identityCacheMu /
	// cachedAccountID / cachedAccountErr). Wired into *ServiceClients.
	// IdentityStore on every ClientsReadyMsg so Pattern-C related checkers
	// (Glue tags, EBS Backup) see a per-profile/region scoped cache rather
	// than a process-global one. Distinct from Session.Identity (the resolved
	// *awsclient.CallerIdentity) which holds the human-readable identity
	// metadata for the header / IdentityModel.
	IdentityStore *identityStore

	// RuleSets is the per-session, single-slot cache for the SES v1
	// DescribeActiveReceiptRuleSet response. Replaces the package-level
	// globals previously in core/aws/ses_related.go (sesRuleSetCacheMu
	// + sesRuleSetCaches map keyed by *ServiceClients pointer). Wired into
	// *ServiceClients.RuleSets on every ClientsReadyMsg so checkSESLambda /
	// checkSESS3 see a session-scoped cache rather than a process-global map.
	RuleSets *ruleSetStore

	// ProbeStatus is the per-type most-recent-scan-outcome record (#462),
	// keyed by resource short name. Guarded by probeStatusMu — unlike
	// AvailChecked/EnrichChecked and the rest of the Wave-1/Wave-2
	// bookkeeping above, which is single-goroutine (written only from the
	// handler loop), a host (desktop/web) may call Core.ScanStatus() to
	// read this map from a different goroutine. Access only through
	// SetProbeStatus/GetProbeStatus/AllProbeStatus — never read/write this
	// field directly outside probeStatusMu.
	ProbeStatus   map[string]ProbeStatusRecord
	probeStatusMu sync.Mutex

	// NewFindingPairs is the per-type count of (row, finding-code) pairs
	// newly observed on the most recent on-disk cache save for that type
	// (#463 — new-since-previous-scan deltas for FindingsOverview), keyed by
	// resource short name then domain.FindingCode. Replaced wholesale per
	// type on each save (core/runtime.saveResourceListCache), never
	// accumulated across saves — same access discipline as ProbeStatus:
	// guarded by newFindingPairsMu, access only through
	// SetNewFindingPairs/GetNewFindingPairs/AllNewFindingPairs.
	NewFindingPairs   map[string]map[domain.FindingCode]int
	newFindingPairsMu sync.Mutex
}

// New constructs a fresh Session with all maps initialized and generation
// counters seeded at 1. The seed=1 convention makes Generation=0 (unset)
// always stale, so synthetic test messages or early-return paths that leave
// Gen at its zero value are rejected by the gen guards.
func New() *Session {
	return &Session{
		cacheRoot: cache.Root(),
		// C10: a navigation issued before the first connect must trigger the
		// active-list re-fetch once connected, exactly like a post-switch
		// reconnect. On a menu-only startup, maybeRefreshIntents consumes this
		// flag harmlessly via the HasActiveRL gate.
		PendingRefresh:         true,
		EnrichmentRan:          make(map[string]bool),
		EnrichmentTypeGen:      make(map[string]domain.Gen),
		EnrichmentTruncatedIDs: make(map[string]map[string]bool),
		RowStore:               NewRowStore(),
		RelatedCache:           NewRelatedCacheLRU(MaxRelatedCacheEntries),
		FilteredRows:           NewFilteredRowsLRU(MaxFilteredRowsEntries),
		SweptPairs:             make(map[string]bool),
		RelatedGen:             1,
		EnrichGen:              1,
		EnrichmentGen:          1,
		AvailabilityGen:        1,
		PolicyDocCache:         &awsclient.PolicyDocumentCache{},
		IAMPolicies:            NewPolicyStore(),
		IdentityStore:          NewIdentityStore(),
		RuleSets:               NewRuleSetStore(),
	}
}

// CurrentPair returns the live Profile/Region pair while holding pairMu, so a
// caller on any goroutine observes a consistent snapshot rather than reading
// the two fields separately (which could race a concurrent SetProfileRegion
// writing one and not yet the other). Callers needing the pair together with
// a CacheStore decision (EnsureCacheStore/WithCacheStore/ReadCacheStore) read
// it from inside their own pairMu-held critical section instead of calling
// this method, to avoid a lock-release-then-reacquire window between the
// pair read and the store decision.
func (s *Session) CurrentPair() (profile, region string) {
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	return s.Profile, s.Region
}

// SetProfileRegion sets the live Profile/Region pair while holding pairMu.
// Every write to Session.Profile/Session.Region MUST go through this method
// (never a direct field assignment) so a concurrent EnsureCacheStore/
// WithCacheStore/ReadCacheStore call on another goroutine can never observe a
// torn or half-written pair, and never races the write itself (see pairMu's
// doc comment for the CI-caught race this closes).
func (s *Session) SetProfileRegion(profile, region string) {
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	s.Profile = profile
	s.Region = region
}

// sweptPairKey returns the SweptPairs key for a profile/region pair — always
// profile+"--"+region, never the reverse or any other separator, so every
// reader/writer of SweptPairs agrees on the same key shape.
func sweptPairKey(profile, region string) string {
	return profile + "--" + region
}

// PairSwept reports whether the CURRENT profile/region pair's Wave-1
// availability sweep has already run to completion at least once this
// session. Reads Profile/Region under pairMu, same discipline as
// CurrentPair.
func (s *Session) PairSwept() bool {
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	return s.SweptPairs[sweptPairKey(s.Profile, s.Region)]
}

// MarkPairSwept records that the CURRENT profile/region pair's Wave-1
// availability sweep has completed. Called by
// handleAvailabilityChecked once AvailChecked reaches AvailTotal for a
// non-empty sweep.
func (s *Session) MarkPairSwept() {
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	s.SweptPairs[sweptPairKey(s.Profile, s.Region)] = true
}

// ClearPairSwept removes the CURRENT profile/region pair's swept memo, so
// the next availability-cache load re-runs the full sweep instead of
// skipping it. Used by the manual full-menu refresh gesture (Ctrl+R on the
// main menu) — an explicit user refresh must always re-probe even an
// already-swept pair.
func (s *Session) ClearPairSwept() {
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	delete(s.SweptPairs, sweptPairKey(s.Profile, s.Region))
}

// EnsureCacheStore returns the *cache.Store for the current Profile/Region
// pair, loading (or reloading) it via cache.LoadDir when no store is
// memoized yet or the memoized store was loaded for a different pair. An
// unresolved pair ("" profile or region — pre-connect or cold boot before
// the first ClientsReady/Rotate settles Profile/Region) returns nil without
// memoizing, so a later call with the real pair always reloads instead of
// forever returning a store keyed by "<profile>--".
//
// The pair is read AND the CacheStore decision made inside the same pairMu
// critical section, so concurrent TUI tea.Cmd goroutines, concurrent web
// drains, and a same-moment profile/region switch (SetProfileRegion/Rotate)
// can never interleave a check-then-write on the field, observe a store
// loaded for the wrong pair, or race the Profile/Region read itself.
func (s *Session) EnsureCacheStore() *cache.Store {
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	return s.ensureCacheStoreLocked(s.Profile, s.Region)
}

// EnsureCacheStoreForRegion is EnsureCacheStore's counterpart for a caller
// (LoadAvailabilityCache) that needs to substitute a locally-resolved default
// region for an unresolved Session.Region without writing that resolution
// back onto the session (connect still owns Session.Region — see
// LoadAvailabilityCache's doc comment for why). Reads Session.Profile under
// pairMu just like EnsureCacheStore; region is the caller-supplied override
// rather than Session.Region.
func (s *Session) EnsureCacheStoreForRegion(region string) *cache.Store {
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	return s.ensureCacheStoreLocked(s.Profile, region)
}

// ensureCacheStoreLocked is EnsureCacheStore/EnsureCacheStoreForRegion's
// shared body, factored out so WithCacheStore/ReadCacheStore can also reuse
// it inside their own already-held pairMu critical section without a
// reentrant Lock call (sync.Mutex is not reentrant).
func (s *Session) ensureCacheStoreLocked(profile, region string) *cache.Store {
	if profile == "" || region == "" {
		return nil
	}
	if s.CacheStore != nil && s.cacheStoreProfile == profile && s.cacheStoreRegion == region {
		return s.CacheStore
	}
	s.CacheStore = cache.LoadDirIn(s.cacheRoot, profile, region)
	s.cacheStoreProfile = profile
	s.cacheStoreRegion = region
	return s.CacheStore
}

// WithCacheStore runs fn against the current Profile/Region pair's
// *cache.Store while holding pairMu for the entire call (both the pair read
// and the store decision, then fn itself), then returns fn's error.
//
// Store-lock serialization (D13): SaveResourceListCache and SaveAvailabilityCache each perform their
// own store.Type (read) / mutate / store.Put+SaveType (write) sequence for
// the same on-disk type file. Obtaining the *cache.Store via EnsureCacheStore
// and then mutating it afterward (the previous shape of both callers) only
// serializes the pointer lookup — the lock is released before the
// read-modify-write runs, so two of these sequences dispatched from
// concurrent tea.Cmd goroutines (e.g. a list's own fetch-completion save
// racing a background availability-sweep save for the same resource type)
// can interleave: each reads the other's stale pre-write TypeFile, and
// whichever's Put+SaveType lands last wins with a Count/Rows pairing that
// never itself violated the persisted-pair invariant (runtime/probes.go) but does not reflect
// either write in full (e.g. one call's Count together with the other
// call's Rows). cache.Store also has no internal locking of its own — two
// goroutines writing s.types[shortName] concurrently is a data race on the
// map, independent of the logical inconsistency above.
//
// fn must not call back into WithCacheStore/EnsureCacheStore/CurrentPair/
// SetProfileRegion (Session's mutex is not reentrant) and should do no
// blocking I/O beyond store.SaveType. Always calls fn (never skips it) —
// when profile/region has not resolved yet, fn receives a nil store, matching
// EnsureCacheStore's nil-store contract; callers already check for a nil
// store inside fn (see SaveAvailabilityCache). The NoCache short-circuit
// lives one layer up, in Core.WithCacheStore/Core.ReadCacheStore, which never
// call down into this method at all when NoCache is set.
func (s *Session) WithCacheStore(fn func(store *cache.Store) error) error {
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	return fn(s.ensureCacheStoreLocked(s.Profile, s.Region))
}

// ReadCacheStore runs fn against the current Profile/Region pair's
// *cache.Store while holding pairMu, for callers that only read
// (store.Type/store.Types) and never Put/SaveType. Pairs with WithCacheStore
// (the store-lock serialization, D13): a reader that bypassed the lock (the
// shape every read call site once had) could observe cache.Store's internal map mid-write from
// a concurrent WithCacheStore call — a data race on the map itself,
// independent of the logical Count/Rows consistency WithCacheStore's callers
// already guard. Same nil-store-to-fn contract as WithCacheStore.
func (s *Session) ReadCacheStore(fn func(store *cache.Store) error) error {
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	return fn(s.ensureCacheStoreLocked(s.Profile, s.Region))
}

// CurrentGenFor implements messages.GenSource. It maps an Aspect to the
// corresponding session generation counter so the central guard in
// Core.HandleEvent can check staleness without importing session.
func (s *Session) CurrentGenFor(a messages.Aspect) domain.Gen {
	switch a {
	case messages.AspectAvailability:
		return s.AvailabilityGen
	case messages.AspectEnrichment:
		return s.EnrichmentGen
	case messages.AspectRelated:
		return s.RelatedGen
	case messages.AspectEnrichDetail:
		return s.EnrichGen
	case messages.AspectConnect:
		return s.ConnectGen
	}
	return 0
}

// SetProbeStatus records shortName's most recent scan-status outcome.
func (s *Session) SetProbeStatus(shortName string, rec ProbeStatusRecord) {
	s.probeStatusMu.Lock()
	defer s.probeStatusMu.Unlock()
	if s.ProbeStatus == nil {
		s.ProbeStatus = make(map[string]ProbeStatusRecord)
	}
	s.ProbeStatus[shortName] = rec
}

// GetProbeStatus returns shortName's most recent scan-status record, if any.
func (s *Session) GetProbeStatus(shortName string) (ProbeStatusRecord, bool) {
	s.probeStatusMu.Lock()
	defer s.probeStatusMu.Unlock()
	rec, ok := s.ProbeStatus[shortName]
	return rec, ok
}

// AllProbeStatus returns a defensive copy of every recorded scan-status
// entry, safe for the caller to range over without holding probeStatusMu.
func (s *Session) AllProbeStatus() map[string]ProbeStatusRecord {
	s.probeStatusMu.Lock()
	defer s.probeStatusMu.Unlock()
	out := make(map[string]ProbeStatusRecord, len(s.ProbeStatus))
	maps.Copy(out, s.ProbeStatus)
	return out
}

// EnrichmentTypeGenGet returns shortName's per-type Wave-2 enrichment
// counter. Zero when no enrichment has run yet for the type.
func (s *Session) EnrichmentTypeGenGet(shortName string) domain.Gen {
	s.enrichmentTypeGenMu.Lock()
	defer s.enrichmentTypeGenMu.Unlock()
	return s.EnrichmentTypeGen[shortName]
}

// EnrichmentTypeGenBump increments shortName's per-type Wave-2 counter and
// returns the new value.
func (s *Session) EnrichmentTypeGenBump(shortName string) domain.Gen {
	s.enrichmentTypeGenMu.Lock()
	defer s.enrichmentTypeGenMu.Unlock()
	s.EnrichmentTypeGen[shortName]++
	return s.EnrichmentTypeGen[shortName]
}

// EnrichmentTypeGenSnapshot returns a defensive copy of every per-type
// Wave-2 counter, safe for a dispatch-time snapshot (Core.CaptureDispatch)
// to carry into an async goroutine without aliasing the live map.
func (s *Session) EnrichmentTypeGenSnapshot() map[string]domain.Gen {
	s.enrichmentTypeGenMu.Lock()
	defer s.enrichmentTypeGenMu.Unlock()
	return maps.Clone(s.EnrichmentTypeGen)
}

// EnrichmentTypeGenReset replaces the per-type Wave-2 counter map wholesale
// with a fresh, empty one (global refresh / Rotate).
func (s *Session) EnrichmentTypeGenReset() {
	s.enrichmentTypeGenMu.Lock()
	defer s.enrichmentTypeGenMu.Unlock()
	s.EnrichmentTypeGen = make(map[string]domain.Gen)
}

// SetNewFindingPairs replaces shortName's new-finding-pair counts wholesale
// with counts (#463) — used by a non-authoritative save (each one is a fresh
// one-step scan baseline, so REPLACE is correct there). A wave2Authoritative
// save must use MergeNewFindingPairs instead (#463 defect 2): it diffs
// against the just-written Wave-1 generation, so a wholesale replace here
// would wipe the Wave-1 new-pair counts that same sweep's earlier save just
// recorded.
func (s *Session) SetNewFindingPairs(shortName string, counts map[domain.FindingCode]int) {
	s.newFindingPairsMu.Lock()
	defer s.newFindingPairsMu.Unlock()
	if s.NewFindingPairs == nil {
		s.NewFindingPairs = make(map[string]map[domain.FindingCode]int)
	}
	s.NewFindingPairs[shortName] = counts
}

// MergeNewFindingPairs adds counts onto shortName's existing new-finding-pair
// entry, summing per code, rather than replacing it (#463 defect 2) — the
// Wave-2-completion save's counterpart to SetNewFindingPairs, used so its
// own (Wave-2-sourced) new pairs accumulate onto whatever the sweep's
// earlier Wave-1 save already recorded this cycle instead of wiping it.
// No-op when counts is empty, so a clean rerun with nothing new never
// touches the map. Cannot double-count a pair across the two saves in one
// cycle: stampFindingFirstSeen only reports a pair as new relative to the
// on-disk generation it diffed against, and the second save's diff already
// sees the first save's just-written generation.
func (s *Session) MergeNewFindingPairs(shortName string, counts map[domain.FindingCode]int) {
	if len(counts) == 0 {
		return
	}
	s.newFindingPairsMu.Lock()
	defer s.newFindingPairsMu.Unlock()
	if s.NewFindingPairs == nil {
		s.NewFindingPairs = make(map[string]map[domain.FindingCode]int)
	}
	existing := s.NewFindingPairs[shortName]
	merged := make(map[domain.FindingCode]int, len(existing)+len(counts))
	maps.Copy(merged, existing)
	for code, n := range counts {
		merged[code] += n
	}
	s.NewFindingPairs[shortName] = merged
}

// GetNewFindingPairs returns shortName's most recently recorded new-finding-
// pair counts, if any.
func (s *Session) GetNewFindingPairs(shortName string) (map[domain.FindingCode]int, bool) {
	s.newFindingPairsMu.Lock()
	defer s.newFindingPairsMu.Unlock()
	rec, ok := s.NewFindingPairs[shortName]
	return rec, ok
}

// AllNewFindingPairs returns a defensive deep copy of every recorded
// new-finding-pair entry, safe for the caller to range over (and mutate)
// without holding newFindingPairsMu or aliasing the stored per-type maps.
func (s *Session) AllNewFindingPairs() map[string]map[domain.FindingCode]int {
	s.newFindingPairsMu.Lock()
	defer s.newFindingPairsMu.Unlock()
	out := make(map[string]map[domain.FindingCode]int, len(s.NewFindingPairs))
	for shortName, counts := range s.NewFindingPairs {
		out[shortName] = maps.Clone(counts)
	}
	return out
}

// Rotate rotates the session when the user switches profile or region. Every
// generation counter is bumped so that in-flight async messages tagged with
// the pre-switch gens are rejected by the handlers' gen guards; all cached
// rows, findings, and queues are cleared so the next session wires up on a
// clean slate.
//
// Callers (handleProfileSelected / handleRegionSelected) retain responsibility
// for UI shell state (header flash, view stack pop, menu availability reset)
// — this method touches only Session-owned fields.
func (s *Session) Rotate() {
	s.RelatedCache.Clear()
	s.FilteredRows.Clear()
	s.RelatedGen.Bump()
	s.EnrichGen.Bump()
	s.AvailabilityGen.Bump()
	s.EnrichmentGen.Bump()
	s.ConnectGen.Bump()

	// Session-identity / rollback-latch / fetch-latch fields. Profile/Region/
	// Clients/PreSuppliedClients/Command/NoCache are deliberately NOT cleared
	// — the caller (handleProfileSelected / handleRegionSelected) is responsible
	// for setting Profile/Region to the new target, and for capturing rollback
	// state via local vars BEFORE Rotate (so the rapid A→B→C case keeps A as
	// the rollback target).
	// C9: drop the old pair's Store atomically, under the same lock
	// EnsureCacheStore uses, so a concurrent EnsureCacheStore call cannot
	// observe a torn state (old Store with a stale/zeroed pair stamp, or
	// vice versa). The new pair's Store is re-obtained via a fresh
	// cache.LoadDir call dispatched by the pair-switch handler
	// (TaskKindLoadAvailCache), never carried over from the old pair.
	s.pairMu.Lock()
	s.CacheStore = nil
	s.cacheStoreProfile = ""
	s.cacheStoreRegion = ""
	s.pairMu.Unlock()

	s.Identity = nil
	s.IdentityFetching = false
	s.PendingRefresh = false
	s.HasPrevState = false
	s.PrevProfile = ""
	s.PrevRegion = ""

	s.EnrichQueue = nil
	s.AvailQueue = nil
	s.AvailChecked = 0
	s.AvailTotal = 0
	s.AvailSweepPending = false
	s.EnrichChecked = 0
	s.EnrichTotal = 0
	s.RowStore.Clear()
	s.EnrichmentRan = make(map[string]bool)
	s.EnrichmentTypeGenReset()
	s.EnrichmentTruncatedIDs = make(map[string]map[string]bool)
	// EnrichSweepMembers/EnrichListOpenPending: a prior profile/region's
	// in-flight sweep-window/list-open bookkeeping must not leak into the
	// next pair's scan — same rationale as EnrichmentRan/EnrichmentTypeGen
	// just above.
	s.EnrichSweepMembers = nil
	s.EnrichListOpenPending = nil

	// ProbeStatus: a prior profile/region's scan-status records must not
	// leak into the next pair's scan (#462) — same rationale as the
	// EnrichmentRan/EnrichmentTypeGen resets just above.
	s.probeStatusMu.Lock()
	s.ProbeStatus = nil
	s.probeStatusMu.Unlock()

	// NewFindingPairs: same rationale — a prior profile/region's new-finding
	// deltas must not leak into the next pair's scan (#463).
	s.newFindingPairsMu.Lock()
	s.NewFindingPairs = nil
	s.newFindingPairsMu.Unlock()

	// Feature caches: swap the PolicyDocumentCache for a fresh instance so
	// documents fetched in the previous account cannot leak into the next.
	s.PolicyDocCache = &awsclient.PolicyDocumentCache{}

	// IAMPolicies: reset to a fresh store so managed/inline entries from the
	// prior account/profile cannot leak into the next session.
	s.IAMPolicies = NewPolicyStore()

	// IdentityStore: reset to a fresh store so the cached account ID + sticky
	// failure (if any) from the prior session cannot leak into the next.
	s.IdentityStore = NewIdentityStore()

	// RuleSets: reset to a fresh store so the cached SES rule set from the
	// prior session cannot leak into the next.
	s.RuleSets = NewRuleSetStore()

	// SweptPairs: deliberately NOT cleared — session-lifetime by design, see
	// SweptPairs doc.
}
