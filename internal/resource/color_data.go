package resource

func init() {
	colorRegistry["glue"] = func(_ Resource) Color { return ColorHealthy }
	colorRegistry["athena"] = colorAthena
}

func colorAthena(r Resource) Color {
	switch r.Fields["state"] {
	case "ENABLED":
		return ColorHealthy
	case "DISABLED":
		return ColorWarning
	}
	return ColorHealthy
}
