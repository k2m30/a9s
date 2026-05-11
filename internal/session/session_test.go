package session_test

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
)

// TestSession_New_InitializesMaps verifies that New() returns a *Session with
// every map field non-nil and generation counters seeded at 1 (so that
// Generation==0, the zero value, is always considered stale by gen guards).
func TestSession_New_InitializesMaps(t *testing.T) {
	t.Parallel()

	s := session.New()
	if s == nil {
		t.Fatal("session.New() returned nil")
	}

	// EnrichmentFindings was moved to tui.Model in PR-03a-fold; it is no
	// longer a Session field and is not initialized here.
	if s.EnrichmentRan == nil {
		t.Error("EnrichmentRan must be non-nil after New()")
	}
	if s.EnrichmentTypeGen == nil {
		t.Error("EnrichmentTypeGen must be non-nil after New()")
	}
	if s.EnrichmentTruncatedIDs == nil {
		t.Error("EnrichmentTruncatedIDs must be non-nil after New()")
	}
	if s.ResourceCache == nil {
		t.Error("ResourceCache must be non-nil after New()")
	}
	if s.LazyResourceCache == nil {
		t.Error("LazyResourceCache must be non-nil after New()")
	}
	if s.RelatedCache == nil {
		t.Error("RelatedCache must be non-nil after New()")
	}
	if s.PolicyDocCache == nil {
		t.Error("PolicyDocCache must be non-nil after New()")
	}

	// Seed=1 convention: generation counters start at 1 so that Gen==0
	// (unset in synthetic messages) is always rejected by gen guards.
	if s.RelatedGen != 1 {
		t.Errorf("RelatedGen = %d, want 1", s.RelatedGen)
	}
	if s.EnrichGen != 1 {
		t.Errorf("EnrichGen = %d, want 1", s.EnrichGen)
	}
	if s.EnrichmentGen != 1 {
		t.Errorf("EnrichmentGen = %d, want 1", s.EnrichmentGen)
	}
}

// TestSession_Rotate_BumpsGenerations verifies that Rotate() increments every
// generation counter by exactly 1, so in-flight messages from before the
// rotation are rejected by their respective gen guards.
func TestSession_Rotate_BumpsGenerations(t *testing.T) {
	t.Parallel()

	s := session.New()

	s.RelatedGen = 5
	s.EnrichGen = 5
	s.AvailabilityGen = 5
	s.EnrichmentGen = 5

	s.Rotate()

	if s.RelatedGen != 6 {
		t.Errorf("RelatedGen after Rotate() = %d, want 6", s.RelatedGen)
	}
	if s.EnrichGen != 6 {
		t.Errorf("EnrichGen after Rotate() = %d, want 6", s.EnrichGen)
	}
	if s.AvailabilityGen != 6 {
		t.Errorf("AvailabilityGen after Rotate() = %d, want 6", s.AvailabilityGen)
	}
	if s.EnrichmentGen != 6 {
		t.Errorf("EnrichmentGen after Rotate() = %d, want 6", s.EnrichmentGen)
	}
}

// TestSession_Rotate_ClearsCaches verifies that Rotate() empties every
// per-session cache so stale results from the previous account/region cannot
// surface after a profile switch.
func TestSession_Rotate_ClearsCaches(t *testing.T) {
	t.Parallel()

	s := session.New()

	// Populate caches with data from a fictional "old session".
	// Note: EnrichmentFindings was moved to tui.Model in PR-03a-fold and is
	// no longer on Session; it is cleared explicitly by profile/region switch
	// handlers in handleProfileSelected / handleRegionSelected.
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{}
	s.LazyResourceCache["ec2"] = nil
	s.EnrichmentRan["ec2"] = true
	s.EnrichmentTypeGen["ec2"] = 42
	s.EnrichmentTruncatedIDs["ec2"] = map[string]bool{"i-x": true}
	s.RelatedCache.Set("ec2:i-001", []session.RelatedCacheResult{
		{DefDisplayName: "VPCs"},
	})

	s.Rotate()

	if len(s.ResourceCache) != 0 {
		t.Errorf("ResourceCache not empty after Rotate(): len=%d", len(s.ResourceCache))
	}
	if len(s.LazyResourceCache) != 0 {
		t.Errorf("LazyResourceCache not empty after Rotate(): len=%d", len(s.LazyResourceCache))
	}
	// EnrichmentFindings is on tui.Model (not Session) after PR-03a-fold;
	// Session.Rotate() no longer clears it — the handler does so explicitly.
	if len(s.EnrichmentRan) != 0 {
		t.Errorf("EnrichmentRan not empty after Rotate(): len=%d", len(s.EnrichmentRan))
	}
	if len(s.EnrichmentTypeGen) != 0 {
		t.Errorf("EnrichmentTypeGen not empty after Rotate(): len=%d", len(s.EnrichmentTypeGen))
	}
	if len(s.EnrichmentTruncatedIDs) != 0 {
		t.Errorf("EnrichmentTruncatedIDs not empty after Rotate(): len=%d", len(s.EnrichmentTruncatedIDs))
	}
	if s.RelatedCache.Len() != 0 {
		t.Errorf("RelatedCache not empty after Rotate(): len=%d", s.RelatedCache.Len())
	}
}

// TestSession_Rotate_ResetsQueueState verifies that Rotate() zeros all
// in-flight queue scalars and nils all queue slices/maps so the next session
// starts on a clean slate.
func TestSession_Rotate_ResetsQueueState(t *testing.T) {
	t.Parallel()

	s := session.New()

	s.AvailQueue = []string{"ec2"}
	s.AvailChecked = 3
	s.AvailTotal = 10
	s.EnrichQueue = []string{"rds"}
	s.EnrichChecked = 2
	s.EnrichTotal = 5
	s.ProbeResources = map[string][]resource.Resource{
		"ec2": {{}},
	}
	s.ProbeTruncated = map[string]bool{"ec2": true}

	s.Rotate()

	if s.AvailChecked != 0 {
		t.Errorf("AvailChecked after Rotate() = %d, want 0", s.AvailChecked)
	}
	if s.AvailTotal != 0 {
		t.Errorf("AvailTotal after Rotate() = %d, want 0", s.AvailTotal)
	}
	if s.EnrichChecked != 0 {
		t.Errorf("EnrichChecked after Rotate() = %d, want 0", s.EnrichChecked)
	}
	if s.EnrichTotal != 0 {
		t.Errorf("EnrichTotal after Rotate() = %d, want 0", s.EnrichTotal)
	}
	if s.AvailQueue != nil {
		t.Errorf("AvailQueue after Rotate() = %v, want nil", s.AvailQueue)
	}
	if s.EnrichQueue != nil {
		t.Errorf("EnrichQueue after Rotate() = %v, want nil", s.EnrichQueue)
	}
	if s.ProbeResources != nil {
		t.Errorf("ProbeResources after Rotate() = %v, want nil", s.ProbeResources)
	}
	if s.ProbeTruncated != nil {
		t.Errorf("ProbeTruncated after Rotate() = %v, want nil", s.ProbeTruncated)
	}
}

// TestSession_Rotate_ConnectGenAndIdentityLatches pins the PR-05a-h2 contract
// for the lifecycle fields that Rotate() OWNS: ConnectGen must increment by
// exactly 1, and Identity / IdentityFetching / PendingRefresh / HasPrevState /
// PrevProfile / PrevRegion must all be zeroed.
//
// Without this test a future regression that forgets to bump ConnectGen would
// allow a pre-switch ClientsReadyMsg to be accepted as fresh after a profile
// switch, and a regression that fails to clear Identity / IdentityFetching
// would leak the previous account's caller identity into the new session.
func TestSession_Rotate_ConnectGenAndIdentityLatches(t *testing.T) {
	t.Parallel()

	s := session.New()

	// Seed the latches with non-zero values from a fictional pre-switch state.
	s.ConnectGen = 5
	s.Identity = &awsclient.CallerIdentity{AccountID: "111122223333"}
	s.IdentityFetching = true
	s.PendingRefresh = true
	s.HasPrevState = true
	s.PrevProfile = "old-profile"
	s.PrevRegion = "us-east-1"

	s.Rotate()

	if s.ConnectGen != 6 {
		t.Errorf("ConnectGen after Rotate() = %d, want 6", s.ConnectGen)
	}
	if s.Identity != nil {
		t.Errorf("Identity after Rotate() = %+v, want nil — pre-switch identity must not leak", s.Identity)
	}
	if s.IdentityFetching {
		t.Error("IdentityFetching after Rotate() = true, want false — in-flight latch must be cleared")
	}
	if s.PendingRefresh {
		t.Error("PendingRefresh after Rotate() = true, want false")
	}
	if s.HasPrevState {
		t.Error("HasPrevState after Rotate() = true, want false — rollback latch must be cleared so the caller can re-seed it")
	}
	if s.PrevProfile != "" {
		t.Errorf("PrevProfile after Rotate() = %q, want \"\"", s.PrevProfile)
	}
	if s.PrevRegion != "" {
		t.Errorf("PrevRegion after Rotate() = %q, want \"\"", s.PrevRegion)
	}
}

// TestSession_Rotate_PreservesProfileRegionClientsCommandNoCache pins the
// PR-05a-h2 preserve-list contract. Rotate() MUST NOT touch Profile, Region,
// Clients, PreSuppliedClients, Command, or NoCache — the caller
// (handleProfileSelected / handleRegionSelected / cmd/a9s/main.go bootstrap)
// owns those fields and writes the next target into Profile/Region
// immediately after Rotate. Clearing them inside Rotate would either drop the
// just-written target or silently undo bootstrap state (e.g. demo --clients).
func TestSession_Rotate_PreservesProfileRegionClientsCommandNoCache(t *testing.T) {
	t.Parallel()

	s := session.New()

	// Seed the preserve-list. Use distinguishable values so a regression that
	// zeros them is obvious in the test output.
	preClients := &awsclient.ServiceClients{}
	preSupplied := &awsclient.ServiceClients{}
	s.Profile = "dev-account"
	s.Region = "eu-central-1"
	s.Clients = preClients
	s.PreSuppliedClients = preSupplied
	s.Command = "ec2"
	s.NoCache = true

	s.Rotate()

	if s.Profile != "dev-account" {
		t.Errorf("Profile after Rotate() = %q, want %q (preserve-list)", s.Profile, "dev-account")
	}
	if s.Region != "eu-central-1" {
		t.Errorf("Region after Rotate() = %q, want %q (preserve-list)", s.Region, "eu-central-1")
	}
	if s.Clients != preClients {
		t.Errorf("Clients after Rotate() = %p, want %p (preserve-list — caller swaps after reconnect)", s.Clients, preClients)
	}
	if s.PreSuppliedClients != preSupplied {
		t.Errorf("PreSuppliedClients after Rotate() = %p, want %p (preserve-list — demo/test bootstrap channel)", s.PreSuppliedClients, preSupplied)
	}
	if s.Command != "ec2" {
		t.Errorf("Command after Rotate() = %q, want %q (preserve-list — one-shot CLI -c flag)", s.Command, "ec2")
	}
	if !s.NoCache {
		t.Error("NoCache after Rotate() = false, want true (preserve-list — CLI --no-cache flag)")
	}
}

// TestSession_Rotate_SwapsPolicyDocCache verifies that Rotate() replaces the
// PolicyDocCache with a fresh instance. The old pointer must not be reused so
// that documents fetched in a previous account cannot leak into the next.
func TestSession_Rotate_SwapsPolicyDocCache(t *testing.T) {
	t.Parallel()

	s := session.New()

	// Capture pointer identity before rotation.
	beforePtr := s.PolicyDocCache

	s.Rotate()

	afterPtr := s.PolicyDocCache

	if afterPtr == nil {
		t.Fatal("PolicyDocCache is nil after Rotate() — must be a fresh non-nil instance")
	}
	if afterPtr == beforePtr {
		t.Error("PolicyDocCache pointer is identical after Rotate(): old cache may leak across accounts")
	}
}
