// aws_scope_integration_test.go — contract tests verifying that reader closures
// pick up stores from *awsclient.Scope, not from a bare *ServiceClients.
//
// NOTE TO CODER: resource.GetFetchByIDs must be created (parallel to GetRelated).
//
// These tests will FAIL TO COMPILE until the Coder creates:
//   - internal/aws/scope.go   (type Scope + interfaces)
//   - resource.GetFetchByIDs  (parallel to resource.GetRelated)
//
// That is the intended red-light state for Stage 3.
package unit_test

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
)

// trackingPolicyStore wraps session.PolicyStore and records Lookup calls so
// tests can assert that the store on *Scope (not on bare *ServiceClients) was
// used.
type trackingPolicyStore struct {
	session.PolicyStore
	lookupCalled bool
	lookupKey    string
}

func (t *trackingPolicyStore) Lookup(key string) (resource.Resource, bool) {
	t.lookupCalled = true
	t.lookupKey = key
	return t.PolicyStore.Lookup(key)
}

// TestScopeIntegration_ReadersUseScope groups all sub-tests that verify reader
// closures correctly accept *awsclient.Scope and fail-closed on bad input.
func TestScopeIntegration_ReadersUseScope(t *testing.T) {
	t.Run("policy/FetchByIDs/uses-scope-store", func(t *testing.T) {
		t.Parallel()
		tracker := &trackingPolicyStore{PolicyStore: session.NewPolicyStore()}
		scope := &awsclient.Scope{
			Clients:     &awsclient.ServiceClients{},
			IAMPolicies: tracker,
		}
		ctx := context.Background()
		fn := resource.GetFetchByIDs("policy")
		if fn == nil {
			t.Fatal("resource.GetFetchByIDs(\"policy\") returned nil — not yet registered")
		}
		fn(ctx, scope, []string{"arn:aws:iam::123456789012:policy/test"}) //nolint:errcheck
		if !tracker.lookupCalled {
			t.Error("Lookup was not called on scope.IAMPolicies — FetchByIDs must use the scope store, not bare ServiceClients")
		}
	})

	t.Run("policy/FetchByIDs/bare-clients-fails-closed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		fn := resource.GetFetchByIDs("policy")
		if fn == nil {
			t.Fatal("resource.GetFetchByIDs(\"policy\") returned nil — not yet registered")
		}
		// Pass a bare *ServiceClients (not a *Scope) — must fail closed, not panic.
		// Either an error or an empty/nil result slice is acceptable.
		// A panic is NOT acceptable (the test framework would catch it as a failure).
		fn(ctx, &awsclient.ServiceClients{}, []string{"arn:aws:iam::123456789012:policy/test"}) //nolint:errcheck
	})

	t.Run("ses/lambda/nil-scope-fails-closed", func(t *testing.T) {
		t.Parallel()
		// Scope has Clients but no RuleSets wired — must fail closed with Count=-1.
		scope := &awsclient.Scope{Clients: &awsclient.ServiceClients{}}
		src := resource.Resource{
			ID:     "any@example.com",
			Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
		}
		checker := sesCheckerByTarget(t, "lambda")
		result := checker(context.Background(), scope, src, resource.ResourceCache{})
		if result.Count != -1 {
			t.Errorf("ses/lambda with nil RuleSets in scope: Count = %d, want -1 (fail-closed)", result.Count)
		}
	})

	t.Run("ses/lambda/bare-clients-fails-closed", func(t *testing.T) {
		t.Parallel()
		// Pass bare *ServiceClients (not *Scope) — must fail closed, not panic.
		src := resource.Resource{
			ID:     "any@example.com",
			Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
		}
		checker := sesCheckerByTarget(t, "lambda")
		result := checker(context.Background(), &awsclient.ServiceClients{}, src, resource.ResourceCache{})
		if result.Count != -1 {
			t.Errorf("ses/lambda with bare *ServiceClients: Count = %d, want -1 (fail-closed)", result.Count)
		}
	})

	t.Run("glue/CFN/nil-identity-store-fails-closed", func(t *testing.T) {
		t.Parallel()
		// Scope has Clients but no IdentityStore — must fail closed with Count=-1.
		scope := &awsclient.Scope{Clients: &awsclient.ServiceClients{}}
		src := resource.Resource{ID: "some-glue-job", Name: "some-glue-job"}

		var checker resource.RelatedChecker
		for _, def := range resource.GetRelated("glue") {
			if def.TargetType == "cfn" {
				checker = def.Checker
				break
			}
		}
		if checker == nil {
			t.Skip("glue→cfn related checker not registered yet")
		}
		result := checker(context.Background(), scope, src, resource.ResourceCache{})
		if result.Count != -1 {
			t.Errorf("glue/CFN with nil IdentityStore in scope: Count = %d, want -1 (fail-closed)", result.Count)
		}
	})

	t.Run("ebs/backup/nil-identity-store-fails-closed", func(t *testing.T) {
		t.Parallel()
		// Scope has Clients but no IdentityStore — must fail closed with Count=-1.
		scope := &awsclient.Scope{Clients: &awsclient.ServiceClients{}}
		src := resource.Resource{ID: "vol-0123456789abcdef0", Name: "vol-0123456789abcdef0"}

		var checker resource.RelatedChecker
		for _, def := range resource.GetRelated("ebs") {
			if def.TargetType == "backup" {
				checker = def.Checker
				break
			}
		}
		if checker == nil {
			t.Skip("ebs→backup related checker not registered yet")
		}
		result := checker(context.Background(), scope, src, resource.ResourceCache{})
		if result.Count != -1 {
			t.Errorf("ebs/backup with nil IdentityStore in scope: Count = %d, want -1 (fail-closed)", result.Count)
		}
	})
}
