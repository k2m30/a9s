package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	"github.com/k2m30/a9s/v3/core/domain"
)

const t4APIGWNoDeployedStage = "apigw.no-deployed-stages"

func TestT4_APIGW_RESTAuthorizerNoMethodUsesStaysOpen(t *testing.T) {
	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{
				Id:           aws.String("cog01"),
				Name:         aws.String("acme-users"),
				Type:         apigwtypes.AuthorizerTypeCognitoUserPools,
				ProviderARNs: []string{"arn:aws:cognito-idp:eu-central-1:123456789012:userpool/eu-central-1_AbCdEf123"},
			}},
			resources: w6aRESTMethods("NONE", ""),
			stages:    []apigwtypes.Stage{w6aV1Stage("prod")},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst101unused", "acme-unused-authorizer-rest", "REGIONAL", ""),
	)
	w2AssertFinding(t, res.Findings["rst101unused"], w6aAPIGWNoAuthPublic,
		"internet-facing with no authorizer", domain.SevBroken, "wave2")

	attached := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("cog01"), Type: apigwtypes.AuthorizerTypeCognitoUserPools}},
			resources:   w6aRESTMethods("COGNITO_USER_POOLS", "cog01"),
			stages:      []apigwtypes.Stage{w6aV1Stage("prod")},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst102attached", "acme-attached-authorizer-rest", "REGIONAL", ""),
	)
	w2AssertNoCode(t, attached.Findings["rst102attached"], w6aAPIGWNoAuthPublic)
	w2AssertNoCode(t, attached.Findings["rst102attached"], w6aAPIGWNoAuth)
}

func TestT4_APIGW_RESTEveryMethodIAMNeedsNoAuthorizer(t *testing.T) {
	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{resources: w6aRESTMethods("AWS_IAM", ""), stages: []apigwtypes.Stage{w6aV1Stage("prod")}},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst103iam", "acme-signed-rest", "EDGE", ""),
	)
	w2AssertNoCode(t, res.Findings["rst103iam"], w6aAPIGWNoAuthPublic)
	w2AssertNoCode(t, res.Findings["rst103iam"], w6aAPIGWNoAuth)
	if cause, marked := res.TruncatedIDs["rst103iam"]; marked {
		t.Errorf("rst103iam marked not inspected (%q); every read answered", cause)
	}
}

func TestT4_APIGW_RESTStagesWithoutADeploymentAreUndeployed(t *testing.T) {
	draft := w6aV1Stage("draft")
	draft.DeploymentId = nil
	staging := w6aV1Stage("staging")
	staging.DeploymentId = nil

	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{resources: w6aRESTMethods("AWS_IAM", ""), stages: []apigwtypes.Stage{draft, staging}},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst104undeployed", "acme-undeployed-rest", "REGIONAL", ""),
	)
	if !hasCode(res.Findings["rst104undeployed"], t4APIGWNoDeployedStage) {
		t.Errorf("findings = %v, want %s: no stage carries a DeploymentId", w2Codes(res.Findings["rst104undeployed"]), t4APIGWNoDeployedStage)
	}

	deployed := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{resources: w6aRESTMethods("AWS_IAM", ""), stages: []apigwtypes.Stage{draft, w6aV1Stage("prod")}},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst105deployed", "acme-deployed-rest", "REGIONAL", ""),
	)
	if hasCode(deployed.Findings["rst105deployed"], t4APIGWNoDeployedStage) {
		t.Errorf("findings = %v, want no %s: stage prod is deployed", w2Codes(deployed.Findings["rst105deployed"]), t4APIGWNoDeployedStage)
	}
}

func TestT4_APIGW_HTTPStagesWithoutADeploymentAreUndeployed(t *testing.T) {
	pending := w6aV2HealthyStage("$default")
	pending.DeploymentId = nil
	beta := w6aV2HealthyStage("beta")
	beta.DeploymentId = nil
	iam := w6aHTTPRoutes(apigwv2types.AuthorizationTypeAwsIam, "")

	res := w6aEnrichAPIGW(t, nil,
		&w6aAPIGWV2Fake{routes: iam, stages: []apigwv2types.Stage{pending, beta}},
		w6aHTTPRes("htp101undeployed", "acme-undeployed-http"),
	)
	if !hasCode(res.Findings["htp101undeployed"], t4APIGWNoDeployedStage) {
		t.Errorf("findings = %v, want %s: no stage carries a DeploymentId", w2Codes(res.Findings["htp101undeployed"]), t4APIGWNoDeployedStage)
	}

	deployed := w6aEnrichAPIGW(t, nil,
		&w6aAPIGWV2Fake{routes: iam, stages: []apigwv2types.Stage{pending, w6aV2HealthyStage("prod")}},
		w6aHTTPRes("htp102deployed", "acme-deployed-http"),
	)
	if hasCode(deployed.Findings["htp102deployed"], t4APIGWNoDeployedStage) {
		t.Errorf("findings = %v, want no %s: stage prod is deployed", w2Codes(deployed.Findings["htp102deployed"]), t4APIGWNoDeployedStage)
	}
}
