// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

		engine := ""
		if cluster.Engine != nil {
			engine = *cluster.Engine
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

		findings, attentionDetails := computeDBCFindings(cluster)
		statusPhrase := domain.StatusPhrase(findings)

		r := resource.Resource{
			ID:       clusterID,
			Name:     clusterID,
			Findings: findings,
			Fields: map[string]string{
				"cluster_id":              clusterID,
				"engine":                  engine,
				"engine_version":          engineVersion,
				"status":                  statusPhrase,
				"status_raw":              aws.ToString(cluster.Status),
				"instances":               instances,
				"endpoint":                endpoint,
				"arn":                     aws.ToString(cluster.DBClusterArn),
				"has_writer":              hasWriter,
				"writer_count":            strconv.Itoa(writerCount),
				"deletion_protection":     deletionProtection,
				"storage_encrypted":       storageEncrypted,
				"backup_retention_period": backupRetentionPeriod,
			},
			RawStruct:        cluster,
			AttentionDetails: attentionDetails,
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

// transitionalDBCStatusSet contains the DB cluster statuses that indicate a
// transitional (Warning) state. These show a ": in progress" suffix.
var transitionalDBCStatusSet = map[string]struct{}{
	"creating": {}, "modifying": {}, "backing-up": {}, "maintenance": {},
	"upgrading": {}, "starting": {}, "stopping": {}, "resetting-master-credentials": {},
	"renaming": {},
}

// dbClusterState is what a DB cluster's findings are read from. DocumentDB
// and RDS both answer DescribeDBClusters for the same clusters, in two SDK
// types carrying the same fields; a DocumentDB cluster has no
// AutoMinorVersionUpgrade or IAMDatabaseAuthenticationEnabled, and those two
// predicates stay silent when the posture is handed nil for them.
type dbClusterState struct {
	Status                string
	Writers               int
	DeletionProtection    *bool
	StorageEncrypted      *bool
	BackupRetentionPeriod *int32
	Posture               rdsPosture
}

// computeDBClusterFindings returns the findings for a DB cluster plus the
// supporting AttentionDetail rows keyed by the finding that owns them. The
// security-posture pack (rds_posture.go) is evaluated independently of the
// lifecycle status and stacks on top of it.
func computeDBClusterFindings(c dbClusterState) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	postureFindings, postureDetails := rdsPostureFindings(c.Posture, dbcPostureCodes)

	// Broken statuses — first match wins; no warning stacking.
	brokenCode := map[string]domain.FindingCode{
		"failed":                              CodeDBCFailed,
		"inaccessible-encryption-credentials": CodeDBCEncryptionKeyUnreachable,
		"incompatible-parameters":             CodeDBCIncompatibleParameters,
	}
	if code, ok := brokenCode[c.Status]; ok {
		lead := []domain.Finding{wave1Finding(code)}
		return append(lead, postureFindings...), postureDetails
	}

	// No writer on an available cluster — reads only (Broken; beats warnings).
	if c.Status == "available" && c.Writers == 0 {
		lead := []domain.Finding{wave1Finding(CodeDBCNoWriter)}
		return append(lead, postureFindings...), postureDetails
	}

	// A cluster on its way out has no posture worth reporting.
	if isTeardownStatus(c.Status) {
		postureFindings, postureDetails = nil, nil
	}

	// Transitional statuses.
	if _, ok := transitionalDBCStatusSet[c.Status]; ok {
		lead := []domain.Finding{wave1Finding(CodeDBCTransitional, c.Status)}
		return append(lead, postureFindings...), postureDetails
	}

	// Healthy available — collect Wave-1 warnings.
	if c.Status == "available" {
		var findings []domain.Finding
		if c.DeletionProtection != nil && !*c.DeletionProtection {
			findings = append(findings, wave1Finding(CodeDBCDeletionProtectionOff))
		}
		if c.StorageEncrypted != nil && !*c.StorageEncrypted {
			findings = append(findings, wave1Finding(CodeDBCNotEncryptedAtRest))
		}
		if c.BackupRetentionPeriod != nil && *c.BackupRetentionPeriod == 0 {
			findings = append(findings, wave1Finding(CodeDBCNoAutomatedBackups))
		}
		return append(findings, postureFindings...), postureDetails
	}

	// A cluster AWS reported no status for has no keyword to pass through.
	// The transitional wording puts that keyword in front of "in progress",
	// and an empty one claims a transition nobody reported.
	if c.Status == "" {
		return postureFindings, postureDetails
	}

	// Unknown status — bare keyword passthrough (future-proof for new AWS statuses).
	lead := []domain.Finding{wave1Finding(CodeDBCTransitional, c.Status)}
	return append(lead, postureFindings...), postureDetails
}

// computeDBCFindings reads a DocumentDB cluster's findings.
func computeDBCFindings(cluster docdbtypes.DBCluster) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	writers := 0
	for _, m := range cluster.DBClusterMembers {
		if aws.ToBool(m.IsClusterWriter) {
			writers++
		}
	}
	return computeDBClusterFindings(dbClusterState{
		Status:                aws.ToString(cluster.Status),
		Writers:               writers,
		DeletionProtection:    cluster.DeletionProtection,
		StorageEncrypted:      cluster.StorageEncrypted,
		BackupRetentionPeriod: cluster.BackupRetentionPeriod,
		Posture: rdsPosture{
			Engine:         aws.ToString(cluster.Engine),
			MultiAZ:        cluster.MultiAZ,
			MasterUsername: cluster.MasterUsername,
		},
	})
}

// dedupResourcesByID returns rs with duplicate Resource.ID entries removed,
// keeping the first occurrence. Used by the dbc and dbc-snap fetchers where
// the DocDB and RDS SDKs both return overlapping rows for the same cluster /
// snapshot (verified live: the DocDB DescribeDBClusters endpoint
// returns aurora-postgresql clusters too). DocDB-side rows are appended first
// at the call sites, so first-occurrence wins keeps the docdb-side row.
//
// Dedup-by-ID rather than an engine filter at source: an engine filter goes
// silently stale the moment AWS adds a new docdb engine variant or new aurora
// flavor, whereas dedup-by-ID is symmetric across both fetchers and robust to
// SDK drift.
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
