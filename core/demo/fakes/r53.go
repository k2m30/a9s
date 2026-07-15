// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// R53Fake implements aws.Route53API against fixture data loaded at construction time.
type R53Fake struct {
	fix *fixtures.R53Fixtures
}

// NewR53 constructs an R53Fake backed by fixture data from the fixtures package.
func NewR53() *R53Fake {
	return &R53Fake{fix: fixtures.NewR53Fixtures()}
}

func (f *R53Fake) ListHostedZones(_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return &route53.ListHostedZonesOutput{HostedZones: f.fix.HostedZones}, nil
}

func (f *R53Fake) ListResourceRecordSets(_ context.Context, input *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	if input.HostedZoneId == nil {
		return nil, fmt.Errorf("ListResourceRecordSets: HostedZoneId is required")
	}
	records := f.fix.RecordSets[*input.HostedZoneId]
	return &route53.ListResourceRecordSetsOutput{ResourceRecordSets: records}, nil
}

// GetHostedZone returns the hosted zone detail including VPC associations.
// In demo mode the private zone Z1234567890ABCDEFGHIJ is returned with no VPC
// associations, triggering the orphan finding in EnrichRoute53Zone. The
// private zone Z5678901234ABCDEFGHIJ carries a real VPC association —
// required for the r53:vpc related-panel pivot (checkR53VPC) — kept distinct
// from the orphan zone so the Wave-2 finding demo is undisturbed.
func (f *R53Fake) GetHostedZone(_ context.Context, input *route53.GetHostedZoneInput, _ ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error) {
	if input.Id == nil {
		return nil, fmt.Errorf("GetHostedZone: Id is required")
	}
	for i := range f.fix.HostedZones {
		hz := f.fix.HostedZones[i]
		if hz.Id == nil || *hz.Id != *input.Id {
			continue
		}
		out := &route53.GetHostedZoneOutput{HostedZone: &hz}
		if hz.Config != nil && hz.Config.PrivateZone {
			switch *hz.Id {
			case "/hostedzone/Z5678901234ABCDEFGHIJ":
				out.VPCs = []r53types.VPC{
					{VPCId: aws.String("vpc-0abc123def456789a"), VPCRegion: r53types.VPCRegionUsEast1},
				}
			default:
				// Private zone with no VPCs — demo triggers the orphan finding.
				out.VPCs = []r53types.VPC{}
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("GetHostedZone: zone %q not found", *input.Id)
}

// ListQueryLoggingConfigs returns the fixture query-logging configuration
// for the requested hosted zone, backing the r53:logs related-panel pivot
// (checkR53Logs).
func (f *R53Fake) ListQueryLoggingConfigs(_ context.Context, input *route53.ListQueryLoggingConfigsInput, _ ...func(*route53.Options)) (*route53.ListQueryLoggingConfigsOutput, error) {
	var zoneID string
	if input != nil && input.HostedZoneId != nil {
		zoneID = *input.HostedZoneId
	}
	cfg, ok := f.fix.QueryLoggingConfigs[zoneID]
	if !ok {
		return &route53.ListQueryLoggingConfigsOutput{}, nil
	}
	return &route53.ListQueryLoggingConfigsOutput{
		QueryLoggingConfigs: []r53types.QueryLoggingConfig{cfg},
	}, nil
}

// ListHostedZonesByVPC returns the private hosted zones associated with the
// requested VPC, backing the vpce:r53 related-panel pivot (checkVPCER53).
// Mirrors GetHostedZone's VPC associations: only
// Z5678901234ABCDEFGHIJ carries a VPC association in demo mode.
func (f *R53Fake) ListHostedZonesByVPC(_ context.Context, input *route53.ListHostedZonesByVPCInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesByVPCOutput, error) {
	if input == nil || input.VPCId == nil {
		return nil, fmt.Errorf("ListHostedZonesByVPC: VPCId is required")
	}
	if *input.VPCId != "vpc-0abc123def456789a" {
		return &route53.ListHostedZonesByVPCOutput{}, nil
	}
	const zoneID = "Z5678901234ABCDEFGHIJ"
	for i := range f.fix.HostedZones {
		hz := f.fix.HostedZones[i]
		if hz.Id == nil || *hz.Id != "/hostedzone/"+zoneID {
			continue
		}
		return &route53.ListHostedZonesByVPCOutput{
			HostedZoneSummaries: []r53types.HostedZoneSummary{
				{
					HostedZoneId: aws.String(zoneID),
					Name:         hz.Name,
					Owner:        &r53types.HostedZoneOwner{OwningAccount: aws.String("123456789012")},
				},
			},
		}, nil
	}
	return &route53.ListHostedZonesByVPCOutput{}, nil
}
