// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

// CodeBuild build-state findings emitted by FetchCBBuilds. wave1 Findings are
// the sole severity signal for this fetcher: the list view renders the row
// red/yellow/dim from the Finding alone.
const (
	CodeCBBuildFailed     domain.FindingCode = "cb-build.broken.failed"
	CodeCBBuildFault      domain.FindingCode = "cb-build.broken.fault"
	CodeCBBuildTimedOut   domain.FindingCode = "cb-build.broken.timed_out"
	CodeCBBuildInProgress domain.FindingCode = "cb-build.warn.in_progress"
	CodeCBBuildStopped    domain.FindingCode = "cb-build.dim.stopped"
)
