package unit

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// osBatchLimitFake answers DescribeDomains the way OpenSearch does: a request
// naming more than five domains is rejected with a ValidationException, and a
// request naming failName is denied.
type osBatchLimitFake struct {
	domains  map[string]ostypes.DomainStatus
	failName string
	calls    [][]string
}

func (f *osBatchLimitFake) ListDomainNames(_ context.Context, _ *opensearch.ListDomainNamesInput, _ ...func(*opensearch.Options)) (*opensearch.ListDomainNamesOutput, error) {
	out := &opensearch.ListDomainNamesOutput{}
	for _, name := range slices.Sorted(maps.Keys(f.domains)) {
		out.DomainNames = append(out.DomainNames, ostypes.DomainInfo{DomainName: aws.String(name), EngineType: ostypes.EngineTypeOpenSearch})
	}
	return out, nil
}

func (f *osBatchLimitFake) DescribeDomains(_ context.Context, in *opensearch.DescribeDomainsInput, _ ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error) {
	f.calls = append(f.calls, slices.Clone(in.DomainNames))
	if len(in.DomainNames) > 5 {
		return nil, &smithy.GenericAPIError{Code: "ValidationException", Message: "1 validation error detected: Value at 'domainNames' failed to satisfy constraint: Member must have length less than or equal to 5"}
	}
	if slices.Contains(in.DomainNames, f.failName) {
		return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform es:DescribeDomains"}
	}
	out := &opensearch.DescribeDomainsOutput{}
	for _, n := range in.DomainNames {
		out.DomainStatusList = append(out.DomainStatusList, f.domains[n])
	}
	return out, nil
}

// t563OpenSearchRegion holds seven domains; logs-7 has encryption at rest off.
func t563OpenSearchRegion() map[string]ostypes.DomainStatus {
	domains := map[string]ostypes.DomainStatus{}
	for i := 1; i <= 7; i++ {
		name := fmt.Sprintf("logs-%d", i)
		d := osTestBaseDomain(name)
		d.ClusterConfig = &ostypes.ClusterConfig{InstanceType: ostypes.OpenSearchPartitionInstanceTypeR6gLargeSearch, InstanceCount: aws.Int32(3)}
		if i == 7 {
			d.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{Enabled: aws.Bool(false)}
		}
		domains[name] = d
	}
	return domains
}

func t563AssertDescribedRow(t *testing.T, r resource.Resource) {
	t.Helper()
	want := map[string]string{
		"engine_version": "OpenSearch_2.11",
		"instance_type":  string(ostypes.OpenSearchPartitionInstanceTypeR6gLargeSearch),
		"endpoint":       r.ID + ".us-east-1.es.amazonaws.com",
	}
	for k, v := range want {
		if r.Fields[k] != v {
			t.Errorf("%s Fields[%q] = %q, want %q", r.ID, k, r.Fields[k], v)
		}
	}
	wantEncOff := r.ID == "logs-7"
	if got := hasCode(r.Findings, "opensearch.encryption-off"); got != wantEncOff {
		t.Errorf("%s opensearch.encryption-off = %v, want %v; findings = %+v", r.ID, got, wantEncOff, r.Findings)
	}
	if !wantEncOff && len(r.Findings) != 0 {
		t.Errorf("%s is healthy: findings = %+v, want none", r.ID, r.Findings)
	}
}

// DescribeDomains accepts at most five domain names per request (AWS API
// reference, DomainNames: "Maximum number of 5 items"), so a region with more
// domains is described in batches and every listed domain gets its details.
func TestT563_OpenSearchDescribesSevenDomainsInBatchesOfFive(t *testing.T) {
	fake := &osBatchLimitFake{domains: t563OpenSearchRegion()}

	rows, err := awsclient.FetchOpenSearchDomainsAt(context.Background(), fake, fake, time.Now())
	if err != nil {
		t.Fatalf("FetchOpenSearchDomainsAt: %v", err)
	}
	if len(rows) != 7 {
		t.Fatalf("got %d rows, want 7", len(rows))
	}
	for _, r := range rows {
		t563AssertDescribedRow(t, r)
	}

	asked := map[string]int{}
	for _, call := range fake.calls {
		if len(call) > 5 {
			t.Errorf("DescribeDomains called with %d names, the API accepts at most 5: %v", len(call), call)
		}
		for _, n := range call {
			asked[n]++
		}
	}
	for name := range fake.domains {
		if asked[name] != 1 {
			t.Errorf("%s described %d times, want once", name, asked[name])
		}
	}
}

// A failed DescribeDomains batch degrades only the domains it named; the
// other batch's domains keep their engine, instance type, endpoint and
// findings.
func TestT563_OpenSearchFailedBatchDegradesOnlyItsOwnNames(t *testing.T) {
	fake := &osBatchLimitFake{domains: t563OpenSearchRegion(), failName: "logs-1"}

	rows, err := awsclient.FetchOpenSearchDomainsAt(context.Background(), fake, fake, time.Now())
	if err == nil {
		t.Errorf("a denied batch must surface as a partial failure, got nil error")
	}
	if len(rows) != 7 {
		t.Fatalf("got %d rows, want 7 (a listed domain never vanishes)", len(rows))
	}

	var failed []string
	for _, call := range fake.calls {
		if len(call) <= 5 && slices.Contains(call, "logs-1") {
			failed = call
		}
	}
	if len(failed) == 0 {
		t.Fatalf("no batch of at most five named logs-1 apart from the rest; calls = %v", fake.calls)
	}

	degradedCode := awsclient.DegradedDetails("opensearch", "logs-1", &smithy.GenericAPIError{Code: "AccessDeniedException"}).Findings[0].Code
	for _, r := range rows {
		if slices.Contains(failed, r.ID) {
			if len(r.Findings) != 1 || r.Findings[0].Code != degradedCode {
				t.Errorf("%s was in the denied batch: findings = %+v, want exactly %s", r.ID, r.Findings, degradedCode)
			}
			continue
		}
		t563AssertDescribedRow(t, r)
	}
}
