package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// Route53ListHostedZonesAPI defines the interface for the Route53 ListHostedZones operation.
type Route53ListHostedZonesAPI interface {
	ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
}

// Route53ListResourceRecordSetsAPI defines the interface for the Route53 ListResourceRecordSets operation.
type Route53ListResourceRecordSetsAPI interface {
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
}

// Route53GetHostedZoneAPI defines the interface for the Route53 GetHostedZone operation.
// Used by Wave 2 enrichment to retrieve VPC associations for private hosted zones.
type Route53GetHostedZoneAPI interface {
	GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error)
}

// Route53API is the aggregate interface covering all Route53 operations used by a9s fetchers.
// *route53.Client structurally satisfies this interface.
type Route53API interface {
	Route53ListHostedZonesAPI
	Route53ListResourceRecordSetsAPI
	Route53GetHostedZoneAPI // Wave 2 enrichment
}
