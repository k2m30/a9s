package unit

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// S3 User Guide, Blocking public access, The meaning of "public": "Bucket
// policies that grant access conditioned on the aws:SourceIp condition key
// with very broad IP ranges (for example, 0.0.0.0/1) are evaluated as
// "public." This includes values broader than /8 for IPv4 and /32 for IPv6
// (excluding RFC1918 private ranges)." A range of exactly /8 or /32 is not
// broader. CIDROpenToEveryone answers "every address", which only a prefix of
// length 0 reaches.
func TestT3a_Dev_BroadSourceIpRangeIsPublic(t *testing.T) {
	cases := []struct {
		cidr          string
		public, every bool
	}{
		{"0.0.0.0/1", true, false},
		{"128.0.0.0/7", true, false},
		{"10.0.0.0/8", false, false},
		{"198.51.100.0/8", false, false},
		{"2000::/31", true, false},
		{"2001:db8::/32", false, false},
		{"203.0.113.10", false, false},
		{"2001:db8::1", false, false},
		{"not-a-range", true, false},
		{"${aws:SourceIp}", true, false},
	}
	for _, c := range cases {
		t.Run(c.cidr, func(t *testing.T) {
			g := t3aGrant{t3aPublish, t3aTopicARN, "IpAddress", "aws:SourceIp", []string{c.cidr}}
			t3aCheck(t, iampolicy.Evaluate(g.doc(t), t3aOwnAccount), g.actions, c.public)
			if got := awsclient.CIDROpenToEveryone([]string{c.cidr}); got != c.every {
				t.Errorf("CIDROpenToEveryone(%q) = %v, want %v", c.cidr, got, c.every)
			}
		})
	}
}

// A policy variable is not a fixed value ("values that don't contain a
// wildcard or an AWS Identity and Access Management Policy Variable", same
// page), and the trailing "/*" an organization path may carry does not make
// any other key's value fixed.
func TestT3a_Dev_PolicyVariableAndOrgPathSuffix(t *testing.T) {
	cases := []struct {
		name   string
		grant  t3aGrant
		public bool
	}{
		{"principal_arn_variable", t3aGrant{t3aPublish, t3aTopicARN, "ArnLike", "aws:PrincipalArn", []string{"arn:aws:iam::123456789012:user/${aws:username}"}}, true},
		{"principal_org_paths_variable", t3aGrant{t3aPublish, t3aTopicARN, "ForAnyValue:StringLike", "aws:PrincipalOrgPaths", []string{"${aws:PrincipalOrgID}/*"}}, true},
		{"source_arn_trailing_slash_star", t3aGrant{t3aPublish, t3aTopicARN, "ArnLike", "aws:SourceArn", []string{"arn:aws:s3:::acme-bucket/*"}}, true},
		{"org_paths_bare_suffix", t3aGrant{t3aPublish, t3aTopicARN, "ForAnyValue:StringLike", "aws:PrincipalOrgPaths", []string{"/*"}}, true},
		{"org_paths_inner_wildcard", t3aGrant{t3aPublish, t3aTopicARN, "ForAnyValue:StringLike", "aws:PrincipalOrgPaths", []string{"o-acme123456/*/ou-ab12-11111111/*"}}, true},
		{"org_paths_fixed_ou", t3aGrant{t3aPublish, t3aTopicARN, "ForAnyValue:StringEquals", "aws:PrincipalOrgPaths", []string{"o-acme123456/r-ab12/ou-ab12-11111111/"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t3aCheck(t, iampolicy.Evaluate(c.grant.doc(t), t3aOwnAccount), c.grant.actions, c.public)
		})
	}
}
