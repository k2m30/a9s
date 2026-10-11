package unit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func t581HasCode(r resource.Resource, code domain.FindingCode) bool {
	return slices.ContainsFunc(r.Findings, func(f domain.Finding) bool { return f.Code == code })
}

// Every type in this batch has, on the demo bench, a row whose alternative
// setting satisfies the rule (and so must stay clean) and a row the rule
// flags, rendered through the app's own list and detail code.
func TestT581Demo_SatisfyingSettingWitnesses(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	ctx := context.Background()
	merged := func(t *testing.T, shortName string) (resource.ResourceTypeDef, []resource.Resource) {
		t.Helper()
		td := resource.FindResourceType(shortName)
		if td == nil || len(byType[shortName]) == 0 {
			t.Fatalf("%s: no demo rows", shortName)
		}
		return *td, mergeWave2Findings(t, *td, byType[shortName], cache, clients)
	}
	witness := func(t *testing.T, td resource.ResourceTypeDef, rows []resource.Resource, satisfied, flagged []resource.Resource, code domain.FindingCode) {
		t.Helper()
		if len(satisfied) == 0 {
			t.Errorf("%s: no demo row satisfies the rule through its alternative setting", td.ShortName)
		}
		for _, r := range satisfied {
			if t581HasCode(r, code) {
				t.Errorf("%s/%s satisfies the rule but carries %s", td.ShortName, r.ID, code)
			}
		}
		if len(flagged) == 0 {
			t.Errorf("%s: no demo row the rule flags", td.ShortName)
		}
		for _, r := range flagged {
			if !t581HasCode(r, code) {
				t.Errorf("%s/%s fails the rule but carries no %s; findings=%+v", td.ShortName, r.ID, code, r.Findings)
				continue
			}
			t563AssertWitnessRendered(t, td, rows, r)
		}
	}

	t.Run("athena", func(t *testing.T) {
		td, rows := merged(t, "athena")
		var satisfied, flagged []resource.Resource
		for _, r := range rows {
			out, err := clients.Athena.GetWorkGroup(ctx, &athena.GetWorkGroupInput{WorkGroup: aws.String(r.ID)})
			if err != nil || out.WorkGroup == nil || out.WorkGroup.Configuration == nil {
				continue
			}
			cfg := out.WorkGroup.Configuration
			if cfg.EnforceWorkGroupConfiguration == nil || *cfg.EnforceWorkGroupConfiguration {
				continue
			}
			if aws.ToBool(cfg.EnableMinimumEncryptionConfiguration) {
				satisfied = append(satisfied, r)
			} else {
				flagged = append(flagged, r)
			}
		}
		witness(t, td, rows, satisfied, flagged, "athena.settings-not-enforced")
	})

	t.Run("cf", func(t *testing.T) {
		td, rows := merged(t, "cf")
		logs, ok := clients.CloudWatchLogs.(interface {
			DescribeDeliveries(context.Context, *cloudwatchlogs.DescribeDeliveriesInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeDeliveriesOutput, error)
			GetDeliverySource(context.Context, *cloudwatchlogs.GetDeliverySourceInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetDeliverySourceOutput, error)
		})
		if !ok {
			t.Fatal("the demo CloudWatch Logs client answers no DescribeDeliveries/GetDeliverySource")
		}
		delivered := map[string]bool{}
		out, err := logs.DescribeDeliveries(ctx, &cloudwatchlogs.DescribeDeliveriesInput{})
		if err != nil {
			t.Fatalf("demo DescribeDeliveries: %v", err)
		}
		for _, d := range out.Deliveries {
			src, err := logs.GetDeliverySource(ctx, &cloudwatchlogs.GetDeliverySourceInput{Name: d.DeliverySourceName})
			if err != nil || src.DeliverySource == nil {
				continue
			}
			for _, arn := range src.DeliverySource.ResourceArns {
				delivered[arn] = true
			}
		}
		var satisfied, flagged []resource.Resource
		for _, r := range rows {
			cfg, err := clients.CloudFront.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: aws.String(r.ID)})
			if err != nil || cfg.DistributionConfig == nil {
				continue
			}
			if l := cfg.DistributionConfig.Logging; l != nil && aws.ToBool(l.Enabled) {
				continue
			}
			if delivered[r.Fields["arn"]] {
				satisfied = append(satisfied, r)
			} else {
				flagged = append(flagged, r)
			}
		}
		witness(t, td, rows, satisfied, flagged, "cf.logging-off")
	})

	t.Run("waf", func(t *testing.T) {
		td, rows := merged(t, "waf")
		logged := func(arn string, scope wafv2types.LogScope) bool {
			out, err := clients.WAFv2.GetLoggingConfiguration(ctx, &wafv2.GetLoggingConfigurationInput{ResourceArn: aws.String(arn), LogScope: scope})
			return err == nil && out.LoggingConfiguration != nil
		}
		var satisfied, flagged []resource.Resource
		for _, r := range rows {
			arn := r.Fields["arn"]
			if logged(arn, wafv2types.LogScopeCustomer) {
				continue
			}
			if logged(arn, wafv2types.LogScopeSecurityLake) || logged(arn, wafv2types.LogScopeCloudwatchTelemetryRuleManaged) {
				satisfied = append(satisfied, r)
			} else {
				flagged = append(flagged, r)
			}
		}
		witness(t, td, rows, satisfied, flagged, "waf.no-logging")
	})

	t.Run("ecr", func(t *testing.T) {
		td, rows := merged(t, "ecr")
		var mutableExcl, immutableExcl []resource.Resource
		for _, r := range rows {
			repo, ok := r.RawStruct.(ecrtypes.Repository)
			if !ok {
				continue
			}
			switch repo.ImageTagMutability {
			case ecrtypes.ImageTagMutabilityMutableWithExclusion:
				mutableExcl = append(mutableExcl, r)
			case ecrtypes.ImageTagMutabilityImmutableWithExclusion:
				immutableExcl = append(immutableExcl, r)
			}
		}
		witness(t, td, rows, immutableExcl, mutableExcl, "ecr.mutable-tags")
	})

	t.Run("elb", func(t *testing.T) {
		td, rows := merged(t, "elb")
		listeners, ok := clients.ELBv2.(interface {
			DescribeListeners(context.Context, *elbv2.DescribeListenersInput, ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error)
		})
		if !ok {
			t.Fatal("the demo ELBv2 client answers no DescribeListeners")
		}
		weak := []string{"ELBSecurityPolicy-TLS13-1-0-2021-06", "ELBSecurityPolicy-TLS13-1-1-2021-06"}
		var modern, flagged []resource.Resource
		for _, r := range rows {
			out, err := listeners.DescribeListeners(ctx, &elbv2.DescribeListenersInput{LoadBalancerArn: aws.String(r.Fields["load_balancer_arn"])})
			if err != nil || len(out.Listeners) == 0 {
				continue
			}
			var policies []string
			for _, l := range out.Listeners {
				if p := aws.ToString(l.SslPolicy); p != "" {
					policies = append(policies, p)
				}
			}
			switch {
			case slices.ContainsFunc(policies, func(p string) bool { return slices.Contains(weak, p) }):
				flagged = append(flagged, r)
			case slices.Contains(policies, "ELBSecurityPolicy-TLS13-1-2-2021-06"):
				modern = append(modern, r)
			}
		}
		witness(t, td, rows, modern, flagged, "elb.weak-tls-policy")
	})

	t.Run("lt", func(t *testing.T) {
		td, rows := merged(t, "lt")
		var satisfied, flagged []resource.Resource
		for _, r := range rows {
			raw, ok := r.RawStruct.(*awsclient.LTRaw)
			if !ok || raw.DefaultVersion.LaunchTemplateData == nil || raw.DefaultVersion.LaunchTemplateData.MetadataOptions != nil {
				continue
			}
			imageID := aws.ToString(raw.DefaultVersion.LaunchTemplateData.ImageId)
			out, err := clients.EC2.DescribeImages(ctx, &ec2.DescribeImagesInput{ImageIds: []string{imageID}})
			if err != nil || len(out.Images) != 1 {
				continue
			}
			if out.Images[0].ImdsSupport == ec2types.ImdsSupportValuesV20 {
				satisfied = append(satisfied, r)
			} else {
				flagged = append(flagged, r)
			}
		}
		witness(t, td, rows, satisfied, flagged, "lt.warn.imdsv1")
	})
}
