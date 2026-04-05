package demo

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)


func init() {
	demoData["s3"] = s3Buckets
	demoData["dbi"] = rdsInstances
	demoData["redis"] = redisFixtures
	demoData["dbc"] = docdbClusterFixtures

	RegisterChildDemo("dbi_events", func(parentCtx map[string]string) []resource.Resource {
		return dbiEventFixtures(parentCtx["db_identifier"])
	})
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
				ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:acme-prod-sessions"),
				CacheClusterId:            aws.String("acme-prod-sessions"),
				CacheClusterStatus:        aws.String("available"),
				CacheNodeType:             aws.String("cache.r6g.large"),
				Engine:                    aws.String("redis"),
				EngineVersion:             aws.String("7.1"),
				NumCacheNodes:             aws.Int32(3),
				ReplicationGroupId:        aws.String("acme-prod-sessions-rg"),
				PreferredAvailabilityZone: aws.String("us-east-1a"),
				CacheSubnetGroupName:      aws.String("acme-elasticache-subnet-group"),
				AtRestEncryptionEnabled:   aws.Bool(true),
				TransitEncryptionEnabled:  aws.Bool(true),
				AuthTokenEnabled:          aws.Bool(true),
				SnapshotRetentionLimit:    aws.Int32(1),
				PreferredMaintenanceWindow: aws.String("sun:05:00-sun:06:00"),
				ConfigurationEndpoint: &elasticachetypes.Endpoint{
					Address: aws.String("acme-prod-sessions.cfg.usw2.cache.amazonaws.com"),
					Port:    aws.Int32(6379),
				},
				SecurityGroups: []elasticachetypes.SecurityGroupMembership{
					{SecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				CacheNodes: []elasticachetypes.CacheNode{
					{CacheNodeId: aws.String("0001"), CacheNodeStatus: aws.String("available")},
					{CacheNodeId: aws.String("0002"), CacheNodeStatus: aws.String("available")},
					{CacheNodeId: aws.String("0003"), CacheNodeStatus: aws.String("available")},
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
				ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:acme-prod-cache"),
				CacheClusterId:            aws.String("acme-prod-cache"),
				CacheClusterStatus:        aws.String("available"),
				CacheNodeType:             aws.String("cache.m6g.xlarge"),
				Engine:                    aws.String("redis"),
				EngineVersion:             aws.String("7.1"),
				NumCacheNodes:             aws.Int32(2),
				ReplicationGroupId:        aws.String("acme-prod-cache-rg"),
				PreferredAvailabilityZone: aws.String("us-east-1b"),
				CacheSubnetGroupName:      aws.String("acme-elasticache-subnet-group"),
				AtRestEncryptionEnabled:   aws.Bool(true),
				TransitEncryptionEnabled:  aws.Bool(true),
				AuthTokenEnabled:          aws.Bool(false),
				SnapshotRetentionLimit:    aws.Int32(1),
				PreferredMaintenanceWindow: aws.String("mon:05:00-mon:06:00"),
				ConfigurationEndpoint: &elasticachetypes.Endpoint{
					Address: aws.String("acme-prod-cache.cfg.usw2.cache.amazonaws.com"),
					Port:    aws.Int32(6379),
				},
				SecurityGroups: []elasticachetypes.SecurityGroupMembership{
					{SecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				CacheNodes: []elasticachetypes.CacheNode{
					{CacheNodeId: aws.String("0001"), CacheNodeStatus: aws.String("available")},
					{CacheNodeId: aws.String("0002"), CacheNodeStatus: aws.String("available")},
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
				ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:staging-redis"),
				CacheClusterId:            aws.String("staging-redis"),
				CacheClusterStatus:        aws.String("available"),
				CacheNodeType:             aws.String("cache.t3.medium"),
				Engine:                    aws.String("redis"),
				EngineVersion:             aws.String("7.0"),
				NumCacheNodes:             aws.Int32(1),
				ReplicationGroupId:        aws.String("staging-redis-rg"),
				PreferredAvailabilityZone: aws.String("us-east-1a"),
				CacheSubnetGroupName:      aws.String("acme-staging-elasticache-subnet-group"),
				AtRestEncryptionEnabled:   aws.Bool(false),
				TransitEncryptionEnabled:  aws.Bool(false),
				AuthTokenEnabled:          aws.Bool(false),
				SnapshotRetentionLimit:    aws.Int32(0),
				PreferredMaintenanceWindow: aws.String("tue:06:00-tue:07:00"),
				ConfigurationEndpoint: &elasticachetypes.Endpoint{
					Address: aws.String("staging-redis.cfg.usw2.cache.amazonaws.com"),
					Port:    aws.Int32(6379),
				},
				SecurityGroups: []elasticachetypes.SecurityGroupMembership{
					{SecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				CacheNodes: []elasticachetypes.CacheNode{
					{CacheNodeId: aws.String("0001"), CacheNodeStatus: aws.String("available")},
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
				ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:dev-feature-redis"),
				CacheClusterId:            aws.String("dev-feature-redis"),
				CacheClusterStatus:        aws.String("creating"),
				CacheNodeType:             aws.String("cache.t3.small"),
				Engine:                    aws.String("redis"),
				EngineVersion:             aws.String("7.1"),
				NumCacheNodes:             aws.Int32(1),
				ReplicationGroupId:        aws.String("dev-feature-redis-rg"),
				PreferredAvailabilityZone: aws.String("us-east-1a"),
				CacheSubnetGroupName:      aws.String("acme-elasticache-subnet-group"),
				AtRestEncryptionEnabled:   aws.Bool(true),
				TransitEncryptionEnabled:  aws.Bool(true),
				AuthTokenEnabled:          aws.Bool(false),
				SnapshotRetentionLimit:    aws.Int32(0),
				PreferredMaintenanceWindow: aws.String("wed:06:00-wed:07:00"),
				SecurityGroups: []elasticachetypes.SecurityGroupMembership{
					{SecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				CacheNodes:             []elasticachetypes.CacheNode{},
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
				DBClusterIdentifier:        aws.String("acme-docdb-prod"),
				DBClusterArn:               aws.String("arn:aws:rds:us-east-1:123456789012:cluster:acme-docdb-prod"),
				Engine:                     aws.String("docdb"),
				EngineVersion:              aws.String("5.0.0"),
				Status:                     aws.String("available"),
				Endpoint:                   aws.String("acme-docdb-prod.cluster-c9xyz123.us-east-1.docdb.amazonaws.com"),
				ReaderEndpoint:             aws.String("acme-docdb-prod.cluster-ro-xyz.us-east-1.docdb.amazonaws.com"),
				Port:                       aws.Int32(27017),
				StorageEncrypted:           aws.Bool(true),
				KmsKeyId:                   aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				DeletionProtection:         aws.Bool(true),
				BackupRetentionPeriod:      aws.Int32(7),
				PreferredMaintenanceWindow: aws.String("sun:04:00-sun:04:30"),
				DBSubnetGroup:              aws.String("acme-docdb-subnet-group"),
				VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
					{VpcSecurityGroupId: aws.String("sg-0ccc333333333333c"), Status: aws.String("active")},
				},
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("acme-docdb-prod-01"), IsClusterWriter: aws.Bool(true)},
					{DBInstanceIdentifier: aws.String("acme-docdb-prod-02"), IsClusterWriter: aws.Bool(false)},
					{DBInstanceIdentifier: aws.String("acme-docdb-prod-03"), IsClusterWriter: aws.Bool(false)},
				},
				MasterUsername:    aws.String("docdbadmin"),
				MultiAZ:           aws.Bool(true),
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
				MasterUsername:    aws.String("analytics"),
				MultiAZ:           aws.Bool(false),
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
				DBClusterIdentifier:        aws.String("staging-docdb"),
				DBClusterArn:               aws.String("arn:aws:rds:us-east-1:123456789012:cluster:staging-docdb"),
				Engine:                     aws.String("docdb"),
				EngineVersion:              aws.String("4.0.0"),
				Status:                     aws.String("modifying"),
				Endpoint:                   aws.String("staging-docdb.cluster-c9xyz123.us-east-1.docdb.amazonaws.com"),
				ReaderEndpoint:             aws.String("staging-docdb.cluster-ro-c9xyz123.us-east-1.docdb.amazonaws.com"),
				Port:                       aws.Int32(27017),
				StorageEncrypted:           aws.Bool(true),
				KmsKeyId:                   aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				DeletionProtection:         aws.Bool(false),
				BackupRetentionPeriod:      aws.Int32(1),
				PreferredMaintenanceWindow: aws.String("tue:07:00-tue:08:00"),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("staging-docdb-01"), IsClusterWriter: aws.Bool(true)},
				},
				MasterUsername: aws.String("stagingadmin"),
				MultiAZ:        aws.Bool(false),
				VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
					{VpcSecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				DBSubnetGroup:     aws.String("acme-docdb-subnet-group"),
				ClusterCreateTime: aws.Time(mustParseTime("2025-11-05T08:30:00+00:00")),
			},
		},
	}
}

// s3Buckets returns demo S3 bucket fixtures.
func s3Buckets() []resource.Resource {
	buckets := []resource.Resource{
		{
			ID:     "data-pipeline-logs",
			Name:   "data-pipeline-logs",
			Status: "",
			Fields: map[string]string{
				"name":                "data-pipeline-logs",
				"bucket_name":         "data-pipeline-logs",
				"creation_date":       "2025-01-15T09:23:41+00:00",
				"notification_lambda": "arn:aws:lambda:us-east-1:123456789012:function:process-orders",
				"notification_sqs":    "arn:aws:sqs:us-east-1:123456789012:order-processing-queue",
				"notification_sns":    "arn:aws:sns:us-east-1:123456789012:order-events",
			},
			RawStruct: s3types.Bucket{
				Name:         aws.String("data-pipeline-logs"),
				BucketArn:    aws.String("arn:aws:s3:::data-pipeline-logs"),
				BucketRegion: aws.String("us-east-1"),
				CreationDate: aws.Time(mustParseTime("2025-01-15T09:23:41+00:00")),
			},
		},
		{
			ID:     "webapp-assets-prod",
			Name:   "webapp-assets-prod",
			Status: "",
			Fields: map[string]string{
				"name":                "webapp-assets-prod",
				"bucket_name":         "webapp-assets-prod",
				"creation_date":       "2025-03-22T14:07:19+00:00",
				"notification_lambda": "arn:aws:lambda:us-east-1:123456789012:function:image-thumbnail-gen",
				"notification_sqs":    "arn:aws:sqs:us-east-1:123456789012:webhook-ingest-queue.fifo",
				"notification_sns":    "arn:aws:sns:us-east-1:123456789012:deploy-notifications",
			},
			RawStruct: s3types.Bucket{
				Name:         aws.String("webapp-assets-prod"),
				BucketArn:    aws.String("arn:aws:s3:::webapp-assets-prod"),
				BucketRegion: aws.String("us-east-1"),
				CreationDate: aws.Time(mustParseTime("2025-03-22T14:07:19+00:00")),
			},
		},
		{
			ID:     "ml-training-data",
			Name:   "ml-training-data",
			Status: "",
			Fields: map[string]string{
				"name":          "ml-training-data",
				"bucket_name":   "ml-training-data",
				"creation_date": "2025-06-10T08:45:33+00:00",
			},
			RawStruct: s3types.Bucket{
				Name:         aws.String("ml-training-data"),
				BucketArn:    aws.String("arn:aws:s3:::ml-training-data"),
				BucketRegion: aws.String("us-east-1"),
				CreationDate: aws.Time(mustParseTime("2025-06-10T08:45:33+00:00")),
			},
		},
		{
			ID:     "terraform-state-prod",
			Name:   "terraform-state-prod",
			Status: "",
			Fields: map[string]string{
				"name":          "terraform-state-prod",
				"bucket_name":   "terraform-state-prod",
				"creation_date": "2024-11-02T16:30:12+00:00",
			},
			RawStruct: s3types.Bucket{
				Name:         aws.String("terraform-state-prod"),
				BucketArn:    aws.String("arn:aws:s3:::terraform-state-prod"),
				BucketRegion: aws.String("us-east-1"),
				CreationDate: aws.Time(mustParseTime("2024-11-02T16:30:12+00:00")),
			},
		},
		{
			ID:     "cloudtrail-audit-logs",
			Name:   "cloudtrail-audit-logs",
			Status: "",
			Fields: map[string]string{
				"name":          "cloudtrail-audit-logs",
				"bucket_name":   "cloudtrail-audit-logs",
				"creation_date": "2024-08-19T11:12:05+00:00",
			},
			RawStruct: s3types.Bucket{
				Name:         aws.String("cloudtrail-audit-logs"),
				BucketArn:    aws.String("arn:aws:s3:::cloudtrail-audit-logs"),
				BucketRegion: aws.String("us-east-1"),
				CreationDate: aws.Time(mustParseTime("2024-08-19T11:12:05+00:00")),
			},
		},
		{
			ID:     "backup-db-snapshots",
			Name:   "backup-db-snapshots",
			Status: "",
			Fields: map[string]string{
				"name":          "backup-db-snapshots",
				"bucket_name":   "backup-db-snapshots",
				"creation_date": "2025-09-01T07:55:28+00:00",
			},
			RawStruct: s3types.Bucket{
				Name:         aws.String("backup-db-snapshots"),
				BucketArn:    aws.String("arn:aws:s3:::backup-db-snapshots"),
				BucketRegion: aws.String("us-east-1"),
				CreationDate: aws.Time(mustParseTime("2025-09-01T07:55:28+00:00")),
			},
		},
	}

	// Generate 16 more buckets to reach 22 total
	for i := 0; i < 16; i++ {
		name := s3NamePool[i]
		createDate := fmt.Sprintf("2025-%02d-%02dT%02d:%02d:00+00:00", 1+(i%12), 1+i, 8+(i%12), (i*7)%60)
		buckets = append(buckets, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"name":          name,
				"bucket_name":   name,
				"creation_date": createDate,
			},
			RawStruct: s3types.Bucket{
				Name:         aws.String(name),
				BucketArn:    aws.String("arn:aws:s3:::" + name),
				BucketRegion: aws.String("us-east-1"),
				CreationDate: aws.Time(mustParseTime(createDate)),
			},
		})
	}

	for i := range buckets {
		if _, ok := buckets[i].Fields["notification_lambda"]; !ok {
			buckets[i].Fields["notification_lambda"] = ""
		}
		if _, ok := buckets[i].Fields["notification_sqs"]; !ok {
			buckets[i].Fields["notification_sqs"] = ""
		}
		if _, ok := buckets[i].Fields["notification_sns"]; !ok {
			buckets[i].Fields["notification_sns"] = ""
		}
	}

	return buckets
}

func s3ObjDataPipeline() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "logs/2026/03/",
			Name:   "logs/2026/03/",
			Status: "folder",
			Fields: map[string]string{
				"key":           "logs/2026/03/",
				"size":          "-",
				"last_modified": "2026-03-20T12:00:00+00:00",
				"storage_class": "STANDARD",
			},
			RawStruct: s3types.CommonPrefix{
				Prefix: aws.String("logs/2026/03/"),
			},
		},
		{
			ID:     "logs/2026/02/",
			Name:   "logs/2026/02/",
			Status: "folder",
			Fields: map[string]string{
				"key":           "logs/2026/02/",
				"size":          "-",
				"last_modified": "2026-02-28T23:59:59+00:00",
				"storage_class": "STANDARD",
			},
			RawStruct: s3types.CommonPrefix{
				Prefix: aws.String("logs/2026/02/"),
			},
		},
		{
			ID:     "config.json",
			Name:   "config.json",
			Status: "file",
			Fields: map[string]string{
				"key":           "config.json",
				"size":          "2.4 KB",
				"last_modified": "2026-03-18T09:15:22+00:00",
				"storage_class": "STANDARD",
			},
			RawStruct: s3types.Object{
				Key:          aws.String("config.json"),
				Size:         aws.Int64(2458),
				StorageClass: s3types.ObjectStorageClassStandard,
				LastModified: aws.Time(mustParseTime("2026-03-18T09:15:22+00:00")),
			},
		},
		{
			ID:     "schema/pipeline-v2.avro",
			Name:   "schema/pipeline-v2.avro",
			Status: "file",
			Fields: map[string]string{
				"key":           "schema/pipeline-v2.avro",
				"size":          "18.7 KB",
				"last_modified": "2026-01-10T14:32:07+00:00",
				"storage_class": "STANDARD",
			},
			RawStruct: s3types.Object{
				Key:          aws.String("schema/pipeline-v2.avro"),
				Size:         aws.Int64(19148),
				StorageClass: s3types.ObjectStorageClassStandard,
				LastModified: aws.Time(mustParseTime("2026-01-10T14:32:07+00:00")),
			},
		},
		{
			ID:     "archive/2025-q4-summary.parquet",
			Name:   "archive/2025-q4-summary.parquet",
			Status: "file",
			Fields: map[string]string{
				"key":           "archive/2025-q4-summary.parquet",
				"size":          "142.3 MB",
				"last_modified": "2026-01-05T03:00:00+00:00",
				"storage_class": "GLACIER",
			},
			RawStruct: s3types.Object{
				Key:          aws.String("archive/2025-q4-summary.parquet"),
				Size:         aws.Int64(149199462),
				StorageClass: s3types.ObjectStorageClassGlacier,
				LastModified: aws.Time(mustParseTime("2026-01-05T03:00:00+00:00")),
			},
		},
	}
}

func s3ObjWebapp() []resource.Resource {
	return []resource.Resource{
		{ID: "css/", Name: "css/", Status: "folder", Fields: map[string]string{"key": "css/", "size": "-", "last_modified": "2026-03-20T10:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("css/")}},
		{ID: "js/", Name: "js/", Status: "folder", Fields: map[string]string{"key": "js/", "size": "-", "last_modified": "2026-03-20T10:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("js/")}},
		{ID: "images/", Name: "images/", Status: "folder", Fields: map[string]string{"key": "images/", "size": "-", "last_modified": "2026-03-19T15:30:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("images/")}},
		{ID: "index.html", Name: "index.html", Status: "file", Fields: map[string]string{"key": "index.html", "size": "12.4 KB", "last_modified": "2026-03-20T10:05:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("index.html"), Size: aws.Int64(12697), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-20T10:05:00+00:00"))}},
		{ID: "favicon.ico", Name: "favicon.ico", Status: "file", Fields: map[string]string{"key": "favicon.ico", "size": "4.2 KB", "last_modified": "2026-01-10T08:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("favicon.ico"), Size: aws.Int64(4301), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-01-10T08:00:00+00:00"))}},
		{ID: "robots.txt", Name: "robots.txt", Status: "file", Fields: map[string]string{"key": "robots.txt", "size": "68 B", "last_modified": "2025-12-01T12:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("robots.txt"), Size: aws.Int64(68), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2025-12-01T12:00:00+00:00"))}},
	}
}

func s3ObjMLTraining() []resource.Resource {
	return []resource.Resource{
		{ID: "datasets/", Name: "datasets/", Status: "folder", Fields: map[string]string{"key": "datasets/", "size": "-", "last_modified": "2026-03-15T09:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("datasets/")}},
		{ID: "models/", Name: "models/", Status: "folder", Fields: map[string]string{"key": "models/", "size": "-", "last_modified": "2026-03-18T14:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("models/")}},
		{ID: "notebooks/", Name: "notebooks/", Status: "folder", Fields: map[string]string{"key": "notebooks/", "size": "-", "last_modified": "2026-03-10T11:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("notebooks/")}},
		{ID: "config.yaml", Name: "config.yaml", Status: "file", Fields: map[string]string{"key": "config.yaml", "size": "1.8 KB", "last_modified": "2026-03-19T16:22:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("config.yaml"), Size: aws.Int64(1843), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-19T16:22:00+00:00"))}},
		{ID: "training-results-v3.json", Name: "training-results-v3.json", Status: "file", Fields: map[string]string{"key": "training-results-v3.json", "size": "847 KB", "last_modified": "2026-03-18T14:30:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("training-results-v3.json"), Size: aws.Int64(867328), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-18T14:30:00+00:00"))}},
	}
}

func s3ObjTerraform() []resource.Resource {
	return []resource.Resource{
		{ID: "env:/", Name: "env:/", Status: "folder", Fields: map[string]string{"key": "env:/", "size": "-", "last_modified": "2026-03-20T08:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("env:/")}},
		{ID: "prod/vpc.tfstate", Name: "prod/vpc.tfstate", Status: "file", Fields: map[string]string{"key": "prod/vpc.tfstate", "size": "245 KB", "last_modified": "2026-03-20T08:15:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("prod/vpc.tfstate"), Size: aws.Int64(250880), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-20T08:15:00+00:00"))}},
		{ID: "prod/eks.tfstate", Name: "prod/eks.tfstate", Status: "file", Fields: map[string]string{"key": "prod/eks.tfstate", "size": "189 KB", "last_modified": "2026-03-19T22:30:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("prod/eks.tfstate"), Size: aws.Int64(193536), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-19T22:30:00+00:00"))}},
		{ID: "staging/main.tfstate", Name: "staging/main.tfstate", Status: "file", Fields: map[string]string{"key": "staging/main.tfstate", "size": "312 KB", "last_modified": "2026-03-18T11:45:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("staging/main.tfstate"), Size: aws.Int64(319488), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-18T11:45:00+00:00"))}},
	}
}

func s3ObjCloudtrail() []resource.Resource {
	return []resource.Resource{
		{ID: "AWSLogs/", Name: "AWSLogs/", Status: "folder", Fields: map[string]string{"key": "AWSLogs/", "size": "-", "last_modified": "2026-03-21T00:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("AWSLogs/")}},
		{ID: "AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/21/event-001.json.gz", Name: "AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/21/event-001.json.gz", Status: "file", Fields: map[string]string{"key": "AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/21/event-001.json.gz", "size": "54.2 KB", "last_modified": "2026-03-21T00:05:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/21/event-001.json.gz"), Size: aws.Int64(55501), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-21T00:05:00+00:00"))}},
		{ID: "AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/20/digest.json.gz", Name: "AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/20/digest.json.gz", Status: "file", Fields: map[string]string{"key": "AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/20/digest.json.gz", "size": "1.1 KB", "last_modified": "2026-03-20T23:59:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/20/digest.json.gz"), Size: aws.Int64(1127), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-20T23:59:00+00:00"))}},
	}
}

func s3ObjBackups() []resource.Resource {
	return []resource.Resource{
		{ID: "rds/", Name: "rds/", Status: "folder", Fields: map[string]string{"key": "rds/", "size": "-", "last_modified": "2026-03-20T04:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("rds/")}},
		{ID: "docdb/", Name: "docdb/", Status: "folder", Fields: map[string]string{"key": "docdb/", "size": "-", "last_modified": "2026-03-19T04:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("docdb/")}},
		{ID: "rds/prod-api-primary-2026-03-20.snap", Name: "rds/prod-api-primary-2026-03-20.snap", Status: "file", Fields: map[string]string{"key": "rds/prod-api-primary-2026-03-20.snap", "size": "2.3 GB", "last_modified": "2026-03-20T04:15:00+00:00", "storage_class": "STANDARD_IA"}, RawStruct: s3types.Object{Key: aws.String("rds/prod-api-primary-2026-03-20.snap"), Size: aws.Int64(2469606195), StorageClass: s3types.ObjectStorageClassStandardIa, LastModified: aws.Time(mustParseTime("2026-03-20T04:15:00+00:00"))}},
		{ID: "rds/prod-api-primary-2026-03-19.snap", Name: "rds/prod-api-primary-2026-03-19.snap", Status: "file", Fields: map[string]string{"key": "rds/prod-api-primary-2026-03-19.snap", "size": "2.3 GB", "last_modified": "2026-03-19T04:15:00+00:00", "storage_class": "GLACIER"}, RawStruct: s3types.Object{Key: aws.String("rds/prod-api-primary-2026-03-19.snap"), Size: aws.Int64(2469606195), StorageClass: s3types.ObjectStorageClassGlacier, LastModified: aws.Time(mustParseTime("2026-03-19T04:15:00+00:00"))}},
	}
}

// data-pipeline-logs sub-folder fixtures

func s3ObjDataPipelineLogs() []resource.Resource {
	return []resource.Resource{
		{ID: "logs/2026/", Name: "logs/2026/", Status: "folder", Fields: map[string]string{"key": "logs/2026/", "size": "", "last_modified": "", "storage_class": ""}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("logs/2026/")}},
		{ID: "logs/2025/", Name: "logs/2025/", Status: "folder", Fields: map[string]string{"key": "logs/2025/", "size": "", "last_modified": "", "storage_class": ""}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("logs/2025/")}},
	}
}

func s3ObjDataPipelineLogs2026() []resource.Resource {
	return []resource.Resource{
		{ID: "logs/2026/03/", Name: "logs/2026/03/", Status: "folder", Fields: map[string]string{"key": "logs/2026/03/", "size": "", "last_modified": "", "storage_class": ""}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("logs/2026/03/")}},
		{ID: "logs/2026/02/", Name: "logs/2026/02/", Status: "folder", Fields: map[string]string{"key": "logs/2026/02/", "size": "", "last_modified": "", "storage_class": ""}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("logs/2026/02/")}},
		{ID: "logs/2026/01/", Name: "logs/2026/01/", Status: "folder", Fields: map[string]string{"key": "logs/2026/01/", "size": "", "last_modified": "", "storage_class": ""}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("logs/2026/01/")}},
	}
}

func s3ObjDataPipelineLogs202603() []resource.Resource {
	return []resource.Resource{
		{ID: "logs/2026/03/access-2026-03-21.log", Name: "logs/2026/03/access-2026-03-21.log", Status: "file", Fields: map[string]string{"key": "logs/2026/03/access-2026-03-21.log", "size": "4.7 MB", "last_modified": "2026-03-21T06:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("logs/2026/03/access-2026-03-21.log"), Size: aws.Int64(4928307), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-21T06:00:00Z"))}},
		{ID: "logs/2026/03/access-2026-03-20.log", Name: "logs/2026/03/access-2026-03-20.log", Status: "file", Fields: map[string]string{"key": "logs/2026/03/access-2026-03-20.log", "size": "5.1 MB", "last_modified": "2026-03-20T06:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("logs/2026/03/access-2026-03-20.log"), Size: aws.Int64(5347737), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-20T06:00:00Z"))}},
		{ID: "logs/2026/03/error-2026-03-21.log", Name: "logs/2026/03/error-2026-03-21.log", Status: "file", Fields: map[string]string{"key": "logs/2026/03/error-2026-03-21.log", "size": "128 KB", "last_modified": "2026-03-21T06:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("logs/2026/03/error-2026-03-21.log"), Size: aws.Int64(131072), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-21T06:00:00Z"))}},
		{ID: "logs/2026/03/error-2026-03-20.log", Name: "logs/2026/03/error-2026-03-20.log", Status: "file", Fields: map[string]string{"key": "logs/2026/03/error-2026-03-20.log", "size": "96 KB", "last_modified": "2026-03-20T06:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("logs/2026/03/error-2026-03-20.log"), Size: aws.Int64(98304), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-20T06:00:00Z"))}},
	}
}

func s3ObjDataPipelineLogs202602() []resource.Resource {
	return []resource.Resource{
		{ID: "logs/2026/02/access-2026-02-28.log", Name: "logs/2026/02/access-2026-02-28.log", Status: "file", Fields: map[string]string{"key": "logs/2026/02/access-2026-02-28.log", "size": "3.9 MB", "last_modified": "2026-02-28T06:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("logs/2026/02/access-2026-02-28.log"), Size: aws.Int64(4089446), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-02-28T06:00:00Z"))}},
		{ID: "logs/2026/02/access-2026-02-27.log", Name: "logs/2026/02/access-2026-02-27.log", Status: "file", Fields: map[string]string{"key": "logs/2026/02/access-2026-02-27.log", "size": "4.2 MB", "last_modified": "2026-02-27T06:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("logs/2026/02/access-2026-02-27.log"), Size: aws.Int64(4404019), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-02-27T06:00:00Z"))}},
		{ID: "logs/2026/02/error-2026-02-28.log", Name: "logs/2026/02/error-2026-02-28.log", Status: "file", Fields: map[string]string{"key": "logs/2026/02/error-2026-02-28.log", "size": "67 KB", "last_modified": "2026-02-28T06:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("logs/2026/02/error-2026-02-28.log"), Size: aws.Int64(68608), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-02-28T06:00:00Z"))}},
	}
}

// webapp-assets-prod sub-folder fixtures

func s3ObjWebappCSS() []resource.Resource {
	return []resource.Resource{
		{ID: "css/main.css", Name: "css/main.css", Status: "file", Fields: map[string]string{"key": "css/main.css", "size": "24.3 KB", "last_modified": "2026-03-20T10:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("css/main.css"), Size: aws.Int64(24883), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-20T10:00:00Z"))}},
		{ID: "css/vendor.css", Name: "css/vendor.css", Status: "file", Fields: map[string]string{"key": "css/vendor.css", "size": "156 KB", "last_modified": "2026-03-15T09:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("css/vendor.css"), Size: aws.Int64(159744), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-15T09:00:00Z"))}},
	}
}

func s3ObjWebappJS() []resource.Resource {
	return []resource.Resource{
		{ID: "js/app.bundle.js", Name: "js/app.bundle.js", Status: "file", Fields: map[string]string{"key": "js/app.bundle.js", "size": "342 KB", "last_modified": "2026-03-20T10:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("js/app.bundle.js"), Size: aws.Int64(350208), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-20T10:00:00Z"))}},
		{ID: "js/vendor.bundle.js", Name: "js/vendor.bundle.js", Status: "file", Fields: map[string]string{"key": "js/vendor.bundle.js", "size": "1.2 MB", "last_modified": "2026-03-15T09:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("js/vendor.bundle.js"), Size: aws.Int64(1258291), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-15T09:00:00Z"))}},
	}
}

func s3ObjWebappImages() []resource.Resource {
	return []resource.Resource{
		{ID: "images/logo.png", Name: "images/logo.png", Status: "file", Fields: map[string]string{"key": "images/logo.png", "size": "18.4 KB", "last_modified": "2026-01-15T12:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("images/logo.png"), Size: aws.Int64(18841), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-01-15T12:00:00Z"))}},
		{ID: "images/hero-banner.jpg", Name: "images/hero-banner.jpg", Status: "file", Fields: map[string]string{"key": "images/hero-banner.jpg", "size": "245 KB", "last_modified": "2026-03-19T15:30:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("images/hero-banner.jpg"), Size: aws.Int64(250880), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-19T15:30:00Z"))}},
		{ID: "images/favicon-32x32.png", Name: "images/favicon-32x32.png", Status: "file", Fields: map[string]string{"key": "images/favicon-32x32.png", "size": "1.4 KB", "last_modified": "2026-01-15T12:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("images/favicon-32x32.png"), Size: aws.Int64(1434), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-01-15T12:00:00Z"))}},
	}
}

// ml-training-data sub-folder fixtures

func s3ObjMLDatasets() []resource.Resource {
	return []resource.Resource{
		{ID: "datasets/train.csv", Name: "datasets/train.csv", Status: "file", Fields: map[string]string{"key": "datasets/train.csv", "size": "1.8 GB", "last_modified": "2026-03-15T09:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("datasets/train.csv"), Size: aws.Int64(1932735283), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-15T09:00:00Z"))}},
		{ID: "datasets/validation.csv", Name: "datasets/validation.csv", Status: "file", Fields: map[string]string{"key": "datasets/validation.csv", "size": "450 MB", "last_modified": "2026-03-15T09:05:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("datasets/validation.csv"), Size: aws.Int64(471859200), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-15T09:05:00Z"))}},
		{ID: "datasets/test.csv", Name: "datasets/test.csv", Status: "file", Fields: map[string]string{"key": "datasets/test.csv", "size": "220 MB", "last_modified": "2026-03-15T09:10:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("datasets/test.csv"), Size: aws.Int64(230686720), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-15T09:10:00Z"))}},
	}
}

func s3ObjMLModels() []resource.Resource {
	return []resource.Resource{
		{ID: "models/v3-final.tar.gz", Name: "models/v3-final.tar.gz", Status: "file", Fields: map[string]string{"key": "models/v3-final.tar.gz", "size": "892 MB", "last_modified": "2026-03-18T14:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("models/v3-final.tar.gz"), Size: aws.Int64(935329792), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-18T14:00:00Z"))}},
		{ID: "models/v2-baseline.tar.gz", Name: "models/v2-baseline.tar.gz", Status: "file", Fields: map[string]string{"key": "models/v2-baseline.tar.gz", "size": "756 MB", "last_modified": "2026-02-20T11:00:00+00:00", "storage_class": "STANDARD_IA"}, RawStruct: s3types.Object{Key: aws.String("models/v2-baseline.tar.gz"), Size: aws.Int64(792723456), StorageClass: s3types.ObjectStorageClassStandardIa, LastModified: aws.Time(mustParseTime("2026-02-20T11:00:00Z"))}},
	}
}

func s3ObjMLNotebooks() []resource.Resource {
	return []resource.Resource{
		{ID: "notebooks/exploration.ipynb", Name: "notebooks/exploration.ipynb", Status: "file", Fields: map[string]string{"key": "notebooks/exploration.ipynb", "size": "342 KB", "last_modified": "2026-03-10T11:00:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("notebooks/exploration.ipynb"), Size: aws.Int64(350208), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-10T11:00:00Z"))}},
		{ID: "notebooks/feature-engineering.ipynb", Name: "notebooks/feature-engineering.ipynb", Status: "file", Fields: map[string]string{"key": "notebooks/feature-engineering.ipynb", "size": "567 KB", "last_modified": "2026-03-12T16:30:00+00:00", "storage_class": "STANDARD"}, RawStruct: s3types.Object{Key: aws.String("notebooks/feature-engineering.ipynb"), Size: aws.Int64(580608), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustParseTime("2026-03-12T16:30:00Z"))}},
	}
}

// terraform-state-prod sub-folder fixture

func s3ObjTerraformEnv() []resource.Resource {
	return []resource.Resource{
		{ID: "env:/prod/", Name: "env:/prod/", Status: "folder", Fields: map[string]string{"key": "env:/prod/", "size": "", "last_modified": "", "storage_class": ""}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("env:/prod/")}},
		{ID: "env:/staging/", Name: "env:/staging/", Status: "folder", Fields: map[string]string{"key": "env:/staging/", "size": "", "last_modified": "", "storage_class": ""}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("env:/staging/")}},
	}
}

// cloudtrail-audit-logs sub-folder fixture

func s3ObjCloudtrailAWSLogs() []resource.Resource {
	return []resource.Resource{
		{ID: "AWSLogs/123456789012/", Name: "AWSLogs/123456789012/", Status: "folder", Fields: map[string]string{"key": "AWSLogs/123456789012/", "size": "", "last_modified": "", "storage_class": ""}, RawStruct: s3types.CommonPrefix{Prefix: aws.String("AWSLogs/123456789012/")}},
	}
}

// backup-db-snapshots sub-folder fixtures

func s3ObjBackupsRDS() []resource.Resource {
	return []resource.Resource{
		{ID: "rds/prod-api-primary-2026-03-20.snap", Name: "rds/prod-api-primary-2026-03-20.snap", Status: "file", Fields: map[string]string{"key": "rds/prod-api-primary-2026-03-20.snap", "size": "2.3 GB", "last_modified": "2026-03-20T04:15:00+00:00", "storage_class": "STANDARD_IA"}, RawStruct: s3types.Object{Key: aws.String("rds/prod-api-primary-2026-03-20.snap"), Size: aws.Int64(2469606195), StorageClass: s3types.ObjectStorageClassStandardIa, LastModified: aws.Time(mustParseTime("2026-03-20T04:15:00Z"))}},
		{ID: "rds/prod-api-primary-2026-03-19.snap", Name: "rds/prod-api-primary-2026-03-19.snap", Status: "file", Fields: map[string]string{"key": "rds/prod-api-primary-2026-03-19.snap", "size": "2.3 GB", "last_modified": "2026-03-19T04:15:00+00:00", "storage_class": "GLACIER"}, RawStruct: s3types.Object{Key: aws.String("rds/prod-api-primary-2026-03-19.snap"), Size: aws.Int64(2469606195), StorageClass: s3types.ObjectStorageClassGlacier, LastModified: aws.Time(mustParseTime("2026-03-19T04:15:00Z"))}},
	}
}

func s3ObjBackupsDocDB() []resource.Resource {
	return []resource.Resource{
		{ID: "docdb/orders-cluster-2026-03-19.snap", Name: "docdb/orders-cluster-2026-03-19.snap", Status: "file", Fields: map[string]string{"key": "docdb/orders-cluster-2026-03-19.snap", "size": "1.1 GB", "last_modified": "2026-03-19T04:30:00+00:00", "storage_class": "STANDARD_IA"}, RawStruct: s3types.Object{Key: aws.String("docdb/orders-cluster-2026-03-19.snap"), Size: aws.Int64(1181116006), StorageClass: s3types.ObjectStorageClassStandardIa, LastModified: aws.Time(mustParseTime("2026-03-19T04:30:00Z"))}},
	}
}

// rdsInstances returns demo RDS (DB Instance) fixtures.
func rdsInstances() []resource.Resource {
	instances := []resource.Resource{
		{
			ID:     "prod-api-primary",
			Name:   "prod-api-primary",
			Status: "available",
			Fields: map[string]string{
				"db_identifier":  "prod-api-primary",
				"engine":         "aurora-postgresql",
				"engine_version": "16.4",
				"status":         "available",
				"class":          "db.r6g.xlarge",
				"endpoint":       "prod-api-primary.cluster-c9xyz123.us-east-1.rds.amazonaws.com",
				"multi_az":       "Yes",
			},
			RawStruct: rdstypes.DBInstance{
				DBInstanceIdentifier:       aws.String("prod-api-primary"),
				DBInstanceArn:              aws.String("arn:aws:rds:us-east-1:123456789012:db:prod-api-primary"),
				Engine:                     aws.String("aurora-postgresql"),
				EngineVersion:              aws.String("16.4"),
				DBInstanceStatus:           aws.String("available"),
				DBInstanceClass:            aws.String("db.r6g.xlarge"),
				MasterUsername:             aws.String("pgadmin"),
				AvailabilityZone:           aws.String("us-east-1a"),
				AllocatedStorage:           aws.Int32(100),
				StorageType:                aws.String("aurora"),
				StorageEncrypted:           aws.Bool(true),
				KmsKeyId:                   aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				Iops:                       aws.Int32(0),
				BackupRetentionPeriod:      aws.Int32(7),
				PreferredBackupWindow:      aws.String("03:00-04:00"),
				PreferredMaintenanceWindow: aws.String("sun:05:00-sun:06:00"),
				DeletionProtection:         aws.Bool(true),
				PubliclyAccessible:         aws.Bool(false),
				PerformanceInsightsEnabled: aws.Bool(true),
				Endpoint: &rdstypes.Endpoint{
					Address: aws.String("prod-api-primary.cluster-c9xyz123.us-east-1.rds.amazonaws.com"),
					Port:    aws.Int32(5432),
				},
				MultiAZ: aws.Bool(true),
				VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
					{VpcSecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				DBSubnetGroup: &rdstypes.DBSubnetGroup{
					VpcId:             aws.String(prodVPCID),
					DBSubnetGroupName: aws.String("acme-rds-subnet-group"),
				},
				TagList: []rdstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("production")},
					{Key: aws.String("Team"), Value: aws.String("backend")},
				},
			},
		},
		{
			ID:     "prod-api-replica",
			Name:   "prod-api-replica",
			Status: "available",
			Fields: map[string]string{
				"db_identifier":  "prod-api-replica",
				"engine":         "aurora-postgresql",
				"engine_version": "16.4",
				"status":         "available",
				"class":          "db.r6g.large",
				"endpoint":       "prod-api-replica.c9xyz123.us-east-1.rds.amazonaws.com",
				"multi_az":       "No",
			},
			RawStruct: rdstypes.DBInstance{
				DBInstanceIdentifier:       aws.String("prod-api-replica"),
				DBInstanceArn:              aws.String("arn:aws:rds:us-east-1:123456789012:db:prod-api-replica"),
				Engine:                     aws.String("aurora-postgresql"),
				EngineVersion:              aws.String("16.4"),
				DBInstanceStatus:           aws.String("available"),
				DBInstanceClass:            aws.String("db.r6g.large"),
				MasterUsername:             aws.String("pgadmin"),
				AvailabilityZone:           aws.String("us-east-1b"),
				AllocatedStorage:           aws.Int32(100),
				StorageType:                aws.String("aurora"),
				StorageEncrypted:           aws.Bool(true),
				KmsKeyId:                   aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				Iops:                       aws.Int32(0),
				BackupRetentionPeriod:      aws.Int32(7),
				PreferredBackupWindow:      aws.String("03:00-04:00"),
				PreferredMaintenanceWindow: aws.String("sun:05:00-sun:06:00"),
				DeletionProtection:         aws.Bool(true),
				PubliclyAccessible:         aws.Bool(false),
				PerformanceInsightsEnabled: aws.Bool(true),
				Endpoint: &rdstypes.Endpoint{
					Address: aws.String("prod-api-replica.c9xyz123.us-east-1.rds.amazonaws.com"),
					Port:    aws.Int32(5432),
				},
				MultiAZ: aws.Bool(false),
				VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
					{VpcSecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				DBSubnetGroup: &rdstypes.DBSubnetGroup{
					VpcId:             aws.String(prodVPCID),
					DBSubnetGroupName: aws.String("acme-rds-subnet-group"),
				},
				TagList: []rdstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "analytics-warehouse",
			Name:   "analytics-warehouse",
			Status: "available",
			Fields: map[string]string{
				"db_identifier":  "analytics-warehouse",
				"engine":         "postgres",
				"engine_version": "16.2",
				"status":         "available",
				"class":          "db.m6g.2xlarge",
				"endpoint":       "analytics-warehouse.c9xyz123.us-east-1.rds.amazonaws.com",
				"multi_az":       "Yes",
			},
			RawStruct: rdstypes.DBInstance{
				DBInstanceIdentifier:       aws.String("analytics-warehouse"),
				DBInstanceArn:              aws.String("arn:aws:rds:us-east-1:123456789012:db:analytics-warehouse"),
				Engine:                     aws.String("postgres"),
				EngineVersion:              aws.String("16.2"),
				DBInstanceStatus:           aws.String("available"),
				DBInstanceClass:            aws.String("db.m6g.2xlarge"),
				MasterUsername:             aws.String("analytics"),
				AvailabilityZone:           aws.String("us-east-1a"),
				AllocatedStorage:           aws.Int32(500),
				StorageType:                aws.String("gp3"),
				Iops:                       aws.Int32(3000),
				StorageEncrypted:           aws.Bool(true),
				KmsKeyId:                   aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				BackupRetentionPeriod:      aws.Int32(14),
				PreferredBackupWindow:      aws.String("04:00-05:00"),
				PreferredMaintenanceWindow: aws.String("mon:06:00-mon:07:00"),
				DeletionProtection:         aws.Bool(true),
				PubliclyAccessible:         aws.Bool(false),
				PerformanceInsightsEnabled: aws.Bool(true),
				Endpoint: &rdstypes.Endpoint{
					Address: aws.String("analytics-warehouse.c9xyz123.us-east-1.rds.amazonaws.com"),
					Port:    aws.Int32(5432),
				},
				MultiAZ: aws.Bool(true),
				VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
					{VpcSecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				DBSubnetGroup: &rdstypes.DBSubnetGroup{
					VpcId:             aws.String(prodVPCID),
					DBSubnetGroupName: aws.String("acme-rds-subnet-group"),
				},
				TagList: []rdstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Team"), Value: aws.String("data")},
				},
			},
		},
		{
			ID:     "staging-mysql",
			Name:   "staging-mysql",
			Status: "stopped",
			Fields: map[string]string{
				"db_identifier":  "staging-mysql",
				"engine":         "mysql",
				"engine_version": "8.0.36",
				"status":         "stopped",
				"class":          "db.t3.medium",
				"endpoint":       "staging-mysql.c9xyz123.us-east-1.rds.amazonaws.com",
				"multi_az":       "No",
			},
			RawStruct: rdstypes.DBInstance{
				DBInstanceIdentifier:       aws.String("staging-mysql"),
				DBInstanceArn:              aws.String("arn:aws:rds:us-east-1:123456789012:db:staging-mysql"),
				Engine:                     aws.String("mysql"),
				EngineVersion:              aws.String("8.0.36"),
				DBInstanceStatus:           aws.String("stopped"),
				DBInstanceClass:            aws.String("db.t3.medium"),
				MasterUsername:             aws.String("stgadmin"),
				AvailabilityZone:           aws.String("us-east-1b"),
				AllocatedStorage:           aws.Int32(50),
				StorageType:                aws.String("gp2"),
				Iops:                       aws.Int32(0),
				StorageEncrypted:           aws.Bool(false),
				BackupRetentionPeriod:      aws.Int32(1),
				PreferredBackupWindow:      aws.String("05:00-06:00"),
				PreferredMaintenanceWindow: aws.String("tue:07:00-tue:08:00"),
				DeletionProtection:         aws.Bool(false),
				PubliclyAccessible:         aws.Bool(false),
				PerformanceInsightsEnabled: aws.Bool(false),
				Endpoint: &rdstypes.Endpoint{
					Address: aws.String("staging-mysql.c9xyz123.us-east-1.rds.amazonaws.com"),
					Port:    aws.Int32(3306),
				},
				MultiAZ: aws.Bool(false),
				VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
					{VpcSecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				DBSubnetGroup: &rdstypes.DBSubnetGroup{
					VpcId:             aws.String(stagingVPCID),
					DBSubnetGroupName: aws.String("acme-staging-rds-subnet-group"),
				},
				TagList: []rdstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
		{
			ID:     "dev-feature-branch",
			Name:   "dev-feature-branch",
			Status: "creating",
			Fields: map[string]string{
				"db_identifier":  "dev-feature-branch",
				"engine":         "aurora-postgresql",
				"engine_version": "16.4",
				"status":         "creating",
				"class":          "db.t3.medium",
				"endpoint":       "",
				"multi_az":       "No",
			},
			RawStruct: rdstypes.DBInstance{
				DBInstanceIdentifier:       aws.String("dev-feature-branch"),
				DBInstanceArn:              aws.String("arn:aws:rds:us-east-1:123456789012:db:dev-feature-branch"),
				Engine:                     aws.String("aurora-postgresql"),
				EngineVersion:              aws.String("16.4"),
				DBInstanceStatus:           aws.String("creating"),
				DBInstanceClass:            aws.String("db.t3.medium"),
				MasterUsername:             aws.String("devadmin"),
				AvailabilityZone:           aws.String("us-east-1a"),
				AllocatedStorage:           aws.Int32(20),
				StorageType:                aws.String("aurora"),
				Iops:                       aws.Int32(0),
				StorageEncrypted:           aws.Bool(true),
				KmsKeyId:                   aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				BackupRetentionPeriod:      aws.Int32(1),
				PreferredBackupWindow:      aws.String("06:00-07:00"),
				PreferredMaintenanceWindow: aws.String("wed:08:00-wed:09:00"),
				DeletionProtection:         aws.Bool(false),
				PubliclyAccessible:         aws.Bool(false),
				PerformanceInsightsEnabled: aws.Bool(false),
				MultiAZ:                    aws.Bool(false),
				VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
					{VpcSecurityGroupId: aws.String(prodRDSSGID), Status: aws.String("active")},
				},
				DBSubnetGroup: &rdstypes.DBSubnetGroup{
					VpcId:             aws.String(prodVPCID),
					DBSubnetGroupName: aws.String("acme-rds-subnet-group"),
				},
				TagList: []rdstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("dev")},
				},
			},
		},
	}

	// Generate 17 more instances to reach 22 total
	rdsStatuses := []string{
		"available", "available", "available", "available", "stopped",
		"available", "available", "backing-up", "available", "available",
		"available", "modifying", "available", "available", "available",
		"stopped", "available",
	}
	for i := 0; i < 17; i++ {
		eng := rdsEnginePool[i]
		name := rdsNamePool[i]
		class := rdsClassPool[i]
		status := rdsStatuses[i]
		multiAZ := "No"
		if i%3 == 0 {
			multiAZ = "Yes"
		}
		endpoint := fmt.Sprintf("%s.c9xyz123.us-east-1.rds.amazonaws.com", name)
		if status == "creating" {
			endpoint = ""
		}
		instances = append(instances, resource.Resource{
			ID:     name,
			Name:   name,
			Status: status,
			Fields: map[string]string{
				"db_identifier":  name,
				"engine":         eng.Engine,
				"engine_version": eng.EngineVersion,
				"status":         status,
				"class":          class,
				"endpoint":       endpoint,
				"multi_az":       multiAZ,
			},
			RawStruct: rdstypes.DBInstance{
				DBInstanceIdentifier: aws.String(name),
				Engine:               aws.String(eng.Engine),
				EngineVersion:        aws.String(eng.EngineVersion),
				DBInstanceStatus:     aws.String(status),
				DBInstanceClass:      aws.String(class),
				Endpoint: &rdstypes.Endpoint{
					Address: aws.String(endpoint),
					Port:    aws.Int32(eng.Port),
				},
				MultiAZ: aws.Bool(multiAZ == "Yes"),
			},
		})
	}

	return instances
}

// dbiEventFixtures returns demo RDS DB instance event fixtures.
func dbiEventFixtures(dbIdentifier string) []resource.Resource {
	now := time.Now().UTC()
	ts := func(hoursAgo int) time.Time {
		return now.Add(-time.Duration(hoursAgo) * time.Hour)
	}
	fmtTS := func(t time.Time) string {
		return t.Format("2006-01-02 15:04")
	}

	events := []struct {
		hoursAgo   int
		categories []string
		message    string
		sourceType rdstypes.SourceType
	}{
		{2, []string{"maintenance"}, "Applying offline patches to DB instance", rdstypes.SourceTypeDbInstance},
		{6, []string{"maintenance"}, "Finished applying offline patches to DB instance", rdstypes.SourceTypeDbInstance},
		{12, []string{"failover"}, "Started cross AZ failover to DB instance: " + dbIdentifier, rdstypes.SourceTypeDbInstance},
		{18, []string{"availability"}, "DB instance restarted", rdstypes.SourceTypeDbInstance},
		{24, []string{"notification"}, "DB instance is being started", rdstypes.SourceTypeDbInstance},
		{48, []string{"notification"}, "Recovery of the DB instance has started. Recovery time will vary with the amount of data to be recovered.", rdstypes.SourceTypeDbInstance},
		{72, []string{"configuration change"}, "Updated to use DBParameterGroup default.postgres16", rdstypes.SourceTypeDbInstance},
	}

	sourceArn := "arn:aws:rds:us-east-1:123456789012:db:" + dbIdentifier

	var resources []resource.Resource
	for _, e := range events {
		t := ts(e.hoursAgo)
		timestamp := fmtTS(t)
		categories := ""
		for i, c := range e.categories {
			if i > 0 {
				categories += ", "
			}
			categories += c
		}
		resources = append(resources, resource.Resource{
			ID:   timestamp + "/" + dbIdentifier,
			Name: timestamp,
			Fields: map[string]string{
				"timestamp":         timestamp,
				"event_categories":  categories,
				"message":           e.message,
				"source_identifier": dbIdentifier,
				"source_type":       string(e.sourceType),
				"source_arn":        sourceArn,
			},
			RawStruct: rdstypes.Event{
				Date:             aws.Time(t),
				EventCategories:  e.categories,
				Message:          aws.String(e.message),
				SourceIdentifier: aws.String(dbIdentifier),
				SourceType:       e.sourceType,
				SourceArn:        aws.String(sourceArn),
			},
		})
	}

	return resources
}
