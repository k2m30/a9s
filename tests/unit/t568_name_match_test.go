package unit_test

// A name joins two resources only on the boundary AWS gives it: equal names
// are the same name, a task definition's family is the part of its ARN
// before the revision, a domain matches label by label under the rule of the
// service that reads it, and an ARN that is known is the answer, never
// overruled by a name its last field happens to end in. One name that is a
// prefix, suffix or substring of another is a different resource.

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t568RequireExact fails unless r is an exact count of exactly want.
func t568RequireExact(t *testing.T, pivot string, r resource.RelatedCheckResult, want ...string) {
	t.Helper()
	slices.Sort(want)
	if want == nil {
		want = []string{}
	}
	got := sortedIDs(r)
	if got == nil {
		got = []string{}
	}
	if r.EffectiveState() != domain.RelatedResolved || r.Coverage() != domain.CoverageComplete || !slices.Equal(got, want) {
		t.Errorf("%s = %v (state %v, coverage %v, err %v), want exactly %v", pivot, got, r.EffectiveState(), r.Coverage(), r.Err(), want)
	}
}

// ─── task-definition families in a state machine ──────────────────────────

// t568SFN answers DescribeStateMachine with each machine's definition.
type t568SFN struct {
	awsclient.SFNAPI
	defs map[string]string
}

func (f *t568SFN) DescribeStateMachine(_ context.Context, in *sfn.DescribeStateMachineInput, _ ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	arn := aws.ToString(in.StateMachineArn)
	def, ok := f.defs[arn]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "StateMachineDoesNotExist", Message: "State Machine Does Not Exist: '" + arn + "'"}
	}
	return &sfn.DescribeStateMachineOutput{StateMachineArn: aws.String(arn), Definition: aws.String(def)}, nil
}

func t568RunTaskMachine(stateKey, taskDef string) string {
	switch stateKey {
	case "Arguments":
		return `{"QueryLanguage":"JSONata","StartAt":"Run","States":{"Run":{"Type":"Task","Resource":"arn:aws:states:::ecs:runTask.sync","Arguments":{"Cluster":"` + t568Cluster + `","TaskDefinition":"` + taskDef + `","LaunchType":"FARGATE"},"End":true}}}`
	default:
		return `{"StartAt":"Run","States":{"Run":{"Type":"Task","Resource":"arn:aws:states:::ecs:runTask.sync","Parameters":{"Cluster":"` + t568Cluster + `","TaskDefinition":"` + taskDef + `","LaunchType":"FARGATE"},"End":true}}}`
	}
}

// Family api runs in the machines whose Task state names a task definition
// of family api — by ARN at any revision in Parameters, or by family in a
// JSONata Arguments block — and in none that names api-worker or
// api-webhook.
func TestNameMatch_ECSFamilyInAStateMachineIsTheWholeFamily(t *testing.T) {
	const smPrefix = "arn:aws:states:us-east-1:123456789012:stateMachine:"
	machines := map[string]string{
		"acme-api-migrate":   t568RunTaskMachine("Parameters", "arn:aws:ecs:us-east-1:123456789012:task-definition/api:40"),
		"acme-api-backfill":  t568RunTaskMachine("Arguments", "api"),
		"acme-worker-drain":  t568RunTaskMachine("Parameters", t568WorkTaskDef),
		"acme-webhook-retry": t568RunTaskMachine("Arguments", "api-webhook"),
	}
	fake := &t568SFN{defs: map[string]string{}}
	var rows []resource.Resource
	for name, def := range machines {
		fake.defs[smPrefix+name] = def
		rows = append(rows, resource.Resource{ID: name, Name: name, Type: "sfn", Fields: map[string]string{"arn": smPrefix + name, "name": name}})
	}
	clients := &awsclient.ServiceClients{SFN: fake, Region: "us-east-1"}
	got := refChecker(t, "ecs-svc", "sfn")(context.Background(), clients, t568ECSService(), resource.ResourceCache{"sfn": {Resources: rows}})
	t568RequireExact(t, "ecs-svc api → sfn", got, "acme-api-backfill", "acme-api-migrate")
}

// ─── SES receipt-rule recipients ──────────────────────────────────────────

// t568SES answers DescribeActiveReceiptRuleSet with one active rule set.
type t568SES struct {
	awsclient.SESV1API
	rules []sestypes.ReceiptRule
}

func (f *t568SES) DescribeActiveReceiptRuleSet(_ context.Context, _ *ses.DescribeActiveReceiptRuleSetInput, _ ...func(*ses.Options)) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	return &ses.DescribeActiveReceiptRuleSetOutput{
		Metadata: &sestypes.ReceiptRuleSetMetadata{Name: aws.String("acme-inbound")},
		Rules:    f.rules,
	}, nil
}

func t568ReceiptRule(name, fn string, recipients ...string) sestypes.ReceiptRule {
	return sestypes.ReceiptRule{
		Name: aws.String(name), Enabled: true, Recipients: recipients, ScanEnabled: true,
		Actions: []sestypes.ReceiptAction{{LambdaAction: &sestypes.LambdaAction{
			FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + fn), InvocationType: sestypes.InvocationTypeEvent,
		}}},
	}
}

// A receipt rule's recipient "acme.com" matches every address in acme.com
// "but not those within its subdomains"; ".acme.com" matches the subdomains
// and not the parent; "sub.acme.com" matches that subdomain only; an address
// matches that address; no recipient matches every verified domain.
// https://docs.aws.amazon.com/ses/latest/dg/receiving-email-receipt-rules-console-walkthrough.html
func TestNameMatch_SESRecipientFollowsTheReceiptRuleDomainRule(t *testing.T) {
	fake := &t568SES{rules: []sestypes.ReceiptRule{
		t568ReceiptRule("parent-domain", "ses-parent", "acme.com"),
		t568ReceiptRule("all-subdomains", "ses-subdomains", ".acme.com"),
		t568ReceiptRule("this-subdomain", "ses-sub", "sub.acme.com"),
		t568ReceiptRule("this-address", "ses-address", "x@sub.acme.com"),
		t568ReceiptRule("other-address", "ses-other", "y@sub.acme.com"),
		t568ReceiptRule("every-domain", "ses-all"),
	}}
	identity := resource.Resource{ID: "x@sub.acme.com", Name: "x@sub.acme.com", Type: "ses",
		Fields: map[string]string{"identity_type": "email address", "verification_status": "Success"}}
	got := refChecker(t, "ses", "lambda")(context.Background(), &awsclient.ServiceClients{SES: fake, Region: "us-east-1"}, identity, resource.ResourceCache{})
	t568RequireExact(t, "ses x@sub.acme.com → lambda", got, "ses-address", "ses-all", "ses-sub", "ses-subdomains")
}

// The records of a name live in the innermost public hosted zone that holds
// it: a parent zone that delegates the subdomain to a zone of its own does
// not, and a private zone is not the DNS the world resolves the identity in.
func TestNameMatch_SESIdentityZoneIsTheInnermostPublicZone(t *testing.T) {
	zone := func(id, name string, private bool) resource.Resource {
		return resource.Resource{ID: id, Name: name, Type: "r53",
			Fields: map[string]string{"zone_id": id, "name": name, "private_zone": strconv.FormatBool(private), "record_count": "12"},
			RawStruct: route53types.HostedZone{Id: aws.String(id), Name: aws.String(name), CallerReference: aws.String(id),
				Config: &route53types.HostedZoneConfig{PrivateZone: private}, ResourceRecordSetCount: aws.Int64(12)},
		}
	}
	zones := resource.ResourceCache{"r53": {Resources: []resource.Resource{
		zone("/hostedzone/Z0EXAMPLEPARENT", "acme.com.", false),
		zone("/hostedzone/Z0EXAMPLECHILD1", "mail.acme.com.", false),
		zone("/hostedzone/Z0EXAMPLEPRIVAT", "mail.acme.com.", true),
		zone("/hostedzone/Z0EXAMPLEPRIV2Z", "acme.com.", true),
	}}}
	check := refChecker(t, "ses", "r53")
	for _, tc := range []struct {
		identity string
		want     string
	}{
		{"mail.acme.com", "/hostedzone/Z0EXAMPLECHILD1"},
		{"billing@mail.acme.com", "/hostedzone/Z0EXAMPLECHILD1"},
		{"shop.acme.com", "/hostedzone/Z0EXAMPLEPARENT"},
	} {
		t.Run(tc.identity, func(t *testing.T) {
			typ := "domain"
			if strings.Contains(tc.identity, "@") {
				typ = "email address"
			}
			id := resource.Resource{ID: tc.identity, Name: tc.identity, Type: "ses", Fields: map[string]string{"identity_type": typ}}
			t568RequireExact(t, "ses "+tc.identity+" → r53", check(context.Background(), nil, id, zones), tc.want)
		})
	}
}

// ─── an SQS subscription endpoint is the queue's ARN ──────────────────────

// An sqs-protocol subscription's Endpoint is the queue's ARN. A queue named
// orders in another Region or another account ends in the same name and is
// another queue.
func TestNameMatch_SubscriptionEndpointIsTheQueueARN(t *testing.T) {
	const queueARN = "arn:aws:sqs:us-east-1:123456789012:orders"
	queue := resource.Resource{ID: "orders", Name: "orders", Type: "sqs", RawStruct: awsclient.SQSQueueAttributesRow{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/orders", QueueName: "orders",
		Attributes: map[string]string{"QueueArn": queueARN},
	}}
	sub := func(subARN, topicARN, endpoint string) resource.Resource {
		return resource.Resource{ID: subARN, Name: subARN, Type: "sns-sub", Fields: map[string]string{
			"subscription_arn": subARN, "topic_arn": topicARN, "protocol": "sqs", "endpoint": endpoint,
		}}
	}
	const (
		local   = "arn:aws:sns:us-east-1:123456789012:order-events:0f1e2d3c-4b5a-4978-8a9b-0c1d2e3f4a5b"
		west    = "arn:aws:sns:us-west-2:123456789012:order-events:1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"
		foreign = "arn:aws:sns:us-east-1:210987654321:partner-orders:2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e"
		dlq     = "arn:aws:sns:us-east-1:123456789012:order-events:3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f"
	)
	subs := resource.ResourceCache{"sns-sub": {Resources: []resource.Resource{
		sub(local, "arn:aws:sns:us-east-1:123456789012:order-events", queueARN),
		sub(west, "arn:aws:sns:us-west-2:123456789012:order-events", "arn:aws:sqs:us-west-2:123456789012:orders"),
		sub(foreign, "arn:aws:sns:us-east-1:210987654321:partner-orders", "arn:aws:sqs:us-east-1:210987654321:orders"),
		sub(dlq, "arn:aws:sns:us-east-1:123456789012:order-events", "arn:aws:sqs:us-east-1:123456789012:orders-dlq"),
	}}}
	t568RequireExact(t, "sqs orders → sns-sub", refChecker(t, "sqs", "sns-sub")(context.Background(), nil, queue, subs), local)
	t568RequireExact(t, "sqs orders → sns", refChecker(t, "sqs", "sns")(context.Background(), nil, queue, subs), "arn:aws:sns:us-east-1:123456789012:order-events")
}

// ─── a Lambda function's name inside an API's name ────────────────────────

// t568APIGW answers GetIntegrations for each API as API Gateway does.
type t568APIGW struct {
	awsclient.APIGatewayV2API
	integrations map[string][]apigwtypes.Integration
	links        []apigwtypes.VpcLink
}

func (f *t568APIGW) GetIntegrations(_ context.Context, in *apigatewayv2.GetIntegrationsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	return &apigatewayv2.GetIntegrationsOutput{Items: f.integrations[aws.ToString(in.ApiId)]}, nil
}

func (f *t568APIGW) GetVpcLinks(_ context.Context, _ *apigatewayv2.GetVpcLinksInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetVpcLinksOutput, error) {
	return &apigatewayv2.GetVpcLinksOutput{Items: f.links}, nil
}

func t568HTTPAPI(id, name string) resource.Resource {
	return resource.Resource{ID: id, Name: name, Type: "apigw", Fields: map[string]string{"api_id": id, "name": name, "protocol_type": "HTTP"},
		RawStruct: apigwtypes.Api{ApiId: aws.String(id), Name: aws.String(name), ProtocolType: apigwtypes.ProtocolTypeHttp,
			ApiEndpoint: aws.String("https://" + id + ".execute-api.us-east-1.amazonaws.com")}}
}

// An API invokes a function through an integration whose IntegrationUri is
// the function's ARN. An API whose name holds the function's name invokes
// nothing by that; the pivot reads the integrations or reads unknown.
func TestNameMatch_FunctionNameInsideAnAPINameIsNoLink(t *testing.T) {
	const fnARN = "arn:aws:lambda:us-east-1:123456789012:function:orders"
	fn := resource.Resource{ID: "orders", Name: "orders", Type: "lambda", RawStruct: lambdatypes.FunctionConfiguration{
		FunctionName: aws.String("orders"), FunctionArn: aws.String(fnARN), Runtime: lambdatypes.RuntimeNodejs22x,
	}}
	fake := &t568APIGW{integrations: map[string][]apigwtypes.Integration{
		"a1b2c3d4e5": {{IntegrationId: aws.String("int001"), IntegrationType: apigwtypes.IntegrationTypeAwsProxy,
			IntegrationUri: aws.String("arn:aws:lambda:us-east-1:123456789012:function:orders-legacy"), PayloadFormatVersion: aws.String("2.0")}},
		"f6g7h8i9j0": {{IntegrationId: aws.String("int002"), IntegrationType: apigwtypes.IntegrationTypeAwsProxy,
			IntegrationUri: aws.String(fnARN), PayloadFormatVersion: aws.String("2.0")}},
	}}
	apis := resource.ResourceCache{"apigw": {Resources: []resource.Resource{t568HTTPAPI("a1b2c3d4e5", "orders-api"), t568HTTPAPI("f6g7h8i9j0", "checkout")}}}
	got := refChecker(t, "lambda", "apigw")(context.Background(), &awsclient.ServiceClients{APIGatewayV2: fake, Region: "us-east-1"}, fn, apis)
	if slices.Contains(got.ResourceIDs(), "a1b2c3d4e5") {
		t.Errorf("lambda orders → apigw = %v (coverage %v): API orders-api integrates orders-legacy, not orders", got.ResourceIDs(), got.Coverage())
	}
	if got.EffectiveState() == domain.RelatedResolved && got.Coverage() == domain.CoverageComplete {
		t568RequireExact(t, "lambda orders → apigw", got, "f6g7h8i9j0")
	}
}
