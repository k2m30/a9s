package unit_test

import (
	"strings"
	"testing"
	"time"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/v3/internal/config"
)

// ---------------------------------------------------------------------------
// Config builder for types that may not be in this worktree's defaults.go.
// If configForType returns a config without the type's detail paths, we
// build a custom config here.
// ---------------------------------------------------------------------------

func detailConfigForType(typeName string) *config.ViewsConfig {
	cfg := configForType(typeName)
	vd := config.GetViewDef(cfg, typeName)
	if len(vd.Detail) > 0 {
		return cfg
	}
	// Build custom config for types not yet in defaults.go
	detailPaths := map[string][]string{
		"subnet": {
			"SubnetId", "VpcId", "CidrBlock", "AvailabilityZone",
			"State", "AvailableIpAddressCount", "MapPublicIpOnLaunch", "Tags",
		},
		"rtb": {
			"RouteTableId", "VpcId", "Routes", "Associations", "Tags",
		},
		"nat": {
			"NatGatewayId", "VpcId", "SubnetId", "State",
			"ConnectivityType", "NatGatewayAddresses", "CreateTime", "Tags",
		},
		"igw": {
			"InternetGatewayId", "Attachments", "OwnerId", "Tags",
		},
		"eip": {
			"AllocationId", "PublicIp", "AssociationId", "InstanceId",
			"Domain", "PrivateIpAddress", "Tags",
		},
		"tgw": {
			"TransitGatewayId", "State", "OwnerId", "Description",
			"Options", "CreationTime", "Tags",
		},
		"vpce": {
			"VpcEndpointId", "ServiceName", "VpcEndpointType",
			"State", "VpcId", "CreationTimestamp", "Tags",
		},
		"eni": {
			"NetworkInterfaceId", "Status", "InterfaceType",
			"VpcId", "SubnetId", "PrivateIpAddress",
			"MacAddress", "Description", "Groups", "TagSet",
		},
		"rds-snap": {
			"DBSnapshotIdentifier", "DBInstanceIdentifier",
			"Status", "Engine", "EngineVersion", "SnapshotType",
			"SnapshotCreateTime", "AllocatedStorage",
		},
		"docdb-snap": {
			"DBClusterSnapshotIdentifier", "DBClusterIdentifier",
			"Status", "Engine", "SnapshotType", "SnapshotCreateTime",
		},
		"sns-sub": {
			"SubscriptionArn", "TopicArn", "Protocol", "Endpoint", "Owner",
		},
		"policy": {
			"PolicyName", "PolicyId", "Arn", "Path",
			"AttachmentCount", "CreateDate", "Description",
		},
	}
	paths, ok := detailPaths[typeName]
	if !ok {
		return cfg
	}
	return &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			typeName: {Detail: paths},
		},
	}
}

// ===========================================================================
// Realistic SDK struct builders
// ===========================================================================

func realisticVPC() ec2types.Vpc {
	return ec2types.Vpc{
		VpcId:     new("vpc-0abc1234def56789a"),
		CidrBlock: new("10.0.0.0/16"),
		State:     ec2types.VpcStateAvailable,
		IsDefault: new(false),
		OwnerId:   new("123456789012"),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-vpc")},
			{Key: new("env"), Value: new("production")},
		},
	}
}

func realisticSecurityGroup() ec2types.SecurityGroup {
	return ec2types.SecurityGroup{
		GroupId:     new("sg-0abc1234def56789a"),
		GroupName:   new("web-sg"),
		VpcId:       new("vpc-0abc1234"),
		Description: new("Web server security group"),
		OwnerId:     new("123456789012"),
		IpPermissions: []ec2types.IpPermission{
			{
				FromPort:   new(int32(443)),
				ToPort:     new(int32(443)),
				IpProtocol: new("tcp"),
				IpRanges: []ec2types.IpRange{
					{CidrIp: new("0.0.0.0/0"), Description: new("HTTPS from anywhere")},
				},
			},
		},
		IpPermissionsEgress: []ec2types.IpPermission{
			{
				IpProtocol: new("-1"),
				IpRanges:   []ec2types.IpRange{{CidrIp: new("0.0.0.0/0")}},
			},
		},
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("web-sg")},
		},
	}
}

func realisticNodeGroup() ekstypes.Nodegroup {
	return ekstypes.Nodegroup{
		NodegroupName: new("prod-ng-01"),
		ClusterName:   new("prod-cluster"),
		Status:        ekstypes.NodegroupStatusActive,
		InstanceTypes: []string{"t3.large", "t3.xlarge"},
		AmiType:       ekstypes.AMITypesAl2X8664,
		CapacityType:  ekstypes.CapacityTypesOnDemand,
		DiskSize:      new(int32(100)),
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: new(int32(3)),
			MinSize:     new(int32(1)),
			MaxSize:     new(int32(5)),
		},
		NodeRole: new("arn:aws:iam::123456789012:role/eks-node-role"),
		Subnets:  []string{"subnet-0abc1234", "subnet-0def5678"},
		Tags:     map[string]string{"env": "production"},
	}
}

func realisticSubnet() ec2types.Subnet {
	return ec2types.Subnet{
		SubnetId:                new("subnet-0abc1234def56789a"),
		VpcId:                   new("vpc-0abc1234"),
		CidrBlock:               new("10.0.1.0/24"),
		AvailabilityZone:        new("us-east-1a"),
		State:                   ec2types.SubnetStateAvailable,
		AvailableIpAddressCount: new(int32(251)),
		MapPublicIpOnLaunch:     new(true),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("public-subnet-1a")},
		},
	}
}

func realisticRouteTable() ec2types.RouteTable {
	return ec2types.RouteTable{
		RouteTableId: new("rtb-0abc1234def56789a"),
		VpcId:        new("vpc-0abc1234"),
		Routes: []ec2types.Route{
			{DestinationCidrBlock: new("10.0.0.0/16"), GatewayId: new("local")},
			{DestinationCidrBlock: new("0.0.0.0/0"), GatewayId: new("igw-0abc1234")},
		},
		Associations: []ec2types.RouteTableAssociation{
			{
				RouteTableAssociationId: new("rtbassoc-0abc1234"),
				SubnetId:                new("subnet-0abc1234"),
				Main:                    new(false),
			},
		},
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("public-rtb")},
		},
	}
}

func realisticNATGateway() ec2types.NatGateway {
	return ec2types.NatGateway{
		NatGatewayId:     new("nat-0abc1234def56789a"),
		VpcId:            new("vpc-0abc1234"),
		SubnetId:         new("subnet-0abc1234"),
		State:            ec2types.NatGatewayStateAvailable,
		ConnectivityType: ec2types.ConnectivityTypePublic,
		NatGatewayAddresses: []ec2types.NatGatewayAddress{
			{
				AllocationId:       new("eipalloc-0abc1234"),
				PublicIp:           new("54.123.45.67"),
				PrivateIp:          new("10.0.1.100"),
				NetworkInterfaceId: new("eni-0abc1234"),
			},
		},
		CreateTime: new(testTime),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-nat")},
		},
	}
}

func realisticInternetGateway() ec2types.InternetGateway {
	return ec2types.InternetGateway{
		InternetGatewayId: new("igw-0abc1234def56789a"),
		Attachments: []ec2types.InternetGatewayAttachment{
			{VpcId: new("vpc-0abc1234"), State: ec2types.AttachmentStatusAttached},
		},
		OwnerId: new("123456789012"),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-igw")},
		},
	}
}

func realisticEIP() ec2types.Address {
	return ec2types.Address{
		AllocationId:     new("eipalloc-0abc1234def56789a"),
		PublicIp:         new("54.123.45.67"),
		AssociationId:    new("eipassoc-0abc1234"),
		InstanceId:       new("i-0abc1234"),
		Domain:           ec2types.DomainTypeVpc,
		PrivateIpAddress: new("10.0.1.42"),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-eip")},
		},
	}
}

func realisticTransitGateway() ec2types.TransitGateway {
	return ec2types.TransitGateway{
		TransitGatewayId: new("tgw-0abc1234def56789a"),
		State:            ec2types.TransitGatewayStateAvailable,
		OwnerId:          new("123456789012"),
		Description:      new("Production transit gateway"),
		Options: &ec2types.TransitGatewayOptions{
			AutoAcceptSharedAttachments:  ec2types.AutoAcceptSharedAttachmentsValueEnable,
			DefaultRouteTableAssociation: ec2types.DefaultRouteTableAssociationValueEnable,
			DnsSupport:                   ec2types.DnsSupportValueEnable,
			VpnEcmpSupport:               ec2types.VpnEcmpSupportValueEnable,
		},
		CreationTime: new(testTime),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-tgw")},
		},
	}
}

func realisticVPCEndpoint() ec2types.VpcEndpoint {
	return ec2types.VpcEndpoint{
		VpcEndpointId:     new("vpce-0abc1234def56789a"),
		ServiceName:       new("com.amazonaws.us-east-1.s3"),
		VpcEndpointType:   ec2types.VpcEndpointTypeGateway,
		State:             ec2types.StateAvailable,
		VpcId:             new("vpc-0abc1234"),
		CreationTimestamp: new(testTime),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("s3-endpoint")},
		},
	}
}

func realisticENI() ec2types.NetworkInterface {
	return ec2types.NetworkInterface{
		NetworkInterfaceId: new("eni-0abc1234def56789a"),
		Status:             ec2types.NetworkInterfaceStatusInUse,
		InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
		VpcId:              new("vpc-0abc1234"),
		SubnetId:           new("subnet-0abc1234"),
		PrivateIpAddress:   new("10.0.1.42"),
		MacAddress:         new("02:ab:cd:ef:12:34"),
		Description:        new("Primary network interface"),
		Groups: []ec2types.GroupIdentifier{
			{GroupId: new("sg-0abc1234"), GroupName: new("web-sg")},
		},
		TagSet: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-eni")},
		},
	}
}

func realisticRDSSnapshot() rdstypes.DBSnapshot {
	return rdstypes.DBSnapshot{
		DBSnapshotIdentifier: new("rds-snap-prod-20250615"),
		DBInstanceIdentifier: new("prod-db-01"),
		Status:               new("available"),
		Engine:               new("mysql"),
		EngineVersion:        new("8.0.35"),
		SnapshotType:         new("automated"),
		SnapshotCreateTime:   new(testTime),
		AllocatedStorage:     new(int32(100)),
	}
}

func realisticDocDBSnapshot() docdbtypes.DBClusterSnapshot {
	return docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: new("docdb-snap-prod-20250615"),
		DBClusterIdentifier:         new("docdb-prod-cluster"),
		Status:                      new("available"),
		Engine:                      new("docdb"),
		SnapshotType:                new("automated"),
		SnapshotCreateTime:          new(testTime),
	}
}

func realisticSNSSubscription() snstypes.Subscription {
	return snstypes.Subscription{
		SubscriptionArn: new("arn:aws:sns:us-east-1:123456789012:alerts:a1b2c3d4-5678-90ab-cdef-EXAMPLE11111"),
		TopicArn:        new("arn:aws:sns:us-east-1:123456789012:alerts"),
		Protocol:        new("email"),
		Endpoint:        new("user@example.com"),
		Owner:           new("123456789012"),
	}
}

// realisticManagedPolicyDetail is deprecated; use realisticIAMPolicy instead.
// Kept for backward compatibility with TestQA_Detail_Policy_NilFields which
// uses iamtypes.ManagedPolicyDetail{} directly.
func realisticManagedPolicyDetail() iamtypes.ManagedPolicyDetail {
	return iamtypes.ManagedPolicyDetail{
		PolicyName:      new("ReadOnlyAccess"),
		PolicyId:        new("ANPAI1234567890EXAMPLE"),
		Arn:             new("arn:aws:iam::123456789012:policy/ReadOnlyAccess"),
		Path:            new("/"),
		AttachmentCount: new(int32(5)),
		CreateDate:      new(testTime),
		Description:     new("Provides read-only access"),
	}
}

// realisticIAMPolicy returns an iamtypes.Policy matching the type produced by
// internal/aws/iam_policies.go FetchIAMPolicies (which uses ListPolicies API).
func realisticIAMPolicy() iamtypes.Policy {
	return iamtypes.Policy{
		PolicyName:      new("ReadOnlyAccess"),
		PolicyId:        new("ANPAI1234567890EXAMPLE"),
		Arn:             new("arn:aws:iam::123456789012:policy/ReadOnlyAccess"),
		Path:            new("/"),
		AttachmentCount: new(int32(5)),
		CreateDate:      new(testTime),
		Description:     new("Provides read-only access"),
	}
}

// ===========================================================================
// 1. VPC
// ===========================================================================

func TestQA_Detail_VPC_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	vpc := realisticVPC()
	res := buildResource("vpc-0abc1234def56789a", "prod-vpc", vpc)
	cfg := detailConfigForType("vpc")
	m := newDetailModel(res, "vpc", cfg)

	view := m.View()
	for _, expected := range []string{
		"VpcId", "vpc-0abc1234def56789a",
		"CidrBlock", "10.0.0.0/16",
		"State", "available",
		"IsDefault", "No",
		"OwnerId", "123456789012",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("VPC detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_VPC_NilFields(t *testing.T) {
	ensureNoColor(t)
	vpc := ec2types.Vpc{}
	res := buildResource("empty-vpc", "empty-vpc", vpc)
	cfg := detailConfigForType("vpc")
	m := newDetailModel(res, "vpc", cfg)

	view := m.View()
	if view == "" {
		t.Error("VPC detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_VPC_FrameTitle(t *testing.T) {
	vpc := realisticVPC()
	res := buildResource("vpc-0abc1234def56789a", "prod-vpc", vpc)
	cfg := detailConfigForType("vpc")
	m := newDetailModel(res, "vpc", cfg)

	if title := m.FrameTitle(); title != "prod-vpc" {
		t.Errorf("VPC FrameTitle expected %q, got %q", "prod-vpc", title)
	}
}

// ===========================================================================
// 2. SG
// ===========================================================================

func TestQA_Detail_SG_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	sg := realisticSecurityGroup()
	res := buildResource("sg-0abc1234def56789a", "web-sg", sg)
	cfg := detailConfigForType("sg")
	m := newDetailModel(res, "sg", cfg)

	view := m.View()
	for _, expected := range []string{
		"GroupId", "sg-0abc1234def56789a",
		"GroupName", "web-sg",
		"VpcId", "vpc-0abc1234",
		"Description", "Web server security group",
		"OwnerId", "123456789012",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("SG detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_SG_NilFields(t *testing.T) {
	ensureNoColor(t)
	sg := ec2types.SecurityGroup{}
	res := buildResource("empty-sg", "empty-sg", sg)
	cfg := detailConfigForType("sg")
	m := newDetailModel(res, "sg", cfg)

	view := m.View()
	if view == "" {
		t.Error("SG detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_SG_FrameTitle(t *testing.T) {
	sg := realisticSecurityGroup()
	res := buildResource("sg-0abc1234def56789a", "web-sg", sg)
	cfg := detailConfigForType("sg")
	m := newDetailModel(res, "sg", cfg)

	if title := m.FrameTitle(); title != "web-sg" {
		t.Errorf("SG FrameTitle expected %q, got %q", "web-sg", title)
	}
}

// ===========================================================================
// 3. NG (Node Group)
// ===========================================================================

func TestQA_Detail_NG_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ng := realisticNodeGroup()
	res := buildResource("prod-ng-01", "prod-ng-01", ng)
	cfg := detailConfigForType("ng")
	m := newDetailModel(res, "ng", cfg)

	view := m.View()
	for _, expected := range []string{
		"NodegroupName", "prod-ng-01",
		"ClusterName", "prod-cluster",
		"Status", "ACTIVE",
		"CapacityType", "ON_DEMAND",
		"DiskSize", "100",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("NG detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_NG_NilFields(t *testing.T) {
	ensureNoColor(t)
	ng := ekstypes.Nodegroup{}
	res := buildResource("empty-ng", "empty-ng", ng)
	cfg := detailConfigForType("ng")
	m := newDetailModel(res, "ng", cfg)

	view := m.View()
	if view == "" {
		t.Error("NG detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_NG_FrameTitle(t *testing.T) {
	ng := realisticNodeGroup()
	res := buildResource("prod-ng-01", "prod-ng-01", ng)
	cfg := detailConfigForType("ng")
	m := newDetailModel(res, "ng", cfg)

	if title := m.FrameTitle(); title != "prod-ng-01" {
		t.Errorf("NG FrameTitle expected %q, got %q", "prod-ng-01", title)
	}
}

// ===========================================================================
// 4. Subnet
// ===========================================================================

func TestQA_Detail_Subnet_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	subnet := realisticSubnet()
	res := buildResource("subnet-0abc1234def56789a", "public-subnet-1a", subnet)
	cfg := detailConfigForType("subnet")
	m := newDetailModel(res, "subnet", cfg)

	view := m.View()
	for _, expected := range []string{
		"SubnetId", "subnet-0abc1234def56789a",
		"VpcId", "vpc-0abc1234",
		"CidrBlock", "10.0.1.0/24",
		"AvailabilityZone", "us-east-1a",
		"State", "available",
		"AvailableIpAddressCo", "251",
		"MapPublicIpOnLaunch", "Yes",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Subnet detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Subnet_NilFields(t *testing.T) {
	ensureNoColor(t)
	subnet := ec2types.Subnet{}
	res := buildResource("empty-subnet", "empty-subnet", subnet)
	cfg := detailConfigForType("subnet")
	m := newDetailModel(res, "subnet", cfg)

	view := m.View()
	if view == "" {
		t.Error("Subnet detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_Subnet_FrameTitle(t *testing.T) {
	subnet := realisticSubnet()
	res := buildResource("subnet-0abc1234def56789a", "public-subnet-1a", subnet)
	cfg := detailConfigForType("subnet")
	m := newDetailModel(res, "subnet", cfg)

	if title := m.FrameTitle(); title != "public-subnet-1a" {
		t.Errorf("Subnet FrameTitle expected %q, got %q", "public-subnet-1a", title)
	}
}

// ===========================================================================
// 5. RTB (Route Table)
// ===========================================================================

func TestQA_Detail_RTB_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	rtb := realisticRouteTable()
	res := buildResource("rtb-0abc1234def56789a", "public-rtb", rtb)
	cfg := detailConfigForType("rtb")
	m := newDetailModel(res, "rtb", cfg)

	view := m.View()
	for _, expected := range []string{
		"RouteTableId", "rtb-0abc1234def56789a",
		"VpcId", "vpc-0abc1234",
		"Routes",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("RTB detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_RTB_NilFields(t *testing.T) {
	ensureNoColor(t)
	rtb := ec2types.RouteTable{}
	res := buildResource("empty-rtb", "empty-rtb", rtb)
	cfg := detailConfigForType("rtb")
	m := newDetailModel(res, "rtb", cfg)

	view := m.View()
	if view == "" {
		t.Error("RTB detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_RTB_FrameTitle(t *testing.T) {
	rtb := realisticRouteTable()
	res := buildResource("rtb-0abc1234def56789a", "public-rtb", rtb)
	cfg := detailConfigForType("rtb")
	m := newDetailModel(res, "rtb", cfg)

	if title := m.FrameTitle(); title != "public-rtb" {
		t.Errorf("RTB FrameTitle expected %q, got %q", "public-rtb", title)
	}
}

// ===========================================================================
// 6. NAT (NAT Gateway)
// ===========================================================================

func TestQA_Detail_NAT_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	nat := realisticNATGateway()
	res := buildResource("nat-0abc1234def56789a", "prod-nat", nat)
	cfg := detailConfigForType("nat")
	m := newDetailModel(res, "nat", cfg)

	view := m.View()
	for _, expected := range []string{
		"NatGatewayId", "nat-0abc1234def56789a",
		"VpcId", "vpc-0abc1234",
		"SubnetId", "subnet-0abc1234",
		"State", "available",
		"ConnectivityType", "public",
		"CreateTime", "2025-06-15 10:30",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("NAT detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_NAT_NilFields(t *testing.T) {
	ensureNoColor(t)
	nat := ec2types.NatGateway{}
	res := buildResource("empty-nat", "empty-nat", nat)
	cfg := detailConfigForType("nat")
	m := newDetailModel(res, "nat", cfg)

	view := m.View()
	if view == "" {
		t.Error("NAT detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_NAT_FrameTitle(t *testing.T) {
	nat := realisticNATGateway()
	res := buildResource("nat-0abc1234def56789a", "prod-nat", nat)
	cfg := detailConfigForType("nat")
	m := newDetailModel(res, "nat", cfg)

	if title := m.FrameTitle(); title != "prod-nat" {
		t.Errorf("NAT FrameTitle expected %q, got %q", "prod-nat", title)
	}
}

// ===========================================================================
// 7. IGW (Internet Gateway)
// ===========================================================================

func TestQA_Detail_IGW_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	igw := realisticInternetGateway()
	res := buildResource("igw-0abc1234def56789a", "prod-igw", igw)
	cfg := detailConfigForType("igw")
	m := newDetailModel(res, "igw", cfg)

	view := m.View()
	for _, expected := range []string{
		"InternetGatewayId", "igw-0abc1234def56789a",
		"OwnerId", "123456789012",
		"Attachments",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("IGW detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_IGW_NilFields(t *testing.T) {
	ensureNoColor(t)
	igw := ec2types.InternetGateway{}
	res := buildResource("empty-igw", "empty-igw", igw)
	cfg := detailConfigForType("igw")
	m := newDetailModel(res, "igw", cfg)

	view := m.View()
	if view == "" {
		t.Error("IGW detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_IGW_FrameTitle(t *testing.T) {
	igw := realisticInternetGateway()
	res := buildResource("igw-0abc1234def56789a", "prod-igw", igw)
	cfg := detailConfigForType("igw")
	m := newDetailModel(res, "igw", cfg)

	if title := m.FrameTitle(); title != "prod-igw" {
		t.Errorf("IGW FrameTitle expected %q, got %q", "prod-igw", title)
	}
}

// ===========================================================================
// 8. EIP (Elastic IP)
// ===========================================================================

func TestQA_Detail_EIP_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	eip := realisticEIP()
	res := buildResource("eipalloc-0abc1234def56789a", "prod-eip", eip)
	cfg := detailConfigForType("eip")
	m := newDetailModel(res, "eip", cfg)

	view := m.View()
	for _, expected := range []string{
		"AllocationId", "eipalloc-0abc1234def56789a",
		"PublicIp", "54.123.45.67",
		"AssociationId", "eipassoc-0abc1234",
		"InstanceId", "i-0abc1234",
		"Domain", "vpc",
		"PrivateIpAddress", "10.0.1.42",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("EIP detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_EIP_NilFields(t *testing.T) {
	ensureNoColor(t)
	eip := ec2types.Address{}
	res := buildResource("empty-eip", "empty-eip", eip)
	cfg := detailConfigForType("eip")
	m := newDetailModel(res, "eip", cfg)

	view := m.View()
	if view == "" {
		t.Error("EIP detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_EIP_FrameTitle(t *testing.T) {
	eip := realisticEIP()
	res := buildResource("eipalloc-0abc1234def56789a", "prod-eip", eip)
	cfg := detailConfigForType("eip")
	m := newDetailModel(res, "eip", cfg)

	if title := m.FrameTitle(); title != "prod-eip" {
		t.Errorf("EIP FrameTitle expected %q, got %q", "prod-eip", title)
	}
}

// ===========================================================================
// 9. TGW (Transit Gateway)
// ===========================================================================

func TestQA_Detail_TGW_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	tgw := realisticTransitGateway()
	res := buildResource("tgw-0abc1234def56789a", "prod-tgw", tgw)
	cfg := detailConfigForType("tgw")
	m := newDetailModel(res, "tgw", cfg)

	view := m.View()
	for _, expected := range []string{
		"TransitGatewayId", "tgw-0abc1234def56789a",
		"State", "available",
		"OwnerId", "123456789012",
		"Description", "Production transit gateway",
		"CreationTime", "2025-06-15 10:30",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("TGW detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_TGW_NilFields(t *testing.T) {
	ensureNoColor(t)
	tgw := ec2types.TransitGateway{}
	res := buildResource("empty-tgw", "empty-tgw", tgw)
	cfg := detailConfigForType("tgw")
	m := newDetailModel(res, "tgw", cfg)

	view := m.View()
	if view == "" {
		t.Error("TGW detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_TGW_FrameTitle(t *testing.T) {
	tgw := realisticTransitGateway()
	res := buildResource("tgw-0abc1234def56789a", "prod-tgw", tgw)
	cfg := detailConfigForType("tgw")
	m := newDetailModel(res, "tgw", cfg)

	if title := m.FrameTitle(); title != "prod-tgw" {
		t.Errorf("TGW FrameTitle expected %q, got %q", "prod-tgw", title)
	}
}

// ===========================================================================
// 10. VPCE (VPC Endpoint)
// ===========================================================================

func TestQA_Detail_VPCE_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	vpce := realisticVPCEndpoint()
	res := buildResource("vpce-0abc1234def56789a", "s3-endpoint", vpce)
	cfg := detailConfigForType("vpce")
	m := newDetailModel(res, "vpce", cfg)

	view := m.View()
	for _, expected := range []string{
		"VpcEndpointId", "vpce-0abc1234def56789a",
		"ServiceName", "com.amazonaws.us-east-1.s3",
		"VpcEndpointType", "Gateway",
		"VpcId", "vpc-0abc1234",
		"CreationTimestamp", "2025-06-15 10:30",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("VPCE detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_VPCE_NilFields(t *testing.T) {
	ensureNoColor(t)
	vpce := ec2types.VpcEndpoint{}
	res := buildResource("empty-vpce", "empty-vpce", vpce)
	cfg := detailConfigForType("vpce")
	m := newDetailModel(res, "vpce", cfg)

	view := m.View()
	if view == "" {
		t.Error("VPCE detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_VPCE_FrameTitle(t *testing.T) {
	vpce := realisticVPCEndpoint()
	res := buildResource("vpce-0abc1234def56789a", "s3-endpoint", vpce)
	cfg := detailConfigForType("vpce")
	m := newDetailModel(res, "vpce", cfg)

	if title := m.FrameTitle(); title != "s3-endpoint" {
		t.Errorf("VPCE FrameTitle expected %q, got %q", "s3-endpoint", title)
	}
}

// ===========================================================================
// 11. ENI (Network Interface)
// ===========================================================================

func TestQA_Detail_ENI_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	eni := realisticENI()
	res := buildResource("eni-0abc1234def56789a", "prod-eni", eni)
	cfg := detailConfigForType("eni")
	m := newDetailModel(res, "eni", cfg)

	view := m.View()
	for _, expected := range []string{
		"NetworkInterfaceId", "eni-0abc1234def56789a",
		"Status", "in-use",
		"InterfaceType", "interface",
		"VpcId", "vpc-0abc1234",
		"SubnetId", "subnet-0abc1234",
		"PrivateIpAddress", "10.0.1.42",
		"MacAddress", "02:ab:cd:ef:12:34",
		"Description", "Primary network interface",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ENI detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ENI_NilFields(t *testing.T) {
	ensureNoColor(t)
	eni := ec2types.NetworkInterface{}
	res := buildResource("empty-eni", "empty-eni", eni)
	cfg := detailConfigForType("eni")
	m := newDetailModel(res, "eni", cfg)

	view := m.View()
	if view == "" {
		t.Error("ENI detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_ENI_FrameTitle(t *testing.T) {
	eni := realisticENI()
	res := buildResource("eni-0abc1234def56789a", "prod-eni", eni)
	cfg := detailConfigForType("eni")
	m := newDetailModel(res, "eni", cfg)

	if title := m.FrameTitle(); title != "prod-eni" {
		t.Errorf("ENI FrameTitle expected %q, got %q", "prod-eni", title)
	}
}

// ===========================================================================
// 12. RDS Snapshot
// ===========================================================================

func TestQA_Detail_RDSSnap_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	snap := realisticRDSSnapshot()
	res := buildResource("rds-snap-prod-20250615", "rds-snap-prod-20250615", snap)
	cfg := detailConfigForType("rds-snap")
	m := newDetailModel(res, "rds-snap", cfg)

	view := m.View()
	for _, expected := range []string{
		"DBSnapshotIdentifier", "rds-snap-prod-20250615",
		"DBInstanceIdentifier", "prod-db-01",
		"Status", "available",
		"Engine", "mysql",
		"EngineVersion", "8.0.35",
		"SnapshotType", "automated",
		"SnapshotCreateTime", "2025-06-15 10:30",
		"AllocatedStorage", "100",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("RDSSnap detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_RDSSnap_NilFields(t *testing.T) {
	ensureNoColor(t)
	snap := rdstypes.DBSnapshot{}
	res := buildResource("empty-snap", "empty-snap", snap)
	cfg := detailConfigForType("rds-snap")
	m := newDetailModel(res, "rds-snap", cfg)

	view := m.View()
	if view == "" {
		t.Error("RDSSnap detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_RDSSnap_FrameTitle(t *testing.T) {
	snap := realisticRDSSnapshot()
	res := buildResource("rds-snap-prod-20250615", "rds-snap-prod-20250615", snap)
	cfg := detailConfigForType("rds-snap")
	m := newDetailModel(res, "rds-snap", cfg)

	if title := m.FrameTitle(); title != "rds-snap-prod-20250615" {
		t.Errorf("RDSSnap FrameTitle expected %q, got %q", "rds-snap-prod-20250615", title)
	}
}

// ===========================================================================
// 13. DocDB Snapshot
// ===========================================================================

func TestQA_Detail_DocDBSnap_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	snap := realisticDocDBSnapshot()
	res := buildResource("docdb-snap-prod-20250615", "docdb-snap-prod-20250615", snap)
	cfg := detailConfigForType("docdb-snap")
	m := newDetailModel(res, "docdb-snap", cfg)

	view := m.View()
	for _, expected := range []string{
		"DBClusterSnapshotId", "docdb-snap-prod-20250615",
		"DBClusterIdentifier", "docdb-prod-cluster",
		"Status", "available",
		"Engine", "docdb",
		"SnapshotType", "automated",
		"SnapshotCreateTime", "2025-06-15 10:30",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("DocDBSnap detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_DocDBSnap_NilFields(t *testing.T) {
	ensureNoColor(t)
	snap := docdbtypes.DBClusterSnapshot{}
	res := buildResource("empty-docdb-snap", "empty-docdb-snap", snap)
	cfg := detailConfigForType("docdb-snap")
	m := newDetailModel(res, "docdb-snap", cfg)

	view := m.View()
	if view == "" {
		t.Error("DocDBSnap detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_DocDBSnap_FrameTitle(t *testing.T) {
	snap := realisticDocDBSnapshot()
	res := buildResource("docdb-snap-prod-20250615", "docdb-snap-prod-20250615", snap)
	cfg := detailConfigForType("docdb-snap")
	m := newDetailModel(res, "docdb-snap", cfg)

	if title := m.FrameTitle(); title != "docdb-snap-prod-20250615" {
		t.Errorf("DocDBSnap FrameTitle expected %q, got %q", "docdb-snap-prod-20250615", title)
	}
}

// ===========================================================================
// 14. SNS Subscription
// ===========================================================================

func TestQA_Detail_SNSSub_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	sub := realisticSNSSubscription()
	res := buildResource("arn:aws:sns:us-east-1:123456789012:alerts:a1b2c3d4-5678-90ab-cdef-EXAMPLE11111", "alerts-sub", sub)
	cfg := detailConfigForType("sns-sub")
	m := newDetailModel(res, "sns-sub", cfg)

	view := m.View()
	for _, expected := range []string{
		"TopicArn", "arn:aws:sns:us-east-1:123456789012:alerts",
		"Protocol", "email",
		"Endpoint", "user@example.com",
		"Owner", "123456789012",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("SNSSub detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_SNSSub_NilFields(t *testing.T) {
	ensureNoColor(t)
	sub := snstypes.Subscription{}
	res := buildResource("empty-sns-sub", "empty-sns-sub", sub)
	cfg := detailConfigForType("sns-sub")
	m := newDetailModel(res, "sns-sub", cfg)

	view := m.View()
	if view == "" {
		t.Error("SNSSub detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_SNSSub_FrameTitle(t *testing.T) {
	sub := realisticSNSSubscription()
	res := buildResource("arn:aws:sns:us-east-1:123456789012:alerts:a1b2c3d4", "alerts-sub", sub)
	cfg := detailConfigForType("sns-sub")
	m := newDetailModel(res, "sns-sub", cfg)

	if title := m.FrameTitle(); title != "alerts-sub" {
		t.Errorf("SNSSub FrameTitle expected %q, got %q", "alerts-sub", title)
	}
}

// ===========================================================================
// 15. IAM Policy (ManagedPolicyDetail)
// ===========================================================================

func TestQA_Detail_Policy_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	policy := realisticManagedPolicyDetail()
	res := buildResource("ANPAI1234567890EXAMPLE", "ReadOnlyAccess", policy)
	cfg := detailConfigForType("policy")
	m := newDetailModel(res, "policy", cfg)

	view := m.View()
	for _, expected := range []string{
		"PolicyName", "ReadOnlyAccess",
		"PolicyId", "ANPAI1234567890EXAMPLE",
		"Arn", "arn:aws:iam::123456789012:policy/ReadOnlyAccess",
		"Path", "/",
		"AttachmentCount", "5",
		"CreateDate", "2025-06-15 10:30",
		"Description", "Provides read-only access",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Policy detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Policy_NilFields(t *testing.T) {
	ensureNoColor(t)
	policy := iamtypes.ManagedPolicyDetail{}
	res := buildResource("empty-policy", "empty-policy", policy)
	cfg := detailConfigForType("policy")
	m := newDetailModel(res, "policy", cfg)

	view := m.View()
	if view == "" {
		t.Error("Policy detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_Policy_FrameTitle(t *testing.T) {
	policy := realisticManagedPolicyDetail()
	res := buildResource("ANPAI1234567890EXAMPLE", "ReadOnlyAccess", policy)
	cfg := detailConfigForType("policy")
	m := newDetailModel(res, "policy", cfg)

	if title := m.FrameTitle(); title != "ReadOnlyAccess" {
		t.Errorf("Policy FrameTitle expected %q, got %q", "ReadOnlyAccess", title)
	}
}

// ===========================================================================
// Suppress "imported and not used" for time package (used by testTime)
// ===========================================================================
var _ = time.Now
