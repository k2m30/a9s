package unit

// prowler_w6b_cfn_test.go — behavioural pins for the two cfn posture signals
// of batch w6b: cfn.termination-protection-off and cfn.output-secret.
//
// Both are wave 1. FetchCloudFormationStacksPage already holds the whole
// cloudformation Stack — EnableTerminationProtection, ParentId and Outputs
// all arrive in the DescribeStacks response the fetcher makes — so neither
// condition may cost a second call.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w6bCFNCodeTerminationProtectionOff = domain.FindingCode("cfn.termination-protection-off")
	w6bCFNCodeOutputSecret             = domain.FindingCode("cfn.output-secret")
)

// w6bCFNFake serves DescribeStacks for the wave-1 fetcher path.
type w6bCFNFake struct {
	stacks []cfntypes.Stack
}

func (f *w6bCFNFake) DescribeStacks(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	return &cloudformation.DescribeStacksOutput{Stacks: f.stacks}, nil
}

// w6bCFNStack builds a stack in the shape DescribeStacks returns. protection
// is a tri-state on purpose: AWS omits EnableTerminationProtection entirely on
// some stacks, and "absent" is the shape that ships unprotected.
func w6bCFNStack(name, status string, protection *bool) cfntypes.Stack {
	return cfntypes.Stack{
		StackName:                   aws.String(name),
		StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" + name + "/aaaa1111-bbbb-2222-cccc-333344445555"),
		StackStatus:                 cfntypes.StackStatus(status),
		CreationTime:                aws.Time(time.Now().Add(-200 * 24 * time.Hour)),
		Description:                 aws.String("acme " + name),
		EnableTerminationProtection: protection,
	}
}

func w6bFetchCFN(t *testing.T, stacks ...cfntypes.Stack) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchCloudFormationStacksPage(context.Background(), &w6bCFNFake{stacks: stacks}, "")
	if err != nil {
		t.Fatalf("FetchCloudFormationStacksPage: %v", err)
	}
	return out.Resources
}

// ─── row 1: cfn.termination-protection-off ──────────────────────────────────

// A live top-level stack with protection off can be deleted by a single API
// call, taking every resource it owns with it.
func TestW6BCFN_TerminationProtectionOff_Disabled(t *testing.T) {
	const name = "acme-legacy-api"
	rs := w6bFetchCFN(t, w6bCFNStack(name, "CREATE_COMPLETE", aws.Bool(false)))
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bCFNCodeTerminationProtectionOff,
		"termination protection off", domain.SevWarn, "wave1")
	// The phrase is the whole fact; a "Termination protection: off" row under
	// it would print that fact twice (U11).
	w6bRequireNoRows(t, w6bWave1Rows(r, w6bCFNCodeTerminationProtectionOff))
}

// AWS omits the field on stacks that predate the setting. Absent is not
// "protected", so the finding still fires — this is the row the spec calls out
// as the exception to "nil never triggers a finding".
func TestW6BCFN_TerminationProtectionOff_FieldAbsent(t *testing.T) {
	const name = "acme-ancient-stack"
	rs := w6bFetchCFN(t, w6bCFNStack(name, "UPDATE_COMPLETE", nil))
	pw1RequireFinding(t, pw1ResourceByID(t, rs, name).Findings,
		w6bCFNCodeTerminationProtectionOff, "termination protection off", domain.SevWarn, "wave1")
}

func TestW6BCFN_TerminationProtectionOn_IsHealthy(t *testing.T) {
	const name = "acme-prod-network"
	rs := w6bFetchCFN(t, w6bCFNStack(name, "CREATE_COMPLETE", aws.Bool(true)))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCFNCodeTerminationProtectionOff)
}

// A nested stack inherits the root stack's protection; flagging it would ask
// the operator to change a setting that is not theirs to change.
func TestW6BCFN_NestedStack_NotEvaluated(t *testing.T) {
	const name = "acme-prod-network-NestedVPC-1ABCDEF"
	stack := w6bCFNStack(name, "CREATE_COMPLETE", aws.Bool(false))
	stack.ParentId = aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-prod-network/aaaa1111-bbbb-2222-cccc-333344445555")
	rs := w6bFetchCFN(t, stack)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCFNCodeTerminationProtectionOff)
}

// A stack that is being torn down, or that failed to build, has no posture to
// fix — contract rule 4.
func TestW6BCFN_DeletedAndFailedStacks_EmitNoPostureFinding(t *testing.T) {
	for _, status := range []string{"DELETE_COMPLETE", "DELETE_IN_PROGRESS", "DELETE_FAILED", "CREATE_FAILED", "ROLLBACK_FAILED"} {
		t.Run(status, func(t *testing.T) {
			name := "acme-gone-" + status
			rs := w6bFetchCFN(t, w6bCFNStack(name, status, aws.Bool(false)))
			pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCFNCodeTerminationProtectionOff)
		})
	}
}

// ─── row 2: cfn.output-secret ───────────────────────────────────────────────

// A stack output is readable by anyone who can call DescribeStacks, so a
// password pasted into one is a credential handed to every reader of the
// account.
func TestW6BCFN_OutputSecret_PlaintextCredential(t *testing.T) {
	const name = "acme-rds-aurora"
	const secret = "hunter2hunter2"
	stack := w6bCFNStack(name, "CREATE_COMPLETE", aws.Bool(true))
	stack.Outputs = []cfntypes.Output{
		{OutputKey: aws.String("ClusterEndpoint"), OutputValue: aws.String("acme-aurora.cluster-abc.us-east-1.rds.amazonaws.com")},
		{OutputKey: aws.String("DBPassword"), OutputValue: aws.String(secret)},
	}
	rs := w6bFetchCFN(t, stack)
	r := pw1ResourceByID(t, rs, name)

	f := pw1RequireFinding(t, r.Findings, w6bCFNCodeOutputSecret,
		"credential in stack outputs", domain.SevBroken, "wave1")
	rows := w6bWave1Rows(r, w6bCFNCodeOutputSecret)
	pw1RequireRow(t, rows, "DBPassword", "keyword")
	w6bRequireNoSecretLeak(t, f, rows, secret)
}

// Outputs naming a Secrets Manager ARN are the fix, not the defect: the
// indirection is exactly what an operator is told to do.
func TestW6BCFN_OutputSecretReference_IsHealthy(t *testing.T) {
	const name = "acme-rds-aurora-fixed"
	stack := w6bCFNStack(name, "CREATE_COMPLETE", aws.Bool(true))
	stack.Outputs = []cfntypes.Output{
		{OutputKey: aws.String("DBPasswordSecretArn"), OutputValue: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-AbCdEf")},
	}
	rs := w6bFetchCFN(t, stack)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCFNCodeOutputSecret)
}

func TestW6BCFN_NoOutputs_IsHealthy(t *testing.T) {
	const name = "acme-no-outputs"
	rs := w6bFetchCFN(t, w6bCFNStack(name, "CREATE_COMPLETE", aws.Bool(true)))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bCFNCodeOutputSecret)
}

// ─── independence ───────────────────────────────────────────────────────────

// Contract rule 4: two conditions on one stack are two findings, and neither
// swallows the lifecycle finding the fetcher already emitted.
func TestW6BCFN_BothConditions_ProduceTwoFindings(t *testing.T) {
	const name = "acme-double-trouble"
	stack := w6bCFNStack(name, "UPDATE_IN_PROGRESS", aws.Bool(false))
	stack.Outputs = []cfntypes.Output{
		{OutputKey: aws.String("ApiToken"), OutputValue: aws.String("swordfish99")},
	}
	rs := w6bFetchCFN(t, stack)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bCFNCodeTerminationProtectionOff,
		"termination protection off", domain.SevWarn, "wave1")
	pw1RequireFinding(t, r.Findings, w6bCFNCodeOutputSecret,
		"credential in stack outputs", domain.SevBroken, "wave1")
	if _, ok := pw1FindFinding(r.Findings, awsclient.CodeCFNStackInProgress); !ok {
		t.Errorf("lifecycle finding lost when posture findings were added: %+v", r.Findings)
	}
}

// ─── catalog ────────────────────────────────────────────────────────────────

func TestW6BCFN_FindingDefsRegistered(t *testing.T) {
	w2AssertFindingDef(t, "cfn", string(w6bCFNCodeTerminationProtectionOff),
		"termination protection off", domain.SevWarn, "wave1")
	w2AssertFindingDef(t, "cfn", string(w6bCFNCodeOutputSecret),
		"credential in stack outputs", domain.SevBroken, "wave1")
}
