package session_test

import (
	"testing"

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
