package demo

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["cfn"] = cfnFixtures
	demoData["ecr"] = ecrFixtures
	demoData["codeartifact"] = codeartifactFixtures

	RegisterChildDemo("cfn_events", func(parentCtx map[string]string) []resource.Resource {
		return cfnEventFixtures(parentCtx["stack_name"])
	})
	RegisterChildDemo("cfn_resources", func(parentCtx map[string]string) []resource.Resource {
		return cfnResourceFixtures(parentCtx["stack_name"])
	})
	RegisterChildDemo("ecr_images", func(parentCtx map[string]string) []resource.Resource {
		return ecrImageFixtures(parentCtx["repository_name"], parentCtx["repository_uri"])
	})
}

// cfnFixtures returns demo CloudFormation Stack fixtures.
func cfnFixtures() []resource.Resource {
	stacks := []resource.Resource{
		{
			ID:     "acme-vpc-stack",
			Name:   "acme-vpc-stack",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"stack_name":    "acme-vpc-stack",
				"status":        "CREATE_COMPLETE",
				"creation_time": "2024-10-15T09:00:00+00:00",
				"last_updated":  "2025-06-20T14:30:00+00:00",
				"description":   "Core VPC networking stack for Acme Corp production",
			},
			RawStruct: cfntypes.Stack{
				StackName:                   aws.String("acme-vpc-stack"),
				StackStatus:                 cfntypes.StackStatusCreateComplete,
				DetailedStatus:              cfntypes.DetailedStatusConfigurationComplete,
				StackStatusReason:           aws.String("Stack CREATE_COMPLETE"),
				CreationTime:                aws.Time(mustParseTime("2024-10-15T09:00:00+00:00")),
				LastUpdatedTime:             aws.Time(mustParseTime("2025-06-20T14:30:00+00:00")),
				DeletionTime:                aws.Time(mustParseTime("2099-01-01T00:00:00+00:00")),
				Description:                 aws.String("Core VPC networking stack for Acme Corp production"),
				StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-vpc-stack/11111111-1111-1111-1111-111111111111"),
				RoleARN:                     aws.String(prodCIDeployRoleARN),
				Capabilities:                []cfntypes.Capability{cfntypes.CapabilityCapabilityIam},
				EnableTerminationProtection: aws.Bool(true),
				DriftInformation: &cfntypes.StackDriftInformation{
					StackDriftStatus:   cfntypes.StackDriftStatusInSync,
					LastCheckTimestamp: aws.Time(mustParseTime("2026-03-20T10:00:00+00:00")),
				},
				Parameters: []cfntypes.Parameter{
					{ParameterKey: aws.String("VpcCidr"), ParameterValue: aws.String("10.0.0.0/16")},
					{ParameterKey: aws.String("Environment"), ParameterValue: aws.String("production")},
				},
				Outputs: []cfntypes.Output{
					{OutputKey: aws.String("VpcId"), OutputValue: aws.String("vpc-0abc123def456789a"), Description: aws.String("Production VPC ID")},
					{OutputKey: aws.String("PublicSubnets"), OutputValue: aws.String("subnet-0aaa111111111111a,subnet-0bbb222222222222b")},
				},
				Tags: []cfntypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("production")},
					{Key: aws.String("Team"), Value: aws.String("platform")},
				},
			},
		},
		{
			ID:     "acme-eks-cluster",
			Name:   "acme-eks-cluster",
			Status: "UPDATE_COMPLETE",
			Fields: map[string]string{
				"stack_name":    "acme-eks-cluster",
				"status":        "UPDATE_COMPLETE",
				"creation_time": "2025-01-10T11:00:00+00:00",
				"last_updated":  "2026-03-15T08:45:00+00:00",
				"description":   "EKS cluster and managed node groups",
			},
			RawStruct: cfntypes.Stack{
				StackName:       aws.String("acme-eks-cluster"),
				StackStatus:     cfntypes.StackStatusUpdateComplete,
				CreationTime:    aws.Time(mustParseTime("2025-01-10T11:00:00+00:00")),
				LastUpdatedTime: aws.Time(mustParseTime("2026-03-15T08:45:00+00:00")),
				Description:     aws.String("EKS cluster and managed node groups"),
				StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-eks-cluster/22222222-2222-2222-2222-222222222222"),
				RoleARN:         aws.String(prodCIDeployRoleARN),
			},
		},
		{
			ID:     "acme-rds-aurora",
			Name:   "acme-rds-aurora",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"stack_name":    "acme-rds-aurora",
				"status":        "CREATE_COMPLETE",
				"creation_time": "2025-03-05T16:20:00+00:00",
				"last_updated":  "",
				"description":   "Aurora PostgreSQL cluster for API backend",
			},
			RawStruct: cfntypes.Stack{
				StackName:    aws.String("acme-rds-aurora"),
				StackStatus:  cfntypes.StackStatusCreateComplete,
				CreationTime: aws.Time(mustParseTime("2025-03-05T16:20:00+00:00")),
				Description:  aws.String("Aurora PostgreSQL cluster for API backend"),
				StackId:      aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-rds-aurora/33333333-3333-3333-3333-333333333333"),
				RoleARN:      aws.String(prodCIDeployRoleARN),
			},
		},
		{
			ID:     "acme-monitoring",
			Name:   "acme-monitoring",
			Status: "UPDATE_ROLLBACK_COMPLETE",
			Fields: map[string]string{
				"stack_name":    "acme-monitoring",
				"status":        "UPDATE_ROLLBACK_COMPLETE",
				"creation_time": "2025-02-01T10:00:00+00:00",
				"last_updated":  "2026-03-18T22:15:00+00:00",
				"description":   "CloudWatch alarms and dashboards",
			},
			RawStruct: cfntypes.Stack{
				StackName:       aws.String("acme-monitoring"),
				StackStatus:     cfntypes.StackStatusUpdateRollbackComplete,
				CreationTime:    aws.Time(mustParseTime("2025-02-01T10:00:00+00:00")),
				LastUpdatedTime: aws.Time(mustParseTime("2026-03-18T22:15:00+00:00")),
				Description:     aws.String("CloudWatch alarms and dashboards"),
				StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-monitoring/44444444-4444-4444-4444-444444444444"),
				RoleARN:         aws.String(prodCIDeployRoleARN),
			},
		},
		{
			ID:     "awseb-e-acmeprodapi-stack",
			Name:   "awseb-e-acmeprodapi-stack",
			Status: "UPDATE_COMPLETE",
			Fields: map[string]string{
				"stack_name":    "awseb-e-acmeprodapi-stack",
				"status":        "UPDATE_COMPLETE",
				"creation_time": "2025-05-20T09:00:00+00:00",
				"last_updated":  "2026-03-10T14:22:00+00:00",
				"description":   "Elastic Beanstalk managed stack for acme-prod-api",
			},
			RawStruct: cfntypes.Stack{
				StackName:       aws.String("awseb-e-acmeprodapi-stack"),
				StackStatus:     cfntypes.StackStatusUpdateComplete,
				CreationTime:    aws.Time(mustParseTime("2025-05-20T09:00:00+00:00")),
				LastUpdatedTime: aws.Time(mustParseTime("2026-03-10T14:22:00+00:00")),
				Description:     aws.String("Elastic Beanstalk managed stack for acme-prod-api"),
				StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/awseb-e-acmeprodapi-stack/55555555-5555-5555-5555-555555555555"),
			},
		},
	}

	// Generate 18 more stacks to reach 22 total
	cfnStatusMap := map[string]cfntypes.StackStatus{
		"CREATE_COMPLETE": cfntypes.StackStatusCreateComplete,
		"UPDATE_COMPLETE": cfntypes.StackStatusUpdateComplete,
	}
	for i := 0; i < 18; i++ {
		name := cfnNamePool[i]
		status := cfnStatusPool[i]
		creationTime := fmt.Sprintf("2025-%02d-%02dT%02d:00:00+00:00", 1+(i%12), 1+i, 9+(i%10))
		lastUpdated := ""
		var lastUpdatedTime *time.Time
		if status == "UPDATE_COMPLETE" {
			lu := fmt.Sprintf("2026-%02d-%02dT%02d:00:00+00:00", 1+(i%3), 1+i%28, 10+i%12)
			lastUpdated = lu
			t := mustParseTime(lu)
			lastUpdatedTime = &t
		}
		stackID := fmt.Sprintf("arn:aws:cloudformation:us-east-1:123456789012:stack/%s/%08d-%04d-%04d-%04d-%012d", name, i+10, i, i, i, i+100)
		desc := fmt.Sprintf("Infrastructure stack for %s", strings.TrimPrefix(name, "acme-"))

		s := cfntypes.Stack{
			StackName:    aws.String(name),
			StackStatus:  cfnStatusMap[status],
			CreationTime: aws.Time(mustParseTime(creationTime)),
			Description:  aws.String(desc),
			StackId:      aws.String(stackID),
			RoleARN:      aws.String(prodCIDeployRoleARN),
		}
		if lastUpdatedTime != nil {
			s.LastUpdatedTime = lastUpdatedTime
		}

		stacks = append(stacks, resource.Resource{
			ID:     name,
			Name:   name,
			Status: status,
			Fields: map[string]string{
				"stack_name":    name,
				"status":        status,
				"creation_time": creationTime,
				"last_updated":  lastUpdated,
				"description":   desc,
			},
			RawStruct: s,
		})
	}

	return stacks
}

// ecrFixtures returns demo ECR Repository fixtures.
func ecrFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme/api-service",
			Name:   "acme/api-service",
			Status: "",
			Fields: map[string]string{
				"repository_name": "acme/api-service",
				"uri":             prodECRAPIImageURI,
				"tag_mutability":  "IMMUTABLE",
				"scan_on_push":    "true",
				"created_at":      "2025-03-01 10:00:00",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("acme/api-service"),
				RepositoryUri:  aws.String(prodECRAPIImageURI),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/api-service"),
				RegistryId:     aws.String("123456789012"),
				ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
				ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
					ScanOnPush: true,
				},
				EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
					EncryptionType: ecrtypes.EncryptionTypeAes256,
				},
				CreatedAt: aws.Time(mustParseTime("2025-03-01T10:00:00+00:00")),
			},
		},
		{
			ID:     "acme/frontend",
			Name:   "acme/frontend",
			Status: "",
			Fields: map[string]string{
				"repository_name": "acme/frontend",
				"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/frontend",
				"tag_mutability":  "MUTABLE",
				"scan_on_push":    "true",
				"created_at":      "2025-03-01 10:05:00",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("acme/frontend"),
				RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/frontend"),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/frontend"),
				RegistryId:     aws.String("123456789012"),
				ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
				ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
					ScanOnPush: true,
				},
				EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
					EncryptionType: ecrtypes.EncryptionTypeAes256,
				},
				CreatedAt: aws.Time(mustParseTime("2025-03-01T10:05:00+00:00")),
			},
		},
		{
			ID:     "acme/base-images",
			Name:   "acme/base-images",
			Status: "",
			Fields: map[string]string{
				"repository_name": "acme/base-images",
				"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/base-images",
				"tag_mutability":  "IMMUTABLE",
				"scan_on_push":    "false",
				"created_at":      "2025-01-15 08:30:00",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("acme/base-images"),
				RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/base-images"),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/base-images"),
				RegistryId:     aws.String("123456789012"),
				ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
				ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
					ScanOnPush: false,
				},
				EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
					EncryptionType: ecrtypes.EncryptionTypeAes256,
				},
				CreatedAt: aws.Time(mustParseTime("2025-01-15T08:30:00+00:00")),
			},
		},
		{
			ID:     "acme/batch-processor",
			Name:   "acme/batch-processor",
			Status: "",
			Fields: map[string]string{
				"repository_name": "acme/batch-processor",
				"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-processor",
				"tag_mutability":  "MUTABLE",
				"scan_on_push":    "true",
				"created_at":      "2025-06-20 12:00:00",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("acme/batch-processor"),
				RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-processor"),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/batch-processor"),
				RegistryId:     aws.String("123456789012"),
				ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
				ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
					ScanOnPush: true,
				},
				EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
					EncryptionType: ecrtypes.EncryptionTypeAes256,
				},
				CreatedAt: aws.Time(mustParseTime("2025-06-20T12:00:00+00:00")),
			},
		},
	}
}

// codeartifactFixtures returns demo CodeArtifact Repository fixtures.
func codeartifactFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-npm",
			Name:   "acme-npm",
			Status: "",
			Fields: map[string]string{
				"repo_name":     "acme-npm",
				"domain_name":   "acme-artifacts",
				"domain_owner":  "123456789012",
				"arn":           "arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-npm",
				"description":   "Private npm registry for Acme frontend packages",
				"admin_account": "123456789012",
				"created_time":  "2025-04-01 09:00:00",
			},
			RawStruct: codeartifacttypes.RepositorySummary{
				Name:                 aws.String("acme-npm"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-npm"),
				Description:          aws.String("Private npm registry for Acme frontend packages"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseTime("2025-04-01T09:00:00+00:00")),
			},
		},
		{
			ID:     "acme-pypi",
			Name:   "acme-pypi",
			Status: "",
			Fields: map[string]string{
				"repo_name":     "acme-pypi",
				"domain_name":   "acme-artifacts",
				"domain_owner":  "123456789012",
				"arn":           "arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-pypi",
				"description":   "Private PyPI repository for data pipeline packages",
				"admin_account": "123456789012",
				"created_time":  "2025-04-01 09:15:00",
			},
			RawStruct: codeartifacttypes.RepositorySummary{
				Name:                 aws.String("acme-pypi"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-pypi"),
				Description:          aws.String("Private PyPI repository for data pipeline packages"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseTime("2025-04-01T09:15:00+00:00")),
			},
		},
		{
			ID:     "acme-maven",
			Name:   "acme-maven",
			Status: "",
			Fields: map[string]string{
				"repo_name":     "acme-maven",
				"domain_name":   "acme-artifacts",
				"domain_owner":  "123456789012",
				"arn":           "arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-maven",
				"description":   "Maven repository for Java microservices",
				"admin_account": "123456789012",
				"created_time":  "2025-04-01 09:30:00",
			},
			RawStruct: codeartifacttypes.RepositorySummary{
				Name:                 aws.String("acme-maven"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-maven"),
				Description:          aws.String("Maven repository for Java microservices"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseTime("2025-04-01T09:30:00+00:00")),
			},
		},
	}
}

// cfnEventFixtures returns demo CloudFormation Stack Event fixtures for a typical
// stack update cycle.
func cfnEventFixtures(stackName string) []resource.Resource {
	return []resource.Resource{
		{
			ID:     "evt-001",
			Name:   "2026-03-20 10:00:00",
			Status: "UPDATE_IN_PROGRESS",
			Fields: map[string]string{
				"timestamp":              "2026-03-20 10:00:00",
				"logical_resource_id":    stackName,
				"resource_type":          "AWS::CloudFormation::Stack",
				"resource_status":        "UPDATE_IN_PROGRESS",
				"resource_status_reason": "User Initiated",
			},
			RawStruct: cfntypes.StackEvent{
				EventId:              aws.String("evt-001"),
				StackName:            aws.String(stackName),
				Timestamp:            aws.Time(mustParseTime("2026-03-20T10:00:00+00:00")),
				LogicalResourceId:    aws.String(stackName),
				ResourceType:         aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:       cfntypes.ResourceStatusUpdateInProgress,
				ResourceStatusReason: aws.String("User Initiated"),
			},
		},
		{
			ID:     "evt-002",
			Name:   "2026-03-20 10:00:15",
			Status: "UPDATE_IN_PROGRESS",
			Fields: map[string]string{
				"timestamp":              "2026-03-20 10:00:15",
				"logical_resource_id":    "WebServerASG",
				"resource_type":          "AWS::AutoScaling::AutoScalingGroup",
				"resource_status":        "UPDATE_IN_PROGRESS",
				"resource_status_reason": "Resource update initiated",
			},
			RawStruct: cfntypes.StackEvent{
				EventId:              aws.String("evt-002"),
				StackName:            aws.String(stackName),
				Timestamp:            aws.Time(mustParseTime("2026-03-20T10:00:15+00:00")),
				LogicalResourceId:    aws.String("WebServerASG"),
				ResourceType:         aws.String("AWS::AutoScaling::AutoScalingGroup"),
				ResourceStatus:       cfntypes.ResourceStatusUpdateInProgress,
				ResourceStatusReason: aws.String("Resource update initiated"),
			},
		},
		{
			ID:     "evt-003",
			Name:   "2026-03-20 10:01:30",
			Status: "UPDATE_COMPLETE",
			Fields: map[string]string{
				"timestamp":              "2026-03-20 10:01:30",
				"logical_resource_id":    "WebServerASG",
				"resource_type":          "AWS::AutoScaling::AutoScalingGroup",
				"resource_status":        "UPDATE_COMPLETE",
				"resource_status_reason": "",
			},
			RawStruct: cfntypes.StackEvent{
				EventId:           aws.String("evt-003"),
				StackName:         aws.String(stackName),
				Timestamp:         aws.Time(mustParseTime("2026-03-20T10:01:30+00:00")),
				LogicalResourceId: aws.String("WebServerASG"),
				ResourceType:      aws.String("AWS::AutoScaling::AutoScalingGroup"),
				ResourceStatus:    cfntypes.ResourceStatusUpdateComplete,
			},
		},
		{
			ID:     "evt-004",
			Name:   "2026-03-20 10:01:45",
			Status: "UPDATE_IN_PROGRESS",
			Fields: map[string]string{
				"timestamp":              "2026-03-20 10:01:45",
				"logical_resource_id":    "AppSecurityGroup",
				"resource_type":          "AWS::EC2::SecurityGroup",
				"resource_status":        "UPDATE_IN_PROGRESS",
				"resource_status_reason": "Resource update initiated",
			},
			RawStruct: cfntypes.StackEvent{
				EventId:              aws.String("evt-004"),
				StackName:            aws.String(stackName),
				Timestamp:            aws.Time(mustParseTime("2026-03-20T10:01:45+00:00")),
				LogicalResourceId:    aws.String("AppSecurityGroup"),
				ResourceType:         aws.String("AWS::EC2::SecurityGroup"),
				ResourceStatus:       cfntypes.ResourceStatusUpdateInProgress,
				ResourceStatusReason: aws.String("Resource update initiated"),
			},
		},
		{
			ID:     "evt-005",
			Name:   "2026-03-20 10:02:00",
			Status: "UPDATE_COMPLETE",
			Fields: map[string]string{
				"timestamp":              "2026-03-20 10:02:00",
				"logical_resource_id":    "AppSecurityGroup",
				"resource_type":          "AWS::EC2::SecurityGroup",
				"resource_status":        "UPDATE_COMPLETE",
				"resource_status_reason": "",
			},
			RawStruct: cfntypes.StackEvent{
				EventId:           aws.String("evt-005"),
				StackName:         aws.String(stackName),
				Timestamp:         aws.Time(mustParseTime("2026-03-20T10:02:00+00:00")),
				LogicalResourceId: aws.String("AppSecurityGroup"),
				ResourceType:      aws.String("AWS::EC2::SecurityGroup"),
				ResourceStatus:    cfntypes.ResourceStatusUpdateComplete,
			},
		},
		{
			ID:     "evt-006",
			Name:   "2026-03-20 10:02:30",
			Status: "UPDATE_COMPLETE",
			Fields: map[string]string{
				"timestamp":              "2026-03-20 10:02:30",
				"logical_resource_id":    stackName,
				"resource_type":          "AWS::CloudFormation::Stack",
				"resource_status":        "UPDATE_COMPLETE",
				"resource_status_reason": "",
			},
			RawStruct: cfntypes.StackEvent{
				EventId:           aws.String("evt-006"),
				StackName:         aws.String(stackName),
				Timestamp:         aws.Time(mustParseTime("2026-03-20T10:02:30+00:00")),
				LogicalResourceId: aws.String(stackName),
				ResourceType:      aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:    cfntypes.ResourceStatusUpdateComplete,
			},
		},
	}
}

// cfnResourceFixtures returns demo CloudFormation Stack Resource fixtures for a
// typical infrastructure stack.
func cfnResourceFixtures(stackName string) []resource.Resource {
	return []resource.Resource{
		{
			ID:     "VPC",
			Name:   "VPC",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"logical_resource_id":  "VPC",
				"physical_resource_id": "vpc-0abc123def456789a",
				"resource_type":        "AWS::EC2::VPC",
				"resource_status":      "CREATE_COMPLETE",
				"drift_status":         "IN_SYNC",
				"last_updated":         "2024-10-15 09:02:00",
			},
			RawStruct: cfntypes.StackResourceSummary{
				LogicalResourceId:    aws.String("VPC"),
				PhysicalResourceId:   aws.String("vpc-0abc123def456789a"),
				ResourceType:         aws.String("AWS::EC2::VPC"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(mustParseTime("2024-10-15T09:02:00+00:00")),
				DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
					StackResourceDriftStatus: cfntypes.StackResourceDriftStatusInSync,
				},
			},
		},
		{
			ID:     "PublicSubnet1",
			Name:   "PublicSubnet1",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"logical_resource_id":  "PublicSubnet1",
				"physical_resource_id": "subnet-0aaa111111111111a",
				"resource_type":        "AWS::EC2::Subnet",
				"resource_status":      "CREATE_COMPLETE",
				"drift_status":         "IN_SYNC",
				"last_updated":         "2024-10-15 09:03:00",
			},
			RawStruct: cfntypes.StackResourceSummary{
				LogicalResourceId:    aws.String("PublicSubnet1"),
				PhysicalResourceId:   aws.String("subnet-0aaa111111111111a"),
				ResourceType:         aws.String("AWS::EC2::Subnet"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(mustParseTime("2024-10-15T09:03:00+00:00")),
				DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
					StackResourceDriftStatus: cfntypes.StackResourceDriftStatusInSync,
				},
			},
		},
		{
			ID:     "PrivateSubnet1",
			Name:   "PrivateSubnet1",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"logical_resource_id":  "PrivateSubnet1",
				"physical_resource_id": "subnet-0bbb222222222222b",
				"resource_type":        "AWS::EC2::Subnet",
				"resource_status":      "CREATE_COMPLETE",
				"drift_status":         "MODIFIED",
				"last_updated":         "2024-10-15 09:03:30",
			},
			RawStruct: cfntypes.StackResourceSummary{
				LogicalResourceId:    aws.String("PrivateSubnet1"),
				PhysicalResourceId:   aws.String("subnet-0bbb222222222222b"),
				ResourceType:         aws.String("AWS::EC2::Subnet"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(mustParseTime("2024-10-15T09:03:30+00:00")),
				DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
					StackResourceDriftStatus: cfntypes.StackResourceDriftStatusModified,
				},
			},
		},
		{
			ID:     "InternetGateway",
			Name:   "InternetGateway",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"logical_resource_id":  "InternetGateway",
				"physical_resource_id": "igw-0ccc333333333333c",
				"resource_type":        "AWS::EC2::InternetGateway",
				"resource_status":      "CREATE_COMPLETE",
				"drift_status":         "NOT_CHECKED",
				"last_updated":         "2024-10-15 09:04:00",
			},
			RawStruct: cfntypes.StackResourceSummary{
				LogicalResourceId:    aws.String("InternetGateway"),
				PhysicalResourceId:   aws.String("igw-0ccc333333333333c"),
				ResourceType:         aws.String("AWS::EC2::InternetGateway"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(mustParseTime("2024-10-15T09:04:00+00:00")),
				DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
					StackResourceDriftStatus: cfntypes.StackResourceDriftStatusNotChecked,
				},
			},
		},
		{
			ID:     "NATGateway",
			Name:   "NATGateway",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"logical_resource_id":  "NATGateway",
				"physical_resource_id": "nat-0ddd444444444444d",
				"resource_type":        "AWS::EC2::NatGateway",
				"resource_status":      "CREATE_COMPLETE",
				"drift_status":         "IN_SYNC",
				"last_updated":         "2024-10-15 09:06:00",
			},
			RawStruct: cfntypes.StackResourceSummary{
				LogicalResourceId:    aws.String("NATGateway"),
				PhysicalResourceId:   aws.String("nat-0ddd444444444444d"),
				ResourceType:         aws.String("AWS::EC2::NatGateway"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(mustParseTime("2024-10-15T09:06:00+00:00")),
				DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
					StackResourceDriftStatus: cfntypes.StackResourceDriftStatusInSync,
				},
			},
		},
		{
			ID:     "AppSecurityGroup",
			Name:   "AppSecurityGroup",
			Status: "UPDATE_COMPLETE",
			Fields: map[string]string{
				"logical_resource_id":  "AppSecurityGroup",
				"physical_resource_id": "sg-0eee555555555555e",
				"resource_type":        "AWS::EC2::SecurityGroup",
				"resource_status":      "UPDATE_COMPLETE",
				"drift_status":         "IN_SYNC",
				"last_updated":         "2026-03-20 10:02:00",
			},
			RawStruct: cfntypes.StackResourceSummary{
				LogicalResourceId:    aws.String("AppSecurityGroup"),
				PhysicalResourceId:   aws.String("sg-0eee555555555555e"),
				ResourceType:         aws.String("AWS::EC2::SecurityGroup"),
				ResourceStatus:       cfntypes.ResourceStatusUpdateComplete,
				LastUpdatedTimestamp: aws.Time(mustParseTime("2026-03-20T10:02:00+00:00")),
				DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
					StackResourceDriftStatus: cfntypes.StackResourceDriftStatusInSync,
				},
			},
		},
	}
}

// ecrImageFixtures returns demo ECR Image fixtures for a given repository.
func ecrImageFixtures(repoName, repoURI string) []resource.Resource {
	pushedAt1 := mustParseTime("2026-03-22T10:00:00+00:00")
	pushedAt2 := mustParseTime("2026-03-21T14:30:00+00:00")
	pushedAt3 := mustParseTime("2026-03-20T08:15:00+00:00")
	pushedAt4 := mustParseTime("2026-03-18T16:00:00+00:00")
	pushedAt5 := mustParseTime("2026-03-15T11:45:00+00:00")
	pushedAt6 := mustParseTime("2026-03-10T09:00:00+00:00")

	makeURI := func(tag string) string {
		if tag != "" {
			return repoURI + ":" + tag
		}
		return ""
	}

	makeDigestShort := func(digest string) string {
		s := strings.TrimPrefix(digest, "sha256:")
		if len(s) > 12 {
			return s[:12]
		}
		return s
	}

	digest1 := "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	digest2 := "sha256:b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3"
	digest3 := "sha256:c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"
	digest4 := "sha256:d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5"
	digest5 := "sha256:e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6"
	digest6 := "sha256:f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1"

	return []resource.Resource{
		{
			ID:     digest1,
			Name:   "latest, v2.3.1",
			Status: "",
			Fields: map[string]string{
				"image_tags":      "latest, v2.3.1",
				"digest_short":    makeDigestShort(digest1),
				"pushed_at":       "2026-03-22 10:00:00",
				"image_size":      "85 MB",
				"scan_status":     "COMPLETE",
				"finding_counts":  "",
				"image_uri":       makeURI("latest"),
				"image_digest":    digest1,
				"repository_name": repoName,
			},
			RawStruct: ecrtypes.ImageDetail{
				ImageDigest:      aws.String(digest1),
				ImageTags:        []string{"latest", "v2.3.1"},
				ImagePushedAt:    &pushedAt1,
				ImageSizeInBytes: aws.Int64(89128960),
				ImageScanStatus: &ecrtypes.ImageScanStatus{
					Status: ecrtypes.ScanStatusComplete,
				},
				ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
					FindingSeverityCounts: map[string]int32{},
				},
			},
		},
		{
			ID:     digest2,
			Name:   "v2.3.0",
			Status: "pending",
			Fields: map[string]string{
				"image_tags":      "v2.3.0",
				"digest_short":    makeDigestShort(digest2),
				"pushed_at":       "2026-03-21 14:30:00",
				"image_size":      "84 MB",
				"scan_status":     "COMPLETE",
				"finding_counts":  "2H 5M",
				"image_uri":       makeURI("v2.3.0"),
				"image_digest":    digest2,
				"repository_name": repoName,
			},
			RawStruct: ecrtypes.ImageDetail{
				ImageDigest:      aws.String(digest2),
				ImageTags:        []string{"v2.3.0"},
				ImagePushedAt:    &pushedAt2,
				ImageSizeInBytes: aws.Int64(88080384),
				ImageScanStatus: &ecrtypes.ImageScanStatus{
					Status: ecrtypes.ScanStatusComplete,
				},
				ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
					FindingSeverityCounts: map[string]int32{
						"HIGH":   2,
						"MEDIUM": 5,
					},
				},
			},
		},
		{
			ID:     digest3,
			Name:   "v2.2.0",
			Status: "failed",
			Fields: map[string]string{
				"image_tags":      "v2.2.0",
				"digest_short":    makeDigestShort(digest3),
				"pushed_at":       "2026-03-20 08:15:00",
				"image_size":      "82 MB",
				"scan_status":     "COMPLETE",
				"finding_counts":  "1C 3H 8M",
				"image_uri":       makeURI("v2.2.0"),
				"image_digest":    digest3,
				"repository_name": repoName,
			},
			RawStruct: ecrtypes.ImageDetail{
				ImageDigest:      aws.String(digest3),
				ImageTags:        []string{"v2.2.0"},
				ImagePushedAt:    &pushedAt3,
				ImageSizeInBytes: aws.Int64(85983232),
				ImageScanStatus: &ecrtypes.ImageScanStatus{
					Status: ecrtypes.ScanStatusComplete,
				},
				ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
					FindingSeverityCounts: map[string]int32{
						"CRITICAL": 1,
						"HIGH":     3,
						"MEDIUM":   8,
					},
				},
			},
		},
		{
			ID:     digest4,
			Name:   "staging",
			Status: "",
			Fields: map[string]string{
				"image_tags":      "staging",
				"digest_short":    makeDigestShort(digest4),
				"pushed_at":       "2026-03-18 16:00:00",
				"image_size":      "86 MB",
				"scan_status":     "COMPLETE",
				"finding_counts":  "",
				"image_uri":       makeURI("staging"),
				"image_digest":    digest4,
				"repository_name": repoName,
			},
			RawStruct: ecrtypes.ImageDetail{
				ImageDigest:      aws.String(digest4),
				ImageTags:        []string{"staging"},
				ImagePushedAt:    &pushedAt4,
				ImageSizeInBytes: aws.Int64(90177536),
				ImageScanStatus: &ecrtypes.ImageScanStatus{
					Status: ecrtypes.ScanStatusComplete,
				},
				ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
					FindingSeverityCounts: map[string]int32{},
				},
			},
		},
		{
			ID:     digest5,
			Name:   makeDigestShort(digest5),
			Status: "terminated",
			Fields: map[string]string{
				"image_tags":      "<untagged>",
				"digest_short":    makeDigestShort(digest5),
				"pushed_at":       "2026-03-15 11:45:00",
				"image_size":      "80 MB",
				"scan_status":     "",
				"finding_counts":  "",
				"image_uri":       fmt.Sprintf("%s@%s", repoURI, digest5),
				"image_digest":    digest5,
				"repository_name": repoName,
			},
			RawStruct: ecrtypes.ImageDetail{
				ImageDigest:      aws.String(digest5),
				ImageTags:        []string{},
				ImagePushedAt:    &pushedAt5,
				ImageSizeInBytes: aws.Int64(83886080),
			},
		},
		{
			ID:     digest6,
			Name:   "v2.1.0",
			Status: "failed",
			Fields: map[string]string{
				"image_tags":      "v2.1.0",
				"digest_short":    makeDigestShort(digest6),
				"pushed_at":       "2026-03-10 09:00:00",
				"image_size":      "79 MB",
				"scan_status":     "FAILED",
				"finding_counts":  "",
				"image_uri":       makeURI("v2.1.0"),
				"image_digest":    digest6,
				"repository_name": repoName,
			},
			RawStruct: ecrtypes.ImageDetail{
				ImageDigest:      aws.String(digest6),
				ImageTags:        []string{"v2.1.0"},
				ImagePushedAt:    &pushedAt6,
				ImageSizeInBytes: aws.Int64(82837504),
				ImageScanStatus: &ecrtypes.ImageScanStatus{
					Status: ecrtypes.ScanStatusFailed,
				},
			},
		},
	}
}
