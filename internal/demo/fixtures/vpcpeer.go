// Package fixtures provides VPC Peering Connection fixture data for the EC2
// fake. VPC peering rides the EC2 service client (DescribeVpcPeeringConnections)
// — no dedicated client field, see internal/demo/client.go and
// internal/demo/fakes/ec2.go.
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

func buildVpcPeerConnections() []ec2types.VpcPeeringConnection {
	return []ec2types.VpcPeeringConnection{
		{
			VpcPeeringConnectionId: aws.String(ProdPeerSharedID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodeActive, ""),
			RequesterVpcInfo: func() *ec2types.VpcPeeringConnectionVpcInfo {
				info := vpcPeerInfo(fixtProdVPCID, "123456789012", "10.0.0.0/16")
				info.PeeringOptions = &ec2types.VpcPeeringConnectionOptionsDescription{AllowDnsResolutionFromRemoteVpc: aws.Bool(true)}
				return info
			}(),
			AccepterVpcInfo: func() *ec2types.VpcPeeringConnectionVpcInfo {
				info := vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID, "192.168.0.0/16")
				info.PeeringOptions = &ec2types.VpcPeeringConnectionOptionsDescription{AllowDnsResolutionFromRemoteVpc: aws.Bool(false)}
				return info
			}(),
		},
		{
			VpcPeeringConnectionId: aws.String(WarnPeerPendingID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance, ""),
			ExpirationTime:         aws.Time(time.Now().UTC().Add(72 * time.Hour)),
			RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012"),
			AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID),
		},
		{
			VpcPeeringConnectionId: aws.String(WarnPeerExpiredID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodeExpired, ""),
			RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012"),
			AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID),
		},
		{
			VpcPeeringConnectionId: aws.String(BrokenPeerRejectedID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodeRejected, "Rejected by accepter: CIDR conflict"),
			RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012"),
			AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID),
		},
		{
			VpcPeeringConnectionId: aws.String(BrokenPeerFailedID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodeFailed, "Failed to activate the peering connection due to an internal error"),
			RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012"),
			AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID),
		},
		{
			VpcPeeringConnectionId: aws.String(WarnPeerDeletingID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodeDeleting, ""),
			RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012"),
			AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID),
		},
		{
			VpcPeeringConnectionId: aws.String(DimPeerDeletedID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodeDeleted, ""),
			RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012"),
			AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID),
		},
		{
			VpcPeeringConnectionId: aws.String(WarnPeerOverlapID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodeActive, ""),
			RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012", "10.0.0.0/16"),
			AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID, "10.0.0.0/16"),
		},
		{
			VpcPeeringConnectionId: aws.String(WarnPeerNoRouteID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodeActive, ""),
			RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012", "10.0.0.0/16"),
			AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID, "172.16.0.0/16"),
		},
		{
			VpcPeeringConnectionId: aws.String(WarnPeerBlackholeID),
			Status:                 vpcPeerStatus(ec2types.VpcPeeringConnectionStateReasonCodeActive, ""),
			RequesterVpcInfo:       vpcPeerInfo(fixtProdVPCID, "123456789012", "10.0.0.0/16"),
			AccepterVpcInfo:        vpcPeerInfo(vpcPeerRemoteVpcID, vpcPeerRemoteOwnerID, "10.30.0.0/16"),
		},
	}
}
