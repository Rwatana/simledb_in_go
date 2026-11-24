package btree

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewBTreeIndexInitializesEmptyFilesInJavaOrder(t *testing.T) {
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	transaction := newBTreeIndexInitializationTransaction(leafLayout)

	index, err := NewBTreeIndex(transaction, "students", leafLayout)
	if err != nil {
		t.Fatalf("NewBTreeIndex() error = %v", err)
	}
	if index == nil {
		t.Fatal("NewBTreeIndex() index = nil, want initialized index")
	}
	if index.transaction != transaction || index.indexName != "students" {
		t.Errorf("index transaction/name = (%p, %q), want (%p, students)", index.transaction, index.indexName, transaction)
	}
	if index.leafFileName != "studentsleaf" || index.directoryFileName != "studentsdir" {
		t.Errorf("index files = (%q, %q), want (studentsleaf, studentsdir)", index.leafFileName, index.directoryFileName)
	}
	if index.leafLayout != leafLayout {
		t.Errorf("leaf layout = %p, want retained layout %p", index.leafLayout, leafLayout)
	}
	if index.rootBlock != storage.NewBlockID("studentsdir", 0) {
		t.Errorf("root block = %v, want studentsdir block zero", index.rootBlock)
	}
	assertDerivedDirectoryLayout(t, index.directoryLayout, record.IntegerField, 0, 12)
	if got, want := transaction.sizeFiles, []string{"studentsleaf", "studentsdir"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Size call order = %v, want %v", got, want)
	}
	if got, want := transaction.appends, []string{"studentsleaf", "studentsdir"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Append call order = %v, want %v", got, want)
	}
	if transaction.pinCount != 2 || transaction.unpinCount != 2 {
		t.Errorf("page pin lifecycle = (%d, %d), want (2, 2)", transaction.pinCount, transaction.unpinCount)
	}
	leafLastWrite, directoryFirstWrite := -1, -1
	for i, write := range transaction.writes {
		switch write.block.FileName() {
		case "studentsleaf":
			leafLastWrite = i
		case "studentsdir":
			if directoryFirstWrite < 0 {
				directoryFirstWrite = i
			}
		}
	}
	if leafLastWrite < 0 || directoryFirstWrite < 0 || leafLastWrite >= directoryFirstWrite {
		t.Errorf("page write order has leaf end %d and directory start %d, want leaf fully initialized first", leafLastWrite, directoryFirstWrite)
	}
}

func TestNewBTreeIndexInitializesVarcharDirectoryRoot(t *testing.T) {
	leafLayout := pageInsertLeafLayout(record.VarcharField)
	transaction := newBTreeIndexInitializationTransaction(leafLayout)

	index, err := NewBTreeIndex(transaction, "names", leafLayout)
	if err != nil {
		t.Fatalf("NewBTreeIndex(varchar) error = %v", err)
	}
	assertDerivedDirectoryLayout(t, index.directoryLayout, record.VarcharField, 5, 32)
	position := int32(pageInsertDirPosition(index.directoryLayout, 0, "dataval"))
	foundMinimum := false
	for _, write := range transaction.writes {
		if write.block.FileName() == "namesdir" && write.offset == position && write.isString && write.textValue == "" && write.logged {
			foundMinimum = true
		}
	}
	if !foundMinimum {
		t.Fatal("directory root has no logged empty-varchar minimum-key entry")
	}
}

func TestNewBTreeIndexLeavesExistingFilesAndInitializesOnlyTheMissingFile(t *testing.T) {
	for _, test := range []struct {
		name          string
		leafSize      int
		directorySize int
		wantAppend    []string
	}{
		{name: "both exist", leafSize: 2, directorySize: 3},
		{name: "leaf exists", leafSize: 2, directorySize: 0, wantAppend: []string{"studentsdir"}},
		{name: "directory exists", leafSize: 0, directorySize: 3, wantAppend: []string{"studentsleaf"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			leafLayout := pageInsertLeafLayout(record.IntegerField)
			transaction := newBTreeIndexInitializationTransaction(leafLayout)
			transaction.sizes["studentsleaf"] = test.leafSize
			transaction.sizes["studentsdir"] = test.directorySize

			index, err := NewBTreeIndex(transaction, "students", leafLayout)
			if err != nil {
				t.Fatalf("NewBTreeIndex() error = %v", err)
			}
			if index == nil {
				t.Fatal("NewBTreeIndex() index = nil, want initialized index")
			}
			if !reflect.DeepEqual(transaction.appends, test.wantAppend) {
				t.Errorf("Append calls = %v, want %v", transaction.appends, test.wantAppend)
			}
			if len(transaction.writes) != 0 && len(test.wantAppend) == 0 {
				t.Errorf("existing files were formatted: %+v", transaction.writes)
			}
			if transaction.pinCount != len(test.wantAppend) || transaction.unpinCount != len(test.wantAppend) {
				t.Errorf("page pin lifecycle = (%d, %d), want (%d, %d)", transaction.pinCount, transaction.unpinCount, len(test.wantAppend), len(test.wantAppend))
			}
		})
	}
}

func TestNewBTreeIndexRejectsInvalidInputsBeforeFileAccess(t *testing.T) {
	validLayout := pageInsertLeafLayout(record.IntegerField)
	var typedNil *btreeIndexInitializationTransaction
	tests := []struct {
		name      string
		indexName string
		layout    *record.Layout
		newTx     func() btreeFileTransaction
	}{
		{name: "nil transaction", indexName: "students", layout: validLayout},
		{name: "typed nil transaction", indexName: "students", layout: validLayout, newTx: func() btreeFileTransaction { return typedNil }},
		{name: "empty name", indexName: "", layout: validLayout, newTx: func() btreeFileTransaction { return newBTreeIndexInitializationTransaction(validLayout) }},
		{name: "whitespace name", indexName: "  ", layout: validLayout, newTx: func() btreeFileTransaction { return newBTreeIndexInitializationTransaction(validLayout) }},
		{name: "nil leaf layout", indexName: "students", newTx: func() btreeFileTransaction { return newBTreeIndexInitializationTransaction(validLayout) }},
		{name: "invalid leaf layout", indexName: "students", layout: newLeafLayoutWithoutDataval(), newTx: func() btreeFileTransaction { return newBTreeIndexInitializationTransaction(validLayout) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var transaction btreeFileTransaction
			if test.newTx != nil {
				transaction = test.newTx()
			}
			index, err := NewBTreeIndex(transaction, test.indexName, test.layout)
			if err == nil {
				t.Fatalf("NewBTreeIndex() = %v, nil error; want validation error", index)
			}
			if index != nil {
				t.Errorf("NewBTreeIndex() index = %v, want nil on error", index)
			}
			if probe, ok := transaction.(*btreeIndexInitializationTransaction); ok && probe != nil && len(probe.sizeFiles) != 0 {
				t.Errorf("Size calls = %v, want no file access", probe.sizeFiles)
			}
		})
	}
}

func TestNewBTreeIndexReturnsLeafAndDirectoryInitializationErrors(t *testing.T) {
	cause := errors.New("B-tree index file operation failed")
	for _, test := range []struct {
		name        string
		configure   func(*btreeIndexInitializationTransaction)
		wantSize    []string
		wantAppends []string
	}{
		{name: "leaf size", configure: func(tx *btreeIndexInitializationTransaction) { tx.sizeErrors["studentsleaf"] = cause }, wantSize: []string{"studentsleaf"}},
		{name: "directory size after leaf append", configure: func(tx *btreeIndexInitializationTransaction) { tx.sizeErrors["studentsdir"] = cause }, wantSize: []string{"studentsleaf", "studentsdir"}, wantAppends: []string{"studentsleaf"}},
		{name: "directory append after leaf append", configure: func(tx *btreeIndexInitializationTransaction) { tx.appendErrors["studentsdir"] = cause }, wantSize: []string{"studentsleaf", "studentsdir"}, wantAppends: []string{"studentsleaf", "studentsdir"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			leafLayout := pageInsertLeafLayout(record.IntegerField)
			transaction := newBTreeIndexInitializationTransaction(leafLayout)
			test.configure(transaction)

			index, err := NewBTreeIndex(transaction, "students", leafLayout)
			if !errors.Is(err, cause) {
				t.Fatalf("NewBTreeIndex() error = %v, want wrapped cause", err)
			}
			if index != nil {
				t.Errorf("NewBTreeIndex() index = %v, want nil on error", index)
			}
			if !reflect.DeepEqual(transaction.sizeFiles, test.wantSize) {
				t.Errorf("Size calls = %v, want %v", transaction.sizeFiles, test.wantSize)
			}
			if !reflect.DeepEqual(transaction.appends, test.wantAppends) {
				t.Errorf("Append calls = %v, want %v", transaction.appends, test.wantAppends)
			}
		})
	}
}

type btreeIndexInitializationTransaction struct {
	*btreeFileTransactionProbe
	sizes        map[string]int
	sizeErrors   map[string]error
	sizeFiles    []string
	appendErrors map[string]error
	appendBlocks map[string]storage.BlockID
	appends      []string
}

func newBTreeIndexInitializationTransaction(layout *record.Layout) *btreeIndexInitializationTransaction {
	return &btreeIndexInitializationTransaction{
		btreeFileTransactionProbe: newBtreeFileTransaction(layout, 0),
		sizes:                     make(map[string]int),
		sizeErrors:                make(map[string]error),
		appendErrors:              make(map[string]error),
		appendBlocks:              make(map[string]storage.BlockID),
	}
}

func (transaction *btreeIndexInitializationTransaction) Size(fileName string) (int, error) {
	transaction.sizeFiles = append(transaction.sizeFiles, fileName)
	return transaction.sizes[fileName], transaction.sizeErrors[fileName]
}

func (transaction *btreeIndexInitializationTransaction) Append(fileName string) (storage.BlockID, error) {
	transaction.appends = append(transaction.appends, fileName)
	block, ok := transaction.appendBlocks[fileName]
	if !ok {
		block = storage.NewBlockID(fileName, 0)
	}
	return block, transaction.appendErrors[fileName]
}

var _ btreeFileTransaction = (*btreeIndexInitializationTransaction)(nil)
