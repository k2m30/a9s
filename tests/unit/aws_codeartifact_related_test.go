package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	catypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_Codeartifact_CB_ReturnsUnknown was deleted: codeartifact→cb is in
// the Explicitly excluded list (unanimous sometimes — no first-class AWS field).
// See docs/related-resources.md "Explicitly excluded" section.

// TestRelated_Codeartifact_Registered was deleted: the only registered pair
// for codeartifact (cb) has been dropped. The remaining codeartifact→kms
// registration is tested via the golden contract tests.

// codeartifactCheckerByTarget returns the RelatedChecker for the given target
// type registered under "codeartifact". Fails immediately if not found or nil.
func codeartifactCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("codeartifact") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("codeartifact related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("codeartifact related checker for %s not found", target)
	return nil
}

// ---------------------------------------------------------------------------
// checkCodeartifactKMS — Pattern C: DescribeDomain call to get EncryptionKey
// ---------------------------------------------------------------------------

// TestRelated_Codeartifact_KMS_Match verifies that a repository with a domain
// name, and a fake that returns an EncryptionKey ARN, yields Count=1.
func TestRelated_Codeartifact_KMS_Match(t *testing.T) {
	const keyARN = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-1234-5678-abcd-111111111111"

	src := resource.Resource{
		ID: "my-repo",
		RawStruct: catypes.RepositorySummary{
			Name:       aws.String("my-repo"),
			DomainName: aws.String("my-domain"),
		},
	}
	clients := &awsclient.ServiceClients{
		CodeArtifact: newFakeCodeArtifactWithKMSKey(keyARN),
	}
	checker := codeartifactCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 {
		t.Fatalf("ResourceIDs = %v, want 1 entry", result.ResourceIDs)
	}
	// arnLastSegment extracts the UUID portion after the last "/".
	if result.ResourceIDs[0] != "a1b2c3d4-1234-5678-abcd-111111111111" {
		t.Errorf("ResourceIDs[0] = %q, want key UUID", result.ResourceIDs[0])
	}
}

// TestRelated_Codeartifact_KMS_Empty verifies that a repository with an empty
// DomainName returns Count=0 without calling the API.
func TestRelated_Codeartifact_KMS_Empty(t *testing.T) {
	src := resource.Resource{
		ID: "my-repo",
		RawStruct: catypes.RepositorySummary{
			Name:       aws.String("my-repo"),
			DomainName: aws.String(""),
		},
	}
	checker := codeartifactCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty domain name)", result.Count)
	}
}

// TestRelated_Codeartifact_KMS_WrongRawStruct verifies that a resource with a
// non-RepositorySummary RawStruct returns Count=-1 (assertStruct fails).
func TestRelated_Codeartifact_KMS_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "my-repo",
		RawStruct: "not-a-repository-summary",
	}
	checker := codeartifactCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

