package ctevent

import "strings"

// Actor computes the actor display string for the ACTOR section of the detail view,
// covering all 12 userIdentity variants from taxonomy §4.
//
// See specs/013-ct-event-detail-v2/contracts/ctdetail-api.md for the full contract,
// and specs/013-ct-event-detail-v2/data-model.md for the UserIdentity type definition.
//
// Guarantees:
//   - Never returns an empty string.
//   - Pure function: same input → same output, no I/O.
func Actor(event *Event) string {
	if event == nil {
		return "Unknown"
	}

	ui := event.UserIdentity

	// Empty identity — degenerate case.
	if ui.Type == "" && ui.ARN == "" && ui.UserName == "" && ui.AccountID == "" && ui.InvokedBy == "" {
		return "Unknown"
	}

	switch ui.Type {
	case "IAMUser":
		name := ui.UserName
		if name == "" {
			name = arnLastSegment(ui.ARN)
		}
		if name == "" {
			name = ui.ARN
		}
		return "IAMUser: " + name

	case "AssumedRole":
		return actorAssumedRole(ui)

	case "Root":
		return "Root"

	case "AWSService":
		svc := ui.InvokedBy
		if svc == "" {
			svc = ui.ARN
		}
		if svc == "" {
			svc = "AWSService"
		}
		return "AWSService: " + svc

	case "FederatedUser":
		name := ui.UserName
		if name == "" {
			name = ui.ARN
		}
		return "Federated: " + name

	case "SAMLUser":
		name := ui.UserName
		if name == "" {
			name = ui.ARN
		}
		return "SAML: " + name

	case "AWSAccount":
		id := ui.AccountID
		if id == "" {
			id = ui.PrincipalID
		}
		return "AWSAccount: " + id

	default:
		// Unknown / Directory / any other variant — return non-empty, no panic.
		if ui.Type != "" {
			return "Unknown: " + ui.Type
		}
		return "Unknown"
	}
}

// actorAssumedRole handles the AssumedRole type, which covers three sub-variants:
//  1. SSO / IdentityCenter: ARN role segment starts with "AWSReservedSSO_" AND sourceIdentity is set.
//  2. IRSA / WebIdentity: sessionContext has WebIDFederationData.
//  3. Plain AssumedRole: role name + session label from ARN.
func actorAssumedRole(ui UserIdentity) string {
	// Sub-variant: IRSA/WebIdentity — detect first (before SSO check).
	if ui.SessionContext != nil && ui.SessionContext.WebIDFederationData != nil {
		role := arnAssumedRoleSegment(ui.ARN)
		if role == "" {
			role = "WebIdentity"
		}
		return "WebIdentity: " + role
	}

	// Sub-variant: SSO / IdentityCenter.
	// Detect by: ARN role segment starts with "AWSReservedSSO_" AND sourceIdentity is non-empty.
	if ui.SessionContext != nil && ui.SessionContext.SourceIdentity != "" {
		role, session := arnAssumedRoleAndSession(ui.ARN)
		if strings.HasPrefix(role, "AWSReservedSSO_") {
			// Strip the trailing _<hash> suffix from the permission set name.
			permSet := stripSSOHash(role)
			human := ui.SessionContext.SourceIdentity
			_ = session
			return "SSO: " + human + " via " + permSet
		}
	}

	// Plain AssumedRole: extract role name and session label from ARN.
	role, session := arnAssumedRoleAndSession(ui.ARN)
	if role == "" {
		// Fall back to SessionIssuer.UserName if ARN parse fails.
		if ui.SessionContext != nil && ui.SessionContext.SessionIssuer != nil {
			role = ui.SessionContext.SessionIssuer.UserName
		}
	}
	if role == "" {
		return "AssumedRole"
	}
	if session == "" {
		return "AssumedRole: " + role
	}
	return "AssumedRole: " + role + "/" + session
}

// arnAssumedRoleSegment returns the role-name segment from an assumed-role ARN.
// e.g. "arn:aws:sts::666:assumed-role/eks-checkout-svc-sa/token" → "eks-checkout-svc-sa"
func arnAssumedRoleSegment(arn string) string {
	role, _ := arnAssumedRoleAndSession(arn)
	return role
}

// arnAssumedRoleAndSession parses an assumed-role ARN and returns (roleName, sessionLabel).
// ARN format: arn:aws:sts::<account>:assumed-role/<role-name>/<session-label>
func arnAssumedRoleAndSession(arn string) (role, session string) {
	// Find "assumed-role/" prefix within the ARN resource segment.
	const marker = "assumed-role/"
	_, rest, found := strings.Cut(arn, marker)
	if !found {
		return "", ""
	}
	parts := strings.SplitN(rest, "/", 2)
	role = parts[0]
	if len(parts) == 2 {
		session = parts[1]
	}
	return role, session
}

// arnLastSegment returns the last "/"-delimited segment of an ARN.
// e.g. "arn:aws:iam::333:user/bob" → "bob"
func arnLastSegment(arn string) string {
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx == len(arn)-1 {
		return ""
	}
	return arn[idx+1:]
}

// stripSSOHash removes the trailing "_<hex>" suffix from an AWSReservedSSO permission-set name.
// e.g. "AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d" → "AWSReservedSSO_AdminAccess"
func stripSSOHash(name string) string {
	// Find the last underscore — everything after it is the hash.
	idx := strings.LastIndex(name, "_")
	if idx < 0 {
		return name
	}
	candidate := name[idx+1:]
	// The hash is all hex digits and typically 16 chars long.
	if len(candidate) >= 8 && isHex(candidate) {
		return name[:idx]
	}
	return name
}

// isHex returns true if all characters in s are valid hexadecimal digits.
func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return len(s) > 0
}
