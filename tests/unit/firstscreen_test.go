// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// firstscreen_test.go — the first screen says what it is doing and why.
//
// Four contracts:
//
//  1. While the Wave-1 availability sweep runs, the menu frame title carries
//     its progress ("[verifying 12/71]") in the same slot and shape as the
//     Wave-2 "[enriching N/M]" counter, and both come from ONE formatter, so
//     the headless ViewState.FrameTitle and the TUI title cannot drift.
//  2. A probe that FAILED is distinguishable on the row from one that has not
//     run yet: the cached count stays, and the row gains a short cause word in
//     the alias column. When every probe in the sweep fails for the same
//     cause, the title says it once instead of marking every row.
//  3. A denied enrichment is one log line per type per cause — the action the
//     role lacks, how many resources it covered, one example id — never a
//     verbatim wall of AWS errors carrying request ids, host ids and encoded
//     authorization messages.
//  4. A type whose list the operator opened and fetched live counts as
//     verified on the menu, without waiting for its probe.
package unit_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// firstScreenEntry returns the root-menu entry for shortName from the
// controller's own snapshot — the rendered surface, not internal state.
func firstScreenEntry(t *testing.T, c *app.Controller, shortName string) app.MenuEntry {
	t.Helper()
	body := c.Snapshot().Body.Menu
	if body == nil {
		t.Fatal("Snapshot().Body.Menu is nil — expected the root menu screen")
	}
	for _, e := range body.Entries {
		if e.ShortName == shortName {
			return e
		}
	}
	t.Fatalf("no menu entry for %q", shortName)
	return app.MenuEntry{}
}

// ---------------------------------------------------------------------------
// Row 1 — the sweep announces itself in the title
// ---------------------------------------------------------------------------

// TestMenuTitle_SweepProgress_VerifyingCounter drives a three-type sweep
// through the controller and reads the title at each step: it must carry the
// sweep's progress while the sweep runs and drop it when the sweep is done.
func TestMenuTitle_SweepProgress_VerifyingCounter(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	types := []string{"ec2", "s3", "dbi"}
	core.Session().AvailTotal = len(types)
	core.Session().AvailChecked = 0
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuCheckProgress{Checked: 0, Total: len(types)},
	})

	if got := c.MenuFrameTitle(); !strings.Contains(got, "[verifying 0/3]") {
		t.Errorf("title at sweep start = %q, want it to carry %q — a sweep in flight must say so on the first screen",
			got, "[verifying 0/3]")
	}

	for i, rt := range types {
		c.Handle(messages.AvailabilityChecked{
			ResourceType: rt,
			HasResources: true,
			Count:        1,
			Resources:    []resource.Resource{{ID: rt + "-first-screen-1", Type: rt}},
			Gen:          core.AvailabilityGen(),
		})
		if i == len(types)-1 {
			break
		}
		want := fmt.Sprintf("[verifying %d/%d]", i+1, len(types))
		if got := c.MenuFrameTitle(); !strings.Contains(got, want) {
			t.Errorf("title after %d of %d probes = %q, want it to carry %q", i+1, len(types), got, want)
		}
		// The headless body and the TUI read the same text from one
		// formatter: the frame title must contain the body's progress verbatim.
		snap := c.Snapshot()
		if snap.FrameTitle != c.MenuFrameTitle() {
			t.Errorf("headless FrameTitle = %q but TUI MenuFrameTitle = %q — one formatter, one text",
				snap.FrameTitle, c.MenuFrameTitle())
		}
		if p := snap.Body.Menu.Progress; p == "" || !strings.Contains(snap.FrameTitle, p) {
			t.Errorf("MenuBody.Progress = %q is not carried in FrameTitle %q", p, snap.FrameTitle)
		}
	}

	if got := c.MenuFrameTitle(); strings.Contains(got, "verifying") {
		t.Errorf("title after the sweep finished = %q, want no verifying counter", got)
	}
}

// ---------------------------------------------------------------------------
// Row 2 — a failed probe says why, and says it once when it is account-wide
// ---------------------------------------------------------------------------

// TestMenuRow_FailedProbe_CarriesCauseAndKeepsCount pins that a row whose
// probe failed keeps its cached count AND gains a short cause word, so
// "not checked yet" and "checked and refused" no longer look identical.
func TestMenuRow_FailedProbe_CarriesCauseAndKeepsCount(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"access denied", &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/s is not authorized to perform: ec2:DescribeInstances",
		}, "denied"},
		{"expired session", &smithy.GenericAPIError{
			Code:    "ExpiredTokenException",
			Message: "The security token included in the request is expired",
		}, "expired"},
		{"throttled", &smithy.GenericAPIError{
			Code:    "ThrottlingException",
			Message: "Rate exceeded",
		}, "throttled"},
		{"anything else", errors.New("dial tcp: connection reset by peer"), "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, core := newTestControllerAndCore(t)
			core.Session().AvailTotal = len(resource.AllShortNames())
			c.ApplyIntents([]runtime.UIIntent{
				runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 7, Origin: runtime.OriginCache},
			})

			c.Handle(messages.AvailabilityChecked{
				ResourceType: "ec2",
				Err:          tc.err,
				Gen:          core.AvailabilityGen(),
			})

			e := firstScreenEntry(t, c, "ec2")
			if e.Cause != tc.want {
				t.Errorf("menu entry Cause = %q, want %q — a refused probe must be distinguishable from an unchecked one", e.Cause, tc.want)
			}
			if e.Availability != 7 || !e.AvailKnown {
				t.Errorf("cached count lost after a failed probe: Availability=%d AvailKnown=%v, want 7/true",
					e.Availability, e.AvailKnown)
			}
		})
	}
}

// TestMenuTitle_WholeSweepFailed_SaysCauseOnce pins the account-wide case:
// when every probe fails for the same cause the title carries it once and no
// row repeats it, instead of one mark per resource type.
func TestMenuTitle_WholeSweepFailed_SaysCauseOnce(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"denied", &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/s is not authorized to perform: ec2:DescribeInstances",
		}, "sweep: access denied"},
		{"expired", &smithy.GenericAPIError{
			Code:    "ExpiredTokenException",
			Message: "The security token included in the request is expired",
		}, "session expired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, core := newTestControllerAndCore(t)
			names := resource.AllShortNames()
			core.Session().AvailTotal = len(names)
			for _, rt := range names {
				c.Handle(messages.AvailabilityChecked{
					ResourceType: rt,
					Err:          tc.err,
					Gen:          core.AvailabilityGen(),
				})
			}

			title := c.MenuFrameTitle()
			if !strings.Contains(title, tc.want) {
				t.Errorf("title after an all-failed sweep = %q, want it to carry %q", title, tc.want)
			}
			body := c.Snapshot().Body.Menu
			if body == nil {
				t.Fatal("Snapshot().Body.Menu is nil")
			}
			for _, e := range body.Entries {
				if e.Cause != "" {
					t.Errorf("entry %q carries Cause %q, want none — the title already said it once for the whole sweep",
						e.ShortName, e.Cause)
					break
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Row 3 — a denied enrichment is one line, not a wall
// ---------------------------------------------------------------------------

// TestAggregateFailures_DeniedEnrichment_OneLinePerCause feeds 46 identical
// AccessDenied failures (the reported shape: one per resource, each carrying
// a request id and an encoded authorization message) through the one helper
// that phrases an aggregated failure, and pins the operator-readable result.
func TestAggregateFailures_DeniedEnrichment_OneLinePerCause(t *testing.T) {
	const total = 46
	failures := make([]awsclient.Failure, 0, total)
	for i := range total {
		id := fmt.Sprintf("snap-%04d", i)
		awsErr := fmt.Errorf("operation error EC2: DescribeSnapshotAttribute, "+
			"https response error StatusCode: 403, RequestID: 11111111-2222-3333-4444-5555555%05d, "+
			"api error UnauthorizedOperation: %w", i, &smithy.GenericAPIError{
			Code: "UnauthorizedOperation",
			Message: "You are not authorized to perform this operation. " +
				"User: arn:aws:sts::123456789012:assumed-role/example-readonly/session " +
				"is not authorized to perform: ec2:DescribeSnapshotAttribute because no identity-based " +
				"policy allows the ec2:DescribeSnapshotAttribute action. " +
				"Encoded authorization failure message: " + strings.Repeat("QUJDRE", 20),
		})
		failures = append(failures, awsclient.FailedCall(id, awsErr))
	}

	err := awsclient.AggregateFailures("ebs-snap-enrich DescribeSnapshotAttribute", failures, total)
	if err == nil {
		t.Fatal("AggregateFailures returned nil for 46 failures, want a composite error")
	}
	msg := err.Error()

	if strings.Contains(msg, "\n") {
		t.Errorf("aggregated failure spans more than one line:\n%s", msg)
	}
	if !strings.Contains(msg, "ec2:DescribeSnapshotAttribute") {
		t.Errorf("aggregated failure does not name the action the role lacks; got: %s", msg)
	}
	if !strings.Contains(msg, "46 of 46") {
		t.Errorf("aggregated failure does not carry how many resources it covered; got: %s", msg)
	}
	named := 0
	for i := range total {
		if strings.Contains(msg, fmt.Sprintf("snap-%04d", i)) {
			named++
		}
	}
	if named != 1 {
		t.Errorf("aggregated failure names %d resource ids, want exactly 1 example; got: %s", named, msg)
	}
	for _, banned := range []string{"RequestID", "HostID", "Encoded authorization failure message", "https response error"} {
		if strings.Contains(msg, banned) {
			t.Errorf("aggregated failure still carries %q, which no operator can act on; got: %s", banned, msg)
		}
	}
	if len(msg) > 220 {
		t.Errorf("aggregated failure is %d bytes, want a single readable line; got: %s", len(msg), msg)
	}
}

// TestAggregateFailures_DistinctCauses_OneLineEach pins that grouping is per
// cause, not "collapse everything": two different reasons stay visible, each
// with its own count and example.
func TestAggregateFailures_DistinctCauses_OneLineEach(t *testing.T) {
	failures := []awsclient.Failure{
		awsclient.FailedCall("snap-0001", context.DeadlineExceeded),
		awsclient.FailedCall("snap-0002", context.DeadlineExceeded),
		awsclient.UnusableAnswer("snap-0003", "no metadata"),
	}
	err := awsclient.AggregateFailures("ebs-snap-enrich", failures, 9)
	if err == nil {
		t.Fatal("AggregateFailures returned nil, want a composite error")
	}
	msg := err.Error()
	// "timeout" is the class's own word for a deadline; the Go error's text is
	// not read by anything.
	for _, want := range []string{"3 of 9", "timeout", "no metadata"} {
		if !strings.Contains(msg, want) {
			t.Errorf("aggregated failure missing %q; got: %s", want, msg)
		}
	}
}

// ---------------------------------------------------------------------------
// Row 4 — an opened list verifies its type
// ---------------------------------------------------------------------------

// TestMenuOrigin_FailedListFetch_DoesNotVerify is row 2's rule in the list
// lane: a fetch that came back refused, with no rows, must not mark the type
// verified and must not overwrite its cached count with a zero nobody
// observed. Found by probing row 4's own change.
func TestMenuOrigin_FailedListFetch_DoesNotVerify(t *testing.T) {
	c, _ := newTestControllerAndCore(t)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 5, Origin: runtime.OriginCache},
	})
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    nil,
		Err: &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/s is not authorized to perform: ec2:DescribeInstances",
		},
		Provenance: messages.FetchProvenanceCanonicalList,
	})
	c.Apply(app.Action{Kind: app.ActionBack})

	e := firstScreenEntry(t, c, "ec2")
	if e.Origin == runtime.OriginVerified {
		t.Error("a refused list fetch marked ec2 verified — nothing was observed")
	}
	if e.Availability != 5 {
		t.Errorf("cached count = %d after a refused list fetch, want the cached 5 kept", e.Availability)
	}
}

// TestAggregateFailures_NothingToSay_StillSaysSomething pins that a failure
// whose error carries neither a code nor a message does not aggregate into an
// empty phrase.
// A failure is recorded from the error's fields — there is no string lane —
// so the hole is probed with an error that has no fields.
func TestAggregateFailures_NothingToSay_StillSaysSomething(t *testing.T) {
	err := awsclient.AggregateFailures("op",
		[]awsclient.Failure{awsclient.FailedCall("id-1", &smithy.GenericAPIError{})}, 3)
	if err == nil {
		t.Fatal("AggregateFailures returned nil, want a composite error")
	}
	if strings.Contains(err.Error(), "IDs:  ") {
		t.Errorf("aggregated failure has an empty cause: %s", err.Error())
	}
}

// TestMenuOrigin_ListFetch_MarksTypeVerified pins that opening a list and
// fetching it live marks the type verified on the menu, without waiting for
// the sweep to reach it.
func TestMenuOrigin_ListFetch_MarksTypeVerified(t *testing.T) {
	c, _ := newTestControllerAndCore(t)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 3, Origin: runtime.OriginCache},
	})
	if got := firstScreenEntry(t, c, "ec2").Origin; got != runtime.OriginCache {
		t.Fatalf("precondition: ec2 Origin = %q, want %q", got, runtime.OriginCache)
	}

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-0firstscreen001", Type: "ec2"},
			{ID: "i-0firstscreen002", Type: "ec2"},
			{ID: "i-0firstscreen003", Type: "ec2"},
		},
		Provenance: messages.FetchProvenanceCanonicalList,
	})
	c.Apply(app.Action{Kind: app.ActionBack})

	if got := firstScreenEntry(t, c, "ec2").Origin; got != runtime.OriginVerified {
		t.Errorf("ec2 Origin after a live list fetch = %q, want %q — a type the operator just fetched is not \"not verified\"",
			got, runtime.OriginVerified)
	}
}
