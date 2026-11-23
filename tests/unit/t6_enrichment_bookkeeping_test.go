package unit_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

// t6App is one session holding short's rows, driven only through the
// executor and the controller the TUI and the web lane share.
type t6App struct {
	short string
	core  *runtime.Core
	c     *app.Controller
}

func t6NewApp(t *testing.T, short string, clients *awsclient.ServiceClients, rows []resource.Resource, siblings resource.ResourceCache, openList bool) *t6App {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "us-east-1"
	s.Clients = clients
	core := runtime.New(s, catalog.All())
	c := newBlessedController(t, core)
	for name, e := range siblings {
		if name == short {
			continue
		}
		core.ObserveRows(name, e.Resources, &resource.PaginationMeta{PageSize: len(e.Resources), TotalHint: len(e.Resources)}, session.OriginProbe, false)
	}
	if openList {
		c.Apply(app.Action{Kind: app.ActionCommand, Arg: short})
		c.ApplyResourcesLoaded(short, rows, nil, false)
	}
	if rows != nil {
		core.ObserveRows(short, rows, &resource.PaginationMeta{PageSize: len(rows), TotalHint: len(rows)}, session.OriginFetch, false)
	}
	return &t6App{short: short, core: core, c: c}
}

func (a *t6App) wave2(t *testing.T) messages.EnrichmentChecked {
	t.Helper()
	ev, err := a.core.ExecuteTaskAt(context.Background(), runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindProbeEnrich, Scope: a.short}}, a.core.CaptureDispatch())
	if err != nil {
		t.Fatalf("%s Wave 2 task: %v", a.short, err)
	}
	checked, ok := ev.(messages.EnrichmentChecked)
	if !ok {
		t.Fatalf("%s Wave 2 task answered %T, want messages.EnrichmentChecked", a.short, ev)
	}
	a.c.Handle(checked)
	return checked
}

// wave1 runs the type's Wave 1 availability probe through the executor and
// delivers it, as a menu refresh does.
func (a *t6App) wave1(t *testing.T) {
	t.Helper()
	ev, err := a.core.ExecuteTaskAt(context.Background(), runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindProbeAvailability, Scope: a.short}}, a.core.CaptureDispatch())
	if err != nil {
		t.Fatalf("%s Wave 1 probe: %v", a.short, err)
	}
	if _, ok := ev.(messages.AvailabilityChecked); !ok {
		t.Fatalf("%s Wave 1 probe answered %T, want messages.AvailabilityChecked", a.short, ev)
	}
	a.c.Handle(ev)
}

func (a *t6App) statusCell(t *testing.T, id string) string {
	t.Helper()
	body := a.c.Snapshot().Body.List
	if body == nil {
		t.Fatalf("%s: no list body", a.short)
	}
	return statusCellFor(t, *body, id)
}

// requireNoListFinding fails the test unless the list row id carries no
// finding of code: the answer that resolved it reached the screen.
func (a *t6App) requireNoListFinding(t *testing.T, id string, code domain.FindingCode) {
	t.Helper()
	for _, r := range a.c.GetListAllResources() {
		if r.ID == id && t576HasCode(r.Findings, code) {
			t.Fatalf("precondition: the list row %s still carries %s, so the second answer did not land", id, code)
		}
	}
}

func (a *t6App) scanOutcome(t *testing.T) runtime.ProbeOutcome {
	t.Helper()
	for _, st := range a.core.ScanStatus() {
		if st.ShortName == a.short {
			return st.Outcome
		}
	}
	t.Fatalf("%s has no scan status", a.short)
	return ""
}

func t6DemoRows(t *testing.T, short string) []resource.Resource {
	t.Helper()
	td := resource.FindResourceType(short)
	if td == nil {
		t.Fatalf("%s not registered", short)
	}
	rows, ok := unit.DrainFixtures(t, *td, demo.NewServiceClients())
	if !ok || len(rows) < 2 {
		t.Fatalf("%s: %d demo rows, the test needs two", short, len(rows))
	}
	return rows
}

// t6BackupJobs answers ListBackupJobs the way AWS does: only the jobs created
// after ByCreatedAfter.
type t6BackupJobs struct {
	awsclient.BackupAPI
	jobs []backuptypes.BackupJob
}

func (f *t6BackupJobs) ListBackupJobs(_ context.Context, in *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	out := &backup.ListBackupJobsOutput{}
	for _, j := range f.jobs {
		if in.ByCreatedAfter == nil || j.CreationDate.After(*in.ByCreatedAfter) {
			out.BackupJobs = append(out.BackupJobs, j)
		}
	}
	return out, nil
}

func t6BackupJob(id, planID string, state backuptypes.BackupJobState, created time.Time) backuptypes.BackupJob {
	return backuptypes.BackupJob{
		BackupJobId:  aws.String(id),
		State:        state,
		CreationDate: aws.Time(created),
		CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String(planID)},
		ResourceArn:  aws.String("arn:aws:rds:us-east-1:123456789012:db:orders-primary"),
		ResourceType: aws.String("RDS"),
	}
}

// The Status cell of a backup plan reports the failed jobs of the last 24
// hours as Wave 2 last answered them: an answer with none leaves it blank.
func TestT6_BackupStatusClearsWhenFailedJobsAgeOut(t *testing.T) {
	rows := t6DemoRows(t, "backup")
	failing, healthy := rows[0].ID, rows[1].ID
	now := time.Now()
	jobs := &t6BackupJobs{jobs: []backuptypes.BackupJob{
		t6BackupJob("job-0001", failing, backuptypes.BackupJobStateFailed, now.Add(-3*time.Hour)),
		t6BackupJob("job-0002", failing, backuptypes.BackupJobStateFailed, now.Add(-5*time.Hour)),
		t6BackupJob("job-0003", healthy, backuptypes.BackupJobStateCompleted, now.Add(-2*time.Hour)),
	}}
	a := t6NewApp(t, "backup", &awsclient.ServiceClients{Backup: jobs}, rows, nil, true)

	a.wave2(t)
	if got := a.statusCell(t, failing); got != "2 jobs failed in last 24h" {
		t.Fatalf("first answer: %s Status cell = %q, want %q", failing, got, "2 jobs failed in last 24h")
	}
	if got := a.statusCell(t, healthy); got != "" {
		t.Errorf("first answer: healthy plan %s Status cell = %q, want blank", healthy, got)
	}

	jobs.jobs[0].CreationDate = aws.Time(now.Add(-26 * time.Hour))
	jobs.jobs[1].CreationDate = aws.Time(now.Add(-28 * time.Hour))
	a.wave2(t)
	a.requireNoListFinding(t, failing, "backup.job-failed")
	if got := a.statusCell(t, failing); got != "" {
		t.Errorf("after the failures aged out: %s Status cell = %q, want blank", failing, got)
	}
	if got := a.statusCell(t, healthy); got != "" {
		t.Errorf("second answer: healthy plan %s Status cell = %q, want blank", healthy, got)
	}
}

type t6S3Posture struct {
	awsclient.S3API
	partial map[string]bool
}

func (f *t6S3Posture) GetPublicAccessBlock(_ context.Context, in *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	on := !f.partial[aws.ToString(in.Bucket)]
	return &s3.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
		BlockPublicAcls:       aws.Bool(on),
		IgnorePublicAcls:      aws.Bool(true),
		BlockPublicPolicy:     aws.Bool(true),
		RestrictPublicBuckets: aws.Bool(true),
	}}, nil
}

func (f *t6S3Posture) GetBucketPolicyStatus(context.Context, *s3.GetBucketPolicyStatusInput, ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return &s3.GetBucketPolicyStatusOutput{PolicyStatus: &s3types.PolicyStatus{IsPublic: aws.Bool(false)}}, nil
}

func (f *t6S3Posture) GetBucketAcl(context.Context, *s3.GetBucketAclInput, ...func(*s3.Options)) (*s3.GetBucketAclOutput, error) {
	owner := "79a59df900b949e55d96a1e698fbacedfd6e09d98eacf8f8d5218e7cd47ef2be"
	return &s3.GetBucketAclOutput{
		Owner: &s3types.Owner{ID: aws.String(owner)},
		Grants: []s3types.Grant{{
			Grantee:    &s3types.Grantee{Type: s3types.TypeCanonicalUser, ID: aws.String(owner)},
			Permission: s3types.PermissionFullControl,
		}},
	}, nil
}

func (f *t6S3Posture) GetBucketVersioning(context.Context, *s3.GetBucketVersioningInput, ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return &s3.GetBucketVersioningOutput{Status: s3types.BucketVersioningStatusEnabled, MFADelete: s3types.MFADeleteStatusEnabled}, nil
}

func (f *t6S3Posture) GetBucketLogging(context.Context, *s3.GetBucketLoggingInput, ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return &s3.GetBucketLoggingOutput{LoggingEnabled: &s3types.LoggingEnabled{TargetBucket: aws.String("acme-access-logs"), TargetPrefix: aws.String("s3/")}}, nil
}

func (f *t6S3Posture) GetBucketLifecycleConfiguration(context.Context, *s3.GetBucketLifecycleConfigurationInput, ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return &s3.GetBucketLifecycleConfigurationOutput{Rules: []s3types.LifecycleRule{{
		ID:         aws.String("expire-noncurrent"),
		Status:     s3types.ExpirationStatusEnabled,
		Expiration: &s3types.LifecycleExpiration{Days: aws.Int32(365)},
	}}}, nil
}

func (f *t6S3Posture) GetObjectLockConfiguration(context.Context, *s3.GetObjectLockConfigurationInput, ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return &s3.GetObjectLockConfigurationOutput{ObjectLockConfiguration: &s3types.ObjectLockConfiguration{ObjectLockEnabled: s3types.ObjectLockEnabledEnabled}}, nil
}

// The Status cell of a bucket reports its public access block as Wave 2 last
// read it: an answer with all four settings on leaves it blank.
func TestT6_S3StatusClearsWhenPublicAccessBlockCompletes(t *testing.T) {
	rows := t6DemoRows(t, "s3")[:2]
	target, healthy := rows[0], rows[1]
	posture := &t6S3Posture{partial: map[string]bool{target.Name: true}}
	a := t6NewApp(t, "s3", &awsclient.ServiceClients{S3: posture}, rows, nil, true)

	a.wave2(t)
	if got := a.statusCell(t, target.ID); got != "public access block incomplete" {
		t.Fatalf("first answer: %s Status cell = %q, want %q", target.ID, got, "public access block incomplete")
	}
	if got := a.statusCell(t, healthy.ID); got != "" {
		t.Errorf("first answer: healthy bucket %s Status cell = %q, want blank", healthy.ID, got)
	}

	posture.partial[target.Name] = false
	a.wave2(t)
	a.requireNoListFinding(t, target.ID, "s3.public-access-block-incomplete")
	if got := a.statusCell(t, target.ID); got != "" {
		t.Errorf("after the block was completed: %s Status cell = %q, want blank", target.ID, got)
	}
	if got := a.statusCell(t, healthy.ID); got != "" {
		t.Errorf("second answer: healthy bucket %s Status cell = %q, want blank", healthy.ID, got)
	}
}

// The trail fields Wave 2 writes are carried over a Wave 1 refresh like every
// other enricher field, so the row keeps them until the next Wave 2 answer.
func TestT6_TrailWave2FieldsSurviveAWave1Refresh(t *testing.T) {
	keys := []string{"is_logging", "latest_delivery_error", "latest_delivery_time"}
	_, cache := buildVisibilityTypeCache(t)
	a := t6NewApp(t, "trail", demo.NewServiceClients(), nil, cache, false)

	a.wave1(t)
	ev := a.wave2(t)
	want := map[string]map[string]string{}
	seen := map[string]bool{}
	for id, fields := range ev.FieldUpdates {
		for _, k := range keys {
			if v := fields[k]; v != "" {
				if want[id] == nil {
					want[id] = map[string]string{}
				}
				want[id][k] = v
				seen[k] = true
			}
		}
	}
	for _, k := range keys {
		if !seen[k] {
			t.Fatalf("the demo trail Wave 2 answer writes no %s; the test has no witness", k)
		}
	}

	gen := a.core.Session().RowStore.Snapshot("trail").Gen
	a.wave1(t)
	if a.core.Session().RowStore.Snapshot("trail").Gen == gen {
		t.Fatal("precondition: the Wave 1 refresh did not land on the stored rows")
	}
	rows, _ := a.core.ProbeResources("trail")
	byID := map[string]resource.Resource{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	for id, fields := range want {
		r, ok := byID[id]
		if !ok {
			t.Errorf("trail %s is gone after the Wave 1 refresh", id)
			continue
		}
		for k, v := range fields {
			if got := r.Fields[k]; got != v {
				t.Errorf("trail %s after a Wave 1 refresh: %s = %q, want the Wave 2 value %q", id, k, got, v)
			}
		}
	}
}

type t6SES struct {
	awsclient.SESv2API
	domains []string
	denied  map[string]bool
}

func (f *t6SES) ListEmailIdentities(context.Context, *sesv2.ListEmailIdentitiesInput, ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error) {
	out := &sesv2.ListEmailIdentitiesOutput{}
	for _, d := range f.domains {
		out.EmailIdentities = append(out.EmailIdentities, sesv2types.IdentityInfo{
			IdentityName:       aws.String(d),
			IdentityType:       sesv2types.IdentityTypeDomain,
			SendingEnabled:     true,
			VerificationStatus: sesv2types.VerificationStatusSuccess,
		})
	}
	return out, nil
}

func (f *t6SES) GetAccount(context.Context, *sesv2.GetAccountInput, ...func(*sesv2.Options)) (*sesv2.GetAccountOutput, error) {
	return &sesv2.GetAccountOutput{
		EnforcementStatus:       aws.String("HEALTHY"),
		ProductionAccessEnabled: true,
		SendingEnabled:          true,
		SendQuota:               &sesv2types.SendQuota{Max24HourSend: 50000, MaxSendRate: 14, SentLast24Hours: 1200},
	}, nil
}

func (f *t6SES) GetEmailIdentity(_ context.Context, in *sesv2.GetEmailIdentityInput, _ ...func(*sesv2.Options)) (*sesv2.GetEmailIdentityOutput, error) {
	if f.denied[aws.ToString(in.EmailIdentity)] {
		return nil, t576Denied("SESv2", "GetEmailIdentity")
	}
	return &sesv2.GetEmailIdentityOutput{
		IdentityType:   sesv2types.IdentityTypeDomain,
		DkimAttributes: &sesv2types.DkimAttributes{SigningEnabled: true, Status: sesv2types.DkimStatusSuccess},
	}, nil
}

func t6SESApp(t *testing.T, n int, denied ...string) *t6App {
	t.Helper()
	f := &t6SES{denied: map[string]bool{}}
	for i := range n {
		f.domains = append(f.domains, fmt.Sprintf("mail%03d.example.com", i))
	}
	for _, d := range denied {
		f.denied[d] = true
	}
	clients := &awsclient.ServiceClients{SESv2: f}
	td := resource.FindResourceType("ses")
	if td == nil {
		t.Fatal("ses not registered")
	}
	res, err := td.Fetcher(context.Background(), clients, "")
	if err != nil || len(res.Resources) != n {
		t.Fatalf("ses fetch: %d identities, err %v; want %d", len(res.Resources), err, n)
	}
	return t6NewApp(t, "ses", clients, res.Resources, nil, false)
}

// ses declares Wave-2 findings at broken severity, so an identity Wave 2 did
// not inspect can hide one: the scan reports partial coverage and the badge is
// a lower bound. An account where every identity was inspected reports
// neither.
func TestT6_SESUninspectedIdentityIsPartialCoverage(t *testing.T) {
	if _, ok := rvBrokenWave2(t, "ses"); !ok {
		t.Fatal("precondition: ses declares no broken-severity Wave-2 finding")
	}
	cases := []struct {
		name    string
		app     func(t *testing.T) *t6App
		partial bool
	}{
		{"one identity read refused", func(t *testing.T) *t6App {
			return t6SESApp(t, 3, "mail001.example.com")
		}, true},
		{"identities past the inspection cap", func(t *testing.T) *t6App {
			return t6SESApp(t, awsclient.EnrichmentCap+1)
		}, true},
		{"every identity inspected", func(t *testing.T) *t6App {
			return t6SESApp(t, 3)
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := tc.app(t)
			ev := a.wave2(t)
			if tc.partial && len(ev.TruncatedIDs) == 0 {
				t.Fatalf("precondition: Wave 2 marked no identity not inspected")
			}
			want := runtime.ProbeOK
			if tc.partial {
				want = runtime.ProbePartial
			}
			if got := a.scanOutcome(t); got != want {
				t.Errorf("ses scan status = %q, want %q", got, want)
			}
			if got := a.c.GetMenuIssueTruncated()["ses"]; got != tc.partial {
				t.Errorf("ses issue badge lower bound = %v, want %v", got, tc.partial)
			}
		})
	}
}

type t6KMS struct {
	awsclient.KMSAPI
}

func (t6KMS) GetKeyRotationStatus(_ context.Context, in *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{KeyId: in.KeyId, KeyRotationEnabled: true, RotationPeriodInDays: aws.Int32(365)}, nil
}

func (t6KMS) GetKeyPolicy(_ context.Context, in *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	return &kms.GetKeyPolicyOutput{
		PolicyName: in.PolicyName,
		Policy:     aws.String(`{"Version":"2012-10-17","Id":"key-default-1","Statement":[{"Sid":"Enable IAM User Permissions","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"kms:*","Resource":"*"}]}`),
	}, nil
}

// t6PastTheCap is the type's demo rows followed by copies of them under new
// ids, enough that the rows the enricher applies to exceed EnrichmentCap and
// the enricher stops before the last of them.
func t6PastTheCap(rows []resource.Resource) []resource.Resource {
	out := append([]resource.Resource(nil), rows...)
	for i := 0; len(out) <= 4*awsclient.EnrichmentCap; i++ {
		r := rows[i%len(rows)]
		r.ID = fmt.Sprintf("%s-copy-%03d", r.ID, i)
		r.Name = r.ID
		out = append(out, r)
	}
	return out
}

// Rows an enricher leaves uninspected — here the rows past its inspection
// cap — make the type's scan partial whatever else the enricher reports, and
// a run that inspected every row reports ok.
func TestT6_MarkedRowsAloneMakeTheScanPartial(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	for _, short := range []string{"cfn", "tg", "glue", "tgw", "pipeline", "asg", "s3", "secrets", "lambda", "athena", "sqs", "kms", "sns"} {
		t.Run(short, func(t *testing.T) {
			rows := byType[short]
			if len(rows) == 0 {
				t.Fatalf("%s has no demo rows", short)
			}
			clients := func() *awsclient.ServiceClients {
				c := demo.NewServiceClients()
				if short == "kms" {
					c.KMS = t6KMS{KMSAPI: c.KMS}
				}
				return c
			}

			clean := t6NewApp(t, short, clients(), rows, cache, false)
			ev := clean.wave2(t)
			cleanCapped := 0
			for _, check := range ev.TruncatedIDs {
				if check == awsclient.CheckCap {
					cleanCapped++
				}
			}
			if ev.Err != nil || cleanCapped != 0 {
				t.Fatalf("precondition: the demo rows are not inspected cleanly: err %v, %d rows marked at the cap", ev.Err, cleanCapped)
			}
			// A demo witness marked for its own reason would make the clean run
			// partial; the clean and the cut runs judge the other rows.
			if len(ev.TruncatedIDs) > 0 {
				rows = slices.DeleteFunc(slices.Clone(rows), func(r resource.Resource) bool {
					_, marked := ev.TruncatedIDs[r.ID]
					return marked
				})
				clean = t6NewApp(t, short, clients(), rows, cache, false)
				clean.wave2(t)
			}
			if got := clean.scanOutcome(t); got != runtime.ProbeOK {
				t.Errorf("every row inspected: scan status = %q, want %q", got, runtime.ProbeOK)
			}

			cut := t6NewApp(t, short, clients(), t6PastTheCap(rows), cache, false)
			ev = cut.wave2(t)
			capped := 0
			for _, check := range ev.TruncatedIDs {
				if check == awsclient.CheckCap {
					capped++
				}
			}
			if ev.Err != nil || capped == 0 {
				t.Fatalf("precondition: the cut is not a clean cap: err %v, %d rows marked at the cap", ev.Err, capped)
			}
			if got := cut.scanOutcome(t); got != runtime.ProbePartial {
				t.Errorf("%d rows left at the inspection cap: scan status = %q, want %q", capped, got, runtime.ProbePartial)
			}
		})
	}
}

// On the demo bench every row carrying the failed-jobs or the incomplete-block
// finding renders it as list and detail render any finding, and a row that
// carries none of its type's findings has a blank Status cell.
func TestT6Demo_StatusFieldWitnessesRender(t *testing.T) {
	for short, code := range map[string]domain.FindingCode{
		"backup": "backup.job-failed",
		"s3":     "s3.public-access-block-incomplete",
	} {
		t.Run(short, func(t *testing.T) {
			td, rows := t563Merged(t, short)
			witnesses, stacked, healthy := 0, 0, 0
			for _, r := range rows {
				switch {
				case t576HasCode(r.Findings, code):
					witnesses++
					if len(r.Findings) > 1 {
						stacked++
					}
					t563AssertWitnessRendered(t, td, rows, r)
				case len(r.Findings) == 0:
					healthy++
					if cell, _ := listStatusCellFor(t, td, rows, r.ID); cell != "" {
						t.Errorf("%s has no finding, Status cell = %q, want blank", r.ID, cell)
					}
				}
			}
			if witnesses == 0 || healthy == 0 {
				t.Fatalf("demo %s: %d rows carry %s and %d carry no finding; the bench needs both", short, witnesses, code, healthy)
			}
			if short == "s3" && stacked == 0 {
				t.Fatalf("no demo bucket carries %s beside another finding; the selector is never put to the test", code)
			}
		})
	}
}
