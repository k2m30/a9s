// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
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
	rc.Targets, _, _ = cachedRelatedList(cache, target)
	return rc
}

// policyRefContext is refContext for principals read from a resource's
// policy: when STS has not answered, the owning account is the one the
// resource's own ARN names, so a foreign principal never reads as local.
func policyRefContext(clients any, cache resource.ResourceCache, target, resourceARN string) domain.RefContext {
	rc := refContext(clients, cache, target)
	rc.AccountID = cmp.Or(rc.AccountID, iampolicy.AccountFromARN(resourceARN))
	return rc
}

// grantedPrincipalRefs returns the IAM principal ARNs of kind ("role/",
// "user/", "group/") that policy grants once its explicit Deny statements are
// applied, in every account: the resolver leaves out another account's. ok is
// false when the policy does not parse.
func grantedPrincipalRefs(policy, kind string) (refs []string, ok bool) {
	doc, err := iampolicy.Parse(policy)
	if err != nil {
		return nil, false
	}
	for _, p := range iampolicy.AllowedPrincipals(doc, "") {
		if a, isIAM := ARNForService(p.Value, "iam"); p.Kind == "AWS" && isIAM && strings.HasPrefix(a.Resource, kind) {
			refs = append(refs, p.Value)
		}
	}
	return refs, true
}

// relatedRefs is the result of a checker that found refs to target: each ref
// read through the target's resolver, de-duplicated. A ref that names no
// local row is left out and makes the count a lower bound.
func relatedRefs(target string, refs []string, rc domain.RefContext) resource.RelatedCheckResult {
	ids, dropped := resolveRefs(target, refs, rc)
	return relatedResultTrunc(target, ids, dropped)
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
// pivot counts only what the target list holds.
//
// lowerBound says the answer is missing a row. The references are a closed
// set the source names, so a page of the target list nobody read can hold
// more rows of that type but none this source named: when every named
// reference produced a row the count is exact however far the list was read.
// A reference that produced none — unreadable, or naming a row this list does
// not hold — is a row missing from the answer, and the resolvers are
// syntactic, so a bare name or an ARN of an account the session has not
// learned yet resolves without proving the row is there.
func listedRefs(target string, refs []string, rc domain.RefContext, list []resource.Resource) (ids []string, lowerBound bool) {
	rc.Targets = list
	names, dropped := resolveRefs(target, refs, rc)
	for _, r := range list {
		if slices.Contains(names, r.ID) && !slices.Contains(ids, r.ID) {
			ids = append(ids, r.ID)
		}
	}
	return ids, dropped || len(ids) < len(names)
}

// listedRelated is the result of a checker whose refs only the target list
// can confirm: the refs that resolve to a listed row, a lower bound when one
// did not or truncated is set, and unknown when the list cannot be read.
func listedRelated(ctx context.Context, clients any, cache resource.ResourceCache, target string, refs []string, truncated bool) resource.RelatedCheckResult {
	if !slices.ContainsFunc(refs, func(r string) bool { return r != "" }) {
		return relatedResultTrunc(target, nil, truncated)
	}
	list, _, err := FetchRelatedTarget(ctx, clients, cache, target)
	if list == nil {
		if err != nil && !errors.Is(err, errClientMissing) {
			return ReadFailed(target, err)
		}
		return NotRead(target)
	}
	ids, lowerBound := listedRefs(target, refs, refContext(clients, cache, target), list)
	return relatedResultTrunc(target, ids, truncated || lowerBound)
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
	iamUserRefToID = func(ref string, rc domain.RefContext) (string, bool) { return iamNameRef(ref, rc, "user/") }
	ecsRefToID     = arnNameRef("ecs", "cluster/", "")
	kinesisRefToID = arnNameRef("kinesis", "stream/", "/")
	ddbRefToID     = arnNameRef("dynamodb", "table/", "/")
	mskRefToID     = arnNameRef("kafka", "cluster/", "/")
	sqsRefToID     = arnNameRef("sqs", "", "")
	tgRefToID      = arnNameRef("elasticloadbalancing", "targetgroup/", "/")
	acmRefToID     = arnWholeRef("acm")
	snsRefToID     = arnWholeRef("sns")
)

// ssmRefToID reads a parameter ARN ("parameter/<name>") or name as the
// parameter's name. A name in a hierarchy keeps its leading "/", which the
// ARN folds into "parameter/" ("/app/db" is "parameter/app/db"); a flat name
// has none ("db_password" is "parameter/db_password")
// (docs.aws.amazon.com/systems-manager/latest/userguide/sysman-paramstore-su-create.html).
// A single-level "/db" and "db" share an ARN, and the loaded list tells
// which it is.
func ssmRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "ssm")
	if !ok || !isARN {
		return res, ok
	}
	name, ok := afterPrefix(res, "parameter/")
	if !ok {
		return "", false
	}
	if rc.Targets != nil && !listHolds(rc, name) && listHolds(rc, "/"+name) || strings.Contains(name, "/") {
		return "/" + name, true
	}
	return name, true
}

// lambdaFunctionName is the shape of a function name: letters, digits,
// hyphens and underscores, at most 64
// (docs.aws.amazon.com/lambda/latest/api/API_CreateFunction.html#lambda-CreateFunction-request-FunctionName).
// A stage-variable placeholder or a JSONata expression is none.
var lambdaFunctionName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// lambdaRefToID reads every form a function is named by — its name, the name
// with a version or alias ("my-fn:prod"), the partial ARN
// "<account>:function:my-fn" and the full ARN — as the function's name
// (docs.aws.amazon.com/lambda/latest/api/API_Invoke.html#lambda-Invoke-request-FunctionName).
func lambdaRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "lambda")
	if !ok {
		return "", false
	}
	if isARN {
		if res, ok = afterPrefix(res, "function:"); !ok {
			return "", false
		}
	} else if owner, rest, partial := strings.Cut(res, ":function:"); partial {
		if account := lastSegment(owner, ":"); rc.AccountID != "" && account != rc.AccountID {
			return "", false
		}
		res = rest
	}
	name, _, _ := strings.Cut(res, ":")
	return name, lambdaFunctionName.MatchString(name)
}

// s3RefToID reads a bucket ARN, an s3://bucket/key URI or a bare
// bucket/key location as the bucket's name; a bucket name holds no "/".
func s3RefToID(ref string, rc domain.RefContext) (string, bool) {
	if !arn.IsARN(ref) {
		bucket, _, _ := strings.Cut(strings.TrimPrefix(ref, "s3://"), "/")
		return bucket, bucket != ""
	}
	return s3ARNRefToID(ref, rc)
}

// s3ARNRefToID reads a bucket ARN ("arn:aws:s3:::<bucket>[/<key>]"), the one
// s3 ARN with no Region and no account; an access point, Multi-Region access
// point, Batch Operations job or Storage Lens ARN names no bucket.
func s3ARNRefToID(ref string, rc domain.RefContext) (string, bool) {
	if a, err := arn.Parse(ref); err != nil || a.Region != "" || a.AccountID != "" {
		return "", false
	}
	return arnNameRef("s3", "", "/")(ref, rc)
}

// ecsSvcRefToID reads a service ARN ("service/<cluster>/<name>", or the
// older "service/<name>"), the cluster-qualified ID itself, or a task's
// group "service:<name>" as the service row's ID. A reference naming no
// cluster is a service name, which is unique inside a cluster only: it names
// the loaded row of that name, and with two of them the first.
func ecsSvcRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "ecs")
	if !ok {
		return "", false
	}
	if isARN {
		if res, ok = afterPrefix(res, "service/"); !ok {
			return "", false
		}
	} else {
		res, _ = strings.CutPrefix(res, "service:")
	}
	if cluster, name, qualified := strings.Cut(res, "/"); qualified {
		return ecsSvcID(cluster, name), cluster != "" && name != ""
	}
	return idOrLoadedName(res, rc)
}

// ecsSvcRefFromTask is the reference a task makes to the service that
// started it: ECS writes the service's bare name into the task's group, and
// a service name is unique inside the cluster the task itself runs in.
func ecsSvcRefFromTask(res resource.Resource, task ecstypes.Task) (ref string, ofService bool) {
	name, ofService := strings.CutPrefix(aws.ToString(task.Group), "service:")
	if !ofService || name == "" {
		return "", false
	}
	cluster, _ := ecsRefToID(cmp.Or(aws.ToString(task.ClusterArn), res.Fields["cluster"]), domain.RefContext{})
	return ecsSvcID(cluster, name), true
}

// policyRefToID reads an IAM policy reference as the policy row's ID, which
// is the policy's ARN. An AWS-managed policy's ARN names the account "aws"
// rather than the session's, and is this account's policy all the same. A
// bare name is the name of a loaded policy, and names nothing else: two
// policies of one name, one the account's own and one AWS's, are told apart
// by nothing else a reference carries.
func policyRefToID(ref string, rc domain.RefContext) (string, bool) {
	a, err := arn.Parse(ref)
	if err != nil {
		return idOrLoadedName(ref, rc)
	}
	if a.Service != "iam" || !strings.HasPrefix(a.Resource, "policy/") {
		return "", false
	}
	// AWS owns its managed policies under the account "aws", and they are
	// attachable in every account; any other account's policy is that
	// account's.
	if a.AccountID != "aws" && rc.AccountID != "" && a.AccountID != "" && a.AccountID != rc.AccountID {
		return "", false
	}
	return ref, true
}

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
// or ARN) as the key among rc.Targets that carries it. With the list loaded,
// an alias no target carries names no row; kmsRefContext adds the key a local
// alias names.
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
		return res, !isARN && (isKMSKeyID(res) || rc.Targets != nil && listHolds(rc, res))
	}
	for _, t := range rc.Targets {
		if t.Fields["alias"] == res || t.Name == res || slices.Contains(strings.Split(t.Fields["aliases"], ","), res) {
			return t.ID, true
		}
	}
	// With no key list to read it against, a customer alias names its key by
	// itself: the kms by-ID read (DescribeKey) accepts an alias and returns
	// the key's own row. An AWS-managed alias names a key no kms row lists.
	if rc.Targets == nil && !strings.HasPrefix(res, "alias/aws/") {
		return ref, true
	}
	return "", false
}

// kmsRefContext is refContext for the kms target in region, with the key
// behind every local alias no loaded row carries added to Targets. The
// session's cache holds its own Region's keys and none of another's, so a
// read elsewhere goes through that Region's clients with no list. The kms list holds
// customer keys only, so an AWS-managed alias (alias/aws/*) is never on it,
// and an alias may be newer than the list. The keys come from the kms type's
// own by-ID lookup: DescribeKey accepts an alias name or alias ARN. Another
// account's alias is never looked up. The error is the lookup's; the rows it
// did return are in Targets either way.
func kmsRefContext(ctx context.Context, clients any, cache resource.ResourceCache, region string, refs []string) (domain.RefContext, error) {
	if c, ok := clients.(*ServiceClients); ok && c != nil {
		if elsewhere := c.InRegion(region); elsewhere != c {
			clients, cache = elsewhere, nil
		}
	}
	rc := refContext(clients, cache, "kms")
	var missing []string
	for _, ref := range refs {
		res, _, local := localARN(ref, rc, "kms")
		if id, known := resource.ResolveRef("kms", ref, rc); local && strings.HasPrefix(res, "alias/") && (!known || id == ref) {
			missing = append(missing, ref)
		}
	}
	if len(missing) == 0 {
		return rc, nil
	}
	// A non-nil target list makes the panel count an alias only through the
	// key that carries it, never the alias itself.
	targets := append(make([]resource.Resource, 0, len(rc.Targets)), rc.Targets...)
	var err error
	if c, ok := clients.(*ServiceClients); ok && c != nil && c.KMS != nil {
		var rows []resource.Resource
		rows, err = FetchKMSKeysByIDs(ctx, c, missing)
		targets = append(targets, rows...)
	}
	rc.Targets = targets
	return rc, err
}

// kmsResolve resolves refs to kms row IDs through kmsRefContext, in region.
// A lookup that failed leaves what did resolve as a lower bound; its error is
// the answer only when nothing resolved.
func kmsResolve(ctx context.Context, clients any, cache resource.ResourceCache, region string, refs []string) (ids []string, lowerBound bool, err error) {
	rc, err := kmsRefContext(ctx, clients, cache, region, refs)
	ids, dropped := resolveRefs("kms", refs, rc)
	if len(ids) == 0 && err != nil {
		return nil, false, err
	}
	return ids, dropped || err != nil, nil
}

// kmsRelated is kmsRelatedIn in the Region the keys' ARNs name.
func kmsRelated(ctx context.Context, clients any, cache resource.ResourceCache, refs []string) resource.RelatedCheckResult {
	return kmsRelatedIn(ctx, clients, cache, kmsRegion(refs), refs)
}

// kmsRelatedIn is relatedRefs for the kms target through kmsResolve, for keys
// that live in region; the result is read there.
func kmsRelatedIn(ctx context.Context, clients any, cache resource.ResourceCache, region string, refs []string) resource.RelatedCheckResult {
	ids, lowerBound, err := kmsResolve(ctx, clients, cache, region, refs)
	if err != nil {
		return ReadFailed("kms", err)
	}
	return inRegion(clients, region, relatedResultTrunc("kms", ids, lowerBound))
}

// kmsRegion is the Region the first key or alias ARN among refs names; a
// bare ID or alias names none, which reads as the session's.
func kmsRegion(refs []string) string {
	for _, ref := range refs {
		if region := arnRegionOf(ref, "kms"); region != "" {
			return region
		}
	}
	return ""
}

// isKMSKeyID reports whether s has the shape of a key ID: a UUID, or
// "mrk-" for a multi-Region key.
func isKMSKeyID(s string) bool {
	return strings.HasPrefix(s, "mrk-") || len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

// secretsRefToID reads a secret ARN as the name of the loaded secret it
// names. Secrets Manager appends "-" and six random characters to the name
// in a full ARN, a partial ARN omits them, and a name may itself end in
// "-XXXXXX", so only the list tells the two apart: a partial ARN names the
// row of that name, a full one the row whose ARN it is. With no list loaded
// an ARN whose name cannot end in that suffix is a partial one and names the
// secret of that name; any other ARN, less the ":json-key:version-stage:
// version-id" a container's ValueFrom may carry after it, names its secret by
// itself: DescribeSecret takes a full or a partial ARN.
func secretsRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "secretsmanager")
	if !ok {
		return "", false
	}
	if !isARN {
		// A bare reference is "secret-id[:json-key[:version-stage:version-id]]"
		// (CodeBuild, ECS); a secret name holds no ":".
		name, _, _ := strings.Cut(res, ":")
		return name, name != ""
	}
	name, ok := afterPrefix(res, "secret:")
	if !ok {
		return "", false
	}
	name, _, _ = strings.Cut(name, ":")
	full := ref[:len(ref)-len(res)] + "secret:" + name
	if rc.Targets == nil {
		if !secretSuffix.MatchString(name) {
			return name, true
		}
		return full, true
	}
	// A row named as written answers before a row whose ARN it is: the list
	// holds a secret of exactly that name.
	if listHolds(rc, name) {
		return name, true
	}
	for _, t := range rc.Targets {
		if t.Fields["arn"] == full {
			return t.ID, true
		}
	}
	return "", false
}

// secretSuffix is the shape of the "-" and six characters Secrets Manager
// appends to a secret's name in its full ARN. A name that does not end in it
// can only be a partial ARN's.
var secretSuffix = regexp.MustCompile(`-[A-Za-z0-9]{6}$`)

// elbRefToID reads a load balancer ARN (application, network, gateway —
// "loadbalancer/<kind>/<name>/<id>" — or classic — "loadbalancer/<name>")
// as the load balancer's name, and a listener ARN
// ("listener/<kind>/<name>/<id>/<listener-id>") as the name of the one load
// balancer the listener belongs to. A bare name never holds a "/".
func elbRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "elasticloadbalancing")
	if !ok || !isARN {
		return res, ok && !strings.Contains(res, "/")
	}
	if rest, isListener := afterPrefix(res, "listener/"); isListener {
		parts := strings.Split(rest, "/")
		return parts[min(1, len(parts)-1)], len(parts) == 4
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
// to is in its API mappings, not in the reference. A bare value is the ID it
// is or, since CloudWatch names a REST API by name, the ID of the loaded API
// of that name.
func apigwRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "apigateway")
	if !ok {
		return "", false
	}
	if !isARN {
		if rc.Targets == nil && !apigwID.MatchString(ref) {
			return "", false
		}
		return idOrLoadedName(ref, rc)
	}
	for _, prefix := range []string{"/restapis/", "/apis/"} {
		if rest, found := afterPrefix(res, prefix); found {
			id, _, _ := strings.Cut(rest, "/")
			return id, true
		}
	}
	return "", false
}

// apigwDomainRefToID reads an API Gateway custom domain's ARN
// ("/domainnames/<name>") as the domain's name.
// https://docs.aws.amazon.com/apigateway/latest/developerguide/arn-format-reference.html
func apigwDomainRefToID(ref string) (string, bool) {
	a, ok := ARNForService(ref, "apigateway")
	if !ok {
		return "", false
	}
	name, ok := afterPrefix(a.Resource, "/domainnames/")
	name, _, _ = strings.Cut(name, "/")
	return name, ok
}

// apigwID is the shape of an API ID: lower-case letters and digits. A custom
// domain's ID ("d-…") has a hyphen, and a name is read only against the
// loaded list.
var apigwID = regexp.MustCompile(`^[a-z0-9]+$`)

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
	family, _, _ := strings.Cut(taskDefRevision(ref), ":")
	return family
}

// taskDefRevision reads a task definition ARN or "<family>:<revision>" as
// "<family>:<revision>".
func taskDefRevision(ref string) string {
	rev, _ := arnNameRef("ecs", "task-definition/", "")(ref, domain.RefContext{})
	return rev
}

// extractEventBusName reads an event bus ARN ("event-bus/<name>") or a bare
// name as the bus name. An event bus is no a9s row: eb-rule rows are matched
// by the bus they sit on.
func extractEventBusName(ref string) string {
	name, _ := arnNameRef("events", "event-bus/", "")(ref, domain.RefContext{})
	return name
}

var (
	iamGroupRefToID     = func(ref string, rc domain.RefContext) (string, bool) { return iamNameRef(ref, rc, "group/") }
	sfnRefToID          = arnNameRef("states", "stateMachine:", ":")
	cbRefToID           = arnNameRef("codebuild", "project/", "")
	cfRefToID           = arnNameRef("cloudfront", "distribution/", "")
	cfnRefToID          = arnNameRef("cloudformation", "stack/", "/")
	codeartifactRefToID = arnNameRef("codeartifact", "repository/", "")
	dbcRefToID          = arnNameRef("rds", "cluster:", "")
	dbiSnapRefToID      = arnNameRef("rds", "snapshot:", "")
	eksRefToID          = arnNameRef("eks", "cluster/", "")
	pipelineRefToID     = arnNameRef("codepipeline", "", "")
	trailRefToID        = arnNameRef("cloudtrail", "trail/", "")
	dbcSnapRefToID      = arnNameRef("rds", "cluster-snapshot:", "")
	tgwRefToID          = ec2ARNRef("transit-gateway/", "tgw-")
)

// asgRefToID reads an Auto Scaling group ARN
// ("autoScalingGroup:<uuid>:autoScalingGroupName/<name>") as the group's name.
func asgRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "autoscaling")
	if !ok || !isARN {
		return res, ok
	}
	_, name, found := strings.Cut(res, ":autoScalingGroupName/")
	return name, found && strings.HasPrefix(res, "autoScalingGroup:") && name != ""
}

// ngRefToID reads a node group ARN ("nodegroup/<cluster>/<name>/<uuid>") as
// the "<cluster>/<name>" the node group rows are keyed by.
func ngRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "eks")
	if !ok || !isARN {
		return res, ok
	}
	rest, ok := afterPrefix(res, "nodegroup/")
	parts := strings.Split(rest, "/")
	if !ok || len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return ngRowID(parts[0], parts[1]), true
}

// ecsTaskRefToID reads a task ARN ("task/<cluster>/<id>", or the older
// "task/<id>") as the task ID the task rows are keyed by.
func ecsTaskRefToID(ref string, rc domain.RefContext) (string, bool) {
	id, ok := arnNameRef("ecs", "task/", "")(ref, rc)
	return lastSegment(id, "/"), ok && id != ""
}

// ebRefToID reads an environment ARN ("environment/<app>/<env>") as the ID of
// the loaded environment it names, and a bare value as an environment ID or
// name. The ID ("e-…") is not in the ARN, so an ARN reads only against the
// loaded list.
func ebRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "elasticbeanstalk")
	if !ok {
		return "", false
	}
	if !isARN {
		return idOrLoadedName(ref, rc)
	}
	if _, ok := afterPrefix(res, "environment/"); !ok {
		return "", false
	}
	for _, t := range rc.Targets {
		if t.Fields["environment_arn"] == ref {
			return t.ID, true
		}
	}
	return "", false
}

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
	id, ok := ec2ARNRef("volume/", "vol-")(ref, rc)
	return id, ok && id != copiedSnapshotVolumeID && listHolds(rc, id)
}

// copiedSnapshotVolumeID is the volume ID a snapshot made by CopySnapshot
// carries: it names no volume
// (docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html, volumeId).
const copiedSnapshotVolumeID = "vol-ffffffff"

// amiRefToID reads an image ARN ("image/ami-…") or an AMI ID. A launch
// template's ImageId may instead be "resolve:ssm:<parameter>", which names a
// parameter holding an AMI ID, not an image.
func amiRefToID(ref string, rc domain.RefContext) (string, bool) {
	return ec2ARNRef("image/", "ami-")(ref, rc)
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
	return idOrLoadedName(ref, rc)
}

// idOrLoadedName reads a bare value as the ID of the loaded row of that name,
// or as the ID it is; with the list loaded, a value that is neither names no
// row.
func idOrLoadedName(ref string, rc domain.RefContext) (string, bool) {
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

// ec2ARNRef is the resolver of an EC2 type keyed by its ID: an ARN of kind
// ("vpc/", "security-group/", "instance/") reads as the ID after it, and a
// bare value is the ID already. Either way the ID carries the prefix EC2
// gives every ID of the type ("vpc-", "sg-", "i-"); a value without it (an
// SSM managed node's "mi-…", a "resolve:ssm:" image) is no row of the type.
func ec2ARNRef(kind, idPrefix string) func(string, domain.RefContext) (string, bool) {
	return func(ref string, rc domain.RefContext) (string, bool) {
		id, isARN, ok := localARN(ref, rc, "ec2")
		if ok && isARN {
			id, ok = afterPrefix(id, kind)
		}
		return id, ok && strings.HasPrefix(id, idPrefix)
	}
}

var (
	ec2RefToID    = ec2ARNRef("instance/", "i-")
	vpcRefToID    = ec2ARNRef("vpc/", "vpc-")
	sgRefToID     = ec2ARNRef("security-group/", "sg-")
	subnetARNToID = ec2ARNRef("subnet/", "subnet-")
)

// subnetRefToID reads a subnet ARN ("subnet/<id>") or subnet ID. An ASG's
// VPCZoneIdentifier joins several IDs with commas; that names no one row.
func subnetRefToID(ref string, rc domain.RefContext) (string, bool) {
	id, ok := subnetARNToID(ref, rc)
	return id, ok && !strings.Contains(id, ",")
}
