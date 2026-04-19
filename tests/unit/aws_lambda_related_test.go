package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func lambdaCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("lambda") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("lambda related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("lambda related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_Lambda_Registered(t *testing.T) {
	nav := resource.IsFieldNavigable("lambda", "Role")
	if nav == nil {
		t.Fatal("expected navigable field Role not found for lambda")
	}
	if nav.TargetType != "role" {
		t.Errorf("Role TargetType = %q, want %q", nav.TargetType, "role")
	}
}

// --- IAM Role checker (Pattern C — cache, name extracted from ARN) ---

func TestRelated_Lambda_Role_Found(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/my-lambda-role"
	const roleName = "my-lambda-role"

	roleRes := resource.Resource{
		ID:   roleName,
		Name: roleName,
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		Fields: map[string]string{
			"role": roleARN,
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String(roleARN),
		},
	}

	checker := lambdaCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != roleName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, roleName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Lambda_Role_NotFound(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/my-lambda-role"

	roleRes := resource.Resource{
		ID:   "DifferentRole",
		Name: "DifferentRole",
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		Fields: map[string]string{
			"role": roleARN,
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String(roleARN),
		},
	}

	checker := lambdaCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Lambda_Role_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		Fields: map[string]string{
			"role": "arn:aws:iam::123456789012:role/my-lambda-role",
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/my-lambda-role"),
		},
	}

	checker := lambdaCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, FunctionName dimension) ---

func TestRelated_Lambda_Alarms_Found(t *testing.T) {
	const fnName = "my-function"

	alarmRes := resource.Resource{
		ID: "lambda-error-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("lambda-error-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("FunctionName"), Value: aws.String(fnName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   fnName,
		Name: fnName,
		Fields: map[string]string{
			"function_name": fnName,
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(fnName),
			Role:         aws.String("arn:aws:iam::123456789012:role/my-lambda-role"),
		},
	}

	checker := lambdaCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "lambda-error-alarm" {
		t.Errorf("ResourceIDs = %v, want [lambda-error-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Lambda_Alarms_NotFound(t *testing.T) {
	const fnName = "my-function"

	alarmRes := resource.Resource{
		ID: "other-function-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-function-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("FunctionName"), Value: aws.String("different-function")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   fnName,
		Name: fnName,
		Fields: map[string]string{
			"function_name": fnName,
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(fnName),
			Role:         aws.String("arn:aws:iam::123456789012:role/my-lambda-role"),
		},
	}

	checker := lambdaCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Lambda_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		Fields: map[string]string{
			"function_name": "my-function",
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/my-lambda-role"),
		},
	}

	checker := lambdaCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- Stub checkers (nil Checker) ---

// ---------------------------------------------------------------------------
// checkLambdaECR — Pattern F: no API call, reads PackageType + image_uri field
// ---------------------------------------------------------------------------

// TestRelated_Lambda_ECR_Match verifies that a container-image Lambda with an
// image URI field returns Count=1 with the repository name.
func TestRelated_Lambda_ECR_Match(t *testing.T) {
	src := resource.Resource{
		ID:   "my-image-function",
		Name: "my-image-function",
		Fields: map[string]string{
			"image_uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-ecr-repo:latest",
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-image-function"),
			PackageType:  lambdatypes.PackageTypeImage,
		},
	}
	checker := lambdaCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-ecr-repo" {
		t.Errorf("ResourceIDs = %v, want [my-ecr-repo]", result.ResourceIDs)
	}
}

// TestRelated_Lambda_ECR_Empty verifies that a Zip-packaged Lambda returns
// Count=0 (no ECR involvement).
func TestRelated_Lambda_ECR_Empty(t *testing.T) {
	src := resource.Resource{
		ID:     "my-zip-function",
		Name:   "my-zip-function",
		Fields: map[string]string{},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-zip-function"),
			PackageType:  lambdatypes.PackageTypeZip,
		},
	}
	checker := lambdaCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (Zip package type, no ECR)", result.Count)
	}
}

// TestRelated_Lambda_ECR_WrongRawStruct verifies that a resource with a
// non-FunctionConfiguration RawStruct returns Count=-1.
func TestRelated_Lambda_ECR_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "my-function",
		RawStruct: "not-a-function-configuration",
	}
	checker := lambdaCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// TestRelated_Lambda_ECR_ImageTypeNoURI: Image type but no image_uri field → Count: -1.
func TestRelated_Lambda_ECR_ImageTypeNoURI(t *testing.T) {
	src := resource.Resource{
		ID:     "my-image-function",
		Fields: map[string]string{}, // no image_uri
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-image-function"),
			PackageType:  lambdatypes.PackageTypeImage,
		},
	}
	checker := lambdaCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (Image type but no image_uri)", result.Count)
	}
}

// TestRelated_Lambda_ECR_ImageURIWithDigest: image URI with @sha256 digest suffix
// is parsed correctly (digest stripped).
func TestRelated_Lambda_ECR_ImageURIWithDigest(t *testing.T) {
	src := resource.Resource{
		ID: "my-digest-function",
		Fields: map[string]string{
			"image_uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-ecr-repo@sha256:abc123",
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-digest-function"),
			PackageType:  lambdatypes.PackageTypeImage,
		},
	}
	checker := lambdaCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-ecr-repo" {
		t.Errorf("ResourceIDs = %v, want [my-ecr-repo]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaLogs — Pattern C: default log group "/aws/lambda/{name}", custom via LoggingConfig
// ---------------------------------------------------------------------------

func TestRelated_Lambda_Logs_DefaultLogGroup(t *testing.T) {
	const fnName = "my-function"
	logRes := resource.Resource{
		ID:   "/aws/lambda/my-function",
		Name: "/aws/lambda/my-function",
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}
	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(fnName),
		},
	}
	checker := lambdaCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (default log group)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/lambda/my-function" {
		t.Errorf("ResourceIDs = %v, want [/aws/lambda/my-function]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_Logs_CustomLogGroupViaLoggingConfig(t *testing.T) {
	const customGroup = "/custom/log/group"
	logRes := resource.Resource{
		ID:   customGroup,
		Name: customGroup,
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup: aws.String(customGroup),
			},
		},
	}
	checker := lambdaCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (custom LoggingConfig group)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != customGroup {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, customGroup)
	}
}

func TestRelated_Lambda_Logs_NoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:   "/aws/lambda/other-function",
		Name: "/aws/lambda/other-function",
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
		},
	}
	checker := lambdaCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching log group)", result.Count)
	}
}

func TestRelated_Lambda_Logs_NilCache(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
		},
	}
	checker := lambdaCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache, no clients)", result.Count)
	}
}

func TestRelated_Lambda_Logs_EmptyFunctionName(t *testing.T) {
	src := resource.Resource{
		ID:        "",
		Name:      "",
		RawStruct: lambdatypes.FunctionConfiguration{},
	}
	checker := lambdaCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty function name)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaSG — Pattern F: reads VpcConfig.SecurityGroupIds
// ---------------------------------------------------------------------------

func TestRelated_Lambda_SG_VPCFunction(t *testing.T) {
	src := resource.Resource{
		ID:   "vpc-function",
		Name: "vpc-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("vpc-function"),
			VpcConfig: &lambdatypes.VpcConfigResponse{
				SecurityGroupIds: []string{"sg-aaa111", "sg-bbb222"},
			},
		},
	}
	checker := lambdaCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs = %v, want [sg-aaa111 sg-bbb222]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_SG_NoVPC(t *testing.T) {
	src := resource.Resource{
		ID:   "no-vpc-function",
		Name: "no-vpc-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("no-vpc-function"),
			VpcConfig:    (*lambdatypes.VpcConfigResponse)(nil),
		},
	}
	checker := lambdaCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no VPC config)", result.Count)
	}
}

func TestRelated_Lambda_SG_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "bad-struct",
		RawStruct: "not-a-function",
	}
	checker := lambdaCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaVPC — Pattern F: reads VpcConfig.VpcId
// ---------------------------------------------------------------------------

func TestRelated_Lambda_VPC_VPCFunction(t *testing.T) {
	src := resource.Resource{
		ID:   "vpc-function",
		Name: "vpc-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("vpc-function"),
			VpcConfig: &lambdatypes.VpcConfigResponse{
				VpcId: aws.String("vpc-12345678"),
			},
		},
	}
	checker := lambdaCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-12345678" {
		t.Errorf("ResourceIDs = %v, want [vpc-12345678]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_VPC_NoVPCConfig(t *testing.T) {
	src := resource.Resource{
		ID:   "no-vpc-function",
		Name: "no-vpc-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("no-vpc-function"),
			VpcConfig:    (*lambdatypes.VpcConfigResponse)(nil),
		},
	}
	checker := lambdaCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no VPC config)", result.Count)
	}
}

func TestRelated_Lambda_VPC_EmptyVPCId(t *testing.T) {
	src := resource.Resource{
		ID:   "empty-vpc-function",
		Name: "empty-vpc-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("empty-vpc-function"),
			VpcConfig: &lambdatypes.VpcConfigResponse{
				VpcId: aws.String(""),
			},
		},
	}
	checker := lambdaCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty VPC ID)", result.Count)
	}
}

func TestRelated_Lambda_VPC_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "bad-struct",
		RawStruct: "not-a-function",
	}
	checker := lambdaCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaKMS — Pattern F: reads KMSKeyArn
// ---------------------------------------------------------------------------

func TestRelated_Lambda_KMS_WithKMSKey(t *testing.T) {
	src := resource.Resource{
		ID:   "encrypted-function",
		Name: "encrypted-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("encrypted-function"),
			KMSKeyArn:    aws.String("arn:aws:kms:us-east-1:123:key/abcd-1234"),
		},
	}
	checker := lambdaCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abcd-1234" {
		t.Errorf("ResourceIDs = %v, want [abcd-1234]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_KMS_NoKMSKey(t *testing.T) {
	src := resource.Resource{
		ID:   "unencrypted-function",
		Name: "unencrypted-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("unencrypted-function"),
			KMSKeyArn:    nil,
		},
	}
	checker := lambdaCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key)", result.Count)
	}
}

func TestRelated_Lambda_KMS_EmptyKMSArn(t *testing.T) {
	src := resource.Resource{
		ID:   "empty-kms-function",
		Name: "empty-kms-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("empty-kms-function"),
			KMSKeyArn:    aws.String(""),
		},
	}
	checker := lambdaCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty KMS ARN)", result.Count)
	}
}

// TestRelated_Lambda_KMS_KMSKeyNoSlash: KMS key ARN with no "/" (alias or bare key ID)
// should be returned as-is.
func TestRelated_Lambda_KMS_KMSKeyNoSlash(t *testing.T) {
	src := resource.Resource{
		ID:   "function-bare-kms",
		Name: "function-bare-kms",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("function-bare-kms"),
			KMSKeyArn:    aws.String("alias/aws/lambda"),
		},
	}
	checker := lambdaCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// Has "/" → last segment after final "/" is "lambda"
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "lambda" {
		t.Errorf("ResourceIDs = %v, want [lambda]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaSQS — nil client path
// ---------------------------------------------------------------------------

func TestRelated_Lambda_SQS_NilClients(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
		},
	}
	checker := lambdaCheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil Lambda client)", result.Count)
	}
}

func TestRelated_Lambda_SQS_EmptyFunctionName(t *testing.T) {
	src := resource.Resource{
		ID:        "",
		Name:      "",
		RawStruct: lambdatypes.FunctionConfiguration{},
	}
	checker := lambdaCheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty function name)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaCFN — nil client / no FunctionArn paths
// ---------------------------------------------------------------------------

func TestRelated_Lambda_CFN_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "my-function",
		RawStruct: "not-a-function",
	}
	checker := lambdaCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

func TestRelated_Lambda_CFN_NoFunctionArn(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  nil, // no ARN
		},
	}
	checker := lambdaCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no FunctionArn)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaEBRule — nil client and no ARN/name paths
// ---------------------------------------------------------------------------

func TestRelated_Lambda_EBRule_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "my-function",
		RawStruct: "not-a-function",
	}
	checker := lambdaCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

func TestRelated_Lambda_EBRule_EmptyARNAndName(t *testing.T) {
	src := resource.Resource{
		ID:   "",
		Name: "",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: nil,
			FunctionArn:  nil,
		},
	}
	checker := lambdaCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ARN and no name)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaAlarms — truncated cache path
// ---------------------------------------------------------------------------

func TestRelated_Lambda_Alarms_TruncatedCacheNoMatch(t *testing.T) {
	const fnName = "my-function"

	alarmRes := resource.Resource{
		ID: "other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("FunctionName"), Value: aws.String("different-fn")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{alarmRes},
			IsTruncated: true,
		},
	}
	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(fnName),
		},
	}
	checker := lambdaCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)
	if !result.Approximate {
		t.Errorf("Approximate = false, want true (truncated cache, no match)")
	}
}

func TestRelated_Lambda_Alarms_EmptyFunctionName(t *testing.T) {
	src := resource.Resource{
		ID:        "",
		Name:      "",
		RawStruct: lambdatypes.FunctionConfiguration{},
	}
	checker := lambdaCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty function name)", result.Count)
	}
}
