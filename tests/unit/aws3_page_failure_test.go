// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// aws3_page_failure_test.go — spec row 6 (task aws3), the paged-walk half: a
// walk stopped by a failed page names the page as a page. Every other id in a
// composite error is something the operator can go and look at, and "page 2"
// posing as one sends them looking for a resource that does not exist.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backupsdk "github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// backupWalkFailsOnSecondPage answers one page with a token and then refuses.
type backupWalkFailsOnSecondPage struct {
	awsclient.BackupAPI
	calls int
}

func (f *backupWalkFailsOnSecondPage) ListBackupJobs(
	_ context.Context, _ *backupsdk.ListBackupJobsInput, _ ...func(*backupsdk.Options),
) (*backupsdk.ListBackupJobsOutput, error) {
	f.calls++
	if f.calls == 1 {
		return &backupsdk.ListBackupJobsOutput{NextToken: aws.String("p2")}, nil
	}
	return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "User is not authorized to perform: backup:ListBackupJobs"}
}

func TestPagedWalkFailureReadsAsAPage(t *testing.T) {
	clients := &awsclient.ServiceClients{Backup: &backupWalkFailsOnSecondPage{}}
	rows := []resource.Resource{{ID: "plan-1", Name: "plan-1", Fields: map[string]string{"plan_id": "plan-1"}}}

	_, err := awsclient.EnrichBackupJobs(context.Background(), clients, rows, nil)
	if err == nil {
		t.Fatal("expected a composite error when the walk's second page was refused")
	}
	if strings.Contains(err.Error(), "e.g. page") {
		t.Errorf("composite = %q — a page is offered as an example resource id", err.Error())
	}
	if !strings.Contains(err.Error(), "on page 2") {
		t.Errorf("composite = %q, want the failure phrased as being on page 2", err.Error())
	}
}
