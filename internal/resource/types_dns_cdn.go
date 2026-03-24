package resource

func dnsCdnResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:      "Route 53 Hosted Zones",
			ShortName: "r53",
			Aliases:   []string{"r53", "route53", "dns", "hosted-zones"},
			Category:  "DNS & CDN",
			Columns: []Column{
				{Key: "zone_id", Title: "Zone ID", Width: 30, Sortable: true},
				{Key: "name", Title: "Name", Width: 36, Sortable: true},
				{Key: "record_count", Title: "Records", Width: 9, Sortable: true},
				{Key: "private_zone", Title: "Private", Width: 9, Sortable: true},
				{Key: "comment", Title: "Comment", Width: 30, Sortable: false},
			},
			Children: []ChildViewDef{{
				ChildType:      "r53_records",
				Key:            "enter",
				ContextKeys:    map[string]string{"zone_id": "ID", "zone_name": "Name"},
				DisplayNameKey: "zone_name",
			}},
		},
		{
			Name:      "CloudFront Distributions",
			ShortName: "cf",
			Aliases:   []string{"cf", "cloudfront", "cdn"},
			Category:  "DNS & CDN",
			Columns: []Column{
				{Key: "distribution_id", Title: "Distribution ID", Width: 16, Sortable: true},
				{Key: "domain_name", Title: "Domain Name", Width: 40, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "enabled", Title: "Enabled", Width: 9, Sortable: true},
				{Key: "aliases", Title: "Aliases", Width: 30, Sortable: false},
				{Key: "price_class", Title: "Price Class", Width: 16, Sortable: true},
			},
		},
		{
			Name:      "ACM Certificates",
			ShortName: "acm",
			Aliases:   []string{"acm", "certificates", "certs"},
			Category:  "DNS & CDN",
			Columns: []Column{
				{Key: "domain_name", Title: "Domain Name", Width: 40, Sortable: true},
				{Key: "status", Title: "Status", Width: 14, Sortable: true},
				{Key: "type", Title: "Type", Width: 14, Sortable: true},
				{Key: "not_after", Title: "Expires", Width: 22, Sortable: true},
				{Key: "in_use", Title: "In Use", Width: 8, Sortable: true},
			},
		},
		{
			Name:      "API Gateways",
			ShortName: "apigw",
			Aliases:   []string{"apigw", "apigateway", "api-gateway"},
			Category:  "DNS & CDN",
			Columns: []Column{
				{Key: "api_id", Title: "API ID", Width: 14, Sortable: true},
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "protocol", Title: "Protocol", Width: 12, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 50, Sortable: false},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
			},
		},
	}
}
