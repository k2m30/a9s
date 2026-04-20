package unit_test

// aws_sfn_related_extra_test.go — additional coverage for sfn_related.go.
// Covers checkSFNLambda, checkSFNKMS, and checkSFNRole (found case).
// checkSFNLogs, checkSFNAlarm, checkSFNRole (empty ARN + nil clients), and
// checkSFNEbRule are already covered in aws_sfn_related_test.go.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	sfnsvc "github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// fakeSFNExtra — satisfies awsclient.SFNAPI via embedding.
// Only DescribeStateMachine is overridden; all other methods are inherited stubs.
// ---------------------------------------------------------------------------

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

// sfnExtSrc builds a source Resource for SFN checkers with the given ARN.
func sfnExtSrc(arn string) resource.Resource {
	return resource.Resource{
		ID:   "my-state-machine",
		Name: "my-state-machine",
		Fields: map[string]string{
			"arn": arn,
		},
	}
}

// sfnClientsWithFake wraps fakeSFNExtra in *ServiceClients.
func sfnClientsWithFake(f *fakeSFNExtra) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{SFN: f}
}

// ---------------------------------------------------------------------------
// checkSFNRole — found case (DescribeStateMachine returns a RoleArn)
// ---------------------------------------------------------------------------

func TestRelated_SFN_Role_Found(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow"

	fake := &fakeSFNExtra{
		output: &sfnsvc.DescribeStateMachineOutput{
			RoleArn: aws.String("arn:aws:iam::123456789012:role/sfn-execution-role"),
		},
	}

	checker := sfnCheckerByTarget(t, "role")
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "sfn-execution-role" {
		t.Errorf("ResourceIDs = %v, want [sfn-execution-role]", result.ResourceIDs)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no RoleArn in output)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkSFNKMS — found case and edge cases
// ---------------------------------------------------------------------------

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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != kmsKeyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, kmsKeyID)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no EncryptionConfiguration)", result.Count)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty KmsKeyId)", result.Count)
	}
}

func TestRelated_SFN_KMS_NilClients_ReturnsNegOne(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:nil-clients-kms"

	checker := sfnCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

func TestRelated_SFN_KMS_EmptyARN_ReturnsZero(t *testing.T) {
	src := resource.Resource{
		ID:     "no-arn-machine",
		Fields: map[string]string{},
	}
	checker := sfnCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN short-circuit)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkSFNLambda — found case and edge cases
// ---------------------------------------------------------------------------

func TestRelated_SFN_Lambda_FoundFromResourceARN(t *testing.T) {
	// ASL with a Lambda Task state referencing an ARN.
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
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "process-order" {
		t.Errorf("ResourceIDs = %v, want [process-order]", result.ResourceIDs)
	}
}

func TestRelated_SFN_Lambda_FoundFromParametersFunctionName(t *testing.T) {
	// ASL with Lambda invocation via Parameters.FunctionName.
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
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "validate-input" {
		t.Errorf("ResourceIDs = %v, want [validate-input]", result.ResourceIDs)
	}
}

func TestRelated_SFN_Lambda_DeduplicatesMultipleReferences(t *testing.T) {
	// Two states referencing the same Lambda function → deduplicated to Count=1.
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
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (deduplicated)", result.Count)
	}
}

func TestRelated_SFN_Lambda_MultipleDifferentFunctions(t *testing.T) {
	// Workflow with two different Lambda functions.
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
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs {
		found[id] = true
	}
	for _, name := range []string{"fn-alpha", "fn-beta"} {
		if !found[name] {
			t.Errorf("ResourceIDs %v missing %q", result.ResourceIDs, name)
		}
	}
}

func TestRelated_SFN_Lambda_NoLambdaInDefinition_ReturnsZero(t *testing.T) {
	// Workflow with no Lambda resources at all.
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no Lambda in ASL)", result.Count)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Definition)", result.Count)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty Definition)", result.Count)
	}
}

func TestRelated_SFN_Lambda_NilClients_ReturnsNegOne(t *testing.T) {
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:nil-clients-lambda"

	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, sfnExtSrc(sfnARN), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

func TestRelated_SFN_Lambda_EmptyARN_ReturnsZero(t *testing.T) {
	src := resource.Resource{
		ID:     "no-arn-machine",
		Fields: map[string]string{},
	}
	checker := sfnCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN short-circuit)", result.Count)
	}
}

func TestRelated_SFN_Lambda_StatesIntegrationResourceIgnored(t *testing.T) {
	// "arn:aws:states:::lambda:invoke" is NOT a real Lambda ARN — it should not match.
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
	result := checker(context.Background(), sfnClientsWithFake(fake), sfnExtSrc(sfnARN), resource.ResourceCache{})

	// "arn:aws:states:::lambda:invoke" must not match (not a Lambda ARN).
	// "arn:aws:lambda:...:function:real-function" must match via FunctionName.
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (states::: ARN ignored, FunctionName extracted)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "real-function" {
		t.Errorf("ResourceIDs = %v, want [real-function]", result.ResourceIDs)
	}
}
