// fakes_us1_test.go contains fake implementations of AWS service client
// interfaces used by the US1 batch-1 checker tests. All fakes are in
// package unit_test (external test package) so they do NOT rely on any
// shared state in mocks_test.go (package unit).
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	catypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ---------------------------------------------------------------------------
// SSM fake — implements SSMAPI (SSMDescribeParametersAPI +
// SSMGetParameterAPI + SSMDescribeInstanceInformationAPI)
// ---------------------------------------------------------------------------

type fakeSSMUS1 struct {
	instanceInfoOutput *ssm.DescribeInstanceInformationOutput
	instanceInfoErr    error
}

func (f *fakeSSMUS1) DescribeParameters(_ context.Context, _ *ssm.DescribeParametersInput, _ ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	return &ssm.DescribeParametersOutput{}, nil
}

func (f *fakeSSMUS1) GetParameter(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	return &ssm.GetParameterOutput{}, nil
}

func (f *fakeSSMUS1) DescribeInstanceInformation(_ context.Context, _ *ssm.DescribeInstanceInformationInput, _ ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	if f.instanceInfoErr != nil {
		return nil, f.instanceInfoErr
	}
	if f.instanceInfoOutput != nil {
		return f.instanceInfoOutput, nil
	}
	return &ssm.DescribeInstanceInformationOutput{}, nil
}

// newFakeSSMWithInstances constructs a fakeSSMUS1 returning the supplied
// instance information entries.
func newFakeSSMWithInstances(entries []ssmtypes.InstanceInformation) *fakeSSMUS1 {
	return &fakeSSMUS1{
		instanceInfoOutput: &ssm.DescribeInstanceInformationOutput{
			InstanceInformationList: entries,
		},
	}
}

// ---------------------------------------------------------------------------
// EventBridge fake — implements EventBridgeAPI
// (EventBridgeListRulesAPI + EventBridgeListTargetsByRuleAPI +
//  EventBridgeListRuleNamesByTargetAPI)
// ---------------------------------------------------------------------------

type fakeEventBridgeUS1 struct {
	ruleNames []string
	err       error
}

func (f *fakeEventBridgeUS1) ListRules(_ context.Context, _ *eventbridge.ListRulesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	return &eventbridge.ListRulesOutput{}, nil
}

func (f *fakeEventBridgeUS1) ListTargetsByRule(_ context.Context, _ *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	return &eventbridge.ListTargetsByRuleOutput{}, nil
}

func (f *fakeEventBridgeUS1) ListRuleNamesByTarget(_ context.Context, _ *eventbridge.ListRuleNamesByTargetInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRuleNamesByTargetOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &eventbridge.ListRuleNamesByTargetOutput{RuleNames: f.ruleNames}, nil
}

// ---------------------------------------------------------------------------
// Backup fake — implements BackupAPI
// ---------------------------------------------------------------------------

type fakeBackupUS1 struct {
	recoveryPointsOutput *backup.ListRecoveryPointsByResourceOutput
	recoveryPointsErr    error
}

func (f *fakeBackupUS1) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return &backup.ListBackupPlansOutput{}, nil
}

func (f *fakeBackupUS1) ListBackupJobs(_ context.Context, _ *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	return &backup.ListBackupJobsOutput{}, nil
}

func (f *fakeBackupUS1) GetBackupPlan(_ context.Context, _ *backup.GetBackupPlanInput, _ ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error) {
	return &backup.GetBackupPlanOutput{}, nil
}

func (f *fakeBackupUS1) ListBackupSelections(_ context.Context, _ *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	return &backup.ListBackupSelectionsOutput{}, nil
}

func (f *fakeBackupUS1) DescribeBackupVault(_ context.Context, _ *backup.DescribeBackupVaultInput, _ ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error) {
	return &backup.DescribeBackupVaultOutput{}, nil
}

func (f *fakeBackupUS1) GetBackupVaultNotifications(_ context.Context, _ *backup.GetBackupVaultNotificationsInput, _ ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error) {
	return &backup.GetBackupVaultNotificationsOutput{}, nil
}

func (f *fakeBackupUS1) ListRecoveryPointsByResource(_ context.Context, _ *backup.ListRecoveryPointsByResourceInput, _ ...func(*backup.Options)) (*backup.ListRecoveryPointsByResourceOutput, error) {
	if f.recoveryPointsErr != nil {
		return nil, f.recoveryPointsErr
	}
	if f.recoveryPointsOutput != nil {
		return f.recoveryPointsOutput, nil
	}
	return &backup.ListRecoveryPointsByResourceOutput{}, nil
}

// newFakeBackupWithRecoveryPoints constructs a fakeBackupUS1 with the given
// recovery point entries for ListRecoveryPointsByResource.
func newFakeBackupWithRecoveryPoints(entries []backuptypes.RecoveryPointByResource) *fakeBackupUS1 {
	return &fakeBackupUS1{
		recoveryPointsOutput: &backup.ListRecoveryPointsByResourceOutput{
			RecoveryPoints: entries,
		},
	}
}

// ---------------------------------------------------------------------------
// KMS fake — implements KMSAPI
// ---------------------------------------------------------------------------

type fakeKMSUS1 struct {
	listGrantsOutput *kms.ListGrantsOutput
	listGrantsErr    error
	getKeyPolicyOut  *kms.GetKeyPolicyOutput
	getKeyPolicyErr  error
}

func (f *fakeKMSUS1) ListKeys(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return &kms.ListKeysOutput{}, nil
}

func (f *fakeKMSUS1) DescribeKey(_ context.Context, _ *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	return &kms.DescribeKeyOutput{}, nil
}

func (f *fakeKMSUS1) ListAliases(_ context.Context, _ *kms.ListAliasesInput, _ ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	return &kms.ListAliasesOutput{}, nil
}

func (f *fakeKMSUS1) GetKeyRotationStatus(_ context.Context, _ *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{}, nil
}

func (f *fakeKMSUS1) ListGrants(_ context.Context, _ *kms.ListGrantsInput, _ ...func(*kms.Options)) (*kms.ListGrantsOutput, error) {
	if f.listGrantsErr != nil {
		return nil, f.listGrantsErr
	}
	if f.listGrantsOutput != nil {
		return f.listGrantsOutput, nil
	}
	return &kms.ListGrantsOutput{}, nil
}

func (f *fakeKMSUS1) GetKeyPolicy(_ context.Context, _ *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	if f.getKeyPolicyErr != nil {
		return nil, f.getKeyPolicyErr
	}
	if f.getKeyPolicyOut != nil {
		return f.getKeyPolicyOut, nil
	}
	return &kms.GetKeyPolicyOutput{}, nil
}

// newFakeKMSWithGrants constructs a fakeKMSUS1 returning the supplied grant entries.
func newFakeKMSWithGrants(entries []kmstypes.GrantListEntry) *fakeKMSUS1 {
	return &fakeKMSUS1{
		listGrantsOutput: &kms.ListGrantsOutput{Grants: entries},
	}
}

// ---------------------------------------------------------------------------
// DynamoDB fake — implements DynamoDBAPI
// (DDBListTablesAPI + DDBDescribeTableAPI +
//  DynamoDBDescribeContinuousBackupsAPI +
//  DynamoDBDescribeKinesisStreamingDestinationAPI)
// ---------------------------------------------------------------------------

type fakeDynamoDBUS1 struct {
	kinesisDestOutput *dynamodb.DescribeKinesisStreamingDestinationOutput
	kinesisDestErr    error
}

func (f *fakeDynamoDBUS1) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return &dynamodb.ListTablesOutput{}, nil
}

func (f *fakeDynamoDBUS1) DescribeTable(_ context.Context, _ *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{}, nil
}

func (f *fakeDynamoDBUS1) DescribeContinuousBackups(_ context.Context, _ *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	return &dynamodb.DescribeContinuousBackupsOutput{}, nil
}

func (f *fakeDynamoDBUS1) DescribeKinesisStreamingDestination(_ context.Context, _ *dynamodb.DescribeKinesisStreamingDestinationInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
	if f.kinesisDestErr != nil {
		return nil, f.kinesisDestErr
	}
	if f.kinesisDestOutput != nil {
		return f.kinesisDestOutput, nil
	}
	return &dynamodb.DescribeKinesisStreamingDestinationOutput{}, nil
}

// newFakeDDBWithKinesisDestinations returns a fakeDynamoDBUS1 with the given
// Kinesis streaming destination entries.
func newFakeDDBWithKinesisDestinations(entries []ddbtypes.KinesisDataStreamDestination) *fakeDynamoDBUS1 {
	return &fakeDynamoDBUS1{
		kinesisDestOutput: &dynamodb.DescribeKinesisStreamingDestinationOutput{
			KinesisDataStreamDestinations: entries,
		},
	}
}

// ---------------------------------------------------------------------------
// Lambda fake — implements LambdaAPI
// (LambdaListFunctionsAPI + LambdaListEventSourceMappingsAPI +
//  LambdaGetFunctionAPI + LambdaListTagsAPI)
// ---------------------------------------------------------------------------

type fakeLambdaUS1 struct {
	getFunctionOutput *lambdapkg.GetFunctionOutput
	getFunctionErr    error
}

func (f *fakeLambdaUS1) ListFunctions(_ context.Context, _ *lambdapkg.ListFunctionsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListFunctionsOutput, error) {
	return &lambdapkg.ListFunctionsOutput{}, nil
}

func (f *fakeLambdaUS1) ListEventSourceMappings(_ context.Context, _ *lambdapkg.ListEventSourceMappingsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListEventSourceMappingsOutput, error) {
	return &lambdapkg.ListEventSourceMappingsOutput{}, nil
}

func (f *fakeLambdaUS1) GetFunction(_ context.Context, _ *lambdapkg.GetFunctionInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.GetFunctionOutput, error) {
	if f.getFunctionErr != nil {
		return nil, f.getFunctionErr
	}
	if f.getFunctionOutput != nil {
		return f.getFunctionOutput, nil
	}
	return &lambdapkg.GetFunctionOutput{}, nil
}

func (f *fakeLambdaUS1) ListTags(_ context.Context, _ *lambdapkg.ListTagsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListTagsOutput, error) {
	return &lambdapkg.ListTagsOutput{}, nil
}

// newFakeLambdaWithKMSKey returns a fakeLambdaUS1 whose GetFunction returns
// a FunctionConfiguration with the specified KMSKeyArn.
func newFakeLambdaWithKMSKey(kmsKeyARN string) *fakeLambdaUS1 {
	return &fakeLambdaUS1{
		getFunctionOutput: &lambdapkg.GetFunctionOutput{
			Configuration: &lambdatypes.FunctionConfiguration{
				KMSKeyArn: &kmsKeyARN,
			},
		},
	}
}

// newFakeLambdaWithImageURI returns a fakeLambdaUS1 whose GetFunction returns
// a FunctionConfiguration with the specified ImageUri in Code.
func newFakeLambdaWithImageURI(imageURI string) *fakeLambdaUS1 {
	return &fakeLambdaUS1{
		getFunctionOutput: &lambdapkg.GetFunctionOutput{
			Configuration: &lambdatypes.FunctionConfiguration{
				PackageType: lambdatypes.PackageTypeImage,
			},
			Code: &lambdatypes.FunctionCodeLocation{
				ImageUri: &imageURI,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// CodeArtifact fake — implements CodeArtifactAPI
// (CodeArtifactListRepositoriesAPI + CodeArtifactGetRepositoryPermissionsPolicyAPI +
//  CodeArtifactDescribeRepositoryAPI + CodeArtifactGetDomainPermissionsPolicyAPI +
//  CodeArtifactDescribeDomainAPI)
// ---------------------------------------------------------------------------

type fakeCodeArtifactUS1 struct {
	describeDomainOutput *codeartifact.DescribeDomainOutput
	describeDomainErr    error
}

func (f *fakeCodeArtifactUS1) ListRepositories(_ context.Context, _ *codeartifact.ListRepositoriesInput, _ ...func(*codeartifact.Options)) (*codeartifact.ListRepositoriesOutput, error) {
	return &codeartifact.ListRepositoriesOutput{}, nil
}

func (f *fakeCodeArtifactUS1) GetRepositoryPermissionsPolicy(_ context.Context, _ *codeartifact.GetRepositoryPermissionsPolicyInput, _ ...func(*codeartifact.Options)) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error) {
	return &codeartifact.GetRepositoryPermissionsPolicyOutput{}, nil
}

func (f *fakeCodeArtifactUS1) DescribeRepository(_ context.Context, _ *codeartifact.DescribeRepositoryInput, _ ...func(*codeartifact.Options)) (*codeartifact.DescribeRepositoryOutput, error) {
	return &codeartifact.DescribeRepositoryOutput{}, nil
}

func (f *fakeCodeArtifactUS1) GetDomainPermissionsPolicy(_ context.Context, _ *codeartifact.GetDomainPermissionsPolicyInput, _ ...func(*codeartifact.Options)) (*codeartifact.GetDomainPermissionsPolicyOutput, error) {
	return &codeartifact.GetDomainPermissionsPolicyOutput{}, nil
}

func (f *fakeCodeArtifactUS1) DescribeDomain(_ context.Context, _ *codeartifact.DescribeDomainInput, _ ...func(*codeartifact.Options)) (*codeartifact.DescribeDomainOutput, error) {
	if f.describeDomainErr != nil {
		return nil, f.describeDomainErr
	}
	if f.describeDomainOutput != nil {
		return f.describeDomainOutput, nil
	}
	return &codeartifact.DescribeDomainOutput{}, nil
}

// newFakeCodeArtifactWithKMSKey returns a fakeCodeArtifactUS1 whose DescribeDomain
// returns the supplied KMS key ARN in Domain.EncryptionKey.
func newFakeCodeArtifactWithKMSKey(keyARN string) *fakeCodeArtifactUS1 {
	return &fakeCodeArtifactUS1{
		describeDomainOutput: &codeartifact.DescribeDomainOutput{
			Domain: &catypes.DomainDescription{
				EncryptionKey: &keyARN,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// APIGatewayV2 fake — implements APIGatewayV2API (GetApisAPI + GetStagesAPI)
// AND APIGatewayV2GetIntegrationsAPI so the type assertion in checkApigwKMS
// succeeds.
// ---------------------------------------------------------------------------

type fakeAPIGWV2US1 struct {
	integrations    []apigwv2types.Integration
	integrationsErr error
}

func (f *fakeAPIGWV2US1) GetApis(_ context.Context, _ *apigatewayv2.GetApisInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	return &apigatewayv2.GetApisOutput{}, nil
}

func (f *fakeAPIGWV2US1) GetStages(_ context.Context, _ *apigatewayv2.GetStagesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
	return &apigatewayv2.GetStagesOutput{}, nil
}

func (f *fakeAPIGWV2US1) GetIntegrations(_ context.Context, _ *apigatewayv2.GetIntegrationsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	if f.integrationsErr != nil {
		return nil, f.integrationsErr
	}
	return &apigatewayv2.GetIntegrationsOutput{Items: f.integrations}, nil
}

func (f *fakeAPIGWV2US1) GetDomainNames(_ context.Context, _ *apigatewayv2.GetDomainNamesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetDomainNamesOutput, error) {
	return &apigatewayv2.GetDomainNamesOutput{}, nil
}

func (f *fakeAPIGWV2US1) GetApiMappings(_ context.Context, _ *apigatewayv2.GetApiMappingsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApiMappingsOutput, error) {
	return &apigatewayv2.GetApiMappingsOutput{}, nil
}

// newFakeAPIGWV2WithLambdaIntegration returns a fakeAPIGWV2US1 whose
// GetIntegrations returns a single integration pointing at the given Lambda
// function name.
func newFakeAPIGWV2WithLambdaIntegration(functionName string) *fakeAPIGWV2US1 {
	uri := "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:123456789012:function:" + functionName + "/invocations"
	return &fakeAPIGWV2US1{
		integrations: []apigwv2types.Integration{
			{IntegrationUri: &uri},
		},
	}
}
