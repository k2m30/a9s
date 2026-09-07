package unit

// partial_answer_findings_test.go — the sites the batch's own rule reaches and
// its fixes did not. Rows 3 and 4 replaced a work list sliced at EnrichmentCap
// with capAtEnrichmentCap, which marks every row the dropped items would have
// answered for. Three more enrichers still slice the same way, and the rows
// past their caps still render inspected-and-clean.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	codebuild "github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// ── finding 1: lambda's own cap, in the file row 1 changed ────────────────

// TestPartialFindingLambda_FunctionsPastTheCapAreNotInspected pins the cap
// rule on the enricher the batch already owns. The work list is filtered and
// then sliced, so the 51st function's resource policy and function URL are
// never read — and its row claims a function nobody asked about is not
// invokable by anyone.
func TestPartialFindingLambda_FunctionsPastTheCapAreNotInspected(t *testing.T) {
	const n = awsclient.EnrichmentCap + 1
	dropped := fmt.Sprintf("acme-fn-%03d", n-1)

	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	clients := &awsclient.ServiceClients{Lambda: &partialLambdaFake{
		policy:  partialLambdaPublicPolicy,
		hasURL:  true,
		urlAuth: lambdatypes.FunctionUrlAuthTypeNone,
	}}
	clients.SetIdentityStore(store)

	rows := make([]resource.Resource, 0, n)
	for i := range n {
		rows = append(rows, w2Res(fmt.Sprintf("acme-fn-%03d", i), nil))
	}

	res, _ := awsclient.EnrichLambdaPosture(context.Background(), clients, rows, nil)
	w2AssertEnricherShape(t, res)

	w4AssertNoCode(t, res.Findings[dropped], "lambda.public-policy")
	partialAssertUninspected(t, res, dropped)
	partialAssertInspected(t, res, "acme-fn-000")
}

// ── finding 2: asg, where a comment already states the rule ───────────────

// partialASGFake answers DescribeLaunchConfigurations for whatever names it is
// asked for, with a configuration that trips the IMDSv1 rule. Scaling
// activities are empty, so the launch-configuration walk is the only thing
// under test.
type partialASGFake struct {
	awsclient.ASGAPI
}

func (f *partialASGFake) DescribeScalingActivities(_ context.Context, _ *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	return &autoscaling.DescribeScalingActivitiesOutput{}, nil
}

func (f *partialASGFake) DescribeLaunchConfigurations(_ context.Context, in *autoscaling.DescribeLaunchConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	out := &autoscaling.DescribeLaunchConfigurationsOutput{}
	for _, name := range in.LaunchConfigurationNames {
		out.LaunchConfigurations = append(out.LaunchConfigurations, asgtypes.LaunchConfiguration{
			LaunchConfigurationName: aws.String(name),
			ImageId:                 aws.String("ami-0a1b2c3d4e5f60001"),
			InstanceType:            aws.String("m6i.large"),
		})
	}
	return out, nil
}

// TestPartialFindingASG_GroupsPastTheCapAreNotInspected pins the same rule on
// the enricher that already states it. Two comments below the slice say a
// configuration the walk never read leaves its groups "not inspected" rather
// than "nothing to report", and the page-cap branch below implements exactly
// that — over the already-truncated name list, so the configurations the slice
// dropped are the one set nothing marks.
func TestPartialFindingASG_GroupsPastTheCapAreNotInspected(t *testing.T) {
	const n = awsclient.EnrichmentCap + 1
	dropped := fmt.Sprintf("acme-asg-%03d", n-1)

	rows := make([]resource.Resource, 0, n)
	for i := range n {
		rows = append(rows, asgGroupOn(fmt.Sprintf("acme-asg-%03d", i), fmt.Sprintf("acme-lc-%03d", i)))
	}

	res, _ := awsclient.EnrichASGScalingActivities(context.Background(),
		&awsclient.ServiceClients{AutoScaling: &partialASGFake{}}, rows, nil)
	w2AssertEnricherShape(t, res)

	w4AssertNoCode(t, res.Findings[dropped], "asg.launch-config.imdsv1")
	partialAssertUninspected(t, res, dropped)
	partialAssertInspected(t, res, "acme-asg-000")
}

// ── finding 3: codebuild ──────────────────────────────────────────────────

// partialCBFake gives every project one build, and every build has failed.
type partialCBFake struct {
	awsclient.CodeBuildAPI
}

func (f *partialCBFake) ListBuildsForProject(_ context.Context, in *codebuild.ListBuildsForProjectInput, _ ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error) {
	return &codebuild.ListBuildsForProjectOutput{Ids: []string{aws.ToString(in.ProjectName) + ":a1b2c3d4-1111-2222-3333-444455556666"}}, nil
}

func (f *partialCBFake) BatchGetBuilds(_ context.Context, in *codebuild.BatchGetBuildsInput, _ ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error) {
	out := &codebuild.BatchGetBuildsOutput{}
	for _, id := range in.Ids {
		out.Builds = append(out.Builds, cbtypes.Build{
			Id:            aws.String(id),
			BuildStatus:   cbtypes.StatusTypeFailed,
			BuildComplete: true,
			EndTime:       aws.Time(time.Now().Add(-3 * time.Hour)),
			Phases: []cbtypes.BuildPhase{
				{PhaseType: cbtypes.BuildPhaseTypeBuild, PhaseStatus: cbtypes.StatusTypeFailed},
			},
		})
	}
	return out, nil
}

// TestPartialFindingCodeBuild_ProjectsPastTheCapAreNotInspected pins the third
// site. The project list is sliced before any build is listed, so a project
// past the cap shows no build status at all while its row reads as inspected.
func TestPartialFindingCodeBuild_ProjectsPastTheCapAreNotInspected(t *testing.T) {
	const n = awsclient.EnrichmentCap + 1
	dropped := fmt.Sprintf("acme-build-%03d", n-1)

	rows := make([]resource.Resource, 0, n)
	for i := range n {
		id := fmt.Sprintf("acme-build-%03d", i)
		rows = append(rows, resource.Resource{ID: id, Name: id, Fields: map[string]string{}})
	}

	res, _ := awsclient.EnrichCodeBuildStatus(context.Background(),
		&awsclient.ServiceClients{CodeBuild: &partialCBFake{}}, rows, nil)
	w2AssertEnricherShape(t, res)

	w4AssertNoCode(t, res.Findings[dropped], "cb.latest-build-failed")
	partialAssertUninspected(t, res, dropped)
	partialAssertInspected(t, res, "acme-build-000")
}
