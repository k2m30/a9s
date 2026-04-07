package unit

// Tests for §2.1 verb classification table.
//
// TestClassifyCTVerb_V2Table is a single table-driven test covering every entry
// in the §2.1 verb table plus the bug-fix cases (BatchGetImage, Decrypt, Encrypt,
// Sign, ReEncrypt, GenerateDataKey*).
//
// Expected to FAIL for the bug-fix cases until the P1 coder updates ClassifyCTVerb
// in internal/aws/ct_events.go (currently Batch* → W, Decrypt → W, etc.).

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

func TestClassifyCTVerb_V2Table(t *testing.T) {
	// Spec: §2.1 verb table — order matters, first match wins.
	cases := []struct {
		name          string
		eventName     string
		eventCategory string
		eventType     string
		want          string
	}{
		// ---------------------------------------------------------------
		// Destructive prefixes — §2.1 D row
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
		// Read prefixes — §2.1 R row (first block)
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
		// Bug-fix cases per §2.1 — currently misclassified in production code
		// ---------------------------------------------------------------
		// BatchGet* → R (was W because "Batch" prefix hit writePrefixes first)
		{"BatchGetImage_R", "BatchGetImage", "", "", "R"},
		{"BatchGetSecretValue_R", "BatchGetSecretValue", "", "", "R"},
		{"BatchGetItem_R", "BatchGetItem", "", "", "R"},
		// KMS use-key ops → R (no resource mutation per §2.1 note + §10 decision #5)
		{"Decrypt_R", "Decrypt", "", "", "R"},
		{"Encrypt_R", "Encrypt", "", "", "R"},
		{"Sign_R", "Sign", "", "", "R"},
		{"ReEncrypt_R", "ReEncrypt", "", "", "R"},
		{"GenerateDataKey_R", "GenerateDataKey", "", "", "R"},
		{"GenerateDataKeyWithoutPlaintext_R", "GenerateDataKeyWithoutPlaintext", "", "", "R"},

		// ---------------------------------------------------------------
		// §1.4 exact-match overrides — AssumeRoleWithWebIdentity is R, not W.
		// AssumeRole and AssumeRoleWithSAML keep their Assume* → W classification.
		// ---------------------------------------------------------------
		// AssumeRoleWithWebIdentity: exact-match R (IRSA/OIDC — not a write op)
		{"AssumeRoleWithWebIdentity_R", "AssumeRoleWithWebIdentity", "", "", "R"},
		// AssumeRole: still W via "Assume" prefix (human/automation cross-role)
		{"AssumeRole_W", "AssumeRole", "", "", "W"},
		// AssumeRoleWithSAML: still W via "Assume" prefix (enterprise federation)
		{"AssumeRoleWithSAML_W", "AssumeRoleWithSAML", "", "", "W"},

		// ---------------------------------------------------------------
		// Write prefixes — §2.1 W row
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
		// BatchWriteItem: BatchGet prefix → R, but BatchWrite hits W ("Batch" prefix is W,
		// but §2.1 says BatchGet → R and other Batch* fall through to W/D normally).
		// "BatchWriteItem" has prefix "Batch" (W) but does NOT have prefix "BatchGet" (R),
		// so it stays W.
		{"BatchWriteItem_W", "BatchWriteItem", "", "", "W"},

		// ---------------------------------------------------------------
		// Category-based verbs — §2.1
		// ---------------------------------------------------------------
		{"Insight_I", "ApiCallRateInsight", "Insight", "", "I"},
		{"NetworkActivity_N", "VpcEndpointAccess", "NetworkActivity", "", "N"},
		{"AwsServiceEvent_S", "InvokeExecution", "", "AwsServiceEvent", "S"},

		// ---------------------------------------------------------------
		// Unknown verb — §2.1 fallback
		// ---------------------------------------------------------------
		{"Unknown_Question", "FrobnicateWidgets", "", "", "?"},
	}

	for _, c := range cases {
		c := c // capture
		t.Run(c.name, func(t *testing.T) {
			got := awsclient.ClassifyCTVerb(c.eventName, c.eventCategory, c.eventType)
			if got != c.want {
				t.Errorf("ClassifyCTVerb(%q, %q, %q) = %q, want %q per §2.1",
					c.eventName, c.eventCategory, c.eventType, got, c.want)
			}
		})
	}
}
