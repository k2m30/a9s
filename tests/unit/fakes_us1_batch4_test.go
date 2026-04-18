// fakes_us1_batch4_test.go contains lightweight fake implementations of AWS
// service client interfaces used by the US1 batch-4 checker tests.
// Covered: CodePipelineAPI (cb→pipeline, ecr→pipeline), ECRAPI (ecr→role),
// ECSAPI (ecs-svc→ecr, ecs-svc→secrets), DynamoDBAPI (kinesis→ddb).
// All types are in package unit_test (external test package).
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// fakeCodePipelineBatch4 — implements CodePipelineAPI
// (CodePipelineListPipelinesAPI + CodePipelineGetPipelineStateAPI +
//  CodePipelineGetPipelineAPI)
// Controllable method: GetPipeline — keyed by pipeline name.
// ---------------------------------------------------------------------------

type fakeCodePipelineBatch4 struct {
	declarationsByName map[string]*cptypes.PipelineDeclaration
}

func (f *fakeCodePipelineBatch4) ListPipelines(_ context.Context, _ *codepipeline.ListPipelinesInput, _ ...func(*codepipeline.Options)) (*codepipeline.ListPipelinesOutput, error) {
	return &codepipeline.ListPipelinesOutput{}, nil
}

func (f *fakeCodePipelineBatch4) GetPipelineState(_ context.Context, _ *codepipeline.GetPipelineStateInput, _ ...func(*codepipeline.Options)) (*codepipeline.GetPipelineStateOutput, error) {
	return &codepipeline.GetPipelineStateOutput{}, nil
}

func (f *fakeCodePipelineBatch4) GetPipeline(_ context.Context, input *codepipeline.GetPipelineInput, _ ...func(*codepipeline.Options)) (*codepipeline.GetPipelineOutput, error) {
	if input.Name == nil {
		return &codepipeline.GetPipelineOutput{}, nil
	}
	name := *input.Name
	if f.declarationsByName != nil {
		if decl, ok := f.declarationsByName[name]; ok {
			return &codepipeline.GetPipelineOutput{Pipeline: decl}, nil
		}
	}
	return &codepipeline.GetPipelineOutput{}, nil
}

// Compile-time check: fakeCodePipelineBatch4 satisfies CodePipelineAPI.
var _ awsclient.CodePipelineAPI = (*fakeCodePipelineBatch4)(nil)

// newFakeCodePipelineWithDeclarations returns a fakeCodePipelineBatch4 whose
// GetPipeline returns the declaration keyed by pipeline name.
func newFakeCodePipelineWithDeclarations(declarations map[string]*cptypes.PipelineDeclaration) *fakeCodePipelineBatch4 {
	return &fakeCodePipelineBatch4{declarationsByName: declarations}
}

// pipelineDeclarationWithCodeBuildAction builds a minimal PipelineDeclaration
// whose single stage contains one CodeBuild action referencing projectName.
func pipelineDeclarationWithCodeBuildAction(pipelineName, projectName string) *cptypes.PipelineDeclaration {
	return &cptypes.PipelineDeclaration{
		Name: aws.String(pipelineName),
		Stages: []cptypes.StageDeclaration{
			{
				Name: aws.String("Build"),
				Actions: []cptypes.ActionDeclaration{
					{
						Name: aws.String("BuildAction"),
						ActionTypeId: &cptypes.ActionTypeId{
							Category: cptypes.ActionCategoryBuild,
							Owner:    cptypes.ActionOwnerAws,
							Provider: aws.String("CodeBuild"),
							Version:  aws.String("1"),
						},
						Configuration: map[string]string{
							"ProjectName": projectName,
						},
					},
				},
			},
		},
	}
}

// pipelineDeclarationWithECRSourceAction builds a minimal PipelineDeclaration
// whose single stage contains one ECR Source action referencing repoName.
func pipelineDeclarationWithECRSourceAction(pipelineName, repoName string) *cptypes.PipelineDeclaration {
	return &cptypes.PipelineDeclaration{
		Name: aws.String(pipelineName),
		Stages: []cptypes.StageDeclaration{
			{
				Name: aws.String("Source"),
				Actions: []cptypes.ActionDeclaration{
					{
						Name: aws.String("ECRSource"),
						ActionTypeId: &cptypes.ActionTypeId{
							Category: cptypes.ActionCategorySource,
							Owner:    cptypes.ActionOwnerAws,
							Provider: aws.String("ECR"),
							Version:  aws.String("1"),
						},
						Configuration: map[string]string{
							"RepositoryName": repoName,
						},
					},
				},
			},
		},
	}
}

// pipelineDeclarationEmpty builds a minimal PipelineDeclaration with no
// matching actions (unrelated pipeline).
func pipelineDeclarationEmpty(pipelineName string) *cptypes.PipelineDeclaration {
	return &cptypes.PipelineDeclaration{
		Name: aws.String(pipelineName),
		Stages: []cptypes.StageDeclaration{
			{
				Name: aws.String("Deploy"),
				Actions: []cptypes.ActionDeclaration{
					{
						Name: aws.String("DeployAction"),
						ActionTypeId: &cptypes.ActionTypeId{
							Category: cptypes.ActionCategoryDeploy,
							Owner:    cptypes.ActionOwnerAws,
							Provider: aws.String("ECS"),
							Version:  aws.String("1"),
						},
						Configuration: map[string]string{},
					},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// fakeECRBatch4 — implements ECRAPI
// (ECRDescribeRepositoriesAPI + ECRDescribeImagesAPI +
//  ECRDescribeImageScanFindingsAPI + ECRGetRepositoryPolicyAPI)
// Controllable method: GetRepositoryPolicy.
// ---------------------------------------------------------------------------

type fakeECRBatch4 struct {
	getPolicyOutput *ecr.GetRepositoryPolicyOutput
	getPolicyErr    error
}

func (f *fakeECRBatch4) DescribeRepositories(_ context.Context, _ *ecr.DescribeRepositoriesInput, _ ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
	return &ecr.DescribeRepositoriesOutput{}, nil
}

func (f *fakeECRBatch4) DescribeImages(_ context.Context, _ *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	return &ecr.DescribeImagesOutput{}, nil
}

func (f *fakeECRBatch4) DescribeImageScanFindings(_ context.Context, _ *ecr.DescribeImageScanFindingsInput, _ ...func(*ecr.Options)) (*ecr.DescribeImageScanFindingsOutput, error) {
	return &ecr.DescribeImageScanFindingsOutput{}, nil
}

func (f *fakeECRBatch4) GetRepositoryPolicy(_ context.Context, _ *ecr.GetRepositoryPolicyInput, _ ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error) {
	if f.getPolicyErr != nil {
		return nil, f.getPolicyErr
	}
	if f.getPolicyOutput != nil {
		return f.getPolicyOutput, nil
	}
	return &ecr.GetRepositoryPolicyOutput{}, nil
}

// Compile-time check: fakeECRBatch4 satisfies ECRAPI.
var _ awsclient.ECRAPI = (*fakeECRBatch4)(nil)

// newFakeECRWithRepositoryPolicy returns a fakeECRBatch4 whose GetRepositoryPolicy
// returns the given IAM policy JSON text.
func newFakeECRWithRepositoryPolicy(policyText string) *fakeECRBatch4 {
	return &fakeECRBatch4{
		getPolicyOutput: &ecr.GetRepositoryPolicyOutput{
			PolicyText: aws.String(policyText),
		},
	}
}

// newFakeECRWithNoPolicyError returns a fakeECRBatch4 whose GetRepositoryPolicy
// returns a RepositoryPolicyNotFoundException error.
func newFakeECRWithNoPolicyError() *fakeECRBatch4 {
	return &fakeECRBatch4{
		getPolicyErr: &ecrtypes.RepositoryPolicyNotFoundException{
			Message: aws.String("Repository policy not found"),
		},
	}
}

// ---------------------------------------------------------------------------
// fakeECSBatch4 — implements ECSAPI
// (ECSListClustersAPI + ECSDescribeClustersAPI + ECSListServicesAPI +
//  ECSDescribeServicesAPI + ECSListTasksAPI + ECSDescribeTasksAPI +
//  ECSDescribeTaskDefinitionAPI)
// Controllable method: DescribeTaskDefinition.
// ---------------------------------------------------------------------------

type fakeECSBatch4 struct {
	describeTaskDefFn func(*ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
}

func (f *fakeECSBatch4) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{}, nil
}

func (f *fakeECSBatch4) DescribeClusters(_ context.Context, _ *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return &ecs.DescribeClustersOutput{}, nil
}

func (f *fakeECSBatch4) ListServices(_ context.Context, _ *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	return &ecs.ListServicesOutput{}, nil
}

func (f *fakeECSBatch4) DescribeServices(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return &ecs.DescribeServicesOutput{}, nil
}

func (f *fakeECSBatch4) ListTasks(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return &ecs.ListTasksOutput{}, nil
}

func (f *fakeECSBatch4) DescribeTasks(_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	return &ecs.DescribeTasksOutput{}, nil
}

func (f *fakeECSBatch4) DescribeTaskDefinition(_ context.Context, input *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	if f.describeTaskDefFn != nil {
		return f.describeTaskDefFn(input)
	}
	return &ecs.DescribeTaskDefinitionOutput{}, nil
}

// Compile-time check: fakeECSBatch4 satisfies ECSAPI.
var _ awsclient.ECSAPI = (*fakeECSBatch4)(nil)

// newFakeECSWithTaskDefinition returns a fakeECSBatch4 whose DescribeTaskDefinition
// always returns the given task definition.
func newFakeECSWithTaskDefinition(td *ecstypes.TaskDefinition) *fakeECSBatch4 {
	return &fakeECSBatch4{
		describeTaskDefFn: func(_ *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
			return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: td}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// fakeDynamoDBBatch4 — implements DynamoDBAPI
// (DDBListTablesAPI + DDBDescribeTableAPI +
//  DynamoDBDescribeContinuousBackupsAPI +
//  DynamoDBDescribeKinesisStreamingDestinationAPI)
// Controllable method: DescribeKinesisStreamingDestination — keyed by table name.
// ---------------------------------------------------------------------------

type fakeDynamoDBBatch4 struct {
	kinesisDestByTable map[string][]ddbtypes.KinesisDataStreamDestination
}

func (f *fakeDynamoDBBatch4) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return &dynamodb.ListTablesOutput{}, nil
}

func (f *fakeDynamoDBBatch4) DescribeTable(_ context.Context, _ *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{}, nil
}

func (f *fakeDynamoDBBatch4) DescribeContinuousBackups(_ context.Context, _ *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	return &dynamodb.DescribeContinuousBackupsOutput{}, nil
}

func (f *fakeDynamoDBBatch4) DescribeKinesisStreamingDestination(_ context.Context, input *dynamodb.DescribeKinesisStreamingDestinationInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
	if input.TableName == nil {
		return &dynamodb.DescribeKinesisStreamingDestinationOutput{}, nil
	}
	tableName := *input.TableName
	if f.kinesisDestByTable != nil {
		if dests, ok := f.kinesisDestByTable[tableName]; ok {
			return &dynamodb.DescribeKinesisStreamingDestinationOutput{
				TableName:                        aws.String(tableName),
				KinesisDataStreamDestinations:    dests,
			}, nil
		}
	}
	return &dynamodb.DescribeKinesisStreamingDestinationOutput{
		TableName:                     aws.String(tableName),
		KinesisDataStreamDestinations: []ddbtypes.KinesisDataStreamDestination{},
	}, nil
}

// Compile-time check: fakeDynamoDBBatch4 satisfies DynamoDBAPI.
var _ awsclient.DynamoDBAPI = (*fakeDynamoDBBatch4)(nil)

// newFakeDynamoDBWithKinesisDestination returns a fakeDynamoDBBatch4 whose
// DescribeKinesisStreamingDestination returns the given stream ARN as an
// ACTIVE destination for the given table name.
func newFakeDynamoDBWithKinesisDestination(tableName, streamARN string) *fakeDynamoDBBatch4 {
	return &fakeDynamoDBBatch4{
		kinesisDestByTable: map[string][]ddbtypes.KinesisDataStreamDestination{
			tableName: {
				{
					StreamArn:            aws.String(streamARN),
					DestinationStatus:    ddbtypes.DestinationStatusActive,
				},
			},
		},
	}
}
