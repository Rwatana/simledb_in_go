package plan

import (
	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// UpdatePlanner describes the contract for executing parsed SQL update commands.
type UpdatePlanner interface {
	ExecuteInsert(data parse.InsertData, transaction record.TableScanTransaction) (int, error)
	ExecuteDelete(data parse.DeleteData, transaction record.TableScanTransaction) (int, error)
	ExecuteModify(data parse.ModifyData, transaction record.TableScanTransaction) (int, error)
	ExecuteCreateTable(data parse.CreateTableData, transaction record.TableScanTransaction) (int, error)
	ExecuteCreateView(data parse.CreateViewData, transaction record.TableScanTransaction) (int, error)
	ExecuteCreateIndex(data parse.CreateIndexData, transaction record.TableScanTransaction) (int, error)
}
