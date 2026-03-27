package demo

// fixtures_expansion.go provides helper data used by the fixture functions
// to expand certain resource types to 20–25 items for pagination testing.
// The actual expansion happens in each fixture function; this file only
// holds lookup tables and constants to keep the generators concise.

// ec2NamePool provides realistic instance names for generated EC2 instances.
var ec2NamePool = []string{
	"cache-node", "queue-worker", "scheduler", "metrics-collector",
	"gateway-proxy", "auth-service", "search-indexer", "notification-svc",
	"payment-processor", "inventory-sync", "report-generator", "etl-worker",
	"log-shipper", "config-server", "health-checker",
}

// ec2StatePool cycles through realistic EC2 states for generated items.
var ec2StatePool = []string{
	"running", "running", "running", "running", "running",
	"running", "running", "stopped", "stopped", "running",
	"running", "running", "running", "stopped", "running",
}

// lambdaNamePool provides realistic function names for generated Lambda functions.
var lambdaNamePool = []string{
	"user-signup-handler", "inventory-sync", "pdf-generator",
	"email-sender", "cache-warmer", "db-migrator",
	"report-scheduler", "webhook-processor", "file-cleanup",
	"audit-logger", "config-validator", "health-monitor",
	"rate-limiter", "token-refresher", "data-exporter",
	"schema-validator", "event-router", "log-archiver",
	"metric-aggregator",
}

// lambdaRuntimePool cycles through Lambda runtimes for generated items.
var lambdaRuntimePool = []string{
	"nodejs20.x", "python3.12", "go1.x", "java21", "nodejs20.x",
	"python3.12", "nodejs20.x", "go1.x", "python3.12", "java21",
	"nodejs20.x", "python3.12", "go1.x", "nodejs20.x", "python3.12",
	"java21", "nodejs20.x", "python3.12", "go1.x",
}

// lambdaHandlerPool matches runtime patterns for handlers.
var lambdaHandlerPool = []string{
	"index.handler", "handler.lambda_handler", "main",
	"com.example.Handler::handleRequest", "index.handler",
	"app.lambda_handler", "handler.handler", "main",
	"process.lambda_handler", "com.example.Processor::handle",
	"index.handler", "checker.lambda_handler", "main",
	"index.handler", "export.lambda_handler",
	"com.example.Validator::handle", "index.handler",
	"archive.lambda_handler", "main",
}

// rdsEnginePool provides realistic RDS engine combos for generated instances.
var rdsEnginePool = []struct {
	Engine        string
	EngineVersion string
	Port          int32
}{
	{"aurora-postgresql", "16.4", 5432},
	{"postgres", "16.2", 5432},
	{"mysql", "8.0.36", 3306},
	{"aurora-mysql", "3.07.0", 3306},
	{"aurora-postgresql", "16.4", 5432},
	{"postgres", "15.7", 5432},
	{"mysql", "8.0.36", 3306},
	{"aurora-postgresql", "16.4", 5432},
	{"postgres", "16.2", 5432},
	{"aurora-mysql", "3.07.0", 3306},
	{"postgres", "16.2", 5432},
	{"aurora-postgresql", "16.4", 5432},
	{"mysql", "8.0.36", 3306},
	{"postgres", "15.7", 5432},
	{"aurora-postgresql", "16.4", 5432},
	{"aurora-mysql", "3.07.0", 3306},
	{"postgres", "16.2", 5432},
}

// rdsClassPool provides realistic instance classes.
var rdsClassPool = []string{
	"db.r6g.large", "db.r6g.xlarge", "db.t3.medium", "db.m6g.large",
	"db.r6g.2xlarge", "db.t3.large", "db.m6g.xlarge", "db.r6g.large",
	"db.t3.medium", "db.m6g.large", "db.r6g.xlarge", "db.t3.large",
	"db.m6g.2xlarge", "db.r6g.large", "db.t3.medium", "db.m6g.large",
	"db.r6g.xlarge",
}

// rdsNamePool provides realistic DB instance names.
var rdsNamePool = []string{
	"user-service-db", "reporting-replica", "inventory-primary",
	"auth-db-primary", "payment-ledger", "catalog-read-replica",
	"event-store", "metadata-db", "notification-db",
	"search-backend", "billing-primary", "audit-replica",
	"workflow-db", "cache-db-primary", "integration-test-db",
	"analytics-replica", "migration-temp",
}

// sgNamePool provides realistic security group names.
var sgNamePool = []string{
	"eks-node-sg", "redis-access-sg", "lambda-vpc-sg",
	"monitoring-sg", "jenkins-sg", "kafka-broker-sg",
	"elasticsearch-sg", "ecs-tasks-sg", "nat-gateway-sg",
	"vpn-endpoint-sg", "grpc-service-sg", "graphql-api-sg",
	"batch-processing-sg", "data-pipeline-sg", "backup-sg",
	"admin-tools-sg", "dev-default-sg", "test-workload-sg",
	"canary-sg", "blue-green-sg",
}

// sgDescriptionPool matches sgNamePool with descriptions.
var sgDescriptionPool = []string{
	"EKS worker node security group",
	"Redis cluster access from app tier",
	"Lambda functions in VPC",
	"Monitoring stack (Prometheus/Grafana)",
	"Jenkins CI/CD server",
	"Kafka broker inter-node communication",
	"OpenSearch cluster access",
	"ECS Fargate task networking",
	"NAT Gateway outbound traffic",
	"VPN client endpoint",
	"gRPC microservice mesh",
	"GraphQL API gateway",
	"Batch processing workers",
	"Data pipeline ETL access",
	"Backup infrastructure access",
	"Admin and management tools",
	"Dev environment default SG",
	"Test workload isolation",
	"Canary deployment testing",
	"Blue-green deployment cutover",
}

// roleNamePool provides realistic IAM role names.
var roleNamePool = []string{
	"ecs-task-execution", "codepipeline-deploy", "glue-crawler-role",
	"step-functions-exec", "cloudwatch-events", "api-gateway-role",
	"redshift-spectrum", "emr-service-role", "backup-service-role",
	"config-service-role", "s3-replication-role", "kinesis-firehose",
	"sso-admin-role", "org-access-role", "audit-reader-role",
	"dlm-lifecycle-role", "cloudformation-exec", "batch-service-role",
	"sagemaker-execution", "transit-gw-role", "vpc-flow-logs-role",
}

// roleDescPool matches roleNamePool with descriptions.
var roleDescPool = []string{
	"ECS task execution role for ECR pull and logging",
	"CodePipeline deployment role",
	"Glue crawler and ETL job execution role",
	"Step Functions state machine execution role",
	"CloudWatch Events rule execution role",
	"API Gateway CloudWatch logging role",
	"Redshift Spectrum S3 access role",
	"EMR service-linked role",
	"AWS Backup service role",
	"AWS Config recording role",
	"S3 cross-region replication role",
	"Kinesis Firehose delivery role",
	"SSO administrator role",
	"Organization cross-account access role",
	"Security audit read-only access role",
	"Data Lifecycle Manager execution role",
	"CloudFormation stack execution role",
	"AWS Batch service role",
	"SageMaker notebook execution role",
	"Transit Gateway peering role",
	"VPC Flow Logs publishing role",
}

// alarmNamePool provides realistic alarm names.
var alarmNamePool = []string{
	"sqs-queue-depth-high", "ecs-cpu-high", "ecs-memory-high",
	"nat-error-packets", "lambda-throttles", "rds-free-storage-low",
	"redis-evictions-high", "s3-4xx-errors", "api-latency-p99",
	"kinesis-iterator-age", "dynamodb-throttle", "cloudfront-5xx-high",
	"nlb-unhealthy-hosts", "ebs-burst-balance-low",
	"billing-threshold-100", "sns-delivery-failures", "sqs-dlq-not-empty",
}

// alarmMetricPool matches alarmNamePool with metric/namespace pairs.
var alarmMetricPool = []struct {
	MetricName string
	Namespace  string
	Threshold  float64
	State      string
}{
	{"ApproximateNumberOfMessagesVisible", "AWS/SQS", 1000, "OK"},
	{"CPUUtilization", "AWS/ECS", 85, "OK"},
	{"MemoryUtilization", "AWS/ECS", 90, "ALARM"},
	{"ErrorPortAllocation", "AWS/NATGateway", 0, "OK"},
	{"Throttles", "AWS/Lambda", 5, "OK"},
	{"FreeStorageSpace", "AWS/RDS", 5368709120, "OK"},
	{"Evictions", "AWS/ElastiCache", 100, "ALARM"},
	{"4xxErrorRate", "AWS/S3", 5, "OK"},
	{"Latency", "AWS/ApiGateway", 1.0, "OK"},
	{"GetRecords.IteratorAgeMilliseconds", "AWS/Kinesis", 60000, "INSUFFICIENT_DATA"},
	{"ThrottledRequests", "AWS/DynamoDB", 10, "OK"},
	{"5xxErrorRate", "AWS/CloudFront", 1.0, "OK"},
	{"UnHealthyHostCount", "AWS/NetworkELB", 1, "OK"},
	{"BurstBalance", "AWS/EBS", 20, "OK"},
	{"EstimatedCharges", "AWS/Billing", 100, "OK"},
	{"NumberOfNotificationsFailed", "AWS/SNS", 1, "INSUFFICIENT_DATA"},
	{"ApproximateNumberOfMessagesVisible", "AWS/SQS", 1, "ALARM"},
}

// logGroupNamePool provides realistic log group names.
var logGroupNamePool = []string{
	"/aws/lambda/data-pipeline-transform",
	"/aws/lambda/order-processor",
	"/aws/lambda/payment-webhook",
	"/aws/ecs/acme-services/api-gateway",
	"/aws/ecs/acme-services/web-frontend",
	"/aws/ecs/acme-batch/etl-runner",
	"/aws/rds/instance/analytics-warehouse/postgresql",
	"/acme/application/worker",
	"/acme/application/scheduler",
	"/aws/codebuild/acme-api-build",
	"/aws/apigateway/acme-rest-api",
	"/aws/vpn/connection-logs",
	"/aws/route53/hosted-zone-queries",
	"/aws/waf/acme-prod-api-waf",
	"/acme/application/frontend",
	"/aws/codepipeline/acme-api-pipeline",
	"/aws/stepfunctions/order-workflow",
}

// sqsNamePool provides realistic SQS queue names.
var sqsNamePool = []string{
	"payment-processing-queue", "user-events-queue", "image-resize-queue",
	"analytics-pipeline-queue", "audit-log-queue", "notification-dispatch",
	"search-index-queue", "report-generation-queue", "cache-invalidation-queue",
	"inventory-update-queue", "email-bounce-queue", "dead-letter-global",
	"webhook-retry-queue", "file-upload-queue", "export-request-queue",
	"billing-events-queue", "deploy-notification-queue", "health-check-results",
}

// snsNamePool provides realistic SNS topic names.
var snsNamePool = []string{
	"billing-alerts", "security-findings", "deployment-events",
	"user-signup-events", "inventory-changes", "payment-confirmations",
	"system-health-alerts", "audit-trail-events", "config-change-notifications",
	"error-aggregation", "compliance-alerts", "infra-scaling-events",
	"release-notifications", "incident-updates", "backup-completion",
	"cost-anomaly-alerts", "certificate-expiry", "pipeline-status",
	"service-discovery",
}

// cfnNamePool provides realistic CloudFormation stack names.
var cfnNamePool = []string{
	"acme-ecs-services", "acme-lambda-functions", "acme-api-gateway",
	"acme-redis-cluster", "acme-dynamodb-tables", "acme-cloudfront",
	"acme-codepipeline", "acme-iam-roles", "acme-waf-rules",
	"acme-route53-records", "acme-sns-topics", "acme-sqs-queues",
	"acme-cloudwatch-alarms", "acme-s3-buckets", "acme-secrets",
	"acme-backup-plans", "acme-step-functions", "acme-kinesis-streams",
}

// cfnStatusPool cycles through realistic CFN stack statuses.
var cfnStatusPool = []string{
	"CREATE_COMPLETE", "CREATE_COMPLETE", "UPDATE_COMPLETE",
	"CREATE_COMPLETE", "UPDATE_COMPLETE", "CREATE_COMPLETE",
	"UPDATE_COMPLETE", "CREATE_COMPLETE", "CREATE_COMPLETE",
	"UPDATE_COMPLETE", "CREATE_COMPLETE", "CREATE_COMPLETE",
	"CREATE_COMPLETE", "UPDATE_COMPLETE", "CREATE_COMPLETE",
	"CREATE_COMPLETE", "UPDATE_COMPLETE", "CREATE_COMPLETE",
}

// elbNamePool provides realistic ELB names.
var elbNamePool = []string{
	"acme-grpc-api", "acme-websocket-lb", "acme-admin-panel",
	"acme-docs-site", "acme-monitoring-lb", "acme-staging-api",
	"acme-canary-lb", "acme-partner-api", "acme-graphql-lb",
	"acme-static-assets", "acme-webhook-lb", "acme-payment-lb",
	"acme-search-lb", "acme-batch-api", "acme-reporting-lb",
	"acme-mobile-api", "acme-analytics-lb", "acme-auth-lb",
}

// tgNamePool provides realistic target group names.
var tgNamePool = []string{
	"acme-auth-tg", "acme-search-tg", "acme-payment-tg",
	"acme-batch-tg", "acme-monitoring-tg", "acme-webhook-tg",
	"acme-admin-tg", "acme-docs-tg", "acme-static-tg",
	"acme-graphql-tg", "acme-mobile-tg", "acme-analytics-tg",
	"acme-partner-tg", "acme-staging-api-tg", "acme-canary-tg",
	"acme-reporting-tg", "acme-ws-tg", "acme-grpc-backend-tg",
}

// subnetNamePool provides realistic subnet names.
var subnetNamePool = []string{
	"prod-data-1a", "prod-data-1b", "prod-cache-1a",
	"prod-cache-1b", "staging-private-1a", "staging-private-1b",
	"dev-public-1a", "dev-private-1a", "prod-eks-1a",
	"prod-eks-1b", "prod-eks-1c", "prod-lambda-1a",
	"prod-lambda-1b", "staging-public-1a", "staging-public-1b",
	"dev-public-1b", "prod-tgw-attach-1a",
}

// policyNamePool provides realistic IAM policy names.
var policyNamePool = []string{
	"acme-ecr-push", "acme-dynamodb-access", "acme-lambda-invoke",
	"acme-sqs-consumer", "acme-sns-publish", "acme-kms-decrypt",
	"acme-ses-send", "acme-ecs-task-execution", "acme-step-functions-exec",
	"acme-glue-job-runner", "acme-kinesis-consumer", "acme-backup-operator",
	"acme-config-reader", "acme-ssm-parameter-read", "acme-vpc-flow-logs",
	"acme-codebuild-access", "acme-api-gateway-invoke", "acme-redshift-access",
}

// ssmNamePool provides realistic SSM parameter names.
var ssmNamePool = []string{
	"/acme/prod/redis/endpoint", "/acme/prod/api/base-url",
	"/acme/prod/jwt-secret", "/acme/staging/app/config",
	"/acme/prod/third-party/api-key", "/acme/prod/db/read-endpoint",
	"/acme/staging/feature-flags", "/acme/prod/cdn/distribution-id",
	"/acme/prod/smtp/credentials", "/acme/staging/db/connection-string",
	"/acme/prod/oauth/client-id", "/acme/prod/oauth/client-secret",
	"/acme/prod/monitoring/datadog-key", "/acme/staging/redis/endpoint",
	"/acme/prod/ecs/cluster-name", "/acme/prod/backup/schedule",
	"/acme/prod/waf/ip-whitelist", "/acme/staging/jwt-secret",
}

// ssmTypePool cycles through SSM parameter types.
var ssmTypePool = []string{
	"String", "String", "SecureString", "String",
	"SecureString", "String", "StringList", "String",
	"SecureString", "SecureString", "String", "SecureString",
	"SecureString", "String", "String", "String",
	"StringList", "SecureString",
}

// secretNamePool provides realistic secrets manager secret names.
var secretNamePool = []string{
	"prod/api/twilio-key", "prod/oauth/github-secret",
	"prod/database/read-replica", "staging/api/stripe-key",
	"prod/monitoring/pagerduty-key", "prod/smtp/sendgrid-key",
	"prod/cache/redis-password", "staging/oauth/google-secret",
	"prod/api/datadog-api-key", "prod/database/analytics-creds",
	"prod/third-party/jira-token", "staging/database/postgres",
	"prod/jwt/signing-key", "prod/api/cloudflare-token",
	"prod/mq/rabbitmq-creds", "staging/cache/redis-auth",
	"prod/api/newrelic-key", "prod/ssh/deploy-key",
}

// secretDescPool matches secretNamePool with descriptions.
var secretDescPool = []string{
	"Twilio API credentials for SMS",
	"GitHub OAuth client secret",
	"Read replica database credentials",
	"Staging Stripe API key",
	"PagerDuty integration key",
	"SendGrid SMTP API key",
	"Redis AUTH password for production cache",
	"Google OAuth client secret for staging",
	"Datadog API key for monitoring",
	"Analytics database connection credentials",
	"JIRA API token for issue tracking",
	"Staging PostgreSQL connection string",
	"JWT token signing key",
	"Cloudflare API token for DNS management",
	"RabbitMQ broker credentials",
	"Staging Redis AUTH token",
	"New Relic APM license key",
	"SSH deploy key for CI/CD",
}

// ddbNamePool provides realistic DynamoDB table names.
var ddbNamePool = []string{
	"acme-user-profiles", "acme-product-catalog", "acme-notifications",
	"acme-rate-limits", "acme-feature-flags", "acme-api-keys",
	"acme-webhooks", "acme-cache-metadata", "acme-job-queue",
	"acme-config-store", "acme-user-preferences", "acme-analytics-events",
	"acme-auth-tokens", "acme-file-metadata", "acme-search-index",
	"acme-payment-intents", "acme-deployment-history", "acme-health-checks",
}

// s3NamePool provides realistic S3 bucket names.
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

// ecsServiceNamePool provides realistic ECS service names.
var ecsServiceNamePool = []string{
	"auth-service", "user-profile-svc", "notification-worker",
	"payment-processor", "search-indexer", "recommendation-engine",
	"analytics-ingester", "email-sender", "image-processor",
	"cache-warmer", "data-sync", "audit-logger",
	"rate-limiter", "webhook-dispatcher", "report-generator",
	"health-aggregator", "config-reloader",
}
