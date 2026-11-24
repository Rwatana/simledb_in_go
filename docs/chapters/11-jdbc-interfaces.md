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
metadata facade. The facade's catalog file initialization is a separate transaction-backed operation. Process-restart
recovery, planner selection, and transaction commit remain separate behaviors.

`Managers.NewTransaction` uses one ID allocator and lock table shared by the manager set, while each returned transaction
keeps its own buffer ownership. Creation appends a START record to the shared log and supplies the log iterator needed
for transaction rollback. A failed START append returns no transaction. Process-restart recovery, catalog initialization,
planner selection, transaction lifecycle, and client adapters remain separate behaviors.

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

The manager-composition tests first failed to compile because `NewManagers` did not exist. The transaction tests first
failed because `Managers.NewTransaction` did not exist, and the metadata test first failed because `Managers.Metadata`
did not exist. The catalog test first failed because `Managers.InitializeCatalogs` was undefined. These tests use
temporary database directories and keep catalog initialization separate from process-restart recovery, planner setup,
transaction completion, and JDBC client operations.

## Intentional differences

- Do not reproduce the Java `java.sql` adapter hierarchy mechanically.
- Do not introduce RMI merely to match Java's network classes; define a Go transport only when a server/client use case is implemented.
- Preserve explicit Go errors for initialization and lifecycle failures rather than relying on JDBC `SQLException` conventions.
- Keep the upstream Java distribution and book PDF outside Git.
