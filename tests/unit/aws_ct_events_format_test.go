package unit

// Tests for §3 (FormatCTTimestamp) and §5 (FormatCTTarget) format helpers.
//
// FormatCTTimestamp and FormatCTTarget are NEW exports to be added by the P1 coder
// in internal/aws/ct_events.go. These tests will FAIL TO COMPILE until they are
// added. That is expected — the tests are written first (TDD).

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ===========================================================================
// §3: FormatCTTimestamp
// Format: "Jan 02 15:04:05" — fixed 15 characters, zero-padded day.
// ===========================================================================

func TestFormatCTTimestamp(t *testing.T) {
	cases := []struct {
		rfc3339 string
		want    string
	}{
		{"2026-04-07T17:00:59Z", "Apr 07 17:00:59"},
		{"2026-01-01T00:00:00Z", "Jan 01 00:00:00"},
		{"2026-12-31T23:59:59Z", "Dec 31 23:59:59"},
		{"", ""},
	}

	for _, c := range cases {
		got := awsclient.FormatCTTimestamp(c.rfc3339)
		if got != c.want {
			t.Errorf("FormatCTTimestamp(%q) = %q, want %q", c.rfc3339, got, c.want)
		}
		// Non-empty inputs must produce exactly 15 characters per §3.
		if c.rfc3339 != "" && len(got) != 15 {
			t.Errorf("FormatCTTimestamp(%q) = %q (len %d), want exactly 15 chars per §3", c.rfc3339, got, len(got))
		}
	}
}

// ===========================================================================
// §5: FormatCTTarget
// Strips ARN noise. Cross-account exception retains account ID inline.
// ===========================================================================

func TestFormatCTTarget(t *testing.T) {
	const localAccount = "123456789012"

	cases := []struct {
		rawARN string
		want   string
	}{
		// S3 global bucket ARN — no account segment, always strip.
		{"arn:aws:s3:::webapp-assets-prod", "webapp-assets-prod"},
		// IAM user — same account, strip account.
		{"arn:aws:iam::123456789012:user/alice", "user/alice"},
		// Lambda function — same account, strip account.
		{"arn:aws:lambda:us-east-1:123456789012:function:my-fn", "function:my-fn"},
		// EC2 instance — same account, strip account.
		{"arn:aws:ec2:us-east-1:123456789012:instance/i-0abc", "instance/i-0abc"},
		// Cross-account IAM role — ARN account differs from local → retain account inline.
		{"arn:aws:iam::999988887777:role/Admin", "999988887777:role/Admin"},
		// S3 cross-account bucket ARN — no account segment, always strip regardless of "cross".
		{"arn:aws:s3:::shared-bucket", "shared-bucket"},
		// Not an ARN — passthrough unchanged.
		{"not-an-arn", "not-an-arn"},
		// Empty — return empty.
		{"", ""},
	}

	for _, c := range cases {
		got := awsclient.FormatCTTarget(c.rawARN, localAccount)
		if got != c.want {
			t.Errorf("FormatCTTarget(%q, %q) = %q, want %q per §5",
				c.rawARN, localAccount, got, c.want)
		}
	}
}
