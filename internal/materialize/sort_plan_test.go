package materialize

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type sortPlanFailure struct {
	call int
	err  error
}

type sortPlanTransaction struct {
	*sortScanTransaction
	sizeCalls        map[string]int
	sizeFailureAt    map[string]sortPlanFailure
	appendCalls      map[string]int
	appendFailureAt  map[string]sortPlanFailure
	setIntCalls      map[sortScanPosition]int
	setIntFailureAt  map[sortScanPosition]sortPlanFailure
	setStringCalls   map[sortScanPosition]int
	setStringFailure map[sortScanPosition]sortPlanFailure
	unpinCalls       map[string]int
	unpinFailureAt   map[string]sortPlanFailure
	readStringErrors map[sortScanPosition]error
}

func newSortPlanTransaction() *sortPlanTransaction {
	return &sortPlanTransaction{
		sortScanTransaction: newSortScanTransaction(),
		sizeCalls:           make(map[string]int),
		sizeFailureAt:       make(map[string]sortPlanFailure),
		appendCalls:         make(map[string]int),
		appendFailureAt:     make(map[string]sortPlanFailure),
		setIntCalls:         make(map[sortScanPosition]int),
		setIntFailureAt:     make(map[sortScanPosition]sortPlanFailure),
		setStringCalls:      make(map[sortScanPosition]int),
		setStringFailure:    make(map[sortScanPosition]sortPlanFailure),
		unpinCalls:          make(map[string]int),
		unpinFailureAt:      make(map[string]sortPlanFailure),
		readStringErrors:    make(map[sortScanPosition]error),
	}
}

func (tx *sortPlanTransaction) Size(fileName string) (int, error) {
	tx.sizeCalls[fileName]++
	if failure, ok := tx.sizeFailureAt[fileName]; ok && tx.sizeCalls[fileName] == failure.call {
		tx.calls++
		return 0, failure.err
	}
	return tx.sortScanTransaction.Size(fileName)
}

func (tx *sortPlanTransaction) Append(fileName string) (storage.BlockID, error) {
	tx.appendCalls[fileName]++
	if failure, ok := tx.appendFailureAt[fileName]; ok && tx.appendCalls[fileName] == failure.call {
		tx.calls++
		return storage.BlockID{}, failure.err
	}
	return tx.sortScanTransaction.Append(fileName)
}

func (tx *sortPlanTransaction) SetInt(block storage.BlockID, offset int32, value int32, log bool) error {
	position := sortScanPosition{file: block.FileName(), offset: int(offset)}
	tx.setIntCalls[position]++
	if failure, ok := tx.setIntFailureAt[position]; ok && tx.setIntCalls[position] == failure.call {
		tx.calls++
		return failure.err
	}
	return tx.sortScanTransaction.SetInt(block, offset, value, log)
}

func (tx *sortPlanTransaction) SetString(block storage.BlockID, offset int32, value string, log bool) error {
	position := sortScanPosition{file: block.FileName(), offset: int(offset)}
	tx.setStringCalls[position]++
	if failure, ok := tx.setStringFailure[position]; ok && tx.setStringCalls[position] == failure.call {
		tx.calls++
		return failure.err
	}
	return tx.sortScanTransaction.SetString(block, offset, value, log)
}

func (tx *sortPlanTransaction) Unpin(block storage.BlockID) error {
	fileName := block.FileName()
	tx.unpinCalls[fileName]++
	if failure, ok := tx.unpinFailureAt[fileName]; ok && tx.unpinCalls[fileName] == failure.call {
		tx.calls++
		tx.unpins++
		return failure.err
	}
	return tx.sortScanTransaction.Unpin(block)
}

func (tx *sortPlanTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	value, err := tx.sortScanTransaction.GetString(block, offset)
	if err != nil {
		return "", err
	}
	if injected := tx.readStringErrors[sortScanPosition{file: block.FileName(), offset: offset}]; injected != nil {
		return "", injected
	}
	return value, nil
}

func sortPlanSource(rows []sortScanRow) *materializePlan {
	source := &materializePlan{schema: newSortScanSchema(), records: len(rows), blocks: 3, distinct: map[string]int{"id": len(rows)}}
	for _, row := range rows {
		source.rows = append(source.rows, []query.Constant{query.NewIntConstant(row.id), query.NewStringConstant(row.label)})
	}
	return source
}

func sortPlanRunNames(t *testing.T, tx *sortPlanTransaction, schema *record.Schema, count int) []string {
	t.Helper()
	marker, err := NewTempTable(tx, schema)
	if err != nil {
		t.Fatal(err)
	}
	markerNumber, err := strconv.Atoi(strings.TrimPrefix(marker.TableName(), "temp"))
	if err != nil {
		t.Fatalf("temporary table name %q: %v", marker.TableName(), err)
	}
	names := make([]string, count)
	for index := range names {
		names[index] = fmt.Sprintf("temp%d", markerNumber+index+1)
	}
	return names
}

func collectPlanRows(t *testing.T, scan query.Scan) []sortScanRow {
	t.Helper()
	var rows []sortScanRow
	for {
		hasRow, err := scan.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if !hasRow {
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

func TestSortPlanOpenSortsInputAcrossRunAndMergePasses(t *testing.T) {
	tx := newSortPlanTransaction()
	source := sortPlanSource([]sortScanRow{{8, "eight"}, {1, "one"}, {7, "seven"}, {2, "two"}, {6, "six"}, {3, "three"}, {5, "five"}, {4, "four"}})
	scan, err := NewSortPlan(tx, source, []string{"id"})
	if err != nil {
		t.Fatalf("NewSortPlan() error = %v", err)
	}
	if got := scan.Schema(); got != source.schema {
		t.Fatalf("Schema() = %p, want source schema %p", got, source.schema)
	}
	if got := scan.RecordsOutput(); got != len(source.rows) {
		t.Fatalf("RecordsOutput() = %d, want %d", got, len(source.rows))
	}
	if got := scan.DistinctValues("id"); got != len(source.rows) {
		t.Fatalf("DistinctValues(id) = %d", got)
	}
	if got := scan.BlocksAccessed(); got != 2 {
		t.Fatalf("BlocksAccessed() = %d, want materialized estimate 2", got)
	}
	finalScan := mustOpenSortPlan(t, scan)
	if source.openCalls != 1 || source.scan == nil || source.scan.closeCalls != 1 {
		t.Fatalf("source opens/closes = %d/%d, want 1/1", source.openCalls, source.scan.closeCalls)
	}
	if got, want := collectPlanRows(t, finalScan), []sortScanRow{{1, "one"}, {2, "two"}, {3, "three"}, {4, "four"}, {5, "five"}, {6, "six"}, {7, "seven"}, {8, "eight"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted rows = %#v, want %#v", got, want)
	}
	if err := finalScan.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
}

func mustOpenSortPlan(t *testing.T, plan *SortPlan) query.Scan {
	t.Helper()
	scan, err := plan.Open()
	if err != nil {
		t.Fatalf("SortPlan.Open() error = %v", err)
	}
	return scan
}

func TestSortPlanOpenSupportsSingleRunAndEmptySource(t *testing.T) {
	for _, test := range []struct {
		name string
		rows []sortScanRow
	}{
		{name: "single run", rows: []sortScanRow{{1, "one"}, {2, "two"}, {3, "three"}}},
		{name: "empty source", rows: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx := newSortPlanTransaction()
			source := sortPlanSource(test.rows)
			sortPlan, err := NewSortPlan(tx, source, []string{"id"})
			if err != nil {
				t.Fatal(err)
			}
			scan := mustOpenSortPlan(t, sortPlan)
			if got := collectPlanRows(t, scan); !reflect.DeepEqual(got, test.rows) {
				t.Fatalf("rows = %#v, want %#v", got, test.rows)
			}
			if err := scan.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})
	}
}

func TestSortPlanOpenKeepsTieOrderWithinAndAcrossRuns(t *testing.T) {
	tx := newSortPlanTransaction()
	source := sortPlanSource([]sortScanRow{{2, "first"}, {1, "middle"}, {2, "last"}})
	plan, err := NewSortPlan(tx, source, []string{"id"})
	if err != nil {
		t.Fatal(err)
	}
	scan := mustOpenSortPlan(t, plan)
	want := []sortScanRow{{1, "middle"}, {2, "last"}, {2, "first"}}
	if got := collectPlanRows(t, scan); !reflect.DeepEqual(got, want) {
		t.Fatalf("tie rows = %#v, want Java-compatible order %#v", got, want)
	}
	if err := scan.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSortPlanOpenMergesOddRunCount(t *testing.T) {
	tx := newSortPlanTransaction()
	rows := []sortScanRow{{4, "four"}, {2, "two"}, {3, "three"}, {1, "one"}, {5, "five"}}
	plan, err := NewSortPlan(tx, sortPlanSource(rows), []string{"id"})
	if err != nil {
		t.Fatal(err)
	}
	scan := mustOpenSortPlan(t, plan)
	want := []sortScanRow{{1, "one"}, {2, "two"}, {3, "three"}, {4, "four"}, {5, "five"}}
	if got := collectPlanRows(t, scan); !reflect.DeepEqual(got, want) {
		t.Fatalf("odd-run merge = %#v, want %#v", got, want)
	}
	if err := scan.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewSortPlanRejectsInvalidDependenciesWithoutTransactionCalls(t *testing.T) {
	tx := newSortPlanTransaction()
	valid := sortPlanSource(nil)
	invalidSchema := &materializePlan{}
	for _, test := range []struct {
		name   string
		tx     *sortPlanTransaction
		source *materializePlan
		fields []string
	}{
		{name: "nil transaction", source: valid, fields: []string{"id"}},
		{name: "nil source", tx: tx, fields: []string{"id"}},
		{name: "nil schema", tx: tx, source: invalidSchema, fields: []string{"id"}},
		{name: "unknown sort field", tx: tx, source: valid, fields: []string{"missing"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := tx.calls
			if _, err := NewSortPlan(test.tx, test.source, test.fields); err == nil {
				t.Fatal("NewSortPlan() error = nil, want invalid dependency error")
			}
			if tx.calls != before {
				t.Fatalf("transaction calls changed from %d to %d", before, tx.calls)
			}
		})
	}
}

func TestSortPlanRejectsIncompletePlansAndDetectsTypedNilDependencies(t *testing.T) {
	tx := newSortPlanTransaction()
	source := sortPlanSource(nil)
	valid := &SortPlan{transaction: tx, source: source, schema: source.schema, comparator: NewRecordComparator(nil)}
	tests := []struct {
		name string
		plan *SortPlan
	}{
		{name: "nil transaction", plan: &SortPlan{source: source, schema: source.schema, comparator: valid.comparator}},
		{name: "typed nil transaction", plan: &SortPlan{transaction: (*sortPlanTransaction)(nil), source: source, schema: source.schema, comparator: valid.comparator}},
		{name: "nil source", plan: &SortPlan{transaction: tx, schema: source.schema, comparator: valid.comparator}},
		{name: "typed nil source", plan: &SortPlan{transaction: tx, source: (*materializePlan)(nil), schema: source.schema, comparator: valid.comparator}},
		{name: "nil schema", plan: &SortPlan{transaction: tx, source: source, comparator: valid.comparator}},
		{name: "nil comparator", plan: &SortPlan{transaction: tx, source: source, schema: source.schema}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.plan.Open(); err == nil {
				t.Fatal("Open() error = nil, want incomplete plan error")
			}
			if source.openCalls != 0 {
				t.Fatalf("source opened %d times before dependency validation", source.openCalls)
			}
		})
	}

	value := 1
	for _, test := range []struct {
		name  string
		value any
		want  bool
	}{
		{name: "nil", value: nil, want: true},
		{name: "typed nil pointer", value: (*int)(nil), want: true},
		{name: "non-nil pointer", value: &value},
		{name: "non-pointer value", value: value},
	} {
		t.Run("dependency "+test.name, func(t *testing.T) {
			if got := isNilSortPlanDependency(test.value); got != test.want {
				t.Fatalf("isNilSortPlanDependency(%#v) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}

func TestSortPlanMethodsHandleNilReceiverAndDelegateStatistics(t *testing.T) {
	var nilPlan *SortPlan
	if _, err := nilPlan.Open(); err == nil {
		t.Fatal("nil Open() error = nil")
	}
	if nilPlan.Schema() != nil || nilPlan.BlocksAccessed() != 0 || nilPlan.RecordsOutput() != 0 || nilPlan.DistinctValues("id") != 0 {
		t.Fatal("nil plan returned non-zero metadata")
	}
	tx := newSortPlanTransaction()
	source := sortPlanSource(nil)
	sortPlan, err := NewSortPlan(tx, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := sortPlan.DistinctValues("id"); got != 0 || !reflect.DeepEqual(source.distinctCalls, []string{"id"}) {
		t.Fatalf("DistinctValues(id) = %d, calls = %v", got, source.distinctCalls)
	}
	if (&SortPlan{}).BlocksAccessed() != 0 || (&SortPlan{}).RecordsOutput() != 0 || (&SortPlan{}).DistinctValues("id") != 0 || (&SortPlan{}).Schema() != nil {
		t.Fatal("zero-value plan returned metadata without dependencies")
	}
	if (&SortPlan{source: (*materializePlan)(nil)}).RecordsOutput() != 0 || (&SortPlan{source: (*materializePlan)(nil)}).DistinctValues("id") != 0 {
		t.Fatal("typed nil source returned metadata")
	}
}

func TestSortPlanOpenPreservesSourceAndDestinationErrors(t *testing.T) {
	openErr := errors.New("source plan open failed")
	partialCloseErr := errors.New("partial source scan close failed")
	readErr := errors.New("source next failed")
	valueErr := errors.New("source value failed")
	closeErr := errors.New("source close failed")
	appendErr := errors.New("temporary append failed")
	for _, test := range []struct {
		name      string
		source    *materializePlan
		configure func(*sortPlanTransaction)
		cause     error
	}{
		{name: "source open", source: &materializePlan{schema: newSortScanSchema(), openErr: openErr}, cause: openErr},
		{name: "source open with partial scan", source: &materializePlan{schema: newSortScanSchema(), openErr: openErr, openScanOnError: true, closeErr: partialCloseErr}, cause: openErr},
		{name: "nil source scan", source: &materializePlan{schema: newSortScanSchema(), nilScan: true}},
		{name: "source iteration", source: &materializePlan{schema: newSortScanSchema(), rows: [][]query.Constant{{query.NewIntConstant(1), query.NewStringConstant("one")}}, nextErr: readErr}, cause: readErr},
		{name: "source rewind", source: &materializePlan{schema: newSortScanSchema(), rows: [][]query.Constant{{query.NewIntConstant(1), query.NewStringConstant("one")}}, beforeFirstErr: readErr}, cause: readErr},
		{name: "source value", source: &materializePlan{schema: newSortScanSchema(), rows: [][]query.Constant{{query.NewIntConstant(1), query.NewStringConstant("one")}}, getValErr: valueErr}, cause: valueErr},
		{name: "source close", source: &materializePlan{schema: newSortScanSchema(), rows: [][]query.Constant{{query.NewIntConstant(1), query.NewStringConstant("one")}}, closeErr: closeErr}, cause: closeErr},
		{name: "temporary append", source: sortPlanSource([]sortScanRow{{1, "one"}}), configure: func(tx *sortPlanTransaction) { tx.appendErr = appendErr }, cause: appendErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx := newSortPlanTransaction()
			if test.configure != nil {
				test.configure(tx)
			}
			sortPlan, err := NewSortPlan(tx, test.source, []string{"id"})
			if err != nil {
				t.Fatal(err)
			}
			_, openErr := sortPlan.Open()
			if openErr == nil {
				t.Fatal("Open() error = nil, want failure")
			} else if test.cause != nil && !errors.Is(openErr, test.cause) {
				t.Fatalf("Open() error = %v, want cause %v", openErr, test.cause)
			}
			if test.name == "source open with partial scan" && !errors.Is(openErr, partialCloseErr) {
				t.Fatalf("Open() error = %v, want partial scan cleanup cause %v", openErr, partialCloseErr)
			}
			if test.source.scan != nil && test.source.scan.closeCalls != 1 {
				t.Errorf("source close calls = %d, want 1", test.source.scan.closeCalls)
			}
		})
	}
}

func TestSortPlanOpenPreservesRunWriteAndCleanupFailures(t *testing.T) {
	t.Run("run insert", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runs := sortPlanRunNames(t, tx, newSortScanSchema(), 1)
		cause := errors.New("run slot insert failed")
		tx.setIntFailureAt[sortScanPosition{file: runs[0] + ".tbl", offset: 0}] = sortPlanFailure{call: 2, err: cause}
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
	})

	t.Run("run integer write", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runs := sortPlanRunNames(t, tx, newSortScanSchema(), 1)
		cause := errors.New("run integer write failed")
		tx.setIntFailureAt[sortScanPosition{file: runs[0] + ".tbl", offset: 4}] = sortPlanFailure{call: 2, err: cause}
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
	})

	t.Run("run string write", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runs := sortPlanRunNames(t, tx, newSortScanSchema(), 1)
		cause := errors.New("run string write failed")
		tx.setStringFailure[sortScanPosition{file: runs[0] + ".tbl", offset: 8}] = sortPlanFailure{call: 2, err: cause}
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
	})

	t.Run("close run after descent", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runs := sortPlanRunNames(t, tx, newSortScanSchema(), 2)
		cause := errors.New("close sorted run failed")
		tx.unpinErrors[runs[0]+".tbl"] = cause
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{2, "two"}, {1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
	})

	t.Run("close run at end", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runs := sortPlanRunNames(t, tx, newSortScanSchema(), 1)
		cause := errors.New("close final sorted run failed")
		tx.unpinErrors[runs[0]+".tbl"] = cause
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
	})

	t.Run("cleanup joins a split failure", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runs := sortPlanRunNames(t, tx, newSortScanSchema(), 1)
		readErr := errors.New("source field read failed")
		closeErr := errors.New("failed run cleanup")
		tx.unpinErrors[runs[0]+".tbl"] = closeErr
		source := sortPlanSource([]sortScanRow{{1, "one"}})
		source.getValErr = readErr
		plan, err := NewSortPlan(tx, source, []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
			t.Fatalf("Open() error = %v, want read and cleanup errors", err)
		}
	})

	t.Run("source advances after copied row", func(t *testing.T) {
		tx := newSortPlanTransaction()
		cause := errors.New("source advance failed")
		source := sortPlanSource([]sortScanRow{{1, "one"}, {2, "two"}})
		source.nextErr = cause
		source.nextErrAt = 2
		plan, err := NewSortPlan(tx, source, []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
	})
}

func TestSortPlanOpenPreservesSplitAndMergeFailures(t *testing.T) {
	t.Run("split comparator read", func(t *testing.T) {
		tx := newSortPlanTransaction()
		schema := newSortScanSchema()
		runNames := sortPlanRunNames(t, tx, schema, 4)
		cause := errors.New("run record read failed")
		fieldOffset, _ := record.NewLayout(schema).Offset("id")
		tx.readErrors[sortScanPosition{file: runNames[0] + ".tbl", offset: fieldOffset}] = cause
		source := sortPlanSource([]sortScanRow{{5, "five"}, {1, "one"}})
		sortPlan, err := NewSortPlan(tx, source, []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sortPlan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
		if source.scan == nil || source.scan.closeCalls != 1 {
			t.Fatalf("source close calls = %v, want 1", source.scan)
		}
	})

	t.Run("merge second input open", func(t *testing.T) {
		tx := newSortPlanTransaction()
		schema := newSortScanSchema()
		runNames := sortPlanRunNames(t, tx, schema, 4)
		cause := errors.New("second run open failed during merge")
		tx.sizeFailureAt[runNames[1]+".tbl"] = sortPlanFailure{call: 2, err: cause}
		source := sortPlanSource([]sortScanRow{{5, "five"}, {1, "one"}, {4, "four"}, {2, "two"}})
		sortPlan, err := NewSortPlan(tx, source, []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sortPlan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
		if source.scan == nil || source.scan.closeCalls != 1 {
			t.Fatalf("source close calls = %v, want 1", source.scan)
		}
	})
}

func TestSortPlanOpenPreservesEmptyRunAndMergePassFailures(t *testing.T) {
	t.Run("empty final run open", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runNames := sortPlanRunNames(t, tx, newSortScanSchema(), 1)
		cause := errors.New("empty output run open failed")
		tx.sizeFailureAt[runNames[0]+".tbl"] = sortPlanFailure{call: 1, err: cause}
		plan, err := NewSortPlan(tx, sortPlanSource(nil), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
	})

	t.Run("final sorted scan open", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runNames := sortPlanRunNames(t, tx, newSortScanSchema(), 2)
		cause := errors.New("final sorted run open failed")
		tx.sizeFailureAt[runNames[0]+".tbl"] = sortPlanFailure{call: 2, err: cause}
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{2, "two"}, {1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, cause) {
			t.Fatalf("Open() error = %v, want cause %v", err, cause)
		}
	})

	t.Run("merge destination open closes inputs", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runNames := sortPlanRunNames(t, tx, newSortScanSchema(), 4)
		openErr := errors.New("merge destination open failed")
		closeErr := errors.New("merge input close failed")
		tx.sizeFailureAt[runNames[3]+".tbl"] = sortPlanFailure{call: 1, err: openErr}
		tx.unpinFailureAt[runNames[0]+".tbl"] = sortPlanFailure{call: 2, err: closeErr}
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{5, "five"}, {3, "three"}, {4, "four"}, {1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, openErr) || !errors.Is(err, closeErr) {
			t.Fatalf("Open() error = %v, want open and cleanup errors", err)
		}
	})

	t.Run("merge source advance closes scans", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runNames := sortPlanRunNames(t, tx, newSortScanSchema(), 4)
		readErr := errors.New("merge source advance failed")
		closeErr := errors.New("merge input cleanup failed")
		tx.readErrors[sortScanPosition{file: runNames[0] + ".tbl", offset: record.NewLayout(newSortScanSchema()).SlotSize()}] = readErr
		tx.unpinFailureAt[runNames[0]+".tbl"] = sortPlanFailure{call: 2, err: closeErr}
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{3, "three"}, {2, "two"}, {1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
			t.Fatalf("Open() error = %v, want read and cleanup errors", err)
		}
	})

	t.Run("merge copy reads every schema field", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runNames := sortPlanRunNames(t, tx, newSortScanSchema(), 4)
		fieldOffset, _ := record.NewLayout(newSortScanSchema()).Offset("label")
		readErr := errors.New("merge string field read failed")
		tx.readStringErrors[sortScanPosition{file: runNames[0] + ".tbl", offset: fieldOffset}] = readErr
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{3, "three"}, {2, "two"}, {1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, readErr) {
			t.Fatalf("Open() error = %v, want cause %v", err, readErr)
		}
	})

	t.Run("merge destination write", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runNames := sortPlanRunNames(t, tx, newSortScanSchema(), 4)
		writeErr := errors.New("merged destination integer write failed")
		tx.setIntFailureAt[sortScanPosition{file: runNames[3] + ".tbl", offset: 4}] = sortPlanFailure{call: 2, err: writeErr}
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{3, "three"}, {2, "two"}, {1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, writeErr) {
			t.Fatalf("Open() error = %v, want cause %v", err, writeErr)
		}
	})

	t.Run("merge closes source and destination", func(t *testing.T) {
		tx := newSortPlanTransaction()
		runNames := sortPlanRunNames(t, tx, newSortScanSchema(), 4)
		inputClose := errors.New("merge input close failed")
		destinationClose := errors.New("merge destination close failed")
		tx.unpinFailureAt[runNames[0]+".tbl"] = sortPlanFailure{call: 2, err: inputClose}
		tx.unpinErrors[runNames[3]+".tbl"] = destinationClose
		plan, err := NewSortPlan(tx, sortPlanSource([]sortScanRow{{3, "three"}, {2, "two"}, {1, "one"}}), []string{"id"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Open(); !errors.Is(err, inputClose) || !errors.Is(err, destinationClose) {
			t.Fatalf("Open() error = %v, want both close errors", err)
		}
	})
}
