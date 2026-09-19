package unit_test

// The shape that narrows an ambiguous code is the service's SENTENCE, not the
// phrase inside it. A cloudformation validation failure can say "does not
// exist" about an argument while saying nothing about a stack, so a shape must
// be a fragment only the not-found sentence contains — "Stack with id", which
// cloudformation writes before the name — rather than the verb both sentences
// share.

import (
	"strings"
	"testing"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// malformedRequestsThatBorrowTheNotFoundVerb are requests the service could not
// process, each worded so that a shape taken from the not-found sentence's VERB
// rather than its subject would match it.
var malformedRequestsThatBorrowTheNotFoundVerb = []struct {
	service string
	code    string
	message string
	borrows string
}{
	{
		service: "cfn",
		code:    "ValidationError",
		message: "1 validation error detected: Value 'x' at 'parameterKey' does not exist in the template",
		borrows: "does not exist",
	},
	{
		service: "asg",
		code:    "ValidationError",
		message: "1 validation error detected: parameter 'AutoScalingGroupName' not found in the request",
		borrows: "not found",
	},
}

func TestAmbiguousShapeIsTheSentenceNotItsVerb(t *testing.T) {
	for _, c := range malformedRequestsThatBorrowTheNotFoundVerb {
		t.Run(c.service, func(t *testing.T) {
			err := &smithy.GenericAPIError{Code: c.code, Message: c.message}
			if !strings.Contains(c.message, c.borrows) {
				t.Fatalf("test sanity: this fixture is only meaningful while its message contains %q", c.borrows)
			}
			if awsclient.IsNotFoundErr(err) {
				t.Errorf("%s: %s with the malformed-request message %q classifies as not-found because it "+
					"contains %q — the shape has to be a fragment only the service's not-found SENTENCE "+
					"carries, not the verb the two sentences share",
					c.service, c.code, c.message, c.borrows)
			}
		})
	}

	// The real not-found sentence still matches.
	for _, msg := range []string{
		"Stack with id acme-not-in-any-fixture does not exist",
		"AutoScalingGroup name not found - AutoScalingGroup: acme-not-in-any-fixture not found",
	} {
		if !awsclient.IsNotFoundErr(&smithy.GenericAPIError{Code: "ValidationError", Message: msg}) {
			t.Errorf("the service's own not-found sentence %q no longer classifies as not-found", msg)
		}
	}
}
