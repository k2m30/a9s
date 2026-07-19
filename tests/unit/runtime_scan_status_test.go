package unit

// runtime_scan_status_test.go — behavioral tests for Core.ScanStatus (#462:
// per-probe scan status + durations).
//
// Pinned production contract under test (core/runtime/scan_status.go):
//
//	ScanStatus() returns one ProbeStatus per resource type this session has
//	probed, sorted by ShortName. A type with no declarative Findings AND no
//	registered Wave-2 issue enricher (len(td.Findings)==0 &&
//	!HasIssueEnricher) is "skipped-no-rules" once its availability probe
//	completes cleanly. Otherwise: Err+no-resources -> "failed";
//	Truncated or (Err+resources-present) -> "partial"; else "ok". A
//	following EnrichmentChecked result folds onto the existing record:
//	Duration accumulates (availability + enrichment, summed), an
//	enrichment Err/Truncated degrades ok/skipped-no-rules to "partial" but
//	never turns "failed" into anything else, and Err is re-classified from
//	the enrichment error whenever the enrichment probe itself errored
//	(independent of whether the outcome actually changed). Session.Rotate
//	clears the whole map.
//
// newExecutorCore (runtime_executor_test.go) is reused throughout: it builds
// a demo-mode Core with real fake AWS clients wired via
// SetPreSuppliedClients + HandleClientsReady, which is what makes the
// executor-driven sweep in the first test produce real, non-zero
// Durations on fine-grained clocks (Linux/macOS); Windows only asserts
// non-negative.

import (
	"context"
	"fmt"
	stdruntime "runtime"
	"testing"
	"time"

	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// scanStatusFor returns shortName's entry from c.ScanStatus(), or false if
// no probe has recorded a status for it yet.
func scanStatusFor(c *runtime.Core, shortName string) (runtime.ProbeStatus, bool) {
	for _, st := range c.ScanStatus() {
		if st.ShortName == shortName {
			return st, true
		}
	}
	return runtime.ProbeStatus{}, false
}

// ────────────────────────────────────────────────────────────────────────────
// 1 — demo sweep acceptance test
// ────────────────────────────────────────────────────────────────────────────

// TestScanStatus_DemoSweep_OneEntryPerTypeWithDuration drives a real
// availability sweep (plus its follow-on Wave-2 enrichment) against demo
// fake clients, executing every dispatched task for real via
// Core.ExecuteTask so Duration reflects genuine wall time rather than a
// synthesized zero value. After the sweep drains, ScanStatus must carry
// exactly one entry per catalog resource type, every entry must have a
// positive Duration and a non-zero completion timestamp, and every type with
// neither a declarative Finding nor a Wave-2 issue enricher must be
// classified "skipped-no-rules".
func TestScanStatus_DemoSweep_OneEntryPerTypeWithDuration(t *testing.T) {
	c := newExecutorCore(t)

	_, tasks := c.HandleEvent(messages.AvailabilityCacheLoaded{})
	pending := tasks
	for len(pending) > 0 {
		req := pending[0]
		pending = pending[1:]
		ev, err := c.ExecuteTask(context.Background(), req)
		if err != nil || ev == nil {
			continue
		}
		_, follow := c.HandleEvent(ev)
		pending = append(pending, follow...)
	}

	statuses := c.ScanStatus()
	allTypes := resource.AllResourceTypes()
	if len(statuses) != len(allTypes) {
		t.Errorf("ScanStatus returned %d entries, want %d (one per resource type)", len(statuses), len(allTypes))
	}

	byName := make(map[string]runtime.ProbeStatus, len(statuses))
	for _, st := range statuses {
		byName[st.ShortName] = st
	}

	for _, td := range allTypes {
		st, ok := byName[td.ShortName]
		if !ok {
			t.Errorf("ScanStatus missing entry for %q", td.ShortName)
			continue
		}
		// Windows' wall-clock granularity (~1-15ms) rounds a sub-tick demo probe
		// to 0, so a strictly-positive Duration is only assertable where the clock
		// is fine-grained (Linux/macOS). ponytail: GOOS guard; drop it if the
		// executor ever gains an injectable clock.
		if stdruntime.GOOS == "windows" {
			if st.Duration < 0 {
				t.Errorf("%s: Duration = %v, want >= 0", td.ShortName, st.Duration)
			}
		} else if st.Duration <= 0 {
			t.Errorf("%s: Duration = %v, want > 0", td.ShortName, st.Duration)
		}
		if st.At.IsZero() {
			t.Errorf("%s: At is zero, want a completion timestamp", td.ShortName)
		}
		if st.Outcome == "" {
			t.Errorf("%s: Outcome is empty", td.ShortName)
		}
		wantSkipped := len(td.Findings) == 0 && !c.HasIssueEnricher(td.ShortName)
		if wantSkipped && st.Outcome != runtime.ProbeSkippedNoRules {
			t.Errorf("%s: Outcome = %q, want %q (no FindingDefs, no Wave-2 enricher)", td.ShortName, st.Outcome, runtime.ProbeSkippedNoRules)
		}
		if !wantSkipped && st.Outcome == runtime.ProbeSkippedNoRules {
			t.Errorf("%s: Outcome = skipped-no-rules, but the type has FindingDefs or a Wave-2 enricher", td.ShortName)
		}
	}

	for i := 1; i < len(statuses); i++ {
		if statuses[i-1].ShortName > statuses[i].ShortName {
			t.Errorf("ScanStatus not sorted by ShortName: %q before %q", statuses[i-1].ShortName, statuses[i].ShortName)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 2 — partial + access-denied classification
// ────────────────────────────────────────────────────────────────────────────

// TestScanStatus_PartialOutcome_AccessDenied pins the "partial + Err ==
// access-denied" outcome for a type whose availability probe returns rows
// alongside a per-item AccessDeniedException, using DynamoDB's real
// listed-but-denied fixture identifiers for realism.
//
// The event is constructed directly rather than driven through the real
// FetchDynamoDBTablesPage fetcher: that fetcher aggregates per-table
// DescribeTable failures into a single fmt.Errorf("...: %s", ...) string
// (core/aws/ddb.go's AggregateFailures call), which does not implement
// smithy.APIError — routing this scenario through the real fetcher would
// classify as "Unknown", not "access-denied". Constructing the event
// directly keeps the original smithy.APIError in the error chain, so what's
// actually under test is classifyProbeErr's access-denied branch, not the
// DynamoDB fetcher's (separate, pre-existing) error-string flattening.
func TestScanStatus_PartialOutcome_AccessDenied(t *testing.T) {
	c := newExecutorCore(t)

	deniedErr := fmt.Errorf("ddb: DescribeTable failed for 1 of 2 IDs: %s: %w",
		fixtures.WarnDDBDetailsDeniedID,
		&smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "not authorized to perform: dynamodb:DescribeTable on resource: " + fixtures.WarnDDBDetailsDeniedID,
		},
	)
	resources := []resource.Resource{
		{ID: fixtures.OrdersProdID, Name: fixtures.OrdersProdID, Type: "ddb"},
		{ID: fixtures.WarnDDBDetailsDeniedID, Name: fixtures.WarnDDBDetailsDeniedID, Type: "ddb"},
	}

	c.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "ddb",
		HasResources: true,
		Count:        len(resources),
		Resources:    resources,
		Err:          deniedErr,
		Gen:          c.AvailabilityGen(),
		Duration:     15 * time.Millisecond,
	})

	got, ok := scanStatusFor(c, "ddb")
	if !ok {
		t.Fatal("ScanStatus has no entry for ddb")
	}
	if got.Outcome != runtime.ProbePartial {
		t.Errorf("ddb Outcome = %q, want %q", got.Outcome, runtime.ProbePartial)
	}
	if got.Err != "access-denied" {
		t.Errorf("ddb Err = %q, want %q", got.Err, "access-denied")
	}
	if got.Duration != 15*time.Millisecond {
		t.Errorf("ddb Duration = %v, want 15ms", got.Duration)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 3 — outcome-mapping table
// ────────────────────────────────────────────────────────────────────────────

// TestScanStatus_OutcomeMapping_Table exercises availabilityOutcome,
// classifyProbeErr, and degradeForEnrichment directly against
// Core.HandleEvent, without any real probe execution. Each case uses its own
// synthetic resource-type name so cases never interfere with each other.
func TestScanStatus_OutcomeMapping_Table(t *testing.T) {
	accessDeniedErr := func(resourceID string) error {
		return fmt.Errorf("describe %s: %w", resourceID, &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "not authorized",
		})
	}
	throttledErr := &smithy.GenericAPIError{Code: "ThrottlingException", Message: "Rate exceeded"}
	timeoutErr := fmt.Errorf("probe context: %w", context.DeadlineExceeded)
	oneResource := []resource.Resource{{ID: "res-1", Name: "res-1", Type: "synthetic"}}

	cases := []struct {
		name        string
		drive       func(c *runtime.Core, shortName string)
		wantOutcome runtime.ProbeOutcome
		wantErr     string
		wantDur     time.Duration
	}{
		{
			name: "ok",
			drive: func(c *runtime.Core, shortName string) {
				c.HandleEvent(messages.AvailabilityChecked{
					ResourceType: shortName, HasResources: true, Resources: oneResource,
					Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
				})
			},
			wantOutcome: runtime.ProbeOK, wantErr: "", wantDur: 10 * time.Millisecond,
		},
		{
			name: "truncated_becomes_partial",
			drive: func(c *runtime.Core, shortName string) {
				c.HandleEvent(messages.AvailabilityChecked{
					ResourceType: shortName, HasResources: true, Resources: oneResource, Truncated: true,
					Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
				})
			},
			wantOutcome: runtime.ProbePartial, wantErr: "", wantDur: 10 * time.Millisecond,
		},
		{
			name: "err_with_resources_becomes_partial_access_denied",
			drive: func(c *runtime.Core, shortName string) {
				c.HandleEvent(messages.AvailabilityChecked{
					ResourceType: shortName, HasResources: true, Resources: oneResource, Err: accessDeniedErr("res-1"),
					Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
				})
			},
			wantOutcome: runtime.ProbePartial, wantErr: "access-denied", wantDur: 10 * time.Millisecond,
		},
		{
			name: "err_no_resources_becomes_failed_throttled",
			drive: func(c *runtime.Core, shortName string) {
				c.HandleEvent(messages.AvailabilityChecked{
					ResourceType: shortName, HasResources: false, Err: throttledErr,
					Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
				})
			},
			wantOutcome: runtime.ProbeFailed, wantErr: "throttled", wantDur: 10 * time.Millisecond,
		},
		{
			name: "err_no_resources_becomes_failed_timeout",
			drive: func(c *runtime.Core, shortName string) {
				c.HandleEvent(messages.AvailabilityChecked{
					ResourceType: shortName, HasResources: false, Err: timeoutErr,
					Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
				})
			},
			wantOutcome: runtime.ProbeFailed, wantErr: "timeout", wantDur: 10 * time.Millisecond,
		},
		{
			name: "enrichment_error_degrades_ok_to_partial_and_sums_duration",
			drive: func(c *runtime.Core, shortName string) {
				c.HandleEvent(messages.AvailabilityChecked{
					ResourceType: shortName, HasResources: true, Resources: oneResource,
					Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
				})
				c.HandleEvent(messages.EnrichmentChecked{
					ResourceType: shortName, Err: accessDeniedErr("res-1"),
					Gen: c.EnrichmentGen(), Duration: 5 * time.Millisecond,
				})
			},
			wantOutcome: runtime.ProbePartial, wantErr: "access-denied", wantDur: 15 * time.Millisecond,
		},
		{
			name: "enrichment_truncated_degrades_ok_to_partial_keeps_empty_err",
			drive: func(c *runtime.Core, shortName string) {
				c.HandleEvent(messages.AvailabilityChecked{
					ResourceType: shortName, HasResources: true, Resources: oneResource,
					Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
				})
				c.HandleEvent(messages.EnrichmentChecked{
					ResourceType: shortName, Truncated: true,
					Gen: c.EnrichmentGen(), Duration: 5 * time.Millisecond,
				})
			},
			wantOutcome: runtime.ProbePartial, wantErr: "", wantDur: 15 * time.Millisecond,
		},
		{
			// A Wave-1 "failed" outcome has nothing worse to degrade to and
			// passes through unchanged — but Err is still re-classified from
			// whichever probe most recently errored, independent of the
			// outcome decision, so the enrichment error's class (not the
			// original availability error's) is what must survive here.
			name: "enrichment_error_on_failed_type_keeps_failed_but_overwrites_err_class",
			drive: func(c *runtime.Core, shortName string) {
				c.HandleEvent(messages.AvailabilityChecked{
					ResourceType: shortName, HasResources: false, Err: throttledErr,
					Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
				})
				c.HandleEvent(messages.EnrichmentChecked{
					ResourceType: shortName, Err: accessDeniedErr("res-1"),
					Gen: c.EnrichmentGen(), Duration: 5 * time.Millisecond,
				})
			},
			wantOutcome: runtime.ProbeFailed, wantErr: "access-denied", wantDur: 15 * time.Millisecond,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newExecutorCore(t)
			shortName := fmt.Sprintf("scan-status-case-%d", i)
			tc.drive(c, shortName)

			got, ok := scanStatusFor(c, shortName)
			if !ok {
				t.Fatalf("ScanStatus has no entry for %q", shortName)
			}
			if got.Outcome != tc.wantOutcome {
				t.Errorf("Outcome = %q, want %q", got.Outcome, tc.wantOutcome)
			}
			if got.Err != tc.wantErr {
				t.Errorf("Err = %q, want %q", got.Err, tc.wantErr)
			}
			if got.Duration != tc.wantDur {
				t.Errorf("Duration = %v, want %v", got.Duration, tc.wantDur)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 4 — reset on profile/region rotate
// ────────────────────────────────────────────────────────────────────────────

// TestScanStatus_ResetOnRotate pins Session.Rotate clearing ScanStatus: a
// profile/region switch must not leak the previous pair's scan-status
// records into the next session.
func TestScanStatus_ResetOnRotate(t *testing.T) {
	c := newExecutorCore(t)

	c.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Resources:    []resource.Resource{{ID: "i-0123456789abcdef0", Name: "i-0123456789abcdef0", Type: "ec2"}},
		Gen:          c.AvailabilityGen(),
		Duration:     10 * time.Millisecond,
	})

	if len(c.ScanStatus()) == 0 {
		t.Fatal("precondition: ScanStatus is empty before Rotate — nothing to reset")
	}

	c.Session().Rotate()

	if got := c.ScanStatus(); len(got) != 0 {
		t.Errorf("ScanStatus after Rotate() = %v, want empty — a profile/region switch must clear the prior pair's scan status", got)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 5 — enrichment rerun without a fresh availability probe
// ────────────────────────────────────────────────────────────────────────────

// TestScanStatus_EnrichmentRerun_ReflectsLatestProbe pins the intended
// contract for a type re-enriched (list re-open, Ctrl+R re-enrich) without
// an intervening AvailabilityChecked: each EnrichmentChecked must recompute
// the exposed Outcome/Err from the availability baseline plus THIS
// enrichment probe only — not fold onto whatever the previous
// EnrichmentChecked left behind. A stale partial/Err from an earlier failed
// enrichment must not survive a subsequent clean rerun, and Duration must
// stay availability-baseline + latest-enrichment, never accumulate every
// enrichment rerun this type has ever had.
func TestScanStatus_EnrichmentRerun_ReflectsLatestProbe(t *testing.T) {
	accessDeniedErr := func(resourceID string) error {
		return fmt.Errorf("describe %s: %w", resourceID, &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "not authorized",
		})
	}
	oneResource := []resource.Resource{{ID: "res-1", Name: "res-1", Type: "synthetic"}}

	t.Run("failed_enrichment_then_clean_rerun_recovers_ok", func(t *testing.T) {
		c := newExecutorCore(t)
		const shortName = "scan-status-rerun-recover"

		c.HandleEvent(messages.AvailabilityChecked{
			ResourceType: shortName, HasResources: true, Resources: oneResource,
			Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
		})
		c.HandleEvent(messages.EnrichmentChecked{
			ResourceType: shortName, Err: accessDeniedErr("res-1"),
			Gen: c.EnrichmentGen(), Duration: 3 * time.Millisecond,
		})
		c.HandleEvent(messages.EnrichmentChecked{
			ResourceType: shortName,
			Gen:          c.EnrichmentGen(), Duration: 4 * time.Millisecond,
		})

		got, ok := scanStatusFor(c, shortName)
		if !ok {
			t.Fatalf("ScanStatus has no entry for %q", shortName)
		}
		if got.Outcome != runtime.ProbeOK {
			t.Errorf("Outcome = %q, want %q — a clean rerun must clear a stale partial from an earlier failed enrichment", got.Outcome, runtime.ProbeOK)
		}
		if got.Err != "" {
			t.Errorf("Err = %q, want %q — a clean rerun must clear the stale classification from an earlier failed enrichment", got.Err, "")
		}
		wantDur := 10*time.Millisecond + 4*time.Millisecond
		if got.Duration != wantDur {
			t.Errorf("Duration = %v, want %v (availability + LATEST enrichment only, not every enrichment rerun summed)", got.Duration, wantDur)
		}
	})

	t.Run("two_clean_reruns_dont_accumulate_duration", func(t *testing.T) {
		c := newExecutorCore(t)
		const shortName = "scan-status-rerun-clean"

		c.HandleEvent(messages.AvailabilityChecked{
			ResourceType: shortName, HasResources: true, Resources: oneResource,
			Gen: c.AvailabilityGen(), Duration: 10 * time.Millisecond,
		})
		c.HandleEvent(messages.EnrichmentChecked{
			ResourceType: shortName,
			Gen:          c.EnrichmentGen(), Duration: 3 * time.Millisecond,
		})
		c.HandleEvent(messages.EnrichmentChecked{
			ResourceType: shortName,
			Gen:          c.EnrichmentGen(), Duration: 4 * time.Millisecond,
		})

		got, ok := scanStatusFor(c, shortName)
		if !ok {
			t.Fatalf("ScanStatus has no entry for %q", shortName)
		}
		if got.Outcome != runtime.ProbeOK {
			t.Errorf("Outcome = %q, want %q", got.Outcome, runtime.ProbeOK)
		}
		wantDur := 10*time.Millisecond + 4*time.Millisecond
		if got.Duration != wantDur {
			t.Errorf("Duration = %v, want %v (availability + LATEST enrichment only, not accumulated across reruns)", got.Duration, wantDur)
		}
	})
}
