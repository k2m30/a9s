// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// colorCF classifies a CloudFront distribution. Prefers colorFromAnyFinding
// so real fetched resources (Findings populated by cfWave1Findings, cf.go,
// Source: "wave1", and EnrichCloudFrontDistribution, Source: "wave2:cf")
// color from their own Finding; the raw-field checks below are the
// identical-precedence fallback for callers that construct a Resource with
// only Fields set (e.g. qa_cf_color_test.go).
func colorCF(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	if r.Fields["enabled"] == "false" {
		return domain.ColorDim
	}
	switch r.Fields["status"] {
	case "Deployed":
		return domain.ColorHealthy
	case "InProgress":
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

// colorAPIGW classifies an API Gateway. All signals are Wave-2-only
// (apigwCodeNoDeployedStages, apigwCodeStageConfigIssues) — no raw-field
// structural state exists at Wave 1.
func colorAPIGW(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

var dnsCdnTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Route 53 Hosted Zones",
		ShortName:     "r53",
		Aliases:       []string{"r53", "route53", "dns", "hosted-zones"},
		Category:      "DNS & CDN",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 36, Sortable: true},
			{Key: "zone_id", Title: "Zone ID", Width: 30, Sortable: true},
			{Key: "record_count", Title: "Records", Width: 9, Sortable: true},
			{Key: "private_zone", Title: "Private", Width: 9, Sortable: true},
			{Key: "comment", Title: "Comment", Width: 30, Sortable: false},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "r53_records",
			Key:            "enter",
			ContextKeys:    map[string]string{"zone_id": "ID", "zone_name": "Name"},
			DisplayNameKey: "zone_name",
		}},
		Color: r53Color,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchHostedZonesPage(ctx, c.Route53, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichRoute53Zone, Priority: 100},
		FieldKeys: []string{"zone_id", "name", "record_count", "private_zone", "comment", "alias_targets", "s3website_alias_names"},
		Related: []domain.RelatedDef{
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkR53ELB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkR53CF, NeedsTargetCache: true, Truncated: true},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkR53ACM},
			{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkR53APIGW, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkR53Logs, Truncated: true},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkR53S3, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpc", DisplayName: "VPCs", Checker: checkR53VPC},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("r53")},
		},
		Findings: []catalog.FindingDef{
			{Code: r53CodeUnusedZone, Phrase: "only default NS/SOA records remain", Severity: domain.SevWarn, Source: "wave1"},
			{Code: r53CodeOrphanPrivateZone, Phrase: "private zone with no VPC associations (orphan)", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "CloudFront Distributions",
		ShortName:     "cf",
		Aliases:       []string{"cf", "cloudfront", "cdn"},
		Category:      "DNS & CDN",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "domain_name", Title: "Domain Name", Width: 40, Sortable: true},
			{Key: "distribution_id", Title: "Distribution ID", Width: 16, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "enabled", Title: "Enabled", Width: 9, Sortable: true},
			{Key: "aliases", Title: "Aliases", Width: 30, Sortable: false},
			{Key: "price_class", Title: "Price Class", Width: 16, Sortable: true},
		},
		Color: colorCF,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchCloudFrontDistributionsPage(ctx, c.CloudFront, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichCloudFrontDistribution, Priority: 100},
		// lambda_function_arns — required for the lambda:cf related-panel
		// pivot (checkLambdaCF); cache-restored rows have no RawStruct, so
		// this must survive the YAML cache round-trip via FieldKeys.
		FieldKeys: []string{"distribution_id", "domain_name", "status", "enabled", "aliases", "price_class", "lambda_function_arns"},
		Related: []domain.RelatedDef{
			{TargetType: "s3", DisplayName: "S3 Buckets (origin)", Checker: checkCfS3, NeedsTargetCache: true, Truncated: true},
			{TargetType: "elb", DisplayName: "Load Balancers (origin)", Checker: checkCfELB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "waf", DisplayName: "WAF Web ACLs", Checker: checkCfWAF, NeedsTargetCache: true, Truncated: true},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkCfACM, NeedsTargetCache: true, Truncated: true},
			{TargetType: "r53", DisplayName: "Route 53 Zones", Checker: checkCfR53, Truncated: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkCfAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda@Edge", Checker: checkCfLambda},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkCfLogs},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("cf")},
		},
		// cftypes.DistributionSummary: no NavigableFields — Origins[].DomainName is a hostname
		// (e.g. bucket.s3.amazonaws.com), not a bucket name ID; all relationships handled by
		// checkCf* related checkers at runtime. WebACLId is on GetDistributionConfig, not the summary.
		Findings: []catalog.FindingDef{
			{Code: cfCodeInsecureProtocol, Phrase: "no HTTPS redirect (insecure); origin without TLS", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "ACM Certificates",
		ShortName:     "acm",
		Aliases:       []string{"acm", "certificates", "certs"},
		Category:      "DNS & CDN",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "domain_name", Title: "Domain Name", Width: 40, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "not_after", Title: "Expires", Width: 22, Sortable: true},
			{Key: "in_use", Title: "In Use", Width: 8, Sortable: true},
		},
		Color: acmColor,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchACMCertificatesPage(ctx, c.ACM, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichACMCertificate, Priority: 100},
		FieldKeys: []string{"domain_name", "status", "type", "not_after", "in_use", "days_left"},
		Related: []domain.RelatedDef{
			{TargetType: "cf", DisplayName: "CloudFront Distros", Checker: checkACMCF, NeedsTargetCache: true, Truncated: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkACMELB},
			{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkACMAPIGW},
			{TargetType: "r53", DisplayName: "Route 53 Zones", Checker: checkACMR53, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("acm")},
		},
		// No NavigableFields — CertificateSummary has no forward refs to other resource types
		Findings: []catalog.FindingDef{
			{Code: acmCodeExpiresSoon, Phrase: "expires in <N> days", Severity: domain.SevBroken, Source: "wave2"},
			{Code: acmCodeOrphan, Phrase: "certificate not in use (orphan)", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "API Gateways",
		ShortName:     "apigw",
		Aliases:       []string{"apigw", "apigateway", "api-gateway"},
		Category:      "DNS & CDN",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "api_id", Title: "API ID", Width: 14, Sortable: true},
			{Key: "protocol", Title: "Protocol", Width: 12, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 50, Sortable: false},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Color:                  colorAPIGW,
		Fetcher:                fetcherWithClients(FetchAPIGatewaysPageMerged),
		Wave2:                  IssueEnricher{Fn: EnrichAPIGatewayStage, Priority: 100},
		FieldKeys:              []string{"api_id", "name", "protocol", "endpoint", "description"},
		IssueEnricherFieldKeys: []string{"stages_count"},
		Related: []domain.RelatedDef{
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkApigwLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkApigwLambda},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkApigwACM},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkApigwAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkApigwCF, Truncated: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkApigwELB, Truncated: true},
			// Weak pair (3-sometimes/2-no consensus). API Gateway has no direct KMS field;
			// we follow Lambda integrations as a best effort.
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkApigwKMS, NeedsTargetCache: false},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkApigwRole},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("apigw")},
		},
		Findings: []catalog.FindingDef{
			{Code: apigwCodeNoDeployedStages, Phrase: "no deployed stages", Severity: domain.SevWarn, Source: "wave2"},
			{Code: apigwCodeStageConfigIssues, Phrase: "no throttling configured (DoS risk); access logs disabled", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
}

var dnsCdnChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "R53 Records",
		ShortName: "r53_records",
		Columns:   resource.R53RecordColumns(),
		FieldKeys: []string{"name", "type", "ttl", "values"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchR53Records(ctx, c.Route53, parentCtx["zone_id"], continuationToken)
		}),
	},
}
