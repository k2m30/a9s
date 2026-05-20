// scope.go — per-dispatch *aws.Scope constructor (AS-660).
//
// Lives in internal/runtime so it can import both internal/aws and
// internal/session without cycle. Called by the BT adapter's related-checker
// dispatch and by FetchByIDs lazy-add for the four resource types listed in
// internal/aws/scope.go.

package runtime

import (
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/session"
)

// NewScope constructs an *awsclient.Scope from a *session.Session. Returns
// nil when the session is nil or has not yet had Clients wired (pre-connect
// state). A nil return signals "no Scope available; fail closed" to the
// reader closures.
func NewScope(s *session.Session) *awsclient.Scope {
	if s == nil || s.Clients == nil {
		return nil
	}
	return &awsclient.Scope{
		Clients:       s.Clients,
		IAMPolicies:   s.IAMPolicies,
		IdentityStore: s.IdentityStore,
		RuleSets:      s.RuleSets,
	}
}
