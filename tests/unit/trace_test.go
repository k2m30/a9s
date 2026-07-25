// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// trace_test.go — regression coverage for core/trace, the opt-in JSON-lines
// event stream for the detail-operation lifecycle (see core/trace/trace.go's
// package doc for the "~50 defects thousands of green tests missed" context
// this package exists to make observable). package unit_test (not unit) so
// these tests can reuse the already-allowlisted newTestControllerAndCore
// helper (app_patch_cache_intents_test.go) — required by the construction
// discipline gate (qa_controller_construction_discipline_test.go) — and
// detail_operation_test.go's detailOpSfnFake / detailOpFindTaskKind, the
// existing real-SFN-related-checker fixtures.
//
// Every Enable call here is paired with t.Cleanup(trace.Disable); none of
// these tests calls t.Parallel(), so core/trace's package-level global state
// is never touched concurrently by two tests (Go never schedules two
// non-parallel tests at once).
package unit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/trace"
)

// TestTrace_DisabledByDefault_EmitIsNoOp pins both halves of "zero-cost when
// off": Emit is a no-op before any Enable call ever ran, AND after a
// previously-installed sink is explicitly disabled — a stale sink reference
// must never receive a write once Disable (or Enable(nil)) has run.
func TestTrace_DisabledByDefault_EmitIsNoOp(t *testing.T) {
	trace.Disable()
	if trace.Enabled() {
		t.Fatal("trace.Enabled() = true with no Enable call ever made")
	}
	trace.Emit(trace.Event{Kind: trace.KindAWSCall, API: "should-not-be-written"})

	var buf bytes.Buffer
	trace.Enable(&buf)
	trace.Disable()
	t.Cleanup(trace.Disable)
	if trace.Enabled() {
		t.Fatal("trace.Enabled() = true immediately after Disable")
	}
	trace.Emit(trace.Event{Kind: trace.KindFold, EventType: "should-not-be-written-either"})

	if buf.Len() != 0 {
		t.Errorf("buf.Len() = %d, want 0 — Emit must no-op once disabled, even against a sink that was installed earlier in the same test", buf.Len())
	}
}

// TestTrace_EnabledToBuffer_DetailOpenEmitsBeginEventWithRightOperationID pins
// that a real Core.BeginDetailOperation call — the detail-view open/refresh
// entry point — emits exactly one KindDetailOpBegin event carrying the
// REAL minted operation ID (not a placeholder/zero value) plus the resource
// type and ID that were opened.
func TestTrace_EnabledToBuffer_DetailOpenEmitsBeginEventWithRightOperationID(t *testing.T) {
	_, core := newTestControllerAndCore(t)

	var buf bytes.Buffer
	trace.Enable(&buf)
	t.Cleanup(trace.Disable)

	op, _ := core.BeginDetailOperation("ec2", resource.Resource{ID: "i-traceopen0001"}, false)

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("BeginDetailOperation emitted no trace line while tracing was enabled")
	}
	var ev trace.Event
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatalf("emitted line is not valid JSON: %v\nline: %s", err, line)
	}
	if ev.Kind != trace.KindDetailOpBegin {
		t.Errorf("Kind = %q, want %q", ev.Kind, trace.KindDetailOpBegin)
	}
	if ev.OperationID != uint64(op.ID) {
		t.Errorf("OperationID = %d, want %d (the real op.ID BeginDetailOperation minted)", ev.OperationID, op.ID)
	}
	if ev.ResourceType != "ec2" {
		t.Errorf("ResourceType = %q, want %q", ev.ResourceType, "ec2")
	}
	if ev.ResourceID != "i-traceopen0001" {
		t.Errorf("ResourceID = %q, want %q", ev.ResourceID, "i-traceopen0001")
	}
}

// TestTrace_SupersededFold_EmitsRejectionNamingBothOperations pins
// Controller.traceFoldAcceptance's rejection path: a result stamped with an
// operation ID that a later BeginDetailOperation call has since superseded
// must be traced as Accepted=false, with Reason naming BOTH the superseded
// operation (the event's own OperationID) and the currently active one — the
// exact pair a human reading a trace needs to diagnose a "stale result
// almost folded" defect.
func TestTrace_SupersededFold_EmitsRejectionNamingBothOperations(t *testing.T) {
	ctrl, core := newTestControllerAndCore(t)

	op1, _ := core.BeginDetailOperation("ec2", resource.Resource{ID: "i-stale0001"}, false)
	op2, _ := core.BeginDetailOperation("ec2", resource.Resource{ID: "i-active0001"}, false)

	var buf bytes.Buffer
	trace.Enable(&buf)
	t.Cleanup(trace.Disable)

	ctrl.Handle(messages.EnrichDetailResult{
		ResourceType: "ec2",
		ResourceID:   "i-stale0001",
		OperationID:  op1.ID,
	})

	lines := nonEmptyTraceLines(t, buf.String())
	if len(lines) != 1 {
		t.Fatalf("got %d trace line(s) for one superseded EnrichDetailResult, want 1: %v", len(lines), lines)
	}
	var ev trace.Event
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("emitted line is not valid JSON: %v\nline: %s", err, lines[0])
	}
	if ev.Kind != trace.KindFold {
		t.Errorf("Kind = %q, want %q", ev.Kind, trace.KindFold)
	}
	if ev.EventType != "EnrichDetailResult" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "EnrichDetailResult")
	}
	if ev.Accepted {
		t.Error("Accepted = true, want false — op1 was superseded by op2 before Handle ran")
	}
	if ev.OperationID != uint64(op1.ID) {
		t.Errorf("OperationID = %d, want %d (the superseded operation the event itself was stamped with)", ev.OperationID, op1.ID)
	}
	wantOp1 := fmt.Sprintf("%d", op1.ID)
	wantOp2 := fmt.Sprintf("%d", op2.ID)
	if !strings.Contains(ev.Reason, wantOp1) || !strings.Contains(ev.Reason, wantOp2) {
		t.Errorf("Reason = %q, want it to name both the superseded operation %s and the active operation %s", ev.Reason, wantOp1, wantOp2)
	}
}

// TestTrace_ConcurrentRelatedFanout_ProducesWellFormedNonInterleavedJSONLines
// drives the REAL production related-checker fan-out (Core.ExecuteTask →
// runtime.runRelatedCheckers' bounded MaxConcurrentProbes goroutines →
// RunRelatedDef → the genuine checkSFNRole/checkSFNKMS/checkSFNLambda
// checkers, not a synthetic loop of raw goroutines calling trace.Emit
// directly) under a blocked fake SFN client, forcing real concurrent
// DescribeStateMachine callers to race on the same coalescing key. Must be
// run with -race: trace.Emit's sink write is meant to be safe for exactly
// this concurrent-caller shape (see core/trace/trace.go's Emit doc comment).
//
// Every resulting line must still parse as one complete, well-formed JSON
// object — the mechanical proof that trace.Emit's internal mutex actually
// serializes concurrent writers rather than merely being expected to.
func TestTrace_ConcurrentRelatedFanout_ProducesWellFormedNonInterleavedJSONLines(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:trace-fanout"
	fake := &detailOpSfnFake{describeBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingSFN(fake)

	_, core := newTestControllerAndCore(t)
	core.Session().Clients = &awsclient.ServiceClients{SFN: decorated}

	var buf bytes.Buffer
	trace.Enable(&buf)
	t.Cleanup(trace.Disable)

	op, tasks := core.BeginDetailOperation("sfn", resource.Resource{
		ID:     "trace-fanout",
		Fields: map[string]string{"arn": arn},
	}, false)
	relatedTask := detailOpFindTaskKind(tasks, runtime.KindRelatedCheck)
	if relatedTask == nil {
		t.Fatal(`BeginDetailOperation("sfn", ...) returned no KindRelatedCheck task — "sfn" must have registered Related defs for this test to drive the real fan-out`)
	}

	type execResult struct {
		ev  messages.Event
		err error
	}
	done := make(chan execResult, 1)
	go func() {
		ev, err := core.ExecuteTask(context.Background(), *relatedTask)
		done <- execResult{ev, err}
	}()

	time.Sleep(20 * time.Millisecond)
	close(fake.describeBlock)

	var res execResult
	select {
	case res = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteTask(KindRelatedCheck) did not return within 5s — the real related-checker fan-out may have deadlocked")
	}
	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if _, ok := res.ev.(messages.RelatedCheckBatch); !ok {
		t.Fatalf("ExecuteTask(KindRelatedCheck) returned %T, want messages.RelatedCheckBatch", res.ev)
	}

	// checkSFNRole/checkSFNKMS/checkSFNLambda all read the identical
	// res.Fields["arn"] and call sfnDescribe concurrently under
	// runRelatedCheckers' fan-out, so exactly one of the three reaches the
	// inner fake and the other two join the in-flight singleflight call —
	// the real, non-synthetic concurrency this test exists to drive.
	if got := fake.describeCalls.Load(); got != 1 {
		t.Errorf("DescribeStateMachine reached the inner fake %d times, want 1 (role/kms/lambda checkers must share one coalesced call)", got)
	}

	lines := nonEmptyTraceLines(t, buf.String())
	if len(lines) == 0 {
		t.Fatal("no trace lines emitted for the related-checker fan-out")
	}
	var executed, served int
	for i, line := range lines {
		var ev trace.Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("line %d is not well-formed JSON (possible interleaved concurrent write): %v\nline: %q", i, err, line)
		}
		if ev.Kind != trace.KindAWSCall || ev.API != "sfn.DescribeStateMachine" {
			continue
		}
		if ev.OperationID != uint64(op.ID) {
			t.Errorf("line %d: OperationID = %d, want %d", i, ev.OperationID, op.ID)
		}
		switch ev.Outcome {
		case "executed":
			executed++
		case "served":
			served++
		default:
			t.Errorf("line %d: Outcome = %q, want %q or %q", i, ev.Outcome, "executed", "served")
		}
	}
	if executed != 1 || served != 2 {
		t.Errorf("aws_call trace outcomes for sfn.DescribeStateMachine = (executed=%d, served=%d), want (1, 2) — role/kms/lambda each ask, only one actually calls out", executed, served)
	}
}

// nonEmptyTraceLines splits raw (a trace sink's accumulated writes) into its
// constituent JSON lines, dropping any trailing empty line left by the final
// '\n'. Shared by every test in this file that inspects more than one
// possible line.
func nonEmptyTraceLines(t *testing.T, raw string) []string {
	t.Helper()
	raw = strings.TrimRight(raw, "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}
