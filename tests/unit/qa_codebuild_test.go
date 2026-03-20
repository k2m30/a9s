package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
)

// ---------------------------------------------------------------------------
// T-CB01 - Test CodeBuild two-step fetch (ListProjects -> BatchGetProjects)
// ---------------------------------------------------------------------------

func TestFetchCodeBuildProjects_ParsesMultipleProjects(t *testing.T) {
	listMock := &mockCodeBuildListProjectsClient{
		output: &codebuild.ListProjectsOutput{
			Projects: []string{"project-alpha", "project-beta"},
		},
	}

	batchMock := &mockCodeBuildBatchGetProjectsClient{
		output: &codebuild.BatchGetProjectsOutput{
			Projects: []cbtypes.Project{
				{
					Name:        aws.String("project-alpha"),
					Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/project-alpha"),
					Description: aws.String("Alpha build project"),
					Source: &cbtypes.ProjectSource{
						Type: cbtypes.SourceTypeCodecommit,
					},
					ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-role"),
				},
				{
					Name:        aws.String("project-beta"),
					Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/project-beta"),
					Description: aws.String("Beta build project"),
					Source: &cbtypes.ProjectSource{
						Type: cbtypes.SourceTypeGithub,
					},
					ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-role"),
				},
			},
		},
	}

	resources, err := awsclient.FetchCodeBuildProjects(context.Background(), listMock, batchMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields
	requiredFields := []string{"name", "source_type", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first project
	r0 := resources[0]
	if r0.ID != "project-alpha" {
		t.Errorf("resource[0].ID: expected %q, got %q", "project-alpha", r0.ID)
	}
	if r0.Name != "project-alpha" {
		t.Errorf("resource[0].Name: expected %q, got %q", "project-alpha", r0.Name)
	}
	if r0.Fields["name"] != "project-alpha" {
		t.Errorf("resource[0].Fields[\"name\"]: expected %q, got %q", "project-alpha", r0.Fields["name"])
	}
	if r0.Fields["source_type"] != "CODECOMMIT" {
		t.Errorf("resource[0].Fields[\"source_type\"]: expected %q, got %q", "CODECOMMIT", r0.Fields["source_type"])
	}
	if r0.Fields["description"] != "Alpha build project" {
		t.Errorf("resource[0].Fields[\"description\"]: expected %q, got %q", "Alpha build project", r0.Fields["description"])
	}

	// Verify second project
	r1 := resources[1]
	if r1.ID != "project-beta" {
		t.Errorf("resource[1].ID: expected %q, got %q", "project-beta", r1.ID)
	}
	if r1.Fields["source_type"] != "GITHUB" {
		t.Errorf("resource[1].Fields[\"source_type\"]: expected %q, got %q", "GITHUB", r1.Fields["source_type"])
	}

	// Verify RawStruct is set
	if r0.RawStruct == nil {
		t.Error("resource[0].RawStruct should not be nil")
	}

	// Verify RawJSON is non-empty
	if r0.RawJSON == "" {
		t.Error("resource[0].RawJSON should not be empty")
	}
}

func TestFetchCodeBuildProjects_ListError(t *testing.T) {
	listMock := &mockCodeBuildListProjectsClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}
	batchMock := &mockCodeBuildBatchGetProjectsClient{}

	resources, err := awsclient.FetchCodeBuildProjects(context.Background(), listMock, batchMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchCodeBuildProjects_EmptyResponse(t *testing.T) {
	listMock := &mockCodeBuildListProjectsClient{
		output: &codebuild.ListProjectsOutput{
			Projects: []string{},
		},
	}
	batchMock := &mockCodeBuildBatchGetProjectsClient{}

	resources, err := awsclient.FetchCodeBuildProjects(context.Background(), listMock, batchMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchCodeBuildProjects_BatchGetError(t *testing.T) {
	listMock := &mockCodeBuildListProjectsClient{
		output: &codebuild.ListProjectsOutput{
			Projects: []string{"proj-1"},
		},
	}
	batchMock := &mockCodeBuildBatchGetProjectsClient{
		err: fmt.Errorf("batch get failed"),
	}

	resources, err := awsclient.FetchCodeBuildProjects(context.Background(), listMock, batchMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T-CB02 - Resource type definition
// ---------------------------------------------------------------------------

func TestCodeBuild_ResourceTypeDef(t *testing.T) {
	rt := resource.FindResourceType("cb")
	if rt == nil {
		t.Fatal("resource type 'cb' not found")
	}

	if rt.Name != "CodeBuild Projects" {
		t.Errorf("expected name %q, got %q", "CodeBuild Projects", rt.Name)
	}

	expected := []struct {
		title string
		key   string
		width int
	}{
		{"Project Name", "name", 32},
		{"Source Type", "source_type", 14},
		{"Description", "description", 36},
		{"Last Modified", "last_modified", 22},
	}

	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}

	for i, want := range expected {
		col := rt.Columns[i]
		if col.Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, col.Title)
		}
		if col.Key != want.key {
			t.Errorf("column %d (%s): expected key %q, got %q", i, want.title, want.key, col.Key)
		}
		if col.Width != want.width {
			t.Errorf("column %d (%s): expected width %d, got %d", i, want.title, want.width, col.Width)
		}
	}
}

func TestCodeBuild_Aliases(t *testing.T) {
	aliases := []string{"cb", "codebuild"}
	for _, alias := range aliases {
		rt := resource.FindResourceType(alias)
		if rt == nil {
			t.Errorf("expected resource type for alias %q, got nil", alias)
		}
	}
}
