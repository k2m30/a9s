// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
	iamUserCodeNoMFA            domain.FindingCode = "iam-user.no-mfa"
	iamUserCodeOldKey           domain.FindingCode = "iam-user.old-key"
	iamUserCodeAdminAttached    domain.FindingCode = "iam-user.admin-attached"
	iamUserCodeConsoleNeverUsed domain.FindingCode = "iam-user.console-never-used"
	iamUserCodeKeyUnused        domain.FindingCode = "iam-user.access-key-unused"
	iamUserCodeTwoActiveKeys    domain.FindingCode = "iam-user.two-active-keys"
	iamUserCodeConsoleDormant   domain.FindingCode = "iam-user.console-dormant"

	iamUserNoMFADetail = "This user signs in to the console with a password alone, so a leaked or guessed " +
		"password is a full takeover. Register an MFA device for the user, or remove the console password if " +
		"the user only needs programmatic access."
	iamUserOldKeyDetail = "This access key has been valid for more than 90 days, so a copy taken at any point " +
		"since it was created still works. Create a replacement key, move callers onto it, then deactivate and " +
		"delete the old one."
	iamUserConsoleDormantDetail   = "Nobody has signed in to this console login for over 90 days. Confirm the person still needs it and delete the login profile if they do not."
	iamUserConsoleNeverUsedDetail = "This user has a console password that has never been used since the account " +
		"was created, so it is an unguarded sign-in path nobody is watching. Delete the login profile and leave " +
		"the user with programmatic access only."
	iamUserKeyUnusedDetail = "This access key is active but has not signed a request in over 90 days, so it is a " +
		"live credential with no owner watching it. Deactivate the key, confirm nothing breaks, then delete it."
	iamUserTwoActiveKeysDetail = "This user has both of its access-key slots active at once, which doubles the " +
		"exposure and means a rotation cannot be completed. Deactivate and delete the key that is no longer in use."

	// unusedCredentialAge is the age past which an untouched credential is
	// reported. Matches the 90-day threshold used for key rotation.
	unusedCredentialAge = 90 * 24 * time.Hour
)

// EnrichIAMUserMFA calls GetLoginProfile + ListMFADevices + ListAccessKeys per user
// (capped at EnrichmentCap) to surface console users without MFA and stale access keys.
//
// Findings:
//   - GetLoginProfile succeeds AND ListMFADevices empty → "!" finding "console user without MFA"
//   - Any active access key with CreateDate >90d → "~" finding "key <id> >90d (rotation)"
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

	keyLastUsedAPI, _ := clients.IAM.(IAMGetAccessKeyLastUsedAPI)
	truncated := len(resources) > EnrichmentCap
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
			if !isNoSuchEntity(err) {
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

		hasMFA := false
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
		}

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

		attachedUser, aerr := listAttachedUserPolicies(ctx, clients.IAM, userName)
		adminPolicy := adminAttachedPolicyName(attachedUser)
		if aerr != nil {
			mu.Lock()
			result.TruncatedIDs[r.ID] = true
			mu.Unlock()
		}

		activeKeys := activeAccessKeys(keysOut.AccessKeyMetadata)
		unusedKeys, keyUseKnown := unusedAccessKeys(ctx, keyLastUsedAPI, activeKeys)

		mu.Lock()
		defer mu.Unlock()

		if !keyUseKnown {
			result.TruncatedIDs[r.ID] = true
		}

		mfaVal := "false"
		if hasMFA || !hasConsolePassword {
			mfaVal = "true"
		}
		consolePasswordVal := "false"
		if hasConsolePassword {
			consolePasswordVal = "true"
		}
		riskLabel := ""

		if hasConsolePassword && !hasMFA {
			riskLabel = riskNoMFA
			setWave2Finding(&result, r.ID, iamUserCodeNoMFA, "console user without MFA", "!", "iam-user",
				[]domain.DetailRow{{Label: "MFA device", Value: "none registered", Tier: "!"}}, iamUserNoMFADetail)
		}

		for _, f := range iamUserConsoleDormantFindings(consolePasswordVal, r.Fields["password_last_used"]) {
			if riskLabel == "" {
				riskLabel = riskConsoleDormant
			}
			setWave2Finding(&result, r.ID, f.Code, f.Phrase, "~", "iam-user",
				[]domain.DetailRow{{Label: "Last Sign-in", Value: r.Fields["password_last_used"], Tier: "~"}}, f.Detail)
		}

		if hasConsolePassword && r.Fields["password_last_used"] == "Never" &&
			olderThan(r.Fields["create_date"], unusedCredentialAge) {
			if riskLabel == "" {
				riskLabel = riskConsoleNeverUsed
			}
			setWave2Finding(&result, r.ID, iamUserCodeConsoleNeverUsed, "console password never used", "~", "iam-user",
				[]domain.DetailRow{{Label: "Created", Value: r.Fields["create_date"], Tier: "~"}}, iamUserConsoleNeverUsedDetail)
		}

		for _, key := range activeKeys {
			if key.CreateDate == nil || time.Since(*key.CreateDate) <= unusedCredentialAge {
				continue
			}
			if riskLabel == "" {
				riskLabel = riskKeyTooOld
			}
			phrase := fmt.Sprintf("key %s >90d (rotation)", lastFourOfKeyID(aws.ToString(key.AccessKeyId)))
			setWave2Finding(&result, r.ID, iamUserCodeOldKey, phrase, "~", "iam-user",
				[]domain.DetailRow{{Label: "Access key", Value: lastFourOfKeyID(aws.ToString(key.AccessKeyId)), Tier: "~"}}, iamUserOldKeyDetail)
			break
		}

		if len(unusedKeys) > 0 {
			k := unusedKeys[0]
			if riskLabel == "" {
				riskLabel = riskKeyUnused
			}
			setWave2Finding(&result, r.ID, iamUserCodeKeyUnused,
				fmt.Sprintf("access key unused for %d days", k.idleDays), "~", "iam-user",
				[]domain.DetailRow{
					{Label: "Key", Value: k.suffix, Tier: "~"},
					{Label: "Last used", Value: k.lastUsed, Tier: "~"},
				}, iamUserKeyUnusedDetail)
		}

		if len(activeKeys) >= 2 {
			if riskLabel == "" {
				riskLabel = riskTwoActiveKeys
			}
			setWave2Finding(&result, r.ID, iamUserCodeTwoActiveKeys, "two active access keys", "~", "iam-user",
				[]domain.DetailRow{{Label: "Keys", Value: "2 active", Tier: "~"}}, iamUserTwoActiveKeysDetail)
		}

		if adminPolicy != "" {
			if riskLabel == "" {
				riskLabel = riskAdminPolicy
			}
			setWave2Finding(&result, r.ID, iamUserCodeAdminAttached, adminAttachedPhrase, "~", "iam-user",
				adminAttachedRows(adminPolicy), adminAttachedDetail)
		}

		result.FieldUpdates[r.ID] = map[string]string{
			"mfa":                  mfaVal,
			"risk":                 riskLabel,
			"has_console_password": consolePasswordVal, //nolint:gosec // not a credential, display field key
		}
	})
	result.Truncated = truncated
	return result, nil
}

// isNoSuchEntity reports the IAM "this entity does not exist" error, which
// GetLoginProfile returns for a user with no console password — an answer,
// not a failure.
func isNoSuchEntity(err error) bool {
	var noSuchEntity *iamtypes.NoSuchEntityException
	var apiErr smithy.APIError
	return errors.As(err, &noSuchEntity) ||
		(errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchEntityException")
}

// olderThan reports whether a "2006-01-02 15:04" field value parses and is
// further in the past than age. An unparseable or absent date is never old:
// unknown is not misconfigured.
func olderThan(formatted string, age time.Duration) bool {
	t, err := time.Parse("2006-01-02 15:04", formatted)
	return err == nil && time.Since(t) > age
}

func activeAccessKeys(keys []iamtypes.AccessKeyMetadata) []iamtypes.AccessKeyMetadata {
	var out []iamtypes.AccessKeyMetadata
	for _, k := range keys {
		if k.Status == iamtypes.StatusTypeActive {
			out = append(out, k)
		}
	}
	return out
}

// idleKey describes one active access key that has gone unused long enough
// to report. suffix is the last four characters of the key ID only — the
// full ID is a credential identifier and never leaves this package.
type idleKey struct {
	suffix   string
	lastUsed string
	idleDays int
}

// unusedAccessKeys calls GetAccessKeyLastUsed per active key and returns the
// keys whose last use (or, absent any use, whose creation) is older than
// unusedCredentialAge. ok is false when any key's last use could not be read:
// the user is then unknown for this check, not clean, since the unread key is
// exactly the one that might be idle. A client that does not serve the API
// reports no keys and stays ok — nothing was attempted, so nothing is unknown.
func unusedAccessKeys(ctx context.Context, api IAMGetAccessKeyLastUsedAPI, keys []iamtypes.AccessKeyMetadata) (out []idleKey, ok bool) {
	if api == nil {
		return nil, true
	}
	ok = true
	for _, k := range keys {
		keyID := aws.ToString(k.AccessKeyId)
		if keyID == "" {
			continue
		}
		lastUsedOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.GetAccessKeyLastUsedOutput, error) {
			return api.GetAccessKeyLastUsed(ctx, &iam.GetAccessKeyLastUsedInput{AccessKeyId: aws.String(keyID)})
		})
		if err != nil {
			ok = false
			continue
		}
		var lastUsedAt *time.Time
		if lastUsedOut.AccessKeyLastUsed != nil {
			lastUsedAt = lastUsedOut.AccessKeyLastUsed.LastUsedDate
		}
		since, label := lastUsedAt, "never"
		if lastUsedAt != nil {
			label = lastUsedAt.Format("2006-01-02")
		} else {
			// Never used: the key has been idle since it was created.
			since = k.CreateDate
		}
		if since == nil {
			continue
		}
		idle := time.Since(*since)
		if idle <= unusedCredentialAge {
			continue
		}
		out = append(out, idleKey{
			suffix:   lastFourOfKeyID(keyID),
			lastUsed: label,
			idleDays: int(idle.Hours() / 24),
		})
	}
	return out, ok
}

// lastFourOfKeyID renders an access key ID as its last four characters only.
// Every surface that names a key goes through here: a key ID is a credential
// identifier and has no business in a list cell, a detail row or a log.
func lastFourOfKeyID(keyID string) string {
	if len(keyID) <= 4 {
		return keyID
	}
	return "…" + keyID[len(keyID)-4:]
}

// iamUserConsoleDormantFindings is the one predicate for a console login
// nobody has used in unusedCredentialAge. colorIAMUser runs it over Fields for
// rows built outside the enricher. A password that was never used at all is a
// different finding (iamUserCodeConsoleNeverUsed) and is not reported here.
func iamUserConsoleDormantFindings(hasConsolePassword, passwordLastUsed string) []domain.Finding {
	if hasConsolePassword != "true" || !olderThan(passwordLastUsed, unusedCredentialAge) {
		return nil
	}
	return []domain.Finding{{
		Code: iamUserCodeConsoleDormant, Phrase: "console sign-in unused for 90 days",
		Detail:   iamUserConsoleDormantDetail,
		Severity: domain.SevWarn, Source: "wave2:iam-user",
	}}
}
