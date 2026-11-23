package main

import (
	"encoding/json"
	"slices"
	"strconv"
)

// mirror of cmd/snapshot's normalized s3 data (the data contract, redeclared so
// the checklist generator depends on no collector internals).
type s3Read struct {
	Outcome   string `json:"outcome"`
	ErrorCode string `json:"error_code"`
}

type s3GroupGrant struct {
	URI string `json:"uri"`
}

type s3Data struct {
	Buckets []struct {
		Name string `json:"name"`
		PAB  struct {
			s3Read
			BlockPublicAcls       bool `json:"block_public_acls"`
			IgnorePublicAcls      bool `json:"ignore_public_acls"`
			BlockPublicPolicy     bool `json:"block_public_policy"`
			RestrictPublicBuckets bool `json:"restrict_public_buckets"`
		} `json:"pab"`
		PolicyStatus struct {
			s3Read
			IsPublic bool `json:"is_public"`
		} `json:"policy_status"`
		ACL struct {
			s3Read
			GroupGrants []s3GroupGrant `json:"group_grants"`
		} `json:"acl"`
		Versioning struct {
			s3Read
			Status    string `json:"status"`
			MFADelete string `json:"mfa_delete"`
		} `json:"versioning"`
		Logging struct {
			s3Read
			Enabled bool `json:"enabled"`
		} `json:"logging"`
		Lifecycle struct {
			s3Read
			EnabledRules int `json:"enabled_rules"`
		} `json:"lifecycle"`
		ObjectLock struct {
			s3Read
			Enabled bool `json:"enabled"`
		} `json:"object_lock"`
	} `json:"buckets"`
}

// s3Finding is one docs/resources/s3.md §4 row.
type s3Finding struct {
	phrase string
	glyph  string
}

var (
	s3FindingPAB        = s3Finding{"public access block incomplete", "~"}
	s3FindingPublic     = s3Finding{"publicly accessible", "!"}
	s3FindingVersioning = s3Finding{"versioning off", "~"}
	s3FindingMFADelete  = s3Finding{"MFA delete off", "~"}
	s3FindingLogging    = s3Finding{"access logging off", "~"}
	s3FindingLifecycle  = s3Finding{"no lifecycle rules", "~"}
	s3FindingObjectLock = s3Finding{"object lock off", "~"}
)

// s3Unreachable are the codes by which S3 answers a per-bucket read for a
// bucket deleted after the list, or one sent to another region's endpoint:
// the bucket's posture is not known.
var s3Unreachable = []string{"NoSuchBucket", "NotFound", "PermanentRedirect", "IllegalLocationConstraintException"}

// s3PublicGroups are the ACL grantee groups that reach the whole internet.
var s3PublicGroups = []string{
	"http://acs.amazonaws.com/groups/global/AllUsers",
	"http://acs.amazonaws.com/groups/global/AuthenticatedUsers",
}

// checklistS3 derives the expected s3 root+list checklist from raw bucket facts.
//
// docs/resources/s3.md and docs/attention-signals.md rules applied here:
//   - no Wave-1 signal → every row is Healthy (green); Wave-2 findings do not
//     color a row.
//   - Wave-2 findings, judged per bucket from the recorded answers: a read
//     that answered that the bucket is gone or in another region leaves the
//     bucket not inspected with no finding; any other read that failed, or
//     that the snapshot does not record, leaves its own findings unjudged and
//     the bucket not inspected. The public access block is read when it was
//     returned or AWS stated there is none; the public finding is judged only
//     against a block that was read.
//   - a flagged row carries the glyph of its most severe finding and the
//     Status phrase of the first such finding, with " (+N)" for the others.
//   - the issue badge counts the rows with a "!" finding; it is a lower bound
//     when the list is longer than a page or a bucket left not inspected
//     carries no "!" finding.
func checklistS3(section json.RawMessage) (Checklist, error) {
	var d s3Data
	if err := json.Unmarshal(section, &d); err != nil {
		return Checklist{}, err
	}

	total := len(d.Buckets)
	shown := min(total, pageSize)

	colors := map[string]int{"healthy": 0, "warning": 0, "broken": 0, "dim": 0}
	decorators := map[string]int{"!": 0, "~": 0}
	var flagged []FlaggedRow
	issues, uncounted := 0, 0

	for _, b := range d.Buckets[:shown] {
		colors["healthy"]++
		reads := []s3Read{b.PAB.s3Read, b.PolicyStatus.s3Read, b.ACL.s3Read, b.Versioning.s3Read, b.Logging.s3Read, b.Lifecycle.s3Read, b.ObjectLock.s3Read}
		if slices.ContainsFunc(reads, func(r s3Read) bool { return slices.Contains(s3Unreachable, r.ErrorCode) }) {
			uncounted++
			continue
		}
		answered := func(r s3Read) bool { return r.Outcome == "ok" || r.Outcome == "none" }

		var found []s3Finding
		pabRead := b.PAB.Outcome == "configured" || b.PAB.Outcome == "none"
		pab := b.PAB
		if pabRead && (pab.Outcome == "none" || !pab.BlockPublicAcls || !pab.IgnorePublicAcls || !pab.BlockPublicPolicy || !pab.RestrictPublicBuckets) {
			found = append(found, s3FindingPAB)
		}
		publicPolicy := b.PolicyStatus.Outcome == "ok" && b.PolicyStatus.IsPublic && !pab.RestrictPublicBuckets
		publicACL := b.ACL.Outcome == "ok" && !pab.IgnorePublicAcls && slices.ContainsFunc(b.ACL.GroupGrants, func(g s3GroupGrant) bool {
			return slices.Contains(s3PublicGroups, g.URI)
		})
		if pabRead && (publicPolicy || publicACL) {
			found = append(found, s3FindingPublic)
		}
		if b.Versioning.Outcome == "ok" {
			switch {
			case b.Versioning.Status != "Enabled":
				found = append(found, s3FindingVersioning)
			case b.Versioning.MFADelete != "Enabled":
				found = append(found, s3FindingMFADelete)
			}
		}
		if b.Logging.Outcome == "ok" && !b.Logging.Enabled {
			found = append(found, s3FindingLogging)
		}
		if answered(b.Lifecycle.s3Read) && b.Lifecycle.EnabledRules == 0 {
			found = append(found, s3FindingLifecycle)
		}
		if answered(b.ObjectLock.s3Read) && !b.ObjectLock.Enabled {
			found = append(found, s3FindingObjectLock)
		}

		public := slices.Contains(found, s3FindingPublic)
		if public {
			issues++
		} else if slices.ContainsFunc(reads, func(r s3Read) bool { return !answered(r) && r.Outcome != "configured" }) {
			uncounted++
		}
		if len(found) == 0 {
			continue
		}
		top := found[0]
		if public {
			top = s3FindingPublic
		}
		status := top.phrase
		if len(found) > 1 {
			status += " (+" + strconv.Itoa(len(found)-1) + ")"
		}
		decorators[top.glyph]++
		flagged = append(flagged, FlaggedRow{Name: b.Name, Decorator: top.glyph, Status: status})
	}

	return Checklist{
		Type: "s3",
		Menu: ChecklistMenu{
			Display:         "S3 Buckets",
			Availability:    shown,
			AvailTruncated:  total > pageSize,
			Issues:          issues,
			IssuesTruncated: total > enrichmentCap || uncounted > 0,
		},
		List: ChecklistList{
			Columns:    []string{"Bucket Name", "Region", "Creation Date", "Status"},
			Jargon:     []string{"Public Access", "CIS", "Flags", "Policy", "Issues", "NOBKP", "UNENC", "PUB", "NOPROT"},
			Shown:      shown,
			Truncated:  total > pageSize,
			Colors:     colors,
			Decorators: decorators,
			Flagged:    flagged,
		},
	}, nil
}
