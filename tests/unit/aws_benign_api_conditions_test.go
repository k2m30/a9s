package unit

// aws_benign_api_conditions_test.go — RED tests pinning the intended contract
// for two live owner-reported error-lane floods (acme-dev logs):
//
//  1. sfn EXPRESS state machines: ListExecutions is not a supported operation
//     for EXPRESS state machines (StateMachineTypeNotSupported). The fetcher
//     already threads Fields["type"] from ListStateMachines through to the
//     enricher's resources slice (sfn.go FetchStepFunctionsPage). Contract:
//     EnrichStepFunctionsStatus must not call ListExecutions for a resource
//     whose Fields["type"] == "EXPRESS" — zero error, zero finding from this
//     enricher for that resource. A STANDARD machine is enriched normally.
//     If a StateMachineTypeNotSupported error is still produced by the API for
//     some resource, it must be treated as a benign skip and never surface in
//     the composite error.
//
//  2. kms per-key AccessDeniedException on DescribeKey inside FetchKMSKeysPage:
//     must NOT contribute to the composite fetch error (28/29 succeeding with
//     only access-denied failures ⇒ nil error), and the denied key's row must
//     still be present in Resources carrying a wave1 finding with an owner
//     phrase distinguishing it from other KMS states. Other error kinds
//     (throttle exhaustion, 500) must keep contributing to the composite
//     error exactly as today (AggregateFailures path), and must NOT get a
//     resource row assembled for that key.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// --- sfn EXPRESS skip -------------------------------------------------

// sfnTypeAwareFake implements SFNAPI (via embedding) and records every
// StateMachineArn passed to ListExecutions so the test can assert EXPRESS
// machines are never queried. Also supports forcing a
// StateMachineTypeNotSupported error for a given ARN, to pin the benign-skip
// fallback path (in case the "don't call it" guard is bypassed or partial).
type sfnTypeAwareFake struct {
	awsclient.SFNAPI
	executions       map[string]sfntypes.ExecutionStatus
	notSupportedARNs map[string]bool
	calledARNs       map[string]bool
	// unloggedARNs names the machines whose DescribeStateMachine body comes
	// back with logging off and no customer key. Everything else answers
	// healthy: an empty DescribeStateMachineOutput is NOT neutral — nil
	// logging means off and nil encryption means the AWS-owned key — so a
	// blanket empty stub would add two findings to every scenario in this
	// file.
	unloggedARNs map[string]bool
}

func (f *sfnTypeAwareFake) DescribeStateMachine(
	_ context.Context,
	params *sfn.DescribeStateMachineInput,
	_ ...func(*sfn.Options),
) (*sfn.DescribeStateMachineOutput, error) {
	arn := aws.ToString(params.StateMachineArn)
	out := &sfn.DescribeStateMachineOutput{
		StateMachineArn: params.StateMachineArn,
		Definition:      aws.String(`{"StartAt":"Done","States":{"Done":{"Type":"Succeed"}}}`),
	}
	if !f.unloggedARNs[arn] {
		out.LoggingConfiguration = &sfntypes.LoggingConfiguration{Level: sfntypes.LogLevelAll}
		out.EncryptionConfiguration = &sfntypes.EncryptionConfiguration{Type: sfntypes.EncryptionTypeCustomerManagedKmsKey}
	}
	return out, nil
}

func (f *sfnTypeAwareFake) ListExecutions(
	_ context.Context,
	params *sfn.ListExecutionsInput,
	_ ...func(*sfn.Options),
) (*sfn.ListExecutionsOutput, error) {
	arn := aws.ToString(params.StateMachineArn)
	if f.calledARNs == nil {
		f.calledARNs = make(map[string]bool)
	}
	f.calledARNs[arn] = true
	if f.notSupportedARNs[arn] {
		return nil, &smithy.GenericAPIError{
			Code:    "StateMachineTypeNotSupported",
			Message: "This operation is not supported by this type of state machine",
			Fault:   smithy.FaultClient,
		}
	}
	if status, ok := f.executions[arn]; ok {
		return &sfn.ListExecutionsOutput{
			Executions: []sfntypes.ExecutionListItem{
				{Status: status, StateMachineArn: &arn},
			},
		}, nil
	}
	return &sfn.ListExecutionsOutput{}, nil
}

// TestEnrichStepFunctionsStatus_ExpressMachineSkipsListExecutions pins that an
// EXPRESS state machine (Fields["type"] == "EXPRESS") never has ListExecutions
// called against it — AWS rejects that call for EXPRESS machines with
// StateMachineTypeNotSupported, so the enricher must recognize the type ahead
// of the call rather than reacting to the error. A sibling STANDARD machine in
// the same batch is enriched normally (finding produced when its latest
// execution is FAILED). The EXPRESS machine still gets the configuration
// findings, which are derived from DescribeStateMachine and apply to every
// state machine type.
func TestEnrichStepFunctionsStatus_ExpressMachineSkipsListExecutions(t *testing.T) {
	standardName := "standard-sm"
	standardARN := "arn:aws:states:us-east-1:111111111111:stateMachine:standard-sm"
	expressName := "express-sm"
	expressARN := "arn:aws:states:us-east-1:111111111111:stateMachine:express-sm"

	fake := &sfnTypeAwareFake{
		executions: map[string]sfntypes.ExecutionStatus{
			standardARN: sfntypes.ExecutionStatusFailed,
		},
		notSupportedARNs: map[string]bool{
			expressARN: true,
		},
		unloggedARNs: map[string]bool{
			expressARN: true,
		},
	}
	clients := &awsclient.ServiceClients{SFN: fake}
	resources := []resource.Resource{
		{ID: standardName, Fields: map[string]string{"arn": standardARN, "type": "STANDARD"}},
		{ID: expressName, Fields: map[string]string{"arn": expressARN, "type": "EXPRESS"}},
	}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected composite error: %v", err)
	}

	if fake.calledARNs[expressARN] {
		t.Errorf("ListExecutions must NOT be called for an EXPRESS state machine (arn=%s), but it was", expressARN)
	}
	if !fake.calledARNs[standardARN] {
		t.Errorf("ListExecutions must still be called for the STANDARD state machine (arn=%s)", standardARN)
	}

	// The EXPRESS machine has no EXECUTION finding, because ListExecutions is
	// never called for it, but its logging, encryption and definition are all
	// readable and all three checks apply to it; asserting no finding at all
	// would make exempting every EXPRESS workflow look intended.
	for _, f := range result.Findings[expressName] {
		if f.Code == "sfn.latest-execution-failed" {
			t.Errorf("EXPRESS state machine %q carries %q; ListExecutions is never called for it, so there is nothing to derive that from", expressName, f.Code)
		}
	}
	// The express machine's DescribeStateMachine body has logging off and no
	// customer key, so the configuration checks must still reach it.
	expressCodes := map[string]bool{}
	for _, f := range result.Findings[expressName] {
		expressCodes[string(f.Code)] = true
	}
	for _, want := range []string{"sfn.logging-off", "sfn.no-cmk"} {
		if !expressCodes[want] {
			t.Errorf("EXPRESS state machine %q is missing %q; skipping the execution listing must not skip the whole item", expressName, want)
		}
	}
	if _, ok := result.Findings[standardName]; !ok {
		t.Errorf("expected a finding for the STANDARD state machine %q (latest execution FAILED)", standardName)
	}

	if _, marked := result.TruncatedIDs[expressName]; marked {
		t.Errorf("EXPRESS state machine %q must not be marked truncated/errored", expressName)
	}
	if result.Truncated {
		t.Error("Truncated must be false: the only non-STANDARD machine was correctly skipped, no real failures occurred")
	}
}

// TestEnrichStepFunctionsStatus_StateMachineTypeNotSupportedIsBenignSkip pins
// the fallback contract: even if a StateMachineTypeNotSupported error reaches
// the enricher's ListExecutions call (e.g. the pre-call EXPRESS guard didn't
// fire because Fields["type"] was unset/stale), that specific error must be
// swallowed as a benign, non-erroring skip — it must NOT appear in the
// composite error and must NOT mark the resource truncated.
func TestEnrichStepFunctionsStatus_StateMachineTypeNotSupportedIsBenignSkip(t *testing.T) {
	name := "legacy-express-sm"
	arn := "arn:aws:states:us-east-1:111111111111:stateMachine:legacy-express-sm"

	fake := &sfnTypeAwareFake{
		notSupportedARNs: map[string]bool{arn: true},
	}
	clients := &awsclient.ServiceClients{SFN: fake}
	// Fields["type"] deliberately omitted/empty to simulate the guard not
	// having type information available — the API-level error must still be
	// treated as benign.
	resources := []resource.Resource{
		{ID: name, Fields: map[string]string{"arn": arn}},
	}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("StateMachineTypeNotSupported must never surface in the composite error, got: %v", err)
	}
	if strings.Contains(fdump(result), "StateMachineTypeNotSupported") {
		t.Error("result must carry no trace of StateMachineTypeNotSupported")
	}
	if _, ok := result.Findings[name]; ok {
		t.Errorf("expected no finding for %q when its ListExecutions call is benignly skipped", name)
	}
	if _, marked := result.TruncatedIDs[name]; marked {
		t.Errorf("resource %q must not be marked truncated for a benign StateMachineTypeNotSupported skip", name)
	}
	if result.Truncated {
		t.Error("Truncated must be false: StateMachineTypeNotSupported is not a real failure")
	}
}

// fdump renders enough of the result to grep for leaked raw AWS error text.
func fdump(r awsclient.IssueEnricherResult) string {
	var sb strings.Builder
	for id, fs := range r.Findings {
		for _, f := range fs {
			sb.WriteString(id)
			sb.WriteString(":")
			sb.WriteString(f.Phrase)
			sb.WriteString(" ")
			sb.WriteString(f.Detail)
			sb.WriteString(";")
		}
	}
	return sb.String()
}

// --- kms per-key AccessDenied -------------------------------------------

// kmsPageFake implements KMSAPI for FetchKMSKeysPage testing. ListKeys and
// ListAliases return fixed pages; DescribeKey is driven per-key-ID from a map
// of canned responses/errors.
type kmsPageFake struct {
	awsclient.KMSAPI
	keyIDs      []string
	aliases     map[string]string // keyID -> alias name
	describeErr map[string]error
	describeOK  map[string]kmstypes.KeyState
}

func (f *kmsPageFake) ListKeys(
	_ context.Context,
	_ *kms.ListKeysInput,
	_ ...func(*kms.Options),
) (*kms.ListKeysOutput, error) {
	entries := make([]kmstypes.KeyListEntry, 0, len(f.keyIDs))
	for _, id := range f.keyIDs {
		id := id
		entries = append(entries, kmstypes.KeyListEntry{KeyId: aws.String(id)})
	}
	return &kms.ListKeysOutput{Keys: entries, Truncated: false}, nil
}

func (f *kmsPageFake) ListAliases(
	_ context.Context,
	_ *kms.ListAliasesInput,
	_ ...func(*kms.Options),
) (*kms.ListAliasesOutput, error) {
	aliases := make([]kmstypes.AliasListEntry, 0, len(f.aliases))
	for keyID, name := range f.aliases {
		keyID, name := keyID, name
		aliases = append(aliases, kmstypes.AliasListEntry{TargetKeyId: aws.String(keyID), AliasName: aws.String(name)})
	}
	return &kms.ListAliasesOutput{Aliases: aliases, Truncated: false}, nil
}

func (f *kmsPageFake) DescribeKey(
	_ context.Context,
	params *kms.DescribeKeyInput,
	_ ...func(*kms.Options),
) (*kms.DescribeKeyOutput, error) {
	id := aws.ToString(params.KeyId)
	if err, ok := f.describeErr[id]; ok {
		return nil, err
	}
	state := kmstypes.KeyStateEnabled
	if s, ok := f.describeOK[id]; ok {
		state = s
	}
	return &kms.DescribeKeyOutput{
		KeyMetadata: &kmstypes.KeyMetadata{
			KeyId:      aws.String(id),
			KeyState:   state,
			KeyManager: kmstypes.KeyManagerTypeCustomer,
		},
	}, nil
}

// kmsAccessDeniedErr mirrors the real DescribeKey AccessDeniedException shape
// ("kms:DescribeKey").
func kmsAccessDeniedErr() error {
	return &smithy.GenericAPIError{
		Code:    "AccessDeniedException",
		Message: "User is not authorized to perform kms:DescribeKey",
		Fault:   smithy.FaultClient,
	}
}

// TestFetchKMSKeysPage_PerKeyAccessDeniedDoesNotContributeToCompositeError
// pins: when the only DescribeKey failures across a batch are
// AccessDeniedException, the composite fetch error is nil (a 28-of-29
// succeeding live pattern, modeled here at smaller scale).
func TestFetchKMSKeysPage_PerKeyAccessDeniedDoesNotContributeToCompositeError(t *testing.T) {
	fake := &kmsPageFake{
		keyIDs: []string{"key-ok-1", "key-denied", "key-ok-2"},
		describeErr: map[string]error{
			"key-denied": kmsAccessDeniedErr(),
		},
	}
	clients := &awsclient.ServiceClients{KMS: fake}

	result, err := awsclient.FetchKMSKeysPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("a per-key AccessDeniedException must not contribute to the composite fetch error, got: %v", err)
	}
	if len(result.Resources) != 3 {
		t.Fatalf("expected all 3 keys present as resources (including the denied one), got %d", len(result.Resources))
	}
}

// TestFetchKMSKeysPage_AccessDeniedKeyRowCarriesOwnerFinding pins the shape of
// the finding attached to a key whose DescribeKey call was denied: it must be
// present in Resources (not dropped/skipped) and carry a distinguishable
// owner-facing finding (code + human phrase), not folded into the generic
// "unavailable" bucket used for unmapped KeyState values.
func TestFetchKMSKeysPage_AccessDeniedKeyRowCarriesOwnerFinding(t *testing.T) {
	fake := &kmsPageFake{
		keyIDs: []string{"key-denied"},
		describeErr: map[string]error{
			"key-denied": kmsAccessDeniedErr(),
		},
	}
	clients := &awsclient.ServiceClients{KMS: fake}

	result, err := awsclient.FetchKMSKeysPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("unexpected composite error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected the access-denied key's row to still be present, got %d resources", len(result.Resources))
	}
	r := result.Resources[0]
	if r.ID != "key-denied" {
		t.Fatalf("resource ID = %q, want %q", r.ID, "key-denied")
	}
	if len(r.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding on the access-denied key row, got %d", len(r.Findings))
	}
	f := r.Findings[0]
	wantCode := "kms.access-denied"
	if string(f.Code) != wantCode {
		t.Errorf("finding Code = %q, want %q", f.Code, wantCode)
	}
	wantPhrase := "access denied (kms:DescribeKey)"
	if f.Phrase != wantPhrase {
		t.Errorf("finding Phrase = %q, want %q", f.Phrase, wantPhrase)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("finding Severity = %v, want SevBroken (access denial hides real key state — treat as broken, not a soft warning)", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("finding Source = %q, want %q (fetcher-level finding, not a Wave-2 enricher)", f.Source, "wave1")
	}
}

// TestFetchKMSKeysPage_ThrottleAndServerErrorsStillContributeToCompositeError
// pins that non-AccessDenied error kinds (throttle exhaustion, 500/internal)
// are NOT reclassified as benign — they must keep contributing to the
// composite AggregateFailures error exactly as before, and the failing key
// must be excluded from Resources (current fetcher behavior: `continue`,
// no partial row for a failed DescribeKey outside the access-denied case).
func TestFetchKMSKeysPage_ThrottleAndServerErrorsStillContributeToCompositeError(t *testing.T) {
	throttleErr := &smithy.GenericAPIError{Code: "ThrottlingException", Message: "Rate exceeded", Fault: smithy.FaultClient}
	serverErr := &smithy.GenericAPIError{Code: "InternalServiceErrorException", Message: "internal error", Fault: smithy.FaultServer}

	fake := &kmsPageFake{
		keyIDs: []string{"key-ok", "key-throttled", "key-servererr"},
		describeErr: map[string]error{
			"key-throttled": throttleErr,
			"key-servererr": serverErr,
		},
	}
	clients := &awsclient.ServiceClients{KMS: fake}

	result, err := awsclient.FetchKMSKeysPage(context.Background(), clients, "")
	if err == nil {
		t.Fatal("expected a non-nil composite error: throttle/server errors must still contribute")
	}
	if !strings.Contains(err.Error(), "2 of 3") {
		t.Errorf("composite error = %q, want it to report 2 of 3 IDs failing", err.Error())
	}
	if strings.Contains(err.Error(), "AccessDenied") {
		t.Errorf("composite error must not mention AccessDenied when no access-denied failures occurred: %q", err.Error())
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected only the 1 successful key as a resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "key-ok" {
		t.Errorf("surviving resource ID = %q, want %q", result.Resources[0].ID, "key-ok")
	}
}

// TestFetchKMSKeysPage_MixedAccessDeniedAndThrottleErrors pins the combined
// case: composite error must report and count ONLY the non-access-denied
// failure, while the access-denied key still gets a synthesized row.
func TestFetchKMSKeysPage_MixedAccessDeniedAndThrottleErrors(t *testing.T) {
	throttleErr := &smithy.GenericAPIError{Code: "ThrottlingException", Message: "Rate exceeded", Fault: smithy.FaultClient}

	fake := &kmsPageFake{
		keyIDs: []string{"key-denied", "key-throttled", "key-ok"},
		describeErr: map[string]error{
			"key-denied":    kmsAccessDeniedErr(),
			"key-throttled": throttleErr,
		},
	}
	clients := &awsclient.ServiceClients{KMS: fake}

	result, err := awsclient.FetchKMSKeysPage(context.Background(), clients, "")
	if err == nil {
		t.Fatal("expected a non-nil composite error from the throttle failure alone")
	}
	if !strings.Contains(err.Error(), "1 of 3") {
		t.Errorf("composite error = %q, want it to report exactly 1 of 3 IDs failing (throttle only, access-denied excluded)", err.Error())
	}

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources (key-ok and the synthesized key-denied row), got %d", len(result.Resources))
	}
	var sawDenied bool
	for _, r := range result.Resources {
		if r.ID == "key-denied" {
			sawDenied = true
			if len(r.Findings) != 1 || string(r.Findings[0].Code) != "kms.access-denied" {
				t.Errorf("key-denied row findings = %+v, want single kms.access-denied finding", r.Findings)
			}
		}
	}
	if !sawDenied {
		t.Error("expected key-denied to be present as a resource row despite the DescribeKey failure")
	}
}
