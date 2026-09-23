package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
)

// A pipeline action's configuration is provider-defined and free-form: a
// CodeBuild action's EnvironmentVariables holds PLAINTEXT values and a Lambda
// action's UserParameters is arbitrary text, so the record keeps keys only.
func TestSecretValue_PipelineActionKeepsConfigurationKeysOnly(t *testing.T) {
	action := cptypes.ActionDeclaration{
		Name: aws.String("Build"),
		ActionTypeId: &cptypes.ActionTypeId{
			Category: cptypes.ActionCategoryBuild,
			Owner:    cptypes.ActionOwnerAws,
			Provider: aws.String("CodeBuild"),
			Version:  aws.String("1"),
		},
		Configuration: map[string]string{
			"ProjectName":          "orders-build",
			"EnvironmentVariables": `[{"name":"API_TOKEN","value":"tok-example-123","type":"PLAINTEXT"}]`,
		},
		RoleArn: aws.String("arn:aws:iam::123456789012:role/pipeline-action"),
	}

	pa := pipelineActionOf("BuildStage", action)

	if want := []string{"EnvironmentVariables", "ProjectName"}; !reflect.DeepEqual(pa.ConfigurationKeys, want) {
		t.Errorf("ConfigurationKeys = %q, want %q", pa.ConfigurationKeys, want)
	}
	if pa.StageName != "BuildStage" || pa.ActionName != "Build" || pa.Provider != "CodeBuild" || pa.RoleArn != "arn:aws:iam::123456789012:role/pipeline-action" {
		t.Errorf("pipelineActionOf lost identity fields: %+v", pa)
	}
	raw, err := json.Marshal(pa)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "tok-example-123") {
		t.Errorf("record JSON carries a configuration value: %s", raw)
	}

	raw, err = json.Marshal(pipelineActionOf("Source", cptypes.ActionDeclaration{Name: aws.String("Checkout")}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "configuration") {
		t.Errorf("action without configuration emits a configuration field: %s", raw)
	}
}

// CodeBuild treats a variable with no type as PLAINTEXT, so its value is the
// secret itself.
func TestSecretValue_CodeBuildEnvVarWithoutTypeDropsValue(t *testing.T) {
	got := cbEnvVarOf(cbtypes.EnvironmentVariable{Name: aws.String("API_TOKEN"), Value: aws.String("tok-example-123")})
	if got.Name != "API_TOKEN" || got.Value != "" {
		t.Errorf("cbEnvVarOf = %+v, want name kept and no value", got)
	}
}

// A CloudTrail event's requestParameters and responseElements echo the API
// call's own payload (a CreateProject body with PLAINTEXT variables, a Glue
// CreateJob with its arguments); the envelope around them is CloudTrail's.
func TestSecretValue_CloudTrailEventDropsRequestAndResponsePayload(t *testing.T) {
	raw := `{"eventName":"CreateProject","errorCode":"AccessDenied","userIdentity":{"type":"IAMUser"},` +
		`"requestParameters":{"environment":{"environmentVariables":[{"name":"API_TOKEN","value":"tok-example-123","type":"PLAINTEXT"}]}},` +
		`"responseElements":{"project":{"environment":{"environmentVariables":[{"value":"tok-example-123"}]}}}}`

	got := ctEventEnvelopeOf(raw)

	if strings.Contains(got, "tok-example-123") || strings.Contains(got, "requestParameters") || strings.Contains(got, "responseElements") {
		t.Errorf("event keeps the call payload: %s", got)
	}
	for _, kept := range []string{`"eventName":"CreateProject"`, `"errorCode":"AccessDenied"`, `"userIdentity":{"type":"IAMUser"}`} {
		if !strings.Contains(got, kept) {
			t.Errorf("event lost %s: %s", kept, got)
		}
	}
	if got := ctEventEnvelopeOf(`not json tok-example-123`); got != "" {
		t.Errorf("unparseable event copied verbatim: %q", got)
	}
}

// A Git source URL may embed the repository credential as userinfo; the
// address without it is what the record keeps.
func TestSecretValue_CodeBuildSourceLocationDropsUserinfo(t *testing.T) {
	cases := map[string]string{
		"https://ci-bot:ghp-example-token@github.com/acme/orders.git": "https://github.com/acme/orders.git",
		"https://ghp-example-token@bitbucket.org/acme/orders.git":     "https://bitbucket.org/acme/orders.git",
		"https://github.com/acme/orders.git":                          "https://github.com/acme/orders.git",
		"https://git-codecommit.us-east-1.amazonaws.com/v1/repos/app": "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/app",
		"acme-artifacts/source/app.zip":                               "acme-artifacts/source/app.zip",
		"":                                                            "",
	}
	for in, want := range cases {
		if got := cbSourceLocationOf(in); got != want {
			t.Errorf("cbSourceLocationOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// An HTTP(S) subscription's path and query routinely are the webhook's shared
// secret and its userinfo is SNS's documented basic-auth form, so only the
// origin is kept; the other protocols' endpoints are ARNs, addresses or
// numbers.
func TestSecretValue_SNSSubscriptionEndpointKeepsOriginForURLs(t *testing.T) {
	cases := []struct{ protocol, endpoint, want string }{
		{"https", "https://events.pagerduty.example/integration/key-example-123/enqueue", "https://events.pagerduty.example"},
		{"http", "http://hooks.acme.example/notify?token=tok-example-123", "http://hooks.acme.example"},
		{"https", "https://svc:pw-example-123@hooks.acme.example:8443/sns", "https://hooks.acme.example:8443"},
		{"https", "::not a url tok-example-123", ""},
		{"email", "ops@acme.example", "ops@acme.example"},
		{"sqs", "arn:aws:sqs:us-east-1:123456789012:orders", "arn:aws:sqs:us-east-1:123456789012:orders"},
		{"lambda", "arn:aws:lambda:us-east-1:123456789012:function:notify", "arn:aws:lambda:us-east-1:123456789012:function:notify"},
		{"sms", "+15555550100", "+15555550100"},
	}
	for _, tc := range cases {
		if got := snsSubEndpointOf(tc.protocol, tc.endpoint); got != tc.want {
			t.Errorf("snsSubEndpointOf(%q, %q) = %q, want %q", tc.protocol, tc.endpoint, got, tc.want)
		}
	}
}
