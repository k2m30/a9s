// Package aws — Scope carrier (AS-660).
//
// Scope bundles AWS transport with the per-session capability stores read by
// fetcher and related-checker code. Constructed by the runtime per dispatch
// from session.Session; passed as the registry's "clients any" carrier for
// the four resource types whose fetcher or checker closures consult
// session-scoped state:
//
//   - policy (FetchByIDs → reads IAMPolicies)
//   - glue   (checkGlueCFN → reads IdentityStore)
//   - vol    (checkEBSBackup → reads IdentityStore)
//   - ses    (checkSESLambda / checkSESS3 → reads RuleSets)
//
// All other fetchers continue to receive a bare *ServiceClients via the same
// registry parameter — the transport remains reachable via Scope.Clients.
//
// Lifetime: a Scope is read-only after construction. The stores it references
// are concurrency-safe internally (session.PolicyStore / IdentityStore /
// RuleSetStore each own their own synchronization). The runtime constructs a
// fresh Scope whenever it dispatches one of the four readers; no mutex on
// Scope itself is required.

package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// Scope is the per-dispatch carrier passed to registry fetcher and checker
// closures that need session-scoped capability stores in addition to the AWS
// transport. See package-level doc for the rationale.
type Scope struct {
	Clients       *ServiceClients
	IAMPolicies   IAMPolicyAccess
	IdentityStore IdentityAccess
	RuleSets      RuleSetAccess
}

// IAMPolicyAccess is the subset of session.PolicyStore consumed by
// FetchIAMPoliciesByIDsFull / buildAllManagedPolicies. Defined locally in
// internal/aws to avoid a cycle with internal/session; session.PolicyStore
// satisfies it via Go's structural typing.
type IAMPolicyAccess interface {
	Lookup(key string) (resource.Resource, bool)
	Set(key string, r resource.Resource)
	ManagedBuilt() bool
	MarkManagedBuilt()
	InlineBuilt() bool
	MarkInlineBuilt()
	Clear()
}

// IdentityAccess is the subset of session.IdentityStore consumed by
// accountIDFromClients. session.IdentityStore satisfies it via structural
// typing.
type IdentityAccess interface {
	AccountID() string
	Err() error
	Set(id string, err error)
}

// RuleSetAccess is the subset of session.RuleSetStore consumed by
// sesActiveReceiptRuleSet. session.RuleSetStore satisfies it via structural
// typing.
type RuleSetAccess interface {
	Get() (any, bool)
	Set(any)
	Clear()
	GetOrFetch(context.Context, func(context.Context) (any, error)) (any, error)
}

// serviceClientsFromAny returns the underlying *ServiceClients carried by
// `clients`. Accepts either *Scope (unwrapped) or *ServiceClients (returned
// as-is). Returns nil for any other value, or when the Scope carries no
// Clients. Helper for checker functions and FetchRelatedTarget — keeps
// transport-only fetchers agnostic about whether the dispatcher wrapped the
// transport in a Scope.
func serviceClientsFromAny(clients any) *ServiceClients {
	if s, ok := clients.(*Scope); ok {
		if s == nil {
			return nil
		}
		return s.Clients
	}
	c, _ := clients.(*ServiceClients)
	return c
}
