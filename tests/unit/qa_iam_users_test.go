package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
)

func TestQA_IAMUsers_FetchSuccess(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-24 * time.Hour)
	mock := &mockIAMListUsersClient{
		output: &iam.ListUsersOutput{
			Users: []iamtypes.User{
				{
					UserName:         aws.String("alice"),
					UserId:           aws.String("AIDA123456789"),
					Arn:              aws.String("arn:aws:iam::123456789012:user/alice"),
					Path:             aws.String("/"),
					CreateDate:       &now,
					PasswordLastUsed: &lastUsed,
				},
				{
					UserName:   aws.String("bob"),
					UserId:     aws.String("AIDA987654321"),
					Arn:        aws.String("arn:aws:iam::123456789012:user/developers/bob"),
					Path:       aws.String("/developers/"),
					CreateDate: &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchIAMUsers(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first user
	r := resources[0]
	if r.ID != "alice" {
		t.Errorf("expected ID 'alice', got %q", r.ID)
	}
	if r.Name != "alice" {
		t.Errorf("expected Name 'alice', got %q", r.Name)
	}
	if r.Status != "" {
		t.Errorf("expected empty status, got %q", r.Status)
	}
	if r.Fields["user_name"] != "alice" {
		t.Errorf("expected user_name 'alice', got %q", r.Fields["user_name"])
	}
	if r.Fields["user_id"] != "AIDA123456789" {
		t.Errorf("expected user_id 'AIDA123456789', got %q", r.Fields["user_id"])
	}
	if r.Fields["path"] != "/" {
		t.Errorf("expected path '/', got %q", r.Fields["path"])
	}
	if r.Fields["password_last_used"] == "Never" {
		t.Errorf("password_last_used should not be 'Never' for alice")
	}

	// Verify second user has "Never" for password_last_used
	r2 := resources[1]
	if r2.Fields["password_last_used"] != "Never" {
		t.Errorf("expected password_last_used 'Never' for bob, got %q", r2.Fields["password_last_used"])
	}
	if r2.Fields["path"] != "/developers/" {
		t.Errorf("expected path '/developers/', got %q", r2.Fields["path"])
	}

	// Verify RawStruct is set
	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_IAMUsers_FetchEmpty(t *testing.T) {
	mock := &mockIAMListUsersClient{
		output: &iam.ListUsersOutput{
			Users: []iamtypes.User{},
		},
	}

	resources, err := awsclient.FetchIAMUsers(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_IAMUsers_FetchError(t *testing.T) {
	mock := &mockIAMListUsersClient{
		err: fmt.Errorf("access denied"),
	}

	_, err := awsclient.FetchIAMUsers(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_IAMUsers_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("iam-user")
	if rt == nil {
		t.Fatal("resource type 'iam-user' not found")
	}
	if rt.Name != "IAM Users" {
		t.Errorf("expected Name 'IAM Users', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"user_name", "User Name"},
		{"user_id", "User ID"},
		{"path", "Path"},
		{"create_date", "Created"},
		{"password_last_used", "Password Last Used"},
	}
	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}
	for i, want := range expected {
		if rt.Columns[i].Key != want.key {
			t.Errorf("column %d: expected key %q, got %q", i, want.key, rt.Columns[i].Key)
		}
		if rt.Columns[i].Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, rt.Columns[i].Title)
		}
	}
}

func TestQA_IAMUsers_Aliases(t *testing.T) {
	for _, alias := range []string{"iam-user", "iam-users", "users"} {
		rt := resource.FindResourceType(alias)
		if rt == nil {
			t.Errorf("alias %q should resolve to iam-user resource type", alias)
		}
	}
}
