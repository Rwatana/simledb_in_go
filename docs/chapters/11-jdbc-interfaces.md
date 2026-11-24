# Chapter 11: JDBC Interfaces

## Chapter focus

This chapter connects the database engine to client programs. It describes the JDBC-facing types and contrasts an
embedded driver, which creates the engine in the application process, with a network driver, which delegates work to a
server. The existing [client-boundary diagram](../diagrams/ch02-client-boundary.svg) shows these two paths; its
[editable Draw.io source](../diagrams/ch02-client-boundary.drawio) is shared with the Chapter 2 concept note.

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
| `simpledb/jdbc/network/Remote*` | Implement the server-side remote objects used by the Java RMI transport |

## Go design and current status

The Go repository has lower-level file, log, buffer, transaction, metadata, query, and planning packages, but does not yet
have a JDBC adapter. `internal/server.NewManagers` now creates the file manager, then the log manager for `simpledb.log`,
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
is maintained beside it.

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
initialization, planner selection, ordinary transaction completion, and client adapters remain separate behaviors.

The embedded and network drivers share a client contract in Java, but Go should introduce an adapter only when a concrete
caller needs it. The RMI proxy classes are transport-specific and should not be reproduced as a Go class hierarchy or
copied into the embedded path. A future Go client boundary can use the standard `database/sql/driver` contract if its
actual scope requires SQL driver compatibility.

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
undefined. These tests use temporary database directories and keep catalog initialization, explicit process-restart
recovery, SELECT planner composition, update planner composition, SQL SELECT planning, SQL update dispatch, ordinary
transaction completion, and JDBC client operations as separate behaviors.

## Intentional differences

- Do not reproduce the Java `java.sql` adapter hierarchy mechanically.
- Do not introduce RMI merely to match Java's network classes; define a Go transport only when a server/client use case is implemented.
- Preserve explicit Go errors for initialization and lifecycle failures rather than relying on JDBC `SQLException` conventions.
- Keep the upstream Java distribution and book PDF outside Git.
