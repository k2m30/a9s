// aws_scope_test.go — unit tests for internal/runtime.NewScope constructor.
//
// These tests will FAIL TO COMPILE until the Coder creates:
//   - internal/aws/scope.go   (type Scope struct + interfaces)
//   - internal/runtime/scope.go (func NewScope(*session.Session) *awsclient.Scope)
//
// That is the intended red-light state for Stage 3.
package unit_test

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	appruntime "github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// TestNewScope_NilSession_ReturnsNil verifies that NewScope(nil) returns nil
// without panicking.
func TestNewScope_NilSession_ReturnsNil(t *testing.T) {
	t.Parallel()
	got := appruntime.NewScope(nil)
	if got != nil {
		t.Errorf("NewScope(nil) = %v, want nil", got)
	}
}

// TestNewScope_NilClients_ReturnsNil verifies that NewScope returns nil when
// the session exists but its Clients field is nil. A Scope without a live
// transport is not usable and must not be handed to callers.
func TestNewScope_NilClients_ReturnsNil(t *testing.T) {
	t.Parallel()
	s := session.New()
	// Clients is nil on a freshly constructed Session (set only after a
	// successful ClientsReadyMsg).
	if s.Clients != nil {
		t.Skip("test precondition violated: Clients must be nil on a fresh Session")
	}
	got := appruntime.NewScope(s)
	if got != nil {
		t.Errorf("NewScope(s with nil Clients) = %v, want nil", got)
	}
}

// TestNewScope_HappyPath_FieldsPointAtSessionStores verifies that a valid
// Scope carries the same store references as the Session it was built from,
// and that the Clients pointer is identical.
func TestNewScope_HappyPath_FieldsPointAtSessionStores(t *testing.T) {
	t.Parallel()
	s := session.New()
	s.Clients = &awsclient.ServiceClients{}

	scope := appruntime.NewScope(s)
	if scope == nil {
		t.Fatal("NewScope returned nil for a session with non-nil Clients")
	}

	if scope.Clients != s.Clients {
		t.Error("scope.Clients does not point at s.Clients")
	}
	if scope.IAMPolicies == nil {
		t.Error("scope.IAMPolicies is nil — must reference s.IAMPolicies")
	}
	if scope.IAMPolicies != s.IAMPolicies {
		t.Error("scope.IAMPolicies does not reference the same store as s.IAMPolicies")
	}
	if scope.IdentityStore == nil {
		t.Error("scope.IdentityStore is nil — must reference s.IdentityStore")
	}
	if scope.IdentityStore != s.IdentityStore {
		t.Error("scope.IdentityStore does not reference the same store as s.IdentityStore")
	}
	if scope.RuleSets == nil {
		t.Error("scope.RuleSets is nil — must reference s.RuleSets")
	}
	if scope.RuleSets != s.RuleSets {
		t.Error("scope.RuleSets does not reference the same store as s.RuleSets")
	}
}
