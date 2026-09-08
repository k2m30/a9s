// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// s3_issue_enrichment.go — Wave 2 issue enrichment for the s3 resource type.
package aws

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// s3 canonical FindingCodes.
const (
	s3CodePublicAccessBlockIncomplete domain.FindingCode = "s3.public-access-block-incomplete"
	s3CodePublic                      domain.FindingCode = "s3.public"
	s3CodeVersioningOff               domain.FindingCode = "s3.versioning-off"
	s3CodeMFADeleteOff                domain.FindingCode = "s3.mfa-delete-off"
	s3CodeAccessLoggingOff            domain.FindingCode = "s3.access-logging-off"
	s3CodeNoLifecycle                 domain.FindingCode = "s3.no-lifecycle"
	s3CodeNoObjectLock                domain.FindingCode = "s3.no-object-lock"
)

// EnrichS3Posture inspects each bucket's security posture (cap EnrichmentCap)
// with a small set of read-only per-bucket calls and emits one independently
// evaluated Finding per condition:
//
//   - GetPublicAccessBlock     → s3.public-access-block-incomplete
//   - GetBucketPolicyStatus    → s3.public
//   - GetBucketVersioning      → s3.versioning-off / s3.mfa-delete-off
//   - GetBucketLogging         → s3.access-logging-off
//   - GetBucketLifecycle...    → s3.no-lifecycle
//   - GetObjectLockConfig...   → s3.no-object-lock
//
// Contract (public access block Finding):
//   - Severity is always "~" (a missing/partial bucket-level PAB is a risk,
//     not a certainty — account-level PAB may still apply — so this signal
//     never paints a row Broken).
//   - Summary is always "public access block incomplete" — stable across all instances.
//   - Rows carry the per-case detail (never duplicated in Summary).
//
// On NoSuchPublicAccessBlockConfiguration:
//
//	Rows: {Label:"Status", Value:"no public access block configuration"},
//	      {Label:"Account-level PAB", Value:"may still apply"}
//
// On out.PublicAccessBlockConfiguration == nil (no API error):
//
//	Same Rows as above.
//
// On partial PAB (one or more flags false):
//
//	Rows: one entry per unset flag: {Label:"<FlagName>", Value:"off (<FlagName>)"},
//	      plus {Label:"Account-level PAB", Value:"may still apply"}
//
// On PermanentRedirect (301) / IllegalLocationConstraintException (400):
//
//	The bucket lives in a different region than the configured S3 client.
//	ListBuckets returns ALL buckets globally regardless of region, but
//	per-bucket calls require the bucket's regional endpoint. Mark
//	TruncatedIDs[id]=true (data incomplete → row "?" marker), skip every
//	remaining per-bucket call for that bucket, and do NOT add to the
//	failure-aggregate error: cross-region buckets are operational, not
//	bugs, and surfacing them in the `!` log produces noise on multi-region
//	accounts.
//
// On NoSuchBucket / bare "NotFound" (IsNotFoundErr):
//
//	The bucket was deleted between ListBuckets and this per-bucket call —
//	an operational race, not a bug, for the same reason as the
//	PermanentRedirect case above. Mark TruncatedIDs[id]=true (data
//	incomplete → row "?" marker) but do NOT add to the failure-aggregate
//	error, and do NOT emit a finding: a deleted bucket has no posture
//	state to report.
//
// On any other API error: no finding emitted for that call; TruncatedIDs[id]
// = true and the failure aggregates into the returned composite error.
func EnrichS3Posture(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.S3 == nil {
		return result, nil
	}
	truncated := false
	var failures []Failure
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	total := len(resources)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, total, EnrichmentParallelism, func(i int) {
		r := resources[i]
		bucketName := r.Name
		if bucketName == "" {
			bucketName = r.ID
		}
		if bucketName == "" {
			return
		}
		p := scanS3BucketPosture(ctx, clients.S3, bucketName)

		mu.Lock()
		defer mu.Unlock()
		if p.unreachable {
			truncated = true
			// Cross-region bucket or a bucket deleted between ListBuckets and
			// this call: data incomplete, but neither is a failure to log.
			result.TruncatedIDs[r.ID] = true
			return
		}
		if len(p.failures) > 0 {
			// One entry per failed BUCKET, not per failed call: the six calls
			// share a cause (usually one denied permission), and counting
			// each would report more failures than there were buckets.
			truncated = true
			MarkSkipped(&result, r.ID, &failures, p.failures[0])
		}
		for _, f := range p.findings {
			setWave2Finding(&result, bucketName, f.code, f.rows)
		}
		if p.pabIncomplete {
			result.FieldUpdates[bucketName] = map[string]string{"status": "public access block incomplete"}
		}
	})

	SetTruncated(&result, truncated)
	return result, AggregateFailures("bucket posture", failures, total)
}

// s3PostureFinding is one emitted condition, held until the shared result
// maps can be written under the mutex.
type s3PostureFinding struct {
	code domain.FindingCode
	rows []domain.DetailRow
}

// s3BucketPosture is the outcome of scanning one bucket: the conditions that
// fired, the per-call errors worth logging, and whether the bucket was
// reachable from this client's region at all.
type s3BucketPosture struct {
	findings      []s3PostureFinding
	failures      []error
	unreachable   bool
	pabIncomplete bool
}

// s3PostureAPI is the set of read-only per-bucket calls scanS3BucketPosture
// makes. *s3.Client and the demo S3 fake both satisfy it.
type s3PostureAPI interface {
	S3GetPublicAccessBlockAPI
	S3GetBucketPolicyStatusAPI
	S3GetBucketVersioningAPI
	S3GetBucketLoggingAPI
	S3GetBucketLifecycleAPI
	S3GetObjectLockConfigurationAPI
}

// scanS3BucketPosture runs the per-bucket posture calls in order and collects
// what fired. The first call decides reachability: a cross-region or
// already-deleted bucket short-circuits the rest, since every remaining call
// would fail the same way.
func scanS3BucketPosture(ctx context.Context, api s3PostureAPI, bucket string) s3BucketPosture {
	var p s3BucketPosture
	add := func(code domain.FindingCode, rows []domain.DetailRow) {
		p.findings = append(p.findings, s3PostureFinding{code: code, rows: rows})
	}

	pabOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetPublicAccessBlockOutput, error) {
		return api.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)})
	})
	switch {
	case err == nil || isS3APIErrCode(err, "NoSuchPublicAccessBlockConfiguration"):
		if rows := s3PABRows(pabOut, err); rows != nil {
			p.pabIncomplete = true
			add(s3CodePublicAccessBlockIncomplete, rows)
		}
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		p.unreachable = true
		return p
	default:
		p.failures = append(p.failures, err)
	}

	statusOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketPolicyStatusOutput, error) {
		return api.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{Bucket: aws.String(bucket)})
	})
	switch {
	case err == nil:
		if statusOut.PolicyStatus != nil && aws.ToBool(statusOut.PolicyStatus.IsPublic) {
			add(s3CodePublic,
				[]domain.DetailRow{{Label: "Policy status", Value: "public", Tier: "!"}})
		}
	// A bucket with no policy at all cannot be public by policy.
	case isS3APIErrCode(err, "NoSuchBucketPolicy"):
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		p.unreachable = true
		return p
	default:
		p.failures = append(p.failures, err)
	}

	versioningOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketVersioningOutput, error) {
		return api.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	})
	switch {
	case err == nil:
		switch {
		case versioningOut.Status != s3types.BucketVersioningStatusEnabled:
			state := "never enabled"
			if versioningOut.Status != "" {
				state = strings.ToLower(string(versioningOut.Status))
			}
			add(s3CodeVersioningOff,
				[]domain.DetailRow{{Label: "Versioning", Value: state, Tier: "~"}})
		case versioningOut.MFADelete != s3types.MFADeleteStatusEnabled:
			add(s3CodeMFADeleteOff, nil)
		}
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		p.unreachable = true
		return p
	default:
		p.failures = append(p.failures, err)
	}

	loggingOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketLoggingOutput, error) {
		return api.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucket)})
	})
	switch {
	case err == nil:
		if loggingOut.LoggingEnabled == nil {
			add(s3CodeAccessLoggingOff, nil)
		}
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		p.unreachable = true
		return p
	default:
		p.failures = append(p.failures, err)
	}

	lifecycleOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketLifecycleConfigurationOutput, error) {
		return api.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucket)})
	})
	switch {
	case err == nil, isS3APIErrCode(err, "NoSuchLifecycleConfiguration"):
		enabled := 0
		if lifecycleOut != nil {
			for _, rule := range lifecycleOut.Rules {
				if rule.Status == s3types.ExpirationStatusEnabled {
					enabled++
				}
			}
		}
		if enabled == 0 {
			add(s3CodeNoLifecycle,
				[]domain.DetailRow{{Label: "Lifecycle rules", Value: strconv.Itoa(enabled), Tier: "~"}})
		}
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		p.unreachable = true
		return p
	default:
		p.failures = append(p.failures, err)
	}

	lockOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetObjectLockConfigurationOutput, error) {
		return api.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{Bucket: aws.String(bucket)})
	})
	switch {
	case err == nil, isS3APIErrCode(err, "ObjectLockConfigurationNotFoundError"):
		locked := lockOut != nil && lockOut.ObjectLockConfiguration != nil &&
			lockOut.ObjectLockConfiguration.ObjectLockEnabled == s3types.ObjectLockEnabledEnabled
		if !locked {
			add(s3CodeNoObjectLock, nil)
		}
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		p.unreachable = true
		return p
	default:
		p.failures = append(p.failures, err)
	}

	return p
}

// s3PABRows returns the AttentionDetail rows for an incomplete bucket-level
// public access block, or nil when the block is complete. err is non-nil only
// for the NoSuchPublicAccessBlockConfiguration case, which reads the same as
// a nil configuration body.
func s3PABRows(out *s3.GetPublicAccessBlockOutput, err error) []domain.DetailRow {
	if err != nil || out == nil || out.PublicAccessBlockConfiguration == nil {
		return []domain.DetailRow{
			{Label: "Status", Value: "no public access block configuration"},
			{Label: "Account-level PAB", Value: "may still apply"},
		}
	}
	cfg := out.PublicAccessBlockConfiguration
	// The label is what the console calls the setting; the SDK flag name
	// rides in the value, which the style gate exempts because a value
	// legitimately is an identifier.
	flags := []struct {
		label string
		flag  string
		value *bool
	}{
		{"Block public access control lists", "BlockPublicAcls", cfg.BlockPublicAcls},
		{"Ignore public access control lists", "IgnorePublicAcls", cfg.IgnorePublicAcls},
		{"Block public bucket policy", "BlockPublicPolicy", cfg.BlockPublicPolicy},
		{"Restrict public buckets", "RestrictPublicBuckets", cfg.RestrictPublicBuckets},
	}
	var rows []domain.DetailRow
	for _, fc := range flags {
		if !aws.ToBool(fc.value) {
			rows = append(rows, domain.DetailRow{Label: fc.label, Value: "off (" + fc.flag + ")"})
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return append(rows, domain.DetailRow{Label: "Account-level PAB", Value: "may still apply"})
}

// isS3APIErrCode reports whether err is the named S3 API error code. The
// "absent configuration" responses S3 returns as errors (NoSuchBucketPolicy,
// NoSuchLifecycleConfiguration, ObjectLockConfigurationNotFoundError) are
// answers, not failures, and each caller matches its own.
func isS3APIErrCode(err error, code string) bool {
	return ErrCodeIs(err, code)
}
