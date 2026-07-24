package unit_test

// detail_operation_test.go — lifecycle contract tests for the single
// per-detail-view identity core/runtime.DetailOperation was rebuilt around
// (Core.BeginDetailOperation / OperationID / messages.AspectDetailOp),
// replacing the old per-message RelatedCheckStarted/EnrichDetail trigger
// messages and the Generation staleness field. Core.DetailOperationTasks was
// later deleted (#261 boundary-sealing wave): BeginDetailOperation itself now
// returns (op, enrichTask, relatedTask) in one call — see Group 4.
//
// Five invariants, one per group below:
//
//  1. Acceptance ordering: a result stamped with a superseded operation is
//     dropped whole regardless of whether it arrives before or after the
//     active operation's own result — the guard compares against the
//     CURRENT active operation at receipt time, never against "the last
//     accepted" value, so delivery order cannot matter.
//  2. Rotation (profile/region switch) invalidates every in-flight
//     operation: a result stamped with a pre-rotation operation ID must
//     never fold after Session.Rotate() runs.
//  3. One-flight-per-operation: concurrent identical AWS calls made under
//     the SAME real DetailOperation.ID coalesce into one underlying call
//     (via awsclient.NewCoalescingSFN + awsclient.WithDetailOp), and two
//     separate operations — even for the identical AWS-level key — never
//     coalesce with each other.
//  4. Core.BeginDetailOperation's task-gating table: enrich/related tasks
//     are present only when the resource type has a registered
//     enricher/related defs, and Refresh=true sets
//     EnrichDetailPayload.DetailCtx.SkipCache.
//  5. Core.BeginDetailOperation is strictly monotonic across calls, and
//     Core.ActiveDetailOp always reflects the most recently begun operation.

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ---------------------------------------------------------------------------
// Group 1 — acceptance ordering invariant
// ---------------------------------------------------------------------------

// TestDetailOperation_AcceptanceOrdering_SupersededResultNeverFoldsRegardlessOfDeliveryOrder
// pins that Controller.Handle's OperationID acceptance guard (messages.IsStale
// against messages.AspectDetailOp) is a pure equality check against the
// CURRENT active operation, not a "reject only if it arrives after its
// successor" ordering check — so a stale-operation result is dropped whether
// it is delivered before or after the current operation's own result.
func TestDetailOperation_AcceptanceOrdering_SupersededResultNeverFoldsRegardlessOfDeliveryOrder(t *testing.T) {
	cases := []struct {
		name       string
		staleFirst bool
	}{
		{name: "CurrentFirst_StaleSecond", staleFirst: false},
		{name: "StaleFirst_CurrentSecond", staleFirst: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			replaceEC2Related(t, []resource.RelatedDef{
				{TargetType: "sg", DisplayName: "Security Groups", Checker: noopChecker},
			})
			c, core := newTestControllerAndCore(t)
			c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
			c.ApplyResourcesLoaded("ec2", []resource.Resource{
				{ID: "i-order0001", Type: "ec2", Name: "order-test-instance"},
			}, nil, false)
			c.Apply(app.Action{Kind: app.ActionSelect})

			staleOp := core.ActiveDetailOp()
			// A refresh begins a brand-new operation, superseding staleOp —
			// exactly what Ctrl+R does (core/app/actions_list.go's
			// handleActionRefresh).
			currentOp, _, _ := core.BeginDetailOperation("ec2", resource.Resource{ID: "i-order0001"}, true)

			staleMsg := messages.RelatedCheckResult{
				ResourceType:     "ec2",
				SourceResourceID: "i-order0001",
				DefDisplayName:   "Security Groups",
				Result: resource.RelatedCheckResult{
					TargetType:  "sg",
					Count:       999,
					ResourceIDs: []string{"sg-stale"},
				},
				OperationID: staleOp,
			}
			currentMsg := messages.RelatedCheckResult{
				ResourceType:     "ec2",
				SourceResourceID: "i-order0001",
				DefDisplayName:   "Security Groups",
				Result: resource.RelatedCheckResult{
					TargetType:  "sg",
					Count:       2,
					ResourceIDs: []string{"sg-a", "sg-b"},
				},
				OperationID: currentOp.ID,
			}

			if tc.staleFirst {
				c.Handle(staleMsg)
				c.Handle(currentMsg)
			} else {
				c.Handle(currentMsg)
				c.Handle(staleMsg)
			}

			key := runtime.RelatedCacheKey("ec2", "i-order0001")
			cached, hit := core.RelatedCacheGet(key)
			if !hit {
				t.Fatal("expected a RelatedCache hit after the current-operation result")
			}
			found := false
			for _, entry := range cached {
				if entry.Result.TargetType != "sg" {
					continue
				}
				found = true
				if entry.Result.Count != 2 {
					t.Errorf("Count = %d, want 2 (current operation) — a superseded-operation result must never fold, regardless of delivery order", entry.Result.Count)
				}
				if len(entry.Result.ResourceIDs) != 2 || entry.Result.ResourceIDs[0] != "sg-a" || entry.Result.ResourceIDs[1] != "sg-b" {
					t.Errorf("ResourceIDs = %v, want [sg-a sg-b] (current operation)", entry.Result.ResourceIDs)
				}
			}
			if !found {
				t.Fatal("no sg entry present in RelatedCache")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Group 2 — rotation invalidates an operation
// ---------------------------------------------------------------------------

// TestDetailOperation_Rotation_InvalidatesPreRotationOperation pins that
// Session.Rotate() (profile/region switch) bumps DetailOpGen, so a
// RelatedCheckResult stamped with the operation active before the rotation
// is stale after it and never populates RelatedCache — an in-flight result
// from the previous profile/region must never land in the new session.
func TestDetailOperation_Rotation_InvalidatesPreRotationOperation(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: noopChecker},
	})
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-rotate0001", Type: "ec2", Name: "rotate-test-instance"},
	}, nil, false)
	c.Apply(app.Action{Kind: app.ActionSelect})

	preRotateOp := core.ActiveDetailOp()

	core.Session().Rotate()

	if !messages.IsStale(messages.RelatedCheckResult{OperationID: preRotateOp}, core) {
		t.Fatal("expected the pre-rotation operation to be stale immediately after Rotate()")
	}

	c.Handle(messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: "i-rotate0001",
		DefDisplayName:   "Security Groups",
		Result: resource.RelatedCheckResult{
			TargetType:  "sg",
			Count:       5,
			ResourceIDs: []string{"sg-post-rotate"},
		},
		OperationID: preRotateOp,
	})

	key := runtime.RelatedCacheKey("ec2", "i-rotate0001")
	if _, hit := core.RelatedCacheGet(key); hit {
		t.Error("Rotate() must invalidate the pre-rotation DetailOperation — a result stamped with it must never populate RelatedCache")
	}
}

// ---------------------------------------------------------------------------
// Group 3 — one-flight-per-operation via NewCoalescingSFN
// ---------------------------------------------------------------------------

// detailOpSfnFake implements awsclient.SFNAPI, counting DescribeStateMachine
// calls and optionally blocking on describeBlock before returning — the same
// gated-release idiom aws_coalesce_test.go's coalesceSfnFake uses (not
// reusable here directly: this file is package unit_test, that one lives in
// package unit).
type detailOpSfnFake struct {
	describeCalls atomic.Int64
	describeBlock chan struct{}
}

func (f *detailOpSfnFake) DescribeStateMachine(_ context.Context, in *sfn.DescribeStateMachineInput, _ ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	f.describeCalls.Add(1)
	if f.describeBlock != nil {
		<-f.describeBlock
	}
	return &sfn.DescribeStateMachineOutput{
		Status:  sfntypes.StateMachineStatusActive,
		RoleArn: in.StateMachineArn,
	}, nil
}
func (f *detailOpSfnFake) ListStateMachines(_ context.Context, _ *sfn.ListStateMachinesInput, _ ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
	return &sfn.ListStateMachinesOutput{}, nil
}
func (f *detailOpSfnFake) ListExecutions(_ context.Context, _ *sfn.ListExecutionsInput, _ ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error) {
	return &sfn.ListExecutionsOutput{}, nil
}
func (f *detailOpSfnFake) GetExecutionHistory(_ context.Context, _ *sfn.GetExecutionHistoryInput, _ ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
	return &sfn.GetExecutionHistoryOutput{}, nil
}
func (f *detailOpSfnFake) ListTagsForResource(_ context.Context, _ *sfn.ListTagsForResourceInput, _ ...func(*sfn.Options)) (*sfn.ListTagsForResourceOutput, error) {
	return &sfn.ListTagsForResourceOutput{}, nil
}

var _ awsclient.SFNAPI = (*detailOpSfnFake)(nil)

// TestDetailOperation_OneFlightPerOperation_ConcurrentCallsUnderSameOpCoalesce
// ties a REAL Core.BeginDetailOperation ID (not a raw domain.Gen literal) to
// awsclient.WithDetailOp's coalescing namespace end to end: N concurrent
// identical DescribeStateMachine calls made under the same operation share
// exactly one underlying call.
func TestDetailOperation_OneFlightPerOperation_ConcurrentCallsUnderSameOpCoalesce(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	fake := &detailOpSfnFake{describeBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingSFN(fake)

	_, core := newTestControllerAndCore(t)
	op, _, _ := core.BeginDetailOperation("sfn", resource.Resource{ID: arn}, false)
	ctx := awsclient.WithDetailOp(context.Background(), op.ID)

	const n = 6
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = decorated.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)})
		}(i)
	}
	time.Sleep(20 * time.Millisecond)
	close(fake.describeBlock)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if got := fake.describeCalls.Load(); got != 1 {
		t.Errorf("DescribeStateMachine reached the inner fake %d times across %d concurrent calls under the same real DetailOperation, want 1", got, n)
	}
}

// TestDetailOperation_OneFlightPerOperation_DifferentOperationsNeverCoalesce
// pins that two REAL, successive Core.BeginDetailOperation calls (the shape
// of an initial open followed by a Ctrl+R refresh) never share a singleflight
// group, even for the identical AWS-level key — each operation's calls must
// execute independently.
func TestDetailOperation_OneFlightPerOperation_DifferentOperationsNeverCoalesce(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	fake := &detailOpSfnFake{describeBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingSFN(fake)

	_, core := newTestControllerAndCore(t)
	res := resource.Resource{ID: arn}
	op1, _, _ := core.BeginDetailOperation("sfn", res, false)
	op2, _, _ := core.BeginDetailOperation("sfn", res, true)

	if op2.ID == op1.ID {
		t.Fatal("BeginDetailOperation must return a fresh ID for the refresh — got the same ID as the initial open")
	}

	ctx1 := awsclient.WithDetailOp(context.Background(), op1.ID)
	ctx2 := awsclient.WithDetailOp(context.Background(), op2.ID)

	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errs[0] = decorated.DescribeStateMachine(ctx1, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)})
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = decorated.DescribeStateMachine(ctx2, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)})
	}()
	time.Sleep(20 * time.Millisecond)
	close(fake.describeBlock)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if got := fake.describeCalls.Load(); got != 2 {
		t.Errorf("DescribeStateMachine reached the inner fake %d times for two REAL, successive DetailOperations calling the identical key, want 2 (operations must never coalesce with each other)", got)
	}
}

// ---------------------------------------------------------------------------
// Group 4 — Core.DetailOperationTasks gating table
// ---------------------------------------------------------------------------

// TestDetailOperationTasks_GatingTable exercises every combination of
// (enricher registered, related defs registered) plus the Refresh flag,
// pinning: enrichTask is non-nil iff resource.HasDetailEnricher is true;
// relatedTask is non-nil iff resource.GetRelated is non-empty; and
// Refresh=true sets EnrichDetailPayload.DetailCtx.SkipCache=true.
func TestDetailOperationTasks_GatingTable(t *testing.T) {
	cases := []struct {
		name        string
		hasEnricher bool
		hasRelated  bool
		refresh     bool
	}{
		{name: "NeitherEnricherNorRelated", hasEnricher: false, hasRelated: false, refresh: false},
		{name: "EnricherOnly", hasEnricher: true, hasRelated: false, refresh: false},
		{name: "RelatedOnly", hasEnricher: false, hasRelated: true, refresh: false},
		{name: "Both_NoRefresh", hasEnricher: true, hasRelated: true, refresh: false},
		{name: "Both_Refresh_SetsSkipCache", hasEnricher: true, hasRelated: true, refresh: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shortName := "test-gating-" + tc.name

			if tc.hasEnricher {
				resource.SetDetailEnricherForTest(shortName, func(_ context.Context, _ any, res resource.Resource) (resource.Resource, error) {
					return res, nil
				})
				t.Cleanup(func() { resource.CleanupDetailEnricherForTest(shortName) })
			}
			if tc.hasRelated {
				resource.SetRelatedForTest(shortName, []resource.RelatedDef{
					{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
				})
				t.Cleanup(func() { resource.CleanupRelatedForTest(shortName) })
			}

			_, core := newTestControllerAndCore(t)
			res := resource.Resource{ID: "gate-" + tc.name}
			op, enrichTask, relatedTask := core.BeginDetailOperation(shortName, res, tc.refresh)

			if tc.hasEnricher {
				if enrichTask == nil {
					t.Fatal("expected a non-nil enrich task")
				}
				if enrichTask.Key.Kind != runtime.KindEnrichDetail {
					t.Errorf("enrichTask.Key.Kind = %v, want KindEnrichDetail", enrichTask.Key.Kind)
				}
				payload, ok := enrichTask.Payload.(runtime.EnrichDetailPayload)
				if !ok {
					t.Fatalf("enrichTask.Payload = %T, want runtime.EnrichDetailPayload", enrichTask.Payload)
				}
				if payload.DetailCtx == nil {
					t.Fatal("expected non-nil DetailCtx (newTestControllerAndCore's session.New() always seeds PolicyDocCache/DetailDocCache)")
				}
				if payload.DetailCtx.SkipCache != tc.refresh {
					t.Errorf("DetailCtx.SkipCache = %v, want %v (op.Refresh)", payload.DetailCtx.SkipCache, tc.refresh)
				}
				if payload.DetailCtx.OpID != op.ID {
					t.Errorf("DetailCtx.OpID = %d, want %d (op.ID)", payload.DetailCtx.OpID, op.ID)
				}
			} else if enrichTask != nil {
				t.Errorf("expected nil enrich task for a type with no registered enricher, got %+v", enrichTask)
			}

			if tc.hasRelated {
				if relatedTask == nil {
					t.Fatal("expected a non-nil related task")
				}
				if relatedTask.Key.Kind != runtime.KindRelatedCheck {
					t.Errorf("relatedTask.Key.Kind = %v, want KindRelatedCheck", relatedTask.Key.Kind)
				}
				payload, ok := relatedTask.Payload.(runtime.RelatedCheckPayload)
				if !ok {
					t.Fatalf("relatedTask.Payload = %T, want runtime.RelatedCheckPayload", relatedTask.Payload)
				}
				if payload.Op.ID != op.ID {
					t.Errorf("relatedTask payload Op.ID = %d, want %d (op.ID)", payload.Op.ID, op.ID)
				}
			} else if relatedTask != nil {
				t.Errorf("expected nil related task for a type with no registered related defs, got %+v", relatedTask)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Group 5 — Core.BeginDetailOperation monotonicity
// ---------------------------------------------------------------------------

// TestBeginDetailOperation_MonotonicallyIncreasingAcrossCalls pins that every
// call to BeginDetailOperation returns a strictly greater ID than the last —
// each open, re-open, and Ctrl+R refresh mints a fresh identity — and that
// Core.ActiveDetailOp always reflects the most recently begun operation.
func TestBeginDetailOperation_MonotonicallyIncreasingAcrossCalls(t *testing.T) {
	_, core := newTestControllerAndCore(t)
	res := resource.Resource{ID: "i-mono0001"}

	op1, _, _ := core.BeginDetailOperation("ec2", res, false)
	op2, _, _ := core.BeginDetailOperation("ec2", res, false)
	op3, _, _ := core.BeginDetailOperation("ec2", res, true)

	if op2.ID <= op1.ID {
		t.Errorf("op2.ID (%d) must be strictly greater than op1.ID (%d)", op2.ID, op1.ID)
	}
	if op3.ID <= op2.ID {
		t.Errorf("op3.ID (%d) must be strictly greater than op2.ID (%d)", op3.ID, op2.ID)
	}
	if got := core.ActiveDetailOp(); got != op3.ID {
		t.Errorf("ActiveDetailOp() = %d, want %d (the most recently begun operation)", got, op3.ID)
	}
}
