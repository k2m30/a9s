package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/route53"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("r53", []string{"zone_id", "name", "record_count", "private_zone", "comment"})
	resource.Register("r53", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchHostedZones(ctx, c.Route53)
	})
}

// FetchHostedZones calls the Route53 ListHostedZones API and converts
// the response into a slice of generic Resource structs.
func FetchHostedZones(ctx context.Context, api Route53ListHostedZonesAPI) ([]resource.Resource, error) {
	output, err := api.ListHostedZones(ctx, &route53.ListHostedZonesInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, zone := range output.HostedZones {
		zoneID := ""
		if zone.Id != nil {
			zoneID = *zone.Id
		}

		name := ""
		if zone.Name != nil {
			name = *zone.Name
		}

		recordCount := ""
		if zone.ResourceRecordSetCount != nil {
			recordCount = fmt.Sprintf("%d", *zone.ResourceRecordSetCount)
		}

		privateZone := "false"
		comment := ""
		if zone.Config != nil {
			if zone.Config.PrivateZone {
				privateZone = "true"
			}
			if zone.Config.Comment != nil {
				comment = *zone.Config.Comment
			}
		}

		callerRef := ""
		if zone.CallerReference != nil {
			callerRef = *zone.CallerReference
		}

		// Build DetailData
		detail := map[string]string{
			"Zone ID":          zoneID,
			"Name":             name,
			"Record Count":     recordCount,
			"Private Zone":     privateZone,
			"Comment":          comment,
			"Caller Reference": callerRef,
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(zone, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     zoneID,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"zone_id":      zoneID,
				"name":         name,
				"record_count": recordCount,
				"private_zone": privateZone,
				"comment":      comment,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  zone,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
