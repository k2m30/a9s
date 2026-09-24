package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// --- route targets ---------------------------------------------------------

// A route goes blackhole when its target stops carrying traffic while the
// target itself can still be listed: an internet gateway detached from the
// VPC, a transit gateway whose VPC attachment was deleted, a NAT gateway in
// its "deleted" hour. The route table does not list such a gateway, so the
// gateway must not list the route table either.
func predRouteTable() resource.Resource {
	rtb := ec2types.RouteTable{
		RouteTableId: aws.String("rtb-0a1b2c3d4e5f60718"),
		VpcId:        aws.String("vpc-0a1b2c3d4e5f60718"),
		OwnerId:      aws.String("123456789012"),
		Routes: []ec2types.Route{
			{DestinationCidrBlock: aws.String("10.20.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRouteTable},
			{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String("igw-0live000000000001"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			{DestinationIpv6CidrBlock: aws.String("::/0"), GatewayId: aws.String("igw-0detached0000001"), State: ec2types.RouteStateBlackhole, Origin: ec2types.RouteOriginCreateRoute},
			{DestinationCidrBlock: aws.String("10.50.0.0/16"), GatewayId: aws.String("vgw-0onprem000000001"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			{DestinationCidrBlock: aws.String("10.40.0.0/16"), TransitGatewayId: aws.String("tgw-0live000000000001"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			{DestinationCidrBlock: aws.String("10.30.0.0/16"), TransitGatewayId: aws.String("tgw-0noattach0000001"), State: ec2types.RouteStateBlackhole, Origin: ec2types.RouteOriginCreateRoute},
			{DestinationCidrBlock: aws.String("172.16.0.0/12"), NatGatewayId: aws.String("nat-0live000000000001"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			{DestinationCidrBlock: aws.String("192.168.0.0/16"), NatGatewayId: aws.String("nat-0deleted00000001"), State: ec2types.RouteStateBlackhole, Origin: ec2types.RouteOriginCreateRoute},
		},
	}
	return resource.Resource{
		ID:        "rtb-0a1b2c3d4e5f60718",
		Name:      "acme-app-private",
		Fields:    map[string]string{"route_table_id": "rtb-0a1b2c3d4e5f60718", "vpc_id": "vpc-0a1b2c3d4e5f60718"},
		RawStruct: rtb,
	}
}

func predGateway(kind, id string) resource.Resource {
	var raw any
	switch kind {
	case "igw":
		igw := ec2types.InternetGateway{InternetGatewayId: aws.String(id), OwnerId: aws.String("123456789012")}
		if id == "igw-0live000000000001" {
			igw.Attachments = []ec2types.InternetGatewayAttachment{{VpcId: aws.String("vpc-0a1b2c3d4e5f60718"), State: ec2types.AttachmentStatusAttached}}
		}
		raw = igw
	case "tgw":
		raw = ec2types.TransitGateway{TransitGatewayId: aws.String(id), State: ec2types.TransitGatewayStateAvailable, OwnerId: aws.String("123456789012")}
	case "nat":
		state := ec2types.NatGatewayStateAvailable
		if id == "nat-0deleted00000001" {
			state = ec2types.NatGatewayStateDeleted
		}
		raw = ec2types.NatGateway{NatGatewayId: aws.String(id), State: state, VpcId: aws.String("vpc-0a1b2c3d4e5f60718")}
	}
	return resource.Resource{ID: id, Name: id, Fields: map[string]string{}, RawStruct: raw}
}

// TestRouteTargetsSkipBlackholeRoutesBothWays pins one rule for a route and
// the gateway it names, read from either end: an active route links the two,
// a blackhole route links neither, and a route to a virtual private gateway
// or "local" names no internet gateway.
func TestRouteTargetsSkipBlackholeRoutesBothWays(t *testing.T) {
	rtb := predRouteTable()
	cache := resource.ResourceCache{"rtb": {Resources: []resource.Resource{rtb}}}
	ctx := context.Background()

	for _, tc := range []struct {
		kind, live, dead string
	}{
		{"igw", "igw-0live000000000001", "igw-0detached0000001"},
		{"tgw", "tgw-0live000000000001", "tgw-0noattach0000001"},
		{"nat", "nat-0live000000000001", "nat-0deleted00000001"},
	} {
		fromTable := checkerByTarget(t, "rtb", tc.kind)(ctx, nil, rtb, nil)
		assertSameIDs(t, "rtb -> "+tc.kind, fromTable.ResourceIDs(), []string{tc.live})

		live := checkerByTarget(t, tc.kind, "rtb")(ctx, nil, predGateway(tc.kind, tc.live), cache)
		assertSameIDs(t, tc.kind+" "+tc.live+" -> rtb", live.ResourceIDs(), []string{rtb.ID})

		dead := checkerByTarget(t, tc.kind, "rtb")(ctx, nil, predGateway(tc.kind, tc.dead), cache)
		assertSameIDs(t, tc.kind+" "+tc.dead+" -> rtb (route is blackhole)", dead.ResourceIDs(), nil)
		if dead.EffectiveState() != domain.RelatedResolved {
			t.Errorf("%s %s -> rtb: state = %v, want a resolved zero", tc.kind, tc.dead, dead.EffectiveState())
		}
	}
}

// --- ENIs through the real fetcher -----------------------------------------

type predENIFake struct{ enis []ec2types.NetworkInterface }

func (f predENIFake) DescribeNetworkInterfaces(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return &ec2.DescribeNetworkInterfacesOutput{NetworkInterfaces: f.enis}, nil
}

func predFetchENIs(t *testing.T, enis ...ec2types.NetworkInterface) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchNetworkInterfacesPage(context.Background(), predENIFake{enis: enis}, "")
	if err != nil {
		t.Fatalf("FetchNetworkInterfacesPage: %v", err)
	}
	return out.Resources
}

func predENI(id, subnet string, typ ec2types.NetworkInterfaceType, desc, requester string, managed bool) ec2types.NetworkInterface {
	eni := ec2types.NetworkInterface{
		NetworkInterfaceId: aws.String(id),
		InterfaceType:      typ,
		Status:             ec2types.NetworkInterfaceStatusInUse,
		SubnetId:           aws.String(subnet),
		VpcId:              aws.String("vpc-0a1b2c3d4e5f60718"),
		AvailabilityZone:   aws.String("us-east-1a"),
		PrivateIpAddress:   aws.String("10.20.1.25"),
		OwnerId:            aws.String("123456789012"),
		Description:        aws.String(desc),
		RequesterManaged:   aws.Bool(managed),
	}
	if requester != "" {
		eni.RequesterId = aws.String(requester)
	}
	return eni
}

func predFunction(name string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(name),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name),
			PackageType:  lambdatypes.PackageTypeZip,
			VpcConfig: &lambdatypes.VpcConfigResponse{
				VpcId:     aws.String("vpc-0a1b2c3d4e5f60718"),
				SubnetIds: []string{"subnet-0aaa000000000001"},
			},
		},
	}
}

// TestLambdaENIsMatchOnInterfaceTypeBothWays pins the one test for "Lambda
// owns this ENI": EC2's InterfaceType "lambda". A Hyperplane ENI serves every
// function attached to its subnet with its exact set of security groups
// (docs.aws.amazon.com/lambda/latest/dg/configuration-vpc.html, "Other
// functions in your account that use the same subnet and security group
// combination can also use this ENI"), whichever one its description names.
// An ordinary ENI in the same subnet and group, whose description merely
// mentions Lambda, belongs to no function: a known zero rather than an unknown.
func TestLambdaENIsMatchOnInterfaceTypeBothWays(t *testing.T) {
	withGroups := func(eni ec2types.NetworkInterface, sgs ...string) ec2types.NetworkInterface {
		for _, sg := range sgs {
			eni.Groups = append(eni.Groups, ec2types.GroupIdentifier{GroupId: aws.String(sg), GroupName: aws.String(sg)})
		}
		return eni
	}
	enis := predFetchENIs(t,
		withGroups(predENI("eni-0lambda0000000001", "subnet-0aaa000000000001", ec2types.NetworkInterfaceTypeLambda,
			"AWS Lambda VPC ENI-orders-api-a1b2c3d4-5678-90ab-cdef-111111111111", "210987654321", true), "sg-0aaa000000000001"),
		withGroups(predENI("eni-0jumphost00000001", "subnet-0aaa000000000001", ec2types.NetworkInterfaceTypeInterface,
			"Lambda debugging jump host", "", false), "sg-0aaa000000000001"),
	)
	inGroups := func(fn resource.Resource, sgs ...string) resource.Resource {
		cfg := fn.RawStruct.(lambdatypes.FunctionConfiguration)
		vpc := *cfg.VpcConfig
		vpc.SecurityGroupIds = sgs
		cfg.VpcConfig = &vpc
		fn.RawStruct = cfg
		return fn
	}
	api := inGroups(predFunction("orders-api"), "sg-0aaa000000000001")
	orders := inGroups(predFunction("orders"), "sg-0aaa000000000001")
	billing := inGroups(predFunction("billing"), "sg-0aaa000000000001", "sg-0bbb000000000002")
	cache := resource.ResourceCache{
		"eni":    {Resources: enis},
		"lambda": {Resources: []resource.Resource{api, orders, billing}},
	}
	ctx := context.Background()
	byID := map[string]resource.Resource{}
	for _, r := range enis {
		byID[r.ID] = r
	}

	owned := checkerByTarget(t, "eni", "lambda")(ctx, nil, byID["eni-0lambda0000000001"], cache)
	assertSameIDs(t, "eni eni-0lambda0000000001 -> lambda", owned.ResourceIDs(), []string{"orders", "orders-api"})

	for _, tc := range []struct {
		fn   resource.Resource
		want []string
	}{
		{api, []string{"eni-0lambda0000000001"}},
		{orders, []string{"eni-0lambda0000000001"}},
		{billing, nil},
	} {
		got := checkerByTarget(t, "lambda", "eni")(ctx, nil, tc.fn, cache)
		assertSameIDs(t, "lambda "+tc.fn.ID+" -> eni", got.ResourceIDs(), tc.want)
	}

	plain := checkerByTarget(t, "eni", "lambda")(ctx, nil, byID["eni-0jumphost00000001"], cache)
	if plain.EffectiveState() != domain.RelatedResolved || len(plain.ResourceIDs()) != 0 {
		t.Errorf("eni eni-0jumphost00000001 -> lambda: state = %v, ids = %v; an ENI of InterfaceType interface belongs to no function, want a resolved zero",
			plain.EffectiveState(), plain.ResourceIDs())
	}
}

// TestEFSMountTargetENIsMatchOnFileSystemIDBothWays pins the one reading of a
// mount target ENI's Description: the file system ID in it, whole. EFS writes
// "Mount target fsmt-… for file system fs-…" (and consoles show "EFS mount
// target for fs-… (fsmt-…)"); either way the subnet lists the file system and
// the file system lists the subnet, and fs-9a13661e is not the prefix of
// fs-9a13661e0b1c2d3e4.
func TestEFSMountTargetENIsMatchOnFileSystemIDBothWays(t *testing.T) {
	enis := predFetchENIs(t,
		predENI("eni-0efs000000000001", "subnet-0aaa000000000001", ec2types.NetworkInterfaceTypeInterface,
			"Mount target fsmt-0123456789abcdef0 for file system fs-0123456789abcdef0", "EFS", true),
		predENI("eni-0efs000000000002", "subnet-0bbb000000000002", ec2types.NetworkInterfaceTypeInterface,
			"EFS mount target for fs-0fedcba9876543210 (fsmt-0fedcba9876543210)", "EFS", true),
		predENI("eni-0efs000000000003", "subnet-0ccc000000000003", ec2types.NetworkInterfaceTypeInterface,
			"Mount target fsmt-9a13661e0b1c2d3e4 for file system fs-9a13661e0b1c2d3e4", "EFS", true),
		predENI("eni-0efs000000000004", "subnet-0ddd000000000004", ec2types.NetworkInterfaceTypeInterface,
			"Mount target fsmt-9a13661e for file system fs-9a13661e", "EFS", true),
	)
	fsIDs := []string{"fs-0123456789abcdef0", "fs-0fedcba9876543210", "fs-9a13661e0b1c2d3e4", "fs-9a13661e"}
	var fileSystems []resource.Resource
	for _, id := range fsIDs {
		fileSystems = append(fileSystems, resource.Resource{
			ID: id, Name: id, Fields: map[string]string{},
			RawStruct: efstypes.FileSystemDescription{FileSystemId: aws.String(id), LifeCycleState: efstypes.LifeCycleStateAvailable},
		})
	}
	cache := resource.ResourceCache{"eni": {Resources: enis}, "efs": {Resources: fileSystems}}
	ctx := context.Background()

	pairs := map[string]string{
		"fs-0123456789abcdef0": "subnet-0aaa000000000001",
		"fs-0fedcba9876543210": "subnet-0bbb000000000002",
		"fs-9a13661e0b1c2d3e4": "subnet-0ccc000000000003",
		"fs-9a13661e":          "subnet-0ddd000000000004",
	}
	for fs, subnet := range pairs {
		fromFS := checkerByTarget(t, "efs", "subnet")(ctx, nil, resource.Resource{ID: fs, Name: fs}, cache)
		assertSameIDs(t, "efs "+fs+" -> subnet", fromFS.ResourceIDs(), []string{subnet})

		fromSubnet := checkerByTarget(t, "subnet", "efs")(ctx, nil, resource.Resource{ID: subnet, Name: subnet}, cache)
		assertSameIDs(t, "subnet "+subnet+" -> efs", fromSubnet.ResourceIDs(), []string{fs})
	}
}

// --- load balancer DNS names -----------------------------------------------

const (
	predALBDNS = "acme-web-alb-1234567890.us-east-1.elb.amazonaws.com"
	// Network load balancers answer under elb.<region>.amazonaws.com, not
	// <region>.elb.amazonaws.com.
	predNLBDNS = "acme-api-nlb-0123456789abcdef.elb.us-east-1.amazonaws.com"
)

func predLoadBalancer(name, dns string, typ elbv2types.LoadBalancerTypeEnum) resource.Resource {
	arnKind := "app"
	if typ == elbv2types.LoadBalancerTypeEnumNetwork {
		arnKind = "net"
	}
	arn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/" + arnKind + "/" + name + "/50dc6c495c0c9188"
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"name": name, "dns_name": dns, "type": string(typ), "load_balancer_arn": arn,
		},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String(name),
			LoadBalancerArn:  aws.String(arn),
			DNSName:          aws.String(dns),
			Type:             typ,
		},
	}
}

func predDistribution(id, domainName string, origins ...string) resource.Resource {
	var items []cftypes.Origin
	for i, o := range origins {
		items = append(items, cftypes.Origin{Id: aws.String(id + "-origin-" + string(rune('a'+i))), DomainName: aws.String(o)})
	}
	return resource.Resource{
		ID:     id,
		Name:   id,
		Fields: map[string]string{"domain_name": domainName},
		RawStruct: cftypes.DistributionSummary{
			Id:         aws.String(id),
			DomainName: aws.String(domainName),
			Origins:    &cftypes.Origins{Items: items, Quantity: aws.Int32(int32(len(items)))},
		},
	}
}

// TestLoadBalancerDNSMatchesCanonicallyEverywhere pins DNS-name equality for
// load balancers as one comparison, canonicalDNS, from every side that makes
// it: a CloudFront origin and the load balancer list each other whatever
// load balancer kind it is and whatever case the origin was typed in, and a
// hosted zone's alias finds a network load balancer as it finds an
// application one.
func TestLoadBalancerDNSMatchesCanonicallyEverywhere(t *testing.T) {
	alb := predLoadBalancer("acme-web-alb", predALBDNS, elbv2types.LoadBalancerTypeEnumApplication)
	nlb := predLoadBalancer("acme-api-nlb", predNLBDNS, elbv2types.LoadBalancerTypeEnumNetwork)
	webDist := predDistribution("E1WEB0000000AA", "d111111abcdef8.cloudfront.net", "Acme-Web-ALB-1234567890.us-east-1.elb.amazonaws.com")
	apiDist := predDistribution("E2API0000000BB", "d222222abcdef8.cloudfront.net", predNLBDNS)
	cache := resource.ResourceCache{
		"elb": {Resources: []resource.Resource{alb, nlb}},
		"cf":  {Resources: []resource.Resource{webDist, apiDist}},
	}
	ctx := context.Background()

	assertSameIDs(t, "elb acme-web-alb -> cf", checkerByTarget(t, "elb", "cf")(ctx, nil, alb, cache).ResourceIDs(), []string{"E1WEB0000000AA"})
	assertSameIDs(t, "elb acme-api-nlb -> cf", checkerByTarget(t, "elb", "cf")(ctx, nil, nlb, cache).ResourceIDs(), []string{"E2API0000000BB"})
	assertSameIDs(t, "cf E1WEB0000000AA -> elb", checkerByTarget(t, "cf", "elb")(ctx, nil, webDist, cache).ResourceIDs(), []string{"acme-web-alb"})
	assertSameIDs(t, "cf E2API0000000BB -> elb", checkerByTarget(t, "cf", "elb")(ctx, nil, apiDist, cache).ResourceIDs(), []string{"acme-api-nlb"})

	zoneClients := &awsclient.ServiceClients{Route53: &nilCacheR53Fake{records: []r53types.ResourceRecordSet{
		aliasRecord("www.acme.example.", "dualstack."+predALBDNS+"."),
		aliasRecord("api.acme.example.", "dualstack."+predNLBDNS+"."),
	}}}
	zone := checkerByTarget(t, "r53", "elb")(ctx, zoneClients, r53Zone(), cache)
	assertSameIDs(t, "r53 acme.example -> elb", zone.ResourceIDs(), []string{"acme-web-alb", "acme-api-nlb"})
}
