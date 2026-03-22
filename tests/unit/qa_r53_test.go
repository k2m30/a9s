package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Route 53 Hosted Zones fetcher tests
// ---------------------------------------------------------------------------

func TestFetchHostedZones_ParsesMultiple(t *testing.T) {
	mock := &mockRoute53Client{
		output: &route53.ListHostedZonesOutput{
			HostedZones: []r53types.HostedZone{
				{
					Id:                     aws.String("/hostedzone/Z1234567890ABC"),
					Name:                   aws.String("example.com."),
					CallerReference:        aws.String("ref-001"),
					ResourceRecordSetCount: aws.Int64(42),
					Config: &r53types.HostedZoneConfig{
						Comment:     aws.String("Production zone"),
						PrivateZone: false,
					},
				},
				{
					Id:                     aws.String("/hostedzone/ZDEF987654321"),
					Name:                   aws.String("internal.corp."),
					CallerReference:        aws.String("ref-002"),
					ResourceRecordSetCount: aws.Int64(5),
					Config: &r53types.HostedZoneConfig{
						Comment:     aws.String("Private zone"),
						PrivateZone: true,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchHostedZones(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first hosted zone
	r0 := resources[0]
	if r0.ID != "/hostedzone/Z1234567890ABC" {
		t.Errorf("resource[0].ID: expected %q, got %q", "/hostedzone/Z1234567890ABC", r0.ID)
	}
	if r0.Name != "example.com." {
		t.Errorf("resource[0].Name: expected %q, got %q", "example.com.", r0.Name)
	}

	// Verify required fields
	requiredFields := []string{"zone_id", "name", "record_count", "private_zone", "comment"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	if r0.Fields["zone_id"] != "/hostedzone/Z1234567890ABC" {
		t.Errorf("resource[0].Fields[\"zone_id\"]: expected %q, got %q", "/hostedzone/Z1234567890ABC", r0.Fields["zone_id"])
	}
	if r0.Fields["record_count"] != "42" {
		t.Errorf("resource[0].Fields[\"record_count\"]: expected %q, got %q", "42", r0.Fields["record_count"])
	}
	if r0.Fields["private_zone"] != "false" {
		t.Errorf("resource[0].Fields[\"private_zone\"]: expected %q, got %q", "false", r0.Fields["private_zone"])
	}

	// Verify second zone (private)
	r1 := resources[1]
	if r1.Fields["private_zone"] != "true" {
		t.Errorf("resource[1].Fields[\"private_zone\"]: expected %q, got %q", "true", r1.Fields["private_zone"])
	}
	if r1.Fields["comment"] != "Private zone" {
		t.Errorf("resource[1].Fields[\"comment\"]: expected %q, got %q", "Private zone", r1.Fields["comment"])
	}
}

func TestFetchHostedZones_RawStructPopulated(t *testing.T) {
	mock := &mockRoute53Client{
		output: &route53.ListHostedZonesOutput{
			HostedZones: []r53types.HostedZone{
				{
					Id:              aws.String("/hostedzone/ZRAW123"),
					Name:            aws.String("raw.example.com."),
					CallerReference: aws.String("ref-raw"),
				},
			},
		},
	}

	resources, err := awsclient.FetchHostedZones(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	zone, ok := r.RawStruct.(r53types.HostedZone)
	if !ok {
		t.Fatalf("RawStruct should be r53types.HostedZone, got %T", r.RawStruct)
	}
	if zone.Id == nil || *zone.Id != "/hostedzone/ZRAW123" {
		t.Errorf("RawStruct.Id: expected %q", "/hostedzone/ZRAW123")
	}
}

func TestFetchHostedZones_ErrorResponse(t *testing.T) {
	mock := &mockRoute53Client{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchHostedZones(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

func TestFetchHostedZones_EmptyResponse(t *testing.T) {
	mock := &mockRoute53Client{
		output: &route53.ListHostedZonesOutput{
			HostedZones: []r53types.HostedZone{},
		},
	}

	resources, err := awsclient.FetchHostedZones(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
