// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// cf_issue_enrichment.go — Wave 2 issue enrichment for the cf resource type.
package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
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
	// CodeCFLoggingOff — Logging.Enabled false or absent.
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

// cfBucketGoneFunc returns the authoritative "does this bucket exist" check,
// or nil when the session has no S3 client to ask with. HeadBucket is the
// only read-only call that answers for a bucket outside this account.
func cfBucketGoneFunc(clients *ServiceClients) func(context.Context, string) bool {
	api, ok := clients.S3.(S3HeadBucketAPI)
	if !ok || api == nil {
		return nil
	}
	return func(ctx context.Context, bucket string) bool {
		_, err := api.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
		return S3HeadBucketSaysMissing(err)
	}
}

// cfConfigFindings evaluates every config-derived posture row for one
// distribution. Each condition is independent: a distribution failing all of
// them carries one finding per condition, with its own supporting rows.
//
// knownBuckets is nil when the s3 cache is absent or truncated, which means
// the origin-bucket join cannot tell a deleted bucket from an unloaded page
// and is skipped rather than guessed.
//
// bucketGone answers whether a bucket the cache does not list is genuinely
// absent. It is nil when the session has no S3 client to ask, and an
// unanswerable question raises nothing.
func cfConfigFindings(ctx context.Context, result *IssueEnricherResult, distID string, cfg *cftypes.DistributionConfig, knownBuckets map[string]bool, bucketGone func(context.Context, string) bool) {
	emit := func(code domain.FindingCode, rows ...domain.DetailRow) {
		setWave2Finding(result, distID, code, "cf", rows)
	}

	var origins []cftypes.Origin
	if cfg.Origins != nil {
		origins = cfg.Origins.Items
	}
	for _, origin := range origins {
		domainName := aws.ToString(origin.DomainName)
		bucket, isS3 := S3OriginBucket(domainName)
		if !isS3 {
			continue
		}
		// The cache lists this account's buckets, so it can confirm a bucket
		// exists but never that one does not: an origin in another account is
		// absent from it while working perfectly. Only HeadBucket can say
		// gone, and only when it says not-found — a denial is AWS answering
		// about this session's access, which it can only do after finding the
		// bucket.
		if knownBuckets != nil && !knownBuckets[bucket] && bucketGone != nil && bucketGone(ctx, bucket) {
			emit(CodeCFOriginBucketMissing,
				domain.DetailRow{Label: "Origin", Value: domainName, Tier: "!"})
		}
		// An origin access identity is deprecated, not absent: a distribution
		// already restricting its bucket through one is not exposed.
		hasOAC := aws.ToString(origin.OriginAccessControlId) != ""
		hasOAI := origin.S3OriginConfig != nil && aws.ToString(origin.S3OriginConfig.OriginAccessIdentity) != ""
		if !hasOAC && !hasOAI {
			emit(CodeCFS3OriginNoOAC,
				domain.DetailRow{Label: "Origin", Value: domainName, Tier: "~"})
		}
	}

	if vc := cfg.ViewerCertificate; vc != nil {
		if word := cfTLSBelow12Word(vc.MinimumProtocolVersion); word != "" {
			emit(CodeCFDeprecatedTLS,
				domain.DetailRow{Label: "Minimum TLS version", Value: word, Tier: "~"})
		}
		// A distribution with no alias legitimately serves on its
		// cloudfront.net name with the default certificate.
		if aws.ToBool(vc.CloudFrontDefaultCertificate) && cfg.Aliases != nil && len(cfg.Aliases.Items) > 0 {
			emit(CodeCFDefaultCertificate,
				domain.DetailRow{Label: "Alias", Value: cfg.Aliases.Items[0], Tier: "~"})
		}
	}

	// CloudFront omits the logging block entirely when logging was never
	// configured, so absent is off rather than unknown.
	if cfg.Logging == nil || !aws.ToBool(cfg.Logging.Enabled) {
		emit(CodeCFLoggingOff,
			domain.DetailRow{Label: "Log bucket", Value: "none", Tier: "~"})
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
//   - DefaultCacheBehavior.ViewerProtocolPolicy == "allow-all" → "no HTTPS redirect (insecure)"
//   - Any Origin with CustomOriginConfig.OriginProtocolPolicy == "http-only" → "origin without TLS"
//
// Skip if clients.CloudFront == nil. Per-distribution errors → truncated.
func EnrichCloudFrontDistribution(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.CloudFront == nil {
		return result, nil
	}
	knownBuckets := cachedBucketNames(cache)
	bucketGone := cfBucketGoneFunc(clients)
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		distID := r.ID
		if distID == "" {
			return
		}
		out, err := clients.CloudFront.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{
			Id: aws.String(distID),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if out.DistributionConfig == nil {
			return
		}
		cfg := out.DistributionConfig
		var rows []domain.DetailRow

		// Check viewer protocol policy on default cache behavior.
		if cfg.DefaultCacheBehavior != nil &&
			cfg.DefaultCacheBehavior.ViewerProtocolPolicy == cftypes.ViewerProtocolPolicyAllowAll {
			rows = append(rows, domain.DetailRow{
				Label: "Viewer protocol policy",
				Value: "allow-all",
				Tier:  "~",
			})
		}

		// Check origin protocol policies.
		if cfg.Origins != nil {
			for _, origin := range cfg.Origins.Items {
				if origin.CustomOriginConfig != nil &&
					origin.CustomOriginConfig.OriginProtocolPolicy == cftypes.OriginProtocolPolicyHttpOnly {
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
						Label: "OriginProtocolPolicy",
						Value: "http-only",
						Tier:  "~",
					})
				}
			}
		}

		cfConfigFindings(ctx, &result, distID, cfg, knownBuckets, bucketGone)

		if len(rows) == 0 {
			return
		}
		setWave2Finding(&result, distID, cfCodeInsecureProtocol, "cf", rows)
	})
	return result, AggregateFailures("GetDistributionConfig", failures, n)
}
