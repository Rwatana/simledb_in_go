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

// BestFactor chooses the first ceiling quotient of size that fits the buffer budget.
// It reserves two buffers and returns one when the remaining budget or size is
// too small for a larger allocation.
func BestFactor(available, size int) int {
	if available <= 3 || size <= 1 {
		return 1
	}

	usable := available - 2
	low, high := 1, size
	for low < high {
		divisor := low + (high-low)/2
		if javaCeilingQuotient(size, divisor) <= usable {
			high = divisor
		} else {
			low = divisor + 1
		}
	}
	return javaCeilingQuotient(size, low)
}

func javaCeilingRoot(size, exponent int) int {
	root := math.Ceil(math.Pow(float64(size), 1/float64(exponent)))
	return saturatingInt(root)
}

func javaCeilingQuotient(size, divisor int) int {
	quotient := math.Ceil(float64(size) / float64(divisor))
	return saturatingInt(quotient)
}

func saturatingInt(value float64) int {
	maxInt := int(^uint(0) >> 1)
	if value >= float64(maxInt) {
		return maxInt
	}
	return int(value)
}
