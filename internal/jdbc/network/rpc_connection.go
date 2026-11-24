package network

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"reflect"
	"sync"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/server"
	"github.com/Rwatana/simledb_in_go/internal/transaction"
)

// ConnectRequest is the empty request for creating a remote database session.
type ConnectRequest struct{}

// ConnectResponse contains the transaction identifier for a connected session.
type ConnectResponse struct {
	TransactionID int64
}

// CommitRequest is the empty request for committing a remote database session.
type CommitRequest struct{}

// CommitResponse contains the transaction identifier created after commit.
type CommitResponse struct {
	TransactionID int64
}

// RollbackRequest is the empty request for rolling back a remote database session.
type RollbackRequest struct{}

// RollbackResponse contains the transaction identifier created after rollback.
type RollbackResponse struct {
	TransactionID int64
}

// ExecuteUpdateRequest carries one SQL update statement for a remote database session.
type ExecuteUpdateRequest struct {
	Statement string
}

// ExecuteUpdateResponse reports the number of rows affected by a remote SQL update.
type ExecuteUpdateResponse struct {
	RowsAffected int64
}

// ExecuteQueryRequest carries one SQL SELECT statement for a remote database session.
type ExecuteQueryRequest struct {
	Statement string
}

// QueryColumn describes one projected result column in schema order.
type QueryColumn struct {
	Name string
	Type string
	// DisplaySize is the JDBC-compatible recommended width for rendering this column.
	DisplaySize int
}

// QueryValue contains one typed scalar result without gob interface registration.
type QueryValue struct {
	Kind    string
	Integer int32
	String  string
}

// ExecuteQueryResponse contains ordered result columns and their materialized rows.
type ExecuteQueryResponse struct {
	Columns []QueryColumn
	Rows    [][]QueryValue
}

// CloseRequest is the empty request for closing a remote database session.
type CloseRequest struct{}

// CloseResponse is the empty response returned after a remote session is closed.
type CloseResponse struct{}

// DriverService serves requests over one RPC connection and retains its transaction.
type DriverService struct {
	managers    *server.Managers
	mu          sync.Mutex
	transaction *transaction.Transaction
	closed      bool
}

// ServeConnection serves a gob-encoded RPC client on conn until it closes.
func ServeConnection(conn net.Conn, managers *server.Managers) error {
	return serveConnection(conn, managers, rpc.NewServer())
}

// ServeListener accepts network clients and serves each RPC connection independently.
// The caller retains ownership of listener and stops accepting by closing it.
func ServeListener(listener net.Listener, managers *server.Managers) error {
	return serveListener(listener, managers, func(conn net.Conn, managers *server.Managers) error {
		return ServeConnection(conn, managers)
	})
}

type connectionHandler func(net.Conn, *server.Managers) error

func serveListener(listener net.Listener, managers *server.Managers, serve connectionHandler) error {
	if listener == nil {
		return fmt.Errorf("serve network RPC listener: listener is nil")
	}
	if managers == nil {
		return fmt.Errorf("serve network RPC listener: manager set is nil")
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("serve network RPC listener: %w", err)
		}
		go func() {
			defer conn.Close()
			_ = serve(conn, managers)
		}()
	}
}

type rpcConnectionServer interface {
	rpcRegistrar
	ServeConn(io.ReadWriteCloser)
}

func serveConnection(conn net.Conn, managers *server.Managers, rpcServer rpcConnectionServer) error {
	if conn == nil {
		return fmt.Errorf("serve network RPC connection: connection is nil")
	}
	if managers == nil {
		return fmt.Errorf("serve network RPC connection: manager set is nil")
	}
	if rpcServer == nil {
		return fmt.Errorf("serve network RPC connection: RPC server is nil")
	}
	service := &DriverService{managers: managers}
	if err := registerDriverService(rpcServer, service); err != nil {
		return fmt.Errorf("serve network RPC connection: %w", err)
	}
	rpcServer.ServeConn(conn)
	if err := service.disconnect(); err != nil {
		return fmt.Errorf("disconnect network RPC session: %w", err)
	}
	return nil
}

type rpcRegistrar interface {
	RegisterName(string, any) error
}

func registerDriverService(registrar rpcRegistrar, service *DriverService) error {
	if registrar == nil {
		return fmt.Errorf("register network RPC service: registrar is nil")
	}
	if service == nil {
		return fmt.Errorf("register network RPC service: service is nil")
	}
	if err := registrar.RegisterName("SimpleDB", service); err != nil {
		return fmt.Errorf("register SimpleDB RPC service: %w", err)
	}
	return nil
}

// Connect creates and retains the transaction used by later requests on this RPC connection.
func (s *DriverService) Connect(_ ConnectRequest, reply *ConnectResponse) error {
	if reply != nil {
		*reply = ConnectResponse{}
	}
	if s == nil {
		return fmt.Errorf("connect network RPC session: service is nil")
	}
	if reply == nil {
		return fmt.Errorf("connect network RPC session: reply is nil")
	}
	if s.managers == nil {
		return fmt.Errorf("connect network RPC session: manager set is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("connect network RPC session: session is closed")
	}
	if s.transaction != nil {
		return fmt.Errorf("connect network RPC session: session is already connected")
	}
	created, err := s.managers.NewTransaction()
	if err != nil {
		return fmt.Errorf("connect network RPC session: %w", err)
	}
	s.transaction = created
	reply.TransactionID = int64(created.ID())
	return nil
}

// Commit commits the active transaction and starts its successor for this RPC connection.
func (s *DriverService) Commit(_ CommitRequest, reply *CommitResponse) error {
	if reply != nil {
		*reply = CommitResponse{}
	}
	if s == nil {
		return fmt.Errorf("commit network RPC session: service is nil")
	}
	if reply == nil {
		return fmt.Errorf("commit network RPC session: reply is nil")
	}
	if s.managers == nil {
		return fmt.Errorf("commit network RPC session: manager set is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("commit network RPC session: session is closed")
	}
	if s.transaction == nil {
		return fmt.Errorf("commit network RPC session: active transaction is nil")
	}
	if _, err := s.transaction.Commit(); err != nil {
		return fmt.Errorf("commit network RPC transaction: %w", err)
	}
	s.transaction = nil
	created, err := s.managers.NewTransaction()
	if err != nil {
		return fmt.Errorf("start network RPC transaction after commit: %w", err)
	}
	s.transaction = created
	reply.TransactionID = int64(created.ID())
	return nil
}

// Rollback rolls back the active transaction and starts its successor for this RPC connection.
func (s *DriverService) Rollback(_ RollbackRequest, reply *RollbackResponse) error {
	if reply != nil {
		*reply = RollbackResponse{}
	}
	if s == nil {
		return fmt.Errorf("rollback network RPC session: service is nil")
	}
	if reply == nil {
		return fmt.Errorf("rollback network RPC session: reply is nil")
	}
	if s.managers == nil {
		return fmt.Errorf("rollback network RPC session: manager set is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("rollback network RPC session: session is closed")
	}
	if s.transaction == nil {
		return fmt.Errorf("rollback network RPC session: active transaction is nil")
	}
	if _, err := s.transaction.Rollback(); err != nil {
		return fmt.Errorf("rollback network RPC transaction: %w", err)
	}
	s.transaction = nil
	created, err := s.managers.NewTransaction()
	if err != nil {
		return fmt.Errorf("start network RPC transaction after rollback: %w", err)
	}
	s.transaction = created
	reply.TransactionID = int64(created.ID())
	return nil
}

// ExecuteUpdate runs one SQL update on the active transaction without completing or replacing it.
func (s *DriverService) ExecuteUpdate(request ExecuteUpdateRequest, reply *ExecuteUpdateResponse) error {
	if reply != nil {
		*reply = ExecuteUpdateResponse{}
	}
	if s == nil {
		return fmt.Errorf("execute network RPC update: service is nil")
	}
	if reply == nil {
		return fmt.Errorf("execute network RPC update: reply is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("execute network RPC update: session is closed")
	}
	if s.managers == nil {
		return fmt.Errorf("execute network RPC update: manager set is nil")
	}
	if s.transaction == nil {
		return fmt.Errorf("execute network RPC update: active transaction is nil")
	}
	rows, err := s.managers.ExecuteUpdate(request.Statement, s.transaction)
	if err != nil {
		return fmt.Errorf("execute network RPC update: %w", err)
	}
	reply.RowsAffected = int64(rows)
	return nil
}

// ExecuteQuery materializes a SQL SELECT on the active transaction without completing it.
func (s *DriverService) ExecuteQuery(request ExecuteQueryRequest, reply *ExecuteQueryResponse) error {
	if reply != nil {
		*reply = ExecuteQueryResponse{}
	}
	if s == nil {
		return fmt.Errorf("execute network RPC query: service is nil")
	}
	if reply == nil {
		return fmt.Errorf("execute network RPC query: reply is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("execute network RPC query: session is closed")
	}
	if s.managers == nil {
		return fmt.Errorf("execute network RPC query: manager set is nil")
	}
	if s.transaction == nil {
		return fmt.Errorf("execute network RPC query: active transaction is nil")
	}
	queryPlan, err := s.managers.CreateQueryPlan(request.Statement, s.transaction)
	if err != nil {
		return fmt.Errorf("execute network RPC query: %w", err)
	}
	if queryPlan == nil {
		return fmt.Errorf("execute network RPC query: query planner returned a nil plan")
	}
	schema := queryPlan.Schema()
	if schema == nil {
		return fmt.Errorf("execute network RPC query: query plan schema is nil")
	}
	scan, err := queryPlan.Open()
	if err != nil {
		if !isNilNetworkQueryScan(scan) {
			if closeErr := scan.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close rejected query scan: %w", closeErr))
			}
		}
		return fmt.Errorf("execute network RPC query: open query scan: %w", err)
	}
	columns, rows, err := materializeNetworkQuery(scan, schema)
	if err != nil {
		return fmt.Errorf("execute network RPC query: %w", err)
	}
	reply.Columns = columns
	reply.Rows = rows
	return nil
}

func materializeNetworkQuery(scan query.Scan, schema *record.Schema) (columns []QueryColumn, rows [][]QueryValue, resultErr error) {
	if isNilNetworkQueryScan(scan) {
		return nil, nil, fmt.Errorf("materialize network query: scan is nil")
	}
	defer func() {
		if closeErr := scan.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close query scan: %w", closeErr))
		}
		if resultErr != nil {
			columns = nil
			rows = nil
		}
	}()
	fields := schema.Fields()
	columns = make([]QueryColumn, 0, len(fields))
	for _, field := range fields {
		definition, _ := schema.Field(field)
		displayWidth := 6
		var typeName string
		switch definition.Type {
		case record.IntegerField:
			typeName = "INTEGER"
		case record.VarcharField:
			typeName = "VARCHAR"
			displayWidth = definition.Length
		default:
			return nil, nil, fmt.Errorf("materialize network query: field %q has unsupported type %d", field, definition.Type)
		}
		if len(field) > displayWidth {
			displayWidth = len(field)
		}
		columns = append(columns, QueryColumn{Name: field, Type: typeName, DisplaySize: displayWidth + 1})
	}
	if err := scan.BeforeFirst(); err != nil {
		return nil, nil, fmt.Errorf("position query scan: %w", err)
	}
	for {
		available, err := scan.Next()
		if err != nil {
			return nil, nil, fmt.Errorf("advance query scan: %w", err)
		}
		if !available {
			break
		}
		row := make([]QueryValue, len(fields))
		for index, field := range fields {
			switch columns[index].Type {
			case "INTEGER":
				value, err := scan.GetInt(field)
				if err != nil {
					return nil, nil, fmt.Errorf("read query integer column %q: %w", field, err)
				}
				row[index] = QueryValue{Kind: "INTEGER", Integer: value}
			case "VARCHAR":
				value, err := scan.GetString(field)
				if err != nil {
					return nil, nil, fmt.Errorf("read query string column %q: %w", field, err)
				}
				row[index] = QueryValue{Kind: "VARCHAR", String: value}
			}
		}
		rows = append(rows, row)
	}
	return columns, rows, nil
}

func isNilNetworkQueryScan(scan query.Scan) bool {
	if scan == nil {
		return true
	}
	value := reflect.ValueOf(scan)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

// Close commits the active transaction and marks this RPC session closed without starting another one.
func (s *DriverService) Close(_ CloseRequest, reply *CloseResponse) error {
	if reply != nil {
		*reply = CloseResponse{}
	}
	if s == nil {
		return fmt.Errorf("close network RPC session: service is nil")
	}
	if reply == nil {
		return fmt.Errorf("close network RPC session: reply is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("close network RPC session: session is closed")
	}
	if s.transaction == nil {
		return fmt.Errorf("close network RPC session: active transaction is nil")
	}
	if _, err := s.transaction.Commit(); err != nil {
		return fmt.Errorf("close network RPC transaction: %w", err)
	}
	s.transaction = nil
	s.closed = true
	return nil
}

func (s *DriverService) disconnect() error {
	if s == nil {
		return fmt.Errorf("disconnect network RPC session: service is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.transaction == nil {
		return nil
	}
	active := s.transaction
	s.transaction = nil
	if _, err := active.Rollback(); err != nil {
		return fmt.Errorf("rollback active network RPC transaction after disconnect: %w", err)
	}
	return nil
}
