// Package fixtures provides EC2 fixture data for the EC2 fake.
package fixtures

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// EC2Fixtures holds all EC2 domain objects served by the fake.
// Data is populated from the existing demo category files via demo/client.go.
type EC2Fixtures struct {
	Reservations      []ec2types.Reservation
	InstanceStatuses  []ec2types.InstanceStatus
	Vpcs              []ec2types.Vpc
	SecurityGroups    []ec2types.SecurityGroup
	Subnets           []ec2types.Subnet
	RouteTables       []ec2types.RouteTable
	NatGateways       []ec2types.NatGateway
	InternetGateways  []ec2types.InternetGateway
	Addresses         []ec2types.Address
	TransitGateways   []ec2types.TransitGateway
	TGWAttachments    []ec2types.TransitGatewayAttachment
	VpcEndpoints      []ec2types.VpcEndpoint
	NetworkInterfaces []ec2types.NetworkInterface
	Volumes           []ec2types.Volume
	Snapshots         []ec2types.Snapshot
	Images            []ec2types.Image
}

// shared constants (mirrors internal/demo/constants_shared.go — no import allowed)
const (
	fixtProdVPCID               = "vpc-0abc123def456789a"
	fixtStagingVPCID            = "vpc-0def456789abc123d"
	fixtProdPublicSubnetA       = "subnet-0aaa111111111111a"
	fixtProdPublicSubnetB       = "subnet-0bbb222222222222b"
	fixtProdPrivateSubnetA      = "subnet-0ccc333333333333c"
	fixtProdPrivateSubnetB      = "subnet-0ddd444444444444d"
	fixtStagingSubnetA          = "subnet-0eee555555555555e"
	fixtStagingSubnetB          = "subnet-0fff666666666666f"
	fixtProdWebALBSGID          = "sg-0aaa111111111111a"
	fixtProdAPIInternalSGID     = "sg-0bbb222222222222b"
	fixtProdRDSSGID             = "sg-0ccc333333333333c"
	fixtProdDBProxySGID         = "sg-0ddd444444444444d"
	fixtProdAMIID1              = "ami-0a1b2c3d4e5f60001"
	fixtProdAMIID2              = "ami-0a1b2c3d4e5f60002"
	fixtProdAMIID3              = "ami-0a1b2c3d4e5f60003"
	fixtProdInstanceProfileARN  = "arn:aws:iam::123456789012:instance-profile/acme-ec2-instance-profile"
	fixtProdEKSClusterName      = "acme-prod-eks"
	fixtRelatedEC2NGNodeGroupID = "acme-node-group-01"
)

// NewEC2Fixtures builds and returns a fully-populated EC2Fixtures struct
// with deterministic demo data that matches the data served by the old demo code paths.
// This is the single source of truth for all EC2 fake responses.
func NewEC2Fixtures() *EC2Fixtures {
	f := &EC2Fixtures{}
	f.Reservations = buildReservations()
	f.InstanceStatuses = buildInstanceStatuses(f.Reservations)
	f.Vpcs = buildVpcs()
	f.SecurityGroups = buildSecurityGroups()
	f.Subnets = buildSubnets()
	f.RouteTables = buildRouteTables()
	f.NatGateways = buildNatGateways()
	f.InternetGateways = buildInternetGateways()
	f.Addresses = buildAddresses()
	f.TransitGateways = buildTransitGateways()
	f.TGWAttachments = buildTGWAttachments()
	f.VpcEndpoints = buildVpcEndpoints()
	f.NetworkInterfaces = buildNetworkInterfaces()
	f.Volumes = buildVolumes()
	f.Snapshots = buildSnapshots()
	f.Images = buildImages()
	return f
}

// ---------------------------------------------------------------------------
// EC2 Instances
// ---------------------------------------------------------------------------

type instExtras struct {
	imageID        string
	keyName        string
	architecture   ec2types.ArchitectureValues
	az             string
	securityGroups []ec2types.GroupIdentifier
}

var namedExtras = map[string]instExtras{
	"i-0a1b2c3d4e5f60001": {
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			{GroupId: aws.String(fixtProdAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0a1b2c3d4e5f60002": {
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			{GroupId: aws.String(fixtProdAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0a1b2c3d4e5f60003": {
		imageID: fixtProdAMIID2, keyName: "acme-staging-keypair",
		architecture: ec2types.ArchitectureValuesArm64, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0a1b2c3d4e5f60004": {
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdRDSSGID), GroupName: aws.String("acme-worker-sg")},
		},
	},
	"i-0a1b2c3d4e5f60005": {
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
		},
	},
	"i-0a1b2c3d4e5f60006": {
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1c",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdDBProxySGID), GroupName: aws.String("acme-db-proxy-sg")},
		},
	},
	"i-0a1b2c3d4e5f60007": {
		imageID: fixtProdAMIID2, keyName: "acme-staging-keypair",
		architecture: ec2types.ArchitectureValuesArm64, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0a1b2c3d4e5f60008": {
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdAPIInternalSGID), GroupName: aws.String("acme-api-internal-sg")},
		},
	},
	"i-0a1b2c3d4e5f60009": {
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdRDSSGID), GroupName: aws.String("acme-worker-sg")},
		},
	},
	"i-0a1b2c3d4e5f60010": {
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
}

func defaultExtras(instanceID string) instExtras {
	if ex, ok := namedExtras[instanceID]; ok {
		return ex
	}
	suffix := instanceID[len(instanceID)-4:]
	amiIDs := []string{fixtProdAMIID1, fixtProdAMIID2, fixtProdAMIID3}
	keyNames := []string{"acme-prod-keypair", "acme-staging-keypair", "acme-dev-keypair"}
	archs := []ec2types.ArchitectureValues{ec2types.ArchitectureValuesX8664, ec2types.ArchitectureValuesArm64}
	azs := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	idx := 0
	for _, ch := range suffix {
		idx += int(ch)
	}
	return instExtras{
		imageID:      amiIDs[idx%len(amiIDs)],
		keyName:      keyNames[idx%len(keyNames)],
		architecture: archs[idx%len(archs)],
		az:           azs[idx%len(azs)],
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
		},
	}
}

func stateCode(name ec2types.InstanceStateName) int32 {
	switch name {
	case ec2types.InstanceStateNamePending:
		return 0
	case ec2types.InstanceStateNameRunning:
		return 16
	case ec2types.InstanceStateNameShuttingDown:
		return 32
	case ec2types.InstanceStateNameTerminated:
		return 48
	case ec2types.InstanceStateNameStopping:
		return 64
	case ec2types.InstanceStateNameStopped:
		return 80
	default:
		return -1
	}
}

func privateDNS(ip string) string {
	var dashed strings.Builder
	for _, ch := range ip {
		if ch == '.' {
			dashed.WriteString("-")
		} else {
			dashed.WriteString(string(ch))
		}
	}
	return "ip-" + dashed.String() + ".ec2.internal"
}

func volumeIDForInstance(instanceID string) string {
	volIDs := []string{
		"vol-0a1b2c3d4e5f60001",
		"vol-0a1b2c3d4e5f60002",
		"vol-0a1b2c3d4e5f60003",
		"vol-0a1b2c3d4e5f60005",
	}
	idx := 0
	for _, ch := range instanceID {
		idx += int(ch)
	}
	return volIDs[idx%len(volIDs)]
}

func makeInstance(
	instanceID, name, state string,
	instanceType ec2types.InstanceType,
	privateIP, publicIP string,
	vpcID, subnetID string,
	launchTime time.Time,
	lifecycle ec2types.InstanceLifecycleType,
) ec2types.Instance {
	stateName := ec2types.InstanceStateName(state)
	code := stateCode(stateName)
	extras := defaultExtras(instanceID)

	inst := ec2types.Instance{
		InstanceId:       aws.String(instanceID),
		InstanceType:     instanceType,
		PrivateIpAddress: aws.String(privateIP),
		State: &ec2types.InstanceState{
			Name: stateName,
			Code: aws.Int32(code),
		},
		VpcId:          aws.String(vpcID),
		SubnetId:       aws.String(subnetID),
		ImageId:        aws.String(extras.imageID),
		KeyName:        aws.String(extras.keyName),
		Architecture:   extras.architecture,
		Placement:      &ec2types.Placement{AvailabilityZone: aws.String(extras.az)},
		SecurityGroups: extras.securityGroups,
		IamInstanceProfile: &ec2types.IamInstanceProfile{
			Arn: aws.String(fixtProdInstanceProfileARN),
		},
		BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/xvda"),
				Ebs: &ec2types.EbsInstanceBlockDevice{
					VolumeId: aws.String(volumeIDForInstance(instanceID)),
					Status:   ec2types.AttachmentStatusAttached,
				},
			},
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String(name)},
		},
		LaunchTime:        aws.Time(launchTime),
		InstanceLifecycle: lifecycle,
		EbsOptimized:      aws.Bool(true),
		MetadataOptions: &ec2types.InstanceMetadataOptionsResponse{
			State:                   ec2types.InstanceMetadataOptionsStateApplied,
			HttpEndpoint:            ec2types.InstanceMetadataEndpointStateEnabled,
			HttpTokens:              ec2types.HttpTokensStateRequired,
			HttpPutResponseHopLimit: aws.Int32(2),
		},
		PrivateDnsName: aws.String(privateDNS(privateIP)),
	}
	if instanceID == "i-0a1b2c3d4e5f60001" {
		inst.Platform = ec2types.PlatformValuesWindows
		inst.Tags = append(inst.Tags, ec2types.Tag{
			Key:   aws.String("kubernetes.io/cluster/" + fixtProdEKSClusterName),
			Value: aws.String("owned"),
		})
	}
	if instanceID == "i-0a1b2c3d4e5f60003" {
		inst.Tags = append(inst.Tags,
			ec2types.Tag{Key: aws.String("eks:cluster-name"), Value: aws.String(fixtProdEKSClusterName)},
			ec2types.Tag{Key: aws.String("eks:nodegroup-name"), Value: aws.String(fixtRelatedEC2NGNodeGroupID)},
		)
	}
	if publicIP != "" {
		inst.PublicIpAddress = aws.String(publicIP)
	}
	return inst
}

func buildReservations() []ec2types.Reservation {
	type instSpec struct {
		id, name, state string
		itype           ec2types.InstanceType
		privateIP       string
		publicIP        string
		vpcID           string
		subnetID        string
		launchTime      time.Time
		lifecycle       ec2types.InstanceLifecycleType
	}

	subnets := []string{
		fixtProdPublicSubnetA, fixtProdPublicSubnetB, fixtProdPrivateSubnetA,
		fixtProdPrivateSubnetB, fixtStagingSubnetA, fixtStagingSubnetB,
	}
	generatedTypes := []ec2types.InstanceType{
		ec2types.InstanceTypeT3Medium, ec2types.InstanceTypeM5Large,
		ec2types.InstanceTypeC5Large, ec2types.InstanceTypeR5Large,
		ec2types.InstanceTypeT3Large, ec2types.InstanceTypeM5Xlarge,
		ec2types.InstanceTypeC5Xlarge, ec2types.InstanceTypeT3Small,
		ec2types.InstanceTypeR5Xlarge, ec2types.InstanceTypeT3Micro,
		ec2types.InstanceTypeM5Large, ec2types.InstanceTypeC5Large,
		ec2types.InstanceTypeT3Medium, ec2types.InstanceTypeR5Large,
		ec2types.InstanceTypeT3Large,
	}
	namePool := []string{"worker", "api", "web", "db", "cache", "monitor", "proxy", "queue", "search", "batch"}
	statePool := []string{"running", "stopped", "running", "running", "stopped"}

	named := []instSpec{
		{"i-0a1b2c3d4e5f60001", "web-prod-01", "running", ec2types.InstanceTypeT3Large, "10.0.1.10", "54.210.33.112", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 11, 15, 8, 30, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60002", "web-prod-02", "running", ec2types.InstanceTypeT3Large, "10.0.1.11", "54.210.33.113", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 11, 15, 8, 32, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60003", "api-staging-01", "running", ec2types.InstanceTypeM5Xlarge, "10.0.2.50", "", fixtProdVPCID, fixtProdPublicSubnetB, time.Date(2026, 1, 20, 14, 15, 0, 0, time.UTC), ec2types.InstanceLifecycleTypeSpot},
		{"i-0a1b2c3d4e5f60004", "worker-batch-03", "stopped", ec2types.InstanceTypeC5Xlarge, "10.0.3.100", "", fixtProdVPCID, fixtProdPrivateSubnetA, time.Date(2025, 9, 5, 11, 0, 0, 0, time.UTC), ec2types.InstanceLifecycleTypeSpot},
		{"i-0a1b2c3d4e5f60005", "bastion-prod", "running", ec2types.InstanceTypeT3Micro, "10.0.0.5", "52.87.221.44", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60006", "db-proxy-01", "running", ec2types.InstanceTypeR5Large, "10.0.4.200", "", fixtProdVPCID, fixtProdPrivateSubnetB, time.Date(2025, 12, 10, 18, 45, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60007", "web-staging-01", "pending", ec2types.InstanceTypeT3Medium, "10.0.2.70", "", fixtProdVPCID, fixtProdPublicSubnetB, time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC), ec2types.InstanceLifecycleTypeSpot},
		{"i-0a1b2c3d4e5f60008", "ml-trainer-gpu", "stopping", ec2types.InstanceTypeG4dnXlarge, "10.0.5.30", "", fixtProdVPCID, fixtStagingSubnetA, time.Date(2026, 2, 14, 22, 0, 0, 0, time.UTC), ec2types.InstanceLifecycleTypeSpot},
		{"i-0a1b2c3d4e5f60009", "temp-load-test", "shutting-down", ec2types.InstanceTypeC5Large, "10.0.3.55", "", fixtProdVPCID, fixtProdPrivateSubnetA, time.Date(2026, 3, 20, 16, 30, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60010", "old-migration-worker", "terminated", ec2types.InstanceTypeT3Small, "", "", fixtProdVPCID, fixtProdPublicSubnetB, time.Date(2025, 8, 1, 12, 0, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60030", "dev-sandbox-01", "stopped", ec2types.InstanceTypeT3Medium, "10.1.0.20", "", fixtStagingVPCID, fixtStagingSubnetA, time.Date(2025, 10, 12, 7, 0, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60031", "dev-sandbox-02", "stopped", ec2types.InstanceTypeT3Small, "10.1.0.21", "", fixtStagingVPCID, fixtStagingSubnetB, time.Date(2025, 10, 12, 7, 5, 0, 0, time.UTC), ""},
	}

	var reservations []ec2types.Reservation
	for _, s := range named {
		inst := makeInstance(s.id, s.name, s.state, s.itype, s.privateIP, s.publicIP, s.vpcID, s.subnetID, s.launchTime, s.lifecycle)
		reservations = append(reservations, ec2types.Reservation{
			ReservationId: aws.String("r-" + s.id[2:]),
			OwnerId:       aws.String("123456789012"),
			Instances:     []ec2types.Instance{inst},
		})
	}

	for i := range 15 {
		idx := i + 11
		name := fmt.Sprintf("%s-%02d", namePool[i%len(namePool)], idx)
		state := statePool[i%len(statePool)]
		ip := fmt.Sprintf("10.0.%d.%d", (idx/10)+1, 10+idx)
		publicIP := ""
		if i%5 == 0 {
			publicIP = fmt.Sprintf("54.210.%d.%d", 34+i, 100+i)
		}
		instanceID := fmt.Sprintf("i-0a1b2c3d4e5f6%04d", idx)
		inst := makeInstance(
			instanceID, name, state,
			generatedTypes[i],
			ip, publicIP,
			fixtProdVPCID,
			subnets[i%len(subnets)],
			time.Date(2025, time.Month(6+(i%7)), 1+i, 8+i, 0, 0, 0, time.UTC),
			"",
		)
		reservations = append(reservations, ec2types.Reservation{
			ReservationId: aws.String(fmt.Sprintf("r-gen%04d", idx)),
			OwnerId:       aws.String("123456789012"),
			Instances:     []ec2types.Instance{inst},
		})
	}

	return reservations
}

// buildInstanceStatuses derives InstanceStatus records from reservations.
// Only running instances get status checks (matching the live enrichment logic).
func buildInstanceStatuses(reservations []ec2types.Reservation) []ec2types.InstanceStatus {
	// Named instance status checks (indices 0-9 in reservations)
	namedStatuses := map[string][2]string{
		"i-0a1b2c3d4e5f60001": {"ok", "ok"},
		"i-0a1b2c3d4e5f60002": {"ok", "ok"},
		"i-0a1b2c3d4e5f60003": {"ok", "impaired"},
		"i-0a1b2c3d4e5f60005": {"ok", "ok"},
		"i-0a1b2c3d4e5f60006": {"initializing", "initializing"},
	}

	var statuses []ec2types.InstanceStatus
	for _, r := range reservations {
		for _, inst := range r.Instances {
			if inst.State == nil || inst.State.Name != ec2types.InstanceStateNameRunning {
				continue
			}
			id := aws.ToString(inst.InstanceId)
			systemStatus := "ok"
			instanceStatus := "ok"
			if s, ok := namedStatuses[id]; ok {
				systemStatus = s[0]
				instanceStatus = s[1]
			}
			statuses = append(statuses, ec2types.InstanceStatus{
				InstanceId:       aws.String(id),
				AvailabilityZone: inst.Placement.AvailabilityZone,
				InstanceState:    inst.State,
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatus(systemStatus),
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatus(instanceStatus),
				},
			})
		}
	}
	return statuses
}

// ---------------------------------------------------------------------------
// VPCs
// ---------------------------------------------------------------------------

func buildVpcs() []ec2types.Vpc {
	return []ec2types.Vpc{
		{
			VpcId:           aws.String(fixtProdVPCID),
			CidrBlock:       aws.String("10.0.0.0/16"),
			State:           ec2types.VpcStateAvailable,
			IsDefault:       aws.Bool(false),
			InstanceTenancy: ec2types.TenancyDefault,
			DhcpOptionsId:   aws.String("dopt-0abc123def456789a"),
			OwnerId:         aws.String("123456789012"),
			CidrBlockAssociationSet: []ec2types.VpcCidrBlockAssociation{
				{
					AssociationId: aws.String("vpc-cidr-assoc-01"),
					CidrBlock:     aws.String("10.0.0.0/16"),
					CidrBlockState: &ec2types.VpcCidrBlockState{
						State: ec2types.VpcCidrBlockStateCodeAssociated,
					},
				},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-prod")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			VpcId:           aws.String(fixtStagingVPCID),
			CidrBlock:       aws.String("10.1.0.0/16"),
			State:           ec2types.VpcStateAvailable,
			IsDefault:       aws.Bool(false),
			InstanceTenancy: ec2types.TenancyDefault,
			DhcpOptionsId:   aws.String("dopt-0def456789abc123d"),
			OwnerId:         aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-staging")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		{
			VpcId:           aws.String("vpc-0default00000000"),
			CidrBlock:       aws.String("172.31.0.0/16"),
			State:           ec2types.VpcStateAvailable,
			IsDefault:       aws.Bool(true),
			InstanceTenancy: ec2types.TenancyDefault,
			DhcpOptionsId:   aws.String("dopt-0default0000000"),
			OwnerId:         aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("default")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Security Groups
// ---------------------------------------------------------------------------

func buildSecurityGroups() []ec2types.SecurityGroup {
	sgs := []ec2types.SecurityGroup{
		{
			GroupId:          aws.String("sg-0aaa111111111111a"),
			GroupName:        aws.String("acme-web-alb-sg"),
			VpcId:            aws.String(fixtProdVPCID),
			Description:      aws.String("Security group for production web ALB"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aaa111111111111a"),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(443),
					ToPort:     aws.Int32(443),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("HTTPS from anywhere")}},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(80),
					ToPort:     aws.Int32(80),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("HTTP from anywhere (redirect)")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-web-alb-sg")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			GroupId:     aws.String("sg-0bbb222222222222b"),
			GroupName:   aws.String("acme-api-internal-sg"),
			VpcId:       aws.String(fixtProdVPCID),
			Description: aws.String("Internal API service security group"),
			OwnerId:     aws.String("123456789012"),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(8080),
					ToPort:     aws.Int32(8080),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("API traffic from VPC")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-api-internal-sg")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			GroupId:     aws.String("sg-0ccc333333333333c"),
			GroupName:   aws.String("acme-rds-sg"),
			VpcId:       aws.String(fixtProdVPCID),
			Description: aws.String("RDS PostgreSQL access from app tier"),
			OwnerId:     aws.String("123456789012"),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(5432),
					ToPort:     aws.Int32(5432),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("PostgreSQL from VPC")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-rds-sg")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			GroupId:     aws.String("sg-0eee555555555555e"),
			GroupName:   aws.String("acme-bastion-sg"),
			VpcId:       aws.String(fixtProdVPCID),
			Description: aws.String("Bastion host SSH access"),
			OwnerId:     aws.String("123456789012"),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(22),
					ToPort:     aws.Int32(22),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("203.0.113.0/24"), Description: aws.String("Office VPN")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-bastion-sg")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			GroupId:     aws.String("sg-0fff888888888888f"),
			GroupName:   aws.String("staging-default-sg"),
			VpcId:       aws.String(fixtStagingVPCID),
			Description: aws.String("Default staging VPC security group"),
			OwnerId:     aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-default-sg")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
	}

	vpcIDs := []string{fixtProdVPCID, fixtProdVPCID, fixtProdVPCID, fixtStagingVPCID}
	sgNames := []string{"app-sg", "cache-sg", "worker-sg", "monitoring-sg", "lambda-sg", "batch-sg", "data-sg", "analytics-sg", "admin-sg", "internal-sg"}
	sgDescs := []string{"Application tier", "Cache tier", "Worker tier", "Monitoring", "Lambda functions", "Batch jobs", "Data pipeline", "Analytics", "Admin access", "Internal services"}
	for i := range 20 {
		sgID := fmt.Sprintf("sg-0gen%016x", i+100)
		name := sgNames[i%len(sgNames)]
		desc := sgDescs[i%len(sgDescs)]
		vpcID := vpcIDs[i%len(vpcIDs)]
		env := "prod"
		if vpcID == fixtStagingVPCID {
			env = "staging"
		}
		sgs = append(sgs, ec2types.SecurityGroup{
			GroupId:     aws.String(sgID),
			GroupName:   aws.String(name),
			VpcId:       aws.String(vpcID),
			Description: aws.String(desc),
			OwnerId:     aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String(name)},
				{Key: aws.String("Environment"), Value: aws.String(env)},
			},
		})
	}
	return sgs
}

// ---------------------------------------------------------------------------
// Subnets
// ---------------------------------------------------------------------------

func buildSubnets() []ec2types.Subnet {
	named := []ec2types.Subnet{
		{
			SubnetId:                aws.String(fixtProdPublicSubnetA),
			VpcId:                   aws.String(fixtProdVPCID),
			CidrBlock:               aws.String("10.0.1.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			AvailabilityZoneId:      aws.String("use1-az1"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(243),
			MapPublicIpOnLaunch:     aws.Bool(true),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + fixtProdPublicSubnetA),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-public-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("public")},
			},
		},
		{
			SubnetId:                aws.String(fixtProdPublicSubnetB),
			VpcId:                   aws.String(fixtProdVPCID),
			CidrBlock:               aws.String("10.0.2.0/24"),
			AvailabilityZone:        aws.String("us-east-1b"),
			AvailabilityZoneId:      aws.String("use1-az2"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(248),
			MapPublicIpOnLaunch:     aws.Bool(true),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-public-1b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("public")},
			},
		},
		{
			SubnetId:                aws.String(fixtProdPrivateSubnetA),
			VpcId:                   aws.String(fixtProdVPCID),
			CidrBlock:               aws.String("10.0.3.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			AvailabilityZoneId:      aws.String("use1-az1"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(200),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-private-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("private")},
			},
		},
		{
			SubnetId:                aws.String(fixtProdPrivateSubnetB),
			VpcId:                   aws.String(fixtProdVPCID),
			CidrBlock:               aws.String("10.0.4.0/24"),
			AvailabilityZone:        aws.String("us-east-1b"),
			AvailabilityZoneId:      aws.String("use1-az2"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(200),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-private-1b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("private")},
			},
		},
		{
			SubnetId:                aws.String(fixtStagingSubnetA),
			VpcId:                   aws.String(fixtStagingVPCID),
			CidrBlock:               aws.String("10.1.1.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			AvailabilityZoneId:      aws.String("use1-az1"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(240),
			MapPublicIpOnLaunch:     aws.Bool(true),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-public-1a")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		{
			SubnetId:                aws.String(fixtStagingSubnetB),
			VpcId:                   aws.String(fixtStagingVPCID),
			CidrBlock:               aws.String("10.1.2.0/24"),
			AvailabilityZone:        aws.String("us-east-1b"),
			AvailabilityZoneId:      aws.String("use1-az2"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(240),
			MapPublicIpOnLaunch:     aws.Bool(true),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-public-1b")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
	}

	vpcPool := []string{fixtProdVPCID, fixtProdVPCID, fixtStagingVPCID}
	azPool := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	for i := range 16 {
		subnetID := fmt.Sprintf("subnet-0gen%016x", i+100)
		vpcID := vpcPool[i%len(vpcPool)]
		az := azPool[i%len(azPool)]
		cidr := fmt.Sprintf("10.%d.%d.0/24", (i/8)+2, i+10)
		env := "prod"
		if vpcID == fixtStagingVPCID {
			env = "staging"
		}
		named = append(named, ec2types.Subnet{
			SubnetId:                aws.String(subnetID),
			VpcId:                   aws.String(vpcID),
			CidrBlock:               aws.String(cidr),
			AvailabilityZone:        aws.String(az),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(int32(200 + i)),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("gen-subnet-%02d", i+1))},
				{Key: aws.String("Environment"), Value: aws.String(env)},
			},
		})
	}
	return named
}

// ---------------------------------------------------------------------------
// Route Tables
// ---------------------------------------------------------------------------

func buildRouteTables() []ec2types.RouteTable {
	return []ec2types.RouteTable{
		{
			RouteTableId: aws.String("rtb-0aaa111111111111a"),
			VpcId:        aws.String(fixtProdVPCID),
			OwnerId:      aws.String("123456789012"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRouteTable},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), NatGatewayId: aws.String("nat-0aaa111111111111a"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			},
			Associations: []ec2types.RouteTableAssociation{
				{Main: aws.Bool(true), RouteTableAssociationId: aws.String("rtbassoc-0aaa111111111111a"), RouteTableId: aws.String("rtb-0aaa111111111111a")},
				{Main: aws.Bool(false), RouteTableAssociationId: aws.String("rtbassoc-0aaa222222222222a"), RouteTableId: aws.String("rtb-0aaa111111111111a"), SubnetId: aws.String(fixtProdPrivateSubnetA)},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-main")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			RouteTableId: aws.String("rtb-0bbb222222222222b"),
			VpcId:        aws.String(fixtProdVPCID),
			OwnerId:      aws.String("123456789012"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRouteTable},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String("igw-0aaa111111111111a"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
				{DestinationCidrBlock: aws.String("10.1.0.0/16"), NatGatewayId: aws.String("nat-0aaa111111111111a"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			},
			Associations: []ec2types.RouteTableAssociation{
				{Main: aws.Bool(false), RouteTableAssociationId: aws.String("rtbassoc-0bbb222222222222b"), RouteTableId: aws.String("rtb-0bbb222222222222b"), SubnetId: aws.String(fixtProdPublicSubnetA)},
				{Main: aws.Bool(false), RouteTableAssociationId: aws.String("rtbassoc-0ccc333333333333c"), RouteTableId: aws.String("rtb-0bbb222222222222b"), SubnetId: aws.String(fixtProdPublicSubnetB)},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-public")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			RouteTableId: aws.String("rtb-0ccc333333333333c"),
			VpcId:        aws.String(fixtProdVPCID),
			OwnerId:      aws.String("123456789012"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRouteTable},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), NatGatewayId: aws.String("nat-0aaa111111111111a"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			},
			Associations: []ec2types.RouteTableAssociation{
				{Main: aws.Bool(false), RouteTableAssociationId: aws.String("rtbassoc-0ddd444444444444d"), RouteTableId: aws.String("rtb-0ccc333333333333c"), SubnetId: aws.String(fixtProdPrivateSubnetA)},
				{Main: aws.Bool(false), RouteTableAssociationId: aws.String("rtbassoc-0eee555555555555e"), RouteTableId: aws.String("rtb-0ccc333333333333c"), SubnetId: aws.String(fixtProdPrivateSubnetB)},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-private")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			RouteTableId: aws.String("rtb-0ddd444444444444d"),
			VpcId:        aws.String(fixtStagingVPCID),
			OwnerId:      aws.String("123456789012"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.2.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRouteTable},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String("igw-0bbb222222222222b"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
				{DestinationCidrBlock: aws.String("10.2.0.0/16"), NatGatewayId: aws.String("nat-0ccc333333333333c"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			},
			Associations: []ec2types.RouteTableAssociation{
				{Main: aws.Bool(true), RouteTableAssociationId: aws.String("rtbassoc-0fff666666666666f"), RouteTableId: aws.String("rtb-0ddd444444444444d")},
				{Main: aws.Bool(false), RouteTableAssociationId: aws.String("rtbassoc-0ggg777777777777g"), RouteTableId: aws.String("rtb-0ddd444444444444d"), SubnetId: aws.String(fixtStagingSubnetA)},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-main")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// NAT Gateways
// ---------------------------------------------------------------------------

func buildNatGateways() []ec2types.NatGateway {
	t1 := aws.Time(time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC))
	t2 := aws.Time(time.Date(2025, 6, 1, 10, 5, 0, 0, time.UTC))
	t3 := aws.Time(time.Date(2025, 11, 15, 8, 0, 0, 0, time.UTC))
	return []ec2types.NatGateway{
		{
			NatGatewayId:     aws.String("nat-0aaa111111111111a"),
			VpcId:            aws.String(fixtProdVPCID),
			SubnetId:         aws.String(fixtProdPublicSubnetA),
			State:            ec2types.NatGatewayStateAvailable,
			ConnectivityType: ec2types.ConnectivityTypePublic,
			CreateTime:       t1,
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{AllocationId: aws.String("eipalloc-0aaa111111111111a"), PublicIp: aws.String("54.210.33.200"), PrivateIp: aws.String("10.0.1.50"), IsPrimary: aws.Bool(true)},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-nat-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			NatGatewayId:     aws.String("nat-0bbb222222222222b"),
			VpcId:            aws.String(fixtProdVPCID),
			SubnetId:         aws.String(fixtProdPublicSubnetB),
			State:            ec2types.NatGatewayStateAvailable,
			ConnectivityType: ec2types.ConnectivityTypePublic,
			CreateTime:       t2,
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{AllocationId: aws.String("eipalloc-0bbb222222222222b"), PublicIp: aws.String("54.210.33.201"), PrivateIp: aws.String("10.0.2.50"), IsPrimary: aws.Bool(true)},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-nat-1b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			NatGatewayId:     aws.String("nat-0ccc333333333333c"),
			VpcId:            aws.String(fixtStagingVPCID),
			SubnetId:         aws.String(fixtStagingSubnetA),
			State:            ec2types.NatGatewayStateDeleting,
			ConnectivityType: ec2types.ConnectivityTypePublic,
			CreateTime:       t3,
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{AllocationId: aws.String("eipalloc-0ccc333333333333c"), PublicIp: aws.String("52.87.100.10"), PrivateIp: aws.String("10.1.1.50"), IsPrimary: aws.Bool(true)},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-nat")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Internet Gateways
// ---------------------------------------------------------------------------

func buildInternetGateways() []ec2types.InternetGateway {
	return []ec2types.InternetGateway{
		{
			InternetGatewayId: aws.String("igw-0aaa111111111111a"),
			OwnerId:           aws.String("123456789012"),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String(fixtProdVPCID), State: ec2types.AttachmentStatusAttached},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-igw")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			InternetGatewayId: aws.String("igw-0bbb222222222222b"),
			OwnerId:           aws.String("123456789012"),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String(fixtStagingVPCID), State: ec2types.AttachmentStatusAttached},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-igw")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Elastic IPs
// ---------------------------------------------------------------------------

func buildAddresses() []ec2types.Address {
	return []ec2types.Address{
		{
			AllocationId: aws.String("eipalloc-0aaa111111111111a"), PublicIp: aws.String("54.210.33.200"),
			AssociationId: aws.String("eipassoc-0aaa111111111111a"), InstanceId: aws.String("i-0a1b2c3d4e5f60001"),
			SubnetId: aws.String(fixtProdPublicSubnetA), Domain: ec2types.DomainTypeVpc,
			NetworkBorderGroup: aws.String("us-east-1"), NetworkInterfaceId: aws.String("eni-0aaa111111111111a"),
			PrivateIpAddress: aws.String("10.0.1.50"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-nat-eip-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			AllocationId: aws.String("eipalloc-0bbb222222222222b"), PublicIp: aws.String("54.210.33.201"),
			AssociationId: aws.String("eipassoc-0bbb222222222222b"), InstanceId: aws.String("i-0a1b2c3d4e5f60002"),
			Domain: ec2types.DomainTypeVpc, NetworkBorderGroup: aws.String("us-east-1"),
			NetworkInterfaceId: aws.String("eni-0bbb222222222222b"), PrivateIpAddress: aws.String("10.0.2.50"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-nat-eip-1b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			AllocationId: aws.String("eipalloc-0ddd444444444444d"), PublicIp: aws.String("52.87.221.44"),
			AssociationId: aws.String("eipassoc-0ddd444444444444d"), InstanceId: aws.String("i-0a1b2c3d4e5f60005"),
			NetworkInterfaceId: aws.String("eni-0eee555555555555e"), Domain: ec2types.DomainTypeVpc,
			NetworkBorderGroup: aws.String("us-east-1"), PrivateIpAddress: aws.String("10.0.0.5"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("bastion-eip")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			AllocationId: aws.String("eipalloc-0ccc333333333333c"), PublicIp: aws.String("52.87.100.10"),
			AssociationId: aws.String("eipassoc-0ccc333333333333c"),
			Domain:        ec2types.DomainTypeVpc, NetworkBorderGroup: aws.String("us-east-1"),
			NetworkInterfaceId: aws.String("eni-0aaa111111111111a"), PrivateIpAddress: aws.String("10.1.1.50"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-nat-eip")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		{
			AllocationId: aws.String("eipalloc-0eee555555555555e"), PublicIp: aws.String("3.218.100.50"),
			Domain: ec2types.DomainTypeVpc, NetworkBorderGroup: aws.String("us-east-1"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("unassociated-eip")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Transit Gateways
// ---------------------------------------------------------------------------

func buildTransitGateways() []ec2types.TransitGateway {
	t1 := aws.Time(time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC))
	t2 := aws.Time(time.Date(2025, 9, 15, 14, 0, 0, 0, time.UTC))
	t3 := aws.Time(time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC))
	return []ec2types.TransitGateway{
		{
			TransitGatewayId:  aws.String("tgw-0aaa111111111111a"),
			TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0aaa111111111111a"),
			State:             ec2types.TransitGatewayStateAvailable,
			OwnerId:           aws.String("123456789012"),
			Description:       aws.String("Central hub transit gateway for Acme Corp VPCs"),
			CreationTime:      t1,
			Options: &ec2types.TransitGatewayOptions{
				AmazonSideAsn:                aws.Int64(64512),
				AutoAcceptSharedAttachments:  ec2types.AutoAcceptSharedAttachmentsValueEnable,
				DefaultRouteTableAssociation: ec2types.DefaultRouteTableAssociationValueEnable,
				DefaultRouteTablePropagation: ec2types.DefaultRouteTablePropagationValueEnable,
				DnsSupport:                   ec2types.DnsSupportValueEnable,
				VpnEcmpSupport:               ec2types.VpnEcmpSupportValueEnable,
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-hub-tgw")},
				{Key: aws.String("Environment"), Value: aws.String("shared")},
			},
		},
		{
			TransitGatewayId:  aws.String("tgw-0bbb222222222222b"),
			TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0bbb222222222222b"),
			State:             ec2types.TransitGatewayStateAvailable,
			OwnerId:           aws.String("123456789012"),
			Description:       aws.String("Disaster recovery cross-region transit gateway"),
			CreationTime:      t2,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-dr-tgw")},
				{Key: aws.String("Environment"), Value: aws.String("dr")},
			},
		},
		{
			TransitGatewayId:  aws.String("tgw-0ccc333333333333c"),
			TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0ccc333333333333c"),
			State:             ec2types.TransitGatewayStateDeleting,
			OwnerId:           aws.String("123456789012"),
			Description:       aws.String("Development transit gateway (decommissioning)"),
			CreationTime:      t3,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-dev-tgw")},
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Transit Gateway Attachments
// ---------------------------------------------------------------------------

// buildTGWAttachments creates VPC-type attachments for each active TGW.
// The checkTGWVPC checker in tgw_related.go filters by transit-gateway-id and resource-type=vpc.
func buildTGWAttachments() []ec2types.TransitGatewayAttachment {
	t1 := aws.Time(time.Date(2025, 3, 5, 10, 0, 0, 0, time.UTC))
	t2 := aws.Time(time.Date(2025, 3, 5, 10, 5, 0, 0, time.UTC))
	t3 := aws.Time(time.Date(2025, 9, 20, 14, 0, 0, 0, time.UTC))
	t4 := aws.Time(time.Date(2025, 9, 20, 14, 5, 0, 0, time.UTC))
	return []ec2types.TransitGatewayAttachment{
		// hub TGW → prod VPC
		{
			TransitGatewayAttachmentId: aws.String("tgw-attach-0aaa111111111111a"),
			TransitGatewayId:           aws.String("tgw-0aaa111111111111a"),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpc,
			ResourceId:                 aws.String(fixtProdVPCID),
			State:                      ec2types.TransitGatewayAttachmentStateAvailable,
			TransitGatewayOwnerId:      aws.String("123456789012"),
			ResourceOwnerId:            aws.String("123456789012"),
			CreationTime:               t1,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("hub-tgw-prod-vpc")},
			},
		},
		// hub TGW → staging VPC
		{
			TransitGatewayAttachmentId: aws.String("tgw-attach-0bbb222222222222b"),
			TransitGatewayId:           aws.String("tgw-0aaa111111111111a"),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpc,
			ResourceId:                 aws.String(fixtStagingVPCID),
			State:                      ec2types.TransitGatewayAttachmentStateAvailable,
			TransitGatewayOwnerId:      aws.String("123456789012"),
			ResourceOwnerId:            aws.String("123456789012"),
			CreationTime:               t2,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("hub-tgw-staging-vpc")},
			},
		},
		// DR TGW → prod VPC
		{
			TransitGatewayAttachmentId: aws.String("tgw-attach-0ccc333333333333c"),
			TransitGatewayId:           aws.String("tgw-0bbb222222222222b"),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpc,
			ResourceId:                 aws.String(fixtProdVPCID),
			State:                      ec2types.TransitGatewayAttachmentStateAvailable,
			TransitGatewayOwnerId:      aws.String("123456789012"),
			ResourceOwnerId:            aws.String("123456789012"),
			CreationTime:               t3,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("dr-tgw-prod-vpc")},
			},
		},
		// DR TGW → staging VPC
		{
			TransitGatewayAttachmentId: aws.String("tgw-attach-0ddd444444444444d"),
			TransitGatewayId:           aws.String("tgw-0bbb222222222222b"),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpc,
			ResourceId:                 aws.String(fixtStagingVPCID),
			State:                      ec2types.TransitGatewayAttachmentStateAvailable,
			TransitGatewayOwnerId:      aws.String("123456789012"),
			ResourceOwnerId:            aws.String("123456789012"),
			CreationTime:               t4,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("dr-tgw-staging-vpc")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// VPC Endpoints
// ---------------------------------------------------------------------------

func buildVpcEndpoints() []ec2types.VpcEndpoint {
	t1 := aws.Time(time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))
	t2 := aws.Time(time.Date(2025, 6, 15, 12, 5, 0, 0, time.UTC))
	t3 := aws.Time(time.Date(2025, 8, 1, 9, 30, 0, 0, time.UTC))
	t4 := aws.Time(time.Date(2026, 3, 21, 7, 0, 0, 0, time.UTC))
	return []ec2types.VpcEndpoint{
		{
			VpcEndpointId:       aws.String("vpce-0aaa111111111111a"),
			ServiceName:         aws.String("com.amazonaws.us-east-1.s3"),
			VpcEndpointType:     ec2types.VpcEndpointTypeGateway,
			State:               ec2types.StateAvailable,
			VpcId:               aws.String(fixtProdVPCID),
			RouteTableIds:       []string{"rtb-0aaa111111111111a", "rtb-0ccc333333333333c"},
			SubnetIds:           []string{fixtProdPrivateSubnetA, fixtProdPrivateSubnetB},
			NetworkInterfaceIds: []string{"eni-0ccc333333333333c"},
			Groups: []ec2types.SecurityGroupIdentifier{
				{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			},
			PrivateDnsEnabled: aws.Bool(false),
			PolicyDocument:    aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"*","Resource":"*"}]}`),
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: t1,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-s3-endpoint")},
			},
		},
		{
			VpcEndpointId:     aws.String("vpce-0bbb222222222222b"),
			ServiceName:       aws.String("com.amazonaws.us-east-1.dynamodb"),
			VpcEndpointType:   ec2types.VpcEndpointTypeGateway,
			State:             ec2types.StateAvailable,
			VpcId:             aws.String(fixtProdVPCID),
			RouteTableIds:     []string{"rtb-0aaa111111111111a", "rtb-0ccc333333333333c"},
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: t2,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-dynamodb-endpoint")},
			},
		},
		{
			VpcEndpointId:       aws.String("vpce-0ccc333333333333c"),
			ServiceName:         aws.String("com.amazonaws.us-east-1.secretsmanager"),
			VpcEndpointType:     ec2types.VpcEndpointTypeInterface,
			State:               ec2types.StateAvailable,
			VpcId:               aws.String(fixtProdVPCID),
			SubnetIds:           []string{fixtProdPrivateSubnetA, fixtProdPrivateSubnetB},
			NetworkInterfaceIds: []string{"eni-0ccc333333333333c", "eni-0ddd444444444444d"},
			PrivateDnsEnabled:   aws.Bool(true),
			OwnerId:             aws.String("123456789012"),
			CreationTimestamp:   t3,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-secrets-endpoint")},
			},
		},
		{
			VpcEndpointId:     aws.String("vpce-0ddd444444444444d"),
			ServiceName:       aws.String("com.amazonaws.us-east-1.ecr.dkr"),
			VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
			State:             ec2types.StatePending,
			VpcId:             aws.String(fixtProdVPCID),
			SubnetIds:         []string{fixtProdPrivateSubnetA},
			PrivateDnsEnabled: aws.Bool(true),
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: t4,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-ecr-endpoint")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Network Interfaces
// ---------------------------------------------------------------------------

func buildNetworkInterfaces() []ec2types.NetworkInterface {
	return []ec2types.NetworkInterface{
		{
			NetworkInterfaceId: aws.String("eni-0aaa111111111111a"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeNatGateway,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPublicSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.1.50"),
			PrivateDnsName:     aws.String("ip-10-0-1-50.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:01"),
			Description:        aws.String("Interface for NAT Gateway nat-0aaa111111111111a"),
			OwnerId:            aws.String("123456789012"),
			RequesterId:        aws.String("amazon-elb"),
			RequesterManaged:   aws.Bool(true),
			SourceDestCheck:    aws.Bool(false),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				AttachmentId: aws.String("eni-attach-01"), InstanceId: aws.String("i-0a1b2c3d4e5f60001"),
				DeviceIndex: aws.Int32(0), Status: ec2types.AttachmentStatusAttached, DeleteOnTermination: aws.Bool(true),
			},
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			},
			Association: &ec2types.NetworkInterfaceAssociation{
				PublicIp: aws.String("54.210.33.112"), PublicDnsName: aws.String("ec2-54-210-33-112.compute-1.amazonaws.com"),
				IpOwnerId: aws.String("amazon"), AllocationId: aws.String("eipalloc-0aaa111111111111a"),
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("prod-nat-eni-1a")}},
		},
		{
			NetworkInterfaceId: aws.String("eni-0bbb222222222222b"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeNatGateway,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPublicSubnetB),
			AvailabilityZone:   aws.String("us-east-1b"),
			PrivateIpAddress:   aws.String("10.0.2.50"),
			PrivateDnsName:     aws.String("ip-10-0-2-50.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:02"),
			Description:        aws.String("Interface for NAT Gateway nat-0bbb222222222222b"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(true),
			SourceDestCheck:    aws.Bool(false),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				AttachmentId: aws.String("eni-attach-02"), InstanceId: aws.String("i-0a1b2c3d4e5f60002"),
				DeviceIndex: aws.Int32(0), Status: ec2types.AttachmentStatusAttached, DeleteOnTermination: aws.Bool(true),
			},
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			},
			Association: &ec2types.NetworkInterfaceAssociation{
				PublicIp: aws.String("54.210.33.113"), IpOwnerId: aws.String("amazon"), AllocationId: aws.String("eipalloc-0bbb222222222222b"),
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("prod-nat-eni-1b")}},
		},
		{
			NetworkInterfaceId: aws.String("eni-0eee555555555555e"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPublicSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.1.10"),
			PrivateDnsName:     aws.String("ip-10-0-1-10.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:05"),
			Description:        aws.String("Primary network interface for web-prod-01"),
			OwnerId:            aws.String("123456789012"),
			RequesterId:        aws.String("amazon-elb"),
			RequesterManaged:   aws.Bool(false),
			SourceDestCheck:    aws.Bool(true),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				AttachmentId: aws.String("eni-attach-05"), InstanceId: aws.String("i-0a1b2c3d4e5f60001"),
				DeviceIndex: aws.Int32(0), Status: ec2types.AttachmentStatusAttached, DeleteOnTermination: aws.Bool(true),
			},
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String("sg-0aaa111111111111a"), GroupName: aws.String("acme-web-alb-sg")},
			},
			Association: &ec2types.NetworkInterfaceAssociation{
				PublicIp: aws.String("54.210.33.115"), IpOwnerId: aws.String("amazon"), AllocationId: aws.String("eipalloc-0aaa111111111111a"),
			},
			TagSet: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("web-prod-01-primary")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			NetworkInterfaceId: aws.String("eni-0fff666666666666f"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeVpcEndpoint,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPrivateSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.3.100"),
			PrivateDnsName:     aws.String("ip-10-0-3-100.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:06"),
			Description:        aws.String("VPC Endpoint Interface for Secrets Manager"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(true),
			SourceDestCheck:    aws.Bool(true),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				AttachmentId: aws.String("eni-attach-06"), InstanceId: aws.String("i-0a1b2c3d4e5f60003"),
				DeviceIndex: aws.Int32(0), Status: ec2types.AttachmentStatusAttached, DeleteOnTermination: aws.Bool(false),
			},
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(fixtProdAPIInternalSGID), GroupName: aws.String("acme-api-internal-sg")},
			},
			Association: &ec2types.NetworkInterfaceAssociation{
				PublicIp: aws.String("54.210.33.116"), IpOwnerId: aws.String("amazon"), AllocationId: aws.String("eipalloc-0ddd444444444444d"),
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("vpce-secrets-eni-1a")}},
		},
		{
			NetworkInterfaceId: aws.String("eni-0ggg777777777777g"),
			Status:             ec2types.NetworkInterfaceStatusAvailable,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPrivateSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.3.200"),
			PrivateDnsName:     aws.String("ip-10-0-3-200.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:07"),
			Description:        aws.String("Detached network interface"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(false),
			SourceDestCheck:    aws.Bool(true),
			TagSet: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("detached-eni")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// EBS Volumes
// ---------------------------------------------------------------------------

func buildVolumes() []ec2types.Volume {
	t1 := time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 11, 15, 8, 30, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 20, 14, 15, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	t5 := time.Date(2026, 3, 28, 9, 30, 0, 0, time.UTC)
	return []ec2types.Volume{
		{
			VolumeId: aws.String("vol-0a1b2c3d4e5f60001"), State: ec2types.VolumeStateInUse,
			Size: aws.Int32(50), VolumeType: ec2types.VolumeTypeGp3, Iops: aws.Int32(3000), Throughput: aws.Int32(125),
			Encrypted: aws.Bool(true), AvailabilityZone: aws.String("us-east-1a"), CreateTime: aws.Time(t2),
			KmsKeyId:           aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			MultiAttachEnabled: aws.Bool(false),
			Attachments:        []ec2types.VolumeAttachment{{InstanceId: aws.String("i-0a1b2c3d4e5f60001")}},
			Tags:               []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web-prod-01-root")}},
		},
		{
			VolumeId: aws.String("vol-0a1b2c3d4e5f60002"), State: ec2types.VolumeStateInUse,
			Size: aws.Int32(200), VolumeType: ec2types.VolumeTypeIo1, Iops: aws.Int32(6000),
			Encrypted: aws.Bool(true), AvailabilityZone: aws.String("us-east-1b"), CreateTime: aws.Time(t1),
			Attachments: []ec2types.VolumeAttachment{{InstanceId: aws.String("i-0a1b2c3d4e5f60003")}},
			Tags:        []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("api-staging-data")}},
		},
		{
			VolumeId: aws.String("vol-0a1b2c3d4e5f60003"), State: ec2types.VolumeStateAvailable,
			Size: aws.Int32(100), VolumeType: ec2types.VolumeTypeGp2, Iops: aws.Int32(300),
			Encrypted: aws.Bool(false), AvailabilityZone: aws.String("us-east-1a"), CreateTime: aws.Time(t3),
			Attachments: []ec2types.VolumeAttachment{{InstanceId: aws.String("i-0a1b2c3d4e5f60002")}},
			Tags:        []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("orphaned-backup-vol")}},
		},
		{
			VolumeId: aws.String("vol-0a1b2c3d4e5f60004"), State: ec2types.VolumeStateAvailable,
			Size: aws.Int32(500), VolumeType: ec2types.VolumeTypeGp3, Iops: aws.Int32(3000),
			Encrypted: aws.Bool(true), AvailabilityZone: aws.String("us-east-1c"), CreateTime: aws.Time(t4),
			Attachments: []ec2types.VolumeAttachment{{InstanceId: aws.String("i-0a1b2c3d4e5f60004")}},
			Tags:        []ec2types.Tag{},
		},
		{
			VolumeId: aws.String("vol-0a1b2c3d4e5f60005"), State: ec2types.VolumeStateCreating,
			Size: aws.Int32(1000), VolumeType: ec2types.VolumeTypeIo2, Iops: aws.Int32(16000),
			Encrypted: aws.Bool(true), AvailabilityZone: aws.String("us-east-1b"), CreateTime: aws.Time(t5),
			Attachments: []ec2types.VolumeAttachment{{InstanceId: aws.String("i-0a1b2c3d4e5f60006")}},
			Tags:        []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("new-db-volume")}},
		},
	}
}

// ---------------------------------------------------------------------------
// EBS Snapshots
// ---------------------------------------------------------------------------

func buildSnapshots() []ec2types.Snapshot {
	t1 := time.Date(2025, 9, 1, 2, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 21, 4, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 28, 4, 0, 0, 0, time.UTC)
	return []ec2types.Snapshot{
		{
			SnapshotId: aws.String("snap-0a1b2c3d4e5f60001"), State: ec2types.SnapshotStateCompleted,
			VolumeId: aws.String("vol-0a1b2c3d4e5f60001"), VolumeSize: aws.Int32(50),
			Encrypted: aws.Bool(true), Description: aws.String("Quarterly backup of web-prod-01 root volume"),
			StartTime: aws.Time(t1), Progress: aws.String("100%"), OwnerId: aws.String("123456789012"),
			KmsKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
			Tags:     []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web-prod-snapshot-2025q3")}},
		},
		{
			SnapshotId: aws.String("snap-0a1b2c3d4e5f60002"), State: ec2types.SnapshotStateCompleted,
			VolumeId: aws.String("vol-0a1b2c3d4e5f60002"), VolumeSize: aws.Int32(200),
			Encrypted: aws.Bool(true), Description: aws.String("Q1 2026 backup of api-staging-data volume"),
			StartTime: aws.Time(t2), Progress: aws.String("100%"), OwnerId: aws.String("123456789012"),
			KmsKeyId: aws.String("b2c3d4e5-6789-01ab-cdef-222222222222"),
			Tags:     []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("api-data-snapshot-2026q1")}},
		},
		{
			SnapshotId: aws.String("snap-0a1b2c3d4e5f60003"), State: ec2types.SnapshotStateCompleted,
			VolumeId: aws.String("vol-0a1b2c3d4e5f60003"), VolumeSize: aws.Int32(100),
			Encrypted: aws.Bool(true), Description: aws.String("Pre-deletion backup"),
			StartTime: aws.Time(t3), Progress: aws.String("100%"), OwnerId: aws.String("123456789012"),
			KmsKeyId: aws.String("c3d4e5f6-7890-12ab-cdef-333333333333"),
			Tags:     []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("orphaned-vol-snapshot")}},
		},
		{
			SnapshotId: aws.String("snap-0a1b2c3d4e5f60004"), State: ec2types.SnapshotStatePending,
			VolumeId: aws.String("vol-0a1b2c3d4e5f60005"), VolumeSize: aws.Int32(1000),
			Encrypted: aws.Bool(true), Description: aws.String("Initial snapshot of new-db-volume"),
			StartTime: aws.Time(t4), Progress: aws.String("23%"), OwnerId: aws.String("123456789012"),
			KmsKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
			Tags:     []ec2types.Tag{},
		},
	}
}

// ---------------------------------------------------------------------------
// AMIs
// ---------------------------------------------------------------------------

func buildImages() []ec2types.Image {
	return []ec2types.Image{
		{
			ImageId: aws.String(fixtProdAMIID1), Name: aws.String("acme-app-server-x86-v2.3.1"),
			State: ec2types.ImageStateAvailable, Architecture: ec2types.ArchitectureValuesX8664,
			PlatformDetails: aws.String("Linux/UNIX"), RootDeviceType: ec2types.DeviceTypeEbs,
			RootDeviceName: aws.String("/dev/xvda"), Hypervisor: ec2types.HypervisorTypeXen,
			VirtualizationType: ec2types.VirtualizationTypeHvm, ImageType: ec2types.ImageTypeValuesMachine,
			CreationDate: aws.String("2026-02-15T10:30:00.000Z"), Public: aws.Bool(false),
			OwnerId: aws.String("123456789012"), Description: aws.String("Production app server image x86_64 v2.3.1"),
			EnaSupport: aws.Bool(true),
			BlockDeviceMappings: []ec2types.BlockDeviceMapping{
				{DeviceName: aws.String("/dev/xvda"), Ebs: &ec2types.EbsBlockDevice{VolumeSize: aws.Int32(20), VolumeType: ec2types.VolumeTypeGp3, DeleteOnTermination: aws.Bool(true)}},
			},
			BootMode: ec2types.BootModeValuesUefi, DeprecationTime: aws.String("2028-01-01T00:00:00Z"),
			ImageLocation: aws.String("123456789012/amazon-linux-2023-x86_64"), ImageOwnerAlias: aws.String("amazon"),
			SriovNetSupport: aws.String("simple"), UsageOperation: aws.String("RunInstances"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-app-server-x86-v2.3.1")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			ImageId: aws.String(fixtProdAMIID2), Name: aws.String("acme-app-server-arm64-v2.3.1"),
			State: ec2types.ImageStateAvailable, Architecture: ec2types.ArchitectureValuesArm64,
			PlatformDetails: aws.String("Linux/UNIX"), RootDeviceType: ec2types.DeviceTypeEbs,
			RootDeviceName: aws.String("/dev/xvda"), Hypervisor: ec2types.HypervisorTypeXen,
			VirtualizationType: ec2types.VirtualizationTypeHvm, ImageType: ec2types.ImageTypeValuesMachine,
			CreationDate: aws.String("2026-02-15T10:35:00.000Z"), Public: aws.Bool(false),
			OwnerId: aws.String("123456789012"), Description: aws.String("Production app server image arm64 v2.3.1"),
			EnaSupport: aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-app-server-arm64-v2.3.1")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			ImageId: aws.String(fixtProdAMIID3), Name: aws.String("acme-worker-x86-v1.8.0"),
			State: ec2types.ImageStateAvailable, Architecture: ec2types.ArchitectureValuesX8664,
			PlatformDetails: aws.String("Linux/UNIX"), RootDeviceType: ec2types.DeviceTypeEbs,
			RootDeviceName: aws.String("/dev/xvda"), Hypervisor: ec2types.HypervisorTypeXen,
			VirtualizationType: ec2types.VirtualizationTypeHvm, ImageType: ec2types.ImageTypeValuesMachine,
			CreationDate: aws.String("2025-09-10T08:00:00.000Z"), Public: aws.Bool(false),
			OwnerId: aws.String("123456789012"), Description: aws.String("Batch worker image x86_64 v1.8.0"),
			EnaSupport: aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-worker-x86-v1.8.0")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			ImageId: aws.String("ami-0a1b2c3d4e5f60004"), Name: aws.String("acme-bastion-x86-v3.0.0-deprecated"),
			State: ec2types.ImageStateDeregistered, Architecture: ec2types.ArchitectureValuesX8664,
			PlatformDetails: aws.String("Linux/UNIX"), RootDeviceType: ec2types.DeviceTypeEbs,
			RootDeviceName: aws.String("/dev/xvda"), Hypervisor: ec2types.HypervisorTypeXen,
			VirtualizationType: ec2types.VirtualizationTypeHvm, ImageType: ec2types.ImageTypeValuesMachine,
			CreationDate: aws.String("2024-06-01T12:00:00.000Z"), Public: aws.Bool(false),
			OwnerId: aws.String("123456789012"), Description: aws.String("Deprecated bastion host image"),
			EnaSupport: aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-bastion-x86-v3.0.0-deprecated")},
			},
		},
	}
}
