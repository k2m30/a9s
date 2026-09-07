// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// errors_one_voice_test.go — a failed call has one voice. The cause is read
// off the error's own fields, the class decides the branch, and every
// error-history line is built by one formatter.

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// apiErrWithMessage builds the chain the AWS SDK produces for a modeled API
// error: an operation error naming the service and the operation, wrapping a
// generic API error whose code and message are fields of their own. The
// request id lives in the SDK's formatted text, never in the message field.
func apiErrWithMessage(code, message string) error {
	return &smithy.OperationError{
		ServiceID:     "EC2",
		OperationName: "DescribeSnapshotAttribute",
		Err: fmt.Errorf("https response error StatusCode: 400, RequestID: 11111111-2222-3333-4444-555555555555, %w",
			&smithy.GenericAPIError{Code: code, Message: message}),
	}
}

// TestCauseOf_ReadsTheErrorsFields pins spec row 1: the cause comes from the
// error's own fields, so per-call noise the SDK prints (the request id, the
// host id, the service and operation preamble) is never in it — and a message
// that legitimately spells one of those words keeps its words.
func TestCauseOf_ReadsTheErrorsFields(t *testing.T) {
	t.Run("noise in the fields reaches none of the cause", func(t *testing.T) {
		cause := awsclient.CauseOf(apiErrWithMessage("InvalidParameterValue", "Snapshot volume size is invalid"))
		for _, banned := range []string{"RequestID", "11111111-2222", "operation error", "https response error", "DescribeSnapshotAttribute"} {
			if strings.Contains(cause, banned) {
				t.Errorf("CauseOf = %q carries %q — that is a per-call field, not a cause", cause, banned)
			}
		}
		if !strings.Contains(cause, "Snapshot volume size is invalid") {
			t.Errorf("CauseOf = %q dropped the API error's own message", cause)
		}
		if !strings.Contains(cause, "InvalidParameterValue") {
			t.Errorf("CauseOf = %q dropped the API error's own code", cause)
		}
	})

	t.Run("a message that spells RequestID keeps its words", func(t *testing.T) {
		// A real modeled message can name a parameter called RequestID. A
		// substring scan of the formatted chain truncated the message here;
		// reading the message field cannot.
		const msg = "The parameter RequestID is required for this operation"
		cause := awsclient.CauseOf(apiErrWithMessage("ValidationException", msg))
		if !strings.Contains(cause, msg) {
			t.Errorf("CauseOf = %q truncated a message whose own words contain RequestID", cause)
		}
	})

	t.Run("a denied call names the action the role lacks", func(t *testing.T) {
		cause := awsclient.CauseOf(apiErrWithMessage("AccessDenied",
			"User: arn:aws:iam::123456789012:role/example-readonly is not authorized to perform: ec2:DescribeSnapshotAttribute on resource: snap-0abc"))
		if got, want := cause, "not authorized to perform ec2:DescribeSnapshotAttribute"; got != want {
			t.Errorf("CauseOf = %q, want %q", got, want)
		}
	})
}

// TestCauseOf_EdgeCasesTheFieldsHandle pins two cases the field read has to
// answer for itself, both found by probing the rewrite: an API error whose
// code is empty is still a failure and must not render as none, and a message
// that spans lines must not carry its newlines onto a one-line surface.
func TestCauseOf_EdgeCasesTheFieldsHandle(t *testing.T) {
	if got := awsclient.ErrClass(apiErrWithMessage("", "something broke")); got == "" {
		t.Error("ErrClass of a coded-less API error is empty — an empty class means no failure at all, and the menu row would show its alias as if the call had worked")
	}
	if got := awsclient.RowWord(awsclient.ErrClass(apiErrWithMessage("", "something broke"))); got == "" {
		t.Error("a coded-less API error puts no word on the menu row")
	}

	cause := awsclient.CauseOf(apiErrWithMessage("ValidationException", "line one\nline two"))
	if strings.Contains(cause, "\n") {
		t.Errorf("CauseOf = %q carries a newline — a flash and a menu row are one line each", cause)
	}
	if !strings.Contains(cause, "line one") {
		t.Errorf("CauseOf = %q dropped the first line of the message", cause)
	}
}

// tlsFailureErr is a TLS verification failure: the endpoint resolved and the
// connection was made, and the certificate did not verify. The operator's
// action is a proxy or a clock, not a network path.
func tlsFailureErr() error {
	return &smithy.OperationError{
		ServiceID: "S3", OperationName: "ListBuckets",
		Err: &url.Error{Op: "Get", URL: "https://s3.eu-west-1.amazonaws.com/",
			Err: &tls.CertificateVerificationError{
				Err: x509.CertificateInvalidError{Reason: x509.Expired, Detail: "certificate has expired"},
			}},
	}
}

// dnsFailureErr is a DNS failure that is NOT a missing endpoint: the resolver
// itself misbehaved. The operator's action is resolution, not a missing
// service.
func dnsFailureErr() error {
	return &smithy.OperationError{
		ServiceID: "EC2", OperationName: "DescribeInstances",
		Err: &url.Error{Op: "Post", URL: "https://ec2.eu-west-1.amazonaws.com/",
			Err: &net.DNSError{Err: "server misbehaving", Name: "ec2.eu-west-1.amazonaws.com", IsTemporary: true}},
	}
}

// TestTransportClasses_SplitWhereTheActionDiffers pins spec row 5: a TLS
// failure, a refused connection and a resolver failure are three classes,
// because the operator does three different things about them. Each reads its
// own word on the row, in the account-wide title and in the cause.
func TestTransportClasses_SplitWhereTheActionDiffers(t *testing.T) {
	for _, tc := range []struct {
		name, class, rowWord string
		err                  error
	}{
		{"tls", "tls", "tls", tlsFailureErr()},
		{"dns", "dns", "dns", dnsFailureErr()},
		{"refused connection", "transport", "transport", refusedTransportErr()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := awsclient.ErrClass(tc.err); got != tc.class {
				t.Fatalf("ErrClass = %q, want %q", got, tc.class)
			}
			if got := awsclient.RowWord(tc.class); got != tc.rowWord {
				t.Errorf("RowWord(%q) = %q, want %q", tc.class, got, tc.rowWord)
			}
			if got := awsclient.SweepTitleForWord(tc.rowWord); got == "" {
				t.Errorf("class %q has no account-wide sweep title", tc.class)
			}
			cause := awsclient.CauseOf(tc.err)
			if cause == "" {
				t.Fatalf("CauseOf(%s) is empty", tc.name)
			}
			for _, banned := range []string{"https://", "operation error", "203.0.113.7", "ec2.eu-west-1.amazonaws.com"} {
				if strings.Contains(cause, banned) {
					t.Errorf("cause %q carries %q — an address is not a cause", cause, banned)
				}
			}

			c, core := newTestControllerAndCore(t)
			probeAllTypes(t, c, core, tc.err, []string{"ec2"})
			if got := menuEntryByShortName(t, c, "ec2").Cause; got != tc.rowWord {
				t.Errorf("menu row cause word = %q, want %q", got, tc.rowWord)
			}
		})
	}

	// Three classes, three causes: a shared phrase would tell the operator to
	// do the same thing about three different problems.
	seen := map[string]string{}
	for _, class := range []string{"tls", "dns", "transport"} {
		cause := awsclient.CauseForClass(class)
		if prev, dup := seen[cause]; dup {
			t.Errorf("classes %q and %q share the cause %q", prev, class, cause)
		}
		seen[cause] = class
		if n := len(strings.Fields(awsclient.RowWord(class))); n > 3 {
			t.Errorf("row word for %q is %d words, want at most 3", class, n)
		}
	}
}

// TestRegionGap_BothBranchesReadTheClass pins spec row 2: the availability
// handler's region-gap branch is the class's decision, not a second one, and
// the line it writes carries the class's phrase and the region.
func TestRegionGap_BothBranchesReadTheClass(t *testing.T) {
	if got, want := awsclient.ErrClass(regionGapErr()), awsclient.ClassRegionUnavailable; got != want {
		t.Fatalf("ErrClass(region gap) = %q, want %q", got, want)
	}

	c, core := newTestControllerAndCore(t)
	probeAllTypes(t, c, core, regionGapErr(), []string{"ec2"})
	lines := c.ErrorHistoryLines()
	if len(lines) == 0 {
		t.Fatal("a region gap logged nothing")
	}
	if !strings.Contains(lines[0], awsclient.CauseForClass(awsclient.ClassRegionUnavailable)) {
		t.Errorf("error-history line %q does not carry the class's phrase", lines[0])
	}

	// The region is named by the class, not by the branch: any other class
	// gets the same sentence without a region.
	if got := awsclient.CauseInRegion(regionGapErr(), "eu-central-2"); !strings.Contains(got, "eu-central-2") {
		t.Errorf("CauseInRegion(region gap) = %q does not name the region", got)
	}
	if got := awsclient.CauseInRegion(timeoutShapedErr(), "eu-central-2"); strings.Contains(got, "eu-central-2") {
		t.Errorf("CauseInRegion(timeout) = %q names a region — a timeout is not about the region", got)
	}
}

// TestErrorHistory_OneShapeForEveryFailedCall pins spec row 3: a partial
// result and a region gap are the same sentence — subject, then the cause the
// class supplies — instead of the three shapes the two branches and the flash
// used to build.
func TestErrorHistory_OneShapeForEveryFailedCall(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		rows []resource.Resource
	}{
		{"region gap, no rows", regionGapErr(), nil},
		{"partial result, rows present", awsclient.AggregateFailures("ec2: DescribeInstances",
			[]awsclient.Failure{awsclient.FailedCall("i-0abc", apiErrWithMessage("RequestLimitExceeded", "Request limit exceeded"))}, 3),
			[]resource.Resource{{ID: "i-0abc", Name: "web-01"}, {ID: "i-0def", Name: "web-02"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, core := newTestControllerAndCore(t)
			core.Session().AvailTotal = 2
			intents, _ := core.HandleEvent(messages.AvailabilityChecked{
				ResourceType: "ec2",
				Err:          tc.err,
				Resources:    tc.rows,
				Gen:          core.Session().AvailabilityGen,
			})
			c.ApplyIntents(intents)

			lines := c.ErrorHistoryLines()
			if len(lines) == 0 {
				t.Fatalf("%s logged nothing", tc.name)
			}
			// INVERTED for the "skipped" spec row 5: the wanted line was
			// "availability ec2: " + the cause, which for a partial-batch
			// aggregate said the type twice ("availability ec2: ec2:
			// DescribeInstances failed for ..."). The shape is still subject
			// then cause; the subject is now said once. Do not restore the
			// concatenation.
			cause := awsclient.CauseInRegion(tc.err, "us-east-1")
			if !strings.HasPrefix(lines[0][strings.Index(lines[0], "] ")+2:], "availability ec2") {
				t.Errorf("error-history line %q does not lead with its subject", lines[0])
			}
			if !strings.HasSuffix(lines[0], strings.TrimPrefix(cause, "ec2: ")) {
				t.Errorf("error-history line %q does not carry the cause %q — one shape for every failed call", lines[0], cause)
			}
			if n := strings.Count(lines[0], "ec2:"); n != 1 {
				t.Errorf("error-history line %q says the type %d times, want 1", lines[0], n)
			}
		})
	}
}

// TestCostsDrillRefusalNote_ReadsTheSharedMessage pins spec row 4: the costs
// footer note reads the API error's message through the same helper every
// other surface uses, instead of its own errors.As extraction.
func TestCostsDrillRefusalNote_ReadsTheSharedMessage(t *testing.T) {
	const msg = "Resource-level data granularity is an opt-in only feature"
	err := fmt.Errorf("%w: %w", awsclient.ErrCostsAccessDenied,
		apiErrWithMessage("AccessDeniedException", msg))

	if got := awsclient.MessageOf(err); got != msg {
		t.Errorf("MessageOf = %q, want the API error's own message %q", got, msg)
	}
	if got := awsclient.MessageOf(errors.New("plain failure")); got != "plain failure" {
		t.Errorf("MessageOf(non-API error) = %q, want its own words", got)
	}
	if got := awsclient.MessageOf(nil); got != "" {
		t.Errorf("MessageOf(nil) = %q, want empty", got)
	}
}

// TestAPIErrorHandler_SpeaksTheCause pins the acceptance reject on spec row 3:
// the API error handler built "[code] message" out of the classifier's raw
// message, so a denial put the encoded authorization blob on the flash and in
// the error log while the same denial through any other path read the cause.
// One formatter, one sentence, both surfaces.
func TestAPIErrorHandler_SpeaksTheCause(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	denial := apiErrWithMessage("UnauthorizedOperation",
		"You are not authorized to perform: ec2:DescribeSnapshotAttribute. Encoded authorization failure message: bV9lbmNvZGVkX21lc3NhZ2VfYmxvYg")

	intents, _ := core.HandleEvent(messages.APIError{ResourceType: "ec2", Err: denial})
	c.ApplyIntents(intents)

	want := awsclient.CauseOf(denial)
	flash := c.Snapshot().Header.Flash.Text
	if flash != want {
		t.Errorf("flash = %q, want the cause %q", flash, want)
	}
	lines := c.ErrorHistoryLines()
	if len(lines) == 0 {
		t.Fatal("an API error logged nothing")
	}
	if !strings.Contains(lines[0], want) {
		t.Errorf("error-history line %q does not carry the cause %q the flash shows", lines[0], want)
	}
	for _, banned := range []string{"Encoded authorization failure message", "bV9lbmNvZGVkX21lc3NhZ2VfYmxvYg", "RequestID", "11111111-2222-3333-4444-555555555555"} {
		for _, surface := range []string{flash, lines[0]} {
			if strings.Contains(surface, banned) {
				t.Errorf("%q carries %q — per-call noise no operator can act on", surface, banned)
			}
		}
	}
}

// TestProbeBanner_SpeaksTheCauseNotTheClass pins the other half of the
// acceptance reject: a row-less probe failure banner read "probe ec2: failed:
// transport", the internal outcome and class names, beside "availability ec2:
// <cause>" for the same event on the soft path.
func TestProbeBanner_SpeaksTheCauseNotTheClass(t *testing.T) {
	denial := apiErrWithMessage("UnauthorizedOperation",
		"You are not authorized to perform: ec2:DescribeSnapshotAttribute")

	c, core := newTestControllerAndCore(t)
	probeAllTypes(t, c, core, denial, []string{"ec2"})

	flash := c.Snapshot().Header.Flash.Text
	want := "availability ec2: " + awsclient.CauseOf(denial)
	if flash != want {
		t.Errorf("probe banner = %q, want %q — the same sentence the soft path logs", flash, want)
	}
	for _, banned := range []string{"failed:", "UnauthorizedOperation:"} {
		if strings.Contains(flash, banned) {
			t.Errorf("probe banner %q carries %q — an internal outcome or class name is not a cause", flash, banned)
		}
	}
}

// TestPrefetchBanner_SpeaksTheCause pins the third shape the widened sweep
// found in the same file: the hard prefetch failure flashed the error's raw
// text while the soft one, six lines below, went through the formatter.
func TestPrefetchBanner_SpeaksTheCause(t *testing.T) {
	denial := apiErrWithMessage("UnauthorizedOperation",
		"You are not authorized to perform: ec2:DescribeInstances. Encoded authorization failure message: bV9lbmNvZGVkX21lc3NhZ2VfYmxvYg")

	c, core := newTestControllerAndCore(t)
	intents, _ := core.HandleEvent(messages.AvailabilityPrefetched{
		PrefetchErr: denial,
		Gen:         core.Session().AvailabilityGen,
	})
	c.ApplyIntents(intents)

	flash := c.Snapshot().Header.Flash.Text
	if want := "availability: " + awsclient.CauseOf(denial); flash != want {
		t.Errorf("prefetch banner = %q, want %q", flash, want)
	}
	for _, banned := range []string{"Encoded authorization failure message", "RequestID", "operation error"} {
		if strings.Contains(flash, banned) {
			t.Errorf("prefetch banner %q carries %q", flash, banned)
		}
	}
}

// TestLastProbeFailure_BannerSurvivesTheSweepsOwnClear pins what acceptance
// observed: the probe that completes the sweep raises its failure banner and,
// three intents later in the same batch, the completion appends a ClearFlash
// meant for the progress message. The last type to fail was the one type whose
// failure never reached the screen.
func TestLastProbeFailure_BannerSurvivesTheSweepsOwnClear(t *testing.T) {
	denial := apiErrWithMessage("UnauthorizedOperation",
		"You are not authorized to perform: ec2:DescribeInstances")

	c, core := newTestControllerAndCore(t)
	core.Session().AvailTotal = 1
	intents, _ := core.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "ec2",
		Err:          denial,
		Gen:          core.Session().AvailabilityGen,
	})
	c.ApplyIntents(intents)

	if got := c.Snapshot().Header.Flash.Text; !strings.Contains(got, awsclient.CauseOf(denial)) {
		t.Errorf("flash = %q — the completing probe's failure banner was wiped by its own batch's clear", got)
	}
}

// TestResourceHandlers_SpeakThroughTheOneFormatter pins spec row 6: the four
// remaining sites in handlers_resources.go. Two put the raw SDK chain on the
// status bar — the same class acceptance rejected on the API error handler —
// and two build the formatter's sentence by hand, so neither could ever name
// the region a region gap is about.
func TestResourceHandlers_SpeakThroughTheOneFormatter(t *testing.T) {
	denial := apiErrWithMessage("UnauthorizedOperation",
		"You are not authorized to perform: ec2:DescribeInstances. Encoded authorization failure message: bV9lbmNvZGVkX21lc3NhZ2VfYmxvYg")
	cause := awsclient.CauseOf(denial)

	drive := func(t *testing.T, run func(*runtime.Core) []runtime.UIIntent) string {
		t.Helper()
		c, core := newTestControllerAndCore(t)
		c.ApplyIntents(run(core))
		return c.Snapshot().Header.Flash.Text
	}

	t.Run("a failed list fetch reads the cause", func(t *testing.T) {
		got := drive(t, func(core *runtime.Core) []runtime.UIIntent {
			in, _ := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{ResourceType: "ec2", Err: denial})
			return in
		})
		if want := "fetch ec2: " + cause; got != want {
			t.Errorf("flash = %q, want %q", got, want)
		}
	})

	t.Run("a failed detail enrichment reads the cause", func(t *testing.T) {
		got := drive(t, func(core *runtime.Core) []runtime.UIIntent {
			in, _ := core.HandleEnrichDetailResult(runtime.EnrichDetailResultEvent{ResourceType: "ec2", ResourceID: "i-0abc", Err: denial})
			return in
		})
		if want := "enrich ec2: " + cause; got != want {
			t.Errorf("flash = %q, want %q", got, want)
		}
	})

	t.Run("a failed lazy add reads the cause", func(t *testing.T) {
		got := drive(t, func(core *runtime.Core) []runtime.UIIntent {
			in, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
				ResourceType: "ec2", SourceResourceID: "i-0abc",
				Result:       resource.KnownRelated("sg", nil, false),
				LazyAddError: denial,
			})
			return in
		})
		if want := "related-fetch: " + cause; got != want {
			t.Errorf("flash = %q, want %q", got, want)
		}
	})

	t.Run("a failed related check reads the cause", func(t *testing.T) {
		got := drive(t, func(core *runtime.Core) []runtime.UIIntent {
			in, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
				ResourceType: "ec2", SourceResourceID: "i-0abc",
				Result: resource.ErrorRelated("sg", denial),
			})
			return in
		})
		if want := "related sg: " + cause; got != want {
			t.Errorf("flash = %q, want %q", got, want)
		}
	})

	// The two hand-built sentences were byte-identical to the formatter's for
	// an error the class says nothing extra about; the copies were only ever
	// invisible because no region-gap error had reached them. Drive one and
	// the difference shows.
	t.Run("a region gap through these sites names the region", func(t *testing.T) {
		got := drive(t, func(core *runtime.Core) []runtime.UIIntent {
			in, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
				ResourceType: "ec2", SourceResourceID: "i-0abc",
				Result: resource.ErrorRelated("sg", regionGapErr()),
			})
			return in
		})
		if !strings.Contains(got, "us-east-1") {
			t.Errorf("flash = %q does not name the region a region gap is about", got)
		}
	})
}

// TestRevealFailure_SpeaksTheCause pins the site my own round 2 sweep
// misverdicted: handlers.go:547 sits among the theme-error flashes and is not
// one — a failed reveal is a failed AWS call, and it put the raw chain on the
// status bar.
func TestRevealFailure_SpeaksTheCause(t *testing.T) {
	denial := apiErrWithMessage("AccessDeniedException",
		"User: arn:aws:iam::123456789012:role/example-readonly is not authorized to perform: secretsmanager:GetSecretValue on resource: acme-db-password")

	c, core := newTestControllerAndCore(t)
	intents, _ := core.HandleValueRevealed(runtime.ValueRevealedEvent{Err: denial})
	c.ApplyIntents(intents)

	got := c.Snapshot().Header.Flash.Text
	if want := "reveal: " + awsclient.CauseOf(denial); got != want {
		t.Errorf("flash = %q, want %q", got, want)
	}
	for _, banned := range []string{"RequestID", "operation error", "11111111-2222-3333-4444-555555555555"} {
		if strings.Contains(got, banned) {
			t.Errorf("reveal flash %q carries %q", got, banned)
		}
	}
}

// TestFailureLine_EmptySubjectLeavesNoDanglingSeparator pins what probing row
// 6 found: three call sites build their subject as a prefix plus a type, and
// the type can be empty — an unregistered fetch, a detail enrichment with no
// type on its event, a related result with no target. The sentence read
// "fetch : connection reset", a colon with nothing in front of it.
func TestFailureLine_EmptySubjectLeavesNoDanglingSeparator(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	intents, _ := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: "", Err: errors.New("connection reset"),
	})
	c.ApplyIntents(intents)

	if got := c.Snapshot().Header.Flash.Text; strings.Contains(got, " :") {
		t.Errorf("flash = %q — a subject that is nothing but its prefix must not leave a dangling separator", got)
	}
}
