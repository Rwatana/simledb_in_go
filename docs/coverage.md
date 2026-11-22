# Go test coverage

The `go-quality` job runs the full Go test suite with a coverage profile and writes total statement coverage to the
GitHub Actions job summary. This keeps the measurement visible on every pull request and main-branch push.

At the 2026-11-23 checkpoint for PR #573, the full-module statement coverage is **93.9%** (rounded to one
decimal place). Package coverage is buffer 100.0%, concurrency 92.9%, file 96.9%, log 82.4%, metadata 86.1%, query
100.0%, record 96.7%, recovery 97.4%, storage 100.0%, and transaction 91.3%. Recent focused tests closed existing
`Expression.Evaluate`, `Page.GetBytes`, `Page.SetBytes`, and metadata nil-schema, integer-range, scan-open, scan-close,
both catalog-initialization failure paths, table- and field-catalog row insertion, field-catalog table-name,
field-name, type, length, and offset writes, table slot-size write failures, table-catalog layout scan, registered-name,
slot-size read failures, invalid catalog slot-size rejection, field-catalog scan and field-owner/field-name read failures,
duplicate field metadata rejection, field-catalog type/length/offset read failures, invalid type rejection, and negative field-length rejection during layout lookup,
oversized table-layout rejection, out-of-range field-length and field-offset rejection, empty file-manager directory rejection, database directory creation failure, nil-page write rejection, block append and page write open failures, file length stat failure, append and page positioned-write failures on Unix, partial-block `ReadAt` EOF handling, 32-bit block-count conversion limits, and rejection of a zero-value buffer FileManager. A focused internal page-factory test also verifies that Page allocation failures retain both their cause and Buffer-construction context. A platform-aware test checks that log initialization rejects a block size above the int32 boundary before creating a log file. A separate constructor test verifies nil FileManager rejection and its file-name context. Corrupted tail-boundary tests cover values below the four-byte minimum and above the block capacity. A Darwin/Linux FIFO test exercises initial log-block append failure and checks both the wrapped `ESPIPE` cause and file context. A constructor guard test verifies `transaction.New` rejects multiple recovery iterator factories before dependency access or START logging. A statistics test exercises the threshold-triggered refresh failure on lookup 101 and verifies both lookup and refresh context are retained without caching a partial result. The partial-read assertion verifies the wrapped `io.EOF`, block and offset context, and a `15 of 16 bytes` count. Because `os.File.ReadAt` follows the `io.ReaderAt` contract and reports a non-nil error for every short read, the separate short-count fallback after `err == nil` was unreachable; removing it makes `file.Manager.Read` 100.0% covered without changing the `ReadAt` error behavior. A Unix FIFO test also exercises the page `WriteAt` failure path while preserving `ESPIPE` and the zero-byte count. The short-count fallbacks after nil-error `os.File.WriteAt` results in both block `Write` and `Append` were likewise unreachable under the `io.WriterAt` contract; they were removed while the wrapped write-error paths remain covered. The pure block-count conversion helper is tested with a synthetic 32-bit limit, making both `blocksForSize` and that helper fully covered on the 64-bit host without large files. `buffer.New` and its internal Page factory path are now 100.0% covered. A zero-value buffer FileManager makes `NewManager`’s first-frame error path deterministic; its focused test brings the buffer package to 100.0%. The module remains at 93.9% while the remaining package gaps are audited.
The audit also found and fixed a real metadata integrity defect: negative varchar lengths are now rejected before any
catalog row is written. Remaining
uncovered statements are being inspected by package and behavior rather than hidden behind a premature gate.

The current-block iterator boundary test also covers the error returned when an in-memory log-page boundary is outside
the valid range, while confirming no iterator is returned and the durable tail is unchanged. The CI step reports
coverage without enforcing a threshold while the repository is below 100%. Add a required 100%
threshold only after the complete suite measures 100.0%; otherwise CI would be permanently red before the target is met.

For a local full-module measurement, run:

```sh
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1
```

Inspect individual uncovered functions with:

```sh
go tool cover -func=coverage.out
```
