package resource

import "strings"

func init() {
	colorRegistry["sqs"] = func(_ Resource) Color { return ColorHealthy }
	colorRegistry["sns"] = func(_ Resource) Color { return ColorHealthy }
	colorRegistry["sns-sub"] = colorSNSSub
	colorRegistry["eb-rule"] = colorEBRule
	colorRegistry["kinesis"] = colorKinesis
	colorRegistry["msk"] = colorMSK
	colorRegistry["sfn"] = func(_ Resource) Color { return ColorHealthy }
	colorRegistry["ses"] = colorSES
}

func colorSNSSub(r Resource) Color {
	switch r.Fields["subscription_arn"] {
	case "PendingConfirmation":
		return ColorWarning
	case "Deleted":
		return ColorDim
	default:
		return ColorHealthy
	}
}

func colorEBRule(r Resource) Color {
	switch strings.ToUpper(r.Fields["state"]) {
	case "ENABLED", "ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS":
		return ColorHealthy
	case "DISABLED":
		return ColorDim
	}
	return ColorHealthy
}

func colorKinesis(r Resource) Color {
	switch r.Fields["stream_status"] {
	case "ACTIVE":
		return ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return ColorWarning
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		return ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return ColorWarning
	}
	return ColorHealthy
}

func colorMSK(r Resource) Color {
	switch r.Fields["state"] {
	case "ACTIVE":
		return ColorHealthy
	case "CREATING", "UPDATING", "MAINTENANCE", "REBOOTING_BROKER", "HEALING":
		return ColorWarning
	case "FAILED":
		return ColorBroken
	}
	return ColorHealthy
}

func colorSES(r Resource) Color {
	phrase := StripFindingSuffix(r.Fields["status"])
	switch phrase {
	case "verification failed", "verify: temp failure", "verification not started",
		"account SHUTDOWN", "account PROBATION":
		return ColorBroken
	case "pending verification", "sending disabled":
		return ColorWarning
	}
	return ColorHealthy
}
