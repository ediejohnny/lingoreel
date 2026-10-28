package lrdev

import (
	"cmp"
	"slices"
)

// quantileOf is the value a share q (0 to 1) of the way through the values
// once sorted; the zero value of an empty list. The values are left as given.
func quantileOf[T cmp.Ordered](values []T, q float64) T {
	var zero T
	if len(values) == 0 {
		return zero
	}
	sorted := slices.Sorted(slices.Values(values))
	return sorted[min(max(int(q*float64(len(sorted)-1)+0.5), 0), len(sorted)-1)]
}
