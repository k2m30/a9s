// scope_dispatch.go — per-type carrier selection for the related-check
// dispatcher (AS-660).
//
// The four reader closures defined in internal/aws consume session-scoped
// stores via *aws.Scope:
//
//   - source "ses" → checkSESLambda / checkSESS3 read scope.RuleSets
//   - source "glue" → checkGlueCFN reads scope.IdentityStore
//   - source "vol"  → checkEBSBackup reads scope.IdentityStore
//   - target "policy" → FetchByIDs reads scope.IAMPolicies
//
// All other registry fetchers continue to receive a bare *ServiceClients per
// spec §3.4 (transport-only fetchers stay untouched). The helpers below
// encapsulate that selection so callers in runtime_adapter_related.go don't
// inline a switch in every call site.

package tui

import (
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// relatedCheckerClients returns the "clients any" carrier to hand to a
// related-check Checker function for the given source resource type.
// Sources whose registered Checkers consume session-scoped stores receive
// a fresh *aws.Scope; all others receive the bare transport.
func relatedCheckerClients(s *session.Session, sourceType string) any {
	if s == nil {
		return nil
	}
	switch sourceType {
	case "ses", "glue", "vol":
		if sc := runtime.NewScope(s); sc != nil {
			return sc
		}
		return s.Clients
	default:
		return s.Clients
	}
}

// fetchByIDsClients returns the "clients any" carrier to hand to a
// FetchByIDs closure for the given target resource type.
func fetchByIDsClients(s *session.Session, targetType string) any {
	if s == nil {
		return nil
	}
	if targetType == "policy" {
		if sc := runtime.NewScope(s); sc != nil {
			return sc
		}
		return s.Clients
	}
	return s.Clients
}
