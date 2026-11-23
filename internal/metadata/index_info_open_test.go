package metadata

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/index/hash"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexInfoOpenBuildsConfiguredStaticHashIndexWithoutOpeningBucket(t *testing.T) {
	tests := []struct {
		name      string
		addField  func(*record.Schema)
		searchKey query.Constant
	}{
		{name: "integer", addField: func(schema *record.Schema) { schema.AddIntegerField("key") }, searchKey: query.NewIntConstant(-7)},
		{name: "varchar", addField: func(schema *record.Schema) { schema.AddVarcharField("key", 24) }, searchKey: query.NewStringConstant("key")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction := newEmptyCatalogTransaction(512)
			var schema record.Schema
			test.addField(&schema)
			info, err := NewIndexInfo("student_key", "key", &schema, transaction, NewStatInfo(2, 6))
			if err != nil {
				t.Fatalf("NewIndexInfo() error = %v", err)
			}

			indexScan := info.Open()
			if indexScan == nil {
				t.Fatal("Open() returned a nil index")
			}
			if len(transaction.appendCalls) != 0 || len(transaction.pins) != 0 {
				t.Fatalf("Open() performed bucket I/O: appends=%v pins=%v", transaction.appendCalls, transaction.pins)
			}

			if err := indexScan.BeforeFirst(test.searchKey); err != nil {
				t.Fatalf("BeforeFirst() error = %v", err)
			}
			bucket, err := hash.BucketNumber(test.searchKey)
			if err != nil {
				t.Fatalf("BucketNumber(%v) error = %v", test.searchKey, err)
			}
			wantTable := fmt.Sprintf("student_key%d.tbl", bucket)
			if !reflect.DeepEqual(transaction.appendCalls, []string{wantTable}) {
				t.Fatalf("bucket tables created = %v, want [%s]", transaction.appendCalls, wantTable)
			}
			if err := indexScan.Insert(test.searchKey, record.NewRID(3, 4)); err != nil {
				t.Fatalf("Insert() through opened index error = %v", err)
			}
			if err := indexScan.BeforeFirst(test.searchKey); err != nil {
				t.Fatalf("reposition opened index error = %v", err)
			}
			found, err := indexScan.Next()
			if err != nil || !found {
				t.Fatalf("Next() = (%t, %v), want one stored entry", found, err)
			}
			gotRID, err := indexScan.GetDataRID()
			if err != nil || gotRID != record.NewRID(3, 4) {
				t.Fatalf("GetDataRID() = (%v, %v), want (3,4)", gotRID, err)
			}
			if err := indexScan.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})
	}
}
