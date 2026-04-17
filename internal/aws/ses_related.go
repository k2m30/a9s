// ses_related.go contains SES related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkSESR53 searches the R53 cache for hosted zones whose domain matches the
// SES identity domain. Pattern N — naming convention.
//
// EMAIL_ADDRESS identities: extract domain after "@".
// DOMAIN identities: use the identity name directly.
// Hosted zone names have a trailing dot (e.g. "acme-corp.com.") which is stripped
// before comparison.
func checkSESR53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domain := sesIdentityDomain(res)
	if domain == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}

	r53List, truncated, err := sesRelatedResources(ctx, clients, cache, "r53")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1, Err: err}
	}
	if r53List == nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
	}

	var ids []string
	for _, zone := range r53List {
		zoneName := strings.TrimSuffix(zone.Name, ".")
		if strings.EqualFold(zoneName, domain) || strings.HasSuffix(domain, "."+zoneName) {
			ids = append(ids, zone.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
	}
	return relatedResult("r53", ids)
}

// sesIdentityDomain extracts the domain from a SES identity resource.
// For EMAIL_ADDRESS identities (containing "@"), it returns the part after "@".
// For DOMAIN identities, it returns the identity name directly.
func sesIdentityDomain(res resource.Resource) string {
	name := res.ID
	if name == "" {
		return ""
	}
	// EMAIL_ADDRESS: extract domain after @
	if idx := strings.LastIndex(name, "@"); idx >= 0 {
		return name[idx+1:]
	}
	// DOMAIN: use as-is
	return name
}




// SES identity list RawStruct (sesv2types.IdentityInfo) exposes only IdentityName,
// IdentityType, SendingEnabled, VerificationStatus. Relationships to ACM, Kinesis,
// KMS, Lambda, Logs, S3, SNS, EventBridge, IAM roles, and CloudWatch alarms require
// per-identity or per-configuration-set calls (GetEmailIdentity,
// GetConfigurationSetEventDestinations, DescribeReceiptRule) — N+1 at panel render.
// All such checkers return Count: -1.

// sesRelatedResources returns the resource list for target from cache or by fetching the first page.
func sesRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}









