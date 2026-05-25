package unit_test

// aws_related_def_fragility_test.go — AS-1243 regression coverage for three
// pre-existing related-def fragilities surfaced by AS-1242 Stage 6.5:
//
//  1. checkEKSAMI must soft-skip nodegroups whose launch template has been
//     deleted upstream (InvalidLaunchTemplateId.NotFound) instead of hard
//     failing the whole AMI panel.
//  2. checkNGAMI must return Count:0 (a true zero) when the launch template
//     has been deleted upstream, not Count:-1 with the API error.
//  3. checkELBWAF must skip the wafv2:GetWebACLForResource call for non-ALB
//     load balancers (NLB / GWLB), because AWS WAFv2 only supports ALBs and
//     would return WAFInvalidParameterException.
//
// The fixes live in:
//   - internal/aws/eks_related_extra.go (checkEKSAMI)
//   - internal/aws/ng_related.go (checkNGAMI)
//   - internal/aws/elb_related.go (checkELBWAF)

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	smithy "github.com/aws/smithy-go"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Fix 1 — checkEKSAMI: soft-skip InvalidLaunchTemplateId.NotFound per NG
// ---------------------------------------------------------------------------

// TestCheckEKSAMI_SkipsDeletedLaunchTemplate verifies that when one node group
// references a launch template that has been deleted upstream, the AMI panel
// still surfaces the AMIs from the other node groups (rather than hard-failing
// with Count:-1). The deleted-LT NG is recorded as a partial failure entry in
// the aggregated error per the existing AggregateFailures format.
func TestCheckEKSAMI_SkipsDeletedLaunchTemplate(t *testing.T) {
	const (
		ltGood    = "lt-good001"
		ltDeleted = "lt-deleted999"
		amiGood   = "ami-0a1b2c3d4e5f60001"
	)

	eksNodegroups := map[string]*ekstypes.Nodegroup{
		"ng-good": {
			NodegroupName: aws.String("ng-good"),
			ClusterName:   aws.String("acme-services"),
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
				Id:      aws.String(ltGood),
				Version: aws.String("1"),
			},
		},
		"ng-deleted-lt": {
			NodegroupName: aws.String("ng-deleted-lt"),
			ClusterName:   aws.String("acme-services"),
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
				Id:      aws.String(ltDeleted),
				Version: aws.String("1"),
			},
		},
	}
	fakeEKS := newFakeEKSWithNodegroups([]string{"ng-good", "ng-deleted-lt"}, eksNodegroups)

	fakeEC2 := &fakeEC2Batch2{
		describeLaunchTemplateVersionsFn: func(input *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			ltID := ""
			if input.LaunchTemplateId != nil {
				ltID = *input.LaunchTemplateId
			}
			if ltID == ltDeleted {
				return nil, &smithy.GenericAPIError{
					Code:    "InvalidLaunchTemplateId.NotFound",
					Message: "The launch template ID '" + ltDeleted + "' does not exist",
				}
			}
			return &ec2.DescribeLaunchTemplateVersionsOutput{
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{
						LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
							ImageId: aws.String(amiGood),
						},
					},
				},
			}, nil
		},
	}

	clients := &awsclient.ServiceClients{EKS: fakeEKS, EC2: fakeEC2}
	res := eksClusterSrcResource()
	checker := eksCheckerByTarget(t, "ami")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (good NG contributes AMI even though deleted-LT NG was skipped); ResourceIDs=%v Err=%v",
			result.Count, result.ResourceIDs, result.Err)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != amiGood {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, amiGood)
	}
	if result.Err == nil {
		// Acceptable: the issue allows Err nil OR a soft-skip note.
		return
	}
	if !strings.Contains(result.Err.Error(), "launch template deleted") {
		t.Errorf("Err = %v; expected nil or a 'launch template deleted' soft-skip note", result.Err)
	}
	if !strings.Contains(result.Err.Error(), "ng-deleted-lt") {
		t.Errorf("Err = %v; expected the soft-skip note to name 'ng-deleted-lt'", result.Err)
	}
}

// TestCheckEKSAMI_HardFailsOnOtherErrors verifies that errors other than
// InvalidLaunchTemplateId.NotFound keep the existing partial-failure behavior:
// the failure is aggregated into Err, surfacing the underlying error.
func TestCheckEKSAMI_HardFailsOnOtherErrors(t *testing.T) {
	eksNodegroups := map[string]*ekstypes.Nodegroup{
		"ng-throttled": {
			NodegroupName: aws.String("ng-throttled"),
			ClusterName:   aws.String("acme-services"),
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
				Id:      aws.String("lt-throttled"),
				Version: aws.String("1"),
			},
		},
	}
	fakeEKS := newFakeEKSWithNodegroups([]string{"ng-throttled"}, eksNodegroups)

	fakeEC2 := &fakeEC2Batch2{
		describeLaunchTemplateVersionsFn: func(_ *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			return nil, fmt.Errorf("unexpected 5xx from EC2 DescribeLaunchTemplateVersions")
		},
	}

	clients := &awsclient.ServiceClients{EKS: fakeEKS, EC2: fakeEC2}
	res := eksClusterSrcResource()
	checker := eksCheckerByTarget(t, "ami")
	result := checker(context.Background(), clients, res, nil)

	if result.Err == nil {
		t.Fatal("Err = nil, want non-nil for non-NotFound error")
	}
	if strings.Contains(result.Err.Error(), "launch template deleted") {
		t.Errorf("Err = %v; non-NotFound error should not be classified as a soft skip", result.Err)
	}
	if !strings.Contains(result.Err.Error(), "ng-throttled") {
		t.Errorf("Err = %v; expected failure note to name 'ng-throttled'", result.Err)
	}
}

// ---------------------------------------------------------------------------
// Fix 2 — checkNGAMI: Count:0 (true zero) on InvalidLaunchTemplateId.NotFound
// ---------------------------------------------------------------------------

// TestCheckNGAMI_SkipsDeletedLaunchTemplate verifies that when the node group's
// launch template has been deleted upstream, the AMI checker returns
// Count:0 / Err:nil instead of Count:-1 with the API error — the LT is gone,
// so there is no AMI to relate to.
func TestCheckNGAMI_SkipsDeletedLaunchTemplate(t *testing.T) {
	const ltDeleted = "lt-deleted999"

	fakeEC2 := &fakeEC2Batch2{
		describeLaunchTemplateVersionsFn: func(_ *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			return nil, &smithy.GenericAPIError{
				Code:    "InvalidLaunchTemplateId.NotFound",
				Message: "The launch template ID '" + ltDeleted + "' does not exist",
			}
		},
	}
	clients := &awsclient.ServiceClients{EC2: fakeEC2}
	res := ngSrcResourceWithLaunchTemplate(ltDeleted, "1")
	checker := ngCheckerByTarget(t, "ami")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (deleted LT is a true zero); Err=%v", result.Count, result.Err)
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil (NotFound is soft-skipped to a true zero)", result.Err)
	}
}

// ---------------------------------------------------------------------------
// Fix 3 — checkELBWAF: gate on Fields["type"] before calling WAF API
// ---------------------------------------------------------------------------

// fakeWAFv2NoCallExpected fails the test if GetWebACLForResource is invoked.
// Used to enforce the type-gate skip in checkELBWAF.
type fakeWAFv2NoCallExpected struct {
	t *testing.T
}

func (f *fakeWAFv2NoCallExpected) GetWebACLForResource(_ context.Context, _ *wafv2.GetWebACLForResourceInput, _ ...func(*wafv2.Options)) (*wafv2.GetWebACLForResourceOutput, error) {
	f.t.Fatal("GetWebACLForResource was called for a non-ALB ELB; the type gate should have skipped it")
	return nil, nil
}

func (f *fakeWAFv2NoCallExpected) ListWebACLs(_ context.Context, _ *wafv2.ListWebACLsInput, _ ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	return &wafv2.ListWebACLsOutput{}, nil
}

func (f *fakeWAFv2NoCallExpected) ListResourcesForWebACL(_ context.Context, _ *wafv2.ListResourcesForWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.ListResourcesForWebACLOutput, error) {
	return &wafv2.ListResourcesForWebACLOutput{}, nil
}

func (f *fakeWAFv2NoCallExpected) GetLoggingConfiguration(_ context.Context, _ *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error) {
	return &wafv2.GetLoggingConfigurationOutput{}, nil
}

// TestCheckELBWAF_SkipsNetworkLoadBalancer verifies that for an NLB (Fields["type"]="network"),
// the checker returns Count:0 without calling wafv2:GetWebACLForResource.
func TestCheckELBWAF_SkipsNetworkLoadBalancer(t *testing.T) {
	const nlbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/prod-nlb/1234567890abcdef"
	source := resource.Resource{
		ID:     "prod-nlb",
		Name:   "prod-nlb",
		Fields: map[string]string{"load_balancer_arn": nlbARN, "type": "network"},
	}
	clients := &awsclient.ServiceClients{
		WAFv2: &fakeWAFv2NoCallExpected{t: t},
	}
	checker := elbCheckerByTarget(t, "waf")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (NLB is not WAFv2-compatible)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil (skip is a true zero)", result.Err)
	}
}

// TestCheckELBWAF_SkipsGatewayLoadBalancer verifies that for a GWLB
// (Fields["type"]="gateway"), the checker returns Count:0 without calling the API.
func TestCheckELBWAF_SkipsGatewayLoadBalancer(t *testing.T) {
	const gwlbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/gwy/prod-gwlb/abcdef1234567890"
	source := resource.Resource{
		ID:     "prod-gwlb",
		Name:   "prod-gwlb",
		Fields: map[string]string{"load_balancer_arn": gwlbARN, "type": "gateway"},
	}
	clients := &awsclient.ServiceClients{
		WAFv2: &fakeWAFv2NoCallExpected{t: t},
	}
	checker := elbCheckerByTarget(t, "waf")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (GWLB is not WAFv2-compatible)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil (skip is a true zero)", result.Err)
	}
}

// TestCheckELBWAF_CallsAPIForApplicationLB verifies that the happy path for
// ALB (Fields["type"]="application") still calls the WAF API and surfaces the
// Web ACL.
func TestCheckELBWAF_CallsAPIForApplicationLB(t *testing.T) {
	const albARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	const wafID = "waf-web-acl-id-abc123"
	source := resource.Resource{
		ID:     "prod-alb",
		Name:   "prod-alb",
		Fields: map[string]string{"load_balancer_arn": albARN, "type": "application"},
	}
	clients := &awsclient.ServiceClients{
		WAFv2: &fakeWAFv2ForResource{
			output: &wafv2.GetWebACLForResourceOutput{
				WebACL: &wafv2types.WebACL{
					Id:   aws.String(wafID),
					Name: aws.String("prod-alb-waf"),
					ARN:  aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/prod-alb-waf/" + wafID),
				},
			},
		},
	}
	checker := elbCheckerByTarget(t, "waf")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1; ResourceIDs=%v Err=%v", result.Count, result.ResourceIDs, result.Err)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != wafID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, wafID)
	}
}

// TestCheckELBWAF_EmptyTypeFallbackToRawStruct verifies the defense-in-depth
// fallback: when Fields["type"] is empty (e.g. cache rehydration paths that
// drop the field), the checker falls back to RawStruct.Type. If RawStruct
// resolves to "application", the WAF API call still proceeds. This mirrors
// the existing elbARN RawStruct fallback at lines 370-376 and prevents a
// silent false-negative on a security-relevant pivot.
func TestCheckELBWAF_EmptyTypeFallbackToRawStruct(t *testing.T) {
	const albARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	const wafID = "waf-web-acl-id-fallback"
	source := resource.Resource{
		ID:     "prod-alb",
		Name:   "prod-alb",
		Fields: map[string]string{"load_balancer_arn": albARN}, // type missing
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerArn: aws.String(albARN),
			Type:            elbv2types.LoadBalancerTypeEnumApplication,
		},
	}
	clients := &awsclient.ServiceClients{
		WAFv2: &fakeWAFv2ForResource{
			output: &wafv2.GetWebACLForResourceOutput{
				WebACL: &wafv2types.WebACL{
					Id:   aws.String(wafID),
					Name: aws.String("prod-alb-waf"),
					ARN:  aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/prod-alb-waf/" + wafID),
				},
			},
		},
	}
	checker := elbCheckerByTarget(t, "waf")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (RawStruct fallback should have surfaced ALB type); ResourceIDs=%v Err=%v", result.Count, result.ResourceIDs, result.Err)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != wafID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, wafID)
	}
}
