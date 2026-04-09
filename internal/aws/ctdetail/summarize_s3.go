package ctdetail

import (
	"sort"
)

func init() {
	RegisterSummarizer("s3.amazonaws.com", SummarizeS3)
}

// SummarizeS3 summarizes the REQUEST section for S3 events.
// It receives cleaned params (bucketName and key already lifted by ExtractTarget).
// All residual fields are emitted as non-navigable rows — bucketName and key are
// handled as TARGET upstream and must not be re-extracted here.
func SummarizeS3(_ string, params map[string]any) []Row {
	rows := []Row{}
	if len(params) == 0 {
		return rows
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := params[k]
		rows = append(rows, Row{
			Key:   k,
			Value: renderGenericValue(v),
		})
	}
	return rows
}
