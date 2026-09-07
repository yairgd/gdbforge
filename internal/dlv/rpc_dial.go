package dlv

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-delve/delve/service/rpc2"
)

// DialRPC connects to a Delve headless JSON-RPC server.
func DialRPC(addr string, timeout time.Duration) (*rpc2.RPCClient, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := dialRPCConn(addr)
		if err == nil {
			return rpc2.NewClientFromConn(conn), nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("dlv rpc dial %s: %w", addr, lastErr)
}

func dialRPCConn(addr string) (net.Conn, error) {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, "unix:") {
		return net.DialTimeout("unix", strings.TrimPrefix(addr, "unix:"), 2*time.Second)
	}
	return net.DialTimeout("tcp", normalizeConnectAddr(addr), 2*time.Second)
}

// PickListenAddr returns a free localhost TCP address for headless dlv.
func PickListenAddr() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr, nil
}
