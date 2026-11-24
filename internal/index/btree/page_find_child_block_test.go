package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageFindChildReadFailure struct {
	*pageInsertDirTransaction
	intOffset           int
	intFailureNumber    int
	intReadNumber       int
	stringOffset        int
	stringFailureNumber int
	stringReadNumber    int
	cause               error
}

func (tx *pageFindChildReadFailure) GetInt(block storage.BlockID, offset int) (int32, error) {
	if offset == tx.intOffset {
		tx.intReadNumber++
		if tx.intReadNumber == tx.intFailureNumber {
			return 0, tx.cause
		}
	}
	return tx.pageInsertDirTransaction.GetInt(block, offset)
}

func (tx *pageFindChildReadFailure) GetString(block storage.BlockID, offset int) (string, error) {
	if offset == tx.stringOffset {
		tx.stringReadNumber++
		if tx.stringReadNumber == tx.stringFailureNumber {
			return "", tx.cause
		}
	}
	return tx.pageInsertDirTransaction.GetString(block, offset)
}

func TestPageFindChildBlockRoutesIntegerKeys(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	transaction := pageInsertDirTransactionWithCount(layout, 4)
	block := storage.NewBlockID("directory.idx", 7)
	page := &Page{transaction: transaction, block: block, layout: layout}
	for slot, entry := range []struct{ key, child int32 }{
		{10, 100}, {20, 200}, {20, 201}, {40, 400},
	} {
		transaction.ints[pageInsertDirPosition(layout, slot, "dataval")] = entry.key
		transaction.ints[pageInsertDirPosition(layout, slot, "block")] = entry.child
	}

	for _, test := range []struct {
		name string
		key  int32
		want int
	}{
		{name: "minimum exact key", key: 10, want: 100},
		{name: "first duplicate exact key", key: 20, want: 200},
		{name: "greatest predecessor", key: 21, want: 201},
		{name: "last exact key", key: 40, want: 400},
		{name: "greater than last key", key: 50, want: 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := page.FindChildBlock(query.NewIntConstant(test.key))
			if err != nil {
				t.Fatalf("FindChildBlock(%d) error = %v", test.key, err)
			}
			if got != storage.NewBlockID(block.FileName(), test.want) {
				t.Fatalf("FindChildBlock(%d) = %v, want block %d in %q", test.key, got, test.want, block.FileName())
			}
		})
	}
	if transaction.pinCount != 0 || transaction.unpinCount != 0 || len(transaction.writes) != 0 {
		t.Fatal("FindChildBlock changed page pin ownership or contents")
	}
}

func TestPageFindChildBlockRoutesVarcharKeys(t *testing.T) {
	layout := pageInsertDirLayout(record.VarcharField)
	transaction := pageInsertDirTransactionWithCount(layout, 4)
	block := storage.NewBlockID("names.idx", 2)
	page := &Page{transaction: transaction, block: block, layout: layout}
	for slot, entry := range []struct {
		key   string
		child int32
	}{{"apple", 11}, {"banana", 21}, {"banana", 22}, {"pear", 41}} {
		transaction.strings[pageInsertDirPosition(layout, slot, "dataval")] = entry.key
		transaction.ints[pageInsertDirPosition(layout, slot, "block")] = entry.child
	}

	for _, test := range []struct {
		key  string
		want int
	}{{"apple", 11}, {"banana", 21}, {"peach", 22}, {"zebra", 41}} {
		got, err := page.FindChildBlock(query.NewStringConstant(test.key))
		if err != nil || got != storage.NewBlockID(block.FileName(), test.want) {
			t.Errorf("FindChildBlock(%q) = (%v, %v), want child %d", test.key, got, err, test.want)
		}
	}
}

func TestPageFindChildBlockRejectsInvalidDirectoryPages(t *testing.T) {
	block := storage.NewBlockID("directory.idx", 1)
	transaction := pageInsertDirTransactionWithCount(pageInsertDirLayout(record.IntegerField), 1)
	layout := pageInsertDirLayout(record.IntegerField)
	var typedNil *pageInsertDirTransaction
	var emptySchema record.Schema
	var datavalOnly record.Schema
	datavalOnly.AddIntegerField("dataval")
	var blockOnly record.Schema
	blockOnly.AddIntegerField("block")
	var varcharBlockSchema record.Schema
	varcharBlockSchema.AddIntegerField("dataval")
	varcharBlockSchema.AddVarcharField("block", 8)
	var unsupportedBlockSchema record.Schema
	unsupportedBlockSchema.AddIntegerField("dataval")
	unsupportedBlockSchema.AddField("block", record.FieldType(99), 1)
	var unsupportedKeySchema record.Schema
	unsupportedKeySchema.AddField("dataval", record.FieldType(99), 1)
	unsupportedKeySchema.AddIntegerField("block")

	cases := []struct {
		name string
		page *Page
		key  query.Constant
	}{
		{name: "nil page", key: query.NewIntConstant(10)},
		{name: "closed page", page: &Page{transaction: transaction, layout: layout}, key: query.NewIntConstant(10)},
		{name: "nil transaction", page: &Page{block: block, layout: layout}, key: query.NewIntConstant(10)},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: block, layout: layout}, key: query.NewIntConstant(10)},
		{name: "nil layout", page: &Page{transaction: transaction, block: block}, key: query.NewIntConstant(10)},
		{name: "nil schema", page: &Page{transaction: transaction, block: block, layout: record.NewLayoutFromMetadata(nil, nil, 8)}, key: query.NewIntConstant(10)},
		{name: "missing dataval", page: &Page{transaction: transaction, block: block, layout: record.NewLayout(&emptySchema)}, key: query.NewIntConstant(10)},
		{name: "missing block", page: &Page{transaction: transaction, block: block, layout: record.NewLayout(&datavalOnly)}, key: query.NewIntConstant(10)},
		{name: "missing dataval offset", page: &Page{transaction: transaction, block: block, layout: record.NewLayoutFromMetadata(pageInsertDirSchema(record.IntegerField, record.IntegerField), map[string]int{"block": 0}, 8)}, key: query.NewIntConstant(10)},
		{name: "missing block offset", page: &Page{transaction: transaction, block: block, layout: record.NewLayoutFromMetadata(pageInsertDirSchema(record.IntegerField, record.IntegerField), map[string]int{"dataval": 0}, 8)}, key: query.NewIntConstant(10)},
		{name: "varchar block field", page: &Page{transaction: transaction, block: block, layout: record.NewLayout(&varcharBlockSchema)}, key: query.NewIntConstant(10)},
		{name: "unsupported block field", page: &Page{transaction: transaction, block: block, layout: record.NewLayout(&unsupportedBlockSchema)}, key: query.NewIntConstant(10)},
		{name: "unsupported dataval field", page: &Page{transaction: transaction, block: block, layout: record.NewLayout(&unsupportedKeySchema)}, key: query.NewIntConstant(10)},
		{name: "duplicate field", page: &Page{transaction: transaction, block: block, layout: pageInsertDirLayoutWithDuplicateField()}, key: query.NewIntConstant(10)},
		{name: "unset key", page: &Page{transaction: transaction, block: block, layout: layout}},
		{name: "wrong key type", page: &Page{transaction: transaction, block: block, layout: layout}, key: query.NewStringConstant("10")},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.FindChildBlock(test.key); err == nil {
				t.Fatal("FindChildBlock() error = nil, want validation error")
			}
		})
	}
	var stringSchema record.Schema
	stringSchema.AddVarcharField("dataval", 8)
	stringSchema.AddIntegerField("block")
	if _, err := (&Page{transaction: transaction, block: block, layout: record.NewLayout(&stringSchema)}).FindChildBlock(query.NewIntConstant(10)); err == nil {
		t.Fatal("FindChildBlock() with integer search key and varchar dataval returned no error")
	}
}

func TestPageFindChildBlockRejectsEmptyBelowMinimumAndInvalidChildren(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	block := storage.NewBlockID("directory.idx", 3)
	for _, test := range []struct {
		name  string
		count int32
		key   int32
		value int32
	}{
		{name: "empty directory", count: 0, key: 10},
		{name: "negative record count", count: -1, key: 10},
		{name: "below minimum", count: 1, key: 9, value: 10},
		{name: "negative child block", count: 1, key: 10, value: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := pageInsertDirTransactionWithCount(layout, test.count)
			transaction.ints[pageInsertDirPosition(layout, 0, "dataval")] = test.value
			transaction.ints[pageInsertDirPosition(layout, 0, "block")] = test.value
			page := &Page{transaction: transaction, block: block, layout: layout}
			if _, err := page.FindChildBlock(query.NewIntConstant(test.key)); err == nil {
				t.Fatal("FindChildBlock() error = nil, want directory validation error")
			}
		})
	}
}

func TestPageFindChildBlockWrapsDirectoryReadErrors(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	block := storage.NewBlockID("directory.idx", 0)
	for _, test := range []struct {
		name   string
		offset int
	}{
		{name: "record count", offset: 4},
		{name: "directory key", offset: pageInsertDirPosition(layout, 0, "dataval")},
		{name: "child block", offset: pageInsertDirPosition(layout, 0, "block")},
	} {
		t.Run(test.name, func(t *testing.T) {
			cause := errors.New("directory read failed")
			transaction := pageInsertDirTransactionWithCount(layout, 1)
			transaction.ints[pageInsertDirPosition(layout, 0, "dataval")] = 10
			transaction.ints[pageInsertDirPosition(layout, 0, "block")] = 20
			transaction.readErrors[test.offset] = cause
			page := &Page{transaction: transaction, block: block, layout: layout}
			if _, err := page.FindChildBlock(query.NewIntConstant(10)); !errors.Is(err, cause) {
				t.Fatalf("FindChildBlock() error = %v, want wrapped cause", err)
			}
		})
	}
}

func TestPageFindChildBlockWrapsCandidateKeyReadFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		layout *record.Layout
		key    query.Constant
		setKey func(*pageInsertDirTransaction, *record.Layout)
		offset func(*record.Layout) int
		string bool
	}{
		{
			name:   "integer candidate",
			layout: pageInsertDirLayout(record.IntegerField),
			key:    query.NewIntConstant(20),
			setKey: func(tx *pageInsertDirTransaction, layout *record.Layout) {
				tx.ints[pageInsertDirPosition(layout, 0, "dataval")] = 10
				tx.ints[pageInsertDirPosition(layout, 1, "dataval")] = 20
				tx.ints[pageInsertDirPosition(layout, 1, "block")] = 200
			},
			offset: func(layout *record.Layout) int { return pageInsertDirPosition(layout, 1, "dataval") },
		},
		{
			name:   "varchar candidate",
			layout: pageInsertDirLayout(record.VarcharField),
			key:    query.NewStringConstant("banana"),
			setKey: func(tx *pageInsertDirTransaction, layout *record.Layout) {
				tx.strings[pageInsertDirPosition(layout, 0, "dataval")] = "apple"
				tx.strings[pageInsertDirPosition(layout, 1, "dataval")] = "banana"
				tx.ints[pageInsertDirPosition(layout, 1, "block")] = 200
			},
			offset: func(layout *record.Layout) int { return pageInsertDirPosition(layout, 1, "dataval") },
			string: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := pageInsertDirTransactionWithCount(test.layout, 2)
			test.setKey(base, test.layout)
			cause := errors.New("candidate key read failed")
			transaction := &pageFindChildReadFailure{
				pageInsertDirTransaction: base,
				intOffset:                test.offset(test.layout),
				intFailureNumber:         2,
				stringOffset:             test.offset(test.layout),
				stringFailureNumber:      2,
				cause:                    cause,
			}
			if test.string {
				transaction.intFailureNumber = 0
			} else {
				transaction.stringFailureNumber = 0
			}
			page := &Page{transaction: transaction, block: storage.NewBlockID("directory.idx", 0), layout: test.layout}
			if _, err := page.FindChildBlock(test.key); !errors.Is(err, cause) {
				t.Fatalf("FindChildBlock() error = %v, want wrapped candidate-key cause", err)
			}
		})
	}
}
