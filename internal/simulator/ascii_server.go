// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package simulator

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lumberbarons/modbus"
)

const (
	asciiStart   = ":"
	asciiEnd     = "\r\n"
	asciiMinSize = 11 // :AAFFDD..LRC\r\n minimum (1+2+2+2+2+2 = 11)
	asciiMaxSize = 513
)

// ASCIIServer implements a Modbus ASCII server.
type ASCIIServer struct {
	handler  *Handler
	pty      *PtyPair
	slaveID  byte
	baudRate int
	logger   *log.Logger
	stopChan chan struct{}
	doneChan chan struct{}
}

// ASCIIServerConfig holds configuration for the ASCII server.
type ASCIIServerConfig struct {
	SlaveID  byte
	BaudRate int
	Logger   *log.Logger
}

// NewASCIIServer creates a new ASCII server with the given data store and configuration.
func NewASCIIServer(ds *DataStore, config *ASCIIServerConfig) (*ASCIIServer, error) {
	if config == nil {
		config = &ASCIIServerConfig{}
	}
	if config.SlaveID == 0 {
		config.SlaveID = 1
	}
	if config.BaudRate == 0 {
		config.BaudRate = 19200
	}
	if config.Logger == nil {
		config.Logger = log.New(os.Stdout, "ascii-server: ", log.LstdFlags)
	}

	pty, err := CreatePtyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to create pty: %w", err)
	}

	return &ASCIIServer{
		handler:  NewHandler(ds),
		pty:      pty,
		slaveID:  config.SlaveID,
		baudRate: config.BaudRate,
		logger:   config.Logger,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}, nil
}

// ClientDevicePath returns the device path that clients should connect to.
func (s *ASCIIServer) ClientDevicePath() string {
	return s.pty.SlavePath
}

// Start starts the ASCII server in a goroutine.
func (s *ASCIIServer) Start() error {
	go s.serve()
	// Give the server and pty time to fully initialize
	time.Sleep(200 * time.Millisecond)
	return nil
}

// Stop stops the ASCII server and waits for it to finish.
func (s *ASCIIServer) Stop() error {
	close(s.stopChan)

	// Close the pty to unblock any pending reads
	if err := s.pty.Close(); err != nil {
		s.logger.Printf("error closing pty: %v", err)
	}

	// Wait for server goroutine to finish with a timeout
	select {
	case <-s.doneChan:
		// Clean shutdown
	case <-time.After(1 * time.Second):
		// Timeout - the goroutine is stuck in a blocking read
		s.logger.Printf("ASCII server stop timed out (goroutine may still be reading)")
	}

	return nil
}

// serve is the main server loop that reads requests and sends responses.
func (s *ASCIIServer) serve() {
	defer close(s.doneChan)

	s.logger.Printf("ASCII server listening - server pty: %s, client pty: %s (slave ID: %d)", s.pty.MasterPath, s.pty.SlavePath, s.slaveID)

	for {
		select {
		case <-s.stopChan:
			s.logger.Printf("ASCII server stopping")
			return
		default:
			if err := s.handleRequest(); err != nil {
				if err == io.EOF {
					// File closed, stop serving
					s.logger.Printf("ASCII server stopping (pty closed)")
					return
				}
				s.logger.Printf("error handling request: %v", err)
			}
		}
	}
}

// handleRequest reads a single request frame and sends a response.
func (s *ASCIIServer) handleRequest() error {
	// Set read timeout to allow checking stopChan periodically
	if err := s.pty.Master.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		// Ignore deadline errors - not critical (ptys don't support deadlines)
		s.logger.Printf("warning: failed to set read deadline: %v", err)
	}

	// Read ASCII frame
	adu, err := s.readFrame()
	if err != nil {
		if os.IsTimeout(err) {
			// Timeout is expected, allows checking stopChan
			return nil
		}
		// Check if error is due to closed file
		if err == io.EOF || err == os.ErrClosed {
			return io.EOF
		}
		s.logger.Printf("error reading frame: %v", err)
		return nil
	}

	s.logger.Printf("received: %s", strings.TrimSpace(string(adu)))

	// Decode the frame
	packager := &asciiPackager{SlaveID: s.slaveID}
	pdu, err := packager.Decode(adu)
	if err != nil {
		s.logger.Printf("failed to decode frame: %v", err)
		return nil
	}

	// Check slave ID
	slaveID := adu[1:3]
	expectedSlaveID := fmt.Sprintf("%02X", s.slaveID)
	if string(slaveID) != expectedSlaveID && string(slaveID) != "00" {
		// Not for us, ignore
		return nil
	}

	// Handle the request
	responsePDU := s.handler.HandleRequest(pdu)

	// Encode the response
	responseADU, err := packager.Encode(responsePDU)
	if err != nil {
		s.logger.Printf("failed to encode response: %v", err)
		return nil
	}

	s.logger.Printf("sending: %s", strings.TrimSpace(string(responseADU)))

	// Send the response
	n, err := s.pty.Master.Write(responseADU)
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	s.logger.Printf("wrote %d bytes", n)

	return nil
}

// readFrame reads a complete ASCII frame from the serial port.
// ASCII frames are: :<hex data>\r\n
func (s *ASCIIServer) readFrame() ([]byte, error) {
	var buffer bytes.Buffer
	tmpBuf := make([]byte, 1)

	// Read until we find the start character ':'
	for {
		n, err := s.pty.Master.Read(tmpBuf)
		if err != nil {
			return nil, err
		}
		if n > 0 && tmpBuf[0] == ':' {
			buffer.WriteByte(tmpBuf[0])
			break
		}
	}

	// Read until we find CRLF
	for {
		n, err := s.pty.Master.Read(tmpBuf)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			buffer.WriteByte(tmpBuf[0])
			// Check if we have CRLF at the end
			data := buffer.Bytes()
			if len(data) >= 2 && data[len(data)-2] == '\r' && data[len(data)-1] == '\n' {
				return data, nil
			}
			// Safety check to prevent reading too much
			if buffer.Len() > asciiMaxSize {
				return nil, fmt.Errorf("frame too large: %d bytes", buffer.Len())
			}
		}
	}
}

// asciiPackager implements Modbus ASCII framing.
type asciiPackager struct {
	SlaveID byte
}

// Encode encodes a PDU into an ASCII frame with slave ID and LRC.
// Format: :<SlaveID><FunctionCode><Data><LRC>\r\n (all in hex ASCII)
func (p *asciiPackager) Encode(pdu *modbus.ProtocolDataUnit) ([]byte, error) {
	var buf bytes.Buffer

	// Start character
	buf.WriteString(asciiStart)

	// Encode slave ID, function code, and data as hex
	buf.WriteString(fmt.Sprintf("%02X", p.SlaveID))
	buf.WriteString(fmt.Sprintf("%02X", pdu.FunctionCode))
	buf.WriteString(strings.ToUpper(hex.EncodeToString(pdu.Data)))

	// Calculate LRC (on binary data, not ASCII)
	binaryData := []byte{p.SlaveID, pdu.FunctionCode}
	binaryData = append(binaryData, pdu.Data...)
	lrc := lrc8(binaryData)

	// Append LRC as hex
	buf.WriteString(fmt.Sprintf("%02X", lrc))

	// End characters
	buf.WriteString(asciiEnd)

	return buf.Bytes(), nil
}

// Decode decodes an ASCII frame into a PDU and verifies the LRC.
func (p *asciiPackager) Decode(adu []byte) (*modbus.ProtocolDataUnit, error) {
	// Check minimum length: :<2 hex chars for ID><2 hex chars for FC><2 hex chars for LRC>\r\n = 11 chars
	if len(adu) < asciiMinSize {
		return nil, fmt.Errorf("frame too short: %d bytes", len(adu))
	}

	// Remove start and end characters
	if adu[0] != ':' {
		return nil, fmt.Errorf("missing start character")
	}
	if adu[len(adu)-2] != '\r' || adu[len(adu)-1] != '\n' {
		return nil, fmt.Errorf("missing end characters")
	}

	// Extract hex data (without : and \r\n)
	hexData := adu[1 : len(adu)-2]

	// Decode hex to binary
	binaryData, err := hex.DecodeString(string(hexData))
	if err != nil {
		return nil, fmt.Errorf("invalid hex data: %w", err)
	}

	// Check minimum binary length: SlaveID(1) + FunctionCode(1) + LRC(1) = 3
	if len(binaryData) < 3 {
		return nil, fmt.Errorf("decoded data too short: %d bytes", len(binaryData))
	}

	// Verify LRC
	dataWithoutLRC := binaryData[:len(binaryData)-1]
	expectedLRC := lrc8(dataWithoutLRC)
	actualLRC := binaryData[len(binaryData)-1]

	if actualLRC != expectedLRC {
		return nil, fmt.Errorf("LRC mismatch: expected %02X, got %02X", expectedLRC, actualLRC)
	}

	// Extract PDU (skip slave ID, include function code and data, exclude LRC)
	return &modbus.ProtocolDataUnit{
		FunctionCode: binaryData[1],
		Data:         binaryData[2 : len(binaryData)-1],
	}, nil
}
