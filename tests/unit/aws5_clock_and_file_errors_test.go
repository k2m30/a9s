package unit

// aws5_clock_and_file_errors_test.go — two failures that only show up later:
// a demo witness stamped once at startup, and a local file error wearing the
// network's class word.

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ── The demo event witness holds for the life of a session ────────────────

// TestECSDemo_EventWitnessSurvivesALongSession pins the clock. The enricher
// reads a ten-minute window, so an event stamped once when the fixtures were
// built ages out of a session left open longer than that: the finding's Event
// and Reason rows vanish and the demo bench drifts with the wall clock. The
// fake stamps the event against the call instead.
func TestECSDemo_EventWitnessSurvivesALongSession(t *testing.T) {
	fake := fakes.NewECS()
	built := time.Now()
	fake.Now = func() time.Time { return built.Add(time.Hour) }

	out, err := fake.DescribeServices(context.Background(), &ecs.DescribeServicesInput{
		Cluster:  aws.String("acme-services"),
		Services: []string{fixtures.ECSServiceNoTasksRunning},
	})
	if err != nil {
		t.Fatalf("DescribeServices: %v", err)
	}
	if len(out.Services) != 1 {
		t.Fatalf("got %d services, want 1", len(out.Services))
	}
	if evs := out.Services[0].Events; len(evs) != 1 || evs[0].CreatedAt == nil {
		t.Fatalf("the witness served no stamped event: %+v", evs)
	}

	res, err := awsclient.EnrichECSServices(context.Background(),
		&awsclient.ServiceClients{ECS: fake, Region: "us-east-1"},
		[]resource.Resource{{
			ID:   fixtures.ECSServiceNoTasksRunning,
			Name: fixtures.ECSServiceNoTasksRunning,
			Type: "ecs-svc",
			Fields: map[string]string{
				"cluster":      "acme-services",
				"service_name": fixtures.ECSServiceNoTasksRunning,
			},
		}}, nil)
	if err != nil {
		t.Fatalf("EnrichECSServices: %v", err)
	}
	rows := res.AttentionDetails[fixtures.ECSServiceNoTasksRunning][d3CodeECSDeployFailed].Rows
	if !aws5HasRow(rows, "Event", "unable to place task") {
		t.Errorf("rows = %v an hour into the session, want the placement event still there", rows)
	}
}

// ── A local file failure is not a transport failure ───────────────────────

// TestErrClass_LocalFileFailureIsNotTransport pins the three shapes a local
// file error takes. Every one of them unwraps to a syscall.Errno, which
// carries Timeout and Temporary and so satisfies net.Error — which is how a
// missing config file came to read "transport failure", pointing the operator
// at the network for a problem on their own disk.
func TestErrClass_LocalFileFailureIsNotTransport(t *testing.T) {
	dir := t.TempDir()

	unreadable := filepath.Join(dir, "unreadable.yaml")
	if err := os.WriteFile(unreadable, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	_, missingErr := os.ReadFile(filepath.Join(dir, "absent.yaml"))
	_, modeErr := os.ReadFile(unreadable)
	_, dirErr := os.ReadFile(dir)
	renameErr := os.Rename(filepath.Join(dir, "absent.yaml"), filepath.Join(dir, "moved.yaml"))

	for _, tc := range []struct {
		name  string
		err   error
		words string
	}{
		{"missing file", missingErr, "no such file or directory"},
		{"mode 000", modeErr, "permission denied"},
		{"a directory", dirErr, "is a directory"},
		// os.Rename answers *os.LinkError, not *fs.PathError; both unwrap to
		// the same Errno, so excluding only one shape would leave this one
		// reading as the network.
		{"rename of a missing file", renameErr, "no such file or directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatal("the operating system reported no error; the case proves nothing")
			}
			if got := awsclient.ErrClass(tc.err); got == "transport" {
				t.Errorf("ErrClass(%v) = %q: this is the local disk, not the network path", tc.err, got)
			}
			if cause := awsclient.CauseOf(tc.err); !strings.Contains(cause, tc.words) {
				t.Errorf("CauseOf(%v) = %q, want the operating system's own words %q", tc.err, cause, tc.words)
			}
		})
	}
}

// TestErrClass_RefusedConnectionIsStillTransport is the other half: narrowing
// the arm must not stop recognising the shape it was written for.
func TestErrClass_RefusedConnectionIsStillTransport(t *testing.T) {
	if got := awsclient.ErrClass(aws5RefusedTransportErr()); got != "transport" {
		t.Errorf("ErrClass(refused connection) = %q, want %q", got, "transport")
	}
}

// aws5RefusedTransportErr is the shape an AWS call that never reached a
// service arrives in: the SDK's operation error over net/http's client
// wrapper over the dial failure.
func aws5RefusedTransportErr() error {
	return &smithy.OperationError{
		ServiceID: "EC2", OperationName: "DescribeInstances",
		Err: &url.Error{Op: "Post", URL: "https://ec2.eu-west-1.amazonaws.com/",
			Err: &net.OpError{Op: "dial", Net: "tcp",
				Addr: &net.TCPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 443},
				Err:  errors.New("connect: connection refused")}},
	}
}
