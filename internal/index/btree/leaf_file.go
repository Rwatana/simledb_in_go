package btree

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type leafFileTransaction interface {
	PageTransaction
	Size(string) (int, error)
	Append(string) (storage.BlockID, error)
}

func ensureLeafFile(transaction leafFileTransaction, fileName string, layout *record.Layout) error {
	if isNilPageTransaction(transaction) {
		return fmt.Errorf("ensure B-tree leaf file %q: transaction is nil", fileName)
	}
	if strings.TrimSpace(fileName) == "" {
		return fmt.Errorf("ensure B-tree leaf file: file name is empty")
	}
	if layout == nil {
		return fmt.Errorf("ensure B-tree leaf file %q: layout is nil", fileName)
	}
	if _, err := leafPageFields(layout); err != nil {
		return fmt.Errorf("ensure B-tree leaf file %q: invalid layout: %w", fileName, err)
	}
	size, err := transaction.Size(fileName)
	if err != nil {
		return fmt.Errorf("ensure B-tree leaf file %q: read size: %w", fileName, err)
	}
	if size < 0 {
		return fmt.Errorf("ensure B-tree leaf file %q: invalid file size %d", fileName, size)
	}
	if size > 0 {
		return nil
	}
	block, err := transaction.Append(fileName)
	if err != nil {
		return fmt.Errorf("ensure B-tree leaf file %q: append first block: %w", fileName, err)
	}
	if block.FileName() != fileName || block.Number() != 0 {
		return fmt.Errorf("ensure B-tree leaf file %q: append returned block %v, want block zero in that file", fileName, block)
	}
	page, err := NewPage(transaction, block, layout)
	if err != nil {
		return fmt.Errorf("ensure B-tree leaf file %q: open first page: %w", fileName, err)
	}
	initializeErr := page.InitializeLeafPage()
	closeErr := page.Close()
	if initializeErr != nil && closeErr != nil {
		return fmt.Errorf("ensure B-tree leaf file %q: initialize first page: %w", fileName, errors.Join(initializeErr, fmt.Errorf("close first page: %w", closeErr)))
	}
	if initializeErr != nil {
		return fmt.Errorf("ensure B-tree leaf file %q: initialize first page: %w", fileName, initializeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("ensure B-tree leaf file %q: close first page: %w", fileName, closeErr)
	}
	return nil
}

func leafPageFields(layout *record.Layout) ([]btreePageField, error) {
	if layout == nil {
		return nil, fmt.Errorf("layout is nil")
	}
	fields, err := btreeRecordFields(layout)
	if err != nil {
		return nil, fmt.Errorf("layout metadata: %w", err)
	}
	if len(fields) != 3 {
		return nil, fmt.Errorf("layout must contain dataval, block, and id fields")
	}
	datavalFound, blockFound, idFound := false, false, false
	for _, field := range fields {
		switch field.name {
		case "dataval":
			datavalFound = true
		case "block":
			blockFound = true
			if field.typeOf != record.IntegerField {
				return nil, fmt.Errorf("block field is not an integer")
			}
		case "id":
			idFound = true
			if field.typeOf != record.IntegerField {
				return nil, fmt.Errorf("id field is not an integer")
			}
		}
	}
	if !datavalFound || !blockFound || !idFound {
		return nil, fmt.Errorf("layout requires dataval, block, and id fields")
	}
	return fields, nil
}
