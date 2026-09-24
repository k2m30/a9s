// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// backup_match.go — the one AWS Backup coverage evaluator every backup pivot
// and the coverage join call.
package aws

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/semantics/selector"
)

// BackupPlanSelections returns the selections the fetcher read for a backup
// plan row, and whether it read all of them. A row with no selections read —
// one replayed from the disk cache, which does not keep RawStruct — is
// incomplete.
func BackupPlanSelections(r resource.Resource) ([]backuptypes.BackupSelection, bool) {
	row, ok := assertStruct[BackupPlanRow](r.RawStruct)
	if !ok {
		return nil, false
	}
	return row.Selections, row.selectionsComplete
}

// BackupSelectionCovers applies one selection as AWS Backup does:
//
//	(Resources match OR ListOfTags match) AND every Conditions clause AND NOT NotResources match
//
// where an empty Resources and an empty ListOfTags stand for every resource.
func BackupSelectionCovers(sel backuptypes.BackupSelection, arn string, tags map[string]string) bool {
	if backupARNsMatch(sel.NotResources, arn, false) {
		return false
	}
	if len(sel.Resources)+len(sel.ListOfTags) > 0 && !backupARNsMatch(sel.Resources, arn, true) &&
		!slices.ContainsFunc(sel.ListOfTags, func(c backuptypes.Condition) bool {
			return backupTagEquals(c.ConditionKey, c.ConditionValue, tags)
		}) {
		return false
	}
	c := sel.Conditions
	if c == nil {
		return true
	}
	for _, p := range c.StringEquals {
		if !backupTagEquals(p.ConditionKey, p.ConditionValue, tags) {
			return false
		}
	}
	for _, p := range c.StringNotEquals {
		if backupTagEquals(p.ConditionKey, p.ConditionValue, tags) {
			return false
		}
	}
	for _, p := range c.StringLike {
		if !backupTagLike(p, tags) {
			return false
		}
	}
	for _, p := range c.StringNotLike {
		if backupTagLike(p, tags) {
			return false
		}
	}
	return true
}

// BackupPlanCovers reports whether any selection of the plan covers the
// resource. known is false when the answer is "not covered" but a selection
// nobody read, a tag clause over tags nobody read, a tag clause AWS returned
// without its key or value, or a Region opt-in nobody read could still change
// it.
func BackupPlanCovers(plan resource.Resource, arn string, tags map[string]string, tagsKnown bool) (covered, known bool) {
	t := backupTarget{arn: arn, tags: tags}
	if !tagsKnown {
		t.unread = "resource tags"
	}
	covered, undecided := backupPlanVerdict(plan, t)
	return covered, covered || undecided == ""
}

// backupTarget is one resource as a backup selection is evaluated against it.
type backupTarget struct {
	arn string
	// engine is an RDS cluster's engine: Aurora, DocumentDB and Neptune
	// clusters share one ARN format and each has its own Region opt-in.
	engine string
	tags   map[string]string
	// unread names the read the target is missing: the call that returns its
	// tags, or what its ARN could not be built without. "" when nothing is.
	unread string
}

// withTags is t once its tags were read: a read that failed leaves t
// missing them.
func (t backupTarget) withTags(tags map[string]string, err error) backupTarget {
	if err == nil {
		t.tags, t.unread = tags, ""
	}
	return t
}

// backupPlanVerdict reports whether the plan covers the target and, when it
// does not, the check that could still change the answer ("" when none
// could).
//
// A selection takes a resource in unconditionally when a Resources pattern
// naming its resource type or its exact ARN matches; a match that rests only
// on "*", a service name or tags, and any match on an Aurora, DocumentDB or
// Neptune cluster, counts when the Region has the resource's type opted in
// (DescribeRegionSettings).
func backupPlanVerdict(plan resource.Resource, t backupTarget) (covered bool, undecided string) {
	row, ok := assertStruct[BackupPlanRow](plan.RawStruct)
	if !ok || !row.selectionsComplete {
		undecided = checkListIncomplete("backup")
	}
	for _, sel := range row.Selections {
		switch {
		case t.unread != "" && backupSelectionNeedsTags(sel, t.arn):
			undecided = cmp.Or(undecided, t.unread)
		case !BackupSelectionCovers(sel, t.arn, t.tags):
			if !backupSelectionFilled(sel) {
				undecided = cmp.Or(undecided, "GetBackupSelection")
			}
		case !backupNeedsOptIn(t) && slices.ContainsFunc(sel.Resources, func(p string) bool {
			return backupARNsMatch([]string{p}, t.arn, true) && backupPatternNamesType(p, t.arn)
		}):
			return true, ""
		default:
			in, check := backupOptedIn(row.optIn, t)
			if in {
				return true, ""
			}
			undecided = cmp.Or(undecided, check)
		}
	}
	return false, undecided
}

// backupPlansCovering returns the IDs of every plan that covers the resource,
// and the check that could still add a plan ("" when none could).
func backupPlansCovering(plans []resource.Resource, t backupTarget) (ids []string, undecided string) {
	for _, plan := range plans {
		covered, check := backupPlanVerdict(plan, t)
		if covered {
			ids = append(ids, plan.ID)
		}
		undecided = cmp.Or(undecided, check)
	}
	return ids, undecided
}

// backupPivot is every backup related-panel answer: the covering plans, a
// lower bound when some plan cannot be decided, and unknown when nothing is
// known to cover the resource and something might. An empty ARN decides
// nothing unless there is no plan to decide against.
func backupPivot(plans []resource.Resource, truncated bool, t backupTarget) resource.RelatedCheckResult {
	// A nil list is a list nobody read: whether a plan covers the target is
	// unanswered, not answered "none".
	if plans == nil {
		return NotRead("backup")
	}
	if t.arn == "" && len(plans) > 0 {
		return NotRead("backup")
	}
	ids, undecided := backupPlansCovering(plans, t)
	if undecided != "" && len(ids) == 0 {
		return NotRead("backup")
	}
	return relatedResultTrunc("backup", ids, truncated || undecided != "")
}

// backupNeedsOptIn reports whether AWS Backup includes the target only while
// its type is opted in, however a selection names it: Aurora, DocumentDB and
// Neptune clusters (Getting started, Service Opt-in). A cluster of unknown
// engine may be one of them.
func backupNeedsOptIn(t backupTarget) bool {
	return isClusterARN(t.arn) && (t.engine == "" || rdsClusterOptInType(t.engine) != "")
}

// backupPatternNamesType reports whether a Resources pattern that matches the
// ARN names the resource's type: the exact ARN, or a pattern that keeps the
// ARN up to its resource type ("arn:aws:ec2:*:*:volume/*"). "*" and a service
// name ("arn:aws:ec2:*") do not. An S3 bucket ARN has no resource-type
// segment, so a pattern over the bucket name names the type.
func backupPatternNamesType(pattern, ref string) bool {
	if !strings.Contains(pattern, "*") {
		return true
	}
	p, perr := arn.Parse(pattern)
	a, aerr := arn.Parse(ref)
	if perr != nil || aerr != nil {
		return false
	}
	return !strings.ContainsAny(a.Resource, "/:") || !strings.HasPrefix(p.Resource, "*")
}

// backupOptInTypes maps a resource ARN's service and resource type to its
// ResourceTypeOptInPreference key.
var backupOptInTypes = map[string]string{
	"dynamodb:table":                "DynamoDB",
	"ec2:instance":                  "EC2",
	"ec2:volume":                    "EBS",
	"elasticfilesystem:file-system": "EFS",
	"rds:db":                        "RDS",
	"s3:":                           "S3",
}

// checkBackupOptInType is the check a resource records when a9s cannot tell
// which Region opt-in applies to it.
const checkBackupOptInType = "backup opt-in type unknown"

// rdsClusterOptInType is the ResourceTypeOptInPreference key of an engine
// whose instances always run in a cluster, or "".
func rdsClusterOptInType(engine string) string {
	switch {
	case strings.HasPrefix(engine, "aurora"):
		return "Aurora"
	case engine == "docdb":
		return "DocumentDB"
	case engine == "neptune":
		return "Neptune"
	}
	return ""
}

// backupOptedIn reports whether the Region has the target's resource type
// opted in, or the check that stops it telling. A cluster of unknown engine
// is decided only when Aurora, DocumentDB and Neptune agree. An RDS Multi-AZ
// DB cluster (MySQL or PostgreSQL) is an Amazon RDS resource type, so the
// RDS opt-in governs it.
func backupOptedIn(optIn map[string]bool, t backupTarget) (bool, string) {
	if optIn == nil {
		return false, "DescribeRegionSettings"
	}
	var types []string
	switch {
	case !isClusterARN(t.arn):
		if a, err := arn.Parse(t.arn); err == nil {
			typ := ""
			if i := strings.IndexAny(a.Resource, "/:"); i >= 0 {
				typ = a.Resource[:i]
			}
			if key, ok := backupOptInTypes[a.Service+":"+typ]; ok {
				types = []string{key}
			}
		}
	case t.engine == "":
		types = []string{"Aurora", "DocumentDB", "Neptune"}
	case rdsClusterOptInType(t.engine) != "":
		types = []string{rdsClusterOptInType(t.engine)}
	case t.engine == "mysql" || t.engine == "postgres":
		types = []string{"RDS"}
	}
	if len(types) == 0 {
		return false, checkBackupOptInType
	}
	first := optIn[types[0]]
	for _, typ := range types {
		if on, ok := optIn[typ]; !ok || on != first {
			return false, checkBackupOptInType
		}
	}
	return first, ""
}

// backupSelectionNeedsTags reports whether the selection's verdict on the ARN
// turns on the resource's tags.
func backupSelectionNeedsTags(sel backuptypes.BackupSelection, arn string) bool {
	if backupARNsMatch(sel.NotResources, arn, false) {
		return false
	}
	inByARN := len(sel.Resources)+len(sel.ListOfTags) == 0 || backupARNsMatch(sel.Resources, arn, true)
	c := sel.Conditions
	if c == nil || len(c.StringEquals)+len(c.StringNotEquals)+len(c.StringLike)+len(c.StringNotLike) == 0 {
		return !inByARN && len(sel.ListOfTags) > 0
	}
	return inByARN || len(sel.ListOfTags) > 0
}

// backupSelectionFilled reports whether every tag clause of the selection
// arrived with both its key and its value. A clause missing either cannot be
// evaluated, and treating it as unmatched would narrow the selection — the
// direction that invents a "not covered" finding.
func backupSelectionFilled(sel backuptypes.BackupSelection) bool {
	for _, c := range sel.ListOfTags {
		if c.ConditionKey == nil || c.ConditionValue == nil {
			return false
		}
	}
	if c := sel.Conditions; c != nil {
		for _, p := range slices.Concat(c.StringEquals, c.StringNotEquals, c.StringLike, c.StringNotLike) {
			if p.ConditionKey == nil || p.ConditionValue == nil {
				return false
			}
		}
	}
	return true
}

// backupARNsMatch reports whether any selection pattern takes in the ARN. `?`,
// `[` and `]` are characters AWS Backup gives a meaning this evaluator does
// not model; a pattern carrying one matches as unmodelled says, which callers
// set to the answer that never invents a "not covered" finding.
func backupARNsMatch(patterns []string, arn string, unmodelled bool) bool {
	return slices.ContainsFunc(patterns, func(p string) bool {
		if strings.ContainsAny(p, "?[]") {
			return unmodelled
		}
		return selector.MatchARN(p, arn)
	})
}

// backupTagKey accepts a condition key in either form AWS returns it.
func backupTagKey(key *string) string {
	if key == nil {
		return ""
	}
	return strings.TrimPrefix(*key, "aws:ResourceTag/")
}

func backupTagEquals(key, value *string, tags map[string]string) bool {
	v, ok := tags[backupTagKey(key)]
	return ok && value != nil && v == *value
}

// backupTagLike applies a StringLike clause, where `*` is zero or more
// non-whitespace characters and everything else is literal.
func backupTagLike(p backuptypes.ConditionParameter, tags map[string]string) bool {
	v, ok := tags[backupTagKey(p.ConditionKey)]
	if !ok || p.ConditionValue == nil {
		return false
	}
	pattern := strings.ReplaceAll(regexp.QuoteMeta(*p.ConditionValue), `\*`, `\S*`)
	return regexp.MustCompile(`^` + pattern + `$`).MatchString(v)
}
