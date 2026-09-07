package unit

// aws_cf_enricher_test.go — Behavioral tests for EnrichCloudFrontDistribution.
//
// Contract assertions:
//   - GetDistributionConfig is called once per CF resource (keyed by distribution ID).
//   - DefaultCacheBehavior.ViewerProtocolPolicy=redirect-to-https AND all origins https-only → 0 findings.
//   - DefaultCacheBehavior.ViewerProtocolPolicy=allow-all → 1 finding sev "~" "no HTTPS redirect" for that distro.
//   - An origin with CustomOriginConfig.OriginProtocolPolicy=http-only → 1 finding sev "~" "origin without TLS" for that distro.
//   - clients.CloudFront == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for a resource → 0 findings for that resource, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// cfGetDistributionConfigFake implements CloudFrontAPI for enrichment testing.
// It embeds the aggregate interface and overrides only GetDistributionConfig.
// The results map is keyed by distribution ID so the fake can serve different
// responses per resource.
type cfGetDistributionConfigFake struct {
	awsclient.CloudFrontAPI
	// results maps distribution ID → DistributionConfig.
	results map[string]*cftypes.DistributionConfig
	// errByID maps distribution ID → error; overrides results when set.
	errByID map[string]error
}

func (f *cfGetDistributionConfigFake) GetDistributionConfig(
	_ context.Context,
	in *cloudfront.GetDistributionConfigInput,
	_ ...func(*cloudfront.Options),
) (*cloudfront.GetDistributionConfigOutput, error) {
	id := ""
	if in != nil && in.Id != nil {
		id = *in.Id
	}
	if f.errByID != nil {
		if err, ok := f.errByID[id]; ok {
			return nil, err
		}
	}
	cfg, ok := f.results[id]
	if !ok {
		return &cloudfront.GetDistributionConfigOutput{}, nil
	}
	return &cloudfront.GetDistributionConfigOutput{DistributionConfig: cfg}, nil
}

// Compile-time check: cfGetDistributionConfigFake satisfies CloudFrontAPI.
var _ awsclient.CloudFrontAPI = (*cfGetDistributionConfigFake)(nil)

// cfDistroResources returns a slice of CF Resource stubs with the given distribution IDs.
func cfDistroResources(ids ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		res = append(res, resource.Resource{
			ID:   id,
			Name: id,
			Fields: map[string]string{
				"distribution_id": id,
				"domain_name":     id + ".cloudfront.net",
				"status":          "Deployed",
				"enabled":         "true",
				"aliases":         "",
				"price_class":     "PriceClass_100",
			},
		})
	}
	return res
}

// cfDistroConfigRedirectHTTPS builds a DistributionConfig with ViewerProtocolPolicy=redirect-to-https
// and all origins using https-only origin protocol.
func cfDistroConfigRedirectHTTPS(id string) *cftypes.DistributionConfig {
	return cfHealthyForW6ARows(&cftypes.DistributionConfig{
		Comment: aws.String("test distribution " + id),
		Enabled: aws.Bool(true),
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
			ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
			TargetOriginId:       aws.String("origin-1"),
		},
		Origins: &cftypes.Origins{
			Quantity: aws.Int32(1),
			Items: []cftypes.Origin{
				{
					Id:         aws.String("origin-1"),
					DomainName: aws.String("example.com"),
					CustomOriginConfig: &cftypes.CustomOriginConfig{
						HTTPPort:             aws.Int32(80),
						HTTPSPort:            aws.Int32(443),
						OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpsOnly,
					},
				},
			},
		},
	})
}

// cfHealthyForW6ARows fills the settings batch w6a added rows for — access
// logging, a default root object, a geo restriction, a custom certificate at
// TLS 1.2, and an origin access control on every S3 origin. These tests are
// about cf.insecure-protocol; without this their minimal configs also trip
// six unrelated rows, and the assertions below count findings rather than
// look one up by code.
func cfHealthyForW6ARows(cfg *cftypes.DistributionConfig) *cftypes.DistributionConfig {
	cfg.DefaultRootObject = aws.String("index.html")
	cfg.Logging = &cftypes.LoggingConfig{
		Enabled: aws.Bool(true),
		Bucket:  aws.String("acme-cdn-logs.s3.amazonaws.com"),
		Prefix:  aws.String("cf/"),
	}
	cfg.Restrictions = &cftypes.Restrictions{GeoRestriction: &cftypes.GeoRestriction{
		RestrictionType: cftypes.GeoRestrictionTypeWhitelist,
		Quantity:        aws.Int32(1),
		Items:           []string{"DE"},
	}}
	cfg.ViewerCertificate = &cftypes.ViewerCertificate{
		ACMCertificateArn:            aws.String("arn:aws:acm:us-east-1:123456789012:certificate/1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"),
		CloudFrontDefaultCertificate: aws.Bool(false),
		MinimumProtocolVersion:       cftypes.MinimumProtocolVersionTLSv122021,
	}
	if cfg.Origins != nil {
		for i := range cfg.Origins.Items {
			if cfg.Origins.Items[i].S3OriginConfig != nil {
				cfg.Origins.Items[i].OriginAccessControlId = aws.String("E1A2B3C4D5E6F7")
			}
		}
	}
	return cfg
}

const (
	cfDistroID1 = "EDFDVBD6EXAMPLE1"
	cfDistroID2 = "EDFDVBD6EXAMPLE2"
)

// TestEnrichCloudFrontDistribution_HTTPSRedirectAndTLSOriginsProducesNoFindings verifies
// that when all distributions have ViewerProtocolPolicy=redirect-to-https and all origins
// use https-only protocol policy, no findings are produced.
func TestEnrichCloudFrontDistribution_HTTPSRedirectAndTLSOriginsProducesNoFindings(t *testing.T) {
	fake := &cfGetDistributionConfigFake{
		results: map[string]*cftypes.DistributionConfig{
			cfDistroID1: cfDistroConfigRedirectHTTPS(cfDistroID1),
			cfDistroID2: cfDistroConfigRedirectHTTPS(cfDistroID2),
		},
	}
	clients := &awsclient.ServiceClients{CloudFront: fake}
	resources := cfDistroResources(cfDistroID1, cfDistroID2)

	result, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

// TestEnrichCloudFrontDistribution_AllowAllViewerProtocolProducesFindingSevTilde verifies
// that when distro-1 has ViewerProtocolPolicy=allow-all, a finding with severity "~" and
// a summary containing "HTTPS redirect" is produced for distro-1 only.
func TestEnrichCloudFrontDistribution_AllowAllViewerProtocolProducesFindingSevTilde(t *testing.T) {
	distro1NoHTTPS := cfHealthyForW6ARows(&cftypes.DistributionConfig{
		Comment: aws.String("test distribution " + cfDistroID1),
		Enabled: aws.Bool(true),
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
			ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyAllowAll,
			TargetOriginId:       aws.String("origin-1"),
		},
		Origins: &cftypes.Origins{
			Quantity: aws.Int32(1),
			Items: []cftypes.Origin{
				{
					Id:         aws.String("origin-1"),
					DomainName: aws.String("example.com"),
					CustomOriginConfig: &cftypes.CustomOriginConfig{
						HTTPPort:             aws.Int32(80),
						HTTPSPort:            aws.Int32(443),
						OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpsOnly,
					},
				},
			},
		},
	})
	fake := &cfGetDistributionConfigFake{
		results: map[string]*cftypes.DistributionConfig{
			cfDistroID1: distro1NoHTTPS,
			cfDistroID2: cfDistroConfigRedirectHTTPS(cfDistroID2),
		},
	}
	clients := &awsclient.ServiceClients{CloudFront: fake}
	resources := cfDistroResources(cfDistroID1, cfDistroID2)

	result, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[cfDistroID1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no HTTPS redirect)", cfDistroID1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	// Inverted for spec row "phrase": the wording belongs to the code and the
	// offending item is a supporting row. Do not restore the old assertion.
	if want := catalog.Phrase("cf.insecure-protocol"); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	if rows := fmt.Sprintf("%v", result.AttentionDetails[cfDistroID1]["cf.insecure-protocol"].Rows); !strings.Contains(strings.ToLower(rows), "allow-all") {
		t.Errorf("no supporting row names the viewer protocol policy: %s", rows)
	}
	if _, ok := result.Findings[cfDistroID2]; ok {
		t.Error("distro-2 must NOT appear in Findings — it has redirect-to-https")
	}
}

// TestEnrichCloudFrontDistribution_HTTPOnlyOriginProducesFindingSevTilde verifies that when
// distro-1 has an origin with OriginProtocolPolicy=http-only, a finding with severity "~"
// and a summary containing "origin" and "TLS" is produced for distro-1 only.
func TestEnrichCloudFrontDistribution_HTTPOnlyOriginProducesFindingSevTilde(t *testing.T) {
	distro1HTTPOrigin := cfHealthyForW6ARows(&cftypes.DistributionConfig{
		Comment: aws.String("test distribution " + cfDistroID1),
		Enabled: aws.Bool(true),
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
			ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
			TargetOriginId:       aws.String("origin-1"),
		},
		Origins: &cftypes.Origins{
			Quantity: aws.Int32(1),
			Items: []cftypes.Origin{
				{
					Id:         aws.String("origin-1"),
					DomainName: aws.String("example.com"),
					CustomOriginConfig: &cftypes.CustomOriginConfig{
						HTTPPort:             aws.Int32(80),
						HTTPSPort:            aws.Int32(443),
						OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpOnly,
					},
				},
			},
		},
	})
	fake := &cfGetDistributionConfigFake{
		results: map[string]*cftypes.DistributionConfig{
			cfDistroID1: distro1HTTPOrigin,
			cfDistroID2: cfDistroConfigRedirectHTTPS(cfDistroID2),
		},
	}
	clients := &awsclient.ServiceClients{CloudFront: fake}
	resources := cfDistroResources(cfDistroID1, cfDistroID2)

	result, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[cfDistroID1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (http-only origin)", cfDistroID1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	// Inverted for spec row "phrase": the wording belongs to the code and the
	// offending item is a supporting row. Do not restore the old assertion.
	if want := catalog.Phrase("cf.insecure-protocol"); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	if rows := fmt.Sprintf("%v", result.AttentionDetails[cfDistroID1]["cf.insecure-protocol"].Rows); !strings.Contains(strings.ToLower(rows), "origin") {
		t.Errorf("no supporting row names the offending origin: %s", rows)
	}
	if _, ok := result.Findings[cfDistroID2]; ok {
		t.Error("distro-2 must NOT appear in Findings — all its origins use https-only")
	}
}

// TestEnrichCloudFrontDistribution_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.CloudFront is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichCloudFrontDistribution_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{CloudFront: nil}

	result, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients, cfDistroResources(cfDistroID1, cfDistroID2), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when CloudFront client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichCloudFrontDistribution_APIErrorMarksRowTruncatedIDNotBadge verifies that when
// the API call for distro-1 returns an error, the enricher marks that
// distribution's row via TruncatedIDs, produces 0 findings for that distro,
// and does not propagate the error. cf is a "~"-only enricher (IssueCount
// always 0), so the coverage gap must never lower-bound the aggregate issue
// badge — Truncated stays false.
func TestEnrichCloudFrontDistribution_APIErrorMarksRowTruncatedIDNotBadge(t *testing.T) {
	apiErr := errors.New("cloudfront: GetDistributionConfig throttled")
	fake := &cfGetDistributionConfigFake{
		errByID: map[string]error{
			cfDistroID1: apiErr,
		},
		results: map[string]*cftypes.DistributionConfig{
			cfDistroID2: cfDistroConfigRedirectHTTPS(cfDistroID2),
		},
	}
	clients := &awsclient.ServiceClients{CloudFront: fake}
	resources := cfDistroResources(cfDistroID1, cfDistroID2)

	result, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients, resources, nil)
	// INVERTED for the "skipped" spec row 5: this required err == nil, which
	// meant the row could render "?" with nothing in the error log to say what
	// refused. The call that failed is recorded now. Do not restore.
	if err == nil {
		t.Fatal("a failed per-resource call returned no error — the reason never reaches the log")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if result.Truncated {
		t.Error("Truncated must stay false: cf is a \"~\"-only enricher, so an API error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if !result.TruncatedIDs[cfDistroID1] {
		t.Errorf("TruncatedIDs[%q] must be true — the GetDistributionConfig error must mark that distribution's row with a \"?\" coverage gap", cfDistroID1)
	}
}
