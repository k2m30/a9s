// Package session owns the in-memory orchestration state for the active
// profile/region session: fetch-availability queue, Wave 2 enrichment queue,
// per-type caches, and the generation counters that invalidate stale async
// results on profile/region switch or refresh.
//
// Session is held as Session *session.Session on tui.Model. Access sites use
// m.Session.ResourceCache, m.Session.ProbeResources, m.Session.RelatedGen etc.
// directly.
//
// Rules of ownership:
//
//   - Only session-scoped orchestration state belongs here. UI shell concerns
//     (view stack, header, input mode, theme) stay on the surrounding Model.
//   - Maps that handler paths write into directly (ResourceCache,
//     EnrichmentRan, EnrichmentTypeGen, EnrichmentTruncatedIDs) MUST be
//     constructed by New(). ProbeResources and the availability/enrich queues
//     stay nil until a probe retains its first batch — they are built in place.
//   - EnrichmentFindings was removed in PR-03a-fold; it now lives directly on
//     tui.Model so it is not subject to Session.Rotate() clearing. The Model
//     owner (handleProfileSelected / handleRegionSelected) clears it explicitly.
//   - Session rotation (profile/region switch) MUST bump every generation and
//     replace/clear the caches, so in-flight messages tagged with old gens are
//     discarded by the handlers' gen guards.
package session

import (
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ResourceCacheEntry stores the state of a previously-viewed resource list.
// Used to restore the list when the user re-enters the same resource type
// from the main menu, avoiding redundant API calls.
type ResourceCacheEntry struct {
	Resources     []resource.Resource
	Pagination    *resource.PaginationMeta
	FilterText    string
	AttentionOnly bool // §7.3: ctrl+z toggle persisted across view re-entry
	SortColIdx    int
	SortAsc       bool
	CursorPos     int
	HScrollOffset int
}

// Session owns the in-memory orchestration state for the active
// profile/region session.
type Session struct {
	// Session identity — set by the caller (tui.New / handler) before/after Rotate.
	Profile string
	Region  string

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
	// active resource list (set by profile/region switch handlers). Cleared by
	// Rotate; re-set to true after Rotate in the switch handlers.
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

	// NoCache disables on-disk availability caching and background probes
	// (set by the --no-cache / --demo CLI flags). Survives Rotate — it is a
	// static policy, not session state.
	NoCache bool

	// Wave 1 availability scan.
	AvailabilityGen domain.Gen // bumped on profile/region switch to cancel stale probes
	AvailQueue      []string // resource short names remaining to probe
	AvailChecked    int      // number probed so far in current gen
	AvailTotal      int      // total types to probe in current gen

	// Wave 2 issue-enrichment dispatch.
	ProbeResources map[string][]resource.Resource // retained first-page resources from Wave 1
	ProbeTruncated map[string]bool                // per-type truncation signal from Wave 1 probe
	EnrichQueue    []string                       // resource types pending Wave 2 enrichment
	EnrichmentGen  domain.Gen                     // session-wide gen counter for Wave 2
	EnrichChecked  int                            // number of enrichment probes completed in current gen
	EnrichTotal    int                            // total enrichment probes to run in current gen

	// Per-type Wave 2 finding state (feature 018-enrichment-visibility).
	// NOTE: EnrichmentFindings was moved to tui.Model in PR-03a-fold so that
	// it survives Session.Rotate() and is cleared explicitly by the profile/
	// region switch handlers. The remaining maps stay here because they do not
	// need to persist across a Rotate().
	EnrichmentRan          map[string]bool
	EnrichmentTypeGen      map[string]domain.Gen
	EnrichmentTruncatedIDs map[string]map[string]bool

	// Session-scoped caches + stale-result guards.
	ResourceCache map[string]*ResourceCacheEntry
	// LazyResourceCache holds resources pulled via FetchByIDs for filtered-target
	// drills. Consulted by related-navigation only; NEVER by top-level list
	// navigation. Ensures lazy-added out-of-scope entries (e.g. AWS-managed KMS
	// keys) do not pollute the scope-filtered main-menu list.
	LazyResourceCache map[string][]resource.Resource
	RelatedCache      *RelatedCacheLRU
	RelatedGen        domain.Gen // bumped on refresh/profile/region switch
	EnrichGen         domain.Gen // bumped on refresh/profile/region switch (detail-enrichment only)
	EnrichResKey      string // "resourceType:resourceID" of last detail-enrichment dispatch

	// Feature-specific session caches. These used to hang off *ServiceClients
	// but that blurred the AWS-transport/session-state boundary; they live
	// here instead and are passed to detail enrichers via DetailEnrichmentCtx.
	PolicyDocCache *awsclient.PolicyDocumentCache

	// IAMPolicies is the per-session cache for IAM policy resources, keyed by
	// both PolicyName and ARN. Replaces the package-level globals previously in
	// internal/aws/iam_policies.go. Wired into *ServiceClients.IAMPolicies on
	// every ClientsReadyMsg so FetchIAMPoliciesByIDsFull uses the session store.
	IAMPolicies PolicyStore

	// IdentityStore is the per-session cache for the AWS caller's account ID
	// used by Pattern-C related checkers. Replaces the package-level globals
	// previously in internal/aws/identity_cache.go (identityCacheMu /
	// cachedAccountID / cachedAccountErr). Wired into *ServiceClients.
	// IdentityStore on every ClientsReadyMsg so Pattern-C related checkers
	// (Glue tags, EBS Backup) see a per-profile/region scoped cache rather
	// than a process-global one. Distinct from Session.Identity (the resolved
	// *awsclient.CallerIdentity) which holds the human-readable identity
	// metadata for the header / IdentityModel.
	IdentityStore IdentityStore

	// RuleSets is the per-session, single-slot cache for the SES v1
	// DescribeActiveReceiptRuleSet response. Replaces the package-level
	// globals previously in internal/aws/ses_related.go (sesRuleSetCacheMu
	// + sesRuleSetCaches map keyed by *ServiceClients pointer). Wired into
	// *ServiceClients.RuleSets on every ClientsReadyMsg so checkSESLambda /
	// checkSESS3 see a session-scoped cache rather than a process-global map.
	RuleSets RuleSetStore
}

// New constructs a fresh Session with all maps initialized and generation
// counters seeded at 1. The seed=1 convention makes Generation=0 (unset)
// always stale, so synthetic test messages or early-return paths that leave
// Gen at its zero value are rejected by the gen guards.
func New() *Session {
	return &Session{
		ProbeResources:         nil, // initialized lazily on first probe retention
		EnrichmentRan:          make(map[string]bool),
		EnrichmentTypeGen:      make(map[string]domain.Gen),
		EnrichmentTruncatedIDs: make(map[string]map[string]bool),
		ResourceCache:          make(map[string]*ResourceCacheEntry),
		LazyResourceCache:      make(map[string][]resource.Resource),
		RelatedCache:           NewRelatedCacheLRU(MaxRelatedCacheEntries),
		RelatedGen:             1,
		EnrichGen:              1,
		EnrichmentGen:          1,
		PolicyDocCache:         &awsclient.PolicyDocumentCache{},
		IAMPolicies:            NewPolicyStore(),
		IdentityStore:          NewIdentityStore(),
		RuleSets:               NewRuleSetStore(),
	}
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
	s.Identity = nil
	s.IdentityFetching = false
	s.PendingRefresh = false
	s.HasPrevState = false
	s.PrevProfile = ""
	s.PrevRegion = ""

	s.EnrichQueue = nil
	s.ProbeResources = nil
	s.ProbeTruncated = nil
	s.AvailQueue = nil
	s.AvailChecked = 0
	s.AvailTotal = 0
	s.EnrichChecked = 0
	s.EnrichTotal = 0
	s.ResourceCache = make(map[string]*ResourceCacheEntry)
	s.LazyResourceCache = make(map[string][]resource.Resource)
	s.EnrichmentRan = make(map[string]bool)
	s.EnrichmentTypeGen = make(map[string]domain.Gen)
	s.EnrichmentTruncatedIDs = make(map[string]map[string]bool)

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
}
