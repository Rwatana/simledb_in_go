# Chapter 11: JDBC Interfaces

## Chapter focus

This chapter connects the database engine to client programs. It describes the JDBC-facing types and contrasts an
embedded driver, which creates the engine in the application process, with a network driver, which delegates work to a
server. The existing [client-boundary diagram](../diagrams/ch02-client-boundary.svg) shows these two paths; its
[editable Draw.io source](../diagrams/ch02-client-boundary.drawio) is shared with the Chapter 2 concept note.
The initial Go embedded adapter now lives at `internal/jdbc/embedded`; it uses `database/sql/driver` rather than
reproducing the JDBC class hierarchy.

The first Go behavior composes the file, log, and buffer managers from explicit database settings. The next adds an
independent, in-memory metadata facade to each successful manager set. The [manager assembly
diagram](../diagrams/ch11-server-manager-assembly.svg) shows this composition and the low-level manager failure boundary;
the [editable Draw.io source](../diagrams/ch11-server-manager-assembly.drawio) is maintained beside it. Creating the
metadata facade does not create catalog files; catalog initialization is a separate transaction-backed behavior.

The next behavior opens transactions from that shared manager set. The [transaction creation diagram](../diagrams/ch11-server-transaction-creation.svg)
shows the per-server ID and lock state, shared storage dependencies, START logging, and the no-transaction error path;
its [editable Draw.io source](../diagrams/ch11-server-transaction-creation.drawio) is maintained beside it.

`Managers.InitializeCatalogs` adapts a caller-owned transaction to the record-scan operations needed by the metadata
facade. It creates the physical table and field catalog files and registers the table, field, view, and index catalog
layouts. The [catalog initialization diagram](../diagrams/ch11-server-catalog-initialization.svg) shows the adapter
boundary and transaction ownership; its [editable Draw.io source](../diagrams/ch11-server-catalog-initialization.drawio)
is maintained beside it. A successful call leaves commit or rollback to the caller, and failures retain catalog
initialization context.

`Managers.Recover` explicitly performs restart recovery before a reopened manager set is used. It creates a fresh
transaction, asks that transaction to undo unfinished durable history and flush the CHECKPOINT record, then commits
the recovery transaction to release its locks and buffers. The [restart recovery diagram](../diagrams/ch11-server-restart-recovery.svg)
shows the durable history scan, undo, checkpoint, and final commit; its [editable Draw.io source](../diagrams/ch11-server-restart-recovery.drawio)
is kept beside the SVG. Recovery is not hidden inside `NewManagers`, so callers choose when a reopened database is ready
for use; a recovery or final commit error is returned with restart context.

## Java source map

| Java source | Responsibility |
| --- | --- |
| `simpledb/server/SimpleDB.java` | Compose file, log, buffer, metadata, and planner managers; create transactions |
| `simpledb/server/StartServer.java` | Start the network-facing server process |
| `simpledb/jdbc/*Adapter.java` | Supply default behavior for JDBC interfaces |
| `simpledb/jdbc/embedded/EmbeddedDriver.java`, `EmbeddedConnection.java` | Interpret an embedded URL and bind a connection to an in-process database and transaction |
| `simpledb/jdbc/embedded/EmbeddedStatement.java`, `EmbeddedResultSet.java`, `EmbeddedMetaData.java` | Execute through the planner, expose a scan as a result cursor, and describe its columns |
| `simpledb/jdbc/network/NetworkDriver.java`, `NetworkConnection.java`, `NetworkStatement.java`, `NetworkResultSet.java`, `NetworkMetaData.java` | Adapt a network client to the same JDBC-facing contract |
| `simpledb/jdbc/network/Remote*` | Implement the server-side remote objects used by Java RMI, including per-connection transactions and their commit/rollback cycle |

The Java network driver strips `jdbc:simpledb://` from its data-source name and contacts the RMI registry on the
host's default port 1099. The Go `network.ParseNetworkURL` behavior keeps this URL interpretation explicit: it returns
the hostname, rejects URL port overrides and non-host components, and does not dial or create a remote session. The
[network URL diagram](../diagrams/ch11-network-url.svg) shows the parser boundary and the separate caller-owned dial;
its [editable Draw.io source](../diagrams/ch11-network-url.drawio) is maintained beside the SVG.

`network.DialNetworkURL` uses the parsed host to open a context-aware TCP connection on port 1099. It wraps dial
failures with the host while preserving their cause, and the caller owns closing a successful connection. This Go dial
is only the socket-opening boundary; it does not implement the Java RMI handshake, SQL request protocol, or remote
session lifecycle. The [network dial diagram](../diagrams/ch11-network-dial.svg) shows validation, address construction,
and connection ownership; its [editable Draw.io source](../diagrams/ch11-network-dial.drawio) is kept beside the SVG.

`ConnectRPCSession` sends `SimpleDB.Connect` through a caller-supplied `rpc.Client` and retains the returned transaction ID
on a `RemoteSession`. The client and underlying transport remain caller-owned; a failed RPC returns no session. The
[network RPC client-session diagram](../diagrams/ch11-network-rpc-client-session.svg) shows the request, server-side
START boundary, returned ID, and ownership; its [editable Draw.io source](../diagrams/ch11-network-rpc-client-session.drawio)
is kept beside the SVG. `RemoteSession.Commit` sends `SimpleDB.Commit` on that client and replaces its retained ID with
the server's successor ID only after a successful response; a failed call preserves the previous ID and does not close
the client.

`network.ServeConnection` registers a small gob-based `net/rpc` service for one accepted connection. A successful
`SimpleDB.Connect` creates one transaction through the shared manager set, retains it on that connection's service, and
returns its transaction ID. The [network RPC session diagram](../diagrams/ch11-network-rpc-session.svg) shows the connect
request, START-log boundary, and per-connection transaction; its [editable Draw.io source](../diagrams/ch11-network-rpc-session.drawio)
is maintained beside the SVG. This is a Go-native server boundary and does not implement Java RMI framing. `DriverService.Commit` commits the retained transaction and starts its successor on the same connection, matching the
Java server connection cycle without exposing RMI. Its [network commit diagram](../diagrams/ch11-network-rpc-commit.svg)
shows the durable COMMIT marker followed by the next START record; its [editable Draw.io source](../diagrams/ch11-network-rpc-commit.drawio)
is maintained beside the SVG. `DriverService.Rollback` also completes the current transaction and opens the next one,
using the distinct durable rollback path. The [network RPC rollback diagram](../diagrams/ch11-network-rpc-rollback.svg)
shows the ROLLBACK record and successor START; its [editable Draw.io source](../diagrams/ch11-network-rpc-rollback.drawio)
is maintained beside the SVG. `DriverService.Close` commits the active transaction and marks the RPC session closed
without starting a successor. The [network RPC close diagram](../diagrams/ch11-network-rpc-close.svg) shows the final
COMMIT and the absence of a next START; its [editable Draw.io source](../diagrams/ch11-network-rpc-close.drawio) is
maintained beside the SVG. The caller still owns the transport `net.Conn`.
`DriverService.ExecuteUpdate` parses and dispatches a SQL update using that same retained transaction, returning the
affected-row count without committing or replacing the transaction. Its [network RPC update diagram](../diagrams/ch11-network-rpc-exec-update.svg)
shows the manager/parser/planner boundary and caller-owned completion; its [editable Draw.io source](../diagrams/ch11-network-rpc-exec-update.drawio)
is kept beside the SVG. Parse and planner errors return through RPC with a zero server-side result; remote SELECT is
handled by `ExecuteQuery` below, while client-side JDBC adaptation remains separate.
`DriverService.ExecuteQuery` materializes a SELECT scan into gob-friendly scalar values, preserving schema column order
and the session transaction. Its [network RPC query diagram](../diagrams/ch11-network-rpc-query.svg) shows planning,
scan cleanup, typed rows, and explicit transaction ownership; its [editable Draw.io source](../diagrams/ch11-network-rpc-query.drawio)
is kept beside the SVG. The caller may then commit, roll back, or close the session independently of result materialization.

## Go design and current status

The Go repository has lower-level file, log, buffer, transaction, metadata, query, and planning packages, plus an initial
embedded connection adapter. `internal/server.NewManagers` now creates the file manager, then the log manager for `simpledb.log`,
then the buffer manager using the supplied directory, block size, and buffer count, and finally a fresh in-memory metadata
facade. It returns the complete manager set only after the fallible storage managers succeed; initialization failures are
wrapped with the failing stage and no partial set is returned. Every successful server manager set owns an independent
metadata facade. The facade's catalog file initialization is a separate transaction-backed operation. The caller can
invoke `Managers.Recover` after reopening the durable database; this creates and commits a dedicated recovery
transaction. `NewManagers` does not recover implicitly. Planner selection and ordinary transaction lifecycle remain
separate behaviors.

Each successful `Managers` set also exposes a `plan.QueryPlanner`, currently composed as a
`plan.BasicQueryPlanner` backed by the same metadata facade. Callers pass the active transaction to
`CreatePlan`, so the planner can resolve the table schema and statistics through that transaction without owning
transaction lifecycle. The [query planner composition diagram](../diagrams/ch11-query-planner-composition.svg)
shows this dependency and the returned query plan; its [editable Draw.io source](../diagrams/ch11-query-planner-composition.drawio)
is maintained beside it. `Managers.SetQueryPlannerStrategy` can replace this default with a basic or cost-aware
`plan.BetterQueryPlanner`; both choices receive the manager set's existing metadata facade. The
[query-planner strategy diagram](../diagrams/ch11-query-planner-strategy.svg) shows that selection while keeping the
metadata and transaction shared; its [editable Draw.io source](../diagrams/ch11-query-planner-strategy.drawio) is
maintained beside the SVG. The zero-valued strategy preserves the basic default, and a rejected selection leaves the
current planner intact.

Each manager set also exposes a `plan.UpdatePlanner`, currently composed as a `plan.BasicUpdatePlanner` backed by
that same metadata facade. Callers pass the transaction to update operations, so the planner can create catalog rows
without taking transaction ownership. The [update planner composition diagram](../diagrams/ch11-update-planner-composition.svg)
shows this dependency and the CREATE TABLE metadata path; its [editable Draw.io source](../diagrams/ch11-update-planner-composition.drawio)
is maintained beside it. Update planner strategy selection and transaction lifecycle are separate behaviors; parsed
SQL command dispatch is described below.

`Managers.CreateQueryPlan` is the first SQL-facing query boundary: it accepts a SELECT string, parses it through
`internal/parse`, and passes the resulting query plus the supplied transaction to the configured query planner. Parse
and planning failures keep server-query context, while the caller retains transaction ownership. The [SELECT query
dispatch diagram](../diagrams/ch11-query-dispatch.svg) shows this path and its dependencies; its [editable Draw.io
source](../diagrams/ch11-query-dispatch.drawio) is maintained beside the SVG. SQL update dispatch and result-cursor
adaptation remain separate behaviors.

`Managers.ExecuteUpdate` is the matching SQL-facing update boundary. It parses the statement with
`Parser.ParseUpdateCommand`, routes each concrete command value to the corresponding `UpdatePlanner` operation, and
returns that operation's affected-row count. The [update dispatch diagram](../diagrams/ch11-update-dispatch.svg) shows
the six command paths and unchanged transaction ownership; its [editable Draw.io source](../diagrams/ch11-update-dispatch.drawio)
is kept beside the SVG. Parse and planner failures retain server-update context, and transaction completion remains
the caller's responsibility.

`Managers.NewTransaction` uses one ID allocator and lock table shared by the manager set, while each returned transaction
keeps its own buffer ownership. Creation appends a START record to the shared log and supplies the log iterator needed
for transaction rollback. A failed START append returns no transaction. Restart recovery through `Managers.Recover`, catalog
initialization and update-planner strategy selection remain separate behaviors. The embedded connection's transaction
adapter is described below; SQL execution and result adapters remain separate.

`embedded.Connection` is the first concrete client adapter. It implements `driver.Conn`, starts a transaction from the
manager set in `Begin`, and returns a `driver.Tx` adapter for explicit commit or rollback. Closing the connection prevents
new begins but leaves already returned transactions caller-owned. Its `driver.Execer` implementation gives a
database-level `DB.Exec` call its own transaction and commits that update on success. `driver.ExecerContext` checks for
nil or canceled contexts before starting work and preserves the same autocommit versus active-transaction behavior.
When `database/sql` routes a `sql.Tx.Exec` call through the same connection, the connection reuses its active
transaction and leaves completion to the caller. The [embedded execution diagram](../diagrams/ch11-database-autocommit.svg)
shows the context check and two transaction-ownership paths; its [editable Draw.io source](../diagrams/ch11-database-autocommit.drawio) is maintained beside the SVG. The
[embedded transaction lifecycle
diagram](../diagrams/ch11-embedded-transaction-lifecycle.svg) shows the connection ownership boundaries and its
[editable Draw.io source](../diagrams/ch11-embedded-transaction-lifecycle.drawio) is maintained beside the SVG.

`embedded.Connector` binds a caller-supplied `server.Managers` set to Go's `driver.Connector` interface. Each
`Connect(ctx)` call returns a fresh `Connection` and respects context cancellation. Its `Driver()` adapter also creates
fresh connections from the already-bound manager set; `Driver.Open` ignores the data source name because the connector
has already selected a database. `ParseEmbeddedURL` separately extracts the database name following the Java-compatible
`jdbc:simpledb:` prefix. Callers choose the storage directory and manager settings from that name, then bind the
resulting managers to a connector. `sql.OpenDB(connector)` uses the same in-process database without registering a
global driver. The [embedded connector diagram](../diagrams/ch11-embedded-connector.svg) shows both standard-library
entry points; its [editable Draw.io source](../diagrams/ch11-embedded-connector.drawio) is kept beside the SVG. The
[embedded URL flow](../diagrams/ch11-embedded-url.svg) separates name parsing from directory and manager selection;
its [editable Draw.io source](../diagrams/ch11-embedded-url.drawio) is maintained beside the SVG.

`Connection.Prepare` retains the SQL string and connection in a `driver.Stmt` handle. `Close` is idempotent, and
`NumInput` counts positional `?` markers outside single-quoted literals so `database/sql` can check arity before
dispatch. Unbalanced SQL quoting reports unknown arity, leaving the parser to return the syntax error. `Stmt.Exec` and
`Stmt.Query` accept ordered `int64` and `string` values; the adapter formats those values as SQL constants and doubles
embedded single quotes so a string remains data. Too few or too many arguments, named parameters, and unsupported
driver values fail before planner dispatch. `Stmt.Exec` routes the bound update through `Managers.ExecuteUpdate` using
the active connection transaction and returns the planner's affected-row count; the transaction remains open for
explicit completion. `Stmt.ExecContext` validates its context before applying the same binding behavior. `Stmt.Query`
creates a query plan using the active caller-owned transaction and returns its projected fields through `driver.Rows`.
Integer columns become
`int64` driver values, strings remain strings, exhaustion returns `io.EOF`, and closing rows closes the underlying scan.
`Stmt.QueryContext` rejects nil or canceled contexts before binding or creating the plan, then uses the same caller-owned
transaction and row path. `driver.RowsColumnTypeDatabaseTypeName` reports `VARCHAR` and `INTEGER` from the projected
schema in column order, so `database/sql.Rows.ColumnTypes()` exposes those database-specific names without advancing
the scan. `driver.RowsColumnTypeScanType` reports `string` for varchar values and `int64` for integer values, matching
the driver's row values. `driver.RowsColumnTypeLength` reports the declared varchar character limit and returns no
variable-length bound for integer columns. `driver.RowsColumnTypeNullable` reports that supported scalar fields are
known non-nullable because the current record/query value model cannot represent NULL. Precision/scale remain
independent optional metadata behaviors.
The [statement lifecycle and query diagram](../diagrams/ch11-embedded-statement.svg) shows positional scalar binding,
scan creation, and rows plus database type names, Go scan types, varchar lengths, and nullability; its [editable Draw.io
source](../diagrams/ch11-embedded-statement.drawio) is kept beside the SVG.
The [update execution diagram](../diagrams/ch11-embedded-update-execution.svg) shows transaction ownership during a
statement call; its [editable Draw.io source](../diagrams/ch11-embedded-update-execution.drawio) is maintained too.

The embedded and network drivers share a client contract in Java. Go's `network.ParseNetworkURL` extracts a host
from `jdbc:simpledb://host` while preserving the Java driver's default registry-port convention; it rejects explicit
ports and URL components other than the host. Parsing remains independent from socket dialing and remote-session
creation. The RMI proxy classes are transport-specific and should not be reproduced as a Go class hierarchy or copied
into the embedded path. The standard `database/sql/driver` contract is used only for the connection and transaction
operations currently implemented; its remaining optional interfaces stay out until a concrete use case needs them.

## Test map

| Test | Observable behavior |
| --- | --- |
| `TestNewManagersUsesConfiguredFileLogAndBufferManagers` | Explicit block size and buffer count reach the constructed managers, and the log gets its initial block |
| `TestNewManagersCreatesIndependentMetadataManagers` | Each successful manager set receives a distinct, non-nil metadata facade |
| `TestNewManagersReturnsNoManagersWhenFileManagerFails` | Directory creation failure is wrapped and exposes no partial manager set |
| `TestNewManagersReturnsNoManagersWhenLogManagerFails` | An unusable log block size is reported after file-manager creation with no partial manager set |
| `TestNewManagersReturnsNoManagersWhenBufferManagerFails` | Invalid buffer capacity is reported after log initialization with no partial manager set |
| `TestManagersInitializeCatalogsRegistersCompleteCatalogSet` | All four catalog layouts are registered and the caller commits the physical catalog files |
| `TestManagersInitializeCatalogsRejectsMissingState` | Nil/incomplete manager sets and a nil transaction fail before catalog work |
| `TestManagersInitializeCatalogsReportsMetadataCreationFailure` | A catalog-layout failure retains server initialization context |
| `TestManagersNewTransactionSharesIdentityAndAppendsStartRecords` | Successful transactions share sequential IDs, append START records, and can use the configured log iterator |
| `TestManagersNewTransactionRejectsNilAndIncompleteManagers` | Nil and zero-value manager sets fail before returning a transaction |
| `TestManagersNewTransactionRejectsMissingDependencies` | An incomplete dependency is rejected before log access |
| `TestManagersNewTransactionReportsMissingInternalState` | Missing shared lock state is rejected before log access |
| `TestManagersNewTransactionReturnsNoTransactionWhenStartCannotFit` | A log that cannot store START returns contextual failure and no transaction |
| `TestManagersNewTransactionUsesFileBackedStorage` | A committed transaction writes through the configured file and buffer managers |
| `TestManagersRecoverDurablyUndoesFlushedUncommittedUpdate` | A reopened server manager set restores the prior value, flushes CHECKPOINT, and commits its recovery transaction |
| `TestManagersRecoverRejectsNilAndIncompleteManagerSets` | Nil and incomplete manager sets fail before recovery work |
| `TestManagersRecoverPreservesRestartHistoryError` | Corrupt durable history returns contextual restart-recovery failure |
| `TestFinishRestartRecoveryReportsRecoveryAndCommitFailures` | Recovery failure prevents commit, while commit failure preserves the durable checkpoint LSN |
| `TestManagersComposeBasicQueryPlannerWithTheirMetadata` | A manager set's SELECT planner resolves an initialized table through that set's metadata and caller-owned transaction |
| `TestManagersSetQueryPlannerStrategyUsesSharedMetadata` | Basic and cost-aware choices replace the default planner and both plan through shared metadata |
| `TestManagersSetQueryPlannerStrategyRejectsInvalidInputsWithoutReplacingPlanner` | Nil manager sets, missing metadata, and unknown strategies leave the current planner unchanged |
| `TestNewConnectionValidatesManagersAndImplementsDriverConnection` | A non-nil manager set creates a driver connection that retains SQL in a prepared statement handle |
| `TestConnectionBeginsAndCompletesCallerControlledTransactions` | Begin creates a driver transaction that delegates commit and rollback, and the connection can begin again |
| `TestConnectionCloseIsIdempotentAndLeavesReturnedTransactionCallerOwned` | Close prevents new operations without completing a transaction already returned to the caller |
| `TestConnectionRejectsInvalidReceiversAndManagerState` | Nil, closed, and incomplete connections return contextual errors |
| `TestTransactionAdapterRejectsNilAndMissingTransactions` | Nil transaction adapters reject commit and rollback safely |
| `TestNewConnectorReturnsFreshDriverConnections` | A connector implements `driver.Connector` and returns distinct connections |
| `TestParseEmbeddedURL` | The JDBC-compatible embedded URL prefix is removed while relative and absolute database paths are preserved |
| `TestParseEmbeddedURLRejectsUnsupportedAndEmptyNames` | Missing/unsupported URL prefixes and empty database names return errors |
| `TestParseNetworkURL` | Network JDBC URLs preserve DNS, IPv4, and bracketed IPv6 hostnames |
| `TestParseNetworkURLRejectsMalformedDataSourceNames` | Empty or unsupported URLs, ports, and non-host URL components return errors |
| `TestDialNetworkURLUsesContextAndDefaultPort` | URL hosts become TCP addresses on port 1099 and receive the caller context |
| `TestDialNetworkURLRejectsInvalidInputsBeforeDial` | Nil contexts and invalid URLs fail before dialing |
| `TestDialNetworkURLWrapsDialFailure` | Dial failures retain their cause and host context |
| `TestDialNetworkURLRejectsNilConnection` | A dialer that succeeds without a connection returns an error |
| `TestDialNetworkURLRejectsNilDialer` | Missing dial dependencies are rejected before dialing |
| `TestDialNetworkURLHonorsCanceledContext` | Canceled contexts stop the default dial path |
| `TestServeConnectionCreatesOneTransactionOnRemoteConnect` | RPC Connect creates one START-logged transaction retained for the connection and rejects replacement |
| `TestServeConnectionReportsManagerTransactionFailureOverRPC` | START-record failures are returned over RPC without a transaction ID |
| `TestServeConnectionRejectsNilAndIncompleteInputs` | Nil connections/managers and incomplete transaction manager sets are rejected |
| `TestConnectRPCSessionRetainsTransactionIDAndLeavesClientOpen` | The client constructor stores the returned transaction ID and leaves the caller-owned RPC client usable |
| `TestConnectRPCSessionRejectsNilClient` | Nil clients return no session and a nil session reports transaction ID zero |
| `TestConnectRPCSessionWrapsRemoteConnectErrorWithoutReturningSession` | A failed server-side Connect is contextual and returns no client session |
| `TestRemoteSessionCommitRetainsSuccessorTransactionID` | Commit replaces the retained ID with the successor transaction ID and keeps the caller's RPC client usable |
| `TestRemoteSessionCommitRejectsInvalidStateAndPreservesIDOnRPCFailure` | Nil session/client and RPC errors are contextual, and a failed Commit retains the last known ID |
| `TestDriverServiceConnectRejectsInvalidReceiverAndReply` | Nil service/reply and incomplete managers fail, clearing any partial transaction ID |
| `TestRegisterDriverServiceRejectsInvalidInputsAndDuplicateRegistration` | RPC service registration validates dependencies and wraps duplicate-name failures |
| `TestDriverServiceCommitCommitsAndStartsNextTransactionOverRPC` | RPC Commit persists the current outcome, returns a new transaction ID, and retains exactly one successor |
| `TestDriverServiceCommitRejectsInvalidStateAndReply` | Nil state, manager set, reply, and active transaction are rejected |
| `TestDriverServiceCommitKeepsTransactionWhenCommitFails` | A failed commit returns no new ID and does not replace the active transaction |
| `TestDriverServiceCommitLeavesNoStaleTransactionWhenNextStartFails` | Successful commit followed by successor-start failure leaves no completed transaction attached and permits reconnect |
| `TestDriverServiceRollbackRollsBackAndStartsNextTransactionOverRPC` | RPC Rollback persists the current rollback, returns a new transaction ID, and retains exactly one successor |
| `TestDriverServiceRollbackRejectsInvalidStateAndReply` | Nil state, manager set, reply, and active transaction are rejected |
| `TestDriverServiceRollbackKeepsTransactionWhenRollbackFails` | A failed rollback returns no new ID and does not replace the active transaction |
| `TestDriverServiceRollbackLeavesNoStaleTransactionWhenNextStartFails` | Successful rollback followed by successor-start failure leaves no completed transaction attached and permits reconnect |
| `TestDriverServiceCloseCommitsWithoutSuccessorOverRPC` | RPC Close commits the active transaction and starts no successor |
| `TestDriverServiceCloseRejectsInvalidStateAndReply` | Nil service/reply and a missing active transaction are rejected |
| `TestDriverServiceCloseKeepsSessionOpenWhenCommitFails` | A failed close commit preserves the active transaction and open session |
| `TestDriverServiceRejectsNewOperationsAfterClose` | Connect, Commit, Rollback, and repeated Close are rejected after closure |
| `TestDriverServiceExecuteUpdateRunsWithinActiveTransactionOverRPC` | RPC updates return affected rows, preserve the session transaction, and defer commit to an explicit request |
| `TestDriverServiceExecuteUpdateRejectsInvalidStateAndClosedSession` | Nil service/reply, missing manager/transaction, and closed sessions return errors with cleared results |
| `TestDriverServiceExecuteUpdatePreservesSessionAfterStatementAndPlannerErrors` | Parse/planner failures clear results while keeping the current transaction available for completion |
| `TestDriverServiceExecuteQueryReturnsTypedProjectedRowsOverRPC` | RPC SELECT returns projected column metadata and typed row values without completing the active transaction |
| `TestDriverServiceExecuteQueryRejectsInvalidStateAndResetsResponse` | Nil service/reply, missing manager/transaction, and closed sessions return errors with cleared results |
| `TestDriverServiceExecuteQueryReturnsPlanningAndOpeningErrors` | Parse/planner, nil plan/schema/scan, and scan-open errors return no partial rows |
| `TestDriverServiceExecuteQueryClosesScansOnMaterializationFailures` | Position, advance, value, unsupported-type, and close failures release the opened scan and clear results |
| `TestDriverServiceExecuteQueryClosesPartiallyOpenedScans` | A plan that returns a scan with an open error still closes that scan and joins cleanup errors |
| `TestConnectorConnectValidatesReceiverContextAndCancellation` | Nil state and canceled contexts return errors before creating a connection |
| `TestConnectorDriverOpenAndDatabaseSQLUseConfiguredManagers` | Driver.Open and `sql.OpenDB` use the bound manager set and support transaction completion |
| `TestConnectorDriverRejectsInvalidState` | Nil driver and connector states fail with contextual errors |
| `TestStatementRejectsNilAndClosedOperations` | Nil and closed statements return contextual lifecycle errors |
| `TestPreparedStatementDeclaresZeroInputsToDatabaseSQL` | Extra prepared-statement arguments are rejected by `database/sql` before driver dispatch |
| `TestPreparedStatementQueryReturnsProjectedRowsToDatabaseSQL` | Prepared SELECT returns ordered columns and typed rows through `database/sql` |
| `TestStatementQueryRejectsInvalidStateAndArguments` | Prepared queries reject nil, closed, unbound, inactive, and argument-bearing calls |
| `TestStatementQueryWrapsPlannerOpenAndRowsCreationFailures` | Planner, scan-open, and rejected-result cleanup failures retain query context |
| `TestPreparedStatementQueryContextHonorsCancellationAndTransaction` | A canceled query does not prevent a valid prepared query in the same caller-owned transaction |
| `TestStatementQueryContextImplementsDriverContractAndRejectsInvalidInputs` | Context-aware query rejects nil/closed statements, nil/canceled contexts, and arguments |
| `TestNewStatementRowsValidatesScanAndSchema` | Result rows reject missing scans and schemas |
| `TestStatementRowsReturnsTypedValuesAndClosesUnderlyingScan` | Rows expose defensive column names, map supported values, report EOF, and close once |
| `TestStatementRowsReportsDatabaseTypeNames` | Driver rows and `database/sql.Rows.ColumnTypes()` report projected integer and varchar database type names |
| `TestStatementRowsReportsScanTypes` | Driver rows and `database/sql.Rows.ColumnTypes()` report the Go types used to scan projected integer and varchar values |
| `TestStatementRowsReportsColumnLengths` | Driver rows and `database/sql.Rows.ColumnTypes()` report varchar limits and no variable-length bound for integer columns |
| `TestStatementRowsReportsNullability` | Driver rows and `database/sql.Rows.ColumnTypes()` report supported scalar columns as known non-nullable |
| `TestStatementRowsReportsInvalidStateAndScanFailures` | Closed rows, wrong destinations, scan/read errors, unsupported types, and close failures are returned |
| `TestDatabaseSQLExecutesUpdatesInsideExplicitTransaction` | CREATE TABLE and INSERT execute through `sql.Tx`, report affected rows, commit, and roll back independently |
| `TestConnectionTracksOneActiveTransactionForStatementExecution` | A connection rejects nested transactions and becomes reusable after explicit completion |
| `TestStatementExecRejectsInvalidStateAndWrapsUpdateErrors` | Exec rejects nil, closed, unbound, argument-bearing, malformed, and planner-failing calls |
| `TestDatabaseExecAutocommitsEmbeddedUpdates` | `sql.DB.Exec` commits CREATE TABLE and INSERT and reports affected rows |
| `TestConnectionExecRejectsInvalidStateAndUpdateFailures` | Autocommit rejects invalid connections and arguments, rolls back failures, and accepts a later update |
| `TestConnectionExecReportsCommitAndRollbackFailures` | Durable commit and recovery failures retain autocommit context |
| `TestStatementExecLeavesCallerTransactionOpen` | Prepared statement updates remain caller-owned and are undone by rollback |
| `TestDatabaseExecContextUsesAndHonorsContext` | `DB.ExecContext` commits on success, rejects cancellation before mutation, and keeps an explicit transaction usable |
| `TestConnectionExecContextRejectsNilCanceledAndArgumentInputs` | Context execution rejects nil/canceled contexts and unsupported parameters before dispatch |
| `TestPreparedStatementExecContextUsesCallerTransaction` | A prepared statement observes cancellation, returns affected rows, and leaves rollback to its caller |
| `TestStatementExecContextRejectsInvalidInputs` | Context-aware statement calls reject nil/closed state, canceled contexts, missing transactions, and arguments |
| `TestPreparedStatementCountsParametersOutsideQuotedStrings` | `NumInput` counts positional markers and ignores question marks inside SQL string literals |
| `TestPreparedStatementParameterBindingValidatesValuesAndSQL` | Binding quotes scalar values safely and rejects invalid arity, quoting, and value types |
| `TestPreparedStatementBindsScalarParametersThroughDatabaseSQL` | Prepared INSERT, SELECT, and UPDATE bind integer/string values through context calls in the caller transaction |
| `TestPreparedStatementLegacyExecAndQueryBindScalarValues` | Legacy `driver.Stmt.Exec` and `Query` also bind integer/string values and return projected driver rows |
| `TestManagersComposeBasicUpdatePlannerWithTheirMetadata` | A manager set's update planner registers a table through shared metadata and a caller-owned transaction |
| `TestManagersCreateQueryPlanParsesAndPlansSelect` | SELECT text is parsed and planned through the manager set without taking ownership of its transaction |
| `TestManagersCreateQueryPlanReportsInvalidInputsAndFailures` | Incomplete managers, nil transactions, parse errors, and plan errors are returned with context |
| `TestManagersExecuteUpdateDispatchesParsedCommands` | INSERT, DELETE, UPDATE, and three CREATE commands reach their matching planner operations |
| `TestManagersExecuteUpdateReportsInvalidInputsAndFailures` | Incomplete managers, nil transactions, malformed SQL, and planner failures return with context |
| `TestExecuteUpdateCommandRejectsUnsupportedData` | Unknown parsed command values are rejected instead of disappearing silently |

The manager-composition tests first failed to compile because `NewManagers` did not exist. The transaction tests first
failed because `Managers.NewTransaction` did not exist, and the metadata test first failed because `Managers.Metadata`
did not exist. The catalog test first failed because `Managers.InitializeCatalogs` was undefined. Restart recovery tests
first failed because `Managers.Recover` and its recovery-transaction lifecycle helper were undefined. The query-planner
test first failed because `Managers.QueryPlanner` was undefined. The update planner test first failed because
`Managers.UpdatePlanner` was undefined. The SQL SELECT dispatch test first failed because `Managers.CreateQueryPlan` was
undefined. The SQL update dispatch tests first failed because `Managers.ExecuteUpdate` and its command dispatcher were
undefined. The planner-strategy tests first failed because `QueryPlannerStrategy` and
`Managers.SetQueryPlannerStrategy` were undefined. These tests use temporary database directories and keep catalog initialization, explicit process-restart
recovery, SELECT planner composition, update planner composition, SQL SELECT planning, SQL update dispatch, embedded
connection transaction lifecycle, ordinary transaction completion, and JDBC client operations as separate behaviors.
The embedded-connection tests first failed because `Connection`, `NewConnection`, and its transaction adapter were
undefined. Connector tests first failed because `Connector`, `NewConnector`, and the driver adapter were undefined.
Statement tests first failed because the connection returned no statement handle and the `statement` adapter was
undefined. Statement-execution tests first failed because the connection did not expose an active transaction to the
prepared statement. Autocommit tests first failed because `Connection` did not implement `driver.Execer`. Context-execution
tests first failed because `Connection` did not implement `driver.ExecerContext`. Prepared-statement context tests first
failed because the statement adapter did not implement `driver.StmtExecContext`.
Prepared-query integration tests first failed because `Stmt.Query` returned `driver.ErrSkip` rather than opening a SELECT plan.
Prepared-query context tests first failed to compile because the statement did not implement `driver.StmtQueryContext`.
The prepared-statement arity test first failed because `NumInput` reported unknown arity and the driver rejected the
argument only after dispatch. Parameter-count tests first failed because the statement reported zero for positional
markers, and the prepared-statement integration test first failed when `database/sql` rejected a bound INSERT as having
arguments for a zero-input statement.
These tests use temporary database directories and keep catalog initialization, explicit process-restart recovery,
planner composition and strategy selection, embedded connection lifecycle, connector creation, statement preparation,
transaction-bound SQL updates, database-level autocommit, context-aware connection and prepared-statement execution,
scalar prepared-statement binding, query rows, and JDBC client operations as separate behaviors.

The embedded URL test-first run initially failed to compile because `ParseEmbeddedURL` was undefined. The parser deliberately
does not create directories or managers; URL interpretation, storage configuration, and connector construction remain
separate responsibilities.

The network URL test-first run initially failed to compile because `ParseNetworkURL` was undefined. It returns only a
validated hostname; dialing and remote-session creation remain separate responsibilities, and explicit URL ports are
rejected to preserve the Java driver's default registry port behavior.

The network dial test-first run initially failed to compile because `DialNetworkURL` and its injectable dial boundary
were undefined. The implementation passes the caller's context to the TCP dialer, uses the default port, and returns a
caller-owned connection without starting an RPC or SQL session.

The network RPC-session test-first run initially failed to compile because `ConnectRequest`, `ConnectResponse`,
`DriverService`, and `ServeConnection` were undefined. Tests exercise the gob RPC method through `net.Pipe`, verify one
START record and the returned transaction ID, check manager and registration errors, and confirm a second Connect cannot
replace the connection transaction. The service retains transaction ownership for later remote methods; finalization and
SQL execution remain independent.


The network RPC-commit test-first run initially failed because `CommitRequest`, `CommitResponse`, and `DriverService.Commit`
were undefined. Tests verify the COMMIT and successor START records through RPC, invalid service state, completed-transaction
errors, and the case where the commit succeeds but the next START cannot be created. In that last case the completed
transaction is not retained and Connect can be retried; rollback and connection close remain independent.

The network RPC-rollback test-first run initially failed because `RollbackRequest`, `RollbackResponse`, and
`DriverService.Rollback` were undefined. Tests verify the ROLLBACK and successor START records through RPC, invalid state,
completed-transaction errors, and a failed successor START after durable rollback. In that last case the completed
transaction is detached and Connect can be retried; connection close and SQL dispatch remain separate.

The network RPC-close test-first run initially failed because `CloseRequest`, `CloseResponse`, and `DriverService.Close`
were undefined. Tests verify the final COMMIT through RPC without a successor START, reject invalid or repeated lifecycle
operations, and confirm that a failed commit leaves the transaction attached and the session open. A successful close
ends the service session but leaves transport ownership with the caller; remote SQL remains separate.

The network RPC-update test-first run initially failed because `ExecuteUpdateRequest`, `ExecuteUpdateResponse`, and
`DriverService.ExecuteUpdate` were undefined. RPC tests create catalogs and a table, insert a row, inspect the affected
count, and verify that the same transaction remains active until explicit Commit. Direct method tests cover malformed SQL,
planner errors, cleared results, missing state, and closed-session rejection. Query RPCs and client-side JDBC adaptation
remain separate.

The network RPC-query test-first run initially failed because `ExecuteQueryRequest`, `ExecuteQueryResponse`, and
`DriverService.ExecuteQuery` were undefined. RPC tests execute typed, projected rows after catalog-backed updates and
verify the session transaction remains active until explicit Commit. Direct tests cover parse/planner/open failures,
invalid state, schema/scan validation, materialization cleanup, partial-open cleanup, no partial results, and empty results.
The response uses concrete gob-friendly structs rather than interface-valued cells.

The network RPC client-session test-first run initially failed because `RemoteSession`, `ConnectRPCSession`, and
`TransactionID` were undefined. Tests verify the transaction ID returned by the `SimpleDB.Connect` RPC, nil-client
validation, contextual server errors with no session, and continued use of the caller-owned RPC client.

The network RPC client-commit test-first run initially failed because `RemoteSession.Commit` was undefined. Tests verify
the successor transaction ID after successful RPC completion, nil session/client validation, preservation of the last
known ID after a remote error, and continued use of the caller-owned RPC client.

## Intentional differences

- Do not reproduce the Java `java.sql` adapter hierarchy mechanically.
- Do not introduce RMI merely to match Java's network classes; define a Go transport only when a server/client use case is implemented.
- Preserve explicit Go errors for initialization and lifecycle failures rather than relying on JDBC `SQLException` conventions.
- Keep the upstream Java distribution and book PDF outside Git.
- Parse the Java-compatible embedded URL prefix, but keep path resolution, database initialization, and manager settings explicit in Go.
- Parse the Java-compatible network URL into a host, but keep dialing and remote-session lifecycle separate from parsing; the current Go network parser preserves the default registry port convention.
- Open a TCP socket to the parsed network host with caller context, then serve Go `net/rpc` Connect, Commit, Rollback, Close, ExecuteUpdate, and ExecuteQuery requests. Commit and Rollback start successor transactions; Close commits without a successor and ends the RPC session while leaving the transport `net.Conn` caller-owned. ExecuteUpdate and ExecuteQuery use the active transaction without completing it. Keep client-side JDBC operations separate.
