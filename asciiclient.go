// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package modbus

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	asciiStart   = ":"
	asciiEnd     = "\r\n"
	asciiMinSize = 3
	asciiMaxSize = 513

	hexTable = "0123456789ABCDEF"
)

// ASCIIClientHandler implements Packager and Transporter interface.
type ASCIIClientHandler struct {
	asciiPackager
	asciiSerialTransporter
}

// NewASCIIClientHandler allocates and initializes a ASCIIClientHandler.
func NewASCIIClientHandler(address string) *ASCIIClientHandler {
	handler := &ASCIIClientHandler{}
	handler.Address = address
	handler.BaudRate = 19200
	handler.DataBits = 8
	handler.StopBits = OneStopBit
	handler.Parity = EvenParity
	handler.Timeout = serialTimeout
	handler.IdleTimeout = serialIdleTimeout
	return handler
}

// ASCIIClient creates ASCII client with default handler and given connect string.
func ASCIIClient(address string) Client {
	handler := NewASCIIClientHandler(address)
	return NewClient(handler)
}

// asciiPackager implements Packager interface.
type asciiPackager struct {
	SlaveID byte
}

// Encode encodes PDU in a ASCII frame:
//
//	Start           : 1 char
//	Address         : 2 chars
//	Function        : 2 chars
//	Data            : 0 up to 2x252 chars
//	LRC             : 2 chars
//	End             : 2 chars
func (mb *asciiPackager) Encode(pdu *ProtocolDataUnit) (adu []byte, err error) {
	var buf bytes.Buffer

	if _, err = buf.WriteString(asciiStart); err != nil {
		return nil, fmt.Errorf("writing start: %w", err)
	}
	if err = writeHex(&buf, []byte{mb.SlaveID, pdu.FunctionCode}); err != nil {
		return nil, fmt.Errorf("writing header: %w", err)
	}
	if err = writeHex(&buf, pdu.Data); err != nil {
		return nil, fmt.Errorf("writing data: %w", err)
	}
	// Exclude the beginning colon and terminating CRLF pair characters
	var lrc lrc
	lrc.reset()
	lrc.pushByte(mb.SlaveID).pushByte(pdu.FunctionCode).pushBytes(pdu.Data)
	if err = writeHex(&buf, []byte{lrc.value()}); err != nil {
		return nil, fmt.Errorf("writing LRC: %w", err)
	}
	if _, err = buf.WriteString(asciiEnd); err != nil {
		return nil, fmt.Errorf("writing end: %w", err)
	}
	return buf.Bytes(), nil
}

// Verify verifies response length, frame boundary and slave id.
func (mb *asciiPackager) Verify(aduRequest, aduResponse []byte) (err error) {
	length := len(aduResponse)
	// Minimum size (including address, function and LRC)
	if length < asciiMinSize+6 {
		return fmt.Errorf("%w: response length '%v' does not meet minimum '%v'", ErrShortFrame, length, 9)
	}
	// Length excluding colon must be an even number
	if length%2 != 1 {
		return fmt.Errorf("%w: response length '%v' is not an even number", ErrProtocolError, length-1)
	}
	// First char must be a colon
	str := string(aduResponse[0:len(asciiStart)])
	if str != asciiStart {
		return fmt.Errorf("%w: response frame '%v'... is not started with '%v'", ErrProtocolError, str, asciiStart)
	}
	// 2 last chars must be \r\n
	str = string(aduResponse[len(aduResponse)-len(asciiEnd):])
	if str != asciiEnd {
		return fmt.Errorf("%w: response frame ...'%v' is not ended with '%v'", ErrProtocolError, str, asciiEnd)
	}
	// Slave id
	responseVal, err := readHex(aduResponse[1:])
	if err != nil {
		return fmt.Errorf("reading response slave id: %w", err)
	}
	requestVal, err := readHex(aduRequest[1:])
	if err != nil {
		return fmt.Errorf("reading request slave id: %w", err)
	}
	if responseVal != requestVal {
		return fmt.Errorf("%w: response slave id '%v' does not match request '%v'", ErrProtocolError, responseVal, requestVal)
	}
	return nil
}

// Decode extracts PDU from ASCII frame and verify LRC.
func (mb *asciiPackager) Decode(adu []byte) (pdu *ProtocolDataUnit, err error) {
	pdu = &ProtocolDataUnit{}
	// Slave address
	address, err := readHex(adu[1:])
	if err != nil {
		return nil, fmt.Errorf("reading slave address: %w", err)
	}
	// Function code
	if pdu.FunctionCode, err = readHex(adu[3:]); err != nil {
		return nil, fmt.Errorf("reading function code: %w", err)
	}
	// Data
	dataEnd := len(adu) - 4
	data := adu[5:dataEnd]
	pdu.Data = make([]byte, hex.DecodedLen(len(data)))
	if _, err = hex.Decode(pdu.Data, data); err != nil {
		return nil, fmt.Errorf("decoding data: %w", err)
	}
	// LRC
	lrcVal, err := readHex(adu[dataEnd:])
	if err != nil {
		return nil, fmt.Errorf("reading LRC: %w", err)
	}
	// Calculate checksum
	var lrc lrc
	lrc.reset()
	lrc.pushByte(address).pushByte(pdu.FunctionCode).pushBytes(pdu.Data)
	if lrcVal != lrc.value() {
		return nil, fmt.Errorf("%w: response lrc '%v' does not match expected '%v'", ErrProtocolError, lrcVal, lrc.value())
	}
	return pdu, nil
}

// asciiSerialTransporter implements Transporter interface.
type asciiSerialTransporter struct {
	serialPort
}

func (mb *asciiSerialTransporter) Send(ctx context.Context, aduRequest []byte) (aduResponse []byte, err error) {
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
	mb.logf("modbus: sending %q\n", aduRequest)
	if _, err = mb.port.Write(aduRequest); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	// Check context after write
	if err = ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	// Get the response
	var n int
	var data [asciiMaxSize]byte
	length := 0
	for {
		// Check context before each read iteration
		if err = ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled: %w", err)
		}

		if n, err = mb.port.Read(data[length:]); err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
		length += n
		if length >= asciiMaxSize || n == 0 {
			break
		}
		// Expect end of frame in the data received
		if length > asciiMinSize {
			if string(data[length-len(asciiEnd):length]) == asciiEnd {
				break
			}
		}
	}
	aduResponse = data[:length]
	mb.logf("modbus: received %q\n", aduResponse)
	return aduResponse, nil
}

// writeHex encodes byte to string in hexadecimal, e.g. 0xA5 => "A5"
// (encoding/hex only supports lowercase string).
func writeHex(buf *bytes.Buffer, value []byte) (err error) {
	var str [2]byte
	for _, v := range value {
		str[0] = hexTable[v>>4]
		str[1] = hexTable[v&0x0F]

		if _, err = buf.Write(str[:]); err != nil {
			return
		}
	}
	return
}

// readHex decodes hexa string to byte, e.g. "8C" => 0x8C.
func readHex(data []byte) (value byte, err error) {
	var dst [1]byte
	if _, err = hex.Decode(dst[:], data[0:2]); err != nil {
		return
	}
	value = dst[0]
	return
}
