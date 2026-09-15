package unit

// codex10_failed_of_total_test.go — the "failed for N of M IDs" line counts
// what it names.
//
// N is resources, M is resources: a check that issues several calls per
// resource must not report more failures than there are resources to fail.
// And the operation a multi-call check hands to the aggregate names the check,
// not whichever of its calls the caller happened to write first.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrsvc "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// deniedCall builds the error AWS answers for an action a role lacks: an
// operation error carrying the API error whose message names the action.
func deniedCall(operation, action string) error {
	return &smithy.OperationError{
		OperationName: operation,
		Err: &smithy.GenericAPIError{
			Code: "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session " +
				"is not authorized to perform: " + action +
				" because no identity-based policy allows the " + action + " action",
		},
	}
}

// aggCounts reads the numerator and denominator out of an aggregate's line.
func aggCounts(t *testing.T, line string) (failed, total int) {
	t.Helper()
	_, rest, ok := strings.Cut(line, " failed for ")
	if !ok {
		t.Fatalf("aggregate line has no %q clause: %q", "failed for", line)
	}
	if _, err := fmt.Sscanf(rest, "%d of %d IDs", &failed, &total); err != nil {
		t.Fatalf("aggregate line %q does not carry an N of M count: %v", line, err)
	}
	return failed, total
}

// aggOperation returns the operation label the aggregate leads with.
func aggOperation(t *testing.T, line string) string {
	t.Helper()
	op, _, ok := strings.Cut(line, " failed for ")
	if !ok {
		t.Fatalf("aggregate line has no %q clause: %q", "failed for", line)
	}
	return op
}

// TestAggregateFailures_NumeratorIsResourcesNotCalls pins that a check making
// several calls per resource reports one failed resource, not one failure per
// refused call: a role denied two of a posture pass's reads on the same bucket
// has failed that bucket once. The per-cause groups keep their own call counts
// — one denial never speaks for another — so only the numerator changes.
func TestAggregateFailures_NumeratorIsResourcesNotCalls(t *testing.T) {
	failures := []awsclient.Failure{
		awsclient.FailedCall("bucket-alpha", deniedCall("GetBucketPolicy", "s3:GetBucketPolicy")),
		awsclient.FailedCall("bucket-alpha", deniedCall("GetBucketAcl", "s3:GetBucketAcl")),
	}
	err := awsclient.AggregateFailures("bucket posture", failures, 3)
	if err == nil {
		t.Fatal("two refused calls must still aggregate to an error")
	}
	const want = "bucket posture failed for 1 of 3 IDs: " +
		"not authorized to perform s3:GetBucketAcl (1, e.g. bucket-alpha); " +
		"not authorized to perform s3:GetBucketPolicy (1, e.g. bucket-alpha)"
	if got := err.Error(); got != want {
		t.Errorf("aggregate line\n got: %q\nwant: %q", got, want)
	}
}

// TestAggregateFailures_NumeratorCountsEachResourceOnce pins the same rule
// with the failures spread over two of three resources, so a numerator that
// counted calls (3) and one that counted resources (2) differ from the total
// and from each other.
func TestAggregateFailures_NumeratorCountsEachResourceOnce(t *testing.T) {
	failures := []awsclient.Failure{
		awsclient.FailedCall("bucket-alpha", deniedCall("GetBucketPolicy", "s3:GetBucketPolicy")),
		awsclient.FailedCall("bucket-alpha", deniedCall("GetBucketAcl", "s3:GetBucketAcl")),
		awsclient.FailedCall("bucket-beta", deniedCall("GetBucketAcl", "s3:GetBucketAcl")),
	}
	err := awsclient.AggregateFailures("bucket posture", failures, 3)
	if err == nil {
		t.Fatal("three refused calls must still aggregate to an error")
	}
	const want = "bucket posture failed for 2 of 3 IDs: " +
		"not authorized to perform s3:GetBucketAcl (2, e.g. bucket-alpha); " +
		"not authorized to perform s3:GetBucketPolicy (1, e.g. bucket-alpha)"
	if got := err.Error(); got != want {
		t.Errorf("aggregate line\n got: %q\nwant: %q", got, want)
	}
}

// TestAggregateFailures_OneCallPerResourceIsUnchanged is the counterpart: a
// check with one call per resource already counted resources, and the line it
// produced must read exactly as it did.
func TestAggregateFailures_OneCallPerResourceIsUnchanged(t *testing.T) {
	failures := []awsclient.Failure{
		awsclient.FailedCall("bucket-alpha", deniedCall("GetBucketAcl", "s3:GetBucketAcl")),
		awsclient.FailedCall("bucket-beta", deniedCall("GetBucketAcl", "s3:GetBucketAcl")),
		awsclient.FailedCall("bucket-gamma", deniedCall("GetBucketAcl", "s3:GetBucketAcl")),
	}
	err := awsclient.AggregateFailures("bucket posture", failures, 3)
	if err == nil {
		t.Fatal("three refused calls must still aggregate to an error")
	}
	const want = "bucket posture failed for 3 of 3 IDs: " +
		"not authorized to perform s3:GetBucketAcl (e.g. bucket-alpha)"
	if got := err.Error(); got != want {
		t.Errorf("aggregate line\n got: %q\nwant: %q", got, want)
	}
}

// TestAggregateFailures_PageWalkStaysWithinItsTotal covers the records that
// name no resource at all: a page that never arrived holds nothing to count as
// a resource, and the count must still stay within the total it names while
// every page keeps its own group.
func TestAggregateFailures_PageWalkStaysWithinItsTotal(t *testing.T) {
	failures := []awsclient.Failure{
		awsclient.FailedOnPage(1, deniedCall("ListObjectsV2", "s3:ListBucket")),
		awsclient.FailedOnPage(2, deniedCall("ListObjectsV2", "s3:GetObject")),
	}
	err := awsclient.AggregateFailures("object walk", failures, 2)
	if err == nil {
		t.Fatal("two failed pages must aggregate to an error")
	}
	line := err.Error()
	failed, total := aggCounts(t, line)
	if failed > total {
		t.Errorf("aggregate claims %d failed of %d: %q", failed, total, line)
	}
	for _, want := range []string{"on page 1", "on page 2"} {
		if !strings.Contains(line, want) {
			t.Errorf("aggregate line must keep the group for %q; got: %q", want, line)
		}
	}
}

// ecrPostureFake answers the three calls the ECR posture pass makes. Each
// policy read is refused on its own, which is what a partially denied
// read-only role does to it. deniedRepo scopes the refusals to one repository;
// empty refuses them for every repository.
type ecrPostureFake struct {
	awsclient.ECRAPI
	deniedRepo    string
	repoPolicyErr error
	lifecycleErr  error
}

// refuses reports whether this repository is the one the role may not read.
func (f *ecrPostureFake) refuses(name *string) bool {
	return f.deniedRepo == "" || (name != nil && *name == f.deniedRepo)
}

func (f *ecrPostureFake) DescribeImages(
	_ context.Context,
	in *ecrsvc.DescribeImagesInput,
	_ ...func(*ecrsvc.Options),
) (*ecrsvc.DescribeImagesOutput, error) {
	return &ecrsvc.DescribeImagesOutput{
		ImageDetails: []ecrtypes.ImageDetail{
			{
				RepositoryName: in.RepositoryName,
				ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
					FindingSeverityCounts: map[string]int32{
						string(ecrtypes.FindingSeverityCritical): 0,
						string(ecrtypes.FindingSeverityHigh):     0,
					},
				},
			},
		},
	}, nil
}

func (f *ecrPostureFake) GetRepositoryPolicy(
	_ context.Context,
	in *ecrsvc.GetRepositoryPolicyInput,
	_ ...func(*ecrsvc.Options),
) (*ecrsvc.GetRepositoryPolicyOutput, error) {
	if f.repoPolicyErr != nil && f.refuses(in.RepositoryName) {
		return nil, f.repoPolicyErr
	}
	return &ecrsvc.GetRepositoryPolicyOutput{
		RepositoryName: in.RepositoryName,
		PolicyText: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
			`"Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"ecr:GetDownloadUrlForLayer"}]}`),
	}, nil
}

func (f *ecrPostureFake) GetLifecyclePolicy(
	_ context.Context,
	in *ecrsvc.GetLifecyclePolicyInput,
	_ ...func(*ecrsvc.Options),
) (*ecrsvc.GetLifecyclePolicyOutput, error) {
	if f.lifecycleErr != nil && f.refuses(in.RepositoryName) {
		return nil, f.lifecycleErr
	}
	return &ecrsvc.GetLifecyclePolicyOutput{
		RepositoryName: in.RepositoryName,
		LifecyclePolicyText: aws.String(
			`{"rules":[{"rulePriority":1,"action":{"type":"expire"},` +
				`"selection":{"tagStatus":"untagged","countType":"sinceImagePushed","countUnit":"days","countNumber":30}}]}`),
	}, nil
}

var _ awsclient.ECRGetRepositoryPolicyAPI = (*ecrPostureFake)(nil)
var _ awsclient.ECRGetLifecyclePolicyAPI = (*ecrPostureFake)(nil)

func ecrPostureRepo(name string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"repository_name": name,
			"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/" + name,
			"scan_on_push":    "true",
		},
	}
}

// TestEnrichECRRepository_DeniedPostureNamesTheCheckNotACall pins what the
// error log says when a role may list images but not read either policy: the
// line names the posture pass the way every other multi-call check names
// itself ("bucket posture", "key policy and rotation"), and counts the one
// repository once. Naming DescribeImages there points the operator at the one
// call that answered.
func TestEnrichECRRepository_DeniedPostureNamesTheCheckNotACall(t *testing.T) {
	const repo = "example-app"
	fake := &ecrPostureFake{
		repoPolicyErr: deniedCall("GetRepositoryPolicy", "ecr:GetRepositoryPolicy"),
		lifecycleErr:  deniedCall("GetLifecyclePolicy", "ecr:GetLifecyclePolicy"),
	}
	clients := &awsclient.ServiceClients{ECR: fake}

	_, err := awsclient.EnrichECRRepository(context.Background(), clients,
		[]resource.Resource{ecrPostureRepo(repo)}, nil)
	if err == nil {
		t.Fatal("two refused policy reads must surface as a composite error")
	}
	line := err.Error()

	op := aggOperation(t, line)
	if op == "DescribeImages" {
		t.Errorf("operation names a call that answered; the two that failed are the policy reads: %q", line)
	}
	if strings.ToLower(op) != op || !strings.Contains(op, " ") {
		t.Errorf("operation %q is not a phrase for the check; the siblings read %q, %q, %q",
			op, "bucket posture", "log bucket posture", "key policy and rotation")
	}

	failed, total := aggCounts(t, line)
	if failed != 1 || total != 1 {
		t.Errorf("one repository refusing two reads reads as %d of %d IDs, want 1 of 1: %q",
			failed, total, line)
	}

	for _, want := range []string{
		"not authorized to perform ecr:GetLifecyclePolicy",
		"not authorized to perform ecr:GetRepositoryPolicy",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("composite must state the cause %q; got: %q", want, line)
		}
	}
	if !strings.Contains(line, repo) {
		t.Errorf("composite must name the repository %q; got: %q", repo, line)
	}
}

// TestEnrichECRRepository_ReadablePostureReportsNoFailure is the counterpart:
// a role that may read both policies produces no failure line at all.
func TestEnrichECRRepository_ReadablePostureReportsNoFailure(t *testing.T) {
	clients := &awsclient.ServiceClients{ECR: &ecrPostureFake{}}

	result, err := awsclient.EnrichECRRepository(context.Background(), clients,
		[]resource.Resource{ecrPostureRepo("example-app")}, nil)
	if err != nil {
		t.Fatalf("a posture pass that read both policies must report no failure; got: %v", err)
	}
	if result.Truncated {
		t.Error("a fully read repository must not be marked data-incomplete")
	}
}

// TestEnrichECRRepository_OneDeniedReadOfTwoRepos pins the mixed case: one of
// two repositories refuses both policy reads, so the line reads 1 of 2 — the
// numerator can neither count the two refused calls nor the readable
// repository.
func TestEnrichECRRepository_OneDeniedReadOfTwoRepos(t *testing.T) {
	const deniedRepo = "example-denied"
	const okRepo = "example-readable"
	fake := &ecrPostureFake{
		deniedRepo:    deniedRepo,
		repoPolicyErr: deniedCall("GetRepositoryPolicy", "ecr:GetRepositoryPolicy"),
		lifecycleErr:  deniedCall("GetLifecyclePolicy", "ecr:GetLifecyclePolicy"),
	}
	clients := &awsclient.ServiceClients{ECR: fake}

	_, err := awsclient.EnrichECRRepository(context.Background(), clients,
		[]resource.Resource{ecrPostureRepo(okRepo), ecrPostureRepo(deniedRepo)}, nil)
	if err == nil {
		t.Fatal("a denied repository must surface as a composite error")
	}
	failed, total := aggCounts(t, err.Error())
	if failed != 1 || total != 2 {
		t.Errorf("one denied repository of two reads as %d of %d IDs, want 1 of 2: %q",
			failed, total, err.Error())
	}
}
