// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

// CIDROpenToEveryone reports whether a list of CIDR blocks reaches every
// address on the internet. A prefix of length zero in either family says so:
// "0.0.0.0/0" and "::/0" are the same statement about different address
// spaces, and a dual-stack resource open to all IPv6 clients is open. Any
// longer prefix names a subset, whatever base address is written in front of
// it, and an entry that does not parse names no range at all.
//
// It is the single owner of that question for every site in this package that
// tests a CIDR list for "everyone".
func CIDROpenToEveryone(cidrs []string) bool {
	return slices.ContainsFunc(cidrs, func(c string) bool {
		p, err := netip.ParsePrefix(c)
		return err == nil && p.Bits() == 0
	})
}

// S3OriginBucket returns the bucket an origin domain name addresses, and
// whether the domain is an S3 endpoint at all.
//
// The bucket is every label before the endpoint marker, because a bucket name
// may itself contain dots; the marker is the "s3" label or one of its dashed
// variants (s3-website, s3-accelerate), and the last one wins so a bucket
// whose own name carries an s3 label keeps it. The suffix belongs to the
// partition, so a China endpoint ending in amazonaws.com.cn is an S3 endpoint
// exactly as its commercial twin is.
func S3OriginBucket(host string) (string, bool) {
	rest, ok := strings.CutSuffix(host, ".amazonaws.com.cn")
	if !ok {
		rest, ok = strings.CutSuffix(host, ".amazonaws.com")
	}
	if !ok {
		return "", false
	}
	labels := strings.Split(rest, ".")
	for i := len(labels) - 1; i > 0; i-- {
		if labels[i] == "s3" || strings.HasPrefix(labels[i], "s3-") {
			bucket := strings.Join(labels[:i], ".")
			return bucket, bucket != ""
		}
	}
	return "", false
}

// awsManagedPolicyAccount is the account segment AWS puts on its own managed
// policy ARNs, in every partition.
const awsManagedPolicyAccount = "aws"

// isSecret reports whether a value is a Secrets Manager ARN. Task definitions
// and function environments carry secret references beside plain values, and
// three sites ask the same question of them.
func isSecret(v string) bool {
	_, ok := ARNForService(v, "secretsmanager")
	return ok
}

// awsManagedPolicy parses a policy ARN and reports whether it is one of AWS's
// own rather than an account's. AWS publishes them in every partition under
// the same account segment and the same resource path, so the answer is in
// those two fields and never in the partition.
func awsManagedPolicy(policyARN string) (arn.ARN, bool) {
	a, ok := ARNForService(policyARN, "iam")
	if !ok || a.AccountID != awsManagedPolicyAccount || !strings.HasPrefix(a.Resource, "policy/") {
		return arn.ARN{}, false
	}
	return a, true
}

// PartitionForRegion returns the ARN partition a region belongs to, read from
// the SDK's own partition catalogue (core/aws/data/partitions.json): "aws-cn"
// for the China regions, "aws-us-gov" for GovCloud, "aws-iso"/"aws-iso-b"/
// "aws-iso-e"/"aws-iso-f" for the isolated partitions, "aws-eusc" for the
// European Sovereign Cloud, "aws" otherwise. The catalogue's patterns are
// anchored on whole segments, so us-west-2 is commercial and only us-gov- is
// GovCloud.
//
// It is the only place this package decides a partition, and it decides it
// from the same list the SDK builds its endpoints from. An ARN built with a
// hard-coded "aws" is not malformed outside the commercial partition — it
// simply never matches anything AWS returned, so the failure reads as a
// confident zero.
//
// A region the catalogue does not recognise, including the empty string, is
// commercial: the caller has a region it can name, and refusing to build an
// ARN for a region AWS has not yet listed would lose every row in it.
func PartitionForRegion(region string) string {
	for _, p := range partitionRegexes {
		if p.re.MatchString(region) {
			return p.id
		}
	}
	return "aws"
}

// ARNForService parses an ARN AWS returned and reports whether it names the
// given service. The partition and the region are never compared: an ARN in
// aws-cn or aws-us-gov names the same service as its aws twin, and matching
// the literal "arn:aws:" discards every resource outside the commercial
// partition — a filter that renders as a proven zero rather than an error.
// A malformed or non-ARN string is not a match.
func ARNForService(s, service string) (arn.ARN, bool) {
	a, err := arn.Parse(s)
	if err != nil || a.Service != service {
		return arn.ARN{}, false
	}
	return a, true
}

// lambdaIntegrationARN returns the function ARN an API Gateway integration
// URI names: the URI itself when it is a function ARN, or the ARN an invoke
// URI wraps ("arn:aws:apigateway:<region>:lambda:path/<date>/functions/<function
// ARN>/invocations"). Any other integration names no function, and the answer
// is "".
func lambdaIntegrationARN(uri string) string {
	if _, rest, ok := strings.Cut(uri, "/functions/"); ok {
		uri = strings.TrimSuffix(rest, "/invocations")
	}
	if _, ok := ARNForService(uri, "lambda"); !ok {
		return ""
	}
	return uri
}

// canonicalDNS lowercases and strips a single trailing dot.
func canonicalDNS(s string) string {
	s = strings.ToLower(s)
	return strings.TrimSuffix(s, ".")
}

// elbv2Dimension returns the CloudWatch dimension value ELBv2 metrics carry
// for an ELBv2 ARN: for a load balancer the resource after "loadbalancer/"
// ("app/<name>/<id>"), for a target group the resource from "targetgroup/"
// on ("targetgroup/<name>/<id>"). Any other value is returned as is.
func elbv2Dimension(ref string) string {
	a, ok := ARNForService(ref, "elasticloadbalancing")
	if !ok {
		return ref
	}
	if after, ok := strings.CutPrefix(a.Resource, "loadbalancer/"); ok {
		return after
	}
	if strings.HasPrefix(a.Resource, "targetgroup/") {
		return a.Resource
	}
	return ref
}

// elbNameFromENIDescription returns the load balancer name an ELB-owned ENI's
// Description carries ("ELB app/<name>/<id>"), or "" for any other
// description.
func elbNameFromENIDescription(desc string) string {
	rest, ok := strings.CutPrefix(desc, "ELB ")
	if !ok {
		return ""
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// logGroupOwner returns the resource a log group is named after by AWS's
// naming convention prefix<owner>[/<suffix>] ("/aws/lambda/<function>",
// "/ecs/<family>", "API-Gateway-Execution-Logs_<api-id>/<stage>"), or "" when
// the group does not follow it.
func logGroupOwner(group, prefix string) string {
	rest, ok := strings.CutPrefix(group, prefix)
	if !ok {
		return ""
	}
	owner, _, _ := strings.Cut(rest, "/")
	return owner
}

// acmValidationDomain returns the domain an ACM DNS validation record name
// ("_<token>.<domain>") validates.
func acmValidationDomain(name string) (string, bool) {
	token, domain, ok := strings.Cut(canonicalDNS(name), ".")
	return domain, ok && strings.HasPrefix(token, "_")
}

// certDomain returns the domain a certificate name covers, without the
// wildcard label: the one its DNS validation record validates.
func certDomain(name string) string {
	return strings.TrimPrefix(canonicalDNS(name), "*.")
}

// emailDomain returns the domain of an e-mail address: what follows its last
// "@".
func emailDomain(addr string) (string, bool) {
	i := strings.LastIndex(addr, "@")
	return addr[i+1:], i >= 0
}

// executeAPIHostID returns the API ID an API Gateway default endpoint host
// ("<api-id>.execute-api.<region>.amazonaws.com") carries, or "" for any other
// host.
func executeAPIHostID(host string) string {
	id, _, ok := strings.Cut(host, ".execute-api.")
	if !ok {
		return ""
	}
	return id
}
