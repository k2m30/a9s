// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// elb_issue_enrichment.go — Wave 2 issue enrichment for the elb resource type.
package aws

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/catalog"
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
// type ("application" / "network"), returning the finding code and the
// Attention row, or ok=false when the listener is fine. The phrase is the
// caller's: both codes name every offending port of the balancer at once, so
// no single listener knows what it will say.
//
// An ALB listener speaking plain HTTP exposes traffic unless its default
// action redirects to HTTPS; an NLB listener speaking TCP on 443 is a TLS
// port with no TLS termination, which is the same exposure a level down.
func elbListenerExposure(lbType string, listener elbtypes.Listener) (domain.FindingCode, domain.DetailRow, bool) {
	port := int32(0)
	if listener.Port != nil {
		port = *listener.Port
	}
	protocol := string(listener.Protocol)
	switch listener.Protocol {
	case elbtypes.ProtocolEnumHttps, elbtypes.ProtocolEnumTls:
		if isWeakTLSPolicy(aws.ToString(listener.SslPolicy)) {
			return elbCodeWeakTLSPolicy,
				domain.DetailRow{Label: "Security policy", Value: fmt.Sprintf("%d: %s", port, aws.ToString(listener.SslPolicy)), Tier: "~"}, true
		}
	case elbtypes.ProtocolEnumHttp:
		if lbType == "application" && !redirectsToHTTPS(listener) {
			return elbCodePlainHTTPListener,
				domain.DetailRow{Label: "Listener", Value: fmt.Sprintf("%d/%s", port, protocol), Tier: "~"}, true
		}
	case elbtypes.ProtocolEnumTcp:
		if lbType == "network" && port == 443 {
			return elbCodePlainHTTPListener,
				domain.DetailRow{Label: "Listener", Value: fmt.Sprintf("%d/%s", port, protocol), Tier: "~"}, true
		}
	}
	return "", domain.DetailRow{}, false
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
		isALB := r.Fields["type"] == "application"
		for _, attr := range out.Attributes {
			if attr.Key == nil || attr.Value == nil {
				continue
			}
			switch *attr.Key {
			case "deletion_protection.enabled":
				if *attr.Value == "false" {
					rows = append(rows, domain.DetailRow{Label: "Deletion Protection", Value: "disabled", Tier: "~"})
				}
			case "access_logs.s3.enabled":
				if *attr.Value == "false" {
					rows = append(rows, domain.DetailRow{Label: "Access Logs", Value: "disabled", Tier: "~"})
				}
			case "routing.http.desync_mitigation_mode":
				if isALB && *attr.Value == elbDesyncMonitorMode {
					setWave2Finding(&result, r.ID, elbCodeDesyncMitigationOff, "HTTP desync mitigation off", "~", "elb",
						[]domain.DetailRow{{Label: "Desync mitigation", Value: elbDesyncMonitorMode, Tier: "~"}})

				}
			case "routing.http.drop_invalid_header_fields.enabled":
				if isALB && *attr.Value != "true" {
					setWave2Finding(&result, r.ID, elbCodeInvalidHeadersKept, "invalid HTTP headers not dropped", "~", "elb",
						[]domain.DetailRow{{Label: "Drop invalid headers", Value: "disabled", Tier: "~"}})

				}
			}
		}
		if len(rows) > 0 {
			setWave2Finding(&result, r.ID, elbCodeMisconfigured, catalog.Phrase(elbCodeMisconfigured), "~", "elb", rows)
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
		listeners, err := allELBListeners(ctx, clients.ELBv2, lbARN)
		mu.Lock()
		defer mu.Unlock()
		total++
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, "DescribeListeners", err)
			return
		}
		// Every offending port is a port to fix, so each finding names all of
		// them rather than sending the operator back to the console for the
		// rest of a list only the balancer knows. DescribeListeners does not
		// promise an order, so the ports are sorted numerically and the rows
		// follow them: an unchanged balancer must render the same cell twice.
		offenders := map[domain.FindingCode][]elbOffendingListener{}
		for _, listener := range listeners {
			code, row, bad := elbListenerExposure(r.Fields["type"], listener)
			if !bad {
				continue
			}
			offenders[code] = append(offenders[code],
				elbOffendingListener{port: aws.ToInt32(listener.Port), row: row})
		}
		if ports, rows := elbOffendersInPortOrder(offenders[elbCodePlainHTTPListener]); ports != "" {
			setWave2Finding(&result, r.ID, elbCodePlainHTTPListener,
				elbCount(rows, "port ", "ports ")+ports+" in the clear", "~", "elb", rows)

		}
		if ports, rows := elbOffendersInPortOrder(offenders[elbCodeWeakTLSPolicy]); ports != "" {
			setWave2Finding(&result, r.ID, elbCodeWeakTLSPolicy,
				"weak TLS policy on "+elbCount(rows, "port ", "ports ")+ports, "~", "elb", rows)

		}
	})
	sort.Strings(failures)
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result, AggregateFailures("elb-enrich: DescribeLoadBalancerAttributes/DescribeListeners", failures, total)
}

// allELBListeners reads a balancer's listeners to the end. DescribeListeners
// pages, and a cleartext listener on the second page is as exposed as one on
// the first; the walk is bounded by PerParentPageCap like every other
// per-parent sweep.
func allELBListeners(ctx context.Context, api ELBv2DescribeListenersAPI, lbARN string) ([]elbtypes.Listener, error) {
	var out []elbtypes.Listener
	token := ""
	for range PerParentPageCap {
		page, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (resource.FetchResult, error) {
			return FetchELBListeners(ctx, api, map[string]string{"load_balancer_arn": lbARN}, token)
		})
		if err != nil {
			return nil, err
		}
		for _, lr := range page.Resources {
			if listener, ok := assertStruct[elbtypes.Listener](lr.RawStruct); ok {
				out = append(out, listener)
			}
		}
		if page.Pagination == nil || page.Pagination.NextToken == "" {
			break
		}
		token = page.Pagination.NextToken
	}
	return out, nil
}

// elbOffendingListener pairs a listener's port with the row that describes it,
// so sorting one keeps the other alongside.
type elbOffendingListener struct {
	port int32
	row  domain.DetailRow
}

// elbOffendersInPortOrder renders the joined port list and the rows in the
// same numeric order. An empty port list means there is nothing to report.
func elbOffendersInPortOrder(offenders []elbOffendingListener) (string, []domain.DetailRow) {
	if len(offenders) == 0 {
		return "", nil
	}
	slices.SortFunc(offenders, func(a, b elbOffendingListener) int { return cmp.Compare(a.port, b.port) })
	ports := make([]string, 0, len(offenders))
	rows := make([]domain.DetailRow, 0, len(offenders))
	for _, o := range offenders {
		ports = append(ports, strconv.Itoa(int(o.port)))
		rows = append(rows, o.row)
	}
	return strings.Join(ports, ", "), rows
}

// elbCount picks the singular wording when exactly one listener offends — one
// port under a plural heading reads as a list that got truncated. There is one
// row per offending listener, so the rows are the count.
func elbCount(rows []domain.DetailRow, one, many string) string {
	if len(rows) == 1 {
		return one
	}
	return many
}
