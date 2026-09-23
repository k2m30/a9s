// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// computeRDSDBClusterFindings reads an RDS-side (Aurora / Multi-AZ) DB
// cluster's findings.
func computeRDSDBClusterFindings(cluster rdstypes.DBCluster, expectsWriter *bool) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
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
			Engine:                  aws.ToString(cluster.Engine),
			MultiAZ:                 cluster.MultiAZ,
			AutoMinorVersionUpgrade: cluster.AutoMinorVersionUpgrade,
			IAMAuthEnabled:          cluster.IAMDatabaseAuthenticationEnabled,
			MasterUsername:          cluster.MasterUsername,
		},
	})
}

// isDBCListedEngine reports whether DB Clusters lists clusters of engine.
// Neptune shares the RDS endpoint but is not a DB Clusters row. A deny-list
// keeps a new Aurora engine variant listed.
func isDBCListedEngine(engine string) bool {
	return strings.ToLower(engine) != "neptune"
}

// isRDSSideDBCEngine reports whether a cluster or cluster snapshot of engine
// is listed from the RDS call; DocumentDB rows come from the DocumentDB call.
func isRDSSideDBCEngine(engine string) bool {
	return strings.ToLower(engine) != "docdb" && isDBCListedEngine(engine)
}

// FetchRDSDBClustersPage fetches a single page of Aurora + Multi-AZ DB clusters
// via the RDS SDK.
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

	roles := readRDSGlobalRoles(ctx, api)
	withheld := 0
	var resources []resource.Resource

	for _, cluster := range output.DBClusters {
		engine := strings.ToLower(aws.ToString(cluster.Engine))
		if !isRDSSideDBCEngine(engine) {
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

		expectsWriter := roles.expectsWriter(aws.ToString(cluster.DBClusterArn),
			aws.ToString(cluster.ReplicationSourceIdentifier) != "", aws.ToString(cluster.GlobalClusterIdentifier) != "")
		if expectsWriter == nil && writerCount == 0 {
			withheld++
		}
		findings, attentionDetails := computeRDSDBClusterFindings(cluster, expectsWriter)
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
