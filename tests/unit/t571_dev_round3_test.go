package unit_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// t571ECRPagedFake serves images over pages of one image each, so the walk
// has to follow NextToken, and scan findings over pages of one finding each.
// endless keeps handing back a token, which is what a repository larger
// than the walk's cap looks like.
type t571ECRPagedFake struct {
	awsclient.ECRAPI
	images   []ecrtypes.ImageDetail
	findings map[string][]ecrtypes.ImageScanFinding
	enhanced map[string][]ecrtypes.EnhancedImageScanFinding
	endless  bool
}

func (f *t571ECRPagedFake) DescribeImages(_ context.Context, in *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	i := 0
	if in.NextToken != nil {
		i, _ = strconv.Atoi(*in.NextToken)
	}
	out := &ecr.DescribeImagesOutput{ImageDetails: []ecrtypes.ImageDetail{f.images[i%len(f.images)]}}
	if f.endless || i+1 < len(f.images) {
		out.NextToken = aws.String(strconv.Itoa(i + 1))
	}
	return out, nil
}

func (f *t571ECRPagedFake) DescribeImageScanFindings(_ context.Context, in *ecr.DescribeImageScanFindingsInput, _ ...func(*ecr.Options)) (*ecr.DescribeImageScanFindingsOutput, error) {
	digest := aws.ToString(in.ImageId.ImageDigest)
	basic, enhanced := f.findings[digest], f.enhanced[digest]
	i := 0
	if in.NextToken != nil {
		i, _ = strconv.Atoi(*in.NextToken)
	}
	page := &ecrtypes.ImageScanFindings{
		// The page's own counts disagree with its findings on purpose.
		FindingSeverityCounts: map[string]int32{"CRITICAL": 1},
	}
	switch {
	case i < len(basic):
		page.Findings = basic[i : i+1]
	case i-len(basic) < len(enhanced):
		page.EnhancedFindings = enhanced[i-len(basic) : i-len(basic)+1]
	}
	out := &ecr.DescribeImageScanFindingsOutput{
		ImageScanStatus:   &ecrtypes.ImageScanStatus{Status: ecrtypes.ScanStatusComplete},
		ImageScanFindings: page,
	}
	if i+1 < len(basic)+len(enhanced) {
		out.NextToken = aws.String(strconv.Itoa(i + 1))
	}
	return out, nil
}

func t571ECRImage(digest string, pushed time.Time) ecrtypes.ImageDetail {
	return ecrtypes.ImageDetail{ImageDigest: aws.String(digest), ImagePushedAt: aws.Time(pushed), RepositoryName: aws.String("acme/api")}
}

func t571ECRRepo() resource.Resource {
	return resource.Resource{ID: "acme/api", Name: "acme/api", Fields: map[string]string{"repository_name": "acme/api"}}
}

// The newest image by push time decides the counts wherever it lands in
// DescribeImages' unordered pages, and its counts are the findings on every
// scan page, not a page's findingSeverityCounts.
func TestT571_ECRCountsTheNewestImageOverEveryPage(t *testing.T) {
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	fake := &t571ECRPagedFake{
		images: []ecrtypes.ImageDetail{
			t571ECRImage("sha256:old", day),
			t571ECRImage("sha256:newest", day.Add(48*time.Hour)),
			t571ECRImage("sha256:middle", day.Add(24*time.Hour)),
		},
		findings: map[string][]ecrtypes.ImageScanFinding{
			"sha256:old": {{Severity: ecrtypes.FindingSeverityCritical}, {Severity: ecrtypes.FindingSeverityCritical}},
			"sha256:newest": {
				{Severity: ecrtypes.FindingSeverityHigh},
				{Severity: ecrtypes.FindingSeverityHigh},
				{Severity: ecrtypes.FindingSeverityMedium},
			},
		},
		enhanced: map[string][]ecrtypes.EnhancedImageScanFinding{
			"sha256:newest": {{Severity: aws.String("HIGH")}},
		},
	}
	res, _ := awsclient.EnrichECRRepository(context.Background(), &awsclient.ServiceClients{ECR: fake}, []resource.Resource{t571ECRRepo()}, nil)

	fields := res.FieldUpdates["acme/api"]
	if fields["critical_vulns"] != "0" || fields["high_vulns"] != "3" || fields["images_scanned"] != "1" {
		t.Errorf("critical/high/scanned = %q/%q/%q, want 0/3/1 from the newest image's findings on every page", fields["critical_vulns"], fields["high_vulns"], fields["images_scanned"])
	}
	if _, marked := res.TruncatedIDs["acme/api"]; marked {
		t.Errorf("a repository read in full is marked %q", res.TruncatedIDs["acme/api"])
	}
}

// A repository whose image walk stopped at the cap may hold its newest image
// on a page nobody read: it is not inspected, never counted.
func TestT571_ECRImageWalkAtTheCapIsNotInspected(t *testing.T) {
	fake := &t571ECRPagedFake{
		images:  []ecrtypes.ImageDetail{t571ECRImage("sha256:a", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))},
		endless: true,
	}
	res, _ := awsclient.EnrichECRRepository(context.Background(), &awsclient.ServiceClients{ECR: fake}, []resource.Resource{t571ECRRepo()}, nil)
	if got := res.TruncatedIDs["acme/api"]; got != awsclient.CheckCap {
		t.Errorf("TruncatedIDs = %q, want %q", got, awsclient.CheckCap)
	}
	if fields, ok := res.FieldUpdates["acme/api"]; ok {
		t.Errorf("counts %v reported for a repository whose images were not all read", fields)
	}
}

// t571DepthPerRowFetcher answers page 1 whole, with a per-row failure on one
// of its own rows, then page 2.
func t571DepthPerRowFetcher(t *testing.T, shortName string) {
	t.Helper()
	perRow := awsclient.AggregateFailures("records", []awsclient.Failure{awsclient.FailedCall(bucketID(0), errors.New("AccessDenied"))}, 50)
	resource.SetPaginatedForTest(shortName, func(_ context.Context, _ any, token string) (resource.FetchResult, error) {
		if token == "" {
			return resource.FetchResult{
				Resources:  page1Resources(50),
				Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "p2", TotalHint: -1, PageSize: 50},
			}, perRow
		}
		return resource.FetchResult{
			Resources:  page2Resources(50, 5),
			Pagination: &resource.PaginationMeta{TotalHint: 55, PageSize: 5},
		}, nil
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(shortName) })
}

// A page that answers with its rows and a failure on one of them is a page
// read: the re-fetch walks on to the depth already shown and keeps the
// failure beside the rows.
func TestT571_RefetchWalksPastPerRowFailures(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "s3pilot"
	t571DepthPerRowFetcher(t, shortName)
	seedCachedRows(t, shortName, 55, true)

	c := newDepthExecutorCore(t, false)
	ev, err := c.ExecuteTask(context.Background(), runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: shortName},
	})
	if err != nil {
		t.Fatalf("ExecuteTask: %v", err)
	}
	got, ok := ev.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("got %T, want messages.ResourcesLoaded", ev)
	}
	if len(got.Resources) != 55 {
		t.Errorf("len(Resources) = %d, want 55: a per-row failure on page 1 stopped the walk", len(got.Resources))
	}
	if got.Err == nil {
		t.Error("the per-row failure on page 1 was dropped")
	}
}

type t571RestAPIs struct {
	awsclient.APIGatewayV1API
	failAt   string
	requests []string
}

func (f *t571RestAPIs) GetRestApis(_ context.Context, in *apigateway.GetRestApisInput, _ ...func(*apigateway.Options)) (*apigateway.GetRestApisOutput, error) {
	pos := aws.ToString(in.Position)
	f.requests = append(f.requests, pos)
	if pos == f.failAt {
		return nil, errors.New("ThrottlingException: Rate exceeded")
	}
	if pos == "" {
		return &apigateway.GetRestApisOutput{Items: []apigwtypes.RestApi{{Id: aws.String("rest1"), Name: aws.String("orders")}}, Position: aws.String("p2")}, nil
	}
	return &apigateway.GetRestApisOutput{Items: []apigwtypes.RestApi{{Id: aws.String("rest2"), Name: aws.String("billing")}}}, nil
}

type t571HTTPAPIs struct{ awsclient.APIGatewayV2API }

func (t571HTTPAPIs) GetApis(_ context.Context, _ *apigatewayv2.GetApisInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	return &apigatewayv2.GetApisOutput{Items: []apigwv2types.Api{{ApiId: aws.String("http1"), Name: aws.String("webhooks"), ProtocolType: apigwv2types.ProtocolTypeHttp}}}, nil
}

func t571IDs(rows []resource.Resource) string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return strings.Join(ids, ",")
}

// A REST API page that fails keeps the REST APIs read before it and the HTTP
// APIs, and the cursor resumes the REST walk at the page that failed.
func TestT571_APIGatewayLaneFailureKeepsTheRowsRead(t *testing.T) {
	v1 := &t571RestAPIs{failAt: "p2"}
	c := &awsclient.ServiceClients{APIGatewayV1: v1, APIGatewayV2: t571HTTPAPIs{}}
	res, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), c, "")
	if err == nil {
		t.Fatal("the failed REST page's error was dropped")
	}
	if got := t571IDs(res.Resources); got != "rest1,http1" {
		t.Errorf("rows = %q, want rest1,http1", got)
	}
	if res.Pagination == nil || !res.Pagination.IsTruncated || res.Pagination.NextToken == "" {
		t.Fatalf("pagination = %+v, want a cursor resuming the REST walk", res.Pagination)
	}
	v1.failAt = "never"
	next, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), c, res.Pagination.NextToken)
	if err != nil {
		t.Fatalf("resumed page: %v", err)
	}
	if got := t571IDs(next.Resources); got != "rest2" {
		t.Errorf("resumed rows = %q, want rest2 (the REST page that failed, and no HTTP API twice)", got)
	}
	if next.Pagination == nil || next.Pagination.IsTruncated {
		t.Errorf("resumed pagination = %+v, want the walk complete", next.Pagination)
	}
}

type t571WebACLs struct {
	scope wafv2types.Scope
	fail  bool
	calls int
}

func (f *t571WebACLs) ListWebACLs(_ context.Context, _ *wafv2.ListWebACLsInput, _ ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	f.calls++
	if f.fail {
		return nil, errors.New("WAFInternalErrorException")
	}
	id := strings.ToLower(string(f.scope)) + "-acl"
	return &wafv2.ListWebACLsOutput{WebACLs: []wafv2types.WebACLSummary{{Id: aws.String(id), Name: aws.String(id), ARN: aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/" + id + "/" + id)}}}, nil
}

// A CLOUDFRONT scope that fails keeps the REGIONAL web ACLs, and the cursor
// resumes CLOUDFRONT alone.
func TestT571_WAFScopeFailureKeepsTheRowsRead(t *testing.T) {
	regional := &t571WebACLs{scope: wafv2types.ScopeRegional}
	cf := &t571WebACLs{scope: wafv2types.ScopeCloudfront, fail: true}
	res, err := awsclient.FetchWAFWebACLsPageWithCloudFront(context.Background(), regional, cf, "")
	if err == nil {
		t.Fatal("the CLOUDFRONT failure was dropped")
	}
	if got := t571IDs(res.Resources); got != "regional-acl" {
		t.Errorf("rows = %q, want regional-acl", got)
	}
	if res.Pagination == nil || !res.Pagination.IsTruncated || res.Pagination.NextToken == "" {
		t.Fatalf("pagination = %+v, want a cursor resuming CLOUDFRONT", res.Pagination)
	}
	cf.fail = false
	next, err := awsclient.FetchWAFWebACLsPageWithCloudFront(context.Background(), regional, cf, res.Pagination.NextToken)
	if err != nil {
		t.Fatalf("resumed page: %v", err)
	}
	if got := t571IDs(next.Resources); got != "cloudfront-acl" {
		t.Errorf("resumed rows = %q, want cloudfront-acl alone", got)
	}
	if regional.calls != 1 {
		t.Errorf("REGIONAL listed %d times, want once", regional.calls)
	}
}

type t571DeniedDocDB struct{ awsclient.DocDBAPI }

type t571AuroraRDS struct{ awsclient.RDSAPI }

func (t571AuroraRDS) DescribeDBClusters(_ context.Context, _ *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{
		DBClusters: []rdstypes.DBCluster{{DBClusterIdentifier: aws.String("aurora-orders"), Engine: aws.String("aurora-postgresql"), Status: aws.String("available")}},
	}, nil
}

func (t571AuroraRDS) DescribeGlobalClusters(_ context.Context, _ *rds.DescribeGlobalClustersInput, _ ...func(*rds.Options)) (*rds.DescribeGlobalClustersOutput, error) {
	return &rds.DescribeGlobalClustersOutput{}, nil
}

func (t571DeniedDocDB) DescribeDBClusters(_ context.Context, _ *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return nil, errors.New("AccessDenied: not authorized to perform rds:DescribeDBClusters")
}

// A DocumentDB read that fails does not hide the Aurora clusters: they come
// back as a lower bound with the error, and the cursor resumes DocumentDB.
func TestT571_DBClusterDocDBFailureKeepsTheRDSRows(t *testing.T) {
	c := &awsclient.ServiceClients{
		DocDB: t571DeniedDocDB{},
		RDS:   t571AuroraRDS{},
	}
	res, err := resource.GetPaginatedFetcher("dbc")(context.Background(), c, "")
	if err == nil {
		t.Fatal("the DocumentDB failure was dropped")
	}
	if got := t571IDs(res.Resources); got != "aurora-orders" {
		t.Errorf("rows = %q, want aurora-orders", got)
	}
	if res.Pagination == nil || !res.Pagination.IsTruncated || !strings.HasPrefix(res.Pagination.NextToken, "docdb-retry:") {
		t.Errorf("pagination = %+v, want truncated with a cursor resuming DocumentDB", res.Pagination)
	}
}

type t571VPCEC2 struct {
	awsclient.EC2API
	subnetsErr error
	flowCalls  int
}

func (f *t571VPCEC2) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{Vpcs: []ec2types.Vpc{{VpcId: aws.String("vpc-0a1b2c3d"), CidrBlock: aws.String("10.0.0.0/16"), State: ec2types.VpcStateAvailable}}}, nil
}

func (f *t571VPCEC2) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return nil, f.subnetsErr
}

func (f *t571VPCEC2) DescribeFlowLogs(_ context.Context, _ *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	f.flowCalls++
	return &ec2.DescribeFlowLogsOutput{}, nil
}

// A VPC whose subnets could not be read is not inspected for flow logs: the
// VPC id alone cannot say whether its subnets are covered.
func TestT571_VPCWithUnreadSubnetsIsNotInspected(t *testing.T) {
	api := &t571VPCEC2{subnetsErr: errors.New("UnauthorizedOperation")}
	page, err := awsclient.FetchVPCsPage(context.Background(), api, "")
	if err != nil {
		t.Fatalf("FetchVPCsPage: %v", err)
	}
	res, _ := awsclient.EnrichVPCFlowLogs(context.Background(), &awsclient.ServiceClients{EC2: api}, page.Resources, nil)
	if _, marked := res.TruncatedIDs["vpc-0a1b2c3d"]; !marked {
		t.Error("a VPC whose subnets were not read is not marked uninspected")
	}
	if len(res.Findings["vpc-0a1b2c3d"]) != 0 {
		t.Errorf("findings = %v on a VPC whose subnets were not read", res.Findings["vpc-0a1b2c3d"])
	}
	if api.flowCalls != 0 {
		t.Errorf("DescribeFlowLogs called %d times about the VPC id alone", api.flowCalls)
	}
}

type t571FlakyDocDB struct {
	awsclient.DocDBAPI
	denied bool
}

func (f *t571FlakyDocDB) DescribeDBClusters(_ context.Context, _ *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	if f.denied {
		return nil, errors.New("AccessDenied: not authorized to perform rds:DescribeDBClusters")
	}
	return &docdb.DescribeDBClustersOutput{DBClusters: []docdbtypes.DBCluster{{DBClusterIdentifier: aws.String("docdb-catalog"), Engine: aws.String("docdb"), Status: aws.String("available")}}}, nil
}

// t571PagedAuroraRDS answers two RDS cluster pages and records every marker
// it was asked for.
type t571PagedAuroraRDS struct {
	awsclient.RDSAPI
	markers []string
}

func (f *t571PagedAuroraRDS) DescribeDBClusters(_ context.Context, in *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	marker := aws.ToString(in.Marker)
	f.markers = append(f.markers, marker)
	if marker == "" {
		return &rds.DescribeDBClustersOutput{
			DBClusters: []rdstypes.DBCluster{{DBClusterIdentifier: aws.String("aurora-orders"), Engine: aws.String("aurora-postgresql"), Status: aws.String("available")}},
			Marker:     aws.String("r2"),
		}, nil
	}
	return &rds.DescribeDBClustersOutput{
		DBClusters: []rdstypes.DBCluster{{DBClusterIdentifier: aws.String("aurora-billing"), Engine: aws.String("aurora-mysql"), Status: aws.String("available")}},
	}, nil
}

func (*t571PagedAuroraRDS) DescribeGlobalClusters(_ context.Context, _ *rds.DescribeGlobalClustersInput, _ ...func(*rds.Options)) (*rds.DescribeGlobalClustersOutput, error) {
	return &rds.DescribeGlobalClustersOutput{}, nil
}

// A DocumentDB denial that persists still walks RDS to its last page, each
// page once, and the DocumentDB page is retried until it answers, without
// reading RDS again.
func TestT571_DBClusterPersistentDocDBDenialWalksRDSToTheEnd(t *testing.T) {
	docDB := &t571FlakyDocDB{denied: true}
	rdsAPI := &t571PagedAuroraRDS{}
	c := &awsclient.ServiceClients{DocDB: docDB, RDS: rdsAPI}
	fetch := resource.GetPaginatedFetcher("dbc")

	var seen []string
	token := ""
	for range 2 {
		res, err := fetch(context.Background(), c, token)
		if err == nil {
			t.Fatal("the DocumentDB denial was dropped")
		}
		seen = append(seen, t571IDs(res.Resources))
		if res.Pagination == nil || !res.Pagination.IsTruncated || res.Pagination.NextToken == "" {
			t.Fatalf("pagination = %+v, want a cursor while DocumentDB is unread", res.Pagination)
		}
		token = res.Pagination.NextToken
	}
	if got := strings.Join(seen, ","); got != "aurora-orders,aurora-billing" {
		t.Errorf("rows over two pages = %q, want aurora-orders,aurora-billing", got)
	}
	if got := strings.Join(rdsAPI.markers, "|"); got != "|r2" {
		t.Errorf("RDS markers = %q, want the first page then r2", got)
	}

	res, err := fetch(context.Background(), c, token)
	if err == nil || len(res.Resources) != 0 {
		t.Errorf("DocumentDB still denied after RDS ended: rows %q, err %v; want the error alone", t571IDs(res.Resources), err)
	}

	docDB.denied = false
	res, err = fetch(context.Background(), c, token)
	if err != nil {
		t.Fatalf("DocumentDB answered: %v", err)
	}
	if got := t571IDs(res.Resources); got != "docdb-catalog" {
		t.Errorf("rows = %q, want docdb-catalog alone", got)
	}
	if res.Pagination == nil || res.Pagination.IsTruncated {
		t.Errorf("pagination = %+v, want the list complete", res.Pagination)
	}
	if len(rdsAPI.markers) != 2 {
		t.Errorf("RDS read %d times, want each page once", len(rdsAPI.markers))
	}
}
