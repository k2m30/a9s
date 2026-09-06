package unit

// prowler_w6b_glue_test.go — behavioural pins for the three glue posture
// signals of batch w6b.
//
// All three are wave 1: GetJobs returns SecurityConfiguration and
// DefaultArguments inline, so the fetcher already holds everything.
//
// glue.no-security-configuration deliberately folds three Prowler checks (S3,
// CloudWatch-logs and job-bookmark encryption) into one signal. All three
// settings live on the security configuration a job either names or does not,
// so a job that names none fails all three at once and describing the named
// configuration would add a call to split one answer into three.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w6bGlueCodeNoSecurityConfig = domain.FindingCode("glue.no-security-configuration")
	w6bGlueCodeLoggingOff       = domain.FindingCode("glue.continuous-logging-off")
	w6bGlueCodeArgumentSecret   = domain.FindingCode("glue.argument-secret")
)

const (
	w6bGluePhraseNoSecurityConfig = "no security configuration"
	w6bGluePhraseLoggingOff       = "continuous logging off"
	w6bGluePhraseArgumentSecret   = "credential in job arguments"
	w6bGlueContinuousLogArg       = "--enable-continuous-cloudwatch-log"
)

type w6bGlueFake struct {
	jobs []gluetypes.Job
}

func (f *w6bGlueFake) GetJobs(_ context.Context, _ *glue.GetJobsInput, _ ...func(*glue.Options)) (*glue.GetJobsOutput, error) {
	return &glue.GetJobsOutput{Jobs: f.jobs}, nil
}

// w6bGlueJob builds a healthy job: it names a security configuration, turns
// continuous logging on and keeps its credentials in Secrets Manager.
func w6bGlueJob(name string) gluetypes.Job {
	return gluetypes.Job{
		Name:                  aws.String(name),
		Role:                  aws.String("arn:aws:iam::123456789012:role/acme-glue-etl"),
		GlueVersion:           aws.String("4.0"),
		WorkerType:            gluetypes.WorkerTypeG1x,
		NumberOfWorkers:       aws.Int32(4),
		MaxRetries:            1,
		CreatedOn:             aws.Time(time.Now().Add(-180 * 24 * time.Hour)),
		LastModifiedOn:        aws.Time(time.Now().Add(-7 * 24 * time.Hour)),
		Command:               &gluetypes.JobCommand{Name: aws.String("glueetl"), ScriptLocation: aws.String("s3://acme-glue-scripts/" + name + ".py")},
		SecurityConfiguration: aws.String("acme-glue-security"),
		DefaultArguments: map[string]string{
			w6bGlueContinuousLogArg: "true",
			"--job-language":        "python",
			"--db-secret-arn":       "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/warehouse-AbCdEf",
		},
	}
}

func w6bFetchGlue(t *testing.T, jobs ...gluetypes.Job) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchGlueJobsPage(context.Background(), &w6bGlueFake{jobs: jobs}, "")
	if err != nil {
		t.Fatalf("FetchGlueJobsPage: %v", err)
	}
	return out.Resources
}

// ─── row 12: glue.no-security-configuration ─────────────────────────────────

// Without a security configuration the job writes its S3 output, its
// CloudWatch logs and its bookmarks unencrypted.
func TestW6BGlue_NoSecurityConfiguration_Absent(t *testing.T) {
	const name = "acme-data-catalog-crawler"
	job := w6bGlueJob(name)
	job.SecurityConfiguration = nil
	rs := w6bFetchGlue(t, job)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bGlueCodeNoSecurityConfig,
		w6bGluePhraseNoSecurityConfig, domain.SevWarn, "wave1")
	w6bRequireNoRows(t, w6bWave1Rows(r, w6bGlueCodeNoSecurityConfig))
}

// An empty string is the same as absent — AWS returns either shape.
func TestW6BGlue_NoSecurityConfiguration_EmptyString(t *testing.T) {
	const name = "acme-empty-secconf"
	job := w6bGlueJob(name)
	job.SecurityConfiguration = aws.String("")
	rs := w6bFetchGlue(t, job)
	pw1RequireFinding(t, pw1ResourceByID(t, rs, name).Findings,
		w6bGlueCodeNoSecurityConfig, w6bGluePhraseNoSecurityConfig, domain.SevWarn, "wave1")
}

func TestW6BGlue_SecurityConfigurationNamed_IsHealthy(t *testing.T) {
	const name = "acme-etl-clickstream"
	rs := w6bFetchGlue(t, w6bGlueJob(name))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bGlueCodeNoSecurityConfig)
}

// ─── row 13: glue.continuous-logging-off ────────────────────────────────────

// Without continuous logging the job's driver and executor output never
// reaches CloudWatch, so a failed run leaves nothing to read.
func TestW6BGlue_ContinuousLoggingOff_ArgumentUnset(t *testing.T) {
	const name = "acme-etl-clickstream"
	job := w6bGlueJob(name)
	delete(job.DefaultArguments, w6bGlueContinuousLogArg)
	rs := w6bFetchGlue(t, job)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bGlueCodeLoggingOff,
		w6bGluePhraseLoggingOff, domain.SevWarn, "wave1")
	// The row earns its place by naming the job argument to add — the phrase
	// says logging is off, the row says which flag turns it on.
	w6bRequireRowValueContains(t, w6bWave1Rows(r, w6bGlueCodeLoggingOff), w6bGlueContinuousLogArg)
}

// The argument is a string flag, so "false" is off just as surely as absent.
func TestW6BGlue_ContinuousLoggingOff_ArgumentFalse(t *testing.T) {
	const name = "acme-logging-false"
	job := w6bGlueJob(name)
	job.DefaultArguments[w6bGlueContinuousLogArg] = "false"
	rs := w6bFetchGlue(t, job)
	pw1RequireFinding(t, pw1ResourceByID(t, rs, name).Findings,
		w6bGlueCodeLoggingOff, w6bGluePhraseLoggingOff, domain.SevWarn, "wave1")
}

func TestW6BGlue_ContinuousLoggingOn_IsHealthy(t *testing.T) {
	const name = "acme-data-catalog-crawler"
	rs := w6bFetchGlue(t, w6bGlueJob(name))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bGlueCodeLoggingOff)
}

// A job with no default arguments at all has logging off, and must not panic
// on the nil map.
func TestW6BGlue_NilDefaultArguments_LoggingIsOff(t *testing.T) {
	const name = "acme-no-arguments"
	job := w6bGlueJob(name)
	job.DefaultArguments = nil
	rs := w6bFetchGlue(t, job)
	r := pw1ResourceByID(t, rs, name)
	pw1RequireFinding(t, r.Findings, w6bGlueCodeLoggingOff,
		w6bGluePhraseLoggingOff, domain.SevWarn, "wave1")
	pw1RequireNoFinding(t, r.Findings, w6bGlueCodeArgumentSecret)
}

// ─── row 14: glue.argument-secret ───────────────────────────────────────────

// Default arguments are visible to anyone who can call GetJobs and are echoed
// into the job's own logs.
func TestW6BGlue_ArgumentSecret_PlaintextPassword(t *testing.T) {
	const name = "glue-error-run"
	const secret = "hunter2hunter2"
	job := w6bGlueJob(name)
	job.DefaultArguments["--db-password"] = secret
	rs := w6bFetchGlue(t, job)
	r := pw1ResourceByID(t, rs, name)

	f := pw1RequireFinding(t, r.Findings, w6bGlueCodeArgumentSecret,
		w6bGluePhraseArgumentSecret, domain.SevBroken, "wave1")
	rows := w6bWave1Rows(r, w6bGlueCodeArgumentSecret)
	// The leading "--" is Glue's argument syntax, not part of the name an
	// operator recognises, and leaving it on makes the scanner miss the
	// credential keyword entirely.
	pw1RequireRow(t, rows, "db-password", "keyword")
	w6bRequireNoSecretLeak(t, f, rows, secret)
}

// An argument pointing at Secrets Manager is the recommended shape; reporting
// it would flag the fix as the defect.
func TestW6BGlue_ArgumentSecretReference_IsHealthy(t *testing.T) {
	const name = "acme-etl-clickstream"
	rs := w6bFetchGlue(t, w6bGlueJob(name))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bGlueCodeArgumentSecret)
}

// ─── independence ───────────────────────────────────────────────────────────

func TestW6BGlue_AllThreeConditions_ProduceThreeFindings(t *testing.T) {
	const name = "acme-worst-job"
	job := w6bGlueJob(name)
	job.SecurityConfiguration = nil
	job.DefaultArguments = map[string]string{"--api-key": "swordfish99"}
	rs := w6bFetchGlue(t, job)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, w6bGlueCodeNoSecurityConfig, w6bGluePhraseNoSecurityConfig, domain.SevWarn, "wave1")
	pw1RequireFinding(t, r.Findings, w6bGlueCodeLoggingOff, w6bGluePhraseLoggingOff, domain.SevWarn, "wave1")
	pw1RequireFinding(t, r.Findings, w6bGlueCodeArgumentSecret, w6bGluePhraseArgumentSecret, domain.SevBroken, "wave1")
}

// ─── catalog ────────────────────────────────────────────────────────────────

func TestW6BGlue_FindingDefsRegistered(t *testing.T) {
	w2AssertFindingDef(t, "glue", string(w6bGlueCodeNoSecurityConfig), w6bGluePhraseNoSecurityConfig, domain.SevWarn, "wave1")
	w2AssertFindingDef(t, "glue", string(w6bGlueCodeLoggingOff), w6bGluePhraseLoggingOff, domain.SevWarn, "wave1")
	w2AssertFindingDef(t, "glue", string(w6bGlueCodeArgumentSecret), w6bGluePhraseArgumentSecret, domain.SevBroken, "wave1")
}
