// Package fixtures provides MWAA (Managed Airflow) fixture data for the MWAA fake.
package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"
)

// MWAAFixtures holds typed fixture data for Managed Airflow (MWAA).
type MWAAFixtures struct {
	// Environments maps environment name (Resource.ID, bare — MWAA addresses
	// environments by name, no ARN params exist in the API surface) to the
	// GetEnvironment response body. ListEnvironments returns only the map
	// keys; every Wave-2 finding and every related-panel field is read off
	// this value.
	Environments map[string]mwaatypes.Environment
	// DeniedNames are listed by ListEnvironments but GetEnvironment returns
	// AccessDeniedException for them — the live-witnessed IAM shape where a
	// role may list environments but not read their details. The fetcher
	// keeps these as name-only degraded rows (finding mwaa.warn.details_denied).
	DeniedNames []string
}

// Exported environment-name constants — referenced by sibling fixture files
// (as the alarm dimension value / log-group name segment) and by QA tests.
const (
	ProdAirflowEtlID            = "prod-airflow-etl"
	ProdAirflowReportingID      = "prod-airflow-reporting"
	WarnAirflowCreatingID       = "warn-airflow-creating"
	WarnAirflowSnapshottingID   = "warn-airflow-snapshotting"
	WarnAirflowPendingID        = "warn-airflow-pending"
	WarnAirflowUpdatingID       = "warn-airflow-updating"
	WarnAirflowMaintenanceID    = "warn-airflow-maintenance"
	WarnAirflowRollbackID       = "warn-airflow-rollback"
	BrokenAirflowCreateFailedID = "broken-airflow-create-failed"
	BrokenAirflowUpdateFailedID = "broken-airflow-update-failed"
	BrokenAirflowUnavailableID  = "broken-airflow-unavailable"
	DimAirflowDeletingID        = "dim-airflow-deleting"
	DimAirflowDeletedID         = "dim-airflow-deleted"
	WarnAirflowStaleUpdateID    = "warn-airflow-stale-update"
	WarnAirflowPublicID         = "warn-airflow-public"
	WarnAirflowMultiID          = "warn-airflow-multi"
	WarnAirflowDetailsDeniedID  = "warn-airflow-details-denied"
)

const (
	mwaaRegion    = "us-east-1"
	mwaaAccountID = "123456789012"

	// mwaaPrimaryKMSKeyARN reuses the widely-shared "primary production key"
	// (kms.go) rather than minting a dedicated MWAA CMK.
	mwaaPrimaryKMSKeyARN = "arn:aws:kms:" + mwaaRegion + ":" + mwaaAccountID + ":key/a1b2c3d4-5678-90ab-cdef-111111111111"

	// mwaaServiceRoleArn is the account's AWS-managed MWAA service-linked
	// role — a detail-only fact, never a related-panel pivot.
	mwaaServiceRoleArn = "arn:aws:iam::" + mwaaAccountID + ":role/aws-service-role/airflow.amazonaws.com/AWSServiceRoleForAmazonMWAA"
)

// mwaaLogGroupName follows MWAA's "airflow-<environment>-<Component>"
// CloudWatch log-group naming convention (bare name — matches CWLogsFixtures'
// LogGroupName field for the mwaa→logs related-panel pivot).
func mwaaLogGroupName(envName, component string) string {
	return "airflow-" + envName + "-" + component
}

// mwaaLogGroupArn is the full CloudWatch log-group ARN AWS reports on
// LoggingConfiguration.*.CloudWatchLogGroupArn (trailing ":*" wildcard,
// matching the real API's documented example shape).
func mwaaLogGroupArn(envName, component string) string {
	return "arn:aws:logs:" + mwaaRegion + ":" + mwaaAccountID + ":log-group:" + mwaaLogGroupName(envName, component) + ":*"
}

// mwaaLoggingConfig builds all five per-component logging configurations for
// envName. webserverEnabled=false models the wave-3 anti-test fixture
// (prod-airflow-reporting): a disabled logging component must never raise a
// finding.
func mwaaLoggingConfig(envName string, webserverEnabled bool) *mwaatypes.LoggingConfiguration {
	component := func(name string, enabled bool) *mwaatypes.ModuleLoggingConfiguration {
		return &mwaatypes.ModuleLoggingConfiguration{
			CloudWatchLogGroupArn: aws.String(mwaaLogGroupArn(envName, name)),
			Enabled:               aws.Bool(enabled),
			LogLevel:              mwaatypes.LoggingLevelInfo,
		}
	}
	return &mwaatypes.LoggingConfiguration{
		DagProcessingLogs: component("DAGProcessing", true),
		SchedulerLogs:     component("Scheduler", true),
		WebserverLogs:     component("WebServer", webserverEnabled),
		WorkerLogs:        component("Worker", true),
		TaskLogs:          component("Task", true),
	}
}

// mwaaNetworkConfig reuses two existing prod-VPC private subnets and two
// existing prod-VPC security groups (ec2.go) — every MWAA demo environment
// shares one account/VPC network fleet, which is also realistic.
func mwaaNetworkConfig() *mwaatypes.NetworkConfiguration {
	return &mwaatypes.NetworkConfiguration{
		SubnetIds:        []string{fixtProdPrivateSubnetA, fixtProdPrivateSubnetB},
		SecurityGroupIds: []string{fixtProdAPIInternalSGID, fixtProdRDSSGID},
	}
}

// mwaaPrivateWebserverURL builds the PRIVATE_ONLY VPC-endpoint webserver
// hostname shape: "<uuid>-vpce.c1.airflow.<region>.on.aws".
func mwaaPrivateWebserverURL(uuidNoDashes string) string {
	return uuidNoDashes + "-vpce.c1.airflow." + mwaaRegion + ".on.aws"
}

// mwaaPublicWebserverURL builds a PUBLIC_ONLY/PUBLIC_AND_PRIVATE webserver
// hostname — reachable from the internet, still IAM-authed.
func mwaaPublicWebserverURL(uuidNoDashes string) string {
	return uuidNoDashes + ".c10.airflow." + mwaaRegion + ".on.aws"
}

// mwaaCeleryQueueARN builds the Celery Executor queue ARN in an AWS-owned
// account (471100000000) — distinct from the customer account, mirroring
// the real AWS-owned-account reality (detail fact only, no related-panel
// pivot: docs/resources/mwaa.md §2 `sqs` exclusion).
func mwaaCeleryQueueARN(uuid string) string {
	return "arn:aws:sqs:" + mwaaRegion + ":471100000000:airflow-celery-" + uuid
}

// mwaaBaseEnvironment returns the common shape shared by every demo
// environment; callers override the fields that vary per fixture.
func mwaaBaseEnvironment(name string, status mwaatypes.EnvironmentStatus, class string, uuid string) mwaatypes.Environment {
	return mwaatypes.Environment{
		Name:                         aws.String(name),
		Arn:                          aws.String("arn:aws:airflow:" + mwaaRegion + ":" + mwaaAccountID + ":environment/" + name),
		Status:                       status,
		AirflowVersion:               aws.String("2.10.3"),
		EnvironmentClass:             aws.String(class),
		MinWorkers:                   aws.Int32(1),
		MaxWorkers:                   aws.Int32(5),
		Schedulers:                   aws.Int32(1),
		MinWebservers:                aws.Int32(2),
		MaxWebservers:                aws.Int32(2),
		WebserverAccessMode:          mwaatypes.WebserverAccessModePrivateOnly,
		EndpointManagement:           mwaatypes.EndpointManagementService,
		WeeklyMaintenanceWindowStart: aws.String("SUN:03:00"),
		CreatedAt:                    aws.Time(time.Date(2025, 9, 1, 8, 0, 0, 0, time.UTC)),
		KmsKey:                       aws.String(mwaaPrimaryKMSKeyARN),
		ExecutionRoleArn:             aws.String(fixtIAMProdLambdaRoleARN),
		ServiceRoleArn:               aws.String(mwaaServiceRoleArn),
		SourceBucketArn:              aws.String(HealthyBucketARN),
		DagS3Path:                    aws.String("dags"),
		RequirementsS3Path:           aws.String("requirements/requirements.txt"),
		LoggingConfiguration:         mwaaLoggingConfig(name, true),
		NetworkConfiguration:         mwaaNetworkConfig(),
		CeleryExecutorQueue:          aws.String(mwaaCeleryQueueARN(uuid)),
		WebserverUrl:                 aws.String(mwaaPrivateWebserverURL(uuid)),
		LastUpdate: &mwaatypes.LastUpdate{
			Status:    mwaatypes.UpdateStatusSuccess,
			CreatedAt: aws.Time(time.Date(2025, 12, 1, 6, 0, 0, 0, time.UTC)),
		},
	}
}

// withLastUpdateFailed overrides env's LastUpdate to a failed update carrying
// errorCode/errorMessage — the "last update failed" §3.2 finding witness.
func withLastUpdateFailed(env mwaatypes.Environment, errorCode, errorMessage string) mwaatypes.Environment {
	env.LastUpdate = &mwaatypes.LastUpdate{
		Status:    mwaatypes.UpdateStatusFailed,
		CreatedAt: aws.Time(time.Date(2025, 12, 10, 4, 0, 0, 0, time.UTC)),
		Error: &mwaatypes.UpdateError{
			ErrorCode:    aws.String(errorCode),
			ErrorMessage: aws.String(errorMessage),
		},
	}
	return env
}

// NewMWAAFixtures constructs MWAAFixtures from the canonical demo data.
var sharedMWAAFixtures = sync.OnceValue(func() *MWAAFixtures {
	envs := make(map[string]mwaatypes.Environment, 16)

	// GRAPH ROOT — rich pivots: kms 1, logs 5, role 1, s3 1, sg 2, subnet 2,
	// alarm 2 (siblings added in cloudwatch.go/cwlogs.go). AVAILABLE, healthy.
	etl := mwaaBaseEnvironment(ProdAirflowEtlID, mwaatypes.EnvironmentStatusAvailable, "mw1.large",
		"6f3a9c1e2b4d4e8f91a37d5c8e2f4b6a")
	etl.Schedulers = aws.Int32(2)
	etl.WeeklyMaintenanceWindowStart = aws.String("WED:22:30")
	envs[ProdAirflowEtlID] = etl

	// Healthy silence row: WebserverLogs disabled (wave-3 anti-test — a
	// disabled logging component must never raise a finding). mw1.small, 1
	// scheduler.
	reporting := mwaaBaseEnvironment(ProdAirflowReportingID, mwaatypes.EnvironmentStatusAvailable, "mw1.small",
		"8b2d4f6a1c3e4a7b9d5f2e4c6a8b0d1f")
	reporting.Schedulers = aws.Int32(1)
	reporting.LoggingConfiguration = mwaaLoggingConfig(ProdAirflowReportingID, false)
	envs[ProdAirflowReportingID] = reporting

	envs[WarnAirflowCreatingID] = mwaaBaseEnvironment(WarnAirflowCreatingID, mwaatypes.EnvironmentStatusCreating, "mw1.small",
		"1a2b3c4d5e6f4a7b8c9d0e1f2a3b4c5d")
	envs[WarnAirflowSnapshottingID] = mwaaBaseEnvironment(WarnAirflowSnapshottingID, mwaatypes.EnvironmentStatusCreatingSnapshot, "mw1.small",
		"2b3c4d5e6f7a4b8c9d0e1f2a3b4c5d6e")
	envs[WarnAirflowPendingID] = mwaaBaseEnvironment(WarnAirflowPendingID, mwaatypes.EnvironmentStatusPending, "mw1.small",
		"3c4d5e6f7a8b4c9d0e1f2a3b4c5d6e7f")
	envs[WarnAirflowUpdatingID] = mwaaBaseEnvironment(WarnAirflowUpdatingID, mwaatypes.EnvironmentStatusUpdating, "mw1.small",
		"4d5e6f7a8b9c4d0e1f2a3b4c5d6e7f80")
	envs[WarnAirflowMaintenanceID] = mwaaBaseEnvironment(WarnAirflowMaintenanceID, mwaatypes.EnvironmentStatusMaintenance, "mw1.small",
		"5e6f7a8b9c0d4e1f2a3b4c5d6e7f8091")

	// ROLLING_BACK + LastUpdate FAILED — two findings stack on a yellow row:
	// the state phrase ("rolling back: update failed") and the failed-update
	// sentence with its ErrorMessage.
	rollback := mwaaBaseEnvironment(WarnAirflowRollbackID, mwaatypes.EnvironmentStatusRollingBack, "mw1.small",
		"6f7a8b9c0d1e4f2a3b4c5d6e7f809122")
	rollback = withLastUpdateFailed(rollback, "ROLLBACK_IN_PROGRESS", "Update to 2.11.0 failed; restoring metadata snapshot")
	envs[WarnAirflowRollbackID] = rollback

	envs[BrokenAirflowCreateFailedID] = mwaaBaseEnvironment(BrokenAirflowCreateFailedID, mwaatypes.EnvironmentStatusCreateFailed, "mw1.small",
		"7a8b9c0d1e2f4a3b4c5d6e7f80912233")

	updateFailed := mwaaBaseEnvironment(BrokenAirflowUpdateFailedID, mwaatypes.EnvironmentStatusUpdateFailed, "mw1.small",
		"8b9c0d1e2f3a4b4c5d6e7f8091223344")
	updateFailed = withLastUpdateFailed(updateFailed, "INCORRECT_CONFIGURATION", "Environment update failed; rolled back to previous state")
	envs[BrokenAirflowUpdateFailedID] = updateFailed

	envs[BrokenAirflowUnavailableID] = mwaaBaseEnvironment(BrokenAirflowUnavailableID, mwaatypes.EnvironmentStatusUnavailable, "mw1.small",
		"9c0d1e2f3a4b4c5d6e7f809122334455")
	envs[DimAirflowDeletingID] = mwaaBaseEnvironment(DimAirflowDeletingID, mwaatypes.EnvironmentStatusDeleting, "mw1.small",
		"0d1e2f3a4b4c4d5e6f70912233445566")
	envs[DimAirflowDeletedID] = mwaaBaseEnvironment(DimAirflowDeletedID, mwaatypes.EnvironmentStatusDeleted, "mw1.small",
		"1a1e2f3a4b4c4d5e6f70912233445567")

	// AVAILABLE + LastUpdate FAILED — background `~` finding, row stays green.
	staleUpdate := mwaaBaseEnvironment(WarnAirflowStaleUpdateID, mwaatypes.EnvironmentStatusAvailable, "mw1.small",
		"1e2f3a4b4c5d4e6f7081223344556677")
	staleUpdate = withLastUpdateFailed(staleUpdate, "INCORRECT_CONFIGURATION", "Scheduler failed to launch: requirements.txt install failed")
	envs[WarnAirflowStaleUpdateID] = staleUpdate

	// AVAILABLE + WebserverAccessMode PUBLIC_ONLY — background `~` finding.
	public := mwaaBaseEnvironment(WarnAirflowPublicID, mwaatypes.EnvironmentStatusAvailable, "mw1.small",
		"2f3a4b4c5d6e4f708122334455667788")
	public.WebserverAccessMode = mwaatypes.WebserverAccessModePublicOnly
	public.WebserverUrl = aws.String(mwaaPublicWebserverURL("2f3a4b4c5d6e4f708122334455667788"))
	envs[WarnAirflowPublicID] = public

	// AVAILABLE + LastUpdate FAILED + PUBLIC_ONLY — both background findings
	// on the same green row: S4 "last update failed (+1)".
	multi := mwaaBaseEnvironment(WarnAirflowMultiID, mwaatypes.EnvironmentStatusAvailable, "mw1.small",
		"3a4b4c5d6e7f40819223344556677899")
	multi = withLastUpdateFailed(multi, "INCORRECT_CONFIGURATION", "Scheduler failed to launch: requirements.txt install failed")
	multi.WebserverAccessMode = mwaatypes.WebserverAccessModePublicOnly
	multi.WebserverUrl = aws.String(mwaaPublicWebserverURL("3a4b4c5d6e7f40819223344556677899"))
	envs[WarnAirflowMultiID] = multi

	// Listed but GetEnvironment-denied — the live-witnessed IAM shape (a
	// readonly role allowing ListEnvironments while denying
	// airflow:GetEnvironment). Renders as a name-only degraded row with the
	// `details denied` finding; deliberately NOT in the Environments map.
	denied := []string{WarnAirflowDetailsDeniedID}

	return &MWAAFixtures{Environments: envs, DeniedNames: denied}
})

func NewMWAAFixtures() *MWAAFixtures {
	return sharedMWAAFixtures()
}
