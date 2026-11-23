package recovery

import (
	"fmt"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

// AppendCheckpointRecord appends one encoded quiescent checkpoint record.
func AppendCheckpointRecord(appender LogAppender) (logmanager.LSN, error) {
	lsn, err := appender.Append(EncodeCheckpointRecord())
	if err != nil {
		return 0, fmt.Errorf("append checkpoint record: %w", err)
	}
	return lsn, nil
}
