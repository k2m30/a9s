package unit_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ecsIncludeFake answers DescribeClusters the way ECS does: configuration
// and tags are returned only when the caller names them in Include.
type ecsIncludeFake struct {
	clusters []ecstypes.Cluster
}

func (f *ecsIncludeFake) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	out := &ecs.ListClustersOutput{}
	for _, c := range f.clusters {
		out.ClusterArns = append(out.ClusterArns, *c.ClusterArn)
	}
	return out, nil
}

func (f *ecsIncludeFake) DescribeClusters(_ context.Context, in *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	out := &ecs.DescribeClustersOutput{}
	for _, c := range f.clusters {
		if !slices.Contains(in.Clusters, *c.ClusterArn) {
			continue
		}
		if !slices.Contains(in.Include, ecstypes.ClusterFieldConfigurations) {
			c.Configuration = nil
		}
		if !slices.Contains(in.Include, ecstypes.ClusterFieldTags) {
			c.Tags = nil
		}
		out.Clusters = append(out.Clusters, c)
	}
	return out, nil
}

const (
	covKMSKeyID  = "1234abcd-12ab-34cd-56ef-1234567890ab"
	covKMSKeyARN = "arn:aws:kms:us-east-1:123456789012:key/" + covKMSKeyID
)

func covECSClusters() []ecstypes.Cluster {
	return []ecstypes.Cluster{
		{
			ClusterName:       aws.String("acme-orders"),
			ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-orders"),
			Status:            aws.String("ACTIVE"),
			CapacityProviders: []string{"FARGATE", "FARGATE_SPOT"},
			RunningTasksCount: 6,
			Configuration: &ecstypes.ClusterConfiguration{
				ExecuteCommandConfiguration: &ecstypes.ExecuteCommandConfiguration{
					KmsKeyId: aws.String(covKMSKeyARN),
					Logging:  ecstypes.ExecuteCommandLoggingDefault,
				},
			},
			Tags: []ecstypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-orders-stack")},
				{Key: aws.String("team"), Value: aws.String("orders")},
			},
		},
		{
			ClusterName:       aws.String("acme-batch"),
			ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-batch"),
			Status:            aws.String("ACTIVE"),
			CapacityProviders: []string{"FARGATE"},
			Configuration: &ecstypes.ClusterConfiguration{
				ExecuteCommandConfiguration: &ecstypes.ExecuteCommandConfiguration{
					Logging: ecstypes.ExecuteCommandLoggingDefault,
				},
			},
			Tags: []ecstypes.Tag{{Key: aws.String("team"), Value: aws.String("batch")}},
		},
	}
}

func covECSRows(t *testing.T) map[string]resource.Resource {
	t.Helper()
	page, err := awsclient.FetchECSClustersPage(context.Background(), &ecsIncludeFake{clusters: covECSClusters()}, &ecsIncludeFake{clusters: covECSClusters()}, "")
	if err != nil {
		t.Fatalf("FetchECSClustersPage: %v", err)
	}
	rows := map[string]resource.Resource{}
	for _, r := range page.Resources {
		r.Type = "ecs"
		rows[r.ID] = r
	}
	return rows
}

// The cluster's execute-command key and its CloudFormation tag are in the
// account; a row that never asked ECS for them has not searched for them, so
// it may not report a proven zero. A cluster whose configuration and tags
// were read and carry neither is a proven zero.
func TestRelatedCoverage_ECSFieldsNeverRequested(t *testing.T) {
	rows := covECSRows(t)
	cache := resource.ResourceCache{
		"kms": {Resources: []resource.Resource{{ID: covKMSKeyID, Name: covKMSKeyID, Type: "kms", Fields: map[string]string{"key_id": covKMSKeyID, "arn": covKMSKeyARN}}}},
		"cfn": {Resources: []resource.Resource{{ID: "acme-orders-stack", Name: "acme-orders-stack", Type: "cfn", Fields: map[string]string{"stack_name": "acme-orders-stack", "status": "CREATE_COMPLETE"}}}},
	}
	for _, target := range []string{"kms", "cfn"} {
		check := refChecker(t, "ecs", target)
		t.Run(target+"/configured cluster", func(t *testing.T) {
			r := check(context.Background(), refClients(), rows["acme-orders"], cache)
			if r.State() == domain.RelatedResolved && r.Count() == 0 && r.Coverage() == resource.CoverageComplete {
				t.Errorf("ecs acme-orders → %s: complete zero, but the cluster carries the reference in a field the fetch never requested", target)
			}
		})
		t.Run(target+"/cluster without the reference", func(t *testing.T) {
			r := check(context.Background(), refClients(), rows["acme-batch"], cache)
			if r.State() != domain.RelatedResolved || r.Count() != 0 || r.Coverage() != resource.CoverageComplete {
				t.Errorf("ecs acme-batch → %s = (state %v, count %d, coverage %v), want a complete zero: its configuration and tags were read and name none",
					target, r.State(), r.Count(), r.Coverage())
			}
		})
	}
}

// A target list holding rows whose details could not be read is a partial
// list: the unreadable rows may be the ones that match, so the count is a
// lower bound, never a proven number.
func TestRelatedCoverage_DegradedTargetRowsMakeResultPartial(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/acme-eks-cluster-role"
	role := resource.Resource{
		ID:   "acme-eks-cluster-role",
		Name: "acme-eks-cluster-role",
		Type: "role",
		Fields: map[string]string{
			"role_name": "acme-eks-cluster-role",
			"arn":       roleARN,
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("acme-eks-cluster-role"),
			Arn:      aws.String(roleARN),
			Path:     aws.String("/"),
			RoleId:   aws.String("AROAEXAMPLEID0000001"),
		},
	}
	denied := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform: eks:DescribeCluster"}

	otherCluster := resource.Resource{
		ID: "acme-payments", Name: "acme-payments", Type: "eks",
		Fields:    map[string]string{"cluster_name": "acme-payments", "status": "ACTIVE"},
		RawStruct: ekstypes.Cluster{Name: aws.String("acme-payments"), RoleArn: aws.String("arn:aws:iam::123456789012:role/acme-payments-role"), Status: ekstypes.ClusterStatusActive},
	}
	matchCluster := resource.Resource{
		ID: "acme-orders", Name: "acme-orders", Type: "eks",
		Fields:    map[string]string{"cluster_name": "acme-orders", "status": "ACTIVE"},
		RawStruct: ekstypes.Cluster{Name: aws.String("acme-orders"), RoleArn: aws.String(roleARN), Status: ekstypes.ClusterStatusActive},
	}
	otherNG := resource.Resource{
		ID: "acme-payments/general", Name: "general", Type: "ng",
		Fields:    map[string]string{"nodegroup_name": "general", "cluster_name": "acme-payments", "status": "ACTIVE"},
		RawStruct: ekstypes.Nodegroup{NodegroupName: aws.String("general"), ClusterName: aws.String("acme-payments"), NodeRole: aws.String("arn:aws:iam::123456789012:role/acme-payments-node"), Status: ekstypes.NodegroupStatusActive},
	}
	matchNG := resource.Resource{
		ID: "acme-orders/general", Name: "general", Type: "ng",
		Fields:    map[string]string{"nodegroup_name": "general", "cluster_name": "acme-orders", "status": "ACTIVE"},
		RawStruct: ekstypes.Nodegroup{NodegroupName: aws.String("general"), ClusterName: aws.String("acme-orders"), NodeRole: aws.String(roleARN), Status: ekstypes.NodegroupStatusActive},
	}

	cases := []struct {
		name    string
		target  string
		rows    []resource.Resource
		wantIDs []string
		wantCov resource.RelatedCoverage
	}{
		{"eks: denied row, no readable match", "eks", []resource.Resource{otherCluster, awsclient.DegradedDetails("eks", "acme-staging", denied)}, nil, resource.CoveragePartial},
		{"eks: denied row beside a match", "eks", []resource.Resource{matchCluster, awsclient.DegradedDetails("eks", "acme-staging", denied)}, []string{"acme-orders"}, resource.CoveragePartial},
		{"eks: every row readable", "eks", []resource.Resource{otherCluster}, nil, resource.CoverageComplete},
		{"ng: unavailable row, no readable match", "ng", []resource.Resource{otherNG, awsclient.DegradedDetails("ng", "acme-staging/general", nil)}, nil, resource.CoveragePartial},
		{"ng: unavailable row beside a match", "ng", []resource.Resource{matchNG, awsclient.DegradedDetails("ng", "acme-staging/general", nil)}, []string{"acme-orders/general"}, resource.CoveragePartial},
		{"ng: every row readable", "ng", []resource.Resource{otherNG}, nil, resource.CoverageComplete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache := resource.ResourceCache{tc.target: {Resources: tc.rows}}
			r := refChecker(t, "role", tc.target)(context.Background(), refClients(), role, cache)
			if r.State() != domain.RelatedResolved {
				t.Fatalf("state = %v, want resolved", r.State())
			}
			if got := sortedIDs(r); !slices.Equal(got, tc.wantIDs) {
				t.Errorf("ids = %v, want %v", got, tc.wantIDs)
			}
			if r.Coverage() != tc.wantCov {
				t.Errorf("coverage = %v, want %v", r.Coverage(), tc.wantCov)
			}
		})
	}
}

// Each of these pivots matches by a shared property rather than a link AWS
// records: an RDS-managed ENI on a shared security group, the account-wide
// service-linked role, a secret whose name or tags mention CodeArtifact. Every match it offers on the demo bench
// is a heuristic match.
func TestRelatedCoverage_HeuristicPivotsOnDemoBench(t *testing.T) {
	b := newRefBench(t)
	for _, p := range []struct{ source, target string }{{"dbi", "eni"}, {"tgw", "role"}, {"secrets", "codeartifact"}} {
		t.Run(p.source+"/"+p.target, func(t *testing.T) {
			check := refChecker(t, p.source, p.target)
			var matched, wrong []string
			for _, row := range b.byType[p.source] {
				r := check(context.Background(), refClients(), row, b.cache)
				if len(r.ResourceIDs()) == 0 {
					continue
				}
				matched = append(matched, row.ID)
				if r.Coverage() != resource.CoverageHeuristic {
					wrong = append(wrong, row.ID)
				}
			}
			if len(matched) == 0 {
				t.Fatalf("no demo %s row matched any %s; the bench cannot show the heuristic", p.source, p.target)
			}
			if len(wrong) > 0 {
				t.Errorf("%d of %d matching %s rows answer %s without CoverageHeuristic, first %s", len(wrong), len(matched), p.source, p.target, wrong[0])
			}
		})
	}
}

// failingGetFunctionLambda answers every GetFunction with a throttle, the
// failed read after which the rotation Lambda's log group is only Lambda's
// default name, not a group anyone read.
type failingGetFunctionLambda struct {
	awsclient.LambdaAPI
}

func (failingGetFunctionLambda) GetFunction(_ context.Context, _ *lambdapkg.GetFunctionInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.GetFunctionOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "ThrottlingException", Message: "Rate exceeded"}
}

func TestRelatedCoverage_SecretsLogGroupAfterFailedRead(t *testing.T) {
	b := newRefBench(t)
	secret := b.row(t, "secrets", "prod/database/primary")
	check := refChecker(t, "secrets", "logs")

	t.Run("GetFunction failed", func(t *testing.T) {
		clients := refClients()
		clients.Lambda = failingGetFunctionLambda{LambdaAPI: clients.Lambda}
		r := check(context.Background(), clients, secret, b.cache)
		if got := sortedIDs(r); !slices.Equal(got, []string{"/aws/lambda/rotate-rds-credentials"}) {
			t.Errorf("ids = %v, want the default group [/aws/lambda/rotate-rds-credentials]", got)
		}
		if r.Coverage() != resource.CoverageHeuristic {
			t.Errorf("coverage = %v, want CoverageHeuristic: the log group was derived, not read", r.Coverage())
		}
	})
	t.Run("GetFunction answered", func(t *testing.T) {
		r := check(context.Background(), refClients(), secret, b.cache)
		if got := sortedIDs(r); !slices.Equal(got, []string{"/aws/lambda/rotate-rds-credentials"}) {
			t.Errorf("ids = %v, want [/aws/lambda/rotate-rds-credentials]", got)
		}
		if r.Coverage() != resource.CoverageComplete {
			t.Errorf("coverage = %v, want CoverageComplete: the function's logging config was read", r.Coverage())
		}
	})
}

// Glue jobs record no Athena workgroup, and CodePipeline has no CodeArtifact
// action provider, so neither pivot can discover anything: it is either not
// registered or answers NoDiscoveryPath for every row, never a proven zero.
func TestRelatedCoverage_NoDiscoveryPathPivotsOnDemoBench(t *testing.T) {
	b := newRefBench(t)
	for _, p := range []struct{ source, target string }{{"glue", "athena"}, {"pipeline", "codeartifact"}} {
		t.Run(p.source+"/"+p.target, func(t *testing.T) {
			var check resource.RelatedChecker
			for _, def := range resource.GetRelated(p.source) {
				if def.TargetType == p.target {
					check = def.Checker
				}
			}
			if check == nil {
				return
			}
			rows := b.byType[p.source]
			if len(rows) == 0 {
				t.Fatalf("demo bench has no %s rows", p.source)
			}
			for _, row := range rows {
				r := check(context.Background(), refClients(), row, b.cache)
				if r.Coverage() != resource.CoverageNoPath {
					t.Errorf("%s %s → %s = (state %v, count %d, coverage %v), want CoverageNoPath",
						p.source, row.ID, p.target, r.State(), r.Count(), r.Coverage())
				}
			}
		})
	}
}

// A heuristic scan that read the whole list and found no candidate has
// proven there is none: its row is the dead end a proven zero is, not a live
// row into the target type's whole list.
func TestRelatedCoverage_HeuristicPivotWithNoCandidatesIsAProvenZero(t *testing.T) {
	b := newRefBench(t)

	t.Run("secret that names no repository", func(t *testing.T) {
		src := b.row(t, "secrets", "prod/api/gateway-key")
		r := refChecker(t, "secrets", "codeartifact")(context.Background(), refClients(), src, b.cache)
		assertProvenZeroRow(t, src, "secrets", "codeartifact", r)
	})

	t.Run("secret that names one", func(t *testing.T) {
		src := b.row(t, "secrets", "prod/codeartifact/npm-publish-token")
		r := refChecker(t, "secrets", "codeartifact")(context.Background(), refClients(), src, b.cache)
		if got := sortedIDs(r); !slices.Equal(got, []string{"acme-artifacts/acme-npm"}) {
			t.Fatalf("ids = %v, want [acme-artifacts/acme-npm]", got)
		}
		if r.Coverage() != resource.CoverageHeuristic {
			t.Errorf("coverage = %v, want CoverageHeuristic", r.Coverage())
		}
		name := relatedDisplayName(t, "secrets", "codeartifact")
		blocks, rendered := renderRelatedPanel(t, src, map[string]resource.RelatedCheckResult{name: r})
		assertRelatedRow(t, blocks, rendered, name, "", true)
	})

	// An instance in a VPC that holds no target group: the VPC match rule
	// looked at every target group in the account and none can front it.
	t.Run("instance in a VPC with no target group", func(t *testing.T) {
		src := b.row(t, "ec2", "i-0a1b2c3d4e5f60001")
		const emptyVPC = "vpc-0f10a1b2c3d4e5f60"
		inst, ok := src.RawStruct.(ec2types.Instance)
		if !ok {
			t.Fatalf("demo ec2 row carries %T, want an ec2 Instance", src.RawStruct)
		}
		inst.VpcId = aws.String(emptyVPC)
		src.RawStruct = inst
		fields := make(map[string]string, len(src.Fields))
		maps.Copy(fields, src.Fields)
		fields["vpc_id"] = emptyVPC
		src.Fields = fields

		r := refChecker(t, "ec2", "tg")(context.Background(), refClients(), src, b.cache)
		assertProvenZeroRow(t, src, "ec2", "tg", r)
	})

	t.Run("every demo row with no candidate", func(t *testing.T) {
		for _, p := range []struct{ source, target string }{{"dbi", "eni"}, {"tgw", "role"}, {"secrets", "codeartifact"}} {
			check := refChecker(t, p.source, p.target)
			var wrong []string
			for _, row := range b.byType[p.source] {
				r := check(context.Background(), refClients(), row, b.cache)
				if len(r.ResourceIDs()) > 0 || r.State() != domain.RelatedResolved {
					continue
				}
				if r.Coverage() != resource.CoverageComplete || r.Truncated() {
					wrong = append(wrong, row.ID)
				}
			}
			if len(wrong) > 0 {
				t.Errorf("%s → %s: %d rows found no candidate and do not answer a proven zero, first %s", p.source, p.target, len(wrong), wrong[0])
			}
		}
	})
}

func assertProvenZeroRow(t *testing.T, src resource.Resource, source, target string, r resource.RelatedCheckResult) {
	t.Helper()
	if r.State() != domain.RelatedResolved || r.Count() != 0 || r.Truncated() {
		t.Fatalf("%s → %s = (state %v, count %d, truncated %v), want a resolved exact zero", source, target, r.State(), r.Count(), r.Truncated())
	}
	if r.Coverage() != resource.CoverageComplete {
		t.Errorf("coverage = %v, want CoverageComplete", r.Coverage())
	}
	name := relatedDisplayName(t, source, target)
	blocks, rendered := renderRelatedPanel(t, src, map[string]resource.RelatedCheckResult{name: r})
	assertRelatedRow(t, blocks, rendered, name, "(0)", false)
}
