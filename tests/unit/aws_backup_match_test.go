package unit

import (
	"testing"

	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

func TestBackupSelectionCovers_NotResources(t *testing.T) {
	cases := []struct {
		name         string
		resources    []string
		notResources []string
		targetARN    string
		want         bool
	}{
		{
			name:      "resources-only match returns true",
			resources: []string{"arn:aws:dynamodb:*:*:table/*"},
			targetARN: "arn:aws:dynamodb:us-east-1:123:table/orders",
			want:      true,
		},
		{
			name:         "NotResources exact match excludes target",
			resources:    []string{"arn:aws:dynamodb:*:*:table/*"},
			notResources: []string{"arn:aws:dynamodb:us-east-1:123:table/audit-log"},
			targetARN:    "arn:aws:dynamodb:us-east-1:123:table/audit-log",
			want:         false,
		},
		{
			name:         "NotResources wildcard excludes matching target",
			resources:    []string{"arn:aws:s3:::*"},
			notResources: []string{"arn:aws:s3:::quarantine-*"},
			targetARN:    "arn:aws:s3:::quarantine-x",
			want:         false,
		},
		{
			name:         "NotResources defined but non-matching allows coverage",
			resources:    []string{"arn:aws:s3:::*"},
			notResources: []string{"arn:aws:s3:::quarantine-*"},
			targetARN:    "arn:aws:s3:::prod",
			want:         true,
		},
		{
			name:      "empty selection takes every resource",
			targetARN: "arn:aws:s3:::any-bucket",
			want:      true,
		},
		{
			name:      "empty targetARN never matches a pattern",
			resources: []string{"arn:aws:s3:::*"},
			targetARN: "",
			want:      false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel := backuptypes.BackupSelection{Resources: tc.resources, NotResources: tc.notResources}
			if got := awsclient.BackupSelectionCovers(sel, tc.targetARN, nil); got != tc.want {
				t.Errorf("BackupSelectionCovers(%v minus %v, %q) = %v, want %v",
					tc.resources, tc.notResources, tc.targetARN, got, tc.want)
			}
		})
	}
}
