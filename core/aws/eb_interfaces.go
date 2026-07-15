package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
)

// EBDescribeEnvironmentsAPI defines the interface for the Elastic Beanstalk DescribeEnvironments operation.
type EBDescribeEnvironmentsAPI interface {
	DescribeEnvironments(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentsInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error)
}

// EBDescribeApplicationVersionsAPI defines the interface for the Elastic Beanstalk DescribeApplicationVersions operation.
type EBDescribeApplicationVersionsAPI interface {
	DescribeApplicationVersions(ctx context.Context, params *elasticbeanstalk.DescribeApplicationVersionsInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeApplicationVersionsOutput, error)
}

// EBDescribeConfigurationSettingsAPI for eb→{role, s3, sg, elb, tg} via ConfigurationSettings option values.
type EBDescribeConfigurationSettingsAPI interface {
	DescribeConfigurationSettings(ctx context.Context, params *elasticbeanstalk.DescribeConfigurationSettingsInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error)
}

// EBDescribeEnvironmentResourcesAPI for eb→{elb, asg, ec2, tg} via EnvironmentResources.
type EBDescribeEnvironmentResourcesAPI interface {
	DescribeEnvironmentResources(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentResourcesInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error)
}

// ElasticBeanstalkDescribeEnvironmentHealthAPI defines the interface for the
// Elastic Beanstalk DescribeEnvironmentHealth operation.
// Used by EnrichEBEnvironmentHealth (Wave 2 enrichment).
type ElasticBeanstalkDescribeEnvironmentHealthAPI interface {
	DescribeEnvironmentHealth(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentHealthInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentHealthOutput, error)
}

// ElasticBeanstalkAPI is the aggregate interface covering all ElasticBeanstalk operations used by a9s fetchers.
// *elasticbeanstalk.Client structurally satisfies this interface.
type ElasticBeanstalkAPI interface {
	EBDescribeEnvironmentsAPI
	ElasticBeanstalkDescribeEnvironmentHealthAPI // Wave 2 enrichment
	EBDescribeConfigurationSettingsAPI
	EBDescribeEnvironmentResourcesAPI
	EBDescribeApplicationVersionsAPI
}
