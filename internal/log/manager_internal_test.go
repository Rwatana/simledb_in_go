package log

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type readFailureFileManager struct {
	*storagefile.Manager
	err error
}

type shortPageFileManager struct {
	*storagefile.Manager
	page *storage.Page
}

type lengthFailureFileManager struct {
	*storagefile.Manager
	err error
}

type writeFailureFileManager struct {
	*storagefile.Manager
	err error
}

type writeOnNthFailureFileManager struct {
	*storagefile.Manager
	writeCalls int
	failAt     int
	err        error
}

func (f readFailureFileManager) Read(storage.BlockID) (*storage.Page, error) {
	return nil, f.err
}

func (f shortPageFileManager) Read(storage.BlockID) (*storage.Page, error) {
	return f.page, nil
}

func (f lengthFailureFileManager) Length(string) (int, error) {
	return 0, f.err
}

func (f writeFailureFileManager) Write(storage.BlockID, *storage.Page) error {
	return f.err
}

func (f *writeOnNthFailureFileManager) Write(block storage.BlockID, page *storage.Page) error {
	f.writeCalls++
	if f.writeCalls == f.failAt {
		return f.err
	}
	return f.Manager.Write(block, page)
}

func TestNewManagerPreservesTailBlockReadFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	_, files := newLogManager(t, 32)
	readErr := errors.New("tail read failed")

	manager, err := newManager(readFailureFileManager{Manager: files, err: readErr}, fileName)
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	if err == nil || !errors.Is(err, readErr) {
		t.Fatalf("newManager() error = %v, want wrapped tail read failure", err)
	}
	for _, context := range []string{"create log manager for \"simpledb.log\"", "read tail block [file simpledb.log, block 0]"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("newManager() error = %q, want context %q", err, context)
		}
	}
}

func TestNewManagerPreservesTailBoundaryReadFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	_, files := newLogManager(t, 32)
	shortPage, err := storage.NewPage(3)
	if err != nil {
		t.Fatalf("storage.NewPage() error = %v", err)
	}

	manager, err := newManager(shortPageFileManager{Manager: files, page: shortPage}, fileName)
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	for _, context := range []string{
		"create log manager for \"simpledb.log\"",
		"read tail boundary",
		"get integer at offset 0",
		"byte range [0:4] exceeds page capacity 3",
	} {
		if err == nil || !strings.Contains(err.Error(), context) {
			t.Errorf("newManager() error = %v, want context %q", err, context)
		}
	}
}

func TestNewManagerPreservesFileLengthFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	_, files := newLogManager(t, 32)
	lengthErr := errors.New("file length failed")

	manager, err := newManager(lengthFailureFileManager{Manager: files, err: lengthErr}, fileName)
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	if err == nil || !errors.Is(err, lengthErr) {
		t.Fatalf("newManager() error = %v, want wrapped file-length error", err)
	}
	for _, context := range []string{"create log manager for \"simpledb.log\"", "file length failed"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("newManager() error = %q, want context %q", err, context)
		}
	}
}

func TestNewManagerPreservesInitialBlockWriteFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	directory := filepath.Join(t.TempDir(), "database")
	files, err := storagefile.NewManager(directory, 32)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	writeErr := errors.New("initial block write failed")

	manager, err := newManager(writeFailureFileManager{Manager: files, err: writeErr}, fileName)
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	if err == nil || !errors.Is(err, writeErr) {
		t.Fatalf("newManager() error = %v, want wrapped initial write failure", err)
	}
	for _, context := range []string{"create log manager for \"simpledb.log\"", "write first block"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("newManager() error = %q, want context %q", err, context)
		}
	}
	if length, err := files.Length(fileName); err != nil || length != 1 {
		t.Errorf("file length after append/write failure = (%d, %v), want (1, nil)", length, err)
	}
}

func TestNewManagerPreservesInitialTailPageCreationFailure(t *testing.T) {
	t.Parallel()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), 32)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	pageErr := errors.New("tail page allocation failed")
	manager, err := newManagerWithPageFactory(files, "simpledb.log", func(int) (*storage.Page, error) {
		return nil, pageErr
	})
	if manager != nil {
		t.Errorf("newManagerWithPageFactory() manager = %v, want nil", manager)
	}
	if !errors.Is(err, pageErr) || !strings.Contains(err.Error(), "create tail page") {
		t.Errorf("newManagerWithPageFactory() error = %v, want wrapped tail page allocation failure", err)
	}
	if length, lengthErr := files.Length("simpledb.log"); lengthErr != nil || length != 0 {
		t.Errorf("log file length after page creation failure = (%d, %v), want (0, nil)", length, lengthErr)
	}
}

func TestNewManagerPreservesInitialTailBoundaryInitializationFailure(t *testing.T) {
	t.Parallel()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), 32)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	manager, err := newManagerWithPageFactory(files, "simpledb.log", func(int) (*storage.Page, error) {
		return storage.NewPage(3)
	})
	if manager != nil {
		t.Errorf("newManagerWithPageFactory() manager = %v, want nil", manager)
	}
	for _, context := range []string{"initialize boundary", "byte range [0:4] exceeds page capacity 3"} {
		if err == nil || !strings.Contains(err.Error(), context) {
			t.Errorf("newManagerWithPageFactory() error = %v, want context %q", err, context)
		}
	}
	if length, lengthErr := files.Length("simpledb.log"); lengthErr != nil || length != 0 {
		t.Errorf("log file length after boundary initialization failure = (%d, %v), want (0, nil)", length, lengthErr)
	}
}

func TestAppendPreservesRolloverInitializedBlockWriteFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	manager, files := newLogManager(t, 20)
	writeErr := errors.New("initialized tail write failed")
	fileManager := &writeOnNthFailureFileManager{Manager: files, failAt: 2, err: writeErr}
	manager.files = fileManager

	firstLSN, err := manager.Append([]byte("123456789012"))
	if err != nil || firstLSN != 1 {
		t.Fatalf("Append(full tail) = (%d, %v), want (1, nil)", firstLSN, err)
	}
	beforePage := manager.page.Snapshot()
	beforeBlock := manager.block

	lsn, err := manager.Append([]byte("x"))
	if lsn != 0 {
		t.Errorf("Append(rollover) LSN = %d, want 0 after initialized block write failure", lsn)
	}
	if !errors.Is(err, writeErr) {
		t.Fatalf("Append(rollover) error = %v, want wrapped initialized block write failure", err)
	}
	for _, context := range []string{
		"append log record of 1 bytes",
		"roll over full tail [file simpledb.log, block 0]",
		"write initialized tail block [file simpledb.log, block 1]",
	} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Append(rollover) error = %q, want context %q", err, context)
		}
	}
	if fileManager.writeCalls != 2 {
		t.Errorf("file writes = %d, want old-tail flush and failed initialized-tail write", fileManager.writeCalls)
	}
	if got := manager.page.Snapshot(); !bytes.Equal(got, beforePage) {
		t.Errorf("tail Page changed after failed rollover: got %v, want %v", got, beforePage)
	}
	if manager.block != beforeBlock {
		t.Errorf("tail block = %v, want unchanged %v", manager.block, beforeBlock)
	}
	if manager.latestLSN != firstLSN || manager.lastSavedLSN != firstLSN {
		t.Errorf("LSNs after failed rollover = (latest %d, saved %d), want (%d, %d)", manager.latestLSN, manager.lastSavedLSN, firstLSN, firstLSN)
	}
}
