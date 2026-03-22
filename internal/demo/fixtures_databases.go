package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["redis"] = redisFixtures
	demoData["dbc"] = docdbClusterFixtures
	demoData["ddb"] = dynamodbFixtures
	demoData["opensearch"] = opensearchFixtures
	demoData["redshift"] = redshiftFixtures
	demoData["efs"] = efsFixtures
	demoData["rds-snap"] = rdsSnapshotFixtures
	demoData["docdb-snap"] = docdbSnapshotFixtures
}

// redisFixtures returns demo ElastiCache Redis cluster fixtures.
// Field keys: cluster_id, engine_version, node_type, status, nodes, endpoint
func redisFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-prod-sessions",
			Name:   "acme-prod-sessions",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":     "acme-prod-sessions",
				"engine_version": "7.1",
				"node_type":      "cache.r6g.large",
				"status":         "available",
				"nodes":          "3",
				"endpoint":       "acme-prod-sessions.cfg.usw2.cache.amazonaws.com",
			},
			RawStruct: elasticachetypes.CacheCluster{
				ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:acme-prod-sessions"),
				CacheClusterId:     aws.String("acme-prod-sessions"),
				CacheClusterStatus: aws.String("available"),
				CacheNodeType:      aws.String("cache.r6g.large"),
				Engine:             aws.String("redis"),
				EngineVersion:      aws.String("7.1"),
				NumCacheNodes:      aws.Int32(3),
				ConfigurationEndpoint: &elasticachetypes.Endpoint{
					Address: aws.String("acme-prod-sessions.cfg.usw2.cache.amazonaws.com"),
					Port:    aws.Int32(6379),
				},
				CacheClusterCreateTime: aws.Time(mustParseTime("2025-06-10T14:30:00+00:00")),
			},
		},
		{
			ID:     "acme-prod-cache",
			Name:   "acme-prod-cache",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":     "acme-prod-cache",
				"engine_version": "7.1",
				"node_type":      "cache.m6g.xlarge",
				"status":         "available",
				"nodes":          "2",
				"endpoint":       "acme-prod-cache.cfg.usw2.cache.amazonaws.com",
			},
			RawStruct: elasticachetypes.CacheCluster{
				ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:acme-prod-cache"),
				CacheClusterId:     aws.String("acme-prod-cache"),
				CacheClusterStatus: aws.String("available"),
				CacheNodeType:      aws.String("cache.m6g.xlarge"),
				Engine:             aws.String("redis"),
				EngineVersion:      aws.String("7.1"),
				NumCacheNodes:      aws.Int32(2),
				ConfigurationEndpoint: &elasticachetypes.Endpoint{
					Address: aws.String("acme-prod-cache.cfg.usw2.cache.amazonaws.com"),
					Port:    aws.Int32(6379),
				},
				CacheClusterCreateTime: aws.Time(mustParseTime("2025-03-22T09:15:00+00:00")),
			},
		},
		{
			ID:     "staging-redis",
			Name:   "staging-redis",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":     "staging-redis",
				"engine_version": "7.0",
				"node_type":      "cache.t3.medium",
				"status":         "available",
				"nodes":          "1",
				"endpoint":       "staging-redis.cfg.usw2.cache.amazonaws.com",
			},
			RawStruct: elasticachetypes.CacheCluster{
				ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:staging-redis"),
				CacheClusterId:     aws.String("staging-redis"),
				CacheClusterStatus: aws.String("available"),
				CacheNodeType:      aws.String("cache.t3.medium"),
				Engine:             aws.String("redis"),
				EngineVersion:      aws.String("7.0"),
				NumCacheNodes:      aws.Int32(1),
				ConfigurationEndpoint: &elasticachetypes.Endpoint{
					Address: aws.String("staging-redis.cfg.usw2.cache.amazonaws.com"),
					Port:    aws.Int32(6379),
				},
				CacheClusterCreateTime: aws.Time(mustParseTime("2025-09-01T11:00:00+00:00")),
			},
		},
		{
			ID:     "dev-feature-redis",
			Name:   "dev-feature-redis",
			Status: "creating",
			Fields: map[string]string{
				"cluster_id":     "dev-feature-redis",
				"engine_version": "7.1",
				"node_type":      "cache.t3.small",
				"status":         "creating",
				"nodes":          "1",
				"endpoint":       "",
			},
			RawStruct: elasticachetypes.CacheCluster{
				ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:dev-feature-redis"),
				CacheClusterId:     aws.String("dev-feature-redis"),
				CacheClusterStatus: aws.String("creating"),
				CacheNodeType:      aws.String("cache.t3.small"),
				Engine:             aws.String("redis"),
				EngineVersion:      aws.String("7.1"),
				NumCacheNodes:      aws.Int32(1),
				CacheClusterCreateTime: aws.Time(mustParseTime("2026-03-21T08:00:00+00:00")),
			},
		},
	}
}

// docdbClusterFixtures returns demo DocumentDB cluster fixtures.
// Field keys: cluster_id, engine_version, status, instances, endpoint
func docdbClusterFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-docdb-prod",
			Name:   "acme-docdb-prod",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":     "acme-docdb-prod",
				"engine_version": "5.0.0",
				"status":         "available",
				"instances":      "3",
				"endpoint":       "acme-docdb-prod.cluster-c9xyz123.us-east-1.docdb.amazonaws.com",
			},
			RawStruct: docdbtypes.DBCluster{
				DBClusterIdentifier: aws.String("acme-docdb-prod"),
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:acme-docdb-prod"),
				Engine:              aws.String("docdb"),
				EngineVersion:       aws.String("5.0.0"),
				Status:              aws.String("available"),
				Endpoint:            aws.String("acme-docdb-prod.cluster-c9xyz123.us-east-1.docdb.amazonaws.com"),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("acme-docdb-prod-01"), IsClusterWriter: aws.Bool(true)},
					{DBInstanceIdentifier: aws.String("acme-docdb-prod-02"), IsClusterWriter: aws.Bool(false)},
					{DBInstanceIdentifier: aws.String("acme-docdb-prod-03"), IsClusterWriter: aws.Bool(false)},
				},
				MasterUsername:  aws.String("docdbadmin"),
				MultiAZ:         aws.Bool(true),
				ClusterCreateTime: aws.Time(mustParseTime("2025-04-15T10:20:00+00:00")),
			},
		},
		{
			ID:     "analytics-docdb",
			Name:   "analytics-docdb",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":     "analytics-docdb",
				"engine_version": "5.0.0",
				"status":         "available",
				"instances":      "2",
				"endpoint":       "analytics-docdb.cluster-c9xyz123.us-east-1.docdb.amazonaws.com",
			},
			RawStruct: docdbtypes.DBCluster{
				DBClusterIdentifier: aws.String("analytics-docdb"),
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:analytics-docdb"),
				Engine:              aws.String("docdb"),
				EngineVersion:       aws.String("5.0.0"),
				Status:              aws.String("available"),
				Endpoint:            aws.String("analytics-docdb.cluster-c9xyz123.us-east-1.docdb.amazonaws.com"),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("analytics-docdb-01"), IsClusterWriter: aws.Bool(true)},
					{DBInstanceIdentifier: aws.String("analytics-docdb-02"), IsClusterWriter: aws.Bool(false)},
				},
				MasterUsername:  aws.String("analytics"),
				MultiAZ:         aws.Bool(false),
				ClusterCreateTime: aws.Time(mustParseTime("2025-08-20T16:45:00+00:00")),
			},
		},
		{
			ID:     "staging-docdb",
			Name:   "staging-docdb",
			Status: "modifying",
			Fields: map[string]string{
				"cluster_id":     "staging-docdb",
				"engine_version": "4.0.0",
				"status":         "modifying",
				"instances":      "1",
				"endpoint":       "staging-docdb.cluster-c9xyz123.us-east-1.docdb.amazonaws.com",
			},
			RawStruct: docdbtypes.DBCluster{
				DBClusterIdentifier: aws.String("staging-docdb"),
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:staging-docdb"),
				Engine:              aws.String("docdb"),
				EngineVersion:       aws.String("4.0.0"),
				Status:              aws.String("modifying"),
				Endpoint:            aws.String("staging-docdb.cluster-c9xyz123.us-east-1.docdb.amazonaws.com"),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("staging-docdb-01"), IsClusterWriter: aws.Bool(true)},
				},
				MasterUsername:  aws.String("stagingadmin"),
				MultiAZ:         aws.Bool(false),
				ClusterCreateTime: aws.Time(mustParseTime("2025-11-05T08:30:00+00:00")),
			},
		},
	}
}

// dynamodbFixtures returns demo DynamoDB table fixtures.
// Field keys: table_name, status, item_count, size_bytes, billing_mode
// Note: the production fetcher stores *ddbtypes.TableDescription (pointer) as RawStruct.
func dynamodbFixtures() []resource.Resource {
	return []resource.Resource{
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
				ItemCount:      aws.Int64(2458103),
				TableSizeBytes: aws.Int64(1073741824),
				CreationDateTime: aws.Time(mustParseTime("2025-02-10T09:00:00+00:00")),
				BillingModeSummary: &ddbtypes.BillingModeSummary{
					BillingMode: ddbtypes.BillingModePayPerRequest,
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
				ARN:        aws.String("arn:aws:es:us-east-1:123456789012:domain/acme-logs"),
				DomainId:   aws.String("123456789012/acme-logs"),
				DomainName: aws.String("acme-logs"),
				EngineVersion: aws.String("OpenSearch_2.11"),
				Endpoint:      aws.String("search-acme-logs-abc123.us-east-1.es.amazonaws.com"),
				Created:       aws.Bool(true),
				Deleted:       aws.Bool(false),
				ClusterConfig: &ostypes.ClusterConfig{
					InstanceType:  ostypes.OpenSearchPartitionInstanceTypeR6gLargeSearch,
					InstanceCount: aws.Int32(3),
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
				ARN:        aws.String("arn:aws:es:us-east-1:123456789012:domain/acme-product-search"),
				DomainId:   aws.String("123456789012/acme-product-search"),
				DomainName: aws.String("acme-product-search"),
				EngineVersion: aws.String("OpenSearch_2.11"),
				Endpoint:      aws.String("search-acme-product-search-def456.us-east-1.es.amazonaws.com"),
				Created:       aws.Bool(true),
				Deleted:       aws.Bool(false),
				ClusterConfig: &ostypes.ClusterConfig{
					InstanceType:  ostypes.OpenSearchPartitionInstanceTypeR6gXlargeSearch,
					InstanceCount: aws.Int32(2),
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
				ARN:        aws.String("arn:aws:es:us-east-1:123456789012:domain/staging-analytics"),
				DomainId:   aws.String("123456789012/staging-analytics"),
				DomainName: aws.String("staging-analytics"),
				EngineVersion: aws.String("OpenSearch_2.9"),
				Endpoint:      aws.String("search-staging-analytics-ghi789.us-east-1.es.amazonaws.com"),
				Created:       aws.Bool(true),
				Deleted:       aws.Bool(false),
				ClusterConfig: &ostypes.ClusterConfig{
					InstanceType:  ostypes.OpenSearchPartitionInstanceTypeM6gLargeSearch,
					InstanceCount: aws.Int32(1),
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
				ClusterIdentifier: aws.String("acme-warehouse"),
				ClusterStatus:     aws.String("available"),
				NodeType:          aws.String("ra3.xlplus"),
				NumberOfNodes:     aws.Int32(4),
				DBName:            aws.String("analytics"),
				MasterUsername:    aws.String("admin"),
				ClusterCreateTime: aws.Time(mustParseTime("2025-03-10T09:00:00+00:00")),
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
				ClusterIdentifier: aws.String("acme-reporting"),
				ClusterStatus:     aws.String("available"),
				NodeType:          aws.String("ra3.xlplus"),
				NumberOfNodes:     aws.Int32(2),
				DBName:            aws.String("reporting"),
				MasterUsername:    aws.String("admin"),
				ClusterCreateTime: aws.Time(mustParseTime("2025-07-22T14:30:00+00:00")),
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
				ClusterIdentifier: aws.String("staging-dwh"),
				ClusterStatus:     aws.String("paused"),
				NodeType:          aws.String("dc2.large"),
				NumberOfNodes:     aws.Int32(2),
				DBName:            aws.String("staging"),
				MasterUsername:    aws.String("stgadmin"),
				ClusterCreateTime: aws.Time(mustParseTime("2025-10-15T08:00:00+00:00")),
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

// rdsSnapshotFixtures returns demo RDS DB snapshot fixtures.
// Field keys: snapshot_id, db_instance, status, engine, snapshot_type, created
func rdsSnapshotFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "rds:prod-api-primary-2026-03-20",
			Name:   "rds:prod-api-primary-2026-03-20",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":   "rds:prod-api-primary-2026-03-20",
				"db_instance":   "prod-api-primary",
				"status":        "available",
				"engine":        "aurora-postgresql",
				"snapshot_type": "automated",
				"created":       "2026-03-20T03:00:00Z",
			},
			RawStruct: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("rds:prod-api-primary-2026-03-20"),
				DBInstanceIdentifier: aws.String("prod-api-primary"),
				Status:               aws.String("available"),
				Engine:               aws.String("aurora-postgresql"),
				EngineVersion:        aws.String("16.4"),
				SnapshotType:         aws.String("automated"),
				SnapshotCreateTime:   aws.Time(mustParseTime("2026-03-20T03:00:00+00:00")),
				AllocatedStorage:     aws.Int32(100),
				DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:rds:prod-api-primary-2026-03-20"),
			},
		},
		{
			ID:     "rds:analytics-warehouse-2026-03-20",
			Name:   "rds:analytics-warehouse-2026-03-20",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":   "rds:analytics-warehouse-2026-03-20",
				"db_instance":   "analytics-warehouse",
				"status":        "available",
				"engine":        "postgres",
				"snapshot_type": "automated",
				"created":       "2026-03-20T03:30:00Z",
			},
			RawStruct: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("rds:analytics-warehouse-2026-03-20"),
				DBInstanceIdentifier: aws.String("analytics-warehouse"),
				Status:               aws.String("available"),
				Engine:               aws.String("postgres"),
				EngineVersion:        aws.String("16.2"),
				SnapshotType:         aws.String("automated"),
				SnapshotCreateTime:   aws.Time(mustParseTime("2026-03-20T03:30:00+00:00")),
				AllocatedStorage:     aws.Int32(500),
				DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:rds:analytics-warehouse-2026-03-20"),
			},
		},
		{
			ID:     "pre-migration-snapshot",
			Name:   "pre-migration-snapshot",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":   "pre-migration-snapshot",
				"db_instance":   "staging-mysql",
				"status":        "available",
				"engine":        "mysql",
				"snapshot_type": "manual",
				"created":       "2026-03-15T22:00:00Z",
			},
			RawStruct: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("pre-migration-snapshot"),
				DBInstanceIdentifier: aws.String("staging-mysql"),
				Status:               aws.String("available"),
				Engine:               aws.String("mysql"),
				EngineVersion:        aws.String("8.0.36"),
				SnapshotType:         aws.String("manual"),
				SnapshotCreateTime:   aws.Time(mustParseTime("2026-03-15T22:00:00+00:00")),
				AllocatedStorage:     aws.Int32(50),
				DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:pre-migration-snapshot"),
			},
		},
		{
			ID:     "dev-feature-branch-snap",
			Name:   "dev-feature-branch-snap",
			Status: "creating",
			Fields: map[string]string{
				"snapshot_id":   "dev-feature-branch-snap",
				"db_instance":   "dev-feature-branch",
				"status":        "creating",
				"engine":        "aurora-postgresql",
				"snapshot_type": "manual",
				"created":       "2026-03-21T10:30:00Z",
			},
			RawStruct: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("dev-feature-branch-snap"),
				DBInstanceIdentifier: aws.String("dev-feature-branch"),
				Status:               aws.String("creating"),
				Engine:               aws.String("aurora-postgresql"),
				EngineVersion:        aws.String("16.4"),
				SnapshotType:         aws.String("manual"),
				SnapshotCreateTime:   aws.Time(mustParseTime("2026-03-21T10:30:00+00:00")),
				AllocatedStorage:     aws.Int32(20),
				DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:dev-feature-branch-snap"),
			},
		},
	}
}

// docdbSnapshotFixtures returns demo DocumentDB cluster snapshot fixtures.
// Field keys: snapshot_id, cluster_id, status, engine, snapshot_type, snapshot_create_time, storage_type
func docdbSnapshotFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "rds:acme-docdb-prod-2026-03-20",
			Name:   "rds:acme-docdb-prod-2026-03-20",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":          "rds:acme-docdb-prod-2026-03-20",
				"cluster_id":           "acme-docdb-prod",
				"status":               "available",
				"engine":               "docdb",
				"snapshot_type":        "automated",
				"snapshot_create_time": "2026-03-20T04:00:00Z",
				"storage_type":         "standard",
			},
			RawStruct: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("rds:acme-docdb-prod-2026-03-20"),
				DBClusterIdentifier:         aws.String("acme-docdb-prod"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:acme-docdb-prod-2026-03-20"),
				Status:                      aws.String("available"),
				Engine:                      aws.String("docdb"),
				EngineVersion:               aws.String("5.0.0"),
				SnapshotType:                aws.String("automated"),
				SnapshotCreateTime:          aws.Time(mustParseTime("2026-03-20T04:00:00+00:00")),
				StorageType:                 aws.String("standard"),
				StorageEncrypted:            aws.Bool(true),
				VpcId:                       aws.String("vpc-0abc123def456789a"),
			},
		},
		{
			ID:     "pre-upgrade-docdb-snap",
			Name:   "pre-upgrade-docdb-snap",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":          "pre-upgrade-docdb-snap",
				"cluster_id":           "staging-docdb",
				"status":               "available",
				"engine":               "docdb",
				"snapshot_type":        "manual",
				"snapshot_create_time": "2026-03-18T20:00:00Z",
				"storage_type":         "standard",
			},
			RawStruct: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("pre-upgrade-docdb-snap"),
				DBClusterIdentifier:         aws.String("staging-docdb"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:pre-upgrade-docdb-snap"),
				Status:                      aws.String("available"),
				Engine:                      aws.String("docdb"),
				EngineVersion:               aws.String("4.0.0"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          aws.Time(mustParseTime("2026-03-18T20:00:00+00:00")),
				StorageType:                 aws.String("standard"),
				StorageEncrypted:            aws.Bool(true),
				VpcId:                       aws.String("vpc-0abc123def456789a"),
			},
		},
		{
			ID:     "rds:analytics-docdb-2026-03-20",
			Name:   "rds:analytics-docdb-2026-03-20",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":          "rds:analytics-docdb-2026-03-20",
				"cluster_id":           "analytics-docdb",
				"status":               "available",
				"engine":               "docdb",
				"snapshot_type":        "automated",
				"snapshot_create_time": "2026-03-20T04:30:00Z",
				"storage_type":         "iopt1",
			},
			RawStruct: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("rds:analytics-docdb-2026-03-20"),
				DBClusterIdentifier:         aws.String("analytics-docdb"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:analytics-docdb-2026-03-20"),
				Status:                      aws.String("available"),
				Engine:                      aws.String("docdb"),
				EngineVersion:               aws.String("5.0.0"),
				SnapshotType:                aws.String("automated"),
				SnapshotCreateTime:          aws.Time(mustParseTime("2026-03-20T04:30:00+00:00")),
				StorageType:                 aws.String("iopt1"),
				StorageEncrypted:            aws.Bool(true),
				VpcId:                       aws.String("vpc-0abc123def456789a"),
			},
		},
	}
}
