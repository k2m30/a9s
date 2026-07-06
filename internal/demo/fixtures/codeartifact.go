package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
)

// CodeArtifactFixtures holds typed fixture data for CodeArtifact.
type CodeArtifactFixtures struct {
	Repositories []codeartifacttypes.RepositorySummary
	// Domains maps domain name -> domain description, served by
	// DescribeDomain. Required for the codeartifact:kms related-panel pivot
	// witness (checkCodeartifactKMS reads Domain.EncryptionKey — CodeArtifact
	// encryption is a domain-level, not repository-level, property).
	Domains map[string]codeartifacttypes.DomainDescription
}

func mustParseCATime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewCodeArtifactFixtures constructs CodeArtifactFixtures from the canonical demo data.
var sharedCodeArtifactFixtures = sync.OnceValue(func() *CodeArtifactFixtures {
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
		Domains: map[string]codeartifacttypes.DomainDescription{
			"acme-artifacts": {
				Name:          aws.String("acme-artifacts"),
				Owner:         aws.String("123456789012"),
				Arn:           aws.String("arn:aws:codeartifact:us-east-1:123456789012:domain/acme-artifacts"),
				EncryptionKey: aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				CreatedTime:   aws.Time(mustParseCATime("2025-04-01T08:00:00+00:00")),
			},
		},
	}
})

func NewCodeArtifactFixtures() *CodeArtifactFixtures {
	return sharedCodeArtifactFixtures()
}
