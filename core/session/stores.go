// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package session

import (
	"slices"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// clientStores are the session stores the AWS transport reads through. Each
// names the types whose refresh reads again what it remembers; New, Rotate,
// WireStores and RefreshStores all walk this list.
var clientStores = []struct {
	refreshedBy []string
	reset       func(*Session)
	wire        func(*Session, *awsclient.ServiceClients)
}{
	{
		// A drill into a policy resolves through this store, and role, user
		// and group details name the policies attached to them.
		refreshedBy: []string{"policy", "role", "iam-user", "iam-group"},
		reset:       func(s *Session) { s.IAMPolicies = NewPolicyStore() },
		wire:        func(s *Session, c *awsclient.ServiceClients) { c.SetIAMPolicies(s.IAMPolicies) },
	},
	{
		// The caller's account ID, which no resource refresh changes.
		reset: func(s *Session) { s.IdentityStore = NewIdentityStore() },
		wire:  func(s *Session, c *awsclient.ServiceClients) { c.SetIdentityStore(s.IdentityStore) },
	},
	{
		refreshedBy: []string{"ses"},
		reset:       func(s *Session) { s.RuleSets = NewRuleSetStore() },
		wire:        func(s *Session, c *awsclient.ServiceClients) { c.SetRuleSets(s.RuleSets) },
	},
}

func (s *Session) resetClientStores() {
	for _, st := range clientStores {
		st.reset(s)
	}
}

// WireStores points c at the session's stores.
func (s *Session) WireStores(c *awsclient.ServiceClients) {
	for _, st := range clientStores {
		st.wire(s, c)
	}
}

// RefreshStores drops what the session remembers of rt beside its rows, so a
// refresh of its list or of one of its details reads it again. The previous
// store is left to any in-flight read that captured it.
func (s *Session) RefreshStores(rt string) {
	for _, st := range clientStores {
		if slices.Contains(st.refreshedBy, rt) {
			st.reset(s)
			st.wire(s, s.Clients)
		}
	}
}
