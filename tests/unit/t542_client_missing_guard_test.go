package unit_test

// t542_client_missing_guard_test.go — a related pivot whose target's service
// client is absent reads that list as not read. FetchRelatedTarget detects
// the absent client before the target's fetcher runs; a type that declares no
// client for its fetcher panics or answers something else here, whichever
// type it is.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestFetchRelatedTarget_EveryTargetWithoutItsClientIsClientMissing(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		if resource.GetPaginatedFetcher(td.ShortName) == nil {
			continue
		}
		t.Run(td.ShortName, func(t *testing.T) {
			list, _, err := func() (list []resource.Resource, truncated bool, err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic: %v", r)
					}
				}()
				return awsclient.FetchRelatedTarget(context.Background(), &awsclient.ServiceClients{}, nil, td.ShortName)
			}()
			if list != nil || err == nil || !strings.Contains(err.Error(), "AWS service client not initialized") {
				t.Errorf("FetchRelatedTarget(%q) with no clients = (%d rows, %v), want no list and the client-missing error", td.ShortName, len(list), err)
			}
		})
	}
}
