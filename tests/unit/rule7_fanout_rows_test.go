package unit_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

func rule7Checker(t *testing.T, source, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(source) {
		if def.TargetType == target {
			return def.Checker
		}
	}
	t.Fatalf("no %s → %s checker", source, target)
	return nil
}

type rule7DeniedECS struct {
	awsclient.ECSAPI
	calls int
}

func (f *rule7DeniedECS) DescribeTaskDefinition(context.Context, *ecs.DescribeTaskDefinitionInput, ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	f.calls++
	return nil, errors.New("AccessDeniedException")
}

// Tasks the list joined to their definition answer from their fields: sixty
// of them cost no call and none of them is cut by the per-row cap.
func TestRule7_JoinedTasksAnswerSecretsWithoutACall(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password-AbCdEf"
	var tasks []resource.Resource
	for i := range 60 {
		refs := ""
		if i == 55 {
			refs = secretARN
		}
		tasks = append(tasks, resource.Resource{
			ID:     fmt.Sprintf("task-%03d", i),
			Fields: map[string]string{"task_definition": "arn:aws:ecs:us-east-1:123456789012:task-definition/api:7", "secret_arns": refs},
		})
	}
	source := resource.Resource{ID: "prod/db/password", Name: "prod/db/password", Fields: map[string]string{"arn": secretARN, "name": "prod/db/password"}}
	api := &rule7DeniedECS{}
	result := rule7Checker(t, "secrets", "ecs-task")(context.Background(), &awsclient.ServiceClients{ECS: api}, source,
		resource.ResourceCache{"ecs-task": {Resources: tasks}})

	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != "task-055" {
		t.Errorf("ids = %v, want [task-055]", ids)
	}
	if result.Truncated() {
		t.Error("a list read in full from its join is a lower bound")
	}
	if api.calls != 0 {
		t.Errorf("DescribeTaskDefinition called %d times for tasks the list joined", api.calls)
	}
}

type rule7ImageLambda struct {
	awsclient.LambdaAPI
	calls int
}

func (f *rule7ImageLambda) GetFunction(_ context.Context, in *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	f.calls++
	return &lambda.GetFunctionOutput{Code: &lambdatypes.FunctionCodeLocation{ImageUri: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api:" + aws.ToString(in.FunctionName))}}, nil
}

// Only a container-image function costs a GetFunction: sixty zip functions
// beside one image function leave the count exact.
func TestRule7_OnlyImageFunctionsCountTowardTheCap(t *testing.T) {
	var fns []resource.Resource
	for i := range 60 {
		fns = append(fns, resource.Resource{ID: fmt.Sprintf("zip-%03d", i), Fields: map[string]string{"package_type": "Zip"}})
	}
	fns = append(fns, resource.Resource{ID: "image-fn", Fields: map[string]string{"package_type": "Image"}})
	source := resource.Resource{ID: "acme/api", Name: "acme/api", Fields: map[string]string{"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api"}}
	api := &rule7ImageLambda{}
	result := rule7Checker(t, "ecr", "lambda")(context.Background(), &awsclient.ServiceClients{Lambda: api}, source,
		resource.ResourceCache{"lambda": {Resources: fns}})

	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != "image-fn" {
		t.Errorf("ids = %v, want [image-fn]", ids)
	}
	if result.Truncated() {
		t.Error("one image function is under the cap, so the count is exact")
	}
	if api.calls != 1 {
		t.Errorf("GetFunction called %d times, want once for the one image function", api.calls)
	}
}
