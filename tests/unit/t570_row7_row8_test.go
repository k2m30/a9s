package unit_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	t570ENI          = "eni-0aaa111111111111a"
	t570SecondaryIP  = "10.0.1.25"
	t570SecondaryEIP = "eipalloc-0e570000000000025"
)

// t570ENIWithSecondaryIP gives one demo ENI a secondary private IP holding an
// Elastic IP, and the address that EIP is, as EC2 reports both.
func t570ENIWithSecondaryIP() *awsclient.ServiceClients {
	c := t570Demo()
	c.EC2 = &t570EC2{EC2API: c.EC2,
		enis: func(enis []ec2types.NetworkInterface) []ec2types.NetworkInterface {
			for i := range enis {
				if aws.ToString(enis[i].NetworkInterfaceId) != t570ENI {
					continue
				}
				primary := aws.ToString(enis[i].PrivateIpAddress)
				enis[i].PrivateIpAddresses = []ec2types.NetworkInterfacePrivateIpAddress{
					{PrivateIpAddress: aws.String(primary), Primary: aws.Bool(true)},
					{PrivateIpAddress: aws.String(t570SecondaryIP), Primary: aws.Bool(false), Association: &ec2types.NetworkInterfaceAssociation{
						PublicIp:      aws.String("203.0.113.25"),
						AllocationId:  aws.String(t570SecondaryEIP),
						AssociationId: aws.String("eipassoc-0e570000000000025"),
						IpOwnerId:     aws.String(t570Account),
					}},
				}
			}
			return enis
		},
		addresses: func(addrs []ec2types.Address) []ec2types.Address {
			return append(addrs, ec2types.Address{
				AllocationId:            aws.String(t570SecondaryEIP),
				AssociationId:           aws.String("eipassoc-0e570000000000025"),
				PublicIp:                aws.String("203.0.113.25"),
				Domain:                  ec2types.DomainTypeVpc,
				NetworkInterfaceId:      aws.String(t570ENI),
				NetworkInterfaceOwnerId: aws.String(t570Account),
				PrivateIpAddress:        aws.String(t570SecondaryIP),
				PublicIpv4Pool:          aws.String("amazon"),
				NetworkBorderGroup:      aws.String("us-east-1"),
			})
		},
	}
	return c
}

// The ENI detail shows every private IP the interface holds
// (NetworkInterface.PrivateIpAddresses), and the Elastic IP on a secondary IP
// is a link to that EIP like the primary's.
func TestT570_ENIDetail_RendersSecondaryIPsAndTheirEIP(t *testing.T) {
	c := t570ENIWithSecondaryIP()
	eni := t570Row(t, c, "eni", t570ENI)
	ctrl := newVisibilityDetailController(t)
	ctrl.EnsureDetailState(eni, "eni")
	body := ctrl.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("no detail body for the ENI")
	}
	var rendered []string
	secondaryShown, eipLinked := false, false
	for _, f := range body.Fields {
		rendered = append(rendered, f.Key+"="+f.Value)
		if strings.HasPrefix(f.Path, "PrivateIpAddresses") && strings.Contains(f.Value, t570SecondaryIP) {
			secondaryShown = true
		}
		if f.IsNavigable && f.TargetType == "eip" && (f.NavID == t570SecondaryEIP || f.Value == t570SecondaryEIP) {
			eipLinked = true
		}
	}
	if !secondaryShown {
		t.Errorf("detail does not render secondary IP %s under PrivateIpAddresses; rows %v", t570SecondaryIP, rendered)
	}
	if !eipLinked {
		t.Errorf("detail does not link the secondary IP's Elastic IP %s to eip; rows %v", t570SecondaryEIP, rendered)
	}
}

// An EIP associated with a secondary private IP names its ENI
// (Address.NetworkInterfaceId), so eip -> eni reaches it.
func TestT570_EIPENI_SecondaryIPAssociationReachesTheENI(t *testing.T) {
	c := t570ENIWithSecondaryIP()
	eip := t570Row(t, c, "eip", t570SecondaryEIP)
	t570Exact(t, t570Pivot(t, c, eip, "eip", "eni"), t570ENI)
}

type t570EBEnv struct {
	awsclient.ElasticBeanstalkAPI
	env, operationsRole string
}

func (f *t570EBEnv) DescribeEnvironments(ctx context.Context, in *elasticbeanstalk.DescribeEnvironmentsInput, opt ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
	out, err := f.ElasticBeanstalkAPI.DescribeEnvironments(ctx, in, opt...)
	if err != nil {
		return out, err
	}
	for i := range out.Environments {
		if aws.ToString(out.Environments[i].EnvironmentId) == f.env {
			out.Environments[i].OperationsRole = aws.String(f.operationsRole)
		}
	}
	return out, nil
}

// t570RoleNotIn is a demo role absent from ids.
func t570RoleNotIn(t *testing.T, c *awsclient.ServiceClients, ids []string) resource.Resource {
	t.Helper()
	for _, r := range t570List(t, c, "role") {
		if !slices.Contains(ids, r.ID) {
			return r
		}
	}
	t.Fatal("every demo role is already listed")
	return resource.Resource{}
}

// An environment's operations role (EnvironmentDescription.OperationsRole)
// is the role Elastic Beanstalk assumes to operate it.
func TestT570_EbRole_CountsTheOperationsRole(t *testing.T) {
	c := t570Demo()
	env := t570Row(t, c, "eb", "e-acmeprodapi")
	role := t570RoleNotIn(t, c, t570Pivot(t, c, env, "eb", "role").ResourceIDs())
	c.ElasticBeanstalk = &t570EBEnv{ElasticBeanstalkAPI: c.ElasticBeanstalk, env: "e-acmeprodapi", operationsRole: role.Fields["arn"]}
	env = t570Row(t, c, "eb", "e-acmeprodapi")
	t570Contains(t, t570Pivot(t, c, env, "eb", "role"), []string{role.ID}, nil)
}

type t570EKSCluster struct {
	awsclient.EKSAPI
	cluster  string
	nodeRole string
}

func (f *t570EKSCluster) DescribeCluster(ctx context.Context, in *eks.DescribeClusterInput, opt ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	out, err := f.EKSAPI.DescribeCluster(ctx, in, opt...)
	if err != nil || aws.ToString(in.Name) != f.cluster || out.Cluster == nil {
		return out, err
	}
	cl := *out.Cluster
	cl.ComputeConfig = &ekstypes.ComputeConfigResponse{Enabled: aws.Bool(true), NodePools: []string{"general-purpose", "system"}, NodeRoleArn: aws.String(f.nodeRole)}
	return &eks.DescribeClusterOutput{Cluster: &cl}, nil
}

// An EKS Auto Mode cluster launches its nodes with ComputeConfig.NodeRoleArn,
// a role of the cluster alongside Cluster.RoleArn.
func TestT570_EKSRole_CountsTheAutoModeNodeRole(t *testing.T) {
	c := t570Demo()
	cl := t570Row(t, c, "eks", "acme-prod")
	role := t570RoleNotIn(t, c, t570Pivot(t, c, cl, "eks", "role").ResourceIDs())
	c.EKS = &t570EKSCluster{EKSAPI: c.EKS, cluster: "acme-prod", nodeRole: role.Fields["arn"]}
	cl = t570Row(t, c, "eks", "acme-prod")
	t570Contains(t, t570Pivot(t, c, cl, "eks", "role"), []string{role.ID}, nil)
}

// DynamoDB Contributor Insights creates CloudWatch Contributor Insights
// rules, not log groups (DynamoDB Developer Guide, contributorinsights_HowItWorks).
// A log group that merely carries the table's name under /aws/dynamodb/ is
// not the table's.
func TestT570_DdbLogs_NoLogGroupByContributorInsightsName(t *testing.T) {
	registered := false
	for _, def := range resource.GetRelated("ddb") {
		registered = registered || def.TargetType == "logs"
	}
	if !registered {
		return
	}
	c := t570Demo()
	table := t570Row(t, c, "ddb", "orders-prod")
	const named = "/aws/dynamodb/tables/orders-prod/insights/default"
	t570Row(t, c, "logs", named)
	got := t570Pivot(t, c, table, "ddb", "logs")
	if slices.Contains(got.ResourceIDs(), named) {
		t.Errorf("ddb -> logs lists %s from its name alone; ids %v", named, got.ResourceIDs())
	}
}

// The secrets -> codeartifact pivot lands on CodeArtifact repositories, and
// its label says so.
func TestT570_SecretsCodeArtifact_LabelNamesRepositories(t *testing.T) {
	for _, def := range resource.GetRelated("secrets") {
		if def.TargetType == "codeartifact" {
			if def.DisplayName != "CodeArtifact Repositories" {
				t.Errorf("DisplayName = %q, want %q", def.DisplayName, "CodeArtifact Repositories")
			}
			return
		}
	}
	t.Fatal("no secrets -> codeartifact pivot registered")
}

type t570IAMEntities struct {
	awsclient.IAMAPI
	roles []string
}

func (f *t570IAMEntities) ListEntitiesForPolicy(ctx context.Context, in *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := &iam.ListEntitiesForPolicyOutput{}
	for _, r := range f.roles {
		out.PolicyRoles = append(out.PolicyRoles, iamtypes.PolicyRole{RoleName: aws.String(r), RoleId: aws.String("AROAEXAMPLE" + strings.ToUpper(r[:4]))})
	}
	return out, nil
}

// t570ManagedPolicy is an AWS-managed policy row as the policy list builds
// it, under an ARN no other test reads.
func t570ManagedPolicy(t *testing.T, name string) resource.Resource {
	t.Helper()
	row := t570List(t, t570Demo(), "policy")[0]
	arn := "arn:aws:iam::aws:policy/" + name
	row.ID = arn
	row.Name = name
	fields := map[string]string{}
	for k, v := range row.Fields {
		fields[k] = v
	}
	fields["arn"] = arn
	fields["policy_name"] = name
	row.Fields = fields
	if p, ok := row.RawStruct.(iamtypes.Policy); ok {
		p.Arn = aws.String(arn)
		p.PolicyName = aws.String(name)
		row.RawStruct = p
	}
	return row
}

// Who a policy is attached to belongs to one account: after a profile
// switch, the same AWS-managed policy ARN answers from the new account.
func TestT570_PolicyEntities_DoNotOutliveAProfileSwitch(t *testing.T) {
	policy := t570ManagedPolicy(t, "T570ReadOnlyAccessProfileSwitch")
	first := t570Demo()
	first.IAM = &t570IAMEntities{IAMAPI: first.IAM, roles: []string{"acme-eks-node-role"}}
	t570Exact(t, t570Pivot(t, first, policy, "policy", "role"), "acme-eks-node-role")

	second := t570Demo()
	second.IAM = &t570IAMEntities{IAMAPI: second.IAM, roles: []string{"acme-ci-deploy-role"}}
	t570Exact(t, t570Pivot(t, second, policy, "policy", "role"), "acme-ci-deploy-role")
}

// A read that was cancelled answered nothing; the next open reads again.
func TestT570_PolicyEntities_CancelledReadIsNotRemembered(t *testing.T) {
	policy := t570ManagedPolicy(t, "T570ReadOnlyAccessCancelled")
	c := t570Demo()
	c.IAM = &t570IAMEntities{IAMAPI: c.IAM, roles: []string{"acme-eks-node-role"}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checkerByTarget(t, "policy", "role")(ctx, c, policy, nil)
	t570Exact(t, t570Pivot(t, c, policy, "policy", "role"), "acme-eks-node-role")
}
