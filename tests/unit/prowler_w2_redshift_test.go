package unit

// prowler_w2_redshift_test.go — redshift rows 25–26 of the w2 Prowler batch:
// audit logging off and SSL not required.
//
// Both are driven through EnrichRedshiftPosture, the enricher the batch adds
// for redshift. Audit logging is specified as a Wave-1 row on the premise that
// the fetcher already calls DescribeLoggingStatus — it does not; only the
// redshift→s3 related checker does (core/aws/redshift_related.go). Placing the
// row in the enricher is the only implementation that does not add an API call
// to a fetcher signature every caller shares, so that is what these tests pin.
// If the round rules the Wave-1 placement in, the Source assertion here is the
// line to change, and only that line.

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w2RedshiftCodeAuditLoggingOff = "redshift.audit-logging-off"
	w2RedshiftCodeRequireSSLOff   = "redshift.require-ssl-off"
	w2RedshiftSource              = "wave2:redshift"
)

// w2RedshiftFake answers both DescribeLoggingStatus (per cluster) and
// DescribeClusterParameters (per parameter group). Absent entries are healthy:
// logging on, require_ssl true.
type w2RedshiftFake struct {
	awsclient.RedshiftAPI

	loggingOff  map[string]bool
	requireSSL  map[string]string // parameter group name → require_ssl value
	clusterErrs map[string]error
	groupErrs   map[string]error

	// The enricher fans this fake out through ForEachParallel, so every
	// recorded call is written from a different goroutine.
	mu         sync.Mutex
	paramCalls map[string]int
}

func (f *w2RedshiftFake) DescribeLoggingStatus(_ context.Context, in *redshift.DescribeLoggingStatusInput, _ ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error) {
	id := aws.ToString(in.ClusterIdentifier)
	if err := f.clusterErrs[id]; err != nil {
		return nil, err
	}
	if f.loggingOff[id] {
		return &redshift.DescribeLoggingStatusOutput{LoggingEnabled: aws.Bool(false)}, nil
	}
	return &redshift.DescribeLoggingStatusOutput{
		LoggingEnabled: aws.Bool(true),
		BucketName:     aws.String("acme-redshift-audit"),
	}, nil
}

func (f *w2RedshiftFake) DescribeClusterParameters(_ context.Context, in *redshift.DescribeClusterParametersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClusterParametersOutput, error) {
	g := aws.ToString(in.ParameterGroupName)
	f.mu.Lock()
	if f.paramCalls == nil {
		f.paramCalls = map[string]int{}
	}
	f.paramCalls[g]++
	f.mu.Unlock()
	if err := f.groupErrs[g]; err != nil {
		return nil, err
	}
	v, ok := f.requireSSL[g]
	if !ok {
		v = "true"
	}
	return &redshift.DescribeClusterParametersOutput{
		Parameters: []redshifttypes.Parameter{
			{ParameterName: aws.String("enable_user_activity_logging"), ParameterValue: aws.String("true")},
			{ParameterName: aws.String("require_ssl"), ParameterValue: aws.String(v)},
		},
	}, nil
}

func w2RedshiftCluster(id, paramGroup string) redshifttypes.Cluster {
	return redshifttypes.Cluster{
		ClusterIdentifier:         aws.String(id),
		ClusterStatus:             aws.String("available"),
		ClusterAvailabilityStatus: aws.String("Available"),
		NodeType:                  aws.String("ra3.xlplus"),
		NumberOfNodes:             aws.Int32(2),
		DBName:                    aws.String("analytics"),
		MasterUsername:            aws.String("acme_ops"),
		Encrypted:                 aws.Bool(true),
		PubliclyAccessible:        aws.Bool(false),
		Endpoint:                  &redshifttypes.Endpoint{Address: aws.String(id + ".redshift.amazonaws.com"), Port: aws.Int32(5439)},
		ClusterParameterGroups: []redshifttypes.ClusterParameterGroupStatus{
			{ParameterGroupName: aws.String(paramGroup), ParameterApplyStatus: aws.String("in-sync")},
		},
	}
}

func w2RedshiftEnrich(t *testing.T, fake *w2RedshiftFake, clusters ...redshifttypes.Cluster) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w2RedshiftRun(t, fake, clusters...)
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}
	return res
}

func w2RedshiftRun(t *testing.T, fake *w2RedshiftFake, clusters ...redshifttypes.Cluster) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	rs := make([]resource.Resource, 0, len(clusters))
	for _, c := range clusters {
		rs = append(rs, w2Res(aws.ToString(c.ClusterIdentifier), c))
	}
	res, err := w2Enricher(t, "redshift")(context.Background(), &awsclient.ServiceClients{Redshift: fake}, rs, nil)
	w2AssertEnricherShape(t, res)
	return res, err
}

// ---------------------------------------------------------------------------
// row 25 — audit logging
// ---------------------------------------------------------------------------

func TestW2RedshiftAuditLoggingOff(t *testing.T) {
	fake := &w2RedshiftFake{loggingOff: map[string]bool{"acme-reporting": true}}
	res := w2RedshiftEnrich(t, fake,
		w2RedshiftCluster("acme-reporting", "acme-params"),
		w2RedshiftCluster("acme-analytics", "acme-params"),
	)

	w2AssertFinding(t, res.Findings["acme-reporting"], w2RedshiftCodeAuditLoggingOff, "audit logging off", domain.SevWarn, w2RedshiftSource)
	w2AssertNoRows(t, res, "acme-reporting", w2RedshiftCodeAuditLoggingOff)
	w2AssertNoCode(t, res.Findings["acme-analytics"], w2RedshiftCodeAuditLoggingOff)
	w2AssertFindingDef(t, "redshift", w2RedshiftCodeAuditLoggingOff, "audit logging off", domain.SevWarn, "wave2")
}

// ---------------------------------------------------------------------------
// row 26 — require_ssl
// ---------------------------------------------------------------------------

func TestW2RedshiftRequireSSLOff(t *testing.T) {
	fake := &w2RedshiftFake{requireSSL: map[string]string{"acme-params-open": "false"}}
	res := w2RedshiftEnrich(t, fake,
		w2RedshiftCluster("acme-reporting", "acme-params-open"),
		w2RedshiftCluster("acme-analytics", "acme-params"),
	)

	w2AssertFinding(t, res.Findings["acme-reporting"], w2RedshiftCodeRequireSSLOff, "SSL not required", domain.SevWarn, w2RedshiftSource)
	// d4 row 20: the parameter identifier still rides as the aside, but the
	// value in front of it is a word. Do not restore "false (require_ssl)" —
	// TestNetworkingRowValues_AreWordsNotLiterals fails on it.
	w2AssertRow(t, w2Rows(t, res, "acme-reporting", w2RedshiftCodeRequireSSLOff), "Requires encrypted connections", "off (require_ssl)")
	w2AssertNoCode(t, res.Findings["acme-analytics"], w2RedshiftCodeRequireSSLOff)
	w2AssertFindingDef(t, "redshift", w2RedshiftCodeRequireSSLOff, "SSL not required", domain.SevWarn, "wave2")
}

// Redshift stores the parameter as a lowercase string. Anything that is not
// "true" leaves plaintext connections accepted, including an empty default.
func TestW2RedshiftRequireSSLNonTrueValues(t *testing.T) {
	for _, v := range []string{"false", "", "0"} {
		fake := &w2RedshiftFake{requireSSL: map[string]string{"acme-params-open": v}}
		res := w2RedshiftEnrich(t, fake, w2RedshiftCluster("acme-reporting", "acme-params-open"))
		w2AssertFinding(t, res.Findings["acme-reporting"], w2RedshiftCodeRequireSSLOff, "SSL not required", domain.SevWarn, w2RedshiftSource)
	}
}

// Twenty clusters on one parameter group must cost one DescribeClusterParameters
// call, not twenty — the row is only affordable per distinct group.
func TestW2RedshiftParameterGroupLookupCachedPerGroup(t *testing.T) {
	var clusters []redshifttypes.Cluster
	for i := range 20 {
		clusters = append(clusters, w2RedshiftCluster("acme-shard-"+strconv.Itoa(i), "acme-params-open"))
	}
	fake := &w2RedshiftFake{requireSSL: map[string]string{"acme-params-open": "false"}}
	res := w2RedshiftEnrich(t, fake, clusters...)

	if got := fake.paramCalls["acme-params-open"]; got != 1 {
		t.Errorf("DescribeClusterParameters called %d times for one parameter group; want 1", got)
	}
	for _, c := range clusters {
		id := aws.ToString(c.ClusterIdentifier)
		w2AssertFinding(t, res.Findings[id], w2RedshiftCodeRequireSSLOff, "SSL not required", domain.SevWarn, w2RedshiftSource)
	}
}

// ---------------------------------------------------------------------------
// error handling, independence, caps
// ---------------------------------------------------------------------------

func TestW2RedshiftBothConditionsOnOneCluster(t *testing.T) {
	fake := &w2RedshiftFake{
		loggingOff: map[string]bool{"acme-reporting": true},
		requireSSL: map[string]string{"acme-params-open": "false"},
	}
	res := w2RedshiftEnrich(t, fake, w2RedshiftCluster("acme-reporting", "acme-params-open"))

	w2AssertFinding(t, res.Findings["acme-reporting"], w2RedshiftCodeAuditLoggingOff, "audit logging off", domain.SevWarn, w2RedshiftSource)
	w2AssertFinding(t, res.Findings["acme-reporting"], w2RedshiftCodeRequireSSLOff, "SSL not required", domain.SevWarn, w2RedshiftSource)
	w2AssertNoRows(t, res, "acme-reporting", w2RedshiftCodeAuditLoggingOff)
	// d4 row 20: the parameter identifier still rides as the aside, but the
	// value in front of it is a word. Do not restore "false (require_ssl)" —
	// TestNetworkingRowValues_AreWordsNotLiterals fails on it.
	w2AssertRow(t, w2Rows(t, res, "acme-reporting", w2RedshiftCodeRequireSSLOff), "Requires encrypted connections", "off (require_ssl)")
}

func TestW2RedshiftErrorOnOneClusterKeepsTheRest(t *testing.T) {
	fake := &w2RedshiftFake{
		loggingOff: map[string]bool{"acme-reporting": true},
		clusterErrs: map[string]error{
			"acme-denied": &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"},
		},
	}
	res, err := w2RedshiftRun(t, fake,
		w2RedshiftCluster("acme-denied", "acme-params"),
		w2RedshiftCluster("acme-reporting", "acme-params"),
	)
	if err == nil {
		t.Error("a denied logging-status read was not folded into the composite error")
	}

	if !res.TruncatedIDs["acme-denied"] {
		t.Error("cluster whose logging status could not be read was not marked in TruncatedIDs")
	}
	w2AssertNoCode(t, res.Findings["acme-denied"], w2RedshiftCodeAuditLoggingOff)
	w2AssertFinding(t, res.Findings["acme-reporting"], w2RedshiftCodeAuditLoggingOff, "audit logging off", domain.SevWarn, w2RedshiftSource)
}

// A parameter group that cannot be read leaves require_ssl unknown for every
// cluster on it, and must not be reported as compliant.
func TestW2RedshiftParameterGroupErrorMarksItsClustersUnknown(t *testing.T) {
	fake := &w2RedshiftFake{
		groupErrs: map[string]error{
			"acme-params-denied": &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"},
		},
	}
	res, err := w2RedshiftRun(t, fake, w2RedshiftCluster("acme-reporting", "acme-params-denied"))
	if err == nil {
		t.Error("a denied parameter-group read was not folded into the composite error")
	}

	if !res.TruncatedIDs["acme-reporting"] {
		t.Error("cluster on an unreadable parameter group was not marked in TruncatedIDs")
	}
	w2AssertNoCode(t, res.Findings["acme-reporting"], w2RedshiftCodeRequireSSLOff)
}

func TestW2RedshiftNilClientIsSafe(t *testing.T) {
	c := w2RedshiftCluster("acme-reporting", "acme-params")
	res, err := w2Enricher(t, "redshift")(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{w2Res("acme-reporting", c)}, nil)
	w2AssertEnricherInvariants(t, res, err)
	if len(res.Findings) != 0 {
		t.Errorf("nil Redshift client produced findings %v", res.Findings)
	}
}

func TestW2RedshiftCapBoundary(t *testing.T) {
	var clusters []redshifttypes.Cluster
	for i := 0; i <= awsclient.EnrichmentCap; i++ {
		clusters = append(clusters, w2RedshiftCluster("acme-cluster-"+strconv.Itoa(i), "acme-params-open"))
	}
	ssl := map[string]string{"acme-params-open": "false"}

	atCap := w2RedshiftEnrich(t, &w2RedshiftFake{requireSSL: ssl}, clusters[:awsclient.EnrichmentCap]...)
	if atCap.Truncated {
		t.Error("exactly EnrichmentCap clusters reported Truncated")
	}

	overCap := w2RedshiftEnrich(t, &w2RedshiftFake{requireSSL: ssl}, clusters...)
	if !overCap.Truncated {
		t.Error("EnrichmentCap+1 clusters did not report Truncated")
	}
}

// A per-item failure is itself a reason the issue count is a lower bound, so
// Truncated must be true even when the cap was never reached. The flag is set
// by Finish, which runs as an operand of the same return statement that yields
// the result value; Go does not specify that ordering for non-call operands,
// so this pins the outcome rather than the mechanism.
func TestW2RedshiftFailureAloneRaisesTruncated(t *testing.T) {
	fake := &w2RedshiftFake{
		clusterErrs: map[string]error{
			"acme-denied": &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"},
		},
	}
	res, err := w2RedshiftRun(t, fake,
		w2RedshiftCluster("acme-denied", "acme-params"),
		w2RedshiftCluster("acme-reporting", "acme-params"),
	)
	if err == nil {
		t.Fatal("expected a composite error for the denied cluster")
	}
	if !res.Truncated {
		t.Error("a failed item left Truncated false; the issue count is presented as complete when it is a lower bound")
	}
}

// A cluster being torn down is not a posture problem: nobody can enable audit
// logging or require_ssl on it, and a row the operator cannot act on still
// counts into the main-menu badge.
func TestW2RedshiftDeletingClusterEmitsNoPostureFinding(t *testing.T) {
	deleting := w2RedshiftCluster("acme-old-cluster", "acme-params-open")
	deleting.ClusterStatus = aws.String("deleting")

	fake := &w2RedshiftFake{
		loggingOff: map[string]bool{"acme-old-cluster": true},
		requireSSL: map[string]string{"acme-params-open": "false"},
	}

	// The enricher reads the row the fetcher produced, so drive both.
	out, err := awsclient.FetchRedshiftClustersPage(context.Background(), &w2RedshiftClustersFake{clusters: []redshifttypes.Cluster{deleting}}, "")
	if err != nil {
		t.Fatalf("FetchRedshiftClustersPage: %v", err)
	}
	res, err := w2Enricher(t, "redshift")(context.Background(), &awsclient.ServiceClients{Redshift: fake}, out.Resources, nil)
	w2AssertEnricherInvariants(t, res, err)

	w2AssertNoCode(t, res.Findings["acme-old-cluster"], w2RedshiftCodeAuditLoggingOff)
	w2AssertNoCode(t, res.Findings["acme-old-cluster"], w2RedshiftCodeRequireSSLOff)
}

type w2RedshiftClustersFake struct {
	clusters []redshifttypes.Cluster
}

func (f *w2RedshiftClustersFake) DescribeClusters(_ context.Context, _ *redshift.DescribeClustersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	return &redshift.DescribeClustersOutput{Clusters: f.clusters}, nil
}
