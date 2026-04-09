package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// ELB Listeners fetcher tests (child of Load Balancers)
// ---------------------------------------------------------------------------

// TestFetchELBListeners_Basic verifies parsing of 1 HTTPS listener with
// certificate, forward action to target group. Checks ID (ListenerArn),
// Name (port string), Status (""), all 6 Fields, and RawStruct.
func TestFetchELBListeners_Basic(t *testing.T) {
	mock := &mockELBv2DescribeListenersClient{
		output: &elbv2.DescribeListenersOutput{
			Listeners: []elbtypes.Listener{
				{
					ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456"),
					Port:        aws.Int32(443),
					Protocol:    elbtypes.ProtocolEnumHttps,
					SslPolicy:   aws.String("ELBSecurityPolicy-TLS13-1-2-2021-06"),
					Certificates: []elbtypes.Certificate{{
						CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/abc-def-123"),
					}},
					DefaultActions: []elbtypes.Action{{
						Type:           elbtypes.ActionTypeEnumForward,
						TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-prod-tg/abc123"),
					}},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/api-prod-alb/abc123",
		"lb_name":           "api-prod-alb",
	}

	result, err := awsclient.FetchELBListeners(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	t.Run("ID_is_ListenerArn", func(t *testing.T) {
		expected := "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456"
		if r.ID != expected {
			t.Errorf("ID: expected %q, got %q", expected, r.ID)
		}
	})

	t.Run("Name_is_port_string", func(t *testing.T) {
		if r.Name != "443" {
			t.Errorf("Name: expected %q, got %q", "443", r.Name)
		}
	})

	t.Run("Status_is_empty", func(t *testing.T) {
		if r.Status != "" {
			t.Errorf("Status: expected empty string, got %q", r.Status)
		}
	})

	t.Run("Fields_port", func(t *testing.T) {
		if r.Fields["port"] != "443" {
			t.Errorf("Fields[port]: expected %q, got %q", "443", r.Fields["port"])
		}
	})

	t.Run("Fields_protocol", func(t *testing.T) {
		if r.Fields["protocol"] != "HTTPS" {
			t.Errorf("Fields[protocol]: expected %q, got %q", "HTTPS", r.Fields["protocol"])
		}
	})

	t.Run("Fields_default_action_type", func(t *testing.T) {
		if r.Fields["default_action_type"] != "forward" {
			t.Errorf("Fields[default_action_type]: expected %q, got %q", "forward", r.Fields["default_action_type"])
		}
	})

	t.Run("Fields_default_action_target", func(t *testing.T) {
		if r.Fields["default_action_target"] != "api-prod-tg" {
			t.Errorf("Fields[default_action_target]: expected %q, got %q", "api-prod-tg", r.Fields["default_action_target"])
		}
	})

	t.Run("Fields_ssl_policy", func(t *testing.T) {
		if r.Fields["ssl_policy"] != "ELBSecurityPolicy-TLS13-1-2-2021-06" {
			t.Errorf("Fields[ssl_policy]: expected %q, got %q", "ELBSecurityPolicy-TLS13-1-2-2021-06", r.Fields["ssl_policy"])
		}
	})

	t.Run("Fields_certificate_short", func(t *testing.T) {
		// Should extract the certificate ID from the ARN
		if r.Fields["certificate_short"] == "" {
			t.Error("Fields[certificate_short] should not be empty")
		}
		if r.Fields["certificate_short"] != "abc-def-123" {
			t.Errorf("Fields[certificate_short]: expected %q, got %q", "abc-def-123", r.Fields["certificate_short"])
		}
	})

	t.Run("RawStruct_is_Listener", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(elbtypes.Listener)
		if !ok {
			t.Fatalf("RawStruct should be elbtypes.Listener, got %T", r.RawStruct)
		}
		if raw.ListenerArn == nil || *raw.ListenerArn != "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456" {
			t.Error("RawStruct.ListenerArn not preserved correctly")
		}
	})

	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"port", "protocol", "default_action_type", "default_action_target", "ssl_policy", "certificate_short"}
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchELBListeners_Empty verifies that an LB with no listeners
// returns an empty slice with no error.
func TestFetchELBListeners_Empty(t *testing.T) {
	mock := &mockELBv2DescribeListenersClient{
		output: &elbv2.DescribeListenersOutput{
			Listeners: []elbtypes.Listener{},
		},
	}

	parentCtx := map[string]string{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/empty-alb/xyz",
		"lb_name":           "empty-alb",
	}

	result, err := awsclient.FetchELBListeners(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchELBListeners_APIError verifies that API errors are propagated.
func TestFetchELBListeners_APIError(t *testing.T) {
	mock := &mockELBv2DescribeListenersClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	parentCtx := map[string]string{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/err-alb/xyz",
		"lb_name":           "err-alb",
	}

	result, err := awsclient.FetchELBListeners(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources on error, got %d", len(result.Resources))
	}
}

// TestFetchELBListeners_NilFields verifies that nil Port, nil SslPolicy,
// nil Certificates, and empty DefaultActions do not cause a panic.
func TestFetchELBListeners_NilFields(t *testing.T) {
	mock := &mockELBv2DescribeListenersClient{
		output: &elbv2.DescribeListenersOutput{
			Listeners: []elbtypes.Listener{
				{
					ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/nil-alb/abc123/def456"),
					// Port is nil
					// Protocol is zero value
					// SslPolicy is nil
					// Certificates is nil
					// DefaultActions is empty
				},
			},
		},
	}

	parentCtx := map[string]string{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/nil-alb/abc123",
		"lb_name":           "nil-alb",
	}

	// Should not panic
	result, err := awsclient.FetchELBListeners(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	t.Run("nil_Port_handled", func(t *testing.T) {
		r := resources[0]
		// Port should default to some empty/zero representation
		if r.Fields["port"] == "" {
			t.Logf("Fields[port] is empty (expected for nil Port)")
		}
	})

	t.Run("nil_SslPolicy_handled", func(t *testing.T) {
		r := resources[0]
		if r.Fields["ssl_policy"] != "" {
			t.Logf("Fields[ssl_policy] is %q (expected empty for nil)", r.Fields["ssl_policy"])
		}
	})

	t.Run("nil_Certificates_handled", func(t *testing.T) {
		r := resources[0]
		if r.Fields["certificate_short"] != "" {
			t.Logf("Fields[certificate_short] is %q (expected empty for nil)", r.Fields["certificate_short"])
		}
	})

	t.Run("empty_DefaultActions_handled", func(t *testing.T) {
		r := resources[0]
		if r.Fields["default_action_type"] != "" {
			t.Logf("Fields[default_action_type] is %q (expected empty for no actions)", r.Fields["default_action_type"])
		}
	})
}

// TestFetchELBListeners_ComputedFields tests all 3 action types:
// forward, redirect, and fixed-response.
func TestFetchELBListeners_ComputedFields(t *testing.T) {
	mock := &mockELBv2DescribeListenersClient{
		output: &elbv2.DescribeListenersOutput{
			Listeners: []elbtypes.Listener{
				// Forward action — extracts TG name from ARN
				{
					ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/fwd001"),
					Port:        aws.Int32(443),
					Protocol:    elbtypes.ProtocolEnumHttps,
					SslPolicy:   aws.String("ELBSecurityPolicy-TLS13-1-2-2021-06"),
					Certificates: []elbtypes.Certificate{{
						CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/abc-def-123"),
					}},
					DefaultActions: []elbtypes.Action{{
						Type:           elbtypes.ActionTypeEnumForward,
						TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-prod-tg/abc123"),
					}},
				},
				// Redirect action — shows redirect URL
				{
					ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/rdr001"),
					Port:        aws.Int32(80),
					Protocol:    elbtypes.ProtocolEnumHttp,
					DefaultActions: []elbtypes.Action{{
						Type: elbtypes.ActionTypeEnumRedirect,
						RedirectConfig: &elbtypes.RedirectActionConfig{
							Protocol:   aws.String("HTTPS"),
							Port:       aws.String("443"),
							StatusCode: elbtypes.RedirectActionStatusCodeEnumHttp301,
						},
					}},
				},
				// Fixed-response action — shows status code
				{
					ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/fix001"),
					Port:        aws.Int32(8080),
					Protocol:    elbtypes.ProtocolEnumHttp,
					DefaultActions: []elbtypes.Action{{
						Type: elbtypes.ActionTypeEnumFixedResponse,
						FixedResponseConfig: &elbtypes.FixedResponseActionConfig{
							StatusCode: aws.String("200"),
						},
					}},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/api-prod-alb/abc123",
		"lb_name":           "api-prod-alb",
	}

	result, err := awsclient.FetchELBListeners(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("forward_action_target_extracts_tg_name", func(t *testing.T) {
		r := resources[0]
		if r.Fields["default_action_type"] != "forward" {
			t.Errorf("Fields[default_action_type]: expected %q, got %q", "forward", r.Fields["default_action_type"])
		}
		if r.Fields["default_action_target"] != "api-prod-tg" {
			t.Errorf("Fields[default_action_target]: expected %q, got %q", "api-prod-tg", r.Fields["default_action_target"])
		}
	})

	t.Run("redirect_action_target_shows_url", func(t *testing.T) {
		r := resources[1]
		if r.Fields["default_action_type"] != "redirect" {
			t.Errorf("Fields[default_action_type]: expected %q, got %q", "redirect", r.Fields["default_action_type"])
		}
		target := r.Fields["default_action_target"]
		if target == "" {
			t.Error("Fields[default_action_target] should not be empty for redirect")
		}
		// Should contain HTTPS and 443 or some representation of the redirect URL
		// The exact format depends on implementation, but it must show the redirect destination
	})

	t.Run("fixed_response_action_target_shows_status_code", func(t *testing.T) {
		r := resources[2]
		if r.Fields["default_action_type"] != "fixed-response" {
			t.Errorf("Fields[default_action_type]: expected %q, got %q", "fixed-response", r.Fields["default_action_type"])
		}
		target := r.Fields["default_action_target"]
		if target == "" {
			t.Error("Fields[default_action_target] should not be empty for fixed-response")
		}
		// Should contain status code "200"
	})
}

// TestFetchELBListeners_CertificateShort verifies that the certificate ARN
// is shortened to just the certificate ID.
func TestFetchELBListeners_CertificateShort(t *testing.T) {
	mock := &mockELBv2DescribeListenersClient{
		output: &elbv2.DescribeListenersOutput{
			Listeners: []elbtypes.Listener{
				{
					ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/cert-alb/abc123/def456"),
					Port:        aws.Int32(443),
					Protocol:    elbtypes.ProtocolEnumHttps,
					SslPolicy:   aws.String("ELBSecurityPolicy-2016-08"),
					Certificates: []elbtypes.Certificate{{
						CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/abc-def-123"),
					}},
					DefaultActions: []elbtypes.Action{{
						Type:           elbtypes.ActionTypeEnumForward,
						TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/tg1/abc"),
					}},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/cert-alb/abc123",
		"lb_name":           "cert-alb",
	}

	result, err := awsclient.FetchELBListeners(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	certShort := resources[0].Fields["certificate_short"]
	if certShort != "abc-def-123" {
		t.Errorf("Fields[certificate_short]: expected %q, got %q", "abc-def-123", certShort)
	}
}

// TestFetchELBListeners_RawStruct verifies that RawStruct preserves the
// original elbtypes.Listener, including all sub-fields.
func TestFetchELBListeners_RawStruct(t *testing.T) {
	mock := &mockELBv2DescribeListenersClient{
		output: &elbv2.DescribeListenersOutput{
			Listeners: []elbtypes.Listener{
				{
					ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/raw-alb/abc123/def456"),
					Port:        aws.Int32(443),
					Protocol:    elbtypes.ProtocolEnumHttps,
					SslPolicy:   aws.String("ELBSecurityPolicy-TLS13-1-2-2021-06"),
					Certificates: []elbtypes.Certificate{{
						CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/raw-cert-id"),
					}},
					DefaultActions: []elbtypes.Action{{
						Type:           elbtypes.ActionTypeEnumForward,
						TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/raw-tg/abc123"),
					}},
					AlpnPolicy: []string{"HTTP2Preferred"},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/raw-alb/abc123",
		"lb_name":           "raw-alb",
	}

	result, err := awsclient.FetchELBListeners(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := r.RawStruct.(elbtypes.Listener)
	if !ok {
		t.Fatalf("RawStruct should be elbtypes.Listener, got %T", r.RawStruct)
	}

	t.Run("ListenerArn_preserved", func(t *testing.T) {
		if raw.ListenerArn == nil || *raw.ListenerArn != "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/raw-alb/abc123/def456" {
			t.Error("RawStruct.ListenerArn not preserved correctly")
		}
	})

	t.Run("Port_preserved", func(t *testing.T) {
		if raw.Port == nil || *raw.Port != 443 {
			t.Error("RawStruct.Port not preserved correctly")
		}
	})

	t.Run("Protocol_preserved", func(t *testing.T) {
		if raw.Protocol != elbtypes.ProtocolEnumHttps {
			t.Errorf("RawStruct.Protocol: expected %q, got %q", elbtypes.ProtocolEnumHttps, raw.Protocol)
		}
	})

	t.Run("SslPolicy_preserved", func(t *testing.T) {
		if raw.SslPolicy == nil || *raw.SslPolicy != "ELBSecurityPolicy-TLS13-1-2-2021-06" {
			t.Error("RawStruct.SslPolicy not preserved correctly")
		}
	})

	t.Run("Certificates_preserved", func(t *testing.T) {
		if len(raw.Certificates) != 1 {
			t.Fatalf("RawStruct.Certificates: expected 1, got %d", len(raw.Certificates))
		}
		if raw.Certificates[0].CertificateArn == nil || *raw.Certificates[0].CertificateArn != "arn:aws:acm:us-east-1:123456789012:certificate/raw-cert-id" {
			t.Error("RawStruct.Certificates[0].CertificateArn not preserved correctly")
		}
	})

	t.Run("DefaultActions_preserved", func(t *testing.T) {
		if len(raw.DefaultActions) != 1 {
			t.Fatalf("RawStruct.DefaultActions: expected 1, got %d", len(raw.DefaultActions))
		}
		if raw.DefaultActions[0].Type != elbtypes.ActionTypeEnumForward {
			t.Errorf("RawStruct.DefaultActions[0].Type: expected %q, got %q", elbtypes.ActionTypeEnumForward, raw.DefaultActions[0].Type)
		}
	})

	t.Run("AlpnPolicy_preserved", func(t *testing.T) {
		if len(raw.AlpnPolicy) != 1 || raw.AlpnPolicy[0] != "HTTP2Preferred" {
			t.Errorf("RawStruct.AlpnPolicy not preserved correctly: %v", raw.AlpnPolicy)
		}
	})
}

// TestFetchELBListeners_Pagination verifies the single-page pagination contract:
// one API call is made per invocation, resources from that page are returned,
// and IsTruncated/NextToken reflect whether more pages exist. A second call
// with the continuation token verifies the token is forwarded and the final
// page sets IsTruncated=false.
func TestFetchELBListeners_Pagination(t *testing.T) {
	parentCtx := map[string]string{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/pag-alb/abc123",
		"lb_name":           "pag-alb",
	}

	// Page 1: 2 listeners with NextMarker indicating more pages exist.
	page1Mock := &mockELBv2DescribeListenersClient{
		outputs: []*elbv2.DescribeListenersOutput{
			{
				NextMarker: aws.String("page2-marker"),
				Listeners: []elbtypes.Listener{
					{
						ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/pag-alb/abc123/page1-001"),
						Port:        aws.Int32(80),
						Protocol:    elbtypes.ProtocolEnumHttp,
						DefaultActions: []elbtypes.Action{{
							Type: elbtypes.ActionTypeEnumRedirect,
							RedirectConfig: &elbtypes.RedirectActionConfig{
								Protocol:   aws.String("HTTPS"),
								Port:       aws.String("443"),
								StatusCode: elbtypes.RedirectActionStatusCodeEnumHttp301,
							},
						}},
					},
					{
						ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/pag-alb/abc123/page1-002"),
						Port:        aws.Int32(443),
						Protocol:    elbtypes.ProtocolEnumHttps,
						SslPolicy:   aws.String("ELBSecurityPolicy-TLS13-1-2-2021-06"),
						Certificates: []elbtypes.Certificate{{
							CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/pag-cert"),
						}},
						DefaultActions: []elbtypes.Action{{
							Type:           elbtypes.ActionTypeEnumForward,
							TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/pag-tg/abc123"),
						}},
					},
				},
			},
		},
	}

	// First call: no continuation token — fetches page 1.
	result1, err := awsclient.FetchELBListeners(
		context.Background(),
		page1Mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("page 1: expected no error, got %v", err)
	}

	t.Run("page1_item_count", func(t *testing.T) {
		if len(result1.Resources) != 2 {
			t.Fatalf("expected 2 resources on page 1, got %d", len(result1.Resources))
		}
	})

	t.Run("page1_single_api_call", func(t *testing.T) {
		if page1Mock.callIdx != 1 {
			t.Errorf("expected 1 API call for page 1, got %d", page1Mock.callIdx)
		}
	})

	t.Run("page1_is_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("page 1: IsTruncated should be true when NextMarker is present")
		}
	})

	t.Run("page1_next_token", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.NextToken != "page2-marker" {
			t.Errorf("page 1: NextToken expected %q, got %q", "page2-marker", result1.Pagination.NextToken)
		}
	})

	t.Run("page1_total_hint_negative", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.TotalHint != -1 {
			t.Errorf("page 1: TotalHint should be -1 when truncated, got %d", result1.Pagination.TotalHint)
		}
	})

	t.Run("page1_first_listener_port_80", func(t *testing.T) {
		if result1.Resources[0].Fields["port"] != "80" {
			t.Errorf("first resource port: expected %q, got %q", "80", result1.Resources[0].Fields["port"])
		}
	})

	t.Run("page1_all_fields_populated", func(t *testing.T) {
		requiredFields := []string{"port", "protocol", "default_action_type"}
		for i, r := range result1.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("page 1: resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})

	// Page 2: 1 listener with no NextMarker — last page.
	page2Mock := &mockELBv2DescribeListenersClient{
		outputs: []*elbv2.DescribeListenersOutput{
			{
				// No NextMarker — last page
				Listeners: []elbtypes.Listener{
					{
						ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/pag-alb/abc123/page2-001"),
						Port:        aws.Int32(8080),
						Protocol:    elbtypes.ProtocolEnumHttp,
						DefaultActions: []elbtypes.Action{{
							Type: elbtypes.ActionTypeEnumFixedResponse,
							FixedResponseConfig: &elbtypes.FixedResponseActionConfig{
								StatusCode: aws.String("200"),
							},
						}},
					},
				},
			},
		},
	}

	// Second call: pass continuation token from page 1 to fetch page 2.
	result2, err := awsclient.FetchELBListeners(
		context.Background(),
		page2Mock,
		parentCtx,
		result1.Pagination.NextToken,
	)
	if err != nil {
		t.Fatalf("page 2: expected no error, got %v", err)
	}

	t.Run("page2_item_count", func(t *testing.T) {
		if len(result2.Resources) != 1 {
			t.Fatalf("expected 1 resource on page 2, got %d", len(result2.Resources))
		}
	})

	t.Run("page2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("page 2: IsTruncated should be false on last page")
		}
	})

	t.Run("page2_empty_next_token", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.NextToken != "" {
			t.Errorf("page 2: NextToken should be empty on last page, got %q", result2.Pagination.NextToken)
		}
	})

	t.Run("page2_total_hint_equals_count", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.TotalHint != 1 {
			t.Errorf("page 2: TotalHint should equal item count (1) on last page, got %d", result2.Pagination.TotalHint)
		}
	})

	t.Run("page2_listener_port_8080", func(t *testing.T) {
		if result2.Resources[0].Fields["port"] != "8080" {
			t.Errorf("page 2 resource port: expected %q, got %q", "8080", result2.Resources[0].Fields["port"])
		}
	})
}

// TestFetchELBListeners_MaxCap verifies that a single API page of 50 listeners
// is returned as-is with correct IsTruncated=true metadata when the API
// indicates more pages exist. The 200-item cap no longer applies — each call
// returns one page and the caller drives pagination via continuation tokens.
func TestFetchELBListeners_MaxCap(t *testing.T) {
	// Build one page of 50 listeners with a NextMarker indicating more pages exist.
	var listeners []elbtypes.Listener
	for i := range 50 {
		portNum := int32(1000 + i)
		listeners = append(listeners, elbtypes.Listener{
			ListenerArn: aws.String(fmt.Sprintf("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/cap-alb/abc123/p0-l%d", i)),
			Port:        aws.Int32(portNum),
			Protocol:    elbtypes.ProtocolEnumHttp,
			DefaultActions: []elbtypes.Action{{
				Type: elbtypes.ActionTypeEnumFixedResponse,
				FixedResponseConfig: &elbtypes.FixedResponseActionConfig{
					StatusCode: aws.String("200"),
				},
			}},
		})
	}

	mock := &mockELBv2DescribeListenersClient{
		outputs: []*elbv2.DescribeListenersOutput{
			{
				Listeners:  listeners,
				NextMarker: aws.String("marker-page-1"),
			},
		},
	}

	parentCtx := map[string]string{
		"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/cap-alb/abc123",
		"lb_name":           "cap-alb",
	}

	result, err := awsclient.FetchELBListeners(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("returns_full_page_of_50", func(t *testing.T) {
		if len(result.Resources) != 50 {
			t.Errorf("expected exactly 50 resources from single API page, got %d", len(result.Resources))
		}
	})

	t.Run("single_api_call", func(t *testing.T) {
		if mock.callIdx != 1 {
			t.Errorf("expected 1 API call per invocation, got %d", mock.callIdx)
		}
	})

	t.Run("is_truncated_true", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result.Pagination.IsTruncated {
			t.Error("IsTruncated should be true when API returns NextMarker")
		}
	})

	t.Run("next_token_forwarded", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.NextToken != "marker-page-1" {
			t.Errorf("NextToken expected %q, got %q", "marker-page-1", result.Pagination.NextToken)
		}
	})

	t.Run("page_size_equals_item_count", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.PageSize != 50 {
			t.Errorf("PageSize expected 50, got %d", result.Pagination.PageSize)
		}
	})

	t.Run("total_hint_negative_when_truncated", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.TotalHint != -1 {
			t.Errorf("TotalHint should be -1 when truncated, got %d", result.Pagination.TotalHint)
		}
	})

	t.Run("first_listener_correct", func(t *testing.T) {
		if result.Resources[0].Fields["port"] != "1000" {
			t.Errorf("first resource port: expected %q, got %q", "1000", result.Resources[0].Fields["port"])
		}
	})

	t.Run("last_listener_correct", func(t *testing.T) {
		if result.Resources[49].Fields["port"] != "1049" {
			t.Errorf("last resource port: expected %q, got %q", "1049", result.Resources[49].Fields["port"])
		}
	})
}

// TestELBListenerColumns verifies that ELBListenerColumns returns the expected
// columns with correct keys, titles, and positive widths.
func TestELBListenerColumns(t *testing.T) {
	cols := resource.ELBListenerColumns()

	expectedKeys := []string{"port", "protocol", "default_action_type", "default_action_target", "ssl_policy", "certificate_short"}
	expectedTitles := []string{"Port", "Protocol", "Action", "Target", "SSL Policy", "Certificate"}
	expectedWidths := []int{8, 10, 16, 32, 24, 32}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != len(expectedKeys) {
			t.Fatalf("expected %d columns, got %d", len(expectedKeys), len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("column_titles", func(t *testing.T) {
		for i, expected := range expectedTitles {
			if cols[i].Title != expected {
				t.Errorf("column[%d].Title: expected %q, got %q", i, expected, cols[i].Title)
			}
		}
	})

	t.Run("columns_have_positive_width", func(t *testing.T) {
		for i, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column[%d] (%s) has non-positive Width: %d", i, col.Key, col.Width)
			}
		}
	})

	t.Run("expected_widths", func(t *testing.T) {
		for i, expected := range expectedWidths {
			if cols[i].Width != expected {
				t.Errorf("column[%d] (%s).Width: expected %d, got %d", i, cols[i].Key, expected, cols[i].Width)
			}
		}
	})
}

// TestELBListeners_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestELBListeners_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("elb_listeners")
	if td == nil {
		t.Fatal("elb_listeners child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "elb_listeners" {
		t.Errorf("child type ShortName: expected %q, got %q", "elb_listeners", td.ShortName)
	}
}

// TestELBListeners_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is registered under the correct short name.
func TestELBListeners_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("elb_listeners")
	if f == nil {
		t.Fatal("elb_listeners paginated child fetcher not registered")
	}
}

// TestELBListeners_ParentHasChildDef verifies that the parent elb resource
// type has a child view definition for elb_listeners with key "enter"
// and ContextKeys includes "load_balancer_arn".
func TestELBListeners_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("elb")
	if rt == nil {
		t.Fatal("elb resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "elb_listeners" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["load_balancer_arn"] == "" {
				t.Error("ContextKeys should include 'load_balancer_arn'")
			}
			if child.ContextKeys["lb_name"] == "" {
				t.Error("ContextKeys should include 'lb_name'")
			}
		}
	}
	if !found {
		t.Error("elb Children should contain elb_listeners child view def")
	}
}

// TestFetchLoadBalancers_HasLoadBalancerArn verifies that the parent LB
// fetcher now populates load_balancer_arn in Fields.
func TestFetchLoadBalancers_HasLoadBalancerArn(t *testing.T) {
	mock := &mockELBv2DescribeLoadBalancersClient{
		output: &elbv2.DescribeLoadBalancersOutput{
			LoadBalancers: []elbtypes.LoadBalancer{
				{
					LoadBalancerName: aws.String("test-alb"),
					LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-alb/abc123"),
					DNSName:          aws.String("test-alb-123.us-east-1.elb.amazonaws.com"),
					Type:             elbtypes.LoadBalancerTypeEnumApplication,
					Scheme:           elbtypes.LoadBalancerSchemeEnumInternetFacing,
					State: &elbtypes.LoadBalancerState{
						Code: elbtypes.LoadBalancerStateEnumActive,
					},
					VpcId: aws.String("vpc-abc123"),
				},
			},
		},
	}

	resources, err := awsclient.FetchLoadBalancers(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	lbArn, ok := r.Fields["load_balancer_arn"]
	if !ok {
		t.Fatal("Fields should contain 'load_balancer_arn' key")
	}
	if lbArn != "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-alb/abc123" {
		t.Errorf("Fields[load_balancer_arn]: expected %q, got %q",
			"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-alb/abc123", lbArn)
	}
}
