package unit_test

// menu_rotation_cause_test.go — the menu row after a profile/region rotation,
// and the words a failed probe or enrichment puts on screen.
//
//   - a rotation clears EVERY per-type field of MenuState, not the subset a
//     reader happened to remember (w102);
//   - the enrichment-failure flash carries the extracted cause, never an AWS
//     error's raw %v (w103);
//   - a cause loses the "operation error <Svc>: <Op>," prefix for every error
//     class, not only for API errors (w106).

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// fillPerTypeMenuFields populates every map-typed (per-resource-type) field of
// ms with one entry, so a clear that misses a field leaves a witness behind.
func fillPerTypeMenuFields(t *testing.T, ms *app.MenuState) {
	t.Helper()
	v := reflect.ValueOf(ms).Elem()
	filled := 0
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() != reflect.Map {
			continue
		}
		m := reflect.MakeMap(f.Type())
		val := reflect.New(f.Type().Elem()).Elem()
		switch val.Kind() {
		case reflect.Int:
			val.SetInt(7)
		case reflect.Bool:
			val.SetBool(true)
		case reflect.String:
			val.SetString("seeded")
		default:
			t.Fatalf("MenuState.%s has unhandled per-type value kind %s — extend this gate", v.Type().Field(i).Name, val.Kind())
		}
		m.SetMapIndex(reflect.ValueOf("ec2"), val)
		f.Set(m)
		filled++
	}
	if filled == 0 {
		t.Fatal("no per-type map fields found on MenuState — the gate is not looking at the right type")
	}
}

// TestMenuStateClearAvailability_ResetsEveryPerTypeField is the gate: the
// rotation clear point resets every per-type field of MenuState, so a field
// added later cannot be missed by a hand-maintained list.
func TestMenuStateClearAvailability_ResetsEveryPerTypeField(t *testing.T) {
	var ms app.MenuState
	fillPerTypeMenuFields(t, &ms)

	ms.ClearAvailability()

	v := reflect.ValueOf(&ms).Elem()
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() != reflect.Map {
			continue
		}
		if f.Len() != 0 {
			t.Errorf("MenuState.%s survived ClearAvailability with %d entries — a rotation must leave no per-type state from the previous profile/region pair", v.Type().Field(i).Name, f.Len())
		}
	}
	for _, p := range []struct {
		name string
		got  int
	}{
		{"AvailChecked", ms.AvailChecked}, {"AvailTotal", ms.AvailTotal},
		{"EnrichChecked", ms.EnrichChecked}, {"EnrichTotal", ms.EnrichTotal},
	} {
		if p.got != 0 {
			t.Errorf("MenuState.%s = %d after ClearAvailability, want 0", p.name, p.got)
		}
	}
}

// TestMenuRotation_VerifiedOriginDoesNotSurvive pins the rendered claim: a type
// verified under the old profile/region pair must not still read "verified"
// under the new one — nothing has probed it there yet.
func TestMenuRotation_VerifiedOriginDoesNotSurvive(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 5, Origin: runtime.OriginVerified},
	})
	before := menuEntryByShortName(t, c, "ec2")
	if before.Origin != runtime.OriginVerified {
		t.Fatalf("precondition: ec2 Origin = %q, want %q", before.Origin, runtime.OriginVerified)
	}

	c.ApplyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})

	after := menuEntryByShortName(t, c, "ec2")
	if after.Origin == runtime.OriginVerified {
		t.Errorf("after a rotation ec2 still renders Origin=%q — the new profile/region pair has verified nothing, so the row claims a probe that never ran", after.Origin)
	}
}

// menuEntryByShortName returns the menu entry for shortName from the current
// ViewState, failing the test when it is absent.
func menuEntryByShortName(t *testing.T, c *app.Controller, shortName string) app.MenuEntry {
	t.Helper()
	body := requireMenuBody(t, c)
	for _, e := range body.Entries {
		if e.ShortName == shortName {
			return e
		}
	}
	t.Fatalf("menu entry %q not found", shortName)
	return app.MenuEntry{}
}

// bareAWSError is an unauthorized-operation error exactly as the SDK returns
// it: an operation prefix, a request id, a host id and an encoded
// authorization message, none of which an operator can act on. Values are
// synthetic.
func bareAWSError() error {
	return &smithy.OperationError{
		ServiceID:     "EC2",
		OperationName: "DescribeSnapshotAttribute",
		Err:           fmt.Errorf("https response error StatusCode: 403, RequestID: 11111111-2222-3333-4444-555555555555, HostID: qUdGZm9ja2VkaG9zdA==, api error UnauthorizedOperation: You are not authorized to perform: ec2:DescribeSnapshotAttribute. Encoded authorization failure message: bV9lbmNvZGVkX21lc3NhZ2VfYmxvYg"),
	}
}

// TestEnrichmentFailureFlash_CarriesCauseNotRawError pins the rendered flash:
// an enricher that returns a bare AWS error puts the cause on screen, never
// the error's %v with its per-call tokens.
func TestEnrichmentFailureFlash_CarriesCauseNotRawError(t *testing.T) {
	c, core := newTestControllerAndCore(t)

	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		Err:          bareAWSError(),
	})
	c.ApplyIntents(intents)

	flash := c.Snapshot().Header.Flash.Text
	if flash == "" {
		t.Fatal("an enrichment failure emitted no flash — the operator is told nothing")
	}
	for _, banned := range []string{"RequestID", "HostID", "Encoded authorization failure message", "11111111-2222-3333-4444-555555555555"} {
		if strings.Contains(flash, banned) {
			t.Errorf("flash %q carries %q — per-call transport noise no operator can act on", flash, banned)
		}
	}
	if !strings.Contains(flash, "ec2:DescribeSnapshotAttribute") {
		t.Errorf("flash %q does not name the action the role lacks", flash)
	}
}

// timeoutShapedErr / transportShapedErr are the two non-API error classes a
// probe or enricher hits: a request that ran out of time, and one that never
// reached a service endpoint.
func timeoutShapedErr() error {
	return &smithy.OperationError{
		ServiceID: "EC2", OperationName: "DescribeInstances",
		Err: fmt.Errorf("exceeded maximum number of attempts, 3, https response error StatusCode: 0, RequestID: , request send failed, Post \"https://ec2.eu-west-1.amazonaws.com/\": %w", context.DeadlineExceeded),
	}
}

func transportShapedErr() error {
	return &smithy.OperationError{
		ServiceID: "CodeArtifact", OperationName: "ListRepositories",
		Err: &url.Error{Op: "Post", URL: "https://codeartifact.eu-central-2.amazonaws.com/",
			Err: &net.DNSError{Err: "no such host", Name: "codeartifact.eu-central-2.amazonaws.com", IsNotFound: true}},
	}
}

// TestAggregateFailures_StripsOperationPrefixForEveryClass pins that the
// aggregated line loses the "operation error <Svc>: <Op>," prefix whatever the
// error class is — it was previously stripped only for API errors.
func TestAggregateFailures_StripsOperationPrefixForEveryClass(t *testing.T) {
	for name, err := range map[string]error{
		"timeout":   timeoutShapedErr(),
		"transport": transportShapedErr(),
		"api":       bareAWSError(),
	} {
		t.Run(name, func(t *testing.T) {
			agg := awsclient.AggregateFailures("ec2 FetchByIDs", []string{"i-0abc: " + err.Error()}, 1)
			if agg == nil {
				t.Fatal("AggregateFailures returned nil for one failure")
			}
			if strings.Contains(agg.Error(), "operation error") {
				t.Errorf("aggregated line %q keeps the SDK operation prefix — the operation is already named by the op label", agg.Error())
			}
		})
	}
}

// TestAggregateFailures_AllNoisePrefixNotRestored pins the fallback path: a
// reason left empty by the noise trim falls back to its own words, and those
// words no longer include the operation prefix that was already stripped.
func TestAggregateFailures_AllNoisePrefixNotRestored(t *testing.T) {
	agg := awsclient.AggregateFailures("ec2 FetchByIDs",
		[]string{"i-0abc: operation error EC2: DescribeInstances, RequestID: 11111111-2222-3333-4444-555555555555"}, 1)
	if agg == nil {
		t.Fatal("AggregateFailures returned nil for one failure")
	}
	if strings.Contains(agg.Error(), "operation error") {
		t.Errorf("aggregated line %q restored the operation prefix through the empty-cause fallback", agg.Error())
	}
}

// TestMenuStateClearAvailability_KeepsTheOperatorsOwnState pins the other side
// of the clear: what the operator typed and where they were is not a fact
// about the old profile/region pair, and a rotation leaves it alone.
func TestMenuStateClearAvailability_KeepsTheOperatorsOwnState(t *testing.T) {
	ms := app.MenuState{Filter: "sg", Cursor: 4, ScrollOffset: 2, AttentionOnly: true}
	ms.ClearAvailability()
	if ms.Filter != "sg" || ms.Cursor != 4 || ms.ScrollOffset != 2 || !ms.AttentionOnly {
		t.Errorf("ClearAvailability reset the operator's own menu state: %+v", ms)
	}
}

// TestCauseOf_ClassWordsComeFromTheClassTable pins that a timeout's cause is
// the class word the menu row's own classification uses, not the SDK's retry
// prose.
func TestCauseOf_ClassWordsComeFromTheClassTable(t *testing.T) {
	if got, want := awsclient.ErrClass(timeoutShapedErr()), "timeout"; got != want {
		t.Errorf("ErrClass(timeout) = %q, want %q", got, want)
	}
	if got, want := awsclient.CauseOf(timeoutShapedErr()), "timeout"; got != want {
		t.Errorf("CauseOf(timeout) = %q, want %q", got, want)
	}
	// INVERTED by the acceptance ruling on pass 1 (spec "After acceptance
	// pass 1" (a)): a transport failure's own words are a URL and a socket
	// address, which name the endpoint and not the failure, so the class now
	// supplies the phrase. The old assertion required "no such host" — the
	// hostname it arrives with is exactly what must not reach the screen. Do
	// not restore it.
	cause := awsclient.CauseOf(transportShapedErr())
	if strings.Contains(cause, "operation error") {
		t.Errorf("CauseOf(transport) = %q keeps the SDK operation prefix", cause)
	}
	if cause != "transport failure" {
		t.Errorf("CauseOf(transport) = %q, want the transport class's own phrase", cause)
	}
	if strings.Contains(cause, "codeartifact.eu-central-2.amazonaws.com") {
		t.Errorf("CauseOf(transport) = %q carries the endpoint address", cause)
	}
	if awsclient.CauseOf(nil) != "" {
		t.Errorf("CauseOf(nil) = %q, want empty", awsclient.CauseOf(nil))
	}
	if !errors.Is(timeoutShapedErr(), context.DeadlineExceeded) {
		t.Fatal("fixture drift: the timeout-shaped error no longer unwraps to context.DeadlineExceeded")
	}
}

// refusedTransportErr is a transport failure that is NOT a region gap: the
// endpoint resolved and the connection was refused. A DNS not-found takes the
// dedicated "service not available in region" path instead.
func refusedTransportErr() error {
	return &smithy.OperationError{
		ServiceID: "EC2", OperationName: "DescribeInstances",
		Err: &url.Error{Op: "Post", URL: "https://ec2.eu-west-1.amazonaws.com/",
			Err: &net.OpError{Op: "dial", Net: "tcp",
				Addr: &net.TCPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 443},
				Err:  errors.New("connect: connection refused")}},
	}
}

// probeAllTypes drives one probe failure per resource type through the runtime
// and applies the resulting intents, the shape a sweep in which every type
// fails the same way takes.
func probeAllTypes(t *testing.T, c *app.Controller, core *runtime.Core, err error, types []string) {
	t.Helper()
	// One more type is expected than is delivered, so the sweep stays in
	// flight: its completion emits ClearFlash, which would wipe the very
	// failure text this drives to the screen.
	core.Session().AvailTotal = len(types) + 1
	for _, shortName := range types {
		intents, _ := core.HandleEvent(messages.AvailabilityChecked{
			ResourceType: shortName,
			Err:          err,
			Gen:          core.Session().AvailabilityGen,
		})
		c.ApplyIntents(intents)
	}
}

// TestProbeFailure_RowTitleAndLogAgreeOnTheClass pins that a timed-out and an
// unreachable probe read as themselves everywhere the operator meets them: the
// row's alias column, the account-wide title when every type failed that way,
// and the flash the failure logs. Before this, both read "error" on the row
// while the log said "timeout" — two tables, two vocabularies.
func TestProbeFailure_RowTitleAndLogAgreeOnTheClass(t *testing.T) {
	for _, tc := range []struct {
		name, class, rowWord, sweepTitle string
		err                              error
	}{
		{"timeout", "timeout", "timeout", "sweep: timeout", timeoutShapedErr()},
		{"transport", "transport", "transport", "sweep: transport failure", refusedTransportErr()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := awsclient.ErrClass(tc.err); got != tc.class {
				t.Fatalf("ErrClass = %q, want %q", got, tc.class)
			}

			// One type fails: the row carries the word, the title does not.
			c, core := newTestControllerAndCore(t)
			probeAllTypes(t, c, core, tc.err, []string{"ec2"})
			if got := menuEntryByShortName(t, c, "ec2").Cause; got != tc.rowWord {
				t.Errorf("menu row cause word = %q, want %q", got, tc.rowWord)
			}
			flash := c.Snapshot().Header.Flash.Text
			if !strings.Contains(flash, tc.class) {
				t.Errorf("probe flash %q does not name the class %q the row shows", flash, tc.class)
			}
			for _, banned := range []string{"https://", "203.0.113.7", "operation error"} {
				if strings.Contains(flash, banned) {
					t.Errorf("probe flash %q carries %q — an address is not a cause", flash, banned)
				}
			}
			if cause := awsclient.CauseOf(tc.err); !strings.Contains(cause, tc.class) ||
				strings.Contains(cause, "https://") || strings.Contains(cause, "203.0.113.7") {
				t.Errorf("CauseOf = %q, want a phrase naming %q with no URL or address", cause, tc.class)
			}

			// Every type fails the same way: one phrase in the title instead.
			cAll, coreAll := newTestControllerAndCore(t)
			probeAllTypes(t, cAll, coreAll, tc.err, resource.AllShortNames())
			title := cAll.MenuFrameTitle()
			if !strings.Contains(title, tc.sweepTitle) {
				t.Errorf("frame title = %q, want it to carry %q", title, tc.sweepTitle)
			}
		})
	}
}

// TestErrClassTable_EveryClassHasARowWordAndASweepTitle is the completeness
// gate: a class added to ErrClass without a word for the row and a phrase for
// the title would render as the generic "error" on one surface and as itself
// on another.
func TestErrClassTable_EveryClassHasARowWordAndASweepTitle(t *testing.T) {
	classes := awsclient.NamedErrClasses()
	if len(classes) == 0 {
		t.Fatal("NamedErrClasses is empty — the gate is not looking at the class table")
	}
	for _, class := range classes {
		word := awsclient.RowWord(class)
		if word == "" || word == "error" {
			t.Errorf("class %q has no row word of its own (got %q) — it would read as a generic error on the menu row", class, word)
		}
		if awsclient.SweepTitleForWord(word) == "" {
			t.Errorf("class %q (row word %q) has no account-wide sweep title", class, word)
		}
	}
	if got := awsclient.RowWord(""); got != "" {
		t.Errorf("RowWord(\"\") = %q, want empty — no failure, no word", got)
	}
	if got := awsclient.RowWord("SomeUnmodeledAWSCode"); got != "error" {
		t.Errorf("RowWord(unmodeled code) = %q, want %q", got, "error")
	}
}
