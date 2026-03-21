// Package demo provides synthetic fixture data for demo mode.
// When a9s is launched with --demo, these fixtures replace real AWS API calls,
// allowing the full TUI to run without AWS credentials.
package demo

import "github.com/k2m30/a9s/internal/resource"

// DemoRegion is the synthetic region displayed in demo mode.
const DemoRegion = "us-east-1"

// DemoProfile is the synthetic profile displayed in demo mode.
const DemoProfile = "demo"

// demoData maps resource short names to fixture generator functions.
// Each call returns a fresh slice (no shared global state).
var demoData = map[string]func() []resource.Resource{
	"s3":     s3Buckets,
	"lambda": lambdaFunctions,
	"dbi":    rdsInstances,
	// "ec2" is added in fixtures_ec2.go init()
}

// GetResources returns fixture data for the given resource type.
// The resourceType should be the canonical short name (e.g., "ec2", "dbi").
// Returns nil, false for resource types without demo data.
func GetResources(resourceType string) ([]resource.Resource, bool) {
	gen, ok := demoData[resourceType]
	if !ok {
		return nil, false
	}
	return gen(), true
}

// GetS3Objects returns fixture data for S3 objects within a bucket.
// Returns nil, false if the bucket is not in demo data.
func GetS3Objects(bucket, prefix string) ([]resource.Resource, bool) {
	if bucket == "data-pipeline-logs" {
		return s3Objects(), true
	}
	return nil, false
}

// s3Buckets returns demo S3 bucket fixtures.
func s3Buckets() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "data-pipeline-logs",
			Name:   "data-pipeline-logs",
			Status: "",
			Fields: map[string]string{
				"name":          "data-pipeline-logs",
				"bucket_name":   "data-pipeline-logs",
				"creation_date": "2025-01-15T09:23:41+00:00",
			},
		},
		{
			ID:     "webapp-assets-prod",
			Name:   "webapp-assets-prod",
			Status: "",
			Fields: map[string]string{
				"name":          "webapp-assets-prod",
				"bucket_name":   "webapp-assets-prod",
				"creation_date": "2025-03-22T14:07:19+00:00",
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
		},
	}
}

// s3Objects returns demo S3 objects for the "data-pipeline-logs" bucket.
func s3Objects() []resource.Resource {
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
		},
	}
}

// lambdaFunctions returns demo Lambda function fixtures.
func lambdaFunctions() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "api-gateway-authorizer",
			Name:   "api-gateway-authorizer",
			Status: "nodejs20.x",
			Fields: map[string]string{
				"function_name": "api-gateway-authorizer",
				"runtime":       "nodejs20.x",
				"memory":        "256",
				"timeout":       "10",
				"handler":       "index.handler",
				"last_modified": "2026-03-15T08:22:14+00:00",
				"code_size":     "1048576",
			},
		},
		{
			ID:     "data-pipeline-transform",
			Name:   "data-pipeline-transform",
			Status: "python3.12",
			Fields: map[string]string{
				"function_name": "data-pipeline-transform",
				"runtime":       "python3.12",
				"memory":        "512",
				"timeout":       "300",
				"handler":       "transform.lambda_handler",
				"last_modified": "2026-03-10T16:45:33+00:00",
				"code_size":     "5242880",
			},
		},
		{
			ID:     "order-processor",
			Name:   "order-processor",
			Status: "go1.x",
			Fields: map[string]string{
				"function_name": "order-processor",
				"runtime":       "go1.x",
				"memory":        "128",
				"timeout":       "30",
				"handler":       "main",
				"last_modified": "2026-02-28T11:03:47+00:00",
				"code_size":     "8388608",
			},
		},
		{
			ID:     "image-thumbnail-gen",
			Name:   "image-thumbnail-gen",
			Status: "python3.12",
			Fields: map[string]string{
				"function_name": "image-thumbnail-gen",
				"runtime":       "python3.12",
				"memory":        "1024",
				"timeout":       "60",
				"handler":       "thumbnail.handler",
				"last_modified": "2026-03-01T09:18:55+00:00",
				"code_size":     "15728640",
			},
		},
		{
			ID:     "payment-webhook",
			Name:   "payment-webhook",
			Status: "java21",
			Fields: map[string]string{
				"function_name": "payment-webhook",
				"runtime":       "java21",
				"memory":        "512",
				"timeout":       "15",
				"handler":       "com.example.PaymentHandler::handleRequest",
				"last_modified": "2026-03-12T20:11:09+00:00",
				"code_size":     "31457280",
			},
		},
		{
			ID:     "cloudwatch-slack-notifier",
			Name:   "cloudwatch-slack-notifier",
			Status: "nodejs20.x",
			Fields: map[string]string{
				"function_name": "cloudwatch-slack-notifier",
				"runtime":       "nodejs20.x",
				"memory":        "128",
				"timeout":       "10",
				"handler":       "notify.handler",
				"last_modified": "2026-01-20T13:42:00+00:00",
				"code_size":     "524288",
			},
		},
	}
}

// rdsInstances returns demo RDS (DB Instance) fixtures.
func rdsInstances() []resource.Resource {
	return []resource.Resource{
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
		},
	}
}
