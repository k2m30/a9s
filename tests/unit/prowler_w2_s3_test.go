package unit

// prowler_w2_s3_test.go — s3 posture rows 1–6 of the w2 Prowler batch:
// public bucket policy, versioning off, MFA-delete off, access logging off,
// no lifecycle rules, no object lock.
//
// The enricher under test is whatever function the s3 catalog literal wires
// into Wave2 — the rename of EnrichS3Posture to EnrichS3Posture is
// permitted by the batch spec, so the tests reach it through the registry.
// Every bucket in these tests is otherwise healthy for the five conditions it
// is not exercising, so a finding that appears is unambiguously the one the
// case set up.

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w2S3CodePublic           = "s3.public"
	w2S3CodeVersioningOff    = "s3.versioning-off"
	w2S3CodeMFADeleteOff     = "s3.mfa-delete-off"
	w2S3CodeAccessLoggingOff = "s3.access-logging-off"
	w2S3CodeNoLifecycle      = "s3.no-lifecycle"
	w2S3CodeNoObjectLock     = "s3.no-object-lock"

	w2S3Source = "wave2:s3"
)

// ---------------------------------------------------------------------------
// mock
// ---------------------------------------------------------------------------

// w2S3Posture is a per-bucket posture fake. Absent map entries mean "healthy":
// PAB fully on, policy not public, versioning + MFA delete enabled, logging
// configured, one enabled lifecycle rule, object lock enabled. That default
// keeps every case's other five conditions silent.
//
// The embedded S3API makes the struct satisfy the aggregate; any method the
// enricher calls that this fake does not define is a nil-interface panic,
// which is the correct loud failure for an unexpected API call.
type w2S3Posture struct {
	awsclient.S3API

	policyPublic map[string]bool
	noPolicy     map[string]bool // GetBucketPolicyStatus → NoSuchBucketPolicy
	versioning   map[string]*s3.GetBucketVersioningOutput
	noLogging    map[string]bool
	noLifecycle  map[string]bool
	emptyRules   map[string]bool // lifecycle present but zero enabled rules
	noObjectLock map[string]bool
	errs         map[string]error // any call for this bucket fails with this error
}

func (f *w2S3Posture) fail(bucket string) error { return f.errs[bucket] }

func (f *w2S3Posture) GetPublicAccessBlock(_ context.Context, in *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	b := aws.ToString(in.Bucket)
	if err := f.fail(b); err != nil {
		return nil, err
	}
	return &s3.GetPublicAccessBlockOutput{
		PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(true),
			IgnorePublicAcls:      aws.Bool(true),
			BlockPublicPolicy:     aws.Bool(true),
			RestrictPublicBuckets: aws.Bool(true),
		},
	}, nil
}

func (f *w2S3Posture) GetBucketPolicyStatus(_ context.Context, in *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	b := aws.ToString(in.Bucket)
	if err := f.fail(b); err != nil {
		return nil, err
	}
	if f.noPolicy[b] {
		return nil, &smithy.GenericAPIError{Code: "NoSuchBucketPolicy", Message: "The bucket policy does not exist"}
	}
	return &s3.GetBucketPolicyStatusOutput{
		PolicyStatus: &s3types.PolicyStatus{IsPublic: aws.Bool(f.policyPublic[b])},
	}, nil
}

func (f *w2S3Posture) GetBucketVersioning(_ context.Context, in *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	b := aws.ToString(in.Bucket)
	if err := f.fail(b); err != nil {
		return nil, err
	}
	if out, ok := f.versioning[b]; ok {
		return out, nil
	}
	return &s3.GetBucketVersioningOutput{
		Status:    s3types.BucketVersioningStatusEnabled,
		MFADelete: s3types.MFADeleteStatusEnabled,
	}, nil
}

func (f *w2S3Posture) GetBucketLogging(_ context.Context, in *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	b := aws.ToString(in.Bucket)
	if err := f.fail(b); err != nil {
		return nil, err
	}
	if f.noLogging[b] {
		return &s3.GetBucketLoggingOutput{}, nil
	}
	return &s3.GetBucketLoggingOutput{
		LoggingEnabled: &s3types.LoggingEnabled{
			TargetBucket: aws.String("acme-access-logs"),
			TargetPrefix: aws.String(b + "/"),
		},
	}, nil
}

func (f *w2S3Posture) GetBucketLifecycleConfiguration(_ context.Context, in *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	b := aws.ToString(in.Bucket)
	if err := f.fail(b); err != nil {
		return nil, err
	}
	if f.noLifecycle[b] {
		return nil, &smithy.GenericAPIError{Code: "NoSuchLifecycleConfiguration", Message: "The lifecycle configuration does not exist"}
	}
	if f.emptyRules[b] {
		return &s3.GetBucketLifecycleConfigurationOutput{
			Rules: []s3types.LifecycleRule{
				{ID: aws.String("disabled-rule"), Status: s3types.ExpirationStatusDisabled},
			},
		}, nil
	}
	return &s3.GetBucketLifecycleConfigurationOutput{
		Rules: []s3types.LifecycleRule{
			{ID: aws.String("expire-old"), Status: s3types.ExpirationStatusEnabled},
		},
	}, nil
}

func (f *w2S3Posture) GetObjectLockConfiguration(_ context.Context, in *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	b := aws.ToString(in.Bucket)
	if err := f.fail(b); err != nil {
		return nil, err
	}
	if f.noObjectLock[b] {
		return nil, &smithy.GenericAPIError{Code: "ObjectLockConfigurationNotFoundError", Message: "Object Lock configuration does not exist for this bucket"}
	}
	return &s3.GetObjectLockConfigurationOutput{
		ObjectLockConfiguration: &s3types.ObjectLockConfiguration{
			ObjectLockEnabled: s3types.ObjectLockEnabledEnabled,
		},
	}, nil
}

func w2S3RunErr(t *testing.T, fake *w2S3Posture, buckets ...string) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	rs := make([]resource.Resource, 0, len(buckets))
	for _, b := range buckets {
		rs = append(rs, w2Res(b, nil))
	}
	res, err := w2Enricher(t, "s3")(context.Background(), &awsclient.ServiceClients{S3: fake}, rs, nil)
	w2AssertEnricherShape(t, res)
	return res, err
}

func w2S3Run(t *testing.T, fake *w2S3Posture, buckets ...string) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w2S3RunErr(t, fake, buckets...)
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}
	return res
}

// ---------------------------------------------------------------------------
// row 1 — s3.public
// ---------------------------------------------------------------------------

func TestW2S3PublicPolicyStatus(t *testing.T) {
	res := w2S3Run(t, &w2S3Posture{policyPublic: map[string]bool{"acme-public": true}}, "acme-public", "acme-private")

	w2AssertFinding(t, res.Findings["acme-public"], w2S3CodePublic, "publicly accessible", domain.SevBroken, w2S3Source)
	w2AssertRow(t, w2Rows(t, res, "acme-public", w2S3CodePublic), "Policy status", "public")
	w2AssertFindingDef(t, "s3", w2S3CodePublic, "publicly accessible", domain.SevBroken, "wave2")

	// The healthy sibling in the same batch must stay clean — a public bucket
	// never contaminates the row next to it.
	w2AssertNoCode(t, res.Findings["acme-private"], w2S3CodePublic)
}

// A bucket with no policy at all is not public. NoSuchBucketPolicy is the
// normal answer for most buckets and must not be treated as a call failure.
func TestW2S3NoBucketPolicyIsNotPublicAndNotTruncated(t *testing.T) {
	res := w2S3Run(t, &w2S3Posture{noPolicy: map[string]bool{"acme-nopolicy": true}}, "acme-nopolicy")

	w2AssertNoCode(t, res.Findings["acme-nopolicy"], w2S3CodePublic)
	if res.TruncatedIDs["acme-nopolicy"] {
		t.Error("NoSuchBucketPolicy marked the bucket unknown; it is a definite not-public answer")
	}
}

// IsPublic == false is an explicit "not public" and must emit nothing.
func TestW2S3PolicyStatusNotPublicEmitsNothing(t *testing.T) {
	res := w2S3Run(t, &w2S3Posture{}, "acme-private")
	w2AssertNoCode(t, res.Findings["acme-private"], w2S3CodePublic)
}

// ---------------------------------------------------------------------------
// rows 2 & 3 — versioning / MFA delete
// ---------------------------------------------------------------------------

func TestW2S3VersioningOff(t *testing.T) {
	fake := &w2S3Posture{versioning: map[string]*s3.GetBucketVersioningOutput{
		// Never-enabled bucket: both fields absent.
		"acme-noversion": {},
		"acme-suspended": {Status: s3types.BucketVersioningStatusSuspended},
	}}
	res := w2S3Run(t, fake, "acme-noversion", "acme-suspended", "acme-versioned")

	w2AssertFinding(t, res.Findings["acme-noversion"], w2S3CodeVersioningOff, "versioning off", domain.SevWarn, w2S3Source)
	w2AssertRow(t, w2Rows(t, res, "acme-noversion", w2S3CodeVersioningOff), "Versioning", "never enabled")

	w2AssertFinding(t, res.Findings["acme-suspended"], w2S3CodeVersioningOff, "versioning off", domain.SevWarn, w2S3Source)
	w2AssertRow(t, w2Rows(t, res, "acme-suspended", w2S3CodeVersioningOff), "Versioning", "Suspended")

	w2AssertNoCode(t, res.Findings["acme-versioned"], w2S3CodeVersioningOff)
	w2AssertFindingDef(t, "s3", w2S3CodeVersioningOff, "versioning off", domain.SevWarn, "wave2")
}

func TestW2S3MFADeleteOff(t *testing.T) {
	fake := &w2S3Posture{versioning: map[string]*s3.GetBucketVersioningOutput{
		"acme-mfaoff": {Status: s3types.BucketVersioningStatusEnabled},
	}}
	res := w2S3Run(t, fake, "acme-mfaoff", "acme-versioned")

	w2AssertFinding(t, res.Findings["acme-mfaoff"], w2S3CodeMFADeleteOff, "MFA delete off", domain.SevWarn, w2S3Source)
	w2AssertRow(t, w2Rows(t, res, "acme-mfaoff", w2S3CodeMFADeleteOff), "MFA delete", "disabled")
	w2AssertNoCode(t, res.Findings["acme-versioned"], w2S3CodeMFADeleteOff)
	w2AssertFindingDef(t, "s3", w2S3CodeMFADeleteOff, "MFA delete off", domain.SevWarn, "wave2")
}

// MFA delete is only configurable once versioning is on. A bucket with
// versioning off must report exactly one problem, not two, or the operator
// gets sent to fix a setting that cannot be changed yet.
func TestW2S3VersioningOffSuppressesMFADeleteRow(t *testing.T) {
	fake := &w2S3Posture{versioning: map[string]*s3.GetBucketVersioningOutput{
		"acme-noversion": {},
	}}
	res := w2S3Run(t, fake, "acme-noversion")

	w2AssertFinding(t, res.Findings["acme-noversion"], w2S3CodeVersioningOff, "versioning off", domain.SevWarn, w2S3Source)
	w2AssertNoCode(t, res.Findings["acme-noversion"], w2S3CodeMFADeleteOff)
}

// ---------------------------------------------------------------------------
// rows 4, 5, 6 — logging / lifecycle / object lock
// ---------------------------------------------------------------------------

func TestW2S3AccessLoggingOff(t *testing.T) {
	res := w2S3Run(t, &w2S3Posture{noLogging: map[string]bool{"acme-nolog": true}}, "acme-nolog", "acme-logged")

	w2AssertFinding(t, res.Findings["acme-nolog"], w2S3CodeAccessLoggingOff, "access logging off", domain.SevWarn, w2S3Source)
	w2AssertRow(t, w2Rows(t, res, "acme-nolog", w2S3CodeAccessLoggingOff), "Access logging", "off")
	w2AssertNoCode(t, res.Findings["acme-logged"], w2S3CodeAccessLoggingOff)
	w2AssertFindingDef(t, "s3", w2S3CodeAccessLoggingOff, "access logging off", domain.SevWarn, "wave2")
}

func TestW2S3NoLifecycle(t *testing.T) {
	fake := &w2S3Posture{
		noLifecycle: map[string]bool{"acme-nolc": true},
		emptyRules:  map[string]bool{"acme-disabledlc": true},
	}
	res := w2S3Run(t, fake, "acme-nolc", "acme-disabledlc", "acme-withlc")

	w2AssertFinding(t, res.Findings["acme-nolc"], w2S3CodeNoLifecycle, "no lifecycle rules", domain.SevWarn, w2S3Source)
	w2AssertRow(t, w2Rows(t, res, "acme-nolc", w2S3CodeNoLifecycle), "Lifecycle rules", "0")

	// A configuration whose only rule is Disabled expires nothing — it counts
	// as zero enabled rules, not as a configured lifecycle.
	w2AssertFinding(t, res.Findings["acme-disabledlc"], w2S3CodeNoLifecycle, "no lifecycle rules", domain.SevWarn, w2S3Source)

	w2AssertNoCode(t, res.Findings["acme-withlc"], w2S3CodeNoLifecycle)
	w2AssertFindingDef(t, "s3", w2S3CodeNoLifecycle, "no lifecycle rules", domain.SevWarn, "wave2")
}

func TestW2S3NoObjectLock(t *testing.T) {
	res := w2S3Run(t, &w2S3Posture{noObjectLock: map[string]bool{"acme-nolock": true}}, "acme-nolock", "acme-locked")

	w2AssertFinding(t, res.Findings["acme-nolock"], w2S3CodeNoObjectLock, "object lock off", domain.SevWarn, w2S3Source)
	w2AssertRow(t, w2Rows(t, res, "acme-nolock", w2S3CodeNoObjectLock), "Object lock", "off")
	w2AssertNoCode(t, res.Findings["acme-locked"], w2S3CodeNoObjectLock)
	w2AssertFindingDef(t, "s3", w2S3CodeNoObjectLock, "object lock off", domain.SevWarn, "wave2")
}

// ---------------------------------------------------------------------------
// cross-cutting Wave-2 discipline
// ---------------------------------------------------------------------------

// Two independent conditions on one bucket produce two findings; neither is
// folded into the other's rows, and each keeps its own AttentionDetail entry.
func TestW2S3TwoConditionsProduceTwoFindings(t *testing.T) {
	fake := &w2S3Posture{
		policyPublic: map[string]bool{"acme-bad": true},
		noLogging:    map[string]bool{"acme-bad": true},
	}
	res := w2S3Run(t, fake, "acme-bad")

	w2AssertFinding(t, res.Findings["acme-bad"], w2S3CodePublic, "publicly accessible", domain.SevBroken, w2S3Source)
	w2AssertFinding(t, res.Findings["acme-bad"], w2S3CodeAccessLoggingOff, "access logging off", domain.SevWarn, w2S3Source)
	w2AssertRow(t, w2Rows(t, res, "acme-bad", w2S3CodePublic), "Policy status", "public")
	w2AssertRow(t, w2Rows(t, res, "acme-bad", w2S3CodeAccessLoggingOff), "Access logging", "off")
}

// A cross-region bucket answers PermanentRedirect from the session's endpoint.
// The row must go unknown rather than silently reporting a healthy posture it
// never measured, and the neighbouring bucket must still be evaluated.
func TestW2S3CrossRegionBucketIsUnknownNotHealthy(t *testing.T) {
	fake := &w2S3Posture{
		errs:      map[string]error{"acme-elsewhere": &smithy.GenericAPIError{Code: "PermanentRedirect", Message: "wrong region"}},
		noLogging: map[string]bool{"acme-local": true},
	}
	res := w2S3Run(t, fake, "acme-elsewhere", "acme-local")

	if !res.TruncatedIDs["acme-elsewhere"] {
		t.Error("cross-region bucket not marked in TruncatedIDs")
	}
	if len(res.Findings["acme-elsewhere"]) != 0 {
		t.Errorf("cross-region bucket emitted findings %v; nothing was measured", w2Codes(res.Findings["acme-elsewhere"]))
	}
	w2AssertFinding(t, res.Findings["acme-local"], w2S3CodeAccessLoggingOff, "access logging off", domain.SevWarn, w2S3Source)
}

// An AccessDenied on one bucket must not fail the whole batch.
func TestW2S3PerBucketErrorDoesNotSinkTheBatch(t *testing.T) {
	fake := &w2S3Posture{
		errs:      map[string]error{"acme-denied": &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"}},
		noLogging: map[string]bool{"acme-ok": true},
	}
	res, err := w2S3RunErr(t, fake, "acme-denied", "acme-ok")
	if err == nil {
		t.Error("a denied bucket read was not folded into the composite error")
	}
	// The composite error counts one failure per denied BUCKET. Counting one
	// per denied CALL reports more failures than there were items, which is
	// what an operator reads as the scale of the outage.
	if err != nil && !strings.Contains(err.Error(), "1 of 2 IDs") {
		t.Errorf("composite error miscounts the batch: %v", err)
	}

	if !res.TruncatedIDs["acme-denied"] {
		t.Error("AccessDenied bucket not marked in TruncatedIDs")
	}
	w2AssertFinding(t, res.Findings["acme-ok"], w2S3CodeAccessLoggingOff, "access logging off", domain.SevWarn, w2S3Source)
}

func TestW2S3NilClientIsSafe(t *testing.T) {
	res, err := w2Enricher(t, "s3")(context.Background(), &awsclient.ServiceClients{}, []resource.Resource{w2Res("acme-any", nil)}, nil)
	w2AssertEnricherInvariants(t, res, err)
	if len(res.Findings) != 0 {
		t.Errorf("nil S3 client produced findings %v", res.Findings)
	}
}

// Past the cap the extra buckets are not evaluated, and the result says so
// rather than presenting an unmeasured bucket as clean.
func TestW2S3CapBoundary(t *testing.T) {
	names := make([]string, 0, awsclient.EnrichmentCap+1)
	noLog := map[string]bool{}
	for i := 0; i <= awsclient.EnrichmentCap; i++ {
		n := "acme-bucket-" + strconv.Itoa(i)
		names = append(names, n)
		noLog[n] = true
	}

	atCap := w2S3Run(t, &w2S3Posture{noLogging: noLog}, names[:awsclient.EnrichmentCap]...)
	if atCap.Truncated {
		t.Error("exactly EnrichmentCap buckets reported Truncated")
	}

	overCap := w2S3Run(t, &w2S3Posture{noLogging: noLog}, names...)
	if !overCap.Truncated {
		t.Error("EnrichmentCap+1 buckets did not report Truncated")
	}
}
