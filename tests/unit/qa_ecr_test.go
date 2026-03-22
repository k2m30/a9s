package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// ECR Repository fetcher tests
// ---------------------------------------------------------------------------

func TestFetchECRRepositories_ParsesMultiple(t *testing.T) {
	now := time.Now()
	mock := &mockECRClient{
		output: &ecr.DescribeRepositoriesOutput{
			Repositories: []ecrtypes.Repository{
				{
					RepositoryName: aws.String("my-app"),
					RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app"),
					RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/my-app"),
					RegistryId:     aws.String("123456789012"),
					CreatedAt:      &now,
					ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
					ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
						ScanOnPush: true,
					},
				},
				{
					RepositoryName: aws.String("nginx-proxy"),
					RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/nginx-proxy"),
					RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/nginx-proxy"),
					RegistryId:     aws.String("123456789012"),
					CreatedAt:      &now,
					ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
				},
			},
		},
	}

	resources, err := awsclient.FetchECRRepositories(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first repo
	r0 := resources[0]
	if r0.ID != "my-app" {
		t.Errorf("resource[0].ID: expected %q, got %q", "my-app", r0.ID)
	}
	if r0.Name != "my-app" {
		t.Errorf("resource[0].Name: expected %q, got %q", "my-app", r0.Name)
	}

	// Verify required fields
	requiredFields := []string{"repository_name", "uri", "tag_mutability", "scan_on_push", "created_at"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	if r0.Fields["repository_name"] != "my-app" {
		t.Errorf("resource[0].Fields[\"repository_name\"]: expected %q, got %q", "my-app", r0.Fields["repository_name"])
	}
	if r0.Fields["uri"] != "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app" {
		t.Errorf("resource[0].Fields[\"uri\"]: expected URI, got %q", r0.Fields["uri"])
	}
	if r0.Fields["tag_mutability"] != "MUTABLE" {
		t.Errorf("resource[0].Fields[\"tag_mutability\"]: expected %q, got %q", "MUTABLE", r0.Fields["tag_mutability"])
	}
	if r0.Fields["scan_on_push"] != "true" {
		t.Errorf("resource[0].Fields[\"scan_on_push\"]: expected %q, got %q", "true", r0.Fields["scan_on_push"])
	}

	// Verify second repo (immutable, no scan config)
	r1 := resources[1]
	if r1.Fields["tag_mutability"] != "IMMUTABLE" {
		t.Errorf("resource[1].Fields[\"tag_mutability\"]: expected %q, got %q", "IMMUTABLE", r1.Fields["tag_mutability"])
	}
	if r1.Fields["scan_on_push"] != "false" {
		t.Errorf("resource[1].Fields[\"scan_on_push\"]: expected %q, got %q", "false", r1.Fields["scan_on_push"])
	}
}

func TestFetchECRRepositories_RawStructPopulated(t *testing.T) {
	now := time.Now()
	mock := &mockECRClient{
		output: &ecr.DescribeRepositoriesOutput{
			Repositories: []ecrtypes.Repository{
				{
					RepositoryName:     aws.String("raw-repo"),
					RepositoryUri:      aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/raw-repo"),
					CreatedAt:          &now,
					ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
				},
			},
		},
	}

	resources, err := awsclient.FetchECRRepositories(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	repo, ok := r.RawStruct.(ecrtypes.Repository)
	if !ok {
		t.Fatalf("RawStruct should be ecrtypes.Repository, got %T", r.RawStruct)
	}
	if repo.RepositoryName == nil || *repo.RepositoryName != "raw-repo" {
		t.Errorf("RawStruct.RepositoryName: expected %q", "raw-repo")
	}
}

func TestFetchECRRepositories_ErrorResponse(t *testing.T) {
	mock := &mockECRClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchECRRepositories(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

func TestFetchECRRepositories_EmptyResponse(t *testing.T) {
	mock := &mockECRClient{
		output: &ecr.DescribeRepositoriesOutput{
			Repositories: []ecrtypes.Repository{},
		},
	}

	resources, err := awsclient.FetchECRRepositories(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
