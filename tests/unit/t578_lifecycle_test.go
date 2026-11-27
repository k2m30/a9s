package unit

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

func t1aIssueResult(res runtime.ProbeEnrichmentResult) awsclient.IssueEnricherResult {
	return awsclient.IssueEnricherResult{Findings: res.Findings, AttentionDetails: res.AttentionDetails, TruncatedIDs: res.TruncatedIDs}
}

var t1aNoBackupPlans = resource.ResourceCache{"backup": {Resources: []resource.Resource{}}}

// ---- cfn: posture findings follow the stack's lifecycle class ----

func t578CFNStackWithSecretOutput(name, status string) cfntypes.Stack {
	s := w6bCFNStack(name, status, aws.Bool(false))
	s.Outputs = []cfntypes.Output{
		{OutputKey: aws.String("ServiceEndpoint"), OutputValue: aws.String("https://" + name + ".example.com")},
		{OutputKey: aws.String("DBPassword"), OutputValue: aws.String("hunter2hunter2")},
	}
	return s
}

// A stack whose update or update-rollback failed still owns every resource it
// built and still accepts ContinueUpdateRollback or a new update, so its
// protection and its outputs are as exposed as on any live stack.
func TestT578CFN_FailedButLiveStacksKeepPostureFindings(t *testing.T) {
	for _, tc := range []struct {
		status    string
		stateCode domain.FindingCode
		phrase    string
	}{
		{"UPDATE_ROLLBACK_FAILED", awsclient.CodeCFNStackRollback, "update rollback failed"},
		{"UPDATE_FAILED", awsclient.CodeCFNStackFailed, "update failed"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			name := "acme-orders-" + strings.ToLower(tc.status)
			r := pw1ResourceByID(t, w6bFetchCFN(t, t578CFNStackWithSecretOutput(name, tc.status)), name)

			pw1RequireFinding(t, r.Findings, tc.stateCode, tc.phrase, domain.SevBroken, "wave1")
			pw1RequireFinding(t, r.Findings, w6bCFNCodeTerminationProtectionOff,
				"termination protection off", domain.SevWarn, "wave1")
			pw1RequireFinding(t, r.Findings, w6bCFNCodeOutputSecret,
				"credential in stack outputs", domain.SevBroken, "wave1")
			if got := r.Fields["termination_protection"]; got != "off" {
				t.Errorf("Fields[termination_protection] = %q, want %q", got, "off")
			}
		})
	}
}

// A stack being deleted or deleted keeps no posture to fix.
func TestT578CFN_DeleteStacksStayTearingDown(t *testing.T) {
	for _, status := range []string{"DELETE_IN_PROGRESS", "DELETE_COMPLETE"} {
		t.Run(status, func(t *testing.T) {
			name := "acme-retired-" + strings.ToLower(status)
			r := pw1ResourceByID(t, w6bFetchCFN(t, t578CFNStackWithSecretOutput(name, status)), name)
			pw1RequireNoFinding(t, r.Findings, w6bCFNCodeTerminationProtectionOff)
			pw1RequireNoFinding(t, r.Findings, w6bCFNCodeOutputSecret)
		})
	}
}

// ROLLBACK_COMPLETE is a create that failed and was undone: AWS accepts only a
// delete on such a stack, so asking to protect it from deletion is advice
// against the one action left. The rollback state itself is the finding.
func TestT578CFN_RollbackCompleteRaisesNoTerminationProtection(t *testing.T) {
	const name = "acme-legacy-api-v2"
	r := pw1ResourceByID(t, w6bFetchCFN(t, w6bCFNStack(name, "ROLLBACK_COMPLETE", nil)), name)

	pw1RequireFinding(t, r.Findings, awsclient.CodeCFNStackRollback, "rollback complete", domain.SevBroken, "wave1")
	pw1RequireNoFinding(t, r.Findings, w6bCFNCodeTerminationProtectionOff)

	// UPDATE_ROLLBACK_COMPLETE is a live stack back on its previous template.
	const live = "acme-monitoring-v2"
	lr := pw1ResourceByID(t, w6bFetchCFN(t, w6bCFNStack(live, "UPDATE_ROLLBACK_COMPLETE", nil)), live)
	pw1RequireFinding(t, lr.Findings, w6bCFNCodeTerminationProtectionOff,
		"termination protection off", domain.SevWarn, "wave1")
}

// ---- cfn: recent resource failure belongs to the latest operation ----

type t578CFNEventsFake struct {
	awsclient.CFNAPI
	events map[string][]cfntypes.StackEvent
}

func (f *t578CFNEventsFake) DescribeStackEvents(_ context.Context, in *cloudformation.DescribeStackEventsInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
	return &cloudformation.DescribeStackEventsOutput{StackEvents: f.events[aws.ToString(in.StackName)]}, nil
}

func t578StackArn(name string) string {
	return "arn:aws:cloudformation:us-east-1:123456789012:stack/" + name + "/0f1e2d3c-4b5a-6978-8a9b-0c1d2e3f4a5b"
}

// t578Event builds one DescribeStackEvents entry. An empty logical id makes it
// the stack's own event: AWS reports those with the stack name as
// LogicalResourceId and the stack ARN as PhysicalResourceId.
func t578Event(stack string, minute int, logicalID, resType string, status cfntypes.ResourceStatus, reason string) cfntypes.StackEvent {
	physical := logicalID + "-physical"
	if logicalID == "" {
		logicalID, resType, physical = stack, "AWS::CloudFormation::Stack", t578StackArn(stack)
	}
	ev := cfntypes.StackEvent{
		EventId:            aws.String(stack + "-" + logicalID + "-" + string(status)),
		StackId:            aws.String(t578StackArn(stack)),
		StackName:          aws.String(stack),
		LogicalResourceId:  aws.String(logicalID),
		PhysicalResourceId: aws.String(physical),
		ResourceType:       aws.String(resType),
		ResourceStatus:     status,
		Timestamp:          aws.Time(time.Date(2026, 9, 30, 10, minute, 0, 0, time.UTC)),
	}
	if reason != "" {
		ev.ResourceStatusReason = aws.String(reason)
	}
	return ev
}

func t578FailedResourceValues(res awsclient.IssueEnricherResult, id string) []string {
	var out []string
	for _, row := range pw1Rows(res, id, "cfn.recent-resource-failure") {
		if row.Label == "Failed resource" {
			out = append(out, row.Value)
		}
	}
	return out
}

// DescribeStackEvents answers newest first and keeps every past operation. A
// failure from an update that was rolled back and then superseded by a
// successful update is history, not a current problem; the failure that made
// the latest update roll back is the current problem. The latest operation
// starts at the newest stack-level CREATE/UPDATE/DELETE/IMPORT _IN_PROGRESS of
// the stack itself — not at its own rollback or cleanup phase, and not at a
// nested stack resource's event.
func TestT578CFN_RecentFailureOnlyFromLatestOperation(t *testing.T) {
	const healed, failing = "acme-orders", "acme-payments"
	fake := &t578CFNEventsFake{events: map[string][]cfntypes.StackEvent{
		healed: {
			t578Event(healed, 50, "", "", cfntypes.ResourceStatusUpdateComplete, ""),
			t578Event(healed, 49, "", "", cfntypes.ResourceStatus("UPDATE_COMPLETE_CLEANUP_IN_PROGRESS"), ""),
			t578Event(healed, 48, "OrdersQueue", "AWS::SQS::Queue", cfntypes.ResourceStatusUpdateComplete, ""),
			t578Event(healed, 47, "", "", cfntypes.ResourceStatusUpdateInProgress, "User Initiated"),
			t578Event(healed, 30, "", "", cfntypes.ResourceStatus("UPDATE_ROLLBACK_COMPLETE"), ""),
			t578Event(healed, 29, "OrdersTable", "AWS::DynamoDB::Table", cfntypes.ResourceStatusUpdateComplete, ""),
			t578Event(healed, 28, "", "", cfntypes.ResourceStatus("UPDATE_ROLLBACK_IN_PROGRESS"), "The following resource(s) failed to update: [OrdersTable]."),
			t578Event(healed, 27, "OrdersTable", "AWS::DynamoDB::Table", cfntypes.ResourceStatusUpdateFailed, "Subscriber limit exceeded: Provisioned throughput decreases are limited within a given UTC day"),
			t578Event(healed, 26, "", "", cfntypes.ResourceStatusUpdateInProgress, "User Initiated"),
		},
		failing: {
			t578Event(failing, 40, "", "", cfntypes.ResourceStatus("UPDATE_ROLLBACK_COMPLETE"), ""),
			t578Event(failing, 39, "", "", cfntypes.ResourceStatus("UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS"), ""),
			t578Event(failing, 38, "PaymentsFunction", "AWS::Lambda::Function", cfntypes.ResourceStatusUpdateComplete, ""),
			t578Event(failing, 37, "", "", cfntypes.ResourceStatus("UPDATE_ROLLBACK_IN_PROGRESS"), "The following resource(s) failed to update: [PaymentsFunction]."),
			t578Event(failing, 36, "AuditNested", "AWS::CloudFormation::Stack", cfntypes.ResourceStatusUpdateInProgress, ""),
			t578Event(failing, 35, "PaymentsFunction", "AWS::Lambda::Function", cfntypes.ResourceStatusUpdateFailed, "Resource handler returned message: \"Unzipped size must be smaller than 262144000 bytes\""),
			t578Event(failing, 34, "", "", cfntypes.ResourceStatusUpdateInProgress, "User Initiated"),
			t578Event(failing, 20, "", "", cfntypes.ResourceStatusUpdateComplete, ""),
			t578Event(failing, 19, "PaymentsQueue", "AWS::SQS::Queue", cfntypes.ResourceStatusUpdateFailed, "Resource handler returned message: \"Invalid value for the parameter VisibilityTimeout\""),
			t578Event(failing, 18, "", "", cfntypes.ResourceStatusUpdateInProgress, "User Initiated"),
		},
	}}
	rs := w6bFetchCFN(t,
		w6bCFNStack(healed, "UPDATE_COMPLETE", aws.Bool(true)),
		w6bCFNStack(failing, "UPDATE_ROLLBACK_COMPLETE", aws.Bool(true)),
	)
	res, err := awsclient.EnrichCFNStackEvents(context.Background(), &awsclient.ServiceClients{CloudFormation: fake}, rs, nil)
	if err != nil {
		t.Fatalf("EnrichCFNStackEvents: %v", err)
	}

	if got := res.Findings[healed]; len(got) != 0 {
		t.Errorf("%s (UPDATE_COMPLETE, failure only in an earlier operation) findings = %v, want none", healed, got)
	}
	if !hasCode(res.Findings[failing], "cfn.recent-resource-failure") {
		t.Fatalf("%s findings = %v, want cfn.recent-resource-failure for the update that rolled back", failing, res.Findings[failing])
	}
	want := []string{"AWS::Lambda::Function/PaymentsFunction"}
	if got := t578FailedResourceValues(res, failing); !slices.Equal(got, want) {
		t.Errorf("%s Failed resource rows = %v, want %v (the earlier PaymentsQueue failure is history)", failing, got, want)
	}
}

// ---- dbi: a live instance mid-operation keeps its posture findings ----

type t578RDSFake struct{ dbs []rdstypes.DBInstance }

func (f *t578RDSFake) DescribeDBInstances(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return &rds.DescribeDBInstancesOutput{DBInstances: f.dbs}, nil
}

func t578ExposedDBI(id, status string) rdstypes.DBInstance {
	return rdstypes.DBInstance{
		DBInstanceIdentifier:             aws.String(id),
		DBInstanceArn:                    aws.String("arn:aws:rds:us-east-1:123456789012:db:" + id),
		DBInstanceStatus:                 aws.String(status),
		DBInstanceClass:                  aws.String("db.t3.medium"),
		Engine:                           aws.String("postgres"),
		EngineVersion:                    aws.String("16.4"),
		Endpoint:                         &rdstypes.Endpoint{Address: aws.String(id + ".abc123xyz.us-east-1.rds.amazonaws.com"), Port: aws.Int32(5432)},
		MultiAZ:                          aws.Bool(true),
		AutoMinorVersionUpgrade:          aws.Bool(true),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		MasterUsername:                   aws.String("acme_app"),
		PubliclyAccessible:               aws.Bool(true),
		StorageEncrypted:                 aws.Bool(false),
		BackupRetentionPeriod:            aws.Int32(0),
		DeletionProtection:               aws.Bool(false),
	}
}

var t578DBIPostureCodes = []domain.FindingCode{
	awsclient.CodeDBIPubliclyAccessible,
	awsclient.CodeDBIUnencryptedStorage,
	awsclient.CodeDBINoAutomatedBackups,
	awsclient.CodeDBIDeletionProtectionOff,
}

func t578FetchDBI(t *testing.T, db rdstypes.DBInstance) resource.Resource {
	t.Helper()
	out, err := BindPage("dbi")(awsclient.FetchRDSInstancesPage(context.Background(), &t578RDSFake{dbs: []rdstypes.DBInstance{db}}, ""))
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage: %v", err)
	}
	return pw1ResourceByID(t, out.Resources, aws.ToString(db.DBInstanceIdentifier))
}

// A backup, a modification or a stop leaves the endpoint public and the
// storage unencrypted; the state finding sits beside those, it does not hide
// them.
func TestT578DBI_LiveTransitionalAndStoppedKeepPostureFindings(t *testing.T) {
	for _, tc := range []struct {
		status    string
		stateCode domain.FindingCode
	}{
		{"backing-up", awsclient.CodeDBITransitional},
		{"modifying", awsclient.CodeDBITransitional},
		{"stopped", awsclient.CodeDBIStopped},
	} {
		t.Run(tc.status, func(t *testing.T) {
			r := t578FetchDBI(t, t578ExposedDBI("acme-orders-db-"+tc.status, tc.status))
			if !hasCode(r.Findings, tc.stateCode) {
				t.Errorf("findings %v lack the state finding %s", r.Findings, tc.stateCode)
			}
			for _, code := range t578DBIPostureCodes {
				if !hasCode(r.Findings, code) {
					t.Errorf("%s instance: missing %s among %v", tc.status, code, r.Findings)
				}
			}
		})
	}
}

func TestT578DBI_DeletingInstancesSkipPostureFindings(t *testing.T) {
	r := t578FetchDBI(t, t578ExposedDBI("acme-retired-db", "deleting"))
	for _, code := range t578DBIPostureCodes {
		pw1RequireNoFinding(t, r.Findings, code)
	}
	if got := r.Fields["status"]; got != "deleting" {
		t.Errorf("Fields[status] = %q, want the AWS value %q", got, "deleting")
	}
}

// ---- dbc: Aurora and DocumentDB clusters mid-operation keep posture ----

type t1aDocDBClusters struct{ clusters []docdbtypes.DBCluster }

func (f t1aDocDBClusters) DescribeDBClusters(context.Context, *docdb.DescribeDBClustersInput, ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{DBClusters: f.clusters}, nil
}

type t1aRDSClusters struct{ clusters []rdstypes.DBCluster }

func (f t1aRDSClusters) DescribeDBClusters(context.Context, *rds.DescribeDBClustersInput, ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{DBClusters: f.clusters}, nil
}

func t1aExposedCluster(t *testing.T, lane, id, status string) resource.Resource {
	t.Helper()
	c := t564Cluster{id: id, status: status, subnetGroup: "acme-db-subnets", autoMinor: true, iamAuth: true}
	var res resource.FetchResult
	var err error
	if lane == "docdb" {
		c.engine = "docdb"
		d := c.docdb()
		d.DeletionProtection, d.StorageEncrypted = aws.Bool(false), aws.Bool(false)
		res, err = BindPage("dbc")(awsclient.FetchDocDBClustersPage(context.Background(), t1aDocDBClusters{[]docdbtypes.DBCluster{d}}, ""))
	} else {
		c.engine = "aurora-postgresql"
		r := c.rds()
		r.DeletionProtection, r.StorageEncrypted = aws.Bool(false), aws.Bool(false)
		res, err = BindPage("dbc")(awsclient.FetchRDSDBClustersPage(context.Background(), t1aRDSClusters{[]rdstypes.DBCluster{r}}, ""))
	}
	if err != nil {
		t.Fatalf("%s lane: %v", lane, err)
	}
	return pw1ResourceByID(t, res.Resources, id)
}

var t1aDBCPostureCodes = []domain.FindingCode{awsclient.CodeDBCDeletionProtectionOff, awsclient.CodeDBCNotEncryptedAtRest}

// An Aurora or DocumentDB cluster that is backing up, modifying or stopped is
// still the cluster that holds the data; its missing deletion protection and
// unencrypted storage stand beside the state finding. A deleting cluster
// keeps only its state.
func TestT1aDBC_LiveTransitionalAndStoppedKeepPostureFindings(t *testing.T) {
	for _, lane := range []string{"docdb", "aurora"} {
		for _, tc := range []struct {
			status    string
			stateCode domain.FindingCode
		}{
			{"backing-up", awsclient.CodeDBCTransitional},
			{"modifying", awsclient.CodeDBCTransitional},
			{"stopped", awsclient.CodeDBCStopped},
		} {
			t.Run(lane+"/"+tc.status, func(t *testing.T) {
				r := t1aExposedCluster(t, lane, "acme-orders-"+lane+"-"+tc.status, tc.status)
				if !hasCode(r.Findings, tc.stateCode) {
					t.Errorf("findings %v lack the state finding %s", r.Findings, tc.stateCode)
				}
				for _, code := range t1aDBCPostureCodes {
					if !hasCode(r.Findings, code) {
						t.Errorf("%s cluster: missing %s among %v", tc.status, code, r.Findings)
					}
				}
				if got := r.Fields["status"]; got != tc.status {
					t.Errorf("Fields[status] = %q, want the AWS value %q", got, tc.status)
				}
			})
		}
		t.Run(lane+"/deleting", func(t *testing.T) {
			r := t1aExposedCluster(t, lane, "acme-retired-"+lane, "deleting")
			for _, code := range t1aDBCPostureCodes {
				pw1RequireNoFinding(t, r.Findings, code)
			}
			if len(r.Findings) == 0 {
				t.Errorf("deleting cluster carries no state finding")
			}
		})
	}
}

// ---- redshift: every cluster status renders; final-snapshot is teardown ----

type t578RedshiftFake struct {
	awsclient.RedshiftAPI
	clusters []redshifttypes.Cluster

	mu         sync.Mutex
	logCalls   []string
	paramCalls []string
}

func (f *t578RedshiftFake) DescribeClusters(_ context.Context, _ *redshift.DescribeClustersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	return &redshift.DescribeClustersOutput{Clusters: f.clusters}, nil
}

func (f *t578RedshiftFake) DescribeLoggingStatus(_ context.Context, in *redshift.DescribeLoggingStatusInput, _ ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error) {
	f.mu.Lock()
	f.logCalls = append(f.logCalls, aws.ToString(in.ClusterIdentifier))
	f.mu.Unlock()
	return &redshift.DescribeLoggingStatusOutput{LoggingEnabled: aws.Bool(false)}, nil
}

func (f *t578RedshiftFake) DescribeClusterParameters(_ context.Context, in *redshift.DescribeClusterParametersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClusterParametersOutput, error) {
	f.mu.Lock()
	f.paramCalls = append(f.paramCalls, aws.ToString(in.ParameterGroupName))
	f.mu.Unlock()
	return &redshift.DescribeClusterParametersOutput{Parameters: []redshifttypes.Parameter{
		{ParameterName: aws.String("require_ssl"), ParameterValue: aws.String("false")},
	}}, nil
}

func t578RedshiftCluster(id, status string) redshifttypes.Cluster {
	c := w2RedshiftCluster(id, id+"-params")
	c.ClusterStatus = aws.String(status)
	return c
}

func t578FetchRedshift(t *testing.T, fake *t578RedshiftFake) []resource.Resource {
	t.Helper()
	out, err := BindPage("redshift")(awsclient.FetchRedshiftClustersPage(context.Background(), fake, ""))
	if err != nil {
		t.Fatalf("FetchRedshiftClustersPage: %v", err)
	}
	return out.Resources
}

// docs/resources/redshift.md §5: paused is an admin-quiescent state and reads
// Dim; the other transient statuses read Warning. Either way the row carries a
// state finding, so the Status cell names the state, and the status field
// keeps the value AWS reported.
func TestT578Redshift_TransientStatusesNeverRenderBlank(t *testing.T) {
	for _, tc := range []struct {
		status string
		sev    domain.Severity
	}{
		{"paused", domain.SevDim},
		{"final-snapshot", domain.SevWarn},
		{"rotating-keys", domain.SevWarn},
		{"updating-hsm", domain.SevWarn},
		{"cancelling-resize", domain.SevWarn},
		{"available, prep-for-resize", domain.SevWarn},
		{"available, resize-cleanup", domain.SevWarn},
	} {
		t.Run(tc.status, func(t *testing.T) {
			id := "acme-dwh-" + strings.NewReplacer(", ", "-", " ", "-").Replace(tc.status)
			r := pw1ResourceByID(t, t578FetchRedshift(t, &t578RedshiftFake{clusters: []redshifttypes.Cluster{t578RedshiftCluster(id, tc.status)}}), id)
			if got := r.Fields["status"]; got != tc.status {
				t.Errorf("ClusterStatus %q: Fields[status] = %q, want the AWS value", tc.status, got)
			}
			top, ok := domain.TopFinding(r.Findings)
			if !ok {
				t.Fatalf("ClusterStatus %q carries no finding", tc.status)
			}
			if top.Severity != tc.sev {
				t.Errorf("ClusterStatus %q: top finding %s severity = %v, want %v", tc.status, top.Code, top.Severity, tc.sev)
			}
		})
	}
}

// storage-full, like every failed-but-live status, leaves the endpoint public
// and the data unencrypted; both stay on the row beside the state finding.
func TestT1aRedshift_StorageFullKeepsPostureFindings(t *testing.T) {
	const id = "acme-dwh-full"
	c := t578RedshiftCluster(id, "storage-full")
	c.PubliclyAccessible = aws.Bool(true)
	c.Encrypted = aws.Bool(false)
	r := pw1ResourceByID(t, t578FetchRedshift(t, &t578RedshiftFake{clusters: []redshifttypes.Cluster{c}}), id)
	for _, code := range []domain.FindingCode{awsclient.CodeRedshiftStorageFull, awsclient.CodeRedshiftPubliclyAccessible, awsclient.CodeRedshiftUnencryptedAtRest} {
		if !hasCode(r.Findings, code) {
			t.Errorf("storage-full cluster: missing %s among %v", code, r.Findings)
		}
	}
}

// A status outside the documented list is shown as what AWS said, never as a
// healthy blank row.
func TestT1aRedshift_UnrecognisedStatusIsShown(t *testing.T) {
	const id, raw = "acme-dwh-new", "quiescing"
	r := pw1ResourceByID(t, t578FetchRedshift(t, &t578RedshiftFake{clusters: []redshifttypes.Cluster{t578RedshiftCluster(id, raw)}}), id)
	td := resource.FindResourceType("redshift")
	if td == nil {
		t.Fatal("redshift is not registered")
	}
	if td.UnknownState == "" {
		t.Fatal("redshift declares no code for an unrecognised status")
	}
	var matched []domain.Finding
	for _, f := range r.Findings {
		if f.Code == td.UnknownState {
			matched = append(matched, f)
		}
	}
	if len(matched) != 1 {
		t.Fatalf("want exactly one %s finding, got %v", td.UnknownState, r.Findings)
	}
	f := matched[0]
	if f.Severity != catalog.Severity(td.UnknownState) || f.Source != "wave1" {
		t.Errorf("%s: severity %v source %q, want %v wave1", f.Code, f.Severity, f.Source, catalog.Severity(td.UnknownState))
	}
	if !strings.Contains(f.Phrase, raw) {
		t.Errorf("unrecognised-status phrase %q does not name %q", f.Phrase, raw)
	}
	if got := r.Fields["status"]; got != raw {
		t.Errorf("Fields[status] = %q, want %q", got, raw)
	}
}

// final-snapshot is the last step of a delete: the cluster is about to stop
// existing, so neither its posture fields nor a per-cluster Wave-2 call has
// anything to report. A live sibling is still read and flagged.
func TestT578Redshift_FinalSnapshotIsTeardown(t *testing.T) {
	const gone, live = "acme-dwh-retiring", "acme-dwh-live"
	goneCluster := t578RedshiftCluster(gone, "final-snapshot")
	goneCluster.PubliclyAccessible = aws.Bool(true)
	goneCluster.Encrypted = aws.Bool(false)
	liveCluster := t578RedshiftCluster(live, "available")
	liveCluster.PubliclyAccessible = aws.Bool(true)
	liveCluster.Encrypted = aws.Bool(false)
	fake := &t578RedshiftFake{clusters: []redshifttypes.Cluster{goneCluster, liveCluster}}

	rs := t578FetchRedshift(t, fake)
	g := pw1ResourceByID(t, rs, gone)
	pw1RequireNoFinding(t, g.Findings, awsclient.CodeRedshiftPubliclyAccessible)
	pw1RequireNoFinding(t, g.Findings, awsclient.CodeRedshiftUnencryptedAtRest)
	if len(g.Findings) == 0 {
		t.Errorf("final-snapshot cluster carries no state finding")
	}
	l := pw1ResourceByID(t, rs, live)
	if !hasCode(l.Findings, awsclient.CodeRedshiftPubliclyAccessible) || !hasCode(l.Findings, awsclient.CodeRedshiftUnencryptedAtRest) {
		t.Errorf("available cluster findings = %v, want public and unencrypted", l.Findings)
	}

	res := ProbeWave2(t, "redshift", &awsclient.ServiceClients{Redshift: fake}, rs)
	if res.Err != nil {
		t.Fatalf("redshift Wave 2: %v", res.Err)
	}
	if slices.Contains(fake.logCalls, gone) {
		t.Errorf("DescribeLoggingStatus called for the final-snapshot cluster; calls = %v", fake.logCalls)
	}
	if slices.Contains(fake.paramCalls, gone+"-params") {
		t.Errorf("DescribeClusterParameters called for the final-snapshot cluster's group; calls = %v", fake.paramCalls)
	}
	if len(res.Findings[gone]) != 0 {
		t.Errorf("final-snapshot cluster Wave-2 findings = %v, want none", res.Findings[gone])
	}
	if _, marked := res.TruncatedIDs[gone]; marked {
		t.Errorf("final-snapshot cluster marked not inspected (%q); skipping it is the answer", res.TruncatedIDs[gone])
	}
	if !slices.Contains(fake.logCalls, live) || !hasCode(res.Findings[live], "redshift.audit-logging-off") {
		t.Errorf("live cluster: logging calls %v, findings %v; want it read and flagged", fake.logCalls, res.Findings[live])
	}
}

// ---- eb: ended environments are not asked for health; live ones by id ----

type t578EBFake struct {
	awsclient.ElasticBeanstalkAPI
	envs           []ebtypes.EnvironmentDescription
	causesByID     map[string][]string
	causesByName   map[string][]string
	mu             sync.Mutex
	healthRequests []elasticbeanstalk.DescribeEnvironmentHealthInput
}

func (f *t578EBFake) DescribeEnvironments(_ context.Context, _ *elasticbeanstalk.DescribeEnvironmentsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
	return &elasticbeanstalk.DescribeEnvironmentsOutput{Environments: f.envs}, nil
}

// DescribeEnvironmentHealth by name resolves to the environment currently
// holding that name, which is the live one, never a terminated namesake.
func (f *t578EBFake) DescribeEnvironmentHealth(_ context.Context, in *elasticbeanstalk.DescribeEnvironmentHealthInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentHealthOutput, error) {
	f.mu.Lock()
	f.healthRequests = append(f.healthRequests, *in)
	f.mu.Unlock()
	causes := f.causesByName[aws.ToString(in.EnvironmentName)]
	if id := aws.ToString(in.EnvironmentId); id != "" {
		causes = f.causesByID[id]
	}
	return &elasticbeanstalk.DescribeEnvironmentHealthOutput{Causes: causes, HealthStatus: aws.String("Degraded")}, nil
}

func (f *t578EBFake) DescribeConfigurationSettings(_ context.Context, _ *elasticbeanstalk.DescribeConfigurationSettingsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
	return &elasticbeanstalk.DescribeConfigurationSettingsOutput{}, nil
}

func t578EBEnv(id, name string, status ebtypes.EnvironmentStatus, health ebtypes.EnvironmentHealth) ebtypes.EnvironmentDescription {
	return ebtypes.EnvironmentDescription{
		EnvironmentId:     aws.String(id),
		EnvironmentName:   aws.String(name),
		ApplicationName:   aws.String("acme-checkout-app"),
		EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/acme-checkout-app/" + name),
		Status:            status,
		Health:            health,
		SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.3.0 running Python 3.12"),
		DateCreated:       aws.Time(time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)),
	}
}

func TestT578EB_EndedEnvironmentsSkipHealthAndLiveOnesAreAddressedByID(t *testing.T) {
	const (
		oldID, liveID, goingID = "e-acmeold0001", "e-acmelive002", "e-acmegone003"
		sharedName             = "acme-checkout-prod"
	)
	liveCause := "Incorrect application version found on all instances. Expected version \"v42\"."
	fake := &t578EBFake{
		envs: []ebtypes.EnvironmentDescription{
			t578EBEnv(oldID, sharedName, ebtypes.EnvironmentStatusTerminated, ebtypes.EnvironmentHealthGrey),
			t578EBEnv(liveID, sharedName, ebtypes.EnvironmentStatusReady, ebtypes.EnvironmentHealthYellow),
			t578EBEnv(goingID, "acme-batch-old", ebtypes.EnvironmentStatusTerminating, ebtypes.EnvironmentHealthGrey),
		},
		causesByID:   map[string][]string{liveID: {liveCause}},
		causesByName: map[string][]string{sharedName: {liveCause}, "acme-batch-old": {"None of the instances are sending data."}},
	}
	out, err := BindPage("eb")(awsclient.FetchEBEnvironmentsPage(context.Background(), fake, ""))
	if err != nil {
		t.Fatalf("FetchEBEnvironmentsPage: %v", err)
	}
	probe := ProbeWave2(t, "eb", &awsclient.ServiceClients{ElasticBeanstalk: fake}, out.Resources)
	if probe.Err != nil {
		t.Fatalf("eb Wave 2: %v", probe.Err)
	}
	res := t1aIssueResult(probe)

	var asked []string
	for _, in := range fake.healthRequests {
		asked = append(asked, aws.ToString(in.EnvironmentId)+"|"+aws.ToString(in.EnvironmentName))
		if aws.ToString(in.EnvironmentId) != liveID {
			t.Errorf("DescribeEnvironmentHealth asked with EnvironmentId=%q EnvironmentName=%q; only the live environment %s, by id, may be asked",
				aws.ToString(in.EnvironmentId), aws.ToString(in.EnvironmentName), liveID)
		}
	}
	if len(fake.healthRequests) != 1 {
		t.Errorf("DescribeEnvironmentHealth requests = %v, want exactly one for %s", asked, liveID)
	}
	for _, id := range []string{oldID, goingID} {
		if got := res.Findings[id]; len(got) != 0 {
			t.Errorf("ended environment %s findings = %v, want none", id, got)
		}
		if _, marked := res.TruncatedIDs[id]; marked {
			t.Errorf("ended environment %s marked not inspected (%q)", id, res.TruncatedIDs[id])
		}
	}
	if !hasCode(res.Findings[liveID], "eb.environment-causes") {
		t.Fatalf("live environment %s findings = %v, want eb.environment-causes", liveID, res.Findings[liveID])
	}
	pw1RequireRow(t, pw1Rows(res, liveID, "eb.environment-causes"), "Cause", liveCause)
}

// ---- efs: no "no mount targets" while the file system is mid-lifecycle ----

type t578EFSFake struct {
	fss []efstypes.FileSystemDescription
}

func (f *t578EFSFake) DescribeFileSystems(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
	return &efs.DescribeFileSystemsOutput{FileSystems: f.fss}, nil
}

func t578FileSystem(id string, state efstypes.LifeCycleState, mountTargets int32) efstypes.FileSystemDescription {
	return efstypes.FileSystemDescription{
		FileSystemId:         aws.String(id),
		FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/" + id),
		Name:                 aws.String("acme-shared-" + string(state)),
		CreationToken:        aws.String("acme-" + id),
		OwnerId:              aws.String("123456789012"),
		CreationTime:         aws.Time(time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)),
		LifeCycleState:       state,
		NumberOfMountTargets: mountTargets,
		PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
		ThroughputMode:       efstypes.ThroughputModeElastic,
		Encrypted:            aws.Bool(true),
		SizeInBytes:          &efstypes.FileSystemSize{Value: 6144},
	}
}

// Mount targets are created after the file system reaches available, and are
// removed before it is deleted; zero of them during creating, updating or
// deleting is the lifecycle, not a misconfiguration.
func TestT578EFS_NoMountTargetsOnlyOnAvailable(t *testing.T) {
	for _, tc := range []struct {
		state  efstypes.LifeCycleState
		code   domain.FindingCode
		phrase string
	}{
		{efstypes.LifeCycleStateCreating, awsclient.CodeEFSCreating, "creating"},
		{efstypes.LifeCycleStateUpdating, awsclient.CodeEFSUpdating, "updating"},
		{efstypes.LifeCycleStateDeleting, awsclient.CodeEFSDeleting, "deleting"},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			id := "fs-0t578" + string(tc.state)
			out, err := BindPage("efs")(awsclient.FetchEFSFileSystemsPage(context.Background(), &t578EFSFake{fss: []efstypes.FileSystemDescription{t578FileSystem(id, tc.state, 0)}}, ""))
			if err != nil {
				t.Fatalf("FetchEFSFileSystemsPage: %v", err)
			}
			r := pw1ResourceByID(t, out.Resources, id)
			pw1RequireNoFinding(t, r.Findings, awsclient.CodeEFSNoMountTargets)
			pw1RequireFinding(t, r.Findings, tc.code, tc.phrase, domain.SevWarn, "wave1")
			if got := r.Fields["status"]; got != string(tc.state) {
				t.Errorf("Fields[status] = %q, want the AWS value %q", got, tc.state)
			}
		})
	}

	const id = "fs-0t578available"
	out, err := BindPage("efs")(awsclient.FetchEFSFileSystemsPage(context.Background(), &t578EFSFake{fss: []efstypes.FileSystemDescription{t578FileSystem(id, efstypes.LifeCycleStateAvailable, 0)}}, ""))
	if err != nil {
		t.Fatalf("FetchEFSFileSystemsPage: %v", err)
	}
	r := pw1ResourceByID(t, out.Resources, id)
	pw1RequireFinding(t, r.Findings, awsclient.CodeEFSNoMountTargets, "no mount targets", domain.SevBroken, "wave1")
	if got := r.Fields["status"]; got != "available" {
		t.Errorf("available, zero mount targets: Fields[status] = %q, want the AWS value %q", got, "available")
	}
}

// ---- kms: rotation is read only for keys that can rotate automatically ----

type t1aKMSFake struct {
	awsclient.KMSAPI
	keys []kmstypes.KeyMetadata

	mu            sync.Mutex
	rotationCalls []string
}

func (f *t1aKMSFake) ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	out := &kms.ListKeysOutput{}
	for _, k := range f.keys {
		out.Keys = append(out.Keys, kmstypes.KeyListEntry{KeyId: k.KeyId, KeyArn: k.Arn})
	}
	return out, nil
}

func (f *t1aKMSFake) ListAliases(context.Context, *kms.ListAliasesInput, ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	out := &kms.ListAliasesOutput{}
	for _, k := range f.keys {
		out.Aliases = append(out.Aliases, kmstypes.AliasListEntry{
			AliasName:   aws.String("alias/" + aws.ToString(k.Description)),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/" + aws.ToString(k.Description)),
			TargetKeyId: k.KeyId,
		})
	}
	return out, nil
}

func (f *t1aKMSFake) DescribeKey(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	for i := range f.keys {
		if aws.ToString(f.keys[i].KeyId) == aws.ToString(in.KeyId) {
			meta := f.keys[i]
			return &kms.DescribeKeyOutput{KeyMetadata: &meta}, nil
		}
	}
	return nil, &kmstypes.NotFoundException{Message: aws.String("Key '" + aws.ToString(in.KeyId) + "' does not exist")}
}

func (f *t1aKMSFake) GetKeyRotationStatus(_ context.Context, in *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	f.mu.Lock()
	f.rotationCalls = append(f.rotationCalls, aws.ToString(in.KeyId))
	f.mu.Unlock()
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: false}, nil
}

func (f *t1aKMSFake) GetKeyPolicy(context.Context, *kms.GetKeyPolicyInput, ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	return &kms.GetKeyPolicyOutput{PolicyName: aws.String("default"), Policy: aws.String(w4KMSPrivatePolicyDoc)}, nil
}

func t1aKMSKey(id, name string, state kmstypes.KeyState) kmstypes.KeyMetadata {
	k := kmstypes.KeyMetadata{
		KeyId:                 aws.String(id),
		Arn:                   aws.String("arn:aws:kms:us-east-1:123456789012:key/" + id),
		AWSAccountId:          aws.String("123456789012"),
		Description:           aws.String(name),
		KeyManager:            kmstypes.KeyManagerTypeCustomer,
		KeyState:              state,
		Enabled:               state == kmstypes.KeyStateEnabled,
		KeySpec:               kmstypes.KeySpecSymmetricDefault,
		KeyUsage:              kmstypes.KeyUsageTypeEncryptDecrypt,
		Origin:                kmstypes.OriginTypeAwsKms,
		CreationDate:          aws.Time(time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)),
		EncryptionAlgorithms:  []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
		MultiRegion:           aws.Bool(false),
		CustomerMasterKeySpec: kmstypes.CustomerMasterKeySpecSymmetricDefault,
	}
	if state == kmstypes.KeyStatePendingDeletion || state == kmstypes.KeyStatePendingReplicaDeletion {
		k.DeletionDate = aws.Time(time.Date(2026, 11, 29, 0, 0, 0, 0, time.UTC))
		k.PendingDeletionWindowInDays = aws.Int32(7)
	}
	return k
}

func t1aFetchKMS(t *testing.T, fake *t1aKMSFake) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchKMSKeysPage(context.Background(), &awsclient.ServiceClients{KMS: fake, Region: "us-east-1"}, "")
	if err != nil {
		t.Fatalf("FetchKMSKeysPage: %v", err)
	}
	return out.Resources
}

// GetKeyRotationStatus answers for a key that can rotate automatically: a
// symmetric encryption key with AWS_KMS key material outside a custom key
// store. A key pending deletion will be destroyed in days, so rotation advice
// about it is advice nobody can act on and its rotation is never read. A
// customer key that cannot rotate automatically (asymmetric, imported key
// material, custom key store) has rotation off by construction and is flagged
// without the call.
func TestT578KMS_PendingDeletionKeysRaiseNoRotationFinding(t *testing.T) {
	const (
		enabled  = "1a2b3c4d-5e6f-4a5b-8c9d-0e1f2a3b4c5d"
		pending  = "2b3c4d5e-6f7a-4b5c-9d0e-1f2a3b4c5d6e"
		replica  = "3c4d5e6f-7a8b-4c5d-8e0f-2a3b4c5d6e7f"
		signing  = "4d5e6f7a-8b9c-4d5e-9f0a-3b4c5d6e7f8a"
		imported = "5e6f7a8b-9c0d-4e5f-8a1b-4c5d6e7f8a9b"
		hsm      = "6f7a8b9c-0d1e-4f5a-9b2c-5d6e7f8a9b0c"
	)
	sign := t1aKMSKey(signing, "acme-release-signing", kmstypes.KeyStateEnabled)
	sign.KeySpec, sign.KeyUsage, sign.CustomerMasterKeySpec = kmstypes.KeySpecRsa2048, kmstypes.KeyUsageTypeSignVerify, kmstypes.CustomerMasterKeySpecRsa2048
	sign.EncryptionAlgorithms = nil
	sign.SigningAlgorithms = []kmstypes.SigningAlgorithmSpec{kmstypes.SigningAlgorithmSpecRsassaPssSha256}
	ext := t1aKMSKey(imported, "acme-byok", kmstypes.KeyStateEnabled)
	ext.Origin = kmstypes.OriginTypeExternal
	ext.ExpirationModel = kmstypes.ExpirationModelTypeKeyMaterialDoesNotExpire
	cks := t1aKMSKey(hsm, "acme-hsm-backed", kmstypes.KeyStateEnabled)
	cks.Origin = kmstypes.OriginTypeAwsCloudhsm
	cks.CustomKeyStoreId = aws.String("cks-1234567890abcdef0")
	cks.CloudHsmClusterId = aws.String("cluster-1a2b3c4d5e6")
	rep := t1aKMSKey(replica, "acme-replica", kmstypes.KeyStatePendingReplicaDeletion)
	rep.MultiRegion = aws.Bool(true)

	fake := &t1aKMSFake{keys: []kmstypes.KeyMetadata{
		t1aKMSKey(enabled, "acme-app-data", kmstypes.KeyStateEnabled),
		t1aKMSKey(pending, "acme-retired", kmstypes.KeyStatePendingDeletion),
		rep, sign, ext, cks,
	}}
	rows := t1aFetchKMS(t, fake)
	res := ProbeWave2(t, "kms", &awsclient.ServiceClients{KMS: fake, Region: "us-east-1"}, rows)
	if res.Err != nil {
		t.Fatalf("kms Wave 2: %v", res.Err)
	}

	if !slices.Equal(fake.rotationCalls, []string{enabled}) {
		t.Errorf("GetKeyRotationStatus asked for %v, want only the rotatable key %s", fake.rotationCalls, enabled)
	}
	for _, id := range []string{enabled, signing, imported, hsm} {
		if !hasCode(res.Findings[id], "kms.rotation-disabled") {
			t.Errorf("key %s findings = %v, want kms.rotation-disabled", id, res.Findings[id])
		}
	}
	for _, id := range []string{pending, replica} {
		if hasCode(res.Findings[id], "kms.rotation-disabled") {
			t.Errorf("key %s pending deletion carries kms.rotation-disabled", id)
		}
		if _, unknown := res.TruncatedIDs[id]; unknown {
			t.Errorf("key %s pending deletion is marked not inspected (%q); its answer is complete", id, res.TruncatedIDs[id])
		}
		r := pw1ResourceByID(t, rows, id)
		if !hasCode(r.Findings, awsclient.CodeKMSStatePendingDeletion) {
			t.Errorf("key %s findings = %v, want %s", id, r.Findings, awsclient.CodeKMSStatePendingDeletion)
		}
	}
	for _, s := range []kmstypes.KeyState{kmstypes.KeyStatePendingDeletion, kmstypes.KeyStatePendingReplicaDeletion} {
		if got := awsclient.LifecycleClassOf("kms", string(s)); got != catalog.LifecyclePendingDeletion {
			t.Errorf("kms %s class = %v, want LifecyclePendingDeletion", s, got)
		}
	}
}

// A key being created (a multi-Region replica, a custom-key-store key) or
// updated becomes usable on its own; it is a transitional state, not a broken
// key, so it reads at the transitional class's severity.
func TestT1aKMS_CreatingAndUpdatingAreTransitional(t *testing.T) {
	const creating, updating = "7a8b9c0d-1e2f-4a5b-8c3d-6e7f8a9b0c1d", "8b9c0d1e-2f3a-4b5c-9d4e-7f8a9b0c1d2e"
	c := t1aKMSKey(creating, "acme-eu-replica", kmstypes.KeyStateCreating)
	c.MultiRegion = aws.Bool(true)
	rows := t1aFetchKMS(t, &t1aKMSFake{keys: []kmstypes.KeyMetadata{c, t1aKMSKey(updating, "acme-ledger", kmstypes.KeyStateUpdating)}})
	want := catalog.LifecycleTransitional.DefaultSeverity()
	if want == domain.SevBroken {
		t.Fatalf("the transitional class defaults to Broken")
	}
	for _, tc := range []struct {
		id    string
		state kmstypes.KeyState
	}{{creating, kmstypes.KeyStateCreating}, {updating, kmstypes.KeyStateUpdating}} {
		if got := awsclient.LifecycleClassOf("kms", string(tc.state)); got != catalog.LifecycleTransitional {
			t.Errorf("kms %s class = %v, want LifecycleTransitional", tc.state, got)
		}
		r := pw1ResourceByID(t, rows, tc.id)
		top, ok := domain.TopFinding(r.Findings)
		if !ok {
			t.Fatalf("kms %s key carries no finding", tc.state)
		}
		if top.Severity != want {
			t.Errorf("kms %s: %s severity = %v, want the transitional default %v", tc.state, top.Code, top.Severity, want)
		}
		if top.Code == awsclient.CodeKMSStateUnavailable {
			t.Errorf("kms %s raises %s, the unavailable-key finding", tc.state, top.Code)
		}
		if got := r.Fields["status"]; got != string(tc.state) {
			t.Errorf("kms %s: Fields[status] = %q, want the AWS value", tc.state, got)
		}
	}
}

// ---- ddb: every TableStatus is a visible state ----

// REPLICATION_NOT_AUTHORIZED is one of the values DescribeTable reports in
// TableStatus; a table in it is not the ACTIVE table a blank row claims.
func TestT1aDDB_ReplicationNotAuthorizedIsAVisibleState(t *testing.T) {
	const name, raw = "acme-global-orders", "REPLICATION_NOT_AUTHORIZED"
	tb := w2DDBTable(name)
	tb.TableStatus = ddbtypes.TableStatus(raw)
	r := w2DDBFetch(t, tb)[name]
	td := resource.FindResourceType("ddb")
	if td == nil {
		t.Fatal("ddb is not registered")
	}
	_, code := catalog.Lifecycle(*td, raw)
	if code == "" {
		t.Fatalf("the ddb state table raises no finding for %s", raw)
	}
	f := pw1RequireFinding(t, r.Findings, code, catalog.Phrase(code), catalog.Severity(code), "wave1")
	if f.Phrase == "" {
		t.Errorf("%s finding has an empty phrase", code)
	}
	if got := r.Fields["status"]; got != raw {
		t.Errorf("Fields[status] = %q, want the AWS value %q", got, raw)
	}

	healthy := w2DDBFetch(t, w2DDBTable("acme-billing"))["acme-billing"]
	if len(healthy.Findings) != 0 {
		t.Errorf("ACTIVE, protected table carries %v, want none", healthy.Findings)
	}
}
