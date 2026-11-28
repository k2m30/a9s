// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwv1types "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
)

// API Gateway quota: "GetResources | 5 requests every 2 seconds per account |
// can't be increased" (API Gateway Developer Guide, "Amazon API Gateway
// quotas"). The SDK retryer absorbs a throttle now and then; a Wave 2 pass
// that sends one call per REST API at row parallelism would spend its retries
// on the quota and mark the rows not inspected.
const (
	apigwGetResourcesCalls  = 5
	apigwGetResourcesWindow = 2 * time.Second
	apigwGetResourcesLimit  = 500
)

// callWindow hands each call a send time: now, or the moment the n-th call
// before it leaves the window, so calls already waiting keep their order.
type callWindow struct {
	mu       sync.Mutex
	reserved []time.Time
}

func (w *callWindow) admit(ctx context.Context, n int, per time.Duration) error {
	w.mu.Lock()
	at := time.Now()
	if len(w.reserved) >= n {
		if free := w.reserved[len(w.reserved)-n].Add(per); free.After(at) {
			at = free
		}
	}
	w.reserved = append(w.reserved, at)
	if len(w.reserved) > n {
		w.reserved = w.reserved[len(w.reserved)-n:]
	}
	w.mu.Unlock()

	wait := time.Until(at)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// apigwRESTResources is every caller's read of a REST API's resources with
// their methods embedded, at the largest page GetResources accepts and inside
// the account's GetResources quota, which the issue enricher and the related
// panel share through the client set.
func apigwRESTResources(ctx context.Context, c *ServiceClients, api APIGatewayV1GetResourcesAPI, apiID string) ([]apigwv1types.Resource, bool, error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, position *string) ([]apigwv1types.Resource, *string, error) {
		// The quota is the service's: a substitute client answers none.
		if _, sdk := api.(*apigateway.Client); sdk {
			if err := c.apigwResources.admit(ctx, apigwGetResourcesCalls, apigwGetResourcesWindow); err != nil {
				return nil, nil, err
			}
		}
		out, err := api.GetResources(ctx, &apigateway.GetResourcesInput{
			RestApiId: aws.String(apiID),
			Embed:     []string{"methods"},
			Limit:     aws.Int32(apigwGetResourcesLimit),
			Position:  position,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.Position, nil
	})
}
