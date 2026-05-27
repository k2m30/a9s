package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

type capturingDescribeImagesClient struct {
	output *ec2.DescribeImagesOutput
	err    error
	inputs []*ec2.DescribeImagesInput
}

func (m *capturingDescribeImagesClient) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	m.inputs = append(m.inputs, params)
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// AMI fetcher tests
// ---------------------------------------------------------------------------

func TestFetchAMIs_ParsesMultipleImages(t *testing.T) {
	mock := &mockEC2DescribeImagesClient{
		output: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId:         aws.String("ami-0abc111222333444a"),
					Name:            aws.String("my-web-server-ami"),
					State:           ec2types.ImageStateAvailable,
					Architecture:    ec2types.ArchitectureValuesX8664,
					PlatformDetails: aws.String("Linux/UNIX"),
					RootDeviceType:  ec2types.DeviceTypeEbs,
					CreationDate:    aws.String("2025-01-15T10:30:00.000Z"),
					Public:          aws.Bool(false),
				},
				{
					ImageId:         aws.String("ami-0xyz999888777666b"),
					Name:            aws.String("my-arm64-ami"),
					State:           ec2types.ImageStateAvailable,
					Architecture:    ec2types.ArchitectureValuesArm64,
					PlatformDetails: aws.String("Linux/UNIX"),
					RootDeviceType:  ec2types.DeviceTypeEbs,
					CreationDate:    aws.String("2025-02-01T08:00:00.000Z"),
					Public:          aws.Bool(true),
				},
			},
		},
	}

	resources, err := awsclient.FetchAMIs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first image
	r0 := resources[0]
	if r0.ID != "ami-0abc111222333444a" {
		t.Errorf("resource[0].ID: expected %q, got %q", "ami-0abc111222333444a", r0.ID)
	}
	if r0.Name != "my-web-server-ami" {
		t.Errorf("resource[0].Name: expected %q, got %q", "my-web-server-ami", r0.Name)
	}
	// Post-fold contract: available state is healthy → no Finding.
	if len(r0.Findings) != 0 {
		t.Errorf("resource[0].Findings: expected 0 for available AMI, got %d", len(r0.Findings))
	}

	// Verify second image
	r1 := resources[1]
	if r1.ID != "ami-0xyz999888777666b" {
		t.Errorf("resource[1].ID: expected %q, got %q", "ami-0xyz999888777666b", r1.ID)
	}
	if r1.Name != "my-arm64-ami" {
		t.Errorf("resource[1].Name: expected %q, got %q", "my-arm64-ami", r1.Name)
	}
	// Post-fold contract: available state is healthy → no Finding.
	if len(r1.Findings) != 0 {
		t.Errorf("resource[1].Findings: expected 0 for available AMI, got %d", len(r1.Findings))
	}
}

func TestFetchAMIs_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeImagesClient{
		output: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{},
		},
	}

	resources, err := awsclient.FetchAMIs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchAMIs_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeImagesClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchAMIs(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchAMIs_FieldExtraction(t *testing.T) {
	mock := &mockEC2DescribeImagesClient{
		output: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId:         aws.String("ami-0abc111222333444a"),
					Name:            aws.String("my-web-server-ami"),
					State:           ec2types.ImageStateAvailable,
					Architecture:    ec2types.ArchitectureValuesX8664,
					PlatformDetails: aws.String("Linux/UNIX"),
					RootDeviceType:  ec2types.DeviceTypeEbs,
					CreationDate:    aws.String("2025-01-15T10:30:00.000Z"),
					Public:          aws.Bool(false),
				},
			},
		},
	}

	resources, err := awsclient.FetchAMIs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// Verify all FieldKeys are present and have exact values
	if r.Fields["image_id"] != "ami-0abc111222333444a" {
		t.Errorf("Fields[\"image_id\"]: expected %q, got %q", "ami-0abc111222333444a", r.Fields["image_id"])
	}
	if r.Fields["name"] != "my-web-server-ami" {
		t.Errorf("Fields[\"name\"]: expected %q, got %q", "my-web-server-ami", r.Fields["name"])
	}
	if r.Fields["state"] != "available" {
		t.Errorf("Fields[\"state\"]: expected %q, got %q", "available", r.Fields["state"])
	}
	if r.Fields["architecture"] != "x86_64" {
		t.Errorf("Fields[\"architecture\"]: expected %q, got %q", "x86_64", r.Fields["architecture"])
	}
	if r.Fields["platform"] != "Linux/UNIX" {
		t.Errorf("Fields[\"platform\"]: expected %q, got %q", "Linux/UNIX", r.Fields["platform"])
	}
	if r.Fields["root_device_type"] != "ebs" {
		t.Errorf("Fields[\"root_device_type\"]: expected %q, got %q", "ebs", r.Fields["root_device_type"])
	}
	if r.Fields["creation_date"] != "2025-01-15T10:30:00.000Z" {
		t.Errorf("Fields[\"creation_date\"]: expected %q, got %q", "2025-01-15T10:30:00.000Z", r.Fields["creation_date"])
	}
	if r.Fields["public"] != "false" {
		t.Errorf("Fields[\"public\"]: expected %q, got %q", "false", r.Fields["public"])
	}
}

func TestFetchAMIs_NameFromDirectField(t *testing.T) {
	// AMI Name comes from Image.Name directly, NOT from Tags
	mock := &mockEC2DescribeImagesClient{
		output: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId:      aws.String("ami-direct-name"),
					Name:         aws.String("direct-name-ami"),
					State:        ec2types.ImageStateAvailable,
					Architecture: ec2types.ArchitectureValuesX8664,
				},
			},
		},
	}

	resources, err := awsclient.FetchAMIs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].Name != "direct-name-ami" {
		t.Errorf("Name: expected %q from Image.Name field, got %q", "direct-name-ami", resources[0].Name)
	}
}

func TestFetchAMIs_PublicTrue(t *testing.T) {
	mock := &mockEC2DescribeImagesClient{
		output: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId:      aws.String("ami-public"),
					Name:         aws.String("public-ami"),
					State:        ec2types.ImageStateAvailable,
					Architecture: ec2types.ArchitectureValuesArm64,
					Public:       aws.Bool(true),
				},
			},
		},
	}

	resources, err := awsclient.FetchAMIs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].Fields["public"] != "true" {
		t.Errorf("Fields[\"public\"]: expected %q for public AMI, got %q", "true", resources[0].Fields["public"])
	}
}

func TestFetchAMIs_RawStructIsImage(t *testing.T) {
	mock := &mockEC2DescribeImagesClient{
		output: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId:      aws.String("ami-rawstruct"),
					Name:         aws.String("rawstruct-test-ami"),
					State:        ec2types.ImageStateAvailable,
					Architecture: ec2types.ArchitectureValuesX8664,
				},
			},
		},
	}

	resources, err := awsclient.FetchAMIs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	img, ok := r.RawStruct.(ec2types.Image)
	if !ok {
		t.Fatalf("RawStruct should be ec2types.Image, got %T", r.RawStruct)
	}
	if img.ImageId == nil || *img.ImageId != "ami-rawstruct" {
		t.Errorf("RawStruct.ImageId: expected %q, got %v", "ami-rawstruct", img.ImageId)
	}
}

func TestFetchAMIByID_UsesExactImageIDWithoutOwnersFilter(t *testing.T) {
	mock := &capturingDescribeImagesClient{
		output: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId: aws.String("ami-public-exact"),
					Name:    aws.String("public-exact-ami"),
					State:   ec2types.ImageStateAvailable,
				},
			},
		},
	}

	res, err := awsclient.FetchAMIByID(context.Background(), mock, "ami-public-exact")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(mock.inputs) != 1 {
		t.Fatalf("expected exactly one DescribeImages call, got %d", len(mock.inputs))
	}
	in := mock.inputs[0]
	if len(in.ImageIds) != 1 || in.ImageIds[0] != "ami-public-exact" {
		t.Fatalf("FetchAMIByID should query the exact AMI id, got ImageIds=%v", in.ImageIds)
	}
	if len(in.Owners) != 0 {
		t.Fatalf("FetchAMIByID must not set Owners filtering for exact-ID lookups, got %v", in.Owners)
	}
	if res.ID != "ami-public-exact" {
		t.Fatalf("expected fetched resource ID %q, got %q", "ami-public-exact", res.ID)
	}
}

func TestFetchAMIByID_NotFound(t *testing.T) {
	mock := &capturingDescribeImagesClient{
		output: &ec2.DescribeImagesOutput{Images: nil},
	}

	_, err := awsclient.FetchAMIByID(context.Background(), mock, "ami-missing")
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
	if !strings.Contains(err.Error(), "ami-missing") {
		t.Fatalf("expected error to mention missing ami id, got %v", err)
	}
}
