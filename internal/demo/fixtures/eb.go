// Package fixtures provides Elastic Beanstalk fixture data for the EB fake.
package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
)

// EBFixtures holds all Elastic Beanstalk domain objects served by the fake.
type EBFixtures struct {
	// Environments is the full list returned by DescribeEnvironments.
	Environments []ebtypes.EnvironmentDescription
}

// NewEBFixtures builds and returns a fully-populated EBFixtures struct.
func NewEBFixtures() *EBFixtures {
	return &EBFixtures{
		Environments: buildEBEnvironments(),
	}
}

func buildEBEnvironments() []ebtypes.EnvironmentDescription {
	return []ebtypes.EnvironmentDescription{
		{
			EnvironmentName: aws.String("acme-prod-api"),
			EnvironmentId:   aws.String("e-acmeprodapi"),
			ApplicationName: aws.String("acme-api"),
			VersionLabel:    aws.String("v2.4.1"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.1 running Docker"),
			Health:          ebtypes.EnvironmentHealthGreen,
			Status:          ebtypes.EnvironmentStatusReady,
			CNAME:           aws.String("acme-prod-api.us-east-1.elasticbeanstalk.com"),
			EndpointURL:     aws.String("awseb-acme-prod-api-elb-123456789.us-east-1.elb.amazonaws.com"),
			DateCreated:     aws.Time(mustTime("2025-01-10T09:00:00Z")),
			DateUpdated:     aws.Time(mustTime("2026-03-15T14:30:00Z")),
		},
		{
			EnvironmentName: aws.String("acme-staging-api"),
			EnvironmentId:   aws.String("e-acmestagapi"),
			ApplicationName: aws.String("acme-api"),
			VersionLabel:    aws.String("v2.5.0-rc1"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.1 running Docker"),
			Health:          ebtypes.EnvironmentHealthYellow,
			Status:          ebtypes.EnvironmentStatusReady,
			CNAME:           aws.String("acme-staging-api.us-east-1.elasticbeanstalk.com"),
			EndpointURL:     aws.String("awseb-acme-stag-api-elb-987654321.us-east-1.elb.amazonaws.com"),
			DateCreated:     aws.Time(mustTime("2025-02-01T11:00:00Z")),
			DateUpdated:     aws.Time(mustTime("2026-03-20T10:00:00Z")),
		},
		{
			EnvironmentName: aws.String("acme-prod-web"),
			EnvironmentId:   aws.String("e-acmeprodweb"),
			ApplicationName: aws.String("acme-web"),
			VersionLabel:    aws.String("v3.1.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2023 v6.1.0 running Node.js 20"),
			Health:          ebtypes.EnvironmentHealthGreen,
			Status:          ebtypes.EnvironmentStatusUpdating,
			CNAME:           aws.String("acme-prod-web.us-east-1.elasticbeanstalk.com"),
			EndpointURL:     aws.String("awseb-acme-prod-web-elb-111222333.us-east-1.elb.amazonaws.com"),
			DateCreated:     aws.Time(mustTime("2024-11-05T08:00:00Z")),
			DateUpdated:     aws.Time(mustTime("2026-03-22T09:45:00Z")),
		},
		{
			EnvironmentName: aws.String("acme-legacy-worker"),
			EnvironmentId:   aws.String("e-acmelegacy"),
			ApplicationName: aws.String("acme-worker"),
			VersionLabel:    aws.String("v1.0.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2 v3.5.9 running Python 3.8"),
			Health:          ebtypes.EnvironmentHealthGrey,
			Status:          ebtypes.EnvironmentStatusTerminating,
			CNAME:           aws.String("acme-legacy-worker.us-east-1.elasticbeanstalk.com"),
			DateCreated:     aws.Time(mustTime("2023-06-01T14:00:00Z")),
			DateUpdated:     aws.Time(mustTime("2026-03-22T11:00:00Z")),
		},
		// Issue: Health=Red → Broken (environment in critical health state)
		{
			EnvironmentName:   aws.String("acme-eb-red"),
			EnvironmentId:     aws.String("e-acmeebred"),
			ApplicationName:   aws.String("acme-api"),
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
			VersionLabel:      aws.String("v2.0.0"),
			SolutionStackName: aws.String("64bit Amazon Linux 2 v3.5.9 running Python 3.8"),
			Health:            ebtypes.EnvironmentHealthGrey,
			Status:            ebtypes.EnvironmentStatusTerminated,
			CNAME:             aws.String("acme-eb-terminated.us-east-1.elasticbeanstalk.com"),
			DateCreated:       aws.Time(mustTime("2024-01-10T09:00:00Z")),
			DateUpdated:       aws.Time(mustTime("2026-01-15T18:00:00Z")),
		},
	}
}
