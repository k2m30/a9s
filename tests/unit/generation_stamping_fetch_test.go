// generation_stamping_fetch_test.go — session-stamp guards on the
// GenStamped messages in core/runtime/messages/event.go. Each carries a Gen
// domain.Gen field with GenStamp() / GenAspect() / AcceptZeroGen() methods; a
// message stamped before Session.Rotate() is dropped.
package unit

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// ── stale ResourcesLoaded is dropped ────────────────────────────────

// AcceptZeroGen=true means a Gen=0 stamp is never stale, so the stale stamp
// must be non-zero.
func TestResourcesLoaded_Stale_Dropped(t *testing.T) {
	m := newRootSizedModel()

	m.Core().Session().Rotate()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	sentinel := []resource.Resource{
		{ID: "i-before-rotate", Name: "before-rotate-server"},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    sentinel,
		Gen:          m.Core().Session().AvailabilityGen,
	})
	if got := m.ActiveListResources(); len(got) == 0 || got[0].ID != "i-before-rotate" {
		t.Fatalf("baseline: expected sentinel resource i-before-rotate in list, got %v", got)
	}

	staleGen := m.Core().Session().AvailabilityGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard (AcceptZeroGen=true would pass zero)")
	}
	m.Core().Session().Rotate()

	staleMsg := messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-stale-account", Name: "stale-server"}},
		Gen:          staleGen,
	}
	m, _ = rootApplyMsg(m, staleMsg)

	got := m.ActiveListResources()
	for _, r := range got {
		if r.ID == "i-stale-account" {
			t.Errorf("stale ResourcesLoaded was NOT dropped: found i-stale-account in list after Rotate() — guard regression")
		}
	}
}

// ── stale IdentityLoaded is dropped ─────────────────────────────────

// A stale IdentityLoaded would show the previous account's ID in the header
// after a profile switch.
func TestIdentityLoaded_Stale_Dropped(t *testing.T) {
	m := newRootSizedModel()

	m.Core().Session().Rotate()

	staleGen := m.Core().Session().ConnectGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard")
	}
	m.Core().Session().Rotate()

	if m.Core().Session().Identity != nil {
		t.Fatal("precondition failed: Session.Identity should be nil after Rotate()")
	}

	staleIdentity := &awsclient.CallerIdentity{AccountID: "111122223333"}
	staleMsg := messages.IdentityLoaded{
		Identity: staleIdentity,
		Gen:      staleGen,
	}
	m, _ = rootApplyMsg(m, staleMsg)

	if m.Core().Session().Identity != nil {
		t.Errorf("stale IdentityLoaded was NOT dropped: Session.Identity = %+v, expected nil — guard regression", m.Core().Session().Identity)
	}
}

// ── stale ValueRevealed is dropped ──────────────────────────────────

// A stale ValueRevealed would render the previous profile's secret.
func TestValueRevealed_Stale_Dropped(t *testing.T) {
	m := newRootSizedModel()

	m.Core().Session().Rotate()

	staleGen := m.Core().Session().ConnectGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard")
	}
	m.Core().Session().Rotate()

	const staleSecret = "PREV_ACCOUNT_SECRET_VALUE_xyzzy_42"

	staleMsg := messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "prod/api/key",
		Value:        staleSecret,
		Gen:          staleGen,
	}
	m, _ = rootApplyMsg(m, staleMsg)

	rendered := rootViewContent(m)
	plain := stripANSI(rendered)
	if strings.Contains(plain, staleSecret) {
		t.Errorf("stale ValueRevealed was NOT dropped: secret value %q appears in rendered output — guard regression (secret leak)", staleSecret)
	}
}

// ── matching gen (happy path) applies all three message types ────────

func TestHappyPath_MatchingGen_ResourcesLoaded(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	currentGen := m.Core().Session().AvailabilityGen
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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

func TestHappyPath_MatchingGen_IdentityLoaded(t *testing.T) {
	m := newRootSizedModel()
	currentGen := m.Core().Session().ConnectGen
	freshIdentity := &awsclient.CallerIdentity{AccountID: "999988887777"}
	m, _ = rootApplyMsg(m, messages.IdentityLoaded{
		Identity: freshIdentity,
		Gen:      currentGen,
	})
	if m.Core().Session().Identity == nil {
		t.Fatal("happy-path IdentityLoaded with matching gen was not applied: Session.Identity is nil")
	}
	if m.Core().Session().Identity.AccountID != "999988887777" {
		t.Errorf("Session.Identity.AccountID = %q, want 999988887777", m.Core().Session().Identity.AccountID)
	}
}

func TestHappyPath_MatchingGen_ValueRevealed(t *testing.T) {
	m := newRootSizedModel()
	currentGen := m.Core().Session().ConnectGen
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

// TestHappyPath_ZeroGen_AcceptedByAllThree pins AcceptZeroGen=true for
// ResourcesLoaded, IdentityLoaded, and ValueRevealed: a Gen=0 stamp is
// accepted regardless of the live counter, because demo, --no-cache and test
// callers dispatch these messages without a Gen field set. Each subtest forces
// the live counter non-zero first, so only the "stamp == 0 &&
// AcceptZeroGen()" branch of IsStale can admit the message.
func TestHappyPath_ZeroGen_AcceptedByAllThree(t *testing.T) {
	t.Run("ResourcesLoaded", func(t *testing.T) {
		m := newRootSizedModel()
		m.Core().Session().Rotate()
		if got := m.Core().Session().AvailabilityGen; got == 0 {
			t.Fatalf("precondition: AvailabilityGen must be non-zero, got 0")
		}

		m, _ = rootApplyMsg(m, messages.Navigate{
			Target:       messages.TargetResourceList,
			ResourceType: "ec2",
		})
		m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
			t.Error("ResourcesLoaded with Gen=0 was dropped while AvailabilityGen!=0 — AcceptZeroGen=true regression")
		}
	})

	t.Run("IdentityLoaded", func(t *testing.T) {
		m := newRootSizedModel()
		m.Core().Session().Rotate()
		if got := m.Core().Session().ConnectGen; got == 0 {
			t.Fatalf("precondition: ConnectGen must be non-zero, got 0")
		}

		freshID := &awsclient.CallerIdentity{AccountID: "zero-gen-account"}
		m, _ = rootApplyMsg(m, messages.IdentityLoaded{
			Identity: freshID,
			Gen:      domain.Gen(0),
		})
		if m.Core().Session().Identity == nil {
			t.Error("IdentityLoaded with Gen=0 was dropped while ConnectGen!=0 — AcceptZeroGen=true regression")
		}
	})

	t.Run("ValueRevealed", func(t *testing.T) {
		m := newRootSizedModel()
		// ValueRevealed uses AspectConnect.
		m.Core().Session().Rotate()
		if got := m.Core().Session().ConnectGen; got == 0 {
			t.Fatalf("precondition: ConnectGen must be non-zero, got 0")
		}

		const zeroGenSecret = "ZERO_GEN_SECRET_demo_mode_xyz"
		m, _ = rootApplyMsg(m, messages.ValueRevealed{
			ResourceType: "secrets",
			ResourceID:   "demo/key",
			Value:        zeroGenSecret,
			Gen:          domain.Gen(0),
		})
		plain := stripANSI(rootViewContent(m))
		if !strings.Contains(plain, zeroGenSecret) {
			t.Errorf("ValueRevealed with Gen=0 was dropped while ConnectGen!=0 — AcceptZeroGen=true regression; rendered: %q", plain)
		}
	})
}

// ── stale APIError / IdentityError do not flash ─────────────────────

// A stale APIError would flash a slow "fetch ec2: ..." error from a region
// the user already left.
func TestAPIError_Stale_NoFlash(t *testing.T) {
	m := newRootSizedModel()

	m.Core().Session().Rotate()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "baseline flash"})
	genBefore := m.FlashGen()
	if genBefore == 0 {
		t.Fatalf("precondition: baseline flash.gen should be >0 after Flash msg, got %d", genBefore)
	}

	staleGen := m.Core().Session().AvailabilityGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard")
	}
	m.Core().Session().Rotate()

	staleErr := messages.APIError{
		ResourceType: "ec2",
		Err:          errString("stale region fetch error"),
		Gen:          staleGen,
	}
	m, _ = rootApplyMsg(m, staleErr)

	if got := m.FlashGen(); got != genBefore {
		t.Errorf("stale APIError advanced flash.gen: was %d, now %d — guard regression would show 'stale region fetch error' flash to user", genBefore, got)
	}
}

// A stale IdentityError would clear the IdentityFetching set by the
// post-rotate dispatch, hiding the header spinner while a real fetch is still
// in flight.
func TestIdentityError_Stale_DoesNotClearFetching(t *testing.T) {
	m := newRootSizedModel()

	m.Core().Session().Rotate()

	staleGen := m.Core().Session().ConnectGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard")
	}
	m.Core().Session().Rotate()

	m.Core().Session().IdentityFetching = true

	staleErr := messages.IdentityError{
		Err: "stale region identity error",
		Gen: staleGen,
	}
	m, _ = rootApplyMsg(m, staleErr)

	if !m.Core().Session().IdentityFetching {
		t.Errorf("stale IdentityError cleared Session.IdentityFetching for the new session — guard regression: post-rotate spinner would disappear while real fetch still in flight")
	}
}

// ── Complement: fresh APIError / IdentityError DO flash ─────────────────────

func TestAPIError_Fresh_DoesFlash(t *testing.T) {
	m := newRootSizedModel()

	genBefore := m.FlashGen()
	currentGen := m.Core().Session().AvailabilityGen

	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "ec2",
		Err:          errString("current region fetch error"),
		Gen:          currentGen,
	})

	if got := m.FlashGen(); got == genBefore {
		t.Errorf("fresh APIError did not advance flash.gen (was %d, still %d) — flash guard too broad", genBefore, got)
	}
}

func TestIdentityError_Fresh_DoesProcess(t *testing.T) {
	m := newRootSizedModel()

	m.Core().Session().IdentityFetching = true
	currentGen := m.Core().Session().ConnectGen

	m, _ = rootApplyMsg(m, messages.IdentityError{
		Err: "some identity error",
		Gen: currentGen,
	})

	if m.Core().Session().IdentityFetching {
		t.Error("fresh IdentityError was dropped or did not clear IdentityFetching — guard too broad")
	}
}

// ── stale AvailabilityPrefetched is dropped ────────────────

// The demoPrefetchCounts dispatch path captures Session.AvailabilityGen at
// dispatch time, so a slow pre-switch prefetch would repopulate menu counts
// and ResourceCache for the new profile/region. AvailabilityPrefetched rejects
// a zero stamp (AcceptZeroGen() is false) and session.New() seeds
// AvailabilityGen at 1, so a zero-stamped prefetch cannot pass the guard
// either.
func TestAvailabilityPrefetched_Stale_Dropped(t *testing.T) {
	m := newRootSizedModel()

	m.Core().Session().Rotate()

	staleGen := m.Core().Session().AvailabilityGen
	if staleGen == 0 {
		t.Fatal("precondition failed: staleGen must be non-zero to test IsStale guard (AcceptZeroGen=false rejects zero unconditionally)")
	}

	m.Core().Session().Rotate()
	if m.Core().Session().AvailabilityGen == staleGen {
		t.Fatalf("precondition failed: Rotate() must advance AvailabilityGen past staleGen=%d", staleGen)
	}

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

	if tr := m.Core().Session().RowStore.Snapshot(targetType); tr.Gen != 0 {
		t.Errorf("stale AvailabilityPrefetched was NOT dropped: RowStore.Snapshot(%q)=%+v — post-rotate session contaminated by pre-rotate prefetch (AS-648-h4 regression)", targetType, tr)
	}
}

// A zero-stamped AvailabilityPrefetched is stale (AcceptZeroGen=false) even
// before any Rotate(); the AvailabilityGen=1 seed in session.New() keeps a
// legitimate first prefetch from carrying Gen=0.
func TestAvailabilityPrefetched_ZeroGen_Dropped(t *testing.T) {
	m := newRootSizedModel()

	if got := m.Core().Session().AvailabilityGen; got != 1 {
		t.Fatalf("precondition failed: fresh AvailabilityGen = %d, want 1 (session.New seed)", got)
	}

	const targetType = "ec2"
	zeroMsg := messages.AvailabilityPrefetched{
		Entries:   map[string]int{targetType: 1},
		Resources: map[string][]resource.Resource{targetType: {{ID: "i-zero", Name: "zero-stamped"}}},
		Gen:       0,
	}
	m, _ = rootApplyMsg(m, zeroMsg)

	if tr := m.Core().Session().RowStore.Snapshot(targetType); tr.Gen != 0 {
		t.Errorf("zero-stamped AvailabilityPrefetched was NOT dropped: RowStore.Snapshot(%q)=%+v — guard regression: AcceptZeroGen() must remain false", targetType, tr)
	}
}

// ── ConnectGen seed: a pre-switch stamp never folds after Rotate ──────────

// A fresh session's ConnectGen is seeded at 1, so a task dispatched before any
// profile/region switch captures stamp 1 and loses a plain inequality against
// the post-Rotate value 2 regardless of AcceptZeroGen. A stamp of 0 would pass
// AcceptZeroGen=true on every AspectConnect event and install the previous
// account's identity, a decrypted secret, or cost data into the new session.
// Each subtest drives the real dispatch path (rootApplyMsg /
// Controller.Handle) rather than calling messages.IsStale.
func TestConnectGenLeak_PreSwitchStamp_DoesNotFoldAfterRotate(t *testing.T) {
	t.Run("Identity", func(t *testing.T) {
		m := newRootSizedModel()
		preSwitchGen := m.Core().Session().ConnectGen
		if preSwitchGen != 1 {
			t.Fatalf("precondition: fresh session's ConnectGen = %d, want 1 (the seed)", preSwitchGen)
		}

		m.Core().Session().Rotate() // simulates the profile/region switch

		staleIdentity := &awsclient.CallerIdentity{AccountID: "111122223333"}
		m, _ = rootApplyMsg(m, messages.IdentityLoaded{
			Identity: staleIdentity,
			Gen:      preSwitchGen,
		})

		if m.Core().Session().Identity != nil {
			t.Errorf("pre-switch-stamped IdentityLoaded (Gen=%d) folded after Rotate() bumped ConnectGen to %d — "+
				"cross-account identity leak regression: Session.Identity = %+v, want nil",
				preSwitchGen, m.Core().Session().ConnectGen, m.Core().Session().Identity)
		}
	})

	t.Run("ValueRevealed", func(t *testing.T) {
		m := newRootSizedModel()
		preSwitchGen := m.Core().Session().ConnectGen
		if preSwitchGen != 1 {
			t.Fatalf("precondition: fresh session's ConnectGen = %d, want 1 (the seed)", preSwitchGen)
		}

		m.Core().Session().Rotate()

		const staleSecret = "PRE_SWITCH_SECRET_leak_check_xyz"
		m, _ = rootApplyMsg(m, messages.ValueRevealed{
			ResourceType: "secrets",
			ResourceID:   "prod/api/key",
			Value:        staleSecret,
			Gen:          preSwitchGen,
		})

		plain := stripANSI(rootViewContent(m))
		if strings.Contains(plain, staleSecret) {
			t.Errorf("pre-switch-stamped ValueRevealed (Gen=%d) folded after Rotate() bumped ConnectGen to %d — "+
				"secret leak regression: value %q appears in rendered output", preSwitchGen, m.Core().Session().ConnectGen, staleSecret)
		}
	})

	t.Run("Costs", func(t *testing.T) {
		now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
		s, c := leakPinCostsController(t, now)

		preSwitchGen := s.ConnectGen
		if preSwitchGen != 1 {
			t.Fatalf("precondition: fresh session's ConnectGen = %d, want 1 (the seed)", preSwitchGen)
		}

		q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

		// Seed a pre-switch error, accepted because it is dispatched at the
		// live (pre-rotate) gen.
		c.Handle(messages.CostsLoaded{
			Query: q,
			Err:   errors.New("cost explorer: no client configured for this session"),
			Gen:   preSwitchGen,
		})
		if got := c.Snapshot().Body.Costs.ErrorMsg; got == "" {
			t.Fatal("precondition: the pre-switch costs fetch failure must set ErrorMsg")
		}

		s.Rotate() // simulates the profile/region switch

		// The pre-switch task's late-arriving "recovery" carries preSwitchGen,
		// which differs from the post-Rotate ConnectGen — it must be dropped, not
		// folded into the new session's costs state.
		c.Handle(messages.CostsLoaded{
			Query: q,
			Grid: costs.GridResult{Fetched: true, Records: []costs.Record{{
				Period:  costs.Period{Start: "2026-07-01", End: "2026-08-01"},
				Keys:    []string{"Amazon EC2"},
				Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 999.0, Unit: "USD"}},
			}}},
			Gen: preSwitchGen,
		})

		if got := c.Snapshot().Body.Costs.ErrorMsg; got == "" {
			t.Errorf("pre-switch-stamped CostsLoaded (Gen=%d) folded after Rotate() bumped ConnectGen to %d — "+
				"the pre-switch error was cleared as if the stale delivery were fresh (cost-data cross-account leak regression)",
				preSwitchGen, s.ConnectGen)
		}
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

// leakPinCostsController builds a *session.Session + *app.Controller with
// ScreenCosts pushed and EnsureCostsState seeded. It is listed in
// qa_controller_construction_discipline_test.go's ccdBlessedHelpers: it pairs
// t.TempDir() + t.Cleanup(c.Close) in the correct LIFO order, and returns the
// *session.Session so the caller can read and Rotate() ConnectGen.
func leakPinCostsController(t *testing.T, now time.Time) (*session.Session, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "leak-pin-costs"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(now)
	return s, c
}

type errString string

func (e errString) Error() string { return string(e) }
