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
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// mskDescribeClusterV2Fake implements MSKAPI for enrichment testing.
// It embeds the interface and overrides only DescribeClusterV2.
// The results map is keyed by ClusterArn (from the input) so the fake can
// serve different responses per resource.
type mskDescribeClusterV2Fake struct {
	awsclient.MSKAPI
	// results maps ClusterArn → cluster. If absent the fake returns errByArn.
	results map[string]*kafkatypes.Cluster
	// errByArn maps ClusterArn → error; overrides results when set.
	errByArn map[string]error
}

func (f *mskDescribeClusterV2Fake) DescribeClusterV2(
	_ context.Context,
	in *kafka.DescribeClusterV2Input,
	_ ...func(*kafka.Options),
) (*kafka.DescribeClusterV2Output, error) {
	arn := ""
	if in != nil && in.ClusterArn != nil {
		arn = *in.ClusterArn
	}
	if f.errByArn != nil {
		if err, ok := f.errByArn[arn]; ok {
			return nil, err
		}
	}
	clusterInfo, ok := f.results[arn]
	if !ok {
		return &kafka.DescribeClusterV2Output{}, nil
	}
	return &kafka.DescribeClusterV2Output{ClusterInfo: clusterInfo}, nil
}

// Compile-time check: mskDescribeClusterV2Fake satisfies MSKAPI.
var _ awsclient.MSKAPI = (*mskDescribeClusterV2Fake)(nil)

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
	fake := &mskDescribeClusterV2Fake{
		results: map[string]*kafkatypes.Cluster{
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
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichMSKCluster_OutdatedVersionProducesFindingSevTilde verifies that when
// cluster-1 uses KafkaVersion=2.6.0 (below 3.0), a finding with severity "~" is
// produced for cluster-1, and cluster-2 (modern version) produces no finding.
func TestEnrichMSKCluster_OutdatedVersionProducesFindingSevTilde(t *testing.T) {
	fake := &mskDescribeClusterV2Fake{
		results: map[string]*kafkatypes.Cluster{
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
	f, ok := result.Findings[mskName1]
	if !ok {
		t.Fatalf("expected finding keyed by bare cluster name %q", mskName1)
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings[mskName2]; ok {
		t.Error("cluster-2 must NOT appear in Findings — it uses modern Kafka version")
	}
	// "~" findings do NOT contribute to IssueCount per the EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichMSKCluster_PlaintextEncryptionProducesFindingSevTilde verifies that when
// cluster-1 uses EncryptionInTransit=PLAINTEXT, a finding with severity "~" is
// produced for cluster-1, and cluster-2 (TLS) produces no finding.
func TestEnrichMSKCluster_PlaintextEncryptionProducesFindingSevTilde(t *testing.T) {
	fake := &mskDescribeClusterV2Fake{
		results: map[string]*kafkatypes.Cluster{
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
	f, ok := result.Findings[mskName1]
	if !ok {
		t.Fatalf("expected finding keyed by bare cluster name %q", mskName1)
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings[mskName2]; ok {
		t.Error("cluster-2 must NOT appear in Findings — it uses TLS")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichMSKCluster_ServerlessClusterSkipped verifies that when cluster-1 is
// serverless (Provisioned==nil, Serverless!=nil), it is skipped and produces no finding.
func TestEnrichMSKCluster_ServerlessClusterSkipped(t *testing.T) {
	fake := &mskDescribeClusterV2Fake{
		results: map[string]*kafkatypes.Cluster{
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
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
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

// TestEnrichMSKCluster_APIErrorSetsTruncatedAndSurfacesError verifies that when the
// API call for cluster-1 returns an error, the enricher sets Truncated=true, produces
// 0 findings for that cluster, and returns a composite error containing the enricher
// prefix and the failing cluster ARN.
func TestEnrichMSKCluster_APIErrorSetsTruncatedAndSurfacesError(t *testing.T) {
	apiErr := errors.New("kafka: DescribeClusterV2 throttled")
	fake := &mskDescribeClusterV2Fake{
		errByArn: map[string]error{
			mskARN1: apiErr,
		},
		results: map[string]*kafkatypes.Cluster{
			mskARN2: provisionedCluster(mskARN2, "3.5.1", kafkatypes.ClientBrokerTls),
		},
	}
	clients := &awsclient.ServiceClients{MSK: fake}
	resources := mskClusterResources(mskARN1, mskARN2)

	result, err := awsclient.EnrichMSKCluster(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when an API call fails")
	}
	if errStr := err.Error(); !strings.Contains(errStr, "msk-enrich:") {
		t.Errorf("composite error must contain \"msk-enrich:\", got: %q", errStr)
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
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
}
