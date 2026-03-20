package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T-CA-001 - Test CodeArtifact Repositories response parsing
// ---------------------------------------------------------------------------

func TestFetchCodeArtifactRepos_ParsesMultipleRepos(t *testing.T) {
	now := time.Now()
	mock := &mockCodeArtifactClient{
		output: &codeartifact.ListRepositoriesOutput{
			Repositories: []codeartifacttypes.RepositorySummary{
				{
					Name:                 aws.String("my-repo"),
					DomainName:           aws.String("my-domain"),
					DomainOwner:          aws.String("123456789012"),
					Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/my-domain/my-repo"),
					Description:          aws.String("Main repo"),
					AdministratorAccount: aws.String("123456789012"),
					CreatedTime:          &now,
				},
				{
					Name:        aws.String("shared-repo"),
					DomainName:  aws.String("shared-domain"),
					DomainOwner: aws.String("987654321098"),
					Arn:         aws.String("arn:aws:codeartifact:us-east-1:987654321098:repository/shared-domain/shared-repo"),
				},
			},
		},
	}

	resources, err := awsclient.FetchCodeArtifactRepos(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "my-repo" {
		t.Errorf("expected Name 'my-repo', got %q", r.Name)
	}
	if r.ID != "my-repo" {
		t.Errorf("expected ID 'my-repo', got %q", r.ID)
	}
	if r.Fields["repo_name"] != "my-repo" {
		t.Errorf("expected Fields[repo_name] 'my-repo', got %q", r.Fields["repo_name"])
	}
	if r.Fields["domain_name"] != "my-domain" {
		t.Errorf("expected Fields[domain_name] 'my-domain', got %q", r.Fields["domain_name"])
	}
	if r.Fields["domain_owner"] != "123456789012" {
		t.Errorf("expected Fields[domain_owner] '123456789012', got %q", r.Fields["domain_owner"])
	}
	if r.Fields["description"] != "Main repo" {
		t.Errorf("expected Fields[description] 'Main repo', got %q", r.Fields["description"])
	}

	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}

	// Second repo
	r2 := resources[1]
	if r2.Fields["domain_name"] != "shared-domain" {
		t.Errorf("expected Fields[domain_name] 'shared-domain', got %q", r2.Fields["domain_name"])
	}
}

func TestFetchCodeArtifactRepos_EmptyResponse(t *testing.T) {
	mock := &mockCodeArtifactClient{
		output: &codeartifact.ListRepositoriesOutput{
			Repositories: []codeartifacttypes.RepositorySummary{},
		},
	}

	resources, err := awsclient.FetchCodeArtifactRepos(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchCodeArtifactRepos_APIError(t *testing.T) {
	mock := &mockCodeArtifactClient{
		err: &mockAPIError{code: "AccessDeniedException", message: "access denied"},
	}

	_, err := awsclient.FetchCodeArtifactRepos(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchCodeArtifactRepos_NilFields(t *testing.T) {
	mock := &mockCodeArtifactClient{
		output: &codeartifact.ListRepositoriesOutput{
			Repositories: []codeartifacttypes.RepositorySummary{
				{},
			},
		},
	}

	resources, err := awsclient.FetchCodeArtifactRepos(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "" {
		t.Errorf("expected empty Name, got %q", r.Name)
	}
}
