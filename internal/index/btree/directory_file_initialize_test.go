package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestEnsureDirectoryFileInitializesIntegerAndVarcharRoots(t *testing.T) {
	for _, test := range []struct {
		name    string
		keyType record.FieldType
	}{
		{name: "integer minimum key", keyType: record.IntegerField},
		{name: "varchar empty key", keyType: record.VarcharField},
	} {
		t.Run(test.name, func(t *testing.T) {
			layout := pageInsertDirLayout(test.keyType)
			tx := newBtreeFileTransaction(layout, 0)
			tx.appendBlock = storage.NewBlockID("studentsdir", 0)

			if err := ensureDirectoryFile(tx, "studentsdir", layout); err != nil {
				t.Fatalf("ensureDirectoryFile() error = %v", err)
			}
			if tx.sizeCalls != 1 || tx.lastSizeFile != "studentsdir" {
				t.Errorf("Size calls = (%d, %q), want (1, studentsdir)", tx.sizeCalls, tx.lastSizeFile)
			}
			if len(tx.appendFiles) != 1 || tx.appendFiles[0] != "studentsdir" {
				t.Fatalf("Append calls = %v, want [studentsdir]", tx.appendFiles)
			}
			if tx.pinCount != 1 || tx.unpinCount != 1 {
				t.Errorf("pin lifecycle = (%d, %d), want (1, 1)", tx.pinCount, tx.unpinCount)
			}
			if tx.ints[0] != 0 || tx.ints[4] != 1 {
				t.Errorf("directory header = (level %d, count %d), want (0, 1)", tx.ints[0], tx.ints[4])
			}
			if tx.ints[pageInsertDirPosition(layout, 0, "block")] != 0 {
				t.Errorf("initial child block = %d, want zero", tx.ints[pageInsertDirPosition(layout, 0, "block")])
			}
			if test.keyType == record.IntegerField {
				if got := tx.ints[pageInsertDirPosition(layout, 0, "dataval")]; got != math.MinInt32 {
					t.Errorf("integer minimum key = %d, want %d", got, int32(math.MinInt32))
				}
			} else if got := tx.strings[pageInsertDirPosition(layout, 0, "dataval")]; got != "" {
				t.Errorf("varchar minimum key = %q, want empty string", got)
			}
			for i, write := range tx.writes {
				if i < 4 && write.logged {
					t.Errorf("format write %d requested logging: %+v", i, write)
				}
				if i >= 4 && !write.logged {
					t.Errorf("root entry write %d did not request logging: %+v", i, write)
				}
			}
		})
	}
}

func TestEnsureDirectoryFileLeavesExistingFileUnchanged(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	tx := newBtreeFileTransaction(layout, 2)
	tx.appendBlock = storage.NewBlockID("studentsdir", 0)

	if err := ensureDirectoryFile(tx, "studentsdir", layout); err != nil {
		t.Fatalf("ensureDirectoryFile() error = %v", err)
	}
	if len(tx.appendFiles) != 0 || tx.pinCount != 0 || tx.unpinCount != 0 || len(tx.writes) != 0 {
		t.Errorf("existing directory file changed: appends=%v pins=%d/%d writes=%+v", tx.appendFiles, tx.pinCount, tx.unpinCount, tx.writes)
	}
}

func TestEnsureDirectoryFileRejectsInvalidInputsBeforeAppend(t *testing.T) {
	validLayout := pageInsertDirLayout(record.IntegerField)
	var typedNil *btreeFileTransactionProbe
	tests := []struct {
		name     string
		fileName string
		layout   *record.Layout
		newTx    func() btreeFileTransaction
	}{
		{name: "nil transaction", fileName: "dir", layout: validLayout},
		{name: "typed nil transaction", fileName: "dir", layout: validLayout, newTx: func() btreeFileTransaction { return typedNil }},
		{name: "empty file name", fileName: "", layout: validLayout, newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
		{name: "nil layout", fileName: "dir", newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
		{name: "leaf layout", fileName: "dir", layout: pageInsertLeafLayout(record.IntegerField), newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
		{name: "noninteger block field", fileName: "dir", layout: pageInsertDirLayoutWithTypes(record.IntegerField, record.VarcharField), newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
		{name: "missing dataval field", fileName: "dir", layout: record.NewLayout(pageInsertDirSchemaWithoutDataval()), newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
		{name: "missing block field", fileName: "dir", layout: record.NewLayout(pageInsertDirSchemaWithoutBlock()), newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
		{name: "invalid layout metadata", fileName: "dir", layout: record.NewLayoutFromMetadata(nil, nil, 8), newTx: func() btreeFileTransaction { return newBtreeFileTransaction(validLayout, 0) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var tx btreeFileTransaction
			if test.newTx != nil {
				tx = test.newTx()
			}
			if err := ensureDirectoryFile(tx, test.fileName, test.layout); err == nil {
				t.Fatal("ensureDirectoryFile() error = nil, want validation error")
			}
			if probe, ok := tx.(*btreeFileTransactionProbe); ok && probe != nil && len(probe.appendFiles) != 0 {
				t.Errorf("Append calls = %v, want none", probe.appendFiles)
			}
		})
	}
}

func TestEnsureDirectoryFileWrapsStorageAndPageErrors(t *testing.T) {
	cause := errors.New("directory file operation failed")
	tests := []struct {
		name      string
		configure func(*btreeFileTransactionProbe)
	}{
		{name: "size", configure: func(tx *btreeFileTransactionProbe) { tx.sizeErr = cause }},
		{name: "negative size", configure: func(tx *btreeFileTransactionProbe) { tx.fileSize = -1 }},
		{name: "append", configure: func(tx *btreeFileTransactionProbe) { tx.appendErr = cause }},
		{name: "wrong appended file", configure: func(tx *btreeFileTransactionProbe) { tx.appendBlock = storage.NewBlockID("otherdir", 0) }},
		{name: "wrong appended number", configure: func(tx *btreeFileTransactionProbe) { tx.appendBlock = storage.NewBlockID("studentsdir", 1) }},
		{name: "pin", configure: func(tx *btreeFileTransactionProbe) { tx.pinErr = cause }},
		{name: "format", configure: func(tx *btreeFileTransactionProbe) { tx.writeErrorAt = 1; tx.writeError = cause }},
		{name: "root entry", configure: func(tx *btreeFileTransactionProbe) { tx.writeErrorAt = 5; tx.writeError = cause }},
		{name: "format and close", configure: func(tx *btreeFileTransactionProbe) {
			tx.writeErrorAt = 1
			tx.writeError = cause
			tx.unpinErr = errors.New("unpin failed")
		}},
		{name: "close", configure: func(tx *btreeFileTransactionProbe) { tx.unpinErr = cause }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout := pageInsertDirLayout(record.IntegerField)
			tx := newBtreeFileTransaction(layout, 0)
			tx.appendBlock = storage.NewBlockID("studentsdir", 0)
			test.configure(tx)
			err := ensureDirectoryFile(tx, "studentsdir", layout)
			if err == nil {
				t.Fatal("ensureDirectoryFile() error = nil, want failure")
			}
			if test.name != "negative size" && test.name != "wrong appended file" && test.name != "wrong appended number" && !errors.Is(err, cause) {
				t.Fatalf("ensureDirectoryFile() error = %v, want wrapped %v", err, cause)
			}
		})
	}

	layout := pageInsertDirLayout(record.IntegerField)
	tx := newBtreeFileTransaction(layout, 0)
	tx.appendBlock = storage.NewBlockID("studentsdir", 0)
	other := errors.New("unpin failed")
	tx.writeErrorAt = 1
	tx.writeError = cause
	tx.unpinErr = other
	err := ensureDirectoryFile(tx, "studentsdir", layout)
	if !errors.Is(err, cause) || !errors.Is(err, other) {
		t.Fatalf("combined initialization/close error = %v, want both causes", err)
	}
}

func TestDirectoryPageKeyTypeRejectsNilLayout(t *testing.T) {
	if _, err := directoryPageKeyType(nil); err == nil {
		t.Fatal("directoryPageKeyType(nil) error = nil, want validation error")
	}
}
