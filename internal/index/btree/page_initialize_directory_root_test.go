package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageInitializeDirectoryRootFormatsIntegerMinimumEntry(t *testing.T) {
	page, tx, block, layout := newDirectoryRootInitializationPage(t, record.IntegerField, 1)
	minimum := query.NewIntConstant(math.MinInt32)

	if err := page.InitializeDirectoryRoot(minimum); err != nil {
		t.Fatalf("InitializeDirectoryRoot() error = %v", err)
	}
	if got := tx.ints[0]; got != 0 {
		t.Errorf("root level = %d, want 0", got)
	}
	if got := tx.ints[4]; got != 1 {
		t.Errorf("root record count = %d, want 1", got)
	}
	if got := tx.ints[pageInsertDirPosition(layout, 0, "dataval")]; got != math.MinInt32 {
		t.Errorf("minimum key = %d, want %d", got, int32(math.MinInt32))
	}
	if got := tx.ints[pageInsertDirPosition(layout, 0, "block")]; got != 0 {
		t.Errorf("initial leaf child = %d, want block zero", got)
	}
	if page.block != block {
		t.Errorf("root page block = %v, want unchanged %v", page.block, block)
	}
	if tx.pinCount != 0 || tx.unpinCount != 0 {
		t.Errorf("pin lifecycle changed: pin=%d unpin=%d", tx.pinCount, tx.unpinCount)
	}
	if len(tx.writes) < 5 {
		t.Fatalf("writes = %+v, want format plus entry writes", tx.writes)
	}
	for index, write := range tx.writes[:4] {
		if write.logged {
			t.Errorf("format write %d unexpectedly requested logging: %+v", index, write)
		}
	}
	for index, write := range tx.writes[4:] {
		if !write.logged {
			t.Errorf("directory entry write %d did not request logging: %+v", index, write)
		}
	}
}

func TestPageInitializeDirectoryRootFormatsVarcharMinimumEntry(t *testing.T) {
	page, tx, _, layout := newDirectoryRootInitializationPage(t, record.VarcharField, 1)
	if err := page.InitializeDirectoryRoot(query.NewStringConstant("")); err != nil {
		t.Fatalf("InitializeDirectoryRoot(varchar) error = %v", err)
	}
	if got := tx.ints[0]; got != 0 {
		t.Errorf("root level = %d, want 0", got)
	}
	if got := tx.ints[4]; got != 1 {
		t.Errorf("root record count = %d, want 1", got)
	}
	if got := tx.strings[pageInsertDirPosition(layout, 0, "dataval")]; got != "" {
		t.Errorf("minimum key = %q, want empty string", got)
	}
	if got := tx.ints[pageInsertDirPosition(layout, 0, "block")]; got != 0 {
		t.Errorf("initial leaf child = %d, want block zero", got)
	}
}

func TestPageInitializeDirectoryRootRejectsInvalidStateBeforeWriting(t *testing.T) {
	validLayout := pageInsertDirLayout(record.IntegerField)
	root := storage.NewBlockID("root-init.idx", 0)
	var typedNil *pageFormatTransaction
	tests := []struct {
		name string
		key  query.Constant
		page *Page
		tx   *pageFormatTransaction
	}{
		{name: "nil page", key: query.NewIntConstant(0)},
		{name: "closed page", page: &Page{}, key: query.NewIntConstant(0)},
		{name: "nil transaction", page: &Page{block: root, layout: validLayout}, key: query.NewIntConstant(0)},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: root, layout: validLayout}, key: query.NewIntConstant(0)},
		{name: "nil layout", page: &Page{block: root}, tx: newDirectoryRootInitializationTransaction(validLayout, 8+validLayout.SlotSize()), key: query.NewIntConstant(0)},
		{name: "not block zero", page: &Page{block: storage.NewBlockID("root-init.idx", 1), layout: validLayout}, tx: newDirectoryRootInitializationTransaction(validLayout, 8+validLayout.SlotSize()), key: query.NewIntConstant(0)},
		{name: "mismatched key type", key: query.NewStringConstant("wrong")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := test.page
			tx := test.tx
			if page == nil && test.name != "nil page" {
				page, tx, _, _ = newDirectoryRootInitializationPage(t, record.IntegerField, 1)
			}
			if page != nil && test.tx != nil {
				tx = test.tx
				page.transaction = tx
			}
			if page != nil && test.name == "not block zero" {
				page.transaction = tx
			}
			if page != nil && test.name == "mismatched key type" {
				page, tx, _, _ = newDirectoryRootInitializationPage(t, record.IntegerField, 1)
			}
			if page != nil && test.name == "typed nil transaction" {
				tx = nil
			}
			if err := page.InitializeDirectoryRoot(test.key); err == nil {
				t.Fatal("InitializeDirectoryRoot() error = nil, want validation error")
			}
			if tx != nil && len(tx.writes) != 0 {
				t.Errorf("invalid root initialization wrote page data: %+v", tx.writes)
			}
		})
	}

	for name, layout := range map[string]*record.Layout{
		"leaf layout":             pageInsertLeafLayout(record.IntegerField),
		"noninteger block field":  pageInsertDirLayoutWithTypes(record.IntegerField, record.VarcharField),
		"missing dataval field":   newDirectoryLayoutWithoutDataVal(),
		"missing block field":     newDirectoryOnlyLayout(record.IntegerField),
		"invalid layout metadata": record.NewLayoutFromMetadata(nil, nil, 8),
	} {
		t.Run(name, func(t *testing.T) {
			tx := newDirectoryRootInitializationTransaction(layout, 8+layout.SlotSize())
			page := &Page{transaction: tx, block: root, layout: layout}
			if err := page.InitializeDirectoryRoot(query.NewIntConstant(0)); err == nil {
				t.Fatal("InitializeDirectoryRoot() error = nil, want directory-layout validation error")
			}
			if len(tx.writes) != 0 {
				t.Errorf("invalid layout wrote page data: %+v", tx.writes)
			}
		})
	}
}

func TestPageInitializeDirectoryRootRejectsInsufficientCapacityAndBlockSizeErrors(t *testing.T) {
	page, tx, _, _ := newDirectoryRootInitializationPage(t, record.IntegerField, 0)
	if err := page.InitializeDirectoryRoot(query.NewIntConstant(math.MinInt32)); err == nil {
		t.Fatal("InitializeDirectoryRoot() with no slot capacity returned nil")
	}
	if len(tx.writes) != 0 {
		t.Fatalf("no-capacity root initialization wrote page data: %+v", tx.writes)
	}

	page, tx, _, _ = newDirectoryRootInitializationPage(t, record.IntegerField, 1)
	tx.blockSize = 7
	if err := page.InitializeDirectoryRoot(query.NewIntConstant(math.MinInt32)); err == nil {
		t.Fatal("InitializeDirectoryRoot() with a short header returned nil")
	}
	if len(tx.writes) != 0 {
		t.Fatalf("short-header root initialization wrote page data: %+v", tx.writes)
	}

	page, tx, _, _ = newDirectoryRootInitializationPage(t, record.IntegerField, 1)
	tx.blockSizeErr = errors.New("block size read failed")
	if err := page.InitializeDirectoryRoot(query.NewIntConstant(math.MinInt32)); !errors.Is(err, tx.blockSizeErr) {
		t.Fatalf("InitializeDirectoryRoot() error = %v, want wrapped block-size failure", err)
	}
	if len(tx.writes) != 0 {
		t.Fatalf("block-size error wrote page data: %+v", tx.writes)
	}
}

func TestPageInitializeDirectoryRootWrapsFormatAndEntryWriteFailures(t *testing.T) {
	cause := errors.New("root format write failed")
	page, tx, _, _ := newDirectoryRootInitializationPage(t, record.IntegerField, 1)
	tx.writeErrorAt = 1
	tx.writeError = cause
	if err := page.InitializeDirectoryRoot(query.NewIntConstant(math.MinInt32)); !errors.Is(err, cause) {
		t.Fatalf("format error = %v, want wrapped cause", err)
	}

	page, tx, _, _ = newDirectoryRootInitializationPage(t, record.IntegerField, 1)
	formatWrites := 4
	tx.writeErrorAt = formatWrites + 2
	tx.writeError = cause
	if err := page.InitializeDirectoryRoot(query.NewIntConstant(math.MinInt32)); !errors.Is(err, cause) {
		t.Fatalf("entry error = %v, want wrapped cause", err)
	}
}

func TestPageInitializeDirectoryRootRejectsWrongVarcharKeyTypeBeforeWriting(t *testing.T) {
	page, tx, _, _ := newDirectoryRootInitializationPage(t, record.VarcharField, 1)
	if err := page.InitializeDirectoryRoot(query.NewIntConstant(0)); err == nil {
		t.Fatal("InitializeDirectoryRoot() with integer key for varchar directory returned nil")
	}
	if len(tx.writes) != 0 {
		t.Fatalf("mismatched varchar root key wrote page data: %+v", tx.writes)
	}
}

func newDirectoryRootInitializationPage(t *testing.T, keyType record.FieldType, slots int) (*Page, *pageFormatTransaction, storage.BlockID, *record.Layout) {
	t.Helper()
	layout := pageInsertDirLayout(keyType)
	tx := newDirectoryRootInitializationTransaction(layout, 8+slots*layout.SlotSize())
	block := storage.NewBlockID("root-init.idx", 0)
	page := &Page{transaction: tx, block: block, layout: layout}
	return page, tx, block, layout
}

func newDirectoryRootInitializationTransaction(layout *record.Layout, blockSize int) *pageFormatTransaction {
	return &pageFormatTransaction{pageInsertDirTransaction: pageInsertDirTransactionWithCount(layout, 0), blockSize: blockSize}
}
