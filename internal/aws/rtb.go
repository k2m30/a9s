package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// rtbCodeBlackholeRoute is the canonical FindingCode for a route table with
// at least one blackhole route (target deleted, e.g. a NAT gateway or peering
// connection that no longer exists).
const rtbCodeBlackholeRoute domain.FindingCode = "rtb.route.blackhole"

// rtbCodeOrphanUnassociated is the canonical FindingCode for a non-main route
// table with zero subnet associations — unreachable, likely orphaned.
const rtbCodeOrphanUnassociated domain.FindingCode = "rtb.orphan-unassociated"

// FetchRouteTablesPage fetches a single page of route tables.
func FetchRouteTablesPage(ctx context.Context, api EC2DescribeRouteTablesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeRouteTablesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeRouteTables(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching route tables: %w", err)
	}

	var resources []resource.Resource

	for _, rtb := range output.RouteTables {
		rtbID := ""
		if rtb.RouteTableId != nil {
			rtbID = *rtb.RouteTableId
		}

		name := ""
		for _, tag := range rtb.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		vpcID := ""
		if rtb.VpcId != nil {
			vpcID = *rtb.VpcId
		}

		routesCount := fmt.Sprintf("%d", len(rtb.Routes))
		associationsCount := fmt.Sprintf("%d", len(rtb.Associations))

		// Determine if this is the main route table
		isMain := "false"
		for _, assoc := range rtb.Associations {
			if assoc.Main != nil && *assoc.Main {
				isMain = "true"
				break
			}
		}

		// Count blackhole routes (target deleted)
		blackholeCount := 0
		for _, route := range rtb.Routes {
			if route.State == ec2types.RouteStateBlackhole {
				blackholeCount++
			}
		}

		r := resource.Resource{
			ID:   rtbID,
			Name: name,
			Fields: map[string]string{
				"route_table_id":         rtbID,
				"name":                   name,
				"vpc_id":                 vpcID,
				"routes_count":           routesCount,
				"associations_count":     associationsCount,
				"blackhole_routes_count": fmt.Sprintf("%d", blackholeCount),
				"is_main":                isMain,
			},
			RawStruct: rtb,
		}

		// mirrors colorRTB's own precedence: blackhole routes win first, then
		// an unassociated non-main table.
		switch {
		case blackholeCount > 0:
			r.Findings = []domain.Finding{{
				Code: rtbCodeBlackholeRoute, Phrase: "blackhole route (target deleted)",
				Severity: domain.SevBroken, Source: "wave1",
			}}
		case len(rtb.Associations) == 0 && isMain != "true":
			r.Findings = []domain.Finding{{
				Code: rtbCodeOrphanUnassociated, Phrase: "no subnet associations",
				Severity: domain.SevWarn, Source: "wave1",
			}}
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
