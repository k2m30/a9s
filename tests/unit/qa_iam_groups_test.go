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

func TestQA_IAMGroups_FetchSuccess(t *testing.T) {
	now := time.Now()
	mock := &mockIAMListGroupsClient{
		output: &iam.ListGroupsOutput{
			Groups: []iamtypes.Group{
				{
					GroupName:  aws.String("admins"),
					GroupId:    aws.String("AGPA123456789"),
					Arn:        aws.String("arn:aws:iam::123456789012:group/admins"),
					Path:       aws.String("/"),
					CreateDate: &now,
				},
				{
					GroupName:  aws.String("developers"),
					GroupId:    aws.String("AGPA987654321"),
					Arn:        aws.String("arn:aws:iam::123456789012:group/teams/developers"),
					Path:       aws.String("/teams/"),
					CreateDate: &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchIAMGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "admins" {
		t.Errorf("expected ID 'admins', got %q", r.ID)
	}
	if r.Name != "admins" {
		t.Errorf("expected Name 'admins', got %q", r.Name)
	}
	if r.Status != "" {
		t.Errorf("expected empty status, got %q", r.Status)
	}
	if r.Fields["group_name"] != "admins" {
		t.Errorf("expected group_name 'admins', got %q", r.Fields["group_name"])
	}
	if r.Fields["group_id"] != "AGPA123456789" {
		t.Errorf("expected group_id 'AGPA123456789', got %q", r.Fields["group_id"])
	}
	if r.Fields["path"] != "/" {
		t.Errorf("expected path '/', got %q", r.Fields["path"])
	}

	r2 := resources[1]
	if r2.Fields["arn"] != "arn:aws:iam::123456789012:group/teams/developers" {
		t.Errorf("expected correct ARN, got %q", r2.Fields["arn"])
	}
	if r2.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_IAMGroups_FetchEmpty(t *testing.T) {
	mock := &mockIAMListGroupsClient{
		output: &iam.ListGroupsOutput{
			Groups: []iamtypes.Group{},
		},
	}

	resources, err := awsclient.FetchIAMGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_IAMGroups_FetchError(t *testing.T) {
	mock := &mockIAMListGroupsClient{
		err: fmt.Errorf("access denied"),
	}

	_, err := awsclient.FetchIAMGroups(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_IAMGroups_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("iam-group")
	if rt == nil {
		t.Fatal("resource type 'iam-group' not found")
	}
	if rt.Name != "IAM Groups" {
		t.Errorf("expected Name 'IAM Groups', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"group_name", "Group Name"},
		{"group_id", "Group ID"},
		{"path", "Path"},
		{"create_date", "Created"},
		{"arn", "ARN"},
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

func TestQA_IAMGroups_Aliases(t *testing.T) {
	for _, alias := range []string{"iam-group", "iam-groups", "groups"} {
		rt := resource.FindResourceType(alias)
		if rt == nil {
			t.Errorf("alias %q should resolve to iam-group resource type", alias)
		}
	}
}
