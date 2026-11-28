package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type t9cDevIAM struct {
	enrichPolicyIAM
	roles      map[string]iamtypes.Role
	getRole    int
	getUser    int
	getPolicy  int
	getVersion int
	version    string
}

func (f *t9cDevIAM) GetRole(_ context.Context, in *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	f.getRole++
	role := f.roles[aws.ToString(in.RoleName)]
	return &iam.GetRoleOutput{Role: &role}, nil
}

func (f *t9cDevIAM) GetUser(_ context.Context, in *iam.GetUserInput, _ ...func(*iam.Options)) (*iam.GetUserOutput, error) {
	f.getUser++
	return &iam.GetUserOutput{User: &iamtypes.User{
		UserName:            in.UserName,
		Path:                aws.String("/from-get/"),
		PermissionsBoundary: &iamtypes.AttachedPermissionsBoundary{PermissionsBoundaryArn: aws.String("arn:aws:iam::123456789012:policy/acme-boundary")},
	}}, nil
}

func (f *t9cDevIAM) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	f.getPolicy++
	return &iam.GetPolicyOutput{Policy: &iamtypes.Policy{Arn: in.PolicyArn, DefaultVersionId: aws.String("v9"), Description: aws.String("from GetPolicy")}}, nil
}

func (f *t9cDevIAM) GetPolicyVersion(_ context.Context, in *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	f.getVersion++
	f.version = aws.ToString(in.VersionId)
	return &iam.GetPolicyVersionOutput{PolicyVersion: &iamtypes.PolicyVersion{VersionId: in.VersionId, Document: aws.String(buildVersionDoc(`{"Statement":[]}`))}}, nil
}

func t9cDevCtx(api *t9cDevIAM) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{Clients: &awsclient.ServiceClients{IAM: api}, PolicyDocs: &awsclient.PolicyDocumentCache{}, DetailDocs: &awsclient.DetailDocCache{}}
}

// A service-linked role's Last Used cell is GetRole's answer like any role's:
// no finding stands on it, and the column still does.
func TestT9cDev_ServiceLinkedRoleGetsLastUsed(t *testing.T) {
	used := time.Date(2026, 5, 6, 7, 8, 0, 0, time.UTC)
	api := &t9cDevIAM{roles: map[string]iamtypes.Role{
		"AWSServiceRoleForECS": {RoleName: aws.String("AWSServiceRoleForECS"), Path: aws.String("/aws-service-role/ecs.amazonaws.com/"), RoleLastUsed: &iamtypes.RoleLastUsed{LastUsedDate: &used}},
	}}
	row := resource.Resource{
		ID:        "AWSServiceRoleForECS",
		Fields:    map[string]string{"role_name": "AWSServiceRoleForECS"},
		RawStruct: api.roles["AWSServiceRoleForECS"],
	}
	res, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), &awsclient.ServiceClients{IAM: api}, []resource.Resource{row}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.FieldUpdates[row.ID]["last_used"]; got != "2026-05-06 07:08" {
		t.Errorf("last_used = %q, want GetRole's date", got)
	}
	if len(res.Findings[row.ID]) != 0 {
		t.Errorf("findings on a service-linked role: %v", res.Findings[row.ID])
	}
}

// The detail takes only the members the list call leaves out from the Get
// answer, and reads that answer once a session: a member both return is the
// row's, and a second open asks AWS nothing.
func TestT9cDev_GetAnswerFillsOmissionsOnceASession(t *testing.T) {
	api := &t9cDevIAM{}
	dctx := t9cDevCtx(api)
	user := resource.Resource{ID: "alice", RawStruct: iamtypes.User{UserName: aws.String("alice"), Path: aws.String("/listed/")}}
	var got resource.Resource
	for range 2 {
		var err error
		if got, err = enrichIAMUserForTest(t)(context.Background(), dctx, user); err != nil {
			t.Fatal(err)
		}
	}
	u, ok := got.RawStruct.(iamtypes.User)
	if !ok {
		t.Fatalf("RawStruct = %T", got.RawStruct)
	}
	if u.PermissionsBoundary == nil {
		t.Error("PermissionsBoundary, which ListUsers leaves out, is not GetUser's")
	}
	if aws.ToString(u.Path) != "/listed/" {
		t.Errorf("Path = %q, want the listed value", aws.ToString(u.Path))
	}
	if api.getUser != 1 {
		t.Errorf("GetUser called %d times over two opens, want 1", api.getUser)
	}
}

// A policy's document is read for the version the row shows, and GetPolicy is
// read once a session beside it, so the shown version and the document are
// one read.
func TestT9cDev_PolicyDocumentIsTheShownVersion(t *testing.T) {
	api := &t9cDevIAM{}
	dctx := t9cDevCtx(api)
	const arn = "arn:aws:iam::123456789012:policy/acme-read"
	row := resource.Resource{ID: arn, RawStruct: iamtypes.Policy{Arn: aws.String(arn), DefaultVersionId: aws.String("v2")}}
	var got resource.Resource
	for range 2 {
		var err error
		if got, err = enrichPolicyEnricher(t)(context.Background(), dctx, row); err != nil {
			t.Fatal(err)
		}
	}
	p, ok := got.RawStruct.(awsclient.PolicyEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T", got.RawStruct)
	}
	if aws.ToString(p.DefaultVersionId) != "v2" || aws.ToString(p.Description) != "from GetPolicy" {
		t.Errorf("DefaultVersionId %q Description %q, want the row's v2 and GetPolicy's description", aws.ToString(p.DefaultVersionId), aws.ToString(p.Description))
	}
	if api.version != "v2" {
		t.Errorf("document read for version %q, want the row's v2", api.version)
	}
	if api.getPolicy != 1 || api.getVersion != 1 {
		t.Errorf("GetPolicy %d, GetPolicyVersion %d over two opens, want 1 and 1", api.getPolicy, api.getVersion)
	}
}

func enrichIAMUserForTest(t *testing.T) resource.DetailEnricher {
	t.Helper()
	e := resource.GetDetailEnricher("iam-user")
	if e == nil {
		t.Fatal("iam-user detail enricher not registered")
	}
	return e
}
