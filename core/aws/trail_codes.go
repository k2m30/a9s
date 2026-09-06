// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// trail_codes.go — canonical FindingCode constants for the trail resource
// type. The fetcher writes Findings using these codes; colorTrail
// (catalog_monitoring.go) reads the same underlying Fields to color rows —
// both derive from the identical DescribeTrails/GetTrailStatus conditions so
// the Findings list and the row color never disagree.
package aws

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// trailDeliveryStaleAfter is the delivery-staleness threshold (docs/resources/trail.md §3.2):
// a logging trail with no delivered file in over an hour is Broken.
const trailDeliveryStaleAfter = time.Hour

const (
	// CodeTrailLogFileValidationDisabled — LogFileValidationEnabled==false.
	// Log files are not signed; tamper-evidence is off. Severity: SevWarn.
	CodeTrailLogFileValidationDisabled domain.FindingCode = "trail.log-file-validation.disabled"

	// CodeTrailNotLogging — IsLogging==false (GetTrailStatus). The trail has
	// stopped capturing events. Severity: SevBroken.
	CodeTrailNotLogging domain.FindingCode = "trail.not-logging"

	// CodeTrailDeliveryError — LatestDeliveryError non-empty (GetTrailStatus).
	// S3 delivery of log files is failing. Severity: SevBroken.
	CodeTrailDeliveryError domain.FindingCode = "trail.delivery-error"

	// CodeTrailDeliveryStale — LatestDeliveryTime older than
	// trailDeliveryStaleAfter on a logging trail (GetTrailStatus). No file has
	// been delivered recently even though the trail believes it's logging.
	// Severity: SevBroken.
	CodeTrailDeliveryStale domain.FindingCode = "trail.delivery-stale"

	// CodeTrailSingleRegion — this trail is single-region and no trail in the
	// account is multi-region. Severity: SevWarn.
	CodeTrailSingleRegion domain.FindingCode = "trail.single-region"

	// CodeTrailNoCloudWatchLogs — CloudWatchLogsLogGroupArn empty. Events land
	// in S3 only, so no metric filter or alarm can watch them. Severity: SevWarn.
	CodeTrailNoCloudWatchLogs domain.FindingCode = "trail.no-cloudwatch-logs"

	// CodeTrailNoKMS — KmsKeyId empty. Delivered log files are encrypted with
	// S3-managed keys only. Severity: SevWarn.
	CodeTrailNoKMS domain.FindingCode = "trail.no-kms"

	// CodeTrailLogBucketPublic — the S3 bucket this trail delivers to carries
	// the s3.public finding. Severity: SevBroken.
	CodeTrailLogBucketPublic domain.FindingCode = "trail.log-bucket-public"

	// CodeTrailLogBucketNoAccessLogging — the S3 bucket this trail delivers to
	// carries the s3.access-logging-off finding. Severity: SevWarn.
	CodeTrailLogBucketNoAccessLogging domain.FindingCode = "trail.log-bucket-no-access-logging"
)

// S5 detail sentences: what is wrong, what it exposes, what fixing it takes.
const (
	trailSingleRegionDetail = "The account has no multi-region trail, so API activity in every other region goes unrecorded. Recreate this trail with multi-region delivery enabled, or add one trail that covers all regions."

	trailNoCloudWatchLogsDetail = "Events are delivered to S3 only, so no metric filter or alarm can watch them and nobody is paged on suspicious API activity. Attach a CloudWatch Logs log group to this trail."

	trailNoKMSDetail = "Delivered log files use S3-managed encryption, so anyone who can read the bucket can read the audit trail. Set a KMS key on the trail so log files are encrypted with a key you control."

	trailLogBucketPublicDetail = "The bucket holding this trail's log files is publicly accessible, so the account's audit history can be read by anyone. Remove the public grant from that bucket's policy and access control list."

	trailLogBucketNoAccessLoggingDetail = "The bucket holding this trail's log files records no access logging, so reads of the audit history leave no trace. Enable server access logging on that bucket."
)

// trailPostureFindings returns the posture findings readable from the
// DescribeTrails payload alone, and attaches each one's supporting rows.
//
// accountHasMultiRegion is the account-wide half of the single-region rule:
// Prowler's check is satisfied by one multi-region trail anywhere, so a
// single-region trail beside a multi-region one is not a gap. DescribeTrails
// omits IsMultiRegionTrail only for trails that are not multi-region, so
// absent is off here rather than unknown.
func trailPostureFindings(r *resource.Resource, trail cttypes.Trail, accountHasMultiRegion bool) {
	if !accountHasMultiRegion {
		r.Findings = append(r.Findings, domain.Finding{
			Code: CodeTrailSingleRegion, Phrase: "single-region trail",
			Severity: domain.SevWarn, Source: "wave1", Detail: trailSingleRegionDetail,
		})
		addWave1Rows(r, CodeTrailSingleRegion, domain.DetailRow{
			Label: "Multi-region", Value: "no", Tier: "~",
		})
	}
	if aws.ToString(trail.CloudWatchLogsLogGroupArn) == "" {
		r.Findings = append(r.Findings, domain.Finding{
			Code: CodeTrailNoCloudWatchLogs, Phrase: "not delivering to CloudWatch Logs",
			Severity: domain.SevWarn, Source: "wave1", Detail: trailNoCloudWatchLogsDetail,
		})
		addWave1Rows(r, CodeTrailNoCloudWatchLogs, domain.DetailRow{
			Label: "Log group", Value: "none", Tier: "~",
		})
	}
	if aws.ToString(trail.KmsKeyId) == "" {
		r.Findings = append(r.Findings, domain.Finding{
			Code: CodeTrailNoKMS, Phrase: "log files not KMS-encrypted",
			Severity: domain.SevWarn, Source: "wave1", Detail: trailNoKMSDetail,
		})
		addWave1Rows(r, CodeTrailNoKMS, domain.DetailRow{
			Label: "KMS key", Value: "none", Tier: "~",
		})
	}
}

// trailListHasMultiRegion reports whether any trail in the account's list is
// multi-region.
func trailListHasMultiRegion(trails []cttypes.Trail) bool {
	for _, t := range trails {
		if aws.ToBool(t.IsMultiRegionTrail) {
			return true
		}
	}
	return false
}

// trailDeliveryIsStale reports whether latestDeliveryTime (RFC3339) is older
// than trailDeliveryStaleAfter, given the trail is currently logging. Both
// trailWave1Wave2Findings and colorTrail call this so the Findings list and
// the row color never disagree about the stale-delivery condition.
func trailDeliveryIsStale(isLogging, latestDeliveryTime string) bool {
	if isLogging != "true" || latestDeliveryTime == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, latestDeliveryTime)
	if err != nil {
		return false
	}
	return time.Since(t) > trailDeliveryStaleAfter
}

// trailWave1Wave2Findings returns the Finding slice for a CloudTrail trail
// given the same fields colorTrail (catalog_monitoring.go) reads: is_logging,
// latest_delivery_error, latest_delivery_time, and
// log_file_validation_enabled. Mirrors colorTrail's precedence (broken
// conditions checked before the warning condition) so the two never disagree
// about a trail's health.
func trailWave1Wave2Findings(isLogging, latestDeliveryError, latestDeliveryTime, logFileValidationEnabled string) []domain.Finding {
	var findings []domain.Finding
	if isLogging == "false" {
		findings = append(findings, domain.Finding{
			Code: CodeTrailNotLogging, Phrase: "not logging",
			Severity: domain.SevBroken, Source: "wave2",
		})
	}
	if latestDeliveryError != "" && latestDeliveryError != "-" {
		findings = append(findings, domain.Finding{
			Code: CodeTrailDeliveryError, Phrase: "delivery error: " + latestDeliveryError,
			Severity: domain.SevBroken, Source: "wave2",
		})
	}
	if trailDeliveryIsStale(isLogging, latestDeliveryTime) {
		findings = append(findings, domain.Finding{
			Code: CodeTrailDeliveryStale, Phrase: "delivery stale since " + latestDeliveryTime,
			Severity: domain.SevBroken, Source: "wave2",
		})
	}
	if logFileValidationEnabled == "false" {
		findings = append(findings, domain.Finding{
			Code: CodeTrailLogFileValidationDisabled, Phrase: "log file validation disabled",
			Severity: domain.SevWarn, Source: "wave1",
		})
	}
	return findings
}
