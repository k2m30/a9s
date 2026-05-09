package resource

import (
	"strings"
	"time"
)

func init() {
	colorRegistry["dbi"] = colorDBI
	colorRegistry["s3"] = colorS3
	colorRegistry["redis"] = colorRedis
	colorRegistry["dbc"] = colorDBC
	colorRegistry["ddb"] = colorDDB
	colorRegistry["opensearch"] = colorOpenSearch
	colorRegistry["redshift"] = colorRedshift
	colorRegistry["efs"] = colorEFS
	colorRegistry["dbi-snap"] = colorDBISnap
	colorRegistry["dbc-snap"] = colorDBCSnap
}

func colorDBI(r Resource) Color {
	status := r.Fields["status"]
	stripped := StripFindingSuffix(status)
	switch stripped {
	case "failed", "storage-full", "restore-error", "stopped",
		"incompatible-network", "incompatible-option-group",
		"incompatible-parameters", "incompatible-restore",
		"encryption key unavailable":
		return ColorBroken
	}
	if strings.HasPrefix(stripped, "incompatible-") || strings.HasPrefix(stripped, "inaccessible-") {
		return ColorBroken
	}
	switch stripped {
	case "no automated backups", "publicly accessible",
		"unencrypted storage", "deletion protection off":
		return ColorWarning
	}
	if stripped != "" && stripped != "available" && stripped != "maintenance scheduled" {
		if strings.Contains(stripped, ":") {
			return ColorWarning
		}
		switch stripped {
		case "creating", "modifying", "backing-up", "rebooting",
			"renaming", "resetting-master-credentials", "starting",
			"stopping", "upgrading", "maintenance",
			"configuring-enhanced-monitoring", "configuring-iam-database-auth",
			"configuring-log-exports", "converting-to-vpc", "moving-to-vpc",
			"storage-optimization", "deleting":
			return ColorWarning
		}
	}
	base := ColorHealthy
	if r.Fields["publicly_accessible"] == "true" {
		if base < ColorWarning {
			base = ColorWarning
		}
	}
	if r.Fields["storage_encrypted"] == "false" {
		if base < ColorWarning {
			base = ColorWarning
		}
	}
	if r.Fields["deletion_protection"] == "false" {
		if base < ColorWarning {
			base = ColorWarning
		}
	}
	if r.Fields["backup_retention_period"] == "0" {
		if base < ColorWarning {
			base = ColorWarning
		}
	}
	return base
}

func colorS3(_ Resource) Color { return ColorHealthy }

func colorRedis(r Resource) Color {
	phrase := StripFindingSuffix(r.Fields["status"])
	if phrase == "deleted" {
		return ColorDim
	}
	switch phrase {
	case "create failed — see events":
		return ColorBroken
	}
	switch phrase {
	case "creating — new group",
		"modifying — config change",
		"snapshotting — backup running",
		"deleting — teardown",
		"multi-AZ without auto-failover":
		return ColorWarning
	}
	if strings.HasPrefix(phrase, "shard ") {
		return ColorWarning
	}
	return ColorHealthy
}

func colorDBC(r Resource) Color {
	phrase := StripFindingSuffix(r.Fields["status"])
	switch phrase {
	case "":
		return ColorHealthy
	case "failed: cluster operation",
		"encryption key unreachable",
		"parameter group incompatible",
		"no writer: reads only":
		return ColorBroken
	case "delete-protection off",
		"not encrypted at rest",
		"no automated backups":
		return ColorWarning
	case "maintenance overdue":
		return ColorHealthy
	}
	if strings.HasSuffix(phrase, ": in progress") {
		return ColorWarning
	}
	return ColorHealthy
}

func colorDDB(r Resource) Color {
	phrase := StripFindingSuffix(r.Fields["status"])
	switch phrase {
	case "":
		return ColorHealthy
	case "creating", "updating", "deleting", "archiving":
		return ColorWarning
	case "kms key inaccessible", "archived: kms key lost":
		return ColorBroken
	case "PITR off":
		return ColorHealthy
	}
	return ColorHealthy
}

func colorOpenSearch(r Resource) Color {
	if r.Fields["deleted"] == "true" {
		return ColorDim
	}
	status := r.Status
	if status == "" {
		status = r.Fields["status"]
	}
	stripped := StripFindingSuffix(status)
	if strings.HasPrefix(stripped, "isolated:") || r.Fields["domain_processing_status"] == "Isolated" {
		return ColorBroken
	}
	if strings.HasPrefix(stripped, "processing:") ||
		r.Fields["processing"] == "true" ||
		r.Fields["upgrade_processing"] == "true" {
		return ColorWarning
	}
	return ColorHealthy
}

func colorRedshift(r Resource) Color {
	phrase := StripFindingSuffix(r.Fields["status"])
	if phrase == "" {
		phrase = StripFindingSuffix(r.Status)
	}
	switch phrase {
	case "unavailable", "failed":
		return ColorBroken
	}
	if len(phrase) >= len("broken:") && phrase[:len("broken:")] == "broken:" {
		return ColorBroken
	}
	var base Color
	switch r.Fields["cluster_status"] {
	case "available":
		base = ColorHealthy
	case "creating", "modifying", "resizing", "rebooting", "renaming", "deleting":
		base = ColorWarning
	case "incompatible-hsm", "incompatible-network", "incompatible-parameters",
		"incompatible-restore", "hardware-failure", "storage-full":
		base = ColorBroken
	default:
		base = ColorHealthy
	}
	if base == ColorBroken {
		return ColorBroken
	}
	switch r.Fields["cluster_availability_status"] {
	case "Unavailable", "Failed":
		return ColorBroken
	case "Maintenance", "Modifying":
		if base == ColorHealthy {
			base = ColorWarning
		}
	}
	if base == ColorBroken {
		return ColorBroken
	}
	switch phrase {
	case "pending change queued", "maintenance deferred",
		"maintenance", "modifying",
		"publicly accessible", "unencrypted at rest":
		if base == ColorHealthy {
			base = ColorWarning
		}
	}
	if r.Fields["publicly_accessible"] == "true" && base == ColorHealthy {
		base = ColorWarning
	}
	if r.Fields["encrypted"] == "false" && base == ColorHealthy {
		base = ColorWarning
	}
	return base
}

func colorEFS(r Resource) Color {
	status := r.Fields["status"]
	if status == "" {
		status = r.Status
	}
	phrase := StripFindingSuffix(status)
	switch phrase {
	case "":
		return ColorHealthy
	case "error", "no mount targets", "mount target down":
		return ColorBroken
	case "creating", "updating", "deleting":
		return ColorWarning
	default:
		return ColorHealthy
	}
}

func colorDBISnap(r Resource) Color {
	phrase := StripFindingSuffix(r.Fields["status"])
	if phrase == "failed" {
		return ColorBroken
	}
	if strings.HasPrefix(phrase, "incompatible-") {
		return ColorBroken
	}
	if phrase == "" || phrase == "available" {
		if r.Fields["encrypted"] == "false" {
			return ColorWarning
		}
		return ColorHealthy
	}
	return ColorWarning
}

func colorDBCSnap(r Resource) Color {
	phrase := StripFindingSuffix(r.Fields["status"])
	if phrase == "failed" {
		return ColorBroken
	}
	if strings.HasPrefix(phrase, "incompatible-") {
		return ColorBroken
	}
	if phrase != "" && phrase != "available" {
		return ColorWarning
	}
	if r.Fields["storage_encrypted"] == "false" {
		return ColorWarning
	}
	if r.Fields["snapshot_type"] == "manual" {
		if ts, err := time.Parse("2006-01-02 15:04", r.Fields["snapshot_create_time"]); err == nil {
			if time.Since(ts) > 365*24*time.Hour {
				return ColorWarning
			}
		}
	}
	return ColorHealthy
}
