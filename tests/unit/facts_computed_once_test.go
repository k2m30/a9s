package unit_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// --- SSM plaintext classifier ----------------------------------------------

type factsSSMFake struct{ params []ssmtypes.ParameterMetadata }

func (f factsSSMFake) DescribeParameters(_ context.Context, _ *ssm.DescribeParametersInput, _ ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	return &ssm.DescribeParametersOutput{Parameters: f.params}, nil
}

func factsParam(name string, typ ssmtypes.ParameterType, modified time.Time) ssmtypes.ParameterMetadata {
	return ssmtypes.ParameterMetadata{
		Name:             aws.String(name),
		ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter" + name),
		Type:             typ,
		Version:          4,
		Tier:             ssmtypes.ParameterTierStandard,
		DataType:         aws.String("text"),
		LastModifiedDate: aws.Time(modified),
		LastModifiedUser: aws.String("arn:aws:iam::123456789012:user/deploy-bot"),
	}
}

// TestSSMStatusCellAndColourComeFromOneClassifier pins that an SSM parameter
// is judged once: whatever decides "this plaintext parameter holds a
// credential" or "this value is stale" decides it for the Status cell, the
// finding and the row colour together. A parameter shows either its finding's
// phrase in that finding's colour, or no problem word at all — including the
// names two rules can read differently ("/password" as a path segment against
// "_password" as a suffix) and a stale String against a stale SecureString.
func TestSSMStatusCellAndColourComeFromOneClassifier(t *testing.T) {
	recent := time.Now().Add(-30 * 24 * time.Hour)
	stale := time.Now().Add(-2 * 365 * 24 * time.Hour)
	out, err := awsclient.FetchSSMParametersPage(context.Background(), factsSSMFake{params: []ssmtypes.ParameterMetadata{
		factsParam("/prod/orders/db/password", ssmtypes.ParameterTypeString, recent),
		factsParam("/prod/orders/db_password", ssmtypes.ParameterTypeString, recent),
		factsParam("/prod/orders/stripe/token", ssmtypes.ParameterTypeString, recent),
		factsParam("/prod/orders/log_level", ssmtypes.ParameterTypeString, recent),
		factsParam("/prod/orders/feature_flags", ssmtypes.ParameterTypeString, stale),
		factsParam("/prod/orders/signing_secret", ssmtypes.ParameterTypeSecureString, stale),
		factsParam("/prod/orders/api_token", ssmtypes.ParameterTypeSecureString, recent),
	}}, "")
	if err != nil {
		t.Fatalf("FetchSSMParametersPage: %v", err)
	}
	td := catalog.FindAny("ssm")
	if td == nil {
		t.Fatal("ssm is not a registered type")
	}
	problemWords := map[string]bool{}
	for _, def := range td.Findings {
		problemWords[def.Phrase] = true
	}
	for _, w := range []string{"plaintext", "stale"} {
		problemWords[w] = true
	}

	byType, _ := buildVisibilityTypeCache(t)
	demoRows := byType["ssm"]
	if len(demoRows) == 0 {
		t.Fatal("the demo bench has no ssm rows")
	}

	for _, rows := range [][]resource.Resource{out.Resources, demoRows} {
		for _, row := range rows {
			cell, ok := listStatusCellFor(t, *td, rows, row.ID)
			if !ok {
				t.Fatalf("%s: no Status cell rendered", row.ID)
			}
			top, has := domain.TopFinding(row.Findings)
			if !has {
				if problemWords[strings.TrimSpace(cell)] {
					t.Errorf("%s: Status cell %q names a problem, but the row carries no finding and is coloured %v",
						row.ID, cell, td.Color(row))
				}
				continue
			}
			if cell != top.Phrase {
				t.Errorf("%s: Status cell = %q, want the finding's phrase %q", row.ID, cell, top.Phrase)
			}
			if got, want := td.Color(row), top.Severity.Color(); got != want {
				t.Errorf("%s: row colour = %v, want %v (the colour of its top finding %s)", row.ID, got, want, top.Code)
			}
		}
	}
	flagged := 0
	for _, row := range out.Resources {
		if len(row.Findings) > 0 {
			flagged++
		}
	}
	if flagged == 0 || flagged == len(out.Resources) {
		t.Errorf("%d of %d parameters flagged: the bench needs both flagged and clean rows to falsify the rule", flagged, len(out.Resources))
	}
}

// --- DB cluster status findings --------------------------------------------

type factsDocDBFake struct{ clusters []docdbtypes.DBCluster }

func (f factsDocDBFake) DescribeDBClusters(_ context.Context, _ *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{DBClusters: f.clusters}, nil
}

type factsRDSFake struct{ clusters []rdstypes.DBCluster }

func (f factsRDSFake) DescribeDBClusters(_ context.Context, _ *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{DBClusters: f.clusters}, nil
}

type factsCluster struct {
	status                       *string
	writer, protected, encrypted bool
	retention                    int32
}

func factsFindingsOf(t *testing.T, label string, fetch func() (resource.FetchResult, error)) (lines []string) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("%s: fetching the cluster panicked: %v", label, p)
			lines = nil
		}
	}()
	out, err := fetch()
	if err != nil {
		t.Errorf("%s: fetch error: %v", label, err)
		return nil
	}
	if len(out.Resources) != 1 {
		t.Errorf("%s: %d rows, want 1", label, len(out.Resources))
		return nil
	}
	for _, f := range out.Resources[0].Findings {
		lines = append(lines, fmt.Sprintf("%s %q %v", f.Code, f.Phrase, f.Severity))
	}
	return lines
}

// TestDBClusterFindingsAreTheSameThroughBothSDKs pins that a DB cluster's
// status findings are one computation: DescribeDBClusters answers through the
// DocumentDB and the RDS SDK alike, and a cluster with the same status,
// members and settings gets the same findings, in the same order, whichever
// answered — including a cluster AWS reported without a status.
func TestDBClusterFindingsAreTheSameThroughBothSDKs(t *testing.T) {
	cases := map[string]factsCluster{
		"available":                           {status: aws.String("available"), writer: true, protected: true, encrypted: true, retention: 7},
		"available, no writer":                {status: aws.String("available"), protected: true, encrypted: true, retention: 7},
		"available, unprotected":              {status: aws.String("available"), writer: true, retention: 0},
		"failed":                              {status: aws.String("failed"), writer: true, protected: true, encrypted: true, retention: 7},
		"inaccessible-encryption-credentials": {status: aws.String("inaccessible-encryption-credentials"), writer: true, protected: true, encrypted: true, retention: 7},
		"incompatible-parameters":             {status: aws.String("incompatible-parameters"), writer: true, protected: true, encrypted: true, retention: 7},
		"backing-up":                          {status: aws.String("backing-up"), writer: true, protected: true, encrypted: true, retention: 7},
		"deleting":                            {status: aws.String("deleting"), writer: true, retention: 0},
		"stopped":                             {status: aws.String("stopped"), writer: true, protected: true, encrypted: true, retention: 7},
		"storage-optimization":                {status: aws.String("storage-optimization"), writer: true, protected: true, encrypted: true, retention: 7},
		"no status reported":                  {writer: true, protected: true, encrypted: true, retention: 7},
	}
	const id = "acme-orders-cluster"
	arn := "arn:aws:rds:us-east-1:123456789012:cluster:" + id
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			doc := docdbtypes.DBCluster{
				DBClusterIdentifier:   aws.String(id),
				DBClusterArn:          aws.String(arn),
				Engine:                aws.String("aurora-postgresql"),
				EngineVersion:         aws.String("16.4"),
				Status:                c.status,
				MultiAZ:               aws.Bool(true),
				MasterUsername:        aws.String("orders_admin"),
				DeletionProtection:    aws.Bool(c.protected),
				StorageEncrypted:      aws.Bool(c.encrypted),
				BackupRetentionPeriod: aws.Int32(c.retention),
				Endpoint:              aws.String(id + ".cluster-c1a2b3c4d5e6.us-east-1.rds.amazonaws.com"),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String(id + "-1"), IsClusterWriter: aws.Bool(c.writer)},
					{DBInstanceIdentifier: aws.String(id + "-2"), IsClusterWriter: aws.Bool(false)},
				},
			}
			rdsCluster := rdstypes.DBCluster{
				DBClusterIdentifier:   doc.DBClusterIdentifier,
				DBClusterArn:          doc.DBClusterArn,
				Engine:                doc.Engine,
				EngineVersion:         doc.EngineVersion,
				Status:                doc.Status,
				MultiAZ:               doc.MultiAZ,
				MasterUsername:        doc.MasterUsername,
				DeletionProtection:    doc.DeletionProtection,
				StorageEncrypted:      doc.StorageEncrypted,
				BackupRetentionPeriod: doc.BackupRetentionPeriod,
				Endpoint:              doc.Endpoint,
				DBClusterMembers: []rdstypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String(id + "-1"), IsClusterWriter: aws.Bool(c.writer)},
					{DBInstanceIdentifier: aws.String(id + "-2"), IsClusterWriter: aws.Bool(false)},
				},
			}
			viaDocDB := factsFindingsOf(t, "docdb", func() (resource.FetchResult, error) {
				return awsclient.FetchDocDBClustersPage(context.Background(), factsDocDBFake{clusters: []docdbtypes.DBCluster{doc}}, "")
			})
			viaRDS := factsFindingsOf(t, "rds", func() (resource.FetchResult, error) {
				return awsclient.FetchRDSDBClustersPage(context.Background(), factsRDSFake{clusters: []rdstypes.DBCluster{rdsCluster}}, "")
			})
			if strings.Join(viaDocDB, "\n") != strings.Join(viaRDS, "\n") {
				t.Errorf("findings differ by SDK:\n  docdb: %v\n  rds:   %v", viaDocDB, viaRDS)
			}
		})
	}
}

// --- launch template version default ---------------------------------------

// An Auto Scaling group or an EKS node group that names a launch template
// without a version launches from the template's default version: AWS
// documents "$Default" as the default of LaunchTemplateSpecification.Version
// for Auto Scaling, and EKS uses "the template's default version" when none
// is given. A template whose latest version (a canary) is not its default
// tells the two apart.
const (
	factsLTID        = "lt-0a1b2c3d4e5f60718"
	factsDefaultAMI  = "ami-0defa0000000000003"
	factsLatestAMI   = "ami-0cana0000000000005"
	factsDefaultSG   = "sg-0defa000000000003"
	factsLatestSG    = "sg-0cana000000000005"
	factsDefaultRole = "acme-web-node"
	factsLatestRole  = "acme-web-canary-node"
)

func factsLaunchTemplateEC2() *fakeEC2ForASG {
	version := func(n int64, isDefault bool, ami, sg, profile string) ec2types.LaunchTemplateVersion {
		return ec2types.LaunchTemplateVersion{
			LaunchTemplateId:   aws.String(factsLTID),
			LaunchTemplateName: aws.String("acme-web"),
			VersionNumber:      aws.Int64(n),
			DefaultVersion:     aws.Bool(isDefault),
			LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
				ImageId:          aws.String(ami),
				InstanceType:     ec2types.InstanceTypeM6iLarge,
				SecurityGroupIds: []string{sg},
				IamInstanceProfile: &ec2types.LaunchTemplateIamInstanceProfileSpecification{
					Arn: aws.String("arn:aws:iam::123456789012:instance-profile/" + profile),
				},
			},
		}
	}
	byVersion := map[string]ec2types.LaunchTemplateVersion{
		"$Default": version(3, true, factsDefaultAMI, factsDefaultSG, factsDefaultRole+"-profile"),
		"3":        version(3, true, factsDefaultAMI, factsDefaultSG, factsDefaultRole+"-profile"),
		"$Latest":  version(5, false, factsLatestAMI, factsLatestSG, factsLatestRole+"-profile"),
		"5":        version(5, false, factsLatestAMI, factsLatestSG, factsLatestRole+"-profile"),
	}
	return &fakeEC2ForASG{
		describeLaunchTemplateVersionsFn: func(in *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			var out []ec2types.LaunchTemplateVersion
			for _, v := range in.Versions {
				if lv, ok := byVersion[v]; ok {
					out = append(out, lv)
				}
			}
			return &ec2.DescribeLaunchTemplateVersionsOutput{LaunchTemplateVersions: out}, nil
		},
	}
}

func factsIAM() *fakeIAMForASG {
	return &fakeIAMForASG{getInstanceProfileFn: func(in *iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
		role := strings.TrimSuffix(aws.ToString(in.InstanceProfileName), "-profile")
		return &iam.GetInstanceProfileOutput{InstanceProfile: &iamtypes.InstanceProfile{
			InstanceProfileName: in.InstanceProfileName,
			Roles:               []iamtypes.Role{{RoleName: aws.String(role), Arn: aws.String("arn:aws:iam::123456789012:role/" + role)}},
		}}, nil
	}}
}

func factsASG(name string, version *string, mixed bool) resource.Resource {
	spec := &asgtypes.LaunchTemplateSpecification{LaunchTemplateId: aws.String(factsLTID), LaunchTemplateName: aws.String("acme-web"), Version: version}
	asg := asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String(name),
		AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:6f1c2a3b-4d5e-6f70-8192-a3b4c5d6e7f8:autoScalingGroupName/" + name),
		MinSize:              aws.Int32(2),
		MaxSize:              aws.Int32(6),
		DesiredCapacity:      aws.Int32(3),
		VPCZoneIdentifier:    aws.String("subnet-0aaa000000000001,subnet-0bbb000000000002"),
	}
	if mixed {
		asg.MixedInstancesPolicy = &asgtypes.MixedInstancesPolicy{LaunchTemplate: &asgtypes.LaunchTemplate{LaunchTemplateSpecification: spec}}
	} else {
		asg.LaunchTemplate = spec
	}
	return resource.Resource{ID: name, Name: name, Fields: map[string]string{}, RawStruct: asg}
}

// TestLaunchTemplateWithoutVersionResolvesTheDefaultVersion pins that every
// pivot reading a launch template reads the version the group launches from:
// the default one when the group names none, and the named one otherwise.
func TestLaunchTemplateWithoutVersionResolvesTheDefaultVersion(t *testing.T) {
	ctx := context.Background()
	clients := &awsclient.ServiceClients{EC2: factsLaunchTemplateEC2(), IAM: factsIAM(), AutoScaling: &fakeASGChecker{}}

	for _, tc := range []struct {
		name    string
		asg     resource.Resource
		ami, sg string
		role    string
	}{
		{"asg without version", factsASG("acme-web-asg", nil, false), factsDefaultAMI, factsDefaultSG, factsDefaultRole},
		{"asg with empty version", factsASG("acme-web-asg", aws.String(""), false), factsDefaultAMI, factsDefaultSG, factsDefaultRole},
		{"mixed-instances asg without version", factsASG("acme-web-spot-asg", nil, true), factsDefaultAMI, factsDefaultSG, factsDefaultRole},
		{"asg pinned to $Latest", factsASG("acme-web-canary-asg", aws.String("$Latest"), false), factsLatestAMI, factsLatestSG, factsLatestRole},
	} {
		assertSameIDs(t, tc.name+" -> ami", checkerByTarget(t, "asg", "ami")(ctx, clients, tc.asg, nil).ResourceIDs(), []string{tc.ami})
		assertSameIDs(t, tc.name+" -> sg", checkerByTarget(t, "asg", "sg")(ctx, clients, tc.asg, nil).ResourceIDs(), []string{tc.sg})
		roles := checkerByTarget(t, "asg", "role")(ctx, clients, tc.asg, nil).ResourceIDs()
		other := map[string]string{factsDefaultRole: factsLatestRole, factsLatestRole: factsDefaultRole}[tc.role]
		if !slices.Contains(roles, tc.role) || slices.Contains(roles, other) {
			t.Errorf("%s -> role: ResourceIDs = %v, want %s and not the other version's role", tc.name, roles, tc.role)
		}
	}

	ng := func(version *string) resource.Resource {
		return resource.Resource{
			ID: "general-pool", Name: "general-pool", Fields: map[string]string{"cluster_name": "acme-prod"},
			RawStruct: ekstypes.Nodegroup{
				NodegroupName:  aws.String("general-pool"),
				ClusterName:    aws.String("acme-prod"),
				LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Id: aws.String(factsLTID), Name: aws.String("acme-web"), Version: version},
			},
		}
	}
	assertSameIDs(t, "ng without version -> ami", checkerByTarget(t, "ng", "ami")(ctx, clients, ng(nil), nil).ResourceIDs(), []string{factsDefaultAMI})
	assertSameIDs(t, "ng pinned to 5 -> ami", checkerByTarget(t, "ng", "ami")(ctx, clients, ng(aws.String("5")), nil).ResourceIDs(), []string{factsLatestAMI})

	eksClients := &awsclient.ServiceClients{EC2: factsLaunchTemplateEC2(), EKS: newFakeEKSWithNodegroups([]string{"general-pool"}, map[string]*ekstypes.Nodegroup{
		"general-pool": {
			NodegroupName:  aws.String("general-pool"),
			ClusterName:    aws.String("acme-services"),
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Id: aws.String(factsLTID), Name: aws.String("acme-web")},
		},
	})}
	assertSameIDs(t, "eks acme-services (node group without version) -> ami",
		checkerByTarget(t, "eks", "ami")(ctx, eksClients, eksClusterSrcResource(), nil).ResourceIDs(), []string{factsDefaultAMI})
}
