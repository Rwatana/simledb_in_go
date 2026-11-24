package network

import (
	"fmt"
	"net/rpc"
	"sync"
)

// RemoteSession retains the caller-owned RPC client and transaction ID for one remote session.
type RemoteSession struct {
	mu            sync.RWMutex
	client        *rpc.Client
	transactionID int64
	closed        bool
}

// ConnectRPCSession requests one server-side transaction over the supplied RPC client.
// The caller retains ownership of the client and its underlying transport.
func ConnectRPCSession(client *rpc.Client) (*RemoteSession, error) {
	if client == nil {
		return nil, fmt.Errorf("connect network RPC client session: client is nil")
	}
	var response ConnectResponse
	if err := client.Call("SimpleDB.Connect", ConnectRequest{}, &response); err != nil {
		return nil, fmt.Errorf("connect network RPC client session: %w", err)
	}
	return &RemoteSession{client: client, transactionID: response.TransactionID}, nil
}

// TransactionID returns the active remote transaction ID, or zero for a nil or closed session.
func (s *RemoteSession) TransactionID() int64 {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return 0
	}
	return s.transactionID
}

// Commit commits the remote transaction and stores the server's successor transaction ID.
func (s *RemoteSession) Commit() error {
	if s == nil {
		return fmt.Errorf("commit network RPC client session: session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("commit network RPC client session: session is closed")
	}
	if s.client == nil {
		return fmt.Errorf("commit network RPC client session: client is nil")
	}
	var response CommitResponse
	if err := s.client.Call("SimpleDB.Commit", CommitRequest{}, &response); err != nil {
		return fmt.Errorf("commit network RPC client session: %w", err)
	}
	s.transactionID = response.TransactionID
	return nil
}

// Rollback rolls back the remote transaction and stores the server's successor transaction ID.
func (s *RemoteSession) Rollback() error {
	if s == nil {
		return fmt.Errorf("roll back network RPC client session: session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("roll back network RPC client session: session is closed")
	}
	if s.client == nil {
		return fmt.Errorf("roll back network RPC client session: client is nil")
	}
	var response RollbackResponse
	if err := s.client.Call("SimpleDB.Rollback", RollbackRequest{}, &response); err != nil {
		return fmt.Errorf("roll back network RPC client session: %w", err)
	}
	s.transactionID = response.TransactionID
	return nil
}

// ExecuteUpdate runs a SQL update in the active remote transaction and returns its affected-row count.
func (s *RemoteSession) ExecuteUpdate(statement string) (int64, error) {
	if s == nil {
		return 0, fmt.Errorf("execute network RPC client update: session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, fmt.Errorf("execute network RPC client update: session is closed")
	}
	if s.client == nil {
		return 0, fmt.Errorf("execute network RPC client update: client is nil")
	}
	var response ExecuteUpdateResponse
	if err := s.client.Call("SimpleDB.ExecuteUpdate", ExecuteUpdateRequest{Statement: statement}, &response); err != nil {
		return 0, fmt.Errorf("execute network RPC client update: %w", err)
	}
	return response.RowsAffected, nil
}

// ExecuteQuery returns typed, schema-ordered rows from the active remote transaction.
func (s *RemoteSession) ExecuteQuery(statement string) (ExecuteQueryResponse, error) {
	if s == nil {
		return ExecuteQueryResponse{}, fmt.Errorf("execute network RPC client query: session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ExecuteQueryResponse{}, fmt.Errorf("execute network RPC client query: session is closed")
	}
	if s.client == nil {
		return ExecuteQueryResponse{}, fmt.Errorf("execute network RPC client query: client is nil")
	}
	var response ExecuteQueryResponse
	if err := s.client.Call("SimpleDB.ExecuteQuery", ExecuteQueryRequest{Statement: statement}, &response); err != nil {
		return ExecuteQueryResponse{}, fmt.Errorf("execute network RPC client query: %w", err)
	}
	return response, nil
}

// Close closes the remote database session without closing the caller-owned RPC client.
func (s *RemoteSession) Close() error {
	if s == nil {
		return fmt.Errorf("close network RPC client session: session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("close network RPC client session: session is closed")
	}
	if s.client == nil {
		return fmt.Errorf("close network RPC client session: client is nil")
	}
	if err := s.client.Call("SimpleDB.Close", CloseRequest{}, &CloseResponse{}); err != nil {
		return fmt.Errorf("close network RPC client session: %w", err)
	}
	s.closed = true
	s.transactionID = 0
	return nil
}
