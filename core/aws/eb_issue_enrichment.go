// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eb_issue_enrichment.go — Wave 2 issue enrichment for the eb resource type.
package aws

import (
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// eb canonical FindingCodes.
const (
	ebCodeEnvironmentCauses domain.FindingCode = "eb.environment-causes"
	ebCodeManagedUpdatesOff domain.FindingCode = "eb.managed-updates-off"
	ebCodeEnhancedHealthOff domain.FindingCode = "eb.enhanced-health-off"
	ebCodeCWLogsOff         domain.FindingCode = "eb.cloudwatch-logs-off"
)

// S5 operator sentences for the environment configuration codes above.
// The three environment settings rows 13-15 read, each identified by the
// namespace AND option name AWS returns it under. Matching on the option
// name alone reads a same-named option from another namespace.
const (
	ebNamespaceManagedActions = "aws:elasticbeanstalk:managedactions"
	ebOptionManagedActions    = "ManagedActionsEnabled"
	ebNamespaceHealthSystem   = "aws:elasticbeanstalk:healthreporting:system"
	ebOptionSystemType        = "SystemType"
	ebNamespaceCWLogs         = "aws:elasticbeanstalk:cloudwatch:logs"
	ebOptionStreamLogs        = "StreamLogs"
)

// EnrichEBEnvironmentHealth calls DescribeEnvironmentHealth for each Elastic
// Beanstalk environment (1 per environment, cap 50). Returns an informational
// "~" finding for each environment with a non-empty Causes slice.
// Summary: "EB causes: <first cause>" — causes are informational signals,
// not broken-state indicators.
func EnrichEBEnvironmentHealth(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.ElasticBeanstalk == nil {
		return result, nil
	}
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		name := r.Name
		if name == "" {
			name = r.Fields["environment_name"]
		}
		if name == "" {
			return
		}
		out, err := clients.ElasticBeanstalk.DescribeEnvironmentHealth(ctx, &elasticbeanstalk.DescribeEnvironmentHealthInput{
			EnvironmentName: aws.String(name),
			AttributeNames:  []ebtypes.EnvironmentHealthAttribute{ebtypes.EnvironmentHealthAttributeCauses},
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if len(out.Causes) == 0 {
			return
		}
		var rows []domain.DetailRow
		for _, cause := range out.Causes {
			rows = append(rows, domain.DetailRow{Label: "Cause", Value: cause, Tier: "~"})
		}
		// Key on resource ID (environment ID) for registry consistency.
		// Fall back to name if ID is not set.
		key := r.ID
		if key == "" {
			key = name
		}
		setWave2Finding(&result, key, ebCodeEnvironmentCauses, rows)
	})
	settingsErr := ebConfigurationPosture(ctx, clients, &result, resources)
	MarkInformationalOnly(&result)
	return result, errors.Join(AggregateFailures("DescribeEnvironmentHealth", failures, n), settingsErr)
}

// ebConfigurationPosture reads DescribeConfigurationSettings once per
// environment (cap EnrichmentCap). The one response carries all three option
// values rows 13-15 need, so it is parsed once and emits three independent
// findings rather than calling three times.
func ebConfigurationPosture(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, resources []resource.Resource) error {
	api, ok := clients.ElasticBeanstalk.(EBDescribeConfigurationSettingsAPI)
	if !ok {
		return nil
	}
	resources = capAtEnrichmentCap(result, resources, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		name := r.Name
		if name == "" {
			name = r.Fields["environment_name"]
		}
		app := r.Fields["application_name"]
		if name == "" || app == "" {
			return
		}
		// Rule 4: an environment being torn down has no posture worth
		// reporting, and its settings are about to stop existing.
		if ebLifecycleEnded(r.Fields["status"]) {
			return
		}
		key := r.ID
		if key == "" {
			key = name
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
			return api.DescribeConfigurationSettings(ctx, &elasticbeanstalk.DescribeConfigurationSettingsInput{
				ApplicationName: aws.String(app),
				EnvironmentName: aws.String(name),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(result, key, &failures, err)
			return
		}
		var options []ebtypes.ConfigurationOptionSetting
		for _, cs := range out.ConfigurationSettings {
			options = append(options, cs.OptionSettings...)
		}
		// An option AWS did not return is unknown. All three settings default
		// to a value other than the flagged one, so reading an absent option
		// as the bad one would flag environments nobody misconfigured.
		if v, ok := ebOptionValue(options, ebNamespaceManagedActions, ebOptionManagedActions); ok && v != "true" {
			setWave2Finding(result, key, ebCodeManagedUpdatesOff, nil)
		}
		if v, ok := ebOptionValue(options, ebNamespaceHealthSystem, ebOptionSystemType); ok && v != "enhanced" {
			setWave2Finding(result, key, ebCodeEnhancedHealthOff, []domain.DetailRow{{Label: "Reporting level", Value: v, Tier: tierOf(ebCodeEnhancedHealthOff)}})
		}
		if v, ok := ebOptionValue(options, ebNamespaceCWLogs, ebOptionStreamLogs); ok && v != "true" {
			setWave2Finding(result, key, ebCodeCWLogsOff, nil)
		}
	})
	return AggregateFailures("DescribeConfigurationSettings", failures, n)
}

// ebOptionValue finds one option by namespace AND name, reporting whether
// AWS returned it at all.
func ebOptionValue(options []ebtypes.ConfigurationOptionSetting, namespace, name string) (string, bool) {
	for _, o := range options {
		if aws.ToString(o.Namespace) == namespace && aws.ToString(o.OptionName) == name {
			return aws.ToString(o.Value), true
		}
	}
	return "", false
}

// ebLifecycleEnded reports whether an environment is on its way out, or
// already gone. Fields["status"] carries the raw DescribeEnvironments value.
func ebLifecycleEnded(status string) bool {
	return status == string(ebtypes.EnvironmentStatusTerminating) ||
		status == string(ebtypes.EnvironmentStatusTerminated)
}
