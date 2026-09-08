// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import "github.com/aws/aws-sdk-go-v2/aws"

// A fake stands in for a whole account. A key its fixtures never registered is
// therefore a key the account does not have, and the only truthful answer to a
// by-id lookup of one is the service's not-found error.
//
// Answering an empty result instead turns a fixture gap into a confident zero:
// a pivot to a resource nobody modelled renders "none", which is the exact
// class of lie demo mode exists to disprove, and the reason the IAM fake grew
// this rule first.
//
// Two constraints on the refusal. The code is the SERVICE's own spelling, not
// a uniform one — AWS spells this differently per API, and a fake answering a
// code its own SDK never emits would be a shape production has no reason to
// handle. And every one of those codes is a code aws.IsNotFoundErr classifies,
// because a refusal production cannot classify is a refusal no caller can act
// on.
//
// The refusal keys on the fixtures' own REGISTRY of the thing being asked
// about — the API list, not the per-id detail map. A registered resource with
// no detail entry still answers empty, because "this API has no stages" and
// "there is no such API" are different facts.

// notFoundMessage is the wording every refusal below shares, in the shape AWS
// writes it.
func notFoundMessage(kind, id string) *string {
	return aws.String(kind + " " + id + " cannot be found.")
}
