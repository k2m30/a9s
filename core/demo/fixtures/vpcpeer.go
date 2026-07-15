// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides VPC Peering Connection fixture data for the EC2
// fake. VPC peering rides the EC2 service client (DescribeVpcPeeringConnections)
// — no dedicated client field, see core/demo/client.go and
// core/demo/fakes/ec2.go.
package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// VpcPeerFixtures holds all VPC Peering Connection domain objects served by
// the fake. DescribeVpcPeeringConnections returns full detail (Status,
// ExpirationTime, both VpcInfo sides) in the single list call — no describe,
// no denied-IDs path exists for this type.
type VpcPeerFixtures struct {
	Connections []ec2types.VpcPeeringConnection
}

// Exported stable IDs — sibling fixture files (ec2.go's RouteTables) and the
// EC2 fake reference peering connections by these symbols, not by string
// literal.
const (
	// ProdPeerSharedID is the graph root: active connection between the local
	// prod VPC (a vpc cache member — resolves the vpc related-panel pivot)
	// and a cross-account remote VPC that is NOT in the local vpc cache — the
	// cache-membership-gate witness (docs/resources/vpc-peer.md §2 vpc).
	// Two existing rtb fixtures (ec2.go) gain routes to it, backing the rtb
	// pivot's ≥2 witness.
	ProdPeerSharedID = "pcx-0prodpeershared1a"
	// WarnPeerProvisioningID is being provisioned.
	WarnPeerProvisioningID = "pcx-0warnprovision1a"
	// WarnPeerInitiatingID is initiating the peering request.
	WarnPeerInitiatingID = "pcx-0warninitiating1a"
	// WarnPeerPendingID has not been accepted; ExpirationTime is evergreen
	// (now+3d) so the "pending acceptance: expires in 3d" phrase never rots.
	WarnPeerPendingID = "pcx-0warnpending1111a"
	// WarnPeerExpiredID's request died unaccepted.
	WarnPeerExpiredID = "pcx-0warnexpired1111a"
	// BrokenPeerRejectedID carries the peer's rejection reason verbatim in
	// Status.Message.
	BrokenPeerRejectedID = "pcx-0brokenrejected1a"
	// BrokenPeerFailedID carries the failure reason verbatim in Status.Message.
	BrokenPeerFailedID = "pcx-0brokenfailed111a"
	// WarnPeerDeletingID is mid-teardown.
	WarnPeerDeletingID = "pcx-0warndeleting111a"
	// DimPeerDeletedID is fully torn down; AWS keeps it listed for a window.
	DimPeerDeletedID = "pcx-0dimdeleted11111a"
	// WarnPeerOverlapID is active with identical CidrBlockSets on both sides
	// — the CIDR-overlap Wave 1 signal witness.
	WarnPeerOverlapID = "pcx-0warnoverlap1111a"
	// WarnPeerNoRouteID is active with valid disjoint CIDRs but deliberately
	// has NO loaded rtb route referencing it — the "no local route to peer"
	// cache-scan witness.
	WarnPeerNoRouteID = "pcx-0warnnoroute1111a"
	// WarnPeerBlackholeID is active, but the one rtb route referencing it
	// (ec2.go) has State blackhole — the "route to peer blackholed" witness.
	WarnPeerBlackholeID = "pcx-0warnblackhole1a"
)

// vpcPeerRemoteOwnerID / vpcPeerRemoteVpcID model the cross-account far side
// of every peering connection: a real AWS account distinct from this demo's
// own 123456789012, whose VPC is never in the locally-loaded vpc cache.
const (
	vpcPeerRemoteOwnerID = "210987654321"
	vpcPeerRemoteVpcID   = "vpc-0remoteacct1111a"
)

var sharedVpcPeerFixtures = sync.OnceValue(func() *VpcPeerFixtures {
	return &VpcPeerFixtures{Connections: buildVpcPeerConnections()}
})

func NewVpcPeerFixtures() *VpcPeerFixtures {
	return sharedVpcPeerFixtures()
}

// vpcPeerInfo builds one side of a peering connection. CidrBlock/CidrBlockSet
// are only populated when cidrs is non-empty — the SDK returns CIDR info
// only for active connections (VpcPeeringConnection doc comment), so callers
// must omit cidrs entirely for every non-active state.
func vpcPeerInfo(vpcID, ownerID string, cidrs ...string) *ec2types.VpcPeeringConnectionVpcInfo {
	info := &ec2types.VpcPeeringConnectionVpcInfo{
		VpcId:   aws.String(vpcID),
		OwnerId: aws.String(ownerID),
	}
	if len(cidrs) == 0 {
		return info
	}
	info.CidrBlock = aws.String(cidrs[0])
	blocks := make([]ec2types.CidrBlock, len(cidrs))
	for i, c := range cidrs {
		blocks[i] = ec2types.CidrBlock{CidrBlock: aws.String(c)}
	}
	info.CidrBlockSet = blocks
	return info
}

func vpcPeerStatus(code ec2types.VpcPeeringConnectionStateReasonCode, message string) *ec2types.VpcPeeringConnectionStateReason {
	status := &ec2types.VpcPeeringConnectionStateReason{Code: code}
	if message != "" {
		status.Message = aws.String(message)
	}
	return status
}

// peer builds one VpcPeeringConnection: requester is always the local prod
// VPC (account 123456789012), accepter is always the cross-account remote
// VPC. reqCidrs/accCidrs are omitted (nil) for every non-active state — the
// SDK only returns CIDR info for active connections.
func peer(id string, code ec2types.VpcPeeringConnectionStateReasonCode, msg string, reqCidrs, accCidrs []string) ec2types.VpcPeeringConnection {
	return ec2types.VpcPeeringConnection{
		VpcPeeringConnectionId: aws.String(id),
		Status:                 vpcPeerStatus(code, msg),
		RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012", reqCidrs...),
		AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID, accCidrs...),
	}
}

func buildVpcPeerConnections() []ec2types.VpcPeeringConnection {
	root := peer(ProdPeerSharedID, ec2types.VpcPeeringConnectionStateReasonCodeActive, "",
		[]string{"10.0.0.0/16"}, []string{"192.168.0.0/16"})
	root.RequesterVpcInfo.PeeringOptions = &ec2types.VpcPeeringConnectionOptionsDescription{AllowDnsResolutionFromRemoteVpc: aws.Bool(true)}
	root.AccepterVpcInfo.PeeringOptions = &ec2types.VpcPeeringConnectionOptionsDescription{AllowDnsResolutionFromRemoteVpc: aws.Bool(false)}

	pending := peer(WarnPeerPendingID, ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance, "", nil, nil)
	pending.ExpirationTime = aws.Time(time.Now().UTC().Add(72 * time.Hour))

	return []ec2types.VpcPeeringConnection{
		root,
		peer(WarnPeerProvisioningID, ec2types.VpcPeeringConnectionStateReasonCodeProvisioning, "", nil, nil),
		peer(WarnPeerInitiatingID, ec2types.VpcPeeringConnectionStateReasonCodeInitiatingRequest, "", nil, nil),
		pending,
		peer(WarnPeerExpiredID, ec2types.VpcPeeringConnectionStateReasonCodeExpired, "", nil, nil),
		peer(BrokenPeerRejectedID, ec2types.VpcPeeringConnectionStateReasonCodeRejected, "Rejected by accepter: CIDR conflict", nil, nil),
		peer(BrokenPeerFailedID, ec2types.VpcPeeringConnectionStateReasonCodeFailed, "Failed to activate the peering connection due to an internal error", nil, nil),
		peer(WarnPeerDeletingID, ec2types.VpcPeeringConnectionStateReasonCodeDeleting, "", nil, nil),
		peer(DimPeerDeletedID, ec2types.VpcPeeringConnectionStateReasonCodeDeleted, "", nil, nil),
		peer(WarnPeerOverlapID, ec2types.VpcPeeringConnectionStateReasonCodeActive, "", []string{"10.0.0.0/16"}, []string{"10.0.0.0/16"}),
		peer(WarnPeerNoRouteID, ec2types.VpcPeeringConnectionStateReasonCodeActive, "", []string{"10.0.0.0/16"}, []string{"172.16.0.0/16"}),
		peer(WarnPeerBlackholeID, ec2types.VpcPeeringConnectionStateReasonCodeActive, "", []string{"10.0.0.0/16"}, []string{"10.30.0.0/16"}),
	}
}
