// handlers_test.go — failing unit coverage for the six handlers ported from
// internal/tui/app_flash.go + internal/tui/app_session.go to runtime.Core in
// AS-324 (PR-05a-h3). Companion QA for AS-327; sibling impl is AS-324.
//
// Until both AS-323 (session field migration) and AS-324 (handler port) land,
// this file fails to compile — by design. The compile failure IS the red
// signal that TDD requires before Coder makes them pass.
//
// References:
//   - Spec: docs/refactor/05-pr-05a-h2.md §"PR-05a-h3 (AS-315b)"
//   - Predecessor: AS-323 (session field migration)
//   - Sibling impl: AS-324
package runtime

import (
	"errors"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/aws/smithy-go"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/session"
)

// smithyAPIErrorStub satisfies smithy.APIError so internal/aws.ClassifyAWSError
// returns a proper code/message pair (otherwise it falls through to the
// "Unknown" branch and strips the [CODE] prefix the handler is expected to set).
type smithyAPIErrorStub struct {
	code string
	msg  string
}

func (e *smithyAPIErrorStub) Error() string             { return e.msg }
func (e *smithyAPIErrorStub) ErrorCode() string         { return e.code }
func (e *smithyAPIErrorStub) ErrorMessage() string      { return e.msg }
func (e *smithyAPIErrorStub) ErrorFault() smithy.ErrorFault {
	return smithy.FaultClient
}

// newCoreWithSession wraps the standard runtime.New constructor so each test
// gets a freshly-rotated session pointing at the new Core.
func newCoreWithSession(t *testing.T) (*Core, *session.Session) {
	t.Helper()
	s := session.New()
	c := New(s, nil)
	return c, s
}

// findIntent returns the first intent matching the predicate (or nil).
func findIntent[T UIIntent](xs []UIIntent) (T, bool) {
	var zero T
	for _, x := range xs {
		if v, ok := x.(T); ok {
			return v, true
		}
	}
	return zero, false
}

// countIntents counts intents of type T in xs.
func countIntents[T UIIntent](xs []UIIntent) int {
	n := 0
	for _, x := range xs {
		if _, ok := x.(T); ok {
			n++
		}
	}
	return n
}

// findTaskPayload returns the first TaskRequest whose Payload is of type T.
func findTaskPayload[T TaskPayload](tasks []TaskRequest) (T, bool) {
	var zero T
	for _, tr := range tasks {
		if v, ok := tr.Payload.(T); ok {
			return v, true
		}
	}
	return zero, false
}

// ----------------------------------------------------------------------------
// TestCoreHandleFlash
// ----------------------------------------------------------------------------

// TestCoreHandleFlash_NonError verifies the happy path: non-error flash emits
// one FlashIntent + one FlashTickPayload TaskRequest, and bumps session.FlashGen.
// No AppendErrorHistoryIntent appears because IsError is false.
func TestCoreHandleFlash_NonError(t *testing.T) {
	c, s := newCoreWithSession(t)
	startGen := s.FlashGen

	intents, tasks := c.HandleFlash(FlashEvent{Text: "hello world", IsError: false})

	if got, want := s.FlashGen, startGen+1; got != want {
		t.Errorf("session.FlashGen = %d, want %d (gen must bump by exactly 1)", got, want)
	}

	fi, ok := findIntent[FlashIntent](intents)
	if !ok {
		t.Fatalf("FlashIntent missing from intents=%v", intents)
	}
	if fi.Text != "hello world" {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "hello world")
	}
	if fi.IsError {
		t.Errorf("FlashIntent.IsError = true, want false")
	}

	if got := countIntents[AppendErrorHistoryIntent](intents); got != 0 {
		t.Errorf("AppendErrorHistoryIntent count = %d, want 0 for non-error flash", got)
	}

	tick, ok := findTaskPayload[FlashTickPayload](tasks)
	if !ok {
		t.Fatalf("FlashTickPayload missing from tasks=%v", tasks)
	}
	if tick.Gen != s.FlashGen {
		t.Errorf("FlashTickPayload.Gen = %d, want %d (must match bumped FlashGen)", tick.Gen, s.FlashGen)
	}
}

// TestCoreHandleFlash_Error verifies error-flash semantics: FlashIntent +
// AppendErrorHistoryIntent + FlashTickPayload TaskRequest. The error message
// is preserved verbatim on both intents.
func TestCoreHandleFlash_Error(t *testing.T) {
	c, s := newCoreWithSession(t)
	startGen := s.FlashGen

	intents, tasks := c.HandleFlash(FlashEvent{Text: "boom", IsError: true})

	if got, want := s.FlashGen, startGen+1; got != want {
		t.Errorf("session.FlashGen = %d, want %d", got, want)
	}

	fi, ok := findIntent[FlashIntent](intents)
	if !ok {
		t.Fatalf("FlashIntent missing from intents=%v", intents)
	}
	if !fi.IsError {
		t.Errorf("FlashIntent.IsError = false, want true")
	}
	if fi.Text != "boom" {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "boom")
	}

	hist, ok := findIntent[AppendErrorHistoryIntent](intents)
	if !ok {
		t.Fatalf("AppendErrorHistoryIntent missing from intents=%v", intents)
	}
	if hist.Message != "boom" {
		t.Errorf("AppendErrorHistoryIntent.Message = %q, want %q", hist.Message, "boom")
	}
	if hist.Time.IsZero() {
		t.Errorf("AppendErrorHistoryIntent.Time is zero — must be set to time.Now() equivalent")
	}

	if _, ok := findTaskPayload[FlashTickPayload](tasks); !ok {
		t.Errorf("FlashTickPayload missing from tasks=%v", tasks)
	}
}

// TestCoreHandleFlash_GenMonotonic verifies that consecutive HandleFlash calls
// bump session.FlashGen by exactly 1 each time — never repeating or skipping.
// Catches the off-by-one bug where two consecutive flashes share a gen and the
// first one's tick prematurely clears the second.
func TestCoreHandleFlash_GenMonotonic(t *testing.T) {
	c, s := newCoreWithSession(t)
	startGen := s.FlashGen

	c.HandleFlash(FlashEvent{Text: "a"})
	c.HandleFlash(FlashEvent{Text: "b"})
	c.HandleFlash(FlashEvent{Text: "c"})

	if got, want := s.FlashGen, startGen+3; got != want {
		t.Errorf("session.FlashGen after 3 HandleFlash calls = %d, want %d (must be monotonically +1 each)", got, want)
	}
}

// ----------------------------------------------------------------------------
// TestCoreHandleClearFlash
// ----------------------------------------------------------------------------

// TestCoreHandleClearFlash_StaleGen verifies that ClearFlash events tagged with
// a gen older than session.FlashGen are dropped: no intents emitted, no tasks.
// This is the staleness guard that prevents a tick from a superseded flash from
// clearing the currently-visible newer flash.
func TestCoreHandleClearFlash_StaleGen(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.FlashGen = 5

	intents, tasks := c.HandleClearFlash(ClearFlashEvent{Gen: 3})

	if intents != nil {
		t.Errorf("intents = %v, want nil (stale gen must drop the clear)", intents)
	}
	if tasks != nil {
		t.Errorf("tasks = %v, want nil", tasks)
	}
}

// TestCoreHandleClearFlash_CurrentGen_NonError verifies the current-gen clear
// path on a non-error flash: emit ClearFlash only, do not emit SetErrorHintIntent.
func TestCoreHandleClearFlash_CurrentGen_NonError(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.FlashGen = 7
	s.FlashIsError = false

	intents, tasks := c.HandleClearFlash(ClearFlashEvent{Gen: 7})

	if _, ok := findIntent[ClearFlash](intents); !ok {
		t.Errorf("ClearFlash intent missing from intents=%v", intents)
	}
	if got := countIntents[SetErrorHintIntent](intents); got != 0 {
		t.Errorf("SetErrorHintIntent count = %d, want 0 for non-error flash clear", got)
	}
	if len(tasks) != 0 {
		t.Errorf("tasks = %v, want empty for ClearFlash", tasks)
	}
}

// TestCoreHandleClearFlash_CurrentGen_Error verifies the current-gen clear path
// on an error flash: emit ClearFlash AND SetErrorHintIntent. The original
// handler sets m.showErrorHint = true when an error flash auto-clears, so the
// emitted intent must request Show=true (port preserves behavior).
func TestCoreHandleClearFlash_CurrentGen_Error(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.FlashGen = 9
	s.FlashIsError = true

	intents, _ := c.HandleClearFlash(ClearFlashEvent{Gen: 9})

	if _, ok := findIntent[ClearFlash](intents); !ok {
		t.Errorf("ClearFlash intent missing from intents=%v", intents)
	}
	hint, ok := findIntent[SetErrorHintIntent](intents)
	if !ok {
		t.Fatalf("SetErrorHintIntent missing from intents=%v (must emit when cleared flash was an error)", intents)
	}
	if !hint.Show {
		t.Errorf("SetErrorHintIntent.Show = false, want true (original handler sets showErrorHint=true after error clears)")
	}
}

// ----------------------------------------------------------------------------
// TestCoreHandleAPIError
// ----------------------------------------------------------------------------

// TestCoreHandleAPIError_SmithyCoded verifies the AWS-error classification
// path: an AWS smithy error with a real code produces a FlashIntent whose Text
// is "[CODE] message". Also emits ClearActiveListLoadingIntent (so the active
// list-view loading spinner stops), AppendErrorHistoryIntent, and a FlashTick
// TaskRequest.
func TestCoreHandleAPIError_SmithyCoded(t *testing.T) {
	c, s := newCoreWithSession(t)
	startGen := s.FlashGen
	awsErr := &smithyAPIErrorStub{code: "AccessDenied", msg: "User not authorized"}

	intents, tasks := c.HandleAPIError(APIErrorEvent{Err: awsErr, ResourceType: "ec2"})

	if got, want := s.FlashGen, startGen+1; got != want {
		t.Errorf("session.FlashGen = %d, want %d", got, want)
	}

	fi, ok := findIntent[FlashIntent](intents)
	if !ok {
		t.Fatalf("FlashIntent missing from intents=%v", intents)
	}
	if !fi.IsError {
		t.Errorf("FlashIntent.IsError = false, want true")
	}
	if !strings.Contains(fi.Text, "[AccessDenied]") {
		t.Errorf("FlashIntent.Text = %q, want to contain %q", fi.Text, "[AccessDenied]")
	}
	if !strings.Contains(fi.Text, "User not authorized") {
		t.Errorf("FlashIntent.Text = %q, want to contain %q", fi.Text, "User not authorized")
	}

	if _, ok := findIntent[ClearActiveListLoadingIntent](intents); !ok {
		t.Errorf("ClearActiveListLoadingIntent missing from intents=%v", intents)
	}
	if _, ok := findIntent[AppendErrorHistoryIntent](intents); !ok {
		t.Errorf("AppendErrorHistoryIntent missing from intents=%v", intents)
	}
	if _, ok := findTaskPayload[FlashTickPayload](tasks); !ok {
		t.Errorf("FlashTickPayload missing from tasks=%v", tasks)
	}
}

// TestCoreHandleAPIError_PlainError verifies the fallback path: a plain Go
// error (not a smithy.APIError) produces a FlashIntent whose Text is the bare
// error message — NO "[CODE]" prefix, because ClassifyAWSError returns code
// "Unknown" for non-smithy errors and the formatter must skip the prefix in
// that case.
func TestCoreHandleAPIError_PlainError(t *testing.T) {
	c, _ := newCoreWithSession(t)

	intents, _ := c.HandleAPIError(APIErrorEvent{Err: errors.New("network timeout"), ResourceType: "rds"})

	fi, ok := findIntent[FlashIntent](intents)
	if !ok {
		t.Fatalf("FlashIntent missing")
	}
	if strings.HasPrefix(fi.Text, "[") {
		t.Errorf("FlashIntent.Text = %q, want NO [CODE] prefix for plain error", fi.Text)
	}
	if !strings.Contains(fi.Text, "network timeout") {
		t.Errorf("FlashIntent.Text = %q, want to contain %q", fi.Text, "network timeout")
	}
}

// ----------------------------------------------------------------------------
// TestCoreHandleClientsReady
// ----------------------------------------------------------------------------

// TestCoreHandleClientsReady_StaleGen verifies stale-gen rejection: an event
// whose Gen doesn't match session.ConnectGen produces no intents, no tasks,
// and does NOT mutate session state (clients, profile, identity).
func TestCoreHandleClientsReady_StaleGen(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.ConnectGen = 5
	s.Profile = "alice"
	s.Region = "us-east-1"
	startGen := s.ConnectGen
	startProfile := s.Profile

	intents, tasks := c.HandleClientsReady(ClientsReadyEvent{Gen: 3, Clients: &awsclient.ServiceClients{}, Region: "us-west-2"})

	if intents != nil {
		t.Errorf("intents = %v, want nil (stale gen)", intents)
	}
	if tasks != nil {
		t.Errorf("tasks = %v, want nil", tasks)
	}
	if s.ConnectGen != startGen {
		t.Errorf("session.ConnectGen mutated: got %d, want %d (stale gen must not mutate)", s.ConnectGen, startGen)
	}
	if s.Profile != startProfile {
		t.Errorf("session.Profile mutated: got %q, want %q", s.Profile, startProfile)
	}
}

// TestCoreHandleClientsReady_SuccessNoRefresh verifies the happy-path success
// branch with no pending refresh: tasks emitted include FetchIdentity +
// LoadAvailCache (since NoCache==false). No RefreshActiveListIntent. No flash.
func TestCoreHandleClientsReady_SuccessNoRefresh(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.ConnectGen = 1
	s.NoCache = false
	s.Profile = "alice"
	s.PendingRefresh = false

	clients := &awsclient.ServiceClients{}
	_, tasks := c.HandleClientsReady(ClientsReadyEvent{Gen: 1, Clients: clients, Region: "us-east-1"})

	if _, ok := findTaskPayload[FetchIdentityPayload](tasks); !ok {
		t.Errorf("FetchIdentityPayload missing from tasks (NoCache=false path must fetch identity)")
	}
	if _, ok := findTaskPayload[LoadAvailCachePayload](tasks); !ok {
		t.Errorf("LoadAvailCachePayload missing from tasks (NoCache=false path must load avail cache)")
	}
	if _, ok := findTaskPayload[DemoPrefetchCountsPayload](tasks); ok {
		t.Errorf("DemoPrefetchCountsPayload present in tasks (must only appear in NoCache=true path)")
	}
	if s.Clients == nil {
		t.Errorf("session.Clients still nil after success — expected to be wired to event.Clients")
	}
}

// TestCoreHandleClientsReady_SuccessNoRefresh_NoCache verifies the NoCache=true
// (demo-mode) branch: emits DemoPrefetchCountsPayload instead of LoadAvailCache.
// FetchIdentity is skipped (the original handler skips identity fetch in demo).
func TestCoreHandleClientsReady_SuccessNoRefresh_NoCache(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.ConnectGen = 1
	s.NoCache = true
	s.PendingRefresh = false

	_, tasks := c.HandleClientsReady(ClientsReadyEvent{Gen: 1, Clients: &awsclient.ServiceClients{}, Region: "us-east-1"})

	if _, ok := findTaskPayload[DemoPrefetchCountsPayload](tasks); !ok {
		t.Errorf("DemoPrefetchCountsPayload missing from tasks (NoCache=true path must demo-prefetch)")
	}
	if _, ok := findTaskPayload[LoadAvailCachePayload](tasks); ok {
		t.Errorf("LoadAvailCachePayload present (must only appear in NoCache=false path)")
	}
}

// TestCoreHandleClientsReady_SuccessWithRefresh verifies the pendingRefresh
// branch: when session.PendingRefresh is true AND the adapter has an active
// resource list (modeled here by the runtime emitting RefreshActiveListIntent
// unconditionally — the adapter no-ops if no list is active), the handler
// emits RefreshActiveListIntent and a "Connected. Refreshing..." FlashIntent.
// session.PendingRefresh is cleared.
func TestCoreHandleClientsReady_SuccessWithRefresh(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.ConnectGen = 2
	s.PendingRefresh = true

	intents, _ := c.HandleClientsReady(ClientsReadyEvent{Gen: 2, Clients: &awsclient.ServiceClients{}, Region: "us-east-1"})

	if _, ok := findIntent[RefreshActiveListIntent](intents); !ok {
		t.Errorf("RefreshActiveListIntent missing from intents (pendingRefresh path must emit refresh)")
	}

	flash, ok := findIntent[FlashIntent](intents)
	if !ok {
		t.Errorf("FlashIntent missing")
	} else if !strings.Contains(flash.Text, "Refreshing") {
		t.Errorf("FlashIntent.Text = %q, want to contain 'Refreshing'", flash.Text)
	}

	if s.PendingRefresh {
		t.Errorf("session.PendingRefresh = true after handler, want false (handler must clear the latch)")
	}
}

// TestCoreHandleClientsReady_Failure verifies the rollback path: when the
// event carries an Err, session.Profile and session.Region must be rolled back
// to PrevProfile/PrevRegion, then HasPrevState/PrevProfile/PrevRegion cleared.
// PendingRefresh is cleared. Intents include FlashIntent (error text) and
// AppendErrorHistoryIntent; tasks include FlashTickPayload plus the
// identity-refetch + avail-cache-reload tasks (since the still-valid old
// clients are retained — see PR-02e P3 finding).
func TestCoreHandleClientsReady_Failure(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.ConnectGen = 4
	s.Profile = "broken-profile"
	s.Region = "us-west-2"
	s.PrevProfile = "alice"
	s.PrevRegion = "us-east-1"
	s.HasPrevState = true
	s.PendingRefresh = true
	s.Clients = &awsclient.ServiceClients{} // still-valid old clients

	intents, tasks := c.HandleClientsReady(ClientsReadyEvent{Gen: 4, Err: errors.New("connect failed")})

	if s.Profile != "alice" {
		t.Errorf("session.Profile = %q, want %q (rollback from PrevProfile)", s.Profile, "alice")
	}
	if s.Region != "us-east-1" {
		t.Errorf("session.Region = %q, want %q (rollback from PrevRegion)", s.Region, "us-east-1")
	}
	if s.HasPrevState {
		t.Errorf("session.HasPrevState = true after rollback, want false")
	}
	if s.PrevProfile != "" {
		t.Errorf("session.PrevProfile = %q, want empty after rollback", s.PrevProfile)
	}
	if s.PendingRefresh {
		t.Errorf("session.PendingRefresh = true after failure, want false (must clear on rollback)")
	}

	if _, ok := findIntent[FlashIntent](intents); !ok {
		t.Errorf("FlashIntent missing on failure path")
	}
	if _, ok := findIntent[AppendErrorHistoryIntent](intents); !ok {
		t.Errorf("AppendErrorHistoryIntent missing on failure path")
	}
	if _, ok := findTaskPayload[FlashTickPayload](tasks); !ok {
		t.Errorf("FlashTickPayload missing from failure tasks")
	}
	// With still-valid old clients, the handler restores identity + avail cache
	// using those clients so the header / menu repopulate.
	if _, ok := findTaskPayload[FetchIdentityPayload](tasks); !ok {
		t.Errorf("FetchIdentityPayload missing — rollback path with non-nil session.Clients must refetch identity")
	}
	if _, ok := findTaskPayload[LoadAvailCachePayload](tasks); !ok {
		t.Errorf("LoadAvailCachePayload missing — rollback path with non-nil session.Clients must reload cache")
	}
}

// ----------------------------------------------------------------------------
// TestCoreHandleProfileSelected
// ----------------------------------------------------------------------------

// TestCoreHandleProfileSelected_FirstSwitch verifies the profile-switch
// orchestration: Rotate() bumps session generations + clears caches; ConnectGen
// is bumped by exactly 1; HasPrevState/PrevProfile/PrevRegion latch the OLD
// values; emitted intents include MenuClearAvailabilityIntent + PopSelectorIntent
// + FlashIntent ("Switching to alice..."). Task: ConnectPayload with the new
// profile and the bumped Gen.
func TestCoreHandleProfileSelected_FirstSwitch(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.Profile = "default"
	s.Region = "us-east-1"
	s.ConnectGen = 0
	s.HasPrevState = false
	// Pre-populate a known generation so we can detect Rotate() ran.
	prevRelatedGen := s.RelatedGen

	intents, tasks := c.HandleProfileSelected(ProfileSelectedEvent{Profile: "alice"})

	// Rotate() bumps RelatedGen — proxy proof it was called.
	if s.RelatedGen == prevRelatedGen {
		t.Errorf("session.RelatedGen unchanged after HandleProfileSelected — Session.Rotate() was not invoked")
	}
	if s.ConnectGen != 1 {
		t.Errorf("session.ConnectGen = %d, want 1 (must bump by 1)", s.ConnectGen)
	}
	if !s.HasPrevState {
		t.Errorf("session.HasPrevState = false, want true on first switch")
	}
	if s.PrevProfile != "default" {
		t.Errorf("session.PrevProfile = %q, want %q (must latch old profile)", s.PrevProfile, "default")
	}
	if s.PrevRegion != "us-east-1" {
		t.Errorf("session.PrevRegion = %q, want %q (must latch old region)", s.PrevRegion, "us-east-1")
	}
	if s.Profile != "alice" {
		t.Errorf("session.Profile = %q, want %q (must update to new profile)", s.Profile, "alice")
	}
	if !s.PendingRefresh {
		t.Errorf("session.PendingRefresh = false, want true (handler must latch refresh for the next ClientsReady)")
	}

	if _, ok := findIntent[MenuClearAvailabilityIntent](intents); !ok {
		t.Errorf("MenuClearAvailabilityIntent missing")
	}
	if _, ok := findIntent[PopSelectorIntent](intents); !ok {
		t.Errorf("PopSelectorIntent missing")
	}
	flash, ok := findIntent[FlashIntent](intents)
	if !ok {
		t.Errorf("FlashIntent missing")
	} else if !strings.Contains(flash.Text, "alice") {
		t.Errorf("FlashIntent.Text = %q, want to contain new profile name 'alice'", flash.Text)
	}

	connect, ok := findTaskPayload[ConnectPayload](tasks)
	if !ok {
		t.Fatalf("ConnectPayload missing from tasks")
	}
	if connect.Profile != "alice" {
		t.Errorf("ConnectPayload.Profile = %q, want %q", connect.Profile, "alice")
	}
	if connect.Gen != s.ConnectGen {
		t.Errorf("ConnectPayload.Gen = %d, want %d (must match session.ConnectGen)", connect.Gen, s.ConnectGen)
	}
}

// TestCoreHandleProfileSelected_RapidSwitchKeepsOriginalPrev verifies the
// first-switch-only latch invariant: rapid A→B→C must keep PrevProfile=A, not
// overwrite to B. Catches the off-by-one rollback bug where a second switch
// destroys the original anchor.
func TestCoreHandleProfileSelected_RapidSwitchKeepsOriginalPrev(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.Profile = "A"
	s.Region = "us-east-1"
	s.HasPrevState = false

	c.HandleProfileSelected(ProfileSelectedEvent{Profile: "B"})
	if s.PrevProfile != "A" {
		t.Fatalf("after A→B: PrevProfile = %q, want %q", s.PrevProfile, "A")
	}

	c.HandleProfileSelected(ProfileSelectedEvent{Profile: "C"})
	if s.PrevProfile != "A" {
		t.Errorf("after A→B→C: PrevProfile = %q, want %q (rapid switch must keep ORIGINAL anchor, not overwrite)", s.PrevProfile, "A")
	}
}

// ----------------------------------------------------------------------------
// TestCoreHandleRegionSelected
// ----------------------------------------------------------------------------

// TestCoreHandleRegionSelected_FirstSwitch mirrors HandleProfileSelected for
// region: Rotate() runs, ConnectGen bumps, PrevRegion latches, intents are
// MenuClearAvailability + PopSelector + Flash, task is ConnectPayload with the
// new region.
func TestCoreHandleRegionSelected_FirstSwitch(t *testing.T) {
	c, s := newCoreWithSession(t)
	s.Profile = "alice"
	s.Region = "us-east-1"
	s.ConnectGen = 0
	s.HasPrevState = false
	prevRelatedGen := s.RelatedGen

	intents, tasks := c.HandleRegionSelected(RegionSelectedEvent{Region: "eu-west-1"})

	if s.RelatedGen == prevRelatedGen {
		t.Errorf("session.RelatedGen unchanged — Session.Rotate() was not invoked")
	}
	if s.ConnectGen != 1 {
		t.Errorf("session.ConnectGen = %d, want 1", s.ConnectGen)
	}
	if s.PrevRegion != "us-east-1" {
		t.Errorf("session.PrevRegion = %q, want %q", s.PrevRegion, "us-east-1")
	}
	if s.Region != "eu-west-1" {
		t.Errorf("session.Region = %q, want %q", s.Region, "eu-west-1")
	}
	if !s.PendingRefresh {
		t.Errorf("session.PendingRefresh = false, want true")
	}

	if _, ok := findIntent[MenuClearAvailabilityIntent](intents); !ok {
		t.Errorf("MenuClearAvailabilityIntent missing")
	}
	if _, ok := findIntent[PopSelectorIntent](intents); !ok {
		t.Errorf("PopSelectorIntent missing")
	}
	flash, ok := findIntent[FlashIntent](intents)
	if !ok {
		t.Errorf("FlashIntent missing")
	} else if !strings.Contains(flash.Text, "eu-west-1") {
		t.Errorf("FlashIntent.Text = %q, want to contain new region name", flash.Text)
	}

	connect, ok := findTaskPayload[ConnectPayload](tasks)
	if !ok {
		t.Fatalf("ConnectPayload missing")
	}
	if connect.Region != "eu-west-1" {
		t.Errorf("ConnectPayload.Region = %q, want %q", connect.Region, "eu-west-1")
	}
	if connect.Profile != "alice" {
		t.Errorf("ConnectPayload.Profile = %q, want %q (must preserve current profile)", connect.Profile, "alice")
	}
}

// ----------------------------------------------------------------------------
// TestRuntimeBoundary_HandlersTest
// ----------------------------------------------------------------------------

// TestRuntimeBoundary_HandlersTest is a compile-time guard: handlers_test.go
// itself must only import internal/runtime peers + stdlib + the AWS error
// helpers package. NO bubbletea/lipgloss/bubbles — those belong in the TUI
// adapter, not in runtime tests. The full boundary check across the whole
// internal/runtime package runs under `make ready-to-push` per AS-323
// acceptance.
func TestRuntimeBoundary_HandlersTest(t *testing.T) {
	const path = "handlers_test.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	forbidden := []string{"bubbletea", "lipgloss", "bubbles"}
	for _, imp := range file.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		for _, f := range forbidden {
			if strings.Contains(p, f) {
				t.Errorf("handlers_test.go imports forbidden package %q (contains %q)", p, f)
			}
		}
	}
}
