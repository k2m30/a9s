// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// elb_related.go contains ELB related-resource checker functions.
package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkELBTargetGroups checks the cache for target groups whose LoadBalancerArns
// contains this ELB's ARN.
func checkELBTargetGroups(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return keyMissing("tg", "elbARN")
	}

	tgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "tg")
	if err != nil {
		return ReadFailed("tg", err)
	}
	if tgList == nil {
		return NotRead("tg")
	}

	var ids []string
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		if !ok {
			continue
		}
		if slices.Contains(raw.LoadBalancerArns, elbARN) {
			ids = append(ids, tgRes.ID)
		}
	}
	return relatedResultTrunc("tg", ids, truncated)
}

// checkELBAlarms checks the cache for CloudWatch alarms with a "LoadBalancer"
// dimension matching the ARN suffix of this ELB (everything after "loadbalancer/").
func checkELBAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "elb", res)
}

// checkELBSG extracts security group IDs from the ELBv2 LoadBalancer's
// SecurityGroups slice (ALBs only; NLBs and GLBs return an empty list).
// Pattern F — no cache needed.
func checkELBSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	var ids []string
	for _, sgID := range raw.SecurityGroups {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkELBVPC returns the VPC this load balancer runs in (Pattern F).
// Reads vpc_id from Fields which is populated by the ELB fetcher.
func checkELBVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return foundNone("vpc", "vpcID")
	}
	return relatedResultTrunc("vpc", []string{vpcID}, false)
}

// checkELBCFN reports the CloudFormation stack owning this ELB via the
// aws:cloudformation:stack-name tag. Pattern C: one elbv2:DescribeTags call
// keyed by the LoadBalancer ARN.
func checkELBCFN(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return keyMissing("cfn", "elbARN")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return NotRead("cfn")
	}
	api, ok := c.ELBv2.(ELBv2DescribeTagsAPI)
	if !ok {
		return NotRead("cfn")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTagsOutput, error) {
		return api.DescribeTags(ctx, &elbv2.DescribeTagsInput{ResourceArns: []string{elbARN}})
	})
	if err != nil {
		return ReadFailed("cfn", err)
	}
	for _, td := range out.TagDescriptions {
		for _, tag := range td.Tags {
			if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil && *tag.Value != "" {
				return relatedResultTrunc("cfn", []string{*tag.Value}, false)
			}
		}
	}
	return foundNone("cfn", "the aws:cloudformation:stack-name tag")
}

// checkELBACM reports ACM certificates attached to this ELB's HTTPS/TLS
// listeners. Pattern C: one elbv2:DescribeListeners call per ELB, plus one
// elbv2:DescribeListenerCertificates per HTTPS/TLS listener. Listener
// .Certificates carries the listener's default certificate alone; the
// certificates it serves by SNI are what DescribeListenerCertificates
// answers with, and ACM records the load balancer as a user of every one of
// them.
func checkELBACM(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return keyMissing("acm", "elbARN")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return NotRead("acm")
	}
	listeners, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]elbv2types.Listener, *string, error) {
		out, err := c.ELBv2.DescribeListeners(ctx, &elbv2.DescribeListenersInput{LoadBalancerArn: &elbARN, Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		return out.Listeners, out.NextMarker, nil
	})
	if err != nil {
		return ReadFailed("acm", err)
	}
	var ids []string
	seen := make(map[string]bool)
	add := func(certs []elbv2types.Certificate) {
		for _, cert := range certs {
			arn := aws.ToString(cert.CertificateArn)
			if arn == "" || seen[arn] {
				continue
			}
			seen[arn] = true
			ids = append(ids, arn)
		}
	}
	certAPI, sniReadable := c.ELBv2.(ELBv2DescribeListenerCertificatesAPI)
	var failures []Failure
	for _, ls := range listeners {
		add(ls.Certificates)
		if ls.Protocol != elbv2types.ProtocolEnumHttps && ls.Protocol != elbv2types.ProtocolEnumTls {
			continue
		}
		if !sniReadable {
			complete = false
			continue
		}
		listenerARN := aws.ToString(ls.ListenerArn)
		if listenerARN == "" {
			continue
		}
		sni, sniComplete, sniErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]elbv2types.Certificate, *string, error) {
			out, err := certAPI.DescribeListenerCertificates(ctx, &elbv2.DescribeListenerCertificatesInput{ListenerArn: &listenerARN, Marker: marker})
			if err != nil {
				return nil, nil, err
			}
			return out.Certificates, out.NextMarker, nil
		})
		if sniErr != nil {
			failures = append(failures, FailedCall(listenerARN, sniErr))
			continue
		}
		complete = complete && sniComplete
		add(sni)
	}
	if aggErr := AggregateFailures("elb-related: DescribeListenerCertificates", failures, len(listeners)); aggErr != nil && len(ids) == 0 {
		// Nothing was read at all: the failures establish nothing about how
		// many certificates the listeners carry, only that the attempt failed.
		return ReadFailed("acm", aggErr)
	}
	// A listener whose certificate list could not be read may serve one this
	// count does not name, so what was read is a lower bound rather than a
	// dead end.
	return relatedResultTrunc("acm", ids, !complete || len(failures) > 0)
}

// checkELBCF reports CloudFront distributions using this ELB as an origin.
// CloudFront Origins' DomainName may reference this ELB's DNS name.
func checkELBCF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dnsName := res.Fields["dns_name"]
	if dnsName == "" {
		return keyMissing("cf", "dnsName")
	}

	cfList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cf")
	if err != nil {
		return ReadFailed("cf", err)
	}
	if cfList == nil {
		return NotRead("cf")
	}

	var ids []string
	for _, cfRes := range cfList {
		dist, ok := assertStruct[cftypes.DistributionSummary](cfRes.RawStruct)
		if !ok || dist.Origins == nil {
			continue
		}
		if slices.ContainsFunc(dist.Origins.Items, func(o cftypes.Origin) bool {
			return dnsAliasNames(aws.ToString(o.DomainName), dnsName)
		}) {
			ids = append(ids, cfRes.ID)
		}
	}
	return relatedResultTrunc("cf", ids, truncated)
}

// checkELBENI reports ENIs owned by this ELB. ELB-owned ENIs have
// RequesterId "amazon-elb" and Description "ELB app/<name>/<hash>".
// Scans the eni cache for matching descriptions.
func checkELBENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	lbName := res.Fields["name"]
	if lbName == "" {
		lbName = res.Name
	}
	if lbName == "" {
		return keyMissing("eni", "lbName")
	}

	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("eni", err)
	}
	if eniList == nil {
		return NotRead("eni")
	}

	var ids []string
	for _, eniRes := range eniList {
		raw, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if raw.RequesterId == nil || *raw.RequesterId != "amazon-elb" {
			continue
		}
		if raw.Description == nil {
			continue
		}
		if elbNameFromENIDescription(*raw.Description) == lbName {
			ids = append(ids, eniRes.ID)
		}
	}
	return relatedResultTrunc("eni", ids, truncated)
}

// checkELBS3 reports the S3 bucket receiving ELB access logs, read from
// DescribeLoadBalancerAttributes: "access_logs.s3.bucket", which "is required
// if access logs are enabled" and so can outlive them, counts only while
// "access_logs.s3.enabled" is true
// (https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancerAttribute.html).
func checkELBS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return keyMissing("s3", "elbARN")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return NotRead("s3")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
		return c.ELBv2.DescribeLoadBalancerAttributes(ctx, &elbv2.DescribeLoadBalancerAttributesInput{LoadBalancerArn: &elbARN})
	})
	if err != nil {
		return ReadFailed("s3", err)
	}
	attrs := map[string]string{}
	for _, a := range out.Attributes {
		attrs[aws.ToString(a.Key)] = aws.ToString(a.Value)
	}
	if attrs["access_logs.s3.enabled"] != "true" {
		return foundNone("s3", "access_logs.s3.enabled")
	}
	return relatedResultTrunc("s3", nonEmpty(attrs["access_logs.s3.bucket"]), false)
}

// checkELBSubnet extracts subnet IDs from the LB's AvailabilityZones slice.
func checkELBSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	var ids []string
	seen := make(map[string]bool)
	for _, az := range raw.AvailabilityZones {
		if az.SubnetId == nil || *az.SubnetId == "" {
			continue
		}
		if seen[*az.SubnetId] {
			continue
		}
		seen[*az.SubnetId] = true
		ids = append(ids, *az.SubnetId)
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkELBWAF reports the WAF Web ACL attached to this ELB.
// Pattern C: one wafv2:GetWebACLForResource call with the ELB ARN.
//
// AWS WAFv2 only supports CloudFront, ALB, API Gateway, AppSync, Cognito,
// Verified Access, and App Runner — NLBs and GWLBs are unsupported and
// calling GetWebACLForResource with a non-ALB ELB returns
// WAFInvalidParameterException. Short-circuit when the ELB type is a known
// unsupported value (network or gateway) to keep the related-panel result a
// definitive zero instead of a fetch failure. Fall through to the API call
// for "application" and unknown types so ALBs with missing Fields["type"]
// (e.g. cache rehydration paths) still get checked via RawStruct fallback.
func checkELBWAF(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return keyMissing("waf", "elbARN")
	}
	lbType := res.Fields["type"]
	if lbType == "" {
		if raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct); ok {
			lbType = string(raw.Type)
		}
	}
	if lbType == "network" || lbType == "gateway" {
		return foundNone("waf", "lbType")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.WAFv2 == nil {
		return NotRead("waf")
	}
	api, ok := c.WAFv2.(WAFv2GetWebACLForResourceAPI)
	if !ok {
		return NotRead("waf")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2.GetWebACLForResourceOutput, error) {
		return api.GetWebACLForResource(ctx, &wafv2.GetWebACLForResourceInput{ResourceArn: &elbARN})
	})
	if err != nil {
		return ReadFailed("waf", err)
	}
	if out.WebACL == nil {
		return foundNone("waf", "out.WebACL")
	}
	id := ""
	if out.WebACL.Id != nil {
		id = *out.WebACL.Id
	}
	if id == "" && out.WebACL.ARN != nil {
		id = *out.WebACL.ARN
	}
	if id == "" {
		return foundNone("waf", "id")
	}
	return relatedResultTrunc("waf", []string{id}, false)
}
