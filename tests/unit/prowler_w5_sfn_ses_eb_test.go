package unit

// prowler_w5_sfn_ses_eb_test.go — batch w5 rows 9-15: the three Step
// Functions signals, the SES DKIM signal, and the three Elastic Beanstalk
// environment settings.
//
// All seven are additions to enrichers that already exist, per rule 5 of the
// batch contract — never a second enricher for a type that has one. The
// fakes below therefore have to serve BOTH the call the enricher already
// made and the new one, or the pre-existing findings vanish and the tests
// would pass for the wrong reason.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ebsvc "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	sesv2svc "github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	sfnsvc "github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Rows 9, 10, 11 — sfn.logging-off, sfn.no-cmk, sfn.definition-secret
// ---------------------------------------------------------------------------

// w5SFNFake serves ListExecutions (which EnrichStepFunctionsStatus already
// called) and DescribeStateMachine (which rows 9-11 need).
type w5SFNFake struct {
	awsclient.SFNAPI
	describe    map[string]*sfnsvc.DescribeStateMachineOutput
	describeErr map[string]error
	executions  map[string][]sfntypes.ExecutionListItem
}

func newW5SFNFake() *w5SFNFake {
	return &w5SFNFake{
		describe:    map[string]*sfnsvc.DescribeStateMachineOutput{},
		describeErr: map[string]error{},
		executions:  map[string][]sfntypes.ExecutionListItem{},
	}
}

func (f *w5SFNFake) DescribeStateMachine(_ context.Context, in *sfnsvc.DescribeStateMachineInput, _ ...func(*sfnsvc.Options)) (*sfnsvc.DescribeStateMachineOutput, error) {
	arn := ""
	if in != nil && in.StateMachineArn != nil {
		arn = *in.StateMachineArn
	}
	if err, ok := f.describeErr[arn]; ok {
		return nil, err
	}
	if out, ok := f.describe[arn]; ok {
		return out, nil
	}
	return &sfnsvc.DescribeStateMachineOutput{}, nil
}

func (f *w5SFNFake) ListExecutions(_ context.Context, in *sfnsvc.ListExecutionsInput, _ ...func(*sfnsvc.Options)) (*sfnsvc.ListExecutionsOutput, error) {
	arn := ""
	if in != nil && in.StateMachineArn != nil {
		arn = *in.StateMachineArn
	}
	return &sfnsvc.ListExecutionsOutput{Executions: f.executions[arn]}, nil
}

var _ awsclient.SFNAPI = (*w5SFNFake)(nil)

func w5SFNArn(name string) string {
	return "arn:aws:states:us-east-1:123456789012:stateMachine:" + name
}

// w5SFNRes mirrors the sfn fetcher: ID is the bare name, the ARN is in
// Fields["arn"], and Fields["type"] carries STANDARD or EXPRESS.
func w5SFNRes(name, smType string) resource.Resource {
	return resource.Resource{
		ID:     name,
		Name:   name,
		Fields: map[string]string{"name": name, "arn": w5SFNArn(name), "type": smType},
	}
}

// w5CleanDefinition is an Amazon States Language document with no credential
// in it — the healthy counterpart for row 11.
const w5CleanDefinition = `{
  "Comment": "Order settlement",
  "StartAt": "Validate",
  "States": {
    "Validate": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:us-east-1:123456789012:function:acme-validate",
      "Next": "Settle"
    },
    "Settle": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:us-east-1:123456789012:function:acme-settle",
      "End": true
    }
  }
}`

// w5SecretDefinition embeds a credential in a task's Parameters, which is
// how it usually happens: a developer inlines the value rather than reading
// it from Secrets Manager at run time.
const w5SecretDefinition = `{
  "Comment": "Order settlement",
  "StartAt": "CallPartner",
  "States": {
    "CallPartner": {
      "Type": "Task",
      "Resource": "arn:aws:states:::http:invoke",
      "Parameters": {
        "ApiEndpoint": "https://partner.example.com/settle",
        "DB_PASSWORD": "hunter2-correct-horse-battery"
      },
      "End": true
    }
  }
}`

// w5HealthyStateMachine is the healthy DescribeStateMachine body: logging on
// at ALL, a customer managed key, a clean definition.
func w5HealthyStateMachine(name string) *sfnsvc.DescribeStateMachineOutput {
	return &sfnsvc.DescribeStateMachineOutput{
		StateMachineArn: aws.String(w5SFNArn(name)),
		Name:            aws.String(name),
		Type:            sfntypes.StateMachineTypeStandard,
		Status:          sfntypes.StateMachineStatusActive,
		RoleArn:         aws.String("arn:aws:iam::123456789012:role/acme-sfn-exec"),
		Definition:      aws.String(w5CleanDefinition),
		LoggingConfiguration: &sfntypes.LoggingConfiguration{
			Level:                sfntypes.LogLevelAll,
			IncludeExecutionData: true,
		},
		EncryptionConfiguration: &sfntypes.EncryptionConfiguration{
			Type:     sfntypes.EncryptionTypeCustomerManagedKmsKey,
			KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
		},
	}
}

func w5EnrichSFN(t *testing.T, f *w5SFNFake, rows ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichStepFunctionsStatus(
		context.Background(),
		&awsclient.ServiceClients{SFN: f},
		rows,
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	return res
}

// A state machine with logging off leaves no record of which state failed,
// so a production incident has nothing to read back.
func TestW5_SFNLoggingOff_Positive(t *testing.T) {
	tests := []struct {
		name string
		cfg  *sfntypes.LoggingConfiguration
	}{
		{"logging configuration absent", nil},
		{"level OFF", &sfntypes.LoggingConfiguration{Level: sfntypes.LogLevelOff}},
		{"level unset", &sfntypes.LoggingConfiguration{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sm := "acme-settlement-silent"
			f := newW5SFNFake()
			out := w5HealthyStateMachine(sm)
			out.LoggingConfiguration = tc.cfg
			f.describe[w5SFNArn(sm)] = out

			res := w5EnrichSFN(t, f, w5SFNRes(sm, "STANDARD"))
			w2AssertFinding(t, res.Findings[sm], "sfn.logging-off",
				"execution logging off", domain.SevWarn, "wave2")

			rows := w2Rows(t, res, sm, "sfn.logging-off")
			w2AssertRow(t, rows, "Logging level", "off")
		})
	}
}

// ERROR and FATAL are partial but real logging. Only OFF is the finding.
func TestW5_SFNLoggingOff_AnyLevelAboveOffIsHealthy(t *testing.T) {
	for _, level := range []sfntypes.LogLevel{
		sfntypes.LogLevelAll,
		sfntypes.LogLevelError,
		sfntypes.LogLevelFatal,
	} {
		t.Run(string(level), func(t *testing.T) {
			sm := "acme-settlement"
			f := newW5SFNFake()
			out := w5HealthyStateMachine(sm)
			out.LoggingConfiguration = &sfntypes.LoggingConfiguration{Level: level}
			f.describe[w5SFNArn(sm)] = out

			res := w5EnrichSFN(t, f, w5SFNRes(sm, "STANDARD"))
			w2AssertNoCode(t, res.Findings[sm], "sfn.logging-off")
		})
	}
}

// EXPRESS state machines are skipped for the execution-listing finding
// because ListExecutions rejects them outright. Logging, encryption and the
// definition are all readable for an EXPRESS machine and the Prowler checks
// apply to it, so skipping the whole item would silently exempt every
// EXPRESS workflow in the account.
func TestW5_SFNExpressStateMachineStillReportsConfigurationFindings(t *testing.T) {
	sm := "acme-ingest-express"
	f := newW5SFNFake()
	out := w5HealthyStateMachine(sm)
	out.Type = sfntypes.StateMachineTypeExpress
	out.LoggingConfiguration = &sfntypes.LoggingConfiguration{Level: sfntypes.LogLevelOff}
	out.EncryptionConfiguration = nil
	f.describe[w5SFNArn(sm)] = out

	res := w5EnrichSFN(t, f, w5SFNRes(sm, "EXPRESS"))
	w2AssertFinding(t, res.Findings[sm], "sfn.logging-off",
		"execution logging off", domain.SevWarn, "wave2")
	w2AssertFinding(t, res.Findings[sm], "sfn.no-cmk",
		"not encrypted with a customer key", domain.SevWarn, "wave2")
}

// A state machine on the AWS-owned key cannot have its data access audited
// or revoked independently of AWS.
func TestW5_SFNNoCMK_Positive(t *testing.T) {
	tests := []struct {
		name string
		cfg  *sfntypes.EncryptionConfiguration
	}{
		{"encryption configuration absent", nil},
		{"AWS owned key", &sfntypes.EncryptionConfiguration{Type: sfntypes.EncryptionTypeAwsOwnedKey}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sm := "acme-settlement-awskey"
			f := newW5SFNFake()
			out := w5HealthyStateMachine(sm)
			out.EncryptionConfiguration = tc.cfg
			f.describe[w5SFNArn(sm)] = out

			res := w5EnrichSFN(t, f, w5SFNRes(sm, "STANDARD"))
			w2AssertFinding(t, res.Findings[sm], "sfn.no-cmk",
				"not encrypted with a customer key", domain.SevWarn, "wave2")
		})
	}
}

func TestW5_SFNNoCMK_CustomerManagedKeyIsHealthy(t *testing.T) {
	sm := "acme-settlement"
	f := newW5SFNFake()
	f.describe[w5SFNArn(sm)] = w5HealthyStateMachine(sm)

	res := w5EnrichSFN(t, f, w5SFNRes(sm, "STANDARD"))
	w2AssertNoCode(t, res.Findings[sm], "sfn.no-cmk")
}

// A credential inlined in the definition is readable by anyone with
// states:DescribeStateMachine and is in every copy of the template.
func TestW5_SFNDefinitionSecret_Positive(t *testing.T) {
	sm := "acme-settlement-leaky"
	f := newW5SFNFake()
	out := w5HealthyStateMachine(sm)
	out.Definition = aws.String(w5SecretDefinition)
	f.describe[w5SFNArn(sm)] = out

	res := w5EnrichSFN(t, f, w5SFNRes(sm, "STANDARD"))
	finding := w2AssertFinding(t, res.Findings[sm], "sfn.definition-secret",
		"credential in state machine definition", domain.SevBroken, "wave2")

	// Rule 7: the value never appears anywhere an operator can read it.
	// Where and Kind are the whole of the evidence.
	rows := w2Rows(t, res, sm, "sfn.definition-secret")
	if len(rows) == 0 {
		t.Fatal("the secret finding carries no supporting row; Where and Kind are the only evidence there can be")
	}
	for _, r := range rows {
		if strings.Contains(r.Value, "hunter2") || strings.Contains(r.Label, "hunter2") {
			t.Errorf("row %q: %q contains the secret value itself", r.Label, r.Value)
		}
	}
	for _, s := range []string{finding.Phrase, finding.Detail} {
		if strings.Contains(s, "hunter2") {
			t.Errorf("the secret value leaked into a rendered string: %q", s)
		}
	}
}

func TestW5_SFNDefinitionSecret_CleanDefinitionIsHealthy(t *testing.T) {
	sm := "acme-settlement"
	f := newW5SFNFake()
	f.describe[w5SFNArn(sm)] = w5HealthyStateMachine(sm)

	res := w5EnrichSFN(t, f, w5SFNRes(sm, "STANDARD"))
	w2AssertNoCode(t, res.Findings[sm], "sfn.definition-secret")
}

// An absent Definition says nothing. AWS omits it for a machine the caller
// may list but not read, and treating absence as clean is right while
// treating it as a hit would flag a permissions gap as a leak.
func TestW5_SFNDefinitionSecret_AbsentDefinitionIsNotAFinding(t *testing.T) {
	sm := "acme-settlement-nodef"
	f := newW5SFNFake()
	out := w5HealthyStateMachine(sm)
	out.Definition = nil
	f.describe[w5SFNArn(sm)] = out

	res := w5EnrichSFN(t, f, w5SFNRes(sm, "STANDARD"))
	w2AssertNoCode(t, res.Findings[sm], "sfn.definition-secret")
}

// Rule 4: a state machine that is unlogged, on the AWS-owned key, and
// carrying a credential reports three findings, not one.
func TestW5_SFNAllThreeConditions_AreThreeFindings(t *testing.T) {
	sm := "acme-settlement-worst"
	f := newW5SFNFake()
	out := w5HealthyStateMachine(sm)
	out.LoggingConfiguration = nil
	out.EncryptionConfiguration = nil
	out.Definition = aws.String(w5SecretDefinition)
	f.describe[w5SFNArn(sm)] = out

	res := w5EnrichSFN(t, f, w5SFNRes(sm, "STANDARD"))
	for _, code := range []string{"sfn.logging-off", "sfn.no-cmk", "sfn.definition-secret"} {
		if _, ok := w2Find(res.Findings[sm], code); !ok {
			t.Errorf("no finding %q; got %v", code, w2Codes(res.Findings[sm]))
		}
	}
}

// The pre-existing execution finding must survive the addition. An enricher
// that returns early on the new call would silently drop it.
func TestW5_SFNNewFindingsDoNotDisplaceTheExecutionFinding(t *testing.T) {
	sm := "acme-settlement-failed"
	f := newW5SFNFake()
	out := w5HealthyStateMachine(sm)
	out.LoggingConfiguration = nil
	f.describe[w5SFNArn(sm)] = out
	f.executions[w5SFNArn(sm)] = []sfntypes.ExecutionListItem{{
		ExecutionArn:    aws.String(w5SFNArn(sm) + ":run-0001"),
		StateMachineArn: aws.String(w5SFNArn(sm)),
		Name:            aws.String("run-0001"),
		Status:          sfntypes.ExecutionStatusFailed,
	}}

	res := w5EnrichSFN(t, f, w5SFNRes(sm, "STANDARD"))
	if _, ok := w2Find(res.Findings[sm], "sfn.latest-execution-failed"); !ok {
		t.Errorf("the pre-existing execution finding is gone; got %v", w2Codes(res.Findings[sm]))
	}
	if _, ok := w2Find(res.Findings[sm], "sfn.logging-off"); !ok {
		t.Errorf("no sfn.logging-off alongside it; got %v", w2Codes(res.Findings[sm]))
	}
}

// One state machine's DescribeStateMachine failing marks that one unknown
// and leaves the rest evaluated.
func TestW5_SFN_APIErrorOnOneMachineTruncatesOnlyThatMachine(t *testing.T) {
	broken, silent := "acme-broken", "acme-settlement-silent"
	f := newW5SFNFake()
	f.describeErr[w5SFNArn(broken)] = errors.New("AccessDeniedException: not authorized to perform states:DescribeStateMachine")
	out := w5HealthyStateMachine(silent)
	out.LoggingConfiguration = nil
	f.describe[w5SFNArn(silent)] = out

	res, _ := awsclient.EnrichStepFunctionsStatus(
		context.Background(),
		&awsclient.ServiceClients{SFN: f},
		[]resource.Resource{w5SFNRes(broken, "STANDARD"), w5SFNRes(silent, "STANDARD")},
		nil,
	)
	w2AssertEnricherShape(t, res)

	if _, marked := res.TruncatedIDs[broken]; !marked {
		t.Errorf("TruncatedIDs[%q] = false; a state machine that could not be described is unknown", broken)
	}
	w2AssertNoCode(t, res.Findings[broken], "sfn.logging-off")
	w2AssertFinding(t, res.Findings[silent], "sfn.logging-off",
		"execution logging off", domain.SevWarn, "wave2")
}

func TestW5_SFNCatalogDefs(t *testing.T) {
	w2AssertFindingDef(t, "sfn", "sfn.logging-off",
		"execution logging off", domain.SevWarn, "wave2")
	w2AssertFindingDef(t, "sfn", "sfn.no-cmk",
		"not encrypted with a customer key", domain.SevWarn, "wave2")
	w2AssertFindingDef(t, "sfn", "sfn.definition-secret",
		"credential in state machine definition", domain.SevBroken, "wave2")
}

// ---------------------------------------------------------------------------
// Row 12 — ses.dkim-off
// ---------------------------------------------------------------------------

// w5SESFake serves GetAccount (which EnrichSESAccount already called) and
// GetEmailIdentity per identity (which row 12 needs).
type w5SESFake struct {
	awsclient.SESv2API
	account     *sesv2svc.GetAccountOutput
	identities  map[string]*sesv2svc.GetEmailIdentityOutput
	identityErr map[string]error
}

func newW5SESFake() *w5SESFake {
	return &w5SESFake{
		// A healthy account, so the account-level findings stay silent and
		// every assertion below is about DKIM alone.
		account: &sesv2svc.GetAccountOutput{
			EnforcementStatus: aws.String("HEALTHY"),
			SendingEnabled:    true,
			SendQuota:         &sesv2types.SendQuota{Max24HourSend: 50000, SentLast24Hours: 120},
		},
		identities:  map[string]*sesv2svc.GetEmailIdentityOutput{},
		identityErr: map[string]error{},
	}
}

func (f *w5SESFake) GetAccount(_ context.Context, _ *sesv2svc.GetAccountInput, _ ...func(*sesv2svc.Options)) (*sesv2svc.GetAccountOutput, error) {
	return f.account, nil
}

func (f *w5SESFake) GetEmailIdentity(_ context.Context, in *sesv2svc.GetEmailIdentityInput, _ ...func(*sesv2svc.Options)) (*sesv2svc.GetEmailIdentityOutput, error) {
	name := ""
	if in != nil && in.EmailIdentity != nil {
		name = *in.EmailIdentity
	}
	if err, ok := f.identityErr[name]; ok {
		return nil, err
	}
	if out, ok := f.identities[name]; ok {
		return out, nil
	}
	return &sesv2svc.GetEmailIdentityOutput{}, nil
}

var _ awsclient.SESv2API = (*w5SESFake)(nil)

// w5SESRes mirrors the ses fetcher: ID is the identity name.
func w5SESRes(name, identityType string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"identity_name":       name,
			"identity_type":       identityType,
			"sending_enabled":     "true",
			"verification_status": "SUCCESS",
		},
	}
}

func w5DomainIdentity(signing bool) *sesv2svc.GetEmailIdentityOutput {
	return &sesv2svc.GetEmailIdentityOutput{
		IdentityType:             sesv2types.IdentityTypeDomain,
		VerifiedForSendingStatus: true,
		DkimAttributes: &sesv2types.DkimAttributes{
			SigningEnabled:          signing,
			Status:                  sesv2types.DkimStatusSuccess,
			SigningAttributesOrigin: sesv2types.DkimSigningAttributesOriginAwsSes,
		},
	}
}

func w5EnrichSES(t *testing.T, f *w5SESFake, rows ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichSESAccount(
		context.Background(),
		&awsclient.ServiceClients{SESv2: f},
		rows,
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	return res
}

// A domain that does not sign its outbound mail lets anyone forge mail from
// it, and receivers have no way to tell the difference.
func TestW5_SESDKIMOff_Positive(t *testing.T) {
	tests := []struct {
		name  string
		build func() *sesv2svc.GetEmailIdentityOutput
	}{
		{"signing disabled", func() *sesv2svc.GetEmailIdentityOutput { return w5DomainIdentity(false) }},
		{"DKIM attributes absent", func() *sesv2svc.GetEmailIdentityOutput {
			out := w5DomainIdentity(false)
			out.DkimAttributes = nil
			return out
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id := "nodkim.acme-corp.com"
			f := newW5SESFake()
			f.identities[id] = tc.build()

			res := w5EnrichSES(t, f, w5SESRes(id, "DOMAIN"))
			w2AssertFinding(t, res.Findings[id], "ses.dkim-off",
				"DKIM not enabled", domain.SevWarn, "wave2")

			rows := w2Rows(t, res, id, "ses.dkim-off")
			w2AssertRow(t, rows, "DKIM signing", "disabled")
		})
	}
}

func TestW5_SESDKIMOff_SigningEnabledIsHealthy(t *testing.T) {
	id := "acme-corp.com"
	f := newW5SESFake()
	f.identities[id] = w5DomainIdentity(true)

	res := w5EnrichSES(t, f, w5SESRes(id, "DOMAIN"))
	w2AssertNoCode(t, res.Findings[id], "ses.dkim-off")
}

// DKIM is a domain-level mechanism. A single verified address cannot have
// it, so flagging one would be an issue an operator can never close.
func TestW5_SESDKIMOff_EmailAddressIdentityIsNotAFinding(t *testing.T) {
	id := "ops@acme-corp.com"
	f := newW5SESFake()
	out := w5DomainIdentity(false)
	out.IdentityType = sesv2types.IdentityTypeEmailAddress
	f.identities[id] = out

	res := w5EnrichSES(t, f, w5SESRes(id, "EMAIL_ADDRESS"))
	w2AssertNoCode(t, res.Findings[id], "ses.dkim-off")
}

// The finding is per identity. The account-level findings this enricher
// already produced are replicated onto every row, and DKIM must not inherit
// that shape: one unsigned domain does not make every other domain unsigned.
func TestW5_SESDKIMOff_IsPerIdentityNotReplicated(t *testing.T) {
	bad, good := "nodkim.acme-corp.com", "acme-corp.com"
	f := newW5SESFake()
	f.identities[bad] = w5DomainIdentity(false)
	f.identities[good] = w5DomainIdentity(true)

	res := w5EnrichSES(t, f, w5SESRes(bad, "DOMAIN"), w5SESRes(good, "DOMAIN"))
	w2AssertFinding(t, res.Findings[bad], "ses.dkim-off",
		"DKIM not enabled", domain.SevWarn, "wave2")
	w2AssertNoCode(t, res.Findings[good], "ses.dkim-off")
}

// One identity's GetEmailIdentity failing marks that identity unknown and
// leaves the others evaluated.
func TestW5_SESDKIMOff_APIErrorOnOneIdentityTruncatesOnlyThatIdentity(t *testing.T) {
	broken, bad := "denied.acme-corp.com", "nodkim.acme-corp.com"
	f := newW5SESFake()
	f.identityErr[broken] = errors.New("AccessDeniedException: not authorized to perform ses:GetEmailIdentity")
	f.identities[bad] = w5DomainIdentity(false)

	res, _ := awsclient.EnrichSESAccount(
		context.Background(),
		&awsclient.ServiceClients{SESv2: f},
		[]resource.Resource{w5SESRes(broken, "DOMAIN"), w5SESRes(bad, "DOMAIN")},
		nil,
	)
	w2AssertEnricherShape(t, res)

	if _, marked := res.TruncatedIDs[broken]; !marked {
		t.Errorf("TruncatedIDs[%q] = false; an identity that could not be read is unknown, not signed", broken)
	}
	w2AssertNoCode(t, res.Findings[broken], "ses.dkim-off")
	w2AssertFinding(t, res.Findings[bad], "ses.dkim-off",
		"DKIM not enabled", domain.SevWarn, "wave2")
}

// The pre-existing account-level finding must still reach every row.
func TestW5_SESDKIMDoesNotDisplaceTheAccountFinding(t *testing.T) {
	id := "nodkim.acme-corp.com"
	f := newW5SESFake()
	f.account = &sesv2svc.GetAccountOutput{
		EnforcementStatus: aws.String("SHUTDOWN"),
		SendQuota:         &sesv2types.SendQuota{Max24HourSend: 50000, SentLast24Hours: 120},
	}
	f.identities[id] = w5DomainIdentity(false)

	res := w5EnrichSES(t, f, w5SESRes(id, "DOMAIN"))
	if _, ok := w2Find(res.Findings[id], "ses.account-shutdown"); !ok {
		t.Errorf("the account-level shutdown finding is gone; got %v", w2Codes(res.Findings[id]))
	}
	if _, ok := w2Find(res.Findings[id], "ses.dkim-off"); !ok {
		t.Errorf("no ses.dkim-off alongside it; got %v", w2Codes(res.Findings[id]))
	}
}

func TestW5_SESCatalogDef(t *testing.T) {
	w2AssertFindingDef(t, "ses", "ses.dkim-off",
		"DKIM not enabled", domain.SevWarn, "wave2")
}

// ---------------------------------------------------------------------------
// Rows 13, 14, 15 — the three Elastic Beanstalk environment settings
// ---------------------------------------------------------------------------

// w5EBFake serves DescribeEnvironmentHealth (which EnrichEBEnvironmentHealth
// already called) and DescribeConfigurationSettings (which rows 13-15 need).
type w5EBFake struct {
	awsclient.ElasticBeanstalkAPI
	options   map[string][]ebtypes.ConfigurationOptionSetting
	optionErr map[string]error
	causes    map[string][]string
	// mu guards requests: the enricher fans its per-environment calls out
	// through ForEachParallel, so every recording write here is concurrent.
	mu       sync.Mutex
	requests []ebsvc.DescribeConfigurationSettingsInput
}

func newW5EBFake() *w5EBFake {
	return &w5EBFake{
		options:   map[string][]ebtypes.ConfigurationOptionSetting{},
		optionErr: map[string]error{},
		causes:    map[string][]string{},
	}
}

func (f *w5EBFake) DescribeConfigurationSettings(_ context.Context, in *ebsvc.DescribeConfigurationSettingsInput, _ ...func(*ebsvc.Options)) (*ebsvc.DescribeConfigurationSettingsOutput, error) {
	env := ""
	if in != nil {
		f.mu.Lock()
		f.requests = append(f.requests, *in)
		f.mu.Unlock()
		if in.EnvironmentName != nil {
			env = *in.EnvironmentName
		}
	}
	if err, ok := f.optionErr[env]; ok {
		return nil, err
	}
	return &ebsvc.DescribeConfigurationSettingsOutput{
		ConfigurationSettings: []ebtypes.ConfigurationSettingsDescription{{
			ApplicationName: aws.String("acme-storefront"),
			EnvironmentName: aws.String(env),
			OptionSettings:  f.options[env],
		}},
	}, nil
}

func (f *w5EBFake) DescribeEnvironmentHealth(_ context.Context, in *ebsvc.DescribeEnvironmentHealthInput, _ ...func(*ebsvc.Options)) (*ebsvc.DescribeEnvironmentHealthOutput, error) {
	env := ""
	if in != nil && in.EnvironmentName != nil {
		env = *in.EnvironmentName
	}
	return &ebsvc.DescribeEnvironmentHealthOutput{Causes: f.causes[env]}, nil
}

var _ awsclient.ElasticBeanstalkAPI = (*w5EBFake)(nil)

func w5EBOption(namespace, name, value string) ebtypes.ConfigurationOptionSetting {
	return ebtypes.ConfigurationOptionSetting{
		Namespace:  aws.String(namespace),
		OptionName: aws.String(name),
		Value:      aws.String(value),
	}
}

// w5HealthyEBOptions is the option set for an environment with all three
// settings on. Each test overrides exactly one.
func w5HealthyEBOptions() []ebtypes.ConfigurationOptionSetting {
	return []ebtypes.ConfigurationOptionSetting{
		w5EBOption("aws:elasticbeanstalk:managedactions", "ManagedActionsEnabled", "true"),
		w5EBOption("aws:elasticbeanstalk:healthreporting:system", "SystemType", "enhanced"),
		w5EBOption("aws:elasticbeanstalk:cloudwatch:logs", "StreamLogs", "true"),
		w5EBOption("aws:autoscaling:asg", "MinSize", "2"),
	}
}

// w5SetEBOption replaces one option's value in a copy of the healthy set.
func w5SetEBOption(namespace, name, value string) []ebtypes.ConfigurationOptionSetting {
	out := w5HealthyEBOptions()
	for i := range out {
		if *out[i].Namespace == namespace && *out[i].OptionName == name {
			out[i].Value = aws.String(value)
		}
	}
	return out
}

// w5EBRes mirrors the eb fetcher: ID is the environment ID, Name the
// environment name the enricher addresses its calls by.
func w5EBRes(name string) resource.Resource {
	return resource.Resource{
		ID:   "e-" + name,
		Name: name,
		Fields: map[string]string{
			"environment_name": name,
			"application_name": "acme-storefront",
			"health":           "Green",
			"status":           "Ready",
		},
	}
}

func w5EnrichEB(t *testing.T, f *w5EBFake, rows ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichEBEnvironmentHealth(
		context.Background(),
		&awsclient.ServiceClients{ElasticBeanstalk: f},
		rows,
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	return res
}

// One DescribeConfigurationSettings call returns all three option values, so
// the enricher reads them once and emits three independent rows from the one
// response rather than calling three times.
func TestW5_EBReadsAllThreeSettingsFromOneCall(t *testing.T) {
	env := "acme-eb-bare"
	f := newW5EBFake()
	f.options[env] = []ebtypes.ConfigurationOptionSetting{
		w5EBOption("aws:elasticbeanstalk:managedactions", "ManagedActionsEnabled", "false"),
		w5EBOption("aws:elasticbeanstalk:healthreporting:system", "SystemType", "basic"),
		w5EBOption("aws:elasticbeanstalk:cloudwatch:logs", "StreamLogs", "false"),
	}

	res := w5EnrichEB(t, f, w5EBRes(env))
	id := "e-" + env
	for _, code := range []string{"eb.managed-updates-off", "eb.enhanced-health-off", "eb.cloudwatch-logs-off"} {
		if _, ok := w2Find(res.Findings[id], code); !ok {
			t.Errorf("no finding %q; got %v", code, w2Codes(res.Findings[id]))
		}
	}
	f.mu.Lock()
	calls := len(f.requests)
	f.mu.Unlock()
	if calls != 1 {
		t.Errorf("DescribeConfigurationSettings called %d times for one environment; the response carries all three options, so once is enough", calls)
	}
}

// An environment with managed platform updates off never picks up a patched
// platform version on its own.
func TestW5_EBManagedUpdatesOff_Positive(t *testing.T) {
	env := "acme-eb-unmanaged"
	f := newW5EBFake()
	f.options[env] = w5SetEBOption("aws:elasticbeanstalk:managedactions", "ManagedActionsEnabled", "false")

	res := w5EnrichEB(t, f, w5EBRes(env))
	w2AssertFinding(t, res.Findings["e-"+env], "eb.managed-updates-off",
		"managed platform updates off", domain.SevWarn, "wave2")
}

func TestW5_EBManagedUpdatesOff_EnabledIsHealthy(t *testing.T) {
	env := "acme-eb-green"
	f := newW5EBFake()
	f.options[env] = w5HealthyEBOptions()

	res := w5EnrichEB(t, f, w5EBRes(env))
	w2AssertNoCode(t, res.Findings["e-"+env], "eb.managed-updates-off")
}

// Basic health reporting reports only whether instances respond, not why an
// application is failing.
func TestW5_EBEnhancedHealthOff_Positive(t *testing.T) {
	env := "acme-eb-basic-health"
	f := newW5EBFake()
	f.options[env] = w5SetEBOption("aws:elasticbeanstalk:healthreporting:system", "SystemType", "basic")

	res := w5EnrichEB(t, f, w5EBRes(env))
	w2AssertFinding(t, res.Findings["e-"+env], "eb.enhanced-health-off",
		"enhanced health reporting off", domain.SevWarn, "wave2")

	rows := w2Rows(t, res, "e-"+env, "eb.enhanced-health-off")
	var got []string
	for _, r := range rows {
		got = append(got, r.Label+": "+r.Value)
		if r.Value == "basic" {
			return
		}
	}
	t.Errorf("no supporting row naming the reporting level in use; got %v", got)
}

func TestW5_EBEnhancedHealthOff_EnhancedIsHealthy(t *testing.T) {
	env := "acme-eb-green"
	f := newW5EBFake()
	f.options[env] = w5HealthyEBOptions()

	res := w5EnrichEB(t, f, w5EBRes(env))
	w2AssertNoCode(t, res.Findings["e-"+env], "eb.enhanced-health-off")
}

// Without log streaming the instance logs go away with the instance, which
// is exactly when they are wanted.
func TestW5_EBCloudWatchLogsOff_Positive(t *testing.T) {
	env := "acme-eb-no-logs"
	f := newW5EBFake()
	f.options[env] = w5SetEBOption("aws:elasticbeanstalk:cloudwatch:logs", "StreamLogs", "false")

	res := w5EnrichEB(t, f, w5EBRes(env))
	w2AssertFinding(t, res.Findings["e-"+env], "eb.cloudwatch-logs-off",
		"log streaming to CloudWatch off", domain.SevWarn, "wave2")
}

func TestW5_EBCloudWatchLogsOff_StreamingIsHealthy(t *testing.T) {
	env := "acme-eb-green"
	f := newW5EBFake()
	f.options[env] = w5HealthyEBOptions()

	res := w5EnrichEB(t, f, w5EBRes(env))
	w2AssertNoCode(t, res.Findings["e-"+env], "eb.cloudwatch-logs-off")
}

// An option AWS did not return is unknown. All three settings have a default
// that differs from the flagged value, so reading an absent option as the
// bad one would flag environments nobody has misconfigured.
func TestW5_EBAbsentOptionIsNotAFinding(t *testing.T) {
	env := "acme-eb-silent"
	f := newW5EBFake()
	f.options[env] = []ebtypes.ConfigurationOptionSetting{w5EBOption("aws:autoscaling:asg", "MinSize", "2")}

	res := w5EnrichEB(t, f, w5EBRes(env))
	id := "e-" + env
	for _, code := range []string{"eb.managed-updates-off", "eb.enhanced-health-off", "eb.cloudwatch-logs-off"} {
		w2AssertNoCode(t, res.Findings[id], code)
	}
}

// The option lookup is by namespace AND option name. An environment carrying
// an option of the same name under a different namespace must not satisfy or
// trip the check.
func TestW5_EBOptionLookupIsNamespaceScoped(t *testing.T) {
	env := "acme-eb-lookalike"
	f := newW5EBFake()
	f.options[env] = []ebtypes.ConfigurationOptionSetting{
		w5EBOption("aws:elasticbeanstalk:managedactions", "ManagedActionsEnabled", "true"),
		w5EBOption("aws:elasticbeanstalk:healthreporting:system", "SystemType", "enhanced"),
		w5EBOption("aws:elasticbeanstalk:cloudwatch:logs", "StreamLogs", "true"),
		// Same option names, wrong namespaces. A lookup that matched on the
		// name alone would read these and report the environment broken.
		w5EBOption("aws:elasticbeanstalk:application:environment", "StreamLogs", "false"),
		w5EBOption("aws:elasticbeanstalk:application:environment", "SystemType", "basic"),
		w5EBOption("aws:elasticbeanstalk:application:environment", "ManagedActionsEnabled", "false"),
	}

	res := w5EnrichEB(t, f, w5EBRes(env))
	id := "e-" + env
	for _, code := range []string{"eb.managed-updates-off", "eb.enhanced-health-off", "eb.cloudwatch-logs-off"} {
		w2AssertNoCode(t, res.Findings[id], code)
	}
}

// One environment's DescribeConfigurationSettings failing marks that one
// unknown and leaves the rest evaluated.
func TestW5_EB_APIErrorOnOneEnvironmentTruncatesOnlyThatEnvironment(t *testing.T) {
	broken, unmanaged := "acme-eb-denied", "acme-eb-unmanaged"
	f := newW5EBFake()
	f.optionErr[broken] = errors.New("InsufficientPrivilegesException: not authorized to perform elasticbeanstalk:DescribeConfigurationSettings")
	f.options[unmanaged] = w5SetEBOption("aws:elasticbeanstalk:managedactions", "ManagedActionsEnabled", "false")

	res, _ := awsclient.EnrichEBEnvironmentHealth(
		context.Background(),
		&awsclient.ServiceClients{ElasticBeanstalk: f},
		[]resource.Resource{w5EBRes(broken), w5EBRes(unmanaged)},
		nil,
	)
	w2AssertEnricherShape(t, res)

	if _, marked := res.TruncatedIDs["e-"+broken]; !marked {
		t.Errorf("TruncatedIDs[%q] = false; an environment whose settings could not be read is unknown", broken)
	}
	for _, code := range []string{"eb.managed-updates-off", "eb.enhanced-health-off", "eb.cloudwatch-logs-off"} {
		w2AssertNoCode(t, res.Findings["e-"+broken], code)
	}
	w2AssertFinding(t, res.Findings["e-"+unmanaged], "eb.managed-updates-off",
		"managed platform updates off", domain.SevWarn, "wave2")
}

// The pre-existing causes finding must survive the addition.
func TestW5_EBNewFindingsDoNotDisplaceTheCausesFinding(t *testing.T) {
	env := "acme-eb-red"
	f := newW5EBFake()
	f.causes[env] = []string{"ELB processes are not healthy on all instances."}
	f.options[env] = w5SetEBOption("aws:elasticbeanstalk:cloudwatch:logs", "StreamLogs", "false")

	res := w5EnrichEB(t, f, w5EBRes(env))
	id := "e-" + env
	if _, ok := w2Find(res.Findings[id], "eb.environment-causes"); !ok {
		t.Errorf("the pre-existing causes finding is gone; got %v", w2Codes(res.Findings[id]))
	}
	if _, ok := w2Find(res.Findings[id], "eb.cloudwatch-logs-off"); !ok {
		t.Errorf("no eb.cloudwatch-logs-off alongside it; got %v", w2Codes(res.Findings[id]))
	}
}

func TestW5_EB_CapPlusOne(t *testing.T) {
	f := newW5EBFake()
	var rows []resource.Resource
	for i := range awsclient.EnrichmentCap + 1 {
		env := fmt.Sprintf("acme-eb-%03d", i)
		f.options[env] = w5SetEBOption("aws:elasticbeanstalk:cloudwatch:logs", "StreamLogs", "false")
		rows = append(rows, w5EBRes(env))
	}

	res := w5EnrichEB(t, f, rows...)
	if len(res.Findings) > awsclient.EnrichmentCap {
		t.Errorf("%d environments carry findings with a cap of %d; the enricher inspected past its own bound",
			len(res.Findings), awsclient.EnrichmentCap)
	}
	if len(res.Findings) == 0 {
		t.Error("no environment carries a finding; the cap must bound the work, not eliminate it")
	}
}

func TestW5_EB_NilClientReturnsEmptyResult(t *testing.T) {
	res, err := awsclient.EnrichEBEnvironmentHealth(
		context.Background(),
		&awsclient.ServiceClients{},
		[]resource.Resource{w5EBRes("acme-eb-green")},
		nil,
	)
	w2AssertEnricherInvariants(t, res, err)
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v on a session with no Elastic Beanstalk client", res.Findings)
	}
}

func TestW5_EBCatalogDefs(t *testing.T) {
	w2AssertFindingDef(t, "eb", "eb.managed-updates-off",
		"managed platform updates off", domain.SevWarn, "wave2")
	w2AssertFindingDef(t, "eb", "eb.enhanced-health-off",
		"enhanced health reporting off", domain.SevWarn, "wave2")
	w2AssertFindingDef(t, "eb", "eb.cloudwatch-logs-off",
		"log streaming to CloudWatch off", domain.SevWarn, "wave2")
}

// Rule 4: a resource being torn down emits no posture finding. The demo
// terminated environment escaped only because its fixture happened to carry
// no configuration settings, so nothing in the code was stopping it — the
// row below is the one the old code would have coloured, built on purpose:
// a Terminating and a Terminated environment whose settings trip all three
// conditions at once.
func TestW5_EBLifecycleEndedEmitsNoConfigurationFinding(t *testing.T) {
	for _, status := range []string{"Terminating", "Terminated"} {
		t.Run(status, func(t *testing.T) {
			env := "acme-eb-going-away"
			f := newW5EBFake()
			f.options[env] = []ebtypes.ConfigurationOptionSetting{
				w5EBOption("aws:elasticbeanstalk:managedactions", "ManagedActionsEnabled", "false"),
				w5EBOption("aws:elasticbeanstalk:healthreporting:system", "SystemType", "basic"),
				w5EBOption("aws:elasticbeanstalk:cloudwatch:logs", "StreamLogs", "false"),
			}

			r := w5EBRes(env)
			r.Fields["status"] = status

			res := w5EnrichEB(t, f, r)
			id := "e-" + env
			for _, code := range []string{"eb.managed-updates-off", "eb.enhanced-health-off", "eb.cloudwatch-logs-off"} {
				w2AssertNoCode(t, res.Findings[id], code)
			}
		})
	}
}

// The guard must be scoped to the ended states, not to anything that merely
// reads as unhealthy: an environment still running with a red health bar is
// exactly the one whose configuration an operator wants reported.
func TestW5_EBRunningEnvironmentStillReportsConfiguration(t *testing.T) {
	for _, status := range []string{"Ready", "Updating", "Launching"} {
		t.Run(status, func(t *testing.T) {
			env := "acme-eb-alive"
			f := newW5EBFake()
			f.options[env] = w5SetEBOption("aws:elasticbeanstalk:cloudwatch:logs", "StreamLogs", "false")

			r := w5EBRes(env)
			r.Fields["status"] = status
			r.Fields["health"] = "Red"

			res := w5EnrichEB(t, f, r)
			w2AssertFinding(t, res.Findings["e-"+env], "eb.cloudwatch-logs-off",
				"log streaming to CloudWatch off", domain.SevWarn, "wave2")
		})
	}
}
