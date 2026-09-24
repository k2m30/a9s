package unit

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// pagedEndless is a list longer than any capped walk can read.
const pagedEndless = 1 << 30

// pagedList serves total items in pages of size, the way an AWS list API
// does: the token is opaque to the caller (here, the offset of the next
// page) and is absent on the last page. AWS allows a page to hold fewer
// items than the requested maximum, so size is the fake's own page size
// whatever the caller asks for.
type pagedList struct {
	total, size int
	calls, read int
}

func (p *pagedList) page(token *string) (lo, hi int, next *string) {
	p.calls++
	if token != nil {
		lo, _ = strconv.Atoi(*token)
	}
	hi = min(lo+p.size, p.total)
	p.read = max(p.read, hi)
	if hi < p.total {
		next = aws.String(strconv.Itoa(hi))
	}
	return lo, hi, next
}

func pagedIDs(n int, id func(int) string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = id(i)
	}
	return out
}

// assertPagedExact: a list that ends is read to its end and reported exact.
func assertPagedExact(t *testing.T, r resource.RelatedCheckResult, want []string) {
	t.Helper()
	assertPagedWalked(t, r, want, resource.CoverageComplete)
}

// assertPagedWalked: the walk read every page, so the ids are all the API
// offered and the count is not a lower bound, whatever the coverage of the
// match rule that produced them.
func assertPagedWalked(t *testing.T, r resource.RelatedCheckResult, want []string, cov resource.RelatedCoverage) {
	t.Helper()
	if r.State() != domain.RelatedResolved {
		t.Fatalf("state = %v (err %v), want resolved", r.State(), r.Err())
	}
	if r.Truncated() {
		t.Errorf("truncated = true, want false: every page was read")
	}
	got := slices.Sorted(slices.Values(r.ResourceIDs()))
	want = slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		t.Errorf("ids = %d, want %d: got[:3]=%v want[:3]=%v", len(got), len(want), got[:min(3, len(got))], want[:min(3, len(want))])
	}
	if r.Count() != len(want) {
		t.Errorf("count = %d, want %d", r.Count(), len(want))
	}
	if r.Coverage() != cov {
		t.Errorf("coverage = %v, want %v", r.Coverage(), cov)
	}
}

// assertPagedCapped: a list that does not end within the walk's cap is read
// past its first page, stops, and is reported as a lower bound over every
// item it did read.
func assertPagedCapped(t *testing.T, r resource.RelatedCheckResult, p *pagedList, id func(int) string) {
	t.Helper()
	assertPagedCappedCoverage(t, r, p, id, resource.CoveragePartial)
}

// assertPagedCappedCoverage: the same capped walk, for a pivot whose match
// rule sets the coverage — a heuristic match stays heuristic however much of
// the list the walk read.
func assertPagedCappedCoverage(t *testing.T, r resource.RelatedCheckResult, p *pagedList, id func(int) string, cov resource.RelatedCoverage) {
	t.Helper()
	if r.State() != domain.RelatedResolved {
		t.Fatalf("state = %v (err %v), want resolved", r.State(), r.Err())
	}
	if p.calls < 2 {
		t.Errorf("list requested %d times, want the walk to go past page 1", p.calls)
	}
	if p.calls > 1000 {
		t.Errorf("list requested %d times, want a capped walk", p.calls)
	}
	if !r.Truncated() {
		t.Errorf("truncated = false after a walk that stopped with a token pending; the count %d is shown as exact", r.Count())
	}
	got := slices.Sorted(slices.Values(r.ResourceIDs()))
	want := slices.Sorted(slices.Values(pagedIDs(p.read, id)))
	if !slices.Equal(got, want) {
		t.Errorf("ids = %d, want the %d read: got[:3]=%v want[:3]=%v", len(got), len(want), got[:min(3, len(got))], want[:min(3, len(want))])
	}
	if r.Count() != p.read {
		t.Errorf("count = %d, want the %d read", r.Count(), p.read)
	}
	if r.Coverage() != cov {
		t.Errorf("coverage = %v, want %v", r.Coverage(), cov)
	}
}

func pagedChecker(t *testing.T, source, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(source) {
		if def.TargetType == target && def.Checker != nil {
			return def.Checker
		}
	}
	t.Fatalf("%s -> %s checker not registered", source, target)
	return nil
}

type pagedIAM struct {
	awsclient.IAMAPI
	groupUsers, userGroups, userPolicies, rolePolicies, groupPolicies, groupInline *pagedList
}

func pagedUserName(i int) string   { return fmt.Sprintf("user-%04d", i) }
func pagedGroupName(i int) string  { return fmt.Sprintf("group-%04d", i) }
func pagedPolicyName(i int) string { return fmt.Sprintf("policy-%04d", i) }
func pagedInlineName(i int) string { return fmt.Sprintf("inline-%04d", i) }

// pagedPolicyID is the row ID of a customer-managed policy: IAM identifies a
// policy by its ARN, and a name can be shared with an AWS-managed policy.
func pagedPolicyID(i int) string { return "arn:aws:iam::123456789012:policy/" + pagedPolicyName(i) }

// pagedInlineID is the row ID of that inline policy on the group the paging
// tests walk: an inline policy name is the group's to choose.
func pagedInlineID(i int) string { return "inline/platform-engineers/" + pagedInlineName(i) }

func pagedAttached(p *pagedList, marker *string) ([]iamtypes.AttachedPolicy, *string) {
	if p == nil {
		return nil, nil
	}
	lo, hi, next := p.page(marker)
	var out []iamtypes.AttachedPolicy
	for i := lo; i < hi; i++ {
		out = append(out, iamtypes.AttachedPolicy{
			PolicyName: aws.String(pagedPolicyName(i)),
			PolicyArn:  aws.String(pagedPolicyID(i)),
		})
	}
	return out, next
}

func (f *pagedIAM) GetGroup(_ context.Context, in *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	lo, hi, next := f.groupUsers.page(in.Marker)
	out := &iam.GetGroupOutput{
		Group:       &iamtypes.Group{GroupName: in.GroupName, Arn: aws.String("arn:aws:iam::123456789012:group/" + aws.ToString(in.GroupName))},
		IsTruncated: next != nil,
		Marker:      next,
	}
	for i := lo; i < hi; i++ {
		out.Users = append(out.Users, iamtypes.User{
			UserName: aws.String(pagedUserName(i)),
			Arn:      aws.String("arn:aws:iam::123456789012:user/" + pagedUserName(i)),
		})
	}
	return out, nil
}

func (f *pagedIAM) ListGroupsForUser(_ context.Context, in *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	lo, hi, next := f.userGroups.page(in.Marker)
	out := &iam.ListGroupsForUserOutput{IsTruncated: next != nil, Marker: next}
	for i := lo; i < hi; i++ {
		out.Groups = append(out.Groups, iamtypes.Group{
			GroupName: aws.String(pagedGroupName(i)),
			Arn:       aws.String("arn:aws:iam::123456789012:group/" + pagedGroupName(i)),
		})
	}
	return out, nil
}

func (f *pagedIAM) ListAttachedUserPolicies(_ context.Context, in *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	pols, next := pagedAttached(f.userPolicies, in.Marker)
	return &iam.ListAttachedUserPoliciesOutput{AttachedPolicies: pols, IsTruncated: next != nil, Marker: next}, nil
}

func (f *pagedIAM) ListAttachedRolePolicies(_ context.Context, in *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	pols, next := pagedAttached(f.rolePolicies, in.Marker)
	return &iam.ListAttachedRolePoliciesOutput{AttachedPolicies: pols, IsTruncated: next != nil, Marker: next}, nil
}

func (f *pagedIAM) ListAttachedGroupPolicies(_ context.Context, in *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	pols, next := pagedAttached(f.groupPolicies, in.Marker)
	return &iam.ListAttachedGroupPoliciesOutput{AttachedPolicies: pols, IsTruncated: next != nil, Marker: next}, nil
}

func (f *pagedIAM) ListGroupPolicies(_ context.Context, in *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	out := &iam.ListGroupPoliciesOutput{}
	if f.groupInline == nil {
		return out, nil
	}
	lo, hi, next := f.groupInline.page(in.Marker)
	out.IsTruncated, out.Marker = next != nil, next
	for i := lo; i < hi; i++ {
		out.PolicyNames = append(out.PolicyNames, pagedInlineName(i))
	}
	return out, nil
}

// An IAM group of 250 members answers GetGroup in pages of 100 (the IAM
// default MaxItems); the Users pivot counts all 250, exactly.
func TestRelatedPaging_IAMGroupUsers_AllPagesExact(t *testing.T) {
	for _, total := range []int{250, 200} { // 200: the last page is exactly full
		t.Run(strconv.Itoa(total), func(t *testing.T) {
			users := &pagedList{total: total, size: 100}
			clients := &awsclient.ServiceClients{IAM: &pagedIAM{groupUsers: users}}
			r := pagedChecker(t, "iam-group", "iam-user")(context.Background(), clients, resource.Resource{ID: "platform-engineers", Name: "platform-engineers"}, resource.ResourceCache{})
			assertPagedExact(t, r, pagedIDs(total, pagedUserName))
		})
	}
}

func TestRelatedPaging_IAMGroupUsers_CappedWalkIsLowerBound(t *testing.T) {
	users := &pagedList{total: pagedEndless, size: 100}
	clients := &awsclient.ServiceClients{IAM: &pagedIAM{groupUsers: users}}
	r := pagedChecker(t, "iam-group", "iam-user")(context.Background(), clients, resource.Resource{ID: "platform-engineers", Name: "platform-engineers"}, resource.ResourceCache{})
	assertPagedCapped(t, r, users, pagedUserName)
}

// IAM may return fewer than MaxItems with IsTruncated set even for short
// lists, so the small per-principal lists below are served in pages of 5.
func TestRelatedPaging_IAMUserGroupsAndPolicies(t *testing.T) {
	user := resource.Resource{ID: "alice.johnson", Name: "alice.johnson"}

	t.Run("groups exact", func(t *testing.T) {
		groups := &pagedList{total: 8, size: 5}
		clients := &awsclient.ServiceClients{IAM: &pagedIAM{userGroups: groups}}
		assertPagedExact(t, pagedChecker(t, "iam-user", "iam-group")(context.Background(), clients, user, resource.ResourceCache{}), pagedIDs(8, pagedGroupName))
	})
	t.Run("groups capped", func(t *testing.T) {
		groups := &pagedList{total: pagedEndless, size: 5}
		clients := &awsclient.ServiceClients{IAM: &pagedIAM{userGroups: groups}}
		assertPagedCapped(t, pagedChecker(t, "iam-user", "iam-group")(context.Background(), clients, user, resource.ResourceCache{}), groups, pagedGroupName)
	})
	t.Run("policies exact", func(t *testing.T) {
		pols := &pagedList{total: 8, size: 5}
		clients := &awsclient.ServiceClients{IAM: &pagedIAM{userPolicies: pols}}
		assertPagedExact(t, pagedChecker(t, "iam-user", "policy")(context.Background(), clients, user, resource.ResourceCache{}), pagedIDs(8, pagedPolicyID))
	})
	t.Run("policies capped", func(t *testing.T) {
		pols := &pagedList{total: pagedEndless, size: 5}
		clients := &awsclient.ServiceClients{IAM: &pagedIAM{userPolicies: pols}}
		assertPagedCapped(t, pagedChecker(t, "iam-user", "policy")(context.Background(), clients, user, resource.ResourceCache{}), pols, pagedPolicyID)
	})
}

func TestRelatedPaging_RolePolicies(t *testing.T) {
	role := resource.Resource{ID: "app-runtime-role", Name: "app-runtime-role", RawStruct: iamtypes.Role{RoleName: aws.String("app-runtime-role")}}

	t.Run("exact", func(t *testing.T) {
		pols := &pagedList{total: 8, size: 5}
		clients := &awsclient.ServiceClients{IAM: &pagedIAM{rolePolicies: pols}}
		assertPagedExact(t, pagedChecker(t, "role", "policy")(context.Background(), clients, role, resource.ResourceCache{}), pagedIDs(8, pagedPolicyID))
	})
	t.Run("capped", func(t *testing.T) {
		pols := &pagedList{total: pagedEndless, size: 5}
		clients := &awsclient.ServiceClients{IAM: &pagedIAM{rolePolicies: pols}}
		assertPagedCapped(t, pagedChecker(t, "role", "policy")(context.Background(), clients, role, resource.ResourceCache{}), pols, pagedPolicyID)
	})
}

// The group Policies pivot adds two lists, attached and inline; each is
// walked to its end, and a cap on either one makes the sum a lower bound.
func TestRelatedPaging_IAMGroupPolicies(t *testing.T) {
	group := resource.Resource{ID: "platform-engineers", Name: "platform-engineers"}

	t.Run("both exact", func(t *testing.T) {
		clients := &awsclient.ServiceClients{IAM: &pagedIAM{
			groupPolicies: &pagedList{total: 7, size: 5},
			groupInline:   &pagedList{total: 6, size: 5},
		}}
		r := pagedChecker(t, "iam-group", "policy")(context.Background(), clients, group, resource.ResourceCache{})
		assertPagedExact(t, r, append(pagedIDs(7, pagedPolicyID), pagedIDs(6, pagedInlineID)...))
	})
	t.Run("inline capped", func(t *testing.T) {
		inline := &pagedList{total: pagedEndless, size: 5}
		clients := &awsclient.ServiceClients{IAM: &pagedIAM{
			groupPolicies: &pagedList{total: 2, size: 5},
			groupInline:   inline,
		}}
		r := pagedChecker(t, "iam-group", "policy")(context.Background(), clients, group, resource.ResourceCache{})
		if !r.Truncated() || r.Coverage() != resource.CoveragePartial {
			t.Errorf("truncated = %v, coverage = %v with the inline list capped, want a partial lower bound", r.Truncated(), r.Coverage())
		}
		if want := 2 + inline.read; r.Count() != want {
			t.Errorf("count = %d, want %d (2 attached + %d inline read)", r.Count(), want, inline.read)
		}
		if inline.calls < 2 {
			t.Errorf("inline list requested %d times, want the walk to go past page 1", inline.calls)
		}
	})
}

type pagedLambda struct {
	awsclient.LambdaAPI
	esm *pagedList
	arn func(int) string
}

func (f *pagedLambda) ListEventSourceMappings(_ context.Context, in *lambda.ListEventSourceMappingsInput, _ ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	lo, hi, next := f.esm.page(in.Marker)
	out := &lambda.ListEventSourceMappingsOutput{NextMarker: next}
	for i := lo; i < hi; i++ {
		out.EventSourceMappings = append(out.EventSourceMappings, lambdatypes.EventSourceMappingConfiguration{
			UUID:           aws.String(fmt.Sprintf("esm-%04d", i)),
			EventSourceArn: aws.String(f.arn(i)),
			FunctionArn:    aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + aws.ToString(in.FunctionName)),
			State:          aws.String("Enabled"),
		})
	}
	return out, nil
}

// ListEventSourceMappings returns at most 100 mappings per page; every
// event-source pivot of a function reads all of them.
func TestRelatedPaging_LambdaEventSourcePivots(t *testing.T) {
	cases := []struct {
		target string
		arn    func(int) string
		id     func(int) string
	}{
		{"sqs", func(i int) string { return fmt.Sprintf("arn:aws:sqs:us-east-1:123456789012:orders-%04d", i) }, func(i int) string { return fmt.Sprintf("orders-%04d", i) }},
		{"ddb", func(i int) string {
			return fmt.Sprintf("arn:aws:dynamodb:us-east-1:123456789012:table/orders-%04d/stream/2026-01-01T00:00:00.000", i)
		}, func(i int) string { return fmt.Sprintf("orders-%04d", i) }},
		{"kinesis", func(i int) string { return fmt.Sprintf("arn:aws:kinesis:us-east-1:123456789012:stream/events-%04d", i) }, func(i int) string { return fmt.Sprintf("events-%04d", i) }},
		{"msk", func(i int) string {
			return fmt.Sprintf("arn:aws:kafka:us-east-1:123456789012:cluster/ingest-%04d/0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0-%d", i, i%9+1)
		}, func(i int) string { return fmt.Sprintf("ingest-%04d", i) }},
	}
	fn := resource.Resource{ID: "process-orders", Name: "process-orders"}
	for _, tc := range cases {
		t.Run(tc.target+" exact", func(t *testing.T) {
			esm := &pagedList{total: 130, size: 100}
			clients := &awsclient.ServiceClients{Lambda: &pagedLambda{esm: esm, arn: tc.arn}}
			assertPagedExact(t, pagedChecker(t, "lambda", tc.target)(context.Background(), clients, fn, resource.ResourceCache{}), pagedIDs(130, tc.id))
		})
		t.Run(tc.target+" capped", func(t *testing.T) {
			esm := &pagedList{total: pagedEndless, size: 100}
			clients := &awsclient.ServiceClients{Lambda: &pagedLambda{esm: esm, arn: tc.arn}}
			assertPagedCapped(t, pagedChecker(t, "lambda", tc.target)(context.Background(), clients, fn, resource.ResourceCache{}), esm, tc.id)
		})
	}
}

type pagedEC2 struct {
	awsclient.EC2API
	enis, tgwAtt *pagedList
}

func pagedENIID(i int) string { return fmt.Sprintf("eni-0a1b2c3d4e%07d", i) }
func pagedTGWID(i int) string { return fmt.Sprintf("tgw-0f1e2d3c4b%07d", i) }

func (f *pagedEC2) DescribeNetworkInterfaces(_ context.Context, in *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	lo, hi, next := f.enis.page(in.NextToken)
	out := &ec2.DescribeNetworkInterfacesOutput{NextToken: next}
	for i := lo; i < hi; i++ {
		out.NetworkInterfaces = append(out.NetworkInterfaces, ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(pagedENIID(i)),
			Description:        aws.String("RDSNetworkInterface"),
			VpcId:              aws.String("vpc-0a1b2c3d4e5f60718"),
			Groups:             []ec2types.GroupIdentifier{{GroupId: aws.String("sg-0a1b2c3d4e5f60718")}},
		})
	}
	return out, nil
}

func (f *pagedEC2) DescribeTransitGatewayAttachments(_ context.Context, in *ec2.DescribeTransitGatewayAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	lo, hi, next := f.tgwAtt.page(in.NextToken)
	out := &ec2.DescribeTransitGatewayAttachmentsOutput{NextToken: next}
	for i := lo; i < hi; i++ {
		out.TransitGatewayAttachments = append(out.TransitGatewayAttachments, ec2types.TransitGatewayAttachment{
			TransitGatewayAttachmentId: aws.String(fmt.Sprintf("tgw-attach-0a1b2c3d%07d", i)),
			TransitGatewayId:           aws.String(pagedTGWID(i)),
			ResourceId:                 aws.String("vpc-0a1b2c3d4e5f60718"),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpc,
			State:                      ec2types.TransitGatewayAttachmentStateAvailable,
		})
	}
	return out, nil
}

// AWS records no link from a DB instance to its network interfaces: the
// interfaces are matched by the instance's security groups, which also carry
// interfaces of other resources. Paging still owns every page being read.
func TestRelatedPaging_DBInstanceENIs(t *testing.T) {
	db := resource.Resource{ID: "orders-db", Name: "orders-db", RawStruct: rdstypes.DBInstance{
		DBInstanceIdentifier: aws.String("orders-db"),
		VpcSecurityGroups:    []rdstypes.VpcSecurityGroupMembership{{VpcSecurityGroupId: aws.String("sg-0a1b2c3d4e5f60718"), Status: aws.String("active")}},
	}}
	t.Run("exact", func(t *testing.T) {
		enis := &pagedList{total: 7, size: 5}
		clients := &awsclient.ServiceClients{EC2: &pagedEC2{enis: enis}}
		assertPagedWalked(t, pagedChecker(t, "dbi", "eni")(context.Background(), clients, db, resource.ResourceCache{}), pagedIDs(7, pagedENIID), resource.CoverageHeuristic)
	})
	t.Run("capped", func(t *testing.T) {
		enis := &pagedList{total: pagedEndless, size: 5}
		clients := &awsclient.ServiceClients{EC2: &pagedEC2{enis: enis}}
		assertPagedCappedCoverage(t, pagedChecker(t, "dbi", "eni")(context.Background(), clients, db, resource.ResourceCache{}), enis, pagedENIID, resource.CoverageHeuristic)
	})
}

func TestRelatedPaging_VPCTransitGateways(t *testing.T) {
	vpc := resource.Resource{ID: "vpc-0a1b2c3d4e5f60718", Name: "vpc-0a1b2c3d4e5f60718", Fields: map[string]string{"vpc_id": "vpc-0a1b2c3d4e5f60718"}}
	t.Run("exact", func(t *testing.T) {
		att := &pagedList{total: 7, size: 5}
		clients := &awsclient.ServiceClients{EC2: &pagedEC2{tgwAtt: att}}
		assertPagedExact(t, pagedChecker(t, "vpc", "tgw")(context.Background(), clients, vpc, resource.ResourceCache{}), pagedIDs(7, pagedTGWID))
	})
	t.Run("capped", func(t *testing.T) {
		att := &pagedList{total: pagedEndless, size: 5}
		clients := &awsclient.ServiceClients{EC2: &pagedEC2{tgwAtt: att}}
		assertPagedCapped(t, pagedChecker(t, "vpc", "tgw")(context.Background(), clients, vpc, resource.ResourceCache{}), att, pagedTGWID)
	})
}

type pagedEventBridge struct {
	awsclient.EventBridgeAPI
	targets *pagedList
}

func pagedFnName(i int) string { return fmt.Sprintf("dispatch-%04d", i) }

func (f *pagedEventBridge) ListTargetsByRule(_ context.Context, in *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	lo, hi, next := f.targets.page(in.NextToken)
	out := &eventbridge.ListTargetsByRuleOutput{NextToken: next}
	for i := lo; i < hi; i++ {
		out.Targets = append(out.Targets, ebridgetypes.Target{
			Id:  aws.String(fmt.Sprintf("target-%04d", i)),
			Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + pagedFnName(i)),
		})
	}
	return out, nil
}

func TestRelatedPaging_EventBridgeRuleTargets(t *testing.T) {
	rule := resource.Resource{ID: "default/order-events", Name: "order-events", Fields: map[string]string{"name": "order-events", "event_bus": "default"}}
	t.Run("exact", func(t *testing.T) {
		targets := &pagedList{total: 7, size: 5}
		clients := &awsclient.ServiceClients{EventBridge: &pagedEventBridge{targets: targets}}
		assertPagedExact(t, pagedChecker(t, "eb-rule", "lambda")(context.Background(), clients, rule, resource.ResourceCache{}), pagedIDs(7, pagedFnName))
	})
	t.Run("capped", func(t *testing.T) {
		targets := &pagedList{total: pagedEndless, size: 5}
		clients := &awsclient.ServiceClients{EventBridge: &pagedEventBridge{targets: targets}}
		assertPagedCapped(t, pagedChecker(t, "eb-rule", "lambda")(context.Background(), clients, rule, resource.ResourceCache{}), targets, pagedFnName)
	})
}

type pagedASG struct {
	awsclient.ASGAPI
	notifs *pagedList
}

func pagedTopicARN(i int) string {
	return fmt.Sprintf("arn:aws:sns:us-east-1:123456789012:scaling-alerts-%04d", i)
}

func (f *pagedASG) DescribeNotificationConfigurations(_ context.Context, in *autoscaling.DescribeNotificationConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeNotificationConfigurationsOutput, error) {
	lo, hi, next := f.notifs.page(in.NextToken)
	out := &autoscaling.DescribeNotificationConfigurationsOutput{NextToken: next}
	for i := lo; i < hi; i++ {
		out.NotificationConfigurations = append(out.NotificationConfigurations, asgtypes.NotificationConfiguration{
			AutoScalingGroupName: aws.String("web-asg"),
			NotificationType:     aws.String("autoscaling:EC2_INSTANCE_LAUNCH"),
			TopicARN:             aws.String(pagedTopicARN(i)),
		})
	}
	return out, nil
}

func (f *pagedASG) DescribeLifecycleHooks(_ context.Context, _ *autoscaling.DescribeLifecycleHooksInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLifecycleHooksOutput, error) {
	return &autoscaling.DescribeLifecycleHooksOutput{}, nil
}

func TestRelatedPaging_ASGNotificationTopics(t *testing.T) {
	asg := resource.Resource{ID: "web-asg", Name: "web-asg", RawStruct: asgtypes.AutoScalingGroup{AutoScalingGroupName: aws.String("web-asg")}}
	t.Run("exact", func(t *testing.T) {
		notifs := &pagedList{total: 60, size: 50}
		clients := &awsclient.ServiceClients{AutoScaling: &pagedASG{notifs: notifs}}
		assertPagedExact(t, pagedChecker(t, "asg", "sns")(context.Background(), clients, asg, resource.ResourceCache{}), pagedIDs(60, pagedTopicARN))
	})
	t.Run("capped", func(t *testing.T) {
		notifs := &pagedList{total: pagedEndless, size: 50}
		clients := &awsclient.ServiceClients{AutoScaling: &pagedASG{notifs: notifs}}
		assertPagedCapped(t, pagedChecker(t, "asg", "sns")(context.Background(), clients, asg, resource.ResourceCache{}), notifs, pagedTopicARN)
	})
}

type pagedEB struct {
	awsclient.ElasticBeanstalkAPI
}

func (pagedEB) DescribeEnvironmentResources(_ context.Context, in *elasticbeanstalk.DescribeEnvironmentResourcesInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
	return &elasticbeanstalk.DescribeEnvironmentResourcesOutput{EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
		EnvironmentName: in.EnvironmentName,
		LoadBalancers:   []ebtypes.LoadBalancer{{Name: aws.String("awseb-e-m-AWSEBLoa-1ABCDEF")}},
	}}, nil
}

type pagedELBv2 struct {
	awsclient.ELBv2API
}

const pagedLBARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/awseb-e-m-AWSEBLoa-1ABCDEF/50dc6c495c0c9188"

func pagedTGName(i int) string { return fmt.Sprintf("eb-web-tg-%04d", i) }

func (f *pagedELBv2) DescribeLoadBalancers(_ context.Context, in *elbv2.DescribeLoadBalancersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	return &elbv2.DescribeLoadBalancersOutput{LoadBalancers: []elbv2types.LoadBalancer{{
		LoadBalancerName: aws.String(in.Names[0]),
		LoadBalancerArn:  aws.String(pagedLBARN),
		Type:             elbv2types.LoadBalancerTypeEnumApplication,
	}}}, nil
}

// pagedTGRows is a target-group list of n groups forwarded to by the
// environment's load balancer, plus one another load balancer uses.
func pagedTGRows(n int) []resource.Resource {
	row := func(name string, lbARNs ...string) resource.Resource {
		return resource.Resource{ID: name, Name: name, RawStruct: elbv2types.TargetGroup{
			TargetGroupName:  aws.String(name),
			TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/" + name + "/0123456789abcdef"),
			LoadBalancerArns: lbARNs,
		}}
	}
	rows := []resource.Resource{row("other-app-tg", "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/other-app/0fedcba987654321")}
	for i := range n {
		rows = append(rows, row(pagedTGName(i), pagedLBARN))
	}
	return rows
}

// TestRelatedPaging_BeanstalkTargetGroups: a target group names every load
// balancer that forwards to it, so the environment's groups are read off the
// target-group list. A whole list gives an exact count; a list cut short
// gives what it holds as a lower bound.
func TestRelatedPaging_BeanstalkTargetGroups(t *testing.T) {
	env := resource.Resource{ID: "web-prod", Name: "web-prod", RawStruct: ebtypes.EnvironmentDescription{EnvironmentName: aws.String("web-prod")}}
	clients := &awsclient.ServiceClients{ElasticBeanstalk: pagedEB{}, ELBv2: &pagedELBv2{}}
	t.Run("exact", func(t *testing.T) {
		cache := resource.ResourceCache{"tg": {Resources: pagedTGRows(7)}}
		assertPagedExact(t, pagedChecker(t, "eb", "tg")(context.Background(), clients, env, cache), pagedIDs(7, pagedTGName))
	})
	t.Run("list cut short", func(t *testing.T) {
		cache := resource.ResourceCache{"tg": {Resources: pagedTGRows(5), IsTruncated: true}}
		got := pagedChecker(t, "eb", "tg")(context.Background(), clients, env, cache)
		ids := slices.Sorted(slices.Values(got.ResourceIDs()))
		if !slices.Equal(ids, pagedIDs(5, pagedTGName)) || !got.Truncated() {
			t.Errorf("eb → tg over a truncated tg list = %v truncated=%v, want the 5 listed groups as a lower bound", ids, got.Truncated())
		}
	})
}

type pagedAPIGW struct {
	awsclient.APIGatewayV2API
	domains *pagedList
	// mappings answers GetApiMappings for the domain at index i.
	mappings func(i int) (*pagedList, func(j int) string)
}

func pagedDomain(i int) string { return fmt.Sprintf("api-%03d.example.com", i) }
func pagedCertARN(i int) string {
	return fmt.Sprintf("arn:aws:acm:us-east-1:123456789012:certificate/0f1e2d3c-4b5a-6978-8796-%012d", i)
}

func (f *pagedAPIGW) GetDomainNames(_ context.Context, in *apigatewayv2.GetDomainNamesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetDomainNamesOutput, error) {
	lo, hi, next := f.domains.page(in.NextToken)
	out := &apigatewayv2.GetDomainNamesOutput{NextToken: next}
	for i := lo; i < hi; i++ {
		out.Items = append(out.Items, apigwv2types.DomainName{
			DomainName:  aws.String(pagedDomain(i)),
			RoutingMode: apigwv2types.RoutingModeApiMappingOnly,
			DomainNameConfigurations: []apigwv2types.DomainNameConfiguration{{
				CertificateArn:   aws.String(pagedCertARN(i)),
				EndpointType:     apigwv2types.EndpointTypeRegional,
				DomainNameStatus: apigwv2types.DomainNameStatusAvailable,
			}},
		})
	}
	return out, nil
}

func (f *pagedAPIGW) GetApiMappings(_ context.Context, in *apigatewayv2.GetApiMappingsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApiMappingsOutput, error) {
	var idx int
	if _, err := fmt.Sscanf(aws.ToString(in.DomainName), "api-%03d.example.com", &idx); err != nil {
		return nil, err
	}
	list, apiFor := f.mappings(idx)
	lo, hi, next := list.page(in.NextToken)
	out := &apigatewayv2.GetApiMappingsOutput{NextToken: next}
	for j := lo; j < hi; j++ {
		out.Items = append(out.Items, apigwv2types.ApiMapping{
			ApiId:        aws.String(apiFor(j)),
			ApiMappingId: aws.String(fmt.Sprintf("map%05d", j)),
			Stage:        aws.String("prod"),
		})
	}
	return out, nil
}

const pagedAPIID = "a1b2c3d4e5"

func pagedOtherAPI(int) string { return "z9y8x7w6v5" }

func pagedApigwACM(t *testing.T, f *pagedAPIGW) resource.RelatedCheckResult {
	t.Helper()
	clients := &awsclient.ServiceClients{APIGatewayV2: f}
	return pagedChecker(t, "apigw", "acm")(context.Background(), clients, resource.Resource{ID: pagedAPIID, Name: "orders-http-api"}, resource.ResourceCache{})
}

// A custom domain listed on the second page of GetDomainNames that maps to
// this API contributes its certificate.
func TestRelatedPaging_ApigwACM_DomainOnSecondPage(t *testing.T) {
	r := pagedApigwACM(t, &pagedAPIGW{
		domains: &pagedList{total: 2, size: 1},
		mappings: func(i int) (*pagedList, func(int) string) {
			if i == 1 {
				return &pagedList{total: 1, size: 1}, func(int) string { return pagedAPIID }
			}
			return &pagedList{total: 1, size: 1}, pagedOtherAPI
		},
	})
	assertPagedExact(t, r, []string{pagedCertARN(1)})
}

// A domain whose mapping to this API is on the second page of its
// GetApiMappings answer still maps to this API.
func TestRelatedPaging_ApigwACM_MappingOnSecondPage(t *testing.T) {
	r := pagedApigwACM(t, &pagedAPIGW{
		domains: &pagedList{total: 1, size: 1},
		mappings: func(int) (*pagedList, func(int) string) {
			return &pagedList{total: 2, size: 1}, func(j int) string {
				if j == 1 {
					return pagedAPIID
				}
				return "z9y8x7w6v5"
			}
		},
	})
	assertPagedExact(t, r, []string{pagedCertARN(0)})
}

// ApigwACMCappedMappings answers apigw → acm for an API mapped from the first
// of two custom domains while the second domain's GetApiMappings never ends,
// and how many GetApiMappings pages were requested for that second domain.
// Whether the second domain maps here is unknown, so the one certificate is a
// lower bound.
func ApigwACMCappedMappings(t *testing.T) (resource.RelatedCheckResult, int) {
	t.Helper()
	endless := &pagedList{total: pagedEndless, size: 25}
	r := pagedApigwACM(t, &pagedAPIGW{
		domains: &pagedList{total: 2, size: 25},
		mappings: func(i int) (*pagedList, func(int) string) {
			if i == 0 {
				return &pagedList{total: 1, size: 25}, func(int) string { return pagedAPIID }
			}
			return endless, pagedOtherAPI
		},
	})
	return r, endless.calls
}

func TestRelatedPaging_ApigwACM_CappedMappingsAreLowerBound(t *testing.T) {
	r, pages := ApigwACMCappedMappings(t)
	if r.State() != domain.RelatedResolved {
		t.Fatalf("state = %v (err %v), want resolved", r.State(), r.Err())
	}
	if !slices.Equal(r.ResourceIDs(), []string{pagedCertARN(0)}) {
		t.Errorf("ids = %v, want [%s]", r.ResourceIDs(), pagedCertARN(0))
	}
	if pages < 2 {
		t.Errorf("second domain's mappings requested %d times, want the walk to go past page 1", pages)
	}
	if !r.Truncated() || r.Coverage() != resource.CoveragePartial {
		t.Errorf("truncated = %v, coverage = %v, want a partial lower bound", r.Truncated(), r.Coverage())
	}
}

func TestRelatedPaging_ApigwACM_CappedDomainListIsLowerBound(t *testing.T) {
	domains := &pagedList{total: pagedEndless, size: 25}
	r := pagedApigwACM(t, &pagedAPIGW{
		domains: domains,
		mappings: func(int) (*pagedList, func(int) string) {
			return &pagedList{total: 1, size: 25}, func(int) string { return pagedAPIID }
		},
	})
	assertPagedCapped(t, r, domains, pagedCertARN)
}
