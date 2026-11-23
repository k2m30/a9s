package unit

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

var t577FailedForRe = regexp.MustCompile(`failed for (\d+) of (\d+)`)

// t577AssertCountsSane checks every "failed for N of M" in err: a failure
// count is a subset of what was attempted, and nothing fails out of nothing.
func t577AssertCountsSane(t *testing.T, err error) {
	t.Helper()
	matches := t577FailedForRe.FindAllStringSubmatch(err.Error(), -1)
	if len(matches) == 0 {
		t.Errorf("error %q states no \"failed for N of M\" count", err)
	}
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		total, _ := strconv.Atoi(m[2])
		if total == 0 || n > total {
			t.Errorf("error %q: count %q has failed > attempted or attempted == 0", err, m[0])
		}
	}
}

type t577DDBFake struct {
	awsclient.DynamoDBAPI

	mu          sync.Mutex
	policyCalls []string
}

func (f *t577DDBFake) GetResourcePolicy(ctx context.Context, in *dynamodb.GetResourcePolicyInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetResourcePolicyOutput, error) {
	f.mu.Lock()
	f.policyCalls = append(f.policyCalls, aws.ToString(in.ResourceArn))
	f.mu.Unlock()
	if aws.ToString(in.ResourceArn) == "" {
		// What DynamoDB answers for a request with no ARN.
		return nil, &smithy.OperationError{ServiceID: "DynamoDB", OperationName: "GetResourcePolicy", Err: &smithy.GenericAPIError{Code: "ValidationException", Message: "1 validation error detected: Value '' at 'resourceArn' failed to satisfy constraint"}}
	}
	return f.DynamoDBAPI.GetResourcePolicy(ctx, in, opts...)
}

// A table whose DescribeTable failed is a name-only row with no ARN; the demo
// carries one denied and one unavailable. Its resource policy cannot be asked
// for — GetResourcePolicy takes an ARN, and an empty one is a request DynamoDB
// rejects — so the row stays not inspected on the call that actually failed,
// DescribeTable, and no GetResourcePolicy failure is invented for it.
func TestT577_DDB_DegradedRow_NoPolicyReadWithEmptyARN(t *testing.T) {
	clients := demo.NewServiceClients()
	fake := &t577DDBFake{DynamoDBAPI: clients.DynamoDB}
	clients.DynamoDB = fake
	page, fetchErr := awsclient.FetchDynamoDBTablesPage(context.Background(), fake, fake, "")
	if fetchErr == nil {
		t.Fatal("precondition: the demo ddb list carries no DescribeTable failure")
	}

	var degraded, described []resource.Resource
	for _, r := range page.Resources {
		if r.Fields["arn"] == "" {
			if r.RawStruct != nil {
				t.Fatalf("precondition: %s has no arn but a RawStruct %T", r.ID, r.RawStruct)
			}
			degraded = append(degraded, r)
		} else {
			described = append(described, r)
		}
	}
	if len(degraded) == 0 || len(described) == 0 {
		t.Fatalf("precondition: want degraded and described tables, got %d and %d", len(degraded), len(described))
	}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, page.Resources, nil)

	for _, arn := range fake.policyCalls {
		if arn == "" {
			t.Errorf("GetResourcePolicy was called with an empty ResourceArn (calls: %q)", fake.policyCalls)
			break
		}
	}
	for _, r := range degraded {
		if got := result.TruncatedIDs[r.ID]; got != "DescribeTable" {
			t.Errorf("TruncatedIDs[%s] = %q, want %q — the row is uninspected because DescribeTable failed", r.ID, got, "DescribeTable")
		}
	}
	if err != nil && strings.Contains(err.Error(), "GetResourcePolicy") {
		t.Errorf("error = %q names GetResourcePolicy, which never failed for a table it was asked about", err)
	}

	public := 0
	for _, r := range described {
		if result.TruncatedIDs[r.ID] == "DescribeTable" {
			t.Errorf("described table %s is marked uninspected on DescribeTable", r.ID)
		}
		if t577HasFinding(result.Findings[r.ID], "ddb.public-policy") {
			public++
		}
	}
	if public == 0 {
		t.Error("no described table carries ddb.public-policy; the policy read of tables with an ARN must still run")
	}
}

type t577EKSFake struct {
	awsclient.EKSAPI
	denyList     map[string]bool
	denyDescribe string
}

// The demo fails reads on every run; hiding its witnesses leaves each case
// with only the failure it injects.
func (f *t577EKSFake) ListNodegroups(ctx context.Context, in *eks.ListNodegroupsInput, opts ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	cluster := aws.ToString(in.ClusterName)
	if f.denyList[cluster] {
		return nil, t577Denied("EKS", "ListNodegroups", "eks:ListNodegroups")
	}
	if cluster == fixtures.WarnEKSDetailsDeniedID {
		return &eks.ListNodegroupsOutput{}, nil
	}
	out, err := f.EKSAPI.ListNodegroups(ctx, in, opts...)
	if err != nil {
		return out, err
	}
	deniedLT := fixtures.NewLTFixtures().DeniedIDs
	cp := *out
	cp.Nodegroups = slices.DeleteFunc(slices.Clone(out.Nodegroups), func(n string) bool {
		if strings.HasPrefix(n, "warn-ng-details-") {
			return true
		}
		d, err := f.EKSAPI.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{ClusterName: in.ClusterName, NodegroupName: aws.String(n)})
		return err == nil && d.Nodegroup != nil && d.Nodegroup.LaunchTemplate != nil && slices.Contains(deniedLT, aws.ToString(d.Nodegroup.LaunchTemplate.Id))
	})
	return &cp, nil
}

func (f *t577EKSFake) DescribeNodegroup(ctx context.Context, in *eks.DescribeNodegroupInput, opts ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	if aws.ToString(in.NodegroupName) == f.denyDescribe {
		return nil, t577Denied("EKS", "DescribeNodegroup", "eks:DescribeNodegroup")
	}
	return f.EKSAPI.DescribeNodegroup(ctx, in, opts...)
}

type t577EC2Fake struct {
	awsclient.EC2API
	failLTV bool
	ltvCall int
}

func (f *t577EC2Fake) DescribeLaunchTemplateVersions(ctx context.Context, in *ec2.DescribeLaunchTemplateVersionsInput, opts ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	f.ltvCall++
	if f.failLTV {
		return nil, &smithy.OperationError{ServiceID: "EC2", OperationName: "DescribeLaunchTemplateVersions", Err: &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "You are not authorized to perform this operation."}}
	}
	return f.EC2API.(interface {
		DescribeLaunchTemplateVersions(context.Context, *ec2.DescribeLaunchTemplateVersionsInput, ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error)
	}).DescribeLaunchTemplateVersions(ctx, in, opts...)
}

func t577NGFetch(t *testing.T, eksFake *t577EKSFake, ec2Fake *t577EC2Fake) ([]resource.Resource, error) {
	t.Helper()
	clients := demo.NewServiceClients()
	eksFake.EKSAPI = clients.EKS
	ec2Fake.EC2API = clients.EC2
	clients.EKS = eksFake
	clients.EC2 = ec2Fake
	fetch := resource.GetPaginatedFetcher("ng")
	if fetch == nil {
		t.Fatal("no paginated fetcher registered for ng")
	}
	res, err := fetch(context.Background(), clients, "")
	return res.Resources, err
}

func t577DemoClusters(t *testing.T) []string {
	t.Helper()
	out, err := demo.NewServiceClients().EKS.ListClusters(context.Background(), &eks.ListClustersInput{})
	if err != nil || len(out.Clusters) == 0 {
		t.Fatalf("precondition: demo ListClusters: %v (%d clusters)", err, len(out.Clusters))
	}
	return slices.DeleteFunc(out.Clusters, func(c string) bool { return c == fixtures.WarnEKSDetailsDeniedID })
}

// A denied ListNodegroups is a failure of ListNodegroups for that cluster, not
// of DescribeNodegroup: no node group of the cluster was ever named, so none
// was described. The count states failed clusters out of clusters attempted.
func TestT577_NG_ListNodegroupsDenied_FailureNamesListNodegroups(t *testing.T) {
	clusters := t577DemoClusters(t)
	baseline, err := t577NGFetch(t, &t577EKSFake{}, &t577EC2Fake{})
	if err != nil {
		t.Fatalf("precondition: clean ng fetch: %v", err)
	}

	t.Run("every cluster denied", func(t *testing.T) {
		deny := map[string]bool{}
		for _, c := range clusters {
			deny[c] = true
		}
		rows, err := t577NGFetch(t, &t577EKSFake{denyList: deny}, &t577EC2Fake{})
		if err == nil {
			t.Fatal("ng fetch returned nil error with every ListNodegroups denied")
		}
		if len(rows) != 0 {
			t.Errorf("got %d rows with no cluster's node groups listable, want 0", len(rows))
		}
		if !strings.Contains(err.Error(), "ListNodegroups") {
			t.Errorf("error = %q, want it to name ListNodegroups", err)
		}
		if strings.Contains(err.Error(), "DescribeNodegroup") {
			t.Errorf("error = %q names DescribeNodegroup, which was never called", err)
		}
		t577AssertCountsSane(t, err)
	})

	t.Run("one cluster denied", func(t *testing.T) {
		rows, err := t577NGFetch(t, &t577EKSFake{denyList: map[string]bool{clusters[0]: true}}, &t577EC2Fake{})
		if err == nil {
			t.Fatal("ng fetch returned nil error with one cluster's ListNodegroups denied")
		}
		if !strings.Contains(err.Error(), "ListNodegroups") {
			t.Errorf("error = %q, want it to name ListNodegroups", err)
		}
		if strings.Contains(err.Error(), "DescribeNodegroup") {
			t.Errorf("error = %q names DescribeNodegroup, which did not fail", err)
		}
		t577AssertCountsSane(t, err)
		for _, r := range rows {
			if r.Fields["cluster_name"] == clusters[0] {
				t.Errorf("row %s belongs to the denied cluster %s", r.ID, clusters[0])
			}
		}
		if len(rows) > len(baseline) {
			t.Errorf("got %d rows, more than the clean %d", len(rows), len(baseline))
		}
	})
}

// A launch template version that could not be read is a DescribeLaunchTemplateVersions
// failure; the node group itself was described and stays listed.
func TestT577_NG_LaunchTemplateVersionsFailed_FailureNamesThatCall(t *testing.T) {
	cleanEC2 := &t577EC2Fake{}
	baseline, err := t577NGFetch(t, &t577EKSFake{}, cleanEC2)
	if err != nil {
		t.Fatalf("precondition: clean ng fetch: %v", err)
	}
	if cleanEC2.ltvCall == 0 {
		t.Fatal("precondition: no demo node group names a launch template, so DescribeLaunchTemplateVersions is never called")
	}

	rows, err := t577NGFetch(t, &t577EKSFake{}, &t577EC2Fake{failLTV: true})
	if err == nil {
		t.Fatal("ng fetch returned nil error with DescribeLaunchTemplateVersions failing")
	}
	if !strings.Contains(err.Error(), "DescribeLaunchTemplateVersions") {
		t.Errorf("error = %q, want it to name DescribeLaunchTemplateVersions", err)
	}
	if strings.Contains(err.Error(), "DescribeNodegroup") {
		t.Errorf("error = %q names DescribeNodegroup, which answered", err)
	}
	t577AssertCountsSane(t, err)
	if len(rows) != len(baseline) {
		t.Errorf("got %d rows, want all %d node groups still listed", len(rows), len(baseline))
	}
}

func TestT577_NG_DescribeNodegroupDenied_FailureNamesDescribeNodegroup(t *testing.T) {
	baseline, err := t577NGFetch(t, &t577EKSFake{}, &t577EC2Fake{})
	if err != nil || len(baseline) == 0 {
		t.Fatalf("precondition: clean ng fetch: %v (%d rows)", err, len(baseline))
	}
	target := baseline[0].Name
	_, err = t577NGFetch(t, &t577EKSFake{denyDescribe: target}, &t577EC2Fake{})
	if err == nil {
		t.Fatal("ng fetch returned nil error with DescribeNodegroup denied")
	}
	if !strings.Contains(err.Error(), "DescribeNodegroup") {
		t.Errorf("error = %q, want it to name DescribeNodegroup", err)
	}
	if want := "failed for 1 of " + strconv.Itoa(len(baseline)); !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want %q", err, want)
	}
}
