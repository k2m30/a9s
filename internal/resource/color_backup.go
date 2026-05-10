package resource

func init() {
	colorRegistry["backup"] = func(_ Resource) Color { return ColorHealthy }
}
