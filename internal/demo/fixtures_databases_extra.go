package demo

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["ddb"] = dynamodbFixtures
	demoData["opensearch"] = opensearchFixtures
	demoData["redshift"] = redshiftFixtures
	demoData["efs"] = efsFixtures
}

// dynamodbFixtures returns demo DynamoDB table fixtures.
// Field keys: table_name, status, item_count, size_bytes, billing_mode
// Note: the production fetcher stores *ddbtypes.TableDescription (pointer) as RawStruct.
func dynamodbFixtures() []resource.Resource {
	tables := []resource.Resource{
		{
			ID:     "acme-orders",
			Name:   "acme-orders",
			Status: "ACTIVE",
			Fields: map[string]string{
				"table_name":   "acme-orders",
				"status":       "ACTIVE",
				"item_count":   "2458103",
				"size_bytes":   "1073741824",
				"billing_mode": "PAY_PER_REQUEST",
			},
			RawStruct: &ddbtypes.TableDescription{
				TableName:      aws.String("acme-orders"),
				TableStatus:    ddbtypes.TableStatusActive,
				TableArn:       aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders"),
				TableId:        aws.String("a1b2c3d4-0000-1111-2222-333333333333"),
				ItemCount:      aws.Int64(2458103),
				TableSizeBytes: aws.Int64(1073741824),
				CreationDateTime: aws.Time(mustParseTime("2025-02-10T09:00:00+00:00")),
				BillingModeSummary: &ddbtypes.BillingModeSummary{
					BillingMode: ddbtypes.BillingModePayPerRequest,
				},
				DeletionProtectionEnabled: aws.Bool(true),
				AttributeDefinitions: []ddbtypes.AttributeDefinition{
					{AttributeName: aws.String("OrderId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
					{AttributeName: aws.String("CustomerId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
				},
				KeySchema: []ddbtypes.KeySchemaElement{
					{AttributeName: aws.String("OrderId"), KeyType: ddbtypes.KeyTypeHash},
					{AttributeName: aws.String("CustomerId"), KeyType: ddbtypes.KeyTypeRange},
				},
				GlobalSecondaryIndexes: []ddbtypes.GlobalSecondaryIndexDescription{
					{
						IndexName:      aws.String("CustomerId-index"),
						IndexArn:       aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders/index/CustomerId-index"),
						IndexStatus:    ddbtypes.IndexStatusActive,
						IndexSizeBytes: aws.Int64(256000000),
						ItemCount:      aws.Int64(2458103),
					},
				},
				LocalSecondaryIndexes: []ddbtypes.LocalSecondaryIndexDescription{
					{
						IndexName:      aws.String("OrderDate-index"),
						IndexArn:       aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders/index/OrderDate-index"),
						IndexSizeBytes: aws.Int64(128000000),
						ItemCount:      aws.Int64(2458103),
					},
				},
				ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
					ReadCapacityUnits:  aws.Int64(25),
					WriteCapacityUnits: aws.Int64(25),
				},
				SSEDescription: &ddbtypes.SSEDescription{
					Status:        ddbtypes.SSEStatusEnabled,
					SSEType:       ddbtypes.SSETypeKms,
					KMSMasterKeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				},
				StreamSpecification: &ddbtypes.StreamSpecification{
					StreamEnabled:  aws.Bool(true),
					StreamViewType: ddbtypes.StreamViewTypeNewAndOldImages,
				},
			},
		},
		{
			ID:     "acme-sessions",
			Name:   "acme-sessions",
			Status: "ACTIVE",
			Fields: map[string]string{
				"table_name":   "acme-sessions",
				"status":       "ACTIVE",
				"item_count":   "89421",
				"size_bytes":   "52428800",
				"billing_mode": "PAY_PER_REQUEST",
			},
			RawStruct: &ddbtypes.TableDescription{
				TableName:      aws.String("acme-sessions"),
				TableStatus:    ddbtypes.TableStatusActive,
				TableArn:       aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-sessions"),
				ItemCount:      aws.Int64(89421),
				TableSizeBytes: aws.Int64(52428800),
				CreationDateTime: aws.Time(mustParseTime("2025-05-18T14:30:00+00:00")),
				BillingModeSummary: &ddbtypes.BillingModeSummary{
					BillingMode: ddbtypes.BillingModePayPerRequest,
				},
			},
		},
		{
			ID:     "acme-inventory",
			Name:   "acme-inventory",
			Status: "ACTIVE",
			Fields: map[string]string{
				"table_name":   "acme-inventory",
				"status":       "ACTIVE",
				"item_count":   "345678",
				"size_bytes":   "209715200",
				"billing_mode": "PROVISIONED",
			},
			RawStruct: &ddbtypes.TableDescription{
				TableName:      aws.String("acme-inventory"),
				TableStatus:    ddbtypes.TableStatusActive,
				TableArn:       aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-inventory"),
				ItemCount:      aws.Int64(345678),
				TableSizeBytes: aws.Int64(209715200),
				CreationDateTime: aws.Time(mustParseTime("2025-01-08T11:15:00+00:00")),
				BillingModeSummary: &ddbtypes.BillingModeSummary{
					BillingMode: ddbtypes.BillingModeProvisioned,
				},
			},
		},
		{
			ID:     "acme-audit-log",
			Name:   "acme-audit-log",
			Status: "ACTIVE",
			Fields: map[string]string{
				"table_name":   "acme-audit-log",
				"status":       "ACTIVE",
				"item_count":   "15234567",
				"size_bytes":   "5368709120",
				"billing_mode": "PAY_PER_REQUEST",
			},
			RawStruct: &ddbtypes.TableDescription{
				TableName:      aws.String("acme-audit-log"),
				TableStatus:    ddbtypes.TableStatusActive,
				TableArn:       aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-audit-log"),
				ItemCount:      aws.Int64(15234567),
				TableSizeBytes: aws.Int64(5368709120),
				CreationDateTime: aws.Time(mustParseTime("2024-11-20T07:00:00+00:00")),
				BillingModeSummary: &ddbtypes.BillingModeSummary{
					BillingMode: ddbtypes.BillingModePayPerRequest,
				},
			},
		},
	}

	// Generate 18 more tables to reach 22 total
	itemCounts := []int64{12345, 567890, 234567, 89012, 1234567, 45678, 890123, 23456, 678901, 12345678, 34567, 901234, 56789, 2345678, 78901, 123456, 9012345, 456789}
	sizesBytes := []int64{10485760, 104857600, 52428800, 20971520, 536870912, 10485760, 209715200, 5242880, 104857600, 1073741824, 10485760, 209715200, 10485760, 536870912, 20971520, 52428800, 1073741824, 104857600}
	for i := 0; i < 18; i++ {
		name := ddbNamePool[i]
		billing := "PAY_PER_REQUEST"
		billingMode := ddbtypes.BillingModePayPerRequest
		if i%4 == 0 {
			billing = "PROVISIONED"
			billingMode = ddbtypes.BillingModeProvisioned
		}
		createDate := fmt.Sprintf("2025-%02d-%02dT%02d:00:00+00:00", 1+(i%12), 1+i, 8+(i%10))
		tables = append(tables, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "ACTIVE",
			Fields: map[string]string{
				"table_name":   name,
				"status":       "ACTIVE",
				"item_count":   fmt.Sprintf("%d", itemCounts[i]),
				"size_bytes":   fmt.Sprintf("%d", sizesBytes[i]),
				"billing_mode": billing,
			},
			RawStruct: &ddbtypes.TableDescription{
				TableName:        aws.String(name),
				TableStatus:      ddbtypes.TableStatusActive,
				TableArn:         aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/" + name),
				ItemCount:        aws.Int64(itemCounts[i]),
				TableSizeBytes:   aws.Int64(sizesBytes[i]),
				CreationDateTime: aws.Time(mustParseTime(createDate)),
				BillingModeSummary: &ddbtypes.BillingModeSummary{
					BillingMode: billingMode,
				},
			},
		})
	}

	return tables
}

// opensearchFixtures returns demo OpenSearch domain fixtures.
// Field keys: domain_name, engine_version, instance_type, instance_count, endpoint
func opensearchFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-logs",
			Name:   "acme-logs",
			Status: "",
			Fields: map[string]string{
				"domain_name":    "acme-logs",
				"engine_version": "OpenSearch_2.11",
				"instance_type":  "r6g.large.search",
				"instance_count": "3",
				"endpoint":       "search-acme-logs-abc123.us-east-1.es.amazonaws.com",
			},
			RawStruct: ostypes.DomainStatus{
				ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/acme-logs"),
				DomainId:      aws.String("123456789012/acme-logs"),
				DomainName:    aws.String("acme-logs"),
				EngineVersion: aws.String("OpenSearch_2.11"),
				Endpoint:      aws.String("search-acme-logs-abc123.us-east-1.es.amazonaws.com"),
				Created:       aws.Bool(true),
				Deleted:       aws.Bool(false),
				ClusterConfig: &ostypes.ClusterConfig{
					InstanceType:  ostypes.OpenSearchPartitionInstanceTypeR6gLargeSearch,
					InstanceCount: aws.Int32(3),
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
				Endpoints: map[string]string{
					"vpc": "vpc-search-acme-logs-abc123.us-east-1.es.amazonaws.com",
				},
				AdvancedSecurityOptions: &ostypes.AdvancedSecurityOptions{
					Enabled:                     aws.Bool(true),
					InternalUserDatabaseEnabled: aws.Bool(false),
				},
			},
		},
		{
			ID:     "acme-product-search",
			Name:   "acme-product-search",
			Status: "",
			Fields: map[string]string{
				"domain_name":    "acme-product-search",
				"engine_version": "OpenSearch_2.11",
				"instance_type":  "r6g.xlarge.search",
				"instance_count": "2",
				"endpoint":       "search-acme-product-search-def456.us-east-1.es.amazonaws.com",
			},
			RawStruct: ostypes.DomainStatus{
				ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/acme-product-search"),
				DomainId:      aws.String("123456789012/acme-product-search"),
				DomainName:    aws.String("acme-product-search"),
				EngineVersion: aws.String("OpenSearch_2.11"),
				Endpoint:      aws.String("search-acme-product-search-def456.us-east-1.es.amazonaws.com"),
				Created:       aws.Bool(true),
				Deleted:       aws.Bool(false),
				ClusterConfig: &ostypes.ClusterConfig{
					InstanceType:  ostypes.OpenSearchPartitionInstanceTypeR6gXlargeSearch,
					InstanceCount: aws.Int32(2),
				},
				EBSOptions: &ostypes.EBSOptions{
					EBSEnabled: aws.Bool(true),
					VolumeType: ostypes.VolumeTypeGp3,
					VolumeSize: aws.Int32(200),
				},
				EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
					Enabled: aws.Bool(true),
				},
				DomainEndpointOptions: &ostypes.DomainEndpointOptions{
					EnforceHTTPS: aws.Bool(true),
				},
			},
		},
		{
			ID:     "staging-analytics",
			Name:   "staging-analytics",
			Status: "",
			Fields: map[string]string{
				"domain_name":    "staging-analytics",
				"engine_version": "OpenSearch_2.9",
				"instance_type":  "m6g.large.search",
				"instance_count": "1",
				"endpoint":       "search-staging-analytics-ghi789.us-east-1.es.amazonaws.com",
			},
			RawStruct: ostypes.DomainStatus{
				ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/staging-analytics"),
				DomainId:      aws.String("123456789012/staging-analytics"),
				DomainName:    aws.String("staging-analytics"),
				EngineVersion: aws.String("OpenSearch_2.9"),
				Endpoint:      aws.String("search-staging-analytics-ghi789.us-east-1.es.amazonaws.com"),
				Created:       aws.Bool(true),
				Deleted:       aws.Bool(false),
				ClusterConfig: &ostypes.ClusterConfig{
					InstanceType:  ostypes.OpenSearchPartitionInstanceTypeM6gLargeSearch,
					InstanceCount: aws.Int32(1),
				},
				EBSOptions: &ostypes.EBSOptions{
					EBSEnabled: aws.Bool(true),
					VolumeType: ostypes.VolumeTypeGp3,
					VolumeSize: aws.Int32(50),
				},
				EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
					Enabled: aws.Bool(false),
				},
				DomainEndpointOptions: &ostypes.DomainEndpointOptions{
					EnforceHTTPS: aws.Bool(true),
				},
			},
		},
	}
}

// redshiftFixtures returns demo Redshift cluster fixtures.
// Field keys: cluster_id, status, node_type, num_nodes, db_name, endpoint
func redshiftFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-warehouse",
			Name:   "acme-warehouse",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":  "acme-warehouse",
				"status":      "available",
				"node_type":   "ra3.xlplus",
				"num_nodes":   "4",
				"db_name":     "analytics",
				"endpoint":    "acme-warehouse.c9xyz123.us-east-1.redshift.amazonaws.com",
				"master_user": "admin",
				"create_time": "2025-03-10 09:00:00",
			},
			RawStruct: redshifttypes.Cluster{
				ClusterIdentifier:   aws.String("acme-warehouse"),
				ClusterStatus:       aws.String("available"),
				NodeType:            aws.String("ra3.xlplus"),
				NumberOfNodes:       aws.Int32(4),
				DBName:              aws.String("analytics"),
				MasterUsername:      aws.String("admin"),
				ClusterCreateTime:   aws.Time(mustParseTime("2025-03-10T09:00:00+00:00")),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:acme-warehouse"),
				AvailabilityZone:    aws.String("us-east-1a"),
				VpcId:               aws.String(prodVPCID),
				Endpoint: &redshifttypes.Endpoint{
					Address: aws.String("acme-warehouse.c9xyz123.us-east-1.redshift.amazonaws.com"),
					Port:    aws.Int32(5439),
				},
			},
		},
		{
			ID:     "acme-reporting",
			Name:   "acme-reporting",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":  "acme-reporting",
				"status":      "available",
				"node_type":   "ra3.xlplus",
				"num_nodes":   "2",
				"db_name":     "reporting",
				"endpoint":    "acme-reporting.c9xyz123.us-east-1.redshift.amazonaws.com",
				"master_user": "admin",
				"create_time": "2025-07-22 14:30:00",
			},
			RawStruct: redshifttypes.Cluster{
				ClusterIdentifier:   aws.String("acme-reporting"),
				ClusterStatus:       aws.String("available"),
				NodeType:            aws.String("ra3.xlplus"),
				NumberOfNodes:       aws.Int32(2),
				DBName:              aws.String("reporting"),
				MasterUsername:      aws.String("admin"),
				ClusterCreateTime:   aws.Time(mustParseTime("2025-07-22T14:30:00+00:00")),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:acme-reporting"),
				AvailabilityZone:    aws.String("us-east-1b"),
				VpcId:               aws.String(prodVPCID),
				Endpoint: &redshifttypes.Endpoint{
					Address: aws.String("acme-reporting.c9xyz123.us-east-1.redshift.amazonaws.com"),
					Port:    aws.Int32(5439),
				},
			},
		},
		{
			ID:     "staging-dwh",
			Name:   "staging-dwh",
			Status: "paused",
			Fields: map[string]string{
				"cluster_id":  "staging-dwh",
				"status":      "paused",
				"node_type":   "dc2.large",
				"num_nodes":   "2",
				"db_name":     "staging",
				"endpoint":    "staging-dwh.c9xyz123.us-east-1.redshift.amazonaws.com",
				"master_user": "stgadmin",
				"create_time": "2025-10-15 08:00:00",
			},
			RawStruct: redshifttypes.Cluster{
				ClusterIdentifier:   aws.String("staging-dwh"),
				ClusterStatus:       aws.String("paused"),
				NodeType:            aws.String("dc2.large"),
				NumberOfNodes:       aws.Int32(2),
				DBName:              aws.String("staging"),
				MasterUsername:      aws.String("stgadmin"),
				ClusterCreateTime:   aws.Time(mustParseTime("2025-10-15T08:00:00+00:00")),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:staging-dwh"),
				AvailabilityZone:    aws.String("us-east-1a"),
				VpcId:               aws.String(stagingVPCID),
				Endpoint: &redshifttypes.Endpoint{
					Address: aws.String("staging-dwh.c9xyz123.us-east-1.redshift.amazonaws.com"),
					Port:    aws.Int32(5439),
				},
			},
		},
	}
}

// efsFixtures returns demo EFS file system fixtures.
// Field keys: file_system_id, name, life_cycle_state, performance_mode, encrypted, mount_targets
func efsFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "fs-0abc111111111111a",
			Name:   "acme-shared-data",
			Status: "available",
			Fields: map[string]string{
				"file_system_id":   "fs-0abc111111111111a",
				"name":             "acme-shared-data",
				"life_cycle_state": "available",
				"performance_mode": "generalPurpose",
				"throughput_mode":  "elastic",
				"encrypted":        "true",
				"mount_targets":    "3",
			},
			RawStruct: efstypes.FileSystemDescription{
				FileSystemId:       aws.String("fs-0abc111111111111a"),
				FileSystemArn:      aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0abc111111111111a"),
				Name:               aws.String("acme-shared-data"),
				LifeCycleState:     efstypes.LifeCycleStateAvailable,
				PerformanceMode:    efstypes.PerformanceModeGeneralPurpose,
				ThroughputMode:     efstypes.ThroughputModeElastic,
				Encrypted:          aws.Bool(true),
				NumberOfMountTargets: 3,
				CreationTime:       aws.Time(mustParseTime("2025-04-01T10:00:00+00:00")),
				CreationToken:      aws.String("acme-shared-data"),
				OwnerId:            aws.String("123456789012"),
				SizeInBytes: &efstypes.FileSystemSize{
					Value: 10737418240,
				},
				Tags: []efstypes.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-shared-data")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "fs-0def222222222222b",
			Name:   "ml-training-storage",
			Status: "available",
			Fields: map[string]string{
				"file_system_id":   "fs-0def222222222222b",
				"name":             "ml-training-storage",
				"life_cycle_state": "available",
				"performance_mode": "maxIO",
				"throughput_mode":  "bursting",
				"encrypted":        "true",
				"mount_targets":    "2",
			},
			RawStruct: efstypes.FileSystemDescription{
				FileSystemId:       aws.String("fs-0def222222222222b"),
				FileSystemArn:      aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0def222222222222b"),
				Name:               aws.String("ml-training-storage"),
				LifeCycleState:     efstypes.LifeCycleStateAvailable,
				PerformanceMode:    efstypes.PerformanceModeMaxIo,
				ThroughputMode:     efstypes.ThroughputModeBursting,
				Encrypted:          aws.Bool(true),
				NumberOfMountTargets: 2,
				CreationTime:       aws.Time(mustParseTime("2025-08-15T14:30:00+00:00")),
				CreationToken:      aws.String("ml-training-storage"),
				OwnerId:            aws.String("123456789012"),
				SizeInBytes: &efstypes.FileSystemSize{
					Value: 53687091200,
				},
				Tags: []efstypes.Tag{
					{Key: aws.String("Name"), Value: aws.String("ml-training-storage")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "fs-0ghi333333333333c",
			Name:   "staging-efs",
			Status: "creating",
			Fields: map[string]string{
				"file_system_id":   "fs-0ghi333333333333c",
				"name":             "staging-efs",
				"life_cycle_state": "creating",
				"performance_mode": "generalPurpose",
				"throughput_mode":  "bursting",
				"encrypted":        "false",
				"mount_targets":    "0",
			},
			RawStruct: efstypes.FileSystemDescription{
				FileSystemId:       aws.String("fs-0ghi333333333333c"),
				FileSystemArn:      aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0ghi333333333333c"),
				Name:               aws.String("staging-efs"),
				LifeCycleState:     efstypes.LifeCycleStateCreating,
				PerformanceMode:    efstypes.PerformanceModeGeneralPurpose,
				ThroughputMode:     efstypes.ThroughputModeBursting,
				Encrypted:          aws.Bool(false),
				NumberOfMountTargets: 0,
				CreationTime:       aws.Time(mustParseTime("2026-03-21T09:00:00+00:00")),
				CreationToken:      aws.String("staging-efs"),
				OwnerId:            aws.String("123456789012"),
				SizeInBytes: &efstypes.FileSystemSize{
					Value: 0,
				},
				Tags: []efstypes.Tag{
					{Key: aws.String("Name"), Value: aws.String("staging-efs")},
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
	}
}
