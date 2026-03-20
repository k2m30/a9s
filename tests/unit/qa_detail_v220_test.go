package unit_test

import (
	"strings"
	"testing"
	"time"

	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	codebuildtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	codepipelinetypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
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
		Id:         ptrString("E1A2B3C4D5E6F7"),
		DomainName: ptrString("d1234abcdef.cloudfront.net"),
		Status:     ptrString("Deployed"),
		Enabled:    ptrBool(true),
		Comment:    ptrString("Production CDN"),
		ARN:        ptrString("arn:aws:cloudfront::123456789012:distribution/E1A2B3C4D5E6F7"),
		Aliases: &cloudfronttypes.Aliases{
			Quantity: ptrInt32(1),
			Items:    []string{"cdn.example.com"},
		},
		PriceClass:       cloudfronttypes.PriceClassPriceClassAll,
		HttpVersion:      cloudfronttypes.HttpVersionHttp2,
		LastModifiedTime: ptrTime(testTime),
		Origins: &cloudfronttypes.Origins{
			Quantity: ptrInt32(1),
			Items: []cloudfronttypes.Origin{
				{
					Id:         ptrString("S3-origin"),
					DomainName: ptrString("my-bucket.s3.amazonaws.com"),
				},
			},
		},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
			TargetOriginId:       ptrString("S3-origin"),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyRedirectToHttps,
		},
	}
}

func realisticR53Zone() route53types.HostedZone {
	return route53types.HostedZone{
		Id:                     ptrString("/hostedzone/Z1234567890ABC"),
		Name:                   ptrString("example.com."),
		CallerReference:        ptrString("unique-ref-20250615"),
		ResourceRecordSetCount: ptrInt64(42),
		Config: &route53types.HostedZoneConfig{
			PrivateZone: false,
			Comment:     ptrString("Production hosted zone"),
		},
	}
}

func realisticAPIGW() apigatewayv2types.Api {
	return apigatewayv2types.Api{
		ApiId:        ptrString("abc123def4"),
		Name:         ptrString("prod-api"),
		ProtocolType: apigatewayv2types.ProtocolTypeHttp,
		ApiEndpoint:  ptrString("https://abc123def4.execute-api.us-east-1.amazonaws.com"),
		Description:  ptrString("Production REST API"),
		CreatedDate:  ptrTime(testTime),
		Tags:         map[string]string{"env": "production"},
	}
}

func realisticECR() ecrtypes.Repository {
	return ecrtypes.Repository{
		RepositoryName: ptrString("my-app"),
		RepositoryUri:  ptrString("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app"),
		RepositoryArn:  ptrString("arn:aws:ecr:us-east-1:123456789012:repository/my-app"),
		RegistryId:     ptrString("123456789012"),
		ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
		ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
			ScanOnPush: true,
		},
		EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
			EncryptionType: ecrtypes.EncryptionTypeAes256,
		},
		CreatedAt: ptrTime(testTime),
	}
}

func realisticEFS() efstypes.FileSystemDescription {
	return efstypes.FileSystemDescription{
		FileSystemId:         ptrString("fs-0abc1234def56789a"),
		Name:                 ptrString("prod-efs"),
		LifeCycleState:       efstypes.LifeCycleStateAvailable,
		PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
		ThroughputMode:       efstypes.ThroughputModeBursting,
		Encrypted:            ptrBool(true),
		NumberOfMountTargets: 3,
		FileSystemArn:        ptrString("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0abc1234def56789a"),
		OwnerId:              ptrString("123456789012"),
		SizeInBytes: &efstypes.FileSystemSize{
			Value: 1073741824,
		},
		CreationTime: ptrTime(testTime),
		Tags: []efstypes.Tag{
			{Key: ptrString("env"), Value: ptrString("production")},
		},
	}
}

func realisticEBRule() eventbridgetypes.Rule {
	return eventbridgetypes.Rule{
		Name:               ptrString("daily-backup-rule"),
		Arn:                ptrString("arn:aws:events:us-east-1:123456789012:rule/daily-backup-rule"),
		State:              eventbridgetypes.RuleStateEnabled,
		Description:        ptrString("Daily backup trigger"),
		EventBusName:       ptrString("default"),
		ScheduleExpression: ptrString("cron(0 2 * * ? *)"),
		RoleArn:            ptrString("arn:aws:iam::123456789012:role/backup-role"),
	}
}

func realisticSFN() sfntypes.StateMachineListItem {
	return sfntypes.StateMachineListItem{
		Name:            ptrString("order-processing"),
		StateMachineArn: ptrString("arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"),
		Type:            sfntypes.StateMachineTypeStandard,
		CreationDate:    ptrTime(testTime),
	}
}

func realisticPipeline() codepipelinetypes.PipelineSummary {
	return codepipelinetypes.PipelineSummary{
		Name:          ptrString("deploy-pipeline"),
		PipelineType:  codepipelinetypes.PipelineTypeV2,
		Version:       ptrInt32(3),
		Created:       ptrTime(testTime),
		Updated:       ptrTime(testTime),
		ExecutionMode: codepipelinetypes.ExecutionModeQueued,
	}
}

func realisticKinesis() kinesistypes.StreamSummary {
	return kinesistypes.StreamSummary{
		StreamName:              ptrString("events-stream"),
		StreamARN:               ptrString("arn:aws:kinesis:us-east-1:123456789012:stream/events-stream"),
		StreamStatus:            kinesistypes.StreamStatusActive,
		StreamCreationTimestamp: ptrTime(testTime),
		StreamModeDetails: &kinesistypes.StreamModeDetails{
			StreamMode: kinesistypes.StreamModeOnDemand,
		},
	}
}

func realisticWAF() wafv2types.WebACLSummary {
	return wafv2types.WebACLSummary{
		Name:        ptrString("prod-waf-acl"),
		Id:          ptrString("a1b2c3d4-5678-90ab-cdef-EXAMPLE11111"),
		ARN:         ptrString("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/prod-waf-acl/a1b2c3d4"),
		Description: ptrString("Production WAF rules"),
		LockToken:   ptrString("abcdef12-3456-7890-abcd-ef1234567890"),
	}
}

func realisticGlueJob() gluetypes.Job {
	return gluetypes.Job{
		Name:            ptrString("etl-daily-job"),
		Role:            ptrString("arn:aws:iam::123456789012:role/glue-role"),
		GlueVersion:     ptrString("4.0"),
		WorkerType:      gluetypes.WorkerTypeG2x,
		NumberOfWorkers: ptrInt32(10),
		MaxRetries:      3,
		Command: &gluetypes.JobCommand{
			Name: ptrString("glueetl"),
		},
		CreatedOn:      ptrTime(testTime),
		LastModifiedOn: ptrTime(testTime),
	}
}

func realisticEB() ebtypes.EnvironmentDescription {
	return ebtypes.EnvironmentDescription{
		EnvironmentName:   ptrString("prod-api-env"),
		EnvironmentId:     ptrString("e-abc1234def"),
		ApplicationName:   ptrString("my-web-app"),
		Status:            ebtypes.EnvironmentStatusReady,
		Health:            ebtypes.EnvironmentHealthGreen,
		HealthStatus:      ebtypes.EnvironmentHealthStatusOk,
		VersionLabel:      ptrString("v1.2.3"),
		SolutionStackName: ptrString("64bit Amazon Linux 2023 v4.3.0 running Docker"),
		PlatformArn:       ptrString("arn:aws:elasticbeanstalk:us-east-1::platform/Docker running on 64bit Amazon Linux 2023/4.3.0"),
		EndpointURL:       ptrString("awseb-e-abc1234def.us-east-1.elb.amazonaws.com"),
		CNAME:             ptrString("prod-api-env.us-east-1.elasticbeanstalk.com"),
		DateCreated:       ptrTime(testTime),
		DateUpdated:       ptrTime(testTime),
		EnvironmentArn:    ptrString("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/my-web-app/prod-api-env"),
	}
}

func realisticRedshift() redshifttypes.Cluster {
	return redshifttypes.Cluster{
		ClusterIdentifier:   ptrString("analytics-cluster"),
		ClusterStatus:       ptrString("available"),
		NodeType:            ptrString("dc2.large"),
		NumberOfNodes:       ptrInt32(4),
		DBName:              ptrString("analytics_db"),
		MasterUsername:      ptrString("admin"),
		ClusterCreateTime:   ptrTime(testTime),
		ClusterNamespaceArn: ptrString("arn:aws:redshift:us-east-1:123456789012:namespace:abc-123"),
		AvailabilityZone:    ptrString("us-east-1a"),
		Endpoint: &redshifttypes.Endpoint{
			Address: ptrString("analytics-cluster.abc123.us-east-1.redshift.amazonaws.com"),
			Port:    ptrInt32(5439),
		},
	}
}

func realisticTrail() cloudtrailtypes.Trail {
	return cloudtrailtypes.Trail{
		Name:                       ptrString("org-trail"),
		TrailARN:                   ptrString("arn:aws:cloudtrail:us-east-1:123456789012:trail/org-trail"),
		S3BucketName:               ptrString("cloudtrail-logs-bucket"),
		HomeRegion:                 ptrString("us-east-1"),
		IsMultiRegionTrail:         ptrBool(true),
		IsOrganizationTrail:        ptrBool(true),
		LogFileValidationEnabled:   ptrBool(true),
		IncludeGlobalServiceEvents: ptrBool(true),
		KmsKeyId:                   ptrString("arn:aws:kms:us-east-1:123456789012:key/12345678"),
		CloudWatchLogsLogGroupArn:  ptrString("arn:aws:logs:us-east-1:123456789012:log-group:cloudtrail-logs"),
	}
}

func realisticAthena() athenatypes.WorkGroupSummary {
	return athenatypes.WorkGroupSummary{
		Name:         ptrString("analytics-wg"),
		State:        athenatypes.WorkGroupStateEnabled,
		Description:  ptrString("Analytics workgroup"),
		CreationTime: ptrTime(testTime),
		EngineVersion: &athenatypes.EngineVersion{
			EffectiveEngineVersion: ptrString("Athena engine version 3"),
		},
	}
}

func realisticCodeArtifact() codeartifacttypes.RepositorySummary {
	return codeartifacttypes.RepositorySummary{
		Name:                 ptrString("shared-libs"),
		DomainName:           ptrString("my-domain"),
		DomainOwner:          ptrString("123456789012"),
		Arn:                  ptrString("arn:aws:codeartifact:us-east-1:123456789012:repository/my-domain/shared-libs"),
		Description:          ptrString("Shared libraries repository"),
		AdministratorAccount: ptrString("123456789012"),
		CreatedTime:          ptrTime(testTime),
	}
}

func realisticCodeBuild() codebuildtypes.Project {
	return codebuildtypes.Project{
		Name:        ptrString("build-project"),
		Description: ptrString("CI build project"),
		Arn:         ptrString("arn:aws:codebuild:us-east-1:123456789012:project/build-project"),
		Source: &codebuildtypes.ProjectSource{
			Type: codebuildtypes.SourceTypeCodecommit,
		},
		Environment: &codebuildtypes.ProjectEnvironment{
			Type:  codebuildtypes.EnvironmentTypeLinuxContainer,
			Image: ptrString("aws/codebuild/standard:7.0"),
		},
		ServiceRole:  ptrString("arn:aws:iam::123456789012:role/codebuild-role"),
		Created:      ptrTime(testTime),
		LastModified: ptrTime(testTime),
		Tags: []codebuildtypes.Tag{
			{Key: ptrString("env"), Value: ptrString("production")},
		},
	}
}

func realisticOpenSearch() opensearchtypes.DomainStatus {
	return opensearchtypes.DomainStatus{
		DomainName:    ptrString("search-prod"),
		DomainId:      ptrString("123456789012/search-prod"),
		ARN:           ptrString("arn:aws:es:us-east-1:123456789012:domain/search-prod"),
		EngineVersion: ptrString("OpenSearch_2.11"),
		Endpoint:      ptrString("search-prod-abc123.us-east-1.es.amazonaws.com"),
		ClusterConfig: &opensearchtypes.ClusterConfig{
			InstanceType:  opensearchtypes.OpenSearchPartitionInstanceTypeR6gLargeSearch,
			InstanceCount: ptrInt32(3),
		},
		EBSOptions: &opensearchtypes.EBSOptions{
			EBSEnabled: ptrBool(true),
			VolumeType: opensearchtypes.VolumeTypeGp3,
			VolumeSize: ptrInt32(100),
		},
		Created: ptrBool(true),
		Deleted: ptrBool(false),
	}
}

func realisticKMS() *kmstypes.KeyMetadata {
	return &kmstypes.KeyMetadata{
		KeyId:        ptrString("12345678-1234-1234-1234-123456789012"),
		Arn:          ptrString("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
		Description:  ptrString("Production encryption key"),
		KeyState:     kmstypes.KeyStateEnabled,
		KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
		KeySpec:      kmstypes.KeySpecSymmetricDefault,
		KeyManager:   kmstypes.KeyManagerTypeCustomer,
		Enabled:      true,
		CreationDate: ptrTime(testTime),
		Origin:       kmstypes.OriginTypeAwsKms,
		MultiRegion:  ptrBool(false),
	}
}

func realisticMSK() kafkatypes.Cluster {
	return kafkatypes.Cluster{
		ClusterName:    ptrString("events-kafka"),
		ClusterArn:     ptrString("arn:aws:kafka:us-east-1:123456789012:cluster/events-kafka/abc-123"),
		ClusterType:    kafkatypes.ClusterTypeProvisioned,
		State:          kafkatypes.ClusterStateActive,
		CurrentVersion: ptrString("K3AEGXETSR30VB"),
		CreationTime:   ptrTime(testTime),
		Tags:           map[string]string{"env": "production"},
	}
}

func realisticBackup() backuptypes.BackupPlansListMember {
	return backuptypes.BackupPlansListMember{
		BackupPlanName:    ptrString("daily-backup-plan"),
		BackupPlanId:      ptrString("abc12345-1234-1234-1234-123456789012"),
		BackupPlanArn:     ptrString("arn:aws:backup:us-east-1:123456789012:backup-plan:abc12345"),
		CreationDate:      ptrTime(testTime),
		LastExecutionDate: ptrTime(testTime),
		VersionId:         ptrString("MjEyYzUyNzUtNWU0MC00NTBjLThjNDktOGQ1YzkyZGIwODlh"),
	}
}

// ===========================================================================
// 1. CloudFront (cf)
// ===========================================================================

func TestQA_Detail_CF_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	cf := realisticCFDistribution()
	res := buildResource("E1A2B3C4D5E6F7", "E1A2B3C4D5E6F7", cf)
	cfg := detailConfigForType("cf")
	m := newDetailModel(res, "cf", cfg)

	view := m.View()
	for _, expected := range []string{
		"Id", "E1A2B3C4D5E6F7",
		"DomainName", "d1234abcdef.cloudfront.net",
		"Status", "Deployed",
		"Enabled", "Yes",
		"Comment", "Production CDN",
		"PriceClass", "PriceClass_All",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("CF detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_CF_NilFields(t *testing.T) {
	ensureNoColor(t)
	cf := cloudfronttypes.DistributionSummary{}
	res := buildResource("empty-cf", "empty-cf", cf)
	cfg := detailConfigForType("cf")
	m := newDetailModel(res, "cf", cfg)

	view := m.View()
	if view == "" {
		t.Error("CF detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_CF_FrameTitle(t *testing.T) {
	cf := realisticCFDistribution()
	res := buildResource("E1A2B3C4D5E6F7", "E1A2B3C4D5E6F7", cf)
	cfg := detailConfigForType("cf")
	m := newDetailModel(res, "cf", cfg)

	if title := m.FrameTitle(); title != "E1A2B3C4D5E6F7" {
		t.Errorf("CF FrameTitle expected %q, got %q", "E1A2B3C4D5E6F7", title)
	}
}

// ===========================================================================
// 2. Route 53 (r53)
// ===========================================================================

func TestQA_Detail_R53_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	zone := realisticR53Zone()
	res := buildResource("/hostedzone/Z1234567890ABC", "example.com.", zone)
	cfg := detailConfigForType("r53")
	m := newDetailModel(res, "r53", cfg)

	view := m.View()
	for _, expected := range []string{
		"Id", "/hostedzone/Z1234567890ABC",
		"Name", "example.com.",
		"CallerReference", "unique-ref-20250615",
		"ResourceRecordSetCo", "42",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("R53 detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_R53_NilFields(t *testing.T) {
	ensureNoColor(t)
	zone := route53types.HostedZone{}
	res := buildResource("empty-r53", "empty-r53", zone)
	cfg := detailConfigForType("r53")
	m := newDetailModel(res, "r53", cfg)

	view := m.View()
	if view == "" {
		t.Error("R53 detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_R53_FrameTitle(t *testing.T) {
	zone := realisticR53Zone()
	res := buildResource("/hostedzone/Z1234567890ABC", "example.com.", zone)
	cfg := detailConfigForType("r53")
	m := newDetailModel(res, "r53", cfg)

	if title := m.FrameTitle(); title != "example.com." {
		t.Errorf("R53 FrameTitle expected %q, got %q", "example.com.", title)
	}
}

// ===========================================================================
// 3. API Gateway (apigw)
// ===========================================================================

func TestQA_Detail_APIGW_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	api := realisticAPIGW()
	res := buildResource("abc123def4", "prod-api", api)
	cfg := detailConfigForType("apigw")
	m := newDetailModel(res, "apigw", cfg)

	view := m.View()
	for _, expected := range []string{
		"ApiId", "abc123def4",
		"Name", "prod-api",
		"ProtocolType", "HTTP",
		"ApiEndpoint", "https://abc123def4.execute-api.us-east-1.amazonaws.com",
		"Description", "Production REST API",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("APIGW detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_APIGW_NilFields(t *testing.T) {
	ensureNoColor(t)
	api := apigatewayv2types.Api{}
	res := buildResource("empty-apigw", "empty-apigw", api)
	cfg := detailConfigForType("apigw")
	m := newDetailModel(res, "apigw", cfg)

	view := m.View()
	if view == "" {
		t.Error("APIGW detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_APIGW_FrameTitle(t *testing.T) {
	api := realisticAPIGW()
	res := buildResource("abc123def4", "prod-api", api)
	cfg := detailConfigForType("apigw")
	m := newDetailModel(res, "apigw", cfg)

	if title := m.FrameTitle(); title != "prod-api" {
		t.Errorf("APIGW FrameTitle expected %q, got %q", "prod-api", title)
	}
}

// ===========================================================================
// 4. ECR
// ===========================================================================

func TestQA_Detail_ECR_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	repo := realisticECR()
	res := buildResource("my-app", "my-app", repo)
	cfg := detailConfigForType("ecr")
	m := newDetailModel(res, "ecr", cfg)

	view := m.View()
	for _, expected := range []string{
		"RepositoryName", "my-app",
		"RepositoryUri", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
		"ImageTagMutability", "IMMUTABLE",
		"CreatedAt", "2025-06-15",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ECR detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ECR_NilFields(t *testing.T) {
	ensureNoColor(t)
	repo := ecrtypes.Repository{}
	res := buildResource("empty-ecr", "empty-ecr", repo)
	cfg := detailConfigForType("ecr")
	m := newDetailModel(res, "ecr", cfg)

	view := m.View()
	if view == "" {
		t.Error("ECR detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_ECR_FrameTitle(t *testing.T) {
	repo := realisticECR()
	res := buildResource("my-app", "my-app", repo)
	cfg := detailConfigForType("ecr")
	m := newDetailModel(res, "ecr", cfg)

	if title := m.FrameTitle(); title != "my-app" {
		t.Errorf("ECR FrameTitle expected %q, got %q", "my-app", title)
	}
}

// ===========================================================================
// 5. EFS
// ===========================================================================

func TestQA_Detail_EFS_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	fs := realisticEFS()
	res := buildResource("fs-0abc1234def56789a", "prod-efs", fs)
	cfg := detailConfigForType("efs")
	m := newDetailModel(res, "efs", cfg)

	view := m.View()
	for _, expected := range []string{
		"FileSystemId", "fs-0abc1234def56789a",
		"Name", "prod-efs",
		"LifeCycleState", "available",
		"PerformanceMode", "generalPurpose",
		"Encrypted", "Yes",
		"NumberOfMountTarget", "3",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("EFS detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_EFS_NilFields(t *testing.T) {
	ensureNoColor(t)
	fs := efstypes.FileSystemDescription{}
	res := buildResource("empty-efs", "empty-efs", fs)
	cfg := detailConfigForType("efs")
	m := newDetailModel(res, "efs", cfg)

	view := m.View()
	if view == "" {
		t.Error("EFS detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_EFS_FrameTitle(t *testing.T) {
	fs := realisticEFS()
	res := buildResource("fs-0abc1234def56789a", "prod-efs", fs)
	cfg := detailConfigForType("efs")
	m := newDetailModel(res, "efs", cfg)

	if title := m.FrameTitle(); title != "prod-efs" {
		t.Errorf("EFS FrameTitle expected %q, got %q", "prod-efs", title)
	}
}

// ===========================================================================
// 6. EventBridge Rule (eb-rule)
// ===========================================================================

func TestQA_Detail_EBRule_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	rule := realisticEBRule()
	res := buildResource("daily-backup-rule", "daily-backup-rule", rule)
	cfg := detailConfigForType("eb-rule")
	m := newDetailModel(res, "eb-rule", cfg)

	view := m.View()
	for _, expected := range []string{
		"Name", "daily-backup-rule",
		"State", "ENABLED",
		"Description", "Daily backup trigger",
		"EventBusName", "default",
		"ScheduleExpression", "cron(0 2 * * ? *)",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("EBRule detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_EBRule_NilFields(t *testing.T) {
	ensureNoColor(t)
	rule := eventbridgetypes.Rule{}
	res := buildResource("empty-rule", "empty-rule", rule)
	cfg := detailConfigForType("eb-rule")
	m := newDetailModel(res, "eb-rule", cfg)

	view := m.View()
	if view == "" {
		t.Error("EBRule detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_EBRule_FrameTitle(t *testing.T) {
	rule := realisticEBRule()
	res := buildResource("daily-backup-rule", "daily-backup-rule", rule)
	cfg := detailConfigForType("eb-rule")
	m := newDetailModel(res, "eb-rule", cfg)

	if title := m.FrameTitle(); title != "daily-backup-rule" {
		t.Errorf("EBRule FrameTitle expected %q, got %q", "daily-backup-rule", title)
	}
}

// ===========================================================================
// 7. Step Functions (sfn)
// ===========================================================================

func TestQA_Detail_SFN_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	sm := realisticSFN()
	res := buildResource("order-processing", "order-processing", sm)
	cfg := detailConfigForType("sfn")
	m := newDetailModel(res, "sfn", cfg)

	view := m.View()
	for _, expected := range []string{
		"Name", "order-processing",
		"StateMachineArn",
		"Type", "STANDARD",
		"CreationDate", "2025-06-15",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("SFN detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_SFN_NilFields(t *testing.T) {
	ensureNoColor(t)
	sm := sfntypes.StateMachineListItem{}
	res := buildResource("empty-sfn", "empty-sfn", sm)
	cfg := detailConfigForType("sfn")
	m := newDetailModel(res, "sfn", cfg)

	view := m.View()
	if view == "" {
		t.Error("SFN detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_SFN_FrameTitle(t *testing.T) {
	sm := realisticSFN()
	res := buildResource("order-processing", "order-processing", sm)
	cfg := detailConfigForType("sfn")
	m := newDetailModel(res, "sfn", cfg)

	if title := m.FrameTitle(); title != "order-processing" {
		t.Errorf("SFN FrameTitle expected %q, got %q", "order-processing", title)
	}
}

// ===========================================================================
// 8. CodePipeline (pipeline)
// ===========================================================================

func TestQA_Detail_Pipeline_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	pl := realisticPipeline()
	res := buildResource("deploy-pipeline", "deploy-pipeline", pl)
	cfg := detailConfigForType("pipeline")
	m := newDetailModel(res, "pipeline", cfg)

	view := m.View()
	for _, expected := range []string{
		"Name", "deploy-pipeline",
		"PipelineType", "V2",
		"Version", "3",
		"ExecutionMode", "QUEUED",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Pipeline detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Pipeline_NilFields(t *testing.T) {
	ensureNoColor(t)
	pl := codepipelinetypes.PipelineSummary{}
	res := buildResource("empty-pipeline", "empty-pipeline", pl)
	cfg := detailConfigForType("pipeline")
	m := newDetailModel(res, "pipeline", cfg)

	view := m.View()
	if view == "" {
		t.Error("Pipeline detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_Pipeline_FrameTitle(t *testing.T) {
	pl := realisticPipeline()
	res := buildResource("deploy-pipeline", "deploy-pipeline", pl)
	cfg := detailConfigForType("pipeline")
	m := newDetailModel(res, "pipeline", cfg)

	if title := m.FrameTitle(); title != "deploy-pipeline" {
		t.Errorf("Pipeline FrameTitle expected %q, got %q", "deploy-pipeline", title)
	}
}

// ===========================================================================
// 9. Kinesis
// ===========================================================================

func TestQA_Detail_Kinesis_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	stream := realisticKinesis()
	res := buildResource("events-stream", "events-stream", stream)
	cfg := detailConfigForType("kinesis")
	m := newDetailModel(res, "kinesis", cfg)

	view := m.View()
	for _, expected := range []string{
		"StreamName", "events-stream",
		"StreamStatus", "ACTIVE",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Kinesis detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Kinesis_NilFields(t *testing.T) {
	ensureNoColor(t)
	stream := kinesistypes.StreamSummary{}
	res := buildResource("empty-kinesis", "empty-kinesis", stream)
	cfg := detailConfigForType("kinesis")
	m := newDetailModel(res, "kinesis", cfg)

	view := m.View()
	if view == "" {
		t.Error("Kinesis detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_Kinesis_FrameTitle(t *testing.T) {
	stream := realisticKinesis()
	res := buildResource("events-stream", "events-stream", stream)
	cfg := detailConfigForType("kinesis")
	m := newDetailModel(res, "kinesis", cfg)

	if title := m.FrameTitle(); title != "events-stream" {
		t.Errorf("Kinesis FrameTitle expected %q, got %q", "events-stream", title)
	}
}

// ===========================================================================
// 10. WAF
// ===========================================================================

func TestQA_Detail_WAF_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	acl := realisticWAF()
	res := buildResource("a1b2c3d4-5678-90ab-cdef-EXAMPLE11111", "prod-waf-acl", acl)
	cfg := detailConfigForType("waf")
	m := newDetailModel(res, "waf", cfg)

	view := m.View()
	for _, expected := range []string{
		"Name", "prod-waf-acl",
		"Id", "a1b2c3d4-5678-90ab-cdef-EXAMPLE11111",
		"Description", "Production WAF rules",
		"LockToken",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("WAF detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_WAF_NilFields(t *testing.T) {
	ensureNoColor(t)
	acl := wafv2types.WebACLSummary{}
	res := buildResource("empty-waf", "empty-waf", acl)
	cfg := detailConfigForType("waf")
	m := newDetailModel(res, "waf", cfg)

	view := m.View()
	if view == "" {
		t.Error("WAF detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_WAF_FrameTitle(t *testing.T) {
	acl := realisticWAF()
	res := buildResource("a1b2c3d4-5678-90ab-cdef-EXAMPLE11111", "prod-waf-acl", acl)
	cfg := detailConfigForType("waf")
	m := newDetailModel(res, "waf", cfg)

	if title := m.FrameTitle(); title != "prod-waf-acl" {
		t.Errorf("WAF FrameTitle expected %q, got %q", "prod-waf-acl", title)
	}
}

// ===========================================================================
// 11. Glue
// ===========================================================================

func TestQA_Detail_Glue_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	job := realisticGlueJob()
	res := buildResource("etl-daily-job", "etl-daily-job", job)
	cfg := detailConfigForType("glue")
	m := newDetailModel(res, "glue", cfg)

	view := m.View()
	for _, expected := range []string{
		"Name", "etl-daily-job",
		"GlueVersion", "4.0",
		"WorkerType", "G.2X",
		"NumberOfWorkers", "10",
		"MaxRetries", "3",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Glue detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Glue_NilFields(t *testing.T) {
	ensureNoColor(t)
	job := gluetypes.Job{}
	res := buildResource("empty-glue", "empty-glue", job)
	cfg := detailConfigForType("glue")
	m := newDetailModel(res, "glue", cfg)

	view := m.View()
	if view == "" {
		t.Error("Glue detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_Glue_FrameTitle(t *testing.T) {
	job := realisticGlueJob()
	res := buildResource("etl-daily-job", "etl-daily-job", job)
	cfg := detailConfigForType("glue")
	m := newDetailModel(res, "glue", cfg)

	if title := m.FrameTitle(); title != "etl-daily-job" {
		t.Errorf("Glue FrameTitle expected %q, got %q", "etl-daily-job", title)
	}
}

// ===========================================================================
// 12. Elastic Beanstalk (eb)
// ===========================================================================

func TestQA_Detail_EB_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	env := realisticEB()
	res := buildResource("e-abc1234def", "prod-api-env", env)
	cfg := detailConfigForType("eb")
	m := newDetailModel(res, "eb", cfg)

	view := m.View()
	for _, expected := range []string{
		"EnvironmentName", "prod-api-env",
		"ApplicationName", "my-web-app",
		"Status", "Ready",
		"Health", "Green",
		"VersionLabel", "v1.2.3",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("EB detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_EB_NilFields(t *testing.T) {
	ensureNoColor(t)
	env := ebtypes.EnvironmentDescription{}
	res := buildResource("empty-eb", "empty-eb", env)
	cfg := detailConfigForType("eb")
	m := newDetailModel(res, "eb", cfg)

	view := m.View()
	if view == "" {
		t.Error("EB detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_EB_FrameTitle(t *testing.T) {
	env := realisticEB()
	res := buildResource("e-abc1234def", "prod-api-env", env)
	cfg := detailConfigForType("eb")
	m := newDetailModel(res, "eb", cfg)

	if title := m.FrameTitle(); title != "prod-api-env" {
		t.Errorf("EB FrameTitle expected %q, got %q", "prod-api-env", title)
	}
}

// ===========================================================================
// 13. SES (uses buildResourceWithFields, no RawStruct in config)
// ===========================================================================

func TestQA_Detail_SES_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	res := buildResourceWithFields("user@example.com", "user@example.com", map[string]string{
		"identity_name":       "user@example.com",
		"identity_type":       "EMAIL_ADDRESS",
		"sending_enabled":     "true",
		"verification_status": "SUCCESS",
	})
	cfg := detailConfigForType("ses")
	m := newDetailModel(res, "ses", cfg)

	view := m.View()
	for _, expected := range []string{
		"IdentityName", "user@example.com",
		"IdentityType", "EMAIL_ADDRESS",
		"SendingEnabled", "true",
		"VerificationStatus", "SUCCESS",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("SES detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_SES_NilFields(t *testing.T) {
	ensureNoColor(t)
	res := buildResourceWithFields("empty-ses", "empty-ses", map[string]string{})
	cfg := detailConfigForType("ses")
	m := newDetailModel(res, "ses", cfg)

	view := m.View()
	if view == "" {
		t.Error("SES detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_SES_FrameTitle(t *testing.T) {
	res := buildResourceWithFields("user@example.com", "user@example.com", map[string]string{
		"identity_name": "user@example.com",
	})
	cfg := detailConfigForType("ses")
	m := newDetailModel(res, "ses", cfg)

	if title := m.FrameTitle(); title != "user@example.com" {
		t.Errorf("SES FrameTitle expected %q, got %q", "user@example.com", title)
	}
}

// ===========================================================================
// 14. Redshift
// ===========================================================================

func TestQA_Detail_Redshift_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticRedshift()
	res := buildResource("analytics-cluster", "analytics-cluster", cluster)
	cfg := detailConfigForType("redshift")
	m := newDetailModel(res, "redshift", cfg)

	view := m.View()
	for _, expected := range []string{
		"ClusterIdentifier", "analytics-cluster",
		"ClusterStatus", "available",
		"NodeType", "dc2.large",
		"NumberOfNodes", "4",
		"DBName", "analytics_db",
		"MasterUsername", "admin",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Redshift detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Redshift_NilFields(t *testing.T) {
	ensureNoColor(t)
	cluster := redshifttypes.Cluster{}
	res := buildResource("empty-redshift", "empty-redshift", cluster)
	cfg := detailConfigForType("redshift")
	m := newDetailModel(res, "redshift", cfg)

	view := m.View()
	if view == "" {
		t.Error("Redshift detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_Redshift_FrameTitle(t *testing.T) {
	cluster := realisticRedshift()
	res := buildResource("analytics-cluster", "analytics-cluster", cluster)
	cfg := detailConfigForType("redshift")
	m := newDetailModel(res, "redshift", cfg)

	if title := m.FrameTitle(); title != "analytics-cluster" {
		t.Errorf("Redshift FrameTitle expected %q, got %q", "analytics-cluster", title)
	}
}

// ===========================================================================
// 15. CloudTrail (trail)
// ===========================================================================

func TestQA_Detail_Trail_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	trail := realisticTrail()
	res := buildResource("org-trail", "org-trail", trail)
	cfg := detailConfigForType("trail")
	m := newDetailModel(res, "trail", cfg)

	view := m.View()
	for _, expected := range []string{
		"Name", "org-trail",
		"S3BucketName", "cloudtrail-logs-bucket",
		"HomeRegion", "us-east-1",
		"IsMultiRegionTrail", "Yes",
		"IsOrganizationTrail", "Yes",
		"LogFileValidationEn", "Yes",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Trail detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Trail_NilFields(t *testing.T) {
	ensureNoColor(t)
	trail := cloudtrailtypes.Trail{}
	res := buildResource("empty-trail", "empty-trail", trail)
	cfg := detailConfigForType("trail")
	m := newDetailModel(res, "trail", cfg)

	view := m.View()
	if view == "" {
		t.Error("Trail detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_Trail_FrameTitle(t *testing.T) {
	trail := realisticTrail()
	res := buildResource("org-trail", "org-trail", trail)
	cfg := detailConfigForType("trail")
	m := newDetailModel(res, "trail", cfg)

	if title := m.FrameTitle(); title != "org-trail" {
		t.Errorf("Trail FrameTitle expected %q, got %q", "org-trail", title)
	}
}

// ===========================================================================
// 16. Athena
// ===========================================================================

func TestQA_Detail_Athena_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	wg := realisticAthena()
	res := buildResource("analytics-wg", "analytics-wg", wg)
	cfg := detailConfigForType("athena")
	m := newDetailModel(res, "athena", cfg)

	view := m.View()
	for _, expected := range []string{
		"Name", "analytics-wg",
		"State", "ENABLED",
		"Description", "Analytics workgroup",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Athena detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Athena_NilFields(t *testing.T) {
	ensureNoColor(t)
	wg := athenatypes.WorkGroupSummary{}
	res := buildResource("empty-athena", "empty-athena", wg)
	cfg := detailConfigForType("athena")
	m := newDetailModel(res, "athena", cfg)

	view := m.View()
	if view == "" {
		t.Error("Athena detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_Athena_FrameTitle(t *testing.T) {
	wg := realisticAthena()
	res := buildResource("analytics-wg", "analytics-wg", wg)
	cfg := detailConfigForType("athena")
	m := newDetailModel(res, "athena", cfg)

	if title := m.FrameTitle(); title != "analytics-wg" {
		t.Errorf("Athena FrameTitle expected %q, got %q", "analytics-wg", title)
	}
}

// ===========================================================================
// 17. CodeArtifact
// ===========================================================================

func TestQA_Detail_CodeArtifact_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	repo := realisticCodeArtifact()
	res := buildResource("shared-libs", "shared-libs", repo)
	cfg := detailConfigForType("codeartifact")
	m := newDetailModel(res, "codeartifact", cfg)

	view := m.View()
	for _, expected := range []string{
		"Name", "shared-libs",
		"DomainName", "my-domain",
		"DomainOwner", "123456789012",
		"Description", "Shared libraries repository",
		"AdministratorAccount", "123456789012",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("CodeArtifact detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_CodeArtifact_NilFields(t *testing.T) {
	ensureNoColor(t)
	repo := codeartifacttypes.RepositorySummary{}
	res := buildResource("empty-ca", "empty-ca", repo)
	cfg := detailConfigForType("codeartifact")
	m := newDetailModel(res, "codeartifact", cfg)

	view := m.View()
	if view == "" {
		t.Error("CodeArtifact detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_CodeArtifact_FrameTitle(t *testing.T) {
	repo := realisticCodeArtifact()
	res := buildResource("shared-libs", "shared-libs", repo)
	cfg := detailConfigForType("codeartifact")
	m := newDetailModel(res, "codeartifact", cfg)

	if title := m.FrameTitle(); title != "shared-libs" {
		t.Errorf("CodeArtifact FrameTitle expected %q, got %q", "shared-libs", title)
	}
}

// ===========================================================================
// 18. CodeBuild (cb)
// ===========================================================================

func TestQA_Detail_CB_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	project := realisticCodeBuild()
	res := buildResource("build-project", "build-project", project)
	cfg := detailConfigForType("cb")
	m := newDetailModel(res, "cb", cfg)

	view := m.View()
	for _, expected := range []string{
		"Name", "build-project",
		"Description", "CI build project",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("CB detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_CB_NilFields(t *testing.T) {
	ensureNoColor(t)
	project := codebuildtypes.Project{}
	res := buildResource("empty-cb", "empty-cb", project)
	cfg := detailConfigForType("cb")
	m := newDetailModel(res, "cb", cfg)

	view := m.View()
	if view == "" {
		t.Error("CB detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_CB_FrameTitle(t *testing.T) {
	project := realisticCodeBuild()
	res := buildResource("build-project", "build-project", project)
	cfg := detailConfigForType("cb")
	m := newDetailModel(res, "cb", cfg)

	if title := m.FrameTitle(); title != "build-project" {
		t.Errorf("CB FrameTitle expected %q, got %q", "build-project", title)
	}
}

// ===========================================================================
// 19. OpenSearch
// ===========================================================================

func TestQA_Detail_OpenSearch_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	domain := realisticOpenSearch()
	res := buildResource("search-prod", "search-prod", domain)
	cfg := detailConfigForType("opensearch")
	m := newDetailModel(res, "opensearch", cfg)

	view := m.View()
	for _, expected := range []string{
		"DomainName", "search-prod",
		"EngineVersion", "OpenSearch_2.11",
		"Endpoint", "search-prod-abc123.us-east-1.es.amazonaws.com",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("OpenSearch detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_OpenSearch_NilFields(t *testing.T) {
	ensureNoColor(t)
	domain := opensearchtypes.DomainStatus{}
	res := buildResource("empty-os", "empty-os", domain)
	cfg := detailConfigForType("opensearch")
	m := newDetailModel(res, "opensearch", cfg)

	view := m.View()
	if view == "" {
		t.Error("OpenSearch detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_OpenSearch_FrameTitle(t *testing.T) {
	domain := realisticOpenSearch()
	res := buildResource("search-prod", "search-prod", domain)
	cfg := detailConfigForType("opensearch")
	m := newDetailModel(res, "opensearch", cfg)

	if title := m.FrameTitle(); title != "search-prod" {
		t.Errorf("OpenSearch FrameTitle expected %q, got %q", "search-prod", title)
	}
}

// ===========================================================================
// 20. KMS
// ===========================================================================

func TestQA_Detail_KMS_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	meta := realisticKMS()
	res := buildResource("12345678-1234-1234-1234-123456789012", "alias/prod-key", meta)
	cfg := detailConfigForType("kms")
	m := newDetailModel(res, "kms", cfg)

	view := m.View()
	for _, expected := range []string{
		"KeyId", "12345678-1234-1234-1234-123456789012",
		"Description", "Production encryption key",
		"KeyState", "Enabled",
		"KeyUsage", "ENCRYPT_DECRYPT",
		"KeyManager", "CUSTOMER",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("KMS detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_KMS_NilFields(t *testing.T) {
	ensureNoColor(t)
	meta := &kmstypes.KeyMetadata{}
	res := buildResource("empty-kms", "empty-kms", meta)
	cfg := detailConfigForType("kms")
	m := newDetailModel(res, "kms", cfg)

	view := m.View()
	if view == "" {
		t.Error("KMS detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_KMS_FrameTitle(t *testing.T) {
	meta := realisticKMS()
	res := buildResource("12345678-1234-1234-1234-123456789012", "alias/prod-key", meta)
	cfg := detailConfigForType("kms")
	m := newDetailModel(res, "kms", cfg)

	if title := m.FrameTitle(); title != "alias/prod-key" {
		t.Errorf("KMS FrameTitle expected %q, got %q", "alias/prod-key", title)
	}
}

// ===========================================================================
// 21. MSK
// ===========================================================================

func TestQA_Detail_MSK_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticMSK()
	res := buildResource("events-kafka", "events-kafka", cluster)
	cfg := detailConfigForType("msk")
	m := newDetailModel(res, "msk", cfg)

	view := m.View()
	for _, expected := range []string{
		"ClusterName", "events-kafka",
		"ClusterType", "PROVISIONED",
		"State", "ACTIVE",
		"CurrentVersion", "K3AEGXETSR30VB",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("MSK detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_MSK_NilFields(t *testing.T) {
	ensureNoColor(t)
	cluster := kafkatypes.Cluster{}
	res := buildResource("empty-msk", "empty-msk", cluster)
	cfg := detailConfigForType("msk")
	m := newDetailModel(res, "msk", cfg)

	view := m.View()
	if view == "" {
		t.Error("MSK detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_MSK_FrameTitle(t *testing.T) {
	cluster := realisticMSK()
	res := buildResource("events-kafka", "events-kafka", cluster)
	cfg := detailConfigForType("msk")
	m := newDetailModel(res, "msk", cfg)

	if title := m.FrameTitle(); title != "events-kafka" {
		t.Errorf("MSK FrameTitle expected %q, got %q", "events-kafka", title)
	}
}

// ===========================================================================
// 22. Backup
// ===========================================================================

func TestQA_Detail_Backup_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	plan := realisticBackup()
	res := buildResource("abc12345-1234-1234-1234-123456789012", "daily-backup-plan", plan)
	cfg := detailConfigForType("backup")
	m := newDetailModel(res, "backup", cfg)

	view := m.View()
	for _, expected := range []string{
		"BackupPlanName", "daily-backup-plan",
		"BackupPlanId", "abc12345-1234-1234-1234-123456789012",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Backup detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Backup_NilFields(t *testing.T) {
	ensureNoColor(t)
	plan := backuptypes.BackupPlansListMember{}
	res := buildResource("empty-backup", "empty-backup", plan)
	cfg := detailConfigForType("backup")
	m := newDetailModel(res, "backup", cfg)

	view := m.View()
	if view == "" {
		t.Error("Backup detail should not be empty even with nil fields")
	}
}

func TestQA_Detail_Backup_FrameTitle(t *testing.T) {
	plan := realisticBackup()
	res := buildResource("abc12345-1234-1234-1234-123456789012", "daily-backup-plan", plan)
	cfg := detailConfigForType("backup")
	m := newDetailModel(res, "backup", cfg)

	if title := m.FrameTitle(); title != "daily-backup-plan" {
		t.Errorf("Backup FrameTitle expected %q, got %q", "daily-backup-plan", title)
	}
}

// ===========================================================================
// Suppress "imported and not used" for time package (used by testTime)
// ===========================================================================
var _ = time.Now
