package unit

// d3_exposure_test.go — rows 9, 10, 11, 15 and 16: listeners read one page
// deep, a phrase that names only the first offending port, a policy verdict
// reached by substring, an action wilddcard scoped to one service, and user
// data that does not decode.

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	d3CodeELBPlainHTTP     domain.FindingCode = "elb.plain-http-listener"
	d3CodeELBWeakTLS       domain.FindingCode = "elb.weak-tls-policy"
	d3CodeCAPublicPolicy   domain.FindingCode = "codeartifact.public-access-policy"
	d3CodeVPCEPolicyOpen   domain.FindingCode = "vpce.policy-open"
	d3CodeLTUserDataSecret domain.FindingCode = "lt.user-data-secret"

	d3LBArn = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-web/abc123"
)

// --- rows 9 and 10: the listener pass ---------------------------------------

// d3ELBFake serves listeners across two pages and attributes that raise
// nothing, so the listener findings are the only ones under test.
type d3ELBFake struct {
	awsclient.ELBv2API
	page1, page2 []elbtypes.Listener
}

func (f *d3ELBFake) DescribeListeners(
	_ context.Context, in *elbv2.DescribeListenersInput, _ ...func(*elbv2.Options),
) (*elbv2.DescribeListenersOutput, error) {
	if in != nil && in.Marker != nil && *in.Marker == "page-2" {
		return &elbv2.DescribeListenersOutput{Listeners: f.page2}, nil
	}
	out := &elbv2.DescribeListenersOutput{Listeners: f.page1}
	if len(f.page2) > 0 {
		out.NextMarker = aws.String("page-2")
	}
	return out, nil
}

func (f *d3ELBFake) DescribeLoadBalancerAttributes(
	_ context.Context, _ *elbv2.DescribeLoadBalancerAttributesInput, _ ...func(*elbv2.Options),
) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
	return &elbv2.DescribeLoadBalancerAttributesOutput{Attributes: []elbtypes.LoadBalancerAttribute{
		{Key: aws.String("deletion_protection.enabled"), Value: aws.String("true")},
		{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("true")},
		{Key: aws.String("routing.http.drop_invalid_header_fields.enabled"), Value: aws.String("true")},
	}}, nil
}

var _ awsclient.ELBv2API = (*d3ELBFake)(nil)

func d3PlainHTTPListener(port int32) elbtypes.Listener {
	return elbtypes.Listener{
		ListenerArn:     aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-web/abc123/l" + string(rune('0'+port%10))),
		LoadBalancerArn: aws.String(d3LBArn),
		Port:            aws.Int32(port),
		Protocol:        elbtypes.ProtocolEnumHttp,
		DefaultActions: []elbtypes.Action{{
			Type:           elbtypes.ActionTypeEnumForward,
			TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web/def456"),
		}},
	}
}

func d3ELBResource() resource.Resource {
	return resource.Resource{
		ID: "acme-web", Name: "acme-web", Type: "elb",
		Fields: map[string]string{
			"load_balancer_arn": d3LBArn,
			"type":              "application",
			"state":             "active",
		},
	}
}

func d3EnrichELB(t *testing.T, fake *d3ELBFake) awsclient.IssueEnricherResult {
	t.Helper()
	res, _ := awsclient.EnrichELBAttributes(context.Background(),
		&awsclient.ServiceClients{ELBv2: fake, Region: "us-east-1"},
		[]resource.Resource{d3ELBResource()}, nil)
	return res
}

// TestD3ListenerOnSecondPageIsSeen pins row 9: a balancer's listeners are read
// to the end. Stopping after one page hides a cleartext listener behind
// whichever listeners happened to be returned first.
func TestD3ListenerOnSecondPageIsSeen(t *testing.T) {
	fake := &d3ELBFake{
		page1: []elbtypes.Listener{{
			ListenerArn:     aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-web/abc123/l1"),
			LoadBalancerArn: aws.String(d3LBArn),
			Port:            aws.Int32(443),
			Protocol:        elbtypes.ProtocolEnumHttps,
			SslPolicy:       aws.String("ELBSecurityPolicy-TLS13-1-2-2021-06"),
		}},
		page2: []elbtypes.Listener{d3PlainHTTPListener(80)},
	}
	res := d3EnrichELB(t, fake)
	w4AssertFinding(t, res.Findings["acme-web"], d3CodeELBPlainHTTP,
		"port 80 in the clear", domain.SevWarn, "wave2:elb")
	// d4 row 26: one listener, so the phrase is singular. Do not restore "ports".
}

// TestD3EveryCleartextPortIsNamed pins row 10: three listeners in the clear
// are three ports to close, and a phrase naming only the first sends the
// operator back to the console to find the rest.
func TestD3EveryCleartextPortIsNamed(t *testing.T) {
	fake := &d3ELBFake{page1: []elbtypes.Listener{
		d3PlainHTTPListener(80),
		d3PlainHTTPListener(8080),
		d3PlainHTTPListener(8081),
	}}
	res := d3EnrichELB(t, fake)

	w4AssertFinding(t, res.Findings["acme-web"], d3CodeELBPlainHTTP,
		"ports 80, 8080, 8081 in the clear", domain.SevWarn, "wave2:elb")
	ad, ok := res.AttentionDetails["acme-web"][d3CodeELBPlainHTTP]
	if !ok || len(ad.Rows) != 3 {
		t.Fatalf("rows = %+v, want one per offending listener", ad.Rows)
	}
}

// d3WeakTLSListener is an HTTPS listener on a retired security policy.
func d3WeakTLSListener(port int32) elbtypes.Listener {
	return elbtypes.Listener{
		ListenerArn:     aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-web/abc123/t" + string(rune('0'+port%10))),
		LoadBalancerArn: aws.String(d3LBArn),
		Port:            aws.Int32(port),
		Protocol:        elbtypes.ProtocolEnumHttps,
		SslPolicy:       aws.String("ELBSecurityPolicy-2016-08"),
	}
}

// TestD3EveryWeakTLSPortIsNamed pins the weak-TLS finding to the same shape as
// the cleartext one: two listeners on a retired policy are two ports to
// change, and each row leads with the port so the operator knows which
// listener a policy name belongs to. The listeners arrive in descending port
// order, so the phrase and the rows are only both ascending if they are
// ordered together rather than taken as the API answered.
func TestD3EveryWeakTLSPortIsNamed(t *testing.T) {
	res := d3EnrichELB(t, &d3ELBFake{page1: []elbtypes.Listener{
		d3WeakTLSListener(8443),
		d3WeakTLSListener(443),
	}})

	w4AssertFinding(t, res.Findings["acme-web"], d3CodeELBWeakTLS,
		"weak TLS policy on ports 443, 8443", domain.SevWarn, "wave2:elb")
	w4AssertRows(t, res.AttentionDetails["acme-web"], d3CodeELBWeakTLS, []domain.DetailRow{
		{Label: "Security policy", Value: "443: ELBSecurityPolicy-2016-08"},
		{Label: "Security policy", Value: "8443: ELBSecurityPolicy-2016-08"},
	})
}

// TestD3ModernTLSListenerIsNotWeak pins the negative case: a listener on a
// current security policy raises nothing, so merging ports must not start
// reporting healthy listeners.
func TestD3ModernTLSListenerIsNotWeak(t *testing.T) {
	modern := d3WeakTLSListener(443)
	modern.SslPolicy = aws.String("ELBSecurityPolicy-TLS13-1-2-2021-06")
	res := d3EnrichELB(t, &d3ELBFake{page1: []elbtypes.Listener{modern}})
	w4AssertNoCode(t, res.Findings["acme-web"], d3CodeELBWeakTLS)
}

// TestD3RedirectingListenerIsNotInTheClear pins the negative case: a listener
// that sends every request to HTTPS serves nothing in the clear, so merging
// ports must not start reporting it.
func TestD3RedirectingListenerIsNotInTheClear(t *testing.T) {
	redirect := d3PlainHTTPListener(80)
	redirect.DefaultActions = []elbtypes.Action{{
		Type: elbtypes.ActionTypeEnumRedirect,
		RedirectConfig: &elbtypes.RedirectActionConfig{
			Protocol: aws.String("HTTPS"), Port: aws.String("443"), StatusCode: elbtypes.RedirectActionStatusCodeEnumHttp301,
		},
	}}
	res := d3EnrichELB(t, &d3ELBFake{page1: []elbtypes.Listener{redirect}})
	w4AssertNoCode(t, res.Findings["acme-web"], d3CodeELBPlainHTTP)
}

// --- row 11: the codeartifact policy verdict --------------------------------

type d3CodeArtifactFake struct {
	awsclient.CodeArtifactAPI
	domains []codeartifacttypes.DomainSummary
	repos   []codeartifacttypes.RepositorySummary
	policy  string
}

func (f *d3CodeArtifactFake) ListDomains(
	_ context.Context, _ *codeartifact.ListDomainsInput, _ ...func(*codeartifact.Options),
) (*codeartifact.ListDomainsOutput, error) {
	return &codeartifact.ListDomainsOutput{Domains: f.domains}, nil
}

func (f *d3CodeArtifactFake) ListRepositories(
	_ context.Context, _ *codeartifact.ListRepositoriesInput, _ ...func(*codeartifact.Options),
) (*codeartifact.ListRepositoriesOutput, error) {
	return &codeartifact.ListRepositoriesOutput{Repositories: f.repos}, nil
}

func (f *d3CodeArtifactFake) GetRepositoryPermissionsPolicy(
	_ context.Context, _ *codeartifact.GetRepositoryPermissionsPolicyInput, _ ...func(*codeartifact.Options),
) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error) {
	if f.policy == "" {
		return nil, &codeartifacttypes.ResourceNotFoundException{Message: aws.String("no policy")}
	}
	return &codeartifact.GetRepositoryPermissionsPolicyOutput{
		Policy: &codeartifacttypes.ResourcePolicy{Document: aws.String(f.policy)},
	}, nil
}

func (f *d3CodeArtifactFake) GetDomainPermissionsPolicy(
	_ context.Context, _ *codeartifact.GetDomainPermissionsPolicyInput, _ ...func(*codeartifact.Options),
) (*codeartifact.GetDomainPermissionsPolicyOutput, error) {
	return nil, &codeartifacttypes.ResourceNotFoundException{Message: aws.String("no policy")}
}

var _ awsclient.CodeArtifactAPI = (*d3CodeArtifactFake)(nil)

// A wildcard principal written the way AWS pretty-prints it, and scoped by an
// organisation condition. Substring matching sees neither: the first has a
// space the pattern lacks, and the second is not public at all.
const d3CAConditionedPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "codeartifact:ReadFromRepository",
      "Resource": "*",
      "Condition": {"StringEquals": {"aws:PrincipalOrgID": "o-acme12345"}}
    }
  ]
}`

const d3CAOpenPolicyPretty = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": "*"},
      "Action": "codeartifact:ReadFromRepository",
      "Resource": "*"
    }
  ]
}`

func d3EnrichCodeArtifact(t *testing.T, policy string) awsclient.IssueEnricherResult {
	t.Helper()
	fake := &d3CodeArtifactFake{
		domains: []codeartifacttypes.DomainSummary{{Name: aws.String("acme-artifacts")}},
		repos: []codeartifacttypes.RepositorySummary{{
			Name: aws.String("acme-npm"), DomainName: aws.String("acme-artifacts"),
		}},
		policy: policy,
	}
	res, err := awsclient.EnrichCodeArtifactRepository(context.Background(),
		&awsclient.ServiceClients{CodeArtifact: fake, Region: "us-east-1"},
		[]resource.Resource{{
			ID: "acme-artifacts", Name: "acme-artifacts", Type: "codeartifact",
			Fields: map[string]string{"domain_name": "acme-artifacts"},
		}}, nil)
	if err != nil {
		t.Fatalf("EnrichCodeArtifactPolicies: %v", err)
	}
	return res
}

// TestD3CodeArtifactOpenPolicyFoundWhateverTheSpacing pins half of row 11: the
// verdict comes from parsing the document, so a policy AWS pretty-printed
// reads the same as one written without spaces.
func TestD3CodeArtifactOpenPolicyFoundWhateverTheSpacing(t *testing.T) {
	res := d3EnrichCodeArtifact(t, d3CAOpenPolicyPretty)
	found := false
	for _, fs := range res.Findings {
		if _, ok := w4FindingByCode(t, fs, d3CodeCAPublicPolicy); ok {
			found = true
		}
	}
	if !found {
		t.Errorf("a pretty-printed wildcard principal was not read as public")
	}
}

// TestD3CodeArtifactConditionedPolicyIsNotPublic pins the other half: a
// wildcard principal narrowed to one organisation is a scoped grant. A
// substring match cannot see the condition and calls it public.
func TestD3CodeArtifactConditionedPolicyIsNotPublic(t *testing.T) {
	res := d3EnrichCodeArtifact(t, d3CAConditionedPolicy)
	for id, fs := range res.Findings {
		if _, ok := w4FindingByCode(t, fs, d3CodeCAPublicPolicy); ok {
			t.Errorf("%s: a policy scoped by aws:PrincipalOrgID was reported as public", id)
		}
	}
}

// --- row 15: an action wildcard inside one service --------------------------

type d3VPCEFake struct {
	policy string
}

func (f *d3VPCEFake) DescribeVpcEndpoints(
	_ context.Context, _ *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options),
) (*ec2.DescribeVpcEndpointsOutput, error) {
	return &ec2.DescribeVpcEndpointsOutput{VpcEndpoints: []ec2types.VpcEndpoint{{
		VpcEndpointId:   aws.String("vpce-0acme1234567890"),
		VpcId:           aws.String("vpc-0acme12345"),
		ServiceName:     aws.String("com.amazonaws.us-east-1.s3"),
		VpcEndpointType: ec2types.VpcEndpointTypeGateway,
		State:           ec2types.StateAvailable,
		PolicyDocument:  aws.String(f.policy),
	}}}, nil
}

// d3VPCEPolicy builds an endpoint policy granting action to everyone.
func d3VPCEPolicy(action string) string {
	return `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*",` +
		`"Action":"` + action + `","Resource":"*"}]}`
}

func d3FetchVPCE(t *testing.T, policy string) resource.Resource {
	t.Helper()
	out, err := awsclient.FetchVPCEndpointsPage(context.Background(), &d3VPCEFake{policy: policy}, "")
	if err != nil {
		t.Fatalf("FetchVPCEndpointsPage: %v", err)
	}
	if len(out.Resources) != 1 {
		t.Fatalf("fetched %d endpoints, want 1", len(out.Resources))
	}
	return out.Resources[0]
}

// TestD3VPCEServiceWideActionWildcardIsOpen pins row 15's ruling: an endpoint
// that grants every action of its own service to every principal is as open
// as one granting "*". The endpoint only ever fronts that service, so the
// narrower wildcard withholds nothing.
func TestD3VPCEServiceWideActionWildcardIsOpen(t *testing.T) {
	r := d3FetchVPCE(t, d3VPCEPolicy("s3:*"))
	w4AssertFinding(t, r.Findings, d3CodeVPCEPolicyOpen,
		"endpoint policy open to anyone", domain.SevWarn, "wave1")
}

// TestD3VPCESingleActionIsNotOpen pins the negative case: one named action
// granted to everyone is a deliberate share, not the open-endpoint signal.
func TestD3VPCESingleActionIsNotOpen(t *testing.T) {
	r := d3FetchVPCE(t, d3VPCEPolicy("s3:GetObject"))
	w4AssertNoCode(t, r.Findings, d3CodeVPCEPolicyOpen)
}

// --- row 16: user data that does not decode ---------------------------------

// d3LTResource builds a launch template whose default version carries the
// given user data verbatim, the way the fetcher hands it to the enricher.
func d3LTResource(userData string) resource.Resource {
	return resource.Resource{
		ID: "lt-0acme1234567890", Name: "acme-web-template", Type: "lt",
		Fields: map[string]string{"launch_template_name": "acme-web-template"},
		RawStruct: awsclient.LTRaw{
			Template: ec2types.LaunchTemplate{
				LaunchTemplateId:   aws.String("lt-0acme1234567890"),
				LaunchTemplateName: aws.String("acme-web-template"),
			},
			DefaultVersion: ec2types.LaunchTemplateVersion{
				VersionNumber: aws.Int64(3),
				LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
					UserData: aws.String(userData),
				},
			},
		},
	}
}

// TestD3UndecodableUserDataIsScannedRaw pins row 16's ruling: a credential
// pasted into user data is a credential whether or not the text was ever
// base64. The fixture is a shell script with characters base64 has no
// alphabet for, so any decode of it must fail and the raw text is what there
// is to scan.
func TestD3UndecodableUserDataIsScannedRaw(t *testing.T) {
	const raw = "#!/bin/bash\nexport DB_PASSWORD=hunter2-acme-prod\n"
	if _, err := base64.StdEncoding.DecodeString(raw); err == nil {
		t.Fatalf("fixture decodes as base64; it must not, or the test proves nothing")
	}

	res, err := awsclient.EnrichLTDeprecatedAMI(context.Background(),
		&awsclient.ServiceClients{Region: "us-east-1"},
		[]resource.Resource{d3LTResource(raw)}, nil)
	if err != nil {
		t.Fatalf("EnrichLTDeprecatedAMI: %v", err)
	}
	w4AssertFinding(t, res.Findings["lt-0acme1234567890"], d3CodeLTUserDataSecret,
		"credential in user data", domain.SevBroken, "wave2:lt")
}

// TestD3CleanUserDataReportsNothing pins the negative case: a template whose
// user data holds no credential is not flagged, so scanning raw text does not
// turn every script into a finding.
func TestD3CleanUserDataReportsNothing(t *testing.T) {
	const raw = "#!/bin/bash\nyum install -y nginx\nsystemctl enable nginx\n"
	res, err := awsclient.EnrichLTDeprecatedAMI(context.Background(),
		&awsclient.ServiceClients{Region: "us-east-1"},
		[]resource.Resource{d3LTResource(raw)}, nil)
	if err != nil {
		t.Fatalf("EnrichLTDeprecatedAMI: %v", err)
	}
	w4AssertNoCode(t, res.Findings["lt-0acme1234567890"], d3CodeLTUserDataSecret)
}

// TestD3PortListIsStableWhateverTheAPIOrder attacks the merged phrase: the
// listener order DescribeListeners returns is not part of the contract, so a
// phrase built by appending in arrival order reads differently between two
// refreshes of the same unchanged balancer. The status cell must be a
// function of the balancer, not of the order the API happened to answer in.
func TestD3PortListIsStableWhateverTheAPIOrder(t *testing.T) {
	ascending := d3EnrichELB(t, &d3ELBFake{page1: []elbtypes.Listener{
		d3PlainHTTPListener(80), d3PlainHTTPListener(8080), d3PlainHTTPListener(8081),
	}})
	descending := d3EnrichELB(t, &d3ELBFake{page1: []elbtypes.Listener{
		d3PlainHTTPListener(8081), d3PlainHTTPListener(8080), d3PlainHTTPListener(80),
	}})

	a, ok := w4FindingByCode(t, ascending.Findings["acme-web"], d3CodeELBPlainHTTP)
	if !ok {
		t.Fatalf("no cleartext finding in the ascending case")
	}
	d, ok := w4FindingByCode(t, descending.Findings["acme-web"], d3CodeELBPlainHTTP)
	if !ok {
		t.Fatalf("no cleartext finding in the descending case")
	}
	if a.Phrase != d.Phrase {
		t.Errorf("the phrase follows the API's order:\n  %q\n  %q", a.Phrase, d.Phrase)
	}
}

// TestD3CleartextPortsMergeAcrossPages attacks rows 9 and 10 together: paging
// and merging have to compose, or a balancer with one offending listener per
// page reports only the page the loop happened to finish on.
func TestD3CleartextPortsMergeAcrossPages(t *testing.T) {
	res := d3EnrichELB(t, &d3ELBFake{
		page1: []elbtypes.Listener{d3PlainHTTPListener(80)},
		page2: []elbtypes.Listener{d3PlainHTTPListener(8080)},
	})
	w4AssertFinding(t, res.Findings["acme-web"], d3CodeELBPlainHTTP,
		"ports 80, 8080 in the clear", domain.SevWarn, "wave2:elb")
}
