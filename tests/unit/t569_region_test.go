package unit_test

// A value that names its Region, and a resource that lives in one fixed
// Region, are read in that Region: a pipeline action's Region, a private
// zone's VPCRegion, a Lambda@Edge function's us-east-1, and the Region a row
// was listed in when its detail opens.
//
// The world below extends the region test world (aws_region_world_test.go)
// with the services these pivots read, answered per Region the way AWS does.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	t569Account     = "123456789012"
	t569Pipeline    = "acme-release"
	t569LocalPipe   = "acme-local-release"
	t569Project     = "acme-api-build"
	t569Repo        = "acme/api-service"
	t569Stack       = "acme-api-stack"
	t569Function    = "acme-release-notifier"
	t569EdgeFn      = "acme-edge-auth"
	t569Cluster     = "acme-services"
	t569Service     = "api-gateway"
	t569ActionRgn   = "eu-west-1"
	t569EUVpc       = "vpc-0eu1111111111111a"
	t569USWVpc      = "vpc-0usw222222222222b"
	t569PrivateZone = "Z0PRIVATE1ACME"
	t569EdgeDist    = "E3EXAMPLEEDGE1"
)

// t569Data is what each Region holds, by type, by name. Both Regions carry a
// resource of every name, the way infrastructure code deployed per Region
// names them: a name identifies a resource only inside its Region.
var t569Data = map[string]map[string][]string{ //nolint:gochecknoglobals // read-only fixture world
	"us-east-1": {"cb": {t569Project}, "ecr": {t569Repo}, "cfn": {t569Stack}, "lambda": {t569Function, t569EdgeFn}, "ecs": {t569Service}},
	"eu-west-1": {"cb": {t569Project}, "ecr": {t569Repo}, "cfn": {t569Stack}, "lambda": {t569Function, t569EdgeFn}, "ecs": {t569Service}, "vpc": {t569EUVpc}},
	"us-west-2": {"vpc": {t569USWVpc}},
}

type t569World struct {
	rw *rwWorld
	// pipelines answers GetPipeline by name.
	pipelines map[string][]map[string]any
	// zoneVPCs answers GetHostedZone for the private zone.
	zoneVPCs []r53types.VPC
}

func newT569World() *t569World {
	return &t569World{rw: newRegionWorld(), pipelines: map[string][]map[string]any{}}
}

func (w *t569World) clients(sessionRegion string) *awsclient.ServiceClients {
	return awsclient.CreateServiceClients(aws.Config{
		Region:           sessionRegion,
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		HTTPClient:       &http.Client{Transport: w},
		RetryMaxAttempts: 1,
	})
}

func (w *t569World) record(service, region, op, subject string) {
	w.rw.mu.Lock()
	defer w.rw.mu.Unlock()
	w.rw.calls = append(w.rw.calls, rwCall{Service: service, Region: region, Op: op, Subject: subject})
}

func (w *t569World) RoundTrip(req *http.Request) (*http.Response, error) {
	region, service := rwScope(req.Header.Get("Authorization"))
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body) //nolint:errcheck // an unreadable body decodes as an empty request
	}
	target := req.Header.Get("X-Amz-Target")
	_, op, _ := strings.Cut(target, ".")
	var in map[string]any
	_ = json.Unmarshal(body, &in)           //nolint:errcheck // a query or empty body is read below
	form, _ := url.ParseQuery(string(body)) //nolint:errcheck // a JSON body parses as no form
	names := t569Data[region]

	var resp *http.Response
	switch {
	case service == "codepipeline" && op == "GetPipeline":
		name, _ := in["name"].(string)
		resp = w.getPipeline(region, name)
	case service == "codebuild" && op == "ListProjects":
		resp = rwJSON(map[string]any{"projects": names["cb"]})
	case service == "codebuild" && op == "BatchGetProjects":
		var projects []map[string]any
		for _, n := range names["cb"] {
			projects = append(projects, map[string]any{
				"name": n, "arn": "arn:aws:codebuild:" + region + ":" + t569Account + ":project/" + n,
				"environment": map[string]any{"type": "LINUX_CONTAINER", "image": "aws/codebuild/standard:7.0", "computeType": "BUILD_GENERAL1_SMALL"},
				"serviceRole": "arn:aws:iam::" + t569Account + ":role/acme-codebuild",
			})
		}
		resp = rwJSON(map[string]any{"projects": projects})
	case service == "ecr" && op == "DescribeRepositories":
		var repos []map[string]any
		for _, n := range names["ecr"] {
			repos = append(repos, map[string]any{
				"repositoryName": n, "registryId": t569Account,
				"repositoryArn":      "arn:aws:ecr:" + region + ":" + t569Account + ":repository/" + n,
				"repositoryUri":      t569Account + ".dkr.ecr." + region + ".amazonaws.com/" + n,
				"imageTagMutability": "IMMUTABLE", "createdAt": 1.7e9,
			})
		}
		resp = rwJSON(map[string]any{"repositories": repos})
	case service == "ecs" && op == "ListClusters":
		resp = rwJSON(map[string]any{"clusterArns": []string{"arn:aws:ecs:" + region + ":" + t569Account + ":cluster/" + t569Cluster}})
	case service == "ecs" && op == "ListServices":
		var arns []string
		for _, n := range names["ecs"] {
			arns = append(arns, "arn:aws:ecs:"+region+":"+t569Account+":service/"+t569Cluster+"/"+n)
		}
		resp = rwJSON(map[string]any{"serviceArns": arns})
	case service == "ecs" && op == "DescribeServices":
		var svcs []map[string]any
		for _, n := range names["ecs"] {
			svcs = append(svcs, map[string]any{
				"serviceName": n, "serviceArn": "arn:aws:ecs:" + region + ":" + t569Account + ":service/" + t569Cluster + "/" + n,
				"clusterArn": "arn:aws:ecs:" + region + ":" + t569Account + ":cluster/" + t569Cluster,
				"status":     "ACTIVE", "desiredCount": 2, "runningCount": 2, "launchType": "FARGATE",
				"taskDefinition": "arn:aws:ecs:" + region + ":" + t569Account + ":task-definition/" + n + ":4",
			})
		}
		resp = rwJSON(map[string]any{"services": svcs})
	case service == "lambda" && strings.HasPrefix(req.URL.Path, "/2015-03-31/functions"):
		var fns []map[string]any
		for _, n := range names["lambda"] {
			fns = append(fns, map[string]any{
				"FunctionName": n, "FunctionArn": "arn:aws:lambda:" + region + ":" + t569Account + ":function:" + n,
				"Runtime": "nodejs20.x", "PackageType": "Zip", "State": "Active", "LastModified": "2026-01-10T09:00:00.000+0000",
			})
		}
		resp = rwResponse(200, "application/json", mustJSON(map[string]any{"Functions": fns}), nil)
	case service == "cloudformation" && form.Get("Action") == "DescribeStacks":
		var members strings.Builder
		for _, n := range names["cfn"] {
			fmt.Fprintf(&members, `<member><StackName>%s</StackName><StackId>arn:aws:cloudformation:%s:%s:stack/%s/0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d</StackId><StackStatus>UPDATE_COMPLETE</StackStatus><CreationTime>2025-06-01T10:00:00Z</CreationTime></member>`, n, region, t569Account, n)
		}
		resp = rwResponse(200, "text/xml", []byte(`<DescribeStacksResponse xmlns="http://cloudformation.amazonaws.com/doc/2010-05-15/"><DescribeStacksResult><Stacks>`+members.String()+`</Stacks></DescribeStacksResult><ResponseMetadata><RequestId>1</RequestId></ResponseMetadata></DescribeStacksResponse>`), nil)
	case service == "ec2" && form.Get("Action") == "DescribeVpcs":
		var items strings.Builder
		for _, id := range names["vpc"] {
			fmt.Fprintf(&items, `<item><vpcId>%s</vpcId><ownerId>%s</ownerId><state>available</state><cidrBlock>10.20.0.0/16</cidrBlock><isDefault>false</isDefault></item>`, id, t569Account)
		}
		resp = rwResponse(200, "text/xml", []byte(`<DescribeVpcsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>1</requestId><vpcSet>`+items.String()+`</vpcSet></DescribeVpcsResponse>`), nil)
	case service == "route53" && strings.Contains(req.URL.Path, "/hostedzone/"+t569PrivateZone):
		resp = w.getHostedZone()
	case service == "cloudfront" && strings.HasSuffix(req.URL.Path, "/distribution/"+t569EdgeDist+"/config"):
		resp = t569EdgeConfig()
	default:
		req.Body = io.NopCloser(strings.NewReader(string(body)))
		return w.rw.RoundTrip(req)
	}
	subject := req.URL.Path
	if n, ok := in["name"].(string); ok {
		subject = n
	}
	w.record(service, region, op+form.Get("Action"), subject)
	return resp, nil
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v) //nolint:errcheck // test-built maps always marshal
	return b
}

// t569Action is one pipeline action of provider, in region ("" for the
// pipeline's own Region), with its configuration.
func t569Action(name, category, provider, region string, config map[string]string) map[string]any {
	a := map[string]any{
		"name":          name,
		"actionTypeId":  map[string]any{"category": category, "owner": "AWS", "provider": provider, "version": "1"},
		"configuration": config,
		"runOrder":      1,
	}
	if region != "" {
		a["region"] = region
	}
	return a
}

func (w *t569World) getPipeline(region, name string) *http.Response {
	actions, ok := w.pipelines[name]
	if !ok {
		return rwJSONError(400, "PipelineNotFoundException", "Account '"+t569Account+"' does not have a pipeline with name '"+name+"'")
	}
	var stages []map[string]any
	for i, a := range actions {
		stages = append(stages, map[string]any{"name": "Stage" + strconv.Itoa(i+1), "actions": []map[string]any{a}})
	}
	return rwJSON(map[string]any{"pipeline": map[string]any{
		"name":          name,
		"roleArn":       "arn:aws:iam::" + t569Account + ":role/acme-codepipeline",
		"version":       3,
		"artifactStore": map[string]any{"type": "S3", "location": "acme-pipeline-artifacts-" + region},
		"stages":        stages,
	}})
}

func (w *t569World) getHostedZone() *http.Response {
	var vpcs strings.Builder
	for _, v := range w.zoneVPCs {
		fmt.Fprintf(&vpcs, `<VPC><VPCRegion>%s</VPCRegion><VPCId>%s</VPCId></VPC>`, v.VPCRegion, aws.ToString(v.VPCId))
	}
	return rwResponse(200, "text/xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<GetHostedZoneResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><HostedZone><Id>/hostedzone/`+t569PrivateZone+`</Id><Name>internal.acme-example.com.</Name><CallerReference>acme-internal</CallerReference><Config><PrivateZone>true</PrivateZone></Config><ResourceRecordSetCount>4</ResourceRecordSetCount></HostedZone><VPCs>`+vpcs.String()+`</VPCs></GetHostedZoneResponse>`), nil)
}

func t569EdgeConfig() *http.Response {
	return rwResponse(200, "text/xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<DistributionConfig xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/"><CallerReference>acme-edge</CallerReference><Comment></Comment><Enabled>true</Enabled><Origins><Quantity>1</Quantity><Items><Origin><Id>web</Id><DomainName>acme-web-use1.s3.us-east-1.amazonaws.com</DomainName></Origin></Items></Origins><DefaultCacheBehavior><TargetOriginId>web</TargetOriginId><ViewerProtocolPolicy>redirect-to-https</ViewerProtocolPolicy><LambdaFunctionAssociations><Quantity>1</Quantity><Items><LambdaFunctionAssociation><LambdaFunctionARN>arn:aws:lambda:us-east-1:`+t569Account+`:function:`+t569EdgeFn+`:3</LambdaFunctionARN><EventType>viewer-request</EventType><IncludeBody>false</IncludeBody></LambdaFunctionAssociation></Items></LambdaFunctionAssociations></DefaultCacheBehavior></DistributionConfig>`),
		http.Header{"Etag": []string{"E2QWRUHEXAMPLE"}})
}

// t569Row returns the row of typ named name as typ's fetcher lists it in the
// Region clients answer for.
func t569Row(t *testing.T, clients *awsclient.ServiceClients, typ, name string) resource.Resource {
	t.Helper()
	for _, r := range rwFetch(t, clients, typ) {
		if r.Name == name || r.ID == name {
			r.Type = typ
			return r
		}
	}
	t.Fatalf("the %s list in %s has no %q", typ, clients.Region, name)
	return resource.Resource{}
}

// ─── a pipeline action's Region ─────────────────────────────

// TestT569Pipeline_ActionRegionNamesTheTarget: ActionDeclaration.Region is
// the Region an action runs in, and the project, stack, repository, service
// or function its configuration names is the one in that Region. The
// same-named resource of the session's Region is another resource.
func TestT569Pipeline_ActionRegionNamesTheTarget(t *testing.T) {
	w := newT569World()
	session := w.clients("us-east-1")
	elsewhere := session.InRegion(t569ActionRgn)
	w.pipelines[t569Pipeline] = []map[string]any{
		t569Action("Source", "Source", "ECR", t569ActionRgn, map[string]string{"RepositoryName": t569Repo, "ImageTag": "latest"}),
		t569Action("Build", "Build", "CodeBuild", t569ActionRgn, map[string]string{"ProjectName": t569Project}),
		t569Action("Deploy", "Deploy", "CloudFormation", t569ActionRgn, map[string]string{"ActionMode": "CREATE_UPDATE", "StackName": t569Stack}),
		t569Action("DeployECS", "Deploy", "ECS", t569ActionRgn, map[string]string{"ClusterName": t569Cluster, "ServiceName": t569Service}),
		t569Action("Notify", "Invoke", "Lambda", t569ActionRgn, map[string]string{"FunctionName": t569Function}),
	}
	pipeline := resource.Resource{ID: t569Pipeline, Name: t569Pipeline, Type: "pipeline",
		Fields: map[string]string{"arn": "arn:aws:codepipeline:us-east-1:" + t569Account + ":" + t569Pipeline}}
	cache := resource.ResourceCache{}
	for _, typ := range []string{"cb", "ecr", "cfn", "ecs-svc", "lambda"} {
		cache[typ] = resource.ResourceCacheEntry{Resources: rwFetch(t, session, typ)}
	}

	for typ, name := range map[string]string{"cb": t569Project, "ecr": t569Repo, "cfn": t569Stack, "ecs-svc": t569Service, "lambda": t569Function} {
		t.Run("pipeline→"+typ, func(t *testing.T) {
			want := t569Row(t, elsewhere, typ, name).ID
			got := rwChecker(t, "pipeline", typ)(context.Background(), session, pipeline, cache)
			if !slices.Equal(got.ResourceIDs(), []string{want}) || got.Region() != t569ActionRgn {
				t.Errorf("pipeline → %s for an action in %s = %s %v Region %q, want [%s] in %s: the session's same-named %s is another resource",
					typ, t569ActionRgn, t569Badge(got), got.ResourceIDs(), got.Region(), want, t569ActionRgn, typ)
			}
		})
	}

	t.Run("an action in the pipeline's own Region", func(t *testing.T) {
		w.pipelines[t569LocalPipe] = []map[string]any{t569Action("Build", "Build", "CodeBuild", "", map[string]string{"ProjectName": t569Project})}
		local := resource.Resource{ID: t569LocalPipe, Name: t569LocalPipe, Type: "pipeline"}
		got := rwChecker(t, "pipeline", "cb")(context.Background(), session, local, cache)
		if want := t569Row(t, session, "cb", t569Project).ID; !slices.Equal(got.ResourceIDs(), []string{want}) || got.Region() != "" {
			t.Errorf("pipeline → cb for an action with no Region = %v Region %q, want [%s] in the session's Region", got.ResourceIDs(), got.Region(), want)
		}
	})
}

// TestT569Pipeline_ReverseCountsOnlyItsOwnRegionsActions: a pipeline acts on
// the session's project only through an action in the session's Region.
func TestT569Pipeline_ReverseCountsOnlyItsOwnRegionsActions(t *testing.T) {
	w := newT569World()
	session := w.clients("us-east-1")
	w.pipelines[t569Pipeline] = []map[string]any{
		t569Action("Source", "Source", "ECR", t569ActionRgn, map[string]string{"RepositoryName": t569Repo}),
		t569Action("Build", "Build", "CodeBuild", t569ActionRgn, map[string]string{"ProjectName": t569Project}),
	}
	w.pipelines[t569LocalPipe] = []map[string]any{
		t569Action("Source", "Source", "ECR", "", map[string]string{"RepositoryName": t569Repo}),
		t569Action("Build", "Build", "CodeBuild", "", map[string]string{"ProjectName": t569Project}),
	}
	cache := resource.ResourceCache{"pipeline": {Resources: []resource.Resource{
		{ID: t569Pipeline, Name: t569Pipeline, Type: "pipeline"},
		{ID: t569LocalPipe, Name: t569LocalPipe, Type: "pipeline"},
	}}}
	for typ, name := range map[string]string{"cb": t569Project, "ecr": t569Repo} {
		t.Run(typ+"→pipeline", func(t *testing.T) {
			got := rwChecker(t, typ, "pipeline")(context.Background(), session, t569Row(t, session, typ, name), cache)
			if !slices.Equal(got.ResourceIDs(), []string{t569LocalPipe}) {
				t.Errorf("session %s %s → pipeline = %v, want [%s]: %s's action names the %s one", typ, name, got.ResourceIDs(), t569LocalPipe, t569Pipeline, t569ActionRgn)
			}
		})
	}
}

func t569Badge(r resource.RelatedCheckResult) string {
	return resource.FormatRelatedCount(r.State(), r.Count(), r.Truncated())
}

// ─── a private zone's VPCs in their own Regions ─────────────

// TestT569R53VPC_EachVPCInItsVPCRegion: GetHostedZone names each associated
// VPC with its VPCRegion; a VPC of another Region is that Region's.
func TestT569R53VPC_EachVPCInItsVPCRegion(t *testing.T) {
	w := newT569World()
	session := w.clients("eu-west-1")
	zone := resource.Resource{ID: "/hostedzone/" + t569PrivateZone, Name: "internal.acme-example.com.", Type: "r53",
		Fields: map[string]string{"zone_id": "/hostedzone/" + t569PrivateZone, "name": "internal.acme-example.com.", "private_zone": "true"}}
	cache := resource.ResourceCache{"vpc": {Resources: rwFetch(t, session, "vpc")}}
	check := rwChecker(t, "r53", "vpc")

	w.zoneVPCs = []r53types.VPC{{VPCId: aws.String(t569EUVpc), VPCRegion: r53types.VPCRegionEuWest1}}
	if got := check(context.Background(), session, zone, cache); !slices.Equal(got.ResourceIDs(), []string{t569EUVpc}) || got.Region() != "" {
		t.Errorf("zone on a session-Region VPC → vpc = %v Region %q, want [%s] in the session's Region", got.ResourceIDs(), got.Region(), t569EUVpc)
	}

	w.zoneVPCs = []r53types.VPC{{VPCId: aws.String(t569USWVpc), VPCRegion: r53types.VPCRegionUsWest2}}
	got := check(context.Background(), session, zone, cache)
	if slices.Contains(got.ResourceIDs(), t569USWVpc) && got.Region() != "us-west-2" {
		t.Errorf("zone on a us-west-2 VPC → vpc = %s %v Region %q, want the VPC read in us-west-2, not the session's Region", t569Badge(got), got.ResourceIDs(), got.Region())
	}
	if !slices.Contains(got.ResourceIDs(), t569USWVpc) && t569Badge(got) == "(0)" {
		t.Errorf("zone on a us-west-2 VPC → vpc = (0), a proven zero for a zone that names a VPC")
	}
}

// ─── Lambda@Edge functions are in us-east-1 ─────────────────

// TestT569CFLambda_EdgeFunctionsResolveInUSEast1: a Lambda@Edge function is
// created in us-east-1 and its association names it by a us-east-1 ARN,
// whatever Region the session is in.
func TestT569CFLambda_EdgeFunctionsResolveInUSEast1(t *testing.T) {
	w := newT569World()
	session := w.clients("eu-west-1")
	dist := resource.Resource{ID: t569EdgeDist, Name: t569EdgeDist, Type: "cf", Fields: map[string]string{"domain_name": "d222222abcdef8.cloudfront.net"}}
	cache := resource.ResourceCache{"lambda": {Resources: rwFetch(t, session, "lambda")}}

	got := rwChecker(t, "cf", "lambda")(context.Background(), session, dist, cache)
	if got.Truncated() || !slices.Equal(got.ResourceIDs(), []string{t569EdgeFn}) || got.Region() != "us-east-1" {
		t.Errorf("cf → lambda from eu-west-1 = %s %v Region %q, want (1) [%s] in us-east-1", t569Badge(got), got.ResourceIDs(), got.Region(), t569EdgeFn)
	}
}

// ─── a detail opened on another Region's row ─────────

// t569Controller is a session in eu-west-1 over the world, with the cf
// list open on the given distributions.
func t569Controller(t *testing.T, w *t569World, dists ...resource.Resource) (*app.Controller, *runtime.Core) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "eu-west-1"
	s.Clients = w.clients("eu-west-1")
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "cf"})
	c.ApplyResourcesLoaded("cf", dists, nil, false)
	return c, core
}

// t569Run executes tasks the way the executor does and hands each result back
// to the controller, returning the tasks those results spawned.
func t569Run(t *testing.T, c *app.Controller, core *runtime.Core, tasks []runtime.TaskRequest) []runtime.TaskRequest {
	t.Helper()
	var next []runtime.TaskRequest
	for _, task := range tasks {
		ev, err := core.ExecuteTask(context.Background(), task)
		if err != nil || ev == nil {
			continue
		}
		_, more := c.Handle(ev)
		next = append(next, more...)
	}
	return next
}

// t569DetailOps returns the operations of the detail tasks among tasks.
func t569DetailOps(tasks []runtime.TaskRequest) []runtime.DetailOperation {
	var ops []runtime.DetailOperation
	for _, task := range tasks {
		switch p := task.Payload.(type) {
		case runtime.EnrichDetailPayload:
			ops = append(ops, p.Op)
		case runtime.RelatedCheckPayload:
			ops = append(ops, p.Op)
		}
	}
	return ops
}

// TestT569Detail_ARowListedInUSEast1RunsInUSEast1: cf → acm lists the
// distribution's certificate in us-east-1. The certificate's detail, its
// related rows and its CloudTrail row read us-east-1, where it is.
func TestT569Detail_ARowListedInUSEast1RunsInUSEast1(t *testing.T) {
	w := newT569World()
	c, core := t569Controller(t, w, rwDistribution())
	c.Apply(app.Action{Kind: app.ActionSelect})

	got := rwChecker(t, "cf", "acm")(context.Background(), core.Session().Clients, rwDistribution(), resource.ResourceCache{})
	def, idx := rdRelatedDef(t, "cf", "acm")
	c.ApplyDetailRelatedResultForResource("cf", rwDistID, def.DisplayName, def.TargetType,
		got.EffectiveState(), got.Count(), false, "", got.Truncated(), got.ResourceIDs(), got.FetchFilter(), got.Region())
	if got.Region() != "us-east-1" {
		t.Fatalf("cf → acm Region = %q, want us-east-1 (the drill's precondition)", got.Region())
	}

	_, drill := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx)})
	opened := t569Run(t, c, core, drill)
	ops := t569DetailOps(opened)
	if len(ops) == 0 {
		_, sel := c.Apply(app.Action{Kind: app.ActionSelect})
		opened = sel
		ops = t569DetailOps(sel)
	}
	if len(ops) == 0 {
		t.Fatal("opening the certificate listed by cf → acm began no detail work")
	}
	for _, op := range ops {
		if op.Resource.ID != rwEdgeCertArn {
			t.Fatalf("the detail opened on %q, want the us-east-1 certificate %s", op.Resource.ID, rwEdgeCertArn)
		}
		if op.Clients == nil || op.Clients.Region != "us-east-1" {
			region := "<nil>"
			if op.Clients != nil {
				region = op.Clients.Region
			}
			t.Errorf("the certificate's detail operation reads Region %s, want us-east-1 where the row was listed", region)
		}
	}

	w.rw.resetCalls()
	t569Run(t, c, core, opened)
	w.rw.requireCallsOnlyIn(t, "acm", rwEdgeCertArn, "us-east-1")
	for _, svc := range []string{"elasticloadbalancing", "apigateway", "cloudtrail"} {
		for _, call := range w.rw.callsFor(svc, "") {
			if call.Region != "us-east-1" {
				t.Errorf("the certificate's %s read (%s) went to %s, want us-east-1", svc, call.Op, call.Region)
			}
		}
	}

	w.rw.resetCalls()
	_, refresh := c.Apply(app.Action{Kind: app.ActionRefresh})
	for _, op := range t569DetailOps(refresh) {
		if op.Clients == nil || op.Clients.Region != "us-east-1" {
			t.Errorf("refreshing the certificate's detail moved its operation off us-east-1")
		}
	}
}

// TestT569Detail_CrossRegionListIsReadOncePerSession: two details that need
// us-east-1's certificate list read it once; it never becomes the session
// Region's own certificate list.
func TestT569Detail_CrossRegionListIsReadOncePerSession(t *testing.T) {
	w := newT569World()
	second := rwDistribution()
	second.ID, second.Name = "E4EXAMPLE2ACME", "E4EXAMPLE2ACME"
	c, core := t569Controller(t, w, rwDistribution(), second)

	w.rw.resetCalls()
	_, first := c.Apply(app.Action{Kind: app.ActionSelect})
	t569Run(t, c, core, first)
	c.Apply(app.Action{Kind: app.ActionBack})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	_, next := c.Apply(app.Action{Kind: app.ActionSelect})
	t569Run(t, c, core, next)

	var lists int
	for _, call := range w.rw.callsFor("acm", "") {
		if call.Op == "ListCertificates" && call.Region == "us-east-1" {
			lists++
		}
	}
	if lists != 1 {
		t.Errorf("us-east-1 ListCertificates issued %d times across two details, want once", lists)
	}
	for _, r := range core.BuildResourceCacheSnapshot()["acm"].Resources {
		if r.ID == rwEdgeCertArn {
			t.Errorf("the us-east-1 certificate %s entered the session's eu-west-1 certificate list", rwEdgeCertArn)
		}
	}
}
