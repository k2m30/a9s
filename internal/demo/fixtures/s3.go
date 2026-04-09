// Package fixtures provides S3 fixture data for the S3 fake.
package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Fixtures holds all S3 domain objects served by the fake.
type S3Fixtures struct {
	// Buckets is the full list of buckets returned by ListBuckets.
	Buckets []s3types.Bucket
	// NotificationConfigs maps bucket names to their notification configuration.
	// Buckets not in this map return an empty notification config.
	NotificationConfigs map[string]*s3.GetBucketNotificationConfigurationOutput
	// Objects maps bucket name → prefix → slice of S3 objects at that prefix level.
	Objects map[string]map[string][]s3types.Object
	// CommonPrefixes maps bucket name → prefix → slice of common prefixes (folders).
	CommonPrefixes map[string]map[string][]s3types.CommonPrefix
}

// mustTime parses an RFC3339 timestamp or panics.
func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("fixtures/s3: invalid time: " + s)
	}
	return t
}

// NewS3Fixtures builds and returns a fully-populated S3Fixtures struct
// with deterministic demo data matching the legacy demo code paths.
func NewS3Fixtures() *S3Fixtures {
	f := &S3Fixtures{
		NotificationConfigs: buildS3NotificationConfigs(),
		Objects:             buildS3Objects(),
		CommonPrefixes:      buildS3CommonPrefixes(),
	}
	f.Buckets = buildS3Buckets()
	return f
}

// ---------------------------------------------------------------------------
// Bucket list
// ---------------------------------------------------------------------------

var s3NamePool = []string{
	"acme-logs-archive", "acme-static-assets-staging",
	"acme-data-lake-raw", "acme-etl-temp",
	"acme-model-artifacts", "acme-config-backup",
	"acme-user-uploads-prod", "acme-reports-output",
	"acme-compliance-audit", "acme-container-images-cache",
	"acme-disaster-recovery", "acme-api-docs-static",
	"acme-lambda-artifacts", "acme-athena-results",
	"acme-redshift-unload", "acme-cdn-origin",
}

func buildS3Buckets() []s3types.Bucket {
	// Named buckets with objects
	namedBuckets := []struct {
		name, arn, region, created string
	}{
		{"data-pipeline-logs", "arn:aws:s3:::data-pipeline-logs", "us-east-1", "2025-01-15T09:23:41+00:00"},
		{"webapp-assets-prod", "arn:aws:s3:::webapp-assets-prod", "us-east-1", "2025-03-22T14:07:19+00:00"},
		{"ml-training-data", "arn:aws:s3:::ml-training-data", "us-east-1", "2025-06-10T08:45:33+00:00"},
		{"terraform-state-prod", "arn:aws:s3:::terraform-state-prod", "us-east-1", "2024-11-02T16:30:12+00:00"},
		{"cloudtrail-audit-logs", "arn:aws:s3:::cloudtrail-audit-logs", "us-east-1", "2024-08-19T11:12:05+00:00"},
		{"backup-db-snapshots", "arn:aws:s3:::backup-db-snapshots", "us-east-1", "2025-09-01T07:55:28+00:00"},
	}

	// CT-event cross-reference buckets
	ctBuckets := []struct{ name, created string }{
		{"prod-logs", "2025-02-10T08:00:00+00:00"},
		{"prod-artifacts", "2025-03-15T10:30:00+00:00"},
		{"checkout-config", "2025-05-20T14:00:00+00:00"},
		{"shared-artifacts", "2025-04-01T09:15:00+00:00"},
		{"prod-lake", "2025-01-25T11:45:00+00:00"},
	}

	var buckets []s3types.Bucket

	for _, b := range namedBuckets {
		buckets = append(buckets, s3types.Bucket{
			Name:         aws.String(b.name),
			BucketArn:    aws.String(b.arn),
			BucketRegion: aws.String(b.region),
			CreationDate: aws.Time(mustTime(b.created)),
		})
	}

	for _, b := range ctBuckets {
		buckets = append(buckets, s3types.Bucket{
			Name:         aws.String(b.name),
			BucketArn:    aws.String("arn:aws:s3:::" + b.name),
			BucketRegion: aws.String("us-east-1"),
			CreationDate: aws.Time(mustTime(b.created)),
		})
	}

	// Generated buckets to reach 22+ total
	for i, name := range s3NamePool {
		createDate := time.Date(2025, time.Month(1+(i%12)), 1+i, 8+(i%12), (i*7)%60, 0, 0, time.UTC)
		buckets = append(buckets, s3types.Bucket{
			Name:         aws.String(name),
			BucketArn:    aws.String("arn:aws:s3:::" + name),
			BucketRegion: aws.String("us-east-1"),
			CreationDate: aws.Time(createDate),
		})
	}

	return buckets
}

// ---------------------------------------------------------------------------
// Notification configs (for GetBucketNotificationConfiguration)
// ---------------------------------------------------------------------------

func buildS3NotificationConfigs() map[string]*s3.GetBucketNotificationConfigurationOutput {
	return map[string]*s3.GetBucketNotificationConfigurationOutput{
		"data-pipeline-logs": {
			LambdaFunctionConfigurations: []s3types.LambdaFunctionConfiguration{
				{LambdaFunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:process-orders")},
			},
			QueueConfigurations: []s3types.QueueConfiguration{
				{QueueArn: aws.String("arn:aws:sqs:us-east-1:123456789012:order-processing-queue")},
			},
			TopicConfigurations: []s3types.TopicConfiguration{
				{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:order-events")},
			},
		},
		"webapp-assets-prod": {
			LambdaFunctionConfigurations: []s3types.LambdaFunctionConfiguration{
				{LambdaFunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:image-thumbnail-gen")},
			},
			QueueConfigurations: []s3types.QueueConfiguration{
				{QueueArn: aws.String("arn:aws:sqs:us-east-1:123456789012:webhook-ingest-queue.fifo")},
			},
			TopicConfigurations: []s3types.TopicConfiguration{
				{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications")},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// S3 Objects (files) per bucket → prefix
// ---------------------------------------------------------------------------

func buildS3Objects() map[string]map[string][]s3types.Object {
	return map[string]map[string][]s3types.Object{
		"data-pipeline-logs": {
			"": {
				{Key: aws.String("config.json"), Size: aws.Int64(2458), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-18T09:15:22+00:00"))},
				{Key: aws.String("schema/pipeline-v2.avro"), Size: aws.Int64(19148), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-01-10T14:32:07+00:00"))},
				{Key: aws.String("archive/2025-q4-summary.parquet"), Size: aws.Int64(149199462), StorageClass: s3types.ObjectStorageClassGlacier, LastModified: aws.Time(mustTime("2026-01-05T03:00:00+00:00"))},
			},
			"logs/": {
				{Key: aws.String("logs/2026/"), Size: aws.Int64(0), StorageClass: s3types.ObjectStorageClassStandard},
				{Key: aws.String("logs/2025/"), Size: aws.Int64(0), StorageClass: s3types.ObjectStorageClassStandard},
			},
			"logs/2026/": {
				{Key: aws.String("logs/2026/03/"), Size: aws.Int64(0), StorageClass: s3types.ObjectStorageClassStandard},
				{Key: aws.String("logs/2026/02/"), Size: aws.Int64(0), StorageClass: s3types.ObjectStorageClassStandard},
				{Key: aws.String("logs/2026/01/"), Size: aws.Int64(0), StorageClass: s3types.ObjectStorageClassStandard},
			},
			"logs/2026/03/": {
				{Key: aws.String("logs/2026/03/access-2026-03-21.log"), Size: aws.Int64(4928307), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-21T06:00:00Z"))},
				{Key: aws.String("logs/2026/03/access-2026-03-20.log"), Size: aws.Int64(5347737), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-20T06:00:00Z"))},
				{Key: aws.String("logs/2026/03/error-2026-03-21.log"), Size: aws.Int64(131072), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-21T06:00:00Z"))},
				{Key: aws.String("logs/2026/03/error-2026-03-20.log"), Size: aws.Int64(98304), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-20T06:00:00Z"))},
			},
			"logs/2026/02/": {
				{Key: aws.String("logs/2026/02/access-2026-02-28.log"), Size: aws.Int64(4089446), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-02-28T06:00:00Z"))},
				{Key: aws.String("logs/2026/02/access-2026-02-27.log"), Size: aws.Int64(4404019), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-02-27T06:00:00Z"))},
				{Key: aws.String("logs/2026/02/error-2026-02-28.log"), Size: aws.Int64(68608), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-02-28T06:00:00Z"))},
			},
		},
		"webapp-assets-prod": {
			"": {
				{Key: aws.String("index.html"), Size: aws.Int64(12697), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-20T10:05:00+00:00"))},
				{Key: aws.String("favicon.ico"), Size: aws.Int64(4301), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-01-10T08:00:00+00:00"))},
				{Key: aws.String("robots.txt"), Size: aws.Int64(68), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2025-12-01T12:00:00+00:00"))},
			},
			"css/": {
				{Key: aws.String("css/main.css"), Size: aws.Int64(24883), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-20T10:00:00Z"))},
				{Key: aws.String("css/vendor.css"), Size: aws.Int64(159744), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-15T09:00:00Z"))},
			},
			"js/": {
				{Key: aws.String("js/app.bundle.js"), Size: aws.Int64(350208), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-20T10:00:00Z"))},
				{Key: aws.String("js/vendor.bundle.js"), Size: aws.Int64(1258291), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-15T09:00:00Z"))},
			},
			"images/": {
				{Key: aws.String("images/logo.png"), Size: aws.Int64(18841), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-01-15T12:00:00Z"))},
				{Key: aws.String("images/hero-banner.jpg"), Size: aws.Int64(250880), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-19T15:30:00Z"))},
				{Key: aws.String("images/favicon-32x32.png"), Size: aws.Int64(1434), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-01-15T12:00:00Z"))},
			},
			"2026/": {},
			"2026/04/": {},
			"2026/04/07/": {
				{Key: aws.String("2026/04/07/app.log"), Size: aws.Int64(5033164), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T22:15:09Z"))},
				{Key: aws.String("2026/04/07/error.log"), Size: aws.Int64(319488), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T22:14:55Z"))},
				{Key: aws.String("2026/04/07/access.log"), Size: aws.Int64(11744051), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T22:15:00Z"))},
			},
		},
		"ml-training-data": {
			"": {
				{Key: aws.String("config.yaml"), Size: aws.Int64(1843), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-19T16:22:00+00:00"))},
				{Key: aws.String("training-results-v3.json"), Size: aws.Int64(867328), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-18T14:30:00+00:00"))},
			},
			"datasets/": {
				{Key: aws.String("datasets/train.csv"), Size: aws.Int64(1932735283), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-15T09:00:00Z"))},
				{Key: aws.String("datasets/validation.csv"), Size: aws.Int64(471859200), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-15T09:05:00Z"))},
				{Key: aws.String("datasets/test.csv"), Size: aws.Int64(230686720), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-15T09:10:00Z"))},
			},
			"models/": {
				{Key: aws.String("models/v3-final.tar.gz"), Size: aws.Int64(935329792), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-18T14:00:00Z"))},
				{Key: aws.String("models/v2-baseline.tar.gz"), Size: aws.Int64(792723456), StorageClass: s3types.ObjectStorageClassStandardIa, LastModified: aws.Time(mustTime("2026-02-20T11:00:00Z"))},
			},
			"notebooks/": {
				{Key: aws.String("notebooks/exploration.ipynb"), Size: aws.Int64(350208), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-10T11:00:00Z"))},
				{Key: aws.String("notebooks/feature-engineering.ipynb"), Size: aws.Int64(580608), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-12T16:30:00Z"))},
			},
		},
		"terraform-state-prod": {
			"": {
				{Key: aws.String("prod/vpc.tfstate"), Size: aws.Int64(250880), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-20T08:15:00+00:00"))},
				{Key: aws.String("prod/eks.tfstate"), Size: aws.Int64(193536), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-19T22:30:00+00:00"))},
				{Key: aws.String("staging/main.tfstate"), Size: aws.Int64(319488), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-18T11:45:00+00:00"))},
			},
			"env:/": {},
		},
		"cloudtrail-audit-logs": {
			"": {
				{Key: aws.String("AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/21/event-001.json.gz"), Size: aws.Int64(55501), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-21T00:05:00+00:00"))},
				{Key: aws.String("AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/20/digest.json.gz"), Size: aws.Int64(1127), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-20T23:59:00+00:00"))},
			},
			"AWSLogs/": {
				{Key: aws.String("AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/21/event-001.json.gz"), Size: aws.Int64(55501), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-21T00:05:00+00:00"))},
				{Key: aws.String("AWSLogs/123456789012/CloudTrail/us-east-1/2026/03/20/digest.json.gz"), Size: aws.Int64(1127), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-20T23:59:00+00:00"))},
			},
		},
		"backup-db-snapshots": {
			"": {
				{Key: aws.String("rds/prod-api-primary-2026-03-20.snap"), Size: aws.Int64(2469606195), StorageClass: s3types.ObjectStorageClassStandardIa, LastModified: aws.Time(mustTime("2026-03-20T04:15:00+00:00"))},
				{Key: aws.String("rds/prod-api-primary-2026-03-19.snap"), Size: aws.Int64(2469606195), StorageClass: s3types.ObjectStorageClassGlacier, LastModified: aws.Time(mustTime("2026-03-19T04:15:00+00:00"))},
			},
			"rds/": {
				{Key: aws.String("rds/prod-api-primary-2026-03-20.snap"), Size: aws.Int64(2469606195), StorageClass: s3types.ObjectStorageClassStandardIa, LastModified: aws.Time(mustTime("2026-03-20T04:15:00Z"))},
				{Key: aws.String("rds/prod-api-primary-2026-03-19.snap"), Size: aws.Int64(2469606195), StorageClass: s3types.ObjectStorageClassGlacier, LastModified: aws.Time(mustTime("2026-03-19T04:15:00Z"))},
			},
			"docdb/": {
				{Key: aws.String("docdb/orders-cluster-2026-03-19.snap"), Size: aws.Int64(1181116006), StorageClass: s3types.ObjectStorageClassStandardIa, LastModified: aws.Time(mustTime("2026-03-19T04:30:00Z"))},
			},
		},
		"checkout-config": {
			"": {
				{Key: aws.String("README.md"), Size: aws.Int64(2150), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-01-15T08:00:00Z"))},
			},
			"prod/": {
				{Key: aws.String("prod/config.json"), Size: aws.Int64(8602), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T11:02:33Z"))},
				{Key: aws.String("prod/config.json.bak"), Size: aws.Int64(8294), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-06T10:15:00Z"))},
				{Key: aws.String("prod/schema.json"), Size: aws.Int64(3277), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-03-01T09:00:00Z"))},
			},
		},
		"shared-artifacts": {
			"": {
				{Key: aws.String("build-4821.tar.gz"), Size: aws.Int64(56963891), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T14:38:12Z"))},
				{Key: aws.String("build-4820.tar.gz"), Size: aws.Int64(56754278), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T12:15:00Z"))},
				{Key: aws.String("build-4819.tar.gz"), Size: aws.Int64(56544870), StorageClass: s3types.ObjectStorageClassStandardIa, LastModified: aws.Time(mustTime("2026-04-07T09:50:00Z"))},
				{Key: aws.String("latest.tar.gz"), Size: aws.Int64(56963891), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T14:38:20Z"))},
			},
		},
		"prod-lake": {
			"": {},
			"landing/": {
				{Key: aws.String("landing/2025/"), Size: aws.Int64(0), StorageClass: s3types.ObjectStorageClassStandard},
			},
			"landing/2026/": {
				{Key: aws.String("landing/2026/03/"), Size: aws.Int64(0), StorageClass: s3types.ObjectStorageClassStandard},
			},
			"landing/2026/04/": {
				{Key: aws.String("landing/2026/04/06/"), Size: aws.Int64(0), StorageClass: s3types.ObjectStorageClassStandard},
			},
			"landing/2026/04/07/": {
				{Key: aws.String("landing/2026/04/07/batch-0719.parquet"), Size: aws.Int64(134963814), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T19:05:44Z"))},
				{Key: aws.String("landing/2026/04/07/batch-0718.parquet"), Size: aws.Int64(137597338), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T18:05:11Z"))},
				{Key: aws.String("landing/2026/04/07/batch-0717.parquet"), Size: aws.Int64(132578918), StorageClass: s3types.ObjectStorageClassStandard, LastModified: aws.Time(mustTime("2026-04-07T17:05:02Z"))},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// S3 CommonPrefixes (folders) per bucket → prefix
// ---------------------------------------------------------------------------

func buildS3CommonPrefixes() map[string]map[string][]s3types.CommonPrefix {
	return map[string]map[string][]s3types.CommonPrefix{
		"data-pipeline-logs": {
			"": {
				{Prefix: aws.String("logs/2026/03/")},
				{Prefix: aws.String("logs/2026/02/")},
			},
			"logs/": {
				{Prefix: aws.String("logs/2026/")},
				{Prefix: aws.String("logs/2025/")},
			},
			"logs/2026/": {
				{Prefix: aws.String("logs/2026/03/")},
				{Prefix: aws.String("logs/2026/02/")},
				{Prefix: aws.String("logs/2026/01/")},
			},
		},
		"webapp-assets-prod": {
			"": {
				{Prefix: aws.String("css/")},
				{Prefix: aws.String("js/")},
				{Prefix: aws.String("images/")},
			},
			"2026/": {
				{Prefix: aws.String("2026/04/")},
				{Prefix: aws.String("2026/03/")},
			},
			"2026/04/": {
				{Prefix: aws.String("2026/04/07/")},
				{Prefix: aws.String("2026/04/06/")},
			},
		},
		"ml-training-data": {
			"": {
				{Prefix: aws.String("datasets/")},
				{Prefix: aws.String("models/")},
				{Prefix: aws.String("notebooks/")},
			},
		},
		"terraform-state-prod": {
			"": {
				{Prefix: aws.String("env:/")},
			},
			"env:/": {
				{Prefix: aws.String("env:/prod/")},
				{Prefix: aws.String("env:/staging/")},
			},
		},
		"cloudtrail-audit-logs": {
			"": {
				{Prefix: aws.String("AWSLogs/")},
			},
			"AWSLogs/": {
				{Prefix: aws.String("AWSLogs/123456789012/")},
			},
		},
		"backup-db-snapshots": {
			"": {
				{Prefix: aws.String("rds/")},
				{Prefix: aws.String("docdb/")},
			},
		},
		"checkout-config": {
			"": {
				{Prefix: aws.String("prod/")},
				{Prefix: aws.String("staging/")},
			},
		},
		"prod-lake": {
			"": {
				{Prefix: aws.String("landing/")},
				{Prefix: aws.String("processed/")},
				{Prefix: aws.String("archive/")},
			},
			"landing/": {
				{Prefix: aws.String("landing/2026/")},
				{Prefix: aws.String("landing/2025/")},
			},
			"landing/2026/": {
				{Prefix: aws.String("landing/2026/04/")},
				{Prefix: aws.String("landing/2026/03/")},
			},
			"landing/2026/04/": {
				{Prefix: aws.String("landing/2026/04/07/")},
				{Prefix: aws.String("landing/2026/04/06/")},
			},
		},
	}
}
