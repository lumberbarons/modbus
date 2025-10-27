package modbus

import (
	"bytes"
	"io"
	"testing"
	"time"
)

type nopCloser struct {
	io.ReadWriter

	closed bool
}

func (n *nopCloser) Close() error {
	n.closed = true
	return nil
}

func TestSerialCloseIdle(t *testing.T) {
	port := &nopCloser{
		ReadWriter: &bytes.Buffer{},
	}
	s := serialPort{
		port:        port,
		IdleTimeout: 100 * time.Millisecond,
	}
	s.lastActivity = time.Now()
	s.startCloseTimer()

	time.Sleep(150 * time.Millisecond)
	s.mu.Lock()
	closed := port.closed
	portNil := s.port == nil
	s.mu.Unlock()
	if !closed || !portNil {
		t.Fatalf("serial port is not closed when inactivity: %+v", port)
	}
}
