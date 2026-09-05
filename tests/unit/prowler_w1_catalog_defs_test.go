package unit

// prowler_w1_catalog_defs_test.go — pins the catalog row every batch-w1
// compute finding needs.
//
// A FindingDef is what lets the findings overview, the machine registry and
// the docs-sync gate name a code without having seen it emitted. A code that
// is emitted but not declared renders as a bare string in those surfaces, and
// a declared Phrase that has drifted from the emitted one makes the overview
// disagree with the list.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

type pw1DefExpectation struct {
	shortName string
	code      domain.FindingCode
	phrase    string
	severity  domain.Severity
	source    string
}

// pw1ComputeDefs is the batch-w1 row table, transcribed. Phrases with a
// variable part are written with the catalog's placeholder convention.
var pw1ComputeDefs = []pw1DefExpectation{
	{"ec2", "ec2.imdsv1-allowed", "IMDSv1 allowed", domain.SevWarn, "wave1"},
	{"ec2", "ec2.public-ip", "public address", domain.SevWarn, "wave1"},
	{"ec2", "ec2.internet-exposed", "port(s) <list> reachable from the internet", domain.SevBroken, "wave2"},
	{"ec2", "ec2.user-data-secret", "credential in user data", domain.SevBroken, "wave2"},
	{"ami", "ami.public", "shared with all AWS accounts", domain.SevBroken, "wave1"},
	{"ecs-svc", "ecs-svc.public-ip", "tasks get public IPs", domain.SevWarn, "wave2"},
	{"ecs-task", "ecs-task.privileged", "privileged container", domain.SevBroken, "wave2"},
	{"ecs-task", "ecs-task.host-namespace", "shares the host network or process namespace", domain.SevWarn, "wave2"},
	{"ecs-task", "ecs-task.writable-root", "writable root filesystem", domain.SevWarn, "wave2"},
	{"ecs-task", "ecs-task.no-logging", "container without log driver", domain.SevWarn, "wave2"},
	{"ecs-task", "ecs-task.env-secret", "credential in container environment", domain.SevBroken, "wave2"},
	{"lambda", "lambda.public-policy", "invokable by anyone", domain.SevBroken, "wave2"},
	{"lambda", "lambda.function-url-public", "function endpoint open without authentication", domain.SevBroken, "wave2"},
	{"lambda", "lambda.env-secret", "credential in environment variables", domain.SevBroken, "wave1"},
	{"asg", "asg.launch-config.legacy", "uses a launch configuration", domain.SevWarn, "wave1"},
	{"asg", "asg.single-az", "single availability zone", domain.SevWarn, "wave1"},
	{"asg", "asg.no-elb-health-check", "no load balancer health check", domain.SevWarn, "wave1"},
	{"asg", "asg.launch-config.imdsv1", "launch configuration allows IMDSv1", domain.SevWarn, "wave2"},
	{"asg", "asg.launch-config.public-ip", "launch configuration assigns public IPs", domain.SevWarn, "wave2"},
	{"asg", "asg.launch-config.secret", "credential in launch configuration user data", domain.SevBroken, "wave2"},
	{"ebs-snap", "ebs-snap.public", "shared with all AWS accounts", domain.SevBroken, "wave2"},
	{"lt", "lt.user-data-secret", "credential in user data", domain.SevBroken, "wave2"},
}

// TestProwlerW1_ComputeFindingDefsDeclared pins each new code's catalog row.
func TestProwlerW1_ComputeFindingDefsDeclared(t *testing.T) {
	for _, want := range pw1ComputeDefs {
		td := catalog.Find(want.shortName)
		if td == nil {
			t.Errorf("%s: not in the catalog", want.shortName)
			continue
		}
		var got *catalog.FindingDef
		for i := range td.Findings {
			if td.Findings[i].Code == want.code {
				got = &td.Findings[i]
				break
			}
		}
		if got == nil {
			t.Errorf("%s: no FindingDef for %q", want.shortName, want.code)
			continue
		}
		if got.Phrase != want.phrase {
			t.Errorf("%s %q: Phrase = %q, want %q", want.shortName, want.code, got.Phrase, want.phrase)
		}
		if got.Severity != want.severity {
			t.Errorf("%s %q: Severity = %v, want %v", want.shortName, want.code, got.Severity, want.severity)
		}
		if got.Source != want.source {
			t.Errorf("%s %q: Source = %q, want %q", want.shortName, want.code, got.Source, want.source)
		}
	}
}

// TestProwlerW1_ComputeCodesAreUnique pins that no code is declared twice on
// the same type — a duplicate row makes the findings overview double-count.
func TestProwlerW1_ComputeCodesAreUnique(t *testing.T) {
	for _, want := range pw1ComputeDefs {
		td := catalog.Find(want.shortName)
		if td == nil {
			continue
		}
		n := 0
		for _, def := range td.Findings {
			if def.Code == want.code {
				n++
			}
		}
		if n > 1 {
			t.Errorf("%s: %q declared %d times", want.shortName, want.code, n)
		}
	}
}
