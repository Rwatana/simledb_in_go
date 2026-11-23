# Go test coverage

The `go-quality` job runs the full Go test suite with a coverage profile and writes total statement coverage to the
GitHub Actions job summary. This keeps the measurement visible on every pull request and main-branch push.

At the 2026-11-23 checkpoint for Issue #662, the full-module statement coverage is **96.7%** (rounded to one
decimal place). Package coverage is buffer 100.0%, concurrency 95.6%, file 96.9%, log 82.4%, metadata 97.6%, query
100.0%, record 96.7%, recovery 97.4%, storage 100.0%, and transaction 95.4%. Recent focused tests closed existing
`Expression.Evaluate`, `Page.GetBytes`, `Page.SetBytes`, and metadata nil-schema, integer-range, scan-open, scan-close,
both catalog-initialization failure paths, table- and field-catalog row insertion, field-catalog table-name,
field-name, type, length, and offset writes, table slot-size write failures, table-catalog layout scan, registered-name,
slot-size read failures, invalid catalog slot-size rejection, field-catalog scan and field-owner/field-name read failures,
duplicate field metadata rejection, field-catalog type/length/offset read failures, invalid type rejection, and negative field-length rejection during layout lookup,
oversized table-layout rejection, out-of-range field-length and field-offset rejection, empty file-manager directory rejection, database directory creation failure, nil-page write rejection, block append and page write open failures, file length stat failure, append and page positioned-write failures on Unix, partial-block `ReadAt` EOF handling, 32-bit block-count conversion limits, and rejection of a zero-value buffer FileManager. A focused internal page-factory test also verifies that Page allocation failures retain both their cause and Buffer-construction context. A platform-aware test checks that log initialization rejects a block size above the int32 boundary before creating a log file. A separate constructor test verifies nil FileManager rejection and its file-name context. Corrupted tail-boundary tests cover values below the four-byte minimum and above the block capacity. A Darwin/Linux FIFO test exercises initial log-block append failure and checks both the wrapped `ESPIPE` cause and file context. A constructor guard test verifies `transaction.New` rejects multiple recovery iterator factories before dependency access or START logging. A statistics test exercises the threshold-triggered refresh failure on lookup 101 and verifies both lookup and refresh context are retained without caching a partial result. The partial-read assertion verifies the wrapped `io.EOF`, block and offset context, and a `15 of 16 bytes` count. Because `os.File.ReadAt` follows the `io.ReaderAt` contract and reports a non-nil error for every short read, the separate short-count fallback after `err == nil` was unreachable; removing it makes `file.Manager.Read` 100.0% covered without changing the `ReadAt` error behavior. A Unix FIFO test also exercises the page `WriteAt` failure path while preserving `ESPIPE` and the zero-byte count. The short-count fallbacks after nil-error `os.File.WriteAt` results in both block `Write` and `Append` were likewise unreachable under the `io.WriterAt` contract; they were removed while the wrapped write-error paths remain covered. The pure block-count conversion helper is tested with a synthetic 32-bit limit, making both `blocksForSize` and that helper fully covered on the 64-bit host without large files. `buffer.New` and its internal Page factory path are now 100.0% covered. A zero-value buffer FileManager makes `NewManager`’s first-frame error path deterministic; its focused test brings the buffer package to 100.0%. The module remains at 94.3% while the remaining package gaps are audited.
The audit also found and fixed a real metadata integrity defect: negative varchar lengths are now rejected before any
catalog row is written. Remaining
uncovered statements are being inspected by package and behavior rather than hidden behind a premature gate.

The transaction lock-manager tests inject shared-wait and exclusive-upgrade errors through its narrow internal table
interface. They verify failed shared acquisition is not recorded, and a failed upgrade preserves only shared local
ownership; the public constructor still accepts the concrete `*LockTable`. `Manager.LockShared` and `Manager.LockExclusive` are fully covered. A view-catalog initialization failure test also
verifies the original unpin/close error and view-specific context are preserved. The overall module is 94.3% while
remaining paths are handled as separate observable behaviors. An exclusive request whose prerequisite shared acquisition
fails returns the cause without creating local ownership.

A focused view metadata test also covers the error returned when loading the table-catalog layout fails during view creation, preserving the cause and view-name context. Metadata coverage is 87.4%; the full module remains at 94.3%.

The metadata facade regression test also verifies that a view-catalog layout failure is wrapped with the requested view and facade context, and that no view row is appended after the failure. `MetadataManager.CreateView` is now 100.0% covered; metadata coverage is 87.7%, and full-module coverage is 94.4%.

A table-catalog initialization failure test confirms that the metadata facade preserves the underlying size error and stops bootstrap before appending any catalog block. `MetadataManager.InitializeCatalogs` coverage is now 71.4%; metadata coverage is 87.9%, while the full module remains at 94.4%.

A separate view-catalog bootstrap test fails the table-catalog lookup after the self-describing table and field catalogs are initialized. It verifies facade context, original-cause preservation, and that index-catalog registration is skipped. `MetadataManager.InitializeCatalogs` now has 85.7% function coverage; metadata coverage is 88.2%, and full-module coverage is 94.5%.

The index-catalog bootstrap error path is now covered as well: after tblcat, fldcat, and viewcat registration succeeds, an injected layout-size failure retains index-stage context and leaves the index catalog unregistered. `MetadataManager.InitializeCatalogs` is now 100.0% covered; metadata coverage is 88.7%, and full-module coverage is 94.6%.

The metadata facade's nil-schema table-creation validation path is covered without requiring a transaction. `MetadataManager.CreateTable` is now 100.0% covered; metadata coverage is 89.0%, while the full-module total remains 94.6%.

The metadata facade's index-registration dependency error is now covered as well. A missing table manager returns index-specific context before a transaction can be accessed, and `MetadataManager.CreateIndex` is now 100.0% covered. Metadata coverage is 89.2%; the full module is 94.7%.

The facade layout-lookup error branch is covered by injecting a table-catalog size failure after startup. The test confirms no layout is returned and the wrapped cause and table name survive. `MetadataManager.Layout` is now 100.0% covered; metadata coverage is 89.5% and full-module coverage remains 94.7%.

The facade view-definition lookup failure is covered by injecting a table-catalog size error after catalog initialization. The test confirms the empty not-found result, requested-view and facade context, and original cause. `MetadataManager.ViewDefinition` is now 100.0% covered; metadata coverage is 90.0% and full-module coverage is 94.8%.

The facade index lookup error is covered by injecting a table-catalog size failure after initialization. The test verifies a nil result, table-specific context at both index-manager and facade layers, and preservation of the original cause. `MetadataManager.IndexInfo` is now 100.0% covered; metadata coverage is 90.6% and full-module coverage is 94.9%.

The index registration path now also covers failure while loading the `idxcat` layout. The regression test verifies index-specific layout context, the original cause, and that no registration row is appended. `IndexManager.CreateIndex` is now 70.6% covered; metadata coverage is 90.8% and full-module coverage is 95.0%.

Index registration also verifies that a failure storing the first catalog name is wrapped with the index name and field-stage context. `IndexManager.CreateIndex` is now 82.4% covered; metadata coverage is 91.3% and full-module coverage is 95.1%.

The next index-catalog field write now has an independent failure-path test. It confirms that a table-name write failure preserves the original cause and `tablename` stage context; `IndexManager.CreateIndex` is now 88.2% covered, metadata coverage is 91.6%, and full-module coverage remains 95.1%.

The final index-catalog name write now has a focused error-path check. The cause, requested index, and `fieldname` stage are preserved; `IndexManager.CreateIndex` is now 94.1% covered, metadata coverage is 91.9%, and full-module coverage is 95.2%.

Index-catalog row insertion failure is now tested by failing the next slot-occupancy write after one successful registration. The test verifies no false success and preserves the cause and index-specific insertion context; `IndexManager.CreateIndex` is now 100.0% covered, metadata coverage is 92.1%, and full-module coverage is 95.2%.

The `IndexManager.IndexInfo` scan-advance error is now injected specifically on the `idxcat.tbl` occupancy read. The test verifies that the catalog scan cause and table/catalog context survive and that a partially built map is not returned. `IndexManager.IndexInfo` is now 84.2% covered, metadata coverage is 92.7%, and full-module coverage is 95.3%. The remaining branches in this function are being tested independently.

The `IndexManager.IndexInfo` indexed-table-name read failure is now isolated to the `tablename` field in `idxcat.tbl`; table-layout reconstruction runs before the injection. The test confirms that the original cause and field/table context survive without returning a partial map. `IndexManager.IndexInfo` is now 86.8% covered, metadata coverage is 92.9%, and full-module coverage is 95.4%.

The matching index-name read failure now has its own injection at the `indexname` offset in `idxcat.tbl`. The matching table-name read succeeds before failure, and the test verifies the table/field context, wrapped cause, and nil result map. `IndexManager.IndexInfo` is now 89.5% covered, metadata coverage is 93.2%, and full-module coverage remains 95.4%.

The matching indexed field-name read failure is covered independently after the table and index names have been read.
The test confirms the original cause and requested field/table context are preserved and no partial map escapes.
`IndexManager.IndexInfo` is now 92.1% covered, metadata coverage is 93.4%, and full-module coverage is 95.5%.

The indexed table-layout lookup failure is covered by making the second `tblcat.tbl` size request fail: the first
request reconstructs `idxcat`, and the second occurs only after its matching row has been read. The cause and table
context remain visible, and `IndexInfo` returns no partial map. Function coverage is now 94.7%, metadata coverage is
93.7%, and full-module coverage is 95.6%.

The index-statistics failure is now isolated by failing the first `student.tbl` size request. The index catalog row
and indexed table layout are read successfully before statistics calculation starts; the lookup preserves the cause
and table/statistics context and returns no partial map. `IndexManager.IndexInfo` is now 97.4% covered, metadata
coverage is 94.5%, and full-module coverage is 95.7%.

The final `IndexManager.IndexInfo` branch now has a focused check: an index row refers to a field absent from the
reconstructed table schema. The lookup returns the missing-field error with index/table context and no partial map.
`IndexManager.IndexInfo` is now 100.0% covered, metadata coverage is 94.8%, and full-module coverage is 95.8%.

`ViewManager.CreateView` now exercises its catalog view-name write failure independently. The test confirms the
wrapped cause and view context remain visible and that the scan releases its catalog block. `CreateView` is now
80.0% covered, metadata coverage is 95.3%, and full-module coverage is 95.9%.

The subsequent `viewdef` write failure path is tested after a successful view-name write. Its error keeps the cause
and requested-view context, and the scan releases its block. `ViewManager.CreateView` is now 86.7% covered, metadata
coverage is 95.5%, and full-module coverage is 95.9%.

The view-catalog initializer's missing table-manager guard now has a regression test proving rejection before
transaction access. `ViewManager.InitializeCatalog` is now 100.0% covered, metadata coverage is 95.8%, and
full-module coverage is 96.0%.

The `ViewDefinition` nil table-manager guard is covered independently and returns an empty result with the requested
view context before transaction access. `ViewDefinition` is now 84.0% covered, metadata coverage is 96.1%, and the
full-module statement coverage is 96.0%.

The `CreateView` nil table-manager guard now has a focused test that passes a nil transaction to prove the dependency
is rejected first. `CreateView` is now 93.3% covered, metadata coverage is 96.3%, and full-module coverage is 96.1%.

The catalog row-insertion failure path now has a focused error test that verifies cause and view context are retained,
the scan releases its catalog block, and field storage is not started. `ViewManager.CreateView` is now 100.0% covered,
metadata coverage is 96.6%, and full-module coverage is 96.1%.

The `ViewDefinition` scan-advance failure path now injects an occupancy read error only for `viewcat.tbl`. It returns an
empty result with the requested-view context and closes the scan. `ViewDefinition` is now 92.0% covered, metadata
coverage is 97.1%, and full-module coverage is 96.2%.

The separate `viewname` read failure in `ViewDefinition` now returns the cause and requested-view context, an empty
result, and closes the scan. `ViewDefinition` is now 96.0% covered, metadata coverage is 97.4%, and full-module
coverage is 96.3%.

The matching-row `viewdef` read failure in `ViewDefinition` now returns the wrapped cause and requested-view context,
an empty result, and closes the scan. `ViewDefinition` is now 100.0% covered, metadata coverage is 97.6%, and
full-module coverage is 96.3%.

`NewWithFileSizer` now has a test proving a nil file-size dependency is rejected before allocating a transaction ID or
writing START. The constructor is 85.7% covered, transaction coverage is 91.9%, and full-module coverage is 96.4%.

`NewWithFileManager` now has the same independent guard test: a nil file manager is rejected before allocating an ID
or writing START. The constructor is 85.7% covered, transaction coverage is 92.5%, and full-module coverage remains
96.4%.

`NewWithFileSizer` now verifies the constructor's START-append error path with a valid sizer. The returned transaction
is nil and both the append cause and transaction identifier context remain visible. The constructor is 100.0%
covered, transaction coverage is 93.1%, and full-module coverage is 96.5%.

`NewWithFileManager` now verifies that the same START-append error is preserved through the full-file-manager
constructor. It returns no transaction while retaining the cause and transaction ID context. The constructor is
100.0% covered, transaction coverage is 93.6%, and full-module coverage remains 96.5%.

`Transaction.Size` now tests its zero-value guard when no file sizer is configured, confirming it returns zero and
the existing configuration error before touching lock state. The method is 88.9% covered, transaction coverage is
94.2%, and full-module coverage is 96.6%.

The EOF shared-lock failure path now verifies that `Transaction.Size` preserves lock and file context and skips the
file-sizer call. The method is now 100.0% covered, transaction coverage is 94.8%, and full-module coverage remains
96.6%.

`Transaction.Append` now tests both its zero-value guard and the EOF exclusive-lock failure. The lock failure preserves
the conflict and file-operation context, returns a zero block, and prevents the file-manager append. The method is now
100.0% covered, transaction coverage is 96.0%, and full-module coverage remains 96.7%.

`Transaction.GetInt` now tests the existing shared-lock failure branch for a pinned block. It returns zero, preserves
the conflict and block/offset context, and stops before page access. The method is now 100.0% covered, transaction
coverage is 96.5%, and full-module coverage is 96.8%.

`Transaction.GetString` now tests the existing shared-lock failure branch for a pinned block. It returns an empty
string, preserves the conflict and block/offset context, and stops before page access. The method is now 100.0%
covered, transaction coverage is 97.1%, and full-module coverage remains 96.8%.

`Transaction.SetInt` now tests its existing exclusive-lock failure path. The lock cause and operation context remain
visible, and the page stays unchanged and clean. Method coverage is 95.0%, transaction coverage is 97.7%, and
full-module coverage is 96.9%.

`Transaction.SetString` now tests its existing exclusive-lock failure path. The lock cause and operation context remain
visible, and the stored string and page metadata stay unchanged. Method coverage is 95.0%, transaction coverage is
98.3%, and full-module coverage remains 96.9%.

`Transaction.Rollback` now tests its missing recovery-iterator guard. It returns the configuration error before flushing
buffers or appending a rollback marker. The method is now 100.0% covered, transaction coverage is 98.8%, and full-module
coverage is 97.0%.

The current-block iterator boundary test covers the error returned when an in-memory log-page boundary is outside
the valid range, while confirming no iterator is returned and the durable tail is unchanged. The index metadata tests
also cover early rejection of index lookups with missing table- or statistics-manager dependencies and an index
registration with a missing table-manager dependency, before catalog or transaction access.

`MetadataManager.Statistics` now has an independent refresh-failure test. It confirms the facade returns no partial
snapshot, keeps both table/facade and refresh context, counts the failed lookup, and does not cache a result. The method
is now 100.0% covered; metadata package coverage is 97.9%, while the full module remains 97.0%.

`StatManager.Refresh` now exercises its table-catalog layout failure path. The test checks the wrapped read error and
verifies the refresh keeps the old cache and lookup count intact. Metadata package coverage is now 98.2%,
`refreshLocked` is 81.5% covered, and full-module statement coverage is 97.1%.

`recovery.LogPreviousInteger` now covers the assigned-buffer case where page contents are nil. The focused test confirms
the transaction/block context is retained and no undo record is appended; the function is 100.0% covered, recovery
package coverage is 97.6%, and full-module statement coverage remains 97.1%.

`recovery.LogPreviousString` now covers the assigned-buffer case where page contents are nil. The focused test verifies
transaction/block context and confirms no undo record is appended; the function is 100.0% covered, recovery package
coverage is 97.9%, and full-module statement coverage is 97.2%.

`recovery.UndoRestartRecords` now verifies the integer-undo failure path independently from string restoration. The error
retains its cause and SETINT context, and older history is not consumed; the function is 100.0% covered, recovery
package coverage is 98.2%, and full-module statement coverage remains 97.2%.

`recovery.UndoRollbackRecords` now has an independent SETINT restore-failure test. It preserves the cause and SETINT
context, stops before older rollback history, and confirms cleanup; function coverage is 100.0%, recovery package
coverage is 98.5%, and full-module statement coverage is 97.3%.

`LockTable.WaitSharedFor` now also covers an already-expired deadline with a zero-duration request. The test verifies
that timeout is immediate and leaves the exclusive holder and unrelated lock state unchanged. Concurrency package
coverage is 96.5%, and full-module statement coverage remains 97.3%.

An uncontended request on a zero-value `LockTable` now verifies that `WaitSharedFor` initializes the lock map and
grants exactly one shared hold. Concurrency package coverage is 97.3%, and full-module statement coverage is 97.4%.

`DecodeSetStringRecord` now has a separate truncated-filename-length case. It verifies rejection occurs before reading
the later fixed fields; the decoder is 100.0% covered, recovery package coverage is 98.8%, and full-module coverage
remains 97.4%.

The SETINT encoder's signed filename-length bound now has a synthetic boundary test through a small internal validator,
so the rejection is verified without allocating a filename larger than two gigabytes. Encoded output for ordinary
filenames is unchanged.

The SETINT encoded-size overflow guard now accepts a synthetic host-int maximum in a small internal sizing helper.
Its overflow path is tested without depending on the machine architecture or allocating a large record.

SETSTRING's signed int32 component-length guard now tests both the filename and old-value inputs through the same
encoder path using a small synthetic limit. Both are rejected before record allocation, with the production limit and
wire bytes unchanged.

The SETSTRING combined-size check now accepts a synthetic host-int maximum. Separate filename and old-value cases
verify each contribution is rejected before allocation when it exceeds the remaining encoded capacity.

`LockTable.WaitExclusiveFor` now also covers a blocked request whose timeout is already zero. The test verifies the
shared holders remain unchanged and the request returns before registering an exclusive waiter.

Bounded shared and exclusive waits now share a timeout callback helper. Its completed-wait branch is called directly in
a deterministic test, while the existing deadline tests continue to cover active expiry and condition notification.

`log.Manager.Iterator` now has a Darwin/Linux FIFO regression test for current-tail flush failure. It confirms that
iterator creation returns no partial iterator and retains both the `ESPIPE` cause and operation context.

The log iterator boundary parser now directly covers a Page too short to hold its four-byte boundary, retaining both
the block identifier and field-read stage in the returned error.
The CI step reports
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
