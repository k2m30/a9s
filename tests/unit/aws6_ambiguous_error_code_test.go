package unit_test

// aws6_ambiguous_error_code_test.go — an ambiguous error code is not a fact on
// its own.
//
// Most services answer a deleted resource with a code that says so and says
// nothing else: ResourceNotFoundException, NoSuchBucket, NoSuchHostedZone. Three
// do not. autoscaling and cloudformation both answer ValidationError, and athena
// answers InvalidRequestException — and each of those codes is ALSO what the
// service returns for a request that was simply malformed.
//
// Classifying on the code alone therefore reads a caller's own bad request as a
// resource that went away, and MarkSkipped then swallows it: the row is marked
// uninspected and no failure is recorded, so the operator is told nothing went
// wrong while nothing was in fact inspected. That is the one thing the partial
// answer machinery exists to prevent.
//
// The rule pinned here: for these three codes, code plus the service's
// not-found message shape classifies as not-found; the same code with any other
// message does not, and is reported as a failure.

import (
	"strings"
	"testing"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// ambiguousCodeCase is one service's two answers under one code: the shape it
// uses for a resource that is gone, and the shape it uses for a request it
// could not parse.
//
// The messages are the ones the services actually send. A shape invented for
// the test would let a matcher pass here and miss in production, which is the
// failure mode a message-based rule has and a code-based one does not, so the
// wording is the whole point of the fixture.
var ambiguousCodeCases = []struct {
	service     string
	code        string
	notFoundMsg string
	malformed   string
}{
	{
		service:     "asg",
		code:        "ValidationError",
		notFoundMsg: "AutoScalingGroup name not found - AutoScalingGroup: acme-not-in-any-fixture not found",
		malformed: "1 validation error detected: Value '' at 'autoScalingGroupName' failed to satisfy " +
			"constraint: Member must have length greater than or equal to 1",
	},
	{
		service:     "cfn",
		code:        "ValidationError",
		notFoundMsg: "Stack with id acme-not-in-any-fixture does not exist",
		malformed: "1 validation error detected: Value 'acme not in any fixture' at 'stackName' failed to " +
			"satisfy constraint: Member must satisfy regular expression pattern: [a-zA-Z][-a-zA-Z0-9]*",
	},
	{
		service:     "athena",
		code:        "InvalidRequestException",
		notFoundMsg: "WorkGroup acme-not-in-any-fixture is not found.",
		malformed:   "line 1:1: mismatched input 'SELCT'. Expecting: 'ALTER', 'ANALYZE', 'CALL'",
	},
}

// TestAmbiguousCodeNeedsTheNotFoundShape pins both sides per service.
func TestAmbiguousCodeNeedsTheNotFoundShape(t *testing.T) {
	for _, c := range ambiguousCodeCases {
		t.Run(c.service+"/not_found", func(t *testing.T) {
			err := &smithy.GenericAPIError{Code: c.code, Message: c.notFoundMsg}
			if !awsclient.IsNotFoundErr(err) {
				t.Errorf("%s: %s with the service's not-found message %q does not classify as not-found; "+
					"the row went away between the list call and this one, which is a race to skip, not a "+
					"failure to report", c.service, c.code, c.notFoundMsg)
			}
		})

		t.Run(c.service+"/malformed", func(t *testing.T) {
			err := &smithy.GenericAPIError{Code: c.code, Message: c.malformed}
			if awsclient.IsNotFoundErr(err) {
				t.Errorf("%s: %s with a malformed-request message %q classifies as not-found; a request a9s "+
					"built wrong then reads as a resource that went away, and the operator is told nothing "+
					"failed while nothing was inspected", c.service, c.code, c.malformed)
			}
		})
	}
}

// TestMarkSkippedReportsAMalformedRequest pins the consequence at the one site
// that acts on the classification.
//
// Both answers leave the row uninspected — that is not in question. What
// separates them is whether the operator is told: a race is silent, a request
// a9s got wrong is a failure with the id on it.
func TestMarkSkippedReportsAMalformedRequest(t *testing.T) {
	for _, c := range ambiguousCodeCases {
		t.Run(c.service, func(t *testing.T) {
			const id = "acme-not-in-any-fixture"

			raceResult := awsclient.IssueEnricherResult{TruncatedIDs: map[string]string{}}
			var raceFailures []awsclient.Failure
			awsclient.MarkSkipped(&raceResult, id,
				&raceFailures, &smithy.GenericAPIError{Code: c.code, Message: c.notFoundMsg})

			if _, marked := raceResult.TruncatedIDs[id]; !marked {
				t.Errorf("%s: a skipped row must still be marked uninspected so it renders \"?\"", c.service)
			}
			if len(raceFailures) != 0 {
				t.Errorf("%s: a resource that went away recorded %d failure(s) (%+v); the race is silent",
					c.service, len(raceFailures), raceFailures)
			}

			badResult := awsclient.IssueEnricherResult{TruncatedIDs: map[string]string{}}
			var badFailures []awsclient.Failure
			awsclient.MarkSkipped(&badResult, id,
				&badFailures, &smithy.GenericAPIError{Code: c.code, Message: c.malformed})

			if _, marked := badResult.TruncatedIDs[id]; !marked {
				t.Errorf("%s: a row whose call was malformed is uninspected too and still renders \"?\"", c.service)
			}
			if len(badFailures) != 1 {
				t.Fatalf("%s: a malformed request recorded %d failure(s), want exactly 1 — %s with a "+
					"non-not-found message is a failure the operator has to account for",
					c.service, len(badFailures), c.code)
			}
			if badFailures[0].ID != id {
				t.Errorf("%s: recorded failure names %q, want the id %q that failed", c.service, badFailures[0].ID, id)
			}
		})
	}
}

// TestUnambiguousCodesStillClassifyOnCodeAlone is the guard against fixing the
// three ambiguous codes by giving every code a message rule.
//
// A code that only ever means "gone" carries the whole fact; requiring a
// message shape from it would make classification depend on wording AWS is free
// to reword, and every service would break the day it did.
func TestUnambiguousCodesStillClassifyOnCodeAlone(t *testing.T) {
	unambiguous := []string{
		"ResourceNotFoundException",
		"NoSuchBucket",
		"NoSuchHostedZone",
		"NoSuchEntity",
		"NotFoundException",
		"ClusterNotFoundException",
		"LoadBalancerNotFound",
	}
	for _, code := range unambiguous {
		t.Run(code, func(t *testing.T) {
			for _, msg := range []string{
				"",
				"something the service worded some other way",
				"1 validation error detected: Value '' at 'name'",
			} {
				err := &smithy.GenericAPIError{Code: code, Message: msg}
				if !awsclient.IsNotFoundErr(err) {
					t.Errorf("%s with message %q no longer classifies as not-found; this code means one "+
						"thing, so it classifies on the code and never on wording AWS can change",
						code, msg)
				}
			}
		})
	}

	// The negative case: a code that means neither is still not not-found,
	// whatever it says.
	for _, code := range []string{"AccessDenied", "Throttling", "RequestLimitExceeded"} {
		err := &smithy.GenericAPIError{Code: code, Message: "acme-not-in-any-fixture does not exist"}
		if awsclient.IsNotFoundErr(err) {
			t.Errorf("%s classifies as not-found because its MESSAGE reads like one; the message narrows an "+
				"ambiguous code, it never promotes an unrelated one", code)
		}
	}
}

// TestAmbiguousCodeShapeIsNotASubstringOfTheWordNotFound guards the lazy
// matcher: "does the message contain 'not found'" passes every case above and
// still mistakes a malformed request that happens to quote a missing argument.
func TestAmbiguousCodeShapeIsNotASubstringOfTheWordNotFound(t *testing.T) {
	// A real autoscaling validation failure whose message contains the words
	// "not found" while describing the REQUEST, not a resource.
	err := &smithy.GenericAPIError{
		Code:    "ValidationError",
		Message: "1 validation error detected: parameter 'AutoScalingGroupName' not found in the request",
	}
	if awsclient.IsNotFoundErr(err) {
		t.Errorf("a malformed request classifies as not-found because its message contains %q; "+
			"the shape a service uses for a missing resource is more specific than that phrase",
			"not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatal("test sanity: this fixture is only meaningful while its message contains the phrase")
	}
}
