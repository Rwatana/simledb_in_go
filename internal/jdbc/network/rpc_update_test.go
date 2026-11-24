package network

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestDriverServiceExecuteUpdateRunsWithinActiveTransactionOverRPC(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	initializer, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("catalog NewTransaction() error = %v", err)
	}
	if err := managers.InitializeCatalogs(initializer); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	if _, err := initializer.Commit(); err != nil {
		t.Fatalf("catalog transaction Commit() error = %v", err)
	}
	client := newTestRPCClient(t, managers)
	var connected ConnectResponse
	if err := client.Call("SimpleDB.Connect", ConnectRequest{}, &connected); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if connected.TransactionID != 2 {
		t.Fatalf("Connect() transaction ID = %d, want 2", connected.TransactionID)
	}

	var created ExecuteUpdateResponse
	if err := client.Call("SimpleDB.ExecuteUpdate", ExecuteUpdateRequest{Statement: "create table student (id int, name varchar(20))"}, &created); err != nil {
		t.Fatalf("ExecuteUpdate(create table) error = %v", err)
	}
	if created.RowsAffected != 0 {
		t.Errorf("create-table affected rows = %d, want 0", created.RowsAffected)
	}
	var inserted ExecuteUpdateResponse
	if err := client.Call("SimpleDB.ExecuteUpdate", ExecuteUpdateRequest{Statement: "insert into student (id, name) values (1, 'Ada')"}, &inserted); err != nil {
		t.Fatalf("ExecuteUpdate(insert) error = %v", err)
	}
	if inserted.RowsAffected != 1 {
		t.Errorf("insert affected rows = %d, want 1", inserted.RowsAffected)
	}

	iterator, err := managers.Logs.Iterator()
	if err != nil {
		t.Fatalf("Logs.Iterator() error = %v", err)
	}
	starts := make(map[recovery.TransactionID]int)
	for {
		encoded, ok, err := iterator.Next()
		if err != nil {
			t.Fatalf("Iterator.Next() error = %v", err)
		}
		if !ok {
			break
		}
		decoded, err := recovery.DecodeRecord(encoded)
		if err != nil {
			t.Fatalf("DecodeRecord() error = %v", err)
		}
		if record, ok := decoded.(interface {
			Operation() recovery.Operation
			TransactionID() recovery.TransactionID
		}); ok {
			if record.Operation() == recovery.OperationStart {
				starts[record.TransactionID()]++
				if record.TransactionID() > recovery.TransactionID(connected.TransactionID) {
					t.Errorf("update RPC started successor transaction %d before explicit completion", record.TransactionID())
				}
			}
			if (record.Operation() == recovery.OperationCommit || record.Operation() == recovery.OperationRollback) && record.TransactionID() == recovery.TransactionID(connected.TransactionID) {
				t.Fatalf("update RPC completed active transaction early with operation %d", record.Operation())
			}
		}
	}
	if starts[recovery.TransactionID(connected.TransactionID)] != 1 {
		t.Errorf("active transaction START records = %d, want exactly one for transaction %d", starts[recovery.TransactionID(connected.TransactionID)], connected.TransactionID)
	}

	var committed CommitResponse
	if err := client.Call("SimpleDB.Commit", CommitRequest{}, &committed); err != nil {
		t.Fatalf("Commit() after updates error = %v", err)
	}
	if committed.TransactionID != 3 {
		t.Errorf("Commit() next transaction ID = %d, want 3", committed.TransactionID)
	}
	if err := client.Call("SimpleDB.Close", CloseRequest{}, &CloseResponse{}); err != nil {
		t.Fatalf("Close() after commit error = %v", err)
	}
}

func TestDriverServiceExecuteUpdateRejectsInvalidStateAndClosedSession(t *testing.T) {
	var nilService *DriverService
	response := ExecuteUpdateResponse{RowsAffected: 72}
	if err := nilService.ExecuteUpdate(ExecuteUpdateRequest{}, &response); err == nil {
		t.Error("nil service ExecuteUpdate() succeeded, want error")
	}
	if response.RowsAffected != 0 {
		t.Errorf("nil-service affected rows = %d, want 0", response.RowsAffected)
	}
	if err := (&DriverService{}).ExecuteUpdate(ExecuteUpdateRequest{}, nil); err == nil {
		t.Error("ExecuteUpdate() with nil reply succeeded, want error")
	}
	response.RowsAffected = 72
	if err := (&DriverService{}).ExecuteUpdate(ExecuteUpdateRequest{}, &response); err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("ExecuteUpdate() without managers error = %v, want manager error", err)
	}
	if response.RowsAffected != 0 {
		t.Errorf("manager-less affected rows = %d, want 0", response.RowsAffected)
	}
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	response.RowsAffected = 72
	if err := (&DriverService{managers: managers}).ExecuteUpdate(ExecuteUpdateRequest{}, &response); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("ExecuteUpdate() without active transaction error = %v, want transaction error", err)
	}

	active, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	service := &DriverService{managers: managers, transaction: active}
	if err := service.Close(CloseRequest{}, &CloseResponse{}); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	response.RowsAffected = 72
	if err := service.ExecuteUpdate(ExecuteUpdateRequest{Statement: "create table student (id int)"}, &response); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("ExecuteUpdate() after Close() error = %v, want closed-session error", err)
	}
	if response.RowsAffected != 0 {
		t.Errorf("closed-session affected rows = %d, want 0", response.RowsAffected)
	}
}

func TestDriverServiceExecuteUpdatePreservesSessionAfterStatementAndPlannerErrors(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	active, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	service := &DriverService{managers: managers, transaction: active}
	for _, test := range []struct {
		name      string
		statement string
		planner   plan.UpdatePlanner
		wantError string
	}{
		{name: "parse", statement: "insert", wantError: "parse"},
		{name: "planner", statement: "create table student (id int)", planner: failingNetworkUpdatePlanner{err: errors.New("planner unavailable")}, wantError: "planner unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.planner != nil {
				managers.UpdatePlanner = test.planner
			}
			response := ExecuteUpdateResponse{RowsAffected: 91}
			err := service.ExecuteUpdate(ExecuteUpdateRequest{Statement: test.statement}, &response)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("ExecuteUpdate() error = %v, want %q", err, test.wantError)
			}
			if response.RowsAffected != 0 {
				t.Errorf("failed ExecuteUpdate() affected rows = %d, want 0", response.RowsAffected)
			}
		})
	}
	managers.UpdatePlanner = plan.NewBasicUpdatePlanner(managers.Metadata)
	if service.transaction != active || service.closed {
		t.Fatal("failed update replaced the active transaction or closed the session")
	}
	var committed CommitResponse
	if err := service.Commit(CommitRequest{}, &committed); err != nil {
		t.Fatalf("Commit() after rejected updates error = %v", err)
	}
	if committed.TransactionID != 2 {
		t.Errorf("Commit() next transaction ID = %d, want 2", committed.TransactionID)
	}
	if err := service.Close(CloseRequest{}, &CloseResponse{}); err != nil {
		t.Fatalf("Close() after commit error = %v", err)
	}
}

type failingNetworkUpdatePlanner struct{ err error }

func (planner failingNetworkUpdatePlanner) ExecuteInsert(parse.InsertData, record.TableScanTransaction) (int, error) {
	return 0, planner.err
}
func (planner failingNetworkUpdatePlanner) ExecuteDelete(parse.DeleteData, record.TableScanTransaction) (int, error) {
	return 0, planner.err
}
func (planner failingNetworkUpdatePlanner) ExecuteModify(parse.ModifyData, record.TableScanTransaction) (int, error) {
	return 0, planner.err
}
func (planner failingNetworkUpdatePlanner) ExecuteCreateTable(parse.CreateTableData, record.TableScanTransaction) (int, error) {
	return 0, planner.err
}
func (planner failingNetworkUpdatePlanner) ExecuteCreateView(parse.CreateViewData, record.TableScanTransaction) (int, error) {
	return 0, planner.err
}
func (planner failingNetworkUpdatePlanner) ExecuteCreateIndex(parse.CreateIndexData, record.TableScanTransaction) (int, error) {
	return 0, planner.err
}
