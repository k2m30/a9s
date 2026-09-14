// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// detail_redacts_credential_test.go — a value the secret scanner flags never
// reaches the detail screen in the clear. The scanner's own rows name only
// where a credential sits; the resource's configuration dump beside them
// (a Lambda's environment) showed the value itself.

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/secretscan"
)

const flaggedSecret = "example-plaintext-credential-1234"

// A quoted section keeps AWS's wording, not its secrets: a credential in a
// CloudTrail event's request block is masked like one anywhere else.
func TestDetail_RedactsTheValueInARawEvent(t *testing.T) {
	raw := `{
		"eventVersion": "1.08",
		"eventName": "UpdateFunctionConfiguration20150331v2",
		"eventSource": "lambda.amazonaws.com",
		"requestParameters": {
			"functionName": "payment-webhook",
			"environment": {"variables": {"STRIPE_SECRET_KEY": "` + flaggedSecret + `", "REGION": "us-east-1"}}
		},
		"eventID": "e-d4e5f6a7-test",
		"eventType": "AwsApiCall",
		"recipientAccountId": "123456789012"
	}`
	res := resource.Resource{
		ID: "e-d4e5f6a7-test", Name: "e-d4e5f6a7-test",
		RawStruct: cloudtrailtypes.Event{
			EventId:         aws.String("e-d4e5f6a7-test"),
			EventName:       aws.String("UpdateFunctionConfiguration20150331v2"),
			EventSource:     aws.String("lambda.amazonaws.com"),
			CloudTrailEvent: aws.String(raw),
		},
	}
	c := newVisibilityDetailController(t)
	c.EnsureDetailState(res, "ct-events")
	assertDetailRedacts(t, c, flaggedSecret)
}

func TestDetail_RedactsTheValueTheScannerFlagged(t *testing.T) {
	const secret = flaggedSecret
	fn := lambdatypes.FunctionConfiguration{
		FunctionName: aws.String("payment-webhook"),
		FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:payment-webhook"),
		Runtime:      lambdatypes.RuntimeNodejs20x,
		Environment: &lambdatypes.EnvironmentResponse{Variables: map[string]string{
			"STRIPE_SECRET_KEY": secret,
			"REGION":            "us-east-1",
		}},
	}
	res := resource.Resource{
		ID: "payment-webhook", Name: "payment-webhook", Type: "lambda",
		Fields:    map[string]string{"name": "payment-webhook"},
		RawStruct: fn,
	}

	c := newVisibilityDetailController(t)
	c.EnsureDetailState(res, "lambda")
	assertDetailRedacts(t, c, secret)
}

// assertDetailRedacts reads the detail on top of the stack: the secret is
// absent, its redacted form present, and a value the scanner did not flag
// (the region) still shown.
func assertDetailRedacts(t *testing.T, c *app.Controller, secret string) {
	t.Helper()
	detail := c.Snapshot().Body.Detail
	if detail == nil {
		t.Fatal("no detail body after EnsureDetailState")
	}
	var lines []string
	sawRedacted, sawRegion := false, false
	for _, f := range detail.Fields {
		line := f.Key + "=" + f.Value
		lines = append(lines, line)
		if strings.Contains(line, secret) {
			t.Errorf("the detail prints the flagged value in the clear: %q", line)
		}
		if strings.Contains(line, secretscan.Redact(secret)) {
			sawRedacted = true
		}
		if strings.Contains(line, "us-east-1") {
			sawRegion = true
		}
	}
	if !sawRedacted {
		t.Errorf("the detail carries no redacted form %q of the flagged value; fields were:\n%s", secretscan.Redact(secret), strings.Join(lines, "\n"))
	}
	if !sawRegion {
		t.Errorf("a value the scanner did not flag was hidden too; fields were:\n%s", strings.Join(lines, "\n"))
	}
}
