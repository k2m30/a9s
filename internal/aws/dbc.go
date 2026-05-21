package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchDocDBClusters calls the DocumentDB DescribeDBClusters API and converts the
// response into a slice of generic Resource structs. This covers DocumentDB
// clusters only. The docdb SDK docstring
// (docdb@v1.48.12/api_op_DescribeDBClusters.go:14-19) instructs callers to use
// filterName=engine,Values=docdb for DocDB-only results — unfiltered behavior is
// documented as ambiguous, not engine-agnostic. Aurora + Multi-AZ clusters are
// fetched separately via FetchRDSDBClustersPage.
func FetchDocDBClusters(ctx context.Context, api DocDBDescribeDBClustersAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchDocDBClustersPage(ctx, api, token)
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

// FetchDocDBClustersPage fetches a single page of DocumentDB clusters.
func FetchDocDBClustersPage(ctx context.Context, api DocDBDescribeDBClustersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &docdb.DescribeDBClustersInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*docdb.DescribeDBClustersOutput, error) {
		return api.DescribeDBClusters(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching DocumentDB clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range output.DBClusters {
		clusterID := ""
		if cluster.DBClusterIdentifier != nil {
			clusterID = *cluster.DBClusterIdentifier
		}

		engineVersion := ""
		if cluster.EngineVersion != nil {
			engineVersion = *cluster.EngineVersion
		}

		instances := fmt.Sprintf("%d", len(cluster.DBClusterMembers))

		endpoint := ""
		if cluster.Endpoint != nil {
			endpoint = *cluster.Endpoint
		}

		// has_writer: "true" if at least one member has IsClusterWriter == true.
		// writer_count: number of members with IsClusterWriter == true (healthy = 1).
		hasWriter := "false"
		writerCount := 0
		for _, m := range cluster.DBClusterMembers {
			if m.IsClusterWriter != nil && *m.IsClusterWriter {
				hasWriter = "true"
				writerCount++
			}
		}

		deletionProtection := "true"
		if cluster.DeletionProtection != nil && !*cluster.DeletionProtection {
			deletionProtection = "false"
		}

		storageEncrypted := "true"
		if cluster.StorageEncrypted != nil && !*cluster.StorageEncrypted {
			storageEncrypted = "false"
		}

		backupRetentionPeriod := "0"
		if cluster.BackupRetentionPeriod != nil {
			backupRetentionPeriod = fmt.Sprintf("%d", *cluster.BackupRetentionPeriod)
		}

		findings := computeDBCFindings(cluster)
		statusPhrase := phraseFromFindings(findings)

		r := resource.Resource{
			ID:       clusterID,
			Name:     clusterID,
			Findings: findings,
			Fields: map[string]string{
				"cluster_id":              clusterID,
				"engine_version":          engineVersion,
				"status":                  statusPhrase,
				"instances":               instances,
				"endpoint":                endpoint,
				"arn":                     aws.ToString(cluster.DBClusterArn),
				"has_writer":              hasWriter,
				"writer_count":            strconv.Itoa(writerCount),
				"deletion_protection":     deletionProtection,
				"storage_encrypted":       storageEncrypted,
				"backup_retention_period": backupRetentionPeriod,
			},
			RawStruct: cluster,
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

// transitionalDBCStatusSet contains DocumentDB cluster statuses that indicate a
// transitional (Warning) state. These show a ": in progress" suffix.
var transitionalDBCStatusSet = map[string]struct{}{
	"creating": {}, "modifying": {}, "backing-up": {}, "maintenance": {},
	"upgrading": {}, "starting": {}, "stopping": {}, "resetting-master-credentials": {},
	"renaming": {},
}

// countWriters returns the number of DBClusterMembers with IsClusterWriter == true.
func countWriters(members []docdbtypes.DBClusterMember) int {
	n := 0
	for _, m := range members {
		if m.IsClusterWriter != nil && *m.IsClusterWriter {
			n++
		}
	}
	return n
}

// computeDBCFindings returns []domain.Finding for a DocumentDB cluster.
func computeDBCFindings(cluster docdbtypes.DBCluster) []domain.Finding {
	status := aws.ToString(cluster.Status)

	// Broken statuses — first match wins; no warning stacking.
	brokenCode := map[string]domain.FindingCode{
		"failed":                              CodeDBCFailed,
		"inaccessible-encryption-credentials": CodeDBCEncryptionKeyUnreachable,
		"incompatible-parameters":             CodeDBCIncompatibleParameters,
	}
	brokenPhrase := map[string]string{
		"failed":                              "failed: cluster operation",
		"inaccessible-encryption-credentials": "encryption key unreachable",
		"incompatible-parameters":             "parameter group incompatible",
	}
	if code, ok := brokenCode[status]; ok {
		return []domain.Finding{{Code: code, Phrase: brokenPhrase[status], Severity: domain.SevBroken, Source: "wave1"}}
	}

	// No writer on an available cluster — reads only (Broken; beats warnings).
	if status == "available" && countWriters(cluster.DBClusterMembers) == 0 {
		return []domain.Finding{{Code: CodeDBCNoWriter, Phrase: "no writer: reads only", Severity: domain.SevBroken, Source: "wave1"}}
	}

	// Transitional statuses.
	if _, ok := transitionalDBCStatusSet[status]; ok {
		phrase := status + ": in progress"
		return []domain.Finding{{Code: CodeDBCTransitional, Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"}}
	}

	// Healthy available — collect Wave-1 warnings in spec §4 table order.
	if status == "available" {
		var findings []domain.Finding
		if cluster.DeletionProtection != nil && !*cluster.DeletionProtection {
			findings = append(findings, domain.Finding{Code: CodeDBCDeletionProtectionOff, Phrase: "delete-protection off", Severity: domain.SevWarn, Source: "wave1"})
		}
		if cluster.StorageEncrypted != nil && !*cluster.StorageEncrypted {
			findings = append(findings, domain.Finding{Code: CodeDBCNotEncryptedAtRest, Phrase: "not encrypted at rest", Severity: domain.SevWarn, Source: "wave1"})
		}
		if cluster.BackupRetentionPeriod != nil && *cluster.BackupRetentionPeriod == 0 {
			findings = append(findings, domain.Finding{Code: CodeDBCNoAutomatedBackups, Phrase: "no automated backups", Severity: domain.SevWarn, Source: "wave1"})
		}
		return findings
	}

	// Unknown status — bare keyword passthrough (future-proof for new AWS statuses).
	return []domain.Finding{{Code: CodeDBCTransitional, Phrase: status, Severity: domain.SevWarn, Source: "wave1"}}
}

// dedupResourcesByID returns rs with duplicate Resource.ID entries removed,
// keeping the first occurrence. Used by the dbc and dbc-snap fetchers where
// the DocDB and RDS SDKs both return overlapping rows for the same cluster /
// snapshot (AS-145, verified live: the DocDB DescribeDBClusters endpoint
// returns aurora-postgresql clusters too). DocDB-side rows are appended first
// at the call sites, so first-occurrence wins keeps the docdb-side row.
//
// Engine-filter at source was considered and rejected: it goes silently stale
// the moment AWS adds a new docdb engine variant or new aurora flavor, whereas
// dedup-by-ID is symmetric across both fetchers and robust to SDK drift.
func dedupResourcesByID(rs []resource.Resource) []resource.Resource {
	if len(rs) < 2 {
		return rs
	}
	seen := make(map[string]struct{}, len(rs))
	out := make([]resource.Resource, 0, len(rs))
	for _, r := range rs {
		if _, dup := seen[r.ID]; dup {
			continue
		}
		seen[r.ID] = struct{}{}
		out = append(out, r)
	}
	return out
}
