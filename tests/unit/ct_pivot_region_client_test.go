package unit_test

// A CloudTrail lookup pinned to another Region needs a client for that
// Region. It is built from the session's own config, once per Region — and
// only when the session is really talking to AWS: a session whose CloudTrail
// client was replaced (demo, tests) is served that replacement wherever the
// lookup is routed, or --demo would reach the network.

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
)

func TestCTRegion_ClientForAnotherRegion(t *testing.T) {
	live := awsclient.CreateServiceClients(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", ""),
	})
	if live.CloudTrailIn("us-east-1") != live.CloudTrail {
		t.Error("the session Region asked for a client of its own")
	}
	if live.CloudTrailIn("") != live.CloudTrail {
		t.Error("an unpinned lookup asked for a client of its own")
	}
	other := live.CloudTrailIn("eu-central-1")
	if other == live.CloudTrail {
		t.Fatal("a lookup pinned to another Region reused the session client")
	}
	if again := live.CloudTrailIn("eu-central-1"); again != other {
		t.Error("a second lookup in the same Region built a second client")
	}
}

func TestCTRegion_SubstitutedClientServesEveryRegion(t *testing.T) {
	d := demo.NewServiceClients()
	for _, region := range []string{"", "us-east-1", "eu-west-1"} {
		if d.CloudTrailIn(region) != d.CloudTrail {
			t.Errorf("demo lookup in %q did not use the demo CloudTrail client", region)
		}
	}
}
