package unit

// semantics_selector_match_arn_test.go — wildcard ARN matching semantics for
// internal/semantics/selector. Migrated from aws_backup_match_test.go to keep
// the package-level test where the package lives.
//
// These tests pin:
//   - Wildcard expansion: '*' matches any sequence (including empty).
//   - Regex-metachar literalness: '.' in a pattern must NOT match arbitrary chars.
//   - Empty-pattern / empty-arn short-circuit.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/semantics/selector"
)

func TestMatchARN(t *testing.T) {
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
			got := selector.MatchARN(tc.pattern, tc.arn)
			if got != tc.want {
				t.Errorf("MatchARN(%q, %q) = %v, want %v", tc.pattern, tc.arn, got, tc.want)
			}
		})
	}
}
