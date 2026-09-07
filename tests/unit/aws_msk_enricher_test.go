package unit

// aws_msk_enricher_test.go — Behavioral tests for EnrichMSKCluster.
//
// Contract assertions:
//   - DescribeClusterV2 is called once per MSK resource (keyed by cluster name).
//   - KafkaVersion >= 3.0 AND EncryptionInTransit=TLS → 0 findings.
//   - KafkaVersion < 3.0 → 1 finding sev "~" for that cluster.
//   - EncryptionInTransit=PLAINTEXT → 1 finding sev "~" for that cluster.
//   - Serverless cluster (Provisioned==nil, Serverless!=nil) → skipped, 0 findings.
//   - clients.MSK == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for a resource → 0 findings for that resource, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// The fake client for DescribeClusterV2 now lives in fakes_msk_test.go
// (fakeMSKDescribeClusterV2) — see that file's header for the one-fake-per-
// interface convention.

// mskClusterResources returns a slice of MSK Resource stubs with the given ARNs.
// Mirrors the fetcher contract: ID = bare cluster name, Fields["cluster_arn"] = full ARN.
func mskClusterResources(arns ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(arns))
	for _, arn := range arns {
		name := "msk-" + arn[len(arn)-8:]
		res = append(res, resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"cluster_arn":  arn,
				"cluster_name": name,
				"cluster_type": "PROVISIONED",
				"state":        "ACTIVE",
				"version":      "3.5.1",
			},
		})
	}
	return res
}

// provisionedCluster builds a Cluster with Provisioned set (TLS + given KafkaVersion).
func provisionedCluster(arn, kafkaVersion string, clientBroker kafkatypes.ClientBroker) *kafkatypes.Cluster {
	return &kafkatypes.Cluster{
		ClusterArn:  aws.String(arn),
		ClusterName: aws.String("msk-" + arn[len(arn)-8:]),
		Provisioned: &kafkatypes.Provisioned{
			CurrentBrokerSoftwareInfo: &kafkatypes.BrokerSoftwareInfo{
				KafkaVersion: aws.String(kafkaVersion),
			},
			EncryptionInfo: &kafkatypes.EncryptionInfo{
				EncryptionInTransit: &kafkatypes.EncryptionInTransit{
					ClientBroker: clientBroker,
				},
			},
		},
	}
}

// serverlessCluster builds a Cluster with Serverless set and Provisioned nil.
func serverlessCluster(arn string) *kafkatypes.Cluster {
	return &kafkatypes.Cluster{
		ClusterArn:  aws.String(arn),
		ClusterName: aws.String("msk-serverless"),
		Serverless:  &kafkatypes.Serverless{},
	}
}

const (
	mskARN1 = "arn:aws:kafka:us-east-1:123456789012:cluster/msk-cluster-1/aaaaaaaa"
	mskARN2 = "arn:aws:kafka:us-east-1:123456789012:cluster/msk-cluster-2/bbbbbbbb"

	// mskName* are the bare cluster names derived from the ARN suffix by mskClusterResources.
	// They mirror what the fetcher sets as r.ID.
	// Suffix formula: arn[len(arn)-8:] → "aaaaaaaa", "bbbbbbbb".
	mskName1 = "msk-aaaaaaaa"
	mskName2 = "msk-bbbbbbbb"
)

// TestEnrichMSKCluster_ModernTLSProducesNoFindings verifies that when both clusters
// use KafkaVersion >= 3.0 and EncryptionInTransit=TLS, no findings are produced.
func TestEnrichMSKCluster_ModernTLSProducesNoFindings(t *testing.T) {
	fake := &fakeMSKDescribeClusterV2{
		Results: map[string]*kafkatypes.Cluster{
			mskARN1: provisionedCluster(mskARN1, "3.5.1", kafkatypes.ClientBrokerTls),
			mskARN2: provisionedCluster(mskARN2, "3.5.1", kafkatypes.ClientBrokerTls),
		},
	}
	clients := &awsclient.ServiceClients{MSK: fake}
	resources := mskClusterResources(mskARN1, mskARN2)

	result, err := awsclient.EnrichMSKCluster(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

// TestEnrichMSKCluster_OutdatedVersionProducesFindingSevTilde verifies that when
// cluster-1 uses KafkaVersion=2.6.0 (below 3.0), a finding with severity "~" is
// produced for cluster-1, and cluster-2 (modern version) produces no finding.
func TestEnrichMSKCluster_OutdatedVersionProducesFindingSevTilde(t *testing.T) {
	fake := &fakeMSKDescribeClusterV2{
		Results: map[string]*kafkatypes.Cluster{
			mskARN1: provisionedCluster(mskARN1, "2.6.0", kafkatypes.ClientBrokerTls),
			mskARN2: provisionedCluster(mskARN2, "3.5.1", kafkatypes.ClientBrokerTls),
		},
	}
	clients := &awsclient.ServiceClients{MSK: fake}
	resources := mskClusterResources(mskARN1, mskARN2)

	result, err := awsclient.EnrichMSKCluster(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[mskName1]
	if !ok {
		t.Fatalf("expected finding keyed by bare cluster name %q", mskName1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings[mskName2]; ok {
		t.Error("cluster-2 must NOT appear in Findings — it uses modern Kafka version")
	}
}

// TestEnrichMSKCluster_PlaintextEncryptionProducesFindingSevTilde verifies that when
// cluster-1 uses EncryptionInTransit=PLAINTEXT, a finding with severity "~" is
// produced for cluster-1, and cluster-2 (TLS) produces no finding.
func TestEnrichMSKCluster_PlaintextEncryptionProducesFindingSevTilde(t *testing.T) {
	fake := &fakeMSKDescribeClusterV2{
		Results: map[string]*kafkatypes.Cluster{
			mskARN1: provisionedCluster(mskARN1, "3.5.1", kafkatypes.ClientBrokerPlaintext),
			mskARN2: provisionedCluster(mskARN2, "3.5.1", kafkatypes.ClientBrokerTls),
		},
	}
	clients := &awsclient.ServiceClients{MSK: fake}
	resources := mskClusterResources(mskARN1, mskARN2)

	result, err := awsclient.EnrichMSKCluster(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[mskName1]
	if !ok {
		t.Fatalf("expected finding keyed by bare cluster name %q", mskName1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings[mskName2]; ok {
		t.Error("cluster-2 must NOT appear in Findings — it uses TLS")
	}
}

// TestEnrichMSKCluster_ServerlessClusterSkipped verifies that when cluster-1 is
// serverless (Provisioned==nil, Serverless!=nil), it is skipped and produces no finding.
func TestEnrichMSKCluster_ServerlessClusterSkipped(t *testing.T) {
	fake := &fakeMSKDescribeClusterV2{
		Results: map[string]*kafkatypes.Cluster{
			mskARN1: serverlessCluster(mskARN1),
			mskARN2: provisionedCluster(mskARN2, "3.5.1", kafkatypes.ClientBrokerTls),
		},
	}
	clients := &awsclient.ServiceClients{MSK: fake}
	resources := mskClusterResources(mskARN1, mskARN2)

	result, err := awsclient.EnrichMSKCluster(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for serverless skip, got %d", len(result.Findings))
	}
}

// TestEnrichMSKCluster_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.MSK is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichMSKCluster_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{MSK: nil}

	result, err := awsclient.EnrichMSKCluster(context.Background(), clients, mskClusterResources(mskARN1, mskARN2), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when MSK client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichMSKCluster_OutdatedVersionAndPlaintextEncryption_ProducesBothFindings
// is a RED regression pin for a P2 bug found by Codex in the v3.47.0 #52 landing:
// EnrichMSKCluster's encryption-in-transit check is gated by
// `if _, alreadyFound := result.Findings[r.ID]; !alreadyFound` (msk_issue_enrichment.go),
// a pre-#52 "only one finding per resource" short-circuit that setWave2Finding's
// append-style contract (#52) obsoletes. When cluster-1 is BOTH broker-outdated
// (KafkaVersion 2.6.0) AND not using TLS (ClientBroker=PLAINTEXT), the broker check
// runs first and its setWave2Finding call populates result.Findings[r.ID], so the
// encryption check's alreadyFound guard trips and the second, independently-evaluated
// condition is silently dropped — the cluster shows only "broker software outdated",
// never "encryption in transit not enforced", even though both are true.
func TestEnrichMSKCluster_OutdatedVersionAndPlaintextEncryption_ProducesBothFindings(t *testing.T) {
	fake := &fakeMSKDescribeClusterV2{
		Results: map[string]*kafkatypes.Cluster{
			mskARN1: provisionedCluster(mskARN1, "2.6.0", kafkatypes.ClientBrokerPlaintext),
		},
	}
	clients := &awsclient.ServiceClients{MSK: fake}
	resources := mskClusterResources(mskARN1)

	result, err := awsclient.EnrichMSKCluster(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[mskName1]
	if !ok {
		t.Fatalf("expected findings keyed by bare cluster name %q, got Findings=%+v", mskName1, result.Findings)
	}

	var haveBrokerOutdated, haveEncryptionNotTLS bool
	for _, f := range fs {
		switch f.Code {
		case domain.FindingCode("msk.broker-outdated"):
			haveBrokerOutdated = true
		case domain.FindingCode("msk.encryption-not-tls"):
			haveEncryptionNotTLS = true
		}
	}
	if !haveBrokerOutdated {
		t.Errorf("result.Findings[%q] missing Code \"msk.broker-outdated\" — test fixture assumption broken; got %+v", mskName1, fs)
	}
	if !haveEncryptionNotTLS {
		t.Errorf(`BUG: result.Findings[%q] missing Code "msk.encryption-not-tls" (%d entries, want 2) — EnrichMSKCluster's encryption-in-transit check is gated behind "if _, alreadyFound := result.Findings[r.ID]; !alreadyFound" (core/aws/msk_issue_enrichment.go), so once the broker-outdated check appends its finding first, the encryption check short-circuits and never runs, even though setWave2Finding is append-style (#52) and both conditions independently hold. Got %+v`, mskName1, len(fs), fs)
	}
}

// TestEnrichMSKCluster_APIErrorMarksRowTruncatedIDNotBadge verifies that when the
// API call for cluster-1 returns an error, the enricher marks that
// cluster's row via TruncatedIDs, produces 0 findings for that cluster, and
// returns a composite error containing the enricher prefix and the failing
// cluster ARN. msk only ever emits "~" (informational) findings, so the
// aggregate Truncated flag must stay false — a coverage gap never
// lower-bounds the issue badge.
func TestEnrichMSKCluster_APIErrorMarksRowTruncatedIDNotBadge(t *testing.T) {
	apiErr := errors.New("kafka: DescribeClusterV2 throttled")
	fake := &fakeMSKDescribeClusterV2{
		ErrByArn: map[string]error{
			mskARN1: apiErr,
		},
		Results: map[string]*kafkatypes.Cluster{
			mskARN2: provisionedCluster(mskARN2, "3.5.1", kafkatypes.ClientBrokerTls),
		},
	}
	clients := &awsclient.ServiceClients{MSK: fake}
	resources := mskClusterResources(mskARN1, mskARN2)

	result, err := awsclient.EnrichMSKCluster(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when an API call fails")
	}
	// INVERTED for the "skipped" spec row 6: the label was "msk-enrich:", which
	// made the rendered line say the type twice ("enrich msk: DescribeClusterV2 ..."). The
	// type comes from the registry key at the surface; the aggregate names
	// the call. Do not restore the type in the label.
	if errStr := err.Error(); !strings.Contains(errStr, "DescribeClusterV2") {
		t.Errorf("composite error must name the call, %q, got: %q", "DescribeClusterV2", errStr)
	}
	// Composite error reports the failing resource's ID (bare cluster name
	// set by the msk fetcher), not the ARN, because that's what operators
	// see in the Status column and map back to.
	if errStr := err.Error(); !strings.Contains(errStr, mskName1) {
		t.Errorf("composite error must contain the failing cluster ID %q, got: %q", mskName1, errStr)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if result.Truncated {
		t.Error("Truncated must stay false: msk only emits \"~\" findings, so an API error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if !result.TruncatedIDs[mskName1] {
		t.Errorf("TruncatedIDs[%q] must be true — the DescribeClusterV2 error must mark that cluster's row with a \"?\" coverage gap", mskName1)
	}
}
