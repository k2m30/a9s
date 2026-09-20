package unit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// An image URI names a repository by its registry host
// (<account>.dkr.ecr.<region>.amazonaws.com) and its full repository path,
// with a ":tag" or "@sha256:digest" after it. Two images belong to the same
// repository only when both halves are equal: "api" is not "api-worker" nor
// "team/api", and "api" in another account or region is another repository.
const (
	ecrIdentRegistry = "123456789012.dkr.ecr.us-east-1.amazonaws.com"
	ecrIdentDigest   = "sha256:3f1c9a7e2b4d6f8a0c1e3b5d7f9a1c3e5b7d9f1a3c5e7b9d1f3a5c7e9b1d3f5a"
)

// ecrIdentImages maps each image a workload can run to the local repository
// it belongs to, or "" when it belongs to none of them.
var ecrIdentImages = []struct {
	key, image, repo string
}{
	{"api-tag", ecrIdentRegistry + "/api:v1.4.2", "api"},
	{"api-digest", ecrIdentRegistry + "/api@" + ecrIdentDigest, "api"},
	{"api-worker", ecrIdentRegistry + "/api-worker:v1.4.2", "api-worker"},
	{"team-api", ecrIdentRegistry + "/team/api:2026-09-01", "team/api"},
	{"other-account", "210987654321.dkr.ecr.us-east-1.amazonaws.com/api:v1.4.2", ""},
	{"other-region", "123456789012.dkr.ecr.us-west-2.amazonaws.com/api:v1.4.2", ""},
	{"docker-hub", "nginx:1.27", ""},
}

func ecrIdentRepo(name string) resource.Resource {
	uri := ecrIdentRegistry + "/" + name
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"repository_name": name,
			"uri":             uri,
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String(name),
			RepositoryUri:  aws.String(uri),
			RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/" + name),
			RegistryId:     aws.String("123456789012"),
		},
	}
}

func ecrIdentRepoCache() resource.ResourceCacheEntry {
	return resource.ResourceCacheEntry{Resources: []resource.Resource{
		ecrIdentRepo("api"), ecrIdentRepo("api-worker"), ecrIdentRepo("team/api"),
	}}
}

// ecrIdentWant is the IDs of the workloads whose image belongs to repo.
func ecrIdentWant(repo string) []string {
	var ids []string
	for _, img := range ecrIdentImages {
		if img.repo == repo {
			ids = append(ids, img.key)
		}
	}
	return ids
}

func assertSameIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	g, w := slices.Clone(got), slices.Clone(want)
	slices.Sort(g)
	slices.Sort(w)
	if !slices.Equal(g, w) {
		t.Errorf("%s: ResourceIDs = %v, want %v", label, g, w)
	}
}

// ecrIdentLambdaFake answers GetFunction with the image each function runs,
// the only call that returns Code.ImageUri.
type ecrIdentLambdaFake struct {
	fakeLambdaGetFunctionAPI
	images map[string]string
}

func (f *ecrIdentLambdaFake) GetFunction(_ context.Context, in *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	uri, ok := f.images[aws.ToString(in.FunctionName)]
	if !ok {
		return &lambda.GetFunctionOutput{}, nil
	}
	return &lambda.GetFunctionOutput{
		Configuration: &lambdatypes.FunctionConfiguration{FunctionName: in.FunctionName, PackageType: lambdatypes.PackageTypeImage},
		Code:          &lambdatypes.FunctionCodeLocation{RepositoryType: aws.String("ECR"), ImageUri: aws.String(uri)},
	}, nil
}

func ecrIdentLambdaClients() *awsclient.ServiceClients {
	images := make(map[string]string, len(ecrIdentImages))
	for _, img := range ecrIdentImages {
		images[img.key] = img.image
	}
	return &awsclient.ServiceClients{Lambda: &ecrIdentLambdaFake{images: images}}
}

func ecrIdentLambdaRow(name string, pkg lambdatypes.PackageType) resource.Resource {
	return resource.Resource{
		ID:     name,
		Name:   name,
		Fields: map[string]string{"package_type": string(pkg)},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(name),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name),
			PackageType:  pkg,
		},
	}
}

func ecrIdentTaskRow(key, image string) resource.Resource {
	return resource.Resource{
		ID:     key,
		Name:   key,
		Fields: map[string]string{"container_images": image},
		RawStruct: ecstypes.Task{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/" + key),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/" + key + ":3"),
			Containers:        []ecstypes.Container{{Name: aws.String("app"), Image: aws.String(image)}},
		},
	}
}

func ecrIdentProjectRow(key, image string) resource.Resource {
	return resource.Resource{
		ID:   key,
		Name: key,
		RawStruct: cbtypes.Project{
			Name: aws.String(key),
			Arn:  aws.String("arn:aws:codebuild:us-east-1:123456789012:project/" + key),
			Environment: &cbtypes.ProjectEnvironment{
				Type:        cbtypes.EnvironmentTypeLinuxContainer,
				ComputeType: cbtypes.ComputeTypeBuildGeneral1Small,
				Image:       aws.String(image),
			},
		},
	}
}

// TestECRRepoListsOnlyItsOwnImages pins the repository side of every image
// pivot: repository "api" lists the workloads running an image of "api" and
// nothing that merely contains "/api" in its URI — not "api-worker", not
// "team/api", not "api" in another account's or region's registry.
func TestECRRepoListsOnlyItsOwnImages(t *testing.T) {
	var tasks, projects, functions []resource.Resource
	for _, img := range ecrIdentImages {
		tasks = append(tasks, ecrIdentTaskRow(img.key, img.image))
		projects = append(projects, ecrIdentProjectRow(img.key, img.image))
		if img.key != "docker-hub" { // Lambda runs images from ECR only
			functions = append(functions, ecrIdentLambdaRow(img.key, lambdatypes.PackageTypeImage))
		}
	}
	functions = append(functions, ecrIdentLambdaRow("zip-handler", lambdatypes.PackageTypeZip))

	cache := resource.ResourceCache{
		"ecr":      ecrIdentRepoCache(),
		"ecs-task": {Resources: tasks},
		"cb":       {Resources: projects},
		"lambda":   {Resources: functions},
	}
	clients := ecrIdentLambdaClients()

	for _, repo := range []string{"api", "api-worker", "team/api"} {
		for _, target := range []string{"ecs-task", "cb", "lambda"} {
			res := checkerByTarget(t, "ecr", target)(context.Background(), clients, ecrIdentRepo(repo), cache)
			label := "ecr " + repo + " -> " + target
			if res.EffectiveState() != domain.RelatedResolved {
				t.Errorf("%s: state = %v (err %v), want a resolved answer", label, res.EffectiveState(), res.Err())
				continue
			}
			assertSameIDs(t, label, res.ResourceIDs(), ecrIdentWant(repo))
		}
	}
}

// TestECRWorkloadListsOnlyTheRepoOfItsImage pins the workload side: a task, a
// service, a build project and a function running an image list the one local
// repository that image belongs to, and none for an image in another
// account's or region's registry. The session here knows its region but not
// yet its account (STS has not answered, or is denied): the repository's URI
// already names its registry, so the answer must not wait on the session.
func TestECRWorkloadListsOnlyTheRepoOfItsImage(t *testing.T) {
	cache := resource.ResourceCache{"ecr": ecrIdentRepoCache()}
	session := &awsclient.ServiceClients{Region: "us-east-1"}
	lambdaClients := ecrIdentLambdaClients()
	lambdaClients.Region = "us-east-1"

	for _, img := range ecrIdentImages {
		var want []string
		if img.repo != "" {
			want = []string{img.repo}
		}
		ctx := context.Background()

		task := checkerByTarget(t, "ecs-task", "ecr")(ctx, session, ecrIdentTaskRow(img.key, img.image), cache)
		assertSameIDs(t, "ecs-task "+img.key+" -> ecr", task.ResourceIDs(), want)

		fakeECS := newFakeECSWithTaskDefinition(&ecstypes.TaskDefinition{
			Family:               aws.String(img.key),
			ContainerDefinitions: []ecstypes.ContainerDefinition{{Name: aws.String("app"), Image: aws.String(img.image)}},
		})
		svc := checkerByTarget(t, "ecs-svc", "ecr")(ctx, &awsclient.ServiceClients{ECS: fakeECS, Region: "us-east-1"},
			ecsSvcWithTaskDef(img.key, "arn:aws:ecs:us-east-1:123456789012:task-definition/"+img.key+":3"), cache)
		assertSameIDs(t, "ecs-svc "+img.key+" -> ecr", svc.ResourceIDs(), want)

		cb := checkerByTarget(t, "cb", "ecr")(ctx, session, ecrIdentProjectRow(img.key, img.image), cache)
		assertSameIDs(t, "cb "+img.key+" -> ecr", cb.ResourceIDs(), want)

		if img.key == "docker-hub" {
			continue
		}
		fn := checkerByTarget(t, "lambda", "ecr")(ctx, lambdaClients, ecrIdentLambdaRow(img.key, lambdatypes.PackageTypeImage), cache)
		assertSameIDs(t, "lambda "+img.key+" -> ecr", fn.ResourceIDs(), want)
	}
}
