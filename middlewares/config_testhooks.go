//go:build prest_test_hooks

package middlewares

// ResetForTest clears cached middleware app state between tests.
func ResetForTest() {
	resetAppState()
}
