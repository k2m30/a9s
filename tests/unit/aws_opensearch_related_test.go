package unit_test

// aws_opensearch_related_test.go — related-panel checker tests for the opensearch
// resource type.
//
// Tests use the graph-root fixture (fixtures.GraphRootDomain / acme-logs) and
// per-pivot caches populated from sibling fixtures. Adversarial cases (nil
// RawStruct, ListTags error, DescribeDomainConfig error) are constructed inline.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Mocks — OpenSearch API surfaces used by related checkers
// ---------------------------------------------------------------------------

// mockOSFullAPI implements OpenSearchAPI + OpenSearchListTagsAPI +
// OpenSearchDescribeDomainConfigAPI so it can be assigned to
// ServiceClients.OpenSearch.
type mockOSFullAPI struct {
	// ListDomainNames
	listOutput *opensearch.ListDomainNamesOutput
	listErr    error
	// DescribeDomains
	describeOutput *opensearch.DescribeDomainsOutput
	describeErr    error
	// ListTags
	listTagsOutput *opensearch.ListTagsOutput
	listTagsErr    error
	// DescribeDomainConfig
	describeDomainConfigOutput *opensearch.DescribeDomainConfigOutput
	describeDomainConfigErr    error
}

func (m *mockOSFullAPI) ListDomainNames(
	_ context.Context,
	_ *opensearch.ListDomainNamesInput,
	_ ...func(*opensearch.Options),
) (*opensearch.ListDomainNamesOutput, error) {
	if m.listOutput == nil {
		return &opensearch.ListDomainNamesOutput{}, m.listErr
	}
	return m.listOutput, m.listErr
}

func (m *mockOSFullAPI) DescribeDomains(
	_ context.Context,
	_ *opensearch.DescribeDomainsInput,
	_ ...func(*opensearch.Options),
) (*opensearch.DescribeDomainsOutput, error) {
	if m.describeOutput == nil {
		return &opensearch.DescribeDomainsOutput{}, m.describeErr
	}
	return m.describeOutput, m.describeErr
}

func (m *mockOSFullAPI) ListTags(
	_ context.Context,
	_ *opensearch.ListTagsInput,
	_ ...func(*opensearch.Options),
) (*opensearch.ListTagsOutput, error) {
	if m.listTagsOutput == nil {
		return &opensearch.ListTagsOutput{}, m.listTagsErr
	}
	return m.listTagsOutput, m.listTagsErr
}

func (m *mockOSFullAPI) DescribeDomainConfig(
	_ context.Context,
	_ *opensearch.DescribeDomainConfigInput,
	_ ...func(*opensearch.Options),
) (*opensearch.DescribeDomainConfigOutput, error) {
	if m.describeDomainConfigOutput == nil {
		return &opensearch.DescribeDomainConfigOutput{}, m.describeDomainConfigErr
	}
	return m.describeDomainConfigOutput, m.describeDomainConfigErr
}

// ---------------------------------------------------------------------------
// Helper — opensearchCheckerByTarget
// ---------------------------------------------------------------------------

func opensearchCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("opensearch") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("opensearch related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("opensearch related checker for %s not found", target)
	return nil
}

// ---------------------------------------------------------------------------
// Helper — osGraphRootResource builds the graph-root resource from the fixture.
// ---------------------------------------------------------------------------

func osGraphRootResource() resource.Resource {
	fix := fixtures.NewOpenSearchFixtures()
	for _, d := range fix.Domains {
		if d.DomainName != nil && *d.DomainName == fixtures.GraphRootDomain {
			return resource.Resource{
				ID:        fixtures.GraphRootDomain,
				Name:      fixtures.GraphRootDomain,
				Status:    "",
				Fields: map[string]string{
					"domain_name": fixtures.GraphRootDomain,
					"arn":         fixtures.GraphRootDomainARN,
				},
				RawStruct: d,
			}
		}
	}
	panic("GraphRootDomain fixture not found — check fixtures.NewOpenSearchFixtures()")
}

// ---------------------------------------------------------------------------
// 1. ACM — resolves via DescribeDomainConfig.CustomEndpointCertificateArn
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_ACM(t *testing.T) {
	clients := &awsclient.ServiceClients{
		OpenSearch: &mockOSFullAPI{
			describeDomainConfigOutput: &opensearch.DescribeDomainConfigOutput{
				DomainConfig: &ostypes.DomainConfig{
					DomainEndpointOptions: &ostypes.DomainEndpointOptionsStatus{
						Options: &ostypes.DomainEndpointOptions{
							CustomEndpointEnabled:        aws.Bool(true),
							CustomEndpoint:               aws.String("acme-logs.internal.com"),
							CustomEndpointCertificateArn: aws.String(fixtures.OpenSearchACMCertARN),
						},
					},
				},
			},
		},
	}

	// The ACM fetcher indexes Resource.ID by DomainName. The checker now
	// reverse-scans the acm cache for a CertificateSummary whose ARN matches
	// and returns the target Resource.ID (DomainName) so drill-through lands.
	acmDomainName := "acme-logs.internal.com"
	acmRes := resource.Resource{
		ID:   acmDomainName,
		Name: acmDomainName,
		Fields: map[string]string{
			"domain_name": acmDomainName,
		},
		RawStruct: acmtypes.CertificateSummary{
			DomainName:     aws.String(acmDomainName),
			CertificateArn: aws.String(fixtures.OpenSearchACMCertARN),
		},
	}
	cache := resource.ResourceCache{
		"acm": resource.ResourceCacheEntry{Resources: []resource.Resource{acmRes}},
	}

	checker := opensearchCheckerByTarget(t, "acm")
	result := checker(context.Background(), clients, osGraphRootResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != acmDomainName {
		t.Errorf("ResourceIDs = %v, want [%s] (ACM fetcher indexes by DomainName, not bare cert ID)", result.ResourceIDs, acmDomainName)
	}
}

// ---------------------------------------------------------------------------
// 2. Alarm — reverse-scan by Namespace=AWS/ES + DomainName dimension
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_Alarm(t *testing.T) {
	alarmA := resource.Resource{
		ID:   "acme-logs-cluster-red",
		Name: "acme-logs-cluster-red",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("acme-logs-cluster-red"),
			Namespace: aws.String("AWS/ES"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DomainName"), Value: aws.String(fixtures.GraphRootDomain)},
			},
		},
	}
	alarmB := resource.Resource{
		ID:   "acme-logs-freestorage-low",
		Name: "acme-logs-freestorage-low",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("acme-logs-freestorage-low"),
			Namespace: aws.String("AWS/ES"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DomainName"), Value: aws.String(fixtures.GraphRootDomain)},
			},
		},
	}
	unrelated := resource.Resource{
		ID:   "unrelated-alarm",
		Name: "unrelated-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:  aws.String("unrelated-alarm"),
			Dimensions: []cwtypes.Dimension{},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{alarmA, alarmB, unrelated},
		},
	}

	checker := opensearchCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, osGraphRootResource(), cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (two alarms match DomainName=%s)", result.Count, fixtures.GraphRootDomain)
	}
}

// ---------------------------------------------------------------------------
// 3. CFN — resolves via ListTags returning aws:cloudformation:stack-name
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_CFN(t *testing.T) {
	clients := &awsclient.ServiceClients{
		OpenSearch: &mockOSFullAPI{
			listTagsOutput: &opensearch.ListTagsOutput{
				TagList: []ostypes.Tag{
					{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String(fixtures.OpenSearchCFNStackName)},
					{Key: aws.String("Environment"), Value: aws.String("production")},
				},
			},
		},
	}

	cfnRes := resource.Resource{
		ID:   fixtures.OpenSearchCFNStackName,
		Name: fixtures.OpenSearchCFNStackName,
		Fields: map[string]string{
			"stack_name": fixtures.OpenSearchCFNStackName,
		},
		RawStruct: cfntypes.Stack{
			StackName: aws.String(fixtures.OpenSearchCFNStackName),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}

	checker := opensearchCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, osGraphRootResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != fixtures.OpenSearchCFNStackName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, fixtures.OpenSearchCFNStackName)
	}
}

// ---------------------------------------------------------------------------
// 4. KMS — resolves via EncryptionAtRestOptions.KmsKeyId (bare key ID)
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_KMS(t *testing.T) {
	kmsRes := resource.Resource{
		ID:   fixtures.OpenSearchKMSKeyID,
		Name: fixtures.OpenSearchKMSKeyID,
		Fields: map[string]string{
			"key_id": fixtures.OpenSearchKMSKeyID,
			"arn":    fixtures.OpenSearchKMSKeyARN,
		},
	}
	cache := resource.ResourceCache{}

	checker := opensearchCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, osGraphRootResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != fixtures.OpenSearchKMSKeyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, fixtures.OpenSearchKMSKeyID)
	}
	_ = kmsRes // kms checker uses forward-lookup from DomainStatus, not cache scan
}

// ---------------------------------------------------------------------------
// 5. Logs — resolves via LogPublishingOptions (3 groups from graph-root fixture)
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_Logs(t *testing.T) {
	logA := resource.Resource{ID: fixtures.OpenSearchLogGroupSearchSlow, Name: fixtures.OpenSearchLogGroupSearchSlow, Fields: map[string]string{}}
	logB := resource.Resource{ID: fixtures.OpenSearchLogGroupIndexSlow, Name: fixtures.OpenSearchLogGroupIndexSlow, Fields: map[string]string{}}
	logC := resource.Resource{ID: fixtures.OpenSearchLogGroupAudit, Name: fixtures.OpenSearchLogGroupAudit, Fields: map[string]string{}}
	unrelated := resource.Resource{ID: "/aws/other/logs", Name: "/aws/other/logs", Fields: map[string]string{}}

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{logA, logB, logC, unrelated},
		},
	}

	checker := opensearchCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, osGraphRootResource(), cache)

	if result.Count < 3 {
		t.Errorf("Count = %d, want >= 3 (three log groups in graph-root fixture)", result.Count)
	}

	wantIDs := map[string]bool{
		fixtures.OpenSearchLogGroupSearchSlow: false,
		fixtures.OpenSearchLogGroupIndexSlow:  false,
		fixtures.OpenSearchLogGroupAudit:      false,
	}
	for _, id := range result.ResourceIDs {
		if _, expected := wantIDs[id]; expected {
			wantIDs[id] = true
		}
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("ResourceIDs = %v — missing expected log group ID %q", result.ResourceIDs, id)
		}
	}
}

// ---------------------------------------------------------------------------
// 6. SG — resolves via VPCOptions.SecurityGroupIds
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_SG(t *testing.T) {
	// SG checker uses forward-lookup from DomainStatus, not cache scan, so
	// the cache is intentionally empty here.
	cache := resource.ResourceCache{}

	checker := opensearchCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, osGraphRootResource(), cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (two SGs in graph-root VPCOptions)", result.Count)
	}

	ids := make(map[string]bool)
	for _, id := range result.ResourceIDs {
		ids[id] = true
	}
	if !ids[fixtures.OpenSearchSGA] {
		t.Errorf("ResourceIDs = %v, missing %s", result.ResourceIDs, fixtures.OpenSearchSGA)
	}
	if !ids[fixtures.OpenSearchSGB] {
		t.Errorf("ResourceIDs = %v, missing %s", result.ResourceIDs, fixtures.OpenSearchSGB)
	}
}

// ---------------------------------------------------------------------------
// 7. Subnet — resolves via VPCOptions.SubnetIds
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_Subnet(t *testing.T) {
	subnetA := resource.Resource{ID: fixtures.OpenSearchSubnetA, Name: fixtures.OpenSearchSubnetA, Fields: map[string]string{}}
	subnetB := resource.Resource{ID: fixtures.OpenSearchSubnetB, Name: fixtures.OpenSearchSubnetB, Fields: map[string]string{}}
	cache := resource.ResourceCache{}

	checker := opensearchCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, osGraphRootResource(), cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (two subnets in graph-root VPCOptions)", result.Count)
	}

	ids := make(map[string]bool)
	for _, id := range result.ResourceIDs {
		ids[id] = true
	}
	if !ids[fixtures.OpenSearchSubnetA] {
		t.Errorf("ResourceIDs = %v, missing %s", result.ResourceIDs, fixtures.OpenSearchSubnetA)
	}
	if !ids[fixtures.OpenSearchSubnetB] {
		t.Errorf("ResourceIDs = %v, missing %s", result.ResourceIDs, fixtures.OpenSearchSubnetB)
	}
	_, _ = subnetA, subnetB // subnet checker uses forward-lookup from DomainStatus, not cache scan
}

// ---------------------------------------------------------------------------
// 8. VPC — resolves via VPCOptions.VPCId
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_VPC(t *testing.T) {
	vpcRes := resource.Resource{ID: fixtures.OpenSearchVPCID, Name: fixtures.OpenSearchVPCID, Fields: map[string]string{}}
	cache := resource.ResourceCache{}

	checker := opensearchCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, osGraphRootResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != fixtures.OpenSearchVPCID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, fixtures.OpenSearchVPCID)
	}
	_ = vpcRes // VPC checker uses forward-lookup from DomainStatus, not cache scan
}

// ---------------------------------------------------------------------------
// 9. Public domain returns Count=0 for VPC pivots (not -1)
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_PublicDomain_VPCPivotsZero(t *testing.T) {
	// Build a public domain resource (no VPCOptions).
	fix := fixtures.NewOpenSearchFixtures()
	var healthyBaseline ostypes.DomainStatus
	for _, d := range fix.Domains {
		if d.DomainName != nil && *d.DomainName == fixtures.HealthyBaselineDomain {
			healthyBaseline = d
			break
		}
	}
	if healthyBaseline.DomainName == nil {
		t.Fatalf("HealthyBaselineDomain fixture not found")
	}
	// Confirm it has no VPCOptions (public endpoint).
	if healthyBaseline.VPCOptions != nil {
		t.Skipf("HealthyBaselineDomain has VPCOptions — test precondition not met")
	}

	publicRes := resource.Resource{
		ID:        fixtures.HealthyBaselineDomain,
		Name:      fixtures.HealthyBaselineDomain,
		Fields:    map[string]string{},
		RawStruct: healthyBaseline,
	}
	cache := resource.ResourceCache{}

	for _, target := range []string{"vpc", "sg", "subnet"} {
		checker := opensearchCheckerByTarget(t, target)
		result := checker(context.Background(), nil, publicRes, cache)
		if result.Count != 0 {
			t.Errorf("%s: Count = %d, want 0 for public domain (no VPCOptions)", target, result.Count)
		}
		if result.Count == -1 {
			t.Errorf("%s: Count = -1, want 0 — public domain has no VPC, not unknown", target)
		}
	}
}

// ---------------------------------------------------------------------------
// 10. ct-events — verify a checker is registered on opensearch
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_CtEvents_Registered(t *testing.T) {
	defs := resource.GetRelated("opensearch")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for opensearch")
	}
	for _, def := range defs {
		if def.TargetType == "ct-events" {
			if def.Checker == nil {
				t.Error("ct-events checker is nil for opensearch")
			}
			return
		}
	}
	t.Error("ct-events not registered for opensearch (universal pivot must be present)")
}

// ---------------------------------------------------------------------------
// Adversarial 1 — nil RawStruct → all field-based checkers return Count=-1
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_Adversarial_NilRawStruct(t *testing.T) {
	nilRes := resource.Resource{
		ID:        "nil-raw-domain",
		Name:      "nil-raw-domain",
		Fields:    map[string]string{"arn": "arn:aws:es:us-east-1:123456789012:domain/nil-raw-domain"},
		RawStruct: nil,
	}
	cache := resource.ResourceCache{}

	// All pattern-F checkers (sg, subnet, vpc, kms, logs) must return Count=-1
	// when RawStruct cannot be asserted to DomainStatus.
	for _, target := range []string{"sg", "subnet", "vpc", "logs"} {
		checker := opensearchCheckerByTarget(t, target)
		result := checker(context.Background(), nil, nilRes, cache)
		if result.Count != -1 {
			t.Errorf("%s: Count = %d, want -1 (nil RawStruct → unknown)", target, result.Count)
		}
	}
}

// ---------------------------------------------------------------------------
// Adversarial 2 — ListTags returns error → cfn checker returns Count=-1
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_Adversarial_ListTagsError(t *testing.T) {
	clients := &awsclient.ServiceClients{
		OpenSearch: &mockOSFullAPI{
			listTagsErr: errors.New("simulated ListTags API error"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
	}

	checker := opensearchCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, osGraphRootResource(), cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (ListTags error → unknown)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// Adversarial 3 — DescribeDomainConfig returns error → acm checker returns Count=-1
// ---------------------------------------------------------------------------

func TestRelated_OpenSearch_Adversarial_DescribeDomainConfigError(t *testing.T) {
	clients := &awsclient.ServiceClients{
		OpenSearch: &mockOSFullAPI{
			describeDomainConfigErr: errors.New("simulated DescribeDomainConfig API error"),
		},
	}
	cache := resource.ResourceCache{}

	checker := opensearchCheckerByTarget(t, "acm")
	result := checker(context.Background(), clients, osGraphRootResource(), cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (DescribeDomainConfig error → unknown)", result.Count)
	}
}
