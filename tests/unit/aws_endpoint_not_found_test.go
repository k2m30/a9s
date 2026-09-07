package unit

// aws_endpoint_not_found_test.go — region-gap classification (live witness
// 2026-07-14: CodeArtifact does not exist in eu-central-2; its endpoint DNS
// does not resolve). The classifier must be narrow — only DNS not-found
// qualifies — and the availability handler must render the plain-language
// plain-language "service not available" line in the `!` error log, worded
// once in the class table, with NO blocking banner.

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// sdkStyleDNSNotFound mirrors the error chain the AWS SDK produces for a
// missing regional endpoint: operation error wraps *url.Error wraps
// *net.DNSError with IsNotFound.
func sdkStyleDNSNotFound() error {
	dnsErr := &net.DNSError{
		Err:        "no such host",
		Name:       "codeartifact.eu-central-2.amazonaws.com",
		IsNotFound: true,
	}
	urlErr := &url.Error{Op: "Post", URL: "https://codeartifact.eu-central-2.amazonaws.com/v1/repositories", Err: dnsErr}
	return fmt.Errorf("operation error codeartifact: ListRepositories, request send failed, %w", urlErr)
}

// INVERTED by spec row 2 (task "errors"): the region gap is decided once, by
// ErrClass, and the separate IsEndpointNotFound predicate two branches used to
// call alongside it is gone. The assertions now read the class the branches
// read. Do not restore a second predicate — a branch and a class that agree
// only by luck is the defect this row removed. A resolver failure that is not
// a missing host is its own class now ("dns"), not a plain false.
func TestRegionGapClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"sdk-style DNS not-found chain", sdkStyleDNSNotFound(), awsclient.ClassRegionUnavailable},
		{"bare DNSError not-found", &net.DNSError{Err: "no such host", IsNotFound: true}, awsclient.ClassRegionUnavailable},
		{"DNSError without IsNotFound (flaky DNS)", &net.DNSError{Err: "server misbehaving", IsTemporary: true}, "dns"},
		{"generic error", errors.New("AccessDeniedException: not authorized"), "Unknown"},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := awsclient.ErrClass(tc.err); got != tc.want {
				t.Errorf("ErrClass(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestHandleAvailabilityChecked_RegionGapLogsPlainLanguage drives the live
// per-type availability handler with a row-less DNS-not-found failure and
// asserts: no error flash banner, and the `!` error log carries the
// plain-language region-gap line instead of transport jargon.
func TestHandleAvailabilityChecked_RegionGapLogsPlainLanguage(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, cmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "codeartifact",
		Err:          sdkStyleDNSNotFound(),
		HasResources: false,
		Count:        0,
		Gen:          m.Core().Session().AvailabilityGen,
	})

	if cmd != nil {
		for _, raw := range drainAllMessages(cmd) {
			if fm, ok := raw.(messages.Flash); ok && fm.IsError {
				t.Errorf("region gap must be error-log-only; got error FlashMsg %q", fm.Text)
			}
		}
	}

	logModel, logCmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})
	if logCmd != nil {
		if raw := logCmd(); raw != nil {
			logModel, _ = rootApplyMsg(logModel, raw)
		}
	}
	logView := stripANSI(rootViewContent(logModel))
	// The phrase moved into the class table (task menu row 4: the region gap
	// is a class of its own, and its cause is worded once so the menu row's
	// "no service" and this line cannot drift). Read it from the table rather
	// than restoring the old literal, which would pin the wording in a second
	// place — the thing the row removed.
	if want := awsclient.CauseOf(sdkStyleDNSNotFound()); !strings.Contains(logView, want) {
		t.Errorf("`!` error log must carry the plain-language region-gap line %q; got view:\n%s", want, logView)
	}
	if !strings.Contains(logView, "us-east-1") {
		t.Errorf("`!` error log must still name the region; got view:\n%s", logView)
	}
	if strings.Contains(logView, "no such host") {
		t.Errorf("`!` error log must not carry raw DNS jargon for a region gap; got view:\n%s", logView)
	}
}
