package network

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
)

const defaultRegistryPort = "1099"

type contextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// DialNetworkURL opens a TCP connection to a network database URL's host on the default registry port.
// The caller owns the returned connection and must close it.
func DialNetworkURL(ctx context.Context, dataSourceName string) (net.Conn, error) {
	return dialNetworkURL(ctx, dataSourceName, &net.Dialer{})
}

// DialNetworkRPCClient opens a Go net/rpc client for the URL host on the default registry port.
// The caller owns the returned client and must close it.
func DialNetworkRPCClient(ctx context.Context, dataSourceName string) (*rpc.Client, error) {
	return dialNetworkRPCClient(ctx, dataSourceName, &net.Dialer{})
}

func dialNetworkRPCClient(ctx context.Context, dataSourceName string, dialer contextDialer) (*rpc.Client, error) {
	conn, err := dialNetworkURL(ctx, dataSourceName, dialer)
	if err != nil {
		return nil, fmt.Errorf("dial network RPC client: %w", err)
	}
	return rpc.NewClient(conn), nil
}

func dialNetworkURL(ctx context.Context, dataSourceName string, dialer contextDialer) (net.Conn, error) {
	if ctx == nil {
		return nil, fmt.Errorf("dial network database: context is nil")
	}
	if dialer == nil {
		return nil, fmt.Errorf("dial network database: dialer is nil")
	}
	host, err := ParseNetworkURL(dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("dial network database: %w", err)
	}
	address := net.JoinHostPort(host, defaultRegistryPort)
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial network database host %q: %w", host, err)
	}
	if conn == nil {
		return nil, fmt.Errorf("dial network database host %q: dialer returned a nil connection", host)
	}
	return conn, nil
}
