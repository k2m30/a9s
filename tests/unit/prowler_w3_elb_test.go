package unit

// prowler_w3_elb_test.go — the four load-balancer posture signals added to
// EnrichELBAttributes.
//
// Two come from the attributes map already fetched (desync mitigation left in
// monitor mode, invalid header fields not dropped) and two from a
// DescribeListeners call per load balancer (a listener terminating plaintext,
// a listener pinned to a retired TLS policy). All four are independent: a
// balancer with several problems reports several findings, each with its own
// code, phrase and supporting rows, and a balancer whose listeners could not
// be read is marked unknown rather than silently reported healthy.

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w3CodeELBDesyncOff      = domain.FindingCode("elb.desync-mitigation-off")
	w3CodeELBInvalidHeaders = domain.FindingCode("elb.invalid-headers-kept")
	w3CodeELBPlainHTTP      = domain.FindingCode("elb.plain-http-listener")
	w3CodeELBWeakTLS        = domain.FindingCode("elb.weak-tls-policy")
	w3CodeELBMisconfigured  = domain.FindingCode("elb.misconfigured")

	w3PhraseELBDesyncOff      = "HTTP desync mitigation off"
	w3PhraseELBInvalidHeaders = "invalid HTTP headers not dropped"
)

// w3ELBArn builds the ARN shape the elb fetcher stores in
// Fields["load_balancer_arn"].
func w3ELBArn(name, kind string) string {
	return "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/" + kind + "/" + name + "/50dc6c495c0c9188"
}

// w3ELBRes mirrors the elb fetcher: ID is the bare name, the ARN and the
// balancer type live in Fields.
func w3ELBRes(name, lbType string) resource.Resource {
	kind := "app"
	if lbType == "network" {
		kind = "net"
	}
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"name":              name,
			"type":              lbType,
			"scheme":            "internet-facing",
			"state":             "active",
			"vpc_id":            "vpc-0abc123",
			"dns_name":          name + "-1234567890.us-east-1.elb.amazonaws.com",
			"load_balancer_arn": w3ELBArn(name, kind),
		},
	}
}

// w3ELBAttrs is the healthy attribute set an ALB returns once hardened; the
// caller overrides the one key under test so no unrelated finding fires.
func w3ELBAttrs(overrides map[string]string) []elbtypes.LoadBalancerAttribute {
	base := map[string]string{
		"deletion_protection.enabled":                     "true",
		"access_logs.s3.enabled":                          "true",
		"routing.http.desync_mitigation_mode":             "defensive",
		"routing.http.drop_invalid_header_fields.enabled": "true",
		"idle_timeout.timeout_seconds":                    "60",
		"routing.http2.enabled":                           "true",
	}
	for k, v := range overrides {
		base[k] = v
	}
	out := make([]elbtypes.LoadBalancerAttribute, 0, len(base))
	for k, v := range base {
		out = append(out, elbtypes.LoadBalancerAttribute{Key: aws.String(k), Value: aws.String(v)})
	}
	return out
}

// w3Listener builds a realistic DescribeListeners entry.
func w3Listener(arnSuffix string, port int32, proto elbtypes.ProtocolEnum, sslPolicy string, actions []elbtypes.Action) elbtypes.Listener {
	l := elbtypes.Listener{
		ListenerArn:    aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme/50dc6c495c0c9188/" + arnSuffix),
		Port:           aws.Int32(port),
		Protocol:       proto,
		DefaultActions: actions,
	}
	if sslPolicy != "" {
		l.SslPolicy = aws.String(sslPolicy)
		l.Certificates = []elbtypes.Certificate{{
			CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/abc-def-123"),
			IsDefault:      aws.Bool(true),
		}}
	}
	return l
}

func w3ForwardAction() []elbtypes.Action {
	return []elbtypes.Action{{
		Type:           elbtypes.ActionTypeEnumForward,
		Order:          aws.Int32(1),
		TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-tg/73e2d6bc24d8a067"),
	}}
}

// w3RedirectToHTTPSAction is the standard "port 80 exists only to bounce you
// to 443" default action — a plaintext listener that terminates nothing.
func w3RedirectToHTTPSAction() []elbtypes.Action {
	return []elbtypes.Action{{
		Type:  elbtypes.ActionTypeEnumRedirect,
		Order: aws.Int32(1),
		RedirectConfig: &elbtypes.RedirectActionConfig{
			Protocol:   aws.String("HTTPS"),
			Port:       aws.String("443"),
			Host:       aws.String("#{host}"),
			Path:       aws.String("/#{path}"),
			Query:      aws.String("#{query}"),
			StatusCode: elbtypes.RedirectActionStatusCodeEnumHttp301,
		},
	}}
}

// w3ELBv2Fake answers per-ARN. attrErr / listenerErr inject a per-balancer
// failure so a partial answer can be told apart from a healthy one.
type w3ELBv2Fake struct {
	awsclient.ELBv2API

	mu           sync.Mutex
	attrs        map[string][]elbtypes.LoadBalancerAttribute
	listeners    map[string][]elbtypes.Listener
	attrErr      map[string]error
	listenerErr  map[string]error
	listenerHits map[string]int
}

func (f *w3ELBv2Fake) DescribeLoadBalancerAttributes(_ context.Context, in *elbv2.DescribeLoadBalancerAttributesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
	arn := aws.ToString(in.LoadBalancerArn)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.attrErr[arn]; err != nil {
		return nil, err
	}
	return &elbv2.DescribeLoadBalancerAttributesOutput{Attributes: f.attrs[arn]}, nil
}

func (f *w3ELBv2Fake) DescribeListeners(_ context.Context, in *elbv2.DescribeListenersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	arn := aws.ToString(in.LoadBalancerArn)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listenerHits[arn]++
	if err := f.listenerErr[arn]; err != nil {
		return nil, err
	}
	return &elbv2.DescribeListenersOutput{Listeners: f.listeners[arn]}, nil
}

func w3NewELBFake() *w3ELBv2Fake {
	return &w3ELBv2Fake{
		attrs:        map[string][]elbtypes.LoadBalancerAttribute{},
		listeners:    map[string][]elbtypes.Listener{},
		attrErr:      map[string]error{},
		listenerErr:  map[string]error{},
		listenerHits: map[string]int{},
	}
}

func (f *w3ELBv2Fake) hits(arn string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listenerHits[arn]
}

// w3RunELBEnrich drives the real enricher over the given resources.
func w3RunELBEnrich(t *testing.T, fake *w3ELBv2Fake, resources ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichELBAttributes(context.Background(),
		&awsclient.ServiceClients{ELBv2: fake}, resources, nil)
	if err != nil {
		t.Logf("enricher returned composite error (expected only when a call was made to fail): %v", err)
	}
	if res.Findings == nil || res.TruncatedIDs == nil {
		t.Fatal("Findings and TruncatedIDs must be non-nil")
	}
	return res
}

// ── Row 5: desync mitigation left in monitor mode ─────────────────────────

func TestW3ELBDesync_MonitorModeFlagged(t *testing.T) {
	r := w3ELBRes("acme-public-alb", "application")
	fake := w3NewELBFake()
	fake.attrs[r.Fields["load_balancer_arn"]] = w3ELBAttrs(map[string]string{
		"routing.http.desync_mitigation_mode": "monitor",
	})

	res := w3RunELBEnrich(t, fake, r)
	f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBDesyncOff)
	if !ok {
		t.Fatalf("desync_mitigation_mode=monitor produced no %s; findings=%+v", w3CodeELBDesyncOff, res.Findings[r.ID])
	}
	if f.Phrase != w3PhraseELBDesyncOff {
		t.Errorf("Phrase = %q, want %q", f.Phrase, w3PhraseELBDesyncOff)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	if f.Source != "wave2:elb" {
		t.Errorf("Source = %q, want %q", f.Source, "wave2:elb")
	}
	if f.Detail == "" {
		t.Error("Detail is empty; every finding carries an operator sentence")
	}
	w3AssertRows(t, res.AttentionDetails[r.ID][w3CodeELBDesyncOff].Rows, [][2]string{
		{"Desync mitigation", "monitor"},
	})
}

func TestW3ELBDesync_SafeModesHealthy(t *testing.T) {
	for _, mode := range []string{"defensive", "strictest"} {
		t.Run(mode, func(t *testing.T) {
			r := w3ELBRes("acme-public-alb", "application")
			fake := w3NewELBFake()
			fake.attrs[r.Fields["load_balancer_arn"]] = w3ELBAttrs(map[string]string{
				"routing.http.desync_mitigation_mode": mode,
			})
			res := w3RunELBEnrich(t, fake, r)
			if f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBDesyncOff); ok {
				t.Errorf("mode %q produced %s (%q), want no finding", mode, f.Code, f.Phrase)
			}
		})
	}
}

// ── Row 6: invalid header fields not dropped ──────────────────────────────

func TestW3ELBInvalidHeaders_Flagged(t *testing.T) {
	r := w3ELBRes("acme-public-alb", "application")
	fake := w3NewELBFake()
	fake.attrs[r.Fields["load_balancer_arn"]] = w3ELBAttrs(map[string]string{
		"routing.http.drop_invalid_header_fields.enabled": "false",
	})

	res := w3RunELBEnrich(t, fake, r)
	f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBInvalidHeaders)
	if !ok {
		t.Fatalf("drop_invalid_header_fields disabled produced no %s; findings=%+v", w3CodeELBInvalidHeaders, res.Findings[r.ID])
	}
	if f.Phrase != w3PhraseELBInvalidHeaders {
		t.Errorf("Phrase = %q, want %q", f.Phrase, w3PhraseELBInvalidHeaders)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	if f.Source != "wave2:elb" {
		t.Errorf("Source = %q, want %q", f.Source, "wave2:elb")
	}
	if f.Detail == "" {
		t.Error("Detail is empty; every finding carries an operator sentence")
	}
	w3AssertRows(t, res.AttentionDetails[r.ID][w3CodeELBInvalidHeaders].Rows, [][2]string{
		{"Drop invalid headers", "disabled"},
	})
}

func TestW3ELBInvalidHeaders_EnabledIsHealthy(t *testing.T) {
	r := w3ELBRes("acme-public-alb", "application")
	fake := w3NewELBFake()
	fake.attrs[r.Fields["load_balancer_arn"]] = w3ELBAttrs(nil)

	res := w3RunELBEnrich(t, fake, r)
	if len(res.Findings[r.ID]) != 0 {
		t.Errorf("fully hardened ALB produced %+v, want no findings", res.Findings[r.ID])
	}
}

// TestW3ELBAttributeRows_AreALBOnly pins that the two HTTP-routing attributes
// are not asserted against network load balancers, which never carry them.
func TestW3ELBAttributeRows_AreALBOnly(t *testing.T) {
	r := w3ELBRes("acme-public-nlb", "network")
	fake := w3NewELBFake()
	// NLB attributes: no routing.http.* keys at all.
	fake.attrs[r.Fields["load_balancer_arn"]] = []elbtypes.LoadBalancerAttribute{
		{Key: aws.String("deletion_protection.enabled"), Value: aws.String("true")},
		{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("true")},
		{Key: aws.String("load_balancing.cross_zone.enabled"), Value: aws.String("true")},
	}
	fake.listeners[r.Fields["load_balancer_arn"]] = []elbtypes.Listener{
		w3Listener("tls", 443, elbtypes.ProtocolEnumTls, "ELBSecurityPolicy-TLS13-1-2-2021-06", w3ForwardAction()),
	}

	res := w3RunELBEnrich(t, fake, r)
	for _, code := range []domain.FindingCode{w3CodeELBDesyncOff, w3CodeELBInvalidHeaders} {
		if f, ok := w3FindingByCode(res.Findings[r.ID], code); ok {
			t.Errorf("network load balancer carries ALB-only %s (%q)", f.Code, f.Phrase)
		}
	}
}

// TestW3ELBAttributes_TwoConditionsTwoFindings pins independence: the
// existing elb.misconfigured emission must not swallow the new ones, and the
// two new attribute conditions must not collapse into one.
func TestW3ELBAttributes_TwoConditionsTwoFindings(t *testing.T) {
	r := w3ELBRes("acme-public-alb", "application")
	fake := w3NewELBFake()
	fake.attrs[r.Fields["load_balancer_arn"]] = w3ELBAttrs(map[string]string{
		"routing.http.desync_mitigation_mode":             "monitor",
		"routing.http.drop_invalid_header_fields.enabled": "false",
		"deletion_protection.enabled":                     "false",
	})

	res := w3RunELBEnrich(t, fake, r)
	for _, code := range []domain.FindingCode{w3CodeELBDesyncOff, w3CodeELBInvalidHeaders, w3CodeELBMisconfigured} {
		if _, ok := w3FindingByCode(res.Findings[r.ID], code); !ok {
			t.Errorf("missing %s; findings=%+v", code, res.Findings[r.ID])
		}
	}
	// Each finding keeps its own supporting rows.
	if len(res.AttentionDetails[r.ID]) < 3 {
		t.Errorf("AttentionDetails = %+v, want one entry per emitted finding code", res.AttentionDetails[r.ID])
	}
}

// ── Row 7: listener terminating plaintext ─────────────────────────────────

func TestW3ELBPlainHTTP_ALBHTTPListenerFlagged(t *testing.T) {
	r := w3ELBRes("acme-public-alb", "application")
	fake := w3NewELBFake()
	arn := r.Fields["load_balancer_arn"]
	fake.attrs[arn] = w3ELBAttrs(nil)
	fake.listeners[arn] = []elbtypes.Listener{
		w3Listener("http", 80, elbtypes.ProtocolEnumHttp, "", w3ForwardAction()),
	}

	res := w3RunELBEnrich(t, fake, r)
	f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBPlainHTTP)
	if !ok {
		t.Fatalf("HTTP listener forwarding to a target group produced no %s; findings=%+v", w3CodeELBPlainHTTP, res.Findings[r.ID])
	}
	if want := "listener without TLS on port 80"; f.Phrase != want {
		t.Errorf("Phrase = %q, want %q", f.Phrase, want)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	if f.Source != "wave2:elb" {
		t.Errorf("Source = %q, want %q", f.Source, "wave2:elb")
	}
	if f.Detail == "" {
		t.Error("Detail is empty; every finding carries an operator sentence")
	}
	w3AssertRows(t, res.AttentionDetails[r.ID][w3CodeELBPlainHTTP].Rows, [][2]string{
		{"Listener", "80/HTTP"},
	})
	if got := fake.hits(arn); got != 1 {
		t.Errorf("DescribeListeners called %d times for one balancer, want 1", got)
	}
}

// TestW3ELBPlainHTTP_RedirectToHTTPSIsHealthy is the negative half: a port-80
// listener whose only job is a 301 to HTTPS terminates no traffic in the
// clear and is the recommended configuration.
func TestW3ELBPlainHTTP_RedirectToHTTPSIsHealthy(t *testing.T) {
	r := w3ELBRes("acme-public-alb", "application")
	fake := w3NewELBFake()
	arn := r.Fields["load_balancer_arn"]
	fake.attrs[arn] = w3ELBAttrs(nil)
	fake.listeners[arn] = []elbtypes.Listener{
		w3Listener("http", 80, elbtypes.ProtocolEnumHttp, "", w3RedirectToHTTPSAction()),
		w3Listener("https", 443, elbtypes.ProtocolEnumHttps, "ELBSecurityPolicy-TLS13-1-2-2021-06", w3ForwardAction()),
	}

	res := w3RunELBEnrich(t, fake, r)
	if len(res.Findings[r.ID]) != 0 {
		t.Errorf("redirect-only HTTP listener plus a modern HTTPS listener produced %+v, want no findings", res.Findings[r.ID])
	}
}

// TestW3ELBPlainHTTP_NLBPort443WithoutTLS pins the network-balancer half: a
// TCP listener on 443 passes encrypted-looking traffic straight through with
// no TLS termination or policy of its own.
func TestW3ELBPlainHTTP_NLBPort443WithoutTLS(t *testing.T) {
	r := w3ELBRes("acme-public-nlb", "network")
	fake := w3NewELBFake()
	arn := r.Fields["load_balancer_arn"]
	fake.attrs[arn] = []elbtypes.LoadBalancerAttribute{
		{Key: aws.String("deletion_protection.enabled"), Value: aws.String("true")},
		{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("true")},
	}
	fake.listeners[arn] = []elbtypes.Listener{
		w3Listener("tcp443", 443, elbtypes.ProtocolEnumTcp, "", w3ForwardAction()),
	}

	res := w3RunELBEnrich(t, fake, r)
	f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBPlainHTTP)
	if !ok {
		t.Fatalf("NLB TCP listener on 443 produced no %s; findings=%+v", w3CodeELBPlainHTTP, res.Findings[r.ID])
	}
	if want := "listener without TLS on port 443"; f.Phrase != want {
		t.Errorf("Phrase = %q, want %q", f.Phrase, want)
	}
	w3AssertRows(t, res.AttentionDetails[r.ID][w3CodeELBPlainHTTP].Rows, [][2]string{
		{"Listener", "443/TCP"},
	})
}

// TestW3ELBPlainHTTP_NLBPlainTCPPortIsHealthy pins the boundary: TCP
// pass-through on a non-TLS port is what an NLB is for.
func TestW3ELBPlainHTTP_NLBPlainTCPPortIsHealthy(t *testing.T) {
	r := w3ELBRes("acme-public-nlb", "network")
	fake := w3NewELBFake()
	arn := r.Fields["load_balancer_arn"]
	fake.attrs[arn] = []elbtypes.LoadBalancerAttribute{
		{Key: aws.String("deletion_protection.enabled"), Value: aws.String("true")},
		{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("true")},
	}
	fake.listeners[arn] = []elbtypes.Listener{
		w3Listener("tcp5432", 5432, elbtypes.ProtocolEnumTcp, "", w3ForwardAction()),
	}

	res := w3RunELBEnrich(t, fake, r)
	if f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBPlainHTTP); ok {
		t.Errorf("TCP listener on 5432 produced %s (%q), want no finding", f.Code, f.Phrase)
	}
}

// ── Row 8: retired TLS policy ─────────────────────────────────────────────

func TestW3ELBWeakTLS_RetiredPoliciesFlagged(t *testing.T) {
	for _, policy := range []string{
		"ELBSecurityPolicy-2015-05",
		"ELBSecurityPolicy-2016-08",
		"ELBSecurityPolicy-TLS-1-0-2015-04",
		"ELBSecurityPolicy-TLS-1-1-2017-01",
		"ELBSecurityPolicy-FS-2018-06",
		"ELBSecurityPolicy-FS-1-1-2019-08",
	} {
		t.Run(policy, func(t *testing.T) {
			r := w3ELBRes("acme-public-alb", "application")
			fake := w3NewELBFake()
			arn := r.Fields["load_balancer_arn"]
			fake.attrs[arn] = w3ELBAttrs(nil)
			fake.listeners[arn] = []elbtypes.Listener{
				w3Listener("https", 443, elbtypes.ProtocolEnumHttps, policy, w3ForwardAction()),
			}

			res := w3RunELBEnrich(t, fake, r)
			f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBWeakTLS)
			if !ok {
				t.Fatalf("SslPolicy %q produced no %s; findings=%+v", policy, w3CodeELBWeakTLS, res.Findings[r.ID])
			}
			if want := "weak TLS policy on listener 443"; f.Phrase != want {
				t.Errorf("Phrase = %q, want %q", f.Phrase, want)
			}
			if f.Severity != domain.SevWarn {
				t.Errorf("Severity = %v, want SevWarn", f.Severity)
			}
			if f.Source != "wave2:elb" {
				t.Errorf("Source = %q, want %q", f.Source, "wave2:elb")
			}
			if f.Detail == "" {
				t.Error("Detail is empty; every finding carries an operator sentence")
			}
			w3AssertRows(t, res.AttentionDetails[r.ID][w3CodeELBWeakTLS].Rows, [][2]string{
				{"Security policy", policy},
			})
		})
	}
}

func TestW3ELBWeakTLS_ModernPoliciesHealthy(t *testing.T) {
	for _, policy := range []string{
		"ELBSecurityPolicy-TLS13-1-2-2021-06",
		"ELBSecurityPolicy-TLS13-1-2-Res-2021-06",
		"ELBSecurityPolicy-TLS-1-2-2017-01",
		"ELBSecurityPolicy-TLS-1-2-Ext-2018-06",
		"ELBSecurityPolicy-FS-1-2-Res-2020-10",
	} {
		t.Run(policy, func(t *testing.T) {
			r := w3ELBRes("acme-public-alb", "application")
			fake := w3NewELBFake()
			arn := r.Fields["load_balancer_arn"]
			fake.attrs[arn] = w3ELBAttrs(nil)
			fake.listeners[arn] = []elbtypes.Listener{
				w3Listener("https", 443, elbtypes.ProtocolEnumHttps, policy, w3ForwardAction()),
			}
			res := w3RunELBEnrich(t, fake, r)
			if f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBWeakTLS); ok {
				t.Errorf("SslPolicy %q produced %s (%q), want no finding", policy, f.Code, f.Phrase)
			}
		})
	}
}

// TestW3ELBWeakTLS_AppliesToNLBTLSListener pins that the policy check follows
// the protocol, not the balancer type — an NLB TLS listener carries an
// SslPolicy exactly like an ALB HTTPS one.
func TestW3ELBWeakTLS_AppliesToNLBTLSListener(t *testing.T) {
	r := w3ELBRes("acme-public-nlb", "network")
	fake := w3NewELBFake()
	arn := r.Fields["load_balancer_arn"]
	fake.attrs[arn] = []elbtypes.LoadBalancerAttribute{
		{Key: aws.String("deletion_protection.enabled"), Value: aws.String("true")},
		{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("true")},
	}
	fake.listeners[arn] = []elbtypes.Listener{
		w3Listener("tls", 8443, elbtypes.ProtocolEnumTls, "ELBSecurityPolicy-TLS-1-0-2015-04", w3ForwardAction()),
	}

	res := w3RunELBEnrich(t, fake, r)
	f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBWeakTLS)
	if !ok {
		t.Fatalf("NLB TLS listener on a retired policy produced no %s; findings=%+v", w3CodeELBWeakTLS, res.Findings[r.ID])
	}
	if want := "weak TLS policy on listener 8443"; f.Phrase != want {
		t.Errorf("Phrase = %q, want %q", f.Phrase, want)
	}
}

// ── Cross-cutting: independence, partial answers, cap ─────────────────────

// TestW3ELBListeners_TwoBadListenersTwoFindings pins that both listener
// conditions on one balancer survive as separate findings — a plaintext
// listener and a retired-policy listener are two different remediations.
func TestW3ELBListeners_TwoBadListenersTwoFindings(t *testing.T) {
	r := w3ELBRes("acme-public-alb", "application")
	fake := w3NewELBFake()
	arn := r.Fields["load_balancer_arn"]
	fake.attrs[arn] = w3ELBAttrs(nil)
	fake.listeners[arn] = []elbtypes.Listener{
		w3Listener("http", 80, elbtypes.ProtocolEnumHttp, "", w3ForwardAction()),
		w3Listener("https", 443, elbtypes.ProtocolEnumHttps, "ELBSecurityPolicy-2016-08", w3ForwardAction()),
	}

	res := w3RunELBEnrich(t, fake, r)
	for _, code := range []domain.FindingCode{w3CodeELBPlainHTTP, w3CodeELBWeakTLS} {
		if _, ok := w3FindingByCode(res.Findings[r.ID], code); !ok {
			t.Errorf("missing %s; findings=%+v", code, res.Findings[r.ID])
		}
	}
	if got := res.AttentionDetails[r.ID][w3CodeELBPlainHTTP].Rows; len(got) == 0 {
		t.Errorf("%s lost its supporting rows when a second finding landed", w3CodeELBPlainHTTP)
	}
	if got := res.AttentionDetails[r.ID][w3CodeELBWeakTLS].Rows; len(got) == 0 {
		t.Errorf("%s lost its supporting rows when a second finding landed", w3CodeELBWeakTLS)
	}
}

// TestW3ELBListeners_FailureMarksOnlyThatBalancer pins the partial-answer
// rule: a balancer whose listeners could not be read renders "?" rather than
// disappearing or being reported clean, and its neighbours are still judged.
func TestW3ELBListeners_FailureMarksOnlyThatBalancer(t *testing.T) {
	bad := w3ELBRes("acme-denied-alb", "application")
	good := w3ELBRes("acme-public-alb", "application")
	fake := w3NewELBFake()
	for _, r := range []resource.Resource{bad, good} {
		fake.attrs[r.Fields["load_balancer_arn"]] = w3ELBAttrs(nil)
	}
	fake.listenerErr[bad.Fields["load_balancer_arn"]] = errors.New("AccessDenied: not authorized to perform elasticloadbalancing:DescribeListeners")
	fake.listeners[good.Fields["load_balancer_arn"]] = []elbtypes.Listener{
		w3Listener("http", 80, elbtypes.ProtocolEnumHttp, "", w3ForwardAction()),
	}

	res := w3RunELBEnrich(t, fake, bad, good)

	if !res.TruncatedIDs[bad.ID] {
		t.Errorf("TruncatedIDs = %v, want %s marked so its row renders unknown rather than healthy", res.TruncatedIDs, bad.ID)
	}
	if _, ok := w3FindingByCode(res.Findings[bad.ID], w3CodeELBPlainHTTP); ok {
		t.Errorf("balancer whose listeners failed to load reported a listener finding: %+v", res.Findings[bad.ID])
	}
	if _, ok := w3FindingByCode(res.Findings[good.ID], w3CodeELBPlainHTTP); !ok {
		t.Errorf("one failed balancer suppressed the others; %s findings=%+v", good.ID, res.Findings[good.ID])
	}
	if res.TruncatedIDs[good.ID] {
		t.Errorf("healthy-path balancer %s was marked truncated", good.ID)
	}
}

// TestW3ELBEnrich_CapBoundsCallsWithoutTruncatingTheCount pins the "~"-only
// rule: the per-resource call cap limits how much informational coverage the
// enricher buys, but must never set Truncated, which would lower-bound the
// issue badge for warnings.
func TestW3ELBEnrich_CapBoundsCallsWithoutTruncatingTheCount(t *testing.T) {
	fake := w3NewELBFake()
	resources := make([]resource.Resource, 0, awsclient.EnrichmentCap+1)
	for i := range awsclient.EnrichmentCap + 1 {
		r := w3ELBRes("acme-alb-"+strconv.Itoa(i), "application")
		fake.attrs[r.Fields["load_balancer_arn"]] = w3ELBAttrs(map[string]string{
			"routing.http.desync_mitigation_mode": "monitor",
		})
		fake.listeners[r.Fields["load_balancer_arn"]] = []elbtypes.Listener{
			w3Listener("http", 80, elbtypes.ProtocolEnumHttp, "", w3ForwardAction()),
		}
		resources = append(resources, r)
	}

	res := w3RunELBEnrich(t, fake, resources...)

	if res.Truncated {
		t.Error("Truncated = true on a warn-only enricher; the cap bounds coverage, not the issue count")
	}
	inspected := 0
	for _, r := range resources {
		if len(res.Findings[r.ID]) > 0 {
			inspected++
		}
	}
	if inspected != awsclient.EnrichmentCap {
		t.Errorf("%d balancers carry findings, want exactly EnrichmentCap (%d)", inspected, awsclient.EnrichmentCap)
	}
	last := resources[awsclient.EnrichmentCap]
	if fake.hits(last.Fields["load_balancer_arn"]) != 0 {
		t.Errorf("balancer past the cap still triggered a DescribeListeners call")
	}
}

// TestW3ELBEnrich_NilClientReturnsEmpty pins that Wave 2 dispatched before the
// ELBv2 client exists yields an empty, non-nil result rather than panicking
// mid-refresh.
func TestW3ELBEnrich_NilClientReturnsEmpty(t *testing.T) {
	res, err := awsclient.EnrichELBAttributes(context.Background(),
		&awsclient.ServiceClients{}, []resource.Resource{w3ELBRes("acme-public-alb", "application")}, nil)
	if err != nil {
		t.Fatalf("EnrichELBAttributes: %v", err)
	}
	if len(res.Findings) != 0 || len(res.TruncatedIDs) != 0 {
		t.Errorf("got Findings=%+v TruncatedIDs=%+v, want both empty", res.Findings, res.TruncatedIDs)
	}
}
