# Go test coverage

The `go-quality` job runs the full Go test suite with a coverage profile and writes total statement coverage to the
GitHub Actions job summary. This keeps the measurement visible on every pull request and main-branch push.

At the 2026-11-23 checkpoint for PR #551, the full-module statement coverage is **93.3%** (rounded to one
decimal place). Package coverage is buffer 99.1%, concurrency 92.9%, file 94.9%, log 78.2%, metadata 85.6%, query
100.0%, record 96.7%, recovery 97.4%, storage 100.0%, and transaction 90.8%. Recent focused tests closed existing
`Expression.Evaluate`, `Page.GetBytes`, `Page.SetBytes`, and metadata nil-schema, integer-range, scan-open, scan-close,
both catalog-initialization failure paths, table- and field-catalog row insertion, field-catalog table-name,
field-name, type, length, and offset writes, table slot-size write failures, table-catalog layout scan, registered-name,
slot-size read failures, invalid catalog slot-size rejection, field-catalog scan and field-owner/field-name read failures,
duplicate field metadata rejection, field-catalog type/length/offset read failures, invalid type rejection, and negative field-length rejection during layout lookup,
oversized table-layout rejection, out-of-range field-length and field-offset rejection, empty file-manager directory rejection, database directory creation failure, nil-page write rejection, block append and page write open failures, file length stat failure, append and page positioned-write failures on Unix, and partial-block `ReadAt` EOF handling. The latter assertion verifies the wrapped `io.EOF`, block and offset context, and a `15 of 16 bytes` count. Because `os.File.ReadAt` follows the `io.ReaderAt` contract and reports a non-nil error for every short read, the separate short-count fallback after `err == nil` was unreachable; removing it makes `file.Manager.Read` 100.0% covered without changing the `ReadAt` error behavior. A Unix FIFO test also exercises the page `WriteAt` failure path while preserving `ESPIPE` and the zero-byte count. The short-count fallback after a nil-error `os.File.WriteAt` result was likewise unreachable under the `io.WriterAt` contract and has been removed; the wrapped write-error path remains covered.
The audit also found and fixed a real metadata integrity defect: negative varchar lengths are now rejected before any
catalog row is written. Remaining
uncovered statements are being inspected by package and behavior rather than hidden behind a premature gate.

The CI step reports coverage without enforcing a threshold while the repository is below 100%. Add a required 100%
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
