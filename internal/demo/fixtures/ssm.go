package fixtures

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// SSMFixtures holds typed fixture data for SSM Parameter Store.
type SSMFixtures struct {
	Parameters []ssmtypes.ParameterMetadata
	// ParameterValues maps parameter name to its current value (for GetParameter).
	ParameterValues map[string]string
}

var ssmNamePool = []string{
	"/acme/prod/app/api-url", "/acme/prod/app/max-connections", "/acme/prod/app/log-level",
	"/acme/prod/eks/cluster-name", "/acme/prod/eks/node-role-arn", "/acme/prod/s3/backup-bucket",
	"/acme/staging/app/api-url", "/acme/staging/app/log-level", "/acme/staging/db/host",
	"/acme/dev/app/api-url", "/acme/dev/app/debug-mode", "/acme/shared/app/region",
	"/acme/shared/app/account-id", "/acme/prod/app/cors-origins", "/acme/prod/app/rate-limit",
	"/acme/prod/monitoring/alert-email", "/acme/prod/app/session-timeout", "/acme/prod/app/cache-ttl",
}

var ssmTypePool = []string{
	"String", "String", "String",
	"String", "String", "String",
	"String", "String", "SecureString",
	"String", "String", "String",
	"String", "StringList", "String",
	"String", "String", "String",
}

func mustParseSSMTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewSSMFixtures constructs SSMFixtures from the canonical demo data.
func NewSSMFixtures() *SSMFixtures {
	ssmTypeMap := map[string]ssmtypes.ParameterType{
		"String":       ssmtypes.ParameterTypeString,
		"SecureString": ssmtypes.ParameterTypeSecureString,
		"StringList":   ssmtypes.ParameterTypeStringList,
	}

	params := []ssmtypes.ParameterMetadata{
		{
			Name:             aws.String("/acme/prod/app/config"),
			ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/prod/app/config"),
			Type:             ssmtypes.ParameterTypeString,
			Version:          12,
			LastModifiedDate: aws.Time(mustParseSSMTime("2026-03-15T14:30:00+00:00")),
			Description:      aws.String("Production application configuration"),
			DataType:         aws.String("text"),
			AllowedPattern:   aws.String(".*"),
			KeyId:            aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			LastModifiedUser: aws.String("arn:aws:iam::123456789012:user/admin"),
			Tier:             ssmtypes.ParameterTierStandard,
		},
		{
			Name:             aws.String("/acme/prod/db/connection-string"),
			ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/prod/db/connection-string"),
			Type:             ssmtypes.ParameterTypeSecureString,
			Version:          5,
			LastModifiedDate: aws.Time(mustParseSSMTime("2026-02-20T09:15:00+00:00")),
			Description:      aws.String("Encrypted database connection string"),
			KeyId:            aws.String("alias/acme-prod-key"),
			DataType:         aws.String("text"),
		},
		{
			Name:             aws.String("/acme/prod/feature-flags"),
			ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/prod/feature-flags"),
			Type:             ssmtypes.ParameterTypeStringList,
			Version:          28,
			LastModifiedDate: aws.Time(mustParseSSMTime("2026-03-20T11:45:00+00:00")),
			Description:      aws.String("Feature flag list for production"),
			DataType:         aws.String("text"),
		},
		{
			Name:             aws.String("/acme/staging/ami-id"),
			ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/staging/ami-id"),
			Type:             ssmtypes.ParameterTypeString,
			Version:          3,
			LastModifiedDate: aws.Time(mustParseSSMTime("2026-01-10T16:00:00+00:00")),
			Description:      aws.String("Latest approved AMI for staging"),
			DataType:         aws.String("aws:ec2:image"),
		},
		// Issue: Type=SecureString AND LastModifiedDate>365d → Warning (stale encrypted parameter)
		{
			Name:             aws.String("/acme/legacy/db/password"),
			ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/legacy/db/password"),
			Type:             ssmtypes.ParameterTypeSecureString,
			Version:          1,
			LastModifiedDate: aws.Time(mustParseSSMTime("2024-09-01T09:00:00+00:00")),
			Description:      aws.String("Legacy database password — not rotated in over a year"),
			KeyId:            aws.String("alias/aws/ssm"),
			DataType:         aws.String("text"),
		},
		// Issue: Type=String AND name suffix=/password → Warning (plaintext sensitive value)
		{
			Name:             aws.String("/acme/shared/thirdparty-api-token"),
			ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/shared/thirdparty-api-token"),
			Type:             ssmtypes.ParameterTypeString,
			Version:          2,
			LastModifiedDate: aws.Time(mustParseSSMTime("2025-11-20T14:00:00+00:00")),
			Description:      aws.String("Third-party API token stored as plaintext — should be SecureString"),
			DataType:         aws.String("text"),
		},
	}

	for i := range 18 {
		name := ssmNamePool[i]
		paramType := ssmTypePool[i]
		version := int64(1 + (i * 3 % 20))
		lastMod := fmt.Sprintf("2026-%02d-%02dT%02d:00:00+00:00", 1+(i%3), 1+i, 8+(i%12))
		desc := fmt.Sprintf("Parameter %s", name)
		params = append(params, ssmtypes.ParameterMetadata{
			Name:             aws.String(name),
			ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter" + name),
			Type:             ssmTypeMap[paramType],
			Version:          version,
			LastModifiedDate: aws.Time(mustParseSSMTime(lastMod)),
			Description:      aws.String(desc),
			DataType:         aws.String("text"),
		})
	}

	return &SSMFixtures{
		Parameters: params,
		ParameterValues: map[string]string{
			"/acme/prod/app/config":           "app_env=production,log_level=info",
			"/acme/prod/db/connection-string": "[REDACTED]",
			"/acme/prod/feature-flags":        "feature-a,feature-b,feature-c",
			"/acme/staging/ami-id":            "ami-0123456789abcdef0",
		},
	}
}
