package recovery

import "fmt"

// RestartRecordIterator yields decoded records newer than the first CHECKPOINT.
type RestartRecordIterator struct {
	source  EncodedRecordIterator
	stopped bool
}

// NewRestartRecordIterator creates a checkpoint-bounded restart iterator.
func NewRestartRecordIterator(source EncodedRecordIterator) *RestartRecordIterator {
	return &RestartRecordIterator{source: source}
}

// Next returns the next decoded record, excluding the CHECKPOINT boundary.
func (i *RestartRecordIterator) Next() (Record, bool, error) {
	if i.stopped {
		return nil, false, nil
	}
	encoded, ok, err := i.source.Next()
	if err != nil {
		return nil, false, fmt.Errorf("iterate restart recovery: next encoded record: %w", err)
	}
	if !ok {
		i.stopped = true
		return nil, false, nil
	}
	record, err := DecodeRecord(encoded)
	if err != nil {
		return nil, false, fmt.Errorf("iterate restart recovery: decode record: %w", err)
	}
	if record.Operation() == OperationCheckpoint {
		i.stopped = true
		return nil, false, nil
	}
	return record, true, nil
}
