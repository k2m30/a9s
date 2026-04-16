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

// checkSESCFN returns Count: 0 because SES identity tags are not included in
// the ListEmailIdentities response — the CFN relationship cannot be determined
// from cache alone.
func checkSESCFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

// checkSESRole returns Count: 0 because SES identities do not expose an IAM role
// ARN in the ListEmailIdentities response.
func checkSESRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}

// checkSESKMS is a stub. The SES v2 ListEmailIdentities API does not include
// DKIM signing key or configuration-set KMS details in the list response.
func checkSESKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
}

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

func checkSESACM(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
}

func checkSESAlarm(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
}

func checkSESEbRule(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
}

func checkSESKinesis(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kinesis", Count: 0}
}

func checkSESLambda(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
}

func checkSESLogs(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
}

func checkSESS3(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
}

func checkSESSNS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
}

func checkSESTrail(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "trail", Count: 0}
}
