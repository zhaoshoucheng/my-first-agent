// Package util contains small helpers shared by the local langchaingo
// adapter packages, equivalent to a tiny subset of the upstream util package.
package util

// ToPtr returns a pointer to v.
func ToPtr[T any](v T) *T {
	return &v
}
