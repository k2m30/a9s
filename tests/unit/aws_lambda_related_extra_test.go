package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// fakeLambdaWithESMArns — full LambdaAPI that returns event source mappings
// with configurable EventSourceArns. Used by DDB/Kinesis/MSK checker tests.
// ---------------------------------------------------------------------------

type fakeLambdaWithESMArns struct {
	eventSourceArns []string
}

func (f *fakeLambdaWithESMArns) ListFunctions(_ context.Context, _ *lambdapkg.ListFunctionsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListFunctionsOutput, error) {
	return &lambdapkg.ListFunctionsOutput{}, nil
}

func (f *fakeLambdaWithESMArns) ListEventSourceMappings(_ context.Context, _ *lambdapkg.ListEventSourceMappingsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListEventSourceMappingsOutput, error) {
	var mappings []lambdatypes.EventSourceMappingConfiguration
	for i := range f.eventSourceArns {
		arn := f.eventSourceArns[i]
		mappings = append(mappings, lambdatypes.EventSourceMappingConfiguration{
			EventSourceArn: &arn,
		})
	}
	return &lambdapkg.ListEventSourceMappingsOutput{EventSourceMappings: mappings}, nil
}

func (f *fakeLambdaWithESMArns) GetFunction(_ context.Context, _ *lambdapkg.GetFunctionInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.GetFunctionOutput, error) {
	return &lambdapkg.GetFunctionOutput{}, nil
}

func (f *fakeLambdaWithESMArns) ListTags(_ context.Context, _ *lambdapkg.ListTagsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListTagsOutput, error) {
	return &lambdapkg.ListTagsOutput{}, nil
}

// lambdaExtraCheckerByTarget returns the RelatedChecker registered under "lambda"
// for the given target type. Shared with aws_lambda_related_test.go via lambdaCheckerByTarget —
// but we duplicate the helper here to avoid test-file dependencies.
func lambdaExtraCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
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

// ---------------------------------------------------------------------------
// checkLambdaSubnet — Pattern F: reads VpcConfig.SubnetIds
// ---------------------------------------------------------------------------

func TestRelated_Lambda_Subnet_VPCFunction(t *testing.T) {
	src := resource.Resource{
		ID:   "vpc-function",
		Name: "vpc-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("vpc-function"),
			VpcConfig: &lambdatypes.VpcConfigResponse{
				SubnetIds: []string{"subnet-aaa111", "subnet-bbb222"},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs = %v, want [subnet-aaa111 subnet-bbb222]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_Subnet_NoVPCConfig(t *testing.T) {
	src := resource.Resource{
		ID:   "no-vpc-function",
		Name: "no-vpc-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("no-vpc-function"),
			VpcConfig:    (*lambdatypes.VpcConfigResponse)(nil),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no VPC config)", result.Count)
	}
}

func TestRelated_Lambda_Subnet_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "bad-struct",
		RawStruct: "not-a-function",
	}
	checker := lambdaExtraCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

func TestRelated_Lambda_Subnet_EmptySubnetIDs(t *testing.T) {
	src := resource.Resource{
		ID:   "empty-subnets-function",
		Name: "empty-subnets-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("empty-subnets-function"),
			VpcConfig: &lambdatypes.VpcConfigResponse{
				SubnetIds: []string{"", "subnet-valid"},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// Empty string subnet ID is skipped; only "subnet-valid" is returned.
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (empty subnet ID skipped)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "subnet-valid" {
		t.Errorf("ResourceIDs = %v, want [subnet-valid]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaEFS — Pattern F: reads FileSystemConfigs ARNs
// ---------------------------------------------------------------------------

func TestRelated_Lambda_EFS_WithAccessPoints(t *testing.T) {
	src := resource.Resource{
		ID:   "efs-function",
		Name: "efs-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("efs-function"),
			FileSystemConfigs: []lambdatypes.FileSystemConfig{
				{Arn: aws.String("arn:aws:elasticfilesystem:us-east-1:123:access-point/fsap-aaa111")},
				{Arn: aws.String("arn:aws:elasticfilesystem:us-east-1:123:access-point/fsap-bbb222")},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "efs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs = %v, want [fsap-aaa111 fsap-bbb222]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_EFS_NoFileSystemConfigs(t *testing.T) {
	src := resource.Resource{
		ID:   "no-efs-function",
		Name: "no-efs-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName:      aws.String("no-efs-function"),
			FileSystemConfigs: nil,
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "efs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no EFS configs)", result.Count)
	}
}

func TestRelated_Lambda_EFS_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "bad-struct",
		RawStruct: "not-a-function",
	}
	checker := lambdaExtraCheckerByTarget(t, "efs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

func TestRelated_Lambda_EFS_NilARN(t *testing.T) {
	src := resource.Resource{
		ID:   "nil-arn-function",
		Name: "nil-arn-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("nil-arn-function"),
			FileSystemConfigs: []lambdatypes.FileSystemConfig{
				{Arn: nil}, // nil ARN should be skipped
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "efs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil ARN skipped)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaAPIGW — cache scan (name/tag heuristic)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_APIGW_MatchByName(t *testing.T) {
	const fnName = "my-function"
	apiRes := resource.Resource{
		ID:   "api-id-123",
		Name: "api-for-my-function",
		RawStruct: apigwtypes.Api{
			ApiId: aws.String("api-id-123"),
			Name:  aws.String("api-for-my-function"),
		},
	}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{apiRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (name contains function name)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "api-id-123" {
		t.Errorf("ResourceIDs = %v, want [api-id-123]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_APIGW_MatchByTag(t *testing.T) {
	const fnName = "my-tagged-fn"
	apiRes := resource.Resource{
		ID:   "tagged-api-id",
		Name: "tagged-api",
		RawStruct: apigwtypes.Api{
			ApiId: aws.String("tagged-api-id"),
			Name:  aws.String("tagged-api"),
			Tags:  map[string]string{fnName: "true"},
		},
	}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{apiRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (tag key matches function name)", result.Count)
	}
}

func TestRelated_Lambda_APIGW_NoMatch(t *testing.T) {
	const fnName = "my-function"
	apiRes := resource.Resource{
		ID:   "api-id-456",
		Name: "api-id-456",
		RawStruct: apigwtypes.Api{
			ApiId: aws.String("api-id-456"),
			Name:  aws.String("some-unrelated-api"),
		},
	}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{apiRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no name/tag match)", result.Count)
	}
}

func TestRelated_Lambda_APIGW_NilCache(t *testing.T) {
	src := resource.Resource{ID: "my-function", Name: "my-function"}
	checker := lambdaExtraCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache, no clients)", result.Count)
	}
}

func TestRelated_Lambda_APIGW_EmptyFunctionName(t *testing.T) {
	src := resource.Resource{ID: "", Name: ""}
	checker := lambdaExtraCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty function name)", result.Count)
	}
}

func TestRelated_Lambda_APIGW_TruncatedCacheNoMatch(t *testing.T) {
	const fnName = "my-function"
	apiRes := resource.Resource{
		ID:   "unrelated-api",
		Name: "unrelated-api",
		RawStruct: apigwtypes.Api{
			Name: aws.String("unrelated-api"),
		},
	}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{apiRes},
			IsTruncated: true,
		},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, src, cache)
	if !result.Approximate {
		t.Errorf("Approximate = false, want true (truncated cache, no match)")
	}
}

// ---------------------------------------------------------------------------
// checkLambdaCF — cache scan (lambda_function_arn field match)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_CF_MatchByField(t *testing.T) {
	const fnARN = "arn:aws:lambda:us-east-1:123:function:my-edge-fn"

	cfRes := resource.Resource{
		ID:   "E1EDGE123456",
		Name: "my-cf-dist",
		Fields: map[string]string{
			"lambda_function_arn": fnARN,
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{cfRes}},
	}
	src := resource.Resource{
		ID:   "my-edge-fn",
		Name: "my-edge-fn",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-edge-fn"),
			FunctionArn:  aws.String(fnARN),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "E1EDGE123456" {
		t.Errorf("ResourceIDs = %v, want [E1EDGE123456]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_CF_NoMatch(t *testing.T) {
	const fnARN = "arn:aws:lambda:us-east-1:123:function:my-edge-fn"

	cfRes := resource.Resource{
		ID:   "E1OTHER9999",
		Name: "other-dist",
		Fields: map[string]string{
			"lambda_function_arn": "arn:aws:lambda:us-east-1:123:function:other-fn",
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{cfRes}},
	}
	src := resource.Resource{
		ID:   "my-edge-fn",
		Name: "my-edge-fn",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-edge-fn"),
			FunctionArn:  aws.String(fnARN),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ARN match)", result.Count)
	}
}

func TestRelated_Lambda_CF_EmptyFunctionARN(t *testing.T) {
	src := resource.Resource{
		ID:   "no-arn-function",
		Name: "no-arn-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("no-arn-function"),
			FunctionArn:  nil,
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no FunctionArn → skip)", result.Count)
	}
}

func TestRelated_Lambda_CF_NilCache(t *testing.T) {
	src := resource.Resource{
		ID:   "my-edge-fn",
		Name: "my-edge-fn",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-edge-fn"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123:function:my-edge-fn"),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache, no clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaDDB — nil client path only (live API required for full coverage)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_DDB_NilClients(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "ddb")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil Lambda client)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaCTEvents — cache scan (ResourceName match)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_CTEvents_MatchByExactName(t *testing.T) {
	const fnName = "my-function"
	evRes := resource.Resource{
		ID:   "event-id-abc123",
		Name: "event-id-abc123",
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String("event-id-abc123"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String(fnName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (exact ResourceName match)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "event-id-abc123" {
		t.Errorf("ResourceIDs = %v, want [event-id-abc123]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_CTEvents_MatchByARNSuffix(t *testing.T) {
	const fnName = "my-function"
	evRes := resource.Resource{
		ID:   "event-id-def456",
		Name: "event-id-def456",
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String("event-id-def456"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("arn:aws:lambda:us-east-1:123:function:" + fnName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (ARN suffix match)", result.Count)
	}
}

func TestRelated_Lambda_CTEvents_NoMatch(t *testing.T) {
	const fnName = "my-function"
	evRes := resource.Resource{
		ID:   "event-id-ghi789",
		Name: "event-id-ghi789",
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String("event-id-ghi789"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("other-function")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching event)", result.Count)
	}
}

func TestRelated_Lambda_CTEvents_NilCache(t *testing.T) {
	src := resource.Resource{ID: "my-function", Name: "my-function"}
	checker := lambdaExtraCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

func TestRelated_Lambda_CTEvents_EmptyFunctionName(t *testing.T) {
	src := resource.Resource{ID: "", Name: ""}
	checker := lambdaExtraCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty function name)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaTG — cache scan (target_type=lambda, name/arn match)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_TG_MatchByFunctionName(t *testing.T) {
	const fnName = "my-function"
	tgRes := resource.Resource{
		ID:   "tg-abc123",
		Name: "tg-abc123",
		Fields: map[string]string{
			"target_type":          "lambda",
			"lambda_function_name": fnName,
		},
	}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (lambda_function_name match)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "tg-abc123" {
		t.Errorf("ResourceIDs = %v, want [tg-abc123]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_TG_MatchByARNSuffix(t *testing.T) {
	const fnName = "my-function"
	tgRes := resource.Resource{
		ID:   "tg-def456",
		Name: "tg-def456",
		Fields: map[string]string{
			"target_type": "lambda",
			"target_arn":  "arn:aws:lambda:us-east-1:123:function:" + fnName,
		},
	}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (target_arn suffix match)", result.Count)
	}
}

func TestRelated_Lambda_TG_WrongTargetType(t *testing.T) {
	const fnName = "my-function"
	tgRes := resource.Resource{
		ID:   "tg-not-lambda",
		Name: "tg-not-lambda",
		Fields: map[string]string{
			"target_type":          "instance", // not lambda
			"lambda_function_name": fnName,
		},
	}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (target_type != lambda)", result.Count)
	}
}

func TestRelated_Lambda_TG_NilCache(t *testing.T) {
	src := resource.Resource{ID: "my-function", Name: "my-function"}
	checker := lambdaExtraCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaSNS — sns-sub cache scan → topic ARNs
// ---------------------------------------------------------------------------

func TestRelated_Lambda_SNS_MatchByFnARN(t *testing.T) {
	const fnARN = "arn:aws:lambda:us-east-1:123:function:my-function"
	const topicARN = "arn:aws:sns:us-east-1:123:my-topic"
	subRes := resource.Resource{
		ID:   "sub-abc123",
		Name: "sub-abc123",
		Fields: map[string]string{
			"protocol":  "lambda",
			"endpoint":  fnARN,
			"topic_arn": topicARN,
		},
	}
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{subRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String(fnARN),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (endpoint matches FunctionArn)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != topicARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, topicARN)
	}
}

func TestRelated_Lambda_SNS_MatchByFnNameSuffix(t *testing.T) {
	const fnName = "my-function"
	const topicARN = "arn:aws:sns:us-east-1:123:my-topic"
	subRes := resource.Resource{
		ID:   "sub-bbb",
		Name: "sub-bbb",
		Fields: map[string]string{
			"protocol":  "lambda",
			"endpoint":  "arn:aws:lambda:us-east-1:123:function:" + fnName,
			"topic_arn": topicARN,
		},
	}
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{subRes}},
	}
	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(fnName),
			FunctionArn:  nil, // no ARN — use name suffix match
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (suffix match on function name)", result.Count)
	}
}

func TestRelated_Lambda_SNS_WrongProtocol(t *testing.T) {
	subRes := resource.Resource{
		ID:   "sub-ccc",
		Name: "sub-ccc",
		Fields: map[string]string{
			"protocol":  "sqs", // not lambda
			"endpoint":  "arn:aws:lambda:us-east-1:123:function:my-function",
			"topic_arn": "arn:aws:sns:us-east-1:123:some-topic",
		},
	}
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{subRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong protocol)", result.Count)
	}
}

func TestRelated_Lambda_SNS_NilSubCache(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123:function:my-function"),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// When subList is nil → Count: 0 (no sns-sub cache → treat as definitive zero).
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil sns-sub cache → no subs found)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaSNSSub — sns-sub cache scan (subscription IDs)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_SNSSub_MatchByFnARN(t *testing.T) {
	const fnARN = "arn:aws:lambda:us-east-1:123:function:my-function"
	subRes := resource.Resource{
		ID:   "arn:aws:sns:us-east-1:123:my-topic:sub-abc",
		Name: "sub-abc",
		Fields: map[string]string{
			"protocol": "lambda",
			"endpoint": fnARN,
		},
	}
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{subRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String(fnARN),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (endpoint == FunctionArn)", result.Count)
	}
}

func TestRelated_Lambda_SNSSub_NoMatch(t *testing.T) {
	subRes := resource.Resource{
		ID:   "sub-ddd",
		Name: "sub-ddd",
		Fields: map[string]string{
			"protocol": "lambda",
			"endpoint": "arn:aws:lambda:us-east-1:123:function:other-function",
		},
	}
	cache := resource.ResourceCache{
		"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{subRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123:function:my-function"),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (endpoint mismatch)", result.Count)
	}
}

func TestRelated_Lambda_SNSSub_NilCache(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123:function:my-function"),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "sns-sub")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaS3 — s3 cache scan (notification_lambda field)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_S3_MatchByFnARN(t *testing.T) {
	const fnARN = "arn:aws:lambda:us-east-1:123:function:my-function"
	bRes := resource.Resource{
		ID:   "my-bucket",
		Name: "my-bucket",
		Fields: map[string]string{
			"notification_lambda": fnARN,
		},
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{bRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String(fnARN),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (notification_lambda == FunctionArn)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-bucket" {
		t.Errorf("ResourceIDs = %v, want [my-bucket]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_S3_MatchByFnNameSuffix(t *testing.T) {
	const fnName = "my-function"
	bRes := resource.Resource{
		ID:   "another-bucket",
		Name: "another-bucket",
		Fields: map[string]string{
			"notification_lambda": "arn:aws:lambda:us-east-1:123:function:" + fnName,
		},
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{bRes}},
	}
	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(fnName),
			FunctionArn:  nil,
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (suffix match on function name)", result.Count)
	}
}

func TestRelated_Lambda_S3_NoNotificationField(t *testing.T) {
	bRes := resource.Resource{
		ID:     "empty-notif-bucket",
		Name:   "empty-notif-bucket",
		Fields: map[string]string{}, // no notification_lambda
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{bRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123:function:my-function"),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no notification_lambda field)", result.Count)
	}
}

func TestRelated_Lambda_S3_NilCache(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123:function:my-function"),
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaENI — eni cache scan (description contains function name)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_ENI_MatchByDescription(t *testing.T) {
	const fnName = "my-vpc-function"
	eniRes := resource.Resource{
		ID:   "eni-aaa111",
		Name: "eni-aaa111",
		Fields: map[string]string{
			"description": "AWS Lambda VPC ENI-my-vpc-function-abcdef",
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	src := resource.Resource{ID: fnName, Name: fnName}
	checker := lambdaExtraCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (description contains function name)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "eni-aaa111" {
		t.Errorf("ResourceIDs = %v, want [eni-aaa111]", result.ResourceIDs)
	}
}

func TestRelated_Lambda_ENI_NoDescriptionField(t *testing.T) {
	eniRes := resource.Resource{
		ID:     "eni-bbb222",
		Name:   "eni-bbb222",
		Fields: map[string]string{}, // no description
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	src := resource.Resource{ID: "my-vpc-function", Name: "my-vpc-function"}
	checker := lambdaExtraCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no description field)", result.Count)
	}
}

func TestRelated_Lambda_ENI_NilCache(t *testing.T) {
	src := resource.Resource{ID: "my-vpc-function", Name: "my-vpc-function"}
	checker := lambdaExtraCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaSecrets — env var scan (arn:aws:secretsmanager: prefix)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_Secrets_MatchByEnvVar(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123:secret:my-secret-abc"
	secretRes := resource.Resource{
		ID:     secretARN,
		Name:   "my-secret-abc",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{
					"DB_PASSWORD": secretARN,
					"OTHER_VAR":   "not-a-secret",
				},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (env var is secretsmanager ARN)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != secretARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, secretARN)
	}
}

func TestRelated_Lambda_Secrets_MatchByArnField(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123:secret:other-secret-xyz"
	secretRes := resource.Resource{
		ID:   "some-other-id",
		Name: "other-secret-xyz",
		Fields: map[string]string{
			"arn": secretARN,
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{
					"SECRET_ARN": secretARN,
				},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (arn field fallback match)", result.Count)
	}
}

func TestRelated_Lambda_Secrets_NoSecretsInEnv(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{
					"PLAIN_VAR": "just-a-string",
				},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no secretsmanager ARNs in env)", result.Count)
	}
}

func TestRelated_Lambda_Secrets_NoEnvironment(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Environment:  nil,
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Environment)", result.Count)
	}
}

func TestRelated_Lambda_Secrets_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "my-function",
		RawStruct: "not-a-function",
	}
	checker := lambdaExtraCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct type → early exit)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaSSM — env var scan (slash-prefixed values → SSM parameter names)
// ---------------------------------------------------------------------------

func TestRelated_Lambda_SSM_MatchByParamID(t *testing.T) {
	const paramName = "/my/param/db-password"
	paramRes := resource.Resource{
		ID:   paramName,
		Name: paramName,
	}
	cache := resource.ResourceCache{
		"ssm": resource.ResourceCacheEntry{Resources: []resource.Resource{paramRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{
					"DB_PARAM": paramName,
					"OTHER":    "not-a-param",
				},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (env var matches SSM parameter ID)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != paramName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, paramName)
	}
}

func TestRelated_Lambda_SSM_MatchByParamName(t *testing.T) {
	const paramPath = "/my/param/api-key"
	paramRes := resource.Resource{
		ID:   "ssm-id-xyz",
		Name: paramPath,
	}
	cache := resource.ResourceCache{
		"ssm": resource.ResourceCacheEntry{Resources: []resource.Resource{paramRes}},
	}
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{
					"API_PARAM": paramPath,
				},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (env var matches SSM parameter Name)", result.Count)
	}
}

func TestRelated_Lambda_SSM_NoSlashValues(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{
					"PLAIN_VAR": "just-a-string",
				},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no slash-prefixed env values)", result.Count)
	}
}

func TestRelated_Lambda_SSM_NoEnvironment(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Environment:  nil,
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Environment)", result.Count)
	}
}

func TestRelated_Lambda_SSM_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "my-function",
		RawStruct: "not-a-function",
	}
	checker := lambdaExtraCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct type → early exit)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaDDB — event source mapping DynamoDB path
// ---------------------------------------------------------------------------

// TestRelated_Lambda_DDB_FoundViaDynamoDBStreamARN verifies that checkLambdaDDB
// extracts the table name from a DynamoDB stream event source mapping ARN.
func TestRelated_Lambda_DDB_FoundViaDynamoDBStreamARN(t *testing.T) {
	fnName := "process-orders"
	streamARN := "arn:aws:dynamodb:us-east-1:123456789012:table/orders/stream/2024-01-01T00:00:00.000"

	fakeLambda := &fakeLambdaWithESMArns{
		eventSourceArns: []string{streamARN},
	}
	clients := &awsclient.ServiceClients{
		Lambda: fakeLambda,
	}

	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
	}

	checker := lambdaExtraCheckerByTarget(t, "ddb")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (table from DynamoDB stream ARN)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "orders" {
		t.Errorf("ResourceIDs = %v, want [orders]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Lambda_DDB_NonDynamoARNIgnored verifies that checkLambdaDDB
// returns Count=0 when the event source mappings contain no DynamoDB ARNs.
func TestRelated_Lambda_DDB_NonDynamoARNIgnored(t *testing.T) {
	fnName := "process-orders"
	sqsARN := "arn:aws:sqs:us-east-1:123456789012:my-queue"

	fakeLambda := &fakeLambdaWithESMArns{
		eventSourceArns: []string{sqsARN},
	}
	clients := &awsclient.ServiceClients{
		Lambda: fakeLambda,
	}

	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
	}

	checker := lambdaExtraCheckerByTarget(t, "ddb")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (non-DynamoDB ARN filtered)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaKinesis — event source mapping Kinesis path
// ---------------------------------------------------------------------------

// TestRelated_Lambda_Kinesis_FoundViaKinesisStreamARN verifies that
// checkLambdaKinesis extracts the stream name from a Kinesis stream ARN.
func TestRelated_Lambda_Kinesis_FoundViaKinesisStreamARN(t *testing.T) {
	fnName := "stream-processor"
	kinesisARN := "arn:aws:kinesis:us-east-1:123456789012:stream/events-stream"

	fakeLambda := &fakeLambdaWithESMArns{
		eventSourceArns: []string{kinesisARN},
	}
	clients := &awsclient.ServiceClients{
		Lambda: fakeLambda,
	}

	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
	}

	checker := lambdaExtraCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (Kinesis stream name extracted)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "events-stream" {
		t.Errorf("ResourceIDs = %v, want [events-stream]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Lambda_Kinesis_NonKinesisARNIgnored verifies that checkLambdaKinesis
// returns Count=0 when the event source mappings contain no Kinesis ARNs.
func TestRelated_Lambda_Kinesis_NonKinesisARNIgnored(t *testing.T) {
	fnName := "stream-processor"
	dynamoARN := "arn:aws:dynamodb:us-east-1:123456789012:table/orders/stream/2024-01-01T00:00:00.000"

	fakeLambda := &fakeLambdaWithESMArns{
		eventSourceArns: []string{dynamoARN},
	}
	clients := &awsclient.ServiceClients{
		Lambda: fakeLambda,
	}

	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
	}

	checker := lambdaExtraCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (non-Kinesis ARN filtered)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLambdaMSK — event source mapping MSK/Kafka path
// ---------------------------------------------------------------------------

// TestRelated_Lambda_MSK_FoundViaKafkaARN verifies that checkLambdaMSK extracts
// the MSK cluster name (last segment) from a Kafka cluster ARN.
func TestRelated_Lambda_MSK_FoundViaKafkaARN(t *testing.T) {
	fnName := "kafka-consumer"
	kafkaARN := "arn:aws:kafka:us-east-1:123456789012:cluster/my-msk-cluster/abc-def-ghi"

	fakeLambda := &fakeLambdaWithESMArns{
		eventSourceArns: []string{kafkaARN},
	}
	clients := &awsclient.ServiceClients{
		Lambda: fakeLambda,
	}

	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
	}

	checker := lambdaExtraCheckerByTarget(t, "msk")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (MSK cluster name extracted)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abc-def-ghi" {
		t.Errorf("ResourceIDs = %v, want [abc-def-ghi]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Lambda_MSK_NonKafkaARNIgnored verifies that checkLambdaMSK
// returns Count=0 when the event source mappings contain no Kafka ARNs.
func TestRelated_Lambda_MSK_NonKafkaARNIgnored(t *testing.T) {
	fnName := "kafka-consumer"
	sqsARN := "arn:aws:sqs:us-east-1:123456789012:my-queue"

	fakeLambda := &fakeLambdaWithESMArns{
		eventSourceArns: []string{sqsARN},
	}
	clients := &awsclient.ServiceClients{
		Lambda: fakeLambda,
	}

	src := resource.Resource{
		ID:   fnName,
		Name: fnName,
	}

	checker := lambdaExtraCheckerByTarget(t, "msk")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (non-Kafka ARN filtered)", result.Count)
	}
}

func TestRelated_Lambda_SSM_NilCache(t *testing.T) {
	src := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{
					"PARAM": "/my/param",
				},
			},
		},
	}
	checker := lambdaExtraCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache after slash-value found)", result.Count)
	}
}
