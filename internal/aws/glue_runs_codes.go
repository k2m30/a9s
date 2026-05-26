package aws

import "github.com/k2m30/a9s/v3/internal/domain"

// Glue job-run findings emitted by FetchGlueJobRuns. FAILED/TIMEOUT/ERROR
// classify as broken; RUNNING/STARTING/STOPPING/WAITING as warn; STOPPED as
// dim. SUCCEEDED rows emit no finding and render healthy.
const (
	CodeGlueRunFailed   domain.FindingCode = "glue-run.broken.failed"
	CodeGlueRunTimeout  domain.FindingCode = "glue-run.broken.timeout"
	CodeGlueRunError    domain.FindingCode = "glue-run.broken.error"
	CodeGlueRunExpired  domain.FindingCode = "glue-run.broken.expired"
	CodeGlueRunRunning  domain.FindingCode = "glue-run.warn.running"
	CodeGlueRunStarting domain.FindingCode = "glue-run.warn.starting"
	CodeGlueRunStopping domain.FindingCode = "glue-run.warn.stopping"
	CodeGlueRunWaiting  domain.FindingCode = "glue-run.warn.waiting"
	CodeGlueRunStopped  domain.FindingCode = "glue-run.dim.stopped"
)
