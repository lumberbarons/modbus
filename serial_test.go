package modbus

import (
	"bytes"
	"io"
	"testing"
	"time"

	"go.bug.st/serial"
)

type nopCloser struct {
	io.ReadWriter

	closed bool
}

func (n *nopCloser) Close() error {
	n.closed = true
	return nil
}

func (n *nopCloser) SetMode(_ *serial.Mode) error {
	return nil
}

func (n *nopCloser) Drain() error {
	return nil
}

func (n *nopCloser) ResetInputBuffer() error {
	return nil
}

func (n *nopCloser) ResetOutputBuffer() error {
	return nil
}

func (n *nopCloser) SetDTR(_ bool) error {
	return nil
}

func (n *nopCloser) SetRTS(_ bool) error {
	return nil
}

func (n *nopCloser) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}

func (n *nopCloser) SetReadTimeout(_ time.Duration) error {
	return nil
}

func (n *nopCloser) Break(_ time.Duration) error {
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
