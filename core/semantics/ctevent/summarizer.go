package ctevent

// Summarizer is the per-service request-parameters summarizer signature.
// See specs/013-ct-event-detail-v2/contracts/ctevent-api.md for the contract.
type Summarizer func(eventName string, params map[string]any) []Row

// summarizerByService maps a CloudTrail eventSource to its service-specific
// request summarizer. Static and declarative — adding a service means adding
// one entry here plus the Summarize<Svc> function. Duplicate keys are a compile
// error, so no runtime guard is needed. SummarizeGeneric is the fallback and is
// intentionally NOT in this map (see sections.go).
var summarizerByService = map[string]Summarizer{
	"iam.amazonaws.com": SummarizeIAM,
	"s3.amazonaws.com":  SummarizeS3,
	"ec2.amazonaws.com": SummarizeEC2,
}
