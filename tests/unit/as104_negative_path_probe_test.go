package unit_test

// AS-104 NEGATIVE-PATH PROBE — TEMPORARY, DO NOT MERGE.
//
// This test deliberately blocks the suite for 360 seconds so the new
// `test-budget` CI job (scripts/test-budget-gate.sh) observably fails on
// this commit and updates the sticky PR comment to ❌. The next commit
// reverts this file in full; the failing run URL is captured in PR #334's
// description as the recorded negative-path evidence per AS-111 / AS-104.

import (
	"testing"
	"time"
)

func TestAS104_NegativePathProbe_DoNotMerge(t *testing.T) {
	t.Parallel()
	time.Sleep(360 * time.Second)
}
