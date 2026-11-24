package btree

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func directoryLayoutFromLeaf(leafLayout *record.Layout) (*record.Layout, error) {
	if leafLayout == nil {
		return nil, fmt.Errorf("derive B-tree directory layout: leaf layout is nil")
	}
	if _, err := leafPageFields(leafLayout); err != nil {
		return nil, fmt.Errorf("derive B-tree directory layout: invalid leaf layout: %w", err)
	}
	dataval, _ := leafLayout.Schema().Field("dataval")
	var directorySchema record.Schema
	directorySchema.AddIntegerField("block")
	directorySchema.AddField("dataval", dataval.Type, dataval.Length)
	return record.NewLayout(&directorySchema), nil
}
