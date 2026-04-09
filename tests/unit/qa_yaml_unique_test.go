package unit

// qa_yaml_unique_test.go — preserved unique assertions from qa_yaml_v220_test.go
// that go beyond the generic table-driven sweep in qa_yaml_all_test.go.
//
// Preserved tests:
//   - TestQA_YAML_CloudTrailEvent_JSONFieldRenderedAsNestedYAML
//       Source: qa_yaml_v220_test.go
//       Reason: verifies that the CloudTrailEvent JSON string field is expanded
//               into nested YAML keys rather than emitted as a raw JSON blob —
//               a specific rendering contract not covered by any other test.
//
//   - TestQA_YAML_CloudTrailEvent_NullValuesInJSON
//       Source: qa_yaml_v220_test.go
//       Reason: regression guard — null JSON values in the CT event blob must
//               not panic or produce invalid output; non-null keys must be present.

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestQA_YAML_CloudTrailEvent_JSONFieldRenderedAsNestedYAML(t *testing.T) {
	cloudTrailEventJSON := `{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE","arn":"arn:aws:sts::123456789012:assumed-role/test-role/session","accountId":"123456789012"},"eventTime":"2026-03-28T14:30:15Z","eventSource":"ec2.amazonaws.com","eventName":"RunInstances","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.1","requestParameters":{"instanceType":"t3.micro"},"responseElements":null,"readOnly":false,"eventType":"AwsApiCall"}`

	eventTime := time.Date(2026, 3, 28, 14, 30, 15, 0, time.UTC)
	event := cloudtrailtypes.Event{
		EventId:         aws.String("evt-yaml-json-0001"),
		EventName:       aws.String("RunInstances"),
		EventTime:       &eventTime,
		EventSource:     aws.String("ec2.amazonaws.com"),
		Username:        aws.String("test-user"),
		CloudTrailEvent: aws.String(cloudTrailEventJSON),
	}

	res := resource.Resource{
		ID:        "evt-yaml-json-0001",
		Name:      "RunInstances",
		RawStruct: event,
	}

	out := yamlView(t, res, 120, 40)

	// CloudTrailEvent JSON must be rendered as nested YAML keys, not a raw JSON blob.
	for _, want := range []string{
		"eventVersion",
		"userIdentity",
		"AssumedRole",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("CloudTrail Event YAML JSON field: expected %q in nested YAML output, got:\n%s", want, out)
		}
	}

	// Must NOT contain the raw JSON blob starting with {"eventVersion"
	if strings.Contains(out, `{"eventVersion"`) {
		t.Errorf("CloudTrail Event YAML JSON field: output contains raw JSON blob, expected nested YAML:\n%s", out)
	}
}

func TestQA_YAML_CloudTrailEvent_NullValuesInJSON(t *testing.T) {
	cloudTrailEventJSON := `{"requestParameters":{"bucketName":"test-bucket"},"responseElements":null}`

	eventTime := time.Date(2026, 3, 28, 14, 30, 15, 0, time.UTC)
	event := cloudtrailtypes.Event{
		EventId:         aws.String("evt-yaml-json-0002"),
		EventName:       aws.String("GetObject"),
		EventTime:       &eventTime,
		CloudTrailEvent: aws.String(cloudTrailEventJSON),
	}

	res := resource.Resource{
		ID:        "evt-yaml-json-0002",
		Name:      "GetObject",
		RawStruct: event,
	}

	out := yamlView(t, res, 120, 40)

	// requestParameters must appear as a YAML key.
	if !strings.Contains(out, "requestParameters") {
		t.Errorf("CloudTrail Event YAML null values: expected 'requestParameters' in output, got:\n%s", out)
	}

	// responseElements with null is acceptable as null, empty, or omitted — no strict assertion.
	// The test verifies it doesn't crash and that non-null content is present.
}
