package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
)

// CodeArtifactFixtures holds typed fixture data for CodeArtifact.
type CodeArtifactFixtures struct {
	Repositories []codeartifacttypes.RepositorySummary
}

func mustParseCATime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewCodeArtifactFixtures constructs CodeArtifactFixtures from the canonical demo data.
func NewCodeArtifactFixtures() *CodeArtifactFixtures {
	return &CodeArtifactFixtures{
		Repositories: []codeartifacttypes.RepositorySummary{
			{
				Name:                 aws.String("acme-npm"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-npm"),
				Description:          aws.String("Private npm registry for Acme frontend packages"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseCATime("2025-04-01T09:00:00+00:00")),
			},
			{
				Name:                 aws.String("acme-pypi"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-pypi"),
				Description:          aws.String("Private PyPI repository for data pipeline packages"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseCATime("2025-04-01T09:15:00+00:00")),
			},
			{
				Name:                 aws.String("acme-maven"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-maven"),
				Description:          aws.String("Maven repository for Java microservices"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseCATime("2025-04-01T09:30:00+00:00")),
			},
		},
	}
}
