package main

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/middleware"
)

// Route 53 ListHostedZones echoes the request's Marker and names the next page
// in NextMarker; Lambda ListFunctions takes Marker and answers NextMarker.
func TestT11aDev_ResumeFields_SendsNextMarkerBackAsMarker(t *testing.T) {
	for _, c := range [][2]reflect.Type{
		{reflect.TypeFor[route53.ListHostedZonesInput](), reflect.TypeFor[route53.ListHostedZonesOutput]()},
		{reflect.TypeFor[lambda.ListFunctionsInput](), reflect.TypeFor[lambda.ListFunctionsOutput]()},
	} {
		resume, unresolved := resumeFields(c[0], c[1])
		if fmt.Sprint(resume) != "map[NextMarker:Marker]" || len(unresolved) > 0 {
			t.Errorf("%s: resume %v, unresolved %v", c[0].Name(), resume, unresolved)
		}
	}
}

func TestT11aDev_DescribeListed_AsksAboutAnIDListedTwiceOnce(t *testing.T) {
	var asked []string
	rows := describeListed([]string{"a", "b", "a"}, 10, func(batch []string) (map[string]string, []string, error) {
		asked = append(asked, batch...)
		return map[string]string{"a": "x", "b": "y"}, nil, nil
	})
	var got []string
	for _, r := range rows {
		got = append(got, r.id+":"+r.read.Outcome)
	}
	if !slices.Equal(asked, []string{"a", "b"}) || !slices.Equal(got, []string{"a:ok", "b:ok"}) {
		t.Errorf("asked %v, rows %v", asked, got)
	}
}

func TestT11aDev_RowOf_AnIDTheServiceReportedGoneIsNotFound(t *testing.T) {
	rows := describeListed([]string{"kept", "deleted"}, 10, func([]string) (map[string]string, []string, error) {
		return map[string]string{"kept": "x"}, []string{"deleted"}, nil
	})
	if got := rowOf(rows, "deleted"); got.item != nil || got.read.Outcome != outcomeNotFound {
		t.Errorf("rowOf(deleted) = %+v", got)
	}
	if got := rowOf(rows, "kept"); got.item == nil || got.read.Outcome != "ok" {
		t.Errorf("rowOf(kept) = %+v", got)
	}
}

// AWS answers GetLoginProfile with NoSuchEntity for a user with no console
// password, and GetRole with NoSuchEntity for a role deleted after the list.
func TestT11aDev_OutcomeOf_AnAbsentConfigurationIsNoneAndAGoneItemIsNotFound(t *testing.T) {
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		switch op {
		case "ListUsers":
			return &iam.ListUsersOutput{Users: []iamtypes.User{{UserName: aws.String("ci-deployer"), Arn: aws.String("arn:aws:iam::123456789012:user/ci-deployer")}}}, nil
		case "ListRoles":
			return &iam.ListRolesOutput{Roles: []iamtypes.Role{{RoleName: aws.String("retired-role"), Arn: aws.String("arn:aws:iam::123456789012:role/retired-role")}}}, nil
		case "GetLoginProfile", "GetRole":
			return nil, t11aAPIError("NoSuchEntity", "The user or role cannot be found.")
		case "ListAccessKeys":
			return &iam.ListAccessKeysOutput{}, nil
		case "ListRolePolicies":
			return &iam.ListRolePoliciesOutput{}, nil
		}
		return t11aDefault(service, in)
	})
	users, err := captureIAMUser(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := users.(iamUserData).Users[0].LoginProfile; got != (readOutcome{Outcome: outcomeNone}) {
		t.Errorf("login profile = %+v, want none", got)
	}
	roles, err := captureRole(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := roles.(roleData).Roles[0].RoleLastUsed; got != (readOutcome{Outcome: outcomeNotFound}) {
		t.Errorf("role last used = %+v, want not_found", got)
	}
}

func TestT11aDev_CaptureEC2_ARefusedStatusReadKeepsTheInstances(t *testing.T) {
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		switch op {
		case "DescribeInstances":
			return &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{InstanceId: aws.String("i-0abc1234def567890")}}}}}, nil
		case "DescribeInstanceStatus":
			return nil, t11aAPIError("UnauthorizedOperation", "You are not authorized to perform this operation.")
		}
		return t11aDefault(service, in)
	})
	got, err := captureEC2(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	d := got.(ec2Data)
	if len(d.Instances) != 1 || d.StatusesRead.ErrorCode != "UnauthorizedOperation" || len(d.partial()) != 1 {
		t.Errorf("instances %d, statuses read %+v, partial %v", len(d.Instances), d.StatusesRead, d.partial())
	}
}

func TestT11aDev_CaptureECSSvc_OneClusterRefusedOrGoneKeepsTheOthers(t *testing.T) {
	const kept, denied, gone = "arn:aws:ecs:eu-west-1:123456789012:cluster/kept", "arn:aws:ecs:eu-west-1:123456789012:cluster/denied", "arn:aws:ecs:eu-west-1:123456789012:cluster/gone"
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		switch op {
		case "ListClusters":
			return &ecs.ListClustersOutput{ClusterArns: []string{kept, denied, gone}}, nil
		case "ListServices":
			switch aws.ToString(in.(*ecs.ListServicesInput).Cluster) {
			case denied:
				return nil, t11aAPIError("AccessDeniedException", "not authorized")
			case gone:
				return nil, t11aAPIError("ClusterNotFoundException", "Cluster not found.")
			}
			return &ecs.ListServicesOutput{ServiceArns: []string{"arn:aws:ecs:eu-west-1:123456789012:service/kept/web"}}, nil
		case "DescribeServices":
			arn := in.(*ecs.DescribeServicesInput).Services[0]
			return &ecs.DescribeServicesOutput{Services: []ecstypes.Service{{ServiceArn: aws.String(arn), ServiceName: aws.String("web"), ClusterArn: aws.String(kept)}}}, nil
		}
		return t11aDefault(service, in)
	})
	got, err := captureECSSvc(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	d := got.(ecsSvcData)
	if len(d.Services) != 1 || d.Services[0].Outcome != "ok" || len(d.partial()) != 1 {
		t.Errorf("services %+v, partial %v", d.Services, d.partial())
	}
}

func TestT11aDev_PageAll_StopsWhenTheCursorDoesNotAdvance(t *testing.T) {
	calls := 0
	items, err := pageAll(context.Background(), &lambda.ListFunctionsInput{},
		func(_ context.Context, in *lambda.ListFunctionsInput) (*lambda.ListFunctionsOutput, error) {
			calls++
			if calls > 5 {
				t.Fatal("pageAll kept asking for the same page")
			}
			return &lambda.ListFunctionsOutput{Functions: []lambdatypes.FunctionConfiguration{{FunctionName: aws.String("fn")}}, NextMarker: aws.String("page-2")}, nil
		},
		func(out *lambda.ListFunctionsOutput) []lambdatypes.FunctionConfiguration { return out.Functions }, allPages)
	if err == nil || len(items) != 2 || calls != 2 {
		t.Errorf("items %d, calls %d, err %v; want the two pages read and an error", len(items), calls, err)
	}
}

// ListBuckets names each bucket's Region, and a bucket's own Region answers
// every read about it.
func TestT11aDev_CaptureS3_AsksTheListedBucketRegion(t *testing.T) {
	var mu sync.Mutex
	regions := map[string]string{}
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		switch op {
		case "ListBuckets":
			return &s3.ListBucketsOutput{Buckets: []s3types.Bucket{{Name: aws.String("acme-logs"), BucketRegion: aws.String("eu-central-1")}}}, nil
		case "GetBucketLocation":
			return &s3.GetBucketLocationOutput{LocationConstraint: s3types.BucketLocationConstraintEuCentral1}, nil
		}
		return t11aDefault(service, in)
	})
	cfg.APIOptions = append(cfg.APIOptions, func(st *middleware.Stack) error {
		return st.Initialize.Add(middleware.InitializeMiddlewareFunc("t11aRegion",
			func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
				mu.Lock()
				regions[awsmiddleware.GetOperationName(ctx)] = awsmiddleware.GetRegion(ctx)
				mu.Unlock()
				return next.HandleInitialize(ctx, in)
			}), middleware.Before)
	})
	cfg.Region = "us-east-1"
	if _, err := captureS3(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"GetBucketLocation", "GetPublicAccessBlock", "GetBucketVersioning"} {
		if regions[op] != "eu-central-1" {
			t.Errorf("%s sent to %q, want eu-central-1 (all: %v)", op, regions[op], regions)
		}
	}
}

func TestT11aDev_CapturePolicy_ADeniedGroupKeepsTheLaterGroupsInlinePolicies(t *testing.T) {
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		switch op {
		case "ListPolicies":
			return &iam.ListPoliciesOutput{}, nil
		case "ListGroups":
			return &iam.ListGroupsOutput{Groups: []iamtypes.Group{{GroupName: aws.String("admins")}, {GroupName: aws.String("auditors")}, {GroupName: aws.String("deployers")}}}, nil
		case "ListGroupPolicies":
			group := aws.ToString(in.(*iam.ListGroupPoliciesInput).GroupName)
			if group == "auditors" {
				return nil, t11aAPIError("AccessDenied", "not authorized")
			}
			return &iam.ListGroupPoliciesOutput{PolicyNames: []string{group + "-inline"}}, nil
		}
		return t11aDefault(service, in)
	})
	got, err := capturePolicy(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	words := map[string]string{}
	for _, g := range got.(policyData).GroupInlinePolicies {
		words[g.GroupName] = g.Outcome + fmt.Sprint(g.PolicyNames)
	}
	want := map[string]string{"admins": "ok[admins-inline]", "auditors": "error[]", "deployers": "ok[deployers-inline]"}
	if fmt.Sprint(words) != fmt.Sprint(want) {
		t.Errorf("group inline policies %v, want %v", words, want)
	}
}

func TestT11aDev_CaptureDBI_ARefusedSecondPageKeepsTheFirst(t *testing.T) {
	cfg := answeringConfig(func(service, op string, in any) (any, error) {
		if op == "DescribeDBInstances" {
			if in.(*rds.DescribeDBInstancesInput).Marker == nil {
				return &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{{DBInstanceIdentifier: aws.String("orders-db")}}, Marker: aws.String("page-2")}, nil
			}
			return nil, t11aAPIError("AccessDenied", "not authorized")
		}
		return t11aDefault(service, in)
	})
	got, err := captureDBI(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	d := got.(dbiData)
	if len(d.Instances) != 1 || d.Instances[0].DBInstanceIdentifier != "orders-db" || len(d.partial()) != 1 {
		t.Errorf("instances %+v, partial %v", d.Instances, d.partial())
	}
}

func TestT11aDev_RequestQuestion_KeepsTheLookbackOfAClockWindow(t *testing.T) {
	window := func(now time.Time, back time.Duration) string {
		return requestQuestion(&cloudtrail.LookupEventsInput{StartTime: aws.Time(now.Add(-back)), EndTime: aws.Time(now)})
	}
	now := time.Now()
	if window(now, time.Hour) != window(now.Add(-2*time.Second), time.Hour) {
		t.Error("the same one-hour window asked two seconds apart reads as two questions")
	}
	if window(now, time.Hour) == window(now, 7*24*time.Hour) {
		t.Error("a one-hour and a seven-day window read as one question")
	}
}
