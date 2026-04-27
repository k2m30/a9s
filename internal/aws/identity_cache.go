// identity_cache.go provides a session-scoped lookup for the caller's AWS
// account ID and the resolved region. Used by related-panel Pattern C checkers
// that need to construct resource ARNs for APIs like Backup
// ListRecoveryPointsByResource and Glue GetTags.
//
// Resolution is best-effort: the per-session store is populated on first
// access via STS GetCallerIdentity (for account) and falls back to environment
// AWS_REGION or us-east-1 when region isn't otherwise known. Callers receive
// "" from these helpers when the info cannot be resolved; honest Count: -1
// follows.
//
// Concurrency note (PR-02b precedent): no top-level lock is held across the
// "check store / fetch / set" sequence. Two concurrent Pattern C checks may
// both observe AccountID()=="" + Err()==nil and both invoke
// STS.GetCallerIdentity. The store itself remains correct; duplicate Set
// calls are last-write-wins on identical successful results. Acceptable
// because related-panel checks are not high-volume.
package aws

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// identityStore is the unexported shape internal/aws expects from a
// per-Session identity cache. session.IdentityStore() satisfies this via
// duck-typing — internal/aws cannot import internal/session without a cycle,
// so the local interface mirrors the methods needed here.
type identityStore interface {
	AccountID() string
	Err() error
	Set(id string, err error)
}

// accountIDFromClients returns the caller's AWS account ID, fetched and
// cached via STS GetCallerIdentity on first call. Returns "" on any failure
// so callers emit Count: -1.
//
// The store argument carries per-session state; pass c.IdentityStore() at the
// call site so each profile/region session uses an isolated cache.
func accountIDFromClients(ctx context.Context, c *ServiceClients, store identityStore) string {
	if store == nil {
		// Defensive: store must always be wired by handleClientsReady. Fall
		// back to a fresh STS call so the related-checker still works (slow
		// path) but log nothing — surfacing a missing-store error here would
		// just spam the flash log on every related-check pass.
		return liveAccountID(ctx, c)
	}
	if id := store.AccountID(); id != "" {
		return id
	}
	if store.Err() != nil {
		return ""
	}
	id := liveAccountID(ctx, c)
	if id == "" {
		store.Set("", errAccountUnresolved)
		return ""
	}
	store.Set(id, nil)
	return id
}

// liveAccountID issues a fresh STS GetCallerIdentity call and returns the
// account ID, or "" on any failure (no client, no STS API, API error, nil
// response).
func liveAccountID(ctx context.Context, c *ServiceClients) string {
	if c == nil || c.STS == nil {
		return ""
	}
	out, err := c.STS.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || out == nil || out.Account == nil {
		return ""
	}
	return *out.Account
}

// errAccountUnresolved sentinel marks a sticky failure: the STS fetch was
// attempted and returned nothing useful. Stored in IdentityStore.Err() so
// subsequent calls within the session skip retry.
var errAccountUnresolved = errSentinel("account-unresolved")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

// regionFromEnv reads the default region from AWS_REGION or AWS_DEFAULT_REGION.
// Returns "" if neither is set — callers must Count: -1 in that case.
func regionFromEnv() string {
	if r := os.Getenv("AWS_REGION"); r != "" {
		return r
	}
	return os.Getenv("AWS_DEFAULT_REGION")
}
