package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Local mock: IAMGetGroupAPI
// ---------------------------------------------------------------------------

type mockIAMGetGroupClient struct {
	outputs []*iam.GetGroupOutput
	err     error
	callIdx int
}

func (m *mockIAMGetGroupClient) GetGroup(
	ctx context.Context,
	params *iam.GetGroupInput,
	optFns ...func(*iam.Options),
) (*iam.GetGroupOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &iam.GetGroupOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestFetchIAMGroupMembers_Basic verifies parsing of 2 users with all fields.
func TestFetchIAMGroupMembers_Basic(t *testing.T) {
	createDate1 := time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC)
	createDate2 := time.Date(2024, 6, 1, 14, 0, 0, 0, time.UTC)

	mock := &mockIAMGetGroupClient{
		outputs: []*iam.GetGroupOutput{
			{
				Group: &iamtypes.Group{
					GroupName: aws.String("developers"),
				},
				Users: []iamtypes.User{
					{
						UserName:         aws.String("alice"),
						UserId:           aws.String("AIDAEXAMPLE1111111111"),
						Arn:              aws.String("arn:aws:iam::123456789012:user/alice"),
						Path:             aws.String("/"),
						CreateDate:       &createDate1,
						PasswordLastUsed: nil, // GetGroup always returns nil for this
					},
					{
						UserName:         aws.String("bob"),
						UserId:           aws.String("AIDAEXAMPLE2222222222"),
						Arn:              aws.String("arn:aws:iam::123456789012:user/bob"),
						Path:             aws.String("/devops/"),
						CreateDate:       &createDate2,
						PasswordLastUsed: nil,
					},
				},
				IsTruncated: false,
			},
		},
	}

	parentCtx := map[string]string{"group_name": "developers"}

	resources, err := awsclient.FetchIAMGroupMembers(
		context.Background(),
		mock,
		parentCtx,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r0 := resources[0]
	t.Run("user_name", func(t *testing.T) {
		if r0.Fields["user_name"] != "alice" {
			t.Errorf("Fields[user_name]: expected %q, got %q", "alice", r0.Fields["user_name"])
		}
	})
	t.Run("user_id", func(t *testing.T) {
		if r0.Fields["user_id"] != "AIDAEXAMPLE1111111111" {
			t.Errorf("Fields[user_id]: expected %q, got %q", "AIDAEXAMPLE1111111111", r0.Fields["user_id"])
		}
	})
	t.Run("ID_is_user_name", func(t *testing.T) {
		if r0.ID != "alice" {
			t.Errorf("ID: expected %q, got %q", "alice", r0.ID)
		}
	})
	t.Run("Name_is_user_name", func(t *testing.T) {
		if r0.Name != "alice" {
			t.Errorf("Name: expected %q, got %q", "alice", r0.Name)
		}
	})
	t.Run("Status_is_empty", func(t *testing.T) {
		if r0.Status != "" {
			t.Errorf("Status: expected empty, got %q", r0.Status)
		}
	})

	t.Run("required_fields_present_on_all_rows", func(t *testing.T) {
		requiredFields := []string{"user_name", "user_id", "create_date", "password_last_used"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("Row %d Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchIAMGroupMembers_Empty verifies that a group with no users
// returns an empty slice with no error.
func TestFetchIAMGroupMembers_Empty(t *testing.T) {
	mock := &mockIAMGetGroupClient{
		outputs: []*iam.GetGroupOutput{
			{
				Group: &iamtypes.Group{GroupName: aws.String("empty-group")},
				Users: []iamtypes.User{},
			},
		},
	}

	parentCtx := map[string]string{"group_name": "empty-group"}
	resources, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchIAMGroupMembers_APIError verifies that API errors are propagated.
func TestFetchIAMGroupMembers_APIError(t *testing.T) {
	mock := &mockIAMGetGroupClient{
		err: fmt.Errorf("AWS API error: group not found"),
	}

	parentCtx := map[string]string{"group_name": "bad-group"}
	_, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// TestFetchIAMGroupMembers_NilFields verifies that nil fields on iamtypes.User
// do not cause a panic.
func TestFetchIAMGroupMembers_NilFields(t *testing.T) {
	mock := &mockIAMGetGroupClient{
		outputs: []*iam.GetGroupOutput{
			{
				Group: &iamtypes.Group{GroupName: aws.String("nil-group")},
				Users: []iamtypes.User{
					{
						// All pointer fields nil
					},
				},
				IsTruncated: false,
			},
		},
	}

	parentCtx := map[string]string{"group_name": "nil-group"}
	resources, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
}

// TestFetchIAMGroupMembers_Pagination verifies that the fetcher handles
// Marker/IsTruncated pagination from GetGroup.
func TestFetchIAMGroupMembers_Pagination(t *testing.T) {
	createDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	mock := &mockIAMGetGroupClient{
		outputs: []*iam.GetGroupOutput{
			{
				Group: &iamtypes.Group{GroupName: aws.String("big-group")},
				Users: []iamtypes.User{
					{UserName: aws.String("user1"), UserId: aws.String("UID1"), Arn: aws.String("arn1"), Path: aws.String("/"), CreateDate: &createDate},
				},
				IsTruncated: true,
				Marker:      aws.String("marker1"),
			},
			{
				Group: &iamtypes.Group{GroupName: aws.String("big-group")},
				Users: []iamtypes.User{
					{UserName: aws.String("user2"), UserId: aws.String("UID2"), Arn: aws.String("arn2"), Path: aws.String("/"), CreateDate: &createDate},
					{UserName: aws.String("user3"), UserId: aws.String("UID3"), Arn: aws.String("arn3"), Path: aws.String("/"), CreateDate: &createDate},
				},
				IsTruncated: false,
			},
		},
	}

	parentCtx := map[string]string{"group_name": "big-group"}
	resources, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
	}
}

// TestFetchIAMGroupMembers_DateFormat verifies that CreateDate is formatted
// as "2006-01-02 15:04".
func TestFetchIAMGroupMembers_DateFormat(t *testing.T) {
	createDate := time.Date(2024, 3, 15, 9, 30, 45, 0, time.UTC)

	mock := &mockIAMGetGroupClient{
		outputs: []*iam.GetGroupOutput{
			{
				Group: &iamtypes.Group{GroupName: aws.String("date-group")},
				Users: []iamtypes.User{
					{
						UserName:   aws.String("date-user"),
						UserId:     aws.String("UID1"),
						Arn:        aws.String("arn1"),
						Path:       aws.String("/"),
						CreateDate: &createDate,
					},
				},
				IsTruncated: false,
			},
		},
	}

	parentCtx := map[string]string{"group_name": "date-group"}
	resources, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	expected := "2024-03-15 09:30"
	if resources[0].Fields["create_date"] != expected {
		t.Errorf("Fields[create_date]: expected %q, got %q", expected, resources[0].Fields["create_date"])
	}
}

// TestFetchIAMGroupMembers_PasswordLastUsedAlwaysNA verifies that the
// PasswordLastUsed field is always "N/A (not in API)" because GetGroup
// does not return this field.
func TestFetchIAMGroupMembers_PasswordLastUsedAlwaysNA(t *testing.T) {
	createDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	pwLastUsed := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	mock := &mockIAMGetGroupClient{
		outputs: []*iam.GetGroupOutput{
			{
				Group: &iamtypes.Group{GroupName: aws.String("pw-group")},
				Users: []iamtypes.User{
					{
						UserName:         aws.String("user-with-pw"),
						UserId:           aws.String("UID1"),
						Arn:              aws.String("arn1"),
						Path:             aws.String("/"),
						CreateDate:       &createDate,
						PasswordLastUsed: &pwLastUsed, // even if SDK returns a value, we override
					},
				},
				IsTruncated: false,
			},
		},
	}

	parentCtx := map[string]string{"group_name": "pw-group"}
	resources, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	expected := "N/A (not in API)"
	if resources[0].Fields["password_last_used"] != expected {
		t.Errorf("Fields[password_last_used]: expected %q, got %q", expected, resources[0].Fields["password_last_used"])
	}
}

// TestFetchIAMGroupMembers_RawStruct verifies that RawStruct is the
// original iamtypes.User.
func TestFetchIAMGroupMembers_RawStruct(t *testing.T) {
	createDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	mock := &mockIAMGetGroupClient{
		outputs: []*iam.GetGroupOutput{
			{
				Group: &iamtypes.Group{GroupName: aws.String("raw-group")},
				Users: []iamtypes.User{
					{
						UserName:   aws.String("raw-user"),
						UserId:     aws.String("AIDAEXAMPLE"),
						Arn:        aws.String("arn:aws:iam::123456789012:user/raw-user"),
						Path:       aws.String("/"),
						CreateDate: &createDate,
					},
				},
				IsTruncated: false,
			},
		},
	}

	parentCtx := map[string]string{"group_name": "raw-group"}
	resources, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	user, ok := r.RawStruct.(iamtypes.User)
	if !ok {
		t.Fatalf("RawStruct should be iamtypes.User, got %T", r.RawStruct)
	}
	if user.UserName == nil || *user.UserName != "raw-user" {
		t.Errorf("RawStruct.UserName: expected %q, got %v", "raw-user", user.UserName)
	}
}

// TestIAMGroupMemberColumns verifies the column count, keys, titles, and widths.
func TestIAMGroupMemberColumns(t *testing.T) {
	cols := resource.IAMGroupMemberColumns()

	if len(cols) != 4 {
		t.Fatalf("IAMGroupMemberColumns() returned %d columns, expected 4", len(cols))
	}

	wantCols := []struct {
		key   string
		title string
		width int
	}{
		{"user_name", "User Name", 28},
		{"user_id", "User ID", 24},
		{"create_date", "Created", 22},
		{"password_last_used", "Password Last Used", 22},
	}

	for i, want := range wantCols {
		if i >= len(cols) {
			t.Errorf("Missing column at index %d", i)
			continue
		}
		if cols[i].Key != want.key {
			t.Errorf("Column %d Key: expected %q, got %q", i, want.key, cols[i].Key)
		}
		if cols[i].Title != want.title {
			t.Errorf("Column %d Title: expected %q, got %q", i, want.title, cols[i].Title)
		}
		if cols[i].Width != want.width {
			t.Errorf("Column %d Width: expected %d, got %d", i, want.width, cols[i].Width)
		}
	}
}

// TestIAMGroupMembers_ChildFetcherRegistered verifies that the child fetcher
// is registered under the correct short name.
func TestIAMGroupMembers_ChildFetcherRegistered(t *testing.T) {
	f := resource.GetChildFetcher("iam_group_members")
	if f == nil {
		t.Fatal("iam_group_members child fetcher not registered")
	}
}

// TestIAMGroupMembers_ParentHasChildDef verifies that the iam-group parent
// resource type has a Children entry for iam_group_members.
func TestIAMGroupMembers_ParentHasChildDef(t *testing.T) {
	var groupType *resource.ResourceTypeDef
	for _, rt := range resource.AllResourceTypes() {
		if rt.ShortName == "iam-group" {
			groupType = &rt
			break
		}
	}
	if groupType == nil {
		t.Fatal("iam-group resource type not found")
	}

	found := false
	for _, child := range groupType.Children {
		if child.ChildType == "iam_group_members" {
			found = true
			if child.Key != "enter" {
				t.Errorf("iam_group_members child def Key: expected %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["group_name"] != "ID" {
				t.Errorf("iam_group_members ContextKeys[group_name]: expected %q, got %q",
					"ID", child.ContextKeys["group_name"])
			}
			break
		}
	}
	if !found {
		t.Error("iam-group resource type missing Children entry for iam_group_members")
	}
}
