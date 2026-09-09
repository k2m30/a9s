package unit

// aws_ec2_enricher_test.go — Behavioral tests for EnrichEC2InstanceStatus.
//
// Contract assertions:
//   - DescribeInstanceStatus is called once (all instances, no resource iteration).
//   - InstanceStatus.Status="impaired" → 1 finding keyed by InstanceId, severity "!".
//   - SystemStatus.Status="impaired" → 1 finding keyed by InstanceId, severity "!".
//   - ScheduledEvent within next 5 days → 1 finding keyed by InstanceId, severity "~".
//   - All status checks ok, no events → 0 findings.
//   - clients.EC2 == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error → (EnricherResult{}, error propagated).
//
// docs/attention-signals.md `ec2` Wave 2 mandates FOUR distinct, severity-
// stamped FindingCodes rather than one code reused across conditions:
//   - "ec2.instance-status-impaired" (SevBroken) — impaired status check.
//   - "ec2.instance-status.initializing" (SevWarn) — initializing status check.
//   - "ec2.instance-status.insufficient-data" (SevWarn) — insufficient-data status check.
//   - "ec2.scheduled-event" (SevWarn) — scheduled retirement/reboot within 7d.
//
// Current code stamps every condition under the single
// "ec2.instance-status-impaired" code (core/aws/ec2_issue_enrichment.go:20-25),
// so all four conditions share one phrase and one severity.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	ec2CodeInstanceStatusImpaired     = domain.FindingCode("ec2.instance-status-impaired")
	ec2CodeInstanceStatusInitializing = domain.FindingCode("ec2.instance-status.initializing")
	ec2CodeInstanceStatusInsufficient = domain.FindingCode("ec2.instance-status.insufficient-data")
	ec2CodeScheduledEvent             = domain.FindingCode("ec2.scheduled-event")
)

// ec2InstanceStatusFake implements EC2API for enrichment testing.
// It embeds the interface and overrides only DescribeInstanceStatus.
type ec2InstanceStatusFake struct {
	awsclient.EC2API
	statuses []ec2types.InstanceStatus
	err      error
}

func (f *ec2InstanceStatusFake) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &ec2.DescribeInstanceStatusOutput{InstanceStatuses: f.statuses}, nil
}

// Compile-time check: ec2InstanceStatusFake satisfies EC2API.
var _ awsclient.EC2API = (*ec2InstanceStatusFake)(nil)

// daysFromNow returns a *time.Time n days in the future.
func daysFromNow(n int) *time.Time {
	t := time.Now().Add(time.Duration(n) * 24 * time.Hour)
	return &t
}

// TestEnrichEC2InstanceStatus_InstanceStatusImpairedProducesFindingSevBang verifies
// that an instance with InstanceStatus.Status="impaired" produces a finding with
// severity "!" keyed by the instance ID.
func TestEnrichEC2InstanceStatus_InstanceStatusImpairedProducesFindingSevBang(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0aaaa1111bbbbb222"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0aaaa1111bbbbb222", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0aaaa1111bbbbb222"]
	if !ok {
		t.Fatalf("expected finding keyed by instance ID %q", "i-0aaaa1111bbbbb222")
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
}

// TestEnrichEC2InstanceStatus_SystemStatusImpairedProducesFindingSevBang verifies
// that an instance with SystemStatus.Status="impaired" produces a finding with
// severity "!" keyed by the instance ID.
func TestEnrichEC2InstanceStatus_SystemStatusImpairedProducesFindingSevBang(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0bbbb2222ccccc333"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0bbbb2222ccccc333", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0bbbb2222ccccc333"]
	if !ok {
		t.Fatalf("expected finding keyed by instance ID %q for impaired system status", "i-0bbbb2222ccccc333")
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
}

// TestEnrichEC2InstanceStatus_ScheduledEventSoonProducesFindingSevTilde verifies
// that a running instance with ok status checks but a scheduled event within the
// next 5 days produces a finding with severity "~".
func TestEnrichEC2InstanceStatus_ScheduledEventSoonProducesFindingSevTilde(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0cccc3333ddddd444"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				Events: []ec2types.InstanceStatusEvent{
					{
						Code:        ec2types.EventCodeSystemReboot,
						Description: aws.String("Scheduled reboot"),
						NotBefore:   daysFromNow(3),
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0cccc3333ddddd444", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0cccc3333ddddd444"]
	if !ok {
		t.Fatalf("expected finding keyed by instance ID %q for imminent scheduled event", "i-0cccc3333ddddd444")
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
}

// TestEnrichEC2InstanceStatus_HealthyInstanceProducesNoFinding verifies that a
// running instance with all status checks ok and no events produces zero findings.
func TestEnrichEC2InstanceStatus_HealthyInstanceProducesNoFinding(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0dddd4444eeeee555"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0dddd4444eeeee555", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["i-0dddd4444eeeee555"]; ok {
		t.Error("healthy instance must NOT appear in Findings")
	}
}

// TestEnrichEC2InstanceStatus_NilClientReturnsEmptyFindingsNoError verifies that
// when clients.EC2 is nil, the enricher returns a non-nil empty Findings map and
// no error.
func TestEnrichEC2InstanceStatus_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{EC2: nil}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when EC2 client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichEC2InstanceStatus_SystemStatusInitializingIsWarningNotImpaired pins
// Finding B (core/aws/ec2_issue_enrichment.go ~line 134): AWS
// "initializing" status must NOT be stamped with the "impaired" wording.
//
// docs/resources/ec2.md §4 row (line 226):
//
//	| `SystemStatus.Status == initializing` | 2 | Warning | `~` | S3, S4, S5 |
//	`initializing: checks in progress` | `Instance status checks have not yet
//	passed since start.` |
//
// Current code (ec2_issue_enrichment.go lines 87-97) stamps Tier:"!" and
// severity="!" for ANY non-"ok" status, including "initializing" — which is
// wrong per the doc row above (Warning/"~", not Broken/"!", and the summary
// text must not claim "impaired").
func TestEnrichEC2InstanceStatus_SystemStatusInitializingIsWarningNotImpaired(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0eeee5555fffff666"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusInitializing,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0eeee5555fffff666", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0eeee5555fffff666"]
	if !ok {
		t.Fatalf("expected finding keyed by instance ID %q for initializing system status", "i-0eeee5555fffff666")
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v (ec2.md line 226: initializing -> Warning, not Broken)", f.Severity, domain.SevWarn)
	}
	if strings.Contains(f.Phrase, "impaired") {
		t.Errorf("Phrase = %q must NOT contain %q — ec2.md line 226 mandates the phrase %q for initializing status", f.Phrase, "impaired", "initializing: checks in progress")
	}
	wantPhrase := "initializing: checks in progress"
	if f.Phrase != wantPhrase {
		t.Errorf("Phrase = %q, want %q (ec2.md line 226 List text (S4) column)", f.Phrase, wantPhrase)
	}
}

// TestEnrichEC2InstanceStatus_InstanceStatusInsufficientDataIsWarningNotImpaired
// pins Finding B for the "insufficient-data" status value.
//
// docs/resources/ec2.md §4 row (line 227):
//
//	| `SystemStatus.Status == insufficient-data` | 2 | Warning | `~` | S3, S4,
//	S5 | `status unknown: AWS insufficient-data` | `AWS cannot determine status
//	— insufficient data from the hypervisor.` |
//
// Current code stamps Tier:"!" / severity="!" for this status too, which is
// wrong per the doc row above.
func TestEnrichEC2InstanceStatus_InstanceStatusInsufficientDataIsWarningNotImpaired(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0ffff6666aaaaa777"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusInsufficientData,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0ffff6666aaaaa777", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0ffff6666aaaaa777"]
	if !ok {
		t.Fatalf("expected finding keyed by instance ID %q for insufficient-data instance status", "i-0ffff6666aaaaa777")
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v (ec2.md line 227: insufficient-data -> Warning, not Broken)", f.Severity, domain.SevWarn)
	}
	if strings.Contains(f.Phrase, "impaired") {
		t.Errorf("Phrase = %q must NOT contain %q — ec2.md line 227 mandates the phrase %q for insufficient-data status", f.Phrase, "impaired", "status unknown: AWS insufficient-data")
	}
	wantPhrase := "status unknown: AWS insufficient-data"
	if f.Phrase != wantPhrase {
		t.Errorf("Phrase = %q, want %q (ec2.md line 227 List text (S4) column)", f.Phrase, wantPhrase)
	}
}

// TestEnrichEC2InstanceStatus_APIErrorIsPropagated verifies that an API error from
// DescribeInstanceStatus is propagated as the enricher's return error.
func TestEnrichEC2InstanceStatus_APIErrorIsPropagated(t *testing.T) {
	apiErr := errors.New("ec2: describe instance status failed")
	fake := &ec2InstanceStatusFake{err: apiErr}
	clients := &awsclient.ServiceClients{EC2: fake}

	_, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("error = %v, want to wrap %v", err, apiErr)
	}
}

// TestEnrichEC2InstanceStatus_ImpairedUsesImpairedCode pins that an impaired
// status check keeps the "ec2.instance-status-impaired" code (SevBroken) —
// this is the one condition that keeps its historical code.
func TestEnrichEC2InstanceStatus_ImpairedUsesImpairedCode(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId:     aws.String("i-0impaired0000001"),
				SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusImpaired},
				InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0impaired0000001", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0impaired0000001"]
	if !ok || len(fs) == 0 {
		t.Fatalf("expected a finding for i-0impaired0000001")
	}
	if fs[0].Code != ec2CodeInstanceStatusImpaired {
		t.Errorf("Code = %q, want %q", fs[0].Code, ec2CodeInstanceStatusImpaired)
	}
	if fs[0].Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want %v", fs[0].Severity, domain.SevBroken)
	}
}

// TestEnrichEC2InstanceStatus_InitializingUsesDistinctCode pins that
// "initializing" gets its OWN FindingCode ("ec2.instance-status.initializing"),
// distinct from the impaired code. Today both are stamped
// "ec2.instance-status-impaired", so an initializing instance borrows the
// impaired phrase and severity.
func TestEnrichEC2InstanceStatus_InitializingUsesDistinctCode(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId:     aws.String("i-0initializing00001"),
				SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusInitializing},
				InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0initializing00001", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0initializing00001"]
	if !ok || len(fs) == 0 {
		t.Fatalf("expected a finding for i-0initializing00001")
	}
	if fs[0].Code != ec2CodeInstanceStatusInitializing {
		t.Errorf("Code = %q, want %q (must NOT reuse %q)", fs[0].Code, ec2CodeInstanceStatusInitializing, ec2CodeInstanceStatusImpaired)
	}
	if fs[0].Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want %v", fs[0].Severity, domain.SevWarn)
	}
}

// TestEnrichEC2InstanceStatus_InsufficientDataUsesDistinctCode pins that
// "insufficient-data" gets its OWN FindingCode
// ("ec2.instance-status.insufficient-data"), distinct from both the impaired
// and initializing codes.
func TestEnrichEC2InstanceStatus_InsufficientDataUsesDistinctCode(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId:     aws.String("i-0insufficient00001"),
				SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
				InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusInsufficientData},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0insufficient00001", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0insufficient00001"]
	if !ok || len(fs) == 0 {
		t.Fatalf("expected a finding for i-0insufficient00001")
	}
	if fs[0].Code != ec2CodeInstanceStatusInsufficient {
		t.Errorf("Code = %q, want %q (must NOT reuse %q)", fs[0].Code, ec2CodeInstanceStatusInsufficient, ec2CodeInstanceStatusImpaired)
	}
	if fs[0].Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want %v", fs[0].Severity, domain.SevWarn)
	}
}

// TestEnrichEC2InstanceStatus_ScheduledEventUsesDistinctCode pins that a
// scheduled retirement/reboot event on an otherwise-ok instance gets its OWN
// FindingCode ("ec2.scheduled-event"), distinct from the impaired code. Today
// the scheduled-event row is folded into whatever "ec2.instance-status-impaired"
// finding exists for the resource (or creates one under that same code if no
// status-check finding exists).
func TestEnrichEC2InstanceStatus_ScheduledEventUsesDistinctCode(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId:     aws.String("i-0scheduledevent0001"),
				SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
				InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
				Events: []ec2types.InstanceStatusEvent{
					{
						Code:        ec2types.EventCodeSystemReboot,
						Description: aws.String("Scheduled reboot"),
						NotBefore:   daysFromNow(3),
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0scheduledevent0001", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0scheduledevent0001"]
	if !ok || len(fs) == 0 {
		t.Fatalf("expected a finding for i-0scheduledevent0001")
	}
	if fs[0].Code != ec2CodeScheduledEvent {
		t.Errorf("Code = %q, want %q (must NOT reuse %q)", fs[0].Code, ec2CodeScheduledEvent, ec2CodeInstanceStatusImpaired)
	}
	if fs[0].Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want %v", fs[0].Severity, domain.SevWarn)
	}
}

// TestEnrichEC2InstanceStatus_ImpairedAndScheduledEventProduceTwoFindings pins
// that an instance with BOTH an impaired status check AND a scheduled event
// produces TWO separate findings — one per FindingCode — rather than one
// finding whose Rows merge both conditions under a single code. Findings is
// map[string][]domain.Finding (multi-finding-per-resource, see commit history
// around the "AllFindings" -> "Findings" rename), so this is a supported shape.
func TestEnrichEC2InstanceStatus_ImpairedAndScheduledEventProduceTwoFindings(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId:     aws.String("i-0impairedandevent01"),
				SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusImpaired},
				InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
				Events: []ec2types.InstanceStatusEvent{
					{
						Code:        ec2types.EventCodeSystemReboot,
						Description: aws.String("Scheduled reboot"),
						NotBefore:   daysFromNow(3),
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0impairedandevent01", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["i-0impairedandevent01"]
	if !ok {
		t.Fatalf("expected findings for i-0impairedandevent01")
	}
	if len(fs) != 2 {
		t.Fatalf("expected 2 distinct findings (impaired + scheduled-event), got %d: %v", len(fs), fs)
	}
	seen := map[domain.FindingCode]domain.Severity{}
	for _, f := range fs {
		seen[f.Code] = f.Severity
	}
	sev, ok := seen[ec2CodeInstanceStatusImpaired]
	if !ok {
		t.Errorf("missing finding with code %q; got codes %v", ec2CodeInstanceStatusImpaired, seen)
	} else if sev != domain.SevBroken {
		t.Errorf("[%s] Severity = %v, want %v", ec2CodeInstanceStatusImpaired, sev, domain.SevBroken)
	}
	sev, ok = seen[ec2CodeScheduledEvent]
	if !ok {
		t.Errorf("missing finding with code %q; got codes %v", ec2CodeScheduledEvent, seen)
	} else if sev != domain.SevWarn {
		t.Errorf("[%s] Severity = %v, want %v", ec2CodeScheduledEvent, sev, domain.SevWarn)
	}
}

// TestEC2Catalog_FindingDefsDeclareDistinctCodesAndSeverities pins the
// catalog-declaration side (core/aws/catalog_compute.go "ec2" entry's
// Findings []catalog.FindingDef): each of the four Wave 2 conditions must be
// declared under its OWN code with its real severity, so each renders its own
// phrase and its own Attention entry instead of one. Today the catalog
// declares only
// ec2CodeInstanceStatusImpaired for Wave 2.
func TestEC2Catalog_FindingDefsDeclareDistinctCodesAndSeverities(t *testing.T) {
	td := catalog.FindAny("ec2")
	if td == nil {
		t.Fatal(`catalog.FindAny("ec2") returned nil`)
	}
	want := map[domain.FindingCode]domain.Severity{
		ec2CodeInstanceStatusImpaired:     domain.SevBroken,
		ec2CodeInstanceStatusInitializing: domain.SevWarn,
		ec2CodeInstanceStatusInsufficient: domain.SevWarn,
		ec2CodeScheduledEvent:             domain.SevWarn,
	}
	got := map[domain.FindingCode]domain.Severity{}
	for _, fd := range td.Findings {
		got[fd.Code] = fd.Severity
	}
	for code, wantSev := range want {
		gotSev, ok := got[code]
		if !ok {
			t.Errorf("catalog.FindAny(%q).Findings missing declaration for code %q", "ec2", code)
			continue
		}
		if gotSev != wantSev {
			t.Errorf("catalog.FindAny(%q).Findings[%q].Severity = %v, want %v", "ec2", code, gotSev, wantSev)
		}
	}
}
