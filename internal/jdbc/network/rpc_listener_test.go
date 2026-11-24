package network

import (
	"errors"
	"net"
	"net/rpc"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestServeListenerKeepsConcurrentClientTransactionsIndependent(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	firstServerConn, firstClientConn := net.Pipe()
	secondServerConn, secondClientConn := net.Pipe()
	listener := &shutdownRPCListener{
		connections: []net.Conn{firstServerConn, secondServerConn},
		closed:      make(chan struct{}),
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- ServeListener(listener, managers) }()
	firstClient := rpc.NewClient(firstClientConn)
	secondClient := rpc.NewClient(secondClientConn)
	t.Cleanup(func() {
		_ = listener.Close()
		_ = firstClient.Close()
		_ = secondClient.Close()
	})
	firstSession, err := ConnectRPCSession(firstClient)
	if err != nil {
		t.Fatalf("ConnectRPCSession(first) error = %v", err)
	}
	secondSession, err := ConnectRPCSession(secondClient)
	if err != nil {
		t.Fatalf("ConnectRPCSession(second) error = %v", err)
	}
	if firstSession.TransactionID() == secondSession.TransactionID() {
		t.Fatalf("sessions share transaction ID %d", firstSession.TransactionID())
	}
	if err := firstSession.Close(); err != nil {
		t.Fatalf("first session Close() error = %v", err)
	}
	if err := firstClient.Close(); err != nil {
		t.Errorf("first rpc.Client.Close() error = %v", err)
	}
	if err := secondSession.Commit(); err != nil {
		t.Fatalf("second session Commit() after first closed error = %v", err)
	}
	if err := secondSession.Close(); err != nil {
		t.Fatalf("second session Close() error = %v", err)
	}
	if err := secondClient.Close(); err != nil {
		t.Errorf("second rpc.Client.Close() error = %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Errorf("listener.Close() error = %v", err)
	}
	select {
	case err := <-serveDone:
		if err != nil {
			t.Errorf("ServeListener() after caller closes listener error = %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("ServeListener() did not stop after caller closed the listener")
	}
}

func TestServeListenerRejectsNilInputsAndWrapsAcceptFailure(t *testing.T) {
	listener := &queuedRPCListener{acceptErr: net.ErrClosed}
	if err := ServeListener(nil, &server.Managers{}); err == nil || !strings.Contains(err.Error(), "listener is nil") {
		t.Errorf("ServeListener(nil, managers) error = %v, want listener error", err)
	}
	if err := ServeListener(listener, nil); err == nil || !strings.Contains(err.Error(), "manager set is nil") {
		t.Errorf("ServeListener(listener, nil) error = %v, want manager error", err)
	}
	if listener.acceptCalls != 0 {
		t.Errorf("Accept() calls after nil-input validation = %d, want 0", listener.acceptCalls)
	}

	cause := errors.New("accept interrupted")
	if err := ServeListener(&queuedRPCListener{acceptErr: cause}, &server.Managers{}); err == nil || !errors.Is(err, cause) {
		t.Errorf("ServeListener() error = %v, want wrapped accept error %v", err, cause)
	}
	if err := ServeListener(listener, &server.Managers{}); err != nil {
		t.Errorf("ServeListener() after caller closes listener error = %v, want nil", err)
	}
}

func TestServeListenerAcceptsWhileConnectionIsBeingServedAndClosesConnection(t *testing.T) {
	serverConn, peer := net.Pipe()
	tracked := &closeTrackingConn{Conn: serverConn, closed: make(chan struct{})}
	listener := &queuedRPCListener{connections: []net.Conn{tracked}, acceptErr: net.ErrClosed}
	started := make(chan struct{})
	release := make(chan struct{})
	served := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- serveListener(listener, &server.Managers{}, func(conn net.Conn, _ *server.Managers) error {
			if conn != tracked {
				return errors.New("unexpected accepted connection")
			}
			close(started)
			<-release
			close(served)
			return nil
		})
	}()
	defer peer.Close()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("accepted connection was not served")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveListener() shutdown error = %v, want nil", err)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("listener stopped accepting while its connection was still being served")
	}
	close(release)
	select {
	case <-served:
	case <-time.After(time.Second):
		t.Fatal("connection service did not return")
	}
	select {
	case <-tracked.closed:
	case <-time.After(time.Second):
		t.Fatal("accepted connection was not closed after its service returned")
	}
}

type queuedRPCListener struct {
	mu          sync.Mutex
	connections []net.Conn
	acceptErr   error
	acceptCalls int
}

func (listener *queuedRPCListener) Accept() (net.Conn, error) {
	listener.mu.Lock()
	defer listener.mu.Unlock()
	listener.acceptCalls++
	if len(listener.connections) == 0 {
		return nil, listener.acceptErr
	}
	conn := listener.connections[0]
	listener.connections = listener.connections[1:]
	return conn, nil
}

func (*queuedRPCListener) Close() error { return nil }

func (*queuedRPCListener) Addr() net.Addr { return queuedRPCAddr("test") }

type queuedRPCAddr string

func (addr queuedRPCAddr) Network() string { return "test" }

func (addr queuedRPCAddr) String() string { return string(addr) }

type shutdownRPCListener struct {
	mu          sync.Mutex
	connections []net.Conn
	closed      chan struct{}
	closeOnce   sync.Once
}

func (listener *shutdownRPCListener) Accept() (net.Conn, error) {
	listener.mu.Lock()
	if len(listener.connections) > 0 {
		conn := listener.connections[0]
		listener.connections = listener.connections[1:]
		listener.mu.Unlock()
		return conn, nil
	}
	closed := listener.closed
	listener.mu.Unlock()
	<-closed
	return nil, net.ErrClosed
}

func (listener *shutdownRPCListener) Close() error {
	listener.closeOnce.Do(func() { close(listener.closed) })
	return nil
}

func (*shutdownRPCListener) Addr() net.Addr { return queuedRPCAddr("shutdown") }

type closeTrackingConn struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
}

func (conn *closeTrackingConn) Close() error {
	conn.once.Do(func() { close(conn.closed) })
	return conn.Conn.Close()
}
