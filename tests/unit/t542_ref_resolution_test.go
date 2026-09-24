package unit_test

// Every id a related pivot emits and every navigable value is read through the
// target type's own resolver, the same way on the panel and on Enter, and a
// resolver accepts only what the target's list can hold. A value read any
// other way counts a row nothing opens, or opens the wrong row.

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	sfnsvc "github.com/aws/aws-sdk-go-v2/service/sfn"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// ─── row 1: CloudTrail TARGET rows open their own rows ─────────────────────

type t542Resource struct {
	ARN       string `json:"ARN"`
	AccountID string `json:"accountId"`
	Type      string `json:"type"`
}

// t542Event is one CloudTrail management event as LookupEvents returns it:
// the event record JSON carries the resources[] envelope and the request
// parameters the TARGET section is built from.
func t542Event(id, name, source string, resources []t542Resource, params map[string]any) cloudtrailtypes.Event {
	rec := map[string]any{
		"eventVersion": "1.09",
		"userIdentity": map[string]any{
			"type":      "AssumedRole",
			"arn":       "arn:aws:sts::" + refAccount + ":assumed-role/acme-ops/jdoe",
			"accountId": refAccount,
		},
		"eventTime":          "2026-09-20T10:15:00Z",
		"eventSource":        source,
		"eventName":          name,
		"awsRegion":          refRegion,
		"recipientAccountId": refAccount,
		"eventType":          "AwsApiCall",
		"eventCategory":      "Management",
		"requestParameters":  params,
	}
	if len(resources) > 0 {
		rec["resources"] = resources
	}
	body, err := json.Marshal(rec)
	if err != nil {
		panic(err)
	}
	ev := cloudtrailtypes.Event{
		EventId:         aws.String(id),
		EventName:       aws.String(name),
		EventSource:     aws.String(source),
		CloudTrailEvent: aws.String(string(body)),
	}
	for _, r := range resources {
		ev.Resources = append(ev.Resources, cloudtrailtypes.Resource{ResourceName: aws.String(r.ARN), ResourceType: aws.String(r.Type)})
	}
	return ev
}

// t542EventFields opens the event's detail on the demo bench, with the
// session's identity known, and returns its rendered rows.
func t542EventFields(t *testing.T, c *app.Controller, event cloudtrailtypes.Event) []app.FieldRow {
	t.Helper()
	page, err := awsclient.FetchCloudTrailEventsPage(t.Context(), &ctOneEventAPI{event: event}, "")
	if err != nil || len(page.Resources) != 1 {
		t.Fatalf("building the event: %v (%d rows)", err, len(page.Resources))
	}
	c.ApplyIntents([]runtime.UIIntent{runtime.SetIdentityIntent{
		Identity: &domain.CallerIdentity{AccountID: refAccount, Arn: "arn:aws:iam::" + refAccount + ":user/demo"},
	}})
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ct-events", ResourceID: page.Resources[0].ID},
	}})
	c.EnsureDetailState(page.Resources[0], "ct-events")
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("the event opened no detail body")
	}
	return body.Fields
}

func t542IsTargetRow(f app.FieldRow) bool {
	return strings.HasPrefix(f.Path, "TARGET") && !f.IsSection && !f.IsHeader && !f.IsSpacer
}

// t542TargetRows returns the rows of the event detail's TARGET section.
func t542TargetRows(t *testing.T, c *app.Controller, event cloudtrailtypes.Event) []app.FieldRow {
	t.Helper()
	var rows []app.FieldRow
	for _, f := range t542EventFields(t, c, event) {
		if t542IsTargetRow(f) {
			rows = append(rows, f)
		}
	}
	return rows
}

func TestCTTarget_EachResourceOpensItsOwnRow(t *testing.T) {
	b := newRefBench(t)
	c := refDetailController(t, b)

	const user = "ci-service-account"
	secret := b.row(t, "secrets", "prod/database/primary")
	bucket := b.byType["s3"][0].ID
	fn := b.byType["lambda"][0].ID
	vpc := b.byType["vpc"][0].ID
	sg := b.byType["sg"][0].ID
	subnet := b.byType["subnet"][0].ID
	arnEC2 := "arn:aws:ec2:" + refRegion + ":" + refAccount + ":"
	if secret.Fields["arn"] == "" || !b.has("iam-user", user) {
		t.Fatalf("demo bench lacks a secret ARN (%q) or the pathed user %q", secret.Fields["arn"], user)
	}

	cases := []struct {
		name       string
		event      cloudtrailtypes.Event
		wantType   string
		wantID     string
		notLabeled string
	}{
		{"IAM user with a path", t542Event("e-t542-01", "AttachUserPolicy", "iam.amazonaws.com",
			[]t542Resource{{"arn:aws:iam::" + refAccount + ":user/service-accounts/" + user, refAccount, "AWS::IAM::User"}}, nil),
			"iam-user", user, ""},
		{"secret by ARN in resources[]", t542Event("e-t542-02", "PutSecretValue", "secretsmanager.amazonaws.com",
			[]t542Resource{{secret.Fields["arn"], refAccount, "AWS::SecretsManager::Secret"}}, nil),
			"secrets", secret.ID, ""},
		{"GetSecretValue by ARN", t542Event("e-t542-03", "GetSecretValue", "secretsmanager.amazonaws.com", nil,
			map[string]any{"secretId": secret.Fields["arn"]}),
			"secrets", secret.ID, ""},
		{"GetSecretValue by name", t542Event("e-t542-10", "GetSecretValue", "secretsmanager.amazonaws.com", nil,
			map[string]any{"secretId": secret.ID}),
			"secrets", secret.ID, ""},
		{"S3 object", t542Event("e-t542-04", "DeleteObject", "s3.amazonaws.com",
			[]t542Resource{{"arn:aws:s3:::" + bucket + "/reports/2026/q3.csv", "", "AWS::S3::Object"}}, nil),
			"s3", bucket, ""},
		{"Lambda function", t542Event("e-t542-05", "UpdateFunctionConfiguration20150331v2", "lambda.amazonaws.com",
			[]t542Resource{{"arn:aws:lambda:" + refRegion + ":" + refAccount + ":function:" + fn, refAccount, "AWS::Lambda::Function"}}, nil),
			"lambda", fn, ""},
		{"VPC", t542Event("e-t542-06", "ModifyVpcAttribute", "ec2.amazonaws.com",
			[]t542Resource{{arnEC2 + "vpc/" + vpc, refAccount, "AWS::EC2::VPC"}}, nil),
			"vpc", vpc, "Instance"},
		{"security group", t542Event("e-t542-07", "AuthorizeSecurityGroupIngress", "ec2.amazonaws.com",
			[]t542Resource{{arnEC2 + "security-group/" + sg, refAccount, "AWS::EC2::SecurityGroup"}}, nil),
			"sg", sg, "Instance"},
		{"security group by ARN alone", t542Event("e-t542-08", "AuthorizeSecurityGroupIngress", "ec2.amazonaws.com",
			[]t542Resource{{arnEC2 + "security-group/" + sg, refAccount, ""}}, nil),
			"sg", sg, "Instance"},
		{"subnet", t542Event("e-t542-09", "ModifySubnetAttribute", "ec2.amazonaws.com",
			[]t542Resource{{arnEC2 + "subnet/" + subnet, refAccount, "AWS::EC2::Subnet"}}, nil),
			"subnet", subnet, "Instance"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !b.has(tc.wantType, tc.wantID) {
				t.Fatalf("demo bench has no %s row %q", tc.wantType, tc.wantID)
			}
			rows := t542TargetRows(t, c, tc.event)
			if len(rows) != 1 {
				t.Fatalf("TARGET rows = %+v, want exactly one", rows)
			}
			r := rows[0]
			if !r.IsNavigable || r.TargetType != tc.wantType || navTarget(r) != tc.wantID {
				t.Errorf("TARGET %s %q: navigable=%v type=%q opens %q, want type %q opening %q",
					r.Key, r.Value, r.IsNavigable, r.TargetType, navTarget(r), tc.wantType, tc.wantID)
			}
			if tc.notLabeled != "" && r.Key == tc.notLabeled {
				t.Errorf("TARGET row for a %s is labelled %q", tc.wantType, r.Key)
			}
		})
	}
}

// ─── row 3: values a target can never hold are not its rows ────────────────

const (
	t542SSMImage   = "resolve:ssm:/aws/service/eks/optimized-ami/1.31/amazon-linux-2023/x86_64/standard/recommended/image_id"
	t542StageVarFn = "arn:aws:lambda:us-east-1:123456789012:function:${stageVariables.fn}"
	t542JSONataFn  = "{% $states.input.fn %}"
	t542CustomDom  = "d-7f3k2m9x1q"
	t542GhostFn    = "acme-ghost-fn"
	t542CLB        = "awseb-e-m3k2p9x7q1-AWSEBLoa-1ABCDEFGHIJKL"
)

// t542RequireNotCounted fails when any of values is among r's row ids.
func t542RequireNotCounted(t *testing.T, pivot string, r resource.RelatedCheckResult, values ...string) {
	t.Helper()
	for _, v := range values {
		if slices.Contains(r.ResourceIDs(), v) {
			t.Errorf("%s counts %q as a row: ids %v", pivot, v, r.ResourceIDs())
		}
	}
}

// t542RequireListed fails when r holds an id the target's loaded list does not.
func t542RequireListed(t *testing.T, b refBench, pivot, target string, r resource.RelatedCheckResult) {
	t.Helper()
	for _, id := range r.ResourceIDs() {
		if !b.has(target, id) {
			t.Errorf("%s counts %q, which no %s row carries (ids %v)", pivot, id, target, r.ResourceIDs())
		}
	}
}

// A resolver refuses by shape what AWS guarantees the target never carries:
// an AMI id is ami-…, a Lambda function name has no ${…} or {% … %}, an API
// Gateway API id has no "d-" custom-domain prefix, and vol-ffffffff is the
// volume id AWS writes on a copied snapshot that has no source volume.
func TestResolveRef_RefusesWhatTheTargetCanNeverHold(t *testing.T) {
	b := newRefBench(t)
	for _, tc := range []struct{ target, ref string }{
		{"ami", t542SSMImage},
		{"lambda", t542StageVarFn},
		{"lambda", "${stageVariables.fn}"},
		{"lambda", t542JSONataFn},
		{"apigw", t542CustomDom},
		{"ebs", "vol-ffffffff"},
	} {
		for name, rc := range map[string]domain.RefContext{"list loaded": b.rc(tc.target), "list not loaded": {AccountID: refAccount, Region: refRegion}} {
			if id, ok := resource.ResolveRef(tc.target, tc.ref, rc); ok {
				t.Errorf("ResolveRef(%s, %q) with the %s = (%q, true), want refused", tc.target, tc.ref, name, id)
			}
		}
	}
}

func TestLaunchTemplateSSMImage_IsNotAnAMIRow(t *testing.T) {
	const ltID = "lt-0a1b2c3d4e5f60718"
	versions := []ec2types.LaunchTemplateVersion{{
		LaunchTemplateId:   aws.String(ltID),
		VersionNumber:      aws.Int64(3),
		LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{ImageId: aws.String(t542SSMImage)},
	}}

	asgRow := resource.Resource{ID: "acme-web-asg", Name: "acme-web-asg", Fields: map[string]string{}, RawStruct: asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String("acme-web-asg"),
		LaunchTemplate:       &asgtypes.LaunchTemplateSpecification{LaunchTemplateId: aws.String(ltID), Version: aws.String("$Latest")},
	}}
	asg := asgCheckerByTarget(t, "ami")(context.Background(),
		&awsclient.ServiceClients{EC2: newFakeEC2WithLaunchTemplateVersions(versions), AutoScaling: &fakeASGChecker{}}, asgRow, resource.ResourceCache{})
	t542RequireNotCounted(t, "asg → AMI", asg, t542SSMImage)

	ng := ngCheckerByTarget(t, "ami")(context.Background(),
		&awsclient.ServiceClients{EC2: newFakeEC2WithLaunchTemplateVersions(versions)}, ngSrcResourceWithLaunchTemplate(ltID, "3"), nil)
	t542RequireNotCounted(t, "ng → AMI", ng, t542SSMImage)

	w := eksNodeWorld{
		nodegroups: []*ekstypes.Nodegroup{{
			NodegroupName:  aws.String("general"),
			ClusterName:    aws.String("acme-prod"),
			Status:         ekstypes.NodegroupStatusActive,
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Id: aws.String(ltID), Version: aws.String("3")},
		}},
		ltAMI: map[string]string{ltID: t542SSMImage},
	}
	t542RequireNotCounted(t, "eks → AMI", w.check(t, eksCluster("acme-prod"), "ami"), t542SSMImage)
}

func TestApigwLambda_PlaceholderAndUnlistedFunctionAreNotRows(t *testing.T) {
	b := newRefBench(t)
	fn := b.byType["lambda"][0].ID
	uris := []string{
		refLambdaInvokeURI(t542StageVarFn),
		refLambdaInvokeURI("arn:aws:lambda:us-east-1:123456789012:function:" + t542GhostFn),
		refLambdaInvokeURI("arn:aws:lambda:us-east-1:123456789012:function:" + fn),
	}
	src := b.row(t, "apigw", "abc123def4")

	clients := refClients()
	clients.APIGatewayV2 = &refAPIGWFake{uris: uris}
	got := refChecker(t, "apigw", "lambda")(context.Background(), clients, src, b.cache)
	t542RequireNotCounted(t, "apigw → Lambda", got, "${stageVariables.fn}", t542StageVarFn, t542GhostFn)
	t542RequireListed(t, b, "apigw → Lambda", "lambda", got)
	if !slices.Contains(got.ResourceIDs(), fn) {
		t.Errorf("apigw → Lambda = %v, want the listed function %s among them", got.ResourceIDs(), fn)
	}

	unloaded := t542WithoutList(b.cache, "lambda")
	clients = refClients()
	clients.APIGatewayV2 = &refAPIGWFake{uris: uris}
	clients.Lambda = nil
	if got := refChecker(t, "apigw", "lambda")(context.Background(), clients, src, unloaded); got.EffectiveState() == domain.RelatedResolved {
		t.Errorf("apigw → Lambda with the lambda list not loaded = %v (state resolved), want unknown", got.ResourceIDs())
	}
}

type t542EBFake struct {
	awsclient.ElasticBeanstalkAPI
	lbNames []string
}

func (f *t542EBFake) DescribeEnvironmentResources(_ context.Context, in *elasticbeanstalk.DescribeEnvironmentResourcesInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
	res := &ebtypes.EnvironmentResourceDescription{EnvironmentName: in.EnvironmentName}
	for _, n := range f.lbNames {
		res.LoadBalancers = append(res.LoadBalancers, ebtypes.LoadBalancer{Name: aws.String(n)})
	}
	return &elasticbeanstalk.DescribeEnvironmentResourcesOutput{EnvironmentResources: res}, nil
}

// The elb list holds Application and Network Load Balancers; a Classic Load
// Balancer is no row of it. An environment on a listed ALB counts it, by ARN
// or by name.
func TestEbELB_ClassicLoadBalancerIsNotAnELBRow(t *testing.T) {
	b := newRefBench(t)
	env := b.byType["eb"][0]
	const alb = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"
	if !b.has("elb", "acme-prod-web") {
		t.Fatal("demo bench has no elb row acme-prod-web")
	}

	clients := refClients()
	clients.ElasticBeanstalk = &t542EBFake{lbNames: []string{t542CLB}}
	clb := refChecker(t, "eb", "elb")(context.Background(), clients, env, b.cache)
	t542RequireNotCounted(t, "eb → ELB (Classic)", clb, t542CLB)
	t542RequireListed(t, b, "eb → ELB (Classic)", "elb", clb)

	for _, named := range []string{alb, "acme-prod-web"} {
		clients.ElasticBeanstalk = &t542EBFake{lbNames: []string{named}}
		got := refChecker(t, "eb", "elb")(context.Background(), clients, env, b.cache)
		if ids := sortedIDs(got); !slices.Equal(ids, []string{"acme-prod-web"}) {
			t.Errorf("eb → ELB on the ALB named %q = %v, want [acme-prod-web]", named, ids)
		}
	}
}

// A Task state's FunctionName may carry an alias, a JSONata expression or a
// partial ARN; none of those strings is a function's row id, and a function
// name the loaded list does not carry is no row either.
func TestSfnLambda_FunctionNameValuesCountOnlyListedFunctions(t *testing.T) {
	b := newRefBench(t)
	fn := b.byType["lambda"][0].ID
	sm := b.byType["sfn"][0]
	values := []string{fn + ":prod", t542JSONataFn, refAccount + ":function:" + fn, t542GhostFn}
	states := map[string]any{}
	for i, v := range values {
		next := map[string]any{"End": true}
		if i < len(values)-1 {
			next = map[string]any{"Next": "Step" + string(rune('B'+i))}
		}
		st := map[string]any{"Type": "Task", "Resource": "arn:aws:states:::lambda:invoke", "Arguments": map[string]any{"FunctionName": v, "Payload": "{% $states.input %}"}}
		if i == 0 {
			st["Parameters"] = map[string]any{"FunctionName": v, "Payload.$": "$"}
			delete(st, "Arguments")
		}
		maps.Copy(st, next)
		states["Step"+string(rune('A'+i))] = st
	}
	def, err := json.Marshal(map[string]any{"QueryLanguage": "JSONata", "StartAt": "StepA", "States": states})
	if err != nil {
		t.Fatal(err)
	}

	clients := refClients()
	clients.SFN = &fakeSFNExtra{output: &sfnsvc.DescribeStateMachineOutput{
		StateMachineArn: aws.String(sm.Fields["arn"]),
		Name:            aws.String(sm.Name),
		Definition:      aws.String(string(def)),
	}}
	got := refChecker(t, "sfn", "lambda")(context.Background(), clients, sm, b.cache)
	t542RequireNotCounted(t, "sfn → Lambda", got, values...)
	t542RequireListed(t, b, "sfn → Lambda", "lambda", got)

	clients.Lambda = nil
	if got := refChecker(t, "sfn", "lambda")(context.Background(), clients, sm, t542WithoutList(b.cache, "lambda")); got.EffectiveState() == domain.RelatedResolved {
		t.Errorf("sfn → Lambda with the lambda list not loaded = %v (state resolved), want unknown", got.ResourceIDs())
	}
}

// A Route 53 alias to an execute-api custom domain names the domain, not an
// API; it is no row of the API list.
func TestR53APIGW_CustomDomainAliasIsNotAnAPIRow(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	clients.Route53 = &refR53Fake{aliases: []string{t542CustomDom + ".execute-api.us-east-1.amazonaws.com"}}
	got := refChecker(t, "r53", "apigw")(context.Background(), clients, b.byType["r53"][0], b.cache)
	t542RequireNotCounted(t, "r53 → API Gateway", got, t542CustomDom)
	t542RequireListed(t, b, "r53 → API Gateway", "apigw", got)
}

func t542WithoutList(cache resource.ResourceCache, typ string) resource.ResourceCache {
	out := maps.Clone(cache)
	delete(out, typ)
	return out
}

// ─── row 4: a KMS alias resolves the same way on Enter and on the panel ────

// t542WithKMSField returns a copy of the demo row with its KMS field set to
// ref, in the shape its fetcher stores the AWS struct.
func t542WithKMSField(t *testing.T, r resource.Resource, ref string) resource.Resource {
	t.Helper()
	switch v := r.RawStruct.(type) {
	case ssmtypes.ParameterMetadata:
		v.KeyId = aws.String(ref)
		r.RawStruct = v
	case smtypes.SecretListEntry:
		v.KmsKeyId = aws.String(ref)
		r.RawStruct = v
	case cloudtrailtypes.Trail:
		v.KmsKeyId = aws.String(ref)
		r.RawStruct = v
	case kafkatypes.Cluster:
		if v.Provisioned == nil || v.Provisioned.EncryptionInfo == nil || v.Provisioned.EncryptionInfo.EncryptionAtRest == nil {
			t.Fatalf("msk row %s has no encryption-at-rest block", r.ID)
		}
		p := *v.Provisioned
		enc := *p.EncryptionInfo
		rest := *enc.EncryptionAtRest
		rest.DataVolumeKMSKeyId = aws.String(ref)
		enc.EncryptionAtRest = &rest
		p.EncryptionInfo = &enc
		v.Provisioned = &p
		r.RawStruct = v
	default:
		t.Fatalf("row %s RawStruct %T is not a modelled KMS source", r.ID, r.RawStruct)
	}
	r.Fields = maps.Clone(r.Fields)
	return r
}

// With the kms list not loaded, a navigable KMS field holding a customer
// alias is a link whose target is the alias. Enter opens it through the kms
// by-id read (DescribeKey accepts an alias), which lands on the key's own row
// — the row the related panel counts for the same value. A field holding an
// AWS-managed alias, a key no kms row lists, is not a link.
func TestKMSAliasField_EnterAndPanelResolveTheSameKey(t *testing.T) {
	b := newRefBench(t)
	const alias = "alias/acme-prod-key"
	aliasARN := "arn:aws:kms:" + refRegion + ":" + refAccount + ":" + alias
	wantKey := kmsKeyByAlias(t, b, alias)
	unloaded := t542WithoutList(b.cache, "kms")
	kmsTD := resource.FindResourceType("kms")
	if kmsTD == nil || kmsTD.FetchByIDs == nil {
		t.Fatal("kms has no FetchByIDs to open a key by alias")
	}

	cases := []struct {
		typ, path string
		src       resource.Resource
	}{
		{"ssm", "KeyId", b.row(t, "ssm", "/acme/prod/db/connection-string")},
		{"secrets", "KmsKeyId", t542WithKMSField(t, b.row(t, "secrets", "prod/database/primary"), alias)},
		{"trail", "KmsKeyId", t542WithKMSField(t, b.byType["trail"][0], aliasARN)},
		{"msk", "Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId", t542WithKMSField(t, b.byType["msk"][0], aliasARN)},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			c := refDetailController(t, b, "kms")
			f := fieldAt(t, openDetail(c, tc.typ, tc.src), tc.path)
			if !f.IsNavigable || f.TargetType != "kms" || (navTarget(f) != alias && navTarget(f) != aliasARN) {
				t.Errorf("%s %s %q with the kms list not loaded: navigable=%v type=%q target %q, want a kms link to the alias",
					tc.typ, tc.path, f.Value, f.IsNavigable, f.TargetType, navTarget(f))
				return
			}
			opened, err := kmsTD.FetchByIDs(context.Background(), refClients(), []string{navTarget(f)})
			if err != nil || len(opened) != 1 || opened[0].ID != wantKey {
				t.Errorf("Enter on %s %s (%q) opens %v (err %v), want the key's own row %s",
					tc.typ, tc.path, navTarget(f), resourceIDsOf(opened), err, wantKey)
			}
			panel := refChecker(t, tc.typ, "kms")(context.Background(), refClients(), tc.src, unloaded)
			if ids := sortedIDs(panel); !slices.Equal(ids, []string{wantKey}) {
				t.Errorf("%s → KMS Key with the kms list not loaded = %v (state %v), want [%s]", tc.typ, ids, panel.EffectiveState(), wantKey)
			}
		})
	}

	t.Run("AWS-managed alias", func(t *testing.T) {
		c := refDetailController(t, b, "kms")
		f := fieldAt(t, openDetail(c, "ssm", b.row(t, "ssm", "/acme/legacy/db/password")), "KeyId")
		if f.IsNavigable {
			t.Errorf("KeyId %q (an AWS-managed key no kms row lists) is a link to kms %q", f.Value, navTarget(f))
		}
	})
}

// ─── row 5: a blackholed route target is not a link ────────────────────────

func TestRouteTable_BlackholedTargetIsNotNavigable(t *testing.T) {
	b := newRefBench(t)
	src := b.byType["rtb"][0]
	nat := b.byType["nat"][0].ID
	table, ok := src.RawStruct.(ec2types.RouteTable)
	if !ok {
		t.Fatalf("rtb RawStruct is %T", src.RawStruct)
	}
	blackholed := map[string]ec2types.Route{
		"nat-0dead1111aaaa2222b": {DestinationCidrBlock: aws.String("10.40.0.0/16"), NatGatewayId: aws.String("nat-0dead1111aaaa2222b"), State: ec2types.RouteStateBlackhole, Origin: ec2types.RouteOriginCreateRoute},
		"eni-0dead1111aaaa2222b": {DestinationCidrBlock: aws.String("10.41.0.0/16"), NetworkInterfaceId: aws.String("eni-0dead1111aaaa2222b"), State: ec2types.RouteStateBlackhole, Origin: ec2types.RouteOriginCreateRoute},
		"tgw-0dead1111aaaa2222b": {DestinationCidrBlock: aws.String("10.42.0.0/16"), TransitGatewayId: aws.String("tgw-0dead1111aaaa2222b"), State: ec2types.RouteStateBlackhole, Origin: ec2types.RouteOriginCreateRoute},
	}
	table.Routes = slices.Clone(table.Routes)
	for _, r := range blackholed {
		table.Routes = append(table.Routes, r)
	}
	table.Routes = append(table.Routes, ec2types.Route{DestinationCidrBlock: aws.String("10.43.0.0/16"), NatGatewayId: aws.String(nat), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute})
	src.RawStruct = table

	fields := openDetail(refDetailController(t, b), "rtb", src)
	seen := map[string]bool{}
	for _, f := range fields {
		v := strings.TrimPrefix(strings.TrimSpace(f.Value), "- ")
		if _, dead := blackholed[v]; dead {
			seen[v] = true
			if f.IsNavigable {
				t.Errorf("blackholed route target %s (%s) is a link to %s", v, f.Path, f.TargetType)
			}
		}
		if v == nat && !f.IsNavigable {
			t.Errorf("active route target %s (%s) is not a link", v, f.Path)
		}
	}
	for id := range blackholed {
		if !seen[id] {
			t.Errorf("the route table detail shows no row for blackholed target %s", id)
		}
	}
}

// ─── row 6: CloudTrail lookups name the resource the way AWS records it ────

// t542CTWorld answers LookupEvents by an exact ResourceName match against each
// event's resources, as CloudTrail does.
type t542CTWorld struct {
	events  []cloudtrailtypes.Event
	lookups []string
}

func (w *t542CTWorld) LookupEvents(_ context.Context, in *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	name := ""
	for _, a := range in.LookupAttributes {
		if a.AttributeKey == cloudtrailtypes.LookupAttributeKeyResourceName {
			name = aws.ToString(a.AttributeValue)
		}
	}
	w.lookups = append(w.lookups, name)
	out := &cloudtrail.LookupEventsOutput{}
	for _, e := range w.events {
		if slices.ContainsFunc(e.Resources, func(r cloudtrailtypes.Resource) bool { return aws.ToString(r.ResourceName) == name }) {
			out.Events = append(out.Events, e)
		}
	}
	return out, nil
}

// KMS records most calls against a key under the key's ARN. The pivot's
// lookup reaches an event that names the key only by ARN.
func TestKMSCloudTrail_FindsEventsNamingTheKeyByARN(t *testing.T) {
	b := newRefBench(t)
	key := b.byType["kms"][0]
	keyARN := "arn:aws:kms:" + refRegion + ":" + refAccount + ":key/" + key.ID
	world := &t542CTWorld{events: []cloudtrailtypes.Event{
		t542Event("e-t542-kms1", "Decrypt", "kms.amazonaws.com", []t542Resource{{keyARN, refAccount, "AWS::KMS::Key"}}, map[string]any{"encryptionAlgorithm": "SYMMETRIC_DEFAULT"}),
	}}

	r := refChecker(t, "kms", "ct-events")(context.Background(), refClients(), key, b.cache)
	filter := r.FetchFilter()
	if filter == nil {
		t.Fatalf("kms → CloudTrail Events offers no lookup (state %v)", r.EffectiveState())
	}
	page, err := awsclient.FetchCloudTrailEventsPageFiltered(context.Background(), world, filter, "")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(page.Resources) != 1 {
		t.Errorf("kms → CloudTrail Events found %d events (lookups %q), want the Decrypt that names %s", len(page.Resources), world.lookups, keyARN)
	}
}

// A subscription that has no ARN yet (pending confirmation, deleted) has no
// name CloudTrail records it under: the row offers no lookup. A confirmed
// subscription looks up its ARN.
func TestSNSSubCloudTrail_NoLookupWithoutASubscriptionARN(t *testing.T) {
	b := newRefBench(t)
	checker := refChecker(t, "sns-sub", "ct-events")
	var pending, confirmed int
	for _, sub := range b.byType["sns-sub"] {
		r := checker(context.Background(), refClients(), sub, b.cache)
		if strings.HasPrefix(sub.ID, "arn:") {
			confirmed++
			if f := r.FetchFilter(); f["ResourceName"] != sub.ID {
				t.Errorf("confirmed subscription %s looks up %v, want its ARN", sub.ID, f)
			}
			continue
		}
		pending++
		if r.FetchFilter() != nil || r.EffectiveState() == domain.RelatedDeferred {
			t.Errorf("subscription %s without an ARN offers a lookup %v (state %v)", sub.ID, r.FetchFilter(), r.EffectiveState())
		}
	}
	if pending == 0 || confirmed == 0 {
		t.Fatalf("demo bench has %d subscriptions without an ARN and %d with one; both are needed", pending, confirmed)
	}
}

// ─── row 7: exact-id drills ────────────────────────────────────────────────

// t542EC2ByID answers DescribeNetworkInterfaces and DescribeSnapshots by id
// as EC2 does: an id that does not exist fails the whole call with a NotFound
// error naming it.
type t542EC2ByID struct {
	awsclient.EC2API
	enis  []ec2types.NetworkInterface
	snaps []ec2types.Snapshot
}

func (f *t542EC2ByID) DescribeNetworkInterfaces(_ context.Context, in *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	var out []ec2types.NetworkInterface
	for _, id := range in.NetworkInterfaceIds {
		i := slices.IndexFunc(f.enis, func(n ec2types.NetworkInterface) bool { return aws.ToString(n.NetworkInterfaceId) == id })
		if i < 0 {
			return nil, &smithy.GenericAPIError{Code: "InvalidNetworkInterfaceID.NotFound", Message: "The networkInterface ID '" + id + "' does not exist"}
		}
		out = append(out, f.enis[i])
	}
	if len(in.NetworkInterfaceIds) == 0 {
		out = f.enis
	}
	return &ec2.DescribeNetworkInterfacesOutput{NetworkInterfaces: out}, nil
}

func (f *t542EC2ByID) DescribeSnapshots(_ context.Context, in *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	var out []ec2types.Snapshot
	for _, id := range in.SnapshotIds {
		i := slices.IndexFunc(f.snaps, func(s ec2types.Snapshot) bool { return aws.ToString(s.SnapshotId) == id })
		if i < 0 {
			return nil, &smithy.GenericAPIError{Code: "InvalidSnapshot.NotFound", Message: "The snapshot '" + id + "' does not exist."}
		}
		out = append(out, f.snaps[i])
	}
	if len(in.SnapshotIds) == 0 {
		out = f.snaps
	}
	return &ec2.DescribeSnapshotsOutput{Snapshots: out}, nil
}

func TestExactIDDrill_ENIPastTheFirstPageOpens(t *testing.T) {
	td := resource.FindResourceType("eni")
	if td == nil || td.FetchByIDs == nil {
		t.Fatal("eni has no FetchByIDs: an exact-id drill falls back to the first list page")
	}
	const want = "eni-0f9e8d7c6b5a40312"
	fake := &t542EC2ByID{}
	for i := range 120 {
		fake.enis = append(fake.enis, ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(fmt.Sprintf("eni-0a1b2c3d4e5f6%04x", i)),
			SubnetId:           aws.String("subnet-0f10a1b2c3d4e5f60"),
			VpcId:              aws.String("vpc-0abc123def456789a"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			PrivateIpAddress:   aws.String(fmt.Sprintf("10.0.%d.%d", i/200, 10+i%200)),
		})
	}
	fake.enis = append(fake.enis, ec2types.NetworkInterface{
		NetworkInterfaceId: aws.String(want), SubnetId: aws.String("subnet-0f10a1b2c3d4e5f60"), VpcId: aws.String("vpc-0abc123def456789a"),
		Status: ec2types.NetworkInterfaceStatusAvailable, InterfaceType: ec2types.NetworkInterfaceTypeInterface, PrivateIpAddress: aws.String("10.0.9.9"),
	})
	rows, err := td.FetchByIDs(context.Background(), &awsclient.ServiceClients{EC2: fake}, []string{want})
	if err != nil {
		t.Fatalf("FetchByIDs: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != want {
		t.Errorf("FetchByIDs(%s) = %d rows %v, want that ENI", want, len(rows), resourceIDsOf(rows))
	}
}

// DescribeSnapshots fails the whole call when one id no longer exists. The
// drill shows the snapshots that exist and reports the missing one.
func TestExactIDDrill_EBSSnapshotsShowTheExistingAndReportTheMissing(t *testing.T) {
	td := resource.FindResourceType("ebs-snap")
	if td == nil || td.FetchByIDs == nil {
		t.Fatal("ebs-snap has no FetchByIDs")
	}
	const gone = "snap-0dead1111aaaa2222b"
	fake := &t542EC2ByID{snaps: []ec2types.Snapshot{
		{SnapshotId: aws.String("snap-0a1b2c3d4e5f60001"), VolumeId: aws.String("vol-0a1b2c3d4e5f60001"), State: ec2types.SnapshotStateCompleted, VolumeSize: aws.Int32(100), OwnerId: aws.String(refAccount)},
		{SnapshotId: aws.String("snap-0a1b2c3d4e5f60002"), VolumeId: aws.String("vol-0a1b2c3d4e5f60002"), State: ec2types.SnapshotStateCompleted, VolumeSize: aws.Int32(50), OwnerId: aws.String(refAccount)},
	}}
	ids := []string{"snap-0a1b2c3d4e5f60001", gone, "snap-0a1b2c3d4e5f60002"}
	rows, err := td.FetchByIDs(context.Background(), &awsclient.ServiceClients{EC2: fake}, ids)
	if got := resourceIDsOf(rows); !slices.Equal(got, []string{"snap-0a1b2c3d4e5f60001", "snap-0a1b2c3d4e5f60002"}) {
		t.Errorf("FetchByIDs(%v) rows = %v, want the two snapshots that exist", ids, got)
	}
	if err == nil || !strings.Contains(err.Error(), gone) {
		t.Errorf("FetchByIDs(%v) error = %v, want one naming the missing %s", ids, err, gone)
	}
}

func resourceIDsOf(rs []resource.Resource) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.ID)
	}
	slices.Sort(out)
	return out
}
