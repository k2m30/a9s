package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

// TestClassifyCTVerb_SESVerifyIsWrite asserts that SES verification operations
// classify as "W" (write) despite the "Verify" prefix that would otherwise
// match the read-prefix table. SES Verify* operations create a verification
// record and trigger an outbound email — they are state-mutating writes.
//
// This test FAILS on pre-fix code (SES Verify* is classified as "R" by the
// read-prefix table's "Verify" entry) and PASSES after the coder's exact-match
// override lands in classify.go.
func TestClassifyCTVerb_SESVerifyIsWrite(t *testing.T) {
	cases := []struct {
		name      string
		eventName string
	}{
		{"email identity", "VerifyEmailIdentity"},
		{"domain identity", "VerifyDomainIdentity"},
		{"email address", "VerifyEmailAddress"},
		{"domain dkim", "VerifyDomainDkim"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ctevent.ClassifyCTVerb(tc.eventName, "Management", "AwsApiCall")
			if got != "W" {
				t.Errorf("ClassifyCTVerb(%q) = %q, want %q (SES Verify* are mutating writes)",
					tc.eventName, got, "W")
			}
		})
	}
}

// TestClassifyCTVerb_NonSESVerifyIsRead asserts that other Verify* operations
// (e.g. KMS Verify) still classify as "R" via the existing exact-match.
// This guards against the SES override being too broad (accidentally catching
// the bare "Verify" name used by KMS GenerateDataKey operations).
func TestClassifyCTVerb_NonSESVerifyIsRead(t *testing.T) {
	got := ctevent.ClassifyCTVerb("Verify", "Management", "AwsApiCall")
	if got != "R" {
		t.Errorf("ClassifyCTVerb(\"Verify\") = %q, want %q (KMS-style use-key Verify is a read)", got, "R")
	}
}
