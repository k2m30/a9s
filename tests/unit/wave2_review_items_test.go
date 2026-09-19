package unit_test

// Wave 2 partial failures. Each test starts from
// a refused, throttled, missing or not-yet-loaded input. The rule is always the same: a check that
// did not answer leaves its row marked, never inspected-and-clean, and the
// badge says whether its count can be short.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	codebuild "github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ec2sdk "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	lambdasvc "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// rvWave2Codes returns shortName's Wave-2 FindingDefs from the catalog.
func rvWave2Codes(t *testing.T, shortName string) []catalog.FindingDef {
	t.Helper()
	for _, td := range catalog.All() {
		if td.ShortName != shortName {
			continue
		}
		var out []catalog.FindingDef
		for _, f := range td.Findings {
			if f.Source == "wave2" {
				out = append(out, f)
			}
		}
		return out
	}
	t.Fatalf("%s is not in the catalog", shortName)
	return nil
}

func rvBrokenWave2(t *testing.T, shortName string) (catalog.FindingDef, bool) {
	t.Helper()
	for _, f := range rvWave2Codes(t, shortName) {
		if f.Severity == domain.SevBroken {
			return f, true
		}
	}
	return catalog.FindingDef{}, false
}

func rvFinding(f catalog.FindingDef) domain.Finding {
	return domain.Finding{Code: f.Code, Phrase: f.Phrase, Detail: f.Detail, Severity: f.Severity, Source: "wave2"}
}

func rvRows(shortName string, ids ...string) []resource.Resource {
	out := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		out = append(out, resource.Resource{ID: id, Name: id, Type: shortName, Fields: map[string]string{"name": id}})
	}
	return out
}

// rvMenuPatch delivers one enrichment answer to a Core holding rows and
// returns the menu badge it produces.
func rvMenuPatch(t *testing.T, core *runtime.Core, ev messages.EnrichmentChecked) runtime.PatchMenu {
	t.Helper()
	intents, _ := core.HandleEvent(ev)
	for _, in := range intents {
		if pm, ok := in.(runtime.PatchMenu); ok && pm.ResourceType == ev.ResourceType {
			return pm
		}
	}
	t.Fatalf("%s: the enrichment answer produced no menu badge; intents were %+v", ev.ResourceType, intents)
	return runtime.PatchMenu{}
}

// TestBadge_RefusedRowOnIssueCapableTypeIsALowerBound: one row's check was
// refused and another row carries a "!" finding. The refused row may hide a
// second one, so the badge reads "at least 1", whatever flag the enricher
// itself raised. A "~"-only type is the counterpart: a refused row there
// cannot hide an issue, and its badge stays exact.
func TestBadge_RefusedRowOnIssueCapableTypeIsALowerBound(t *testing.T) {
	for _, short := range []string{"apigw", "cf", "kms", "msk", "r53", "sns", "sqs", "trail", "sfn"} {
		t.Run(short, func(t *testing.T) {
			def, ok := rvBrokenWave2(t, short)
			if !ok {
				t.Fatalf("%s declares no \"!\" Wave-2 code", short)
			}
			rows := rvRows(short, "acme-"+short+"-refused", "acme-"+short+"-flagged")
			_, core := newTestControllerAndCore(t)
			core.ObserveRows(short, rows, nil, session.OriginFetch, false)

			pm := rvMenuPatch(t, core, messages.EnrichmentChecked{
				ResourceType: short,
				Findings:     map[string][]domain.Finding{rows[1].ID: {rvFinding(def)}},
				TruncatedIDs: map[string]string{rows[0].ID: "GetResourcePolicy"},
			})
			if pm.Issues != 1 || !pm.Truncated {
				t.Errorf("badge = (%d, lower bound %v), want (1, true) — a refused row on a type that raises %s can hide an issue",
					pm.Issues, pm.Truncated, def.Code)
			}
		})
	}

	t.Run("vpc is informational-only", func(t *testing.T) {
		if def, ok := rvBrokenWave2(t, "vpc"); ok {
			t.Fatalf("precondition: vpc declares the \"!\" Wave-2 code %s", def.Code)
		}
		rows := rvRows("vpc", "vpc-0a1b2c3d4e5f60001", "vpc-0a1b2c3d4e5f60002")
		_, core := newTestControllerAndCore(t)
		core.ObserveRows("vpc", rows, nil, session.OriginFetch, false)
		pm := rvMenuPatch(t, core, messages.EnrichmentChecked{
			ResourceType: "vpc",
			Findings:     map[string][]domain.Finding{rows[1].ID: {rvFinding(rvWave2Codes(t, "vpc")[0])}},
			TruncatedIDs: map[string]string{rows[0].ID: "DescribeFlowLogs"},
			Truncated:    true,
		})
		if pm.Truncated {
			t.Error("vpc badge reads as a lower bound, but a refused flow-log check cannot hide an issue")
		}
	})
}

func rvThrottled(service, op string) error {
	return bkOpErr(service, op, "Throttling")
}

type rvThrottledEC2 struct {
	awsclient.EC2API
}

func (rvThrottledEC2) DescribeInstanceStatus(context.Context, *ec2sdk.DescribeInstanceStatusInput, ...func(*ec2sdk.Options)) (*ec2sdk.DescribeInstanceStatusOutput, error) {
	return nil, rvThrottled("EC2", "DescribeInstanceStatus")
}

func (rvThrottledEC2) DescribeVolumeStatus(context.Context, *ec2sdk.DescribeVolumeStatusInput, ...func(*ec2sdk.Options)) (*ec2sdk.DescribeVolumeStatusOutput, error) {
	return nil, rvThrottled("EC2", "DescribeVolumeStatus")
}

type rvThrottledRDS struct {
	awsclient.RDSAPI
}

func (rvThrottledRDS) DescribePendingMaintenanceActions(context.Context, *rds.DescribePendingMaintenanceActionsInput, ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return nil, rvThrottled("RDS", "DescribePendingMaintenanceActions")
}

func (rvThrottledRDS) DescribeDBEngineVersions(context.Context, *rds.DescribeDBEngineVersionsInput, ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return &rds.DescribeDBEngineVersionsOutput{}, nil
}

// TestProbe_ThrottledAccountWalkKeepsThePartialAnswer: the account-wide walk
// stays throttled past every retry. The probe hands the runtime what the
// enricher did establish — here, that every row is uninspected because of
// the walk's call — rather than an empty result the runtime reads as
// "answered nobody", which keeps stale rows for another sweep.
func TestProbe_ThrottledAccountWalkKeepsThePartialAnswer(t *testing.T) {
	defer awsclient.SetRetryConfigForTest(&awsclient.RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond})()

	cases := []struct {
		short   string
		call    string
		clients *awsclient.ServiceClients
		ids     []string
	}{
		{"ebs", "DescribeVolumeStatus", &awsclient.ServiceClients{EC2: rvThrottledEC2{}}, []string{"vol-0a1b2c3d4e5f60001", "vol-0a1b2c3d4e5f60002"}},
		{"ec2", "DescribeInstanceStatus", &awsclient.ServiceClients{EC2: rvThrottledEC2{}}, []string{"i-0aaa111111111111a", "i-0bbb222222222222b"}},
		{"dbi", "DescribePendingMaintenanceActions", &awsclient.ServiceClients{RDS: rvThrottledRDS{}}, []string{"acme-orders-db", "acme-billing-db"}},
	}
	for _, tc := range cases {
		t.Run(tc.short, func(t *testing.T) {
			_, core := newTestControllerAndCore(t)
			core.ObserveRows(tc.short, rvRows(tc.short, tc.ids...), nil, session.OriginFetch, false)

			got := core.ProbeEnrichment(context.Background(), tc.clients, tc.short)

			if got.Err == nil {
				t.Error("a walk throttled past its retries returned no error")
			}
			for _, id := range tc.ids {
				if mark := got.TruncatedIDs[id]; mark != tc.call {
					t.Errorf("%s: TruncatedIDs = %q, want %q — the partial result was thrown away", id, mark, tc.call)
				}
			}
		})
	}
}

type rvCFFake struct {
	awsclient.CloudFrontAPI
}

func (rvCFFake) GetDistributionConfig(_ context.Context, in *cloudfront.GetDistributionConfigInput, _ ...func(*cloudfront.Options)) (*cloudfront.GetDistributionConfigOutput, error) {
	return &cloudfront.GetDistributionConfigOutput{DistributionConfig: &cftypes.DistributionConfig{
		Enabled: aws.Bool(true),
		Origins: &cftypes.Origins{Quantity: aws.Int32(1), Items: []cftypes.Origin{{
			Id:                    aws.String("acme-assets-origin"),
			DomainName:            aws.String("acme-retired-assets.s3.us-east-1.amazonaws.com"),
			OriginAccessControlId: aws.String("E2QWRUHAPOMQZL"),
		}}},
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps},
		ViewerCertificate:    &cftypes.ViewerCertificate{CloudFrontDefaultCertificate: aws.Bool(true)},
	}}, nil
}

type rvHeadBucketFake struct {
	awsclient.S3API
	err error
}

func (f rvHeadBucketFake) HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return nil, f.err
}

// TestCFOriginBucket_UnansweredHeadBucketMarksTheDistribution: the origin
// bucket is not in this account's list, so only HeadBucket can say it is
// gone. Throttled, it said nothing: the distribution is uninspected, and
// neither "missing" nor "fine". NotFound is the counterpart that does answer.
func TestCFOriginBucket_UnansweredHeadBucketMarksTheDistribution(t *testing.T) {
	const dist = "E1A2B3C4D5E6F7"
	cache := resource.ResourceCache{"s3": {Resources: rvRows("s3", "acme-live-assets")}}
	run := func(headErr error) awsclient.IssueEnricherResult {
		res, _ := awsclient.EnrichCloudFrontDistribution(context.Background(), //nolint:errcheck // judged by its marks
			&awsclient.ServiceClients{CloudFront: rvCFFake{}, S3: rvHeadBucketFake{err: headErr}}, rvRows("cf", dist), cache)
		return res
	}

	throttled := run(bkOpErr("S3", "HeadBucket", "SlowDown"))
	if bkHasCode(throttled.Findings[dist], awsclient.CodeCFOriginBucketMissing) {
		t.Errorf("a throttled HeadBucket raised %s", awsclient.CodeCFOriginBucketMissing)
	}
	if got := throttled.TruncatedIDs[dist]; got != "HeadBucket" {
		t.Errorf("TruncatedIDs[%s] = %q after a throttled HeadBucket, want %q — the row reads as a bucket that exists", dist, got, "HeadBucket")
	}

	gone := run(&s3types.NotFound{})
	if !bkHasCode(gone.Findings[dist], awsclient.CodeCFOriginBucketMissing) {
		t.Errorf("HeadBucket answered NotFound, want %s; carries %v", awsclient.CodeCFOriginBucketMissing, bkCodes(gone.Findings[dist]))
	}
}

type rvKMSFake struct {
	awsclient.KMSAPI
	denied map[string]bool
}

func (rvKMSFake) GetKeyPolicy(context.Context, *kms.GetKeyPolicyInput, ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	return &kms.GetKeyPolicyOutput{Policy: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"kms:*","Resource":"*"}]}`)}, nil
}

func (f rvKMSFake) GetKeyRotationStatus(_ context.Context, in *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	if f.denied[aws.ToString(in.KeyId)] {
		return nil, bkOpErr("KMS", "GetKeyRotationStatus", "AccessDeniedException")
	}
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: false}, nil
}

// TestKMSRotation_DeniedOnCustomerKeyIsUninspected: the kms list holds
// customer-managed keys only, so a denial on one of them is a permission the
// session lacks, not an AWS-managed key. The key is uninspected; the key
// whose read succeeded is the counterpart and carries the finding.
func TestKMSRotation_DeniedOnCustomerKeyIsUninspected(t *testing.T) {
	const denied, answered = "1a2b3c4d-0000-4000-8000-00000000000a", "1a2b3c4d-0000-4000-8000-00000000000b"
	keys := []resource.Resource{
		{ID: denied, Name: denied, Type: "kms", Fields: map[string]string{"key_id": denied, "alias": "alias/acme-orders", "status": "Enabled", "manager": "CUSTOMER"}},
		{ID: answered, Name: answered, Type: "kms", Fields: map[string]string{"key_id": answered, "alias": "alias/acme-billing", "status": "Enabled", "manager": "CUSTOMER"}},
	}
	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	clients := &awsclient.ServiceClients{KMS: rvKMSFake{denied: map[string]bool{denied: true}}}
	clients.SetIdentityStore(store)

	res, _ := awsclient.EnrichKMSRotation(context.Background(), clients, keys, nil) //nolint:errcheck // judged by its marks

	if got := res.TruncatedIDs[denied]; got != "GetKeyRotationStatus" {
		t.Errorf("TruncatedIDs[%s] = %q, want %q — the rotation check never ran and the key reads as checked", denied, got, "GetKeyRotationStatus")
	}
	if !bkHasCode(res.Findings[answered], "kms.rotation-disabled") {
		t.Errorf("the answered key carries %v, want kms.rotation-disabled", bkCodes(res.Findings[answered]))
	}
}

// TestBadge_EveryRowRefusedIsNotConfirmedZero: every ec2 row's check was
// refused and no row has an issue. Zero is what was counted, not what was
// found, so the badge is a lower bound live and on disk alike.
func TestBadge_EveryRowRefusedIsNotConfirmedZero(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"
	rows := uninspectedEC2Rows()
	core, ctrl := newLiveWebStyleController(t, profile, region)
	seedFromDisk(ctrl, profile, region)
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	ev := messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings:     map[string][]domain.Finding{},
		TruncatedIDs: map[string]string{rows[0].ID: uninspectedCheck, rows[1].ID: uninspectedCheck},
	}
	intents, _ := core.HandleEvent(ev)
	live := runtime.PatchMenu{}
	for _, in := range intents {
		if pm, ok := in.(runtime.PatchMenu); ok && pm.ResourceType == "ec2" {
			live = pm
		}
	}
	if live.Issues != 0 || !live.Truncated {
		t.Errorf("live badge = (%d, lower bound %v), want (0, true) — every row was refused, so zero is not confirmed", live.Issues, live.Truncated)
	}
	sweepSaveAfterEnrichment(t, core, ctrl, ev)
	if _, _, lower := issuesOnDisk(t, profile, region); !lower {
		t.Error("the badge saved to disk is an exact zero although every row was refused")
	}
}

// TestBadge_EnricherCutWithoutRowMarksSurvivesRestart: the enricher reports a
// cut walk with no per-row marks and one "!" finding. The badge that shows
// live is the badge the next session boots on.
func TestBadge_EnricherCutWithoutRowMarksSurvivesRestart(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"
	rows := uninspectedEC2Rows()
	def, ok := rvBrokenWave2(t, "ec2")
	if !ok {
		t.Fatal("ec2 declares no \"!\" Wave-2 code")
	}
	core, ctrl := newLiveWebStyleController(t, profile, region)
	seedFromDisk(ctrl, profile, region)
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	ev := messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings:     map[string][]domain.Finding{rows[0].ID: {rvFinding(def)}},
		TruncatedIDs: map[string]string{},
		Truncated:    true,
	}
	intents, _ := core.HandleEvent(ev)
	var live runtime.PatchMenu
	for _, in := range intents {
		if pm, ok := in.(runtime.PatchMenu); ok && pm.ResourceType == "ec2" {
			live = pm
		}
	}
	sweepSaveAfterEnrichment(t, core, ctrl, ev)
	issues, _, lower := issuesOnDisk(t, profile, region)
	if live.Issues != 1 || !live.Truncated {
		t.Errorf("live badge = (%d, lower bound %v), want (1, true)", live.Issues, live.Truncated)
	}
	if issues != live.Issues || lower != live.Truncated {
		t.Errorf("disk badge = (%d, lower bound %v), live badge = (%d, lower bound %v) — a restart changes what the badge claims",
			issues, lower, live.Issues, live.Truncated)
	}
}

// TestBadge_OnDemandRowCheckKeepsTheSweepsRule: elb raises only "~" Wave-2
// findings, so rows its sweep did not reach cannot hide an issue and the
// sweep's badge is exact. Answering one of them on demand must not turn the
// badge into a lower bound because the others are still unreached.
func TestBadge_OnDemandRowCheckKeepsTheSweepsRule(t *testing.T) {
	if def, ok := rvBrokenWave2(t, "elb"); ok {
		t.Fatalf("precondition: elb declares the \"!\" Wave-2 code %s", def.Code)
	}
	rows := rvRows("elb", "acme-web-alb", "acme-api-alb", "acme-internal-nlb")
	_, core := newTestControllerAndCore(t)
	core.ObserveRows("elb", rows, nil, session.OriginFetch, false)

	sweep := rvMenuPatch(t, core, messages.EnrichmentChecked{
		ResourceType: "elb",
		Findings:     map[string][]domain.Finding{},
		TruncatedIDs: map[string]string{rows[1].ID: awsclient.CheckCap, rows[2].ID: awsclient.CheckCap},
	})

	intents, _ := core.HandleEvent(messages.RowEnriched{
		ResourceType: "elb",
		ResourceID:   rows[1].ID,
		Findings:     map[string][]domain.Finding{},
		Gen:          core.Session().EnrichmentGen,
		TypeGen:      core.EnrichmentTypeGen("elb"),
	})
	var onDemand *runtime.PatchMenu
	for _, in := range intents {
		if pm, ok := in.(runtime.PatchMenu); ok && pm.ResourceType == "elb" {
			onDemand = &pm
		}
	}
	if onDemand == nil {
		t.Fatalf("the on-demand answer produced no menu badge; intents were %+v", intents)
	}
	if sweep.Truncated || onDemand.Truncated {
		t.Errorf("elb badge lower bound: sweep %v, after one on-demand answer %v — want false both times", sweep.Truncated, onDemand.Truncated)
	}
}

// TestWave2CacheGate_EveryEnricherMarksRowsItCouldNotJudge runs each enricher
// over the demo rows three ways: with every sibling list loaded, with no
// sibling list, and with every sibling list cut short. A finding the full run
// raises must, in the other two, be raised again or sit on a marked row. A
// finding that simply vanishes is a check that was skipped and reported as
// passed.
func TestWave2CacheGate_EveryEnricherMarksRowsItCouldNotJudge(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()

	for _, w := range awsclient.AllWave2() {
		rows := byType[w.ShortName]
		if len(rows) == 0 {
			continue
		}
		own := resource.ResourceCache{w.ShortName: cache[w.ShortName]}
		cut := make(resource.ResourceCache, len(cache))
		for k, e := range cache {
			if k != w.ShortName {
				e.IsTruncated = true
			}
			cut[k] = e
		}
		t.Run(w.ShortName, func(t *testing.T) {
			full, _ := w.Enricher.Fn(context.Background(), clients, rows, cache) //nolint:errcheck // judged by its findings
			for name, variant := range map[string]resource.ResourceCache{"siblings absent": own, "siblings cut": cut} {
				got, _ := w.Enricher.Fn(context.Background(), clients, rows, variant) //nolint:errcheck // judged by its rows
				for _, r := range rows {
					for _, f := range full.Findings[r.ID] {
						if bkHasCode(got.Findings[r.ID], f.Code) {
							continue
						}
						if _, marked := got.TruncatedIDs[r.ID]; marked {
							continue
						}
						t.Errorf("%s, %s: row %q lost %s and carries no not-inspected mark", w.ShortName, name, r.ID, f.Code)
					}
				}
			}
		})
	}
}

type rvRedshiftFake struct {
	awsclient.RedshiftAPI
}

func (rvRedshiftFake) DescribeLoggingStatus(context.Context, *redshift.DescribeLoggingStatusInput, ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error) {
	return &redshift.DescribeLoggingStatusOutput{LoggingEnabled: aws.Bool(false)}, nil
}

// TestRedshiftBadge_CapDoesNotMakeTheCountALowerBound: 51 clusters, audit
// logging off everywhere. Both redshift Wave-2 codes are "~", so the cluster
// past the cap cannot hide an issue and the badge is exact.
func TestRedshiftBadge_CapDoesNotMakeTheCountALowerBound(t *testing.T) {
	if def, ok := rvBrokenWave2(t, "redshift"); ok {
		t.Fatalf("precondition: redshift declares the \"!\" Wave-2 code %s", def.Code)
	}
	ids := make([]string, 0, awsclient.EnrichmentCap+1)
	for i := range awsclient.EnrichmentCap + 1 {
		ids = append(ids, fmt.Sprintf("acme-dw-%03d", i))
	}
	rows := rvRows("redshift", ids...)
	res, _ := awsclient.EnrichRedshiftPosture(context.Background(), &awsclient.ServiceClients{Redshift: rvRedshiftFake{}}, rows, nil) //nolint:errcheck // judged by the badge
	if _, marked := res.TruncatedIDs[ids[awsclient.EnrichmentCap]]; !marked {
		t.Fatalf("precondition: the cluster past the cap is not marked: %v", res.TruncatedIDs)
	}

	_, core := newTestControllerAndCore(t)
	core.ObserveRows("redshift", rows, nil, session.OriginFetch, false)
	pm := rvMenuPatch(t, core, messages.EnrichmentChecked{
		ResourceType: "redshift",
		Findings:     res.Findings,
		TruncatedIDs: res.TruncatedIDs,
		Truncated:    res.Truncated,
	})
	if pm.Truncated {
		t.Error("the redshift badge reads as a lower bound, but a capped cluster can only hide \"~\" findings")
	}
}

type rvLambdaFake struct {
	awsclient.LambdaAPI
	policy string
}

func (f rvLambdaFake) GetPolicy(context.Context, *lambdasvc.GetPolicyInput, ...func(*lambdasvc.Options)) (*lambdasvc.GetPolicyOutput, error) {
	return &lambdasvc.GetPolicyOutput{Policy: aws.String(f.policy), RevisionId: aws.String("a1b2c3d4-1111-2222-3333-444455556666")}, nil
}

func (rvLambdaFake) ListFunctionUrlConfigs(context.Context, *lambdasvc.ListFunctionUrlConfigsInput, ...func(*lambdasvc.Options)) (*lambdasvc.ListFunctionUrlConfigsOutput, error) {
	return &lambdasvc.ListFunctionUrlConfigsOutput{}, nil
}

func (rvLambdaFake) GetFunction(context.Context, *lambdasvc.GetFunctionInput, ...func(*lambdasvc.Options)) (*lambdasvc.GetFunctionOutput, error) {
	return &lambdasvc.GetFunctionOutput{}, nil
}

// TestLambdaPolicy_UnparseablePolicyIsUninspected: GetPolicy answered with a
// document the policy engine rejects. Who may invoke the function is unknown,
// so the row is marked, the way kms and secrets treat the same failure.
func TestLambdaPolicy_UnparseablePolicyIsUninspected(t *testing.T) {
	const fn = "acme-orders-handler"
	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	clients := &awsclient.ServiceClients{Lambda: rvLambdaFake{policy: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":`}}
	clients.SetIdentityStore(store)

	res, _ := awsclient.EnrichLambdaPosture(context.Background(), clients, rvRows("lambda", fn), nil) //nolint:errcheck // judged by its marks

	if _, marked := res.TruncatedIDs[fn]; !marked {
		t.Errorf("%s: an unparseable resource policy left the row unmarked — it reads as \"not public\"", fn)
	}
}

const rvClusterARN = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod"

type rvECSFake struct {
	awsclient.ECSAPI
	missing map[string]bool
}

func (f rvECSFake) DescribeClusters(ctx context.Context, in *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := &ecs.DescribeClustersOutput{}
	for _, name := range in.Clusters {
		if f.missing[name] {
			out.Failures = append(out.Failures, ecstypes.Failure{Arn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/" + name), Reason: aws.String("MISSING")})
			continue
		}
		out.Clusters = append(out.Clusters, ecstypes.Cluster{ClusterName: aws.String(name), ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/" + name), Status: aws.String("ACTIVE")})
	}
	return out, nil
}

func (f rvECSFake) DescribeServices(ctx context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := &ecs.DescribeServicesOutput{}
	for _, name := range in.Services {
		if f.missing[name] {
			out.Failures = append(out.Failures, ecstypes.Failure{Arn: aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-prod/" + name), Reason: aws.String("MISSING")})
			continue
		}
		out.Services = append(out.Services, ecstypes.Service{ServiceName: aws.String(name), Status: aws.String("ACTIVE"), DesiredCount: 2, RunningCount: 2})
	}
	return out, nil
}

func (f rvECSFake) DescribeTasks(ctx context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := &ecs.DescribeTasksOutput{}
	for _, id := range in.Tasks {
		arn := "arn:aws:ecs:us-east-1:123456789012:task/acme-prod/" + id
		if f.missing[id] {
			out.Failures = append(out.Failures, ecstypes.Failure{Arn: aws.String(arn), Reason: aws.String("MISSING")})
			continue
		}
		out.Tasks = append(out.Tasks, ecstypes.Task{TaskArn: aws.String(arn), ClusterArn: aws.String(rvClusterARN), LastStatus: aws.String("RUNNING")})
	}
	return out, nil
}

func rvECSClusters(names ...string) []resource.Resource {
	out := make([]resource.Resource, 0, len(names))
	for _, n := range names {
		out = append(out, resource.Resource{ID: n, Name: n, Type: "ecs", Fields: map[string]string{"cluster_name": n}})
	}
	return out
}

func rvECSServices(names ...string) []resource.Resource {
	out := make([]resource.Resource, 0, len(names))
	for _, n := range names {
		out = append(out, resource.Resource{ID: n, Name: n, Type: "ecs-svc", Fields: map[string]string{"cluster": "acme-prod", "service_name": n}})
	}
	return out
}

func rvECSTasks(ids ...string) []resource.Resource {
	out := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		out = append(out, resource.Resource{ID: id, Name: id, Type: "ecs-task", Fields: map[string]string{"cluster": rvClusterARN, "task_id": id}})
	}
	return out
}

type rvCBFake struct {
	awsclient.CodeBuildAPI
	notFound map[string]bool
}

func (rvCBFake) ListBuildsForProject(_ context.Context, in *codebuild.ListBuildsForProjectInput, _ ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error) {
	return &codebuild.ListBuildsForProjectOutput{Ids: []string{aws.ToString(in.ProjectName) + ":a1b2c3d4-1111-2222-3333-444455556666"}}, nil
}

func (f rvCBFake) BatchGetBuilds(_ context.Context, in *codebuild.BatchGetBuildsInput, _ ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error) {
	out := &codebuild.BatchGetBuildsOutput{}
	for _, id := range in.Ids {
		if f.notFound[id] {
			out.BuildsNotFound = append(out.BuildsNotFound, id)
			continue
		}
		out.Builds = append(out.Builds, cbtypes.Build{Id: aws.String(id), BuildStatus: cbtypes.StatusTypeSucceeded, BuildComplete: true})
	}
	return out, nil
}

// TestBatchDescribe_IDsTheResponseDidNotReturnAreUninspected: the resource
// went away between the list call and the batch describe, and the response
// names it in Failures / BuildsNotFound. Nothing was inspected for it, so
// its row is marked; the ID that came back is the counterpart and is not.
func TestBatchDescribe_IDsTheResponseDidNotReturnAreUninspected(t *testing.T) {
	check := func(t *testing.T, res awsclient.IssueEnricherResult, gone, present string) {
		t.Helper()
		if _, marked := res.TruncatedIDs[gone]; !marked {
			t.Errorf("%s was missing from the batch response yet is not marked — it reads as inspected and healthy", gone)
		}
		if len(res.Findings[gone]) != 0 {
			t.Errorf("%s was missing from the batch response yet carries %v", gone, bkCodes(res.Findings[gone]))
		}
		if mark, marked := res.TruncatedIDs[present]; marked {
			t.Errorf("%s came back in the response yet is marked %q", present, mark)
		}
	}

	t.Run("ecs", func(t *testing.T) {
		res, _ := awsclient.EnrichECSClusters(context.Background(), //nolint:errcheck // judged by its marks
			&awsclient.ServiceClients{ECS: rvECSFake{missing: map[string]bool{"acme-retired": true}}}, rvECSClusters("acme-retired", "acme-prod"), nil)
		check(t, res, "acme-retired", "acme-prod")
	})
	t.Run("ecs-svc", func(t *testing.T) {
		res, _ := awsclient.EnrichECSServices(context.Background(), //nolint:errcheck // judged by its marks
			&awsclient.ServiceClients{ECS: rvECSFake{missing: map[string]bool{"acme-legacy-api": true}}}, rvECSServices("acme-legacy-api", "acme-web"), nil)
		check(t, res, "acme-legacy-api", "acme-web")
	})
	t.Run("ecs-task", func(t *testing.T) {
		const gone, present = "0a1b2c3d4e5f60718293a4b5c6d7e8f9", "1a1b2c3d4e5f60718293a4b5c6d7e8f9"
		res, _ := awsclient.EnrichECSTasks(context.Background(), //nolint:errcheck // judged by its marks
			&awsclient.ServiceClients{ECS: rvECSFake{missing: map[string]bool{gone: true}}}, rvECSTasks(gone, present), nil)
		check(t, res, gone, present)
	})
	t.Run("cb", func(t *testing.T) {
		res, _ := awsclient.EnrichCodeBuildStatus(context.Background(), //nolint:errcheck // judged by its marks
			&awsclient.ServiceClients{CodeBuild: rvCBFake{notFound: map[string]bool{"acme-retired-build:a1b2c3d4-1111-2222-3333-444455556666": true}}},
			rvRows("cb", "acme-retired-build", "acme-web-build"), nil)
		check(t, res, "acme-retired-build", "acme-web-build")
	})
}

// TestECSBatches_DeadlineMarksEveryUnreachedRow: the Wave-2 deadline has
// passed before the batch describe is sent, and the SDK answers with the
// context's error. Every row the batch would have answered for names the
// deadline, like every other Wave-2 loop, not an unnamed failed call.
func TestECSBatches_DeadlineMarksEveryUnreachedRow(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	clients := &awsclient.ServiceClients{ECS: rvECSFake{}}

	clusters, _ := awsclient.EnrichECSClusters(ctx, clients, rvECSClusters("acme-prod", "acme-staging"), nil) //nolint:errcheck // judged by its marks
	for _, id := range []string{"acme-prod", "acme-staging"} {
		if got := clusters.TruncatedIDs[id]; got != awsclient.CheckDeadline {
			t.Errorf("ecs %s: TruncatedIDs = %q, want %q", id, got, awsclient.CheckDeadline)
		}
	}
	services, _ := awsclient.EnrichECSServices(ctx, clients, rvECSServices("acme-web", "acme-api"), nil) //nolint:errcheck // judged by its marks
	for _, id := range []string{"acme-web", "acme-api"} {
		if got := services.TruncatedIDs[id]; got != awsclient.CheckDeadline {
			t.Errorf("ecs-svc %s: TruncatedIDs = %q, want %q", id, got, awsclient.CheckDeadline)
		}
	}
}

// rvOwnRolePolicy grants the account's own role — cross-account only when the
// account is unknown.
const rvOwnRolePolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-app"},"Action":"*","Resource":"*"}]}`

type rvSecretsFake struct {
	awsclient.SecretsManagerAPI
	policies map[string]string
}

func (f rvSecretsFake) GetResourcePolicy(_ context.Context, in *secretsmanager.GetResourcePolicyInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetResourcePolicyOutput, error) {
	out := &secretsmanager.GetResourcePolicyOutput{ARN: in.SecretId}
	if p, ok := f.policies[aws.ToString(in.SecretId)]; ok {
		out.ResourcePolicy = aws.String(p)
	}
	return out, nil
}

type rvDDBFake struct {
	awsclient.DynamoDBAPI
	policies map[string]string
}

func (rvDDBFake) DescribeContinuousBackups(context.Context, *dynamodb.DescribeContinuousBackupsInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	return &dynamodb.DescribeContinuousBackupsOutput{ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
		ContinuousBackupsStatus:        ddbtypes.ContinuousBackupsStatusEnabled,
		PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled},
	}}, nil
}

func (f rvDDBFake) GetResourcePolicy(_ context.Context, in *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetResourcePolicyOutput, error) {
	if p, ok := f.policies[aws.ToString(in.ResourceArn)]; ok {
		return &dynamodb.GetResourcePolicyOutput{Policy: aws.String(p)}, nil
	}
	return nil, bkOpErr("DynamoDB", "GetResourcePolicy", "PolicyNotFoundException")
}

func (rvDDBFake) ListTagsOfResource(context.Context, *dynamodb.ListTagsOfResourceInput, ...func(*dynamodb.Options)) (*dynamodb.ListTagsOfResourceOutput, error) {
	return &dynamodb.ListTagsOfResourceOutput{}, nil
}

// TestOwnAccountFromARN_CrossAccountCheckRunsWithoutSTS: when STS cannot
// name the session's account, the resource's own ARN names the owning
// account, so a policy granting only that account's role is read as it is: no
// cross-account finding and no mark. A resource with no policy stays
// unmarked.
func TestOwnAccountFromARN_CrossAccountCheckRunsWithoutSTS(t *testing.T) {
	unknown := session.NewIdentityStore()
	unknown.Set("", errors.New("sts:GetCallerIdentity refused"))
	t.Run("secrets", func(t *testing.T) {
		const named, bare = "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/orders-db-AbCdEf", "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/billing-db-GhIjKl"
		rows := []resource.Resource{
			{ID: named, Name: "acme/orders-db", Type: "secrets", Fields: map[string]string{"arn": named, "status": "active"}},
			{ID: bare, Name: "acme/billing-db", Type: "secrets", Fields: map[string]string{"arn": bare, "status": "active"}},
		}
		clients := &awsclient.ServiceClients{SecretsManager: rvSecretsFake{policies: map[string]string{named: rvOwnRolePolicy}}}
		clients.SetIdentityStore(unknown)

		res, _ := awsclient.EnrichSecretsPolicy(context.Background(), clients, rows, nil) //nolint:errcheck // judged by its marks

		if bkHasCode(res.Findings[named], "secrets.cross-account-policy") {
			t.Errorf("the secret granting its own account's role is reported as granting another account")
		}
		if mark, marked := res.TruncatedIDs[named]; marked {
			t.Errorf("the secret whose ARN names its account is marked %q", mark)
		}
		if mark, marked := res.TruncatedIDs[bare]; marked {
			t.Errorf("the secret with no policy is marked %q", mark)
		}
	})
	t.Run("ddb", func(t *testing.T) {
		const namedARN, bareARN = "arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders", "arn:aws:dynamodb:us-east-1:123456789012:table/acme-sessions"
		rows := []resource.Resource{
			{ID: "acme-orders", Name: "acme-orders", Type: "ddb", Fields: map[string]string{"arn": namedARN, "status": "ACTIVE"}},
			{ID: "acme-sessions", Name: "acme-sessions", Type: "ddb", Fields: map[string]string{"arn": bareARN, "status": "ACTIVE"}},
		}
		clients := &awsclient.ServiceClients{DynamoDB: rvDDBFake{policies: map[string]string{namedARN: rvOwnRolePolicy}}}
		clients.SetIdentityStore(unknown)

		// A loaded, empty plan list: backup coverage answers "no plan" for
		// both tables, so the only check that can leave a mark is the
		// cross-account one under test.
		backupLoaded := resource.ResourceCache{"backup": {Resources: []resource.Resource{}}}
		res, _ := awsclient.EnrichDynamoDBPITR(context.Background(), clients, rows, backupLoaded) //nolint:errcheck // judged by its marks

		if bkHasCode(res.Findings["acme-orders"], "ddb.cross-account-policy") {
			t.Errorf("the table granting its own account's role is reported as granting another account")
		}
		if mark, marked := res.TruncatedIDs["acme-orders"]; marked {
			t.Errorf("the table whose ARN names its account is marked %q", mark)
		}
		if mark, marked := res.TruncatedIDs["acme-sessions"]; marked {
			t.Errorf("the table with no policy is marked %q", mark)
		}
	})
}

type rvS3PostureFake struct {
	awsclient.S3API
	pabErr error
}

func (f *rvS3PostureFake) GetPublicAccessBlock(context.Context, *s3.GetPublicAccessBlockInput, ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	if f.pabErr != nil {
		return nil, f.pabErr
	}
	return &s3.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
		BlockPublicAcls: aws.Bool(true), IgnorePublicAcls: aws.Bool(false), BlockPublicPolicy: aws.Bool(true), RestrictPublicBuckets: aws.Bool(true),
	}}, nil
}

func (f *rvS3PostureFake) GetBucketPolicyStatus(context.Context, *s3.GetBucketPolicyStatusInput, ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return &s3.GetBucketPolicyStatusOutput{PolicyStatus: &s3types.PolicyStatus{IsPublic: aws.Bool(false)}}, nil
}

func (f *rvS3PostureFake) GetBucketAcl(context.Context, *s3.GetBucketAclInput, ...func(*s3.Options)) (*s3.GetBucketAclOutput, error) {
	return &s3.GetBucketAclOutput{
		Owner: &s3types.Owner{ID: aws.String("79a59df900b949e55d96a1e698fbacedfd6e09d98eacf8f8d5218e7cd47ef2be")},
		Grants: []s3types.Grant{{
			Grantee:    &s3types.Grantee{Type: s3types.TypeGroup, URI: aws.String("http://acs.amazonaws.com/groups/global/AllUsers")},
			Permission: s3types.PermissionRead,
		}},
	}, nil
}

func (f *rvS3PostureFake) GetBucketVersioning(context.Context, *s3.GetBucketVersioningInput, ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return &s3.GetBucketVersioningOutput{Status: s3types.BucketVersioningStatusEnabled, MFADelete: s3types.MFADeleteStatusEnabled}, nil
}

func (f *rvS3PostureFake) GetBucketLogging(context.Context, *s3.GetBucketLoggingInput, ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return &s3.GetBucketLoggingOutput{LoggingEnabled: &s3types.LoggingEnabled{TargetBucket: aws.String("acme-access-logs"), TargetPrefix: aws.String("legacy/")}}, nil
}

func (f *rvS3PostureFake) GetBucketLifecycleConfiguration(context.Context, *s3.GetBucketLifecycleConfigurationInput, ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return &s3.GetBucketLifecycleConfigurationOutput{Rules: []s3types.LifecycleRule{{ID: aws.String("expire-old"), Status: s3types.ExpirationStatusEnabled}}}, nil
}

func (f *rvS3PostureFake) GetObjectLockConfiguration(context.Context, *s3.GetObjectLockConfigurationInput, ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return &s3.GetObjectLockConfigurationOutput{ObjectLockConfiguration: &s3types.ObjectLockConfiguration{ObjectLockEnabled: s3types.ObjectLockEnabledEnabled}}, nil
}

// TestS3Public_ACLRouteNeedsTheBlockItWasJudgedAgainst: the bucket carries a
// legacy AllUsers grant, and GetPublicAccessBlock was refused, so whether S3
// ignores that grant is unknown. The ACL route raises nothing and the row is
// marked. A block read with IgnorePublicAcls off is the counterpart: the
// grant is live and the bucket is public.
func TestS3Public_ACLRouteNeedsTheBlockItWasJudgedAgainst(t *testing.T) {
	const bucket = "acme-legacy-downloads"
	refused, _ := awsclient.EnrichS3Posture(context.Background(), //nolint:errcheck // judged by its findings and marks
		&awsclient.ServiceClients{S3: &rvS3PostureFake{pabErr: bkOpErr("S3", "GetPublicAccessBlock", "AccessDenied")}}, rvRows("s3", bucket), nil)
	if bkHasCode(refused.Findings[bucket], "s3.public") {
		t.Error("GetPublicAccessBlock was refused, yet the ACL grant was judged as if IgnorePublicAcls were off and s3.public raised")
	}
	if got := refused.TruncatedIDs[bucket]; got != "GetPublicAccessBlock" {
		t.Errorf("TruncatedIDs[%s] = %q, want %q", bucket, got, "GetPublicAccessBlock")
	}

	read, _ := awsclient.EnrichS3Posture(context.Background(), //nolint:errcheck // judged by its findings
		&awsclient.ServiceClients{S3: &rvS3PostureFake{}}, rvRows("s3", bucket), nil)
	if !bkHasCode(read.Findings[bucket], "s3.public") {
		t.Errorf("the block reads IgnorePublicAcls=false and the ACL grants AllUsers, want s3.public; carries %v", bkCodes(read.Findings[bucket]))
	}
}
