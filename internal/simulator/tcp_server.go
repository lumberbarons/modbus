// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package simulator

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/lumberbarons/modbus"
)

const (
	tcpProtocolIdentifier uint16 = 0x0000
	tcpHeaderSize         uint16 = 7
	tcpMaxLength          uint16 = 260
)

// TCPServer implements a Modbus TCP server.
type TCPServer struct {
	handler  *Handler
	listener net.Listener
	address  string
	logger   *log.Logger
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// TCPServerConfig holds configuration for the TCP server.
type TCPServerConfig struct {
	Address string // e.g., "localhost:5020" or ":502"
	Logger  *log.Logger
}

// NewTCPServer creates a new TCP server with the given data store and configuration.
func NewTCPServer(ds *DataStore, config *TCPServerConfig) (*TCPServer, error) {
	if config == nil {
		config = &TCPServerConfig{}
	}
	if config.Address == "" {
		config.Address = "localhost:5020"
	}
	if config.Logger == nil {
		config.Logger = log.New(os.Stdout, "tcp-server: ", log.LstdFlags)
	}

	return &TCPServer{
		handler:  NewHandler(ds),
		address:  config.Address,
		logger:   config.Logger,
		stopChan: make(chan struct{}),
	}, nil
}

// Address returns the address the server is listening on.
func (s *TCPServer) Address() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.address
}

// Start starts the TCP server and begins accepting connections.
func (s *TCPServer) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}

	s.listener = listener
	s.logger.Printf("TCP server listening on %s", s.listener.Addr())

	s.wg.Add(1)
	go s.acceptLoop()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the TCP server and waits for all connections to close.
func (s *TCPServer) Stop() error {
	close(s.stopChan)

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all goroutines to finish
	s.wg.Wait()
	s.logger.Printf("TCP server stopped")
	return nil
}

// acceptLoop accepts new client connections.
func (s *TCPServer) acceptLoop() {
	defer s.wg.Done()

	for {
		// Set a deadline so Accept doesn't block forever
		if tcpListener, ok := s.listener.(*net.TCPListener); ok {
			if err := tcpListener.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
				s.logger.Printf("warning: failed to set accept deadline: %v", err)
			}
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				// Server is stopping, exit gracefully
				return
			default:
				// Check if it's a timeout (expected)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				// Check if listener is closed
				if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
					return
				}
				s.logger.Printf("error accepting connection: %v", err)
				continue
			}
		}

		s.logger.Printf("accepted connection from %s", conn.RemoteAddr())
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single client connection.
func (s *TCPServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	s.logger.Printf("handling connection from %s", conn.RemoteAddr())

	for {
		select {
		case <-s.stopChan:
			s.logger.Printf("closing connection from %s (server stopping)", conn.RemoteAddr())
			return
		default:
			// Set read deadline
			if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
				s.logger.Printf("warning: failed to set read deadline: %v", err)
				return
			}

			// Read MBAP header (7 bytes)
			header := make([]byte, tcpHeaderSize)
			_, err := io.ReadFull(conn, header)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is expected, allows checking stopChan
					continue
				}
				if err == io.EOF {
					s.logger.Printf("connection closed by %s", conn.RemoteAddr())
					return
				}
				s.logger.Printf("error reading header from %s: %v", conn.RemoteAddr(), err)
				return
			}

			// Parse MBAP header
			transactionID := binary.BigEndian.Uint16(header[0:2])
			protocolID := binary.BigEndian.Uint16(header[2:4])
			length := binary.BigEndian.Uint16(header[4:6])
			unitID := header[6]

			// Verify protocol ID
			if protocolID != tcpProtocolIdentifier {
				s.logger.Printf("invalid protocol ID: %d", protocolID)
				continue
			}

			// Validate length
			if length < 2 || length > tcpMaxLength {
				s.logger.Printf("invalid length: %d", length)
				continue
			}

			// Read PDU (length - 1 byte for unit ID)
			pduLength := int(length) - 1
			pduData := make([]byte, pduLength)
			_, err = io.ReadFull(conn, pduData)
			if err != nil {
				s.logger.Printf("error reading PDU from %s: %v", conn.RemoteAddr(), err)
				return
			}

			// Log the full request
			fullRequest := make([]byte, 0, len(header)+len(pduData))
			fullRequest = append(fullRequest, header...)
			fullRequest = append(fullRequest, pduData...)
			s.logger.Printf("received from %s: % x", conn.RemoteAddr(), fullRequest)

			// Extract function code and data
			functionCode := pduData[0]
			data := pduData[1:]

			// Create PDU
			pdu := &modbus.ProtocolDataUnit{
				FunctionCode: functionCode,
				Data:         data,
			}

			// Handle the request
			responsePDU := s.handler.HandleRequest(pdu)

			// Check if timeout simulation (no response)
			if responsePDU == nil {
				// Don't send any response - simulate timeout
				// Keep connection open but don't respond to this request
				continue
			}

			// Build response MBAP header
			responseLength := uint16(1 + 1 + len(responsePDU.Data)) // unit ID + function code + data
			responseHeader := make([]byte, tcpHeaderSize)
			binary.BigEndian.PutUint16(responseHeader[0:2], transactionID)
			binary.BigEndian.PutUint16(responseHeader[2:4], protocolID)
			binary.BigEndian.PutUint16(responseHeader[4:6], responseLength)
			responseHeader[6] = unitID

			// Build response PDU
			response := make([]byte, 0, len(responseHeader)+1+len(responsePDU.Data))
			response = append(response, responseHeader...)
			response = append(response, responsePDU.FunctionCode)
			response = append(response, responsePDU.Data...)

			s.logger.Printf("sending to %s: % x", conn.RemoteAddr(), response)

			// Send response
			if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				s.logger.Printf("warning: failed to set write deadline: %v", err)
				return
			}
			_, err = conn.Write(response)
			if err != nil {
				s.logger.Printf("error writing response to %s: %v", conn.RemoteAddr(), err)
				return
			}

			s.logger.Printf("wrote %d bytes to %s", len(response), conn.RemoteAddr())
		}
	}
}
