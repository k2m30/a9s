package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// CloudFront Distribution fetcher tests
// ---------------------------------------------------------------------------

func TestFetchCloudFrontDistributions_ParsesMultiple(t *testing.T) {
	now := time.Now()
	mock := &mockCloudFrontClient{
		output: &cloudfront.ListDistributionsOutput{
			DistributionList: &cftypes.DistributionList{
				Items: []cftypes.DistributionSummary{
					{
						Id:               aws.String("E1ABC2DEF3GHIJ"),
						DomainName:       aws.String("d111111abcdef8.cloudfront.net"),
						Status:           aws.String("Deployed"),
						Enabled:          aws.Bool(true),
						Comment:          aws.String("Production CDN"),
						LastModifiedTime: &now,
						ARN:              aws.String("arn:aws:cloudfront::123456789012:distribution/E1ABC2DEF3GHIJ"),
						Aliases: &cftypes.Aliases{
							Quantity: aws.Int32(1),
							Items:    []string{"cdn.example.com"},
						},
						Origins: &cftypes.Origins{
							Quantity: aws.Int32(1),
						},
						PriceClass: cftypes.PriceClassPriceClassAll,
						HttpVersion: cftypes.HttpVersionHttp2,
					},
					{
						Id:               aws.String("E4XYZ5GHI6JKLM"),
						DomainName:       aws.String("d222222ghijkl9.cloudfront.net"),
						Status:           aws.String("InProgress"),
						Enabled:          aws.Bool(false),
						Comment:          aws.String(""),
						LastModifiedTime: &now,
						ARN:              aws.String("arn:aws:cloudfront::123456789012:distribution/E4XYZ5GHI6JKLM"),
						Aliases: &cftypes.Aliases{
							Quantity: aws.Int32(0),
						},
						Origins: &cftypes.Origins{
							Quantity: aws.Int32(1),
						},
						PriceClass: cftypes.PriceClassPriceClass100,
						HttpVersion: cftypes.HttpVersionHttp2,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudFrontDistributions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first distribution
	r0 := resources[0]
	if r0.ID != "E1ABC2DEF3GHIJ" {
		t.Errorf("resource[0].ID: expected %q, got %q", "E1ABC2DEF3GHIJ", r0.ID)
	}
	if r0.Name != "E1ABC2DEF3GHIJ" {
		t.Errorf("resource[0].Name: expected %q, got %q", "E1ABC2DEF3GHIJ", r0.Name)
	}
	if r0.Status != "Deployed" {
		t.Errorf("resource[0].Status: expected %q, got %q", "Deployed", r0.Status)
	}

	// Verify required fields
	requiredFields := []string{"distribution_id", "domain_name", "status", "enabled", "aliases", "price_class"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify specific field values
	if r0.Fields["distribution_id"] != "E1ABC2DEF3GHIJ" {
		t.Errorf("resource[0].Fields[\"distribution_id\"]: expected %q, got %q", "E1ABC2DEF3GHIJ", r0.Fields["distribution_id"])
	}
	if r0.Fields["domain_name"] != "d111111abcdef8.cloudfront.net" {
		t.Errorf("resource[0].Fields[\"domain_name\"]: expected %q, got %q", "d111111abcdef8.cloudfront.net", r0.Fields["domain_name"])
	}
	if r0.Fields["enabled"] != "true" {
		t.Errorf("resource[0].Fields[\"enabled\"]: expected %q, got %q", "true", r0.Fields["enabled"])
	}
	if r0.Fields["aliases"] != "cdn.example.com" {
		t.Errorf("resource[0].Fields[\"aliases\"]: expected %q, got %q", "cdn.example.com", r0.Fields["aliases"])
	}

	// Verify second distribution (disabled, no aliases)
	r1 := resources[1]
	if r1.Status != "InProgress" {
		t.Errorf("resource[1].Status: expected %q, got %q", "InProgress", r1.Status)
	}
	if r1.Fields["enabled"] != "false" {
		t.Errorf("resource[1].Fields[\"enabled\"]: expected %q, got %q", "false", r1.Fields["enabled"])
	}
}

func TestFetchCloudFrontDistributions_RawStructPopulated(t *testing.T) {
	now := time.Now()
	mock := &mockCloudFrontClient{
		output: &cloudfront.ListDistributionsOutput{
			DistributionList: &cftypes.DistributionList{
				Items: []cftypes.DistributionSummary{
					{
						Id:               aws.String("ERAW123"),
						DomainName:       aws.String("raw.cloudfront.net"),
						Status:           aws.String("Deployed"),
						Enabled:          aws.Bool(true),
						Comment:          aws.String(""),
						LastModifiedTime: &now,
						ARN:              aws.String("arn:aws:cloudfront::123456789012:distribution/ERAW123"),
						Aliases:          &cftypes.Aliases{Quantity: aws.Int32(0)},
						Origins:          &cftypes.Origins{Quantity: aws.Int32(1)},
						PriceClass:       cftypes.PriceClassPriceClassAll,
						HttpVersion:      cftypes.HttpVersionHttp2,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudFrontDistributions(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	dist, ok := r.RawStruct.(cftypes.DistributionSummary)
	if !ok {
		t.Fatalf("RawStruct should be cftypes.DistributionSummary, got %T", r.RawStruct)
	}
	if dist.Id == nil || *dist.Id != "ERAW123" {
		t.Errorf("RawStruct.Id: expected %q", "ERAW123")
	}
}

func TestFetchCloudFrontDistributions_ErrorResponse(t *testing.T) {
	mock := &mockCloudFrontClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchCloudFrontDistributions(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

func TestFetchCloudFrontDistributions_EmptyResponse(t *testing.T) {
	mock := &mockCloudFrontClient{
		output: &cloudfront.ListDistributionsOutput{
			DistributionList: &cftypes.DistributionList{
				Items: []cftypes.DistributionSummary{},
			},
		},
	}

	resources, err := awsclient.FetchCloudFrontDistributions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchCloudFrontDistributions_NilDistributionList(t *testing.T) {
	mock := &mockCloudFrontClient{
		output: &cloudfront.ListDistributionsOutput{
			DistributionList: nil,
		},
	}

	resources, err := awsclient.FetchCloudFrontDistributions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
