package unit

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// pageAllScript answers PageAll's next callback from a fixed table keyed by
// the incoming token ("" for the first call), recording every token it saw.
type pageAllScript struct {
	pages  map[string]pageAllPage
	tokens []*string
}

type pageAllPage struct {
	items []string
	next  *string
	// errs are returned, in order, before the page itself is served.
	errs []error
}

func (s *pageAllScript) next(_ context.Context, token *string) ([]string, *string, error) {
	s.tokens = append(s.tokens, token)
	key := aws.ToString(token)
	p := s.pages[key]
	if len(p.errs) > 0 {
		err := p.errs[0]
		p.errs = p.errs[1:]
		s.pages[key] = p
		return nil, nil, err
	}
	return p.items, p.next, nil
}

func (s *pageAllScript) callsFor(token string) int {
	n := 0
	for _, tk := range s.tokens {
		if aws.ToString(tk) == token {
			n++
		}
	}
	return n
}

func TestPageAll_WalksEveryPageInOrder(t *testing.T) {
	s := &pageAllScript{pages: map[string]pageAllPage{
		"":      {items: []string{"a", "b"}, next: aws.String("tok-2")},
		"tok-2": {items: []string{"c"}, next: aws.String("tok-3")},
		"tok-3": {items: []string{"d", "e"}},
	}}
	items, complete, err := awsclient.PageAll(context.Background(), 10, s.next)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !complete {
		t.Error("complete = false, want true: the last page carried no token")
	}
	if want := []string{"a", "b", "c", "d", "e"}; !slices.Equal(items, want) {
		t.Errorf("items = %v, want %v", items, want)
	}
	if len(s.tokens) != 3 {
		t.Fatalf("next called %d times, want 3", len(s.tokens))
	}
	// The first request of an AWS list call carries no token at all, not an
	// empty one: several services reject NextToken="" as an invalid token.
	if s.tokens[0] != nil {
		t.Errorf("first token = %q, want nil", *s.tokens[0])
	}
	if aws.ToString(s.tokens[1]) != "tok-2" || aws.ToString(s.tokens[2]) != "tok-3" {
		t.Errorf("tokens = [%v %q %q], want [nil tok-2 tok-3]", s.tokens[0], aws.ToString(s.tokens[1]), aws.ToString(s.tokens[2]))
	}
}

// Some services answer the last page with an empty-string token rather than
// omitting it; that is the end of the list, not a page to request.
func TestPageAll_EmptyStringTokenEndsTheWalk(t *testing.T) {
	s := &pageAllScript{pages: map[string]pageAllPage{
		"": {items: []string{"only"}, next: aws.String("")},
	}}
	items, complete, err := awsclient.PageAll(context.Background(), 10, s.next)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !complete {
		t.Error("complete = false, want true")
	}
	if !slices.Equal(items, []string{"only"}) {
		t.Errorf("items = %v, want [only]", items)
	}
	if len(s.tokens) != 1 {
		t.Errorf("next called %d times, want 1", len(s.tokens))
	}
}

func TestPageAll_StopsAtMaxPagesAndReportsIncomplete(t *testing.T) {
	calls := 0
	next := func(_ context.Context, _ *string) ([]int, *string, error) {
		calls++
		return []int{calls}, aws.String("more"), nil
	}
	items, complete, err := awsclient.PageAll(context.Background(), 3, next)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if complete {
		t.Error("complete = true, want false: the walk stopped at the cap with a token still pending")
	}
	if calls != 3 {
		t.Errorf("next called %d times, want exactly maxPages = 3", calls)
	}
	if !slices.Equal(items, []int{1, 2, 3}) {
		t.Errorf("items = %v, want [1 2 3]", items)
	}
}

// A list whose last page is exactly the cap is complete: the cap only makes
// a walk incomplete when a token is still pending after it.
func TestPageAll_LastPageAtTheCapIsComplete(t *testing.T) {
	s := &pageAllScript{pages: map[string]pageAllPage{
		"":      {items: []string{"a"}, next: aws.String("tok-2")},
		"tok-2": {items: []string{"b"}, next: aws.String("tok-3")},
		"tok-3": {items: []string{"c"}},
	}}
	items, complete, err := awsclient.PageAll(context.Background(), 3, s.next)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !complete {
		t.Error("complete = false, want true: page 3 of 3 carried no token")
	}
	if !slices.Equal(items, []string{"a", "b", "c"}) {
		t.Errorf("items = %v, want [a b c]", items)
	}
}

func TestPageAll_FailedPageReturnsItemsReadSoFar(t *testing.T) {
	denied := &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized to perform: iam:GetGroup"}
	s := &pageAllScript{pages: map[string]pageAllPage{
		"":      {items: []string{"a", "b"}, next: aws.String("tok-2")},
		"tok-2": {errs: []error{denied}, items: []string{"c"}, next: aws.String("tok-3")},
		"tok-3": {items: []string{"d"}},
	}}
	items, _, err := awsclient.PageAll(context.Background(), 10, s.next)
	if err == nil {
		t.Fatal("err = nil, want the failed page's error")
	}
	apiErr, ok := errors.AsType[smithy.APIError](err)
	if !ok || apiErr.ErrorCode() != "AccessDenied" {
		t.Errorf("err = %v, want it to carry the AccessDenied API error", err)
	}
	if !slices.Equal(items, []string{"a", "b"}) {
		t.Errorf("items = %v, want the first page [a b]", items)
	}
	// AccessDenied is not a throttle: one attempt, and no page after it.
	if n := s.callsFor("tok-2"); n != 1 {
		t.Errorf("failed page requested %d times, want 1 (no retry of a non-throttle error)", n)
	}
	if n := s.callsFor("tok-3"); n != 0 {
		t.Errorf("page after the failure requested %d times, want 0", n)
	}
}

func TestPageAll_RetriesAThrottledPage(t *testing.T) {
	restore := awsclient.SetRetryConfigForTest(&awsclient.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    2 * time.Millisecond,
	})
	defer restore()

	throttled := &smithy.GenericAPIError{Code: "ThrottlingException", Message: "Rate exceeded"}
	s := &pageAllScript{pages: map[string]pageAllPage{
		"":      {items: []string{"a"}, next: aws.String("tok-2")},
		"tok-2": {errs: []error{throttled}, items: []string{"b"}},
	}}
	items, complete, err := awsclient.PageAll(context.Background(), 10, s.next)
	if err != nil {
		t.Fatalf("err = %v, want nil after the retry succeeded", err)
	}
	if !complete {
		t.Error("complete = false, want true")
	}
	if !slices.Equal(items, []string{"a", "b"}) {
		t.Errorf("items = %v, want [a b]", items)
	}
	if n := s.callsFor("tok-2"); n != 2 {
		t.Errorf("throttled page requested %d times, want 2 (one throttled, one served)", n)
	}
}
