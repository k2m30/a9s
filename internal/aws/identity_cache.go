// identity_cache.go provides a lazy, per-process cache for the caller's AWS
// account ID and the resolved region. Used by related-panel Pattern C checkers
// that need to construct resource ARNs for APIs like Backup
// ListRecoveryPointsByResource and Glue GetTags.
//
// Resolution is best-effort: the cache is populated on first access via STS
// GetCallerIdentity (for account) and falls back to environment AWS_REGION
// or us-east-1 when region isn't otherwise known. Callers receive "" from
// these helpers when the info cannot be resolved; honest Count: -1 follows.
package aws

import (
	"context"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

var (
	identityCacheMu sync.Mutex //nolint:gochecknoglobals // process-scoped Pattern C cache

	cachedAccountID string //nolint:gochecknoglobals // set once; cleared on profile/region switch

	cachedAccountErr error //nolint:gochecknoglobals
)

// accountIDFromClients returns the caller's AWS account ID, fetched and cached
// via STS GetCallerIdentity on first call. Returns "" on any failure so
// callers emit Count: -1.
func accountIDFromClients(ctx context.Context, c *ServiceClients) string {
	identityCacheMu.Lock()
	defer identityCacheMu.Unlock()
	if cachedAccountID != "" {
		return cachedAccountID
	}
	if cachedAccountErr != nil {
		return ""
	}
	if c == nil || c.STS == nil {
		return ""
	}
	out, err := c.STS.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || out == nil || out.Account == nil {
		cachedAccountErr = err
		return ""
	}
	cachedAccountID = *out.Account
	return cachedAccountID
}

// regionFromEnv reads the default region from AWS_REGION or AWS_DEFAULT_REGION.
// Returns "" if neither is set — callers must Count: -1 in that case.
func regionFromEnv() string {
	if r := os.Getenv("AWS_REGION"); r != "" {
		return r
	}
	return os.Getenv("AWS_DEFAULT_REGION")
}
