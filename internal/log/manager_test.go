package log

import (
	"bytes"
	"errors"
	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	"github.com/Rwatana/simledb_in_go/internal/storage"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNewManagerInitializesTheFirstLogBlock(t *testing.T) {
	t.Parallel()

	const blockSize = 32
	files := newFileManager(t, blockSize)
	const logFile = "simpledb.log"

	if _, err := NewManager(files, logFile); err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	length, err := files.Length(logFile)
	if err != nil {
		t.Fatalf("Length() error = %v", err)
	}
	if length != 1 {
		t.Fatalf("Length() = %d, want 1", length)
	}

	page, err := files.Read(storage.NewBlockID(logFile, 0))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	boundary, err := page.GetInt(0)
	if err != nil {
		t.Fatalf("GetInt(0) error = %v", err)
	}
	if boundary != blockSize {
		t.Errorf("initial boundary = %d, want %d", boundary, blockSize)
	}
	if got, want := page.Snapshot()[4:], make([]byte, blockSize-4); !bytes.Equal(got, want) {
		t.Errorf("bytes after boundary = %v, want zero-filled bytes", got)
	}
}

func TestNewManagerLoadsExistingTailWithoutAppending(t *testing.T) {
	t.Parallel()

	const blockSize = 32
	files := newFileManager(t, blockSize)
	const logFile = "simpledb.log"
	if _, err := NewManager(files, logFile); err != nil {
		t.Fatalf("first NewManager() error = %v", err)
	}
	before, err := files.Read(storage.NewBlockID(logFile, 0))
	if err != nil {
		t.Fatalf("Read() before reopen error = %v", err)
	}

	if _, err := NewManager(files, logFile); err != nil {
		t.Fatalf("second NewManager() error = %v", err)
	}
	length, err := files.Length(logFile)
	if err != nil {
		t.Fatalf("Length() after reopen error = %v", err)
	}
	if length != 1 {
		t.Fatalf("Length() after reopen = %d, want 1", length)
	}
	after, err := files.Read(storage.NewBlockID(logFile, 0))
	if err != nil {
		t.Fatalf("Read() after reopen error = %v", err)
	}
	if !bytes.Equal(after.Snapshot(), before.Snapshot()) {
		t.Error("reopening LogManager changed the existing tail block")
	}
}

func TestNewManagerRejectsInvalidInitialization(t *testing.T) {
	t.Parallel()

	for _, blockSize := range []int{1, 2, 3} {
		files := newFileManager(t, blockSize)
		if _, err := NewManager(files, "simpledb.log"); err == nil {
			t.Errorf("NewManager() block size %d error = nil, want a boundary size error", blockSize)
		}
	}

	files := newFileManager(t, 32)
	if _, err := NewManager(files, "../escape.log"); err == nil {
		t.Error("NewManager() path traversal error = nil, want an invalid file name error")
	}
}

func TestNewManagerRejectsBlockSizeOutsideBoundaryCapacity(t *testing.T) {
	t.Parallel()

	if strconv.IntSize <= 32 {
		t.Skip("block size cannot exceed the four-byte boundary on 32-bit platforms")
	}
	blockSize := int64(math.MaxInt32) + 1
	directory := filepath.Join(t.TempDir(), "database")
	files, err := storagefile.NewManager(directory, int(blockSize))
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}

	manager, err := NewManager(files, "oversized.log")
	if manager != nil {
		t.Errorf("NewManager() manager = %v, want nil", manager)
	}
	if err == nil || !strings.Contains(err.Error(), "oversized.log") || !strings.Contains(err.Error(), "four-byte boundary capacity") {
		t.Fatalf("NewManager() error = %v, want file-specific boundary-capacity error", err)
	}
	if _, statErr := os.Stat(filepath.Join(directory, "oversized.log")); !os.IsNotExist(statErr) {
		t.Errorf("oversized log file stat error = %v, want no file created", statErr)
	}
}

func TestNewManagerRejectsNilFileManager(t *testing.T) {
	t.Parallel()

	manager, err := NewManager(nil, "simpledb.log")
	if manager != nil {
		t.Errorf("NewManager() manager = %v, want nil", manager)
	}
	if err == nil || !strings.Contains(err.Error(), `"simpledb.log"`) || !strings.Contains(err.Error(), "file manager must not be nil") {
		t.Errorf("NewManager() error = %v, want file-specific nil dependency context", err)
	}
}

func TestNewManagerRejectsInvalidPersistedTailBoundary(t *testing.T) {
	t.Parallel()

	const blockSize = 32
	for _, boundary := range []int32{3, blockSize + 1} {
		boundary := boundary
		t.Run(strconv.FormatInt(int64(boundary), 10), func(t *testing.T) {
			t.Parallel()

			files := newFileManager(t, blockSize)
			const logFile = "simpledb.log"
			if _, err := NewManager(files, logFile); err != nil {
				t.Fatalf("initial NewManager() error = %v", err)
			}
			block := storage.NewBlockID(logFile, 0)
			page, err := files.Read(block)
			if err != nil {
				t.Fatalf("files.Read() error = %v", err)
			}
			if err := page.SetInt(0, boundary); err != nil {
				t.Fatalf("Page.SetInt() error = %v", err)
			}
			if err := files.Write(block, page); err != nil {
				t.Fatalf("files.Write() error = %v", err)
			}

			manager, err := NewManager(files, logFile)
			if manager != nil {
				t.Errorf("NewManager() manager = %v, want nil for invalid tail boundary", manager)
			}
			want := "tail boundary " + strconv.FormatInt(int64(boundary), 10) + " outside [4, 32]"
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("NewManager() error = %v, want %q context", err, want)
			}
		})
	}
}

func TestNewManagerPreservesInitialBlockAppendFailure(t *testing.T) {
	t.Parallel()

	files := newFileManager(t, 16)
	appendErr := errors.New("initial block append failed")
	manager, err := newManager(appendFailureFileManager{Manager: files, err: appendErr}, "simpledb.log")
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	if err == nil || !errors.Is(err, appendErr) || !strings.Contains(err.Error(), "append first block") {
		t.Fatalf("newManager() error = %v, want wrapped initial append failure", err)
	}
}

func TestNewManagerPreservesFileLengthFailure(t *testing.T) {
	t.Parallel()

	files := newFileManager(t, 32)
	lengthErr := errors.New("file length failed")
	manager, err := newManager(lengthFailureFileManager{Manager: files, err: lengthErr}, "simpledb.log")
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	if err == nil || !errors.Is(err, lengthErr) || !strings.Contains(err.Error(), "create log manager for \"simpledb.log\"") {
		t.Errorf("newManager() error = %v, want wrapped file-length failure", err)
	}
}

func TestNewManagerPreservesTailBlockReadFailure(t *testing.T) {
	t.Parallel()

	files := newFileManager(t, 32)
	if _, err := NewManager(files, "simpledb.log"); err != nil {
		t.Fatalf("initial NewManager() error = %v", err)
	}
	readErr := errors.New("tail read failed")
	manager, err := newManager(readFailureFileManager{Manager: files, err: readErr}, "simpledb.log")
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

	files := newFileManager(t, 32)
	if _, err := NewManager(files, "simpledb.log"); err != nil {
		t.Fatalf("initial NewManager() error = %v", err)
	}
	shortPage, err := storage.NewPage(3)
	if err != nil {
		t.Fatalf("storage.NewPage() error = %v", err)
	}
	manager, err := newManager(shortPageFileManager{Manager: files, page: shortPage}, "simpledb.log")
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

func TestNewManagerPreservesInitialBlockWriteFailure(t *testing.T) {
	t.Parallel()

	files := newFileManager(t, 32)
	writeErr := errors.New("initial block write failed")
	manager, err := newManager(writeFailureFileManager{Manager: files, err: writeErr}, "simpledb.log")
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	if err == nil || !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "write first block") {
		t.Fatalf("newManager() error = %v, want wrapped initial write failure", err)
	}
	if length, err := files.Length("simpledb.log"); err != nil || length != 1 {
		t.Errorf("file length after append/write failure = (%d, %v), want (1, nil)", length, err)
	}
}

func TestNewManagerPreservesInitialTailPageCreationFailure(t *testing.T) {
	t.Parallel()

	files := newFileManager(t, 32)
	pageErr := errors.New("tail page allocation failed")
	manager, err := newManagerWithPageFactory(files, "simpledb.log", func(int) (*storage.Page, error) {
		return nil, pageErr
	})
	if manager != nil {
		t.Errorf("newManagerWithPageFactory() manager = %v, want nil", manager)
	}
	if !errors.Is(err, pageErr) || !strings.Contains(err.Error(), "create tail page") {
		t.Errorf("newManagerWithPageFactory() error = %v, want wrapped page creation failure", err)
	}
	if length, lengthErr := files.Length("simpledb.log"); lengthErr != nil || length != 0 {
		t.Errorf("log file length after page creation failure = (%d, %v), want (0, nil)", length, lengthErr)
	}
}

func TestNewManagerPreservesInitialTailBoundaryInitializationFailure(t *testing.T) {
	t.Parallel()

	files := newFileManager(t, 32)
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

type readFailureFileManager struct {
	*storagefile.Manager
	err error
}

func (f readFailureFileManager) Read(storage.BlockID) (*storage.Page, error) {
	return nil, f.err
}

type shortPageFileManager struct {
	*storagefile.Manager
	page *storage.Page
}

func (f shortPageFileManager) Read(storage.BlockID) (*storage.Page, error) {
	return f.page, nil
}

type lengthFailureFileManager struct {
	*storagefile.Manager
	err error
}

func (f lengthFailureFileManager) Length(string) (int, error) { return 0, f.err }

type appendFailureFileManager struct {
	*storagefile.Manager
	err error
}

func (f appendFailureFileManager) Append(string) (storage.BlockID, error) {
	return storage.BlockID{}, f.err
}

type writeFailureFileManager struct {
	*storagefile.Manager
	err error
}

func (f writeFailureFileManager) Write(storage.BlockID, *storage.Page) error { return f.err }

func newFileManager(t *testing.T, blockSize int) *storagefile.Manager {
	t.Helper()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), blockSize)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	return files
}

func TestAppendRejectsSyntheticOversizedRecordBeforeStateChanges(t *testing.T) {
	t.Parallel()

	manager, _ := newLogManager(t, 32)
	before := manager.page.Snapshot()
	lsn, err := manager.appendWithMaxInt32([]byte("12345"), 8)
	if lsn != 0 {
		t.Errorf("appendWithMaxInt32() LSN = %d, want 0", lsn)
	}
	if err == nil || !strings.Contains(err.Error(), "append log record of 5 bytes: encoded length exceeds four-byte boundary capacity") {
		t.Errorf("appendWithMaxInt32() error = %v, want the four-byte boundary capacity error", err)
	}
	if got := manager.page.Snapshot(); !bytes.Equal(got, before) {
		t.Errorf("tail Page changed after rejected append: got %v, want %v", got, before)
	}
	if manager.latestLSN != 0 {
		t.Errorf("latest LSN = %d, want unchanged value 0", manager.latestLSN)
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

func TestAppendPreservesRolloverTailPageCreationFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	manager, files := newInternalLogManager(t, 20)
	firstLSN, err := manager.Append([]byte("123456789012"))
	if err != nil || firstLSN != 1 {
		t.Fatalf("Append(full tail) = (%d, %v), want (1, nil)", firstLSN, err)
	}
	beforePage := manager.page.Snapshot()
	beforeBlock := manager.block
	pageErr := errors.New("new tail page allocation failed")
	pageFactoryCalls := 0
	manager.newPage = func(int) (*storage.Page, error) {
		pageFactoryCalls++
		return nil, pageErr
	}

	lsn, err := manager.Append([]byte("x"))
	if lsn != 0 {
		t.Errorf("Append(rollover) LSN = %d, want 0 after new tail page failure", lsn)
	}
	if !errors.Is(err, pageErr) {
		t.Fatalf("Append(rollover) error = %v, want wrapped new tail page failure", err)
	}
	for _, context := range []string{"roll over full tail", "create new tail page"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Append(rollover) error = %q, want context %q", err, context)
		}
	}
	if pageFactoryCalls != 1 {
		t.Errorf("new-page factory calls = %d, want 1", pageFactoryCalls)
	}
	assertRolloverPageSetupFailureState(t, manager, files, beforePage, beforeBlock, firstLSN, fileName)
}

func TestAppendPreservesRolloverTailBoundaryInitializationFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	manager, files := newInternalLogManager(t, 20)
	firstLSN, err := manager.Append([]byte("123456789012"))
	if err != nil || firstLSN != 1 {
		t.Fatalf("Append(full tail) = (%d, %v), want (1, nil)", firstLSN, err)
	}
	beforePage := manager.page.Snapshot()
	beforeBlock := manager.block
	manager.newPage = func(int) (*storage.Page, error) {
		return storage.NewPage(3)
	}

	lsn, err := manager.Append([]byte("x"))
	if lsn != 0 {
		t.Errorf("Append(rollover) LSN = %d, want 0 after boundary initialization failure", lsn)
	}
	for _, context := range []string{"roll over full tail", "initialize new tail boundary", "page capacity 3"} {
		if err == nil || !strings.Contains(err.Error(), context) {
			t.Errorf("Append(rollover) error = %v, want context %q", err, context)
		}
	}
	assertRolloverPageSetupFailureState(t, manager, files, beforePage, beforeBlock, firstLSN, fileName)
}

func newInternalLogManager(t *testing.T, blockSize int) (*Manager, *storagefile.Manager) {
	t.Helper()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), blockSize)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	manager, err := NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager, files
}

func assertRolloverPageSetupFailureState(t *testing.T, manager *Manager, files *storagefile.Manager, beforePage []byte, beforeBlock storage.BlockID, firstLSN LSN, fileName string) {
	t.Helper()

	if got := manager.page.Snapshot(); !bytes.Equal(got, beforePage) {
		t.Errorf("tail Page changed after failed rollover: got %v, want %v", got, beforePage)
	}
	if manager.block != beforeBlock {
		t.Errorf("tail block = %v, want unchanged %v", manager.block, beforeBlock)
	}
	if manager.latestLSN != firstLSN || manager.lastSavedLSN != firstLSN {
		t.Errorf("LSNs after failed rollover = (latest %d, saved %d), want (%d, %d)", manager.latestLSN, manager.lastSavedLSN, firstLSN, firstLSN)
	}
	if length, err := files.Length(fileName); err != nil || length != 1 {
		t.Errorf("log file length after page setup failure = (%d, %v), want (1, nil)", length, err)
	}
}

func TestSetAppendBoundaryPreservesPageWriteFailure(t *testing.T) {
	t.Parallel()

	manager, _ := newInternalLogManager(t, 32)
	shortPage, err := storage.NewPage(3)
	if err != nil {
		t.Fatalf("storage.NewPage() error = %v", err)
	}
	manager.page = shortPage
	beforeLSN := manager.latestLSN

	lsn, err := manager.setAppendBoundary(4)
	if lsn != 0 {
		t.Errorf("setAppendBoundary() LSN = %d, want 0", lsn)
	}
	for _, context := range []string{"append log record to [file simpledb.log, block 0]", "update boundary to 4", "page capacity 3"} {
		if err == nil || !strings.Contains(err.Error(), context) {
			t.Errorf("setAppendBoundary() error = %v, want context %q", err, context)
		}
	}
	if manager.latestLSN != beforeLSN {
		t.Errorf("latest LSN = %d, want unchanged value %d", manager.latestLSN, beforeLSN)
	}
}
