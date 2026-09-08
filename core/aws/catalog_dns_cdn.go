// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// colorCF derives the row colour from the distribution's findings, and for a
// row that carries none from the same predicate the fetcher used, over the two
// fields it wrote there.
func colorCF(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(cfWave1Findings(r.Fields["enabled"], r.Fields["status"]))
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
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			zone := strings.TrimPrefix(r.ID, "/hostedzone/")
			return consolelink.Global(region, "route53/v2/hostedzones#ListRecordSets/"+zone)
		},
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
			{Code: CodeR53QueryLoggingOff, Phrase: "query logging off", Severity: domain.SevWarn, Source: "wave2", Detail: "Nothing records who resolves names in this public zone, so a subdomain being probed or abused leaves no evidence. Create a query logging configuration for the zone."},
			{Code: CodeR53DanglingRecord, Phrase: "record points at an unassociated elastic IP", Severity: domain.SevBroken, Source: "wave2", Detail: "The record answers with an elastic IP this account holds but has attached to nothing, so every request for this name reaches an address that serves no traffic. Attach the address to the instance or load balancer meant to answer, or repoint the record."},
		},
	},
	{
		Name:           "CloudFront Distributions",
		ShortName:      "cf",
		HumanizeFields: []string{"status"},
		Aliases:        []string{"cf", "cloudfront", "cdn"},
		Category:       "DNS & CDN",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Global(region, "cloudfront/v4/home#/distributions/"+r.ID)
		},
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
			{Code: cfCodeDisabled, Phrase: "disabled (admin-off)", Severity: domain.SevDim, Source: "wave1", Detail: "The distribution is switched off, so it serves nothing and the edge locations answer with an error. Nothing here needs fixing unless it was meant to be serving; enable it, or delete it once you are sure."},
			{Code: cfCodeInProgress, Phrase: "deploying: config propagating", Severity: domain.SevWarn, Source: "wave1", Detail: "A configuration change is still reaching the edge locations, so viewers may get the old behaviour or the new one depending on where they are. Wait for it to finish before judging anything else about the distribution."},
			{Code: cfCodeInsecureProtocol, Phrase: "traffic allowed without TLS", Severity: domain.SevWarn, Source: "wave2"},
			{Code: CodeCFOriginBucketMissing, Phrase: "S3 origin bucket does not exist", Severity: domain.SevBroken, Source: "wave2", Detail: "The distribution forwards requests to a bucket that no longer exists, so those paths fail and anyone who creates a bucket with that name starts serving your traffic. Repoint the origin at a bucket you own, or remove it."},
			{Code: CodeCFDeprecatedTLS, Phrase: "minimum TLS below 1.2", Severity: domain.SevWarn, Source: "wave2", Detail: "Viewers may negotiate a protocol version with known weaknesses, which modern browsers already refuse. Raise the distribution's minimum protocol version to TLS 1.2 or later."},
			{Code: CodeCFLoggingOff, Phrase: "access logging off", Severity: domain.SevWarn, Source: "wave2", Detail: "The distribution records no request logs, so an attack or abuse pattern at the edge leaves nothing to investigate. Turn on standard logging and give it a destination."},
			{Code: CodeCFNoDefaultRootObject, Phrase: "no default root object", Severity: domain.SevWarn, Source: "wave2", Detail: "A request for the distribution root returns whatever the origin serves there, which can expose object names you did not mean to publish. Set a default root object such as index.html."},
			{Code: CodeCFS3OriginNoOAC, Phrase: "S3 origin without origin access control", Severity: domain.SevWarn, Source: "wave2", Detail: "The bucket behind this origin must be open to reach it through CloudFront, so viewers can bypass the distribution and read from the bucket directly. Attach an origin access control and restrict the bucket policy to it."},
			{Code: CodeCFDefaultCertificate, Phrase: "uses the default CloudFront certificate", Severity: domain.SevWarn, Source: "wave2", Detail: "The distribution serves custom domains with the default CloudFront certificate, so viewers reaching those names get a certificate mismatch warning. Attach a certificate that covers the aliases."},
			{Code: CodeCFNoGeoRestriction, Phrase: "no geo restriction", Severity: domain.SevWarn, Source: "wave2", Detail: "Content is served to every country, including any the account is not meant to serve. Add a geographic restriction if the distribution should be limited."},
		},
	},
	{
		Name:           "ACM Certificates",
		ShortName:      "acm",
		HumanizeFields: []string{"Type"},
		Aliases:        []string{"acm", "certificates", "certs"},
		Category:       "DNS & CDN",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			uuid := r.ID
			if idx := strings.LastIndex(r.ID, "/"); idx >= 0 {
				uuid = r.ID[idx+1:]
			}
			return consolelink.Regional(region, "acm/home?region="+region+"#/certificates/"+uuid)
		},
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
			{Code: acmCodeExpired, Phrase: "expired", Severity: domain.SevBroken, Source: "wave1", Detail: "The certificate has already expired, so every client reaching a listener that serves it refuses the connection. Replace it and confirm the listeners have picked up the new one."},
			{Code: acmCodeExpiresCritical, Phrase: "expires in <N day(s)>", Severity: domain.SevBroken, Source: "wave1", Detail: "The certificate expires within a week and every client reaching a listener that serves it will then refuse the connection. Renew or replace it now and confirm the listeners have picked up the new one."},
			{Code: acmCodeExpiresSoon, Phrase: "expires in <N day(s)>", Severity: domain.SevWarn, Source: "wave1", Detail: "The certificate expires within a month, which is enough time to renew it calmly and not enough to forget about it. Check that automatic renewal is configured and that its validation records are still published."},
			{Code: acmCodeOrphan, Phrase: "certificate not in use (orphan)", Severity: domain.SevWarn, Source: "wave1", Detail: "Nothing is serving this certificate, so it is renewed and tracked for no traffic, and it clutters the list an operator scans during an incident. Delete it, or attach it to the listener it was requested for."},
			{Code: acmCodeStatusPendingValidation, Phrase: "pending validation", Severity: domain.SevWarn, Source: "wave1", Detail: "The certificate has been requested but not issued: the domain is still waiting to be proved yours, so nothing can serve TLS with it yet. Publish the validation record in the domain's zone, or answer the validation email, before the request times out."},
			{Code: acmCodeStatusFailed, Phrase: "<status, in words>", Severity: domain.SevBroken, Source: "wave1", Detail: "The certificate cannot terminate TLS: it has expired, been revoked, failed issuance, or run out of time to validate, and the status says which. Anything still pointing at it is serving a broken handshake, so request a replacement and move the listeners onto it."},
			{Code: acmCodeStatusInactive, Phrase: "inactive", Severity: domain.SevDim, Source: "wave1", Detail: "An imported certificate marked inactive, because nothing is using it to terminate TLS. It costs nothing to keep, so this is a note rather than a fault; delete it once you are sure nothing will need it."},
			{Code: CodeACMWeakKey, Phrase: "weak key algorithm", Severity: domain.SevWarn, Source: "wave1", Detail: "The certificate's key is short enough to be worth attacking, and browsers are withdrawing trust from keys this size. Reissue the certificate with a key of 2048 bits or more, or an elliptic-curve key."},
		},
	},
	{
		Name:           "API Gateways",
		ShortName:      "apigw",
		HumanizeFields: []string{"protocol"},
		Aliases:        []string{"apigw", "apigateway", "api-gateway"},
		Category:       "DNS & CDN",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			if r.Fields["protocol"] == "REST" {
				return consolelink.Regional(region, "apigateway/home?region="+region+"#/apis/"+r.ID)
			}
			return consolelink.Regional(region, "apigateway/main/api-detail?api="+r.ID+"&region="+region)
		},
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
			{Code: apigwCodeStageConfigIssues, Phrase: "stage configuration issues", Severity: domain.SevWarn, Source: "wave2"},
			{Code: CodeAPIGWNoAuthorizerPublic, Phrase: "internet-facing with no authorizer", Severity: domain.SevBroken, Source: "wave2", Detail: "Anyone on the internet can call every route this gateway exposes, because nothing checks the caller's identity. Attach an authorizer, or scope the resource policy to the callers that should reach it."},
			{Code: CodeAPIGWNoAuthorizer, Phrase: "no authorizer", Severity: domain.SevWarn, Source: "wave2", Detail: "Nothing checks the caller's identity, so any client that can reach the network this gateway sits on can call every route. Attach an authorizer."},
			{Code: CodeAPIGWNoAccessLogs, Phrase: "no access logs", Severity: domain.SevWarn, Source: "wave2", Detail: "The stage records no access logs, so a burst of abusive or failing requests leaves nothing to investigate. Point the stage's access logging at a log group."},
			{Code: CodeAPIGWTracingOff, Phrase: "X-Ray tracing off", Severity: domain.SevWarn, Source: "wave2", Detail: "Requests through this stage are not traced, so a slow or failing integration cannot be followed to its cause. Turn on X-Ray tracing for the stage."},
			{Code: CodeAPIGWStageVariableSecret, Phrase: "credential in stage variables", Severity: domain.SevBroken, Source: "wave2", Detail: "A stage variable holds what looks like a credential, and stage variables are readable by anyone who can read the gateway's configuration. Move the value into Secrets Manager and reference it from the integration."},
		},
	},
}

var dnsCdnChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "R53 Records",
		ShortName: "r53_records",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			zoneID := r.Fields["zone_id"]
			if zoneID == "" {
				return ""
			}
			zone := strings.TrimPrefix(zoneID, "/hostedzone/")
			return consolelink.Global(region, "route53/v2/hostedzones#ListRecordSets/"+zone)
		},
		Columns:   resource.R53RecordColumns(),
		FieldKeys: []string{"name", "type", "ttl", "values", "zone_id"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchR53Records(ctx, c.Route53, parentCtx["zone_id"], continuationToken)
		}),
	},
}
