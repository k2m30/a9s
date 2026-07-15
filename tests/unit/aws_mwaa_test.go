package unit

// aws_mwaa_test.go — fetcher tests for FetchMWAAEnvironmentsPage
// (docs/resources/mwaa.md §3/§4, docs/resources/mwaa-impl-plan.md §0/§1).
//
// MWAA has no issue/detail enricher: ListEnvironments returns names only, so
// every §3.2 signal and every related-panel field comes from the same
// GetEnvironment N+1 call (the eks pattern) and is emitted fetcher-side with
// Source "wave1". These tests exercise that fetcher directly against the
// shared demo fixtures plus inline adversarial fakes.

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/mwaa"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// fetchMWAADemoPage fetches the shared demo fixture page. The demo set
// includes the details-denied witness, so the fetch is a designed E5 partial
// success (rows + composite error naming only that fixture); any OTHER error
// fails the test.
func fetchMWAADemoPage(t *testing.T) resource.FetchResult {
	t.Helper()
	clients := &awsclient.ServiceClients{MWAA: fakes.NewMWAA()}
	result, err := awsclient.FetchMWAAEnvironmentsPage(context.Background(), clients, "")
	if err != nil && !strings.Contains(err.Error(), fixtures.WarnAirflowDetailsDeniedID) {
		t.Fatalf("expected only the details-denied composite error, got %v", err)
	}
	return result
}

// mustFindMWAAResource returns the resource with the given ID from a
// FetchMWAAEnvironmentsPage result, failing the test if absent.
func mustFindMWAAResource(t *testing.T, resources []domain.Resource, id string) domain.Resource {
	t.Helper()
	for _, r := range resources {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("resource %q not found in fetch result", id)
	return domain.Resource{}
}

// ---------------------------------------------------------------------------
// U1 — healthy AVAILABLE silence + Silence bullet
// ---------------------------------------------------------------------------

func TestFetchMWAAEnvironmentsPage_HealthyAvailableSilence(t *testing.T) {
	result := fetchMWAADemoPage(t)

	reporting := mustFindMWAAResource(t, result.Resources, fixtures.ProdAirflowReportingID)
	if len(reporting.Findings) != 0 {
		t.Errorf("Findings: expected 0 for healthy AVAILABLE environment, got %d: %+v",
			len(reporting.Findings), reporting.Findings)
	}
}

// ---------------------------------------------------------------------------
// U3 — LastUpdate.Status == FAILED on an AVAILABLE row (background finding)
// U11 (adapted) — the short S4 Phrase must never echo the raw ErrorMessage
// ---------------------------------------------------------------------------

func TestFetchMWAAEnvironmentsPage_LastUpdateFailedFinding(t *testing.T) {
	result := fetchMWAADemoPage(t)
	r := mustFindMWAAResource(t, result.Resources, fixtures.WarnAirflowStaleUpdateID)

	var finding *domain.Finding
	for i := range r.Findings {
		if r.Findings[i].Phrase == "last update failed" {
			finding = &r.Findings[i]
			break
		}
	}
	if finding == nil {
		t.Fatalf("no %q finding in Findings: %+v", "last update failed", r.Findings)
	}
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}

	// Error Code rows are humanized (domain.HumanizeStatusPhrase) — the raw
	// AWS enum "INCORRECT_CONFIGURATION" is never rendered verbatim.
	const errorCode = "incorrect configuration"
	const errorMessage = "Scheduler failed to launch: requirements.txt install failed"

	if strings.Contains(finding.Phrase, errorMessage) {
		t.Errorf("Phrase %q must not contain the raw ErrorMessage text %q — S4 stays short, "+
			"the full sentence belongs in S5/AttentionDetails", finding.Phrase, errorMessage)
	}

	ad, ok := r.AttentionDetails[finding.Code]
	if !ok {
		t.Fatalf("AttentionDetails[%v] not found", finding.Code)
	}
	var foundCode, foundMessage bool
	for _, row := range ad.Rows {
		if row.Value == errorCode {
			foundCode = true
		}
		if row.Value == errorMessage {
			foundMessage = true
		}
	}
	if !foundCode {
		t.Errorf("AttentionDetails rows missing Error Code %q: %+v", errorCode, ad.Rows)
	}
	if !foundMessage {
		t.Errorf("AttentionDetails rows missing Error Message %q: %+v", errorMessage, ad.Rows)
	}
}

// ---------------------------------------------------------------------------
// U3 — WebserverAccessMode public (background finding)
// ---------------------------------------------------------------------------

func TestFetchMWAAEnvironmentsPage_WebserverPublicFinding(t *testing.T) {
	result := fetchMWAADemoPage(t)
	r := mustFindMWAAResource(t, result.Resources, fixtures.WarnAirflowPublicID)

	var finding *domain.Finding
	for i := range r.Findings {
		if r.Findings[i].Phrase == "webserver public" {
			finding = &r.Findings[i]
			break
		}
	}
	if finding == nil {
		t.Fatalf("no %q finding in Findings: %+v", "webserver public", r.Findings)
	}
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}
	if !strings.Contains(finding.Detail, "reachable from the internet") {
		t.Errorf("Detail = %q, want it to contain %q", finding.Detail, "reachable from the internet")
	}
	// The WebserverAccessMode enum is humanized (domain.HumanizeStatusPhrase),
	// not embedded verbatim as "PUBLIC_ONLY".
	if !strings.Contains(finding.Detail, "access mode: public only") {
		t.Errorf("Detail = %q, want it to contain the humanized access mode %q",
			finding.Detail, "access mode: public only")
	}
}

// ---------------------------------------------------------------------------
// U7a analog — both background findings on the same green row, in §4 order
// ---------------------------------------------------------------------------

func TestFetchMWAAEnvironmentsPage_MultiFindingsOrderedOnGreenRow(t *testing.T) {
	result := fetchMWAADemoPage(t)
	r := mustFindMWAAResource(t, result.Resources, fixtures.WarnAirflowMultiID)

	got := make([]string, len(r.Findings))
	for i, f := range r.Findings {
		got[i] = f.Phrase
	}
	want := []string{"last update failed", "webserver public"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ordered Findings phrases = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// U7b/U7c analog — a finding on an already non-green row must stack, not
// disappear; AttentionDetails still carries the failed-update Error rows.
// ---------------------------------------------------------------------------

func TestFetchMWAAEnvironmentsPage_FindingStacksOnNonGreenRow(t *testing.T) {
	result := fetchMWAADemoPage(t)

	cases := []struct {
		id               string
		wantPhrases      []string
		wantErrorMessage string
	}{
		{fixtures.WarnAirflowRollbackID, []string{"rolling back: update failed", "last update failed"},
			"Update to 2.11.0 failed; restoring metadata snapshot"},
		{fixtures.BrokenAirflowUpdateFailedID, []string{"update failed: rolled back", "last update failed"},
			"Environment update failed; rolled back to previous state"},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			r := mustFindMWAAResource(t, result.Resources, tc.id)

			got := make([]string, len(r.Findings))
			for i, f := range r.Findings {
				got[i] = f.Phrase
			}
			if !reflect.DeepEqual(got, tc.wantPhrases) {
				t.Errorf("ordered Findings phrases = %v, want %v — a finding on a non-green row must not silently disappear",
					got, tc.wantPhrases)
			}

			var lastUpdateCode domain.FindingCode
			for _, f := range r.Findings {
				if f.Phrase == "last update failed" {
					lastUpdateCode = f.Code
					break
				}
			}
			ad, ok := r.AttentionDetails[lastUpdateCode]
			if !ok || len(ad.Rows) == 0 {
				t.Fatalf("AttentionDetails[%v] missing or empty for the stacked failed-update finding", lastUpdateCode)
			}
			var found bool
			for _, row := range ad.Rows {
				if row.Value == tc.wantErrorMessage {
					found = true
				}
			}
			if !found {
				t.Errorf("AttentionDetails rows missing ErrorMessage %q: %+v", tc.wantErrorMessage, ad.Rows)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// status_mapping_table_unit — all 12 EnvironmentStatus enum values, incl.
// DELETED (no demo fixture: a deleted environment never appears in
// ListEnvironments in practice).
// ---------------------------------------------------------------------------

// mwaaStatusOnlyEnvironment builds a minimal-but-complete Environment for the
// given status — used only to exercise the status→bucket mapping. LastUpdate
// is always SUCCESS and WebserverAccessMode is always PRIVATE_ONLY so no
// background finding leaks into the single-finding assertions below.
func mwaaStatusOnlyEnvironment(name string, status mwaatypes.EnvironmentStatus) mwaatypes.Environment {
	return mwaatypes.Environment{
		Name:                 aws.String(name),
		Arn:                  aws.String("arn:aws:airflow:us-east-1:123456789012:environment/" + name),
		Status:               status,
		AirflowVersion:       aws.String("2.10.3"),
		EnvironmentClass:     aws.String("mw1.small"),
		WebserverAccessMode:  mwaatypes.WebserverAccessModePrivateOnly,
		LastUpdate:           &mwaatypes.LastUpdate{Status: mwaatypes.UpdateStatusSuccess},
		LoggingConfiguration: &mwaatypes.LoggingConfiguration{},
		NetworkConfiguration: &mwaatypes.NetworkConfiguration{},
	}
}

// mwaaStatusOnlyFake implements awsclient.MWAAAPI over a name->Environment map.
type mwaaStatusOnlyFake struct {
	envs map[string]mwaatypes.Environment
}

func (f *mwaaStatusOnlyFake) ListEnvironments(
	_ context.Context, _ *mwaa.ListEnvironmentsInput, _ ...func(*mwaa.Options),
) (*mwaa.ListEnvironmentsOutput, error) {
	names := make([]string, 0, len(f.envs))
	for name := range f.envs {
		names = append(names, name)
	}
	return &mwaa.ListEnvironmentsOutput{Environments: names}, nil
}

func (f *mwaaStatusOnlyFake) GetEnvironment(
	_ context.Context, input *mwaa.GetEnvironmentInput, _ ...func(*mwaa.Options),
) (*mwaa.GetEnvironmentOutput, error) {
	name := aws.ToString(input.Name)
	env, ok := f.envs[name]
	if !ok {
		return nil, fmt.Errorf("environment %q not found", name)
	}
	return &mwaa.GetEnvironmentOutput{Environment: &env}, nil
}

var _ awsclient.MWAAAPI = (*mwaaStatusOnlyFake)(nil)

func TestFetchMWAAEnvironmentsPage_StatusMappingAllTwelveValues(t *testing.T) {
	cases := []struct {
		status   mwaatypes.EnvironmentStatus
		phrase   string // "" means healthy: no finding at all
		severity domain.Severity
	}{
		{mwaatypes.EnvironmentStatusAvailable, "", domain.SevOK},
		{mwaatypes.EnvironmentStatusCreating, "creating", domain.SevWarn},
		{mwaatypes.EnvironmentStatusCreatingSnapshot, "creating snapshot", domain.SevWarn},
		{mwaatypes.EnvironmentStatusPending, "pending: awaiting VPC endpoints", domain.SevWarn},
		{mwaatypes.EnvironmentStatusUpdating, "updating", domain.SevWarn},
		{mwaatypes.EnvironmentStatusRollingBack, "rolling back: update failed", domain.SevWarn},
		{mwaatypes.EnvironmentStatusMaintenance, "maintenance in progress", domain.SevWarn},
		{mwaatypes.EnvironmentStatusCreateFailed, "create failed", domain.SevBroken},
		{mwaatypes.EnvironmentStatusUpdateFailed, "update failed: rolled back", domain.SevBroken},
		{mwaatypes.EnvironmentStatusUnavailable, "unavailable: not stable", domain.SevBroken},
		{mwaatypes.EnvironmentStatusDeleting, "deleting", domain.SevDim},
		{mwaatypes.EnvironmentStatusDeleted, "deleted", domain.SevDim},
	}

	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			name := "status-test-" + strings.ToLower(string(tc.status))
			fake := &mwaaStatusOnlyFake{envs: map[string]mwaatypes.Environment{
				name: mwaaStatusOnlyEnvironment(name, tc.status),
			}}
			clients := &awsclient.ServiceClients{MWAA: fake}
			result, err := awsclient.FetchMWAAEnvironmentsPage(context.Background(), clients, "")
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			r := mustFindMWAAResource(t, result.Resources, name)

			if tc.phrase == "" {
				if len(r.Findings) != 0 {
					t.Errorf("Findings: expected 0 for %s, got %d: %+v", tc.status, len(r.Findings), r.Findings)
				}
				return
			}
			if len(r.Findings) == 0 {
				t.Fatalf("Findings: expected at least 1 for %s, got 0", tc.status)
			}
			if r.Findings[0].Phrase != tc.phrase {
				t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, tc.phrase)
			}
			if r.Findings[0].Severity != tc.severity {
				t.Errorf("Findings[0].Severity = %v, want %v", r.Findings[0].Severity, tc.severity)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// access_denied_is_an_error_not_zero — AccessDenied on ListEnvironments must
// never render as an empty successful result (honest-degradation contract).
// ---------------------------------------------------------------------------

type mwaaListErrorFake struct {
	err error
}

func (f *mwaaListErrorFake) ListEnvironments(
	_ context.Context, _ *mwaa.ListEnvironmentsInput, _ ...func(*mwaa.Options),
) (*mwaa.ListEnvironmentsOutput, error) {
	return nil, f.err
}

func (f *mwaaListErrorFake) GetEnvironment(
	_ context.Context, _ *mwaa.GetEnvironmentInput, _ ...func(*mwaa.Options),
) (*mwaa.GetEnvironmentOutput, error) {
	return nil, fmt.Errorf("GetEnvironment should not be called when ListEnvironments fails")
}

var _ awsclient.MWAAAPI = (*mwaaListErrorFake)(nil)

func TestFetchMWAAEnvironmentsPage_AccessDeniedIsErrorNotZero(t *testing.T) {
	fake := &mwaaListErrorFake{
		err: &mwaatypes.AccessDeniedException{Message: aws.String("User is not authorized to perform mwaa:ListEnvironments")},
	}
	clients := &awsclient.ServiceClients{MWAA: fake}

	result, err := awsclient.FetchMWAAEnvironmentsPage(context.Background(), clients, "")
	if err == nil {
		t.Fatal("FetchMWAAEnvironmentsPage must return a non-nil error when ListEnvironments is denied — " +
			"never an empty successful result")
	}
	if !strings.Contains(err.Error(), "AccessDenied") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "AccessDenied")
	}
	if len(result.Resources) != 0 {
		t.Errorf("Resources: expected 0 on error, got %d", len(result.Resources))
	}
}

// ---------------------------------------------------------------------------
// U12/E3/E5 — partial GetEnvironment failure: partial results survive,
// composite error names every failed environment and reason.
// ---------------------------------------------------------------------------

type mwaaPartialGetFailureFake struct {
	names     []string
	envs      map[string]mwaatypes.Environment
	errByName map[string]error
}

func (f *mwaaPartialGetFailureFake) ListEnvironments(
	_ context.Context, _ *mwaa.ListEnvironmentsInput, _ ...func(*mwaa.Options),
) (*mwaa.ListEnvironmentsOutput, error) {
	return &mwaa.ListEnvironmentsOutput{Environments: f.names}, nil
}

func (f *mwaaPartialGetFailureFake) GetEnvironment(
	_ context.Context, input *mwaa.GetEnvironmentInput, _ ...func(*mwaa.Options),
) (*mwaa.GetEnvironmentOutput, error) {
	name := aws.ToString(input.Name)
	if err, ok := f.errByName[name]; ok {
		return nil, err
	}
	env, ok := f.envs[name]
	if !ok {
		return nil, fmt.Errorf("environment %q not found", name)
	}
	return &mwaa.GetEnvironmentOutput{Environment: &env}, nil
}

var _ awsclient.MWAAAPI = (*mwaaPartialGetFailureFake)(nil)

func TestFetchMWAAEnvironmentsPage_PartialGetFailure(t *testing.T) {
	names := []string{"env-a", "env-b", "env-c", "env-denied", "env-missing"}
	fake := &mwaaPartialGetFailureFake{
		names: names,
		envs: map[string]mwaatypes.Environment{
			"env-a": mwaaStatusOnlyEnvironment("env-a", mwaatypes.EnvironmentStatusAvailable),
			"env-b": mwaaStatusOnlyEnvironment("env-b", mwaatypes.EnvironmentStatusAvailable),
			"env-c": mwaaStatusOnlyEnvironment("env-c", mwaatypes.EnvironmentStatusAvailable),
		},
		errByName: map[string]error{
			"env-denied":  &mwaatypes.AccessDeniedException{Message: aws.String("not authorized")},
			"env-missing": &mwaatypes.ResourceNotFoundException{Message: aws.String("Environment env-missing not found")},
		},
	}
	clients := &awsclient.ServiceClients{MWAA: fake}

	result, err := awsclient.FetchMWAAEnvironmentsPage(context.Background(), clients, "")
	if err == nil {
		t.Fatal("FetchMWAAEnvironmentsPage must return a composite error when GetEnvironment fails for some environments")
	}
	if len(result.Resources) != 5 {
		t.Fatalf("got %d resources, want 5 (the 2 failing environments are KEPT as name-only degraded "+
			"rows — a denied GetEnvironment must never make a listed environment vanish)", len(result.Resources))
	}
	const wantDetail = "Access to environment details was denied; only the name is visible."
	degraded := map[string]bool{"env-denied": true, "env-missing": true}
	for _, r := range result.Resources {
		if !degraded[r.ID] {
			continue
		}
		if len(r.Findings) != 1 || r.Findings[0].Phrase != "details denied" {
			t.Errorf("degraded row %q must carry exactly the %q finding, got %+v", r.ID, "details denied", r.Findings)
		}
		if r.Fields["status"] != "details denied" {
			t.Errorf("degraded row %q status = %q, want %q", r.ID, r.Fields["status"], "details denied")
		}
		if len(r.Findings) == 1 && r.Findings[0].Detail != wantDetail {
			t.Errorf("degraded row %q Findings[0].Detail = %q, want %q (mwaa-specific — name-only row, no rich fields)",
				r.ID, r.Findings[0].Detail, wantDetail)
		}
	}
	errStr := err.Error()
	for _, want := range []string{"env-denied", "env-missing", "AccessDenied", "ResourceNotFound", "2 of 5"} {
		if !strings.Contains(errStr, want) {
			t.Errorf("composite error must contain %q, got: %q", want, errStr)
		}
	}
}

// ---------------------------------------------------------------------------
// Adversarial: nil Environment in GetEnvironment output
// ---------------------------------------------------------------------------

type mwaaNilEnvironmentFake struct{}

func (f *mwaaNilEnvironmentFake) ListEnvironments(
	_ context.Context, _ *mwaa.ListEnvironmentsInput, _ ...func(*mwaa.Options),
) (*mwaa.ListEnvironmentsOutput, error) {
	return &mwaa.ListEnvironmentsOutput{Environments: []string{"nil-env"}}, nil
}

func (f *mwaaNilEnvironmentFake) GetEnvironment(
	_ context.Context, _ *mwaa.GetEnvironmentInput, _ ...func(*mwaa.Options),
) (*mwaa.GetEnvironmentOutput, error) {
	return &mwaa.GetEnvironmentOutput{Environment: nil}, nil
}

var _ awsclient.MWAAAPI = (*mwaaNilEnvironmentFake)(nil)

func TestFetchMWAAEnvironmentsPage_NilEnvironmentInGetEnvironmentOutput(t *testing.T) {
	clients := &awsclient.ServiceClients{MWAA: &mwaaNilEnvironmentFake{}}
	result, err := awsclient.FetchMWAAEnvironmentsPage(context.Background(), clients, "")
	if err == nil {
		t.Fatal("FetchMWAAEnvironmentsPage must surface an error when GetEnvironment returns a nil Environment " +
			"(not panic, not silently drop)")
	}
	if !strings.Contains(err.Error(), "nil-env") {
		t.Errorf("composite error must name the affected environment %q, got: %q", "nil-env", err.Error())
	}
	if len(result.Resources) != 1 {
		t.Fatalf("Resources: expected 1 degraded name-only row, got %d", len(result.Resources))
	}
	if got := result.Resources[0].Fields["status"]; got != "details denied" {
		t.Errorf("degraded row status = %q, want %q", got, "details denied")
	}
}

// ---------------------------------------------------------------------------
// Adversarial: nil LoggingConfiguration/NetworkConfiguration must not panic
// ---------------------------------------------------------------------------

type mwaaNilSubConfigFake struct{}

func (f *mwaaNilSubConfigFake) ListEnvironments(
	_ context.Context, _ *mwaa.ListEnvironmentsInput, _ ...func(*mwaa.Options),
) (*mwaa.ListEnvironmentsOutput, error) {
	return &mwaa.ListEnvironmentsOutput{Environments: []string{"bare-env"}}, nil
}

func (f *mwaaNilSubConfigFake) GetEnvironment(
	_ context.Context, _ *mwaa.GetEnvironmentInput, _ ...func(*mwaa.Options),
) (*mwaa.GetEnvironmentOutput, error) {
	env := mwaatypes.Environment{
		Name:                 aws.String("bare-env"),
		Arn:                  aws.String("arn:aws:airflow:us-east-1:123456789012:environment/bare-env"),
		Status:               mwaatypes.EnvironmentStatusAvailable,
		LastUpdate:           &mwaatypes.LastUpdate{Status: mwaatypes.UpdateStatusSuccess},
		LoggingConfiguration: nil,
		NetworkConfiguration: nil,
	}
	return &mwaa.GetEnvironmentOutput{Environment: &env}, nil
}

var _ awsclient.MWAAAPI = (*mwaaNilSubConfigFake)(nil)

func TestFetchMWAAEnvironmentsPage_NilLoggingAndNetworkConfiguration(t *testing.T) {
	clients := &awsclient.ServiceClients{MWAA: &mwaaNilSubConfigFake{}}
	result, err := awsclient.FetchMWAAEnvironmentsPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("expected no error with nil LoggingConfiguration/NetworkConfiguration, got %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(result.Resources))
	}
	if len(result.Resources[0].Findings) != 0 {
		t.Errorf("Findings: expected 0 for a healthy AVAILABLE environment, got %d", len(result.Resources[0].Findings))
	}
}

// ---------------------------------------------------------------------------
// wave3_anti_tests — no Wave-3 metric names anywhere; AirflowVersion and a
// disabled logging component never become findings.
// ---------------------------------------------------------------------------

func TestFetchMWAAEnvironmentsPage_WaveThreeAntiTests(t *testing.T) {
	result := fetchMWAADemoPage(t)

	forbidden := []string{"SchedulerHeartbeat", "ImportErrors", "TotalParseTime", "QueuedTasks", "RunningTasks"}
	for _, r := range result.Resources {
		for _, f := range r.Findings {
			for _, s := range forbidden {
				if strings.Contains(f.Phrase, s) || strings.Contains(f.Detail, s) {
					t.Errorf("resource %q: Finding %+v must never surface Wave-3 metric name %q", r.ID, f, s)
				}
			}
			if strings.Contains(strings.ToLower(f.Phrase), "version") || strings.Contains(strings.ToLower(f.Detail), "eol") {
				t.Errorf("resource %q: Finding %+v must never be derived from AirflowVersion (no stable EOL source)",
					r.ID, f)
			}
		}
		for k, v := range r.Fields {
			for _, s := range forbidden {
				if strings.Contains(v, s) {
					t.Errorf("resource %q: Fields[%q] = %q must never surface Wave-3 metric name %q", r.ID, k, v, s)
				}
			}
		}
	}

	// prod-airflow-reporting has WebserverLogs.Enabled=false — a disabled
	// logging component must never raise a finding (spec §3.2).
	reporting := mustFindMWAAResource(t, result.Resources, fixtures.ProdAirflowReportingID)
	if len(reporting.Findings) != 0 {
		t.Errorf("disabled WebserverLogs component must not raise a finding, got Findings: %+v", reporting.Findings)
	}
}

// ---------------------------------------------------------------------------
// E7 sanity — Resource.ID is the bare environment name; Fields["arn"] holds
// the full ARN.
// ---------------------------------------------------------------------------

func TestFetchMWAAEnvironmentsPage_ResourceIDAndArnMapping(t *testing.T) {
	clients := &awsclient.ServiceClients{MWAA: fakes.NewMWAA()}
	result, err := awsclient.FetchMWAAEnvironmentsPage(context.Background(), clients, "")
	if err != nil && !strings.Contains(err.Error(), fixtures.WarnAirflowDetailsDeniedID) {
		// The demo set's details-denied witness makes the fetch a designed
		// E5 partial success; any OTHER error is a real failure.
		t.Fatalf("expected only the details-denied composite error, got %v", err)
	}
	r := mustFindMWAAResource(t, result.Resources, fixtures.ProdAirflowEtlID)

	if r.ID != fixtures.ProdAirflowEtlID {
		t.Errorf("ID = %q, want bare environment name %q", r.ID, fixtures.ProdAirflowEtlID)
	}
	wantArn := "arn:aws:airflow:us-east-1:123456789012:environment/" + fixtures.ProdAirflowEtlID
	if r.Fields["arn"] != wantArn {
		t.Errorf("Fields[\"arn\"] = %q, want %q", r.Fields["arn"], wantArn)
	}
}
