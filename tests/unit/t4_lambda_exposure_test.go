package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

func t4OpenURL(id string) []lambdatypes.FunctionUrlConfig {
	return []lambdatypes.FunctionUrlConfig{{
		FunctionUrl: aws.String("https://" + id + ".lambda-url.us-east-1.on.aws/"),
		FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:acme-url"),
		AuthType:    lambdatypes.FunctionUrlAuthTypeNone,
	}}
}

func t4LambdaExposures(res awsclient.IssueEnricherResult, name string) []domain.Finding {
	var out []domain.Finding
	for _, f := range res.Findings[name] {
		if f.Code == pw1LambdaCodePublicPolicy || f.Code == pw1LambdaCodeURLPublic {
			out = append(out, f)
		}
	}
	return out
}

func TestT4_Lambda_OpenURLAndItsGrantAreOneFinding(t *testing.T) {
	const name = "acme-url-open"
	fake := &pw1LambdaPostureFake{
		policies:   map[string]string{name: pw1FunctionURLPolicy(name, "NONE")},
		urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{name: t4OpenURL("t4aaaa1111")},
	}
	res := pw1EnrichLambda(t, fake, name)

	got := t4LambdaExposures(res, name)
	if len(got) != 1 || got[0].Code != pw1LambdaCodeURLPublic {
		t.Fatalf("exposure findings = %+v, want exactly one, %s", got, pw1LambdaCodeURLPublic)
	}
	broken := 0
	for _, f := range res.Findings[name] {
		if f.Severity == domain.SevBroken {
			broken++
		}
	}
	if broken != 1 {
		t.Errorf("%d Broken findings on %s, want 1: one open endpoint is one issue", broken, name)
	}
}

func TestT4_Lambda_OpenURLWithoutAPublicGrantIsNotPublic(t *testing.T) {
	const name = "acme-url-ungranted"
	fake := &pw1LambdaPostureFake{
		urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{name: t4OpenURL("t4bbbb2222")},
	}
	res := pw1EnrichLambda(t, fake, name)
	if got := t4LambdaExposures(res, name); len(got) != 0 {
		t.Errorf("exposure findings = %+v, want none: no statement lets anyone call the URL", got)
	}
}

// "If you delete your function URL with auth type NONE, Lambda doesn't
// automatically delete the associated resource-based policy."
// lambda:InvokeFunctionUrl authorizes calls to the URL alone.
func TestT4_Lambda_URLGrantWithoutAURLReachesNothing(t *testing.T) {
	const name = "acme-url-deleted"
	fake := &pw1LambdaPostureFake{policies: map[string]string{name: pw1FunctionURLPolicy(name, "NONE")}}
	res := pw1EnrichLambda(t, fake, name)
	if got := t4LambdaExposures(res, name); len(got) != 0 {
		t.Errorf("exposure findings = %+v, want none: the function has no URL", got)
	}
}

func TestT4_Lambda_PublicInvokeGrantStaysWithOrWithoutAURL(t *testing.T) {
	const name = "acme-open"
	bare := pw1EnrichLambda(t, &pw1LambdaPostureFake{policies: map[string]string{name: pw1PublicInvokePolicy}}, name)
	pw1RequireFinding(t, bare.Findings[name], pw1LambdaCodePublicPolicy,
		"resource policy open to anyone", domain.SevBroken, "wave2")

	withURL := pw1EnrichLambda(t, &pw1LambdaPostureFake{
		policies:   map[string]string{name: pw1PublicInvokePolicy},
		urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{name: t4OpenURL("t4cccc3333")},
	}, name)
	pw1RequireFinding(t, withURL.Findings[name], pw1LambdaCodePublicPolicy,
		"resource policy open to anyone", domain.SevBroken, "wave2")
}
