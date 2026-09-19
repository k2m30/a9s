package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

var _ awsclient.SESv2API = (*sesEnrichmentFake)(nil)

func sesResourceRow(id, _ string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   id,
		Fields: map[string]string{"identity_name": id},
	}
}

func TestEnrichSESAccount_ShutdownFindingPerRow(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{
		sesResourceRow("acme-corp.com", ""),
		sesResourceRow("noreply@acme-corp.com", ""),
	}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Errorf("Findings count = %d, want 2 (one per row)", len(result.Findings))
	}
	for _, id := range []string{"acme-corp.com", "noreply@acme-corp.com"} {
		fs, ok := result.Findings[id]
		if !ok {
			t.Errorf("Findings missing key %q", id)
			continue
		}
		f := fs[0]
		if f.Severity != domain.SevBroken {
			t.Errorf("identity %q: Severity = %v, want SevBroken", id, f.Severity)
		}
		if f.Phrase != "sending paused by AWS (shutdown)" {
			t.Errorf("identity %q: Summary = %q, want %q", id, f.Phrase, "sending paused by AWS (shutdown)")
		}
	}
}

func TestEnrichSESAccount_ShutdownNilResources(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Findings are replicated per row; no rows means no findings.
	if len(result.Findings) != 0 {
		t.Errorf("Findings count = %d, want 0 (nil resources — nothing to replicate onto)", len(result.Findings))
	}
}

func TestEnrichSESAccount_ShutdownFindingRowActionable(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["acme-corp.com"]
	if !ok {
		t.Fatal("expected finding for identity acme-corp.com")
	}
	f := fs[0]
	ad := result.AttentionDetails["acme-corp.com"][f.Code]
	if len(ad.Rows) == 0 {
		t.Fatal("expected at least one row in SHUTDOWN finding")
	}
	row := ad.Rows[0]
	if row.Label != "Action" {
		t.Errorf("Row.Label = %q, want %q (operator-actionable context)", row.Label, "Action")
	}
	if row.Value == "" {
		t.Error("Row.Value must not be empty — carries the remediation hint")
	}
	if row.Tier != "!" {
		t.Errorf("Row.Tier = %q, want %q", row.Tier, "!")
	}
	_ = f
}

func TestEnrichSESAccount_ProbationFindingPerRow(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("PROBATION")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{
		sesResourceRow("acme-corp.com", ""),
	}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Errorf("Findings count = %d, want 1", len(result.Findings))
	}
	fs, ok := result.Findings["acme-corp.com"]
	if !ok {
		t.Fatal("expected finding keyed by identity ID")
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want %q (PROBATION is severity !)", f.Severity, "!")
	}
	if f.Phrase != "account under review (probation)" {
		t.Errorf("Summary = %q, want %q", f.Phrase, "account under review (probation)")
	}
}

func TestEnrichSESAccount_ProbationFindingRowActionable(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("PROBATION")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["acme-corp.com"]
	if !ok {
		t.Fatal("expected finding for identity acme-corp.com")
	}
	f := fs[0]
	ad := result.AttentionDetails["acme-corp.com"][f.Code]
	if len(ad.Rows) == 0 {
		t.Fatal("expected at least one row in PROBATION finding")
	}
	row := ad.Rows[0]
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

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected findings for 90% quota usage, got 0")
	}
	f := result.Findings["acme-corp.com"][0]
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want %q for quota > 80%%", f.Severity, "~")
	}
	if f.Phrase != "quota 80%+ used" {
		t.Errorf("Summary = %q, want %q", f.Phrase, "quota 80%+ used")
	}
}

// The quota threshold is strict >, not >=.
func TestEnrichSESAccount_QuotaExactly80PercentNoFinding(t *testing.T) {
	fake := &sesEnrichmentFake{
		sendQuota: &sesv2types.SendQuota{
			Max24HourSend:   10000.0,
			SentLast24Hours: 8000.0, // exactly 80% — must NOT fire
		},
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings = %d, want 0 (exactly 80%% is below strict > threshold)", len(result.Findings))
	}
}

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

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["acme-corp.com"]
	if !ok {
		t.Fatal("expected finding for PROBATION+quota case")
	}
	f := fs[0]
	if f.Phrase != "account under review (probation)" {
		t.Errorf("Summary = %q, want %q (PROBATION must win over quota)", f.Phrase, "account under review (probation)")
	}
}

// The Wave-2 phrase reaches the list view through r.Findings
// (phraseFromFindings) and the row color through colorSES, so the enricher
// writes no FieldUpdates on any path.
func TestEnrichSESAccount_FieldUpdatesNeverWritten(t *testing.T) {
	cases := []struct {
		name string
		fake *sesEnrichmentFake
		rows []resource.Resource
	}{
		{
			name: "SHUTDOWN, healthy row",
			fake: &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")},
			rows: []resource.Resource{sesResourceRow("acme-corp.com", "")},
		},
		{
			name: "SHUTDOWN, row preceded by Wave-1 status (former Bump path)",
			fake: &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")},
			rows: []resource.Resource{sesResourceRow("broken.acme-corp.com", "verification failed")},
		},
		{
			name: "PROBATION, healthy row",
			fake: &sesEnrichmentFake{enforcementStatus: aws.String("PROBATION")},
			rows: []resource.Resource{sesResourceRow("acme-corp.com", "")},
		},
		{
			name: "quota >80%, healthy row",
			fake: &sesEnrichmentFake{
				sendQuota: &sesv2types.SendQuota{Max24HourSend: 10000.0, SentLast24Hours: 9000.0},
			},
			rows: []resource.Resource{sesResourceRow("acme-corp.com", "")},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clients := &awsclient.ServiceClients{SESv2: tc.fake}
			result, err := awsclient.EnrichSESAccount(context.Background(), clients, tc.rows, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.FieldUpdates == nil {
				t.Fatal("FieldUpdates must remain non-nil for symmetry")
			}
			if len(result.FieldUpdates) != 0 {
				t.Errorf("AS-1397: FieldUpdates must be empty (the enricher must not write to it); got %d entries: %+v", len(result.FieldUpdates), result.FieldUpdates)
			}
		})
	}
}

func TestEnrichSESAccount_FieldUpdatesNonNilOnHealthyAccount(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("HEALTHY")}
	clients := &awsclient.ServiceClients{SESv2: fake}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FieldUpdates == nil {
		t.Error("FieldUpdates must not be nil on healthy-account path")
	}
}

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

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings = %d, want 0 (HEALTHY account below quota threshold)", len(result.Findings))
	}
}

func TestEnrichSESAccount_NilSESv2ReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{SESv2: nil}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, nil, nil)
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

func TestEnrichSESAccount_APIErrorPropagated(t *testing.T) {
	sentinel := errors.New("ses: get account failed")
	fake := &sesEnrichmentFake{err: sentinel}
	clients := &awsclient.ServiceClients{SESv2: fake}

	_, err := awsclient.EnrichSESAccount(context.Background(), clients, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want to wrap %v", err, sentinel)
	}
}

func TestEnrichSESAccount_ShutdownSummaryDoesNotContainRowValue(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("SHUTDOWN")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := result.Findings["acme-corp.com"][0]
	for _, row := range result.AttentionDetails["acme-corp.com"][f.Code].Rows {
		if row.Value != "" && strings.Contains(f.Phrase, row.Value) {
			t.Errorf("U11 violation: Summary %q contains Row.Value %q", f.Phrase, row.Value)
		}
	}
}

func TestEnrichSESAccount_ProbationSummaryDoesNotContainRowValue(t *testing.T) {
	fake := &sesEnrichmentFake{enforcementStatus: aws.String("PROBATION")}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := result.Findings["acme-corp.com"][0]
	for _, row := range result.AttentionDetails["acme-corp.com"][f.Code].Rows {
		if row.Value != "" && strings.Contains(f.Phrase, row.Value) {
			t.Errorf("U11 violation: Summary %q contains Row.Value %q", f.Phrase, row.Value)
		}
	}
}

func TestEnrichSESAccount_QuotaSummaryDoesNotContainSentOrMaxValues(t *testing.T) {
	fake := &sesEnrichmentFake{
		sendQuota: &sesv2types.SendQuota{
			Max24HourSend:   10000.0,
			SentLast24Hours: 9000.0,
		},
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow("acme-corp.com", "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["acme-corp.com"]
	if !ok {
		t.Fatal("expected quota finding for 90% usage")
	}
	f := fs[0]
	for _, row := range result.AttentionDetails["acme-corp.com"][f.Code].Rows {
		if row.Value != "" && strings.Contains(f.Phrase, row.Value) {
			t.Errorf("U11 violation: Summary %q contains Row.Value %q", f.Phrase, row.Value)
		}
	}
}

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
			result, err := awsclient.EnrichSESAccount(context.Background(), p.clients, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.TruncatedIDs == nil {
				t.Error("TruncatedIDs must not be nil")
			}
		})
	}
}

func TestEnrichSESAccount_FixtureHealthyAccountProducesNoFindings(t *testing.T) {
	f := fixtures.NewSESFixtures()
	defaultAccount := f.GetAccountDefault

	fake := &sesEnrichmentFake{
		enforcementStatus: defaultAccount.EnforcementStatus,
		sendQuota:         defaultAccount.SendQuota,
	}
	clients := &awsclient.ServiceClients{SESv2: fake}
	rows := []resource.Resource{sesResourceRow(fixtures.SESGraphRootIdentity, "")}

	result, err := awsclient.EnrichSESAccount(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error with fixture healthy account: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for fixture healthy account, got %d: %+v", len(result.Findings), result.Findings)
	}
}

// Without reading r.Findings, a pure Wave-2 row would render ColorHealthy
// while phraseFromFindings still shows "sending paused by AWS (shutdown)" in
// the Status column.
func TestSES_ColorReadsWave2FindingsForAccountFindings(t *testing.T) {
	td := resource.FindResourceType("ses")
	if td == nil {
		t.Fatal("ses resource type not registered")
	}

	wave2 := func(code domain.FindingCode, phrase string, sev domain.Severity) domain.Finding {
		return domain.Finding{Code: code, Phrase: phrase, Severity: sev, Source: "wave2:ses"}
	}
	wave1Failed := domain.Finding{
		Code: awsclient.CodeSESVerificationFailed, Phrase: "verification failed",
		Severity: domain.SevBroken, Source: "wave1",
	}

	cases := []struct {
		name      string
		r         resource.Resource
		wantColor resource.Color
	}{
		{
			name: "SHUTDOWN Wave-2 finding → ColorBroken",
			r: resource.Resource{
				ID: "acme-corp.com",
				Findings: []domain.Finding{
					wave2("ses.account-shutdown", "sending paused by AWS (shutdown)", domain.SevBroken),
				},
				Fields: map[string]string{
					"verification_status": "SUCCESS",
					"sending_enabled":     "true",
				},
			},
			wantColor: resource.ColorBroken,
		},
		{
			name: "PROBATION Wave-2 finding → ColorBroken",
			r: resource.Resource{
				ID: "noreply@acme-corp.com",
				Findings: []domain.Finding{
					wave2("ses.account-probation", "account under review (probation)", domain.SevBroken),
				},
				Fields: map[string]string{
					"verification_status": "SUCCESS",
					"sending_enabled":     "true",
				},
			},
			wantColor: resource.ColorBroken,
		},
		{
			// docs/resources/ses.md colors the quota finding like the rest, so a
			// Warn finding never sits on a green row.
			name: "quota 80%+ Wave-2 finding (SevWarn) → ColorWarning",
			r: resource.Resource{
				ID: "acme-corp.com",
				Findings: []domain.Finding{
					wave2("ses.quota-high", "quota 80%+ used", domain.SevWarn),
				},
				Fields: map[string]string{
					"verification_status": "SUCCESS",
					"sending_enabled":     "true",
				},
			},
			wantColor: resource.ColorWarning,
		},
		{
			// Wave-2 wins when stacked with Wave-1. To prove precedence, the
			// Wave-1 fallback (Fields["status"]="sending disabled" → ColorWarning)
			// would land on a different color than the Wave-2 SHUTDOWN finding
			// (SevBroken → ColorBroken). wantColor=ColorBroken can only be
			// satisfied by colorSES short-circuiting on the wave2:ses entry.
			name: "Wave-1 sending disabled + Wave-2 SHUTDOWN → Wave-2 ColorBroken (precedence)",
			r: resource.Resource{
				ID: "shutdown.acme-corp.com",
				Findings: []domain.Finding{
					{
						Code: awsclient.CodeSESSendingDisabled, Phrase: "sending disabled",
						Severity: domain.SevWarn, Source: "wave1",
					},
					wave2("ses.account-shutdown", "sending paused by AWS (shutdown)", domain.SevBroken),
				},
				Fields: map[string]string{
					"verification_status": "SUCCESS",
					"sending_enabled":     "false",
					"status":              "sending disabled",
				},
			},
			wantColor: resource.ColorBroken,
		},
		{
			// Wave-1 only (no Wave-2): color falls back to the SES fetcher's
			// Fields["status"] phrase set by sesTopPhrase.
			name: "Wave-1 verification failed only → Fields[status] path, ColorBroken",
			r: resource.Resource{
				ID:       "broken.acme-corp.com",
				Findings: []domain.Finding{wave1Failed},
				Fields: map[string]string{
					"verification_status": "FAILED",
					"sending_enabled":     "true",
					"status":              "verification failed",
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
				t.Errorf("ResolveColor(%q, Findings=%+v, Fields[status]=%q) = %v, want %v",
					tc.r.ID, tc.r.Findings, tc.r.Fields["status"], got, tc.wantColor)
			}
		})
	}
}

// sesEnrichmentFake embeds SESv2API as a nil interface, whose promoted
// methods panic. GetEmailIdentity answers an empty output so no DKIM posture
// enters these scenarios.
func (f *sesEnrichmentFake) GetEmailIdentity(_ context.Context, _ *sesv2.GetEmailIdentityInput, _ ...func(*sesv2.Options)) (*sesv2.GetEmailIdentityOutput, error) {
	return &sesv2.GetEmailIdentityOutput{}, nil
}
