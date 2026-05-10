package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("dbi-snap", []string{"snapshot_id", "db_instance", "status", "engine", "snapshot_type", "created", "arn"})

	// dbc pivot is intentionally NOT registered here: real AWS rejects
	// CreateDBSnapshot on Aurora cluster members, so an dbi-snap (DBSnapshot)
	// is structurally never associated with a DBCluster. Aurora cluster
	// snapshots live in dbc-snap (DBClusterSnapshot). A registered pivot that
	// always resolves Count=0 is dead UX — drop it.
	resource.RegisterRelated("dbi-snap", []resource.RelatedDef{
		{TargetType: "dbi", DisplayName: "DB Instances", Checker: checkDBISnapDBI, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkDBISnapKMS, NeedsTargetCache: true},
		{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDBISnapBackup},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDBISnapCTEvents, NeedsTargetCache: true},
	})

	// rdstypes.DBSnapshot does not expose a VpcId field — only DBInstanceIdentifier
	// and KmsKeyId resolve directly. The vpc pivot (when needed) is reachable via
	// the dbi cross-ref.
	resource.RegisterDefaultNavFields("dbi-snap", []resource.NavigableField{
		{FieldPath: "DBInstanceIdentifier", TargetType: "dbi"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})

	resource.RegisterPaginated("dbi-snap", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchDBISnapshotsPage(ctx, c.RDS, continuationToken)
	})
}

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
	if rawStatus == "creating" {
		pct := int32(0)
		if snap.PercentProgress != nil {
			pct = *snap.PercentProgress
		}
		phrase := fmt.Sprintf("creating: %d%%", pct)
		findings = append(findings, domain.Finding{Code: CodeDBISnapCreating, Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"})
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

// buildStatusFromIssues returns the top phrase with a (+N) suffix when there
// are multiple issues, or empty string when there are none.
func buildStatusFromIssues(issues []string) string {
	if len(issues) == 0 {
		return ""
	}
	if len(issues) == 1 {
		return issues[0]
	}
	return resource.BumpFindingSuffix(issues[0])
}

// FetchDBISnapshots calls the RDS DescribeDBSnapshots API and converts the
// response into a slice of generic Resource structs.
func FetchDBISnapshots(ctx context.Context, api RDSDescribeDBSnapshotsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchDBISnapshotsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
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
		statusPhrase := phraseFromFindings(findings)

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
