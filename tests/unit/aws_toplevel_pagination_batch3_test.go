package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	codebuildtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	codepipelinetypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// 1. IAM ListGroups — Marker/IsTruncated pagination
// ---------------------------------------------------------------------------

type mockIAMListGroupsPaginatedClient struct {
	outputs []*iam.ListGroupsOutput
	err     error
	callIdx int
}

func (m *mockIAMListGroupsPaginatedClient) ListGroups(
	ctx context.Context,
	params *iam.ListGroupsInput,
	optFns ...func(*iam.Options),
) (*iam.ListGroupsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &iam.ListGroupsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchIAMGroups_Pagination(t *testing.T) {
	mock := &mockIAMListGroupsPaginatedClient{
		outputs: []*iam.ListGroupsOutput{
			{
				IsTruncated: true,
				Marker:      aws.String("page2-marker"),
				Groups: []iamtypes.Group{
					{GroupName: aws.String("page1-group-1"), GroupId: aws.String("AGPAEXAMPLE1"), Path: aws.String("/")},
					{GroupName: aws.String("page1-group-2"), GroupId: aws.String("AGPAEXAMPLE2"), Path: aws.String("/")},
				},
			},
			{
				IsTruncated: false,
				Groups: []iamtypes.Group{
					{GroupName: aws.String("page2-group-1"), GroupId: aws.String("AGPAEXAMPLE3"), Path: aws.String("/admin/")},
				},
			},
		},
	}

	resources, err := awsclient.FetchIAMGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_groups", func(t *testing.T) {
		if resources[0].ID != "page1-group-1" {
			t.Errorf("expected %q, got %q", "page1-group-1", resources[0].ID)
		}
		if resources[1].ID != "page1-group-2" {
			t.Errorf("expected %q, got %q", "page1-group-2", resources[1].ID)
		}
	})

	t.Run("page2_group", func(t *testing.T) {
		if resources[2].ID != "page2-group-1" {
			t.Errorf("expected %q, got %q", "page2-group-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 2. API Gateway V2 GetApis — NextToken pagination
// ---------------------------------------------------------------------------

type mockAPIGWPaginatedClient struct {
	outputs []*apigatewayv2.GetApisOutput
	err     error
	callIdx int
}

func (m *mockAPIGWPaginatedClient) GetApis(
	ctx context.Context,
	params *apigatewayv2.GetApisInput,
	optFns ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetApisOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &apigatewayv2.GetApisOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchAPIGateways_Pagination(t *testing.T) {
	mock := &mockAPIGWPaginatedClient{
		outputs: []*apigatewayv2.GetApisOutput{
			{
				NextToken: aws.String("page2-token"),
				Items: []apigatewayv2types.Api{
					{ApiId: aws.String("api-page1-001"), Name: aws.String("page1-api-1"), ProtocolType: apigatewayv2types.ProtocolTypeHttp},
				},
			},
			{
				Items: []apigatewayv2types.Api{
					{ApiId: aws.String("api-page2-001"), Name: aws.String("page2-api-1"), ProtocolType: apigatewayv2types.ProtocolTypeWebsocket},
					{ApiId: aws.String("api-page2-002"), Name: aws.String("page2-api-2"), ProtocolType: apigatewayv2types.ProtocolTypeHttp},
				},
			},
		},
	}

	resources, err := awsclient.FetchAPIGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_api", func(t *testing.T) {
		if resources[0].ID != "api-page1-001" {
			t.Errorf("expected %q, got %q", "api-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_apis", func(t *testing.T) {
		if resources[1].ID != "api-page2-001" {
			t.Errorf("expected %q, got %q", "api-page2-001", resources[1].ID)
		}
		if resources[2].ID != "api-page2-002" {
			t.Errorf("expected %q, got %q", "api-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 3. Athena ListWorkGroups — NextToken pagination
// ---------------------------------------------------------------------------

type mockAthenaPaginatedClient struct {
	outputs []*athena.ListWorkGroupsOutput
	err     error
	callIdx int
}

func (m *mockAthenaPaginatedClient) ListWorkGroups(
	ctx context.Context,
	params *athena.ListWorkGroupsInput,
	optFns ...func(*athena.Options),
) (*athena.ListWorkGroupsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &athena.ListWorkGroupsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchAthenaWorkgroups_Pagination(t *testing.T) {
	mock := &mockAthenaPaginatedClient{
		outputs: []*athena.ListWorkGroupsOutput{
			{
				NextToken: aws.String("page2-token"),
				WorkGroups: []athenatypes.WorkGroupSummary{
					{Name: aws.String("page1-wg-1"), State: athenatypes.WorkGroupStateEnabled},
				},
			},
			{
				WorkGroups: []athenatypes.WorkGroupSummary{
					{Name: aws.String("page2-wg-1"), State: athenatypes.WorkGroupStateDisabled},
					{Name: aws.String("page2-wg-2"), State: athenatypes.WorkGroupStateEnabled},
				},
			},
		},
	}

	resources, err := awsclient.FetchAthenaWorkgroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_wg", func(t *testing.T) {
		if resources[0].ID != "page1-wg-1" {
			t.Errorf("expected %q, got %q", "page1-wg-1", resources[0].ID)
		}
	})

	t.Run("page2_wgs", func(t *testing.T) {
		if resources[1].ID != "page2-wg-1" {
			t.Errorf("expected %q, got %q", "page2-wg-1", resources[1].ID)
		}
		if resources[2].ID != "page2-wg-2" {
			t.Errorf("expected %q, got %q", "page2-wg-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 4. Backup ListBackupPlans — NextToken pagination
// ---------------------------------------------------------------------------

type mockBackupPaginatedClient struct {
	outputs []*backup.ListBackupPlansOutput
	err     error
	callIdx int
}

func (m *mockBackupPaginatedClient) ListBackupPlans(
	ctx context.Context,
	params *backup.ListBackupPlansInput,
	optFns ...func(*backup.Options),
) (*backup.ListBackupPlansOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &backup.ListBackupPlansOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchBackupPlans_Pagination(t *testing.T) {
	mock := &mockBackupPaginatedClient{
		outputs: []*backup.ListBackupPlansOutput{
			{
				NextToken: aws.String("page2-token"),
				BackupPlansList: []backuptypes.BackupPlansListMember{
					{BackupPlanName: aws.String("page1-plan-1"), BackupPlanId: aws.String("bp-page1-001")},
				},
			},
			{
				BackupPlansList: []backuptypes.BackupPlansListMember{
					{BackupPlanName: aws.String("page2-plan-1"), BackupPlanId: aws.String("bp-page2-001")},
					{BackupPlanName: aws.String("page2-plan-2"), BackupPlanId: aws.String("bp-page2-002")},
				},
			},
		},
	}

	resources, err := awsclient.FetchBackupPlans(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_plan", func(t *testing.T) {
		if resources[0].ID != "bp-page1-001" {
			t.Errorf("expected %q, got %q", "bp-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_plans", func(t *testing.T) {
		if resources[1].ID != "bp-page2-001" {
			t.Errorf("expected %q, got %q", "bp-page2-001", resources[1].ID)
		}
		if resources[2].ID != "bp-page2-002" {
			t.Errorf("expected %q, got %q", "bp-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 5. CodeBuild ListProjects — NextToken pagination (two-step: List + BatchGet)
// ---------------------------------------------------------------------------

type mockCodeBuildListProjectsPaginatedClient struct {
	outputs []*codebuild.ListProjectsOutput
	err     error
	callIdx int
}

func (m *mockCodeBuildListProjectsPaginatedClient) ListProjects(
	ctx context.Context,
	params *codebuild.ListProjectsInput,
	optFns ...func(*codebuild.Options),
) (*codebuild.ListProjectsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &codebuild.ListProjectsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

type mockCodeBuildBatchGetProjectsPaginatedClient struct {
	output *codebuild.BatchGetProjectsOutput
	err    error
}

func (m *mockCodeBuildBatchGetProjectsPaginatedClient) BatchGetProjects(
	ctx context.Context,
	params *codebuild.BatchGetProjectsInput,
	optFns ...func(*codebuild.Options),
) (*codebuild.BatchGetProjectsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Filter projects based on requested names
	var filtered []codebuildtypes.Project
	nameSet := make(map[string]bool)
	for _, n := range params.Names {
		nameSet[n] = true
	}
	for _, p := range m.output.Projects {
		if p.Name != nil && nameSet[*p.Name] {
			filtered = append(filtered, p)
		}
	}
	return &codebuild.BatchGetProjectsOutput{Projects: filtered}, nil
}

func TestFetchCodeBuildProjects_Pagination(t *testing.T) {
	listMock := &mockCodeBuildListProjectsPaginatedClient{
		outputs: []*codebuild.ListProjectsOutput{
			{
				NextToken: aws.String("page2-token"),
				Projects:  []string{"page1-project-1"},
			},
			{
				Projects: []string{"page2-project-1", "page2-project-2"},
			},
		},
	}

	batchMock := &mockCodeBuildBatchGetProjectsPaginatedClient{
		output: &codebuild.BatchGetProjectsOutput{
			Projects: []codebuildtypes.Project{
				{Name: aws.String("page1-project-1"), Description: aws.String("Project 1")},
				{Name: aws.String("page2-project-1"), Description: aws.String("Project 2")},
				{Name: aws.String("page2-project-2"), Description: aws.String("Project 3")},
			},
		},
	}

	resources, err := awsclient.FetchCodeBuildProjects(context.Background(), listMock, batchMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_project", func(t *testing.T) {
		if resources[0].ID != "page1-project-1" {
			t.Errorf("expected %q, got %q", "page1-project-1", resources[0].ID)
		}
	})

	t.Run("page2_projects", func(t *testing.T) {
		if resources[1].ID != "page2-project-1" {
			t.Errorf("expected %q, got %q", "page2-project-1", resources[1].ID)
		}
		if resources[2].ID != "page2-project-2" {
			t.Errorf("expected %q, got %q", "page2-project-2", resources[2].ID)
		}
	})

	t.Run("list_api_called_twice", func(t *testing.T) {
		if listMock.callIdx != 2 {
			t.Errorf("expected 2 ListProjects API calls, got %d", listMock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 6. CodePipeline ListPipelines — NextToken pagination
// ---------------------------------------------------------------------------

type mockCodePipelinePaginatedClient struct {
	outputs []*codepipeline.ListPipelinesOutput
	err     error
	callIdx int
}

func (m *mockCodePipelinePaginatedClient) ListPipelines(
	ctx context.Context,
	params *codepipeline.ListPipelinesInput,
	optFns ...func(*codepipeline.Options),
) (*codepipeline.ListPipelinesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &codepipeline.ListPipelinesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchCodePipelines_Pagination(t *testing.T) {
	mock := &mockCodePipelinePaginatedClient{
		outputs: []*codepipeline.ListPipelinesOutput{
			{
				NextToken: aws.String("page2-token"),
				Pipelines: []codepipelinetypes.PipelineSummary{
					{Name: aws.String("page1-pipeline-1")},
				},
			},
			{
				Pipelines: []codepipelinetypes.PipelineSummary{
					{Name: aws.String("page2-pipeline-1")},
					{Name: aws.String("page2-pipeline-2")},
				},
			},
		},
	}

	resources, err := awsclient.FetchCodePipelines(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_pipeline", func(t *testing.T) {
		if resources[0].ID != "page1-pipeline-1" {
			t.Errorf("expected %q, got %q", "page1-pipeline-1", resources[0].ID)
		}
	})

	t.Run("page2_pipelines", func(t *testing.T) {
		if resources[1].ID != "page2-pipeline-1" {
			t.Errorf("expected %q, got %q", "page2-pipeline-1", resources[1].ID)
		}
		if resources[2].ID != "page2-pipeline-2" {
			t.Errorf("expected %q, got %q", "page2-pipeline-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 7. CodeArtifact ListRepositories — NextToken pagination
// ---------------------------------------------------------------------------

type mockCodeArtifactPaginatedClient struct {
	outputs []*codeartifact.ListRepositoriesOutput
	err     error
	callIdx int
}

func (m *mockCodeArtifactPaginatedClient) ListRepositories(
	ctx context.Context,
	params *codeartifact.ListRepositoriesInput,
	optFns ...func(*codeartifact.Options),
) (*codeartifact.ListRepositoriesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &codeartifact.ListRepositoriesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchCodeArtifactRepos_Pagination(t *testing.T) {
	mock := &mockCodeArtifactPaginatedClient{
		outputs: []*codeartifact.ListRepositoriesOutput{
			{
				NextToken: aws.String("page2-token"),
				Repositories: []codeartifacttypes.RepositorySummary{
					{Name: aws.String("page1-repo-1"), DomainName: aws.String("my-domain")},
				},
			},
			{
				Repositories: []codeartifacttypes.RepositorySummary{
					{Name: aws.String("page2-repo-1"), DomainName: aws.String("my-domain")},
					{Name: aws.String("page2-repo-2"), DomainName: aws.String("my-domain")},
				},
			},
		},
	}

	resources, err := awsclient.FetchCodeArtifactRepos(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_repo", func(t *testing.T) {
		if resources[0].ID != "page1-repo-1" {
			t.Errorf("expected %q, got %q", "page1-repo-1", resources[0].ID)
		}
	})

	t.Run("page2_repos", func(t *testing.T) {
		if resources[1].ID != "page2-repo-1" {
			t.Errorf("expected %q, got %q", "page2-repo-1", resources[1].ID)
		}
		if resources[2].ID != "page2-repo-2" {
			t.Errorf("expected %q, got %q", "page2-repo-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 8. Elastic Beanstalk DescribeEnvironments — NextToken pagination
// ---------------------------------------------------------------------------

type mockEBPaginatedClient struct {
	outputs []*elasticbeanstalk.DescribeEnvironmentsOutput
	err     error
	callIdx int
}

func (m *mockEBPaginatedClient) DescribeEnvironments(
	ctx context.Context,
	params *elasticbeanstalk.DescribeEnvironmentsInput,
	optFns ...func(*elasticbeanstalk.Options),
) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &elasticbeanstalk.DescribeEnvironmentsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchEBEnvironments_Pagination(t *testing.T) {
	mock := &mockEBPaginatedClient{
		outputs: []*elasticbeanstalk.DescribeEnvironmentsOutput{
			{
				NextToken: aws.String("page2-token"),
				Environments: []ebtypes.EnvironmentDescription{
					{EnvironmentName: aws.String("page1-env-1"), EnvironmentId: aws.String("e-page1001"), Status: ebtypes.EnvironmentStatusReady},
				},
			},
			{
				Environments: []ebtypes.EnvironmentDescription{
					{EnvironmentName: aws.String("page2-env-1"), EnvironmentId: aws.String("e-page2001"), Status: ebtypes.EnvironmentStatusReady},
					{EnvironmentName: aws.String("page2-env-2"), EnvironmentId: aws.String("e-page2002"), Status: ebtypes.EnvironmentStatusTerminating},
				},
			},
		},
	}

	resources, err := awsclient.FetchEBEnvironments(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_env", func(t *testing.T) {
		if resources[0].ID != "e-page1001" {
			t.Errorf("expected %q, got %q", "e-page1001", resources[0].ID)
		}
	})

	t.Run("page2_envs", func(t *testing.T) {
		if resources[1].ID != "e-page2001" {
			t.Errorf("expected %q, got %q", "e-page2001", resources[1].ID)
		}
		if resources[2].ID != "e-page2002" {
			t.Errorf("expected %q, got %q", "e-page2002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 9. EFS DescribeFileSystems — Marker/NextMarker pagination
// ---------------------------------------------------------------------------

type mockEFSPaginatedClient struct {
	outputs []*efs.DescribeFileSystemsOutput
	err     error
	callIdx int
}

func (m *mockEFSPaginatedClient) DescribeFileSystems(
	ctx context.Context,
	params *efs.DescribeFileSystemsInput,
	optFns ...func(*efs.Options),
) (*efs.DescribeFileSystemsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &efs.DescribeFileSystemsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchEFSFileSystems_Pagination(t *testing.T) {
	mock := &mockEFSPaginatedClient{
		outputs: []*efs.DescribeFileSystemsOutput{
			{
				NextMarker: aws.String("page2-marker"),
				FileSystems: []efstypes.FileSystemDescription{
					{FileSystemId: aws.String("fs-page1-001"), Name: aws.String("page1-fs-1"), LifeCycleState: efstypes.LifeCycleStateAvailable},
				},
			},
			{
				FileSystems: []efstypes.FileSystemDescription{
					{FileSystemId: aws.String("fs-page2-001"), Name: aws.String("page2-fs-1"), LifeCycleState: efstypes.LifeCycleStateAvailable},
					{FileSystemId: aws.String("fs-page2-002"), Name: aws.String("page2-fs-2"), LifeCycleState: efstypes.LifeCycleStateCreating},
				},
			},
		},
	}

	resources, err := awsclient.FetchEFSFileSystems(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_fs", func(t *testing.T) {
		if resources[0].ID != "fs-page1-001" {
			t.Errorf("expected %q, got %q", "fs-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_fs", func(t *testing.T) {
		if resources[1].ID != "fs-page2-001" {
			t.Errorf("expected %q, got %q", "fs-page2-001", resources[1].ID)
		}
		if resources[2].ID != "fs-page2-002" {
			t.Errorf("expected %q, got %q", "fs-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 10. EKS ListClusters + DescribeCluster — NextToken pagination on ListClusters
// ---------------------------------------------------------------------------

type mockEKSListClustersPaginatedClient struct {
	outputs []*eks.ListClustersOutput
	err     error
	callIdx int
}

func (m *mockEKSListClustersPaginatedClient) ListClusters(
	ctx context.Context,
	params *eks.ListClustersInput,
	optFns ...func(*eks.Options),
) (*eks.ListClustersOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &eks.ListClustersOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

type mockEKSDescribeClusterPaginatedClient struct {
	clusters map[string]*eks.DescribeClusterOutput
	err      error
}

func (m *mockEKSDescribeClusterPaginatedClient) DescribeCluster(
	ctx context.Context,
	params *eks.DescribeClusterInput,
	optFns ...func(*eks.Options),
) (*eks.DescribeClusterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if out, ok := m.clusters[*params.Name]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("cluster %s not found", *params.Name)
}

func TestFetchEKSClusters_Pagination(t *testing.T) {
	listMock := &mockEKSListClustersPaginatedClient{
		outputs: []*eks.ListClustersOutput{
			{
				NextToken: aws.String("page2-token"),
				Clusters:  []string{"page1-cluster-1"},
			},
			{
				Clusters: []string{"page2-cluster-1", "page2-cluster-2"},
			},
		},
	}

	describeMock := &mockEKSDescribeClusterPaginatedClient{
		clusters: map[string]*eks.DescribeClusterOutput{
			"page1-cluster-1": {Cluster: &ekstypes.Cluster{Name: aws.String("page1-cluster-1"), Version: aws.String("1.28"), Status: ekstypes.ClusterStatusActive}},
			"page2-cluster-1": {Cluster: &ekstypes.Cluster{Name: aws.String("page2-cluster-1"), Version: aws.String("1.27"), Status: ekstypes.ClusterStatusActive}},
			"page2-cluster-2": {Cluster: &ekstypes.Cluster{Name: aws.String("page2-cluster-2"), Version: aws.String("1.29"), Status: ekstypes.ClusterStatusCreating}},
		},
	}

	resources, err := awsclient.FetchEKSClusters(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_cluster", func(t *testing.T) {
		if resources[0].ID != "page1-cluster-1" {
			t.Errorf("expected %q, got %q", "page1-cluster-1", resources[0].ID)
		}
	})

	t.Run("page2_clusters", func(t *testing.T) {
		if resources[1].ID != "page2-cluster-1" {
			t.Errorf("expected %q, got %q", "page2-cluster-1", resources[1].ID)
		}
		if resources[2].ID != "page2-cluster-2" {
			t.Errorf("expected %q, got %q", "page2-cluster-2", resources[2].ID)
		}
	})

	t.Run("list_api_called_twice", func(t *testing.T) {
		if listMock.callIdx != 2 {
			t.Errorf("expected 2 ListClusters API calls, got %d", listMock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 11. Glue GetJobs — NextToken pagination
// ---------------------------------------------------------------------------

type mockGluePaginatedClient struct {
	outputs []*glue.GetJobsOutput
	err     error
	callIdx int
}

func (m *mockGluePaginatedClient) GetJobs(
	ctx context.Context,
	params *glue.GetJobsInput,
	optFns ...func(*glue.Options),
) (*glue.GetJobsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &glue.GetJobsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchGlueJobs_Pagination(t *testing.T) {
	mock := &mockGluePaginatedClient{
		outputs: []*glue.GetJobsOutput{
			{
				NextToken: aws.String("page2-token"),
				Jobs: []gluetypes.Job{
					{Name: aws.String("page1-job-1"), GlueVersion: aws.String("3.0")},
				},
			},
			{
				Jobs: []gluetypes.Job{
					{Name: aws.String("page2-job-1"), GlueVersion: aws.String("4.0")},
					{Name: aws.String("page2-job-2"), GlueVersion: aws.String("3.0")},
				},
			},
		},
	}

	resources, err := awsclient.FetchGlueJobs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_job", func(t *testing.T) {
		if resources[0].ID != "page1-job-1" {
			t.Errorf("expected %q, got %q", "page1-job-1", resources[0].ID)
		}
	})

	t.Run("page2_jobs", func(t *testing.T) {
		if resources[1].ID != "page2-job-1" {
			t.Errorf("expected %q, got %q", "page2-job-1", resources[1].ID)
		}
		if resources[2].ID != "page2-job-2" {
			t.Errorf("expected %q, got %q", "page2-job-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 12. MSK ListClustersV2 — NextToken pagination
// ---------------------------------------------------------------------------

type mockMSKPaginatedClient struct {
	outputs []*kafka.ListClustersV2Output
	err     error
	callIdx int
}

func (m *mockMSKPaginatedClient) ListClustersV2(
	ctx context.Context,
	params *kafka.ListClustersV2Input,
	optFns ...func(*kafka.Options),
) (*kafka.ListClustersV2Output, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &kafka.ListClustersV2Output{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchMSKClusters_Pagination(t *testing.T) {
	mock := &mockMSKPaginatedClient{
		outputs: []*kafka.ListClustersV2Output{
			{
				NextToken: aws.String("page2-token"),
				ClusterInfoList: []kafkatypes.Cluster{
					{ClusterName: aws.String("page1-msk-1"), State: kafkatypes.ClusterStateActive},
				},
			},
			{
				ClusterInfoList: []kafkatypes.Cluster{
					{ClusterName: aws.String("page2-msk-1"), State: kafkatypes.ClusterStateActive},
					{ClusterName: aws.String("page2-msk-2"), State: kafkatypes.ClusterStateCreating},
				},
			},
		},
	}

	resources, err := awsclient.FetchMSKClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_cluster", func(t *testing.T) {
		if resources[0].ID != "page1-msk-1" {
			t.Errorf("expected %q, got %q", "page1-msk-1", resources[0].ID)
		}
	})

	t.Run("page2_clusters", func(t *testing.T) {
		if resources[1].ID != "page2-msk-1" {
			t.Errorf("expected %q, got %q", "page2-msk-1", resources[1].ID)
		}
		if resources[2].ID != "page2-msk-2" {
			t.Errorf("expected %q, got %q", "page2-msk-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 13. SES ListEmailIdentities — NextToken pagination
// ---------------------------------------------------------------------------

type mockSESPaginatedClient struct {
	outputs []*sesv2.ListEmailIdentitiesOutput
	err     error
	callIdx int
}

func (m *mockSESPaginatedClient) ListEmailIdentities(
	ctx context.Context,
	params *sesv2.ListEmailIdentitiesInput,
	optFns ...func(*sesv2.Options),
) (*sesv2.ListEmailIdentitiesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &sesv2.ListEmailIdentitiesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchSESIdentities_Pagination(t *testing.T) {
	mock := &mockSESPaginatedClient{
		outputs: []*sesv2.ListEmailIdentitiesOutput{
			{
				NextToken: aws.String("page2-token"),
				EmailIdentities: []sesv2types.IdentityInfo{
					{IdentityName: aws.String("page1-identity@example.com"), IdentityType: sesv2types.IdentityTypeEmailAddress},
				},
			},
			{
				EmailIdentities: []sesv2types.IdentityInfo{
					{IdentityName: aws.String("page2-domain.example.com"), IdentityType: sesv2types.IdentityTypeDomain},
					{IdentityName: aws.String("page2-identity@example.com"), IdentityType: sesv2types.IdentityTypeEmailAddress},
				},
			},
		},
	}

	resources, err := awsclient.FetchSESIdentities(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_identity", func(t *testing.T) {
		if resources[0].ID != "page1-identity@example.com" {
			t.Errorf("expected %q, got %q", "page1-identity@example.com", resources[0].ID)
		}
	})

	t.Run("page2_identities", func(t *testing.T) {
		if resources[1].ID != "page2-domain.example.com" {
			t.Errorf("expected %q, got %q", "page2-domain.example.com", resources[1].ID)
		}
		if resources[2].ID != "page2-identity@example.com" {
			t.Errorf("expected %q, got %q", "page2-identity@example.com", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 14. WAF ListWebACLs — NextMarker pagination
// ---------------------------------------------------------------------------

type mockWAFPaginatedClient struct {
	outputs []*wafv2.ListWebACLsOutput
	err     error
	callIdx int
}

func (m *mockWAFPaginatedClient) ListWebACLs(
	ctx context.Context,
	params *wafv2.ListWebACLsInput,
	optFns ...func(*wafv2.Options),
) (*wafv2.ListWebACLsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &wafv2.ListWebACLsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchWAFWebACLs_Pagination(t *testing.T) {
	mock := &mockWAFPaginatedClient{
		outputs: []*wafv2.ListWebACLsOutput{
			{
				NextMarker: aws.String("page2-marker"),
				WebACLs: []wafv2types.WebACLSummary{
					{Name: aws.String("page1-acl-1"), Id: aws.String("waf-page1-001")},
				},
			},
			{
				WebACLs: []wafv2types.WebACLSummary{
					{Name: aws.String("page2-acl-1"), Id: aws.String("waf-page2-001")},
					{Name: aws.String("page2-acl-2"), Id: aws.String("waf-page2-002")},
				},
			},
		},
	}

	resources, err := awsclient.FetchWAFWebACLs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_acl", func(t *testing.T) {
		if resources[0].ID != "waf-page1-001" {
			t.Errorf("expected %q, got %q", "waf-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_acls", func(t *testing.T) {
		if resources[1].ID != "waf-page2-001" {
			t.Errorf("expected %q, got %q", "waf-page2-001", resources[1].ID)
		}
		if resources[2].ID != "waf-page2-002" {
			t.Errorf("expected %q, got %q", "waf-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 15. Node Groups — ListClusters + ListNodegroups + DescribeNodegroup pagination
// ---------------------------------------------------------------------------

type mockEKSListNodegroupsPaginatedClient struct {
	outputs map[string][]*eks.ListNodegroupsOutput
	callIdx map[string]int
	err     error
}

func (m *mockEKSListNodegroupsPaginatedClient) ListNodegroups(
	ctx context.Context,
	params *eks.ListNodegroupsInput,
	optFns ...func(*eks.Options),
) (*eks.ListNodegroupsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	cluster := *params.ClusterName
	if m.callIdx == nil {
		m.callIdx = make(map[string]int)
	}
	idx := m.callIdx[cluster]
	pages := m.outputs[cluster]
	if idx >= len(pages) {
		return &eks.ListNodegroupsOutput{}, nil
	}
	out := pages[idx]
	m.callIdx[cluster] = idx + 1
	return out, nil
}

type mockEKSDescribeNodegroupPaginatedClient struct {
	nodegroups map[string]*eks.DescribeNodegroupOutput
	err        error
}

func (m *mockEKSDescribeNodegroupPaginatedClient) DescribeNodegroup(
	ctx context.Context,
	params *eks.DescribeNodegroupInput,
	optFns ...func(*eks.Options),
) (*eks.DescribeNodegroupOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := *params.ClusterName + "/" + *params.NodegroupName
	if out, ok := m.nodegroups[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("nodegroup %s not found", key)
}

func TestFetchNodeGroups_Pagination(t *testing.T) {
	// ListClusters returns 2 pages with 1 cluster each
	listClustersMock := &mockEKSListClustersPaginatedClient{
		outputs: []*eks.ListClustersOutput{
			{
				NextToken: aws.String("page2-token"),
				Clusters:  []string{"cluster-A"},
			},
			{
				Clusters: []string{"cluster-B"},
			},
		},
	}

	// ListNodegroups for cluster-A returns 2 pages; cluster-B returns 1 page
	listNGMock := &mockEKSListNodegroupsPaginatedClient{
		outputs: map[string][]*eks.ListNodegroupsOutput{
			"cluster-A": {
				{
					NextToken:  aws.String("ng-page2"),
					Nodegroups: []string{"ng-a1"},
				},
				{
					Nodegroups: []string{"ng-a2"},
				},
			},
			"cluster-B": {
				{
					Nodegroups: []string{"ng-b1"},
				},
			},
		},
	}

	describeNGMock := &mockEKSDescribeNodegroupPaginatedClient{
		nodegroups: map[string]*eks.DescribeNodegroupOutput{
			"cluster-A/ng-a1": {Nodegroup: &ekstypes.Nodegroup{NodegroupName: aws.String("ng-a1"), ClusterName: aws.String("cluster-A"), Status: ekstypes.NodegroupStatusActive}},
			"cluster-A/ng-a2": {Nodegroup: &ekstypes.Nodegroup{NodegroupName: aws.String("ng-a2"), ClusterName: aws.String("cluster-A"), Status: ekstypes.NodegroupStatusActive}},
			"cluster-B/ng-b1": {Nodegroup: &ekstypes.Nodegroup{NodegroupName: aws.String("ng-b1"), ClusterName: aws.String("cluster-B"), Status: ekstypes.NodegroupStatusCreating}},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across paginated calls, got %d", len(resources))
		}
	})

	t.Run("cluster_A_nodegroups", func(t *testing.T) {
		if resources[0].ID != "ng-a1" {
			t.Errorf("expected %q, got %q", "ng-a1", resources[0].ID)
		}
		if resources[1].ID != "ng-a2" {
			t.Errorf("expected %q, got %q", "ng-a2", resources[1].ID)
		}
	})

	t.Run("cluster_B_nodegroup", func(t *testing.T) {
		if resources[2].ID != "ng-b1" {
			t.Errorf("expected %q, got %q", "ng-b1", resources[2].ID)
		}
	})

	t.Run("list_clusters_called_twice", func(t *testing.T) {
		if listClustersMock.callIdx != 2 {
			t.Errorf("expected 2 ListClusters API calls, got %d", listClustersMock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// 16. SNS ListSubscriptions — NextToken pagination
// ---------------------------------------------------------------------------

type mockSNSSubscriptionsPaginatedClient struct {
	outputs []*sns.ListSubscriptionsOutput
	err     error
	callIdx int
}

func (m *mockSNSSubscriptionsPaginatedClient) ListSubscriptions(
	ctx context.Context,
	params *sns.ListSubscriptionsInput,
	optFns ...func(*sns.Options),
) (*sns.ListSubscriptionsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &sns.ListSubscriptionsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchSNSSubscriptions_Pagination(t *testing.T) {
	mock := &mockSNSSubscriptionsPaginatedClient{
		outputs: []*sns.ListSubscriptionsOutput{
			{
				NextToken: aws.String("page2-token"),
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:111122223333:topic1:sub-page1-001"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:111122223333:page1-topic"),
						Protocol:        aws.String("email"),
						Endpoint:        aws.String("user@example.com"),
					},
				},
			},
			{
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:111122223333:topic2:sub-page2-001"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:111122223333:page2-topic"),
						Protocol:        aws.String("sqs"),
						Endpoint:        aws.String("arn:aws:sqs:us-east-1:111122223333:my-queue"),
					},
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:111122223333:topic3:sub-page2-002"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:111122223333:page2-topic-2"),
						Protocol:        aws.String("https"),
						Endpoint:        aws.String("https://example.com/webhook"),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchSNSSubscriptions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_subscription", func(t *testing.T) {
		if resources[0].ID != "arn:aws:sns:us-east-1:111122223333:topic1:sub-page1-001" {
			t.Errorf("expected subscription ARN from page 1, got %q", resources[0].ID)
		}
		// Name is derived from TopicArn — last segment after ":"
		if !strings.HasSuffix(resources[0].Name, "page1-topic") {
			t.Errorf("expected name containing %q, got %q", "page1-topic", resources[0].Name)
		}
	})

	t.Run("page2_subscriptions", func(t *testing.T) {
		if resources[1].ID != "arn:aws:sns:us-east-1:111122223333:topic2:sub-page2-001" {
			t.Errorf("expected subscription ARN from page 2, got %q", resources[1].ID)
		}
		if resources[2].ID != "arn:aws:sns:us-east-1:111122223333:topic3:sub-page2-002" {
			t.Errorf("expected subscription ARN from page 2, got %q", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}
