// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// cf_issue_enrichment.go — Wave 2 issue enrichment for the cf resource type.
package aws

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// cf canonical FindingCodes.
const (
	cfCodeInsecureProtocol domain.FindingCode = "cf.insecure-protocol"

	// cfCodeInProgress — Status==InProgress. Distribution config change is
	// still propagating to edge locations.
	cfCodeInProgress domain.FindingCode = "cf.status.in-progress"
	// cfCodeDisabled — Enabled==false. Distribution is administratively
	// disabled and not serving traffic.
	cfCodeDisabled domain.FindingCode = "cf.disabled"

	// CodeCFOriginBucketMissing — an S3 origin naming a bucket absent from
	// the account.
	CodeCFOriginBucketMissing domain.FindingCode = "cf.origin-bucket-missing"
	// CodeCFDeprecatedTLS — ViewerCertificate.MinimumProtocolVersion below
	// TLS 1.2.
	CodeCFDeprecatedTLS domain.FindingCode = "cf.deprecated-tls"
	// CodeCFLoggingOff — Logging.Enabled false or absent, and no standard
	// logging (v2) delivery for the distribution.
	CodeCFLoggingOff domain.FindingCode = "cf.logging-off"
	// CodeCFNoDefaultRootObject — DefaultRootObject empty.
	CodeCFNoDefaultRootObject domain.FindingCode = "cf.no-default-root-object"
	// CodeCFS3OriginNoOAC — an S3 origin with neither an origin access
	// control nor a legacy origin access identity.
	CodeCFS3OriginNoOAC domain.FindingCode = "cf.s3-origin-no-oac"
	// CodeCFDefaultCertificate — the default CloudFront certificate on a
	// distribution that also carries custom aliases.
	CodeCFDefaultCertificate domain.FindingCode = "cf.default-certificate"
	// CodeCFNoGeoRestriction — GeoRestriction.RestrictionType is none.
	CodeCFNoGeoRestriction domain.FindingCode = "cf.no-geo-restriction"
)

// cfTLSBelow12Word maps a minimum protocol version AWS still accepts but that
// sits below TLS 1.2 to the word the row renders, and returns "" for the
// versions that are fine. The two enums that both mean TLS 1.0 carry their
// policy year so the row stays distinct.
func cfTLSBelow12Word(v cftypes.MinimumProtocolVersion) string {
	switch v {
	case cftypes.MinimumProtocolVersionSSLv3:
		return "SSL 3.0"
	case cftypes.MinimumProtocolVersionTLSv1:
		return "TLS 1.0"
	case cftypes.MinimumProtocolVersionTLSv12016:
		return "TLS 1.0 (2016)"
	case cftypes.MinimumProtocolVersionTLSv112016:
		return "TLS 1.1 (2016)"
	default:
		return ""
	}
}

// errS3ListIncomplete is cfOriginBucketsGone's answer when the s3 list is not
// loaded in full: a bucket missing from it may sit on an unread page.
var errS3ListIncomplete = errors.New(checkListIncomplete("s3"))

// cfOriginBucketsGone answers which of the distribution's S3 origin buckets
// are gone. It runs outside the enricher's lock, since each answer may take a
// HeadBucket round trip.
//
// The cache lists this account's buckets, so it can confirm a bucket exists
// but never that one does not: an origin in another account is absent from it
// while working perfectly. Only HeadBucket can say gone, and only when it says
// not-found. A denial, or a redirect to the bucket's own region, is AWS
// answering about a bucket it found (s3HeadBucketSaysExists). Any other error
// — a 400 such as an expired token, a server fault past its retries, a
// timeout — answers nothing: the first is returned beside the buckets the
// other origins proved gone, as is errS3ListIncomplete when the cache cannot
// confirm anything.
// api is nil when the session has no S3 client, and the question is not asked.
func cfOriginBucketsGone(ctx context.Context, api S3HeadBucketAPI, origins []cftypes.Origin, knownBuckets map[string]bool) (map[string]bool, error) {
	if api == nil {
		return nil, nil
	}
	var gone map[string]bool
	var unread error
	for _, origin := range origins {
		bucket, isS3 := S3OriginBucket(aws.ToString(origin.DomainName))
		if !isS3 {
			continue
		}
		if knownBuckets == nil {
			return nil, errS3ListIncomplete
		}
		if knownBuckets[bucket] {
			continue
		}
		_, err := api.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
		if S3HeadBucketSaysMissing(err) {
			if gone == nil {
				gone = make(map[string]bool)
			}
			gone[bucket] = true
			continue
		}
		if err != nil && !s3HeadBucketSaysExists(err) && unread == nil {
			unread = err
		}
	}
	return gone, unread
}

// cfOrigins is the distribution's origin list: from its config when that was
// read, otherwise from the list row's DistributionSummary, which carries the
// same Origins.
func cfOrigins(r resource.Resource, cfg *cftypes.DistributionConfig) []cftypes.Origin {
	var origins *cftypes.Origins
	if cfg != nil {
		origins = cfg.Origins
	} else {
		summary, _ := assertStruct[cftypes.DistributionSummary](r.RawStruct)
		origins = summary.Origins
	}
	if origins == nil {
		return nil
	}
	return origins.Items
}

// cfMissingOriginFindings stands apart from cfConfigFindings because its
// origins may come from the list row: it runs when the config read failed.
func cfMissingOriginFindings(result *IssueEnricherResult, distID string, origins []cftypes.Origin, goneBuckets map[string]bool) {
	for _, origin := range origins {
		domainName := aws.ToString(origin.DomainName)
		if bucket, isS3 := S3OriginBucket(domainName); isS3 && goneBuckets[bucket] {
			setWave2Finding(result, distID, CodeCFOriginBucketMissing, []domain.DetailRow{{Label: "Origin", Value: domainName, Tier: "!"}})
		}
	}
}

// cfConfigFindings evaluates every config-derived posture row for one
// distribution. Each condition is independent: a distribution failing all of
// them carries one finding per condition, with its own supporting rows.
func cfConfigFindings(result *IssueEnricherResult, distID string, cfg *cftypes.DistributionConfig, loggingOff bool) {
	emit := func(code domain.FindingCode, rows ...domain.DetailRow) {
		setWave2Finding(result, distID, code, rows)
	}

	var origins []cftypes.Origin
	if cfg.Origins != nil {
		origins = cfg.Origins.Items
	}
	for _, origin := range origins {
		domainName := aws.ToString(origin.DomainName)
		if _, isS3 := S3OriginBucket(domainName); !isS3 {
			continue
		}
		// An origin access identity is deprecated, not absent: a distribution
		// already restricting its bucket through one is not exposed.
		// A website endpoint takes neither: it is public by construction and
		// has no private form for an access control to protect, so its
		// absence there is not a gap the operator can close.
		hasOAC := aws.ToString(origin.OriginAccessControlId) != ""
		hasOAI := origin.S3OriginConfig != nil && aws.ToString(origin.S3OriginConfig.OriginAccessIdentity) != ""
		if !hasOAC && !hasOAI && !isS3WebsiteEndpoint(domainName) {
			emit(CodeCFS3OriginNoOAC,
				domain.DetailRow{Label: "Origin", Value: domainName, Tier: "~"})
		}
	}

	if vc := cfg.ViewerCertificate; vc != nil {
		// CloudFront sets the security policy to TLSv1 on a default
		// certificate whatever the config says, so there is no minimum to
		// change on such a distribution.
		usesDefaultCert := aws.ToBool(vc.CloudFrontDefaultCertificate)
		if word := cfTLSBelow12Word(vc.MinimumProtocolVersion); word != "" && !usesDefaultCert {
			emit(CodeCFDeprecatedTLS,
				domain.DetailRow{Label: "Minimum TLS version", Value: word, Tier: "~"})
		}
		// A distribution with no alias legitimately serves on its
		// cloudfront.net name with the default certificate.
		if usesDefaultCert && cfg.Aliases != nil && len(cfg.Aliases.Items) > 0 {
			emit(CodeCFDefaultCertificate,
				domain.DetailRow{Label: "Alias", Value: cfg.Aliases.Items[0], Tier: "~"})
		}
	}

	if loggingOff {
		emit(CodeCFLoggingOff,
			domain.DetailRow{Label: "Log destination", Value: "no legacy bucket, no v2 delivery", Tier: "~"})
	}

	if aws.ToString(cfg.DefaultRootObject) == "" {
		emit(CodeCFNoDefaultRootObject,
			domain.DetailRow{Label: "Default root object", Value: "none", Tier: "~"})
	}

	if cfg.Restrictions != nil && cfg.Restrictions.GeoRestriction != nil &&
		cfg.Restrictions.GeoRestriction.RestrictionType == cftypes.GeoRestrictionTypeNone {
		emit(CodeCFNoGeoRestriction,
			domain.DetailRow{Label: "Countries", Value: "none", Tier: "~"})
	}
}

// cfLegacyLoggingOff reports whether the distribution's own logging block is
// off. CloudFront omits the block entirely when logging was never configured,
// so absent is off rather than unknown.
func cfLegacyLoggingOff(cfg *cftypes.DistributionConfig) bool {
	return cfg.Logging == nil || !aws.ToBool(cfg.Logging.Enabled)
}

// cfStandardLoggingARNs returns the resource ARNs a standard logging (v2)
// delivery sends logs for: the sources named by a delivery, and the ARNs those
// sources log. A source with no delivery sends nothing anywhere. CloudFront
// keeps these in CloudWatch Logs in us-east-1 whatever the destination's
// Region. A nil api reads as no v2 delivery.
func cfStandardLoggingARNs(ctx context.Context, api CWLogsDeliveriesAPI) (map[string]bool, error) {
	if api == nil {
		return nil, nil
	}
	deliveries, deliveriesComplete, deliveriesErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]cwlogstypes.Delivery, *string, error) {
		out, err := api.DescribeDeliveries(ctx, &cloudwatchlogs.DescribeDeliveriesInput{NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Deliveries, out.NextToken, nil
	})
	sources, sourcesComplete, sourcesErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]cwlogstypes.DeliverySource, *string, error) {
		out, err := api.DescribeDeliverySources(ctx, &cloudwatchlogs.DescribeDeliverySourcesInput{NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.DeliverySources, out.NextToken, nil
	})
	if err := errors.Join(deliveriesErr, sourcesErr); err != nil {
		return nil, err
	}
	if !deliveriesComplete || !sourcesComplete {
		return nil, errPageCap
	}
	delivered := make(map[string]bool, len(deliveries))
	for _, d := range deliveries {
		delivered[aws.ToString(d.DeliverySourceName)] = true
	}
	logged := make(map[string]bool)
	for _, s := range sources {
		if delivered[aws.ToString(s.Name)] {
			for _, arn := range s.ResourceArns {
				logged[arn] = true
			}
		}
	}
	return logged, nil
}

// cachedBucketNames returns the set of bucket names the s3 cache holds, or nil
// when the cache is absent or truncated and therefore not evidence of absence.
func cachedBucketNames(cache resource.ResourceCache) map[string]bool {
	entry, ok := cache["s3"]
	if !ok || entry.IsTruncated {
		return nil
	}
	names := make(map[string]bool, len(entry.Resources))
	for _, b := range entry.Resources {
		names[b.ID] = true
	}
	return names
}

// EnrichCloudFrontDistribution calls GetDistributionConfig per distribution (cap EnrichmentCap)
// and returns a Finding for any distribution with insecure viewer or origin protocol settings.
//
// Findings (severity "~" — informational):
//   - the default or any ordered cache behaviour with ViewerProtocolPolicy "allow-all"
//   - Any Origin with CustomOriginConfig.OriginProtocolPolicy == "http-only" → "origin without TLS"
//
// Skip if clients.CloudFront == nil. Per-distribution errors → truncated.
func EnrichCloudFrontDistribution(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
	}
	if clients.CloudFront == nil {
		return result, nil
	}
	knownBuckets := cachedBucketNames(cache)
	headAPI, _ := clients.S3.(S3HeadBucketAPI)
	deliveriesAPI, _ := clients.InRegion("us-east-1").CloudWatchLogs.(CWLogsDeliveriesAPI)
	// The deliveries are account-wide and matter only to a distribution with
	// legacy logging off, so they are read when the first such one is found.
	standardLogging := sync.OnceValues(func() (map[string]bool, error) {
		return cfStandardLoggingARNs(ctx, deliveriesAPI)
	})
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		distID := r.ID
		if distID == "" {
			return
		}
		out, err := clients.CloudFront.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{
			Id: aws.String(distID),
		})
		var cfg *cftypes.DistributionConfig
		if err == nil {
			cfg = out.DistributionConfig
		}
		origins := cfOrigins(r, cfg)
		gone, goneErr := cfOriginBucketsGone(ctx, headAPI, origins, knownBuckets)
		var standardLogged map[string]bool
		var standardErr error
		if cfg != nil && cfLegacyLoggingOff(cfg) {
			standardLogged, standardErr = standardLogging()
		}
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
		}
		switch {
		case errors.Is(goneErr, errS3ListIncomplete):
			markUninspected(&result, r.ID, goneErr.Error())
		case goneErr != nil:
			MarkSkipped(&result, r.ID, &failures, goneErr)
		}
		cfMissingOriginFindings(&result, distID, origins, gone)
		if cfg == nil {
			return
		}
		var rows []domain.DetailRow

		// A request matching an ordered behaviour's path is served by that
		// behaviour's policy alone, whatever the default says.
		var plainHTTP []string
		if cfg.DefaultCacheBehavior != nil &&
			cfg.DefaultCacheBehavior.ViewerProtocolPolicy == cftypes.ViewerProtocolPolicyAllowAll {
			plainHTTP = append(plainHTTP, "Default (*)")
		}
		if cfg.CacheBehaviors != nil {
			for _, b := range cfg.CacheBehaviors.Items {
				if b.ViewerProtocolPolicy == cftypes.ViewerProtocolPolicyAllowAll {
					plainHTTP = append(plainHTTP, aws.ToString(b.PathPattern))
				}
			}
		}
		if len(plainHTTP) > 0 {
			rows = append(rows, domain.DetailRow{
				Label: "Viewer protocol policy",
				Value: "allow-all on " + strings.Join(plainHTTP, ", "),
				Tier:  "~",
			})
		}

		// Check origin protocol policies.
		if cfg.Origins != nil {
			for _, origin := range cfg.Origins.Items {
				if origin.CustomOriginConfig != nil &&
					origin.CustomOriginConfig.OriginProtocolPolicy == cftypes.OriginProtocolPolicyHttpOnly &&
					// An S3 static-website endpoint serves HTTP only, so
					// http-only is the sole policy CloudFront accepts for it
					// and there is nothing here for the operator to switch.
					// Reaching that content over TLS means moving to the REST
					// endpoint with origin access control, which is a
					// different change from the one this finding asks for.
					!isS3WebsiteEndpoint(aws.ToString(origin.DomainName)) {
					originID := ""
					if origin.Id != nil {
						originID = *origin.Id
					}
					rows = append(rows, domain.DetailRow{
						Label: "Origin",
						Value: originID,
						Tier:  "~",
					})
					rows = append(rows, domain.DetailRow{
						Label: "Connection to origin",
						Value: "http-only",
						Tier:  "~",
					})
				}
			}
		}

		loggingOff := false
		if cfLegacyLoggingOff(cfg) {
			if standardErr != nil {
				MarkSkipped(&result, r.ID, &failures, standardErr)
			} else {
				loggingOff = !standardLogged[r.Fields["arn"]]
			}
		}
		cfConfigFindings(&result, distID, cfg, loggingOff)

		if len(rows) == 0 {
			return
		}
		setWave2Finding(&result, distID, cfCodeInsecureProtocol, rows)
	})
	return result, errors.Join(loopErr, AggregateFailures("distribution config and origin buckets", failures, n))
}
