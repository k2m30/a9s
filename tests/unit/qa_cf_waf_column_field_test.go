package unit

// The cf WAF column (defaults_dns_cdn.go,
// Path "WebACLId") resolves from the DistributionSummary list-API shape via
// fieldpath.ExtractScalar.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestFetchCloudFrontDistributions_WAFColumnResolves verifies that the WAF
// column path "WebACLId" resolves to a non-empty value from the DistributionSummary
// RawStruct for at least one distribution that has a WebACL attached.
//
// cftypes.DistributionSummary embeds WebACLId as a *string field.
// If the field is absent from the SDK struct, fieldpath.ExtractScalar returns "".
func TestFetchCloudFrontDistributions_WAFColumnResolves(t *testing.T) {
	now := time.Now()
	webACLID := "arn:aws:wafv2:us-east-1:123456789012:global/webacl/MyWebACL/abc12345"

	mock := &fakeCloudFrontListDistributions{
		Output: &cloudfront.ListDistributionsOutput{
			DistributionList: &cftypes.DistributionList{
				Items: []cftypes.DistributionSummary{
					{
						Id:               aws.String("EWAF001"),
						DomainName:       aws.String("d111.cloudfront.net"),
						Status:           aws.String("Deployed"),
						Enabled:          aws.Bool(true),
						LastModifiedTime: &now,
						ARN:              aws.String("arn:aws:cloudfront::123456789012:distribution/EWAF001"),
						Aliases:          &cftypes.Aliases{Quantity: aws.Int32(0)},
						Origins:          &cftypes.Origins{Quantity: aws.Int32(1)},
						PriceClass:       cftypes.PriceClassPriceClassAll,
						HttpVersion:      cftypes.HttpVersionHttp2,
						WebACLId:         aws.String(webACLID),
					},
					{
						Id:               aws.String("ENOWAF002"),
						DomainName:       aws.String("d222.cloudfront.net"),
						Status:           aws.String("Deployed"),
						Enabled:          aws.Bool(true),
						LastModifiedTime: &now,
						ARN:              aws.String("arn:aws:cloudfront::123456789012:distribution/ENOWAF002"),
						Aliases:          &cftypes.Aliases{Quantity: aws.Int32(0)},
						Origins:          &cftypes.Origins{Quantity: aws.Int32(1)},
						PriceClass:       cftypes.PriceClassPriceClassAll,
						HttpVersion:      cftypes.HttpVersionHttp2,
						// WebACLId intentionally nil — no WAF attached
					},
				},
			},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudFrontDistributionsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	wafDist := resources[0]
	if wafDist.ID != "EWAF001" {
		t.Fatalf("expected first resource to be EWAF001, got %q", wafDist.ID)
	}

	resolved := fieldpath.ExtractScalar(wafDist.RawStruct, "WebACLId")
	if resolved == "" {
		t.Errorf("distribution %q: fieldpath.ExtractScalar(RawStruct, \"WebACLId\") returned empty string "+
			"— WAF column will be blank for every row (P2.5 bug: WebACLId is on GetDistributionConfig, "+
			"not DistributionSummary, or the field name differs in the SDK struct)", wafDist.ID)
	}
}
