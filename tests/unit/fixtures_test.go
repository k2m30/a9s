package unit

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/internal/resource"
)

// fixtureS3Buckets returns sanitized S3 bucket data for testing.
// Source: sanitized from real AWS data (5 buckets shown).
func fixtureS3Buckets() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "test-app-state",
			Name:   "test-app-state",
			Status: "",
			Fields: map[string]string{
				"name":          "test-app-state",
				"bucket_name":   "test-app-state",
				"creation_date": "2025-06-20T11:35:46+00:00",
			},
		},
		{
			ID:     "cdn-logs.example.com",
			Name:   "cdn-logs.example.com",
			Status: "",
			Fields: map[string]string{
				"name":          "cdn-logs.example.com",
				"bucket_name":   "cdn-logs.example.com",
				"creation_date": "2025-05-12T19:24:13+00:00",
			},
		},
		{
			ID:     "cdn-website.example.com",
			Name:   "cdn-website.example.com",
			Status: "",
			Fields: map[string]string{
				"name":          "cdn-website.example.com",
				"bucket_name":   "cdn-website.example.com",
				"creation_date": "2025-05-13T17:36:40+00:00",
			},
		},
		{
			ID:     "dev-fileshare",
			Name:   "dev-fileshare",
			Status: "",
			Fields: map[string]string{
				"name":          "dev-fileshare",
				"bucket_name":   "dev-fileshare",
				"creation_date": "2025-03-06T11:49:21+00:00",
			},
		},
		{
			ID:     "dev-loki-chunks",
			Name:   "dev-loki-chunks",
			Status: "",
			Fields: map[string]string{
				"name":          "dev-loki-chunks",
				"bucket_name":   "dev-loki-chunks",
				"creation_date": "2025-07-01T13:57:58+00:00",
			},
		},
	}
}

// fixtureS3Objects returns sanitized S3 object data for testing.
// Source: sanitized from real AWS data.
func fixtureS3Objects() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "dev/terraform.tfstate",
			Name:   "dev/terraform.tfstate",
			Status: "file",
			Fields: map[string]string{
				"key":           "dev/terraform.tfstate",
				"size":          "61.9 KB",
				"last_modified": "2025-10-14T08:49:08+00:00",
				"storage_class": "STANDARD",
			},
		},
	}
}

// fixtureEC2Instances returns sanitized EC2 instance data for testing.
// Source: sanitized from real AWS data (6 instances).
func fixtureEC2Instances() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "i-0aaa111111111111a",
			Name:   "",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-0aaa111111111111a",
				"name":        "",
				"state":       "running",
				"type":        "g4dn.xlarge",
				"private_ip":  "10.0.48.186",
				"public_ip":   "203.0.113.20",
				"launch_time": "2026-02-25T17:03:15+00:00",
			},
		},
		{
			ID:     "i-0bbb222222222222b",
			Name:   "VPN",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-0bbb222222222222b",
				"name":        "VPN",
				"state":       "running",
				"type":        "t3.large",
				"private_ip":  "10.0.48.175",
				"public_ip":   "203.0.113.10",
				"launch_time": "2025-07-25T12:26:50+00:00",
			},
		},
		{
			ID:     "i-0ccc333333333333c",
			Name:   "kafka",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-0ccc333333333333c",
				"name":        "kafka",
				"state":       "running",
				"type":        "t3.large",
				"private_ip":  "10.0.12.47",
				"public_ip":   "",
				"launch_time": "2025-09-05T11:53:44+00:00",
			},
		},
		{
			ID:     "i-0ddd444444444444d",
			Name:   "monitoring",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-0ddd444444444444d",
				"name":        "monitoring",
				"state":       "running",
				"type":        "t3.large",
				"private_ip":  "10.0.0.32",
				"public_ip":   "",
				"launch_time": "2026-03-06T14:06:22+00:00",
			},
		},
		{
			ID:     "i-0eee555555555555e",
			Name:   "apps-on-demand",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-0eee555555555555e",
				"name":        "apps-on-demand",
				"state":       "running",
				"type":        "t3.xlarge",
				"private_ip":  "10.0.3.140",
				"public_ip":   "",
				"launch_time": "2026-03-17T14:17:19+00:00",
			},
		},
		{
			ID:     "i-0fff666666666666f",
			Name:   "apps",
			Status: "terminated",
			Fields: map[string]string{
				"instance_id": "i-0fff666666666666f",
				"name":        "apps",
				"state":       "terminated",
				"type":        "t3.large",
				"private_ip":  "",
				"public_ip":   "",
				"launch_time": "2026-03-18T01:00:03+00:00",
			},
		},
	}
}

// fixtureRDSInstances returns sanitized RDS instance data for testing.
// Source: sanitized from real AWS data (2 instances).
func fixtureRDSInstances() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "test-docdb-1",
			Name:   "test-docdb-1",
			Status: "available",
			Fields: map[string]string{
				"db_identifier":  "test-docdb-1",
				"engine":         "dbc",
				"engine_version": "5.0.0",
				"status":         "available",
				"class":          "db.r5.large",
				"endpoint":       "test-docdb-1.cluster-abc123def.us-east-1.docdb.amazonaws.com",
				"multi_az":       "No",
			},
		},
		{
			ID:     "test-rds-1",
			Name:   "test-rds-1",
			Status: "available",
			Fields: map[string]string{
				"db_identifier":  "test-rds-1",
				"engine":         "aurora-postgresql",
				"engine_version": "16.8",
				"status":         "available",
				"class":          "db.t3.medium",
				"endpoint":       "test-rds-1.cluster-abc123def.us-east-1.rds.amazonaws.com",
				"multi_az":       "No",
			},
		},
	}
}

// fixtureRedisClusters returns sanitized ElastiCache Redis cluster data for testing.
// Source: sanitized from real AWS data (1 cluster).
func fixtureRedisClusters() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "test-redis-1",
			Name:   "test-redis-1",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":     "test-redis-1",
				"engine_version": "7.0.7",
				"node_type":      "cache.t2.micro",
				"status":         "available",
				"nodes":          "1",
				"endpoint":       "",
			},
			RawStruct: elasticachetypes.CacheCluster{
				CacheClusterId:     aws.String("test-redis-1"),
				Engine:             aws.String("redis"),
				EngineVersion:      aws.String("7.0.7"),
				CacheNodeType:      aws.String("cache.t2.micro"),
				CacheClusterStatus: aws.String("available"),
				NumCacheNodes:      aws.Int32(1),
				// ConfigurationEndpoint is nil (matches empty endpoint in Fields)
			},
		},
	}
}

// fixtureDocDBClusters returns sanitized DocumentDB cluster data for testing.
// Source: sanitized from real AWS data (2 clusters).
func fixtureDocDBClusters() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "test-docdb-cluster",
			Name:   "test-docdb-cluster",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":     "test-docdb-cluster",
				"engine_version": "5.0.0",
				"status":         "available",
				"instances":      "1",
				"endpoint":       "test-docdb-cluster.cluster-abc123def.us-east-1.docdb.amazonaws.com",
			},
			RawStruct: docdbtypes.DBCluster{
				DBClusterIdentifier: aws.String("test-docdb-cluster"),
				EngineVersion:       aws.String("5.0.0"),
				Status:              aws.String("available"),
				Endpoint:            aws.String("test-docdb-cluster.cluster-abc123def.us-east-1.docdb.amazonaws.com"),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("test-docdb-instance-1"), IsClusterWriter: aws.Bool(true)},
				},
			},
		},
		{
			ID:     "test-rds-cluster",
			Name:   "test-rds-cluster",
			Status: "available",
			Fields: map[string]string{
				"cluster_id":     "test-rds-cluster",
				"engine_version": "16.8",
				"status":         "available",
				"instances":      "1",
				"endpoint":       "test-rds-cluster.cluster-abc123def.us-east-1.rds.amazonaws.com",
			},
			RawStruct: docdbtypes.DBCluster{
				DBClusterIdentifier: aws.String("test-rds-cluster"),
				EngineVersion:       aws.String("16.8"),
				Status:              aws.String("available"),
				Endpoint:            aws.String("test-rds-cluster.cluster-abc123def.us-east-1.rds.amazonaws.com"),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("test-rds-instance-1"), IsClusterWriter: aws.Bool(true)},
				},
			},
		},
	}
}

// fixtureEKSClusters returns sanitized EKS cluster data for testing.
// Source: sanitized from real AWS data (1 cluster).
func fixtureEKSClusters() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "test-cluster-1",
			Name:   "test-cluster-1",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name":     "test-cluster-1",
				"version":          "1.31",
				"status":           "ACTIVE",
				"endpoint":         "https://ABCDEF0123456789ABCDEF0123456789.gr7.us-east-1.eks.amazonaws.com",
				"platform_version": "eks.52",
			},
		},
	}
}

// fixtureSecrets returns sanitized Secrets Manager data for testing.
// Source: sanitized from real AWS data (5 secrets shown).
func fixtureSecrets() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "test/integration",
			Name:   "test/integration",
			Status: "",
			Fields: map[string]string{
				"secret_name":      "test/integration",
				"description":      "",
				"last_accessed":    "2025-12-08",
				"last_changed":     "2025-04-17",
				"rotation_enabled": "No",
			},
		},
		{
			ID:     "test/github-app",
			Name:   "test/github-app",
			Status: "",
			Fields: map[string]string{
				"secret_name":      "test/github-app",
				"description":      "",
				"last_accessed":    "2025-12-03",
				"last_changed":     "2025-04-24",
				"rotation_enabled": "No",
			},
		},
		{
			ID:     "test/docdb-credentials",
			Name:   "test/docdb-credentials",
			Status: "",
			Fields: map[string]string{
				"secret_name":      "test/docdb-credentials",
				"description":      "",
				"last_accessed":    "2025-10-23",
				"last_changed":     "2025-05-15",
				"rotation_enabled": "No",
			},
		},
		{
			ID:     "test/redis-credentials",
			Name:   "test/redis-credentials",
			Status: "",
			Fields: map[string]string{
				"secret_name":      "test/redis-credentials",
				"description":      "",
				"last_accessed":    "2026-03-18",
				"last_changed":     "2025-05-30",
				"rotation_enabled": "No",
			},
		},
		{
			ID:     "test/rds-credentials",
			Name:   "test/rds-credentials",
			Status: "",
			Fields: map[string]string{
				"secret_name":      "test/rds-credentials",
				"description":      "",
				"last_accessed":    "2026-03-18",
				"last_changed":     "2025-05-30",
				"rotation_enabled": "No",
			},
		},
	}
}
