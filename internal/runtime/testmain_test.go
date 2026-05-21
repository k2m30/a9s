package runtime

import (
	"os"
	"testing"

	"github.com/k2m30/a9s/v3/internal/aws"
)

// TestMain installs the AWS catalog before any runtime test runs.
// See tests/unit/testmain_test.go for the rationale.
func TestMain(m *testing.M) {
	aws.Install()
	os.Exit(m.Run())
}
