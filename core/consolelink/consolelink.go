// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package consolelink builds AWS Management Console deep-link URLs for a9s
// resources, backing the TUI's "o" (open) / "O" (copy) keys and the web
// client's equivalent actions.
package consolelink

import (
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

// consoleHosts lists every partition console host Valid accepts, either as
// an exact match or as the suffix of a "<region>." subdomain.
var consoleHosts = []string{ //nolint:gochecknoglobals // fixed partition list, not configuration
	"console.aws.amazon.com",
	"console.amazonaws-us-gov.com",
	"console.amazonaws.cn",
}

// Domain returns the partition console host for region: GovCloud regions use
// console.amazonaws-us-gov.com, China regions use console.amazonaws.cn, and
// every other region uses the commercial console.aws.amazon.com.
func Domain(region string) string {
	switch {
	case strings.HasPrefix(region, "us-gov-"):
		return "console.amazonaws-us-gov.com"
	case strings.HasPrefix(region, "cn-"):
		return "console.amazonaws.cn"
	default:
		return "console.aws.amazon.com"
	}
}

// Regional builds a region-subdomain console URL for tail (e.g. an EC2 or
// RDS deep link, which AWS always pins to a region subdomain).
func Regional(region, tail string) string {
	return "https://" + region + "." + Domain(region) + "/" + tail
}

// Global builds a partition-aware console URL for tail with no region
// subdomain (e.g. S3, IAM, Route 53 — services whose console has no
// per-region host).
func Global(region, tail string) string {
	return "https://" + Domain(region) + "/" + tail
}

// GoView builds a console URL via AWS's generic ARN resolver
// (/go/view?arn=...). GET only — the endpoint 400s on HEAD, so callers must
// open it in a browser rather than probe it first.
func GoView(region, arn string) string {
	return Global(region, "go/view?arn="+url.QueryEscape(arn))
}

// RDSKind is the page an RDS-API row opens.
type RDSKind int

const (
	RDSInstance RDSKind = iota
	RDSCluster
	RDSInstanceSnapshot
	RDSClusterSnapshot
)

// RDSFamily is the console link of a row the RDS API lists, which serves the
// RDS, DocumentDB and Neptune engine families: the engine picks the console.
// A DocumentDB cluster opens its cluster page; any other DocumentDB row and
// every Neptune row open that service's console home for the region. A
// cluster with no engine has no link (a DocumentDB or Neptune cluster would
// be sent to the wrong console). Any other row with no engine opens RDS: a DB
// snapshot is always an RDS snapshot (DocumentDB and Neptune have cluster
// snapshots only), and RDS is the console of the API that lists the row.
func RDSFamily(engine string, kind RDSKind, id, region string) string {
	engine = strings.ToLower(engine)
	switch {
	case engine == "" && kind == RDSCluster:
		return ""
	case strings.HasPrefix(engine, "docdb"):
		if kind == RDSCluster {
			return Regional(region, "docdb/home?region="+region+"#cluster-details/"+url.PathEscape(id))
		}
		return Regional(region, "docdb/home?region="+region)
	case strings.HasPrefix(engine, "neptune"):
		return Regional(region, "neptune/home?region="+region)
	}
	switch kind {
	case RDSCluster:
		return Regional(region, "rds/home?region="+region+"#database:id="+url.PathEscape(id)+";is-cluster=true")
	case RDSInstanceSnapshot, RDSClusterSnapshot:
		return Regional(region, "rds/home?region="+region+"#db-snapshot:id="+url.PathEscape(id))
	default:
		return Regional(region, "rds/home?region="+region+"#database:id="+url.PathEscape(id)+";is-cluster=false")
	}
}

// AccountFromARN returns the account ID ref names, or "" when ref is not a
// well-formed ARN.
func AccountFromARN(ref string) string {
	a, err := arn.Parse(ref)
	if err != nil {
		return ""
	}
	return a.AccountID
}

// Valid reports whether u is a well-formed https console URL whose host is
// one of consoleHosts or a "<region>." subdomain of one — the guard
// openBrowserCmd runs before ever exec'ing a URL.
func Valid(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || strings.HasPrefix(parsed.Host, ".") {
		return false
	}
	for _, host := range consoleHosts {
		if parsed.Host == host || strings.HasSuffix(parsed.Host, "."+host) {
			return true
		}
	}
	return false
}

// Resolve returns the console URL for r under td, or ("", false) when the
// type has no console page and r carries no ARN for the generic resolver
// fallback. td.ConsoleURL wins when it resolves to a non-empty URL;
// otherwise Fields["arn"], if present, is resolved via GoView.
func Resolve(td catalog.ResourceTypeDef, r domain.Resource, region, accountID string) (string, bool) {
	if td.ConsoleURL != nil {
		if u := td.ConsoleURL(r, region, accountID); u != "" {
			return u, true
		}
	}
	if arn := r.Fields["arn"]; arn != "" {
		return GoView(region, arn), true
	}
	return "", false
}
