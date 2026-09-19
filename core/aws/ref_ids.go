// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// This file holds the target types' reference resolvers (catalog RefToID):
// the one reading of every shape AWS hands back for a resource. Related
// checkers and navigable fields reach them through resource.ResolveRef and
// never parse a reference themselves.

// refContext is what a related checker knows when it resolves a reference to
// target: the session's account (when STS has answered) and region, and the
// target's cached rows.
func refContext(clients any, cache resource.ResourceCache, target string) domain.RefContext {
	var rc domain.RefContext
	if c, ok := clients.(*ServiceClients); ok && c != nil {
		rc.Region = c.Region
		if store := c.IdentityStore(); store != nil {
			rc.AccountID = store.AccountID()
		}
	}
	if entry, ok := cache[target]; ok {
		rc.Targets = entry.Resources
	}
	return rc
}

// relatedRefs is the result of a checker that found refs to target: each ref
// read through the target's resolver, de-duplicated. A ref that names no
// local row is left out and makes the count a lower bound.
func relatedRefs(target string, refs []string, rc domain.RefContext) resource.RelatedCheckResult {
	ids, dropped := resolveRefs(target, refs, rc)
	return resource.KnownRelated(target, ids, dropped)
}

// resolveRefs is relatedRefs for a checker that has a truncation of its own
// to fold in: the resolved, de-duplicated IDs, and whether any ref was left
// out.
func resolveRefs(target string, refs []string, rc domain.RefContext) (ids []string, dropped bool) {
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		id, ok := resource.ResolveRef(target, ref, rc)
		if !ok {
			dropped = true
			continue
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, dropped
}

// listedRefs is resolveRefs kept to the rows of list, for a checker whose
// pivot counts only what the target list holds. A ref that names no local
// row is left out and reported, as in resolveRefs; one that names a local
// row the list does not hold is left out silently.
func listedRefs(target string, refs []string, rc domain.RefContext, list []resource.Resource) (ids []string, dropped bool) {
	rc.Targets = list
	names, dropped := resolveRefs(target, refs, rc)
	for _, r := range list {
		if slices.Contains(names, r.ID) {
			ids = append(ids, r.ID)
		}
	}
	return ids, dropped
}

// localARN reads ref as an ARN of service. isARN is false for anything that
// is not an ARN, which the caller reads as a bare ID. ok is false for another
// service's ARN, or one that names another account or region than rc's; an
// ARN without an account or region (IAM, S3, Route 53) is not held to them.
func localARN(ref string, rc domain.RefContext, service string) (res string, isARN, ok bool) {
	a, err := arn.Parse(ref)
	if err != nil {
		return ref, false, ref != ""
	}
	if a.Service != service || a.Resource == "" ||
		(rc.AccountID != "" && a.AccountID != "" && a.AccountID != rc.AccountID) ||
		(rc.Region != "" && a.Region != "" && a.Region != rc.Region) {
		return "", true, false
	}
	return a.Resource, true, true
}

// arnsOnly keeps the refs that are ARNs, for a field AWS fills with ARNs only:
// anything else there is malformed, and a resolver would read it as a bare ID.
func arnsOnly(refs []string) []string {
	var arns []string
	for _, r := range refs {
		if arn.IsARN(r) {
			arns = append(arns, r)
		}
	}
	return arns
}

// afterPrefix returns what follows prefix in s, or ok=false when s does not
// start with it or nothing follows.
func afterPrefix(s, prefix string) (string, bool) {
	rest, ok := strings.CutPrefix(s, prefix)
	return rest, ok && rest != ""
}

// arnNameRef is the resolver of a type whose rows are keyed by the name an
// ARN of service carries after prefix ("role/", "cluster/", "function:"),
// up to the first of stop. A bare value is already the name.
func arnNameRef(service, prefix, stop string) func(string, domain.RefContext) (string, bool) {
	return func(ref string, rc domain.RefContext) (string, bool) {
		res, isARN, ok := localARN(ref, rc, service)
		if !ok || !isARN {
			return res, ok
		}
		name, ok := afterPrefix(res, prefix)
		if !ok {
			return "", false
		}
		if stop != "" {
			if i := strings.IndexAny(name, stop); i >= 0 {
				name = name[:i]
			}
		}
		return name, name != ""
	}
}

// arnWholeRef is the resolver of a type whose rows are keyed by the ARN
// itself (acm, sns): a local ARN of service is its own ID.
func arnWholeRef(service string) func(string, domain.RefContext) (string, bool) {
	return func(ref string, rc domain.RefContext) (string, bool) {
		if _, isARN, ok := localARN(ref, rc, service); !ok || !isARN {
			return "", false
		}
		return ref, true
	}
}

// lastSegment returns what follows the last sep in s.
func lastSegment(s, sep string) string {
	return s[strings.LastIndex(s, sep)+1:]
}

var (
	lambdaRefToID  = arnNameRef("lambda", "function:", ":")
	iamUserRefToID = func(ref string, rc domain.RefContext) (string, bool) { return iamNameRef(ref, rc, "user/") }
	ecsRefToID     = arnNameRef("ecs", "cluster/", "")
	kinesisRefToID = arnNameRef("kinesis", "stream/", "/")
	mskRefToID     = arnNameRef("kafka", "cluster/", "/")
	sqsRefToID     = arnNameRef("sqs", "", "")
	s3RefToID      = arnNameRef("s3", "", "/")
	tgRefToID      = arnNameRef("elasticloadbalancing", "targetgroup/", "/")
	acmRefToID     = arnWholeRef("acm")
	snsRefToID     = arnWholeRef("sns")
)

// roleRefToID reads a role ARN — or the STS assumed-role ARN a session of
// the role acts as, "assumed-role/<role>/<session>" — as the role's name.
func roleRefToID(ref string, rc domain.RefContext) (string, bool) {
	if res, isARN, ok := localARN(ref, rc, "sts"); isARN && ok {
		rest, found := afterPrefix(res, "assumed-role/")
		role, _, _ := strings.Cut(rest, "/")
		return role, found
	}
	return iamNameRef(ref, rc, "role/")
}

// iamNameRef reads an IAM ARN of kind ("role/", "user/"): the name is the last
// path segment, since a path ("service-role/") is not part of it.
func iamNameRef(ref string, rc domain.RefContext, kind string) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "iam")
	if !ok || !isARN {
		return res, ok
	}
	rest, ok := afterPrefix(res, kind)
	if !ok {
		return "", false
	}
	return lastSegment(rest, "/"), true
}

// kmsRefToID reads a key ARN or bare key ID as the key ID, and an alias (bare
// or ARN) as the key among rc.Targets that carries it. An alias no target
// carries names no row; kmsRefContext adds the key a local alias names.
func kmsRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "kms")
	if !ok {
		return "", false
	}
	if isARN {
		if id, isKey := afterPrefix(res, "key/"); isKey {
			return id, true
		}
	}
	if !strings.HasPrefix(res, "alias/") {
		return res, !isARN && isKMSKeyID(res)
	}
	for _, t := range rc.Targets {
		// "alias" alone is what a row cached before "aliases" existed carries.
		if t.Fields["alias"] == res || slices.Contains(strings.Split(t.Fields["aliases"], ","), res) {
			return t.ID, true
		}
	}
	return "", false
}

// kmsRefContext is refContext for the kms target, with the key behind every
// local alias no loaded row carries added to Targets. The kms list holds
// customer keys only, so an AWS-managed alias (alias/aws/*) is never on it,
// and an alias may be newer than the list. The keys come from the kms type's
// own by-ID lookup: DescribeKey accepts an alias name or alias ARN. Another
// account's alias is never looked up.
func kmsRefContext(ctx context.Context, clients any, cache resource.ResourceCache, refs []string) (domain.RefContext, error) {
	rc := refContext(clients, cache, "kms")
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.KMS == nil {
		return rc, nil
	}
	var missing []string
	for _, ref := range refs {
		res, _, local := localARN(ref, rc, "kms")
		if _, known := resource.ResolveRef("kms", ref, rc); local && !known && strings.HasPrefix(res, "alias/") {
			missing = append(missing, ref)
		}
	}
	if len(missing) == 0 {
		return rc, nil
	}
	rows, err := FetchKMSKeysByIDs(ctx, c, missing)
	rc.Targets = append(slices.Clone(rc.Targets), rows...)
	return rc, err
}

// kmsRelated is relatedRefs for the kms target, through kmsRefContext.
func kmsRelated(ctx context.Context, clients any, cache resource.ResourceCache, refs []string) resource.RelatedCheckResult {
	rc, err := kmsRefContext(ctx, clients, cache, refs)
	if err != nil {
		return resource.ErrorRelated("kms", err)
	}
	return relatedRefs("kms", refs, rc)
}

// isKMSKeyID reports whether s has the shape of a key ID: a UUID, or
// "mrk-" for a multi-Region key.
func isKMSKeyID(s string) bool {
	return strings.HasPrefix(s, "mrk-") || len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

// secretsRefToID reads a secret ARN as the secret's name. Secrets Manager
// appends "-" and six random characters to the name in the ARN, and a
// container's ValueFrom may carry ":json-key:version-stage:version-id" after
// it. A row whose ID or ARN is the reference as written wins over stripping
// the suffix, since a name may itself end in "-XXXXXX".
func secretsRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "secretsmanager")
	if !ok || !isARN {
		return res, ok
	}
	name, ok := afterPrefix(res, "secret:")
	if !ok {
		return "", false
	}
	name, _, _ = strings.Cut(name, ":")
	full := ref[:len(ref)-len(res)] + "secret:" + name
	for _, t := range rc.Targets {
		if t.ID == name || t.Fields["arn"] == full {
			return t.ID, true
		}
	}
	if i := strings.LastIndex(name, "-"); i > 0 && len(name)-i == 7 {
		name = name[:i]
	}
	return name, true
}

// elbRefToID reads a load balancer ARN (application, network, gateway —
// "loadbalancer/<kind>/<name>/<id>" — or classic — "loadbalancer/<name>")
// as the load balancer's name. A bare name never holds a "/".
func elbRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "elasticloadbalancing")
	if !ok || !isARN {
		return res, ok && !strings.Contains(res, "/")
	}
	rest, ok := afterPrefix(res, "loadbalancer/")
	if !ok {
		return "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 3 {
		return parts[1], true
	}
	return parts[0], len(parts) == 1
}

// efsRefToID reads a file system ARN as its ID, and an access point (ARN or
// fsap- ID) as the file system the loaded list says it belongs to.
func efsRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "elasticfilesystem")
	if !ok {
		return "", false
	}
	if isARN {
		if id, isFS := afterPrefix(res, "file-system/"); isFS {
			return id, true
		}
		if res, ok = afterPrefix(res, "access-point/"); !ok {
			return "", false
		}
	}
	if !strings.HasPrefix(res, "fsap-") {
		return res, true
	}
	for _, t := range rc.Targets {
		for ap := range strings.SplitSeq(t.Fields["access_point_ids"], ",") {
			if ap == res {
				return t.ID, true
			}
		}
	}
	return "", false
}

// logsRefToID reads a log group ARN, with or without the ":*" AWS appends
// for the group's streams, as the group's name.
func logsRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "logs")
	if !ok || !isARN {
		return res, ok
	}
	name, ok := afterPrefix(res, "log-group:")
	if !ok {
		return "", false
	}
	name = strings.TrimSuffix(name, ":*")
	return name, name != ""
}

// r53RefToID reads a zone ID in any of the forms Route 53 and its callers
// use — bare, "/hostedzone/<id>", or a zone ARN — as the "/hostedzone/<id>"
// the zone list is keyed by.
func r53RefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "route53")
	if !ok {
		return "", false
	}
	id := strings.TrimPrefix(strings.TrimPrefix(res, "/"), "hostedzone/")
	if isARN && id == res {
		return "", false
	}
	return "/hostedzone/" + id, id != ""
}

// igwRefToID accepts only an internet gateway ID: a route's GatewayId also
// names "local" and virtual private gateways, which are no IGW.
func igwRefToID(ref string, _ domain.RefContext) (string, bool) {
	return ref, strings.HasPrefix(ref, "igw-")
}

// apigwRefToID reads an API Gateway ARN ("/restapis/<id>/...", "/apis/<id>/...")
// as the API's ID. A custom domain's ARN names no API: which APIs it maps
// to is in its API mappings, not in the reference.
func apigwRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "apigateway")
	if !ok || !isARN {
		return res, ok
	}
	for _, prefix := range []string{"/restapis/", "/apis/"} {
		if rest, found := afterPrefix(res, prefix); found {
			id, _, _ := strings.Cut(rest, "/")
			return id, true
		}
	}
	return "", false
}

// instanceProfileName reads an instance profile ARN
// ("instance-profile/<path>/<name>") or a bare name as the profile's name.
// A profile is no a9s row; its name is what iam:GetInstanceProfile takes and
// what a manual setup gives its role.
func instanceProfileName(ref string) string {
	res, isARN, _ := localARN(ref, domain.RefContext{}, "iam")
	if rest, ok := afterPrefix(res, "instance-profile/"); ok {
		return lastSegment(rest, "/")
	}
	if isARN {
		return ""
	}
	return ref
}

// taskDefFamily reads a task definition ARN
// ("task-definition/<family>:<revision>") or "<family>:<revision>" as the
// family. A family is no a9s row; checkers match log groups and state
// machines against it.
func taskDefFamily(ref string) string {
	if _, rest, ok := strings.Cut(ref, ":task-definition/"); ok {
		ref = rest
	}
	family, _, _ := strings.Cut(ref, ":")
	return family
}

// extractEventBusName extracts the bus name from an EventBridge event bus ARN.
// ARN format: arn:aws:events:REGION:ACCOUNT:event-bus/NAME
// If the input contains no "/", it is returned as-is (handles already-extracted names).
// An event bus is no a9s row: eb-rule rows are matched by the bus they sit on.
func extractEventBusName(arn string) string {
	return lastSegment(arn, "/")
}

var (
	iamGroupRefToID = func(ref string, rc domain.RefContext) (string, bool) { return iamNameRef(ref, rc, "group/") }
	sfnRefToID      = arnNameRef("states", "stateMachine:", ":")
)

// listHolds reports whether the loaded target list holds id; with no list
// loaded nothing is known against it.
func listHolds(rc domain.RefContext, id string) bool {
	return rc.Targets == nil || slices.ContainsFunc(rc.Targets, func(r resource.Resource) bool { return r.ID == id })
}

// dbiRefToID reads a DB instance ARN ("db:<identifier>") or identifier. A
// snapshot outlives the instance it was taken from, so an identifier the
// loaded list does not hold names no row.
func dbiRefToID(ref string, rc domain.RefContext) (string, bool) {
	id, isARN, ok := localARN(ref, rc, "rds")
	if ok && isARN {
		id, ok = afterPrefix(id, "db:")
	}
	return id, ok && listHolds(rc, id)
}

// ebsRefToID reads a volume ARN ("volume/<id>") or volume ID. A snapshot
// outlives the volume it was taken from, so an ID the loaded list does not
// hold names no row.
func ebsRefToID(ref string, rc domain.RefContext) (string, bool) {
	id, isARN, ok := localARN(ref, rc, "ec2")
	if ok && isARN {
		id, ok = afterPrefix(id, "volume/")
	}
	return id, ok && listHolds(rc, id)
}

// wafRefToID reads a web ACL ARN ("<scope>/webacl/<name>/<id>") as its ID,
// and a bare value as the ID it is or, since CloudWatch names a web ACL by
// name, as the ID of the loaded web ACL of that name.
func wafRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "wafv2")
	if !ok {
		return "", false
	}
	if isARN {
		parts := strings.Split(res, "/")
		return parts[len(parts)-1], len(parts) == 4 && parts[1] == "webacl"
	}
	for _, t := range rc.Targets {
		if t.Name == ref && t.ID != ref {
			return t.ID, true
		}
	}
	return ref, listHolds(rc, ref)
}

// ecrRefToID reads a repository ARN ("repository/<name>") or an image URI
// ("<account>.dkr.ecr.<region>.amazonaws.com/<name>[:<tag>|@<digest>]") as
// the repository's name. Anything else is a name already; a repository name
// may itself contain "/".
func ecrRefToID(ref string, rc domain.RefContext) (string, bool) {
	if res, isARN, ok := localARN(ref, rc, "ecr"); isARN {
		name, found := afterPrefix(res, "repository/")
		return name, ok && found
	}
	host, repo, isURI := strings.Cut(ref, "/")
	account, rest, isECR := strings.Cut(host, ".dkr.ecr.")
	if !isURI || !isECR {
		return ref, ref != ""
	}
	region, _, _ := strings.Cut(rest, ".")
	if (rc.AccountID != "" && account != rc.AccountID) || (rc.Region != "" && region != rc.Region) {
		return "", false
	}
	repo, _, _ = strings.Cut(repo, "@")
	repo, _, _ = strings.Cut(repo, ":")
	return repo, repo != ""
}

// subnetRefToID reads a subnet ARN ("subnet/<id>") or subnet ID. An ASG's
// VPCZoneIdentifier joins several IDs with commas; that names no one row.
func subnetRefToID(ref string, rc domain.RefContext) (string, bool) {
	id, isARN, ok := localARN(ref, rc, "ec2")
	if ok && isARN {
		id, ok = afterPrefix(id, "subnet/")
	}
	return id, ok && !strings.Contains(id, ",")
}
