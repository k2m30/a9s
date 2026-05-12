// generation_stamping_fetch_test.go — regression tests for AS-657 session-stamp
// guards on ResourcesLoaded / APIError / IdentityLoaded / IdentityError / ValueRevealed.
//
// These tests FAIL TO COMPILE on main (before AS-657 lands) because the five
// message types in internal/runtime/messages/event.go do not yet carry a Gen
// field. Once Coder adds:
//   - Gen domain.Gen on each of the five types
//   - GenStamp() / GenAspect() / AcceptZeroGen() methods
//   - messages.IsStale guards at the top of each case branch in app.go
// the tests compile AND pass.
//
// AC coverage:
//   AC #1 — stale ResourcesLoaded is dropped (resources unchanged, cache not poisoned)
//   AC #2 — stale IdentityLoaded is dropped (Session.Identity unchanged, header unchanged)
//   AC #3 — stale ValueRevealed is dropped (reveal view not pushed, secret not rendered)
//   AC #4 — happy path with matching gen applies all three message types
//   AC #5 — stale APIError does not flash; stale IdentityError does not flash
//
// Harness pattern follows qa_clients_ready_flash_gen_test.go: real Session,
// Rotate() to bump counters, synthesised messages with stale stamps, assert
// observable state after Update().
package unit

import (
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ── AC #1 — stale ResourcesLoaded is dropped ────────────────────────────────

// TestResourcesLoaded_Stale_Dropped verifies that a ResourcesLoaded arriving
// after Session.Rotate() with Gen == pre-rotate AvailabilityGen is silently
// discarded: the active resource list and the write-through ResourceCache must
// remain unchanged.
//
// Reverting the messages.IsStale guard in app.go (case messages.ResourcesLoaded)
// makes this test fail because the stale resources overwrite m.allResources.
//
// Setup: two Rotate() calls ensure staleGen > 0. AcceptZeroGen=true means a
// Gen=0 stamp is never stale; we need a non-zero staleGen to truly test the guard.
func TestResourcesLoaded_Stale_Dropped(t *testing.T) {
	m := newRootSizedModel()

	// Rotate once so AvailabilityGen becomes non-zero (starts at 0).
	// This ensures staleGen > 0, bypassing AcceptZeroGen=true.
	m.Session().Rotate()

	// Navigate to ec2 list and load sentinel resources with the current (non-zero) gen.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	sentinel := []resource.Resource{
		{ID: "i-before-rotate", Name: "before-rotate-server"},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    sentinel,
		Gen:          m.Session().AvailabilityGen, // current non-zero gen — matches
	})
	// Verify baseline: sentinel resource is present.
	if got := m.ActiveListResources(); len(got) == 0 || got[0].ID != "i-before-rotate" {
		t.Fatalf("baseline: expected sentinel resource i-before-rotate in list, got %v", got)
	}

	// Capture stale gen (non-zero) BEFORE the second Rotate.
	staleGen := m.Session().AvailabilityGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard (AcceptZeroGen=true would pass zero)")
	}
	// Second Rotate bumps AvailabilityGen to staleGen+1.
	m.Session().Rotate()

	// Dispatch a stale ResourcesLoaded (Gen == staleGen, current is staleGen+1).
	staleMsg := messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-stale-account", Name: "stale-server"}},
		Gen:          staleGen,
	}
	m, _ = rootApplyMsg(m, staleMsg)

	// Assert: active list resources must still be the sentinel (stale was dropped).
	got := m.ActiveListResources()
	for _, r := range got {
		if r.ID == "i-stale-account" {
			t.Errorf("stale ResourcesLoaded was NOT dropped: found i-stale-account in list after Rotate() — guard regression")
		}
	}
}

// ── AC #2 — stale IdentityLoaded is dropped ─────────────────────────────────

// TestIdentityLoaded_Stale_Dropped verifies that a IdentityLoaded arriving after
// Session.Rotate() with a stale ConnectGen stamp is discarded: Session.Identity
// must remain nil (the pre-rotate Identity was cleared by Rotate()).
//
// This closes the "old account ID in header after profile switch" bug.
// Reverting the IsStale guard in app.go (case messages.IdentityLoaded) makes
// this test fail because Session.Identity is set to the stale value.
//
// Setup: Rotate() once to make ConnectGen non-zero, then capture staleGen,
// then Rotate() again. AcceptZeroGen=true means Gen=0 is never stale.
func TestIdentityLoaded_Stale_Dropped(t *testing.T) {
	m := newRootSizedModel()

	// First Rotate: ConnectGen becomes non-zero.
	m.Session().Rotate()

	// Capture stale ConnectGen (non-zero) BEFORE the second Rotate.
	staleGen := m.Session().ConnectGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard")
	}
	// Second Rotate: ConnectGen becomes staleGen+1, Identity cleared.
	m.Session().Rotate()

	// Session.Identity is nil after Rotate() (per session.go:233).
	if m.Session().Identity != nil {
		t.Fatal("precondition failed: Session.Identity should be nil after Rotate()")
	}

	// Dispatch stale IdentityLoaded with pre-rotate stamp.
	staleIdentity := &awsclient.CallerIdentity{AccountID: "111122223333"}
	staleMsg := messages.IdentityLoaded{
		Identity: staleIdentity,
		Gen:      staleGen,
	}
	m, _ = rootApplyMsg(m, staleMsg)

	// Session.Identity must still be nil — the stale message was dropped.
	if m.Session().Identity != nil {
		t.Errorf("stale IdentityLoaded was NOT dropped: Session.Identity = %+v, expected nil — guard regression", m.Session().Identity)
	}
}

// ── AC #3 — stale ValueRevealed is dropped ──────────────────────────────────

// TestValueRevealed_Stale_Dropped verifies that a ValueRevealed arriving after
// Session.Rotate() with a stale ConnectGen stamp is discarded: the reveal view
// must NOT be pushed, and the secret value must not appear in the rendered output.
//
// This closes the secret-value-leak path. Reverting the IsStale guard in app.go
// (case messages.ValueRevealed) makes this test fail because pushView is called
// with the stale secret.
//
// Setup: Rotate() once to make ConnectGen non-zero, then capture staleGen,
// then Rotate() again. AcceptZeroGen=true means Gen=0 is never stale.
func TestValueRevealed_Stale_Dropped(t *testing.T) {
	m := newRootSizedModel()

	// First Rotate: ConnectGen becomes non-zero.
	m.Session().Rotate()

	// Capture stale ConnectGen (non-zero) BEFORE the second Rotate.
	staleGen := m.Session().ConnectGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard")
	}
	// Second Rotate: ConnectGen becomes staleGen+1.
	m.Session().Rotate()

	// The secret value from a previous profile that must NOT appear.
	const staleSecret = "PREV_ACCOUNT_SECRET_VALUE_xyzzy_42"

	// Dispatch stale ValueRevealed with pre-rotate stamp and no error.
	staleMsg := messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "prod/api/key",
		Value:        staleSecret,
		Gen:          staleGen,
	}
	m, _ = rootApplyMsg(m, staleMsg)

	// The reveal view must not be in the rendered output.
	rendered := rootViewContent(m)
	plain := stripANSI(rendered)
	if strings.Contains(plain, staleSecret) {
		t.Errorf("stale ValueRevealed was NOT dropped: secret value %q appears in rendered output — guard regression (secret leak)", staleSecret)
	}
}

// ── AC #4 — matching gen (happy path) applies all three message types ────────

// TestHappyPath_MatchingGen_ResourcesLoaded verifies that a ResourcesLoaded with
// Gen == current AvailabilityGen is applied: resources appear in the list.
func TestHappyPath_MatchingGen_ResourcesLoaded(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	currentGen := m.Session().AvailabilityGen
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-fresh", Name: "fresh-server"}},
		Gen:          currentGen,
	})
	got := m.ActiveListResources()
	found := false
	for _, r := range got {
		if r.ID == "i-fresh" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("happy-path ResourcesLoaded with matching gen was not applied: resource i-fresh not found in %v", got)
	}
}

// TestHappyPath_MatchingGen_IdentityLoaded verifies that IdentityLoaded with
// Gen == current ConnectGen sets Session.Identity.
func TestHappyPath_MatchingGen_IdentityLoaded(t *testing.T) {
	m := newRootSizedModel()
	currentGen := m.Session().ConnectGen
	freshIdentity := &awsclient.CallerIdentity{AccountID: "999988887777"}
	m, _ = rootApplyMsg(m, messages.IdentityLoaded{
		Identity: freshIdentity,
		Gen:      currentGen,
	})
	if m.Session().Identity == nil {
		t.Fatal("happy-path IdentityLoaded with matching gen was not applied: Session.Identity is nil")
	}
	if m.Session().Identity.AccountID != "999988887777" {
		t.Errorf("Session.Identity.AccountID = %q, want 999988887777", m.Session().Identity.AccountID)
	}
}

// TestHappyPath_MatchingGen_ValueRevealed verifies that ValueRevealed with
// Gen == current ConnectGen pushes the reveal view.
func TestHappyPath_MatchingGen_ValueRevealed(t *testing.T) {
	m := newRootSizedModel()
	currentGen := m.Session().ConnectGen
	const freshSecret = "FRESH_CURRENT_SESSION_SECRET_abc123"
	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "prod/fresh-key",
		Value:        freshSecret,
		Gen:          currentGen,
	})
	rendered := rootViewContent(m)
	plain := stripANSI(rendered)
	if !strings.Contains(plain, freshSecret) {
		t.Errorf("happy-path ValueRevealed with matching gen was not applied: secret %q not in rendered output", freshSecret)
	}
}

// TestHappyPath_ZeroGen_AcceptedByAllThree verifies that Gen==0 is never treated
// as stale for ResourcesLoaded, IdentityLoaded, or ValueRevealed (AcceptZeroGen=true).
// This covers the --demo / --no-cache bootstrap where Rotate() has not been called
// and AvailabilityGen/ConnectGen start at zero.
func TestHappyPath_ZeroGen_AcceptedByAllThree(t *testing.T) {
	m := newRootSizedModel()

	// AvailabilityGen and ConnectGen start at 0 on a fresh session.
	if got := m.Session().AvailabilityGen; got != 0 {
		t.Skipf("test requires initial AvailabilityGen==0, got %d (Rotate already called)", got)
	}

	// ResourcesLoaded with Gen=0 must be applied.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-zero-gen", Name: "zero-gen-server"}},
		Gen:          domain.Gen(0),
	})
	got := m.ActiveListResources()
	found := false
	for _, r := range got {
		if r.ID == "i-zero-gen" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ResourcesLoaded with Gen=0 was dropped on fresh session — AcceptZeroGen=true regression")
	}

	// IdentityLoaded with Gen=0 must be applied.
	m2 := newRootSizedModel()
	freshID := &awsclient.CallerIdentity{AccountID: "zero-gen-account"}
	m2, _ = rootApplyMsg(m2, messages.IdentityLoaded{
		Identity: freshID,
		Gen:      domain.Gen(0),
	})
	if m2.Session().Identity == nil {
		t.Error("IdentityLoaded with Gen=0 was dropped on fresh session — AcceptZeroGen=true regression")
	}

	// ValueRevealed with Gen=0 must be applied (reveal view pushed).
	m3 := newRootSizedModel()
	const zeroGenSecret = "ZERO_GEN_SECRET_demo_mode_xyz"
	m3, _ = rootApplyMsg(m3, messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "demo/key",
		Value:        zeroGenSecret,
		Gen:          domain.Gen(0),
	})
	plain3 := stripANSI(rootViewContent(m3))
	if !strings.Contains(plain3, zeroGenSecret) {
		t.Errorf("ValueRevealed with Gen=0 was dropped on fresh session — AcceptZeroGen=true regression; rendered: %q", plain3)
	}
}

// ── AC #5 — stale APIError / IdentityError do not flash ─────────────────────

// TestAPIError_Stale_NoFlash verifies that a stale APIError (Gen == pre-rotate
// AvailabilityGen) does not advance flash.gen or render a flash message.
//
// The danger: without the guard, a slow "fetch ec2: ..." error from a previous
// region arrives after the user switched and flashes a confusing error about a
// region they already left.
//
// Setup: Rotate() once to make AvailabilityGen non-zero before capturing staleGen,
// then Rotate() again. AcceptZeroGen=true means Gen=0 is never stale.
func TestAPIError_Stale_NoFlash(t *testing.T) {
	m := newRootSizedModel()

	// First Rotate: AvailabilityGen becomes non-zero.
	m.Session().Rotate()

	// Establish baseline flash.gen (non-zero) via a legitimate flash.
	m, _ = rootApplyMsg(m, messages.Flash{Text: "baseline flash"})
	genBefore := m.FlashGen()
	if genBefore == 0 {
		t.Fatalf("precondition: baseline flash.gen should be >0 after Flash msg, got %d", genBefore)
	}

	// Capture stale gen (non-zero) and rotate a second time.
	staleGen := m.Session().AvailabilityGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard")
	}
	m.Session().Rotate()

	// Dispatch stale APIError.
	staleErr := messages.APIError{
		ResourceType: "ec2",
		Err:          errString("stale region fetch error"),
		Gen:          staleGen,
	}
	m, _ = rootApplyMsg(m, staleErr)

	// flash.gen must be unchanged — stale error must not increment it.
	if got := m.FlashGen(); got != genBefore {
		t.Errorf("stale APIError advanced flash.gen: was %d, now %d — guard regression would show 'stale region fetch error' flash to user", genBefore, got)
	}
}

// TestIdentityError_Stale_DoesNotClearFetching verifies that a stale IdentityError
// (Gen == pre-rotate ConnectGen) does not clear Session.IdentityFetching for the
// new session. Without the guard, a slow pre-rotate identity fetch that errors
// would clear IdentityFetching=true set by the post-rotate dispatch, causing the
// header spinner to disappear while a real fetch is still in flight.
//
// AC #5 companion for IdentityError: stale errors must not mutate session state.
//
// Setup: Rotate() once to make ConnectGen non-zero before capturing staleGen,
// then Rotate() again. AcceptZeroGen=true means Gen=0 is never stale.
func TestIdentityError_Stale_DoesNotClearFetching(t *testing.T) {
	m := newRootSizedModel()

	// First Rotate: ConnectGen becomes non-zero.
	m.Session().Rotate()

	// Capture stale ConnectGen (non-zero) and rotate a second time so a new
	// identity fetch could be in flight for the post-rotate session.
	staleGen := m.Session().ConnectGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard")
	}
	m.Session().Rotate()

	// Simulate: a new identity fetch is in flight for the post-rotate session.
	m.Session().IdentityFetching = true

	// Dispatch stale IdentityError from the pre-rotate session.
	staleErr := messages.IdentityError{
		Err: "stale region identity error",
		Gen: staleGen,
	}
	m, _ = rootApplyMsg(m, staleErr)

	// IdentityFetching must still be true — the stale error must not clear it.
	if !m.Session().IdentityFetching {
		t.Errorf("stale IdentityError cleared Session.IdentityFetching for the new session — guard regression: post-rotate spinner would disappear while real fetch still in flight")
	}
}

// ── Complement: fresh APIError / IdentityError DO flash ─────────────────────

// TestAPIError_Fresh_DoesFlash verifies that a non-stale APIError (matching Gen)
// does advance flash.gen (the error IS displayed to the user).
func TestAPIError_Fresh_DoesFlash(t *testing.T) {
	m := newRootSizedModel()

	genBefore := m.FlashGen()
	currentGen := m.Session().AvailabilityGen

	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "ec2",
		Err:          errString("current region fetch error"),
		Gen:          currentGen,
	})

	if got := m.FlashGen(); got == genBefore {
		t.Errorf("fresh APIError did not advance flash.gen (was %d, still %d) — flash guard too broad", genBefore, got)
	}
}

// TestIdentityError_Fresh_DoesProcess verifies that a non-stale IdentityError
// (matching Gen) is processed (IdentityFetching cleared) without panicking.
func TestIdentityError_Fresh_DoesProcess(t *testing.T) {
	m := newRootSizedModel()

	m.Session().IdentityFetching = true
	currentGen := m.Session().ConnectGen

	m, _ = rootApplyMsg(m, messages.IdentityError{
		Err: "some identity error",
		Gen: currentGen,
	})

	// handleIdentityError clears IdentityFetching.
	if m.Session().IdentityFetching {
		t.Error("fresh IdentityError was dropped or did not clear IdentityFetching — guard too broad")
	}
}

// ── AC #6 (AS-659) — stale AvailabilityPrefetched is dropped ────────────────

// TestAvailabilityPrefetched_Stale_Dropped pins the AS-648-h4 / AS-659 contract:
// an AvailabilityPrefetched whose Gen no longer matches the live
// Session.AvailabilityGen (because Rotate() has bumped the counter past it) must
// be discarded. Without the guard, the demoPrefetchCounts dispatch path
// captures Session.AvailabilityGen at dispatch time and a slow pre-switch
// prefetch can repopulate menu counts and ResourceCache for the new
// profile/region.
//
// Reverting AvailabilityPrefetched.AcceptZeroGen() back to true OR removing
// the AvailabilityGen=1 seed in session.New() resurrects the bug: a
// zero-stamped prefetch would silently bypass the guard once AvailabilityGen
// is non-zero, contaminating the post-rotate session.
//
// Mirrors TestResourcesLoaded_Stale_Dropped — same pre-rotate / post-rotate
// staleGen capture pattern.
func TestAvailabilityPrefetched_Stale_Dropped(t *testing.T) {
	m := newRootSizedModel()

	// AvailabilityGen seeded at 1 by session.New(); rotate once so staleGen > 1.
	m.Session().Rotate()

	// Capture stale gen (non-zero) BEFORE the second Rotate.
	staleGen := m.Session().AvailabilityGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard (AcceptZeroGen=false rejects zero unconditionally)")
	}

	// Second Rotate bumps AvailabilityGen past staleGen and clears caches.
	m.Session().Rotate()
	if m.Session().AvailabilityGen == staleGen {
		t.Fatalf("precondition failed: Rotate() must advance AvailabilityGen past staleGen=%d", staleGen)
	}

	// Dispatch a stale AvailabilityPrefetched from the pre-rotate session.
	const targetType = "ec2"
	staleResource := resource.Resource{ID: "i-stale-prefetch", Name: "stale-prefetch-target"}
	staleMsg := messages.AvailabilityPrefetched{
		Entries:        map[string]int{targetType: 1},
		Truncated:      map[string]bool{},
		IssueCounts:    map[string]int{},
		IssueTruncated: map[string]bool{},
		Resources:      map[string][]resource.Resource{targetType: {staleResource}},
		Pagination:     map[string]*resource.PaginationMeta{},
		Gen:            staleGen,
	}
	m, _ = rootApplyMsg(m, staleMsg)

	// ResourceCache must NOT have been seeded — the stale prefetch was dropped.
	if entry, ok := m.Session().ResourceCache[targetType]; ok {
		t.Errorf("stale AvailabilityPrefetched was NOT dropped: ResourceCache[%q]=%+v — post-rotate session contaminated by pre-rotate prefetch (AS-648-h4 regression)", targetType, entry)
	}
}

// TestAvailabilityPrefetched_ZeroGen_Dropped pins the symmetric guard: a
// zero-stamped AvailabilityPrefetched is also stale (AcceptZeroGen=false) and
// must be dropped even if no Rotate() has happened yet. Without the
// AvailabilityGen=1 seed in session.New(), a legitimate first prefetch
// captured Gen=0 and matched the live AvailabilityGen=0, so this test could
// not even be written.
func TestAvailabilityPrefetched_ZeroGen_Dropped(t *testing.T) {
	m := newRootSizedModel()

	// Fresh session: AvailabilityGen is seeded at 1 (AS-648-h4 / AS-659).
	if got := m.Session().AvailabilityGen; got != 1 {
		t.Fatalf("precondition failed: fresh AvailabilityGen = %d, want 1 (session.New seed)", got)
	}

	const targetType = "ec2"
	zeroMsg := messages.AvailabilityPrefetched{
		Entries:   map[string]int{targetType: 1},
		Resources: map[string][]resource.Resource{targetType: {{ID: "i-zero", Name: "zero-stamped"}}},
		Gen:       0,
	}
	m, _ = rootApplyMsg(m, zeroMsg)

	if entry, ok := m.Session().ResourceCache[targetType]; ok {
		t.Errorf("zero-stamped AvailabilityPrefetched was NOT dropped: ResourceCache[%q]=%+v — guard regression: AcceptZeroGen() must remain false", targetType, entry)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// errString is a minimal error implementation for test stubs.
type errString string

func (e errString) Error() string { return string(e) }
