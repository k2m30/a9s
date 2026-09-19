package unit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
)

// checkECSSvcELB resolves Service.LoadBalancers[].TargetGroupArn → the tg
// list → TargetGroup.LoadBalancerArns[] → elb rows by
// Fields["load_balancer_arn"]. An elb row's ID is the bare LB name, never an
// ARN.

func TestECSSvc_Related_ELB_MatchesByLoadBalancerArnField(t *testing.T) {
	tgArn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/checkout-tg/abc123"
	lbArn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/checkout-alb/def456"

	svc := ecstypes.Service{
		LoadBalancers: []ecstypes.LoadBalancer{
			{TargetGroupArn: &tgArn},
		},
	}
	svcRes := resource.Resource{ID: "checkout-svc", Name: "checkout-svc", RawStruct: svc}

	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "checkout-tg",
					Name: "checkout-tg",
					RawStruct: elbv2types.TargetGroup{
						TargetGroupArn:   &tgArn,
						LoadBalancerArns: []string{lbArn},
					},
				},
			},
		},
		"elb": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "checkout-alb",
					Name:   "checkout-alb",
					Fields: map[string]string{"load_balancer_arn": lbArn},
				},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, svcRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec ecs-svc.md:66 cross-refs elb by LoadBalancerArn field, not by bare ID)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "checkout-alb" {
		t.Fatalf("ResourceIDs = %v, want [checkout-alb]", result.ResourceIDs())
	}
}

// checkLambdaTG calls DescribeTargetHealth for each tg with
// TargetType==lambda and matches Targets[].Id against the FunctionArn; the tg
// fetcher carries no function name for a Lambda target.

type fakeELBv2TargetHealth struct {
	healthByTG map[string][]elbv2types.TargetHealthDescription
	calls      int
}

func (f *fakeELBv2TargetHealth) DescribeLoadBalancers(context.Context, *elbv2.DescribeLoadBalancersInput, ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	return &elbv2.DescribeLoadBalancersOutput{}, nil
}

func (f *fakeELBv2TargetHealth) DescribeTargetGroups(context.Context, *elbv2.DescribeTargetGroupsInput, ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error) {
	return &elbv2.DescribeTargetGroupsOutput{}, nil
}

func (f *fakeELBv2TargetHealth) DescribeTargetHealth(_ context.Context, params *elbv2.DescribeTargetHealthInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	f.calls++
	if params.TargetGroupArn == nil {
		return &elbv2.DescribeTargetHealthOutput{}, nil
	}
	return &elbv2.DescribeTargetHealthOutput{TargetHealthDescriptions: f.healthByTG[*params.TargetGroupArn]}, nil
}

func (f *fakeELBv2TargetHealth) DescribeListeners(context.Context, *elbv2.DescribeListenersInput, ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	return &elbv2.DescribeListenersOutput{}, nil
}

func (f *fakeELBv2TargetHealth) DescribeRules(context.Context, *elbv2.DescribeRulesInput, ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error) {
	return &elbv2.DescribeRulesOutput{}, nil
}

func (f *fakeELBv2TargetHealth) DescribeLoadBalancerAttributes(context.Context, *elbv2.DescribeLoadBalancerAttributesInput, ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
	return &elbv2.DescribeLoadBalancerAttributesOutput{}, nil
}

func TestLambda_Related_TG_MatchesViaDescribeTargetHealth(t *testing.T) {
	fnArn := "arn:aws:lambda:us-east-1:123456789012:function:order-processor"
	fnRes := resource.Resource{
		ID:        "order-processor",
		Name:      "order-processor",
		RawStruct: lambdatypes.FunctionConfiguration{FunctionArn: &fnArn, FunctionName: strPtr("order-processor")},
	}

	tgArn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/order-tg/aaa111"
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "order-tg",
					Name: "order-tg",
					// target_group_arn mirrors what the real tg fetcher emits
					// (core/aws/tg.go: Fields["target_group_arn"] = tgArn) —
					// the checker and this fake are both keyed by ARN.
					Fields: map[string]string{"target_type": "lambda", "target_group_arn": tgArn},
				},
			},
		},
	}

	fake := &fakeELBv2TargetHealth{
		healthByTG: map[string][]elbv2types.TargetHealthDescription{
			tgArn: {
				{Target: &elbv2types.TargetDescription{Id: &fnArn}},
			},
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}

	checker := lambdaCheckerByTarget(t, "tg")
	result := checker(context.Background(), clients, fnRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec lambda.md:169 DescribeTargetHealth Targets[].Id==FunctionArn)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "order-tg" {
		t.Fatalf("ResourceIDs = %v, want [order-tg]", result.ResourceIDs())
	}
}

func TestLambda_Related_TG_InstanceTargetTypeSkipsDescribeTargetHealth(t *testing.T) {
	fnArn := "arn:aws:lambda:us-east-1:123456789012:function:order-processor"
	fnRes := resource.Resource{
		ID:        "order-processor",
		Name:      "order-processor",
		RawStruct: lambdatypes.FunctionConfiguration{FunctionArn: &fnArn, FunctionName: strPtr("order-processor")},
	}

	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "web-tg",
					Name:   "web-tg",
					Fields: map[string]string{"target_type": "instance"},
				},
			},
		},
	}

	fake := &fakeELBv2TargetHealth{healthByTG: map[string][]elbv2types.TargetHealthDescription{}}
	clients := &awsclient.ServiceClients{ELBv2: fake}

	checker := lambdaCheckerByTarget(t, "tg")
	result := checker(context.Background(), clients, fnRes, cache)

	if result.Count() != 0 {
		t.Fatalf("Count = %d, want 0 for an instance-type TG", result.Count())
	}
	if fake.calls != 0 {
		t.Fatalf("DescribeTargetHealth calls = %d, want 0 — instance-type TGs must not be probed", fake.calls)
	}
}

// checkLambdaECR reads Code.ImageUri from GetFunction: ListFunctions and
// FunctionConfiguration do not carry it.

// fakeLambdaGetFunctionAPI serves one named function's Code.ImageUri from
// GetFunction.
type fakeLambdaGetFunctionAPI struct {
	functionName string
	imageURI     string
	calls        int
}

func (f *fakeLambdaGetFunctionAPI) ListFunctions(context.Context, *lambda.ListFunctionsInput, ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	return &lambda.ListFunctionsOutput{}, nil
}

func (f *fakeLambdaGetFunctionAPI) ListEventSourceMappings(context.Context, *lambda.ListEventSourceMappingsInput, ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	return &lambda.ListEventSourceMappingsOutput{}, nil
}

func (f *fakeLambdaGetFunctionAPI) ListTags(context.Context, *lambda.ListTagsInput, ...func(*lambda.Options)) (*lambda.ListTagsOutput, error) {
	return &lambda.ListTagsOutput{Tags: map[string]string{}}, nil
}

func (f *fakeLambdaGetFunctionAPI) GetFunction(_ context.Context, params *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	f.calls++
	if params.FunctionName == nil || *params.FunctionName != f.functionName {
		return &lambda.GetFunctionOutput{}, nil
	}
	return &lambda.GetFunctionOutput{
		Code: &lambdatypes.FunctionCodeLocation{ImageUri: &f.imageURI},
	}, nil
}

func TestLambda_Related_ECR_ResolvesViaGetFunction(t *testing.T) {
	fnRes := resource.Resource{
		ID:   "image-fn",
		Name: "image-fn",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: strPtr("image-fn"),
			PackageType:  lambdatypes.PackageTypeImage,
		},
		Fields: map[string]string{"package_type": "Image"},
	}

	cache := resource.ResourceCache{
		"ecr": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "my-repo", Name: "my-repo"},
			},
		},
	}

	fake := &fakeLambdaGetFunctionAPI{
		functionName: "image-fn",
		imageURI:     "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:latest",
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	checker := lambdaCheckerByTarget(t, "ecr")
	result := checker(context.Background(), clients, fnRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec lambda.md:72 GetFunction Code.ImageUri -> ecr repo)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-repo" {
		t.Fatalf("ResourceIDs = %v, want [my-repo]", result.ResourceIDs())
	}
}

func TestLambda_Related_ECR_ZipPackageReturnsZeroNoGetFunctionCall(t *testing.T) {
	fnRes := resource.Resource{
		ID:   "zip-fn",
		Name: "zip-fn",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: strPtr("zip-fn"),
			PackageType:  lambdatypes.PackageTypeZip,
		},
		Fields: map[string]string{"package_type": "Zip"},
	}

	fake := &fakeLambdaGetFunctionAPI{functionName: "zip-fn", imageURI: "should-not-be-used"}
	clients := &awsclient.ServiceClients{Lambda: fake}

	checker := lambdaCheckerByTarget(t, "ecr")
	result := checker(context.Background(), clients, fnRes, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Fatalf("Count = %d, want 0 for a Zip-package function", result.Count())
	}
	if fake.calls != 0 {
		t.Fatalf("GetFunction calls = %d, want 0 — Zip-package functions must not trigger the fan-out call", fake.calls)
	}
}

// checkLambdaCF matches CloudFront LambdaFunctionAssociations against the
// function's versioned ARN: a CloudFront association always references
// ":function:name:N".

func TestLambda_Related_CF_MatchesVersionedArnPrefix(t *testing.T) {
	fnArn := "arn:aws:lambda:us-east-1:123456789012:function:my-fn"
	fnRes := resource.Resource{
		ID:        "my-fn",
		Name:      "my-fn",
		RawStruct: lambdatypes.FunctionConfiguration{FunctionArn: &fnArn, FunctionName: strPtr("my-fn")},
	}

	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "E1234567890ABC",
					Name:   "E1234567890ABC",
					Fields: map[string]string{"lambda_function_arns": fnArn + ":3"},
				},
			},
		},
	}

	checker := lambdaCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, fnRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec lambda.md:42 versioned-ARN association match)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "E1234567890ABC" {
		t.Fatalf("ResourceIDs = %v, want [E1234567890ABC]", result.ResourceIDs())
	}
}

// The ECS-task fetcher stores RawStruct = ecstypes.Task, never a
// TaskDefinition. Roles, secrets and SSM parameters come from the
// DescribeTaskDefinition join as Fields["task_role"], ["execution_role"],
// ["secret_arns"] and ["ssm_param_names"]. The SG pivot cross-references
// Task → ENI → SG.

func mechanismECSTaskFixture(id string) ecstypes.Task {
	return ecstypes.Task{
		TaskArn:           strPtr("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/" + id),
		ClusterArn:        strPtr("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		TaskDefinitionArn: strPtr("arn:aws:ecs:us-east-1:123456789012:task-definition/checkout:7"),
		LastStatus:        strPtr("RUNNING"),
	}
}

// A role ARN counts only when the loaded role list contains it: a stale ARN
// (a role deleted after the task started) names no related role.
func TestECSTask_Related_Role_CrossReferencesLoadedRoleCache(t *testing.T) {
	task := mechanismECSTaskFixture("abc123")
	taskRes := resource.Resource{
		ID:        "abc123",
		Name:      "abc123",
		RawStruct: task,
		Fields: map[string]string{
			"task_role":      "arn:aws:iam::123456789012:role/checkout-task-role",
			"execution_role": "arn:aws:iam::123456789012:role/stale-deleted-execution-role",
		},
	}

	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "checkout-task-role", Name: "checkout-task-role"},
			},
		},
	}

	checker := ecsTaskCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, taskRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec ecs-task.md:79 cross-references the role list; a role ARN absent from the loaded cache must not be counted)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "checkout-task-role" {
		t.Fatalf("ResourceIDs = %v, want [checkout-task-role]", result.ResourceIDs())
	}
}

func TestECSTask_Related_Secrets_ReadsFromRealTaskRawStruct(t *testing.T) {
	task := mechanismECSTaskFixture("def456")
	secretArn := "arn:aws:secretsmanager:us-east-1:123456789012:secret:checkout/db-password-AbCdEf"
	taskRes := resource.Resource{
		ID:        "def456",
		Name:      "def456",
		RawStruct: task,
		Fields:    map[string]string{"secret_arns": secretArn},
	}

	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "checkout/db-password", Name: "checkout/db-password", Fields: map[string]string{"arn": secretArn}},
			},
		},
	}

	checker := ecsTaskCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, taskRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec ecs-task.md:84 — real Task RawStruct must not silently zero this pivot)", result.Count())
	}
}

func TestECSTask_Related_SSM_ReadsFromRealTaskRawStruct(t *testing.T) {
	task := mechanismECSTaskFixture("ghi789")
	paramName := "/checkout/feature-flags"
	taskRes := resource.Resource{
		ID:        "ghi789",
		Name:      "ghi789",
		RawStruct: task,
		Fields:    map[string]string{"ssm_param_names": paramName},
	}

	cache := resource.ResourceCache{
		"ssm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: paramName, Name: paramName},
			},
		},
	}

	checker := ecsTaskCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, taskRes, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (spec ecs-task.md:96 — real Task RawStruct must not silently zero this pivot)", result.Count())
	}
}

func TestECSTask_Related_SG_ViaTaskENISecurityGroupCrossRef(t *testing.T) {
	task := ecstypes.Task{
		TaskArn:           strPtr("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/jkl012"),
		ClusterArn:        strPtr("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		TaskDefinitionArn: strPtr("arn:aws:ecs:us-east-1:123456789012:task-definition/checkout:7"),
		Attachments: []ecstypes.Attachment{
			{
				Type: strPtr("ElasticNetworkInterface"),
				Details: []ecstypes.KeyValuePair{
					{Name: strPtr("networkInterfaceId"), Value: strPtr("eni-0a1b2c3d")},
				},
			},
		},
	}
	taskRes := resource.Resource{ID: "jkl012", Name: "jkl012", RawStruct: task}

	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "eni-0a1b2c3d",
					Name:   "eni-0a1b2c3d",
					Fields: map[string]string{"security_groups": "sg-11111111,sg-22222222"},
				},
			},
		},
		"sg": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "sg-11111111", Name: "checkout-sg"},
				{ID: "sg-22222222", Name: "checkout-db-sg"},
			},
		},
	}

	checker := ecsTaskCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, taskRes, cache)

	if result.Count() != 2 {
		t.Fatalf("Count = %d, want 2 (spec ecs-task.md:90 Task -> ENI -> SG cross-reference)", result.Count())
	}
}

// checkEC2Backup matches the loaded backup plans' Fields["resources"] and
// ["not_resources"] ARN patterns against the instance ARN, with
// BackupPlanCoversARN semantics.

func TestEC2_Related_Backup_MatchesLoadedPlanSelectionARN(t *testing.T) {
	instanceARN := "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123def456"
	instRes := resource.Resource{
		ID:     "i-0abc123def456",
		Name:   "i-0abc123def456",
		Fields: map[string]string{"arn": instanceARN},
	}

	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "plan-prod-ec2",
					Name: "plan-prod-ec2",
					Fields: map[string]string{
						"resources":     "arn:aws:ec2:us-east-1:123456789012:instance/*",
						"not_resources": "",
					},
				},
			},
		},
	}

	checker := mechanismEC2CheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, instRes, cache)

	if result.Count() < 1 {
		t.Fatalf("Count = %d, want >=1 (spec ec2.md:49 — loaded plan's selection ARN pattern covers this instance)", result.Count())
	}
}

// mechanismEC2CheckerByTarget mirrors the ec2CheckerByTarget helper declared
// in the internal "unit" test package (aws_ec2_related_test.go), which is
// not visible from this external "unit_test" package.
func mechanismEC2CheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ec2 related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ec2 related checker for %s not found", target)
	return nil
}

// Demo mode must serve real data for these checkers, or the panel reads
// zero: checkLambdaCFN needs LambdaFake.ListTags to serve per-function tags,
// and checkECSSvcSFN needs SFNFake.DescribeStateMachine to return a
// Definition.

func TestFakes_LambdaListTags_ServesCloudFormationStackNameTag(t *testing.T) {
	fake := fakes.NewLambda()
	fnArn := "arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer"

	out, err := fake.ListTags(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTags returned error: %v", err)
	}
	if len(out.Tags) == 0 {
		t.Fatalf("LambdaFake.ListTags returned no tags at all — demo fixtures must carry aws:cloudformation:stack-name for at least one function so checkLambdaCFN can surface a real match")
	}

	fnRes := resource.Resource{
		ID:        "api-gateway-authorizer",
		Name:      "api-gateway-authorizer",
		RawStruct: lambdatypes.FunctionConfiguration{FunctionArn: &fnArn, FunctionName: strPtr("api-gateway-authorizer")},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "acme-eks-cluster", Name: "acme-eks-cluster", Fields: map[string]string{"stack_name": "acme-eks-cluster"}},
			},
		},
	}
	clients := &awsclient.ServiceClients{Lambda: fake}
	checker := lambdaCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, fnRes, cache)
	if result.Count() < 1 {
		t.Fatalf("checkLambdaCFN Count = %d, want >=1 once LambdaFake.ListTags serves a real aws:cloudformation:stack-name tag", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "acme-eks-cluster" {
		t.Fatalf("ResourceIDs = %v, want [acme-eks-cluster]", result.ResourceIDs())
	}
}

// The call names a state machine ARN the fake holds: the fake refuses an
// unregistered ARN, and a nil input would be answered with the placeholder
// definition "{}" — non-empty, so the assertion would pass without ever
// reaching a fixture.
func TestFakes_SFNDescribeStateMachine_ServesECSRunTaskDefinition(t *testing.T) {
	fake := fakes.NewSFN()
	listed, err := fake.ListStateMachines(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListStateMachines returned error: %v", err)
	}
	found := ""
	for _, m := range listed.StateMachines {
		out, err := fake.DescribeStateMachine(context.Background(),
			&sfn.DescribeStateMachineInput{StateMachineArn: m.StateMachineArn})
		if err != nil {
			t.Fatalf("DescribeStateMachine(%q) returned error: %v", aws.ToString(m.StateMachineArn), err)
		}
		if out.Definition != nil && strings.Contains(*out.Definition, "ecs:runTask") {
			found = aws.ToString(m.StateMachineArn)
		}
	}
	if found == "" {
		t.Fatalf("no demo state machine has an ASL containing an ecs:runTask state — demo fixtures must model at least one whose ASL references a demo task-definition family, so checkECSSvcSFN can surface a real match")
	}
}

func strPtr(s string) *string { return &s }
