package resource

import "strings"

func init() {
	colorRegistry["cfn"] = colorCFN
	colorRegistry["pipeline"] = func(_ Resource) Color { return ColorHealthy }
	colorRegistry["cb"] = func(_ Resource) Color { return ColorHealthy }
	colorRegistry["ecr"] = func(_ Resource) Color { return ColorHealthy }
	colorRegistry["codeartifact"] = func(_ Resource) Color { return ColorHealthy }
}

func colorCFN(r Resource) Color {
	return cfnStackColor(r.Fields["status"])
}

// cfnStackColor maps CloudFormation stack status strings to a Color.
func cfnStackColor(status string) Color {
	switch status {
	case "CREATE_COMPLETE", "UPDATE_COMPLETE", "IMPORT_COMPLETE":
		return ColorHealthy
	case "DELETE_COMPLETE":
		return ColorDim
	case "ROLLBACK_COMPLETE", "ROLLBACK_FAILED",
		"UPDATE_ROLLBACK_COMPLETE", "UPDATE_ROLLBACK_FAILED",
		"IMPORT_ROLLBACK_COMPLETE", "IMPORT_ROLLBACK_FAILED":
		return ColorBroken
	}
	if strings.HasSuffix(status, "_IN_PROGRESS") {
		return ColorWarning
	}
	if strings.HasSuffix(status, "_FAILED") {
		return ColorBroken
	}
	return ColorHealthy
}
