package ctdetail

// Summarizer is the per-service request-parameters summarizer signature.
// See specs/013-ct-event-detail-v2/contracts/ctdetail-api.md for the contract.
type Summarizer func(eventName string, params map[string]any) []Row

// summarizerByService is the registry. Populated via init() in summarize_<service>.go files.
var summarizerByService = map[string]Summarizer{}

// RegisterSummarizer registers a per-service summarizer.
// Duplicate registration for the same eventSource panics during init.
func RegisterSummarizer(eventSource string, fn Summarizer) {
	if _, exists := summarizerByService[eventSource]; exists {
		panic("ctdetail: duplicate summarizer registration for " + eventSource)
	}
	summarizerByService[eventSource] = fn
}
