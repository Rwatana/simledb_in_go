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

// DriverService serves requests over one RPC connection and retains its transaction.
type DriverService struct {
	managers    *server.Managers
	mu          sync.Mutex
	transaction *transaction.Transaction
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
