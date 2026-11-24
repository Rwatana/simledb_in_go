// Package multibuffer contains helpers for choosing bounded buffer allocations.
package multibuffer

import "math"

// BestRoot chooses the first ceiling root of size that fits the buffer budget.
// It reserves two buffers and returns one when the remaining budget or size is
// too small for a larger root.
func BestRoot(available, size int) int {
	if available <= 3 || size <= 1 {
		return 1
	}

	usable := available - 2
	for exponent := 1; ; exponent++ {
		root := javaCeilingRoot(size, exponent)
		if root <= usable {
			return root
		}
	}
}

func javaCeilingRoot(size, exponent int) int {
	root := math.Ceil(math.Pow(float64(size), 1/float64(exponent)))
	maxInt := int(^uint(0) >> 1)
	if root >= float64(maxInt) {
		return maxInt
	}
	return int(root)
}
