package unit

// misc4_r1_exposure_phrase_test.go — misc4 row 1.
//
// A registered wording is read aloud by an operator, so a hedge like "port(s)"
// outside a slot reaches the screen verbatim: the reader is told the tool does
// not know whether there is one port or several, when the emitter knew. The
// slot machinery already agrees number off the value ("<port(s) LIST>"), and
// the wide-open case is a different sentence, not a list whose only member is
// the word "all".

import (
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
)

const (
	misc4CodeExposed    = domain.FindingCode("ec2.internet-exposed")
	misc4CodeExposedAll = domain.FindingCode("ec2.internet-exposed-all")
)

// TestEC2ExposedOnePortReadsSingular pins that one open port is announced in
// the singular, off the slot's own agreement.
func TestEC2ExposedOnePortReadsSingular(t *testing.T) {
	const id = "i-0misc4one0aaaaa1"
	cache := pw1SGCache(t, pw1SG("sg-0misc4ssh0aaaaa1", 22, 22, false))

	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache,
		pw1Instance(id, "running", "203.0.113.40", ec2types.HttpTokensStateRequired, "sg-0misc4ssh0aaaaa1"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireFinding(t, res.Findings[id], misc4CodeExposed,
		"port 22 reachable from the internet", domain.SevBroken, "wave2")
}

// TestEC2ExposedTwoPortsReadsPlural pins the plural form of the same slot.
func TestEC2ExposedTwoPortsReadsPlural(t *testing.T) {
	const id = "i-0misc4two0aaaaa1"
	cache := pw1SGCache(t,
		pw1SG("sg-0misc4ssh1aaaaa1", 22, 22, false),
		pw1SG("sg-0misc4rdp1aaaaa1", 3389, 3389, false))

	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache,
		pw1Instance(id, "running", "203.0.113.41", ec2types.HttpTokensStateRequired,
			"sg-0misc4ssh1aaaaa1", "sg-0misc4rdp1aaaaa1"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireFinding(t, res.Findings[id], misc4CodeExposed,
		"ports 22, 3389 reachable from the internet", domain.SevBroken, "wave2")
}

// TestEC2ExposedWideOpenIsItsOwnCode pins that an all-protocols group is a
// sentence of its own rather than a port list whose one member is the word
// "all", and that the port-list code stays silent for it.
func TestEC2ExposedWideOpenIsItsOwnCode(t *testing.T) {
	const id = "i-0misc4wide0aaaa1"
	cache := pw1SGCache(t, pw1SG("sg-0misc4wide0aaaa1", 0, 0, true))

	res, err := pw1EnrichEC2(t, &pw1EC2EnrichFake{}, cache,
		pw1Instance(id, "running", "203.0.113.42", ec2types.HttpTokensStateRequired, "sg-0misc4wide0aaaa1"))
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	pw1RequireFinding(t, res.Findings[id], misc4CodeExposedAll,
		"every port reachable from the internet", domain.SevBroken, "wave2")
	pw1RequireNoFinding(t, res.Findings[id], misc4CodeExposed)
	pw1RequireRow(t, pw1Rows(res, id, misc4CodeExposedAll), "Ports", "all")
}
