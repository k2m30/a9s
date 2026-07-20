// consolelink_test.go — helper-level tests for core/consolelink, the package
// backing the "o = open in AWS console / O = copy console URL" feature
// (spec: console-url-spec.md). This package does not exist yet at RED time;
// these tests define its contract: Domain, Regional, Global, GoView,
// AccountFromARN, Valid, Resolve.
//
// Per-type builder coverage (all 70 top-level types) lives in
// console_url_types_test.go. This file covers only the shared helpers.
package unit_test

import (
	"net/url"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
)

// ─── Domain ─────────────────────────────────────────────────────────────────

func TestConsoleLink_Domain_PartitionRouting(t *testing.T) {
	cases := []struct {
		region string
		want   string
	}{
		{"us-east-1", "console.aws.amazon.com"},
		{"eu-central-1", "console.aws.amazon.com"},
		{"ap-southeast-2", "console.aws.amazon.com"},
		{"us-gov-west-1", "console.amazonaws-us-gov.com"},
		{"us-gov-east-1", "console.amazonaws-us-gov.com"},
		{"cn-north-1", "console.amazonaws.cn"},
		{"cn-northwest-1", "console.amazonaws.cn"},
	}
	for _, c := range cases {
		if got := consolelink.Domain(c.region); got != c.want {
			t.Errorf("Domain(%q) = %q, want %q", c.region, got, c.want)
		}
	}
}

// ─── Regional / Global ──────────────────────────────────────────────────────

func TestConsoleLink_Regional_BuildsRegionSubdomainHost(t *testing.T) {
	cases := []struct {
		region, tail, want string
	}{
		{
			"us-east-1", "ec2/home?region=us-east-1#InstanceDetails:instanceId=i-0abc123",
			"https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-0abc123",
		},
		{
			"us-gov-west-1", "ec2/home?region=us-gov-west-1",
			"https://us-gov-west-1.console.amazonaws-us-gov.com/ec2/home?region=us-gov-west-1",
		},
		{
			"cn-north-1", "s3/buckets/my-bucket",
			"https://cn-north-1.console.amazonaws.cn/s3/buckets/my-bucket",
		},
	}
	for _, c := range cases {
		if got := consolelink.Regional(c.region, c.tail); got != c.want {
			t.Errorf("Regional(%q, %q) = %q, want %q", c.region, c.tail, got, c.want)
		}
	}
}

func TestConsoleLink_Global_OmitsRegionSubdomain(t *testing.T) {
	cases := []struct {
		region, tail, want string
	}{
		{"us-east-1", "iam/home#/roles/details/my-role", "https://console.aws.amazon.com/iam/home#/roles/details/my-role"},
		{"eu-west-1", "s3/buckets/my-bucket", "https://console.aws.amazon.com/s3/buckets/my-bucket"},
		{"us-gov-west-1", "iam/home", "https://console.amazonaws-us-gov.com/iam/home"},
		{"cn-north-1", "iam/home", "https://console.amazonaws.cn/iam/home"},
	}
	for _, c := range cases {
		if got := consolelink.Global(c.region, c.tail); got != c.want {
			t.Errorf("Global(%q, %q) = %q, want %q", c.region, c.tail, got, c.want)
		}
	}
}

// ─── GoView ──────────────────────────────────────────────────────────────────

func TestConsoleLink_GoView_QueryEscapesARN(t *testing.T) {
	cases := []struct {
		region, arn string
	}{
		{"us-east-1", "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123"},
		{"us-gov-west-1", "arn:aws-us-gov:ec2:us-gov-west-1:123456789012:instance/i-0abc123"},
		{"cn-north-1", "arn:aws-cn:ec2:cn-north-1:123456789012:instance/i-0abc123"},
		{"eu-west-1", "arn:aws:codeartifact:eu-west-1:123456789012:repository/dom ain/repo name"},
	}
	for _, c := range cases {
		want := consolelink.Global(c.region, "go/view?arn="+url.QueryEscape(c.arn))
		if got := consolelink.GoView(c.region, c.arn); got != want {
			t.Errorf("GoView(%q, %q) = %q, want %q", c.region, c.arn, got, want)
		}
	}
}

// ─── AccountFromARN ──────────────────────────────────────────────────────────

func TestConsoleLink_AccountFromARN(t *testing.T) {
	cases := []struct {
		name, arn, want string
	}{
		{"standard resource arn", "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123", "123456789012"},
		{"empty region field (iam)", "arn:aws:iam::123456789012:role/my-role", "123456789012"},
		{"s3 arn has no account field", "arn:aws:s3:::my-bucket", ""},
		{"not an arn at all", "not-an-arn", ""},
		{"empty string", "", ""},
		{"too few colon fields", "arn:aws:ec2", ""},
	}
	for _, c := range cases {
		if got := consolelink.AccountFromARN(c.arn); got != c.want {
			t.Errorf("%s: AccountFromARN(%q) = %q, want %q", c.name, c.arn, got, c.want)
		}
	}
}

// ─── Valid ───────────────────────────────────────────────────────────────────

func TestConsoleLink_Valid_AcceptsRealConsoleHosts(t *testing.T) {
	valid := []string{
		"https://console.aws.amazon.com/iam/home",
		"https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1",
		"https://eu-central-1.console.aws.amazon.com/s3/buckets/x",
		"https://console.amazonaws-us-gov.com/iam/home",
		"https://us-gov-west-1.console.amazonaws-us-gov.com/ec2/home",
		"https://console.amazonaws.cn/iam/home",
		"https://cn-north-1.console.amazonaws.cn/ec2/home",
	}
	for _, u := range valid {
		if !consolelink.Valid(u) {
			t.Errorf("Valid(%q) = false, want true", u)
		}
	}
}

func TestConsoleLink_Valid_RejectsNonConsoleOrInsecureURLs(t *testing.T) {
	invalid := []string{
		"http://console.aws.amazon.com/iam/home",            // not https
		"https://evil.example.com/iam/home",                 // wrong host entirely
		"https://console.aws.amazon.com.evil.example.com/x", // suffix-spoofing: real domain as a prefix of an attacker host
		"",                 // empty
		"not-a-url-at-all", // unparseable
	}
	for _, u := range invalid {
		if consolelink.Valid(u) {
			t.Errorf("Valid(%q) = true, want false", u)
		}
	}
}

// ─── Resolve ─────────────────────────────────────────────────────────────────

func TestConsoleLink_Resolve_UsesConsoleURLWhenSet(t *testing.T) {
	td := catalog.ResourceTypeDef{
		ShortName: "ec2",
		ConsoleURL: func(r domain.Resource, region, accountID string) string {
			return "https://" + region + ".console.aws.amazon.com/ec2/home?region=" + region + "#InstanceDetails:instanceId=" + r.ID
		},
	}
	r := domain.Resource{ID: "i-0abc123"}

	got, ok := consolelink.Resolve(td, r, "us-east-1", "123456789012")
	want := "https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-0abc123"
	if !ok {
		t.Fatal("Resolve() ok = false, want true when ConsoleURL is set and non-empty")
	}
	if got != want {
		t.Errorf("Resolve() = %q, want %q", got, want)
	}
}

func TestConsoleLink_Resolve_NilConsoleURLFallsBackToGoViewOnArnField(t *testing.T) {
	// ct-events has no ConsoleURL builder per spec (deliberately nil).
	td := catalog.ResourceTypeDef{ShortName: "ct-events"}
	arn := "arn:aws:cloudtrail:us-east-1:123456789012:trail/acme-management-trail"
	r := domain.Resource{ID: "evt-1", Fields: map[string]string{"arn": arn}}

	got, ok := consolelink.Resolve(td, r, "us-east-1", "123456789012")
	want := "https://console.aws.amazon.com/go/view?arn=" + url.QueryEscape(arn)
	if !ok {
		t.Fatal("Resolve() ok = false, want true when Fields[\"arn\"] is present")
	}
	if got != want {
		t.Errorf("Resolve() = %q, want %q", got, want)
	}
}

func TestConsoleLink_Resolve_NoConsoleURLAndNoArnField_ReturnsFalse(t *testing.T) {
	td := catalog.ResourceTypeDef{ShortName: "ct-events"}
	r := domain.Resource{ID: "evt-1", Fields: map[string]string{"event_name": "RunInstances"}}

	got, ok := consolelink.Resolve(td, r, "us-east-1", "123456789012")
	if ok {
		t.Error("Resolve() ok = true, want false when neither ConsoleURL nor Fields[\"arn\"] resolve a URL")
	}
	if got != "" {
		t.Errorf("Resolve() = %q, want empty string", got)
	}
}

func TestConsoleLink_Resolve_ConsoleURLReturningEmptyFallsBackToGoView(t *testing.T) {
	// Simulates e.g. trail/msk/sfn/ecs-svc when their required Fields key is
	// missing at runtime: the type-specific builder itself returns "", but a
	// bare Fields["arn"] is still present, so Resolve must not give up.
	td := catalog.ResourceTypeDef{
		ShortName: "trail",
		ConsoleURL: func(r domain.Resource, region, accountID string) string {
			return ""
		},
	}
	arn := "arn:aws:cloudtrail:us-east-1:123456789012:trail/no-trail-arn-field"
	r := domain.Resource{ID: "t-1", Fields: map[string]string{"arn": arn}}

	got, ok := consolelink.Resolve(td, r, "us-east-1", "123456789012")
	want := "https://console.aws.amazon.com/go/view?arn=" + url.QueryEscape(arn)
	if !ok {
		t.Fatal("Resolve() ok = false, want true (GoView fallback) when ConsoleURL returns \"\" but Fields[\"arn\"] is present")
	}
	if got != want {
		t.Errorf("Resolve() = %q, want %q", got, want)
	}
}
