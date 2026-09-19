package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// TestQA_ClientsReady_CorrectType_Assigns verifies the happy path: when
// ClientsReadyMsg carries a *awsclient.ServiceClients, the model assigns it
// and no error flash is surfaced.
func TestQA_ClientsReady_CorrectType_Assigns(t *testing.T) {
	m := newRootSizedModel()

	clients := &awsclient.ServiceClients{}
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Clients: clients,
		Region:  "us-east-1",
		Gen:     1,
	})

	view := stripANSI(rootViewContent(m))

	if strings.Contains(view, "internal:") {
		t.Errorf("unexpected error flash containing 'internal:' in view: %s", view)
	}
	if strings.Contains(view, "unexpected") {
		t.Errorf("unexpected error flash containing 'unexpected' in view: %s", view)
	}
}

// TestQA_ClientsReady_NilClients_DemoFallback verifies that when
// ClientsReadyMsg carries nil Clients, the model falls back to preSuppliedClients
// (the demo path) and no error is surfaced.
//
// tui.WithClients is the public Option that sets preSuppliedClients. We use it
// to construct a model that already has a pre-supplied client set.
func TestQA_ClientsReady_NilClients_DemoFallback(t *testing.T) {
	preSupplied := &awsclient.ServiceClients{}
	m := newBlessedModel(t, "testprofile", "us-east-1", tui.WithClients(preSupplied))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Clients: nil,
		Region:  "us-east-1",
		Gen:     1,
	})

	view := stripANSI(rootViewContent(m))

	if strings.Contains(view, "internal:") {
		t.Errorf("unexpected error flash containing 'internal:' in view: %s", view)
	}
	if strings.Contains(view, "unexpected") {
		t.Errorf("unexpected error flash containing 'unexpected' in view: %s", view)
	}
}

// TestQA_ClientsReady_WrongType_EmitsError: when ClientsReadyMsg.Clients is a
// non-nil value of the wrong concrete type, the handler surfaces an error —
// either via the returned tea.Cmd resolving to a messages.APIError, or via an
// error flash in the view.
func TestQA_ClientsReady_WrongType_EmitsError(t *testing.T) {
	m := newRootSizedModel()

	// "definitely not a client" — a string, which is not *awsclient.ServiceClients.
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, cmd := rootApplyMsg(m, messages.ClientsReady{
		Clients: "definitely not a client",
		Region:  "us-east-1",
		Gen:     1,
	})

	if cmd != nil {
		result := cmd()
		if errMsg, ok := result.(messages.APIError); ok {
			errStr := ""
			if errMsg.Err != nil {
				errStr = errMsg.Err.Error()
			}
			if !strings.Contains(errStr, "unexpected") && !strings.Contains(errStr, "internal") {
				t.Errorf("APIErrorMsg.Err should contain 'unexpected' or 'internal', got: %q", errStr)
			}
			return
		}

		// cmd returned something other than APIErrorMsg; drain it into the model
		// and fall through to the view check.
		result2 := cmd()
		if result2 != nil {
			m, _ = rootApplyMsg(m, result2)
		}
	}

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "internal") && !strings.Contains(view, "unexpected") {
		t.Errorf(
			"expected an error flash containing 'internal' or 'unexpected' when ClientsReadyMsg.Clients has wrong type, but view contained neither.\nView: %s",
			view,
		)
	}
}
