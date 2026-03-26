package config

func dnsCdnDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"r53": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 36},
				{Title: "Zone ID", Path: "Id", Width: 30},
				{Title: "Records", Path: "ResourceRecordSetCount", Width: 9},
				{Title: "Private", Path: "Config.PrivateZone", Width: 9},
				{Title: "Comment", Path: "Config.Comment", Width: 30},
			},
			Detail: []string{
				"Id", "Name", "CallerReference", "ResourceRecordSetCount",
				"Config", "LinkedService",
			},
		},
		"cf": {
			List: []ListColumn{
				{Title: "Domain Name", Path: "DomainName", Width: 40},
				{Title: "Distribution ID", Path: "Id", Width: 16},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Enabled", Path: "Enabled", Width: 9},
				{Title: "Aliases", Path: "Aliases.Items", Width: 30},
				{Title: "Price Class", Path: "PriceClass", Width: 16},
			},
			Detail: []string{
				"Id", "DomainName", "Status", "Enabled", "Comment",
				"ARN", "Aliases", "Origins", "PriceClass", "HttpVersion",
				"LastModifiedTime", "DefaultCacheBehavior",
			},
		},
		"acm": {
			List: []ListColumn{
				{Title: "Domain Name", Path: "DomainName", Width: 40},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Type", Path: "Type", Width: 14},
				{Title: "Expires", Path: "NotAfter", Width: 22},
				{Title: "In Use", Path: "InUse", Width: 8},
			},
			Detail: []string{
				"DomainName", "CertificateArn", "SubjectAlternativeNameSummaries",
				"Status", "Type", "NotBefore", "NotAfter",
				"IssuedAt", "ImportedAt", "InUse", "CreatedAt",
				"RenewalEligibility", "KeyAlgorithm",
			},
		},
		"apigw": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 28},
				{Title: "API ID", Path: "ApiId", Width: 14},
				{Title: "Protocol", Path: "ProtocolType", Width: 12},
				{Title: "Endpoint", Path: "ApiEndpoint", Width: 50},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"ApiId", "Name", "ProtocolType", "ApiEndpoint",
				"Description", "CreatedDate", "ApiKeySelectionExpression",
				"RouteSelectionExpression", "CorsConfiguration", "Tags",
			},
		},
		// Child views for DNS/CDN resources
		"r53_records": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 40},
				{Title: "Type", Path: "Type", Width: 8},
				{Title: "TTL", Path: "TTL", Width: 8},
				{Title: "Values", Path: "", Key: "values", Width: 50},
			},
			Detail: []string{
				"Name", "Type", "TTL", "ResourceRecords", "AliasTarget",
				"SetIdentifier", "Weight", "Region", "Failover",
				"GeoLocation", "HealthCheckId", "MultiValueAnswer",
			},
		},
	}
}
