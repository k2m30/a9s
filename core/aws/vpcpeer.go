// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — vpcpeer.go: EC2 VPC Peering Connection fetcher.
//
// docs/resources/vpc-peer.md §1: DescribeVpcPeeringConnections is the ONLY
// call — it returns full detail (Status, ExpirationTime, both VpcInfo sides)
// in the single paginated list response. NO per-connection describe exists,
// so vpc-peer is the one type with no N+1 fan-out and no degraded-row story:
// a DescribeVpcPeeringConnections denial is a whole-list error (the menu
// shows the error state), never a partial success (docs/resources/vpc-peer
// -impl-plan.md §0).
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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// vpc-peer.* FindingCodes — docs/resources/vpc-peer.md §3.1/§4 (Source
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
)

// FetchVpcPeeringConnectionsPage fetches a single page of VPC Peering
// Connections. DescribeVpcPeeringConnections carries full detail (Status,
// ExpirationTime, both VpcInfo sides) in the list call — no per-connection
// describe, no degraded-row path (docs/resources/vpc-peer.md §1).
func FetchVpcPeeringConnectionsPage(ctx context.Context, api EC2DescribeVpcPeeringConnectionsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeVpcPeeringConnectionsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeVpcPeeringConnectionsOutput, error) {
		return api.DescribeVpcPeeringConnections(ctx, input)
	})
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
	findings := computeVpcPeerFindings(pc)

	requesterVPC, requesterOwner := vpcPeerSide(pc.RequesterVpcInfo)
	accepterVPC, accepterOwner := vpcPeerSide(pc.AccepterVpcInfo)
	return resource.Resource{
		ID:   id,
		Name: name,
		Fields: map[string]string{
			"pcx_id":          id,
			"status":          domain.StatusPhrase(findings),
			"requester_vpc":   requesterVPC,
			"requester_owner": requesterOwner,
			"accepter_vpc":    accepterVPC,
			"accepter_owner":  accepterOwner,
			"expires":         ltFormatTime(pc.ExpirationTime),
		},
		RawStruct: pc,
		Findings:  findings,
	}
}

// vpcPeerSide extracts one side's VpcId/OwnerId, "","" for a nil pointer.
func vpcPeerSide(info *ec2types.VpcPeeringConnectionVpcInfo) (vpcID, ownerID string) {
	if info == nil {
		return "", ""
	}
	return aws.ToString(info.VpcId), aws.ToString(info.OwnerId)
}

// computeVpcPeerFindings builds the ordered Finding slice for one peering
// connection: state first, then (active-only) CIDR overlap —
// docs/resources/vpc-peer.md §4 precedence order. An active connection with
// disjoint CIDRs returns nil (blank Status, Healthy).
func computeVpcPeerFindings(pc *ec2types.VpcPeeringConnection) []domain.Finding {
	if pc.Status == nil {
		return nil
	}
	code := pc.Status.Code
	message := aws.ToString(pc.Status.Message)

	var findings []domain.Finding
	switch code {
	case ec2types.VpcPeeringConnectionStateReasonCodeProvisioning:
		findings = append(findings, domain.Finding{
			Code: vpcPeerCodeProvisioning, Phrase: "provisioning",
			Detail: "Peering connection is being provisioned.", Severity: domain.SevWarn, Source: "wave1",
		})
	case ec2types.VpcPeeringConnectionStateReasonCodeInitiatingRequest:
		findings = append(findings, domain.Finding{
			Code: vpcPeerCodeInitiating, Phrase: "initiating",
			Detail: "Peering request is being initiated.", Severity: domain.SevWarn, Source: "wave1",
		})
	case ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance:
		findings = append(findings, vpcPeerPendingAcceptanceFinding(pc.ExpirationTime))
	case ec2types.VpcPeeringConnectionStateReasonCodeExpired:
		findings = append(findings, domain.Finding{
			Code: vpcPeerCodeExpired, Phrase: "expired: never accepted",
			Detail: "The peering request expired unaccepted; recreate it if still needed.", Severity: domain.SevWarn, Source: "wave1",
		})
	case ec2types.VpcPeeringConnectionStateReasonCodeRejected:
		findings = append(findings, domain.Finding{
			Code: vpcPeerCodeRejected, Phrase: "rejected", Detail: message, Severity: domain.SevBroken, Source: "wave1",
		})
	case ec2types.VpcPeeringConnectionStateReasonCodeFailed:
		findings = append(findings, domain.Finding{
			Code: vpcPeerCodeFailed, Phrase: "failed", Detail: message, Severity: domain.SevBroken, Source: "wave1",
		})
	case ec2types.VpcPeeringConnectionStateReasonCodeDeleting:
		findings = append(findings, domain.Finding{
			Code: vpcPeerCodeDeleting, Phrase: "deleting",
			Detail: "Peering connection is being deleted.", Severity: domain.SevWarn, Source: "wave1",
		})
	case ec2types.VpcPeeringConnectionStateReasonCodeDeleted:
		findings = append(findings, domain.Finding{
			Code: vpcPeerCodeDeleted, Phrase: "deleted",
			Detail: "AWS keeps deleted connections listed for a window.", Severity: domain.SevDim, Source: "wave1",
		})
	}

	// CIDR overlap is active-only: CidrBlock/CidrBlockSet are nil for every
	// non-active connection (VpcPeeringConnectionVpcInfo doc comment) —
	// docs/resources/vpc-peer.md §3.1.
	if code == ec2types.VpcPeeringConnectionStateReasonCodeActive {
		if detail, overlap := vpcPeerCIDROverlapDetail(pc.RequesterVpcInfo, pc.AccepterVpcInfo); overlap {
			findings = append(findings, domain.Finding{
				Code: vpcPeerCodeCidrOverlap, Phrase: "CIDR overlap with peer",
				Detail:   fmt.Sprintf("Requester and accepter CIDR ranges overlap: %s; overlapping subsets blackhole.", detail),
				Severity: domain.SevWarn, Source: "wave1",
			})
		}
	}
	return findings
}

// vpcPeerPendingAcceptanceFinding builds the pending-acceptance Finding: the
// countdown to ExpirationTime IS the S4 phrase (docs/resources/vpc-peer.md
// §4), rounded up to at least 1 day so a request expiring within hours never
// reads as "0d".
func vpcPeerPendingAcceptanceFinding(expiration *time.Time) domain.Finding {
	days := 1
	detail := "The peer has not accepted; AWS expires the request."
	if expiration != nil {
		if remaining := time.Until(*expiration); remaining > 0 {
			if d := int(math.Ceil(remaining.Hours() / 24)); d > days {
				days = d
			}
		}
		detail = fmt.Sprintf("The peer has not accepted; AWS expires the request on %s.", expiration.Format("2006-01-02"))
	}
	return domain.Finding{
		Code:     vpcPeerCodePendingAcceptance,
		Phrase:   fmt.Sprintf("pending acceptance: expires in %dd", days),
		Detail:   detail,
		Severity: domain.SevWarn,
		Source:   "wave1",
	}
}

// vpcPeerCIDROverlapDetail reports whether any IPv4 prefix in requester's
// CidrBlockSet overlaps accepter's, returning the overlapping range pair(s)
// for the S5 Detail sentence. Callers MUST only invoke this for an active
// connection — CidrBlockSet is nil otherwise (docs/resources/vpc-peer.md
// §3.1 load-bearing SDK fact); nil-safe regardless.
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
