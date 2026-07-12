package unit

// iam_inline_sweep_test.go pins the live production defect where the "policy"
// resource type's availability/count probe blocks the whole main-menu badge
// refresh on a per-group IAM inline-policy sweep.
//
// Verified root-cause chain (traced against the code at HEAD, not assumed):
//
//  1. internal/runtime/probes.go:630-649 (Core.ProbeResourceAvailability)
//     resolves pf := resource.GetPaginatedFetcher("policy"), opens a 10s
//     context.WithTimeout at line 644, and calls pf(probeCtx, clients, "")
//     — i.e. the FIRST PAGE of the "policy" fetcher, continuationToken="".
//  2. internal/aws/catalog_security.go:144-169 registers that exact Fetcher
//     closure for ShortName "policy". On the first page (continuationToken
//     == "") it calls FetchIAMPoliciesPage (cheap, ListPolicies Scope=Local
//     only) and THEN unconditionally calls fetchInlineGroupPolicies(ctx,
//     c.IAM) before returning — so the probe's cheap managed-only intent is
//     defeated: it always also runs the full per-group inline sweep.
//  3. internal/aws/iam_policies.go:420-462 (fetchInlineGroupPolicies) lists
//     ALL IAM groups via ListGroups, then loops them SEQUENTIALLY, issuing
//     one ListGroupPolicies per group, each wrapped in RetryOnThrottle
//     (internal/aws/retry.go:38). On a profile with many groups this burns
//     the probe's shared 10s deadline; groups whose call lands after the
//     deadline fail with a non-retryable ctx error (context errors are not
//     smithy.APIError, so ClassifyAWSError — internal/aws/errors.go:11 —
//     always returns retryable=false for them, so RetryOnThrottle does not
//     even get a backoff cycle to make things worse, it just fails fast).
//  4. Per-group failures are collected into one composite via
//     internal/aws/partial_errors.go:33 AggregateFailures("ListGroupPolicies",
//     failures, total), which joins EVERY failing ID with no cap — on the
//     reported profile (49 IAM groups, 36 timed out) this produced the
//     single unbounded flash line:
//     "availability policy: ListGroupPolicies failed for 36 of 49 IDs: <36
//     joined per-group reasons>" (formatted by
//     internal/runtime/handlers_availability.go:317).
//
// Three independent, behavior-level regressions are pinned below, each
// against the real seam a fix must touch.
//
// Item 1 was re-pinned after a coder proved the original item-1 and item-2
// tests were mutually exclusive at the shared seam: both drove
// resource.GetPaginatedFetcher("policy") with continuationToken="", and
// there is no production-observable signal at that single function that
// tells a "probe wants cheap managed-only" call apart from a "real list-open
// wants everything" call — the coder's item-2 fix (bounded concurrency,
// landed) legitimately keeps fetchInlineGroupPolicies in that same closure
// for real list-opens. Per the repo's stated preference for deterministic
// per-type config over signal-guessing, item 1 now pins an EXPLICIT new
// registry seam instead, following the existing per-type registration
// family in internal/resource (SetPaginatedForTest / GetPaginatedFetcher /
// CleanupPaginatedForTest in accessors.go; SetFetchByIDsForTest /
// GetFetchByIDs / CleanupFetchByIDsForTest in related.go):
//
//   - resource.SetAvailabilityFetcherForTest(shortName, f) /
//     resource.GetAvailabilityFetcher(shortName) /
//     resource.CleanupAvailabilityFetcherForTest(shortName) — a new registry,
//     same PaginatedFetcher shape, NOT YET IMPLEMENTED as of this test file.
//   - internal/runtime/probes.go's Core.ProbeResourceAvailability must call
//     resource.GetAvailabilityFetcher(shortName) first and use it WHEN
//     REGISTERED, falling back to resource.GetPaginatedFetcher(shortName)
//     otherwise — zero behavior change for the other 66 types that never
//     register one.
//   - internal/aws must register a cheap managed-only availability fetcher
//     for "policy" (e.g. alongside the Fetcher closure in
//     catalog_security.go) that never calls ListGroupPolicies.
//
// The three new item-1 tests below (TestProbeResourceAvailability_*,
// TestPolicyAvailabilityFetcher_Registered*) reference these not-yet-existing
// resource.*AvailabilityFetcher* symbols directly. This is a deliberate,
// scoped TDD RED step (per this repo's established multi-symbol red-phase
// practice): tests/unit will not build until a coder lands the registry +
// probe wiring + "policy" registration. Items 2 and 3 below are unchanged
// from the prior (already-landed, green) round; their source is left as-is
// so the diff a coder needs to unblock the package is exactly the three new
// symbols plus the probes.go branch and the "policy" registration — nothing
// else in this file changes.
//
//   - TestFetchInlineGroupPolicies_SequentialSweepMissesDeadline and
//     TestFetchInlineGroupPolicies_ConcurrencyStaysBounded pin (3): a
//     deadline-bound completion assertion (RED today — sequential can't
//     finish 49 groups before a deadline sized to make that structurally
//     impossible) plus a forward-looking max-in-flight guard (already green
//     today, since sequential trivially satisfies "at most 5 concurrent" —
//     it becomes a real regression guard once a bounded-concurrency fix
//     lands, per the confirmed scope: "keep item 2's in-flight-bound
//     assertion as the forward-looking guard").
//     fetchInlineGroupPolicies itself is unexported, so both tests drive it
//     indirectly through the same resource.GetPaginatedFetcher("policy")
//     seam used above (and by the existing
//     tests/unit/aws_iam_group_inline_policy_test.go callPolicyFetcher
//     pattern) — there is no other way to reach it from package unit.
//   - The three AggregateFailures tests pin (4): the composite builder must
//     cap per-ID enumeration and summarize the remainder instead of joining
//     an unbounded list. RED today (current implementation joins every
//     failure with no cap — see internal/aws/partial_errors.go:33-39).
//
// Timing values below are scaled down from the reported 200ms/10s numbers
// to keep this file fast (whole-file wall time budget: a few seconds), while
// preserving the same structural ratio: sequential total time clearly
// exceeds the deadline, bounded-concurrency time clearly does not.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ---------------------------------------------------------------------------
// Shared local mock: inlineSweepIAM
//
// Implements awsclient.IAMAPI. Only the methods the "policy" paginated
// fetcher can reach (ListPolicies, ListGroups, ListGroupPolicies) have real
// behavior; every other method panics so an unexpected call surfaces
// immediately rather than silently returning a zero value. Mirrors the
// panic-stub convention already used by stubGroupPolicyIAM in
// tests/unit/aws_iam_group_inline_policy_test.go, kept local to this file
// per QA mock-locality rules (a distinct type, not a shared import, since
// the two files pin different scenarios).
// ---------------------------------------------------------------------------

type inlineSweepIAM struct {
	t *testing.T

	groups []iamtypes.Group

	// managedPolicies is returned verbatim by ListPolicies.
	managedPolicies []iamtypes.Policy

	// perCallSleep, when non-zero, is how long each ListGroupPolicies call
	// blocks before succeeding. Context-aware: an already-expired ctx (or one
	// that expires mid-sleep) returns ctx.Err() immediately instead of
	// waiting out the full duration — this is what lets a deadline-bound
	// sequential run fail fast instead of hanging past its context.
	perCallSleep time.Duration

	// failListGroupPolicies, when true, fails the test the moment
	// ListGroupPolicies is invoked at all — used to pin that the
	// availability-probe seam must never reach the per-group inline sweep.
	failListGroupPolicies bool

	mu          sync.Mutex
	calls       int
	inFlight    int
	maxInFlight int
}

func (s *inlineSweepIAM) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{Policies: s.managedPolicies}, nil
}

func (s *inlineSweepIAM) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{Groups: s.groups}, nil
}

func (s *inlineSweepIAM) ListGroupPolicies(ctx context.Context, in *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	if s.failListGroupPolicies {
		s.t.Helper()
		s.t.Fatalf("ListGroupPolicies(%s) called on the availability-probe seam; "+
			"the probe's first-page fetch (internal/runtime/probes.go:644, "+
			"resource.GetPaginatedFetcher(\"policy\")) must stay on the cheap "+
			"managed-only path and never reach the per-group inline sweep "+
			"(internal/aws/catalog_security.go:159, internal/aws/iam_policies.go:425)",
			aws.ToString(in.GroupName))
	}

	s.mu.Lock()
	s.calls++
	s.inFlight++
	if s.inFlight > s.maxInFlight {
		s.maxInFlight = s.inFlight
	}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.inFlight--
		s.mu.Unlock()
	}()

	if s.perCallSleep > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.perCallSleep):
		}
	}

	return &iam.ListGroupPoliciesOutput{
		PolicyNames: []string{fmt.Sprintf("inline-%s", aws.ToString(in.GroupName))},
	}, nil
}

func (s *inlineSweepIAM) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *inlineSweepIAM) maxConcurrency() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxInFlight
}

// Methods below are part of awsclient.IAMAPI but are never called by the
// "policy" paginated fetcher (they belong to unrelated related-checkers,
// lazy-add, or Wave 2 enrichment paths) — panic so a future change that
// accidentally routes through this mock is caught immediately.
func (s *inlineSweepIAM) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	panic("inlineSweepIAM.ListRoles called unexpectedly")
}
func (s *inlineSweepIAM) ListUsers(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	panic("inlineSweepIAM.ListUsers called unexpectedly")
}
func (s *inlineSweepIAM) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	panic("inlineSweepIAM.ListAttachedRolePolicies called unexpectedly")
}
func (s *inlineSweepIAM) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	panic("inlineSweepIAM.ListRolePolicies called unexpectedly")
}
func (s *inlineSweepIAM) ListAttachedUserPolicies(_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	panic("inlineSweepIAM.ListAttachedUserPolicies called unexpectedly")
}
func (s *inlineSweepIAM) ListAttachedGroupPolicies(_ context.Context, _ *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	panic("inlineSweepIAM.ListAttachedGroupPolicies called unexpectedly")
}
func (s *inlineSweepIAM) ListGroupsForUser(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	panic("inlineSweepIAM.ListGroupsForUser called unexpectedly")
}
func (s *inlineSweepIAM) ListEntitiesForPolicy(_ context.Context, _ *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	panic("inlineSweepIAM.ListEntitiesForPolicy called unexpectedly")
}
func (s *inlineSweepIAM) ListAccountAliases(_ context.Context, _ *iam.ListAccountAliasesInput, _ ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	panic("inlineSweepIAM.ListAccountAliases called unexpectedly")
}
func (s *inlineSweepIAM) GetGroup(_ context.Context, _ *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	panic("inlineSweepIAM.GetGroup called unexpectedly")
}
func (s *inlineSweepIAM) GetPolicy(_ context.Context, _ *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	panic("inlineSweepIAM.GetPolicy called unexpectedly")
}
func (s *inlineSweepIAM) GetPolicyVersion(_ context.Context, _ *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	panic("inlineSweepIAM.GetPolicyVersion called unexpectedly")
}
func (s *inlineSweepIAM) GetRolePolicy(_ context.Context, _ *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	panic("inlineSweepIAM.GetRolePolicy called unexpectedly")
}
func (s *inlineSweepIAM) GetLoginProfile(_ context.Context, _ *iam.GetLoginProfileInput, _ ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error) {
	panic("inlineSweepIAM.GetLoginProfile called unexpectedly")
}
func (s *inlineSweepIAM) ListMFADevices(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	panic("inlineSweepIAM.ListMFADevices called unexpectedly")
}
func (s *inlineSweepIAM) ListAccessKeys(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	panic("inlineSweepIAM.ListAccessKeys called unexpectedly")
}
func (s *inlineSweepIAM) GetInstanceProfile(_ context.Context, _ *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	panic("inlineSweepIAM.GetInstanceProfile called unexpectedly")
}

// compile-time check
var _ awsclient.IAMAPI = (*inlineSweepIAM)(nil)

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

// inlineSweepGroupName produces a realistic, synthetic IAM group name in the
// "team@example-env" style — never a real client/account identifier.
func inlineSweepGroupName(i int) string {
	return fmt.Sprintf("devops@example-prod-%02d", i+1)
}

func newInlineSweepGroups(n int) []iamtypes.Group {
	groups := make([]iamtypes.Group, 0, n)
	for i := 0; i < n; i++ {
		name := inlineSweepGroupName(i)
		groups = append(groups, iamtypes.Group{
			GroupName: aws.String(name),
			GroupId:   aws.String(fmt.Sprintf("AGPA%05d", i+1)),
			Arn:       aws.String(fmt.Sprintf("arn:aws:iam::123456789012:group/%s", name)),
			Path:      aws.String("/"),
		})
	}
	return groups
}

// callInlineSweepPolicyFetcher drives the registered "policy" paginated
// fetcher — the same function resource.GetPaginatedFetcher("policy") hands
// to Core.ProbeResourceAvailability (internal/runtime/probes.go:637-649) —
// with continuationToken="" (first page), exactly as the probe calls it.
func callInlineSweepPolicyFetcher(t *testing.T, ctx context.Context, stub *inlineSweepIAM) ([]resource.Resource, error) {
	t.Helper()
	fetcher := resource.GetPaginatedFetcher("policy")
	if fetcher == nil {
		t.Fatal("no paginated fetcher registered for 'policy' — internal/aws not imported?")
	}
	clients := &awsclient.ServiceClients{IAM: stub}
	result, err := fetcher(ctx, clients, "")
	return result.Resources, err
}

// ---------------------------------------------------------------------------
// Item 1 — availability probe must use a dedicated, explicitly-registered
// availability fetcher when one exists, instead of guessing intent at the
// shared paginated-fetcher seam.
// ---------------------------------------------------------------------------

// policyManagedFixture is the same two-policy managed fixture the prior round
// of this file used, shared by the two production-registration tests below.
func policyManagedFixture() []iamtypes.Policy {
	return []iamtypes.Policy{
		{
			PolicyName:      aws.String("s3-read-only"),
			Arn:             aws.String("arn:aws:iam::123456789012:policy/s3-read-only"),
			AttachmentCount: aws.Int32(3),
			IsAttachable:    true,
		},
		{
			PolicyName:      aws.String("deploy-policy"),
			Arn:             aws.String("arn:aws:iam::123456789012:policy/deploy-policy"),
			AttachmentCount: aws.Int32(1),
			IsAttachable:    true,
		},
	}
}

// TestProbeResourceAvailability_UsesRegisteredAvailabilityFetcher pins the
// registry-level + probe-wiring contract: once ANY availability fetcher is
// registered for a short name, Core.ProbeResourceAvailability must call it
// instead of the paginated fetcher, and never touch the IAM group sweep. A
// deliberately trivial marker fetcher (not the real "policy" managed-only
// implementation — that is pinned separately below) proves the probe really
// dispatched to the registered function rather than falling back.
func TestProbeResourceAvailability_UsesRegisteredAvailabilityFetcher(t *testing.T) {
	const marker = "marker-from-registered-availability-fetcher"
	resource.SetAvailabilityFetcherForTest("policy", func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources: []resource.Resource{{ID: marker, Name: marker}},
		}, nil
	})
	defer resource.CleanupAvailabilityFetcherForTest("policy")

	// If the probe ignores the registration above and falls back to the real
	// "policy" paginated fetcher, it will reach fetchInlineGroupPolicies and
	// this stub trips the test immediately.
	stub := &inlineSweepIAM{
		t:                     t,
		groups:                newInlineSweepGroups(49),
		failListGroupPolicies: true,
		managedPolicies:       policyManagedFixture(),
	}
	clients := &awsclient.ServiceClients{IAM: stub}

	core := runtime.New(session.New(), catalog.All())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := core.ProbeResourceAvailability(ctx, clients, "policy")
	if result.Err != nil {
		t.Fatalf("probe returned an error using the registered availability fetcher: %v", result.Err)
	}
	if result.Count != 1 || len(result.Resources) != 1 || result.Resources[0].ID != marker {
		t.Errorf("probe did not use the registered availability fetcher: got Count=%d Resources=%+v, want the single %q marker resource",
			result.Count, result.Resources, marker)
	}
}

// TestProbeResourceAvailability_FallsBackWithoutRegisteredAvailabilityFetcher
// pins the zero-behavior-change guarantee for the other 66 types: when no
// availability fetcher is registered for a short name, the probe must still
// call the registered paginated fetcher exactly as it does today.
func TestProbeResourceAvailability_FallsBackWithoutRegisteredAvailabilityFetcher(t *testing.T) {
	const shortName = "inline-sweep-fallback-probe-type"

	if resource.GetAvailabilityFetcher(shortName) != nil {
		t.Fatal("test setup: unexpected availability fetcher already registered for a synthetic short name")
	}

	calledPaginated := false
	resource.SetPaginatedForTest(shortName, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		calledPaginated = true
		return resource.FetchResult{Resources: []resource.Resource{{ID: "x", Name: "x"}}}, nil
	})
	defer resource.CleanupPaginatedForTest(shortName)

	core := runtime.New(session.New(), catalog.All())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := core.ProbeResourceAvailability(ctx, &awsclient.ServiceClients{}, shortName)
	if !calledPaginated {
		t.Error("probe did not fall back to the registered paginated fetcher when no availability " +
			"fetcher was registered — this breaks the zero-behavior-change guarantee for every " +
			"other resource type")
	}
	if result.Err != nil {
		t.Errorf("unexpected error from the fallback path: %v", result.Err)
	}
}

// TestPolicyAvailabilityFetcher_RegisteredForPolicy_ManagedOnly pins the
// third piece: internal/aws must actually register a cheap, managed-only
// availability fetcher for "policy" (not just leave the registry mechanism
// unused). Fetches it directly via resource.GetAvailabilityFetcher("policy")
// — bypassing SetAvailabilityFetcherForTest entirely — so this exercises the
// real production registration, not a test override.
func TestPolicyAvailabilityFetcher_RegisteredForPolicy_ManagedOnly(t *testing.T) {
	fetcher := resource.GetAvailabilityFetcher("policy")
	if fetcher == nil {
		t.Fatal("no availability fetcher registered for \"policy\"; internal/aws must register one " +
			"(e.g. alongside the Fetcher closure in internal/aws/catalog_security.go) so the " +
			"availability probe never falls back to the full paginated fetcher and its inline sweep")
	}

	stub := &inlineSweepIAM{
		t:                     t,
		groups:                newInlineSweepGroups(49),
		failListGroupPolicies: true,
		managedPolicies:       policyManagedFixture(),
	}
	clients := &awsclient.ServiceClients{IAM: stub}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := fetcher(ctx, clients, "")
	if err != nil {
		t.Fatalf("registered \"policy\" availability fetcher returned an error: %v", err)
	}
	if len(result.Resources) != 2 {
		t.Fatalf("expected the 2 cheap managed policies back from the \"policy\" availability fetcher, got %d: %+v",
			len(result.Resources), result.Resources)
	}
	names := map[string]bool{}
	for _, r := range result.Resources {
		names[r.Name] = true
	}
	for _, want := range []string{"s3-read-only", "deploy-policy"} {
		if !names[want] {
			t.Errorf("managed policy %q missing from the \"policy\" availability fetcher's result; got: %v", want, result.Resources)
		}
	}
}

// ---------------------------------------------------------------------------
// Item 2 — fetchInlineGroupPolicies must complete N slow groups within a
// realistic deadline via bounded concurrency, without unbounded fan-out.
// ---------------------------------------------------------------------------

// TestFetchInlineGroupPolicies_SequentialSweepMissesDeadline carries the RED
// signal for item 2: 49 groups whose ListGroupPolicies each take 100ms
// cannot all complete sequentially inside a 3s deadline (100ms * 49 = 4.9s).
// A fix that fans the sweep out with bounded concurrency (4-5 workers) would
// finish comfortably inside this deadline (49/5 * 100ms ~= 1s); today's
// sequential loop cannot, and this test pins that gap. Numbers are scaled
// down from the reported 200ms/49-groups/10s scenario to keep the file fast
// while preserving the same "sequential clearly overruns, bounded
// concurrency clearly doesn't" ratio.
func TestFetchInlineGroupPolicies_SequentialSweepMissesDeadline(t *testing.T) {
	const numGroups = 49
	const perCallSleep = 100 * time.Millisecond
	const deadline = 3 * time.Second

	stub := &inlineSweepIAM{
		t:            t,
		groups:       newInlineSweepGroups(numGroups),
		perCallSleep: perCallSleep,
	}

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	start := time.Now()
	resources, err := callInlineSweepPolicyFetcher(t, ctx, stub)
	elapsed := time.Since(start)

	inlineCount := 0
	for _, r := range resources {
		if strings.HasPrefix(r.Name, "inline-") {
			inlineCount++
		}
	}

	if err == nil && inlineCount == numGroups {
		// Would only happen once bounded concurrency ships — document the win.
		t.Logf("all %d groups completed within %v (elapsed %v) — bounded concurrency is in place", numGroups, deadline, elapsed)
	} else {
		t.Errorf("RED: only %d/%d groups' inline policies were recovered within a %v deadline "+
			"(sequential worst case is %d * %v = %v > %v deadline); err=%v; "+
			"internal/aws/iam_policies.go:425 fetchInlineGroupPolicies must fan the "+
			"per-group ListGroupPolicies sweep out with bounded concurrency instead "+
			"of looping sequentially",
			inlineCount, numGroups, deadline, numGroups, perCallSleep, numGroups*perCallSleep, deadline, err)
	}

	// Sanity: the function must still respect the context and return at (or
	// shortly after) the deadline rather than hanging indefinitely, whether
	// or not the fix has landed yet.
	const slack = 1 * time.Second
	if elapsed > deadline+slack {
		t.Errorf("fetchInlineGroupPolicies took %v to return, want <= deadline(%v)+slack(%v)=%v; "+
			"it must respect ctx and stop promptly once the deadline passes",
			elapsed, deadline, slack, deadline+slack)
	}
}

// TestFetchInlineGroupPolicies_ConcurrencyStaysBounded is the forward-looking
// guard flagged in scoring: today's sequential loop trivially satisfies
// "at most 5 concurrent ListGroupPolicies calls" (max in-flight is always 1),
// so this assertion is green now. It becomes load-bearing the moment a
// bounded-concurrency fix lands for item 2 above — it is what stops that fix
// from regressing into unbounded goroutine fan-out (IAM throttling risk).
func TestFetchInlineGroupPolicies_ConcurrencyStaysBounded(t *testing.T) {
	const numGroups = 49
	const perCallSleep = 10 * time.Millisecond
	const maxAllowedInFlight = 5
	// Deadline generous enough that even today's sequential loop
	// (49 * 10ms ~= 490ms) finishes comfortably — this test isolates the
	// concurrency-bound question from the deadline question above.
	const deadline = 5 * time.Second

	stub := &inlineSweepIAM{
		t:            t,
		groups:       newInlineSweepGroups(numGroups),
		perCallSleep: perCallSleep,
	}

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	resources, err := callInlineSweepPolicyFetcher(t, ctx, stub)
	if err != nil {
		t.Fatalf("expected all %d groups to succeed within the generous %v deadline, got error: %v", numGroups, deadline, err)
	}
	if len(resources) != numGroups {
		t.Fatalf("expected %d inline policy resources (one per group), got %d", numGroups, len(resources))
	}
	if stub.callCount() != numGroups {
		t.Fatalf("expected exactly %d ListGroupPolicies calls (one per group), got %d", numGroups, stub.callCount())
	}

	if got := stub.maxConcurrency(); got > maxAllowedInFlight {
		t.Errorf("fetchInlineGroupPolicies ran %d ListGroupPolicies calls concurrently, want <= %d; "+
			"unbounded goroutine fan-out risks IAM throttling on real accounts "+
			"(internal/aws/iam_policies.go:425)",
			got, maxAllowedInFlight)
	}
}

// ---------------------------------------------------------------------------
// Item 3 — partial-failure aggregation must cap enumeration
// ---------------------------------------------------------------------------

// syntheticGroupFailures builds n realistic "<group>: <reason>" failure
// strings in the exact shape fetchInlineGroupPolicies feeds to
// AggregateFailures (internal/aws/iam_policies.go:444).
func syntheticGroupFailures(n, startIdx int) []string {
	failures := make([]string, 0, n)
	for i := 0; i < n; i++ {
		name := inlineSweepGroupName(startIdx + i)
		failures = append(failures, fmt.Sprintf("%s: context deadline exceeded", name))
	}
	return failures
}

// TestAggregateFailures_CapsPerIDEnumeration_36of49GroupSweep reproduces the
// exact reported scenario (49 groups, 36 failing) and pins that the
// composite error names at most 5 IDs and summarizes the remainder, instead
// of joining all 36 into one unbounded line. RED today — AggregateFailures
// (internal/aws/partial_errors.go:33) joins every failure with no cap.
func TestAggregateFailures_CapsPerIDEnumeration_36of49GroupSweep(t *testing.T) {
	const total = 49
	const failCount = 36
	const capLimit = 5
	failures := syntheticGroupFailures(failCount, total-failCount)

	err := awsclient.AggregateFailures("ListGroupPolicies", failures, total)
	if err == nil {
		t.Fatal("AggregateFailures with 36/49 failures returned nil, want a composite error")
	}
	msg := err.Error()

	if !strings.HasPrefix(msg, fmt.Sprintf("ListGroupPolicies failed for %d of %d IDs", failCount, total)) {
		t.Errorf("composite error dropped its header shape; got: %s", msg)
	}

	named := 0
	for _, f := range failures {
		if strings.Contains(msg, f) {
			named++
		}
	}
	if named > capLimit {
		t.Errorf("composite error names %d of %d failing IDs verbatim, want at most %d; "+
			"AggregateFailures (internal/aws/partial_errors.go:33) must cap per-ID "+
			"enumeration and summarize the remainder (e.g. \"and %d more\") — a "+
			"36-of-49 IAM group sweep currently produces one unbounded log line\ngot: %s",
			named, failCount, capLimit, failCount-capLimit, msg)
	}

	wantRemainder := fmt.Sprintf("and %d more", failCount-capLimit)
	if !strings.Contains(msg, wantRemainder) {
		t.Errorf("composite error does not summarize the elided failures; want it to contain %q, got: %s",
			wantRemainder, msg)
	}

	const maxLen = 450
	if len(msg) > maxLen {
		t.Errorf("composite error is %d bytes, want <= %d; unbounded per-ID enumeration "+
			"makes a single flash/log line grow without limit on wide profiles\ngot: %s",
			len(msg), maxLen, msg)
	}
}

// TestAggregateFailures_NoCapWhenFailureCountIsSmall pins that the cap does
// not kick in below the threshold: a 3-of-49 failure set must still name all
// 3 IDs verbatim and must NOT carry an "and N more" suffix. This guards
// against an overzealous fix that always truncates regardless of count.
func TestAggregateFailures_NoCapWhenFailureCountIsSmall(t *testing.T) {
	const total = 49
	const failCount = 3
	failures := syntheticGroupFailures(failCount, 0)

	err := awsclient.AggregateFailures("ListGroupPolicies", failures, total)
	if err == nil {
		t.Fatal("AggregateFailures with 3/49 failures returned nil, want a composite error")
	}
	msg := err.Error()

	for _, f := range failures {
		if !strings.Contains(msg, f) {
			t.Errorf("small failure set must name every ID verbatim; %q missing from: %s", f, msg)
		}
	}
	if strings.Contains(msg, "more") {
		t.Errorf("small (%d) failure set should not be truncated with an \"and N more\" suffix; got: %s", failCount, msg)
	}
}

// TestAggregateFailures_CapsEnumeration_AllIDsFailing is the edge case where
// every attempted ID fails (49 of 49) — the cap must still apply rather than
// only kicking in for partial failure.
func TestAggregateFailures_CapsEnumeration_AllIDsFailing(t *testing.T) {
	const total = 49
	const failCount = 49
	const capLimit = 5
	failures := syntheticGroupFailures(failCount, 0)

	err := awsclient.AggregateFailures("ListGroupPolicies", failures, total)
	if err == nil {
		t.Fatal("AggregateFailures with 49/49 failures returned nil, want a composite error")
	}
	msg := err.Error()

	named := 0
	for _, f := range failures {
		if strings.Contains(msg, f) {
			named++
		}
	}
	if named > capLimit {
		t.Errorf("composite error names %d of %d failing IDs verbatim, want at most %d even "+
			"when every ID fails; got: %s", named, failCount, capLimit, msg)
	}

	wantRemainder := fmt.Sprintf("and %d more", failCount-capLimit)
	if !strings.Contains(msg, wantRemainder) {
		t.Errorf("composite error does not summarize the elided failures; want it to contain %q, got: %s",
			wantRemainder, msg)
	}
}

// ---------------------------------------------------------------------------
// C7 (self-review, regression from this branch) — the "policy" availability
// fetcher (internal/aws/catalog_security.go's AvailabilityFetcher) is JUST
// FetchIAMPoliciesPage — a direct pass-through of ListPolicies' own
// IsTruncated flag. On an inline-only account (zero managed policies,
// inline policies live entirely on groups this cheap probe deliberately
// never checks), ListPolicies genuinely has no more MANAGED pages, so
// IsTruncated comes back false — the probe then wrongly reports "confirmed
// empty, 0 total, fully known" even though the account may have many
// inline-only policies this probe structurally cannot see. The
// availability fetcher's whole POINT is a cheap lower bound: it must never
// claim IsTruncated=false, regardless of what ListPolicies itself says,
// because it deliberately never checks group-inline coverage.
// ---------------------------------------------------------------------------

func TestPolicyAvailabilityFetcher_InlineOnlyAccount_NeverConfirmedEmpty(t *testing.T) {
	fetcher := resource.GetAvailabilityFetcher("policy")
	if fetcher == nil {
		t.Fatal("no availability fetcher registered for \"policy\"")
	}

	// Zero managed policies; groups DO have inline policies (this cheap
	// probe never checks — failListGroupPolicies pins that it must not).
	stub := &inlineSweepIAM{
		t:                     t,
		groups:                newInlineSweepGroups(3),
		failListGroupPolicies: true,
		managedPolicies:       nil,
	}
	clients := &awsclient.ServiceClients{IAM: stub}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := fetcher(ctx, clients, "")
	if err != nil {
		t.Fatalf("registered \"policy\" availability fetcher returned an error: %v", err)
	}
	if len(result.Resources) != 0 {
		t.Fatalf("precondition: expected 0 resources from the zero-managed-policies fixture, got %d", len(result.Resources))
	}
	if result.Pagination == nil || !result.Pagination.IsTruncated {
		t.Errorf("policy availability fetcher on a zero-managed/inline-only account reports IsTruncated=%v (Pagination=%+v) — want true (lower-bound-only semantics): this probe never checks group-inline policies, so it can never confirm a true zero total, and the main menu must render \"0+\"/navigable, not a wrongly-confirmed exact \"0\"", result.Pagination != nil && result.Pagination.IsTruncated, result.Pagination)
	}
}

// ---------------------------------------------------------------------------
// C8 (self-review) — fetchInlineGroupPolicies must surface ctx cancellation
// honestly: a deadline expiring mid-sweep must yield a composite error
// naming the unswept remainder, never nil-error-with-partial-rows. Traced
// precisely: fetchInlineGroupPolicies (internal/aws/iam_policies.go)
// discards ForEachParallel's own return value (`_ =
// ForEachParallel(ctx, n, ..., func(i int) {...})`), and groupFailures is
// only appended to by a group's OWN ListGroupPolicies call actually
// failing — a group whose turn never comes before the ctx deadline expires
// (never scheduled at all under bounded concurrency) contributes NOTHING
// to groupFailures. AggregateFailures returns nil when len(failures)==0
// (partial_errors.go), so a ctx that expires before ANY group's call
// starts failing yields (partialResources, nil) — a silent, undetectable
// partial result.
// ---------------------------------------------------------------------------

func TestFetchInlineGroupPolicies_CtxCancelledMidSweep_NamesUnsweptRemainder(t *testing.T) {
	const numGroups = 20
	const perCallSleep = 300 * time.Millisecond
	// Short enough that bounded concurrency (5 workers) schedules at most
	// the first wave before the deadline fires — most of the 20 groups
	// never even start.
	const deadline = 50 * time.Millisecond

	stub := &inlineSweepIAM{
		t:            t,
		groups:       newInlineSweepGroups(numGroups),
		perCallSleep: perCallSleep,
	}
	clients := &awsclient.ServiceClients{IAM: stub}
	fetcher := resource.GetPaginatedFetcher("policy")
	if fetcher == nil {
		t.Fatal("no paginated fetcher registered for \"policy\"")
	}

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	result, err := fetcher(ctx, clients, "")

	if err == nil {
		t.Fatalf("fetchInlineGroupPolicies with a ctx that expired mid-sweep returned a nil error — got %d resources silently, want a composite error naming the unswept remainder (never nil-error-with-partial-rows)", len(result.Resources))
	}
	msg := err.Error()
	sweptOrFailed := stub.callCount()
	unswept := numGroups - sweptOrFailed
	if unswept <= 0 {
		t.Skip("precondition not met: every group's call started before the deadline fired in this run (timing-sensitive) — cannot exercise the unswept-remainder path")
	}
	if !strings.Contains(msg, fmt.Sprintf("%d", unswept)) {
		t.Errorf("composite error %q does not name the unswept remainder (%d of %d groups never visited)", msg, unswept, numGroups)
	}
}
