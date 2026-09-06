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

// trailPostureFindings returns the posture findings readable from the
// DescribeTrails payload alone, and attaches each one's supporting rows.
func trailPostureFindings(r *resource.Resource, trail cttypes.Trail) {
	if aws.ToString(trail.CloudWatchLogsLogGroupArn) == "" {
		addWave1Finding(r, CodeTrailNoCloudWatchLogs, "not delivering to CloudWatch Logs", domain.SevWarn)
		addWave1Rows(r, CodeTrailNoCloudWatchLogs, domain.DetailRow{
			Label: "Log group", Value: "none", Tier: "~",
		})
	}
	if aws.ToString(trail.KmsKeyId) == "" {
		addWave1Finding(r, CodeTrailNoKMS, "log files not KMS-encrypted", domain.SevWarn)
		addWave1Rows(r, CodeTrailNoKMS, domain.DetailRow{
			Label: "KMS key", Value: "none", Tier: "~",
		})
	}
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
