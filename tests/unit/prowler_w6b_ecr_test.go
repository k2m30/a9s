package unit

// prowler_w6b_ecr_test.go — behavioural pins for the four ecr posture signals
// of batch w6b.
//
// Two waves, deliberately split by what DescribeRepositories already answers.
// scan-on-push and tag mutability are on the repository record the fetcher
// holds, so they are wave 1. The repository policy and the lifecycle policy
// each need their own call, so they belong to EnrichECRRepository — the type's
// single enricher, extended rather than duplicated.

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w6bECRCodeScanOnPushOff = domain.FindingCode("ecr.scan-on-push-off")
	w6bECRCodeMutableTags   = domain.FindingCode("ecr.mutable-tags")
	w6bECRCodePublicPolicy  = domain.FindingCode("ecr.public-policy")
	w6bECRCodeNoLifecycle   = domain.FindingCode("ecr.no-lifecycle-policy")
)

const (
	w6bECRPhraseScanOnPushOff = "scan on push off"
	w6bECRPhraseMutableTags   = "tags are mutable"
	w6bECRPhrasePublicPolicy  = "repository policy open to anyone"
	w6bECRPhraseNoLifecycle   = "no lifecycle policy"
	w6bECRSource              = "wave2:ecr"
)

// ─── wave 1: DescribeRepositories ───────────────────────────────────────────

type w6bECRListFake struct {
	repos []ecrtypes.Repository
}

func (f *w6bECRListFake) DescribeRepositories(_ context.Context, _ *ecr.DescribeRepositoriesInput, _ ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
	return &ecr.DescribeRepositoriesOutput{Repositories: f.repos}, nil
}

// w6bECRRepo builds a healthy repository: immutable tags and scan on push.
func w6bECRRepo(name string) ecrtypes.Repository {
	return ecrtypes.Repository{
		RepositoryName:             aws.String(name),
		RepositoryArn:              aws.String("arn:aws:ecr:us-east-1:123456789012:repository/" + name),
		RepositoryUri:              aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/" + name),
		RegistryId:                 aws.String("123456789012"),
		CreatedAt:                  aws.Time(time.Now().Add(-300 * 24 * time.Hour)),
		ImageTagMutability:         ecrtypes.ImageTagMutabilityImmutable,
		ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
	}
}

func w6bFetchECR(t *testing.T, repos ...ecrtypes.Repository) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchECRRepositoriesPage(context.Background(), &w6bECRListFake{repos: repos}, "")
	if err != nil {
		t.Fatalf("FetchECRRepositoriesPage: %v", err)
	}
	return out.Resources
}

// ─── row 8: ecr.scan-on-push-off ────────────────────────────────────────────

// Without scan on push, a vulnerable image is only discovered when someone
// asks — and nobody asks.
func TestW6BECR_ScanOnPushOff_Disabled(t *testing.T) {
	const name = "acme/base-images"
	repo := w6bECRRepo(name)
	repo.ImageScanningConfiguration = &ecrtypes.ImageScanningConfiguration{ScanOnPush: false}
	rs := w6bFetchECR(t, repo)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bECRCodeScanOnPushOff,
		w6bECRPhraseScanOnPushOff, domain.SevWarn, "wave1")
	w6bRequireNoRows(t, w6bWave1Rows(r, w6bECRCodeScanOnPushOff))
}

// An absent scanning configuration is the same exposure as an explicit false:
// AWS does not scan. This is the documented exception to "nil never triggers".
func TestW6BECR_ScanOnPushOff_ConfigurationAbsent(t *testing.T) {
	const name = "acme/legacy-images"
	repo := w6bECRRepo(name)
	repo.ImageScanningConfiguration = nil
	rs := w6bFetchECR(t, repo)
	pw1RequireFinding(t, pw1ResourceByID(t, rs, name).Findings,
		w6bECRCodeScanOnPushOff, w6bECRPhraseScanOnPushOff, domain.SevWarn, "wave1")
}

func TestW6BECR_ScanOnPushOn_IsHealthy(t *testing.T) {
	const name = "acme/frontend"
	rs := w6bFetchECR(t, w6bECRRepo(name))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bECRCodeScanOnPushOff)
}

// ─── row 9: ecr.mutable-tags ────────────────────────────────────────────────

// Mutable tags mean the image behind :v1.2.3 can be replaced after review, so
// what was scanned is not necessarily what runs.
func TestW6BECR_MutableTags_Mutable(t *testing.T) {
	const name = "acme/frontend"
	repo := w6bECRRepo(name)
	repo.ImageTagMutability = ecrtypes.ImageTagMutabilityMutable
	rs := w6bFetchECR(t, repo)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bECRCodeMutableTags,
		w6bECRPhraseMutableTags, domain.SevWarn, "wave1")
	// "Tag mutability: mutable" adds nothing an operator did not read in the
	// phrase, and the raw MUTABLE the table proposed is a banned enum.
	w6bRequireNoRows(t, w6bWave1Rows(r, w6bECRCodeMutableTags))
}

func TestW6BECR_ImmutableTags_IsHealthy(t *testing.T) {
	const name = "acme/base-images"
	rs := w6bFetchECR(t, w6bECRRepo(name))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bECRCodeMutableTags)
}

// IMMUTABLE_WITH_EXCLUSION still pins the tags that matter. Only the two
// mutable settings are the finding, so a switch on "not immutable" is wrong.
func TestW6BECR_ImmutableWithExclusion_IsHealthy(t *testing.T) {
	const name = "acme/exclusion-repo"
	repo := w6bECRRepo(name)
	repo.ImageTagMutability = ecrtypes.ImageTagMutabilityImmutableWithExclusion
	rs := w6bFetchECR(t, repo)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bECRCodeMutableTags)
}

// AWS omits the field on nothing today, but an empty enum must not be read as
// mutable: unknown is not misconfigured.
func TestW6BECR_TagMutabilityUnset_IsHealthy(t *testing.T) {
	const name = "acme/unknown-mutability"
	repo := w6bECRRepo(name)
	repo.ImageTagMutability = ""
	rs := w6bFetchECR(t, repo)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bECRCodeMutableTags)
}

// ─── wave 2: the enricher ───────────────────────────────────────────────────

// w6bECRFake answers the three calls EnrichECRRepository makes. Embedding the
// aggregate keeps the fake honest about which methods the enricher may use:
// anything not implemented here panics rather than silently returning zero.
type w6bECRFake struct {
	awsclient.ECRAPI
	policies   map[string]string // repo → repository policy document
	lifecycles map[string]string // repo → lifecycle policy document
	policyErr  map[string]error  // repo → error from GetRepositoryPolicy
	imagesErr  map[string]error  // repo → error from DescribeImages
}

func (f *w6bECRFake) DescribeImages(_ context.Context, in *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	if err, ok := f.imagesErr[aws.ToString(in.RepositoryName)]; ok {
		return nil, err
	}
	return &ecr.DescribeImagesOutput{}, nil
}

func (f *w6bECRFake) GetRepositoryPolicy(_ context.Context, in *ecr.GetRepositoryPolicyInput, _ ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error) {
	name := aws.ToString(in.RepositoryName)
	if err, ok := f.policyErr[name]; ok {
		return nil, err
	}
	doc, ok := f.policies[name]
	if !ok {
		return nil, &ecrtypes.RepositoryPolicyNotFoundException{Message: aws.String("no policy")}
	}
	return &ecr.GetRepositoryPolicyOutput{RepositoryName: in.RepositoryName, PolicyText: aws.String(doc)}, nil
}

func (f *w6bECRFake) GetLifecyclePolicy(_ context.Context, in *ecr.GetLifecyclePolicyInput, _ ...func(*ecr.Options)) (*ecr.GetLifecyclePolicyOutput, error) {
	name := aws.ToString(in.RepositoryName)
	doc, ok := f.lifecycles[name]
	if !ok {
		return nil, &ecrtypes.LifecyclePolicyNotFoundException{Message: aws.String("no lifecycle policy")}
	}
	return &ecr.GetLifecyclePolicyOutput{RepositoryName: in.RepositoryName, LifecyclePolicyText: aws.String(doc)}, nil
}

const w6bECRLifecycleDoc = `{"rules":[{"rulePriority":1,"description":"expire untagged after 14 days","selection":{"tagStatus":"untagged","countType":"sinceImagePushed","countUnit":"days","countNumber":14},"action":{"type":"expire"}}]}`

const w6bECRPublicPolicyDoc = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AnyonePull",
      "Effect": "Allow",
      "Principal": "*",
      "Action": ["ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer"]
    }
  ]
}`

// w6bECREnrich runs the type's registered enricher, which is how the app
// reaches it — going straight to EnrichECRRepository would not prove the
// catalog still points at it.
func w6bECREnrich(t *testing.T, fake *w6bECRFake, names ...string) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	rs := make([]resource.Resource, 0, len(names))
	for _, n := range names {
		rs = append(rs, resource.Resource{ID: n, Name: n, RawStruct: w6bECRRepo(n)})
	}
	res, err := w2Enricher(t, "ecr")(context.Background(), &awsclient.ServiceClients{ECR: fake}, rs, nil)
	w2AssertEnricherShape(t, res)
	return res, err
}

func w6bECREnrichOK(t *testing.T, fake *w6bECRFake, names ...string) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w6bECREnrich(t, fake, names...)
	if err != nil {
		t.Fatalf("ecr enricher returned an error for a run where every call answered: %v", err)
	}
	return res
}

// ─── row 7: ecr.public-policy ───────────────────────────────────────────────

// A wildcard principal on a repository policy lets anyone in the world pull
// the images, which for a private registry is the whole build output.
func TestW6BECR_PublicPolicy_WildcardPrincipal(t *testing.T) {
	const name = "acme/public-mirror"
	fake := &w6bECRFake{
		policies:   map[string]string{name: w6bECRPublicPolicyDoc},
		lifecycles: map[string]string{name: w6bECRLifecycleDoc},
	}
	res := w6bECREnrichOK(t, fake, name)

	w2AssertFinding(t, res.Findings[name], string(w6bECRCodePublicPolicy),
		w6bECRPhrasePublicPolicy, domain.SevBroken, w6bECRSource)
	rows := w2Rows(t, res, name, string(w6bECRCodePublicPolicy))
	w2AssertRow(t, rows, "Principal", "*")
	w2AssertRow(t, rows, "Actions", "ecr:BatchGetImage, ecr:GetDownloadUrlForLayer")
}

// No policy at all is the default and the safe state. It is a definite answer,
// so it must not mark the repository unknown.
func TestW6BECR_NoRepositoryPolicy_IsHealthyNotTruncated(t *testing.T) {
	const name = "acme/private-repo"
	fake := &w6bECRFake{lifecycles: map[string]string{name: w6bECRLifecycleDoc}}
	res := w6bECREnrichOK(t, fake, name)

	w2AssertNoCode(t, res.Findings[name], string(w6bECRCodePublicPolicy))
	if res.TruncatedIDs[name] {
		t.Error("RepositoryPolicyNotFound marked the repository unknown; it is a definite no-policy answer")
	}
}

// A policy naming this account's roles is the intended configuration.
func TestW6BECR_ScopedPolicy_IsHealthy(t *testing.T) {
	const name = "acme/scoped-repo"
	doc := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-deploy"},"Action":"ecr:BatchGetImage"}]}`
	fake := &w6bECRFake{
		policies:   map[string]string{name: doc},
		lifecycles: map[string]string{name: w6bECRLifecycleDoc},
	}
	res := w6bECREnrichOK(t, fake, name)
	w2AssertNoCode(t, res.Findings[name], string(w6bECRCodePublicPolicy))
}

// A wildcard fenced by a condition is a scoped grant, not an open door —
// contract rule 6 says Conditioned is not the finding.
func TestW6BECR_ConditionedWildcard_IsNotPublic(t *testing.T) {
	const name = "acme/org-repo"
	doc := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"ecr:BatchGetImage","Condition":{"StringEquals":{"aws:PrincipalOrgID":"o-acme12345"}}}]}`
	fake := &w6bECRFake{
		policies:   map[string]string{name: doc},
		lifecycles: map[string]string{name: w6bECRLifecycleDoc},
	}
	res := w6bECREnrichOK(t, fake, name)
	w2AssertNoCode(t, res.Findings[name], string(w6bECRCodePublicPolicy))
}

// ─── row 10: ecr.no-lifecycle-policy ────────────────────────────────────────

// Here the NotFound answer IS the finding: no lifecycle policy means untagged
// layers accumulate until the registry bill notices.
func TestW6BECR_NoLifecyclePolicy_NotFoundIsTheFinding(t *testing.T) {
	const name = "acme/batch-processor"
	fake := &w6bECRFake{}
	res := w6bECREnrichOK(t, fake, name)

	w2AssertFinding(t, res.Findings[name], string(w6bECRCodeNoLifecycle),
		w6bECRPhraseNoLifecycle, domain.SevWarn, w6bECRSource)
	w2AssertNoRows(t, res, name, string(w6bECRCodeNoLifecycle))
	if res.TruncatedIDs[name] {
		t.Error("LifecyclePolicyNotFound marked the repository unknown; the absence is the answer, not a failed read")
	}
}

func TestW6BECR_LifecyclePolicyPresent_IsHealthy(t *testing.T) {
	const name = "acme/frontend"
	fake := &w6bECRFake{lifecycles: map[string]string{name: w6bECRLifecycleDoc}}
	res := w6bECREnrichOK(t, fake, name)
	w2AssertNoCode(t, res.Findings[name], string(w6bECRCodeNoLifecycle))
}

// ─── wave-2 discipline ──────────────────────────────────────────────────────

// A repository whose policy read failed is unknown, not clean. The row must go
// to "?" rather than quietly disappear, and its neighbours must still be
// evaluated — one failed call is not a failed pass.
func TestW6BECR_PolicyReadError_MarksOnlyThatRepoUnknown(t *testing.T) {
	const broken = "acme/unreadable"
	const healthy = "acme/public-mirror"
	fake := &w6bECRFake{
		policies:   map[string]string{healthy: w6bECRPublicPolicyDoc},
		lifecycles: map[string]string{healthy: w6bECRLifecycleDoc, broken: w6bECRLifecycleDoc},
		policyErr:  map[string]error{broken: errors.New("AccessDeniedException: not authorized to perform ecr:GetRepositoryPolicy")},
	}
	res, _ := w6bECREnrich(t, fake, broken, healthy)

	if !res.TruncatedIDs[broken] {
		t.Errorf("a repository whose policy could not be read must be marked unknown, got TruncatedIDs=%v", res.TruncatedIDs)
	}
	w2AssertNoCode(t, res.Findings[broken], string(w6bECRCodePublicPolicy))
	w2AssertFinding(t, res.Findings[healthy], string(w6bECRCodePublicPolicy),
		w6bECRPhrasePublicPolicy, domain.SevBroken, w6bECRSource)
	if res.TruncatedIDs[healthy] {
		t.Error("one repository's failure marked an unrelated repository unknown")
	}
}

// A cross-region or NotFound answer on the images call is not a failure of the
// pass, but the repository it happened to is still unknown.
func TestW6BECR_ImagesReadError_DoesNotSinkTheOtherRepos(t *testing.T) {
	const broken = "acme/cross-region"
	const neighbour = "acme/frontend"
	// The neighbour is given no lifecycle policy on purpose. It is the one
	// still-readable repository in the batch, so the finding it carries is the
	// proof that the failed read next to it did not sink the whole pass — an
	// earlier draft handed it a policy and then asserted the no-lifecycle
	// finding on it, which no repository with a policy can carry.
	fake := &w6bECRFake{
		imagesErr: map[string]error{broken: errors.New("RepositoryNotFoundException: does not exist in the registry")},
	}
	res, _ := w6bECREnrich(t, fake, broken, neighbour)

	if !res.TruncatedIDs[broken] {
		t.Errorf("the repository whose read failed must be marked unknown, got %v", res.TruncatedIDs)
	}
	w2AssertFinding(t, res.Findings[neighbour], string(w6bECRCodeNoLifecycle),
		w6bECRPhraseNoLifecycle, domain.SevWarn, w6bECRSource)
	if res.TruncatedIDs[neighbour] {
		t.Error("one repository's failed read marked its neighbour unknown")
	}
}

// Past the cap the pass reports itself truncated rather than silently
// evaluating a prefix and presenting it as the whole registry.
func TestW6BECR_PastEnrichmentCap_ReportsTruncated(t *testing.T) {
	fake := &w6bECRFake{}
	names := make([]string, 0, awsclient.EnrichmentCap+1)
	for i := range awsclient.EnrichmentCap + 1 {
		names = append(names, "acme/repo-"+strconv.Itoa(i))
	}
	res := w6bECREnrichOK(t, fake, names...)
	if !res.Truncated {
		t.Errorf("%d repositories is past the cap of %d; the result must report Truncated",
			len(names), awsclient.EnrichmentCap)
	}
}

func TestW6BECR_AtEnrichmentCap_IsNotTruncated(t *testing.T) {
	fake := &w6bECRFake{}
	names := make([]string, 0, awsclient.EnrichmentCap)
	for i := range awsclient.EnrichmentCap {
		names = append(names, "acme/repo-"+strconv.Itoa(i))
	}
	res := w6bECREnrichOK(t, fake, names...)
	if res.Truncated {
		t.Errorf("exactly %d repositories is within the cap; the result must not report Truncated", awsclient.EnrichmentCap)
	}
}

// A nil client is a pass that has nothing to say, not a crash.
func TestW6BECR_NilClient_ReturnsEmptyResult(t *testing.T) {
	res, err := w2Enricher(t, "ecr")(context.Background(), &awsclient.ServiceClients{}, []resource.Resource{
		{ID: "acme/frontend", Name: "acme/frontend"},
	}, nil)
	if err != nil {
		t.Fatalf("nil ECR client must not be an error: %v", err)
	}
	w2AssertEnricherShape(t, res)
	if len(res.Findings) != 0 {
		t.Errorf("nil client produced findings: %+v", res.Findings)
	}
}

// ─── independence ───────────────────────────────────────────────────────────

// Contract rule 4, across the wave boundary: a repository that is wrong in
// four ways carries four findings, and the two waves do not overwrite each
// other's entries.
func TestW6BECR_AllFourConditions_ProduceFourFindings(t *testing.T) {
	const name = "acme/worst-case"
	repo := w6bECRRepo(name)
	repo.ImageTagMutability = ecrtypes.ImageTagMutabilityMutable
	repo.ImageScanningConfiguration = &ecrtypes.ImageScanningConfiguration{ScanOnPush: false}

	rs := w6bFetchECR(t, repo)
	r := pw1ResourceByID(t, rs, name)
	pw1RequireFinding(t, r.Findings, w6bECRCodeScanOnPushOff, w6bECRPhraseScanOnPushOff, domain.SevWarn, "wave1")
	pw1RequireFinding(t, r.Findings, w6bECRCodeMutableTags, w6bECRPhraseMutableTags, domain.SevWarn, "wave1")

	fake := &w6bECRFake{policies: map[string]string{name: w6bECRPublicPolicyDoc}}
	res := w6bECREnrichOK(t, fake, name)
	w2AssertFinding(t, res.Findings[name], string(w6bECRCodePublicPolicy), w6bECRPhrasePublicPolicy, domain.SevBroken, w6bECRSource)
	w2AssertFinding(t, res.Findings[name], string(w6bECRCodeNoLifecycle), w6bECRPhraseNoLifecycle, domain.SevWarn, w6bECRSource)
}

// ─── catalog ────────────────────────────────────────────────────────────────

func TestW6BECR_FindingDefsRegistered(t *testing.T) {
	w2AssertFindingDef(t, "ecr", string(w6bECRCodeScanOnPushOff), w6bECRPhraseScanOnPushOff, domain.SevWarn, "wave1")
	w2AssertFindingDef(t, "ecr", string(w6bECRCodeMutableTags), w6bECRPhraseMutableTags, domain.SevWarn, "wave1")
	w2AssertFindingDef(t, "ecr", string(w6bECRCodePublicPolicy), w6bECRPhrasePublicPolicy, domain.SevBroken, "wave2")
	w2AssertFindingDef(t, "ecr", string(w6bECRCodeNoLifecycle), w6bECRPhraseNoLifecycle, domain.SevWarn, "wave2")
}

// The enricher reaches GetLifecyclePolicy by type assertion, so a client that
// predates the call must degrade to "no lifecycle finding" rather than
// reporting every repository as unpolicied.
func TestW6BECR_ClientWithoutLifecycleCall_EmitsNoLifecycleFinding(t *testing.T) {
	const name = "acme/frontend"
	res := w6bECRNoLifecycleAPI(t, name)
	w2AssertNoCode(t, res.Findings[name], string(w6bECRCodeNoLifecycle))
}

// w6bECRLegacyFake implements everything the enricher needs EXCEPT
// GetLifecyclePolicy.
type w6bECRLegacyFake struct {
	awsclient.ECRAPI
}

func (f *w6bECRLegacyFake) DescribeImages(_ context.Context, _ *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	return &ecr.DescribeImagesOutput{}, nil
}

func (f *w6bECRLegacyFake) GetRepositoryPolicy(_ context.Context, in *ecr.GetRepositoryPolicyInput, _ ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error) {
	return nil, &ecrtypes.RepositoryPolicyNotFoundException{Message: aws.String(fmt.Sprintf("no policy for %s", aws.ToString(in.RepositoryName)))}
}

func w6bECRNoLifecycleAPI(t *testing.T, name string) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w2Enricher(t, "ecr")(context.Background(),
		&awsclient.ServiceClients{ECR: &w6bECRLegacyFake{}},
		[]resource.Resource{{ID: name, Name: name}}, nil)
	if err != nil {
		t.Fatalf("a client without GetLifecyclePolicy must not fail the pass: %v", err)
	}
	w2AssertEnricherShape(t, res)
	return res
}
