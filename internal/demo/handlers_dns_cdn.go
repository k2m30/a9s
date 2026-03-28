package demo

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// registerDNSCDNHandlers registers Route53, CloudFront, and API Gateway handlers.
func registerDNSCDNHandlers(t *Transport) {
	registerRoute53Handlers(t)
	registerCloudFrontHandlers(t)
	registerAPIGatewayHandlers(t)
}

// ---------------------------------------------------------------------------
// Route53 (restxml, service "route53")
// ---------------------------------------------------------------------------

func registerRoute53Handlers(t *Transport) {
	t.Handle("route53", "ListHostedZones", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["r53"]()
		zones := ExtractSDK[r53types.HostedZone](resources)

		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		sb.WriteString(`<ListHostedZonesResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">`)
		sb.WriteString(`<HostedZones>`)
		for _, z := range zones {
			zoneID := aws.ToString(z.Id)
			name := aws.ToString(z.Name)
			callerRef := aws.ToString(z.CallerReference)
			recordCount := int64(0)
			if z.ResourceRecordSetCount != nil {
				recordCount = *z.ResourceRecordSetCount
			}
			privateZone := "false"
			comment := ""
			if z.Config != nil {
				if z.Config.PrivateZone {
					privateZone = "true"
				}
				comment = aws.ToString(z.Config.Comment)
			}

			fmt.Fprintf(&sb, `<HostedZone>`)
			fmt.Fprintf(&sb, `<Id>%s</Id>`, xmlEscape(zoneID))
			fmt.Fprintf(&sb, `<Name>%s</Name>`, xmlEscape(name))
			fmt.Fprintf(&sb, `<CallerReference>%s</CallerReference>`, xmlEscape(callerRef))
			fmt.Fprintf(&sb, `<ResourceRecordSetCount>%d</ResourceRecordSetCount>`, recordCount)
			fmt.Fprintf(&sb, `<Config>`)
			fmt.Fprintf(&sb, `<PrivateZone>%s</PrivateZone>`, privateZone)
			if comment != "" {
				fmt.Fprintf(&sb, `<Comment>%s</Comment>`, xmlEscape(comment))
			}
			fmt.Fprintf(&sb, `</Config>`)
			fmt.Fprintf(&sb, `</HostedZone>`)
		}
		sb.WriteString(`</HostedZones>`)
		sb.WriteString(`<IsTruncated>false</IsTruncated>`)
		sb.WriteString(`<MaxItems>100</MaxItems>`)
		sb.WriteString(`</ListHostedZonesResponse>`)

		return XMLResponse(sb.String()), nil
	})

	t.Handle("route53", "ListResourceRecordSets", func(req *http.Request) (*http.Response, error) {
		// Extract zone ID from URL path: /2013-04-01/hostedzone/{HostedZoneId}/rrset
		// The SDK v2 strips the "/hostedzone/" prefix, so the path contains the bare
		// zone ID (e.g. Z0123456789ABCDEFGHIJ). We then prepend "/hostedzone/" to
		// match the fixture key format used in r53RecordData.
		rawPath := req.URL.EscapedPath()
		zoneID := zoneIDFromRRSetPath(rawPath)
		if zoneID != "" && !strings.HasPrefix(zoneID, "/hostedzone/") {
			zoneID = "/hostedzone/" + zoneID
		}
		zoneRecords, ok := GetR53Records(zoneID)
		if !ok {
			// Unknown zone — return empty record set
			var sb strings.Builder
			sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
			sb.WriteString(`<ListResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">`)
			sb.WriteString(`<ResourceRecordSets/>`)
			sb.WriteString(`<IsTruncated>false</IsTruncated>`)
			sb.WriteString(`<MaxItems>100</MaxItems>`)
			sb.WriteString(`</ListResourceRecordSetsResponse>`)
			return XMLResponse(sb.String()), nil
		}

		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		sb.WriteString(`<ListResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">`)
		sb.WriteString(`<ResourceRecordSets>`)
		for _, rr := range zoneRecords {
			rrSet, ok := rr.RawStruct.(r53types.ResourceRecordSet)
			if !ok {
				continue
			}
			name := aws.ToString(rrSet.Name)
			rrType := string(rrSet.Type)
			ttl := ""
			if rrSet.TTL != nil {
				ttl = fmt.Sprintf("%d", *rrSet.TTL)
			}

			fmt.Fprintf(&sb, `<ResourceRecordSet>`)
			fmt.Fprintf(&sb, `<Name>%s</Name>`, xmlEscape(name))
			fmt.Fprintf(&sb, `<Type>%s</Type>`, xmlEscape(rrType))
			if ttl != "" {
				fmt.Fprintf(&sb, `<TTL>%s</TTL>`, ttl)
			}
			if len(rrSet.ResourceRecords) > 0 {
				sb.WriteString(`<ResourceRecords>`)
				for _, rec := range rrSet.ResourceRecords {
					fmt.Fprintf(&sb, `<ResourceRecord><Value>%s</Value></ResourceRecord>`, xmlEscape(aws.ToString(rec.Value)))
				}
				sb.WriteString(`</ResourceRecords>`)
			}
			if rrSet.AliasTarget != nil {
				fmt.Fprintf(&sb, `<AliasTarget>`)
				fmt.Fprintf(&sb, `<HostedZoneId>%s</HostedZoneId>`, xmlEscape(aws.ToString(rrSet.AliasTarget.HostedZoneId)))
				fmt.Fprintf(&sb, `<DNSName>%s</DNSName>`, xmlEscape(aws.ToString(rrSet.AliasTarget.DNSName)))
				fmt.Fprintf(&sb, `<EvaluateTargetHealth>%v</EvaluateTargetHealth>`, rrSet.AliasTarget.EvaluateTargetHealth)
				fmt.Fprintf(&sb, `</AliasTarget>`)
			}
			fmt.Fprintf(&sb, `</ResourceRecordSet>`)
		}
		sb.WriteString(`</ResourceRecordSets>`)
		sb.WriteString(`<IsTruncated>false</IsTruncated>`)
		sb.WriteString(`<MaxItems>100</MaxItems>`)
		sb.WriteString(`</ListResourceRecordSetsResponse>`)

		return XMLResponse(sb.String()), nil
	})
}

// zoneIDFromRRSetPath extracts the bare Route53 hosted zone ID from a
// ListResourceRecordSets URL path.
//
// The AWS SDK v2 strips the "/hostedzone/" prefix from zone IDs before
// inserting them into the URL path, so the SDK sends:
//
//	/2013-04-01/hostedzone/ZXXX/rrset
//
// This function strips the known prefix (/2013-04-01/hostedzone/) and suffix
// (/rrset), then URL-decodes the remaining segment (to handle the rare case
// where older callers URL-encoded the zone ID). The returned value is a bare
// zone ID such as "ZXXX" — callers that need the "/hostedzone/ZXXX" form
// must prepend the prefix themselves.
func zoneIDFromRRSetPath(rawPath string) string {
	const prefix = "/2013-04-01/hostedzone/"
	const suffix = "/rrset"
	if !strings.HasPrefix(rawPath, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(rawPath, prefix)
	if idx := strings.Index(rest, suffix); idx != -1 {
		rest = rest[:idx]
	}
	decoded, err := url.PathUnescape(rest)
	if err != nil {
		return rest
	}
	return decoded
}

// ---------------------------------------------------------------------------
// CloudFront (restxml, service "cloudfront")
// ---------------------------------------------------------------------------

func registerCloudFrontHandlers(t *Transport) {
	t.Handle("cloudfront", "ListDistributions", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["cf"]()
		dists := ExtractSDK[cftypes.DistributionSummary](resources)

		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		// CloudFront restxml: the root element IS <DistributionList> — no <ListDistributionsResponse> wrapper.
		// The SDK calls FetchRootElement then awsRestxml_deserializeDocumentDistributionList directly.
		sb.WriteString(`<DistributionList xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/">`)
		fmt.Fprintf(&sb, `<MaxItems>%d</MaxItems>`, len(dists))
		fmt.Fprintf(&sb, `<Quantity>%d</Quantity>`, len(dists))
		sb.WriteString(`<IsTruncated>false</IsTruncated>`)
		sb.WriteString(`<Items>`)
		for _, d := range dists {
			distID := aws.ToString(d.Id)
			arn := aws.ToString(d.ARN)
			domainName := aws.ToString(d.DomainName)
			status := aws.ToString(d.Status)
			enabled := "false"
			if d.Enabled != nil && *d.Enabled {
				enabled = "true"
			}
			comment := aws.ToString(d.Comment)
			priceClass := string(d.PriceClass)

			fmt.Fprintf(&sb, `<DistributionSummary>`)
			fmt.Fprintf(&sb, `<Id>%s</Id>`, xmlEscape(distID))
			fmt.Fprintf(&sb, `<ARN>%s</ARN>`, xmlEscape(arn))
			fmt.Fprintf(&sb, `<DomainName>%s</DomainName>`, xmlEscape(domainName))
			fmt.Fprintf(&sb, `<Status>%s</Status>`, xmlEscape(status))
			fmt.Fprintf(&sb, `<Enabled>%s</Enabled>`, enabled)
			fmt.Fprintf(&sb, `<Comment>%s</Comment>`, xmlEscape(comment))
			fmt.Fprintf(&sb, `<PriceClass>%s</PriceClass>`, xmlEscape(priceClass))
			// Aliases
			aliases := []string{}
			if d.Aliases != nil {
				aliases = d.Aliases.Items
			}
			fmt.Fprintf(&sb, `<Aliases><Quantity>%d</Quantity>`, len(aliases))
			if len(aliases) > 0 {
				sb.WriteString(`<Items>`)
				for _, alias := range aliases {
					fmt.Fprintf(&sb, `<CNAME>%s</CNAME>`, xmlEscape(alias))
				}
				sb.WriteString(`</Items>`)
			}
			sb.WriteString(`</Aliases>`)
			// Origins (required by SDK)
			sb.WriteString(`<Origins><Quantity>0</Quantity></Origins>`)
			// DefaultCacheBehavior (required by SDK)
			sb.WriteString(`<DefaultCacheBehavior>`)
			sb.WriteString(`<TargetOriginId>demo</TargetOriginId>`)
			sb.WriteString(`<ViewerProtocolPolicy>redirect-to-https</ViewerProtocolPolicy>`)
			sb.WriteString(`<TrustedSigners><Enabled>false</Enabled><Quantity>0</Quantity></TrustedSigners>`)
			sb.WriteString(`<ForwardedValues><QueryString>false</QueryString><Cookies><Forward>none</Forward></Cookies></ForwardedValues>`)
			sb.WriteString(`<MinTTL>0</MinTTL>`)
			sb.WriteString(`</DefaultCacheBehavior>`)
			// CacheBehaviors (required)
			sb.WriteString(`<CacheBehaviors><Quantity>0</Quantity></CacheBehaviors>`)
			// CustomErrorResponses (required)
			sb.WriteString(`<CustomErrorResponses><Quantity>0</Quantity></CustomErrorResponses>`)
			// Restrictions (required)
			sb.WriteString(`<Restrictions><GeoRestriction><RestrictionType>none</RestrictionType><Quantity>0</Quantity></GeoRestriction></Restrictions>`)
			// WebAclId
			sb.WriteString(`<WebACLId/>`)
			// HttpVersion
			sb.WriteString(`<HttpVersion>http2</HttpVersion>`)
			// IsIPV6Enabled
			sb.WriteString(`<IsIPV6Enabled>true</IsIPV6Enabled>`)
			fmt.Fprintf(&sb, `</DistributionSummary>`)
		}
		sb.WriteString(`</Items>`)
		sb.WriteString(`</DistributionList>`)

		return XMLResponse(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// API Gateway V2 (restjson1, service "apigateway")
// The deserializer expects lowercase "items".
// ---------------------------------------------------------------------------

func registerAPIGatewayHandlers(t *Transport) {
	t.Handle("apigateway", "GetApis", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["apigw"]()
		apis := ExtractSDK[apigwtypes.Api](resources)

		apiMaps := make([]map[string]interface{}, 0, len(apis))
		for _, a := range apis {
			m := map[string]interface{}{
				"name":         aws.ToString(a.Name),
				"apiId":        aws.ToString(a.ApiId),
				"apiEndpoint":  aws.ToString(a.ApiEndpoint),
				"protocolType": string(a.ProtocolType),
			}
			if a.Description != nil {
				m["description"] = *a.Description
			}
			apiMaps = append(apiMaps, m)
		}

		return JSONResponse(map[string]interface{}{"items": apiMaps})
	})
}
