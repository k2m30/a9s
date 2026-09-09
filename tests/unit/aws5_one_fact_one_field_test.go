package unit

// aws5_one_fact_one_field_test.go — one field per fact, a region that declines
// instead of guessing, a cause that stays one line, and the backup enricher's
// uninspected marking.
//
// Rows 1-4 of the aws5 spec. Every assertion here is about what a fetcher
// writes, what a related checker answers when the session lost its region,
// what CauseOf renders on a one-line surface, and which plans an uncut backup
// walk leaves marked uninspected.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	backupsdk "github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// ── Row 1: one field per fact ─────────────────────────────────────────────

// aws5NoSecondKey fails when a resource carries a key that names a fact one of
// its other keys already carries.
func aws5NoSecondKey(t *testing.T, r resource.Resource, dropped, kept, wantKept string) {
	t.Helper()
	if v, ok := r.Fields[dropped]; ok {
		t.Errorf("Fields[%q] = %q: the fact is %q's, and two fields for one fact drift", dropped, v, kept)
	}
	if got := r.Fields[kept]; got != wantKept {
		t.Errorf("Fields[%q] = %q, want %q", kept, got, wantKept)
	}
}

// TestLogGroup_RetentionIsOneField pins the log group's retention as one
// field. It was two: a number that was empty for exactly the groups the
// never-expire warning fires on, so the Retention cell went blank beside the
// warning that explains it.
func TestLogGroup_RetentionIsOneField(t *testing.T) {
	mock := &mockCWLogsDescribeLogGroupsClient{
		output: &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []cwlogstypes.LogGroup{
				{LogGroupName: aws.String("/aws/lambda/kept"), RetentionInDays: aws.Int32(30)},
				{LogGroupName: aws.String("/aws/lambda/forever")},
			},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudWatchLogGroupsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("FetchCloudWatchLogGroupsPage: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("got %d resources, want 2", len(resources))
	}
	aws5NoSecondKey(t, resources[0], "retention_days", "retention", "30 days")
	aws5NoSecondKey(t, resources[1], "retention_days", "retention", "never expire")

	td := aws5TypeDef(t, "logs")
	for _, c := range td.Columns {
		if c.Key == "retention_days" {
			t.Errorf("logs column %q still reads the dropped key", c.Title)
		}
	}
	for _, k := range td.FieldKeys {
		if k == "retention_days" {
			t.Errorf("logs FieldKeys still names the dropped key %q", k)
		}
	}
}

// TestS3Bucket_NameIsOneField pins the bucket name as one field. Fields["name"]
// is what the built-in column reads; bucket_name was a second copy of it.
func TestS3Bucket_NameIsOneField(t *testing.T) {
	listMock := &fakeS3ListBuckets{
		Output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{{Name: aws.String("acme-app-state")}},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchS3BucketsPageWithNotifications(context.Background(), listMock, nil, token)
	})
	if err != nil {
		t.Fatalf("FetchS3BucketsPageWithNotifications: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(resources))
	}
	aws5NoSecondKey(t, resources[0], "bucket_name", "name", "acme-app-state")
	for _, k := range aws5TypeDef(t, "s3").FieldKeys {
		if k == "bucket_name" {
			t.Errorf("s3 FieldKeys still names the dropped key %q", k)
		}
	}
}

// TestAsgActivity_StatusIsOneField pins the scaling activity's status as one
// field. status_code is the key both the built-in column and the YAML view
// read; status was the same string under a second name.
func TestAsgActivity_StatusIsOneField(t *testing.T) {
	mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			Activities: []autoscalingtypes.Activity{{
				ActivityId:           aws.String("act-1"),
				AutoScalingGroupName: aws.String("acme-web"),
				StatusCode:           autoscalingtypes.ScalingActivityStatusCodeSuccessful,
				Cause:                aws.String("a scale-out"),
			}},
		},
	}
	result, err := awsclient.FetchAsgActivities(context.Background(), mock,
		map[string]string{"asg_name": "acme-web"}, "")
	if err != nil {
		t.Fatalf("FetchAsgActivities: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(result.Resources))
	}
	aws5NoSecondKey(t, result.Resources[0], "status", "status_code", "Successful")
}

// TestTargetHealth_StatusIsOneField pins the target's health as one field.
// health is the type's LifecycleKey and its column key; status was a copy.
func TestTargetHealth_StatusIsOneField(t *testing.T) {
	mock := &mockELBv2DescribeTargetHealthClient{
		output: &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: []elbv2types.TargetHealthDescription{{
				Target:       &elbv2types.TargetDescription{Id: aws.String("i-0123456789abcdef0"), Port: aws.Int32(80)},
				TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy},
			}},
		},
	}
	result, err := awsclient.FetchTargetHealth(context.Background(), mock,
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme/0123456789abcdef", "")
	if err != nil {
		t.Fatalf("FetchTargetHealth: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(result.Resources))
	}
	aws5NoSecondKey(t, result.Resources[0], "status", "health", "healthy")
}

// TestCTEvent_EventTimeIsOneField pins the event's raw timestamp as one field.
// event_time is what the YAML view sorts on; event_time_raw was the same
// string under a second name and nothing read it.
func TestCTEvent_EventTimeIsOneField(t *testing.T) {
	when := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{{
				EventId:   aws.String("evt-0001"),
				EventName: aws.String("RunInstances"),
				EventTime: &when,
			}},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudTrailEventsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(resources))
	}
	if _, ok := resources[0].Fields["event_time_raw"]; ok {
		t.Errorf("Fields[\"event_time_raw\"] still set: event_time already carries the raw timestamp")
	}
	if resources[0].Fields["event_time"] == "" {
		t.Errorf("Fields[\"event_time\"] is empty")
	}
	for _, k := range aws5TypeDef(t, "ct-events").FieldKeys {
		if k == "event_time_raw" {
			t.Errorf("ct-events FieldKeys still names the dropped key %q", k)
		}
	}
}

func aws5TypeDef(t *testing.T, short string) resource.ResourceTypeDef {
	t.Helper()
	for _, d := range resource.AllResourceTypes() {
		if d.ShortName == short {
			return d
		}
	}
	t.Fatalf("no registered resource type %q", short)
	return resource.ResourceTypeDef{}
}

// ── Row 2: a region that declines instead of guessing ─────────────────────

// aws5RelatedChecker is checkerByTarget for this package: the existing one
// lives in the external unit_test package, and the fetcher mocks these rows
// need live in this one.
func aws5RelatedChecker(t *testing.T, sourceType, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(sourceType) {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("%s related checker for %q is nil", sourceType, target)
			}
			return def.Checker
		}
	}
	t.Fatalf("%s related checker for %q not found", sourceType, target)
	return nil
}

// TestS3BackupPivot_DeclinesWhenTheSessionHasNoRegion pins the decline. An S3
// bucket ARN names no region but it does name a partition, and a session that
// recorded no region cannot say which. Answering the commercial one makes a
// China or GovCloud bucket read as a proven "not backed up".
func TestS3BackupPivot_DeclinesWhenTheSessionHasNoRegion(t *testing.T) {
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID:     "plan-s3",
			Fields: map[string]string{"resource_arn": "arn:aws:s3:::acme-app-state"},
		}}},
	}
	bucket := resource.Resource{ID: "acme-app-state", Name: "acme-app-state"}
	result := aws5RelatedChecker(t, "s3", "backup")(
		context.Background(), &awsclient.ServiceClients{}, bucket, cache)
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v (count %d), want unknown: a session with no region cannot name the partition",
			result.State(), result.Count())
	}
}

// TestS3BackupPivot_NamesTheSessionsPartition is the same walk in a session
// that did record its region: the ARN carries that region's partition.
func TestS3BackupPivot_NamesTheSessionsPartition(t *testing.T) {
	for _, tc := range []struct{ region, arn string }{
		{"us-east-1", "arn:aws:s3:::acme-app-state"},
		{"cn-north-1", "arn:aws-cn:s3:::acme-app-state"},
		{"us-gov-west-1", "arn:aws-us-gov:s3:::acme-app-state"},
	} {
		t.Run(tc.region, func(t *testing.T) {
			cache := resource.ResourceCache{
				"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{{
					ID:     "plan-s3",
					Fields: map[string]string{"resource_arn": tc.arn},
				}}},
			}
			bucket := resource.Resource{ID: "acme-app-state", Name: "acme-app-state"}
			result := aws5RelatedChecker(t, "s3", "backup")(
				context.Background(), &awsclient.ServiceClients{Region: tc.region}, bucket, cache)
			if result.Count() != 1 {
				t.Errorf("Count = %d, want 1 for a plan naming %s", result.Count(), tc.arn)
			}
		})
	}
}

// aws5GlueFake answers GetTags for one ARN and nothing else. A job ARN built
// from the wrong region simply misses, which is the confident zero this row
// is about.
type aws5GlueFake struct {
	awsclient.GlueAPI
	arn  string
	tags map[string]string
}

func (f *aws5GlueFake) GetTags(_ context.Context, in *glue.GetTagsInput, _ ...func(*glue.Options)) (*glue.GetTagsOutput, error) {
	if in != nil && in.ResourceArn != nil && *in.ResourceArn == f.arn {
		return &glue.GetTagsOutput{Tags: f.tags}, nil
	}
	return &glue.GetTagsOutput{}, nil
}

// TestGlueCFNPivot_DeclinesWhenTheSessionHasNoRegion pins the second caller.
// A job ARN carries the region in a segment of its own, so a session with no
// region can only build one that matches nothing.
func TestGlueCFNPivot_DeclinesWhenTheSessionHasNoRegion(t *testing.T) {
	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	clients := &awsclient.ServiceClients{Glue: &aws5GlueFake{
		arn:  "arn:aws:glue:eu-west-2:123456789012:job/acme-etl-job",
		tags: map[string]string{"aws:cloudformation:stack-name": "acme-etl"},
	}}
	clients.SetIdentityStore(store)

	job := resource.Resource{ID: "acme-etl-job", Name: "acme-etl-job"}
	result := aws5RelatedChecker(t, "glue", "cfn")(
		context.Background(), clients, job, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v (count %d), want unknown: a session with no region cannot build a job ARN",
			result.State(), result.Count())
	}
}

// ── Row 3: a cause that stays one line ────────────────────────────────────

// TestCauseOf_UnmodeledResponseKeepsItsFirstClause pins the fallback for a
// response the SDK could not model. Its words are the service's, not a9s's,
// and a future one could carry a host and a port; a flash and a menu row are
// one line each, so the cause is the clause that leads.
func TestCauseOf_UnmodeledResponseKeepsItsFirstClause(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"colon", "connect failed: dial 10.0.0.1:443", "connect failed"},
		{"semicolon", "read config; /home/someone/.aws/config", "read config"},
		{"newline", "walk stopped\nat page 3", "walk stopped"},
		{"no separator", "the walk stopped", "the walk stopped"},
		// A response whose text leads with the separator has no first clause,
		// and an empty cause reads as a failure with no reason at all.
		{"leading separator", ": no clause of its own", ": no clause of its own"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := &smithy.OperationError{
				ServiceID:     "Backup",
				OperationName: "ListBackupJobs",
				Err:           errors.New(tc.in),
			}
			if got := awsclient.CauseOf(err); got != tc.want {
				t.Errorf("CauseOf(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCauseOf_ComposedSentenceKeepsBothClauses is the other half of the same
// decision: a9s's own composites are unclassed too, and their second clause
// is the whole diagnostic. Cutting every unclassed error at the first colon
// left "fetch ec2: partial failure" where the operator needed the reason.
func TestCauseOf_ComposedSentenceKeepsBothClauses(t *testing.T) {
	const sentence = "partial failure: one item timed out"
	if got := awsclient.CauseOf(errors.New(sentence)); got != sentence {
		t.Errorf("CauseOf(%q) = %q, want it whole", sentence, got)
	}
}

// TestCauseOf_JoinedAggregatesStayOneLine pins the separator. Two aggregates
// joined with errors.Join render with a newline between them, and a one-line
// surface would show the second line as a line of its own.
func TestCauseOf_JoinedAggregatesStayOneLine(t *testing.T) {
	denied := &aws5APIErr{code: "AccessDenied", msg: "not authorized to perform: kms:DescribeKey"}
	a := awsclient.AggregateFailures("kms FetchByIDs", []awsclient.Failure{awsclient.FailedCall("key-1", denied)}, 1)
	b := awsclient.AggregateFailures("kms ListAliases", []awsclient.Failure{awsclient.FailedCall("key-2", denied)}, 1)
	got := awsclient.CauseOf(errors.Join(a, b))
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("CauseOf(join) = %q: a one-line surface cannot render a second line", got)
	}
	if !strings.Contains(got, "kms FetchByIDs") || !strings.Contains(got, "kms ListAliases") {
		t.Errorf("CauseOf(join) = %q: both aggregates must survive the join", got)
	}
}

type aws5APIErr struct {
	code string
	msg  string
}

func (e *aws5APIErr) Error() string                 { return e.code + ": " + e.msg }
func (e *aws5APIErr) ErrorCode() string             { return e.code }
func (e *aws5APIErr) ErrorMessage() string          { return e.msg }
func (e *aws5APIErr) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// ── Row 4: an uncut backup walk leaves no plan uninspected ────────────────

type aws5BackupFake struct {
	awsclient.BackupAPI
	jobs []backuptypes.BackupJob
}

func (f *aws5BackupFake) ListBackupJobs(_ context.Context, _ *backupsdk.ListBackupJobsInput, _ ...func(*backupsdk.Options)) (*backupsdk.ListBackupJobsOutput, error) {
	return &backupsdk.ListBackupJobsOutput{BackupJobs: f.jobs}, nil
}

// TestEnrichBackupJobs_UncutWalkLeavesNoPlanUninspected is the row-4 check.
// The walk marks every plan uninspected when it is cut, because a plan's jobs
// are spread over the pages; the question is whether a plan with no job on a
// completed walk is marked too, which would render a healthy plan as "?".
func TestEnrichBackupJobs_UncutWalkLeavesNoPlanUninspected(t *testing.T) {
	now := time.Now()
	rows := []resource.Resource{
		{ID: "plan-with-jobs", Name: "plan-with-jobs", Type: "backup"},
		{ID: "plan-with-none", Name: "plan-with-none", Type: "backup"},
	}
	fake := &aws5BackupFake{jobs: []backuptypes.BackupJob{{
		BackupJobId:  aws.String("job-1"),
		State:        backuptypes.BackupJobStateCompleted,
		CreationDate: &now,
		CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-with-jobs")},
	}}}
	result, err := awsclient.EnrichBackupJobs(context.Background(),
		&awsclient.ServiceClients{Backup: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs: %v", err)
	}
	if result.Truncated {
		t.Errorf("Truncated = true on a walk that read its last page")
	}
	for _, r := range rows {
		if _, marked := result.TruncatedIDs[r.ID]; marked {
			t.Errorf("plan %q marked uninspected on a walk that read its last page", r.ID)
		}
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings = %v, want none: every job completed", result.Findings)
	}
}
