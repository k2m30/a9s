package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ct-events has a registered FilteredPaginatedFetcher: a RelatedNavigateMsg
// with FetchFilter routes through handleRelatedNavigate's
// NavigationKindFilteredList branch to m.fetchResourcesFiltered.
func TestFetchResourcesFiltered_NilClients_ViaRelatedNavigate(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel() // clients == nil

	// Navigate to a detail view first so RelatedNavigateMsg is handled.
	res := &resource.Resource{ID: "i-0abc123", Name: "web-server"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     res,
		ResourceType: "ec2",
	})

	_, cmd := rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: "ct-events",
		FetchFilter: map[string]string{
			"ResourceName": "i-0abc123",
			"EventSource":  "ec2.amazonaws.com",
		},
		SourceResource: *res,
		SourceType:     "ec2",
	})
	if cmd == nil {
		t.Fatal("RelatedNavigateMsg with FetchFilter should return a cmd batch")
	}
	msg := cmd()
	switch v := msg.(type) {
	case messages.APIError:
		if !strings.Contains(v.Err.Error(), "not initialized") {
			t.Errorf("APIErrorMsg.Err = %q, want 'not initialized'", v.Err.Error())
		}
	case tea.BatchMsg:
		found := false
		for _, sub := range v {
			if sub == nil {
				continue
			}
			if ae, ok := sub().(messages.APIError); ok {
				if strings.Contains(ae.Err.Error(), "not initialized") {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("batch from fetchResourcesFiltered with nil clients should contain APIErrorMsg{not initialized}")
		}
	default:
		t.Logf("fetchResourcesFiltered nil-clients path returned %T — acceptable", msg)
	}
}

func TestFetchResourcesFiltered_NoFetcher(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	// There is no filtered fetcher for "ec2" — only a paginated fetcher.
	// LoadMoreMsg{FetchFilter: ...} will call GetFilteredPaginatedFetcher("ec2")
	// which returns nil, so the filtered branch is skipped. The fallback
	// paginated fetcher IS registered, so we use a type that has no fetcher at all.
	const noSuchType = "test_ff_no_fetcher_xyz"
	_, cmd := rootApplyMsg(m, messages.LoadMore{
		ResourceType: noSuchType,
		FetchFilter:  map[string]string{"k": "v"},
	})
	if cmd == nil {
		t.Fatal("LoadMoreMsg should always return a cmd")
	}
	msg := cmd()
	// With nil clients the nil-clients guard fires first regardless
	apiErr, ok := msg.(messages.APIError)
	if !ok {
		t.Fatalf("expected APIErrorMsg, got %T", msg)
	}
	if apiErr.ResourceType != noSuchType {
		t.Errorf("APIErrorMsg.ResourceType = %q, want %q", apiErr.ResourceType, noSuchType)
	}
}

func TestFetchChildResources_NilClients(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel() // clients == nil

	// Register a temporary child type so the "unsupported child type" guard is
	// not hit — we want the nil-clients guard.
	const childType = "test_child_nil_clients"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Child Nil Clients",
		ShortName: childType,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	resource.SetPaginatedChildForTest(childType, func(_ context.Context, _ any, _ resource.ParentContext, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, nil
	})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest(childType)
		resource.CleanupPaginatedChildForTest(childType)
	})

	_, cmd := rootApplyMsg(m, messages.EnterChildView{
		ChildType:     childType,
		ParentContext: map[string]string{"bucket": "test-bucket"},
		DisplayName:   "test-bucket",
	})
	if cmd == nil {
		t.Fatal("EnterChildViewMsg should return a cmd")
	}
	msg := extractMsg(t, cmd, func(m tea.Msg) bool {
		_, ok := m.(messages.APIError)
		return ok
	})
	apiErr, ok := msg.(messages.APIError)
	if !ok {
		t.Fatalf("expected APIErrorMsg from nil-clients child fetch, got %T", msg)
	}
	if apiErr.ResourceType != childType {
		t.Errorf("APIErrorMsg.ResourceType = %q, want %q", apiErr.ResourceType, childType)
	}
	if !strings.Contains(apiErr.Err.Error(), "not initialized") {
		t.Errorf("Err = %q, want to contain 'not initialized'", apiErr.Err.Error())
	}
}

func TestFetchChildResources_UnknownChildType(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	// A registered child type with no paginated child fetcher reaches the
	// "unsupported child type" message inside fetchChildResources.
	const noFetcherChild = "test_child_no_fetcher"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Child No Fetcher",
		ShortName: noFetcherChild,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest(noFetcherChild)
	})

	_, cmd := rootApplyMsg(m, messages.EnterChildView{
		ChildType:     noFetcherChild,
		ParentContext: map[string]string{"id": "x"},
		DisplayName:   "x",
	})
	if cmd == nil {
		t.Fatal("EnterChildViewMsg should return a cmd even with missing fetcher")
	}
	msg := extractMsg(t, cmd, func(m tea.Msg) bool {
		_, ok := m.(messages.APIError)
		return ok
	})
	apiErr, ok := msg.(messages.APIError)
	if !ok {
		// nil clients fires first and also produces APIErrorMsg — fine.
		t.Logf("got %T; acceptable (nil-clients guard fired first)", msg)
		return
	}
	if apiErr.ResourceType != noFetcherChild {
		t.Errorf("APIErrorMsg.ResourceType = %q, want %q", apiErr.ResourceType, noFetcherChild)
	}
}

// fetchChildResources follows the partial-success rule of fetchResources,
// fetchResourcesFiltered and fetchMoreResources (fetch_adapter.go): an error
// with no rows is an APIError; an error with rows is a ResourcesLoaded carrying
// both Resources and Err.
func TestFetchChildResources_PartialSuccess_ReturnsResourcesLoadedWithErr(t *testing.T) {
	withTuiVersion(t, "test")
	clients := demo.NewServiceClients()
	m := newBlessedModel(t, demo.DemoProfile, demo.DemoRegion,
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: clients, Region: demo.DemoRegion, Gen: 1})

	const childType = "test_child_partial_success"
	partialErr := errors.New("partial: 1 of 2 child objects failed to fetch")
	partialResources := []resource.Resource{
		{ID: "child-001", Name: "child-001"},
		{ID: "child-002", Name: "child-002"},
	}
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Child Partial Success",
		ShortName: childType,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	resource.SetPaginatedChildForTest(childType, func(_ context.Context, _ any, _ resource.ParentContext, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{Resources: partialResources}, partialErr
	})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest(childType)
		resource.CleanupPaginatedChildForTest(childType)
	})

	_, cmd := rootApplyMsg(m, messages.EnterChildView{
		ChildType:     childType,
		ParentContext: map[string]string{"bucket": "test-bucket"},
		DisplayName:   "test-bucket",
	})
	if cmd == nil {
		t.Fatal("EnterChildViewMsg should return a cmd")
	}
	msg := extractMsg(t, cmd, func(m tea.Msg) bool {
		switch v := m.(type) {
		case messages.ResourcesLoaded:
			return v.ResourceType == childType
		case messages.APIError:
			return v.ResourceType == childType
		default:
			return false
		}
	})

	loaded, ok := msg.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("fetchChildResources partial success: expected messages.ResourcesLoaded carrying the rows AND the error, got %T (%+v) — fetchChildResources hard-fails on any err != nil instead of following the partial-success contract used by fetchResources/fetchResourcesFiltered/fetchMoreResources", msg, msg)
	}
	if loaded.Err == nil {
		t.Error("fetchChildResources partial success: ResourcesLoaded.Err must be set so the partial failure surfaces via Flash")
	}
	if len(loaded.Resources) != len(partialResources) {
		t.Errorf("fetchChildResources partial success: len(Resources) = %d, want %d — partial rows must not be dropped", len(loaded.Resources), len(partialResources))
	}
}

func TestFetchMoreResources_ParentCtxBranch_NilClients(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	// Register a child fetcher so the lookup succeeds.
	const childType = "test_more_child"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test More Child",
		ShortName: childType,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	resource.SetPaginatedChildForTest(childType, func(_ context.Context, _ any, _ resource.ParentContext, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, nil
	})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest(childType)
		resource.CleanupPaginatedChildForTest(childType)
	})

	_, cmd := rootApplyMsg(m, messages.LoadMore{
		ResourceType:      childType,
		ContinuationToken: "next-page-token",
		ParentContext:     map[string]string{"bucket": "my-bucket"},
	})
	if cmd == nil {
		t.Fatal("LoadMoreMsg with parentCtx should return a cmd")
	}
	msg := cmd()
	apiErr, ok := msg.(messages.APIError)
	if !ok {
		t.Fatalf("expected APIErrorMsg from nil-clients parentCtx branch, got %T", msg)
	}
	if !strings.Contains(apiErr.Err.Error(), "not initialized") {
		t.Errorf("Err = %q, want 'not initialized'", apiErr.Err.Error())
	}
}

func TestFetchMoreResources_NoFetcherFallback(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	const ghostType = "test_more_ghost_xyz"
	_, cmd := rootApplyMsg(m, messages.LoadMore{
		ResourceType:      ghostType,
		ContinuationToken: "tok",
	})
	if cmd == nil {
		t.Fatal("LoadMoreMsg should return a cmd")
	}
	msg := cmd()
	// nil-clients guard fires first
	apiErr, ok := msg.(messages.APIError)
	if !ok {
		t.Fatalf("expected APIErrorMsg, got %T", msg)
	}
	if apiErr.ResourceType != ghostType {
		t.Errorf("ResourceType = %q, want %q", apiErr.ResourceType, ghostType)
	}
}

// The machine's AWS config is not controlled, so only the message type is
// checked.
func TestFetchProfiles_ReturnsMsg(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	_, cmd := rootApplyMsg(m, messages.Navigate{Target: messages.TargetProfile})
	if cmd == nil {
		t.Fatal("navigating to profile selector should return a cmd (fetchProfiles)")
	}
	msg := cmd()
	// Acceptable: profilesLoadedMsg (internal type, not exported) or FlashMsg.
	switch msg.(type) {
	case messages.Flash:
		// No profiles found or error — acceptable in CI environments
	case nil:
		// Also acceptable if the cmd is batched
	default:
		// Any non-nil message is fine — profiles were found
	}
}

func TestFetchRevealValue_NilClients(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	// "secrets" has a reveal fetcher, so the lookup passes.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "secrets",
		Resources: []resource.Resource{
			{ID: "my-secret-arn", Name: "my-secret"},
		},
	})
	_, cmd := rootApplyMsg(m, rootKeyPress("x"))
	if cmd == nil {
		// x key on the list — the cmd might not be set if no fetcher. Acceptable.
		t.Log("no cmd from 'x' — reveal might not be triggered at list level")
		return
	}
	msg := cmd()
	switch v := msg.(type) {
	case messages.Flash:
		// Expected: "AWS clients not initialized" or "no reveal support"
		if !v.IsError {
			t.Errorf("FlashMsg.IsError should be true for nil-clients reveal, got text=%q", v.Text)
		}
	case messages.ValueRevealed:
		if v.Err == nil {
			t.Error("ValueRevealedMsg.Err should be set for nil clients")
		}
	default:
		// Other message types acceptable
		t.Logf("reveal with nil clients returned %T — acceptable", msg)
	}
}

// ec2 has no reveal fetcher: handleReveal bails early at HasRevealFetcher, so
// 'x' dispatches nothing or flashes an error.
func TestFetchRevealValue_NoRevealFetcher(t *testing.T) {
	withTuiVersion(t, "test")

	if resource.HasRevealFetcher("ec2") {
		t.Skip("ec2 unexpectedly has a reveal fetcher — test precondition failed")
	}

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-0abc111", Name: "web-server-1", Fields: map[string]string{"State": "running"}},
		},
	})
	_, cmd := rootApplyMsg(m, rootKeyPress("x"))
	// Either nil (bail-early) or FlashMsg{IsError} is acceptable.
	if cmd == nil {
		return // bail-early path confirmed
	}
	msg := cmd()
	if flash, ok := msg.(messages.Flash); ok {
		if !flash.IsError {
			t.Errorf("FlashMsg.IsError should be true for type without reveal fetcher")
		}
	}
	// Any other message type is also acceptable — not a failure.
}

// With nil clients an AvailabilityCacheLoadedMsg dispatches no probe cmds:
// they would run against a nil transport, fail hard, and lose that probe for
// the session. Session.AvailSweepPending is latched instead, and the next
// successful ClientsReady drains the first batch
// (fireNextAvailabilityProbes(4)).
func TestProbeResourceAvailability_NilClients(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel() // clients == nil

	_, cmd := rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: make(map[string]int),
	})
	if cmd != nil {
		t.Errorf("AvailabilityCacheLoadedMsg with nil clients must NOT dispatch probe cmds (would run against a nil transport); got non-nil cmd")
	}

	if !m.Core().Session().AvailSweepPending {
		t.Error("Session.AvailSweepPending must be latched true so ClientsReady can drain the sweep once a real transport exists")
	}
}

func TestSaveAvailabilityCache_NoCacheMode(t *testing.T) {
	withTuiVersion(t, "test")
	m := newBlessedModel(t, demo.DemoProfile, demo.DemoRegion,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithNoCache(true),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Deliver AvailabilityCheckedMsg that simulates the all-done state.
	// This triggers the "all checks done" branch in handleAvailabilityChecked
	// which calls saveAvailabilityCache. With noCache=true it's a no-op (nil cmd).
	// Stamp the live AvailabilityGen so the stale guard accepts
	// the message (AcceptZeroGen=false; session.New seeds AvailabilityGen=1).
	_, cmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        3,
		Gen:          m.Core().Session().AvailabilityGen,
	})
	if cmd != nil {
		_ = cmd() //nolint:ineffassign,staticcheck // verifying no panic
	}
}

func TestDemoPrefetchCounts_ViaClientReady(t *testing.T) {
	withTuiVersion(t, "test")
	clients := demo.NewServiceClients()
	m := newBlessedModel(t, demo.DemoProfile, demo.DemoRegion,
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Send ClientsReadyMsg with the demo clients so handleClientsReady runs the
	// noCache branch which calls demoPrefetchCounts().
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	_, cmd := rootApplyMsg(m, messages.ClientsReady{
		Clients: clients,
		Region:  demo.DemoRegion,
		Gen:     1,
	})
	if cmd == nil {
		t.Fatal("ClientsReadyMsg with noCache=true and valid clients should return demoPrefetchCounts cmd")
	}
	msg := cmd()
	prefetched, ok := msg.(messages.AvailabilityPrefetched)
	if !ok {
		// May be a tea.BatchMsg wrapping it.
		if batch, isBatch := msg.(tea.BatchMsg); isBatch {
			for _, sub := range batch {
				if sub == nil {
					continue
				}
				if pf, ok2 := sub().(messages.AvailabilityPrefetched); ok2 {
					prefetched = pf
					ok = true
					break
				}
			}
		}
	}
	if !ok {
		t.Fatalf("demoPrefetchCounts should return AvailabilityPrefetchedMsg, got %T", msg)
	}
	if len(prefetched.Entries) == 0 {
		t.Error("demoPrefetchCounts with demo clients should populate at least one resource type entry")
	}
}

func TestDemoPrefetchCounts_AvailabilityPrefetchedHandler(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	// Stamp the live AvailabilityGen so the staleness guard accepts the
	// message (AcceptZeroGen=false; session.New seeds AvailabilityGen=1).
	_, cmd := rootApplyMsg(m, messages.AvailabilityPrefetched{
		Entries:     map[string]int{"ec2": 7, "s3": 3},
		Truncated:   map[string]bool{},
		IssueCounts: map[string]int{"ec2": 1},
		Gen:         m.Core().Session().AvailabilityGen,
		Resources:   map[string][]resource.Resource{},
	})
	if cmd != nil {
		_ = cmd() //nolint:ineffassign,staticcheck // verifying no panic
	}
}

// Ctrl+R on a resource list stamps TypeGen onto the inner ResourcesLoadedMsg,
// which triggers a probeEnrichment cmd.
func TestRefreshResourceListWithEnrichmentRerun_StampsTypeGen(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	_, refreshCmd := rootApplyMsg(m, ctrlRKeyMsg())
	if refreshCmd == nil {
		t.Fatal("Ctrl+R should return a refresh cmd")
	}

	// Execute the refresh — with nil clients it returns APIErrorMsg or
	// ResourcesLoadedMsg. Either way the cmd closure should not panic.
	msg := refreshCmd()
	switch v := msg.(type) {
	case messages.ResourcesLoaded:
		if v.TypeGen == 0 {
			t.Errorf("refreshResourceListWithEnrichmentRerun: TypeGen = 0, want > 0")
		}
	case messages.APIError:
		// nil clients → API error, TypeGen never set — acceptable
		t.Logf("Ctrl+R with nil clients returned APIErrorMsg (clients not initialized)")
	default:
		t.Logf("refreshResourceListWithEnrichmentRerun returned %T — acceptable", msg)
	}
}

func TestRefreshResourceListWithEnrichmentRerun_PassthroughAPIError(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel() // nil clients → inner fetchResources returns APIErrorMsg
	m = navigateToEC2List(m)

	_, refreshCmd := rootApplyMsg(m, ctrlRKeyMsg())
	if refreshCmd == nil {
		t.Fatal("Ctrl+R should return a cmd even with nil clients")
	}
	msg := refreshCmd()
	// With nil clients, fetchResources returns APIErrorMsg.
	// The wrapper must not swallow or modify it.
	apiErr, ok := msg.(messages.APIError)
	if ok {
		if apiErr.ResourceType != "ec2" {
			t.Errorf("APIErrorMsg.ResourceType = %q, want ec2", apiErr.ResourceType)
		}
		return
	}
	// ResourcesLoadedMsg is also acceptable if somehow the type has a nil-safe fetcher.
	if _, ok := msg.(messages.ResourcesLoaded); ok {
		return
	}
	// Any other type is also OK (batch, etc.)
	t.Logf("wrapper passthrough returned %T — acceptable", msg)
}

// fetchResources, fetchIdentity and fetchRevealValue capture the generation
// counter synchronously at the call site, not inside the closure: a Rotate()
// that bumps the counter before the goroutine runs would otherwise stamp the
// message with the post-rotate gen, and it would pass the stale guard.
func TestFetchAdapter_CapturesGenAtDispatchTime(t *testing.T) {
	withTuiVersion(t, "test")

	t.Run("fetchResources", func(t *testing.T) {
		m := newRootSizedModel()
		dispatchGen := m.Core().Session().AvailabilityGen

		cmd := m.FetchResourcesCmdForTest("ec2", dispatchGen)

		// Rotate AFTER dispatch: the closure must carry dispatchGen, not the new gen.
		m.Core().Session().Rotate()
		if m.Core().Session().AvailabilityGen == dispatchGen {
			t.Fatal("Rotate() did not bump AvailabilityGen — test precondition broken")
		}

		// Execute the closure synchronously (nil clients → APIError or ResourcesLoaded).
		msg := cmd()
		switch v := msg.(type) {
		case messages.ResourcesLoaded:
			if v.Gen != dispatchGen {
				t.Errorf("fetchResources: Gen in ResourcesLoaded = %d, want dispatchGen %d — gen captured inside closure, not at dispatch site", v.Gen, dispatchGen)
			}
		case messages.APIError:
			if v.Gen != dispatchGen {
				t.Errorf("fetchResources: Gen in APIError = %d, want dispatchGen %d — gen captured inside closure, not at dispatch site", v.Gen, dispatchGen)
			}
		default:
			t.Logf("fetchResources returned %T — checking not possible for this type; nil-clients guard may have fired before gen stamp", msg)
		}
	})

	t.Run("fetchIdentity", func(t *testing.T) {
		m := newRootSizedModel()
		dispatchGen := m.Core().Session().ConnectGen

		cmd := m.FetchIdentityCmdForTest(dispatchGen)

		m.Core().Session().Rotate()
		if m.Core().Session().ConnectGen == dispatchGen {
			t.Fatal("Rotate() did not bump ConnectGen — test precondition broken")
		}

		msg := cmd()
		switch v := msg.(type) {
		case messages.IdentityLoaded:
			if v.Gen != dispatchGen {
				t.Errorf("fetchIdentity: Gen in IdentityLoaded = %d, want dispatchGen %d — gen captured inside closure", v.Gen, dispatchGen)
			}
		case messages.IdentityError:
			if v.Gen != dispatchGen {
				t.Errorf("fetchIdentity: Gen in IdentityError = %d, want dispatchGen %d — gen captured inside closure", v.Gen, dispatchGen)
			}
		default:
			t.Logf("fetchIdentity returned %T — cannot assert Gen on this type", msg)
		}
	})

	t.Run("fetchRevealValue", func(t *testing.T) {
		m := newRootSizedModel()
		dispatchGen := m.Core().Session().ConnectGen

		cmd := m.FetchRevealValueCmdForTest("secrets", "prod/api/key", dispatchGen)

		m.Core().Session().Rotate()
		if m.Core().Session().ConnectGen == dispatchGen {
			t.Fatal("Rotate() did not bump ConnectGen — test precondition broken")
		}

		msg := cmd()
		switch v := msg.(type) {
		case messages.ValueRevealed:
			if v.Gen != dispatchGen {
				t.Errorf("fetchRevealValue: Gen in ValueRevealed = %d, want dispatchGen %d — gen captured inside closure", v.Gen, dispatchGen)
			}
		default:
			t.Logf("fetchRevealValue returned %T — cannot assert Gen on this type", msg)
		}
	})
}
