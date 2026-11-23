// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// elb_issue_enrichment.go — Wave 2 issue enrichment for the elb resource type.
package aws

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
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

// elbDesyncMonitorMode is the desync mitigation mode that only observes.
// AWS defaults to "defensive"; "strictest" is stricter still.
const elbDesyncMonitorMode = "monitor"

// elbPolicyProtocols reads the protocols AWS states for every predefined
// security policy (DescribeSSLPolicies, which answers every policy when no
// name is given), keyed by policy name. It is the one source of the weak-TLS
// verdict: a policy is judged by what it negotiates, never by its name. A nil
// api reads nothing.
func elbPolicyProtocols(ctx context.Context, api ELBv2DescribeSSLPoliciesAPI) (map[string][]string, error) {
	if api == nil {
		return nil, nil
	}
	policies, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]elbtypes.SslPolicy, *string, error) {
		out, err := api.DescribeSSLPolicies(ctx, &elasticloadbalancingv2.DescribeSSLPoliciesInput{Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		return out.SslPolicies, out.NextMarker, nil
	})
	if err == nil && !complete {
		err = errPageCap
	}
	if err != nil {
		return nil, err
	}
	protocols := make(map[string][]string, len(policies))
	for _, p := range policies {
		protocols[aws.ToString(p.Name)] = p.SslProtocols
	}
	return protocols, nil
}

// elbProtocolsBelowTLS12 are the protocol names DescribeSSLPolicies lists for
// versions older than TLS 1.2.
var elbProtocolsBelowTLS12 = []string{"TLSv1", "TLSv1.1"} //nolint:gochecknoglobals // AWS protocol names

// elbListenerLogged reports whether access logs record the listener's
// traffic: an Application Load Balancer logs the HTTP and HTTPS requests it
// serves, and a Network Load Balancer's access logs "are created only if the
// load balancer has a TLS listener" (ELB Network Load Balancers guide, "Access
// logs").
func elbListenerLogged(l elbtypes.Listener) bool {
	switch l.Protocol {
	case elbtypes.ProtocolEnumHttp, elbtypes.ProtocolEnumHttps, elbtypes.ProtocolEnumTls:
		return true
	}
	return false
}

// elbCheckPolicyUnknown is what a balancer records when a listener names a
// security policy DescribeSSLPolicies did not return.
const elbCheckPolicyUnknown = "security policy not returned by DescribeSSLPolicies"

// elbListenerExposure classifies one listener of a load balancer for
// cleartext exposure, returning the finding code and the Attention row, or
// ok=false when the listener is fine.
// The phrase is the caller's: the code names every offending port of the
// balancer at once, so no single listener knows what it will say. The
// security policy of an HTTPS or TLS listener is judged from the protocols
// elbPolicyProtocols reads, through elbBelowTLS12.
//
// A listener carrying traffic in the clear exposes it unless its default
// action redirects to HTTPS. The balancer type is not consulted: an
// Application Load Balancer speaks only HTTP and HTTPS and a Network one only
// TCP, TLS and the datagram protocols, so the listener protocol already says
// which kind of balancer this is.
func elbListenerExposure(listener elbtypes.Listener) (domain.FindingCode, domain.DetailRow, bool) {
	port := int32(0)
	if listener.Port != nil {
		port = *listener.Port
	}
	protocol := string(listener.Protocol)
	switch listener.Protocol {
	case elbtypes.ProtocolEnumHttp, elbtypes.ProtocolEnumTcp:
		if ELBListenerIsPlaintext(listener.Protocol, port) && !redirectsToHTTPS(listener) {
			return elbCodePlainHTTPListener,
				domain.DetailRow{Label: "Listener", Value: fmt.Sprintf("%d/%s", port, protocol), Tier: "~"}, true
		}
	}
	return "", domain.DetailRow{}, false
}

// elbListenerSecurityPolicy returns the security policy an HTTPS or TLS
// listener terminates with, or "" for a listener that has none to judge.
func elbListenerSecurityPolicy(listener elbtypes.Listener) string {
	if listener.Protocol != elbtypes.ProtocolEnumHttps && listener.Protocol != elbtypes.ProtocolEnumTls {
		return ""
	}
	return aws.ToString(listener.SslPolicy)
}

// elbBelowTLS12 reports whether a protocol name DescribeSSLPolicies lists is a
// version older than TLS 1.2.
func elbBelowTLS12(protocol string) bool { return slices.Contains(elbProtocolsBelowTLS12, protocol) }

// ELBListenerIsPlaintext reports whether a listener carries traffic no one
// encrypted. HTTP does. TCP is a byte pipe, so what it carries depends on the
// port: a TCP listener on 443 is TLS passthrough — the session terminates on
// the target rather than on the balancer — while one on a conventionally
// plaintext port is clear traffic. TLS and HTTPS terminate on the balancer
// and are never plaintext.
//
// The load balancer type is not a parameter: an Application Load Balancer
// speaks only HTTP and HTTPS, a Network one only TCP, TLS, UDP and the rest,
// so the protocol already says which kind of balancer it is on.
func ELBListenerIsPlaintext(protocol elbtypes.ProtocolEnum, port int32) bool {
	switch protocol {
	case elbtypes.ProtocolEnumHttp:
		return true
	case elbtypes.ProtocolEnumTcp:
		return port == 80 || port == 8080
	default:
		return false
	}
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
// would paint every freshly-created, unhardened LB red.
//
// Per-LB API failures aggregate into a composite error returned alongside
// the partial findings. LoadBalancerArn is read from
// r.Fields["load_balancer_arn"] — the elb fetcher emits ID = bare LB name
// and stores the ARN in Fields.
func EnrichELBAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.ELBv2 == nil {
		return result, nil
	}
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, func(r resource.Resource) bool {
		return r.Fields["load_balancer_arn"] != ""
	}, resourceIDsOf)
	ids := resourceIDs(resources)
	policiesAPI, _ := clients.ELBv2.(ELBv2DescribeSSLPoliciesAPI)
	// The policies are region-wide and matter only to a listener that
	// terminates TLS, so they are read when the first such one is found.
	policyProtocols := sync.OnceValues(func() (map[string][]string, error) {
		return elbPolicyProtocols(ctx, policiesAPI)
	})
	// Access logs record what a listener of a logged protocol carries, so the
	// access-log setting is judged once the listeners are read.
	accessLogsOff := map[string]bool{}
	var mu sync.Mutex
	attrErr := ForEachRow(ctx, &result, ids, EnrichmentParallelism, func(i int) {
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
		out, err := clients.ELBv2.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
			LoadBalancerArn: aws.String(lbARN),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		var rows []domain.DetailRow
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
				accessLogsOff[r.ID] = *attr.Value == "false"
			case "routing.http.desync_mitigation_mode":
				if *attr.Value == elbDesyncMonitorMode {
					setWave2Finding(&result, r.ID, elbCodeDesyncMitigationOff, []domain.DetailRow{{Label: "Desync mitigation", Value: elbDesyncMonitorMode, Tier: tierOf(elbCodeDesyncMitigationOff)}})

				}
			case "routing.http.drop_invalid_header_fields.enabled":
				if *attr.Value != "true" {
					setWave2Finding(&result, r.ID, elbCodeInvalidHeadersKept, []domain.DetailRow{{Label: "Drop invalid headers", Value: "disabled", Tier: tierOf(elbCodeInvalidHeadersKept)}})

				}
			}
		}
		if len(rows) > 0 {
			setWave2Finding(&result, r.ID, elbCodeMisconfigured, rows)
		}
	})
	// Listener posture needs a second read per load balancer, so it runs as
	// its own pass rather than serialising behind the attributes call.
	listenerErr := ForEachRow(ctx, &result, ids, EnrichmentParallelism, func(i int) {
		r := resources[i]
		lbARN := r.Fields["load_balancer_arn"]
		if r.ID == "" || lbARN == "" {
			return
		}
		listeners, err := allELBListeners(ctx, clients.ELBv2, lbARN)
		var protocols map[string][]string
		var policiesErr error
		if policiesAPI != nil && slices.ContainsFunc(listeners, func(l elbtypes.Listener) bool { return elbListenerSecurityPolicy(l) != "" }) {
			protocols, policiesErr = policyProtocols()
		}
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if policiesErr != nil {
			MarkSkipped(&result, r.ID, &failures, policiesErr)
		}
		if accessLogsOff[r.ID] && slices.ContainsFunc(listeners, elbListenerLogged) {
			setWave2Finding(&result, r.ID, elbCodeMisconfigured, []domain.DetailRow{{Label: "Access Logs", Value: "disabled", Tier: "~"}})
		}
		// Every offending port is a port to fix, so each finding names all of
		// them rather than sending the operator back to the console for the
		// rest of a list only the balancer knows. DescribeListeners does not
		// promise an order, so the ports are sorted numerically and the rows
		// follow them: an unchanged balancer must render the same cell twice.
		offenders := map[domain.FindingCode][]elbOffendingListener{}
		for _, listener := range listeners {
			if policy := elbListenerSecurityPolicy(listener); policy != "" && policiesAPI != nil && policiesErr == nil {
				stated, known := protocols[policy]
				switch {
				case !known:
					markUninspected(&result, r.ID, elbCheckPolicyUnknown)
				case slices.ContainsFunc(stated, elbBelowTLS12):
					port := aws.ToInt32(listener.Port)
					offenders[elbCodeWeakTLSPolicy] = append(offenders[elbCodeWeakTLSPolicy], elbOffendingListener{port: port,
						row: domain.DetailRow{Label: "Security policy", Value: fmt.Sprintf("%d: %s", port, policy), Tier: "~"}})
				}
			}
			code, row, bad := elbListenerExposure(listener)
			if !bad {
				continue
			}
			offenders[code] = append(offenders[code],
				elbOffendingListener{port: aws.ToInt32(listener.Port), row: row})
		}
		if ports, rows := elbOffendersInPortOrder(offenders[elbCodePlainHTTPListener]); ports != "" {
			setWave2Finding(&result, r.ID, elbCodePlainHTTPListener, rows, ports)

		}
		if ports, rows := elbOffendersInPortOrder(offenders[elbCodeWeakTLSPolicy]); ports != "" {
			setWave2Finding(&result, r.ID, elbCodeWeakTLSPolicy, rows, ports)

		}
	})

	return result, errors.Join(attrErr, listenerErr, AggregateFailures("load balancer attributes and listeners", failures, total))
}

// allELBListeners reads a balancer's listeners to the end. DescribeListeners
// pages, and a cleartext listener on the second page is as exposed as one on
// the first; the walk is bounded by PerParentPageCap like every other
// per-parent sweep.
func allELBListeners(ctx context.Context, api ELBv2DescribeListenersAPI, lbARN string) ([]elbtypes.Listener, error) {
	listeners, _, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]elbtypes.Listener, *string, error) {
		page, err := FetchELBListeners(ctx, api, map[string]string{"load_balancer_arn": lbARN}, aws.ToString(token))
		if err != nil {
			return nil, nil, err
		}
		var out []elbtypes.Listener
		for _, lr := range page.Resources {
			if listener, ok := assertStruct[elbtypes.Listener](lr.RawStruct); ok {
				out = append(out, listener)
			}
		}
		if page.Pagination == nil {
			return out, nil, nil
		}
		return out, &page.Pagination.NextToken, nil
	})
	if err != nil {
		return nil, err
	}
	return listeners, nil
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
