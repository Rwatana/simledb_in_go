package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestEnsureLeafFileInitializesOnlyAnEmptyFile(t *testing.T) {
	for _, test := range []struct {
		name    string
		keyType record.FieldType
		size    int
	}{
		{name: "integer empty file", keyType: record.IntegerField},
		{name: "varchar empty file", keyType: record.VarcharField},
		{name: "existing file", keyType: record.IntegerField, size: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			layout := pageInsertLeafLayout(test.keyType)
			transaction := newBtreeFileTransaction(layout, test.size)

			if err := ensureLeafFile(transaction, "studentsleaf", layout); err != nil {
				t.Fatalf("ensureLeafFile() error = %v", err)
			}
			if transaction.sizeCalls != 1 || transaction.lastSizeFile != "studentsleaf" {
				t.Errorf("Size calls = (%d, %q), want (1, studentsleaf)", transaction.sizeCalls, transaction.lastSizeFile)
			}
			if test.size > 0 {
				if len(transaction.appendFiles) != 0 || transaction.pinCount != 0 || len(transaction.writes) != 0 {
					t.Errorf("existing leaf file changed: appends=%v pins=%d writes=%+v", transaction.appendFiles, transaction.pinCount, transaction.writes)
				}
				return
			}
			if len(transaction.appendFiles) != 1 || transaction.appendFiles[0] != "studentsleaf" {
				t.Fatalf("Append calls = %v, want [studentsleaf]", transaction.appendFiles)
			}
			if transaction.pinCount != 1 || transaction.unpinCount != 1 {
				t.Errorf("pin lifecycle = (%d, %d), want (1, 1)", transaction.pinCount, transaction.unpinCount)
			}
			if transaction.ints[0] != -1 || transaction.ints[4] != 0 {
				t.Errorf("leaf header = (flag %d, count %d), want (-1, 0)", transaction.ints[0], transaction.ints[4])
			}
			for _, write := range transaction.writes {
				if write.logged {
					t.Errorf("leaf file initialization unexpectedly logged a formatting write: %+v", write)
				}
			}
		})
	}
}

func TestEnsureLeafFileRejectsInvalidInputsBeforeAppend(t *testing.T) {
	validLayout := pageInsertLeafLayout(record.IntegerField)
	var typedNil *btreeFileTransactionProbe
	tests := []struct {
		name     string
		fileName string
		layout   *record.Layout
		newTx    func() btreeFileTransaction
	}{
		{name: "nil transaction", fileName: "leaf", layout: validLayout},
		{name: "typed nil transaction", fileName: "leaf", layout: validLayout, newTx: func() btreeFileTransaction { return typedNil }},
		{name: "empty file name", fileName: "", layout: validLayout, newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
		{name: "whitespace file name", fileName: "  ", layout: validLayout, newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
		{name: "nil layout", fileName: "leaf", newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
		{name: "directory layout", fileName: "leaf", layout: pageInsertDirLayout(record.IntegerField), newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var tx btreeFileTransaction
			if test.newTx != nil {
				tx = test.newTx()
			}
			if err := ensureLeafFile(tx, test.fileName, test.layout); err == nil {
				t.Fatal("ensureLeafFile() error = nil, want validation error")
			}
			if probe, ok := tx.(*btreeFileTransactionProbe); ok && probe != nil && len(probe.appendFiles) != 0 {
				t.Errorf("Append calls = %v, want none", probe.appendFiles)
			}
		})
	}
}

func TestEnsureLeafFileWrapsStorageAndPageErrors(t *testing.T) {
	cause := errors.New("leaf file operation failed")
	tests := []struct {
		name      string
		configure func(*btreeFileTransactionProbe)
		wantPins  int
		wantClose int
	}{
		{name: "size", configure: func(tx *btreeFileTransactionProbe) { tx.sizeErr = cause }},
		{name: "negative size", configure: func(tx *btreeFileTransactionProbe) { tx.fileSize = -1 }},
		{name: "append", configure: func(tx *btreeFileTransactionProbe) { tx.appendErr = cause }},
		{name: "wrong appended file", configure: func(tx *btreeFileTransactionProbe) { tx.appendBlock = storage.NewBlockID("otherleaf", 0) }},
		{name: "wrong appended number", configure: func(tx *btreeFileTransactionProbe) { tx.appendBlock = storage.NewBlockID("studentsleaf", 1) }},
		{name: "pin", configure: func(tx *btreeFileTransactionProbe) { tx.pinErr = cause }, wantPins: 1},
		{name: "format", configure: func(tx *btreeFileTransactionProbe) { tx.writeErrorAt = 1; tx.writeError = cause }, wantPins: 1, wantClose: 1},
		{name: "format and close", configure: func(tx *btreeFileTransactionProbe) {
			tx.writeErrorAt = 1
			tx.writeError = cause
			tx.unpinErr = errors.New("unpin failed")
		}, wantPins: 1, wantClose: 1},
		{name: "close", configure: func(tx *btreeFileTransactionProbe) { tx.unpinErr = cause }, wantPins: 1, wantClose: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout := pageInsertLeafLayout(record.IntegerField)
			tx := newBtreeFileTransaction(layout, 0)
			test.configure(tx)
			err := ensureLeafFile(tx, "studentsleaf", layout)
			if err == nil {
				t.Fatal("ensureLeafFile() error = nil, want failure")
			}
			if test.name != "negative size" && test.name != "wrong appended file" && test.name != "wrong appended number" && !errors.Is(err, cause) {
				t.Fatalf("ensureLeafFile() error = %v, want wrapped %v", err, cause)
			}
			if tx.pinCount != test.wantPins || tx.unpinCount != test.wantClose {
				t.Errorf("pin lifecycle = (%d, %d), want (%d, %d)", tx.pinCount, tx.unpinCount, test.wantPins, test.wantClose)
			}
		})
	}

	layout := pageInsertLeafLayout(record.IntegerField)
	tx := newBtreeFileTransaction(layout, 0)
	other := errors.New("unpin failed")
	tx.writeErrorAt = 1
	tx.writeError = cause
	tx.unpinErr = other
	err := ensureLeafFile(tx, "studentsleaf", layout)
	if !errors.Is(err, cause) || !errors.Is(err, other) {
		t.Fatalf("combined initialization/close error = %v, want both causes", err)
	}
}

func TestLeafPageFieldsRejectsNilLayout(t *testing.T) {
	if _, err := leafPageFields(nil); err == nil {
		t.Fatal("leafPageFields(nil) error = nil, want validation error")
	}
}

type btreeFileTransactionProbe struct {
	*pageFormatTransaction
	fileSize     int
	sizeErr      error
	sizeCalls    int
	lastSizeFile string
	appendErr    error
	appendBlock  storage.BlockID
	appendFiles  []string
	pinErr       error
	unpinErr     error
}

func newBtreeFileTransaction(layout *record.Layout, fileSize int) *btreeFileTransactionProbe {
	return &btreeFileTransactionProbe{
		pageFormatTransaction: newPageFormatTransaction(layout, 8+layout.SlotSize()),
		fileSize:              fileSize,
		appendBlock:           storage.NewBlockID("studentsleaf", 0),
	}
}

func (tx *btreeFileTransactionProbe) Size(fileName string) (int, error) {
	tx.sizeCalls++
	tx.lastSizeFile = fileName
	return tx.fileSize, tx.sizeErr
}

func (tx *btreeFileTransactionProbe) Append(fileName string) (storage.BlockID, error) {
	tx.appendFiles = append(tx.appendFiles, fileName)
	return tx.appendBlock, tx.appendErr
}

func (tx *btreeFileTransactionProbe) Pin(block storage.BlockID) error {
	tx.pinCount++
	return tx.pinErr
}

func (tx *btreeFileTransactionProbe) Unpin(storage.BlockID) error {
	tx.unpinCount++
	return tx.unpinErr
}

var _ btreeFileTransaction = (*btreeFileTransactionProbe)(nil)
