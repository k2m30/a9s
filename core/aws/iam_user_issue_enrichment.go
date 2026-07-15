// iam_user_issue_enrichment.go — Wave 2 issue enrichment for the iam-user resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	smithy "github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// iam-user canonical FindingCodes.
const (
	iamUserCodeNoMFA  domain.FindingCode = "iam-user.no-mfa"
	iamUserCodeOldKey domain.FindingCode = "iam-user.old-key"
)

// EnrichIAMUserMFA calls GetLoginProfile + ListMFADevices + ListAccessKeys per user
// (capped at EnrichmentCap) to surface console users without MFA and stale access keys.
//
// Findings:
//   - GetLoginProfile succeeds AND ListMFADevices empty → "!" finding "console user without MFA"
//   - Any active access key with CreateDate >90d → "~" finding "access key >90d (rotation)"
//
// Skip when clients.IAM == nil.
func EnrichIAMUserMFA(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.IAM == nil {
		return result, nil
	}
	loginProfileAPI, ok1 := clients.IAM.(IAMGetLoginProfileAPI)
	mfaAPI, ok2 := clients.IAM.(IAMListMFADevicesAPI)
	accessKeyAPI, ok3 := clients.IAM.(IAMListAccessKeysAPI)
	if !ok1 || !ok2 || !ok3 {
		return result, nil
	}

	truncated := len(resources) > EnrichmentCap
	issueCount := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		userName := r.Fields["user_name"]
		if userName == "" {
			userName = r.ID
		}
		if userName == "" {
			return
		}

		// Determine if the user has a console password via GetLoginProfile.
		hasConsolePassword := false
		_, err := loginProfileAPI.GetLoginProfile(ctx, &iam.GetLoginProfileInput{
			UserName: aws.String(userName),
		})
		if err != nil {
			var noSuchEntity *iamtypes.NoSuchEntityException
			var apiErr smithy.APIError
			isNoSuchEntity := errors.As(err, &noSuchEntity) ||
				(errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchEntityException")
			if !isNoSuchEntity {
				// Unexpected error — skip this user but flag truncation.
				mu.Lock()
				truncated = true
				result.TruncatedIDs[r.ID] = true
				mu.Unlock()
				return
			}
			// NoSuchEntityException means the user has no console password.
		} else {
			hasConsolePassword = true
		}

		var rows []domain.DetailRow
		severity := "~"
		hasMFA := false
		riskLabel := ""
		localIssue := false

		// Check MFA only for console users.
		if hasConsolePassword {
			mfaOut, mfaErr := mfaAPI.ListMFADevices(ctx, &iam.ListMFADevicesInput{
				UserName: aws.String(userName),
			})
			if mfaErr != nil {
				mu.Lock()
				truncated = true
				result.TruncatedIDs[r.ID] = true
				mu.Unlock()
				return
			}
			hasMFA = len(mfaOut.MFADevices) > 0
			if !hasMFA {
				rows = append(rows, domain.DetailRow{
					Label: "MFA",
					Value: "console user without MFA",
					Tier:  "!",
				})
				severity = "!"
				localIssue = true
				riskLabel = "NO_MFA"
			}
		}

		// Check access key age regardless of console password presence.
		keysOut, keysErr := accessKeyAPI.ListAccessKeys(ctx, &iam.ListAccessKeysInput{
			UserName: aws.String(userName),
		})
		if keysErr != nil {
			mu.Lock()
			truncated = true
			result.TruncatedIDs[r.ID] = true
			mu.Unlock()
			return
		}
		hasOldKey := false
		for _, key := range keysOut.AccessKeyMetadata {
			if key.Status != iamtypes.StatusTypeActive {
				continue
			}
			if key.CreateDate == nil {
				continue
			}
			if time.Since(*key.CreateDate) > 90*24*time.Hour {
				hasOldKey = true
				keyID := ""
				if key.AccessKeyId != nil {
					keyID = *key.AccessKeyId
				}
				rows = append(rows, domain.DetailRow{
					Label: "Access Key",
					Value: fmt.Sprintf("key %s >90d (rotation)", keyID),
					Tier:  "~",
				})
				if riskLabel == "" {
					riskLabel = "OLD_KEY"
				}
			}
		}
		_ = hasOldKey

		// Write field updates for mfa and risk columns.
		mfaVal := "false"
		if hasMFA || !hasConsolePassword {
			mfaVal = "true"
		}

		mu.Lock()
		defer mu.Unlock()
		if localIssue {
			issueCount++
		}
		consolePasswordVal := "false"
		if hasConsolePassword {
			consolePasswordVal = "true"
		}

		result.FieldUpdates[r.ID] = map[string]string{
			"mfa":                  mfaVal,
			"risk":                 riskLabel,
			"has_console_password": consolePasswordVal, //nolint:gosec // not a credential, display field key
		}

		if len(rows) == 0 {
			return
		}
		// Use the no-mfa code when severity is "!", otherwise old-key code.
		code := iamUserCodeOldKey
		if severity == "!" {
			code = iamUserCodeNoMFA
		}
		setWave2Finding(&result, r.ID, code, rows[0].Value, severity, "iam-user", rows, "")
	})
	result.IssueCount = issueCount
	result.Truncated = truncated
	return result, nil
}
