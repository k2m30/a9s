// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "slices"

// CIDROpenToEveryone reports whether a list of CIDR blocks reaches every
// address on the internet. A prefix of length zero in either family says so:
// "0.0.0.0/0" and "::/0" are the same statement about different address
// spaces, and a dual-stack resource open to all IPv6 clients is open. An
// entry that does not parse names no range and is ignored.
//
// It is the single owner of that question for every site in this package that
// tests a CIDR list for "everyone".
func CIDROpenToEveryone(cidrs []string) bool {
	return slices.Contains(cidrs, "0.0.0.0/0")
}

// S3OriginBucket returns the bucket an origin domain name addresses, and
// whether the domain is an S3 endpoint at all. The bucket is the whole prefix
// before the endpoint marker, so a dotted bucket name keeps every label; the
// suffix is the partition's, so China endpoints ending in amazonaws.com.cn
// are S3 endpoints too.
func S3OriginBucket(host string) (string, bool) {
	return cfS3OriginBucket(host)
}

// PartitionForRegion returns the ARN partition a region belongs to: "aws-cn"
// for the China regions, "aws-us-gov" for GovCloud, "aws" otherwise. It is
// the only place this package decides a partition — an ARN built with a
// hard-coded "aws" never matches in the other two.
func PartitionForRegion(region string) string {
	return "aws"
}
