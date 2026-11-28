package unit

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	s3controltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	t4S3Public     = domain.FindingCode("s3.public")
	t4S3Incomplete = domain.FindingCode("s3.public-access-block-incomplete")
	t4Bucket       = "acme-public-site"
)

// t4S3Account answers S3 Control GetPublicAccessBlock, the account-level
// block. A nil block is an account that never configured one, which S3 Control
// reports as NoSuchPublicAccessBlockConfiguration.
type t4S3Account struct {
	block *s3controltypes.PublicAccessBlockConfiguration
	err   error

	mu    sync.Mutex
	asked []string
}

func (f *t4S3Account) GetPublicAccessBlock(_ context.Context, in *s3control.GetPublicAccessBlockInput, _ ...func(*s3control.Options)) (*s3control.GetPublicAccessBlockOutput, error) {
	f.mu.Lock()
	f.asked = append(f.asked, aws.ToString(in.AccountId))
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if f.block == nil {
		return nil, &smithy.GenericAPIError{
			Code:    "NoSuchPublicAccessBlockConfiguration",
			Message: "The public access block configuration was not found",
		}
	}
	return &s3control.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: f.block}, nil
}

func (f *t4S3Account) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.asked)
}

func t4S3Clients(bucket awsclient.S3API, account *t4S3Account) *awsclient.ServiceClients {
	return identityResolved(&awsclient.ServiceClients{S3: bucket, S3Control: account, Region: "us-east-1"})
}

func t4WithAccount(c *awsclient.ServiceClients) *awsclient.ServiceClients {
	c.S3Control = &t4S3Account{}
	return identityResolved(c)
}

func t4AccountBlock(blockACLs, ignoreACLs, blockPolicy, restrict bool) *s3controltypes.PublicAccessBlockConfiguration {
	return &s3controltypes.PublicAccessBlockConfiguration{
		BlockPublicAcls:       aws.Bool(blockACLs),
		IgnorePublicAcls:      aws.Bool(ignoreACLs),
		BlockPublicPolicy:     aws.Bool(blockPolicy),
		RestrictPublicBuckets: aws.Bool(restrict),
	}
}

func t4BucketBlock(blockACLs, ignoreACLs, blockPolicy, restrict bool) *s3types.PublicAccessBlockConfiguration {
	return &s3types.PublicAccessBlockConfiguration{
		BlockPublicAcls:       aws.Bool(blockACLs),
		IgnorePublicAcls:      aws.Bool(ignoreACLs),
		BlockPublicPolicy:     aws.Bool(blockPolicy),
		RestrictPublicBuckets: aws.Bool(restrict),
	}
}

func t4AllUsersRead() []s3types.Grant {
	return []s3types.Grant{{
		Grantee:    &s3types.Grantee{Type: s3types.TypeGroup, URI: aws.String(t562AllUsers)},
		Permission: s3types.PermissionRead,
	}}
}

func t4Refusal(service, op string) error {
	return &smithy.OperationError{
		ServiceID:     service,
		OperationName: op,
		Err: &smithy.GenericAPIError{
			Code:    "AccessDenied",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform this operation",
		},
	}
}

func t4Buckets(names ...string) []resource.Resource {
	rs := make([]resource.Resource, 0, len(names))
	for _, n := range names {
		rs = append(rs, resource.Resource{ID: n, Name: n, Fields: map[string]string{"name": n}})
	}
	return rs
}

func t4EnrichS3(t *testing.T, clients *awsclient.ServiceClients, names ...string) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichS3Posture(context.Background(), clients, t4Buckets(names...), nil)
	if err != nil && len(res.TruncatedIDs) == 0 {
		t.Fatalf("EnrichS3Posture: %v", err)
	}
	return res
}

// "Amazon S3 enforces the most restrictive combination of account-level and
// bucket-level settings": the account's IgnorePublicAcls disarms the grant a
// bucket without its own block still carries.
func TestT4_S3_AccountIgnorePublicAclsClosesAnACLGrant(t *testing.T) {
	account := &t4S3Account{block: t4AccountBlock(false, true, false, false)}
	res := t4EnrichS3(t, t4S3Clients(&t562S3Fake{grants: t4AllUsersRead()}, account), t4Bucket)
	if hasCode(res.Findings[t4Bucket], t4S3Public) {
		t.Errorf("findings = %v, want no %s: the account ignores public ACLs", res.Findings[t4Bucket], t4S3Public)
	}
	if _, marked := res.TruncatedIDs[t4Bucket]; marked {
		t.Errorf("bucket marked not inspected (%q); every read answered", res.TruncatedIDs[t4Bucket])
	}
	if got := account.calls(); len(got) != 1 || got[0] != "123456789012" {
		t.Errorf("S3 Control GetPublicAccessBlock AccountId per call = %v, want [123456789012]", got)
	}

	open := t4EnrichS3(t, t4S3Clients(&t562S3Fake{grants: t4AllUsersRead()}, &t4S3Account{}), t4Bucket)
	if !hasCode(open.Findings[t4Bucket], t4S3Public) {
		t.Errorf("findings = %v, want %s: neither the bucket nor the account blocks the AllUsers grant", open.Findings[t4Bucket], t4S3Public)
	}
}

func TestT4_S3_AccountRestrictPublicBucketsClosesAPublicPolicy(t *testing.T) {
	account := &t4S3Account{block: t4AccountBlock(false, false, false, true)}
	res := t4EnrichS3(t, t4S3Clients(&t562S3Fake{policyPublic: true}, account), t4Bucket)
	if hasCode(res.Findings[t4Bucket], t4S3Public) {
		t.Errorf("findings = %v, want no %s: the account restricts public buckets", res.Findings[t4Bucket], t4S3Public)
	}
}

// IgnorePublicAcls and the two policy flags do not restrict a bucket whose
// policy is public; only RestrictPublicBuckets does.
func TestT4_S3_AccountBlockWithoutRestrictLeavesAPublicPolicyPublic(t *testing.T) {
	account := &t4S3Account{block: t4AccountBlock(true, true, true, false)}
	res := t4EnrichS3(t, t4S3Clients(&t562S3Fake{policyPublic: true}, account), t4Bucket)
	if !hasCode(res.Findings[t4Bucket], t4S3Public) {
		t.Fatalf("findings = %v, want %s: nothing restricts the public policy", res.Findings[t4Bucket], t4S3Public)
	}
	if v := t4RowValue(res.AttentionDetails[t4Bucket][t4S3Public].Rows, "Policy status"); v != "public" {
		t.Errorf("row Policy status = %q, want %q", v, "public")
	}
}

func TestT4_S3_AccountFlagCompletesTheBucketBlock(t *testing.T) {
	bucket := &t562S3Fake{pab: t4BucketBlock(false, true, true, true)}

	covered := t4EnrichS3(t, t4S3Clients(bucket, &t4S3Account{block: t4AccountBlock(true, false, false, false)}), t4Bucket)
	if hasCode(covered.Findings[t4Bucket], t4S3Incomplete) {
		t.Errorf("findings = %v, want no %s: the account sets the one flag the bucket lacks", covered.Findings[t4Bucket], t4S3Incomplete)
	}
	if got := covered.FieldUpdates[t4Bucket]["status"]; got == "public access block incomplete" {
		t.Errorf("Status cell = %q, want it not to say incomplete", got)
	}

	gap := t4EnrichS3(t, t4S3Clients(bucket, &t4S3Account{block: t4AccountBlock(false, true, true, true)}), t4Bucket)
	if !hasCode(gap.Findings[t4Bucket], t4S3Incomplete) {
		t.Fatalf("findings = %v, want %s: neither layer blocks public ACLs", gap.Findings[t4Bucket], t4S3Incomplete)
	}
	if got := gap.FieldUpdates[t4Bucket]["status"]; got != "public access block incomplete" {
		t.Errorf("Status cell = %q, want %q", got, "public access block incomplete")
	}
	t4RequireNoHedge(t, gap.AttentionDetails[t4Bucket][t4S3Incomplete].Rows)
}

func TestT4_S3_AccountBlockCoversABucketWithNoBlockOfItsOwn(t *testing.T) {
	res := t4EnrichS3(t, t4S3Clients(&t562S3Fake{}, &t4S3Account{block: t4AccountBlock(true, true, true, true)}), t4Bucket)
	if hasCode(res.Findings[t4Bucket], t4S3Incomplete) {
		t.Errorf("findings = %v, want no %s: the account blocks all four", res.Findings[t4Bucket], t4S3Incomplete)
	}
	if got := res.FieldUpdates[t4Bucket]["status"]; got == "public access block incomplete" {
		t.Errorf("Status cell = %q, want it not to say incomplete", got)
	}

	none := t4EnrichS3(t, t4S3Clients(&t562S3Fake{}, &t4S3Account{}), t4Bucket)
	if !hasCode(none.Findings[t4Bucket], t4S3Incomplete) {
		t.Fatalf("findings = %v, want %s: no block on either layer", none.Findings[t4Bucket], t4S3Incomplete)
	}
	t4RequireNoHedge(t, none.AttentionDetails[t4Bucket][t4S3Incomplete].Rows)
}

func TestT4_S3_RefusedAccountReadMarksTheBucketItDecides(t *testing.T) {
	refusal := t4Refusal("S3 Control", "GetPublicAccessBlock")
	res, err := awsclient.EnrichS3Posture(context.Background(),
		t4S3Clients(&t562S3Fake{policyPublic: true, grants: t4AllUsersRead()}, &t4S3Account{err: refusal}),
		t4Buckets(t4Bucket), nil)
	if err == nil {
		t.Error("error = nil, want the refused account read aggregated")
	}
	cause, marked := res.TruncatedIDs[t4Bucket]
	if !marked {
		t.Fatalf("TruncatedIDs = %v, want %s marked: its verdict depends on the account block nobody read", res.TruncatedIDs, t4Bucket)
	}
	if !strings.Contains(cause, "GetPublicAccessBlock") {
		t.Errorf("cause = %q, want it to name the refused GetPublicAccessBlock read", cause)
	}
	for _, code := range []domain.FindingCode{t4S3Public, t4S3Incomplete} {
		if hasCode(res.Findings[t4Bucket], code) {
			t.Errorf("findings = %v, want no %s from an unread account block", res.Findings[t4Bucket], code)
		}
	}
	if got := res.FieldUpdates[t4Bucket]["status"]; got == "public access block incomplete" {
		t.Errorf("Status cell = %q, want it not to say incomplete from an unread account block", got)
	}
}

func TestT4_S3_BucketClosedByItsOwnBlockIgnoresARefusedAccountRead(t *testing.T) {
	account := &t4S3Account{err: t4Refusal("S3 Control", "GetPublicAccessBlock")}
	bucket := &t562S3Fake{pab: t4BucketBlock(true, true, true, true), policyPublic: true, grants: t4AllUsersRead()}
	res, err := awsclient.EnrichS3Posture(context.Background(), t4S3Clients(bucket, account), t4Buckets(t4Bucket), nil)
	if err != nil {
		t.Errorf("error = %v, want nil: no read this bucket needed was refused", err)
	}
	if cause, marked := res.TruncatedIDs[t4Bucket]; marked {
		t.Errorf("bucket marked not inspected (%q), want unmarked: its own block decides", cause)
	}
	for _, code := range []domain.FindingCode{t4S3Public, t4S3Incomplete} {
		if hasCode(res.Findings[t4Bucket], code) {
			t.Errorf("findings = %v, want no %s", res.Findings[t4Bucket], code)
		}
	}
}

func TestT4_S3_AccountBlockIsNotReadWhenNoBucketNeedsIt(t *testing.T) {
	account := &t4S3Account{block: t4AccountBlock(true, true, true, true)}
	closed := &t562S3Fake{pab: t4BucketBlock(true, true, true, true), policyPublic: true, grants: t4AllUsersRead()}
	clients := t4S3Clients(closed, account)

	t4EnrichS3(t, clients, "acme-closed-a", "acme-closed-b")
	t4EnrichS3(t, clients)
	if got := account.calls(); len(got) != 0 {
		t.Errorf("S3 Control GetPublicAccessBlock called %d times, want 0: every bucket closes itself", len(got))
	}
}

func TestT4_S3_AccountBlockIsReadOncePerAccount(t *testing.T) {
	account := &t4S3Account{}
	clients := t4S3Clients(&t562S3Fake{grants: t4AllUsersRead()}, account)

	first := t4EnrichS3(t, clients, "acme-site-a", "acme-site-b", "acme-site-c")
	second := t4EnrichS3(t, clients, "acme-site-a", "acme-site-b", "acme-site-c")
	if got := account.calls(); len(got) != 1 {
		t.Errorf("S3 Control GetPublicAccessBlock called %d times over two passes of three buckets, want 1", len(got))
	}
	for _, res := range []awsclient.IssueEnricherResult{first, second} {
		for _, b := range []string{"acme-site-a", "acme-site-b", "acme-site-c"} {
			if !hasCode(res.Findings[b], t4S3Public) {
				t.Errorf("%s findings = %v, want %s", b, res.Findings[b], t4S3Public)
			}
		}
	}
}

// A trail delivering into a bucket is judged by the same verdict as the
// bucket's own row, so the account block closes it there too.
func TestT4_S3_TrailLogBucketReadsTheAccountBlock(t *testing.T) {
	trails, err := awsclient.FetchCloudTrailTrails(context.Background(), t562TrailListFake{})
	if err != nil {
		t.Fatalf("FetchCloudTrailTrails: %v", err)
	}
	bucket := &t562S3Fake{grants: t4AllUsersRead()}

	closed, err := awsclient.EnrichTrailLogBucket(context.Background(),
		t4S3Clients(bucket, &t4S3Account{block: t4AccountBlock(false, true, false, false)}), trails, nil)
	if err != nil {
		t.Fatalf("EnrichTrailLogBucket: %v", err)
	}
	if hasCode(closed.Findings[trails[0].ID], awsclient.CodeTrailLogBucketPublic) {
		t.Errorf("trail findings = %v, want no %s: the account ignores public ACLs", closed.Findings[trails[0].ID], awsclient.CodeTrailLogBucketPublic)
	}

	open, err := awsclient.EnrichTrailLogBucket(context.Background(), t4S3Clients(bucket, &t4S3Account{}), trails, nil)
	if err != nil {
		t.Fatalf("EnrichTrailLogBucket: %v", err)
	}
	if !hasCode(open.Findings[trails[0].ID], awsclient.CodeTrailLogBucketPublic) {
		t.Errorf("trail findings = %v, want %s", open.Findings[trails[0].ID], awsclient.CodeTrailLogBucketPublic)
	}
}

func t4RowValue(rows []domain.DetailRow, label string) string {
	for _, r := range rows {
		if r.Label == label {
			return r.Value
		}
	}
	return ""
}

func t4RequireNoHedge(t *testing.T, rows []domain.DetailRow) {
	t.Helper()
	for _, r := range rows {
		if r.Value == "may still apply" {
			t.Errorf("row %q = %q, want the account block's read value", r.Label, r.Value)
		}
	}
}
