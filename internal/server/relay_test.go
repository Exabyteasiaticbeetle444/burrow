package server

import (
	"io"
	"net"
	"testing"
	"time"
)

func startEchoServer(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo server listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()
	return ln
}

func TestRelayForwardsData(t *testing.T) {
	echo := startEchoServer(t)
	defer echo.Close()

	echoAddr := echo.Addr().(*net.TCPAddr)

	relay, err := NewRelay(&RelayConfig{
		ListenPort:     0,
		UpstreamServer: "127.0.0.1",
		UpstreamPort:   uint16(echoAddr.Port),
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	// Override listener to use port 0 (random available port)
	relay.listener.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("relay listen: %v", err)
	}
	relay.listener = ln
	defer relay.Close()

	if err := relay.Start(); err != nil {
		t.Fatalf("start relay: %v", err)
	}

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial relay: %v", err)
	}
	defer conn.Close()

	msg := "hello relay"
	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, len(msg))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(buf) != msg {
		t.Errorf("expected %q, got %q", msg, string(buf))
	}
}

func TestRelayClosesCleanly(t *testing.T) {
	echo := startEchoServer(t)
	defer echo.Close()

	echoAddr := echo.Addr().(*net.TCPAddr)

	relay, err := NewRelay(&RelayConfig{
		ListenPort:     0,
		UpstreamServer: "127.0.0.1",
		UpstreamPort:   uint16(echoAddr.Port),
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	relay.listener.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("relay listen: %v", err)
	}
	relay.listener = ln

	if err := relay.Start(); err != nil {
		t.Fatalf("start relay: %v", err)
	}

	if err := relay.Close(); err != nil {
		t.Fatalf("close relay: %v", err)
	}

	// After close, new connections should be refused
	_, err = net.DialTimeout("tcp", ln.Addr().String(), 500*time.Millisecond)
	if err == nil {
		t.Error("expected connection refused after relay close")
	}
}

func TestRelayUpstreamUnreachable(t *testing.T) {
	// Relay pointing to a port with nothing listening
	relay, err := NewRelay(&RelayConfig{
		ListenPort:     0,
		UpstreamServer: "127.0.0.1",
		UpstreamPort:   19999,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	relay.listener.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("relay listen: %v", err)
	}
	relay.listener = ln
	defer relay.Close()

	if err := relay.Start(); err != nil {
		t.Fatalf("start relay: %v", err)
	}

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial relay: %v", err)
	}
	defer conn.Close()

	// Write some data — the relay should close the connection since upstream is unreachable
	conn.Write([]byte("hello"))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Error("expected error reading from relay with unreachable upstream")
	}
}

func TestNewRelayInvalidPort(t *testing.T) {
	// Port 0 should work (OS assigns)
	relay, err := NewRelay(&RelayConfig{
		ListenPort:     0,
		UpstreamServer: "127.0.0.1",
		UpstreamPort:   443,
	})
	if err != nil {
		t.Fatalf("new relay with port 0: %v", err)
	}
	relay.Close()
}

func TestRelayMultipleConnections(t *testing.T) {
	echo := startEchoServer(t)
	defer echo.Close()

	echoAddr := echo.Addr().(*net.TCPAddr)

	relay, err := NewRelay(&RelayConfig{
		ListenPort:     0,
		UpstreamServer: "127.0.0.1",
		UpstreamPort:   uint16(echoAddr.Port),
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	relay.listener.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("relay listen: %v", err)
	}
	relay.listener = ln
	defer relay.Close()

	if err := relay.Start(); err != nil {
		t.Fatalf("start relay: %v", err)
	}

	for i := 0; i < 5; i++ {
		conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}

		msg := []byte("concurrent test")
		conn.Write(msg)
		buf := make([]byte, len(msg))
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		io.ReadFull(conn, buf)
		conn.Close()

		if string(buf) != string(msg) {
			t.Errorf("connection %d: expected %q, got %q", i, msg, buf)
		}
	}
}
