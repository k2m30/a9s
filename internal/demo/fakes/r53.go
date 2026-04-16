package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
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
// associations, triggering the orphan finding in EnrichRoute53Zone.
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
		// Private zone with no VPCs — demo triggers the orphan finding.
		if hz.Config != nil && hz.Config.PrivateZone {
			out.VPCs = []r53types.VPC{}
		}
		return out, nil
	}
	return nil, fmt.Errorf("GetHostedZone: zone %q not found", *input.Id)
}
