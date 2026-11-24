package network

import (
	"context"
	"errors"
	"net"
	"testing"
)

type dialCall struct {
	ctx     context.Context
	network string
	address string
}

type fakeContextDialer struct {
	conn net.Conn
	err  error
	call dialCall
}

func (d *fakeContextDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.call = dialCall{ctx: ctx, network: network, address: address}
	return d.conn, d.err
}

func TestDialNetworkURLUsesContextAndDefaultPort(t *testing.T) {
	for _, test := range []struct {
		name string
		url  string
		want string
	}{
		{name: "dns host", url: "jdbc:simpledb://Db.Example.org", want: "Db.Example.org:1099"},
		{name: "ipv4 host", url: "jdbc:simpledb://127.0.0.1", want: "127.0.0.1:1099"},
		{name: "ipv6 host", url: "jdbc:simpledb://[2001:db8::1]", want: "[2001:db8::1]:1099"},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn, peer := net.Pipe()
			defer peer.Close()
			dialer := &fakeContextDialer{conn: conn}
			ctx := context.WithValue(context.Background(), struct{}{}, "caller-context")
			got, err := dialNetworkURL(ctx, test.url, dialer)
			if err != nil {
				t.Fatalf("dialNetworkURL() error = %v", err)
			}
			defer got.Close()
			if got != conn {
				t.Fatal("dialNetworkURL() did not return the dialer's connection")
			}
			if dialer.call.ctx != ctx || dialer.call.network != "tcp" || dialer.call.address != test.want {
				t.Errorf("DialContext() = (%v, %q, %q), want caller context, tcp, %q", dialer.call.ctx, dialer.call.network, dialer.call.address, test.want)
			}
		})
	}
}

func TestDialNetworkURLRejectsInvalidInputsBeforeDial(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name string
		ctx  context.Context
		url  string
	}{
		{name: "nil context", url: "jdbc:simpledb://localhost"},
		{name: "invalid URL", ctx: ctx, url: "jdbc:simpledb://localhost:1099"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dialer := &fakeContextDialer{}
			if conn, err := dialNetworkURL(test.ctx, test.url, dialer); err == nil || conn != nil {
				t.Fatalf("dialNetworkURL() = (%v, %v), want error and nil connection", conn, err)
			}
			if dialer.call.network != "" {
				t.Errorf("DialContext() called before validation: %+v", dialer.call)
			}
		})
	}
	if conn, err := DialNetworkURL(nil, "jdbc:simpledb://localhost"); err == nil || conn != nil {
		t.Fatalf("DialNetworkURL(nil, valid URL) = (%v, %v), want error and nil connection", conn, err)
	}
}

func TestDialNetworkURLWrapsDialFailure(t *testing.T) {
	cause := errors.New("connection refused")
	dialer := &fakeContextDialer{err: cause}
	conn, err := dialNetworkURL(context.Background(), "jdbc:simpledb://db.example.org", dialer)
	if err == nil || !errors.Is(err, cause) || conn != nil {
		t.Fatalf("dialNetworkURL() = (%v, %v), want wrapped cause and nil connection", conn, err)
	}
	if dialer.call.address != "db.example.org:1099" {
		t.Errorf("DialContext() address = %q, want db.example.org:1099", dialer.call.address)
	}
}

func TestDialNetworkURLRejectsNilConnection(t *testing.T) {
	conn, err := dialNetworkURL(context.Background(), "jdbc:simpledb://localhost", &fakeContextDialer{})
	if err == nil || conn != nil {
		t.Fatalf("dialNetworkURL() = (%v, %v), want error and nil connection", conn, err)
	}
}

func TestDialNetworkURLRejectsNilDialer(t *testing.T) {
	conn, err := dialNetworkURL(context.Background(), "jdbc:simpledb://localhost", nil)
	if err == nil || conn != nil {
		t.Fatalf("dialNetworkURL() = (%v, %v), want error and nil connection", conn, err)
	}
}

func TestDialNetworkURLHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn, err := DialNetworkURL(ctx, "jdbc:simpledb://127.0.0.1")
	if err == nil || !errors.Is(err, context.Canceled) || conn != nil {
		t.Fatalf("DialNetworkURL() = (%v, %v), want context.Canceled and nil connection", conn, err)
	}
}
