// fakes_us1_test.go contains fake implementations of AWS service client
// interfaces used by the US1 batch-1 checker tests. All fakes are in
// package unit_test (external test package) so they do NOT rely on any
// shared state in mocks_test.go (package unit).
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	catypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// The fake client for EventBridgeAPI now lives in fakes_eventbridge_test.go
// (fakeEventBridgeAPI) — see that file's header for the one-fake-per-
// interface convention.

// ---------------------------------------------------------------------------
// Backup fake — implements BackupAPI
// ---------------------------------------------------------------------------

type fakeBackupChecker struct {
	recoveryPointsOutput *backup.ListRecoveryPointsByResourceOutput
	recoveryPointsErr    error
}

func (f *fakeBackupChecker) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return &backup.ListBackupPlansOutput{}, nil
}

func (f *fakeBackupChecker) ListBackupJobs(_ context.Context, _ *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	return &backup.ListBackupJobsOutput{}, nil
}

func (f *fakeBackupChecker) GetBackupPlan(_ context.Context, _ *backup.GetBackupPlanInput, _ ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error) {
	return &backup.GetBackupPlanOutput{}, nil
}

func (f *fakeBackupChecker) ListBackupSelections(_ context.Context, _ *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	return &backup.ListBackupSelectionsOutput{}, nil
}

func (f *fakeBackupChecker) DescribeBackupVault(_ context.Context, _ *backup.DescribeBackupVaultInput, _ ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error) {
	return &backup.DescribeBackupVaultOutput{}, nil
}

func (f *fakeBackupChecker) GetBackupVaultNotifications(_ context.Context, _ *backup.GetBackupVaultNotificationsInput, _ ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error) {
	return &backup.GetBackupVaultNotificationsOutput{}, nil
}

func (f *fakeBackupChecker) ListRecoveryPointsByResource(_ context.Context, _ *backup.ListRecoveryPointsByResourceInput, _ ...func(*backup.Options)) (*backup.ListRecoveryPointsByResourceOutput, error) {
	if f.recoveryPointsErr != nil {
		return nil, f.recoveryPointsErr
	}
	if f.recoveryPointsOutput != nil {
		return f.recoveryPointsOutput, nil
	}
	return &backup.ListRecoveryPointsByResourceOutput{}, nil
}

// newFakeBackupWithRecoveryPoints constructs a fakeBackupChecker with the given
// recovery point entries for ListRecoveryPointsByResource.
func newFakeBackupWithRecoveryPoints(entries []backuptypes.RecoveryPointByResource) *fakeBackupChecker {
	return &fakeBackupChecker{
		recoveryPointsOutput: &backup.ListRecoveryPointsByResourceOutput{
			RecoveryPoints: entries,
		},
	}
}

// ---------------------------------------------------------------------------
// KMS fake — implements KMSAPI
// ---------------------------------------------------------------------------

type fakeKMSChecker struct {
	listGrantsOutput *kms.ListGrantsOutput
	listGrantsErr    error
	getKeyPolicyOut  *kms.GetKeyPolicyOutput
	getKeyPolicyErr  error
}

func (f *fakeKMSChecker) ListKeys(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return &kms.ListKeysOutput{}, nil
}

func (f *fakeKMSChecker) DescribeKey(_ context.Context, _ *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	return &kms.DescribeKeyOutput{}, nil
}

func (f *fakeKMSChecker) ListAliases(_ context.Context, _ *kms.ListAliasesInput, _ ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	return &kms.ListAliasesOutput{}, nil
}

func (f *fakeKMSChecker) GetKeyRotationStatus(_ context.Context, _ *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{}, nil
}

func (f *fakeKMSChecker) ListGrants(_ context.Context, _ *kms.ListGrantsInput, _ ...func(*kms.Options)) (*kms.ListGrantsOutput, error) {
	if f.listGrantsErr != nil {
		return nil, f.listGrantsErr
	}
	if f.listGrantsOutput != nil {
		return f.listGrantsOutput, nil
	}
	return &kms.ListGrantsOutput{}, nil
}

func (f *fakeKMSChecker) GetKeyPolicy(_ context.Context, _ *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	if f.getKeyPolicyErr != nil {
		return nil, f.getKeyPolicyErr
	}
	if f.getKeyPolicyOut != nil {
		return f.getKeyPolicyOut, nil
	}
	return &kms.GetKeyPolicyOutput{}, nil
}

// newFakeKMSWithGrants constructs a fakeKMSChecker returning the supplied grant entries.
func newFakeKMSWithGrants(entries []kmstypes.GrantListEntry) *fakeKMSChecker {
	return &fakeKMSChecker{
		listGrantsOutput: &kms.ListGrantsOutput{Grants: entries},
	}
}

// ---------------------------------------------------------------------------
// DynamoDB fake — implements DynamoDBAPI
// (DDBListTablesAPI + DDBDescribeTableAPI +
//  DynamoDBDescribeContinuousBackupsAPI +
//  DynamoDBDescribeKinesisStreamingDestinationAPI)
// ---------------------------------------------------------------------------

type fakeDynamoDBChecker struct {
	kinesisDestOutput *dynamodb.DescribeKinesisStreamingDestinationOutput
	kinesisDestErr    error
}

func (f *fakeDynamoDBChecker) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return &dynamodb.ListTablesOutput{}, nil
}

func (f *fakeDynamoDBChecker) DescribeTable(_ context.Context, _ *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{}, nil
}

func (f *fakeDynamoDBChecker) DescribeContinuousBackups(_ context.Context, _ *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	return &dynamodb.DescribeContinuousBackupsOutput{}, nil
}

func (f *fakeDynamoDBChecker) DescribeKinesisStreamingDestination(_ context.Context, _ *dynamodb.DescribeKinesisStreamingDestinationInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
	if f.kinesisDestErr != nil {
		return nil, f.kinesisDestErr
	}
	if f.kinesisDestOutput != nil {
		return f.kinesisDestOutput, nil
	}
	return &dynamodb.DescribeKinesisStreamingDestinationOutput{}, nil
}

// ---------------------------------------------------------------------------
// Lambda fake — implements LambdaAPI
// (LambdaListFunctionsAPI + LambdaListEventSourceMappingsAPI +
//  LambdaGetFunctionAPI + LambdaListTagsAPI)
// ---------------------------------------------------------------------------

type fakeLambdaForKMS struct {
	getFunctionOutput *lambdapkg.GetFunctionOutput
	getFunctionErr    error
}

func (f *fakeLambdaForKMS) ListFunctions(_ context.Context, _ *lambdapkg.ListFunctionsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListFunctionsOutput, error) {
	return &lambdapkg.ListFunctionsOutput{}, nil
}

func (f *fakeLambdaForKMS) ListEventSourceMappings(_ context.Context, _ *lambdapkg.ListEventSourceMappingsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListEventSourceMappingsOutput, error) {
	return &lambdapkg.ListEventSourceMappingsOutput{}, nil
}

func (f *fakeLambdaForKMS) GetFunction(_ context.Context, _ *lambdapkg.GetFunctionInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.GetFunctionOutput, error) {
	if f.getFunctionErr != nil {
		return nil, f.getFunctionErr
	}
	if f.getFunctionOutput != nil {
		return f.getFunctionOutput, nil
	}
	return &lambdapkg.GetFunctionOutput{}, nil
}

func (f *fakeLambdaForKMS) ListTags(_ context.Context, _ *lambdapkg.ListTagsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListTagsOutput, error) {
	return &lambdapkg.ListTagsOutput{}, nil
}

// newFakeLambdaWithKMSKey returns a fakeLambdaForKMS whose GetFunction returns
// a FunctionConfiguration with the specified KMSKeyArn.
func newFakeLambdaWithKMSKey(kmsKeyARN string) *fakeLambdaForKMS {
	return &fakeLambdaForKMS{
		getFunctionOutput: &lambdapkg.GetFunctionOutput{
			Configuration: &lambdatypes.FunctionConfiguration{
				KMSKeyArn: &kmsKeyARN,
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

type fakeCodeArtifactChecker struct {
	describeDomainOutput *codeartifact.DescribeDomainOutput
	describeDomainErr    error
}

func (f *fakeCodeArtifactChecker) ListRepositories(_ context.Context, _ *codeartifact.ListRepositoriesInput, _ ...func(*codeartifact.Options)) (*codeartifact.ListRepositoriesOutput, error) {
	return &codeartifact.ListRepositoriesOutput{}, nil
}

func (f *fakeCodeArtifactChecker) GetRepositoryPermissionsPolicy(_ context.Context, _ *codeartifact.GetRepositoryPermissionsPolicyInput, _ ...func(*codeartifact.Options)) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error) {
	return &codeartifact.GetRepositoryPermissionsPolicyOutput{}, nil
}

func (f *fakeCodeArtifactChecker) DescribeRepository(_ context.Context, _ *codeartifact.DescribeRepositoryInput, _ ...func(*codeartifact.Options)) (*codeartifact.DescribeRepositoryOutput, error) {
	return &codeartifact.DescribeRepositoryOutput{}, nil
}

func (f *fakeCodeArtifactChecker) GetDomainPermissionsPolicy(_ context.Context, _ *codeartifact.GetDomainPermissionsPolicyInput, _ ...func(*codeartifact.Options)) (*codeartifact.GetDomainPermissionsPolicyOutput, error) {
	return &codeartifact.GetDomainPermissionsPolicyOutput{}, nil
}

func (f *fakeCodeArtifactChecker) DescribeDomain(_ context.Context, _ *codeartifact.DescribeDomainInput, _ ...func(*codeartifact.Options)) (*codeartifact.DescribeDomainOutput, error) {
	if f.describeDomainErr != nil {
		return nil, f.describeDomainErr
	}
	if f.describeDomainOutput != nil {
		return f.describeDomainOutput, nil
	}
	return &codeartifact.DescribeDomainOutput{}, nil
}

// newFakeCodeArtifactWithKMSKey returns a fakeCodeArtifactChecker whose DescribeDomain
// returns the supplied KMS key ARN in Domain.EncryptionKey.
func newFakeCodeArtifactWithKMSKey(keyARN string) *fakeCodeArtifactChecker {
	return &fakeCodeArtifactChecker{
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

type fakeAPIGWV2Checker struct {
	integrations    []apigwv2types.Integration
	integrationsErr error
}

func (f *fakeAPIGWV2Checker) GetApis(_ context.Context, _ *apigatewayv2.GetApisInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	return &apigatewayv2.GetApisOutput{}, nil
}

func (f *fakeAPIGWV2Checker) GetStages(_ context.Context, _ *apigatewayv2.GetStagesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
	return &apigatewayv2.GetStagesOutput{}, nil
}

func (f *fakeAPIGWV2Checker) GetIntegrations(_ context.Context, _ *apigatewayv2.GetIntegrationsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	if f.integrationsErr != nil {
		return nil, f.integrationsErr
	}
	return &apigatewayv2.GetIntegrationsOutput{Items: f.integrations}, nil
}

func (f *fakeAPIGWV2Checker) GetDomainNames(_ context.Context, _ *apigatewayv2.GetDomainNamesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetDomainNamesOutput, error) {
	return &apigatewayv2.GetDomainNamesOutput{}, nil
}

func (f *fakeAPIGWV2Checker) GetApiMappings(_ context.Context, _ *apigatewayv2.GetApiMappingsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApiMappingsOutput, error) {
	return &apigatewayv2.GetApiMappingsOutput{}, nil
}

func (f *fakeAPIGWV2Checker) GetVpcLinks(_ context.Context, _ *apigatewayv2.GetVpcLinksInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetVpcLinksOutput, error) {
	return &apigatewayv2.GetVpcLinksOutput{}, nil
}

func (f *fakeAPIGWV2Checker) GetAuthorizers(_ context.Context, _ *apigatewayv2.GetAuthorizersInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetAuthorizersOutput, error) {
	return &apigatewayv2.GetAuthorizersOutput{}, nil
}

// newFakeAPIGWV2WithLambdaIntegration returns a fakeAPIGWV2Checker whose
// GetIntegrations returns a single integration pointing at the given Lambda
// function name.
func newFakeAPIGWV2WithLambdaIntegration(functionName string) *fakeAPIGWV2Checker {
	uri := "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:123456789012:function:" + functionName + "/invocations"
	return &fakeAPIGWV2Checker{
		integrations: []apigwv2types.Integration{
			{IntegrationUri: &uri},
		},
	}
}

// ---------------------------------------------------------------------------
// Lambda fake (ESM variant) — implements LambdaAPI
// Returns a configurable list of event source mapping FunctionArns.
// Used by checkSQSLambda tests.
// ---------------------------------------------------------------------------

type fakeLambdaListESM struct {
	// mappings is a slice of FunctionArn strings returned by ListEventSourceMappings.
	mappings []string
}

func (f *fakeLambdaListESM) ListFunctions(_ context.Context, _ *lambdapkg.ListFunctionsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListFunctionsOutput, error) {
	return &lambdapkg.ListFunctionsOutput{}, nil
}

func (f *fakeLambdaListESM) ListEventSourceMappings(_ context.Context, _ *lambdapkg.ListEventSourceMappingsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListEventSourceMappingsOutput, error) {
	var mappings []lambdatypes.EventSourceMappingConfiguration
	for i := range f.mappings {
		arn := f.mappings[i]
		mappings = append(mappings, lambdatypes.EventSourceMappingConfiguration{
			FunctionArn: &arn,
		})
	}
	return &lambdapkg.ListEventSourceMappingsOutput{EventSourceMappings: mappings}, nil
}

func (f *fakeLambdaListESM) GetFunction(_ context.Context, _ *lambdapkg.GetFunctionInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.GetFunctionOutput, error) {
	return &lambdapkg.GetFunctionOutput{}, nil
}

func (f *fakeLambdaListESM) ListTags(_ context.Context, _ *lambdapkg.ListTagsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListTagsOutput, error) {
	return &lambdapkg.ListTagsOutput{}, nil
}

// ---------------------------------------------------------------------------
// Kinesis fake — implements KinesisAPI + KinesisListTagsForStreamAPI +
// KinesisDescribeStreamSummaryAPI via type-assertion upgrade.
// Used by checkKinesisCFN and checkKinesisKMS tests.
// ---------------------------------------------------------------------------

type fakeKinesisWithTagsAndDesc struct {
	// cfnStackName is the aws:cloudformation:stack-name tag value to return.
	// Empty string means no cfn tag.
	cfnStackName string
	// kmsKeyID is the KeyId returned in StreamDescriptionSummary.
	// Empty string means no KMS encryption.
	kmsKeyID string
}

func (f *fakeKinesisWithTagsAndDesc) ListStreams(_ context.Context, _ *kinesis.ListStreamsInput, _ ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error) {
	return &kinesis.ListStreamsOutput{}, nil
}

func (f *fakeKinesisWithTagsAndDesc) ListTagsForStream(_ context.Context, _ *kinesis.ListTagsForStreamInput, _ ...func(*kinesis.Options)) (*kinesis.ListTagsForStreamOutput, error) {
	hasMore := false
	var tags []kinesistypes.Tag
	if f.cfnStackName != "" {
		k := "aws:cloudformation:stack-name"
		v := f.cfnStackName
		tags = append(tags, kinesistypes.Tag{Key: &k, Value: &v})
	}
	return &kinesis.ListTagsForStreamOutput{HasMoreTags: &hasMore, Tags: tags}, nil
}

func (f *fakeKinesisWithTagsAndDesc) DescribeStreamSummary(_ context.Context, _ *kinesis.DescribeStreamSummaryInput, _ ...func(*kinesis.Options)) (*kinesis.DescribeStreamSummaryOutput, error) {
	summary := &kinesistypes.StreamDescriptionSummary{}
	if f.kmsKeyID != "" {
		summary.KeyId = aws.String(f.kmsKeyID)
	}
	return &kinesis.DescribeStreamSummaryOutput{StreamDescriptionSummary: summary}, nil
}
