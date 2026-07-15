// aws_related_checker_mechanism_test.go pins the CORRECT related-panel
// checker mechanism (quoted from the golden per-type specs in
// docs/resources/*.md) for pivots that today return zero or garbage on
// realistic data. Each test is expected to be RED at HEAD until the paired
// coder task lands the fix; the mechanism it asserts is the documented one,
// not the current (broken) implementation.
package unit_test

import (
	"context"
	"testing"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// 1. checkECSSvcELB — ecs-svc.md:66
//
// "resolve via Service.LoadBalancers[].TargetGroupArn -> the already-loaded
// tg list -> TargetGroup.LoadBalancerArns[] -> cross-reference the
// already-loaded elb list by LoadBalancer.LoadBalancerArn"
//
// At HEAD the final step matches elbRes.ID (a bare LB *name*, per
// core/aws/elb.go's `ID: lbName`) against the LoadBalancerArns set (full
// ARNs) — an ID/ARN type mismatch that can never match on real data. The
// correct mechanism cross-references Fields["load_balancer_arn"].
// ---------------------------------------------------------------------------

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

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (spec ecs-svc.md:66 cross-refs elb by LoadBalancerArn field, not by bare ID)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "checkout-alb" {
		t.Fatalf("ResourceIDs = %v, want [checkout-alb]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// 2. checkLambdaTG — lambda.md:169
//
// "cross-reference the tg list -- for each TG with TargetType==lambda, call
// DescribeTargetHealth and match Targets[].Id==FunctionArn."
//
// At HEAD the checker never calls DescribeTargetHealth at all — it matches
// on Fields["lambda_function_name"] / Fields["target_arn"] suffix, fields
// the tg fetcher does not populate for Lambda targets. On realistic data
// (TG with TargetType==lambda, no lambda_function_name field) this always
// returns Count:0 even when DescribeTargetHealth would report the function
// as a live target.
// ---------------------------------------------------------------------------

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

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (spec lambda.md:169 DescribeTargetHealth Targets[].Id==FunctionArn)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "order-tg" {
		t.Fatalf("ResourceIDs = %v, want [order-tg]", result.ResourceIDs)
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

	if result.Count != 0 {
		t.Fatalf("Count = %d, want 0 for an instance-type TG", result.Count)
	}
	if fake.calls != 0 {
		t.Fatalf("DescribeTargetHealth calls = %d, want 0 — instance-type TGs must not be probed", fake.calls)
	}
}

// NOTE: checkLambdaENI (lambda.md:84, "cross-reference the eni list --
// match RequesterId=='AWS Lambda VPC ENI' / Description starting with 'AWS
// Lambda VPC ENI-<FunctionName>-'") requires BOTH the requester_id gate and
// the description prefix. Regression coverage for the two-field mechanism
// (including the requester_id-missing negative case) lives in
// aws_lambda_related_extra_test.go's TestRelated_Lambda_ENI_* tests, not
// here.

// ---------------------------------------------------------------------------
// 4. checkLambdaECR — lambda.md:72
//
// "PackageType==Image; the image URI is returned by GetFunction under
// Code.ImageUri (not on ListFunctions/FunctionConfiguration). Parse the
// repository name from the URI ... and cross-reference the ecr list."
//
// At HEAD the checker reads res.Fields["image_uri"], which is never
// populated by the lambda fetcher (ListFunctions does not carry ImageUri),
// so this pivot is permanently State: RelatedUnknown for every real Image-package
// function. The correct mechanism calls GetFunction for this one function.
// ---------------------------------------------------------------------------

// fakeLambdaGetFunctionAPI implements the aws.LambdaAPI surface, serving a
// single named function's Code.ImageUri from GetFunction — the call
// checkLambdaECR must make per spec, since ImageUri is not present on
// ListFunctions/FunctionConfiguration.
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

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (spec lambda.md:72 GetFunction Code.ImageUri -> ecr repo)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-repo" {
		t.Fatalf("ResourceIDs = %v, want [my-repo]", result.ResourceIDs)
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

	if result.Count != 0 {
		t.Fatalf("Count = %d, want 0 for a Zip-package function", result.Count)
	}
	if fake.calls != 0 {
		t.Fatalf("GetFunction calls = %d, want 0 — Zip-package functions must not trigger the fan-out call", fake.calls)
	}
}

// ---------------------------------------------------------------------------
// 5. checkLambdaCF — lambda.md:42
//
// "cross-reference cf distribution config -- match
// DefaultCacheBehavior.LambdaFunctionAssociations[].LambdaFunctionARN and
// each CacheBehaviors[].LambdaFunctionAssociations[] against the function's
// versioned ARN."
//
// At HEAD the checker matches cfRes.Fields["lambda_function_arn"] against
// the function's UNVERSIONED ARN exactly. Real CloudFront associations
// always reference a versioned ARN (":function:name:N"), so an exact
// unversioned-ARN match never fires on real data.
// ---------------------------------------------------------------------------

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

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (spec lambda.md:42 versioned-ARN association match)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "E1234567890ABC" {
		t.Fatalf("ResourceIDs = %v, want [E1234567890ABC]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// 6. ECS task role/secrets/ssm/sg — ecs-task.md:78/84/96/90
//
// The real ECS-task fetcher stores RawStruct = ecstypes.Task (confirmed by
// checkECSTaskService/Cluster asserting ecstypes.Task), never
// ecstypes.TaskDefinition. checkECSTaskSecrets/checkECSTaskSSM at HEAD
// unconditionally return Count:0 whenever RawStruct is ecstypes.Task — a
// silent-zero on every real task. Per the DescribeTaskDefinition join the
// fetcher will additionally emit Fields["task_role"], ["execution_role"],
// ["secret_arns"], ["ssm_param_names"].
// checkECSTaskSG at HEAD ignores the cache entirely and always returns 0;
// spec ecs-task.md:90 requires a Task -> ENI -> SG cross-reference.
// ---------------------------------------------------------------------------

func mechanismECSTaskFixture(id string) ecstypes.Task {
	return ecstypes.Task{
		TaskArn:           strPtr("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/" + id),
		ClusterArn:        strPtr("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		TaskDefinitionArn: strPtr("arn:aws:ecs:us-east-1:123456789012:task-definition/checkout:7"),
		LastStatus:        strPtr("RUNNING"),
	}
}

// TestECSTask_Related_Role_CrossReferencesLoadedRoleCache verifies the
// checker actually "cross-reference[s] the already-loaded role list by ARN"
// (ecs-task.md:79) rather than blindly trusting any ARN-shaped value in
// Fields["task_role"]/["execution_role"]. At HEAD the checker ignores its
// cache argument entirely (see the discarded "_ resource.ResourceCache"
// parameter in checkECSTaskRole) and always reports both roles as found —
// even a stale/garbage ARN with no matching entry in the role cache is
// counted. A real cross-reference must not report a role that the loaded
// role list does not contain.
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

	// Only the task role exists in the loaded role cache — the execution
	// role ARN is stale (e.g. the role was deleted after the task started).
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "checkout-task-role", Name: "checkout-task-role"},
			},
		},
	}

	checker := ecsTaskCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, taskRes, cache)

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (spec ecs-task.md:79 cross-references the role list; a role ARN absent from the loaded cache must not be counted)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "checkout-task-role" {
		t.Fatalf("ResourceIDs = %v, want [checkout-task-role]", result.ResourceIDs)
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

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (spec ecs-task.md:84 — real Task RawStruct must not silently zero this pivot)", result.Count)
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

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (spec ecs-task.md:96 — real Task RawStruct must not silently zero this pivot)", result.Count)
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

	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2 (spec ecs-task.md:90 Task -> ENI -> SG cross-reference)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// 7. checkEC2Backup — ec2.md:49
//
// "cross-reference the already-loaded backup list; match by backup-plan
// selection tags present on Instance.Tags[] or by ARN via
// backup:ListProtectedResources."
//
// At HEAD checkEC2Backup unconditionally returns Count:0 (see the "we
// conservatively report Count:0 here" comment in ec2_related_extra.go) even
// when a loaded backup plan's Fields["resources"]/["not_resources"] ARN
// pattern lists (populated by FetchBackupPlansPage's
// enumerateBackupPlanResources join) cover this instance's ARN. The correct
// mechanism cross-references those already-loaded selection ARNs, mirroring
// BackupPlanCoversARN's semantics used elsewhere in the same package.
// ---------------------------------------------------------------------------

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

	if result.Count < 1 {
		t.Fatalf("Count = %d, want >=1 (spec ec2.md:49 — loaded plan's selection ARN pattern covers this instance)", result.Count)
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

// ---------------------------------------------------------------------------
// 8. Fakes gap — demo mode returns empty/garbage for three checkers that are
// otherwise correctly implemented, so the panel silently shows zero even
// with a coder fix landed:
//
//   - checkLambdaCFN (lambda.md aws:cloudformation:stack-name tag) needs
//     LambdaFake.ListTags to serve per-function tags. At HEAD ListTags
//     always returns an empty map.
//   - checkEC2SSM (ec2 SSM-managed instance check) needs
//     SSMFake.DescribeInstanceInformation to serve enrolled instance IDs. At
//     HEAD it is a permanent no-op stub returning an empty list.
//   - checkECSSvcSFN (ecs:runTask state-machine cross-ref) needs
//     SFNFake.DescribeStateMachine to return a Definition. At HEAD the fake
//     returns only StateMachineArn, Definition is always nil.
// ---------------------------------------------------------------------------

func TestFakes_LambdaListTags_ServesCloudFormationStackNameTag(t *testing.T) {
	fake := fakes.NewLambda()
	// api-gateway-authorizer / acme-eks-cluster is the real fixture wiring
	// (core/demo/fixtures/lambda.go: Tags["api-gateway-authorizer"] =
	// {"aws:cloudformation:stack-name": "acme-eks-cluster"}, acme-eks-cluster
	// being a real cfn.go stack fixture) — not a synthetic pair.
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
	if result.Count < 1 {
		t.Fatalf("checkLambdaCFN Count = %d, want >=1 once LambdaFake.ListTags serves a real aws:cloudformation:stack-name tag", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "acme-eks-cluster" {
		t.Fatalf("ResourceIDs = %v, want [acme-eks-cluster]", result.ResourceIDs)
	}
}

func TestFakes_SSMDescribeInstanceInformation_ServesEnrolledInstance(t *testing.T) {
	fake := fakes.NewSSM()
	out, err := fake.DescribeInstanceInformation(context.Background(), nil)
	if err != nil {
		t.Fatalf("DescribeInstanceInformation returned error: %v", err)
	}
	if len(out.InstanceInformationList) == 0 {
		t.Fatalf("SSMFake.DescribeInstanceInformation returned no enrolled instances — demo fixtures must model at least one SSM-managed EC2 instance so checkEC2SSM can surface a real match")
	}
}

func TestFakes_SFNDescribeStateMachine_ServesECSRunTaskDefinition(t *testing.T) {
	fake := fakes.NewSFN()
	out, err := fake.DescribeStateMachine(context.Background(), nil)
	if err != nil {
		t.Fatalf("DescribeStateMachine returned error: %v", err)
	}
	if out.Definition == nil || *out.Definition == "" {
		t.Fatalf("SFNFake.DescribeStateMachine returned no Definition — demo fixtures must model at least one state machine whose ASL contains an ecs:runTask state referencing a demo task-definition family, so checkECSSvcSFN can surface a real match")
	}
}

func strPtr(s string) *string { return &s }
