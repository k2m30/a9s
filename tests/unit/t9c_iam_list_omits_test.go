package unit_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

func t9cMemberSet(raw any, member string) bool {
	v, err := fieldpath.ExtractValue(raw, member)
	if err != nil || !v.IsValid() {
		return false
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		return v.Len() > 0
	}
	return !v.IsZero()
}

func t9cShown(rows []app.FieldRow, path string) bool {
	for _, r := range rows {
		if r.Path != path && !strings.HasPrefix(r.Path, path+".") {
			continue
		}
		if v := strings.TrimSpace(r.Value); v != "" && v != "-" {
			return true
		}
	}
	return false
}

func t9cDetailCtx(clients *awsclient.ServiceClients) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{Clients: clients, PolicyDocs: &awsclient.PolicyDocumentCache{}, DetailDocs: &awsclient.DetailDocCache{}}
}

// t9cEnrichedDetail is the detail screen of row once the type's detail
// enricher answered, the screen an operator reads after opening the row.
func t9cEnrichedDetail(t *testing.T, clients *awsclient.ServiceClients, typ string, row resource.Resource) []app.FieldRow {
	t.Helper()
	td := catalog.FindAny(typ)
	if td == nil {
		t.Fatalf("no catalog type %s", typ)
	}
	if td.DetailEnrich == nil {
		t.Fatalf("%s has no detail enricher, so the detail shows the list call's struct", typ)
	}
	enriched, err := td.DetailEnrich(context.Background(), t9cDetailCtx(clients), row)
	if err != nil && !errors.As(err, new(domain.PartialDetailErr)) {
		t.Fatalf("%s %s detail enricher: %v", typ, row.ID, err)
	}
	return t9bDetailRows(t, enriched, typ)
}

func t9cDrain(t *testing.T, typ string, clients *awsclient.ServiceClients) []resource.Resource {
	t.Helper()
	td := catalog.FindAny(typ)
	if td == nil {
		t.Fatalf("no catalog type %s", typ)
	}
	rows, ok := unit.DrainFixtures(t, *td, clients)
	if !ok || len(rows) == 0 {
		t.Fatalf("%s: the demo list returned no rows", typ)
	}
	return rows
}

func t9cRowByID(rows []resource.Resource, id string) (resource.Resource, bool) {
	for _, r := range rows {
		if r.ID == id {
			return r, true
		}
	}
	return resource.Resource{}, false
}

func t9cWantOmits(t *testing.T, typ string, want []string) {
	t.Helper()
	td := catalog.FindAny(typ)
	if td == nil {
		t.Fatalf("no catalog type %s", typ)
	}
	got := t9cListOmits(*td)
	for _, m := range want {
		if !slices.Contains(got, m) {
			t.Errorf("%s ListOmits %v does not declare %s, which its list call does not return", typ, got, m)
		}
	}
}

// ListPolicies returns neither a policy's Description nor its tags: "This
// element is included in the response to the GetPolicy operation. It is not
// included in the response to the ListPolicies operation" (iam types.Policy
// Description), and "this operation does not return tags" (ListPolicies).
// The demo list answer is AWS's, and the detail shows both once GetPolicy
// answered.
func TestT9c_Row5_PolicyDescriptionAndTagsComeFromGetPolicy(t *testing.T) {
	ctx := context.Background()
	clients := demo.NewServiceClients()
	t9cWantOmits(t, "policy", []string{"Description", "Tags"})

	listed, err := clients.IAM.ListPolicies(ctx, &iam.ListPoliciesInput{Scope: iamtypes.PolicyScopeTypeAll})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range listed.Policies {
		if p.Description != nil || len(p.Tags) > 0 {
			t.Errorf("demo ListPolicies answer carries Description %q / %d tags for %s; AWS returns them only from GetPolicy",
				aws.ToString(p.Description), len(p.Tags), aws.ToString(p.Arn))
		}
	}

	rows := t9cDrain(t, "policy", clients)
	witnessed := map[string]int{}
	for _, p := range listed.Policies {
		got, err := clients.IAM.GetPolicy(ctx, &iam.GetPolicyInput{PolicyArn: p.Arn})
		if err != nil || got.Policy == nil {
			continue
		}
		hasDesc, hasTags := got.Policy.Description != nil, len(got.Policy.Tags) > 0
		if !hasDesc && !hasTags {
			continue
		}
		var row resource.Resource
		found := false
		for _, r := range rows {
			if r.Fields["arn"] == aws.ToString(p.Arn) {
				row, found = r, true
			}
		}
		if !found {
			t.Errorf("no policy list row for %s", aws.ToString(p.Arn))
			continue
		}
		shown := t9cEnrichedDetail(t, clients, "policy", row)
		if hasDesc {
			witnessed["Description"]++
			want := aws.ToString(got.Policy.Description)
			if !slices.Contains(t9bPathRows(shown, "Description"), "Description="+want) {
				t.Errorf("policy %s detail Description rows %v, GetPolicy says %q", row.ID, t9bPathRows(shown, "Description"), want)
			}
		}
		if hasTags {
			witnessed["Tags"]++
			if !t9cShown(shown, "Tags") {
				t.Errorf("policy %s detail shows no Tags; GetPolicy returns %d", row.ID, len(got.Policy.Tags))
			}
		}
	}
	for _, m := range []string{"Description", "Tags"} {
		if witnessed[m] == 0 {
			t.Errorf("no demo policy's GetPolicy answer carries %s, so the bench never shows the detail row filled", m)
		}
	}
}

// ListRoles "does not return the following attributes, even though they are
// an attribute of the returned object: PermissionsBoundary, RoleLastUsed,
// Tags" (iam ListRoles). GetRole returns them, so the detail shows each once
// GetRole answered, and the demo list answer carries none.
func TestT9c_Row6_RoleMembersComeFromGetRole(t *testing.T) {
	ctx := context.Background()
	clients := demo.NewServiceClients()
	members := []string{"PermissionsBoundary", "RoleLastUsed", "Tags"}
	t9cWantOmits(t, "role", members)

	listed, err := clients.IAM.ListRoles(ctx, &iam.ListRolesInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range listed.Roles {
		for _, m := range members {
			if t9cMemberSet(r, m) {
				t.Errorf("demo ListRoles answer carries %s for %s; AWS returns it only from GetRole", m, aws.ToString(r.RoleName))
			}
		}
	}

	rows := t9cDrain(t, "role", clients)
	witnessed := map[string]int{}
	for _, r := range listed.Roles {
		got, err := t9cGetRole(t, clients).GetRole(ctx, &iam.GetRoleInput{RoleName: r.RoleName})
		if err != nil || got.Role == nil {
			continue
		}
		var carries []string
		for _, m := range members {
			if t9cMemberSet(*got.Role, m) {
				carries = append(carries, m)
			}
		}
		if len(carries) == 0 {
			continue
		}
		row, ok := t9cRowByID(rows, aws.ToString(r.RoleName))
		if !ok {
			t.Errorf("no role list row for %s", aws.ToString(r.RoleName))
			continue
		}
		shown := t9cEnrichedDetail(t, clients, "role", row)
		for _, m := range carries {
			witnessed[m]++
			if !t9cShown(shown, m) {
				t.Errorf("role %s detail shows no %s; GetRole returns one", row.ID, m)
			}
		}
	}
	for _, m := range members {
		if witnessed[m] == 0 {
			t.Errorf("no demo role's GetRole answer carries %s, so the bench never shows the detail row filled", m)
		}
	}
}

func t9cGetRole(t *testing.T, clients *awsclient.ServiceClients) awsclient.IAMGetRoleAPI {
	t.Helper()
	api, ok := clients.IAM.(awsclient.IAMGetRoleAPI)
	if !ok {
		t.Fatal("the IAM client answers no GetRole")
	}
	return api
}

type t9cGetUserAPI interface {
	GetUser(ctx context.Context, params *iam.GetUserInput, optFns ...func(*iam.Options)) (*iam.GetUserOutput, error)
}

// ListUsers "does not return the following attributes, even though they are
// an attribute of the returned object: PermissionsBoundary, Tags" (iam
// ListUsers). GetUser returns them, so the detail shows each once GetUser
// answered, and the demo list answer carries neither.
func TestT9c_Row6_UserMembersComeFromGetUser(t *testing.T) {
	ctx := context.Background()
	clients := demo.NewServiceClients()
	members := []string{"PermissionsBoundary", "Tags"}
	t9cWantOmits(t, "iam-user", members)

	listed, err := clients.IAM.ListUsers(ctx, &iam.ListUsersInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range listed.Users {
		for _, m := range members {
			if t9cMemberSet(u, m) {
				t.Errorf("demo ListUsers answer carries %s for %s; AWS returns it only from GetUser", m, aws.ToString(u.UserName))
			}
		}
	}

	getUser, ok := clients.IAM.(t9cGetUserAPI)
	if !ok {
		t.Fatal("the IAM client answers no GetUser, the call that returns a user's PermissionsBoundary and Tags")
	}
	rows := t9cDrain(t, "iam-user", clients)
	witnessed := map[string]int{}
	for _, u := range listed.Users {
		got, err := getUser.GetUser(ctx, &iam.GetUserInput{UserName: u.UserName})
		if err != nil || got.User == nil {
			continue
		}
		var carries []string
		for _, m := range members {
			if t9cMemberSet(*got.User, m) {
				carries = append(carries, m)
			}
		}
		if len(carries) == 0 {
			continue
		}
		row, ok := t9cRowByID(rows, aws.ToString(u.UserName))
		if !ok {
			t.Errorf("no iam-user list row for %s", aws.ToString(u.UserName))
			continue
		}
		shown := t9cEnrichedDetail(t, clients, "iam-user", row)
		for _, m := range carries {
			witnessed[m]++
			if !t9cShown(shown, m) {
				t.Errorf("iam-user %s detail shows no %s; GetUser returns one", row.ID, m)
			}
		}
	}
	for _, m := range members {
		if witnessed[m] == 0 {
			t.Errorf("no demo user's GetUser answer carries %s, so the bench never shows the detail row filled", m)
		}
	}
}

func t9cListCells(t *testing.T, typ string, rows []resource.Resource) map[string]map[string]string {
	t.Helper()
	c := newVisibilityListController(t, typ)
	c.ApplyResourcesLoaded(typ, rows, nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatalf("%s: no list body", typ)
	}
	out := map[string]map[string]string{}
	for _, row := range body.Rows {
		cells := map[string]string{}
		for i, col := range body.Columns {
			if i < len(row.Cells) {
				cells[col.Title] = row.Cells[i]
			}
		}
		out[row.ResourceID] = cells
	}
	return out
}

func t9cAfterWave2(t *testing.T, typ string, clients *awsclient.ServiceClients, rows []resource.Resource, seeds ...resource.ResourceCache) []resource.Resource {
	t.Helper()
	td := catalog.FindAny(typ)
	res := unit.ProbeWave2(t, typ, clients, rows, seeds...)
	out := make([]resource.Resource, len(rows))
	for i, r := range rows {
		if fu := res.FieldUpdates[r.ID]; len(fu) > 0 {
			fields := make(map[string]string, len(r.Fields)+len(fu))
			maps.Copy(fields, r.Fields)
			maps.Copy(fields, fu)
			r.Fields = fields
		}
		runtime.ApplyWave2ToRow(&r, *td, res.Findings, res.AttentionDetails)
		out[i] = r
	}
	return out
}

// The role list's Last Used cell is the RoleLastUsed GetRole returns: the
// list call leaves it out, and the Wave 2 read that already calls GetRole for
// every role writes it.
func TestT9c_Row6_RoleLastUsedColumnIsFilledByWave2(t *testing.T) {
	ctx := context.Background()
	clients := demo.NewServiceClients()
	rows := t9cDrain(t, "role", clients)
	want := map[string]time.Time{}
	for _, r := range rows {
		got, err := t9cGetRole(t, clients).GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(r.ID)})
		if err == nil && got.Role != nil && got.Role.RoleLastUsed != nil && got.Role.RoleLastUsed.LastUsedDate != nil {
			want[r.ID] = *got.Role.RoleLastUsed.LastUsedDate
		}
	}
	if len(want) == 0 {
		t.Fatal("no demo role's GetRole answer carries RoleLastUsed.LastUsedDate")
	}

	before := t9cListCells(t, "role", rows)
	after := t9cListCells(t, "role", t9cAfterWave2(t, "role", clients, rows))
	for id, used := range want {
		if cell := before[id]["Last Used"]; cell != "" {
			t.Errorf("role %s: Last Used reads %q before Wave 2, from a list answer AWS never fills", id, cell)
		}
		cell := after[id]["Last Used"]
		utc, local := used.UTC().Format(time.DateOnly), used.Local().Format(time.DateOnly)
		if !strings.Contains(cell, utc) && !strings.Contains(cell, local) {
			t.Errorf("role %s: Last Used reads %q after Wave 2, GetRole says %s", id, cell, used.UTC().Format(time.RFC3339))
		}
	}
	for id, cells := range after {
		if _, used := want[id]; !used && cells["Last Used"] != "" {
			t.Errorf("role %s: Last Used reads %q, GetRole returns no RoleLastUsed", id, cells["Last Used"])
		}
	}
}

// A type that declares members its list call does not return gets list rows
// without them on the demo bench, as AWS answers.
func TestT9c_Row5_DemoListRowsCarryNoDeclaredOmission(t *testing.T) {
	stored, enrichedIdx, _ := t9bDevTreeRowsEnriched(t)
	declaring := 0
	var problems []string
	for _, td := range t9cAllDefs() {
		omits := t9cListOmits(td)
		if len(omits) == 0 {
			continue
		}
		declaring++
		for i, r := range stored {
			if enrichedIdx[i] || r.typ != td.ShortName || r.res.RawStruct == nil {
				continue
			}
			for _, m := range omits {
				if t9cMemberSet(r.res.RawStruct, m) {
					problems = append(problems, fmt.Sprintf("%s %s: list row carries %s", td.ShortName, r.res.ID, m))
				}
			}
		}
	}
	if declaring < 3 {
		t.Errorf("%d types declare ListOmits; role, iam-user and policy each have members their list call does not return", declaring)
	}
	t9bReport(t, "list rows carrying a member the list call does not return", problems)
}
