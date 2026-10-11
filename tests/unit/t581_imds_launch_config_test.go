package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

// t581NoIMDSDefaultEC2 answers DescribeImages for every image id it is asked
// about with an AMI registered without ImdsSupport, whose default leaves
// IMDSv1 allowed.
type t581NoIMDSDefaultEC2 struct{ awsclient.EC2API }

func (t581NoIMDSDefaultEC2) DescribeImages(_ context.Context, in *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	ids := in.ImageIds
	for _, f := range in.Filters {
		if aws.ToString(f.Name) == "image-id" {
			ids = append(ids, f.Values...)
		}
	}
	out := &ec2.DescribeImagesOutput{}
	for _, id := range ids {
		out.Images = append(out.Images, ec2types.Image{ImageId: aws.String(id), State: ec2types.ImageStateAvailable})
	}
	return out, nil
}

// A launch configuration that sets no metadata options takes its AMI's
// default exactly as a launch template does: on an AMI registered with
// ImdsSupport v2.0 its instances require IMDSv2, and an AMI nobody can read
// leaves the group not inspected.
func TestT581ASG_LaunchConfigTakesTheAMIIMDSDefault(t *testing.T) {
	const (
		lcV2   = "acme-web-lc-v2"
		lcGone = "acme-web-lc-gone"
	)
	v2 := pw1LaunchConfig(lcV2)
	v2.MetadataOptions = nil
	v2.ImageId = aws.String(t581AMIv2)
	gone := pw1LaunchConfig(lcGone)
	gone.MetadataOptions = nil
	gone.ImageId = aws.String(t581AMIGone)
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{lcV2: v2, lcGone: gone}}
	ec2Fake := &t581LTEC2Fake{images: map[string]ec2types.Image{t581AMIv2: t581AMI(t581AMIv2, ec2types.ImdsSupportValuesV20)}}

	rs := pw1FetchASGs(t, pw1ASGWithLaunchConfig("acme-v2-asg", lcV2), pw1ASGWithLaunchConfig("acme-gone-asg", lcGone))
	res, err := awsclient.EnrichASGScalingActivities(context.Background(),
		&awsclient.ServiceClients{AutoScaling: fake, EC2: ec2Fake}, rs, nil)
	if err != nil {
		t.Fatalf("EnrichASGScalingActivities: %v", err)
	}
	if hasCode(res.Findings["acme-v2-asg"], domain.FindingCode("asg.launch-config.imdsv1")) {
		t.Errorf("no MetadataOptions on an ImdsSupport=v2.0 AMI raised asg.launch-config.imdsv1")
	}
	if hasCode(res.Findings["acme-gone-asg"], domain.FindingCode("asg.launch-config.imdsv1")) {
		t.Errorf("an AMI DescribeImages does not return raised asg.launch-config.imdsv1")
	}
	if _, marked := res.TruncatedIDs["acme-gone-asg"]; !marked {
		t.Errorf("an unresolvable AMI left the group inspected; TruncatedIDs=%v", res.TruncatedIDs)
	}
	if ec2Fake.imageCalls != 1 {
		t.Errorf("DescribeImages called %d times for two AMIs, want one batched call", ec2Fake.imageCalls)
	}
}

// t581AccountIMDSEC2 is the lt fake with the Region's account-level instance
// metadata defaults beside it.
type t581AccountIMDSEC2 struct {
	*t581LTEC2Fake
	account    ec2types.InstanceMetadataDefaultsResponse
	accountErr error
}

func (f *t581AccountIMDSEC2) GetInstanceMetadataDefaults(_ context.Context, _ *ec2.GetInstanceMetadataDefaultsInput, _ ...func(*ec2.Options)) (*ec2.GetInstanceMetadataDefaultsOutput, error) {
	if f.accountErr != nil {
		return nil, f.accountErr
	}
	return &ec2.GetInstanceMetadataDefaultsOutput{AccountLevel: &f.account}, nil
}

// Each instance metadata option takes the value the launch spec sets, then
// the Region's account-level default, then, for HttpTokens alone, the AMI's
// ImdsSupport (EC2 User Guide, "Order of precedence for instance metadata
// options"). An account that enforces IMDSv2 fails every launch that resolves
// to optional, and a disabled endpoint answers nothing.
func TestT581IMDS_OrderOfPrecedence(t *testing.T) {
	const id = "lt-0a1b2c3d4e5f60009"
	optional := ec2types.LaunchTemplateHttpTokensStateOptional
	required := ec2types.LaunchTemplateHttpTokensStateRequired
	md := func(endpoint ec2types.LaunchTemplateInstanceMetadataEndpointState, tokens ec2types.LaunchTemplateHttpTokensState) *ec2types.LaunchTemplateInstanceMetadataOptions {
		return &ec2types.LaunchTemplateInstanceMetadataOptions{HttpEndpoint: endpoint, HttpTokens: tokens}
	}
	for _, c := range []struct {
		name    string
		spec    *ec2types.LaunchTemplateInstanceMetadataOptions
		account ec2types.InstanceMetadataDefaultsResponse
		ami     string
		flagged bool
	}{
		{"launch optional beats account required and a v2.0 AMI", md("", optional), ec2types.InstanceMetadataDefaultsResponse{HttpTokens: ec2types.HttpTokensStateRequired}, t581AMIv2, true},
		{"launch required beats account optional", md("", required), ec2types.InstanceMetadataDefaultsResponse{HttpTokens: ec2types.HttpTokensStateOptional}, t581AMIDefault, false},
		{"account required beats an AMI without ImdsSupport", nil, ec2types.InstanceMetadataDefaultsResponse{HttpTokens: ec2types.HttpTokensStateRequired}, t581AMIDefault, false},
		{"account optional beats a v2.0 AMI", nil, ec2types.InstanceMetadataDefaultsResponse{HttpTokens: ec2types.HttpTokensStateOptional}, t581AMIv2, true},
		{"no account preference: a v2.0 AMI requires tokens", nil, ec2types.InstanceMetadataDefaultsResponse{}, t581AMIv2, false},
		{"no account preference: an AMI without ImdsSupport allows IMDSv1", nil, ec2types.InstanceMetadataDefaultsResponse{}, t581AMIDefault, true},
		{"account endpoint disabled with the launch endpoint unset", md("", optional), ec2types.InstanceMetadataDefaultsResponse{HttpEndpoint: ec2types.InstanceMetadataEndpointStateDisabled}, t581AMIDefault, false},
		{"launch endpoint enabled beats account endpoint disabled", md(ec2types.LaunchTemplateInstanceMetadataEndpointStateEnabled, optional), ec2types.InstanceMetadataDefaultsResponse{HttpEndpoint: ec2types.InstanceMetadataEndpointStateDisabled}, t581AMIDefault, true},
		{"account enforcement fails every launch that resolves to optional", md("", optional), ec2types.InstanceMetadataDefaultsResponse{HttpTokensEnforced: ec2types.HttpTokensEnforcedStateEnabled}, t581AMIDefault, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			fake := &t581AccountIMDSEC2{
				t581LTEC2Fake: &t581LTEC2Fake{
					templates: []ec2types.LaunchTemplate{t581LT(id, "acme-web")},
					versions:  map[string]ec2types.LaunchTemplateVersion{id: t581LTVersion(id, c.ami, c.spec)},
					images: map[string]ec2types.Image{
						t581AMIv2:      t581AMI(t581AMIv2, ec2types.ImdsSupportValuesV20),
						t581AMIDefault: t581AMI(t581AMIDefault, ""),
					},
				},
				account: c.account,
			}
			rows, result := t581LoadAndEnrich(t, "lt", &awsclient.ServiceClients{EC2: fake}, t581LoadedAMICache())
			if got := hasCode(t581Row(t, rows, id).Findings, t581CodeLTIMDSv1); got != c.flagged {
				t.Errorf("lt.warn.imdsv1 raised = %v, want %v", got, c.flagged)
			}
			if _, marked := result.TruncatedIDs[id]; marked {
				t.Errorf("a template every read answered for is marked not inspected (%q)", result.TruncatedIDs[id])
			}
		})
	}
}

// A refused account-defaults read answers nothing about an unset or optional
// HttpTokens, so the template is not inspected; the AMI read is still made.
func TestT581IMDS_RefusedAccountDefaultsLeaveTemplateNotInspected(t *testing.T) {
	const id = "lt-0a1b2c3d4e5f6000a"
	fake := &t581AccountIMDSEC2{
		t581LTEC2Fake: &t581LTEC2Fake{
			templates: []ec2types.LaunchTemplate{t581LT(id, "acme-web")},
			versions:  map[string]ec2types.LaunchTemplateVersion{id: t581LTVersion(id, t581AMIDefault, nil)},
			images:    map[string]ec2types.Image{t581AMIDefault: t581AMI(t581AMIDefault, "")},
		},
		accountErr: &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "You are not authorized to perform this operation."},
	}
	rows, result := t581LoadAndEnrich(t, "lt", &awsclient.ServiceClients{EC2: fake}, t581LoadedAMICache())
	if hasCode(t581Row(t, rows, id).Findings, t581CodeLTIMDSv1) {
		t.Error("a refused GetInstanceMetadataDefaults raised lt.warn.imdsv1")
	}
	if _, marked := result.TruncatedIDs[id]; !marked {
		t.Errorf("a refused GetInstanceMetadataDefaults left %s inspected", id)
	}
	if fake.imageCalls != 1 {
		t.Errorf("DescribeImages called %d times, want 1 whatever the account read answered", fake.imageCalls)
	}
}
