package materialize

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type sortScanRow struct {
	id    int32
	label string
}

type sortScanPosition struct {
	file   string
	offset int
}

type sortScanTransaction struct {
	*materializePlanTransaction
	sizeErrors  map[string]error
	readErrors  map[sortScanPosition]error
	pinErrors   map[string]error
	unpinErrors map[string]error
}

func newSortScanTransaction() *sortScanTransaction {
	return &sortScanTransaction{
		materializePlanTransaction: newMaterializePlanTransaction(),
		sizeErrors:                 make(map[string]error),
		readErrors:                 make(map[sortScanPosition]error),
		pinErrors:                  make(map[string]error),
		unpinErrors:                make(map[string]error),
	}
}

func (tx *sortScanTransaction) Size(fileName string) (int, error) {
	if err := tx.sizeErrors[fileName]; err != nil {
		tx.calls++
		return 0, err
	}
	return tx.materializePlanTransaction.Size(fileName)
}

func (tx *sortScanTransaction) Pin(block storage.BlockID) error {
	if err := tx.pinErrors[block.FileName()]; err != nil {
		tx.calls++
		tx.pins++
		return err
	}
	return tx.materializePlanTransaction.Pin(block)
}

func (tx *sortScanTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	value, err := tx.materializePlanTransaction.GetInt(block, offset)
	if err != nil {
		return 0, err
	}
	if injected := tx.readErrors[sortScanPosition{file: block.FileName(), offset: offset}]; injected != nil {
		return 0, injected
	}
	return value, nil
}

func (tx *sortScanTransaction) Unpin(block storage.BlockID) error {
	err := tx.materializePlanTransaction.Unpin(block)
	if err != nil {
		return err
	}
	return tx.unpinErrors[block.FileName()]
}

func newSortScanSchema() *record.Schema {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	schema.AddVarcharField("label", 8)
	return schema
}

func newSortScanTable(t *testing.T, tx *sortScanTransaction, schema *record.Schema, rows []sortScanRow) *TempTable {
	t.Helper()
	table, err := NewTempTable(tx, schema)
	if err != nil {
		t.Fatalf("NewTempTable() error = %v", err)
	}
	scan, err := table.Open()
	if err != nil {
		t.Fatalf("TempTable.Open() error = %v", err)
	}
	for _, row := range rows {
		if err := scan.Insert(); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		if err := scan.SetInt("id", row.id); err != nil {
			t.Fatalf("SetInt(id) error = %v", err)
		}
		if err := scan.SetString("label", row.label); err != nil {
			t.Fatalf("SetString(label) error = %v", err)
		}
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() while creating temporary table error = %v", err)
	}
	return table
}

func requireSortScan(t *testing.T, runs []*TempTable, comparator *RecordComparator) *SortScan {
	t.Helper()
	scan, err := NewSortScan(runs, comparator)
	if err != nil {
		t.Fatalf("NewSortScan() error = %v", err)
	}
	return scan
}

func collectSortScanRows(t *testing.T, scan *SortScan) []sortScanRow {
	t.Helper()
	var rows []sortScanRow
	for {
		ok, err := scan.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if !ok {
			return rows
		}
		id, err := scan.GetInt("id")
		if err != nil {
			t.Fatalf("GetInt(id) error = %v", err)
		}
		label, err := scan.GetString("label")
		if err != nil {
			t.Fatalf("GetString(label) error = %v", err)
		}
		rows = append(rows, sortScanRow{id: id, label: label})
	}
}

func TestNewSortScanMergesTwoSortedRunsAndRewinds(t *testing.T) {
	tx := newSortScanTransaction()
	schema := newSortScanSchema()
	first := newSortScanTable(t, tx, schema, []sortScanRow{{id: 1, label: "left"}, {id: 3, label: "left"}, {id: 5, label: "left"}})
	second := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "right"}, {id: 3, label: "right"}, {id: 4, label: "right"}})
	scan := requireSortScan(t, []*TempTable{first, second}, NewRecordComparator([]string{"id"}))

	if got, want := collectSortScanRows(t, scan), []sortScanRow{{1, "left"}, {2, "right"}, {3, "right"}, {3, "left"}, {4, "right"}, {5, "left"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged rows = %#v, want %#v", got, want)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if got, want := collectSortScanRows(t, scan), []sortScanRow{{1, "left"}, {2, "right"}, {3, "right"}, {3, "left"}, {4, "right"}, {5, "left"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rewound rows = %#v, want %#v", got, want)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestNewSortScanPassesThroughOneRunAndSupportsEmptyRuns(t *testing.T) {
	tx := newSortScanTransaction()
	schema := newSortScanSchema()
	comparator := NewRecordComparator([]string{"id"})
	single := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "two"}, {id: 4, label: "four"}})
	singleScan := requireSortScan(t, []*TempTable{single}, comparator)
	if got, want := collectSortScanRows(t, singleScan), []sortScanRow{{2, "two"}, {4, "four"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("single-run rows = %#v, want %#v", got, want)
	}
	if err := singleScan.Close(); err != nil {
		t.Fatalf("single-run Close() error = %v", err)
	}
	singleRewind := requireSortScan(t, []*TempTable{single}, comparator)
	if err := singleRewind.BeforeFirst(); err != nil {
		t.Fatalf("single-run BeforeFirst() error = %v", err)
	}
	if ok, err := singleRewind.Next(); err != nil || !ok {
		t.Fatalf("single-run rewound Next() = (%t, %v), want (true, nil)", ok, err)
	}
	if err := singleRewind.Close(); err != nil {
		t.Fatalf("single-run rewound Close() error = %v", err)
	}

	empty := newSortScanTable(t, tx, schema, nil)
	emptyScan := requireSortScan(t, []*TempTable{empty}, comparator)
	if ok, err := emptyScan.Next(); err != nil || ok {
		t.Fatalf("empty Next() = (%t, %v), want (false, nil)", ok, err)
	}
	if err := emptyScan.Close(); err != nil {
		t.Fatalf("empty Close() error = %v", err)
	}
}

func TestNewSortScanRejectsInvalidInputsBeforeOpeningRuns(t *testing.T) {
	tx := newSortScanTransaction()
	valid, err := NewTempTable(tx, newSortScanSchema())
	if err != nil {
		t.Fatal(err)
	}
	validAgain, err := NewTempTable(tx, newSortScanSchema())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		runs       []*TempTable
		comparator *RecordComparator
	}{
		{name: "no runs", runs: nil, comparator: NewRecordComparator([]string{"id"})},
		{name: "too many runs", runs: []*TempTable{valid, validAgain, valid}, comparator: NewRecordComparator([]string{"id"})},
		{name: "nil comparator", runs: []*TempTable{valid}, comparator: nil},
		{name: "nil run", runs: []*TempTable{nil}, comparator: NewRecordComparator([]string{"id"})},
		{name: "incomplete run", runs: []*TempTable{{}}, comparator: NewRecordComparator([]string{"id"})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := tx.calls
			if _, err := NewSortScan(test.runs, test.comparator); err == nil {
				t.Fatal("NewSortScan() error = nil, want invalid input error")
			}
			if tx.calls != before {
				t.Fatalf("transaction calls changed from %d to %d for invalid input", before, tx.calls)
			}
		})
	}
}

func TestNewSortScanPreservesOpenAndCleanupErrors(t *testing.T) {
	openErr := errors.New("second run size failed")
	closeErr := errors.New("first run close failed")
	tx := newSortScanTransaction()
	schema := newSortScanSchema()
	first := newSortScanTable(t, tx, schema, []sortScanRow{{id: 1, label: "left"}})
	second := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "right"}})
	tx.sizeErrors[second.TableName()+".tbl"] = openErr
	tx.unpinErrors[first.TableName()+".tbl"] = closeErr

	if _, err := NewSortScan([]*TempTable{first, second}, NewRecordComparator([]string{"id"})); !errors.Is(err, openErr) || !errors.Is(err, closeErr) {
		t.Fatalf("NewSortScan() error = %v, want open and cleanup causes", err)
	}
	if tx.unpins != 3 {
		t.Fatalf("unpins = %d, want two setup closes and failed-open cleanup", tx.unpins)
	}
}

func TestNewSortScanPreservesFirstRunOpenError(t *testing.T) {
	tx := newSortScanTransaction()
	table := newSortScanTable(t, tx, newSortScanSchema(), []sortScanRow{{id: 1, label: "one"}})
	cause := errors.New("first run size failed")
	tx.sizeErrors[table.TableName()+".tbl"] = cause
	if _, err := NewSortScan([]*TempTable{table}, NewRecordComparator([]string{"id"})); !errors.Is(err, cause) {
		t.Fatalf("NewSortScan() error = %v, want cause %v", err, cause)
	}
}

func TestNewSortScanClosesRunsAfterInitialPositioningErrors(t *testing.T) {
	readErr := errors.New("record flag read failed")
	cleanupErr := errors.New("scan cleanup failed")
	for _, failedRun := range []int{0, 1} {
		t.Run(fmt.Sprintf("run %d", failedRun+1), func(t *testing.T) {
			tx := newSortScanTransaction()
			schema := newSortScanSchema()
			first := newSortScanTable(t, tx, schema, []sortScanRow{{id: 1, label: "left"}})
			second := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "right"}})
			runs := []*TempTable{first, second}
			tx.readErrors[sortScanPosition{file: runs[failedRun].TableName() + ".tbl", offset: 0}] = readErr
			tx.unpinErrors[runs[0].TableName()+".tbl"] = cleanupErr

			if _, err := NewSortScan(runs, NewRecordComparator([]string{"id"})); !errors.Is(err, readErr) || !errors.Is(err, cleanupErr) {
				t.Fatalf("NewSortScan() error = %v, want positioning and cleanup causes", err)
			}
			if tx.unpins != 4 {
				t.Fatalf("unpins = %d, want two setup closes plus two-run cleanup", tx.unpins)
			}
		})
	}
}

func TestSortScanDelegatesCurrentValuesAndFieldChecks(t *testing.T) {
	tx := newSortScanTransaction()
	table := newSortScanTable(t, tx, newSortScanSchema(), []sortScanRow{{id: 7, label: "seven"}})
	scan := requireSortScan(t, []*TempTable{table}, NewRecordComparator([]string{"id"}))
	if scan.HasField("id") {
		t.Fatal("HasField(id) before Next() = true, want false")
	}
	if _, err := scan.GetVal("id"); err == nil {
		t.Fatal("GetVal() before Next() error = nil")
	}
	if _, err := scan.GetInt("id"); err == nil {
		t.Fatal("GetInt() before Next() error = nil")
	}
	if _, err := scan.GetString("label"); err == nil {
		t.Fatal("GetString() before Next() error = nil")
	}
	if ok, err := scan.Next(); err != nil || !ok {
		t.Fatalf("Next() = (%t, %v), want (true, nil)", ok, err)
	}
	if value, err := scan.GetVal("id"); err != nil || value != query.NewIntConstant(7) {
		t.Fatalf("GetVal(id) = (%v, %v), want (7, nil)", value, err)
	}
	if value, err := scan.GetInt("id"); err != nil || value != 7 {
		t.Fatalf("GetInt(id) = (%d, %v), want (7, nil)", value, err)
	}
	if value, err := scan.GetString("label"); err != nil || value != "seven" {
		t.Fatalf("GetString(label) = (%q, %v), want (seven, nil)", value, err)
	}
	if !scan.HasField("id") || !scan.HasField("label") || scan.HasField("missing") {
		t.Fatal("HasField() did not delegate to the selected run")
	}
	if _, err := scan.GetInt("missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("GetInt(missing) error = %v, want field context", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSortScanPreservesValueReadErrors(t *testing.T) {
	readErr := errors.New("stored value read failed")
	stringErr := errors.New("stored string read failed")
	for _, operation := range []string{"GetVal", "GetInt", "GetString"} {
		t.Run(operation, func(t *testing.T) {
			tx := newSortScanTransaction()
			table := newSortScanTable(t, tx, newSortScanSchema(), []sortScanRow{{id: 1, label: "one"}})
			scan := requireSortScan(t, []*TempTable{table}, NewRecordComparator([]string{"id"}))
			if ok, err := scan.Next(); err != nil || !ok {
				t.Fatalf("Next() = (%t, %v)", ok, err)
			}
			var err error
			switch operation {
			case "GetVal", "GetInt":
				tx.getIntErr = readErr
				if operation == "GetVal" {
					_, err = scan.GetVal("id")
				} else {
					_, err = scan.GetInt("id")
				}
			case "GetString":
				tx.getStringErr = stringErr
				_, err = scan.GetString("label")
			}
			want := readErr
			if operation == "GetString" {
				want = stringErr
			}
			if !errors.Is(err, want) {
				t.Fatalf("%s error = %v, want cause %v", operation, err, want)
			}
			if closeErr := scan.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		})
	}
}

func TestSortScanPreservesAdvanceAndComparisonErrors(t *testing.T) {
	t.Run("advance current run", func(t *testing.T) {
		tx := newSortScanTransaction()
		schema := newSortScanSchema()
		first := newSortScanTable(t, tx, schema, []sortScanRow{{id: 1, label: "left"}, {id: 3, label: "left"}})
		second := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "right"}})
		scan := requireSortScan(t, []*TempTable{first, second}, NewRecordComparator([]string{"id"}))
		if ok, err := scan.Next(); err != nil || !ok {
			t.Fatalf("first Next() = (%t, %v)", ok, err)
		}
		cause := errors.New("next record flag failed")
		tx.readErrors[sortScanPosition{file: first.TableName() + ".tbl", offset: first.Layout().SlotSize()}] = cause
		if _, err := scan.Next(); !errors.Is(err, cause) {
			t.Fatalf("Next() error = %v, want cause %v", err, cause)
		}
		_ = scan.Close()
	})

	t.Run("advance second run", func(t *testing.T) {
		tx := newSortScanTransaction()
		schema := newSortScanSchema()
		first := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "left"}})
		second := newSortScanTable(t, tx, schema, []sortScanRow{{id: 1, label: "right"}, {id: 3, label: "right"}})
		scan := requireSortScan(t, []*TempTable{first, second}, NewRecordComparator([]string{"id"}))
		if ok, err := scan.Next(); err != nil || !ok {
			t.Fatalf("first Next() = (%t, %v)", ok, err)
		}
		cause := errors.New("second run next flag failed")
		tx.readErrors[sortScanPosition{file: second.TableName() + ".tbl", offset: second.Layout().SlotSize()}] = cause
		if _, err := scan.Next(); !errors.Is(err, cause) {
			t.Fatalf("Next() error = %v, want cause %v", err, cause)
		}
		_ = scan.Close()
	})

	t.Run("compare run heads", func(t *testing.T) {
		tx := newSortScanTransaction()
		schema := newSortScanSchema()
		first := newSortScanTable(t, tx, schema, []sortScanRow{{id: 1, label: "left"}})
		second := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "right"}})
		scan := requireSortScan(t, []*TempTable{first, second}, NewRecordComparator([]string{"id"}))
		cause := errors.New("sort key read failed")
		offset, _ := first.Layout().Offset("id")
		tx.readErrors[sortScanPosition{file: first.TableName() + ".tbl", offset: offset}] = cause
		if _, err := scan.Next(); !errors.Is(err, cause) {
			t.Fatalf("Next() comparison error = %v, want cause %v", err, cause)
		}
		_ = scan.Close()
	})
}

func TestSortScanRewindsAndPreservesRunErrors(t *testing.T) {
	for failedRun := 0; failedRun < 2; failedRun++ {
		t.Run(fmt.Sprintf("run %d", failedRun+1), func(t *testing.T) {
			tx := newSortScanTransaction()
			schema := newSortScanSchema()
			first := newSortScanTable(t, tx, schema, []sortScanRow{{id: 1, label: "left"}})
			second := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "right"}})
			runs := []*TempTable{first, second}
			scan := requireSortScan(t, runs, NewRecordComparator([]string{"id"}))
			cause := errors.New("rewind run failed")
			tx.pinErrors[runs[failedRun].TableName()+".tbl"] = cause
			if err := scan.BeforeFirst(); !errors.Is(err, cause) {
				t.Fatalf("BeforeFirst() error = %v, want cause %v", err, cause)
			}
			if scan.HasField("id") {
				t.Fatal("HasField(id) after failed rewind = true, want false")
			}
			_ = scan.Close()
		})
	}

	t.Run("rewind advance", func(t *testing.T) {
		tx := newSortScanTransaction()
		table := newSortScanTable(t, tx, newSortScanSchema(), []sortScanRow{{id: 1, label: "one"}})
		scan := requireSortScan(t, []*TempTable{table}, NewRecordComparator([]string{"id"}))
		cause := errors.New("rewind next failed")
		tx.readErrors[sortScanPosition{file: table.TableName() + ".tbl", offset: 0}] = cause
		if err := scan.BeforeFirst(); !errors.Is(err, cause) {
			t.Fatalf("BeforeFirst() error = %v, want cause %v", err, cause)
		}
		_ = scan.Close()
	})
}

func TestSortScanSelectsSecondRunAfterFirstRunExhausts(t *testing.T) {
	tx := newSortScanTransaction()
	schema := newSortScanSchema()
	first := newSortScanTable(t, tx, schema, []sortScanRow{{id: 1, label: "left"}})
	second := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "right"}, {id: 3, label: "right"}})
	scan := requireSortScan(t, []*TempTable{first, second}, NewRecordComparator([]string{"id"}))
	if got, want := collectSortScanRows(t, scan), []sortScanRow{{1, "left"}, {2, "right"}, {3, "right"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged rows after first run exhaustion = %#v, want %#v", got, want)
	}
	if err := scan.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSortScanCloseJoinsErrorsAndIsIdempotent(t *testing.T) {
	tx := newSortScanTransaction()
	schema := newSortScanSchema()
	first := newSortScanTable(t, tx, schema, []sortScanRow{{id: 1, label: "left"}})
	second := newSortScanTable(t, tx, schema, []sortScanRow{{id: 2, label: "right"}})
	scan := requireSortScan(t, []*TempTable{first, second}, NewRecordComparator([]string{"id"}))
	firstErr := errors.New("first close failed")
	secondErr := errors.New("second close failed")
	tx.unpinErrors[first.TableName()+".tbl"] = firstErr
	tx.unpinErrors[second.TableName()+".tbl"] = secondErr

	if err := scan.Close(); !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("Close() error = %v, want both causes", err)
	}
	if tx.unpins != 4 {
		t.Fatalf("unpins = %d, want two setup closes plus two sort closes", tx.unpins)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("repeated Close() error = %v, want nil", err)
	}
	if tx.unpins != 4 {
		t.Fatalf("repeated Close() changed unpin count to %d", tx.unpins)
	}
	if ok, err := scan.Next(); ok || err == nil {
		t.Fatalf("Next() after Close() = (%t, %v), want (false, error)", ok, err)
	}
	if err := scan.BeforeFirst(); err == nil {
		t.Fatal("BeforeFirst() after Close() error = nil")
	}
	if _, err := scan.GetVal("id"); err == nil {
		t.Fatal("GetVal() after Close() error = nil")
	}
	if scan.HasField("id") {
		t.Fatal("HasField(id) after Close() = true, want false")
	}
}

func TestSortScanRejectsNilReceiverOperations(t *testing.T) {
	var scan *SortScan
	if err := scan.BeforeFirst(); err == nil {
		t.Fatal("nil BeforeFirst() error = nil")
	}
	if ok, err := scan.Next(); ok || err == nil {
		t.Fatalf("nil Next() = (%t, %v), want (false, error)", ok, err)
	}
	if _, err := scan.GetVal("id"); err == nil {
		t.Fatal("nil GetVal() error = nil")
	}
	if _, err := scan.GetInt("id"); err == nil {
		t.Fatal("nil GetInt() error = nil")
	}
	if _, err := scan.GetString("label"); err == nil {
		t.Fatal("nil GetString() error = nil")
	}
	if scan.HasField("id") {
		t.Fatal("nil HasField(id) = true")
	}
	if err := scan.Close(); err == nil {
		t.Fatal("nil Close() error = nil")
	}
}
