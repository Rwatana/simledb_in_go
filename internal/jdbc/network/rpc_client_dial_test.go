package network

import (
	"context"
	"errors"
	"net"
	"net/rpc"
	"strings"
	"testing"
)

func TestDialNetworkRPCClientUsesContextAndReturnsCallerOwnedClient(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName("SimpleDB", &rpcClientDialProbe{}); err != nil {
		t.Fatalf("RegisterName() error = %v", err)
	}
	done := make(chan struct{})
	go func() {
		rpcServer.ServeConn(serverConn)
		close(done)
	}()
	dialer := &fakeContextDialer{conn: clientConn}
	ctx := context.WithValue(context.Background(), struct{}{}, "dial context")
	client, err := dialNetworkRPCClient(ctx, "jdbc:simpledb://db.example.org", dialer)
	if err != nil {
		t.Fatalf("dialNetworkRPCClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("dialNetworkRPCClient() returned nil client")
	}
	if dialer.call.ctx != ctx || dialer.call.network != "tcp" || dialer.call.address != "db.example.org:1099" {
		t.Errorf("DialContext() = (%v, %q, %q), want caller context, tcp, db.example.org:1099", dialer.call.ctx, dialer.call.network, dialer.call.address)
	}
	var response ConnectResponse
	if err := client.Call("SimpleDB.Connect", ConnectRequest{}, &response); err != nil {
		t.Fatalf("caller-owned rpc.Client.Call() error = %v", err)
	}
	if response.TransactionID != 73 {
		t.Errorf("RPC response transaction ID = %d, want 73", response.TransactionID)
	}
	if err := client.Close(); err != nil {
		t.Errorf("caller-owned rpc.Client.Close() error = %v", err)
	}
	<-done
}

func TestDialNetworkRPCClientRejectsInputsAndWrapsDialFailures(t *testing.T) {
	cause := errors.New("network unavailable")
	for _, test := range []struct {
		name   string
		ctx    context.Context
		url    string
		dialer contextDialer
		want   string
		cause  error
	}{
		{name: "nil context", url: "jdbc:simpledb://localhost", dialer: &fakeContextDialer{}, want: "context is nil"},
		{name: "invalid URL", ctx: context.Background(), url: "jdbc:simpledb://localhost:1099", dialer: &fakeContextDialer{}, want: "port"},
		{name: "nil dialer", ctx: context.Background(), url: "jdbc:simpledb://localhost", want: "dialer is nil"},
		{name: "dial failure", ctx: context.Background(), url: "jdbc:simpledb://localhost", dialer: &fakeContextDialer{err: cause}, want: "network unavailable", cause: cause},
		{name: "nil connection", ctx: context.Background(), url: "jdbc:simpledb://localhost", dialer: &fakeContextDialer{}, want: "nil connection"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := dialNetworkRPCClient(test.ctx, test.url, test.dialer)
			if err == nil || client != nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("dialNetworkRPCClient() = (%v, %v), want nil client and error containing %q", client, err, test.want)
			}
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Errorf("dialNetworkRPCClient() error = %v, want cause %v", err, test.cause)
			}
		})
	}
}

func TestDialNetworkRPCClientHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := DialNetworkRPCClient(ctx, "jdbc:simpledb://127.0.0.1")
	if err == nil || client != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("DialNetworkRPCClient() = (%v, %v), want nil client and context.Canceled", client, err)
	}
}

type rpcClientDialProbe struct{}

func (*rpcClientDialProbe) Connect(_ ConnectRequest, reply *ConnectResponse) error {
	reply.TransactionID = 73
	return nil
}
