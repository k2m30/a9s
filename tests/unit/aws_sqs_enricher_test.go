package unit

// aws_sqs_enricher_test.go — Behavioral tests for EnrichSQSAttributes.
//
// Contract assertions:
//   - GetQueueAttributes is called once per SQS resource (keyed by queue URL from resource fields).
//   - Both RedrivePolicy and KmsMasterKeyId present → 0 findings.
//   - Missing RedrivePolicy → finding for that queue, severity "~".
//   - Missing KmsMasterKeyId → finding for that queue, severity "~".
//   - clients.SQS == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for a resource → 0 findings for that resource, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sqsGetQueueAttributesFake implements SQSAPI for enrichment testing.
// It embeds the aggregate interface and overrides only GetQueueAttributes.
// The results map is keyed by QueueUrl (from the input) so the fake
// can serve different responses per resource.
type sqsGetQueueAttributesFake struct {
	awsclient.SQSAPI
	// results maps QueueUrl → attributes. If absent the fake returns errByURL.
	results map[string]map[string]string
	// errByURL maps QueueUrl → error; overrides results when set.
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

// Compile-time check: sqsGetQueueAttributesFake satisfies SQSAPI.
var _ awsclient.SQSAPI = (*sqsGetQueueAttributesFake)(nil)

// sqsResources returns a slice of SQS Resource stubs with the given names.
// The queue_url field is set to a realistic URL derived from the name.
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

// sqsURLFor returns the queue URL used by sqsResources for the given name.
func sqsURLFor(name string) string {
	return "https://sqs.us-east-1.amazonaws.com/123456789012/" + name
}

// TestEnrichSQSAttributes_BothConfiguredProducesNoFindings verifies that when
// both queues have a RedrivePolicy and KmsMasterKeyId no findings are produced.
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
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichSQSAttributes_MissingRedrivePolicyProducesFindingSevTilde verifies
// that when queue-1 has no RedrivePolicy a finding with severity "~" is produced
// for queue-1 and queue-2 (which has both attributes) produces no finding.
func TestEnrichSQSAttributes_MissingRedrivePolicyProducesFindingSevTilde(t *testing.T) {
	fake := &sqsGetQueueAttributesFake{
		results: map[string]map[string]string{
			sqsURLFor("my-queue-1"): {
				// No RedrivePolicy key
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
	f, ok := result.Findings["my-queue-1"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "my-queue-1")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings["my-queue-2"]; ok {
		t.Error("my-queue-2 must NOT appear in Findings — it has both required attributes")
	}
}

// TestEnrichSQSAttributes_MissingEncryptionProducesFindingSevTilde verifies
// that when queue-1 has no KmsMasterKeyId a finding with severity "~" is
// produced for queue-1 and queue-2 (which has both attributes) produces no finding.
func TestEnrichSQSAttributes_MissingEncryptionProducesFindingSevTilde(t *testing.T) {
	fake := &sqsGetQueueAttributesFake{
		results: map[string]map[string]string{
			sqsURLFor("my-queue-1"): {
				"RedrivePolicy": `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:my-queue-1-dlq","maxReceiveCount":"5"}`,
				// No KmsMasterKeyId key
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
	f, ok := result.Findings["my-queue-1"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "my-queue-1")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings["my-queue-2"]; ok {
		t.Error("my-queue-2 must NOT appear in Findings — it has both required attributes")
	}
}

// TestEnrichSQSAttributes_NilClientReturnsEmptyFindingsNoError verifies that
// when clients.SQS is nil the enricher returns a non-nil empty Findings map and no error.
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

// TestEnrichSQSAttributes_APIErrorSetsTruncatedAndSurfacesError verifies that when
// the GetQueueAttributes call for queue-1 returns an error, the enricher sets
// Truncated=true, produces 0 findings for the failed queue, and returns a composite
// error containing the enricher prefix and the failing queue ID.
func TestEnrichSQSAttributes_APIErrorSetsTruncatedAndSurfacesError(t *testing.T) {
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
	if errStr := err.Error(); !strings.Contains(errStr, "sqs-enrich:") {
		t.Errorf("composite error must contain \"sqs-enrich:\", got: %q", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, "my-queue-1") {
		t.Errorf("composite error must contain the failing queue ID \"my-queue-1\", got: %q", errStr)
	}
	if _, ok := result.Findings["my-queue-1"]; ok {
		t.Error("my-queue-1 must NOT have a finding when the API call fails")
	}
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
}
