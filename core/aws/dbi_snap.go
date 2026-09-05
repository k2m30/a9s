// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ComputeDBISnapStatusAndIssues computes findings for an RDS snapshot.
// Returns nil for a healthy (available + encrypted) snapshot.
//
// §0.1 precedence ladder (Broken > Warning, table order within severity):
//  1. Broken: failed
//  2. Broken: incompatible-* (keyword preserved verbatim)
//  3. Warning: creating: <pct>%
//  4. Warning: unencrypted (only when available; suppressed for broken end-states)
//
// Cross-ref signals (orphan, past-retention) are added by the Wave-1 issue enricher
// which has access to the sibling dbi cache.
func ComputeDBISnapStatusAndIssues(snap rdstypes.DBSnapshot) []domain.Finding {
	rawStatus := aws.ToString(snap.Status)

	if rawStatus == "failed" {
		return []domain.Finding{{Code: CodeDBISnapFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
	}
	if strings.HasPrefix(rawStatus, "incompatible-") {
		return []domain.Finding{{Code: CodeDBISnapIncompatible, Phrase: rawStatus, Severity: domain.SevBroken, Source: "wave1"}}
	}

	var findings []domain.Finding
	switch {
	case rawStatus == "creating":
		pct := int32(0)
		if snap.PercentProgress != nil {
			pct = *snap.PercentProgress
		}
		phrase := fmt.Sprintf("creating: %d%%", pct)
		findings = append(findings, domain.Finding{Code: CodeDBISnapCreating, Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"})
	case rawStatus != "" && rawStatus != "available":
		// Any other state AWS reports is one the snapshot cannot be restored
		// from yet — copying, pending, and whatever RDS adds next. Passing the
		// keyword through keeps a new state visible instead of silently ready.
		findings = append(findings, domain.Finding{Code: CodeDBISnapTransitional, Phrase: rawStatus, Severity: domain.SevWarn, Source: "wave1"})
	}
	if isSnapUnencrypted(snap) {
		findings = append(findings, domain.Finding{Code: CodeDBISnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1"})
	}
	return findings
}

// isSnapUnencrypted returns true when the snapshot's Encrypted field is
// explicitly false. nil (field not set) is treated as encrypted (no signal).
// Per spec §4: the unencrypted signal fires on Encrypted == false only.
func isSnapUnencrypted(snap rdstypes.DBSnapshot) bool {
	return snap.Encrypted != nil && !*snap.Encrypted
}

// FetchDBISnapshotsPage fetches a single page of RDS snapshots.
func FetchDBISnapshotsPage(ctx context.Context, api RDSDescribeDBSnapshotsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &rds.DescribeDBSnapshotsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*rds.DescribeDBSnapshotsOutput, error) {
		return api.DescribeDBSnapshots(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching RDS snapshots: %w", err)
	}

	var resources []resource.Resource

	for _, snap := range output.DBSnapshots {
		snapshotID := ""
		if snap.DBSnapshotIdentifier != nil {
			snapshotID = *snap.DBSnapshotIdentifier
		}

		dbInstance := ""
		if snap.DBInstanceIdentifier != nil {
			dbInstance = *snap.DBInstanceIdentifier
		}

		engine := ""
		if snap.Engine != nil {
			engine = *snap.Engine
		}

		snapshotType := ""
		if snap.SnapshotType != nil {
			snapshotType = *snap.SnapshotType
		}

		created := ""
		if snap.SnapshotCreateTime != nil {
			created = snap.SnapshotCreateTime.Format("2006-01-02 15:04")
		}

		snapARN := ""
		if snap.DBSnapshotArn != nil {
			snapARN = *snap.DBSnapshotArn
		}

		// encryptedStr stores the encryption state as a string field so the Color
		// func can classify unencrypted available snapshots as ColorWarning.
		// nil Encrypted is treated as encrypted (no unencrypted signal per spec §4).
		encryptedStr := "true"
		if snap.Encrypted != nil && !*snap.Encrypted {
			encryptedStr = "false"
		}

		// Compute findings per §0.1 precedence ladder.
		findings := ComputeDBISnapStatusAndIssues(snap)
		statusPhrase := domain.StatusPhrase(findings)

		r := resource.Resource{
			ID:       snapshotID,
			Name:     snapshotID,
			Findings: findings,
			Fields: map[string]string{
				"snapshot_id":   snapshotID,
				"db_instance":   dbInstance,
				"status":        statusPhrase,
				"encrypted":     encryptedStr,
				"engine":        engine,
				"snapshot_type": snapshotType,
				"created":       created,
				"arn":           snapARN,
			},
			RawStruct: snap,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.Marker != nil {
		nextToken = *output.Marker
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
