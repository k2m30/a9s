package unit

// d3_enrichers_test.go — rows 5, 6, 7 and 14: a paginated walk that throws
// away what it collected, a per-group read serialised behind the caller's
// lock, an event scan that assumes an order the API does not promise, and a
// "not found" that means "there is none" being read as "we could not look".

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	d3CodeDBIMaintenance  domain.FindingCode = "dbi.pending-maintenance"
	d3CodeRedshiftSSLOff  domain.FindingCode = "redshift.require-ssl-off"
	d3CodeECSDeployFailed domain.FindingCode = "ecs-svc.deployment-failed"
	d3CodeLambdaPublic    domain.FindingCode = "lambda.public-policy"
)

// --- row 5: a page error must not discard the pages already read ------------

// d3RDSFake serves DescribePendingMaintenanceActions as two pages, the second
// of which fails — the shape of a large account whose walk is interrupted.
type d3RDSFake struct {
	awsclient.RDSAPI
	page1 []rdstypes.ResourcePendingMaintenanceActions
}

func (f *d3RDSFake) DescribePendingMaintenanceActions(
	_ context.Context, in *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options),
) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	if in != nil && in.Marker != nil {
		return nil, errors.New("Throttling: rate exceeded")
	}
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: f.page1,
		Marker:                    aws.String("page-2"),
	}, nil
}

var _ awsclient.RDSAPI = (*d3RDSFake)(nil)

// TestD3MaintenanceWalkKeepsThePagesItRead pins row 5: an error on the second
// page loses the instances that page would have named, not the ones already
// read. Returning early throws away a page of true findings because a later
// page failed.
func TestD3MaintenanceWalkKeepsThePagesItRead(t *testing.T) {
	const instance = "acme-orders-db"
	fake := &d3RDSFake{page1: []rdstypes.ResourcePendingMaintenanceActions{{
		ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:123456789012:db:" + instance),
		PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{{
			Action:               aws.String("system-update"),
			Description:          aws.String("New Operating System patch is available"),
			AutoAppliedAfterDate: aws.Time(time.Now().Add(72 * time.Hour)),
			OptInStatus:          aws.String("next-maintenance"),
		}},
	}}}

	res, _ := awsclient.EnrichDBIMaintenance(context.Background(),
		&awsclient.ServiceClients{RDS: fake, Region: "us-east-1"},
		[]resource.Resource{{ID: instance, Name: instance, Type: "dbi"}}, nil)

	w4AssertFinding(t, res.Findings[instance], d3CodeDBIMaintenance,
		"maintenance scheduled", domain.SevWarn, "wave2:dbi")
	if !res.Truncated {
		t.Errorf("Truncated = false; a walk that stopped early has not seen every instance")
	}
}

// --- row 6: the per-group read must not serialise the whole pass ------------

// d3RedshiftFake blocks inside DescribeClusterParameters until two calls are
// in flight at once. A pass that reads parameter groups under the caller's
// mutex can never satisfy that, so the barrier times out.
type d3RedshiftFake struct {
	awsclient.RedshiftAPI
	mu       sync.Mutex
	inFlight int
	calls    map[string]int
	overlap  chan struct{}
	timedOut bool
	once     sync.Once
}

func (f *d3RedshiftFake) DescribeLoggingStatus(
	_ context.Context, in *redshift.DescribeLoggingStatusInput, _ ...func(*redshift.Options),
) (*redshift.DescribeLoggingStatusOutput, error) {
	_ = in
	return &redshift.DescribeLoggingStatusOutput{LoggingEnabled: aws.Bool(true)}, nil
}

func (f *d3RedshiftFake) DescribeClusterParameters(
	_ context.Context, in *redshift.DescribeClusterParametersInput, _ ...func(*redshift.Options),
) (*redshift.DescribeClusterParametersOutput, error) {
	name := aws.ToString(in.ParameterGroupName)
	f.mu.Lock()
	f.calls[name]++
	f.inFlight++
	reached := f.inFlight >= 2
	f.mu.Unlock()

	if reached {
		f.once.Do(func() { close(f.overlap) })
	}
	select {
	case <-f.overlap:
	case <-time.After(2 * time.Second):
		f.mu.Lock()
		f.timedOut = true
		f.mu.Unlock()
	}

	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()
	return &redshift.DescribeClusterParametersOutput{Parameters: []redshifttypes.Parameter{{
		ParameterName: aws.String("require_ssl"), ParameterValue: aws.String("false"),
	}}}, nil
}

var _ awsclient.RedshiftAPI = (*d3RedshiftFake)(nil)

func d3RedshiftCluster(id, group string) resource.Resource {
	return resource.Resource{
		ID: id, Name: id, Type: "redshift",
		Fields: map[string]string{"cluster_identifier": id, "status": "available"},
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String(id),
			ClusterStatus:     aws.String("available"),
			ClusterParameterGroups: []redshifttypes.ClusterParameterGroupStatus{{
				ParameterGroupName: aws.String(group),
			}},
		},
	}
}

// TestD3RedshiftParameterGroupsAreReadConcurrently pins row 6: two clusters on
// two parameter groups read them at the same time. Holding the result mutex
// across the API call turns a parallel pass into a serial one, and the cost
// grows with the cluster count.
func TestD3RedshiftParameterGroupsAreReadConcurrently(t *testing.T) {
	fake := &d3RedshiftFake{calls: map[string]int{}, overlap: make(chan struct{})}
	_, _ = awsclient.EnrichRedshiftPosture(context.Background(),
		&awsclient.ServiceClients{Redshift: fake, Region: "us-east-1"},
		[]resource.Resource{
			d3RedshiftCluster("acme-warehouse", "acme-params-a"),
			d3RedshiftCluster("acme-reporting", "acme-params-b"),
		}, nil)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.timedOut {
		t.Errorf("the two parameter-group reads never overlapped: the pass is serialised behind the caller's lock")
	}
}

// TestD3RedshiftParameterGroupReadOncePerGroup pins the other half of row 6:
// deduplication by group survives whatever makes the reads concurrent. Two
// clusters sharing a group cost one call, not two.
func TestD3RedshiftParameterGroupReadOncePerGroup(t *testing.T) {
	fake := &d3RedshiftFake{calls: map[string]int{}, overlap: make(chan struct{})}
	close(fake.overlap) // no barrier here; this test counts calls only

	res, _ := awsclient.EnrichRedshiftPosture(context.Background(),
		&awsclient.ServiceClients{Redshift: fake, Region: "us-east-1"},
		[]resource.Resource{
			d3RedshiftCluster("acme-warehouse", "acme-shared-params"),
			d3RedshiftCluster("acme-reporting", "acme-shared-params"),
		}, nil)

	fake.mu.Lock()
	got := fake.calls["acme-shared-params"]
	fake.mu.Unlock()
	if got != 1 {
		t.Errorf("DescribeClusterParameters called %d times for one group, want 1", got)
	}
	for _, id := range []string{"acme-warehouse", "acme-reporting"} {
		w4AssertFinding(t, res.Findings[id], d3CodeRedshiftSSLOff,
			"SSL not required", domain.SevWarn, "wave2:redshift")
	}
}

// --- row 7: the event scan must not assume an order -------------------------

type d3ECSFake struct {
	awsclient.ECSAPI
	services []ecstypes.Service
}

func (f *d3ECSFake) DescribeServices(
	_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options),
) (*ecs.DescribeServicesOutput, error) {
	return &ecs.DescribeServicesOutput{Services: f.services}, nil
}

var _ awsclient.ECSAPI = (*d3ECSFake)(nil)

// TestD3ECSRecentEventFoundOutOfOrder pins row 7: a placement failure inside
// the recent window is reported wherever it sits in the list. Stopping at the
// first old event only works if the API promises newest-first, and nothing in
// the code cites that promise.
func TestD3ECSRecentEventFoundOutOfOrder(t *testing.T) {
	const svc = "acme-checkout-svc"
	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now().Add(-1 * time.Minute)
	fake := &d3ECSFake{services: []ecstypes.Service{{
		ServiceName:  aws.String(svc),
		Status:       aws.String("ACTIVE"),
		DesiredCount: 2,
		RunningCount: 2,
		Events: []ecstypes.ServiceEvent{
			{CreatedAt: &old, Message: aws.String("(service acme-checkout-svc) has reached a steady state.")},
			{CreatedAt: &recent, Message: aws.String("(service acme-checkout-svc) was unable to place a task because no container instance met all of its requirements.")},
		},
	}}}

	res, _ := awsclient.EnrichECSServices(context.Background(),
		&awsclient.ServiceClients{ECS: fake, Region: "us-east-1"},
		[]resource.Resource{{
			ID: svc, Name: svc, Type: "ecs-svc",
			Fields: map[string]string{"cluster": "acme-prod", "service_name": svc},
		}}, nil)

	w4AssertFinding(t, res.Findings[svc], d3CodeECSDeployFailed,
		"unable to place task", domain.SevBroken, "wave2:ecs-svc")
}

// TestD3ECSQuietServiceReportsNothing pins the negative case: a service whose
// only recent event is a steady state is healthy, so relaxing the scan must
// not start reporting old noise.
func TestD3ECSQuietServiceReportsNothing(t *testing.T) {
	const svc = "acme-quiet-svc"
	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now().Add(-1 * time.Minute)
	fake := &d3ECSFake{services: []ecstypes.Service{{
		ServiceName:  aws.String(svc),
		Status:       aws.String("ACTIVE"),
		DesiredCount: 2,
		RunningCount: 2,
		Events: []ecstypes.ServiceEvent{
			{CreatedAt: &recent, Message: aws.String("(service acme-quiet-svc) has reached a steady state.")},
			{CreatedAt: &old, Message: aws.String("(service acme-quiet-svc) was unable to place a task because no container instance met all of its requirements.")},
		},
	}}}

	res, _ := awsclient.EnrichECSServices(context.Background(),
		&awsclient.ServiceClients{ECS: fake, Region: "us-east-1"},
		[]resource.Resource{{
			ID: svc, Name: svc, Type: "ecs-svc",
			Fields: map[string]string{"cluster": "acme-prod", "service_name": svc},
		}}, nil)

	w4AssertNoCode(t, res.Findings[svc], d3CodeECSDeployFailed)
}

// --- row 14: "there is none" is an answer -----------------------------------

type d3LambdaFake struct {
	awsclient.LambdaAPI
	policies map[string]string
}

func (f *d3LambdaFake) GetPolicy(
	_ context.Context, in *lambda.GetPolicyInput, _ ...func(*lambda.Options),
) (*lambda.GetPolicyOutput, error) {
	doc, ok := f.policies[aws.ToString(in.FunctionName)]
	if !ok {
		return nil, &lambdatypes.ResourceNotFoundException{
			Message: aws.String("The resource you requested does not exist."),
		}
	}
	return &lambda.GetPolicyOutput{Policy: aws.String(doc)}, nil
}

// A function with no URL configured returns an empty list, not an error.
func (f *d3LambdaFake) ListFunctionUrlConfigs(
	_ context.Context, _ *lambda.ListFunctionUrlConfigsInput, _ ...func(*lambda.Options),
) (*lambda.ListFunctionUrlConfigsOutput, error) {
	return &lambda.ListFunctionUrlConfigsOutput{}, nil
}

var _ awsclient.LambdaAPI = (*d3LambdaFake)(nil)

const d3LambdaPublicPolicy = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowAnyone",` +
	`"Effect":"Allow","Principal":"*","Action":"lambda:InvokeFunction",` +
	`"Resource":"arn:aws:lambda:us-east-1:123456789012:function:acme-open-fn"}]}`

func d3LambdaResource(name string) resource.Resource {
	return resource.Resource{
		ID: name, Name: name, Type: "lambda",
		Fields: map[string]string{"function_name": name, "runtime": "python3.12"},
	}
}

// TestD3LambdaWithoutAPolicyIsNotUnknown pins the one place where "there is
// none" and "we could not look" are told apart by something other than the
// shared not-found predicate: GetPolicy answers ResourceNotFoundException for
// a function that simply has no resource policy, and that is a clean verdict.
// Folding the check into IsNotFoundErr would put a "?" on every private
// function in the account.
func TestD3LambdaWithoutAPolicyIsNotUnknown(t *testing.T) {
	fake := &d3LambdaFake{policies: map[string]string{}}
	res, _ := awsclient.EnrichLambdaPosture(context.Background(),
		&awsclient.ServiceClients{Lambda: fake, Region: "us-east-1"},
		[]resource.Resource{d3LambdaResource("acme-private-fn")}, nil)

	if res.TruncatedIDs["acme-private-fn"] {
		t.Errorf("a function with no resource policy was marked unknown; having none is an answer")
	}
	w4AssertNoCode(t, res.Findings["acme-private-fn"], d3CodeLambdaPublic)
}

// TestD3LambdaWithAPublicPolicyStillReports pins the positive case beside it:
// treating not-found as an answer must not stop the open policy being found.
func TestD3LambdaWithAPublicPolicyStillReports(t *testing.T) {
	fake := &d3LambdaFake{policies: map[string]string{"acme-open-fn": d3LambdaPublicPolicy}}
	res, _ := awsclient.EnrichLambdaPosture(context.Background(),
		&awsclient.ServiceClients{Lambda: fake, Region: "us-east-1"},
		[]resource.Resource{d3LambdaResource("acme-open-fn")}, nil)

	w4AssertFinding(t, res.Findings["acme-open-fn"], d3CodeLambdaPublic,
		"invokable by anyone", domain.SevBroken, "wave2:lambda")
}

// d3NotFoundErr is the shape a service returns when the thing asked about
// does not exist, as opposed to a call that could not be made.
func d3NotFoundErr() error {
	return &smithy.GenericAPIError{
		Code: "ResourceNotFoundException", Message: "not found", Fault: smithy.FaultClient,
	}
}

var _ = d3NotFoundErr

// d3RDSFirstPageFails answers every page with an error, the shape of a
// throttled account where nothing was collected at all.
type d3RDSFirstPageFails struct {
	awsclient.RDSAPI
}

func (f *d3RDSFirstPageFails) DescribePendingMaintenanceActions(
	_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options),
) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return nil, errors.New("Throttling: rate exceeded")
}

var _ awsclient.RDSAPI = (*d3RDSFirstPageFails)(nil)

// TestD3MaintenanceFirstPageFailureIsNotClean attacks row 5 from the other
// side: keeping the pages already read must not turn a walk that read nothing
// into a clean verdict. No pages means no coverage, and the pass says so.
func TestD3MaintenanceFirstPageFailureIsNotClean(t *testing.T) {
	const instance = "acme-orders-db"
	res, err := awsclient.EnrichDBIMaintenance(context.Background(),
		&awsclient.ServiceClients{RDS: &d3RDSFirstPageFails{}, Region: "us-east-1"},
		[]resource.Resource{{ID: instance, Name: instance, Type: "dbi"}}, nil)

	if !res.Truncated && err == nil {
		t.Errorf("a walk that read no pages reported neither truncation nor an error")
	}
	w4AssertNoCode(t, res.Findings[instance], d3CodeDBIMaintenance)
}

// TestD3EveryKeyFailingMarksTheUserOnce attacks row 1: a user whose keys all
// fail to read is unknown once, not once per key, and still carries no
// finding.
func TestD3EveryKeyFailingMarksTheUserOnce(t *testing.T) {
	fake := &d3UserFake{
		withMFA: map[string]bool{"acme-batch-user": true},
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-batch-user": {
				d3Key("acme-batch-user", "AKIAIOSFODNN7EXAMPLE", 10),
				d3Key("acme-batch-user", "AKIAI44QH8DHBEXAMPLE", 10),
			},
		},
		lastUsedErr: map[string]error{
			"AKIAIOSFODNN7EXAMPLE": errors.New("Throttling: rate exceeded"),
			"AKIAI44QH8DHBEXAMPLE": errors.New("Throttling: rate exceeded"),
		},
	}
	res := d3EnrichUsers(t, fake, []resource.Resource{d3UserResource("acme-batch-user")})

	if !res.TruncatedIDs["acme-batch-user"] {
		t.Errorf("TruncatedIDs[acme-batch-user] = false; no key could be read")
	}
	w4AssertNoCode(t, res.Findings["acme-batch-user"], d3CodeUserKeyUnused)
}
