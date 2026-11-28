// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// s3_issue_enrichment.go — Wave 2 issue enrichment for the s3 resource type.
package aws

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	s3controltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"

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
//   - GetPublicAccessBlock     → s3.public-access-block-incomplete (with the
//     account's block, S3 Control GetPublicAccessBlock, when the bucket's
//     own leaves a flag off)
//   - GetBucketPolicyStatus    → s3.public
//   - GetBucketVersioning      → s3.versioning-off / s3.mfa-delete-off
//   - GetBucketLogging         → s3.access-logging-off
//   - GetBucketLifecycle...    → s3.no-lifecycle
//   - GetObjectLockConfig...   → s3.no-object-lock
//
// Contract (public access block Finding):
//   - It reads the effective block, the bucket's and the account's flag by
//     flag, and fires when that leaves a flag off.
//   - Severity is always "~" (a flag off is a risk, not an exposure, so this
//     signal never paints a row Broken).
//   - Rows carry the per-case detail (never duplicated in Summary).
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
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.S3 == nil {
		return result, nil
	}
	var failures []Failure
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	total := len(resources)
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		bucketName := r.Name
		if bucketName == "" {
			bucketName = r.ID
		}
		if bucketName == "" {
			return
		}
		p := scanS3BucketPosture(ctx, clients, clients.s3For(ctx, bucketName), bucketName)

		mu.Lock()
		defer mu.Unlock()
		if p.unreachable != nil {
			// Cross-region bucket or a bucket deleted between ListBuckets and
			// this call: data incomplete, but neither is a failure to log.
			markUninspected(&result, r.ID, checkOf(p.unreachable))
			return
		}
		// One entry per distinct cause, not per failed call: the six calls
		// usually refuse for one reason, and recording each would report six
		// failures where the operator has one permission to grant. Two
		// different refusals are two permissions, though, and the operator
		// who grants only the one named would watch the bucket fail again.
		seen := make(map[string]bool, len(p.failures))
		for _, callErr := range p.failures {
			cause := FailedCall(r.ID, callErr).Cause
			if seen[cause] {
				continue
			}
			seen[cause] = true
			MarkSkipped(&result, r.ID, &failures, callErr)
		}
		for _, f := range p.findings {
			setWave2Finding(&result, bucketName, f.code, f.rows)
		}
		if p.pabIncomplete {
			result.FieldUpdates[bucketName] = map[string]string{"status": "public access block incomplete"}
		}
	})

	return result, errors.Join(loopErr, AggregateFailures("bucket posture", failures, total))
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
	findings []s3PostureFinding
	failures []error
	// unreachable holds the error that made the bucket unreachable, so the row
	// can name the call that refused rather than only that something did.
	unreachable   error
	pabIncomplete bool
}

// s3PostureAPI is the set of read-only per-bucket calls scanS3BucketPosture
// makes. *s3.Client and the demo S3 fake both satisfy it.
type s3PostureAPI interface {
	s3PublicAPI
	S3GetBucketVersioningAPI
	S3GetBucketLoggingAPI
	S3GetBucketLifecycleAPI
	S3GetObjectLockConfigurationAPI
}

// s3PublicAPI is the set of reads s3PublicVerdict judges a bucket by.
type s3PublicAPI interface {
	S3GetPublicAccessBlockAPI
	S3GetBucketPolicyStatusAPI
	S3GetBucketAclAPI
}

// s3PublicRead is what s3PublicVerdict learned about one bucket.
type s3PublicRead struct {
	// rows names each way in from the internet; nil when the bucket is not
	// public.
	rows []domain.DetailRow
	// pabRead is true when the bucket's effective public access block is
	// known: the bucket's own and the account's were read, or one of them
	// alone sets all four flags. bucketPAB is nil for a bucket with none or
	// not read, accountPAB nil for an account with none or not read.
	pabRead    bool
	bucketPAB  *s3types.PublicAccessBlockConfiguration
	accountPAB *s3controltypes.PublicAccessBlockConfiguration
	// foreignRows and foreignPABRead are the same verdict for a bucket
	// another account owns, judged on the bucket's own block alone: the
	// account-level block that applies to a bucket is its owner's.
	foreignRows    []domain.DetailRow
	foreignPABRead bool
	failures       []error
	unreachable    error
}

func (v s3PublicRead) public(foreign bool) (decided, public bool) {
	if foreign {
		return v.foreignPABRead, v.foreignPABRead && v.foreignRows != nil
	}
	return v.pabRead, v.pabRead && v.rows != nil
}

// s3PublicVerdict is the one answer to "is this bucket public", for the
// bucket's own posture and for every resource that delivers into it. A bucket
// is public when AWS evaluates its policy as public and its effective
// RestrictPublicBuckets setting is off, or when its ACL grants a public group
// access and its effective IgnorePublicAcls setting is off. The effective
// block is the bucket's and the account's, flag by flag (s3AccountBlock); the
// account's is read only when the bucket's own leaves a flag off or could not
// be read. Each half is judged only against a block that was read: an unread
// block may be the one closing it. The first call decides reachability: a
// cross-region or already-deleted bucket short-circuits the rest.
func s3PublicVerdict(ctx context.Context, clients *ServiceClients, api s3PublicAPI, bucket string) s3PublicRead {
	var v s3PublicRead

	pabOut, err := api.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)})
	switch {
	case err == nil || isS3APIErrCode(err, "NoSuchPublicAccessBlockConfiguration"):
		v.pabRead = true
		if err == nil && pabOut != nil {
			v.bucketPAB = pabOut.PublicAccessBlockConfiguration
		}
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		v.unreachable = err
		return v
	default:
		v.failures = append(v.failures, err)
	}
	v.foreignPABRead = v.pabRead
	if bucketRead := v.pabRead; !bucketRead || !s3EffectiveBlock(v.bucketPAB, nil).complete() {
		account, accountErr := s3AccountBlock(ctx, clients)
		if accountErr != nil {
			v.failures = append(v.failures, accountErr)
		}
		v.accountPAB = account
		// An account block that sets all four flags closes the bucket
		// whatever its own block says.
		v.pabRead = accountErr == nil && (bucketRead || s3EffectiveBlock(nil, account).complete())
	}

	statusOut, err := api.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{Bucket: aws.String(bucket)})
	policyPublic := false
	switch {
	case err == nil:
		policyPublic = statusOut.PolicyStatus != nil && aws.ToBool(statusOut.PolicyStatus.IsPublic)
	// A bucket with no policy at all cannot be public by policy.
	case isS3APIErrCode(err, "NoSuchBucketPolicy"):
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		v.unreachable = err
		return v
	default:
		v.failures = append(v.failures, err)
	}

	aclOut, err := api.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: aws.String(bucket)})
	switch {
	case err == nil:
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		v.unreachable = err
		return v
	default:
		aclOut = nil
		v.failures = append(v.failures, err)
	}

	// One bucket reachable from the internet is one condition, however many
	// ways in it has. Each half is judged only against a block that was read:
	// an unread block may be the one closing it.
	judge := func(eff s3Block, read bool) []domain.DetailRow {
		if !read {
			return nil
		}
		var rows []domain.DetailRow
		if policyPublic && !eff.restrictPublicBuckets {
			rows = append(rows, domain.DetailRow{Label: "Policy status", Value: "public", Tier: "!"})
		}
		return append(rows, s3ACLPublicRows(aclOut, eff.ignorePublicAcls)...)
	}
	v.rows = judge(s3EffectiveBlock(v.bucketPAB, v.accountPAB), v.pabRead)
	v.foreignRows = judge(s3EffectiveBlock(v.bucketPAB, nil), v.foreignPABRead)
	return v
}

type s3Block struct {
	blockPublicAcls, ignorePublicAcls, blockPublicPolicy, restrictPublicBuckets bool
}

func (b s3Block) complete() bool {
	return b.blockPublicAcls && b.ignorePublicAcls && b.blockPublicPolicy && b.restrictPublicBuckets
}

func s3EffectiveBlock(bucket *s3types.PublicAccessBlockConfiguration, account *s3controltypes.PublicAccessBlockConfiguration) s3Block {
	var b s3Block
	if bucket != nil {
		b = s3Block{aws.ToBool(bucket.BlockPublicAcls), aws.ToBool(bucket.IgnorePublicAcls), aws.ToBool(bucket.BlockPublicPolicy), aws.ToBool(bucket.RestrictPublicBuckets)}
	}
	if account != nil {
		b.blockPublicAcls = b.blockPublicAcls || aws.ToBool(account.BlockPublicAcls)
		b.ignorePublicAcls = b.ignorePublicAcls || aws.ToBool(account.IgnorePublicAcls)
		b.blockPublicPolicy = b.blockPublicPolicy || aws.ToBool(account.BlockPublicPolicy)
		b.restrictPublicBuckets = b.restrictPublicBuckets || aws.ToBool(account.RestrictPublicBuckets)
	}
	return b
}

// scanS3BucketPosture runs the per-bucket posture calls in order and collects
// what fired. The first call decides reachability: a cross-region or
// already-deleted bucket short-circuits the rest, since every remaining call
// would fail the same way.
func scanS3BucketPosture(ctx context.Context, clients *ServiceClients, api s3PostureAPI, bucket string) s3BucketPosture {
	var p s3BucketPosture
	add := func(code domain.FindingCode, rows []domain.DetailRow) {
		p.findings = append(p.findings, s3PostureFinding{code: code, rows: rows})
	}

	pub := s3PublicVerdict(ctx, clients, api, bucket)
	p.failures = pub.failures
	if pub.unreachable != nil {
		p.unreachable = pub.unreachable
		return p
	}
	if pub.pabRead {
		if rows := s3PABRows(pub.bucketPAB, pub.accountPAB); rows != nil {
			p.pabIncomplete = true
			add(s3CodePublicAccessBlockIncomplete, rows)
		}
	}
	if pub.rows != nil {
		add(s3CodePublic, pub.rows)
	}

	versioningOut, err := api.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
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
		p.unreachable = err
		return p
	default:
		p.failures = append(p.failures, err)
	}

	loggingOut, err := api.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucket)})
	switch {
	case err == nil:
		if loggingOut.LoggingEnabled == nil {
			add(s3CodeAccessLoggingOff, nil)
		}
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		p.unreachable = err
		return p
	default:
		p.failures = append(p.failures, err)
	}

	lifecycleOut, err := api.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucket)})
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
		p.unreachable = err
		return p
	default:
		p.failures = append(p.failures, err)
	}

	lockOut, err := api.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{Bucket: aws.String(bucket)})
	switch {
	case err == nil, isS3APIErrCode(err, "ObjectLockConfigurationNotFoundError"):
		locked := lockOut != nil && lockOut.ObjectLockConfiguration != nil &&
			lockOut.ObjectLockConfiguration.ObjectLockEnabled == s3types.ObjectLockEnabledEnabled
		if !locked {
			add(s3CodeNoObjectLock, nil)
		}
	case IsNotFoundErr(err), isS3CrossRegionErr(err):
		p.unreachable = err
		return p
	default:
		p.failures = append(p.failures, err)
	}

	return p
}

// s3PABRows returns the AttentionDetail rows for a bucket whose effective
// public access block leaves a flag off, one row per flag off on both layers,
// or nil when the effective block is complete. A nil bucket is a bucket with
// no block of its own, a nil account an account with none.
func s3PABRows(bucket *s3types.PublicAccessBlockConfiguration, account *s3controltypes.PublicAccessBlockConfiguration) []domain.DetailRow {
	eff := s3EffectiveBlock(bucket, account)
	if eff.complete() {
		return nil
	}
	var rows []domain.DetailRow
	if bucket == nil {
		rows = append(rows, domain.DetailRow{Label: "Status", Value: "no public access block configuration"})
	}
	// The label is what the console calls the setting; the SDK flag name
	// rides in the value, which the style gate exempts because a value
	// legitimately is an identifier.
	for _, fc := range []struct {
		label, flag string
		on          bool
	}{
		{"Block public access control lists", "BlockPublicAcls", eff.blockPublicAcls},
		{"Ignore public access control lists", "IgnorePublicAcls", eff.ignorePublicAcls},
		{"Block public bucket policy", "BlockPublicPolicy", eff.blockPublicPolicy},
		{"Restrict public buckets", "RestrictPublicBuckets", eff.restrictPublicBuckets},
	} {
		if !fc.on {
			rows = append(rows, domain.DetailRow{Label: fc.label, Value: "off (" + fc.flag + ")"})
		}
	}
	accountValue := "off for these settings too"
	if account == nil {
		accountValue = "not configured"
	}
	return append(rows, domain.DetailRow{Label: "Account-level PAB", Value: accountValue})
}

// Canonical group URIs S3 grants an ACL permission to. Both reach the whole
// internet: AuthenticatedUsers is every AWS account, and anyone can open one.
const (
	s3ACLGroupAllUsers           = "http://acs.amazonaws.com/groups/global/AllUsers"
	s3ACLGroupAuthenticatedUsers = "http://acs.amazonaws.com/groups/global/AuthenticatedUsers"
)

// s3ACLPublicRows returns one row per ACL grant that opens the bucket to a
// public group, or nil when the ACL grants nothing publicly.
//
// ignorePublicACLs is the bucket's own IgnorePublicAcls setting: with it on,
// S3 disregards the public grants already on the bucket, so they reach nobody.
// BlockPublicAcls is a different setting — it refuses new grants and leaves
// the existing ones live — so it does not suppress these rows.
func s3ACLPublicRows(out *s3.GetBucketAclOutput, ignorePublicACLs bool) []domain.DetailRow {
	if out == nil || ignorePublicACLs {
		return nil
	}
	var rows []domain.DetailRow
	for _, g := range out.Grants {
		if g.Grantee == nil || g.Grantee.Type != s3types.TypeGroup {
			continue
		}
		group := ""
		switch aws.ToString(g.Grantee.URI) {
		case s3ACLGroupAllUsers:
			group = "AllUsers"
		case s3ACLGroupAuthenticatedUsers:
			group = "AuthenticatedUsers"
		default:
			continue
		}
		rows = append(rows, domain.DetailRow{
			Label: "Access control list",
			Value: group + " " + domain.HumanizeStatusPhrase(string(g.Permission)),
			Tier:  "!",
		})
	}
	return rows
}

// isS3APIErrCode reports whether err is the named S3 API error code. The
// "absent configuration" responses S3 returns as errors (NoSuchBucketPolicy,
// NoSuchLifecycleConfiguration, ObjectLockConfigurationNotFoundError) are
// answers, not failures, and each caller matches its own.
func isS3APIErrCode(err error, code string) bool {
	return ErrCodeIs(err, code)
}
