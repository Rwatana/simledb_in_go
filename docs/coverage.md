# Go test coverage

The `Go test` and `Go coverage` workflows are independent checks. `Go coverage` runs the full suite with a coverage
profile, writes total statement coverage to the GitHub Actions job summary, and fails unless the full module reaches
100.0%. `Go vet`, documentation, issue-policy, pull-request-policy, and commit-count checks each have their own
workflow file and status check.

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

`Iterator.Next` now directly covers its corrupted-position guard, verifying a position beyond Page capacity returns
no record and preserves both numeric values in the error context.

The current-block iterator now also covers a record position with fewer than four bytes remaining for its length
prefix, returning no partial record and retaining the offset and Page-read context.

Previous-block traversal now covers a FileManager read failure, confirming the wrapped cause is returned without
committing a new block or position to the iterator.

`log.Manager.Append` now also covers failure to read the in-memory tail boundary. A short Page deterministically
exercises the read error and verifies that Append returns LSN zero, preserves the block and Page-read context, and
does not advance the latest LSN.

A separate malformed-boundary case covers failure to write the encoded record bytes. It verifies the returned block
and offset context and confirms both the tail Page and latest LSN remain unchanged.

The log record length guard now accepts a synthetic signed-int32 maximum in an internal append helper. Its
rejection path is covered without allocating a multi-gigabyte record, while the public Append path keeps the same
production limit and error text.

On Darwin and Linux, a FIFO regression test now forces `Append` into a full-tail rollover whose flush fails. It
checks the wrapped `ESPIPE` cause and append/rollover/block context, and verifies that the current Page, BlockID, and
LSN remain unchanged after the failed transition.

A separate filesystem test makes the durable log path a directory before a full-tail rollover. This reaches the
new-block append failure and confirms its wrapped cause and context do not advance the in-memory tail state.

`log.NewManager` now uses a private file-operation interface behind its unchanged public constructor. The internal
constructor test injects an existing-tail read error and verifies the wrapped cause, filename, and tail BlockID.

A short-page test exercises the existing-tail boundary decode failure through the same internal seam. It verifies
that construction stops before returning a Manager and preserves the Page read error with file context.

The internal log-file seam also injects a file-length lookup failure, confirming construction returns no Manager and
preserves both the underlying cause and the requested filename context.

An initial-page write test fails the first block write after append succeeds. It confirms the initialization error
retains its cause and operation context and that the appended block remains in the file.

The table-scan constructor now has a regression test for a negative reported file size. It confirms the error keeps
the table and file context and exits before append, pin, or record-page writes. `NewTableScan` is now 100.0% covered;
record package coverage is 97.0%, and full-module statement coverage is 98.5%.

`TableScan.Next` now also has an independent test for a current-block slot-read failure. The test checks that the
wrapped cause and block/slot context survive and that the cursor stays on the same record position. Record package
coverage is now 97.3%; full-module statement coverage remains 98.5%.

A second `TableScan.Next` test exercises failure to pin the next existing block after the current page is exhausted.
It checks that the error retains the next-block context and the current block has already been unpinned. `Next` is
now 100.0% covered; record package coverage is 97.6%, and full-module statement coverage is 98.6%.

`TableScan.Insert` now has a regression test for a slot-search failure in the current existing block. It verifies
the outer scan context, preserves the cause and cursor, and confirms no slot write occurs after the failed search.
Record package coverage is 97.9%, and full-module statement coverage remains 98.6%.

`TableScan.Insert` now also covers a failure when advancing from a full block to the next existing block. The test
checks error context, that the exhausted block is unpinned, and that no insert write occurs. A separate test forces
appending the next block to fail and checks the error context and page cleanup. `Insert` is now 96.0% covered, record
package coverage is 98.5%, and full-module statement coverage remains 98.7%.

The insertion path also has a focused test for a slot-flag read failure after rollover to a newly formatted block.
It verifies the new-block context, preserves the scan state, and confirms no logged record write follows the failure.
`TableScan.Insert` is now 100.0% covered, record package coverage is 98.8%, and full-module statement coverage is 98.8%.

`RecordPage.Format` now has a separate malformed-metadata test: a declared schema field absent from the physical offset
map returns block/slot/field context after writing only the slot's unlogged empty flag. `Format` is now 88.9% covered,
record package coverage is 99.1%, and full-module statement coverage remains 98.8%.

A synthetic two-slot layout now exercises `RecordPage.Format` rejecting a second slot whose flag offset exceeds
signed int32. The test confirms the invalid offset is reported before a write is attempted. `Format` is now 92.6%
covered, record package coverage is 99.4%, and full-module statement coverage is 98.9%.

A separate synthetic layout now tests `RecordPage.Format` rejecting a field offset beyond signed int32. The test checks
field context and confirms no field write follows the valid slot-flag write. `Format` is now 96.3% covered, record
package coverage is 99.7%, and full-module statement coverage remains 98.9%.

`RecordPage.Format` now also verifies failure to initialize an integer field after its slot flag is written. The test
checks the wrapped cause and block/slot/field/offset context, with no later format operation. `Format` and the record
package are now 100.0% covered, while full-module statement coverage is 99.0%.

The SETINT encoder now has a synthetic filename-limit test through its error-wrapping path, avoiding a huge allocation.
It confirms the encoder context and no partial record. The new bounded encoder helper is 94.4% covered, recovery package
coverage is 99.7%, and full-module statement coverage remains 99.0%.

A separate synthetic host-int boundary now exercises the SETINT encoder's record-size error wrapper and verifies that
no partial encoding is returned. The bounded encoder helper and recovery package are now 100.0% covered, and full-module
statement coverage is 99.1%.

Table-statistics calculation now exercises a record scan failure caused by a slot-flag read error. The test verifies
the underlying cause and table/scan context and confirms the scan releases its page pin. `calculateTableStats` is now
93.8% covered, metadata package coverage is 98.4%, and full-module statement coverage remains 99.1%.

Statistics refresh now also exercises failure while advancing the registered-table catalog scan. It verifies the
wrapped cause, cleanup of the failed scan's pin, and retention of the previous cache and lookup count. `refreshLocked`
is now 88.9% covered, metadata package coverage is 99.0%, and full-module statement coverage is 99.2%.

A separate refresh test now covers a registered table-name read failure after layout loading. It checks the wrapped
cause and catalog context, releases the scan pin, and retains the existing statistics snapshot. `refreshLocked` is now
92.6% covered, metadata package coverage is 99.2%, and full-module statement coverage is 99.3%.

Statistics refresh now also covers an invalid slot size while loading a registered table's layout. The test confirms
that the bad catalog value is reported with table context and the old cache and lookup count remain intact.
`refreshLocked` is now 96.3% covered, metadata package coverage is 99.5%, and full-module statement coverage remains
99.3%.

Refresh also now covers a registered table whose record scan fails while reading its first slot flag. The test
verifies the table and scan context, confirms the page pin is released, and keeps the old cache intact. Every statement
in `refreshLocked` is now covered, metadata package coverage is 99.7%, and full-module statement coverage is 99.4%.

The statistics record loop now uses a private scan interface so a defensive `GetRID` failure after a successful
`Next` can be exercised directly. The test checks the record index and wrapped cause; all metadata statements are
now covered, while full-module statement coverage remains 99.4%.

`FileManager.Append` now covers a file-handle `Stat` failure through a private file-operation seam. The test verifies
the wrapped cause and path context, ensures the file is closed, and confirms no write or sync follows. The file package
is now 98.0% covered, `Append` is 95.5% covered, and full-module statement coverage is 99.5%.

A separate `Append` test now covers a sync failure after the complete zero-filled block write. It verifies block/path
context, the wrapped cause, and handle cleanup. `Append` is now 100.0% covered, file package coverage is 99.0%, and
full-module statement coverage remains 99.5%.

`FileManager.Write` now also covers a sync failure after writing the complete page. The test checks block/path context,
the wrapped cause, page bytes, and handle cleanup. `Write` and the file package are now 100.0% covered, and full-module
statement coverage is 99.6%.

Log rollover now covers a failure to write the initialized page for the newly appended tail block. The test verifies that
the old tail remains selected, the record receives no LSN, and the successfully flushed LSN is retained. The
`rollOverLocked` function is now 86.7% covered, log package coverage is 95.9%, and full-module statement coverage
remains 99.6%.

Initial log-tail page setup now uses a private page-factory seam so allocation and boundary initialization failures can
be tested after the file block size has passed validation. Both failures preserve their stage context and append no
block. `newManagerWithPageFactory` is 100.0% covered, log package coverage is 97.5%, and full-module statement
coverage is 99.7%.

Rollover now uses the same private page-factory seam to cover failures while creating or initializing the next tail
page after the old tail flush succeeds. The tests confirm the old in-memory tail remains selected, no LSN is assigned
to the rejected record, the flushed LSN is retained, and no new disk block is appended. `rollOverLocked` is now
100.0% covered, log package coverage is 99.2%, and full-module statement coverage is 99.8%.

Log append now isolates the boundary write and LSN increment into a private helper. A direct error-path test uses an
undersized page to verify the wrapped block/offset context and unchanged LSN when the boundary write fails; ordinary
append still covers the success path. Every statement in the log package is now covered, and full-module statement
coverage is 99.9%. The only remaining package below 100.0% is transaction.

`Transaction.SetInt` now delegates recovery-LSN recording to a private helper whose error path can be tested directly
with a failing recorder. The test checks the wrapped cause and integer block/offset/LSN context. `SetInt` is now
100.0% covered, transaction package coverage is 99.4%, and full-module statement coverage remains 99.9%.

`Transaction.SetString` now uses the same recovery-LSN error helper with string-specific context. The focused failure
test verifies the wrapped cause and block/offset/LSN details. All ten Go packages and the full module now measure
100.0% statement coverage (`/tmp/simpledb-coverage-788.out`). CI keeps the total percentage in the step summary and
fails when full-module statement coverage falls below 100.0%.

At the Issue #792 checkpoint, the full module and each package remain at 100.0% statement coverage: buffer, concurrency,
file, log, metadata, query, record, recovery, storage, and transaction. The Issue #790 CI threshold passed on this stack;
the update-scan contract adds no executable statements. The full suite, race tests, vet, and documentation checks were
rerun for this checkpoint.

The scheduled recheck for Issue #792 also measured 100.0% full-module coverage in `/tmp/simpledb-coverage-792-current.out`;
each of the same ten packages remains at 100.0%, and `go tool cover -func` reports no uncovered functions. Race tests,
vet, documentation validation, and diff checks passed again. No additional existing error-path defect was found by this
regression pass.

The Issue #793 record-scan adapter checkpoint measures 100.0% statement coverage in the full module and each package
in `/tmp/simpledb-coverage-793.out`: buffer, concurrency, file, log, metadata, query, record, recovery, storage, and
transaction. `go tool cover -func` reports no uncovered functions. The tests exercise the adapter's schema, constant-kind,
and wrapped storage-error paths without changing the 100.0% CI threshold.

For a local full-module measurement, run:

```sh
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1
```

Inspect individual uncovered functions with:

```sh
go tool cover -func=coverage.out
```

At the 2026-11-23 scheduled recheck on the Chapter 8 adapter tip, `/tmp/simpledb-coverage-793-recheck.out` measured 100.0% full-module statement coverage, with no uncovered functions. A separate `go test ./... -cover -count=1` also reported 100.0% for buffer, concurrency, file, log, metadata, query, record, recovery, storage, and transaction. Race tests, vet, documentation validation, and diff checks passed.

At the 2026-11-23 selection-scan checkpoint, `/tmp/simpledb-coverage-795.out` measured 100.0% statement coverage for the full module and all ten packages. The selection scan and its tests add no uncovered statements. The complete suite, race tests, vet, documentation validation, and diff checks passed.

At the 2026-11-23 scheduled regression recheck on PR #796's current stack tip, `/tmp/simpledb-coverage-796-current.out` measured 100.0% full-module statement coverage. Each package is also at 100.0%: buffer, concurrency, file, log, metadata, query, record, recovery, storage, and transaction. `go tool cover -func` reports no uncovered functions. Full tests, per-package coverage, race tests, vet, documentation validation, and diff checks passed; no regression or error-path defect was confirmed.

At the 2026-11-23 scheduled recheck on the ProductScan worktree, `/tmp/simpledb-coverage-797.out` measured 100.0% full-module statement coverage and no uncovered profile regions. Buffer, concurrency, file, log, metadata, query, record, recovery, storage, and transaction each measured 100.0%. Every `ProductScan` function also measures 100.0%. The complete suite, per-package coverage, race tests, vet, documentation validation, and diff checks passed; the review found no confirmed regression or existing error-path defect.

At the 2026-11-23 ProjectScan checkpoint, `/tmp/simpledb-coverage-799.out` measured 100.0% for the full module and each package: buffer, concurrency, file, log, metadata, query, record, recovery, storage, and transaction. The raw profile has no zero-count statement regions, and every `ProjectScan` function is 100.0%. The first measurement identified the existing nil-receiver guard in `ProjectScan.HasField` as uncovered; a focused test now verifies that it returns false. Full tests, per-package coverage, race tests, vet, docs, and diff checks all pass.

At the Issue #801 lexer checkpoint, `/tmp/simpledb-coverage-801.out` measures 100.0% statement coverage in the full module and all eleven packages, including the new `internal/parse` package. Each lexer function is 100.0%, the profile contains no zero-count statement regions, and the coverage-enforcing CI threshold remains unchanged.

At the Issue #807 string-token checkpoint, `/tmp/simpledb-coverage-807.out` again measures 100.0% statement coverage in the full module and each of the eleven packages. Both string-constant lexer methods and the updated token scanner are fully covered; the complete suite, per-package coverage, race tests, vet, documentation validation, and diff checks pass.

The scheduled regression recheck on PR #808 produced `/tmp/simpledb-coverage-808-recheck.out`: full-module statement coverage is 100.0%, with no zero-count functions or statements. All eleven packages (buffer, concurrency, file, log, metadata, parse, query, record, recovery, storage, and transaction) remain at 100.0%. The uncached suite, per-package coverage, race tests, vet, docs validation, and diff check all pass.

At the Issue #809 identifier checkpoint, `/tmp/simpledb-coverage-809.out` measures 100.0% for the full module and all eleven packages. Every lexer method and helper is covered, including each reserved-word branch. The complete suite, per-package coverage, race tests, vet, docs validation, and diff check pass.

At the Issue #811 delimiter checkpoint, `/tmp/simpledb-coverage-811.out` measures 100.0% for the full module and all eleven packages. Every lexer method and helper remains fully covered; the complete suite, per-package coverage, race tests, vet, docs validation, and diff check pass.

At the Issue #813 keyword checkpoint, `/tmp/simpledb-coverage-813-audit.out` measures 100.0% full-module statement coverage. All eleven packages—buffer, concurrency, file, log, metadata, parse, query, record, recovery, storage, and transaction—also report 100.0%. The profile contains no zero-count statement regions, and `go tool cover -func` reports every lexer function and helper at 100.0%. The uncached full suite, race tests, vet, documentation validation, and diff check pass. The independent `Go coverage` workflow continues to fail the check unless full-module coverage is exactly 100.0%.

At the Issue #815 parser checkpoint, `/tmp/simpledb-coverage-815-final.out` measures exactly 100.0% full-module statement coverage, and every package (buffer, concurrency, file, log, metadata, parse, query, record, recovery, storage, and transaction) reports 100.0%. The profile has no zero-count statement regions, and `go tool cover -func` reports no uncovered functions. The initial audit found one unreachable parser error branch: a successful `MatchStringConstant` guarantees the following `EatStringConstant` succeeds while the lexer state is unchanged. That redundant branch was removed; malformed-string errors remain covered through the integer fallback. Full tests, race, vet, docs, and diff checks pass, and the existing CI gate enforces 100.0% on pull requests and pushes to main.

At the Issue #817 field-parser checkpoint, `/tmp/simpledb-coverage-817.out` measures 100.0% statement coverage for the full module and all eleven packages. Every parser and lexer function is covered, with zero uncovered profile regions. Full tests, race, vet, documentation validation, and diff checks pass; the merged Go coverage workflow continues enforcing the threshold in CI.

At the Issue #819 expression-parser checkpoint, `/tmp/simpledb-coverage-819.out` measures 100.0% statement coverage for the full module and all eleven packages. The parser expression branches and their field, integer, string, and invalid-token outcomes are covered, and the profile contains no zero-count statement regions. Full tests, race, vet, documentation validation, and diff checks pass; the independent Go coverage workflow continues enforcing 100.0% in CI.

At the Issue #821 term-parser checkpoint, `/tmp/simpledb-coverage-821.out` measures 100.0% statement coverage for the full module and all eleven packages. The left-expression, equals-delimiter, right-expression, and error-context branches are covered with no zero-count profile regions or functions. Full tests, race, vet, documentation validation, and diff checks pass; the independent Go coverage workflow continues enforcing 100.0% in CI.

At the Issue #823 predicate-parser checkpoint, `/tmp/simpledb-coverage-823.out` measures 100.0% statement coverage for the full module and all eleven packages. Single-term completion, recursive `and` handling, and first/later-term failures are covered with no zero-count profile regions or functions. Full tests, race, vet, documentation validation, and diff checks pass; the independent Go coverage workflow continues enforcing 100.0% in CI.

The scheduled regression and coverage audit on PR #824 (`/tmp/simpledb-coverage-824-audit.out`) again reports 100.0% full-module statement coverage. All eleven packages—buffer, concurrency, file, log, metadata, parse, query, record, recovery, storage, and transaction—are 100.0%; `go tool cover -func` lists no uncovered functions, and the profile has zero uncovered statement regions. The uncached full suite, full race suite, vet, docs validation, and diff check pass. No existing regression or uncovered error path was found; the independent Go coverage workflow continues to fail CI below 100.0%.

At the Issue #825 query-data checkpoint, `/tmp/simpledb-coverage-825.out` measures 100.0% statement coverage in the full module and all eleven packages. QueryData construction, list copying, predicate access, optional rendering, and safe incomplete-value handling are covered with no zero-count regions or functions. Full tests, race, vet, docs, and diff checks pass; the Go coverage workflow continues enforcing 100.0% in CI.

At the Issue #834 SELECT-query checkpoint, `/tmp/simpledb-coverage-834.out` measures 100.0% statement coverage in the full module and all eleven packages. `ParseQuery` and `parseQueryNameList` are each 100.0%, and the profile contains no zero-count statement regions. The uncached suite and race tests pass; vet, docs, and diff checks pass. The separate Go coverage workflow continues to require exactly 100.0%.

At the Issue #836 DELETE-data checkpoint, `/tmp/simpledb-coverage-836.out` measures 100.0% statement coverage in the full module and all eleven packages. `DeleteData`, `NewDeleteData`, `TableName`, and `Predicate` are covered, with no zero-count statement regions. The full suite, race tests, vet, and documentation validation pass; the independent coverage workflow continues enforcing 100.0%.
