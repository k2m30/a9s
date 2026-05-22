package aws

import (
	"context"
	"fmt"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func colorCF(r domain.Resource) domain.Color {
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

func colorAPIGW(_ domain.Resource) domain.Color { return domain.ColorHealthy }

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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCloudFrontDistributionsPage(ctx, c.CloudFront, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichCloudFrontDistribution, Priority: 100},
		FieldKeys: []string{"distribution_id", "domain_name", "status", "enabled", "aliases", "price_class"},
		Related: []domain.RelatedDef{
			{TargetType: "s3", DisplayName: "S3 Buckets (origin)", Checker: checkCfS3, NeedsTargetCache: true},
			{TargetType: "elb", DisplayName: "Load Balancers (origin)", Checker: checkCfELB, NeedsTargetCache: true},
			{TargetType: "waf", DisplayName: "WAF Web ACLs", Checker: checkCfWAF, NeedsTargetCache: true},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkCfACM, NeedsTargetCache: true},
			{TargetType: "r53", DisplayName: "Route 53 Zones", Checker: checkCfR53},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkCfAlarm, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda@Edge", Checker: checkCfLambda},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkCfLogs},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("cf")},
		},
		// cftypes.DistributionSummary: no NavigableFields — Origins[].DomainName is a hostname
		// (e.g. bucket.s3.amazonaws.com), not a bucket name ID; all relationships handled by
		// checkCf* related checkers at runtime. WebACLId is on GetDistributionConfig, not the summary.
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
		Color: colorAPIGW,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchAPIGatewaysPageMerged(ctx, c, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichAPIGatewayStage, Priority: 100},
		FieldKeys: []string{"api_id", "name", "protocol", "endpoint", "description"},
		Related: []domain.RelatedDef{
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkApigwLogs, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkApigwLambda},
			{TargetType: "waf", DisplayName: "WAF Web ACLs", Checker: checkApigwWAF},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkApigwACM},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkApigwAlarm, NeedsTargetCache: true},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkApigwCF},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkApigwELB},
			// Weak pair (3-sometimes/2-no consensus). API Gateway has no direct KMS field;
			// we follow Lambda integrations as a best effort.
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkApigwKMS, NeedsTargetCache: false},
			{TargetType: "r53", DisplayName: "Route 53 Zones", Checker: checkApigwR53},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkApigwRole},
			{TargetType: "sfn", DisplayName: "Step Functions", Checker: checkApigwSFN},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkApigwSNS},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkApigwVPCE},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("apigw")},
		},
	},
}
