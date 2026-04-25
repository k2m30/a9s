package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("dbc", []string{"cluster_id", "engine_version", "status", "instances", "endpoint", "arn", "has_writer", "writer_count", "deletion_protection", "storage_encrypted", "backup_retention_period"})

	// dbc fetcher merges results from two separate SDK calls:
	//   c.DocDB.DescribeDBClusters — DocumentDB clusters. The docdb SDK docstring
	//     (docdb@v1.48.12/api_op_DescribeDBClusters.go:14-19) instructs callers to
	//     use filterName=engine,Values=docdb for DocDB-only results — unfiltered
	//     behavior is documented as ambiguous, not engine-agnostic.
	//   c.RDS.DescribeDBClusters   — Aurora + Multi-AZ clusters
	//     (rds@v1.116.3/api_op_DescribeDBClusters.go:19-28). May also return Neptune
	//     / DocumentDB rows — both SDKs must be called to get complete coverage.
	// Token format: "" or "docdb:<tok>" for DocDB pages, then "rds:" (sentinel) or
	// "rds:<tok>" for RDS pages.
	resource.RegisterPaginated("dbc", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}

		// RDS phase: continuation already transitioned to RDS side.
		// Partial DocDB rows were already returned in the prior page; this is a
		// single-side fetch with no prior-state append — an RDS error here returns
		// an empty result with the error so the operator can retry by re-opening the list.
		if rdsTok, ok2 := strings.CutPrefix(continuationToken, "rds:"); ok2 {
			result, err := FetchRDSDBClustersPage(ctx, c.RDS, rdsTok)
			if err != nil {
				return resource.FetchResult{}, err
			}
			if result.Pagination != nil && result.Pagination.IsTruncated {
				result.Pagination.NextToken = "rds:" + result.Pagination.NextToken
			}
			return result, nil
		}

		// DocDB phase (continuationToken == "" or "docdb:<tok>").
		docdbTok, _ := strings.CutPrefix(continuationToken, "docdb:")
		docResult, err := FetchDocDBClustersPage(ctx, c.DocDB, docdbTok)
		if err != nil {
			return resource.FetchResult{}, err
		}
		if docResult.Pagination != nil && docResult.Pagination.IsTruncated {
			// DocDB still has more pages — return with docdb: prefix.
			docResult.Pagination.NextToken = "docdb:" + docResult.Pagination.NextToken
			return docResult, nil
		}

		// Rule E5: preserve partial DocDB rows on RDS failure — return what we have
		// with IsTruncated=true so the operator sees the DocDB rows and a composite
		// error rather than an empty result with a silent discard.
		rdsResult, rdsErr := FetchRDSDBClustersPage(ctx, c.RDS, "")
		if rdsErr != nil {
			return resource.FetchResult{
				Resources: docResult.Resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "rds:",
					PageSize:    len(docResult.Resources),
					TotalHint:   -1,
				},
			}, fmt.Errorf("dbc: RDS-side cluster fetch failed: %w", rdsErr)
		}
		// Combined success: DocDB page + RDS page concatenated. Page size may exceed DefaultPageSize when both SDKs return full pages on the same fetch tick — this is a deliberate trade so the operator sees a unified list rather than waiting for a second tick. Pagination tokens stay correct (docdb: vs rds: prefix tracks side authoritatively).
		docResult.Resources = append(docResult.Resources, rdsResult.Resources...)
		if rdsResult.Pagination != nil && rdsResult.Pagination.IsTruncated {
			return resource.FetchResult{
				Resources: docResult.Resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "rds:" + rdsResult.Pagination.NextToken,
					PageSize:    len(docResult.Resources),
					TotalHint:   -1,
				},
			}, nil
		}
		return resource.FetchResult{
			Resources: docResult.Resources,
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				PageSize:    len(docResult.Resources),
				TotalHint:   len(docResult.Resources),
			},
		}, nil
	})

	resource.RegisterRelated("dbc", []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkDbcSG},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDbcAlarm, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDbcLogs, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbcKMS},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkDbcSecrets, NeedsTargetCache: true},
		{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkDbcDBI, NeedsTargetCache: true},
		{TargetType: "dbc-snap", DisplayName: "DB Cluster Snapshots", Checker: checkDbcDbcSnap, NeedsTargetCache: true},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkDbcSubnet},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbcVPC},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDbcCTEvents},
	})

	// docdb_types.DBCluster: VpcSecurityGroups[].VpcSecurityGroupId (list),
	// KmsKeyId (scalar). DBSubnetGroup on DocDB is just a *string name, not
	// a struct — VPC/Subnet navigation is surfaced via checkDbcVPC /
	// checkDbcSubnet in the related-panel, not via navigable fields.
	resource.RegisterNavigableFields("dbc", []resource.NavigableField{
		{FieldPath: "VpcSecurityGroups.VpcSecurityGroupId", TargetType: "sg"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})
}

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

		computedStatus, computedIssues := computeDBCStatusAndIssues(cluster)

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

// brokenDBCStatusPhrase maps DocumentDB cluster statuses that represent hard failures
// to their display phrases per spec §4.
var brokenDBCStatusPhrase = map[string]string{
	"failed":                              "failed: cluster operation",
	"inaccessible-encryption-credentials": "encryption key unreachable",
	"incompatible-parameters":             "parameter group incompatible",
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

// computeDBCStatusAndIssues returns the top S4 phrase (with `(+N)` suffix when
// multiple warnings stack) AND the full ordered list of every active issue phrase.
// The second return feeds Resource.Issues so the detail view can render all
// active warnings individually (spec rule 7). Broken / transitional / healthy
// states each produce at most one phrase.
func computeDBCStatusAndIssues(cluster docdbtypes.DBCluster) (string, []string) {
	status := aws.ToString(cluster.Status)

	// Broken statuses — first match wins; no warning stacking.
	if phrase, ok := brokenDBCStatusPhrase[status]; ok {
		return phrase, []string{phrase}
	}

	// No writer on an available cluster — reads only (Broken; beats warnings).
	if status == "available" && countWriters(cluster.DBClusterMembers) == 0 {
		const phrase = "no writer: reads only"
		return phrase, []string{phrase}
	}

	// Transitional statuses.
	if _, ok := transitionalDBCStatusSet[status]; ok {
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
