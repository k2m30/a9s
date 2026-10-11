package unit

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

const (
	t581CodeAthenaNotEnforced = domain.FindingCode("athena.settings-not-enforced")
	t581CodeCFLoggingOff      = domain.FindingCode("cf.logging-off")
	t581CodeWAFNoLogging      = domain.FindingCode("waf.no-logging")
	t581CodeECRScanOnPushOff  = domain.FindingCode("ecr.scan-on-push-off")
	t581CodeECRMutableTags    = domain.FindingCode("ecr.mutable-tags")
	t581CodeLTIMDSv1          = domain.FindingCode("lt.warn.imdsv1")
)

// t581LoadAndEnrich runs a type the way the list does: every page through
// its registered fetcher, then its registered Wave 2 enricher, folded into
// the rows by runtime.ApplyWave2ToRow. The finding a row ends up with does
// not depend on which of the two passes decided it.
func t581LoadAndEnrich(t *testing.T, shortName string, clients *awsclient.ServiceClients, cache resource.ResourceCache) ([]resource.Resource, awsclient.IssueEnricherResult) {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	rows, ok := DrainFixtures(t, *td, clients)
	if !ok {
		t.Fatalf("%s has no fetcher", shortName)
	}
	var result awsclient.IssueEnricherResult
	if enricher, ok := awsclient.Wave2EnricherFor(shortName); ok && enricher.Fn != nil {
		var err error
		result, err = enricher.Fn(context.Background(), clients, rows, cache)
		if err != nil {
			t.Logf("%s enricher error: %v", shortName, err)
		}
	}
	for i := range rows {
		runtime.ApplyWave2ToRow(&rows[i], *td, result.Findings, result.AttentionDetails)
	}
	return rows, result
}

func t581Row(t *testing.T, rows []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no row %q among %d rows", id, len(rows))
	return resource.Resource{}
}

// ─── athena ─────────────────────────────────────────────────────────────

func t581AthenaWG(name string, enforce bool, minimumEncryption *bool) *athenatypes.WorkGroup {
	return &athenatypes.WorkGroup{
		Name:  aws.String(name),
		State: athenatypes.WorkGroupStateEnabled,
		Configuration: &athenatypes.WorkGroupConfiguration{
			EnforceWorkGroupConfiguration:        aws.Bool(enforce),
			EnableMinimumEncryptionConfiguration: minimumEncryption,
			PublishCloudWatchMetricsEnabled:      aws.Bool(true),
			ResultConfiguration: &athenatypes.ResultConfiguration{
				OutputLocation: aws.String("s3://acme-athena-results/" + name + "/"),
				EncryptionConfiguration: &athenatypes.EncryptionConfiguration{
					EncryptionOption: athenatypes.EncryptionOptionSseKms,
					KmsKey:           aws.String("arn:aws:kms:us-east-1:123456789012:key/1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"),
				},
			},
		},
	}
}

// With minimum encryption on, a query can only raise the workgroup's
// encryption, never drop it, so the encryption the workgroup configures binds
// every query even though per-query overrides are allowed. Without it, the
// workgroup's settings are advisory.
func TestT581Athena_MinimumEncryptionMakesUnenforcedWorkgroupBinding(t *testing.T) {
	const (
		minimumOn  = "acme-adhoc"
		minimumOff = "acme-legacy"
		minimumNil = "acme-sandbox"
	)
	fake := &athenaGetWorkGroupFake{results: map[string]*athenatypes.WorkGroup{
		minimumOn:  t581AthenaWG(minimumOn, false, aws.Bool(true)),
		minimumOff: t581AthenaWG(minimumOff, false, aws.Bool(false)),
		minimumNil: t581AthenaWG(minimumNil, false, nil),
	}}
	result, err := awsclient.EnrichAthenaWorkGroup(context.Background(),
		&awsclient.ServiceClients{Athena: fake}, athenaWorkGroupResources(minimumOn, minimumOff, minimumNil), nil)
	if err != nil {
		t.Fatalf("EnrichAthenaWorkGroup: %v", err)
	}
	if hasCode(result.Findings[minimumOn], t581CodeAthenaNotEnforced) {
		t.Errorf("%s: EnforceWorkGroupConfiguration=false with EnableMinimumEncryptionConfiguration=true raised %s; findings=%+v",
			minimumOn, t581CodeAthenaNotEnforced, result.Findings[minimumOn])
	}
	for _, wg := range []string{minimumOff, minimumNil} {
		f, ok := w3FindingByCode(result.Findings[wg], t581CodeAthenaNotEnforced)
		if !ok {
			t.Errorf("%s: EnforceWorkGroupConfiguration=false without minimum encryption raised no %s; findings=%+v",
				wg, t581CodeAthenaNotEnforced, result.Findings[wg])
			continue
		}
		if f.Phrase != "settings can be overridden per query" || f.Severity != domain.SevWarn {
			t.Errorf("%s: finding = %q/%v, want %q/SevWarn", wg, f.Phrase, f.Severity, "settings can be overridden per query")
		}
	}
}

// ─── cf ─────────────────────────────────────────────────────────────────

// t581LogsDeliveryFake answers the CloudWatch Logs vended-log reads for
// standard logging (v2): deliveries name their source, and a source names
// the resource ARN it logs for.
type t581LogsDeliveryFake struct {
	awsclient.CWLogsAPI
	sources    []cwltypes.DeliverySource
	deliveries []cwltypes.Delivery
	err        error
}

func (f *t581LogsDeliveryFake) DescribeDeliveries(_ context.Context, _ *cloudwatchlogs.DescribeDeliveriesInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeDeliveriesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &cloudwatchlogs.DescribeDeliveriesOutput{Deliveries: f.deliveries}, nil
}

func (f *t581LogsDeliveryFake) GetDeliverySource(_ context.Context, in *cloudwatchlogs.GetDeliverySourceInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetDeliverySourceOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, s := range f.sources {
		if aws.ToString(s.Name) == aws.ToString(in.Name) {
			return &cloudwatchlogs.GetDeliverySourceOutput{DeliverySource: &s}, nil
		}
	}
	return nil, &cwltypes.ResourceNotFoundException{Message: aws.String("delivery source " + aws.ToString(in.Name) + " not found")}
}

func (f *t581LogsDeliveryFake) DescribeDeliverySources(_ context.Context, _ *cloudwatchlogs.DescribeDeliverySourcesInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeDeliverySourcesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &cloudwatchlogs.DescribeDeliverySourcesOutput{DeliverySources: f.sources}, nil
}

func t581CFArn(id string) string {
	return "arn:aws:cloudfront::123456789012:distribution/" + id
}

func t581CFSource(id string) cwltypes.DeliverySource {
	return cwltypes.DeliverySource{
		Arn:          aws.String("arn:aws:logs:us-east-1:123456789012:delivery-source:cf-" + id),
		Name:         aws.String("cf-" + id),
		LogType:      aws.String("ACCESS_LOGS"),
		Service:      aws.String("cloudfront"),
		ResourceArns: []string{t581CFArn(id)},
	}
}

func t581CFDelivery(id string) cwltypes.Delivery {
	return cwltypes.Delivery{
		Id:                      aws.String("Ab1Cd2Ef3Gh4" + id[len(id)-1:]),
		Arn:                     aws.String("arn:aws:logs:us-east-1:123456789012:delivery:Ab1Cd2Ef3Gh4" + id[len(id)-1:]),
		DeliverySourceName:      aws.String("cf-" + id),
		DeliveryDestinationArn:  aws.String("arn:aws:logs:us-east-1:123456789012:delivery-destination:acme-cdn-logs-s3"),
		DeliveryDestinationType: cwltypes.DeliveryDestinationTypeS3,
	}
}

// t581CFLegacyLoggingOff is a distribution healthy on every other cf check,
// with the legacy logging block CloudFront returns once logging is off.
func t581CFLegacyLoggingOff(id string) *cftypes.DistributionConfig {
	cfg := cfDistroConfigRedirectHTTPS(id)
	cfg.Logging = &cftypes.LoggingConfig{
		Enabled:        aws.Bool(false),
		IncludeCookies: aws.Bool(false),
		Bucket:         aws.String(""),
		Prefix:         aws.String(""),
	}
	return cfg
}

func t581CFResources(ids ...string) []resource.Resource {
	res := cfDistroResources(ids...)
	for i := range res {
		res[i].Fields["arn"] = t581CFArn(res[i].ID)
	}
	return res
}

// Standard logging (v2) is configured in CloudWatch Logs, not on the
// distribution, so a distribution with the legacy block off still logs every
// request when a v2 delivery names it. A delivery source with no delivery
// attached sends nothing anywhere.
func TestT581CF_StandardLoggingV2DeliveryIsAccessLogging(t *testing.T) {
	const (
		v2Logged     = "EDFDVBD6EXAMPLEA"
		sourceOnly   = "EDFDVBD6EXAMPLEB"
		neither      = "EDFDVBD6EXAMPLEC"
		otherAccount = "EDFDVBD6EXAMPLED"
	)
	cf := &cfGetDistributionConfigFake{results: map[string]*cftypes.DistributionConfig{
		v2Logged:   t581CFLegacyLoggingOff(v2Logged),
		sourceOnly: t581CFLegacyLoggingOff(sourceOnly),
		neither:    t581CFLegacyLoggingOff(neither),
	}}
	logs := &t581LogsDeliveryFake{
		sources:    []cwltypes.DeliverySource{t581CFSource(v2Logged), t581CFSource(sourceOnly), t581CFSource(otherAccount)},
		deliveries: []cwltypes.Delivery{t581CFDelivery(v2Logged), t581CFDelivery(otherAccount)},
	}
	clients := &awsclient.ServiceClients{CloudFront: cf, CloudWatchLogs: logs}

	result, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients, t581CFResources(v2Logged, sourceOnly, neither), nil)
	if err != nil {
		t.Fatalf("EnrichCloudFrontDistribution: %v", err)
	}
	if hasCode(result.Findings[v2Logged], t581CodeCFLoggingOff) {
		t.Errorf("%s: legacy logging off with a standard logging v2 delivery raised %s; findings=%+v",
			v2Logged, t581CodeCFLoggingOff, result.Findings[v2Logged])
	}
	if _, marked := result.TruncatedIDs[v2Logged]; marked {
		t.Errorf("%s: a distribution whose v2 delivery was read is marked not inspected (%q)", v2Logged, result.TruncatedIDs[v2Logged])
	}
	for _, id := range []string{sourceOnly, neither} {
		if !hasCode(result.Findings[id], t581CodeCFLoggingOff) {
			t.Errorf("%s: no legacy logging and no v2 delivery raised no %s; findings=%+v", id, t581CodeCFLoggingOff, result.Findings[id])
		}
	}
}

// A refused CloudWatch Logs read says nothing about whether a v2 delivery
// exists, so a distribution with the legacy block off is not inspected,
// never "access logging off".
func TestT581CF_FailedV2ReadLeavesLoggingNotInspected(t *testing.T) {
	const id = "EDFDVBD6EXAMPLEE"
	cf := &cfGetDistributionConfigFake{results: map[string]*cftypes.DistributionConfig{id: t581CFLegacyLoggingOff(id)}}
	logs := &t581LogsDeliveryFake{err: &smithy.GenericAPIError{
		Code:    "AccessDeniedException",
		Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform: logs:DescribeDeliveries",
	}}
	clients := &awsclient.ServiceClients{CloudFront: cf, CloudWatchLogs: logs}

	result, err := awsclient.EnrichCloudFrontDistribution(context.Background(), clients, t581CFResources(id), nil)
	t.Logf("enricher error: %v", err)
	if hasCode(result.Findings[id], t581CodeCFLoggingOff) {
		t.Errorf("a refused v2 delivery read raised %s; findings=%+v", t581CodeCFLoggingOff, result.Findings[id])
	}
	if _, marked := result.TruncatedIDs[id]; !marked {
		t.Errorf("a refused v2 delivery read left %s inspected; TruncatedIDs=%v", id, result.TruncatedIDs)
	}
}

// ─── waf ────────────────────────────────────────────────────────────────

// t581WAFScopedLoggingFake keeps one logging configuration per log scope, as
// WAF does: GetLoggingConfiguration and ListLoggingConfigurations answer the
// CUSTOMER scope unless the request names another.
type t581WAFScopedLoggingFake struct {
	*wafLoggingFake
	logScopes map[string][]wafv2types.LogScope
}

func t581WAFConfig(arn string, scope wafv2types.LogScope) wafv2types.LoggingConfiguration {
	return wafv2types.LoggingConfiguration{
		ResourceArn:           aws.String(arn),
		LogScope:              scope,
		LogType:               wafv2types.LogTypeWafLogs,
		LogDestinationConfigs: []string{"arn:aws:logs:us-east-1:123456789012:log-group:aws-waf-logs-acme"},
	}
}

func (f *t581WAFScopedLoggingFake) GetLoggingConfiguration(_ context.Context, in *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error) {
	scope := t581LogScope(in.LogScope)
	arn := aws.ToString(in.ResourceArn)
	if slices.Contains(f.logScopes[arn], scope) {
		cfg := t581WAFConfig(arn, scope)
		return &wafv2.GetLoggingConfigurationOutput{LoggingConfiguration: &cfg}, nil
	}
	return nil, &wafv2types.WAFNonexistentItemException{Message: aws.String("AWS WAF couldn't perform the operation because your resource doesn't exist.")}
}

func (f *t581WAFScopedLoggingFake) ListLoggingConfigurations(_ context.Context, in *wafv2.ListLoggingConfigurationsInput, _ ...func(*wafv2.Options)) (*wafv2.ListLoggingConfigurationsOutput, error) {
	scope := t581LogScope(in.LogScope)
	var out []wafv2types.LoggingConfiguration
	for arn, scopes := range f.logScopes {
		if slices.Contains(scopes, scope) {
			out = append(out, t581WAFConfig(arn, scope))
		}
	}
	return &wafv2.ListLoggingConfigurationsOutput{LoggingConfigurations: out}, nil
}

func t581LogScope(s wafv2types.LogScope) wafv2types.LogScope {
	if s == "" {
		return wafv2types.LogScopeCustomer
	}
	return s
}

// A web ACL whose logs are collected by Security Lake or by a CloudWatch
// telemetry rule is logged: WAF keeps that configuration under its own log
// scope, so the CUSTOMER scope alone answers "not found" for it.
func TestT581WAF_ManagedLogScopesAreLogging(t *testing.T) {
	const (
		securityLake = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-lake/aaaa1111-2222-3333-4444-555555555555"
		telemetry    = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-telemetry/bbbb1111-2222-3333-4444-555555555555"
		customer     = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-customer/cccc1111-2222-3333-4444-555555555555"
		unlogged     = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-unlogged/dddd1111-2222-3333-4444-555555555555"
	)
	alb := wafResourcesOutput("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-web/50dc6c495c0c9188")
	fake := &t581WAFScopedLoggingFake{
		wafLoggingFake: &wafLoggingFake{resourcesResults: map[string]*wafv2.ListResourcesForWebACLOutput{
			securityLake: alb, telemetry: alb, customer: alb, unlogged: alb,
		}},
		logScopes: map[string][]wafv2types.LogScope{
			securityLake: {wafv2types.LogScopeSecurityLake},
			telemetry:    {wafv2types.LogScopeCloudwatchTelemetryRuleManaged},
			customer:     {wafv2types.LogScopeCustomer},
		},
	}
	result, err := awsclient.EnrichWAFLogging(context.Background(),
		&awsclient.ServiceClients{WAFv2: fake}, wafWebACLResources(securityLake, telemetry, customer, unlogged), nil)
	if err != nil {
		t.Fatalf("EnrichWAFLogging: %v", err)
	}
	for _, arn := range []string{securityLake, telemetry, customer} {
		if hasCode(result.Findings[arn], t581CodeWAFNoLogging) {
			t.Errorf("%s logged under scope %v raised %s", arn, fake.logScopes[arn], t581CodeWAFNoLogging)
		}
		if _, marked := result.TruncatedIDs[arn]; marked {
			t.Errorf("%s is marked not inspected (%q)", arn, result.TruncatedIDs[arn])
		}
	}
	if !hasCode(result.Findings[unlogged], t581CodeWAFNoLogging) {
		t.Errorf("a web ACL with no logging configuration in any scope raised no %s; findings=%+v", t581CodeWAFNoLogging, result.Findings[unlogged])
	}
}

// ─── ecr ────────────────────────────────────────────────────────────────

// t581ECRWrapper serves the demo registry with repository settings and a
// registry scanning configuration of the test's choosing; every other ECR
// read is the demo's own answer.
type t581ECRWrapper struct {
	awsclient.ECRAPI
	mutate   map[string]func(*ecrtypes.Repository)
	scanning *ecrtypes.RegistryScanningConfiguration

	mu            sync.Mutex
	registryCalls int
}

func (w *t581ECRWrapper) DescribeRepositories(ctx context.Context, in *ecr.DescribeRepositoriesInput, opts ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
	out, err := w.ECRAPI.DescribeRepositories(ctx, in, opts...)
	if err != nil {
		return out, err
	}
	cp := *out
	cp.Repositories = slices.Clone(out.Repositories)
	for i := range cp.Repositories {
		if c := cp.Repositories[i].ImageScanningConfiguration; c != nil {
			scan := *c
			cp.Repositories[i].ImageScanningConfiguration = &scan
		}
		if m := w.mutate[aws.ToString(cp.Repositories[i].RepositoryName)]; m != nil {
			m(&cp.Repositories[i])
		}
	}
	return &cp, nil
}

func (w *t581ECRWrapper) GetRegistryScanningConfiguration(_ context.Context, _ *ecr.GetRegistryScanningConfigurationInput, _ ...func(*ecr.Options)) (*ecr.GetRegistryScanningConfigurationOutput, error) {
	w.mu.Lock()
	w.registryCalls++
	w.mu.Unlock()
	return &ecr.GetRegistryScanningConfigurationOutput{
		RegistryId:            aws.String("123456789012"),
		ScanningConfiguration: w.scanning,
	}, nil
}

func (w *t581ECRWrapper) GetRepositoryPolicy(ctx context.Context, in *ecr.GetRepositoryPolicyInput, opts ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error) {
	return w.ECRAPI.(awsclient.ECRGetRepositoryPolicyAPI).GetRepositoryPolicy(ctx, in, opts...)
}

func (w *t581ECRWrapper) GetLifecyclePolicy(ctx context.Context, in *ecr.GetLifecyclePolicyInput, opts ...func(*ecr.Options)) (*ecr.GetLifecyclePolicyOutput, error) {
	return w.ECRAPI.(awsclient.ECRGetLifecyclePolicyAPI).GetLifecyclePolicy(ctx, in, opts...)
}

func (w *t581ECRWrapper) DescribeImageScanFindings(ctx context.Context, in *ecr.DescribeImageScanFindingsInput, opts ...func(*ecr.Options)) (*ecr.DescribeImageScanFindingsOutput, error) {
	return w.ECRAPI.(awsclient.ECRDescribeImageScanFindingsAPI).DescribeImageScanFindings(ctx, in, opts...)
}

func t581ScanOnPushOff(r *ecrtypes.Repository) {
	r.ImageScanningConfiguration = &ecrtypes.ImageScanningConfiguration{ScanOnPush: false}
}

func t581ScanRule(freq ecrtypes.ScanFrequency, filter string) ecrtypes.RegistryScanningRule {
	return ecrtypes.RegistryScanningRule{
		ScanFrequency: freq,
		RepositoryFilters: []ecrtypes.ScanningRepositoryFilter{{
			Filter:     aws.String(filter),
			FilterType: ecrtypes.ScanningRepositoryFilterTypeWildcard,
		}},
	}
}

func t581ECRRun(t *testing.T, w *t581ECRWrapper) []resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	w.ECRAPI = clients.ECR
	clients.ECR = w
	rows, _ := t581LoadAndEnrich(t, "ecr", clients, nil)
	return rows
}

// A registry scanning rule that scans on push, or continuously, covers every
// repository its filter matches whatever the repository's own ScanOnPush
// says. A rule whose frequency is MANUAL scans nothing on push, and once the
// registry has rules, a repository no rule matches is set to manual scanning.
func TestT581ECR_RegistryScanRuleCoversRepositoryScanOnPush(t *testing.T) {
	const (
		apiService = "acme/api-service"
		batch      = "acme/batch-processor"
		baseImages = fixtures.ECRScanOnPushOff
	)
	t.Run("basic scan-on-push rule", func(t *testing.T) {
		w := &t581ECRWrapper{
			mutate: map[string]func(*ecrtypes.Repository){apiService: t581ScanOnPushOff, batch: t581ScanOnPushOff},
			scanning: &ecrtypes.RegistryScanningConfiguration{
				ScanType: ecrtypes.ScanTypeBasic,
				Rules: []ecrtypes.RegistryScanningRule{
					t581ScanRule(ecrtypes.ScanFrequencyScanOnPush, "acme/api-*"),
					t581ScanRule(ecrtypes.ScanFrequencyManual, "acme/batch-*"),
				},
			},
		}
		rows := t581ECRRun(t, w)
		if r := t581Row(t, rows, apiService); hasCode(r.Findings, t581CodeECRScanOnPushOff) {
			t.Errorf("%s: ScanOnPush=false under a matching SCAN_ON_PUSH registry rule raised %s", apiService, t581CodeECRScanOnPushOff)
		}
		for _, name := range []string{batch, baseImages} {
			if r := t581Row(t, rows, name); !hasCode(r.Findings, t581CodeECRScanOnPushOff) {
				t.Errorf("%s: ScanOnPush=false with no scanning rule covering it raised no %s; findings=%+v", name, t581CodeECRScanOnPushOff, r.Findings)
			}
		}
		if w.registryCalls != 1 {
			t.Errorf("GetRegistryScanningConfiguration called %d times for one registry, want 1", w.registryCalls)
		}
	})
	t.Run("enhanced continuous rule", func(t *testing.T) {
		w := &t581ECRWrapper{
			mutate: map[string]func(*ecrtypes.Repository){apiService: t581ScanOnPushOff},
			scanning: &ecrtypes.RegistryScanningConfiguration{
				ScanType: ecrtypes.ScanTypeEnhanced,
				Rules:    []ecrtypes.RegistryScanningRule{t581ScanRule(ecrtypes.ScanFrequencyContinuousScan, baseImages)},
			},
		}
		rows := t581ECRRun(t, w)
		if r := t581Row(t, rows, baseImages); hasCode(r.Findings, t581CodeECRScanOnPushOff) {
			t.Errorf("%s: ScanOnPush=false under a matching CONTINUOUS_SCAN registry rule raised %s", baseImages, t581CodeECRScanOnPushOff)
		}
		if r := t581Row(t, rows, apiService); !hasCode(r.Findings, t581CodeECRScanOnPushOff) {
			t.Errorf("%s: ScanOnPush=false that no rule matches raised no %s; findings=%+v", apiService, t581CodeECRScanOnPushOff, r.Findings)
		}
	})
}

// MUTABLE_WITH_EXCLUSION leaves every tag mutable except the excluded ones,
// so the tags a deployment names can still move; IMMUTABLE_WITH_EXCLUSION
// pins every tag except the excluded ones.
func TestT581ECR_MutableWithExclusionIsMutableTags(t *testing.T) {
	const (
		mutableExcl   = "acme/frontend"
		immutableExcl = "acme/api-service"
	)
	w := &t581ECRWrapper{
		mutate: map[string]func(*ecrtypes.Repository){
			mutableExcl: func(r *ecrtypes.Repository) {
				r.ImageTagMutability = ecrtypes.ImageTagMutabilityMutableWithExclusion
				r.ImageTagMutabilityExclusionFilters = []ecrtypes.ImageTagMutabilityExclusionFilter{{
					Filter: aws.String("release-*"), FilterType: ecrtypes.ImageTagMutabilityExclusionFilterTypeWildcard,
				}}
			},
			immutableExcl: func(r *ecrtypes.Repository) {
				r.ImageTagMutability = ecrtypes.ImageTagMutabilityImmutableWithExclusion
				r.ImageTagMutabilityExclusionFilters = []ecrtypes.ImageTagMutabilityExclusionFilter{{
					Filter: aws.String("latest"), FilterType: ecrtypes.ImageTagMutabilityExclusionFilterTypeWildcard,
				}}
			},
		},
		scanning: &ecrtypes.RegistryScanningConfiguration{ScanType: ecrtypes.ScanTypeBasic},
	}
	rows := t581ECRRun(t, w)
	r := t581Row(t, rows, mutableExcl)
	f, ok := w3FindingByCode(r.Findings, t581CodeECRMutableTags)
	if !ok {
		t.Errorf("%s: MUTABLE_WITH_EXCLUSION raised no %s; findings=%+v", mutableExcl, t581CodeECRMutableTags, r.Findings)
	} else if f.Phrase != "tags are mutable" {
		t.Errorf("%s: phrase = %q, want %q", mutableExcl, f.Phrase, "tags are mutable")
	}
	if r := t581Row(t, rows, immutableExcl); hasCode(r.Findings, t581CodeECRMutableTags) {
		t.Errorf("%s: IMMUTABLE_WITH_EXCLUSION raised %s", immutableExcl, t581CodeECRMutableTags)
	}
}

// ─── elb ────────────────────────────────────────────────────────────────

// The TLS13-1-0 and TLS13-1-1 families add TLS 1.3 on top of TLS 1.0/1.1
// rather than replacing them (ALB security policy table), so a listener on
// one still negotiates a deprecated protocol. The TLS13-1-2 and TLS13-1-3
// families do not.
func TestT581ELB_TLS13PoliciesThatStillAllowTLS10Or11AreWeak(t *testing.T) {
	for _, policy := range []string{
		"ELBSecurityPolicy-TLS13-1-0-2021-06",
		"ELBSecurityPolicy-TLS13-1-1-2021-06",
		"ELBSecurityPolicy-TLS13-1-0-PQ-2025-09",
		"ELBSecurityPolicy-TLS13-1-0-FIPS-2023-04",
		"ELBSecurityPolicy-TLS13-1-1-FIPS-2023-04",
		"ELBSecurityPolicy-TLS13-1-0-FIPS-PQ-2025-09",
	} {
		for _, lb := range []struct {
			lbType string
			proto  elbtypes.ProtocolEnum
		}{{"application", elbtypes.ProtocolEnumHttps}, {"network", elbtypes.ProtocolEnumTls}} {
			t.Run(policy+"/"+lb.lbType, func(t *testing.T) {
				r := w3ELBRes("acme-edge", lb.lbType)
				fake := w3NewELBFake()
				arn := r.Fields["load_balancer_arn"]
				fake.attrs[arn] = w3ELBAttrs(nil)
				fake.listeners[arn] = []elbtypes.Listener{w3Listener("tls", 443, lb.proto, policy, w3ForwardAction())}
				res := w3RunELBEnrich(t, fake, r)
				f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBWeakTLS)
				if !ok {
					t.Fatalf("SslPolicy %q raised no %s; findings=%+v", policy, w3CodeELBWeakTLS, res.Findings[r.ID])
				}
				if want := "weak TLS policy on port 443"; f.Phrase != want {
					t.Errorf("Phrase = %q, want %q", f.Phrase, want)
				}
				w3AssertRows(t, res.AttentionDetails[r.ID][w3CodeELBWeakTLS].Rows, [][2]string{{"Security policy", "443: " + policy}})
			})
		}
	}
	for _, policy := range []string{
		"ELBSecurityPolicy-TLS13-1-2-2021-06",
		"ELBSecurityPolicy-TLS13-1-2-Res-PQ-2025-09",
		"ELBSecurityPolicy-TLS13-1-3-2021-06",
	} {
		t.Run(policy, func(t *testing.T) {
			r := w3ELBRes("acme-edge", "application")
			fake := w3NewELBFake()
			arn := r.Fields["load_balancer_arn"]
			fake.attrs[arn] = w3ELBAttrs(nil)
			fake.listeners[arn] = []elbtypes.Listener{w3Listener("https", 443, elbtypes.ProtocolEnumHttps, policy, w3ForwardAction())}
			res := w3RunELBEnrich(t, fake, r)
			if f, ok := w3FindingByCode(res.Findings[r.ID], w3CodeELBWeakTLS); ok {
				t.Errorf("SslPolicy %q raised %s (%q), want no finding", policy, f.Code, f.Phrase)
			}
		})
	}
}

// ─── lt ─────────────────────────────────────────────────────────────────

type t581LTEC2Fake struct {
	awsclient.EC2API
	templates []ec2types.LaunchTemplate
	versions  map[string]ec2types.LaunchTemplateVersion
	images    map[string]ec2types.Image
	imagesErr error

	mu         sync.Mutex
	imageCalls int
}

func (f *t581LTEC2Fake) DescribeLaunchTemplates(_ context.Context, _ *ec2.DescribeLaunchTemplatesInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error) {
	return &ec2.DescribeLaunchTemplatesOutput{LaunchTemplates: f.templates}, nil
}

func (f *t581LTEC2Fake) DescribeLaunchTemplateVersions(_ context.Context, in *ec2.DescribeLaunchTemplateVersionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	v, ok := f.versions[aws.ToString(in.LaunchTemplateId)]
	if !ok {
		return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
	}
	return &ec2.DescribeLaunchTemplateVersionsOutput{LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{v}}, nil
}

// DescribeImages answers by ImageIds or an image-id filter. An id it does not
// hold is absent from the answer, as a deregistered or unshared AMI is.
func (f *t581LTEC2Fake) DescribeImages(_ context.Context, in *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	f.mu.Lock()
	f.imageCalls++
	f.mu.Unlock()
	if f.imagesErr != nil {
		return nil, f.imagesErr
	}
	ids := slices.Clone(in.ImageIds)
	for _, flt := range in.Filters {
		if aws.ToString(flt.Name) == "image-id" {
			ids = append(ids, flt.Values...)
		}
	}
	out := &ec2.DescribeImagesOutput{}
	for _, id := range ids {
		if img, ok := f.images[id]; ok {
			out.Images = append(out.Images, img)
		}
	}
	return out, nil
}

func t581LT(id, name string) ec2types.LaunchTemplate {
	created := time.Date(2025, 3, 14, 9, 30, 0, 0, time.UTC)
	return ec2types.LaunchTemplate{
		LaunchTemplateId:     aws.String(id),
		LaunchTemplateName:   aws.String(name),
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(1),
		CreatedBy:            aws.String("arn:aws:iam::123456789012:role/example-readonly"),
		CreateTime:           &created,
	}
}

func t581LTVersion(id, imageID string, md *ec2types.LaunchTemplateInstanceMetadataOptions) ec2types.LaunchTemplateVersion {
	return ec2types.LaunchTemplateVersion{
		LaunchTemplateId: aws.String(id),
		VersionNumber:    aws.Int64(1),
		DefaultVersion:   aws.Bool(true),
		LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
			ImageId:         aws.String(imageID),
			InstanceType:    ec2types.InstanceTypeT3Medium,
			MetadataOptions: md,
		},
	}
}

func t581AMI(id string, imds ec2types.ImdsSupportValues) ec2types.Image {
	return ec2types.Image{
		ImageId:      aws.String(id),
		Name:         aws.String("acme-base-" + id),
		OwnerId:      aws.String("123456789012"),
		State:        ec2types.ImageStateAvailable,
		ImdsSupport:  imds,
		Architecture: ec2types.ArchitectureValuesX8664,
	}
}

// The ami list is loaded and complete, so the deprecated-AMI scan has nothing
// to mark: a template is not inspected only when its own AMI read fails.
func t581LoadedAMICache() resource.ResourceCache {
	return resource.ResourceCache{"ami": {Resources: []resource.Resource{}, IsTruncated: false}}
}

const (
	t581AMIv2      = "ami-0a1b2c3d4e5f60001"
	t581AMIDefault = "ami-0a1b2c3d4e5f60002"
	t581AMIGone    = "ami-0a1b2c3d4e5f60003"
)

// A launch template that sets no metadata options launches with the AMI's
// own default: an AMI registered with ImdsSupport v2.0 requires IMDSv2, one
// without it allows IMDSv1. An explicit HttpTokens=optional on the template
// overrides the AMI. An AMI nobody can read leaves the answer unknown.
func TestT581LT_AMIImdsSupportDecidesUnsetMetadataOptions(t *testing.T) {
	const (
		onV2AMI       = "lt-0a1b2c3d4e5f60001"
		onDefaultAMI  = "lt-0a1b2c3d4e5f60002"
		onGoneAMI     = "lt-0a1b2c3d4e5f60003"
		explicitOptIn = "lt-0a1b2c3d4e5f60004"
	)
	fake := &t581LTEC2Fake{
		templates: []ec2types.LaunchTemplate{
			t581LT(onV2AMI, "acme-web"), t581LT(onDefaultAMI, "acme-batch"),
			t581LT(onGoneAMI, "acme-legacy"), t581LT(explicitOptIn, "acme-worker"),
		},
		versions: map[string]ec2types.LaunchTemplateVersion{
			onV2AMI:      t581LTVersion(onV2AMI, t581AMIv2, nil),
			onDefaultAMI: t581LTVersion(onDefaultAMI, t581AMIDefault, nil),
			onGoneAMI:    t581LTVersion(onGoneAMI, t581AMIGone, nil),
			explicitOptIn: t581LTVersion(explicitOptIn, t581AMIv2, &ec2types.LaunchTemplateInstanceMetadataOptions{
				HttpEndpoint: ec2types.LaunchTemplateInstanceMetadataEndpointStateEnabled,
				HttpTokens:   ec2types.LaunchTemplateHttpTokensStateOptional,
			}),
		},
		images: map[string]ec2types.Image{
			t581AMIv2:      t581AMI(t581AMIv2, ec2types.ImdsSupportValuesV20),
			t581AMIDefault: t581AMI(t581AMIDefault, ""),
		},
	}
	rows, result := t581LoadAndEnrich(t, "lt", &awsclient.ServiceClients{EC2: fake}, t581LoadedAMICache())

	if r := t581Row(t, rows, onV2AMI); hasCode(r.Findings, t581CodeLTIMDSv1) {
		t.Errorf("%s: no MetadataOptions on an ImdsSupport=v2.0 AMI raised %s", onV2AMI, t581CodeLTIMDSv1)
	}
	if _, marked := result.TruncatedIDs[onV2AMI]; marked {
		t.Errorf("%s: a template whose AMI was read is marked not inspected (%q)", onV2AMI, result.TruncatedIDs[onV2AMI])
	}
	for _, id := range []string{onDefaultAMI, explicitOptIn} {
		if r := t581Row(t, rows, id); !hasCode(r.Findings, t581CodeLTIMDSv1) {
			t.Errorf("%s: IMDSv1 allowed raised no %s; findings=%+v", id, t581CodeLTIMDSv1, r.Findings)
		}
	}
	if r := t581Row(t, rows, onGoneAMI); hasCode(r.Findings, t581CodeLTIMDSv1) {
		t.Errorf("%s: an AMI DescribeImages does not return raised %s instead of leaving it unknown", onGoneAMI, t581CodeLTIMDSv1)
	}
	if _, marked := result.TruncatedIDs[onGoneAMI]; !marked {
		t.Errorf("%s: an unresolvable AMI left the template inspected; TruncatedIDs=%v", onGoneAMI, result.TruncatedIDs)
	}
	if fake.imageCalls != 1 {
		t.Errorf("DescribeImages called %d times for three AMIs, want one batched call", fake.imageCalls)
	}
}

// A refused DescribeImages answers nothing about the AMI's default, so a
// template that sets no metadata options is not inspected, never "IMDSv1
// allowed".
func TestT581LT_RefusedAMIReadLeavesIMDSNotInspected(t *testing.T) {
	const id = "lt-0a1b2c3d4e5f60005"
	fake := &t581LTEC2Fake{
		templates: []ec2types.LaunchTemplate{t581LT(id, "acme-web")},
		versions:  map[string]ec2types.LaunchTemplateVersion{id: t581LTVersion(id, t581AMIv2, nil)},
		imagesErr: &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "You are not authorized to perform this operation."},
	}
	rows, result := t581LoadAndEnrich(t, "lt", &awsclient.ServiceClients{EC2: fake}, t581LoadedAMICache())
	if r := t581Row(t, rows, id); hasCode(r.Findings, t581CodeLTIMDSv1) {
		t.Errorf("a refused DescribeImages raised %s", t581CodeLTIMDSv1)
	}
	if _, marked := result.TruncatedIDs[id]; !marked {
		t.Errorf("a refused DescribeImages left %s inspected; TruncatedIDs=%v", id, result.TruncatedIDs)
	}
}

// The lt spec states where an unset HttpTokens takes its value from.
func TestT581LT_SpecNamesTheAMIDefault(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "resources", "lt.md"))
	if err != nil {
		t.Fatalf("read lt.md: %v", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if strings.HasPrefix(line, "- **Signal**: `MetadataOptions.HttpTokens") {
			if !strings.Contains(line, "ImdsSupport") {
				t.Errorf("the IMDSv1 signal bullet does not name the AMI's ImdsSupport default:\n%s", line)
			}
			return
		}
	}
	t.Fatal("no IMDSv1 signal bullet in docs/resources/lt.md")
}
