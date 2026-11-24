package btree

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func ensureDirectoryFile(transaction btreeFileTransaction, fileName string, layout *record.Layout) error {
	if isNilPageTransaction(transaction) {
		return fmt.Errorf("ensure B-tree directory file %q: transaction is nil", fileName)
	}
	if strings.TrimSpace(fileName) == "" {
		return fmt.Errorf("ensure B-tree directory file: file name is empty")
	}
	keyType, err := directoryPageKeyType(layout)
	if err != nil {
		return fmt.Errorf("ensure B-tree directory file %q: invalid layout: %w", fileName, err)
	}
	size, err := transaction.Size(fileName)
	if err != nil {
		return fmt.Errorf("ensure B-tree directory file %q: read size: %w", fileName, err)
	}
	if size < 0 {
		return fmt.Errorf("ensure B-tree directory file %q: invalid file size %d", fileName, size)
	}
	if size > 0 {
		return nil
	}
	block, err := transaction.Append(fileName)
	if err != nil {
		return fmt.Errorf("ensure B-tree directory file %q: append root block: %w", fileName, err)
	}
	if block.FileName() != fileName || block.Number() != 0 {
		return fmt.Errorf("ensure B-tree directory file %q: append returned block %v, want block zero in that file", fileName, block)
	}
	page, err := NewPage(transaction, block, layout)
	if err != nil {
		return fmt.Errorf("ensure B-tree directory file %q: open root page: %w", fileName, err)
	}
	minimumKey := query.NewStringConstant("")
	if keyType == record.IntegerField {
		minimumKey = query.NewIntConstant(math.MinInt32)
	}
	initializeErr := page.InitializeDirectoryRoot(minimumKey)
	closeErr := page.Close()
	if initializeErr != nil && closeErr != nil {
		return fmt.Errorf("ensure B-tree directory file %q: initialize root: %w", fileName, errors.Join(initializeErr, fmt.Errorf("close root page: %w", closeErr)))
	}
	if initializeErr != nil {
		return fmt.Errorf("ensure B-tree directory file %q: initialize root: %w", fileName, initializeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("ensure B-tree directory file %q: close root page: %w", fileName, closeErr)
	}
	return nil
}

func directoryPageKeyType(layout *record.Layout) (record.FieldType, error) {
	if layout == nil {
		return 0, fmt.Errorf("layout is nil")
	}
	fields, err := btreeRecordFields(layout)
	if err != nil {
		return 0, fmt.Errorf("layout metadata: %w", err)
	}
	if len(fields) != 2 {
		return 0, fmt.Errorf("layout must contain exactly dataval and block fields")
	}
	var datavalType record.FieldType
	datavalFound, blockFound := false, false
	for _, field := range fields {
		switch field.name {
		case "dataval":
			datavalType, datavalFound = field.typeOf, true
		case "block":
			blockFound = true
			if field.typeOf != record.IntegerField {
				return 0, fmt.Errorf("block field is not an integer")
			}
		}
	}
	if !datavalFound || !blockFound {
		return 0, fmt.Errorf("layout requires dataval and block fields")
	}
	return datavalType, nil
}
