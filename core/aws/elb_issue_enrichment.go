// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// elb_issue_enrichment.go — Wave 2 issue enrichment for the elb resource type.
package aws

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// elb canonical FindingCodes.
const (
	elbCodeMisconfigured       domain.FindingCode = "elb.misconfigured"
	elbCodeDesyncMitigationOff domain.FindingCode = "elb.desync-mitigation-off"
	elbCodeInvalidHeadersKept  domain.FindingCode = "elb.invalid-headers-kept"
	elbCodePlainHTTPListener   domain.FindingCode = "elb.plain-http-listener"
	elbCodeWeakTLSPolicy       domain.FindingCode = "elb.weak-tls-policy"
)

// S5 operator sentences for the elb Wave 2 findings.
const (
	elbDesyncMitigationOffDetail = "The load balancer forwards requests it knows are ambiguous instead of " +
		"rejecting them, so a crafted request can be interpreted one way by the balancer and another by the " +
		"target. Set the desync mitigation mode to defensive or strictest."
	elbInvalidHeadersKeptDetail = "Headers that are not valid HTTP are passed through to the targets instead of " +
		"being dropped, which is how request smuggling reaches an application. Turn on dropping of invalid " +
		"header fields."
	elbPlainHTTPListenerDetail = "This listener carries traffic in the clear, so credentials and session cookies " +
		"cross the network readable by anyone on the path. Terminate TLS on the listener, or redirect it to an " +
		"HTTPS listener."
	elbWeakTLSPolicyDetail = "The listener's security policy still negotiates older protocol versions or ciphers " +
		"without forward secrecy, so a client can be steered onto a breakable connection. Move the listener to one " +
		"of the modern security policies that require version 1.2 or later."
)

// elbDesyncMonitorMode is the desync mitigation mode that only observes.
// AWS defaults to "defensive"; "strictest" is stricter still.
const elbDesyncMonitorMode = "monitor"

// modernTLSPolicyPrefixes are the ELB security-policy families that negotiate
// TLS 1.2 or better with forward secrecy. Anything else — including every
// policy AWS has since superseded and any policy a future account carries —
// is reported as weak: a prefix rule cannot silently go stale the way an
// enumerated deny-list does.
var modernTLSPolicyPrefixes = []string{ //nolint:gochecknoglobals // static policy table
	"ELBSecurityPolicy-TLS13-",
	"ELBSecurityPolicy-TLS-1-2-",
	"ELBSecurityPolicy-FS-1-2-",
}

// isWeakTLSPolicy reports whether an ELB SSL policy name predates the modern
// families. An empty name is unknown, not weak.
func isWeakTLSPolicy(policy string) bool {
	if policy == "" {
		return false
	}
	return !slices.ContainsFunc(modernTLSPolicyPrefixes, func(prefix string) bool {
		return strings.HasPrefix(policy, prefix)
	})
}

// elbListenerExposure classifies one listener of a load balancer of the given
// type ("application" / "network"), returning the finding code, the phrase's
// variable part and the Attention row, or ok=false when the listener is fine.
//
// An ALB listener speaking plain HTTP exposes traffic unless its default
// action redirects to HTTPS; an NLB listener speaking TCP on 443 is a TLS
// port with no TLS termination, which is the same exposure a level down.
func elbListenerExposure(lbType string, listener elbtypes.Listener) (domain.FindingCode, string, domain.DetailRow, bool) {
	port := int32(0)
	if listener.Port != nil {
		port = *listener.Port
	}
	protocol := string(listener.Protocol)
	switch listener.Protocol {
	case elbtypes.ProtocolEnumHttps, elbtypes.ProtocolEnumTls:
		if isWeakTLSPolicy(aws.ToString(listener.SslPolicy)) {
			return elbCodeWeakTLSPolicy, fmt.Sprintf("weak TLS policy on listener %d", port),
				domain.DetailRow{Label: "Security policy", Value: aws.ToString(listener.SslPolicy), Tier: "~"}, true
		}
	case elbtypes.ProtocolEnumHttp:
		if lbType == "application" && !redirectsToHTTPS(listener) {
			return elbCodePlainHTTPListener, fmt.Sprintf("listener without TLS on port %d", port),
				domain.DetailRow{Label: "Listener", Value: fmt.Sprintf("%d/%s", port, protocol), Tier: "~"}, true
		}
	case elbtypes.ProtocolEnumTcp:
		if lbType == "network" && port == 443 {
			return elbCodePlainHTTPListener, fmt.Sprintf("listener without TLS on port %d", port),
				domain.DetailRow{Label: "Listener", Value: fmt.Sprintf("%d/%s", port, protocol), Tier: "~"}, true
		}
	}
	return "", "", domain.DetailRow{}, false
}

// redirectsToHTTPS reports whether every default action of the listener sends
// the client to HTTPS instead of serving the request in the clear.
func redirectsToHTTPS(listener elbtypes.Listener) bool {
	for _, action := range listener.DefaultActions {
		if action.Type != elbtypes.ActionTypeEnumRedirect || action.RedirectConfig == nil {
			return false
		}
		if !strings.EqualFold(aws.ToString(action.RedirectConfig.Protocol), "HTTPS") {
			return false
		}
	}
	return len(listener.DefaultActions) > 0
}

// EnrichELBAttributes calls DescribeLoadBalancerAttributes for each load
// balancer (1 per LB, cap 50) and returns a "~" (SevWarn) finding for each LB
// missing deletion protection or access logging — matching the single
// elbCodeMisconfigured FindingDef declared at SevWarn in
// catalog_networking.go. Both flags missing at once is the AWS
// create-load-balancer default and must not escalate to SevBroken; doing so
// previously painted every freshly-created, unhardened LB red.
//
// Per-LB API failures aggregate into a composite error returned alongside
// the partial findings (E1–E6 contract). LoadBalancerArn is read from
// r.Fields["load_balancer_arn"] — the elb fetcher emits ID = bare LB name
// and stores the ARN in Fields. Each call is wrapped in RetryOnThrottle.
func EnrichELBAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.ELBv2 == nil {
		return result, nil
	}
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.ID == "" {
			return
		}
		// DescribeLoadBalancerAttributes requires the LB ARN. The elb fetcher
		// (elb.go) sets ID = bare name and stores the ARN in
		// Fields["load_balancer_arn"]. Passing r.ID errors with ValidationError.
		lbARN := r.Fields["load_balancer_arn"]
		if lbARN == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticloadbalancingv2.DescribeLoadBalancerAttributesOutput, error) {
			return clients.ELBv2.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
				LoadBalancerArn: aws.String(lbARN),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			result.TruncatedIDs[r.ID] = true
			return
		}
		var rows []domain.DetailRow
		var phrases []string
		isALB := r.Fields["type"] == "application"
		for _, attr := range out.Attributes {
			if attr.Key == nil || attr.Value == nil {
				continue
			}
			switch *attr.Key {
			case "deletion_protection.enabled":
				if *attr.Value == "false" {
					rows = append(rows, domain.DetailRow{Label: "Deletion Protection", Value: "disabled", Tier: "~"})
					phrases = append(phrases, "deletion protection disabled")
				}
			case "access_logs.s3.enabled":
				if *attr.Value == "false" {
					rows = append(rows, domain.DetailRow{Label: "Access Logs", Value: "disabled", Tier: "~"})
					phrases = append(phrases, "access logs disabled")
				}
			case "routing.http.desync_mitigation_mode":
				if isALB && *attr.Value == elbDesyncMonitorMode {
					setWave2Finding(&result, r.ID, elbCodeDesyncMitigationOff, "HTTP desync mitigation off", "~", "elb",
						[]domain.DetailRow{{Label: "Desync mitigation", Value: elbDesyncMonitorMode, Tier: "~"}},
						elbDesyncMitigationOffDetail)
				}
			case "routing.http.drop_invalid_header_fields.enabled":
				if isALB && *attr.Value != "true" {
					setWave2Finding(&result, r.ID, elbCodeInvalidHeadersKept, "invalid HTTP headers not dropped", "~", "elb",
						[]domain.DetailRow{{Label: "Drop invalid headers", Value: "disabled", Tier: "~"}},
						elbInvalidHeadersKeptDetail)
				}
			}
		}
		if len(rows) > 0 {
			setWave2Finding(&result, r.ID, elbCodeMisconfigured, phrases[0], "~", "elb", rows, "")
		}
	})
	// Listener posture needs a second read per load balancer, so it runs as
	// its own pass rather than serialising behind the attributes call.
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		lbARN := r.Fields["load_balancer_arn"]
		if r.ID == "" || lbARN == "" {
			return
		}
		listeners, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (resource.FetchResult, error) {
			return FetchELBListeners(ctx, clients.ELBv2, map[string]string{"load_balancer_arn": lbARN}, "")
		})
		mu.Lock()
		defer mu.Unlock()
		total++
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, "DescribeListeners", err)
			return
		}
		seen := make(map[domain.FindingCode]bool, 2)
		for _, lr := range listeners.Resources {
			listener, ok := assertStruct[elbtypes.Listener](lr.RawStruct)
			if !ok {
				continue
			}
			code, phrase, row, bad := elbListenerExposure(r.Fields["type"], listener)
			if !bad || seen[code] {
				continue
			}
			seen[code] = true
			detail := elbPlainHTTPListenerDetail
			if code == elbCodeWeakTLSPolicy {
				detail = elbWeakTLSPolicyDetail
			}
			setWave2Finding(&result, r.ID, code, phrase, "~", "elb", []domain.DetailRow{row}, detail)
		}
	})
	sort.Strings(failures)
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result, AggregateFailures("elb-enrich: DescribeLoadBalancerAttributes/DescribeListeners", failures, total)
}
