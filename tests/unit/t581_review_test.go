package unit

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func t581Refused(op string) error {
	return &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform: " + op}
}

// ─── account-wide reads are made only when a row needs them ─────────────

type t581CountingLogs struct {
	*t581LogsDeliveryFake
	mu    sync.Mutex
	calls int
}

func (f *t581CountingLogs) count() {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
}

func (f *t581CountingLogs) DescribeDeliveries(ctx context.Context, in *cloudwatchlogs.DescribeDeliveriesInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeDeliveriesOutput, error) {
	f.count()
	return f.t581LogsDeliveryFake.DescribeDeliveries(ctx, in, opts...)
}

func (f *t581CountingLogs) DescribeDeliverySources(ctx context.Context, in *cloudwatchlogs.DescribeDeliverySourcesInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeDeliverySourcesOutput, error) {
	f.count()
	return f.t581LogsDeliveryFake.DescribeDeliverySources(ctx, in, opts...)
}

func (f *t581CountingLogs) GetDeliverySource(ctx context.Context, in *cloudwatchlogs.GetDeliverySourceInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetDeliverySourceOutput, error) {
	f.count()
	return f.t581LogsDeliveryFake.GetDeliverySource(ctx, in, opts...)
}

// A distribution with legacy logging on needs no standard logging (v2)
// answer, so a role that may not read CloudWatch Logs deliveries gets no
// failure when every distribution logs the legacy way.
func TestT581Review_CFDeliveriesReadOnlyForLegacyLoggingOff(t *testing.T) {
	const id = "EDFDVBD6EXAMPLEF"
	on := cfDistroConfigRedirectHTTPS(id)
	on.Logging = &cftypes.LoggingConfig{Enabled: aws.Bool(true), IncludeCookies: aws.Bool(false), Bucket: aws.String("acme-cdn-logs.s3.amazonaws.com"), Prefix: aws.String("cf/")}
	logs := &t581CountingLogs{t581LogsDeliveryFake: &t581LogsDeliveryFake{err: t581Refused("logs:DescribeDeliveries")}}
	clients := &awsclient.ServiceClients{CloudFront: &cfGetDistributionConfigFake{results: map[string]*cftypes.DistributionConfig{id: on}}, CloudWatchLogs: logs}
	result, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients, t581CFResources(id), nil)
	if err != nil {
		t.Errorf("enricher error = %v, want none: no distribution needed the deliveries", err)
	}
	if logs.calls != 0 {
		t.Errorf("CloudWatch Logs delivery reads = %d, want 0", logs.calls)
	}
	if _, marked := result.TruncatedIDs[id]; marked {
		t.Errorf("%s marked not inspected (%q) by a read it did not need", id, result.TruncatedIDs[id])
	}
}

type t581SSLPoliciesELB struct {
	*w3ELBv2Fake
	err   error
	mu    sync.Mutex
	calls int
}

func (f *t581SSLPoliciesELB) DescribeSSLPolicies(ctx context.Context, in *elasticloadbalancingv2.DescribeSSLPoliciesInput, opts ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeSSLPoliciesOutput, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.w3ELBv2Fake.DescribeSSLPolicies(ctx, in, opts...)
}

// A balancer with no TLS listener has no security policy to judge, so the
// Region's policy list is not read.
func TestT581Review_ELBPoliciesReadOnlyForATLSListener(t *testing.T) {
	r := w3ELBRes("acme-internal", "application")
	fake := &t581SSLPoliciesELB{w3ELBv2Fake: w3NewELBFake(), err: t581Refused("elasticloadbalancing:DescribeSSLPolicies")}
	arn := r.Fields["load_balancer_arn"]
	fake.attrs[arn] = w3ELBAttrs(nil)
	fake.listeners[arn] = []elbtypes.Listener{w3Listener("http", 80, elbtypes.ProtocolEnumHttp, "", w3RedirectToHTTPSAction())}
	res, err := awsclient.EnrichELBAttributes(context.Background(), &awsclient.ServiceClients{ELBv2: fake}, []resource.Resource{r}, nil)
	if err != nil {
		t.Errorf("enricher error = %v, want none", err)
	}
	if fake.calls != 0 {
		t.Errorf("DescribeSSLPolicies calls = %d, want 0", fake.calls)
	}
	if _, marked := res.TruncatedIDs[r.ID]; marked {
		t.Errorf("%s marked not inspected (%q) by a read it did not need", r.ID, res.TruncatedIDs[r.ID])
	}
}

// A refused policy list is one failure for the balancer, however many TLS
// listeners it has; the failure count sits beside another cause's count in
// the aggregate.
func TestT581Review_ELBPolicyFailureCountsOncePerBalancer(t *testing.T) {
	tls := w3ELBRes("acme-edge", "application")
	broken := w3ELBRes("acme-legacy", "application")
	fake := &t581SSLPoliciesELB{w3ELBv2Fake: w3NewELBFake(), err: t581Refused("elasticloadbalancing:DescribeSSLPolicies")}
	tlsARN, brokenARN := tls.Fields["load_balancer_arn"], broken.Fields["load_balancer_arn"]
	fake.attrs[tlsARN] = w3ELBAttrs(nil)
	fake.attrs[brokenARN] = w3ELBAttrs(nil)
	fake.listeners[tlsARN] = []elbtypes.Listener{
		w3Listener("a", 443, elbtypes.ProtocolEnumHttps, "ELBSecurityPolicy-TLS13-1-2-2021-06", w3ForwardAction()),
		w3Listener("b", 8443, elbtypes.ProtocolEnumHttps, "ELBSecurityPolicy-TLS13-1-2-2021-06", w3ForwardAction()),
		w3Listener("c", 9443, elbtypes.ProtocolEnumHttps, "ELBSecurityPolicy-TLS13-1-2-2021-06", w3ForwardAction()),
	}
	fake.listenerErr[brokenARN] = t581Refused("elasticloadbalancing:DescribeListeners")
	res, err := awsclient.EnrichELBAttributes(context.Background(), &awsclient.ServiceClients{ELBv2: fake}, []resource.Resource{tls, broken}, nil)
	if _, marked := res.TruncatedIDs[tls.ID]; !marked {
		t.Errorf("%s: a refused policy list left its TLS listeners inspected", tls.ID)
	}
	if err == nil || !strings.Contains(err.Error(), "DescribeSSLPolicies (1, e.g. "+tls.ID+")") {
		t.Errorf("aggregate = %v, want the policy failure counted once for %s", err, tls.ID)
	}
}

type t581IMDSCountingEC2 struct {
	*t581LTEC2Fake
	mu    sync.Mutex
	calls int
}

func (f *t581IMDSCountingEC2) GetInstanceMetadataDefaults(_ context.Context, _ *ec2.GetInstanceMetadataDefaultsInput, _ ...func(*ec2.Options)) (*ec2.GetInstanceMetadataDefaultsOutput, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return nil, t581Refused("ec2:GetInstanceMetadataDefaults")
}

// A template that requires IMDSv2 itself is settled by its own spec, so the
// account defaults are not read for it.
func TestT581Review_IMDSAccountDefaultsReadOnlyForAnOpenSpec(t *testing.T) {
	const id = "lt-0a1b2c3d4e5f6000b"
	fake := &t581IMDSCountingEC2{t581LTEC2Fake: &t581LTEC2Fake{
		templates: []ec2types.LaunchTemplate{t581LT(id, "acme-hardened")},
		versions: map[string]ec2types.LaunchTemplateVersion{id: t581LTVersion(id, t581AMIDefault, &ec2types.LaunchTemplateInstanceMetadataOptions{
			HttpTokens: ec2types.LaunchTemplateHttpTokensStateRequired,
		})},
	}}
	_, result := t581LoadAndEnrich(t, "lt", &awsclient.ServiceClients{EC2: fake}, t581LoadedAMICache())
	if fake.calls != 0 {
		t.Errorf("GetInstanceMetadataDefaults calls = %d, want 0", fake.calls)
	}
	if _, marked := result.TruncatedIDs[id]; marked {
		t.Errorf("%s marked not inspected (%q) by a read it did not need", id, result.TruncatedIDs[id])
	}
}

// ─── launch templates without an EC2 client ─────────────────────────────

// With no EC2 client neither the account defaults, the AMI default nor the
// Region's EBS encryption default can be read, so a template whose verdict
// rests on them is not inspected rather than clean.
func TestT581Review_LTWithoutEC2ClientIsNotInspected(t *testing.T) {
	const id = "lt-0a1b2c3d4e5f6000c"
	version := t581LTVersion(id, t581AMIDefault, &ec2types.LaunchTemplateInstanceMetadataOptions{HttpTokens: ec2types.LaunchTemplateHttpTokensStateOptional})
	version.LaunchTemplateData.BlockDeviceMappings = []ec2types.LaunchTemplateBlockDeviceMapping{t581UnencryptedRoot()}
	fake := &t581LTEC2Fake{templates: []ec2types.LaunchTemplate{t581LT(id, "acme-web")}, versions: map[string]ec2types.LaunchTemplateVersion{id: version}}
	rows, _ := t581LoadAndEnrich(t, "lt", &awsclient.ServiceClients{EC2: fake}, t581LoadedAMICache())

	result, err := awsclient.EnrichLTDeprecatedAMI(context.Background(), &awsclient.ServiceClients{}, rows, t581LoadedAMICache())
	if err != nil {
		t.Errorf("enricher error = %v", err)
	}
	if got := result.TruncatedIDs[id]; !strings.Contains(got, "no EC2 client") {
		t.Errorf("TruncatedIDs[%s] = %q, want it to name the missing EC2 client", id, got)
	}
	for _, code := range []domain.FindingCode{t581CodeLTIMDSv1, "lt.warn.unencrypted"} {
		if hasCode(result.Findings[id], code) {
			t.Errorf("%s raised without the read that decides it", code)
		}
	}
}

// ─── launch templates and EBS encryption by default ─────────────────────

func t581UnencryptedRoot() ec2types.LaunchTemplateBlockDeviceMapping {
	return ec2types.LaunchTemplateBlockDeviceMapping{
		DeviceName: aws.String("/dev/xvda"),
		Ebs: &ec2types.LaunchTemplateEbsBlockDevice{
			Encrypted:  aws.Bool(false),
			VolumeSize: aws.Int32(20),
			VolumeType: ec2types.VolumeTypeGp3,
		},
	}
}

type t581EBSDefaultEC2 struct {
	*t581LTEC2Fake
	byDefault bool
	err       error
	mu        sync.Mutex
	calls     int
}

func (f *t581EBSDefaultEC2) GetEbsEncryptionByDefault(_ context.Context, _ *ec2.GetEbsEncryptionByDefaultInput, _ ...func(*ec2.Options)) (*ec2.GetEbsEncryptionByDefaultOutput, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return &ec2.GetEbsEncryptionByDefaultOutput{EbsEncryptionByDefault: aws.Bool(f.byDefault)}, nil
}

// A block device set to Encrypted=false still gets an encrypted volume in a
// Region that encrypts every new volume by default; the setting is read once,
// and only when a template sets encryption off.
func TestT581Review_LTUnencryptedTakesTheRegionEBSDefault(t *testing.T) {
	const (
		off   = "lt-0a1b2c3d4e5f6000d"
		unset = "lt-0a1b2c3d4e5f6000e"
	)
	required := &ec2types.LaunchTemplateInstanceMetadataOptions{HttpTokens: ec2types.LaunchTemplateHttpTokensStateRequired}
	offVersion := t581LTVersion(off, t581AMIDefault, required)
	offVersion.LaunchTemplateData.BlockDeviceMappings = []ec2types.LaunchTemplateBlockDeviceMapping{t581UnencryptedRoot()}
	unsetVersion := t581LTVersion(unset, t581AMIDefault, required)
	root := t581UnencryptedRoot()
	root.Ebs.Encrypted = nil
	unsetVersion.LaunchTemplateData.BlockDeviceMappings = []ec2types.LaunchTemplateBlockDeviceMapping{root}
	newFake := func(byDefault bool, err error, ids ...string) *t581EBSDefaultEC2 {
		f := &t581EBSDefaultEC2{t581LTEC2Fake: &t581LTEC2Fake{versions: map[string]ec2types.LaunchTemplateVersion{off: offVersion, unset: unsetVersion}}, byDefault: byDefault, err: err}
		for _, id := range ids {
			f.templates = append(f.templates, t581LT(id, "acme-"+id))
		}
		return f
	}
	for _, c := range []struct {
		name      string
		byDefault bool
		err       error
		flagged   bool
		marked    bool
	}{
		{"Region default off", false, nil, true, false},
		{"Region default on", true, nil, false, false},
		{"Region default refused", false, t581Refused("ec2:GetEbsEncryptionByDefault"), false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			fake := newFake(c.byDefault, c.err, off, unset)
			rows, result := t581LoadAndEnrich(t, "lt", &awsclient.ServiceClients{EC2: fake}, t581LoadedAMICache())
			if got := hasCode(t581Row(t, rows, off).Findings, "lt.warn.unencrypted"); got != c.flagged {
				t.Errorf("%s: lt.warn.unencrypted = %v, want %v", off, got, c.flagged)
			}
			if _, got := result.TruncatedIDs[off]; got != c.marked {
				t.Errorf("%s: not inspected = %v, want %v", off, got, c.marked)
			}
			if hasCode(t581Row(t, rows, unset).Findings, "lt.warn.unencrypted") {
				t.Errorf("%s: an unset Encrypted takes the Region default and raised lt.warn.unencrypted", unset)
			}
			if fake.calls != 1 {
				t.Errorf("GetEbsEncryptionByDefault calls = %d, want 1", fake.calls)
			}
		})
	}
	t.Run("no template sets encryption off", func(t *testing.T) {
		fake := newFake(false, nil, unset)
		t581LoadAndEnrich(t, "lt", &awsclient.ServiceClients{EC2: fake}, t581LoadedAMICache())
		if fake.calls != 0 {
			t.Errorf("GetEbsEncryptionByDefault calls = %d, want 0", fake.calls)
		}
	})
}

// ─── ecr ────────────────────────────────────────────────────────────────

// A filter without a wildcard matches every repository name that contains it
// (Amazon ECR User Guide, "Filters to choose which repositories are
// scanned"), and the Scan column states the scanning the rules and the
// repository setting give.
func TestT581Review_ECRFilterAndScanColumn(t *testing.T) {
	const (
		containsMatch = "acme/api-service"
		continuous    = "acme/frontend"
		unmatched     = fixtures.ECRRegistryScanned
	)
	clients := demo.NewServiceClients()
	w := &t581ECRWrapper{
		ECRAPI: clients.ECR,
		mutate: map[string]func(*ecrtypes.Repository){containsMatch: t581ScanOnPushOff, continuous: t581ScanOnPushOff, unmatched: t581ScanOnPushOff},
		scanning: &ecrtypes.RegistryScanningConfiguration{
			ScanType: ecrtypes.ScanTypeEnhanced,
			Rules: []ecrtypes.RegistryScanningRule{
				t581ScanRule(ecrtypes.ScanFrequencyScanOnPush, "api-serv"),
				t581ScanRule(ecrtypes.ScanFrequencyScanOnPush, "acme/front*"),
				t581ScanRule(ecrtypes.ScanFrequencyContinuousScan, "*end"),
			},
		},
	}
	clients.ECR = w
	rows, result := t581LoadAndEnrich(t, "ecr", clients, nil)
	for _, c := range []struct {
		repo, scan string
		flagged    bool
	}{
		{containsMatch, "on push", false},
		{continuous, "continuous", false},
		{unmatched, "off", true},
	} {
		if got := hasCode(t581Row(t, rows, c.repo).Findings, t581CodeECRScanOnPushOff); got != c.flagged {
			t.Errorf("%s: %s = %v, want %v", c.repo, t581CodeECRScanOnPushOff, got, c.flagged)
		}
		if got := result.FieldUpdates[c.repo]["scanning"]; got != c.scan {
			t.Errorf("%s: Scan column = %q, want %q", c.repo, got, c.scan)
		}
	}
}

// t581ECRRuleCovers is the test's own reading of the registry rules: a
// filter with a wildcard matches by pattern, one without by substring.
func t581ECRRuleCovers(cfg *ecrtypes.RegistryScanningConfiguration, repo string) bool {
	for _, rule := range cfg.Rules {
		if rule.ScanFrequency == ecrtypes.ScanFrequencyManual {
			continue
		}
		for _, f := range rule.RepositoryFilters {
			filter := aws.ToString(f.Filter)
			if !strings.Contains(filter, "*") {
				if strings.Contains(repo, filter) {
					return true
				}
				continue
			}
			pattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(filter), `\*`, ".*") + "$"
			if regexp.MustCompile(pattern).MatchString(repo) {
				return true
			}
		}
	}
	return false
}

// On the demo bench, which repositories are scanned is decided from the demo
// registry configuration and, in a basic registry without rules, each
// repository's own setting, never from the findings under test; the finding
// must land on exactly the unscanned ones.
func TestT581Review_ECRDemoWitnessFromTheRegistryConfiguration(t *testing.T) {
	clients := demo.NewServiceClients()
	out, err := clients.ECR.(awsclient.ECRGetRegistryScanningConfigurationAPI).GetRegistryScanningConfiguration(context.Background(), &ecr.GetRegistryScanningConfigurationInput{})
	if err != nil || out.ScanningConfiguration == nil {
		t.Fatalf("demo registry scanning configuration: %v", err)
	}
	cfg := out.ScanningConfiguration
	rows, _ := t581LoadAndEnrich(t, "ecr", clients, nil)
	var covered, flagged int
	for _, r := range rows {
		repo, ok := r.RawStruct.(ecrtypes.Repository)
		if !ok {
			continue
		}
		name := aws.ToString(repo.RepositoryName)
		scanned := t581ECRRuleCovers(cfg, name) ||
			(cfg.ScanType != ecrtypes.ScanTypeEnhanced && len(cfg.Rules) == 0 && repo.ImageScanningConfiguration != nil && repo.ImageScanningConfiguration.ScanOnPush)
		if got := hasCode(r.Findings, t581CodeECRScanOnPushOff); got == scanned {
			t.Errorf("%s: scanned=%v by the registry configuration, %s raised=%v", name, scanned, t581CodeECRScanOnPushOff, got)
		}
		if scanned {
			covered++
		} else {
			flagged++
		}
	}
	if covered == 0 || flagged == 0 {
		t.Errorf("demo bench has %d scanned and %d unscanned repositories; it needs a witness of each", covered, flagged)
	}
}

// ─── redis ──────────────────────────────────────────────────────────────

// In preferred mode a replication group accepts cleartext clients beside
// encrypted ones, so transit encryption is not enforced; required mode, and
// a group from before modes existed, encrypt every connection.
func TestT581Review_RedisPreferredTransitModeIsNotEnforced(t *testing.T) {
	preferred := w2RedisGroup("acme-cache-preferred")
	preferred.TransitEncryptionMode = ectypes.TransitEncryptionModePreferred
	required := w2RedisGroup("acme-cache-required")
	required.TransitEncryptionMode = ectypes.TransitEncryptionModeRequired
	got := w2RedisFetch(t, preferred, required, w2RedisGroup("acme-cache-healthy"))
	w2AssertFinding(t, got["acme-cache-preferred"].Findings, w2RedisCodeTransitOff, "encryption in transit not enforced", domain.SevWarn, "wave1")
	for _, id := range []string{"acme-cache-required", "acme-cache-healthy"} {
		w2AssertNoCode(t, got[id].Findings, w2RedisCodeTransitOff)
	}
}

// ─── msk ────────────────────────────────────────────────────────────────

// Broker-to-broker traffic in cleartext is the same exposure as
// client-to-broker: either hop raises the finding, and the row names it.
func TestT581Review_MSKInClusterPlaintextIsNotTLS(t *testing.T) {
	inClusterOff := provisionedCluster(mskARN1, "3.5.1", kafkatypes.ClientBrokerTls)
	inClusterOff.Provisioned.EncryptionInfo.EncryptionInTransit.InCluster = aws.Bool(false)
	both := provisionedCluster(mskARN2, "3.5.1", kafkatypes.ClientBrokerTls)
	both.Provisioned.EncryptionInfo.EncryptionInTransit.InCluster = aws.Bool(true)
	fake := &fakeMSKDescribeClusterV2{Results: map[string]*kafkatypes.Cluster{mskARN1: inClusterOff, mskARN2: both}}
	result, err := awsclient.EnrichMSKCluster(context.Background(), &awsclient.ServiceClients{MSK: fake}, mskClusterResources(mskARN1, mskARN2), nil)
	if err != nil {
		t.Fatalf("EnrichMSKCluster: %v", err)
	}
	const code = domain.FindingCode("msk.encryption-not-tls")
	if !hasCode(result.Findings[mskName1], code) {
		t.Fatalf("%s: InCluster=false raised no %s: %+v", mskName1, code, result.Findings[mskName1])
	}
	w2AssertRow(t, result.AttentionDetails[mskName1][code].Rows, "Broker to broker", "plaintext")
	if hasCode(result.Findings[mskName2], code) {
		t.Errorf("%s: TLS on both hops raised %s", mskName2, code)
	}
}

// ─── elb: network load balancer access logs ─────────────────────────────

// A Network Load Balancer writes access logs only for TLS listeners, so its
// access-log setting matters only when it has one.
func TestT581Review_NLBAccessLogsOnlyWithATLSListener(t *testing.T) {
	tcp := w3ELBRes("acme-nlb-tcp", "network")
	tls := w3ELBRes("acme-nlb-tls", "network")
	fake := w3NewELBFake()
	for _, r := range []resource.Resource{tcp, tls} {
		fake.attrs[r.Fields["load_balancer_arn"]] = w3ELBAttrs(map[string]string{"access_logs.s3.enabled": "false"})
	}
	fake.listeners[tcp.Fields["load_balancer_arn"]] = []elbtypes.Listener{w3Listener("tcp", 5432, elbtypes.ProtocolEnumTcp, "", w3ForwardAction())}
	fake.listeners[tls.Fields["load_balancer_arn"]] = []elbtypes.Listener{w3Listener("tls", 443, elbtypes.ProtocolEnumTls, "ELBSecurityPolicy-TLS13-1-2-2021-06", w3ForwardAction())}
	res := w3RunELBEnrich(t, fake, tcp, tls)
	const code = domain.FindingCode("elb.misconfigured")
	if hasCode(res.Findings[tcp.ID], code) {
		t.Errorf("%s: TCP-only NLB raised %s for access logs it cannot write", tcp.ID, code)
	}
	if !hasCode(res.Findings[tls.ID], code) {
		t.Fatalf("%s: NLB with a TLS listener and access logs off raised no %s", tls.ID, code)
	}
	w2AssertRow(t, res.AttentionDetails[tls.ID][code].Rows, "Access Logs", "disabled")
}

// ─── vpc flow logs ──────────────────────────────────────────────────────

// A VPC is logged by a delivering flow log on the VPC, or by one on every
// subnet; a flow log whose delivery fails writes nothing.
func TestT581Review_VPCFlowLogCoverage(t *testing.T) {
	const vpc = "vpc-0a1b2c3d4e5f60001"
	subnetLog := func(subnet string) ec2types.FlowLog { return activeFlowLog(subnet, "fl-"+subnet) }
	failing := activeFlowLog(vpc, "fl-0a1b2c3d4e5f60001")
	failing.DeliverLogsStatus = aws.String("FAILED")
	failing.DeliverLogsErrorMessage = aws.String("Access error")
	for _, c := range []struct {
		name    string
		logs    []ec2types.FlowLog
		flagged bool
		row     [2]string
	}{
		{"every subnet logged", []ec2types.FlowLog{subnetLog("subnet-a"), subnetLog("subnet-b")}, false, [2]string{}},
		{"one subnet of two logged", []ec2types.FlowLog{subnetLog("subnet-a")}, true, [2]string{"Subnets logged", "1 of 2"}},
		{"VPC flow log delivery failing", []ec2types.FlowLog{failing}, true, [2]string{"Delivery failing", vpc + ": Access error"}},
		{"VPC flow log delivering", []ec2types.FlowLog{activeFlowLog(vpc, "fl-0a1b2c3d4e5f60002")}, false, [2]string{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			resources := vpcResources(vpc)
			resources[0].Fields["subnet_ids"] = "subnet-a,subnet-b"
			fake := &vpcFlowLogFake{results: map[string][]ec2types.FlowLog{vpc: c.logs}}
			result, err := awsclient.EnrichVPCFlowLogs(context.Background(), &awsclient.ServiceClients{EC2: fake}, resources, nil)
			if err != nil {
				t.Fatalf("EnrichVPCFlowLogs: %v", err)
			}
			const code = domain.FindingCode("vpc.no-flow-logs")
			if got := hasCode(result.Findings[vpc], code); got != c.flagged {
				t.Fatalf("%s = %v, want %v: %+v", code, got, c.flagged, result.Findings[vpc])
			}
			if c.flagged {
				w2AssertRow(t, result.AttentionDetails[vpc][code].Rows, c.row[0], c.row[1])
			}
		})
	}
}

// ─── glue ───────────────────────────────────────────────────────────────

type t581GlueSecConfigFake struct {
	*glueJobFake
	configs map[string]gluetypes.SecurityConfiguration
	err     error
	mu      sync.Mutex
	calls   int
}

func (f *t581GlueSecConfigFake) GetSecurityConfiguration(_ context.Context, in *glue.GetSecurityConfigurationInput, _ ...func(*glue.Options)) (*glue.GetSecurityConfigurationOutput, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	cfg, ok := f.configs[aws.ToString(in.Name)]
	if !ok {
		return nil, &gluetypes.EntityNotFoundException{Message: aws.String("Security configuration not found")}
	}
	return &glue.GetSecurityConfigurationOutput{SecurityConfiguration: &cfg}, nil
}

// t581GlueJob is a Spark ETL job, the kind that keeps job bookmarks.
func t581GlueJob(name, secConfig string) resource.Resource {
	return resource.Resource{ID: name, Name: name, RawStruct: gluetypes.Job{
		Name: aws.String(name), SecurityConfiguration: aws.String(secConfig),
		Command: &gluetypes.JobCommand{Name: aws.String("glueetl")},
	}}
}

// A job that names a security configuration is still unencrypted where that
// configuration leaves a target DISABLED; each configuration is read once.
func TestT581Review_GlueSecurityConfigurationModes(t *testing.T) {
	kms := "arn:aws:kms:us-east-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab"
	fake := &t581GlueSecConfigFake{
		glueJobFake: &glueJobFake{jobRuns: map[string]gluetypes.JobRunState{}},
		configs: map[string]gluetypes.SecurityConfiguration{
			"acme-full": {Name: aws.String("acme-full"), EncryptionConfiguration: &gluetypes.EncryptionConfiguration{
				S3Encryption:           []gluetypes.S3Encryption{{S3EncryptionMode: gluetypes.S3EncryptionModeSsekms, KmsKeyArn: aws.String(kms)}},
				CloudWatchEncryption:   &gluetypes.CloudWatchEncryption{CloudWatchEncryptionMode: gluetypes.CloudWatchEncryptionModeSsekms, KmsKeyArn: aws.String(kms)},
				JobBookmarksEncryption: &gluetypes.JobBookmarksEncryption{JobBookmarksEncryptionMode: gluetypes.JobBookmarksEncryptionModeCsekms, KmsKeyArn: aws.String(kms)},
			}},
			"acme-s3-only": {Name: aws.String("acme-s3-only"), EncryptionConfiguration: &gluetypes.EncryptionConfiguration{
				S3Encryption:           []gluetypes.S3Encryption{{S3EncryptionMode: gluetypes.S3EncryptionModeSses3}},
				CloudWatchEncryption:   &gluetypes.CloudWatchEncryption{CloudWatchEncryptionMode: gluetypes.CloudWatchEncryptionModeDisabled},
				JobBookmarksEncryption: &gluetypes.JobBookmarksEncryption{JobBookmarksEncryptionMode: gluetypes.JobBookmarksEncryptionModeDisabled},
			}},
		},
	}
	jobs := []resource.Resource{t581GlueJob("acme-etl-a", "acme-full"), t581GlueJob("acme-etl-b", "acme-s3-only"), t581GlueJob("acme-etl-c", "acme-s3-only")}
	result, err := awsclient.EnrichGlueJobStatus(context.Background(), &awsclient.ServiceClients{Glue: fake}, jobs, nil)
	if err != nil {
		t.Fatalf("EnrichGlueJobStatus: %v", err)
	}
	const code = domain.FindingCode("glue.security-configuration-encryption-off")
	if hasCode(result.Findings["acme-etl-a"], code) {
		t.Errorf("acme-etl-a: every target encrypted raised %s", code)
	}
	for _, id := range []string{"acme-etl-b", "acme-etl-c"} {
		if !hasCode(result.Findings[id], code) {
			t.Fatalf("%s: CloudWatch and bookmarks DISABLED raised no %s", id, code)
		}
		rows := result.AttentionDetails[id][code].Rows
		w2AssertRow(t, rows, "CloudWatch logs", "not encrypted")
		w2AssertRow(t, rows, "Job bookmarks", "not encrypted")
		for _, r := range rows {
			if r.Label == "S3 output" {
				t.Errorf("%s: S3 output is SSE-S3 encrypted, row %+v", id, r)
			}
		}
	}
	if fake.calls != 2 {
		t.Errorf("GetSecurityConfiguration calls = %d, want 2 (one per configuration name)", fake.calls)
	}

	fake.err = t581Refused("glue:GetSecurityConfiguration")
	result, _ = awsclient.EnrichGlueJobStatus(context.Background(), &awsclient.ServiceClients{Glue: fake}, jobs[:1], nil)
	if _, marked := result.TruncatedIDs["acme-etl-a"]; !marked || hasCode(result.Findings["acme-etl-a"], code) {
		t.Errorf("a refused configuration read: marked=%v findings=%+v, want marked and no finding", marked, result.Findings["acme-etl-a"])
	}
}

// ─── transfer ───────────────────────────────────────────────────────────

type t581TransferPolicies struct {
	awsclient.TransferAPI
	err   error
	mu    sync.Mutex
	calls int
}

func (f *t581TransferPolicies) DescribeSecurityPolicy(_ context.Context, in *transfer.DescribeSecurityPolicyInput, _ ...func(*transfer.Options)) (*transfer.DescribeSecurityPolicyOutput, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	p, ok := fixtures.TransferSecurityPolicies[aws.ToString(in.SecurityPolicyName)]
	if !ok {
		return nil, &transfertypes.ResourceNotFoundException{Message: aws.String("not found")}
	}
	return &transfer.DescribeSecurityPolicyOutput{SecurityPolicy: &p}, nil
}

func t581TransferServer(id, policy string) resource.Resource {
	return resource.Resource{ID: id, Name: id, RawStruct: &transfertypes.DescribedServer{
		Arn:                aws.String("arn:aws:transfer:us-east-1:123456789012:server/" + id),
		ServerId:           aws.String(id),
		SecurityPolicyName: aws.String(policy),
	}}
}

// A security policy is weak by the algorithms AWS states it offers, not by
// its name: TransferSecurityPolicy-2020-06 offers no SHA-1 key exchange or
// MAC and no RSA-key-exchange cipher, while the 2018-11 policies do. Each
// policy is read once.
func TestT581Review_TransferPolicyJudgedByItsAlgorithms(t *testing.T) {
	const code = domain.FindingCode("transfer.warn.legacy_policy")
	fake := &t581TransferPolicies{}
	servers := []resource.Resource{
		t581TransferServer("s-0a1b2c3d4e5f60001", "TransferSecurityPolicy-2018-11"),
		t581TransferServer("s-0a1b2c3d4e5f60002", "TransferSecurityPolicy-2018-11"),
		t581TransferServer("s-0a1b2c3d4e5f60003", "TransferSecurityPolicy-Restricted-2018-11"),
		t581TransferServer("s-0a1b2c3d4e5f60004", "TransferSecurityPolicy-2020-06"),
		t581TransferServer("s-0a1b2c3d4e5f60005", "TransferSecurityPolicy-2024-01"),
	}
	result, err := awsclient.EnrichTransferSecurityPolicy(context.Background(), &awsclient.ServiceClients{Transfer: fake}, servers, nil)
	if err != nil {
		t.Fatalf("EnrichTransferSecurityPolicy: %v", err)
	}
	for _, c := range []struct {
		id   string
		weak bool
	}{
		{"s-0a1b2c3d4e5f60001", true},
		{"s-0a1b2c3d4e5f60002", true},
		{"s-0a1b2c3d4e5f60003", true},
		{"s-0a1b2c3d4e5f60004", false},
		{"s-0a1b2c3d4e5f60005", false},
	} {
		if got := hasCode(result.Findings[c.id], code); got != c.weak {
			t.Errorf("%s: %s = %v, want %v", c.id, code, got, c.weak)
		}
	}
	w2AssertRow(t, result.AttentionDetails["s-0a1b2c3d4e5f60001"][code].Rows, "Weak key exchange", "diffie-hellman-group14-sha1")
	if fake.calls != 4 {
		t.Errorf("DescribeSecurityPolicy calls = %d, want 4 (one per policy name)", fake.calls)
	}

	fake.err = t581Refused("transfer:DescribeSecurityPolicy")
	result, err = awsclient.EnrichTransferSecurityPolicy(context.Background(), &awsclient.ServiceClients{Transfer: fake}, servers[:1], nil)
	if err == nil {
		t.Error("a refused policy read returned no enricher error")
	}
	if _, marked := result.TruncatedIDs["s-0a1b2c3d4e5f60001"]; !marked {
		t.Error("a refused policy read left the server inspected")
	}
}
