// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go"
)

// retryMaxAttempts is the attempt budget of every AWS call a9s makes. A sweep
// or an enrichment fans out many reads at once against per-account API rate
// limits, so a throttled read gets more tries than the SDK's default 3.
const retryMaxAttempts = 5

// retryMaxBackoff caps the wait between two attempts so that every attempt
// fits the shortest deadline a9s puts on a read (5 s): four waits of at most
// 1 s each.
const retryMaxBackoff = time.Second

// retryMaxBackoffForTest, when non-zero, replaces retryMaxBackoff. Set only
// through SetRetryMaxBackoffForTest.
var retryMaxBackoffForTest atomic.Int64

// errNoTimeForRetry ends the retries of a read whose deadline is closer than
// the longest next wait. It deliberately wraps no context error: the read
// ends as the throttle it got, not as a timeout.
var errNoTimeForRetry = errors.New("no time left before the deadline for another attempt")

// isThrottle is the one definition of a throttle in a9s, read by the retryer
// and by ErrClass: an error code the SDK lists as a throttle, or an HTTP 429
// whose code no protocol models as one.
func isThrottle(err error) bool {
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		if _, listed := retry.DefaultThrottleErrorCodes[apiErr.ErrorCode()]; listed {
			return true
		}
	}
	respErr, ok := errors.AsType[interface {
		error
		HTTPStatusCode() int
	}](err)
	return ok && respErr.HTTPStatusCode() == http.StatusTooManyRequests
}

// newRetryer is the one retry policy of a9s, set on the session config by
// NewAWSSessionContext (so the credential clients the SDK builds from it use
// it) and on every service client by CreateServiceClients. It retries what the
// SDK's standard retryer does (5xx, transport failures) and every isThrottle
// error.
//
// The client-side retry quota is off: under a burst of parallel throttles it
// would end retries early, the moment they are needed most. Load on a
// throttled API is bounded by a9s's fan-out caps (EnrichmentParallelism and
// the sweep's window), not by the quota.
func newRetryer() aws.Retryer {
	maxBackoff := retryMaxBackoff
	if d := retryMaxBackoffForTest.Load(); d > 0 {
		maxBackoff = time.Duration(d)
	}
	return deadlineRetryer{
		RetryerV2: retry.NewStandard(func(o *retry.StandardOptions) {
			o.MaxAttempts = retryMaxAttempts
			o.MaxBackoff = maxBackoff
			o.RateLimiter = ratelimit.None
			o.Retryables = append(o.Retryables, retry.IsErrorRetryableFunc(func(err error) aws.Ternary {
				if isThrottle(err) {
					return aws.TrueTernary
				}
				return aws.UnknownTernary
			}))
		}),
		maxBackoff: maxBackoff,
	}
}

// deadlineRetryer declines a retry the caller's deadline would cut short. The
// SDK sleeps into the deadline and then returns only the context error, so a
// throttled read would read as a timeout; a declined retry token instead
// returns the throttle joined with errNoTimeForRetry.
type deadlineRetryer struct {
	aws.RetryerV2
	maxBackoff time.Duration
}

func (r deadlineRetryer) GetRetryToken(ctx context.Context, opErr error) (func(error) error, error) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < r.maxBackoff {
		return nil, errNoTimeForRetry
	}
	return r.RetryerV2.GetRetryToken(ctx, opErr)
}
