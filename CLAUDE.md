# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go implementation of the Modbus protocol, supporting three communication modes:
- **TCP**: Modbus over TCP/IP networks
- **RTU**: Modbus over serial lines using binary encoding with CRC
- **ASCII**: Modbus over serial lines using ASCII encoding with LRC

The library provides a fault-tolerant, fail-fast client implementation for all standard Modbus function codes.

## Development Commands

**Build**:
```bash
go build -v ./...
```

**Run unit tests**:
```bash
go test -v -race .
```

**Run tests with coverage**:
```bash
go test -v -race -coverprofile=coverage.txt -covermode=atomic .
```

**Run linter**:
```bash
golangci-lint run --timeout=5m
```

**Run integration tests** (requires external Modbus simulator like diagslave):
```bash
go test -v ./integration
# Or run specific protocol tests:
go test -v -run TCP ./integration
go test -v -run RTU ./integration
go test -v -run ASCII ./integration
```

**Verify go.mod is tidy**:
```bash
go mod tidy
git diff --exit-code go.mod go.sum
```

## Architecture

The library uses a layered architecture that separates protocol concerns:

### Core Abstractions

1. **ProtocolDataUnit (PDU)**: Protocol-independent data structure containing function code and data
2. **Packager**: Encodes/decodes PDU into transport-specific frames (ADU - Application Data Unit)
3. **Transporter**: Handles physical communication (network/serial I/O)
4. **ClientHandler**: Interface combining Packager + Transporter

### Client Implementations

Each protocol has its own handler that implements both Packager and Transporter:

- **tcpclient.go**: `TCPClientHandler` - handles MBAP header, transaction IDs, TCP connection management
- **rtuclient.go**: `RTUClientHandler` - handles RTU framing with CRC-16 checksum
- **asciiclient.go**: `ASCIIClientHandler` - handles ASCII framing with LRC checksum

All three share the same `Client` interface (defined in api.go) with methods for all Modbus functions.

### Request Flow

1. Client method (e.g., `ReadHoldingRegisters`) creates a PDU
2. PDU is sent to `client.send()` (in client.go)
3. Packager encodes PDU → ADU with protocol-specific framing
4. Transporter sends ADU and receives response
5. Packager verifies and decodes response ADU → PDU
6. Client validates response and returns data

### Key Components

- **modbus.go**: Defines function codes, exception codes, core interfaces (Packager, Transporter), and ModbusError type
- **client.go**: Implements the `Client` interface with all Modbus function logic (validation, request/response handling)
- **api.go**: Defines the `Client` interface with all public methods
- **serial.go**: Common serial port functionality shared by RTU and ASCII
- **crc.go/lrc.go**: Checksum implementations for RTU (CRC-16) and ASCII (LRC)

## Implementation Details

### Connection Management

- TCP: Maintains persistent connection with idle timeout (default 60s)
- Serial (RTU/ASCII): Opens port on first use, closes after idle timeout
- All handlers implement `Connect()` and `Close()` for manual connection management
- Automatic reconnection on transporter failures

### Thread Safety

- TCP and ASCII transporters use mutexes to protect connection state
- RTU transporter does not lock in Send() - ensure external synchronization if sharing handlers
- Transaction IDs (TCP) use atomic operations for thread-safe increments

### Error Handling

- Modbus exceptions are returned as `*ModbusError` with function and exception codes
- Validation errors (quantity limits, value ranges) return standard Go errors
- Response validation checks data lengths, addresses, and checksums

### Serial Communication

- **RTU**: Calculates frame delays based on baud rate (3.5 character times between frames)
- **ASCII**: Reads until CRLF terminator or max buffer size
- Default serial config: 19200 baud, 8 data bits, 1 stop bit, even parity
- Timeout defaults to 5 seconds for both RTU and ASCII

## Testing

- **Unit tests**: Located in root directory (`*_test.go`), test individual components without external dependencies
- **Integration tests**: Located in `integration/` subdirectory, require a Modbus simulator (e.g., diagslave)
  - Use `socat` to create virtual serial port pairs for RTU/ASCII testing
  - See `integration/README.md` for setup instructions

## Go Version

Minimum Go version: 1.22 (specified in go.mod)
CI tests against Go 1.22, 1.23, 1.24, 1.25 on Ubuntu and macOS

- Always run golangci-lint on new code