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
	// PermissionsPolicies maps repository name to its GetRepositoryPermissionsPolicy
	// response. Backs EnrichCodeArtifactRepository's Wave-2 issue checks: a
	// missing entry surfaces as ResourceNotFoundException ("no permissions
	// policy" — "~"); a policy document containing "Principal":"*" surfaces
	// as "public access policy" ("!").
	PermissionsPolicies map[string]*codeartifacttypes.ResourcePolicy
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
			// acme-docker carries a permissions policy scoped to a single IAM
			// role (no "Principal":"*") — the only demo repository for which
			// EnrichCodeArtifactRepository raises neither
			// codeartifact.no-permissions-policy nor
			// codeartifact.public-access-policy, so colorCodeArtifact falls
			// through to its Healthy default.
			{
				Name:                 aws.String("acme-docker"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-docker"),
				Description:          aws.String("Private Docker registry for internal base images"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseCATime("2025-04-01T09:45:00+00:00")),
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
		// PermissionsPolicies — acme-npm carries an over-broad access policy
		// (Principal:"*") so EnrichCodeArtifactRepository's public-access-policy
		// "!" check fires. acme-pypi and acme-maven have no entry, so
		// GetRepositoryPermissionsPolicy returns ResourceNotFoundException and
		// the "no permissions policy" "~" check fires for them instead.
		PermissionsPolicies: map[string]*codeartifacttypes.ResourcePolicy{
			"acme-npm": {
				ResourceArn: aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-npm"),
				Document:    aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"codeartifact:ReadFromRepository","Resource":"*"}]}`),
			},
			// acme-docker's policy scopes read access to a single role — no
			// wildcard Principal, so the public-access-policy check does not
			// fire, and the entry's mere presence skips no-permissions-policy.
			"acme-docker": {
				ResourceArn: aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-docker"),
				Document:    aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-ci-deploy-role"},"Action":"codeartifact:ReadFromRepository","Resource":"*"}]}`),
			},
		},
	}
})

func NewCodeArtifactFixtures() *CodeArtifactFixtures {
	return sharedCodeArtifactFixtures()
}
