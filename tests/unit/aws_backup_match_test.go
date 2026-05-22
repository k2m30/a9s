package unit

// aws_backup_match_test.go — regression tests for BackupPlanCoversARN in
// internal/aws/backup_match.go.
//
// These tests pin:
//   - NotResources exclusion: exclusion always wins over a Resources match.
//   - Whitespace trimming around CSV entries.
//
// Wildcard / regex-metachar / empty-input semantics moved with the matcher to
// internal/semantics/selector; see semantics_selector_match_arn_test.go.

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

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
