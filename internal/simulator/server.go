// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package simulator

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/lumberbarons/modbus"
)

const (
	rtuMinSize = 4
	rtuMaxSize = 256
)

// RTUServer implements a Modbus RTU server.
type RTUServer struct {
	handler  *Handler
	pty      *PtyPair
	slaveID  byte
	baudRate int
	logger   *log.Logger
	stopChan chan struct{}
	doneChan chan struct{}
}

// RTUServerConfig holds configuration for the RTU server.
type RTUServerConfig struct {
	SlaveID  byte
	BaudRate int
	Logger   *log.Logger
}

// NewRTUServer creates a new RTU server with the given data store and configuration.
func NewRTUServer(ds *DataStore, config *RTUServerConfig) (*RTUServer, error) {
	if config == nil {
		config = &RTUServerConfig{}
	}
	if config.SlaveID == 0 {
		config.SlaveID = 1
	}
	if config.BaudRate == 0 {
		config.BaudRate = 19200
	}
	if config.Logger == nil {
		config.Logger = log.New(os.Stdout, "rtu-server: ", log.LstdFlags)
	}

	pty, err := CreatePtyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to create pty: %w", err)
	}

	return &RTUServer{
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
func (s *RTUServer) ClientDevicePath() string {
	return s.pty.SlavePath
}

// Start starts the RTU server in a goroutine.
func (s *RTUServer) Start() error {
	go s.serve()
	// Give the server and socat time to fully initialize
	time.Sleep(200 * time.Millisecond)
	return nil
}

// Stop stops the RTU server and waits for it to finish.
func (s *RTUServer) Stop() error {
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
		// This is OK, it will be garbage collected
		s.logger.Printf("RTU server stop timed out (goroutine may still be reading)")
	}

	return nil
}

// serve is the main server loop that reads requests and sends responses.
func (s *RTUServer) serve() {
	defer close(s.doneChan)

	s.logger.Printf("RTU server listening - server pty: %s, client pty: %s (slave ID: %d)", s.pty.MasterPath, s.pty.SlavePath, s.slaveID)

	for {
		select {
		case <-s.stopChan:
			s.logger.Printf("RTU server stopping")
			return
		default:
			if err := s.handleRequest(); err != nil {
				if err == io.EOF {
					// File closed, stop serving
					s.logger.Printf("RTU server stopping (pty closed)")
					return
				}
				s.logger.Printf("error handling request: %v", err)
			}
		}
	}
}

// handleRequest reads a single request frame and sends a response.
func (s *RTUServer) handleRequest() error {
	// Set read timeout to allow checking stopChan periodically
	if err := s.pty.Master.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		// Ignore deadline errors - not critical
		s.logger.Printf("warning: failed to set read deadline: %v", err)
	}

	// Read RTU frame
	adu, err := s.readFrame()
	if err != nil {
		if os.IsTimeout(err) {
			// Timeout is expected, allows checking stopChan
			return nil
		}
		// Check if error is due to closed file (EOF or bad file descriptor)
		if err == io.EOF || err == os.ErrClosed {
			return io.EOF // Signal to stop serving
		}
		s.logger.Printf("error reading frame: %v", err)
		return nil // Continue serving on other errors
	}

	s.logger.Printf("received: % x", adu)

	// Decode the frame
	packager := &rtuPackager{SlaveID: s.slaveID}
	pdu, err := packager.Decode(adu)
	if err != nil {
		s.logger.Printf("failed to decode frame: %v", err)
		return nil // Don't stop server on bad frame
	}

	// Check slave ID
	if adu[0] != s.slaveID && adu[0] != 0 { // 0 is broadcast
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

	// Add frame delay (3.5 character times)
	delay := s.calculateDelay(len(adu))
	time.Sleep(delay)

	// Send the response
	s.logger.Printf("sending: % x", responseADU)
	n, err := s.pty.Master.Write(responseADU)
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	s.logger.Printf("wrote %d bytes", n)

	// Sync to ensure data is flushed
	if err := s.pty.Master.Sync(); err != nil {
		s.logger.Printf("warning: failed to sync: %v", err)
	}

	return nil
}

// readFrame reads a complete RTU frame from the serial port.
func (s *RTUServer) readFrame() ([]byte, error) {
	var buffer [rtuMaxSize]byte

	// Read minimum frame size first
	n, err := io.ReadAtLeast(s.pty.Master, buffer[:], rtuMinSize)
	if err != nil {
		return nil, err
	}

	// Determine expected frame length based on function code
	expectedLength := s.calculateExpectedLength(buffer[:n])

	// Read remaining bytes if needed
	if expectedLength > n && expectedLength <= rtuMaxSize {
		n2, err := io.ReadFull(s.pty.Master, buffer[n:expectedLength])
		if err != nil {
			return nil, err
		}
		n += n2
	}

	return buffer[:n], nil
}

// calculateExpectedLength estimates the expected frame length based on the function code.
func (s *RTUServer) calculateExpectedLength(data []byte) int {
	if len(data) < 2 {
		return rtuMinSize
	}

	functionCode := data[1]

	// For write functions, check if we have enough data to read the length field
	switch functionCode {
	case modbus.FuncCodeWriteMultipleCoils, modbus.FuncCodeWriteMultipleRegisters:
		if len(data) >= 7 {
			byteCount := int(data[6])
			return 7 + byteCount + 2 // address(2) + quantity(2) + func(1) + slave(1) + byteCount(1) + data + crc(2)
		}
	case modbus.FuncCodeReadWriteMultipleRegisters:
		if len(data) >= 11 {
			byteCount := int(data[10])
			return 11 + byteCount + 2 // fixed header + data + crc
		}
	}

	// For most functions, the request is fixed size
	return s.getFixedRequestLength(functionCode)
}

// getFixedRequestLength returns the expected request length for fixed-size function codes.
func (s *RTUServer) getFixedRequestLength(functionCode byte) int {
	switch functionCode {
	case modbus.FuncCodeReadCoils,
		modbus.FuncCodeReadDiscreteInputs,
		modbus.FuncCodeReadHoldingRegisters,
		modbus.FuncCodeReadInputRegisters,
		modbus.FuncCodeWriteSingleCoil,
		modbus.FuncCodeWriteSingleRegister:
		return 8 // slave(1) + func(1) + address(2) + value(2) + crc(2)
	case modbus.FuncCodeMaskWriteRegister:
		return 10 // slave(1) + func(1) + address(2) + andMask(2) + orMask(2) + crc(2)
	case modbus.FuncCodeReadFIFOQueue:
		return 6 // slave(1) + func(1) + address(2) + crc(2)
	default:
		return rtuMaxSize // Unknown function, read maximum
	}
}

// calculateDelay calculates the frame delay based on baud rate.
// See MODBUS over Serial Line - Specification and Implementation Guide (page 13).
func (s *RTUServer) calculateDelay(chars int) time.Duration {
	var characterDelay, frameDelay int // microseconds

	if s.baudRate <= 0 || s.baudRate > 19200 {
		characterDelay = 750
		frameDelay = 1750
	} else {
		characterDelay = 15000000 / s.baudRate
		frameDelay = 35000000 / s.baudRate
	}

	return time.Duration(characterDelay*chars+frameDelay) * time.Microsecond
}

// rtuPackager implements Modbus RTU framing.
type rtuPackager struct {
	SlaveID byte
}

// Encode encodes a PDU into an RTU frame with slave ID and CRC.
func (p *rtuPackager) Encode(pdu *modbus.ProtocolDataUnit) ([]byte, error) {
	length := len(pdu.Data) + 4 // slave + func + data + crc(2)
	if length > rtuMaxSize {
		return nil, fmt.Errorf("modbus: frame length %d exceeds maximum %d", length, rtuMaxSize)
	}

	adu := make([]byte, length)
	adu[0] = p.SlaveID
	adu[1] = pdu.FunctionCode
	copy(adu[2:], pdu.Data)

	// Calculate and append CRC
	crc := crc16(adu[:length-2])
	adu[length-2] = byte(crc)
	adu[length-1] = byte(crc >> 8)

	return adu, nil
}

// Decode decodes an RTU frame into a PDU and verifies the CRC.
func (p *rtuPackager) Decode(adu []byte) (*modbus.ProtocolDataUnit, error) {
	length := len(adu)
	if length < rtuMinSize {
		return nil, fmt.Errorf("modbus: frame length %d is less than minimum %d", length, rtuMinSize)
	}

	// Verify CRC
	expectedCRC := crc16(adu[:length-2])
	actualCRC := uint16(adu[length-2]) | uint16(adu[length-1])<<8
	if actualCRC != expectedCRC {
		return nil, fmt.Errorf("modbus: CRC mismatch: expected %04x, got %04x", expectedCRC, actualCRC)
	}

	return &modbus.ProtocolDataUnit{
		FunctionCode: adu[1],
		Data:         adu[2 : length-2],
	}, nil
}
