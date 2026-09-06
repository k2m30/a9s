package unit

// prowler_w6b_verify_test.go — the pins the verify round added, each covering
// a way the round-1 implementation could regress without any existing test
// noticing.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ─── ECRGetRepositoryPolicyAPI is out of the aggregate ──────────────────────

// w6bECRAggregateOnlyFake embeds ECRAPI and implements only DescribeImages.
// Before GetRepositoryPolicy left the aggregate, embedding ECRAPI handed this
// fake a nil GetRepositoryPolicy method: the enricher's type assertion
// SUCCEEDED against the embedded interface and the call segfaulted.
//
// The point of narrowing the aggregate is that the assertion now answers
// honestly, so this fake must come back with no policy finding and no panic.
type w6bECRAggregateOnlyFake struct {
	awsclient.ECRAPI
}

func (f *w6bECRAggregateOnlyFake) DescribeImages(_ context.Context, _ *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	return &ecr.DescribeImagesOutput{}, nil
}

// A fake that embeds the aggregate without implementing the policy call must
// not reach that call. A panic here is the regression: it means
// GetRepositoryPolicy is back in ECRAPI and the assertion is answering yes to
// a nil method.
func TestW6BECR_AggregateFakeWithoutPolicyCall_DoesNotReachIt(t *testing.T) {
	const name = "acme/frontend"
	res, err := w2Enricher(t, "ecr")(context.Background(),
		&awsclient.ServiceClients{ECR: &w6bECRAggregateOnlyFake{}},
		[]resource.Resource{{ID: name, Name: name}}, nil)
	if err != nil {
		t.Fatalf("a client without the policy call must not fail the pass: %v", err)
	}
	w2AssertEnricherShape(t, res)
	w2AssertNoCode(t, res.Findings[name], string(w6bECRCodePublicPolicy))
}

// The same argument for the lifecycle call, which never had the bug because it
// was never in the aggregate. Pinned so that "tidying" it back in reintroduces
// the segfault as a test failure instead of a crash in an unrelated suite.
func TestW6BECR_PolicyAndLifecycleCallsAreNotInTheAggregate(t *testing.T) {
	var aggregate awsclient.ECRAPI = &w6bECRAggregateOnlyFake{}
	if _, ok := aggregate.(awsclient.ECRGetRepositoryPolicyAPI); ok {
		t.Error("ECRAPI still carries GetRepositoryPolicy: a fake embedding the aggregate " +
			"satisfies the assertion with a nil method, which segfaults when the enricher calls it")
	}
	if _, ok := aggregate.(awsclient.ECRGetLifecyclePolicyAPI); ok {
		t.Error("ECRAPI now carries GetLifecyclePolicy; same nil-method trap as above")
	}
}

// ─── row 12: the health compare survives the cache ──────────────────────────

// A row rebuilt from the on-disk type cache still holds the SDK's spelling
// from before the column moved to words. A strict compare would retire its
// unhealthy finding, so the cell would read "unhealthy" on a row coloured
// green — the same defect row 12 exists to fix, arriving from the cache
// instead of from the SDK.
func TestW6BECSTask_CachedUppercaseHealth_StillCarriesTheFinding(t *testing.T) {
	for _, health := range []string{"UNHEALTHY", "unhealthy", "Unhealthy"} {
		t.Run(health, func(t *testing.T) {
			rs := w6bFetchECSTasks(t, w6bECSTask("task-"+health, ecstypes.HealthStatus(health), "RUNNING"))
			if _, ok := pw1FindFinding(rs[0].Findings, awsclient.CodeECSTaskHealthUnhealthy); !ok {
				t.Errorf("health %q lost its unhealthy finding: %+v", health, rs[0].Findings)
			}
		})
	}
}

// The words the column renders are lowercase whatever case AWS used, so a
// cached row and a fresh one read the same.
func TestW6BECSTask_HealthColumnIsLowercaseWhateverTheInputCase(t *testing.T) {
	rs := w6bFetchECSTasks(t, w6bECSTask("task-mixedcase", ecstypes.HealthStatus("UnHeAlThY"), "RUNNING"))
	if got := rs[0].Fields["health"]; got != "unhealthy" {
		t.Errorf("Fields[health] = %q, want %q", got, "unhealthy")
	}
}

// ─── cfn list text is words, not the SDK's spelling ─────────────────────────

// Every cfn lifecycle finding draws its Phrase into the list status cell. The
// SDK spells these UPDATE_ROLLBACK_FAILED; the cell has to read the sentence.
// This was flagged in the red round as still emitting the lower-cased raw
// enum, so the pin names the shape rather than one status.
func TestW6BCFN_LifecyclePhrasesAreWordsNotUnderscoredEnums(t *testing.T) {
	for _, status := range []string{
		"ROLLBACK_COMPLETE", "UPDATE_ROLLBACK_FAILED", "IMPORT_ROLLBACK_COMPLETE",
		"CREATE_FAILED", "DELETE_COMPLETE", "UPDATE_IN_PROGRESS",
	} {
		t.Run(status, func(t *testing.T) {
			name := "acme-status-" + status
			rs := w6bFetchCFN(t, w6bCFNStack(name, status, aws.Bool(true)))
			r := pw1ResourceByID(t, rs, name)
			if len(r.Findings) == 0 {
				t.Fatalf("status %q produced no lifecycle finding", status)
			}
			for _, f := range r.Findings {
				if strings.ContainsRune(f.Phrase, '_') {
					t.Errorf("status %q renders Phrase %q; the list cell reads the sentence, "+
						"not the SDK's underscored spelling", status, f.Phrase)
				}
			}
		})
	}
}
