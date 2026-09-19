package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type sqsGetQueueAttributesFake struct {
	awsclient.SQSAPI
	results  map[string]map[string]string
	errByURL map[string]error
}

func (f *sqsGetQueueAttributesFake) GetQueueAttributes(
	_ context.Context,
	in *sqs.GetQueueAttributesInput,
	_ ...func(*sqs.Options),
) (*sqs.GetQueueAttributesOutput, error) {
	url := ""
	if in != nil && in.QueueUrl != nil {
		url = *in.QueueUrl
	}
	if f.errByURL != nil {
		if err, ok := f.errByURL[url]; ok {
			return nil, err
		}
	}
	attrs := f.results[url]
	if attrs == nil {
		attrs = map[string]string{}
	}
	return &sqs.GetQueueAttributesOutput{Attributes: attrs}, nil
}

var _ awsclient.SQSAPI = (*sqsGetQueueAttributesFake)(nil)

func sqsResources(names ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(names))
	for _, name := range names {
		url := "https://sqs.us-east-1.amazonaws.com/123456789012/" + name
		res = append(res, resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"queue_name": name,
				"queue_url":  url,
				"arn":        "arn:aws:sqs:us-east-1:123456789012:" + name,
			},
		})
	}
	return res
}

func sqsURLFor(name string) string {
	return "https://sqs.us-east-1.amazonaws.com/123456789012/" + name
}

func TestEnrichSQSAttributes_BothConfiguredProducesNoFindings(t *testing.T) {
	fake := &sqsGetQueueAttributesFake{
		results: map[string]map[string]string{
			sqsURLFor("my-queue-1"): {
				"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:my-queue-1-dlq","maxReceiveCount":"5"}`,
				"KmsMasterKeyId": "alias/my-key",
			},
			sqsURLFor("my-queue-2"): {
				"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:my-queue-2-dlq","maxReceiveCount":"3"}`,
				"KmsMasterKeyId": "arn:aws:kms:us-east-1:123456789012:key/abcd-1234",
			},
		},
	}
	clients := &awsclient.ServiceClients{SQS: fake}
	resources := sqsResources("my-queue-1", "my-queue-2")

	result, err := awsclient.EnrichSQSAttributes(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

func TestEnrichSQSAttributes_MissingRedrivePolicyProducesFindingSevTilde(t *testing.T) {
	fake := &sqsGetQueueAttributesFake{
		results: map[string]map[string]string{
			sqsURLFor("my-queue-1"): {
				"KmsMasterKeyId": "alias/my-key",
			},
			sqsURLFor("my-queue-2"): {
				"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:my-queue-2-dlq","maxReceiveCount":"3"}`,
				"KmsMasterKeyId": "arn:aws:kms:us-east-1:123456789012:key/abcd-1234",
			},
		},
	}
	clients := &awsclient.ServiceClients{SQS: fake}
	resources := sqsResources("my-queue-1", "my-queue-2")

	result, err := awsclient.EnrichSQSAttributes(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["my-queue-1"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "my-queue-1")
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings["my-queue-2"]; ok {
		t.Error("my-queue-2 must NOT appear in Findings — it has both required attributes")
	}
}

func TestEnrichSQSAttributes_MissingEncryptionProducesFindingSevTilde(t *testing.T) {
	fake := &sqsGetQueueAttributesFake{
		results: map[string]map[string]string{
			sqsURLFor("my-queue-1"): {
				"RedrivePolicy": `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:my-queue-1-dlq","maxReceiveCount":"5"}`,
			},
			sqsURLFor("my-queue-2"): {
				"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:my-queue-2-dlq","maxReceiveCount":"3"}`,
				"KmsMasterKeyId": "arn:aws:kms:us-east-1:123456789012:key/abcd-1234",
			},
		},
	}
	clients := &awsclient.ServiceClients{SQS: fake}
	resources := sqsResources("my-queue-1", "my-queue-2")

	result, err := awsclient.EnrichSQSAttributes(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["my-queue-1"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "my-queue-1")
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings["my-queue-2"]; ok {
		t.Error("my-queue-2 must NOT appear in Findings — it has both required attributes")
	}
}

func TestEnrichSQSAttributes_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{SQS: nil}

	result, err := awsclient.EnrichSQSAttributes(context.Background(), clients, sqsResources("my-queue-1", "my-queue-2"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when SQS client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// sqs only emits "~" findings, so a coverage gap never lower-bounds the
// issue badge: Truncated stays false.
func TestEnrichSQSAttributes_APIErrorMarksRowTruncatedIDNotBadge(t *testing.T) {
	apiErr := errors.New("sqs: GetQueueAttributes throttled")
	fake := &sqsGetQueueAttributesFake{
		errByURL: map[string]error{
			sqsURLFor("my-queue-1"): apiErr,
		},
		results: map[string]map[string]string{
			sqsURLFor("my-queue-2"): {
				"RedrivePolicy":  `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:my-queue-2-dlq","maxReceiveCount":"3"}`,
				"KmsMasterKeyId": "arn:aws:kms:us-east-1:123456789012:key/abcd-1234",
			},
		},
	}
	clients := &awsclient.ServiceClients{SQS: fake}
	resources := sqsResources("my-queue-1", "my-queue-2")

	result, err := awsclient.EnrichSQSAttributes(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when an API call fails")
	}
	// The aggregate names the call, not the type: the type comes from the
	// registry key at the surface, and a type in the label would render it
	// twice ("enrich sqs: sqs: GetQueueAttributes ...").
	if errStr := err.Error(); !strings.Contains(errStr, "GetQueueAttributes") {
		t.Errorf("composite error must name the call, %q, got: %q", "GetQueueAttributes", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, "my-queue-1") {
		t.Errorf("composite error must contain the failing queue ID \"my-queue-1\", got: %q", errStr)
	}
	if _, ok := result.Findings["my-queue-1"]; ok {
		t.Error("my-queue-1 must NOT have a finding when the API call fails")
	}
	if result.Truncated {
		t.Error("Truncated must stay false: sqs only emits \"~\" findings, so an API error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if _, marked := result.TruncatedIDs["my-queue-1"]; !marked {
		t.Error("TruncatedIDs[\"my-queue-1\"] must be true — the GetQueueAttributes error must mark that queue's row with a \"?\" coverage gap")
	}
}
