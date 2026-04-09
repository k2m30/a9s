package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/codeartifact"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// CodeArtifactFake implements aws.CodeArtifactAPI against fixture data loaded at construction time.
type CodeArtifactFake struct {
	fix *fixtures.CodeArtifactFixtures
}

// NewCodeArtifact constructs a CodeArtifactFake backed by fixture data from the fixtures package.
func NewCodeArtifact() *CodeArtifactFake {
	return &CodeArtifactFake{fix: fixtures.NewCodeArtifactFixtures()}
}

func (f *CodeArtifactFake) ListRepositories(_ context.Context, _ *codeartifact.ListRepositoriesInput, _ ...func(*codeartifact.Options)) (*codeartifact.ListRepositoriesOutput, error) {
	return &codeartifact.ListRepositoriesOutput{Repositories: f.fix.Repositories}, nil
}
