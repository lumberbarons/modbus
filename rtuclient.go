// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package modbus

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"
)

const (
	rtuMinSize = 4
	rtuMaxSize = 256

	rtuExceptionSize = 5
)

// RTUClientHandler implements Packager and Transporter interface.
type RTUClientHandler struct {
	rtuPackager
	rtuSerialTransporter
}

// NewRTUClientHandler allocates and initializes a RTUClientHandler.
func NewRTUClientHandler(address string) *RTUClientHandler {
	handler := &RTUClientHandler{}
	handler.Address = address
	handler.BaudRate = 19200
	handler.DataBits = 8
	handler.StopBits = OneStopBit
	handler.Parity = EvenParity
	handler.Timeout = serialTimeout
	handler.IdleTimeout = serialIdleTimeout
	return handler
}

// RTUClient creates RTU client with default handler and given connect string.
func RTUClient(address string) Client {
	handler := NewRTUClientHandler(address)
	return NewClient(handler)
}

// rtuPackager implements Packager interface.
type rtuPackager struct {
	SlaveID byte
}

// Encode encodes PDU in a RTU frame:
//
//	Slave Address   : 1 byte
//	Function        : 1 byte
//	Data            : 0 up to 252 bytes
//	CRC             : 2 byte
func (mb *rtuPackager) Encode(pdu *ProtocolDataUnit) (adu []byte, err error) {
	length := len(pdu.Data) + 4
	if length > rtuMaxSize {
		return nil, fmt.Errorf("%w: length of data '%v' must not be bigger than '%v'", ErrInvalidData, length, rtuMaxSize)
	}
	adu = make([]byte, length)

	adu[0] = mb.SlaveID
	adu[1] = pdu.FunctionCode
	copy(adu[2:], pdu.Data)

	// Append crc
	var crc crc
	crc.reset().pushBytes(adu[0 : length-2])
	checksum := crc.value()

	adu[length-1] = byte(checksum >> 8)
	adu[length-2] = byte(checksum)
	return adu, nil
}

// Verify verifies response length and slave id.
func (mb *rtuPackager) Verify(aduRequest, aduResponse []byte) (err error) {
	length := len(aduResponse)
	// Minimum size (including address, function and CRC)
	if length < rtuMinSize {
		return fmt.Errorf("%w: response length '%v' does not meet minimum '%v'", ErrShortFrame, length, rtuMinSize)
	}
	// Slave address must match
	if aduResponse[0] != aduRequest[0] {
		return fmt.Errorf("%w: response slave id '%v' does not match request '%v'", ErrProtocolError, aduResponse[0], aduRequest[0])
	}
	return nil
}

// Decode extracts PDU from RTU frame and verify CRC.
func (mb *rtuPackager) Decode(adu []byte) (pdu *ProtocolDataUnit, err error) {
	length := len(adu)
	// Calculate checksum
	var crc crc
	crc.reset().pushBytes(adu[0 : length-2])
	checksum := uint16(adu[length-1])<<8 | uint16(adu[length-2])
	if checksum != crc.value() {
		return nil, fmt.Errorf("%w: response crc '%v' does not match expected '%v'", ErrProtocolError, checksum, crc.value())
	}
	// Function code & data
	pdu = &ProtocolDataUnit{}
	pdu.FunctionCode = adu[1]
	pdu.Data = adu[2 : length-2]
	return pdu, nil
}

// rtuSerialTransporter implements Transporter interface.
type rtuSerialTransporter struct {
	serialPort
}

// Send transmits an RTU request and receives the response.
// This implementation uses Read() in a loop with context checks between iterations,
// rather than io.ReadFull(). This approach:
//   - Prevents indefinite hangs when devices send incomplete responses
//   - Allows context cancellation to be detected between read operations
//   - Improves reliability on systems where serial port timeouts are not well-supported
//
// Note: Individual Read() calls may still block if the underlying device/driver
// doesn't support read timeouts (e.g., PTYs in tests). However, context is checked
// between reads, providing better timeout behavior than the previous io.ReadFull() approach.
func (mb *rtuSerialTransporter) Send(ctx context.Context, aduRequest []byte) (aduResponse []byte, err error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	// Check context before starting
	if err = ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before send: %w", err)
	}

	// Make sure port is connected
	if err = mb.connect(); err != nil {
		return nil, fmt.Errorf("connecting: %w", err)
	}

	// Check context after connect
	if err = ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	// Start the timer to close when idle
	mb.lastActivity = time.Now()
	mb.startCloseTimer()

	// Send the request
	mb.logf("modbus: sending % x\n", aduRequest)
	if _, err = mb.port.Write(aduRequest); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	// Check context after write
	if err = ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	function := aduRequest[1]
	functionFail := aduRequest[1] & 0x80
	bytesToRead := calculateResponseLength(aduRequest)
	time.Sleep(mb.calculateDelay(len(aduRequest) + bytesToRead))

	// Check context after delay
	if err = ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	// Set read timeout based on context deadline
	readTimeout := mb.Timeout
	if deadline, ok := ctx.Deadline(); ok {
		timeUntilDeadline := time.Until(deadline)
		if timeUntilDeadline > 0 {
			readTimeout = timeUntilDeadline
		} else {
			return nil, fmt.Errorf("context deadline exceeded before read")
		}
	}
	if err = mb.port.SetReadTimeout(readTimeout); err != nil {
		return nil, fmt.Errorf("setting read timeout: %w", err)
	}

	// Restore original timeout after reads complete
	defer func() {
		if restoreErr := mb.port.SetReadTimeout(mb.Timeout); restoreErr != nil {
			mb.logf("modbus: warning - failed to restore read timeout: %v\n", restoreErr)
		}
	}()

	var n int
	var data [rtuMaxSize]byte

	// Read minimum length with context checks between reads.
	// We use Read() in a loop instead of ReadAtLeast() to allow
	// context cancellation during the read operation.
	for n < rtuMinSize {
		// Check context before each read iteration
		if err = ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled during read: %w", err)
		}

		var nn int
		nn, err = mb.port.Read(data[n:])
		n += nn
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
		if nn == 0 && n < rtuMinSize {
			// No more data available and we haven't reached minimum length
			return nil, fmt.Errorf("reading response: unexpected EOF, got %d bytes, expected at least %d", n, rtuMinSize)
		}
	}

	// Determine how many total bytes we need based on response type
	var targetLength int
	switch data[1] {
	case function:
		targetLength = bytesToRead
	case functionFail:
		targetLength = rtuExceptionSize
	default:
		targetLength = n // Unknown function, use what we have
	}

	// Read remaining bytes with context checks between reads
	if targetLength > rtuMinSize && targetLength <= rtuMaxSize {
		for n < targetLength {
			// Check context before each read iteration
			if err = ctx.Err(); err != nil {
				return nil, fmt.Errorf("context cancelled during read: %w", err)
			}

			var nn int
			nn, err = mb.port.Read(data[n:targetLength])
			n += nn
			if err != nil {
				return nil, fmt.Errorf("reading response body: %w", err)
			}
			if nn == 0 {
				// No more data available and we haven't reached target length
				return nil, fmt.Errorf("reading response body: unexpected EOF, got %d bytes, expected %d", n, targetLength)
			}
		}
	}
	aduResponse = data[:n]
	mb.logf("modbus: received % x\n", aduResponse)
	return aduResponse, nil
}

// calculateDelay roughly calculates time needed for the next frame.
// See MODBUS over Serial Line - Specification and Implementation Guide (page 13).
func (mb *rtuSerialTransporter) calculateDelay(chars int) time.Duration {
	var characterDelay, frameDelay int // us

	if mb.BaudRate <= 0 || mb.BaudRate > 19200 {
		characterDelay = 750
		frameDelay = 1750
	} else {
		characterDelay = 15000000 / mb.BaudRate
		frameDelay = 35000000 / mb.BaudRate
	}
	return time.Duration(characterDelay*chars+frameDelay) * time.Microsecond
}

func calculateResponseLength(adu []byte) int {
	length := rtuMinSize
	switch adu[1] {
	case FuncCodeReadDiscreteInputs,
		FuncCodeReadCoils:
		count := int(binary.BigEndian.Uint16(adu[4:]))
		length += 1 + count/8
		if count%8 != 0 {
			length++
		}
	case FuncCodeReadInputRegisters,
		FuncCodeReadHoldingRegisters,
		FuncCodeReadWriteMultipleRegisters:
		count := int(binary.BigEndian.Uint16(adu[4:]))
		length += 1 + count*2
	case FuncCodeWriteSingleCoil,
		FuncCodeWriteMultipleCoils,
		FuncCodeWriteSingleRegister,
		FuncCodeWriteMultipleRegisters:
		length += 4
	case FuncCodeMaskWriteRegister:
		length += 6
	case FuncCodeReadFIFOQueue:
		// undetermined
	default:
	}
	return length
}
