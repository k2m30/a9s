package unit

// aws_backup_match_test.go — regression tests for ARNMatches and BackupPlanCoversARN
// in internal/aws/backup_match.go.
//
// These tests pin:
//   - Wildcard expansion: '*' matches any sequence (including empty).
//   - Regex-metachar literalness: '.' in a pattern must NOT match arbitrary chars.
//   - NotResources exclusion: exclusion always wins over a Resources match.
//   - Whitespace trimming around CSV entries.
//   - Empty-pattern / empty-arn short-circuit.

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// ARNMatches
// ---------------------------------------------------------------------------

func TestARNMatches(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		arn     string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "arn:aws:dynamodb:us-east-1:123:table/orders",
			arn:     "arn:aws:dynamodb:us-east-1:123:table/orders",
			want:    true,
		},
		{
			name:    "fully wildcard matches any dynamodb table",
			pattern: "arn:aws:dynamodb:*:*:table/*",
			arn:     "arn:aws:dynamodb:us-east-1:123:table/orders",
			want:    true,
		},
		{
			name:    "service mismatch — dynamodb pattern does not match s3 ARN",
			pattern: "arn:aws:dynamodb:*:*:table/*",
			arn:     "arn:aws:s3:::bucket",
			want:    false,
		},
		{
			name:    "prefix wildcard matches correct prefix",
			pattern: "arn:aws:s3:::prod-*",
			arn:     "arn:aws:s3:::prod-logs",
			want:    true,
		},
		{
			name:    "prefix wildcard does not match wrong prefix",
			pattern: "arn:aws:s3:::prod-*",
			arn:     "arn:aws:s3:::staging-logs",
			want:    false,
		},
		{
			name:    "catch-all S3 pattern matches any bucket",
			pattern: "arn:aws:s3:::*",
			arn:     "arn:aws:s3:::any-bucket-name",
			want:    true,
		},
		{
			name:    "empty pattern returns false",
			pattern: "",
			arn:     "arn:aws:s3:::x",
			want:    false,
		},
		{
			name:    "empty arn returns false",
			pattern: "arn:aws:s3:::x",
			arn:     "",
			want:    false,
		},
		{
			name:    "no wildcard — exact only, prefix not accepted",
			pattern: "arn:aws:s3:::prod",
			arn:     "arn:aws:s3:::prod-logs",
			want:    false,
		},
		// Load-bearing regex-metachar guard: '.' in a pattern must be treated as a
		// literal dot, not as the regex '.' (match-any-char). A naive implementation
		// that only does strings.ReplaceAll("*",".*") without regexp.QuoteMeta first
		// would fail this pair.
		{
			name:    "dot in pattern is literal — different char in arn is no match",
			pattern: "arn:aws:dynamodb:us-east-1:123:table/order.items",
			arn:     "arn:aws:dynamodb:us-east-1:123:table/orderXitems",
			want:    false,
		},
		{
			name:    "dot in pattern is literal — exact same arn matches",
			pattern: "arn:aws:dynamodb:us-east-1:123:table/order.items",
			arn:     "arn:aws:dynamodb:us-east-1:123:table/order.items",
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := awsclient.ARNMatches(tc.pattern, tc.arn)
			if got != tc.want {
				t.Errorf("ARNMatches(%q, %q) = %v, want %v", tc.pattern, tc.arn, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BackupPlanCoversARN
// ---------------------------------------------------------------------------

func TestBackupPlanCoversARN(t *testing.T) {
	cases := []struct {
		name            string
		resourcesCSV    string
		notResourcesCSV string
		targetARN       string
		want            bool
	}{
		{
			name:            "resources-only match returns true",
			resourcesCSV:    "arn:aws:dynamodb:*:*:table/*",
			notResourcesCSV: "",
			targetARN:       "arn:aws:dynamodb:us-east-1:123:table/orders",
			want:            true,
		},
		{
			name:            "NotResources exact match excludes target",
			resourcesCSV:    "arn:aws:dynamodb:*:*:table/*",
			notResourcesCSV: "arn:aws:dynamodb:us-east-1:123:table/audit-log",
			targetARN:       "arn:aws:dynamodb:us-east-1:123:table/audit-log",
			want:            false,
		},
		{
			name:            "NotResources wildcard excludes matching target",
			resourcesCSV:    "arn:aws:s3:::*",
			notResourcesCSV: "arn:aws:s3:::quarantine-*",
			targetARN:       "arn:aws:s3:::quarantine-x",
			want:            false,
		},
		{
			name:            "NotResources defined but non-matching allows coverage",
			resourcesCSV:    "arn:aws:s3:::*",
			notResourcesCSV: "arn:aws:s3:::quarantine-*",
			targetARN:       "arn:aws:s3:::prod",
			want:            true,
		},
		{
			name:            "empty resources always returns false",
			resourcesCSV:    "",
			notResourcesCSV: "",
			targetARN:       "arn:aws:s3:::any-bucket",
			want:            false,
		},
		{
			name:            "empty targetARN always returns false",
			resourcesCSV:    "arn:aws:s3:::*",
			notResourcesCSV: "",
			targetARN:       "",
			want:            false,
		},
		{
			name:            "whitespace trimmed around CSV entries",
			resourcesCSV:    " arn:aws:s3:::x , arn:aws:s3:::y ",
			notResourcesCSV: "",
			targetARN:       "arn:aws:s3:::y",
			want:            true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := awsclient.BackupPlanCoversARN(tc.resourcesCSV, tc.notResourcesCSV, tc.targetARN)
			if got != tc.want {
				t.Errorf("BackupPlanCoversARN(%q, %q, %q) = %v, want %v",
					tc.resourcesCSV, tc.notResourcesCSV, tc.targetARN, got, tc.want)
			}
		})
	}
}
