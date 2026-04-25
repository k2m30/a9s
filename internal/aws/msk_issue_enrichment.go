// msk_issue_enrichment.go — Wave 2 issue enrichment for the msk resource type.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	kafkasvc "github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("msk", EnrichMSKCluster, 100)
}

// EnrichMSKCluster calls DescribeClusterV2 per provisioned MSK cluster (cap EnrichmentCap)
// and raises findings for:
//   - Broker software version below 2.8 (major.minor) → "~" "broker software outdated"
//   - EncryptionInTransit.ClientBroker not "TLS" → "~" "encryption in transit not enforced"
//
// Serverless clusters (Provisioned==nil) are skipped.
// Skip if clients.MSK == nil. Per-cluster errors → Truncated.
func EnrichMSKCluster(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.MSK == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		// DescribeClusterV2 requires the cluster ARN. The msk fetcher (msk.go)
		// sets ID = cluster name and stores the ARN in Fields["cluster_arn"].
		// Passing r.ID errors with ValidationError.
		clusterARN := r.Fields["cluster_arn"]
		if clusterARN == "" {
			continue
		}
		total++
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kafkasvc.DescribeClusterV2Output, error) {
			return clients.MSK.DescribeClusterV2(ctx, &kafkasvc.DescribeClusterV2Input{
				ClusterArn: aws.String(clusterARN),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		if out.ClusterInfo == nil {
			continue
		}
		prov := out.ClusterInfo.Provisioned
		if prov == nil {
			// Serverless cluster — skip checks.
			continue
		}
		// Check broker software version.
		if prov.CurrentBrokerSoftwareInfo != nil && prov.CurrentBrokerSoftwareInfo.KafkaVersion != nil {
			if isMSKVersionOutdated(*prov.CurrentBrokerSoftwareInfo.KafkaVersion) {
				findings[r.ID] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  "broker software outdated",
				}
			}
		}
		// Check encryption in transit (only set finding if not already set).
		if _, alreadyFound := findings[r.ID]; !alreadyFound {
			if prov.EncryptionInfo != nil &&
				prov.EncryptionInfo.EncryptionInTransit != nil &&
				prov.EncryptionInfo.EncryptionInTransit.ClientBroker != kafkatypes.ClientBrokerTls {
				findings[r.ID] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  "encryption in transit not enforced",
				}
			}
		}
	}
	// All MSK findings are severity "~" (informational) and do not contribute to the
	// attention menu badge. IssueCount is always 0 for this enricher.
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings},
		AggregateFailures("msk-enrich: DescribeClusterV2", failures, total)
}

// isMSKVersionOutdated returns true when the given Kafka version string is below the
// conservative current cutoff of 2.8 (major.minor). Versions that cannot be parsed
// are treated as up-to-date (safe default — do not produce false-positive findings).
func isMSKVersionOutdated(version string) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	major, err := parseVersionPart(parts[0])
	if err != nil {
		return false
	}
	minor, err := parseVersionPart(parts[1])
	if err != nil {
		return false
	}
	// Current cutoff: 2.8. Anything with major < 2 or (major == 2 && minor < 8) is outdated.
	return major < 2 || (major == 2 && minor < 8)
}

// parseVersionPart parses a numeric version component, returning an error for non-numeric input.
func parseVersionPart(s string) (int, error) {
	val := 0
	if len(s) == 0 {
		return 0, fmt.Errorf("empty version part")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-numeric version part: %q", s)
		}
		val = val*10 + int(c-'0')
	}
	return val, nil
}
