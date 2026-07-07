// msk_issue_enrichment.go — Wave 2 issue enrichment for the msk resource type.
package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	kafkasvc "github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// msk canonical FindingCodes.
const (
	mskCodeBrokerOutdated   domain.FindingCode = "msk.broker-outdated"
	mskCodeEncryptionNotTLS domain.FindingCode = "msk.encryption-not-tls"
)

// EnrichMSKCluster calls DescribeClusterV2 per provisioned MSK cluster (cap EnrichmentCap)
// and raises findings for:
//   - Broker software version below 2.8 (major.minor) → "~" "broker software outdated"
//   - EncryptionInTransit.ClientBroker not "TLS" → "~" "encryption in transit not enforced"
//
// Serverless clusters (Provisioned==nil) are skipped.
// Skip if clients.MSK == nil. Per-cluster errors → Truncated.
func EnrichMSKCluster(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.MSK == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		// DescribeClusterV2 requires the cluster ARN. The msk fetcher (msk.go)
		// sets ID = cluster name and stores the ARN in Fields["cluster_arn"].
		// Passing r.ID errors with ValidationError.
		clusterARN := r.Fields["cluster_arn"]
		if clusterARN == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kafkasvc.DescribeClusterV2Output, error) {
			return clients.MSK.DescribeClusterV2(ctx, &kafkasvc.DescribeClusterV2Input{
				ClusterArn: aws.String(clusterARN),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
			return
		}
		if out.ClusterInfo == nil {
			return
		}
		prov := out.ClusterInfo.Provisioned
		if prov == nil {
			// Serverless cluster — skip checks.
			return
		}
		// Check broker software version.
		if prov.CurrentBrokerSoftwareInfo != nil && prov.CurrentBrokerSoftwareInfo.KafkaVersion != nil {
			if isMSKVersionOutdated(*prov.CurrentBrokerSoftwareInfo.KafkaVersion) {
				setWave2Finding(&result, r.ID, mskCodeBrokerOutdated, "broker software outdated", "~", "msk", nil, "")
			}
		}
		// Check encryption in transit — independently evaluated from the broker
		// version check above; setWave2Finding is append-style, so both
		// conditions surface as separate findings when both hold.
		if prov.EncryptionInfo != nil &&
			prov.EncryptionInfo.EncryptionInTransit != nil &&
			prov.EncryptionInfo.EncryptionInTransit.ClientBroker != kafkatypes.ClientBrokerTls {
			setWave2Finding(&result, r.ID, mskCodeEncryptionNotTLS, "encryption in transit not enforced", "~", "msk", nil, "")
		}
	})
	sort.Strings(failures)
	// All MSK findings are severity "~" (informational) and do not contribute to the
	// attention menu badge. IssueCount is always 0 for this enricher.
	result.IssueCount = 0
	result.Truncated = truncated
	return result,
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
