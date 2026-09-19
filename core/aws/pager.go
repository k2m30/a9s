// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "context"

// PageAll walks a paginated AWS list: next is called with a nil token for the
// first page and with each returned token after it, through RetryOnThrottle,
// until a page carries no token (an empty-string token also ends the list) or
// maxPages pages were read. complete is false when the walk stopped at the cap
// with a token still pending, or on error; on error items holds every page
// read before the failed one.
func PageAll[T any](ctx context.Context, maxPages int, next func(ctx context.Context, token *string) (items []T, nextToken *string, err error)) (items []T, complete bool, err error) {
	type page struct {
		items []T
		token *string
	}
	var token *string
	for range maxPages {
		p, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (page, error) {
			got, tok, err := next(ctx, token)
			return page{got, tok}, err
		})
		if err != nil {
			return items, false, err
		}
		items = append(items, p.items...)
		if p.token == nil || *p.token == "" {
			return items, true, nil
		}
		token = p.token
	}
	return items, false, nil
}
