package unit

// backup_coverage_scope_test.go — what a backup plan can cover beyond the
// selection rule itself.
//
// Aurora, DocumentDB and Neptune are backed up at cluster level: a member
// instance is protected when its cluster is, and "arn:aws:rds:*:*:db:*"
// selects no such instance ("Assign resources with AWS CLI").
//
// A plan backs up only resources in its own Region ("Create S3 continuous or
// periodic backups in AWS Backup": the plan must be in the bucket's Region).
//
// A selection by service name ("arn:aws:ec2:*"), by "*", by an empty
// Resources list or by tags alone includes a resource only when its resource
// type is opted in for the Region (DescribeRegionSettings
// ResourceTypeOptInPreference); a selection by resource type
// ("arn:aws:ec2:*:*:instance/*") or by exact ARN includes it regardless.
// Aurora, DocumentDB and Neptune share the cluster ARN format, so a
// "cluster:*" pattern includes a cluster only when its engine's type is
// opted in ("Shared resource types"). An explicit assignment does not lift
// the opt-in for them either (AWS Backup "Getting started — Service Opt-in":
// "This does not apply to Aurora, Neptune, and Amazon DocumentDB. For these
// services to be included, the opt-in must be enabled.").

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	bkScopeClusterID  = "acme-ledger-cluster"
	bkScopeClusterARN = "arn:aws:rds:us-east-1:123456789012:cluster:acme-ledger-cluster"
	bkScopeMemberID   = "acme-ledger-cluster-instance-1"
	bkScopeMemberARN  = "arn:aws:rds:us-east-1:123456789012:db:acme-ledger-cluster-instance-1"
	bkScopeSoloID     = "acme-ledger-db"
)

func bkScopeFetch(t *testing.T, fake *bk551BackupFake) resource.Resource {
	t.Helper()
	res, err := awsclient.FetchBackupPlansPage(context.Background(), fake, "")
	if len(res.Resources) != 1 {
		t.Fatalf("FetchBackupPlansPage returned %d plans (err %v), want 1", len(res.Resources), err)
	}
	return res.Resources[0]
}

func bkScopePlanOptIn(t *testing.T, optIn map[string]bool, sels ...backuptypes.BackupSelection) resource.Resource {
	t.Helper()
	fake := bk551NewFake(-1, sels)
	fake.optIn = optIn
	return bkScopeFetch(t, fake)
}

func bkScopePlanSettingsDenied(t *testing.T, sels ...backuptypes.BackupSelection) resource.Resource {
	t.Helper()
	fake := bk551NewFake(-1, sels)
	fake.settingsErr = &smithy.OperationError{
		ServiceID:     "Backup",
		OperationName: "DescribeRegionSettings",
		Err: &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform: backup:DescribeRegionSettings",
		},
	}
	return bkScopeFetch(t, fake)
}

// bkScopeEBSCache also carries the snapshot list the ebs enricher declares,
// read to the end and empty.
func bkScopeEBSCache(plan resource.Resource) resource.ResourceCache {
	cache := bk551Cache(plan)
	cache["ebs-snap"] = resource.ResourceCacheEntry{}
	return cache
}

func bkScopeOptedOut(types ...string) map[string]bool {
	m := bk551AllOptedIn()
	for _, typ := range types {
		m[typ] = false
	}
	return m
}

// ── cluster members ──────────────────────────────────────────────────────

var bkScopeEngineVersion = map[string]string{
	"aurora-postgresql": "15.4",
	"docdb":             "5.0.0",
	"neptune":           "1.3.2.0",
}

type bkScopeRDSFake struct {
	awsclient.RDSAPI

	engine      string
	clusterTags map[string]string
	memberTags  map[string]string
}

func bkScopeTagList(tags map[string]string) []rdstypes.Tag {
	var out []rdstypes.Tag
	for k, v := range tags {
		out = append(out, rdstypes.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return out
}

func (f *bkScopeRDSFake) DescribeDBInstances(context.Context, *rds.DescribeDBInstancesInput, ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{
		{
			DBInstanceIdentifier:  aws.String(bkScopeMemberID),
			DBInstanceArn:         aws.String(bkScopeMemberARN),
			DBClusterIdentifier:   aws.String(bkScopeClusterID),
			DBInstanceClass:       aws.String("db.r6g.large"),
			DBInstanceStatus:      aws.String("available"),
			Engine:                aws.String(f.engine),
			EngineVersion:         aws.String(bkScopeEngineVersion[f.engine]),
			AvailabilityZone:      aws.String(bk551Region + "a"),
			StorageEncrypted:      aws.Bool(true),
			BackupRetentionPeriod: aws.Int32(7),
			TagList:               bkScopeTagList(f.memberTags),
		},
		{
			DBInstanceIdentifier:  aws.String(bkScopeSoloID),
			DBInstanceArn:         aws.String(bk551DBARN),
			DBInstanceClass:       aws.String("db.t4g.medium"),
			DBInstanceStatus:      aws.String("available"),
			Engine:                aws.String("postgres"),
			EngineVersion:         aws.String("16.3"),
			AvailabilityZone:      aws.String(bk551Region + "b"),
			MultiAZ:               aws.Bool(true),
			StorageEncrypted:      aws.Bool(true),
			DeletionProtection:    aws.Bool(true),
			BackupRetentionPeriod: aws.Int32(7),
		},
	}}, nil
}

func (f *bkScopeRDSFake) DescribeDBClusters(context.Context, *rds.DescribeDBClustersInput, ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{DBClusters: []rdstypes.DBCluster{{
		DBClusterIdentifier: aws.String(bkScopeClusterID),
		DBClusterArn:        aws.String(bkScopeClusterARN),
		Engine:              aws.String(f.engine),
		EngineVersion:       aws.String(bkScopeEngineVersion[f.engine]),
		Status:              aws.String("available"),
		StorageEncrypted:    aws.Bool(true),
		DeletionProtection:  aws.Bool(true),
		DBClusterMembers: []rdstypes.DBClusterMember{{
			DBInstanceIdentifier: aws.String(bkScopeMemberID),
			IsClusterWriter:      aws.Bool(true),
		}},
		TagList: bkScopeTagList(f.clusterTags),
	}}}, nil
}

func (f *bkScopeRDSFake) ListTagsForResource(_ context.Context, in *rds.ListTagsForResourceInput, _ ...func(*rds.Options)) (*rds.ListTagsForResourceOutput, error) {
	switch aws.ToString(in.ResourceName) {
	case bkScopeClusterARN:
		return &rds.ListTagsForResourceOutput{TagList: bkScopeTagList(f.clusterTags)}, nil
	case bkScopeMemberARN:
		return &rds.ListTagsForResourceOutput{TagList: bkScopeTagList(f.memberTags)}, nil
	}
	return &rds.ListTagsForResourceOutput{}, nil
}

func (f *bkScopeRDSFake) DescribePendingMaintenanceActions(context.Context, *rds.DescribePendingMaintenanceActionsInput, ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return &rds.DescribePendingMaintenanceActionsOutput{}, nil
}

func (f *bkScopeRDSFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return &rds.DescribeDBEngineVersionsOutput{DBEngineVersions: []rdstypes.DBEngineVersion{{
		Engine: in.Engine, EngineVersion: in.EngineVersion, Status: aws.String("available"),
	}}}, nil
}

func bkScopeEnrichDBI(t *testing.T, fake *bkScopeRDSFake, plan resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	ctx := context.Background()
	dbis, err := awsclient.FetchRDSInstancesPage(ctx, fake, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage: %v", err)
	}
	dbcs, err := awsclient.FetchRDSDBClustersPage(ctx, fake, "")
	if err != nil {
		t.Fatalf("FetchRDSDBClustersPage: %v", err)
	}
	clients := bk551Clients()
	clients.RDS = fake
	cache := bk551Cache(plan)
	cache["dbc"] = resource.ResourceCacheEntry{Resources: dbcs.Resources}
	res, err := awsclient.EnrichDBIMaintenance(ctx, clients, dbis.Resources, cache)
	if err != nil && res.Findings == nil {
		t.Fatalf("EnrichDBIMaintenance: %v", err)
	}
	return res
}

func bkScopeTagged(key, value string) *backuptypes.Conditions {
	return &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond(key, value)}}
}

func TestBackupCoverageJoin_ClusterMemberJudgedByItsCluster(t *testing.T) {
	type want struct{ member, solo bool } // true: dbi.not-in-backup-plan raised
	cases := []struct {
		name        string
		sel         backuptypes.BackupSelection
		clusterTags map[string]string
		memberTags  map[string]string
		want        want
	}{
		{"plan names the cluster", backuptypes.BackupSelection{Resources: []string{bkScopeClusterARN}}, nil, nil,
			want{member: false, solo: true}},
		{"plan takes every DB instance", backuptypes.BackupSelection{Resources: []string{"arn:aws:rds:*:*:db:*"}}, nil, nil,
			want{member: true, solo: false}},
		{"plan names the member instance", backuptypes.BackupSelection{Resources: []string{bkScopeMemberARN}}, nil, nil,
			want{member: true, solo: true}},
		{"tag clause met by the cluster", backuptypes.BackupSelection{
			Resources: []string{"arn:aws:rds:*:*:cluster:*"}, Conditions: bkScopeTagged("backup", "true"),
		}, map[string]string{"backup": "true"}, map[string]string{"Name": bkScopeMemberID}, want{member: false, solo: true}},
		{"tag clause met by the member only", backuptypes.BackupSelection{
			Resources: []string{"arn:aws:rds:*:*:cluster:*"}, Conditions: bkScopeTagged("backup", "true"),
		}, map[string]string{"Name": bkScopeClusterID}, map[string]string{"backup": "true"}, want{member: true, solo: true}},
		{"ListOfTags met by the cluster", backuptypes.BackupSelection{
			ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")},
		}, map[string]string{"backup": "true"}, nil, want{member: false, solo: true}},
	}
	for _, engine := range []string{"aurora-postgresql", "docdb", "neptune"} {
		for _, tc := range cases {
			t.Run(engine+"/"+tc.name, func(t *testing.T) {
				fake := &bkScopeRDSFake{engine: engine, clusterTags: tc.clusterTags, memberTags: tc.memberTags}
				res := bkScopeEnrichDBI(t, fake, bk551Plan(t, tc.sel))

				for _, row := range []struct {
					id   string
					want bool
				}{{bkScopeMemberID, tc.want.member}, {bkScopeSoloID, tc.want.solo}} {
					if got := hasCode(res.Findings[row.id], awsclient.CodeDBINotInBackupPlan); got != row.want {
						t.Errorf("%s: dbi.not-in-backup-plan raised = %v, want %v", row.id, got, row.want)
					} else if row.want {
						w4AssertFinding(t, res.Findings[row.id], awsclient.CodeDBINotInBackupPlan,
							"not covered by a backup plan", domain.SevWarn, "wave2")
					}
					if check, ok := res.TruncatedIDs[row.id]; ok {
						t.Errorf("%s marked not inspected (%q), want a verdict", row.id, check)
					}
				}
			})
		}
	}
}

// ── the bucket's Region ─────────────────────────────────────────────────

func TestBackupS3Pivot_PlanCoversOnlyBucketsInItsRegion(t *testing.T) {
	var s3p bk551Pivot
	for _, p := range bk551Pivots() {
		if p.short == "s3" {
			s3p = p
		}
	}
	bucket := func(region *string) resource.Resource {
		r := s3p.res
		r.RawStruct = s3types.Bucket{Name: aws.String(bk551Bucket), BucketRegion: region}
		return r
	}
	cases := []struct {
		name      string
		region    *string
		resources []string
		want      []string
	}{
		{"same Region, star", aws.String(bk551Region), []string{"*"}, []string{bk551PlanID}},
		{"same Region, exact ARN", aws.String(bk551Region), []string{bk551BucketARN}, []string{bk551PlanID}},
		{"other Region, star", aws.String("eu-west-1"), []string{"*"}, nil},
		{"other Region, every bucket", aws.String("eu-west-1"), []string{"arn:aws:s3:::*"}, nil},
		{"other Region, exact ARN", aws.String("eu-west-1"), []string{bk551BucketARN}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := s3p
			p.res = bucket(tc.region)
			got := bk551RunPivot(t, p, bk551Plan(t, backuptypes.BackupSelection{Resources: tc.resources}))
			bk551AssertPlans(t, got, tc.want)
		})
	}

	t.Run("Region absent", func(t *testing.T) {
		p := s3p
		p.res = bucket(nil)
		got := bk551RunPivot(t, p, bk551Plan(t, backuptypes.BackupSelection{Resources: []string{"*"}}))
		if got.State() != domain.RelatedUnknown {
			t.Errorf("State = %v (plans %v), want unknown: the bucket's Region was not returned", got.State(), got.ResourceIDs())
		}
	})
}

// ── Region opt-in ───────────────────────────────────────────────────────

func TestBackupPlanCovers_RegionOptIn(t *testing.T) {
	noDDBNoEBS := bkScopeOptedOut("DynamoDB", "EBS")
	byTag := backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")}}
	tagged := map[string]string{"backup": "true"}
	cases := []struct {
		name        string
		sels        []backuptypes.BackupSelection
		arn         string
		tags        map[string]string
		wantCovered bool
	}{
		{"star, type opted out", []backuptypes.BackupSelection{{Resources: []string{"*"}}}, bk551TableARN, nil, false},
		{"star, type opted in", []backuptypes.BackupSelection{{Resources: []string{"*"}}}, bk551InstanceARN, nil, true},
		{"empty selection, type opted out", []backuptypes.BackupSelection{{}}, bk551TableARN, nil, false},
		{"service name, type opted out", []backuptypes.BackupSelection{{Resources: []string{"arn:aws:dynamodb:*"}}}, bk551TableARN, nil, false},
		{"service name, volume opted out", []backuptypes.BackupSelection{{Resources: []string{"arn:aws:ec2:*"}}}, bk551VolumeARN, nil, false},
		{"service name, instance opted in", []backuptypes.BackupSelection{{Resources: []string{"arn:aws:ec2:*"}}}, bk551InstanceARN, nil, true},
		{"resource type, type opted out", []backuptypes.BackupSelection{{Resources: []string{"arn:aws:dynamodb:*:*:table/*"}}}, bk551TableARN, nil, true},
		{"exact ARN, type opted out", []backuptypes.BackupSelection{{Resources: []string{bk551TableARN}}}, bk551TableARN, nil, true},
		{"ListOfTags, type opted out", []backuptypes.BackupSelection{byTag}, bk551TableARN, tagged, false},
		{"ListOfTags, type opted in", []backuptypes.BackupSelection{byTag}, bk551InstanceARN, tagged, true},
		{"Conditions only, type opted out", []backuptypes.BackupSelection{{Conditions: bkScopeTagged("backup", "true")}}, bk551TableARN, tagged, false},
		{"star, then resource type", []backuptypes.BackupSelection{
			{Resources: []string{"*"}}, {Resources: []string{"arn:aws:dynamodb:*:*:table/*"}},
		}, bk551TableARN, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := bkScopePlanOptIn(t, noDDBNoEBS, tc.sels...)
			covered, known := awsclient.BackupPlanCovers(plan, tc.arn, tc.tags, true)
			if covered != tc.wantCovered || !known {
				t.Errorf("BackupPlanCovers = (%v, %v), want (%v, true)", covered, known, tc.wantCovered)
			}
		})
	}
}

func TestBackupPlanCovers_RegionSettingsUnread(t *testing.T) {
	byTag := backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")}}
	cases := []struct {
		name        string
		sel         backuptypes.BackupSelection
		tags        map[string]string
		wantCovered bool
		wantKnown   bool
	}{
		{"star", backuptypes.BackupSelection{Resources: []string{"*"}}, nil, false, false},
		{"service name", backuptypes.BackupSelection{Resources: []string{"arn:aws:dynamodb:*"}}, nil, false, false},
		{"ListOfTags met", byTag, map[string]string{"backup": "true"}, false, false},
		{"resource type", backuptypes.BackupSelection{Resources: []string{"arn:aws:dynamodb:*:*:table/*"}}, nil, true, true},
		{"exact ARN", backuptypes.BackupSelection{Resources: []string{bk551TableARN}}, nil, true, true},
		{"exact ARN of another table", backuptypes.BackupSelection{Resources: []string{"arn:aws:dynamodb:us-east-1:123456789012:table/acme-audit"}}, nil, false, true},
		{"star minus the table", backuptypes.BackupSelection{Resources: []string{"*"}, NotResources: []string{bk551TableARN}}, nil, false, true},
		{"ListOfTags not met", byTag, map[string]string{"backup": "false"}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := bkScopePlanSettingsDenied(t, tc.sel)
			covered, known := awsclient.BackupPlanCovers(plan, bk551TableARN, tc.tags, true)
			if covered != tc.wantCovered || known != tc.wantKnown {
				t.Errorf("BackupPlanCovers = (%v, %v), want (%v, %v)", covered, known, tc.wantCovered, tc.wantKnown)
			}
		})
	}
}

func TestBackupCoverageJoin_RegionOptIn(t *testing.T) {
	star := backuptypes.BackupSelection{Resources: []string{"*"}}
	byTag := backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")}}
	allVolumes := backuptypes.BackupSelection{Resources: []string{"arn:aws:ec2:*:*:volume/*"}}
	taggedVol := bk551Volume(map[string]string{"Name": "acme-ledger-data", "backup": "true"})

	t.Run("ebs", func(t *testing.T) {
		cases := []struct {
			name  string
			optIn map[string]bool
			sel   backuptypes.BackupSelection
			warn  bool
		}{
			{"star, EBS opted out", bkScopeOptedOut("EBS"), star, true},
			{"star, EBS opted in", bkScopeOptedOut("DynamoDB"), star, false},
			{"tags alone, EBS opted out", bkScopeOptedOut("EBS"), byTag, true},
			{"tags alone, EBS opted in", bkScopeOptedOut("DynamoDB"), byTag, false},
			{"every volume, EBS opted out", bkScopeOptedOut("EBS"), allVolumes, false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				res := bk551EnrichEBS(t, taggedVol, bkScopeEBSCache(bkScopePlanOptIn(t, tc.optIn, tc.sel)))
				if tc.warn {
					w4AssertFinding(t, res.Findings[bk551VolumeID], awsclient.CodeEBSNotInBackupPlan,
						"not covered by a backup plan", domain.SevWarn, "wave2")
				} else {
					w4AssertNoCode(t, res.Findings[bk551VolumeID], awsclient.CodeEBSNotInBackupPlan)
				}
				if check, ok := res.TruncatedIDs[bk551VolumeID]; ok {
					t.Errorf("volume marked not inspected (%q), want a verdict", check)
				}
			})
		}
	})

	t.Run("ddb", func(t *testing.T) {
		table := bk551Pivots()[0].res
		for _, tc := range []struct {
			name  string
			optIn map[string]bool
			warn  bool
		}{
			{"star, DynamoDB opted out", bkScopeOptedOut("DynamoDB"), true},
			{"star, DynamoDB opted in", bkScopeOptedOut("EBS"), false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				res, err := awsclient.EnrichDynamoDBPITR(context.Background(), &awsclient.ServiceClients{},
					[]resource.Resource{table}, bk551Cache(bkScopePlanOptIn(t, tc.optIn, star)))
				if err != nil && res.Findings == nil {
					t.Fatalf("EnrichDynamoDBPITR: %v", err)
				}
				if tc.warn {
					w4AssertFinding(t, res.Findings[table.ID], awsclient.CodeDDBNotInBackupPlan,
						"not covered by a backup plan", domain.SevWarn, "wave2")
				} else {
					w4AssertNoCode(t, res.Findings[table.ID], awsclient.CodeDDBNotInBackupPlan)
				}
			})
		}
	})

	t.Run("settings unread", func(t *testing.T) {
		res := bk551EnrichEBS(t, taggedVol, bkScopeEBSCache(bkScopePlanSettingsDenied(t, star)))
		w4AssertNoCode(t, res.Findings[bk551VolumeID], awsclient.CodeEBSNotInBackupPlan)
		if got := res.TruncatedIDs[bk551VolumeID]; got != "DescribeRegionSettings" {
			t.Errorf("TruncatedIDs[%s] = %q, want \"DescribeRegionSettings\": the opt-in the star match rests on was never read", bk551VolumeID, got)
		}
	})
}

// bkScopeOptInTypes names the ResourceTypeOptInPreference keys of the
// resource each pivot judges; an RDS cluster ARN is shared by Aurora,
// DocumentDB and Neptune.
var bkScopeOptInTypes = map[string][]string{
	"ddb":      {"DynamoDB"},
	"dbi-snap": {"RDS"},
	"dbc-snap": {"Aurora", "DocumentDB", "Neptune"},
	"efs":      {"EFS"},
	"ec2":      {"EC2"},
	"s3":       {"S3"},
	"ebs":      {"EBS"},
	"ebs-snap": {"EBS"},
}

func TestBackupPivots_RegionOptIn(t *testing.T) {
	for _, p := range bk551Pivots() {
		t.Run(p.short, func(t *testing.T) {
			types, ok := bkScopeOptInTypes[p.short]
			if !ok {
				t.Fatalf("no opt-in type for %s", p.short)
			}
			optedOut := bkScopeOptedOut(types...)

			star := bkScopePlanOptIn(t, optedOut, backuptypes.BackupSelection{Resources: []string{"*"}})
			bk551AssertPlans(t, bk551RunPivot(t, p, star), nil)

			optedIn := bkScopePlanOptIn(t, bk551AllOptedIn(), backuptypes.BackupSelection{Resources: []string{"*"}})
			bk551AssertPlans(t, bk551RunPivot(t, p, optedIn), []string{bk551PlanID})

			byType := bkScopePlanOptIn(t, optedOut, backuptypes.BackupSelection{Resources: []string{p.wildcard}})
			want := []string{bk551PlanID}
			if p.short == "dbc-snap" {
				want = nil
			}
			bk551AssertPlans(t, bk551RunPivot(t, p, byType), want)
		})
	}
}

// ── clusters: one ARN format, three opt-in types ─────────────────────────

const bkScopeClusterSnapID = "rds:acme-ledger-cluster-2026-09-18-04-00"

// bkScopeClusterOptInType maps each engine the dbc list carries to its
// ResourceTypeOptInPreference key. Aurora rows come from the RDS API and
// DocumentDB rows from the DocumentDB API.
var bkScopeClusterOptInType = map[string]string{
	"aurora-postgresql": "Aurora",
	"docdb":             "DocumentDB",
}

func bkScopeOtherClusterType(engine string) string {
	if bkScopeClusterOptInType[engine] == "DocumentDB" {
		return "Aurora"
	}
	return "DocumentDB"
}

func (f *bkScopeRDSFake) DescribeDBClusterSnapshots(context.Context, *rds.DescribeDBClusterSnapshotsInput, ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	return &rds.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: []rdstypes.DBClusterSnapshot{{
		DBClusterSnapshotIdentifier: aws.String(bkScopeClusterSnapID),
		DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:" + bkScopeClusterSnapID),
		DBClusterIdentifier:         aws.String(bkScopeClusterID),
		Engine:                      aws.String(f.engine),
		EngineVersion:               aws.String(bkScopeEngineVersion[f.engine]),
		SnapshotType:                aws.String("automated"),
		Status:                      aws.String("available"),
		StorageEncrypted:            aws.Bool(true),
		SnapshotCreateTime:          aws.Time(time.Date(2026, 9, 18, 4, 0, 0, 0, time.UTC)),
	}}}, nil
}

type bkScopeDocDBFake struct {
	awsclient.DocDBAPI
}

func (bkScopeDocDBFake) DescribeDBClusters(context.Context, *docdb.DescribeDBClustersInput, ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{DBClusters: []docdbtypes.DBCluster{{
		DBClusterIdentifier: aws.String(bkScopeClusterID),
		DBClusterArn:        aws.String(bkScopeClusterARN),
		Engine:              aws.String("docdb"),
		EngineVersion:       aws.String("5.0.0"),
		Status:              aws.String("available"),
		StorageEncrypted:    aws.Bool(true),
		DeletionProtection:  aws.Bool(true),
		MultiAZ:             aws.Bool(true),
		DBClusterMembers: []docdbtypes.DBClusterMember{{
			DBInstanceIdentifier: aws.String(bkScopeMemberID),
			IsClusterWriter:      aws.Bool(true),
		}},
	}}}, nil
}

func (bkScopeDocDBFake) DescribeDBClusterSnapshots(context.Context, *docdb.DescribeDBClusterSnapshotsInput, ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	return &docdb.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{{
		DBClusterSnapshotIdentifier: aws.String(bkScopeClusterSnapID),
		DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:" + bkScopeClusterSnapID),
		DBClusterIdentifier:         aws.String(bkScopeClusterID),
		Engine:                      aws.String("docdb"),
		EngineVersion:               aws.String("5.0.0"),
		SnapshotType:                aws.String("automated"),
		Status:                      aws.String("available"),
		StorageEncrypted:            aws.Bool(true),
		SnapshotCreateTime:          aws.Time(time.Date(2026, 9, 18, 4, 0, 0, 0, time.UTC)),
	}}}, nil
}

// bkScopeCluster returns the dbc row and the dbc-snap row the fetchers build
// for one engine.
func bkScopeCluster(t *testing.T, engine string) (cluster, snapshot resource.Resource) {
	t.Helper()
	ctx := context.Background()
	var clusters, snaps resource.FetchResult
	var err1, err2 error
	if engine == "docdb" {
		clusters, err1 = awsclient.FetchDocDBClustersPage(ctx, bkScopeDocDBFake{}, "")
		snaps, err2 = awsclient.FetchDocDBClusterSnapshotsPage(ctx, bkScopeDocDBFake{}, "")
	} else {
		fake := &bkScopeRDSFake{engine: engine}
		clusters, err1 = awsclient.FetchRDSDBClustersPage(ctx, fake, "")
		snaps, err2 = awsclient.FetchRDSDBClusterSnapshotsPage(ctx, fake, "")
	}
	if err1 != nil || err2 != nil || len(clusters.Resources) != 1 || len(snaps.Resources) != 1 {
		t.Fatalf("%s: %d cluster rows (%v), %d snapshot rows (%v); want 1 each", engine, len(clusters.Resources), err1, len(snaps.Resources), err2)
	}
	return clusters.Resources[0], snaps.Resources[0]
}

func TestBackupCoverageJoin_ClusterEngineOptIn(t *testing.T) {
	allClusters := backuptypes.BackupSelection{Resources: []string{"arn:aws:rds:*:*:cluster:*"}}
	exact := backuptypes.BackupSelection{Resources: []string{bkScopeClusterARN}}
	for engine, own := range bkScopeClusterOptInType {
		row, _ := bkScopeCluster(t, engine)
		cases := []struct {
			name  string
			plan  func(t *testing.T) resource.Resource
			warn  bool
			check string
		}{
			{"every cluster, engine type opted out", func(t *testing.T) resource.Resource {
				return bkScopePlanOptIn(t, bkScopeOptedOut(own), allClusters)
			}, true, ""},
			{"every cluster, another cluster type opted out", func(t *testing.T) resource.Resource {
				return bkScopePlanOptIn(t, bkScopeOptedOut(bkScopeOtherClusterType(engine)), allClusters)
			}, false, ""},
			{"exact cluster ARN, engine type opted out", func(t *testing.T) resource.Resource {
				return bkScopePlanOptIn(t, bkScopeOptedOut(own), exact)
			}, true, ""},
			{"exact cluster ARN, another cluster type opted out", func(t *testing.T) resource.Resource {
				return bkScopePlanOptIn(t, bkScopeOptedOut(bkScopeOtherClusterType(engine)), exact)
			}, false, ""},
			{"every cluster, settings unread", func(t *testing.T) resource.Resource {
				return bkScopePlanSettingsDenied(t, allClusters)
			}, false, "DescribeRegionSettings"},
			{"exact cluster ARN, settings unread", func(t *testing.T) resource.Resource {
				return bkScopePlanSettingsDenied(t, exact)
			}, false, "DescribeRegionSettings"},
		}
		for _, tc := range cases {
			t.Run(engine+"/"+tc.name, func(t *testing.T) {
				res, err := awsclient.EnrichDBCMaintenance(context.Background(), &awsclient.ServiceClients{},
					[]resource.Resource{row}, bk551Cache(tc.plan(t)))
				if err != nil && res.Findings == nil {
					t.Fatalf("EnrichDBCMaintenance: %v", err)
				}
				if got := hasCode(res.Findings[row.ID], awsclient.CodeDBCNotInBackupPlan); got != tc.warn {
					t.Errorf("dbc.not-in-backup-plan raised = %v, want %v", got, tc.warn)
				} else if tc.warn {
					w4AssertFinding(t, res.Findings[row.ID], awsclient.CodeDBCNotInBackupPlan,
						"not covered by a backup plan", domain.SevWarn, "wave2")
				}
				if got := res.TruncatedIDs[row.ID]; got != tc.check {
					t.Errorf("TruncatedIDs[%s] = %q, want %q", row.ID, got, tc.check)
				}
			})
		}
	}
}

func TestBackupDBCSnapPivot_ClusterEngineOptIn(t *testing.T) {
	allClusters := backuptypes.BackupSelection{Resources: []string{"arn:aws:rds:*:*:cluster:*"}}
	exact := backuptypes.BackupSelection{Resources: []string{bkScopeClusterARN}}
	for engine, own := range bkScopeClusterOptInType {
		t.Run(engine, func(t *testing.T) {
			cluster, snap := bkScopeCluster(t, engine)
			p := bk551Pivot{
				short: "dbc-snap",
				res:   snap,
				cache: resource.ResourceCache{"dbc": resource.ResourceCacheEntry{Resources: []resource.Resource{cluster}}},
			}

			got := bk551RunPivot(t, p, bkScopePlanOptIn(t, bkScopeOptedOut(own), allClusters))
			bk551AssertPlans(t, got, nil)

			got = bk551RunPivot(t, p, bkScopePlanOptIn(t, bkScopeOptedOut(bkScopeOtherClusterType(engine)), allClusters))
			bk551AssertPlans(t, got, []string{bk551PlanID})

			got = bk551RunPivot(t, p, bkScopePlanOptIn(t, bkScopeOptedOut(own), exact))
			bk551AssertPlans(t, got, nil)

			got = bk551RunPivot(t, p, bkScopePlanOptIn(t, bkScopeOptedOut(bkScopeOtherClusterType(engine)), exact))
			bk551AssertPlans(t, got, []string{bk551PlanID})

			for name, sel := range map[string]backuptypes.BackupSelection{"every cluster": allClusters, "exact cluster ARN": exact} {
				got = bk551RunPivot(t, p, bkScopePlanSettingsDenied(t, sel))
				if got.State() != domain.RelatedUnknown {
					t.Errorf("%s, settings unread: State = %v (plans %v), want unknown", name, got.State(), got.ResourceIDs())
				}
			}
		})
	}
}

func TestBackupCoverageJoin_ClusterMemberNamedClusterOptIn(t *testing.T) {
	exact := backuptypes.BackupSelection{Resources: []string{bkScopeClusterARN}}
	optInType := map[string]string{"aurora-postgresql": "Aurora", "docdb": "DocumentDB", "neptune": "Neptune"}
	for engine, own := range optInType {
		for _, tc := range []struct {
			name  string
			optIn map[string]bool
			warn  bool
		}{
			{"engine type opted out", bkScopeOptedOut(own), true},
			{"engine type opted in", bkScopeOptedOut("DynamoDB"), false},
		} {
			t.Run(engine+"/"+tc.name, func(t *testing.T) {
				fake := &bkScopeRDSFake{engine: engine}
				res := bkScopeEnrichDBI(t, fake, bkScopePlanOptIn(t, tc.optIn, exact))
				if got := hasCode(res.Findings[bkScopeMemberID], awsclient.CodeDBINotInBackupPlan); got != tc.warn {
					t.Errorf("%s: dbi.not-in-backup-plan raised = %v, want %v", bkScopeMemberID, got, tc.warn)
				} else if tc.warn {
					w4AssertFinding(t, res.Findings[bkScopeMemberID], awsclient.CodeDBINotInBackupPlan,
						"not covered by a backup plan", domain.SevWarn, "wave2")
				}
				if check, ok := res.TruncatedIDs[bkScopeMemberID]; ok {
					t.Errorf("%s marked not inspected (%q), want a verdict", bkScopeMemberID, check)
				}
			})
		}
	}
}

// ── rows the join cannot decide ──────────────────────────────────────────

func TestBackupCoverageJoin_UndecidedRowsAreMarked(t *testing.T) {
	ctx := context.Background()
	byTag := backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")}}
	rdsFake := &bkScopeRDSFake{engine: "aurora-postgresql"}
	dbis, err := awsclient.FetchRDSInstancesPage(ctx, rdsFake, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage: %v", err)
	}
	var solo resource.Resource
	for _, r := range dbis.Resources {
		if r.ID == bkScopeSoloID {
			solo = r
		}
	}
	dbcs, err := awsclient.FetchRDSDBClustersPage(ctx, rdsFake, "")
	if err != nil || len(dbcs.Resources) != 1 {
		t.Fatalf("FetchRDSDBClustersPage = %d rows, %v; want 1", len(dbcs.Resources), err)
	}
	cluster := dbcs.Resources[0]
	table := bk551Pivots()[0].res

	unfilled := backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{{
		ConditionType: backuptypes.ConditionTypeStringequals,
		ConditionKey:  aws.String("backup"),
	}}}

	accountUnknown := &awsclient.ServiceClients{Region: bk551Region}
	store := session.NewIdentityStore()
	store.Set("", errors.New("operation error STS: GetCallerIdentity, https response error StatusCode: 403, AccessDenied"))
	accountUnknown.SetIdentityStore(store)

	cases := []struct {
		name      string
		enrich    awsclient.IssueEnricherFunc
		clients   *awsclient.ServiceClients
		row       resource.Resource
		sel       backuptypes.BackupSelection
		wantCheck string
	}{
		{"ebs volume ARN needs the account", awsclient.EnrichEBSVolumeStatus, accountUnknown,
			bk551Volume(nil), backuptypes.BackupSelection{Resources: []string{bk551VolumeARN}}, awsclient.CheckOwnAccountUnknown},
		{"ddb tags never read", awsclient.EnrichDynamoDBPITR, &awsclient.ServiceClients{}, table, byTag, "ListTagsOfResource"},
		{"dbi tags never read", awsclient.EnrichDBIMaintenance, &awsclient.ServiceClients{}, solo, byTag, "ListTagsForResource"},
		{"dbc tags never read", awsclient.EnrichDBCMaintenance, &awsclient.ServiceClients{}, cluster, byTag, "ListTagsForResource"},
		{"selection tag clause without a value", awsclient.EnrichEBSVolumeStatus, bk551Clients(),
			bk551Volume(map[string]string{"backup": "true"}), unfilled, "GetBackupSelection"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := tc.enrich(ctx, tc.clients, []resource.Resource{tc.row}, bkScopeEBSCache(bk551Plan(t, tc.sel)))
			if err != nil && res.Findings == nil {
				t.Fatalf("enricher: %v", err)
			}
			for _, f := range res.Findings[tc.row.ID] {
				if slices.Contains([]domain.FindingCode{
					awsclient.CodeEBSNotInBackupPlan, awsclient.CodeDDBNotInBackupPlan,
					awsclient.CodeDBINotInBackupPlan, awsclient.CodeDBCNotInBackupPlan,
				}, f.Code) {
					t.Errorf("finding %s raised on a row whose coverage could not be decided", f.Code)
				}
			}
			if got := res.TruncatedIDs[tc.row.ID]; got != tc.wantCheck {
				t.Errorf("TruncatedIDs[%s] = %q, want %q", tc.row.ID, got, tc.wantCheck)
			}
		})
	}
}

// ── a declared backup list restored from disk ────────────────────────────

type bkScopeDeniedBackup struct{ awsclient.BackupAPI }

func (bkScopeDeniedBackup) ListBackupPlans(context.Context, *backup.ListBackupPlansInput, ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return nil, &smithy.OperationError{ServiceID: "Backup", OperationName: "ListBackupPlans", Err: &smithy.GenericAPIError{
		Code: "AccessDeniedException", Message: "not authorized to perform: backup:ListBackupPlans",
	}}
}

// A row restored from the disk cache carries Fields and no RawStruct, so a
// plan row from disk has no selections to evaluate; coverage is judged from a
// live read of the plan list, as for a list the session never observed.
func TestWave2DeclaredRead_DiskSeededBackupListIsFetchedLive(t *testing.T) {
	codes := map[string]domain.FindingCode{
		"ddb": awsclient.CodeDDBNotInBackupPlan,
		"ebs": awsclient.CodeEBSNotInBackupPlan,
		"dbi": awsclient.CodeDBINotInBackupPlan,
		"dbc": awsclient.CodeDBCNotInBackupPlan,
	}
	incomplete := "backup list incomplete"

	run := func(t *testing.T, short string, backupOrigin session.Origin, denied bool) (runtime.ProbeEnrichmentResult, []resource.Resource) {
		t.Helper()
		clients := demo.NewServiceClients()
		td := resource.FindResourceType(short)
		btd := resource.FindResourceType("backup")
		if td == nil || btd == nil {
			t.Fatalf("%s or backup is not registered", short)
		}
		rows, ok := DrainFixtures(t, *td, clients)
		if !ok {
			t.Fatalf("the %s demo fixtures drained no rows", short)
		}
		plans, ok := DrainFixtures(t, *btd, clients)
		if !ok {
			t.Fatal("the backup demo fixtures drained no rows")
		}
		if backupOrigin == session.OriginDisk {
			for i := range plans {
				plans[i].RawStruct = nil
			}
		}
		if denied {
			clients.Backup = bkScopeDeniedBackup{clients.Backup}
		}
		sess := session.New()
		sess.Clients = clients
		core := runtime.New(sess, catalog.All())
		core.Session().RowStore.Observe("backup", plans,
			&resource.PaginationMeta{PageSize: len(plans), TotalHint: len(plans)}, backupOrigin, false)
		core.Session().RowStore.Observe(short, rows, nil, session.OriginProbe, false)
		return core.ProbeEnrichment(context.Background(), clients, short), rows
	}
	flagged := func(res runtime.ProbeEnrichmentResult, code domain.FindingCode) []string {
		var ids []string
		for id, fs := range res.Findings {
			for _, f := range fs {
				if f.Code == code {
					ids = append(ids, id)
				}
			}
		}
		slices.Sort(ids)
		return ids
	}

	for short, code := range codes {
		t.Run(short, func(t *testing.T) {
			live, _ := run(t, short, session.OriginProbe, false)
			want := flagged(live, code)
			if len(want) == 0 {
				t.Fatalf("no demo %s row raises %s from the live plan list, so the comparison proves nothing", short, code)
			}

			disk, rows := run(t, short, session.OriginDisk, false)
			var marked []string
			for _, r := range rows {
				if disk.TruncatedIDs[r.ID] == incomplete {
					marked = append(marked, r.ID)
				}
			}
			if len(marked) > 0 {
				t.Errorf("%d of %d %s rows marked %q with a readable plan list seeded from disk: %v", len(marked), len(rows), short, incomplete, marked)
			}
			if got := flagged(disk, code); !slices.Equal(got, want) {
				t.Errorf("%s raised on %v with the plan list seeded from disk, want %v as from the live list", code, got, want)
			}

			refused, rows := run(t, short, session.OriginDisk, true)
			refusedMarks := 0
			for _, r := range rows {
				if refused.TruncatedIDs[r.ID] == incomplete {
					refusedMarks++
				}
			}
			if refusedMarks == 0 {
				t.Errorf("the live plan read was refused, yet no %s row is marked %q: %v", short, incomplete, refused.TruncatedIDs)
			}
			if got := flagged(refused, code); len(got) != 0 {
				t.Errorf("%s raised on %v from plan rows that carry no selections", code, got)
			}
		})
	}
}
