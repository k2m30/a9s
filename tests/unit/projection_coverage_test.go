package unit_test

// projection_coverage_test.go — PR-01 exit criterion #7.
//
// TestProjectorCoverageAllTypes asserts that every registered resource type has a
// working projector (either td.Project, or the Generic fallback) that returns
// non-empty sections for at least one demo fixture.
//
// EXPECTED failure today: projection.Generic is a stub returning nil.
// This test fails with "%s: projector returned zero sections" for every type until
// projection.Generic is implemented in the PR-01 impl step.
//
// When td.Project is added to ResourceTypeDef (PR-01), the loop becomes:
//
//	projector := td.Project
//	if projector == nil { projector = projection.Generic }
//	got := projector(r)

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/projection"
)

// minimalResource returns a domain.Resource with enough content that a real
// projector implementation would produce at least one section. Fields carries
// the type's ShortName as a sentinel value so the projector can emit at least
// an ID row.
func minimalResource(shortName string) domain.Resource {
	return domain.Resource{
		ID:     shortName + "-fixture-id",
		Name:   shortName + "-fixture-name",
		Status: "active",
		Fields: map[string]string{
			"id":     shortName + "-fixture-id",
			"name":   shortName + "-fixture-name",
			"status": "active",
		},
	}
}

// TestProjectorCoverageAllTypes iterates every registered resource type and
// verifies that the Generic projector (the fallback used until td.Project is set)
// returns at least one section for a representative fixture resource.
//
// Structural regression guard: any future per-resource breakage that loses
// detail-view content fails this test. It replaces the old "verify ec2/s3/rds..."
// smoke list and covers all 66+ registered types in one loop.
func TestProjectorCoverageAllTypes(t *testing.T) {
	types := resource.AllResourceTypes()
	if len(types) == 0 {
		t.Fatal("resource.AllResourceTypes() returned empty slice — registry not populated")
	}

	// Build a small cache of fetched resources keyed by ShortName.
	// We populate only the types we can easily fetch via demo clients to get
	// realistic Resources with populated Fields and RawStruct.
	demoResources := fetchDemoResourceSample(t)

	for _, td := range types {
		td := td
		t.Run(td.ShortName, func(t *testing.T) {
			// Use a demo fixture if available; fall back to a minimal synthetic resource.
			var r domain.Resource
			if rs, ok := demoResources[td.ShortName]; ok && len(rs) > 0 {
				r = rs[0]
			} else {
				r = minimalResource(td.ShortName)
			}

			// td.Project is not yet a field (added in PR-01 impl step).
			// After the impl step this should become:
			//   projector := td.Project
			//   if projector == nil { projector = projection.Generic }
			//   got := projector(r)
			got := projection.Generic(r)
			if len(got) == 0 {
				t.Errorf("%s: projector returned zero sections (projection.Generic stub returns nil — impl step needed)", td.ShortName)
			}
		})
	}
}

// fetchDemoResourceSample fetches a small representative set of resources from
// demo fakes to give the projector realistic data with populated RawStruct.
// Types without a fast single-call fetcher are skipped (MinimalResource covers them).
func fetchDemoResourceSample(t *testing.T) map[string][]domain.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	ctx := context.Background()
	out := make(map[string][]domain.Resource)

	tryAdd := func(shortName string, rs []resource.Resource, err error) {
		t.Helper()
		if err != nil || len(rs) == 0 {
			return
		}
		out[shortName] = rs
	}

	// Compute
	ec2res, err := awsclient.FetchEC2Instances(ctx, clients.EC2)
	tryAdd("ec2", ec2res, err)

	// Containers
	ecsRes, err := awsclient.FetchECSClusters(ctx, clients.ECS, clients.ECS)
	tryAdd("ecs", ecsRes, err)

	// Database
	rdsRes, err := awsclient.FetchRDSInstances(ctx, clients.RDS)
	tryAdd("rds", rdsRes, err)

	ddbRes, err := awsclient.FetchDynamoDBTables(ctx, clients.DynamoDB, clients.DynamoDB)
	tryAdd("ddb", ddbRes, err)

	// Storage
	s3Res, err := awsclient.FetchS3Buckets(ctx, clients.S3)
	tryAdd("s3", s3Res, err)

	// Serverless
	lambdaRes, err := awsclient.FetchLambdaFunctions(ctx, clients.Lambda)
	tryAdd("lambda", lambdaRes, err)

	// Security / identity
	iamRoles, err := awsclient.FetchIAMRoles(ctx, clients.IAM)
	tryAdd("role", iamRoles, err)

	secretsRes, err := awsclient.FetchSecrets(ctx, clients.SecretsManager)
	tryAdd("secrets", secretsRes, err)

	// Monitoring
	ctRes, err := awsclient.FetchCloudTrailEvents(ctx, clients.CloudTrail)
	tryAdd("ct-events", ctRes, err)

	// Networking
	sgsRes, err := awsclient.FetchSecurityGroups(ctx, clients.EC2)
	tryAdd("sg", sgsRes, err)

	return out
}
