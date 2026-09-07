package unit

// prowler_w6a_monitoring_test.go — batch w6a rows 1-7: the CloudTrail,
// CloudWatch Logs and CloudWatch alarm posture signals.
//
// Rows 1-3 and 6-7 are Wave-1: everything they need is already in the
// list/describe payload the fetcher holds, so the tests drive the real
// fetcher and read the findings off the produced resource. Rows 4-5 are the
// cache-only trail enricher, driven through the catalog registration so the
// enricher is proven reachable and not merely defined.
//
// Assertions are on literal code and phrase strings, never on the production
// constants, so a silent rename is caught rather than followed.
//
// The resource names here are deliberately NOT the fixture witness constants.
// These drive the real fetcher over hand-built SDK input to pin the predicate;
// they say nothing about whether a demo row exists. Naming them after the
// witnesses is what let four constants point at rows no fixture built while
// these tests stayed green. The bench half is
// TestW6AEveryFindingFiresOnItsNamedWitnessOnly, which reads the fixture
// store.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w6aTrailNoCWLogs       = "trail.no-cloudwatch-logs"
	w6aTrailNoKMS          = "trail.no-kms"
	w6aTrailBucketPublic   = "trail.log-bucket-public"
	w6aTrailBucketNoAccess = "trail.log-bucket-no-access-logging"
	w6aLogsNoKMS           = "logs.no-kms"
	w6aAlarmActionsOff     = "alarm.actions-disabled"
)

// ---------------------------------------------------------------------------
// trail — rows 1-3 (wave 1)
// ---------------------------------------------------------------------------

// w6aTrailFake answers DescribeTrails with the given list. GetTrailStatus
// reports a healthy, currently-logging trail so no runtime finding competes
// with the posture rows under test.
type w6aTrailFake struct {
	trails []cttypes.Trail
}

func (f *w6aTrailFake) DescribeTrails(_ context.Context, _ *cloudtrail.DescribeTrailsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
	return &cloudtrail.DescribeTrailsOutput{TrailList: f.trails}, nil
}

func (f *w6aTrailFake) GetTrailStatus(_ context.Context, _ *cloudtrail.GetTrailStatusInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error) {
	return &cloudtrail.GetTrailStatusOutput{IsLogging: aws.Bool(true)}, nil
}

// w6aTrail returns a healthy trail: multi-region, delivering to CloudWatch
// Logs, KMS-encrypted, with log file validation on. Each test switches off
// exactly the setting it is about.
func w6aTrail(name string) cttypes.Trail {
	return cttypes.Trail{
		Name:                       aws.String(name),
		TrailARN:                   aws.String("arn:aws:cloudtrail:eu-central-1:123456789012:trail/" + name),
		HomeRegion:                 aws.String("eu-central-1"),
		S3BucketName:               aws.String("acme-audit-logs"),
		S3KeyPrefix:                aws.String("cloudtrail"),
		IsMultiRegionTrail:         aws.Bool(true),
		IsOrganizationTrail:        aws.Bool(false),
		LogFileValidationEnabled:   aws.Bool(true),
		IncludeGlobalServiceEvents: aws.Bool(true),
		CloudWatchLogsLogGroupArn:  aws.String("arn:aws:logs:eu-central-1:123456789012:log-group:/aws/cloudtrail/" + name + ":*"),
		CloudWatchLogsRoleArn:      aws.String("arn:aws:iam::123456789012:role/CloudTrailToCloudWatch"),
		KmsKeyId:                   aws.String("arn:aws:kms:eu-central-1:123456789012:key/1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"),
	}
}

func w6aFetchTrails(t *testing.T, trails ...cttypes.Trail) map[string]resource.Resource {
	t.Helper()
	rs, err := awsclient.FetchCloudTrailTrails(context.Background(), &w6aTrailFake{trails: trails})
	if err != nil {
		t.Fatalf("FetchCloudTrailTrails: %v", err)
	}
	byID := make(map[string]resource.Resource, len(rs))
	for _, r := range rs {
		byID[r.ID] = r
	}
	return byID
}

// TestW6ATrailNoCloudWatchLogs pins row 2. A trail that writes only to S3 has
// no live stream to alarm on, so the delivery target is the evidence.
func TestW6ATrailNoCloudWatchLogs(t *testing.T) {
	silent := w6aTrail("unit-no-cwlogs-trail")
	silent.CloudWatchLogsLogGroupArn = nil
	empty := w6aTrail("acme-empty-arn-trail")
	empty.CloudWatchLogsLogGroupArn = aws.String("")

	got := w6aFetchTrails(t, silent, empty, w6aTrail("acme-healthy-trail"))

	for _, id := range []string{"unit-no-cwlogs-trail", "acme-empty-arn-trail"} {
		w2AssertFinding(t, got[id].Findings, w6aTrailNoCWLogs,
			"not delivering to CloudWatch Logs", domain.SevWarn, "wave1")
	}
	w2AssertNoCode(t, got["acme-healthy-trail"].Findings, w6aTrailNoCWLogs)
	w6aAssertRow(t, got["unit-no-cwlogs-trail"], w6aTrailNoCWLogs, "Log group", "none")
}

// TestW6ATrailNoKMS pins row 3.
func TestW6ATrailNoKMS(t *testing.T) {
	plain := w6aTrail("unit-no-kms-trail")
	plain.KmsKeyId = nil

	got := w6aFetchTrails(t, plain, w6aTrail("acme-healthy-trail"))

	w2AssertFinding(t, got["unit-no-kms-trail"].Findings, w6aTrailNoKMS,
		"log files not KMS-encrypted", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-healthy-trail"].Findings, w6aTrailNoKMS)
	w6aAssertRow(t, got["unit-no-kms-trail"], w6aTrailNoKMS, "KMS key", "none")
}

// TestW6ATrail_ConditionsAreIndependent pins contract rule 4 on trail: three
// misconfigurations on one trail produce three findings, not the first one
// the fetcher happens to evaluate.
func TestW6ATrail_ConditionsAreIndependent(t *testing.T) {
	bad := w6aTrail("acme-neglected-trail")
	bad.CloudWatchLogsLogGroupArn = nil
	bad.KmsKeyId = nil

	got := w6aFetchTrails(t, bad)
	fs := got["acme-neglected-trail"].Findings
	for _, code := range []string{w6aTrailNoCWLogs, w6aTrailNoKMS} {
		if _, ok := w2Find(fs, code); !ok {
			t.Errorf("missing %q; got %v", code, w2Codes(fs))
		}
	}
}

// TestW6ATrail_CatalogDefs pins the catalog rows for the wave-1 trail codes.
func TestW6ATrail_CatalogDefs(t *testing.T) {
	w2AssertFindingDef(t, "trail", w6aTrailNoCWLogs, "not delivering to CloudWatch Logs", domain.SevWarn, "wave1")
	w2AssertFindingDef(t, "trail", w6aTrailNoKMS, "log files not KMS-encrypted", domain.SevWarn, "wave1")
}

// ---------------------------------------------------------------------------
// trail — rows 4-5 (wave 2, read from S3 directly)
// ---------------------------------------------------------------------------

// w6aTrailS3Fake answers the two read-only bucket calls the trail enricher
// makes. Buckets default to private with access logging on, so each test
// switches off only the setting it is about.
type w6aTrailS3Fake struct {
	awsclient.S3API

	public     map[string]bool
	unlogged   map[string]bool
	statusErr  map[string]error
	loggingErr map[string]error
}

func (f *w6aTrailS3Fake) GetBucketPolicyStatus(_ context.Context, in *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	b := aws.ToString(in.Bucket)
	if err, ok := f.statusErr[b]; ok {
		return nil, err
	}
	return &s3.GetBucketPolicyStatusOutput{
		PolicyStatus: &s3types.PolicyStatus{IsPublic: aws.Bool(f.public[b])},
	}, nil
}

func (f *w6aTrailS3Fake) GetBucketLogging(_ context.Context, in *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	b := aws.ToString(in.Bucket)
	if err, ok := f.loggingErr[b]; ok {
		return nil, err
	}
	if f.unlogged[b] {
		return &s3.GetBucketLoggingOutput{}, nil
	}
	return &s3.GetBucketLoggingOutput{
		LoggingEnabled: &s3types.LoggingEnabled{
			TargetBucket: aws.String("acme-access-logs"),
			TargetPrefix: aws.String("cloudtrail/"),
		},
	}, nil
}

// w6aTrailRes builds the trail row shape the enricher consumes: the fetcher
// retains the SDK trail, and the log bucket name is read off Fields.
func w6aTrailRes(name, bucket string) resource.Resource {
	tr := w6aTrail(name)
	tr.S3BucketName = aws.String(bucket)
	r := w2Res(name, tr)
	r.Fields["s3_bucket"] = bucket
	return r
}

func w6aEnrichTrail(t *testing.T, fake *w6aTrailS3Fake, rs ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w2Enricher(t, "trail")(context.Background(), &awsclient.ServiceClients{S3: fake}, rs, nil)
	w2AssertEnricherInvariants(t, res, err)
	return res
}

// w6aEnrichTrailErr is w6aEnrichTrail for the case that expects a refusal: a
// bucket the role may not read is a recorded failure ("skipped" spec row 5),
// so the shape half of the invariants is checked and the error is returned.
func w6aEnrichTrailErr(t *testing.T, fake *w6aTrailS3Fake, rs ...resource.Resource) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	res, err := w2Enricher(t, "trail")(context.Background(), &awsclient.ServiceClients{S3: fake}, rs, nil)
	w2AssertEnricherShape(t, res)
	return res, err
}

// TestW6ATrailLogBucketPublic pins row 4. A publicly readable log bucket hands
// the account's audit trail to anyone, so this is the batch's one Broken trail
// row.
func TestW6ATrailLogBucketPublic(t *testing.T) {
	res := w6aEnrichTrail(t,
		&w6aTrailS3Fake{public: map[string]bool{"acme-public-audit-logs": true}},
		w6aTrailRes("acme-public-bucket-trail", "acme-public-audit-logs"),
		w6aTrailRes("acme-healthy-trail", "acme-private-audit-logs"),
	)

	w2AssertFinding(t, res.Findings["acme-public-bucket-trail"], w6aTrailBucketPublic,
		"log bucket is publicly accessible", domain.SevBroken, "wave2:trail")
	w2AssertNoCode(t, res.Findings["acme-healthy-trail"], w6aTrailBucketPublic)
	w2AssertRow(t, w2Rows(t, res, "acme-public-bucket-trail", w6aTrailBucketPublic),
		"Bucket", "acme-public-audit-logs")
}

// TestW6ATrailLogBucketNoAccessLogging pins row 5.
func TestW6ATrailLogBucketNoAccessLogging(t *testing.T) {
	res := w6aEnrichTrail(t,
		&w6aTrailS3Fake{unlogged: map[string]bool{"acme-unlogged-audit-logs": true}},
		w6aTrailRes("acme-unlogged-bucket-trail", "acme-unlogged-audit-logs"),
		w6aTrailRes("acme-healthy-trail", "acme-private-audit-logs"),
	)

	w2AssertFinding(t, res.Findings["acme-unlogged-bucket-trail"], w6aTrailBucketNoAccess,
		"log bucket has no access logging", domain.SevWarn, "wave2:trail")
	w2AssertNoCode(t, res.Findings["acme-healthy-trail"], w6aTrailBucketNoAccess)
	w2AssertRow(t, w2Rows(t, res, "acme-unlogged-bucket-trail", w6aTrailBucketNoAccess),
		"Bucket", "acme-unlogged-audit-logs")
}

// TestW6ATrailLogBucket_BothConditionsOnOneBucket pins contract rule 4 across
// the wave-2 pair: one bucket that is both public and unlogged produces both
// findings on the trail that writes to it.
func TestW6ATrailLogBucket_BothConditionsOnOneBucket(t *testing.T) {
	res := w6aEnrichTrail(t,
		&w6aTrailS3Fake{
			public:   map[string]bool{"acme-worst-audit-logs": true},
			unlogged: map[string]bool{"acme-worst-audit-logs": true},
		},
		w6aTrailRes("acme-worst-trail", "acme-worst-audit-logs"),
	)

	fs := res.Findings["acme-worst-trail"]
	for _, code := range []string{w6aTrailBucketPublic, w6aTrailBucketNoAccess} {
		if _, ok := w2Find(fs, code); !ok {
			t.Errorf("missing %q; got %v", code, w2Codes(fs))
		}
	}
}

// TestW6ATrailLogBucket_NoBucketPolicyIsNotPublic pins the one S3 error that
// is an answer rather than a failure: NoSuchBucketPolicy means there is no
// policy to make the bucket public, so the trail is clean and not unknown.
func TestW6ATrailLogBucket_NoBucketPolicyIsNotPublic(t *testing.T) {
	res := w6aEnrichTrail(t,
		&w6aTrailS3Fake{statusErr: map[string]error{
			"acme-nopolicy-audit-logs": &s3types.NoSuchBucket{Message: aws.String("NoSuchBucketPolicy")},
		}},
		w6aTrailRes("acme-nopolicy-trail", "acme-nopolicy-audit-logs"),
	)
	w2AssertNoCode(t, res.Findings["acme-nopolicy-trail"], w6aTrailBucketPublic)
}

// TestW6ATrailLogBucket_UnreadableBucketIsUnknownNotClean pins the other side:
// a bucket whose posture could not be read marks the row truncated rather than
// reporting it healthy, and the other trails in the batch still resolve.
func TestW6ATrailLogBucket_UnreadableBucketIsUnknownNotClean(t *testing.T) {
	res, err := w6aEnrichTrailErr(t,
		&w6aTrailS3Fake{
			statusErr: map[string]error{"acme-denied-audit-logs": errors.New("AccessDenied: not authorized")},
			public:    map[string]bool{"acme-public-audit-logs": true},
		},
		w6aTrailRes("acme-denied-trail", "acme-denied-audit-logs"),
		w6aTrailRes("acme-public-bucket-trail", "acme-public-audit-logs"),
	)

	// INVERTED for the "skipped" spec row 5: the shared helper failed the test
	// on any error, so a refused bucket read was marked "?" and never
	// explained. Do not restore the error-free helper here.
	if err == nil || !strings.Contains(err.Error(), "acme-denied-trail") {
		t.Errorf("trail enricher err = %v, want it to name the trail whose bucket it could not read", err)
	}
	if !res.TruncatedIDs["acme-denied-trail"] {
		t.Error("a trail whose bucket posture could not be read was not marked truncated")
	}
	w2AssertNoCode(t, res.Findings["acme-denied-trail"], w6aTrailBucketPublic)
	w2AssertFinding(t, res.Findings["acme-public-bucket-trail"], w6aTrailBucketPublic,
		"log bucket is publicly accessible", domain.SevBroken, "wave2:trail")
}

// TestW6ATrailLogBucket_CapBoundsTheIssueCount pins that the "!" row makes the
// cap a lower bound on the issue count, so a capped pass says so.
func TestW6ATrailLogBucket_CapBoundsTheIssueCount(t *testing.T) {
	mk := func(n int) []resource.Resource {
		out := make([]resource.Resource, 0, n)
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("acme-trail-%03d", i)
			out = append(out, w6aTrailRes(name, "acme-private-audit-logs"))
		}
		return out
	}
	if res := w6aEnrichTrail(t, &w6aTrailS3Fake{}, mk(awsclient.EnrichmentCap)...); res.Truncated {
		t.Error("exactly EnrichmentCap trails reported Truncated")
	}
	if res := w6aEnrichTrail(t, &w6aTrailS3Fake{}, mk(awsclient.EnrichmentCap+1)...); !res.Truncated {
		t.Error("EnrichmentCap+1 trails did not report Truncated")
	}
}

// TestW6ATrailLogBucket_CatalogDefs pins the wave-2 catalog rows.
func TestW6ATrailLogBucket_CatalogDefs(t *testing.T) {
	w2AssertFindingDef(t, "trail", w6aTrailBucketPublic, "log bucket is publicly accessible", domain.SevBroken, "wave2")
	w2AssertFindingDef(t, "trail", w6aTrailBucketNoAccess, "log bucket has no access logging", domain.SevWarn, "wave2")
}

// ---------------------------------------------------------------------------
// logs — row 6 (wave 1)
// ---------------------------------------------------------------------------

type w6aLogsFake struct {
	groups []cwltypes.LogGroup
}

func (f *w6aLogsFake) DescribeLogGroups(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: f.groups}, nil
}

// w6aLogGroup returns a healthy, customer-key-encrypted log group.
func w6aLogGroup(name string) cwltypes.LogGroup {
	return cwltypes.LogGroup{
		LogGroupName:      aws.String(name),
		Arn:               aws.String("arn:aws:logs:eu-central-1:123456789012:log-group:" + name + ":*"),
		RetentionInDays:   aws.Int32(90),
		StoredBytes:       aws.Int64(4_194_304),
		MetricFilterCount: aws.Int32(0),
		KmsKeyId:          aws.String("arn:aws:kms:eu-central-1:123456789012:key/1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"),
	}
}

func w6aFetchLogGroups(t *testing.T, groups ...cwltypes.LogGroup) map[string]resource.Resource {
	t.Helper()
	out, err := awsclient.FetchCloudWatchLogGroupsPage(context.Background(), &w6aLogsFake{groups: groups}, "")
	if err != nil {
		t.Fatalf("FetchCloudWatchLogGroupsPage: %v", err)
	}
	byID := make(map[string]resource.Resource, len(out.Resources))
	for _, r := range out.Resources {
		byID[r.ID] = r
	}
	return byID
}

// TestW6ALogsNoKMS pins row 6. A log group with no customer key is readable
// by anyone holding logs:GetLogEvents, with no second control in the way.
func TestW6ALogsNoKMS(t *testing.T) {
	plain := w6aLogGroup("/unit/no-kms-group")
	plain.KmsKeyId = nil
	empty := w6aLogGroup("/app/acme-empty-key")
	empty.KmsKeyId = aws.String("")

	got := w6aFetchLogGroups(t, plain, empty, w6aLogGroup("/app/acme-encrypted"))

	for _, id := range []string{"/unit/no-kms-group", "/app/acme-empty-key"} {
		w2AssertFinding(t, got[id].Findings, w6aLogsNoKMS,
			"not encrypted with KMS", domain.SevWarn, "wave1")
	}
	w2AssertNoCode(t, got["/app/acme-encrypted"].Findings, w6aLogsNoKMS)
	w6aAssertRow(t, got["/unit/no-kms-group"], w6aLogsNoKMS, "KMS key", "none")
	w2AssertFindingDef(t, "logs", w6aLogsNoKMS, "not encrypted with KMS", domain.SevWarn, "wave1")
}

// ---------------------------------------------------------------------------
// alarm — row 7 (wave 1)
// ---------------------------------------------------------------------------

type w6aAlarmFake struct {
	alarms []cwtypes.MetricAlarm
}

func (f *w6aAlarmFake) DescribeAlarms(_ context.Context, _ *cloudwatch.DescribeAlarmsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error) {
	return &cloudwatch.DescribeAlarmsOutput{MetricAlarms: f.alarms}, nil
}

// w6aAlarm returns a healthy alarm: in OK state, with two actions configured
// and action execution enabled.
func w6aAlarm(name string) cwtypes.MetricAlarm {
	return cwtypes.MetricAlarm{
		AlarmName:          aws.String(name),
		AlarmArn:           aws.String("arn:aws:cloudwatch:eu-central-1:123456789012:alarm:" + name),
		StateValue:         cwtypes.StateValueOk,
		MetricName:         aws.String("CPUUtilization"),
		Namespace:          aws.String("AWS/EC2"),
		Statistic:          cwtypes.StatisticAverage,
		Threshold:          aws.Float64(80),
		ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
		EvaluationPeriods:  aws.Int32(2),
		Period:             aws.Int32(300),
		ActionsEnabled:     aws.Bool(true),
		AlarmActions: []string{
			"arn:aws:sns:eu-central-1:123456789012:acme-oncall",
			"arn:aws:sns:eu-central-1:123456789012:acme-audit",
		},
	}
}

func w6aFetchAlarms(t *testing.T, alarms ...cwtypes.MetricAlarm) map[string]resource.Resource {
	t.Helper()
	out, err := awsclient.FetchCloudWatchAlarmsPage(context.Background(), &w6aAlarmFake{alarms: alarms}, "")
	if err != nil {
		t.Fatalf("FetchCloudWatchAlarmsPage: %v", err)
	}
	byID := make(map[string]resource.Resource, len(out.Resources))
	for _, r := range out.Resources {
		byID[r.ID] = r
	}
	return byID
}

// TestW6AAlarmActionsDisabled pins row 7. An alarm with actions configured
// but execution switched off looks wired up in the console and pages nobody.
func TestW6AAlarmActionsDisabled(t *testing.T) {
	off := w6aAlarm("unit-actions-disabled-alarm")
	off.ActionsEnabled = aws.Bool(false)

	got := w6aFetchAlarms(t, off, w6aAlarm("acme-healthy-alarm"))

	w2AssertFinding(t, got["unit-actions-disabled-alarm"].Findings, w6aAlarmActionsOff,
		"actions disabled", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-healthy-alarm"].Findings, w6aAlarmActionsOff)
	// The phrase already says the actions are off; the row's job is to say
	// how much is wired behind the switch (U11).
	w6aAssertRow(t, got["unit-actions-disabled-alarm"], w6aAlarmActionsOff, "Configured actions", "2")
	w2AssertFindingDef(t, "alarm", w6aAlarmActionsOff, "actions disabled", domain.SevWarn, "wave1")
}

// TestW6AAlarmActionsDisabled_NilIsUnknown pins the nil rule: DescribeAlarms
// omits ActionsEnabled only when the field was not resolved, and an
// unresolved switch is not evidence that it is off.
func TestW6AAlarmActionsDisabled_NilIsUnknown(t *testing.T) {
	unknown := w6aAlarm("acme-unknown-actions-alarm")
	unknown.ActionsEnabled = nil

	got := w6aFetchAlarms(t, unknown)
	w2AssertNoCode(t, got["acme-unknown-actions-alarm"].Findings, w6aAlarmActionsOff)
}

// TestW6AAlarmActionsDisabled_CoexistsWithNoActions pins that row 7 and the
// existing alarm.no_actions row are independent: an alarm with no actions AND
// execution disabled carries both, because fixing one leaves the other.
func TestW6AAlarmActionsDisabled_CoexistsWithNoActions(t *testing.T) {
	bare := w6aAlarm("acme-inert-alarm")
	bare.ActionsEnabled = aws.Bool(false)
	bare.AlarmActions = nil

	got := w6aFetchAlarms(t, bare)
	fs := got["acme-inert-alarm"].Findings
	if _, ok := w2Find(fs, w6aAlarmActionsOff); !ok {
		t.Errorf("missing %q; got %v", w6aAlarmActionsOff, w2Codes(fs))
	}
	if _, ok := w2Find(fs, "alarm.no_actions"); !ok {
		t.Errorf("alarm.no_actions lost when alarm.actions-disabled was added; got %v", w2Codes(fs))
	}
	w6aAssertRow(t, got["acme-inert-alarm"], w6aAlarmActionsOff, "Configured actions", "0")
}

// ---------------------------------------------------------------------------
// shared
// ---------------------------------------------------------------------------

// w6aAssertRow pins one AttentionDetail row of a wave-1 finding, which the
// fetcher attaches to the resource rather than to an enricher result.
func w6aAssertRow(t *testing.T, r resource.Resource, code, label, value string) {
	t.Helper()
	ad, ok := r.AttentionDetails[domain.FindingCode(code)]
	if !ok {
		t.Fatalf("no AttentionDetail on %q for code %q", r.ID, code)
	}
	w2AssertRow(t, ad.Rows, label, value)
}

// TestW6ATrailLogBucket_MissingBucketIsSkippedNotUnlogged pins the one answer
// the demo fake cannot give: a bucket that does not exist.
//
// S3 returns NoSuchBucket, and the enricher has to mark the trail unknown. The
// demo fake answers every bucket name it is handed — an absent logging config
// reads as an empty GetBucketLogging output, which is indistinguishable from
// "logging is off" — so a trail pointing at an unfixtured bucket would show a
// finding production never emits. Nothing else pins the difference.
func TestW6ATrailLogBucket_MissingBucketIsSkippedNotUnlogged(t *testing.T) {
	gone := &s3types.NoSuchBucket{Message: aws.String("NoSuchBucket: The specified bucket does not exist")}
	res := w6aEnrichTrail(t,
		&w6aTrailS3Fake{
			statusErr:  map[string]error{"acme-deleted-audit-logs": gone},
			loggingErr: map[string]error{"acme-deleted-audit-logs": gone},
		},
		w6aTrailRes("acme-orphan-bucket-trail", "acme-deleted-audit-logs"),
	)

	if !res.TruncatedIDs["acme-orphan-bucket-trail"] {
		t.Error("a trail whose log bucket does not exist was not marked truncated")
	}
	w2AssertNoCode(t, res.Findings["acme-orphan-bucket-trail"], w6aTrailBucketNoAccess)
	w2AssertNoCode(t, res.Findings["acme-orphan-bucket-trail"], w6aTrailBucketPublic)
}
