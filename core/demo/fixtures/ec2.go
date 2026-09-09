// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides EC2 fixture data for the EC2 fake.
package fixtures

import (
	"fmt"
	"strings"
	"sync"
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
	VolumeStatuses    []ec2types.VolumeStatusItem
	Snapshots         []ec2types.Snapshot
	Images            []ec2types.Image
	// TGWVpcAttachmentSubnets maps a TransitGatewayAttachmentId to the
	// subnet IDs backing that VPC attachment. Backs the tgw→subnet
	// related-panel pivot (checkTGWSubnet), mirroring
	// ec2:DescribeTransitGatewayVpcAttachments.TransitGatewayVpcAttachments[].SubnetIds.
	TGWVpcAttachmentSubnets map[string][]string
	// FlowLogsByResourceID maps a "resource-id" filter value (e.g. a VPC
	// endpoint ID) to the flow logs configured against it. Backs
	// ec2:DescribeFlowLogs for the vpce:logs related-panel pivot
	// (checkVPCELogs).
	FlowLogsByResourceID map[string][]ec2types.FlowLog
	// PublicSnapshotIDs is the set DescribeSnapshots returns when asked for
	// RestorableByUserIds=["all"] — the snapshots shared with every account.
	PublicSnapshotIDs []string
	// UserDataByInstanceID overrides the fake's default bootstrap script for
	// the instances that need their own. Backs ec2:DescribeInstanceAttribute
	// (userData) for the ec2.user-data-secret witness.
	UserDataByInstanceID map[string]string
}

// shared constants (mirrors core/demo/constants_shared.go — no import allowed)
const (
	fixtProdVPCID    = "vpc-0abc123def456789a"
	fixtStagingVPCID = "vpc-0def456789abc123d"
	// VPCSubnetScopedFlowLog is the VPC whose only flow log is attached to a
	// subnet rather than to the VPC itself. A subnet-scoped log writes the
	// same records, so this VPC must carry NO missing-flow-log finding; the
	// prod and default VPCs, which have no log at any scope, are the rows
	// that do.
	VPCSubnetScopedFlowLog = "vpc-0f10a1b2c3d4e5f60"
	// vpcSubnetScopedFlowLogSubnet is that VPC's one subnet, and the resource
	// the flow log names.
	vpcSubnetScopedFlowLogSubnet = "subnet-0f10a1b2c3d4e5f60"
	fixtProdPublicSubnetA        = "subnet-0aaa111111111111a"
	fixtProdPublicSubnetB        = "subnet-0bbb222222222222b"
	fixtProdPrivateSubnetA       = "subnet-0ccc333333333333c"
	fixtProdPrivateSubnetB       = "subnet-0ddd444444444444d"
	fixtStagingSubnetA           = "subnet-0eee555555555555e"
	fixtStagingSubnetB           = "subnet-0fff666666666666f"
	fixtProdWebALBSGID           = "sg-0aaa111111111111a"
	fixtProdAPIInternalSGID      = "sg-0bbb222222222222b"
	fixtProdRDSSGID              = "sg-0ccc333333333333c"
	fixtProdDBProxySGID          = "sg-0ddd444444444444d"
	fixtStagingDefaultSGID       = "sg-0fff888888888888f"
	fixtProdAMIID1               = "ami-0a1b2c3d4e5f60001"
	fixtProdAMIID2               = "ami-0a1b2c3d4e5f60002"
	fixtProdAMIID3               = "ami-0a1b2c3d4e5f60003"
	fixtProdInstanceProfileARN   = "arn:aws:iam::123456789012:instance-profile/acme-ec2-instance-profile"
	// fixtProdEKSClusterName / fixtRelatedEC2NGNodeGroupID must match the real
	// EKS cluster ("acme-prod") and nodegroup ("general-pool") fixture names in
	// eks.go so ec2→ng (checkEC2NodeGroups) and ct-events→ec2 tag-based
	// reverse-scans resolve real cross-file matches instead of pointing at
	// names no sibling fixture defines.
	fixtProdEKSClusterName      = "acme-prod"
	fixtRelatedEC2NGNodeGroupID = "general-pool"
	// EC2 posture witnesses — one demo instance per Prowler-derived finding,
	// every other instance explicitly set to the healthy counterpart so the
	// demo bench shows exactly one row per signal.
	//
	// EC2InstanceIMDSv1 is the only instance whose MetadataOptions leave
	// HttpTokens optional; every other instance requires a session token.
	EC2InstanceIMDSv1 = "i-0a1b2c3d4e5f60006"
	// EC2InstancePublicIPOnly holds a public address behind acme-web-alb-sg,
	// which opens 443/80 only — a public address with no sensitive port
	// behind it. It is also the instance the Addresses fixture associates an
	// Elastic IP with, so the address is the one it would really have.
	EC2InstancePublicIPOnly = "i-0a1b2c3d4e5f60001"
	// EC2InstanceInternetExposed holds a public address AND carries
	// public-ssh-bad (sg-0public0ssh000001, port 22 open to 0.0.0.0/0).
	EC2InstanceInternetExposed = "i-0a1b2c3d4e5f60005"
	// EC2InstanceInternetExposedAll is the only instance behind a group that
	// admits every protocol from 0.0.0.0/0 (public-all-open,
	// sg-0public0all000003); every other public instance is behind a group
	// that names its ports.
	EC2InstanceInternetExposedAll = "i-0a1b2c3d4e5f60050"
	// EC2InstanceHostileTag is the only demo instance whose Name tag was
	// written by someone who wanted the terminal, not the operator, to read
	// it: the value opens an SGR sequence that recolours the rest of the
	// screen and rings the bell. AWS accepts such a value and hands it
	// straight back.
	//
	// It is the Name tag, not a tag beside it, because that is the value that
	// reaches every surface an operator uses: the name field, the identity
	// column, the filter typed against it, the frame title of its detail
	// screen and the clipboard. A witness on any other tag would be visible
	// only in the detail's Tags block, where YAML marshalling escapes a
	// control byte anyway and the boundary is never the thing under test.
	EC2InstanceHostileTag = "i-0a1b2c3d4e5f60031"
	// EC2HostileTagValue is that Name.
	EC2HostileTagValue = "dev-sandbox\x1b[31m-02\x07"
	// EBSSnapPublic is the only demo snapshot restorable by every AWS
	// account; every other snapshot is private to this account.
	EBSSnapPublic = "snap-0a1b2c3d4e5f60002"
	// EBSNotInBackupPlan is the only demo volume no backup plan selects: the
	// fleet-wide plan excludes it by name and no other selection reaches it.
	EBSNotInBackupPlan    = "vol-0a1b2c3d4e5f60004"
	EBSNotInBackupPlanARN = "arn:aws:ec2:us-east-1:123456789012:volume/vol-0a1b2c3d4e5f60004"
	// EBSNoSnapshot is the only attached demo volume the snapshot list holds
	// no snapshot of. It is covered by a plan, so it carries that one finding.
	EBSNoSnapshot = "vol-0unenc00000000c3"
	// AMIPublic is the only demo image whose launch permission includes every
	// AWS account; every other image sets Public=false.
	AMIPublic = "ami-0public00000000001"
	// EC2InstanceUserDataSecret is the only instance whose user data carries
	// a plaintext credential; every other instance returns demoUserData.
	EC2InstanceUserDataSecret = "i-0a1b2c3d4e5f60002"

	// HealthyTGWID is the only demo Transit Gateway with a single VPC
	// attachment left in the Available state — the sole witness for the tgw
	// Healthy color bucket.
	HealthyTGWID = "tgw-0healthy11111111h"
)

// AMIEBSKmsKeyID / AMIEBSKmsKeyARN back the ami→kms related-panel pivot.
// checkAMIKMS (core/aws/ami_related_extra.go) passes the raw
// BlockDeviceMappings[].Ebs.KmsKeyId ARN through as the navigation ID
// unmodified (unlike checkS3KMS/checkDdbKMS, which strip the ARN to a bare
// key ID first). Real DescribeKey-by-ARN always reports the true bare KeyId
// in its response, never the ARN that was searched by — so this AMI cannot
// share the widely-reused "primary" KMS key (referenced by bare ID from many
// sibling fixtures); it needs its own key whose KeyId is the ARN string
// itself, keeping the fake's DescribeKey response self-consistent with what
// checkAMIKMS looked up. See kms.go for the corresponding fixture entry.
const (
	AMIEBSKmsKeyID  = "ami-ebs-boot-volume-key"
	AMIEBSKmsKeyARN = "arn:aws:kms:us-east-1:123456789012:key/" + AMIEBSKmsKeyID
)

// Prowler-gap witnesses for the networking types. Each names the ONE demo
// resource that carries the corresponding finding; every other row of that
// type is set to the healthy value for the same condition.
const (
	// SGDangerousFTP is the only demo group exposing a newly sensitive port
	// (FTP control) to the internet.
	SGDangerousFTP = "sg-0ftp00000000000001"
	// SGDefaultWithRules is the only demo group named "default" that still
	// carries rules.
	SGDefaultWithRules = "sg-0default000000001"
	// SGUnused is the only demo group no network interface references.
	SGUnused = "sg-0unused0000000001"
	// SubnetAutoPublicIP is the only demo subnet that auto-assigns public IPs.
	SubnetAutoPublicIP = fixtProdPublicSubnetA
	// TGWMixedSeverity is the only demo row of the five networking types
	// whose findings run [Warn, Broken]: a gateway still modifying, with a
	// failed attachment the wave-2 enricher appends behind that state.
	TGWMixedSeverity = "tgw-0modifying111111g"
	// TGWAutoAccept is the only demo transit gateway that auto-accepts
	// shared attachments.
	TGWAutoAccept = "tgw-0aaa111111111111a"
	// VPCEPolicyOpen is the only demo VPC endpoint whose policy grants every
	// action to every principal.
	VPCEPolicyOpen = "vpce-0aaa111111111111a"
)

// NewEC2Fixtures builds and returns a fully-populated EC2Fixtures struct
// with deterministic demo data.
// This is the single source of truth for all EC2 fake responses.
var sharedEC2Fixtures = sync.OnceValue(func() *EC2Fixtures {
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
	f.NetworkInterfaces = buildNetworkInterfaces(f.SecurityGroups)
	f.Volumes = buildVolumes()
	f.VolumeStatuses = buildVolumeStatuses()
	f.Snapshots = buildSnapshots()
	f.Images = buildImages()
	// TGWVpcAttachmentSubnets — the hub TGW's prod-VPC attachment spans the
	// two prod public subnets, backing the tgw→subnet related-panel pivot.
	f.TGWVpcAttachmentSubnets = map[string][]string{
		"tgw-attach-0aaa111111111111a": {fixtProdPublicSubnetA, fixtProdPublicSubnetB},
	}
	// FlowLogsByResourceID — the prod S3 gateway endpoint has a flow log
	// delivering to CloudWatch Logs, backing the vpce:logs related-panel
	// pivot (checkVPCELogs via ec2:DescribeFlowLogs filtered by resource-id).
	// The staging VPC's own ACTIVE flow log is the only demo witness for the
	// vpc Healthy color bucket (EnrichVPCFlowLogs raises vpc.no-flow-logs for
	// every VPC without one).
	f.PublicSnapshotIDs = []string{EBSSnapPublic}
	f.UserDataByInstanceID = map[string]string{
		// The only demo instance whose bootstrap script pastes a credential
		// instead of resolving one from Secrets Manager.
		EC2InstanceUserDataSecret: `#!/bin/bash
set -euo pipefail
yum update -y
export DB_PASSWORD=hunter2hunter2
/opt/acme/bin/api-server --db-user acme
`,
	}
	f.FlowLogsByResourceID = map[string][]ec2types.FlowLog{
		"vpce-0aaa111111111111a": {
			{
				FlowLogId:          aws.String("fl-0aaa111111111111a"),
				ResourceId:         aws.String("vpce-0aaa111111111111a"),
				LogDestinationType: ec2types.LogDestinationTypeCloudWatchLogs,
				LogGroupName:       aws.String("/aws/vpc/flowlogs/vpce-s3-endpoint"),
				DeliverLogsStatus:  aws.String("SUCCESS"),
				TrafficType:        ec2types.TrafficTypeAll,
				CreationTime:       aws.Time(time.Date(2025, 6, 15, 12, 10, 0, 0, time.UTC)),
			},
		},
		vpcSubnetScopedFlowLogSubnet: {
			{
				FlowLogId:          aws.String("fl-0ccc333333333333c"),
				ResourceId:         aws.String(vpcSubnetScopedFlowLogSubnet),
				LogDestinationType: ec2types.LogDestinationTypeS3,
				LogDestination:     aws.String("arn:aws:s3:::" + LogsBucketName + "/flowlogs/"),
				DeliverLogsStatus:  aws.String("SUCCESS"),
				FlowLogStatus:      aws.String("ACTIVE"),
				TrafficType:        ec2types.TrafficTypeAll,
				CreationTime:       aws.Time(time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC)),
			},
		},
		fixtStagingVPCID: {
			{
				FlowLogId:          aws.String("fl-0bbb222222222222b"),
				ResourceId:         aws.String(fixtStagingVPCID),
				LogDestinationType: ec2types.LogDestinationTypeCloudWatchLogs,
				LogGroupName:       aws.String("/aws/vpc/flowlogs/acme-staging"),
				DeliverLogsStatus:  aws.String("SUCCESS"),
				FlowLogStatus:      aws.String("ACTIVE"),
				TrafficType:        ec2types.TrafficTypeAll,
				CreationTime:       aws.Time(time.Date(2025, 6, 20, 9, 0, 0, 0, time.UTC)),
			},
		},
	}
	return f
})

func NewEC2Fixtures() *EC2Fixtures {
	return sharedEC2Fixtures()
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
	EC2InstanceInternetExposedAll: {
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String("sg-0public0all000003"), GroupName: aws.String("public-all-open")},
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
	// Staging VPC instances — must use staging SG, not prod web-ALB SG.
	"i-0a1b2c3d4e5f60030": {
		imageID: fixtProdAMIID2, keyName: "acme-staging-keypair",
		architecture: ec2types.ArchitectureValuesArm64, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtStagingDefaultSGID), GroupName: aws.String("staging-default-sg")},
		},
	},
	"i-0a1b2c3d4e5f60031": {
		imageID: fixtProdAMIID2, keyName: "acme-staging-keypair",
		architecture: ec2types.ArchitectureValuesArm64, az: "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtStagingDefaultSGID), GroupName: aws.String("staging-default-sg")},
		},
	},
	// ASG-owned instances (asg.go's asg-underprovisioned/-suspended/
	// -unhealthy-instance/-scaling-failed Instances[] lists) — required for
	// the ec2->asg related-panel pivot (checkEC2ASG matches by InstanceId
	// against each ASG's own Instances[] list, so the graph edge is real
	// once these instances exist) and for qa_demo_related_ids_resolve_test.go's
	// asg:ec2 witness resolution. AZ/subnet mirror the owning ASG's own
	// VPCZoneIdentifier (asgSubnetA==fixtProdPublicSubnetA/us-east-1a,
	// asgSubnetB==fixtProdPublicSubnetB/us-east-1b); AMI/keypair/SG mirror
	// the same acme-prod profile every other prod worker instance in this
	// file uses.
	"i-0aaa111111111111a": { // asg-underprovisioned, api-worker-01
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0bbb222222222222b": { // asg-underprovisioned, api-worker-02
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0ccc333333333333c": { // asg-suspended, batch-processor-01
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdRDSSGID), GroupName: aws.String("acme-worker-sg")},
		},
	},
	"i-0ddd444444444444d": { // asg-suspended, batch-processor-02
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdRDSSGID), GroupName: aws.String("acme-worker-sg")},
		},
	},
	"i-0eee555555555555e": { // asg-unhealthy-instance, web-worker-01 (Healthy)
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
		},
	},
	"i-0fff666666666666f": { // asg-unhealthy-instance, web-worker-02 (Healthy)
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
		},
	},
	"i-0aaa777777777777a": { // asg-unhealthy-instance, web-worker-03 (Unhealthy — failed health check, still running)
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
		},
	},
	"i-0bbb888888888888b": { // asg-scaling-failed, payments-worker-01
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdRDSSGID), GroupName: aws.String("acme-worker-sg")},
		},
	},
	"i-0ccc999999999999c": { // asg-scaling-failed, payments-worker-02
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdRDSSGID), GroupName: aws.String("acme-worker-sg")},
		},
	},
	// ECS EC2-launch-type container host — ecs.go's acme-batch cluster's
	// batch-etl-runner task carries this exact ContainerInstanceArn suffix
	// as its surfaced ecs-task->ec2 link (checkECSTaskEC2's own doc comment:
	// "the backing EC2 instance ID is not in this ARN... return the
	// container-instance UUID as a surfaced link" — mirrors the real API's
	// own limitation, not a fixture bug).
	"e1f2a3b4c5d6e1f2a3b4c5d6": { // acme-batch cluster, batch-etl-host-01
		imageID: fixtProdAMIID1, keyName: "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664, az: "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(fixtProdRDSSGID), GroupName: aws.String("acme-worker-sg")},
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
			HttpTokens:              httpTokensFor(instanceID),
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
		// aws:autoscaling:groupName tag — required for eip→asg related-panel
		// pivot. Matches this instance's membership in acme-web-prod-asg (asg.go).
		inst.Tags = append(inst.Tags, ec2types.Tag{
			Key:   aws.String("aws:autoscaling:groupName"),
			Value: aws.String("acme-web-prod-asg"),
		})
		// backup=daily tag — required for the ec2:backup related-panel pivot
		// witness. Matches the ListOfTags condition on HealthyDailyPlanID's
		// selection (backup.go).
		inst.Tags = append(inst.Tags, ec2types.Tag{
			Key:   aws.String("backup"),
			Value: aws.String("daily"),
		})
		inst.NetworkInterfaces = []ec2types.InstanceNetworkInterface{
			{NetworkInterfaceId: aws.String("eni-0aaa111111111111a")},
		}
	}
	if instanceID == "i-0a1b2c3d4e5f60003" {
		inst.Tags = append(inst.Tags,
			ec2types.Tag{Key: aws.String("eks:cluster-name"), Value: aws.String(fixtProdEKSClusterName)},
			ec2types.Tag{Key: aws.String("eks:nodegroup-name"), Value: aws.String(fixtRelatedEC2NGNodeGroupID)},
		)
	}
	// elasticbeanstalk:environment-name tag — required for eb:ec2 related-panel
	// pivot. acme-prod-api is a real eb.go environment fixture.
	if instanceID == "i-0a1b2c3d4e5f60002" {
		inst.Tags = append(inst.Tags,
			ec2types.Tag{Key: aws.String("elasticbeanstalk:environment-name"), Value: aws.String("acme-prod-api")},
		)
	}
	// aws:cloudformation:stack-name tag — required for ec2→cfn related-panel
	// pivot. acme-eks-cluster is a real stack fixture (cfn.go).
	if instanceID == "i-0a1b2c3d4e5f60005" {
		inst.Tags = append(inst.Tags,
			ec2types.Tag{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
		)
	}
	// aws:ecs:cluster-name tag — required for ecs→ec2 related-panel pivot.
	// acme-services is a real ECS cluster fixture (ecs.go). StateReason.Code
	// with a "Server." prefix is also this suite's only witness for the
	// broken (AWS-initiated stop) bucket of colorEC2/CodeEC2StateStoppedServer
	// — this instance is stopped + spot lifecycle, a natural fit for a
	// spot-interruption AWS-initiated stop.
	if instanceID == "i-0a1b2c3d4e5f60004" {
		inst.Tags = append(inst.Tags,
			ec2types.Tag{Key: aws.String("aws:ecs:cluster-name"), Value: aws.String("acme-services")},
		)
		inst.StateReason = &ec2types.StateReason{
			Code:    aws.String("Server.SpotInstanceShutdown"),
			Message: aws.String("Server.SpotInstanceShutdown: The instance was stopped because the Spot Instance was interrupted."),
		}
	}
	// aws:ecs:cluster-name tag — this is the acme-batch cluster's own
	// EC2-launch-type container host (ecs.go's batch-etl-runner task
	// carries this exact instance ID as its ContainerInstanceArn suffix).
	if instanceID == "e1f2a3b4c5d6e1f2a3b4c5d6" {
		inst.Tags = append(inst.Tags,
			ec2types.Tag{Key: aws.String("aws:ecs:cluster-name"), Value: aws.String("acme-batch")},
		)
	}
	// aws:ec2launchtemplate:id auto-tag — required for the lt->ec2 related-panel
	// pivot (checkLTEC2 cross-references the ec2 cache by this exact tag key).
	// Both instances launched from prod-web-lt (lt.go).
	if instanceID == "i-0a1b2c3d4e5f60006" || instanceID == "i-0a1b2c3d4e5f60007" {
		inst.Tags = append(inst.Tags,
			ec2types.Tag{Key: aws.String("aws:ec2launchtemplate:id"), Value: aws.String(ProdWebLTID)},
		)
	}
	// The bastion is the sole internet-exposure witness: a public address in
	// front of public-ssh-bad, whose port 22 is open to 0.0.0.0/0.
	if instanceID == EC2InstanceInternetExposed {
		inst.SecurityGroups = append(inst.SecurityGroups, ec2types.GroupIdentifier{
			GroupId:   aws.String("sg-0public0ssh000001"),
			GroupName: aws.String("public-ssh-bad"),
		})
	}
	if publicIP != "" {
		inst.PublicIpAddress = aws.String(publicIP)
	}
	return inst
}

// httpTokensFor keeps EC2InstanceIMDSv1 the single demo instance that still
// answers IMDSv1; every other instance requires a session token.
func httpTokensFor(instanceID string) ec2types.HttpTokensState {
	if instanceID == EC2InstanceIMDSv1 {
		return ec2types.HttpTokensStateOptional
	}
	return ec2types.HttpTokensStateRequired
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
		{"i-0a1b2c3d4e5f60002", "web-prod-02", "running", ec2types.InstanceTypeT3Large, "10.0.1.11", "", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 11, 15, 8, 32, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60003", "api-staging-01", "running", ec2types.InstanceTypeM5Xlarge, "10.0.2.50", "", fixtProdVPCID, fixtProdPublicSubnetB, time.Date(2026, 1, 20, 14, 15, 0, 0, time.UTC), ec2types.InstanceLifecycleTypeSpot},
		{"i-0a1b2c3d4e5f60004", "worker-batch-03", "stopped", ec2types.InstanceTypeC5Xlarge, "10.0.3.100", "", fixtProdVPCID, fixtProdPrivateSubnetA, time.Date(2025, 9, 5, 11, 0, 0, 0, time.UTC), ec2types.InstanceLifecycleTypeSpot},
		{"i-0a1b2c3d4e5f60005", "bastion-prod", "running", ec2types.InstanceTypeT3Micro, "10.0.0.5", "52.87.221.44", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60006", "db-proxy-01", "running", ec2types.InstanceTypeR5Large, "10.0.4.200", "", fixtProdVPCID, fixtProdPrivateSubnetB, time.Date(2025, 12, 10, 18, 45, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60007", "web-staging-01", "pending", ec2types.InstanceTypeT3Medium, "10.0.2.70", "", fixtProdVPCID, fixtProdPublicSubnetB, time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC), ec2types.InstanceLifecycleTypeSpot},
		{"i-0a1b2c3d4e5f60008", "ml-trainer-gpu", "stopping", ec2types.InstanceTypeG4dnXlarge, "10.0.5.30", "", fixtProdVPCID, fixtStagingSubnetA, time.Date(2026, 2, 14, 22, 0, 0, 0, time.UTC), ec2types.InstanceLifecycleTypeSpot},
		{"i-0a1b2c3d4e5f60009", "temp-load-test", "shutting-down", ec2types.InstanceTypeC5Large, "10.0.3.55", "", fixtProdVPCID, fixtProdPrivateSubnetA, time.Date(2026, 3, 20, 16, 30, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60010", "old-migration-worker", "terminated", ec2types.InstanceTypeT3Small, "", "", fixtProdVPCID, fixtProdPublicSubnetB, time.Date(2025, 8, 1, 12, 0, 0, 0, time.UTC), ""},
		// ASG-owned instances (see the matching namedExtras block above for
		// the graph-connection rationale) — AZ/subnet mirror the owning
		// ASG's VPCZoneIdentifier, launch time follows shortly after the
		// ASG's own CreatedTime, state/lifecycle mirror the ASG's
		// HealthStatus/LifecycleState (all InService -> "running").
		{"i-0aaa111111111111a", "api-worker-01", "running", ec2types.InstanceTypeM5Large, "10.0.10.10", "", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 6, 1, 10, 5, 0, 0, time.UTC), ""},
		{"i-0bbb222222222222b", "api-worker-02", "running", ec2types.InstanceTypeM5Large, "10.0.10.11", "", fixtProdVPCID, fixtProdPublicSubnetB, time.Date(2025, 6, 1, 10, 6, 0, 0, time.UTC), ""},
		{"i-0ccc333333333333c", "batch-processor-01", "running", ec2types.InstanceTypeC5Large, "10.0.10.20", "", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 4, 20, 8, 5, 0, 0, time.UTC), ""},
		{"i-0ddd444444444444d", "batch-processor-02", "running", ec2types.InstanceTypeC5Large, "10.0.10.21", "", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 4, 20, 8, 6, 0, 0, time.UTC), ""},
		{"i-0eee555555555555e", "web-worker-01", "running", ec2types.InstanceTypeT3Large, "10.0.10.30", "", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 5, 12, 8, 5, 0, 0, time.UTC), ""},
		{"i-0fff666666666666f", "web-worker-02", "running", ec2types.InstanceTypeT3Large, "10.0.10.31", "", fixtProdVPCID, fixtProdPublicSubnetB, time.Date(2025, 5, 12, 8, 6, 0, 0, time.UTC), ""},
		{"i-0aaa777777777777a", "web-worker-03", "running", ec2types.InstanceTypeT3Large, "10.0.10.32", "", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 5, 12, 8, 7, 0, 0, time.UTC), ""},
		{"i-0bbb888888888888b", "payments-worker-01", "running", ec2types.InstanceTypeC5Xlarge, "10.0.10.40", "", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 7, 1, 8, 5, 0, 0, time.UTC), ""},
		{"i-0ccc999999999999c", "payments-worker-02", "running", ec2types.InstanceTypeC5Xlarge, "10.0.10.41", "", fixtProdVPCID, fixtProdPublicSubnetB, time.Date(2025, 7, 1, 8, 6, 0, 0, time.UTC), ""},
		// ECS EC2-launch-type container host (acme-batch cluster) — see the
		// matching namedExtras entry above for the ContainerInstanceArn
		// graph-connection rationale.
		{"e1f2a3b4c5d6e1f2a3b4c5d6", "batch-etl-host-01", "running", ec2types.InstanceTypeM5Large, "10.0.10.50", "", fixtProdVPCID, fixtProdPrivateSubnetA, time.Date(2026, 3, 15, 6, 0, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60030", "dev-sandbox-01", "stopped", ec2types.InstanceTypeT3Medium, "10.1.0.20", "", fixtStagingVPCID, fixtStagingSubnetA, time.Date(2025, 10, 12, 7, 0, 0, 0, time.UTC), ""},
		{EC2InstanceHostileTag, EC2HostileTagValue, "stopped", ec2types.InstanceTypeT3Small, "10.1.0.21", "", fixtStagingVPCID, fixtStagingSubnetB, time.Date(2025, 10, 12, 7, 5, 0, 0, time.UTC), ""},
		// GPU inference fleet — the Cost Explorer growth story's resource-drill
		// target (costs.go's CostsGrowthService/CostsGrowthUsageType,
		// CostsResourceRowsByService): a g5.xlarge fleet scaled up around
		// CostsGrowthMonth and stayed running.
		{"i-0a1b2c3d4e5f60040", "ml-inference-01", "running", ec2types.InstanceTypeG5Xlarge, "10.0.6.10", "", fixtProdVPCID, fixtProdPrivateSubnetA, time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60041", "ml-inference-02", "running", ec2types.InstanceTypeG5Xlarge, "10.0.6.11", "", fixtProdVPCID, fixtProdPrivateSubnetA, time.Date(2026, 1, 12, 9, 0, 0, 0, time.UTC), ""},
		{"i-0a1b2c3d4e5f60042", "ml-inference-03", "running", ec2types.InstanceTypeG5Xlarge, "10.0.6.12", "", fixtProdVPCID, fixtProdPrivateSubnetA, time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC), ""},
		{EC2InstanceInternetExposedAll, "legacy-jump-host", "running", ec2types.InstanceTypeT3Micro, "10.0.0.6", "52.87.221.45", fixtProdVPCID, fixtProdPublicSubnetA, time.Date(2025, 6, 2, 9, 0, 0, 0, time.UTC), ""},
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
		// No generated instance carries a public address: ec2.public-ip and
		// ec2.internet-exposed each keep a single named witness above.
		publicIP := ""
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
		// api-worker-01 — witness for ec2.instance-status.insufficient-data
		// (AWS could not determine status from the hypervisor).
		"i-0aaa111111111111a": {"ok", "insufficient-data"},
	}
	// eventInstanceID is the sole witness for ec2.scheduled-event: a running
	// instance with ok status checks but a AWS-scheduled reboot within the
	// enricher's 7-day cutoff. Computed relative to time.Now() so the
	// fixture stays inside the window regardless of when the demo runs.
	const eventInstanceID = "i-0bbb222222222222b"

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
			status := ec2types.InstanceStatus{
				InstanceId:       aws.String(id),
				AvailabilityZone: inst.Placement.AvailabilityZone,
				InstanceState:    inst.State,
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatus(systemStatus),
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatus(instanceStatus),
				},
			}
			if id == eventInstanceID {
				status.Events = []ec2types.InstanceStatusEvent{
					{
						Code:        ec2types.EventCodeSystemReboot,
						Description: aws.String("Scheduled reboot for AWS hardware maintenance"),
						NotBefore:   aws.Time(time.Now().Add(3 * 24 * time.Hour)),
						NotAfter:    aws.Time(time.Now().Add(3*24*time.Hour + 4*time.Hour)),
					},
				}
			}
			statuses = append(statuses, status)
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
			// aws:cloudformation:stack-name tag — required for vpc→cfn
			// related-panel pivot. acme-eks-cluster is a real stack fixture
			// (cfn.go).
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-prod")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
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
		// VPCSubnetScopedFlowLog: covered by a flow log on its subnet, which
		// is how a team that logs one workload's traffic sets it up.
		{
			VpcId:           aws.String(VPCSubnetScopedFlowLog),
			CidrBlock:       aws.String("10.30.0.0/16"),
			State:           ec2types.VpcStateAvailable,
			IsDefault:       aws.Bool(false),
			InstanceTenancy: ec2types.TenancyDefault,
			DhcpOptionsId:   aws.String("dopt-0f10a1b2c3d4e5f60"),
			OwnerId:         aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-analytics")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
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
		// OpenSearch graph-root VPC — required for opensearch→vpc related-panel pivot.
		// The acme-logs domain's VPCOptions.VPCId = OpenSearchVPCID points here.
		{
			VpcId:           aws.String(OpenSearchVPCID),
			CidrBlock:       aws.String("10.20.0.0/16"),
			State:           ec2types.VpcStateAvailable,
			IsDefault:       aws.Bool(false),
			InstanceTenancy: ec2types.TenancyDefault,
			DhcpOptionsId:   aws.String("dopt-0opensearch00001"),
			OwnerId:         aws.String("123456789012"),
			CidrBlockAssociationSet: []ec2types.VpcCidrBlockAssociation{
				{
					AssociationId: aws.String("vpc-cidr-assoc-os-01"),
					CidrBlock:     aws.String("10.20.0.0/16"),
					CidrBlockState: &ec2types.VpcCidrBlockState{
						State: ec2types.VpcCidrBlockStateCodeAssociated,
					},
				},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-opensearch")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// Redis prod VPC — required for redis→vpc related-panel pivot.
		{
			VpcId:           aws.String(ProdRedisVpcID),
			CidrBlock:       aws.String("10.10.0.0/16"),
			State:           ec2types.VpcStateAvailable,
			IsDefault:       aws.Bool(false),
			InstanceTenancy: ec2types.TenancyDefault,
			DhcpOptionsId:   aws.String("dopt-0redis0000000001"),
			OwnerId:         aws.String("123456789012"),
			CidrBlockAssociationSet: []ec2types.VpcCidrBlockAssociation{
				{
					AssociationId: aws.String("vpc-cidr-assoc-redis-01"),
					CidrBlock:     aws.String("10.10.0.0/16"),
					CidrBlockState: &ec2types.VpcCidrBlockState{
						State: ec2types.VpcCidrBlockStateCodeAssociated,
					},
				},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-redis-prod")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// EFS prod VPC — required for efs→vpc related-panel pivot.
		{
			VpcId:           aws.String(ProdEFSVpcID),
			CidrBlock:       aws.String("10.20.0.0/16"),
			State:           ec2types.VpcStateAvailable,
			IsDefault:       aws.Bool(false),
			InstanceTenancy: ec2types.TenancyDefault,
			DhcpOptionsId:   aws.String("dopt-0efs0prod0000001"),
			OwnerId:         aws.String("123456789012"),
			CidrBlockAssociationSet: []ec2types.VpcCidrBlockAssociation{
				{
					AssociationId: aws.String("vpc-cidr-assoc-efs-prod-01"),
					CidrBlock:     aws.String("10.20.0.0/16"),
					CidrBlockState: &ec2types.VpcCidrBlockState{
						State: ec2types.VpcCidrBlockStateCodeAssociated,
					},
				},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-efs-prod")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// State=pending → wave1 finding (CodeVPCStatePending, SevWarn) → Warning.
		{
			VpcId:           aws.String("vpc-0pending111111111"),
			CidrBlock:       aws.String("10.30.0.0/16"),
			State:           ec2types.VpcStatePending,
			IsDefault:       aws.Bool(false),
			InstanceTenancy: ec2types.TenancyDefault,
			DhcpOptionsId:   aws.String("dopt-0pending000000001"),
			OwnerId:         aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-new-region-vpc")},
				{Key: aws.String("Environment"), Value: aws.String("dev")},
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
			// aws:cloudformation:stack-name tag — required for sg→cfn
			// related-panel pivot. acme-eks-cluster is a real stack fixture
			// (cfn.go).
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-web-alb-sg")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
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
			GroupId:     aws.String(fixtProdDBProxySGID),
			GroupName:   aws.String("acme-db-proxy-sg"),
			VpcId:       aws.String(fixtProdVPCID),
			Description: aws.String("RDS Proxy security group — allows DB access from app tier"),
			OwnerId:     aws.String("123456789012"),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(5432),
					ToPort:     aws.Int32(5432),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{GroupId: aws.String(fixtProdAPIInternalSGID), Description: aws.String("API internal tier")},
					},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-db-proxy-sg")},
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
		// SSH open to 0.0.0.0/0 → Risk column shows PORTS:22
		{
			GroupId:          aws.String("sg-0public0ssh000001"),
			GroupName:        aws.String("public-ssh-bad"),
			VpcId:            aws.String(fixtProdVPCID),
			Description:      aws.String("Misconfigured SG — SSH open to the world"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0public0ssh000001"),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(22),
					ToPort:     aws.Int32(22),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("public-ssh-bad")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// Every protocol open to 0.0.0.0/0 → wide_open, which the ec2
		// exposure signal reports as its own sentence rather than a port list.
		{
			GroupId:          aws.String("sg-0public0all000003"),
			GroupName:        aws.String("public-all-open"),
			VpcId:            aws.String(fixtProdVPCID),
			Description:      aws.String("Misconfigured SG — every protocol open to the world"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0public0all000003"),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("-1"),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("public-all-open")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// DB ports open to 0.0.0.0/0 → Risk column shows PORTS:3306,5432
		{
			GroupId:          aws.String("sg-0public0db0000002"),
			GroupName:        aws.String("public-db-very-bad"),
			VpcId:            aws.String(fixtProdVPCID),
			Description:      aws.String("Misconfigured SG — DB ports open to the world"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0public0db0000002"),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(3306),
					ToPort:     aws.Int32(3306),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(5432),
					ToPort:     aws.Int32(5432),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("public-db-very-bad")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// Redis prod security group — required for redis→sg related-panel pivot.
		// Matches fixtures.ProdRedisSGID on the prod-redis-sessions-001 member cluster.
		{
			GroupId:          aws.String(ProdRedisSGID),
			GroupName:        aws.String("acme-redis-prod-sg"),
			VpcId:            aws.String(ProdRedisVpcID),
			Description:      aws.String("Redis production security group — allows port 6379 from app tier"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + ProdRedisSGID),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(6379),
					ToPort:     aws.Int32(6379),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.10.0.0/16"), Description: aws.String("Redis from prod VPC")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-redis-prod-sg")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// Redshift warehouse security groups — required for redshift→sg related-panel pivot.
		// VpcSecurityGroups on acme-warehouse cluster reference RedshiftWarehouseSGID1 and RedshiftWarehouseSGID2.
		{
			GroupId:          aws.String(RedshiftWarehouseSGID1),
			GroupName:        aws.String("redshift-warehouse-sg-1"),
			VpcId:            aws.String(fixtProdVPCID),
			Description:      aws.String("Primary security group for acme-warehouse Redshift cluster"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + RedshiftWarehouseSGID1),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(5439),
					ToPort:     aws.Int32(5439),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("Redshift from prod VPC")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redshift-warehouse-sg-1")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			GroupId:          aws.String(RedshiftWarehouseSGID2),
			GroupName:        aws.String("redshift-warehouse-sg-2"),
			VpcId:            aws.String(fixtProdVPCID),
			Description:      aws.String("Secondary security group for acme-warehouse Redshift cluster"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + RedshiftWarehouseSGID2),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(5439),
					ToPort:     aws.Int32(5439),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("Redshift from prod VPC")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redshift-warehouse-sg-2")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// Redshift reporting security groups — required for redshift→sg related-panel pivot (second graph-root).
		{
			GroupId:          aws.String(RedshiftReportingSGID1),
			GroupName:        aws.String("redshift-reporting-sg-1"),
			VpcId:            aws.String(fixtProdVPCID),
			Description:      aws.String("Primary security group for acme-reporting Redshift cluster"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + RedshiftReportingSGID1),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(5439),
					ToPort:     aws.Int32(5439),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("Redshift from prod VPC")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redshift-reporting-sg-1")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			GroupId:          aws.String(RedshiftReportingSGID2),
			GroupName:        aws.String("redshift-reporting-sg-2"),
			VpcId:            aws.String(fixtProdVPCID),
			Description:      aws.String("Secondary security group for acme-reporting Redshift cluster"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + RedshiftReportingSGID2),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(5439),
					ToPort:     aws.Int32(5439),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("Redshift from prod VPC")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redshift-reporting-sg-2")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// Protocol -1 open to 0.0.0.0/0 → Risk column shows WIDE_OPEN
		{
			GroupId:          aws.String("sg-0wide0open0000003"),
			GroupName:        aws.String("wide-open-everything"),
			VpcId:            aws.String(fixtProdVPCID),
			Description:      aws.String("Critically misconfigured SG — all traffic open to the world"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0wide0open0000003"),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("-1"),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("wide-open-everything")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// EFS prod SG-A — required for efs→sg related-panel pivot (Count ≥ 2).
		// Attached to all three EFS mount-target ENIs in the ProdEFSVpcID VPC.
		{
			GroupId:          aws.String(ProdEFSSecurityGroupAID),
			GroupName:        aws.String("acme-efs-prod-sg-a"),
			VpcId:            aws.String(ProdEFSVpcID),
			Description:      aws.String("Primary security group for EFS prod mount targets — NFS port 2049"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + ProdEFSSecurityGroupAID),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(2049),
					ToPort:     aws.Int32(2049),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.20.0.0/16"), Description: aws.String("NFS from EFS VPC")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-efs-prod-sg-a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// EFS prod SG-B — second security group for efs→sg pivot so Count = 2.
		{
			GroupId:          aws.String(ProdEFSSecurityGroupBID),
			GroupName:        aws.String("acme-efs-prod-sg-b"),
			VpcId:            aws.String(ProdEFSVpcID),
			Description:      aws.String("Secondary security group for EFS prod mount targets — monitoring"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + ProdEFSSecurityGroupBID),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(2049),
					ToPort:     aws.Int32(2049),
					IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.20.0.0/16"), Description: aws.String("NFS from EFS VPC")}},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-efs-prod-sg-b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
	}

	// OpenSearch graph-root security groups — required for opensearch→sg related-panel pivot.
	// The acme-logs domain's VPCOptions.SecurityGroupIds = [sg-demo-a1, sg-demo-a2].
	sgs = append(sgs, ec2types.SecurityGroup{
		GroupId:          aws.String(OpenSearchSGA),
		GroupName:        aws.String("opensearch-data-sg"),
		VpcId:            aws.String(OpenSearchVPCID),
		Description:      aws.String("Security group for acme-logs OpenSearch data nodes"),
		OwnerId:          aws.String("123456789012"),
		SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + OpenSearchSGA),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(443),
				ToPort:     aws.Int32(443),
				IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.20.0.0/16"), Description: aws.String("HTTPS from VPC")}},
			},
		},
		IpPermissionsEgress: []ec2types.IpPermission{
			{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String("opensearch-data-sg")},
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		},
	})
	sgs = append(sgs, ec2types.SecurityGroup{
		GroupId:          aws.String(OpenSearchSGB),
		GroupName:        aws.String("opensearch-mgmt-sg"),
		VpcId:            aws.String(OpenSearchVPCID),
		Description:      aws.String("Security group for acme-logs OpenSearch management access"),
		OwnerId:          aws.String("123456789012"),
		SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + OpenSearchSGB),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(9200),
				ToPort:     aws.Int32(9200),
				IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.20.0.0/16"), Description: aws.String("OpenSearch API from VPC")}},
			},
		},
		IpPermissionsEgress: []ec2types.IpPermission{
			{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String("opensearch-mgmt-sg")},
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		},
	})

	// FTP control port open to the world — the witness for the widened
	// sensitive-port set (sgCodeDangerousPorts). No other demo group opens
	// any of the ports added alongside FTP.
	sgs = append(sgs, ec2types.SecurityGroup{
		GroupId:          aws.String(SGDangerousFTP),
		GroupName:        aws.String("acme-legacy-ftp-sg"),
		VpcId:            aws.String(fixtProdVPCID),
		Description:      aws.String("Legacy file-drop host — FTP open to the world"),
		OwnerId:          aws.String("123456789012"),
		SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + SGDangerousFTP),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(21),
				ToPort:     aws.Int32(21),
				IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
			},
		},
		IpPermissionsEgress: []ec2types.IpPermission{
			{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String("acme-legacy-ftp-sg")},
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		},
	})

	// The prod VPC's default group, still carrying the self-referencing
	// ingress rule AWS creates it with — the witness for
	// sgCodeDefaultWithRules. It is the only demo group named "default", so
	// every other row is healthy for that condition by construction.
	sgs = append(sgs, ec2types.SecurityGroup{
		GroupId:          aws.String(SGDefaultWithRules),
		GroupName:        aws.String("default"),
		VpcId:            aws.String(fixtProdVPCID),
		Description:      aws.String("default VPC security group"),
		OwnerId:          aws.String("123456789012"),
		SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + SGDefaultWithRules),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("-1"),
				UserIdGroupPairs: []ec2types.UserIdGroupPair{
					{GroupId: aws.String(SGDefaultWithRules), Description: aws.String("All traffic from members of this group")},
				},
			},
		},
		IpPermissionsEgress: []ec2types.IpPermission{
			{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String("default")},
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		},
	})

	// Left out of buildNetworkInterfaces' attachment pass — the single
	// witness for sgCodeUnused.
	sgs = append(sgs, ec2types.SecurityGroup{
		GroupId:          aws.String(SGUnused),
		GroupName:        aws.String("acme-decommissioned-batch-sg"),
		VpcId:            aws.String(fixtProdVPCID),
		Description:      aws.String("Batch tier retired in 2025 — group never deleted"),
		OwnerId:          aws.String("123456789012"),
		SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/" + SGUnused),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(8080),
				ToPort:     aws.Int32(8080),
				IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("Batch API from VPC")}},
			},
		},
		IpPermissionsEgress: []ec2types.IpPermission{
			{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String("acme-decommissioned-batch-sg")},
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		},
	})

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
		// The one subnet of VPCSubnetScopedFlowLog, and the resource its flow
		// log is attached to.
		{
			SubnetId:                aws.String(vpcSubnetScopedFlowLogSubnet),
			VpcId:                   aws.String(VPCSubnetScopedFlowLog),
			CidrBlock:               aws.String("10.30.1.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			AvailabilityZoneId:      aws.String("use1-az1"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(250),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + vpcSubnetScopedFlowLogSubnet),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("analytics-private-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
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
			// aws:cloudformation:stack-name tag — required for subnet→cfn
			// related-panel pivot. acme-eks-cluster is a real stack fixture
			// (cfn.go).
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-public-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("public")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
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
			MapPublicIpOnLaunch:     aws.Bool(false),
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
		// OpenSearch graph-root subnets — required for opensearch→subnet related-panel pivot.
		// The acme-logs domain's VPCOptions.SubnetIds = [subnet-demo-a1, subnet-demo-a2].
		{
			SubnetId:                aws.String(OpenSearchSubnetA),
			VpcId:                   aws.String(OpenSearchVPCID),
			CidrBlock:               aws.String("10.20.1.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			AvailabilityZoneId:      aws.String("use1-az1"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(243),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + OpenSearchSubnetA),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("opensearch-private-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("search")},
			},
		},
		{
			SubnetId:                aws.String(OpenSearchSubnetB),
			VpcId:                   aws.String(OpenSearchVPCID),
			CidrBlock:               aws.String("10.20.2.0/24"),
			AvailabilityZone:        aws.String("us-east-1b"),
			AvailabilityZoneId:      aws.String("use1-az2"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(243),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + OpenSearchSubnetB),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("opensearch-private-1b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("search")},
			},
		},
		// Redshift prod subnet group subnets — required for redshift→subnet related-panel pivot.
		// These are returned by DescribeClusterSubnetGroups for RedshiftProdSubnetGroup.
		{
			SubnetId:                aws.String("subnet-prod-a"),
			VpcId:                   aws.String(fixtProdVPCID),
			CidrBlock:               aws.String("10.0.10.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			AvailabilityZoneId:      aws.String("use1-az1"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(240),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-prod-a"),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redshift-prod-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("data")},
			},
		},
		{
			SubnetId:                aws.String("subnet-prod-b"),
			VpcId:                   aws.String(fixtProdVPCID),
			CidrBlock:               aws.String("10.0.11.0/24"),
			AvailabilityZone:        aws.String("us-east-1b"),
			AvailabilityZoneId:      aws.String("use1-az2"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(240),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-prod-b"),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redshift-prod-1b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("data")},
			},
		},
		// Redshift staging subnet group subnets — required for redshift→subnet related-panel pivot.
		// Returned by DescribeClusterSubnetGroups for RedshiftStagingSubnetGroup.
		{
			SubnetId:                aws.String("subnet-staging-a"),
			VpcId:                   aws.String(fixtStagingVPCID),
			CidrBlock:               aws.String("10.1.10.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			AvailabilityZoneId:      aws.String("use1-az1"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(240),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-staging-a"),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redshift-staging-1a")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
				{Key: aws.String("Tier"), Value: aws.String("data")},
			},
		},
		{
			SubnetId:                aws.String("subnet-staging-b"),
			VpcId:                   aws.String(fixtStagingVPCID),
			CidrBlock:               aws.String("10.1.11.0/24"),
			AvailabilityZone:        aws.String("us-east-1b"),
			AvailabilityZoneId:      aws.String("use1-az2"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(240),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-staging-b"),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redshift-staging-1b")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
				{Key: aws.String("Tier"), Value: aws.String("data")},
			},
		},
		// Redis prod subnets — required for redis→subnet related-panel pivot.
		{
			SubnetId:                aws.String(ProdRedisSubnetA),
			VpcId:                   aws.String(ProdRedisVpcID),
			CidrBlock:               aws.String("10.10.1.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			AvailabilityZoneId:      aws.String("use1-az1"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(251),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + ProdRedisSubnetA),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redis-private-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("cache")},
			},
		},
		{
			SubnetId:                aws.String(ProdRedisSubnetB),
			VpcId:                   aws.String(ProdRedisVpcID),
			CidrBlock:               aws.String("10.10.2.0/24"),
			AvailabilityZone:        aws.String("us-east-1b"),
			AvailabilityZoneId:      aws.String("use1-az2"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(251),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + ProdRedisSubnetB),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("redis-private-1b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("cache")},
			},
		},
		// EFS prod subnets — required for efs→subnet related-panel pivot (Count = 3).
		{
			SubnetId:                aws.String(ProdEFSSubnetAID),
			VpcId:                   aws.String(ProdEFSVpcID),
			CidrBlock:               aws.String("10.20.1.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			AvailabilityZoneId:      aws.String("use1-az1"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(251),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + ProdEFSSubnetAID),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("efs-private-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("storage")},
			},
		},
		{
			SubnetId:                aws.String(ProdEFSSubnetBID),
			VpcId:                   aws.String(ProdEFSVpcID),
			CidrBlock:               aws.String("10.20.2.0/24"),
			AvailabilityZone:        aws.String("us-east-1b"),
			AvailabilityZoneId:      aws.String("use1-az2"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(251),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + ProdEFSSubnetBID),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("efs-private-1b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("storage")},
			},
		},
		{
			SubnetId:                aws.String(ProdEFSSubnetCID),
			VpcId:                   aws.String(ProdEFSVpcID),
			CidrBlock:               aws.String("10.20.3.0/24"),
			AvailabilityZone:        aws.String("us-east-1c"),
			AvailabilityZoneId:      aws.String("use1-az3"),
			State:                   ec2types.SubnetStateAvailable,
			AvailableIpAddressCount: aws.Int32(251),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + ProdEFSSubnetCID),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("efs-private-1c")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Tier"), Value: aws.String("storage")},
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
			MapPublicIpOnLaunch:     aws.Bool(false),
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
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-public-1b")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
	}

	// State=pending → Warning (colorSubnet). State=unavailable → Broken.
	named = append(named,
		ec2types.Subnet{
			SubnetId:                aws.String("subnet-0pending1111111a"),
			VpcId:                   aws.String(fixtStagingVPCID),
			CidrBlock:               aws.String("10.1.20.0/24"),
			AvailabilityZone:        aws.String("us-east-1a"),
			State:                   ec2types.SubnetStatePending,
			AvailableIpAddressCount: aws.Int32(256),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-pending-subnet")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		ec2types.Subnet{
			SubnetId:                aws.String("subnet-0unavail111111b"),
			VpcId:                   aws.String(fixtStagingVPCID),
			CidrBlock:               aws.String("10.1.21.0/24"),
			AvailabilityZone:        aws.String("us-east-1b"),
			State:                   ec2types.SubnetStateUnavailable,
			AvailableIpAddressCount: aws.Int32(0),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-unavailable-subnet")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		// State=failed → wave1 finding (CodeSubnetStateFailed, SevBroken) → Broken.
		ec2types.Subnet{
			SubnetId:                aws.String("subnet-0failed111111c"),
			VpcId:                   aws.String(fixtProdVPCID),
			CidrBlock:               aws.String("10.0.22.0/24"),
			AvailabilityZone:        aws.String("us-east-1c"),
			State:                   ec2types.SubnetState("failed"),
			AvailableIpAddressCount: aws.Int32(0),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-failed-subnet")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// State=failed-insufficient-capacity → wave1 finding
		// (CodeSubnetStateFailedInsufficientCapacity, SevBroken) → Broken.
		ec2types.Subnet{
			SubnetId:                aws.String("subnet-0failedcap1111d"),
			VpcId:                   aws.String(fixtStagingVPCID),
			CidrBlock:               aws.String("10.1.22.0/24"),
			AvailabilityZone:        aws.String("us-east-1c"),
			State:                   ec2types.SubnetStateFailedInsufficientCapacity,
			AvailableIpAddressCount: aws.Int32(0),
			MapPublicIpOnLaunch:     aws.Bool(false),
			DefaultForAz:            aws.Bool(false),
			OwnerId:                 aws.String("123456789012"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-failed-insufficient-capacity-subnet")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
	)

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
				// required for rtb→vpc-peer related-panel pivot (one of the
				// two rtb fixtures routing into ProdPeerSharedID, vpcpeer.go).
				{DestinationCidrBlock: aws.String("192.168.0.0/16"), VpcPeeringConnectionId: aws.String(ProdPeerSharedID), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			},
			Associations: []ec2types.RouteTableAssociation{
				{Main: aws.Bool(true), RouteTableAssociationId: aws.String("rtbassoc-0aaa111111111111a"), RouteTableId: aws.String("rtb-0aaa111111111111a")},
				{Main: aws.Bool(false), RouteTableAssociationId: aws.String("rtbassoc-0aaa222222222222a"), RouteTableId: aws.String("rtb-0aaa111111111111a"), SubnetId: aws.String(fixtProdPrivateSubnetA)},
			},
			// aws:cloudformation:stack-name tag — required for rtb→cfn
			// related-panel pivot. acme-eks-cluster is a real stack fixture
			// (cfn.go), matching the same pattern used on ec2/sg/vpc.
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-main")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
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
				// required for rtb→eni related-panel pivot (checkRTBENI):
				// route via a real ENI in this VPC (eni-0eee555555555555e, ec2.go).
				{DestinationCidrBlock: aws.String("192.168.100.0/24"), NetworkInterfaceId: aws.String("eni-0eee555555555555e"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
				// required for rtb→tgw related-panel pivot (checkRTBTGW):
				// route via the hub transit gateway (tgw-0aaa111111111111a, this file).
				{DestinationCidrBlock: aws.String("10.5.0.0/16"), TransitGatewayId: aws.String("tgw-0aaa111111111111a"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
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
				// required for rtb→vpc-peer related-panel pivot (second of
				// the two rtb fixtures routing into ProdPeerSharedID,
				// vpcpeer.go — the rtb pivot's ≥2 witness).
				{DestinationCidrBlock: aws.String("192.168.0.0/16"), VpcPeeringConnectionId: aws.String(ProdPeerSharedID), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
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
				// required for rtb→vpc-peer related-panel pivot: the route to
				// WarnPeerBlackholeID (vpcpeer.go) is blackholed even though
				// the peering connection itself is Status.Code=active — the
				// "route to peer blackholed" cache-scan witness.
				{DestinationCidrBlock: aws.String("10.30.0.0/16"), VpcPeeringConnectionId: aws.String(WarnPeerBlackholeID), State: ec2types.RouteStateBlackhole, Origin: ec2types.RouteOriginCreateRoute},
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
		// Blackhole route (target deleted) → Broken (colorRTB).
		{
			RouteTableId: aws.String("rtb-0blackhole1111111e"),
			VpcId:        aws.String(fixtStagingVPCID),
			OwnerId:      aws.String("123456789012"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.2.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRouteTable},
				{DestinationCidrBlock: aws.String("0.0.0.0/0"), NatGatewayId: aws.String("nat-0deleted11111111f"), State: ec2types.RouteStateBlackhole, Origin: ec2types.RouteOriginCreateRoute},
			},
			Associations: []ec2types.RouteTableAssociation{
				{Main: aws.Bool(false), RouteTableAssociationId: aws.String("rtbassoc-0hhh888888888888h"), RouteTableId: aws.String("rtb-0blackhole1111111e"), SubnetId: aws.String(fixtStagingSubnetB)},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-blackhole-rtb")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		// Zero associations, not main → Warning (colorRTB).
		{
			RouteTableId: aws.String("rtb-0orphan111111111f"),
			VpcId:        aws.String(fixtStagingVPCID),
			OwnerId:      aws.String("123456789012"),
			Routes: []ec2types.Route{
				{DestinationCidrBlock: aws.String("10.2.0.0/16"), GatewayId: aws.String("local"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRouteTable},
			},
			Associations: nil,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-orphan-rtb")},
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
			// NetworkInterfaceId — required for nat→eni and eni→nat
			// related-panel pivots (checkNATENI / checkENINAT). Matches
			// eni-0nat0000000000002b (this file, buildNetworkInterfaces).
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{AllocationId: aws.String("eipalloc-0bbb222222222222b"), PublicIp: aws.String("54.210.33.201"), PrivateIp: aws.String("10.0.2.50"), IsPrimary: aws.Bool(true), NetworkInterfaceId: aws.String("eni-0nat0000000000002b")},
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
		// State=FAILED with FailureCode → Failure column shows "InsufficientFreeAddressesInSubnet"
		{
			NatGatewayId:     aws.String("nat-0failed111111111d"),
			VpcId:            aws.String(fixtStagingVPCID),
			SubnetId:         aws.String(fixtStagingSubnetB),
			State:            ec2types.NatGatewayStateFailed,
			ConnectivityType: ec2types.ConnectivityTypePublic,
			CreateTime:       aws.Time(time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)),
			FailureCode:      aws.String("InsufficientFreeAddressesInSubnet"),
			FailureMessage:   aws.String("Subnet has insufficient free addresses to create the requested number of Network Interfaces"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("nat-failed-no-addresses")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		// State=PENDING → Warning, no failure info
		{
			NatGatewayId:     aws.String("nat-0pending11111111e"),
			VpcId:            aws.String(fixtProdVPCID),
			SubnetId:         aws.String(fixtProdPublicSubnetA),
			State:            ec2types.NatGatewayStatePending,
			ConnectivityType: ec2types.ConnectivityTypePublic,
			CreateTime:       aws.Time(time.Date(2026, 4, 18, 8, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("nat-pending")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// State=DELETED, no wave1 finding emitted for "deleted" → falls through
		// to colorNAT's structural switch, which maps it to Dim.
		{
			NatGatewayId:     aws.String("nat-0deleted11111111f"),
			VpcId:            aws.String(fixtStagingVPCID),
			SubnetId:         aws.String(fixtStagingSubnetB),
			State:            ec2types.NatGatewayStateDeleted,
			ConnectivityType: ec2types.ConnectivityTypePublic,
			CreateTime:       aws.Time(time.Date(2025, 5, 1, 8, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("nat-decommissioned")},
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
		// No attachments → attachments_count==0 → Warning (colorIGW).
		{
			InternetGatewayId: aws.String("igw-0unattached111111c"),
			OwnerId:           aws.String("123456789012"),
			Attachments:       nil,
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("spare-unattached-igw")},
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
		// Attachment state=attaching → wave1 finding (CodeIGWStateAttaching, SevWarn) → Warning.
		{
			InternetGatewayId: aws.String("igw-0attaching111111d"),
			OwnerId:           aws.String("123456789012"),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String("vpc-0pending111111111"), State: ec2types.AttachmentStatusAttaching},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("new-region-igw-attaching")},
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
		// Attachment state=detaching → wave1 finding (CodeIGWStateDetaching, SevWarn) → Warning.
		{
			InternetGatewayId: aws.String("igw-0detaching111111e"),
			OwnerId:           aws.String("123456789012"),
			Attachments: []ec2types.InternetGatewayAttachment{
				{VpcId: aws.String(fixtStagingVPCID), State: ec2types.AttachmentStatusDetaching},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-igw-detaching")},
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
			AllocationId: aws.String("eipalloc-0fff666666666666f"), PublicIp: aws.String("54.210.33.112"),
			AssociationId: aws.String("eipassoc-0fff666666666666f"), InstanceId: aws.String("i-0a1b2c3d4e5f60001"),
			SubnetId: aws.String(fixtProdPublicSubnetA), Domain: ec2types.DomainTypeVpc,
			NetworkBorderGroup: aws.String("us-east-1"), NetworkInterfaceId: aws.String("eni-0aaa111111111111a"),
			PrivateIpAddress: aws.String("10.0.1.10"),
			// aws:cloudformation:stack-name tag — required for eip→cfn related-panel
			// pivot. acme-eks-cluster is a real stack fixture (cfn.go).
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("web-prod-01-eip")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
			},
		},
		// nat-0aaa111111111111a's and nat-0bbb222222222222b's own addresses. A
		// NAT gateway allocation cannot carry an InstanceId, so these rows have
		// none; they exist so the nat→eip pivot, which resolves by
		// AllocationId, has a row to land on. Each keeps its association id,
		// which is what an attached address has and what keeps it out of the
		// unassociated finding.
		{
			AllocationId: aws.String("eipalloc-0aaa111111111111a"), PublicIp: aws.String("54.210.33.200"),
			AssociationId: aws.String("eipassoc-0aaa111111111111a"),
			Domain:        ec2types.DomainTypeVpc, NetworkBorderGroup: aws.String("us-east-1"),
			PrivateIpAddress: aws.String("10.0.1.50"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-nat-eip-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			AllocationId: aws.String("eipalloc-0bbb222222222222b"), PublicIp: aws.String("54.210.33.201"),
			AssociationId: aws.String("eipassoc-0bbb222222222222b"),
			Domain:        ec2types.DomainTypeVpc, NetworkBorderGroup: aws.String("us-east-1"),
			NetworkInterfaceId: aws.String("eni-0nat0000000000002b"), PrivateIpAddress: aws.String("10.0.2.50"),
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
			// nat-0ccc333333333333c's own address, in the staging VPC.
			AllocationId: aws.String("eipalloc-0ccc333333333333c"), PublicIp: aws.String("52.87.100.10"),
			AssociationId: aws.String("eipassoc-0ccc333333333333c"),
			Domain:        ec2types.DomainTypeVpc, NetworkBorderGroup: aws.String("us-east-1"),
			PrivateIpAddress: aws.String("10.1.1.50"),
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
		// transfer eip pivot — three unattached addresses reserved for the
		// prod-as2-gateway Transfer Family server's EndpointDetails.
		// AddressAllocationIds (transfer.go). Documentation-range PublicIps
		// (RFC 5737 TEST-NET-3, 203.0.113.0/24) since these are not actually
		// bound to any ENI/instance in the demo graph.
		{
			AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60a1a"), PublicIp: aws.String("203.0.113.10"),
			Domain: ec2types.DomainTypeVpc, NetworkBorderGroup: aws.String("us-east-1"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-as2-gateway-eip-a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60a1b"), PublicIp: aws.String("203.0.113.11"),
			Domain: ec2types.DomainTypeVpc, NetworkBorderGroup: aws.String("us-east-1"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-as2-gateway-eip-b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60a1c"), PublicIp: aws.String("203.0.113.12"),
			Domain: ec2types.DomainTypeVpc, NetworkBorderGroup: aws.String("us-east-1"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-as2-gateway-eip-c")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
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
		// State=failed → wave1 finding (CodeTGWStateFailed, SevBroken) → Broken.
		{
			TransitGatewayId:  aws.String("tgw-0failed11111111d"),
			TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0failed11111111d"),
			State:             ec2types.TransitGatewayState("failed"),
			OwnerId:           aws.String("123456789012"),
			Description:       aws.String("Failed transit gateway creation — quota exceeded"),
			CreationTime:      aws.Time(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-failed-tgw")},
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
		// State=deleted: fetcher emits no wave1 finding for "deleted" → falls
		// through to colorTGW's structural switch, which maps it to Dim.
		{
			TransitGatewayId:  aws.String("tgw-0deleted11111111e"),
			TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0deleted11111111e"),
			State:             ec2types.TransitGatewayStateDeleted,
			OwnerId:           aws.String("123456789012"),
			Description:       aws.String("Decommissioned legacy transit gateway"),
			CreationTime:      aws.Time(time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-legacy-tgw")},
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
		// State=pending → wave1 finding (CodeTGWStatePending, SevWarn) → Warning.
		{
			TransitGatewayId:  aws.String("tgw-0pending111111111f"),
			TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0pending111111111f"),
			State:             ec2types.TransitGatewayState("pending"),
			OwnerId:           aws.String("123456789012"),
			Description:       aws.String("New region transit gateway — provisioning"),
			CreationTime:      aws.Time(time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-new-region-tgw")},
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
		// State=modifying → wave1 finding (CodeTGWStateModifying, SevWarn) → Warning.
		{
			TransitGatewayId:  aws.String("tgw-0modifying111111g"),
			TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0modifying111111g"),
			State:             ec2types.TransitGatewayStateModifying,
			OwnerId:           aws.String("123456789012"),
			Description:       aws.String("Disaster recovery transit gateway — ASN change in progress"),
			CreationTime:      aws.Time(time.Date(2025, 9, 15, 14, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-dr-tgw-modifying")},
				{Key: aws.String("Environment"), Value: aws.String("dr")},
			},
		},
		// State=available, and (per buildTGWAttachments below) its only
		// attachment is also available → EnrichTGWAttachments raises no
		// finding. The only demo TGW that resolves to colorTGW's structural
		// Healthy branch.
		{
			TransitGatewayId:  aws.String(HealthyTGWID),
			TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/" + HealthyTGWID),
			State:             ec2types.TransitGatewayStateAvailable,
			OwnerId:           aws.String("123456789012"),
			Description:       aws.String("Spoke transit gateway with a single healthy VPC attachment"),
			CreationTime:      aws.Time(time.Date(2025, 11, 1, 9, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-spoke-tgw")},
				{Key: aws.String("Environment"), Value: aws.String("shared")},
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
		// DR TGW → attachment State=failed → EnrichTGWAttachments emits
		// tgw.attachment-failed ("!") on tgw-0bbb222222222222b.
		{
			TransitGatewayAttachmentId: aws.String("tgw-attach-0eee555555555555e"),
			TransitGatewayId:           aws.String("tgw-0bbb222222222222b"),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpn,
			ResourceId:                 aws.String("vpn-0dr0000000000001e"),
			State:                      ec2types.TransitGatewayAttachmentStateFailed,
			TransitGatewayOwnerId:      aws.String("123456789012"),
			ResourceOwnerId:            aws.String("123456789012"),
			CreationTime:               aws.Time(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("dr-tgw-vpn-failed")},
			},
		},
		// Modifying TGW → attachment State=failed. The gateway's own wave-1
		// finding is "modifying" (SevWarn) and the enricher appends
		// tgw.attachment-failed (SevBroken) AFTER it, so this is the one demo
		// row of the five networking types whose finding slice runs
		// [Warn, Broken]. Picking the head instead of the worst renders it
		// yellow reading "modifying"; TGWMixedSeverity is what makes that
		// visible.
		{
			TransitGatewayAttachmentId: aws.String("tgw-attach-0mixed11111111i"),
			TransitGatewayId:           aws.String(TGWMixedSeverity),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpn,
			ResourceId:                 aws.String("vpn-0mixed0000000001i"),
			State:                      ec2types.TransitGatewayAttachmentStateFailed,
			TransitGatewayOwnerId:      aws.String("123456789012"),
			ResourceOwnerId:            aws.String("123456789012"),
			CreationTime:               aws.Time(time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("modifying-tgw-vpn-failed")},
			},
		},
		// Hub TGW → attachment State=modifying → EnrichTGWAttachments emits
		// tgw.attachment-transitional ("~") on tgw-0aaa111111111111a. Kept off
		// the DR TGW so the "!" attachment-failed witness there does not mask
		// this "~" finding under EnrichTGWAttachments's worst-wins precedence.
		{
			TransitGatewayAttachmentId: aws.String("tgw-attach-0fff666666666666f"),
			TransitGatewayId:           aws.String("tgw-0aaa111111111111a"),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpn,
			ResourceId:                 aws.String("vpn-0hub0000000000002f"),
			State:                      ec2types.TransitGatewayAttachmentStateModifying,
			TransitGatewayOwnerId:      aws.String("123456789012"),
			ResourceOwnerId:            aws.String("123456789012"),
			CreationTime:               aws.Time(time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("hub-tgw-vpn-modifying")},
			},
		},
		// HealthyTGWID's only attachment → staging VPC, left Available.
		{
			TransitGatewayAttachmentId: aws.String("tgw-attach-0healthy1111h"),
			TransitGatewayId:           aws.String(HealthyTGWID),
			ResourceType:               ec2types.TransitGatewayAttachmentResourceTypeVpc,
			ResourceId:                 aws.String(fixtStagingVPCID),
			State:                      ec2types.TransitGatewayAttachmentStateAvailable,
			TransitGatewayOwnerId:      aws.String("123456789012"),
			ResourceOwnerId:            aws.String("123456789012"),
			CreationTime:               aws.Time(time.Date(2025, 11, 1, 9, 5, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("spoke-tgw-staging-vpc")},
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
		// State=Failed → wave1 finding (CodeVPCEStateFailed, SevBroken) → Broken.
		{
			VpcEndpointId:     aws.String("vpce-0failed111111111e"),
			ServiceName:       aws.String("com.amazonaws.us-east-1.sts"),
			VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
			State:             ec2types.StateFailed,
			VpcId:             aws.String(fixtStagingVPCID),
			SubnetIds:         []string{fixtStagingSubnetA},
			PrivateDnsEnabled: aws.Bool(true),
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: aws.Time(time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-sts-endpoint-failed")},
			},
		},
		// State=Deleted: fetcher emits no wave1 finding for "Deleted" → falls
		// through to colorVPCE's structural switch, which maps it to Dim.
		{
			VpcEndpointId:     aws.String("vpce-0deleted111111111f"),
			ServiceName:       aws.String("com.amazonaws.us-east-1.sns"),
			VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
			State:             ec2types.StateDeleted,
			VpcId:             aws.String(fixtStagingVPCID),
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: aws.Time(time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-sns-endpoint-deleted")},
			},
		},
		// State=PendingAcceptance → wave1 finding (CodeVPCEStatePendingAcceptance, SevWarn) → Warning.
		{
			VpcEndpointId:     aws.String("vpce-0pendingaccept001g"),
			ServiceName:       aws.String("com.amazonaws.us-east-1.execute-api"),
			VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
			State:             ec2types.StatePendingAcceptance,
			VpcId:             aws.String(fixtProdVPCID),
			SubnetIds:         []string{fixtProdPrivateSubnetA},
			PrivateDnsEnabled: aws.Bool(false),
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: aws.Time(time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-execute-api-endpoint-pending-accept")},
			},
		},
		// State=Deleting → wave1 finding (CodeVPCEStateDeleting, SevWarn) → Warning.
		{
			VpcEndpointId:     aws.String("vpce-0deleting0000001h"),
			ServiceName:       aws.String("com.amazonaws.us-east-1.ecr.api"),
			VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
			State:             ec2types.StateDeleting,
			VpcId:             aws.String(fixtStagingVPCID),
			SubnetIds:         []string{fixtStagingSubnetA},
			PrivateDnsEnabled: aws.Bool(true),
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: aws.Time(time.Date(2025, 11, 1, 10, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-ecr-api-endpoint-deleting")},
			},
		},
		// State=Rejected → wave1 finding (CodeVPCEStateRejected, SevBroken) → Broken.
		{
			VpcEndpointId:     aws.String("vpce-0rejected0000001i"),
			ServiceName:       aws.String("com.amazonaws.us-east-1.kinesis-streams"),
			VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
			State:             ec2types.StateRejected,
			VpcId:             aws.String(fixtProdVPCID),
			SubnetIds:         []string{fixtProdPrivateSubnetB},
			PrivateDnsEnabled: aws.Bool(false),
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: aws.Time(time.Date(2026, 3, 5, 9, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-kinesis-endpoint-rejected")},
			},
		},
		// State=Expired → wave1 finding (CodeVPCEStateExpired, SevBroken) → Broken.
		{
			VpcEndpointId:     aws.String("vpce-0expired0000001j"),
			ServiceName:       aws.String("com.amazonaws.us-east-1.sqs"),
			VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
			State:             ec2types.StateExpired,
			VpcId:             aws.String(fixtStagingVPCID),
			SubnetIds:         []string{fixtStagingSubnetB},
			PrivateDnsEnabled: aws.Bool(false),
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: aws.Time(time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("staging-sqs-endpoint-expired")},
			},
		},
		// State=Partial → wave1 finding (CodeVPCEStatePartial, SevBroken) → Broken.
		{
			VpcEndpointId:     aws.String("vpce-0partial00000001k"),
			ServiceName:       aws.String("com.amazonaws.us-east-1.logs"),
			VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
			State:             ec2types.StatePartial,
			VpcId:             aws.String(fixtProdVPCID),
			SubnetIds:         []string{fixtProdPrivateSubnetA, fixtProdPrivateSubnetB},
			PrivateDnsEnabled: aws.Bool(true),
			OwnerId:           aws.String("123456789012"),
			CreationTimestamp: aws.Time(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-logs-endpoint-partial")},
			},
		},
		// prod-as2-gateway's auto-created VPC endpoint (transfer.go graph
		// root) — required for the transfer→vpce related-panel pivot to
		// drill through to a real row.
		{
			VpcEndpointId:       aws.String(ProdAS2GatewayVpcEndpointID),
			ServiceName:         aws.String("com.amazonaws.us-east-1.transfer.server"),
			VpcEndpointType:     ec2types.VpcEndpointTypeInterface,
			State:               ec2types.StateAvailable,
			VpcId:               aws.String(fixtProdVPCID),
			SubnetIds:           []string{fixtProdPublicSubnetA, fixtProdPublicSubnetB, fixtProdPrivateSubnetA},
			NetworkInterfaceIds: []string{"eni-0a1b2c3d4e5f60a2a", "eni-0a1b2c3d4e5f60a2b", "eni-0a1b2c3d4e5f60a2c"},
			PrivateDnsEnabled:   aws.Bool(false),
			OwnerId:             aws.String("123456789012"),
			CreationTimestamp:   aws.Time(time.Date(2025, 10, 1, 9, 0, 0, 0, time.UTC)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-as2-gateway-transfer-endpoint")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Network Interfaces
// ---------------------------------------------------------------------------

// buildNetworkInterfaces returns the hand-written interfaces plus one
// attachment per security group that none of them already reference, so the
// sg.unused signal has exactly one witness (SGUnused) instead of firing on
// every group whose owning service the fixtures model without its ENIs.
// Default groups are skipped: AWS creates one per VPC and it is exempt from
// the check.
func buildNetworkInterfaces(sgs []ec2types.SecurityGroup) []ec2types.NetworkInterface {
	enis := namedNetworkInterfaces()
	referenced := make(map[string]bool, len(sgs))
	for _, eni := range enis {
		for _, g := range eni.Groups {
			referenced[aws.ToString(g.GroupId)] = true
		}
	}
	for i, sg := range sgs {
		groupID := aws.ToString(sg.GroupId)
		if groupID == "" || groupID == SGUnused || referenced[groupID] || aws.ToString(sg.GroupName) == "default" {
			continue
		}
		referenced[groupID] = true
		name := aws.ToString(sg.GroupName)
		enis = append(enis, ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(fmt.Sprintf("eni-0sg%017x", i)),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              sg.VpcId,
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String(fmt.Sprintf("10.0.%d.%d", 100+i/250, 10+i%250)),
			MacAddress:         aws.String(fmt.Sprintf("0a:1b:2c:3d:5e:%02x", i%256)),
			Description:        aws.String("Workload interface for " + name),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(false),
			SourceDestCheck:    aws.Bool(true),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: sg.GroupId, GroupName: sg.GroupName},
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String(name + "-eni")}},
		})
	}
	return enis
}

func namedNetworkInterfaces() []ec2types.NetworkInterface {
	return []ec2types.NetworkInterface{
		{
			// web-prod-01's interface: the instance's own NetworkInterfaces
			// list names this one, and nat-0aaa111111111111a has no interface
			// of its own to confuse it with.
			NetworkInterfaceId: aws.String("eni-0aaa111111111111a"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPublicSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.1.10"),
			PrivateDnsName:     aws.String("ip-10-0-1-10.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:01"),
			Description:        aws.String("Primary network interface for web-prod-01"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(false),
			SourceDestCheck:    aws.Bool(true),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				AttachmentId: aws.String("eni-attach-01"), InstanceId: aws.String("i-0a1b2c3d4e5f60001"),
				DeviceIndex: aws.Int32(0), Status: ec2types.AttachmentStatusAttached, DeleteOnTermination: aws.Bool(true),
			},
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			},
			Association: &ec2types.NetworkInterfaceAssociation{
				PublicIp: aws.String("54.210.33.112"), PublicDnsName: aws.String("ec2-54-210-33-112.compute-1.amazonaws.com"),
				IpOwnerId: aws.String("amazon"), AllocationId: aws.String("eipalloc-0fff666666666666f"),
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web-prod-01-primary")}},
		},
		{
			// web-prod-02's interface. nat-0bbb222222222222b has its own,
			// eni-0nat0000000000002b.
			NetworkInterfaceId: aws.String("eni-0bbb222222222222b"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPublicSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.1.11"),
			PrivateDnsName:     aws.String("ip-10-0-1-11.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:02"),
			Description:        aws.String("Primary network interface for web-prod-02"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(false),
			SourceDestCheck:    aws.Bool(true),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				AttachmentId: aws.String("eni-attach-02"), InstanceId: aws.String("i-0a1b2c3d4e5f60002"),
				DeviceIndex: aws.Int32(0), Status: ec2types.AttachmentStatusAttached, DeleteOnTermination: aws.Bool(true),
			},
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web-prod-02-eni")}},
		},
		{
			NetworkInterfaceId: aws.String("eni-0eee555555555555e"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPublicSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.0.5"),
			PrivateDnsName:     aws.String("ip-10-0-0-5.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:05"),
			Description:        aws.String("Primary network interface for bastion-prod"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(false),
			SourceDestCheck:    aws.Bool(true),
			Attachment: &ec2types.NetworkInterfaceAttachment{
				AttachmentId: aws.String("eni-attach-05"), InstanceId: aws.String("i-0a1b2c3d4e5f60005"),
				DeviceIndex: aws.Int32(0), Status: ec2types.AttachmentStatusAttached, DeleteOnTermination: aws.Bool(true),
			},
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String("sg-0aaa111111111111a"), GroupName: aws.String("acme-web-alb-sg")},
			},
			Association: &ec2types.NetworkInterfaceAssociation{
				PublicIp: aws.String("52.87.221.44"), IpOwnerId: aws.String("amazon"), AllocationId: aws.String("eipalloc-0ddd444444444444d"),
			},
			TagSet: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("bastion-prod-primary")},
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
		// EFS mount-target ENIs — Description MUST contain ProdEFSID so efs→sg/subnet/vpc/eni
		// checkers resolve via strings.Contains. No Attachment: mount-target ENIs are not EC2 instances.
		{
			NetworkInterfaceId: aws.String(ProdEFSEniAID),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(ProdEFSVpcID),
			SubnetId:           aws.String(ProdEFSSubnetAID),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.20.1.10"),
			PrivateDnsName:     aws.String("ip-10-20-1-10.ec2.internal"),
			MacAddress:         aws.String("0a:ef:00:00:00:0a"),
			Description:        aws.String("EFS mount target for " + ProdEFSID),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(true),
			SourceDestCheck:    aws.Bool(false),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(ProdEFSSecurityGroupAID), GroupName: aws.String("acme-efs-prod-sg-a")},
				{GroupId: aws.String(ProdEFSSecurityGroupBID), GroupName: aws.String("acme-efs-prod-sg-b")},
			},
			TagSet: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("efs-mt-prod-1a")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			NetworkInterfaceId: aws.String(ProdEFSEniBID),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(ProdEFSVpcID),
			SubnetId:           aws.String(ProdEFSSubnetBID),
			AvailabilityZone:   aws.String("us-east-1b"),
			PrivateIpAddress:   aws.String("10.20.2.10"),
			PrivateDnsName:     aws.String("ip-10-20-2-10.ec2.internal"),
			MacAddress:         aws.String("0a:ef:00:00:00:0b"),
			Description:        aws.String("EFS mount target for " + ProdEFSID),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(true),
			SourceDestCheck:    aws.Bool(false),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(ProdEFSSecurityGroupAID), GroupName: aws.String("acme-efs-prod-sg-a")},
				{GroupId: aws.String(ProdEFSSecurityGroupBID), GroupName: aws.String("acme-efs-prod-sg-b")},
			},
			TagSet: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("efs-mt-prod-1b")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			NetworkInterfaceId: aws.String(ProdEFSEniCID),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(ProdEFSVpcID),
			SubnetId:           aws.String(ProdEFSSubnetCID),
			AvailabilityZone:   aws.String("us-east-1c"),
			PrivateIpAddress:   aws.String("10.20.3.10"),
			PrivateDnsName:     aws.String("ip-10-20-3-10.ec2.internal"),
			MacAddress:         aws.String("0a:ef:00:00:00:0c"),
			Description:        aws.String("EFS mount target for " + ProdEFSID),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(true),
			SourceDestCheck:    aws.Bool(false),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(ProdEFSSecurityGroupAID), GroupName: aws.String("acme-efs-prod-sg-a")},
				{GroupId: aws.String(ProdEFSSecurityGroupBID), GroupName: aws.String("acme-efs-prod-sg-b")},
			},
			TagSet: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("efs-mt-prod-1c")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// ELB-owned ENI — required for eni→elb related-panel pivot
		// (checkENIELB). Real ALB/NLB ENIs are RequesterManaged with
		// RequesterId "amazon-elb" and Description "ELB app/<name>/<hash>";
		// they have no EC2 Attachment. Matches acme-prod-web (elb.go).
		{
			NetworkInterfaceId: aws.String("eni-0elbowned00000001a"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPublicSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.1.60"),
			PrivateDnsName:     aws.String("ip-10-0-1-60.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:e1"),
			Description:        aws.String("ELB app/acme-prod-web/1234567890abcdef"),
			OwnerId:            aws.String("123456789012"),
			RequesterId:        aws.String("amazon-elb"),
			RequesterManaged:   aws.Bool(true),
			SourceDestCheck:    aws.Bool(false),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String("sg-0aaa111111111111a"), GroupName: aws.String("acme-web-alb-sg")},
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("acme-prod-web-eni-1a")}},
		},
		// NAT-backing ENI — required for eni→nat related-panel pivot
		// (checkENINAT). Matches nat-0bbb222222222222b's
		// NatGatewayAddresses[].NetworkInterfaceId (this file, buildNatGateways).
		{
			NetworkInterfaceId: aws.String("eni-0nat0000000000002b"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeNatGateway,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPublicSubnetB),
			AvailabilityZone:   aws.String("us-east-1b"),
			PrivateIpAddress:   aws.String("10.0.2.50"),
			PrivateDnsName:     aws.String("ip-10-0-2-50.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:n2"),
			Description:        aws.String("Interface for NAT Gateway nat-0bbb222222222222b"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(true),
			SourceDestCheck:    aws.Bool(false),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("prod-nat-eni-1b-2")}},
		},
		// VPC-endpoint-owned ENI — required for eni→vpce related-panel pivot
		// (checkENIVPCE) and the vpce→eni reverse pivot. prod-s3-endpoint
		// (vpce-0aaa111111111111a, this file) references this ENI ID in its
		// NetworkInterfaceIds.
		{
			NetworkInterfaceId: aws.String("eni-0ccc333333333333c"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeVpcEndpoint,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPrivateSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.3.150"),
			PrivateDnsName:     aws.String("ip-10-0-3-150.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:v1"),
			Description:        aws.String("VPC Endpoint Interface for S3"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(true),
			SourceDestCheck:    aws.Bool(true),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(fixtProdWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("vpce-s3-eni-1a")}},
		},
		// Lambda hyperplane ENI — required for lambda→eni related-panel pivot.
		// checkLambdaENI matches ENIs whose Description contains the function name.
		{
			NetworkInterfaceId: aws.String("eni-0lambda000000001a"),
			Status:             ec2types.NetworkInterfaceStatusInUse,
			InterfaceType:      ec2types.NetworkInterfaceTypeLambda,
			VpcId:              aws.String(lambdaProdVPCID),
			SubnetId:           aws.String(lambdaProdSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.1.200"),
			PrivateDnsName:     aws.String("ip-10-0-1-200.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:99"),
			Description:        aws.String("AWS Lambda VPC ENI-api-gateway-authorizer-a1b2c3d4-5678-90ab-cdef-111111111111"),
			OwnerId:            aws.String("123456789012"),
			// RequesterId must be the exact real-AWS value "AWS Lambda VPC
			// ENI" — checkLambdaENI matches it exactly (docs/resources/lambda.md).
			RequesterId:      aws.String("AWS Lambda VPC ENI"),
			RequesterManaged: aws.Bool(true),
			SourceDestCheck:  aws.Bool(true),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(lambdaProdALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			},
			TagSet: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("lambda-api-gateway-authorizer-eni")},
			},
		},
		// Status=attaching → wave1 finding (CodeENIStateAttaching, SevWarn) → Warning.
		{
			NetworkInterfaceId: aws.String("eni-0attaching0000001a"),
			Status:             ec2types.NetworkInterfaceStatusAttaching,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPrivateSubnetA),
			AvailabilityZone:   aws.String("us-east-1a"),
			PrivateIpAddress:   aws.String("10.0.3.201"),
			PrivateDnsName:     aws.String("ip-10-0-3-201.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:a1"),
			Description:        aws.String("Secondary interface for worker-batch-03, attach in progress"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(false),
			SourceDestCheck:    aws.Bool(true),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(fixtProdRDSSGID), GroupName: aws.String("acme-worker-sg")},
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("worker-batch-03-eni-attaching")}},
		},
		// Status=detaching → wave1 finding (CodeENIStateDetaching, SevWarn) → Warning.
		{
			NetworkInterfaceId: aws.String("eni-0detaching0000001b"),
			Status:             ec2types.NetworkInterfaceStatusDetaching,
			InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
			VpcId:              aws.String(fixtProdVPCID),
			SubnetId:           aws.String(fixtProdPrivateSubnetB),
			AvailabilityZone:   aws.String("us-east-1b"),
			PrivateIpAddress:   aws.String("10.0.4.202"),
			PrivateDnsName:     aws.String("ip-10-0-4-202.ec2.internal"),
			MacAddress:         aws.String("0a:1b:2c:3d:4e:a2"),
			Description:        aws.String("Secondary interface for db-proxy-01, detach in progress"),
			OwnerId:            aws.String("123456789012"),
			RequesterManaged:   aws.Bool(false),
			SourceDestCheck:    aws.Bool(true),
			Groups: []ec2types.GroupIdentifier{
				{GroupId: aws.String(fixtProdDBProxySGID), GroupName: aws.String("acme-db-proxy-sg")},
			},
			TagSet: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("db-proxy-01-eni-detaching")}},
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
			// aws:cloudformation:stack-name tag — required for ebs→cfn related-panel
			// pivot. acme-eks-cluster is a real stack fixture (cfn.go).
			// backup=daily tag — required for the ebs:backup related-panel
			// pivot witness. Matches the ListOfTags condition on
			// HealthyDailyPlanID's selection (backup.go).
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("web-prod-01-root")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
				{Key: aws.String("backup"), Value: aws.String("daily")},
			},
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
		// Witness for ebs.state.deleting. Encrypted and attached so the state
		// finding is the only one it carries.
		{
			VolumeId: aws.String("vol-0deleting0000000d4"), State: ec2types.VolumeStateDeleting,
			Size: aws.Int32(50), VolumeType: ec2types.VolumeTypeGp3, Iops: aws.Int32(3000),
			Encrypted: aws.Bool(true), AvailabilityZone: aws.String("us-east-1a"), CreateTime: aws.Time(t5),
			Attachments: []ec2types.VolumeAttachment{{InstanceId: aws.String("i-0a1b2c3d4e5f60004")}},
			Tags:        []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("retired-cache-volume")}},
		},
		// Orphan: Available, no attachments, well over 7 days old (fixed past
		// date, not relative to time.Now()) → ebs.orphan-unattached finding.
		{
			VolumeId: aws.String("vol-0orphan00000000a1"), State: ec2types.VolumeStateAvailable,
			Size: aws.Int32(80), VolumeType: ec2types.VolumeTypeGp2, Iops: aws.Int32(240),
			Encrypted: aws.Bool(true), AvailabilityZone: aws.String("us-east-1a"),
			CreateTime:  aws.Time(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)),
			Attachments: nil,
			Tags:        []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("old-orphan-vol")}},
		},
		// Error state → Broken
		{
			VolumeId: aws.String("vol-0error00000000b2"), State: ec2types.VolumeStateError,
			Size: aws.Int32(200), VolumeType: ec2types.VolumeTypeGp3, Iops: aws.Int32(3000),
			Encrypted: aws.Bool(true), AvailabilityZone: aws.String("us-east-1c"),
			CreateTime:  aws.Time(time.Now().AddDate(0, 0, -10)),
			Attachments: nil,
			Tags:        []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("failed-restore-vol")}},
		},
		// Unencrypted InUse (fixed past date, not relative to time.Now()) →
		// CIS EC2.7 violation, ebs.encryption.disabled finding.
		{
			VolumeId: aws.String("vol-0unenc00000000c3"), State: ec2types.VolumeStateInUse,
			Size: aws.Int32(50), VolumeType: ec2types.VolumeTypeGp2, Iops: aws.Int32(150),
			Encrypted: aws.Bool(false), AvailabilityZone: aws.String("us-east-1a"),
			CreateTime:  aws.Time(time.Date(2025, 9, 15, 8, 0, 0, 0, time.UTC)),
			Attachments: []ec2types.VolumeAttachment{{InstanceId: aws.String("i-0a1b2c3d4e5f60002")}},
			Tags:        []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("legacy-unencrypted-vol")}},
		},
	}
}

// buildVolumeStatuses backs EC2:DescribeVolumeStatus for the Wave 2
// ebs.volume-io-degraded enrichment (EnrichEBSVolumeStatus). Only
// vol-0a1b2c3d4e5f60002 (api-staging-data, in-use) carries a non-ok status —
// every other volume above is intentionally left off this list so the
// enrichment's "skip on ok/absent" path also has fixture coverage.
func buildVolumeStatuses() []ec2types.VolumeStatusItem {
	return []ec2types.VolumeStatusItem{
		{
			VolumeId: aws.String("vol-0a1b2c3d4e5f60002"),
			VolumeStatus: &ec2types.VolumeStatusInfo{
				Status: ec2types.VolumeStatusInfoStatusImpaired,
			},
			Events: []ec2types.VolumeStatusEvent{
				{
					EventType:   aws.String("io-performance"),
					Description: aws.String("Degraded IOPS on volume vol-0a1b2c3d4e5f60002"),
				},
			},
			Actions: []ec2types.VolumeStatusAction{
				{Code: aws.String("enable-volume-io")},
			},
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
			Encrypted: aws.Bool(true),
			// Description — required for the ebs-snap:ec2 related-panel pivot
			// (checkEBSSnapEC2 parses "Created by CreateImage(i-xxx)"), matching
			// real AWS behavior where CreateImage auto-generates this snapshot
			// description. This snapshot is also referenced by an AMI's block
			// device mapping (ebs-snap:ami pivot) — consistent with the AMI
			// having been created from this same instance.
			Description: aws.String("Created by CreateImage(i-0a1b2c3d4e5f60001) for ami-0a1b2c3d4e5f60001 from vol-0a1b2c3d4e5f60001"),
			StartTime:   aws.Time(t1), Progress: aws.String("100%"), OwnerId: aws.String("123456789012"),
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
		// AWS Backup-created snapshot — required for the ebs-snap:backup
		// related-panel pivot witness. Description prefix + the
		// aws:backup:source-resource tag are the real AWS Backup signature;
		// the tag value matches the volume ARN in HealthyDailyPlanID's
		// selection (backup.go) so checkEBSSnapBackup resolves a specific plan.
		{
			SnapshotId: aws.String("snap-awsbackup000001"), State: ec2types.SnapshotStateCompleted,
			VolumeId: aws.String("vol-0a1b2c3d4e5f60001"), VolumeSize: aws.Int32(50),
			Encrypted: aws.Bool(true), Description: aws.String("Created by AWS Backup for BackupPlan: acme-daily-backup"),
			StartTime: aws.Time(time.Date(2026, 4, 16, 3, 0, 0, 0, time.UTC)),
			Progress:  aws.String("100%"), OwnerId: aws.String("123456789012"),
			KmsKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("web-prod-01-root-awsbackup")},
				{Key: aws.String("aws:backup:source-resource"), Value: aws.String("arn:aws:ec2:us-east-1:123456789012:volume/vol-0a1b2c3d4e5f60001")},
			},
		},
		// Old automated snapshot (>365d) → aged-automated cost finding.
		// Fixed 2025-01-01 anchor keeps the age deterministically past the
		// 365-day threshold regardless of when the test suite runs.
		{
			SnapshotId: aws.String("snap-completed-old00a"), State: ec2types.SnapshotStateCompleted,
			VolumeId: aws.String("vol-0a1b2c3d4e5f60001"), VolumeSize: aws.Int32(50),
			Encrypted: aws.Bool(true), Description: aws.String("Automated snapshot created by data lifecycle manager"),
			StartTime: aws.Time(time.Date(2025, 1, 1, 3, 0, 0, 0, time.UTC)),
			Progress:  aws.String("100%"), OwnerId: aws.String("123456789012"),
			KmsKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
			Tags:     []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("dlm-auto-old-snap")}},
		},
		// Error state + unencrypted → CIS EC2.1 violation
		{
			SnapshotId: aws.String("snap-error000000000b"), State: ec2types.SnapshotStateError,
			VolumeId: aws.String("vol-0a1b2c3d4e5f60003"), VolumeSize: aws.Int32(100),
			Encrypted: aws.Bool(false), Description: aws.String("Failed backup — disk I/O error during snapshot"),
			StartTime: aws.Time(time.Date(2026, 3, 25, 4, 0, 0, 0, time.UTC)),
			Progress:  aws.String("0%"), OwnerId: aws.String("123456789012"),
			Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("failed-backup-snap")}},
		},
		// Orphan snap: references a deleted volume (vol-deleted-original does
		// not appear in buildVolumes) — required witness for the ebs-snap
		// cross-ref orphan Finding (enrichEBSSnapCrossRef).
		{
			SnapshotId: aws.String("snap-orphan00000000c"), State: ec2types.SnapshotStateCompleted,
			VolumeId: aws.String("vol-deleted-original"), VolumeSize: aws.Int32(200),
			Encrypted: aws.Bool(true), Description: aws.String("Snapshot of deleted volume — orphaned"),
			StartTime: aws.Time(time.Date(2026, 1, 28, 5, 0, 0, 0, time.UTC)),
			Progress:  aws.String("100%"), OwnerId: aws.String("123456789012"),
			KmsKeyId: aws.String("b2c3d4e5-6789-01ab-cdef-222222222222"),
			Tags:     []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("orphan-snap")}},
		},
		// Completed + unencrypted → CIS EC2.1 violation witness (distinct
		// from snap-error000000000b, which is Error-state and never reaches
		// the unencrypted structural check since Error already carries its
		// own state Finding).
		{
			SnapshotId: aws.String("snap-unencrypted00d"), State: ec2types.SnapshotStateCompleted,
			VolumeId: aws.String("vol-0a1b2c3d4e5f60004"), VolumeSize: aws.Int32(80),
			Encrypted: aws.Bool(false), Description: aws.String("Manual snapshot before decommission"),
			StartTime: aws.Time(time.Date(2026, 5, 10, 6, 0, 0, 0, time.UTC)),
			Progress:  aws.String("100%"), OwnerId: aws.String("123456789012"),
			Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("legacy-worker-unencrypted-snap")}},
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
			// SnapshotId/KmsKeyId — required for ami→ebs-snap and ami→kms related-panel
			// pivots. snap-0a1b2c3d4e5f60001 is a real snapshot fixture (ec2.go buildSnapshots).
			BlockDeviceMappings: []ec2types.BlockDeviceMapping{
				{DeviceName: aws.String("/dev/xvda"), Ebs: &ec2types.EbsBlockDevice{
					VolumeSize: aws.Int32(20), VolumeType: ec2types.VolumeTypeGp3, DeleteOnTermination: aws.Bool(true),
					SnapshotId: aws.String("snap-0a1b2c3d4e5f60001"),
					KmsKeyId:   aws.String(AMIEBSKmsKeyARN),
				}},
			},
			BootMode: ec2types.BootModeValuesUefi, DeprecationTime: aws.String("2028-01-01T00:00:00Z"),
			ImageLocation: aws.String("123456789012/amazon-linux-2023-x86_64"), ImageOwnerAlias: aws.String("amazon"),
			SriovNetSupport: aws.String("simple"), UsageOperation: aws.String("RunInstances"),
			// aws:cloudformation:stack-name tag — required for ami→cfn related-panel
			// pivot. acme-eks-cluster is a real stack fixture (cfn.go).
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-app-server-x86-v2.3.1")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
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
		// Deprecated (DeprecationTime in the past) → Warning
		{
			ImageId: aws.String("ami-0deprecated0ubuntu1"), Name: aws.String("acme-ubuntu-20-04-deprecated"),
			State: ec2types.ImageStateAvailable, Architecture: ec2types.ArchitectureValuesX8664,
			PlatformDetails: aws.String("Linux/UNIX"), RootDeviceType: ec2types.DeviceTypeEbs,
			RootDeviceName: aws.String("/dev/sda1"), Hypervisor: ec2types.HypervisorTypeXen,
			VirtualizationType: ec2types.VirtualizationTypeHvm, ImageType: ec2types.ImageTypeValuesMachine,
			CreationDate:    aws.String("2023-01-15T09:00:00.000Z"),
			DeprecationTime: aws.String("2026-01-15T09:00:00.000Z"),
			Public:          aws.Bool(false),
			OwnerId:         aws.String("123456789012"),
			Description:     aws.String("Ubuntu 20.04 LTS — deprecated in favour of 22.04"),
			EnaSupport:      aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-ubuntu-20-04-deprecated")},
			},
		},
		// Failed build → Broken
		{
			ImageId: aws.String("ami-0failed0build00002"), Name: aws.String("acme-app-build-failed"),
			State: ec2types.ImageStateFailed, Architecture: ec2types.ArchitectureValuesX8664,
			PlatformDetails: aws.String("Linux/UNIX"), RootDeviceType: ec2types.DeviceTypeEbs,
			RootDeviceName: aws.String("/dev/xvda"), Hypervisor: ec2types.HypervisorTypeXen,
			VirtualizationType: ec2types.VirtualizationTypeHvm, ImageType: ec2types.ImageTypeValuesMachine,
			CreationDate: aws.String("2026-04-10T07:30:00.000Z"), Public: aws.Bool(false),
			OwnerId: aws.String("123456789012"), Description: aws.String("CI build failed — root device snapshot error"),
			EnaSupport: aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-app-build-failed")},
				{Key: aws.String("Environment"), Value: aws.String("ci")},
			},
		},
		// State=pending → Warning (colorAMI's state-switch branch — not the
		// DeprecationTime path, which is unreachable: the fetcher writes a
		// pre-formatted "deprecated" field but colorAMI reads the raw
		// "deprecation_time" field that FetchAMIsPage never sets).
		{
			ImageId: aws.String("ami-0pending0build0003"), Name: aws.String("acme-app-build-inprogress"),
			State: ec2types.ImageStatePending, Architecture: ec2types.ArchitectureValuesX8664,
			PlatformDetails: aws.String("Linux/UNIX"), RootDeviceType: ec2types.DeviceTypeEbs,
			RootDeviceName: aws.String("/dev/xvda"), Hypervisor: ec2types.HypervisorTypeXen,
			VirtualizationType: ec2types.VirtualizationTypeHvm, ImageType: ec2types.ImageTypeValuesMachine,
			CreationDate: aws.String("2026-04-28T07:30:00.000Z"), Public: aws.Bool(false),
			OwnerId: aws.String("123456789012"), Description: aws.String("CI build in progress — image registration pending"),
			EnaSupport: aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-app-build-inprogress")},
				{Key: aws.String("Environment"), Value: aws.String("ci")},
			},
		},
		// ami-0eks111111111111a — pinned by the EC2 fake's
		// DescribeLaunchTemplateVersions(lt-0eks111111111111a) response (see
		// core/demo/fakes/ec2.go), which the EKS general-pool nodegroup's
		// LaunchTemplate resolves to via resolveNGImageID. Required so the AMI
		// this nodegroup actually launches from exists as a real fixture,
		// closing the ami→ng and eks→ami related-panel pivots.
		{
			ImageId: aws.String("ami-0eks111111111111a"), Name: aws.String("acme-eks-worker-al2-1.29"),
			State: ec2types.ImageStateAvailable, Architecture: ec2types.ArchitectureValuesX8664,
			PlatformDetails: aws.String("Linux/UNIX"), RootDeviceType: ec2types.DeviceTypeEbs,
			RootDeviceName: aws.String("/dev/xvda"), Hypervisor: ec2types.HypervisorTypeXen,
			VirtualizationType: ec2types.VirtualizationTypeHvm, ImageType: ec2types.ImageTypeValuesMachine,
			CreationDate: aws.String("2026-01-10T08:00:00.000Z"), Public: aws.Bool(false),
			OwnerId: aws.String("602401143452"), Description: aws.String("EKS Kubernetes Worker AMI (amazon-eks-node-1.29)"),
			EnaSupport: aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-eks-worker-al2-1.29")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// Public=true → the ami.public witness: an account-owned image whose
		// launch permission was left open to every AWS account.
		{
			ImageId: aws.String(AMIPublic), Name: aws.String("acme-demo-appliance-public"),
			State: ec2types.ImageStateAvailable, Architecture: ec2types.ArchitectureValuesX8664,
			PlatformDetails: aws.String("Linux/UNIX"), RootDeviceType: ec2types.DeviceTypeEbs,
			RootDeviceName: aws.String("/dev/xvda"), Hypervisor: ec2types.HypervisorTypeXen,
			VirtualizationType: ec2types.VirtualizationTypeHvm, ImageType: ec2types.ImageTypeValuesMachine,
			CreationDate: aws.String("2026-03-02T09:15:00.000Z"), Public: aws.Bool(true),
			OwnerId: aws.String("123456789012"), Description: aws.String("Shared appliance image — launch permission left open to all accounts"),
			EnaSupport: aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-demo-appliance-public")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
	}
}

func init() {
	Register(Pin{ShortName: "ec2", Rows: 41, Issues: 16})
	Register(Pin{ShortName: "ebs", Rows: 9, Issues: 6, CoverageGaps: []string{"dim"}})
	Register(Pin{ShortName: "ebs-snap", Rows: 9, Issues: 5, CoverageGaps: []string{"dim"}})
	Register(Pin{ShortName: "ami", Rows: 9, Issues: 4})
	Register(Pin{ShortName: "eip", Rows: 9, Issues: 4, CoverageGaps: []string{"broken", "dim"}})
	Register(Pin{ShortName: "eni", Rows: 48, Issues: 3, CoverageGaps: []string{"broken", "dim"}})
	Register(Pin{ShortName: "igw", Rows: 5, Issues: 3, CoverageGaps: []string{"broken", "dim"}})
	Register(Pin{ShortName: "nat", Rows: 6, Issues: 3})
	Register(Pin{ShortName: "rtb", Rows: 6, Issues: 3, CoverageGaps: []string{"dim"}})
	Register(Pin{ShortName: "sg", Rows: 42, Issues: 6, CoverageGaps: []string{"dim"}})
	Register(Pin{ShortName: "subnet", Rows: 38, Issues: 5, CoverageGaps: []string{"dim"}})
	Register(Pin{ShortName: "tgw", Rows: 8, Issues: 5})
	Register(Pin{ShortName: "vpc", Rows: 8, Issues: 1, CoverageGaps: []string{"broken", "dim"}})
	Register(Pin{ShortName: "vpce", Rows: 12, Issues: 8})
}
