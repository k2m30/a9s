package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

func TestClassifyCTVerb_V2Table(t *testing.T) {
	// Order matters: first match wins.
	cases := []struct {
		name          string
		eventName     string
		eventCategory string
		eventType     string
		want          string
	}{
		// ---------------------------------------------------------------
		// Destructive prefixes
		// ---------------------------------------------------------------
		{"DeleteBucket", "DeleteBucket", "", "", "D"},
		{"TerminateInstances", "TerminateInstances", "", "", "D"},
		{"RevokeSecurityGroupIngress", "RevokeSecurityGroupIngress", "", "", "D"},
		{"DisassociateAddress", "DisassociateAddress", "", "", "D"},
		{"DeregisterInstance", "DeregisterInstance", "", "", "D"},
		{"DisableLogging", "DisableLogging", "", "", "D"},
		{"StopInstances", "StopInstances", "", "", "D"},
		{"DetachVolume", "DetachVolume", "", "", "D"},
		{"CancelJob", "CancelJob", "", "", "D"},
		{"RejectInvitation", "RejectInvitation", "", "", "D"},
		{"AbortMultipartUpload", "AbortMultipartUpload", "", "", "D"},
		{"PurgeQueue", "PurgeQueue", "", "", "D"},
		{"RemoveTags", "RemoveTags", "", "", "D"},
		{"DestroyCluster", "DestroyCluster", "", "", "D"},

		// ---------------------------------------------------------------
		// Read prefixes
		// ---------------------------------------------------------------
		{"GetObject", "GetObject", "", "", "R"},
		{"DescribeInstances", "DescribeInstances", "", "", "R"},
		{"ListBuckets", "ListBuckets", "", "", "R"},
		{"LookupEvents", "LookupEvents", "", "", "R"},
		{"SearchFaces", "SearchFaces", "", "", "R"},
		{"QueryObjects", "QueryObjects", "", "", "R"},
		{"ScanTable", "ScanTable", "", "", "R"},
		{"HeadObject", "HeadObject", "", "", "R"},
		{"TestConnection", "TestConnection", "", "", "R"},
		{"CheckHealth", "CheckHealth", "", "", "R"},
		{"ValidateTemplate", "ValidateTemplate", "", "", "R"},
		{"VerifySignature", "VerifySignature", "", "", "R"},

		// ---------------------------------------------------------------
		// BatchGet* and KMS use-key ops
		// BatchGet* → R even though "Batch" is a write prefix
		{"BatchGetImage_R", "BatchGetImage", "", "", "R"},
		{"BatchGetSecretValue_R", "BatchGetSecretValue", "", "", "R"},
		{"BatchGetItem_R", "BatchGetItem", "", "", "R"},
		// KMS use-key ops → R (no resource mutation)
		{"Decrypt_R", "Decrypt", "", "", "R"},
		{"Encrypt_R", "Encrypt", "", "", "R"},
		{"Sign_R", "Sign", "", "", "R"},
		{"ReEncrypt_R", "ReEncrypt", "", "", "R"},
		{"GenerateDataKey_R", "GenerateDataKey", "", "", "R"},
		{"GenerateDataKeyWithoutPlaintext_R", "GenerateDataKeyWithoutPlaintext", "", "", "R"},

		// ---------------------------------------------------------------
		// Exact-match overrides — all AssumeRole* ops are R (STS session-vending).
		// Identity exchange, not state mutation. Exact-matched before the W prefix table runs.
		// ---------------------------------------------------------------
		// AssumeRoleWithWebIdentity: exact-match R (IRSA/OIDC — not a write op)
		{"AssumeRoleWithWebIdentity_R", "AssumeRoleWithWebIdentity", "", "", "R"},
		// AssumeRole: exact-match R (STS session-vending — identity exchange, not state mutation)
		{"AssumeRole_R", "AssumeRole", "", "", "R"},
		// AssumeRoleWithSAML: exact-match R (SAML federation — STS session-vending, not state mutation)
		{"AssumeRoleWithSAML_R", "AssumeRoleWithSAML", "", "", "R"},

		// ---------------------------------------------------------------
		// Write prefixes
		// ---------------------------------------------------------------
		{"CreateBucket", "CreateBucket", "", "", "W"},
		{"PutObject", "PutObject", "", "", "W"},
		{"UpdateFunctionCode", "UpdateFunctionCode", "", "", "W"},
		{"ModifyInstanceAttribute", "ModifyInstanceAttribute", "", "", "W"},
		{"SetBucketPolicy", "SetBucketPolicy", "", "", "W"},
		{"AddUserToGroup", "AddUserToGroup", "", "", "W"},
		{"AttachVolume", "AttachVolume", "", "", "W"},
		{"AssociateAddress", "AssociateAddress", "", "", "W"},
		{"RegisterInstance", "RegisterInstance", "", "", "W"},
		{"EnableLogging", "EnableLogging", "", "", "W"},
		{"StartInstances", "StartInstances", "", "", "W"},
		{"RunInstances", "RunInstances", "", "", "W"},
		{"RebootInstances", "RebootInstances", "", "", "W"},
		{"TagResource", "TagResource", "", "", "W"},
		// BatchWriteItem has prefix "Batch" (W) but not "BatchGet" (R), so it stays W.
		{"BatchWriteItem_W", "BatchWriteItem", "", "", "W"},

		// ---------------------------------------------------------------
		// Category-based verbs
		// ---------------------------------------------------------------
		{"Insight_I", "ApiCallRateInsight", "Insight", "", "I"},
		{"NetworkActivity_N", "VpcEndpointAccess", "NetworkActivity", "", "N"},
		{"AwsServiceEvent_S", "InvokeExecution", "", "AwsServiceEvent", "S"},

		// ---------------------------------------------------------------
		// Unknown verb fallback
		// ---------------------------------------------------------------
		{"Unknown_Question", "FrobnicateWidgets", "", "", "?"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ctevent.ClassifyCTVerb(c.eventName, c.eventCategory, c.eventType)
			if got != c.want {
				t.Errorf("ClassifyCTVerb(%q, %q, %q) = %q, want %q per §2.1",
					c.eventName, c.eventCategory, c.eventType, got, c.want)
			}
		})
	}
}
