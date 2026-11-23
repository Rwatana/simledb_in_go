// Package hash provides static hash-index primitives.
package hash

import (
	"fmt"
	"unicode/utf16"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

// BucketCount is the fixed number of buckets used by the static hash index.
const BucketCount = 100

// BucketNumber maps a query value to its static hash-index bucket.
func BucketNumber(value query.Constant) (int, error) {
	if integer, ok := value.AsInt(); ok {
		return nonnegativeBucket(integer), nil
	}
	if text, ok := value.AsString(); ok {
		return nonnegativeBucket(javaStringHash(text)), nil
	}
	return 0, fmt.Errorf("select hash bucket: search key is unset")
}

func nonnegativeBucket(hashCode int32) int {
	bucket := int(hashCode % BucketCount)
	if bucket < 0 {
		bucket += BucketCount
	}
	return bucket
}

func javaStringHash(value string) int32 {
	var hash int32
	for _, codeUnit := range utf16.Encode([]rune(value)) {
		hash = 31*hash + int32(codeUnit)
	}
	return hash
}
