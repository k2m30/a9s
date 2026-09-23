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

// FetchDocDBClustersPage fetches a single page of DocumentDB clusters. The
// DocumentDB endpoint is shared with RDS and Neptune and answers every
// engine's clusters unless asked for engine=docdb, as the SDK's
// DescribeDBClusters doc comment directs.
func FetchDocDBClustersPage(ctx context.Context, api DocDBDescribeDBClustersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &docdb.DescribeDBClustersInput{
		MaxRecords: aws.Int32(DefaultPageSize),
		Filters:    []docdbtypes.Filter{{Name: aws.String("engine"), Values: []string{"docdb"}}},
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

	// The DocumentDB cluster record carries no global cluster identifier, so
	// only the member list can rule a global membership in or out.
	roles := readDocDBGlobalRoles(ctx, api)
	withheld := 0
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

		expectsWriter := roles.expectsWriter(aws.ToString(cluster.DBClusterArn), aws.ToString(cluster.ReplicationSourceIdentifier) != "", true)
		if expectsWriter == nil && writerCount == 0 {
			withheld++
		}
		findings, attentionDetails := computeDBCFindings(cluster, expectsWriter)
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
	}, roles.unreadErr(withheld)
}

// dbClusterState is what a DB cluster's findings are read from. DocumentDB
// and RDS both answer DescribeDBClusters for the same clusters, in two SDK
// types carrying the same fields; a DocumentDB cluster has no
// AutoMinorVersionUpgrade or IAMDatabaseAuthenticationEnabled, and those two
// predicates stay silent when the posture is handed nil for them.
type dbClusterState struct {
	Status  string
	Writers int
	// ExpectsWriter says whether a missing writer is broken; nil when the
	// cluster's role could not be read (dbcGlobalRoles.expectsWriter).
	ExpectsWriter         *bool
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
	// A cluster on its way out has no posture worth reporting.
	if isTeardownStatus(c.Status) {
		postureFindings, postureDetails = nil, nil
	}

	if lead, ok := dbcLifecycle.finding(c.Status, c.Status); ok {
		return append([]domain.Finding{lead}, postureFindings...), postureDetails
	}
	if c.Status != "available" {
		return postureFindings, postureDetails
	}

	// No writer on an available cluster — reads only (Broken; beats warnings).
	if c.Writers == 0 && aws.ToBool(c.ExpectsWriter) {
		lead := []domain.Finding{wave1Finding(CodeDBCNoWriter)}
		return append(lead, postureFindings...), postureDetails
	}

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

// computeDBCFindings reads a DocumentDB cluster's findings.
func computeDBCFindings(cluster docdbtypes.DBCluster, expectsWriter *bool) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	writers := 0
	for _, m := range cluster.DBClusterMembers {
		if aws.ToBool(m.IsClusterWriter) {
			writers++
		}
	}
	return computeDBClusterFindings(dbClusterState{
		Status:                aws.ToString(cluster.Status),
		Writers:               writers,
		ExpectsWriter:         expectsWriter,
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
// keeping the first occurrence. The dbc and dbc-snap fetchers append the
// DocumentDB-side rows first, so a row both lanes return keeps its
// DocumentDB shape.
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
