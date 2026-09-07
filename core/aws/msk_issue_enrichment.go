// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// msk_issue_enrichment.go — Wave 2 issue enrichment for the msk resource type.
package aws

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	kafkasvc "github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// msk canonical FindingCodes.
const (
	mskCodeBrokerOutdated   domain.FindingCode = "msk.broker-outdated"
	mskCodeEncryptionNotTLS domain.FindingCode = "msk.encryption-not-tls"
	mskCodePublicAccess     domain.FindingCode = "msk.public-access"
	mskCodeUnauthenticated  domain.FindingCode = "msk.unauthenticated"
)

// S5 operator sentences for the broker exposure codes above. Neither carries
// a supporting row: the phrase is the whole fact.
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
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
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
			MarkSkipped(&result, r.ID, &failures, err)
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
				setWave2Finding(&result, r.ID, mskCodeBrokerOutdated, "broker software outdated", "~", "msk", nil)
			}
		}
		// Check encryption in transit — independently evaluated from the broker
		// version check above; setWave2Finding is append-style, so both
		// conditions surface as separate findings when both hold.
		if prov.EncryptionInfo != nil &&
			prov.EncryptionInfo.EncryptionInTransit != nil &&
			prov.EncryptionInfo.EncryptionInTransit.ClientBroker != kafkatypes.ClientBrokerTls {
			setWave2Finding(&result, r.ID, mskCodeEncryptionNotTLS, "encryption in transit not enforced", "~", "msk", nil)
		}
		// Rule 4: a cluster being torn down, or already broken beyond use,
		// has no posture worth reporting. The two checks above describe the
		// software it is running; the two below describe how it is reachable,
		// which is what stops mattering when it is going away.
		if mskLifecycleEnded(r.Fields["state"]) {
			return
		}
		// A nil anywhere down either chain is unknown, not misconfigured.
		if bng := prov.BrokerNodeGroupInfo; bng != nil &&
			bng.ConnectivityInfo != nil &&
			bng.ConnectivityInfo.PublicAccess != nil &&
			aws.ToString(bng.ConnectivityInfo.PublicAccess.Type) == mskPublicAccessOn {
			setWave2Finding(&result, r.ID, mskCodePublicAccess, "brokers reachable from the internet", "!", "msk", nil)
		}
		if ca := prov.ClientAuthentication; ca != nil &&
			ca.Unauthenticated != nil &&
			aws.ToBool(ca.Unauthenticated.Enabled) {
			setWave2Finding(&result, r.ID, mskCodeUnauthenticated, "unauthenticated access allowed", "!", "msk", nil)
		}
	})

	return result,
		AggregateFailures("msk-enrich: DescribeClusterV2", failures, total)
}

// mskPublicAccessOn is the one PublicAccess.Type value that means the brokers
// carry their own public addresses; every other value, including DISABLED,
// keeps them inside the VPC.
const mskPublicAccessOn = "SERVICE_PROVIDED_EIPS"

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

// mskLifecycleEnded reports whether a cluster is on its way out or has
// failed. Fields["state"] carries the raw ListClustersV2 value.
func mskLifecycleEnded(state string) bool {
	switch kafkatypes.ClusterState(state) {
	case kafkatypes.ClusterStateDeleting, kafkatypes.ClusterStateFailed:
		return true
	default:
		return false
	}
}
