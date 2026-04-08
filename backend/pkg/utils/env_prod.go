//go:build !dev

package utils

// isDevMode returns false for production builds (where 'dev' tag is absent).
func isDevMode() bool {
	return false
}
