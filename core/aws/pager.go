// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "context"

// PageAll walks a paginated AWS list: next is called with a nil token for the
// first page and with each returned token after it, until a page carries no
// token (an empty-string token also ends the list) or maxPages pages were
// read. complete is false when the walk stopped at the cap with a token still
// pending, or on error; on error items holds every page read before the
// failed one.
func PageAll[T any](ctx context.Context, maxPages int, next func(ctx context.Context, token *string) (items []T, nextToken *string, err error)) (items []T, complete bool, err error) {
	var token *string
	for range maxPages {
		got, tok, err := next(ctx, token)
		if err != nil {
			return items, false, err
		}
		items = append(items, got...)
		if tok == nil || *tok == "" {
			return items, true, nil
		}
		token = tok
	}
	return items, false, nil
}

// firstPage reads one page of a paged API on purpose: the request names its
// one item, asks whether a page exists at all, or the API reference documents
// an order that puts the item on the first page (newest first). An API that
// documents no order is walked with PageAll.
//
// It only calls read: it is the marker the paging guard tests key on. A paged
// call inside a firstPage closure is exempt from TestT571_PagedReadsGoThroughPageAll
// and must instead name an API with a documented order
// (TestT571_FirstPageReadsHaveAnOrder); the same call made directly fails the
// first guard.
func firstPage[T any](read func() (T, error)) (T, error) {
	return read()
}
