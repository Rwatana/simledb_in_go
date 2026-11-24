package server

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/transaction"
)

// ExecuteUpdate parses a SQL update statement and dispatches it to the manager set's update planner.
func (m *Managers) ExecuteUpdate(statement string, transaction *transaction.Transaction) (int, error) {
	if m == nil || m.UpdatePlanner == nil {
		return 0, fmt.Errorf("execute server update: manager set is incomplete")
	}
	if transaction == nil {
		return 0, fmt.Errorf("execute server update: transaction is nil")
	}
	command, err := parse.NewParser(statement).ParseUpdateCommand()
	if err != nil {
		return 0, fmt.Errorf("execute server update: parse statement: %w", err)
	}
	rows, err := executeUpdateCommand(m.UpdatePlanner, command, transaction)
	if err != nil {
		return 0, fmt.Errorf("execute server update: %w", err)
	}
	return rows, nil
}

func executeUpdateCommand(planner plan.UpdatePlanner, command any, transaction *transaction.Transaction) (int, error) {
	switch command := command.(type) {
	case parse.InsertData:
		return planner.ExecuteInsert(command, transaction)
	case parse.DeleteData:
		return planner.ExecuteDelete(command, transaction)
	case parse.ModifyData:
		return planner.ExecuteModify(command, transaction)
	case parse.CreateTableData:
		return planner.ExecuteCreateTable(command, transaction)
	case parse.CreateViewData:
		return planner.ExecuteCreateView(command, transaction)
	case parse.CreateIndexData:
		return planner.ExecuteCreateIndex(command, transaction)
	default:
		return 0, fmt.Errorf("dispatch update command: unsupported command data %T", command)
	}
}
