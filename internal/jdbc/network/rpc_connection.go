package network

import (
	"fmt"
	"io"
	"net"
	"net/rpc"
	"sync"

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
