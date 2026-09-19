package unit

// Session.Session's DetailDocCache
// (core/session/session.go) is the session-scoped cache behind
// DetailEnrichmentCtx for the sfn/cfn on-demand detail enrichers. New() seeds
// a fresh instance and Rotate() swaps in another, so documents fetched under
// one profile/region cannot leak into the next.

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/session"
)

func TestSession_New_DetailDocCacheInitialized(t *testing.T) {
	s := session.New()

	if s.DetailDocCache == nil {
		t.Fatal("session.New().DetailDocCache is nil — Session must construct a DetailDocCache")
	}

	if got := s.DetailDocCache.Get("sfn:arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"); got != nil {
		t.Errorf("fresh DetailDocCache should be empty, got %v", got)
	}
	s.DetailDocCache.Set("sfn:probe", "value")
	if got := s.DetailDocCache.Get("sfn:probe"); got != "value" {
		t.Errorf("DetailDocCache.Set/Get roundtrip failed on session-owned cache, got %v", got)
	}
}

func TestSession_Rotate_ReplacesDetailDocCache(t *testing.T) {
	s := session.New()
	before := s.DetailDocCache
	before.Set("sfn:arn:aws:states:us-east-1:123456789012:stateMachine:order-processing", map[string]any{"stale": true})

	s.Rotate()

	if s.DetailDocCache == nil {
		t.Fatal("session.Rotate() left DetailDocCache nil")
	}
	if s.DetailDocCache == before {
		t.Fatal("session.Rotate() must swap in a NEW DetailDocCache instance, not reuse the previous pointer")
	}
	if got := s.DetailDocCache.Get("sfn:arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"); got != nil {
		t.Errorf("post-Rotate DetailDocCache must not carry over documents from the previous profile/region, got %v", got)
	}
}

var _ *awsclient.DetailDocCache = (&session.Session{}).DetailDocCache
