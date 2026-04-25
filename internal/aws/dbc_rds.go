package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// brokenRDSDBCStatusPhrase maps Aurora / Multi-AZ cluster statuses that represent
// hard failures to their display phrases per spec §4.
// Mirrors brokenDBCStatusPhrase for the RDS-side cluster type.
var brokenRDSDBCStatusPhrase = map[string]string{
	"failed":                              "failed: cluster operation",
	"inaccessible-encryption-credentials": "encryption key unreachable",
	"incompatible-parameters":             "parameter group incompatible",
}

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

// computeRDSDBClusterStatusAndIssues returns the §4 phrase and the full ordered
// list of every active issue phrase for an RDS-side (Aurora / Multi-AZ) DB cluster.
// Algorithm mirrors computeDBCStatusAndIssues.
func computeRDSDBClusterStatusAndIssues(cluster rdstypes.DBCluster) (string, []string) {
	status := aws.ToString(cluster.Status)

	// Broken statuses — first match wins; no warning stacking.
	if phrase, ok := brokenRDSDBCStatusPhrase[status]; ok {
		return phrase, []string{phrase}
	}

	// No writer on an available cluster — reads only (Broken; beats warnings).
	if status == "available" && countRDSWriters(cluster.DBClusterMembers) == 0 {
		const phrase = "no writer: reads only"
		return phrase, []string{phrase}
	}

	// Transitional statuses.
	if _, ok := transitionalRDSDBCStatusSet[status]; ok {
		phrase := status + ": in progress"
		return phrase, []string{phrase}
	}

	// Healthy available — collect Wave-1 warnings in spec §4 table order.
	if status == "available" {
		var warnings []string
		// §4 order: delete-protection off, not encrypted at rest, no automated backups.
		if cluster.DeletionProtection != nil && !*cluster.DeletionProtection {
			warnings = append(warnings, "delete-protection off")
		}
		if cluster.StorageEncrypted != nil && !*cluster.StorageEncrypted {
			warnings = append(warnings, "not encrypted at rest")
		}
		if cluster.BackupRetentionPeriod != nil && *cluster.BackupRetentionPeriod == 0 {
			warnings = append(warnings, "no automated backups")
		}
		switch len(warnings) {
		case 0:
			return "", nil
		case 1:
			return warnings[0], warnings
		default:
			return fmt.Sprintf("%s (+%d)", warnings[0], len(warnings)-1), warnings
		}
	}

	// Unknown status — bare keyword passthrough (future-proof for new AWS statuses).
	return status, []string{status}
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

		computedStatus, computedIssues := computeRDSDBClusterStatusAndIssues(cluster)

		r := resource.Resource{
			ID:     clusterID,
			Name:   clusterID,
			Status: computedStatus,
			Issues: computedIssues,
			Fields: map[string]string{
				"cluster_id":              clusterID,
				"engine_version":          engineVersion,
				"status":                  computedStatus,
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
