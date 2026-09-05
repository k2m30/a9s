package unit

// prowler_w4_waf_test.go — behavioural test for the batch-w4 waf row: a Web
// ACL with no rules is attached and logging traffic but blocking nothing.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w4CodeWAFNoRules   domain.FindingCode = "waf.no-rules"
	w4PhraseWAFNoRules                    = "web ACL has no rules"
	w4SourceWAFWave2                      = "wave2:waf"
)

// w4WAFFake serves the WebACL calls the waf enricher makes. Logging and
// associations are healthy by default so the rules condition is what the
// assertions read.
type w4WAFFake struct {
	awsclient.WAFv2API
	// rules maps WebACL name → its rule list.
	rules map[string][]wafv2types.Rule
	// getACLErr maps WebACL name → error from GetWebACL.
	getACLErr map[string]error
}

func (f *w4WAFFake) GetLoggingConfiguration(
	_ context.Context, in *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options),
) (*wafv2.GetLoggingConfigurationOutput, error) {
	return &wafv2.GetLoggingConfigurationOutput{
		LoggingConfiguration: &wafv2types.LoggingConfiguration{
			ResourceArn: in.ResourceArn,
			LogDestinationConfigs: []string{
				"arn:aws:logs:us-east-1:123456789012:log-group:aws-waf-logs-acme",
			},
		},
	}, nil
}

func (f *w4WAFFake) ListResourcesForWebACL(
	_ context.Context, _ *wafv2.ListResourcesForWebACLInput, _ ...func(*wafv2.Options),
) (*wafv2.ListResourcesForWebACLOutput, error) {
	return &wafv2.ListResourcesForWebACLOutput{ResourceArns: []string{
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-web/abc123",
	}}, nil
}

func (f *w4WAFFake) GetWebACL(
	_ context.Context, in *wafv2.GetWebACLInput, _ ...func(*wafv2.Options),
) (*wafv2.GetWebACLOutput, error) {
	name := aws.ToString(in.Name)
	if err, ok := f.getACLErr[name]; ok {
		return nil, err
	}
	return &wafv2.GetWebACLOutput{WebACL: &wafv2types.WebACL{
		Name:  in.Name,
		Id:    in.Id,
		ARN:   aws.String(w4WAFARN(name)),
		Rules: f.rules[name],
	}}, nil
}

var _ awsclient.WAFv2API = (*w4WAFFake)(nil)

func w4WAFARN(name string) string {
	return "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/" + name + "/11111111-2222-3333-4444-555555555555"
}

func w4WAFResource(name string) resource.Resource {
	arn := w4WAFARN(name)
	return resource.Resource{
		ID: arn, Name: name, Type: "waf",
		Fields: map[string]string{
			"name":  name,
			"id":    "11111111-2222-3333-4444-555555555555",
			"scope": "REGIONAL",
			"arn":   arn,
		},
	}
}

// w4BlockRule is a realistic single WAF rule.
func w4BlockRule(name string) wafv2types.Rule {
	return wafv2types.Rule{
		Name:     aws.String(name),
		Priority: 1,
		Action:   &wafv2types.RuleAction{Block: &wafv2types.BlockAction{}},
		Statement: &wafv2types.Statement{
			GeoMatchStatement: &wafv2types.GeoMatchStatement{
				CountryCodes: []wafv2types.CountryCode{wafv2types.CountryCodeKp},
			},
		},
		VisibilityConfig: &wafv2types.VisibilityConfig{
			SampledRequestsEnabled:   true,
			CloudWatchMetricsEnabled: true,
			MetricName:               aws.String(name),
		},
	}
}

func w4EnrichWAF(t *testing.T, fake *w4WAFFake, rs []resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	clients := &awsclient.ServiceClients{WAFv2: fake, Region: "us-east-1"}
	res, err := awsclient.EnrichWAFLogging(context.Background(), clients, rs, nil)
	if err != nil {
		t.Fatalf("EnrichWAFLogging: %v", err)
	}
	return res
}

// TestW4WAFNoRules pins the empty Web ACL: attached and logging, but with no
// rule that could ever block a request. The Web ACL next to it, carrying one
// rule, is not flagged.
func TestW4WAFNoRules(t *testing.T) {
	fake := &w4WAFFake{rules: map[string][]wafv2types.Rule{
		"acme-empty-acl":   nil,
		"acme-blocked-acl": {w4BlockRule("acme-geo-block")},
	}}
	res := w4EnrichWAF(t, fake, []resource.Resource{
		w4WAFResource("acme-empty-acl"), w4WAFResource("acme-blocked-acl"),
	})

	w4AssertFinding(t, res.Findings[w4WAFARN("acme-empty-acl")], w4CodeWAFNoRules,
		w4PhraseWAFNoRules, domain.SevWarn, w4SourceWAFWave2)
	w4AssertRows(t, res.AttentionDetails[w4WAFARN("acme-empty-acl")], w4CodeWAFNoRules,
		[]domain.DetailRow{{Label: "Rules", Value: "0"}})
	w4AssertNoCode(t, res.Findings[w4WAFARN("acme-blocked-acl")], w4CodeWAFNoRules)
}

// TestW4WAFNoRulesUnknownWhenGetWebACLFails pins that a Web ACL whose rules
// could not be read is not reported as empty. Unknown is not zero.
func TestW4WAFNoRulesUnknownWhenGetWebACLFails(t *testing.T) {
	fake := &w4WAFFake{
		rules:     map[string][]wafv2types.Rule{"acme-denied-acl": nil},
		getACLErr: map[string]error{"acme-denied-acl": errors.New("WAFInvalidParameterException")},
	}
	res := w4EnrichWAF(t, fake, []resource.Resource{w4WAFResource("acme-denied-acl")})
	w4AssertNoCode(t, res.Findings[w4WAFARN("acme-denied-acl")], w4CodeWAFNoRules)
}

// TestW4WAFNilClient pins the nil-client contract.
func TestW4WAFNilClient(t *testing.T) {
	res, err := awsclient.EnrichWAFLogging(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{w4WAFResource("acme-empty-acl")}, nil)
	if err != nil {
		t.Fatalf("EnrichWAFLogging: %v", err)
	}
	if res.Findings == nil || res.TruncatedIDs == nil || res.FieldUpdates == nil {
		t.Fatalf("result maps must be non-nil")
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v, want empty", res.Findings)
	}
}

// TestW4WAFFindingDef pins the registry row for the new waf code.
func TestW4WAFFindingDef(t *testing.T) {
	def := w4FindingDef(t, "waf", w4CodeWAFNoRules)
	if def.Phrase != w4PhraseWAFNoRules {
		t.Errorf("Phrase = %q, want %q", def.Phrase, w4PhraseWAFNoRules)
	}
	if def.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", def.Severity)
	}
	if def.Source != "wave2" {
		t.Errorf("Source = %q, want wave2", def.Source)
	}
}
