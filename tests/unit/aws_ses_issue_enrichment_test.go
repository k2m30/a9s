// aws_ses_issue_enrichment_test.go — Behavioral tests for EnrichSESAccount (Wave 2).
//
// Contract assertions (current implementation):
//   - SHUTDOWN → finding per identity row, severity="!", Summary="account SHUTDOWN",
//     IssueCount=1 (counted once, not N times for N identities).
//   - PROBATION → finding per identity row, severity="!", Summary="account PROBATION",
//     IssueCount=1.
//   - quota SentLast24Hours > 0.8*Max24HourSend (strict >) → severity="~",
//     Summary="quota 80%+ used", IssueCount=0.
//   - quota == 80% exactly (8000/10000) → NO finding (strict >, not >=).
//   - Healthy (no status issues, below quota) → 0 findings, IssueCount=0.
//   - nil resources slice → no findings (nothing to replicate onto).
//   - Two resources passed → two entries in Findings map (one per row).
//   - PROBATION beats quota: PROBATION takes precedence, quota not checked.
//   - FieldUpdates: Healthy-row (Status=="") → new status = finding.Summary.
//   - FieldUpdates: Non-Healthy row (Status!="") → bumped via BumpFindingSuffix.
//   - nil clients.SESv2 → empty Findings map (non-nil), 0 IssueCount, no error.
//   - API error → error propagated.
//   - U11 invariant: Summary must NOT contain any row's Value string.
//   - TruncatedIDs is non-nil on all return paths.
//   - FieldUpdates is non-nil on all return paths.
//   - Fixture-based: NewSESFixtures().GetAccountDefault (HEALTHY, ~2.4% usage)
//     → 0 findings, 0 IssueCount.
package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sesEnrichmentFake implements awsclient.SESv2API for issue-enrichment tests.
// Overrides GetAccount; all other methods return safe stubs via embedded interface.
type sesEnrichmentFake struct {
	awsclient.SESv2API
	enforcementStatus *string
	sendQuota         *sesv2types.SendQuota
	err               error
}

func (f *sesEnrichmentFake) GetAccount(
	_ context.Context,
	_ *sesv2.GetAccountInput,
	_ ...func(*sesv2.Options),
) (*sesv2.GetAccountOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &sesv2.GetAccountOutput{
		EnforcementStatus: f.enforcementStatus,
		SendQuota:         f.sendQuota,
	}, nil
}

// Compile-time check: sesEnrichmentFake satisfies SESv2API.
var _ awsclient.SESv2API = (*sesEnrichmentFake)(nil)

// sesResourceRow returns a test identity resource for enrichment input.
func sesResourceRow(id, status string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   id,
		Status: status,
		Fields: map[string]string{"identity_name": id},
	}
}

// ---------------------------------------------------------------------------
// SHUTDOWN — severity "!", IssueCount = 1
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_ShutdownFindingPerRow verifies that SHUTDOWN produces
// one finding entry per identity row, keyed by identity ID.
func TestEnrichSESAccount_ShutdownFindingPerRow(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{
		sesResourceRow("acme-corp.com", ""),
		sesResourceRow("noreply@acme-corp.com", ""),
	}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Errorf("Findings count = %d, want 2 (one per row)", len(result.Findings))
	}
	for _, id := range []string{"acme-corp.com", "noreply@acme-corp.com"} {
		f, ok := result.Findings[id]
		if !ok {
			t.Errorf("Findings missing key %q", id)
			continue
		}
		if f.Severity != "!" {
			t.Errorf("identity %q: Severity = %q, want %q", id, f.Severity, "!")
		}
		if f.Summary != "account SHUTDOWN" {
			t.Errorf("identity %q: Summary = %q, want %q", id, f.Summary, "account SHUTDOWN")
		}
	}
}

// TestEnrichSESAccount_ShutdownIssueCountIsOneNotN verifies that IssueCount is 1
// regardless of how many identity rows are in the input (counted once per account).
func TestEnrichSESAccount_ShutdownIssueCountIsOneNotN(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	// Five identity rows — IssueCount must still be 1.
	rows := []resource.Resource{
		sesResourceRow("id1@acme-corp.com", ""),
		sesResourceRow("id2@acme-corp.com", ""),
		sesResourceRow("id3@acme-corp.com", ""),
		sesResourceRow("id4@acme-corp.com", ""),
		sesResourceRow("id5@acme-corp.com", ""),
	}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1 (counted once for account, not per identity)", result.IssueCount)
	}
}

// TestEnrichSESAccount_ShutdownNilResources verifies that when resources is nil,
// no findings are produced (nothing to replicate onto), but IssueCount is still 1.
func TestEnrichSESAccount_ShutdownNilResources(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Findings are replicated per row; no rows means no findings.
	if len(result.Findings) != 0 {
		t.Errorf("Findings count = %d, want 0 (nil resources — nothing to replicate onto)", len(result.Findings))
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1 (account-level count independent of row count)", result.IssueCount)
	}
}

// TestEnrichSESAccount_ShutdownFindingRowActionable verifies the SHUTDOWN
// finding carries an actionable Row (not a duplicate of the Summary enum).
// U11 contract requires Row.Value NOT appear as substring of Summary.
func TestEnrichSESAccount_ShutdownFindingRowActionable(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["acme-corp.com"]
	if !ok {
		t.Fatal("expected finding for identity acme-corp.com")
	}
	if len(f.Rows) == 0 {
		t.Fatal("expected at least one row in SHUTDOWN finding")
	}
	row := f.Rows[0]
	if row.Label != "Action" {
		t.Errorf("Row.Label = %q, want %q (operator-actionable context)", row.Label, "Action")
	}
	if row.Value == "" {
		t.Error("Row.Value must not be empty — carries the remediation hint")
	}
	if row.Tier != "!" {
		t.Errorf("Row.Tier = %q, want %q", row.Tier, "!")
	}
}

// ---------------------------------------------------------------------------
// PROBATION — severity "!", IssueCount = 1
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_ProbationFindingPerRow verifies that PROBATION produces
// one finding entry per identity row with severity "!".
func TestEnrichSESAccount_ProbationFindingPerRow(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("PROBATION")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{
		sesResourceRow("acme-corp.com", ""),
	}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Errorf("Findings count = %d, want 1", len(result.Findings))
	}
	f, ok := result.Findings["acme-corp.com"]
	if !ok {
		t.Fatal("expected finding keyed by identity ID")
	}
	if f.Severity != "!" {
		t.Errorf("Severity = %q, want %q (PROBATION is severity !)", f.Severity, "!")
	}
	if f.Summary != "account PROBATION" {
		t.Errorf("Summary = %q, want %q", f.Summary, "account PROBATION")
	}
}

// TestEnrichSESAccount_ProbationIssueCountIsOne verifies IssueCount=1 for PROBATION.
func TestEnrichSESAccount_ProbationIssueCountIsOne(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("PROBATION")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1 (PROBATION is severity !, counted once)", result.IssueCount)
	}
}

// TestEnrichSESAccount_ProbationFindingRowActionable verifies the PROBATION
// finding carries an actionable Row. U11 contract forbids Row.Value duplicating
// the Summary enum.
func TestEnrichSESAccount_ProbationFindingRowActionable(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("PROBATION")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := result.Findings["acme-corp.com"]
	if len(f.Rows) == 0 {
		t.Fatal("expected at least one row in PROBATION finding")
	}
	row := f.Rows[0]
	if row.Label != "Action" {
		t.Errorf("Row.Label = %q, want %q (operator-actionable context)", row.Label, "Action")
	}
	if row.Value == "" {
		t.Error("Row.Value must not be empty — carries the remediation hint")
	}
	if row.Tier != "!" {
		t.Errorf("Row.Tier = %q, want %q", row.Tier, "!")
	}
}

// ---------------------------------------------------------------------------
// Quota > 80% — severity "~", IssueCount = 0
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_QuotaOver80PercentProducesTildeFindings verifies that
// SentLast24Hours > 80% of Max24HourSend (strict) produces severity "~" findings.
func TestEnrichSESAccount_QuotaOver80PercentProducesTildeFindings(t *testing.T) {
	fake := &sesEnrichmentFake{
		enforcementStatus: aws.String("HEALTHY"),
		sendQuota: &sesv2types.SendQuota{
			Max24HourSend:   10000.0,
			SentLast24Hours: 9000.0, // 90% — above 80%
			MaxSendRate:     14.0,
		},
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected findings for 90% quota usage, got 0")
	}
	f := result.Findings["acme-corp.com"]
	if f.Severity != "~" {
		t.Errorf("Severity = %q, want %q for quota > 80%%", f.Severity, "~")
	}
	if f.Summary != "quota 80%+ used" {
		t.Errorf("Summary = %q, want %q", f.Summary, "quota 80%+ used")
	}
}

// TestEnrichSESAccount_QuotaOver80PercentIssueCountIsZero verifies that quota
// findings (severity "~") do NOT increment IssueCount.
func TestEnrichSESAccount_QuotaOver80PercentIssueCountIsZero(t *testing.T) {
	fake := &sesEnrichmentFake{
		sendQuota: &sesv2types.SendQuota{
			Max24HourSend:   10000.0,
			SentLast24Hours: 8500.0, // 85%
		},
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (quota severity ~ excluded from badge counter)", result.IssueCount)
	}
}

// TestEnrichSESAccount_QuotaExactly80PercentNoFinding verifies that
// SentLast24Hours == 80% of Max24HourSend does NOT produce a finding.
// The threshold is strict >, not >=.
func TestEnrichSESAccount_QuotaExactly80PercentNoFinding(t *testing.T) {
	fake := &sesEnrichmentFake{
		sendQuota: &sesv2types.SendQuota{
			Max24HourSend:   10000.0,
			SentLast24Hours: 8000.0, // exactly 80% — must NOT fire
		},
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings = %d, want 0 (exactly 80%% is below strict > threshold)", len(result.Findings))
	}
}

// ---------------------------------------------------------------------------
// PROBATION beats quota (precedence)
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_ProbationBeatsQuota verifies that PROBATION takes the
// finding slot — the quota check is NOT reached when enforcement status matches.
func TestEnrichSESAccount_ProbationBeatsQuota(t *testing.T) {
	fake := &sesEnrichmentFake{
		enforcementStatus: aws.String("PROBATION"),
		sendQuota: &sesv2types.SendQuota{
			Max24HourSend:   10000.0,
			SentLast24Hours: 9999.0, // 99.99% — would fire, but PROBATION wins
		},
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["acme-corp.com"]
	if !ok {
		t.Fatal("expected finding for PROBATION+quota case")
	}
	if f.Summary != "account PROBATION" {
		t.Errorf("Summary = %q, want %q (PROBATION must win over quota)", f.Summary, "account PROBATION")
	}
}

// ---------------------------------------------------------------------------
// FieldUpdates — Healthy row and non-Healthy row
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_FieldUpdatesHealthyRowSetToSummary verifies that a row
// with Status=="" gets FieldUpdates["status"] = finding.Summary.
func TestEnrichSESAccount_FieldUpdatesHealthyRowSetToSummary(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")} // Status = ""

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FieldUpdates == nil {
		t.Fatal("FieldUpdates must not be nil")
	}
	updates, ok := result.FieldUpdates["acme-corp.com"]
	if !ok {
		t.Fatal("expected FieldUpdates entry for acme-corp.com")
	}
	if updates["status"] != "account SHUTDOWN" {
		t.Errorf("FieldUpdates[status] = %q, want %q", updates["status"], "account SHUTDOWN")
	}
}

// TestEnrichSESAccount_FieldUpdatesNonHealthyRowBumped verifies that a row
// with an existing Wave-1 Status gets the suffix bumped via BumpFindingSuffix.
func TestEnrichSESAccount_FieldUpdatesNonHealthyRowBumped(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	// Row already has a Wave-1 status phrase.
	rows := []resource.Resource{sesResourceRow("broken.acme-corp.com", "verification failed")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updates := result.FieldUpdates["broken.acme-corp.com"]
	newStatus := updates["status"]
	// BumpFindingSuffix on "verification failed" → "verification failed (+1)"
	expected := "verification failed (+1)"
	if newStatus != expected {
		t.Errorf("FieldUpdates[status] = %q, want %q (Wave-1 status must be bumped)", newStatus, expected)
	}
}

// TestEnrichSESAccount_FieldUpdatesNonNilOnHealthyAccount verifies that
// FieldUpdates is non-nil even when no findings are produced.
func TestEnrichSESAccount_FieldUpdatesNonNilOnHealthyAccount(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("HEALTHY")}
	clients := &awsclient.ServiceClients{SESv2: fake}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FieldUpdates == nil {
		t.Error("FieldUpdates must not be nil on healthy-account path")
	}
}

// ---------------------------------------------------------------------------
// Healthy account — 0 findings
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_HealthyNoQuotaProducesNoFindings verifies that a HEALTHY
// account with quota well below 80% produces 0 findings and IssueCount=0.
func TestEnrichSESAccount_HealthyNoQuotaProducesNoFindings(t *testing.T) {
	fake := &sesEnrichmentFake{
		enforcementStatus: aws.String("HEALTHY"),
		sendQuota: &sesv2types.SendQuota{
			Max24HourSend:   50000.0,
			SentLast24Hours: 100.0, // 0.2%
		},
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings = %d, want 0 (HEALTHY account below quota threshold)", len(result.Findings))
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// ---------------------------------------------------------------------------
// nil client path
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_NilSESv2ReturnsEmptyFindingsNoError verifies that a nil
// SESv2 client returns non-nil empty Findings, non-nil FieldUpdates, and no error.
func TestEnrichSESAccount_NilSESv2ReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{SESv2: nil}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings count = %d, want 0", len(result.Findings))
	}
	if result.FieldUpdates == nil {
		t.Error("FieldUpdates must not be nil")
	}
}

// ---------------------------------------------------------------------------
// API error propagation
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_APIErrorPropagated verifies that a GetAccount API error
// is propagated as the enricher's return error.
func TestEnrichSESAccount_APIErrorPropagated(t *testing.T) {
	sentinel := errors.New("ses: get account failed")
	fake := &sesEnrichmentFake{err: sentinel}
	clients := &awsclient.ServiceClients{SESv2: fake}

	_, err := awsclient.EnrichSESAccount(context.Background(), clients, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want to wrap %v", err, sentinel)
	}
}

// ---------------------------------------------------------------------------
// U11 invariant: Summary must NOT contain row Value
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_ShutdownSummaryDoesNotContainRowValue verifies U11:
// Summary must not embed content already present in Rows.
func TestEnrichSESAccount_ShutdownSummaryDoesNotContainRowValue(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := result.Findings["acme-corp.com"]
	for _, row := range f.Rows {
		if row.Value != "" && strings.Contains(f.Summary, row.Value) {
			t.Errorf("U11 violation: Summary %q contains Row.Value %q", f.Summary, row.Value)
		}
	}
}

// TestEnrichSESAccount_ProbationSummaryDoesNotContainRowValue verifies U11 for PROBATION.
func TestEnrichSESAccount_ProbationSummaryDoesNotContainRowValue(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("PROBATION")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := result.Findings["acme-corp.com"]
	for _, row := range f.Rows {
		if row.Value != "" && strings.Contains(f.Summary, row.Value) {
			t.Errorf("U11 violation: Summary %q contains Row.Value %q", f.Summary, row.Value)
		}
	}
}

// TestEnrichSESAccount_QuotaSummaryDoesNotContainSentOrMaxValues verifies U11
// for the quota finding (Row Values are numeric strings, not in Summary).
func TestEnrichSESAccount_QuotaSummaryDoesNotContainSentOrMaxValues(t *testing.T) {
	fake := &sesEnrichmentFake{
		sendQuota: &sesv2types.SendQuota{
			Max24HourSend:   10000.0,
			SentLast24Hours: 9000.0,
		},
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["acme-corp.com"]
	if !ok {
		t.Fatal("expected quota finding for 90% usage")
	}
	for _, row := range f.Rows {
		if row.Value != "" && strings.Contains(f.Summary, row.Value) {
			t.Errorf("U11 violation: Summary %q contains Row.Value %q", f.Summary, row.Value)
		}
	}
}

// ---------------------------------------------------------------------------
// TruncatedIDs — non-nil on all paths
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_TruncatedIDsNonNilOnAllPaths verifies TruncatedIDs is
// non-nil for SHUTDOWN, HEALTHY, and nil-client paths.
func TestEnrichSESAccount_TruncatedIDsNonNilOnAllPaths(t *testing.T) {
	paths := []struct {
		name    string
		clients *awsclient.ServiceClients
	}{
		{"nil-client", &awsclient.ServiceClients{SESv2: nil}},
		{"SHUTDOWN", &awsclient.ServiceClients{SESv2: &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}}},
		{"HEALTHY", &awsclient.ServiceClients{SESv2: &sesEnrichmentFake{enforcementStatus: aws.String("HEALTHY")}}},
	}
	for _, p := range paths {
		p := p
		t.Run(p.name, func(t *testing.T) {
			result, err := awsclient.EnrichSESAccount(context.Background(), p.clients, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.TruncatedIDs == nil {
				t.Error("TruncatedIDs must not be nil")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Fixture-based: healthy demo account produces no findings
// ---------------------------------------------------------------------------

// TestEnrichSESAccount_FixtureHealthyAccountProducesNoFindings verifies that the
// canonical demo fixture account (HEALTHY, ~2.4% quota usage) produces 0 findings.
func TestEnrichSESAccount_FixtureHealthyAccountProducesNoFindings(t *testing.T) {
	f := fixtures.NewSESFixtures()
	defaultAccount := f.GetAccountDefault

	fake := &sesEnrichmentFake{
		enforcementStatus: defaultAccount.EnforcementStatus,
		sendQuota:         defaultAccount.SendQuota,
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	// Pass a row representing the graph-root identity.
	rows := []resource.Resource{sesResourceRow(fixtures.SESGraphRootIdentity, "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows)
	if err != nil {
		t.Fatalf("unexpected error with fixture healthy account: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for fixture healthy account, got %d: %+v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 for fixture healthy account", result.IssueCount)
	}
}

// ---------------------------------------------------------------------------
// Target #1 — SES Color func must fall back to Fields["status"] when Status=""
// ---------------------------------------------------------------------------

// TestSES_ColorReadsFieldsStatusFallback_AccountSHUTDOWN verifies that the SES
// Color function returns ColorBroken when r.Status is empty but Fields["status"]
// carries the Wave-2 phrase "account SHUTDOWN".
//
// Regression pin: ApplyFieldUpdates writes to Fields["status"], not r.Status.
// Before fix: Color checks r.Status only → stays green (ColorHealthy).
// After fix:  Color falls back to Fields["status"] when r.Status is empty.
func TestSES_ColorReadsFieldsStatusFallback_AccountSHUTDOWN(t *testing.T) {
	td := resource.FindResourceType("ses")
	if td == nil {
		t.Fatal("ses resource type not registered")
	}

	cases := []struct {
		name      string
		r         resource.Resource
		wantColor resource.Color
	}{
		{
			name: "SHUTDOWN in Fields[status] with empty Status → ColorBroken",
			r: resource.Resource{
				ID:     "acme-corp.com",
				Status: "",
				Fields: map[string]string{
					"verification_status": "SUCCESS",
					"sending_enabled":     "true",
					"status":              "account SHUTDOWN",
				},
			},
			wantColor: resource.ColorBroken,
		},
		{
			name: "PROBATION in Fields[status] with empty Status → ColorBroken",
			r: resource.Resource{
				ID:     "noreply@acme-corp.com",
				Status: "",
				Fields: map[string]string{
					"verification_status": "SUCCESS",
					"sending_enabled":     "true",
					"status":              "account PROBATION",
				},
			},
			wantColor: resource.ColorBroken,
		},
		{
			name: "quota 80%+ used in Fields[status] → ColorHealthy (informational, stays green)",
			r: resource.Resource{
				ID:     "acme-corp.com",
				Status: "",
				Fields: map[string]string{
					"verification_status": "SUCCESS",
					"sending_enabled":     "true",
					"status":              "quota 80%+ used",
				},
			},
			wantColor: resource.ColorHealthy,
		},
		{
			// Wave-1 precedence guard: when Status is non-empty, Fields["status"]
			// fallback must NOT override — Wave-1 status wins.
			name: "non-empty Status with SHUTDOWN in Fields[status] → Wave-1 wins (ColorBroken from Status)",
			r: resource.Resource{
				ID:     "broken.acme-corp.com",
				Status: "verification failed",
				Fields: map[string]string{
					"verification_status": "FAILED",
					"sending_enabled":     "true",
					"status":              "account SHUTDOWN",
				},
			},
			wantColor: resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := td.ResolveColor(tc.r)
			if got != tc.wantColor {
				t.Errorf("ResolveColor(%q, Status=%q, Fields[status]=%q) = %v, want %v",
					tc.r.ID, tc.r.Status, tc.r.Fields["status"], got, tc.wantColor)
			}
		})
	}
}
