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
			[]string{"i-0abc: " + apiErrWithMessage("RequestLimitExceeded", "Request limit exceeded").Error()}, 3),
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
			want := "availability ec2: " + awsclient.CauseInRegion(tc.err, "us-east-1")
			if !strings.Contains(lines[0], want) {
				t.Errorf("error-history line %q, want it to carry %q — one shape for every failed call", lines[0], want)
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
