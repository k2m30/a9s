package unit

// aws_codeartifact_enricher_test.go — Behavioral tests for EnrichCodeArtifactRepository.
//
// Contract assertions:
//   - GetRepositoryPermissionsPolicy is called once per CodeArtifact resource keyed by repo name
//     (domain taken from Fields["domain"]).
//   - Both repos have a policy with a specific (non-wildcard) principal → 0 findings.
//   - repo-1 returns ResourceNotFoundException → 1 finding sev "~" "no permissions policy".
//   - repo-1 policy Document contains "Principal":"*" → 1 finding sev "!" "public access".
//   - clients.CodeArtifact == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - Generic API error for a resource → 0 findings for that resource, Truncated=true, no error.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	codeartifactsvc "github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// codeArtifactPermPolicyFake implements CodeArtifactAPI for enrichment testing.
// It embeds the aggregate interface and overrides only GetRepositoryPermissionsPolicy.
// The results map is keyed by "<domain>/<repository>" so the fake can serve different
// responses per resource. errByKey overrides results when set.
type codeArtifactPermPolicyFake struct {
	awsclient.CodeArtifactAPI
	// results maps "<domain>/<repo>" → ResourcePolicy.
	results map[string]*codeartifacttypes.ResourcePolicy
	// errByKey maps "<domain>/<repo>" → error; overrides results when set.
	errByKey map[string]error
}

func (f *codeArtifactPermPolicyFake) GetRepositoryPermissionsPolicy(
	_ context.Context,
	in *codeartifactsvc.GetRepositoryPermissionsPolicyInput,
	_ ...func(*codeartifactsvc.Options),
) (*codeartifactsvc.GetRepositoryPermissionsPolicyOutput, error) {
	domain := ""
	repo := ""
	if in != nil {
		if in.Domain != nil {
			domain = *in.Domain
		}
		if in.Repository != nil {
			repo = *in.Repository
		}
	}
	key := domain + "/" + repo
	if f.errByKey != nil {
		if err, ok := f.errByKey[key]; ok {
			return nil, err
		}
	}
	policy, ok := f.results[key]
	if !ok {
		return &codeartifactsvc.GetRepositoryPermissionsPolicyOutput{}, nil
	}
	return &codeartifactsvc.GetRepositoryPermissionsPolicyOutput{Policy: policy}, nil
}

// Compile-time check: codeArtifactPermPolicyFake satisfies CodeArtifactAPI.
var _ awsclient.CodeArtifactAPI = (*codeArtifactPermPolicyFake)(nil)

// codeArtifactRepoResources returns a slice of CodeArtifact Resource stubs with the given
// repo names. All resources share domain "my-domain" per the scope spec.
func codeArtifactRepoResources(names ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(names))
	for _, name := range names {
		res = append(res, resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"domain":          "my-domain",
				"repository_name": name,
				"format":          "npm",
				"description":     "test repository " + name,
			},
		})
	}
	return res
}

// codeArtifactResourcePolicy builds a ResourcePolicy with the given JSON document string.
func codeArtifactResourcePolicy(document string) *codeartifacttypes.ResourcePolicy {
	return &codeartifacttypes.ResourcePolicy{
		Document: aws.String(document),
	}
}

const (
	caRepo1  = "my-npm-registry"
	caRepo2  = "my-pypi-registry"
	caDomain = "my-domain"
	// specificPrincipalPolicy is a minimal policy granting access to a specific account (not wildcard).
	specificPrincipalPolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"codeartifact:GetPackageVersionAsset","Resource":"*"}]}`
	// wildcardPrincipalPolicy is a policy with Principal:* (public access).
	wildcardPrincipalPolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"codeartifact:GetPackageVersionAsset","Resource":"*"}]}`
)

// TestEnrichCodeArtifactRepository_GoodPolicyProducesNoFindings verifies that when both
// repositories have a policy with a specific (non-wildcard) principal, the enricher
// produces 0 findings and IssueCount=0.
func TestEnrichCodeArtifactRepository_GoodPolicyProducesNoFindings(t *testing.T) {
	fake := &codeArtifactPermPolicyFake{
		results: map[string]*codeartifacttypes.ResourcePolicy{
			caDomain + "/" + caRepo1: codeArtifactResourcePolicy(specificPrincipalPolicy),
			caDomain + "/" + caRepo2: codeArtifactResourcePolicy(specificPrincipalPolicy),
		},
	}
	clients := &awsclient.ServiceClients{CodeArtifact: fake}
	resources := codeArtifactRepoResources(caRepo1, caRepo2)

	result, err := awsclient.EnrichCodeArtifactRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichCodeArtifactRepository_NoPolicyProducesFindingSevTilde verifies that when
// repo-1 returns ResourceNotFoundException (no policy configured), a finding with
// severity "~" and a summary containing "no permissions policy" is produced for repo-1
// only. repo-2 has a good policy and produces no finding.
func TestEnrichCodeArtifactRepository_NoPolicyProducesFindingSevTilde(t *testing.T) {
	notFoundErr := &codeartifacttypes.ResourceNotFoundException{
		Message: aws.String("Repository " + caRepo1 + " does not have a resource policy"),
	}
	fake := &codeArtifactPermPolicyFake{
		errByKey: map[string]error{
			caDomain + "/" + caRepo1: notFoundErr,
		},
		results: map[string]*codeartifacttypes.ResourcePolicy{
			caDomain + "/" + caRepo2: codeArtifactResourcePolicy(specificPrincipalPolicy),
		},
	}
	clients := &awsclient.ServiceClients{CodeArtifact: fake}
	resources := codeArtifactRepoResources(caRepo1, caRepo2)

	result, err := awsclient.EnrichCodeArtifactRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[caRepo1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no policy)", caRepo1)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if !strings.Contains(strings.ToLower(f.Phrase), "no permissions policy") {
		t.Errorf("summary %q must contain \"no permissions policy\"", f.Phrase)
	}
	if _, ok := result.Findings[caRepo2]; ok {
		t.Error("repo-2 must NOT appear in Findings — it has a valid policy")
	}
	// "~" severity does NOT contribute to IssueCount per the EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichCodeArtifactRepository_PublicPolicyProducesFindingSevBang verifies that when
// repo-1 has a policy with Principal:"*" (public access), a finding with severity "!"
// and a summary containing "public access" is produced for repo-1 only.
func TestEnrichCodeArtifactRepository_PublicPolicyProducesFindingSevBang(t *testing.T) {
	fake := &codeArtifactPermPolicyFake{
		results: map[string]*codeartifacttypes.ResourcePolicy{
			caDomain + "/" + caRepo1: codeArtifactResourcePolicy(wildcardPrincipalPolicy),
			caDomain + "/" + caRepo2: codeArtifactResourcePolicy(specificPrincipalPolicy),
		},
	}
	clients := &awsclient.ServiceClients{CodeArtifact: fake}
	resources := codeArtifactRepoResources(caRepo1, caRepo2)

	result, err := awsclient.EnrichCodeArtifactRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[caRepo1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (public access)", caRepo1)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
	if !strings.Contains(strings.ToLower(f.Phrase), "public access") {
		t.Errorf("summary %q must contain \"public access\"", f.Phrase)
	}
	if _, ok := result.Findings[caRepo2]; ok {
		t.Error("repo-2 must NOT appear in Findings — it has a specific principal")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichCodeArtifactRepository_NilClientReturnsEmptyFindingsNoError verifies that
// when clients.CodeArtifact is nil the enricher returns a non-nil empty Findings map
// and no error.
func TestEnrichCodeArtifactRepository_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{CodeArtifact: nil}

	result, err := awsclient.EnrichCodeArtifactRepository(context.Background(), clients, codeArtifactRepoResources(caRepo1, caRepo2), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when CodeArtifact client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichCodeArtifactRepository_APIErrorSetsTruncatedNoError verifies that when the
// API call for repo-1 returns a generic error (not ResourceNotFoundException), the
// enricher sets Truncated=true, produces 0 findings for that repo, and does not
// propagate the error.
func TestEnrichCodeArtifactRepository_APIErrorSetsTruncatedNoError(t *testing.T) {
	apiErr := errors.New("codeartifact: GetRepositoryPermissionsPolicy throttled")
	fake := &codeArtifactPermPolicyFake{
		errByKey: map[string]error{
			caDomain + "/" + caRepo1: apiErr,
		},
		results: map[string]*codeartifacttypes.ResourcePolicy{
			caDomain + "/" + caRepo2: codeArtifactResourcePolicy(specificPrincipalPolicy),
		},
	}
	clients := &awsclient.ServiceClients{CodeArtifact: fake}
	resources := codeArtifactRepoResources(caRepo1, caRepo2)

	result, err := awsclient.EnrichCodeArtifactRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a generic API call fails")
	}
}
