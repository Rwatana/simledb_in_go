# Go test coverage

## Current gate

The current full-module statement coverage is **100.0%**. The independent `Go coverage` workflow runs the full suite
with a coverage profile, reports the module total in the GitHub Actions job summary, and fails unless that total is
100.0%. The repository keeps each check in its own workflow file and status check: Branch name, Commit count,
Documentation, Go coverage, Go test, Go vet, Issue closing reference, Issue policy, Issue reference, and Pull request
title. Issue policy runs on issue events; the other checks validate pull requests. There is no current
`pull-request-policy.yml` workflow.

## Latest audit checkpoint: 2026-11-24

The uncached full-module command `go test ./... -count=1 -coverprofile=coverage.out` reports 100.0% statement
coverage in every executable package. `go tool cover -func=coverage.out` reports 100.0% for every executable function
and the module total; the profile has no uncovered statements. The interface-only `internal/index` package contains no
statements.

| Package | Statement coverage |
| --- | ---: |
| `internal/buffer` | 100.0% |
| `internal/concurrency` | 100.0% |
| `internal/index` | No statements |
| `internal/index/btree` | 100.0% |
| `internal/index/hash` | 100.0% |
| `internal/index/query` | 100.0% |
| `internal/infrastructure/file` | 100.0% |
| `internal/jdbc/embedded` | 100.0% |
| `internal/jdbc/network` | 100.0% |
| `internal/log` | 100.0% |
| `internal/materialize` | 100.0% |
| `internal/metadata` | 100.0% |
| `internal/parse` | 100.0% |
| `internal/plan` | 100.0% |
| `internal/query` | 100.0% |
| `internal/record` | 100.0% |
| `internal/recovery` | 100.0% |
| `internal/server` | 100.0% |
| `internal/storage` | 100.0% |
| `internal/transaction` | 100.0% |

Issue #1117 adds the server-side record-transaction adapter used for catalog initialization. The server manager
operation and each adapter method are 100.0% covered; the full-module profile still reports 100.0% for every
executable package and function.

Issue #1119 adds the explicit `Managers.Recover` restart operation. The durable restart scenario covers undo of a
flushed uncommitted update, CHECKPOINT ordering, and completion of the recovery transaction; failure tests cover
manager validation, corrupt history, and recovery/commit lifecycle errors. The uncached command
`GOCACHE=/tmp/simpledb-go-cache go test ./... -count=1 -coverprofile=/tmp/simpledb-coverage-1119-final.out`
reported 100.0% statement coverage for every executable package. `go tool cover -func` reported every executable
function and the module total at 100.0%, with no zero-count statements.

Issue #1121 adds the `Managers.QueryPlanner` composition. Its integration test creates catalog metadata, registers a
table, and asks the composed basic planner for a SELECT projection through a caller-owned transaction. The uncached
command `GOCACHE=/tmp/simpledb-go-cache go test ./... -count=1 -coverprofile=/tmp/simpledb-coverage-1121-final.out`
reports 100.0% statement coverage for every executable package; `go tool cover -func` reports every executable
function and module total at 100.0%, with no zero-count statements.

Issue #1123 adds the `Managers.UpdatePlanner` composition. Its integration test executes CREATE TABLE through the
composed basic update planner and reads the registered schema through the same metadata facade and transaction. The
uncached command `GOCACHE=/tmp/simpledb-go-cache go test ./... -count=1 -coverprofile=/tmp/simpledb-coverage-1123-final.out`
reports 100.0% statement coverage for every executable package; `go tool cover -func` reports every executable
function and module total at 100.0%, with no zero-count statements.

Issue #1125 adds `Managers.CreateQueryPlan`, joining SQL SELECT parsing to the manager set's query planner while
leaving the supplied transaction caller-owned. Tests cover the successful plan, incomplete managers, a nil
transaction, parse failures, and planner failures. Uncached
`GOCACHE=/tmp/simpledb-go-cache go test ./... -count=1 -coverprofile=/tmp/simpledb-coverage-1125-final.out` reports
100.0% statement coverage in every executable package; `go tool cover -func` reports every executable function and the
module total at 100.0%, with no zero-count statements. The 100.0% workflow remains an independent status check.

Issue #1127 adds `Managers.ExecuteUpdate`, dispatching parsed INSERT, DELETE, UPDATE, CREATE TABLE, CREATE VIEW, and
CREATE INDEX commands to their corresponding update-planner operations without taking transaction ownership. Tests
cover all six routes, unsupported command data, malformed SQL, and planner errors. Uncached
`GOCACHE=/tmp/simpledb-go-cache go test ./... -count=1 -coverprofile=/tmp/simpledb-coverage-1127-final.out` reports
100.0% coverage in all executable packages; the profile has 4,181 statement entries and zero uncovered statements.
`go tool cover -func` reports all 582 executable functions and the module total at 100.0%. The independent coverage
workflow continues to enforce this target.

Issue #1129 adds `Managers.SetQueryPlannerStrategy` for the existing basic and cost-aware query planners. Tests keep
the basic default, exercise both implementations against the same metadata facade, and verify that nil manager state
or an unknown strategy does not replace the installed planner. Uncached
`GOCACHE=/tmp/simpledb-go-cache go test ./... -count=1 -coverprofile=/tmp/simpledb-coverage-1129-final.out` reports
100.0% statement coverage for every executable package; the profile contains 5,356 statement entries and no
uncovered statements. `go tool cover -func` reports 583 executable functions and the module total at 100.0%, with
no function below 100%. The independent coverage workflow remains a separate status check.

Issue #1131 adds the initial `database/sql/driver.Conn` adapter for embedded connections. Tests verify fresh transaction
creation, explicit Commit/Rollback delegation, repeated-completion errors, connection close behavior, unsupported
Prepare, and invalid state. Uncached
`GOCACHE=/tmp/simpledb-go-cache go test ./... -count=1 -coverprofile=/tmp/simpledb-coverage-1131-final.out` reports
100.0% statement coverage for all 17 executable packages; interface-only `internal/index` has no statements. The
profile covers 5,388 statements with zero uncovered; `go tool cover -func` reports 589 executable functions and the
module total at 100.0%, with no function below 100%.

Issue #1131 adds the initial `database/sql/driver.Conn` adapter for embedded connections. Tests verify transaction
creation and explicit Commit/Rollback delegation, repeated completion errors, connection close behavior, unsupported
Prepare, and invalid state. Uncached
`GOCACHE=/tmp/simpledb-go-cache go test ./... -count=1 -coverprofile=/tmp/simpledb-coverage-1131-final.out` reports
100.0% statement coverage for all 17 executable packages; the interface-only `internal/index` package has no
statements. The profile covers 5,388 statements with zero uncovered; `go tool cover -func` reports 589 executable
functions and the module total at 100.0%, with no function below 100%.

Issue #1161 adds `embedded.ParseEmbeddedURL` for the Java-compatible `jdbc:simpledb:` prefix. Table-driven tests preserve relative and absolute path suffixes exactly and reject unsupported prefixes and empty names. Parsing remains separate from path resolution, manager construction, and connector binding. The parser and tests remain within the 100.0% statement, function, and module coverage gate; the independent coverage workflow continues to enforce that gate.

Issue #1163 adds `network.ParseNetworkURL` for the Java-compatible `jdbc:simpledb://` host form. Table-driven tests cover case-preserving DNS names, IPv4 and bracketed IPv6 hosts, plus empty or unsupported URLs, explicit ports, and non-host URL components. Parsing remains independent of dialing and remote-session lifecycle; the default registry port convention is preserved. The separate coverage workflow continues to enforce 100.0% statement coverage.

Issue #1165 adds `network.DialNetworkURL`, which parses a network JDBC URL, uses `net.JoinHostPort` with the Java-compatible default registry port 1099, and forwards the caller context to TCP dialing. Tests cover DNS/IPv4/IPv6 address construction, invalid-input rejection before dialing, cancellation, dial-error wrapping, nil dependencies, and caller ownership of a successful connection. The coverage workflow continues to enforce 100.0% statement coverage.

Issue #1167 adds `network.ServeConnection` and the `SimpleDB.Connect` RPC method. `net.Pipe` tests exercise the gob
RPC protocol without a listener, verify that Connect appends exactly one START record and retains that transaction on
the connection service, and cover duplicate connects, incomplete managers, registration failures, and START-log errors.
Every executable package and function remains subject to the separate 100.0% coverage workflow.


Issue #1169 adds `DriverService.Commit`. An RPC commit durably completes the current transaction, then creates and retains
its successor; failures return no transaction ID. Tests inspect the ordered START/COMMIT log records and cover commit
failure plus the post-commit replacement-start failure. The separate coverage workflow enforces 100.0% in every
executable package and function.

Issue #1171 adds `DriverService.Rollback`, which durably rolls back the connection transaction and starts its successor.
Tests inspect the ROLLBACK/START record sequence and cover invalid state, rollback failure, and a failed successor START
that leaves the completed transaction detached. The independent coverage workflow keeps all executable code at 100.0%.

Issue #1173 adds `DriverService.Close`, which commits the active network RPC transaction and closes the session without
starting a successor. RPC tests inspect the final COMMIT record, confirm subsequent session operations fail, and cover
missing state, repeated close, and commit failure without losing the still-open transaction. The separate coverage
workflow requires every executable package, function, and module statement to remain at 100.0%.

Issue #1174 adds `DriverService.ExecuteUpdate`, routing one SQL update through the retained session transaction and the
existing server parser/planner path without implicitly committing. RPC tests execute catalog-backed CREATE TABLE and
INSERT commands, verify the row count, and confirm that explicit Commit completes the original transaction. Direct method
tests cover parse/planner errors, zeroed responses, missing dependencies, and closed sessions. The independent coverage
workflow continues to require 100.0% in all executable packages, functions, and the module total.

Issue #1178 adds `RemoteSession.Rollback`, which calls the server's Rollback RPC and updates the retained transaction ID only
after a successful reply. Tests cover successor-ID replacement, nil session/client state, RPC failure, and ID
preservation. The independent coverage workflow enforces 100.0% package, function, and module statement coverage.

Issue #1179 adds `RemoteSession.Close`, which calls `SimpleDB.Close` and marks the local session closed only after a
successful reply. It clears the transaction ID on success, preserves it after remote failure so callers can retry, and
rejects later Commit, Rollback, and Close calls locally without closing the caller-owned RPC client. The separate coverage
workflow continues to enforce 100.0% package, function, and module statement coverage.

Issue #1181 adds `RemoteSession.ExecuteUpdate`, which sends one SQL statement through `SimpleDB.ExecuteUpdate`, returns
affected rows, and leaves the current transaction ID and completion to the caller. Client validation and remote errors
are contextual and preserve the ID. The independent coverage workflow continues to require 100.0% package, function,
and module statement coverage.

Issue #1197 adds the `materialize.TempTable` descriptor, retaining a table-scan transaction and schema layout while
allocating concurrent-safe `tempN` names without transaction calls or physical file creation. Tests cover construction,
metadata access, invalid dependencies, and concurrent allocation. The independent coverage workflow continues to require
100.0% statement coverage across executable packages, functions, and the module.

Issue #1198 fixes a pin leak when `record.NewTableScan` fails while formatting a new first block. The constructor now
attempts to unpin the acquired page and preserves both formatting and cleanup errors; failures before pin acquisition do
not unpin. Tests cover successful release, both-cause error wrapping, and early-failure behavior. The independent
coverage workflow continues to enforce 100.0% statement coverage.

Issue #1200 adds `materialize.TempTable.Open`, returning a `query.UpdateScan` through `query.NewRecordScan` and
delegating first-file initialization and page pinning to `record.NewTableScan`. Tests cover query-value reads/writes,
new/existing temporary files, invalid descriptors, wrapped initialization failures, format-error cleanup, and the
caller-owned scan pin. The separate coverage workflow continues to require 100.0% in every executable package,
function, and the module.

Issue #1202 adds `materialize.RecordComparator` for ordered lexicographic comparison of query scan values. Tests cover
numeric and string order, short-circuit and tie behavior, copied field order, empty fields, invalid state, read failures,
and incomparable constants. The independent coverage workflow continues to require 100.0% statement coverage.

Issue #1204 adds `materialize.MaterializePlan`, copying source query rows in schema order into a temporary table and
returning its scan before the first row. Tests cover empty and multi-field results, source metadata, exact and partial
block estimates, each source/destination error phase, and scan cleanup with joined causes. Every statement in the new
materialization-plan implementation is covered; the independent workflow continues to require 100.0% package,
function, and module statement coverage.

Issue #1206 adds `materialize.SortScan`, merging one or two sorted temporary-table runs by their current comparator
heads. Tests cover interleaving and Java-compatible tie order, single/empty runs, rewind, exhaustion, delegated reads,
invalid inputs before storage use, initialization/advance/compare errors, and scan cleanup. The independent workflow
continues to require 100.0% package, function, and module statement coverage.

Issue #1208 adds `materialize.SortPlan`, splitting source records into comparator-ascending temporary runs and
merging run pairs over repeated passes. Tests cover sorted, empty, single, odd, and multi-pass runs; Java-compatible
ties; delegated statistics; source/split/merge/final-open errors; scan cleanup; and preserved causes. The separate
coverage workflow requires 100.0% statement coverage in every executable package, function, and module total.

Issue #1210 adds `multibuffer.BestRoot`, reserving two buffers and selecting the first ceiling root that fits the
remaining count. Boundary, exact-root, non-perfect-root, invalid-size, maximum-int, and exhaustive Java-equivalence
tests keep the new package at 100.0% statement coverage; the separate coverage workflow continues to enforce the same
target across the module.

Issue #1212 adds `multibuffer.BestFactor`, selecting the first floating-point ceiling quotient that fits after the
two-buffer reservation. Binary search preserves the increasing-divisor result while avoiding a linear pass over large
sizes. Reservation, quotient, invalid-input, maximum-int, and exhaustive Java-formula tests keep the package at
100.0%; the independent coverage workflow enforces the module-wide target.

Issue #1195 adds `storage.Page.SetString`, converting UTF-8 bytes through the existing `SetBytes` length-prefix path.
Tests round-trip empty text, embedded NUL, and multibyte text through both string and byte APIs, and verify a failed
capacity write preserves the original page. The separate coverage workflow continues to enforce 100.0% package,
function, and module statement coverage.

Issue #1193 adds `storage.Page.GetString` as a read view over the existing four-byte length-prefixed payload. Tests
cover empty text, embedded NUL, multibyte UTF-8, unchanged page bytes, truncated prefix, negative length, and truncated
payload errors. The separate coverage workflow continues to enforce 100.0% package, function, and module statement
coverage.

Issue #1191 adds JDBC-compatible display widths to remote query columns. Integer width is six; varchar width uses its
declared field length; each result adds one character after taking the larger of that width and the field name. Tests
cover each sizing case and verify metadata survives the RPC response. The separate coverage workflow still requires
100.0% statement coverage in every executable package, function, and module total.

Issue #1189 makes `ServeConnection` roll back a still-active transaction after its RPC transport ends, releasing locks and
buffers. An explicit `SimpleDB.Close` has already committed and is left unchanged. Tests exercise uncommitted catalog
rollback, committed Close behavior, inactive cleanup, and returned rollback failures. The independent coverage workflow
continues to enforce 100.0% statement coverage in every executable package, function, and module total.

Issue #1187 adds `ServeListener` to accept and serve multiple Go net/rpc connections concurrently with an independent
transaction per connection. Tests cover separate client state, progress while another client remains connected, closing
accepted connections after service exit, nil inputs, wrapped Accept failures, and listener-owned shutdown. The independent
coverage workflow continues to require 100.0% statement coverage in every executable package, function, and module total.

Issue #1185 adds `DialNetworkRPCClient`, which turns the context-aware default-port TCP dial into a caller-owned Go
`net/rpc.Client`; closing that client closes its connection. Tests perform a real RPC exchange through an injected dialer
and cover URL/port selection, cancellation, nil inputs/results, and wrapped dial failures. Creating the transport does not
start a database session or claim Java RMI compatibility. The independent coverage workflow continues to enforce 100.0%
statement coverage in every executable package, function, and the module total.

Issue #1183 adds `RemoteSession.ExecuteQuery`, which returns the server's ordered column metadata and typed rows without
completing the active transaction or changing its ID. Nil, client-less, and closed sessions return empty results with
contextual errors; RPC failures preserve their cause and suppress partial results. The coverage workflow continues to
enforce 100.0% statement coverage in every executable package, function, and the module total.

Issue #1177 adds `RemoteSession.Commit`, which calls the server's Commit RPC and updates the retained transaction ID only
after a successful response. Tests cover successor-ID replacement, nil session/client state, RPC failure, and ID
preservation. The separate coverage workflow requires 100.0% package, function, and module statement coverage.

Issue #1176 adds `ConnectRPCSession` and `RemoteSession`, which send one `SimpleDB.Connect` request through a caller-owned
RPC client and retain the returned transaction ID without closing the client. Tests verify the ID, a subsequent client
call, nil input, and a server-side START failure that returns no session. The coverage workflow enforces 100.0% for all
executable packages, functions, and the module total.

Issue #1175 adds `DriverService.ExecuteQuery`, which parses and plans a SELECT, materializes schema-ordered columns and
typed scalar rows into gob-friendly DTOs, and closes the scan without completing the session transaction. RPC tests cover
projected integer/varchar rows; direct fault-injection tests cover planning/open/read/close errors, partial-open cleanup,
invalid lifecycle state, empty results, and response reset. The separate coverage workflow continues to enforce 100.0%
package, function, and module statement coverage.

## Historical coverage checkpoints

The figures and audit notes below are historical snapshots. They describe the state at the named checkpoint, not the
current coverage gate or current module total.

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

At the Issue #838 DELETE-parser checkpoint, `/tmp/simpledb-coverage-838.out` measures 100.0% statement coverage in the full module and all eleven packages. `ParseDelete` is fully covered across both optional-predicate paths, clause and table failures, and nil receiver handling; the profile has no zero-count statement regions. The uncached full suite, race tests, vet, and docs validation pass.

At the Issue #840 INSERT-data checkpoint, `/tmp/simpledb-coverage-840.out` measures 100.0% statement coverage in the full module and all eleven packages. `InsertData`, its constructor, and all accessors are 100.0%, with no zero-count statement regions. Tests verify field/value order, defensive copies on construction and access, and safe zero-value access. The full uncached suite, race tests, vet, and documentation validation pass.

At the Issue #842 INSERT-parser checkpoint, `/tmp/simpledb-coverage-842.out` measures 100.0% statement coverage in the full module and all eleven packages. `ParseInsert`, `parseInsertFieldList`, and `parseInsertConstantList` are each 100.0%, and the profile has no zero-count statement regions. Tests cover both list loops, each required grammar item, contextual byte-offset errors, and preservation of a following token. The uncached full suite, race tests, vet, and documentation validation pass.

At the Issue #844 UPDATE-data checkpoint, `/tmp/simpledb-coverage-844.out` measures 100.0% statement coverage in the full module and all eleven packages. `ModifyData`, its constructor, and all four accessors are each 100.0%, with no zero-count statement regions. Tests cover stored values and the safe zero value; the uncached full suite, race tests, vet, and documentation validation pass.

At the Issue #846 UPDATE-parser checkpoint, `/tmp/simpledb-coverage-846.out` measures 100.0% statement coverage in the full module and all eleven packages. `ParseUpdate` is 100.0%, and the profile contains no zero-count statement regions. Tests cover assignments from a field or constant, optional predicates, preserved following input, each error context and byte offset, and a nil receiver. The uncached full suite, race tests, vet, and docs validation pass.

At the Issue #848 CREATE TABLE-data checkpoint, `/tmp/simpledb-coverage-848.out` measures 100.0% statement coverage in the full module and all eleven packages. `CreateTableData`, its constructor, both accessors, and `copySchema` are each 100.0%, with no zero-count statement regions. Tests verify field order and definitions, schema isolation on construction and access, and safe zero-value behavior. The full uncached suite, race tests, vet, and documentation validation pass.

At the Issue #850 CREATE VIEW-data checkpoint, `/tmp/simpledb-coverage-850.out` measures 100.0% statement coverage in the full module and all eleven packages. `CreateViewData`, its constructor, and both accessors are each 100.0%, with no zero-count statement regions. Tests cover the stored view name, rendering the query definition, and safe zero-value behavior. The full uncached suite, race tests, vet, and documentation validation pass.

At the Issue #851 CREATE INDEX-data checkpoint, `/tmp/simpledb-coverage-851.out` measures 100.0% statement coverage in the full module and all eleven packages. `CreateIndexData`, its constructor, and all three accessors are each 100.0%, with no zero-count statement regions. Tests cover each stored name and the safe zero value. The full uncached suite, race tests, vet, and documentation validation pass.

At the Issue #852 CREATE VIEW-parser checkpoint, `/tmp/simpledb-coverage-852.out` measures 100.0% statement coverage in the full module and all eleven packages. `Parser.ParseCreateView` is fully covered with no zero-count statement regions. Tests cover successful parsing, trailing-token preservation, every required grammar element, malformed query definitions, byte offsets, and the nil receiver. The full uncached suite, race tests, vet, and documentation validation pass.

At the Issue #853 CREATE INDEX-parser checkpoint, `/tmp/simpledb-coverage-853.out` measures 100.0% statement coverage in the full module and all eleven packages. `Parser.ParseCreateIndex` is fully covered with no zero-count statement regions. Tests cover successful parsing, trailing-token preservation, every required keyword/name/delimiter, byte offsets, and the nil receiver. The full uncached suite, race tests, vet, and documentation validation pass.

At the Issue #854 CREATE TABLE-parser checkpoint, `/tmp/simpledb-coverage-854.out` measures 100.0% statement coverage in the full module and all eleven packages. `Parser.ParseCreateTable` and its field-definition helpers are fully covered with no zero-count statement regions. Tests cover ordered integer/varchar fields, trailing-token preservation, malformed syntax and types, byte offsets, and a nil receiver. The full uncached suite, race tests, vet, and documentation validation pass.

At the Issue #855 update-command dispatcher checkpoint, `/tmp/simpledb-coverage-855.out` measures 100.0% statement coverage in the full module and all eleven packages. `Parser.ParseUpdateCommand` and its CREATE dispatcher are fully covered for all six command kinds, malformed input, unsupported CREATE subtypes, unrelated statements, and nil receivers. No zero-count statement regions remain; the uncached suite, race tests, vet, docs, and diagram validation pass.

At the Issue #857 plan-contract checkpoint, `/tmp/simpledb-coverage-857.out` measures 100.0% statement coverage in the full module and all eleven packages with executable statements. `internal/plan` is interface-only and correctly reports `[no statements]`; it adds no uncovered production statements. The compile-time conformance test checks every `Plan` method signature, and the profile has no zero-count statement regions. The uncached suite, race tests, vet, docs, and diagram validation pass.

At the Issue #859 table-plan-schema checkpoint, `/tmp/simpledb-coverage-859.out` measures 100.0% statement coverage in the full module and all eleven packages with executable statements. `TablePlan.Schema` covers both a nil receiver and missing layout, and the profile has no zero-count statement regions. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #861 table-plan-open checkpoint, `/tmp/simpledb-coverage-861.out` measures 100.0% statement coverage in the full module and all twelve packages with executable statements. `TablePlan.Open` covers successful first-row access, storage initialization errors, and nil receiver/transaction/layout cases; the profile has no zero-count statement regions. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #863 table-plan-block-estimate checkpoint, `/tmp/simpledb-coverage-863.out` measures 100.0% statement coverage in the full module and all twelve packages with executable statements. `TablePlan.BlocksAccessed` covers the forwarded estimate plus nil receiver and missing-statistic cases, with no zero-count statement regions. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #865 table-plan-output-estimate checkpoint, `/tmp/simpledb-coverage-865.out` measures 100.0% statement coverage in the full module and all twelve packages with executable statements. `TablePlan.RecordsOutput` covers the forwarded row estimate plus nil receiver and missing-statistic cases, with no zero-count statement regions. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #867 table-plan-distinct-value checkpoint, `/tmp/simpledb-coverage-867.out` measures 100.0% statement coverage in the full module and all twelve packages with executable statements. `TablePlan.DistinctValues` covers the statistic estimate plus nil receiver and missing-statistic cases, with no zero-count statement regions. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #869 select-plan-open checkpoint, `/tmp/simpledb-coverage-869.out` measures 100.0% statement coverage in the full module and all twelve packages with executable statements. `SelectPlan.Open` covers matching and nonmatching rows, child-open errors, nil plans, and nil child scans, with no zero-count statement regions. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #871 select-plan-schema checkpoint, `/tmp/simpledb-coverage-871.out` measures 100.0% statement coverage in the full module and all twelve packages with executable statements. `SelectPlan.Schema` covers child-schema delegation, a nil receiver, and a missing child plan, with no zero-count statement regions. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #873 select-plan-block-estimate checkpoint, `/tmp/simpledb-coverage-873.out` measures 100.0% statement coverage in the full module and all twelve packages with executable statements. `SelectPlan.BlocksAccessed` covers forwarding the child estimate plus nil receiver and missing-child cases, with no zero-count statement regions. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #895 project-plan-open checkpoint, `/tmp/simpledb-coverage-895.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `ProjectPlan.Open` covers nil dependencies, child errors, nil child scans, and successful `ProjectScan` delegation. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #897 project-plan-block-estimate checkpoint, `/tmp/simpledb-coverage-897.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `ProjectPlan.BlocksAccessed` covers child delegation and nil receiver/missing-child behavior. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #899 project-plan-output-estimate checkpoint, `/tmp/simpledb-coverage-899.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `ProjectPlan.RecordsOutput` covers child delegation and nil receiver/missing-child behavior. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #901 project-plan-distinct-estimate checkpoint, `/tmp/simpledb-coverage-901.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `ProjectPlan.DistinctValues` covers field-name delegation and nil receiver/missing-child behavior. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #903 query-planner-contract checkpoint, `/tmp/simpledb-coverage-903.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. The interface adds no executable statements; its compile-time assertion verifies the `CreatePlan` signature. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #905 basic-query-planner checkpoint, `/tmp/simpledb-coverage-905.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `BasicQueryPlanner.CreatePlan` covers table ordering, left-associative product composition, selection/projection order, nil inputs, metadata failures, and invalid projections. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #907 basic-query-view-expansion checkpoint, `/tmp/simpledb-coverage-907.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `BasicQueryPlanner.createPlan` covers view lookup, recursive expansion, multiple-view composition, metadata/parse errors, cycle detection, and the physical-table path. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #909 better-query-planner checkpoint, `/tmp/simpledb-coverage-909.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `BetterQueryPlanner.CreatePlan` covers both cost choices, stable source order on ties, left-deep products, and invalid inputs; its reused source resolver covers table and recursive-view resolution. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #911 update-planner-contract checkpoint, `/tmp/simpledb-coverage-911.out` measures 100.0% statement coverage in the full module and all twelve executable packages. `UpdatePlanner` is interface-only and adds no executable statements; a compile-time test double implements all six parsed command methods. The full uncached suite, race tests, vet, docs, Draw.io/SVG XML, and diff checks pass.

At the Issue #913 basic-update-delete checkpoint, `/tmp/simpledb-coverage-913.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions or functions. `BasicUpdatePlanner.ExecuteDelete` covers matching and unmatched rows, metadata and scan-open failures, read-only/nil scans, deletion and iteration failures, and cleanup error joining. Uncached tests, race, vet, docs, Draw.io/SVG XML, and diff checks pass.

At the Issue #915 basic-update-modify checkpoint, `/tmp/simpledb-coverage-915.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `BasicUpdatePlanner.ExecuteModify` covers matching and unmatched rows, empty predicates, expression evaluation, target writes, nil inputs, metadata/scan-open failures, iteration/evaluation/write failures, and cleanup error joining. Uncached tests, race, vet, docs, Draw.io/SVG XML, and diff checks pass.

At the Issue #917 basic-update-insert checkpoint, `/tmp/simpledb-coverage-917.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `BasicUpdatePlanner.ExecuteInsert` covers ordered field writes, invalid insert data, nil inputs, metadata/layout and scan-open failures, insert/value-write failures, read-only/nil scans, and cleanup error joining. Uncached tests, race, vet, docs, Draw.io/SVG XML, and diff checks pass.

At the Issue #919 basic-update-create-table checkpoint, `/tmp/simpledb-coverage-919.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `BasicUpdatePlanner.ExecuteCreateTable` covers metadata delegation, schema field order/type/length, transaction identity, zero affected rows, nil inputs, an empty table name, and wrapped metadata errors. Uncached tests, race, vet, docs, Draw.io/SVG XML, and diff checks pass.

At the Issue #921 basic-update-create-view checkpoint, `/tmp/simpledb-coverage-921.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `BasicUpdatePlanner.ExecuteCreateView` covers metadata delegation of the rendered SELECT definition and original transaction, zero affected rows, nil inputs, empty names, unset definitions, and wrapped metadata errors. Uncached tests, race, vet, docs, Draw.io/SVG XML, and diff checks pass.

At the Issue #923 basic-update-create-index checkpoint, `/tmp/simpledb-coverage-923.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `BasicUpdatePlanner.ExecuteCreateIndex` covers all metadata argument forwarding, original transaction identity, zero affected rows, nil inputs, empty index/table/field names, and wrapped metadata errors. Uncached tests, race, vet, docs, Draw.io/SVG XML, and diff checks pass.

At the Issue #925 index-contract checkpoint, `/tmp/simpledb-coverage-925.out` preserves 100.0% statement coverage in the full module and every executable package; `internal/index` contains only the interface contract and correctly reports no executable statements. Its compile-time conformance probe verifies all six signatures and the explicit error return shapes. Uncached tests, race, vet, docs, Draw.io/SVG XML, and diff checks pass.

At the Issue #927 static-hash-bucket checkpoint, `/tmp/simpledb-coverage-927.out` measures 100.0% statement coverage in the full module and every executable package, with no zero-count regions. `index/hash.BucketNumber` covers integer and Java-compatible UTF-16 string hashes, negative-remainder normalization, bucket bounds, and unset values. Uncached tests, race, vet, docs, Draw.io/SVG XML, and diff checks pass.

At the Issue #929 static-hash-search-cost checkpoint, `/tmp/simpledb-coverage-929.out` measures 100.0% statement coverage in the full module and every executable package. `index/hash.SearchCost` covers empty, sub-bucket, exact-bucket, truncated, multi-bucket, and records-per-block-independent estimates. Uncached tests, race, vet, docs, gofmt, and diff checks pass.

At the Issue #931 static-hash-index-initialization checkpoint, `/tmp/simpledb-coverage-931.out` measures 100.0% statement coverage in every executable package and every executable function. `hash.NewHashIndex` covers retaining the transaction, index name, and layout while leaving the search key unset and scan unopened without transaction I/O. Uncached tests, race, vet, docs, gofmt, and diff checks pass.

At the Issue #933 static-hash-index-close checkpoint, `/tmp/simpledb-coverage-933.out` measures 100.0% statement coverage in every executable package and every executable function. `hash.HashIndex.Close` covers an absent scan, successful unpin, clearing and repeated close, plus wrapped unpin failure with retryable state. Uncached tests, race, vet, docs, gofmt, and diff checks pass.

At the Issue #935 static-hash-index-position checkpoint, `/tmp/simpledb-coverage-935.out` measures 100.0% statement coverage in every executable package and function. `HashIndex.BeforeFirst` covers nil receiver/dependencies, unset key, Java-compatible bucket table naming, close-before-open ordering, close failure preservation, and table size/pin failures. Uncached tests, race, vet, docs, gofmt, and diff checks pass.

At the Issue #937 static-hash-index-iteration checkpoint, `/tmp/simpledb-coverage-937.out` measures 100.0% statement
coverage in every executable package and function. `HashIndex.Next` covers integer and varchar entry reads, collision
skipping, repeated matches, exhaustion, mixed key types, invalid scan/key/layout/schema/field state, and wrapped scan,
slot-flag, and field-read errors. Uncached tests, race, vet, docs, Draw.io/SVG XML, gofmt, and diff checks pass.

At the Issue #939 static-hash-index-RID checkpoint, `/tmp/simpledb-coverage-939.out` measures 100.0% statement
coverage in every executable package and function. `HashIndex.GetDataRID` covers current block/slot conversion, absent
scan/current row, nil or invalid schema fields, non-integer RID columns, and wrapped block/id reads. Uncached tests,
race, vet, docs, existing Draw.io/SVG XML, gofmt, and diff checks pass.

At the Issue #941 static-hash-index-insert checkpoint, `/tmp/simpledb-coverage-941.out` measures 100.0% statement
coverage in every executable package and function. `HashIndex.Insert` covers integer/varchar values, existing and new
bucket tables, field writes, invalid dependencies/layout/key/RID ranges, bucket open/reposition errors, slot-allocation
and write failures, and retained scan state. Uncached tests, race, vet, docs, Draw.io/SVG XML, gofmt, and diff checks pass.

At the Issue #943 static-hash-index-delete checkpoint, `/tmp/simpledb-coverage-943.out` measures 100.0% statement
coverage in every executable package and function. `HashIndex.Delete` covers collision skipping, exact key/RID deletion,
missing key/RID no-ops, invalid state, and wrapped bucket-open, iteration, RID-read, and slot-delete failures. Uncached
tests, race, vet, docs, existing Draw.io/SVG XML, gofmt, and diff checks pass.

At the Issue #945 index-select-scan-position checkpoint, `/tmp/simpledb-coverage-945.out` measures 100.0% statement
coverage in every executable package and function. `IndexSelectScan` covers constructor validation/initial positioning,
repeat positioning, unset keys, nil receivers/dependencies, and wrapped index-position failures. Uncached tests, race,
vet, docs, existing Draw.io/SVG XML, gofmt, and diff checks pass.

At the Issue #947 index-select-scan-next checkpoint, `/tmp/simpledb-coverage-947.out` measures 100.0% statement
coverage in every executable package and function. `IndexSelectScan.Next` covers multiple RID moves, exhaustion without
movement, nil/incomplete state, index-next/RID-read failures, and table-page movement errors. Uncached tests, race,
vet, docs, existing Draw.io/SVG XML, gofmt, and diff checks pass.

At the Issue #949 index-select-scan-integer-accessor checkpoint, `/tmp/simpledb-coverage-949.out` measures 100.0%
statement coverage in every executable package and function. `IndexSelectScan.GetInt` covers successful forwarding,
nil receiver/table-scan validation, and wrapped table-read errors. The full uncached test suite, race tests, vet, docs,
gofmt, and diff checks pass.

At the Issue #951 index-select-scan-string-accessor checkpoint, `/tmp/simpledb-coverage-951.out` measures 100.0%
statement coverage in every executable package and function. `IndexSelectScan.GetString` covers successful forwarding,
nil receiver/table-scan validation, and wrapped table-read errors. The full uncached test suite, race tests, vet, docs,
gofmt, and diff checks pass.

At the Issue #953 index-select-scan-value-accessor checkpoint, `/tmp/simpledb-coverage-953.out` measures 100.0%
statement coverage in every executable package and function. `IndexSelectScan.GetVal` covers integer and varchar
constants, nil receiver/table-scan validation, and wrapped table-read failures. The full uncached test suite, race
tests, vet, docs, gofmt, and diff checks pass.

At the Issue #955 index-select-scan-field-membership checkpoint, `/tmp/simpledb-coverage-955.out` measures 100.0%
statement coverage in every executable package and function. `IndexSelectScan.HasField` covers present integer/varchar
fields, an absent field, and false results for nil or missing data-scan state. The full uncached test suite, race tests,
vet, docs, gofmt, and diff checks pass.

At the Issue #957 index-select-scan-close checkpoint, `/tmp/simpledb-coverage-957.out` measures 100.0% statement
coverage in every executable package and function. `IndexSelectScan.Close` covers index-before-table order, both close
failures, nil receiver, and absent or partial resources. The full uncached test suite, race tests, vet, docs, gofmt,
and diff checks pass.

At the Issue #959 index-info-open checkpoint, `/tmp/simpledb-coverage-959.out` measures 100.0% statement coverage in
every executable package and function. `IndexInfo.Open` is exercised with integer and varchar index layouts, confirms
that construction itself does not open a bucket, then round-trips a RID through the correctly named static hash bucket.
The full uncached test suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #961 index-select-plan-schema checkpoint, `/tmp/simpledb-coverage-961.out` measures 100.0% statement
coverage in every executable package and function. `IndexSelectPlan.Schema` covers schema identity delegation and nil
plan/child states; constructor tests also verify the retained index metadata and search key. The full uncached test
suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #963 index-select-plan-open checkpoint, `/tmp/simpledb-coverage-963.out` measures 100.0% statement
coverage in every executable package and function. `IndexSelectPlan.Open` covers successful positioned scan creation,
dependency/key validation, child-open failures with partial-scan cleanup, nil/incompatible child scans, and joined
positioning/table-cleanup failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #965 index-select-plan-blocks-accessed checkpoint, `/tmp/simpledb-coverage-965.out` measures 100.0%
statement coverage in every executable package and function. `IndexSelectPlan.BlocksAccessed` covers index traversal plus
matching-row estimates, nil plan/metadata, metadata estimate errors, and signed 32-bit addition overflow. The full
uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #967 index-select-plan-records-output checkpoint, `/tmp/simpledb-coverage-967.out` measures 100.0%
statement coverage in every executable package and function. `IndexSelectPlan.RecordsOutput` covers metadata delegation
without a child plan and zero estimates for nil plan or missing metadata. The full uncached suite, race tests, vet,
docs, gofmt, and diff checks pass.

At the Issue #969 index-select-plan-distinct-values checkpoint, `/tmp/simpledb-coverage-969.out` measures 100.0%
statement coverage in every executable package and function. `IndexSelectPlan.DistinctValues` covers indexed-field and
other-field estimates from the index metadata, ignores conflicting child-plan estimates, and returns zero for nil plan
or missing metadata. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #971 index-join-scan-initial-position checkpoint, `/tmp/simpledb-coverage-971.out` measures 100.0%
statement coverage in every executable package and function. `IndexJoinScan` covers initial and repeated positioning,
empty left inputs, missing dependencies, and wrapped errors from the left scan, join-value read, and index. The full
uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #973 index-join-scan-next checkpoint, `/tmp/simpledb-coverage-973.out` measures 100.0% statement coverage
in every executable package and function. `IndexJoinScan.Next` covers multiple matches across left keys, final
exhaustion, empty input, invalid state, and wrapped errors from index iteration/RID access, right-side movement, left
iteration/key access, and index repositioning. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #975 index-join-scan-integer-access checkpoint, `/tmp/simpledb-coverage-975.out` measures 100.0% statement
coverage in every executable package and function. `IndexJoinScan.GetInt` covers right-table preference, left-scan
fallback, invalid state, and wrapped errors from either selected scan. The full uncached suite, race tests, vet, docs,
gofmt, and diff checks pass.

At the Issue #977 index-join-scan-value-access checkpoint, `/tmp/simpledb-coverage-977.out` measures 100.0% statement
coverage in every executable package and function. `IndexJoinScan.GetVal` covers right-side integer and string values,
left-scan fallback, invalid state, and wrapped errors from either selected scan. The full uncached suite, race tests, vet,
docs, gofmt, and diff checks pass.

At the Issue #979 index-join-scan-string-access checkpoint, `/tmp/simpledb-coverage-979.out` measures 100.0% statement
coverage in every executable package and function. `IndexJoinScan.GetString` covers right-table preference, left-scan
fallback, invalid state, and wrapped errors from either selected scan. The full uncached suite, race tests, vet, docs,
gofmt, and diff checks pass.

At the Issue #981 index-join-scan-field-membership checkpoint, `/tmp/simpledb-coverage-981.out` measures 100.0% statement
coverage in every executable package and function. `IndexJoinScan.HasField` covers right-side precedence, left-side
fallback, absent fields, invalid state, and right-side short-circuiting. The full uncached suite, race tests, vet, docs,
gofmt, and diff checks pass.

At the Issue #983 index-join-scan-close checkpoint, `/tmp/simpledb-coverage-983.out` measures 100.0% statement coverage
in every executable package and function. `IndexJoinScan.Close` covers Java-order resource release, all three joined
errors, partial resources, and nil state. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #985 index-join-plan-schema checkpoint, `/tmp/simpledb-coverage-985.out` measures 100.0% statement coverage
in every executable package and function. `NewIndexJoinPlan` and `IndexJoinPlan.Schema` cover ordered field/type copying,
retained plan state, nil child plans and schemas, and nil receiver handling. The full uncached suite, race tests, vet,
docs, gofmt, and diff checks pass.

At the Issue #893 project-plan-schema checkpoint, `/tmp/simpledb-coverage-893.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `NewProjectPlan` covers nil dependencies, valid ordered fields, empty projections, and missing-field errors; `ProjectPlan.Schema` covers both ordinary and nil receivers. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #891 product-plan-distinct-estimate checkpoint, `/tmp/simpledb-coverage-891.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `ProductPlan.DistinctValues` covers left/right field routing, left-side duplicate-name precedence, nil dependencies, missing schemas, and unknown fields. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #889 product-plan-output-estimate checkpoint, `/tmp/simpledb-coverage-889.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `ProductPlan.RecordsOutput` covers the multiplication, nil inputs, and signed 32-bit overflow. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #887 product-plan-block-estimate checkpoint, `/tmp/simpledb-coverage-887.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count regions. `ProductPlan.BlocksAccessed` covers the nested-loop formula, nil inputs, and signed 32-bit multiplication/addition overflow. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #885 product-plan-open checkpoint, `/tmp/simpledb-coverage-885.out` measures 100.0% statement coverage in the full module and all twelve executable packages, with no zero-count statement regions. `ProductPlan.Open` covers child-plan validation, left/right open errors, missing child scans, product construction cleanup, product-scan initialization failures, and cleanup errors. The full uncached suite, race tests, vet, docs, diagram XML, and diff checks pass.

At the Issue #987 index-join-plan-open checkpoint, `/tmp/simpledb-coverage-987.out` measures 100.0% statement coverage in every executable package and function. `IndexJoinPlan.Open` covers successful initialized scans, missing and typed-nil dependencies, incompatible right scan types, partial child-open failures, left positioning/advancement/join-value errors, index positioning errors, and cleanup failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #989 index-join-plan-blocks-accessed checkpoint, `/tmp/simpledb-coverage-989.out` measures 100.0% statement coverage in every executable package and function. `IndexJoinPlan.BlocksAccessed` covers the index-join formula, nil receiver/children/metadata, index-estimate failure, and Java-compatible signed 32-bit overflow. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #991 index-join-plan-records-output checkpoint, `/tmp/simpledb-coverage-991.out` measures 100.0% statement coverage in every executable package and function. `IndexJoinPlan.RecordsOutput` covers the left-record/index-record product, nil receiver/left/index metadata, right-plan independence, and Java-compatible signed 32-bit overflow. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #993 index-join-plan-distinct-values checkpoint, `/tmp/simpledb-coverage-993.out` measures 100.0% statement coverage in every executable package and function. `IndexJoinPlan.DistinctValues` covers left/right routing, left-side duplicate-name precedence, unknown-field fallback, missing plans/schemas, and nil receiver handling. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #995 B-tree-page-open checkpoint, `/tmp/simpledb-coverage-995.out` measures 100.0% statement coverage in every executable package and function. `btree.NewPage` covers one successful pin and state retention, nil and typed-nil transaction rejection, missing layout/schema, invalid block identifiers, and wrapped pin failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #997 B-tree-slot-search checkpoint, `/tmp/simpledb-coverage-997.out` measures 100.0% statement coverage in every executable package and function. `Page.FindSlotBefore` covers integer and Java UTF-16 varchar lower-bound search, duplicates and boundary positions, malformed state/layout/key/count, offset overflow, and wrapped page-header and key-read errors. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #999 B-tree-page-close checkpoint, `/tmp/simpledb-coverage-999.out` measures 100.0% statement coverage in every executable package and function. `Page.Close` covers successful and repeated close, absent state, nil and typed-nil dependencies, wrapped unpin failure, retry, and rejection of slot searches after successful close. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1000 B-tree-page-capacity checkpoint, `/tmp/simpledb-coverage-1000.out` measures 100.0% statement coverage in every executable package and function. `Page.IsFull` covers fitting and boundary next slots, invalid and closed state, negative record counts, invalid block sizes, checked slot-position overflow, and wrapped count/block-size read failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1002 B-tree-page-data-value checkpoint, `/tmp/simpledb-coverage-1002.out` measures 100.0% statement coverage in every executable package and function. `Page.GetDataVal` covers integer and varchar constants, layout-derived field positions, invalid and closed state, missing or malformed schema metadata, unsupported types, negative and overflowing slots, and wrapped read failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1004 B-tree-page-flag checkpoint, `/tmp/simpledb-coverage-1004.out` measures 100.0% statement coverage in every executable package and function. `Page.GetFlag` covers signed flag values, the page-header offset, independence from layout metadata and pin changes, invalid and typed-nil state, and wrapped read failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1006 B-tree-page-flag-write checkpoint, `/tmp/simpledb-coverage-1006.out` measures 100.0% statement coverage in every executable package and function. `Page.SetFlag` covers signed values, the logged header write at offset zero, independence from layout metadata and reads, invalid and typed-nil state, and wrapped write failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1008 B-tree-page-record-count checkpoint, `/tmp/simpledb-coverage-1008.out` measures 100.0% statement coverage in every executable package and function. `Page.GetNumRecs` covers signed count values, the page-header offset, independence from layout metadata and pin changes, invalid and typed-nil state, and wrapped read failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1010 B-tree-page-record-count-write checkpoint, `/tmp/simpledb-coverage-1010.out` measures 100.0% statement coverage in every executable package and function. `Page.setNumRecs` covers signed header writes, the count offset, transaction logging, independence from layout metadata and reads, invalid and typed-nil state, and wrapped write failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1012 B-tree-directory-child-number checkpoint, `/tmp/simpledb-coverage-1012.out` measures 100.0% statement coverage in every executable package and function. `Page.GetChildNum` covers the signed child value, layout-derived slot position, integer-field validation, malformed and overflowing metadata, nil/typed-nil/closed state, and wrapped transaction reads. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1014 B-tree-leaf-record-identifier checkpoint, `/tmp/simpledb-coverage-1014.out` measures 100.0% statement coverage in every executable package and function. `Page.GetDataRID` covers the block-then-id field reads, layout-derived slot offsets, invalid and typed-nil state, missing/wrong field metadata, negative/overflowing slots, wrapped errors from both reads, and no partial result. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1016 B-tree-page-data-value-write checkpoint, `/tmp/simpledb-coverage-1016.out` measures 100.0% statement coverage in every executable package and function. `Page.SetDataVal` covers logged integer and varchar writes, layout-derived positions, int32 offset overflow, invalid and typed-nil state, absent/unsupported/malformed metadata, unset or mismatched constants, and wrapped write failures. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1018 B-tree-directory-child-number-write checkpoint, `/tmp/simpledb-coverage-1018.out` measures 100.0% statement coverage in every executable package and function. `Page.SetChildNum` covers signed logged writes, layout-derived slot offsets, missing/noninteger/malformed field metadata, nil and typed-nil state, negative/overflowing slots, int32 write-offset overflow, no read/pin changes, and wrapped errors. The full uncached suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1020 B-tree-leaf-record-identifier-write checkpoint, `/tmp/simpledb-coverage-1020.out` measures 100.0% statement coverage in every executable package and function. `Page.SetDataRID` covers both layout-derived logged writes, signed values, validation before mutation, invalid and typed-nil state, absent/noninteger/malformed fields, overflowing slots and identifiers, and wrapped failures from either write. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1022 B-tree-directory-insert checkpoint, `/tmp/simpledb-coverage-1022.out` measures 100.0% statement coverage in every executable package and function. `Page.InsertDir` covers integer and varchar entries, right-to-left full-record shifts, append at the end, metadata/value/count/slot/offset validation before mutation, and wrapped count/copy/count-write/key-write/child-write errors. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1024 B-tree-leaf-insert checkpoint, `/tmp/simpledb-coverage-1024.out` measures 100.0% statement coverage in every executable package and function. `Page.InsertLeaf` covers complete-record shifts, typed keys, block-and-id RID writes in order, validation before mutation, and wrapped storage failures across count, copying, count update, key write, and both RID fields. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1026 B-tree-page-delete checkpoint, `/tmp/simpledb-coverage-1026.out` measures 100.0% statement coverage in every executable package and function. `Page.Delete` covers ascending left-shifts of complete records, deletion without copies at the final slot, pre-mutation validation, checked offsets, and wrapped count/read/copy/write failures. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1028 B-tree-page-format checkpoint, `/tmp/simpledb-coverage-1028.out` measures 100.0% statement coverage in every executable package and function. `Page.Format` covers header initialization, complete-slot defaults for integer/varchar schemas, logging-disabled writes, pin independence, invalid block/layout/size inputs, offset bounds, and wrapped size/write failures. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1030 B-tree-page-append checkpoint, `/tmp/simpledb-coverage-1030.out` measures 100.0% statement coverage in every executable package and function. `Page.AppendNew` covers same-file allocation followed by formatting, pre-append validation, wrapped append/format failures, unpublished partial formatting, invalid returned block IDs, and unchanged pin ownership. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1032 B-tree-page-split checkpoint, `/tmp/simpledb-coverage-1032.out` measures 100.0% statement coverage in every executable package and function. `Page.Split` covers integer/varchar suffix transfer, zero/full split boundaries, capacity and offset validation before append, logged record copying, source deletion and destination counts, pin cleanup, joined operation/close failures, and wrapped append/pin/read/write/flag/unpin errors. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1034 B-tree-directory-child-selection checkpoint, `/tmp/simpledb-coverage-1034.out` measures 100.0% statement coverage in every executable package and function. `Page.FindChildBlock` covers integer and varchar routing, exact duplicate keys, predecessor and final-key selection, empty and below-minimum directories, invalid page/schema/key metadata, negative child numbers, and wrapped count, key, and child reads. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1036 B-tree-directory-child-opening checkpoint, `/tmp/simpledb-coverage-1036.out` measures 100.0% statement coverage in every executable package and function. `Page.OpenChildDirectory` covers selected same-file child pinning, caller-owned child closure, unchanged parent ownership, non-directory levels, and wrapped flag, child-selection, and pin failures. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1039 B-tree-directory-descent checkpoint, `/tmp/simpledb-coverage-1039.out` measures 100.0% statement coverage in every executable package and function. `Page.FindLeafBlock` covers multi-level selection, level-zero leaf-block return, close-before-pin ordering with at most one active directory pin, retained final-page ownership, invalid/closed pages, negative levels, wrapped flag and child-selection errors, retryable unpin failure, and a failed child pin that leaves the receiver closed. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1041 B-tree-leaf-opening checkpoint, `/tmp/simpledb-coverage-1041.out` measures 100.0% statement coverage in every executable package and function. `Page.OpenLeafPage` covers level-zero validation, pre-close block/layout validation, close-before-pin ordering with no more than one active pin, caller ownership of the leaf page, wrapped level/unpin/pin failures, retryable directory unpin, and a closed receiver after leaf pin failure. The uncached full suite, race tests, vet, docs, diagram XML, gofmt, and diff checks pass.

At the Issue #1043 B-tree-leaf-next-slot checkpoint, `/tmp/simpledb-coverage-1043.out` measures 100.0% statement coverage in every executable package and function. `Page.NextLeafSlot` covers one-slot integer/varchar matching, mismatches, exhaustion, malformed key layouts, invalid and overflowing slot inputs, negative counts, wrapped count/key reads, and unchanged page pin ownership. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1045 B-tree-leaf-overflow-selection checkpoint, `/tmp/simpledb-coverage-1045.out` measures 100.0% statement coverage in every executable package and function. `Page.FindOverflowBlock` covers matching integer/varchar first keys, block zero as a valid target, mismatch and negative-flag no-ops, invalid typed page/key state, malformed layouts, wrapped first-key/flag reads, and unchanged pin ownership. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1047 B-tree-leaf-overflow-opening checkpoint, `/tmp/simpledb-coverage-1047.out` measures 100.0% statement coverage in every executable package and function. `Page.OpenOverflowPage` covers validation before unpinning, same-file targets, close-before-pin ordering with one active pin maximum, caller cleanup, retryable unpin failure, and target-pin failure after the receiver closes. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1049 B-tree-leaf-cursor-initialization checkpoint, `/tmp/simpledb-coverage-1049.out` measures 100.0% statement coverage in every executable package and function. `NewLeaf` covers page pinning, lower-bound slot initialization for integer/varchar keys and empty pages, validation and pin errors, cleanup after positioning failures, and joined position/unpin errors. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1051 B-tree-leaf-cursor-next checkpoint, `/tmp/simpledb-coverage-1051.out` measures 100.0% statement coverage in every executable package and function. `Leaf.Next` covers in-page matches, mismatches, exhaustion, same-key overflow traversal, close-before-pin ownership, no-overflow termination, read and transition errors, and invalid cursor state. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1053 B-tree-leaf-current-RID checkpoint, `/tmp/simpledb-coverage-1053.out` measures 100.0% statement coverage in every executable package and function. `Leaf.GetDataRID` covers the current slot on the original or overflow page, unchanged cursor/page pin state, nil state, and wrapped block/id read errors. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1055 B-tree-leaf-delete checkpoint, `/tmp/simpledb-coverage-1055.out` measures 100.0% statement coverage in every executable package and function. `Leaf.Delete` covers the first matching RID, overflow traversal, no-match termination, wrapped cursor/RID/page-delete errors, and retained cursor/page state on failure. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1057 B-tree-directory-entry checkpoint, `/tmp/simpledb-coverage-1057.out` measures 100.0% statement coverage in every executable package and function. `DirEntry` tests preserve integer/varchar keys and zero/negative block values; the value type performs no I/O. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1059 B-tree-leaf-insert checkpoint, `/tmp/simpledb-coverage-1059.out` measures 100.0% statement coverage in every executable package and function. `Leaf.Insert` covers typed in-page insertion, current-slot movement, nil directory result, full-page rejection, invalid cursor state, and wrapped capacity/record errors. Overflow relocation and page-full splits remain separate behaviors. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1061 B-tree-leaf-cursor-close checkpoint, `/tmp/simpledb-coverage-1061.out` measures 100.0% statement coverage in every executable package and function. `Leaf.Close` covers successful and repeated close, post-close cursor operations, retryable wrapped unpin failure, and nil receiver handling. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1063 B-tree-leaf-minimum-insert checkpoint, `/tmp/simpledb-coverage-1063.out` measures 100.0% statement coverage in every executable package and function. `Leaf.Insert` covers splitting slot zero for a new minimum key, retaining the old right-page flag, marking the new left page as an overflow page, returning its directory separator, and preserving the cursor on errors; the shared key comparator preserves Java UTF-16 varchar order. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1065 B-tree-leaf-duplicate-overflow checkpoint, `/tmp/simpledb-coverage-1065.out` measures 100.0% statement coverage in every executable package and function. `Leaf.Insert` covers the post-insert fullness check, equal integer/varchar key detection, slot-one suffix split, preserved overflow flag, source-to-destination link, nil directory entry, unchanged insertion cursor, mixed-key rejection, wrapped storage errors, and signed 32-bit flag range validation. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1067 B-tree-leaf-mixed-split checkpoint, `/tmp/simpledb-coverage-1067.out` measures 100.0% statement coverage in every executable package and function. `Leaf.Insert` covers midpoint selection, moving right past the first duplicate group, moving left to the midpoint group's first value, integer/varchar separators, a new-page flag of -1, unchanged source flag and cursor, wrapped boundary-read/split errors, and signed 32-bit directory block validation. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1069 B-tree-directory-entry-insertion checkpoint, `/tmp/simpledb-coverage-1069.out` measures 100.0% statement coverage in every executable package and function. `Page.InsertDirectoryEntry` covers key ordering, duplicate insertion before the first equal key, integer and varchar keys, last-slot and empty-page insertion, physical-capacity rejection before mutation, directory-layout/level validation, signed 32-bit child-block bounds, and wrapped page metadata/read/write failures. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1071 B-tree-directory-split checkpoint, `/tmp/simpledb-coverage-1071.out` measures 100.0% statement coverage in every executable package and function. `Page.SplitDirectoryPage` covers integer and varchar separator entries, even/odd midpoint boundaries, preserved directory levels and record order, physical-fullness checks before allocation, invalid layout/level/count/block-size rejection, child-range checks, and wrapped append/pin/transfer/unpin errors. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1073 B-tree-directory-insert-split checkpoint, `/tmp/simpledb-coverage-1073.out` measures 100.0% statement coverage in every executable package and function. `Page.InsertDirectoryEntryAndSplit` covers insertion while capacity remains, splitting after the final slot is filled, integer and varchar ordering, returned separator, initially full-page refusal before mutation, and wrapped post-insert count/capacity/split errors. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1075 B-tree-directory-split-propagation checkpoint, `/tmp/simpledb-coverage-1075.out` measures 100.0% statement coverage in every executable package and function. `Page.InsertDirectoryEntryRecursive` covers level-zero insert/split, multi-level descent and separator propagation, child cleanup, parent splits, invalid levels, child-open/insert/close failures, joined errors, and parent split append failure. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1077 B-tree-directory-root-growth checkpoint, `/tmp/simpledb-coverage-1077.out` measures 100.0% statement coverage in every executable package and function. `Page.MakeNewRoot` covers integer and varchar roots, level-zero and internal root growth, block-zero retention, ordered root entries, validation before append, and wrapped metadata, append, pin, split, entry-write, and level-write errors. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1079 B-tree-directory-root-initialization checkpoint, `/tmp/simpledb-coverage-1079.out` measures 100.0% statement coverage in every executable package and function. `Page.InitializeDirectoryRoot` covers the integer and varchar minimum-key sentinels, level-zero formatting, the initial child-block-zero entry, validation before page writes, and wrapped format and logged-entry write failures. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1081 B-tree-leaf-page-initialization checkpoint, `/tmp/simpledb-coverage-1081.out` measures 100.0% statement coverage in every executable package and function. `Page.InitializeLeafPage` covers integer and varchar leaf layouts, initial and overflow block numbers, flag -1, an empty record count, default field values, validation before writes, and wrapped format errors. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1083 B-tree-leaf-file-initialization checkpoint, `/tmp/simpledb-coverage-1083.out` measures 100.0% statement coverage in every executable package and function. `ensureLeafFile` covers existing and empty files, integer/varchar page formats, invalid file/layout inputs, file-size/append/pin/format/unpin errors, and joined initialization/cleanup causes. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1085 B-tree-directory-file-initialization checkpoint, `/tmp/simpledb-coverage-1085.out` measures 100.0% statement coverage in every executable package and function. `ensureDirectoryFile` covers existing and empty files, integer/varchar minimum-key sentinels, block-zero root formatting, invalid file/layout inputs, file-size/append/pin/format/entry-write/unpin errors, and joined initialization/cleanup causes. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1087 B-tree-directory-layout checkpoint, `/tmp/simpledb-coverage-1087.out` measures 100.0% statement coverage in every executable package and function. `directoryLayoutFromLeaf` covers integer and varchar layouts, varchar length preservation, exact field order and offsets, source-schema immutability, and invalid leaf-layout metadata. The full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1091 B-tree-index-initialization checkpoint, `/tmp/simpledb-coverage-1091.out` measures 100.0% statement coverage in every executable package and function. `NewBTreeIndex` covers retained index state, Java file naming and initialization order, existing files, invalid inputs before file access, and wrapped leaf/directory initialization failures. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1093 B-tree-index-positioning checkpoint, `/tmp/simpledb-coverage-1093.out` measures 100.0% statement coverage in every executable package, function, and the module total. `BTreeIndex.BeforeFirst` covers integer and varchar key validation, selected-leaf positioning, closing the previous cursor before directory traversal, one-page pin ownership, wrapped directory/pin/leaf-positioning errors, and retryable cleanup when unpin fails. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1095 B-tree-index-next checkpoint, `/tmp/simpledb-coverage-1095.out` measures 100.0% statement coverage in every executable package, function, and the module total. `BTreeIndex.Next` covers a matching entry, clean exhaustion, nil and unpositioned indexes without I/O, closed cursors, and wrapped leaf-advance errors. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1097 B-tree-index-current-RID checkpoint, `/tmp/simpledb-coverage-1097.out` measures 100.0% statement coverage in every executable package, function, and the module total. `BTreeIndex.GetDataRID` covers the positioned leaf RID, unchanged cursor/page ownership, nil and unpositioned indexes without I/O, and wrapped page-read failures. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1099 B-tree-index-close checkpoint, `/tmp/simpledb-coverage-1099.out` measures 100.0% statement coverage in every executable package, function, and the module total. `BTreeIndex.Close` covers nil receivers, leaf-before-directory cleanup, idempotent repeated close, joined simultaneous unpin failures, retention of failed page handles, and successful cleanup retry. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1101 B-tree-index-insert checkpoint, `/tmp/simpledb-coverage-1101.out` measures 100.0% statement coverage in every executable package, function, and the module total. `BTreeIndex.Insert` covers inserts without a split, leaf split propagation, block-zero root growth, nil and invalid-key rejection, joined leaf mutation/cleanup failures, root pin failure, and joined directory mutation/root cleanup failures with retry. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1103 B-tree-index-delete checkpoint, `/tmp/simpledb-coverage-1103.out` measures 100.0% statement coverage in every executable package, function, and the module total. `BTreeIndex.Delete` covers deleting a matching value/RID pair, leaving a missing RID unchanged, nil and invalid-key rejection, positioning failures, joined mutation/cleanup causes, and retrying a retained leaf page. The uncached full suite, race tests, vet, docs, gofmt, and diff checks pass.

At the Issue #1133 embedded-connector checkpoint, the connector tests cover fresh `driver.Conn` values, canceled contexts, invalid receivers, `driver.Driver.Open`, and transaction completion through `sql.OpenDB`. The independent Go coverage workflow continues to enforce 100.0% statement coverage for the full module.

At the Issue #1135 embedded-statement checkpoint, the connection tests cover statement preparation without SQL parsing, exact query retention, unknown input count, idempotent close, unsupported Exec/Query, and nil/closed statement errors. The independent Go coverage workflow continues to enforce 100.0% statement coverage for the full module.

At the Issue #1137 embedded-update checkpoint, `database/sql` tests execute CREATE TABLE and INSERT through the active transaction, verify affected-row counts and committed schema visibility, then verify rollback removes a later catalog change. Error tests cover missing transaction/manager state, unsupported arguments, malformed SQL, and planner failure. The independent coverage workflow continues to enforce 100.0% module statement coverage.

At the Issue #1139 database-autocommit checkpoint, `database/sql.DB.Exec` tests verify CREATE TABLE and INSERT commit their own transactions, report affected rows, and keep the separate `sql.Tx.Exec` path caller-owned. Error coverage exercises invalid state and arguments, planner and parse failures, commit failure, and rollback failure. The independent Go coverage workflow continues to enforce 100.0% module statement coverage.

At the Issue #1141 execution-context checkpoint, `ExecContext` tests cover nil and canceled contexts before work begins, successful context-aware autocommit, and canceled `sql.Tx.ExecContext` followed by successful caller-owned execution. The independent Go coverage workflow continues to enforce 100.0% module statement coverage.

At the Issue #1143 prepared-statement context checkpoint, tests execute a context-aware prepared update inside `sql.Tx`, verify that a canceled call does not prevent a later successful statement, and confirm rollback remains caller-owned. Direct tests cover canceled/nil contexts and invalid statement state. The independent Go coverage workflow continues to enforce 100.0% module statement coverage.

At the Issue #1145 prepared-statement arity checkpoint, `NumInput` reports zero until placeholder parsing and binding are implemented. The `database/sql` integration test confirms extra arguments are rejected by the standard library before driver execution, while zero-argument updates continue working. The independent Go coverage workflow continues to enforce 100.0% module statement coverage.

At the Issue #1147 embedded-query checkpoint, prepared SELECT statements open plans through the caller-owned transaction and expose projected integer/string columns via `driver.Rows`. Tests cover standard-library row scanning, exhaustion, idempotent cleanup, invalid statement state, planner/scan creation failures, and row-level read failures. The independent coverage workflow continues to enforce 100.0% module statement coverage.

At the Issue #1149 prepared-query context checkpoint, `driver.StmtQueryContext` rejects nil/canceled contexts before planning and delegates valid zero-argument calls through the existing scan/rows path. Tests cancel a query then reuse the same statement and caller-owned transaction. The independent coverage workflow continues to enforce 100.0% module statement coverage.

At the Issue #1151 embedded result-type-name checkpoint, `statementRows` implements `driver.RowsColumnTypeDatabaseTypeName` and reports `VARCHAR` or `INTEGER` in projected-column order without advancing the scan. Tests cover both database/sql's `Rows.ColumnTypes()` path and nil, missing, invalid-index, missing-field, and unsupported-type cases. The coverage profile reports 100.0% statement coverage in every executable package and 100.0% for the module; `go tool cover -func` reports no executable function below 100.0%. The uncached full suite, race suite, vet, docs, gofmt, and diff checks pass.

At the Issue #1153 embedded result-scan-type checkpoint, `statementRows` implements `driver.RowsColumnTypeScanType` and reports `int64` for integer results and `string` for varchar results, matching the values returned to `database/sql`. Direct and `Rows.ColumnTypes().ScanType()` tests cover projected order plus nil, missing, invalid-index, missing-field, and unsupported-type cases. The coverage profile reports 100.0% statement coverage in every executable package and module total; `go tool cover -func` reports every executable function at 100.0% and no uncovered statements. The uncached full suite, race suite, vet, docs, gofmt, and diff checks pass.

At the Issue #1155 embedded result-column-length checkpoint, `statementRows` implements `driver.RowsColumnTypeLength`: varchar results report their declared schema length with `ok == true`, while integer, unsupported, malformed, or out-of-range metadata reports `(0, false)`. Direct and `database/sql.Rows.ColumnTypes().Length()` tests cover varchar and integer projections, nil/missing state, invalid indexes, and negative schema lengths. The coverage profile reports 100.0% statement coverage in every executable package and module total; `go tool cover -func` reports every executable function at 100.0% with no uncovered statements. The uncached full suite, race suite, vet, docs, gofmt, and diff checks pass.

At the Issue #1157 embedded result-nullability checkpoint, `statementRows` implements `driver.RowsColumnTypeNullable`: supported integer and varchar results report known non-nullability because the current record/query value model cannot represent NULL. Direct and `database/sql.Rows.ColumnTypes().Nullable()` tests cover both supported types plus nil, missing, invalid-index, missing-field, and unsupported-type cases. The coverage profile reports 100.0% statement coverage in every executable package and module total; `go tool cover -func` reports every executable function at 100.0% with no uncovered statements. The uncached full suite, race suite, vet, docs, gofmt, and diff checks pass.

At the Issue #1159 prepared-parameter checkpoint, embedded statements count positional `?` markers outside quoted SQL strings and bind ordered `int64`/string values for INSERT, SELECT, and UPDATE. Both legacy `driver.Stmt.Exec`/`Query` and the context-aware entry points are covered. String apostrophes are doubled before the existing parser handles the resulting SQL literal; wrong arity, named values, malformed quoting, and unsupported value types are rejected before planner dispatch. The uncached full suite reports 100.0% statement coverage in every executable package and module total; `go tool cover -func` reports no uncovered executable functions. The race suite, vet, docs, gofmt, diff checks, and diagram/workflow XML audits pass.

At the Issue #1214 chunk-page-ownership checkpoint, `NewChunkScan` tests cover valid inclusive pin ranges, pre-I/O validation, constructor pin failures, and cleanup failures. `ChunkScan.Close` tests cover releasing all pages, joining multiple errors, retaining only failed releases for retry, nil receivers, and idempotent success. The uncached full suite and race suite, vet, docs, gofmt, and diff checks pass; the separate Go coverage workflow continues to enforce 100.0% coverage for every executable package/function and the module total.

At the Issue #1216 chunk in-block traversal checkpoint, `ChunkScan.BeforeFirst` resets the first-page cursor and `Next` advances through occupied slots in that page only. Tests cover occupied/empty slots, exhaustion and repeat exhaustion, rewind without new pins, invalid scan lifecycle state, and wrapped block-size/record-flag read errors. The uncached full suite reports 100.0% statement coverage for every executable package and module total; `go tool cover -func` reports every executable function at 100.0%. The race suite, vet, docs, gofmt, diff check, and ten-workflow one-job audit pass.

At the Issue #1218 chunk block-rollover checkpoint, `ChunkScan.Next` advances after the active page has no remaining occupied slot, skips empty pages, restarts each page at slot -1, and marks exhaustion only at the end of the inclusive range. Tests cover crossing, rewind, stable pins, repeated exhaustion without reads, and a wrapped record-read error on a later page. The uncached full suite reports 100.0% statement coverage for every executable package and module total; `go tool cover -func` reports every executable function at 100.0%. The race suite, vet, docs, gofmt, diff check, and the ten-workflow one-job audit pass.

At the Issue #1220 chunk integer-field checkpoint, `ChunkScan.GetInt` reads from the active page and slot without advancing or repinning. Tests cover first/later-page values, field lookup and wrapped storage failures, plus nil, zero-value, partial, closed, before-first, and exhausted scans that must reject field reads. The uncached full suite reports 100.0% statement coverage for every executable package and module total; `go tool cover -func` reports every executable function at 100.0%. The race suite, vet, docs, gofmt, and diff check pass.

At the Issue #1222 chunk string-field checkpoint, `ChunkScan.GetString` reads the current field through the active page without advancing or repinning. Tests cover first/later-page values, missing fields, wrapped storage failures, and nil, zero-value, partial, closed, before-first, and exhausted scans that must reject string reads. The uncached full suite requires 100.0% statement coverage for every executable package and function; final verification is recorded in the stacked PR.

At the Issue #1224 chunk field-membership checkpoint, `ChunkScan.HasField` reports schema membership without consulting the cursor or touching storage. Tests cover declared/missing/empty names, nil and zero-value scans, closed scans, unchanged page pins, and no field reads. The focused package suite reports 100.0% statement coverage; final full-module coverage is recorded in the stacked PR.

At the Issue #1226 chunk query-value checkpoint, `plan.ChunkValueScan` adapts current integer and varchar values from the active `multibuffer.ChunkScan` to typed query constants without moving the cursor or changing pins. Tests cover repeated reads across the first and later blocks, absent and unsupported schema fields, wrapped integer/string storage errors, nil/zero/uninitialized/partial/before-first/exhausted/closed states, and invalid constructor inputs. The plan package suite reports 100.0% statement coverage; final full-module coverage is recorded in the stacked PR. The adapter remains in `plan` so `multibuffer` does not import `query`.

At the Issue #1228 multibuffer-product initialization checkpoint, `plan.NewMultibufferProductScan` validates inputs, reads `<table>.tbl` size and the buffer budget, selects the first range with `BestFactor`, pins and adapts that range, and rewinds the left scan. Tests cover empty and nonempty relations, invalid inputs, sizing and pin errors, rewind failures, joined cleanup errors, and retryable retained pins. The focused plan package suite reports 100.0% statement coverage; final uncached full-module coverage and repository checks are recorded in the stacked PR.

At the Issue #1230 in-chunk product traversal checkpoint, `MultibufferProductScan.BeforeFirst` and `Next` delegate left-major pairing to `query.ProductScan` without repinning the active chunk. Tests cover multiple right records per left row, reset and stable exhaustion, empty input, left/right scan failures, product-construction failures, and unchanged pins. The focused plan package suite reports 100.0% statement coverage; final uncached full-module coverage and repository checks are recorded in the stacked PR.

At the Issue #1232 multibuffer-product rollover checkpoint, `MultibufferProductScan.Next` releases each exhausted right chunk before pinning the next bounded block range and rewinding the left scan. Tests cover left-major results across chunks, empty earlier chunks, a short final chunk, stable exhaustion, failed-release ordering and retry, later-range pin failure and retry, partial pin cleanup, left-rewind failure and retry, and final-chunk cleanup. The uncached full suite reports 100.0% statement coverage in all 20 executable packages, every executable function, and the module total; interface-only `internal/index` has no statements. The race suite, vet, docs validator, gofmt, diff check, XML parsing of 124 Draw.io and 123 SVG assets, and audit of ten separate one-job workflows pass.
