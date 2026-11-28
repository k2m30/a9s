package aws

import (
	"context"
	"testing"
	"time"
)

// A caller that joined a shared read keeps its own context: the leader's
// cancellation is not its answer.
func TestT5Dev_JoinedCallerSurvivesTheLeadersCancellation(t *testing.T) {
	var c PolicyDocumentCache
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	started := make(chan struct{})
	reads := 0
	read := func(ctx context.Context) (any, error) {
		reads++
		if reads == 1 {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return "document", nil
	}
	leaderDone := make(chan error, 1)
	go func() {
		_, err := c.load(leaderCtx, "managed:arn", read)
		leaderDone <- err
	}()
	<-started
	joined := make(chan struct{})
	var doc any
	var err error
	go func() {
		defer close(joined)
		doc, err = c.load(context.Background(), "managed:arn", read)
	}()
	time.Sleep(20 * time.Millisecond)
	cancelLeader()
	<-joined
	if leaderErr := <-leaderDone; leaderErr == nil {
		t.Error("the cancelled leader got an answer")
	}
	if err != nil || doc != "document" {
		t.Errorf("joined caller: doc %v, err %v; want the document", doc, err)
	}
}
