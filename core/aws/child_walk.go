// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/k2m30/a9s/v3/core/resource"
)

// parentChildWalk lists a top-level type whose rows AWS lists only per
// parent — node groups, services and tasks per cluster — one fetch page at a
// time: a page of parents, every page of each parent's children, and the
// children described batch at a time until the page holds DefaultPageSize
// rows. A page that stops inside a parent returns a resume token that
// continues exactly there.
type parentChildWalk struct {
	listParents  func(ctx context.Context, token *string) (parents []string, next *string, err error)
	listChildren func(ctx context.Context, parent string, token *string) (children []string, next *string, err error)
	describe     func(ctx context.Context, parent string, children []string) ([]resource.Resource, error)
	// batch is the most children one describe call accepts.
	batch int
}

// walkResumeTokenPrefix marks a continuation token as the composite resume
// format below, as opposed to a plain parent-list NextToken, which is what the
// token holds whenever a page ends on a parent boundary. No AWS NextToken is
// documented to take this shape, so a token lacking the prefix is a plain
// parent-list continuation.
const walkResumeTokenPrefix = "walk-resume/v1?" //nolint:gosec // not a credential — a fixed marker prefix for the composite pagination resume token format

// walkResumeState is the parsed form of a composite resume token:
//   - parent: the parent whose children the page stopped inside.
//   - childToken: that parent's own child-list NextToken for the pages after
//     pendingChildren; empty once the parent has no more pages.
//   - pendingChildren: children already listed but not yet described.
//   - pendingParents: parents from the same parent-list page not yet visited.
//   - outerToken: the parent-list NextToken, for continuing past this page
//     once parent and every pending parent are drained.
type walkResumeState struct {
	parent          string
	childToken      string
	pendingChildren []string
	pendingParents  []string
	outerToken      string
}

// encodeWalkResumeToken serializes s as url.Values, so caller-controlled
// names (which could contain "=" or "&") are percent-encoded rather than
// misread as token structure.
func encodeWalkResumeToken(s walkResumeState) string {
	v := url.Values{}
	v.Set("rc", s.parent)
	if s.childToken != "" {
		v.Set("ngt", s.childToken)
	}
	for _, name := range s.pendingChildren {
		v.Add("pn", name)
	}
	for _, p := range s.pendingParents {
		v.Add("pc", p)
	}
	if s.outerToken != "" {
		v.Set("ot", s.outerToken)
	}
	return walkResumeTokenPrefix + v.Encode()
}

// decodeWalkResumeToken reports ok=false for any token not carrying
// walkResumeTokenPrefix (including a malformed query or one missing the
// parent), which is the path for a plain parent-list token.
func decodeWalkResumeToken(token string) (walkResumeState, bool) {
	if !strings.HasPrefix(token, walkResumeTokenPrefix) {
		return walkResumeState{}, false
	}
	v, err := url.ParseQuery(strings.TrimPrefix(token, walkResumeTokenPrefix))
	if err != nil || v.Get("rc") == "" {
		return walkResumeState{}, false
	}
	return walkResumeState{
		parent:          v.Get("rc"),
		childToken:      v.Get("ngt"),
		pendingChildren: v["pn"],
		pendingParents:  v["pc"],
		outerToken:      v.Get("ot"),
	}, true
}

// page returns the fetch page continuationToken names.
func (w parentChildWalk) page(ctx context.Context, continuationToken string) (resource.FetchResult, error) {
	var parents, pending []string
	var childToken *string
	var outerToken string
	resuming := false
	if st, ok := decodeWalkResumeToken(continuationToken); ok {
		resuming = true
		parents = append([]string{st.parent}, st.pendingParents...)
		pending = st.pendingChildren
		if st.childToken != "" {
			childToken = aws.String(st.childToken)
		}
		outerToken = st.outerToken
	} else {
		var token *string
		if continuationToken != "" {
			token = aws.String(continuationToken)
		}
		ps, next, err := w.listParents(ctx, token)
		if err != nil {
			return resource.FetchResult{}, err
		}
		parents, outerToken = ps, aws.ToString(next)
	}

	var rows []resource.Resource
	// describeUntilFull describes names batch at a time and returns the tail
	// left undescribed when the page filled up.
	describeUntilFull := func(parent string, names []string) ([]string, error) {
		for len(names) > 0 {
			if len(rows) >= DefaultPageSize {
				return names, nil
			}
			n := min(w.batch, len(names))
			got, err := w.describe(ctx, parent, names[:n])
			if err != nil {
				return nil, err
			}
			rows = append(rows, got...)
			names = names[n:]
		}
		return nil, nil
	}
	resumeAt := func(parent string, left []string, next *string, rest []string) resource.FetchResult {
		return resource.FetchResult{
			Resources: rows,
			Pagination: &resource.PaginationMeta{
				IsTruncated: true,
				NextToken: encodeWalkResumeToken(walkResumeState{
					parent:          parent,
					childToken:      aws.ToString(next),
					pendingChildren: left,
					pendingParents:  append([]string(nil), rest...),
					outerToken:      outerToken,
				}),
				PageSize:  len(rows),
				TotalHint: -1,
			},
		}
	}

	for i, parent := range parents {
		var token *string
		if i == 0 && resuming {
			left, err := describeUntilFull(parent, pending)
			if err != nil {
				return resource.FetchResult{}, err
			}
			if len(left) > 0 {
				return resumeAt(parent, left, childToken, parents[i+1:]), nil
			}
			if childToken == nil {
				continue
			}
			token = childToken
		}
		for {
			names, next, err := w.listChildren(ctx, parent, token)
			if err != nil {
				return resource.FetchResult{}, err
			}
			left, err := describeUntilFull(parent, names)
			if err != nil {
				return resource.FetchResult{}, err
			}
			if len(left) > 0 {
				return resumeAt(parent, left, next, parents[i+1:]), nil
			}
			if aws.ToString(next) == "" {
				break
			}
			token = next
		}
	}

	return resource.FetchResult{
		Resources: rows,
		Pagination: &resource.PaginationMeta{
			IsTruncated: outerToken != "",
			NextToken:   outerToken,
			PageSize:    len(rows),
			TotalHint:   -1,
		},
	}, nil
}
