// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// rds_posture.go — the security-posture predicates the RDS instance fetcher
// (rds.go) and both cluster fetchers (dbc.go for DocumentDB, dbc_rds.go for
// Aurora / Multi-AZ) evaluate. Instances and clusters expose the same four
// settings under the same names, so one predicate set answers for all three
// call sites; only the FindingCode differs per short name and is
// passed in via rdsPostureCodes.
package aws

import (
	"strconv"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

// rdsPostureCodes carries the per-short-name FindingCode for each posture
// predicate. dbiPostureCodes and dbcPostureCodes are the two instantiations.
type rdsPostureCodes struct {
	singleAZ          domain.FindingCode
	minorUpgrade      domain.FindingCode
	iamAuth           domain.FindingCode
	defaultMasterUser domain.FindingCode
}

// rdsPosture is the subset of an RDS instance or cluster the posture
// predicates read. Every field is the SDK pointer verbatim: a nil pointer is
// "AWS did not report this" and never produces a finding.
type rdsPosture struct {
	Engine                  string
	MultiAZ                 *bool
	IsReadReplica           bool
	AutoMinorVersionUpgrade *bool
	IAMAuthEnabled          *bool
	MasterUsername          *string
	// SkipSingleAZ suppresses the single-AZ predicate for shapes where
	// MultiAZ is not the operator's lever — Aurora instances inherit the
	// cluster's topology.
	SkipSingleAZ bool
}

// iamAuthEngines are the RDS engine families that support IAM database
// authentication. Engines outside this set never emit the iam-auth-off
// finding: the setting does not exist for them, so its absence is not a
// misconfiguration.
var iamAuthEngines = []string{"mysql", "mariadb", "postgres", "aurora"}

// engineSupportsIAMAuth reports whether engine is one of the families that
// accepts IAM database authentication. Aurora flavours ("aurora-mysql",
// "aurora-postgresql") match on the "aurora" prefix.
func engineSupportsIAMAuth(engine string) bool {
	e := strings.ToLower(engine)
	for _, supported := range iamAuthEngines {
		if e == supported || strings.HasPrefix(e, supported) {
			return true
		}
	}
	return false
}

// defaultMasterUsernames are the vendor-default administrative account names
// AWS offers in the console. Leaving one in place hands an attacker half of
// the credential pair for free.
var defaultMasterUsernames = []string{"admin", "postgres", "root", "mysql", "master", "awsuser"}

// isDefaultMasterUsername reports whether name is one of the vendor defaults,
// case-insensitively.
func isDefaultMasterUsername(name string) bool {
	for _, d := range defaultMasterUsernames {
		if strings.EqualFold(name, d) {
			return true
		}
	}
	return false
}

// rdsPostureFindings evaluates the four posture predicates against p and
// returns the findings plus their supporting AttentionDetail rows. Each
// predicate is independent: two active conditions produce two findings.
func rdsPostureFindings(p rdsPosture, c rdsPostureCodes) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	var findings []domain.Finding
	details := map[domain.FindingCode]domain.AttentionDetail{}

	add := func(code domain.FindingCode, rows []domain.DetailRow) {
		findings = append(findings, wave1Finding(code, domain.SevWarn))
		details[code] = domain.AttentionDetail{Rows: rows}
	}

	if !p.SkipSingleAZ && !p.IsReadReplica && p.MultiAZ != nil && !*p.MultiAZ {
		add(c.singleAZ, []domain.DetailRow{{Label: "Multi-AZ", Value: "no", Tier: "~"}})
	}
	if p.AutoMinorVersionUpgrade != nil && !*p.AutoMinorVersionUpgrade {
		add(c.minorUpgrade, nil)
	}
	if engineSupportsIAMAuth(p.Engine) && p.IAMAuthEnabled != nil && !*p.IAMAuthEnabled {
		add(c.iamAuth, []domain.DetailRow{{Label: "Database authentication", Value: "identity-based, off", Tier: "~"}})
	}
	if p.MasterUsername != nil && isDefaultMasterUsername(*p.MasterUsername) {
		add(c.defaultMasterUser, []domain.DetailRow{{Label: "Master username", Value: *p.MasterUsername, Tier: "~"}})
	}

	if len(details) == 0 {
		return findings, nil
	}
	return findings, details
}

// rdsCACertWindowDays is the horizon at which an approaching RDS CA
// certificate expiry becomes worth reporting; rdsCACertUrgentDays is the
// point at which it stops being a warning and becomes an outage on a clock.
const (
	rdsCACertWindowDays = 90
	rdsCACertUrgentDays = 30
)

// rdsCACertFinding returns the CA-certificate-expiry finding for validTill,
// or nil when the certificate is absent or expires beyond the reporting
// window. now is injected so tests can pin the boundary (the acm.go pattern).
//
// Inside rdsCACertUrgentDays the signal is a different one — an outage on a
// clock rather than a task to schedule — so it is a different code carrying
// its own declared severity, the way acm splits expires-soon from
// expires-critical. The severity and the row tier are read back from that
// declaration rather than chosen here.
func rdsCACertFinding(caID string, validTill *time.Time, now time.Time) (*domain.Finding, []domain.DetailRow) {
	if validTill == nil {
		return nil, nil
	}
	days := int(validTill.Sub(now).Hours() / 24)
	if days > rdsCACertWindowDays {
		return nil, nil
	}
	// An already-expired certificate reports 0 rather than a negative count:
	// the phrase is a countdown, and "expires in -12 days" reads as a bug.
	if days < 0 {
		days = 0
	}
	code := CodeDBICACertExpiring
	if days <= rdsCACertUrgentDays {
		code = CodeDBICACertExpiringUrgent
	}
	f := wave1Finding(code, catalog.Severity(code), strconv.Itoa(days))
	tier := "~"
	if f.Severity == domain.SevBroken {
		tier = "!"
	}
	rows := []domain.DetailRow{
		{Label: "Certificate authority", Value: caID, Tier: tier},
		{Label: "Valid till", Value: validTill.Format("2006-01-02"), Tier: tier},
	}
	return &f, rows
}
