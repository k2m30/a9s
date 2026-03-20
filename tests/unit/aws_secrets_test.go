package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T060 - Test SecretsManager ListSecrets response parsing
// ---------------------------------------------------------------------------

func TestFetchSecrets_ParsesMultipleSecrets(t *testing.T) {
	lastAccessed := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	lastChanged := time.Date(2025, 2, 20, 14, 30, 0, 0, time.UTC)

	mock := &mockSecretsManagerClient{
		output: &secretsmanager.ListSecretsOutput{
			SecretList: []smtypes.SecretListEntry{
				{
					Name:              aws.String("prod/database/password"),
					Description:       aws.String("Production database password"),
					LastAccessedDate:  &lastAccessed,
					LastChangedDate:   &lastChanged,
					RotationEnabled:   aws.Bool(true),
				},
				{
					Name:             aws.String("staging/api-key"),
					Description:      aws.String("Staging API key"),
					LastAccessedDate: &lastAccessed,
					LastChangedDate:  &lastChanged,
					RotationEnabled:  aws.Bool(false),
				},
			},
		},
	}

	resources, err := awsclient.FetchSecrets(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"secret_name", "description", "last_accessed", "last_changed", "rotation_enabled"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first secret
	r0 := resources[0]
	if r0.ID != "prod/database/password" {
		t.Errorf("resource[0].ID: expected %q, got %q", "prod/database/password", r0.ID)
	}
	if r0.Name != "prod/database/password" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod/database/password", r0.Name)
	}
	if r0.Fields["secret_name"] != "prod/database/password" {
		t.Errorf("resource[0].Fields[\"secret_name\"]: expected %q, got %q", "prod/database/password", r0.Fields["secret_name"])
	}
	if r0.Fields["description"] != "Production database password" {
		t.Errorf("resource[0].Fields[\"description\"]: expected %q, got %q", "Production database password", r0.Fields["description"])
	}
	if r0.Fields["rotation_enabled"] != "Yes" {
		t.Errorf("resource[0].Fields[\"rotation_enabled\"]: expected %q, got %q", "Yes", r0.Fields["rotation_enabled"])
	}

	// Verify dates are formatted correctly
	if r0.Fields["last_accessed"] != "2025-03-10" {
		t.Errorf("resource[0].Fields[\"last_accessed\"] = %q, want %q", r0.Fields["last_accessed"], "2025-03-10")
	}
	if r0.Fields["last_changed"] != "2025-02-20" {
		t.Errorf("resource[0].Fields[\"last_changed\"] = %q, want %q", r0.Fields["last_changed"], "2025-02-20")
	}

	// Verify second secret
	r1 := resources[1]
	if r1.ID != "staging/api-key" {
		t.Errorf("resource[1].ID: expected %q, got %q", "staging/api-key", r1.ID)
	}
	if r1.Fields["rotation_enabled"] != "No" {
		t.Errorf("resource[1].Fields[\"rotation_enabled\"]: expected %q, got %q", "No", r1.Fields["rotation_enabled"])
	}
}

func TestFetchSecrets_DetailDataPopulated(t *testing.T) {
	lastAccessed := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	lastChanged := time.Date(2025, 2, 20, 14, 30, 0, 0, time.UTC)

	mock := &mockSecretsManagerClient{
		output: &secretsmanager.ListSecretsOutput{
			SecretList: []smtypes.SecretListEntry{
				{
					Name:             aws.String("prod/db/pass"),
					Description:      aws.String("Prod DB password"),
					ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/pass-AbCdEf"),
					LastAccessedDate: &lastAccessed,
					LastChangedDate:  &lastChanged,
					RotationEnabled:  aws.Bool(true),
				},
			},
		},
	}

	resources, err := awsclient.FetchSecrets(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.DetailData == nil {
		t.Fatal("DetailData must not be nil")
	}
	if len(r.DetailData) == 0 {
		t.Fatal("DetailData must not be empty")
	}
	if r.DetailData["Name"] != "prod/db/pass" {
		t.Errorf("DetailData[Name] = %q, want %q", r.DetailData["Name"], "prod/db/pass")
	}
	if r.DetailData["Description"] != "Prod DB password" {
		t.Errorf("DetailData[Description] = %q, want %q", r.DetailData["Description"], "Prod DB password")
	}
	if r.DetailData["ARN"] != "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/pass-AbCdEf" {
		t.Errorf("DetailData[ARN] = %q, want %q", r.DetailData["ARN"], "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/pass-AbCdEf")
	}
}

func TestFetchSecrets_ErrorResponse(t *testing.T) {
	mock := &mockSecretsManagerClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchSecrets(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchSecrets_EmptyResponse(t *testing.T) {
	mock := &mockSecretsManagerClient{
		output: &secretsmanager.ListSecretsOutput{
			SecretList: []smtypes.SecretListEntry{},
		},
	}

	resources, err := awsclient.FetchSecrets(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T043 - Test GetSecretValue (RevealSecret)
// ---------------------------------------------------------------------------

func TestRevealSecret_ReturnsSecretString(t *testing.T) {
	mock := &mockSecretsManagerGetSecretValueClient{
		output: &secretsmanager.GetSecretValueOutput{
			SecretString: aws.String(`{"username":"admin","password":"s3cret!"}`),
		},
	}

	secret, err := awsclient.RevealSecret(context.Background(), mock, "prod/database/password")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := `{"username":"admin","password":"s3cret!"}`
	if secret != expected {
		t.Errorf("expected secret %q, got %q", expected, secret)
	}
}

func TestRevealSecret_ErrorResponse(t *testing.T) {
	mock := &mockSecretsManagerGetSecretValueClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	secret, err := awsclient.RevealSecret(context.Background(), mock, "prod/database/password")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if secret != "" {
		t.Errorf("expected empty secret on error, got %q", secret)
	}
}
