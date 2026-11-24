package multibuffer

import (
	"math"
	"testing"
)

func TestBestRoot(t *testing.T) {
	cases := []struct {
		name      string
		available int
		size      int
		want      int
	}{
		{name: "reserve two buffers", available: 4, size: 100, want: 2},
		{name: "at most one usable buffer", available: 3, size: 100, want: 1},
		{name: "fewer than reserved buffers", available: 1, size: 100, want: 1},
		{name: "exact square root", available: 10, size: 64, want: 8},
		{name: "non-perfect root ceiling", available: 5, size: 65, want: 3},
		{name: "single block", available: 12, size: 1, want: 1},
		{name: "empty size", available: 12, size: 0, want: 1},
		{name: "negative size", available: 12, size: -1, want: 1},
		{name: "maximum integer size", available: 4, size: math.MaxInt, want: 2},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := BestRoot(test.available, test.size); got != test.want {
				t.Fatalf("BestRoot(%d, %d) = %d, want %d", test.available, test.size, got, test.want)
			}
		})
	}
}

func TestBestRootMatchesJavaExponentSearch(t *testing.T) {
	for available := -1; available <= 20; available++ {
		for size := 1; size <= 512; size++ {
			if got, want := BestRoot(available, size), javaBestRoot(available, size); got != want {
				t.Fatalf("BestRoot(%d, %d) = %d, Java exponent search returns %d", available, size, got, want)
			}
		}
	}
}

func javaBestRoot(available, size int) int {
	usable := available - 2
	if usable <= 1 {
		return 1
	}
	for exponent := 1; ; exponent++ {
		root := int(math.Ceil(math.Pow(float64(size), 1/float64(exponent))))
		if root <= usable {
			return root
		}
	}
}

func TestBestFactor(t *testing.T) {
	cases := []struct {
		name      string
		available int
		size      int
		want      int
	}{
		{name: "reserve two buffers", available: 4, size: 100, want: 2},
		{name: "one usable buffer", available: 3, size: 100, want: 1},
		{name: "fewer than reserved buffers", available: 1, size: 100, want: 1},
		{name: "exact quotient", available: 10, size: 30, want: 8},
		{name: "ceiling quotient", available: 5, size: 65, want: 3},
		{name: "single block", available: 12, size: 1, want: 1},
		{name: "empty size", available: 12, size: 0, want: 1},
		{name: "negative size", available: 12, size: -1, want: 1},
		{name: "maximum integer size", available: 4, size: math.MaxInt, want: 2},
		{name: "maximum budget and size", available: math.MaxInt, size: math.MaxInt, want: int(math.Ceil(float64(math.MaxInt) / 2))},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := BestFactor(test.available, test.size); got != test.want {
				t.Fatalf("BestFactor(%d, %d) = %d, want %d", test.available, test.size, got, test.want)
			}
		})
	}
}

func TestBestFactorMatchesJavaDivisorSearch(t *testing.T) {
	for available := -1; available <= 20; available++ {
		for size := 1; size <= 512; size++ {
			if got, want := BestFactor(available, size), javaBestFactor(available, size); got != want {
				t.Fatalf("BestFactor(%d, %d) = %d, Java divisor search returns %d", available, size, got, want)
			}
		}
	}
}

func javaBestFactor(available, size int) int {
	usable := available - 2
	if usable <= 1 || size <= 1 {
		return 1
	}
	for divisor := 1; ; divisor++ {
		quotient := int(math.Ceil(float64(size) / float64(divisor)))
		if quotient <= usable {
			return quotient
		}
	}
}
