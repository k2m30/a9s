// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides Elastic Beanstalk fixture data for the EB fake.
package fixtures

import (
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
)

// EBManagedUpdatesOff, EBEnhancedHealthOff and EBCWLogsOff name the one demo
// environment each for managed platform updates off
// (eb.managed-updates-off), basic health reporting
// (eb.enhanced-health-off) and no log streaming (eb.cloudwatch-logs-off).
// The eb rows are keyed by environment ID, which is what the list renders
// and what the witness gate compares against.
//
// All three are Green and Ready and carry no other finding. A configuration
// signal is severity "~", so an environment already reporting a health
// warning would render THAT phrase in its Status cell and the configuration
// phrase would reach no list row at all — the finding would exist and be
// invisible where an operator looks for it.
const (
	EBManagedUpdatesOff = "e-acmeprodapi"
	EBEnhancedHealthOff = "e-acmeprodwkr"
	EBCWLogsOff         = "e-acmestagweb"
)

// The environment names those three rows carry, used as the
// ConfigurationSettings keys the enricher addresses them by.
const (
	ebEnvUnmanaged   = "acme-prod-api"
	ebEnvBasicHealth = "acme-prod-worker"
	ebEnvNoLogs      = "acme-staging-web"
)

// ebRegion/ebAccountID back every synthetic EnvironmentArn below — the eb
// ConsoleURL builder (core/aws/catalog_messaging.go) parses app/env out of
// EnvironmentArn, so every environment fixture needs a well-formed one.
const (
	ebRegion    = "us-east-1"
	ebAccountID = "123456789012"
)

// EBFixtures holds all Elastic Beanstalk domain objects served by the fake.
type EBFixtures struct {
	// Environments is the full list returned by DescribeEnvironments.
	Environments []ebtypes.EnvironmentDescription
	// ConfigurationSettings maps "applicationName/environmentName" to the
	// configuration set served by DescribeConfigurationSettings. Required
	// for the secrets:eb, eb:sg and eb:role related-panel pivot witnesses.
	ConfigurationSettings map[string][]ebtypes.ConfigurationSettingsDescription
	// EnvironmentResources maps environmentName -> resources, served by
	// DescribeEnvironmentResources. Required for the eb:elb and eb:tg
	// related-panel pivot witnesses (checkEbELB / checkEbTG).
	EnvironmentResources map[string]*ebtypes.EnvironmentResourceDescription
	// ApplicationVersions maps applicationName -> versions, served by
	// DescribeApplicationVersions. Required for the eb:s3 related-panel
	// pivot witness (checkEbS3).
	ApplicationVersions map[string][]ebtypes.ApplicationVersionDescription
	// EnvironmentHealthCauses maps environmentName -> DescribeEnvironmentHealth
	// causes. Backs EnrichEBEnvironmentHealth's Wave-2 "~" issue check.
	EnvironmentHealthCauses map[string][]string
}

// NewEBFixtures builds and returns a fully-populated EBFixtures struct.
var sharedEBFixtures = sync.OnceValue(func() *EBFixtures {
	return &EBFixtures{
		Environments: buildEBEnvironments(),
		ConfigurationSettings: map[string][]ebtypes.ConfigurationSettingsDescription{
			// acme-api/acme-prod-api references prod/database/primary (a
			// real secrets.go fixture) via the Elastic Beanstalk secret
			// resolution syntax in an environment variable.
			"acme-api/acme-prod-api": {
				{
					ApplicationName: aws.String("acme-api"),
					EnvironmentName: aws.String("acme-prod-api"),
					OptionSettings: []ebtypes.ConfigurationOptionSetting{
						{
							Namespace:  aws.String("aws:elasticbeanstalk:application:environment"),
							OptionName: aws.String("DATABASE_URL"),
							Value:      aws.String("{{resolve:secretsmanager:arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf}}"),
						},
						// Required for eb:sg related-panel pivot witness
						// (checkEbSG). sg-0aaa111111111111a is a real ec2.go
						// security group (acme-web-alb-sg).
						{
							Namespace:  aws.String("aws:autoscaling:launchconfiguration"),
							OptionName: aws.String("SecurityGroups"),
							Value:      aws.String("sg-0aaa111111111111a"),
						},
						// Required for eb:role related-panel pivot witness
						// (checkEbRole). acme-ci-deploy-role is a real
						// iam.go role fixture.
						{
							Namespace:  aws.String("aws:elasticbeanstalk:environment"),
							OptionName: aws.String("ServiceRole"),
							Value:      aws.String("acme-ci-deploy-role"),
						},
						// EBManagedUpdatesOff rides this environment, so its
						// three settings sit alongside the pivot options
						// above rather than replacing them.
						ebManagedActionsOption("false"),
						ebHealthSystemOption("enhanced"),
						ebStreamLogsOption("true"),
					},
				},
			},
			"acme-worker/" + ebEnvBasicHealth: ebOptionSet("acme-worker", ebEnvBasicHealth, "true", "basic", "true"),
			"acme-web/" + ebEnvNoLogs:         ebOptionSet("acme-web", ebEnvNoLogs, "true", "enhanced", "false"),
		},
		// EnvironmentResources — required for eb:elb and eb:tg related-panel
		// pivot witnesses (checkEbELB / checkEbTG). acme-prod-web is a real
		// elb.go load balancer fixture with a listener forwarding to a
		// target group.
		EnvironmentResources: map[string]*ebtypes.EnvironmentResourceDescription{
			"acme-prod-api": {
				EnvironmentName: aws.String("acme-prod-api"),
				LoadBalancers: []ebtypes.LoadBalancer{
					{Name: aws.String("acme-prod-web")},
				},
			},
		},
		// ApplicationVersions — required for eb:s3 related-panel pivot
		// witness (checkEbS3). a9s-demo-healthy is a real s3.go bucket.
		ApplicationVersions: map[string][]ebtypes.ApplicationVersionDescription{
			"acme-api": {
				{
					ApplicationName: aws.String("acme-api"),
					VersionLabel:    aws.String("v2.4.1"),
					SourceBundle: &ebtypes.S3Location{
						S3Bucket: aws.String(HealthyBucketName),
						S3Key:    aws.String("acme-api/v2.4.1.zip"),
					},
				},
			},
		},
		// EnvironmentHealthCauses — acme-eb-red (Health=Red) has a real cause
		// explaining the critical health state, so EnrichEBEnvironmentHealth's
		// "~" issue check fires in demo mode.
		EnvironmentHealthCauses: map[string][]string{
			"acme-eb-red": {
				"40% of the requests are failing with HTTP 5xx.",
			},
		},
	}
})

func NewEBFixtures() *EBFixtures {
	return sharedEBFixtures()
}

// ebOptionSet builds the configuration set for one environment, naming all
// three of the settings rows 13-15 read so each witness trips exactly one
// and is explicitly healthy on the other two.
func ebOptionSet(app, env, managedActions, healthSystem, streamLogs string) []ebtypes.ConfigurationSettingsDescription {
	return []ebtypes.ConfigurationSettingsDescription{{
		ApplicationName: aws.String(app),
		EnvironmentName: aws.String(env),
		OptionSettings: []ebtypes.ConfigurationOptionSetting{
			ebManagedActionsOption(managedActions),
			ebHealthSystemOption(healthSystem),
			ebStreamLogsOption(streamLogs),
		},
	}}
}

func ebManagedActionsOption(v string) ebtypes.ConfigurationOptionSetting {
	return ebtypes.ConfigurationOptionSetting{
		Namespace:  aws.String("aws:elasticbeanstalk:managedactions"),
		OptionName: aws.String("ManagedActionsEnabled"),
		Value:      aws.String(v),
	}
}

func ebHealthSystemOption(v string) ebtypes.ConfigurationOptionSetting {
	return ebtypes.ConfigurationOptionSetting{
		Namespace:  aws.String("aws:elasticbeanstalk:healthreporting:system"),
		OptionName: aws.String("SystemType"),
		Value:      aws.String(v),
	}
}

func ebStreamLogsOption(v string) ebtypes.ConfigurationOptionSetting {
	return ebtypes.ConfigurationOptionSetting{
		Namespace:  aws.String("aws:elasticbeanstalk:cloudwatch:logs"),
		OptionName: aws.String("StreamLogs"),
		Value:      aws.String(v),
	}
}

func buildEBEnvironments() []ebtypes.EnvironmentDescription {
	return []ebtypes.EnvironmentDescription{
		{
			EnvironmentName:   aws.String("acme-prod-api"),
			EnvironmentId:     aws.String("e-acmeprodapi"),
			ApplicationName:   aws.String("acme-api"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-api/acme-prod-api"),
			VersionLabel:      aws.String("v2.4.1"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.1 running Docker"),
			Health:            ebtypes.EnvironmentHealthGreen,
			Status:            ebtypes.EnvironmentStatusReady,
			CNAME:             aws.String("acme-prod-api.us-east-1.elasticbeanstalk.com"),
			EndpointURL:       aws.String("awseb-acme-prod-api-elb-123456789.us-east-1.elb.amazonaws.com"),
			DateCreated:       aws.Time(mustTime("2025-01-10T09:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-03-15T14:30:00Z")),
		},
		{
			EnvironmentName:   aws.String("acme-staging-api"),
			EnvironmentId:     aws.String("e-acmestagapi"),
			ApplicationName:   aws.String("acme-api"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-api/acme-staging-api"),
			VersionLabel:      aws.String("v2.5.0-rc1"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.1 running Docker"),
			Health:            ebtypes.EnvironmentHealthYellow,
			Status:            ebtypes.EnvironmentStatusReady,
			CNAME:             aws.String("acme-staging-api.us-east-1.elasticbeanstalk.com"),
			EndpointURL:       aws.String("awseb-acme-stag-api-elb-987654321.us-east-1.elb.amazonaws.com"),
			DateCreated:       aws.Time(mustTime("2025-02-01T11:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-03-20T10:00:00Z")),
		},
		{
			EnvironmentName:   aws.String("acme-prod-web"),
			EnvironmentId:     aws.String("e-acmeprodweb"),
			ApplicationName:   aws.String("acme-web"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-web/acme-prod-web"),
			VersionLabel:      aws.String("v3.1.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v6.1.0 running Node.js 20"),
			Health:            ebtypes.EnvironmentHealthGreen,
			Status:            ebtypes.EnvironmentStatusUpdating,
			CNAME:             aws.String("acme-prod-web.us-east-1.elasticbeanstalk.com"),
			EndpointURL:       aws.String("awseb-acme-prod-web-elb-111222333.us-east-1.elb.amazonaws.com"),
			DateCreated:       aws.Time(mustTime("2024-11-05T08:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-03-22T09:45:00Z")),
		},
		{
			EnvironmentName:   aws.String("acme-legacy-worker"),
			EnvironmentId:     aws.String("e-acmelegacy"),
			ApplicationName:   aws.String("acme-worker"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-worker/acme-legacy-worker"),
			VersionLabel:      aws.String("v1.0.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2 v3.5.9 running Python 3.8"),
			Health:            ebtypes.EnvironmentHealthGrey,
			Status:            ebtypes.EnvironmentStatusTerminating,
			CNAME:             aws.String("acme-legacy-worker.us-east-1.elasticbeanstalk.com"),
			DateCreated:       aws.Time(mustTime("2023-06-01T14:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-03-22T11:00:00Z")),
		},
		// Issue: Health=Red → Broken (environment in critical health state)
		{
			EnvironmentName:   aws.String("acme-eb-red"),
			EnvironmentId:     aws.String("e-acmeebred"),
			ApplicationName:   aws.String("acme-api"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-api/acme-eb-red"),
			VersionLabel:      aws.String("v2.3.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.1 running Docker"),
			Health:            ebtypes.EnvironmentHealthRed,
			Status:            ebtypes.EnvironmentStatusReady,
			CNAME:             aws.String("acme-eb-red.us-east-1.elasticbeanstalk.com"),
			EndpointURL:       aws.String("awseb-acme-eb-red-elb-444555666.us-east-1.elb.amazonaws.com"),
			DateCreated:       aws.Time(mustTime("2025-05-20T10:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-04-18T03:00:00Z")),
		},
		// Issue: Status=Terminated → Dim (environment has been shut down)
		{
			EnvironmentName:   aws.String("acme-eb-terminated"),
			EnvironmentId:     aws.String("e-acmetermed"),
			ApplicationName:   aws.String("acme-web"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-web/acme-eb-terminated"),
			VersionLabel:      aws.String("v2.0.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2 v3.5.9 running Python 3.8"),
			Health:            ebtypes.EnvironmentHealthGrey,
			Status:            ebtypes.EnvironmentStatusTerminated,
			CNAME:             aws.String("acme-eb-terminated.us-east-1.elasticbeanstalk.com"),
			DateCreated:       aws.Time(mustTime("2024-01-10T09:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-01-15T18:00:00Z")),
		},
		// Issue: Status=Terminating with a reporting (Green) health check →
		// Dim via the status-only branch (no Grey/Yellow/Red health signal
		// to take precedence).
		{
			EnvironmentName:   aws.String("acme-batch-worker-old"),
			EnvironmentId:     aws.String("e-acmebatchold"),
			ApplicationName:   aws.String("acme-worker"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-worker/acme-batch-worker-old"),
			VersionLabel:      aws.String("v0.9.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2 v3.5.9 running Python 3.8"),
			Health:            ebtypes.EnvironmentHealthGreen,
			Status:            ebtypes.EnvironmentStatusTerminating,
			CNAME:             aws.String("acme-batch-worker-old.us-east-1.elasticbeanstalk.com"),
			DateCreated:       aws.Time(mustTime("2023-02-15T10:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-05-01T09:00:00Z")),
		},
		// EBEnhancedHealthOff and EBCWLogsOff. Green and Ready, so the
		// configuration phrase is the only thing in their Status cell.
		{
			EnvironmentName:   aws.String(ebEnvBasicHealth),
			EnvironmentId:     aws.String(EBEnhancedHealthOff),
			ApplicationName:   aws.String("acme-worker"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-worker/" + ebEnvBasicHealth),
			VersionLabel:      aws.String("v3.1.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.1 running Docker"),
			Health:            ebtypes.EnvironmentHealthGreen,
			Status:            ebtypes.EnvironmentStatusReady,
			CNAME:             aws.String(ebEnvBasicHealth + ".us-east-1.elasticbeanstalk.com"),
			DateCreated:       aws.Time(mustTime("2025-06-01T09:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-06-01T09:00:00Z")),
		},
		// The healthy bucket's own row: Green, Ready, and with no
		// ConfigurationSettings entry, so all three settings read as unknown
		// and it carries no finding at all.
		{
			EnvironmentName:   aws.String("acme-prod-events"),
			EnvironmentId:     aws.String("e-acmeprodevents"),
			ApplicationName:   aws.String("acme-api"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-api/acme-prod-events"),
			VersionLabel:      aws.String("v1.9.3"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.1 running Docker"),
			Health:            ebtypes.EnvironmentHealthGreen,
			Status:            ebtypes.EnvironmentStatusReady,
			CNAME:             aws.String("acme-prod-events.us-east-1.elasticbeanstalk.com"),
			DateCreated:       aws.Time(mustTime("2025-08-19T09:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-06-03T09:00:00Z")),
		},
		{
			EnvironmentName:   aws.String(ebEnvNoLogs),
			EnvironmentId:     aws.String(EBCWLogsOff),
			ApplicationName:   aws.String("acme-web"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:" + ebRegion + ":" + ebAccountID + ":environment/acme-web/" + ebEnvNoLogs),
			VersionLabel:      aws.String("v2.2.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.1 running Docker"),
			Health:            ebtypes.EnvironmentHealthGreen,
			Status:            ebtypes.EnvironmentStatusReady,
			CNAME:             aws.String(ebEnvNoLogs + ".us-east-1.elasticbeanstalk.com"),
			DateCreated:       aws.Time(mustTime("2025-07-12T09:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-06-02T09:00:00Z")),
		},
	}
}
