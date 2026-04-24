package fixtures

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
)

// RedshiftFixtures holds typed fixture data for Redshift.
type RedshiftFixtures struct {
	Clusters []redshifttypes.Cluster
}

// Stable IDs for Redshift fixtures — imported by sibling fixture files and QA tests.
const (
	// Healthy / graph-roots
	AcmeWarehouseID = "acme-warehouse"
	AcmeReportingID = "acme-reporting"
	StagingDwhID    = "staging-dwh"

	// Wave-1 transitional (ClusterStatus)
	RedshiftResizingID  = "redshift-resizing"
	RedshiftRebootingID = "redshift-rebooting"

	// Wave-1 broken (ClusterStatus)
	RedshiftIncompatibleNetworkID = "redshift-incompatible-network"
	RedshiftHardwareFailureID     = "redshift-hardware-failure"
	RedshiftStorageFullID         = "redshift-storage-full"

	// ClusterAvailabilityStatus-driven (Broken)
	RedshiftAvailUnavailableID = "redshift-avail-unavailable"
	RedshiftAvailFailedID      = "redshift-avail-failed"

	// ClusterAvailabilityStatus-driven (Warning)
	RedshiftAvailMaintenanceID = "redshift-avail-maintenance"
	RedshiftAvailModifyingID   = "redshift-avail-modifying"

	// PendingModifiedValues / DeferredMaintenanceWindows / PubliclyAccessible / Unencrypted
	RedshiftPendingChangeID              = "redshift-pending-change"
	RedshiftMaintenanceDeferredID        = "redshift-maintenance-deferred"
	// Use a non-prefix-sharing ID so `findRow("redshift-maintenance-deferred")` in
	// scenario tests picks the active fixture, not this expired variant.
	RedshiftMaintenanceDeferredExpiredID = "redshift-expired-window"
	RedshiftPubliclyAccessibleID         = "redshift-publicly-accessible"
	RedshiftUnencryptedID                = "redshift-unencrypted"

	// Multi-finding (rule 7)
	WarnRedshiftMultiID = "warn-redshift-multi"
	WarnRedshiftTwoID   = "warn-redshift-two"

	// Severity-precedence vehicles (U8)
	// Severity-precedence fixture IDs — kept under 36 chars so the list-view Cluster ID
	// column renders the full identifier without ellipsis truncation.
	RedshiftBrokenWithWarningHiddenID           = "redshift-broken-w-hidden-warn"
	RedshiftAvailUnavailableWithWarningHiddenID = "redshift-unavail-w-hidden-warn"

	// Security group IDs used by the graph-root fixtures — referenced by ec2.go sibling additions.
	RedshiftWarehouseSGID1  = "sg-warehouse-1"
	RedshiftWarehouseSGID2  = "sg-warehouse-2"
	RedshiftReportingSGID1  = "sg-reporting-1"
	RedshiftReportingSGID2  = "sg-reporting-2"

	// KMS key IDs (bare IDs after stripping ARN prefix).
	RedshiftKMSKeyID1 = "kms-redshift-1"
	RedshiftKMSKeyID2 = "kms-redshift-2"

	// KMS key ARNs (full ARNs stored on Cluster.KmsKeyId).
	RedshiftKMSKeyARN1 = "arn:aws:kms:us-east-1:123456789012:key/kms-redshift-1"
	RedshiftKMSKeyARN2 = "arn:aws:kms:us-east-1:123456789012:key/kms-redshift-2"

	// MasterPasswordSecretArn values for the two graph-root clusters.
	AcmeWarehouseSecretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:redshift!acme-warehouse-AbCdEf"
	AcmeReportingSecretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:redshift!acme-reporting-XxYyZz"

	// Subnet group names (used by DescribeClusterSubnetGroups in the fake).
	RedshiftProdSubnetGroup    = "redshift-prod-subnet-group"
	RedshiftStagingSubnetGroup = "redshift-staging-subnet-group"

	// IAM role names for graph-root clusters (IamRoles slice).
	RedshiftCopyRoleARN          = "arn:aws:iam::123456789012:role/redshift-copy-role"
	RedshiftUnloadRoleARN        = "arn:aws:iam::123456789012:role/redshift-unload-role"
	RedshiftReportingCopyRoleARN = "arn:aws:iam::123456789012:role/redshift-reporting-copy-role"

	// S3 audit bucket for acme-reporting (S3-logging graph-root).
	RedshiftAuditBucket = "acme-redshift-audit"

	// internal helpers — not exported, used within this file only.
	redshiftProdVPCID     = fixtProdVPCID     // "vpc-0abc123def456789a"
	redshiftStagingVPCID  = fixtStagingVPCID   // "vpc-0def456789abc123d"
)

// NewRedshiftFixtures constructs RedshiftFixtures from the canonical demo data.
// Every fixture in docs/resources/redshift-impl-plan.md §2 is present.
// Adversarial fixtures (nil-pointer Cluster, malformed Tags) are excluded —
// those live inline in QA test files per the a9s-create-demo-fixture skill rule.
func NewRedshiftFixtures() *RedshiftFixtures {
	return &RedshiftFixtures{
		Clusters: buildRedshiftClusters(),
	}
}

// redshiftBaselineHealthy returns a fully-configured Redshift cluster with all
// spec §4 fields set to healthy values. Callers mutate specific fields to
// produce issue-state variants.
func redshiftBaselineHealthy(id string) redshifttypes.Cluster {
	return redshifttypes.Cluster{
		ClusterIdentifier:         aws.String(id),
		ClusterStatus:             aws.String("available"),
		ClusterAvailabilityStatus: aws.String("Available"),
		NodeType:                  aws.String("ra3.xlplus"),
		NumberOfNodes:             aws.Int32(2),
		DBName:                    aws.String("db"),
		MasterUsername:            aws.String("admin"),
		ClusterCreateTime:         aws.Time(mustParseRedshiftTime("2025-06-01T10:00:00Z")),
		ClusterNamespaceArn:       aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:" + id),
		AvailabilityZone:          aws.String("us-east-1a"),
		VpcId:                     aws.String(redshiftProdVPCID),
		ClusterSubnetGroupName:    aws.String(RedshiftProdSubnetGroup),
		PubliclyAccessible:        aws.Bool(false),
		Encrypted:                 aws.Bool(true),
		Endpoint: &redshifttypes.Endpoint{
			Address: aws.String(id + ".c9xyz123.us-east-1.redshift.amazonaws.com"),
			Port:    aws.Int32(5439),
		},
	}
}

func buildRedshiftClusters() []redshifttypes.Cluster {
	// -----------------------------------------------------------------------
	// 1. acme-warehouse — Healthy, CloudWatch-logging graph-root (U9 §5.1)
	// Covers: alarm, cfn, kms, logs, role, secrets, sg, subnet, vpc, ct-events
	// -----------------------------------------------------------------------
	warehouse := redshifttypes.Cluster{
		ClusterIdentifier:         aws.String(AcmeWarehouseID),
		ClusterStatus:             aws.String("available"),
		ClusterAvailabilityStatus: aws.String("Available"),
		NodeType:                  aws.String("ra3.xlplus"),
		NumberOfNodes:             aws.Int32(4),
		DBName:                    aws.String("analytics"),
		MasterUsername:            aws.String("admin"),
		ClusterCreateTime:         aws.Time(mustParseRedshiftTime("2025-03-10T09:00:00Z")),
		ClusterNamespaceArn:       aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:" + AcmeWarehouseID),
		AvailabilityZone:          aws.String("us-east-1a"),
		VpcId:                     aws.String(redshiftProdVPCID),
		ClusterSubnetGroupName:    aws.String(RedshiftProdSubnetGroup),
		PubliclyAccessible:        aws.Bool(false),
		Encrypted:                 aws.Bool(true),
		KmsKeyId:                  aws.String(RedshiftKMSKeyARN1),
		MasterPasswordSecretArn:   aws.String(AcmeWarehouseSecretARN),
		Endpoint: &redshifttypes.Endpoint{
			Address: aws.String("acme-warehouse.c9xyz123.us-east-1.redshift.amazonaws.com"),
			Port:    aws.Int32(5439),
		},
		VpcSecurityGroups: []redshifttypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: aws.String(RedshiftWarehouseSGID1), Status: aws.String("active")},
			{VpcSecurityGroupId: aws.String(RedshiftWarehouseSGID2), Status: aws.String("active")},
		},
		IamRoles: []redshifttypes.ClusterIamRole{
			{IamRoleArn: aws.String(RedshiftCopyRoleARN), ApplyStatus: aws.String("in-sync")},
			{IamRoleArn: aws.String(RedshiftUnloadRoleARN), ApplyStatus: aws.String("in-sync")},
		},
		Tags: []redshifttypes.Tag{
			{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-warehouse-stack")},
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		},
	}

	// -----------------------------------------------------------------------
	// 2. acme-reporting — Healthy, S3-logging graph-root (U9 §5.1)
	// Covers: alarm, cfn, kms, role, secrets, s3, sg, subnet, vpc, ct-events
	// (logs=0 by design — S3 logging means no CW log groups)
	// -----------------------------------------------------------------------
	reporting := redshifttypes.Cluster{
		ClusterIdentifier:         aws.String(AcmeReportingID),
		ClusterStatus:             aws.String("available"),
		ClusterAvailabilityStatus: aws.String("Available"),
		NodeType:                  aws.String("ra3.xlplus"),
		NumberOfNodes:             aws.Int32(2),
		DBName:                    aws.String("reporting"),
		MasterUsername:            aws.String("admin"),
		ClusterCreateTime:         aws.Time(mustParseRedshiftTime("2025-07-22T14:30:00Z")),
		ClusterNamespaceArn:       aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:" + AcmeReportingID),
		AvailabilityZone:          aws.String("us-east-1b"),
		VpcId:                     aws.String(redshiftProdVPCID),
		ClusterSubnetGroupName:    aws.String(RedshiftProdSubnetGroup),
		PubliclyAccessible:        aws.Bool(false),
		Encrypted:                 aws.Bool(true),
		KmsKeyId:                  aws.String(RedshiftKMSKeyARN2),
		MasterPasswordSecretArn:   aws.String(AcmeReportingSecretARN),
		Endpoint: &redshifttypes.Endpoint{
			Address: aws.String("acme-reporting.c9xyz123.us-east-1.redshift.amazonaws.com"),
			Port:    aws.Int32(5439),
		},
		VpcSecurityGroups: []redshifttypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: aws.String(RedshiftReportingSGID1), Status: aws.String("active")},
			{VpcSecurityGroupId: aws.String(RedshiftReportingSGID2), Status: aws.String("active")},
		},
		IamRoles: []redshifttypes.ClusterIamRole{
			{IamRoleArn: aws.String(RedshiftReportingCopyRoleARN), ApplyStatus: aws.String("in-sync")},
		},
		Tags: []redshifttypes.Tag{
			{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-reporting-stack")},
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		},
	}

	// -----------------------------------------------------------------------
	// 3. staging-dwh — ClusterStatus=paused (Out of Scope → treated Healthy)
	// -----------------------------------------------------------------------
	stagingDwh := redshifttypes.Cluster{
		ClusterIdentifier:         aws.String(StagingDwhID),
		ClusterStatus:             aws.String("paused"),
		ClusterAvailabilityStatus: aws.String("Available"),
		NodeType:                  aws.String("dc2.large"),
		NumberOfNodes:             aws.Int32(2),
		DBName:                    aws.String("staging"),
		MasterUsername:            aws.String("stgadmin"),
		ClusterCreateTime:         aws.Time(mustParseRedshiftTime("2025-10-15T08:00:00Z")),
		ClusterNamespaceArn:       aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:" + StagingDwhID),
		AvailabilityZone:          aws.String("us-east-1a"),
		VpcId:                     aws.String(redshiftStagingVPCID),
		ClusterSubnetGroupName:    aws.String(RedshiftStagingSubnetGroup),
		PubliclyAccessible:        aws.Bool(false),
		Encrypted:                 aws.Bool(true),
		Endpoint: &redshifttypes.Endpoint{
			Address: aws.String("staging-dwh.c9xyz123.us-east-1.redshift.amazonaws.com"),
			Port:    aws.Int32(5439),
		},
	}

	// -----------------------------------------------------------------------
	// 4. redshift-resizing — ClusterStatus=resizing → Warning
	// -----------------------------------------------------------------------
	resizing := redshiftBaselineHealthy(RedshiftResizingID)
	resizing.ClusterStatus = aws.String("resizing")
	resizing.NumberOfNodes = aws.Int32(4)
	resizing.DBName = aws.String("analytics")
	resizing.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-05-01T10:00:00Z"))

	// -----------------------------------------------------------------------
	// 5. redshift-rebooting — ClusterStatus=rebooting → Warning
	// -----------------------------------------------------------------------
	rebooting := redshiftBaselineHealthy(RedshiftRebootingID)
	rebooting.ClusterStatus = aws.String("rebooting")
	rebooting.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-08-12T09:00:00Z"))

	// -----------------------------------------------------------------------
	// 6. redshift-incompatible-network — ClusterStatus=incompatible-network → Broken
	// -----------------------------------------------------------------------
	incompatNet := redshiftBaselineHealthy(RedshiftIncompatibleNetworkID)
	incompatNet.ClusterStatus = aws.String("incompatible-network")
	incompatNet.NumberOfNodes = aws.Int32(2)
	incompatNet.DBName = aws.String("prod")
	incompatNet.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-11-10T08:00:00Z"))
	incompatNet.AvailabilityZone = aws.String("us-east-1b")
	incompatNet.Endpoint = nil

	// -----------------------------------------------------------------------
	// 7. redshift-hardware-failure — ClusterStatus=hardware-failure → Broken
	// -----------------------------------------------------------------------
	hwFailure := redshiftBaselineHealthy(RedshiftHardwareFailureID)
	hwFailure.ClusterStatus = aws.String("hardware-failure")
	hwFailure.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-09-20T14:00:00Z"))
	hwFailure.Endpoint = nil

	// -----------------------------------------------------------------------
	// 8. redshift-storage-full — ClusterStatus=storage-full → Broken
	// -----------------------------------------------------------------------
	storageFull := redshiftBaselineHealthy(RedshiftStorageFullID)
	storageFull.ClusterStatus = aws.String("storage-full")
	storageFull.NodeType = aws.String("dc2.large")
	storageFull.DBName = aws.String("dwh")
	storageFull.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-08-20T14:00:00Z"))

	// -----------------------------------------------------------------------
	// 9. redshift-avail-unavailable — ClusterAvailabilityStatus=Unavailable → Broken
	// -----------------------------------------------------------------------
	availUnavailable := redshiftBaselineHealthy(RedshiftAvailUnavailableID)
	availUnavailable.ClusterAvailabilityStatus = aws.String("Unavailable")
	availUnavailable.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-12-01T10:00:00Z"))

	// -----------------------------------------------------------------------
	// 10. redshift-avail-failed — ClusterAvailabilityStatus=Failed → Broken
	// -----------------------------------------------------------------------
	availFailed := redshiftBaselineHealthy(RedshiftAvailFailedID)
	availFailed.ClusterAvailabilityStatus = aws.String("Failed")
	availFailed.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2026-01-15T09:00:00Z"))

	// -----------------------------------------------------------------------
	// 11. redshift-avail-maintenance — ClusterAvailabilityStatus=Maintenance → Warning
	// -----------------------------------------------------------------------
	availMaint := redshiftBaselineHealthy(RedshiftAvailMaintenanceID)
	availMaint.ClusterAvailabilityStatus = aws.String("Maintenance")
	availMaint.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-07-10T11:00:00Z"))

	// -----------------------------------------------------------------------
	// 12. redshift-avail-modifying — ClusterAvailabilityStatus=Modifying → Warning
	// -----------------------------------------------------------------------
	availModifying := redshiftBaselineHealthy(RedshiftAvailModifyingID)
	availModifying.ClusterAvailabilityStatus = aws.String("Modifying")
	availModifying.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-06-20T16:00:00Z"))

	// -----------------------------------------------------------------------
	// 13. redshift-pending-change — PendingModifiedValues.NodeType set → Warning
	// -----------------------------------------------------------------------
	pendingChange := redshiftBaselineHealthy(RedshiftPendingChangeID)
	pendingChange.PendingModifiedValues = &redshifttypes.PendingModifiedValues{
		NodeType: aws.String("ra3.4xlarge"),
	}
	pendingChange.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-04-01T08:00:00Z"))

	// -----------------------------------------------------------------------
	// 14. redshift-maintenance-deferred — active DeferredMaintenanceWindow → Warning
	// DeferMaintenanceStartTime = now-1h, DeferMaintenanceEndTime = now+48h
	// -----------------------------------------------------------------------
	now := time.Now()
	maintenanceDeferred := redshiftBaselineHealthy(RedshiftMaintenanceDeferredID)
	maintenanceDeferred.DeferredMaintenanceWindows = []redshifttypes.DeferredMaintenanceWindow{
		{
			DeferMaintenanceIdentifier: aws.String("dmw-active"),
			DeferMaintenanceStartTime:  aws.Time(now.Add(-1 * time.Hour)),
			DeferMaintenanceEndTime:    aws.Time(now.Add(48 * time.Hour)),
		},
	}
	maintenanceDeferred.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-03-15T12:00:00Z"))

	// -----------------------------------------------------------------------
	// 15. redshift-maintenance-deferred-expired — expired DeferredMaintenanceWindow → Healthy (negative case)
	// DeferMaintenanceEndTime is in the past → window inactive
	// -----------------------------------------------------------------------
	maintenanceDeferredExpired := redshiftBaselineHealthy(RedshiftMaintenanceDeferredExpiredID)
	maintenanceDeferredExpired.DeferredMaintenanceWindows = []redshifttypes.DeferredMaintenanceWindow{
		{
			DeferMaintenanceIdentifier: aws.String("dmw-expired"),
			DeferMaintenanceStartTime:  aws.Time(now.Add(-72 * time.Hour)),
			DeferMaintenanceEndTime:    aws.Time(now.Add(-24 * time.Hour)),
		},
	}
	maintenanceDeferredExpired.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-02-28T10:00:00Z"))

	// -----------------------------------------------------------------------
	// 16. redshift-publicly-accessible — PubliclyAccessible=true → Warning
	// -----------------------------------------------------------------------
	publiclyAccessible := redshiftBaselineHealthy(RedshiftPubliclyAccessibleID)
	publiclyAccessible.PubliclyAccessible = aws.Bool(true)
	publiclyAccessible.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-05-20T08:00:00Z"))

	// -----------------------------------------------------------------------
	// 17. redshift-unencrypted — Encrypted=false → Warning
	// -----------------------------------------------------------------------
	unencrypted := redshiftBaselineHealthy(RedshiftUnencryptedID)
	unencrypted.Encrypted = aws.Bool(false)
	unencrypted.KmsKeyId = nil
	unencrypted.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-04-10T14:00:00Z"))

	// -----------------------------------------------------------------------
	// 18. warn-redshift-multi — 3 coexisting §3.1 warnings → "pending change queued (+2)"
	// PendingModifiedValues + PubliclyAccessible + Encrypted=false
	// -----------------------------------------------------------------------
	warnMulti := redshiftBaselineHealthy(WarnRedshiftMultiID)
	warnMulti.PendingModifiedValues = &redshifttypes.PendingModifiedValues{
		NodeType: aws.String("ra3.4xlarge"),
	}
	warnMulti.PubliclyAccessible = aws.Bool(true)
	warnMulti.Encrypted = aws.Bool(false)
	warnMulti.KmsKeyId = nil
	warnMulti.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-07-01T09:00:00Z"))

	// -----------------------------------------------------------------------
	// 19. warn-redshift-two — 2 coexisting §3.1 warnings → "publicly accessible (+1)"
	// PubliclyAccessible + Encrypted=false
	// -----------------------------------------------------------------------
	warnTwo := redshiftBaselineHealthy(WarnRedshiftTwoID)
	warnTwo.PubliclyAccessible = aws.Bool(true)
	warnTwo.Encrypted = aws.Bool(false)
	warnTwo.KmsKeyId = nil
	warnTwo.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-08-05T11:00:00Z"))

	// -----------------------------------------------------------------------
	// 20. redshift-broken-with-warning-hidden — Broken suppresses Warnings (U8)
	// ClusterStatus=storage-full (Broken) + ClusterAvailabilityStatus=Modifying + PubliclyAccessible=true + Encrypted=false
	// Expected: Resource.Status = "broken: storage-full", Issues = ["broken: storage-full"]
	// -----------------------------------------------------------------------
	brokenWithWarning := redshiftBaselineHealthy(RedshiftBrokenWithWarningHiddenID)
	brokenWithWarning.ClusterStatus = aws.String("storage-full")
	brokenWithWarning.ClusterAvailabilityStatus = aws.String("Modifying")
	brokenWithWarning.PubliclyAccessible = aws.Bool(true)
	brokenWithWarning.Encrypted = aws.Bool(false)
	brokenWithWarning.KmsKeyId = nil
	brokenWithWarning.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-09-15T08:00:00Z"))

	// -----------------------------------------------------------------------
	// 21. redshift-avail-unavailable-with-warning-hidden — availability-driven Broken suppresses Warning (U8)
	// ClusterStatus=available, ClusterAvailabilityStatus=Unavailable + PubliclyAccessible=true
	// Expected: Resource.Status = "unavailable", Issues = ["unavailable"]
	// -----------------------------------------------------------------------
	availUnavailableWithWarning := redshiftBaselineHealthy(RedshiftAvailUnavailableWithWarningHiddenID)
	availUnavailableWithWarning.ClusterAvailabilityStatus = aws.String("Unavailable")
	availUnavailableWithWarning.PubliclyAccessible = aws.Bool(true)
	availUnavailableWithWarning.ClusterCreateTime = aws.Time(mustParseRedshiftTime("2025-10-01T10:00:00Z"))

	return []redshifttypes.Cluster{
		warehouse,
		reporting,
		stagingDwh,
		resizing,
		rebooting,
		incompatNet,
		hwFailure,
		storageFull,
		availUnavailable,
		availFailed,
		availMaint,
		availModifying,
		pendingChange,
		maintenanceDeferred,
		maintenanceDeferredExpired,
		publiclyAccessible,
		unencrypted,
		warnMulti,
		warnTwo,
		brokenWithWarning,
		availUnavailableWithWarning,
	}
}

func mustParseRedshiftTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Panic surfaces fixture typos during development — the alternative is
		// a silent zero-time which propagates into ClusterCreateTime and
		// produces confusing UI output in the demo.
		panic(fmt.Sprintf("mustParseRedshiftTime(%q): %v", s, err))
	}
	return t
}
