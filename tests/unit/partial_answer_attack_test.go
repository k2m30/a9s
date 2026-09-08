package unit

// partial_answer_attack_test.go — the adversarial half of the partial-answer
// batch. Each test attacks one fix from a direction its own pin does not
// cover: a sibling row in the same batch, a second condition on one resource,
// the cap boundary from the other side, the shape the fix newly depends on
// surviving a disk-cache round trip.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrsvc "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	redshiftsvc "github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// ── row 1: both reads fail ────────────────────────────────────────────────

// TestPartialAttackLambda_BothReadsFailingIsOneFailedRow pins that a function
// whose two reads both failed is counted once. Each read is marked through the
// same per-resource path, and recording the row twice would make the composite
// error claim more failures than there were rows to fail.
func TestPartialAttackLambda_BothReadsFailingIsOneFailedRow(t *testing.T) {
	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	clients := &awsclient.ServiceClients{Lambda: &partialLambdaFake{
		policyErr: partialAccessDenied(),
		urlErr:    partialAccessDenied(),
	}}
	clients.SetIdentityStore(store)

	res, err := awsclient.EnrichLambdaPosture(context.Background(), clients,
		[]resource.Resource{w2Res(partialLambdaFn, nil)}, nil)

	partialAssertUninspected(t, res, partialLambdaFn)
	if err == nil {
		t.Fatal("two failed reads produced no error")
	}
	if got := strings.Count(err.Error(), partialLambdaFn); got != 1 {
		t.Errorf("the composite error names %s %d times, want 1: one row failed, not two", partialLambdaFn, got)
	}
}

// TestPartialAttackLambda_BothConditionsOnOneFunction pins that moving the
// error branch below the emissions did not make them exclusive. A function
// that is both open by policy and open by URL carries both findings.
func TestPartialAttackLambda_BothConditionsOnOneFunction(t *testing.T) {
	res := partialEnrichLambda(t, &partialLambdaFake{
		policy:  partialLambdaPublicPolicy,
		hasURL:  true,
		urlAuth: lambdatypes.FunctionUrlAuthTypeNone,
	})
	w4AssertFinding(t, res.Findings[partialLambdaFn], "lambda.public-policy",
		"invokable by anyone", domain.SevBroken, "wave2")
	w4AssertFinding(t, res.Findings[partialLambdaFn], "lambda.function-url-public",
		"function endpoint open without authentication", domain.SevBroken, "wave2")
	partialAssertInspected(t, res, partialLambdaFn)
}

// ── row 2: the catalogue answered, but with nothing ───────────────────────

// TestPartialAttackEKS_CatalogueEntryWithNoStatusIsUnknown pins the third way
// the catalogue can fail to answer: the entry exists but AWS left
// VersionStatus empty. An entry is not an answer, and the empty status is the
// value a reader gets from the deprecated field AWS never populates — the one
// shape most likely to be mistaken for a verdict.
func TestPartialAttackEKS_CatalogueEntryWithNoStatusIsUnknown(t *testing.T) {
	const name = "acme-platform"
	got := partialEKSVersionSupport(t, &w6bEKSFake{
		order:    []string{name},
		clusters: map[string]*ekstypes.Cluster{name: w6bEKSCluster(name, "1.30")},
		versions: map[string]w6bEKSVersionInfo{"1.30": {}},
	}, name)
	if got != partialEKSSupportUnknown {
		t.Errorf("version_support = %q, want %q: the catalogue entry carries no status", got, partialEKSSupportUnknown)
	}
}

// ── rows 3 and 4: what the shared cap helper marks ────────────────────────

// TestPartialAttackECSTask_EveryTaskOnADroppedDefinitionIsMarked attacks the
// grouped side of the cap helper. Definitions are shared by design — a
// service's tasks all run the same revision — so dropping one definition
// leaves several tasks uninspected, and marking only one of them would leave
// the rest rendering clean.
func TestPartialAttackECSTask_EveryTaskOnADroppedDefinitionIsMarked(t *testing.T) {
	const defs = 51
	droppedDef := partialECSDefARN(defs - 1)

	fake := &pw1ECSTaskFake{
		tasks: make(map[string]ecstypes.Task, defs+2),
		defs:  make(map[string]ecstypes.TaskDefinition, defs),
	}
	var rows []resource.Resource
	for i := range defs {
		id, defARN := partialECSTaskID(i), partialECSDefARN(i)
		fake.tasks[id] = pw1Task(id, defARN)
		fake.defs[defARN] = pw1TaskDef(defARN, pw1NoRefContainer("app"))
		rows = append(rows, pw1ECSTaskResource(id, defARN))
	}
	// Two more tasks on the definition the cap drops: three tasks lose their
	// answer, not one.
	siblings := []string{partialECSTaskID(90), partialECSTaskID(91)}
	for _, id := range siblings {
		fake.tasks[id] = pw1Task(id, droppedDef)
		rows = append(rows, pw1ECSTaskResource(id, droppedDef))
	}

	res := pw1EnrichECSTasks(t, fake, rows...)

	for _, id := range append(siblings, partialECSTaskID(defs-1)) {
		partialAssertUninspected(t, res, id)
	}
	partialAssertInspected(t, res, partialECSTaskID(0))
}

// TestPartialAttackEC2_SkippedInstancesAreNotCoverageGaps attacks the filtered
// side of the cap helper. A terminated instance is deliberately not read — its
// user data can no longer be changed and the host is gone — so it is left out
// of the work list before the cap and must not come back as a "?" row. The cap
// marks what it dropped, not what the filter excluded.
func TestPartialAttackEC2_SkippedInstancesAreNotCoverageGaps(t *testing.T) {
	fake := &partialEC2Fake{userData: map[string]string{}}
	var rows []resource.Resource
	for i := range 55 {
		id := partialEC2InstanceID(i)
		fake.userData[id] = "#!/bin/bash\nyum update -y\n"
		state := "running"
		if i >= 50 {
			state = "terminated"
		}
		rows = append(rows, resource.Resource{
			ID:     id,
			Name:   fmt.Sprintf("acme-worker-%02d", i),
			Fields: map[string]string{"state": state, "instance_id": id},
		})
	}

	res := partialEnrichEC2(t, fake, rows)

	// Exactly fifty instances are live, so the cap never bites and nothing —
	// live or terminated — is a coverage gap.
	if len(res.TruncatedIDs) != 0 {
		t.Errorf("TruncatedIDs = %v, want none: fifty live instances fit under the cap and terminated ones are not inspected by design", res.TruncatedIDs)
	}
}

// ── row 5: the flag the coverage join now depends on ──────────────────────

// TestPartialAttackBackup_PartialFlagSurvivesACacheReplay is the row-6 defect
// one level up. The abstention added for row 5 hangs on a field the plans
// fetcher writes; if the disk cache drops it, a plan whose selections nobody
// could finish reading comes back looking complete, and every resource in the
// account is judged against a selection list that was never fully read.
func TestPartialAttackBackup_PartialFlagSurvivesACacheReplay(t *testing.T) {
	fake := &partialBackupFake{selectionsErr: partialAccessDenied()}
	live := partialBackupCache(t, fake)["backup"].Resources
	if len(live) != 1 {
		t.Fatalf("fetcher returned %d plan rows, want 1", len(live))
	}

	replayed := partialCacheReplay(t, "backup", live[0])
	plans := w7CacheWith(replayed)

	res := w7EnrichEBS(t, []resource.Resource{w7Volume("in-use")}, plans)
	w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
}

// TestPartialAttackBackup_StringLikeIsAGlob pins that the wildcard shape
// StringLike exists for is read as one. A plan selecting `prod*` protects the
// production volume, and matching the pattern literally would report a covered
// volume as uncovered — the same wrong answer row 5 set out to remove.
func TestPartialAttackBackup_StringLikeIsAGlob(t *testing.T) {
	volume := w7Volume("in-use")
	vol := volume.RawStruct.(ec2types.Volume)
	vol.Tags = []ec2types.Tag{{Key: aws.String("environment"), Value: aws.String("production")}}
	volume.RawStruct = vol

	cacheEntry := partialBackupCache(t, &partialBackupFake{
		selections: [][]backuptypes.BackupSelectionsListMember{{partialBackupSelectionMember("sel-like")}},
		selectionByID: map[string]backuptypes.BackupSelection{
			"sel-like": partialBackupSelection(nil, &backuptypes.Conditions{
				StringLike: []backuptypes.ConditionParameter{{
					ConditionKey:   aws.String("aws:ResourceTag/environment"),
					ConditionValue: aws.String("prod*"),
				}},
			}),
		},
	})

	res := w7EnrichEBS(t, []resource.Resource{volume}, cacheEntry)
	w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
}

// TestPartialAttackBackup_SelectionsPastThePageCapAbstain attacks the bound on
// the new page walk. Running out of pages is the same unknown as a denied
// call: the selection that takes the volume in may sit on the page nobody
// read.
func TestPartialAttackBackup_SelectionsPastThePageCapAbstain(t *testing.T) {
	pages := make([][]backuptypes.BackupSelectionsListMember, awsclient.PerParentPageCap+2)
	byID := map[string]backuptypes.BackupSelection{}
	for i := range pages {
		id := fmt.Sprintf("sel-%02d", i)
		pages[i] = []backuptypes.BackupSelectionsListMember{partialBackupSelectionMember(id)}
		byID[id] = partialBackupSelection([]string{"arn:aws:rds:us-east-1:123456789012:db:acme-orders-db"}, nil)
	}
	cacheEntry := partialBackupCache(t, &partialBackupFake{selections: pages, selectionByID: byID})

	res := w7EnrichEBS(t, []resource.Resource{w7Volume("in-use")}, cacheEntry)
	w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
}

// TestPartialAttackBackup_PartialPlanAbstainsEveryType pins that the
// abstention is a property of the shared join, not of the one type its pin was
// written against. A plan list nobody could finish reading is the same unknown
// for a table, a database and a cluster as it is for a volume.
func TestPartialAttackBackup_PartialPlanAbstainsEveryType(t *testing.T) {
	plans := partialBackupCache(t, &partialBackupFake{selectionsErr: partialAccessDenied()})

	for _, tc := range []struct {
		name   string
		enrich func(*testing.T, []resource.Resource, resource.ResourceCache) awsclient.IssueEnricherResult
		id     string
		arn    string
		code   domain.FindingCode
	}{
		{"ddb", w7EnrichDDB, "acme-orders", w7TableARN, awsclient.CodeDDBNotInBackupPlan},
		{"dbi", w7EnrichDBI, "acme-orders-db", w7DBIARN, awsclient.CodeDBINotInBackupPlan},
		{"dbc", w7EnrichDBC, "acme-orders-cluster", w7DBCARN, awsclient.CodeDBCNotInBackupPlan},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.enrich(t, []resource.Resource{w7Row(tc.id, tc.arn)}, plans)
			w4AssertNoCode(t, res.Findings[tc.id], tc.code)
		})
	}
}

// ── row 6: the abstention must stay narrow ────────────────────────────────

// TestPartialAttackEBS_ARNCoverageStillDecidesAReplayedRow pins that unknown
// tags silence only the question tags answer. A volume a plan names by ARN is
// covered whether or not its tags could be read, and a second plan that
// selects by tag must not turn that settled answer back into an abstention.
func TestPartialAttackEBS_ARNCoverageStillDecidesAReplayedRow(t *testing.T) {
	plans := w7CacheWith(
		w7Plan("", "", "unrelated=yes"),
		w7Plan(w7VolumeARN, "", ""),
	)

	live := w7Volume("in-use")
	liveRes := w7EnrichEBS(t, []resource.Resource{live}, plans)
	w4AssertNoCode(t, liveRes.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)

	replayed := partialCacheReplay(t, "ebs", live)
	replayRes := w7EnrichEBS(t, []resource.Resource{replayed}, plans)
	w4AssertNoCode(t, replayRes.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
}

// ── row 7: the error answers for the whole page ───────────────────────────

// TestPartialAttackEBS_StatusFailureMarksEveryVolumeInTheBatch attacks the
// batch dimension of the kept-findings fix. One account-wide call answers for
// every row on screen, so its failure leaves all of them uninspected — and
// each keeps whatever the cache-only joins already decided about it.
func TestPartialAttackEBS_StatusFailureMarksEveryVolumeInTheBatch(t *testing.T) {
	// The backup-coverage join builds a volume ARN from the session's region;
	// a session with none answers "cannot tell" (aws5 row 2).
	clients := &awsclient.ServiceClients{Region: "us-east-1", EC2: &partialEBSStatusFake{}}
	store := session.NewIdentityStore()
	store.Set(w7Account, nil)
	clients.SetIdentityStore(store)

	second := w7Volume("in-use")
	second.ID = "vol-0a1b2c3d4e5f60002"
	second.Name = "acme-logs"
	second.Fields = map[string]string{
		"volume_id": second.ID, "name": "acme-logs", "state": "in-use",
		"attached_to": "i-0a1b2c3d4e5f60002", "az": w7Region + "b",
	}

	plans := w7CacheWith(w7Plan("arn:aws:rds:us-east-1:123456789012:db:acme-orders-db", "", ""))
	plans["ebs-snap"] = resource.ResourceCacheEntry{IsTruncated: false}

	res, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients,
		[]resource.Resource{w7Volume("in-use"), second}, plans)
	if err == nil {
		t.Fatal("a denied DescribeVolumeStatus returned no error")
	}

	for _, id := range []string{w7VolumeID, second.ID} {
		w4AssertFinding(t, res.Findings[id], awsclient.CodeEBSNotInBackupPlan,
			"not covered by a backup plan", domain.SevWarn, "wave2")
		partialAssertUninspected(t, res, id)
	}
}

// ── row 8: the third answer, and a repository with two problems ───────────

// TestPartialAttackECR_PublicPolicyAndUnreadableLifecycle pins that the two
// policy reads stay independent after the lifecycle read grew a third answer.
// A repository open to anyone whose lifecycle policy nobody could read carries
// the exposure finding AND the coverage gap; collapsing either into the other
// loses a fact the operator needs.
func TestPartialAttackECR_PublicPolicyAndUnreadableLifecycle(t *testing.T) {
	res := partialEnrichECRAPI(t, &partialECRPublicFake{
		partialECRFake: partialECRFake{lifecycleErr: partialAccessDenied()},
	})
	w4AssertFinding(t, res.Findings[partialECRRepo], "ecr.public-policy",
		"repository policy open to anyone", domain.SevBroken, "wave2")
	w4AssertNoCode(t, res.Findings[partialECRRepo], "ecr.no-lifecycle-policy")
	partialAssertUninspected(t, res, partialECRRepo)
}

// partialECRPublicFake is partialECRFake with a repository policy that lets
// anyone pull.
type partialECRPublicFake struct {
	partialECRFake
}

func (f *partialECRPublicFake) GetRepositoryPolicy(_ context.Context, _ *ecrsvc.GetRepositoryPolicyInput, _ ...func(*ecrsvc.Options)) (*ecrsvc.GetRepositoryPolicyOutput, error) {
	return &ecrsvc.GetRepositoryPolicyOutput{
		RepositoryName: aws.String(partialECRRepo),
		PolicyText: aws.String(`{"Version":"2008-10-17","Statement":[` +
			`{"Sid":"AllowPull","Effect":"Allow","Principal":"*",` +
			`"Action":["ecr:BatchGetImage","ecr:GetDownloadUrlForLayer"]}]}`),
	}, nil
}

// TestPartialAttackECR_ClientWithoutTheCallSaysNothing pins that a client that
// cannot serve the lifecycle read is not a coverage gap. The call is asserted
// rather than required precisely so an older client degrades to silence; a "?"
// there would mark every repository in every such session.
func TestPartialAttackECR_ClientWithoutTheCallSaysNothing(t *testing.T) {
	res := partialEnrichECRAPI(t, &partialECRNoLifecycleAPI{})
	w4AssertNoCode(t, res.Findings[partialECRRepo], "ecr.no-lifecycle-policy")
	partialAssertInspected(t, res, partialECRRepo)
}

// ── row 9: nothing is cached, so nothing is inherited ─────────────────────

// TestPartialAttackRedshift_CutShortWalkPoisonsNoSiblingCluster attacks the
// reason row 9 is about a cache at all. Parameter groups are shared across a
// fleet, and one group is read once per run; caching the empty value would
// hand every other cluster on that group the same wrong "SSL not required".
func TestPartialAttackRedshift_CutShortWalkPoisonsNoSiblingCluster(t *testing.T) {
	fake := &partialRedshiftPagingFake{
		pages:          awsclient.PerParentPageCap + 5,
		requireSSLPage: awsclient.PerParentPageCap + 2,
		requireSSL:     "true",
	}
	ids := []string{"acme-analytics", "acme-reporting", "acme-staging"}
	rows := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, w2Res(id, w2RedshiftCluster(id, partialRedshiftGroup)))
	}

	res, _ := awsclient.EnrichRedshiftPosture(context.Background(),
		&awsclient.ServiceClients{Redshift: fake}, rows, nil)
	w2AssertEnricherShape(t, res)

	for _, id := range ids {
		w4AssertNoCode(t, res.Findings[id], "redshift.require-ssl-off")
		partialAssertUninspected(t, res, id)
	}
}

// TestPartialAttackRedshift_MarkerClearedByAnEmptyStringIsComplete pins the
// boundary between "more pages" and "done". AWS ends a walk with either a nil
// marker or an empty one, and reading the empty string as a continuation would
// make every complete walk report itself cut short.
func TestPartialAttackRedshift_MarkerClearedByAnEmptyStringIsComplete(t *testing.T) {
	const id = "acme-reporting"
	res, _ := awsclient.EnrichRedshiftPosture(context.Background(),
		&awsclient.ServiceClients{Redshift: &partialRedshiftEmptyMarkerFake{}},
		[]resource.Resource{w2Res(id, w2RedshiftCluster(id, partialRedshiftGroup))}, nil)
	w2AssertEnricherShape(t, res)

	w4AssertFinding(t, res.Findings[id], "redshift.require-ssl-off",
		"SSL not required", domain.SevWarn, "wave2")
	w2AssertRow(t, w2Rows(t, res, id, "redshift.require-ssl-off"),
		"Requires encrypted connections", "unset (require_ssl)")
	partialAssertInspected(t, res, id)
}

// partialRedshiftEmptyMarkerFake ends its single page with an empty-string
// marker rather than a nil one, and that page does not carry require_ssl — so
// the walk can only end by reading the marker, not by finding a value.
type partialRedshiftEmptyMarkerFake struct {
	awsclient.RedshiftAPI
}

func (f *partialRedshiftEmptyMarkerFake) DescribeLoggingStatus(_ context.Context, _ *redshiftsvc.DescribeLoggingStatusInput, _ ...func(*redshiftsvc.Options)) (*redshiftsvc.DescribeLoggingStatusOutput, error) {
	return &redshiftsvc.DescribeLoggingStatusOutput{LoggingEnabled: aws.Bool(true), BucketName: aws.String("acme-redshift-audit")}, nil
}

func (f *partialRedshiftEmptyMarkerFake) DescribeClusterParameters(_ context.Context, _ *redshiftsvc.DescribeClusterParametersInput, _ ...func(*redshiftsvc.Options)) (*redshiftsvc.DescribeClusterParametersOutput, error) {
	return &redshiftsvc.DescribeClusterParametersOutput{
		Marker: aws.String(""),
		Parameters: []redshifttypes.Parameter{{
			ParameterName:  aws.String("wlm_json_configuration"),
			ParameterValue: aws.String("[{\"query_concurrency\":5}]"),
			Source:         aws.String("engine-default"),
		}},
	}, nil
}

// partialECRNoLifecycleAPI is the ECR client shape that predates the lifecycle
// call: it serves the images read and the repository policy, and nothing else.
type partialECRNoLifecycleAPI struct {
	awsclient.ECRAPI
}

func (f *partialECRNoLifecycleAPI) DescribeImages(_ context.Context, _ *ecrsvc.DescribeImagesInput, _ ...func(*ecrsvc.Options)) (*ecrsvc.DescribeImagesOutput, error) {
	return &ecrsvc.DescribeImagesOutput{ImageDetails: []ecrtypes.ImageDetail{{
		RepositoryName: aws.String(partialECRRepo),
		ImageDigest:    aws.String("sha256:2222222222222222222222222222222222222222222222222222222222222222"),
		ImageTags:      []string{"1.4.2"},
	}}}, nil
}

func (f *partialECRNoLifecycleAPI) GetRepositoryPolicy(_ context.Context, _ *ecrsvc.GetRepositoryPolicyInput, _ ...func(*ecrsvc.Options)) (*ecrsvc.GetRepositoryPolicyOutput, error) {
	return nil, &ecrtypes.RepositoryPolicyNotFoundException{Message: aws.String("Repository policy does not exist")}
}

// ── the surface: the demo bench still shows what it demonstrates ──────────

// TestPartialAttackEKS_DemoBenchStillPlacesEveryCluster is the surface check
// for the seed change. Making "unknown" the default is only safe because the
// catalogue answers; if the demo's version catalogue ever stops being read,
// every cluster silently slides to unknown, the out-of-support witness stops
// demonstrating anything, and the demo still looks healthy while showing less.
func TestPartialAttackEKS_DemoBenchStillPlacesEveryCluster(t *testing.T) {
	rows := d4Rows(t, "eks")
	if len(rows) == 0 {
		t.Fatal("no demo EKS rows, so this invariant would hold vacuously")
	}

	var carriers []string
	for _, r := range rows {
		if got := r.Fields["version_support"]; got == partialEKSSupportUnknown {
			t.Errorf("demo cluster %s reads %q; the demo catalogue covers every fixture version, so an unknown here means it was not read",
				r.ID, got)
		}
		for _, f := range r.Findings {
			if f.Code == awsclient.CodeEKSVersionUnsupported {
				carriers = append(carriers, r.ID)
			}
		}
	}
	if len(carriers) != 1 {
		t.Errorf("%s fires on %d demo rows, want exactly its witness: %v",
			awsclient.CodeEKSVersionUnsupported, len(carriers), carriers)
	}
}
