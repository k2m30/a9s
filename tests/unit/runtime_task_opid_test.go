package unit

// runtime_task_opid_test.go — coverage for runtime.TaskOpID (core/runtime/tasks.go),
// the payload-op-ID derivation helper behind core/web's op-aware background-task
// admission (server.go's drainBackgroundTasks): a same-key task already draining
// is skipped UNLESS the incoming task's TaskOpID is strictly newer than the
// recorded one, so a refresh dispatched while an earlier open's related-check/
// enrich fan-out is still in flight supersedes it instead of being silently
// dropped for sharing the same TaskKey.
//
// The admission DECISION itself (entry.inFlight bookkeeping, the running/opID
// comparison) lives in core/web's unexported sessionEntry/drainBackgroundTasks
// and is not reachable from tests/unit (external, unexported). TaskOpID is the
// one piece of that mechanism the coordinator's spec asked to be covered
// instead, since it landed as exported core/runtime API: every TaskRequest
// admission decision starts from whatever this function returns for a given
// payload, so a wrong return here (e.g. a payload type that should carry an op
// ID falling through to the zero default) would silently defeat the
// same-key-supersede logic for that payload's kind.
import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// TestAdmission_TaskOpID_ExtractsOpFromDetailScopedPayloads covers the two
// payload kinds TaskOpID knows about (EnrichDetailPayload, RelatedCheckPayload)
// — both DetailOperation-carrying kinds dispatched from the same detail-open/
// refresh entry point (Core.DetailOperationTasks) — and the two kinds it does
// not: a payload type with no Op field (FetchMorePayload) and a nil payload,
// both of which must return the zero Gen so a non-detail task kind falls back
// to plain same-key dedup (server.go's opID == 0 branch) rather than being
// mistaken for a superseded/superseding detail operation.
func TestAdmission_TaskOpID_ExtractsOpFromDetailScopedPayloads(t *testing.T) {
	const opID = domain.Gen(7)

	tests := []struct {
		name    string
		payload runtime.TaskPayload
		want    domain.Gen
	}{
		{
			name:    "EnrichDetailPayload returns its Op.ID",
			payload: runtime.EnrichDetailPayload{Op: runtime.DetailOperation{ID: opID}},
			want:    opID,
		},
		{
			name:    "RelatedCheckPayload returns its Op.ID",
			payload: runtime.RelatedCheckPayload{Op: runtime.DetailOperation{ID: opID}},
			want:    opID,
		},
		{
			name:    "a payload with no Op field returns the zero Gen",
			payload: runtime.FetchMorePayload{ContinuationToken: "tok"},
			want:    0,
		},
		{
			name:    "a nil payload returns the zero Gen",
			payload: nil,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runtime.TaskOpID(tt.payload); got != tt.want {
				t.Errorf("TaskOpID(%#v) = %d, want %d", tt.payload, got, tt.want)
			}
		})
	}
}

// TestAdmission_TaskOpID_DistinguishesOlderVsNewerOp pins the ordering
// comparison drainBackgroundTasks actually performs (opID <= running means
// "still stale, skip"; a strictly greater opID supersedes): two
// RelatedCheckPayloads carrying different DetailOperation IDs from the same
// detail-open lineage must compare in the same order their IDs do, since
// domain.Gen's whole contract (session.DetailOpGen.Bump()) is a monotonic
// per-session counter.
func TestAdmission_TaskOpID_DistinguishesOlderVsNewerOp(t *testing.T) {
	older := runtime.RelatedCheckPayload{Op: runtime.DetailOperation{ID: domain.Gen(3)}}
	newer := runtime.RelatedCheckPayload{Op: runtime.DetailOperation{ID: domain.Gen(4)}}

	gotOlder := runtime.TaskOpID(older)
	gotNewer := runtime.TaskOpID(newer)

	if !(gotOlder < gotNewer) {
		t.Errorf("TaskOpID(older)=%d, TaskOpID(newer)=%d — want older < newer so a running task's recorded opID is correctly superseded only by a strictly newer refresh", gotOlder, gotNewer)
	}
	if gotOlder <= 0 {
		t.Errorf("TaskOpID(older) = %d, want > 0 — a real DetailOperation ID must never be mistaken for the opID==0 (non-detail-task) fallback branch", gotOlder)
	}
}
