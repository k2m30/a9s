package unit

import (
	"os"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

func TestMain(m *testing.M) {
	// TEST_SKIP_INSTALL=1 lets sub-process tests exercise the
	// panic-before-SetTypes path without triggering a bootstrap here.
	if os.Getenv("TEST_SKIP_INSTALL") != "1" {
		awsclient.Install()
	}
	os.Exit(m.Run())
}
