package ctdetail

import (
	"sort"
)

func init() {
	RegisterSummarizer("iam.amazonaws.com", SummarizeIAM)
}

// SummarizeIAM summarizes the REQUEST section for IAM events.
// It receives cleaned params with TARGET-lifted fields already removed.
// policyArn fields are marked navigable to "policy" (registered ShortName in
// internal/resource/types_security.go).
func SummarizeIAM(_ string, params map[string]any) []Row {
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
		row := Row{
			Key:   k,
			Value: renderGenericValue(v),
		}
		if k == "policyArn" {
			if arn, ok := v.(string); ok {
				row.Value = arn
				row.IsNavigable = true
				row.TargetType = "policy"
			}
		}
		rows = append(rows, row)
	}
	return rows
}
