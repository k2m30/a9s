package app_test

import (
	"os"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestMain(m *testing.M) {
	awsclient.Install()
	resource.WireProjection()
	os.Exit(m.Run())
}
