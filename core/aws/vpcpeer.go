// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — vpcpeer.go: EC2 VPC Peering Connection fetcher.
//
// DescribeVpcPeeringConnections is the only call — it returns full detail
// (Status, ExpirationTime, both VpcInfo sides) in the single paginated list
// response. AWS has no per-connection describe, so vpc-peer has no N+1
// fan-out and no degraded row: a DescribeVpcPeeringConnections denial is a
// whole-list error (the menu shows the error state), never a partial
// success.
//
// RawStruct is *ec2types.VpcPeeringConnection — a pointer to the loop copy,
// so every *_related.go checker's assertStruct[ec2types.VpcPeeringConnection]
// reads the same shape regardless of value/pointer storage.
package aws

import (
	"context"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// vpc-peer.* FindingCodes — docs/resources/vpc-peer.md (Source
// "wave1", fetcher-written). The two Wave 2 cache-scan codes live in
// vpcpeer_issue_enrichment.go, next to the enricher that emits them.
const (
	vpcPeerCodeProvisioning      domain.FindingCode = "vpc-peer.warn.provisioning"
	vpcPeerCodeInitiating        domain.FindingCode = "vpc-peer.warn.initiating"
	vpcPeerCodePendingAcceptance domain.FindingCode = "vpc-peer.warn.pending_acceptance"
	vpcPeerCodeExpired           domain.FindingCode = "vpc-peer.warn.expired"
	vpcPeerCodeRejected          domain.FindingCode = "vpc-peer.broken.rejected"
	vpcPeerCodeFailed            domain.FindingCode = "vpc-peer.broken.failed"
	vpcPeerCodeDeleting          domain.FindingCode = "vpc-peer.warn.deleting"
	vpcPeerCodeDeleted           domain.FindingCode = "vpc-peer.dim.deleted"
	vpcPeerCodeCidrOverlap       domain.FindingCode = "vpc-peer.warn.cidr_overlap"
	vpcPeerCodeUnrecognised      domain.FindingCode = "vpc-peer.warn.unrecognised_status"
)

// FetchVpcPeeringConnectionsPage fetches a single page of VPC Peering
// Connections. DescribeVpcPeeringConnections carries full detail (Status,
// ExpirationTime, both VpcInfo sides) in the list call.
func FetchVpcPeeringConnectionsPage(ctx context.Context, api EC2DescribeVpcPeeringConnectionsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeVpcPeeringConnectionsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	output, err := api.DescribeVpcPeeringConnections(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing VPC peering connections: %w", err)
	}

	resources := make([]resource.Resource, 0, len(output.VpcPeeringConnections))
	for i := range output.VpcPeeringConnections {
		resources = append(resources, buildVpcPeerResource(&output.VpcPeeringConnections[i]))
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: output.NextToken != nil,
			NextToken:   aws.ToString(output.NextToken),
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, nil
}

// buildVpcPeerResource constructs a Resource from one VpcPeeringConnection.
func buildVpcPeerResource(pc *ec2types.VpcPeeringConnection) resource.Resource {
	id := aws.ToString(pc.VpcPeeringConnectionId)
	name := tagValue(pc.Tags, "Name")
	findings, attention := computeVpcPeerFindings(pc)

	requesterVPC, requesterOwner := vpcPeerSide(pc.RequesterVpcInfo)
	accepterVPC, accepterOwner := vpcPeerSide(pc.AccepterVpcInfo)
	return resource.Resource{
		ID:   id,
		Name: name,
		Fields: map[string]string{
			"pcx_id":          id,
			"status":          vpcPeerStatus(pc),
			"requester_vpc":   requesterVPC,
			"requester_owner": requesterOwner,
			"accepter_vpc":    accepterVPC,
			"accepter_owner":  accepterOwner,
			"expires":         ltFormatTime(pc.ExpirationTime),
		},
		RawStruct:        pc,
		Findings:         findings,
		AttentionDetails: attention,
	}
}

// vpcPeerSide extracts one side's VpcId/OwnerId, "","" for a nil pointer.
func vpcPeerSide(info *ec2types.VpcPeeringConnectionVpcInfo) (vpcID, ownerID string) {
	if info == nil {
		return "", ""
	}
	return aws.ToString(info.VpcId), aws.ToString(info.OwnerId)
}

func vpcPeerStatus(pc *ec2types.VpcPeeringConnection) string {
	if pc.Status == nil {
		return ""
	}
	return string(pc.Status.Code)
}

// computeVpcPeerFindings builds the ordered Finding slice for one peering
// connection: state first, then CIDR overlap, which is declared for an
// active connection only because CidrBlock/CidrBlockSet are nil for every
// other (VpcPeeringConnectionVpcInfo doc comment). The pending-acceptance
// phrase counts down to ExpirationTime.
func computeVpcPeerFindings(pc *ec2types.VpcPeeringConnection) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	status := vpcPeerStatus(pc)
	message := ""
	if pc.Status != nil {
		message = aws.ToString(pc.Status.Message)
	}
	attention := map[domain.FindingCode]domain.AttentionDetail{}
	var slots []string
	_, code := catalog.LifecycleOf("vpc-peer", status)
	switch code {
	case vpcPeerCodePendingAcceptance:
		days, rows := vpcPeerPendingAcceptance(pc.ExpirationTime)
		slots = []string{strconv.Itoa(days)}
		if rows != nil {
			attention[code] = domain.AttentionDetail{Rows: rows}
		}
	case vpcPeerCodeRejected, vpcPeerCodeFailed:
		if message != "" {
			attention[code] = domain.AttentionDetail{Rows: []domain.DetailRow{{Label: "Status message", Value: message, Tier: "!"}}}
		}
	}

	var posture []domain.Finding
	if ranges, overlap := vpcPeerCIDROverlapDetail(pc.RequesterVpcInfo, pc.AccepterVpcInfo); overlap {
		posture = append(posture, wave1Finding(vpcPeerCodeCidrOverlap))
		attention[vpcPeerCodeCidrOverlap] = domain.AttentionDetail{Rows: []domain.DetailRow{{Label: "Overlapping range", Value: ranges, Tier: "~"}}}
	}
	findings := withState("vpc-peer", status, posture, slots...)
	if len(attention) == 0 {
		return findings, nil
	}
	return findings, attention
}

// vpcPeerPendingAcceptance is the countdown to ExpirationTime the
// pending-acceptance phrase carries, rounded up to at least 1 day so a
// request expiring within hours never reads as "0d". The expiration date
// itself is the Attention row.
func vpcPeerPendingAcceptance(expiration *time.Time) (int, []domain.DetailRow) {
	days := 1
	var rows []domain.DetailRow
	if expiration != nil {
		if remaining := time.Until(*expiration); remaining > 0 {
			if d := int(math.Ceil(remaining.Hours() / 24)); d > days {
				days = d
			}
		}
		rows = []domain.DetailRow{{Label: "Expires", Value: expiration.Format("2006-01-02"), Tier: "~"}}
	}
	return days, rows
}

// vpcPeerCIDROverlapDetail reports whether any IPv4 prefix in requester's
// CidrBlockSet overlaps accepter's, returning the overlapping range pair(s)
// for the Detail sentence. Callers MUST only invoke this for an active
// connection — CidrBlockSet is nil otherwise; nil-safe regardless.
func vpcPeerCIDROverlapDetail(requester, accepter *ec2types.VpcPeeringConnectionVpcInfo) (string, bool) {
	if requester == nil || accepter == nil {
		return "", false
	}
	var overlaps []string
	for _, rb := range requester.CidrBlockSet {
		rPrefix, err := netip.ParsePrefix(aws.ToString(rb.CidrBlock))
		if err != nil {
			continue
		}
		for _, ab := range accepter.CidrBlockSet {
			aPrefix, err := netip.ParsePrefix(aws.ToString(ab.CidrBlock))
			if err != nil {
				continue
			}
			if rPrefix.Overlaps(aPrefix) {
				overlaps = append(overlaps, fmt.Sprintf("%s vs %s", rPrefix, aPrefix))
			}
		}
	}
	if len(overlaps) == 0 {
		return "", false
	}
	return strings.Join(overlaps, "; "), true
}
