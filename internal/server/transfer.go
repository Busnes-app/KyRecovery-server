package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"time"
)

// Each transfer owns its deadlines. Only positive byte counts extend the idle window;
// the absolute limit never moves. Unsupported deadlines are normal for test recorders.
type transferDeadline struct {
	idle         time.Duration
	until, limit time.Time
	set          func(time.Time) error
}

func newTransferDeadline(set func(time.Time) error, idle, maximum time.Duration) *transferDeadline {
	now := time.Now()
	return &transferDeadline{idle: idle, until: now.Add(idle), limit: now.Add(maximum), set: set}
}

func (d *transferDeadline) beforeIO() error {
	until := d.until
	if d.limit.Before(until) {
		until = d.limit
	}
	if !time.Now().Before(until) {
		return os.ErrDeadlineExceeded
	}
	if err := d.set(until); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
}

func (d *transferDeadline) progress(n int) {
	if n > 0 {
		d.until = time.Now().Add(d.idle)
	}
}

type transferReader struct {
	io.Reader
	deadline *transferDeadline
}

func (r *transferReader) Read(p []byte) (int, error) {
	if err := r.deadline.beforeIO(); err != nil {
		return 0, err
	}
	n, err := r.Reader.Read(p[:min(len(p), 32<<10)])
	r.deadline.progress(n)
	return n, err
}

type transferWriter struct {
	http.ResponseWriter
	deadline *transferDeadline
}

func (w *transferWriter) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		if err := w.deadline.beforeIO(); err != nil {
			return total, err
		}
		chunk := p[:min(len(p), 32<<10)]
		n, err := w.ResponseWriter.Write(chunk)
		total += n
		w.deadline.progress(n)
		if err != nil {
			return total, err
		}
		if n != len(chunk) {
			return total, io.ErrShortWrite
		}
		p = p[n:]
	}
	return total, nil
}
