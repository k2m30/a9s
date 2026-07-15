// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

// CodePipeline stage-action findings emitted by FetchPipelineStages. A
// row's own ActionExecutionStatus of Failed is broken; Succeeded/InProgress/
// Abandoned/unset carry no finding.
const (
	CodePipelineActionFailed domain.FindingCode = "pipeline-stage.broken.failed"
)
