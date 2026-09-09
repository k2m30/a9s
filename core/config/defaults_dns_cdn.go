// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func dnsCdnDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"r53": {
			Detail: []DetailField{
				{Path: "Id"}, {Path: "Name"}, {Path: "CallerReference"}, {Path: "ResourceRecordSetCount"},
				{Path: "Config"}, {Path: "LinkedService"},
			},
		},
		"cf": {
			Detail: []DetailField{
				{Path: "Id"}, {Path: "DomainName"}, {Path: "Status"}, {Path: "Enabled"}, {Path: "Comment"},
				{Path: "ARN"}, {Path: "Aliases"}, {Path: "Origins"}, {Path: "PriceClass"}, {Path: "HttpVersion"},
				{Path: "LastModifiedTime"}, {Path: "DefaultCacheBehavior"},
			},
		},
		"acm": {
			Detail: []DetailField{
				{Path: "DomainName"}, {Path: "CertificateArn"}, {Path: "SubjectAlternativeNameSummaries"},
				{Path: "Status"}, {Path: "Type"}, {Path: "NotBefore"}, {Path: "NotAfter"},
				{Path: "IssuedAt"}, {Path: "ImportedAt"}, {Path: "InUse"}, {Path: "CreatedAt"},
				{Path: "RenewalEligibility"}, {Path: "KeyAlgorithm"},
			},
		},
		"apigw": {
			Detail: []DetailField{
				{Key: "api_id", Label: "ApiId"}, {Path: "Name"},
				{Key: "protocol", Label: "Protocol"}, {Key: "endpoint", Label: "Endpoint"},
				{Path: "Description"}, {Path: "CreatedDate"}, {Path: "ApiKeySelectionExpression"},
				{Path: "RouteSelectionExpression"}, {Path: "CorsConfiguration"}, {Path: "Tags"},
			},
		},
		// Child views for DNS/CDN resources
		"r53_records": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Type"}, {Path: "TTL"}, {Path: "ResourceRecords"}, {Path: "AliasTarget"},
				{Path: "SetIdentifier"}, {Path: "Weight"}, {Path: "Region"}, {Path: "Failover"},
				{Path: "GeoLocation"}, {Path: "HealthCheckId"}, {Path: "MultiValueAnswer"},
			},
		},
	}
}
