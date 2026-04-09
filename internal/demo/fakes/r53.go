package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/route53"

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
