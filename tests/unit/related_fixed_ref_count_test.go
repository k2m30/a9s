package unit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// fixedRefRows builds the target rows a reference resolves to.
func fixedRefRows(ids ...string) []resource.Resource {
	out := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		out = append(out, resource.Resource{ID: id, Name: id, Fields: map[string]string{}})
	}
	return out
}

// fixedRefCase is one pivot that resolves the references its source names:
// src names exactly the rows found, in the order they appear, and foreign is
// a further reference of another account, which names no row of this one.
type fixedRefCase struct {
	label     string
	shortName string
	target    string
	found     []string
	src       func(refs []string) resource.Resource
	clients   func(refs []string) *awsclient.ServiceClients
	refFor    func(id string) string
	foreign   string
}

func fixedRefECSTaskDefinition(groups []string) *ecstypes.TaskDefinition {
	def := &ecstypes.TaskDefinition{
		Family:            aws.String("acme-api"),
		Revision:          7,
		TaskDefinitionArn: aws.String(onceTaskDefARN),
	}
	for i, g := range groups {
		def.ContainerDefinitions = append(def.ContainerDefinitions, ecstypes.ContainerDefinition{
			Name:  aws.String(fmt.Sprintf("container-%d", i)),
			Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme-api:v1.4.2"),
			LogConfiguration: &ecstypes.LogConfiguration{
				LogDriver: ecstypes.LogDriverAwslogs,
				Options:   map[string]string{"awslogs-group": g, "awslogs-region": "us-east-1"},
			},
		})
	}
	return def
}

func fixedRefECSClients(refs []string) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{
		ECS:    newFakeECSWithTaskDefinition(fixedRefECSTaskDefinition(refs)),
		Region: "us-east-1",
	}
}

func fixedRefNoClients(_ []string) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{Region: "us-east-1"}
}

func fixedRefCases() []fixedRefCase {
	roleARN := func(name string) string { return "arn:aws:iam::123456789012:role/" + name }
	lambdaARN := func(name string) string {
		return "arn:aws:lambda:us-east-1:123456789012:function:" + name
	}
	return []fixedRefCase{
		{
			label: "ecs-svc names its containers' log groups", shortName: "ecs-svc", target: "logs",
			found:   []string{"/ecs/acme-api", "/ecs/acme-api-sidecar"},
			src:     func([]string) resource.Resource { return ecsSvcWithTaskDef("acme-api-svc", onceTaskDefARN) },
			clients: fixedRefECSClients,
			refFor:  func(id string) string { return id },
		},
		{
			label: "ecs-task names its containers' log groups", shortName: "ecs-task", target: "logs",
			found:   []string{"/ecs/acme-api", "/ecs/acme-api-sidecar"},
			src:     func([]string) resource.Resource { return onceECSTaskRow() },
			clients: fixedRefECSClients,
			refFor:  func(id string) string { return id },
		},
		{
			label: "an ECS cluster names its exec-command log group", shortName: "ecs", target: "logs",
			found:   []string{"/ecs/exec/acme-prod"},
			src:     func(refs []string) resource.Resource { return claimCluster("acme-prod", refs[0]) },
			clients: fixedRefNoClients,
			refFor:  func(id string) string { return id },
		},
		{
			label: "a build project names its artifact buckets", shortName: "cb", target: "s3",
			found: []string{"acme-build-artifacts", "acme-build-cache"},
			src: func(refs []string) resource.Resource {
				project := cbtypes.Project{
					Name:      aws.String("acme-api-build"),
					Arn:       aws.String("arn:aws:codebuild:us-east-1:123456789012:project/acme-api-build"),
					Artifacts: &cbtypes.ProjectArtifacts{Type: cbtypes.ArtifactsTypeS3, Location: aws.String(refs[0])},
				}
				for _, extra := range refs[1:] {
					project.SecondaryArtifacts = append(project.SecondaryArtifacts,
						cbtypes.ProjectArtifacts{Type: cbtypes.ArtifactsTypeS3, Location: aws.String(extra)})
				}
				return resource.Resource{ID: "acme-api-build", Name: "acme-api-build", Fields: map[string]string{}, RawStruct: project}
			},
			clients: fixedRefNoClients,
			refFor:  func(id string) string { return id },
		},
		{
			label: "a task names its task and execution roles", shortName: "ecs-task", target: "role",
			found: []string{"acme-api-task", "acme-api-exec"},
			src: func(refs []string) resource.Resource {
				row := onceECSTaskRow()
				row.Fields["task_role"] = refs[0]
				row.Fields["execution_role"] = ""
				if len(refs) > 1 {
					row.Fields["execution_role"] = refs[1]
				}
				return row
			},
			clients: fixedRefNoClients,
			refFor:  roleARN,
			foreign: "arn:aws:iam::210987654321:role/another-accounts-role",
		},
		{
			label: "a secret names its rotation function", shortName: "secrets", target: "lambda",
			found: []string{"acme-api-rotator"},
			src: func(refs []string) resource.Resource {
				return resource.Resource{
					ID: "acme/api/db", Name: "acme/api/db", Fields: map[string]string{},
					RawStruct: smtypes.SecretListEntry{
						Name:              aws.String("acme/api/db"),
						ARN:               aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/api/db-AbCdEf"),
						RotationEnabled:   aws.Bool(true),
						RotationLambdaARN: aws.String(refs[0]),
					},
				}
			},
			clients: fixedRefNoClients,
			refFor:  lambdaARN,
		},
	}
}

// fixedRefRun answers one case's pivot: rows are the target rows loaded,
// truncated is the target list's own truncation, and refs are the references
// the source names.
func fixedRefRun(t *testing.T, tc fixedRefCase, refs, rows []string, truncated bool) resource.RelatedCheckResult {
	t.Helper()
	cache := onceECSCache(tc.shortName)
	cache[tc.target] = resource.ResourceCacheEntry{Resources: fixedRefRows(rows...), IsTruncated: truncated}
	refValues := make([]string, 0, len(refs))
	for _, r := range refs {
		refValues = append(refValues, tc.refFor(r))
	}
	return checkerByTarget(t, tc.shortName, tc.target)(
		context.Background(), tc.clients(refValues), tc.src(refValues), cache)
}

// TestFixedReferenceCountIsExactWhateverTheTargetListTruncation pins what a
// pivot may claim when the source itself names the rows. A task definition
// names its log groups, a task names its roles, a secret names its rotation
// function: the set is closed, so a page of the target list nobody read can
// hold more rows of that type but none this source named. Every named row
// found is therefore an exact count, and the "+" that says another row of
// this relationship may exist would be false.
func TestFixedReferenceCountIsExactWhateverTheTargetListTruncation(t *testing.T) {
	for _, tc := range fixedRefCases() {
		t.Run(tc.label, func(t *testing.T) {
			res := fixedRefRun(t, tc, tc.found, tc.found, true)
			assertSameIDs(t, tc.label, res.ResourceIDs(), tc.found)
			want := fmt.Sprintf("(%d)", len(tc.found))
			if cell := claimCell(res); cell != want {
				t.Errorf("%s -> %s renders %q over a truncated %s list, want %q: the source names every row this relationship has",
					tc.shortName, tc.target, cell, tc.target, want)
			}
			if got := res.Coverage(); got != domain.CoverageComplete {
				t.Errorf("%s -> %s coverage = %v, want %v", tc.shortName, tc.target, got, domain.CoverageComplete)
			}
		})
	}
}

// TestFixedReferenceCountIsALowerBoundWhenAReferenceWasNotResolved pins the
// other half: a reference the source names and the lookup could not turn into
// a row is a row missing from the answer, so the count is a lower bound. It
// holds whether the row is absent because the target list stopped short of it
// or because the reference names another account, and in the second case even
// over a list read to its end.
func TestFixedReferenceCountIsALowerBoundWhenAReferenceWasNotResolved(t *testing.T) {
	for _, tc := range fixedRefCases() {
		t.Run(tc.label+", one row missing from a truncated list", func(t *testing.T) {
			rows := tc.found[1:]
			res := fixedRefRun(t, tc, tc.found, rows, true)
			assertSameIDs(t, tc.label, res.ResourceIDs(), rows)
			want := fmt.Sprintf("(%d+)", len(rows))
			if cell := claimCell(res); cell != want {
				t.Errorf("%s -> %s renders %q, want %q: the source names %d rows and the lookup produced %d",
					tc.shortName, tc.target, cell, want, len(tc.found), len(rows))
			}
			if got := res.Coverage(); got != domain.CoveragePartial {
				t.Errorf("%s -> %s coverage = %v, want %v", tc.shortName, tc.target, got, domain.CoveragePartial)
			}
		})

		if tc.foreign == "" {
			continue
		}
		t.Run(tc.label+", one reference naming a row the account does not hold", func(t *testing.T) {
			cache := onceECSCache(tc.shortName)
			cache[tc.target] = resource.ResourceCacheEntry{Resources: fixedRefRows(tc.found[0])}
			refs := []string{tc.refFor(tc.found[0]), tc.foreign}
			res := checkerByTarget(t, tc.shortName, tc.target)(
				context.Background(), tc.clients(refs), tc.src(refs), cache)
			assertSameIDs(t, tc.label, res.ResourceIDs(), tc.found[:1])
			if cell := claimCell(res); cell != "(1+)" {
				t.Errorf("%s -> %s renders %q over a complete %s list, want \"(1+)\": one of the references named no row this account holds",
					tc.shortName, tc.target, cell, tc.target)
			}
		})
	}
}

// TestScanningPivotOverATruncatedListStaysALowerBound pins the pivot the rule
// above does not reach. A route table naming an internet gateway is found by
// reading route tables, not by the gateway naming them, so a page of the
// route-table list nobody read may hold another one — the count is what was
// seen so far and says so.
func TestScanningPivotOverATruncatedListStaysALowerBound(t *testing.T) {
	const igwID = "igw-0live000000000001"
	rtb := resource.Resource{
		ID: "rtb-0a1b2c3d4e5f60718", Name: "acme-app-public",
		Fields: map[string]string{"route_table_id": "rtb-0a1b2c3d4e5f60718"},
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-0a1b2c3d4e5f60718"),
			VpcId:        aws.String("vpc-0a1b2c3d4e5f60718"),
			OwnerId:      aws.String("123456789012"),
			Routes: []ec2types.Route{{
				DestinationCidrBlock: aws.String("0.0.0.0/0"),
				GatewayId:            aws.String(igwID),
				State:                ec2types.RouteStateActive,
				Origin:               ec2types.RouteOriginCreateRoute,
			}},
		},
	}
	igw := resource.Resource{
		ID: igwID, Name: igwID, Fields: map[string]string{},
		RawStruct: ec2types.InternetGateway{
			InternetGatewayId: aws.String(igwID),
			OwnerId:           aws.String("123456789012"),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String("vpc-0a1b2c3d4e5f60718"), State: ec2types.AttachmentStatusAttached},
			},
		},
	}
	ctx := context.Background()

	partial := resource.ResourceCache{"rtb": {Resources: []resource.Resource{rtb}, IsTruncated: true}}
	res := checkerByTarget(t, "igw", "rtb")(ctx, nil, igw, partial)
	assertSameIDs(t, "igw "+igwID+" -> rtb over a truncated rtb list", res.ResourceIDs(), []string{rtb.ID})
	if cell := claimCell(res); cell != "(1+)" {
		t.Errorf("igw %s -> rtb renders %q over a truncated rtb list, want \"(1+)\": another unread route table may route to it", igwID, cell)
	}

	complete := resource.ResourceCache{"rtb": {Resources: []resource.Resource{rtb}}}
	if cell := claimCell(checkerByTarget(t, "igw", "rtb")(ctx, nil, igw, complete)); cell != "(1)" {
		t.Errorf("igw %s -> rtb renders %q over an rtb list read to its end, want \"(1)\"", igwID, cell)
	}
}
