package server

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/transaction"
)

func TestManagersExecuteUpdateDispatchesParsedCommands(t *testing.T) {
	tests := []struct {
		name        string
		statement   string
		method      string
		commandType string
	}{
		{name: "insert", statement: "insert into student (id) values (1)", method: "insert", commandType: "InsertData"},
		{name: "delete", statement: "delete from student where id=1", method: "delete", commandType: "DeleteData"},
		{name: "update", statement: "update student set id=2 where id=1", method: "modify", commandType: "ModifyData"},
		{name: "create table", statement: "create table course (id int)", method: "create table", commandType: "CreateTableData"},
		{name: "create view", statement: "create view course_ids as select id from course", method: "create view", commandType: "CreateViewData"},
		{name: "create index", statement: "create index course_id_idx on course (id)", method: "create index", commandType: "CreateIndexData"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			managers, transaction := newUpdateDispatchTransaction(t)
			probe := &updatePlannerProbe{rows: 7}
			managers.UpdatePlanner = probe

			rows, err := managers.ExecuteUpdate(test.statement, transaction)
			if err != nil || rows != probe.rows {
				t.Fatalf("ExecuteUpdate() = (%d, %v), want (%d, nil)", rows, err, probe.rows)
			}
			if probe.method != test.method {
				t.Errorf("update planner method = %q, want %q", probe.method, test.method)
			}
			if got := reflect.TypeOf(probe.command).Name(); got != test.commandType {
				t.Errorf("parsed command type = %q, want %q", got, test.commandType)
			}
			if probe.transaction != transaction {
				t.Error("ExecuteUpdate() did not pass the caller-owned transaction")
			}
			if _, err := transaction.Rollback(); err != nil {
				t.Fatalf("Rollback() after dispatched command = %v", err)
			}
		})
	}
}

func TestManagersExecuteUpdateReportsInvalidInputsAndFailures(t *testing.T) {
	managers, transaction := newUpdateDispatchTransaction(t)
	for name, managerSet := range map[string]*Managers{
		"nil":        nil,
		"incomplete": {},
	} {
		t.Run(name, func(t *testing.T) {
			if rows, err := managerSet.ExecuteUpdate("insert into student (id) values (1)", transaction); rows != 0 || err == nil || !strings.Contains(err.Error(), "manager") {
				t.Fatalf("ExecuteUpdate() = (%d, %v), want manager validation error", rows, err)
			}
		})
	}
	if rows, err := managers.ExecuteUpdate("insert into student (id) values (1)", nil); rows != 0 || err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("ExecuteUpdate() with nil transaction = (%d, %v), want transaction error", rows, err)
	}
	if rows, err := managers.ExecuteUpdate("insert", transaction); rows != 0 || err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("ExecuteUpdate() with malformed SQL = (%d, %v), want parse error", rows, err)
	}

	plannerErr := errors.New("planner failure")
	probe := &updatePlannerProbe{err: plannerErr}
	managers.UpdatePlanner = probe
	if rows, err := managers.ExecuteUpdate("insert into student (id) values (1)", transaction); rows != 0 || !errors.Is(err, plannerErr) || !strings.Contains(err.Error(), "execute server update") {
		t.Errorf("ExecuteUpdate() with planner failure = (%d, %v), want wrapped planner error", rows, err)
	}
	if _, err := transaction.Rollback(); err != nil {
		t.Fatalf("Rollback() after rejected update commands error = %v", err)
	}
}

func TestExecuteUpdateCommandRejectsUnsupportedData(t *testing.T) {
	if rows, err := executeUpdateCommand(&updatePlannerProbe{}, struct{}{}, nil); rows != 0 || err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("executeUpdateCommand() = (%d, %v), want unsupported-command error", rows, err)
	}
}

func newUpdateDispatchTransaction(t *testing.T) (*Managers, *transaction.Transaction) {
	t.Helper()
	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 128, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	return managers, transaction
}

type updatePlannerProbe struct {
	method      string
	command     any
	transaction record.TableScanTransaction
	rows        int
	err         error
}

func (probe *updatePlannerProbe) record(method string, command any, transaction record.TableScanTransaction) (int, error) {
	probe.method = method
	probe.command = command
	probe.transaction = transaction
	return probe.rows, probe.err
}

func (probe *updatePlannerProbe) ExecuteInsert(data parse.InsertData, transaction record.TableScanTransaction) (int, error) {
	return probe.record("insert", data, transaction)
}

func (probe *updatePlannerProbe) ExecuteDelete(data parse.DeleteData, transaction record.TableScanTransaction) (int, error) {
	return probe.record("delete", data, transaction)
}

func (probe *updatePlannerProbe) ExecuteModify(data parse.ModifyData, transaction record.TableScanTransaction) (int, error) {
	return probe.record("modify", data, transaction)
}

func (probe *updatePlannerProbe) ExecuteCreateTable(data parse.CreateTableData, transaction record.TableScanTransaction) (int, error) {
	return probe.record("create table", data, transaction)
}

func (probe *updatePlannerProbe) ExecuteCreateView(data parse.CreateViewData, transaction record.TableScanTransaction) (int, error) {
	return probe.record("create view", data, transaction)
}

func (probe *updatePlannerProbe) ExecuteCreateIndex(data parse.CreateIndexData, transaction record.TableScanTransaction) (int, error) {
	return probe.record("create index", data, transaction)
}
