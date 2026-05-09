package resource

import "strconv"

func init() {
	colorRegistry["elb"] = colorELB
	colorRegistry["tg"] = colorTG
	colorRegistry["sg"] = colorSG
	colorRegistry["vpc"] = colorVPC
	colorRegistry["subnet"] = colorSubnet
	colorRegistry["rtb"] = colorRTB
	colorRegistry["nat"] = colorNAT
	colorRegistry["igw"] = colorIGW
	colorRegistry["eip"] = colorEIP
	colorRegistry["vpce"] = colorVPCE
	colorRegistry["tgw"] = colorTGW
	colorRegistry["eni"] = colorENI
}

func colorELB(r Resource) Color {
	if c, ok := ColorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "active", "":
		return ColorHealthy
	case "provisioning", "active_impaired":
		return ColorWarning
	case "failed":
		return ColorBroken
	}
	return ColorHealthy
}

func colorTG(_ Resource) Color { return ColorHealthy }

func colorSG(r Resource) Color {
	if r.Fields["wide_open"] == "true" {
		return ColorBroken
	}
	count, _ := strconv.Atoi(r.Fields["dangerous_open_count"])
	if count > 0 {
		return ColorBroken
	}
	return ColorHealthy
}

func colorVPC(r Resource) Color {
	if c, ok := ColorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "available", "":
		return ColorHealthy
	case "pending":
		return ColorWarning
	}
	return ColorHealthy
}

func colorSubnet(r Resource) Color {
	if c, ok := ColorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "available", "":
		return ColorHealthy
	case "pending":
		return ColorWarning
	case "unavailable", "failed", "failed-insufficient-capacity":
		return ColorBroken
	}
	return ColorHealthy
}

func colorRTB(r Resource) Color {
	blackhole, _ := strconv.Atoi(r.Fields["blackhole_routes_count"])
	if blackhole > 0 {
		return ColorBroken
	}
	assoc, _ := strconv.Atoi(r.Fields["associations_count"])
	if assoc == 0 && r.Fields["is_main"] != "true" {
		return ColorWarning
	}
	return ColorHealthy
}

func colorNAT(r Resource) Color {
	if c, ok := ColorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "available", "":
		return ColorHealthy
	case "pending", "deleting":
		return ColorWarning
	case "failed":
		return ColorBroken
	case "deleted":
		return ColorDim
	}
	return ColorHealthy
}

func colorIGW(r Resource) Color {
	if c, ok := ColorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "attaching", "detaching":
		return ColorWarning
	}
	attachments, _ := strconv.Atoi(r.Fields["attachments_count"])
	if attachments == 0 {
		return ColorWarning
	}
	return ColorHealthy
}

func colorEIP(r Resource) Color {
	if c, ok := ColorFromWave1(r); ok {
		return c
	}
	if r.Fields["association_id"] == "" && r.Fields["instance_id"] == "" {
		return ColorWarning
	}
	return ColorHealthy
}

func colorVPCE(r Resource) Color {
	if c, ok := ColorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "Available", "":
		return ColorHealthy
	case "PendingAcceptance", "Pending", "Deleting":
		return ColorWarning
	case "Failed", "Rejected", "Expired", "Partial":
		return ColorBroken
	case "Deleted":
		return ColorDim
	}
	return ColorHealthy
}

func colorTGW(r Resource) Color {
	if c, ok := ColorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "available", "":
		return ColorHealthy
	case "pending", "modifying", "deleting":
		return ColorWarning
	case "failed":
		return ColorBroken
	case "deleted":
		return ColorDim
	}
	return ColorHealthy
}

func colorENI(r Resource) Color {
	if c, ok := ColorFromWave1(r); ok {
		return c
	}
	switch r.Fields["status"] {
	case "in-use":
		return ColorHealthy
	case "available":
		if r.Fields["requester_managed"] == "true" {
			return ColorHealthy
		}
		return ColorWarning
	case "attaching", "detaching":
		return ColorWarning
	}
	return ColorHealthy
}
