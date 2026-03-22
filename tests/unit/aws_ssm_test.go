package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// SSM - Test FetchSSMParameters response parsing
// ---------------------------------------------------------------------------

func TestFetchSSMParameters_ParsesMultipleParameters(t *testing.T) {
	lastModified := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockSSMDescribeParametersClient{
		output: &ssm.DescribeParametersOutput{
			Parameters: []ssmtypes.ParameterMetadata{
				{
					Name:             aws.String("/prod/database/password"),
					Type:             ssmtypes.ParameterTypeSecureString,
					Version:          3,
					LastModifiedDate: &lastModified,
					Description:      aws.String("Production database password"),
					LastModifiedUser: aws.String("arn:aws:iam::123456789012:user/admin"),
					KeyId:            aws.String("alias/aws/ssm"),
					Tier:             ssmtypes.ParameterTierStandard,
					DataType:         aws.String("text"),
				},
				{
					Name:             aws.String("/staging/api-key"),
					Type:             ssmtypes.ParameterTypeString,
					Version:          1,
					LastModifiedDate: &lastModified,
					Description:      aws.String("Staging API key"),
					Tier:             ssmtypes.ParameterTierStandard,
					DataType:         aws.String("text"),
				},
			},
		},
	}

	resources, err := awsclient.FetchSSMParameters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"name", "type", "version", "last_modified", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first parameter
	r0 := resources[0]
	if r0.ID != "/prod/database/password" {
		t.Errorf("resource[0].ID: expected %q, got %q", "/prod/database/password", r0.ID)
	}
	if r0.Name != "/prod/database/password" {
		t.Errorf("resource[0].Name: expected %q, got %q", "/prod/database/password", r0.Name)
	}
	if r0.Fields["name"] != "/prod/database/password" {
		t.Errorf("resource[0].Fields[\"name\"]: expected %q, got %q", "/prod/database/password", r0.Fields["name"])
	}
	if r0.Fields["type"] != "SecureString" {
		t.Errorf("resource[0].Fields[\"type\"]: expected %q, got %q", "SecureString", r0.Fields["type"])
	}
	if r0.Fields["version"] != "3" {
		t.Errorf("resource[0].Fields[\"version\"]: expected %q, got %q", "3", r0.Fields["version"])
	}
	if r0.Fields["description"] != "Production database password" {
		t.Errorf("resource[0].Fields[\"description\"]: expected %q, got %q", "Production database password", r0.Fields["description"])
	}
	if r0.Fields["last_modified"] == "" {
		t.Error("resource[0].Fields[\"last_modified\"] should not be empty")
	}

	// Verify second parameter
	r1 := resources[1]
	if r1.ID != "/staging/api-key" {
		t.Errorf("resource[1].ID: expected %q, got %q", "/staging/api-key", r1.ID)
	}
	if r1.Fields["type"] != "String" {
		t.Errorf("resource[1].Fields[\"type\"]: expected %q, got %q", "String", r1.Fields["type"])
	}
	if r1.Fields["version"] != "1" {
		t.Errorf("resource[1].Fields[\"version\"]: expected %q, got %q", "1", r1.Fields["version"])
	}
}

func TestFetchSSMParameters_ErrorResponse(t *testing.T) {
	mock := &mockSSMDescribeParametersClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchSSMParameters(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchSSMParameters_EmptyResponse(t *testing.T) {
	mock := &mockSSMDescribeParametersClient{
		output: &ssm.DescribeParametersOutput{
			Parameters: []ssmtypes.ParameterMetadata{},
		},
	}

	resources, err := awsclient.FetchSSMParameters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
