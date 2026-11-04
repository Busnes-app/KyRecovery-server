package server

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestTransferDeadlineRenewsOnlyOnProgressAndNeverPastMaximum(t *testing.T) {
	var set time.Time
	d := newTransferDeadline(func(v time.Time) error { set = v; return nil }, capsuleTransferIdle, capsuleTransferMax)
	if capsuleTransferIdle != 5*time.Minute || capsuleTransferMax != 24*time.Hour {
		t.Fatal("transfer policy changed")
	}
	if err := d.beforeIO(); err != nil {
		t.Fatal(err)
	}
	initial := set
	d.until = d.until.Add(-time.Minute)
	before := d.until
	d.progress(0)
	if !d.until.Equal(before) {
		t.Fatal("zero-byte reads extended deadline")
	}
	d.progress(1)
	if err := d.beforeIO(); err != nil || !set.After(initial) {
		t.Fatalf("progress did not renew deadline: %v", err)
	}
	d.limit = time.Now().Add(time.Minute)
	if err := d.beforeIO(); err != nil || !set.Equal(d.limit) {
		t.Fatalf("absolute cap moved: %v", err)
	}
	d.limit = time.Now().Add(-time.Second)
	if err := d.beforeIO(); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expired cap: %v", err)
	}
	d = newTransferDeadline(func(time.Time) error { return http.ErrNotSupported }, time.Minute, time.Hour)
	if err := d.beforeIO(); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("deadline failure")
	d.set = func(time.Time) error { return boom }
	if err := d.beforeIO(); !errors.Is(err, boom) {
		t.Fatalf("deadline error lost: %v", err)
	}
}

type pipeResponse struct{ net.Conn }

func (w pipeResponse) Header() http.Header { return make(http.Header) }
func (w pipeResponse) WriteHeader(int)     {}

// Real blocked I/O proves both read and write deadlines fire. Progress continues
// beyond one idle window; a stalled peer and a peer that never stops are both bounded.
func TestTransferReadAndWriteSlidingDeadlines(t *testing.T) {
	for _, direction := range []string{"read", "write"} {
		for _, stall := range []bool{true, false} {
			t.Run(direction+map[bool]string{true: "/stall", false: "/maximum"}[stall], func(t *testing.T) {
				conn, peer := net.Pipe()
				defer conn.Close()
				defer peer.Close()
				done := make(chan struct{})
				go func() {
					defer close(done)
					defer peer.Close()
					for i := 0; i < 100; i++ {
						if stall && i == 3 {
							time.Sleep(500 * time.Millisecond)
							return
						}
						time.Sleep(50 * time.Millisecond)
						var err error
						if direction == "read" {
							_, err = peer.Write([]byte{'x'})
						} else {
							_, err = io.ReadFull(peer, make([]byte, 1))
						}
						if err != nil {
							return
						}
					}
				}()
				start := time.Now()
				var total int
				var err error
				if direction == "read" {
					r := &transferReader{Reader: conn, deadline: newTransferDeadline(conn.SetReadDeadline, 250*time.Millisecond, 700*time.Millisecond)}
					for err == nil {
						var n int
						n, err = r.Read(make([]byte, 1))
						total += n
					}
				} else {
					w := &transferWriter{ResponseWriter: pipeResponse{conn}, deadline: newTransferDeadline(conn.SetWriteDeadline, 250*time.Millisecond, 700*time.Millisecond)}
					for err == nil {
						var n int
						n, err = w.Write([]byte{'x'})
						total += n
					}
				}
				conn.Close()
				<-done
				if total < 3 || time.Since(start) > 2*time.Second {
					t.Fatalf("progress/bound failed: bytes=%d elapsed=%v err=%v", total, time.Since(start), err)
				}
				var timeout net.Error
				if !errors.As(err, &timeout) || !timeout.Timeout() {
					t.Fatalf("want timeout, got %v", err)
				}
			})
		}
	}
}

func TestTransferWriterChunksAndKeepsContent(t *testing.T) {
	body := bytes.Repeat([]byte("capsule"), 20000)
	recorder := httptest.NewRecorder()
	calls := 0
	d := newTransferDeadline(func(time.Time) error { calls++; return nil }, time.Minute, time.Hour)
	w := &transferWriter{ResponseWriter: recorder, deadline: d}
	if n, err := w.Write(body); err != nil || n != len(body) {
		t.Fatalf("write: %d %v", n, err)
	}
	if calls < 4 || !bytes.Equal(body, recorder.Body.Bytes()) {
		t.Fatal("stream was not chunked without altering bytes")
	}
}
