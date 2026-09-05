package unit

// prowler_w1_ami_test.go — behavioural pins for ami.public (batch w1).
//
// An AMI marked public is readable by every AWS account: anyone can launch it
// and read whatever the image's filesystem carries. The finding is only sound
// for images the caller owns, which is why the Owners=self filter on the list
// call is pinned here alongside the finding itself.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const pw1AMICodePublic = domain.FindingCode("ami.public")

// pw1AMIFake records the DescribeImages input so the Owners filter can be
// asserted, and returns a fixed image list.
type pw1AMIFake struct {
	images   []ec2types.Image
	lastArgs *ec2.DescribeImagesInput
}

func (f *pw1AMIFake) DescribeImages(_ context.Context, in *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	f.lastArgs = in
	return &ec2.DescribeImagesOutput{Images: f.images}, nil
}

// pw1Image builds a realistic self-owned AMI. public==nil leaves the field
// unset, which is how AWS reports an image whose launch permissions were not
// returned.
func pw1Image(id string, public *bool) ec2types.Image {
	return ec2types.Image{
		ImageId:      aws.String(id),
		Name:         aws.String("acme-" + id),
		OwnerId:      aws.String("123456789012"),
		State:        ec2types.ImageStateAvailable,
		Architecture: ec2types.ArchitectureValuesX8664,
		CreationDate: aws.String(time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)),
		Public:       public,
		BlockDeviceMappings: []ec2types.BlockDeviceMapping{{
			DeviceName: aws.String("/dev/xvda"),
			Ebs:        &ec2types.EbsBlockDevice{Encrypted: aws.Bool(true), VolumeSize: aws.Int32(8)},
		}},
	}
}

func pw1FetchAMIs(t *testing.T, fake *pw1AMIFake) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchAMIsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchAMIsPage: %v", err)
	}
	return out.Resources
}

// TestAMI_Public_SharedWithEveryAccount pins the Broken finding and the row
// naming the launch-permission state.
func TestAMI_Public_SharedWithEveryAccount(t *testing.T) {
	rs := pw1FetchAMIs(t, &pw1AMIFake{images: []ec2types.Image{pw1Image("ami-0public00aaaaaa1", aws.Bool(true))}})
	r := pw1ResourceByID(t, rs, "ami-0public00aaaaaa1")
	pw1RequireFinding(t, r.Findings, pw1AMICodePublic, "shared with all AWS accounts", domain.SevBroken, "wave1")
	pw1RequireRow(t, r.AttentionDetails[pw1AMICodePublic].Rows, "Public", "true")
}

// TestAMI_Public_PrivateIsHealthy pins the negative case.
func TestAMI_Public_PrivateIsHealthy(t *testing.T) {
	rs := pw1FetchAMIs(t, &pw1AMIFake{images: []ec2types.Image{pw1Image("ami-0private00aaaaa1", aws.Bool(false))}})
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "ami-0private00aaaaa1").Findings, pw1AMICodePublic)
}

// TestAMI_Public_NilIsUnknownNotPublic pins that an absent Public pointer is
// unknown: AWS omits it when launch permissions were not resolved, and an
// unknown permission set is not evidence of a public image.
func TestAMI_Public_NilIsUnknownNotPublic(t *testing.T) {
	rs := pw1FetchAMIs(t, &pw1AMIFake{images: []ec2types.Image{pw1Image("ami-0unknown00aaaaa1", nil)}})
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "ami-0unknown00aaaaa1").Findings, pw1AMICodePublic)
}

// TestAMI_Public_ListIsScopedToOwnedImages pins the Owners=self filter the
// finding depends on. Without it the list would carry every public AMI in the
// marketplace and the finding would fire on images the operator cannot change.
func TestAMI_Public_ListIsScopedToOwnedImages(t *testing.T) {
	fake := &pw1AMIFake{images: []ec2types.Image{pw1Image("ami-0scoped000aaaaa1", aws.Bool(false))}}
	pw1FetchAMIs(t, fake)
	if fake.lastArgs == nil {
		t.Fatal("DescribeImages was never called")
	}
	if len(fake.lastArgs.Owners) != 1 || fake.lastArgs.Owners[0] != "self" {
		t.Fatalf("Owners = %v, want [self]", fake.lastArgs.Owners)
	}
}

// TestAMI_Public_CoexistsWithLifecycleFinding pins independence: a public AMI
// that is also deprecated carries both findings.
func TestAMI_Public_CoexistsWithLifecycleFinding(t *testing.T) {
	img := pw1Image("ami-0pubdepr00aaaaa1", aws.Bool(true))
	img.DeprecationTime = aws.String(time.Now().Add(-24 * time.Hour).Format(time.RFC3339))
	rs := pw1FetchAMIs(t, &pw1AMIFake{images: []ec2types.Image{img}})
	r := pw1ResourceByID(t, rs, "ami-0pubdepr00aaaaa1")
	pw1RequireFinding(t, r.Findings, pw1AMICodePublic, "shared with all AWS accounts", domain.SevBroken, "wave1")
	if _, ok := pw1FindFinding(r.Findings, domain.FindingCode("ami.deprecated")); !ok {
		t.Errorf("ami.deprecated lost when ami.public was added: %+v", r.Findings)
	}
}

// TestAMI_DemoBench_OnlyWitnessIsPublic pins that the demo bench shows exactly
// one public AMI, the named witness row.
func TestAMI_DemoBench_OnlyWitnessIsPublic(t *testing.T) {
	out, err := awsclient.FetchAMIsPage(context.Background(), fakes.NewEC2(), "")
	if err != nil {
		t.Fatalf("FetchAMIsPage(demo): %v", err)
	}
	var carriers []string
	for _, r := range out.Resources {
		if _, ok := pw1FindFinding(r.Findings, pw1AMICodePublic); ok {
			carriers = append(carriers, r.Name+"/"+r.ID)
		}
	}
	pw1RequireOnlyWitness(t, pw1AMICodePublic, fixtures.AMIPublic, carriers)
}

// TestAMI_Public_DeregisteredImageIsNotAnExposure pins common contract rule 4
// on ami: a deregistered image cannot be launched by anyone, so its launch
// permission is no longer an exposure and the row is not an open posture item.
// imageResource currently appends ami.public independently of State.
func TestAMI_Public_DeregisteredImageIsNotAnExposure(t *testing.T) {
	for _, state := range []ec2types.ImageState{
		ec2types.ImageStateDeregistered,
		ec2types.ImageStateDisabled,
	} {
		img := pw1Image("ami-0dereg000aaaaaa1", aws.Bool(true))
		img.State = state
		rs := pw1FetchAMIs(t, &pw1AMIFake{images: []ec2types.Image{img}})
		r := pw1ResourceByID(t, rs, "ami-0dereg000aaaaaa1")
		if _, ok := pw1FindFinding(r.Findings, pw1AMICodePublic); ok {
			t.Errorf("state %q: ami.public emitted for an image that cannot be launched", state)
		}
	}
}
