package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
)

// Exported stable identifiers — referenced by sibling fixture files and QA tests.
const (
	// GraphRootDomain is the primary demo showroom domain (acme-logs).
	GraphRootDomain = "acme-logs"

	// GraphRootDomainARN is the full ARN of the graph-root domain.
	GraphRootDomainARN = "arn:aws:es:us-east-1:123456789012:domain/acme-logs"

	// UpdateAvailableDomain is the domain with a pending forced software update.
	UpdateAvailableDomain = "acme-product-search"

	// EncryptionOffDomain is the domain with encryption at rest disabled.
	EncryptionOffDomain = "legacy-analytics"

	// MultiBackgroundDomain has both UpdateAvailable (!) and EncryptionOff (~).
	MultiBackgroundDomain = "acme-metrics"

	// ProcessingDomain is the domain currently applying a config change.
	ProcessingDomain = "acme-events"

	// ProcessingPlusUpdateDomain has UpgradeProcessing + UpdateAvailable stacked.
	ProcessingPlusUpdateDomain = "acme-search-alpha"

	// IsolatedDomain has DomainProcessingStatus=Isolated (quarantined by AWS).
	IsolatedDomain = "legacy-search-isolated"

	// DeletingDomain has Deleted=true (removal in progress).
	DeletingDomain = "obsolete-tenant-logs"

	// HealthyBaselineDomain is a clean, no-signals domain.
	HealthyBaselineDomain = "staging-analytics"

	// OpenSearchKMSKeyID is the bare KMS key ID used by the graph-root domain.
	// The full ARN embeds this after "key/".
	OpenSearchKMSKeyID = "acme-opensearch-key"

	// OpenSearchKMSKeyARN is the full KMS key ARN used by the graph-root domain.
	OpenSearchKMSKeyARN = "arn:aws:kms:us-east-1:123456789012:key/acme-opensearch-key"

	// OpenSearchCFNStackName is the CloudFormation stack that owns the graph-root domain.
	OpenSearchCFNStackName = "acme-search-stack"

	// OpenSearchACMCertARN is the ACM certificate ARN returned by DescribeDomainConfig
	// for the graph-root domain's custom endpoint (acme-logs.internal.com).
	OpenSearchACMCertARN = "arn:aws:acm:us-east-1:123456789012:certificate/os-acme-logs-cert-0001"

	// OpenSearchACMCertID is the bare certificate ID (after stripping "certificate/").
	OpenSearchACMCertID = "os-acme-logs-cert-0001"

	// OpenSearchLogGroupSearchSlow is the CWLogs group for search-slow logs.
	OpenSearchLogGroupSearchSlow = "/aws/opensearch/acme-logs/search-slow"

	// OpenSearchLogGroupIndexSlow is the CWLogs group for index-slow logs.
	OpenSearchLogGroupIndexSlow = "/aws/opensearch/acme-logs/index-slow"

	// OpenSearchLogGroupAudit is the CWLogs group for audit logs.
	OpenSearchLogGroupAudit = "/aws/opensearch/acme-logs/audit"

	// OpenSearchVPCID is the VPC ID the graph-root domain is attached to.
	OpenSearchVPCID = "vpc-demo-a"

	// OpenSearchSubnetA is the first subnet used by the graph-root domain.
	OpenSearchSubnetA = "subnet-demo-a1"

	// OpenSearchSubnetB is the second subnet used by the graph-root domain.
	OpenSearchSubnetB = "subnet-demo-a2"

	// OpenSearchSGA is the first security group attached to the graph-root domain.
	OpenSearchSGA = "sg-demo-a1"

	// OpenSearchSGB is the second security group attached to the graph-root domain.
	OpenSearchSGB = "sg-demo-a2"
)

// OpenSearchFixtures holds typed fixture data for OpenSearch.
type OpenSearchFixtures struct {
	Domains []ostypes.DomainStatus
}

// NewOpenSearchFixtures constructs OpenSearchFixtures from the canonical demo data.
// Fixture order matches the spec §2.1 list exactly:
// 1. healthy_baseline, 2. graph_root, 3. update_available_bang,
// 4. encryption_off_tilde, 5. multi_background, 6. processing_warning,
// 7. processing_plus_update, 8. isolated_broken, 9. deleting_dim.
var sharedOpenSearchFixtures = sync.OnceValue(func() *OpenSearchFixtures {
	return &OpenSearchFixtures{
		Domains: []ostypes.DomainStatus{
			osHealthyBaseline(),
			osGraphRoot(),
			osUpdateAvailableBang(),
			osEncryptionOffTilde(),
			osMultiBackground(),
			osProcessingWarning(),
			osProcessingPlusUpdate(),
			osIsolatedBroken(),
			osDeletingDim(),
		},
	}
})

func NewOpenSearchFixtures() *OpenSearchFixtures {
	return sharedOpenSearchFixtures()
}

// ---------------------------------------------------------------------------
// Baseline helper — fields shared by every non-specialised fixture.
// ---------------------------------------------------------------------------

func osBaseDomain(name, domainID, arn, engineVersion, endpoint string) ostypes.DomainStatus {
	return ostypes.DomainStatus{
		ARN:            aws.String(arn),
		DomainId:       aws.String(domainID),
		DomainName:     aws.String(name),
		EngineVersion:  aws.String(engineVersion),
		Endpoint:       aws.String(endpoint),
		Created:        aws.Bool(true),
		Deleted:        aws.Bool(false),
		Processing:     aws.Bool(false),
		UpgradeProcessing: aws.Bool(false),
		DomainProcessingStatus: ostypes.DomainProcessingStatusTypeActive,
		ClusterConfig: &ostypes.ClusterConfig{
			InstanceType:  ostypes.OpenSearchPartitionInstanceTypeR6gLargeSearch,
			InstanceCount: aws.Int32(2),
		},
		EBSOptions: &ostypes.EBSOptions{
			EBSEnabled: aws.Bool(true),
			VolumeType: ostypes.VolumeTypeGp3,
			VolumeSize: aws.Int32(100),
		},
		EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
			Enabled: aws.Bool(true),
		},
		DomainEndpointOptions: &ostypes.DomainEndpointOptions{
			EnforceHTTPS: aws.Bool(true),
		},
		ServiceSoftwareOptions: &ostypes.ServiceSoftwareOptions{
			UpdateAvailable: aws.Bool(false),
		},
	}
}

// ---------------------------------------------------------------------------
// Fixture 1 — healthy_baseline (staging-analytics)
// ---------------------------------------------------------------------------

func osHealthyBaseline() ostypes.DomainStatus {
	d := osBaseDomain(
		HealthyBaselineDomain,
		"123456789012/staging-analytics",
		"arn:aws:es:us-east-1:123456789012:domain/staging-analytics",
		"OpenSearch_2.9",
		"search-staging-analytics-ghi789.us-east-1.es.amazonaws.com",
	)
	d.ClusterConfig.InstanceType = ostypes.OpenSearchPartitionInstanceTypeM6gLargeSearch
	d.ClusterConfig.InstanceCount = aws.Int32(1)
	d.EBSOptions.VolumeSize = aws.Int32(50)
	// No VPCOptions — public endpoint.
	return d
}

// ---------------------------------------------------------------------------
// Fixture 2 — graph_root (acme-logs) — full §9.3 showroom
// ---------------------------------------------------------------------------

func osGraphRoot() ostypes.DomainStatus {
	d := osBaseDomain(
		GraphRootDomain,
		"123456789012/acme-logs",
		GraphRootDomainARN,
		"OpenSearch_2.11",
		"search-acme-logs-abc123.us-east-1.es.amazonaws.com",
	)
	d.ClusterConfig.InstanceType = ostypes.OpenSearchPartitionInstanceTypeR6gLargeSearch
	d.ClusterConfig.InstanceCount = aws.Int32(3)
	d.EBSOptions.VolumeSize = aws.Int32(100)

	// Encryption at rest with customer KMS key.
	d.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{
		Enabled:  aws.Bool(true),
		KmsKeyId: aws.String(OpenSearchKMSKeyARN),
	}

	// Custom endpoint (ACM cert wired via DescribeDomainConfig).
	d.DomainEndpointOptions = &ostypes.DomainEndpointOptions{
		EnforceHTTPS:         aws.Bool(true),
		CustomEndpointEnabled: aws.Bool(true),
		CustomEndpoint:        aws.String("acme-logs.internal.com"),
	}

	// VPC attachment — drives sg / subnet / vpc pivots.
	d.VPCOptions = &ostypes.VPCDerivedInfo{
		VPCId:            aws.String(OpenSearchVPCID),
		SubnetIds:        []string{OpenSearchSubnetA, OpenSearchSubnetB},
		SecurityGroupIds: []string{OpenSearchSGA, OpenSearchSGB},
	}

	// Log publishing — drives logs pivot (3 groups).
	d.LogPublishingOptions = map[string]ostypes.LogPublishingOption{
		string(ostypes.LogTypeSearchSlowLogs): {
			Enabled:                   aws.Bool(true),
			CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + OpenSearchLogGroupSearchSlow + ":*"),
		},
		string(ostypes.LogTypeIndexSlowLogs): {
			Enabled:                   aws.Bool(true),
			CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + OpenSearchLogGroupIndexSlow + ":*"),
		},
		string(ostypes.LogTypeAuditLogs): {
			Enabled:                   aws.Bool(true),
			CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + OpenSearchLogGroupAudit + ":*"),
		},
	}

	// CFN tag is wired via the fake ListTags (not in DomainStatus itself).
	// Alarm pivot: 2 alarms in cloudwatch.go with Namespace=AWS/ES, DomainName=acme-logs.
	return d
}

// ---------------------------------------------------------------------------
// Fixture 3 — update_available_bang (acme-product-search)
// ---------------------------------------------------------------------------

func osUpdateAvailableBang() ostypes.DomainStatus {
	d := osBaseDomain(
		UpdateAvailableDomain,
		"123456789012/acme-product-search",
		"arn:aws:es:us-east-1:123456789012:domain/acme-product-search",
		"OpenSearch_2.11",
		"search-acme-product-search-def456.us-east-1.es.amazonaws.com",
	)
	d.ClusterConfig.InstanceType = ostypes.OpenSearchPartitionInstanceTypeR6gXlargeSearch
	d.ClusterConfig.InstanceCount = aws.Int32(2)
	d.EBSOptions.VolumeSize = aws.Int32(200)
	// AutomatedUpdateDate in the past (2026-04-20 < 2026-04-24 today).
	d.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{
		UpdateAvailable:     aws.Bool(true),
		AutomatedUpdateDate: aws.Time(time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)),
		CurrentVersion:      aws.String("OpenSearch_2.11"),
		NewVersion:          aws.String("OpenSearch_2.13"),
	}
	return d
}

// ---------------------------------------------------------------------------
// Fixture 4 — encryption_off_tilde (legacy-analytics)
// ---------------------------------------------------------------------------

func osEncryptionOffTilde() ostypes.DomainStatus {
	d := osBaseDomain(
		EncryptionOffDomain,
		"123456789012/legacy-analytics",
		"arn:aws:es:us-east-1:123456789012:domain/legacy-analytics",
		"Elasticsearch_7.10",
		"search-legacy-analytics-xyz111.us-east-1.es.amazonaws.com",
	)
	d.ClusterConfig.InstanceType = ostypes.OpenSearchPartitionInstanceTypeM5LargeSearch
	d.ClusterConfig.InstanceCount = aws.Int32(1)
	d.EBSOptions.VolumeType = ostypes.VolumeTypeGp2
	d.EBSOptions.VolumeSize = aws.Int32(50)
	d.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{
		Enabled: aws.Bool(false),
	}
	return d
}

// ---------------------------------------------------------------------------
// Fixture 5 — multi_background (acme-metrics)
// UpdateAvailable (past auto-update date 2026-04-10) AND EncryptionOff.
// ---------------------------------------------------------------------------

func osMultiBackground() ostypes.DomainStatus {
	d := osBaseDomain(
		MultiBackgroundDomain,
		"123456789012/acme-metrics",
		"arn:aws:es:us-east-1:123456789012:domain/acme-metrics",
		"OpenSearch_2.11",
		"search-acme-metrics-pqr678.us-east-1.es.amazonaws.com",
	)
	d.ClusterConfig.InstanceType = ostypes.OpenSearchPartitionInstanceTypeR6gXlargeSearch
	d.ClusterConfig.InstanceCount = aws.Int32(3)
	d.EBSOptions.VolumeSize = aws.Int32(300)
	d.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{
		UpdateAvailable:     aws.Bool(true),
		AutomatedUpdateDate: aws.Time(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
		CurrentVersion:      aws.String("OpenSearch_2.11"),
		NewVersion:          aws.String("OpenSearch_2.13"),
	}
	d.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{
		Enabled: aws.Bool(false),
	}
	return d
}

// ---------------------------------------------------------------------------
// Fixture 6 — processing_warning (acme-events)
// ---------------------------------------------------------------------------

func osProcessingWarning() ostypes.DomainStatus {
	d := osBaseDomain(
		ProcessingDomain,
		"123456789012/acme-events",
		"arn:aws:es:us-east-1:123456789012:domain/acme-events",
		"OpenSearch_2.13",
		"search-acme-events-mno345.us-east-1.es.amazonaws.com",
	)
	d.ClusterConfig.InstanceCount = aws.Int32(2)
	d.EBSOptions.VolumeSize = aws.Int32(150)
	d.Processing = aws.Bool(true)
	d.DomainProcessingStatus = ostypes.DomainProcessingStatusTypeModifying
	return d
}

// ---------------------------------------------------------------------------
// Fixture 7 — processing_plus_update (acme-search-alpha)
// UpgradeProcessing=true AND UpdateAvailable (past date).
// ---------------------------------------------------------------------------

func osProcessingPlusUpdate() ostypes.DomainStatus {
	d := osBaseDomain(
		ProcessingPlusUpdateDomain,
		"123456789012/acme-search-alpha",
		"arn:aws:es:us-east-1:123456789012:domain/acme-search-alpha",
		"OpenSearch_2.11",
		"search-acme-search-alpha-stu901.us-east-1.es.amazonaws.com",
	)
	d.ClusterConfig.InstanceCount = aws.Int32(2)
	d.EBSOptions.VolumeSize = aws.Int32(200)
	d.UpgradeProcessing = aws.Bool(true)
	d.DomainProcessingStatus = ostypes.DomainProcessingStatusTypeUpgrading
	d.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{
		UpdateAvailable:     aws.Bool(true),
		AutomatedUpdateDate: aws.Time(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
		CurrentVersion:      aws.String("OpenSearch_2.11"),
		NewVersion:          aws.String("OpenSearch_2.13"),
	}
	return d
}

// ---------------------------------------------------------------------------
// Fixture 8 — isolated_broken (legacy-search-isolated)
// ---------------------------------------------------------------------------

func osIsolatedBroken() ostypes.DomainStatus {
	d := osBaseDomain(
		IsolatedDomain,
		"123456789012/legacy-search-isolated",
		"arn:aws:es:us-east-1:123456789012:domain/legacy-search-isolated",
		"Elasticsearch_7.10",
		"search-legacy-search-isolated-jkl012.us-east-1.es.amazonaws.com",
	)
	d.ClusterConfig.InstanceType = ostypes.OpenSearchPartitionInstanceTypeM5LargeSearch
	d.ClusterConfig.InstanceCount = aws.Int32(1)
	d.EBSOptions.VolumeType = ostypes.VolumeTypeGp2
	d.EBSOptions.VolumeSize = aws.Int32(20)
	d.DomainProcessingStatus = ostypes.DomainProcessingStatusTypeIsolated
	return d
}

// ---------------------------------------------------------------------------
// Fixture 9 — deleting_dim (obsolete-tenant-logs)
// ---------------------------------------------------------------------------

func osDeletingDim() ostypes.DomainStatus {
	d := osBaseDomain(
		DeletingDomain,
		"123456789012/obsolete-tenant-logs",
		"arn:aws:es:us-east-1:123456789012:domain/obsolete-tenant-logs",
		"OpenSearch_2.9",
		"search-obsolete-tenant-logs-vwx234.us-east-1.es.amazonaws.com",
	)
	d.ClusterConfig.InstanceType = ostypes.OpenSearchPartitionInstanceTypeM6gLargeSearch
	d.ClusterConfig.InstanceCount = aws.Int32(1)
	d.EBSOptions.VolumeSize = aws.Int32(50)
	d.Deleted = aws.Bool(true)
	return d
}
