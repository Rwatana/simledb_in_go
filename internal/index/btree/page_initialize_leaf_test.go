package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageInitializeLeafPageFormatsIntegerAndVarcharPages(t *testing.T) {
	for _, test := range []struct {
		name     string
		keyType  record.FieldType
		blockNum int
	}{
		{name: "integer first leaf", keyType: record.IntegerField, blockNum: 0},
		{name: "varchar overflow leaf", keyType: record.VarcharField, blockNum: 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			layout := pageInsertLeafLayout(test.keyType)
			block := storage.NewBlockID("leaf-init.tbl", test.blockNum)
			transaction := &pageFormatTransaction{
				pageInsertDirTransaction: pageInsertDirTransactionWithCount(layout, 9),
				blockSize:                8 + 2*layout.SlotSize(),
			}
			page := &Page{transaction: transaction, block: block, layout: layout}

			if err := page.InitializeLeafPage(); err != nil {
				t.Fatalf("InitializeLeafPage() error = %v", err)
			}
			if got := transaction.ints[0]; got != -1 {
				t.Errorf("leaf overflow flag = %d, want -1", got)
			}
			if got := transaction.ints[4]; got != 0 {
				t.Errorf("leaf record count = %d, want 0", got)
			}
			for slot := 0; slot < 2; slot++ {
				if test.keyType == record.IntegerField {
					if got := transaction.ints[pageInsertLeafPosition(layout, slot, "dataval")]; got != 0 {
						t.Errorf("slot %d default integer key = %d, want 0", slot, got)
					}
				} else if got := transaction.strings[pageInsertLeafPosition(layout, slot, "dataval")]; got != "" {
					t.Errorf("slot %d default varchar key = %q, want empty", slot, got)
				}
				for _, field := range []string{"block", "id"} {
					if got := transaction.ints[pageInsertLeafPosition(layout, slot, field)]; got != 0 {
						t.Errorf("slot %d default %s = %d, want 0", slot, field, got)
					}
				}
			}
			if page.block != block {
				t.Errorf("leaf page block = %v, want unchanged %v", page.block, block)
			}
			if transaction.pinCount != 0 || transaction.unpinCount != 0 {
				t.Errorf("pin lifecycle changed: pin=%d unpin=%d", transaction.pinCount, transaction.unpinCount)
			}
			if len(transaction.writes) != 2+2*len(layout.Schema().Fields()) {
				t.Fatalf("format writes = %+v, want header and both leaf slots", transaction.writes)
			}
			for index, write := range transaction.writes {
				if write.logged {
					t.Errorf("format write %d requested logging: %+v", index, write)
				}
			}
		})
	}
}

func TestPageInitializeLeafPageRejectsInvalidPageAndLayoutBeforeWrites(t *testing.T) {
	validLayout := pageInsertLeafLayout(record.IntegerField)
	block := storage.NewBlockID("leaf-init.tbl", 0)
	var typedNil *pageFormatTransaction
	tests := []struct {
		name string
		page *Page
		tx   *pageFormatTransaction
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{}},
		{name: "nil transaction", page: &Page{block: block, layout: validLayout}},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: block, layout: validLayout}},
		{name: "nil layout", page: &Page{block: block}},
		{name: "negative block", page: &Page{block: storage.NewBlockID("leaf-init.tbl", -1), layout: validLayout}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := test.page
			tx := test.tx
			if test.name == "nil page" {
				if err := page.InitializeLeafPage(); err == nil {
					t.Fatal("InitializeLeafPage() on nil page returned nil")
				}
				return
			}
			if page == nil {
				page, tx, _, _ = newLeafInitializationPage(t, record.IntegerField, 0, 1)
			}
			if test.name == "nil layout" {
				tx = newLeafInitializationTransaction(validLayout, 8+validLayout.SlotSize())
				page.transaction = tx
				page.layout = nil
			}
			if test.name == "negative block" {
				tx = newLeafInitializationTransaction(validLayout, 8+validLayout.SlotSize())
				page.transaction = tx
			}
			if err := page.InitializeLeafPage(); err == nil {
				t.Fatal("InitializeLeafPage() error = nil, want validation error")
			}
			if tx != nil && len(tx.writes) != 0 {
				t.Errorf("invalid leaf initialization wrote page data: %+v", tx.writes)
			}
		})
	}

	for name, layout := range map[string]*record.Layout{
		"directory layout":           pageInsertDirLayout(record.IntegerField),
		"missing dataval field":      newLeafLayoutWithoutDataval(),
		"noninteger block field":     pageInsertLeafLayoutWithTypes(record.IntegerField, record.VarcharField, record.IntegerField),
		"noninteger record ID field": pageInsertLeafLayoutWithTypes(record.IntegerField, record.IntegerField, record.VarcharField),
		"invalid layout metadata":    record.NewLayoutFromMetadata(nil, nil, 8),
	} {
		t.Run(name, func(t *testing.T) {
			tx := newLeafInitializationTransaction(layout, 8+layout.SlotSize())
			page := &Page{transaction: tx, block: block, layout: layout}
			if err := page.InitializeLeafPage(); err == nil {
				t.Fatal("InitializeLeafPage() error = nil, want layout validation error")
			}
			if len(tx.writes) != 0 {
				t.Errorf("invalid leaf layout wrote page data: %+v", tx.writes)
			}
		})
	}
}

func TestPageInitializeLeafPageWrapsFormattingErrors(t *testing.T) {
	page, transaction, _, _ := newLeafInitializationPage(t, record.IntegerField, 0, 1)
	cause := errors.New("leaf format failed")
	transaction.writeErrorAt = 1
	transaction.writeError = cause
	if err := page.InitializeLeafPage(); !errors.Is(err, cause) {
		t.Fatalf("InitializeLeafPage() error = %v, want wrapped cause", err)
	}

	page, transaction, _, _ = newLeafInitializationPage(t, record.IntegerField, 0, 1)
	transaction.blockSizeErr = cause
	if err := page.InitializeLeafPage(); !errors.Is(err, cause) {
		t.Fatalf("InitializeLeafPage() block-size error = %v, want wrapped cause", err)
	}
}

func newLeafInitializationPage(t *testing.T, keyType record.FieldType, blockNum, slots int) (*Page, *pageFormatTransaction, storage.BlockID, *record.Layout) {
	t.Helper()
	layout := pageInsertLeafLayout(keyType)
	transaction := newLeafInitializationTransaction(layout, 8+slots*layout.SlotSize())
	block := storage.NewBlockID("leaf-init.tbl", blockNum)
	page := &Page{transaction: transaction, block: block, layout: layout}
	return page, transaction, block, layout
}

func newLeafInitializationTransaction(layout *record.Layout, blockSize int) *pageFormatTransaction {
	return &pageFormatTransaction{pageInsertDirTransaction: pageInsertDirTransactionWithCount(layout, 0), blockSize: blockSize}
}

func newLeafLayoutWithoutDataval() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("child")
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	return record.NewLayout(&schema)
}

func pageInsertLeafLayoutWithTypes(datavalType, blockType, idType record.FieldType) *record.Layout {
	var schema record.Schema
	schema.AddField("dataval", datavalType, 20)
	schema.AddField("block", blockType, 0)
	schema.AddField("id", idType, 0)
	return record.NewLayout(&schema)
}
