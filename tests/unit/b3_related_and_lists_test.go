// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

// TestSecretsListIncludesPlannedDeletion pins the request flag. Without it a
// secret inside its recovery window is absent from the list, and its deleted
// state — the one an operator can still reverse — is never shown.
func TestSecretsListIncludesPlannedDeletion(t *testing.T) {
	mock := &secretsListInputRecorder{}
	if _, err := awsclient.FetchSecretsPage(context.Background(), mock, ""); err != nil {
		t.Fatalf("FetchSecretsPage: %v", err)
	}
	if !aws.ToBool(mock.last.IncludePlannedDeletion) {
		t.Error("ListSecrets was sent without IncludePlannedDeletion")
	}
}

type secretsListInputRecorder struct {
	last *secretsmanager.ListSecretsInput
}

func (m *secretsListInputRecorder) ListSecrets(
	_ context.Context,
	params *secretsmanager.ListSecretsInput,
	_ ...func(*secretsmanager.Options),
) (*secretsmanager.ListSecretsOutput, error) {
	m.last = params
	return &secretsmanager.ListSecretsOutput{}, nil
}

// TestECREbRuleNeedsTheWholeARN pins that a rule scoped to one repository
// does not answer for another whose ARN is a prefix of it.
func TestECREbRuleNeedsTheWholeARN(t *testing.T) {
	const appARN = "arn:aws:ecr:us-east-1:123456789012:repository/app"
	const workerARN = "arn:aws:ecr:us-east-1:123456789012:repository/app-worker"
	rule := resource.Resource{
		ID:   "worker-scan-rule",
		Name: "worker-scan-rule",
		RawStruct: eventbridgetypes.Rule{
			Name:         aws.String("worker-scan-rule"),
			EventPattern: aws.String(`{"source":["aws.ecr"],"resources":["` + workerARN + `"]}`),
		},
	}
	repo := resource.Resource{
		ID:   "app",
		Name: "app",
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("app"),
			RepositoryArn:  aws.String(appARN),
		},
	}
	cache := resource.ResourceCache{"eb-rule": resource.ResourceCacheEntry{Resources: []resource.Resource{rule}}}
	got := relatedCheckerByTarget(t, "ecr", "eb-rule")(context.Background(), nil, repo, cache)
	if got.Count() != 0 {
		t.Errorf("repository %q matched a rule scoped to %q: count = %d", appARN, workerARN, got.Count())
	}
}

// TestCTAccessKeyPivotCoversRoot pins that a root call carrying an access key
// pivots on it. Root can hold access keys, and a root key in use is the case
// this facet exists to trace.
func TestCTAccessKeyPivotCoversRoot(t *testing.T) {
	ctJSON := `{"eventVersion":"1.08","userIdentity":{"type":"Root","accountId":"123456789012"` +
		`,"accessKeyId":"AKIAROOTKEYEXAMPLE1"}` +
		`,"eventTime":"2026-04-07T17:00:00Z","eventSource":"s3.amazonaws.com","eventName":"GetObject"` +
		`,"awsRegion":"us-east-1","userAgent":"aws-cli/2.0","eventCategory":"Management"` +
		`,"eventType":"AwsApiCall","recipientAccountId":"123456789012"}`
	event := cloudtrailtypes.Event{
		EventId:         aws.String("root-key-01"),
		EventName:       aws.String("GetObject"),
		EventTime:       aws.Time(time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC)),
		EventSource:     aws.String("s3.amazonaws.com"),
		CloudTrailEvent: aws.String(ctJSON),
	}
	res, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage: %v", err)
	}
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("ct-events") {
		if def.DisplayName == "CT events by AccessKeyId" {
			checker = def.Checker
		}
	}
	if checker == nil {
		t.Fatal("the access-key self-pivot is not registered")
	}
	if got := checker(context.Background(), nil, res.Resources[0], nil); got.State() != domain.RelatedDeferred {
		t.Errorf("a root call carrying an access key resolved as %s, want a pivot on the key", got.State())
	}
}

// relatedCheckerByTarget returns the registered checker source declares for
// target.
func relatedCheckerByTarget(t *testing.T, source, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(source) {
		if def.TargetType == target && def.Checker != nil {
			return def.Checker
		}
	}
	t.Fatalf("%s declares no related checker for %s", source, target)
	return nil
}

// TestCTFederationRowNeedsAProvider pins that the empty webIdFederationData
// object CloudTrail puts on most assumed-role events raises no blank
// Federation row.
func TestCTFederationRowNeedsAProvider(t *testing.T) {
	ctJSON := `{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","accountId":"123456789012"` +
		`,"arn":"arn:aws:sts::123456789012:assumed-role/deploy/session"` +
		`,"sessionContext":{"sessionIssuer":{"userName":"deploy","type":"Role"},"webIdFederationData":{}}}` +
		`,"eventTime":"2026-04-07T17:00:00Z","eventSource":"s3.amazonaws.com","eventName":"GetObject"` +
		`,"awsRegion":"us-east-1","userAgent":"aws-cli/2.0","eventCategory":"Management"` +
		`,"eventType":"AwsApiCall","recipientAccountId":"123456789012"}`
	ev, err := ctevent.Parse(ctJSON)
	if err != nil {
		t.Fatalf("ctevent.Parse: %v", err)
	}
	for _, section := range ctevent.BuildSections(ev) {
		for _, row := range section.Rows {
			if row.Key == "Federation" {
				t.Errorf("an empty webIdFederationData produced a Federation row with value %q", row.Value)
			}
		}
	}
}

// TestSFNRunningExecutionIsNotOK pins that a state machine whose latest
// execution is still running does not report its last run as OK.
func TestSFNRunningExecutionIsNotOK(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:ingest"
	now := time.Now()
	fake := &sfnFakeW{results: map[string][]sfntypes.ExecutionListItem{
		arn: {{
			ExecutionArn:    aws.String(arn + ":exec-001"),
			StateMachineArn: aws.String(arn),
			Name:            aws.String("exec-001"),
			Status:          sfntypes.ExecutionStatusRunning,
			StartDate:       &now,
		}},
	}}
	rows := []resource.Resource{{ID: "ingest", Name: "ingest", Fields: map[string]string{"arn": arn}}}
	res, err := awsclient.EnrichStepFunctionsStatus(context.Background(),
		&awsclient.ServiceClients{SFN: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichStepFunctionsStatus: %v", err)
	}
	if got := res.FieldUpdates["ingest"]["last_run"]; got != "RUNNING" {
		t.Errorf("last_run = %q, want %q", got, "RUNNING")
	}
}
