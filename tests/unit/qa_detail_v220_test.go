package unit_test

import (
	"time"

	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	codebuildtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	codepipelinetypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
)

// ===========================================================================
// Realistic SDK struct builders for v2.2.0 resource types
// ===========================================================================

func realisticCFDistribution() cloudfronttypes.DistributionSummary {
	return cloudfronttypes.DistributionSummary{
		Id:         new("E1A2B3C4D5E6F7"),
		DomainName: new("d1234abcdef.cloudfront.net"),
		Status:     new("Deployed"),
		Enabled:    new(true),
		Comment:    new("Production CDN"),
		ARN:        new("arn:aws:cloudfront::123456789012:distribution/E1A2B3C4D5E6F7"),
		Aliases: &cloudfronttypes.Aliases{
			Quantity: new(int32(1)),
			Items:    []string{"cdn.example.com"},
		},
		PriceClass:       cloudfronttypes.PriceClassPriceClassAll,
		HttpVersion:      cloudfronttypes.HttpVersionHttp2,
		LastModifiedTime: new(testTime),
		Origins: &cloudfronttypes.Origins{
			Quantity: new(int32(1)),
			Items: []cloudfronttypes.Origin{
				{
					Id:         new("S3-origin"),
					DomainName: new("my-bucket.s3.amazonaws.com"),
				},
			},
		},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
			TargetOriginId:       new("S3-origin"),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyRedirectToHttps,
		},
	}
}

func realisticR53Zone() route53types.HostedZone {
	return route53types.HostedZone{
		Id:                     new("/hostedzone/Z1234567890ABC"),
		Name:                   new("example.com."),
		CallerReference:        new("unique-ref-20250615"),
		ResourceRecordSetCount: new(int64(42)),
		Config: &route53types.HostedZoneConfig{
			PrivateZone: false,
			Comment:     new("Production hosted zone"),
		},
	}
}

func realisticAPIGW() apigatewayv2types.Api {
	return apigatewayv2types.Api{
		ApiId:        new("abc123def4"),
		Name:         new("prod-api"),
		ProtocolType: apigatewayv2types.ProtocolTypeHttp,
		ApiEndpoint:  new("https://abc123def4.execute-api.us-east-1.amazonaws.com"),
		Description:  new("Production REST API"),
		CreatedDate:  new(testTime),
		Tags:         map[string]string{"env": "production"},
	}
}

func realisticECR() ecrtypes.Repository {
	return ecrtypes.Repository{
		RepositoryName:     new("my-app"),
		RepositoryUri:      new("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app"),
		RepositoryArn:      new("arn:aws:ecr:us-east-1:123456789012:repository/my-app"),
		RegistryId:         new("123456789012"),
		ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
		ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
			ScanOnPush: true,
		},
		EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
			EncryptionType: ecrtypes.EncryptionTypeAes256,
		},
		CreatedAt: new(testTime),
	}
}

func realisticEFS() efstypes.FileSystemDescription {
	return efstypes.FileSystemDescription{
		FileSystemId:         new("fs-0abc1234def56789a"),
		Name:                 new("prod-efs"),
		LifeCycleState:       efstypes.LifeCycleStateAvailable,
		PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
		ThroughputMode:       efstypes.ThroughputModeBursting,
		Encrypted:            new(true),
		NumberOfMountTargets: 3,
		FileSystemArn:        new("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0abc1234def56789a"),
		OwnerId:              new("123456789012"),
		SizeInBytes: &efstypes.FileSystemSize{
			Value: 1073741824,
		},
		CreationTime: new(testTime),
		Tags: []efstypes.Tag{
			{Key: new("env"), Value: new("production")},
		},
	}
}

func realisticEBRule() eventbridgetypes.Rule {
	return eventbridgetypes.Rule{
		Name:               new("daily-backup-rule"),
		Arn:                new("arn:aws:events:us-east-1:123456789012:rule/daily-backup-rule"),
		State:              eventbridgetypes.RuleStateEnabled,
		Description:        new("Daily backup trigger"),
		EventBusName:       new("default"),
		ScheduleExpression: new("cron(0 2 * * ? *)"),
		RoleArn:            new("arn:aws:iam::123456789012:role/backup-role"),
	}
}

func realisticSFN() sfntypes.StateMachineListItem {
	return sfntypes.StateMachineListItem{
		Name:            new("order-processing"),
		StateMachineArn: new("arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"),
		Type:            sfntypes.StateMachineTypeStandard,
		CreationDate:    new(testTime),
	}
}

func realisticPipeline() codepipelinetypes.PipelineSummary {
	return codepipelinetypes.PipelineSummary{
		Name:          new("deploy-pipeline"),
		PipelineType:  codepipelinetypes.PipelineTypeV2,
		Version:       new(int32(3)),
		Created:       new(testTime),
		Updated:       new(testTime),
		ExecutionMode: codepipelinetypes.ExecutionModeQueued,
	}
}

func realisticKinesis() kinesistypes.StreamSummary {
	return kinesistypes.StreamSummary{
		StreamName:              new("events-stream"),
		StreamARN:               new("arn:aws:kinesis:us-east-1:123456789012:stream/events-stream"),
		StreamStatus:            kinesistypes.StreamStatusActive,
		StreamCreationTimestamp: new(testTime),
		StreamModeDetails: &kinesistypes.StreamModeDetails{
			StreamMode: kinesistypes.StreamModeOnDemand,
		},
	}
}

func realisticWAF() wafv2types.WebACLSummary {
	return wafv2types.WebACLSummary{
		Name:        new("prod-waf-acl"),
		Id:          new("a1b2c3d4-5678-90ab-cdef-EXAMPLE11111"),
		ARN:         new("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/prod-waf-acl/a1b2c3d4"),
		Description: new("Production WAF rules"),
		LockToken:   new("abcdef12-3456-7890-abcd-ef1234567890"),
	}
}

func realisticGlueJob() gluetypes.Job {
	return gluetypes.Job{
		Name:            new("etl-daily-job"),
		Role:            new("arn:aws:iam::123456789012:role/glue-role"),
		GlueVersion:     new("4.0"),
		WorkerType:      gluetypes.WorkerTypeG2x,
		NumberOfWorkers: new(int32(10)),
		MaxRetries:      3,
		Command: &gluetypes.JobCommand{
			Name: new("glueetl"),
		},
		CreatedOn:      new(testTime),
		LastModifiedOn: new(testTime),
	}
}

func realisticEB() ebtypes.EnvironmentDescription {
	return ebtypes.EnvironmentDescription{
		EnvironmentName:   new("prod-api-env"),
		EnvironmentId:     new("e-abc1234def"),
		ApplicationName:   new("my-web-app"),
		Status:            ebtypes.EnvironmentStatusReady,
		Health:            ebtypes.EnvironmentHealthGreen,
		HealthStatus:      ebtypes.EnvironmentHealthStatusOk,
		VersionLabel:      new("v1.2.3"),
		SolutionStackName: new("64bit Amazon Linux 2023 v4.3.0 running Docker"),
		PlatformArn:       new("arn:aws:elasticbeanstalk:us-east-1::platform/Docker running on 64bit Amazon Linux 2023/4.3.0"),
		EndpointURL:       new("awseb-e-abc1234def.us-east-1.elb.amazonaws.com"),
		CNAME:             new("prod-api-env.us-east-1.elasticbeanstalk.com"),
		DateCreated:       new(testTime),
		DateUpdated:       new(testTime),
		EnvironmentArn:    new("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/my-web-app/prod-api-env"),
	}
}

func realisticRedshift() redshifttypes.Cluster {
	return redshifttypes.Cluster{
		ClusterIdentifier:   new("analytics-cluster"),
		ClusterStatus:       new("available"),
		NodeType:            new("dc2.large"),
		NumberOfNodes:       new(int32(4)),
		DBName:              new("analytics_db"),
		MasterUsername:      new("admin"),
		ClusterCreateTime:   new(testTime),
		ClusterNamespaceArn: new("arn:aws:redshift:us-east-1:123456789012:namespace:abc-123"),
		AvailabilityZone:    new("us-east-1a"),
		Endpoint: &redshifttypes.Endpoint{
			Address: new("analytics-cluster.abc123.us-east-1.redshift.amazonaws.com"),
			Port:    new(int32(5439)),
		},
	}
}

func realisticTrail() cloudtrailtypes.Trail {
	return cloudtrailtypes.Trail{
		Name:                       new("org-trail"),
		TrailARN:                   new("arn:aws:cloudtrail:us-east-1:123456789012:trail/org-trail"),
		S3BucketName:               new("cloudtrail-logs-bucket"),
		HomeRegion:                 new("us-east-1"),
		IsMultiRegionTrail:         new(true),
		IsOrganizationTrail:        new(true),
		LogFileValidationEnabled:   new(true),
		IncludeGlobalServiceEvents: new(true),
		KmsKeyId:                   new("arn:aws:kms:us-east-1:123456789012:key/12345678"),
		CloudWatchLogsLogGroupArn:  new("arn:aws:logs:us-east-1:123456789012:log-group:cloudtrail-logs"),
	}
}

func realisticAthena() athenatypes.WorkGroupSummary {
	return athenatypes.WorkGroupSummary{
		Name:         new("analytics-wg"),
		State:        athenatypes.WorkGroupStateEnabled,
		Description:  new("Analytics workgroup"),
		CreationTime: new(testTime),
		EngineVersion: &athenatypes.EngineVersion{
			EffectiveEngineVersion: new("Athena engine version 3"),
		},
	}
}

func realisticCodeArtifact() codeartifacttypes.RepositorySummary {
	return codeartifacttypes.RepositorySummary{
		Name:                 new("shared-libs"),
		DomainName:           new("my-domain"),
		DomainOwner:          new("123456789012"),
		Arn:                  new("arn:aws:codeartifact:us-east-1:123456789012:repository/my-domain/shared-libs"),
		Description:          new("Shared libraries repository"),
		AdministratorAccount: new("123456789012"),
		CreatedTime:          new(testTime),
	}
}

func realisticCodeBuild() codebuildtypes.Project {
	return codebuildtypes.Project{
		Name:        new("build-project"),
		Description: new("CI build project"),
		Arn:         new("arn:aws:codebuild:us-east-1:123456789012:project/build-project"),
		Source: &codebuildtypes.ProjectSource{
			Type: codebuildtypes.SourceTypeCodecommit,
		},
		Environment: &codebuildtypes.ProjectEnvironment{
			Type:  codebuildtypes.EnvironmentTypeLinuxContainer,
			Image: new("aws/codebuild/standard:7.0"),
		},
		ServiceRole:  new("arn:aws:iam::123456789012:role/codebuild-role"),
		Created:      new(testTime),
		LastModified: new(testTime),
		Tags: []codebuildtypes.Tag{
			{Key: new("env"), Value: new("production")},
		},
	}
}

func realisticOpenSearch() opensearchtypes.DomainStatus {
	return opensearchtypes.DomainStatus{
		DomainName:    new("search-prod"),
		DomainId:      new("123456789012/search-prod"),
		ARN:           new("arn:aws:es:us-east-1:123456789012:domain/search-prod"),
		EngineVersion: new("OpenSearch_2.11"),
		Endpoint:      new("search-prod-abc123.us-east-1.es.amazonaws.com"),
		ClusterConfig: &opensearchtypes.ClusterConfig{
			InstanceType:  opensearchtypes.OpenSearchPartitionInstanceTypeR6gLargeSearch,
			InstanceCount: new(int32(3)),
		},
		EBSOptions: &opensearchtypes.EBSOptions{
			EBSEnabled: new(true),
			VolumeType: opensearchtypes.VolumeTypeGp3,
			VolumeSize: new(int32(100)),
		},
		Created: new(true),
		Deleted: new(false),
	}
}

func realisticKMS() *kmstypes.KeyMetadata {
	return &kmstypes.KeyMetadata{
		KeyId:        new("12345678-1234-1234-1234-123456789012"),
		Arn:          new("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
		Description:  new("Production encryption key"),
		KeyState:     kmstypes.KeyStateEnabled,
		KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
		KeySpec:      kmstypes.KeySpecSymmetricDefault,
		KeyManager:   kmstypes.KeyManagerTypeCustomer,
		Enabled:      true,
		CreationDate: new(testTime),
		Origin:       kmstypes.OriginTypeAwsKms,
		MultiRegion:  new(false),
	}
}

func realisticMSK() kafkatypes.Cluster {
	return kafkatypes.Cluster{
		ClusterName:    new("events-kafka"),
		ClusterArn:     new("arn:aws:kafka:us-east-1:123456789012:cluster/events-kafka/abc-123"),
		ClusterType:    kafkatypes.ClusterTypeProvisioned,
		State:          kafkatypes.ClusterStateActive,
		CurrentVersion: new("K3AEGXETSR30VB"),
		CreationTime:   new(testTime),
		Tags:           map[string]string{"env": "production"},
	}
}

func realisticBackup() backuptypes.BackupPlansListMember {
	return backuptypes.BackupPlansListMember{
		BackupPlanName:    new("daily-backup-plan"),
		BackupPlanId:      new("abc12345-1234-1234-1234-123456789012"),
		BackupPlanArn:     new("arn:aws:backup:us-east-1:123456789012:backup-plan:abc12345"),
		CreationDate:      new(testTime),
		LastExecutionDate: new(testTime),
		VersionId:         new("MjEyYzUyNzUtNWU0MC00NTBjLThjNDktOGQ1YzkyZGIwODlh"),
	}
}

// ===========================================================================
// 1. CloudFront (cf)
// ===========================================================================

var _ = time.Now

// ===========================================================================
// Realistic SDK struct builders for EBS, Snapshots, AMIs, CloudTrail Events
// ===========================================================================

func realisticVolume() ec2types.Volume {
	createTime := time.Date(2025, 3, 10, 14, 0, 0, 0, time.UTC)
	return ec2types.Volume{
		VolumeId:         new("vol-111aabbcc"),
		State:            ec2types.VolumeStateInUse,
		Size:             new(int32(100)),
		VolumeType:       ec2types.VolumeTypeGp3,
		Iops:             new(int32(3000)),
		Encrypted:        new(true),
		AvailabilityZone: new("us-east-1a"),
		CreateTime:       &createTime,
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-data-vol")},
			{Key: new("env"), Value: new("production")},
		},
		Attachments: []ec2types.VolumeAttachment{
			{
				InstanceId: new("i-0abc123456def789"),
				State:      ec2types.VolumeAttachmentStateAttached,
				Device:     new("/dev/xvdf"),
			},
		},
	}
}

func realisticSnapshot() ec2types.Snapshot {
	startTime := time.Date(2025, 2, 20, 9, 15, 0, 0, time.UTC)
	return ec2types.Snapshot{
		SnapshotId:  new("snap-0aabb11cc"),
		State:       ec2types.SnapshotStateCompleted,
		VolumeId:    new("vol-111aabbcc"),
		VolumeSize:  new(int32(100)),
		Encrypted:   new(true),
		Description: new("Daily backup snapshot"),
		StartTime:   &startTime,
		Progress:    new("100%"),
		OwnerId:     new("123456789012"),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-snap-daily")},
		},
	}
}

func realisticImage() ec2types.Image {
	return ec2types.Image{
		ImageId:         new("ami-0abc111222333444a"),
		Name:            new("my-web-server-ami"),
		State:           ec2types.ImageStateAvailable,
		Architecture:    ec2types.ArchitectureValuesX8664,
		PlatformDetails: new("Linux/UNIX"),
		RootDeviceType:  ec2types.DeviceTypeEbs,
		CreationDate:    new("2025-01-15T10:30:00.000Z"),
		Public:          new(false),
		OwnerId:         new("123456789012"),
		Description:     new("Web server base image"),
	}
}

func realisticCloudTrailEvent() cloudtrailtypes.Event {
	eventTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	return cloudtrailtypes.Event{
		EventId:         new("evt-0001-abcd-1234-5678-abcdef012345"),
		EventName:       new("RunInstances"),
		EventTime:       &eventTime,
		EventSource:     new("ec2.amazonaws.com"),
		Username:        new("admin"),
		ReadOnly:        new("false"),
		CloudTrailEvent: new(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE123","arn":"arn:aws:sts::123456789012:assumed-role/test-role/session","accountId":"123456789012"},"eventTime":"2025-03-15T12:00:00Z","eventSource":"ec2.amazonaws.com","eventName":"RunInstances","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.1","userAgent":"console.ec2.amazonaws.com","requestParameters":{"instanceType":"t3.micro","instancesSet":{"items":[{"imageId":"ami-0abc123456def789"}]}},"responseElements":{"instancesSet":{"items":[{"instanceId":"i-0abc123456def789"}]}},"requestID":"req-0001-abcd-1234","eventID":"evt-0001-abcd-1234-5678-abcdef012345","readOnly":false,"eventType":"AwsApiCall","managementEvent":true}`),
		Resources: []cloudtrailtypes.Resource{
			{
				ResourceType: new("AWS::EC2::Instance"),
				ResourceName: new("i-0abc123456def789"),
			},
		},
	}
}

// ===========================================================================
// 23. EBS Volume
// ===========================================================================
