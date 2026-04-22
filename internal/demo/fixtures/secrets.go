package fixtures

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// SecretsFixtures holds typed fixture data for Secrets Manager.
type SecretsFixtures struct {
	Secrets []smtypes.SecretListEntry
	// SecretValues maps secret name to plaintext value (for GetSecretValue).
	SecretValues map[string]string
}

var secretNamePool = []string{
	"prod/app/jwt-secret", "prod/app/oauth-client-secret", "prod/elk/elasticsearch-password",
	"prod/kafka/sasl-password", "prod/monitoring/grafana-admin", "prod/app/sendgrid-api-key",
	"prod/app/twilio-auth-token", "prod/app/github-webhook-secret", "prod/rds/replica-password",
	"prod/app/datadog-api-key", "staging/database/postgres", "staging/app/jwt-secret",
	"staging/app/oauth-client-secret", "dev/database/postgres", "dev/app/jwt-secret",
	"shared/app/encryption-key", "shared/monitoring/pagerduty-key", "prod/app/slack-webhook",
}

var secretDescPool = []string{
	"JWT signing secret", "OAuth2 client secret", "Elasticsearch admin password",
	"Kafka SASL password", "Grafana admin password", "SendGrid API key",
	"Twilio auth token", "GitHub webhook secret", "RDS read replica password",
	"Datadog API key", "Staging PostgreSQL credentials", "Staging JWT secret",
	"Staging OAuth2 client secret", "Dev PostgreSQL credentials", "Dev JWT secret",
	"Shared encryption key", "PagerDuty integration key", "Slack webhook URL",
}

// NewSecretsFixtures constructs SecretsFixtures from the canonical demo data.
func NewSecretsFixtures() *SecretsFixtures {
	secrets := []smtypes.SecretListEntry{
		// RDS-managed secret for prod-dbi-1 — required for dbi→secrets related-panel pivot.
		// ARN matches DBIFixtures.ProdDbiMasterSecretARN (MasterUserSecret.SecretArn on prod-dbi-1).
		{
			Name:             aws.String("rds!db-prod-dbi-1-ABCDEF"),
			ARN:              aws.String(ProdDbiMasterSecretARN),
			Description:      aws.String("RDS-managed master user password for prod-dbi-1"),
			LastAccessedDate: aws.Time(time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)),
			LastChangedDate:  aws.Time(time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)),
			RotationEnabled:  aws.Bool(true),
			CreatedDate:      aws.Time(time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC)),
			KmsKeyId:         aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			RotationRules:    &smtypes.RotationRulesType{AutomaticallyAfterDays: aws.Int64(7)},
			Tags:             []smtypes.Tag{{Key: aws.String("aws:rds:primaryDBInstanceArn"), Value: aws.String(ProdDbiARN)}},
		},
		{
			Name:              aws.String("prod/docdb/acme-docdb-prod"),
			ARN:               aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/docdb/acme-docdb-prod-XyZaBc"),
			Description:       aws.String("DocumentDB cluster credentials for acme-docdb-prod"),
			LastAccessedDate:  aws.Time(time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)),
			LastChangedDate:   aws.Time(time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)),
			RotationEnabled:   aws.Bool(true),
			CreatedDate:       aws.Time(time.Date(2025, 2, 5, 9, 0, 0, 0, time.UTC)),
			KmsKeyId:          aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			LastRotatedDate:   aws.Time(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)),
			RotationLambdaARN: aws.String("arn:aws:lambda:us-east-1:123456789012:function:rotate-docdb-credentials"),
			PrimaryRegion:     aws.String("us-east-1"),
			RotationRules:     &smtypes.RotationRulesType{AutomaticallyAfterDays: aws.Int64(30)},
			Tags:              []smtypes.Tag{{Key: aws.String("Environment"), Value: aws.String("production")}},
		},
		{
			Name:              aws.String("prod/database/primary"),
			ARN:               aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"),
			Description:       aws.String("Aurora PostgreSQL primary connection string"),
			LastAccessedDate:  aws.Time(time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)),
			LastChangedDate:   aws.Time(time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)),
			RotationEnabled:   aws.Bool(true),
			CreatedDate:       aws.Time(time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC)),
			KmsKeyId:          aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			LastRotatedDate:   aws.Time(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)),
			PrimaryRegion:     aws.String("us-east-1"),
			RotationLambdaARN: aws.String("arn:aws:lambda:us-east-1:123456789012:function:rotate-rds-credentials"),
			RotationRules:     &smtypes.RotationRulesType{AutomaticallyAfterDays: aws.Int64(30)},
			Tags:              []smtypes.Tag{{Key: aws.String("Environment"), Value: aws.String("production")}},
		},
		{
			Name:             aws.String("prod/api/stripe-key"),
			ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/stripe-key-GhIjKl"),
			Description:      aws.String("Stripe API secret key for payment processing"),
			LastAccessedDate: aws.Time(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)),
			LastChangedDate:  aws.Time(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)),
			RotationEnabled:  aws.Bool(false),
			CreatedDate:      aws.Time(time.Date(2025, 4, 22, 14, 30, 0, 0, time.UTC)),
		},
		{
			Name:             aws.String("prod/redis/auth-token"),
			ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/redis/auth-token-MnOpQr"),
			Description:      aws.String("ElastiCache Redis AUTH token"),
			LastAccessedDate: aws.Time(time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)),
			LastChangedDate:  aws.Time(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)),
			RotationEnabled:  aws.Bool(true),
			CreatedDate:      aws.Time(time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)),
		},
		{
			Name:             aws.String("staging/database/mysql"),
			ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:staging/database/mysql-StUvWx"),
			Description:      aws.String("Staging MySQL connection credentials"),
			LastAccessedDate: aws.Time(time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)),
			LastChangedDate:  aws.Time(time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)),
			RotationEnabled:  aws.Bool(false),
			CreatedDate:      aws.Time(time.Date(2025, 3, 15, 9, 0, 0, 0, time.UTC)),
		},
		// Issue: RotationEnabled=true, LastRotatedDate=2025-09-01 (>2×30d=60d ago) → Broken (rotation failing)
		{
			Name:              aws.String("prod/app/rotation-broken"),
			ARN:               aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/app/rotation-broken-YzAbCd"),
			Description:       aws.String("API key with broken automatic rotation"),
			LastAccessedDate:  aws.Time(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
			LastChangedDate:   aws.Time(time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)),
			RotationEnabled:   aws.Bool(true),
			LastRotatedDate:   aws.Time(time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)),
			RotationLambdaARN: aws.String("arn:aws:lambda:us-east-1:123456789012:function:rotate-api-key"),
			RotationRules:     &smtypes.RotationRulesType{AutomaticallyAfterDays: aws.Int64(30)},
			CreatedDate:       aws.Time(time.Date(2024, 8, 15, 10, 0, 0, 0, time.UTC)),
			Tags:              []smtypes.Tag{{Key: aws.String("Environment"), Value: aws.String("production")}},
		},
		// Issue: DeletedDate set → Warning (pending deletion, restore window)
		{
			Name:             aws.String("dev/deprecated/old-webhook-key"),
			ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:dev/deprecated/old-webhook-key-EfGhIj"),
			Description:      aws.String("Deprecated webhook key scheduled for deletion"),
			LastAccessedDate: aws.Time(time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)),
			LastChangedDate:  aws.Time(time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)),
			RotationEnabled:  aws.Bool(false),
			CreatedDate:      aws.Time(time.Date(2024, 3, 1, 8, 0, 0, 0, time.UTC)),
			DeletedDate:      aws.Time(time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)),
		},
	}

	for i := range 18 {
		name := secretNamePool[i]
		desc := secretDescPool[i]
		rotation := i%3 == 0
		lastAccessed := time.Date(2026, 3, 15+i%7, 0, 0, 0, 0, time.UTC)
		lastChanged := time.Date(2026, time.Month(1+i%3), 1+i, 0, 0, 0, 0, time.UTC)
		created := time.Date(2025, time.Month(1+i%12), 1+i, 10, 0, 0, 0, time.UTC)
		suffix := fmt.Sprintf("%06x", i+1000)
		secrets = append(secrets, smtypes.SecretListEntry{
			Name:             aws.String(name),
			ARN:              aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s-%s", name, suffix)),
			Description:      aws.String(desc),
			LastAccessedDate: aws.Time(lastAccessed),
			LastChangedDate:  aws.Time(lastChanged),
			RotationEnabled:  aws.Bool(rotation),
			CreatedDate:      aws.Time(created),
		})
	}

	return &SecretsFixtures{
		Secrets: secrets,
		SecretValues: map[string]string{
			"prod/docdb/acme-docdb-prod": `{"username":"admin","password":"[REDACTED]"}`,
			"prod/database/primary":      `{"host":"prod-api-primary.cluster-c9xyz123.us-east-1.rds.amazonaws.com","port":"5432","username":"appuser","password":"[REDACTED]"}`,
			"prod/api/stripe-key":        `{"api_key":"[REDACTED]"}`,
			"prod/redis/auth-token":      `{"auth_token":"[REDACTED]"}`,
			"staging/database/mysql":     `{"host":"staging-mysql.c9xyz123.us-east-1.rds.amazonaws.com","port":"3306","username":"staginguser","password":"[REDACTED]"}`,
		},
	}
}
