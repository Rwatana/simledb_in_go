package materialize

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type materializePlan struct {
	schema        *record.Schema
	rows          [][]query.Constant
	records       int
	blocks        int
	distinct      map[string]int
	openErr       error
	nilScan       bool
	nextErr       error
	getValErr     error
	closeErr      error
	scan          *materializeSourceScan
	openCalls     int
	distinctCalls []string
}

func (p *materializePlan) Open() (query.Scan, error) {
	p.openCalls++
	if p.openErr != nil {
		return nil, p.openErr
	}
	if p.nilScan {
		return nil, nil
	}
	p.scan = &materializeSourceScan{
		schema: p.schema, rows: p.rows, position: -1,
		nextErr: p.nextErr, getValErr: p.getValErr, closeErr: p.closeErr,
	}
	return p.scan, nil
}

func (p *materializePlan) BlocksAccessed() int { return p.blocks }

func (p *materializePlan) RecordsOutput() int { return p.records }

func (p *materializePlan) DistinctValues(fieldName string) int {
	p.distinctCalls = append(p.distinctCalls, fieldName)
	return p.distinct[fieldName]
}

func (p *materializePlan) Schema() *record.Schema { return p.schema }

type materializeSourceScan struct {
	schema         *record.Schema
	rows           [][]query.Constant
	position       int
	nextErr        error
	getValErr      error
	beforeFirstErr error
	closeErr       error
	nextCalls      int
	closeCalls     int
}

func (s *materializeSourceScan) BeforeFirst() error {
	if s.beforeFirstErr != nil {
		return s.beforeFirstErr
	}
	s.position = -1
	return nil
}

func (s *materializeSourceScan) Next() (bool, error) {
	s.nextCalls++
	if s.nextErr != nil {
		return false, s.nextErr
	}
	if s.position+1 >= len(s.rows) {
		return false, nil
	}
	s.position++
	return true, nil
}

func (s *materializeSourceScan) GetInt(fieldName string) (int32, error) {
	value, err := s.GetVal(fieldName)
	if err != nil {
		return 0, err
	}
	integer, ok := value.AsInt()
	if !ok {
		return 0, fmt.Errorf("field %q is not an integer", fieldName)
	}
	return integer, nil
}

func (s *materializeSourceScan) GetString(fieldName string) (string, error) {
	value, err := s.GetVal(fieldName)
	if err != nil {
		return "", err
	}
	text, ok := value.AsString()
	if !ok {
		return "", fmt.Errorf("field %q is not a string", fieldName)
	}
	return text, nil
}

func (s *materializeSourceScan) GetVal(fieldName string) (query.Constant, error) {
	if s.getValErr != nil {
		return query.Constant{}, s.getValErr
	}
	if s.position < 0 || s.position >= len(s.rows) {
		return query.Constant{}, fmt.Errorf("no current source row")
	}
	for index, name := range s.schema.Fields() {
		if name == fieldName {
			return s.rows[s.position][index], nil
		}
	}
	return query.Constant{}, fmt.Errorf("unknown field %q", fieldName)
}

func (s *materializeSourceScan) HasField(fieldName string) bool {
	return s.schema.HasField(fieldName)
}

func (s *materializeSourceScan) Close() error {
	s.closeCalls++
	return s.closeErr
}

type materializeRecordPosition struct {
	block  storage.BlockID
	offset int
}

type materializePlanTransaction struct {
	blockSize        int
	blockSizeErr     error
	sizeErr          error
	appendErr        error
	pinErr           error
	unpinErr         error
	getIntErr        error
	getStringErr     error
	setIntErr        error
	setStringErr     error
	setIntFailErr    error
	setStringFailErr error
	setIntFailAt     int
	setStringFailAt  int
	setIntCalls      int
	setStringCalls   int
	calls            int
	pins             int
	unpins           int
	fileSizes        map[string]int
	integers         map[materializeRecordPosition]int32
	strings          map[materializeRecordPosition]string
}

func newMaterializePlanTransaction() *materializePlanTransaction {
	return &materializePlanTransaction{
		blockSize: 256,
		fileSizes: make(map[string]int),
		integers:  make(map[materializeRecordPosition]int32),
		strings:   make(map[materializeRecordPosition]string),
	}
}

func (tx *materializePlanTransaction) Pin(storage.BlockID) error {
	tx.calls++
	tx.pins++
	return tx.pinErr
}

func (tx *materializePlanTransaction) BlockSize() (int, error) {
	tx.calls++
	return tx.blockSize, tx.blockSizeErr
}

func (tx *materializePlanTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	tx.calls++
	return tx.integers[materializeRecordPosition{block: block, offset: offset}], tx.getIntErr
}

func (tx *materializePlanTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	tx.calls++
	return tx.strings[materializeRecordPosition{block: block, offset: offset}], tx.getStringErr
}

func (tx *materializePlanTransaction) SetInt(block storage.BlockID, offset int32, value int32, _ bool) error {
	tx.calls++
	tx.setIntCalls++
	if tx.setIntErr != nil || tx.setIntFailAt == tx.setIntCalls {
		if tx.setIntErr != nil {
			return tx.setIntErr
		}
		if tx.setIntFailErr != nil {
			return tx.setIntFailErr
		}
		return errors.New("injected integer write failure")
	}
	if tx.integers == nil {
		tx.integers = make(map[materializeRecordPosition]int32)
	}
	tx.integers[materializeRecordPosition{block: block, offset: int(offset)}] = value
	return nil
}

func (tx *materializePlanTransaction) SetString(block storage.BlockID, offset int32, value string, _ bool) error {
	tx.calls++
	tx.setStringCalls++
	if tx.setStringErr != nil || tx.setStringFailAt == tx.setStringCalls {
		if tx.setStringErr != nil {
			return tx.setStringErr
		}
		if tx.setStringFailErr != nil {
			return tx.setStringFailErr
		}
		return errors.New("injected string write failure")
	}
	if tx.strings == nil {
		tx.strings = make(map[materializeRecordPosition]string)
	}
	tx.strings[materializeRecordPosition{block: block, offset: int(offset)}] = value
	return nil
}

func (tx *materializePlanTransaction) Size(fileName string) (int, error) {
	tx.calls++
	return tx.fileSizes[fileName], tx.sizeErr
}

func (tx *materializePlanTransaction) Append(fileName string) (storage.BlockID, error) {
	tx.calls++
	if tx.appendErr != nil {
		return storage.BlockID{}, tx.appendErr
	}
	if tx.fileSizes == nil {
		tx.fileSizes = make(map[string]int)
	}
	blockNumber := tx.fileSizes[fileName]
	tx.fileSizes[fileName]++
	return storage.NewBlockID(fileName, blockNumber), nil
}

func (tx *materializePlanTransaction) Unpin(storage.BlockID) error {
	tx.calls++
	tx.unpins++
	return tx.unpinErr
}

func TestNewMaterializePlanCopiesRowsIntoTemporaryQueryScan(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 16)
	source := &materializePlan{
		schema:   schema,
		records:  2,
		rows:     [][]query.Constant{{query.NewIntConstant(4), query.NewStringConstant("Ada")}, {query.NewIntConstant(9), query.NewStringConstant("Lin")}},
		distinct: map[string]int{"id": 2, "name": 2},
	}
	transaction := newMaterializePlanTransaction()
	materializer := NewMaterializePlan(transaction, source)

	if got := materializer.Schema(); got != schema {
		t.Fatalf("Schema() = %p, want source schema %p", got, schema)
	}
	if got := materializer.RecordsOutput(); got != 2 {
		t.Errorf("RecordsOutput() = %d, want 2", got)
	}
	if got := materializer.DistinctValues("name"); got != 2 || len(source.distinctCalls) != 1 || source.distinctCalls[0] != "name" {
		t.Errorf("DistinctValues(name) = %d, delegated fields %v", got, source.distinctCalls)
	}
	if got := materializer.BlocksAccessed(); got != 1 {
		t.Errorf("BlocksAccessed() = %d, want 1", got)
	}

	scan, err := materializer.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if source.scan == nil || source.scan.closeCalls != 1 {
		t.Fatalf("source scan close calls = %v, want 1", source.scan)
	}
	for _, want := range source.rows {
		found, err := scan.Next()
		if err != nil || !found {
			t.Fatalf("Next() = (%t, %v), want a row", found, err)
		}
		for fieldIndex, fieldName := range schema.Fields() {
			got, err := scan.GetVal(fieldName)
			if err != nil || got != want[fieldIndex] {
				t.Errorf("GetVal(%q) = (%v, %v), want %v", fieldName, got, err, want[fieldIndex])
			}
		}
	}
	if found, err := scan.Next(); err != nil || found {
		t.Errorf("Next() after materialized rows = (%t, %v), want (false, nil)", found, err)
	}
	if err := scan.Close(); err != nil {
		t.Errorf("Close() materialized scan: %v", err)
	}
}

func TestMaterializePlanOpenEmptySourceReturnsEmptyRewindableScan(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	source := &materializePlan{schema: schema, records: 0}
	materializer := NewMaterializePlan(newMaterializePlanTransaction(), source)

	scan, err := materializer.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if found, err := scan.Next(); err != nil || found {
		t.Errorf("Next() on empty materialized scan = (%t, %v), want (false, nil)", found, err)
	}
	if err := scan.Close(); err != nil {
		t.Errorf("Close() materialized scan: %v", err)
	}
}

func TestMaterializePlanRejectsMissingDependencies(t *testing.T) {
	if _, err := (*MaterializePlan)(nil).Open(); err == nil {
		t.Error("nil receiver Open() succeeded")
	}
	if _, err := NewMaterializePlan(nil, &materializePlan{}).Open(); err == nil {
		t.Error("nil transaction Open() succeeded")
	}
	if _, err := NewMaterializePlan(newMaterializePlanTransaction(), nil).Open(); err == nil {
		t.Error("nil source Open() succeeded")
	}
	if _, err := NewMaterializePlan(newMaterializePlanTransaction(), &materializePlan{}).Open(); err == nil {
		t.Error("nil source schema Open() succeeded")
	}
	invalidSource := &materializePlan{}
	invalidTransaction := newMaterializePlanTransaction()
	if _, err := NewMaterializePlan(invalidTransaction, invalidSource).Open(); err == nil {
		t.Error("nil source schema Open() succeeded")
	}
	if invalidSource.openCalls != 0 || invalidTransaction.calls != 0 {
		t.Errorf("invalid schema performed open/calls = %d/%d, want 0/0", invalidSource.openCalls, invalidTransaction.calls)
	}
	if got := (*MaterializePlan)(nil).Schema(); got != nil {
		t.Errorf("nil plan Schema() = %v, want nil", got)
	}
	if got := (*MaterializePlan)(nil).BlocksAccessed(); got != 0 {
		t.Errorf("nil plan BlocksAccessed() = %d, want 0", got)
	}
	if got := (*MaterializePlan)(nil).RecordsOutput(); got != 0 {
		t.Errorf("nil plan RecordsOutput() = %d, want 0", got)
	}
	if got := (*MaterializePlan)(nil).DistinctValues("id"); got != 0 {
		t.Errorf("nil plan DistinctValues() = %d, want 0", got)
	}
	empty := NewMaterializePlan(newMaterializePlanTransaction(), &materializePlan{})
	if got := empty.Schema(); got != nil {
		t.Errorf("plan without source Schema() = %v, want nil", got)
	}
	if got := empty.RecordsOutput(); got != 0 {
		t.Errorf("plan without source RecordsOutput() = %d, want 0", got)
	}
	if got := empty.DistinctValues("id"); got != 0 {
		t.Errorf("plan without source DistinctValues() = %d, want 0", got)
	}
	if got := empty.BlocksAccessed(); got != 0 {
		t.Errorf("plan without source BlocksAccessed() = %d, want 0", got)
	}
}

func TestMaterializePlanOpenWrapsSourceAndDestinationInitializationErrors(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	sourceErr := errors.New("source open failed")
	sizeErr := errors.New("destination size failed")
	appendErr := errors.New("destination append failed")
	pinErr := errors.New("destination pin failed")
	blockSizeErr := errors.New("destination block size failed")
	formatErr := errors.New("destination format failed")
	unpinErr := errors.New("format cleanup unpin failed")
	tests := []struct {
		name            string
		transaction     *materializePlanTransaction
		source          *materializePlan
		causes          []error
		wantSourceClose int
		wantUnpins      int
	}{
		{name: "source open", source: &materializePlan{schema: schema, openErr: sourceErr}, causes: []error{sourceErr}},
		{name: "nil source scan", transaction: newMaterializePlanTransaction(), source: &materializePlan{schema: schema, nilScan: true}},
		{name: "size", transaction: &materializePlanTransaction{sizeErr: sizeErr}, source: &materializePlan{schema: schema}, causes: []error{sizeErr}, wantSourceClose: 1},
		{name: "append", transaction: &materializePlanTransaction{appendErr: appendErr}, source: &materializePlan{schema: schema}, causes: []error{appendErr}, wantSourceClose: 1},
		{name: "pin", transaction: &materializePlanTransaction{pinErr: pinErr}, source: &materializePlan{schema: schema}, causes: []error{pinErr}, wantSourceClose: 1},
		{name: "block size", transaction: &materializePlanTransaction{blockSize: 16, blockSizeErr: blockSizeErr}, source: &materializePlan{schema: schema}, causes: []error{blockSizeErr}, wantSourceClose: 1, wantUnpins: 1},
		{name: "format and cleanup", transaction: &materializePlanTransaction{blockSize: 16, setIntErr: formatErr, unpinErr: unpinErr}, source: &materializePlan{schema: schema}, causes: []error{formatErr, unpinErr}, wantSourceClose: 1, wantUnpins: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transaction := tt.transaction
			if transaction == nil {
				transaction = newMaterializePlanTransaction()
			}
			_, err := NewMaterializePlan(transaction, tt.source).Open()
			if err == nil {
				t.Fatal("Open() succeeded, want initialization error")
			}
			for _, cause := range tt.causes {
				if !errors.Is(err, cause) {
					t.Errorf("Open() error %v does not preserve %v", err, cause)
				}
			}
			if tt.source.scan != nil && tt.source.scan.closeCalls != tt.wantSourceClose {
				t.Errorf("source close calls = %d, want %d", tt.source.scan.closeCalls, tt.wantSourceClose)
			}
			if transaction.unpins != tt.wantUnpins {
				t.Errorf("unpin calls = %d, want %d", transaction.unpins, tt.wantUnpins)
			}
		})
	}
}

func TestMaterializePlanOpenClosesBothScansAndPreservesCopyErrors(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	nextErr := errors.New("source iteration failed")
	readErr := errors.New("source value failed")
	insertErr := errors.New("destination insert failed")
	writeErr := errors.New("destination value write failed")
	closeErr := errors.New("source close failed")
	unpinErr := errors.New("destination unpin failed")
	tests := []struct {
		name            string
		transaction     *materializePlanTransaction
		source          *materializePlan
		causes          []error
		wantSourceClose int
		wantUnpins      int
	}{
		{name: "next", transaction: newMaterializePlanTransaction(), source: &materializePlan{schema: schema, rows: [][]query.Constant{{query.NewIntConstant(1)}}, nextErr: nextErr, closeErr: closeErr}, causes: []error{nextErr, closeErr}, wantSourceClose: 1, wantUnpins: 1},
		{name: "field read", transaction: newMaterializePlanTransaction(), source: &materializePlan{schema: schema, rows: [][]query.Constant{{query.NewIntConstant(1)}}, getValErr: readErr}, causes: []error{readErr}, wantSourceClose: 1, wantUnpins: 1},
		{name: "insert", transaction: func() *materializePlanTransaction {
			tx := newMaterializePlanTransaction()
			tx.setIntFailAt = (tx.blockSize/record.NewLayout(schema).SlotSize())*2 + 1
			tx.setIntFailErr = insertErr
			return tx
		}(), source: &materializePlan{schema: schema, rows: [][]query.Constant{{query.NewIntConstant(1)}}}, causes: []error{insertErr}, wantSourceClose: 1, wantUnpins: 1},
		{name: "write", transaction: func() *materializePlanTransaction {
			tx := newMaterializePlanTransaction()
			tx.setIntFailAt = (tx.blockSize/record.NewLayout(schema).SlotSize())*2 + 2
			tx.setIntFailErr = writeErr
			return tx
		}(), source: &materializePlan{schema: schema, rows: [][]query.Constant{{query.NewIntConstant(1)}}}, causes: []error{writeErr}, wantSourceClose: 1, wantUnpins: 1},
		{name: "both cleanup errors", transaction: &materializePlanTransaction{unpinErr: unpinErr}, source: &materializePlan{schema: schema, rows: [][]query.Constant{{query.NewIntConstant(1)}}, nextErr: nextErr, closeErr: closeErr}, causes: []error{nextErr, closeErr, unpinErr}, wantSourceClose: 1, wantUnpins: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMaterializePlan(tt.transaction, tt.source).Open()
			if err == nil {
				t.Fatal("Open() succeeded, want copy error")
			}
			for _, cause := range tt.causes {
				if !errors.Is(err, cause) {
					t.Errorf("Open() error %v does not preserve %v", err, cause)
				}
			}
			if tt.source.scan == nil || tt.source.scan.closeCalls != tt.wantSourceClose {
				t.Errorf("source scan = %#v, want close calls %d", tt.source.scan, tt.wantSourceClose)
			}
			if tt.transaction.unpins != tt.wantUnpins {
				t.Errorf("destination unpin calls = %d, want %d", tt.transaction.unpins, tt.wantUnpins)
			}
		})
	}
}

func TestMaterializePlanOpenPreservesSourceCloseAndDestinationRewindErrors(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	closeErr := errors.New("source close failed")
	unpinErr := errors.New("destination rewind unpin failed")
	tests := []struct {
		name        string
		transaction *materializePlanTransaction
		source      *materializePlan
		cause       error
		wantUnpins  int
	}{
		{name: "source close", transaction: newMaterializePlanTransaction(), source: &materializePlan{schema: schema, closeErr: closeErr}, cause: closeErr, wantUnpins: 1},
		{name: "destination rewind and cleanup", transaction: &materializePlanTransaction{blockSize: 16, unpinErr: unpinErr}, source: &materializePlan{schema: schema}, cause: unpinErr, wantUnpins: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMaterializePlan(tt.transaction, tt.source).Open()
			if !errors.Is(err, tt.cause) {
				t.Errorf("Open() error = %v, want cause %v", err, tt.cause)
			}
			if tt.transaction.unpins != tt.wantUnpins {
				t.Errorf("unpin calls = %d, want %d", tt.transaction.unpins, tt.wantUnpins)
			}
		})
	}
}

func TestMaterializePlanBlocksAccessedReturnsZeroWhenEstimateIsUnavailable(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	positive := &materializePlan{schema: schema, records: 1}
	if got := NewMaterializePlan(nil, positive).BlocksAccessed(); got != 0 {
		t.Errorf("BlocksAccessed() with nil transaction = %d, want 0", got)
	}
	if got := NewMaterializePlan(newMaterializePlanTransaction(), &materializePlan{schema: schema}).BlocksAccessed(); got != 0 {
		t.Errorf("BlocksAccessed() with no rows = %d, want 0", got)
	}
	if got := NewMaterializePlan(newMaterializePlanTransaction(), &materializePlan{records: 1}).BlocksAccessed(); got != 0 {
		t.Errorf("BlocksAccessed() with nil schema = %d, want 0", got)
	}
	if got := NewMaterializePlan(&materializePlanTransaction{blockSizeErr: errors.New("block size failed")}, positive).BlocksAccessed(); got != 0 {
		t.Errorf("BlocksAccessed() with block size error = %d, want 0", got)
	}
	if got := NewMaterializePlan(&materializePlanTransaction{blockSize: 0}, positive).BlocksAccessed(); got != 0 {
		t.Errorf("BlocksAccessed() with zero block size = %d, want 0", got)
	}
	if got := NewMaterializePlan(&materializePlanTransaction{blockSize: 1}, positive).BlocksAccessed(); got != 0 {
		t.Errorf("BlocksAccessed() with no record per block = %d, want 0", got)
	}
}

func TestMaterializePlanBlocksAccessedRoundsExactAndPartialBlocks(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	slotSize := record.NewLayout(schema).SlotSize()
	for _, tt := range []struct {
		records int
		want    int
	}{{records: 2, want: 1}, {records: 3, want: 2}} {
		transaction := &materializePlanTransaction{blockSize: 2 * slotSize}
		source := &materializePlan{schema: schema, records: tt.records}
		if got := NewMaterializePlan(transaction, source).BlocksAccessed(); got != tt.want {
			t.Errorf("BlocksAccessed(%d records) = %d, want %d", tt.records, got, tt.want)
		}
	}
}

var _ plan.Plan = (*MaterializePlan)(nil)
var _ record.TableScanTransaction = (*materializePlanTransaction)(nil)
