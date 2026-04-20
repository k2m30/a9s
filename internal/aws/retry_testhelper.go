// Package aws — testing helper for retry configuration.
package aws

// SetRetryConfigForTest replaces the RetryConfig returned by DefaultRetryConfig
// and returns a function that restores the previous value. Intended for tests
// that exercise retry paths — without this override, a single retry costs the
// full BaseDelay (500ms) which materially slows the suite.
//
// Typical usage:
//
//	restore := aws.SetRetryConfigForTest(&aws.RetryConfig{
//	    MaxAttempts: 3,
//	    BaseDelay:   1 * time.Millisecond,
//	    MaxDelay:    10 * time.Millisecond,
//	    Jitter:      false,
//	})
//	defer restore()
func SetRetryConfigForTest(cfg *RetryConfig) func() {
	prev := retryConfigOverrideForTest
	retryConfigOverrideForTest = cfg
	return func() { retryConfigOverrideForTest = prev }
}
