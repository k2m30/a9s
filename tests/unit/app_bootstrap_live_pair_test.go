// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
)

// A profile switch that lands before the background connect is dispatched
// wins: authenticating against the pair the caller was started with signs the
// session in to a profile the operator has already left, and every later
// result is attributed to the wrong account.
func TestBootstrapLive_ProfileSwitchBeforeDispatch_ConnectsSwitchedProfile(t *testing.T) {
	const startupProfile = "fake-profile-000000000000"
	const switchedProfile = "fake-profile-111111111111"

	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	awsDir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(awsDir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(awsDir, "credentials"))

	_, ctrl := newHermeticLiveController(t, startupProfile, "us-east-1")
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionSelectProfile, Arg: switchedProfile})

	_ = ctrl.BootstrapLive(startupProfile, "us-east-1")

	flash := ctrl.Snapshot().Header.Flash.Text
	if !strings.Contains(flash, switchedProfile) {
		t.Errorf("connect failure flash = %q, want it to name %q — the startup connect must use the pair the session holds when it is dispatched", flash, switchedProfile)
	}
	if strings.Contains(flash, startupProfile) {
		t.Errorf("connect failure flash = %q, must not name %q — the session left that profile before the connect was dispatched", flash, startupProfile)
	}
}
