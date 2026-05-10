package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// transitionalRDSDBCStatusSet contains Aurora / Multi-AZ cluster statuses that
// indicate a transitional (Warning) state.
// Mirrors transitionalDBCStatusSet for the RDS-side cluster type.
var transitionalRDSDBCStatusSet = map[string]struct{}{
	"creating": {}, "modifying": {}, "backing-up": {}, "maintenance": {},
	"upgrading": {}, "starting": {}, "stopping": {}, "resetting-master-credentials": {},
	"renaming": {},
}

// countRDSWriters returns the number of rdstypes.DBClusterMember with IsClusterWriter == true.
// Sister helper to countWriters for the RDS-side cluster member type.
func countRDSWriters(members []rdstypes.DBClusterMember) int {
	n := 0
	for _, m := range members {
		if m.IsClusterWriter != nil && *m.IsClusterWriter {
			n++
		}
	}
	return n
}

// computeRDSDBClusterStatusAndIssues returns the ordered Wave-1 findings for an
// RDS-side (Aurora / Multi-AZ) DB cluster. Algorithm mirrors computeDBCStatusAndIssues.
func computeRDSDBClusterStatusAndIssues(cluster rdstypes.DBCluster) []domain.Finding {
	status := aws.ToString(cluster.Status)

	// Broken statuses — map to specific codes.
	switch status {
	case "failed":
		return []domain.Finding{{Code: CodeDBCFailed, Phrase: "failed: cluster operation", Severity: domain.SevBroken, Source: "wave1"}}
	case "inaccessible-encryption-credentials":
		return []domain.Finding{{Code: CodeDBCEncryptionKeyUnreachable, Phrase: "encryption key unreachable", Severity: domain.SevBroken, Source: "wave1"}}
	case "incompatible-parameters":
		return []domain.Finding{{Code: CodeDBCIncompatibleParameters, Phrase: "parameter group incompatible", Severity: domain.SevBroken, Source: "wave1"}}
	}

	// No writer on an available cluster — reads only (Broken; beats warnings).
	if status == "available" && countRDSWriters(cluster.DBClusterMembers) == 0 {
		return []domain.Finding{{Code: CodeDBCNoWriter, Phrase: "no writer: reads only", Severity: domain.SevBroken, Source: "wave1"}}
	}

	// Transitional statuses.
	if _, ok := transitionalRDSDBCStatusSet[status]; ok {
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

	// Unknown status — pass through as transitional.
	if status != "" {
		return []domain.Finding{{Code: CodeDBCTransitional, Phrase: status, Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}

// FetchRDSDBClustersPage fetches a single page of Aurora + Multi-AZ DB clusters
// via the RDS SDK.
//
// Per AWS SDK docstring (rds@v1.116.3/api_op_DescribeDBClusters.go:19-28), this
// operation returns Aurora + Multi-AZ explicitly and may also return Neptune /
// DocumentDB rows. The docdb-side DescribeDBClusters docstring
// (docdb@v1.48.12/api_op_DescribeDBClusters.go:14-19) instructs callers to use
// filterName=engine,Values=docdb for DocDB-only results — unfiltered behavior is
// documented as ambiguous, not engine-agnostic.
func FetchRDSDBClustersPage(ctx context.Context, api RDSDescribeDBClustersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &rds.DescribeDBClustersInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*rds.DescribeDBClustersOutput, error) {
		return api.DescribeDBClusters(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching RDS clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range output.DBClusters {
		// Per AWS SDK docstring (rds@v1.116.3/api_op_DescribeDBClusters.go:19-28),
		// DescribeDBClusters may return Neptune and DocDB rows alongside Aurora/Multi-AZ.
		// Skip engines that are handled by their own SDK paths (docdb SDK) or are
		// not supported as dbc resource types (neptune). Use deny-list so future
		// Aurora variants (e.g. "aurora-limitless") are not accidentally dropped.
		engine := strings.ToLower(aws.ToString(cluster.Engine))
		if engine == "neptune" || engine == "docdb" {
			continue
		}

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

		findings := computeRDSDBClusterStatusAndIssues(cluster)
		statusPhrase := ""
		if len(findings) > 0 {
			statusPhrase = findings[0].Phrase
			if len(findings) > 1 {
				statusPhrase = fmt.Sprintf("%s (+%d)", statusPhrase, len(findings)-1)
			}
		}

		r := resource.Resource{
			ID:   clusterID,
			Name: clusterID,
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
			Findings:  findings,
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
