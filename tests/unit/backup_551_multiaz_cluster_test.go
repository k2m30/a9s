package unit

// An RDS Multi-AZ DB cluster (MySQL or PostgreSQL) is an Amazon RDS resource
// type, so AWS Backup applies the RDS opt-in to it as to any other RDS
// resource: a selection naming it by ARN or by the cluster resource type
// includes it regardless, and one that rests on "*" only while RDS is opted in.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const multiAZClusterARN = "arn:aws:rds:us-east-1:123456789012:cluster:acme-orders-maz"

type multiAZClusterFake struct {
	awsclient.RDSAPI
	engine string
}

func (f multiAZClusterFake) DescribeDBClusters(context.Context, *rds.DescribeDBClustersInput, ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{DBClusters: []rdstypes.DBCluster{{
		DBClusterIdentifier:    aws.String("acme-orders-maz"),
		DBClusterArn:           aws.String(multiAZClusterARN),
		Engine:                 aws.String(f.engine),
		EngineVersion:          aws.String("8.0.36"),
		Status:                 aws.String("available"),
		StorageEncrypted:       aws.Bool(true),
		DeletionProtection:     aws.Bool(true),
		MultiAZ:                aws.Bool(true),
		DBClusterInstanceClass: aws.String("db.m6gd.large"),
	}}}, nil
}

func TestBackupCoverageJoin_MultiAZClusterFollowsTheRDSOptIn(t *testing.T) {
	cases := []struct {
		name  string
		optIn map[string]bool
		sel   backuptypes.BackupSelection
		warn  bool
	}{
		{"exact ARN, RDS opted out", bkScopeOptedOut("RDS"), backuptypes.BackupSelection{Resources: []string{multiAZClusterARN}}, false},
		{"cluster resource type, RDS opted out", bkScopeOptedOut("RDS"), backuptypes.BackupSelection{Resources: []string{"arn:aws:rds:*:*:cluster:*"}}, false},
		{"star, RDS opted out", bkScopeOptedOut("RDS"), backuptypes.BackupSelection{Resources: []string{"*"}}, true},
		{"star, RDS opted in", bkScopeOptedOut("Aurora", "DocumentDB", "Neptune"), backuptypes.BackupSelection{Resources: []string{"*"}}, false},
	}
	for _, engine := range []string{"mysql", "postgres"} {
		clusters, err := awsclient.FetchRDSDBClustersPage(context.Background(), multiAZClusterFake{engine: engine}, "")
		if err != nil || len(clusters.Resources) != 1 {
			t.Fatalf("%s: FetchRDSDBClustersPage = %d rows, %v; want 1", engine, len(clusters.Resources), err)
		}
		row := clusters.Resources[0]
		for _, tc := range cases {
			t.Run(engine+"/"+tc.name, func(t *testing.T) {
				res, err := awsclient.EnrichDBCMaintenance(context.Background(), &awsclient.ServiceClients{},
					[]resource.Resource{row}, bk551Cache(bkScopePlanOptIn(t, tc.optIn, tc.sel)))
				if err != nil && res.Findings == nil {
					t.Fatalf("EnrichDBCMaintenance: %v", err)
				}
				if tc.warn {
					w4AssertFinding(t, res.Findings[row.ID], awsclient.CodeDBCNotInBackupPlan,
						"not covered by a backup plan", domain.SevWarn, "wave2")
				} else {
					w4AssertNoCode(t, res.Findings[row.ID], awsclient.CodeDBCNotInBackupPlan)
				}
				if check, ok := res.TruncatedIDs[row.ID]; ok {
					t.Errorf("marked not inspected (%q), want a verdict", check)
				}
			})
		}
	}
}
