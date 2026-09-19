package unit

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
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	ec2CodeInstanceStatusImpaired     = domain.FindingCode("ec2.instance-status-impaired")
	ec2CodeInstanceStatusInitializing = domain.FindingCode("ec2.instance-status.initializing")
	ec2CodeInstanceStatusInsufficient = domain.FindingCode("ec2.instance-status.insufficient-data")
	ec2CodeScheduledEvent             = domain.FindingCode("ec2.scheduled-event")
)

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

var _ awsclient.EC2API = (*ec2InstanceStatusFake)(nil)

func daysFromNow(n int) *time.Time {
	t := time.Now().Add(time.Duration(n) * 24 * time.Hour)
	return &t
}

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

// AWS "initializing" means the status checks have not yet passed since start:
// a warning, not an impairment (docs/resources/ec2.md).
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

// AWS "insufficient-data" means it cannot determine the status: a warning,
// not an impairment (docs/resources/ec2.md).
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
		t.Errorf("Phrase = %q must NOT contain %q — ec2.md line 227 mandates the phrase %q for insufficient-data status", f.Phrase, "impaired", "status unknown: checks not reporting")
	}
	wantPhrase := "status unknown: checks not reporting"
	if f.Phrase != wantPhrase {
		t.Errorf("Phrase = %q, want %q (ec2.md line 227 List text (S4) column)", f.Phrase, wantPhrase)
	}
}

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

// Findings is map[string][]domain.Finding, so one resource carries one finding
// per FindingCode.
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
