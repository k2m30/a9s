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
			Detail: []DetailField{
				{Path: "Id"}, {Path: "Name"}, {Path: "CallerReference"}, {Path: "ResourceRecordSetCount"},
				{Path: "Config"}, {Path: "LinkedService"},
			},
		},
		"cf": {
			List: []ListColumn{
				{Title: "Domain Name", Path: "DomainName", Width: 40},
				{Title: "Distribution ID", Path: "Id", Width: 16},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "WAF", Path: "WebACLId", Width: 14},
				{Title: "TLS", Path: "ViewerCertificate.MinimumProtocolVersion", Width: 14},
				{Title: "Enabled", Path: "Enabled", Width: 9},
				{Title: "Aliases", Path: "Aliases.Items", Width: 30},
				{Title: "Price Class", Path: "PriceClass", Width: 16},
			},
			Detail: []DetailField{
				{Path: "Id"}, {Path: "DomainName"}, {Path: "Status"}, {Path: "Enabled"}, {Path: "Comment"},
				{Path: "ARN"}, {Path: "Aliases"}, {Path: "Origins"}, {Path: "PriceClass"}, {Path: "HttpVersion"},
				{Path: "LastModifiedTime"}, {Path: "DefaultCacheBehavior"},
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
			Detail: []DetailField{
				{Path: "DomainName"}, {Path: "CertificateArn"}, {Path: "SubjectAlternativeNameSummaries"},
				{Path: "Status"}, {Path: "Type"}, {Path: "NotBefore"}, {Path: "NotAfter"},
				{Path: "IssuedAt"}, {Path: "ImportedAt"}, {Path: "InUse"}, {Path: "CreatedAt"},
				{Path: "RenewalEligibility"}, {Path: "KeyAlgorithm"},
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
			Detail: []DetailField{
				{Path: "ApiId"}, {Path: "Name"}, {Path: "ProtocolType"}, {Path: "ApiEndpoint"},
				{Path: "Description"}, {Path: "CreatedDate"}, {Path: "ApiKeySelectionExpression"},
				{Path: "RouteSelectionExpression"}, {Path: "CorsConfiguration"}, {Path: "Tags"},
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
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Type"}, {Path: "TTL"}, {Path: "ResourceRecords"}, {Path: "AliasTarget"},
				{Path: "SetIdentifier"}, {Path: "Weight"}, {Path: "Region"}, {Path: "Failover"},
				{Path: "GeoLocation"}, {Path: "HealthCheckId"}, {Path: "MultiValueAnswer"},
			},
		},
	}
}
