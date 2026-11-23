package transaction

import (
	"sync/atomic"

	"github.com/Rwatana/simledb_in_go/internal/recovery"
)

// IDAllocator assigns monotonically increasing transaction identifiers.
// Its zero value begins with identifier one.
type IDAllocator struct {
	last atomic.Int32
}

// Next returns the next positive recovery transaction identifier.
func (a *IDAllocator) Next() recovery.TransactionID {
	return recovery.TransactionID(a.last.Add(1))
}
