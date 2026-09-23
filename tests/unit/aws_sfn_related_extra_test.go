package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	lambdasvc "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	sfnsvc "github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type fakeSFNExtra struct {
	awsclient.SFNAPI
	output *sfnsvc.DescribeStateMachineOutput
	err    error
}

func (f *fakeSFNExtra) DescribeStateMachine(_ context.Context, _ *sfnsvc.DescribeStateMachineInput, _ ...func(*sfnsvc.Options)) (*sfnsvc.DescribeStateMachineOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.output != nil {
		return f.output, nil
	}
	return &sfnsvc.DescribeStateMachineOutput{}, nil
}

func sfnExtSrc(arn string) resource.Resource {
	return resource.Resource{
		ID:   "my-state-machine",
		Name: "my-state-machine",
		Fields: map[string]string{
			"arn": arn,
		},
	}
}

func sfnClientsWithFake(f *fakeSFNExtra) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{SFN: f}
}

type sfnLambdaList struct{ names []string }

func (l sfnLambdaList) ListFunctions(context.Context, *lambdasvc.ListFunctionsInput, ...func(*lambdasvc.Options)) (*lambdasvc.ListFunctionsOutput, error) {
	out := &lambdasvc.ListFunctionsOutput{}
	for _, n := range l.names {
		out.Functions = append(out.Functions, lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(n),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + n),
			Runtime:      lambdatypes.RuntimePython312,
		})
	}
	return out, nil
}

// sfnLambdaCache is the loaded lambda list holding names, as the lambda
// fetcher emits it.
func sfnLambdaCache(t *testing.T, names ...string) resource.ResourceCache {
	t.Helper()
	page, err := awsclient.FetchLambdaFunctionsPage(context.Background(), sfnLambdaList{names: names}, "")
	if err != nil {
		t.Fatalf("lambda list: %v", err)
	}
	return resource.ResourceCache{"lambda": resource.ResourceCacheEntry{Resources: page.Resources}}
}

func TestRelated_SFN_Role_Found(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow"

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			RoleArn: aws.String("arn:aws:iam::123456789012:role/sfn-execution-role"),
		},
	}

	checker := sfnCheckerByTarget(t, "role")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "sfn-execution-role" {
		t.Errorf("ResourceIDs = %v, want [sfn-execution-role]", result.ResourceIDs())
	}
}

func TestRelated_SFN_Role_NoRoleArn_ReturnsZero(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:no-role-machine"

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			RoleArn: nil,
		},
	}

	checker := sfnCheckerByTarget(t, "role")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no RoleArn in output)", result.Count())
	}
}

func TestRelated_SFN_KMS_Found(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:kms-workflow"
	const kmsKeyID = "mrk-0123456789abcdef0123456789abcdef"

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			EncryptionConfiguration: &sfntypes.EncryptionConfiguration{
				KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/" + kmsKeyID),
			},
		},
	}

	checker := sfnCheckerByTarget(t, "kms")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != kmsKeyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), kmsKeyID)
	}
}

func TestRelated_SFN_KMS_NoEncryptionConfig_ReturnsZero(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:no-kms-workflow"

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			EncryptionConfiguration: nil,
		},
	}

	checker := sfnCheckerByTarget(t, "kms")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no EncryptionConfiguration)", result.Count())
	}
}

func TestRelated_SFN_KMS_EmptyKmsKeyId_ReturnsZero(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:empty-kms-workflow"

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			EncryptionConfiguration: &sfntypes.EncryptionConfiguration{
				KmsKeyId: aws.String(""),
			},
		},
	}

	checker := sfnCheckerByTarget(t, "kms")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty KmsKeyId)", result.Count())
	}
}

func TestRelated_SFN_KMS_NilClients_ReturnsNegOne(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:nil-clients-kms"

	checker := sfnCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

func TestRelated_SFN_KMS_EmptyARN_ReturnsZero(t *testing.T) {
	src := resource.Resource{
		ID:     "no-arn-machine",
		Fields: map[string]string{},
	}
	checker := sfnCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN short-circuit)", result.Count())
	}
}

func TestRelated_SFN_Lambda_FoundFromResourceARN(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:lambda-workflow"
	definition := `{
		"Comment": "A simple workflow",
		"StartAt": "InvokeFunction",
		"States": {
			"InvokeFunction": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:123456789012:function:process-order",
				"End": true
			}
		}
	}`

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			Definition: aws.String(definition),
		},
	}

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), sfnLambdaCache(t, "process-order"))

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "process-order" {
		t.Errorf("ResourceIDs = %v, want [process-order]", result.ResourceIDs())
	}
}

// A function named in the definition is counted against the loaded lambda
// list; with no list to read, the count is unknown rather than the names as
// written.
func TestRelated_SFN_Lambda_UnknownWithoutALambdaList(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:lambda-workflow"
	fake := &fakeSFNExtra{output: &sfnsvc.DescribeStateMachineOutput{Definition: aws.String(`{
		"StartAt": "InvokeFunction",
		"States": {"InvokeFunction": {"Type": "Task", "Resource": "arn:aws:lambda:us-east-1:123456789012:function:process-order", "End": true}}
	}`)}}

	result := sfnCheckerByTarget(t, "lambda")(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})
	if result.EffectiveState() != domain.RelatedUnknown && result.EffectiveState() != domain.RelatedError {
		t.Errorf("state = %v ids %v, want unknown with no lambda list", result.EffectiveState(), result.ResourceIDs())
	}
}

func TestRelated_SFN_Lambda_FoundFromParametersFunctionName(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:param-fn-workflow"
	definition := `{
		"StartAt": "InvokeLambda",
		"States": {
			"InvokeLambda": {
				"Type": "Task",
				"Resource": "arn:aws:states:::lambda:invoke",
				"Parameters": {
					"FunctionName": "arn:aws:lambda:us-east-1:123456789012:function:validate-input",
					"Payload.$": "$"
				},
				"End": true
			}
		}
	}`

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			Definition: aws.String(definition),
		},
	}

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), sfnLambdaCache(t, "validate-input"))

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "validate-input" {
		t.Errorf("ResourceIDs = %v, want [validate-input]", result.ResourceIDs())
	}
}

func TestRelated_SFN_Lambda_DeduplicatesMultipleReferences(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:dedup-workflow"
	definition := `{
		"StartAt": "Step1",
		"States": {
			"Step1": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:123456789012:function:shared-fn",
				"Next": "Step2"
			},
			"Step2": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:123456789012:function:shared-fn",
				"End": true
			}
		}
	}`

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			Definition: aws.String(definition),
		},
	}

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), sfnLambdaCache(t, "shared-fn"))

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (deduplicated)", result.Count())
	}
}

func TestRelated_SFN_Lambda_MultipleDifferentFunctions(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:multi-fn-workflow"
	definition := `{
		"StartAt": "Step1",
		"States": {
			"Step1": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:123456789012:function:fn-alpha",
				"Next": "Step2"
			},
			"Step2": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:123456789012:function:fn-beta",
				"End": true
			}
		}
	}`

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			Definition: aws.String(definition),
		},
	}

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), sfnLambdaCache(t, "fn-alpha", "fn-beta"))

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		found[id] = true
	}
	for _, name := range []string{"fn-alpha", "fn-beta"} {
		if !found[name] {
			t.Errorf("ResourceIDs %v missing %q", result.ResourceIDs(), name)
		}
	}
}

func TestRelated_SFN_Lambda_NoLambdaInDefinition_ReturnsZero(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:no-lambda-workflow"
	definition := `{
		"StartAt": "Wait",
		"States": {
			"Wait": {
				"Type": "Wait",
				"Seconds": 10,
				"End": true
			}
		}
	}`

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			Definition: aws.String(definition),
		},
	}

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no Lambda in ASL)", result.Count())
	}
}

func TestRelated_SFN_Lambda_NilDefinition_ReturnsZero(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:nil-def-workflow"

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			Definition: nil,
		},
	}

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nil Definition)", result.Count())
	}
}

func TestRelated_SFN_Lambda_EmptyDefinition_ReturnsZero(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:empty-def-workflow"

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			Definition: aws.String(""),
		},
	}

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty Definition)", result.Count())
	}
}

func TestRelated_SFN_Lambda_NilClients_ReturnsNegOne(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:nil-clients-lambda"

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

func TestRelated_SFN_Lambda_EmptyARN_ReturnsZero(t *testing.T) {
	src := resource.Resource{
		ID:     "no-arn-machine",
		Fields: map[string]string{},
	}
	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN short-circuit)", result.Count())
	}
}

func TestRelated_SFN_Lambda_StatesIntegrationResourceIgnored(t *testing.T) {
	// arn:aws:states:::lambda:invoke is a Step Functions service-integration ARN,
	// not a Lambda function ARN.
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:states-integration-workflow"
	definition := `{
		"StartAt": "InvokeLambda",
		"States": {
			"InvokeLambda": {
				"Type": "Task",
				"Resource": "arn:aws:states:::lambda:invoke",
				"Parameters": {
					"FunctionName": "arn:aws:lambda:us-east-1:123456789012:function:real-function"
				},
				"End": true
			}
		}
	}`

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			Definition: aws.String(definition),
		},
	}

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), sfnLambdaCache(t, "real-function"))

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (states::: ARN ignored, FunctionName extracted)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "real-function" {
		t.Errorf("ResourceIDs = %v, want [real-function]", result.ResourceIDs())
	}
}
