//go:build prest_test_hooks

package middlewares

// ResetForTest is a no-op; middleware stacks are built per config via New().
func ResetForTest() {}
