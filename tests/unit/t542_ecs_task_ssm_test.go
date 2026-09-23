package unit

// An ECS task definition names an SSM parameter in a secret's valueFrom by
// name or by ARN. A parameter's name is what DescribeParameters returns and
// what the ssm list keys on: a flat name has no leading "/" (db_password,
// ARN …:parameter/db_password) and a path name keeps it (/app/db, ARN
// …:parameter/app/db) — docs.aws.amazon.com/systems-manager/latest/userguide/sysman-paramstore-su-create.html
// and docs.aws.amazon.com/AmazonECS/latest/developerguide/secrets-envvar-ssm-paramstore.html.

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type t542SSMList struct{ names []string }

func (f t542SSMList) DescribeParameters(context.Context, *ssm.DescribeParametersInput, ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	out := &ssm.DescribeParametersOutput{}
	for _, n := range f.names {
		out.Parameters = append(out.Parameters, ssmtypes.ParameterMetadata{
			Name: aws.String(n),
			ARN:  aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/" + trimLeadingSlash(n)),
			Type: ssmtypes.ParameterTypeSecureString,
		})
	}
	return out, nil
}

func trimLeadingSlash(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

func TestECSTaskSSM_FlatAndPathNamesByNameAndByARN(t *testing.T) {
	cases := map[string]string{
		"flat by name": "db_password",
		"flat by ARN":  "arn:aws:ssm:us-east-1:123456789012:parameter/db_password",
		"path by name": "/app/db",
		"path by ARN":  "arn:aws:ssm:us-east-1:123456789012:parameter/app/db",
	}
	want := map[string]string{"flat by name": "db_password", "flat by ARN": "db_password", "path by name": "/app/db", "path by ARN": "/app/db"}

	ssmPage, err := awsclient.FetchSSMParametersPage(context.Background(), t542SSMList{names: []string{"db_password", "/app/db", "/app/db_password"}}, "")
	if err != nil {
		t.Fatalf("ssm list: %v", err)
	}
	cache := resource.ResourceCache{"ssm": resource.ResourceCacheEntry{Resources: ssmPage.Resources}}

	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("ecs-task") {
		if def.TargetType == "ssm" {
			checker = def.Checker
		}
	}
	if checker == nil {
		t.Fatal("ecs-task has no ssm related checker")
	}

	for name, valueFrom := range cases {
		t.Run(name, func(t *testing.T) {
			mock := buildECSTaskMockAPI(func(*ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
				return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{
					TaskDefinitionArn: aws.String(testTaskDefARN),
					ContainerDefinitions: []ecstypes.ContainerDefinition{{
						Name:    aws.String("web"),
						Secrets: []ecstypes.Secret{{Name: aws.String("DB_SECRET"), ValueFrom: aws.String(valueFrom)}},
					}},
				}}, nil
			})
			res, err := ecsTaskPaginatedFetcher(t)(context.Background(), &awsclient.ServiceClients{ECS: mock}, "")
			if err != nil || len(res.Resources) != 1 {
				t.Fatalf("ecs-task fetch: %v (%d rows)", err, len(res.Resources))
			}
			got := checker(context.Background(), nil, res.Resources[0], cache)
			if ids := got.ResourceIDs(); !slices.Equal(ids, []string{want[name]}) {
				t.Errorf("ecs-task → SSM for valueFrom %q = %v, want [%s]", valueFrom, ids, want[name])
			}
		})
	}
}
