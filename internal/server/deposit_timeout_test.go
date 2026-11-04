package server_test

import (
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/ky-primitives/recoverykey"
)

// Keep the production transfer policy unchanged while making a real socket's
// nonzero read deadlines short enough to exercise a stalled deposit in a test.
type shortDepositListener struct{ net.Listener }
type shortDepositConn struct{ net.Conn }

func (l shortDepositListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return shortDepositConn{conn}, nil
}

func (c shortDepositConn) SetReadDeadline(deadline time.Time) error {
	if sooner := time.Now().Add(100 * time.Millisecond); !deadline.IsZero() && deadline.After(sooner) {
		deadline = sooner
	}
	return c.Conn.SetReadDeadline(deadline)
}

func TestTimedOutIncompleteDepositClosesHTTP1Connection(t *testing.T) {
	srv, cookie, _ := newAdminServer(t)
	key, err := recoverykey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	importKey(t, srv, cookie, key.Public(), 3, 5)
	token, _ := pairProduct(t, srv, cookie, "timeout-test")

	httpServer := httptest.NewUnstartedServer(srv)
	httpServer.Listener = shortDepositListener{httpServer.Listener}
	httpServer.Start()
	defer httpServer.Close()
	conn, err := net.Dial("tcp", httpServer.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// Seven bytes remain: net/http's small-body response drain must not wait for
	// them forever after the handler's body read has already timed out.
	if _, err := fmt.Fprintf(conn, "POST /api/backup/deposit HTTP/1.1\r\nHost: recovery.test\r\nAuthorization: Bearer %s\r\nContent-Type: application/octet-stream\r\nContent-Length: 8\r\n\r\nx", token); err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("stalled deposit connection did not close without more body bytes: %v", err)
	}
	if !strings.HasPrefix(string(response), "HTTP/1.1 400 ") {
		t.Fatalf("unexpected timeout response: %q", response)
	}
}
