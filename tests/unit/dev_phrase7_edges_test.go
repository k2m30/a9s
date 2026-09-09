package unit

// dev_phrase7_edges_test.go — edges of the two narrowed conditions that the
// premise pins leave open. Both were found by probing the predicates directly:
// the version parser and the endpoint matcher each accept a shape that makes
// the check answer for a resource it was not looking at.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// A version string carrying a patch component, and a major AWS has not shipped
// yet, are both at or above the release where secrets are encrypted with an
// AWS-owned key without being asked. Reading the minor as an integer and
// failing on anything else silently flags every such cluster as storing its
// secrets in the clear.
func TestDevEKSVersionBeyond128IsNotFlaggedWhateverItsShape(t *testing.T) {
	fake := &eksDescribeFailFake{
		clusters: []string{"patch-version", "next-major"},
		outputs: map[string]*eks.DescribeClusterOutput{
			"patch-version": eksClusterOut("patch-version", "1.28.5", nil),
			"next-major":    eksClusterOut("next-major", "2.0", nil),
		},
	}
	out, err := awsclient.FetchEKSClustersPage(context.Background(),
		&awsclient.ServiceClients{EKS: fake}, "")
	if err != nil {
		t.Fatalf("FetchEKSClustersPage: %v", err)
	}
	for _, r := range out.Resources {
		for _, f := range r.Findings {
			if f.Code == "eks.secrets-not-kms" {
				t.Errorf("cluster %s at version %q is flagged %q (%q); every version "+
					"from 1.28 on encrypts secrets with an AWS-owned key",
					r.Name, r.Fields["version"], f.Code, f.Phrase)
			}
		}
	}
}

// The S3 website endpoint is a hostname under amazonaws.com. A custom origin
// whose own name contains the same token is an ordinary origin that can serve
// HTTPS, and matching it as a website endpoint retires the finding on a
// distribution that really is reaching its origin in the clear.
func TestDevCustomOriginNamedLikeAWebsiteEndpointIsStillFlagged(t *testing.T) {
	lookalike := cfHealthyForW6ARows(&cftypes.DistributionConfig{
		Comment: aws.String("assets"),
		Enabled: aws.Bool(true),
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
			ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
			TargetOriginId:       aws.String("assets-origin"),
		},
		Origins: &cftypes.Origins{Quantity: aws.Int32(1), Items: []cftypes.Origin{{
			Id:         aws.String("assets-origin"),
			DomainName: aws.String("assets.s3-website-cdn.example.com"),
			CustomOriginConfig: &cftypes.CustomOriginConfig{
				HTTPPort:             aws.Int32(80),
				HTTPSPort:            aws.Int32(443),
				OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpOnly,
			},
		}}},
	})
	fake := &cfGetDistributionConfigFake{results: map[string]*cftypes.DistributionConfig{
		cfDistroID1: lookalike,
	}}
	res, err := awsclient.EnrichCloudFrontDistribution(context.Background(),
		&awsclient.ServiceClients{CloudFront: fake},
		[]resource.Resource{{ID: cfDistroID1, Name: cfDistroID1, Fields: map[string]string{}}}, nil)
	if err != nil {
		t.Fatalf("EnrichCloudFrontDistribution: %v", err)
	}
	if _, ok := hasFinding(res, cfDistroID1, "cf.insecure-protocol"); !ok {
		t.Error("a custom origin whose hostname merely contains the website-endpoint " +
			"token is not flagged; only a hostname under amazonaws.com is an S3 " +
			"website endpoint, and this one can serve HTTPS")
	}
}

// devVPCSubnetPagesFake answers DescribeVpcs with one VPC and DescribeSubnets
// across two pages, which is what AWS does for an account with more subnets in
// a VPC than one page holds.
type devVPCSubnetPagesFake struct {
	calls int
}

func (f *devVPCSubnetPagesFake) DescribeVpcs(
	_ context.Context, _ *ec2svc.DescribeVpcsInput, _ ...func(*ec2svc.Options),
) (*ec2svc.DescribeVpcsOutput, error) {
	return &ec2svc.DescribeVpcsOutput{Vpcs: []ec2types.Vpc{{
		VpcId:     aws.String("vpc-0a1b2c3d4e5f60001"),
		State:     ec2types.VpcStateAvailable,
		CidrBlock: aws.String("10.0.0.0/16"),
	}}}, nil
}

func (f *devVPCSubnetPagesFake) DescribeSubnets(
	_ context.Context, in *ec2svc.DescribeSubnetsInput, _ ...func(*ec2svc.Options),
) (*ec2svc.DescribeSubnetsOutput, error) {
	f.calls++
	subnet := func(id string) ec2types.Subnet {
		return ec2types.Subnet{SubnetId: aws.String(id), VpcId: aws.String("vpc-0a1b2c3d4e5f60001")}
	}
	if in.NextToken == nil {
		return &ec2svc.DescribeSubnetsOutput{
			Subnets:   []ec2types.Subnet{subnet("subnet-0a1b2c3d4e5f60001")},
			NextToken: aws.String("page-2"),
		}, nil
	}
	return &ec2svc.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{subnet("subnet-0a1b2c3d4e5f60002")}}, nil
}

// The flow-log check asks about every subnet id on the row. A subnet list read
// from the first page only drops the coverage that lives on the later ones,
// and the VPC is then reported as capturing nothing.
func TestDevVPCSubnetIDsFollowEveryPage(t *testing.T) {
	fake := &devVPCSubnetPagesFake{}
	out, err := awsclient.FetchVPCsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchVPCsPage: %v", err)
	}
	if len(out.Resources) != 1 {
		t.Fatalf("got %d VPC rows, want 1", len(out.Resources))
	}
	got := out.Resources[0].Fields["subnet_ids"]
	const want = "subnet-0a1b2c3d4e5f60001,subnet-0a1b2c3d4e5f60002"
	if got != want {
		t.Errorf("subnet_ids = %q, want %q (pages read: %d)", got, want, fake.calls)
	}
}
