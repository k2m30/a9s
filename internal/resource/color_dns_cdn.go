package resource

import (
	"fmt"
	"strconv"
)


func init() {
	colorRegistry["r53"] = colorR53
	colorRegistry["cf"] = colorCF
	colorRegistry["acm"] = colorACM
	colorRegistry["apigw"] = func(_ Resource) Color { return ColorHealthy }
}

func colorR53(r Resource) Color {
	s := r.Fields["record_count"]
	if s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n <= 2 {
			return ColorWarning
		}
	}
	return ColorHealthy
}

func colorCF(r Resource) Color {
	if r.Fields["enabled"] == "false" {
		return ColorDim
	}
	switch r.Fields["status"] {
	case "Deployed":
		return ColorHealthy
	case "InProgress":
		return ColorWarning
	}
	return ColorHealthy
}

func colorACM(r Resource) Color {
	switch r.Fields["status"] {
	case "ISSUED":
		dl := r.Fields["days_left"]
		if dl == "expired" {
			return ColorBroken
		}
		if dl != "" {
			var n int
			if _, err := fmt.Sscanf(dl, "%d days", &n); err == nil {
				if n < 7 {
					return ColorBroken
				}
				if n < 30 {
					return ColorWarning
				}
			}
		}
		if r.Fields["in_use"] == "false" {
			return ColorWarning
		}
		return ColorHealthy
	case "PENDING_VALIDATION":
		return ColorWarning
	case "EXPIRED", "REVOKED", "FAILED", "VALIDATION_TIMED_OUT":
		return ColorBroken
	case "INACTIVE":
		return ColorDim
	}
	return ColorHealthy
}
