package unit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// An agreement kept from its ListedAgreement still names both profile ids, so
// opening its detail resolves them like any described agreement.
func TestT577Dev_TransferDegradedAgreement_DetailResolvesProfiles(t *testing.T) {
	child := transferAgreementsChildShortName(t)
	denied := &t577TransferFake{TransferAPI: fakes.NewTransfer(), denyDescribeAgreement: true}
	result, _ := awsclient.FetchTransferAgreements(context.Background(), denied, fixtures.ProdAS2GatewayID, "")
	row := mustFindTransferResource(t, result.Resources, fixtures.AgreementProdPartnerID)

	enriched, err := resource.GetDetailEnricher(child)(context.Background(),
		&awsclient.DetailEnrichmentCtx{Clients: &awsclient.ServiceClients{Transfer: fakes.NewTransfer()}}, row)
	if err != nil {
		t.Fatalf("detail enrichment of the degraded agreement: %v", err)
	}
	if enriched.Fields["local_profile"] != "ACME-LOCAL" || enriched.Fields["partner_profile"] != "PARTNER-CO" {
		t.Errorf("profiles = (%q, %q), want (ACME-LOCAL, PARTNER-CO)", enriched.Fields["local_profile"], enriched.Fields["partner_profile"])
	}
}

func TestT577Dev_DetailResultApplies(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"success", nil, true},
		{"partial", domain.PartialDetailErr{Err: errors.New("DescribeCertificate denied")}, true},
		{"wrapped partial", fmt.Errorf("enrich: %w", domain.PartialDetailErr{Err: errors.New("x")}), true},
		{"failure", errors.New("DescribeProfile denied"), false},
		{"skipped", awsclient.ErrDetailEnrichSkipped, false},
	} {
		if got := domain.DetailResultApplies(tc.err); got != tc.want {
			t.Errorf("%s: DetailResultApplies = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// One global database per page: past PerParentPageCap pages the member list
// is a lower bound. A member read on an early page keeps its verdict, a
// member past the cap has none, and nothing is reported as a failed call.
func TestT577Dev_DBCGlobalMembersPastPageCap_LowerBound(t *testing.T) {
	here := func(id string) string { return aws.ToString(t564ClusterARN(id)) }
	g := &t564GlobalEndpoint{}
	for i := range awsclient.PerParentPageCap + 2 {
		id := "acme-global-" + strconv.Itoa(i)
		g.globals = append(g.globals, t564Global{id: id, engine: "aurora-postgresql", writer: map[string]bool{t564OtherRegionARN(id + "-other"): false}})
	}
	g.globals[0].writer[here("acme-early-primary")] = true
	last := len(g.globals) - 1
	g.globals[last].writer[here("acme-late-primary")] = true

	rows, err := t564ListDBC(t, []t564Cluster{
		{id: "acme-early-primary", engine: "aurora-postgresql", global: g.globals[0].id},
		{id: "acme-late-primary", engine: "aurora-postgresql", global: g.globals[last].id},
	}, g)

	if !t564HasCode(rows["acme-early-primary"].Findings, awsclient.CodeDBCNoWriter) {
		t.Errorf("member read on page 1: findings %v, want %s", rows["acme-early-primary"].Findings, awsclient.CodeDBCNoWriter)
	}
	if t564HasCode(rows["acme-late-primary"].Findings, awsclient.CodeDBCNoWriter) {
		t.Errorf("member past the cap: findings %v, want no verdict", rows["acme-late-primary"].Findings)
	}
	if err == nil || !strings.Contains(err.Error(), awsclient.CheckCap) || strings.Contains(err.Error(), "more than") {
		t.Errorf("error = %v, want the unread verdict to name %q, not a failed call", err, awsclient.CheckCap)
	}
}

type t577DevEFS struct {
	awsclient.EFSDescribeFileSystemsAPI
	apCalls int
}

func (f *t577DevEFS) DescribeFileSystems(context.Context, *efs.DescribeFileSystemsInput, ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
	return &efs.DescribeFileSystemsOutput{FileSystems: []efstypes.FileSystemDescription{{
		FileSystemId: aws.String("fs-0a1b2c3d"), Name: aws.String("acme-shared"), LifeCycleState: efstypes.LifeCycleStateAvailable,
		NumberOfMountTargets: 2, Encrypted: aws.Bool(true),
	}}}, nil
}

func (f *t577DevEFS) DescribeAccessPoints(context.Context, *efs.DescribeAccessPointsInput, ...func(*efs.Options)) (*efs.DescribeAccessPointsOutput, error) {
	f.apCalls++
	return &efs.DescribeAccessPointsOutput{
		AccessPoints: []efstypes.AccessPointDescription{{AccessPointId: aws.String(fmt.Sprintf("fsap-%08x", f.apCalls))}},
		NextToken:    aws.String("more"),
	}, nil
}

// A file system with more access-point pages than PerParentPageCap keeps the
// IDs read, and the fetch reports no failed DescribeAccessPoints.
func TestT577Dev_EFSAccessPointsPastPageCap_NoFailure(t *testing.T) {
	api := &t577DevEFS{}
	result, err := awsclient.FetchEFSFileSystemsPage(context.Background(), api, "")
	if err != nil {
		t.Errorf("fetch error = %v, want none: a9s's own page cap is not a failed call", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("got %d rows, want 1", len(result.Resources))
	}
	if got := strings.Count(result.Resources[0].Fields["access_point_ids"], "fsap-"); got != awsclient.PerParentPageCap {
		t.Errorf("access_point_ids holds %d IDs, want the %d read", got, awsclient.PerParentPageCap)
	}
}

type t577DevIAMListCap struct {
	awsclient.IAMAPI
	page int
}

func (f *t577DevIAMListCap) ListPolicies(context.Context, *iam.ListPoliciesInput, ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	f.page++
	name := "acme-policy-" + strconv.Itoa(f.page)
	return &iam.ListPoliciesOutput{
		Policies:    []iamtypes.Policy{{PolicyName: aws.String(name), PolicyId: aws.String("ANPA" + strconv.Itoa(f.page)), Arn: aws.String("arn:aws:iam::123456789012:policy/" + name)}},
		IsTruncated: true,
		Marker:      aws.String("m" + strconv.Itoa(f.page)),
	}, nil
}

func (f *t577DevIAMListCap) ListGroups(context.Context, *iam.ListGroupsInput, ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{}, nil
}

// ListPolicies stopping at PerParentPageCap still answers for the policies it
// read: the lazy-add resolves a policy from page 1 without an error.
func TestT577Dev_IAMListPoliciesPastPageCap_ResolvesWhatWasRead(t *testing.T) {
	results, err := awsclient.FetchIAMPoliciesByIDsFull(context.Background(), &t577DevIAMListCap{}, []string{"acme-policy-1"}, session.NewPolicyStore(), "aws")
	if err != nil {
		t.Errorf("error = %v, want none for a policy read before the cap", err)
	}
	if len(results) != 1 || results[0].Name != "acme-policy-1" {
		t.Errorf("results = %+v, want acme-policy-1", results)
	}
}

type t577DevIAMNoVersion struct {
	awsclient.IAMAPI
}

func (t577DevIAMNoVersion) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return &iam.GetPolicyOutput{Policy: &iamtypes.Policy{Arn: in.PolicyArn}}, nil
}

// A GetPolicy answer without a default version id is an unusable answer: the
// row names the call and the missing field, and the aggregate names both
// calls the enricher makes.
func TestT577Dev_IAMPolicy_GetPolicyWithoutVersion_MarkedUnusable(t *testing.T) {
	arn := "arn:aws:iam::123456789012:policy/acme-deploy"
	result, err := awsclient.EnrichIAMPolicy(context.Background(), &awsclient.ServiceClients{IAM: t577DevIAMNoVersion{}},
		[]resource.Resource{{ID: arn, Name: "acme-deploy", Fields: map[string]string{}}}, nil)
	if got, want := result.TruncatedIDs[arn], "GetPolicy returned no default version id"; got != want {
		t.Errorf("TruncatedIDs[%s] = %q, want %q", arn, got, want)
	}
	if err == nil || !strings.Contains(err.Error(), "GetPolicy and GetPolicyVersion failed for 1 of 1") {
		t.Errorf("error = %v, want it labelled with both calls", err)
	}
}
