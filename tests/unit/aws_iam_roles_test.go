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
)

// ---------------------------------------------------------------------------
// T071 - Test IAM ListRoles response parsing
// ---------------------------------------------------------------------------

func TestFetchIAMRoles_ParsesMultipleRoles(t *testing.T) {
	createDate := time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC)

	mock := &mockIAMListRolesClient{
		output: &iam.ListRolesOutput{
			Roles: []iamtypes.Role{
				{
					RoleName:           aws.String("prod-app-role"),
					RoleId:             aws.String("AROABC1234567890PROD"),
					Path:               aws.String("/"),
					CreateDate:         &createDate,
					Description:        aws.String("Production application role"),
					Arn:                aws.String("arn:aws:iam::123456789012:role/prod-app-role"),
					MaxSessionDuration: aws.Int32(3600),
				},
				{
					RoleName:           aws.String("staging-deploy-role"),
					RoleId:             aws.String("AROABC1234567890STAG"),
					Path:               aws.String("/service-roles/"),
					CreateDate:         &createDate,
					Description:        aws.String("Staging deployment role"),
					Arn:                aws.String("arn:aws:iam::123456789012:role/service-roles/staging-deploy-role"),
					MaxSessionDuration: aws.Int32(7200),
				},
			},
		},
	}

	resources, err := awsclient.FetchIAMRoles(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"role_name", "role_id", "path", "create_date", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first role
	r0 := resources[0]
	if r0.ID != "prod-app-role" {
		t.Errorf("resource[0].ID: expected %q, got %q", "prod-app-role", r0.ID)
	}
	if r0.Name != "prod-app-role" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod-app-role", r0.Name)
	}
	if r0.Fields["role_name"] != "prod-app-role" {
		t.Errorf("resource[0].Fields[\"role_name\"]: expected %q, got %q", "prod-app-role", r0.Fields["role_name"])
	}
	if r0.Fields["role_id"] != "AROABC1234567890PROD" {
		t.Errorf("resource[0].Fields[\"role_id\"]: expected %q, got %q", "AROABC1234567890PROD", r0.Fields["role_id"])
	}
	if r0.Fields["path"] != "/" {
		t.Errorf("resource[0].Fields[\"path\"]: expected %q, got %q", "/", r0.Fields["path"])
	}
	if r0.Fields["description"] != "Production application role" {
		t.Errorf("resource[0].Fields[\"description\"]: expected %q, got %q", "Production application role", r0.Fields["description"])
	}
	if r0.Fields["create_date"] == "" {
		t.Error("resource[0].Fields[\"create_date\"] should not be empty")
	}

	// Verify second role
	r1 := resources[1]
	if r1.ID != "staging-deploy-role" {
		t.Errorf("resource[1].ID: expected %q, got %q", "staging-deploy-role", r1.ID)
	}
	if r1.Fields["role_id"] != "AROABC1234567890STAG" {
		t.Errorf("resource[1].Fields[\"role_id\"]: expected %q, got %q", "AROABC1234567890STAG", r1.Fields["role_id"])
	}
	if r1.Fields["path"] != "/service-roles/" {
		t.Errorf("resource[1].Fields[\"path\"]: expected %q, got %q", "/service-roles/", r1.Fields["path"])
	}
}

func TestFetchIAMRoles_ErrorResponse(t *testing.T) {
	mock := &mockIAMListRolesClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchIAMRoles(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchIAMRoles_EmptyResponse(t *testing.T) {
	mock := &mockIAMListRolesClient{
		output: &iam.ListRolesOutput{
			Roles: []iamtypes.Role{},
		},
	}

	resources, err := awsclient.FetchIAMRoles(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
