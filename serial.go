// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package modbus

import (
	"log"
	"sync"
	"time"

	"go.bug.st/serial"
)

const (
	// Default timeout
	serialTimeout     = 5 * time.Second
	serialIdleTimeout = 60 * time.Second
)

// serialPort has configuration and I/O controller.
type serialPort struct {
	// Serial port configuration.
	Address     string
	BaudRate    int
	DataBits    int
	StopBits    StopBits
	Parity      Parity
	Timeout     time.Duration
	Logger      *log.Logger
	IdleTimeout time.Duration

	mu sync.Mutex
	// port is platform-dependent data structure for serial port.
	port         serial.Port
	lastActivity time.Time
	closeTimer   *time.Timer
}

// toSerialStopBits converts modbus StopBits to serial library StopBits.
func toSerialStopBits(sb StopBits) serial.StopBits {
	switch sb {
	case TwoStopBits:
		return serial.TwoStopBits
	default:
		return serial.OneStopBit
	}
}

// toSerialParity converts modbus Parity to serial library Parity.
func toSerialParity(p Parity) serial.Parity {
	switch p {
	case NoParity:
		return serial.NoParity
	case OddParity:
		return serial.OddParity
	default:
		return serial.EvenParity
	}
}

func (mb *serialPort) Connect() (err error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	return mb.connect()
}

// connect connects to the serial port if it is not connected. Caller must hold the mutex.
func (mb *serialPort) connect() error {
	if mb.port == nil {
		mode := &serial.Mode{
			BaudRate: mb.BaudRate,
			DataBits: mb.DataBits,
			StopBits: toSerialStopBits(mb.StopBits),
			Parity:   toSerialParity(mb.Parity),
		}
		port, err := serial.Open(mb.Address, mode)
		if err != nil {
			return err
		}
		if mb.Timeout > 0 {
			err = port.SetReadTimeout(mb.Timeout)
			if err != nil {
				port.Close()
				return err
			}
		}
		mb.port = port
	}
	return nil
}

func (mb *serialPort) Close() (err error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	return mb.close()
}

// close closes the serial port if it is connected. Caller must hold the mutex.
func (mb *serialPort) close() (err error) {
	if mb.port != nil {
		err = mb.port.Close()
		mb.port = nil
	}
	return
}

func (mb *serialPort) logf(format string, v ...interface{}) {
	if mb.Logger != nil {
		mb.Logger.Printf(format, v...)
	}
}

func (mb *serialPort) startCloseTimer() {
	if mb.IdleTimeout <= 0 {
		return
	}
	if mb.closeTimer == nil {
		mb.closeTimer = time.AfterFunc(mb.IdleTimeout, mb.closeIdle)
	} else {
		mb.closeTimer.Reset(mb.IdleTimeout)
	}
}

// closeIdle closes the connection if last activity is passed behind IdleTimeout.
func (mb *serialPort) closeIdle() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if mb.IdleTimeout <= 0 {
		return
	}
	idle := time.Since(mb.lastActivity)
	if idle >= mb.IdleTimeout {
		mb.logf("modbus: closing connection due to idle timeout: %v", idle)
		mb.close()
	}
}
