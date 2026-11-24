# Chapter 11: JDBC Interfaces

## Chapter focus

This chapter connects the database engine to client programs. It describes the JDBC-facing types and contrasts an
embedded driver, which creates the engine in the application process, with a network driver, which delegates work to a
server. The existing [client-boundary diagram](../diagrams/ch02-client-boundary.svg) shows these two paths; its
[editable Draw.io source](../diagrams/ch02-client-boundary.drawio) is shared with the Chapter 2 concept note.

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
have equivalent JDBC or server packages. The Java three-argument `SimpleDB` constructor is a useful first composition
boundary: it creates the file manager, then the log manager, then the buffer manager using the supplied directory,
block size, and buffer count. The one-argument constructor adds recovery, metadata initialization, planner selection, and
transaction commit; those effects should remain separately testable rather than being hidden in one ported constructor.

The embedded and network drivers share a client contract in Java, but Go should introduce an adapter only when a concrete
caller needs it. The RMI proxy classes are transport-specific and should not be reproduced as a Go class hierarchy or
copied into the embedded path. A future Go client boundary can use the standard `database/sql/driver` contract if its
actual scope requires SQL driver compatibility.

## Test map

This document-only chapter map adds no Go behavior. The first implementation behavior should test manager construction
order, configured values, and cleanup/error propagation through a temporary database directory. Recovery, metadata and
planner bootstrap, transaction creation, and JDBC client operations remain independent follow-up behaviors.

## Intentional differences

- Do not reproduce the Java `java.sql` adapter hierarchy mechanically.
- Do not introduce RMI merely to match Java's network classes; define a Go transport only when a server/client use case is implemented.
- Preserve explicit Go errors for initialization and lifecycle failures rather than relying on JDBC `SQLException` conventions.
- Keep the upstream Java distribution and book PDF outside Git.
