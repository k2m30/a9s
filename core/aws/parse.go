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

// PartitionForRegion returns the ARN partition a region belongs to: "aws-cn"
// for the China regions, "aws-us-gov" for GovCloud, "aws" otherwise. The
// prefix is a whole segment, so us-west-2 is commercial and only us-gov- is
// GovCloud.
//
// It is the only place this package decides a partition. An ARN built with a
// hard-coded "aws" is not malformed in the other two — it simply never
// matches anything AWS returned, so the failure reads as a confident zero.
func PartitionForRegion(region string) string {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "aws-cn"
	case strings.HasPrefix(region, "us-gov-"):
		return "aws-us-gov"
	default:
		return "aws"
	}
}

// ARNForService parses an ARN AWS returned and reports whether it names the
// given service. The partition and the region are never compared: an ARN in
// aws-cn or aws-us-gov names the same service as its aws twin, and matching
// the literal "arn:aws:" discards every resource outside the commercial
// partition — a filter that renders as a proven zero rather than an error.
// A malformed or non-ARN string is not a match.
func ARNForService(s, service string) (arn.ARN, bool) {
	if !strings.HasPrefix(s, "arn:aws:"+service+":") {
		return arn.ARN{}, false
	}
	a, err := arn.Parse(s)
	return a, err == nil
}
