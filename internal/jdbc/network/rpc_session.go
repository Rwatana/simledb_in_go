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

// TransactionID returns the active remote transaction ID, or zero for a nil session.
func (s *RemoteSession) TransactionID() int64 {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.transactionID
}
