package server

import (
	"fmt"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

type restartRecoveryTransaction interface {
	Recover() (logmanager.LSN, error)
	Commit() (logmanager.LSN, error)
}

// Recover restores the durable database history through a fresh server transaction.
func (m *Managers) Recover() (logmanager.LSN, error) {
	recoveryTransaction, err := m.NewTransaction()
	if err != nil {
		return 0, fmt.Errorf("restart server database: create recovery transaction: %w", err)
	}
	return finishRestartRecovery(recoveryTransaction)
}

func finishRestartRecovery(transaction restartRecoveryTransaction) (logmanager.LSN, error) {
	checkpointLSN, err := transaction.Recover()
	if err != nil {
		return 0, fmt.Errorf("restart server database: recover log history: %w", err)
	}
	if _, err := transaction.Commit(); err != nil {
		return checkpointLSN, fmt.Errorf("restart server database: commit recovery transaction: %w", err)
	}
	return checkpointLSN, nil
}
