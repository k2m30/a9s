package unit

// Regression pins for the ECR aggregate interface, the ecs-task health
// compare and the cfn lifecycle phrases.

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

// w6bECRAggregateOnlyFake embeds ECRAPI and implements only DescribeImages.
// GetRepositoryPolicy is not in ECRAPI, so the enricher's type assertion
// answers no for this fake and it must come back with no policy finding and
// no panic. Were the method in the aggregate, the assertion would succeed
// against the nil embedded interface and the call would segfault.
type w6bECRAggregateOnlyFake struct {
	awsclient.ECRAPI
}

func (f *w6bECRAggregateOnlyFake) DescribeImages(_ context.Context, _ *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	return &ecr.DescribeImagesOutput{}, nil
}

// A fake that embeds the aggregate without implementing the policy call must
// not reach that call. A panic here means GetRepositoryPolicy is in ECRAPI
// and the assertion is answering yes to a nil method.
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

// A row rebuilt from the on-disk type cache can hold the SDK's uppercase
// spelling. A strict compare would drop its unhealthy finding, so the cell
// would read "unhealthy" on a row coloured green.
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

// Every cfn lifecycle finding draws its Phrase into the list status cell. The
// SDK spells these UPDATE_ROLLBACK_FAILED; the cell has to read the sentence,
// so the pin names the shape rather than one status.
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
