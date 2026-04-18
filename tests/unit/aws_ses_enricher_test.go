package unit

// aws_ses_enricher_test.go — Behavioral tests for EnrichSESAccount.
//
// Contract assertions:
//   - GetAccount is called once (account-wide, no resource iteration).
//   - EnforcementStatus="SHUTDOWN" → 1 finding key="account", severity "!".
//   - EnforcementStatus="PROBATION" → 1 finding key="account", severity "~".
//   - SendingEnabled=false (any healthy status) → 1 finding key="account", severity "~".
//   - EnforcementStatus="HEALTHY" AND SendingEnabled=true → 0 findings.
//   - clients.SESv2 == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error → (EnricherResult{}, error propagated).

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// sesAccountFake implements SESv2API for enrichment testing.
// It embeds the interface and overrides only GetAccount.
type sesAccountFake struct {
	awsclient.SESv2API
	enforcementStatus *string
	sendingEnabled    bool
	err               error
}

func (f *sesAccountFake) GetAccount(
	_ context.Context,
	_ *sesv2.GetAccountInput,
	_ ...func(*sesv2.Options),
) (*sesv2.GetAccountOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &sesv2.GetAccountOutput{
		EnforcementStatus: f.enforcementStatus,
		SendingEnabled:    f.sendingEnabled,
	}, nil
}

// TestEnrichSESAccount_HealthySendingEnabledProducesNoFindings verifies the
// happy-path: HEALTHY status and sending enabled → zero findings.
func TestEnrichSESAccount_HealthySendingEnabledProducesNoFindings(t *testing.T) {
	fake := &sesAccountFake{
		enforcementStatus: aws.String("HEALTHY"),
		sendingEnabled:    true,
	}
	clients := &awsclient.ServiceClients{SESv2: fake}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for HEALTHY+enabled, got %d", len(result.Findings))
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichSESAccount_ShutdownProducesFindingSevBang verifies that
// EnforcementStatus="SHUTDOWN" produces a finding with key="account" and
// severity "!".
func TestEnrichSESAccount_ShutdownProducesFindingSevBang(t *testing.T) {
	fake := &sesAccountFake{
		enforcementStatus: aws.String("SHUTDOWN"),
		sendingEnabled:    true,
	}
	clients := &awsclient.ServiceClients{SESv2: fake}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["account"]
	if !ok {
		t.Fatalf("expected finding keyed by %q for SHUTDOWN status", "account")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
}

// TestEnrichSESAccount_ProbationProducesFindingSevTilde verifies that
// EnforcementStatus="PROBATION" produces a finding with key="account" and
// severity "~".
func TestEnrichSESAccount_ProbationProducesFindingSevTilde(t *testing.T) {
	fake := &sesAccountFake{
		enforcementStatus: aws.String("PROBATION"),
		sendingEnabled:    true,
	}
	clients := &awsclient.ServiceClients{SESv2: fake}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["account"]
	if !ok {
		t.Fatalf("expected finding keyed by %q for PROBATION status", "account")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
}

// TestEnrichSESAccount_SendingDisabledProducesFindingSevTilde verifies that
// SendingEnabled=false (with HEALTHY status) produces a finding with key="account"
// and severity "~".
func TestEnrichSESAccount_SendingDisabledProducesFindingSevTilde(t *testing.T) {
	fake := &sesAccountFake{
		enforcementStatus: aws.String("HEALTHY"),
		sendingEnabled:    false,
	}
	clients := &awsclient.ServiceClients{SESv2: fake}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["account"]
	if !ok {
		t.Fatalf("expected finding keyed by %q for SendingEnabled=false", "account")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
}

// TestEnrichSESAccount_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.SESv2 is nil, the enricher returns a non-nil empty Findings map and no
// error.
func TestEnrichSESAccount_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{SESv2: nil}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when SESv2 client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichSESAccount_APIErrorIsPropagated verifies that an API error from
// GetAccount is propagated as the enricher's return error.
func TestEnrichSESAccount_APIErrorIsPropagated(t *testing.T) {
	apiErr := errors.New("ses: get account failed")
	fake := &sesAccountFake{err: apiErr}
	clients := &awsclient.ServiceClients{SESv2: fake}

	_, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("error = %v, want to wrap %v", err, apiErr)
	}
}

// Compile-time check: sesAccountFake satisfies SESv2API.
var _ awsclient.SESv2API = (*sesAccountFake)(nil)
