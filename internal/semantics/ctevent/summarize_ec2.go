package ctevent

import (
	"fmt"
	"sort"
)

// SummarizeEC2 summarizes the REQUEST section for EC2 events.
// It receives cleaned params with TARGET-lifted fields already removed.
//
// Navigable fields:
//   - imageId    → "ami"
//   - subnetId   → "subnet"
//   - vpcId      → "vpc"
//   - securityGroupIds items → "sg" (one row per ID)
func SummarizeEC2(_ string, params map[string]any) []Row {
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

		switch k {
		case "securityGroupIds":
			// Emit one navigable row per SG ID.
			if slice, ok := v.([]any); ok {
				for i, item := range slice {
					sgID := fmt.Sprintf("%v", item)
					rows = append(rows, Row{
						Key:         fmt.Sprintf("securityGroupIds[%d]", i),
						Value:       sgID,
						IsNavigable: true,
						TargetType:  "sg",
					})
				}
			} else {
				rows = append(rows, Row{
					Key:   k,
					Value: renderGenericValue(v),
				})
			}

		case "imageId":
			if s, ok := v.(string); ok {
				rows = append(rows, Row{
					Key:         k,
					Value:       s,
					IsNavigable: true,
					TargetType:  "ami",
				})
			} else {
				rows = append(rows, Row{Key: k, Value: renderGenericValue(v)})
			}

		case "subnetId":
			if s, ok := v.(string); ok {
				rows = append(rows, Row{
					Key:         k,
					Value:       s,
					IsNavigable: true,
					TargetType:  "subnet",
				})
			} else {
				rows = append(rows, Row{Key: k, Value: renderGenericValue(v)})
			}

		case "vpcId":
			if s, ok := v.(string); ok {
				rows = append(rows, Row{
					Key:         k,
					Value:       s,
					IsNavigable: true,
					TargetType:  "vpc",
				})
			} else {
				rows = append(rows, Row{Key: k, Value: renderGenericValue(v)})
			}

		default:
			rows = append(rows, Row{
				Key:   k,
				Value: renderGenericValue(v),
			})
		}
	}
	return rows
}
