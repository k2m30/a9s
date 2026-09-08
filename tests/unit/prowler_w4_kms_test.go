package unit

// prowler_w4_kms_test.go — behavioural test for the batch-w4 kms row: a
// customer-managed key whose key policy lets any principal use it.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w4CodeKMSPublicPolicy domain.FindingCode = "kms.public-policy"
	w4PhraseKMSPublic                        = "key policy open to anyone"
	w4SourceKMSWave2                         = "wave2"
)

// A key policy granting use to everyone, alongside the usual root statement.
const w4KMSPublicPolicyDoc = `{"Version":"2012-10-17","Id":"key-default-1","Statement":[` +
	`{"Sid":"Enable IAM User Permissions","Effect":"Allow",` +
	`"Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"kms:*","Resource":"*"},` +
	`{"Sid":"AllowAnyone","Effect":"Allow","Principal":{"AWS":"*"},` +
	`"Action":["kms:Encrypt","kms:Decrypt"],"Resource":"*"}]}`

// The default key policy: only the owning account's principals.
const w4KMSPrivatePolicyDoc = `{"Version":"2012-10-17","Id":"key-default-1","Statement":[` +
	`{"Sid":"Enable IAM User Permissions","Effect":"Allow",` +
	`"Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"kms:*","Resource":"*"}]}`

// A wildcard principal narrowed to the account that owns the key: the
// documented pattern for a key shared with account services, not a finding.
const w4KMSConditionedPolicyDoc = `{"Version":"2012-10-17","Id":"key-default-1","Statement":[` +
	`{"Sid":"AllowViaServices","Effect":"Allow","Principal":{"AWS":"*"},` +
	`"Action":["kms:Encrypt","kms:Decrypt"],"Resource":"*",` +
	`"Condition":{"StringEquals":{"kms:CallerAccount":"123456789012"}}}]}`

// w4KMSFake serves GetKeyRotationStatus and GetKeyPolicy, keyed by key id.
type w4KMSFake struct {
	awsclient.KMSAPI
	policies  map[string]string
	policyErr map[string]error
}

func (f *w4KMSFake) GetKeyRotationStatus(
	_ context.Context, _ *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options),
) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: true}, nil
}

func (f *w4KMSFake) GetKeyPolicy(
	_ context.Context, in *kms.GetKeyPolicyInput, _ ...func(*kms.Options),
) (*kms.GetKeyPolicyOutput, error) {
	id := aws.ToString(in.KeyId)
	if err, ok := f.policyErr[id]; ok {
		return nil, err
	}
	doc, ok := f.policies[id]
	if !ok {
		return nil, errors.New("NotFoundException: key policy not found")
	}
	return &kms.GetKeyPolicyOutput{PolicyName: aws.String("default"), Policy: aws.String(doc)}, nil
}

var _ awsclient.KMSAPI = (*w4KMSFake)(nil)

// w4KMSResource builds a key resource the way the kms fetcher does, carrying
// the typed metadata that says who manages the key.
func w4KMSResource(keyID, alias string, manager kmstypes.KeyManagerType) resource.Resource {
	return resource.Resource{
		ID: keyID, Name: alias, Type: "kms",
		Fields: map[string]string{"key_id": keyID, "alias": alias, "status": "Enabled"},
		RawStruct: kmstypes.KeyMetadata{
			KeyId:      aws.String(keyID),
			Arn:        aws.String("arn:aws:kms:us-east-1:123456789012:key/" + keyID),
			KeyManager: manager,
			KeyState:   kmstypes.KeyStateEnabled,
		},
	}
}

func w4EnrichKMS(t *testing.T, fake *w4KMSFake, rs []resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w4EnrichKMSErr(t, fake, rs)
	if err != nil {
		t.Fatalf("EnrichKMSRotation: %v", err)
	}
	return res
}

// w4EnrichKMSErr is w4EnrichKMS for the case that expects a refusal: a key
// policy the role may not read is a recorded failure ("skipped" spec row 5).
func w4EnrichKMSErr(t *testing.T, fake *w4KMSFake, rs []resource.Resource) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	clients := &awsclient.ServiceClients{KMS: fake, Region: "us-east-1"}
	return awsclient.EnrichKMSRotation(context.Background(), clients, rs, nil)
}

const (
	w4KMSPublicKeyID  = "1a2b3c4d-1111-2222-3333-444455556666"
	w4KMSPrivateKeyID = "9f8e7d6c-1111-2222-3333-444455556666"
)

// TestW4KMSPublicPolicy pins the open key: anyone can encrypt and decrypt
// with it. The key beside it, reachable only by the owning account, is not
// flagged.
func TestW4KMSPublicPolicy(t *testing.T) {
	fake := &w4KMSFake{policies: map[string]string{
		w4KMSPublicKeyID:  w4KMSPublicPolicyDoc,
		w4KMSPrivateKeyID: w4KMSPrivatePolicyDoc,
	}}
	res := w4EnrichKMS(t, fake, []resource.Resource{
		w4KMSResource(w4KMSPublicKeyID, "alias/acme-shared-key", kmstypes.KeyManagerTypeCustomer),
		w4KMSResource(w4KMSPrivateKeyID, "alias/acme-reports-key", kmstypes.KeyManagerTypeCustomer),
	})

	w4AssertFinding(t, res.Findings[w4KMSPublicKeyID], w4CodeKMSPublicPolicy,
		w4PhraseKMSPublic, domain.SevBroken, w4SourceKMSWave2)
	w4AssertRows(t, res.AttentionDetails[w4KMSPublicKeyID], w4CodeKMSPublicPolicy, []domain.DetailRow{
		{Label: "Principal", Value: "*"},
		{Label: "Actions", Value: "kms:Decrypt, kms:Encrypt"},
	})
	w4AssertNoCode(t, res.Findings[w4KMSPrivateKeyID], w4CodeKMSPublicPolicy)
}

// TestW4KMSConditionedPolicyIsNotPublic pins that a wildcard principal scoped
// by a condition is a scoped grant, not an open key.
func TestW4KMSConditionedPolicyIsNotPublic(t *testing.T) {
	fake := &w4KMSFake{policies: map[string]string{w4KMSPublicKeyID: w4KMSConditionedPolicyDoc}}
	res := w4EnrichKMS(t, fake, []resource.Resource{
		w4KMSResource(w4KMSPublicKeyID, "alias/acme-service-key", kmstypes.KeyManagerTypeCustomer),
	})
	w4AssertNoCode(t, res.Findings[w4KMSPublicKeyID], w4CodeKMSPublicPolicy)
}

// TestW4KMSSkipsAWSManagedKeys pins that AWS-managed keys are out of scope:
// the operator cannot edit their policy.
func TestW4KMSSkipsAWSManagedKeys(t *testing.T) {
	fake := &w4KMSFake{policies: map[string]string{w4KMSPublicKeyID: w4KMSPublicPolicyDoc}}
	res := w4EnrichKMS(t, fake, []resource.Resource{
		w4KMSResource(w4KMSPublicKeyID, "alias/aws/s3", kmstypes.KeyManagerTypeAws),
	})
	w4AssertNoCode(t, res.Findings[w4KMSPublicKeyID], w4CodeKMSPublicPolicy)
}

// TestW4KMSPolicyFetchFailureIsUnknown pins that a key whose policy could not
// be read is marked unknown, and the rest of the batch is still judged.
func TestW4KMSPolicyFetchFailureIsUnknown(t *testing.T) {
	fake := &w4KMSFake{
		policies:  map[string]string{w4KMSPublicKeyID: w4KMSPublicPolicyDoc},
		policyErr: map[string]error{w4KMSPrivateKeyID: errors.New("AccessDeniedException: kms:GetKeyPolicy")},
	}
	res, err := w4EnrichKMSErr(t, fake, []resource.Resource{
		w4KMSResource(w4KMSPrivateKeyID, "alias/acme-denied-key", kmstypes.KeyManagerTypeCustomer),
		w4KMSResource(w4KMSPublicKeyID, "alias/acme-shared-key", kmstypes.KeyManagerTypeCustomer),
	})

	// INVERTED for the "skipped" spec row 5: the helper failed the test on any
	// error, so a refused key policy was marked "?" and never explained.
	if err == nil {
		t.Error("a refused GetKeyPolicy returned no error")
	}
	if !res.TruncatedIDs[w4KMSPrivateKeyID] {
		t.Errorf("TruncatedIDs[%s] = false, want true", w4KMSPrivateKeyID)
	}
	w4AssertNoCode(t, res.Findings[w4KMSPrivateKeyID], w4CodeKMSPublicPolicy)
	w4AssertFinding(t, res.Findings[w4KMSPublicKeyID], w4CodeKMSPublicPolicy,
		w4PhraseKMSPublic, domain.SevBroken, w4SourceKMSWave2)
}

// TestW4KMSPublicPolicyBeyondCapIsTruncated pins the cap semantics: the
// kms enricher now emits a "!" finding, so a run that could not reach every
// key must report itself truncated rather than presenting a lower bound as a
// complete count.
func TestW4KMSPublicPolicyBeyondCapIsTruncated(t *testing.T) {
	policies := map[string]string{}
	rs := make([]resource.Resource, 0, awsclient.EnrichmentCap+1)
	for i := range awsclient.EnrichmentCap + 1 {
		id := w4KMSKeyID(i)
		policies[id] = w4KMSPublicPolicyDoc
		rs = append(rs, w4KMSResource(id, "alias/acme-key", kmstypes.KeyManagerTypeCustomer))
	}
	res := w4EnrichKMS(t, &w4KMSFake{policies: policies}, rs)

	if !res.Truncated {
		t.Errorf("Truncated = false with %d keys and a cap of %d", len(rs), awsclient.EnrichmentCap)
	}
}

func w4KMSKeyID(i int) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[i/16%16], hex[i%16]}) + "000000-1111-2222-3333-444455556666"
}

// TestW4KMSNilClient pins the nil-client contract.
func TestW4KMSNilClient(t *testing.T) {
	res, err := awsclient.EnrichKMSRotation(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{w4KMSResource(w4KMSPublicKeyID, "alias/acme-shared-key", kmstypes.KeyManagerTypeCustomer)}, nil)
	if err != nil {
		t.Fatalf("EnrichKMSRotation: %v", err)
	}
	if res.Findings == nil || res.TruncatedIDs == nil || res.FieldUpdates == nil {
		t.Fatalf("result maps must be non-nil")
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v, want empty", res.Findings)
	}
}

// TestW4KMSFindingDef pins the registry row for the new kms code.
func TestW4KMSFindingDef(t *testing.T) {
	def := w4FindingDef(t, "kms", w4CodeKMSPublicPolicy)
	if def.Phrase != w4PhraseKMSPublic {
		t.Errorf("Phrase = %q, want %q", def.Phrase, w4PhraseKMSPublic)
	}
	if def.Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want SevBroken", def.Severity)
	}
	if def.Source != "wave2" {
		t.Errorf("Source = %q, want wave2", def.Source)
	}
}

// TestW4KMSPendingDeletionEmitsNoPostureFinding pins that a key already
// scheduled for deletion is not reported for its key policy. The operator's
// action is to wait or cancel the deletion, not to edit a policy on a key
// that is going away.
func TestW4KMSPendingDeletionEmitsNoPostureFinding(t *testing.T) {
	pending := w4KMSResource(w4KMSPublicKeyID, "alias/acme-retired-key", kmstypes.KeyManagerTypeCustomer)
	pending.Fields["status"] = string(kmstypes.KeyStatePendingDeletion)
	pending.RawStruct = kmstypes.KeyMetadata{
		KeyId:      aws.String(w4KMSPublicKeyID),
		Arn:        aws.String("arn:aws:kms:us-east-1:123456789012:key/" + w4KMSPublicKeyID),
		KeyManager: kmstypes.KeyManagerTypeCustomer,
		KeyState:   kmstypes.KeyStatePendingDeletion,
	}
	fake := &w4KMSFake{policies: map[string]string{w4KMSPublicKeyID: w4KMSPublicPolicyDoc}}
	res := w4EnrichKMS(t, fake, []resource.Resource{pending})
	w4AssertNoCode(t, res.Findings[w4KMSPublicKeyID], w4CodeKMSPublicPolicy)
}
