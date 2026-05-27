package aws

import "github.com/k2m30/a9s/v3/internal/domain"

// CodeBuild build-state findings emitted by FetchCBBuilds. Severity carries
// the color signal so the list view can render the row red/yellow/dim from
// the wave1 Finding alone — no per-status Color fallback against a status
// string (AS-1393 made wave1 Findings the sole severity signal for this
// fetcher).
const (
	CodeCBBuildFailed     domain.FindingCode = "cb-build.broken.failed"
	CodeCBBuildFault      domain.FindingCode = "cb-build.broken.fault"
	CodeCBBuildTimedOut   domain.FindingCode = "cb-build.broken.timed_out"
	CodeCBBuildInProgress domain.FindingCode = "cb-build.warn.in_progress"
	CodeCBBuildStopped    domain.FindingCode = "cb-build.dim.stopped"
)
