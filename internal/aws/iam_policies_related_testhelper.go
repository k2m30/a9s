// Package aws — testing helper for IAM policy related-checkers.
// SetIAMListEntitiesAPIForTest installs a mock IAMListEntitiesForPolicyAPI and
// returns a restore function. Tests call the restore function (typically via defer)
// to reset the package-level variable after the test completes.
package aws

// SetIAMListEntitiesAPIForTest replaces the package-level IAM API used by the
// policy related-checkers with the supplied mock, and returns a function that
// restores the previous value. Typical usage:
//
//	restore := aws.SetIAMListEntitiesAPIForTest(myMock)
//	defer restore()
func SetIAMListEntitiesAPIForTest(api IAMListEntitiesForPolicyAPI) func() {
	prev := iamListEntitiesAPIForTest
	iamListEntitiesAPIForTest = api
	return func() { iamListEntitiesAPIForTest = prev }
}
