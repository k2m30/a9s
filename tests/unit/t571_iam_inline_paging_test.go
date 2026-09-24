package unit_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// t571IAMPagedFake pages ListGroups and ListGroupPolicies the way IAM does:
// IsTruncated plus an opaque Marker that the next request must echo back.
// A request without the marker gets page one again.
type t571IAMPagedFake struct {
	awsclient.IAMAPI

	groupPages  [][]string
	endlessList bool // ListGroups never reports its last page
	policyPages map[string][][]string
}

func t571Marker(page int) *string { return aws.String("t571-page-" + strconv.Itoa(page)) }

func t571PageIndex(marker *string) int {
	if marker == nil {
		return 0
	}
	var n int
	if _, err := fmt.Sscanf(*marker, "t571-page-%d", &n); err != nil {
		return -1
	}
	return n
}

func (f *t571IAMPagedFake) ListPolicies(context.Context, *iam.ListPoliciesInput, ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{}, nil
}

func (f *t571IAMPagedFake) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return nil, &iamtypes.NoSuchEntityException{Message: aws.String("Policy " + aws.ToString(in.PolicyArn) + " was not found.")}
}

func (f *t571IAMPagedFake) ListGroups(_ context.Context, in *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	i := t571PageIndex(in.Marker)
	if f.endlessList {
		name := "ops-shard-" + strconv.Itoa(i)
		return &iam.ListGroupsOutput{
			Groups:      []iamtypes.Group{{GroupName: aws.String(name), GroupId: aws.String("AGPAEXAMPLE" + strconv.Itoa(i)), Path: aws.String("/")}},
			IsTruncated: true,
			Marker:      t571Marker(i + 1),
		}, nil
	}
	if i < 0 || i >= len(f.groupPages) {
		return nil, errors.New("InvalidInput: marker does not name a page")
	}
	out := &iam.ListGroupsOutput{}
	for _, name := range f.groupPages[i] {
		out.Groups = append(out.Groups, iamtypes.Group{GroupName: aws.String(name), GroupId: aws.String("AGPAEXAMPLE" + name), Path: aws.String("/")})
	}
	if i+1 < len(f.groupPages) {
		out.IsTruncated = true
		out.Marker = t571Marker(i + 1)
	}
	return out, nil
}

func (f *t571IAMPagedFake) ListGroupPolicies(_ context.Context, in *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	pages := f.policyPages[aws.ToString(in.GroupName)]
	i := t571PageIndex(in.Marker)
	if len(pages) == 0 && i == 0 {
		return &iam.ListGroupPoliciesOutput{PolicyNames: []string{}}, nil
	}
	if i < 0 || i >= len(pages) {
		return nil, errors.New("InvalidInput: marker does not name a page")
	}
	out := &iam.ListGroupPoliciesOutput{PolicyNames: pages[i]}
	if i+1 < len(pages) {
		out.IsTruncated = true
		out.Marker = t571Marker(i + 1)
	}
	return out, nil
}

func (f *t571IAMPagedFake) ListAttachedGroupPolicies(context.Context, *iam.ListAttachedGroupPoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	return &iam.ListAttachedGroupPoliciesOutput{}, nil
}

func t571InlineID(group, policy string) string { return "inline/" + group + "/" + policy }

// Three pages of groups; "platform-admins" sits on the last one and its
// inline policies span two ListGroupPolicies pages.
func t571PagedAccount() *t571IAMPagedFake {
	return &t571IAMPagedFake{
		groupPages: [][]string{
			{"developers", "readonly-auditors"},
			{"billing", "support"},
			{"platform-admins"},
		},
		policyPages: map[string][][]string{
			"developers":      {{"s3-read"}},
			"billing":         {{"ce-read"}},
			"platform-admins": {{"ec2-admin", "rds-admin"}, {"kms-decrypt"}},
		},
	}
}

// The policy list's inline sweep reaches every group ListGroups returns
// across its Marker pages, and every inline policy ListGroupPolicies returns
// across its own: a group past the first page has inline policies like any
// other, and leaving them out hides them from the list and from every pivot
// that resolves against it.
func TestT571_PolicyListSweep_ReachesEveryGroupAndInlinePolicyThroughMarker(t *testing.T) {
	fake := t571PagedAccount()
	fetch := resource.GetPaginatedFetcher("policy")
	if fetch == nil {
		t.Fatal("no paginated fetcher registered for policy")
	}
	result, err := fetch(context.Background(), &awsclient.ServiceClients{IAM: fake}, "")
	if err != nil {
		t.Fatalf("fetch error = %v, want nil — every page answered", err)
	}
	got := map[string]bool{}
	for _, r := range result.Resources {
		got[r.ID] = true
	}
	for _, id := range []string{
		t571InlineID("developers", "s3-read"),
		t571InlineID("billing", "ce-read"),
		t571InlineID("platform-admins", "ec2-admin"),
		t571InlineID("platform-admins", "rds-admin"),
		t571InlineID("platform-admins", "kms-decrypt"),
	} {
		if !got[id] {
			t.Errorf("inline policy %s missing from the policy list; got %v", id, result.Resources)
		}
	}
	if len(result.Resources) != 5 {
		t.Errorf("policy list has %d rows, want exactly the 5 inline policies", len(result.Resources))
	}
}

// The group -> policy drill resolves an inline policy by its row id against
// the session's policy store. A policy on a group past ListGroups' first page,
// or past its group's first ListGroupPolicies page, opens like one on page one.
func TestT571_GroupPolicyDrill_OpensInlinePolicyOnLaterPages(t *testing.T) {
	store := session.NewPolicyStore()
	ids := []string{
		t571InlineID("developers", "s3-read"),
		t571InlineID("platform-admins", "ec2-admin"),
		t571InlineID("platform-admins", "kms-decrypt"),
	}
	got, err := awsclient.FetchIAMPoliciesByIDsFull(context.Background(), t571PagedAccount(), ids, store, "aws")
	if err != nil {
		t.Errorf("FetchIAMPoliciesByIDsFull error = %v, want nil — every requested policy exists", err)
	}
	opened := map[string]bool{}
	for _, r := range got {
		opened[r.ID] = true
	}
	for _, id := range ids {
		if !opened[id] {
			t.Errorf("drill did not open %s; opened %v", id, got)
		}
	}
}

// An account whose ListGroups never ends within PerParentPageCap pages was
// read in part: the list must say so instead of presenting the inline rows
// it saw as the whole account.
func TestT571_PolicyListSweep_PastPageCapIsALowerBound(t *testing.T) {
	fake := &t571IAMPagedFake{endlessList: true, policyPages: map[string][][]string{}}
	for i := 0; i < awsclient.PerParentPageCap+5; i++ {
		fake.policyPages["ops-shard-"+strconv.Itoa(i)] = [][]string{{"inline-" + strconv.Itoa(i)}}
	}
	result, err := resource.GetPaginatedFetcher("policy")(context.Background(), &awsclient.ServiceClients{IAM: fake}, "")
	partial := err != nil || (result.Pagination != nil && result.Pagination.IsTruncated)
	if !partial {
		t.Errorf("sweep stopped short of an unfinished ListGroups yet reported a complete list: err=nil, IsTruncated=false, %d rows", len(result.Resources))
	}
	if len(result.Resources) < awsclient.PerParentPageCap {
		t.Errorf("sweep kept %d inline rows, want at least the %d groups read within the page cap", len(result.Resources), awsclient.PerParentPageCap)
	}
}

// A group with more inline policies than PerParentPageCap pages is a lower
// bound on the group -> policy pivot, never an exact count.
func TestT571_GroupPolicyPivot_PastPageCapIsALowerBound(t *testing.T) {
	var pages [][]string
	for i := 0; i < awsclient.PerParentPageCap+2; i++ {
		pages = append(pages, []string{"inline-" + strconv.Itoa(i)})
	}
	fake := &t571IAMPagedFake{policyPages: map[string][][]string{"platform-admins": pages}}
	group := resource.Resource{ID: "platform-admins", Name: "platform-admins"}
	got := checkerByTarget(t, "iam-group", "policy")(context.Background(), &awsclient.ServiceClients{IAM: fake}, group, nil)
	if got.State() != domain.RelatedResolved || !got.Truncated() {
		t.Fatalf("state=%v truncated=%v, want resolved lower bound", got.State(), got.Truncated())
	}
	if got.Count() != awsclient.PerParentPageCap {
		t.Errorf("Count = %d, want %d (one policy per page read)", got.Count(), awsclient.PerParentPageCap)
	}

	whole := &t571IAMPagedFake{policyPages: map[string][][]string{"platform-admins": {{"ec2-admin"}, {"kms-decrypt"}}}}
	exact := checkerByTarget(t, "iam-group", "policy")(context.Background(), &awsclient.ServiceClients{IAM: whole}, group, nil)
	if exact.Truncated() || exact.Count() != 2 {
		t.Errorf("group read whole: count=%d truncated=%v, want exact 2", exact.Count(), exact.Truncated())
	}
}
