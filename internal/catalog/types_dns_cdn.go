package catalog

import "github.com/k2m30/a9s/v3/internal/domain"

func colorCF(r domain.Resource) domain.Color {
	if r.Fields["enabled"] == "false" {
		return domain.ColorDim
	}
	switch r.Fields["status"] {
	case "Deployed":
		return domain.ColorHealthy
	case "InProgress":
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorAPIGW(_ domain.Resource) domain.Color { return domain.ColorHealthy }

var dnsCdnTypes = []ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Route 53 Hosted Zones",
		ShortName:     "r53",
		Aliases:       []string{"r53", "route53", "dns", "hosted-zones"},
		Category:      "DNS & CDN",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 36, Sortable: true},
			{Key: "zone_id", Title: "Zone ID", Width: 30, Sortable: true},
			{Key: "record_count", Title: "Records", Width: 9, Sortable: true},
			{Key: "private_zone", Title: "Private", Width: 9, Sortable: true},
			{Key: "comment", Title: "Comment", Width: 30, Sortable: false},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "r53_records",
			Key:            "enter",
			ContextKeys:    map[string]string{"zone_id": "ID", "zone_name": "Name"},
			DisplayNameKey: "zone_name",
		}},
		Color: r53Color,
	},
	{
		Name:          "CloudFront Distributions",
		ShortName:     "cf",
		Aliases:       []string{"cf", "cloudfront", "cdn"},
		Category:      "DNS & CDN",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "domain_name", Title: "Domain Name", Width: 40, Sortable: true},
			{Key: "distribution_id", Title: "Distribution ID", Width: 16, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "enabled", Title: "Enabled", Width: 9, Sortable: true},
			{Key: "aliases", Title: "Aliases", Width: 30, Sortable: false},
			{Key: "price_class", Title: "Price Class", Width: 16, Sortable: true},
		},
		Color: colorCF,
	},
	{
		Name:          "ACM Certificates",
		ShortName:     "acm",
		Aliases:       []string{"acm", "certificates", "certs"},
		Category:      "DNS & CDN",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "domain_name", Title: "Domain Name", Width: 40, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "not_after", Title: "Expires", Width: 22, Sortable: true},
			{Key: "in_use", Title: "In Use", Width: 8, Sortable: true},
		},
		Color: acmColor,
	},
	{
		Name:          "API Gateways",
		ShortName:     "apigw",
		Aliases:       []string{"apigw", "apigateway", "api-gateway"},
		Category:      "DNS & CDN",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "api_id", Title: "API ID", Width: 14, Sortable: true},
			{Key: "protocol", Title: "Protocol", Width: 12, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 50, Sortable: false},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Color: colorAPIGW,
	},
}
