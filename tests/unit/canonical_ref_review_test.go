package unit_test

// canonical_ref_review_test.go — the reference readings the area review and
// acceptance found still done by hand, or still reading a name as an ID: each
// test drives the registered checker, the fetcher or the rendered detail with
// the shape AWS returns and asserts the row ID the target list keys on.

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

const (
	refKeyA1ARN      = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"
	refKeyA1         = "a1b2c3d4-5678-90ab-cdef-111111111111"
	refPrimarySecret = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"
)

func refRelatedDef(t *testing.T, source, target string) resource.RelatedDef {
	t.Helper()
	for _, def := range resource.GetRelated(source) {
		if def.TargetType == target {
			return def
		}
	}
	t.Fatalf("no %s related def for target %q", source, target)
	return resource.RelatedDef{}
}

// ── A: every checker reads its references through the target's resolver ───

// TestTGWRole_ServiceLinkedRoleCountedByName: role rows are keyed by role
// name, so the transit gateway's service-linked role is counted by name, not
// by the ARN GetRole returns.
func TestTGWRole_ServiceLinkedRoleCountedByName(t *testing.T) {
	b := newRefBench(t)
	tgws := b.byType["tgw"]
	if len(tgws) == 0 {
		t.Fatal("demo bench has no tgw rows")
	}
	got := refChecker(t, "tgw", "role")(context.Background(), refClients(), tgws[0], b.cache)
	if ids := sortedIDs(got); !slices.Equal(ids, []string{"AWSServiceRoleForVPCTransitGateway"}) {
		t.Errorf("tgw %s → IAM Role IDs = %v, want [AWSServiceRoleForVPCTransitGateway]", tgws[0].ID, ids)
	}
}

// TestCbSecrets_BothDocumentedFormsCountTheSecretName: a SECRETS_MANAGER
// environment variable holds either the secret ARN or the bare
// "secret-id:json-key[:version-stage:version-id]" form; both name the secret
// row keyed by its name. Another account's secret is not a local row.
func TestCbSecrets_BothDocumentedFormsCountTheSecretName(t *testing.T) {
	b := newRefBench(t)
	env := func(values ...string) cbtypes.Project {
		p := cbtypes.Project{Name: aws.String("acme-api-build"), Environment: &cbtypes.ProjectEnvironment{}}
		for i, v := range values {
			p.Environment.EnvironmentVariables = append(p.Environment.EnvironmentVariables, cbtypes.EnvironmentVariable{
				Name:  aws.String("SECRET_" + string(rune('A'+i))),
				Type:  cbtypes.EnvironmentVariableTypeSecretsManager,
				Value: aws.String(v),
			})
		}
		return p
	}
	cases := []struct {
		name          string
		values        []string
		want          []string
		wantTruncated bool
	}{
		{"ARN with json-key", []string{refPrimarySecret + ":password"}, []string{"prod/database/primary"}, false},
		{"bare name with json-key", []string{"prod/api/gateway-key:token"}, []string{"prod/api/gateway-key"}, false},
		{"bare name with json-key, stage and version", []string{"prod/api/gateway-key:token:AWSCURRENT:"}, []string{"prod/api/gateway-key"}, false},
		{"another account's secret", []string{"arn:aws:secretsmanager:us-east-1:" + refForeignAccount + ":secret:prod/database/primary-AbCdEf:password"}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := resource.Resource{ID: "acme-api-build", Name: "acme-api-build", Type: "cb", RawStruct: env(tc.values...)}
			got := refChecker(t, "cb", "secrets")(context.Background(), refClients(), res, b.cache)
			if ids := sortedIDs(got); !slices.Equal(ids, tc.want) || got.Truncated() != tc.wantTruncated {
				t.Errorf("cb → Secrets = %v truncated=%v, want %v truncated=%v", ids, got.Truncated(), tc.want, tc.wantTruncated)
			}
		})
	}
	if id, ok := resource.ResolveRef("secrets", "prod/api/gateway-key:token", b.rc("secrets")); !ok || id != "prod/api/gateway-key" {
		t.Errorf(`ResolveRef(secrets, "prod/api/gateway-key:token") = (%q, %v), want ("prod/api/gateway-key", true)`, id, ok)
	}
}

type refAPIGWFake struct {
	awsclient.APIGatewayV2API
	uris []string
}

func (f *refAPIGWFake) GetIntegrations(context.Context, *apigatewayv2.GetIntegrationsInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	out := &apigatewayv2.GetIntegrationsOutput{}
	for i, u := range f.uris {
		out.Items = append(out.Items, apigwv2types.Integration{
			IntegrationId:   aws.String("int" + string(rune('a'+i))),
			IntegrationType: apigwv2types.IntegrationTypeAwsProxy,
			IntegrationUri:  aws.String(u),
		})
	}
	return out, nil
}

func refLambdaInvokeURI(fnARN string) string {
	return "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/" + fnARN + "/invocations"
}

// TestApigwLambda_IntegrationFunctionReadThroughTheLambdaResolver: an
// integration names its function by ARN inside the invoke URI. A function in
// another account is not the local function of the same name, and a
// qualified ARN names the function itself.
func TestApigwLambda_IntegrationFunctionReadThroughTheLambdaResolver(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	clients.APIGatewayV2 = &refAPIGWFake{uris: []string{
		refLambdaInvokeURI("arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer"),
		refLambdaInvokeURI("arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-transform:live"),
		refLambdaInvokeURI("arn:aws:lambda:us-east-1:" + refForeignAccount + ":function:process-orders"),
	}}
	got := refChecker(t, "apigw", "lambda")(context.Background(), clients, b.row(t, "apigw", "abc123def4"), b.cache)
	want := []string{"api-gateway-authorizer", "data-pipeline-transform"}
	if ids := sortedIDs(got); !slices.Equal(ids, want) || !got.Truncated() {
		t.Errorf("apigw → Lambda = %v truncated=%v, want %v truncated=true (another account's function left out)", ids, got.Truncated(), want)
	}
}

func refSecretEntry(t *testing.T, r resource.Resource) smtypes.SecretListEntry {
	t.Helper()
	switch v := r.RawStruct.(type) {
	case smtypes.SecretListEntry:
		return v
	case *smtypes.SecretListEntry:
		return *v
	}
	t.Fatalf("secret %s RawStruct is %T", r.ID, r.RawStruct)
	return smtypes.SecretListEntry{}
}

// TestSecretsRotationLambda_QualifiedARNNamesTheFunction: a rotation function
// ARN may carry a version or alias qualifier; the function is the name
// before it, for the Lambda row and for the log group Lambda writes to.
func TestSecretsRotationLambda_QualifiedARNNamesTheFunction(t *testing.T) {
	b := newRefBench(t)
	const fn = "rotate-docdb-credentials"
	if !b.has("lambda", fn) || !b.has("logs", "/aws/lambda/"+fn) {
		t.Fatalf("demo bench lacks lambda %s or its log group", fn)
	}
	secret := b.row(t, "secrets", "prod/docdb/acme-docdb-prod")
	entry := refSecretEntry(t, secret)
	entry.RotationLambdaARN = aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + fn + ":live")
	secret.RawStruct = entry

	if ids := sortedIDs(refChecker(t, "secrets", "lambda")(context.Background(), refClients(), secret, b.cache)); !slices.Equal(ids, []string{fn}) {
		t.Errorf("qualified rotation ARN → Lambda IDs = %v, want [%s]", ids, fn)
	}
	logs := sortedIDs(refChecker(t, "secrets", "logs")(context.Background(), refClients(), secret, b.cache))
	if !slices.Contains(logs, "/aws/lambda/"+fn) || slices.Contains(logs, "/aws/lambda/live") {
		t.Errorf("qualified rotation ARN → Log Group IDs = %v, want /aws/lambda/%s", logs, fn)
	}

	entry.RotationLambdaARN = aws.String("arn:aws:lambda:us-east-1:" + refForeignAccount + ":function:" + fn)
	secret.RawStruct = entry
	if got := refChecker(t, "secrets", "lambda")(context.Background(), refClients(), secret, b.cache); got.Count() != 0 {
		t.Errorf("another account's rotation function → Lambda IDs = %v, want none", got.ResourceIDs())
	}
}

// TestASGELB_ClassicNamesAreNotLoadBalancerRows: the elb list holds
// application and network load balancers only, so a Classic name an ASG
// carries is never counted, even when an ALB happens to share it.
func TestASGELB_ClassicNamesAreNotLoadBalancerRows(t *testing.T) {
	b := newRefBench(t)
	asg := b.row(t, "asg", "acme-web-prod-asg")
	asg.RawStruct = asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String(asg.ID),
		LoadBalancerNames:    []string{"acme-internal-api"},
	}
	got := refChecker(t, "asg", "elb")(context.Background(), refClients(), asg, b.cache)
	if got.Count() != 0 {
		t.Errorf("Classic LoadBalancerNames [acme-internal-api] → Load Balancers = %v, want none", got.ResourceIDs())
	}
}

// ── B: alarm dimensions that hold a name ──────────────────────────────────

func refNamedRow(t *testing.T, b refBench, typ string) resource.Resource {
	t.Helper()
	for _, r := range b.byType[typ] {
		if r.Name != "" && r.Name != r.ID {
			return r
		}
	}
	t.Fatalf("demo %s has no row whose name differs from its ID", typ)
	return resource.Resource{}
}

// TestAlarmNamedDimensions_ResolveToTheRowID: an API Gateway alarm names the
// API by ApiName and a WAF alarm the web ACL by WebACL; both lists key on the
// ID. Each pivot needs its target list loaded to read the name, and a name no
// loaded row carries names nothing.
func TestAlarmNamedDimensions_ResolveToTheRowID(t *testing.T) {
	b := newRefBench(t)
	cases := []struct{ target, namespace, dim string }{
		{"apigw", "AWS/ApiGateway", "ApiName"},
		{"waf", "AWS/WAFV2", "WebACL"},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			if def := refRelatedDef(t, "alarm", tc.target); !def.NeedsTargetCache {
				t.Errorf("alarm → %s NeedsTargetCache = false, want true: a dimension name reads only against the loaded list", tc.target)
			}
			row := refNamedRow(t, b, tc.target)
			alarm := func(v string) resource.Resource {
				return resource.Resource{ID: "acme-" + tc.target + "-alarm", Type: "alarm", RawStruct: cwtypes.MetricAlarm{
					AlarmName:  aws.String("acme-" + tc.target + "-alarm"),
					Namespace:  aws.String(tc.namespace),
					Dimensions: []cwtypes.Dimension{{Name: aws.String(tc.dim), Value: aws.String(v)}},
				}}
			}
			check := refChecker(t, "alarm", tc.target)
			if ids := sortedIDs(check(context.Background(), refClients(), alarm(row.Name), b.cache)); !slices.Equal(ids, []string{row.ID}) {
				t.Errorf("%s=%q → IDs = %v, want [%s]", tc.dim, row.Name, ids, row.ID)
			}
			if got := check(context.Background(), refClients(), alarm("acme-no-such-"+tc.target), b.cache); got.Count() != 0 {
				t.Errorf("%s naming no loaded row → IDs = %v, want none", tc.dim, got.ResourceIDs())
			}
			if id, ok := resource.ResolveRef(tc.target, row.Name, b.rc(tc.target)); !ok || id != row.ID {
				t.Errorf("ResolveRef(%s, %q) = (%q, %v), want (%q, true)", tc.target, row.Name, id, ok, row.ID)
			}
			if _, ok := resource.ResolveRef(tc.target, "acme-no-such-"+tc.target, b.rc(tc.target)); ok {
				t.Errorf("ResolveRef(%s, a name no loaded row carries) ok = true, want false", tc.target)
			}
		})
	}
}

// ── C: an ECS task's host is an EC2 instance ID ───────────────────────────

// TestEcsTaskEC2_HostIsAnInstanceID: a task on the EC2 launch type runs on a
// container instance whose Ec2InstanceId is the ec2 row; the container
// instance's own UUID is no instance ID and no ec2 row can carry it.
func TestEcsTaskEC2_HostIsAnInstanceID(t *testing.T) {
	b := newRefBench(t)
	for _, r := range b.byType["ec2"] {
		if !strings.HasPrefix(r.ID, "i-") {
			t.Errorf("demo ec2 row %q is not an instance ID", r.ID)
		}
	}
	check := refChecker(t, "ecs-task", "ec2")
	witness := false
	for _, task := range b.byType["ecs-task"] {
		ids := check(context.Background(), refClients(), task, b.cache).ResourceIDs()
		for _, id := range ids {
			if !strings.HasPrefix(id, "i-") || !b.has("ec2", id) {
				t.Errorf("ecs-task %s → EC2 Instances ID %q is not an ec2 instance row", task.ID, id)
			}
		}
		if task.ID == "d4e5f6a1b2c3d4e5f6010203" && len(ids) == 1 {
			witness = true
		}
	}
	if !witness {
		t.Error("ecs-task d4e5f6a1b2c3d4e5f6010203 (EC2 launch type) counts no single host instance")
	}
}

// ── D: CloudTrail principals from another account ─────────────────────────

type refCloudTrailFake struct {
	awsclient.CloudTrailAPI
	events []cloudtrailtypes.Event
}

func (f *refCloudTrailFake) LookupEvents(context.Context, *cloudtrail.LookupEventsInput, ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	return &cloudtrail.LookupEventsOutput{Events: f.events}, nil
}

func refCTEvent(id, name, username string, body map[string]any) cloudtrailtypes.Event {
	raw, _ := json.Marshal(body) //nolint:errchkjson // a map of strings and maps always marshals
	return cloudtrailtypes.Event{
		EventId:         aws.String(id),
		EventName:       aws.String(name),
		EventSource:     aws.String("sts.amazonaws.com"),
		Username:        aws.String(username),
		CloudTrailEvent: aws.String(string(raw)),
	}
}

// refCTRows fetches events through the registered ct-events fetcher, with the
// session's account resolved, the way the list loads them.
func refCTRows(t *testing.T, events ...cloudtrailtypes.Event) map[string]resource.Resource {
	t.Helper()
	clients := refClients()
	clients.CloudTrail = &refCloudTrailFake{events: events}
	res, err := resource.GetPaginatedFetcher("ct-events")(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("ct-events fetch: %v", err)
	}
	out := map[string]resource.Resource{}
	for _, r := range res.Resources {
		out[r.ID] = r
	}
	return out
}

// TestCtEventsPrincipals_AnotherAccountIsNotTheLocalNamesake: an event names
// a principal in another account. The IAM Roles pivot does not count the
// local role that shares its name, and the role_name and user fields that
// open a local role or user stay empty for a foreign principal.
func TestCtEventsPrincipals_AnotherAccountIsNotTheLocalNamesake(t *testing.T) {
	b := newRefBench(t)
	const role = "a9s-demo-s3-access-role"
	const user = "alice.johnson"
	assume := func(id, account string) cloudtrailtypes.Event {
		return refCTEvent(id, "AssumeRole", user, map[string]any{
			"eventName":          "AssumeRole",
			"recipientAccountId": refAccount,
			"userIdentity":       map[string]any{"type": "IAMUser", "accountId": refAccount, "userName": user},
			"requestParameters":  map[string]any{"roleArn": "arn:aws:iam::" + account + ":role/" + role, "roleSessionName": "ops"},
		})
	}
	bySession := func(id, account string) cloudtrailtypes.Event {
		return refCTEvent(id, "GetObject", role+"/ops", map[string]any{
			"eventName":          "GetObject",
			"recipientAccountId": refAccount,
			"userIdentity": map[string]any{
				"type": "AssumedRole", "accountId": account,
				"arn": "arn:aws:sts::" + account + ":assumed-role/" + role + "/ops",
				"sessionContext": map[string]any{"sessionIssuer": map[string]any{
					"type": "Role", "accountId": account, "userName": role,
					"arn": "arn:aws:iam::" + account + ":role/" + role,
				}},
			},
		})
	}
	byUser := func(id, account string) cloudtrailtypes.Event {
		return refCTEvent(id, "ListBuckets", user, map[string]any{
			"eventName":          "ListBuckets",
			"recipientAccountId": refAccount,
			"userIdentity":       map[string]any{"type": "IAMUser", "accountId": account, "userName": user},
		})
	}
	rows := refCTRows(t,
		assume("evt-assume-local", refAccount), assume("evt-assume-foreign", refForeignAccount),
		bySession("evt-session-local", refAccount), bySession("evt-session-foreign", refForeignAccount),
		byUser("evt-user-local", refAccount), byUser("evt-user-foreign", refForeignAccount),
	)
	roles := refChecker(t, "ct-events", "role")
	if ids := sortedIDs(roles(context.Background(), refClients(), rows["evt-assume-local"], b.cache)); !slices.Equal(ids, []string{role}) {
		t.Errorf("AssumeRole of the local role → IAM Roles = %v, want [%s]", ids, role)
	}
	if got := roles(context.Background(), refClients(), rows["evt-assume-foreign"], b.cache); got.Count() != 0 {
		t.Errorf("AssumeRole of another account's role → IAM Roles = %v, want none", got.ResourceIDs())
	}

	fields := []struct{ id, key, want string }{
		{"evt-session-local", "role_name", role},
		{"evt-session-foreign", "role_name", ""},
		{"evt-user-local", "user", user},
		{"evt-user-foreign", "user", ""},
	}
	for _, f := range fields {
		if got := rows[f.id].Fields[f.key]; got != f.want {
			t.Errorf("%s Fields[%q] = %q, want %q", f.id, f.key, got, f.want)
		}
	}
}

// ── E: a KMS lookup that partly fails keeps what it resolved ──────────────

// refKMSLookupFake answers DescribeKey for the listed key IDs and aliases,
// NotFound for a retired alias, and a service failure for a flaky one.
type refKMSLookupFake struct {
	awsclient.KMSAPI
	keys           map[string]kmstypes.KeyMetadata
	notFound       []string
	failing        []string
	listAliasesErr error
}

func (f *refKMSLookupFake) ListAliases(context.Context, *kms.ListAliasesInput, ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	if f.listAliasesErr != nil {
		return nil, f.listAliasesErr
	}
	out := &kms.ListAliasesOutput{}
	for name, meta := range f.keys {
		if strings.HasPrefix(name, "alias/") {
			out.Aliases = append(out.Aliases, kmstypes.AliasListEntry{AliasName: aws.String(name), TargetKeyId: meta.KeyId})
		}
	}
	return out, nil
}

func (f *refKMSLookupFake) DescribeKey(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	id := aws.ToString(in.KeyId)
	if i := strings.Index(id, ":alias/"); i >= 0 {
		id = id[i+1:]
	}
	if slices.Contains(f.notFound, id) {
		return nil, &kmstypes.NotFoundException{Message: aws.String("Alias " + id + " is not found.")}
	}
	if slices.Contains(f.failing, id) {
		return nil, &kmstypes.KMSInternalException{Message: aws.String("internal failure")}
	}
	if meta, ok := f.keys[id]; ok {
		return &kms.DescribeKeyOutput{KeyMetadata: &meta}, nil
	}
	return nil, &kmstypes.NotFoundException{Message: aws.String("Key " + id + " is not found.")}
}

type refLambdaKeyFake struct {
	awsclient.LambdaAPI
	keyByFunction map[string]string
}

func (f *refLambdaKeyFake) GetFunction(_ context.Context, in *lambdapkg.GetFunctionInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.GetFunctionOutput, error) {
	name := aws.ToString(in.FunctionName)
	return &lambdapkg.GetFunctionOutput{Configuration: &lambdatypes.FunctionConfiguration{
		FunctionName: aws.String(name),
		KMSKeyArn:    aws.String(f.keyByFunction[name]),
	}}, nil
}

// refKMSPivotRun runs the three KMS pivots that read several references at
// once — an AMI's block devices, an instance's attached volumes, an API's
// Lambda integrations — with refs as their key references, against f.
func refKMSPivotRun(t *testing.T, b refBench, f *refKMSLookupFake, refs []string) map[string]resource.RelatedCheckResult {
	t.Helper()
	ctx := context.Background()
	clients := refClients()
	clients.KMS = f
	out := map[string]resource.RelatedCheckResult{}

	img := ec2types.Image{ImageId: aws.String("ami-0a1b2c3d4e5f60001")}
	for i, ref := range refs {
		img.BlockDeviceMappings = append(img.BlockDeviceMappings, ec2types.BlockDeviceMapping{
			DeviceName: aws.String("/dev/xvd" + string(rune('a'+i))),
			Ebs:        &ec2types.EbsBlockDevice{KmsKeyId: aws.String(ref), Encrypted: aws.Bool(true)},
		})
	}
	out["ami"] = refChecker(t, "ami", "kms")(ctx, clients, resource.Resource{ID: "ami-0a1b2c3d4e5f60001", Type: "ami", RawStruct: img}, b.cache)

	const inst = "i-0a1b2c3d4e5f60001"
	var vols []resource.Resource
	for i, ref := range refs {
		id := "vol-0e1f2a3b4c5d6e7f" + string(rune('a'+i))
		vols = append(vols, resource.Resource{ID: id, Type: "ebs", RawStruct: ec2types.Volume{
			VolumeId:    aws.String(id),
			KmsKeyId:    aws.String(ref),
			Encrypted:   aws.Bool(true),
			Attachments: []ec2types.VolumeAttachment{{InstanceId: aws.String(inst), VolumeId: aws.String(id)}},
		}})
	}
	cache := maps.Clone(b.cache)
	cache["ebs"] = resource.ResourceCacheEntry{Resources: vols}
	out["ec2"] = refChecker(t, "ec2", "kms")(ctx, clients, b.row(t, "ec2", inst), cache)

	keyByFn := map[string]string{}
	var uris []string
	for i, ref := range refs {
		fn := "acme-kms-probe-" + string(rune('a'+i))
		keyByFn[fn] = ref
		uris = append(uris, refLambdaInvokeURI("arn:aws:lambda:us-east-1:123456789012:function:"+fn))
	}
	clients.APIGatewayV2 = &refAPIGWFake{uris: uris}
	clients.Lambda = &refLambdaKeyFake{keyByFunction: keyByFn}
	out["apigw"] = refChecker(t, "apigw", "kms")(ctx, clients, b.row(t, "apigw", "abc123def4"), b.cache)
	return out
}

// TestKMSPivots_PartialLookupFailureKeepsWhatResolved: a key reference the
// loaded list answers is known whatever happens to the alias lookups beside
// it. A deleted alias names no key (dropped, not an error); a failed lookup
// leaves the count a lower bound; only a pivot that resolved nothing reports
// the failure as its answer.
func TestKMSPivots_PartialLookupFailureKeepsWhatResolved(t *testing.T) {
	b := newRefBench(t)
	orders := refOrdersKey()
	fake := func() *refKMSLookupFake {
		return &refKMSLookupFake{
			keys:     map[string]kmstypes.KeyMetadata{"alias/acme-orders": orders, aws.ToString(orders.KeyId): orders},
			notFound: []string{"alias/acme-retired"},
			failing:  []string{"alias/acme-flaky"},
		}
	}
	aliasARN := func(a string) string { return "arn:aws:kms:us-east-1:123456789012:" + a }

	cases := []struct {
		name          string
		refs          []string
		listErr       error
		wantIDs       []string
		wantTruncated bool
		wantError     bool
	}{
		{"deleted alias beside a listed key", []string{refKeyA1ARN, aliasARN("alias/acme-retired")}, nil, []string{refKeyA1}, true, false},
		{"failed alias lookup beside a listed key", []string{refKeyA1ARN, aliasARN("alias/acme-flaky")}, nil, []string{refKeyA1}, true, false},
		{"ListAliases refused, DescribeKey answered", []string{aliasARN("alias/acme-orders")}, errors.New("AccessDeniedException: kms:ListAliases"), []string{aws.ToString(orders.KeyId)}, true, false},
		{"nothing resolved", []string{aliasARN("alias/acme-flaky")}, nil, nil, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := fake()
			f.listAliasesErr = tc.listErr
			for source, got := range refKMSPivotRun(t, b, f, tc.refs) {
				if tc.wantError {
					if got.Err() == nil {
						t.Errorf("%s → KMS = %v (state %v), want the lookup error", source, got.ResourceIDs(), got.State())
					}
					continue
				}
				if got.Err() != nil || !slices.Equal(sortedIDs(got), tc.wantIDs) || got.Truncated() != tc.wantTruncated {
					t.Errorf("%s → KMS = %v truncated=%v err=%v, want %v truncated=%v and no error",
						source, sortedIDs(got), got.Truncated(), got.Err(), tc.wantIDs, tc.wantTruncated)
				}
			}
		})
	}
}

// ── F: a KMS alias field opens the key the related row counts ─────────────

// TestSSMKeyId_AWSManagedAliasNavigableWithoutTheKeyList: the operator opens
// the parameter before ever loading the kms list. Once the KMS row's result
// lazy-adds the AWS-managed key, KeyId opens that key — the badge and the
// field read the alias through one lookup.
func TestSSMKeyId_AWSManagedAliasNavigableWithoutTheKeyList(t *testing.T) {
	b := newRefBench(t)
	ctx := context.Background()
	param := b.row(t, "ssm", "/acme/legacy/db/password")
	related := refChecker(t, "ssm", "kms")(ctx, refClients(), param, b.cache)
	rows, _ := resource.GetFetchByIDs("kms")(ctx, refClients(), related.ResourceIDs()) //nolint:errcheck // the rows are the lazy-add the lane carries; a failure shows as no navigable field

	c, core := refDetailControllerCore(t, b, "kms")
	openDetail(c, "ssm", param)
	intents, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:       "ssm",
		SourceResourceID:   param.ID,
		DefDisplayName:     "KMS Key",
		Result:             related,
		LazyAddedResources: map[string][]resource.Resource{"kms": rows},
	})
	c.ApplyIntents(intents)
	f := fieldAt(t, c.Snapshot().Body.Detail.Fields, "KeyId")
	if !f.IsNavigable || f.TargetType != "kms" || navTarget(f) != fixtures.SSMDefaultKeyID {
		t.Errorf("KeyId with no kms list loaded: navigable=%v target=%q opens %q, want %q (KMS row IDs %v)",
			f.IsNavigable, f.TargetType, navTarget(f), fixtures.SSMDefaultKeyID, related.ResourceIDs())
	}
}

// ── G: KMS → secrets reads the viewed key's own aliases ───────────────────

// TestKMSSecrets_ViewedKeyAliasesMatchWithoutTheKeyList: a secret names its
// key by alias ARN, and the viewed key (opened from a pivot, the kms list not
// loaded) carries that alias. The secret counts; an alias ref nothing can
// read leaves the count a lower bound.
func TestKMSSecrets_ViewedKeyAliasesMatchWithoutTheKeyList(t *testing.T) {
	b := newRefBench(t)
	key := b.row(t, "kms", refKeyA1)
	secret := func(name, keyRef string) resource.Resource {
		return resource.Resource{ID: name, Name: name, Type: "secrets", RawStruct: smtypes.SecretListEntry{
			Name:     aws.String(name),
			ARN:      aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:" + name + "-QwErTy"),
			KmsKeyId: aws.String(keyRef),
		}}
	}
	cache := resource.ResourceCache{"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{
		secret("prod/by-alias-arn", "arn:aws:kms:us-east-1:123456789012:"+key.Fields["alias"]),
		secret("prod/by-key-arn", refKeyA1ARN),
		secret("prod/by-other-alias", "alias/acme-unlisted-key"),
	}}}
	got := refChecker(t, "kms", "secrets")(context.Background(), refClients(), key, cache)
	want := []string{"prod/by-alias-arn", "prod/by-key-arn"}
	if ids := sortedIDs(got); !slices.Equal(ids, want) || !got.Truncated() {
		t.Errorf("kms %s → Secrets = %v truncated=%v, want %v truncated=true", key.ID, ids, got.Truncated(), want)
	}
}

// ── I: the demo bench shows each reference shape ──────────────────────────

func refRawJSON(r resource.Resource) string {
	raw, err := json.Marshal(r.RawStruct)
	if err != nil {
		return ""
	}
	return string(raw)
}

// TestDemoKMS_SecondAliasIsReferencedAndCounted: one demo key carries two
// aliases, and a demo resource names it by the one its row does not display;
// that resource's KMS row counts the key.
func TestDemoKMS_SecondAliasIsReferencedAndCounted(t *testing.T) {
	b := newRefBench(t)
	for _, key := range b.byType["kms"] {
		for _, alias := range strings.Split(key.Fields["aliases"], ",") {
			if alias == "" || alias == key.Name {
				continue
			}
			for _, td := range resource.AllResourceTypes() {
				if td.ShortName == "kms" {
					continue
				}
				for _, def := range resource.GetRelated(td.ShortName) {
					if def.TargetType != "kms" {
						continue
					}
					for _, src := range b.byType[td.ShortName] {
						if !strings.Contains(refRawJSON(src), `"`+alias+`"`) && !strings.Contains(refRawJSON(src), ":"+alias+`"`) {
							continue
						}
						ids := def.Checker(context.Background(), refClients(), src, b.cache).ResourceIDs()
						if !slices.Contains(ids, key.ID) {
							t.Errorf("%s %s names key %s by its second alias %s, but KMS IDs = %v", td.ShortName, src.ID, key.ID, alias, ids)
						}
						return
					}
				}
			}
		}
	}
	t.Fatal("no demo resource names a kms key by a second alias (a key with two aliases, referenced by the one its row does not display)")
}

// TestDemoCb_SecretInARNFormIsCounted: one demo CodeBuild project names its
// secret by ARN, so the demo exercises the form a name-only fixture hides.
func TestDemoCb_SecretInARNFormIsCounted(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "cb", "secrets")
	for _, p := range b.byType["cb"] {
		project, ok := p.RawStruct.(cbtypes.Project)
		if !ok || project.Environment == nil {
			continue
		}
		for _, env := range project.Environment.EnvironmentVariables {
			v := aws.ToString(env.Value)
			if env.Type != cbtypes.EnvironmentVariableTypeSecretsManager || !strings.HasPrefix(v, "arn:") {
				continue
			}
			want, ok := resource.ResolveRef("secrets", v, b.rc("secrets"))
			if !ok || !b.has("secrets", want) {
				t.Fatalf("cb %s secret %q resolves to (%q, %v), not a secrets row", p.ID, v, want, ok)
			}
			if ids := check(context.Background(), refClients(), p, b.cache).ResourceIDs(); !slices.Contains(ids, want) {
				t.Errorf("cb %s → Secrets = %v, want it to include %s", p.ID, ids, want)
			}
			return
		}
	}
	t.Fatal("no demo cb project names a SECRETS_MANAGER secret by ARN")
}

// TestDemoKMS_NoCustomerKeyInTheReservedAliasNamespace: AWS reserves
// alias/aws/ for AWS-managed keys, so no customer key in the list carries such
// an alias, and nothing references the renamed alias/aws/disabled-key.
func TestDemoKMS_NoCustomerKeyInTheReservedAliasNamespace(t *testing.T) {
	b := newRefBench(t)
	for _, key := range b.byType["kms"] {
		aliases := append([]string{key.Name, key.Fields["alias"]}, strings.Split(key.Fields["aliases"], ",")...)
		if i := slices.IndexFunc(aliases, func(a string) bool { return strings.HasPrefix(a, "alias/aws/") }); i >= 0 {
			t.Errorf("customer key %s carries reserved alias %s", key.ID, aliases[i])
		}
	}
	for typ, rows := range b.byType {
		for _, r := range rows {
			if strings.Contains(refRawJSON(r), "alias/aws/disabled-key") {
				t.Errorf("%s %s still references alias/aws/disabled-key", typ, r.ID)
			}
		}
	}
}

// ── J: the CloudTrail JSON fallback keeps the principal's account ─────────

// TestCtEventsPrincipals_JSONFallbackKeepsTheAccount: an event whose only
// role evidence is userIdentity.sessionContext.sessionIssuer (LookupEvents
// reports the session name as Username, and there is no roleArn or role
// resource) names the issuing role with its account. A role in another
// account is no local role; the iam-user pivot reads another account's user
// the same way.
func TestCtEventsPrincipals_JSONFallbackKeepsTheAccount(t *testing.T) {
	b := newRefBench(t)
	const role = "a9s-demo-s3-access-role"
	const user = "alice.johnson"
	bySession := func(id, account string) cloudtrailtypes.Event {
		return refCTEvent(id, "DescribeInstances", "ops-1759", map[string]any{
			"eventName":          "DescribeInstances",
			"recipientAccountId": refAccount,
			"userIdentity": map[string]any{
				"type": "AssumedRole", "accountId": account,
				"arn": "arn:aws:sts::" + account + ":assumed-role/" + role + "/ops-1759",
				"sessionContext": map[string]any{"sessionIssuer": map[string]any{
					"type": "Role", "accountId": account, "userName": role,
					"arn": "arn:aws:iam::" + account + ":role/" + role,
				}},
			},
		})
	}
	byUser := func(id, account string) cloudtrailtypes.Event {
		return refCTEvent(id, "ListBuckets", user, map[string]any{
			"eventName":          "ListBuckets",
			"recipientAccountId": refAccount,
			"userIdentity": map[string]any{
				"type": "IAMUser", "accountId": account, "userName": user,
				"arn": "arn:aws:iam::" + account + ":user/" + user,
			},
		})
	}
	rows := refCTRows(t,
		bySession("evt-json-local", refAccount), bySession("evt-json-foreign", refForeignAccount),
		byUser("evt-user-local", refAccount), byUser("evt-user-foreign", refForeignAccount),
	)
	cases := []struct {
		id, target string
		want       []string
	}{
		{"evt-json-local", "role", []string{role}},
		{"evt-json-foreign", "role", nil},
		{"evt-user-local", "iam-user", []string{user}},
		{"evt-user-foreign", "iam-user", nil},
	}
	for _, tc := range cases {
		got := refChecker(t, "ct-events", tc.target)(context.Background(), refClients(), rows[tc.id], b.cache)
		if ids := sortedIDs(got); !slices.Equal(ids, tc.want) {
			t.Errorf("%s → %s IDs = %v, want %v", tc.id, tc.target, ids, tc.want)
		}
	}
}

// TestDemoCtEvents_ForeignPrincipalReadsNoLocalRole is the demo witness: the
// Karpenter event was made by a role in account 111111111111, so its IAM
// Roles row reads (0). The local role list is the caller's account only, so
// no role row carries another account's ARN.
func TestDemoCtEvents_ForeignPrincipalReadsNoLocalRole(t *testing.T) {
	b := newRefBench(t)
	for _, r := range b.byType["role"] {
		var raw struct{ Arn string }
		if err := json.Unmarshal([]byte(refRawJSON(r)), &raw); err == nil && raw.Arn != "" &&
			!strings.HasPrefix(raw.Arn, "arn:aws:iam::"+refAccount+":") {
			t.Errorf("role %s in the local list carries another account's ARN %s", r.ID, raw.Arn)
		}
	}
	ev := b.row(t, "ct-events", "e-a1b2c3d4")
	got := refChecker(t, "ct-events", "role")(context.Background(), refClients(), ev, b.cache)
	if got.EffectiveState() != domain.RelatedResolved || got.Count() != 0 {
		t.Errorf("e-a1b2c3d4 (KarpenterNodeRole in 111111111111) → IAM Roles = %v (state %v), want a resolved 0",
			got.ResourceIDs(), got.EffectiveState())
	}
}

// ── K: Route 53 → API Gateway reads the host through the apigw resolver ──

type refR53Fake struct {
	awsclient.Route53API
	aliases []string
}

func (f *refR53Fake) ListResourceRecordSets(context.Context, *route53.ListResourceRecordSetsInput, ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	out := &route53.ListResourceRecordSetsOutput{}
	for i, a := range f.aliases {
		out.ResourceRecordSets = append(out.ResourceRecordSets, r53types.ResourceRecordSet{
			Name:        aws.String("api" + string(rune('a'+i)) + ".acme-corp.com."),
			Type:        r53types.RRTypeA,
			AliasTarget: &r53types.AliasTarget{DNSName: aws.String(a + "."), HostedZoneId: aws.String("Z1UJRXOUMOOFQ8")},
		})
	}
	return out, nil
}

// TestR53APIGW_ExecuteAPIHostsReadThroughTheResolver: a zone's alias records
// name APIs by their execute-api host. The API a host names is counted by its
// row ID, and a host naming an API the list does not hold makes the count a
// lower bound instead of a proven number.
func TestR53APIGW_ExecuteAPIHostsReadThroughTheResolver(t *testing.T) {
	b := newRefBench(t)
	zone := b.byType["r53"][0]
	clients := refClients()
	clients.Route53 = &refR53Fake{aliases: []string{
		"abc123def4.execute-api.us-east-1.amazonaws.com",
		"zz99nope00.execute-api.us-east-1.amazonaws.com",
		"d111111abcdef8.cloudfront.net",
	}}
	got := refChecker(t, "r53", "apigw")(context.Background(), clients, zone, b.cache)
	if ids := sortedIDs(got); !slices.Equal(ids, []string{"abc123def4"}) || !got.Truncated() {
		t.Errorf("r53 → API Gateways = %v truncated=%v, want [abc123def4] truncated=true", ids, got.Truncated())
	}
}
