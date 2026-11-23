// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_user_issue_enrichment.go — Wave 2 issue enrichment for the iam-user resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

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
)

// EnrichIAMUserMFA calls GetLoginProfile + ListMFADevices + ListAccessKeys per user
// (capped at EnrichmentCap) to surface console users without MFA and stale access keys.
//
// Findings:
//   - GetLoginProfile succeeds AND ListMFADevices empty → "!" finding "console user without MFA"
//   - Any active access key with CreateDate >90d → "~" finding "key <id> >90d (rotation)"
func EnrichIAMUserMFA(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
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
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		userName := r.Fields["user_name"]
		if userName == "" {
			userName = r.ID
		}
		if userName == "" {
			return
		}

		// The login profile, the MFA devices, the access keys and the attached
		// policies are separate reads: one that fails leaves its own signals
		// unread and the others evaluated.
		var rowErrs []error
		consoleKnown, hasConsolePassword := true, false
		_, err := loginProfileAPI.GetLoginProfile(ctx, &iam.GetLoginProfileInput{
			UserName: aws.String(userName),
		})
		switch {
		case err == nil:
			hasConsolePassword = true
		case isNoSuchEntity(err):
			// NoSuchEntityException means the user has no console password.
		default:
			consoleKnown = false
			rowErrs = append(rowErrs, err)
		}

		// A user whose login profile could not be read may be a console user,
		// so its MFA devices are read too.
		hasMFA := false
		var mfaErr error
		if hasConsolePassword || !consoleKnown {
			// ListMFADevices pages by IsTruncated and Marker
			// (https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListMFADevices.html).
			var devices []iamtypes.MFADevice
			devices, _, mfaErr = PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.MFADevice, *string, error) {
				out, err := mfaAPI.ListMFADevices(ctx, &iam.ListMFADevicesInput{UserName: aws.String(userName), Marker: marker})
				if err != nil {
					return nil, nil, err
				}
				return out.MFADevices, iamNextMarker(out.IsTruncated, out.Marker), nil
			})
			if mfaErr != nil {
				rowErrs = append(rowErrs, mfaErr)
			}
			hasMFA = len(devices) > 0
		}

		// ListAccessKeys pages by IsTruncated and Marker
		// (https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListAccessKeys.html).
		keys, keysComplete, keysErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.AccessKeyMetadata, *string, error) {
			out, err := accessKeyAPI.ListAccessKeys(ctx, &iam.ListAccessKeysInput{UserName: aws.String(userName), Marker: marker})
			if err != nil {
				return nil, nil, err
			}
			return out.AccessKeyMetadata, iamNextMarker(out.IsTruncated, out.Marker), nil
		})
		if keysErr == nil && !keysComplete {
			keysErr = errPageCap
		}
		if keysErr != nil {
			keys = nil
			rowErrs = append(rowErrs, keysErr)
		}

		attachedUser, _, aerr := listAttachedUserPolicies(ctx, clients.IAM, userName)
		adminPolicy := adminAttachedPolicyName(attachedUser)

		activeKeys := activeAccessKeys(keys)
		unusedKeys, keyUseErr := unusedAccessKeys(ctx, keyLastUsedAPI, activeKeys)

		mu.Lock()
		defer mu.Unlock()

		for _, err := range append(rowErrs, aerr, keyUseErr) {
			if err != nil {
				MarkSkipped(&result, r.ID, &failures, err)
			}
		}
		// ListMFADevices is not called for a programmatic-only user, so MFA
		// was never looked at there — "true" would claim coverage nobody
		// checked. An unread read leaves the column empty.
		mfaVal := ""
		switch {
		case consoleKnown && !hasConsolePassword:
			mfaVal = "n/a"
		case mfaErr != nil:
		case hasMFA:
			mfaVal = "true"
		case consoleKnown:
			mfaVal = "false"
		}
		consolePasswordVal := "false"
		if hasConsolePassword {
			consolePasswordVal = "true"
		}
		riskLabel := ""

		if hasConsolePassword && mfaErr == nil && !hasMFA {
			riskLabel = riskNoMFA
			setWave2Finding(&result, r.ID, iamUserCodeNoMFA, []domain.DetailRow{{Label: "MFA device", Value: "none registered", Tier: tierOf(iamUserCodeNoMFA)}})

		}

		for _, f := range iamUserConsoleDormantFindings(consolePasswordVal, r.Fields["password_last_used"]) {
			if riskLabel == "" {
				riskLabel = riskConsoleDormant
			}
			addWave2Finding(&result, r.ID, f, []domain.DetailRow{{Label: "Last Sign-in", Value: r.Fields["password_last_used"], Tier: tierOf(f.Code)}})

		}

		if hasConsolePassword && r.Fields["password_last_used"] == "Never" &&
			olderThan(r.Fields["create_date"], days(unusedCredentialAfterDays)) {
			if riskLabel == "" {
				riskLabel = riskConsoleNeverUsed
			}
			setWave2Finding(&result, r.ID, iamUserCodeConsoleNeverUsed, []domain.DetailRow{{Label: "Created", Value: r.Fields["create_date"], Tier: tierOf(iamUserCodeConsoleNeverUsed)}})

		}

		for _, key := range activeKeys {
			if key.CreateDate == nil || time.Since(*key.CreateDate) <= days(unusedCredentialAfterDays) {
				continue
			}
			if riskLabel == "" {
				riskLabel = riskKeyTooOld
			}
			setWave2Finding(&result, r.ID, iamUserCodeOldKey, []domain.DetailRow{{Label: "Access key", Value: lastFourOfKeyID(aws.ToString(key.AccessKeyId)), Tier: tierOf(iamUserCodeOldKey)}})

		}

		for _, k := range unusedKeys {
			if riskLabel == "" {
				riskLabel = riskKeyUnused
			}
			setWave2Finding(&result, r.ID, iamUserCodeKeyUnused, []domain.DetailRow{
				{Label: "Key", Value: k.suffix, Tier: "~"},
				{Label: "Last used", Value: k.lastUsed, Tier: "~"},
				{Label: "Idle", Value: fmt.Sprintf("%d days", k.idleDays), Tier: "~"},
			})

		}

		if len(activeKeys) >= 2 {
			if riskLabel == "" {
				riskLabel = riskTwoActiveKeys
			}
			setWave2Finding(&result, r.ID, iamUserCodeTwoActiveKeys, []domain.DetailRow{{Label: "Keys", Value: "2 active", Tier: tierOf(iamUserCodeTwoActiveKeys)}})

		}

		if adminPolicy != "" {
			if riskLabel == "" {
				riskLabel = riskAdminPolicy
			}
			setWave2Finding(&result, r.ID, iamUserCodeAdminAttached, adminAttachedRows(adminPolicy))

		}

		fields := map[string]string{"risk": riskLabel}
		if consoleKnown {
			fields["has_console_password"] = consolePasswordVal //nolint:gosec // not a credential, display field key
		}
		if mfaVal != "" {
			fields["mfa"] = mfaVal
		}
		result.FieldUpdates[r.ID] = fields
	})
	return result, errors.Join(loopErr, AggregateFailures("user credentials", failures, n))
}

// isNoSuchEntity reports the IAM "this entity does not exist" error, which
// GetLoginProfile returns for a user with no console password — an answer,
// not a failure.
//
// Both spellings: IAM's modeled *NoSuchEntityException answers ErrorCode()
// "NoSuchEntity", while a response the SDK could not bind to it carries the
// exception name itself.
func isNoSuchEntity(err error) bool {
	return ErrCodeIs(err, "NoSuchEntity", "NoSuchEntityException")
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
// unusedCredentialAfterDays. ok is false when any key's last use could not be read:
// the user is then unknown for this check, not clean, since the unread key is
// exactly the one that might be idle. A client that does not serve the API
// reports no keys and stays ok — nothing was attempted, so nothing is unknown.
func unusedAccessKeys(ctx context.Context, api IAMGetAccessKeyLastUsedAPI, keys []iamtypes.AccessKeyMetadata) (out []idleKey, err error) {
	if api == nil {
		return nil, nil
	}
	for _, k := range keys {
		keyID := aws.ToString(k.AccessKeyId)
		if keyID == "" {
			continue
		}
		lastUsedOut, keyErr := api.GetAccessKeyLastUsed(ctx, &iam.GetAccessKeyLastUsedInput{AccessKeyId: aws.String(keyID)})
		if keyErr != nil {
			err = keyErr
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
		if idle <= days(unusedCredentialAfterDays) {
			continue
		}
		out = append(out, idleKey{
			suffix:   lastFourOfKeyID(keyID),
			lastUsed: label,
			idleDays: int(idle.Hours() / 24),
		})
	}
	return out, err
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
// nobody has used in unusedCredentialAfterDays. A password that was never used at
// all is the separate iamUserCodeConsoleNeverUsed finding.
func iamUserConsoleDormantFindings(hasConsolePassword, passwordLastUsed string) []domain.Finding {
	if hasConsolePassword != "true" || !olderThan(passwordLastUsed, days(unusedCredentialAfterDays)) {
		return nil
	}
	return []domain.Finding{wave2Finding(iamUserCodeConsoleDormant, strconv.Itoa(unusedCredentialAfterDays))}
}
