package unit

// Exposure verdicts judged on everything AWS reports: the address family a
// security-group rule opens against the address family the instance actually
// has, a security-group list that stops short of the attached group, every
// cache behaviour of a distribution rather than its default alone, and the
// public-bucket verdict the trail shares with the S3 posture pass.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	t562VPC      = "vpc-0aaaa1111bbbb2222"
	t562SubnetA  = "subnet-0a1111aaaa2222aaa"
	t562SubnetB  = "subnet-0b1111aaaa2222bbb"
	t562SubnetC  = "subnet-0c1111aaaa2222ccc"
	t562IGW      = "igw-0a1b2c3d4e5f60718"
	t562EIGW     = "eigw-0a1b2c3d4e5f60719"
	t562IPv6B    = "2001:db8:1234:1a00::10"
	t562IPv6C    = "2001:db8:1234:1b00::20"
	t562SSHv6SG  = "sg-0ssh6only0aaaaaa1"
	t562RTBA     = "rtb-0a1111aaaa2222aaa"
	t562RTBB     = "rtb-0b1111aaaa2222bbb"
	t562RTBC     = "rtb-0c1111aaaa2222ccc"
	t562RTBMain  = "rtb-0main111aaaa2222a"
	t562ExpAllCd = domain.FindingCode("ec2.internet-exposed-all")
)

func t562AccessDenied(op string) error {
	return &smithy.GenericAPIError{
		Code:    "UnauthorizedOperation",
		Message: "You are not authorized to perform this operation: " + op,
		Fault:   smithy.FaultClient,
	}
}

// t562EC2EnrichFake refuses the sibling-list reads, so an enricher that
// fetches a missing list itself still sees no answer rather than a nil client.
type t562EC2EnrichFake struct {
	pw1EC2EnrichFake
}

func (f *t562EC2EnrichFake) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return nil, t562AccessDenied("ec2:DescribeRouteTables")
}

func (f *t562EC2EnrichFake) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return nil, t562AccessDenied("ec2:DescribeSecurityGroups")
}

// t562SSHv6Only opens port 22 to ::/0 and to nothing over IPv4.
func t562SSHv6Only(id string) ec2types.SecurityGroup {
	return ec2types.SecurityGroup{
		GroupId:     aws.String(id),
		GroupName:   aws.String("acme-bastion-v6"),
		VpcId:       aws.String(t562VPC),
		Description: aws.String("ssh over ipv6"),
		OwnerId:     aws.String("123456789012"),
		IpPermissions: []ec2types.IpPermission{{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(22),
			ToPort:     aws.Int32(22),
			Ipv6Ranges: []ec2types.Ipv6Range{{CidrIpv6: aws.String("::/0"), Description: aws.String("anyone")}},
		}},
	}
}

// t562Instance is a running instance with one interface in subnet, carrying
// the attached groups the way DescribeInstances reports them: on the instance
// and on the interface. A public IPv4 address is the association of the
// primary private address; an IPv6 address sits on the interface, and a
// primary one is also the instance-level Ipv6Address, which EC2 fills only
// for a primary IPv6. Either address may be "".
func t562Instance(id, subnet, publicIPv4, ipv6 string, primary bool, sgIDs ...string) ec2types.Instance {
	inst := pw1Instance(id, "running", publicIPv4, ec2types.HttpTokensStateRequired, sgIDs...)
	inst.VpcId = aws.String(t562VPC)
	inst.SubnetId = aws.String(subnet)
	inst.PrivateIpAddress = aws.String("10.0.1.25")
	eni := ec2types.InstanceNetworkInterface{
		NetworkInterfaceId: aws.String("eni-0" + id[len("i-0"):]),
		SubnetId:           aws.String(subnet),
		VpcId:              aws.String(t562VPC),
		PrivateIpAddress:   aws.String("10.0.1.25"),
		PrivateIpAddresses: []ec2types.InstancePrivateIpAddress{{Primary: aws.Bool(true), PrivateIpAddress: aws.String("10.0.1.25")}},
	}
	for _, sg := range sgIDs {
		eni.Groups = append(eni.Groups, ec2types.GroupIdentifier{GroupId: aws.String(sg), GroupName: aws.String("acme-" + sg)})
	}
	if publicIPv4 != "" {
		assoc := &ec2types.InstanceNetworkInterfaceAssociation{PublicIp: aws.String(publicIPv4), IpOwnerId: aws.String("amazon")}
		eni.Association = assoc
		eni.PrivateIpAddresses[0].Association = assoc
	}
	if ipv6 != "" {
		eni.Ipv6Addresses = []ec2types.InstanceIpv6Address{{Ipv6Address: aws.String(ipv6), IsPrimaryIpv6: aws.Bool(primary)}}
		if primary {
			inst.Ipv6Address = aws.String(ipv6)
		}
	}
	inst.NetworkInterfaces = []ec2types.InstanceNetworkInterface{eni}
	return inst
}

func t562IPv6Instance(id, subnet, ipv6 string, primary bool, sgIDs ...string) ec2types.Instance {
	return t562Instance(id, subnet, "", ipv6, primary, sgIDs...)
}

func t562RouteTable(id, subnet string, defaultV6 ec2types.Route) ec2types.RouteTable {
	return ec2types.RouteTable{
		RouteTableId: aws.String(id),
		VpcId:        aws.String(t562VPC),
		OwnerId:      aws.String("123456789012"),
		Associations: []ec2types.RouteTableAssociation{{
			RouteTableAssociationId: aws.String("rtbassoc-" + id[len("rtb-"):]),
			RouteTableId:            aws.String(id),
			SubnetId:                aws.String(subnet),
			Main:                    aws.Bool(false),
		}},
		Routes: []ec2types.Route{
			{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive},
			{DestinationIpv6CidrBlock: aws.String("2001:db8:1234::/56"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive},
			defaultV6,
		},
	}
}

func t562IGWRoute() ec2types.Route {
	return ec2types.Route{DestinationIpv6CidrBlock: aws.String("::/0"), GatewayId: aws.String(t562IGW), State: ec2types.RouteStateActive}
}

func t562IGWv4Route() ec2types.Route {
	return ec2types.Route{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String(t562IGW), State: ec2types.RouteStateActive}
}

// AWS reports an egress-only gateway route in its own field; GatewayId is empty.
func t562EIGWRoute() ec2types.Route {
	return ec2types.Route{DestinationIpv6CidrBlock: aws.String("::/0"), EgressOnlyInternetGatewayId: aws.String(t562EIGW), State: ec2types.RouteStateActive}
}

// t562MainTable is the VPC's main table: no explicit subnet, IPv6 default to
// the internet gateway.
func t562MainTable() ec2types.RouteTable {
	return ec2types.RouteTable{
		RouteTableId: aws.String(t562RTBMain),
		VpcId:        aws.String(t562VPC),
		OwnerId:      aws.String("123456789012"),
		Associations: []ec2types.RouteTableAssociation{{
			RouteTableAssociationId: aws.String("rtbassoc-0main111aaaa2222a"),
			RouteTableId:            aws.String(t562RTBMain),
			Main:                    aws.Bool(true),
		}},
		Routes: []ec2types.Route{
			{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive},
			t562IGWRoute(),
		},
	}
}

type t562RTBFake struct{ tables []ec2types.RouteTable }

func (f *t562RTBFake) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{RouteTables: f.tables}, nil
}

func t562RTBEntry(t *testing.T, truncated bool, tables ...ec2types.RouteTable) resource.ResourceCacheEntry {
	t.Helper()
	out, err := awsclient.FetchRouteTablesPage(context.Background(), &t562RTBFake{tables: tables}, "")
	if err != nil {
		t.Fatalf("FetchRouteTablesPage: %v", err)
	}
	return resource.ResourceCacheEntry{Resources: out.Resources, IsTruncated: truncated}
}

// t562Cache pairs the complete sg list holding the IPv6-only SSH group with rtb.
func t562Cache(t *testing.T, rtb *resource.ResourceCacheEntry) resource.ResourceCache {
	t.Helper()
	cache := pw1SGCache(t, t562SSHv6Only(t562SSHv6SG))
	if rtb != nil {
		cache["rtb"] = *rtb
	}
	return cache
}

func t562EnrichEC2(t *testing.T, cache resource.ResourceCache, instances ...ec2types.Instance) awsclient.IssueEnricherResult {
	t.Helper()
	rs := pw1FetchEC2(t, instances...)
	res, err := awsclient.EnrichEC2InstanceStatus(context.Background(), &awsclient.ServiceClients{EC2: &t562EC2EnrichFake{}}, rs, cache)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	return res
}

func t562RequireNoExposure(t *testing.T, res awsclient.IssueEnricherResult, id string) {
	t.Helper()
	for _, f := range res.Findings[id] {
		if f.Code == pw1EC2CodeInternetExposed || f.Code == t562ExpAllCd {
			t.Errorf("%s carries %s %q; the address family the rule opens is not one the instance can be reached on", id, f.Code, f.Phrase)
		}
	}
}

// A rule open to ::/0 reaches an instance only over IPv6; an instance with
// only a public IPv4 address is not exposed by it.
func TestT562_IPv6RuleDoesNotExposeAnIPv4OnlyInstance(t *testing.T) {
	const id = "i-0t562v4only00aaa1"
	// The subnet reaches the internet over both families, so only the rule's
	// address family can keep the instance closed.
	table := t562RouteTable(t562RTBA, t562SubnetA, t562IGWRoute())
	table.Routes = append(table.Routes, t562IGWv4Route())
	rtb := t562RTBEntry(t, false, t562MainTable(), table)
	res := t562EnrichEC2(t, t562Cache(t, &rtb),
		t562Instance(id, t562SubnetA, "203.0.113.40", "", false, t562SSHv6SG))

	t562RequireNoExposure(t, res, id)
	if reason, marked := res.TruncatedIDs[id]; marked {
		t.Errorf("%s marked not inspected (%q); every list it needs was loaded", id, reason)
	}
}

// An IPv6 address is globally routable; with the subnet's ::/0 route on an
// internet gateway, a group open to ::/0 on port 22 exposes it with no public
// IPv4 address at all.
func TestT562_IPv6InstanceBehindInternetGatewayIsExposed(t *testing.T) {
	const id = "i-0t562v6igw000aaa1"
	rtb := t562RTBEntry(t, false, t562MainTable(), t562RouteTable(t562RTBB, t562SubnetB, t562IGWRoute()))
	res := t562EnrichEC2(t, t562Cache(t, &rtb),
		t562IPv6Instance(id, t562SubnetB, t562IPv6B, false, t562SSHv6SG))

	pw1RequireFinding(t, res.Findings[id], pw1EC2CodeInternetExposed,
		"port 22 reachable from the internet", domain.SevBroken, "wave2")
	if reason, marked := res.TruncatedIDs[id]; marked {
		t.Errorf("%s marked not inspected (%q) alongside a decided verdict", id, reason)
	}
}

// An egress-only internet gateway admits no inbound connection, so the same
// group and address behind one expose nothing.
func TestT562_IPv6InstanceBehindEgressOnlyGatewayIsNotExposed(t *testing.T) {
	const id = "i-0t562v6eigw00aaa1"
	rtb := t562RTBEntry(t, false, t562MainTable(), t562RouteTable(t562RTBC, t562SubnetC, t562EIGWRoute()))
	res := t562EnrichEC2(t, t562Cache(t, &rtb),
		t562IPv6Instance(id, t562SubnetC, t562IPv6C, true, t562SSHv6SG))

	t562RequireNoExposure(t, res, id)
	if reason, marked := res.TruncatedIDs[id]; marked {
		t.Errorf("%s marked not inspected (%q); its route table was on a complete list", id, reason)
	}
}

// The ec2 row names the instance's IPv6 address: the primary one when EC2
// reports it, else the first one on its interfaces.
func TestT562_EC2RowCarriesItsIPv6Address(t *testing.T) {
	const onIface, primary, none = "i-0t562v6nic000aaa1", "i-0t562v6pri000aaa1", "i-0t562v4only00aaa2"
	rs := pw1FetchEC2(t,
		t562IPv6Instance(onIface, t562SubnetB, t562IPv6B, false),
		t562IPv6Instance(primary, t562SubnetC, t562IPv6C, true),
		t562Instance(none, t562SubnetA, "203.0.113.41", "", false))

	for id, want := range map[string]string{onIface: t562IPv6B, primary: t562IPv6C, none: ""} {
		if got := pw1ResourceByID(t, rs, id).Fields["ipv6_address"]; got != want {
			t.Errorf("%s Fields[ipv6_address] = %q, want %q", id, got, want)
		}
	}
}

// Whether the subnet reaches an internet gateway is read off its route table;
// a list that was never loaded, or that stops before the subnet's table, cannot
// answer it, so the row reads not inspected rather than clean.
func TestT562_IPv6RouteUnknownReadsNotInspected(t *testing.T) {
	const id = "i-0t562v6unk000aaa1"
	instance := t562IPv6Instance(id, t562SubnetB, t562IPv6B, false, t562SSHv6SG)

	partial := t562RTBEntry(t, true, t562MainTable())
	cases := map[string]*resource.ResourceCacheEntry{
		"route tables never loaded": nil,
		// The main-table fallback is only true when no table on any page names
		// the subnet; an unread page can hold its explicit association.
		"route table list truncated before the subnet's table": &partial,
	}
	for name, rtb := range cases {
		t.Run(name, func(t *testing.T) {
			// A refused read may surface as the enricher's error; the row's
			// marker is what the operator sees either way.
			res, _ := awsclient.EnrichEC2InstanceStatus(context.Background(), //nolint:errcheck // the marker, not the error, is under test
				&awsclient.ServiceClients{EC2: &t562EC2EnrichFake{}}, pw1FetchEC2(t, instance), t562Cache(t, rtb))
			if _, marked := res.TruncatedIDs[id]; !marked {
				t.Errorf("%s not in TruncatedIDs; an undecided IPv6 route renders the row clean instead of \"?\"", id)
			}
			t562RequireNoExposure(t, res, id)
		})
	}
}

// A truncated list that already holds the subnet's explicit association has
// answered: a subnet has at most one explicit route-table association.
func TestT562_IPv6RouteDecidedOnATruncatedListThatHoldsTheSubnet(t *testing.T) {
	const id = "i-0t562v6dec000aaa1"
	rtb := t562RTBEntry(t, true, t562RouteTable(t562RTBB, t562SubnetB, t562IGWRoute()))
	res := t562EnrichEC2(t, t562Cache(t, &rtb),
		t562IPv6Instance(id, t562SubnetB, t562IPv6B, false, t562SSHv6SG))

	pw1RequireFinding(t, res.Findings[id], pw1EC2CodeInternetExposed,
		"port 22 reachable from the internet", domain.SevBroken, "wave2")
	if reason, marked := res.TruncatedIDs[id]; marked {
		t.Errorf("%s marked not inspected (%q); the subnet's own table was on the loaded page", id, reason)
	}
}

// Every group attached to a live instance exists, so a list that does not hold
// one was not read in full, whether or not it says so. The missing group may
// be the one that opens the instance; the loaded groups being closed says
// nothing about it.
func TestT562_AttachedGroupOffTheSGListReadsNotInspected(t *testing.T) {
	const (
		id   = "i-0t562sgpart00aaa1"
		sgA  = "sg-0t562closed0aaaa1"
		sgB  = "sg-0t562unread0aaaa1"
		want = "sg list incomplete"
	)
	instance := t562Instance(id, t562SubnetB, "203.0.113.42", "", false, sgA, sgB)
	// The subnet's table sends both families to the internet gateway, so the
	// route is decided and only the group list is left to answer.
	table := t562RouteTable(t562RTBB, t562SubnetB, t562IGWRoute())
	table.Routes = append(table.Routes, t562IGWv4Route())
	rtb := t562RTBEntry(t, false, table)

	onlyA := pw1SGCache(t, pw1ClosedSG(sgA))["sg"].Resources
	both := pw1SGCache(t, pw1ClosedSG(sgA), pw1ClosedSG(sgB))["sg"].Resources
	cases := []struct {
		name       string
		entry      resource.ResourceCacheEntry
		wantMarked bool
	}{
		{"truncated list without the group", resource.ResourceCacheEntry{Resources: onlyA, IsTruncated: true}, true},
		{"complete list without the group", resource.ResourceCacheEntry{Resources: onlyA, IsTruncated: false}, true},
		{"complete list holding both groups", resource.ResourceCacheEntry{Resources: both, IsTruncated: false}, false},
		{"truncated list holding both groups", resource.ResourceCacheEntry{Resources: both, IsTruncated: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := t562EnrichEC2(t, resource.ResourceCache{"sg": tc.entry, "rtb": rtb}, instance)
			got, marked := res.TruncatedIDs[id]
			switch {
			case tc.wantMarked && (!marked || got != want):
				t.Errorf("TruncatedIDs[%s] = %q (marked=%v), want %q", id, got, marked, want)
			case !tc.wantMarked && marked:
				t.Errorf("%s marked not inspected (%q); every attached group and the route were answered for", id, got)
			}
			t562RequireNoExposure(t, res, id)
		})
	}
}

// ─── CloudFront: every cache behaviour ─────────────────────────────────────

func t562Behaviour(path string, policy cftypes.ViewerProtocolPolicy) cftypes.CacheBehavior {
	return cftypes.CacheBehavior{
		PathPattern:          aws.String(path),
		TargetOriginId:       aws.String("origin-1"),
		ViewerProtocolPolicy: policy,
		Compress:             aws.Bool(true),
		CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
	}
}

func t562Distribution(behaviours ...cftypes.CacheBehavior) *cftypes.DistributionConfig {
	cfg := cfDistroConfigRedirectHTTPS(cfDistroID1)
	cfg.CacheBehaviors = &cftypes.CacheBehaviors{Quantity: aws.Int32(int32(len(behaviours))), Items: behaviours}
	return cfg
}

func t562EnrichCF(t *testing.T, cfg *cftypes.DistributionConfig) awsclient.IssueEnricherResult {
	t.Helper()
	fake := &cfGetDistributionConfigFake{results: map[string]*cftypes.DistributionConfig{cfDistroID1: cfg}}
	res, err := awsclient.EnrichCloudFrontDistribution(context.Background(), &awsclient.ServiceClients{CloudFront: fake}, cfDistroResources(cfDistroID1), nil)
	if err != nil {
		t.Fatalf("EnrichCloudFrontDistribution: %v", err)
	}
	return res
}

// A viewer reaching /api/* over plain HTTP is served over plain HTTP whatever
// the default behaviour says; the row names the path that allows it.
func TestT562_OrderedBehaviourAllowingHTTPIsReported(t *testing.T) {
	const code = domain.FindingCode("cf.insecure-protocol")
	res := t562EnrichCF(t, t562Distribution(
		t562Behaviour("/static/*", cftypes.ViewerProtocolPolicyHttpsOnly),
		t562Behaviour("/api/*", cftypes.ViewerProtocolPolicyAllowAll),
	))

	if !hasCode(res.Findings[cfDistroID1], code) {
		t.Fatalf("findings = %+v, want %s for the allow-all /api/* behaviour", res.Findings[cfDistroID1], code)
	}
	var values []string
	for _, row := range res.AttentionDetails[cfDistroID1][code].Rows {
		if row.Label == "Viewer protocol policy" {
			values = append(values, row.Value)
		}
	}
	if len(values) != 1 {
		t.Fatalf("\"Viewer protocol policy\" rows = %q, want exactly one", values)
	}
	if !strings.Contains(values[0], "/api/*") || !strings.Contains(values[0], "allow-all") {
		t.Errorf("Viewer protocol policy = %q, want it to name /api/* and allow-all", values[0])
	}
	if strings.Contains(values[0], "/static/*") {
		t.Errorf("Viewer protocol policy = %q names /static/*, which is https-only", values[0])
	}
}

func TestT562_OrderedBehavioursAllOnHTTPSReportNothing(t *testing.T) {
	res := t562EnrichCF(t, t562Distribution(
		t562Behaviour("/static/*", cftypes.ViewerProtocolPolicyHttpsOnly),
		t562Behaviour("/api/*", cftypes.ViewerProtocolPolicyRedirectToHttps),
	))
	if len(res.Findings[cfDistroID1]) != 0 {
		t.Errorf("findings = %+v, want none: every behaviour refuses plain HTTP", res.Findings[cfDistroID1])
	}
}

// ─── Trail: the log bucket judged by the S3 posture verdict ────────────────

const (
	t562TrailBucket = "acme-cloudtrail-logs"
	t562TrailName   = "acme-org-trail"
	t562OwnerID     = "c1f2e3d4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2"
	t562AllUsers    = "http://acs.amazonaws.com/groups/global/AllUsers"
	t562S3Public    = domain.FindingCode("s3.public")
)

// t562S3Fake answers every per-bucket read the S3 posture pass and the trail
// pass make; only the public-access block, the policy status and the ACL vary.
type t562S3Fake struct {
	awsclient.S3API
	pab          *s3types.PublicAccessBlockConfiguration
	policyPublic bool
	grants       []s3types.Grant
}

func (f *t562S3Fake) GetPublicAccessBlock(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	if f.pab == nil {
		return nil, &smithy.GenericAPIError{Code: "NoSuchPublicAccessBlockConfiguration", Message: "The public access block configuration was not found"}
	}
	return &s3.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: f.pab}, nil
}

func (f *t562S3Fake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	if !f.policyPublic {
		return nil, &smithy.GenericAPIError{Code: "NoSuchBucketPolicy", Message: "The bucket policy does not exist"}
	}
	return &s3.GetBucketPolicyStatusOutput{PolicyStatus: &s3types.PolicyStatus{IsPublic: aws.Bool(true)}}, nil
}

func (f *t562S3Fake) GetBucketAcl(_ context.Context, _ *s3.GetBucketAclInput, _ ...func(*s3.Options)) (*s3.GetBucketAclOutput, error) {
	return &s3.GetBucketAclOutput{
		Owner:  &s3types.Owner{ID: aws.String(t562OwnerID), DisplayName: aws.String("acme-ops")},
		Grants: f.grants,
	}, nil
}

func (f *t562S3Fake) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return &s3.GetBucketLoggingOutput{LoggingEnabled: &s3types.LoggingEnabled{
		TargetBucket: aws.String("acme-access-logs"),
		TargetPrefix: aws.String(t562TrailBucket + "/"),
	}}, nil
}

func (f *t562S3Fake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return &s3.GetBucketVersioningOutput{Status: s3types.BucketVersioningStatusEnabled, MFADelete: s3types.MFADeleteStatusEnabled}, nil
}

func (f *t562S3Fake) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return &s3.GetBucketLifecycleConfigurationOutput{Rules: []s3types.LifecycleRule{{
		ID:         aws.String("expire-old"),
		Status:     s3types.ExpirationStatusEnabled,
		Filter:     &s3types.LifecycleRuleFilter{Prefix: aws.String("")},
		Expiration: &s3types.LifecycleExpiration{Days: aws.Int32(365)},
	}}}, nil
}

func (f *t562S3Fake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return &s3.GetObjectLockConfigurationOutput{ObjectLockConfiguration: &s3types.ObjectLockConfiguration{ObjectLockEnabled: s3types.ObjectLockEnabledEnabled}}, nil
}

type t562TrailListFake struct{}

func (t562TrailListFake) DescribeTrails(_ context.Context, _ *cloudtrail.DescribeTrailsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
	return &cloudtrail.DescribeTrailsOutput{TrailList: []cttypes.Trail{{
		Name:                       aws.String(t562TrailName),
		TrailARN:                   aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/" + t562TrailName),
		HomeRegion:                 aws.String("us-east-1"),
		S3BucketName:               aws.String(t562TrailBucket),
		IsMultiRegionTrail:         aws.Bool(true),
		IsOrganizationTrail:        aws.Bool(false),
		IncludeGlobalServiceEvents: aws.Bool(true),
		LogFileValidationEnabled:   aws.Bool(true),
		KmsKeyId:                   aws.String("arn:aws:kms:us-east-1:123456789012:key/1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"),
	}}}, nil
}

func (t562TrailListFake) GetTrailStatus(_ context.Context, _ *cloudtrail.GetTrailStatusInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error) {
	return &cloudtrail.GetTrailStatusOutput{IsLogging: aws.Bool(true)}, nil
}

// t562Verdicts runs the trail pass and the S3 posture pass over one bucket and
// reports whether each called it public.
func t562Verdicts(t *testing.T, fake *t562S3Fake) (trailPublic, s3Public bool) {
	t.Helper()
	trails, err := awsclient.FetchCloudTrailTrails(context.Background(), t562TrailListFake{})
	if err != nil {
		t.Fatalf("FetchCloudTrailTrails: %v", err)
	}
	if len(trails) != 1 || trails[0].Fields["s3_bucket"] != t562TrailBucket {
		t.Fatalf("trail rows = %+v, want one delivering to %s", trails, t562TrailBucket)
	}
	clients := &awsclient.ServiceClients{S3: fake, Region: "us-east-1"}
	tr, err := awsclient.EnrichTrailLogBucket(context.Background(), clients, trails, nil)
	if err != nil {
		t.Fatalf("EnrichTrailLogBucket: %v", err)
	}
	if reason, marked := tr.TruncatedIDs[trails[0].ID]; marked {
		t.Fatalf("trail marked not inspected (%q); every read on its bucket answered", reason)
	}
	bucket := []resource.Resource{{ID: t562TrailBucket, Name: t562TrailBucket, Fields: map[string]string{"name": t562TrailBucket}}}
	sr, err := awsclient.EnrichS3Posture(context.Background(), clients, bucket, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture: %v", err)
	}
	return hasCode(tr.Findings[trails[0].ID], awsclient.CodeTrailLogBucketPublic), hasCode(sr.Findings[t562TrailBucket], t562S3Public)
}

// A grant to AllUsers opens the bucket with no policy at all unless the
// bucket's IgnorePublicAcls is on; the trail delivering there is exposed.
func TestT562_TrailLogBucketPublicByACLIsReported(t *testing.T) {
	trailPublic, _ := t562Verdicts(t, &t562S3Fake{
		pab: &s3types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(true),
			IgnorePublicAcls:      aws.Bool(false),
			BlockPublicPolicy:     aws.Bool(true),
			RestrictPublicBuckets: aws.Bool(true),
		},
		grants: []s3types.Grant{{
			Grantee:    &s3types.Grantee{Type: s3types.TypeGroup, URI: aws.String(t562AllUsers)},
			Permission: s3types.PermissionRead,
		}},
	})
	if !trailPublic {
		t.Errorf("trail delivering to an AllUsers-readable bucket carries no %s", awsclient.CodeTrailLogBucketPublic)
	}
}

// The trail and the bucket list answer "is this bucket public" identically for
// every route in and every setting that closes one.
func TestT562_TrailAndS3AgreeOnWhetherTheBucketIsPublic(t *testing.T) {
	allUsers := s3types.Grant{
		Grantee:    &s3types.Grantee{Type: s3types.TypeGroup, URI: aws.String(t562AllUsers)},
		Permission: s3types.PermissionRead,
	}
	owner := s3types.Grant{
		Grantee:    &s3types.Grantee{Type: s3types.TypeCanonicalUser, ID: aws.String(t562OwnerID), DisplayName: aws.String("acme-ops")},
		Permission: s3types.PermissionFullControl,
	}
	ignoreACLs := &s3types.PublicAccessBlockConfiguration{
		BlockPublicAcls:       aws.Bool(true),
		IgnorePublicAcls:      aws.Bool(true),
		BlockPublicPolicy:     aws.Bool(false),
		RestrictPublicBuckets: aws.Bool(false),
	}
	cases := []struct {
		name       string
		fake       *t562S3Fake
		wantPublic bool
	}{
		{"public by policy", &t562S3Fake{policyPublic: true, grants: []s3types.Grant{owner}}, true},
		{"public by ACL, no block", &t562S3Fake{grants: []s3types.Grant{owner, allUsers}}, true},
		{"ACL grant ignored by IgnorePublicAcls", &t562S3Fake{pab: ignoreACLs, grants: []s3types.Grant{owner, allUsers}}, false},
		{"owner-only ACL, no policy", &t562S3Fake{grants: []s3types.Grant{owner}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trailPublic, s3Public := t562Verdicts(t, tc.fake)
			if trailPublic != tc.wantPublic || s3Public != tc.wantPublic {
				t.Errorf("trail public = %v, s3 public = %v, want both %v", trailPublic, s3Public, tc.wantPublic)
			}
		})
	}
}
