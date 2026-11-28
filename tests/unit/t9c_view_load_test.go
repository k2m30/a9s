package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
)

func t9cLoadSQSView(t *testing.T, body string) (*config.ViewsConfig, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sqs.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return config.LoadFromDirs([]string{dir})
}

// A column path that names no field of the struct the sqs fetcher stores is
// blank on every row, so the load reports the type and the path, and keeps
// the rest of the operator's file in use.
func TestT9c_Row4_ViewPathOnNoStoredStructIsReported(t *testing.T) {
	cfg, err := t9cLoadSQSView(t, `list:
  Queue:
    key: queue_name
    width: 30
  Messages:
    path: ApproximateNumberOfMessages
    width: 10
`)
	if err == nil {
		t.Fatal("the load reported nothing for column path ApproximateNumberOfMessages, which names no field of the sqs row")
	}
	msg := err.Error()
	for _, want := range []string{"sqs", "ApproximateNumberOfMessages"} {
		if !strings.Contains(msg, want) {
			t.Errorf("load report %q does not name %q", msg, want)
		}
	}
	if text := config.ReportText(cfg, err); !strings.Contains(text, "ApproximateNumberOfMessages") {
		t.Errorf("flash text %q does not name the path", text)
	}
	if cfg == nil || len(cfg.Views["sqs"].List) != 2 {
		t.Errorf("the operator's sqs file is not in use beside the report: %+v", cfg)
	}
}

// A path that names a field of the stored row, and a key the fetcher
// writes, are both columns something fills: the load says nothing.
func TestT9c_Row4_ViewPathOnTheStoredStructIsNotReported(t *testing.T) {
	_, err := t9cLoadSQSView(t, `list:
  URL:
    path: QueueURL
    width: 40
  Messages:
    key: approx_messages
    width: 10
`)
	if err != nil {
		t.Errorf("the load reported a filled column: %v", err)
	}
}
